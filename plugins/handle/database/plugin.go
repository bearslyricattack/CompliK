package database

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/bearslyricattack/CompliK/pkg/constants"
	"log"
	"time"

	"github.com/bearslyricattack/CompliK/pkg/eventbus"
	"github.com/bearslyricattack/CompliK/pkg/models"
	"github.com/bearslyricattack/CompliK/pkg/plugin"
	"github.com/bearslyricattack/CompliK/pkg/utils/config"
	"github.com/bearslyricattack/CompliK/pkg/utils/logger"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

const (
	pluginName = "Database"
	pluginType = "Handle"
)

func init() {
	plugin.PluginFactories[pluginName] = func() plugin.Plugin {
		return &DatabasePlugin{
			logger: logger.NewLogger(),
		}
	}
}

type DatabasePlugin struct {
	logger *logger.Logger
	db     *gorm.DB
}
type IngressAnalysisRecord struct {
	ID          uint   `gorm:"primaryKey"`
	URL         string `gorm:"type:text;not null"`
	IsIllegal   bool   `gorm:"default:false"`
	Description string `gorm:"type:text"`
	Keywords    string `gorm:"type:text"`
	Namespace   string `gorm:"index"`
	Html        string `gorm:"type:text"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (p *DatabasePlugin) Name() string { return pluginName }
func (p *DatabasePlugin) Type() string { return pluginType }

func (p *DatabasePlugin) Start(ctx context.Context, config config.PluginConfig, eventBus *eventbus.EventBus) error {
	if err := p.initDB(); err != nil {
		return err
	}
	if err := p.db.AutoMigrate(&IngressAnalysisRecord{}); err != nil {
		return err
	}
	subscribe := eventBus.Subscribe(constants.ComplianceWebsiteTopic)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("DatabasePlugin panic: %v", r)
			}
		}()
		for {
			select {
			case event, ok := <-subscribe:
				if !ok {
					log.Println("数据库插件事件通道已关闭")
					return
				}
				result, ok := event.Payload.(*models.IngressAnalysisResult)
				if !ok {
					log.Printf("事件类型错误: %T", event.Payload)
					continue
				}
				if err := p.saveResults(result); err != nil {
					log.Printf("保存数据失败: %v", err)
				}
			case <-ctx.Done():
				log.Println("DatabasePlugin 停止")
				return
			}
		}
	}()

	return nil
}

func (p *DatabasePlugin) Stop(ctx context.Context) error {
	if p.db != nil {
		sqlDB, _ := p.db.DB()
		return sqlDB.Close()
	}
	return nil
}

func (p *DatabasePlugin) initDB() error {
	// 首先连接到 MySQL 服务器（不指定数据库）
	dsn := "root:l6754g75@tcp(dbconn.sealoshzh.site:33144)/?charset=utf8mb4&parseTime=True&loc=Local"

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("连接 MySQL 服务器失败: %v", err)
	}

	// 创建数据库
	databaseName := "complik" // 你的数据库名
	err = db.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci", databaseName)).Error
	if err != nil {
		return fmt.Errorf("创建数据库失败: %v", err)
	}
	// 重新连接到指定的数据库
	newDsn := fmt.Sprintf("root:l6754g75@tcp(dbconn.sealoshzh.site:33144)/%s?charset=utf8mb4&parseTime=True&loc=Local", databaseName)
	db, err = gorm.Open(mysql.Open(newDsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("连接到数据库失败: %v", err)
	}

	p.db = db
	return nil
}

func (p *DatabasePlugin) saveResults(result *models.IngressAnalysisResult) error {
	if p == nil {
		return fmt.Errorf("DatabasePlugin 实例为空")
	}
	if p.db == nil {
		return fmt.Errorf("数据库连接未初始化")
	}
	if result == nil {
		return fmt.Errorf("分析结果为空")
	}
	var record IngressAnalysisRecord
	keywordsJSON := ""
	if result.Keywords != nil && len(result.Keywords) > 0 {
		if data, err := json.Marshal(&result.Keywords); err == nil {
			if data != nil {
				keywordsJSON = string(data)
			}
		}
	}
	record.URL = result.URL
	record.IsIllegal = result.IsIllegal
	record.Description = result.Description
	record.Keywords = keywordsJSON
	record.Namespace = result.Namespace
	record.Html = result.Html

	return p.db.Create(&record).Error
}
