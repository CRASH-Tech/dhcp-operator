package main

import (
	"encoding/binary"
	"errors"
	"net"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv4/server4"
	log "github.com/sirupsen/logrus"
)

func main() {
	laddr := &net.UDPAddr{
		IP:   net.ParseIP("0.0.0.0"),
		Port: 67,
	}
	server, err := server4.NewServer("", laddr, handler)
	if err != nil {
		log.Fatal(err)
	}

	server.Serve()
	//ExampleHandler()

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

	//log.Println(msg)

	// if msg.MessageType() == dhcpv4.MessageTypeDiscover {
	// 	tmp.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeOffer))
	// 	tmp.YourIPAddr = net.ParseIP(cIP)
	// 	tmp.ServerIPAddr = net.ParseIP(sIP)
	// 	tmp.GatewayIPAddr = net.ParseIP(gIP)
	// 	tmp.UpdateOption(dhcpv4.OptServerIdentifier(net.ParseIP(sIP)))
	// 	tmp.UpdateOption(dhcpv4.OptIPAddressLeaseTime(LeaseTime))
	// 	subnetMask, err := getNetmask("255.255.255.0")
	// 	if err != nil {
	// 		log.Fatalln(err)
	// 	}

	// 	tmp.UpdateOption(dhcpv4.OptSubnetMask(subnetMask))
	// 	//dest := &net.UDPAddr{IP: net.IPv4bcast, Port: dhcpv4.ClientPort}
	// 	dest := &net.UDPAddr{IP: net.ParseIP(gIP), Port: dhcpv4.ClientPort}
	// 	n, err := conn.WriteTo(tmp.ToBytes(), dest)
	// 	if err != nil {
	// 		log.Println(tmp)
	// 		log.Fatalln("error writing offer response message", err)
	// 	}
	// 	log.Println("write offer package successfully: ", n)
	// }
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
