package collect

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestCollectorSmoke(t *testing.T) {
	c := New(slog.New(slog.DiscardHandler))
	h := c.Static("test")
	if h.AgentVersion != "test" || h.CPUCores <= 0 || h.MemTotal == 0 || h.Arch == "" {
		t.Fatalf("静态信息异常: %+v", h)
	}
	r := c.Sample()
	if r.TS == 0 || r.Disks == nil {
		t.Fatalf("上报异常: %+v", r)
	}
	if r.NetInSpeed != 0 || r.NetOutSpeed != 0 {
		t.Fatal("首次采样的网速应为 0")
	}
}

func TestWarnRateLimited(t *testing.T) {
	var buf bytes.Buffer
	c := New(slog.New(slog.NewTextHandler(&buf, nil)))
	now := time.Unix(0, 0)
	c.now = func() time.Time { return now }
	c.warnf("cpu", errors.New("x"))
	c.warnf("cpu", errors.New("x"))
	now = now.Add(11 * time.Minute)
	c.warnf("cpu", errors.New("x"))
	if n := strings.Count(buf.String(), "采集失败"); n != 2 {
		t.Fatalf("期望 2 条告警，实际 %d:\n%s", n, buf.String())
	}
}

func TestSetFilterResetsNetBaseline(t *testing.T) {
	c := New(slog.New(slog.DiscardHandler))
	now := time.Unix(1000, 0)
	c.now = func() time.Time { return now }
	c.Sample()
	if c.lastAt.IsZero() {
		t.Fatal("采样后应记录网速基线时间")
	}
	c.SetFilter(Filter{NICInclude: []string{"eth"}})
	if !c.lastAt.IsZero() {
		t.Fatal("SetFilter 后应重置网速基线")
	}
	now = now.Add(2 * time.Second)
	if r := c.Sample(); r.NetInSpeed != 0 || r.NetOutSpeed != 0 {
		t.Fatalf("重置基线后的首次采样网速应为 0: %d %d", r.NetInSpeed, r.NetOutSpeed)
	}
}
