package v1alpha1

import "github.com/CRASH-Tech/dhcp-operator/cmd/kubernetes/api"

type Lease struct {
	APIVersion string                     `json:"apiVersion"`
	Kind       string                     `json:"kind"`
	Metadata   api.CustomResourceMetadata `json:"metadata"`
	Spec       LeaseSpec                  `json:"spec"`
	Status     LeaseStatus                `json:"status"`
}

type LeaseSpec struct {
	Ip     string `json:"ip"`
	Mac    string `json:"mac"`
	Static bool   `json:"static"`
	Pool   string `json:"pool"`
}

type LeaseStatus struct {
	Hostname string `json:"hostname"`
	Starts   string `json:"starts"`
	Ends     string `json:"ends"`
}
