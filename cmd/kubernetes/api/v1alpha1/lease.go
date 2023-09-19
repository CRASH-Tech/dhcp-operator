package v1alpha1

import "github.com/CRASH-Tech/dhcp-operator/cmd/kubernetes/api"

type Lease struct {
	APIVersion string                     `json:"apiVersion"`
	Kind       string                     `json:"kind"`
	Metadata   api.CustomResourceMetadata `json:"metadata"`
	Spec       LeaseSpec                  `json:"spec"`
}

type LeaseSpec struct {
	Ip       string `json:"ip"`
	Mac      string `json:"mac"`
	Hostname string `json:"hostname"`
}
