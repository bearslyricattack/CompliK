package lark

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bearslyricattack/CompliK/pkg/constants"
	"github.com/bearslyricattack/CompliK/pkg/eventbus"
	"github.com/bearslyricattack/CompliK/pkg/models"
	"github.com/bearslyricattack/CompliK/pkg/plugin"
	"github.com/bearslyricattack/CompliK/pkg/utils/config"
	"github.com/bearslyricattack/CompliK/pkg/utils/logger"
	"github.com/bearslyricattack/CompliK/plugins/handle/lark/whitelist"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
	"log"
	"os"
	"time"
)

const (
	pluginName = constants.HandleLark
	pluginType = constants.HandleLarkPluginType
)

func init() {
	plugin.PluginFactories[pluginName] = func() plugin.Plugin {
		return &LarkPlugin{
			logger: logger.NewLogger(),
		}
	}
}

type LarkPlugin struct {
	logger     *logger.Logger
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
		p.logger.Error("解析配置失败: " + err.Error())
		return err
	}
	if configFromJSON.Webhook == "" {
		return errors.New("webhook 配置不能为空")
	}
	if *configFromJSON.EnabledWhitelist == true {
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
		p.larkConfig.Password = configFromJSON.Password
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
		return nil, fmt.Errorf("连接 MySQL 服务器失败: %v", err)
	}
	err = db.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s CHARACTER SET %s COLLATE %s_unicode_ci",
		p.larkConfig.DatabaseName,
		p.larkConfig.Charset,
		p.larkConfig.Charset)).Error
	if err != nil {
		return nil, fmt.Errorf("创建数据库失败: %v", err)
	}
	dbDSN := p.buildDSN(true)
	db, err = gorm.Open(mysql.Open(dbDSN), dbConfig)
	if err != nil {
		return nil, fmt.Errorf("连接到数据库失败: %v", err)
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

func (p *LarkPlugin) Start(ctx context.Context, config config.PluginConfig, eventBus *eventbus.EventBus) error {
	err := p.loadConfig(config.Settings)
	if err != nil {
		return err
	}
	if *p.larkConfig.EnabledWhitelist {
		var db *gorm.DB
		if db, err = p.initDB(); err != nil {
			return fmt.Errorf("初始化数据库失败: %v", err)
		}
		if err := db.AutoMigrate(&whitelist.Whitelist{}); err != nil {
			return fmt.Errorf("数据库迁移失败: %v", err)
		}
		p.notifier = NewNotifier(p.larkConfig.Webhook, db)
	} else {
		p.notifier = NewNotifier(p.larkConfig.Webhook, nil)
	}
	subscribe := eventBus.Subscribe(constants.DetectorTopic)
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
				result, ok := event.Payload.(*models.DetectorInfo)
				if !ok {
					log.Printf("事件负载类型错误，期望*models.DetectorInfo，实际: %T", event.Payload)
					continue
				}
				result.Region = p.larkConfig.Region
				err := p.notifier.SendAnalysisNotification(result)
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

func (p *LarkPlugin) Stop(ctx context.Context) error {
	return nil
}
