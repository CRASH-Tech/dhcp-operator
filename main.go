package main

import (
	"context"
	"flag"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/CRASH-Tech/dhcp-operator/cmd/common"
	"github.com/CRASH-Tech/dhcp-operator/cmd/kubernetes"
	"github.com/CRASH-Tech/dhcp-operator/cmd/kubernetes/api"
	"github.com/CRASH-Tech/dhcp-operator/cmd/kubernetes/api/v1alpha1"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv4/server4"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/dynamic"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// type State struct {
// 	Pool   v1alpha1.Pool
// 	Lease  v1alpha1.Lease
// 	Leases []v1alpha1.Lease
// }

var (
	version = "0.0.1"
	config  common.Config
	kClient *kubernetes.Client
	mutex   sync.Mutex

	leaseExpiration = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "lease_expiration",
			Help: "The time to lease expiration",
		},
		[]string{
			"ip",
			"mac",
			"pool",
			"hostname",
		},
	)
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

	prometheus.MustRegister(leaseExpiration)
}

func main() {
	log.Infof("Starting dhcp-operator %s", version)

	ctx := context.Background()
	kClient = kubernetes.NewClient(ctx, *config.DynamicClient, *config.KubernetesClient)

	ticker := time.NewTicker(10 * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				metrics()
				leaseCleaner()
			}
		}
	}()

	listenPXE()

	laddr := &net.UDPAddr{
		IP:   net.ParseIP("0.0.0.0"),
		Port: config.DhcpPort,
	}

	server, err := server4.NewServer("", laddr, handler)
	if err != nil {
		log.Fatal(err)
	}

	server.Serve()
}

func metrics() {
	leases, err := kClient.V1alpha1().Lease().GetAll()
	if err != nil {
		log.Error(err)

		return
	}

	for _, lease := range leases {
		ends, err := strconv.ParseInt(lease.Status.Ends, 10, 64)
		if err != nil {
			log.Error(err)

			return
		}

		leaseExpiration.WithLabelValues(
			lease.Spec.Ip,
			lease.Spec.Mac,
			lease.Spec.Pool,
			lease.Status.Hostname,
		).Set(float64(ends - time.Now().Unix()))
	}
}

func handler(conn net.PacketConn, peer net.Addr, msg *dhcpv4.DHCPv4) {
	mutex.Lock()
	defer mutex.Unlock()

	switch msgType := msg.MessageType(); msgType {
	case dhcpv4.MessageTypeDiscover:
		discover(conn, peer, *msg)

	case dhcpv4.MessageTypeRequest:
		request(conn, peer, *msg)

	case dhcpv4.MessageTypeInform:
		log.Info("INFORM")

	case dhcpv4.MessageTypeRelease:
		release(conn, peer, *msg)

	default:
		log.Info(msg.MessageType())
	}
}

func discover(conn net.PacketConn, peer net.Addr, msg dhcpv4.DHCPv4) {
	log.Debug("Received DISCOVER message:\n", msg.Summary())

	lease, found, err := getLease(msg)
	if err != nil {
		log.Error(err)

		return
	}
	///EXISTING LEASE
	if found {
		log.Debugf("Found existing lease IP: %s MAC: %s", lease.Spec.Ip, lease.Spec.Mac)
		reply, err := makeReply(msg, lease, dhcpv4.MessageTypeOffer)
		if err != nil {
			log.Error(err)

			return
		}

		err = sendReply(conn, peer, reply)
		if err != nil {
			log.Error(err)

			return
		}

		return
	}

	//NEW LEASE
	var rIP net.IP
	var requested bool
	if msg.RequestedIPAddress() != nil && msg.RequestedIPAddress().String() != "0.0.0.0" {
		log.Debugf("New lease from requested IP: %s", msg.RequestedIPAddress().String())
		rIP = msg.RequestedIPAddress()
		requested = true
	} else {
		rIP = msg.GatewayIPAddr
		requested = false
	}

	pools, err := getAvialablePools(rIP, requested)
	if err != nil {
		log.Error(err)

		return
	}

	sort.Slice(pools[:], func(i, j int) bool {
		return pools[i].Spec.Priority < pools[j].Spec.Priority
	})

	for _, pool := range pools {
		ips, err := getAvialableIPs(pool, rIP, requested)
		if err != nil {
			log.Error(err)

			return
		}

		if len(ips) > 0 {
			lease, err := newLease(ips[0], pool, msg)
			if err != nil {
				log.Error(err)

				return
			}
			reply, err := makeReply(msg, lease, dhcpv4.MessageTypeAck)
			if err != nil {
				log.Error(err)

				return
			}

			err = sendReply(conn, peer, reply)
			if err != nil {
				log.Error(err)

				return
			}

			return
		}
	}

	log.Error("Cannot make reply, no avialable ips:\n", msg.Summary())
}

