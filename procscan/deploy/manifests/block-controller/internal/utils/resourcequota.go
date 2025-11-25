package utils

import (
	"github.com/bearslyricattack/CompliK/block-controller/internal/constants"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateResourceQuota(namespace string, blockStorage bool) *v1.ResourceQuota {
	resources := v1.ResourceList{
		"pods":                   resource.MustParse("0"),
		"services":               resource.MustParse("0"),
		"replicationcontrollers": resource.MustParse("0"),
		"secrets":                resource.MustParse("0"),
		"configmaps":             resource.MustParse("0"),
		"persistentvolumeclaims": resource.MustParse("0"),
		"services.nodeports":     resource.MustParse("0"),
		"services.loadbalancers": resource.MustParse("0"),
		"requests.cpu":           resource.MustParse("0"),
		"requests.memory":        resource.MustParse("0"),
		"limits.cpu":             resource.MustParse("0"),
		"limits.memory":          resource.MustParse("0"),
	}

	if blockStorage {
		resources["requests.storage"] = resource.MustParse("0")
	}

	return &v1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.ResourceQuotaName,
			Namespace: namespace,
		},
		Spec: v1.ResourceQuotaSpec{
			Hard: resources,
		},
	}
}
