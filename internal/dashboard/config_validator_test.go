package internal

import (
	"path/filepath"
	"testing"

	"github.com/ruanun/simple-server-status/internal/dashboard/config"
)

// TestNewConfigValidator 测试创建配置验证器
func TestNewConfigValidator(t *testing.T) {
	cv := NewConfigValidator()
	if cv == nil {
		t.Fatal("创建配置验证器失败")
	}
	if cv.validator == nil {
		t.Error("验证器未初始化")
	}
	if cv.errors == nil {
		t.Error("错误列表未初始化")
	}
}

// TestValidatePort 测试端口验证
func TestValidatePort(t *testing.T) {
	tests := []struct {
		name         string
		port         int
		wantErrorNum int // 期望的错误/警告数量
	}{
		{"有效端口", 8900, 0},
		{"最小有效端口", 1025, 0},
		{"最大有效端口", 65535, 0},
		{"零端口", 0, 1},      // 警告
		{"负数端口", -1, 1},    // 警告
		{"系统端口", 80, 1},    // 警告（系统端口）
		{"超大端口", 70000, 1}, // 错误
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cv := NewConfigValidator()
			cv.validatePort(tt.port)
			if len(cv.errors) != tt.wantErrorNum {
				t.Errorf("端口 %d: 期望 %d 个错误，实际 %d 个", tt.port, tt.wantErrorNum, len(cv.errors))
			}
		})
	}
}

// TestValidateAddress 测试地址验证
func TestValidateAddress(t *testing.T) {
	tests := []struct {
		name         string
		address      string
		wantErrorNum int
		wantLevel    string
	}{
		{"有效IPv4", "192.168.1.1", 0, ""},
		{"有效通配", "0.0.0.0", 0, ""},
		{"空地址", "", 1, "info"},
		{"无效IP", "999.999.999.999", 1, "error"},
		{"无效格式", "not-an-ip", 1, "error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cv := NewConfigValidator()
			cv.validateAddress(tt.address)
			if len(cv.errors) != tt.wantErrorNum {
				t.Errorf("地址 '%s': 期望 %d 个错误，实际 %d 个", tt.address, tt.wantErrorNum, len(cv.errors))
			}
			if tt.wantErrorNum > 0 && cv.errors[0].Level != tt.wantLevel {
				t.Errorf("地址 '%s': 期望错误级别 %s，实际 %s", tt.address, tt.wantLevel, cv.errors[0].Level)
			}
		})
	}
}

// TestValidateWebSocketPath 测试WebSocket路径验证
func TestValidateWebSocketPath(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		wantErrorNum int
		wantLevel    string
	}{
		{"有效路径", "/ws-report", 0, ""},
		{"空路径", "", 1, "info"},
		{"无斜杠开头", "ws-report", 1, "warning"},
		{"包含空格", "/ws report", 1, "error"},     // 只有包含空格错误
		{"无斜杠且含空格", "ws report", 2, "warning"}, // 无斜杠(warning) + 包含空格(error)，第一个是warning
		{"复杂路径", "/api/ws-report", 0, ""},
		{"无斜杠复杂路径", "api/ws-report", 1, "warning"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cv := NewConfigValidator()
			cv.validateWebSocketPath(tt.path)
			if len(cv.errors) != tt.wantErrorNum {
				t.Errorf("路径 '%s': 期望 %d 个错误，实际 %d 个", tt.path, tt.wantErrorNum, len(cv.errors))
			}
			if tt.wantErrorNum > 0 && cv.errors[0].Level != tt.wantLevel {
				t.Errorf("路径 '%s': 期望错误级别 %s，实际 %s", tt.path, tt.wantLevel, cv.errors[0].Level)
			}
		})
	}
}

// TestValidateReportInterval 测试上报间隔验证
func TestValidateReportInterval(t *testing.T) {
	tests := []struct {
		name         string
		interval     int
		wantErrorNum int
	}{
		{"正常间隔", 30, 0},
		{"最小间隔", 5, 0},
		{"最大间隔", 300, 0},
		{"过小间隔", 2, 1},   // 警告
		{"过大间隔", 500, 1}, // 警告
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cv := NewConfigValidator()
			cv.validateReportInterval(tt.interval)
			if len(cv.errors) != tt.wantErrorNum {
				t.Errorf("间隔 %d: 期望 %d 个错误，实际 %d 个", tt.interval, tt.wantErrorNum, len(cv.errors))
			}
		})
	}
}

