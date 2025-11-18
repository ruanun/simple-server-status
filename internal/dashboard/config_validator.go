package internal

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/ruanun/simple-server-status/internal/dashboard/config"
)

// ConfigValidator 配置验证器
type ConfigValidator struct {
	validator *validator.Validate
	errors    []ConfigValidationError
}

// ConfigValidationError 配置验证错误
type ConfigValidationError struct {
	Field   string `json:"field"`
	Value   string `json:"value"`
	Message string `json:"message"`
	Level   string `json:"level"` // error, warning, info
}

// NewConfigValidator 创建新的配置验证器
func NewConfigValidator() *ConfigValidator {
	v := validator.New()
	cv := &ConfigValidator{
		validator: v,
		errors:    make([]ConfigValidationError, 0),
	}

	// 注册自定义验证规则
	cv.registerCustomValidators()

	return cv
}

// registerCustomValidators 注册自定义验证规则
func (cv *ConfigValidator) registerCustomValidators() {
	// 验证端口范围
	if err := cv.validator.RegisterValidation("port_range", func(fl validator.FieldLevel) bool {
		port := fl.Field().Int()
		return port > 0 && port <= 65535
	}); err != nil {
		panic(fmt.Sprintf("注册 port_range 验证器失败: %v", err))
	}

	// 验证IP地址
	if err := cv.validator.RegisterValidation("ip_address", func(fl validator.FieldLevel) bool {
		ip := fl.Field().String()
		if ip == "" {
			return true // 允许空值，使用默认值
		}
		return net.ParseIP(ip) != nil
	}); err != nil {
		panic(fmt.Sprintf("注册 ip_address 验证器失败: %v", err))
	}

	// 验证路径格式
	if err := cv.validator.RegisterValidation("path_format", func(fl validator.FieldLevel) bool {
		path := fl.Field().String()
		if path == "" {
			return true // 允许空值
		}
		// 检查路径是否包含非法字符
		invalidChars := regexp.MustCompile(`[<>:"|?*]`)
		return !invalidChars.MatchString(path)
	}); err != nil {
		panic(fmt.Sprintf("注册 path_format 验证器失败: %v", err))
	}

	// 验证服务器ID格式
	if err := cv.validator.RegisterValidation("server_id", func(fl validator.FieldLevel) bool {
		id := fl.Field().String()
		if len(id) < 3 || len(id) > 50 {
			return false
		}
		// 只允许字母、数字、下划线和连字符
		validID := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
		return validID.MatchString(id)
	}); err != nil {
		panic(fmt.Sprintf("注册 server_id 验证器失败: %v", err))
	}

	// 验证密钥强度
	if err := cv.validator.RegisterValidation("secret_strength", func(fl validator.FieldLevel) bool {
		secret := fl.Field().String()
		return len(secret) >= 8 // 最少8位
	}); err != nil {
		panic(fmt.Sprintf("注册 secret_strength 验证器失败: %v", err))
	}

	// 验证日志级别
	if err := cv.validator.RegisterValidation("log_level", func(fl validator.FieldLevel) bool {
		level := fl.Field().String()
		if level == "" {
			return true // 允许空值
		}
		validLevels := []string{"debug", "info", "warn", "error", "fatal"}
		for _, validLevel := range validLevels {
			if strings.ToLower(level) == validLevel {
				return true
			}
		}
		return false
	}); err != nil {
		panic(fmt.Sprintf("注册 log_level 验证器失败: %v", err))
	}
}

