package app

import (
	"fmt"

	sharedConfig "github.com/ruanun/simple-server-status/internal/shared/config"
	"github.com/spf13/viper"
)

// ConfigLoader 配置加载器接口
type ConfigLoader interface {
	// Validate 验证配置
	Validate() error
	// OnReload 配置重新加载时的回调
	OnReload() error
}

// LoadConfig 通用配置加载函数
// onReloadCallback: 配置重载时的额外处理函数（可选）
func LoadConfig[T ConfigLoader](
	configName string,
	configType string,
	searchPaths []string,
	watchConfig bool,
	onReloadCallback func(T) error,
) (T, error) {
	var cfg T

	// 配置变更回调
	configChangeCallback := func(v *viper.Viper) error {
		var tempCfg T

		// 重新解析配置
		if err := v.Unmarshal(&tempCfg); err != nil {
			fmt.Printf("[ERROR] 重新解析配置失败: %v\n", err)
			return fmt.Errorf("配置反序列化失败: %w", err)
		}

		// 验证新配置
		if err := tempCfg.Validate(); err != nil {
			fmt.Printf("[ERROR] 配置验证失败: %v\n", err)
			return fmt.Errorf("配置验证失败: %w", err)
		}

		// 执行重载回调
		if err := tempCfg.OnReload(); err != nil {
			fmt.Printf("[ERROR] 配置重载失败: %v\n", err)
			return fmt.Errorf("配置重载失败: %w", err)
		}

		// 执行额外的回调处理
		if onReloadCallback != nil {
			if err := onReloadCallback(tempCfg); err != nil {
				fmt.Printf("[ERROR] 配置更新回调失败: %v\n", err)
				return fmt.Errorf("配置更新失败: %w", err)
			}
		}

		// 更新配置
		cfg = tempCfg
		fmt.Printf("[INFO] 配置已热加载并验证成功\n")

		return nil
	}

	// 加载配置
	_, err := sharedConfig.Load(sharedConfig.LoadOptions{
		ConfigName:      configName,
		ConfigType:      configType,
		ConfigEnvKey:    "CONFIG",
		SearchPaths:     searchPaths,
		WatchConfigFile: watchConfig,
		OnConfigChange:  configChangeCallback,
	}, &cfg)

	if err != nil {
		return cfg, fmt.Errorf("加载配置失败: %w", err)
	}

	// 验证初始配置
	if err := cfg.Validate(); err != nil {
		return cfg, fmt.Errorf("配置验证失败: %w", err)
	}

	return cfg, nil
}