// TestValidateLogConfig 测试日志配置验证
func TestValidateLogConfig(t *testing.T) {
	// 创建临时目录用于测试
	tmpDir := t.TempDir()
	validLogPath := filepath.Join(tmpDir, "test.log")

	tests := []struct {
		name      string
		logPath   string
		logLevel  string
		wantError bool
	}{
		{"有效配置", validLogPath, "info", false},
		{"有效debug级别", validLogPath, "debug", false},
		{"有效error级别", validLogPath, "error", false},
		{"空配置", "", "", false},
		{"无效日志级别", validLogPath, "invalid", true},
		{"不存在的目录", "/nonexistent/path/test.log", "info", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cv := NewConfigValidator()
			cv.validateLogConfig(tt.logPath, tt.logLevel)
			hasError := cv.hasErrors()
			if hasError != tt.wantError {
				t.Errorf("日志配置 (%s, %s): 期望错误=%v，实际错误=%v", tt.logPath, tt.logLevel, tt.wantError, hasError)
			}
		})
	}
}

// TestValidateServers 测试服务器配置验证
func TestValidateServers(t *testing.T) {
	tests := []struct {
		name      string
		servers   []*config.ServerConfig
		wantError bool
	}{
		{
			name: "有效配置",
			servers: []*config.ServerConfig{
				{
					Id:          "server-1",
					Name:        "Test Server 1",
					Secret:      "12345678",
					CountryCode: "CN",
					Group:       "production",
				},
			},
			wantError: false,
		},
		{
			name:      "空服务器列表",
			servers:   []*config.ServerConfig{},
			wantError: true,
		},
		{
			name: "重复的服务器ID",
			servers: []*config.ServerConfig{
				{Id: "server-1", Name: "Server 1", Secret: "12345678"},
				{Id: "server-1", Name: "Server 2", Secret: "87654321"},
			},
			wantError: true,
		},
		{
			name: "空ID",
			servers: []*config.ServerConfig{
				{Id: "", Name: "Server", Secret: "12345678"},
			},
			wantError: true,
		},
		{
			name: "ID过短",
			servers: []*config.ServerConfig{
				{Id: "ab", Name: "Server", Secret: "12345678"},
			},
			wantError: true,
		},
		{
			name: "ID过长",
			servers: []*config.ServerConfig{
				{Id: "this-is-a-very-long-server-id-that-exceeds-fifty-characters-limit", Name: "Server", Secret: "12345678"},
			},
			wantError: true,
		},
		{
			name: "无效ID格式",
			servers: []*config.ServerConfig{
				{Id: "server@123", Name: "Server", Secret: "12345678"},
			},
			wantError: true,
		},
		{
			name: "空名称",
			servers: []*config.ServerConfig{
				{Id: "server-1", Name: "", Secret: "12345678"},
			},
			wantError: true,
		},
		{
			name: "空密钥",
			servers: []*config.ServerConfig{
				{Id: "server-1", Name: "Server", Secret: ""},
			},
			wantError: true,
		},
		{
			name: "弱密钥",
			servers: []*config.ServerConfig{
				{Id: "server-1", Name: "Server", Secret: "123456"},
			},
			wantError: false, // 弱密钥是警告，不是错误
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cv := NewConfigValidator()
			cv.validateServers(tt.servers)
			hasError := cv.hasErrors()
			if hasError != tt.wantError {
				t.Errorf("%s: 期望错误=%v，实际错误=%v", tt.name, tt.wantError, hasError)
				t.Logf("错误列表: %+v", cv.errors)
			}
		})
	}
}

// TestValidateConfig 测试完整配置验证
func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name      string
		config    *config.DashboardConfig
		wantError bool
	}{
		{
			name: "有效配置",
			config: &config.DashboardConfig{
				Port:                  8900,
				Address:               "0.0.0.0",
				WebSocketPath:         "/ws-report",
				ReportTimeIntervalMax: 30,
				LogPath:               "",
				LogLevel:              "info",
				Servers: []*config.ServerConfig{
					{
						Id:     "server-1",
						Name:   "Test Server",
						Secret: "test-secret-key",
					},
				},
			},
			wantError: false,
		},
		{
			name: "无服务器配置",
			config: &config.DashboardConfig{
				Port:          8900,
				Address:       "0.0.0.0",
				WebSocketPath: "/ws-report",
				Servers:       []*config.ServerConfig{},
			},
			wantError: true,
		},
		{
			name: "无效端口",
			config: &config.DashboardConfig{
				Port:          70000,
				Address:       "0.0.0.0",
				WebSocketPath: "/ws-report",
				Servers: []*config.ServerConfig{
					{Id: "server-1", Name: "Server", Secret: "12345678"},
				},
			},
			wantError: true,
		},
		{
			name: "无效地址",
			config: &config.DashboardConfig{
				Port:          8900,
				Address:       "invalid-ip",
				WebSocketPath: "/ws-report",
				Servers: []*config.ServerConfig{
					{Id: "server-1", Name: "Server", Secret: "12345678"},
				},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cv := NewConfigValidator()
			err := cv.ValidateConfig(tt.config)
			if (err != nil) != tt.wantError {
				t.Errorf("%s: 期望错误=%v，实际错误=%v", tt.name, tt.wantError, err != nil)
				if err != nil {
					t.Logf("错误信息: %v", err)
				}
			}
		})
	}
}

