package lark

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/bearslyricattack/CompliK/pkg/constants"
	"github.com/bearslyricattack/CompliK/pkg/eventbus"
	"github.com/bearslyricattack/CompliK/pkg/models"
	"github.com/bearslyricattack/CompliK/pkg/plugin"
	"github.com/bearslyricattack/CompliK/pkg/utils/config"
	"github.com/bearslyricattack/CompliK/pkg/utils/logger"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	pluginName = "Lark"
	pluginType = "Handle"
)

func init() {
	plugin.PluginFactories[pluginName] = func() plugin.Plugin {
		return &ResultPlugin{
			logger: logger.NewLogger(),
		}
	}
}

type ResultPlugin struct {
	logger *logger.Logger
}

func (p *ResultPlugin) Name() string {
	return pluginName
}

func (p *ResultPlugin) Type() string {
	return pluginType
}

func (p *ResultPlugin) Start(ctx context.Context, config config.PluginConfig, eventBus *eventbus.EventBus) error {
	subscribe := eventBus.Subscribe(constants.HandleDatabaseTopic)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("WebsitePlugin goroutine panic: %v", r)
			}
		}()
		for {
			select {
			case event, ok := <-subscribe:
				if !ok {
					log.Println("事件订阅通道已关闭")
					return
				}
				result, ok := event.Payload.([]models.IngressAnalysisResult)
				if !ok {
					log.Printf("事件负载类型错误，期望 []models.IngressInfo，实际: %T", event.Payload)
					continue
				}
				// 发送通知
				notifier := NewFeishuNotifier("https://open.feishu.cn/open-apis/bot/v2/hook/57e00497-a1da-41cd-9342-2e645f95e6ec")
				err := notifier.SendAnalysisNotification(result)
				if err != nil {
					log.Printf("发送失败: %v", err)
				}
			case <-ctx.Done():
				log.Println("WebsitePlugin 收到停止信号")
				return
			}
		}
	}()
	return nil
}

// Stop 停止插件
func (p *ResultPlugin) Stop(ctx context.Context) error {
	return nil
}

type ComplianceResult struct {
	IsIllegal   string `json:"is_illegal"`
	Explanation string `json:"explanation"`
}

type UnpassedDetail struct {
	Hostname    string   `json:"hostname"`
	Namespace   string   `json:"namespace"`
	Description string   `json:"description"`
	Reason      string   `json:"reason"`
	Keywords    []string `json:"keywords,omitempty"`
}

type AnalysisSummary struct {
	Cluster         string           `json:"cluster"`
	Date            string           `json:"date"`
	Timestamp       string           `json:"timestamp"`
	TotalSites      int              `json:"total_sites"`
	PassedSites     int              `json:"passed_sites"`
	UnpassedSites   int              `json:"unpassed_sites"`
	UnpassedDetails []UnpassedDetail `json:"unpassed_details"`
}

// 飞书消息结构
type FeishuMessage struct {
	MsgType string `json:"msg_type"`
	Content struct {
		Text string `json:"text"`
	} `json:"content"`
}

// 飞书API响应结构
type FeishuResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

// FeishuNotifier 飞书通知器
type FeishuNotifier struct {
	WebhookURL string
	HTTPClient *http.Client
}

