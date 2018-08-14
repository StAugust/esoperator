package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object


type EsCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              EsClusterSpec `json:"spce"`
	Status            EsStatus `json:"status"`
}

type EsClusterSpec struct {
	Message   string `json:"message"`
	SomeValue *int32 `json:"someValue"`
}

type EsStatus struct {
	state string `json:"state"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type EsClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []EsCluster `json:"items"`
}
