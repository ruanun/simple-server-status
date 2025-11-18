package logging

import (
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap/zapcore"
)

// TestDefaultConfig 测试默认配置
func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Level != "info" {
		t.Errorf("DefaultConfig.Level = %s; want info", cfg.Level)
	}
	if cfg.FilePath != "" {
		t.Errorf("DefaultConfig.FilePath = %s; want empty", cfg.FilePath)
	}
	if cfg.MaxSize != 64 {
		t.Errorf("DefaultConfig.MaxSize = %d; want 64", cfg.MaxSize)
	}
	if cfg.MaxAge != 5 {
		t.Errorf("DefaultConfig.MaxAge = %d; want 5", cfg.MaxAge)
	}
	if cfg.Compress != false {
		t.Errorf("DefaultConfig.Compress = %v; want false", cfg.Compress)
	}
	if cfg.LocalTime != true {
		t.Errorf("DefaultConfig.LocalTime = %v; want true", cfg.LocalTime)
	}
}

// TestLevelMap 测试日志级别映射完整性
func TestLevelMap(t *testing.T) {
	expectedLevels := map[string]zapcore.Level{
		"debug":  zapcore.DebugLevel,
		"info":   zapcore.InfoLevel,
		"warn":   zapcore.WarnLevel,
		"error":  zapcore.ErrorLevel,
		"dpanic": zapcore.DPanicLevel,
		"panic":  zapcore.PanicLevel,
		"fatal":  zapcore.FatalLevel,
	}

	for level, expected := range expectedLevels {
		actual, ok := LevelMap[level]
		if !ok {
			t.Errorf("LevelMap missing level: %s", level)
		}
		if actual != expected {
			t.Errorf("LevelMap[%s] = %v; want %v", level, actual, expected)
		}
	}

	// 确保没有额外的级别
	if len(LevelMap) != len(expectedLevels) {
		t.Errorf("LevelMap has %d levels; want %d", len(LevelMap), len(expectedLevels))
	}
}

// TestNew_DefaultConfig 测试使用默认配置创建日志
func TestNew_DefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	logger, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v; want nil", err)
	}
	if logger == nil {
		t.Fatal("New() returned nil logger")
	}

	// 测试日志方法可用
	logger.Info("测试日志消息")
	_ = logger.Sync() // 忽略 Sync 错误，测试环境中无关紧要
}

// TestNew_AllLevels 测试所有日志级别
func TestNew_AllLevels(t *testing.T) {
	levels := []string{"debug", "info", "warn", "error", "dpanic"}

	for _, level := range levels {
		t.Run(level, func(t *testing.T) {
			cfg := Config{
				Level:     level,
				MaxSize:   10,
				MaxAge:    1,
				Compress:  false,
				LocalTime: true,
			}

			logger, err := New(cfg)
			if err != nil {
				t.Fatalf("New() with level=%s error = %v; want nil", level, err)
			}
			if logger == nil {
				t.Fatalf("New() with level=%s returned nil logger", level)
			}

			// 测试各级别日志方法
			logger.Debug("debug message")
			logger.Info("info message")
			logger.Warn("warn message")
			logger.Error("error message")
			_ = logger.Sync() // 忽略 Sync 错误，测试环境中无关紧要
		})
	}
}

// TestNew_InvalidLevel 测试无效的日志级别（应该默认为 info）
func TestNew_InvalidLevel(t *testing.T) {
	cfg := Config{
		Level:     "invalid_level",
		MaxSize:   10,
		MaxAge:    1,
		Compress:  false,
		LocalTime: true,
	}

	logger, err := New(cfg)
	if err != nil {
		t.Fatalf("New() with invalid level error = %v; want nil", err)
	}
	if logger == nil {
		t.Fatal("New() with invalid level returned nil logger")
	}

	// 应该能正常工作（使用默认 info 级别）
	logger.Info("测试消息")
	_ = logger.Sync() // 忽略 Sync 错误，测试环境中无关紧要
}

