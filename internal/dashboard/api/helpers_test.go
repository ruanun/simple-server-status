package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/coder/websocket"
	"github.com/ruanun/simple-server-status/internal/dashboard/auth"
	"github.com/ruanun/simple-server-status/internal/dashboard/history"
	"github.com/ruanun/simple-server-status/internal/dashboard/hub"
	"github.com/ruanun/simple-server-status/internal/dashboard/store"
	"github.com/ruanun/simple-server-status/internal/dashboard/traffic"
	"github.com/ruanun/simple-server-status/internal/proto"
)

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Add(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

type testEnv struct {
	t     *testing.T
	api   *API
	srv   *httptest.Server
	st    *store.Store
	clock *fakeClock
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	clock := &fakeClock{t: time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)}
	log := slog.New(slog.DiscardHandler)
	a, err := New(context.Background(), Deps{
		Store:   st,
		Hub:     hub.New(clock.Now),
		History: history.NewRecorder(st, clock.Now, log),
		Traffic: traffic.New(st, clock.Now),
		Auth:    auth.NewManager([]byte("test-secret"), clock.Now),
		Limiter: auth.NewLimiter(clock.Now),
		Log:     log,
		Now:     clock.Now,
	})
	if err != nil {
		t.Fatal(err)
	}
	eng, err := a.Router(fstest.MapFS{"index.html": {Data: []byte("<html>app</html>")}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(eng)
	t.Cleanup(func() {
		a.Shutdown()
		srv.Close()
	})
	return &testEnv{t: t, api: a, srv: srv, st: st, clock: clock}
}

// addServer 直接在库中创建服务器并刷新缓存
func (e *testEnv) addServer(s store.Server) store.Server {
	e.t.Helper()
	if err := e.st.CreateServer(context.Background(), &s); err != nil {
		e.t.Fatal(err)
	}
	if err := e.api.reload(context.Background()); err != nil {
		e.t.Fatal(err)
	}
	return s
}

// adminToken 确保存在 admin/password123 并返回其 token
func (e *testEnv) adminToken() string {
	e.t.Helper()
	ctx := context.Background()
	u, err := e.st.GetUserByName(ctx, "admin")
	if errors.Is(err, store.ErrNotFound) {
		hash, _ := auth.HashPassword("password123")
		u = store.User{Username: "admin", PasswordHash: hash}
		if err := e.st.CreateUser(ctx, &u); err != nil {
			e.t.Fatal(err)
		}
	} else if err != nil {
		e.t.Fatal(err)
	}
	tok, err := e.api.Auth.Issue(u.ID, u.TokenVersion)
	if err != nil {
		e.t.Fatal(err)
	}
	return tok
}

// do 发送 JSON 请求，返回状态码与响应体
func (e *testEnv) do(method, path, token string, body any) (int, []byte) {
	e.t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, e.srv.URL+path, r)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

// decodeData 解析 {"data": ...} 响应
func decodeData[T any](t *testing.T, b []byte) T {
	t.Helper()
	var w struct {
		Data T `json:"data"`
	}
	if err := json.Unmarshal(b, &w); err != nil {
		t.Fatalf("解析响应失败: %v, body=%s", err, b)
	}
	return w.Data
}

// errorCode 解析 {"error":{"code":...}} 响应
func errorCode(t *testing.T, b []byte) string {
	t.Helper()
	var w struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(b, &w); err != nil {
		t.Fatalf("解析错误响应失败: %v, body=%s", err, b)
	}
	return w.Error.Code
}

func (e *testEnv) wsURL(path string) string {
	return "ws" + strings.TrimPrefix(e.srv.URL, "http") + path
}

// dialAgent 以 Agent 身份连接
func (e *testEnv) dialAgent(id, secret string) (*websocket.Conn, *http.Response, error) {
	h := http.Header{}
	h.Set("Authorization", "Bearer "+id+":"+secret)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return websocket.Dial(ctx, e.wsURL("/api/agent/ws"), &websocket.DialOptions{HTTPHeader: h})
}

func sendMsg(t *testing.T, conn *websocket.Conn, typ string, data any) {
	t.Helper()
	b, err := proto.Encode(typ, data)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := conn.Write(ctx, websocket.MessageText, b); err != nil {
		t.Fatal(err)
	}
}

func readMsg(t *testing.T, conn *websocket.Conn) proto.Envelope {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, b, err := conn.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	env, err := proto.Decode(b)
	if err != nil {
		t.Fatal(err)
	}
	return env
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("等待条件超时")
}
