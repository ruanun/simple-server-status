package collect

import (
	"runtime"
	"testing"
)

func TestBootIDStable(t *testing.T) {
	switch runtime.GOOS {
	case "linux", "windows", "darwin":
	default:
		t.Skip("该平台没有开机标识")
	}
	a, err := bootID()
	if err != nil || a == "" {
		t.Fatalf("应能读取开机标识: %q %v", a, err)
	}
	if b, _ := bootID(); b != a {
		t.Fatalf("同一次开机内应保持不变: %q != %q", a, b)
	}
}
