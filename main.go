package main

import (
	"context"
	"encoding/binary"
	"errors"
	"flag"
	"io/ioutil"
	"net"
	"os"
	"time"

	"github.com/CRASH-Tech/dhcp-operator/cmd/common"
	"github.com/CRASH-Tech/dhcp-operator/cmd/kubernetes"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv4/server4"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
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

	pools, err := kClient.V1alpha1().Pool().GetAll()
	if err != nil {
		log.Error(err)

		return
	}
	log.Info(pools)
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
	//log.Print(msg.Summary())

	leaseTime, err := time.ParseDuration("1h")
	if err != nil {
		log.Fatalln("lease generation time error", err)
	}

	reply, err := dhcpv4.NewReplyFromRequest(msg)
	if err != nil {
		log.Fatalln("error in constructing offer response message", err)
	}

	//myIP := net.ParseIP("10.171.120.1")
	replyIP := net.ParseIP("10.171.120.254")
	//sIP := net.ParseIP("10.171.123.254")
	cIP := net.ParseIP("10.171.123.113")
	gIP := net.ParseIP("10.171.123.254")
	dIP := net.ParseIP("8.8.8.8")

	switch msgType := msg.MessageType(); msgType {
	case dhcpv4.MessageTypeDiscover:
		log.Info("DISCOVER")

		// reply := dhcpv4.DHCPv4{
		// 	OpCode: dhcpv4.OpcodeBootReply,
		// 	HWType: ,
		// }

		reply.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeOffer))
		reply.YourIPAddr = cIP
		reply.ServerIPAddr = net.ParseIP("0.0.0.0")
		reply.GatewayIPAddr = gIP
		//reply.UpdateOption(dhcpv4.OptServerIdentifier(net.ParseIP(sIP)))
		reply.UpdateOption(dhcpv4.OptRouter(gIP))
		reply.UpdateOption(dhcpv4.OptDNS(dIP))
		reply.UpdateOption(dhcpv4.OptIPAddressLeaseTime(leaseTime))
		reply.UpdateOption(dhcpv4.OptSubnetMask(net.IPv4Mask(255, 255, 255, 0)))
		reply.UpdateOption(dhcpv4.OptBootFileName("test.pxe"))

		//dest := &net.UDPAddr{IP: net.ParseIP("255.255.255.255"), Port: dhcpv4.ClientPort}
		dest := &net.UDPAddr{IP: replyIP, Port: 67}
		n, err := conn.WriteTo(reply.ToBytes(), dest)
		if err != nil {
			log.Println(reply)
			log.Fatalln("error writing offer response message", err)
		}
		log.Println("write offer package successfully: ", n)
		log.Print(reply.Summary())
	case dhcpv4.MessageTypeRequest:
		log.Info("REQUEST")
		// reply := dhcpv4.DHCPv4{
		// 	OpCode: dhcpv4.OpcodeBootReply,
		// 	HWType: ,
		// }

		reply.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeAck))
		reply.YourIPAddr = cIP
		reply.ServerIPAddr = net.ParseIP("0.0.0.0")
		reply.GatewayIPAddr = gIP
		//reply.UpdateOption(dhcpv4.OptServerIdentifier(net.ParseIP(sIP)))
		reply.UpdateOption(dhcpv4.OptRouter(gIP))
		reply.UpdateOption(dhcpv4.OptDNS(dIP))
		reply.UpdateOption(dhcpv4.OptIPAddressLeaseTime(leaseTime))
		reply.UpdateOption(dhcpv4.OptSubnetMask(net.IPv4Mask(255, 255, 255, 0)))
		reply.UpdateOption(dhcpv4.OptBootFileName("http://10.4.122.2/pxe/talos-1.4.3.ipxe"))
		//dest := &net.UDPAddr{IP: net.ParseIP("255.255.255.255"), Port: dhcpv4.ClientPort}
		dest := &net.UDPAddr{IP: net.ParseIP("255.255.255.255"), Port: 67}
		n, err := conn.WriteTo(reply.ToBytes(), dest)
		if err != nil {
			log.Println(reply)
			log.Fatalln("error writing offer response message", err)
		}
		log.Println("write offer package successfully: ", n)
		log.Print(reply.Summary())
	case dhcpv4.MessageTypeInform:
		log.Info("INFORM")
	default:
		log.Info(msg.MessageType())
	}
}

func readConfig(path string) (common.Config, error) {
	config := common.Config{}

	yamlFile, err := ioutil.ReadFile(path)
	if err != nil {
		return common.Config{}, err
	}
	err = yaml.Unmarshal(yamlFile, &config)
	if err != nil {
		return common.Config{}, err
	}

	return config, err
}

func checkValidNetmask(netmask net.IPMask) bool {
	netmaskInt := binary.BigEndian.Uint32(netmask)
	x := ^netmaskInt
	y := x + 1
	return (y & x) == 0
}

func getNetmask(ipMask string) (net.IPMask, error) {
	netmaskIP := net.ParseIP(ipMask)
	if netmaskIP.IsUnspecified() {
		return nil, errors.New("invalid subnet mask")
	}

	netmaskIP = netmaskIP.To4()
	if netmaskIP == nil {
		return nil, errors.New("error converting subnet mask to IPv4 format")
	}

	netmask := net.IPv4Mask(netmaskIP[0], netmaskIP[1], netmaskIP[2], netmaskIP[3])
	if !checkValidNetmask(netmask) {
		return nil, errors.New("illegal subnet mask")
	}
	return netmask, nil
}
