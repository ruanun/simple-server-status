package errors

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// TestNewAppError 测试创建新的应用错误
func TestNewAppError(t *testing.T) {
	tests := []struct {
		name     string
		errType  ErrorType
		severity ErrorSeverity
		code     string
		message  string
	}{
		{
			name:     "网络错误",
			errType:  ErrorTypeNetwork,
			severity: SeverityMedium,
			code:     "NET001",
			message:  "连接超时",
		},
		{
			name:     "配置错误",
			errType:  ErrorTypeConfig,
			severity: SeverityHigh,
			code:     "CFG001",
			message:  "配置文件无效",
		},
		{
			name:     "严重系统错误",
			errType:  ErrorTypeSystem,
			severity: SeverityCritical,
			code:     "SYS001",
			message:  "内存不足",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewAppError(tt.errType, tt.severity, tt.code, tt.message)

			if err.Type != tt.errType {
				t.Errorf("Type = %v, want %v", err.Type, tt.errType)
			}
			if err.Severity != tt.severity {
				t.Errorf("Severity = %v, want %v", err.Severity, tt.severity)
			}
			if err.Code != tt.code {
				t.Errorf("Code = %v, want %v", err.Code, tt.code)
			}
			if err.Message != tt.message {
				t.Errorf("Message = %v, want %v", err.Message, tt.message)
			}
			if err.Timestamp.IsZero() {
				t.Error("Timestamp should not be zero")
			}
			if time.Since(err.Timestamp) > time.Second {
				t.Error("Timestamp should be recent")
			}
		})
	}
}

// TestWrapError 测试包装错误
func TestWrapError(t *testing.T) {
	originalErr := errors.New("原始错误")

	err := WrapError(originalErr, ErrorTypeNetwork, SeverityMedium, "NET002", "网络请求失败")

	if err == nil {
		t.Fatal("Expected non-nil error")
	}
	if err.Cause != originalErr {
		t.Errorf("Cause = %v, want %v", err.Cause, originalErr)
	}
	if !strings.Contains(err.Error(), "原始错误") {
		t.Errorf("Error message should contain cause: %s", err.Error())
	}
}

// TestWrapError_NilError 测试包装空错误
func TestWrapError_NilError(t *testing.T) {
	err := WrapError(nil, ErrorTypeNetwork, SeverityMedium, "NET003", "测试")

	if err != nil {
		t.Errorf("Expected nil error, got %v", err)
	}
}

// TestAppError_WithDetails 测试添加错误详情
func TestAppError_WithDetails(t *testing.T) {
	err := NewAppError(ErrorTypeData, SeverityLow, "DATA001", "数据验证失败").
		WithDetails("字段 'email' 格式不正确")

	if err.Details == "" {
		t.Error("Details should not be empty")
	}
	if !strings.Contains(err.Details, "email") {
		t.Errorf("Details = %v, want to contain 'email'", err.Details)
	}
	if !strings.Contains(err.Error(), err.Details) {
		t.Errorf("Error() should include details: %s", err.Error())
	}
}

// TestAppError_WithCause 测试添加原始错误
func TestAppError_WithCause(t *testing.T) {
	cause := errors.New("底层错误")
	err := NewAppError(ErrorTypeSystem, SeverityHigh, "SYS002", "系统调用失败").
		WithCause(cause)

	if err.Cause != cause {
		t.Errorf("Cause = %v, want %v", err.Cause, cause)
	}
	if !strings.Contains(err.Error(), "底层错误") {
		t.Errorf("Error() should include cause: %s", err.Error())
	}
}

// TestAppError_Error 测试错误消息格式化
func TestAppError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *AppError
		contains []string
	}{
		{
			name: "仅消息",
			err: &AppError{
				Code:    "TEST001",
				Message: "测试错误",
			},
			contains: []string{"TEST001", "测试错误"},
		},
		{
			name: "消息+详情",
			err: &AppError{
				Code:    "TEST002",
				Message: "测试错误",
				Details: "额外的详情",
			},
			contains: []string{"TEST002", "测试错误", "额外的详情"},
		},
		{
			name: "消息+原因",
			err: &AppError{
				Code:    "TEST003",
				Message: "测试错误",
				Cause:   errors.New("原因错误"),
			},
			contains: []string{"TEST003", "测试错误", "原因", "原因错误"},
		},
		{
			name: "完整错误",
			err: &AppError{
				Code:    "TEST004",
				Message: "测试错误",
				Details: "详情",
				Cause:   errors.New("原因"),
			},
			contains: []string{"TEST004", "测试错误", "原因"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errStr := tt.err.Error()
			for _, substr := range tt.contains {
				if !strings.Contains(errStr, substr) {
					t.Errorf("Error() = %v, want to contain %v", errStr, substr)
				}
			}
		})
	}
}

// TestIsRetryable 测试可重试性判断
func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name      string
		errType   ErrorType
		severity  ErrorSeverity
		retryable bool
	}{
		// 网络错误 - 可重试
		{"网络错误-低", ErrorTypeNetwork, SeverityLow, true},
		{"网络错误-中", ErrorTypeNetwork, SeverityMedium, true},
		{"网络错误-高", ErrorTypeNetwork, SeverityHigh, true},
		{"网络错误-严重", ErrorTypeNetwork, SeverityCritical, false}, // 严重错误不可重试

		// 系统错误 - 可重试
		{"系统错误-低", ErrorTypeSystem, SeverityLow, true},
		{"系统错误-中", ErrorTypeSystem, SeverityMedium, true},

		// WebSocket错误 - 可重试
		{"WebSocket错误-中", ErrorTypeWebSocket, SeverityMedium, true},

		// 配置错误 - 不可重试
		{"配置错误-低", ErrorTypeConfig, SeverityLow, false},
		{"配置错误-高", ErrorTypeConfig, SeverityHigh, false},

		// 认证错误 - 不可重试
		{"认证错误-中", ErrorTypeAuthentication, SeverityMedium, false},

		// 严重错误 - 不可重试
		{"数据错误-严重", ErrorTypeData, SeverityCritical, false},
		{"验证错误-严重", ErrorTypeValidation, SeverityCritical, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRetryable(tt.errType, tt.severity)
			if result != tt.retryable {
				t.Errorf("isRetryable(%v, %v) = %v, want %v",
					ErrorTypeNames[tt.errType], SeverityNames[tt.severity], result, tt.retryable)
			}
		})
	}
}

