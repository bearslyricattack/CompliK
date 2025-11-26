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

// Package alert provides functionality for sending security alerts and notifications
// to external systems such as Lark (Feishu) messaging platform.
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
func SendGlobalBatchAlert(results []*NamespaceScanResult, webhookURL string, region string) error {
	if webhookURL == "" {
		return fmt.Errorf("webhook URL cannot be empty")
	}
	if len(results) == 0 {
		return nil // No issues found, skip alert
	}

	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		nodeName = "Unknown Node"
	}

	// Statistics information
	totalProcesses := 0
	for _, r := range results {
		totalProcesses += len(r.ProcessInfos)
	}

	// Build card content
	allElements := []map[string]any{}

	// 1. Overview information - using prominent styling
	summaryText := fmt.Sprintf("**Availability Zone:** `%s`\n**Node:** `%s`\n**Anomalies Found:** %d suspicious processes\n**Affected Namespaces:** %d",
		region, nodeName, totalProcesses, len(results))
	allElements = append(allElements, newMarkdownElement(summaryText))

	// 2. Separator line
	allElements = append(allElements, newHrElement())

	// 3. Detailed information - grouped by namespace
	for idx, r := range results {
		if idx > 0 {
			allElements = append(allElements, newHrElement())
		}

		// Namespace title
		nsTitle := fmt.Sprintf("### Namespace: `%s` (%d anomalies)", r.Namespace, len(r.ProcessInfos))
		allElements = append(allElements, newMarkdownElement(nsTitle))

		// Processing status
		if r.LabelResult != "" {
			statusText := fmt.Sprintf("**Processing Status:** %s", getStatusText(r.LabelResult))
			allElements = append(allElements, newMarkdownElement(statusText))
		}

		// Suspicious process list - using table format
		if len(r.ProcessInfos) > 0 {
			tableHeader := "| Pod | Process | Reason |\n| --- | --- | --- |"
			allElements = append(allElements, newMarkdownElement(tableHeader))

			for _, p := range r.ProcessInfos {
				// Simplify Pod name (if too long)
				podName := p.PodName
				if len(podName) > 30 {
					podName = podName[:27] + "..."
				}

				// Extract key reason
				reason := extractReason(p.Message)

				tableRow := fmt.Sprintf("| `%s` | `%s` | %s |",
					podName,
					p.ProcessName,
					reason)
				allElements = append(allElements, newMarkdownElement(tableRow))
			}
		}
	}

	// 4. Bottom tip
	allElements = append(allElements, newHrElement())
	allElements = append(allElements, newMarkdownElement("**Suggestion:** Please check and handle anomalous processes promptly"))

	cardContent := map[string]any{
		"config": map[string]any{"wide_screen_mode": true},
		"header": map[string]any{
			"template": "red",
			"title": map[string]any{
				"content": "Suspicious Process Alert",
				"tag":     "plain_text",
			},
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

// newHrElement creates a horizontal line element
func newHrElement() map[string]any {
	return map[string]any{
		"tag": "hr",
	}
}

// getStatusText converts label result to user-friendly status text
func getStatusText(labelResult string) string {
	if strings.Contains(labelResult, "disabled") || strings.Contains(labelResult, "Feature disabled") {
		return "Feature Not Enabled"
	}
	if strings.Contains(labelResult, "success") || strings.Contains(labelResult, "Success") {
		return "Marked for Processing"
	}
	if strings.Contains(labelResult, "error") || strings.Contains(labelResult, "Error") {
		return "Processing Failed"
	}
	return "Pending Processing"
}

// extractReason extracts the key reason from alert message
func extractReason(message string) string {
	// Example: "Process name 'bash' matched blacklist rule '^bash$'"
	if strings.Contains(message, "matched blacklist") {
		return "Blacklisted Process"
	}
	if strings.Contains(message, "suspicious") {
		return "Suspicious Behavior"
	}
	if strings.Contains(message, "unauthorized") {
		return "Unauthorized Access"
	}
	// Default: return simplified message
	if len(message) > 20 {
		return message[:20] + "..."
	}
	return message
}
