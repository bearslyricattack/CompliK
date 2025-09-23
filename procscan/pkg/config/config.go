package config

import (
	"fmt"
	"github.com/bearslyricattack/CompliK/procscan/pkg/models"
	"os"

	"gopkg.in/yaml.v3"
)

func LoadConfig(configPath string) (*models.Config, error) {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("配置文件不存在: %s", configPath)
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("配置文件为空: %s", configPath)
	}
	var config models.Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}
	return &config, nil
}
