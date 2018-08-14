package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object


type EsOperator struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              EsOperatorSpec `json:"spce"`
}

type EsOperatorSpec struct {
	Message   string `json:"message"`
	SomeValue *int32 `json:"someValue"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type EsOperatorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []EsOperator `json:"items"`
}
