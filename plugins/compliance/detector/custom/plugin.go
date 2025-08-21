package custom

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
	"github.com/bearslyricattack/CompliK/plugins/compliance/detector/utils"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"log"
	"runtime/debug"
	"strings"
	"time"
)

const (
	pluginName = constants.ComplianceDetectorCustom
	pluginType = constants.ComplianceDetectorPluginType
)

func init() {
	plugin.PluginFactories[pluginName] = func() plugin.Plugin {
		return &CustomPlugin{
			logger: logger.NewLogger(),
		}
	}
}

type CustomPlugin struct {
	logger       *logger.Logger
	reviewer     *utils.ContentReviewer
	db           *gorm.DB
	keywords     []utils.CustomKeywordRule
	customConfig CustomConfig
}

func (p *CustomPlugin) Name() string {
	return pluginName
}

func (p *CustomPlugin) Type() string {
	return pluginType
}

type CustomConfig struct {
	Dsn          string `json:"dsn"`
	DatabaseName string `json:"databaseName"`
	TickerMinute int    `json:"tickerMinute"`
	MaxWorkers   int    `json:"maxWorkers"`
	Host         string `json:"host"`
	Port         string `json:"port"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	Charset      string `json:"charset"`
	TableName    string `json:"tableName"`
	APIKey       string `json:"apiKey"`
	APIBase      string `json:"apiBase"`
	APIPath      string `json:"apiPath"`
	Model        string `json:"model"`
}

func (p *CustomPlugin) getDefaultConfig() CustomConfig {
	return CustomConfig{
		DatabaseName: "custom",
		Charset:      "utf8mb4",
		TickerMinute: 600,
		MaxWorkers:   20,
		TableName:    "CustomKeywordRule",
		Model:        "gpt-5",
		APIBase:      "https://aiproxy.usw.sealos.io/v1",
		APIPath:      "/chat/completions",
	}
}

func (p *CustomPlugin) loadConfig(setting string) error {
	p.customConfig = p.getDefaultConfig()
	if setting == "" {
		return errors.New("配置不能为空")
	}
	var configFromJSON CustomConfig
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
	if configFromJSON.APIKey == "" {
		return errors.New("APIKey 配置不能为空")
	}
	p.customConfig.Host = configFromJSON.Host
	p.customConfig.Port = configFromJSON.Port
	p.customConfig.Username = configFromJSON.Username
	p.customConfig.Password = configFromJSON.Password
	p.customConfig.APIKey = configFromJSON.APIKey

	if configFromJSON.APIPath != "" {
		p.customConfig.APIPath = configFromJSON.APIPath
	}
	if configFromJSON.APIBase != "" {
		p.customConfig.APIBase = configFromJSON.APIBase
	}
	if configFromJSON.Dsn != "" {
		p.customConfig.Dsn = configFromJSON.Dsn
	}
	if configFromJSON.DatabaseName != "" {
		p.customConfig.DatabaseName = configFromJSON.DatabaseName
	}
	if configFromJSON.TickerMinute > 0 {
		p.customConfig.TickerMinute = configFromJSON.TickerMinute
	}
	if configFromJSON.MaxWorkers > 0 {
		p.customConfig.MaxWorkers = configFromJSON.MaxWorkers
	}
	if configFromJSON.Charset != "" {
		p.customConfig.Charset = configFromJSON.Charset
	}
	if configFromJSON.TableName != "" {
		p.customConfig.TableName = configFromJSON.TableName
	}
	if configFromJSON.Model != " " {
		p.customConfig.Model = configFromJSON.Model
	}
	return nil
}

func (p *CustomPlugin) Start(ctx context.Context, config config.PluginConfig, eventBus *eventbus.EventBus) error {
	err := p.loadConfig(config.Settings)
	if err != nil {
		return fmt.Errorf("加载配置失败: %v", err)
	}
	if err := p.initDB(); err != nil {
		return fmt.Errorf("初始化数据库失败: %v", err)
	}
	p.reviewer = utils.NewContentReviewer(p.logger, p.customConfig.APIKey, p.customConfig.APIBase, p.customConfig.APIPath, p.customConfig.Model)
	err = p.readFromDatabase(ctx)
	if err != nil {
		p.logger.Error(fmt.Sprintf("定时任务读取数据库失败: %v", err))
		return err
	}
	subscribe := eventBus.Subscribe(constants.CollectorTopic)
	semaphore := make(chan struct{}, p.customConfig.MaxWorkers)
	ticker := time.NewTicker(time.Duration(p.customConfig.TickerMinute) * time.Minute)
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
						debug.PrintStack()
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
			for i := 0; i < p.customConfig.MaxWorkers; i++ {
				semaphore <- struct{}{}
			}
			return nil
		}
	}
}

func (p *CustomPlugin) Stop(ctx context.Context) error {
	return nil
}

func (p *CustomPlugin) initDB() error {
	serverDSN := p.buildDSN(false)
	db, err := gorm.Open(mysql.Open(serverDSN), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("连接 MySQL 服务器失败: %v", err)
	}
	err = db.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s CHARACTER SET %s COLLATE %s_unicode_ci",
		p.customConfig.DatabaseName,
		p.customConfig.Charset,
		p.customConfig.Charset)).Error
	if err != nil {
		return fmt.Errorf("创建数据库失败: %v", err)
	}
	dbDSN := p.buildDSN(true)
	db, err = gorm.Open(mysql.Open(dbDSN), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("连接到数据库失败: %v", err)
	}
	p.db = db
	tableName := db.NamingStrategy.TableName("CustomKeywordRule")

	err = db.AutoMigrate(&utils.CustomKeywordRule{})
	if err != nil {
		return fmt.Errorf("创建表失败: %v", err)
	}
	var tableExists bool
	err = db.Raw("SELECT COUNT(*) > 0 FROM information_schema.tables WHERE table_schema = ? AND table_name = ?",
		p.customConfig.TableName, tableName).Scan(&tableExists).Error
	if err != nil {
		log.Printf("检查表存在性失败: %v", err)
	}
	var count int64
	err = db.Model(&utils.CustomKeywordRule{}).Count(&count).Error
	if err != nil {
		return fmt.Errorf("查询数据数量失败: %v", err)
	}
	if count == 0 {
		sampleRule := utils.CustomKeywordRule{
			Type:        "malware",
			Keywords:    strings.Join([]string{"virus", "trojan", "malware", "backdoor"}, ","),
			Description: "恶意软件检测规则",
		}
		err = db.Create(&sampleRule).Error
		if err != nil {
			return fmt.Errorf("插入示例数据失败: %v", err)
		}
		var newCount int64
		db.Model(&utils.CustomKeywordRule{}).Count(&newCount)
	}
	return nil
}

func (p *CustomPlugin) buildDSN(includeDB bool) string {
	if p.customConfig.Dsn != "" {
		return p.customConfig.Dsn
	}
	dbPart := "/"
	if includeDB {
		dbPart = "/" + p.customConfig.DatabaseName
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%s)%s?charset=%s&parseTime=True&loc=Local",
		p.customConfig.Username,
		p.customConfig.Password,
		p.customConfig.Host,
		p.customConfig.Port,
		dbPart,
		p.customConfig.Charset,
	)
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
