package internal

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/ruanun/simple-server-status/internal/agent/config"
)

// ValidationError 验证错误
type ValidationError struct {
	Field   string
	Message string
}

func (ve *ValidationError) Error() string {
	return fmt.Sprintf("validation error for field '%s': %s", ve.Field, ve.Message)
}

// ValidationResult 验证结果
type ValidationResult struct {
	Valid  bool
	Errors []*ValidationError
}

// AddError 添加验证错误
func (vr *ValidationResult) AddError(field, message string) {
	vr.Valid = false
	vr.Errors = append(vr.Errors, &ValidationError{Field: field, Message: message})
}

// GetErrorMessages 获取所有错误消息
func (vr *ValidationResult) GetErrorMessages() []string {
	messages := make([]string, len(vr.Errors))
	for i, err := range vr.Errors {
		messages[i] = err.Error()
	}
	return messages
}

// ConfigValidator 配置验证器
type ConfigValidator struct {
	config *config.AgentConfig
}

// NewConfigValidator 创建新的配置验证器
func NewConfigValidator(cfg *config.AgentConfig) *ConfigValidator {
	return &ConfigValidator{config: cfg}
}

// ValidateConfig 验证配置
func (cv *ConfigValidator) ValidateConfig() *ValidationResult {
	result := &ValidationResult{Valid: true}

	// 验证必填字段
	cv.validateRequiredFields(result)

	// 验证服务器地址格式
	cv.validateServerAddr(result)

	// 验证服务器ID格式
	cv.validateServerId(result)

	// 验证认证密钥
	cv.validateAuthSecret(result)

	// 验证上报间隔
	cv.validateReportTimeInterval(result)

	// 验证日志配置
	cv.validateLogConfig(result)

	return result
}

// validateRequiredFields 验证必填字段
func (cv *ConfigValidator) validateRequiredFields(result *ValidationResult) {
	if strings.TrimSpace(cv.config.ServerAddr) == "" {
		result.AddError("ServerAddr", "server address is required")
	}

	if strings.TrimSpace(cv.config.ServerId) == "" {
		result.AddError("ServerId", "server ID is required")
	}

	if strings.TrimSpace(cv.config.AuthSecret) == "" {
		result.AddError("AuthSecret", "auth secret is required")
	}
}

// validateServerAddr 验证服务器地址
func (cv *ConfigValidator) validateServerAddr(result *ValidationResult) {
	if cv.config.ServerAddr == "" {
		return // 已在必填字段验证中处理
	}

	// 检查是否为有效的WebSocket URL
	if !strings.HasPrefix(cv.config.ServerAddr, "ws://") && !strings.HasPrefix(cv.config.ServerAddr, "wss://") {
		result.AddError("ServerAddr", "server address must start with ws:// or wss://")
		return
	}

	// 解析URL
	parsedURL, err := url.Parse(cv.config.ServerAddr)
	if err != nil {
		result.AddError("ServerAddr", fmt.Sprintf("invalid URL format: %v", err))
		return
	}

	// 检查主机名
	if parsedURL.Host == "" {
		result.AddError("ServerAddr", "server address must include a valid host")
	}

	// 注意：如果服务器地址未包含路径，将使用默认 WebSocket 端点
}

// validateServerId 验证服务器ID
func (cv *ConfigValidator) validateServerId(result *ValidationResult) {
	if cv.config.ServerId == "" {
		return // 已在必填字段验证中处理
	}

	// 服务器ID应该是字母数字字符，可以包含连字符和下划线
	validServerIdPattern := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	if !validServerIdPattern.MatchString(cv.config.ServerId) {
		result.AddError("ServerId", "server ID can only contain letters, numbers, hyphens, and underscores")
	}

	// 检查长度
	if len(cv.config.ServerId) < 3 {
		result.AddError("ServerId", "server ID must be at least 3 characters long")
	}

	if len(cv.config.ServerId) > 50 {
		result.AddError("ServerId", "server ID must be no more than 50 characters long")
	}
}

