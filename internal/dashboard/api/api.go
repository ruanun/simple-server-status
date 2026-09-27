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
	"github.com/ruanun/simple-server-status/internal/dashboard/history"
	"github.com/ruanun/simple-server-status/internal/dashboard/hub"
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
}

// New 创建 API 并加载缓存
func New(ctx context.Context, d Deps) (*API, error) {
	a := &API{Deps: d, agents: newAgentRegistry()}
	a.bc = newBroadcaster(a)
	if err := a.reload(ctx); err != nil {
		return nil, err
	}
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
