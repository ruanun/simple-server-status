package api

import "testing"

func TestCompareSemver(t *testing.T) {
	cases := []struct {
		a, b string
		want int
		ok   bool
	}{
		{"2.0.0", "2.0.0", 0, true},
		{"v2.0.1", "2.0.0", 1, true},
		{"2.0.0-beta.1", "2.0.0", -1, true},
		{"2.0.0-beta.2", "2.0.0-beta.10", -1, true},
		{"2.0.0-beta.1", "2.0.0-rc.1", -1, true},
		{"2.0.0-1", "2.0.0-beta", -1, true},
		{"1.10.0", "1.9.9", 1, true},
		{"dev", "2.0.0", 0, false},
		{"2.0.0", "", 0, false},
	}
	for _, c := range cases {
		got, ok := compareSemver(c.a, c.b)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("compareSemver(%q, %q) = %d, %v；期望 %d, %v", c.a, c.b, got, ok, c.want, c.ok)
		}
	}
	if !agentOutdated("2.0.0-beta.1", "2.0.0-beta.2") || agentOutdated("2.0.0", "2.0.0") || agentOutdated("dev", "2.0.0") {
		t.Error("agentOutdated 判断错误")
	}
}
