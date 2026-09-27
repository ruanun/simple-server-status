package dashboard

import (
	"context"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/ruanun/simple-server-status/internal/dashboard/auth"
	"github.com/ruanun/simple-server-status/internal/dashboard/store"
)

func TestBootstrapAdminAndResetPassword(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, DBFile))
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.DiscardHandler)
	if err := bootstrapAdmin(ctx, st, "password123", log); err != nil {
		t.Fatal(err)
	}
	if err := bootstrapAdmin(ctx, st, "other-password", log); err != nil {
		t.Fatal(err)
	}
	u, err := st.GetUserByName(ctx, "admin")
	if err != nil || !auth.CheckPassword(u.PasswordHash, "password123") {
		t.Fatalf("应只在首次创建管理员: %+v %v", u, err)
	}
	_ = st.Close()

	pw, err := ResetPassword(ctx, dir)
	if err != nil || len(pw) != 16 {
		t.Fatalf("ResetPassword = %q, %v", pw, err)
	}
	st, _ = store.Open(filepath.Join(dir, DBFile))
	defer st.Close()
	u, _ = st.GetUserByName(ctx, "admin")
	if !auth.CheckPassword(u.PasswordHash, pw) || u.TokenVersion != 1 {
		t.Fatalf("重置后密码或 token 版本错误: %+v", u)
	}
}
