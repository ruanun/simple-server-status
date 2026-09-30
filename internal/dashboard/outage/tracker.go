// Package outage 根据实时状态记录服务器的离线时段。
package outage

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/ruanun/simple-server-status/internal/dashboard/hub"
	"github.com/ruanun/simple-server-status/internal/dashboard/store"
)

// DefaultThreshold 离线持续多久才记录
const DefaultThreshold = time.Minute

// DefaultGrace 启动宽限期（与 notify 的 baselineWindow 一致），从首轮检查开始计算。
// Dashboard 刚启动时 Agent 还没来得及重连，最后上报时间停留在停机前，
// 宽限期内不为「没有进行中记录」的服务器新建记录，避免每次重启都给所有服务器多记一条；
// 宽限期结束后仍离线的照常新建，开始时间仍取最后上报时间（停机时段计入时长）。
// 门槛非默认值时（仅测试缩短）宽限期取门槛的 2 倍，与默认值 60 秒 → 2 分钟的比例一致。
const DefaultGrace = 2 * time.Minute

// Store 离线记录存储（store.Store 实现）
type Store interface {
	OpenOutages(ctx context.Context) ([]store.Outage, error)
	CreateOutage(ctx context.Context, serverID string, start int64) (int64, error)
	CloseOutage(ctx context.Context, id, end int64) error
}

// Source 当前服务器列表（由 api.API 提供）
type Source interface {
	Servers() []store.Server
}

// Tracker 周期检查服务器在线状态：离线满门槛时新建记录，恢复时补结束时间
type Tracker struct {
	src       Source
	hub       *hub.Hub
	st        Store
	now       func() time.Time
	log       *slog.Logger
	threshold time.Duration
	interval  time.Duration
	grace     time.Duration

	mu      sync.Mutex
	open    map[string]store.Outage // 服务器 ID → 进行中的记录
	loaded  bool
	started time.Time // 首轮检查的时间，宽限期由此开始
}

// New 创建 Tracker；threshold ≤ 0 时使用 DefaultThreshold，检查间隔为门槛的一半；
// 启动宽限期默认 DefaultGrace，门槛非默认值时为门槛的 2 倍
func New(src Source, h *hub.Hub, st Store, now func() time.Time, log *slog.Logger, threshold time.Duration) *Tracker {
	if threshold <= 0 {
		threshold = DefaultThreshold
	}
	grace := DefaultGrace
	if threshold != DefaultThreshold {
		grace = 2 * threshold
	}
	return &Tracker{src: src, hub: h, st: st, now: now, log: log, threshold: threshold, interval: threshold / 2, grace: grace, open: map[string]store.Outage{}}
}

// Check 执行一轮检查；首轮先读取数据库中进行中的记录（Dashboard 重启后继续跟踪）。
// 启动宽限期内只结束已有记录，不新建记录
func (t *Tracker) Check(ctx context.Context) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.started.IsZero() {
		t.started = t.now()
	}
	if !t.loaded {
		list, err := t.st.OpenOutages(ctx)
		if err != nil {
			t.log.Warn("读取进行中的离线记录失败", "err", err)
			return
		}
		for _, o := range list {
			t.open[o.ServerID] = o
		}
		t.loaded = true
	}
	nowT := t.now()
	now := nowT.Unix()
	inGrace := nowT.Sub(t.started) < t.grace
	seen := map[string]bool{}
	for _, srv := range t.src.Servers() {
		seen[srv.ID] = true
		live := t.hub.Get(srv.ID)
		last := live.LastReport
		if last == 0 {
			last = srv.LastSeen
		}
		o, has := t.open[srv.ID]
		switch {
		case live.Online && has:
			if err := t.st.CloseOutage(ctx, o.ID, live.LastReport); err != nil && !errors.Is(err, store.ErrNotFound) {
				t.log.Warn("结束离线记录失败", "id", srv.ID, "err", err)
				continue
			}
			delete(t.open, srv.ID)
		case !live.Online && !has && !inGrace && last > 0 && time.Duration(now-last)*time.Second >= t.threshold:
			id, err := t.st.CreateOutage(ctx, srv.ID, last)
			if err != nil {
				t.log.Warn("新建离线记录失败", "id", srv.ID, "err", err)
				continue
			}
			t.open[srv.ID] = store.Outage{ID: id, ServerID: srv.ID, StartAt: last}
		}
	}
	for id := range t.open {
		if !seen[id] {
			delete(t.open, id)
		}
	}
}

// Forget 删除服务器时清理其内存状态（数据库记录由 DeleteServer 删除）
func (t *Tracker) Forget(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.open, id)
}

// Run 启动时检查一次，之后按间隔检查，直到 ctx 结束
func (t *Tracker) Run(ctx context.Context) {
	t.Check(ctx)
	tk := time.NewTicker(t.interval)
	defer tk.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tk.C:
			t.Check(ctx)
		}
	}
}
