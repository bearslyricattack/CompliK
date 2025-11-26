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

// Package config provides validation rules for configuration fields.
package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// StringRule validates string values
type StringRule struct {
	MinLength int
	MaxLength int
	Required  bool
	Pattern   *regexp.Regexp
}

func (r *StringRule) Validate(value interface{}) *ValidationError {
	str, ok := value.(string)
	if !ok {
		return &ValidationError{
			Code:    "INVALID_TYPE",
			Message: "Value must be a string type",
			Value:   value,
		}
	}

	if r.Required && str == "" {
		return &ValidationError{
			Code:    "REQUIRED",
			Message: "Field cannot be empty",
			Value:   value,
		}
	}

	if str == "" && !r.Required {
		return nil // Empty value and not required, consider valid
	}

	length := len(str)
	if r.MinLength > 0 && length < r.MinLength {
		return &ValidationError{
			Code:    "MIN_LENGTH",
			Message: fmt.Sprintf("String length cannot be less than %d", r.MinLength),
			Value:   value,
		}
	}

	if r.MaxLength > 0 && length > r.MaxLength {
		return &ValidationError{
			Code:    "MAX_LENGTH",
			Message: fmt.Sprintf("String length cannot be greater than %d", r.MaxLength),
			Value:   value,
		}
	}

	if r.Pattern != nil && !r.Pattern.MatchString(str) {
		return &ValidationError{
			Code:    "PATTERN_MISMATCH",
			Message: "String format is incorrect",
			Value:   value,
		}
	}

	return nil
}

// BooleanRule validates boolean values
type BooleanRule struct{}

func (r *BooleanRule) Validate(value interface{}) *ValidationError {
	_, ok := value.(bool)
	if !ok {
		return &ValidationError{
			Code:    "INVALID_TYPE",
			Message: "Value must be a boolean type",
			Value:   value,
		}
	}
	return nil
}

// DurationRule validates duration values
type DurationRule struct {
	Min time.Duration
	Max time.Duration
}

func (r *DurationRule) Validate(value interface{}) *ValidationError {
	duration, ok := value.(time.Duration)
	if !ok {
		return &ValidationError{
			Code:    "INVALID_TYPE",
			Message: "Value must be a duration type",
			Value:   value,
		}
	}

	if r.Min > 0 && duration < r.Min {
		return &ValidationError{
			Code:    "MIN_DURATION",
			Message: fmt.Sprintf("Duration cannot be less than %v", r.Min),
			Value:   value,
		}
	}

	if r.Max > 0 && duration > r.Max {
		return &ValidationError{
			Code:    "MAX_DURATION",
			Message: fmt.Sprintf("Duration cannot be greater than %v", r.Max),
			Value:   value,
		}
	}

	return nil
}

// EnumRule validates enum values
type EnumRule struct {
	AllowedValues []string
	CaseSensitive bool
}

func (r *EnumRule) Validate(value interface{}) *ValidationError {
	str, ok := value.(string)
	if !ok {
		return &ValidationError{
			Code:    "INVALID_TYPE",
			Message: "Value must be a string type",
			Value:   value,
		}
	}

	for _, allowed := range r.AllowedValues {
		if r.CaseSensitive {
			if str == allowed {
				return nil
			}
		} else {
			if strings.ToLower(str) == strings.ToLower(allowed) {
				return nil
			}
		}
	}

	return &ValidationError{
		Code:    "INVALID_ENUM",
		Message: fmt.Sprintf("Value must be one of: %v", r.AllowedValues),
		Value:   value,
	}
}

// URLRule validates URL values
type URLRule struct {
	RequiredSchemes []string
	AllowEmpty      bool
}

func (r *URLRule) Validate(value interface{}) *ValidationError {
	str, ok := value.(string)
	if !ok {
		return &ValidationError{
			Code:    "INVALID_TYPE",
			Message: "Value must be a string type",
			Value:   value,
		}
	}

	if str == "" {
		if r.AllowEmpty {
			return nil
		}
		return &ValidationError{
			Code:    "REQUIRED",
			Message: "URL cannot be empty",
			Value:   value,
		}
	}

	parsedURL, err := url.Parse(str)
	if err != nil {
		return &ValidationError{
			Code:    "INVALID_URL",
			Message: "URL format is incorrect",
			Value:   value,
		}
	}

	if len(r.RequiredSchemes) > 0 {
		schemeValid := false
		for _, scheme := range r.RequiredSchemes {
			if parsedURL.Scheme == scheme {
				schemeValid = true
				break
			}
		}
		if !schemeValid {
			return &ValidationError{
				Code:    "INVALID_SCHEME",
				Message: fmt.Sprintf("URL scheme must be one of: %v", r.RequiredSchemes),
				Value:   value,
			}
		}
	}

	return nil
}

// PathRule validates path values
type PathRule struct {
	MustExist bool
	IsDir     bool
}

