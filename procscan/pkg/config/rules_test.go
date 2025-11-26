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
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestConfigRules(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Config Rules Suite")
}

var _ = Describe("ValidationRules", func() {
	Describe("StringRule", func() {
		It("should validate valid string", func() {
			rule := &StringRule{MinLength: 2, MaxLength: 10}
			err := rule.Validate("hello")
			Expect(err).To(BeNil())
		})

		It("should reject string shorter than minimum", func() {
			rule := &StringRule{MinLength: 5}
			err := rule.Validate("abc")
			Expect(err).NotTo(BeNil())
			Expect(err.Code).To(Equal("MIN_LENGTH"))
		})

		It("should reject string longer than maximum", func() {
			rule := &StringRule{MaxLength: 5}
			err := rule.Validate("verylongstring")
			Expect(err).NotTo(BeNil())
			Expect(err.Code).To(Equal("MAX_LENGTH"))
		})

		It("should reject empty string when required", func() {
			rule := &StringRule{Required: true}
			err := rule.Validate("")
			Expect(err).NotTo(BeNil())
			Expect(err.Code).To(Equal("REQUIRED"))
		})

		It("should allow empty string when not required", func() {
			rule := &StringRule{Required: false}
			err := rule.Validate("")
			Expect(err).To(BeNil())
		})

		It("should validate pattern matching", func() {
			rule := &StringRule{Pattern: regexp.MustCompile("^[a-z]+$")}
			Expect(rule.Validate("abc")).To(BeNil())
			err := rule.Validate("ABC123")
			Expect(err).NotTo(BeNil())
			Expect(err.Code).To(Equal("PATTERN_MISMATCH"))
		})

		It("should reject non-string types", func() {
			rule := &StringRule{}
			err := rule.Validate(123)
			Expect(err).NotTo(BeNil())
			Expect(err.Code).To(Equal("INVALID_TYPE"))
		})
	})

	Describe("BooleanRule", func() {
		It("should validate true", func() {
			rule := &BooleanRule{}
			Expect(rule.Validate(true)).To(BeNil())
		})

		It("should validate false", func() {
			rule := &BooleanRule{}
			Expect(rule.Validate(false)).To(BeNil())
		})

		It("should reject non-boolean types", func() {
			rule := &BooleanRule{}
			err := rule.Validate("true")
			Expect(err).NotTo(BeNil())
			Expect(err.Code).To(Equal("INVALID_TYPE"))
		})
	})

	Describe("DurationRule", func() {
		It("should validate duration within range", func() {
			rule := &DurationRule{Min: 1 * time.Second, Max: 10 * time.Second}
			err := rule.Validate(5 * time.Second)
			Expect(err).To(BeNil())
		})

		It("should reject duration shorter than minimum", func() {
			rule := &DurationRule{Min: 5 * time.Second}
			err := rule.Validate(2 * time.Second)
			Expect(err).NotTo(BeNil())
			Expect(err.Code).To(Equal("MIN_DURATION"))
		})

		It("should reject duration longer than maximum", func() {
			rule := &DurationRule{Max: 10 * time.Second}
			err := rule.Validate(20 * time.Second)
			Expect(err).NotTo(BeNil())
			Expect(err.Code).To(Equal("MAX_DURATION"))
		})

		It("should reject non-duration types", func() {
			rule := &DurationRule{}
			err := rule.Validate("10s")
			Expect(err).NotTo(BeNil())
			Expect(err.Code).To(Equal("INVALID_TYPE"))
		})
	})

	Describe("EnumRule", func() {
		It("should validate allowed values (case insensitive)", func() {
			rule := &EnumRule{
				AllowedValues: []string{"debug", "info", "warn", "error"},
				CaseSensitive: false,
			}
			Expect(rule.Validate("info")).To(BeNil())
			Expect(rule.Validate("INFO")).To(BeNil())
			Expect(rule.Validate("Info")).To(BeNil())
		})

		It("should validate allowed values (case sensitive)", func() {
			rule := &EnumRule{
				AllowedValues: []string{"Debug", "Info"},
				CaseSensitive: true,
			}
			Expect(rule.Validate("Info")).To(BeNil())
			err := rule.Validate("info")
			Expect(err).NotTo(BeNil())
			Expect(err.Code).To(Equal("INVALID_ENUM"))
		})

		It("should reject disallowed values", func() {
			rule := &EnumRule{AllowedValues: []string{"a", "b", "c"}}
			err := rule.Validate("d")
			Expect(err).NotTo(BeNil())
			Expect(err.Code).To(Equal("INVALID_ENUM"))
		})
	})

	Describe("URLRule", func() {
		It("should validate correct HTTP URL", func() {
			rule := &URLRule{RequiredSchemes: []string{"http", "https"}}
			Expect(rule.Validate("https://example.com")).To(BeNil())
			Expect(rule.Validate("http://localhost:8080/webhook")).To(BeNil())
		})

		It("should reject invalid URL format", func() {
			rule := &URLRule{}
			err := rule.Validate("://invalid")
			Expect(err).NotTo(BeNil())
			Expect(err.Code).To(Equal("INVALID_URL"))
		})

		It("should reject disallowed scheme", func() {
			rule := &URLRule{RequiredSchemes: []string{"https"}}
			err := rule.Validate("http://example.com")
			Expect(err).NotTo(BeNil())
			Expect(err.Code).To(Equal("INVALID_SCHEME"))
		})

		It("should allow empty URL when AllowEmpty is true", func() {
			rule := &URLRule{AllowEmpty: true}
			Expect(rule.Validate("")).To(BeNil())
		})

		It("should reject empty URL when AllowEmpty is false", func() {
			rule := &URLRule{AllowEmpty: false}
			err := rule.Validate("")
			Expect(err).NotTo(BeNil())
			Expect(err.Code).To(Equal("REQUIRED"))
		})
	})

	Describe("PathRule", func() {
		var tmpDir string

		BeforeEach(func() {
			var err error
			tmpDir, err = os.MkdirTemp("", "path-test-*")
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			os.RemoveAll(tmpDir)
		})

		It("should validate absolute path", func() {
			rule := &PathRule{}
			Expect(rule.Validate("/usr/bin")).To(BeNil())
		})

		It("should reject relative path", func() {
			rule := &PathRule{}
			err := rule.Validate("relative/path")
			Expect(err).NotTo(BeNil())
			Expect(err.Code).To(Equal("INVALID_PATH"))
		})

		It("should allow empty path", func() {
			rule := &PathRule{}
			Expect(rule.Validate("")).To(BeNil())
		})

		It("should validate existing path when MustExist is true", func() {
			rule := &PathRule{MustExist: true, IsDir: true}
			Expect(rule.Validate(tmpDir)).To(BeNil())
		})

		It("should reject non-existent path when MustExist is true", func() {
			rule := &PathRule{MustExist: true}
			err := rule.Validate("/non/existent/path")
			Expect(err).NotTo(BeNil())
			Expect(err.Code).To(Equal("PATH_NOT_EXIST"))
		})

		It("should validate directory when IsDir is true", func() {
			rule := &PathRule{MustExist: true, IsDir: true}
			Expect(rule.Validate(tmpDir)).To(BeNil())
		})

		It("should reject file when IsDir is true", func() {
			file := filepath.Join(tmpDir, "test.txt")
			os.WriteFile(file, []byte("test"), 0644)

			rule := &PathRule{MustExist: true, IsDir: true}
			err := rule.Validate(file)
			Expect(err).NotTo(BeNil())
			Expect(err.Code).To(Equal("NOT_DIRECTORY"))
		})
	})

	Describe("RegexRule", func() {
		It("should validate correct regex pattern", func() {
			rule := &RegexRule{}
			Expect(rule.Validate("^test.*$")).To(BeNil())
			Expect(rule.Validate("[a-z]+")).To(BeNil())
		})

		It("should reject invalid regex pattern", func() {
			rule := &RegexRule{}
			err := rule.Validate("[invalid")
			Expect(err).NotTo(BeNil())
			Expect(err.Code).To(Equal("INVALID_REGEX"))
		})

		It("should allow empty string", func() {
			rule := &RegexRule{}
			Expect(rule.Validate("")).To(BeNil())
		})

		It("should reject non-string types", func() {
			rule := &RegexRule{}
			err := rule.Validate(123)
			Expect(err).NotTo(BeNil())
			Expect(err.Code).To(Equal("INVALID_TYPE"))
		})
	})

	Describe("SliceRule", func() {
		It("should validate valid slice", func() {
			rule := &SliceRule{
				ElementRule: &StringRule{MaxLength: 10},
				AllowEmpty:  false,
			}
			err := rule.Validate([]string{"a", "b", "c"})
			Expect(err).To(BeNil())
		})

		It("should reject empty slice when not allowed", func() {
			rule := &SliceRule{AllowEmpty: false}
			err := rule.Validate([]string{})
			Expect(err).NotTo(BeNil())
			Expect(err.Code).To(Equal("REQUIRED"))
		})

		It("should allow empty slice when allowed", func() {
			rule := &SliceRule{AllowEmpty: true}
			Expect(rule.Validate([]string{})).To(BeNil())
		})

		It("should validate minimum length", func() {
			rule := &SliceRule{MinLength: 3}
			err := rule.Validate([]string{"a", "b"})
			Expect(err).NotTo(BeNil())
			Expect(err.Code).To(Equal("MIN_LENGTH"))
		})

		It("should validate maximum length", func() {
			rule := &SliceRule{MaxLength: 2}
			err := rule.Validate([]string{"a", "b", "c"})
			Expect(err).NotTo(BeNil())
			Expect(err.Code).To(Equal("MAX_LENGTH"))
		})

		It("should validate each element with ElementRule", func() {
			rule := &SliceRule{
				ElementRule: &StringRule{MaxLength: 5},
				AllowEmpty:  false,
			}
			err := rule.Validate([]string{"ok", "verylongstring"})
			Expect(err).NotTo(BeNil())
			Expect(err.Code).To(Equal("MAX_LENGTH"))
		})

		It("should reject non-slice types", func() {
			rule := &SliceRule{}
			err := rule.Validate("not a slice")
			Expect(err).NotTo(BeNil())
			Expect(err.Code).To(Equal("INVALID_TYPE"))
		})
	})

	Describe("PortRule", func() {
		It("should validate valid port number", func() {
			rule := &PortRule{}
			Expect(rule.Validate(8080)).To(BeNil())
			Expect(rule.Validate("3000")).To(BeNil())
		})

		It("should reject port out of range", func() {
			rule := &PortRule{}
			err := rule.Validate(0)
			Expect(err).NotTo(BeNil())
			Expect(err.Code).To(Equal("INVALID_PORT_RANGE"))

			err = rule.Validate(70000)
			Expect(err).NotTo(BeNil())
			Expect(err.Code).To(Equal("INVALID_PORT_RANGE"))
		})

		It("should validate custom port range", func() {
			rule := &PortRule{MinPort: 8000, MaxPort: 9000}
			Expect(rule.Validate(8080)).To(BeNil())

			err := rule.Validate(7000)
			Expect(err).NotTo(BeNil())
			Expect(err.Code).To(Equal("MIN_PORT"))

			err = rule.Validate(10000)
			Expect(err).NotTo(BeNil())
			Expect(err.Code).To(Equal("MAX_PORT"))
		})

		It("should reject non-numeric string", func() {
			rule := &PortRule{}
			err := rule.Validate("abc")
			Expect(err).NotTo(BeNil())
			Expect(err.Code).To(Equal("INVALID_PORT"))
		})

		It("should reject invalid types", func() {
			rule := &PortRule{}
			err := rule.Validate([]int{8080})
			Expect(err).NotTo(BeNil())
			Expect(err.Code).To(Equal("INVALID_TYPE"))
		})
	})

	Describe("NumberRule", func() {
		It("should validate valid number", func() {
			rule := &NumberRule{}
			Expect(rule.Validate(42)).To(BeNil())
			Expect(rule.Validate("100")).To(BeNil())
		})

		It("should validate minimum value", func() {
			min := 10
			rule := &NumberRule{Min: &min}
			Expect(rule.Validate(15)).To(BeNil())

			err := rule.Validate(5)
			Expect(err).NotTo(BeNil())
			Expect(err.Code).To(Equal("MIN_VALUE"))
		})

		It("should validate maximum value", func() {
			max := 100
			rule := &NumberRule{Max: &max}
			Expect(rule.Validate(50)).To(BeNil())

			err := rule.Validate(150)
			Expect(err).NotTo(BeNil())
			Expect(err.Code).To(Equal("MAX_VALUE"))
		})

		It("should reject non-numeric string", func() {
			rule := &NumberRule{}
			err := rule.Validate("not a number")
			Expect(err).NotTo(BeNil())
			Expect(err.Code).To(Equal("INVALID_NUMBER"))
		})

		It("should reject invalid types", func() {
			rule := &NumberRule{}
			err := rule.Validate([]int{42})
			Expect(err).NotTo(BeNil())
			Expect(err.Code).To(Equal("INVALID_TYPE"))
		})
	})
})
