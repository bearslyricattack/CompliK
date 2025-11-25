package config

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	legacy "github.com/bearslyricattack/CompliK/procscan/pkg/logger/legacy"
	"github.com/bearslyricattack/CompliK/procscan/pkg/models"
)

// EnvLoader 环境变量加载器
type EnvLoader struct {
	prefix     string                   // 环境变量前缀
	separator  string                   // 分隔符，默认为"_"
	mapping    map[string]string        // 字段映射
	converters map[string]TypeConverter // 类型转换器
}

// TypeConverter 类型转换器接口
type TypeConverter interface {
	Convert(value string) (interface{}, error)
}

// NewEnvLoader 创建新的环境变量加载器
func NewEnvLoader(prefix string) *EnvLoader {
	if prefix == "" {
		prefix = "PROCSCAN"
	}

	return &EnvLoader{
		prefix:     strings.ToUpper(prefix),
		separator:  "_",
		mapping:    make(map[string]string),
		converters: make(map[string]TypeConverter),
	}
}

// SetSeparator 设置分隔符
func (e *EnvLoader) SetSeparator(separator string) *EnvLoader {
	e.separator = separator
	return e
}

// AddMapping 添加字段映射
func (e *EnvLoader) AddMapping(field, envKey string) *EnvLoader {
	e.mapping[field] = envKey
	return e
}

// AddConverter 添加类型转换器
func (e *EnvLoader) AddConverter(field string, converter TypeConverter) *EnvLoader {
	e.converters[field] = converter
	return e
}

// LoadFromEnv 从环境变量加载配置
func (e *EnvLoader) LoadFromEnv(config *models.Config) error {
	legacy.L.WithField("prefix", e.prefix).Info("开始从环境变量加载配置")

	// 加载扫描器配置
	if err := e.loadScannerConfig(&config.Scanner); err != nil {
		return fmt.Errorf("加载扫描器配置失败: %w", err)
	}

	// 加载动作配置
	if err := e.loadActionsConfig(&config.Actions); err != nil {
		return fmt.Errorf("加载动作配置失败: %w", err)
	}

	// 加载通知配置
	if err := e.loadNotificationsConfig(&config.Notifications); err != nil {
		return fmt.Errorf("加载通知配置失败: %w", err)
	}

	// 加载检测规则
	if err := e.loadDetectionRules(&config.DetectionRules); err != nil {
		return fmt.Errorf("加载检测规则失败: %w", err)
	}

	legacy.L.Info("环境变量配置加载完成")
	return nil
}

// loadScannerConfig 加载扫描器配置
func (e *EnvLoader) loadScannerConfig(scanner *models.ScannerConfig) error {
	configMap := map[string]interface{}{
		"scanner.proc_path":     &scanner.ProcPath,
		"scanner.scan_interval": &scanner.ScanInterval,
		"scanner.log_level":     &scanner.LogLevel,
	}

	return e.loadConfigMap(configMap)
}

// loadActionsConfig 加载动作配置
func (e *EnvLoader) loadActionsConfig(actions *models.ActionsConfig) error {
	configMap := map[string]interface{}{
		"actions.label.enabled": &actions.Label.Enabled,
		"actions.label.data":    &actions.Label.Data,
	}

	return e.loadConfigMap(configMap)
}

// loadNotificationsConfig 加载通知配置
func (e *EnvLoader) loadNotificationsConfig(notifications *models.NotificationsConfig) error {
	configMap := map[string]interface{}{
		"notifications.lark.webhook": &notifications.Lark.Webhook,
	}

	return e.loadConfigMap(configMap)
}

// loadDetectionRules 加载检测规则
func (e *EnvLoader) loadDetectionRules(rules *models.DetectionRules) error {
	// 黑名单规则
	blacklistMap := map[string]interface{}{
		"detectionRules.blacklist.processes":  &rules.Blacklist.Processes,
		"detectionRules.blacklist.keywords":   &rules.Blacklist.Keywords,
		"detectionRules.blacklist.commands":   &rules.Blacklist.Commands,
		"detectionRules.blacklist.namespaces": &rules.Blacklist.Namespaces,
		"detectionRules.blacklist.podNames":   &rules.Blacklist.PodNames,
	}

	if err := e.loadConfigMap(blacklistMap); err != nil {
		return err
	}

	// 白名单规则
	whitelistMap := map[string]interface{}{
		"detectionRules.whitelist.processes":  &rules.Whitelist.Processes,
		"detectionRules.whitelist.keywords":   &rules.Whitelist.Keywords,
		"detectionRules.whitelist.commands":   &rules.Whitelist.Commands,
		"detectionRules.whitelist.namespaces": &rules.Whitelist.Namespaces,
		"detectionRules.whitelist.podNames":   &rules.Whitelist.PodNames,
	}

	return e.loadConfigMap(whitelistMap)
}

// loadConfigMap 加载配置映射
func (e *EnvLoader) loadConfigMap(configMap map[string]interface{}) error {
	for field, target := range configMap {
		envValue := e.getEnvValue(field)
		if envValue == "" {
			continue
		}

		// 类型转换
		convertedValue, err := e.convertValue(field, envValue)
		if err != nil {
			legacy.L.WithFields(map[string]interface{}{
				"field": field,
				"value": envValue,
				"error": err.Error(),
			}).Error("环境变量类型转换失败")
			return fmt.Errorf("字段 '%s' 类型转换失败: %w", field, err)
		}

		// 设置值
		if err := e.setFieldValue(target, convertedValue); err != nil {
			return fmt.Errorf("设置字段 '%s' 值失败: %w", field, err)
		}

		legacy.L.WithFields(map[string]interface{}{
			"field":   field,
			"env_key": e.getEnvKey(field),
			"value":   convertedValue,
		}).Debug("从环境变量加载配置")
	}

	return nil
}

