package notify

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ruanun/simple-server-status/internal/dashboard/incident"
	"github.com/ruanun/simple-server-status/internal/dashboard/store"
	"github.com/ruanun/simple-server-status/internal/dashboard/traffic"
)

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) Now() time.Time      { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *clock) Add(d time.Duration) { c.mu.Lock(); c.t = c.t.Add(d); c.mu.Unlock() }

type fakeSource struct {
	mu      sync.Mutex
	servers []store.Server
	cfg     store.NotifySettings
}

func (f *fakeSource) Servers() []store.Server {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]store.Server(nil), f.servers...)
}

func (f *fakeSource) NotifySettings() store.NotifySettings {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.cfg
}

func (f *fakeSource) update(fn func(cfg *store.NotifySettings)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fn(&f.cfg)
}

type env struct {
	src    *fakeSource
	rec    *incident.Recorder
	st     *store.Store
	tr     *traffic.Accumulator
	clock  *clock
	hook   string
	events chan webhookPayload
}

func newEnv(t *testing.T, servers ...store.Server) *env {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "n.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	c := &clock{t: time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)}
	events := make(chan webhookPayload, 16)
	hook := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		var p webhookPayload
		_ = json.NewDecoder(r.Body).Decode(&p)
		events <- p
	}))
	t.Cleanup(hook.Close)
	cfg := store.DefaultNotifySettings()
	cfg.WebhookURL = hook.URL
	log := slog.New(slog.DiscardHandler)
	return &env{src: &fakeSource{servers: servers, cfg: cfg}, rec: incident.NewRecorder(st, c.Now, log), st: st,
		tr: traffic.New(st, c.Now), clock: c, hook: hook.URL, events: events}
}

func (e *env) notifier(t *testing.T) *Notifier {
	t.Helper()
	log := slog.New(slog.DiscardHandler)
	s := NewSender(fastOptions(), log)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go s.Run(ctx)
	return NewNotifier(e.src, e.rec, e.tr, e.st, s, fastOptions(), e.clock.Now, log)
}

// pump 把已发布的事件变更交给 Notifier 处理
func (e *env) pump(n *Notifier) {
	for {
		select {
		case c := <-e.rec.Changes():
			n.Handle(context.Background(), c)
		default:
			return
		}
	}
}