func request(conn net.PacketConn, peer net.Addr, msg dhcpv4.DHCPv4) {
	log.Debug("Received REQUEST message:\n", msg.Summary())

	lease, found, err := getLease(msg)
	if err != nil {
		log.Error(err)

		return
	}

	if found {
		reply, err := makeReply(msg, lease, dhcpv4.MessageTypeAck)
		if err != nil {
			log.Error(err)

			return
		}

		err = sendReply(conn, peer, reply)
		if err != nil {
			log.Error(err)

			return
		}

		return
	} else {
		log.Warn("Resend REQUEST to DISCOVER, because lease not found:\n", msg.Summary())
		discover(conn, peer, msg)

		return
	}
}

func release(conn net.PacketConn, peer net.Addr, msg dhcpv4.DHCPv4) {
	log.Debug("Received RELEASE message:\n", msg.Summary())

	lease, found, err := getLease(msg)
	if err != nil {
		log.Error(err)

		return
	}

	if found {
		err := kClient.V1alpha1().Lease().Delete(lease)
		if err != nil {
			log.Error(err)

			return
		}
	} else {
		log.Error("Cannot release lease, lease not found:\n", msg.Summary())

		return
	}
}

func makeReply(msg dhcpv4.DHCPv4, lease v1alpha1.Lease, msgType dhcpv4.MessageType) (*dhcpv4.DHCPv4, error) {
	reply, err := dhcpv4.NewReplyFromRequest(&msg)
	if err != nil {
		return nil, err
	}

	pool, err := kClient.V1alpha1().Pool().Get(lease.Spec.Pool)
	if err != nil {
		return reply, err
	}

	duration, err := time.ParseDuration(pool.Spec.Lease)
	if err != nil {
		return reply, err
	}

	lease.Spec.Static = pool.Spec.Permanent
	lease, err = kClient.V1alpha1().Lease().Patch(lease)
	if err != nil {
		return reply, err
	}

	lease, err = kClient.V1alpha1().Lease().Renew(lease, string(msg.Options.Get(dhcpv4.OptionHostName)), duration)
	if err != nil {
		return reply, err
	}

	poolMask, err := pool.GetMask()
	if err != nil {
		return reply, err
	}

	reply.UpdateOption(dhcpv4.OptMessageType(msgType))
	reply.YourIPAddr = net.ParseIP(lease.Spec.Ip)
	reply.UpdateOption(dhcpv4.OptServerIdentifier(msg.GatewayIPAddr)) //////////////////////////////////////////////////////////////////////////////////TODO: LOL
	reply.UpdateOption(dhcpv4.OptRequestedIPAddress(net.ParseIP(lease.Spec.Ip)))
	reply.UpdateOption(dhcpv4.OptSubnetMask(poolMask))
	reply.UpdateOption(dhcpv4.OptRouter(net.ParseIP(pool.Spec.Routers)))
	reply.UpdateOption(dhcpv4.OptDNS(pool.GetDNS()...))
	reply.UpdateOption(dhcpv4.OptNTPServers(pool.GetNTP()...))
	reply.UpdateOption(dhcpv4.OptIPAddressLeaseTime(duration))
	reply.UpdateOption(dhcpv4.OptHostName(lease.Status.Hostname))
	reply.UpdateOption(dhcpv4.OptBootFileName(pool.Spec.Filename))

	return reply, nil
}

func sendReply(conn net.PacketConn, peer net.Addr, msg *dhcpv4.DHCPv4) error {
	ipPort := strings.Split(peer.String(), ":")
	destIP := net.ParseIP(ipPort[0])
	destPort, err := strconv.Atoi(ipPort[1])
	if err != nil {
		return err
	}

	dest := &net.UDPAddr{IP: destIP, Port: destPort}
	_, err = conn.WriteTo(msg.ToBytes(), dest)
	if err != nil {
		return err
	}

	log.Debug("Reply message:\n", msg.Summary())

	return nil
}

