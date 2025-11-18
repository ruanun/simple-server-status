package app

import (
	"fmt"

	"github.com/ruanun/simple-server-status/internal/shared/logging"
	"go.uber.org/zap"
)

// LoggerConfig 日志配置
type LoggerConfig struct {
	Level     string
	FilePath  string
	MaxSize   int
	MaxAge    int
	Compress  bool
	LocalTime bool
}

// DefaultLoggerConfig 返回默认日志配置
func DefaultLoggerConfig() LoggerConfig {
	return LoggerConfig{
		MaxSize:   64,
		MaxAge:    5,
		Compress:  true,
		LocalTime: true,
	}
}

// InitLogger 初始化日志器
func InitLogger(level, filePath string) (*zap.SugaredLogger, error) {
	cfg := DefaultLoggerConfig()
	cfg.Level = level
	cfg.FilePath = filePath

	logger, err := logging.New(logging.Config{
		Level:     cfg.Level,
		FilePath:  cfg.FilePath,
		MaxSize:   cfg.MaxSize,
		MaxAge:    cfg.MaxAge,
		Compress:  cfg.Compress,
		LocalTime: cfg.LocalTime,
	})

	if err != nil {
		return nil, fmt.Errorf("初始化日志失败: %w", err)
	}

	return logger, nil
}
