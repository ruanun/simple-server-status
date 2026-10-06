package incident

import (
	"context"
	"encoding/json"
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

type src struct {
	list []store.Server
	cfg  store.EventSettings
}

func (s *src) Servers() []store.Server            { return s.list }
func (s *src) EventSettings() store.EventSettings { return s.cfg }

type fixture struct {
	d   *Detector
	rec *Recorder
	hub *hub.Hub
	st  *store.Store
	c   *clock
	src *src
}

func setup(t *testing.T, servers ...store.Server) *fixture {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "i.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	c := &clock{t: time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)}
	h := hub.New(c.Now)
	s := &src{list: servers, cfg: store.DefaultEventSettings()}
	log := slog.New(slog.DiscardHandler)
	rec := NewRecorder(st, c.Now, log)
	return &fixture{d: NewDetector(s, h, rec, c.Now, log, 0), rec: rec, hub: h, st: st, c: c, src: s}
}

func (f *fixture) events(t *testing.T, kinds ...string) []store.Event {
	t.Helper()
	list, _, err := f.st.ListEvents(context.Background(), store.EventFilter{Kinds: kinds}, 100, 0)
	if err != nil {
		t.Fatal(err)
	}
	return list
}

// drain 取出已发布的全部变更
func (f *fixture) drain() []Change {
	var out []Change
	for {
		select {
		case c := <-f.rec.Changes():
			out = append(out, c)
		default:
			return out
		}
	}
}

func TestOfflineOpenAfterThresholdAndClose(t *testing.T) {
	f := setup(t, store.Server{ID: "s1", Name: "hk"})
	ctx := context.Background()
	f.d.Check(ctx) // 先度过启动宽限期
	f.c.Add(DefaultGrace)
	f.hub.Connect("s1", 2)
	f.hub.Report("s1", proto.Report{})
	lastReport := f.c.Now().Unix()
	f.c.Add(30 * time.Second)
	f.d.Check(ctx)
	if len(f.events(t)) != 0 {
		t.Fatal("离线不足 60 秒不应记录")
	}
	f.c.Add(40 * time.Second)
	f.d.Check(ctx)
	list := f.events(t)
	if len(list) != 1 || list[0].Kind != KindOffline || list[0].StartAt != lastReport || list[0].EndAt != nil {
		t.Fatalf("应新建进行中的离线事件，开始时间为最后上报 %+v", list)
	}
	f.c.Add(time.Minute)
	f.d.Check(ctx)
	if len(f.events(t)) != 1 {
		t.Fatal("仍离线不应重复新建")
	}
	f.hub.Report("s1", proto.Report{})
	recovered := f.c.Now().Unix()
	f.d.Check(ctx)
	list = f.events(t)
	if len(list) != 1 || list[0].EndAt == nil || *list[0].EndAt != recovered {
		t.Fatalf("恢复后应补结束时间 %+v", list)
	}
	cs := f.drain()
	if len(cs) != 2 || cs[0].Type != Opened || cs[1].Type != Closed || cs[1].Event.EndAt == nil {
		t.Fatalf("应发布开始与结束两次变更 %+v", cs)
	}
}

func TestOfflineNeverSeenSkipped(t *testing.T) {
	f := setup(t, store.Server{ID: "s1"})
	f.c.Add(time.Hour)
	f.d.Check(context.Background())
	if len(f.events(t)) != 0 {
		t.Fatal("从未上报不应记录")
	}
}

func TestRestartLoadsOpenEvents(t *testing.T) {
	f := setup(t, store.Server{ID: "on"}, store.Server{ID: "off", LastSeen: 1})
	ctx := context.Background()
	on := store.Event{ServerID: "on", Kind: KindOffline, StartAt: 100}
	off := store.Event{ServerID: "off", Kind: KindOffline, StartAt: 1}
	for _, e := range []*store.Event{&on, &off} {
		if err := f.st.CreateEvent(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	f.hub.Connect("on", 2)
	f.hub.Report("on", proto.Report{})
	f.d.Check(ctx) // 首轮加载进行中的事件
	byID := map[int64]store.Event{}
	for _, e := range f.events(t) {
		byID[e.ID] = e
	}
	if len(byID) != 2 || byID[on.ID].EndAt == nil || *byID[on.ID].EndAt != f.c.Now().Unix() || byID[off.ID].EndAt != nil {
		t.Fatalf("重启后已在线的应结束、离线的保持，且不重复新建 %+v", byID)
	}
	if tr, ok := f.rec.Ongoing("off", KindOffline); !ok || !tr.OpenedAt.IsZero() {
		t.Fatalf("从数据库加载的事件 OpenedAt 应为零值 %+v", tr)
	}
	f.src.list = []store.Server{{ID: "on"}} // off 被删除
	f.d.Check(ctx)
	if _, ok := f.rec.Ongoing("off", KindOffline); ok {
		t.Fatal("已删除服务器的内存状态应清理")
	}
}

func TestOfflineGraceAfterStart(t *testing.T) {
	c0 := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	lastSeen := c0.Add(-10 * time.Minute).Unix() // 停机前的最后上报
	f := setup(t, store.Server{ID: "off", LastSeen: lastSeen}, store.Server{ID: "back", LastSeen: lastSeen})
	ctx := context.Background()
	f.d.Check(ctx)
	if len(f.events(t)) != 0 {
		t.Fatal("宽限期内即使离线已超过门槛也不应新建")
	}
	f.c.Add(time.Minute)
	f.hub.Connect("back", 2) // 宽限期内重连
	f.hub.Report("back", proto.Report{})
	f.d.Check(ctx)
	if len(f.events(t)) != 0 {
		t.Fatal("宽限期内不应新建")
	}
	f.c.Add(DefaultGrace - time.Minute)
	f.hub.Report("back", proto.Report{}) // 重连后持续上报
	f.d.Check(ctx)
	list := f.events(t)
	if len(list) != 1 || list[0].ServerID != "off" || list[0].StartAt != lastSeen || list[0].EndAt != nil {
		t.Fatalf("宽限期后仍离线的应新建，开始时间为最后上报 %+v", list)
	}
}

func TestGraceScalesWithThreshold(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
	if g := NewDetector(nil, nil, nil, time.Now, log, 0).grace; g != DefaultGrace {
		t.Fatalf("默认门槛的宽限期应为 %v，实际 %v", DefaultGrace, g)
	}
	if g := NewDetector(nil, nil, nil, time.Now, log, 4*time.Second).grace; g != 8*time.Second {
		t.Fatalf("测试门槛的宽限期应为门槛的 2 倍，实际 %v", g)
	}
}

// report 每 30 秒上报一次，共 n 次
func (f *fixture) report(id string, n int, r proto.Report) {
	for range n {
		f.c.Add(30 * time.Second)
		f.hub.Report(id, r)
	}
}

func loadDetail(t *testing.T, e store.Event) LoadDetail {
	t.Helper()
	var d LoadDetail
	if err := json.Unmarshal(e.Detail, &d); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestLoadPerMetric(t *testing.T) {
	f := setup(t, store.Server{ID: "s1"})
	ctx := context.Background()
	f.hub.Connect("s1", 2)
	f.hub.Report("s1", proto.Report{CPU: 99})
	f.d.Check(ctx)
	if len(f.events(t)) != 0 {
		t.Fatal("窗口内不足 2 个点不判断")
	}
	f.report("s1", 10, proto.Report{CPU: 95})
	f.d.Check(ctx)
	list := f.events(t, KindLoadCPU)
	if len(list) != 1 || list[0].EndAt != nil {
		t.Fatalf("CPU 超过阈值应开始事件 %+v", list)
	}
	if d := loadDetail(t, list[0]); d.Threshold != 90 || d.Minutes != 5 || d.Peak < 95 {
		t.Fatalf("详情错误 %+v", d)
	}
	if len(f.events(t, KindLoadMem, KindLoadDisk)) != 0 {
		t.Fatal("内存与硬盘未超过阈值不应记录")
	}

	f.report("s1", 12, proto.Report{CPU: 99}) // 窗口为闭区间，多报两次确保窗口内全是新值
	f.d.Check(ctx)
	if d := loadDetail(t, f.events(t, KindLoadCPU)[0]); d.Peak != 99 {
		t.Fatalf("峰值应更新为 99，实际 %+v", d)
	}
	f.report("s1", 10, proto.Report{CPU: 50})
	f.d.Check(ctx)
	if list := f.events(t, KindLoadCPU); len(list) != 1 || list[0].EndAt == nil {
		t.Fatalf("窗口均值低于阈值应结束 %+v", list)
	}
}

func TestLoadKeptWhileOffline(t *testing.T) {
	f := setup(t, store.Server{ID: "s1"})
	ctx := context.Background()
	f.hub.Connect("s1", 2)
	f.report("s1", 10, proto.Report{CPU: 99})
	f.d.Check(ctx)
	f.c.Add(10 * time.Minute) // 离线：窗口内已没有数据点
	f.d.Check(ctx)
	if list := f.events(t, KindLoadCPU); len(list) != 1 || list[0].EndAt != nil {
		t.Fatalf("离线期间负载事件应保持 %+v", list)
	}
}

func TestRebooted(t *testing.T) {
	old := &proto.Hello{BootID: "a"}
	cases := []struct {
		old  *proto.Hello
		cur  string
		want bool
	}{
		{nil, "a", false},            // 首次上报
		{old, "a", false},            // 同一次开机
		{old, "b", true},             // 重启
		{old, "", false},             // 新的 Agent 不支持或读取失败
		{&proto.Hello{}, "b", false}, // 旧版 Agent 升级后首次上报
	}
	for i, c := range cases {
		if got := Rebooted(c.old, proto.Hello{BootID: c.cur}); got != c.want {
			t.Errorf("第 %d 个用例：得到 %v，期望 %v", i, got, c.want)
		}
	}
}

func TestIPChanges(t *testing.T) {
	old := &proto.Hello{IPv4: "1.1.1.1", IPv6: "::1"}
	if got := IPChanges(nil, proto.Hello{IPv4: "2.2.2.2"}); len(got) != 0 {
		t.Fatalf("首次上报不算变化 %v", got)
	}
	if got := IPChanges(old, proto.Hello{IPv4: "1.1.1.1", IPv6: ""}); len(got) != 0 {
		t.Fatalf("新值为空不算变化 %v", got)
	}
	got := IPChanges(old, proto.Hello{IPv4: "2.2.2.2", IPv6: "::1"})
	if len(got) != 1 || got["ipv4"] != [2]string{"1.1.1.1", "2.2.2.2"} {
		t.Fatalf("应只包含 IPv4 的变化 %v", got)
	}
}
