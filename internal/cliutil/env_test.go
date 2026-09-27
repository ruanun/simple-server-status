package cliutil

import (
	"testing"

	"github.com/spf13/pflag"
)

func TestApplyEnvRespectsCommandLine(t *testing.T) {
	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	fs.String("listen", ":8900", "")
	fs.String("data-dir", "./data", "")
	if err := fs.Parse([]string{"--data-dir", "cli"}); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SSS_LISTEN", ":9000")
	t.Setenv("SSS_DATA_DIR", "env")
	if err := ApplyEnv(fs, "SSS"); err != nil {
		t.Fatal(err)
	}
	if v, _ := fs.GetString("listen"); v != ":9000" {
		t.Errorf("listen 应取环境变量，实际 %q", v)
	}
	if v, _ := fs.GetString("data-dir"); v != "cli" {
		t.Errorf("data-dir 应保留命令行值，实际 %q", v)
	}
	if !fs.Changed("listen") {
		t.Error("由环境变量设置的参数应标记为 Changed")
	}
}

func TestApplyEnvInvalidValue(t *testing.T) {
	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	fs.Int("port", 1, "")
	if err := fs.Parse(nil); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SSS_PORT", "abc")
	if err := ApplyEnv(fs, "SSS"); err == nil {
		t.Fatal("非法值应报错")
	}
}
