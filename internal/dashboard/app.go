// Package dashboard 组装并运行 Dashboard 服务。
package dashboard

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ruanun/simple-server-status/internal/dashboard/api"
	"github.com/ruanun/simple-server-status/internal/dashboard/auth"
	"github.com/ruanun/simple-server-status/internal/dashboard/history"
	"github.com/ruanun/simple-server-status/internal/dashboard/hub"
	"github.com/ruanun/simple-server-status/internal/dashboard/randx"
	"github.com/ruanun/simple-server-status/internal/dashboard/store"
	"github.com/ruanun/simple-server-status/internal/dashboard/traffic"
)

// DBFile 数据库文件名
const DBFile = "sss.db"

// Options 启动参数
type Options struct {
	Listen         string
	Listener       net.Listener // 非空时忽略 Listen（测试用）
	DataDir        string
	TrustedProxies []string
	AdminPassword  string // 仅首次初始化时使用，为空则随机生成
	Log            *slog.Logger
	Version        string
	WebFS          fs.FS
}

// Run 启动服务，直到 ctx 结束后优雅退出
func Run(ctx context.Context, o Options) error {
	if o.Log == nil {
		o.Log = slog.Default()
	}
	if err := os.MkdirAll(o.DataDir, 0o750); err != nil {
		return fmt.Errorf("创建数据目录失败: %w", err)
	}
	st, err := store.Open(filepath.Join(o.DataDir, DBFile))
	if err != nil {
		return err
	}
	defer func() { _ = st.Close() }()
	if err := bootstrapAdmin(ctx, st, o.AdminPassword, o.Log); err != nil {
		return err
	}
	secret, err := st.JWTSecret(ctx)
	if err != nil {
		return fmt.Errorf("读取 JWT 密钥失败: %w", err)
	}

	now := time.Now
	rec := history.NewRecorder(st, now, o.Log)
	tr := traffic.New(st, now)
	a, err := api.New(ctx, api.Deps{
		Store: st, Hub: hub.New(now), History: rec, Traffic: tr,
		Auth: auth.NewManager(secret, now), Limiter: auth.NewLimiter(now), Log: o.Log, Now: now,
		Version: o.Version,
	})
	if err != nil {
		return err
	}
	eng, err := a.Router(o.WebFS, o.TrustedProxies)
	if err != nil {
		return err
	}
	ln := o.Listener
	if ln == nil {
		if ln, err = net.Listen("tcp", o.Listen); err != nil {
			return fmt.Errorf("监听 %s 失败: %w", o.Listen, err)
		}
	}
	srv := &http.Server{Handler: eng, ReadHeaderTimeout: 10 * time.Second}

	bgCtx, bgCancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(5)
	go func() { defer wg.Done(); rec.Run(bgCtx) }()
	go func() { defer wg.Done(); tr.Run(bgCtx, o.Log) }()
	go func() { defer wg.Done(); a.RunBroadcaster(bgCtx) }()
	go func() { defer wg.Done(); a.RunMaintenance(bgCtx) }()
	go func() { defer wg.Done(); a.RunNotifier(bgCtx) }()

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()
	o.Log.Info("Dashboard 已启动", "addr", ln.Addr().String(), "version", o.Version, "data_dir", o.DataDir)

	var serveErr error
	select {
	case <-ctx.Done():
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			serveErr = err
		}
	}
	a.Shutdown() // 断开长连接并等待其写入最后在线时间、流量等收尾数据
	sctx, scancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer scancel()
	_ = srv.Shutdown(sctx)
	bgCancel()
	wg.Wait() // 各后台任务退出前会写入剩余数据
	o.Log.Info("Dashboard 已停止")
	return serveErr
}

// bootstrapAdmin 数据库中没有用户时创建 admin
func bootstrapAdmin(ctx context.Context, st *store.Store, password string, log *slog.Logger) error {
	n, err := st.CountUsers(ctx)
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	generated := password == ""
	if generated {
		password = randx.String(16)
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	if err := st.CreateUser(ctx, &store.User{Username: "admin", PasswordHash: hash}); err != nil {
		return fmt.Errorf("创建管理员失败: %w", err)
	}
	if generated {
		log.Warn("已创建管理员账户，请登录后立即修改密码", "username", "admin", "password", password)
	} else {
		log.Info("已使用指定密码创建管理员账户", "username", "admin")
	}
	return nil
}

// ResetPassword 为 admin 生成新随机密码并使旧登录失效，返回新密码
func ResetPassword(ctx context.Context, dataDir string) (string, error) {
	st, err := store.Open(filepath.Join(dataDir, DBFile))
	if err != nil {
		return "", err
	}
	defer func() { _ = st.Close() }()
	pw := randx.String(16)
	hash, err := auth.HashPassword(pw)
	if err != nil {
		return "", err
	}
	u, err := st.GetUserByName(ctx, "admin")
	if errors.Is(err, store.ErrNotFound) {
		return pw, st.CreateUser(ctx, &store.User{Username: "admin", PasswordHash: hash})
	}
	if err != nil {
		return "", err
	}
	return pw, st.SetPassword(ctx, u.ID, hash)
}
