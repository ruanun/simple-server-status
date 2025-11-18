package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/viper"
)

// TestConfig 用于测试的配置结构
type TestConfig struct {
	Server struct {
		Host string `mapstructure:"host"`
		Port int    `mapstructure:"port"`
	} `mapstructure:"server"`
	Database struct {
		Name string `mapstructure:"name"`
		User string `mapstructure:"user"`
	} `mapstructure:"database"`
	Debug bool `mapstructure:"debug"`
}

// TestDefaultLoadOptions 测试默认配置选项
func TestDefaultLoadOptions(t *testing.T) {
	opts := DefaultLoadOptions("test-config.yaml")

	if opts.ConfigName != "test-config.yaml" {
		t.Errorf("ConfigName = %s; want test-config.yaml", opts.ConfigName)
	}
	if opts.ConfigType != "yaml" {
		t.Errorf("ConfigType = %s; want yaml", opts.ConfigType)
	}
	if opts.ConfigEnvKey != "CONFIG" {
		t.Errorf("ConfigEnvKey = %s; want CONFIG", opts.ConfigEnvKey)
	}
	if !opts.WatchConfigFile {
		t.Error("WatchConfigFile = false; want true")
	}

	// 检查默认搜索路径
	expectedPaths := []string{".", "./configs", "/etc/sss"}
	if len(opts.SearchPaths) != len(expectedPaths) {
		t.Errorf("SearchPaths length = %d; want %d", len(opts.SearchPaths), len(expectedPaths))
	}
	for i, path := range expectedPaths {
		if i < len(opts.SearchPaths) && opts.SearchPaths[i] != path {
			t.Errorf("SearchPaths[%d] = %s; want %s", i, opts.SearchPaths[i], path)
		}
	}
}

// TestLoad_ValidConfig 测试加载有效的配置文件
func TestLoad_ValidConfig(t *testing.T) {
	// 创建临时目录和配置文件
	tempDir, err := os.MkdirTemp("", "config_test_*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configFile := filepath.Join(tempDir, "test.yaml")
	configContent := `
server:
  host: localhost
  port: 8080
database:
  name: testdb
  user: testuser
debug: true
`
	if err := os.WriteFile(configFile, []byte(configContent), 0600); err != nil {
		t.Fatalf("创建配置文件失败: %v", err)
	}

	// 加载配置
	opts := LoadOptions{
		ConfigName:      "test.yaml",
		ConfigType:      "yaml",
		SearchPaths:     []string{tempDir},
		WatchConfigFile: false, // 测试中禁用监听
	}

	var cfg TestConfig
	v, err := Load(opts, &cfg)
	if err != nil {
		t.Fatalf("Load() error = %v; want nil", err)
	}
	if v == nil {
		t.Fatal("Load() returned nil viper instance")
	}

	// 验证配置值
	if cfg.Server.Host != "localhost" {
		t.Errorf("Server.Host = %s; want localhost", cfg.Server.Host)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("Server.Port = %d; want 8080", cfg.Server.Port)
	}
	if cfg.Database.Name != "testdb" {
		t.Errorf("Database.Name = %s; want testdb", cfg.Database.Name)
	}
	if cfg.Database.User != "testuser" {
		t.Errorf("Database.User = %s; want testuser", cfg.Database.User)
	}
	if !cfg.Debug {
		t.Error("Debug = false; want true")
	}
}

// TestLoad_FileNotFound 测试配置文件不存在的情况
func TestLoad_FileNotFound(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "config_test_*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	opts := LoadOptions{
		ConfigName:      "nonexistent.yaml",
		ConfigType:      "yaml",
		SearchPaths:     []string{tempDir},
		WatchConfigFile: false,
	}

	var cfg TestConfig
	_, err = Load(opts, &cfg)
	if err == nil {
		t.Error("Load() with nonexistent file should return error")
	}
}

// TestLoad_InvalidYAML 测试无效的 YAML 配置文件
func TestLoad_InvalidYAML(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "config_test_*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configFile := filepath.Join(tempDir, "invalid.yaml")
	invalidContent := `
server:
  host: localhost
  port: "invalid_port"  # 类型错误
  [invalid yaml syntax
`
	if err := os.WriteFile(configFile, []byte(invalidContent), 0600); err != nil {
		t.Fatalf("创建配置文件失败: %v", err)
	}

	opts := LoadOptions{
		ConfigName:      "invalid.yaml",
		ConfigType:      "yaml",
		SearchPaths:     []string{tempDir},
		WatchConfigFile: false,
	}

	var cfg TestConfig
	_, err = Load(opts, &cfg)
	if err == nil {
		t.Error("Load() with invalid YAML should return error")
	}
}

