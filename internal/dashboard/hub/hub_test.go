package hub

import (
	"testing"
	"time"

	"github.com/ruanun/simple-server-status/internal/proto"
)

type clock struct{ t time.Time }

func (c *clock) Now() time.Time { return c.t }

func newHub() (*Hub, *clock) {
	c := &clock{t: time.Unix(1000, 0)}
	return New(c.Now), c
}

func TestOnlineRequiresRecentReport(t *testing.T) {
	h, c := newHub()
	h.Connect("a", 2)
	if h.Get("a").Online {
		t.Fatal("未上报前不应在线")
	}
	h.Report("a", proto.Report{CPU: 1})
	if !h.Get("a").Online {
		t.Fatal("上报后应在线")
	}
	c.t = c.t.Add(6 * time.Second)
	if !h.Get("a").Online {
		t.Fatal("6 秒内应在线")
	}
	c.t = c.t.Add(time.Second)
	if h.Get("a").Online {
		t.Fatal("超过 6 秒应离线")
	}
}

func TestOnlineThresholdScalesWithInterval(t *testing.T) {
	h, c := newHub()
	h.Connect("a", 10)
	h.Report("a", proto.Report{})
	c.t = c.t.Add(30 * time.Second)
	if !h.Get("a").Online {
		t.Fatal("间隔 10 秒时 30 秒内应在线")
	}
	c.t = c.t.Add(time.Second)
	if h.Get("a").Online {
		t.Fatal("超过 30 秒应离线")
	}
}

func TestDisconnectIgnoresStaleSession(t *testing.T) {
	h, _ := newHub()
	s1 := h.Connect("a", 2)
	s2 := h.Connect("a", 2)
	h.Report("a", proto.Report{})
	h.Disconnect("a", s1)
	if !h.Get("a").Online {
		t.Fatal("旧会话断开不应影响新连接")
	}
	h.Disconnect("a", s2)
	if l := h.Get("a"); l.Online || l.Connected {
		t.Fatalf("当前会话断开后应离线: %+v", l)
	}
}

func TestReportUsesServerTimeAndStatic(t *testing.T) {
	h, c := newHub()
	h.SetStatic("a", proto.Hello{MemTotal: 1000})
	p := h.Report("a", proto.Report{TS: 1, MemUsed: 250})
	if p.Mem != 25 || p.TS != c.t.Unix() {
		t.Fatalf("指标点错误: %+v", p)
	}
	l := h.Get("a")
	if l.Report == nil || l.Report.TS != c.t.Unix() || l.LastReport != c.t.Unix() || l.Static == nil {
		t.Fatalf("实时状态错误: %+v", l)
	}
}

func TestRingKeepsTenMinutes(t *testing.T) {
	h, c := newHub()
	for i := 0; i < 700; i++ {
		h.Report("a", proto.Report{})
		c.t = c.t.Add(time.Second)
	}
	ring := h.Ring("a")
	if len(ring) != 601 || ring[0].TS != 1099 {
		t.Fatalf("环形缓冲长度 %d，首点 %d", len(ring), ring[0].TS)
	}
	if r := h.Ring("none"); r == nil || len(r) != 0 {
		t.Fatal("未知服务器应返回 []")
	}
}

func TestChangedAndRemove(t *testing.T) {
	h, _ := newHub()
	_, s0 := h.Changed(0)
	h.Report("a", proto.Report{})
	ids, s1 := h.Changed(s0)
	if len(ids) != 1 || ids[0] != "a" {
		t.Fatalf("Changed = %v", ids)
	}
	if ids, _ := h.Changed(s1); len(ids) != 0 {
		t.Fatalf("无变化时应为空: %v", ids)
	}
	h.Remove("a")
	if l := h.Get("a"); l.Report != nil || l.Online {
		t.Fatalf("删除后应无状态: %+v", l)
	}
}
