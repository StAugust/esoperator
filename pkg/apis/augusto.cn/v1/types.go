package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/api/core/v1"
)

const (
	SUBMITTED = "submitted"
	INITIALIZING = "initializing"
	RUNNING = "running"
	RECOVERING = "recovering"
	FAILED = "failed"
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
	EsImage  string          `json:"esimage"`
	Replicas *int32          `json:"replicas"`
	DataPath string          `json:"datapath",omitempty`
	Env      []corev1.EnvVar `json:"env,omitempty" patchStrategy:"merge" patchMergeKey:"name" protobuf:"bytes,7,rep,name=env"`
}

// EsClusterStatus is the status for a EsCluster resource
type EsClusterStatus struct {
	Phase         string               `json:"phase"`
	PodsStatus    []EsInstanceStatus   `json:"podsStatus"`
	ServiceStatus corev1.ServiceStatus `json:"svcStatus"`
}

type EsInstanceStatus struct {
	PodName     string           `json:"podName"`
	PodHostName string           `json:"podHostName"`
	NodeName    string           `json:"nodeName"`
	Status      corev1.PodStatus `json:"status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// EsClusterList is a list of EsCluster resources
type EsClusterList struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ListMeta   `json:"metadata"`
	Items []EsCluster `json:"items"`
}