// TestLoad_WithEnvVariable 测试使用环境变量指定配置文件
func TestLoad_WithEnvVariable(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "config_test_*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 在临时目录创建配置文件
	configFile := filepath.Join(tempDir, "env-config.yaml")
	configContent := `
server:
  host: env-host
  port: 9090
debug: false
`
	if err := os.WriteFile(configFile, []byte(configContent), 0600); err != nil {
		t.Fatalf("创建配置文件失败: %v", err)
	}

	// 设置环境变量
	envKey := "TEST_CONFIG_PATH"
	_ = os.Setenv(envKey, configFile)
	defer os.Unsetenv(envKey)

	opts := LoadOptions{
		ConfigName:      "default.yaml", // 应该被环境变量覆盖
		ConfigType:      "yaml",
		ConfigEnvKey:    envKey,
		SearchPaths:     []string{"."}, // 应该被忽略
		WatchConfigFile: false,
	}

	var cfg TestConfig
	v, err := Load(opts, &cfg)
	if err != nil {
		t.Fatalf("Load() error = %v; want nil", err)
	}
	if v == nil {
		t.Fatal("Load() returned nil viper instance")
	}

	// 验证使用了环境变量指定的配置
	if cfg.Server.Host != "env-host" {
		t.Errorf("Server.Host = %s; want env-host", cfg.Server.Host)
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("Server.Port = %d; want 9090", cfg.Server.Port)
	}
}

// TestLoad_SearchPaths 测试搜索路径的优先级
func TestLoad_SearchPaths(t *testing.T) {
	tempDir1, err := os.MkdirTemp("", "config_test1_*")
	if err != nil {
		t.Fatalf("创建临时目录1失败: %v", err)
	}
	defer os.RemoveAll(tempDir1)

	tempDir2, err := os.MkdirTemp("", "config_test2_*")
	if err != nil {
		t.Fatalf("创建临时目录2失败: %v", err)
	}
	defer os.RemoveAll(tempDir2)

	// 在两个目录创建同名配置文件，值不同
	configFile1 := filepath.Join(tempDir1, "test.yaml")
	configContent1 := `
server:
  host: host-from-path1
  port: 1111
`
	if err := os.WriteFile(configFile1, []byte(configContent1), 0600); err != nil {
		t.Fatalf("创建配置文件1失败: %v", err)
	}

	configFile2 := filepath.Join(tempDir2, "test.yaml")
	configContent2 := `
server:
  host: host-from-path2
  port: 2222
`
	if err := os.WriteFile(configFile2, []byte(configContent2), 0600); err != nil {
		t.Fatalf("创建配置文件2失败: %v", err)
	}

	// 搜索路径：tempDir1 在前，应该优先使用
	opts := LoadOptions{
		ConfigName:      "test.yaml",
		ConfigType:      "yaml",
		SearchPaths:     []string{tempDir1, tempDir2},
		WatchConfigFile: false,
	}

	var cfg TestConfig
	_, err = Load(opts, &cfg)
	if err != nil {
		t.Fatalf("Load() error = %v; want nil", err)
	}

	// 应该使用第一个搜索路径中的配置
	if cfg.Server.Host != "host-from-path1" {
		t.Errorf("Server.Host = %s; want host-from-path1 (应该使用第一个搜索路径)", cfg.Server.Host)
	}
	if cfg.Server.Port != 1111 {
		t.Errorf("Server.Port = %d; want 1111", cfg.Server.Port)
	}
}

