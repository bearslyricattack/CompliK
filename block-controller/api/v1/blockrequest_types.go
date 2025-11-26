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

// Package v1 contains API Schema definitions for the core v1 API group.
// It defines the BlockRequest CRD which is used to manage namespace blocking operations.
package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// BlockRequestSpec defines the desired state of BlockRequest
type BlockRequestSpec struct {
	// NamespaceNames is the list of target namespaces to perform the action on.
	// At least one of NamespaceNames or NamespaceSelector must be specified.
	// +optional
	NamespaceNames []string `json:"namespaceNames,omitempty"`

	// NamespaceSelector is a label selector for target namespaces.
	// At least one of NamespaceNames or NamespaceSelector must be specified.
	// +optional
	NamespaceSelector *metav1.LabelSelector `json:"namespaceSelector,omitempty"`

	// Action defines the action to be performed: 'locked' or 'active'
	// +kubebuilder:validation:Enum=locked;active
	Action string `json:"action"`
}

// NamespaceStatus represents the status of a single namespace operation
type NamespaceStatus struct {
	// Name is the name of the namespace
	Name string `json:"name"`
	// Message contains the result message of the operation
	Message string `json:"message"`
}

// BlockRequestStatus defines the observed state of BlockRequest
type BlockRequestStatus struct {
	// NamespaceStatuses contains the status of each namespace label update
	// +optional
	NamespaceStatuses []NamespaceStatus `json:"namespaceStatuses,omitempty"`

	// ProcessedNamespaceCount is the number of namespaces that have been processed
	// +optional
	ProcessedNamespaceCount int `json:"processedNamespaceCount,omitempty"`

	// SelectorContinueToken is the continue token for paginating through namespaces selected by the selector
	// +optional
	SelectorContinueToken string `json:"selectorContinueToken,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// BlockRequest is the Schema for the blockrequests API
type BlockRequest struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of BlockRequest
	// +required
	Spec BlockRequestSpec `json:"spec"`

	// status defines the observed state of BlockRequest
	// +optional
	Status BlockRequestStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// BlockRequestList contains a list of BlockRequest
type BlockRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BlockRequest `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BlockRequest{}, &BlockRequestList{})
}
