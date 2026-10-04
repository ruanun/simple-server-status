package notify

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/ruanun/simple-server-status/internal/dashboard/incident"
	"github.com/ruanun/simple-server-status/internal/dashboard/store"
	"github.com/ruanun/simple-server-status/internal/dashboard/traffic"
)

// checkInterval 两轮检查的间隔
const checkInterval = 30 * time.Second

// Source 通知所需的服务器列表与设置（由 api.API 提供）
type Source interface {
	Servers() []store.Server
	NotifySettings() store.NotifySettings
}

// Events 事件的订阅与推送状态（incident.Recorder 实现）
type Events interface {
	Changes() <-chan incident.Change
	OngoingKind(kind string) []incident.Tracked
	SetNotifyState(ctx context.Context, e store.Event, state int) (store.Event, error)
}

// StateStore 一次性提醒与通知记录的持久化（store.Store 实现）
type StateStore interface {
	LogStore
	NotifySent(ctx context.Context, serverID, rule, key string) (bool, error)
	MarkNotified(ctx context.Context, serverID, rule, key string, ts int64) error
}

// Notifier 订阅事件并决定是否推送，每 30 秒检查一次离线推送延迟与到期、流量提醒。
// 每个事件只决定一次是否推送，推送状态（发送中、已推送、失败、不推送）随事件持久化，是唯一的状态来源：
// 渠道启用之前开始的事件、静音服务器的事件、推送开关关闭的事件都不推送，之后也不补发；
// 只有已推送的时段事件在结束时推送恢复通知
type Notifier struct {
	src    Source
	ev     Events
	tr     *traffic.Accumulator
	st     StateStore
	sender *Sender
	o      Options
	now    func() time.Time
	log    *slog.Logger

	mu    sync.Mutex
	since time.Time // 渠道从无到有的时间，零值表示当前没有渠道

	pmu      sync.Mutex      // 保护 inflight，发送回调中使用，不得换成 mu
	inflight map[string]bool // 投递中的一次性提醒（serverID|rule|key）
}

// NewNotifier 创建 Notifier
func NewNotifier(src Source, ev Events, tr *traffic.Accumulator, st StateStore, sender *Sender, o Options, now func() time.Time, log *slog.Logger) *Notifier {
	return &Notifier{src: src, ev: ev, tr: tr, st: st, sender: sender, o: o, now: now, log: log, inflight: map[string]bool{}}
}

// channels 读取当前渠道并维护渠道启用时间；调用方须持有 n.mu
func (n *Notifier) channels() (store.NotifySettings, []Channel) {
	cfg := n.src.NotifySettings()
	chs := Channels(cfg, n.o)
	switch {
	case len(chs) == 0:
		n.since = time.Time{}
	case n.since.IsZero():
		n.since = n.now()
	}
	return cfg, chs
}

func (n *Notifier) servers() map[string]store.Server {
	m := map[string]store.Server{}
	for _, s := range n.src.Servers() {
		m[s.ID] = s
	}
	return m
}

// Check 执行一轮检查：离线满推送延迟的事件、到期与流量提醒；未配置渠道时跳过
func (n *Notifier) Check(ctx context.Context) {
	n.mu.Lock()
	defer n.mu.Unlock()
	cfg, chs := n.channels()
	if len(chs) == 0 {
		return
	}
	now := n.now()
	servers := n.servers()
	if cfg.OfflineEnabled {
		for _, t := range n.ev.OngoingKind(incident.KindOffline) {
			if t.NotifyState != store.NotifyUndecided || now.Unix()-t.StartAt < int64(cfg.OfflineMinutes)*60 {
				continue
			}
			srv, ok := servers[t.ServerID]
			if !ok {
				continue
			}
			if srv.NotifyMuted || t.OpenedAt.IsZero() || t.OpenedAt.Before(n.since) {
				n.setState(t.Event, store.NotifySkipped)
				continue
			}
			n.dispatchEvent(chs, t.Event, fromIncident(t.Event, srv, now, false))
		}
	}
	for _, srv := range servers {
		n.remind(ctx, cfg, chs, srv, now)
	}
}

// Handle 处理一次事件变更
func (n *Notifier) Handle(_ context.Context, c incident.Change) {
	n.mu.Lock()
	defer n.mu.Unlock()
	cfg, chs := n.channels()
	e := c.Event
	switch {
	case c.Type == incident.Closed:
		if e.NotifyState == store.NotifyDone {
			n.recover(e) // 仍在发送中的，由发送成功的回调推送恢复
		}
	case e.Kind == incident.KindOffline:
		// 离线在持续满推送延迟后由 Check 决定
	default:
		srv, ok := n.servers()[e.ServerID]
		switch {
		case !ok:
		case len(chs) == 0 || srv.NotifyMuted || !enabled(cfg, e.Kind):
			n.setState(e, store.NotifySkipped)
		default:
			n.dispatchEvent(chs, e, fromIncident(e, srv, n.now(), false))
		}
	}
}

// recover 推送已结束事件的恢复通知；在发送回调中也会调用，不得获取 n.mu
func (n *Notifier) recover(e store.Event) {
	srv, ok := n.servers()[e.ServerID]
	chs := Channels(n.src.NotifySettings(), n.o)
	if !ok || srv.NotifyMuted || len(chs) == 0 {
		return
	}
	n.dispatch(chs, fromIncident(e, srv, n.now(), true), "", nil, nil)
}

// setState 更新事件的推送状态，返回更新后的事件；在发送回调中也会调用，不得获取 n.mu
func (n *Notifier) setState(e store.Event, state int) (store.Event, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cur, err := n.ev.SetNotifyState(ctx, e, state)
	if err != nil {
		n.log.Warn("更新事件推送状态失败", "id", e.ServerID, "err", err)
		return e, false
	}
	return cur, true
}

