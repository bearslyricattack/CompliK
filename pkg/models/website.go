package models

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type IngressAnalysisResult struct {
	URL         string   `json:"url"`
	IsIllegal   bool     `json:"is_illegal"`
	Description string   `json:"description,omitempty"`
	Keywords    []string `json:"keywords,omitempty"`
	Namespace   string   `json:"namespace,omitempty"`
	Html        string   `json:"html,omitempty"`
}

func (ar *IngressAnalysisResult) SaveToFile(dirPath string) error {
	if ar == nil {
		return fmt.Errorf("models.IngressAnalysisResult 为空")
	}
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %v", err)
	}
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("analysis_%s.json", timestamp)
	filePath := filepath.Join(dirPath, filename)
	data, err := json.MarshalIndent(ar, "", "  ")
	if err != nil {
		return fmt.Errorf("JSON序列化失败: %v", err)
	}
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("写入文件失败: %v", err)
	}
	return nil
}

type CollectorResult struct {
	URL        string `json:"url"`
	HTML       string `json:"html"`
	IsEmpty    bool   `json:"is_empty"`
	Screenshot []byte `json:"screenshot"`
	Namespace  string `json:"namespace"`
}
