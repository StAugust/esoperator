package esoperator

import (
	"fmt"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	ResourceKind   = "ElasticsearchCluster"
	ResourcePlural = "elasticsearchclusters"
	GroupName      = "augusto.cn"
	ShortName      = "elasticsearchcluster"
	Version        = "v1"
)

var (
	Name               = fmt.Sprintf("%s.%s", ResourcePlural, GroupName)
	SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: Version}
)
