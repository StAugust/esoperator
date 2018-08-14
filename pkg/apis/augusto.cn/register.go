package esoperator

import (
	"fmt"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	ResourceKind   = "SkyESOperator"
	ResourcePlural = "esclusters"
	GroupName      = "augusto.cn"
	ShortName      = "escluster"
	Version        = "v1"
)

var (
	Name               = fmt.Sprintf("%s.%s", ResourcePlural, GroupName)
	SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: Version}
)
