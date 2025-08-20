package custom

import (
	"context"
	"fmt"
	"github.com/bearslyricattack/CompliK/pkg/constants"
	"github.com/bearslyricattack/CompliK/pkg/eventbus"
	"github.com/bearslyricattack/CompliK/pkg/models"
	"github.com/bearslyricattack/CompliK/pkg/plugin"
	"github.com/bearslyricattack/CompliK/pkg/utils/config"
	"github.com/bearslyricattack/CompliK/pkg/utils/logger"
	"github.com/bearslyricattack/CompliK/plugins/compliance/detector/utils"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"log"
	"time"
)

const (
	pluginName = constants.ComplianceDetectorCustom
	pluginType = constants.ComplianceDetectorPluginType
)

const (
	maxWorkers = 30
)

func init() {
	plugin.PluginFactories[pluginName] = func() plugin.Plugin {
		return &CustomPlugin{
			logger:   logger.NewLogger(),
			reviewer: utils.NewContentReviewer(logger.NewLogger()),
		}
	}
}

type CustomPlugin struct {
	logger   *logger.Logger
	reviewer *utils.ContentReviewer
	db       *gorm.DB
	keywords []utils.CustomKeywordRule
}

func (p *CustomPlugin) Name() string {
	return pluginName
}

func (p *CustomPlugin) Type() string {
	return pluginType
}

func (p *CustomPlugin) initDB() error {
	dsn := "root:l6754g75@tcp(dbconn.sealoshzh.site:33144)/?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("连接 MySQL 服务器失败: %v", err)
	}

	databaseName := "custom"
	err = db.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci", databaseName)).Error
	if err != nil {
		return fmt.Errorf("创建数据库失败: %v", err)
	}

	newDsn := fmt.Sprintf("root:l6754g75@tcp(dbconn.sealoshzh.site:33144)/%s?charset=utf8mb4&parseTime=True&loc=Local", databaseName)
	db, err = gorm.Open(mysql.Open(newDsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("连接到数据库失败: %v", err)
	}

	p.db = db

	err = db.AutoMigrate(&utils.CustomKeywordRule{})
	if err != nil {
		return fmt.Errorf("创建表失败: %v", err)
	}

	var count int64
	db.Model(&utils.CustomKeywordRule{}).Count(&count)
	if count == 0 {
		sampleRule := utils.CustomKeywordRule{
			Type:        "malware",
			Keywords:    []string{"virus", "trojan", "malware", "backdoor"},
			Description: "恶意软件检测规则",
		}
		err = db.Create(&sampleRule).Error
		if err != nil {
			return fmt.Errorf("插入示例数据失败: %v", err)
		}
		log.Println("已插入示例数据")
	}
	return nil
}

func (p *CustomPlugin) Start(ctx context.Context, config config.PluginConfig, eventBus *eventbus.EventBus) error {
	if err := p.initDB(); err != nil {
		return fmt.Errorf("初始化数据库失败: %v", err)
	}
	subscribe := eventBus.Subscribe(constants.CollectorTopic)
	semaphore := make(chan struct{}, maxWorkers)
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
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
				res, ok := e.Payload.(*models.CollectorInfo)
				if !ok {
					log.Printf("事件负载类型错误，期望models.CollectorResult，实际: %T", e.Payload)
					return
				}
				result, err := p.customJudge(ctx, res)
				if err != nil {
					p.logger.Error(fmt.Sprintf("本次判断错误：ingress：%s，%v\n", result.Host, err))
				}
				eventBus.Publish(constants.DetectorTopic, eventbus.Event{
					Payload: result,
				})
			}(event)
		case <-ticker.C:
			go func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("定时任务 goroutine panic: %v", r)
					}
				}()
				err := p.readFromDatabase(ctx)
				if err != nil {
					p.logger.Error(fmt.Sprintf("定时任务读取数据库失败: %v", err))
					return
				}
			}()
		case <-ctx.Done():
			for i := 0; i < maxWorkers; i++ {
				semaphore <- struct{}{}
			}
			return nil
		}
	}
}

func (p *CustomPlugin) readFromDatabase(ctx context.Context) error {
	var models []utils.CustomKeywordRule
	err := p.db.WithContext(ctx).Find(&models).Error
	if err != nil {
		return err
	} else {
		p.keywords = models
		return nil
	}
}

func (p *CustomPlugin) Stop(ctx context.Context) error {
	return nil
}

func (p *CustomPlugin) customJudge(ctx context.Context, collector *models.CollectorInfo) (res *models.DetectorInfo, err error) {
	taskCtx, cancel := context.WithTimeout(ctx, 80*time.Second)
	defer cancel()
	if collector.IsEmpty == true {
		return &models.DetectorInfo{
			DiscoveryName: collector.DiscoveryName,
			CollectorName: collector.CollectorName,
			DetectorName:  p.Name(),
			Name:          collector.Name,
			Namespace:     collector.Namespace,
			Host:          collector.Host,
			Path:          collector.Path,
			URL:           collector.URL,
			IsIllegal:     false,
			Description:   "",
			Keywords:      []string{},
		}, nil
	}
	result, err := p.reviewer.ReviewSiteContent(taskCtx, collector, p.Name(), p.keywords)
	if err != nil {
		return &models.DetectorInfo{
			DiscoveryName: collector.DiscoveryName,
			CollectorName: collector.CollectorName,
			DetectorName:  p.Name(),
			Name:          collector.Name,
			Namespace:     collector.Namespace,
			Host:          collector.Host,
			Path:          collector.Path,
			URL:           collector.URL,
			IsIllegal:     false,
			Description:   "",
			Keywords:      []string{},
		}, err
	} else {
		return result, nil
	}
}