// NewFeishuNotifier 创建飞书通知器实例
func NewFeishuNotifier(webhookURL string) *FeishuNotifier {
	if webhookURL == "" {
		webhookURL = os.Getenv("WEBHOOK_URL")
	}

	return &FeishuNotifier{
		WebhookURL: webhookURL,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// BuildNotificationMessage 构建通知消息内容
func (f *FeishuNotifier) BuildNotificationMessage(results []models.IngressAnalysisResult) string {
	var messageBuilder strings.Builder

	// 统计数据
	totalSites := len(results)
	passedSites := 0
	unpassedSites := 0
	var unpassedResults []models.IngressAnalysisResult

	// 分类统计
	for _, result := range results {
		if result.IsIllegal {
			unpassedSites++
			unpassedResults = append(unpassedResults, result)
		} else {
			passedSites++
		}
	}

	// 计算合规率
	complianceRate := "0%"
	if totalSites > 0 {
		rate := float64(passedSites) / float64(totalSites) * 100
		complianceRate = fmt.Sprintf("%.2f%%", rate)
	}

	// 基础统计信息
	messageBuilder.WriteString("站点合规分析报告\n\n")
	messageBuilder.WriteString(fmt.Sprintf("站点总数: %d\n", totalSites))
	messageBuilder.WriteString(fmt.Sprintf("合规站点: %d\n", passedSites))
	messageBuilder.WriteString(fmt.Sprintf("不合规站点: %d\n", unpassedSites))
	messageBuilder.WriteString(fmt.Sprintf("合规率: %s\n\n", complianceRate))

	// 收集需要封禁的命名空间列表（去重）
	namespaceSet := make(map[string]bool)
	var namespaceList []string

	for _, result := range unpassedResults {
		if result.Namespace != "" && !namespaceSet[result.Namespace] {
			namespaceSet[result.Namespace] = true
			namespaceList = append(namespaceList, result.Namespace)
		}
	}

	// 添加命名空间封禁列表
	if len(namespaceList) > 0 {
		messageBuilder.WriteString("===== 需封禁的命名空间列表 =====\n")
		messageBuilder.WriteString("```\n")
		messageBuilder.WriteString(strings.Join(namespaceList, "\n"))
		messageBuilder.WriteString("\n```\n\n")
	}

	// 添加不合规站点详情
	if len(unpassedResults) > 0 {
		messageBuilder.WriteString("不合规站点详情:\n")

		// 限制显示数量，避免消息过长
		maxDisplay := 10
		displayCount := len(unpassedResults)
		if displayCount > maxDisplay {
			displayCount = maxDisplay
		}

		for i, result := range unpassedResults[:displayCount] {
			// 从URL中提取主机名
			hostname := extractHostnameFromURL(result.URL)

			messageBuilder.WriteString(fmt.Sprintf("%d. %s (命名空间: %s)\n",
				i+1, hostname, result.Namespace))

			// 添加描述信息
			if result.Description != "" {
				messageBuilder.WriteString(fmt.Sprintf("   描述: %s\n", result.Description))
			}

			// 添加关键词
			if len(result.Keywords) > 0 {
				messageBuilder.WriteString(fmt.Sprintf("   关键词: %s\n",
					strings.Join(result.Keywords, ", ")))
			}

			messageBuilder.WriteString("\n")
		}

		if len(unpassedResults) > maxDisplay {
			messageBuilder.WriteString(fmt.Sprintf("... 等共 %d 个不合规站点\n\n",
				len(unpassedResults)))
		}

		// 添加kubectl命令示例
		if len(namespaceList) > 0 {
			messageBuilder.WriteString("===== 命令行操作示例 =====\n")

			// 单个命名空间封禁示例
			exampleNs := namespaceList[0]
			messageBuilder.WriteString("封禁单个命名空间示例:\n")
			messageBuilder.WriteString(fmt.Sprintf("```\nkubectl annotate --overwrite ns %s debt.sealos/status=Suspend\n```\n\n", exampleNs))

			// 批量封禁命令
			messageBuilder.WriteString("批量封禁命名空间:\n```\n")

			// 限制示例中显示的命名空间数量
			exampleCount := len(namespaceList)
			if exampleCount > 3 {
				exampleCount = 3
			}

			messageBuilder.WriteString(fmt.Sprintf("for namespace in %s; do kubectl annotate --overwrite ns $namespace debt.sealos/status=Suspend; done\n",
				strings.Join(namespaceList[:exampleCount], " ")))
			messageBuilder.WriteString("```\n")

			if len(namespaceList) > 3 {
				messageBuilder.WriteString("(示例中仅展示部分命名空间，请使用上方完整列表)\n")
			}
		}
	} else {
		messageBuilder.WriteString("🎉 所有站点均通过合规检查！\n")
	}

	return messageBuilder.String()
}

// extractHostnameFromURL 从URL中提取主机名
func extractHostnameFromURL(url string) string {
	// 移除协议前缀
	if strings.HasPrefix(url, "https://") {
		url = strings.TrimPrefix(url, "https://")
	} else if strings.HasPrefix(url, "http://") {
		url = strings.TrimPrefix(url, "http://")
	}

	// 移除路径部分
	if idx := strings.Index(url, "/"); idx != -1 {
		url = url[:idx]
	}

	// 移除端口号
	if idx := strings.Index(url, ":"); idx != -1 {
		url = url[:idx]
	}

	return url
}

// SendAnalysisNotification 发送分析结果通知（简化版）
func (f *FeishuNotifier) SendAnalysisNotification(results []models.IngressAnalysisResult) error {
	if f.WebhookURL == "" {
		return fmt.Errorf("未设置webhook URL，跳过通知发送")
	}

	// 构建消息内容
	messageText := f.BuildNotificationMessage(results)

	// 构建飞书消息格式
	message := FeishuMessage{
		MsgType: "text",
	}
	message.Content.Text = messageText

	// 发送消息
	return f.sendMessage(message)
}

// sendMessage 发送消息的内部方法
func (f *FeishuNotifier) sendMessage(message FeishuMessage) error {
	jsonData, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("序列化消息失败: %w", err)
	}

	// 发送HTTP请求
	resp, err := f.HTTPClient.Post(
		f.WebhookURL,
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return fmt.Errorf("发送HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应失败: %w", err)
	}

	// 解析飞书API响应
	var feishuResp FeishuResponse
	if err := json.Unmarshal(body, &feishuResp); err != nil {
		return fmt.Errorf("解析响应失败: %w", err)
	}

	// 检查响应状态
	if resp.StatusCode != 200 || feishuResp.Code != 0 {
		return fmt.Errorf("飞书webhook通知发送失败: HTTP状态码 %d, 飞书错误码 %d, 错误信息: %s",
			resp.StatusCode, feishuResp.Code, feishuResp.Msg)
	}

	log.Printf("飞书webhook通知发送成功")
	return nil
}
