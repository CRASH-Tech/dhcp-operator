package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/CRASH-Tech/dhcp-operator/cmd/common"
	"github.com/CRASH-Tech/dhcp-operator/cmd/kubernetes"
	"github.com/CRASH-Tech/dhcp-operator/cmd/kubernetes/api/v1alpha1"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv4/server4"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/dynamic"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	version = "0.0.1"
	config  common.Config
	kClient *kubernetes.Client
)

func init() {
	var configPath string
	flag.StringVar(&configPath, "c", "config.yaml", "config file path. Default: config.yaml")
	c, err := readConfig(configPath)
	if err != nil {
		log.Fatal(err)
	}
	config = c

	switch config.Log.Format {
	case "text":
		log.SetFormatter(&log.TextFormatter{})
	case "json":
		log.SetFormatter(&log.JSONFormatter{})
	default:
		log.SetFormatter(&log.TextFormatter{})
	}

	switch config.Log.Level {
	case "info":
		log.SetLevel(log.InfoLevel)
	case "warn":
		log.SetLevel(log.WarnLevel)
	case "debug":
		log.SetLevel(log.DebugLevel)
	default:
		log.SetLevel(log.InfoLevel)
	}

	var restConfig *rest.Config
	if path, isSet := os.LookupEnv("KUBECONFIG"); isSet {
		log.Printf("Using configuration from '%s'", path)
		restConfig, err = clientcmd.BuildConfigFromFlags("", path)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		log.Info("Using in-cluster configuration")
		restConfig, err = rest.InClusterConfig()
		if err != nil {
			log.Fatal(err)
		}
	}
	config.DynamicClient = dynamic.NewForConfigOrDie(restConfig)
	config.KubernetesClient = k8s.NewForConfigOrDie(restConfig)
}

func main() {
	log.Infof("Starting dhcp-operator %s", version)

	ctx := context.Background()
	kClient = kubernetes.NewClient(ctx, *config.DynamicClient, *config.KubernetesClient)

	// pools, err := kClient.V1alpha1().Pool().GetAll()
	// if err != nil {
	// 	log.Error(err)

	// 	return
	// }
	// log.Info(pools)
	//////
	laddr := &net.UDPAddr{
		IP:   net.ParseIP("0.0.0.0"),
		Port: 67,
	}
	server, err := server4.NewServer("", laddr, handler)
	if err != nil {
		log.Fatal(err)
	}

	server.Serve()

}

func handler(conn net.PacketConn, peer net.Addr, msg *dhcpv4.DHCPv4) {
	switch msgType := msg.MessageType(); msgType {
	case dhcpv4.MessageTypeDiscover:
		discover(conn, peer, *msg)

	case dhcpv4.MessageTypeRequest:
		request(conn, peer, *msg)

	case dhcpv4.MessageTypeInform:
		log.Info("INFORM")

	case dhcpv4.MessageTypeRelease:
		log.Info("RELEASE")

	default:
		log.Info(msg.MessageType())
	}
}

func discover(conn net.PacketConn, peer net.Addr, msg dhcpv4.DHCPv4) {
	log.Debug("Received DISCOVER message:\n", msg.Summary())
	pool, err := getPool(msg.GatewayIPAddr)
	if err != nil {
		log.Error(err)

		return
	}

	leases, err := kClient.V1alpha1().Lease().GetAll()
	if err != nil {
		log.Error(err)

		return
	}

	lease, found, err := pool.FindLease(msg.ClientHWAddr, leases)
	if err != nil {
		log.Error(err)

		return
	}

	if !found {
		ip, err := pool.FindFreeIP(msg.RequestedIPAddress(), msg.ClientHWAddr, leases)
		if err != nil {
			log.Error(err)

			return
		}

		log.Debugf("Create new lease. IP: %s MAC: %s", ip.String(), msg.ClientHWAddr.String)
		lease = v1alpha1.Lease{}
		lease.Metadata.Name = strings.Replace(msg.ClientHWAddr.String(), ":", "-", -1)
		lease.Spec.Ip = ip.String()
		lease.Spec.Mac = msg.ClientHWAddr.String()
		lease.Spec.Hostname = msg.ServerHostName
		lease.Sttaus.Starts = time.Now().String()
		//lease.Sttaus.Ends = time.Now().String()

		lease, err = kClient.V1alpha1().Lease().Create(lease)
		if err != nil {
			log.Error(err)

			return
		}
	} else {
		log.Debug("Found existing lease: ", lease)
	}

	////UPDATE LEASE HERE

	reply, err := makeReply(msg, pool, lease, dhcpv4.MessageTypeOffer)
	if err != nil {
		log.Error(err)

		return
	}

	err = sendReply(conn, reply)
	if err != nil {
		log.Error(err)

		return
	}
}

