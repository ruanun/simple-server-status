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
	c.SetCountry("JP")
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
