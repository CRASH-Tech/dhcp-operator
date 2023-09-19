package api

type CustomResourceMetadata struct {
	Name                       string   `json:"name"`
	Uid                        string   `json:"uid"`
	Generation                 int      `json:"generation"`
	ResourceVersion            string   `json:"resourceVersion"`
	CreationTimestamp          string   `json:"creationTimestamp"`
	DeletionGracePeriodSeconds int      `json:"deletionGracePeriodSeconds,omitempty"`
	DeletionTimestamp          string   `json:"deletionTimestamp,omitempty"`
	Finalizers                 []string `json:"finalizers"`
}