// TestLoad_JSONConfig 测试加载 JSON 格式配置文件
func TestLoad_JSONConfig(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "config_test_*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configFile := filepath.Join(tempDir, "test.json")
	configContent := `{
  "server": {
    "host": "json-host",
    "port": 3000
  },
  "database": {
    "name": "jsondb",
    "user": "jsonuser"
  },
  "debug": true
}`
	if err := os.WriteFile(configFile, []byte(configContent), 0600); err != nil {
		t.Fatalf("创建配置文件失败: %v", err)
	}

	opts := LoadOptions{
		ConfigName:      "test.json",
		ConfigType:      "json",
		SearchPaths:     []string{tempDir},
		WatchConfigFile: false,
	}

	var cfg TestConfig
	_, err = Load(opts, &cfg)
	if err != nil {
		t.Fatalf("Load() JSON config error = %v; want nil", err)
	}

	if cfg.Server.Host != "json-host" {
		t.Errorf("Server.Host = %s; want json-host", cfg.Server.Host)
	}
	if cfg.Server.Port != 3000 {
		t.Errorf("Server.Port = %d; want 3000", cfg.Server.Port)
	}
}

// TestLoad_WithCallback 测试配置变更回调（不实际触发文件变更）
func TestLoad_WithCallback(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "config_test_*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configFile := filepath.Join(tempDir, "test.yaml")
	configContent := `
server:
  host: localhost
  port: 8080
debug: true
`
	if err := os.WriteFile(configFile, []byte(configContent), 0600); err != nil {
		t.Fatalf("创建配置文件失败: %v", err)
	}

	// 定义回调函数
	callback := func(v *viper.Viper) error {
		// 回调函数在配置文件变更时被调用
		return nil
	}

	opts := LoadOptions{
		ConfigName:      "test.yaml",
		ConfigType:      "yaml",
		SearchPaths:     []string{tempDir},
		WatchConfigFile: true,
		OnConfigChange:  callback,
	}

	var cfg TestConfig
	v, err := Load(opts, &cfg)
	if err != nil {
		t.Fatalf("Load() error = %v; want nil", err)
	}
	if v == nil {
		t.Fatal("Load() returned nil viper instance")
	}

	// 验证配置加载成功
	if cfg.Server.Host != "localhost" {
		t.Errorf("Server.Host = %s; want localhost", cfg.Server.Host)
	}

	// 注意：这里只是验证回调设置成功，不实际触发文件变更
	// 因为实际触发需要等待文件系统事件，测试会比较复杂
}

// TestReload 测试重新加载配置
func TestReload(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "config_test_*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configFile := filepath.Join(tempDir, "test.yaml")
	initialContent := `
server:
  host: initial-host
  port: 8080
debug: false
`
	if err := os.WriteFile(configFile, []byte(initialContent), 0600); err != nil {
		t.Fatalf("创建配置文件失败: %v", err)
	}

	// 初始加载
	opts := LoadOptions{
		ConfigName:      "test.yaml",
		ConfigType:      "yaml",
		SearchPaths:     []string{tempDir},
		WatchConfigFile: false,
	}

	var cfg TestConfig
	v, err := Load(opts, &cfg)
	if err != nil {
		t.Fatalf("Load() error = %v; want nil", err)
	}

	if cfg.Server.Host != "initial-host" {
		t.Errorf("初始 Server.Host = %s; want initial-host", cfg.Server.Host)
	}

	// 修改配置文件
	updatedContent := `
server:
  host: updated-host
  port: 9090
debug: true
`
	if err := os.WriteFile(configFile, []byte(updatedContent), 0600); err != nil {
		t.Fatalf("更新配置文件失败: %v", err)
	}

	// 等待文件系统同步
	time.Sleep(100 * time.Millisecond)

	// 重新加载配置
	if err := Reload(v, &cfg); err != nil {
		t.Fatalf("Reload() error = %v; want nil", err)
	}

	// 验证配置已更新
	if cfg.Server.Host != "updated-host" {
		t.Errorf("重新加载后 Server.Host = %s; want updated-host", cfg.Server.Host)
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("重新加载后 Server.Port = %d; want 9090", cfg.Server.Port)
	}
	if !cfg.Debug {
		t.Error("重新加载后 Debug = false; want true")
	}
}

