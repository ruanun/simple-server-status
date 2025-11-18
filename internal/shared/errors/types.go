package errors

import (
	"fmt"
	"time"
)

// ErrorType 错误类型枚举
type ErrorType int

const (
	ErrorTypeNetwork ErrorType = iota
	ErrorTypeSystem
	ErrorTypeConfig
	ErrorTypeData
	ErrorTypeWebSocket
	ErrorTypeValidation
	ErrorTypeAuthentication
	ErrorTypeUnknown
)

// ErrorTypeNames 错误类型名称映射
var ErrorTypeNames = map[ErrorType]string{
	ErrorTypeNetwork:        "网络错误",
	ErrorTypeSystem:         "系统错误",
	ErrorTypeConfig:         "配置错误",
	ErrorTypeData:           "数据错误",
	ErrorTypeWebSocket:      "WebSocket错误",
	ErrorTypeValidation:     "验证错误",
	ErrorTypeAuthentication: "认证错误",
	ErrorTypeUnknown:        "未知错误",
}

// ErrorSeverity 错误严重程度
type ErrorSeverity int

const (
	SeverityLow ErrorSeverity = iota
	SeverityMedium
	SeverityHigh
	SeverityCritical
)

// SeverityNames 严重程度名称映射
var SeverityNames = map[ErrorSeverity]string{
	SeverityLow:      "低",
	SeverityMedium:   "中",
	SeverityHigh:     "高",
	SeverityCritical: "严重",
}

// AppError 应用错误结构
type AppError struct {
	Type      ErrorType     // 错误类型
	Severity  ErrorSeverity // 严重程度
	Code      string        // 错误代码
	Message   string        // 错误消息
	Details   string        // 错误详情
	Cause     error         // 原始错误
	Timestamp time.Time     // 发生时间
	Retryable bool          // 是否可重试
}

// Error 实现 error 接口
func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %s (原因: %v)", e.Code, e.Message, e.Details, e.Cause)
	}
	if e.Details != "" {
		return fmt.Sprintf("[%s] %s: %s", e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// NewAppError 创建新的应用错误
func NewAppError(errType ErrorType, severity ErrorSeverity, code, message string) *AppError {
	return &AppError{
		Type:      errType,
		Severity:  severity,
		Code:      code,
		Message:   message,
		Timestamp: time.Now(),
		Retryable: isRetryable(errType, severity),
	}
}

// WrapError 包装原始错误为应用错误
func WrapError(err error, errType ErrorType, severity ErrorSeverity, code, message string) *AppError {
	if err == nil {
		return nil
	}
	return &AppError{
		Type:      errType,
		Severity:  severity,
		Code:      code,
		Message:   message,
		Cause:     err,
		Timestamp: time.Now(),
		Retryable: isRetryable(errType, severity),
	}
}

// WithDetails 添加错误详情
func (e *AppError) WithDetails(details string) *AppError {
	e.Details = details
	return e
}

// WithCause 添加原始错误
func (e *AppError) WithCause(cause error) *AppError {
	e.Cause = cause
	return e
}

// isRetryable 判断错误是否可重试
func isRetryable(errType ErrorType, severity ErrorSeverity) bool {
	// 配置错误和认证错误通常不可重试
	if errType == ErrorTypeConfig || errType == ErrorTypeAuthentication {
		return false
	}

	// 严重错误不可重试
	if severity == SeverityCritical {
		return false
	}

	// 网络错误和系统错误可以重试
	return errType == ErrorTypeNetwork || errType == ErrorTypeSystem || errType == ErrorTypeWebSocket
}

// IsNetworkError 判断是否为网络相关错误
func IsNetworkError(err error) bool {
	if err == nil {
		return false
	}
	// 检查常见的网络错误关键词
	msg := err.Error()
	keywords := []string{"connection", "network", "timeout", "dial", "refused", "reset"}
	for _, keyword := range keywords {
		if containsIgnoreCase(msg, keyword) {
			return true
		}
	}
	return false
}

// containsIgnoreCase 忽略大小写的字符串包含检查
func containsIgnoreCase(s, substr string) bool {
	sLower := toLower(s)
	substrLower := toLower(substr)
	return contains(sLower, substrLower)
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		result[i] = c
	}
	return string(result)
}

func contains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
