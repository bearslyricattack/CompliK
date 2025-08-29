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
	"github.com/bearslyricattack/CompliK/pkg/logger"
	"github.com/bearslyricattack/CompliK/pkg/models"
	"github.com/bearslyricattack/CompliK/pkg/plugin"
	"github.com/bearslyricattack/CompliK/pkg/utils/config"
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
			log: logger.GetLogger().WithField("plugin", pluginName),
		}
	}
}

type HigressPlugin struct {
	log    logger.Logger
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
	p.log.Debug("Starting Higress plugin", logger.Fields{
		"plugin":     pluginName,
		"maxWorkers": maxWorkers,
	})

	// 解析配置
	if err := p.parseConfig(config); err != nil {
		p.log.Error("Failed to parse plugin config", logger.Fields{
			"error": err.Error(),
		})
		return fmt.Errorf("解析配置失败: %v", err)
	}

	p.log.Debug("Plugin config parsed successfully", logger.Fields{
		"timeRange": p.config.TimeRange,
		"app":       p.config.App,
		"hasAuth":   p.config.Username != "",
	})

	// 初始化 HTTP 客户端
	p.client = &http.Client{
		Timeout: 30 * time.Second,
	}

	subscribe := eventBus.Subscribe(constants.DiscoveryTopic)
	semaphore := make(chan struct{}, maxWorkers)

	p.log.Info("Higress plugin started successfully", logger.Fields{
		"timeout": "30s",
	})

	for {
		select {
		case event, ok := <-subscribe:
			if !ok {
				p.log.Info("Event subscription channel closed")
				return nil
			}
			semaphore <- struct{}{}
			go func(e eventbus.Event) {
				defer func() { <-semaphore }()
				defer func() {
					if r := recover(); r != nil {
						p.log.Error("Goroutine panic recovered", logger.Fields{
							"error": fmt.Sprintf("%v", r),
						})
					}
				}()

				ingress, ok := e.Payload.(models.DiscoveryInfo)
				if !ok {
					p.log.Error("Invalid event payload type", logger.Fields{
						"expected": "models.DiscoveryInfo",
						"actual":   fmt.Sprintf("%T", e.Payload),
					})
					return
				}

				p.log.Debug("Processing discovery event", logger.Fields{
					"host":      ingress.Host,
					"namespace": ingress.Namespace,
					"name":      ingress.Name,
				})

				// 查询日志
				result, err := p.queryLogs(ctx, ingress)
				if err != nil {
					p.log.Error("Failed to query Higress logs", logger.Fields{
						"host":      ingress.Host,
						"namespace": ingress.Namespace,
						"name":      ingress.Name,
						"error":     err.Error(),
					})
				} else {
					// 发布查询结果到收集器主题
					eventBus.Publish(constants.CollectorTopic, eventbus.Event{
						Payload: result,
					})
					p.log.Info("Successfully queried Higress logs", logger.Fields{
						"host":      ingress.Host,
						"namespace": ingress.Namespace,
						"logCount":  len(result),
					})
				}
			}(event)
		case <-ctx.Done():
			p.log.Info("Context cancelled, stopping Higress plugin")
			// 等待所有工作协程完成
			for i := 0; i < maxWorkers; i++ {
				semaphore <- struct{}{}
			}
			p.log.Info("Higress plugin stopped successfully")
			return nil
		}
	}
}

func (p *HigressPlugin) Stop(ctx context.Context) error {
	p.log.Info("Stopping Higress plugin")
	if p.client != nil {
		p.client.CloseIdleConnections()
		p.log.Debug("HTTP client idle connections closed")
	}
	p.log.Info("Higress plugin stopped")
	return nil
}

func (p *HigressPlugin) parseConfig(config config.PluginConfig) error {
	p.log.Debug("Parsing plugin configuration")

	configData, err := json.Marshal(config)
	if err != nil {
		p.log.Error("Failed to marshal config", logger.Fields{
			"error": err.Error(),
		})
		return err
	}

	err = json.Unmarshal(configData, &p.config)
	if err != nil {
		p.log.Error("Failed to unmarshal config to HigressConfig", logger.Fields{
			"error": err.Error(),
		})
		return err
	}

	// 设置默认值
	if p.config.TimeRange == "" {
		p.config.TimeRange = "5m"
		p.log.Debug("Set default time range", logger.Fields{"timeRange": "5m"})
	}
	if p.config.App == "" {
		p.config.App = "higress"
		p.log.Debug("Set default app name", logger.Fields{"app": "higress"})
	}

	p.log.Debug("Configuration parsed successfully", logger.Fields{
		"logServerPath":  p.config.LogServerPath,
		"timeRange":      p.config.TimeRange,
		"app":            p.config.App,
		"hasCredentials": p.config.Username != "",
	})

	return nil
}

