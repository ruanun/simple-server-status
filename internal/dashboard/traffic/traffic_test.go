package traffic

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/ruanun/simple-server-status/internal/dashboard/store"
)

var cst = time.FixedZone("CST", 8*3600)

type clock struct{ t time.Time }

func (c *clock) Now() time.Time { return c.t }

func newAcc(t *testing.T, at time.Time) (*Accumulator, *store.Store, *clock) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	c := &clock{t: at}
	return New(st, c.Now), st, c
}

func mustAdd(t *testing.T, a *Accumulator, in, out uint64) {
	t.Helper()
	if err := a.Add(context.Background(), "s", 1, "", in, out); err != nil {
		t.Fatal(err)
	}
}

func usage(t *testing.T, a *Accumulator) (int64, int64, string) {
	t.Helper()
	in, out, period, err := a.Usage(context.Background(), "s", 1)
	if err != nil {
		t.Fatal(err)
	}
	return in, out, period
}

func TestPeriodStart(t *testing.T) {
	cases := []struct {
		now  time.Time
		day  int
		want string
	}{
		{time.Date(2026, 3, 15, 10, 0, 0, 0, cst), 1, "2026-03-01"},
		{time.Date(2026, 3, 15, 10, 0, 0, 0, cst), 20, "2026-02-20"},
		{time.Date(2026, 1, 5, 0, 0, 0, 0, cst), 10, "2025-12-10"},
		{time.Date(2026, 3, 20, 0, 0, 0, 0, cst), 20, "2026-03-20"},
		{time.Date(2026, 3, 15, 0, 0, 0, 0, cst), 0, "2026-03-01"},
		{time.Date(2026, 3, 15, 0, 0, 0, 0, cst), 31, "2026-03-01"},
	}
	for _, c := range cases {
		if got := PeriodStart(c.now, c.day).Format("2006-01-02"); got != c.want {
			t.Errorf("PeriodStart(%v, %d) = %s，期望 %s", c.now, c.day, got, c.want)
		}
	}
}

func TestAddCountsDeltasAndCounterReset(t *testing.T) {
	a, _, _ := newAcc(t, time.Date(2026, 3, 15, 12, 0, 0, 0, cst))
	mustAdd(t, a, 1000, 500) // 首次上报只建立基线
	if in, out, period := usage(t, a); in != 0 || out != 0 || period != "2026-03-01" {
		t.Fatalf("首次上报后 = %d %d %s", in, out, period)
	}
	mustAdd(t, a, 1600, 700)
	if in, out, _ := usage(t, a); in != 600 || out != 200 {
		t.Fatalf("增量错误: %d %d", in, out)
	}
	mustAdd(t, a, 100, 50) // Agent 重启，计数器归零
	if in, out, _ := usage(t, a); in != 700 || out != 250 {
		t.Fatalf("计数器归零后应以新值为增量: %d %d", in, out)
	}
}

func TestPeriodRollover(t *testing.T) {
	a, st, c := newAcc(t, time.Date(2026, 3, 15, 12, 0, 0, 0, cst))
	mustAdd(t, a, 1000, 1000)
	mustAdd(t, a, 2000, 2000)
	c.t = time.Date(2026, 4, 1, 1, 0, 0, 0, cst)
	mustAdd(t, a, 2500, 2600)
	if in, out, period := usage(t, a); in != 500 || out != 600 || period != "2026-04-01" {
		t.Fatalf("新周期 = %d %d %s", in, out, period)
	}
	old, err := st.GetTraffic(context.Background(), "s", "2026-03-01")
	if err != nil || old.In != 1000 || old.Out != 1000 {
		t.Fatalf("旧周期应已保存: %+v %v", old, err)
	}
}

