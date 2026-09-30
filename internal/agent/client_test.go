package agent

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/ruanun/simple-server-status/internal/agent/collect"
	"github.com/ruanun/simple-server-status/internal/proto"
)

type fakeSampler struct {
	mu     sync.Mutex
	filter collect.Filter
}

func (f *fakeSampler) Static(v string) proto.Hello { return proto.Hello{AgentVersion: v, CPUCores: 2} }
func (f *fakeSampler) Sample() proto.Report        { return proto.Report{CPU: 5} }
func (f *fakeSampler) SetFilter(x collect.Filter) {
	f.mu.Lock()
	f.filter = x
	f.mu.Unlock()
}

func TestClientHandshakeReportAndStop(t *testing.T) {
	gotAuth := make(chan string, 1)
	gotHello := make(chan proto.Hello, 1)
	gotReport := make(chan proto.Report, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth <- r.Header.Get("Authorization")
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		ctx := r.Context()
		_, b, err := conn.Read(ctx)
		if err != nil {
			return
		}
		env, _ := proto.Decode(b)
		var h proto.Hello
		_ = json.Unmarshal(env.Data, &h)
		gotHello <- h
		cfg, _ := proto.Encode(proto.TypeConfig, proto.Config{ReportInterval: 1, NICInclude: []string{"eth"}, FilterID: "f-123"})
		_ = conn.Write(ctx, websocket.MessageText, cfg)
		_, b, err = conn.Read(ctx)
		if err != nil {
			return
		}
		env, _ = proto.Decode(b)
		var rep proto.Report
		_ = json.Unmarshal(env.Data, &rep)
		gotReport <- rep
		stop, _ := proto.Encode(proto.TypeStop, proto.Stop{Reason: "deleted"})
		_ = conn.Write(ctx, websocket.MessageText, stop)
		_, _, _ = conn.Read(ctx) // 等待客户端关闭连接
	}))
	defer srv.Close()

	fs := &fakeSampler{}
	c, err := NewClient(Config{Dashboard: srv.URL, ID: "id1", Secret: "sec"}, fs, slog.New(slog.DiscardHandler), "test")
	if err != nil {
		t.Fatal(err)
	}
	c.SetNetwork(collect.Network{Country: "JP"})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := c.Run(ctx); !errors.Is(err, ErrStopped) {
		t.Fatalf("期望 ErrStopped，实际 %v", err)
	}
	if a := <-gotAuth; a != "Bearer id1:sec" {
		t.Errorf("鉴权头错误: %q", a)
	}
	if h := <-gotHello; h.AgentVersion != "test" || h.Country != "JP" {
		t.Errorf("hello 内容错误: %+v", h)
	}
	if rep := <-gotReport; rep.CPU != 5 || rep.Disks == nil || rep.FilterID != "f-123" {
		t.Errorf("report 内容错误: %+v", rep)
	}
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if len(fs.filter.NICInclude) != 1 || fs.filter.NICInclude[0] != "eth" {
		t.Errorf("未应用下发的过滤规则: %+v", fs.filter)
	}
}

