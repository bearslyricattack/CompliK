package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/bearslyricattack/CompliK/procscan/pkg/models"
)

// ValidationResult 验证结果
type ValidationResult struct {
	Valid    bool     // 是否有效
	Errors   []string // 错误列表
	Warnings []string // 警告列表
}

// ConfigValidator 配置验证器
type ConfigValidator struct {
	rules map[string][]ValidationRule
}

// ValidationRule 验证规则接口
type ValidationRule interface {
	Validate(value interface{}) *ValidationError
}

// ValidationError 验证错误
type ValidationError struct {
	Field   string      // 字段名
	Value   interface{} // 字段值
	Message string      // 错误信息
	Code    string      // 错误代码
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("字段 '%s' 验证失败: %s (值: %v)", e.Field, e.Message, e.Value)
}

// NewConfigValidator 创建新的配置验证器
func NewConfigValidator() *ConfigValidator {
	validator := &ConfigValidator{
		rules: make(map[string][]ValidationRule),
	}

	// 注册默认验证规则
	validator.registerDefaultRules()
	return validator
}

// registerDefaultRules 注册默认验证规则
func (v *ConfigValidator) registerDefaultRules() {
	// 扫描器配置规则
	v.AddRule("scanner.scan_interval", &DurationRule{
		Min: 1 * time.Second,
		Max: 1 * time.Hour,
	})
	v.AddRule("scanner.log_level", &EnumRule{
		AllowedValues: []string{"debug", "info", "warn", "error", "fatal"},
	})
	v.AddRule("scanner.proc_path", &PathRule{})

	// 动作配置规则
	v.AddRule("actions.label.enabled", &BooleanRule{})

	// 通知配置规则
	v.AddRule("notifications.lark.webhook", &URLRule{
		RequiredSchemes: []string{"https", "http"},
		AllowEmpty:      true,
	})

	// 检测规则配置规则
	v.AddRule("detectionRules.blacklist.processes", &SliceRule{
		ElementRule: &RegexRule{},
		AllowEmpty:  true,
	})
	v.AddRule("detectionRules.blacklist.keywords", &SliceRule{
		ElementRule: &StringRule{MaxLength: 100},
		AllowEmpty:  true,
	})
	v.AddRule("detectionRules.blacklist.commands", &SliceRule{
		ElementRule: &StringRule{MaxLength: 500},
		AllowEmpty:  true,
	})
	v.AddRule("detectionRules.blacklist.namespaces", &SliceRule{
		ElementRule: &RegexRule{},
		AllowEmpty:  true,
	})
	v.AddRule("detectionRules.blacklist.podNames", &SliceRule{
		ElementRule: &RegexRule{},
		AllowEmpty:  true,
	})
}

// AddRule 添加验证规则
func (v *ConfigValidator) AddRule(field string, rule ValidationRule) {
	if v.rules[field] == nil {
		v.rules[field] = make([]ValidationRule, 0)
	}
	v.rules[field] = append(v.rules[field], rule)
}

// Validate 验证配置
func (v *ConfigValidator) Validate(config *models.Config) *ValidationResult {
	result := &ValidationResult{
		Valid:    true,
		Errors:   make([]string, 0),
		Warnings: make([]string, 0),
	}

	// 验证扫描器配置
	v.validateScanner(config.Scanner, result)

	// 验证动作配置
	v.validateActions(config.Actions, result)

	// 验证通知配置
	v.validateNotifications(config.Notifications, result)

	// 验证检测规则
	v.validateDetectionRules(config.DetectionRules, result)

	// 跨字段验证
	v.validateCrossFields(config, result)

	if len(result.Errors) > 0 {
		result.Valid = false
	}

	return result
}

// validateScanner 验证扫描器配置
func (v *ConfigValidator) validateScanner(scanner models.ScannerConfig, result *ValidationResult) {
	// 验证扫描间隔
	if err := v.validateField("scanner.scan_interval", scanner.ScanInterval); err != nil {
		result.Errors = append(result.Errors, err.Error())
	}

	// 验证日志级别
	if err := v.validateField("scanner.log_level", scanner.LogLevel); err != nil {
		result.Errors = append(result.Errors, err.Error())
	}

	// 验证proc路径
	if err := v.validateField("scanner.proc_path", scanner.ProcPath); err != nil {
		result.Errors = append(result.Errors, err.Error())
	}

	// 检查proc路径是否存在
	if scanner.ProcPath == "" {
		result.Warnings = append(result.Warnings, "scanner.proc_path 为空，将使用默认值 /host/proc")
	}
}

