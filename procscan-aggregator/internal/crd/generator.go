// Copyright 2025 CompliK Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package crd 提供 CRD 生成功能
package crd

import (
	"fmt"

	"github.com/bearslyricattack/CompliK/procscan-aggregator/pkg/models"
)

// Generator CRD 生成器
type Generator struct{}

// NewGenerator 创建新的 CRD 生成器
func NewGenerator() *Generator {
	return &Generator{}
}

// HigressWASMPluginCRD Higress WASM Plugin CRD 结构（示例）
type HigressWASMPluginCRD struct {
	APIVersion string                   `json:"apiVersion"`
	Kind       string                   `json:"kind"`
	Metadata   map[string]interface{}   `json:"metadata"`
	Spec       HigressWASMPluginCRDSpec `json:"spec"`
}

// HigressWASMPluginCRDSpec Higress WASM Plugin CRD Spec（示例）
type HigressWASMPluginCRDSpec struct {
	// TODO: 根据实际的 Higress WASM Plugin CRD 定义填充字段
	Violations []ViolationInfo `json:"violations"`
}

// NotificationCRD Notification CRD 结构（示例）
type NotificationCRD struct {
	APIVersion string                 `json:"apiVersion"`
	Kind       string                 `json:"kind"`
	Metadata   map[string]interface{} `json:"metadata"`
	Spec       NotificationCRDSpec    `json:"spec"`
}

// NotificationCRDSpec Notification CRD Spec（示例）
type NotificationCRDSpec struct {
	// TODO: 根据实际的 Notification CRD 定义填充字段
	Message    string          `json:"message"`
	Violations []ViolationInfo `json:"violations"`
}

// ViolationInfo 违规信息（用于 CRD）
type ViolationInfo struct {
	Pod       string `json:"pod"`
	Namespace string `json:"namespace"`
	Process   string `json:"process"`
	Type      string `json:"type"`
	Name      string `json:"name"`
}

// GenerateHigressWASMPluginCRD 生成 Higress WASM Plugin CRD
func (g *Generator) GenerateHigressWASMPluginCRD(violations []*models.ViolationRecord) *HigressWASMPluginCRD {
	violationInfos := make([]ViolationInfo, 0, len(violations))
	for _, v := range violations {
		violationInfos = append(violationInfos, ViolationInfo{
			Pod:       v.Pod,
			Namespace: v.Namespace,
			Process:   v.Process,
			Type:      v.Type,
			Name:      v.Name,
		})
	}

	return &HigressWASMPluginCRD{
		APIVersion: "extensions.higress.io/v1alpha1",
		Kind:       "WasmPlugin",
		Metadata: map[string]interface{}{
			"name":      "procscan-violations",
			"namespace": "default",
		},
		Spec: HigressWASMPluginCRDSpec{
			Violations: violationInfos,
		},
	}
}

// GenerateNotificationCRD 生成 Notification CRD
func (g *Generator) GenerateNotificationCRD(violations []*models.ViolationRecord) *NotificationCRD {
	violationInfos := make([]ViolationInfo, 0, len(violations))
	for _, v := range violations {
		violationInfos = append(violationInfos, ViolationInfo{
			Pod:       v.Pod,
			Namespace: v.Namespace,
			Process:   v.Process,
			Type:      v.Type,
			Name:      v.Name,
		})
	}

	message := fmt.Sprintf("检测到 %d 个不合规应用", len(violations))

	return &NotificationCRD{
		APIVersion: "notification.sealos.io/v1",
		Kind:       "Notification",
		Metadata: map[string]interface{}{
			"name":      "procscan-notification",
			"namespace": "default",
		},
		Spec: NotificationCRDSpec{
			Message:    message,
			Violations: violationInfos,
		},
	}
}
