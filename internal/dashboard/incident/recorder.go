// Package incident 检测服务器的运行状况（离线、高负载、重启、IP 变化），记录为事件并发布给订阅者。
package incident

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/ruanun/simple-server-status/internal/dashboard/store"
)

// 事件类型
const (
	KindOffline  = "offline"
	KindLoadCPU  = "load_cpu"
	KindLoadMem  = "load_mem"
	KindLoadDisk = "load_disk"
	KindReboot   = "reboot"
	KindIPChange = "ip_change"
)

// Kinds 全部事件类型
var Kinds = []string{KindOffline, KindLoadCPU, KindLoadMem, KindLoadDisk, KindReboot, KindIPChange}

// LoadMetric 负载事件类型对应的指标（cpu、mem、disk）
var LoadMetric = map[string]string{KindLoadCPU: "cpu", KindLoadMem: "mem", KindLoadDisk: "disk"}

// changeBuffer 变更发布缓冲，满时丢弃（事件记录本身不受影响）
const changeBuffer = 256

// ChangeType 事件变更类型
type ChangeType int

// 事件变更类型
const (
	Opened  ChangeType = iota + 1 // 时段事件开始
	Closed                        // 时段事件结束
	Instant                       // 瞬时事件
)

// Change 一次事件变更
type Change struct {
	Type  ChangeType
	Event store.Event
}

// Tracked 进行中的事件；OpenedAt 为本进程中开始的时间，启动时从数据库加载的为零值
type Tracked struct {
	store.Event
	OpenedAt time.Time
}

// Store 事件存储（store.Store 实现）
type Store interface {
	CreateEvent(ctx context.Context, e *store.Event) error
	CloseEvent(ctx context.Context, id, end int64) error
	SetEventDetail(ctx context.Context, id int64, detail json.RawMessage) error
	SetEventNotifyState(ctx context.Context, id int64, state int) (*int64, error)
	OpenEvents(ctx context.Context) ([]store.Event, error)
}

type openKey struct{ server, kind string }

// Recorder 事件的唯一写入入口：维护进行中的事件，写库成功后发布变更
type Recorder struct {
	st      Store
	now     func() time.Time
	log     *slog.Logger
	changes chan Change

	mu     sync.Mutex
	loaded bool
	open   map[openKey]*Tracked
}

// NewRecorder 创建 Recorder
func NewRecorder(st Store, now func() time.Time, log *slog.Logger) *Recorder {
	return &Recorder{st: st, now: now, log: log, changes: make(chan Change, changeBuffer), open: map[openKey]*Tracked{}}
}

// Changes 事件变更的订阅通道
func (r *Recorder) Changes() <-chan Change { return r.changes }

// Load 从数据库加载进行中的事件（Dashboard 重启后继续跟踪）；已加载过时直接返回
func (r *Recorder) Load(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.loaded {
		return nil
	}
	list, err := r.st.OpenEvents(ctx)
	if err != nil {
		return err
	}
	for _, e := range list {
		r.open[openKey{e.ServerID, e.Kind}] = &Tracked{Event: e}
	}
	r.loaded = true
	return nil
}

func (r *Recorder) publish(c Change) {
	select {
	case r.changes <- c:
	default:
		r.log.Warn("事件变更队列已满，丢弃一条", "id", c.Event.ServerID, "kind", c.Event.Kind)
	}
}

func marshal(detail any) json.RawMessage {
	if detail == nil {
		return json.RawMessage("{}")
	}
	b, err := json.Marshal(detail)
	if err != nil {
		return json.RawMessage("{}")
	}
	return b
}

// Ongoing 返回某台服务器某类进行中的事件
func (r *Recorder) Ongoing(serverID, kind string) (Tracked, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.open[openKey{serverID, kind}]
	if !ok {
		return Tracked{}, false
	}
	return *t, true
}

// OngoingKind 返回某类全部进行中的事件
func (r *Recorder) OngoingKind(kind string) []Tracked {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []Tracked{}
	for k, t := range r.open {
		if k.kind == kind {
			out = append(out, *t)
		}
	}
	return out
}

// Open 开始一个时段事件；同一服务器同类事件已在进行中时忽略
func (r *Recorder) Open(ctx context.Context, serverID, kind string, start int64, detail any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := openKey{serverID, kind}
	if _, ok := r.open[k]; ok {
		return nil
	}
	e := store.Event{ServerID: serverID, Kind: kind, StartAt: start, Detail: marshal(detail)}
	if err := r.st.CreateEvent(ctx, &e); err != nil {
		return err
	}
	r.open[k] = &Tracked{Event: e, OpenedAt: r.now()}
	r.publish(Change{Type: Opened, Event: e})
	return nil
}

// Update 更新进行中事件的详情
func (r *Recorder) Update(ctx context.Context, serverID, kind string, detail any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.open[openKey{serverID, kind}]
	if !ok {
		return nil
	}
	d := marshal(detail)
	if err := r.st.SetEventDetail(ctx, t.ID, d); err != nil {
		return err
	}
	t.Detail = d
	return nil
}

// Close 结束进行中的事件；没有进行中的事件时忽略
func (r *Recorder) Close(ctx context.Context, serverID, kind string, end int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := openKey{serverID, kind}
	t, ok := r.open[k]
	if !ok {
		return nil
	}
	if err := r.st.CloseEvent(ctx, t.ID, end); err != nil {
		return err
	}
	delete(r.open, k)
	e := t.Event
	e.EndAt = &end
	r.publish(Change{Type: Closed, Event: e})
	return nil
}

// Instant 记录一个瞬时事件
func (r *Recorder) Instant(ctx context.Context, serverID, kind string, at int64, detail any) error {
	e := store.Event{ServerID: serverID, Kind: kind, StartAt: at, EndAt: &at, Detail: marshal(detail)}
	if err := r.st.CreateEvent(ctx, &e); err != nil {
		return err
	}
	r.publish(Change{Type: Instant, Event: e})
	return nil
}

// SetNotifyState 更新事件的推送状态，返回更新后的事件（EndAt 为当前结束时间，事件可能已经结束）。
// 与 Close 在同一把锁下执行：结束变更中的推送状态与这里返回的结束时间总有一方反映了另一方，
// 订阅者据此决定由谁推送恢复通知
func (r *Recorder) SetNotifyState(ctx context.Context, e store.Event, state int) (store.Event, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	end, err := r.st.SetEventNotifyState(ctx, e.ID, state)
	if err != nil {
		return e, err
	}
	e.NotifyState, e.EndAt = state, end
	if t, ok := r.open[openKey{e.ServerID, e.Kind}]; ok && t.ID == e.ID {
		t.NotifyState = state
	}
	return e, nil
}

// retain 只保留 ids 中服务器的进行中事件（已删除服务器的在下一轮检测时清理）
func (r *Recorder) retain(ids map[string]bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for k := range r.open {
		if !ids[k.server] {
			delete(r.open, k)
		}
	}
}
