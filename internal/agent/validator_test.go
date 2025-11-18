package internal

import (
	"strings"
	"testing"

	"github.com/ruanun/simple-server-status/internal/agent/config"
)

// TestValidationError_Error 测试验证错误消息
func TestValidationError_Error(t *testing.T) {
	ve := &ValidationError{
		Field:   "TestField",
		Message: "test error message",
	}

	expected := "validation error for field 'TestField': test error message"
	if ve.Error() != expected {
		t.Errorf("Error() = %s; want %s", ve.Error(), expected)
	}
}

// TestValidationResult_AddError 测试添加错误
func TestValidationResult_AddError(t *testing.T) {
	vr := &ValidationResult{Valid: true}

	// 初始状态应该是有效的
	if !vr.Valid {
		t.Error("初始状态应该为 Valid=true")
	}
	if len(vr.Errors) != 0 {
		t.Errorf("初始错误数量 = %d; want 0", len(vr.Errors))
	}

	// 添加一个错误
	vr.AddError("Field1", "Error 1")

	if vr.Valid {
		t.Error("添加错误后 Valid 应该为 false")
	}
	if len(vr.Errors) != 1 {
		t.Errorf("错误数量 = %d; want 1", len(vr.Errors))
	}
	if vr.Errors[0].Field != "Field1" {
		t.Errorf("Field = %s; want Field1", vr.Errors[0].Field)
	}
	if vr.Errors[0].Message != "Error 1" {
		t.Errorf("Message = %s; want 'Error 1'", vr.Errors[0].Message)
	}

	// 添加更多错误
	vr.AddError("Field2", "Error 2")
	vr.AddError("Field3", "Error 3")

	if len(vr.Errors) != 3 {
		t.Errorf("错误数量 = %d; want 3", len(vr.Errors))
	}
}

// TestValidationResult_GetErrorMessages 测试获取错误消息
func TestValidationResult_GetErrorMessages(t *testing.T) {
	vr := &ValidationResult{Valid: true}

	// 空错误列表
	messages := vr.GetErrorMessages()
	if len(messages) != 0 {
		t.Errorf("空错误列表消息数 = %d; want 0", len(messages))
	}

	// 添加错误
	vr.AddError("Field1", "Message1")
	vr.AddError("Field2", "Message2")

	messages = vr.GetErrorMessages()
	if len(messages) != 2 {
		t.Errorf("错误消息数 = %d; want 2", len(messages))
	}

	// 验证消息格式
	for _, msg := range messages {
		if !strings.Contains(msg, "validation error") {
			t.Errorf("消息格式不正确: %s", msg)
		}
	}
}

// TestNewConfigValidator 测试创建配置验证器
func TestNewConfigValidator(t *testing.T) {
	cfg := &config.AgentConfig{}
	cv := NewConfigValidator(cfg)

	if cv == nil {
		t.Fatal("NewConfigValidator() 返回 nil")
	}
	if cv.config != cfg {
		t.Error("配置未正确设置")
	}
}