// ValidateConfig 验证配置
func (cv *ConfigValidator) ValidateConfig(cfg *config.DashboardConfig) error {
	cv.errors = make([]ConfigValidationError, 0)

	// 基础结构验证
	if err := cv.validator.Struct(cfg); err != nil {
		if validationErrors, ok := err.(validator.ValidationErrors); ok {
			for _, fieldError := range validationErrors {
				cv.addError(fieldError.Field(), fmt.Sprintf("%v", fieldError.Value()), cv.getErrorMessage(fieldError), "error")
			}
		}
	}

	// 自定义验证规则
	cv.validatePort(cfg.Port)
	cv.validateAddress(cfg.Address)
	cv.validateWebSocketPath(cfg.WebSocketPath)
	cv.validateReportInterval(cfg.ReportTimeIntervalMax)
	cv.validateLogConfig(cfg.LogPath, cfg.LogLevel)
	cv.validateServers(cfg.Servers)

	// 检查是否有错误
	if cv.hasErrors() {
		return fmt.Errorf("配置验证失败: %s", cv.getErrorSummary())
	}

	return nil
}

// validatePort 验证端口配置
func (cv *ConfigValidator) validatePort(port int) {
	if port <= 0 {
		cv.addError("Port", strconv.Itoa(port), "端口必须大于0，将使用默认值8900", "warning")
	} else if port <= 1024 {
		cv.addError("Port", strconv.Itoa(port), "使用系统端口(<=1024)可能需要管理员权限", "warning")
	} else if port > 65535 {
		cv.addError("Port", strconv.Itoa(port), "端口号不能超过65535", "error")
	}

	// 检查端口是否被占用
	if port > 0 && port <= 65535 {
		if cv.isPortInUse(port) {
			cv.addError("Port", strconv.Itoa(port), fmt.Sprintf("端口%d可能已被占用", port), "warning")
		}
	}
}

// validateAddress 验证地址配置
func (cv *ConfigValidator) validateAddress(address string) {
	if address == "" {
		cv.addError("Address", address, "地址为空，将使用默认值0.0.0.0", "info")
		return
	}

	if net.ParseIP(address) == nil {
		cv.addError("Address", address, "无效的IP地址格式", "error")
	}
}

// validateWebSocketPath 验证WebSocket路径
func (cv *ConfigValidator) validateWebSocketPath(path string) {
	if path == "" {
		cv.addError("WebSocketPath", path, "WebSocket路径为空，将使用默认值/ws-report", "info")
		return
	}

	// 为保持向后兼容，降级为警告而非错误
	// 实际的路径规范化会在 applyDefaultValues 中自动处理
	if !strings.HasPrefix(path, "/") {
		cv.addError("WebSocketPath", path, "WebSocket路径建议以'/'开头，系统将自动添加", "warning")
	}

	if strings.Contains(path, " ") {
		cv.addError("WebSocketPath", path, "WebSocket路径不应包含空格", "error")
	}
}

// validateReportInterval 验证上报间隔
func (cv *ConfigValidator) validateReportInterval(interval int) {
	if interval < 5 {
		cv.addError("ReportTimeIntervalMax", strconv.Itoa(interval), "上报间隔最小值为5秒，将使用默认值30秒", "warning")
	} else if interval > 300 {
		cv.addError("ReportTimeIntervalMax", strconv.Itoa(interval), "上报间隔过长(>300秒)可能导致监控延迟", "warning")
	}
}

// validateLogConfig 验证日志配置
func (cv *ConfigValidator) validateLogConfig(logPath, logLevel string) {
	// 验证日志路径
	if logPath != "" {
		dir := filepath.Dir(logPath)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			cv.addError("LogPath", logPath, fmt.Sprintf("日志目录不存在: %s", dir), "warning")
		}

		// 检查写入权限
		if cv.checkWritePermission(dir) != nil {
			cv.addError("LogPath", logPath, fmt.Sprintf("日志目录无写入权限: %s", dir), "error")
		}
	}

	// 验证日志级别
	if logLevel != "" {
		validLevels := []string{"debug", "info", "warn", "error", "fatal"}
		valid := false
		for _, level := range validLevels {
			if strings.ToLower(logLevel) == level {
				valid = true
				break
			}
		}
		if !valid {
			cv.addError("LogLevel", logLevel, fmt.Sprintf("无效的日志级别，有效值: %s", strings.Join(validLevels, ", ")), "error")
		}
	}
}