// notifyState 读取事件在数据库中的推送状态
func (e *env) notifyState(t *testing.T, id int64) int {
	t.Helper()
	list, _, err := e.st.ListEvents(context.Background(), store.EventFilter{}, 100, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, ev := range list {
		if ev.ID == id {
			return ev.NotifyState
		}
	}
	t.Fatalf("事件 %d 不存在", id)
	return -1
}

func (e *env) ongoing(t *testing.T, server, kind string) store.Event {
	t.Helper()
	tr, ok := e.rec.Ongoing(server, kind)
	if !ok {
		t.Fatalf("%s 没有进行中的 %s 事件", server, kind)
	}
	return tr.Event
}

func expectEvent(t *testing.T, ch chan webhookPayload, kind string) webhookPayload {
	t.Helper()
	select {
	case p := <-ch:
		if p.Event != kind {
			t.Fatalf("期望 %s，收到 %+v", kind, p)
		}
		return p
	case <-time.After(3 * time.Second):
		t.Fatalf("未收到 %s 通知", kind)
	}
	return webhookPayload{}
}

func expectNone(t *testing.T, ch chan webhookPayload) {
	t.Helper()
	select {
	case p := <-ch:
		t.Fatalf("不应收到通知，收到 %+v", p)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestOfflineDelayAndRecovery(t *testing.T) {
	e := newEnv(t, store.Server{ID: "s1", Name: "hk"})
	n := e.notifier(t)
	ctx := context.Background()
	n.Check(ctx) // 渠道启用
	e.clock.Add(time.Minute)
	if err := e.rec.Open(ctx, "s1", incident.KindOffline, e.clock.Now().Add(-time.Minute).Unix(), nil); err != nil {
		t.Fatal(err)
	}
	e.pump(n) // 离线的开始变更不直接推送
	n.Check(ctx)
	expectNone(t, e.events)

	e.clock.Add(2 * time.Minute) // 离线满 3 分钟
	n.Check(ctx)
	p := expectEvent(t, e.events, KindOffline)
	if p.ServerName != "hk" || p.Title != "[离线] hk" {
		t.Fatalf("离线通知内容错误 %+v", p)
	}
	ev := e.ongoing(t, "s1", incident.KindOffline)
	waitUntil(t, func() bool { return e.notifyState(t, ev.ID) == store.NotifyDone })
	n.Check(ctx)
	expectNone(t, e.events) // 已推送不重复

	if err := e.rec.Close(ctx, "s1", incident.KindOffline, e.clock.Now().Unix()); err != nil {
		t.Fatal(err)
	}
	e.pump(n)
	expectEvent(t, e.events, KindRecovered)
	waitUntil(t, func() bool {
		logs, _, _ := e.st.ListNotifyLog(ctx, "s1", store.LogSent, 10, 0)
		return len(logs) == 2 && logs[0].EventID != nil && *logs[0].EventID == ev.ID && logs[1].EventID != nil && *logs[1].EventID == ev.ID
	})
}

// 未推送过离线的事件，恢复时不推送恢复通知
func TestRecoveryOnlyAfterNotified(t *testing.T) {
	e := newEnv(t, store.Server{ID: "s1", Name: "hk"})
	n := e.notifier(t)
	ctx := context.Background()
	n.Check(ctx)
	if err := e.rec.Open(ctx, "s1", incident.KindOffline, e.clock.Now().Unix(), nil); err != nil {
		t.Fatal(err)
	}
	e.clock.Add(time.Minute) // 未满推送延迟即恢复
	if err := e.rec.Close(ctx, "s1", incident.KindOffline, e.clock.Now().Unix()); err != nil {
		t.Fatal(err)
	}
	e.pump(n)
	n.Check(ctx)
	expectNone(t, e.events)
}

// 渠道启用之前开始的离线，之后不补发
func TestOfflineBeforeChannelsSkipped(t *testing.T) {
	e := newEnv(t, store.Server{ID: "s1", Name: "hk"})
	e.src.update(func(c *store.NotifySettings) { c.WebhookURL = "" })
	n := e.notifier(t)
	ctx := context.Background()
	n.Check(ctx)
	if err := e.rec.Open(ctx, "s1", incident.KindOffline, e.clock.Now().Unix(), nil); err != nil {
		t.Fatal(err)
	}
	e.pump(n)
	e.clock.Add(time.Minute)
	e.src.update(func(c *store.NotifySettings) { c.WebhookURL = e.hook })
	n.Check(ctx)
	e.clock.Add(5 * time.Minute)
	n.Check(ctx)
	expectNone(t, e.events)
	if s := e.notifyState(t, e.ongoing(t, "s1", incident.KindOffline).ID); s != store.NotifySkipped {
		t.Fatalf("应记为不推送，实际 %d", s)
	}
}

// Dashboard 重启：加载的进行中离线不推送离线通知；重启前已推送的，恢复时仍推送恢复通知
func TestRestartBehavior(t *testing.T) {
	e := newEnv(t, store.Server{ID: "a", Name: "a"}, store.Server{ID: "b", Name: "b"})
	ctx := context.Background()
	start := e.clock.Now().Add(-time.Hour).Unix()
	a := store.Event{ServerID: "a", Kind: incident.KindOffline, StartAt: start}
	b := store.Event{ServerID: "b", Kind: incident.KindOffline, StartAt: start, NotifyState: store.NotifyDone}
	for _, ev := range []*store.Event{&a, &b} {
		if err := e.st.CreateEvent(ctx, ev); err != nil {
			t.Fatal(err)
		}
	}
	if err := e.rec.Load(ctx); err != nil {
		t.Fatal(err)
	}
	n := e.notifier(t)
	n.Check(ctx)
	e.clock.Add(time.Minute)
	n.Check(ctx)
	expectNone(t, e.events)
	if s := e.notifyState(t, a.ID); s != store.NotifySkipped {
		t.Fatalf("加载的离线事件应记为不推送，实际 %d", s)
	}
	if err := e.rec.Close(ctx, "b", incident.KindOffline, e.clock.Now().Unix()); err != nil {
		t.Fatal(err)
	}
	e.pump(n)
	p := expectEvent(t, e.events, KindRecovered)
	if p.ServerID != "b" {
		t.Fatalf("应为 b 的恢复通知 %+v", p)
	}
}

func TestMutedSkipped(t *testing.T) {
	e := newEnv(t, store.Server{ID: "s1", Name: "hk", NotifyMuted: true})
	n := e.notifier(t)
	ctx := context.Background()
	n.Check(ctx)
	if err := e.rec.Open(ctx, "s1", incident.KindOffline, e.clock.Now().Unix(), nil); err != nil {
		t.Fatal(err)
	}
	if err := e.rec.Open(ctx, "s1", incident.KindLoadCPU, e.clock.Now().Unix(), incident.LoadDetail{Threshold: 90, Minutes: 5, Peak: 99}); err != nil {
		t.Fatal(err)
	}
	e.pump(n)
	e.clock.Add(10 * time.Minute)
	n.Check(ctx)
	// 取消静音后不补发
	e.src.mu.Lock()
	e.src.servers[0].NotifyMuted = false
	e.src.mu.Unlock()
	n.Check(ctx)
	expectNone(t, e.events)
	for _, k := range []string{incident.KindOffline, incident.KindLoadCPU} {
		if s := e.notifyState(t, e.ongoing(t, "s1", k).ID); s != store.NotifySkipped {
			t.Fatalf("%s 应记为不推送，实际 %d", k, s)
		}
	}
}

func TestLoadAndRecovery(t *testing.T) {
	e := newEnv(t, store.Server{ID: "s1", Name: "hk"})
	n := e.notifier(t)
	ctx := context.Background()
	if err := e.rec.Open(ctx, "s1", incident.KindLoadCPU, e.clock.Now().Unix(), incident.LoadDetail{Threshold: 90, Minutes: 5, Peak: 97.5}); err != nil {
		t.Fatal(err)
	}
	e.pump(n)
	p := expectEvent(t, e.events, KindLoad)
	if p.Title != "[高负载] hk" || !strings.Contains(p.Message, "CPU 97.5%") {
		t.Fatalf("高负载通知内容错误 %+v", p)
	}
	if err := e.rec.Close(ctx, "s1", incident.KindLoadCPU, e.clock.Now().Unix()); err != nil {
		t.Fatal(err)
	}
	e.pump(n) // 结束时可能仍在发送中，由发送成功的回调推送恢复
	p = expectEvent(t, e.events, KindLoadRecovered)
	if !strings.Contains(p.Message, "CPU") {
		t.Fatalf("恢复通知应包含指标 %+v", p)
	}
}

func TestSwitchesAndNoChannels(t *testing.T) {
	e := newEnv(t, store.Server{ID: "s1", Name: "hk"})
	e.src.update(func(c *store.NotifySettings) { c.LoadEnabled, c.IPChangeEnabled = false, false })
	n := e.notifier(t)
	ctx := context.Background()
	now := e.clock.Now().Unix()
	if err := e.rec.Open(ctx, "s1", incident.KindLoadMem, now, incident.LoadDetail{Threshold: 90, Minutes: 5, Peak: 95}); err != nil {
		t.Fatal(err)
	}
	if err := e.rec.Instant(ctx, "s1", incident.KindIPChange, now, map[string][2]string{"ipv4": {"1.1.1.1", "2.2.2.2"}}); err != nil {
		t.Fatal(err)
	}
	if err := e.rec.Instant(ctx, "s1", incident.KindReboot, now, nil); err != nil {
		t.Fatal(err)
	}
	e.pump(n)
	p := expectEvent(t, e.events, KindReboot)
	if p.Title != "[重启] hk" {
		t.Fatalf("重启通知内容错误 %+v", p)
	}
	expectNone(t, e.events) // 负载与 IP 变化的开关已关闭

	e.src.update(func(c *store.NotifySettings) { c.WebhookURL = "" })
	if err := e.rec.Instant(ctx, "s1", incident.KindReboot, now+600, nil); err != nil {
		t.Fatal(err)
	}
	e.pump(n)
	list, _, _ := e.st.ListEvents(ctx, store.EventFilter{Kinds: []string{incident.KindReboot}}, 10, 0)
	if len(list) != 2 || list[0].NotifyState != store.NotifySkipped {
		t.Fatalf("没有渠道时应记为不推送 %+v", list)
	}
}

func TestExpireOnceAcrossRestart(t *testing.T) {
	exp := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC).Unix()
	e := newEnv(t, store.Server{ID: "s1", Name: "hk", ExpireAt: &exp})
	ctx := context.Background()
	e.notifier(t).Check(ctx)
	expectEvent(t, e.events, KindExpire)
	// 投递成功的回调在 Webhook 响应后才记为已提醒，等其落库再模拟重启
	key := strconv.FormatInt(exp, 10)
	waitUntil(t, func() bool { ok, _ := e.st.NotifySent(ctx, "s1", RuleExpire, key); return ok })
	e.notifier(t).Check(ctx) // 模拟重启：新的 Notifier、同一数据库
	expectNone(t, e.events)
}

func TestExpireRetriedAfterAllChannelsFail(t *testing.T) {
	exp := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC).Unix()
	e := newEnv(t, store.Server{ID: "s1", Name: "hk", ExpireAt: &exp})
	var calls atomic.Int32
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer bad.Close()
	e.src.cfg.WebhookURL = bad.URL
	n := e.notifier(t)
	ctx := context.Background()
	n.Check(ctx)
	waitUntil(t, func() bool { return calls.Load() == 4 }) // 首次 + 3 次重试
	key := inflightKey("s1", Mark{Rule: RuleExpire, Key: strconv.FormatInt(exp, 10)})
	waitUntil(t, func() bool { // 等失败记录写入且「处理中」已清除
		_, total, _ := e.st.ListNotifyLog(ctx, "s1", store.LogFailed, 10, 0)
		return total == 1 && !n.isInflight(key)
	})
	if ok, _ := e.st.NotifySent(ctx, "s1", RuleExpire, strconv.FormatInt(exp, 10)); ok {
		t.Fatal("全部失败时不应记为已提醒")
	}
	logs, _, _ := e.st.ListNotifyLog(ctx, "s1", store.LogFailed, 10, 0)
	if len(logs) != 1 || logs[0].Error == "" || logs[0].EventID != nil {
		t.Fatalf("应有一条失败记录且不关联事件 %+v", logs)
	}
	if strings.Contains(logs[0].Error, bad.URL) {
		t.Fatalf("失败原因不应包含 Webhook 地址: %s", logs[0].Error)
	}
	n.Check(ctx)
	waitUntil(t, func() bool { return calls.Load() >= 5 })
}

// 离线通知全部渠道失败后记为推送失败，不再重试；之后恢复也不推送恢复通知
func TestOfflineFailedNotRetried(t *testing.T) {
	e := newEnv(t, store.Server{ID: "s1", Name: "hk"})
	var calls atomic.Int32
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer bad.Close()
	e.src.cfg.WebhookURL = bad.URL
	n := e.notifier(t)
	ctx := context.Background()
	n.Check(ctx)
	if err := e.rec.Open(ctx, "s1", incident.KindOffline, e.clock.Now().Unix(), nil); err != nil {
		t.Fatal(err)
	}
	e.clock.Add(5 * time.Minute)
	n.Check(ctx)
	ev := e.ongoing(t, "s1", incident.KindOffline)
	// 首次 + 3 次重试后记为推送失败
	waitUntil(t, func() bool { return calls.Load() == 4 && e.notifyState(t, ev.ID) == store.NotifyFailed })
	n.Check(ctx)
	if err := e.rec.Close(ctx, "s1", incident.KindOffline, e.clock.Now().Unix()); err != nil {
		t.Fatal(err)
	}
	e.pump(n)
	time.Sleep(200 * time.Millisecond)
	if c := calls.Load(); c != 4 {
		t.Fatalf("不应重试，也不应推送恢复，请求 %d 次", c)
	}
}

// 「发生」通知投递期间事件已结束：投递成功后补推恢复通知，失败则不推送
func TestClosedWhileInflight(t *testing.T) {
	for _, ok := range []bool{true, false} {
		e := newEnv(t, store.Server{ID: "s1", Name: "hk"})
		release := make(chan struct{})
		var mu sync.Mutex
		var got []string
		hook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var p webhookPayload
			_ = json.NewDecoder(r.Body).Decode(&p)
			if p.Event == KindLoad {
				<-release
				if !ok {
					w.WriteHeader(http.StatusInternalServerError)
				}
			}
			mu.Lock()
			got = append(got, p.Event)
			mu.Unlock()
		}))
		e.src.cfg.WebhookURL = hook.URL
		n := e.notifier(t)
		ctx := context.Background()
		if err := e.rec.Open(ctx, "s1", incident.KindLoadCPU, e.clock.Now().Unix(), incident.LoadDetail{Threshold: 90, Minutes: 5, Peak: 95}); err != nil {
			t.Fatal(err)
		}
		e.pump(n)
		ev := e.ongoing(t, "s1", incident.KindLoadCPU)
		waitUntil(t, func() bool { return e.notifyState(t, ev.ID) == store.NotifySending })
		if err := e.rec.Close(ctx, "s1", incident.KindLoadCPU, e.clock.Now().Unix()); err != nil {
			t.Fatal(err)
		}
		e.pump(n)
		close(release)
		count := func(kind string) int {
			mu.Lock()
			defer mu.Unlock()
			c := 0
			for _, k := range got {
				if k == kind {
					c++
				}
			}
			return c
		}
		if ok {
			waitUntil(t, func() bool { return count(KindLoadRecovered) == 1 })
		} else {
			waitUntil(t, func() bool { return count(KindLoad) == 4 && e.notifyState(t, ev.ID) == store.NotifyFailed })
			time.Sleep(200 * time.Millisecond)
			if count(KindLoadRecovered) != 0 {
				t.Fatal("高负载通知失败时不应推送恢复")
			}
		}
		hook.Close()
	}
}

