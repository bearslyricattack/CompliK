package higress

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/bearslyricattack/CompliK/pkg/constants"
	"github.com/bearslyricattack/CompliK/pkg/eventbus"
	"github.com/bearslyricattack/CompliK/pkg/models"
	"github.com/bearslyricattack/CompliK/pkg/plugin"
	"github.com/bearslyricattack/CompliK/pkg/utils/config"
	"github.com/bearslyricattack/CompliK/pkg/utils/logger"
	"log"
)

const (
	pluginName = constants.ComplianceCollectorHigressName
	pluginType = constants.ComplianceHigressPluginType
)

const (
	maxWorkers = 20
)

func init() {
	plugin.PluginFactories[pluginName] = func() plugin.Plugin {
		return &HigressPlugin{
			logger: logger.NewLogger(),
		}
	}
}

type HigressPlugin struct {
	logger *logger.Logger
	config HigressConfig
	client *http.Client
}

type HigressConfig struct {
	LogServerPath string `json:"log_server_path"`
	Username      string `json:"username"`
	Password      string `json:"password"`
	TimeRange     string `json:"time_range"` // 默认时间范围，如 "5m"
	App           string `json:"app"`        // 应用名称
}

type LogEntry struct {
	Timestamp string `json:"_time"`
	Message   string `json:"_msg"`
	Pod       string `json:"pod"`
	Container string `json:"container"`
	Path      string `json:"path"`
}

func (p *HigressPlugin) Name() string {
	return pluginName
}

func (p *HigressPlugin) Type() string {
	return pluginType
}

func (p *HigressPlugin) Start(ctx context.Context, config config.PluginConfig, eventBus *eventbus.EventBus) error {
	// 解析配置
	if err := p.parseConfig(config); err != nil {
		return fmt.Errorf("解析配置失败: %v", err)
	}

	// 初始化 HTTP 客户端
	p.client = &http.Client{
		Timeout: 30 * time.Second,
	}

	subscribe := eventBus.Subscribe(constants.DiscoveryTopic)
	semaphore := make(chan struct{}, maxWorkers)

	p.logger.Info("Higress 插件启动成功")

	for {
		select {
		case event, ok := <-subscribe:
			if !ok {
				log.Println("事件订阅通道已关闭")
				return nil
			}
			semaphore <- struct{}{}
			go func(e eventbus.Event) {
				defer func() { <-semaphore }()
				defer func() {
					if r := recover(); r != nil {
						log.Printf("goroutine panic: %v", r)
					}
				}()

				ingress, ok := e.Payload.(models.DiscoveryInfo)
				if !ok {
					p.logger.Error(fmt.Sprintf("事件负载类型错误，期望models.DiscoveryInfo，实际: %T", e.Payload))
					return
				}

				// 查询日志
				result, err := p.queryLogs(ctx, ingress)
				if err != nil {
					p.logger.Error(fmt.Sprintf("本次读取 Higress 日志错误：ingress：%s，错误：%v", ingress.Host, err))
				} else {
					// 发布查询结果到收集器主题
					eventBus.Publish(constants.CollectorTopic, eventbus.Event{
						Payload: result,
					})
					p.logger.Info(fmt.Sprintf("成功查询 Higress 日志：ingress：%s，获取到 %d 条日志", ingress.Host, len(result)))
				}
			}(event)
		case <-ctx.Done():
			// 等待所有工作协程完成
			for i := 0; i < maxWorkers; i++ {
				semaphore <- struct{}{}
			}
			p.logger.Info("Higress 插件停止")
			return nil
		}
	}
}

func (p *HigressPlugin) Stop(ctx context.Context) error {
	p.logger.Info("正在停止 Higress 插件")
	if p.client != nil {
		p.client.CloseIdleConnections()
	}
	return nil
}

func (p *HigressPlugin) parseConfig(config config.PluginConfig) error {
	configData, err := json.Marshal(config)
	if err != nil {
		return err
	}

	err = json.Unmarshal(configData, &p.config)
	if err != nil {
		return err
	}

	// 设置默认值
	if p.config.TimeRange == "" {
		p.config.TimeRange = "5m"
	}
	if p.config.App == "" {
		p.config.App = "higress"
	}

	return nil
}

func (p *HigressPlugin) queryLogs(ctx context.Context, ingress models.DiscoveryInfo) ([]LogEntry, error) {
	// 构建查询参数
	query := p.buildQuery(ingress)

	// 发送请求
	resp, err := p.sendLogQuery(query)
	if err != nil {
		return nil, fmt.Errorf("发送日志查询请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 解析响应
	logs, err := p.parseLogResponse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("解析日志响应失败: %v", err)
	}

	return logs, nil
}

func (p *HigressPlugin) buildQuery(ingress models.DiscoveryInfo) string {
	var builder strings.Builder

	// 基础查询：根据 namespace 和关键词过滤
	builder.WriteString(fmt.Sprintf(`{namespace="%s"} `, ingress.Namespace))

	// 添加路径关键词搜索
	if ingress.Host != "" {
		builder.WriteString(fmt.Sprintf(`"%s" `, ingress.Host))
	}

	// 添加时间范围
	builder.WriteString(fmt.Sprintf(`_time:%s `, p.config.TimeRange))

	// 添加应用过滤
	builder.WriteString(fmt.Sprintf(`app:="%s" `, p.config.App))

	// 添加 JSON 解析和字段提取
	builder.WriteString(`| unpack_json `)

	// 删除不需要的字段
	builder.WriteString(`| Drop _stream_id,_stream,job,node `)

	// 限制返回数量
	builder.WriteString(`| limit 1000`)

	return builder.String()
}

func (p *HigressPlugin) sendLogQuery(query string) (*http.Response, error) {
	// 使用类似 VLogs 的请求方式
	req, err := p.generateRequest(query)
	if err != nil {
		return nil, err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP 请求错误: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("响应错误，状态码: %d", resp.StatusCode)
	}

	return resp, nil
}

func (p *HigressPlugin) generateRequest(query string) (*http.Request, error) {
	// 构建请求 URL
	baseURL := fmt.Sprintf("%s/select/logsql/query?query=%s",
		p.config.LogServerPath,
		strings.ReplaceAll(query, " ", "%20"))

	req, err := http.NewRequest("GET", baseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建 HTTP 请求错误: %v", err)
	}

	// 设置基础认证
	req.SetBasicAuth(p.config.Username, p.config.Password)

	return req, nil
}

func (p *HigressPlugin) parseLogResponse(body io.Reader) ([]LogEntry, error) {
	bodyBytes, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("读取响应体失败: %v", err)
	}

	if len(bodyBytes) == 0 {
		return []LogEntry{}, nil
	}

	var logs []LogEntry
	lines := strings.Split(string(bodyBytes), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var entry LogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			// 如果 JSON 解析失败，创建一个简单的日志条目
			entry = LogEntry{
				Timestamp: time.Now().Format(time.RFC3339),
				Message:   line,
			}
		}

		logs = append(logs, entry)
	}

	return logs, nil
}
