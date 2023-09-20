package kubernetes

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/CRASH-Tech/dhcp-operator/cmd/kubernetes/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type Lease struct {
	client     *Client
	resourceId schema.GroupVersionResource
}

func (Lease *Lease) Create(l v1alpha1.Lease) (v1alpha1.Lease, error) {
	l.APIVersion = "dhcp.xfix.org/v1alpha1"
	l.Kind = "Lease"
	l.Metadata.CreationTimestamp = time.Now().Format("2006-01-02T15:04:05Z")

	if l.Spec.Ip == "" || l.Spec.Mac == "" {
		return v1alpha1.Lease{}, errors.New("cannot create lease, empty data!")
	}

	item, err := Lease.client.dynamicCreate(Lease.resourceId, &l)
	if err != nil {
		return v1alpha1.Lease{}, err
	}

	var result v1alpha1.Lease
	err = json.Unmarshal(item, &result)
	if err != nil {
		return v1alpha1.Lease{}, err
	}

	return result, nil
}

func (Lease *Lease) Get(name string) (v1alpha1.Lease, error) {
	item, err := Lease.client.dynamicGet(Lease.resourceId, name)
	if err != nil {
		return v1alpha1.Lease{}, err
	}

	var result v1alpha1.Lease
	err = json.Unmarshal(item, &result)
	if err != nil {
		return v1alpha1.Lease{}, err
	}

	return result, nil
}

func (Lease *Lease) GetAll() ([]v1alpha1.Lease, error) {
	items, err := Lease.client.dynamicGetAll(Lease.resourceId)
	if err != nil {
		panic(err)
	}

	var result []v1alpha1.Lease
	for _, item := range items {
		var q v1alpha1.Lease
		err = json.Unmarshal(item, &q)
		if err != nil {
			return nil, err
		}

		result = append(result, q)
	}

	return result, nil
}

func (Lease *Lease) Patch(m v1alpha1.Lease) (v1alpha1.Lease, error) {
	jsonData, err := json.Marshal(m)
	if err != nil {
		return v1alpha1.Lease{}, err
	}

	resp, err := Lease.client.dynamicPatch(Lease.resourceId, m.Metadata.Name, jsonData)
	if err != nil {
		return v1alpha1.Lease{}, err
	}

	var result v1alpha1.Lease
	err = json.Unmarshal(resp, &result)
	if err != nil {
		return v1alpha1.Lease{}, err
	}

	return result, nil
}

func (Lease *Lease) UpdateStatus(m v1alpha1.Lease) (v1alpha1.Lease, error) {
	jsonData, err := json.Marshal(m)
	if err != nil {
		return v1alpha1.Lease{}, err
	}

	resp, err := Lease.client.dynamicUpdateStatus(Lease.resourceId, m.Metadata.Name, jsonData)
	if err != nil {
		return v1alpha1.Lease{}, err
	}

	var result v1alpha1.Lease
	err = json.Unmarshal(resp, &result)
	if err != nil {
		return v1alpha1.Lease{}, err
	}

	return result, nil
}
