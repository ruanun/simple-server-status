package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ruanun/simple-server-status/internal/dashboard/auth"
	"github.com/ruanun/simple-server-status/internal/dashboard/history"
	"github.com/ruanun/simple-server-status/internal/dashboard/hub"
	"github.com/ruanun/simple-server-status/internal/dashboard/incident"
	"github.com/ruanun/simple-server-status/internal/dashboard/store"
	"github.com/ruanun/simple-server-status/internal/dashboard/traffic"
)

// addEvent 直接写入一条事件
func (e *testEnv) addEvent(t *testing.T, ev store.Event) store.Event {
	t.Helper()
	if err := e.st.CreateEvent(context.Background(), &ev); err != nil {
		t.Fatal(err)
	}
	return ev
}

func TestEventEndpoints(t *testing.T) {
	e := newTestEnv(t)
	s := e.addServer(store.Server{Name: "hk"})
	hidden := e.addServer(store.Server{Name: "h", Hidden: true})
	now := e.clock.Now().Unix()
	ended := now - 300
	e.addEvent(t, store.Event{ServerID: s.ID, Kind: incident.KindOffline, StartAt: now - 600, EndAt: &ended})
	e.addEvent(t, store.Event{ServerID: s.ID, Kind: incident.KindOffline, StartAt: now - 60})
	e.addEvent(t, store.Event{ServerID: hidden.ID, Kind: incident.KindOffline, StartAt: now - 60})
	at := now - 30
	e.addEvent(t, store.Event{ServerID: s.ID, Kind: incident.KindReboot, StartAt: at, EndAt: &at})
	e.addEvent(t, store.Event{ServerID: s.ID, Kind: incident.KindLoadCPU, StartAt: now - 120, Detail: json.RawMessage(`{"peak":95}`)})

	code, body := e.do("GET", "/api/public/servers/"+s.ID+"/outages", "", nil)
	pub := decodeData[[]struct {
		StartAt  int64  `json:"start_at"`
		EndAt    *int64 `json:"end_at"`
		Duration int64  `json:"duration"`
	}](t, body)
	if code != http.StatusOK || len(pub) != 2 || pub[0].EndAt != nil || pub[0].Duration != 60 || pub[1].Duration != 300 {
		t.Fatalf("公开接口只返回离线记录 %d %+v", code, pub)
	}
	if strings.Contains(string(body), "server_id") {
		t.Fatalf("公开接口不应含服务器 ID 等字段: %s", body)
	}
	if code, _ := e.do("GET", "/api/public/servers/"+hidden.ID+"/outages", "", nil); code != http.StatusNotFound {
		t.Fatalf("隐藏服务器未登录应 404，实际 %d", code)
	}

	type item struct {
		ServerName string          `json:"server_name"`
		Kind       string          `json:"kind"`
		Duration   int64           `json:"duration"`
		Detail     json.RawMessage `json:"detail"`
	}
	type page struct {
		Items []item `json:"items"`
		Total int    `json:"total"`
	}
	tok := e.adminToken()
	code, body = e.do("GET", "/api/admin/events?server_id="+s.ID, tok, nil)
	all := decodeData[page](t, body)
	if code != http.StatusOK || all.Total != 4 || all.Items[0].Kind != incident.KindReboot || all.Items[0].Duration != 0 ||
		string(all.Items[0].Detail) != `{}` || all.Items[0].ServerName != "hk" {
		t.Fatalf("后台事件错误 %d %+v", code, all)
	}
	code, body = e.do("GET", "/api/admin/events?server_id="+s.ID+"&kind=offline,load_cpu&size=1&page=3", tok, nil)
	adm := decodeData[page](t, body)
	if code != http.StatusOK || adm.Total != 3 || len(adm.Items) != 1 || adm.Items[0].Kind != incident.KindOffline || adm.Items[0].Duration != 300 {
		t.Fatalf("按类型筛选与分页错误 %d %+v", code, adm)
	}
	for _, q := range []string{"page=0", "size=abc", "page=-1", "kind=bad", "kind=offline,bad"} {
		if code, b := e.do("GET", "/api/admin/events?"+q, tok, nil); code != http.StatusBadRequest || errorCode(t, b) != "invalid_input" {
			t.Errorf("%s 应返回 invalid_input，实际 %d", q, code)
		}
	}
	if code, _ := e.do("GET", "/api/admin/events", "", nil); code != http.StatusUnauthorized {
		t.Fatalf("未登录应 401，实际 %d", code)
	}
}

func TestNotifyLogEndpoint(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	for i, st := range []string{store.LogSent, store.LogFailed, store.LogPending} {
		if _, err := e.st.AddNotifyLog(ctx, store.NotifyLog{ServerID: "a", ServerName: "hk", Kind: "offline", Channel: "webhook", Status: st, CreatedAt: int64(100 + i)}); err != nil {
			t.Fatal(err)
		}
	}
	tok := e.adminToken()
	code, body := e.do("GET", "/api/admin/notify-log?status=failed", tok, nil)
	got := decodeData[struct {
		Items []store.NotifyLog `json:"items"`
		Total int               `json:"total"`
	}](t, body)
	if code != http.StatusOK || got.Total != 1 || got.Items[0].Status != store.LogFailed {
		t.Fatalf("%d %+v", code, got)
	}
	if code, b := e.do("GET", "/api/admin/notify-log?status=bad", tok, nil); code != http.StatusBadRequest || errorCode(t, b) != "invalid_input" {
		t.Fatalf("非法状态应返回 invalid_input，实际 %d", code)
	}
	code, body = e.do("GET", "/api/admin/notify-log?size=500", tok, nil)
	if all := decodeData[struct {
		Items []store.NotifyLog `json:"items"`
	}](t, body); code != http.StatusOK || len(all.Items) != 3 {
		t.Fatalf("size 超限应按 100 处理 %d", code)
	}
}

