package lark

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/bearslyricattack/CompliK/pkg/constants"
	"github.com/bearslyricattack/CompliK/pkg/eventbus"
	"github.com/bearslyricattack/CompliK/pkg/logger"
	"github.com/bearslyricattack/CompliK/pkg/models"
	"github.com/bearslyricattack/CompliK/pkg/plugin"
	"github.com/bearslyricattack/CompliK/pkg/utils/config"
	"github.com/bearslyricattack/CompliK/plugins/handle/lark/whitelist"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
)

const (
	pluginName = constants.HandleLark
	pluginType = constants.HandleLarkPluginType
)

func init() {
	plugin.PluginFactories[pluginName] = func() plugin.Plugin {
		return &LarkPlugin{
			log: logger.GetLogger().WithField("plugin", pluginName),
		}
	}
}

type LarkPlugin struct {
	log        logger.Logger
	notifier   *Notifier
	larkConfig LarkConfig
}

func (p *LarkPlugin) Name() string {
	return pluginName
}

func (p *LarkPlugin) Type() string {
	return pluginType
}

type LarkConfig struct {
	Region           string `json:"region"`
	Webhook          string `json:"webhook"`
	EnabledWhitelist *bool  `json:"enabled_whitelist"`
	Host             string `json:"host"`
	Port             string `json:"port"`
	Username         string `json:"username"`
	Password         string `json:"password"`
	DatabaseName     string `json:"databaseName"`
	TableName        string `json:"tableName"`
	Charset          string `json:"charset"`
	HostTimeoutHour  int    `json:"host_timeout_hour"`
}

func (p *LarkPlugin) getDefaultConfig() LarkConfig {
	b := false
	return LarkConfig{
		Region:           "UNKNOWN",
		EnabledWhitelist: &b,
		DatabaseName:     "complik",
		TableName:        "whitelist",
		Charset:          "utf8mb4",
	}
}

func (p *LarkPlugin) loadConfig(setting string) error {
	p.larkConfig = p.getDefaultConfig()
	if setting == "" {
		return errors.New("配置不能为空")
	}
	var configFromJSON LarkConfig
	err := json.Unmarshal([]byte(setting), &configFromJSON)
	if err != nil {
		p.log.Error("Failed to parse config", logger.Fields{
			"error": err.Error(),
		})
		return err
	}
	if configFromJSON.Webhook == "" {
		return errors.New("webhook 配置不能为空")
	}
	if configFromJSON.EnabledWhitelist != nil && *configFromJSON.EnabledWhitelist {
		p.larkConfig.EnabledWhitelist = configFromJSON.EnabledWhitelist
		if configFromJSON.Host == "" {
			return errors.New("host 配置不能为空")
		}
		if configFromJSON.Port == "" {
			return errors.New("port 配置不能为空")
		}
		if configFromJSON.Username == "" {
			return errors.New("username 配置不能为空")
		}
		if configFromJSON.Password == "" {
			return errors.New("password 配置不能为空")
		}
		p.larkConfig.Host = configFromJSON.Host
		p.larkConfig.Port = configFromJSON.Port
		p.larkConfig.Username = configFromJSON.Username
		// 支持从环境变量或加密值获取密码
		if pwd, err := config.GetSecureValue(configFromJSON.Password); err == nil {
			p.larkConfig.Password = pwd
		} else {
			p.larkConfig.Password = configFromJSON.Password
		}
	}
	if configFromJSON.HostTimeoutHour > 0 {
		p.larkConfig.HostTimeoutHour = configFromJSON.HostTimeoutHour
	}
	if configFromJSON.DatabaseName != "" {
		p.larkConfig.DatabaseName = configFromJSON.DatabaseName
	}
	if configFromJSON.TableName != "" {
		p.larkConfig.TableName = configFromJSON.TableName
	}
	if configFromJSON.Charset != "" {
		p.larkConfig.Charset = configFromJSON.Charset
	}
	p.larkConfig.Webhook = configFromJSON.Webhook
	if configFromJSON.Region != "" {
		p.larkConfig.Region = configFromJSON.Region
	}
	return nil
}

