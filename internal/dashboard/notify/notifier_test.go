package notify

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/ruanun/simple-server-status/internal/dashboard/hub"
	"github.com/ruanun/simple-server-status/internal/dashboard/store"
	"github.com/ruanun/simple-server-status/internal/dashboard/traffic"
	"github.com/ruanun/simple-server-status/internal/proto"
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

type env struct {
	src    *fakeSource
	hub    *hub.Hub
	st     *store.Store
	tr     *traffic.Accumulator
	clock  *clock
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
	e := &env{src: &fakeSource{servers: servers, cfg: cfg}, hub: hub.New(c.Now), st: st, tr: traffic.New(st, c.Now), clock: c, events: events}
	return e
}

func (e *env) notifier(t *testing.T) *Notifier {
	t.Helper()
	log := slog.New(slog.DiscardHandler)
	s := NewSender(fastOptions(), log)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go s.Run(ctx)
	return NewNotifier(e.src, e.hub, e.tr, e.st, s, fastOptions(), e.clock.Now, log)
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

func TestNotifierOfflineRecovery(t *testing.T) {
	e := newEnv(t, store.Server{ID: "s1", Name: "hk"})
	n := e.notifier(t)
	ctx := context.Background()
	e.hub.Connect("s1", 2)
	e.hub.Report("s1", proto.Report{})
	n.Check(ctx) // 首轮：在线，建立基线
	e.clock.Add(10 * time.Minute)
	n.Check(ctx)
	p := expectEvent(t, e.events, KindOffline)
	if p.ServerName != "hk" || p.Title != "[离线] hk" {
		t.Fatalf("离线通知内容错误 %+v", p)
	}
	e.hub.Report("s1", proto.Report{})
	n.Check(ctx)
	expectEvent(t, e.events, KindRecovered)
}

func TestNotifierBaselineAfterRestart(t *testing.T) {
	e := newEnv(t, store.Server{ID: "s1", Name: "hk", LastSeen: time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC).Unix()})
	n := e.notifier(t)
	ctx := context.Background()
	n.Check(ctx)
	n.Check(ctx)
	expectNone(t, e.events)
}

func TestNotifierMuted(t *testing.T) {
	e := newEnv(t, store.Server{ID: "s1", Name: "hk", NotifyMuted: true, LastSeen: 1})
	n := e.notifier(t)
	ctx := context.Background()
	n.Check(ctx)
	e.clock.Add(time.Hour)
	n.Check(ctx)
	expectNone(t, e.events)
}

func TestNotifierExpireOnceAcrossRestart(t *testing.T) {
	exp := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC).Unix()
	e := newEnv(t, store.Server{ID: "s1", Name: "hk", ExpireAt: &exp})
	ctx := context.Background()
	e.notifier(t).Check(ctx)
	expectEvent(t, e.events, KindExpire)
	e.notifier(t).Check(ctx) // 模拟重启：新的 Notifier、同一数据库
	expectNone(t, e.events)
}

func TestNotifierHighLoad(t *testing.T) {
	e := newEnv(t, store.Server{ID: "s1", Name: "hk"})
	n := e.notifier(t)
	ctx := context.Background()
	e.hub.Connect("s1", 2)
	e.hub.Report("s1", proto.Report{CPU: 99})
	n.Check(ctx) // 首轮：只有 1 个点，不判断负载
	for i := 0; i < 6; i++ {
		e.clock.Add(30 * time.Second)
		e.hub.Report("s1", proto.Report{CPU: 99})
	}
	n.Check(ctx)
	expectEvent(t, e.events, KindLoad)
}

// Dashboard 停机较久时，Agent 按退避在首轮之后陆续重连：基线期内上线不应发出「已恢复」
func TestNotifierBaselineWindowAfterRestart(t *testing.T) {
	e := newEnv(t, store.Server{ID: "s1", Name: "hk", LastSeen: time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC).Unix()})
	n := e.notifier(t)
	ctx := context.Background()
	n.Check(ctx) // 首轮：离线，只建立基线
	e.clock.Add(60 * time.Second)
	e.hub.Connect("s1", 2)
	e.hub.Report("s1", proto.Report{})
	n.Check(ctx) // 基线期内重连：静默清除
	expectNone(t, e.events)

	// 基线期结束后再离线，正常通知
	e.clock.Add(2 * time.Minute)
	e.hub.Report("s1", proto.Report{})
	n.Check(ctx)
	expectNone(t, e.events)
	e.clock.Add(10 * time.Minute)
	n.Check(ctx)
	p := expectEvent(t, e.events, KindOffline)
	if p.Title != "[离线] hk" {
		t.Fatalf("离线通知内容错误 %+v", p)
	}
}

// 渠道从无到有时重新走基线期，不补发配置前的状态变化
func TestNotifierRebaselineWhenChannelsConfigured(t *testing.T) {
	e := newEnv(t, store.Server{ID: "s1", Name: "hk", LastSeen: time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC).Unix()})
	hook := e.src.cfg.WebhookURL
	setHook := func(url string) {
		e.src.mu.Lock()
		e.src.cfg.WebhookURL = url
		e.src.mu.Unlock()
	}
	n := e.notifier(t)
	ctx := context.Background()

	// 渠道先为空再配置：配置后第一轮不补发
	setHook("")
	n.Check(ctx)
	e.clock.Add(5 * time.Minute)
	setHook(hook)
	n.Check(ctx)
	expectNone(t, e.events)

	// 基线期结束后移除渠道；期间服务器恢复在线，再配置渠道后不补发陈旧的恢复通知
	e.clock.Add(5 * time.Minute)
	n.Check(ctx)
	expectNone(t, e.events)
	setHook("")
	e.hub.Connect("s1", 2)
	e.hub.Report("s1", proto.Report{})
	n.Check(ctx)
	e.clock.Add(5 * time.Second)
	setHook(hook)
	n.Check(ctx)
	expectNone(t, e.events)
}
