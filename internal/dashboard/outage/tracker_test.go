package outage

import (
	"context"
	"log/slog"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/ruanun/simple-server-status/internal/dashboard/hub"
	"github.com/ruanun/simple-server-status/internal/dashboard/store"
	"github.com/ruanun/simple-server-status/internal/proto"
)

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) Add(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

type src struct{ list []store.Server }

func (s *src) Servers() []store.Server { return s.list }

func setup(t *testing.T, servers ...store.Server) (*Tracker, *hub.Hub, *store.Store, *clock, *src) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "o.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	c := &clock{t: time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)}
	h := hub.New(c.Now)
	s := &src{list: servers}
	return New(s, h, st, c.Now, slog.New(slog.DiscardHandler), 0), h, st, c, s
}

func outages(t *testing.T, st *store.Store) []store.Outage {
	t.Helper()
	list, _, err := st.ListOutages(context.Background(), "", 100, 0)
	if err != nil {
		t.Fatal(err)
	}
	return list
}

func TestOutageOpenAfterThresholdAndClose(t *testing.T) {
	tr, h, st, c, _ := setup(t, store.Server{ID: "s1", Name: "hk"})
	ctx := context.Background()
	tr.Check(ctx) // 先度过启动宽限期
	c.Add(DefaultGrace)
	h.Connect("s1", 2)
	h.Report("s1", proto.Report{})
	lastReport := c.Now().Unix()
	c.Add(30 * time.Second)
	tr.Check(ctx)
	if len(outages(t, st)) != 0 {
		t.Fatal("离线不足 60 秒不应记录")
	}
	c.Add(40 * time.Second)
	tr.Check(ctx)
	list := outages(t, st)
	if len(list) != 1 || list[0].StartAt != lastReport || list[0].EndAt != nil {
		t.Fatalf("应新建进行中记录，开始时间为最后上报 %+v", list)
	}
	c.Add(time.Minute)
	tr.Check(ctx)
	if len(outages(t, st)) != 1 {
		t.Fatal("仍离线不应重复新建")
	}
	h.Report("s1", proto.Report{})
	recovered := c.Now().Unix()
	tr.Check(ctx)
	list = outages(t, st)
	if len(list) != 1 || list[0].EndAt == nil || *list[0].EndAt != recovered {
		t.Fatalf("恢复后应补结束时间 %+v", list)
	}
}

func TestOutageNeverSeenSkipped(t *testing.T) {
	tr, _, st, c, _ := setup(t, store.Server{ID: "s1"})
	c.Add(time.Hour)
	tr.Check(context.Background())
	if len(outages(t, st)) != 0 {
		t.Fatal("从未上报不应记录")
	}
}

func TestOutageRestartClosesOpen(t *testing.T) {
	tr, h, st, c, s := setup(t, store.Server{ID: "on"}, store.Server{ID: "off", LastSeen: 1})
	ctx := context.Background()
	idOn, _ := st.CreateOutage(ctx, "on", 100)
	if _, err := st.CreateOutage(ctx, "off", 1); err != nil {
		t.Fatal(err)
	}
	h.Connect("on", 2)
	h.Report("on", proto.Report{})
	tr.Check(ctx) // 首轮读取进行中记录
	list := outages(t, st)
	byID := map[int64]store.Outage{}
	for _, o := range list {
		byID[o.ID] = o
	}
	if len(list) != 2 || byID[idOn].EndAt == nil || *byID[idOn].EndAt != c.Now().Unix() {
		t.Fatalf("重启后已在线的应关闭、离线的保持，且不重复新建 %+v", list)
	}
	s.list = []store.Server{{ID: "on"}} // off 被删除
	tr.Check(ctx)
	if _, ok := tr.open["off"]; ok {
		t.Fatal("已删除服务器的内存状态应清理")
	}
}

func TestOutageGraceAfterStart(t *testing.T) {
	c0 := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	lastSeen := c0.Add(-10 * time.Minute).Unix() // 停机前的最后上报
	tr, h, st, c, _ := setup(t, store.Server{ID: "off", LastSeen: lastSeen}, store.Server{ID: "back", LastSeen: lastSeen})
	ctx := context.Background()
	tr.Check(ctx)
	if len(outages(t, st)) != 0 {
		t.Fatal("宽限期内即使离线已超过门槛也不应新建记录")
	}
	c.Add(time.Minute)
	h.Connect("back", 2) // 宽限期内重连
	h.Report("back", proto.Report{})
	tr.Check(ctx)
	if len(outages(t, st)) != 0 {
		t.Fatal("宽限期内不应新建记录")
	}
	c.Add(DefaultGrace - time.Minute)
	h.Report("back", proto.Report{}) // 重连后持续上报
	tr.Check(ctx)
	list := outages(t, st)
	if len(list) != 1 || list[0].ServerID != "off" || list[0].StartAt != lastSeen || list[0].EndAt != nil {
		t.Fatalf("宽限期后仍离线的应新建记录，开始时间为最后上报 %+v", list)
	}
}

func TestOutageGraceScalesWithThreshold(t *testing.T) {
	if g := New(nil, nil, nil, time.Now, slog.New(slog.DiscardHandler), 0).grace; g != DefaultGrace {
		t.Fatalf("默认门槛的宽限期应为 %v，实际 %v", DefaultGrace, g)
	}
	if g := New(nil, nil, nil, time.Now, slog.New(slog.DiscardHandler), 4*time.Second).grace; g != 8*time.Second {
		t.Fatalf("测试门槛的宽限期应为门槛的 2 倍，实际 %v", g)
	}
}