func leaseCleaner() {
	log.Debug("Start lease cleaner...")
	mutex.Lock()
	defer mutex.Unlock()

	leases, err := kClient.V1alpha1().Lease().GetAll()
	if err != nil {
		log.Error(err)
	}

	for _, lease := range leases {
		if lease.Spec.Static {
			log.Debugf("Skip delete static lease: %s", lease)

			continue
		}

		e, err := strconv.ParseInt(lease.Status.Ends, 10, 64)
		if err != nil {
			log.Error(err)

			continue
		}
		ends := time.Unix(e, 0).Add(time.Duration(time.Minute * 5))

		if ends.Before(time.Now()) {
			log.Debug("Delete expired lease: %s", lease)
			err := kClient.V1alpha1().Lease().Delete(lease)
			if err != nil {
				log.Error(err)
			}
		}
	}
}

func getLease(msg dhcpv4.DHCPv4) (v1alpha1.Lease, bool, error) {
	leases, err := kClient.V1alpha1().Lease().GetAll()
	if err != nil {
		return v1alpha1.Lease{}, false, err
	}

	for _, lease := range leases {
		if lease.Spec.Mac == msg.ClientHWAddr.String() {
			return lease, true, nil
		}
	}
	//TODO: CHECK IS LEASE IN RIGHT SUBNET////////////
	return v1alpha1.Lease{}, false, err
}

// func makeLease(msg dhcpv4.DHCPv4) (v1alpha1.Lease, error) {
// 	var rIP net.IP
// 	var requested bool
// 	if msg.RequestedIPAddress() != nil && msg.RequestedIPAddress().String() != "0.0.0.0" {
// 		log.Debugf("New lease from requested IP: %s", msg.RequestedIPAddress().String())
// 		rIP = msg.RequestedIPAddress()
// 		requested = true
// 	} else {
// 		rIP = msg.GatewayIPAddr
// 		requested = false
// 	}

// 	pools, err := getAvialablePools(rIP, requested)
// 	if err != nil {
// 		log.Error(err)

// 		return
// 	}

// 	sort.Slice(pools[:], func(i, j int) bool {
// 		return pools[i].Spec.Priority < pools[j].Spec.Priority
// 	})

// 	for _, pool := range pools {
// 		ips, err := getAvialableIPs(pool, rIP, requested)
// 		if err != nil {
// 			log.Error(err)

// 			return
// 		}

// 		if len(ips) > 0 {
// 			lease, err := newLease(ips[0], pool, msg)
// 			if err != nil {
// 				log.Error(err)

// 				return
// 			}
// 			reply, err := makeReply(msg, lease, dhcpv4.MessageTypeAck)
// 			if err != nil {
// 				log.Error(err)

// 				return
// 			}

// 			err = sendReply(conn, peer, reply)
// 			if err != nil {
// 				log.Error(err)

// 				return
// 			}

// 			return
// 		}
// 	}
// }

func newLease(ip net.IP, pool v1alpha1.Pool, msg dhcpv4.DHCPv4) (v1alpha1.Lease, error) {
	log.Debugf("Create new lease. IP: %s MAC: %s", ip.String(), msg.ClientHWAddr.String())

	duration, err := time.ParseDuration(pool.Spec.Lease)
	if err != nil {
		return v1alpha1.Lease{}, err
	}

	ownerReference := api.CustomResourceOwnerReference{
		ApiVersion:         pool.APIVersion,
		Kind:               pool.Kind,
		Name:               pool.Metadata.Name,
		Uid:                pool.Metadata.Uid,
		BlockOwnerDeletion: true,
	}

	lease := v1alpha1.Lease{}
	lease.Metadata.Name = ip.String()
	lease.Metadata.OwnerReferences = []api.CustomResourceOwnerReference{ownerReference}
	lease.Spec.Ip = ip.String()
	lease.Spec.Mac = msg.ClientHWAddr.String()
	lease.Spec.Pool = pool.Metadata.Name
	lease.Spec.Static = pool.Spec.Permanent
	lease.Status.Ends = strconv.FormatInt(time.Now().Add(duration).Unix(), 10)

	lease, err = kClient.V1alpha1().Lease().Create(lease)
	if err != nil {
		return lease, err
	}

	return lease, nil
}
