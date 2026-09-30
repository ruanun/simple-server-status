package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/ruanun/simple-server-status/internal/dashboard/metric"
	"github.com/ruanun/simple-server-status/internal/dashboard/store"
	"github.com/ruanun/simple-server-status/internal/proto"
)

func TestPublicListHidesHiddenAndSecrets(t *testing.T) {
	e := newTestEnv(t)
	price := 9.9
	e.addServer(store.Server{Name: "pub", Price: &price, Currency: "USD"})
	e.addServer(store.Server{Name: "hid", Hidden: true})

	code, body := e.do("GET", "/api/public/servers", "", nil)
	if code != http.StatusOK {
		t.Fatalf("状态码 %d", code)
	}
	list := decodeData[[]ServerView](t, body)
	if len(list) != 1 || list[0].Name != "pub" || list[0].Price != nil {
		t.Fatalf("公开列表错误: %+v", list)
	}
	for _, bad := range []string{`"secret"`, `"last_ip"`, `"hid"`, `"price"`} {
		if bytes.Contains(body, []byte(bad)) {
			t.Errorf("公开响应不应包含 %s: %s", bad, body)
		}
	}

	_, body = e.do("GET", "/api/public/servers", e.adminToken(), nil)
	list = decodeData[[]ServerView](t, body)
	if len(list) != 2 {
		t.Fatalf("登录后应看到隐藏服务器: %+v", list)
	}
}

func TestPublicShowPriceSetting(t *testing.T) {
	e := newTestEnv(t)
	price := 9.9
	e.addServer(store.Server{Name: "pub", Price: &price, Currency: "USD"})
	st := store.DefaultSettings()
	st.ShowPrice = true
	if err := e.st.SaveSettings(context.Background(), st); err != nil {
		t.Fatal(err)
	}
	_ = e.api.reload(context.Background())
	_, body := e.do("GET", "/api/public/servers", "", nil)
	list := decodeData[[]ServerView](t, body)
	if list[0].Price == nil || *list[0].Price != 9.9 || list[0].Currency != "USD" {
		t.Fatalf("开启公开价格后应返回价格: %+v", list[0])
	}
}

func TestPublicHiddenServerNotReachable(t *testing.T) {
	e := newTestEnv(t)
	h := e.addServer(store.Server{Name: "hid", Hidden: true})
	for _, path := range []string{"/api/public/servers/" + h.ID, "/api/public/servers/" + h.ID + "/metrics?range=1h"} {
		if code, body := e.do("GET", path, "", nil); code != http.StatusNotFound || errorCode(t, body) != "not_found" {
			t.Errorf("%s 未登录应返回 404，实际 %d %s", path, code, body)
		}
		if code, _ := e.do("GET", path, e.adminToken(), nil); code != http.StatusOK {
			t.Errorf("%s 登录后应返回 200，实际 %d", path, code)
		}
	}
}

func TestPublicDetailNeverReported(t *testing.T) {
	e := newTestEnv(t)
	s := e.addServer(store.Server{Name: "a"})
	code, body := e.do("GET", "/api/public/servers/"+s.ID, "", nil)
	if code != http.StatusOK {
		t.Fatalf("状态码 %d", code)
	}
	for _, want := range []string{`"metrics":null`, `"static":null`, `"online":false`, `"limit":null`} {
		if !bytes.Contains(body, []byte(want)) {
			t.Errorf("响应应包含 %s: %s", want, body)
		}
	}
}

func TestPublicMetrics(t *testing.T) {
	e := newTestEnv(t)
	s := e.addServer(store.Server{Name: "a"})
	ts := e.clock.Now().Unix() - 60
	_ = e.st.InsertMetrics(context.Background(), store.Metrics1m, []store.MetricRow{{ServerID: s.ID, Point: metric.Point{TS: ts, CPU: 12}}})
	code, body := e.do("GET", "/api/public/servers/"+s.ID+"/metrics?range=1h", "", nil)
	pts := decodeData[[]metric.Point](t, body)
	if code != http.StatusOK || len(pts) != 1 || pts[0].CPU != 12 {
		t.Fatalf("历史数据错误: %d %s", code, body)
	}
	code, body = e.do("GET", "/api/public/servers/"+s.ID+"/metrics?range=bad", "", nil)
	if code != http.StatusBadRequest || errorCode(t, body) != "bad_range" {
		t.Fatalf("非法范围应返回 400 bad_range，实际 %d %s", code, body)
	}
}