// validateServers 验证服务器配置
func (cv *ConfigValidator) validateServers(servers []*config.ServerConfig) {
	if len(servers) == 0 {
		cv.addError("Servers", "[]", "未配置任何服务器", "error")
		return
	}

	serverIDs := make(map[string]bool)
	serverNames := make(map[string]bool)

	for i, server := range servers {
		prefix := fmt.Sprintf("Servers[%d]", i)

		// 验证服务器ID
		if server.Id == "" {
			cv.addError(prefix+".Id", server.Id, "服务器ID不能为空", "error")
		} else {
			if serverIDs[server.Id] {
				cv.addError(prefix+".Id", server.Id, "服务器ID重复", "error")
			}
			serverIDs[server.Id] = true

			// 验证ID格式
			if len(server.Id) < 3 || len(server.Id) > 50 {
				cv.addError(prefix+".Id", server.Id, "服务器ID长度应在3-50字符之间", "error")
			}

			validID := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
			if !validID.MatchString(server.Id) {
				cv.addError(prefix+".Id", server.Id, "服务器ID只能包含字母、数字、下划线和连字符", "error")
			}
		}

		// 验证服务器名称
		if server.Name == "" {
			cv.addError(prefix+".Name", server.Name, "服务器名称不能为空", "error")
		} else {
			if serverNames[server.Name] {
				cv.addError(prefix+".Name", server.Name, "服务器名称重复，建议使用唯一名称", "warning")
			}
			serverNames[server.Name] = true

			if len(server.Name) > 100 {
				cv.addError(prefix+".Name", server.Name, "服务器名称过长(>100字符)", "warning")
			}
		}

		// 验证密钥
		if server.Secret == "" {
			cv.addError(prefix+".Secret", server.Secret, "服务器密钥不能为空", "error")
		} else {
			if len(server.Secret) < 8 {
				cv.addError(prefix+".Secret", "***", "密钥长度应至少8位以确保安全性", "warning")
			}
			if server.Secret == "123456" || server.Secret == "password" || server.Secret == "admin" {
				cv.addError(prefix+".Secret", "***", "使用弱密钥，建议使用更复杂的密钥", "warning")
			}
		}

		// 验证国家代码
		if server.CountryCode != "" {
			if len(server.CountryCode) != 2 {
				cv.addError(prefix+".CountryCode", server.CountryCode, "国家代码应为2位字母(如: CN, US, JP)", "warning")
			}
		}

		// 验证分组
		if server.Group != "" && len(server.Group) > 50 {
			cv.addError(prefix+".Group", server.Group, "分组名称过长(>50字符)", "warning")
		}
	}
}

// 辅助方法
func (cv *ConfigValidator) addError(field, value, message, level string) {
	cv.errors = append(cv.errors, ConfigValidationError{
		Field:   field,
		Value:   value,
		Message: message,
		Level:   level,
	})
}

func (cv *ConfigValidator) hasErrors() bool {
	for _, err := range cv.errors {
		if err.Level == "error" {
			return true
		}
	}
	return false
}

func (cv *ConfigValidator) getErrorSummary() string {
	var errors []string
	for _, err := range cv.errors {
		if err.Level == "error" {
			errors = append(errors, fmt.Sprintf("%s: %s", err.Field, err.Message))
		}
	}
	return strings.Join(errors, "; ")
}

func (cv *ConfigValidator) getErrorMessage(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "此字段为必填项"
	case "port_range":
		return "端口号必须在1-65535范围内"
	case "ip_address":
		return "无效的IP地址格式"
	case "path_format":
		return "路径格式无效"
	case "server_id":
		return "服务器ID格式无效(3-50字符，仅允许字母数字下划线连字符)"
	case "secret_strength":
		return "密钥强度不足(至少8位)"
	case "log_level":
		return "无效的日志级别"
	default:
		return fmt.Sprintf("验证失败: %s", fe.Tag())
	}
}