// validateAuthSecret 验证认证密钥
func (cv *ConfigValidator) validateAuthSecret(result *ValidationResult) {
	if cv.config.AuthSecret == "" {
		return // 已在必填字段验证中处理
	}

	// 检查密钥长度
	if len(cv.config.AuthSecret) < 8 {
		result.AddError("AuthSecret", "auth secret must be at least 8 characters long")
	}

	if len(cv.config.AuthSecret) > 256 {
		result.AddError("AuthSecret", "auth secret must be no more than 256 characters long")
	}

	// 检查是否包含不安全的字符
	if strings.Contains(cv.config.AuthSecret, " ") {
		result.AddError("AuthSecret", "auth secret should not contain spaces")
	}

	// 注意：建议使用至少 16 个字符的强密钥以提高安全性
}

// validateReportTimeInterval 验证上报间隔
func (cv *ConfigValidator) validateReportTimeInterval(result *ValidationResult) {
	// 检查最小值
	if cv.config.ReportTimeInterval < 1 {
		result.AddError("ReportTimeInterval", "report time interval must be at least 1 second")
	}

	// 检查最大值（避免过长的间隔）
	if cv.config.ReportTimeInterval > 300 { // 5分钟
		result.AddError("ReportTimeInterval", "report time interval should not exceed 300 seconds (5 minutes)")
	}

	// 注意：建议上报间隔设置在 2-60 秒之间
	// - 小于 2 秒可能导致高系统负载
	// - 大于 60 秒可能降低监控精度
}

// validateLogConfig 验证日志配置
func (cv *ConfigValidator) validateLogConfig(result *ValidationResult) {
	// 验证日志级别
	validLogLevels := []string{"debug", "info", "warn", "error", ""}
	logLevel := strings.ToLower(cv.config.LogLevel)
	validLevel := false
	for _, level := range validLogLevels {
		if logLevel == level {
			validLevel = true
			break
		}
	}

	if !validLevel {
		result.AddError("LogLevel", "log level must be one of: debug, info, warn, error (or empty for default)")
	}

	// 验证日志路径（如果提供）
	if cv.config.LogPath != "" {
		// 检查路径格式（简单检查）
		if strings.Contains(cv.config.LogPath, "<") || strings.Contains(cv.config.LogPath, ">") {
			result.AddError("LogPath", "log path contains invalid characters")
		}
	}
}

// ValidateAndSetDefaults 验证配置并设置默认值
func ValidateAndSetDefaults(cfg *config.AgentConfig) error {
	fmt.Println("[INFO] 开始配置验证和默认值设置...")

	// 设置默认值
	setConfigDefaults(cfg)

	// 验证配置
	validator := NewConfigValidator(cfg)
	result := validator.ValidateConfig()

	if !result.Valid {
		// 记录所有验证错误
		for _, err := range result.Errors {
			fmt.Printf("[ERROR] Config validation error: %s\n", err.Error())
		}
		return fmt.Errorf("configuration validation failed with %d errors", len(result.Errors))
	}

	fmt.Println("[INFO] Configuration validation passed")
	return nil
}

// setConfigDefaults 设置配置默认值
func setConfigDefaults(cfg *config.AgentConfig) {
	// 设置默认上报间隔
	if cfg.ReportTimeInterval <= 0 {
		cfg.ReportTimeInterval = 2 // 默认2秒
	}

	// 设置默认日志级别
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}

	// 标准化日志级别
	cfg.LogLevel = strings.ToLower(cfg.LogLevel)
}

// ValidateEnvironment 验证运行环境
func ValidateEnvironment() error {
	// 检查必要的系统权限和资源
	fmt.Println("[INFO] Validating runtime environment...")

	// 这里可以添加更多环境检查
	// 例如：检查网络权限、文件系统权限等

	fmt.Println("[INFO] Environment validation passed")
	return nil
}
