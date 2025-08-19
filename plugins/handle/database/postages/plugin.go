package postages

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bearslyricattack/CompliK/pkg/constants"
	"github.com/bearslyricattack/CompliK/pkg/eventbus"
	"github.com/bearslyricattack/CompliK/pkg/models"
	"github.com/bearslyricattack/CompliK/pkg/plugin"
	"github.com/bearslyricattack/CompliK/pkg/utils/config"
	"github.com/bearslyricattack/CompliK/pkg/utils/logger"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

const (
	pluginName = constants.HandleDatabasePostgres
	pluginType = constants.HandleDatabasePluginType
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

type DetectorRecord struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	DiscoveryName string    `gorm:"size:255" json:"discovery_name"`
	CollectorName string    `gorm:"size:255" json:"collector_name"`
	DetectorName  string    `gorm:"size:255" json:"detector_name"`
	Name          string    `gorm:"size:255" json:"name"`
	Namespace     string    `gorm:"size:255" json:"namespace"`
	Host          string    `gorm:"size:255" json:"host"`
	Path          *string   `gorm:"type:json" json:"path"`
	URL           string    `gorm:"size:500" json:"url"`
	IsIllegal     bool      `json:"is_illegal"`
	Description   string    `gorm:"type:text" json:"description,omitempty"`
	Keywords      *string   `gorm:"type:json" json:"keywords,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (p *DatabasePlugin) Name() string { return pluginName }
func (p *DatabasePlugin) Type() string { return pluginType }

func (p *DatabasePlugin) Start(ctx context.Context, config config.PluginConfig, eventBus *eventbus.EventBus) error {
	if err := p.initDB(); err != nil {
		return fmt.Errorf("初始化数据库失败: %v", err)
	}

	if err := p.db.AutoMigrate(&DetectorRecord{}); err != nil {
		return fmt.Errorf("数据库迁移失败: %v", err)
	}

	subscribe := eventBus.Subscribe(constants.DetectorTopic)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				p.logger.Error(fmt.Sprintf("DatabasePlugin panic: %v", r))
			}
		}()

		for {
			select {
			case event, ok := <-subscribe:
				if !ok {
					p.logger.Info("数据库插件事件通道已关闭")
					return
				}
				result, ok := event.Payload.(*models.DetectorInfo)
				if !ok {
					p.logger.Error(fmt.Sprintf("事件类型错误: %T", event.Payload))
					continue
				}
				if err := p.saveResults(result); err != nil {
					p.logger.Error(fmt.Sprintf("保存数据失败: %v", err))
				}
			case <-ctx.Done():
				p.logger.Info("DatabasePlugin 停止")
				return
			}
		}
	}()

	return nil
}

func (p *DatabasePlugin) Stop(ctx context.Context) error {
	if p.db != nil {
		sqlDB, err := p.db.DB()
		if err != nil {
			return fmt.Errorf("获取数据库连接失败: %v", err)
		}
		return sqlDB.Close()
	}
	return nil
}

func (p *DatabasePlugin) initDB() error {
	dsn := "root:l6754g75@tcp(dbconn.sealoshzh.site:33144)/?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("连接 MySQL 服务器失败: %v", err)
	}

	databaseName := "complik"
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
	return nil
}

func (p *DatabasePlugin) saveResults(result *models.DetectorInfo) error {
	if p == nil {
		return fmt.Errorf("DatabasePlugin 实例为空")
	}
	if p.db == nil {
		return fmt.Errorf("数据库连接未初始化")
	}
	if result == nil {
		return fmt.Errorf("分析结果为空")
	}

	record := DetectorRecord{
		DiscoveryName: result.DiscoveryName,
		CollectorName: result.CollectorName,
		DetectorName:  result.DetectorName,
		Name:          result.Name,
		Namespace:     result.Namespace,
		Host:          result.Host,
		URL:           result.URL,
		IsIllegal:     result.IsIllegal,
		Description:   result.Description,
	}

	// 处理 Path 字段 - 只有在有数据时才设置值
	if result.Path != nil && len(result.Path) > 0 {
		if pathJSON, err := json.Marshal(result.Path); err == nil {
			pathStr := string(pathJSON)
			record.Path = &pathStr
		}
	}
	// 如果 Path 为空，record.Path 保持 nil，数据库中将存储 NULL

	// 处理 Keywords 字段 - 只有在有数据时才设置值
	if result.Keywords != nil && len(result.Keywords) > 0 {
		if keywordsJSON, err := json.Marshal(result.Keywords); err == nil {
			keywordsStr := string(keywordsJSON)
			record.Keywords = &keywordsStr
		}
	}
	// 如果 Keywords 为空，record.Keywords 保持 nil，数据库中将存储 NULL

	return p.db.Create(&record).Error
}
