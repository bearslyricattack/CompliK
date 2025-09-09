package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type DetectorInfo struct {
	DiscoveryName string `json:"discovery_name"`
	CollectorName string `json:"collector_name"`
	DetectorName  string `json:"detector_name"`

	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Region    string `json:"region"`

	Host string   `json:"host"`
	Path []string `json:"path"`
	URL  string   `json:"url"`

	Description string   `json:"description,omitempty"`
	Keywords    []string `json:"keywords,omitempty"`

	IsIllegal   bool   `json:"is_illegal"`
	Explanation string `json:"explanation,omitempty"`
}

func (d *DetectorInfo) SaveToFile(dirPath string) error {
	if d == nil {
		return errors.New("models.IngressAnalysisResult 为空")
	}
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("analysis_%s.json", timestamp)
	filePath := filepath.Join(dirPath, filename)
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return fmt.Errorf("JSON序列化失败: %w", err)
	}
	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}
	return nil
}
