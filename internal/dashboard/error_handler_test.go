package internal

import (
	"testing"
	"time"
)

// MockLogger 模拟日志记录器
type MockLogger struct {
	InfoMessages  []string
	ErrorMessages []string
	WarnMessages  []string
}

func (m *MockLogger) Infof(format string, args ...interface{}) {
	m.InfoMessages = append(m.InfoMessages, format)
}

func (m *MockLogger) Errorf(format string, args ...interface{}) {
	m.ErrorMessages = append(m.ErrorMessages, format)
}

func (m *MockLogger) Warnf(format string, args ...interface{}) {
	m.WarnMessages = append(m.WarnMessages, format)
}

// TestNewErrorHandler 测试创建错误处理器
func TestNewErrorHandler(t *testing.T) {
	logger := &MockLogger{}
	eh := NewErrorHandler(logger)

	if eh == nil {
		t.Fatal("创建错误处理器失败")
	}
	if eh.errorCounts == nil {
		t.Error("错误计数器未初始化")
	}
	if eh.lastErrors == nil {
		t.Error("错误历史未初始化")
	}
	if eh.maxHistory != 100 {
		t.Errorf("最大历史记录应为100，实际为%d", eh.maxHistory)
	}
}

// TestRecordError 测试记录错误
func TestRecordError(t *testing.T) {
	logger := &MockLogger{}
	eh := NewErrorHandler(logger)

	// 创建一个测试错误
	err := &AppError{
		Type:      ErrorTypeValidation,
		Code:      "VALIDATION_ERROR",
		Message:   "验证失败",
		Details:   "字段不能为空",
		Timestamp: time.Now(),
	}

	eh.RecordError(err)

	// 验证错误计数
	if eh.errorCounts[ErrorTypeValidation] != 1 {
		t.Errorf("期望验证错误计数为1，实际为%d", eh.errorCounts[ErrorTypeValidation])
	}

	// 验证错误历史
	if len(eh.lastErrors) != 1 {
		t.Errorf("期望错误历史长度为1，实际为%d", len(eh.lastErrors))
	}

	// 验证日志记录
	if len(logger.InfoMessages) != 1 {
		t.Errorf("期望记录1条信息日志，实际为%d", len(logger.InfoMessages))
	}
}

// TestRecordMultipleErrors 测试记录多个错误
func TestRecordMultipleErrors(t *testing.T) {
	logger := &MockLogger{}
	eh := NewErrorHandler(logger)

	// 记录不同类型的错误
	errors := []struct {
		errorType ErrorType
		expected  int64
	}{
		{ErrorTypeValidation, 2},
		{ErrorTypeAuthentication, 1},
		{ErrorTypeInternal, 1},
	}

	// 记录验证错误2次
	eh.RecordError(&AppError{Type: ErrorTypeValidation, Code: "V1", Message: "错误1"})
	eh.RecordError(&AppError{Type: ErrorTypeValidation, Code: "V2", Message: "错误2"})
	// 记录认证错误1次
	eh.RecordError(&AppError{Type: ErrorTypeAuthentication, Code: "A1", Message: "错误3"})
	// 记录内部错误1次
	eh.RecordError(&AppError{Type: ErrorTypeInternal, Code: "I1", Message: "错误4"})

	// 验证计数
	for _, err := range errors {
		if eh.errorCounts[err.errorType] != err.expected {
			t.Errorf("错误类型 %d: 期望计数%d，实际为%d", err.errorType, err.expected, eh.errorCounts[err.errorType])
		}
	}

	// 验证总错误数
	if len(eh.lastErrors) != 4 {
		t.Errorf("期望总错误数为4，实际为%d", len(eh.lastErrors))
	}
}

// TestErrorHistoryLimit 测试错误历史记录限制
func TestErrorHistoryLimit(t *testing.T) {
	logger := &MockLogger{}
	eh := NewErrorHandler(logger)

	// 记录超过maxHistory的错误
	for i := 0; i < 150; i++ {
		eh.RecordError(&AppError{
			Type:    ErrorTypeInternal,
			Code:    "TEST",
			Message: "测试错误",
		})
	}

	// 验证历史记录不超过限制
	if len(eh.lastErrors) > eh.maxHistory {
		t.Errorf("错误历史记录超过限制: %d > %d", len(eh.lastErrors), eh.maxHistory)
	}
	if len(eh.lastErrors) != eh.maxHistory {
		t.Errorf("期望错误历史记录为%d，实际为%d", eh.maxHistory, len(eh.lastErrors))
	}
}

// TestGetErrorStats 测试获取错误统计
func TestGetErrorStats(t *testing.T) {
	logger := &MockLogger{}
	eh := NewErrorHandler(logger)

	// 记录一些错误
	eh.RecordError(&AppError{Type: ErrorTypeValidation, Code: "V1", Message: "错误1"})
	eh.RecordError(&AppError{Type: ErrorTypeValidation, Code: "V2", Message: "错误2"})
	eh.RecordError(&AppError{Type: ErrorTypeAuthentication, Code: "A1", Message: "错误3"})

	stats := eh.GetErrorStats()

	// 验证统计信息
	if stats == nil {
		t.Fatal("统计信息不应为nil")
	}

	if stats["total_errors"].(int) != 3 {
		t.Errorf("期望总错误数为3，实际为%v", stats["total_errors"])
	}

	if stats["验证错误"].(int64) != 2 {
		t.Errorf("期望验证错误数为2，实际为%v", stats["验证错误"])
	}

	if stats["认证错误"].(int64) != 1 {
		t.Errorf("期望认证错误数为1，实际为%v", stats["认证错误"])
	}
}

