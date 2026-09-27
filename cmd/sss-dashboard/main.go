// sss-dashboard 是 Simple Server Status 的面板服务。
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/ruanun/simple-server-status/internal/cliutil"
	"github.com/ruanun/simple-server-status/internal/dashboard"
	"github.com/ruanun/simple-server-status/internal/dashboard/web"
	"github.com/ruanun/simple-server-status/internal/logx"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// version 由构建时 -ldflags "-X main.version=..." 注入
var version = "dev"

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:          "sss-dashboard",
		Short:        "Simple Server Status 面板",
		SilenceUsage: true,
		RunE:         runServe,
	}
	pf := root.PersistentFlags()
	pf.String("data-dir", "./data", "数据目录（SQLite 数据库所在位置）")
	pf.String("log-level", "info", "日志级别：debug/info/warn/error")
	pf.String("log-file", "", "日志文件路径，留空只输出到标准输出")
	addServeFlags(root.Flags())

	serve := &cobra.Command{Use: "serve", Short: "启动面板服务（默认）", RunE: runServe}
	addServeFlags(serve.Flags())
	reset := &cobra.Command{Use: "reset-password", Short: "重置 admin 密码并使已登录会话失效", RunE: runResetPassword}
	ver := &cobra.Command{Use: "version", Short: "显示版本", Run: func(cmd *cobra.Command, _ []string) {
		fmt.Fprintln(cmd.OutOrStdout(), version)
	}}
	root.AddCommand(serve, reset, ver)
	return root
}

func addServeFlags(fs *pflag.FlagSet) {
	fs.String("listen", ":8900", "监听地址")
	fs.StringSlice("trusted-proxies", nil, "可信反向代理的 IP 或 CIDR，逗号分隔")
	fs.String("admin-password", "", "首次初始化时的管理员密码，留空则随机生成并打印到日志")
}

func runServe(cmd *cobra.Command, _ []string) error {
	fs := cmd.Flags()
	if err := cliutil.ApplyEnv(fs, "SSS"); err != nil {
		return err
	}
	dataDir, _ := fs.GetString("data-dir")
	listen, _ := fs.GetString("listen")
	proxies, _ := fs.GetStringSlice("trusted-proxies")
	adminPw, _ := fs.GetString("admin-password")
	level, _ := fs.GetString("log-level")
	file, _ := fs.GetString("log-file")

	log, closeLog, err := logx.New(level, file)
	if err != nil {
		return err
	}
	defer func() { _ = closeLog() }()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return dashboard.Run(ctx, dashboard.Options{
		Listen: listen, DataDir: dataDir, TrustedProxies: proxies, AdminPassword: adminPw,
		Log: log, Version: version, WebFS: web.FS(),
	})
}

func runResetPassword(cmd *cobra.Command, _ []string) error {
	fs := cmd.Flags()
	if err := cliutil.ApplyEnv(fs, "SSS"); err != nil {
		return err
	}
	dataDir, _ := fs.GetString("data-dir")
	pw, err := dashboard.ResetPassword(cmd.Context(), dataDir)
	if err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "admin 新密码：%s\n", pw)
	return nil
}
