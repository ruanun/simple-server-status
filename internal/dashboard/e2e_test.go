package dashboard_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"testing/fstest"
	"time"

	"github.com/coder/websocket"
	"github.com/ruanun/simple-server-status/internal/agent"
	"github.com/ruanun/simple-server-status/internal/agent/collect"
	"github.com/ruanun/simple-server-status/internal/dashboard"
)

func call[T any](t *testing.T, method, url, token string, body any) T {
	t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, url, r)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var w struct {
		Data T `json:"data"`
	}
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s %s 返回 %d: %s", method, url, resp.StatusCode, b)
	}
	if err := json.Unmarshal(b, &w); err != nil {
		t.Fatalf("解析响应失败: %v %s", err, b)
	}
	return w.Data
}

func waitUntil(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("等待条件超时")
}

func TestEndToEnd(t *testing.T) {
	dataDir := t.TempDir()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	base := "http://" + ln.Addr().String()
	log := slog.New(slog.DiscardHandler)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- dashboard.Run(ctx, dashboard.Options{
			Listener: ln, DataDir: dataDir, AdminPassword: "password123", Log: log, Version: "e2e",
			WebFS: fstest.MapFS{"index.html": {Data: []byte("<html></html>")}},
		})
	}()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("Run 返回错误: %v", err)
		}
	})

	waitUntil(t, 5*time.Second, func() bool {
		resp, err := http.Get(base + "/api/public/site")
		if err != nil {
			return false
		}
		resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	})

	tok := call[struct {
		Token string `json:"token"`
	}](t, "POST", base+"/api/auth/login", "", map[string]string{"username": "admin", "password": "password123"}).Token
	srv := call[struct {
		ID     string `json:"id"`
		Secret string `json:"secret"`
	}](t, "POST", base+"/api/admin/servers", tok, map[string]any{"name": "e2e"})

	agentCtx, agentCancel := context.WithCancel(ctx)
	defer agentCancel()
	client, err := agent.NewClient(agent.Config{Dashboard: base, ID: srv.ID, Secret: srv.Secret}, collect.New(log), log, "e2e")
	if err != nil {
		t.Fatal(err)
	}
	agentDone := make(chan error, 1)
	go func() { agentDone <- client.Run(agentCtx) }()

	waitUntil(t, 15*time.Second, func() bool {
		list := call[[]map[string]any](t, "GET", base+"/api/public/servers", "", nil)
		return len(list) == 1 && list[0]["online"] == true && list[0]["metrics"] != nil && list[0]["static"] != nil
	})
	pts := call[[]map[string]any](t, "GET", base+"/api/public/servers/"+srv.ID+"/metrics?range=realtime", "", nil)
	if len(pts) == 0 {
		t.Fatal("实时历史应至少有 1 个点")
	}

	wsCtx, wsCancel := context.WithTimeout(ctx, 5*time.Second)
	defer wsCancel()
	ws, _, err := websocket.Dial(wsCtx, "ws://"+ln.Addr().String()+"/api/public/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, b, err := ws.Read(wsCtx)
	if err != nil || !bytes.Contains(b, []byte(`"type":"snapshot"`)) {
		t.Fatalf("浏览器首条消息应为快照: %s %v", b, err)
	}
	ws.CloseNow()

	call[map[string]any](t, "DELETE", base+"/api/admin/servers/"+srv.ID, tok, nil)
	select {
	case err := <-agentDone:
		if !errors.Is(err, agent.ErrStopped) {
			t.Fatalf("删除服务器后 Agent 应收到停止指令，实际 %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Agent 未在 10 秒内退出")
	}
}