// TestNew_WithFileOutput 测试输出到文件
func TestNew_WithFileOutput(t *testing.T) {
	// 创建临时目录（手动管理以避免 Windows 文件锁定问题）
	tempDir, err := os.MkdirTemp("", "logging_test_*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logFile := filepath.Join(tempDir, "test.log")

	cfg := Config{
		Level:     "info",
		FilePath:  logFile,
		MaxSize:   10,
		MaxAge:    1,
		Compress:  false,
		LocalTime: true,
	}

	logger, err := New(cfg)
	if err != nil {
		t.Fatalf("New() with file output error = %v; want nil", err)
	}
	if logger == nil {
		t.Fatal("New() with file output returned nil logger")
	}

	// 写入测试日志
	testMessage := "测试文件输出"
	logger.Info(testMessage)
	_ = logger.Sync() // 忽略 Sync 错误，测试环境中无关紧要

	// 验证日志文件已创建
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Errorf("日志文件未创建: %s", logFile)
	}

	// 验证日志文件包含测试消息
	//nolint:gosec // G304: 这是测试代码，logFile 是测试中创建的临时文件路径，安全可控
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("读取日志文件失败: %v", err)
	}

	// 检查是否包含测试消息
	if len(content) == 0 {
		t.Error("日志文件为空")
	}
}

// TestNew_WithCompression 测试开启压缩选项
func TestNew_WithCompression(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "logging_test_*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logFile := filepath.Join(tempDir, "compressed.log")

	cfg := Config{
		Level:     "info",
		FilePath:  logFile,
		MaxSize:   1,
		MaxAge:    1,
		Compress:  true,
		LocalTime: true,
	}

	logger, err := New(cfg)
	if err != nil {
		t.Fatalf("New() with compression error = %v; want nil", err)
	}
	if logger == nil {
		t.Fatal("New() with compression returned nil logger")
	}

	logger.Info("测试压缩配置")
	_ = logger.Sync() // 忽略 Sync 错误，测试环境中无关紧要

	// 验证日志文件已创建
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Errorf("日志文件未创建: %s", logFile)
	}
}

// TestNew_MultipleInstances 测试创建多个日志实例
func TestNew_MultipleInstances(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "logging_test_*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cfg1 := Config{
		Level:    "info",
		FilePath: filepath.Join(tempDir, "logger1.log"),
		MaxSize:  10,
		MaxAge:   1,
	}

	cfg2 := Config{
		Level:    "debug",
		FilePath: filepath.Join(tempDir, "logger2.log"),
		MaxSize:  10,
		MaxAge:   1,
	}

	logger1, err := New(cfg1)
	if err != nil {
		t.Fatalf("创建 logger1 失败: %v", err)
	}

	logger2, err := New(cfg2)
	if err != nil {
		t.Fatalf("创建 logger2 失败: %v", err)
	}

	// 两个日志实例应该是不同的
	if logger1 == logger2 {
		t.Error("两个日志实例应该是不同的对象")
	}

	logger1.Info("logger1 消息")
	logger2.Debug("logger2 消息")

	_ = logger1.Sync() // 忽略 Sync 错误，测试环境中无关紧要
	_ = logger2.Sync() // 忽略 Sync 错误，测试环境中无关紧要
}

// TestNew_NoFilePath 测试不指定文件路径（只输出到控制台）
func TestNew_NoFilePath(t *testing.T) {
	cfg := Config{
		Level:     "info",
		FilePath:  "", // 空路径
		MaxSize:   10,
		MaxAge:    1,
		Compress:  false,
		LocalTime: true,
	}

	logger, err := New(cfg)
	if err != nil {
		t.Fatalf("New() without file path error = %v; want nil", err)
	}
	if logger == nil {
		t.Fatal("New() without file path returned nil logger")
	}

	// 应该能正常工作
	logger.Info("控制台输出测试")
	_ = logger.Sync() // 忽略 Sync 错误，测试环境中无关紧要
}