func TestPublicSite(t *testing.T) {
	e := newTestEnv(t)
	_, body := e.do("GET", "/api/public/site", "", nil)
	site := decodeData[map[string]any](t, body)
	if site["site_title"] != "Simple Server Status" || site["show_price"] != false {
		t.Fatalf("站点信息错误: %v", site)
	}
}

type wsEnvelope struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

func readWS(t *testing.T, conn *websocket.Conn) (string, []ServerView) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, b, err := conn.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var env wsEnvelope
	if err := json.Unmarshal(b, &env); err != nil {
		t.Fatal(err)
	}
	var list []ServerView
	if err := json.Unmarshal(env.Data, &list); err != nil {
		t.Fatal(err)
	}
	return env.Type, list
}

func TestPublicWSSnapshotAndDelta(t *testing.T) {
	e := newTestEnv(t)
	s := e.addServer(store.Server{Name: "pub"})
	h := e.addServer(store.Server{Name: "hid", Hidden: true})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, e.wsURL("/api/public/ws"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()

	typ, list := readWS(t, conn)
	if typ != "snapshot" || len(list) != 1 || list[0].ID != s.ID {
		t.Fatalf("首条应为只含公开服务器的快照: %s %+v", typ, list)
	}

	e.api.Hub.Connect(s.ID, 2)
	e.api.handleReport(ctx, s.ID, proto.Report{CPU: 77})
	e.api.Hub.Connect(h.ID, 2)
	e.api.handleReport(ctx, h.ID, proto.Report{CPU: 1})
	e.api.bc.tick(ctx)

	typ, list = readWS(t, conn)
	if typ != "delta" || len(list) != 1 || list[0].ID != s.ID || list[0].Metrics == nil || list[0].Metrics.CPU != 77 || !list[0].Online {
		t.Fatalf("delta 错误: %s %+v", typ, list)
	}

	e.api.bc.requestSnapshot()
	e.api.bc.tick(ctx)
	if typ, _ := readWS(t, conn); typ != "snapshot" {
		t.Fatalf("请求快照后应推送 snapshot，实际 %s", typ)
	}
}

func TestPublicWSWithTokenIncludesHidden(t *testing.T) {
	e := newTestEnv(t)
	e.addServer(store.Server{Name: "hid", Hidden: true})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, e.wsURL("/api/public/ws?token="+e.adminToken()), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	if _, list := readWS(t, conn); len(list) != 1 {
		t.Fatalf("登录后快照应包含隐藏服务器: %+v", list)
	}
}

func TestPublicWSAcceptsCrossOrigin(t *testing.T) {
	e := newTestEnv(t)
	e.addServer(store.Server{Name: "pub"})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	h := http.Header{}
	h.Set("Origin", "http://other.example")
	conn, _, err := websocket.Dial(ctx, e.wsURL("/api/public/ws"), &websocket.DialOptions{HTTPHeader: h})
	if err != nil {
		t.Fatalf("反向代理等跨源场景应允许连接: %v", err)
	}
	defer conn.CloseNow()
	if typ, list := readWS(t, conn); typ != "snapshot" || len(list) != 1 {
		t.Fatalf("应收到快照: %s %+v", typ, list)
	}
}

func dialPublicWS(t *testing.T, e *testEnv, token string) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	path := "/api/public/ws"
	if token != "" {
		path += "?token=" + token
	}
	conn, _, err := websocket.Dial(ctx, e.wsURL(path), nil)
	if err != nil {
		t.Fatal(err)
	}
	return conn
}

func expectClosed(t *testing.T, conn *websocket.Conn, msg string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		if _, _, err := conn.Read(ctx); err != nil {
			if ctx.Err() != nil {
				t.Fatal(msg)
			}
			return
		}
	}
}

func TestChangePasswordClosesAuthedWS(t *testing.T) {
	e := newTestEnv(t)
	e.addServer(store.Server{Name: "hid", Hidden: true})
	tok := e.adminToken()
	authed := dialPublicWS(t, e, tok)
	defer authed.CloseNow()
	anon := dialPublicWS(t, e, "")
	defer anon.CloseNow()
	readWS(t, authed)
	readWS(t, anon)

	if code, body := e.do("PUT", "/api/auth/password", tok, map[string]string{"old_password": "password123", "new_password": "newpass123"}); code != http.StatusOK {
		t.Fatalf("修改密码失败: %d %s", code, body)
	}
	expectClosed(t, authed, "改密码后已登录的浏览器连接应被关闭")

	e.api.bc.requestSnapshot()
	e.api.bc.tick(context.Background())
	if typ, list := readWS(t, anon); typ != "snapshot" || len(list) != 0 {
		t.Fatalf("未登录连接应不受影响: %s %+v", typ, list)
	}
}

