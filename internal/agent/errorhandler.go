package internal

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ErrorType 错误类型枚举
type ErrorType int

const (
	ErrorTypeNetwork ErrorType = iota
	ErrorTypeSystem
	ErrorTypeConfig
	ErrorTypeData
	ErrorTypeUnknown
)

// ErrorSeverity 错误严重程度
type ErrorSeverity int

const (
	SeverityLow ErrorSeverity = iota
	SeverityMedium
	SeverityHigh
	SeverityCritical
)

// AppError 应用错误结构
type AppError struct {
	Type      ErrorType
	Severity  ErrorSeverity
	Message   string
	Cause     error
	Timestamp time.Time
	Retryable bool
}

func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

// NewAppError 创建新的应用错误
func NewAppError(errType ErrorType, severity ErrorSeverity, message string, cause error) *AppError {
	return &AppError{
		Type:      errType,
		Severity:  severity,
		Message:   message,
		Cause:     cause,
		Timestamp: time.Now(),
		Retryable: isRetryable(errType, severity),
	}
}

// isRetryable 判断错误是否可重试
func isRetryable(errType ErrorType, severity ErrorSeverity) bool {
	switch errType {
	case ErrorTypeNetwork:
		return severity != SeverityCritical
	case ErrorTypeSystem:
		return severity == SeverityLow || severity == SeverityMedium
	case ErrorTypeConfig:
		return false // 配置错误通常不可重试
	case ErrorTypeData:
		return severity != SeverityCritical
	default:
		return false
	}
}

// RetryConfig 重试配置
type RetryConfig struct {
	MaxAttempts   int
	InitialDelay  time.Duration
	MaxDelay      time.Duration
	BackoffFactor float64
	Timeout       time.Duration
}

// DefaultRetryConfig 默认重试配置
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  time.Second,
		MaxDelay:      time.Minute,
		BackoffFactor: 2.0,
		Timeout:       time.Minute * 5,
	}
}

// ErrorHandler 错误处理器
type ErrorHandler struct {
	retryConfig   *RetryConfig
	errorStats    map[ErrorType]int64
	lastErrors    []*AppError
	maxLastErrors int
	logger        interface {
		Infof(string, ...interface{})
		Debugf(string, ...interface{})
		Warnf(string, ...interface{})
		Errorf(string, ...interface{})
	}
	monitor *PerformanceMonitor
	mu      sync.RWMutex
}

// NewErrorHandler 创建新的错误处理器
func NewErrorHandler(logger interface {
	Infof(string, ...interface{})
	Debugf(string, ...interface{})
	Warnf(string, ...interface{})
	Errorf(string, ...interface{})
}, monitor *PerformanceMonitor) *ErrorHandler {
	return &ErrorHandler{
		retryConfig:   DefaultRetryConfig(),
		errorStats:    make(map[ErrorType]int64),
		lastErrors:    make([]*AppError, 0),
		maxLastErrors: 100,
		logger:        logger,
		monitor:       monitor,
	}
}

// HandleError 处理错误
func (eh *ErrorHandler) HandleError(err *AppError) {
	eh.mu.Lock()
	defer eh.mu.Unlock()

	// 记录错误统计
	eh.errorStats[err.Type]++

	// 保存最近的错误
	eh.lastErrors = append(eh.lastErrors, err)
	if len(eh.lastErrors) > eh.maxLastErrors {
		eh.lastErrors = eh.lastErrors[1:]
	}

	// 记录到性能监控
	if eh.monitor != nil {
		eh.monitor.IncrementError()
	}

	// 根据严重程度记录日志
	if eh.logger != nil {
		switch err.Severity {
		case SeverityLow:
			eh.logger.Debugf("Low severity error: %s", err.Error())
		case SeverityMedium:
			eh.logger.Warnf("Medium severity error: %s", err.Error())
		case SeverityHigh:
			eh.logger.Errorf("High severity error: %s", err.Error())
		case SeverityCritical:
			eh.logger.Errorf("Critical error: %s", err.Error())
		}
	}
}

// RetryWithBackoff 带退避的重试机制
func (eh *ErrorHandler) RetryWithBackoff(ctx context.Context, operation func() error, errType ErrorType) error {
	var lastErr error
	delay := eh.retryConfig.InitialDelay

	for attempt := 1; attempt <= eh.retryConfig.MaxAttempts; attempt++ {
		// 检查上下文是否已取消
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// 执行操作
		err := operation()
		if err == nil {
			return nil // 成功
		}

		lastErr = err

		// 创建应用错误
		appErr := NewAppError(errType, SeverityMedium, fmt.Sprintf("Operation failed (attempt %d/%d)", attempt, eh.retryConfig.MaxAttempts), err)
		eh.HandleError(appErr)

		// 如果不可重试或已达到最大重试次数，直接返回
		if !appErr.Retryable || attempt == eh.retryConfig.MaxAttempts {
			break
		}

		// 等待后重试
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// 计算下次延迟时间
			delay = time.Duration(float64(delay) * eh.retryConfig.BackoffFactor)
			if delay > eh.retryConfig.MaxDelay {
				delay = eh.retryConfig.MaxDelay
			}
		}
	}

	return lastErr
}

// GetErrorStats 获取错误统计
func (eh *ErrorHandler) GetErrorStats() map[ErrorType]int64 {
	eh.mu.RLock()
	defer eh.mu.RUnlock()
	stats := make(map[ErrorType]int64)
	for k, v := range eh.errorStats {
		stats[k] = v
	}
	return stats
}

// GetRecentErrors 获取最近的错误
func (eh *ErrorHandler) GetRecentErrors(count int) []*AppError {
	eh.mu.RLock()
	defer eh.mu.RUnlock()

	if count <= 0 || count > len(eh.lastErrors) {
		count = len(eh.lastErrors)
	}

	start := len(eh.lastErrors) - count
	result := make([]*AppError, count)
	copy(result, eh.lastErrors[start:])
	return result
}

// LogErrorStats 记录错误统计信息
func (eh *ErrorHandler) LogErrorStats() {
	if eh.logger == nil {
		return
	}
	stats := eh.GetErrorStats()
	eh.logger.Infof("Error Stats - Network: %d, System: %d, Config: %d, Data: %d, Unknown: %d",
		stats[ErrorTypeNetwork], stats[ErrorTypeSystem], stats[ErrorTypeConfig],
		stats[ErrorTypeData], stats[ErrorTypeUnknown])
}

// WrapError 包装错误为应用错误
func WrapError(err error, errType ErrorType, severity ErrorSeverity, message string) *AppError {
	if err == nil {
		return nil
	}
	return NewAppError(errType, severity, message, err)
}

// IsNetworkError 判断是否为网络错误
func IsNetworkError(err error) bool {
	if err == nil {
		return false
	}
	// 这里可以添加更复杂的网络错误判断逻辑
	errorStr := err.Error()
	return contains(errorStr, "connection") || contains(errorStr, "network") ||
		contains(errorStr, "timeout") || contains(errorStr, "dial")
}

// contains 检查字符串是否包含子字符串（忽略大小写）
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr || len(substr) == 0 ||
			containsIgnoreCase(s, substr))
}

func containsIgnoreCase(s, substr string) bool {
	// 简单的忽略大小写包含检查
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			c1 := s[i+j]
			c2 := substr[j]
			if c1 >= 'A' && c1 <= 'Z' {
				c1 += 32
			}
			if c2 >= 'A' && c2 <= 'Z' {
				c2 += 32
			}
			if c1 != c2 {
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
