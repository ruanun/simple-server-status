package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionCommand(t *testing.T) {
	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"version"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out.String()) != "dev" {
		t.Fatalf("version 输出 %q", out.String())
	}
}

func TestServeFlagsRegistered(t *testing.T) {
	root := newRootCmd()
	for _, name := range []string{"listen", "trusted-proxies", "admin-password", "data-dir", "log-level", "log-file"} {
		if root.Flags().Lookup(name) == nil && root.PersistentFlags().Lookup(name) == nil {
			t.Errorf("缺少参数 --%s", name)
		}
	}
}