// TestApplyDefaultValues 测试默认值应用
func TestApplyDefaultValues(t *testing.T) {
	cfg := &config.DashboardConfig{}

	applyDefaultValues(cfg)

	tests := []struct {
		name     string
		got      interface{}
		expected interface{}
	}{
		{"默认端口", cfg.Port, 8900},
		{"默认地址", cfg.Address, "0.0.0.0"},
		{"默认WebSocket路径", cfg.WebSocketPath, "/ws-report"},
		{"默认上报间隔", cfg.ReportTimeIntervalMax, 30},
		{"默认日志路径", cfg.LogPath, "./.logs/sss-dashboard.log"},
		{"默认日志级别", cfg.LogLevel, "info"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("%s: 期望 %v，实际 %v", tt.name, tt.expected, tt.got)
			}
		})
	}
}

// TestGetErrorsByLevel 测试按级别获取错误
func TestGetErrorsByLevel(t *testing.T) {
	cv := NewConfigValidator()

	cv.addError("field1", "value1", "错误消息", "error")
	cv.addError("field2", "value2", "警告消息", "warning")
	cv.addError("field3", "value3", "信息消息", "info")
	cv.addError("field4", "value4", "另一个错误", "error")

	errorLevel := cv.GetErrorsByLevel("error")
	if len(errorLevel) != 2 {
		t.Errorf("期望 2 个错误级别，实际 %d 个", len(errorLevel))
	}

	warningLevel := cv.GetErrorsByLevel("warning")
	if len(warningLevel) != 1 {
		t.Errorf("期望 1 个警告级别，实际 %d 个", len(warningLevel))
	}

	infoLevel := cv.GetErrorsByLevel("info")
	if len(infoLevel) != 1 {
		t.Errorf("期望 1 个信息级别，实际 %d 个", len(infoLevel))
	}
}

// TestHasErrors 测试错误检查
func TestHasErrors(t *testing.T) {
	tests := []struct {
		name      string
		errors    []ConfigValidationError
		wantError bool
	}{
		{
			name:      "无错误",
			errors:    []ConfigValidationError{},
			wantError: false,
		},
		{
			name: "仅警告",
			errors: []ConfigValidationError{
				{Level: "warning"},
			},
			wantError: false,
		},
		{
			name: "有错误",
			errors: []ConfigValidationError{
				{Level: "error"},
			},
			wantError: true,
		},
		{
			name: "混合错误和警告",
			errors: []ConfigValidationError{
				{Level: "warning"},
				{Level: "error"},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cv := NewConfigValidator()
			cv.errors = tt.errors
			if cv.hasErrors() != tt.wantError {
				t.Errorf("%s: 期望错误=%v，实际错误=%v", tt.name, tt.wantError, cv.hasErrors())
			}
		})
	}
}

// TestGetErrorSummary 测试错误摘要
func TestGetErrorSummary(t *testing.T) {
	cv := NewConfigValidator()

	cv.addError("Port", "70000", "端口号不能超过65535", "error")
	cv.addError("Address", "invalid", "无效的IP地址格式", "error")
	cv.addError("LogLevel", "trace", "无效的日志级别", "warning") // 警告不应出现在摘要中

	summary := cv.getErrorSummary()

	if summary == "" {
		t.Error("期望非空错误摘要")
	}

	// 检查错误消息是否包含在摘要中
	if !contains(summary, "Port") {
		t.Error("错误摘要应包含 Port 字段")
	}
	if !contains(summary, "Address") {
		t.Error("错误摘要应包含 Address 字段")
	}
	if contains(summary, "LogLevel") {
		t.Error("错误摘要不应包含警告级别的 LogLevel")
	}
}

// TestCheckWritePermission 测试写入权限检查
func TestCheckWritePermission(t *testing.T) {
	cv := NewConfigValidator()

	// 测试临时目录（应该可写）
	tmpDir := t.TempDir()
	if err := cv.checkWritePermission(tmpDir); err != nil {
		t.Errorf("临时目录应该可写: %v", err)
	}

	// 测试不存在的目录
	if err := cv.checkWritePermission("/nonexistent/directory"); err == nil {
		t.Error("不存在的目录检查应该返回错误")
	}
}

// 辅助函数
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) >= len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
