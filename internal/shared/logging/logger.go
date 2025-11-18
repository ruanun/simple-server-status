package logging

import (
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Config 日志配置
type Config struct {
	Level     string // 日志级别: debug, info, warn, error, dpanic, panic, fatal
	FilePath  string // 日志文件路径
	MaxSize   int    // 单个日志文件最大大小(MB)
	MaxAge    int    // 日志文件保留天数
	Compress  bool   // 是否压缩旧日志
	LocalTime bool   // 是否使用本地时间
}

// DefaultConfig 返回默认日志配置
func DefaultConfig() Config {
	return Config{
		Level:     "info",
		FilePath:  "",
		MaxSize:   64,
		MaxAge:    5,
		Compress:  false,
		LocalTime: true,
	}
}

// LevelMap 日志级别映射
var LevelMap = map[string]zapcore.Level{
	"debug":  zapcore.DebugLevel,
	"info":   zapcore.InfoLevel,
	"warn":   zapcore.WarnLevel,
	"error":  zapcore.ErrorLevel,
	"dpanic": zapcore.DPanicLevel,
	"panic":  zapcore.PanicLevel,
	"fatal":  zapcore.FatalLevel,
}

// New 创建新的日志实例
func New(cfg Config) (*zap.SugaredLogger, error) {
	// 解析日志级别
	level, ok := LevelMap[cfg.Level]
	if !ok {
		level = zapcore.InfoLevel
	}

	atomicLevel := zap.NewAtomicLevelAt(level)
	core := zapcore.NewCore(
		getEncoder(),
		getLogWriter(cfg),
		atomicLevel,
	)

	logger := zap.New(core, zap.AddCaller())
	sugaredLogger := logger.Sugar()

	sugaredLogger.Infof("日志模块初始化成功 [level=%s, file=%s]", cfg.Level, cfg.FilePath)
	return sugaredLogger, nil
}

// getLogWriter 获取日志输出器
func getLogWriter(cfg Config) zapcore.WriteSyncer {
	writers := []zapcore.WriteSyncer{
		zapcore.AddSync(os.Stdout), // 始终输出到控制台
	}

	// 如果指定了日志文件路径，则同时输出到文件
	if cfg.FilePath != "" {
		writers = append(writers, zapcore.AddSync(&lumberjack.Logger{
			Filename:  cfg.FilePath,
			MaxSize:   cfg.MaxSize,
			MaxAge:    cfg.MaxAge,
			LocalTime: cfg.LocalTime,
			Compress:  cfg.Compress,
		}))
	}

	return zapcore.NewMultiWriteSyncer(writers...)
}

// getEncoder 获取日志编码器
func getEncoder() zapcore.Encoder {
	// 自定义时间格式
	customTimeEncoder := func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.Format("2006-01-02 15:04:05.000"))
	}

	// 自定义代码路径、行号输出
	customCallerEncoder := func(caller zapcore.EntryCaller, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString("[" + caller.TrimmedPath() + "]")
	}

	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = customTimeEncoder
	encoderConfig.TimeKey = "time"
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	encoderConfig.EncodeDuration = zapcore.SecondsDurationEncoder
	encoderConfig.EncodeCaller = customCallerEncoder

	return zapcore.NewConsoleEncoder(encoderConfig)
}
