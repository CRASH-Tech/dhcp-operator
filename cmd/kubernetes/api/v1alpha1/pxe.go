package v1alpha1

import "github.com/CRASH-Tech/dhcp-operator/cmd/kubernetes/api"

type PXE struct {
	APIVersion string                     `json:"apiVersion"`
	Kind       string                     `json:"kind"`
	Metadata   api.CustomResourceMetadata `json:"metadata"`
	Spec       PXESpec                    `json:"spec"`
}

type PXESpec struct {
	Data string `json:"data"`
}
