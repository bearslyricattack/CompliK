// Copyright 2025 CompliK Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package config provides configuration validation functionality.
package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/bearslyricattack/CompliK/procscan/pkg/models"
)

// ValidationResult represents the result of validation
type ValidationResult struct {
	Valid    bool     // Whether the validation passed
	Errors   []string // List of errors
	Warnings []string // List of warnings
}

// ConfigValidator validates configuration
type ConfigValidator struct {
	rules map[string][]ValidationRule
}

// ValidationRule is the interface for validation rules
type ValidationRule interface {
	Validate(value interface{}) *ValidationError
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string      // Field name
	Value   interface{} // Field value
	Message string      // Error message
	Code    string      // Error code
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("Field '%s' validation failed: %s (value: %v)", e.Field, e.Message, e.Value)
}

// NewConfigValidator creates a new configuration validator
func NewConfigValidator() *ConfigValidator {
	validator := &ConfigValidator{
		rules: make(map[string][]ValidationRule),
	}

	// Register default validation rules
	validator.registerDefaultRules()
	return validator
}

// registerDefaultRules registers default validation rules
func (v *ConfigValidator) registerDefaultRules() {
	// Scanner configuration rules
	v.AddRule("scanner.scan_interval", &DurationRule{
		Min: 1 * time.Second,
		Max: 1 * time.Hour,
	})
	v.AddRule("scanner.log_level", &EnumRule{
		AllowedValues: []string{"debug", "info", "warn", "error", "fatal"},
	})
	v.AddRule("scanner.proc_path", &PathRule{})

	// Action configuration rules
	v.AddRule("actions.label.enabled", &BooleanRule{})

	// Notification configuration rules
	v.AddRule("notifications.lark.webhook", &URLRule{
		RequiredSchemes: []string{"https", "http"},
		AllowEmpty:      true,
	})

	// Detection rules configuration rules
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

// AddRule adds a validation rule
func (v *ConfigValidator) AddRule(field string, rule ValidationRule) {
	if v.rules[field] == nil {
		v.rules[field] = make([]ValidationRule, 0)
	}
	v.rules[field] = append(v.rules[field], rule)
}

// Validate validates the configuration
func (v *ConfigValidator) Validate(config *models.Config) *ValidationResult {
	result := &ValidationResult{
		Valid:    true,
		Errors:   make([]string, 0),
		Warnings: make([]string, 0),
	}

	// Validate scanner configuration
	v.validateScanner(config.Scanner, result)

	// Validate actions configuration
	v.validateActions(config.Actions, result)

	// Validate notifications configuration
	v.validateNotifications(config.Notifications, result)

	// Validate detection rules
	v.validateDetectionRules(config.DetectionRules, result)

	// Cross-field validation
	v.validateCrossFields(config, result)

	if len(result.Errors) > 0 {
		result.Valid = false
	}

	return result
}

// validateScanner validates scanner configuration
func (v *ConfigValidator) validateScanner(scanner models.ScannerConfig, result *ValidationResult) {
	// Validate scan interval
	if err := v.validateField("scanner.scan_interval", scanner.ScanInterval); err != nil {
		result.Errors = append(result.Errors, err.Error())
	}

	// Validate log level
	if err := v.validateField("scanner.log_level", scanner.LogLevel); err != nil {
		result.Errors = append(result.Errors, err.Error())
	}

	// Validate proc path
	if err := v.validateField("scanner.proc_path", scanner.ProcPath); err != nil {
		result.Errors = append(result.Errors, err.Error())
	}

	// Check if proc path is empty
	if scanner.ProcPath == "" {
		result.Warnings = append(result.Warnings, "scanner.proc_path is empty, will use default value /host/proc")
	}
}

// validateActions validates actions configuration
func (v *ConfigValidator) validateActions(actions models.ActionsConfig, result *ValidationResult) {
	// Validate label action
	if err := v.validateField("actions.label.enabled", actions.Label.Enabled); err != nil {
		result.Errors = append(result.Errors, err.Error())
	}

	// Security check: when label functionality is working normally, security labels will be automatically added
	if actions.Label.Enabled {
		result.Warnings = append(result.Warnings, "Label functionality is enabled, detected threats will be marked and await external controller processing")
	}
}

// validateNotifications validates notifications configuration
func (v *ConfigValidator) validateNotifications(notifications models.NotificationsConfig, result *ValidationResult) {
	// Validate Lark webhook
	if err := v.validateField("notifications.lark.webhook", notifications.Lark.Webhook); err != nil {
		result.Errors = append(result.Errors, err.Error())
	}

	// Check notification configuration
	if notifications.Lark.Webhook == "" {
		result.Warnings = append(result.Warnings, "Notification webhook not configured, alerts cannot be sent")
	}
}

// validateDetectionRules validates detection rules
func (v *ConfigValidator) validateDetectionRules(rules models.DetectionRules, result *ValidationResult) {
	// Validate blacklist rules
	v.validateRuleSet("detectionRules.blacklist", rules.Blacklist, result)

	// Validate whitelist rules
	v.validateRuleSet("detectionRules.whitelist", rules.Whitelist, result)

	// Check rule logic
	if len(rules.Blacklist.Processes) == 0 && len(rules.Blacklist.Keywords) == 0 {
		result.Warnings = append(result.Warnings, "Blacklist rules are empty, may not detect suspicious processes")
	}
}

// validateRuleSet validates a rule set
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

// validateCrossFields performs cross-field validation
func (v *ConfigValidator) validateCrossFields(config *models.Config, result *ValidationResult) {
	// Check if scan interval is reasonable
	if config.Scanner.ScanInterval < 10*time.Second {
		result.Warnings = append(result.Warnings, "Scan interval is too short, may increase system load")
	}

	// Check action configuration logic
	if config.Actions.Label.Enabled {
		result.Warnings = append(result.Warnings, "Detected threats will be marked, please ensure external controller is monitoring these labels")
	}

	// Check for conflicts between blacklist and whitelist
	v.validateRuleConflicts(config.DetectionRules, result)
}

// validateRuleConflicts validates rule conflicts
func (v *ConfigValidator) validateRuleConflicts(rules models.DetectionRules, result *ValidationResult) {
	// Check process name conflicts
	blacklistProcesses := make(map[string]bool)
	for _, process := range rules.Blacklist.Processes {
		blacklistProcesses[process] = true
	}

	for _, process := range rules.Whitelist.Processes {
		if blacklistProcesses[process] {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("Process '%s' appears in both blacklist and whitelist", process))
		}
	}
}

// validateField validates a single field
func (v *ConfigValidator) validateField(field string, value interface{}) *ValidationError {
	rules, exists := v.rules[field]
	if !exists {
		return nil // No validation rules, consider valid
	}

	for _, rule := range rules {
		if err := rule.Validate(value); err != nil {
			return err
		}
	}

	return nil
}

// ValidateFile validates the configuration file
func (v *ConfigValidator) ValidateFile(configPath string) *ValidationResult {
	// This can add file-level validation, such as file permissions, format, etc.
	result := &ValidationResult{
		Valid:    true,
		Errors:   make([]string, 0),
		Warnings: make([]string, 0),
	}

	// Check file extension
	if !strings.HasSuffix(configPath, ".yaml") && !strings.HasSuffix(configPath, ".yml") {
		result.Warnings = append(result.Warnings, "Configuration file should use .yaml or .yml extension")
	}

	return result
}

// GetFieldRules returns the validation rules for a field
func (v *ConfigValidator) GetFieldRules(field string) []ValidationRule {
	return v.rules[field]
}

// ListAllRules lists all validation rules
func (v *ConfigValidator) ListAllRules() map[string][]ValidationRule {
	return v.rules
}
