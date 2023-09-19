package v1alpha1

import "github.com/CRASH-Tech/dhcp-operator/cmd/kubernetes/api"

type Pool struct {
	APIVersion string                     `json:"apiVersion"`
	Kind       string                     `json:"kind"`
	Metadata   api.CustomResourceMetadata `json:"metadata"`
	Spec       PoolSpec                   `json:"spec"`
}

type PoolSpec struct {
	Subnet    string   `json:"subnet"`
	Range     string   `json:"range"`
	Routers   string   `json:"routers"`
	Broadcast string   `json:"broadcast"`
	Dns       []string `json:"dns"`
	Domain    string   `json:"domain"`
	Lease     string   `json:"lease"`
	Filename  string   `json:"filename"`
}
