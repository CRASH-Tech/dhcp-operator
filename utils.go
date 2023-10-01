package main

import (
	"bytes"
	"encoding/binary"
	"io/ioutil"
	"net"

	"github.com/CRASH-Tech/dhcp-operator/cmd/common"
	"github.com/CRASH-Tech/dhcp-operator/cmd/kubernetes/api/v1alpha1"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

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

func getAvialablePools(ip net.IP, requested bool) ([]v1alpha1.Pool, error) {
	var result []v1alpha1.Pool

	pools, err := kClient.V1alpha1().Pool().GetAll()
	if err != nil {
		return result, err
	}

	for _, pool := range pools {
		_, poolNet, err := net.ParseCIDR(pool.Spec.Subnet)
		if err != nil {
			log.Error(err)

			continue
		}

		if requested {
			if poolNet.Contains(ip) && isIPInPool(ip, pool) {
				result = append(result, pool)
			}
		} else {
			if poolNet.Contains(ip) {
				result = append(result, pool)
			}
		}

	}

	return result, nil
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

func isIPInPool(ip net.IP, pool v1alpha1.Pool) bool {
	if pool.Spec.Start == "" || pool.Spec.End == "" || ip == nil {
		log.Errorf("Cannot find ip: %s-%s", pool.Spec.Start, pool.Spec.End)

		return false
	}

	from16 := net.ParseIP(pool.Spec.Start)
	to16 := net.ParseIP(pool.Spec.End)
	test16 := ip.To16()

	if from16 == nil || to16 == nil || test16 == nil {
		log.Error("An ip did not convert to a 16 byte")

		return false
	}

	if bytes.Compare(test16, from16) >= 0 && bytes.Compare(test16, to16) <= 0 {
		return true
	}

	return false
}

func isIPFree(ip net.IP) bool {
	leases, err := kClient.V1alpha1().Lease().GetAll()
	if err != nil {
		return false
	}

	for _, lease := range leases {
		if lease.Spec.Ip == ip.String() {
			return false
		}
	}

	return true
}

func getAvialableIPs(pool v1alpha1.Pool, requestedIP net.IP, requested bool) ([]net.IP, error) {
	var result []net.IP

	if requested && requestedIP != nil && requestedIP.String() != "0.0.0.0" {
		if isIPFree(requestedIP) {
			result = append(result, requestedIP)

			return result, nil
		}
	}

	leases, err := kClient.V1alpha1().Lease().GetAll()
	if err != nil {
		return result, err
	}

	for _, ip := range cidrHosts(pool.Spec.Subnet) {
		var exists bool
		for _, lease := range leases {
			if lease.Spec.Ip == ip {
				exists = true

				break
			}
		}

		if !exists && isIPInPool(net.ParseIP(ip), pool) {
			result = append(result, net.ParseIP(ip))
		}
	}

	return result, nil
}
