package postages

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
	"github.com/bearslyricattack/CompliK/pkg/models"
	"github.com/bearslyricattack/CompliK/pkg/plugin"
	"github.com/bearslyricattack/CompliK/pkg/utils/config"
	"github.com/bearslyricattack/CompliK/pkg/utils/logger"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
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
	logger         *logger.Logger
	db             *gorm.DB
	databaseConfig DatabaseConfig
}
type DatabaseConfig struct {
	Region       string `json:"region"`
	Host         string `json:"host"`
	Port         string `json:"port"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	DatabaseName string `json:"databaseName"`
	TableName    string `json:"tableName"`
	Charset      string `json:"charset"`
}

func (p *DatabasePlugin) getDefaultConfig() DatabaseConfig {
	return DatabaseConfig{
		DatabaseName: "complik",
		Charset:      "utf8mb4",
		TableName:    "detectorRecord",
		Region:       "UNKNOWN",
	}
}

func (p *DatabasePlugin) loadConfig(setting string) error {
	p.databaseConfig = p.getDefaultConfig()
	if setting == "" {
		return errors.New("配置不能为空")
	}
	var configFromJSON DatabaseConfig
	err := json.Unmarshal([]byte(setting), &configFromJSON)
	if err != nil {
		p.logger.Error("解析配置失败: " + err.Error())
		return err
	}
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
	p.databaseConfig.Host = configFromJSON.Host
	p.databaseConfig.Port = configFromJSON.Port
	p.databaseConfig.Username = configFromJSON.Username
	p.databaseConfig.Password = configFromJSON.Password
	p.databaseConfig.Region = configFromJSON.Region
	if configFromJSON.Region != "" {
		p.databaseConfig.Region = configFromJSON.Region
	}
	if configFromJSON.DatabaseName != "" {
		p.databaseConfig.DatabaseName = configFromJSON.DatabaseName
	}
	if configFromJSON.Charset != "" {
		p.databaseConfig.Charset = configFromJSON.Charset
	}
	if configFromJSON.TableName != "" {
		p.databaseConfig.TableName = configFromJSON.TableName
	}
	return nil
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
	err := p.loadConfig(config.Settings)
	if err != nil {
		return err
	}
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
				result.Region = p.databaseConfig.Region
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
	db, err := gorm.Open(mysql.Open(serverDSN), dbConfig)
	if err != nil {
		return fmt.Errorf("连接 MySQL 服务器失败: %v", err)
	}
	err = db.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s CHARACTER SET %s COLLATE %s_unicode_ci",
		p.databaseConfig.DatabaseName,
		p.databaseConfig.Charset,
		p.databaseConfig.Charset)).Error
	if err != nil {
		return fmt.Errorf("创建数据库失败: %v", err)
	}
	dbDSN := p.buildDSN(true)
	db, err = gorm.Open(mysql.Open(dbDSN), dbConfig)
	if err != nil {
		return fmt.Errorf("连接到数据库失败: %v", err)
	}
	p.db = db
	return nil
}

func (p *DatabasePlugin) buildDSN(includeDB bool) string {
	dbPart := "/"
	if includeDB {
		dbPart = "/" + p.databaseConfig.DatabaseName
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%s)%s?charset=%s&parseTime=True&loc=Local",
		p.databaseConfig.Username,
		p.databaseConfig.Password,
		p.databaseConfig.Host,
		p.databaseConfig.Port,
		dbPart,
		p.databaseConfig.Charset,
	)
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
	if result.Path != nil && len(result.Path) > 0 {
		if pathJSON, err := json.Marshal(result.Path); err == nil {
			pathStr := string(pathJSON)
			record.Path = &pathStr
		}
	}
	if result.Keywords != nil && len(result.Keywords) > 0 {
		if keywordsJSON, err := json.Marshal(result.Keywords); err == nil {
			keywordsStr := string(keywordsJSON)
			record.Keywords = &keywordsStr
		}
	}
	return p.db.Create(&record).Error
}
