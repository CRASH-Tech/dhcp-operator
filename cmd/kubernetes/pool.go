package kubernetes

import (
	"encoding/json"
	"time"

	"github.com/CRASH-Tech/dhcp-operator/cmd/kubernetes/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type Pool struct {
	client     *Client
	resourceId schema.GroupVersionResource
}

func (Pool *Pool) Create(p v1alpha1.Pool) (v1alpha1.Pool, error) {
	p.APIVersion = "dhcp.xfix.org/v1alpha1"
	p.Kind = "Pool"
	p.Metadata.CreationTimestamp = time.Now().Format("2006-01-02T15:04:05Z")

	item, err := Pool.client.dynamicCreate(Pool.resourceId, &p)
	if err != nil {
		return v1alpha1.Pool{}, err
	}

	var result v1alpha1.Pool
	err = json.Unmarshal(item, &result)
	if err != nil {
		return v1alpha1.Pool{}, err
	}

	return result, nil
}

func (Pool *Pool) Get(name string) (v1alpha1.Pool, error) {
	item, err := Pool.client.dynamicGet(Pool.resourceId, name)
	if err != nil {
		return v1alpha1.Pool{}, err
	}

	var result v1alpha1.Pool
	err = json.Unmarshal(item, &result)
	if err != nil {
		return v1alpha1.Pool{}, err
	}

	return result, nil
}

func (Pool *Pool) GetAll() ([]v1alpha1.Pool, error) {
	items, err := Pool.client.dynamicGetAll(Pool.resourceId)
	if err != nil {
		panic(err)
	}

	var result []v1alpha1.Pool
	for _, item := range items {
		var q v1alpha1.Pool
		err = json.Unmarshal(item, &q)
		if err != nil {
			return nil, err
		}

		result = append(result, q)
	}

	return result, nil
}

func (Pool *Pool) Patch(m v1alpha1.Pool) (v1alpha1.Pool, error) {
	jsonData, err := json.Marshal(m)
	if err != nil {
		return v1alpha1.Pool{}, err
	}

	resp, err := Pool.client.dynamicPatch(Pool.resourceId, m.Metadata.Name, jsonData)
	if err != nil {
		return v1alpha1.Pool{}, err
	}

	var result v1alpha1.Pool
	err = json.Unmarshal(resp, &result)
	if err != nil {
		return v1alpha1.Pool{}, err
	}

	return result, nil
}

func (Pool *Pool) UpdateStatus(m v1alpha1.Pool) (v1alpha1.Pool, error) {
	jsonData, err := json.Marshal(m)
	if err != nil {
		return v1alpha1.Pool{}, err
	}

	resp, err := Pool.client.dynamicUpdateStatus(Pool.resourceId, m.Metadata.Name, jsonData)
	if err != nil {
		return v1alpha1.Pool{}, err
	}

	var result v1alpha1.Pool
	err = json.Unmarshal(resp, &result)
	if err != nil {
		return v1alpha1.Pool{}, err
	}

	return result, nil
}