// validateActions 验证动作配置
func (v *ConfigValidator) validateActions(actions models.ActionsConfig, result *ValidationResult) {
	// 验证标签动作
	if err := v.validateField("actions.label.enabled", actions.Label.Enabled); err != nil {
		result.Errors = append(result.Errors, err.Error())
	}

	// 安全检查：标签功能正常工作时，会自动添加安全标签
	if actions.Label.Enabled {
		result.Warnings = append(result.Warnings, "标签功能已启用，检测到的威胁将被标记并等待外部控制器处理")
	}
}

// validateNotifications 验证通知配置
func (v *ConfigValidator) validateNotifications(notifications models.NotificationsConfig, result *ValidationResult) {
	// 验证飞书webhook
	if err := v.validateField("notifications.lark.webhook", notifications.Lark.Webhook); err != nil {
		result.Errors = append(result.Errors, err.Error())
	}

	// 检查通知配置
	if notifications.Lark.Webhook == "" {
		result.Warnings = append(result.Warnings, "未配置通知webhook，将无法发送告警")
	}
}

// validateDetectionRules 验证检测规则
func (v *ConfigValidator) validateDetectionRules(rules models.DetectionRules, result *ValidationResult) {
	// 验证黑名单规则
	v.validateRuleSet("detectionRules.blacklist", rules.Blacklist, result)

	// 验证白名单规则
	v.validateRuleSet("detectionRules.whitelist", rules.Whitelist, result)

	// 检查规则逻辑
	if len(rules.Blacklist.Processes) == 0 && len(rules.Blacklist.Keywords) == 0 {
		result.Warnings = append(result.Warnings, "黑名单规则为空，可能无法检测到可疑进程")
	}
}

// validateRuleSet 验证规则集
func (v *ConfigValidator) validateRuleSet(prefix string, ruleSet models.RuleSet, result *ValidationResult) {
	if err := v.validateField(prefix+".processes", ruleSet.Processes); err != nil {
		result.Errors = append(result.Errors, err.Error())
	}
	if err := v.validateField(prefix+".keywords", ruleSet.Keywords); err != nil {
		result.Errors = append(result.Errors, err.Error())
	}
	if err := v.validateField(prefix+".commands", ruleSet.Commands); err != nil {
		result.Errors = append(result.Errors, err.Error())
	}
	if err := v.validateField(prefix+".namespaces", ruleSet.Namespaces); err != nil {
		result.Errors = append(result.Errors, err.Error())
	}
	if err := v.validateField(prefix+".podNames", ruleSet.PodNames); err != nil {
		result.Errors = append(result.Errors, err.Error())
	}
}

// validateCrossFields 跨字段验证
func (v *ConfigValidator) validateCrossFields(config *models.Config, result *ValidationResult) {
	// 检查扫描间隔是否合理
	if config.Scanner.ScanInterval < 10*time.Second {
		result.Warnings = append(result.Warnings, "扫描间隔过短，可能增加系统负载")
	}

	// 检查动作配置的逻辑
	if config.Actions.Label.Enabled {
		result.Warnings = append(result.Warnings, "检测到的威胁将被标记，请确保外部控制器正在监控这些标签")
	}

	// 检查黑名单和白名单是否有冲突
	v.validateRuleConflicts(config.DetectionRules, result)
}

// validateRuleConflicts 验证规则冲突
func (v *ConfigValidator) validateRuleConflicts(rules models.DetectionRules, result *ValidationResult) {
	// 检查进程名冲突
	blacklistProcesses := make(map[string]bool)
	for _, process := range rules.Blacklist.Processes {
		blacklistProcesses[process] = true
	}

	for _, process := range rules.Whitelist.Processes {
		if blacklistProcesses[process] {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("进程 '%s' 同时出现在黑名单和白名单中", process))
		}
	}
}

// validateField 验证单个字段
func (v *ConfigValidator) validateField(field string, value interface{}) *ValidationError {
	rules, exists := v.rules[field]
	if !exists {
		return nil // 没有验证规则，认为有效
	}

	for _, rule := range rules {
		if err := rule.Validate(value); err != nil {
			return err
		}
	}

	return nil
}

// ValidateFile 验证配置文件
func (v *ConfigValidator) ValidateFile(configPath string) *ValidationResult {
	// 这里可以添加文件级别的验证，比如文件权限、格式等
	result := &ValidationResult{
		Valid:    true,
		Errors:   make([]string, 0),
		Warnings: make([]string, 0),
	}

	// 检查文件扩展名
	if !strings.HasSuffix(configPath, ".yaml") && !strings.HasSuffix(configPath, ".yml") {
		result.Warnings = append(result.Warnings, "配置文件建议使用 .yaml 或 .yml 扩展名")
	}

	return result
}

// GetFieldRules 获取字段的验证规则
func (v *ConfigValidator) GetFieldRules(field string) []ValidationRule {
	return v.rules[field]
}

// ListAllRules 列出所有验证规则
func (v *ConfigValidator) ListAllRules() map[string][]ValidationRule {
	return v.rules
}
