package handler

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ruanun/simple-server-status/internal/dashboard/config"
	"github.com/ruanun/simple-server-status/internal/dashboard/response"
)

// ConfigProvider 配置提供者接口
type ConfigProvider interface {
	GetConfig() *config.DashboardConfig
}

// LoggerProvider 日志提供者接口
type LoggerProvider interface {
	Info(...interface{})
	Infof(string, ...interface{})
}

// ConfigValidationError 配置验证错误（从 internal 包复制以避免循环依赖）
type ConfigValidationError struct {
	Field   string `json:"field"`
	Value   string `json:"value"`
	Message string `json:"message"`
	Level   string `json:"level"` // error, warning, info
}

// ConfigValidatorProvider 配置验证器提供者接口
type ConfigValidatorProvider interface {
	ValidateConfig(cfg *config.DashboardConfig) error
	GetValidationErrors() []ConfigValidationError
	GetErrorsByLevel(level string) []ConfigValidationError
}

// InitConfigAPI 初始化配置相关API
func InitConfigAPI(group *gin.RouterGroup, configProvider ConfigProvider, logger LoggerProvider, validator ConfigValidatorProvider) {
	// 配置验证状态
	group.GET("/config/validation", getConfigValidation(validator, configProvider))
	// 配置信息（脱敏）
	group.GET("/config/info", getConfigInfo(configProvider))
	// 重新验证配置
	group.POST("/config/validate", validateConfig(validator, configProvider, logger))
}

// getConfigValidation 获取配置验证状态
func getConfigValidation(validator ConfigValidatorProvider, configProvider ConfigProvider) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取当前配置
		cfg := configProvider.GetConfig()

		// 执行验证（忽略错误，因为验证结果通过 GetValidationErrors 获取）
		_ = validator.ValidateConfig(cfg)

		// 获取验证错误
		errors := validator.GetValidationErrors()

		// 统计各级别错误数量
		errorCount := len(validator.GetErrorsByLevel("error"))
		warningCount := len(validator.GetErrorsByLevel("warning"))
		infoCount := len(validator.GetErrorsByLevel("info"))

		// 返回验证结果
		data := gin.H{
			"valid":         errorCount == 0,
			"errors":        errors,
			"error_count":   errorCount,
			"warning_count": warningCount,
			"info_count":    infoCount,
		}

		response.Success(c, data)
	}
}

// getConfigInfo 获取配置信息（脱敏）
func getConfigInfo(configProvider ConfigProvider) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取配置（线程安全）
		cfg := configProvider.GetConfig()

		// 创建脱敏的配置信息
		configInfo := gin.H{
			"address":               cfg.Address,
			"port":                  cfg.Port,
			"debug":                 cfg.Debug,
			"webSocketPath":         cfg.WebSocketPath,
			"reportTimeIntervalMax": cfg.ReportTimeIntervalMax,
			"logPath":               cfg.LogPath,
			"logLevel":              cfg.LogLevel,
			"serverCount":           len(cfg.Servers),
		}

		// 脱敏的服务器信息
		var servers []gin.H
		for _, server := range cfg.Servers {
			servers = append(servers, gin.H{
				"id":           server.Id,
				"name":         server.Name,
				"group":        server.Group,
				"countryCode":  server.CountryCode,
				"hasSecret":    server.Secret != "",
				"secretLength": len(server.Secret),
			})
		}
		configInfo["servers"] = servers

		response.Success(c, configInfo)
	}
}

// validateConfig 重新验证配置
func validateConfig(validator ConfigValidatorProvider, configProvider ConfigProvider, logger LoggerProvider) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取当前配置
		cfg := configProvider.GetConfig()

		// 执行验证
		err := validator.ValidateConfig(cfg)

		// 获取验证错误
		errors := validator.GetValidationErrors()

		// 统计错误数量
		errorCount := len(validator.GetErrorsByLevel("error"))

		// 返回验证结果
		data := gin.H{
			"valid":     errorCount == 0,
			"errors":    errors,
			"timestamp": time.Now(),
		}

		if err != nil {
			logger.Infof("配置验证完成，发现 %d 个错误", errorCount)
		} else {
			logger.Info("配置验证通过")
		}

		response.Success(c, data)
	}
}
