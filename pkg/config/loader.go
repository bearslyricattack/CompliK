package config

import (
	"errors"
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
)

func LoadConfig(configPath string) (*Config, error) {
	cfg := &Config{}
	if configPath == "" {
		return nil, errors.New("config path is required")
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}
	return cfg, nil
}
