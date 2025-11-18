package main

import (
	"fmt"
	"time"

	"go.uber.org/zap"

	internal "github.com/ruanun/simple-server-status/internal/dashboard"
	"github.com/ruanun/simple-server-status/internal/dashboard/config"
	"github.com/ruanun/simple-server-status/internal/dashboard/global"
	"github.com/ruanun/simple-server-status/internal/dashboard/server"
	"github.com/ruanun/simple-server-status/internal/shared/app"
)

func main() {
	// 创建应用
	application := app.New("SSS-Dashboard", app.BuildInfo{
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
	// 使用指针捕获，以便热加载回调能访问到 dashboardService
	var dashboardService *internal.DashboardService

	// 1. 加载配置（支持热加载）
	cfg, err := loadConfig(&dashboardService)
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

	// 3. 创建错误处理器
	errorHandler := internal.NewErrorHandler(logger)

	// 4. 初始化 HTTP 服务器（Gin 引擎）
	ginEngine := server.InitServer(cfg, logger, errorHandler)

	// 5. 创建并启动 Dashboard 服务
	dashboardService, err = internal.NewDashboardService(cfg, logger, ginEngine, errorHandler)
	if err != nil {
		return fmt.Errorf("创建 Dashboard 服务失败: %w", err)
	}

	// 启动服务
	if err := dashboardService.Start(); err != nil {
		return fmt.Errorf("启动 Dashboard 服务失败: %w", err)
	}

	// 6. 注册清理函数
	registerCleanups(application, dashboardService)

	return nil
}

func loadConfig(dashboardServicePtr **internal.DashboardService) (*config.DashboardConfig, error) {
	// 使用闭包捕获配置指针以支持热加载
	var currentCfg *config.DashboardConfig

	cfg, err := app.LoadConfig[*config.DashboardConfig](
		"sss-dashboard.yaml",
		"yaml",
		[]string{".", "./configs", "/etc/sssa", "/etc/sss"},
		true,
		func(newCfg *config.DashboardConfig) error {
			// 热加载回调：更新已返回配置对象的内容
			if currentCfg != nil {
				// 验证和设置默认值
				if err := internal.ValidateAndApplyDefaults(newCfg); err != nil {
					return fmt.Errorf("热加载配置验证失败: %w", err)
				}

				// 同步更新 servers map（如果 dashboardService 已创建）
				if dashboardServicePtr != nil && *dashboardServicePtr != nil {
					(*dashboardServicePtr).ReloadServers(newCfg.Servers)
				}

				*currentCfg = *newCfg
				fmt.Println("[INFO] Dashboard 配置已热加载")
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
	if err := internal.ValidateAndApplyDefaults(cfg); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	return cfg, nil
}

func initLogger(cfg *config.DashboardConfig) (*zap.SugaredLogger, error) {
	return app.InitLogger(cfg.LogLevel, cfg.LogPath)
}

func registerCleanups(application *app.Application, dashboardService *internal.DashboardService) {
	// 注册 Dashboard 服务清理
	// 设置 10 秒超时用于优雅关闭
	application.RegisterCleanup(func() error {
		return dashboardService.Stop(10 * time.Second)
	})
}
