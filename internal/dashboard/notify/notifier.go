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

// StateStore 一次性提醒与通知记录的持久化（store.Store 实现）
type StateStore interface {
	LogStore
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

	pmu      sync.Mutex      // 保护 inflight，发送回调中使用，不得换成 mu
	inflight map[string]bool // 投递中的一次性提醒（serverID|rule|key）
}

// NewNotifier 创建 Notifier
func NewNotifier(src Source, h *hub.Hub, tr *traffic.Accumulator, st StateStore, sender *Sender, o Options, now func() time.Time, log *slog.Logger) *Notifier {
	return &Notifier{src: src, hub: h, tr: tr, st: st, sender: sender, o: o, now: now, log: log, states: map[string]*State{}, inflight: map[string]bool{}}
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
			if n.isInflight(id + "|" + rule + "|" + key) {
				return true
			}
			ok, err := n.st.NotifySent(ctx, id, rule, key)
			if err != nil {
				n.log.Warn("读取通知记录失败", "id", id, "err", err)
				return true // 无法确认时不发送，避免重复提醒
			}
			return ok
		}
		events := Evaluate(now, cfg, n.observe(ctx, srv), st, baseline, sent)
		for i := range events {
			Render(&events[i], cfg.Lang)
			n.dispatch(chs, events[i])
		}
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

// dispatch 为每个渠道写入发送中的记录并入队；一次性提醒在任一渠道成功后才记为已提醒，
// 投递期间视为已提醒以免重复入队，全部失败后撤销，下一轮会重新提醒。
// 回调在发送队列协程中（可能并发）执行，不得获取 n.mu（Check 持有 n.mu 时会入队）
func (n *Notifier) dispatch(chs []Channel, e Event) {
	if len(chs) == 0 {
		return // 没有渠道就不会有回调，不能标记投递中
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ids := map[string]int64{}
	for _, ch := range chs {
		id, err := n.st.AddNotifyLog(ctx, store.NotifyLog{ServerID: e.ServerID, ServerName: e.ServerName, Kind: e.Kind,
			Channel: ch.Name(), Title: e.Title, Message: e.Message, Status: store.LogPending, CreatedAt: e.Time})
		if err != nil {
			n.log.Warn("写入通知记录失败", "id", e.ServerID, "err", err)
			continue
		}
		ids[ch.Name()] = id
	}
	key := ""
	if e.Mark != nil {
		key = inflightKey(e.ServerID, *e.Mark)
		n.setInflight(key, true)
	}
	var mu sync.Mutex
	remaining, delivered := len(chs), false
	n.sender.Enqueue(chs, e, func(channel string, err error) {
		n.finishLog(ids[channel], err)
		mu.Lock()
		defer mu.Unlock()
		remaining--
		if err == nil && !delivered {
			delivered = true
			if e.Mark != nil {
				mctx, mcancel := context.WithTimeout(context.Background(), 5*time.Second)
				if merr := n.st.MarkNotified(mctx, e.ServerID, e.Mark.Rule, e.Mark.Key, n.now().Unix()); merr != nil {
					n.log.Warn("记录已提醒失败", "id", e.ServerID, "err", merr)
				}
				mcancel()
				n.setInflight(key, false)
			}
		}
		if remaining == 0 && !delivered && e.Mark != nil {
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
