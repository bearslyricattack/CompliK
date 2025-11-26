/*
Copyright 2025 CompliK Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package utils provides utility functions for the block-controller.
// This includes resource quota creation and management utilities.
package utils

import (
	"github.com/bearslyricattack/CompliK/block-controller/internal/constants"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateResourceQuota creates a ResourceQuota object that restricts resource creation in a namespace.
// It sets all resource limits to 0 to effectively block new resource creation.
// If blockStorage is true, it also restricts storage requests.
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
