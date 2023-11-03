package kubernetes

import (
	"encoding/json"

	"github.com/CRASH-Tech/dhcp-operator/cmd/kubernetes/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type PXE struct {
	client     *Client
	resourceId schema.GroupVersionResource
}

func (PXE *PXE) Get(name string) (v1alpha1.PXE, error) {
	item, err := PXE.client.dynamicGet(PXE.resourceId, name)
	if err != nil {
		return v1alpha1.PXE{}, err
	}

	var result v1alpha1.PXE
	err = json.Unmarshal(item, &result)
	if err != nil {
		return v1alpha1.PXE{}, err
	}

	return result, nil
}

func (PXE *PXE) GetAll() ([]v1alpha1.PXE, error) {
	items, err := PXE.client.dynamicGetAll(PXE.resourceId)
	if err != nil {
		panic(err)
	}

	var result []v1alpha1.PXE
	for _, item := range items {
		var q v1alpha1.PXE
		err = json.Unmarshal(item, &q)
		if err != nil {
			return nil, err
		}

		result = append(result, q)
	}

	return result, nil
}