func (r *PathRule) Validate(value interface{}) *ValidationError {
	str, ok := value.(string)
	if !ok {
		return &ValidationError{
			Code:    "INVALID_TYPE",
			Message: "Value must be a string type",
			Value:   value,
		}
	}

	if str == "" {
		return nil // Empty path is considered valid (use default value)
	}

	// Check path format
	if !filepath.IsAbs(str) {
		return &ValidationError{
			Code:    "INVALID_PATH",
			Message: "Path must be an absolute path",
			Value:   value,
		}
	}

	if r.MustExist {
		info, err := os.Stat(str)
		if os.IsNotExist(err) {
			return &ValidationError{
				Code:    "PATH_NOT_EXIST",
				Message: "Path does not exist",
				Value:   value,
			}
		}
		if err != nil {
			return &ValidationError{
				Code:    "ACCESS_ERROR",
				Message: "Cannot access path",
				Value:   value,
			}
		}

		if r.IsDir && !info.IsDir() {
			return &ValidationError{
				Code:    "NOT_DIRECTORY",
				Message: "Path must be a directory",
				Value:   value,
			}
		}
	}

	return nil
}

// RegexRule validates regular expression values
type RegexRule struct{}

func (r *RegexRule) Validate(value interface{}) *ValidationError {
	str, ok := value.(string)
	if !ok {
		return &ValidationError{
			Code:    "INVALID_TYPE",
			Message: "Value must be a string type",
			Value:   value,
		}
	}

	if str == "" {
		return nil // Empty string is considered valid
	}

	_, err := regexp.Compile(str)
	if err != nil {
		return &ValidationError{
			Code:    "INVALID_REGEX",
			Message: "Regular expression format is incorrect: " + err.Error(),
			Value:   value,
		}
	}

	return nil
}

// SliceRule validates slice values
type SliceRule struct {
	ElementRule ValidationRule
	MinLength   int
	MaxLength   int
	AllowEmpty  bool
}

func (r *SliceRule) Validate(value interface{}) *ValidationError {
	slice, ok := value.([]string)
	if !ok {
		return &ValidationError{
			Code:    "INVALID_TYPE",
			Message: "Value must be a string array type",
			Value:   value,
		}
	}

	if len(slice) == 0 {
		if !r.AllowEmpty {
			return &ValidationError{
				Code:    "REQUIRED",
				Message: "Array cannot be empty",
				Value:   value,
			}
		}
		return nil
	}

	if r.MinLength > 0 && len(slice) < r.MinLength {
		return &ValidationError{
			Code:    "MIN_LENGTH",
			Message: fmt.Sprintf("Array length cannot be less than %d", r.MinLength),
			Value:   value,
		}
	}

	if r.MaxLength > 0 && len(slice) > r.MaxLength {
		return &ValidationError{
			Code:    "MAX_LENGTH",
			Message: fmt.Sprintf("Array length cannot be greater than %d", r.MaxLength),
			Value:   value,
		}
	}

	if r.ElementRule != nil {
		for i, element := range slice {
			if err := r.ElementRule.Validate(element); err != nil {
				err.Field = fmt.Sprintf("[%d]", i)
				return err
			}
		}
	}

	return nil
}

// PortRule validates port values
type PortRule struct {
	MinPort int
	MaxPort int
}

func (r *PortRule) Validate(value interface{}) *ValidationError {
	var port int

	switch v := value.(type) {
	case int:
		port = v
	case string:
		parsed, err := strconv.Atoi(v)
		if err != nil {
			return &ValidationError{
				Code:    "INVALID_PORT",
				Message: "Port number must be numeric",
				Value:   value,
			}
		}
		port = parsed
	default:
		return &ValidationError{
			Code:    "INVALID_TYPE",
			Message: "Port number must be an integer type",
			Value:   value,
		}
	}

	if port < 1 || port > 65535 {
		return &ValidationError{
			Code:    "INVALID_PORT_RANGE",
			Message: "Port number must be in the range 1-65535",
			Value:   value,
		}
	}

	if r.MinPort > 0 && port < r.MinPort {
		return &ValidationError{
			Code:    "MIN_PORT",
			Message: fmt.Sprintf("Port number cannot be less than %d", r.MinPort),
			Value:   value,
		}
	}

	if r.MaxPort > 0 && port > r.MaxPort {
		return &ValidationError{
			Code:    "MAX_PORT",
			Message: fmt.Sprintf("Port number cannot be greater than %d", r.MaxPort),
			Value:   value,
		}
	}

	return nil
}

// NumberRule validates number values
type NumberRule struct {
	Min      *int
	Max      *int
	Required bool
}

func (r *NumberRule) Validate(value interface{}) *ValidationError {
	var num int

	switch v := value.(type) {
	case int:
		num = v
	case string:
		parsed, err := strconv.Atoi(v)
		if err != nil {
			return &ValidationError{
				Code:    "INVALID_NUMBER",
				Message: "Value must be numeric",
				Value:   value,
			}
		}
		num = parsed
	default:
		return &ValidationError{
			Code:    "INVALID_TYPE",
			Message: "Value must be an integer type",
			Value:   value,
		}
	}

	if r.Min != nil && num < *r.Min {
		return &ValidationError{
			Code:    "MIN_VALUE",
			Message: fmt.Sprintf("Value cannot be less than %d", *r.Min),
			Value:   value,
		}
	}

	if r.Max != nil && num > *r.Max {
		return &ValidationError{
			Code:    "MAX_VALUE",
			Message: fmt.Sprintf("Value cannot be greater than %d", *r.Max),
			Value:   value,
		}
	}

	return nil
}
