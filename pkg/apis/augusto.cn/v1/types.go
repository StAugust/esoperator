package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EsCluster is a specification for a EsCluster resource
type EsCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EsClusterSpec   `json:"spec"`
	Status EsClusterStatus `json:"status"`
}

// EsClusterSpec is the spec for a EsCluster resource
type EsClusterSpec struct {
	Message   string `json:"message"`
	SomeValue *int32 `json:"someValue"`
}

// EsClusterStatus is the status for a EsCluster resource
type EsClusterStatus struct {
	State string `json:"state"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EsClusterList is a list of EsCluster resources
type EsClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []EsCluster `json:"items"`
}
