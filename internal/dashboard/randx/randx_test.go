package randx

import "testing"

func TestString(t *testing.T) {
	a, b := String(32), String(32)
	if len(a) != 32 || a == b {
		t.Fatalf("随机串异常: %q %q", a, b)
	}
	for _, r := range a {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			t.Fatalf("包含非字母数字字符: %q", a)
		}
	}
}
