package alert

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/bearslyricattack/CompliK/daemon/internal/types"
)

// Sender 告警发送器
type Sender struct {
	complianceURL string
	httpClient    *http.Client
	nodeName      string
}

// NewSender 创建告警发送器
func NewSender(complianceURL, nodeName string) *Sender {
	return &Sender{
		complianceURL: complianceURL,
		nodeName:      nodeName,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SendAlert 发送告警
func (s *Sender) SendAlert(processInfo types.ProcessInfo) error {
	alert := types.ComplianceAlert{
		AlertType: "malicious_process_detected",
		Message: fmt.Sprintf("在节点 %s 上检测到恶意进程: %s (PID: %d)",
			s.nodeName, processInfo.ProcessName, processInfo.PID),
		Process: processInfo,
	}

	jsonData, err := json.Marshal(alert)
	if err != nil {
		return fmt.Errorf("序列化告警数据失败: %w", err)
	}

	resp, err := s.httpClient.Post(s.complianceURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("发送告警失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("告警发送失败，状态码: %d, 响应: %s", resp.StatusCode, string(body))
	}

	log.Printf("告警发送成功: %s", alert.Message)
	return nil
}

// SendBatchAlerts 批量发送告警
func (s *Sender) SendBatchAlerts(processInfos []types.ProcessInfo) error {
	for _, processInfo := range processInfos {
		if err := s.SendAlert(processInfo); err != nil {
			log.Printf("发送告警失败: %v", err)
			continue
		}
	}
	return nil
}
