package store

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/ruanun/simple-server-status/internal/proto"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestOpenMigratesIdempotently(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var v int
	if err := s.db.QueryRow(`SELECT MAX(version) FROM schema_version`).Scan(&v); err != nil || v != 3 {
		t.Fatalf("schema_version = %d, %v", v, err)
	}
}

func TestServerNoteAndMuted(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	sv := Server{Name: "a", Note: "备注", NotifyMuted: true}
	if err := s.CreateServer(ctx, &sv); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetServer(ctx, sv.ID)
	if err != nil || got.Note != "备注" || !got.NotifyMuted {
		t.Fatalf("新建后读取错误: %+v %v", got, err)
	}
	got.Note, got.NotifyMuted = "新备注", false
	if err := s.UpdateServer(ctx, got); err != nil {
		t.Fatal(err)
	}
	if got, _ = s.GetServer(ctx, sv.ID); got.Note != "新备注" || got.NotifyMuted {
		t.Fatalf("更新后读取错误: %+v", got)
	}
	got.Note, got.NotifyMuted = "导入", true
	if err := s.UpsertServers(ctx, []Server{got}); err != nil {
		t.Fatal(err)
	}
	if got, _ = s.GetServer(ctx, sv.ID); got.Note != "导入" || !got.NotifyMuted {
		t.Fatalf("导入后读取错误: %+v", got)
	}
}

