//go:build !windows

package main

import "context"

// runAsService 非 Windows 平台没有服务管理器，始终返回 false
func runAsService(func(context.Context) error) (bool, error) {
	return false, nil
}

// reportStartupError 非 Windows 平台启动错误已由命令行输出到 stderr（systemd 下进入 journal），无需额外处理
func reportStartupError(error) {}
