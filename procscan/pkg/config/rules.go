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

// StringRule 字符串验证规则
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
			Message: "值必须是字符串类型",
			Value:   value,
		}
	}

	if r.Required && str == "" {
		return &ValidationError{
			Code:    "REQUIRED",
			Message: "字段不能为空",
			Value:   value,
		}
	}

	if str == "" && !r.Required {
		return nil // 空值且非必需，认为有效
	}

	length := len(str)
	if r.MinLength > 0 && length < r.MinLength {
		return &ValidationError{
			Code:    "MIN_LENGTH",
			Message: fmt.Sprintf("字符串长度不能小于 %d", r.MinLength),
			Value:   value,
		}
	}

	if r.MaxLength > 0 && length > r.MaxLength {
		return &ValidationError{
			Code:    "MAX_LENGTH",
			Message: fmt.Sprintf("字符串长度不能大于 %d", r.MaxLength),
			Value:   value,
		}
	}

	if r.Pattern != nil && !r.Pattern.MatchString(str) {
		return &ValidationError{
			Code:    "PATTERN_MISMATCH",
			Message: "字符串格式不正确",
			Value:   value,
		}
	}

	return nil
}

// BooleanRule 布尔值验证规则
type BooleanRule struct{}

func (r *BooleanRule) Validate(value interface{}) *ValidationError {
	_, ok := value.(bool)
	if !ok {
		return &ValidationError{
			Code:    "INVALID_TYPE",
			Message: "值必须是布尔类型",
			Value:   value,
		}
	}
	return nil
}

// DurationRule 时间间隔验证规则
type DurationRule struct {
	Min time.Duration
	Max time.Duration
}

func (r *DurationRule) Validate(value interface{}) *ValidationError {
	duration, ok := value.(time.Duration)
	if !ok {
		return &ValidationError{
			Code:    "INVALID_TYPE",
			Message: "值必须是时间间隔类型",
			Value:   value,
		}
	}

	if r.Min > 0 && duration < r.Min {
		return &ValidationError{
			Code:    "MIN_DURATION",
			Message: fmt.Sprintf("时间间隔不能小于 %v", r.Min),
			Value:   value,
		}
	}

	if r.Max > 0 && duration > r.Max {
		return &ValidationError{
			Code:    "MAX_DURATION",
			Message: fmt.Sprintf("时间间隔不能大于 %v", r.Max),
			Value:   value,
		}
	}

	return nil
}

// EnumRule 枚举值验证规则
type EnumRule struct {
	AllowedValues []string
	CaseSensitive bool
}

func (r *EnumRule) Validate(value interface{}) *ValidationError {
	str, ok := value.(string)
	if !ok {
		return &ValidationError{
			Code:    "INVALID_TYPE",
			Message: "值必须是字符串类型",
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
		Message: fmt.Sprintf("值必须是以下之一: %v", r.AllowedValues),
		Value:   value,
	}
}

// URLRule URL验证规则
type URLRule struct {
	RequiredSchemes []string
	AllowEmpty      bool
}

func (r *URLRule) Validate(value interface{}) *ValidationError {
	str, ok := value.(string)
	if !ok {
		return &ValidationError{
			Code:    "INVALID_TYPE",
			Message: "值必须是字符串类型",
			Value:   value,
		}
	}

	if str == "" {
		if r.AllowEmpty {
			return nil
		}
		return &ValidationError{
			Code:    "REQUIRED",
			Message: "URL不能为空",
			Value:   value,
		}
	}

	parsedURL, err := url.Parse(str)
	if err != nil {
		return &ValidationError{
			Code:    "INVALID_URL",
			Message: "URL格式不正确",
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
				Message: fmt.Sprintf("URL协议必须是以下之一: %v", r.RequiredSchemes),
				Value:   value,
			}
		}
	}

	return nil
}

// PathRule 路径验证规则
type PathRule struct {
	MustExist bool
	IsDir     bool
}

