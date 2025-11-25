package lark

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Notifier represents a Lark (Feishu) notification client
type Notifier struct {
	webhook string
}

// NewNotifier creates a new Lark notifier with the specified webhook URL
func NewNotifier(webhook string) *Notifier {
	return &Notifier{
		webhook: webhook,
	}
}

// Send sends a standard notification message to Lark
func (n *Notifier) Send(message string) error {
	if !n.IsEnabled() {
		return nil
	}

	card := n.buildDetailedCard(message)
	return n.sendCard(card)
}

// SendThreatAlert sends a security threat alert with specialized formatting
func (n *Notifier) SendThreatAlert(threatInfo ThreatInfo) error {
	if !n.IsEnabled() {
		return nil
	}

	card := n.buildThreatAlertCard(threatInfo)
	return n.sendCard(card)
}

// IsEnabled checks if the notifier is properly configured
func (n *Notifier) IsEnabled() bool {
	return n.webhook != "" && strings.HasPrefix(n.webhook, "https://")
}

// buildDetailedCard constructs a detailed alert card for standard messages
func (n *Notifier) buildDetailedCard(message string) map[string]interface{} {
	title := "🛡️ ProcScan 安全告警"
	if len(message) > 50 {
		lines := strings.Split(message, "\n")
		for _, line := range lines {
			if strings.Contains(line, "进程") || strings.Contains(line, "PID") {
				trimmed := strings.TrimSpace(line)
				if len(trimmed) > 30 {
					title = "🛡️ " + trimmed[:30] + "..."
				} else {
					title = "🛡️ " + trimmed
				}
				break
			}
		}
	}

	// Build professional alert content
	var alertContent strings.Builder
	alertContent.WriteString("> 🟠 **SECURITY ALERT**\n\n")

	// Alert details
	alertContent.WriteString("## 📋 Alert Details\n\n")
	if strings.Contains(message, "\n") {
		// Multi-line message, preserve formatting
		alertContent.WriteString(message)
	} else {
		// Single-line message, highlight with quote
		alertContent.WriteString(fmt.Sprintf("> %s", message))
	}

	alertContent.WriteString("\n\n---\n\n")

	// System information
	alertContent.WriteString("## 🖥️ System Status\n\n")
	alertContent.WriteString("| 🔑 Property | 📊 Status |\n")
	alertContent.WriteString("|:--------|:-----|\n")
	alertContent.WriteString(fmt.Sprintf("| **⏰ Detection Time** | `%s` |\n", time.Now().Format("2006-01-02 15:04:05")))
	alertContent.WriteString("| **🖥️ Scan Node** | Kubernetes DaemonSet |\n")
	alertContent.WriteString("| **🛡️ Protection Status** | <font color='green'>✅ Auto Handled</font> |\n")
	alertContent.WriteString("| **🔍 Alert Source** | ProcScan Security Scan |\n")

	return map[string]interface{}{
		"config": map[string]interface{}{
			"wide_screen_mode": true,
		},
		"header": map[string]interface{}{
			"template": "orange",
			"title": map[string]interface{}{
				"content": title,
				"tag":     "plain_text",
			},
			"subtitle": map[string]interface{}{
				"content": fmt.Sprintf("⚠️ Medium Alert | %s", time.Now().Format("2006-01-02 15:04:05")),
				"tag":     "plain_text",
			},
		},
		"elements": []map[string]interface{}{
			{
				"tag": "div",
				"text": map[string]interface{}{
					"content": alertContent.String(),
					"tag":     "lark_md",
				},
			},
			{
				"tag": "action",
				"actions": []map[string]interface{}{
					{
						"tag": "button",
						"text": map[string]interface{}{
							"content": "🔍 View Details",
							"tag":     "plain_text",
						},
						"type": "primary",
						"url":  "https://k8s.console.com",
					},
					{
						"tag": "button",
						"text": map[string]interface{}{
							"content": "✅ Acknowledge",
							"tag":     "plain_text",
						},
						"type": "default",
						"url":  "#",
					},
				},
			},
		},
	}
}

// buildSimpleCard constructs a simple alert card (kept for backward compatibility)
func (n *Notifier) buildSimpleCard(message string) map[string]interface{} {
	return map[string]interface{}{
		"config": map[string]interface{}{"wide_screen_mode": true},
		"header": map[string]interface{}{
			"template": "red",
			"title":    map[string]interface{}{"content": "🚨 ProcScan Alert", "tag": "plain_text"},
		},
		"elements": []map[string]interface{}{
			{
				"tag": "div",
				"text": map[string]interface{}{
					"content": message,
					"tag":     "lark_md",
				},
			},
		},
	}
}

// sendCard sends a card message to Lark webhook
func (n *Notifier) sendCard(card map[string]interface{}) error {
	message := LarkMessage{
		MsgType: "interactive",
		Card:    card,
	}

	jsonData, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("marshal message failed: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Post(n.webhook, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("send request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("notification failed: HTTP %d", resp.StatusCode)
	}

	return nil
}

// LarkMessage represents the Lark message structure
type LarkMessage struct {
	MsgType string         `json:"msg_type"`
	Card    map[string]any `json:"card"`
}

