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

package config

import (
	"os"
	"testing"
	"time"

	"github.com/bearslyricattack/CompliK/procscan-aggregator/pkg/models"
)

func TestLoadConfig(t *testing.T) {
	// 创建临时配置文件
	configContent := `
aggregator:
  scan_interval: "30s"
  port: 8080

daemonset:
  namespace: "test-namespace"
  service_name: "test-service"
  api_port: 9090
  api_path: "/api/test"

logger:
  level: "debug"
  format: "json"
`

	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(configContent); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	tmpFile.Close()

	// 加载配置
	cfg, err := LoadConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// 验证配置值
	if cfg.Aggregator.ScanInterval != "30s" {
		t.Errorf("Expected scan_interval '30s', got '%s'", cfg.Aggregator.ScanInterval)
	}

	if cfg.Aggregator.Port != 8080 {
		t.Errorf("Expected port 8080, got %d", cfg.Aggregator.Port)
	}

	if cfg.DaemonSet.Namespace != "test-namespace" {
		t.Errorf("Expected namespace 'test-namespace', got '%s'", cfg.DaemonSet.Namespace)
	}

	if cfg.DaemonSet.ServiceName != "test-service" {
		t.Errorf("Expected service_name 'test-service', got '%s'", cfg.DaemonSet.ServiceName)
	}

	// 测试 GetScanInterval
	interval, err := GetScanInterval(cfg)
	if err != nil {
		t.Fatalf("Failed to get scan interval: %v", err)
	}

	expectedInterval := 30 * time.Second
	if interval != expectedInterval {
		t.Errorf("Expected interval %v, got %v", expectedInterval, interval)
	}
}

func TestLoadConfigWithDefaults(t *testing.T) {
	// 创建最小配置文件
	configContent := `
daemonset:
  namespace: "test-namespace"
  service_name: "test-service"
`

	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(configContent); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	tmpFile.Close()

	// 加载配置
	cfg, err := LoadConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// 验证默认值
	if cfg.Aggregator.ScanInterval != "60s" {
		t.Errorf("Expected default scan_interval '60s', got '%s'", cfg.Aggregator.ScanInterval)
	}

	if cfg.Aggregator.Port != 8090 {
		t.Errorf("Expected default port 8090, got %d", cfg.Aggregator.Port)
	}

	if cfg.DaemonSet.APIPort != 9090 {
		t.Errorf("Expected default api_port 9090, got %d", cfg.DaemonSet.APIPort)
	}

	if cfg.Logger.Level != "info" {
		t.Errorf("Expected default log level 'info', got '%s'", cfg.Logger.Level)
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *models.Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: &models.Config{
				Aggregator: models.AggregatorConfig{
					ScanInterval: "60s",
					Port:         8090,
				},
				DaemonSet: models.DaemonSetConfig{
					Namespace:   "test",
					ServiceName: "service",
					APIPort:     9090,
				},
			},
			wantErr: false,
		},
		{
			name: "invalid scan interval",
			config: &models.Config{
				Aggregator: models.AggregatorConfig{
					ScanInterval: "invalid",
					Port:         8090,
				},
				DaemonSet: models.DaemonSetConfig{
					Namespace:   "test",
					ServiceName: "service",
					APIPort:     9090,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid port",
			config: &models.Config{
				Aggregator: models.AggregatorConfig{
					ScanInterval: "60s",
					Port:         70000,
				},
				DaemonSet: models.DaemonSetConfig{
					Namespace:   "test",
					ServiceName: "service",
					APIPort:     9090,
				},
			},
			wantErr: true,
		},
		{
			name: "missing namespace",
			config: &models.Config{
				Aggregator: models.AggregatorConfig{
					ScanInterval: "60s",
					Port:         8090,
				},
				DaemonSet: models.DaemonSetConfig{
					ServiceName: "service",
					APIPort:     9090,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
