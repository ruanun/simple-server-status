package store

import (
	"context"
	"errors"
	"path/filepath"
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
	if err := s.db.QueryRow(`SELECT MAX(version) FROM schema_version`).Scan(&v); err != nil || v != 1 {
		t.Fatalf("schema_version = %d, %v", v, err)
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
