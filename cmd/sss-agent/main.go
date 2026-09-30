// sss-agent 是 Simple Server Status 的探针程序。
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ruanun/simple-server-status/internal/agent"
	"github.com/ruanun/simple-server-status/internal/agent/collect"
	"github.com/ruanun/simple-server-status/internal/cliutil"
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
		Use:          "sss-agent",
		Short:        "Simple Server Status 探针 Agent",
		SilenceUsage: true,
		RunE:         runAgent,
	}
	pf := root.PersistentFlags()
	pf.String("config", "", "配置文件路径（默认依次查找 ./sss-agent.yaml、/etc/sss/sss-agent.yaml）")
	pf.String("dashboard", "", "Dashboard 地址，如 https://status.example.com")
	pf.String("id", "", "服务器 ID")
	pf.String("secret", "", "服务器密钥")
	pf.Bool("detect-country", true, "是否通过 Cloudflare trace 探测国家代码与公网 IPv4/IPv6")
	pf.String("log-level", "info", "日志级别：debug/info/warn/error")
	pf.String("log-file", "", "日志文件路径，留空只输出到标准输出")

	root.AddCommand(
		&cobra.Command{Use: "run", Short: "运行 Agent（默认）", RunE: runAgent},
		&cobra.Command{Use: "version", Short: "显示版本", Run: func(*cobra.Command, []string) { fmt.Println(version) }},
	)
	return root
}

// resolveConfig 合并配置：命令行 > 环境变量 > YAML > 默认值
func resolveConfig(fs *pflag.FlagSet) (agent.Config, error) {
	if err := cliutil.ApplyEnv(fs, "SSS"); err != nil {
		return agent.Config{}, err
	}
	path, _ := fs.GetString("config")
	cfg, err := agent.LoadConfig(path)
	if err != nil {
		return cfg, err
	}
	if fs.Changed("dashboard") {
		cfg.Dashboard, _ = fs.GetString("dashboard")
	}
	if fs.Changed("id") {
		cfg.ID, _ = fs.GetString("id")
	}
	if fs.Changed("secret") {
		cfg.Secret, _ = fs.GetString("secret")
	}
	if fs.Changed("detect-country") {
		cfg.DetectCountry, _ = fs.GetBool("detect-country")
	}
	if fs.Changed("log-level") {
		cfg.LogLevel, _ = fs.GetString("log-level")
	}
	if fs.Changed("log-file") {
		cfg.LogFile, _ = fs.GetString("log-file")
	}
	return cfg, cfg.Validate()
}

func runAgent(cmd *cobra.Command, _ []string) error {
	cfg, err := resolveConfig(cmd.Flags())
	if err != nil {
		reportStartupError(err)
		return err
	}
	log, closeLog, err := logx.New(cfg.LogLevel, cfg.LogFile)
	if err != nil {
		reportStartupError(err)
		return err
	}
	defer func() { _ = closeLog() }()

	run := func(ctx context.Context) error { return serve(ctx, cfg, log) }
	if isService, err := runAsService(run); isService || err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return run(ctx)
}

// networkRefresh 定时重新探测公网地址的间隔
const networkRefresh = 6 * time.Hour

// serve 连接 Dashboard 并周期上报，直到 ctx 结束或 Dashboard 要求停止
func serve(ctx context.Context, cfg agent.Config, log *slog.Logger) error {
	// 派生可取消的 ctx：serve 返回时（包括 Run 收到 stop 指令返回 ErrStopped）主动结束探测协程，
	// 不依赖上层 ctx 的取消时机
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	client, err := agent.NewClient(cfg, collect.New(log), log, version)
	if err != nil {
		return err
	}
	if cfg.DetectCountry {
		probe := func() {
			dctx, cancel := context.WithTimeout(ctx, 15*time.Second)
			n, err := collect.DetectNetwork(dctx, collect.TraceURL, collect.TraceURL)
			cancel()
			if err != nil {
				log.Warn("探测网络失败", "err", err)
				return
			}
			client.SetNetwork(n)
		}
		probe()
		go func() {
			t := time.NewTicker(networkRefresh)
			defer t.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-t.C:
					probe()
				}
			}
		}()
	}
	log.Info("Agent 启动", "version", version, "dashboard", cfg.Dashboard)
	err = client.Run(ctx)
	if errors.Is(err, agent.ErrStopped) {
		log.Info("Agent 已按 Dashboard 指令退出")
		return nil
	}
	return err
}
