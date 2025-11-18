package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"go.uber.org/zap"
)

// Application 应用程序生命周期管理器
type Application struct {
	name    string
	version BuildInfo
	logger  *zap.SugaredLogger
	ctx     context.Context
	cancel  context.CancelFunc
	cleanup []CleanupFunc
}

// BuildInfo 构建信息
type BuildInfo struct {
	GitCommit string
	Version   string
	BuiltAt   string
	GoVersion string
}

// CleanupFunc 清理函数类型
type CleanupFunc func() error

// New 创建应用实例
func New(name string, version BuildInfo) *Application {
	ctx, cancel := context.WithCancel(context.Background())
	return &Application{
		name:    name,
		version: version,
		ctx:     ctx,
		cancel:  cancel,
		cleanup: make([]CleanupFunc, 0),
	}
}

// SetLogger 设置日志器
func (a *Application) SetLogger(logger *zap.SugaredLogger) {
	a.logger = logger
}

// RegisterCleanup 注册清理函数
func (a *Application) RegisterCleanup(fn CleanupFunc) {
	a.cleanup = append(a.cleanup, fn)
}

// PrintBuildInfo 打印构建信息
func (a *Application) PrintBuildInfo() {
	fmt.Printf("=== %s ===\n", a.name)
	fmt.Printf("版本: %s\n", a.version.Version)
	fmt.Printf("Git提交: %s\n", a.version.GitCommit)
	fmt.Printf("构建时间: %s\n", a.version.BuiltAt)
	fmt.Printf("Go版本: %s\n", a.version.GoVersion)
	fmt.Println("================")
}

// WaitForShutdown 等待关闭信号
func (a *Application) WaitForShutdown() {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	sig := <-signalChan
	if a.logger != nil {
		a.logger.Infof("接收到信号: %s，开始优雅关闭", sig)
	} else {
		fmt.Printf("接收到信号: %s，开始优雅关闭\n", sig)
	}

	a.Shutdown()
}

// Shutdown 执行清理
// 按照 LIFO 顺序执行所有清理函数，每个函数都有超时保护
func (a *Application) Shutdown() {
	a.cancel()

	// 收集所有清理错误
	var cleanupErrors []string
	cleanupTimeout := 15 * time.Second // 每个清理函数的超时时间

	// 按相反顺序执行清理函数
	for i := len(a.cleanup) - 1; i >= 0; i-- {
		// 为每个清理函数创建带超时的上下文
		ctx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)

		// 在 channel 中执行清理函数，以便支持超时控制
		done := make(chan error, 1)
		go func(fn CleanupFunc) {
			done <- fn()
		}(a.cleanup[i])

		// 等待完成或超时
		select {
		case err := <-done:
			cancel() // 清理完成，取消超时上下文
			if err != nil {
				errMsg := fmt.Sprintf("清理函数 #%d 执行失败: %v", len(a.cleanup)-i, err)
				cleanupErrors = append(cleanupErrors, errMsg)
				if a.logger != nil {
					a.logger.Errorf(errMsg)
				} else {
					fmt.Printf("%s\n", errMsg)
				}
			} else {
				if a.logger != nil {
					a.logger.Debugf("清理函数 #%d 执行成功", len(a.cleanup)-i)
				}
			}
		case <-ctx.Done():
			cancel() // 超时，取消上下文
			errMsg := fmt.Sprintf("清理函数 #%d 执行超时（超过 %v）", len(a.cleanup)-i, cleanupTimeout)
			cleanupErrors = append(cleanupErrors, errMsg)
			if a.logger != nil {
				a.logger.Warnf(errMsg)
			} else {
				fmt.Printf("%s\n", errMsg)
			}
		}
	}

	// 汇总清理结果
	if len(cleanupErrors) > 0 {
		summary := fmt.Sprintf("应用关闭完成，但有 %d 个清理函数失败:\n%s",
			len(cleanupErrors),
			strings.Join(cleanupErrors, "\n"))
		if a.logger != nil {
			a.logger.Warn(summary)
		} else {
			fmt.Println(summary)
		}
	} else {
		if a.logger != nil {
			a.logger.Info("应用已优雅关闭，所有清理函数执行成功")
		} else {
			fmt.Println("应用已优雅关闭，所有清理函数执行成功")
		}
	}
}

// Context 获取应用上下文
func (a *Application) Context() context.Context {
	return a.ctx
}