func (cv *ConfigValidator) isPortInUse(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf(":%d", port), time.Second)
	if err != nil {
		return false
	}
	_ = conn.Close() // 忽略关闭错误，仅用于端口检测
	return true
}

func (cv *ConfigValidator) checkWritePermission(dir string) error {
	// 使用 filepath.Join 安全构建路径，防止路径遍历攻击
	testFile := filepath.Join(dir, ".write_test")
	//nolint:gosec // G304: 这是配置验证的内部函数，dir 来自配置文件，用于测试目录写权限
	file, err := os.Create(testFile)
	if err != nil {
		return err
	}
	_ = file.Close()        // 忽略关闭错误，文件即将被删除
	_ = os.Remove(testFile) // 忽略删除错误，这只是清理临时文件
	return nil
}

// GetValidationErrors 获取所有验证错误
func (cv *ConfigValidator) GetValidationErrors() []ConfigValidationError {
	return cv.errors
}

// GetErrorsByLevel 按级别获取错误
func (cv *ConfigValidator) GetErrorsByLevel(level string) []ConfigValidationError {
	var result []ConfigValidationError
	for _, err := range cv.errors {
		if err.Level == level {
			result = append(result, err)
		}
	}
	return result
}

// PrintValidationReport 打印验证报告
func (cv *ConfigValidator) PrintValidationReport() {
	if len(cv.errors) == 0 {
		fmt.Println("[INFO] 配置验证通过，无错误或警告")
		return
	}

	errorCount := len(cv.GetErrorsByLevel("error"))
	warningCount := len(cv.GetErrorsByLevel("warning"))
	infoCount := len(cv.GetErrorsByLevel("info"))

	fmt.Printf("[INFO] 配置验证完成 - 错误: %d, 警告: %d, 信息: %d\n", errorCount, warningCount, infoCount)

	for _, err := range cv.errors {
		switch err.Level {
		case "error":
			fmt.Printf("[ERROR] [配置错误] %s: %s\n", err.Field, err.Message)
		case "warning":
			fmt.Printf("[WARN] [配置警告] %s: %s\n", err.Field, err.Message)
		case "info":
			fmt.Printf("[INFO] [配置信息] %s: %s\n", err.Field, err.Message)
		}
	}
}

// ValidateAndApplyDefaults 验证配置并应用默认值
func ValidateAndApplyDefaults(cfg *config.DashboardConfig) error {
	validator := NewConfigValidator()

	// 先应用默认值
	applyDefaultValues(cfg)

	// 然后进行验证
	err := validator.ValidateConfig(cfg)

	// 始终打印验证报告（内部会自动选择合适的日志记录器）
	validator.PrintValidationReport()

	return err
}

// applyDefaultValues 应用默认值
func applyDefaultValues(cfg *config.DashboardConfig) {
	if cfg.Port == 0 {
		cfg.Port = 8900
	}
	if cfg.Address == "" {
		cfg.Address = "0.0.0.0"
	}
	if cfg.WebSocketPath == "" {
		cfg.WebSocketPath = "/ws-report"
	} else if !strings.HasPrefix(cfg.WebSocketPath, "/") {
		// 为保持向后兼容，自动添加前导斜杠
		// 兼容旧配置格式如 "ws-report" -> "/ws-report"
		cfg.WebSocketPath = "/" + cfg.WebSocketPath
	}
	if cfg.ReportTimeIntervalMax < 5 {
		cfg.ReportTimeIntervalMax = 30
	}
	if cfg.LogPath == "" {
		cfg.LogPath = "./.logs/sss-dashboard.log"
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}

	// 为服务器配置应用默认值
	for _, server := range cfg.Servers {
		if server.Group == "" {
			server.Group = "DEFAULT"
		}
	}
}