func request(conn net.PacketConn, peer net.Addr, msg dhcpv4.DHCPv4) {
	log.Debug("Received REQUEST message:\n", msg.Summary())

	pool, err := getPool(msg.GatewayIPAddr)
	if err != nil {
		log.Error(err)

		return
	}

	leases, err := kClient.V1alpha1().Lease().GetAll()
	if err != nil {
		log.Error(err)

		return
	}

	lease, found, err := pool.FindLease(msg.ClientHWAddr, leases)
	if err != nil {
		log.Error(err)

		return
	}

	if !found {
		log.Debug("Request lease not found:\n", msg.Summary())

		return
	}

	reply, err := makeReply(msg, pool, lease, dhcpv4.MessageTypeAck)
	if err != nil {
		log.Error(err)

		return
	}

	err = sendReply(conn, reply)
	if err != nil {
		log.Error(err)

		return
	}
}

func sendReply(conn net.PacketConn, msg *dhcpv4.DHCPv4) error {
	dest := &net.UDPAddr{IP: msg.GatewayIPAddr, Port: 67}
	_, err := conn.WriteTo(msg.ToBytes(), dest)
	if err != nil {
		return err
	}

	log.Debug("Reply message:\n", msg.Summary())

	return nil
}

func makeReply(msg dhcpv4.DHCPv4, pool v1alpha1.Pool, lease v1alpha1.Lease, msgType dhcpv4.MessageType) (*dhcpv4.DHCPv4, error) {
	reply, err := dhcpv4.NewReplyFromRequest(&msg)
	if err != nil {
		log.Fatalln("error in constructing offer response message", err)
	}

	poolMask, err := pool.GetMask()
	if err != nil {
		log.Error(err)

		return reply, err
	}

	reply.UpdateOption(dhcpv4.OptMessageType(msgType))
	reply.YourIPAddr = net.ParseIP(lease.Spec.Ip)
	reply.UpdateOption(dhcpv4.OptSubnetMask(poolMask))
	reply.UpdateOption(dhcpv4.OptRouter(net.ParseIP(pool.Spec.Routers)))
	reply.UpdateOption(dhcpv4.OptDNS(pool.GetDNS()...))                ////////////
	reply.UpdateOption(dhcpv4.OptIPAddressLeaseTime(time.Second * 60)) /////////////////
	reply.UpdateOption(dhcpv4.OptHostName(lease.Spec.Hostname))
	reply.UpdateOption(dhcpv4.OptBootFileName(pool.Spec.Filename))

	return reply, nil
}

func getPool(ip net.IP) (v1alpha1.Pool, error) {
	pools, err := kClient.V1alpha1().Pool().GetAll()
	if err != nil {
		return v1alpha1.Pool{}, err
	}

	for _, pool := range pools {
		_, subnet, _ := net.ParseCIDR(pool.Spec.Subnet)
		if subnet.Contains(ip) {
			return pool, nil
		}
	}

	return v1alpha1.Pool{}, fmt.Errorf(fmt.Sprintf("pool not found: %s", ip))
}

