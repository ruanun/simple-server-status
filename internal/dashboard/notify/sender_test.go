package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ruanun/simple-server-status/internal/dashboard/store"
)

func fastOptions() Options {
	return Options{RetryDelays: []time.Duration{10 * time.Millisecond, 10 * time.Millisecond, 10 * time.Millisecond}, Timeout: time.Second}
}

// syncBuffer 并发安全的日志缓冲
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func waitUntil(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("等待超时")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestWebhookPayloadAndRetry(t *testing.T) {
	var calls atomic.Int32
	var got webhookPayload
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		mu.Lock()
		_ = json.NewDecoder(r.Body).Decode(&got)
		mu.Unlock()
	}))
	defer srv.Close()
	s := NewSender(fastOptions(), slog.New(slog.DiscardHandler))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Run(ctx)
	e := Event{Kind: KindOffline, ServerID: "s1", ServerName: "hk", Time: 123, Minutes: 3}
	Render(&e, "zh-CN")
	s.Enqueue(Channels(store.NotifySettings{WebhookURL: srv.URL}, fastOptions()), e, nil)
	waitUntil(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return got.Event != ""
	})
	mu.Lock()
	defer mu.Unlock()
	if calls.Load() != 3 || got.Event != KindOffline || got.ServerID != "s1" || got.ServerName != "hk" || got.Time != 123 || got.Title != "[离线] hk" || got.Message == "" {
		t.Fatalf("Webhook 内容错误 %+v", got)
	}
}