func (p *HigressPlugin) queryLogs(ctx context.Context, ingress models.DiscoveryInfo) ([]LogEntry, error) {
	p.log.Debug("Starting log query", logger.Fields{
		"host":      ingress.Host,
		"namespace": ingress.Namespace,
	})

	// 构建查询参数
	query := p.buildQuery(ingress)
	p.log.Debug("Built log query", logger.Fields{
		"query": query,
	})

	// 发送请求
	resp, err := p.sendLogQuery(query)
	if err != nil {
		p.log.Error("Failed to send log query request", logger.Fields{
			"query": query,
			"error": err.Error(),
		})
		return nil, fmt.Errorf("发送日志查询请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 解析响应
	logs, err := p.parseLogResponse(resp.Body)
	if err != nil {
		p.log.Error("Failed to parse log response", logger.Fields{
			"error": err.Error(),
		})
		return nil, fmt.Errorf("解析日志响应失败: %v", err)
	}

	p.log.Debug("Log query completed successfully", logger.Fields{
		"logCount": len(logs),
	})

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
	p.log.Debug("Sending HTTP request for log query")

	// 使用类似 VLogs 的请求方式
	req, err := p.generateRequest(query)
	if err != nil {
		p.log.Error("Failed to generate HTTP request", logger.Fields{
			"error": err.Error(),
		})
		return nil, err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		p.log.Error("HTTP request failed", logger.Fields{
			"url":   req.URL.String(),
			"error": err.Error(),
		})
		return nil, fmt.Errorf("HTTP 请求错误: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		p.log.Error("HTTP request returned error status", logger.Fields{
			"statusCode": resp.StatusCode,
			"status":     resp.Status,
		})
		resp.Body.Close()
		return nil, fmt.Errorf("响应错误，状态码: %d", resp.StatusCode)
	}

	p.log.Debug("HTTP request successful", logger.Fields{
		"statusCode": resp.StatusCode,
	})

	return resp, nil
}

func (p *HigressPlugin) generateRequest(query string) (*http.Request, error) {
	// 构建请求 URL
	baseURL := fmt.Sprintf("%s/select/logsql/query?query=%s",
		p.config.LogServerPath,
		strings.ReplaceAll(query, " ", "%20"))

	p.log.Debug("Generating HTTP request", logger.Fields{
		"url": baseURL,
	})

	req, err := http.NewRequest("GET", baseURL, nil)
	if err != nil {
		p.log.Error("Failed to create HTTP request", logger.Fields{
			"url":   baseURL,
			"error": err.Error(),
		})
		return nil, fmt.Errorf("创建 HTTP 请求错误: %v", err)
	}

	// 设置基础认证
	req.SetBasicAuth(p.config.Username, p.config.Password)
	p.log.Debug("Set basic authentication", logger.Fields{
		"username": p.config.Username,
	})

	return req, nil
}

func (p *HigressPlugin) parseLogResponse(body io.Reader) ([]LogEntry, error) {
	p.log.Debug("Parsing log response body")

	bodyBytes, err := io.ReadAll(body)
	if err != nil {
		p.log.Error("Failed to read response body", logger.Fields{
			"error": err.Error(),
		})
		return nil, fmt.Errorf("读取响应体失败: %v", err)
	}

	p.log.Debug("Read response body", logger.Fields{
		"bodySize": len(bodyBytes),
	})

	if len(bodyBytes) == 0 {
		p.log.Debug("Empty response body")
		return []LogEntry{}, nil
	}

	var logs []LogEntry
	lines := strings.Split(string(bodyBytes), "\n")
	validLines := 0
	parseErrors := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var entry LogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			parseErrors++
			p.log.Debug("Failed to parse log line as JSON, using raw text", logger.Fields{
				"line":  line,
				"error": err.Error(),
			})
			// 如果 JSON 解析失败，创建一个简单的日志条目
			entry = LogEntry{
				Timestamp: time.Now().Format(time.RFC3339),
				Message:   line,
			}
		} else {
			validLines++
		}

		logs = append(logs, entry)
	}

	p.log.Debug("Log response parsing completed", logger.Fields{
		"totalLines":  len(lines),
		"validLines":  validLines,
		"parseErrors": parseErrors,
		"logEntries":  len(logs),
	})

	return logs, nil
}