func (p *LarkPlugin) initDB() (db *gorm.DB, err error) {
	serverDSN := p.buildDSN(false)
	dbConfig := &gorm.Config{
		Logger: gormLogger.New(
			log.New(os.Stdout, "\r\n", log.LstdFlags),
			gormLogger.Config{
				SlowThreshold: 3 * time.Second,  // 慢查询阈值设为1秒
				LogLevel:      gormLogger.Error, // 只显示错误日志
				Colorful:      false,            // 关闭颜色输出
			},
		),
	}
	db, err = gorm.Open(mysql.Open(serverDSN), dbConfig)
	if err != nil {
		return nil, fmt.Errorf("连接 MySQL 服务器失败: %w", err)
	}
	createDBSQL := fmt.Sprintf(
		"CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET %s COLLATE %s_unicode_ci",
		p.larkConfig.DatabaseName,
		p.larkConfig.Charset,
		p.larkConfig.Charset,
	)
	err = db.Exec(createDBSQL).Error
	if err != nil {
		return nil, fmt.Errorf("创建数据库失败: %w", err)
	}
	dbDSN := p.buildDSN(true)
	db, err = gorm.Open(mysql.Open(dbDSN), dbConfig)
	if err != nil {
		return nil, fmt.Errorf("连接到数据库失败: %w", err)
	}
	return db, nil
}

func (p *LarkPlugin) buildDSN(includeDB bool) string {
	dbPart := "/"
	if includeDB {
		dbPart = "/" + p.larkConfig.DatabaseName
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%s)%s?charset=%s&parseTime=True&loc=Local",
		p.larkConfig.Username,
		p.larkConfig.Password,
		p.larkConfig.Host,
		p.larkConfig.Port,
		dbPart,
		p.larkConfig.Charset,
	)
}

func (p *LarkPlugin) Start(
	ctx context.Context,
	config config.PluginConfig,
	eventBus *eventbus.EventBus,
) error {
	err := p.loadConfig(config.Settings)
	if err != nil {
		return err
	}
	if *p.larkConfig.EnabledWhitelist {
		var db *gorm.DB
		if db, err = p.initDB(); err != nil {
			return fmt.Errorf("初始化数据库失败: %w", err)
		}
		if err := db.AutoMigrate(&whitelist.Whitelist{}); err != nil {
			return fmt.Errorf("数据库迁移失败: %w", err)
		}
		p.notifier = NewNotifier(
			p.larkConfig.Webhook,
			db,
			time.Duration(p.larkConfig.HostTimeoutHour)*time.Hour,
			p.larkConfig.Region,
		)
		var count int64
		db.Model(&whitelist.Whitelist{}).Count(&count)
		if count == 0 {
			testData := &whitelist.Whitelist{
				Region:    "cn-beijing",
				Name:      "测试白名单项目",
				Namespace: "default",
				Hostname:  "test.example.com",
				Type:      "namespace",
				Remark:    "这是一条初始化的测试数据，用于验证白名单功能",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			if err := db.Create(testData).Error; err != nil {
				p.log.Error("Failed to insert test data", logger.Fields{
					"error": err.Error(),
				})
			} else {
				p.log.Info("Test data inserted successfully")
			}
		}
	} else {
		p.notifier = NewNotifier(p.larkConfig.Webhook, nil, 0, "")
	}
	subscribe := eventBus.Subscribe(constants.DetectorTopic)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				p.log.Error("Plugin goroutine panic", logger.Fields{
					"panic": r,
				})
			}
		}()
		for {
			select {
			case event, ok := <-subscribe:
				if !ok {
					p.log.Info("Event subscription channel closed")
					return
				}
				result, ok := event.Payload.(*models.DetectorInfo)
				if !ok {
					p.log.Error("Invalid event payload type", logger.Fields{
						"expected": "*models.DetectorInfo",
						"actual":   fmt.Sprintf("%T", event.Payload),
					})
					continue
				}
				result.Region = p.larkConfig.Region
				err := p.notifier.SendAnalysisNotification(result)
				if err != nil {
					p.log.Error("Failed to send notification", logger.Fields{
						"error": err.Error(),
					})
				}
			case <-ctx.Done():
				p.log.Info("Plugin received stop signal")
				return
			}
		}
	}()
	return nil
}

func (p *LarkPlugin) Stop(ctx context.Context) error {
	return nil
}