func (r *PathRule) Validate(value interface{}) *ValidationError {
	str, ok := value.(string)
	if !ok {
		return &ValidationError{
			Code:    "INVALID_TYPE",
			Message: "值必须是字符串类型",
			Value:   value,
		}
	}

	if str == "" {
		return nil // 空路径认为有效（使用默认值）
	}

	// 检查路径格式
	if !filepath.IsAbs(str) {
		return &ValidationError{
			Code:    "INVALID_PATH",
			Message: "路径必须是绝对路径",
			Value:   value,
		}
	}

	if r.MustExist {
		info, err := os.Stat(str)
		if os.IsNotExist(err) {
			return &ValidationError{
				Code:    "PATH_NOT_EXIST",
				Message: "路径不存在",
				Value:   value,
			}
		}
		if err != nil {
			return &ValidationError{
				Code:    "ACCESS_ERROR",
				Message: "无法访问路径",
				Value:   value,
			}
		}

		if r.IsDir && !info.IsDir() {
			return &ValidationError{
				Code:    "NOT_DIRECTORY",
				Message: "路径必须是目录",
				Value:   value,
			}
		}
	}

	return nil
}

// RegexRule 正则表达式验证规则
type RegexRule struct{}

func (r *RegexRule) Validate(value interface{}) *ValidationError {
	str, ok := value.(string)
	if !ok {
		return &ValidationError{
			Code:    "INVALID_TYPE",
			Message: "值必须是字符串类型",
			Value:   value,
		}
	}

	if str == "" {
		return nil // 空字符串认为有效
	}

	_, err := regexp.Compile(str)
	if err != nil {
		return &ValidationError{
			Code:    "INVALID_REGEX",
			Message: "正则表达式格式不正确: " + err.Error(),
			Value:   value,
		}
	}

	return nil
}

// SliceRule 切片验证规则
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
			Message: "值必须是字符串数组类型",
			Value:   value,
		}
	}

	if len(slice) == 0 {
		if !r.AllowEmpty {
			return &ValidationError{
				Code:    "REQUIRED",
				Message: "数组不能为空",
				Value:   value,
			}
		}
		return nil
	}

	if r.MinLength > 0 && len(slice) < r.MinLength {
		return &ValidationError{
			Code:    "MIN_LENGTH",
			Message: fmt.Sprintf("数组长度不能小于 %d", r.MinLength),
			Value:   value,
		}
	}

	if r.MaxLength > 0 && len(slice) > r.MaxLength {
		return &ValidationError{
			Code:    "MAX_LENGTH",
			Message: fmt.Sprintf("数组长度不能大于 %d", r.MaxLength),
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

// PortRule 端口验证规则
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
				Message: "端口号必须是数字",
				Value:   value,
			}
		}
		port = parsed
	default:
		return &ValidationError{
			Code:    "INVALID_TYPE",
			Message: "端口号必须是整数类型",
			Value:   value,
		}
	}

	if port < 1 || port > 65535 {
		return &ValidationError{
			Code:    "INVALID_PORT_RANGE",
			Message: "端口号必须在 1-65535 范围内",
			Value:   value,
		}
	}

	if r.MinPort > 0 && port < r.MinPort {
		return &ValidationError{
			Code:    "MIN_PORT",
			Message: fmt.Sprintf("端口号不能小于 %d", r.MinPort),
			Value:   value,
		}
	}

	if r.MaxPort > 0 && port > r.MaxPort {
		return &ValidationError{
			Code:    "MAX_PORT",
			Message: fmt.Sprintf("端口号不能大于 %d", r.MaxPort),
			Value:   value,
		}
	}

	return nil
}

// NumberRule 数字验证规则
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
				Message: "值必须是数字",
				Value:   value,
			}
		}
		num = parsed
	default:
		return &ValidationError{
			Code:    "INVALID_TYPE",
			Message: "值必须是整数类型",
			Value:   value,
		}
	}

	if r.Min != nil && num < *r.Min {
		return &ValidationError{
			Code:    "MIN_VALUE",
			Message: fmt.Sprintf("值不能小于 %d", *r.Min),
			Value:   value,
		}
	}

	if r.Max != nil && num > *r.Max {
		return &ValidationError{
			Code:    "MAX_VALUE",
			Message: fmt.Sprintf("值不能大于 %d", *r.Max),
			Value:   value,
		}
	}

	return nil
}