// TestIsNetworkError 测试网络错误识别
func TestIsNetworkError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		isNetwork bool
	}{
		{"空错误", nil, false},
		{"连接超时", errors.New("connection timeout"), true},
		{"网络不可达", errors.New("network unreachable"), true},
		{"拨号失败", errors.New("dial tcp: connection refused"), true},
		{"连接重置", errors.New("connection reset by peer"), true},
		{"大写关键词", errors.New("Connection Timeout"), true},
		{"混合大小写", errors.New("NetWork Error"), true},
		{"普通错误", errors.New("invalid input"), false},
		{"空字符串", errors.New(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsNetworkError(tt.err)
			if result != tt.isNetwork {
				t.Errorf("IsNetworkError(%v) = %v, want %v", tt.err, result, tt.isNetwork)
			}
		})
	}
}

// TestContainsIgnoreCase 测试忽略大小写的字符串包含检查
func TestContainsIgnoreCase(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substr   string
		contains bool
	}{
		{"完全匹配", "hello", "hello", true},
		{"小写在大写中", "HELLO", "hello", true},
		{"大写在小写中", "hello", "HELLO", true},
		{"混合大小写", "HeLLo WoRLd", "lLo wO", true},
		{"不包含", "hello", "world", false},
		{"空子串", "hello", "", true},
		{"空字符串", "", "hello", false},
		{"两者都空", "", "", true},
		{"部分匹配", "connection timeout", "TIMEOUT", true},
		{"中文不影响", "网络错误connection", "CONNECTION", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsIgnoreCase(tt.s, tt.substr)
			if result != tt.contains {
				t.Errorf("containsIgnoreCase(%q, %q) = %v, want %v",
					tt.s, tt.substr, result, tt.contains)
			}
		})
	}
}

// TestToLower 测试小写转换
func TestToLower(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"全大写", "HELLO", "hello"},
		{"全小写", "hello", "hello"},
		{"混合", "HeLLo", "hello"},
		{"带数字", "Test123", "test123"},
		{"带符号", "Hello-World!", "hello-world!"},
		{"空字符串", "", ""},
		{"中文字符", "你好Hello", "你好hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toLower(tt.input)
			if result != tt.want {
				t.Errorf("toLower(%q) = %q, want %q", tt.input, result, tt.want)
			}
		})
	}
}

// TestContains 测试字符串包含检查
func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substr   string
		contains bool
	}{
		{"包含", "hello world", "world", true},
		{"不包含", "hello world", "foo", false},
		{"完全匹配", "hello", "hello", true},
		{"空子串", "hello", "", true},
		{"空字符串", "", "hello", false},
		{"两者都空", "", "", true},
		{"开头", "hello world", "hello", true},
		{"结尾", "hello world", "world", true},
		{"中间", "hello world", "o w", true},
		{"超出长度", "hi", "hello", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.s, tt.substr)
			if result != tt.contains {
				t.Errorf("contains(%q, %q) = %v, want %v",
					tt.s, tt.substr, result, tt.contains)
			}
		})
	}
}

// TestErrorTypeNames 测试错误类型名称映射的完整性
func TestErrorTypeNames(t *testing.T) {
	// 确保所有错误类型都有对应的名称
	expectedTypes := []ErrorType{
		ErrorTypeNetwork,
		ErrorTypeSystem,
		ErrorTypeConfig,
		ErrorTypeData,
		ErrorTypeWebSocket,
		ErrorTypeValidation,
		ErrorTypeAuthentication,
		ErrorTypeUnknown,
	}

	for _, errType := range expectedTypes {
		name, exists := ErrorTypeNames[errType]
		if !exists {
			t.Errorf("ErrorType %v 缺少名称映射", errType)
		}
		if name == "" {
			t.Errorf("ErrorType %v 的名称为空", errType)
		}
	}
}

// TestSeverityNames 测试严重程度名称映射的完整性
func TestSeverityNames(t *testing.T) {
	// 确保所有严重程度都有对应的名称
	expectedSeverities := []ErrorSeverity{
		SeverityLow,
		SeverityMedium,
		SeverityHigh,
		SeverityCritical,
	}

	for _, severity := range expectedSeverities {
		name, exists := SeverityNames[severity]
		if !exists {
			t.Errorf("ErrorSeverity %v 缺少名称映射", severity)
		}
		if name == "" {
			t.Errorf("ErrorSeverity %v 的名称为空", severity)
		}
	}
}

// BenchmarkNewAppError 基准测试：创建应用错误
func BenchmarkNewAppError(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewAppError(ErrorTypeNetwork, SeverityMedium, "NET001", "测试错误")
	}
}

// BenchmarkIsNetworkError 基准测试：网络错误识别
func BenchmarkIsNetworkError(b *testing.B) {
	err := errors.New("connection timeout error")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = IsNetworkError(err)
	}
}

// BenchmarkContainsIgnoreCase 基准测试：忽略大小写的字符串包含检查
func BenchmarkContainsIgnoreCase(b *testing.B) {
	s := "Connection Timeout Error in Network"
	substr := "timeout"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = containsIgnoreCase(s, substr)
	}
}
