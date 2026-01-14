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

// Package config 提供配置加载功能
package config

import (
	"fmt"
	"os"
	"time"

	"github.com/bearslyricattack/CompliK/procscan-aggregator/pkg/models"
	"gopkg.in/yaml.v3"
)

// LoadConfig 从文件加载配置
func LoadConfig(configPath string) (*models.Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config models.Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// 设置默认值
	setDefaults(&config)

	// 验证配置
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &config, nil
}

// setDefaults 设置默认配置值
func setDefaults(config *models.Config) {
	if config.Aggregator.ScanInterval == "" {
		config.Aggregator.ScanInterval = "60s"
	}
	if config.Aggregator.Port == 0 {
		config.Aggregator.Port = 8090
	}
	if config.DaemonSet.Namespace == "" {
		config.DaemonSet.Namespace = "block-system"
	}
	if config.DaemonSet.ServiceName == "" {
		config.DaemonSet.ServiceName = "procscan"
	}
	if config.DaemonSet.APIPort == 0 {
		config.DaemonSet.APIPort = 9090
	}
	if config.DaemonSet.APIPath == "" {
		config.DaemonSet.APIPath = "/api/violations"
	}
	if config.Logger.Level == "" {
		config.Logger.Level = "info"
	}
	if config.Logger.Format == "" {
		config.Logger.Format = "json"
	}
}

// validateConfig 验证配置
func validateConfig(config *models.Config) error {
	// 验证 ScanInterval 是否为有效的 duration
	if _, err := time.ParseDuration(config.Aggregator.ScanInterval); err != nil {
		return fmt.Errorf("invalid scan_interval '%s': %w", config.Aggregator.ScanInterval, err)
	}

	// 验证端口范围
	if config.Aggregator.Port < 1 || config.Aggregator.Port > 65535 {
		return fmt.Errorf("invalid aggregator port %d: must be between 1 and 65535", config.Aggregator.Port)
	}

	if config.DaemonSet.APIPort < 1 || config.DaemonSet.APIPort > 65535 {
		return fmt.Errorf("invalid daemonset api_port %d: must be between 1 and 65535", config.DaemonSet.APIPort)
	}

	// 验证必填字段
	if config.DaemonSet.Namespace == "" {
		return fmt.Errorf("daemonset namespace is required")
	}

	if config.DaemonSet.ServiceName == "" {
		return fmt.Errorf("daemonset service_name is required")
	}

	return nil
}

// GetScanInterval 获取解析后的扫描间隔
func GetScanInterval(config *models.Config) (time.Duration, error) {
	return time.ParseDuration(config.Aggregator.ScanInterval)
}