func TestVerifyAuthedClosesRevokedOrExpired(t *testing.T) {
	e := newTestEnv(t)
	tok := e.adminToken()
	revoked := dialPublicWS(t, e, tok)
	defer revoked.CloseNow()
	readWS(t, revoked)
	// 模拟 reset-password：另一进程直接修改数据库使 token 版本 +1
	u, _ := e.st.GetUserByName(context.Background(), "admin")
	if err := e.st.SetPassword(context.Background(), u.ID, u.PasswordHash); err != nil {
		t.Fatal(err)
	}
	e.api.bc.verifyAuthed(context.Background())
	expectClosed(t, revoked, "token 版本变化后连接应被关闭")

	fresh, _ := e.api.Auth.Issue(u.ID, u.TokenVersion+1)
	expiring := dialPublicWS(t, e, fresh)
	defer expiring.CloseNow()
	readWS(t, expiring)
	e.api.bc.verifyAuthed(context.Background())
	e.api.bc.requestSnapshot()
	e.api.bc.tick(context.Background())
	if typ, _ := readWS(t, expiring); typ != "snapshot" {
		t.Fatalf("有效 token 的连接应保持，收到 %s", typ)
	}
	e.clock.Add(8 * 24 * time.Hour)
	e.api.bc.verifyAuthed(context.Background())
	expectClosed(t, expiring, "token 过期后连接应被关闭")
}

func TestPublicViewHidesFilterID(t *testing.T) {
	e := newTestEnv(t)
	s := e.addServer(store.Server{Name: "a"})
	e.api.Hub.Connect(s.ID, 2)
	e.api.handleReport(context.Background(), s.ID, proto.Report{CPU: 1, FilterID: "abc123"})
	_, body := e.do("GET", "/api/public/servers", "", nil)
	if bytes.Contains(body, []byte("filter_id")) {
		t.Fatalf("公开响应不应包含 filter_id: %s", body)
	}
	if !bytes.Contains(body, []byte(`"cpu":1`)) {
		t.Fatalf("公开响应应包含指标: %s", body)
	}
}

func TestPublicViewHidesAgentVersion(t *testing.T) {
	e := newTestEnv(t)
	live := e.addServer(store.Server{Name: "live"})
	e.api.Hub.SetStatic(live.ID, proto.Hello{OS: "linux", AgentVersion: "2.0.0-beta.3"})
	stored := e.addServer(store.Server{Name: "stored"})
	if err := e.st.SetStaticInfo(context.Background(), stored.ID, "1.2.3.4", proto.Hello{OS: "linux", AgentVersion: "2.0.0-beta.1"}); err != nil {
		t.Fatal(err)
	}
	if err := e.api.reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	tok := e.adminToken()
	for _, req := range []struct{ path, token string }{
		{"/api/public/servers", ""},
		{"/api/public/servers", tok},
		{"/api/public/servers/" + live.ID, ""},
		{"/api/public/servers/" + stored.ID, tok},
	} {
		code, body := e.do("GET", req.path, req.token, nil)
		if code != http.StatusOK || bytes.Contains(body, []byte("2.0.0-beta")) || !bytes.Contains(body, []byte(`"os":"linux"`)) {
			t.Fatalf("%s 应保留静态信息但不含 Agent 版本: %d %s", req.path, code, body)
		}
	}
	_, body := e.do("GET", "/api/admin/servers", tok, nil)
	if !bytes.Contains(body, []byte(`"agent_version":"2.0.0-beta.3"`)) {
		t.Fatalf("后台列表仍应显示 Agent 版本: %s", body)
	}
}

