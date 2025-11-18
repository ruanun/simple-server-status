package errors

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// RetryConfig 重试配置
type RetryConfig struct {
	MaxAttempts   int           // 最大重试次数
	InitialDelay  time.Duration // 初始延迟
	MaxDelay      time.Duration // 最大延迟
	BackoffFactor float64       // 退避因子
	Timeout       time.Duration // 超时时间
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
	logger        *zap.SugaredLogger
	retryConfig   *RetryConfig
	errorStats    map[ErrorType]int64
	lastErrors    []*AppError
	maxLastErrors int
	mu            sync.RWMutex
}

// NewErrorHandler 创建新的错误处理器
func NewErrorHandler(logger *zap.SugaredLogger, config *RetryConfig) *ErrorHandler {
	if config == nil {
		config = DefaultRetryConfig()
	}
	return &ErrorHandler{
		logger:        logger,
		retryConfig:   config,
		errorStats:    make(map[ErrorType]int64),
		lastErrors:    make([]*AppError, 0),
		maxLastErrors: 100,
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

	// 根据严重程度记录日志
	eh.logError(err)
}

// logError 记录错误日志
func (eh *ErrorHandler) logError(err *AppError) {
	if eh.logger == nil {
		return
	}

	logMsg := fmt.Sprintf("[%s] %s", ErrorTypeNames[err.Type], err.Error())

	switch err.Severity {
	case SeverityLow:
		eh.logger.Debug(logMsg)
	case SeverityMedium:
		eh.logger.Warn(logMsg)
	case SeverityHigh:
		eh.logger.Error(logMsg)
	case SeverityCritical:
		eh.logger.Error("⚠️  严重错误: ", logMsg)
	}
}

// RetryWithBackoff 带指数退避的重试机制
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
			// 成功，如果之前有失败记录日志
			if attempt > 1 && eh.logger != nil {
				eh.logger.Infof("操作在第 %d 次尝试后成功", attempt)
			}
			return nil
		}

		lastErr = err

		// 创建应用错误
		appErr := WrapError(err, errType, SeverityMedium,
			"RETRY_FAILED",
			fmt.Sprintf("操作失败 (尝试 %d/%d)", attempt, eh.retryConfig.MaxAttempts))
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
			// 计算下次延迟时间（指数退避）
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

	if count == 0 {
		return []*AppError{}
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
	eh.logger.Infof("错误统计 - 网络: %d, 系统: %d, 配置: %d, 数据: %d, WebSocket: %d, 验证: %d, 认证: %d, 未知: %d",
		stats[ErrorTypeNetwork],
		stats[ErrorTypeSystem],
		stats[ErrorTypeConfig],
		stats[ErrorTypeData],
		stats[ErrorTypeWebSocket],
		stats[ErrorTypeValidation],
		stats[ErrorTypeAuthentication],
		stats[ErrorTypeUnknown])
}

// SafeExecute 安全执行函数，捕获 panic 并转换为错误
func SafeExecute(operation func() error, errType ErrorType, description string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = NewAppError(errType, SeverityCritical, "PANIC",
				fmt.Sprintf("Panic 发生在 %s: %v", description, r))
		}
	}()

	return operation()
}

// SafeGo 安全启动 goroutine，捕获 panic
func SafeGo(handler *ErrorHandler, fn func(), description string) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				panicErr := NewAppError(ErrorTypeSystem, SeverityCritical, "GOROUTINE_PANIC",
					fmt.Sprintf("Goroutine panic: %s", description)).
					WithDetails(fmt.Sprintf("%v", r))
				if handler != nil {
					handler.HandleError(panicErr)
				}
			}
		}()
		fn()
	}()
}
