package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWSURL(t *testing.T) {
	cases := map[string]string{
		"http://a:8900":       "ws://a:8900/api/agent/ws",
		"https://a.com/sub/":  "wss://a.com/sub/api/agent/ws",
		"ws://a":              "ws://a/api/agent/ws",
		" https://a.com?x=1 ": "wss://a.com/api/agent/ws",
	}
	for in, want := range cases {
		got, err := WSURL(in)
		if err != nil || got != want {
			t.Errorf("WSURL(%q) = %q, %v；期望 %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"a.com", "ftp://a", "http://"} {
		if _, err := WSURL(bad); err == nil {
			t.Errorf("WSURL(%q) 应报错", bad)
		}
	}
}

func TestLoadConfigFromFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.yaml")
	if err := os.WriteFile(p, []byte("dashboard: http://x\nid: i\nsecret: s\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Dashboard != "http://x" || cfg.ID != "i" || cfg.Secret != "s" {
		t.Fatalf("配置内容错误: %+v", cfg)
	}
	if !cfg.DetectCountry || cfg.LogLevel != "info" {
		t.Fatalf("未配置的字段应保留默认值: %+v", cfg)
	}
	if _, err := LoadConfig(filepath.Join(dir, "missing.yaml")); err == nil {
		t.Fatal("显式指定的文件不存在时应报错")
	}
}

func TestLoadConfigDefaultPathsMissing(t *testing.T) {
	t.Chdir(t.TempDir())
	old := DefaultConfigPaths
	DefaultConfigPaths = []string{"sss-agent.yaml"}
	t.Cleanup(func() { DefaultConfigPaths = old })
	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg != DefaultConfig() {
		t.Fatalf("应返回默认配置: %+v", cfg)
	}
}

func TestValidate(t *testing.T) {
	ok := Config{Dashboard: "http://x", ID: "i", Secret: "s"}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []Config{
		{ID: "i", Secret: "s"},
		{Dashboard: "http://x", Secret: "s"},
		{Dashboard: "http://x", ID: "i"},
		{Dashboard: "x", ID: "i", Secret: "s"},
	} {
		if err := bad.Validate(); err == nil {
			t.Errorf("配置 %+v 应校验失败", bad)
		}
	}
}
