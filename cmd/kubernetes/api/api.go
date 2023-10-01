package api

type CustomResourceMetadata struct {
	Name                       string                         `json:"name"`
	Uid                        string                         `json:"uid"`
	Generation                 int                            `json:"generation"`
	ResourceVersion            string                         `json:"resourceVersion"`
	OwnerReferences            []CustomResourceOwnerReference `json:"ownerReferences"`
	CreationTimestamp          string                         `json:"creationTimestamp"`
	DeletionGracePeriodSeconds int                            `json:"deletionGracePeriodSeconds,omitempty"`
	DeletionTimestamp          string                         `json:"deletionTimestamp,omitempty"`
	Finalizers                 []string                       `json:"finalizers"`
}

type CustomResourceOwnerReference struct {
	ApiVersion         string `json:"apiVersion"`
	Kind               string `json:"kind"`
	Name               string `json:"name"`
	Uid                string `json:"uid"`
	BlockOwnerDeletion bool   `json:"blockOwnerDeletion"`
}