// ThreatInfo represents threat information structure
type ThreatInfo struct {
	TotalAffected int            `json:"total_affected"`
	ScanTime      string         `json:"scan_time"`
	NodeName      string         `json:"node_name"`
	Threats       []ThreatDetail `json:"threats"`
	Actions       []string       `json:"actions"`
}

// ThreatDetail contains detailed threat information
type ThreatDetail struct {
	Namespace    string        `json:"namespace"`
	ProcessCount int           `json:"process_count"`
	Processes    []ProcessInfo `json:"processes"`
	ActionResult string        `json:"action_result"`
}

// ProcessInfo contains enhanced process information
type ProcessInfo struct {
	PID     int    `json:"pid"`
	Name    string `json:"name"`
	Command string `json:"command"`
	User    string `json:"user"`
	Status  string `json:"status"`
	// Kubernetes related information
	PodName        string `json:"pod_name"`
	PodNamespace   string `json:"pod_namespace"`
	PodUID         string `json:"pod_uid"`
	ContainerName  string `json:"container_name"`
	ContainerID    string `json:"container_id"`
	ContainerImage string `json:"container_image"`
	NodeName       string `json:"node_name"`
	PodIP          string `json:"pod_ip"`
	// Container runtime information
	Runtime        string `json:"runtime"`
	ContainerState string `json:"container_state"`
	// Security related information
	SecurityContext map[string]interface{} `json:"security_context"`
}

