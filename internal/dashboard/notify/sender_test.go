package notify

import (
	"bytes"
	"context"
	"encoding/json"
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
	s.Enqueue(Channels(store.NotifySettings{WebhookURL: srv.URL}, fastOptions()), e)
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
	s.Enqueue(Channels(cfg, o), Event{Kind: KindTest})
	waitUntil(t, func() bool { return strings.Count(logs.String(), "通知发送失败") == 2 })
	if out := logs.String(); strings.Contains(out, "TGSECRET") || strings.Contains(out, "HOOKSECRET") {
		t.Fatalf("日志泄露密钥: %s", out)
	}
}

func TestQueueDropsOldest(t *testing.T) {
	s := NewSender(fastOptions(), slog.New(slog.DiscardHandler))
	chs := Channels(store.NotifySettings{WebhookURL: "http://127.0.0.1:1"}, fastOptions())
	for i := 0; i <= queueCap; i++ {
		s.Enqueue(chs, Event{Kind: KindTest, Time: int64(i)})
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
		s.Enqueue(chs, Event{Kind: KindTest, Time: int64(i)})
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
	res := SendTest(context.Background(), store.NotifySettings{TelegramToken: "123:abc", TelegramChatID: "42", Lang: "en-US"}, o, time.Now())
	if res["webhook"] != nil || res["telegram"] == nil || *res["telegram"] != "ok" {
		t.Fatalf("结果错误 %v", res)
	}
	if got["chat_id"] != "42" || !strings.HasPrefix(got["text"], "[Test] Simple Server Status\n") {
		t.Fatalf("Telegram 内容错误 %v", got)
	}
}
