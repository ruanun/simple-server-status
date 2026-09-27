//go:build windows

package main

import (
	"context"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
)

// serviceName Windows 服务名，与安装脚本保持一致
const serviceName = "sss-agent"

// runAsService 若当前进程由 Windows 服务管理器启动，则以服务方式运行 run 并返回 true
func runAsService(run func(context.Context) error) (bool, error) {
	isService, err := svc.IsWindowsService()
	if err != nil || !isService {
		return false, err
	}
	return true, svc.Run(serviceName, &serviceHandler{run: run})
}

// reportStartupError 以服务方式启动、但在进入服务模式前就失败时（如配置无效），日志文件尚未打开，
// 把错误写入 Windows 应用程序事件日志（来源 sss-agent），便于在事件查看器中排查
func reportStartupError(startErr error) {
	if isService, err := svc.IsWindowsService(); err != nil || !isService {
		return
	}
	// 事件源已注册时会返回错误，忽略即可
	_ = eventlog.InstallAsEventCreate(serviceName, eventlog.Error|eventlog.Warning|eventlog.Info)
	l, err := eventlog.Open(serviceName)
	if err != nil {
		return
	}
	defer func() { _ = l.Close() }()
	_ = l.Error(1, "Agent 启动失败: "+startErr.Error())
}

// serviceHandler 把服务管理器的停止请求转换为 ctx 取消
type serviceHandler struct {
	run func(context.Context) error
}

// Execute 实现 svc.Handler；run 出错时返回服务专用退出码 1，便于服务管理器按失败策略重启
func (h *serviceHandler) Execute(_ []string, req <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	status <- svc.Status{State: svc.StartPending}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- h.run(ctx) }()
	status <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}

	for {
		select {
		case err := <-done:
			status <- svc.Status{State: svc.StopPending}
			if err != nil {
				return true, 1
			}
			return false, 0
		case c := <-req:
			switch c.Cmd {
			case svc.Interrogate:
				status <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				status <- svc.Status{State: svc.StopPending}
				cancel()
				<-done
				return false, 0
			}
		}
	}
}