func TestPublicViewIPFlags(t *testing.T) {
	e := newTestEnv(t)
	s := e.addServer(store.Server{Name: "v4only"})
	// static_info 来自数据库；用 SetStaticInfo + reload 保证后台列表能读到地址（Hub.SetStatic 只更新内存缓存）
	if err := e.st.SetStaticInfo(context.Background(), s.ID, "", proto.Hello{OS: "linux", IPv4: "203.0.113.7"}); err != nil {
		t.Fatal(err)
	}
	if err := e.api.reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	old := e.addServer(store.Server{Name: "old"})
	e.api.Hub.SetStatic(old.ID, proto.Hello{OS: "linux"})
	code, body := e.do("GET", "/api/public/servers", "", nil)
	if code != http.StatusOK || bytes.Contains(body, []byte("203.0.113.7")) {
		t.Fatalf("公开接口不应包含 IP 地址: %s", body)
	}
	got := map[string][2]bool{}
	for _, v := range decodeData[[]struct {
		ID   string `json:"id"`
		IPv4 bool   `json:"ipv4"`
		IPv6 bool   `json:"ipv6"`
	}](t, body) {
		got[v.ID] = [2]bool{v.IPv4, v.IPv6}
	}
	if got[s.ID] != [2]bool{true, false} || got[old.ID] != [2]bool{false, false} {
		t.Fatalf("IP 标记错误 %+v", got)
	}
	_, body = e.do("GET", "/api/admin/servers", e.adminToken(), nil)
	if !bytes.Contains(body, []byte(`"ipv4":"203.0.113.7"`)) {
		t.Fatalf("后台应包含完整地址: %s", body)
	}
}

func TestPublicStats(t *testing.T) {
	e := newTestEnv(t) // 时钟为 2026-03-15 12:00 UTC
	s := e.addServer(store.Server{Name: "a", TrafficResetDay: 1})
	hidden := e.addServer(store.Server{Name: "h", Hidden: true})
	ctx := context.Background()
	for _, v := range []uint64{100, 400} { // 3/15：+300
		if err := e.api.Traffic.Add(ctx, s.ID, 1, "", v, v); err != nil {
			t.Fatal(err)
		}
	}
	code, body := e.do("GET", "/api/public/servers/"+s.ID+"/stats", "", nil)
	if code != http.StatusOK {
		t.Fatalf("%d %s", code, body)
	}
	got := decodeData[struct {
		Uptime24h   *float64             `json:"uptime_24h"`
		Uptime7d    *float64             `json:"uptime_7d"`
		PeriodStart string               `json:"period_start"`
		PeriodEnd   string               `json:"period_end"`
		Daily       []store.DailyTraffic `json:"daily"`
	}](t, body)
	if got.PeriodStart != "2026-03-01" || got.PeriodEnd != "2026-03-31" || len(got.Daily) != 15 {
		t.Fatalf("周期或天数错误 %+v", got)
	}
	if last := got.Daily[14]; last.Day != "2026-03-15" || last.In != 300 || last.Out != 300 || got.Daily[0].In != 0 {
		t.Fatalf("每日流量错误 %+v", got.Daily)
	}
	if code, _ := e.do("GET", "/api/public/servers/"+hidden.ID+"/stats", "", nil); code != http.StatusNotFound {
		t.Fatalf("隐藏服务器未登录应 404，实际 %d", code)
	}
	if code, _ := e.do("GET", "/api/public/servers/"+hidden.ID+"/stats", e.adminToken(), nil); code != http.StatusOK {
		t.Fatalf("登录后应可见，实际 %d", code)
	}
}

func TestPublicServersIncludeUptime(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	now := e.clock.Now()
	if err := e.st.UpsertServers(ctx, []store.Server{{ID: "u1", Name: "u", Secret: "sec", CreatedAt: now.Add(-48 * time.Hour).Unix()}}); err != nil {
		t.Fatal(err)
	}
	if err := e.api.reload(ctx); err != nil {
		t.Fatal(err)
	}
	var rows []store.MetricRow
	start := now.Add(-24 * time.Hour).Unix()
	for i := int64(0); i < 1440; i += 2 { // 每 2 分钟一个点：在线率 50%
		rows = append(rows, store.MetricRow{ServerID: "u1", Point: metric.Point{TS: start + i*60}})
	}
	if err := e.st.InsertMetrics(ctx, store.Metrics1m, rows); err != nil {
		t.Fatal(err)
	}
	e.api.refreshUptime(ctx)
	code, body := e.do("GET", "/api/public/servers", "", nil)
	if code != http.StatusOK {
		t.Fatal(code)
	}
	list := decodeData[[]struct {
		ID        string   `json:"id"`
		Uptime24h *float64 `json:"uptime_24h"`
	}](t, body)
	if len(list) != 1 || list[0].Uptime24h == nil || *list[0].Uptime24h != 50 {
		t.Fatalf("在线率错误 %+v", list)
	}
}