// TestConfigValidator_ValidateRequiredFields 测试必填字段验证
func TestConfigValidator_ValidateRequiredFields(t *testing.T) {
	tests := []struct {
		name          string
		config        *config.AgentConfig
		expectValid   bool
		expectedField string
	}{
		{
			name: "所有必填字段都存在",
			config: &config.AgentConfig{
				ServerAddr: "ws://localhost:8080",
				ServerId:   "test-server",
				AuthSecret: "test-secret",
			},
			expectValid: true,
		},
		{
			name: "ServerAddr 为空",
			config: &config.AgentConfig{
				ServerAddr: "",
				ServerId:   "test-server",
				AuthSecret: "test-secret",
			},
			expectValid:   false,
			expectedField: "ServerAddr",
		},
		{
			name: "ServerId 为空",
			config: &config.AgentConfig{
				ServerAddr: "ws://localhost:8080",
				ServerId:   "",
				AuthSecret: "test-secret",
			},
			expectValid:   false,
			expectedField: "ServerId",
		},
		{
			name: "AuthSecret 为空",
			config: &config.AgentConfig{
				ServerAddr: "ws://localhost:8080",
				ServerId:   "test-server",
				AuthSecret: "",
			},
			expectValid:   false,
			expectedField: "AuthSecret",
		},
		{
			name: "所有字段都为空",
			config: &config.AgentConfig{
				ServerAddr: "",
				ServerId:   "",
				AuthSecret: "",
			},
			expectValid: false,
		},
		{
			name: "字段只包含空格",
			config: &config.AgentConfig{
				ServerAddr: "   ",
				ServerId:   "  ",
				AuthSecret: " ",
			},
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cv := NewConfigValidator(tt.config)
			result := &ValidationResult{Valid: true}
			cv.validateRequiredFields(result)

			if result.Valid != tt.expectValid {
				t.Errorf("Valid = %v; want %v", result.Valid, tt.expectValid)
			}

			if !tt.expectValid && tt.expectedField != "" {
				// 验证包含预期的字段错误
				found := false
				for _, err := range result.Errors {
					if err.Field == tt.expectedField {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("未找到预期的字段错误: %s", tt.expectedField)
				}
			}
		})
	}
}

// TestConfigValidator_ValidateServerAddr 测试服务器地址验证
func TestConfigValidator_ValidateServerAddr(t *testing.T) {
	tests := []struct {
		name        string
		serverAddr  string
		expectValid bool
	}{
		{"有效的 ws 地址", "ws://localhost:8080/api", true},
		{"有效的 wss 地址", "wss://example.com:8443/ws", true},
		{"无效前缀 http", "http://localhost:8080", false},
		{"无效前缀 https", "https://localhost:8080", false},
		{"缺少主机名", "ws:///path", false},
		{"无效的 URL 格式", "ws://[invalid", false},
		{"空地址", "", true}, // 在必填字段验证中处理
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.AgentConfig{ServerAddr: tt.serverAddr}
			cv := NewConfigValidator(cfg)
			result := &ValidationResult{Valid: true}
			cv.validateServerAddr(result)

			if result.Valid != tt.expectValid {
				t.Errorf("Valid = %v; want %v, errors: %v", result.Valid, tt.expectValid, result.GetErrorMessages())
			}
		})
	}
}

// TestConfigValidator_ValidateServerId 测试服务器ID验证
func TestConfigValidator_ValidateServerId(t *testing.T) {
	tests := []struct {
		name        string
		serverId    string
		expectValid bool
	}{
		{"有效ID - 字母数字", "server-123", true},
		{"有效ID - 包含下划线", "server_test_01", true},
		{"有效ID - 包含连字符", "test-server-01", true},
		{"有效ID - 全字母", "testserver", true},
		{"有效ID - 全数字", "123456", true},
		{"无效ID - 包含空格", "test server", false},
		{"无效ID - 包含特殊字符", "test@server", false},
		{"无效ID - 包含点", "test.server", false},
		{"无效ID - 太短", "ab", false},
		{"无效ID - 太长", strings.Repeat("a", 51), false},
		{"边界 - 最短有效长度", "abc", true},
		{"边界 - 最长有效长度", strings.Repeat("a", 50), true},
		{"空ID", "", true}, // 在必填字段验证中处理
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.AgentConfig{ServerId: tt.serverId}
			cv := NewConfigValidator(cfg)
			result := &ValidationResult{Valid: true}
			cv.validateServerId(result)

			if result.Valid != tt.expectValid {
				t.Errorf("Valid = %v; want %v, errors: %v", result.Valid, tt.expectValid, result.GetErrorMessages())
			}
		})
	}
}

