package v1alpha1

import (
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
	Priority  int      `json:"priority"`
	Subnet    string   `json:"subnet"`
	Start     string   `json:"start"`
	End       string   `json:"end"`
	Routers   string   `json:"routers"`
	Broadcast string   `json:"broadcast"`
	Dns       []string `json:"dns"`
	Ntp       []string `json:"ntp"`
	Domain    string   `json:"domain"`
	Lease     string   `json:"lease"`
	Filename  string   `json:"filename"`
	Permanent bool     `json:"permanent"`
}

func (pool *Pool) GetDNS() []net.IP {
	var result []net.IP

	for _, srv := range pool.Spec.Dns {
		result = append(result, net.ParseIP(srv))

	}

	return result
}

func (pool *Pool) GetNTP() []net.IP {
	var result []net.IP

	for _, srv := range pool.Spec.Ntp {
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
