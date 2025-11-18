package internal

import (
	"fmt"
	"net/http"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
)

// ErrorType 错误类型
type ErrorType int

const (
	ErrorTypeValidation ErrorType = iota
	ErrorTypeAuthentication
	ErrorTypeAuthorization
	ErrorTypeNotFound
	ErrorTypeInternal
	ErrorTypeWebSocket
	ErrorTypeConfig
	ErrorTypeNetwork
)

// AppError 应用错误结构
type AppError struct {
	Type       ErrorType `json:"type"`
	Code       string    `json:"code"`
	Message    string    `json:"message"`
	Details    string    `json:"details,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
	Path       string    `json:"path,omitempty"`
	Method     string    `json:"method,omitempty"`
	UserAgent  string    `json:"user_agent,omitempty"`
	IP         string    `json:"ip,omitempty"`
	StackTrace string    `json:"stack_trace,omitempty"`
}

// Error 实现error接口
func (e *AppError) Error() string {
	return fmt.Sprintf("[%s] %s: %s", e.Code, e.Message, e.Details)
}

// ErrorHandler 错误处理器
type ErrorHandler struct {
	errorCounts map[ErrorType]int64
	lastErrors  []*AppError
	maxHistory  int
	logger      interface {
		Infof(string, ...interface{})
		Errorf(string, ...interface{})
		Warnf(string, ...interface{})
	}
}

// NewErrorHandler 创建错误处理器
func NewErrorHandler(logger interface {
	Infof(string, ...interface{})
	Errorf(string, ...interface{})
	Warnf(string, ...interface{})
}) *ErrorHandler {
	return &ErrorHandler{
		errorCounts: make(map[ErrorType]int64),
		lastErrors:  make([]*AppError, 0),
		maxHistory:  100, // 保留最近100个错误
		logger:      logger,
	}
}

// RecordError 记录错误
func (eh *ErrorHandler) RecordError(err *AppError) {
	// 增加错误计数
	eh.errorCounts[err.Type]++

	// 添加到历史记录
	eh.lastErrors = append(eh.lastErrors, err)
	if len(eh.lastErrors) > eh.maxHistory {
		eh.lastErrors = eh.lastErrors[1:]
	}

	// 记录日志
	eh.logError(err)
}

// logError 记录错误日志
func (eh *ErrorHandler) logError(err *AppError) {
	if eh.logger == nil {
		return
	}

	logMsg := fmt.Sprintf("错误类型: %s, 代码: %s, 消息: %s", eh.getErrorTypeName(err.Type), err.Code, err.Message)

	if err.Details != "" {
		logMsg += fmt.Sprintf(", 详情: %s", err.Details)
	}

	if err.Path != "" {
		logMsg += fmt.Sprintf(", 路径: %s %s", err.Method, err.Path)
	}

	if err.IP != "" {
		logMsg += fmt.Sprintf(", IP: %s", err.IP)
	}

	if err.StackTrace != "" {
		logMsg += fmt.Sprintf(", 堆栈: %s", err.StackTrace)
	}

	// 根据错误类型选择日志级别
	switch err.Type {
	case ErrorTypeInternal:
		eh.logger.Errorf(logMsg)
	case ErrorTypeAuthentication, ErrorTypeAuthorization:
		eh.logger.Warnf(logMsg)
	case ErrorTypeValidation, ErrorTypeNotFound:
		eh.logger.Infof(logMsg)
	default:
		eh.logger.Errorf(logMsg)
	}
}

// getErrorTypeName 获取错误类型名称
func (eh *ErrorHandler) getErrorTypeName(errorType ErrorType) string {
	switch errorType {
	case ErrorTypeValidation:
		return "验证错误"
	case ErrorTypeAuthentication:
		return "认证错误"
	case ErrorTypeAuthorization:
		return "授权错误"
	case ErrorTypeNotFound:
		return "资源未找到"
	case ErrorTypeInternal:
		return "内部错误"
	case ErrorTypeWebSocket:
		return "WebSocket错误"
	case ErrorTypeConfig:
		return "配置错误"
	case ErrorTypeNetwork:
		return "网络错误"
	default:
		return "未知错误"
	}
}

// GetErrorStats 获取错误统计
func (eh *ErrorHandler) GetErrorStats() map[string]interface{} {
	stats := make(map[string]interface{})

	for errorType, count := range eh.errorCounts {
		stats[eh.getErrorTypeName(errorType)] = count
	}

	stats["total_errors"] = len(eh.lastErrors)
	stats["recent_errors"] = len(eh.lastErrors)

	return stats
}

// GetRecentErrors 获取最近的错误
func (eh *ErrorHandler) GetRecentErrors(limit int) []*AppError {
	if limit <= 0 || limit > len(eh.lastErrors) {
		limit = len(eh.lastErrors)
	}

	start := len(eh.lastErrors) - limit
	return eh.lastErrors[start:]
}

// ErrorMiddleware Gin错误处理中间件
func ErrorMiddleware(errorHandler *ErrorHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// 检查是否有错误
		if len(c.Errors) > 0 {
			err := c.Errors.Last()

			// 创建应用错误
			appErr := &AppError{
				Type:      ErrorTypeInternal,
				Code:      "INTERNAL_ERROR",
				Message:   "内部服务器错误",
				Details:   err.Error(),
				Timestamp: time.Now(),
				Path:      c.Request.URL.Path,
				Method:    c.Request.Method,
				UserAgent: c.Request.UserAgent(),
				IP:        c.ClientIP(),
			}

			// 记录错误
			if errorHandler != nil {
				errorHandler.RecordError(appErr)
			}

			// 返回错误响应
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code":      appErr.Code,
					"message":   appErr.Message,
					"timestamp": appErr.Timestamp,
				},
			})
			c.Abort()
		}
	}
}

// PanicRecoveryMiddleware Panic恢复中间件
func PanicRecoveryMiddleware(errorHandler *ErrorHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				// 获取堆栈信息
				stack := make([]byte, 4096)
				length := runtime.Stack(stack, false)
				stackTrace := string(stack[:length])

				// 创建应用错误
				appErr := &AppError{
					Type:       ErrorTypeInternal,
					Code:       "PANIC_RECOVERED",
					Message:    "服务器发生严重错误",
					Details:    fmt.Sprintf("%v", r),
					Timestamp:  time.Now(),
					Path:       c.Request.URL.Path,
					Method:     c.Request.Method,
					UserAgent:  c.Request.UserAgent(),
					IP:         c.ClientIP(),
					StackTrace: stackTrace,
				}

				// 记录错误
				if errorHandler != nil {
					errorHandler.RecordError(appErr)
				}

				// 返回错误响应
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": gin.H{
						"code":      appErr.Code,
						"message":   appErr.Message,
						"timestamp": appErr.Timestamp,
					},
				})
				c.Abort()
			}
		}()

		c.Next()
	}
}

// 便捷函数用于创建不同类型的错误

// NewValidationError 创建验证错误
func NewValidationError(message, details string) *AppError {
	return &AppError{
		Type:      ErrorTypeValidation,
		Code:      "VALIDATION_ERROR",
		Message:   message,
		Details:   details,
		Timestamp: time.Now(),
	}
}

// NewAuthenticationError 创建认证错误
func NewAuthenticationError(message, details string) *AppError {
	return &AppError{
		Type:      ErrorTypeAuthentication,
		Code:      "AUTHENTICATION_ERROR",
		Message:   message,
		Details:   details,
		Timestamp: time.Now(),
	}
}

// NewWebSocketError 创建WebSocket错误
func NewWebSocketError(message, details string) *AppError {
	return &AppError{
		Type:      ErrorTypeWebSocket,
		Code:      "WEBSOCKET_ERROR",
		Message:   message,
		Details:   details,
		Timestamp: time.Now(),
	}
}

// NewConfigError 创建配置错误
func NewConfigError(message, details string) *AppError {
	return &AppError{
		Type:      ErrorTypeConfig,
		Code:      "CONFIG_ERROR",
		Message:   message,
		Details:   details,
		Timestamp: time.Now(),
	}
}
