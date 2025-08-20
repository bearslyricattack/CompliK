package higress

//
// import (
// 	"context"
// 	"fmt"
// 	"github.com/bearslyricattack/CompliK/pkg/constants"
// 	"github.com/bearslyricattack/CompliK/pkg/eventbus"
// 	"github.com/bearslyricattack/CompliK/pkg/models"
// 	"github.com/bearslyricattack/CompliK/pkg/plugin"
// 	"github.com/bearslyricattack/CompliK/pkg/utils/config"
// 	"github.com/bearslyricattack/CompliK/pkg/utils/logger"
// 	"github.com/bearslyricattack/CompliK/plugins/compliance/collector/higress/utils"
// 	"log"
// 	"strings"
// 	"time"
// )
//
// const (
// 	pluginName = constants.ComplianceCollectorHigressName
// 	pluginType = constants.ComplianceHigressPluginType
// )
//
// const (
// 	maxWorkers = 20
// )
//
// func init() {
// 	plugin.PluginFactories[pluginName] = func() plugin.Plugin {
// 		return &c{
// 			logger:      logger.NewLogger(),
// 		}
// 	}
// }
//
// type HigressPlugin struct {
// 	logger      *logger.Logger
// }
//
// func (p *HigressPlugin) Name() string {
// 	return pluginName
// }
//
// func (p *HigressPlugin) Type() string {
// 	return pluginType
// }
//
// func (p *HigressPlugin) Start(ctx context.Context, config config.PluginConfig, eventBus *eventbus.EventBus) error {
// 	subscribe := eventBus.Subscribe(constants.DiscoveryTopic)
// 	semaphore := make(chan struct{}, maxWorkers)
//
// 	for {
// 		select {
// 		case event, ok := <-subscribe:
// 			if !ok {
// 				log.Println("事件订阅通道已关闭")
// 				return nil
// 			}
// 			semaphore <- struct{}{}
// 			go func(e eventbus.Event) {
// 				defer func() { <-semaphore }()
// 				defer func() {
// 					if r := recover(); r != nil {
// 						log.Printf("goroutine panic: %v", r)
// 					}
// 				}()
//
// 				ingress, ok := e.Payload.(models.DiscoveryInfo)
// 				if !ok {
// 					p.logger.Error(fmt.Sprintf("事件负载类型错误，期望models.DiscoveryInfo，实际: %T", e.Payload))
// 					return
// 				}
// 					p.logger.Error(fmt.Sprintf("本次读取 Higress 日志错误：ingress：%s，%v\n", ingress.Host, err))
// 				} else {
// 					eventBus.Publish(constants.CollectorTopic, eventbus.Event{
// 						Payload: result,
// 					})
// 				}
// 			}(event)
// 		case <-ctx.Done():
// 			for i := 0; i < maxWorkers; i++ {
// 				semaphore <- struct{}{}
// 			}
// 			return nil
// 		}
// 	}
// }
