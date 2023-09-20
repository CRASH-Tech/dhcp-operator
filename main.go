package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
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

type State struct {
	Pool   v1alpha1.Pool
	Lease  v1alpha1.Lease
	Leases []v1alpha1.Lease
}

var (
	version = "0.0.1"
	config  common.Config
	kClient *kubernetes.Client
	mutex   sync.Mutex
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

	ticker := time.NewTicker(5 * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				leaseCleaner()
			}
		}
	}()

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
		log.Info("RELEASE")

	default:
		log.Info(msg.MessageType())
	}
}

func discover(conn net.PacketConn, peer net.Addr, msg dhcpv4.DHCPv4) {
	log.Debug("Received DISCOVER message:\n", msg.Summary())

	state, err := getState(msg)
	if err != nil {
		log.Error(err)

		return
	}

	if state.Lease.Metadata.Name == "" {
		ip, err := state.Pool.FindFreeIP(msg.RequestedIPAddress(), msg.ClientHWAddr, state.Leases)
		if err != nil {
			log.Error(err)

			return
		}

		log.Debugf("Create new lease. IP: %s MAC: %s", ip.String(), msg.ClientHWAddr.String())
		state.Lease = v1alpha1.Lease{}
		state.Lease.Metadata.Name = strings.Replace(msg.ClientHWAddr.String(), ":", "-", -1)
		state.Lease.Spec.Ip = ip.String()
		state.Lease.Spec.Mac = msg.ClientHWAddr.String()
		state.Lease.Spec.Hostname = msg.ServerHostName

		lease, err := kClient.V1alpha1().Lease().Create(state.Lease)
		if err != nil {
			log.Error(err)

			return
		}

		lease, err = kClient.V1alpha1().Lease().SetStart(lease)
		if err != nil {
			log.Error(err)

			return
		}

		state.Lease = lease
	} else {
		log.Debug("Found existing lease: ", state.Lease)
	}

	reply, err := makeReply(msg, state, dhcpv4.MessageTypeOffer)
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

	state, err := getState(msg)
	if err != nil {
		log.Error(err)

		return
	}

	if state.Lease.Metadata.Name == "" {
		log.Debug("Request lease not found:\n", msg.Summary())

		return
	}

	duration, err := time.ParseDuration(state.Pool.Spec.Lease)
	if err != nil {
		log.Error(err)

		return
	}

	state.Lease, err = kClient.V1alpha1().Lease().Renew(state.Lease, duration)
	if err != nil {
		log.Error(err)

		return
	}

	reply, err := makeReply(msg, state, dhcpv4.MessageTypeAck)
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

func getState(msg dhcpv4.DHCPv4) (State, error) {
	pool, err := getPool(msg.GatewayIPAddr)
	if err != nil {
		return State{}, err
	}

	leases, err := kClient.V1alpha1().Lease().GetAll()
	if err != nil {
		return State{}, err
	}

	lease, err := pool.FindLease(msg.ClientHWAddr, leases)
	if err != nil {
		return State{}, err
	}

	state := State{
		Pool:   pool,
		Lease:  lease,
		Leases: leases,
	}

	return state, nil
}

func makeReply(msg dhcpv4.DHCPv4, state State, msgType dhcpv4.MessageType) (*dhcpv4.DHCPv4, error) {
	reply, err := dhcpv4.NewReplyFromRequest(&msg)
	if err != nil {
		log.Fatalln("error in constructing offer response message", err)
	}

	poolMask, err := state.Pool.GetMask()
	if err != nil {
		log.Error(err)

		return reply, err
	}

	reply.UpdateOption(dhcpv4.OptMessageType(msgType))
	reply.YourIPAddr = net.ParseIP(state.Lease.Spec.Ip)
	reply.UpdateOption(dhcpv4.OptSubnetMask(poolMask))
	reply.UpdateOption(dhcpv4.OptRouter(net.ParseIP(state.Pool.Spec.Routers)))
	reply.UpdateOption(dhcpv4.OptDNS(state.Pool.GetDNS()...))          ////////////
	reply.UpdateOption(dhcpv4.OptIPAddressLeaseTime(time.Second * 60)) /////////////////
	reply.UpdateOption(dhcpv4.OptHostName(state.Lease.Spec.Hostname))
	reply.UpdateOption(dhcpv4.OptBootFileName(state.Pool.Spec.Filename))

	return reply, nil
}

func leaseCleaner() {
	log.Debug("Start lease cleaner")
	mutex.Lock()
	defer mutex.Unlock()

	leases, err := kClient.V1alpha1().Lease().GetAll()
	if err != nil {
		log.Error(err)
	}

	for _, lease := range leases {
		e, err := strconv.ParseInt(lease.Status.Ends, 10, 64)
		if err != nil {
			log.Error(err)

			return
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
