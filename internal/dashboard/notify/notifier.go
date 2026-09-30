package notify

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/ruanun/simple-server-status/internal/dashboard/hub"
	"github.com/ruanun/simple-server-status/internal/dashboard/store"
	"github.com/ruanun/simple-server-status/internal/dashboard/traffic"
)

// checkInterval 两轮检查的间隔
const checkInterval = 30 * time.Second

// baselineWindow 首次检查开始后的基线期：期间离线、高负载只建立状态，恢复静默清除。
// 需大于 Agent 重连退避上限（60 秒 ±20%，约 72 秒），让 Dashboard 重启后陆续重连的 Agent 不触发通知
const baselineWindow = 2 * time.Minute

// Source 通知检查所需的服务器列表与设置（由 api.API 提供）
type Source interface {
	Servers() []store.Server
	NotifySettings() store.NotifySettings
}

// StateStore 一次性提醒的持久化（store.Store 实现）
type StateStore interface {
	NotifySent(ctx context.Context, serverID, rule, key string) (bool, error)
	MarkNotified(ctx context.Context, serverID, rule, key string, ts int64) error
}

// Notifier 每 30 秒检查所有服务器，状态变化时通过发送队列发出通知
type Notifier struct {
	src    Source
	hub    *hub.Hub
	tr     *traffic.Accumulator
	st     StateStore
	sender *Sender
	o      Options
	now    func() time.Time
	log    *slog.Logger

	mu          sync.Mutex
	states      map[string]*State
	firstCheck  time.Time // 首次（有渠道的）检查时间，零值表示尚未开始
	hadChannels bool      // 上一轮是否配置了渠道
}

// NewNotifier 创建 Notifier
func NewNotifier(src Source, h *hub.Hub, tr *traffic.Accumulator, st StateStore, sender *Sender, o Options, now func() time.Time, log *slog.Logger) *Notifier {
	return &Notifier{src: src, hub: h, tr: tr, st: st, sender: sender, o: o, now: now, log: log, states: map[string]*State{}}
}

// Check 执行一轮检查；未配置任何渠道时跳过，渠道从无到有时重置状态并重新走基线期
func (n *Notifier) Check(ctx context.Context) {
	cfg := n.src.NotifySettings()
	chs := Channels(cfg, n.o)
	n.mu.Lock()
	defer n.mu.Unlock()
	if len(chs) == 0 {
		n.hadChannels = false
		return
	}
	if !n.hadChannels {
		n.hadChannels = true
		n.firstCheck = time.Time{}
		n.states = map[string]*State{}
	}
	now := n.now()
	if n.firstCheck.IsZero() {
		n.firstCheck = now
	}
	baseline := now.Sub(n.firstCheck) < baselineWindow
	for _, srv := range n.src.Servers() {
		st := n.states[srv.ID]
		if st == nil {
			st = &State{}
			n.states[srv.ID] = st
		}
		id := srv.ID
		sent := func(rule, key string) bool {
			ok, err := n.st.NotifySent(ctx, id, rule, key)
			if err != nil {
				n.log.Warn("读取通知记录失败", "id", id, "err", err)
				return true // 无法确认时不发送，避免重复提醒
			}
			return ok
		}
		events, marks := Evaluate(now, cfg, n.observe(ctx, srv), st, baseline, sent)
		for _, m := range marks {
			if err := n.st.MarkNotified(ctx, id, m.Rule, m.Key, now.Unix()); err != nil {
				n.log.Warn("记录通知失败", "id", id, "err", err)
			}
		}
		for i := range events {
			Render(&events[i], cfg.Lang)
			n.sender.Enqueue(chs, events[i])
		}
	}
}

// observe 汇总一台服务器的实时状态、历史点、到期与流量
func (n *Notifier) observe(ctx context.Context, srv store.Server) Observation {
	live := n.hub.Get(srv.ID)
	last := live.LastReport
	if last == 0 {
		last = srv.LastSeen
	}
	o := Observation{ID: srv.ID, Name: srv.Name, Muted: srv.NotifyMuted, Online: live.Online, LastSeen: last,
		Ring: n.hub.Ring(srv.ID), ExpireAt: srv.ExpireAt}
	if srv.TrafficLimit != nil {
		in, out, period, err := n.tr.Usage(ctx, srv.ID, srv.TrafficResetDay)
		if err != nil {
			n.log.Warn("读取流量失败", "id", srv.ID, "err", err)
		} else {
			o.TrafficUsed, o.TrafficLimit, o.Period = traffic.Used(srv.TrafficMode, in, out), srv.TrafficLimit, period
		}
	}
	return o
}

// Forget 删除服务器时清理其告警状态
func (n *Notifier) Forget(id string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	delete(n.states, id)
}

// Run 运行发送队列并每 30 秒检查一次，直到 ctx 结束
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
		case <-t.C:
			n.Check(ctx)
		}
	}
}