func TestCleanupEvents(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	old := e.clock.Now().Add(-91 * 24 * time.Hour).Unix()
	end := old + 60
	e.addEvent(t, store.Event{ServerID: "a", Kind: incident.KindOffline, StartAt: old, EndAt: &end})
	_, _ = e.st.AddNotifyLog(ctx, store.NotifyLog{Kind: "test", Channel: "webhook", Status: store.LogSent, CreatedAt: old})
	e.api.cleanupEvents(ctx)
	if _, n, _ := e.st.ListEvents(ctx, store.EventFilter{}, 10, 0); n != 0 {
		t.Fatal("90 天前的事件应清理")
	}
	if _, n, _ := e.st.ListNotifyLog(ctx, "", "", 10, 0); n != 0 {
		t.Fatal("90 天前的通知记录应清理")
	}
}

func TestCleanupEventsRemovesOrphans(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	sv := e.addServer(store.Server{Name: "hk"})
	now := e.clock.Now().Unix()
	for _, id := range []string{sv.ID, "gone"} {
		e.addEvent(t, store.Event{ServerID: id, Kind: incident.KindOffline, StartAt: now})
		_, _ = e.st.AddNotifyLog(ctx, store.NotifyLog{ServerID: id, Kind: "offline", Channel: "webhook", Status: store.LogSent, CreatedAt: now})
	}
	_, _ = e.st.AddNotifyLog(ctx, store.NotifyLog{Kind: "test", Channel: "webhook", Status: store.LogSent, CreatedAt: now})
	e.api.cleanupEvents(ctx)
	if list, _, _ := e.st.ListEvents(ctx, store.EventFilter{}, 10, 0); len(list) != 1 || list[0].ServerID != sv.ID {
		t.Fatalf("已删除服务器的事件应清理，现存服务器的保留 %+v", list)
	}
	if _, n, _ := e.st.ListNotifyLog(ctx, "gone", "", 10, 0); n != 0 {
		t.Fatal("已删除服务器的通知记录应清理")
	}
	if _, n, _ := e.st.ListNotifyLog(ctx, "", "", 10, 0); n != 2 {
		t.Fatalf("现存服务器与测试消息的通知记录应保留，实际 %d 条", n)
	}
}

func TestNewFailsLeftoverPendingLog(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "p.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	id, _ := st.AddNotifyLog(ctx, store.NotifyLog{ServerID: "a", Kind: "offline", Channel: "webhook", Status: store.LogPending, CreatedAt: 1})
	clock := &fakeClock{t: time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)}
	log := slog.New(slog.DiscardHandler)
	a, err := New(ctx, Deps{
		Store: st, Hub: hub.New(clock.Now), History: history.NewRecorder(st, clock.Now, log), Traffic: traffic.New(st, clock.Now),
		Auth: auth.NewManager([]byte("s"), clock.Now), Limiter: auth.NewLimiter(clock.Now), Log: log, Now: clock.Now,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Shutdown)
	list, _, _ := st.ListNotifyLog(ctx, "", "", 10, 0)
	if len(list) != 1 || list[0].ID != id || list[0].Status != store.LogFailed || list[0].Error != "退出时未能发送" ||
		list[0].DoneAt == nil || *list[0].DoneAt != clock.Now().Unix() {
		t.Fatalf("启动时遗留的发送中记录应标记为失败 %+v", list)
	}
}

func TestRefreshUptimeKeepsLastValueOnError(t *testing.T) {
	e := newTestEnv(t)
	ctx := context.Background()
	// 注：不用 e.addServer（其底层 store.CreateServer 用真实系统时间盖 CreatedAt），
	// 因为沙箱真实时间晚于 newTestEnv 里固定的伪造时钟（2026-03-15），会导致
	// CreatedAt 晚于伪造的“当前时间”，history.Uptime 因窗口内采样单位数 < 1 直接
	// 短路返回 (nil, nil)，根本不会执行查询，也就无法验证“查询出错时保留旧值”。
	// 改用 UpsertServers 显式把 CreatedAt 设在伪造时钟之前（与 public_test.go 中
	// TestPublicServersIncludeUptime 的做法一致），确保会真正执行查询。
	now := e.clock.Now()
	if err := e.st.UpsertServers(ctx, []store.Server{{ID: "u1", Name: "a", Secret: "sec", CreatedAt: now.Add(-48 * time.Hour).Unix()}}); err != nil {
		t.Fatal(err)
	}
	if err := e.api.reload(ctx); err != nil {
		t.Fatal(err)
	}
	v := 99.5
	e.api.upMu.Lock()
	e.api.uptimeCache = map[string]*float64{"u1": &v}
	e.api.upMu.Unlock()
	_ = e.st.Close() // 让查询出错
	e.api.refreshUptime(ctx)
	if got := e.api.cachedUptime("u1"); got == nil || *got != 99.5 {
		t.Fatalf("出错时应保留上次的值，得到 %v", got)
	}
}