// TestConfigValidator_ValidateAuthSecret 测试认证密钥验证
func TestConfigValidator_ValidateAuthSecret(t *testing.T) {
	tests := []struct {
		name        string
		authSecret  string
		expectValid bool
	}{
		{"有效密钥 - 16字符", "1234567890123456", true},
		{"有效密钥 - 包含特殊字符", "Test!@#$%^&*()_+", true},
		{"有效密钥 - 混合字符", "Ab12!@#$XyZ", true},
		{"无效密钥 - 太短", "1234567", false},
		{"无效密钥 - 太长", strings.Repeat("a", 257), false},
		{"无效密钥 - 包含空格", "test secret", false},
		{"边界 - 最短有效长度", "12345678", true},
		{"边界 - 最长有效长度", strings.Repeat("a", 256), true},
		{"警告 - 短于16字符", "12345678901", true}, // 有效但会警告
		{"空密钥", "", true},                    // 在必填字段验证中处理
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.AgentConfig{AuthSecret: tt.authSecret}
			cv := NewConfigValidator(cfg)
			result := &ValidationResult{Valid: true}
			cv.validateAuthSecret(result)

			if result.Valid != tt.expectValid {
				t.Errorf("Valid = %v; want %v, errors: %v", result.Valid, tt.expectValid, result.GetErrorMessages())
			}
		})
	}
}

// TestConfigValidator_ValidateReportTimeInterval 测试上报间隔验证
func TestConfigValidator_ValidateReportTimeInterval(t *testing.T) {
	tests := []struct {
		name        string
		interval    int
		expectValid bool
	}{
		{"有效间隔 - 2秒", 2, true},
		{"有效间隔 - 60秒", 60, true},
		{"有效间隔 - 30秒", 30, true},
		{"无效间隔 - 0秒", 0, false},
		{"无效间隔 - 负数", -1, false},
		{"无效间隔 - 超过最大值", 301, false},
		{"边界 - 最小有效值", 1, true},
		{"边界 - 最大有效值", 300, true},
		{"警告 - 太短", 1, true},   // 有效但会警告
		{"警告 - 太长", 120, true}, // 有效但会警告
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.AgentConfig{ReportTimeInterval: tt.interval}
			cv := NewConfigValidator(cfg)
			result := &ValidationResult{Valid: true}
			cv.validateReportTimeInterval(result)

			if result.Valid != tt.expectValid {
				t.Errorf("Valid = %v; want %v, errors: %v", result.Valid, tt.expectValid, result.GetErrorMessages())
			}
		})
	}
}

// TestConfigValidator_ValidateLogConfig 测试日志配置验证
func TestConfigValidator_ValidateLogConfig(t *testing.T) {
	tests := []struct {
		name        string
		logLevel    string
		logPath     string
		expectValid bool
	}{
		{"有效 - debug级别", "debug", "/var/log/test.log", true},
		{"有效 - info级别", "info", "/var/log/test.log", true},
		{"有效 - warn级别", "warn", "/var/log/test.log", true},
		{"有效 - error级别", "error", "/var/log/test.log", true},
		{"有效 - 大写级别", "INFO", "/var/log/test.log", true},
		{"有效 - 混合大小写", "Debug", "/var/log/test.log", true},
		{"有效 - 空级别", "", "/var/log/test.log", true},
		{"有效 - 空路径", "info", "", true},
		{"无效 - 未知级别", "unknown", "/var/log/test.log", false},
		{"无效 - 路径包含非法字符", "info", "/path<>invalid.log", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.AgentConfig{
				LogLevel: tt.logLevel,
				LogPath:  tt.logPath,
			}
			cv := NewConfigValidator(cfg)
			result := &ValidationResult{Valid: true}
			cv.validateLogConfig(result)

			if result.Valid != tt.expectValid {
				t.Errorf("Valid = %v; want %v, errors: %v", result.Valid, tt.expectValid, result.GetErrorMessages())
			}
		})
	}
}

