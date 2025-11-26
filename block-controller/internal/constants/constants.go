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

// Package constants defines all constant values used throughout the block-controller.
// This includes finalizer names, status values, label/annotation keys, and resource names.
package constants

const (
	// BlockRequestFinalizer is the finalizer added to BlockRequest resources to ensure proper cleanup
	BlockRequestFinalizer = "core.clawcloud.run/finalizer"

	// LockedStatus indicates a namespace is in locked state
	LockedStatus = "locked"
	// ActiveStatus indicates a namespace is in active state
	ActiveStatus = "active"

	// StatusLabel is the label key used to mark namespace status (locked/active)
	StatusLabel = "clawcloud.run/status"
	// UnlockTimestampLabel is the label key used to store unlock timestamp
	UnlockTimestampLabel = "clawcloud.run/unlock-timestamp"
	// OriginalReplicasAnnotation is the annotation key used to store original replica count
	OriginalReplicasAnnotation = "core.clawcloud.run/original-replicas"
	// OriginalSuspendAnnotation is the annotation key used to store original suspend state
	OriginalSuspendAnnotation = "core.clawcloud.run/original-suspend"

	// ResourceQuotaName is the name of the ResourceQuota object created by block-controller
	ResourceQuotaName = "block-controller-quota"
)
