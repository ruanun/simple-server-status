package traffic

import (
	"context"
	"path/filepath"
	"reflect"
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

func TestDailyTraffic(t *testing.T) {
	ctx := context.Background()
	a, _, c := newAcc(t, time.Date(2026, 9, 1, 23, 59, 0, 0, cst))
	mustAdd(t, a, 100, 100) // 建立基线
	mustAdd(t, a, 300, 150) // 9/1：+200 / +50
	c.t = time.Date(2026, 9, 2, 0, 1, 0, 0, cst)
	mustAdd(t, a, 50, 160) // 9/2：入站计数器归零，+50 / +10
	want := []store.DailyTraffic{{Day: "2026-09-01", In: 200, Out: 50}, {Day: "2026-09-02", In: 50, Out: 10}, {Day: "2026-09-03", In: 0, Out: 0}}
	got, err := a.Daily(ctx, "s", "2026-09-01", "2026-09-03")
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("未落库时 %+v %v", got, err)
	}
	if err := a.Flush(ctx); err != nil {
		t.Fatal(err)
	}
	if got, _ = a.Daily(ctx, "s", "2026-09-01", "2026-09-03"); !reflect.DeepEqual(got, want) {
		t.Fatalf("落库后 %+v", got)
	}
	mustAdd(t, a, 60, 170) // 9/2 再 +10 / +10，尚未落库
	got, _ = a.Daily(ctx, "s", "2026-09-02", "2026-09-02")
	if !reflect.DeepEqual(got, []store.DailyTraffic{{Day: "2026-09-02", In: 60, Out: 20}}) {
		t.Fatalf("合并未落库增量错误 %+v", got)
	}
}

func TestDailyIgnoresFilterChange(t *testing.T) {
	ctx := context.Background()
	a, _, _ := newAcc(t, time.Date(2026, 9, 1, 12, 0, 0, 0, cst))
	mustAddF(t, a, "f1", 100, 100)
	mustAddF(t, a, "f1", 200, 200)
	mustAddF(t, a, "f2", 5000, 5000) // 口径变化只重建基线
	got, _ := a.Daily(ctx, "s", "2026-09-01", "2026-09-01")
	if got[0].In != 100 || got[0].Out != 100 {
		t.Fatalf("口径变化不应计入每日流量 %+v", got)
	}
}

func TestDailyForgetAndCleanup(t *testing.T) {
	ctx := context.Background()
	a, st, _ := newAcc(t, time.Date(2026, 9, 1, 12, 0, 0, 0, cst))
	mustAdd(t, a, 100, 100)
	mustAdd(t, a, 200, 200)
	a.Forget("s")
	if got, _ := a.Daily(ctx, "s", "2026-09-01", "2026-09-01"); got[0].In != 0 {
		t.Fatalf("Forget 后不应保留未落库增量 %+v", got)
	}
	if err := st.AddTrafficDaily(ctx, []store.DailyDelta{{ServerID: "s", Day: "2025-01-01", In: 1}, {ServerID: "s", Day: "2026-08-31", In: 1}}); err != nil {
		t.Fatal(err)
	}
	if err := a.CleanupDaily(ctx); err != nil {
		t.Fatal(err)
	}
	rows, _ := st.QueryTrafficDaily(ctx, "s", "2000-01-01", "2099-12-31")
	if len(rows) != 1 || rows[0].Day != "2026-08-31" {
		t.Fatalf("应只清理 400 天前的数据 %+v", rows)
	}
}

func TestUsed(t *testing.T) {
	if Used("in", 3, 5) != 3 || Used("out", 3, 5) != 5 || Used("sum", 3, 5) != 8 || Used("", 3, 5) != 8 {
		t.Fatal("Used 计算错误")
	}
}
