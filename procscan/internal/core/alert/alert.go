package alert

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	legacy "github.com/bearslyricattack/CompliK/procscan/pkg/logger/legacy"
	"github.com/bearslyricattack/CompliK/procscan/pkg/models"
)

// LarkMessage defines the card message structure sent to Lark
type LarkMessage struct {
	MsgType string         `json:"msg_type"`
	Card    map[string]any `json:"card"`
}

// NamespaceScanResult encapsulates all scan findings and operation results for a namespace
type NamespaceScanResult struct {
	Namespace    string
	ProcessInfos []*models.ProcessInfo
	LabelResult  string
}

// SendGlobalBatchAlert constructs and sends aggregated alert using Markdown format
func SendGlobalBatchAlert(results []*NamespaceScanResult, webhookURL string) error {
	if webhookURL == "" {
		return fmt.Errorf("webhook URL cannot be empty")
	}
	if len(results) == 0 {
		return nil // No issues found, skip alert
	}

	// Build card content
	namespaceList := make([]string, 0, len(results))
	totalProcesses := 0
	for _, r := range results {
		namespaceList = append(namespaceList, fmt.Sprintf("`%s` (%d processes)", r.Namespace, len(r.ProcessInfos)))
		totalProcesses += len(r.ProcessInfos)
	}
	summaryText := fmt.Sprintf("This scan found **%d** suspicious processes in **%d** namespaces.\n**Affected namespaces:**\n%s",
		totalProcesses, len(results), strings.Join(namespaceList, "\n"))

	allElements := []map[string]any{
		newMarkdownElement(summaryText),
	}

	// Build detailed information for each namespace
	for _, r := range results {
		allElements = append(allElements, newMarkdownElement(fmt.Sprintf("---\n### **📦 Namespace: `%s`**", r.Namespace)))

		nodeName := os.Getenv("NODE_NAME")
		if nodeName == "" {
			nodeName = "unknown"
		}
		allElements = append(allElements, newMarkdownElement(fmt.Sprintf("Node Name:%s", nodeName)))

		var actionText strings.Builder
		if r.LabelResult != "" {
			actionText.WriteString(fmt.Sprintf("**Label Operation:** %s\n", r.LabelResult))
			actionText.WriteString("**Processing Status:** ⏳ Waiting for external controller")
		}
		if actionText.Len() > 0 {
			allElements = append(allElements, newMarkdownElement(actionText.String()))
		}

		// Add details for all suspicious processes in this namespace
		for i, p := range r.ProcessInfos {
			if i > 0 {
				allElements = append(allElements, newMarkdownElement("----------"))
			}
			allElements = append(allElements, newMarkdownElement(fmt.Sprintf("Suspicious Process #%d", i+1)))

			processDetails := []string{
				fmt.Sprintf("Pod Name:%s", p.PodName),
				fmt.Sprintf("Pod Namespace:%s", p.Namespace),
				fmt.Sprintf("Process Name:%s", p.ProcessName),
				fmt.Sprintf("Command:%s", p.Command),
				fmt.Sprintf("Alert Message:%s", p.Message),
				fmt.Sprintf("Detection Time:%s", p.Timestamp),
			}
			allElements = append(allElements, newMarkdownElement(strings.Join(processDetails, "\n")))
		}
	}

	allElements = append(allElements, newMarkdownElement("---"))
	allElements = append(allElements, newMarkdownElement("**❗ Please handle suspicious processes promptly!**"))

	cardContent := map[string]any{
		"config": map[string]any{"wide_screen_mode": true},
		"header": map[string]any{
			"template": "red",
			"title":    map[string]any{"content": "🚨 Node Suspicious Process Scan Report", "tag": "plain_text"},
		},
		"elements": allElements,
	}

	// Send request
	message := LarkMessage{
		MsgType: "interactive",
		Card:    cardContent,
	}
	jsonData, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to serialize message: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Post(webhookURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Lark notification failed: HTTP status code %d", resp.StatusCode)
	}

	legacy.L.Info("Global Lark alert sent successfully")
	return nil
}

// newMarkdownElement creates a standard Lark card Markdown element
func newMarkdownElement(content string) map[string]any {
	return map[string]any{
		"tag": "div",
		"text": map[string]any{
			"content": content,
			"tag":     "lark_md",
		},
	}
}