func TestFailureLogHidesSecrets(t *testing.T) {
	tg := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"ok":false,"description":"Unauthorized"}`))
	}))
	defer tg.Close()
	dead := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	deadURL := dead.URL + "/hook?key=HOOKSECRET"
	dead.Close() // 连接被拒绝，错误中会带完整 URL
	var logs syncBuffer
	o := fastOptions()
	o.TelegramBase = tg.URL
	s := NewSender(o, slog.New(slog.NewTextHandler(&logs, nil)))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Run(ctx)
	cfg := store.NotifySettings{WebhookURL: deadURL, TelegramToken: "123:TGSECRET", TelegramChatID: "42"}
	s.Enqueue(Channels(cfg, o), Event{Kind: KindTest}, nil)
	waitUntil(t, func() bool { return strings.Count(logs.String(), "通知发送失败") == 2 })
	if out := logs.String(); strings.Contains(out, "TGSECRET") || strings.Contains(out, "HOOKSECRET") {
		t.Fatalf("日志泄露密钥: %s", out)
	}
}

func TestQueueDropsOldest(t *testing.T) {
	s := NewSender(fastOptions(), slog.New(slog.DiscardHandler))
	chs := Channels(store.NotifySettings{WebhookURL: "http://127.0.0.1:1"}, fastOptions())
	for i := 0; i <= queueCap; i++ {
		s.Enqueue(chs, Event{Kind: KindTest, Time: int64(i)}, nil)
	}
	if len(s.queue) != queueCap || s.queue[0].ev.Time != 1 || s.queue[queueCap-1].ev.Time != int64(queueCap) {
		t.Fatalf("队列长度 %d，首条 %d", len(s.queue), s.queue[0].ev.Time)
	}
}

// TestRunShutdownRespectsDrainDeadline 验证 ctx 结束时 Run 不会被阻塞在一条条渠道请求上：
// 渠道服务端一直不响应（模拟卡死的对端），把 drainWait 缩短到 200ms 后，Run 应在远小于 1 秒内返回，
// 而不是等满每条请求各自的超时时间。
//
// 服务端 handler 用测试自己控制的 channel 挂起，而不是等待 r.Context().Done()：net/http 的服务端在
// handler 阻塞、不再读写连接期间，并不会因为客户端放弃请求就可靠地取消 r.Context()（这依赖服务端主动
// 检测连接关闭），曾导致 handler 永远收不到取消信号、httptest.Server.Close 也随之永久阻塞。真正决定
// Run 能否按时返回的是客户端侧的请求超时（由 deliver 的 parent ctx 控制），与服务端是否感知无关。
func TestRunShutdownRespectsDrainDeadline(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		<-block // 直到测试主动放行才返回，避免依赖服务端对客户端取消的检测
	}))
	defer srv.Close()
	defer close(block) // 先于 srv.Close 执行，放行 handler，避免 Close 卡住

	o := Options{RetryDelays: []time.Duration{10 * time.Millisecond}, Timeout: 3 * time.Second}
	s := NewSender(o, slog.New(slog.DiscardHandler))
	s.drainWait = 200 * time.Millisecond
	chs := Channels(store.NotifySettings{WebhookURL: srv.URL}, o)
	for i := 0; i < 3; i++ {
		s.Enqueue(chs, Event{Kind: KindTest, Time: int64(i)}, nil)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 队列尚未开始处理就收到退出信号，模拟启动阶段/积压较多时的关闭
	done := make(chan struct{})
	go func() {
		s.Run(ctx)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("关闭应在 drainWait 附近返回，实际超过 1 秒")
	}
}

func TestSendTest(t *testing.T) {
	var got map[string]string
	tg := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bot123:abc/sendMessage" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer tg.Close()
	o := fastOptions()
	o.TelegramBase = tg.URL
	res := SendTest(context.Background(), store.NotifySettings{TelegramToken: "123:abc", TelegramChatID: "42", Lang: "en-US"}, o, time.Now(), nil)
	if res["webhook"] != nil || res["telegram"] == nil || *res["telegram"] != "ok" {
		t.Fatalf("结果错误 %v", res)
	}
	if got["chat_id"] != "42" || !strings.HasPrefix(got["text"], "[Test] Simple Server Status\n") {
		t.Fatalf("Telegram 内容错误 %v", got)
	}
}

type result struct {
	channel string
	err     error
}

func collect(n int) (func(string, error), func(t *testing.T) []result) {
	ch := make(chan result, n+10)
	return func(c string, err error) { ch <- result{c, err} }, func(t *testing.T) []result {
		t.Helper()
		var out []result
		for len(out) < n {
			select {
			case r := <-ch:
				out = append(out, r)
			case <-time.After(3 * time.Second):
				t.Fatalf("只收到 %d/%d 个完成回调", len(out), n)
			}
		}
		select {
		case r := <-ch:
			t.Fatalf("回调次数过多，多出 %+v", r)
		case <-time.After(100 * time.Millisecond):
		}
		return out
	}
}

func TestDoneCalledOnceOnSuccessAndFailure(t *testing.T) {
	ok := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer ok.Close()
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) }))
	defer bad.Close()
	s := NewSender(fastOptions(), slog.New(slog.DiscardHandler))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Run(ctx)
	done, wait := collect(2)
	chs := []Channel{
		Channels(store.NotifySettings{WebhookURL: ok.URL}, fastOptions())[0],
		renamed{Channels(store.NotifySettings{WebhookURL: bad.URL}, fastOptions())[0], "telegram"}, // 改名以区分两个渠道
	}
	s.Enqueue(chs, Event{Kind: KindTest}, done)
	got := map[string]error{}
	for _, r := range wait(t) {
		got[r.channel] = r.err
	}
	if got["webhook"] != nil || got["telegram"] == nil {
		t.Fatalf("成功应为 nil、重试用尽应返回错误 %+v", got)
	}
}

// renamed 包装渠道以改变名称（测试用）
type renamed struct {
	Channel
	name string
}

func (r renamed) Name() string { return r.name }

func TestDoneOnDropAndShutdown(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { <-block }))
	defer srv.Close()
	defer close(block)
	s := NewSender(fastOptions(), slog.New(slog.DiscardHandler))
	s.drainWait = 300 * time.Millisecond
	chs := Channels(store.NotifySettings{WebhookURL: srv.URL}, fastOptions())
	done, wait := collect(queueCap + 1)
	for i := 0; i <= queueCap; i++ {
		s.Enqueue(chs, Event{Kind: KindTest, Time: int64(i)}, done)
	}
	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan struct{})
	go func() { s.Run(ctx); close(finished) }()
	time.Sleep(100 * time.Millisecond) // 第一条正在发送（被服务端阻塞）
	start := time.Now()
	cancel()
	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("退出未在收尾时限内完成")
	}
	if time.Since(start) > time.Second {
		t.Fatalf("退出耗时 %v，超过收尾时限过多", time.Since(start))
	}
	var dropped, shutdown int
	for _, r := range wait(t) {
		switch {
		case errors.Is(r.err, ErrDropped):
			dropped++
		case errors.Is(r.err, ErrShutdown):
			shutdown++
		default:
			t.Fatalf("退出时未发出的任务应以 ErrShutdown 结束，实际 %v", r.err)
		}
	}
	if dropped != 1 || shutdown != queueCap {
		t.Fatalf("应 1 条被丢弃、其余在退出时失败，实际丢弃 %d、失败 %d", dropped, shutdown)
	}
}

// TestDoneOnShutdownWhileWaitingRetry 等待重试（尚在定时器中）的任务在退出时也要结束并回调，
// 不能等到定时器触发（重试间隔可能长达数分钟，进程早已退出）
func TestDoneOnShutdownWhileWaitingRetry(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	o := Options{RetryDelays: []time.Duration{time.Minute}, Timeout: time.Second}
	s := NewSender(o, slog.New(slog.DiscardHandler))
	s.drainWait = 300 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan struct{})
	go func() { s.Run(ctx); close(finished) }()
	done, wait := collect(1)
	s.Enqueue(Channels(store.NotifySettings{WebhookURL: srv.URL}, o), Event{Kind: KindTest}, done)
	waitUntil(t, func() bool { return calls.Load() == 1 }) // 首次失败，进入 1 分钟的重试等待
	cancel()
	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("退出未在收尾时限内完成")
	}
	if r := wait(t); r[0].err == nil {
		t.Fatalf("退出时未发出的任务应以错误结束 %+v", r)
	}
	if calls.Load() != 2 {
		t.Fatalf("收尾应再尝试一次，实际请求 %d 次", calls.Load())
	}
}

func TestSendTestParallelAndLogged(t *testing.T) {
	slow := func() *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(300 * time.Millisecond)
			if strings.Contains(r.URL.Path, "sendMessage") {
				_, _ = w.Write([]byte(`{"ok":true}`))
			}
		}))
	}
	hook, tg := slow(), slow()
	defer hook.Close()
	defer tg.Close()
	o := fastOptions()
	o.TelegramBase = tg.URL
	logs := &memLogs{}
	start := time.Now()
	res := SendTest(context.Background(), store.NotifySettings{WebhookURL: hook.URL, TelegramToken: "1:a", TelegramChatID: "2"}, o, time.Now(), logs)
	if el := time.Since(start); el > 550*time.Millisecond {
		t.Fatalf("两个渠道应并行，耗时 %v", el)
	}
	if *res["webhook"] != "ok" || *res["telegram"] != "ok" {
		t.Fatalf("结果 %v", res)
	}
	if len(logs.rows) != 2 || logs.rows[0].Status != store.LogSent || logs.rows[1].Status != store.LogSent || logs.rows[0].Kind != KindTest {
		t.Fatalf("测试消息应写入两条成功记录 %+v", logs.rows)
	}
}

// memLogs 内存中的通知记录（测试用）
type memLogs struct {
	mu   sync.Mutex
	rows []store.NotifyLog
}

func (m *memLogs) AddNotifyLog(_ context.Context, l store.NotifyLog) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rows = append(m.rows, l)
	return int64(len(m.rows)), nil
}

func (m *memLogs) FinishNotifyLog(_ context.Context, id int64, status, errMsg string, doneAt int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := &m.rows[id-1]
	r.Status, r.Error, r.DoneAt = status, errMsg, &doneAt
	return nil
}