func TestFlushAndRestore(t *testing.T) {
	a, st, c := newAcc(t, time.Date(2026, 3, 15, 12, 0, 0, 0, cst))
	mustAdd(t, a, 1000, 500)
	mustAdd(t, a, 1600, 700)
	if err := a.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	b := New(st, c.Now) // 模拟 Dashboard 重启
	if in, out, _ := usage(t, b); in != 600 || out != 200 {
		t.Fatalf("重启后应从数据库恢复: %d %d", in, out)
	}
	mustAdd(t, b, 1700, 800)
	if in, out, _ := usage(t, b); in != 700 || out != 300 {
		t.Fatalf("重启后应沿用基线继续累计: %d %d", in, out)
	}
}

func TestUsageUnknownServer(t *testing.T) {
	a, _, _ := newAcc(t, time.Date(2026, 3, 15, 12, 0, 0, 0, cst))
	if in, out, _ := usage(t, a); in != 0 || out != 0 {
		t.Fatalf("未知服务器应为 0: %d %d", in, out)
	}
}

func mustAddF(t *testing.T, a *Accumulator, filterID string, in, out uint64) {
	t.Helper()
	if err := a.Add(context.Background(), "s", 1, filterID, in, out); err != nil {
		t.Fatal(err)
	}
}

func TestAddSameFilterIDAccumulates(t *testing.T) {
	a, _, _ := newAcc(t, time.Date(2026, 3, 15, 12, 0, 0, 0, cst))
	mustAddF(t, a, "f1", 1000, 500)
	mustAddF(t, a, "f1", 1600, 700)
	if in, out, _ := usage(t, a); in != 600 || out != 200 {
		t.Fatalf("同一过滤器应正常累加: %d %d", in, out)
	}
}

func TestAddFilterIDChangeRebuildsBaseline(t *testing.T) {
	a, _, _ := newAcc(t, time.Date(2026, 3, 15, 12, 0, 0, 0, cst))
	mustAddF(t, a, "f1", 1000, 500)
	mustAddF(t, a, "f1", 1600, 700)
	// 过滤器缩小：计数变小，不应按计数器回退把新值整体计入
	mustAddF(t, a, "f2", 100, 50)
	if in, out, _ := usage(t, a); in != 600 || out != 200 {
		t.Fatalf("过滤器变化时不应累加: %d %d", in, out)
	}
	mustAddF(t, a, "f2", 150, 80)
	if in, out, _ := usage(t, a); in != 650 || out != 230 {
		t.Fatalf("过滤器变化后应以新基线继续累加: %d %d", in, out)
	}
	// 过滤器扩大：计数变大，不应把新网卡的历史流量计入
	mustAddF(t, a, "f3", 100000, 90000)
	if in, out, _ := usage(t, a); in != 650 || out != 230 {
		t.Fatalf("过滤器扩大时不应累加: %d %d", in, out)
	}
}

func TestAddEmptyEntryFilterIDAdoptsNew(t *testing.T) {
	a, st, c := newAcc(t, time.Date(2026, 3, 15, 12, 0, 0, 0, cst))
	mustAddF(t, a, "f1", 1000, 500)
	mustAddF(t, a, "f1", 1600, 700)
	if err := a.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	b := New(st, c.Now) // 模拟 Dashboard 重启：内存条目的 filterID 为空
	mustAddF(t, b, "f1", 1700, 800)
	if in, out, _ := usage(t, b); in != 700 || out != 300 {
		t.Fatalf("条目 filterID 为空时应直接采用并正常累加: %d %d", in, out)
	}
	mustAddF(t, b, "f2", 10, 10)
	if in, out, _ := usage(t, b); in != 700 || out != 300 {
		t.Fatalf("采用 filterID 后再变化应只重建基线: %d %d", in, out)
	}
}

func TestFilterIDSurvivesPeriodRollover(t *testing.T) {
	a, _, c := newAcc(t, time.Date(2026, 3, 15, 12, 0, 0, 0, cst))
	mustAddF(t, a, "f1", 1000, 1000)
	c.t = time.Date(2026, 4, 1, 1, 0, 0, 0, cst)
	mustAddF(t, a, "f2", 50, 50)
	if in, out, _ := usage(t, a); in != 0 || out != 0 {
		t.Fatalf("跨周期且过滤器变化时不应累加: %d %d", in, out)
	}
}