// dispatchEvent 推送事件的「发生」通知：先记为发送中（避免下一轮重复推送），任一渠道成功后记为已推送，
// 全部渠道在发送队列重试后仍失败记为推送失败、不再重试。时段事件在发送期间已结束的，成功后补推恢复通知
func (n *Notifier) dispatchEvent(chs []Channel, ev store.Event, e Event) {
	if _, ok := n.setState(ev, store.NotifySending); !ok {
		return // 无法记录发送中，下一轮再试
	}
	span := ev.EndAt == nil
	n.dispatch(chs, e, "", func() {
		if cur, ok := n.setState(ev, store.NotifyDone); ok && span && cur.EndAt != nil {
			n.recover(cur)
		}
	}, func() {
		n.setState(ev, store.NotifyFailed)
	})
}

// remind 评估并发送一台服务器的到期与流量提醒
func (n *Notifier) remind(ctx context.Context, cfg store.NotifySettings, chs []Channel, srv store.Server, now time.Time) {
	o := Observation{ID: srv.ID, Name: srv.Name, Muted: srv.NotifyMuted, ExpireAt: srv.ExpireAt}
	if srv.TrafficLimit != nil {
		in, out, period, err := n.tr.Usage(ctx, srv.ID, srv.TrafficResetDay)
		if err != nil {
			n.log.Warn("读取流量失败", "id", srv.ID, "err", err)
		} else {
			o.TrafficUsed, o.TrafficLimit, o.Period = traffic.Used(srv.TrafficMode, in, out), srv.TrafficLimit, period
		}
	}
	sent := func(rule, key string) bool {
		if n.isInflight(inflightKey(srv.ID, Mark{Rule: rule, Key: key})) {
			return true
		}
		ok, err := n.st.NotifySent(ctx, srv.ID, rule, key)
		if err != nil {
			n.log.Warn("读取通知记录失败", "id", srv.ID, "err", err)
			return true // 无法确认时不发送，避免重复提醒
		}
		return ok
	}
	for _, e := range Reminders(now, cfg, o, sent) {
		m := *e.Mark
		n.dispatch(chs, e, inflightKey(srv.ID, m), func() {
			mctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := n.st.MarkNotified(mctx, srv.ID, m.Rule, m.Key, n.now().Unix()); err != nil {
				n.log.Warn("记录已提醒失败", "id", srv.ID, "err", err)
			}
		}, nil)
	}
}

func inflightKey(serverID string, m Mark) string { return serverID + "|" + m.Rule + "|" + m.Key }

func (n *Notifier) setInflight(k string, on bool) {
	n.pmu.Lock()
	defer n.pmu.Unlock()
	if on {
		n.inflight[k] = true
	} else {
		delete(n.inflight, k)
	}
}

func (n *Notifier) isInflight(k string) bool {
	n.pmu.Lock()
	defer n.pmu.Unlock()
	return n.inflight[k]
}

// dispatch 为每个渠道写入发送中的记录并入队。key 非空时投递期间视为处理中以免重复入队；
// 任一渠道成功后调用 onSuccess、全部失败后调用 onFail（均可为 nil，只调用其一且只调用一次），之后清除处理中。
// 回调在发送队列协程中（可能并发）执行，不得获取 n.mu（Check 持有 n.mu 时会入队）
func (n *Notifier) dispatch(chs []Channel, e Event, key string, onSuccess, onFail func()) {
	if len(chs) == 0 {
		return // 没有渠道就不会有回调，不能标记处理中
	}
	Render(&e, n.src.NotifySettings().Lang)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var eventID *int64
	if e.EventID != 0 {
		eventID = &e.EventID
	}
	ids := map[string]int64{}
	for _, ch := range chs {
		id, err := n.st.AddNotifyLog(ctx, store.NotifyLog{ServerID: e.ServerID, ServerName: e.ServerName, EventID: eventID, Kind: e.Kind,
			Channel: ch.Name(), Title: e.Title, Message: e.Message, Status: store.LogPending, CreatedAt: e.Time})
		if err != nil {
			n.log.Warn("写入通知记录失败", "id", e.ServerID, "err", err)
			continue
		}
		ids[ch.Name()] = id
	}
	if key != "" {
		n.setInflight(key, true)
	}
	var mu sync.Mutex
	remaining, finished := len(chs), false
	n.sender.Enqueue(chs, e, func(channel string, err error) {
		n.finishLog(ids[channel], err)
		mu.Lock()
		defer mu.Unlock()
		remaining--
		if finished || (err != nil && remaining > 0) {
			return
		}
		finished = true
		cb := onFail
		if err == nil {
			cb = onSuccess
		}
		if cb != nil {
			cb()
		}
		if key != "" {
			n.setInflight(key, false)
		}
	})
}

// finishLog 更新通知记录的最终状态；id 为 0 表示写入记录时失败，跳过
func (n *Notifier) finishLog(id int64, err error) {
	if id == 0 {
		return
	}
	status, msg := store.LogSent, ""
	if err != nil {
		status, msg = store.LogFailed, err.Error()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if ferr := n.st.FinishNotifyLog(ctx, id, status, msg, n.now().Unix()); ferr != nil {
		n.log.Warn("更新通知记录失败", "err", ferr)
	}
}

// Run 运行发送队列，处理事件变更并每 30 秒检查一次，直到 ctx 结束
func (n *Notifier) Run(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		n.sender.Run(ctx)
	}()
	t := time.NewTicker(checkInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			return
		case c := <-n.ev.Changes():
			n.Handle(ctx, c)
		case <-t.C:
			n.Check(ctx)
		}
	}
}