func TestDeleteServerCleansNewTables(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	sv := Server{Name: "a"}
	if err := s.CreateServer(ctx, &sv); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`INSERT INTO traffic_daily (server_id, day, in_bytes, out_bytes) VALUES (?, '2026-09-01', 1, 1)`,
		`INSERT INTO notify_state (server_id, rule, key, sent_at) VALUES (?, 'expire', '1', 1)`,
	} {
		if _, err := s.db.Exec(q, sv.ID); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.DeleteServer(ctx, sv.ID); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"traffic_daily", "notify_state"} {
		var n int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM `+table+` WHERE server_id = ?`, sv.ID).Scan(&n); err != nil || n != 0 {
			t.Fatalf("%s 未清理: %d %v", table, n, err)
		}
	}
}

func TestServerCRUD(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	list, err := s.ListServers(ctx)
	if err != nil || list == nil || len(list) != 0 {
		t.Fatalf("空列表应为 []: %v %v", list, err)
	}

	price := 9.9
	a := Server{Name: "b-server", Price: &price}
	if err := s.CreateServer(ctx, &a); err != nil {
		t.Fatal(err)
	}
	if len(a.ID) != 12 || len(a.Secret) != 32 {
		t.Fatalf("ID/Secret 长度错误: %q %q", a.ID, a.Secret)
	}
	if a.TrafficMode != "sum" || a.TrafficResetDay != 1 || a.ReportInterval != 2 {
		t.Fatalf("默认值错误: %+v", a)
	}
	b := Server{Name: "a-server", Sort: 1}
	if err := s.CreateServer(ctx, &b); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetServer(ctx, a.ID)
	if err != nil || got.Name != "b-server" || got.Price == nil || *got.Price != 9.9 || got.NICInclude == nil {
		t.Fatalf("GetServer = %+v, %v", got, err)
	}

	got.Name = "renamed"
	got.Hidden = true
	got.NICInclude = []string{"eth"}
	if err := s.UpdateServer(ctx, got); err != nil {
		t.Fatal(err)
	}
	got, _ = s.GetServer(ctx, a.ID)
	if got.Name != "renamed" || !got.Hidden || len(got.NICInclude) != 1 {
		t.Fatalf("更新未生效: %+v", got)
	}

	if err := s.SetServerOrder(ctx, []string{b.ID, a.ID}); err != nil {
		t.Fatal(err)
	}
	list, _ = s.ListServers(ctx)
	if len(list) != 2 || list[0].ID != b.ID {
		t.Fatalf("排序错误: %+v", list)
	}

	sec, err := s.ResetSecret(ctx, a.ID)
	if err != nil || sec == a.Secret || len(sec) != 32 {
		t.Fatalf("ResetSecret = %q, %v", sec, err)
	}

	if err := s.DeleteServer(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetServer(ctx, a.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("删除后应 ErrNotFound，实际 %v", err)
	}
	if err := s.DeleteServer(ctx, a.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("重复删除应 ErrNotFound，实际 %v", err)
	}
	if err := s.UpdateServer(ctx, Server{ID: "missing", Name: "x"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("更新不存在的服务器应 ErrNotFound，实际 %v", err)
	}
}

func TestStaticInfoAndLastSeen(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	a := Server{Name: "a"}
	if err := s.CreateServer(ctx, &a); err != nil {
		t.Fatal(err)
	}
	if err := s.SetStaticInfo(ctx, a.ID, "1.2.3.4", proto.Hello{OS: "linux", MemTotal: 100}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetLastSeen(ctx, a.ID, 123); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetServer(ctx, a.ID)
	if got.StaticInfo == nil || got.StaticInfo.OS != "linux" || got.LastIP != "1.2.3.4" || got.LastSeen != 123 {
		t.Fatalf("静态信息错误: %+v", got)
	}
}

func TestUpsertServers(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	a := Server{Name: "a"}
	if err := s.CreateServer(ctx, &a); err != nil {
		t.Fatal(err)
	}
	_ = s.SetStaticInfo(ctx, a.ID, "ip", proto.Hello{OS: "linux"})
	a.Name = "a2"
	a.Secret = "imported-secret"
	if err := s.UpsertServers(ctx, []Server{a, {ID: "newid000001", Name: "b", Secret: "sb"}}); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetServer(ctx, a.ID)
	if got.Name != "a2" || got.Secret != "imported-secret" || got.StaticInfo == nil {
		t.Fatalf("覆盖结果错误（静态信息应保留）: %+v", got)
	}
	nb, err := s.GetServer(ctx, "newid000001")
	if err != nil || nb.Secret != "sb" || nb.CreatedAt == 0 {
		t.Fatalf("新增结果错误: %+v %v", nb, err)
	}
}

func TestUsers(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if n, _ := s.CountUsers(ctx); n != 0 {
		t.Fatalf("初始用户数应为 0，实际 %d", n)
	}
	u := User{Username: "admin", PasswordHash: "h1"}
	if err := s.CreateUser(ctx, &u); err != nil || u.ID == 0 {
		t.Fatalf("CreateUser: %+v %v", u, err)
	}
	if err := s.SetPassword(ctx, u.ID, "h2"); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetUserByName(ctx, "admin")
	if err != nil || got.PasswordHash != "h2" || got.TokenVersion != 1 {
		t.Fatalf("GetUserByName = %+v, %v", got, err)
	}
	if _, err := s.GetUser(ctx, 999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("不存在的用户应 ErrNotFound，实际 %v", err)
	}
}

func TestSettings(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	st, err := s.GetSettings(ctx)
	if err != nil || st != DefaultSettings() {
		t.Fatalf("默认设置错误: %+v %v", st, err)
	}
	st.SiteTitle = "我的探针"
	st.ShowPrice = true
	st.DefaultReportInterval = 5
	if err := s.SaveSettings(ctx, st); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetSettings(ctx)
	if got != st {
		t.Fatalf("保存后读取不一致: %+v", got)
	}
	k1, err := s.JWTSecret(ctx)
	if err != nil || len(k1) != 32 {
		t.Fatalf("JWTSecret = %x, %v", k1, err)
	}
	k2, _ := s.JWTSecret(ctx)
	if string(k1) != string(k2) {
		t.Fatal("JWTSecret 应保持不变")
	}
}

// TestJWTSecretConcurrent 验证并发首次调用 JWTSecret 时，
// 所有调用方最终都拿到数据库中同一个密钥，不存在读-写竞态。
func TestJWTSecretConcurrent(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	const n = 8
	keys := make([][]byte, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			keys[i], errs[i] = s.JWTSecret(ctx)
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("goroutine %d JWTSecret 出错: %v", i, err)
		}
	}
	for i := 1; i < n; i++ {
		if len(keys[i]) != 32 || string(keys[i]) != string(keys[0]) {
			t.Fatalf("并发 JWTSecret 返回不一致的密钥: %x vs %x", keys[0], keys[i])
		}
	}
}

func TestSettingsNotifyRoundTrip(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	st, err := s.GetSettings(ctx)
	if err != nil || st.Notify != DefaultNotifySettings() || st.Announcement != "" {
		t.Fatalf("默认值错误 %+v %v", st, err)
	}
	st.Announcement = "维护通知"
	st.Notify.WebhookURL = "https://hook.example.com/x"
	st.Notify.TelegramToken, st.Notify.TelegramChatID = "123:abc", "42"
	st.Notify.OfflineEnabled, st.Notify.OfflineMinutes = false, 10
	st.Notify.Lang = "en-US"
	if err := s.SaveSettings(ctx, st); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetSettings(ctx)
	if err != nil || got != st {
		t.Fatalf("读回不一致\n得到 %+v\n期望 %+v", got, st)
	}
}

func TestTrafficDaily(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	rows := []DailyDelta{{ServerID: "a", Day: "2026-09-01", In: 10, Out: 1}, {ServerID: "a", Day: "2026-09-02", In: 5, Out: 5}, {ServerID: "b", Day: "2026-09-01", In: 7, Out: 7}}
	if err := s.AddTrafficDaily(ctx, rows); err != nil {
		t.Fatal(err)
	}
	if err := s.AddTrafficDaily(ctx, []DailyDelta{{ServerID: "a", Day: "2026-09-01", In: 1, Out: 2}}); err != nil {
		t.Fatal(err)
	}
	got, err := s.QueryTrafficDaily(ctx, "a", "2026-09-01", "2026-09-30")
	want := []DailyTraffic{{Day: "2026-09-01", In: 11, Out: 3}, {Day: "2026-09-02", In: 5, Out: 5}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v %v", got, err)
	}
	if n, err := s.DeleteTrafficDailyBefore(ctx, "2026-09-02"); err != nil || n != 2 {
		t.Fatalf("删除行数 %d %v", n, err)
	}
	if got, _ = s.QueryTrafficDaily(ctx, "a", "2026-01-01", "2026-12-31"); len(got) != 1 || got[0].Day != "2026-09-02" {
		t.Fatalf("清理后 %+v", got)
	}
}

func TestOutages(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	id1, err := s.CreateOutage(ctx, "a", 100)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateOutage(ctx, "b", 200); err != nil {
		t.Fatal(err)
	}
	open, _ := s.OpenOutages(ctx)
	if len(open) != 2 {
		t.Fatalf("应有 2 条进行中 %+v", open)
	}
	if err := s.CloseOutage(ctx, id1, 150); err != nil {
		t.Fatal(err)
	}
	if err := s.CloseOutage(ctx, 999, 1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("不存在应返回 ErrNotFound: %v", err)
	}
	all, total, err := s.ListOutages(ctx, "", 10, 0)
	if err != nil || total != 2 || all[0].ServerID != "b" || all[1].EndAt == nil || *all[1].EndAt != 150 {
		t.Fatalf("列表错误 %+v %d %v", all, total, err)
	}
	onlyA, total, _ := s.ListOutages(ctx, "a", 10, 0)
	if total != 1 || len(onlyA) != 1 {
		t.Fatalf("筛选错误 %+v", onlyA)
	}
	page2, _, _ := s.ListOutages(ctx, "", 1, 1)
	if len(page2) != 1 || page2[0].ServerID != "a" {
		t.Fatalf("分页错误 %+v", page2)
	}
	if n, _ := s.DeleteOutagesBefore(ctx, 1000); n != 1 {
		t.Fatalf("只应删除已结束的记录，删除 %d", n)
	}
}

func TestNotifyLog(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	id, err := s.AddNotifyLog(ctx, NotifyLog{ServerID: "a", ServerName: "hk", Kind: "offline", Channel: "webhook", Title: "t", Message: "m", Status: LogPending, CreatedAt: 100})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddNotifyLog(ctx, NotifyLog{Kind: "test", Channel: "telegram", Title: "t", Message: "m", Status: LogPending, CreatedAt: 200}); err != nil {
		t.Fatal(err)
	}
	if err := s.FinishNotifyLog(ctx, id, LogFailed, "对方返回 HTTP 500", 110); err != nil {
		t.Fatal(err)
	}
	list, total, err := s.ListNotifyLog(ctx, "", "", 10, 0)
	if err != nil || total != 2 || list[0].Kind != "test" || list[1].Status != LogFailed || list[1].DoneAt == nil || list[1].Error == "" {
		t.Fatalf("列表错误 %+v %d %v", list, total, err)
	}
	failed, total, _ := s.ListNotifyLog(ctx, "a", LogFailed, 10, 0)
	if total != 1 || len(failed) != 1 {
		t.Fatalf("筛选错误 %+v", failed)
	}
	if n, _ := s.DeleteNotifyLogBefore(ctx, 150); n != 1 {
		t.Fatalf("清理数量 %d", n)
	}
}

func TestDeleteServerCleansEvents(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	sv := Server{Name: "a"}
	if err := s.CreateServer(ctx, &sv); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateOutage(ctx, sv.ID, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddNotifyLog(ctx, NotifyLog{ServerID: sv.ID, Kind: "offline", Channel: "webhook", Status: LogSent, CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteServer(ctx, sv.ID); err != nil {
		t.Fatal(err)
	}
	if _, n, _ := s.ListOutages(ctx, sv.ID, 10, 0); n != 0 {
		t.Fatal("离线记录未清理")
	}
	if _, n, _ := s.ListNotifyLog(ctx, sv.ID, "", 10, 0); n != 0 {
		t.Fatal("通知记录未清理")
	}
}

func TestDeleteOrphanEvents(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	sv := Server{Name: "a"}
	if err := s.CreateServer(ctx, &sv); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{sv.ID, "gone"} {
		if _, err := s.CreateOutage(ctx, id, 1); err != nil {
			t.Fatal(err)
		}
		if _, err := s.AddNotifyLog(ctx, NotifyLog{ServerID: id, Kind: "offline", Channel: "webhook", Status: LogSent, CreatedAt: 1}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.AddNotifyLog(ctx, NotifyLog{Kind: "test", Channel: "webhook", Status: LogSent, CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	if n, err := s.DeleteOrphanOutages(ctx); err != nil || n != 1 {
		t.Fatalf("应删除 1 条孤儿离线记录，实际 %d %v", n, err)
	}
	if n, err := s.DeleteOrphanNotifyLog(ctx); err != nil || n != 1 {
		t.Fatalf("应删除 1 条孤儿通知记录，实际 %d %v", n, err)
	}
	if list, _, _ := s.ListOutages(ctx, "", 10, 0); len(list) != 1 || list[0].ServerID != sv.ID {
		t.Fatalf("现存服务器的离线记录应保留 %+v", list)
	}
	list, _, _ := s.ListNotifyLog(ctx, "", "", 10, 0)
	ids := map[string]bool{}
	for _, l := range list {
		ids[l.ServerID] = true
	}
	if len(list) != 2 || !ids[sv.ID] || !ids[""] {
		t.Fatalf("现存服务器与测试消息的通知记录应保留 %+v", list)
	}
}

func TestFailPendingNotifyLog(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	pending, _ := s.AddNotifyLog(ctx, NotifyLog{ServerID: "a", Kind: "offline", Channel: "webhook", Status: LogPending, CreatedAt: 1})
	sent, _ := s.AddNotifyLog(ctx, NotifyLog{ServerID: "a", Kind: "offline", Channel: "telegram", Status: LogPending, CreatedAt: 1})
	if err := s.FinishNotifyLog(ctx, sent, LogSent, "", 5); err != nil {
		t.Fatal(err)
	}
	if n, err := s.FailPendingNotifyLog(ctx, "退出时未能发送", 10); err != nil || n != 1 {
		t.Fatalf("应更新 1 条，实际 %d %v", n, err)
	}
	list, _, _ := s.ListNotifyLog(ctx, "", "", 10, 0)
	for _, l := range list {
		switch l.ID {
		case pending:
			if l.Status != LogFailed || l.Error != "退出时未能发送" || l.DoneAt == nil || *l.DoneAt != 10 {
				t.Fatalf("发送中的记录应标记为失败 %+v", l)
			}
		case sent:
			if l.Status != LogSent || *l.DoneAt != 5 {
				t.Fatalf("已完成的记录不应改动 %+v", l)
			}
		}
	}
}