// getEnvValue 获取环境变量值
func (e *EnvLoader) getEnvValue(field string) string {
	envKey := e.getEnvKey(field)
	return os.Getenv(envKey)
}

// getEnvKey 获取环境变量键名
func (e *EnvLoader) getEnvKey(field string) string {
	// 检查是否有自定义映射
	if customKey, exists := e.mapping[field]; exists {
		return customKey
	}

	// 默认映射：将点号分隔的字段名转换为下划线分隔
	key := strings.ReplaceAll(field, ".", e.separator)
	key = strings.ReplaceAll(key, "-", "_")
	key = strings.ToUpper(key)
	return e.prefix + e.separator + key
}

// convertValue 转换值类型
func (e *EnvLoader) convertValue(field, value string) (interface{}, error) {
	// 检查是否有自定义转换器
	if converter, exists := e.converters[field]; exists {
		return converter.Convert(value)
	}

	// 自动类型推断
	return e.autoConvert(field, value)
}

// autoConvert 自动类型转换
func (e *EnvLoader) autoConvert(field, value string) (interface{}, error) {
	// 根据字段名推断类型
	if strings.Contains(field, "interval") {
		duration, err := time.ParseDuration(value)
		if err != nil {
			return nil, fmt.Errorf("无效的时间间隔格式: %w", err)
		}
		return duration, nil
	}

	if strings.Contains(field, "enabled") {
		if strings.ToLower(value) == "true" || value == "1" {
			return true, nil
		} else if strings.ToLower(value) == "false" || value == "0" {
			return false, nil
		}
		return nil, fmt.Errorf("无效的布尔值: %s", value)
	}

	if strings.Contains(field, "processes") ||
		strings.Contains(field, "keywords") ||
		strings.Contains(field, "commands") ||
		strings.Contains(field, "namespaces") ||
		strings.Contains(field, "podNames") {
		// 切片类型，用逗号分隔
		if value == "" {
			return []string{}, nil
		}
		return strings.Split(value, ","), nil
	}

	if strings.Contains(field, "data") {
		// map类型，简化处理：key1=value1,key2=value2
		return e.parseMap(value)
	}

	// 默认为字符串
	return value, nil
}

// parseMap 解析map类型
func (e *EnvLoader) parseMap(value string) (map[string]string, error) {
	if value == "" {
		return make(map[string]string), nil
	}

	result := make(map[string]string)
	pairs := strings.Split(value, ",")

	for _, pair := range pairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("无效的map格式: %s", pair)
		}
		result[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
	}

	return result, nil
}

// setFieldValue 设置字段值
func (e *EnvLoader) setFieldValue(target interface{}, value interface{}) error {
	targetValue := reflect.ValueOf(target)
	if targetValue.Kind() != reflect.Ptr {
		return fmt.Errorf("target must be a pointer")
	}

	targetValue = targetValue.Elem()
	if !targetValue.CanSet() {
		return fmt.Errorf("cannot set field value")
	}

	valueType := reflect.ValueOf(value)
	if !valueType.Type().ConvertibleTo(targetValue.Type()) {
		return fmt.Errorf("cannot convert %v to %v", valueType.Type(), targetValue.Type())
	}

	targetValue.Set(valueType.Convert(targetValue.Type()))
	return nil
}

// ListEnvVars 列出所有相关的环境变量
func (e *EnvLoader) ListEnvVars() []string {
	var envVars []string

	// 收集所有环境变量
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, e.prefix+"_") {
			envVars = append(envVars, env)
		}
	}

	return envVars
}

// GetEnvSummary 获取环境变量摘要
func (e *EnvLoader) GetEnvSummary() map[string]interface{} {
	summary := make(map[string]interface{})

	envVars := e.ListEnvVars()
	summary["total_env_vars"] = len(envVars)
	summary["env_vars"] = envVars
	summary["prefix"] = e.prefix
	summary["separator"] = e.separator

	return summary
}

// 具体的类型转换器实现

// StringConverter 字符串转换器
type StringConverter struct{}

func (c *StringConverter) Convert(value string) (interface{}, error) {
	return value, nil
}

// BoolConverter 布尔值转换器
type BoolConverter struct{}

func (c *BoolConverter) Convert(value string) (interface{}, error) {
	switch strings.ToLower(value) {
	case "true", "1", "yes", "on", "enabled":
		return true, nil
	case "false", "0", "no", "off", "disabled":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean value: %s", value)
	}
}

// DurationConverter 时间间隔转换器
type DurationConverter struct{}

func (c *DurationConverter) Convert(value string) (interface{}, error) {
	return time.ParseDuration(value)
}

// IntConverter 整数转换器
type IntConverter struct{}

func (c *IntConverter) Convert(value string) (interface{}, error) {
	return strconv.Atoi(value)
}

// StringSliceConverter 字符串切片转换器
type StringSliceConverter struct {
	Separator string
}

func (c *StringSliceConverter) Convert(value string) (interface{}, error) {
	separator := c.Separator
	if separator == "" {
		separator = ","
	}

	if value == "" {
		return []string{}, nil
	}

	return strings.Split(value, separator), nil
}
