package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveConfigPriority(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.yaml")
	if err := os.WriteFile(p, []byte("dashboard: http://file\nid: file-id\nsecret: file-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SSS_ID", "env-id")
	t.Setenv("SSS_SECRET", "env-secret")
	fs := newRootCmd().PersistentFlags()
	if err := fs.Parse([]string{"--config", p, "--id", "cli-id"}); err != nil {
		t.Fatal(err)
	}
	cfg, err := resolveConfig(fs)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Dashboard != "http://file" {
		t.Errorf("dashboard 应来自文件，实际 %q", cfg.Dashboard)
	}
	if cfg.ID != "cli-id" {
		t.Errorf("id 应来自命令行，实际 %q", cfg.ID)
	}
	if cfg.Secret != "env-secret" {
		t.Errorf("secret 应来自环境变量，实际 %q", cfg.Secret)
	}
}

func TestResolveConfigRequiresDashboard(t *testing.T) {
	t.Chdir(t.TempDir())
	fs := newRootCmd().PersistentFlags()
	if err := fs.Parse([]string{"--config", ""}); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveConfig(fs); err == nil {
		t.Fatal("缺少 dashboard 时应报错")
	}
}
