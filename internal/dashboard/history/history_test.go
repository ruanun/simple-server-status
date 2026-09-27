package history

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/ruanun/simple-server-status/internal/dashboard/metric"
	"github.com/ruanun/simple-server-status/internal/dashboard/store"
)

type clock struct{ t time.Time }

func (c *clock) Now() time.Time { return c.t }

func newRecorder(t *testing.T, at int64) (*Recorder, *store.Store, *clock) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "h.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	c := &clock{t: time.Unix(at, 0)}
	return NewRecorder(st, c.Now, slog.New(slog.DiscardHandler)), st, c
}

func query(t *testing.T, st *store.Store, table string) []metric.Point {
	t.Helper()
	pts, err := st.QueryMetrics(context.Background(), table, "a", 0, 1<<40)
	if err != nil {
		t.Fatal(err)
	}
	return pts
}

func TestAddAggregatesMinute(t *testing.T) {
	r, st, _ := newRecorder(t, 120)
	r.Add("a", metric.Point{TS: 60, CPU: 10})
	r.Add("a", metric.Point{TS: 90, CPU: 30})
	r.Add("a", metric.Point{TS: 125, CPU: 50}) // 进入下一分钟
	if err := r.Flush(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	pts := query(t, st, store.Metrics1m)
	if len(pts) != 1 || pts[0].TS != 60 || pts[0].CPU != 20 {
		t.Fatalf("分钟聚合错误: %+v", pts)
	}
}

func TestFlushWritesStaleBucket(t *testing.T) {
	r, st, c := newRecorder(t, 100)
	r.Add("a", metric.Point{TS: 70, CPU: 10})
	_ = r.Flush(context.Background(), false)
	if pts := query(t, st, store.Metrics1m); len(pts) != 0 {
		t.Fatalf("当前分钟未结束不应写入: %+v", pts)
	}
	c.t = time.Unix(130, 0)
	_ = r.Flush(context.Background(), false)
	if pts := query(t, st, store.Metrics1m); len(pts) != 1 || pts[0].TS != 60 {
		t.Fatalf("已结束的分钟应写入（即使服务器已无新上报）: %+v", pts)
	}
}

func TestFlushAllIncludesCurrentMinute(t *testing.T) {
	r, st, _ := newRecorder(t, 80)
	r.Add("a", metric.Point{TS: 70, CPU: 10})
	if err := r.Flush(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	if pts := query(t, st, store.Metrics1m); len(pts) != 1 {
		t.Fatalf("退出时应写入当前分钟: %+v", pts)
	}
}

func TestTickRollsUpTenMinutes(t *testing.T) {
	r, st, _ := newRecorder(t, 1250)
	_ = st.InsertMetrics(context.Background(), store.Metrics1m, []store.MetricRow{
		{ServerID: "a", Point: metric.Point{TS: 600, CPU: 10}},
		{ServerID: "a", Point: metric.Point{TS: 660, CPU: 30}},
	})
	r.Tick(context.Background())
	pts := query(t, st, store.Metrics10m)
	if len(pts) != 1 || pts[0].TS != 600 || pts[0].CPU != 20 {
		t.Fatalf("10 分钟聚合错误: %+v", pts)
	}
}

func TestTickCleansUpExpired(t *testing.T) {
	r, st, _ := newRecorder(t, 100000)
	_ = st.InsertMetrics(context.Background(), store.Metrics1m, []store.MetricRow{
		{ServerID: "a", Point: metric.Point{TS: 10}},
		{ServerID: "a", Point: metric.Point{TS: 99990}},
	})
	r.Tick(context.Background())
	pts := query(t, st, store.Metrics1m)
	if len(pts) != 1 || pts[0].TS != 99990 {
		t.Fatalf("应只保留 24 小时内的数据: %+v", pts)
	}
}

func TestQueryRanges(t *testing.T) {
	ctx := context.Background()
	_, st, _ := newRecorder(t, 0)
	now := time.Unix(100000, 0)
	_ = st.InsertMetrics(ctx, store.Metrics1m, []store.MetricRow{
		{ServerID: "a", Point: metric.Point{TS: 50000, CPU: 99}},
		{ServerID: "a", Point: metric.Point{TS: 99600, CPU: 10}},
		{ServerID: "a", Point: metric.Point{TS: 99660, CPU: 20}},
		{ServerID: "a", Point: metric.Point{TS: 99900, CPU: 30}},
	})
	_ = st.InsertMetrics(ctx, store.Metrics10m, []store.MetricRow{{ServerID: "a", Point: metric.Point{TS: 99000, CPU: 40}}})
	ring := func(string) []metric.Point { return []metric.Point{{TS: 1}} }

	if pts, err := Query(ctx, st, ring, "a", Range1h, now); err != nil || len(pts) != 3 {
		t.Fatalf("1h = %+v, %v", pts, err)
	}
	pts, err := Query(ctx, st, ring, "a", Range24h, now)
	if err != nil || len(pts) != 3 || pts[1] != (metric.Point{TS: 99600, CPU: 15}) {
		t.Fatalf("24h 应聚合为 5 分钟: %+v, %v", pts, err)
	}
	if pts, _ := Query(ctx, st, ring, "a", Range7d, now); len(pts) != 1 || pts[0].CPU != 40 {
		t.Fatalf("7d = %+v", pts)
	}
	if pts, _ := Query(ctx, st, ring, "a", RangeRealtime, now); len(pts) != 1 {
		t.Fatalf("realtime = %+v", pts)
	}
	if _, err := Query(ctx, st, ring, "a", "bad", now); !errors.Is(err, ErrBadRange) {
		t.Fatalf("非法范围应返回 ErrBadRange，实际 %v", err)
	}
}

func TestForgetDropsBucketAndPending(t *testing.T) {
	r, st, _ := newRecorder(t, 200)
	r.Add("a", metric.Point{TS: 60, CPU: 10})
	r.Add("a", metric.Point{TS: 130, CPU: 20}) // 上一分钟进入待写队列
	r.Add("b", metric.Point{TS: 60, CPU: 30})
	r.Add("b", metric.Point{TS: 130, CPU: 40})
	r.Forget("a")
	if err := r.Flush(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	if pts := query(t, st, store.Metrics1m); len(pts) != 0 {
		t.Fatalf("Forget 后不应再写入该服务器的数据: %+v", pts)
	}
	pts, err := st.QueryMetrics(context.Background(), store.Metrics1m, "b", 0, 1<<40)
	if err != nil || len(pts) != 2 {
		t.Fatalf("其他服务器的数据应保留: %+v %v", pts, err)
	}
}