// TestSetNetworkResendsHello 验证 SetNetwork 在取值变化时让当前连接重新发送 hello，值不变时不重发
func TestSetNetworkResendsHello(t *testing.T) {
	helloCh := make(chan proto.Hello, 4)
	reportCh := make(chan proto.Report, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		ctx := r.Context()
		for {
			_, b, err := conn.Read(ctx)
			if err != nil {
				return
			}
			env, err := proto.Decode(b)
			if err != nil {
				continue
			}
			switch env.Type {
			case proto.TypeHello:
				var h proto.Hello
				_ = json.Unmarshal(env.Data, &h)
				helloCh <- h
			case proto.TypeReport:
				var rep proto.Report
				_ = json.Unmarshal(env.Data, &rep)
				reportCh <- rep
			}
		}
	}))
	defer srv.Close()

	fs := &fakeSampler{}
	c, err := NewClient(Config{Dashboard: srv.URL, ID: "id1", Secret: "sec"}, fs, slog.New(slog.DiscardHandler), "test")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go func() { _ = c.Run(ctx) }()

	select {
	case h := <-helloCh:
		if h.IPv4 != "" {
			t.Errorf("首个 hello 不应带 IPv4: %+v", h)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("未收到首个 hello")
	}

	c.SetNetwork(collect.Network{Country: "JP", IPv4: "1.2.3.4"})
	select {
	case h := <-helloCh:
		if h.IPv4 != "1.2.3.4" {
			t.Fatalf("重发的 hello 应带上新的 IPv4: %+v", h)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("未在 2 秒内收到重发的 hello")
	}

	c.SetNetwork(collect.Network{Country: "JP", IPv4: "1.2.3.4"})
	select {
	case h := <-helloCh:
		t.Fatalf("取值未变化时不应重发 hello: %+v", h)
	case <-reportCh:
		// 期间收到 report 属于预期行为
	case <-time.After(300 * time.Millisecond):
	}
}

// TestSetNetworkBeforeRunIncludedInInitialHello 验证在 Run 之前调用 SetNetwork 时，
// 该值会体现在连接建立后发送的第一条 hello 里，而不会额外触发第二条 hello
func TestSetNetworkBeforeRunIncludedInInitialHello(t *testing.T) {
	helloCh := make(chan proto.Hello, 4)
	reportCh := make(chan proto.Report, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		ctx := r.Context()
		for {
			_, b, err := conn.Read(ctx)
			if err != nil {
				return
			}
			env, err := proto.Decode(b)
			if err != nil {
				continue
			}
			switch env.Type {
			case proto.TypeHello:
				var h proto.Hello
				_ = json.Unmarshal(env.Data, &h)
				helloCh <- h
			case proto.TypeReport:
				var rep proto.Report
				_ = json.Unmarshal(env.Data, &rep)
				reportCh <- rep
			}
		}
	}))
	defer srv.Close()

	fs := &fakeSampler{}
	c, err := NewClient(Config{Dashboard: srv.URL, ID: "id1", Secret: "sec"}, fs, slog.New(slog.DiscardHandler), "test")
	if err != nil {
		t.Fatal(err)
	}
	c.SetNetwork(collect.Network{Country: "JP", IPv4: "1.2.3.4"})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go func() { _ = c.Run(ctx) }()

	select {
	case h := <-helloCh:
		if h.IPv4 != "1.2.3.4" {
			t.Fatalf("Run 前设置的网络信息应体现在首个 hello 中: %+v", h)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("未收到首个 hello")
	}

	select {
	case h := <-helloCh:
		t.Fatalf("不应发送第二条 hello: %+v", h)
	case <-reportCh:
		// 期间收到 report 属于预期行为
	case <-time.After(300 * time.Millisecond):
	}
}

func TestSessionUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "no", http.StatusUnauthorized)
	}))
	defer srv.Close()
	c, err := NewClient(Config{Dashboard: srv.URL, ID: "a", Secret: "b"}, &fakeSampler{}, slog.New(slog.DiscardHandler), "t")
	if err != nil {
		t.Fatal(err)
	}
	err = c.session(context.Background())
	var ue unauthorizedError
	if !errors.As(err, &ue) {
		t.Fatalf("期望 unauthorizedError，实际 %v", err)
	}
}

func TestRunReturnsNilOnCancel(t *testing.T) {
	c, err := NewClient(Config{Dashboard: "http://127.0.0.1:1", ID: "a", Secret: "b"}, &fakeSampler{}, slog.New(slog.DiscardHandler), "t")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	if err := c.Run(ctx); err != nil {
		t.Fatalf("ctx 结束时应返回 nil，实际 %v", err)
	}
}

func TestBackoff(t *testing.T) {
	half := func() float64 { return 0.5 }
	if d := Backoff(0, false, half); d != time.Second {
		t.Errorf("attempt 0 = %v，期望 1s", d)
	}
	if d := Backoff(3, false, half); d != 8*time.Second {
		t.Errorf("attempt 3 = %v，期望 8s", d)
	}
	if d := Backoff(20, false, half); d != time.Minute {
		t.Errorf("attempt 20 = %v，期望 60s 封顶", d)
	}
	if d := Backoff(0, false, func() float64 { return 0 }); d != 800*time.Millisecond {
		t.Errorf("最小抖动 = %v，期望 800ms", d)
	}
	if d := Backoff(0, true, half); d != 5*time.Minute {
		t.Errorf("鉴权失败 = %v，期望 5m", d)
	}
}
