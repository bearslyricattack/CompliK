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

// Package models provides data structures for configuration and process information
// used throughout the process scanner application.
package models

import (
	"time"
)

// --- Configuration structures organized by domain ---

// ScannerConfig contains the core configuration for the scanner itself
type ScannerConfig struct {
	ProcPath     string        `yaml:"proc_path"`
	ScanInterval time.Duration `yaml:"scan_interval"`
	LogLevel     string        `yaml:"log_level"`
}

// LabelActionConfig contains configuration for label actions
type LabelActionConfig struct {
	Enabled bool              `yaml:"enabled"`
	Data    map[string]string `yaml:"data"`
}

// ActionsConfig aggregates all available automated actions
type ActionsConfig struct {
	Label LabelActionConfig `yaml:"label"`
}

// LarkNotificationConfig contains configuration for Lark notification channel
type LarkNotificationConfig struct {
	Webhook string `yaml:"webhook"`
}

// NotificationsConfig aggregates all notification channels
type NotificationsConfig struct {
	Lark   LarkNotificationConfig `yaml:"lark"`
	Region string                 `yaml:"region"`
}

// MetricsConfig contains configuration for Prometheus metrics
type MetricsConfig struct {
	Enabled       bool          `yaml:"enabled"`
	Port          int           `yaml:"port"`
	Path          string        `yaml:"path"`
	ReadTimeout   time.Duration `yaml:"read_timeout"`
	WriteTimeout  time.Duration `yaml:"write_timeout"`
	MaxRetries    int           `yaml:"max_retries"`
	RetryInterval time.Duration `yaml:"retry_interval"`
}

// APIConfig contains configuration for the HTTP API server
type APIConfig struct {
	Enabled bool `yaml:"enabled"`
	Port    int  `yaml:"port"`
}

// RuleSet defines a set of matching rules, all rules will be parsed as regular expressions
type RuleSet struct {
	Processes  []string `yaml:"processes"`
	Keywords   []string `yaml:"keywords"`
	Commands   []string `yaml:"commands"`
	Namespaces []string `yaml:"namespaces"`
	PodNames   []string `yaml:"podNames"`
}

// DetectionRules contains both blacklist and whitelist rule sets
type DetectionRules struct {
	Blacklist RuleSet `yaml:"blacklist"`
	Whitelist RuleSet `yaml:"whitelist"`
}

// Config is the final, unified top-level configuration structure
type Config struct {
	Scanner        ScannerConfig       `yaml:"scanner"`
	Actions        ActionsConfig       `yaml:"actions"`
	Notifications  NotificationsConfig `yaml:"notifications"`
	Metrics        MetricsConfig       `yaml:"metrics"`
	API            APIConfig           `yaml:"api"`
	DetectionRules DetectionRules      `yaml:"detectionRules"`
}

// --- Business data models ---

// ProcessInfo stores complete information for a single detected suspicious process
type ProcessInfo struct {
	PID         int
	ProcessName string
	Command     string
	PodName     string
	Namespace   string
	ContainerID string
	Timestamp   string
	Message     string
	PodLabels   map[string]string // Pod 的 labels
	AppType     string            // "app" 或 "devbox"
	AppName     string            // 应用名称
	MatchedRule string            // 匹配的正则规则
}

// ViolationRecord 表示不合规应用的完整记录信息
// 用于API返回和聚合服务处理
type ViolationRecord struct {
	Pod       string `json:"pod"`       // Pod 名称
	Namespace string `json:"namespace"` // 命名空间
	Process   string `json:"process"`   // 进程名称
	Cmdline   string `json:"cmdline"`   // 完整命令行
	Regex     string `json:"regex"`     // 匹配的正则表达式规则
	Status    string `json:"status"`    // 状态（例如：active, blocked）
	Type      string `json:"type"`      // 类型（app 或 devbox）
	Name      string `json:"name"`      // 应用名称（app label 或 devbox name）
	Timestamp string `json:"timestamp"` // 检测时间
}
