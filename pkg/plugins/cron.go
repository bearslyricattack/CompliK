package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/bearslyricattack/CompliK/pkg/config"
	"github.com/bearslyricattack/CompliK/pkg/eventbus"
	"github.com/bearslyricattack/CompliK/pkg/manager"
)

func init() {
	manager.PluginFactories["cron"] = func() manager.Plugin {
		return &CronPlugin{}
	}
}

// CronPlugin 定时任务插件
type CronPlugin struct{}

// Name 获取插件名称
func (p *CronPlugin) Name() string {
	return "cron"
}

// Type 获取插件类型
func (p *CronPlugin) Type() string {
	return "scheduler"
}

type cron struct {
	Test string `json:"test"`
}

// Start 启动插件
func (p *CronPlugin) Start(ctx context.Context, config config.PluginConfig, eventBus *eventbus.EventBus) error {
	var cro cron
	fmt.Println("=== 开始解析配置 ===")

	setting := config.Settings
	fmt.Printf("原始 setting 内容: '%s'\n", setting)
	fmt.Printf("setting 长度: %d\n", len(setting))
	fmt.Printf("setting 是否为空: %t\n", setting == "")

	fmt.Println("开始 JSON 解析...")
	err := json.Unmarshal([]byte(setting), &cro)
	if err != nil {
		fmt.Printf("JSON 解析失败: %v\n", err)
		fmt.Printf("错误类型: %T\n", err)
	} else {
		fmt.Println("JSON 解析成功")
	}

	fmt.Printf("解析前 cro 结构体: %+v\n", cron{})
	fmt.Printf("解析后 cro 结构体: %+v\n", cro)
	fmt.Printf("cro.test 的值: '%v'\n", cro.Test)
	fmt.Printf("cro.test 的类型: %T\n", cro.Test)
	fmt.Printf("cro.test 是否为零值: %t\n", cro.Test == "")

	fmt.Println("=== 配置解析完成 ===")

	subscribe := eventBus.Subscribe("post")
	go func() {
		for event := range subscribe {
			fmt.Println("接收")
			fmt.Println(event.Payload)
		}
	}()
	return nil
}

// Stop 停止插件
func (p *CronPlugin) Stop(ctx context.Context) error {
	return nil
}
