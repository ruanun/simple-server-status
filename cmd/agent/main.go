package main

import (
	"fmt"
	"time"

	internal "github.com/ruanun/simple-server-status/internal/agent"
	"github.com/ruanun/simple-server-status/internal/agent/config"
	"github.com/ruanun/simple-server-status/internal/agent/global"
	"github.com/ruanun/simple-server-status/internal/shared/app"
	"go.uber.org/zap"
)

func main() {
	// 创建应用
	application := app.New("SSS-Agent", app.BuildInfo{
		GitCommit: global.GitCommit,
		Version:   global.Version,
		BuiltAt:   global.BuiltAt,
		GoVersion: global.GoVersion,
	})

	// 打印构建信息
	application.PrintBuildInfo()

	// 运行应用
	if err := run(application); err != nil {
		panic(fmt.Errorf("应用启动失败: %v", err))
	}

	// 等待关闭信号
	application.WaitForShutdown()
}

func run(application *app.Application) error {
	// 1. 加载配置
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	// 2. 初始化日志
	logger, err := initLogger(cfg)
	if err != nil {
		return err
	}
	application.SetLogger(logger)
	application.RegisterCleanup(func() error {
		return logger.Sync()
	})

	// 3. 环境验证
	if err := internal.ValidateEnvironment(); err != nil {
		logger.Warnf("环境验证警告: %v", err)
	}

	// 4. 创建并启动 Agent 服务
	agentService, err := internal.NewAgentService(cfg, logger)
	if err != nil {
		return fmt.Errorf("创建 Agent 服务失败: %w", err)
	}

	// 启动服务
	if err := agentService.Start(); err != nil {
		return fmt.Errorf("启动 Agent 服务失败: %w", err)
	}

	// 5. 注册清理函数
	registerCleanups(application, agentService)

	return nil
}

func loadConfig() (*config.AgentConfig, error) {
	// 使用闭包捕获配置指针以支持热加载
	var currentCfg *config.AgentConfig

	cfg, err := app.LoadConfig[*config.AgentConfig](
		"sss-agent.yaml",
		"yaml",
		[]string{".", "./configs", "/etc/sssa", "/etc/sss"},
		true,
		func(newCfg *config.AgentConfig) error {
			// 热加载回调：更新已返回配置对象的内容
			if currentCfg != nil {
				// 验证和设置默认值
				if err := internal.ValidateAndSetDefaults(newCfg); err != nil {
					return fmt.Errorf("热加载配置验证失败: %w", err)
				}
				*currentCfg = *newCfg
				fmt.Println("[INFO] Agent 配置已热加载")
			}
			return nil
		},
	)
	if err != nil {
		return nil, err
	}

	// 保存配置指针供闭包使用
	currentCfg = cfg

	// 详细验证和设置默认值
	if err := internal.ValidateAndSetDefaults(cfg); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	return cfg, nil
}

func initLogger(cfg *config.AgentConfig) (*zap.SugaredLogger, error) {
	return app.InitLogger(cfg.LogLevel, cfg.LogPath)
}

func registerCleanups(application *app.Application, agentService *internal.AgentService) {
	// 注册 Agent 服务清理
	// 设置 10 秒超时用于优雅关闭
	application.RegisterCleanup(func() error {
		return agentService.Stop(10 * time.Second)
	})
}