func TestInflightNotDuplicatedAndMarkedOnSuccess(t *testing.T) {
	exp := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC).Unix()
	e := newEnv(t, store.Server{ID: "s1", Name: "hk", ExpireAt: &exp})
	release := make(chan struct{})
	var calls atomic.Int32
	slow := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls.Add(1)
		<-release
	}))
	defer slow.Close()
	e.src.cfg.WebhookURL = slow.URL
	n := e.notifier(t)
	ctx := context.Background()
	n.Check(ctx)
	waitUntil(t, func() bool { return calls.Load() == 1 })
	n.Check(ctx) // 投递进行中，不应重复入队
	time.Sleep(100 * time.Millisecond)
	if calls.Load() != 1 {
		t.Fatalf("进行中不应重复发送，请求 %d 次", calls.Load())
	}
	close(release)
	key := strconv.FormatInt(exp, 10)
	waitUntil(t, func() bool { ok, _ := e.st.NotifySent(ctx, "s1", RuleExpire, key); return ok })
	logs, _, _ := e.st.ListNotifyLog(ctx, "s1", store.LogSent, 10, 0)
	if len(logs) != 1 || logs[0].Kind != KindExpire || logs[0].Channel != "webhook" || logs[0].DoneAt == nil {
		t.Fatalf("应有一条成功记录 %+v", logs)
	}
}
