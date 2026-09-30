// Package api 提供 Dashboard 的 HTTP 与 WebSocket 接口。
package api

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"runtime/debug"
	"slices"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ruanun/simple-server-status/internal/dashboard/auth"
	"github.com/ruanun/simple-server-status/internal/dashboard/captcha"
	"github.com/ruanun/simple-server-status/internal/dashboard/history"
	"github.com/ruanun/simple-server-status/internal/dashboard/hub"
	"github.com/ruanun/simple-server-status/internal/dashboard/notify"
	"github.com/ruanun/simple-server-status/internal/dashboard/outage"
	"github.com/ruanun/simple-server-status/internal/dashboard/store"
	"github.com/ruanun/simple-server-status/internal/dashboard/traffic"
)

// Deps API 依赖的组件
type Deps struct {
	Store   *store.Store
	Hub     *hub.Hub
	History *history.Recorder
	Traffic *traffic.Accumulator
	Auth    *auth.Manager
	Limiter *auth.Limiter
	Log     *slog.Logger
	Now     func() time.Time
	// Version Dashboard 版本；为正式发布版本时，安装命令锁定同版本的安装脚本与 Agent
	Version string
	// NotifyOptions 通知发送选项（零值使用默认值；测试可替换）
	NotifyOptions notify.Options
	// OutageThreshold 离线记录门槛，0 使用默认 60 秒；仅测试缩短
	OutageThreshold time.Duration
	// TurnstileURL Turnstile 核验地址，为空使用 Cloudflare 官方地址；仅测试替换
	TurnstileURL string
	// CaptchaCode 图形验证码生成函数，为空随机生成；仅测试注入固定值
	CaptchaCode func() string
}

// API HTTP 接口集合，缓存服务器列表与设置（修改后立即刷新）
type API struct {
	Deps
	mu       sync.RWMutex
	reloadMu sync.Mutex // 串行化 reload，保证"读库 + 替换缓存"整体原子，避免旧数据覆盖新数据
	list     []store.Server
	byID     map[string]store.Server
	settings store.Settings
	agents   *agentRegistry
	bc       *broadcaster
	conns    connTracker
	notifier *notify.Notifier
	outages  *outage.Tracker

	captchas  *captcha.Store
	turnstile captcha.Turnstile

	upMu        sync.RWMutex
	uptimeCache map[string]*float64 // 24 小时在线率，每分钟刷新

	lastEventCleanup time.Time // 上次清理离线记录与通知记录的时间
}

// New 创建 API 并加载缓存
func New(ctx context.Context, d Deps) (*API, error) {
	a := &API{Deps: d, agents: newAgentRegistry(),
		captchas: captcha.NewStore(d.Now, d.CaptchaCode), turnstile: captcha.Turnstile{URL: d.TurnstileURL}}
	a.bc = newBroadcaster(a)
	a.notifier = notify.NewNotifier(a, d.Hub, d.Traffic, d.Store, notify.NewSender(d.NotifyOptions, d.Log), d.NotifyOptions, d.Now, d.Log)
	a.outages = outage.New(a, d.Hub, d.Store, d.Now, d.Log, d.OutageThreshold)
	if err := a.reload(ctx); err != nil {
		return nil, err
	}
	// 上次进程被强杀时仍在「发送中」的记录不会再有结果，启动时（通知任务运行前）记为失败
	if n, err := d.Store.FailPendingNotifyLog(ctx, notify.ErrShutdown.Error(), d.Now().Unix()); err != nil {
		d.Log.Warn("更新遗留的发送中通知记录失败", "err", err)
	} else if n > 0 {
		d.Log.Info("已将上次退出时未完成的通知记录标记为失败", "count", n)
	}
	a.refreshUptime(ctx)
	return a, nil
}

// reload 从数据库刷新服务器与设置缓存
func (a *API) reload(ctx context.Context) error {
	a.reloadMu.Lock()
	defer a.reloadMu.Unlock()
	list, err := a.Store.ListServers(ctx)
	if err != nil {
		return fmt.Errorf("读取服务器列表失败: %w", err)
	}
	st, err := a.Store.GetSettings(ctx)
	if err != nil {
		return fmt.Errorf("读取设置失败: %w", err)
	}
	byID := make(map[string]store.Server, len(list))
	for _, s := range list {
		byID[s.ID] = s
	}
	a.mu.Lock()
	a.list, a.byID, a.settings = list, byID, st
	a.mu.Unlock()
	return nil
}

// updateCached 数据库写入成功后就地更新缓存中的单台服务器，避免整表 reload；
// 持有 reloadMu，防止与进行中的 reload 交错时被其旧快照覆盖
func (a *API) updateCached(id string, fn func(*store.Server)) {
	a.reloadMu.Lock()
	defer a.reloadMu.Unlock()
	a.mu.Lock()
	defer a.mu.Unlock()
	s, ok := a.byID[id]
	if !ok {
		return
	}
	fn(&s)
	a.byID[id] = s
	for i := range a.list {
		if a.list[i].ID == id {
			a.list[i] = s
			break
		}
	}
}

func (a *API) server(id string) (store.Server, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	s, ok := a.byID[id]
	return s, ok
}

func (a *API) serverList() []store.Server {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return slices.Clone(a.list)
}

func (a *API) currentSettings() store.Settings {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.settings
}

// recovered 处理 handler panic：记录日志并按统一错误信封返回 500
func (a *API) recovered(c *gin.Context, err any) {
	a.Log.Error("处理请求时发生 panic", "method", c.Request.Method, "path", c.Request.URL.Path,
		"err", fmt.Sprint(err), "stack", string(debug.Stack()))
	fail(c, http.StatusInternalServerError, "internal", "服务器内部错误")
}

// Router 构建路由；webFS 为前端产物，trustedProxies 为可信反向代理（为空时不信任任何 X-Forwarded-For）
func (a *API) Router(webFS fs.FS, trustedProxies []string) (*gin.Engine, error) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	if err := r.SetTrustedProxies(trustedProxies); err != nil {
		return nil, fmt.Errorf("可信代理配置无效: %w", err)
	}
	r.Use(gin.CustomRecoveryWithWriter(io.Discard, a.recovered))
	r.GET("/api/agent/ws", a.agentWS)
	a.registerPublic(r)
	a.registerAdmin(r)
	r.NoRoute(spaHandler(webFS))
	return r, nil
}