// TestConfigValidator_ValidateConfig 测试完整配置验证
func TestConfigValidator_ValidateConfig(t *testing.T) {
	t.Run("完全有效的配置", func(t *testing.T) {
		cfg := &config.AgentConfig{
			ServerAddr:         "ws://localhost:8080/ws",
			ServerId:           "test-server-001",
			AuthSecret:         "my-super-secret-key",
			ReportTimeInterval: 10,
			LogLevel:           "info",
			LogPath:            "/var/log/agent.log",
		}

		cv := NewConfigValidator(cfg)
		result := cv.ValidateConfig()

		if !result.Valid {
			t.Errorf("配置应该有效，但验证失败: %v", result.GetErrorMessages())
		}
		if len(result.Errors) != 0 {
			t.Errorf("不应该有错误，但有 %d 个", len(result.Errors))
		}
	})

	t.Run("包含多个错误的配置", func(t *testing.T) {
		cfg := &config.AgentConfig{
			ServerAddr:         "http://invalid", // 错误：不是 ws 协议
			ServerId:           "ab",             // 错误：太短
			AuthSecret:         "short",          // 错误：太短
			ReportTimeInterval: 0,                // 错误：无效值
			LogLevel:           "invalid-level",  // 错误：未知级别
		}

		cv := NewConfigValidator(cfg)
		result := cv.ValidateConfig()

		if result.Valid {
			t.Error("配置应该无效")
		}
		if len(result.Errors) == 0 {
			t.Error("应该有多个验证错误")
		}

		// 验证包含预期的错误字段
		expectedFields := []string{"ServerAddr", "ServerId", "AuthSecret", "ReportTimeInterval", "LogLevel"}
		for _, field := range expectedFields {
			found := false
			for _, err := range result.Errors {
				if err.Field == field {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("未找到字段 %s 的验证错误", field)
			}
		}
	})
}

// TestSetConfigDefaults 测试设置默认值
func TestSetConfigDefaults(t *testing.T) {
	tests := []struct {
		name           string
		input          *config.AgentConfig
		expectedReport int
		expectedLog    string
	}{
		{
			name: "所有字段都为空",
			input: &config.AgentConfig{
				ReportTimeInterval: 0,
				LogLevel:           "",
			},
			expectedReport: 2,
			expectedLog:    "info",
		},
		{
			name: "已有自定义值",
			input: &config.AgentConfig{
				ReportTimeInterval: 10,
				LogLevel:           "debug",
			},
			expectedReport: 10,
			expectedLog:    "debug",
		},
		{
			name: "大写日志级别",
			input: &config.AgentConfig{
				ReportTimeInterval: 5,
				LogLevel:           "WARN",
			},
			expectedReport: 5,
			expectedLog:    "warn",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setConfigDefaults(tt.input)

			if tt.input.ReportTimeInterval != tt.expectedReport {
				t.Errorf("ReportTimeInterval = %d; want %d", tt.input.ReportTimeInterval, tt.expectedReport)
			}
			if tt.input.LogLevel != tt.expectedLog {
				t.Errorf("LogLevel = %s; want %s", tt.input.LogLevel, tt.expectedLog)
			}
		})
	}
}

// TestValidateAndSetDefaults 测试验证和设置默认值
func TestValidateAndSetDefaults(t *testing.T) {
	t.Run("有效配置", func(t *testing.T) {
		cfg := &config.AgentConfig{
			ServerAddr: "ws://localhost:8080",
			ServerId:   "test-server",
			AuthSecret: "test-secret-key",
		}

		err := ValidateAndSetDefaults(cfg)
		if err != nil {
			t.Errorf("ValidateAndSetDefaults() error = %v; want nil", err)
		}

		// 验证默认值已设置
		if cfg.ReportTimeInterval == 0 {
			t.Error("默认的 ReportTimeInterval 未设置")
		}
		if cfg.LogLevel == "" {
			t.Error("默认的 LogLevel 未设置")
		}
	})

	t.Run("无效配置", func(t *testing.T) {
		cfg := &config.AgentConfig{
			ServerAddr: "",
			ServerId:   "",
			AuthSecret: "",
		}

		err := ValidateAndSetDefaults(cfg)
		if err == nil {
			t.Error("ValidateAndSetDefaults() 应该返回错误")
		}
	})
}

// TestValidateEnvironment 测试环境验证
func TestValidateEnvironment(t *testing.T) {
	// 环境验证应该总是成功（当前实现）
	err := ValidateEnvironment()
	if err != nil {
		t.Errorf("ValidateEnvironment() error = %v; want nil", err)
	}
}