// TestGetRecentErrors 测试获取最近的错误
func TestGetRecentErrors(t *testing.T) {
	logger := &MockLogger{}
	eh := NewErrorHandler(logger)

	// 记录5个错误
	for i := 0; i < 5; i++ {
		eh.RecordError(&AppError{
			Type:    ErrorTypeInternal,
			Code:    "TEST",
			Message: "测试错误",
		})
	}

	tests := []struct {
		name  string
		limit int
		want  int
	}{
		{"获取3个错误", 3, 3},
		{"获取所有错误", 10, 5},
		{"获取0个错误", 0, 5},  // 0应返回所有
		{"获取负数错误", -1, 5}, // 负数应返回所有
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := eh.GetRecentErrors(tt.limit)
			if len(errors) != tt.want {
				t.Errorf("期望返回%d个错误，实际返回%d个", tt.want, len(errors))
			}
		})
	}
}

// TestAppErrorError 测试AppError的Error()方法
func TestAppErrorError(t *testing.T) {
	err := &AppError{
		Code:    "TEST_ERROR",
		Message: "测试消息",
		Details: "详细信息",
	}

	errStr := err.Error()
	if errStr == "" {
		t.Error("Error()应返回非空字符串")
	}

	// 验证错误字符串包含关键信息
	if !containsString(errStr, "TEST_ERROR") {
		t.Error("错误字符串应包含错误代码")
	}
	if !containsString(errStr, "测试消息") {
		t.Error("错误字符串应包含错误消息")
	}
}

// TestLogError 测试日志记录
func TestLogError(t *testing.T) {
	logger := &MockLogger{}
	eh := NewErrorHandler(logger)

	tests := []struct {
		name          string
		errorType     ErrorType
		expectedLevel string // "info", "error", "warn"
	}{
		{"验证错误记录为Info", ErrorTypeValidation, "info"},
		{"认证错误记录为Warn", ErrorTypeAuthentication, "warn"},
		{"授权错误记录为Warn", ErrorTypeAuthorization, "warn"},
		{"内部错误记录为Error", ErrorTypeInternal, "error"},
		{"WebSocket错误记录为Error", ErrorTypeWebSocket, "error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger = &MockLogger{} // 重置logger
			eh.logger = logger

			err := &AppError{
				Type:    tt.errorType,
				Code:    "TEST",
				Message: "测试",
			}

			eh.logError(err)

			// 验证日志级别
			switch tt.expectedLevel {
			case "info":
				if len(logger.InfoMessages) != 1 {
					t.Errorf("期望记录1条Info日志，实际%d条", len(logger.InfoMessages))
				}
			case "error":
				if len(logger.ErrorMessages) != 1 {
					t.Errorf("期望记录1条Error日志，实际%d条", len(logger.ErrorMessages))
				}
			case "warn":
				if len(logger.WarnMessages) != 1 {
					t.Errorf("期望记录1条Warn日志，实际%d条", len(logger.WarnMessages))
				}
			}
		})
	}
}

// TestGetErrorTypeName 测试获取错误类型名称
func TestGetErrorTypeName(t *testing.T) {
	logger := &MockLogger{}
	eh := NewErrorHandler(logger)

	tests := []struct {
		errorType ErrorType
		wantName  string
	}{
		{ErrorTypeValidation, "验证错误"},
		{ErrorTypeAuthentication, "认证错误"},
		{ErrorTypeAuthorization, "授权错误"},
		{ErrorTypeNotFound, "资源未找到"},
		{ErrorTypeInternal, "内部错误"},
		{ErrorTypeWebSocket, "WebSocket错误"},
		{ErrorTypeConfig, "配置错误"},
		{ErrorTypeNetwork, "网络错误"},
	}

	for _, tt := range tests {
		t.Run(tt.wantName, func(t *testing.T) {
			name := eh.getErrorTypeName(tt.errorType)
			if name != tt.wantName {
				t.Errorf("期望错误类型名称为'%s'，实际为'%s'", tt.wantName, name)
			}
		})
	}
}

// TestErrorHandlerWithNilLogger 测试无logger的错误处理器
func TestErrorHandlerWithNilLogger(t *testing.T) {
	eh := NewErrorHandler(nil)

	// 应该能正常记录错误，不应panic
	err := &AppError{
		Type:    ErrorTypeInternal,
		Code:    "TEST",
		Message: "测试",
	}

	// 这不应该panic
	eh.RecordError(err)

	// 验证错误仍然被记录
	if len(eh.lastErrors) != 1 {
		t.Error("即使没有logger，错误也应该被记录")
	}
}

// 辅助函数
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
