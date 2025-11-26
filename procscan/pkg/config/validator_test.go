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

package config

import (
	"time"

	"github.com/bearslyricattack/CompliK/procscan/pkg/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ConfigValidator", func() {
	var validator *ConfigValidator

	BeforeEach(func() {
		validator = NewConfigValidator()
	})

	Describe("NewConfigValidator", func() {
		It("should create validator with default rules", func() {
			Expect(validator).NotTo(BeNil())
			Expect(validator.rules).NotTo(BeEmpty())
			Expect(validator.GetFieldRules("scanner.scan_interval")).NotTo(BeEmpty())
			Expect(validator.GetFieldRules("scanner.log_level")).NotTo(BeEmpty())
		})
	})

	Describe("AddRule", func() {
		It("should add custom rule", func() {
			customRule := &StringRule{MinLength: 5}
			validator.AddRule("custom.field", customRule)

			rules := validator.GetFieldRules("custom.field")
			Expect(rules).To(HaveLen(1))
		})

		It("should append to existing rules", func() {
			rule1 := &StringRule{MinLength: 5}
			rule2 := &StringRule{MaxLength: 20}

			validator.AddRule("test.field", rule1)
			validator.AddRule("test.field", rule2)

			rules := validator.GetFieldRules("test.field")
			Expect(rules).To(HaveLen(2))
		})
	})

	Describe("Validate", func() {
		It("should validate valid configuration", func() {
			config := &models.Config{
				Scanner: models.ScannerConfig{
					ProcPath:     "/proc",
					ScanInterval: 60 * time.Second,
					LogLevel:     "info",
				},
				Actions: models.ActionsConfig{
					Label: models.LabelActionConfig{
						Enabled: false,
					},
				},
				Notifications: models.NotificationsConfig{
					Lark: models.LarkNotificationConfig{
						Webhook: "https://open.feishu.cn/webhook",
					},
				},
				DetectionRules: models.DetectionRules{
					Blacklist: models.RuleSet{
						Processes: []string{"minerd", "xmrig"},
						Keywords:  []string{"stratum\\+tcp://"},
					},
					Whitelist: models.RuleSet{
						Namespaces: []string{"kube-system"},
					},
				},
			}

			result := validator.Validate(config)
			Expect(result.Valid).To(BeTrue())
			Expect(result.Errors).To(BeEmpty())
		})

		It("should detect invalid scan interval", func() {
			config := &models.Config{
				Scanner: models.ScannerConfig{
					ScanInterval: 100 * time.Millisecond, // Too short
					LogLevel:     "info",
				},
			}

			result := validator.Validate(config)
			Expect(result.Valid).To(BeFalse())
			Expect(result.Errors).NotTo(BeEmpty())
			Expect(result.Errors[0]).To(ContainSubstring("scan_interval"))
		})

		It("should detect invalid log level", func() {
			config := &models.Config{
				Scanner: models.ScannerConfig{
					ScanInterval: 60 * time.Second,
					LogLevel:     "invalid_level",
				},
			}

			result := validator.Validate(config)
			Expect(result.Valid).To(BeFalse())
			Expect(result.Errors).NotTo(BeEmpty())
		})

		It("should detect invalid regex in blacklist", func() {
			config := &models.Config{
				Scanner: models.ScannerConfig{
					ScanInterval: 60 * time.Second,
					LogLevel:     "info",
				},
				DetectionRules: models.DetectionRules{
					Blacklist: models.RuleSet{
						Processes: []string{"[invalid"},
					},
				},
			}

			result := validator.Validate(config)
			Expect(result.Valid).To(BeFalse())
			Expect(result.Errors).NotTo(BeEmpty())
		})

		It("should warn when webhook is empty", func() {
			config := &models.Config{
				Scanner: models.ScannerConfig{
					ProcPath:     "/proc",
					ScanInterval: 60 * time.Second,
					LogLevel:     "info",
				},
				Notifications: models.NotificationsConfig{
					Lark: models.LarkNotificationConfig{
						Webhook: "",
					},
				},
			}

			result := validator.Validate(config)
			Expect(result.Warnings).NotTo(BeEmpty())
			found := false
			for _, warning := range result.Warnings {
				if containsSubstring(warning, "Notification") {
					found = true
					break
				}
			}
			Expect(found).To(BeTrue())
		})

		It("should warn when blacklist is empty", func() {
			config := &models.Config{
				Scanner: models.ScannerConfig{
					ScanInterval: 60 * time.Second,
					LogLevel:     "info",
				},
				DetectionRules: models.DetectionRules{
					Blacklist: models.RuleSet{
						Processes: []string{},
						Keywords:  []string{},
					},
				},
			}

			result := validator.Validate(config)
			Expect(result.Warnings).NotTo(BeEmpty())
		})

		It("should warn about short scan interval", func() {
			config := &models.Config{
				Scanner: models.ScannerConfig{
					ScanInterval: 5 * time.Second,
					LogLevel:     "info",
				},
			}

			result := validator.Validate(config)
			Expect(result.Warnings).NotTo(BeEmpty())
			found := false
			for _, warning := range result.Warnings {
				if containsSubstring(warning, "too short") {
					found = true
					break
				}
			}
			Expect(found).To(BeTrue())
		})

		It("should warn about rule conflicts", func() {
			config := &models.Config{
				Scanner: models.ScannerConfig{
					ScanInterval: 60 * time.Second,
					LogLevel:     "info",
				},
				DetectionRules: models.DetectionRules{
					Blacklist: models.RuleSet{
						Processes: []string{"python3"},
					},
					Whitelist: models.RuleSet{
						Processes: []string{"python3"},
					},
				},
			}

			result := validator.Validate(config)
			Expect(result.Warnings).NotTo(BeEmpty())
			found := false
			for _, warning := range result.Warnings {
				if containsSubstring(warning, "both blacklist and whitelist") {
					found = true
					break
				}
			}
			Expect(found).To(BeTrue())
		})
	})

	Describe("ValidateFile", func() {
		It("should accept .yaml extension", func() {
			result := validator.ValidateFile("/path/to/config.yaml")
			Expect(result.Valid).To(BeTrue())
			Expect(result.Warnings).To(BeEmpty())
		})

		It("should accept .yml extension", func() {
			result := validator.ValidateFile("/path/to/config.yml")
			Expect(result.Valid).To(BeTrue())
			Expect(result.Warnings).To(BeEmpty())
		})

		It("should warn about non-yaml extension", func() {
			result := validator.ValidateFile("/path/to/config.json")
			Expect(result.Warnings).NotTo(BeEmpty())
			Expect(result.Warnings[0]).To(ContainSubstring(".yaml or .yml"))
		})
	})

	Describe("validateScanner", func() {
		It("should warn when proc_path is empty", func() {
			config := &models.Config{
				Scanner: models.ScannerConfig{
					ProcPath:     "",
					ScanInterval: 60 * time.Second,
					LogLevel:     "info",
				},
			}

			result := validator.Validate(config)
			found := false
			for _, warning := range result.Warnings {
				if containsSubstring(warning, "proc_path") {
					found = true
					break
				}
			}
			Expect(found).To(BeTrue())
		})
	})

	Describe("validateActions", func() {
		It("should warn when label is enabled", func() {
			config := &models.Config{
				Scanner: models.ScannerConfig{
					ScanInterval: 60 * time.Second,
					LogLevel:     "info",
				},
				Actions: models.ActionsConfig{
					Label: models.LabelActionConfig{
						Enabled: true,
					},
				},
			}

			result := validator.Validate(config)
			Expect(result.Warnings).NotTo(BeEmpty())
			found := false
			for _, warning := range result.Warnings {
				if containsSubstring(warning, "Label functionality") {
					found = true
					break
				}
			}
			Expect(found).To(BeTrue())
		})
	})

	Describe("ListAllRules", func() {
		It("should return all registered rules", func() {
			allRules := validator.ListAllRules()
			Expect(allRules).NotTo(BeEmpty())
			Expect(allRules).To(HaveKey("scanner.scan_interval"))
			Expect(allRules).To(HaveKey("scanner.log_level"))
			Expect(allRules).To(HaveKey("notifications.lark.webhook"))
		})
	})
})

// Helper function to check if a string contains a substring
func containsSubstring(str, substr string) bool {
	return len(str) >= len(substr) && findSubstring(str, substr)
}

func findSubstring(str, substr string) bool {
	for i := 0; i <= len(str)-len(substr); i++ {
		if str[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