// buildThreatAlertCard constructs a threat alert card with specialized formatting
func (n *Notifier) buildThreatAlertCard(threat ThreatInfo) map[string]interface{} {
	// Determine styling based on threat severity
	severity := "High"
	severityColor := "red"
	severityEmoji := "🚨"
	severityIcon := "🔴"

	if threat.TotalAffected <= 3 {
		severity = "Medium"
		severityColor = "orange"
		severityEmoji = "⚠️"
		severityIcon = "🟠"
	} else if threat.TotalAffected <= 10 {
		severity = "High"
		severityColor = "red"
		severityEmoji = "🚨"
		severityIcon = "🔴"
	} else {
		severity = "Critical"
		severityColor = "red"
		severityEmoji = "🔥"
		severityIcon = "🆘"
	}

	// Build professional alert summary
	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("> %s **SECURITY THREAT ALERT**\n\n", severityIcon))
	summary.WriteString(fmt.Sprintf("**Severity Level**: <font color='%s'>%s %s</font>\n", severityColor, severityEmoji, severity))
	summary.WriteString(fmt.Sprintf("**Threat Type**: <font color='red'>🛡️ Suspicious Process Activity</font>\n"))
	summary.WriteString(fmt.Sprintf("**Detection Count**: <font color='red'><b>%d</b></font> processes\n", threat.TotalAffected))
	summary.WriteString(fmt.Sprintf("**Impact Scope**: <font color='orange'><b>%d</b></font> namespaces\n", len(threat.Threats)))
	summary.WriteString(fmt.Sprintf("**Scan Node**: 🖥️ `%s`\n", threat.NodeName))
	summary.WriteString(fmt.Sprintf("**Detection Time**: ⏰ %s\n\n", threat.ScanTime))

	// Threat statistics
	if len(threat.Threats) > 0 {
		summary.WriteString("## 📊 Threat Distribution Statistics\n")
		for i, detail := range threat.Threats {
			if i >= 4 { // Show maximum 4 namespaces
				summary.WriteString(fmt.Sprintf("• 📂 ... and <b>%d</b> more namespaces\n", len(threat.Threats)-4))
				break
			}
			summary.WriteString(fmt.Sprintf("• 📂 **`%s`**: <b>%d</b> processes\n", detail.Namespace, detail.ProcessCount))
		}
		summary.WriteString("\n")
	}

	// Build detailed threat information
	var details strings.Builder
	details.WriteString("## 🔍 Threat Analysis Details\n\n")

	for i, detail := range threat.Threats {
		if i >= 2 { // Show maximum 2 namespaces in detail
			details.WriteString(fmt.Sprintf("> ℹ️ ... and <b>%d</b> more namespaces with details\n", len(threat.Threats)-2))
			break
		}

		details.WriteString(fmt.Sprintf("### 📂 Namespace: `%s` (<b>%d processes</b>)\n\n", detail.Namespace, detail.ProcessCount))

		for j, proc := range detail.Processes {
			if j >= 3 { // Show maximum 3 processes per namespace
				details.WriteString(fmt.Sprintf("> ℹ️ ... and <b>%d</b> more processes\n\n", detail.ProcessCount-3))
				break
			}

			details.WriteString(fmt.Sprintf("#### 🔄 Threat Process <font color='red'>#%d</font>\n\n", j+1))

			// Display key information in table format
			details.WriteString("| 🔑 Property | 📋 Value |\n")
			details.WriteString("|:--------|:-----|\n")
			details.WriteString(fmt.Sprintf("| **Process ID** | <font color='red'><code>%d</code></font> |\n", proc.PID))
			details.WriteString(fmt.Sprintf("| **Process Name** | <font color='red'><b>%s</b></font> |\n", proc.Name))

			// Kubernetes related information
			if proc.PodName != "" {
				details.WriteString(fmt.Sprintf("| **🏷️ Pod Name** | <code>%s</code> |\n", proc.PodName))
			}
			if proc.PodNamespace != "" {
				details.WriteString(fmt.Sprintf("| **📦 Pod Namespace** | <code>%s</code> |\n", proc.PodNamespace))
			}
			if proc.ContainerName != "" {
				details.WriteString(fmt.Sprintf("| **🐳 Container Name** | <code>%s</code> |\n", proc.ContainerName))
			}
			if proc.PodIP != "" {
				details.WriteString(fmt.Sprintf("| **🌐 Pod IP** | <code>%s</code> |\n", proc.PodIP))
			}

			// Runtime information
			if proc.Runtime != "" {
				runtimeInfo := fmt.Sprintf("⚙️ %s", proc.Runtime)
				if proc.ContainerState != "" {
					runtimeInfo += fmt.Sprintf(" (🔵 %s)", proc.ContainerState)
				}
				details.WriteString(fmt.Sprintf("| **Runtime Environment** | %s |\n", runtimeInfo))
			}

			// Command line (limited length)
			if proc.Command != "" {
				command := proc.Command
				if len(command) > 70 {
					command = command[:67] + "..."
				}
				details.WriteString(fmt.Sprintf("| **💻 Command** | <code>%s</code> |\n", command))
			}

			// User information
			if proc.User != "" {
				userIcon := "👤"
				if proc.User == "root" {
					userIcon = "🔴"
				}
				details.WriteString(fmt.Sprintf("| **Running User** | %s <code>%s</code> |\n", userIcon, proc.User))
			}

			// Node information (if different from scan node)
			if proc.NodeName != "" && proc.NodeName != threat.NodeName {
				details.WriteString(fmt.Sprintf("| **🖥️ Running Node** | <code>%s</code> |\n", proc.NodeName))
			}

			// Container ID (shortened display)
			if proc.ContainerID != "" {
				containerID := proc.ContainerID
				if len(containerID) > 12 {
					containerID = "..." + containerID[len(containerID)-12:]
				}
				details.WriteString(fmt.Sprintf("| **🆔 Container ID** | <code>%s</code> |\n", containerID))
			}

			// Processing status
			status := "✅ Handled"
			if proc.Status != "" {
				status = proc.Status
			}
			details.WriteString(fmt.Sprintf("| **📊 Status** | <font color='green'>%s</font> |\n", status))
			details.WriteString("\n")
		}

		if i < len(threat.Threats)-1 && i < 1 {
			details.WriteString("---\n\n")
		}
	}

	// Response actions summary
	var actions strings.Builder
	actions.WriteString("## ⚙️ Security Response Actions\n\n")
	if len(threat.Actions) > 0 {
		for _, action := range threat.Actions {
			actions.WriteString(fmt.Sprintf("✅ %s\n", action))
		}
	} else {
		actions.WriteString("⏳ Processing...\n")
	}

	return map[string]interface{}{
		"config": map[string]interface{}{
			"wide_screen_mode": true,
		},
		"header": map[string]interface{}{
			"template": severityColor,
			"title": map[string]interface{}{
				"content": fmt.Sprintf("🛡️ ProcScan Security Alert (%d processes)", threat.TotalAffected),
				"tag":     "plain_text",
			},
			"subtitle": map[string]interface{}{
				"content": fmt.Sprintf("%s %s | %s", severityIcon, severity, threat.ScanTime),
				"tag":     "plain_text",
			},
		},
		"elements": []map[string]interface{}{
			{
				"tag": "div",
				"text": map[string]interface{}{
					"content": summary.String(),
					"tag":     "lark_md",
				},
			},
			{
				"tag": "hr",
			},
			{
				"tag": "div",
				"text": map[string]interface{}{
					"content": details.String(),
					"tag":     "lark_md",
				},
			},
			{
				"tag": "hr",
			},
			{
				"tag": "div",
				"text": map[string]interface{}{
					"content": actions.String() + "\n\n---\n\n> 💡 **Security Reminder**: The system has automatically handled detected suspicious processes. Please check relevant Pod status and logs to ensure threats are completely eliminated.",
					"tag":     "lark_md",
				},
			},
			{
				"tag": "action",
				"actions": []map[string]interface{}{
					{
						"tag": "button",
						"text": map[string]interface{}{
							"content": "🔍 View Pod Status",
							"tag":     "plain_text",
						},
						"type": "primary",
						"url":  "https://k8s.console.com/pods",
					},
					{
						"tag": "button",
						"text": map[string]interface{}{
							"content": "📋 View Logs",
							"tag":     "plain_text",
						},
						"type": "default",
						"url":  "https://k8s.console.com/logs",
					},
					{
						"tag": "button",
						"text": map[string]interface{}{
							"content": "⚙️ Management Console",
							"tag":     "plain_text",
						},
						"type": "default",
						"url":  "https://k8s.console.com",
					},
				},
			},
		},
	}
}