// func getReply(msg *dhcpv4.DHCPv4) (*dhcpv4.DHCPv4, error) {
// 	pools, err := kClient.V1alpha1().Pool().GetAll()
// 	if err != nil {
// 		return msg, err
// 	}

// 	for _, pool := range pools {
// 		_, subnet, _ := net.ParseCIDR(pool.Spec.Subnet)
// 		if subnet.Contains(msg.GatewayIPAddr) {
// 			log.Debug("Found pool: ", pool)
// 			lease, err := findLease(msg)
// 			if err != nil {
// 				return msg, err
// 			}

// 			_, poolIPNet, err := net.ParseCIDR(pool.Spec.Subnet)
// 			if err != nil {
// 				return msg, err
// 			}

// 			msg.UpdateOption(dhcpv4.OptSubnetMask(poolIPNet.Mask))
// 			msg.UpdateOption(dhcpv4.OptRouter(net.IP(pool.Spec.Routers)))
// 			msg.UpdateOption(dhcpv4.OptDNS(net.IP("8.8.8.8")))               ////////////
// 			msg.UpdateOption(dhcpv4.OptIPAddressLeaseTime(time.Second * 60)) /////////////////
// 			msg.UpdateOption(dhcpv4.OptBootFileName(pool.Spec.Filename))

// 			///
// 			if msg.RequestedIPAddress().String()

// 			if lease.Spec.Ip != "" {
// 				log.Debug("Found existing lease: ", lease)
// 				msg.YourIPAddr = net.IP(lease.Spec.Ip)

// 				return msg, nil
// 			} else {
// 				ip, err := findFreeIP(msg, pool)
// 				if err != nil {
// 					return msg, err
// 				}

// 				msg.YourIPAddr = ip

// 				return msg, nil
// 			}
// 		}

// 	}

// 	return msg, errors.New("cannot get reply")
// }

// func findLease(msg *dhcpv4.DHCPv4) (v1alpha1.Lease, error) {
// 	leases, err := kClient.V1alpha1().Lease().GetAll()
// 	if err != nil {
// 		return v1alpha1.Lease{}, err
// 	}

// 	for _, lease := range leases {
// 		if msg.ClientHWAddr.String() == lease.Spec.Mac {
// 			log.Debug("Found lease: ", lease)

// 			return lease, nil
// 		}
// 	}

// 	return v1alpha1.Lease{}, nil
// }

// func findFreeIP(msg *dhcpv4.DHCPv4, pool v1alpha1.Pool) (net.IP, error) {
// 	leases, err := kClient.V1alpha1().Lease().GetAll()
// 	if err != nil {
// 		return nil, err
// 	}

// 	for _, lease := range leases {

// 	}
// }

// func isIpUsed(ip net.IP) (bool, error) {
// 	leases, err := kClient.V1alpha1().Lease().GetAll()
// 	if err != nil {
// 		return true, err
// 	}

// 	for _, lease := range leases {
// 		if lease.Spec.Ip == ip.String() {
// 			log.Debug("IP is already used: ", ip, lease)
// 			return true, nil
// 		}
// 	}

// 	return false, nil
// }

// func checkValidNetmask(netmask net.IPMask) bool {
// 	netmaskInt := binary.BigEndian.Uint32(netmask)
// 	x := ^netmaskInt
// 	y := x + 1
// 	return (y & x) == 0
// }

// func getNetmask(ipMask string) (net.IPMask, error) {
// 	netmaskIP := net.ParseIP(ipMask)
// 	if netmaskIP.IsUnspecified() {
// 		return nil, errors.New("invalid subnet mask")
// 	}

// 	netmaskIP = netmaskIP.To4()
// 	if netmaskIP == nil {
// 		return nil, errors.New("error converting subnet mask to IPv4 format")
// 	}

// 	netmask := net.IPv4Mask(netmaskIP[0], netmaskIP[1], netmaskIP[2], netmaskIP[3])
// 	if !checkValidNetmask(netmask) {
// 		return nil, errors.New("illegal subnet mask")
// 	}
// 	return netmask, nil
// }
