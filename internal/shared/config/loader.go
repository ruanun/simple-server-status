package config

import (
	"fmt"
	"os"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

// LoadOptions 配置加载选项
type LoadOptions struct {
	ConfigName      string                   // 配置文件名（默认值）
	ConfigType      string                   // 配置文件类型: yaml, json, toml 等
	ConfigEnvKey    string                   // 环境变量名（用于覆盖配置文件路径）
	SearchPaths     []string                 // 配置文件搜索路径
	OnConfigChange  func(*viper.Viper) error // 配置变更回调
	WatchConfigFile bool                     // 是否监听配置文件变更
}

// DefaultLoadOptions 返回默认配置加载选项
func DefaultLoadOptions(configName string) LoadOptions {
	return LoadOptions{
		ConfigName:      configName,
		ConfigType:      "yaml",
		ConfigEnvKey:    "CONFIG",
		SearchPaths:     []string{".", "./configs", "/etc/sss"},
		WatchConfigFile: true,
	}
}

// Load 加载配置文件
// 优先级: 命令行 -c 参数 > 环境变量 > 搜索路径
func Load(opts LoadOptions, cfg interface{}) (*viper.Viper, error) {
	configFile, err := resolveConfigPath(opts)
	if err != nil {
		return nil, err
	}

	v := viper.New()
	v.SetConfigFile(configFile)
	v.SetConfigType(opts.ConfigType)

	// 读取配置文件
	if err := v.ReadInConfig(); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("配置文件不存在: %s", configFile)
		}
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	// 解析配置到结构体
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 监听配置文件变更
	if opts.WatchConfigFile {
		v.WatchConfig()
		if opts.OnConfigChange != nil {
			v.OnConfigChange(func(e fsnotify.Event) {
				fmt.Printf("配置文件已变更: %s\n", e.Name)
				// 由回调函数自己处理 Unmarshal 和验证，确保原子性
				if err := opts.OnConfigChange(v); err != nil {
					fmt.Printf("配置变更处理失败: %v\n", err)
				}
			})
		}
	}

	fmt.Printf("配置加载成功: %s\n", configFile)
	return v, nil
}

// resolveConfigPath 解析配置文件路径
// 优先级: 环境变量 > 默认搜索路径
func resolveConfigPath(opts LoadOptions) (string, error) {
	// 1. 优先使用环境变量
	if opts.ConfigEnvKey != "" {
		if configPath := os.Getenv(opts.ConfigEnvKey); configPath != "" {
			fmt.Printf("使用环境变量 %s 指定的配置文件: %s\n", opts.ConfigEnvKey, configPath)
			return configPath, nil
		}
	}

	// 2. 在搜索路径中查找配置文件
	for _, path := range opts.SearchPaths {
		configFile := path + "/" + opts.ConfigName
		if _, err := os.Stat(configFile); err == nil {
			fmt.Printf("使用搜索路径中的配置文件: %s\n", configFile)
			return configFile, nil
		}
	}

	// 3. 使用默认值（即使文件不存在也返回，让后续逻辑处理）
	defaultPath := opts.ConfigName
	fmt.Printf("使用默认配置文件路径: %s\n", defaultPath)
	return defaultPath, nil
}

// Reload 重新加载配置
func Reload(v *viper.Viper, cfg interface{}) error {
	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("重新读取配置文件失败: %w", err)
	}
	if err := v.Unmarshal(cfg); err != nil {
		return fmt.Errorf("重新解析配置失败: %w", err)
	}
	return nil
}