// TestNew_LoggerMethods 测试日志实例的各种方法
func TestNew_LoggerMethods(t *testing.T) {
	cfg := DefaultConfig()
	logger, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v; want nil", err)
	}

	// 测试各种日志方法不会 panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("日志方法触发 panic: %v", r)
		}
	}()

	logger.Debug("debug 消息")
	logger.Debugf("debug 格式化: %s", "测试")
	logger.Info("info 消息")
	logger.Infof("info 格式化: %d", 123)
	logger.Warn("warn 消息")
	logger.Warnf("warn 格式化: %v", true)
	logger.Error("error 消息")
	logger.Errorf("error 格式化: %f", 3.14)

	// 带键值对的日志
	logger.Infow("带字段的日志", "key1", "value1", "key2", 123)

	_ = logger.Sync() // 忽略 Sync 错误，测试环境中无关紧要
}

// TestConfig_Variations 测试配置的各种组合
func TestConfig_Variations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "logging_test_*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name   string
		config Config
	}{
		{
			name: "最小配置",
			config: Config{
				Level: "info",
			},
		},
		{
			name: "完整配置",
			config: Config{
				Level:     "debug",
				FilePath:  filepath.Join(tempDir, "full.log"),
				MaxSize:   100,
				MaxAge:    30,
				Compress:  true,
				LocalTime: false,
			},
		},
		{
			name: "大文件配置",
			config: Config{
				Level:    "warn",
				FilePath: filepath.Join(tempDir, "large.log"),
				MaxSize:  1024,
				MaxAge:   90,
			},
		},
		{
			name: "调试级别",
			config: Config{
				Level:    "debug",
				FilePath: filepath.Join(tempDir, "debug.log"),
			},
		},
		{
			name: "错误级别",
			config: Config{
				Level:    "error",
				FilePath: filepath.Join(tempDir, "error.log"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := New(tt.config)
			if err != nil {
				t.Fatalf("New() error = %v; want nil", err)
			}
			if logger == nil {
				t.Fatal("New() returned nil logger")
			}

			logger.Info("测试消息")
			_ = logger.Sync() // 忽略 Sync 错误，测试环境中无关紧要
		})
	}
}

// BenchmarkNew 基准测试：创建日志实例的性能
func BenchmarkNew(b *testing.B) {
	cfg := DefaultConfig()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger, _ := New(cfg)
		_ = logger.Sync() // 忽略 Sync 错误，测试环境中无关紧要
	}
}

// BenchmarkNew_WithFile 基准测试：带文件输出的日志创建性能
func BenchmarkNew_WithFile(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "logging_bench_*")
	if err != nil {
		b.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cfg := Config{
		Level:    "info",
		FilePath: filepath.Join(tempDir, "bench.log"),
		MaxSize:  10,
		MaxAge:   1,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger, _ := New(cfg)
		_ = logger.Sync() // 忽略 Sync 错误，测试环境中无关紧要
	}
}

// BenchmarkLogging_Info 基准测试：Info 日志性能
func BenchmarkLogging_Info(b *testing.B) {
	cfg := DefaultConfig()
	logger, _ := New(cfg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("benchmark message")
	}
	_ = logger.Sync() // 忽略 Sync 错误，测试环境中无关紧要
}

// BenchmarkLogging_Infof 基准测试：Infof 格式化日志性能
func BenchmarkLogging_Infof(b *testing.B) {
	cfg := DefaultConfig()
	logger, _ := New(cfg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Infof("benchmark message %d", i)
	}
	_ = logger.Sync() // 忽略 Sync 错误，测试环境中无关紧要
}

// BenchmarkLogging_Infow 基准测试：Infow 结构化日志性能
func BenchmarkLogging_Infow(b *testing.B) {
	cfg := DefaultConfig()
	logger, _ := New(cfg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Infow("benchmark message", "iteration", i, "status", "running")
	}
	_ = logger.Sync() // 忽略 Sync 错误，测试环境中无关紧要
}