// TestReload_FileDeleted 测试配置文件被删除后重新加载
func TestReload_FileDeleted(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "config_test_*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configFile := filepath.Join(tempDir, "test.yaml")
	configContent := `
server:
  host: localhost
  port: 8080
`
	if err := os.WriteFile(configFile, []byte(configContent), 0600); err != nil {
		t.Fatalf("创建配置文件失败: %v", err)
	}

	opts := LoadOptions{
		ConfigName:      "test.yaml",
		ConfigType:      "yaml",
		SearchPaths:     []string{tempDir},
		WatchConfigFile: false,
	}

	var cfg TestConfig
	v, err := Load(opts, &cfg)
	if err != nil {
		t.Fatalf("Load() error = %v; want nil", err)
	}

	// 删除配置文件
	if err := os.Remove(configFile); err != nil {
		t.Fatalf("删除配置文件失败: %v", err)
	}

	// 尝试重新加载，应该失败
	err = Reload(v, &cfg)
	if err == nil {
		t.Error("Reload() after file deleted should return error")
	}
}

// TestResolveConfigPath_EnvPriority 测试环境变量优先级
func TestResolveConfigPath_EnvPriority(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "config_test_*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 在搜索路径创建配置文件
	searchConfig := filepath.Join(tempDir, "search.yaml")
	if err := os.WriteFile(searchConfig, []byte("test: search"), 0600); err != nil {
		t.Fatalf("创建搜索路径配置文件失败: %v", err)
	}

	// 环境变量指定的文件
	envConfig := filepath.Join(tempDir, "env.yaml")
	if err := os.WriteFile(envConfig, []byte("test: env"), 0600); err != nil {
		t.Fatalf("创建环境变量配置文件失败: %v", err)
	}

	// 设置环境变量
	envKey := "TEST_PRIORITY_CONFIG"
	_ = os.Setenv(envKey, envConfig)
	defer os.Unsetenv(envKey)

	opts := LoadOptions{
		ConfigName:   "search.yaml",
		ConfigType:   "yaml",
		ConfigEnvKey: envKey,
		SearchPaths:  []string{tempDir},
	}

	path, err := resolveConfigPath(opts)
	if err != nil {
		t.Fatalf("resolveConfigPath() error = %v; want nil", err)
	}

	// 应该返回环境变量指定的路径
	if path != envConfig {
		t.Errorf("resolveConfigPath() = %s; want %s (环境变量应该优先)", path, envConfig)
	}
}

// TestLoadOptions_EmptySearchPaths 测试空搜索路径
func TestLoadOptions_EmptySearchPaths(t *testing.T) {
	opts := LoadOptions{
		ConfigName:      "nonexistent.yaml",
		ConfigType:      "yaml",
		SearchPaths:     []string{},
		WatchConfigFile: false,
	}

	var cfg TestConfig
	_, err := Load(opts, &cfg)
	if err == nil {
		t.Error("Load() with empty search paths and nonexistent file should return error")
	}
}

// BenchmarkLoad 基准测试：配置加载性能
func BenchmarkLoad(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "config_bench_*")
	if err != nil {
		b.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configFile := filepath.Join(tempDir, "bench.yaml")
	configContent := `
server:
  host: localhost
  port: 8080
database:
  name: testdb
  user: testuser
debug: true
`
	if err := os.WriteFile(configFile, []byte(configContent), 0600); err != nil {
		b.Fatalf("创建配置文件失败: %v", err)
	}

	opts := LoadOptions{
		ConfigName:      "bench.yaml",
		ConfigType:      "yaml",
		SearchPaths:     []string{tempDir},
		WatchConfigFile: false,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var cfg TestConfig
		_, err := Load(opts, &cfg)
		if err != nil {
			b.Fatalf("Load() error = %v", err)
		}
	}
}

// BenchmarkReload 基准测试：配置重载性能
func BenchmarkReload(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "config_bench_*")
	if err != nil {
		b.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configFile := filepath.Join(tempDir, "bench.yaml")
	configContent := `
server:
  host: localhost
  port: 8080
`
	if err := os.WriteFile(configFile, []byte(configContent), 0600); err != nil {
		b.Fatalf("创建配置文件失败: %v", err)
	}

	opts := LoadOptions{
		ConfigName:      "bench.yaml",
		ConfigType:      "yaml",
		SearchPaths:     []string{tempDir},
		WatchConfigFile: false,
	}

	var cfg TestConfig
	v, err := Load(opts, &cfg)
	if err != nil {
		b.Fatalf("Load() error = %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := Reload(v, &cfg); err != nil {
			b.Fatalf("Reload() error = %v", err)
		}
	}
}
