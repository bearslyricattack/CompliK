package config

import (
	"fmt"
	"log"
	"os"

	"github.com/bearslyricattack/CompliK/mining/internal/types"
	"github.com/bearslyricattack/CompliK/mining/pkg/utils"
	"gopkg.in/yaml.v3"
)

// Manager 配置管理器
type Manager struct {
	configPath string
	config     types.Config
}

// NewManager 创建配置管理器
func NewManager(configPath string) *Manager {
	return &Manager{
		configPath: configPath,
	}
}

// LoadConfig 加载配置
func (m *Manager) LoadConfig() error {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	if err := yaml.Unmarshal(data, &m.config); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	log.Printf("已加载配置: 禁用进程 %d 个, 关键词 %d 个",
		len(m.config.BannedProcesses), len(m.config.Keywords))
	return nil
}

// GetConfig 获取配置
func (m *Manager) GetConfig() types.Config {
	return m.config
}

// LoadScannerConfig 加载扫描器配置
func LoadScannerConfig() types.ScannerConfig {
	return types.ScannerConfig{
		ComplianceURL: utils.GetEnvOrDefault(
			"COMPLIANCE_URL",
			"http://compliance-service:8080/alert",
		),
		NodeName:      utils.GetEnvOrDefault("NODE_NAME", "unknown"),
		ScanInterval:  utils.GetEnvIntOrDefault("SCAN_INTERVAL", 30),
		ProcPath:      utils.GetEnvOrDefault("PROC_PATH", "/host/proc"),
		ConfigMapPath: utils.GetEnvOrDefault("CONFIG_MAP_PATH", "config.yml"),
	}
}
