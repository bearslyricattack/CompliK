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

// Package models 定义聚合服务的数据模型
package models

import "time"

// Config 聚合服务配置
type Config struct {
	Aggregator AggregatorConfig `yaml:"aggregator"`
	DaemonSet  DaemonSetConfig  `yaml:"daemonset"`
	Logger     LoggerConfig     `yaml:"logger"`
}

// AggregatorConfig 聚合器配置
type AggregatorConfig struct {
	ScanInterval time.Duration `yaml:"scan_interval"` // 扫描间隔
	Port         int           `yaml:"port"`          // HTTP 服务端口
}

// DaemonSetConfig DaemonSet 配置
type DaemonSetConfig struct {
	Namespace   string `yaml:"namespace"`    // DaemonSet 所在的命名空间
	ServiceName string `yaml:"service_name"` // Service 名称
	APIPort     int    `yaml:"api_port"`     // DaemonSet Pod 的 API 端口
	APIPath     string `yaml:"api_path"`     // API 路径
}

// LoggerConfig 日志配置
type LoggerConfig struct {
	Level  string `yaml:"level"`  // 日志级别
	Format string `yaml:"format"` // 日志格式：json/text
}

// ViolationRecord 不合规记录（与 procscan 中的定义保持一致）
type ViolationRecord struct {
	Pod       string `json:"pod"`       // Pod 名称
	Namespace string `json:"namespace"` // 命名空间
	Process   string `json:"process"`   // 进程名称
	Cmdline   string `json:"cmdline"`   // 完整命令行
	Regex     string `json:"regex"`     // 匹配的正则表达式规则
	Status    string `json:"status"`    // 状态
	Type      string `json:"type"`      // 类型（app 或 devbox）
	Name      string `json:"name"`      // 应用名称
	Timestamp string `json:"timestamp"` // 检测时间
}

// AggregatedViolations 聚合后的违规记录
type AggregatedViolations struct {
	Violations []*ViolationRecord `json:"violations"`
	UpdateTime time.Time          `json:"update_time"`
	TotalCount int                `json:"total_count"`
}
