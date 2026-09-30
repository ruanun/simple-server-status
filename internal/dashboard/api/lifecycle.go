package api

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/ruanun/simple-server-status/internal/dashboard/store"
)

// shutdownWait Shutdown 等待长连接收尾的最长时间
const shutdownWait = 5 * time.Second

// maintenanceInterval 在线服务器 last_seen 落库周期
const maintenanceInterval = time.Minute

// connTracker 跟踪进行中的长连接 handler；关闭后拒绝新连接，并可等待已有连接收尾
type connTracker struct {
	mu     sync.Mutex
	closed bool
	wg     sync.WaitGroup
}

// enter 登记一个 handler；已关闭时返回 false。成功时调用方须在退出前调用 leave
func (t *connTracker) enter() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return false
	}
	t.wg.Add(1)
	return true
}

func (t *connTracker) leave() { t.wg.Done() }

func (t *connTracker) close() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.closed = true
}

// wait 等待所有 handler 退出，超时返回 false
func (t *connTracker) wait(d time.Duration) bool {
	done := make(chan struct{})
	go func() {
		t.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-time.After(d):
		return false
	}
}

// Shutdown 拒绝新的长连接并断开现有连接，最多等待 5 秒让 handler 完成收尾（写入最后在线时间、流量等）
func (a *API) Shutdown() {
	a.conns.close()
	a.agents.closeAll()
	a.bc.closeAll()
	if !a.conns.wait(shutdownWait) {
		a.Log.Warn("等待长连接退出超时")
	}
}

// touchOnline 把当前在线服务器的最后在线时间写入数据库，避免异常退出时丢失
func (a *API) touchOnline(ctx context.Context) {
	now := a.Now().Unix()
	for _, srv := range a.serverList() {
		if !a.Hub.Get(srv.ID).Online {
			continue
		}
		a.saveLastSeen(ctx, srv.ID, now)
	}
}

// saveLastSeen 写入最后在线时间并同步缓存
func (a *API) saveLastSeen(ctx context.Context, id string, ts int64) {
	if err := a.Store.SetLastSeen(ctx, id, ts); err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			a.Log.Warn("记录最后在线时间失败", "id", id, "err", err)
		}
		return
	}
	a.updateCached(id, func(s *store.Server) { s.LastSeen = ts })
}

// RunMaintenance 每分钟执行一次周期维护任务，直到 ctx 结束
func (a *API) RunMaintenance(ctx context.Context) {
	t := time.NewTicker(maintenanceInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			a.touchOnline(ctx)
			a.refreshUptime(ctx)
		}
	}
}
