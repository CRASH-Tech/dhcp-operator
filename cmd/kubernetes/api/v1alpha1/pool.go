package v1alpha1

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"net"

	"github.com/CRASH-Tech/dhcp-operator/cmd/kubernetes/api"
	log "github.com/sirupsen/logrus"
)

type Pool struct {
	APIVersion string                     `json:"apiVersion"`
	Kind       string                     `json:"kind"`
	Metadata   api.CustomResourceMetadata `json:"metadata"`
	Spec       PoolSpec                   `json:"spec"`
}

type PoolSpec struct {
	Subnet    string   `json:"subnet"`
	Start     string   `json:"start"`
	End       string   `json:"end"`
	Routers   string   `json:"routers"`
	Broadcast string   `json:"broadcast"`
	Dns       []string `json:"dns"`
	Domain    string   `json:"domain"`
	Lease     string   `json:"lease"`
	Filename  string   `json:"filename"`
}

func (pool *Pool) FindLease(mac net.HardwareAddr, leases []Lease) (Lease, error) {
	for _, lease := range leases {
		if mac.String() == lease.Spec.Mac {
			return lease, nil
		}
	}

	return Lease{}, nil
}

func (pool *Pool) GetDNS() []net.IP {
	var result []net.IP

	for _, srv := range pool.Spec.Dns {
		result = append(result, net.ParseIP(srv))

	}

	return result
}

func (pool *Pool) GetMask() (net.IPMask, error) {
	_, poolIPNet, err := net.ParseCIDR(pool.Spec.Subnet)
	if err != nil {
		log.Error(err)

		return net.IPMask{}, err
	}

	return poolIPNet.Mask, nil
}

func (pool *Pool) FindFreeIP(requestedIP net.IP, mac net.HardwareAddr, leases []Lease) (net.IP, error) {
	if requestedIP != nil && requestedIP.String() != "0.0.0.0" && isIPFree(requestedIP.String(), leases) {
		return requestedIP, nil
	}

	for _, ip := range cidrHosts(pool.Spec.Subnet) {
		if isIpInRange(net.ParseIP(pool.Spec.Start), net.ParseIP(pool.Spec.End), net.ParseIP(ip)) {
			if isIPFree(ip, leases) {
				return net.ParseIP(ip), nil
			}

		}
	}

	return nil, errors.New("cannot find avialable ip")
}

func isIPFree(ip string, leases []Lease) bool {
	for _, lease := range leases {
		if lease.Spec.Ip == ip {
			return false
		}
	}

	return true
}

func cidrHosts(netw string) []string {
	// convert string to IPNet struct
	_, ipv4Net, err := net.ParseCIDR(netw)
	if err != nil {
		log.Fatal(err)
	}
	// convert IPNet struct mask and address to uint32
	mask := binary.BigEndian.Uint32(ipv4Net.Mask)
	// find the start IP address
	start := binary.BigEndian.Uint32(ipv4Net.IP)
	// find the final IP address
	finish := (start & mask) | (mask ^ 0xffffffff)
	// make a slice to return host addresses
	var hosts []string
	// loop through addresses as uint32.
	// I used "start + 1" and "finish - 1" to discard the network and broadcast addresses.
	for i := start + 1; i <= finish-1; i++ {
		// convert back to net.IPs
		// Create IP address of type net.IP. IPv4 is 4 bytes, IPv6 is 16 bytes.
		ip := make(net.IP, 4)
		binary.BigEndian.PutUint32(ip, i)
		hosts = append(hosts, ip.String())
	}
	// return a slice of strings containing IP addresses
	return hosts
}

func isIpInRange(from net.IP, to net.IP, test net.IP) bool {
	if from == nil || to == nil || test == nil {
		fmt.Sprintf("cannot find ip: %s-%s", from, to)
		return false
	}

	from16 := from.To16()
	to16 := to.To16()
	test16 := test.To16()

	if from16 == nil || to16 == nil || test16 == nil {
		log.Warning("An ip did not convert to a 16 byte") // or return an error!?
		return false
	}

	if bytes.Compare(test16, from16) >= 0 && bytes.Compare(test16, to16) <= 0 {
		return true
	}

	return false
}
