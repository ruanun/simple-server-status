package store

import (
	"context"
	"errors"
	"testing"

	"github.com/ruanun/simple-server-status/internal/dashboard/metric"
)

func TestMetricsInsertQueryRollupDelete(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	rows := []MetricRow{
		{ServerID: "a", Point: metric.Point{TS: 600, CPU: 10, NetIn: 100}},
		{ServerID: "a", Point: metric.Point{TS: 660, CPU: 30, NetIn: 300}},
		{ServerID: "b", Point: metric.Point{TS: 600, CPU: 50}},
	}
	if err := s.InsertMetrics(ctx, Metrics1m, rows); err != nil {
		t.Fatal(err)
	}
	// 同一时间点重复写入应覆盖
	if err := s.InsertMetrics(ctx, Metrics1m, []MetricRow{{ServerID: "a", Point: metric.Point{TS: 660, CPU: 30, NetIn: 300}}}); err != nil {
		t.Fatal(err)
	}
	pts, err := s.QueryMetrics(ctx, Metrics1m, "a", 0, 1000)
	if err != nil || len(pts) != 2 || pts[0].TS != 600 {
		t.Fatalf("QueryMetrics = %+v, %v", pts, err)
	}
	empty, _ := s.QueryMetrics(ctx, Metrics1m, "none", 0, 1000)
	if empty == nil || len(empty) != 0 {
		t.Fatal("无数据时应返回 []")
	}

	if err := s.Rollup10m(ctx, 600, 1200); err != nil {
		t.Fatal(err)
	}
	agg, _ := s.QueryMetrics(ctx, Metrics10m, "a", 0, 2000)
	if len(agg) != 1 || agg[0].TS != 600 || agg[0].CPU != 20 || agg[0].NetIn != 200 {
		t.Fatalf("Rollup10m 结果错误: %+v", agg)
	}

	n, err := s.DeleteMetricsBefore(ctx, Metrics1m, 650)
	if err != nil || n != 2 {
		t.Fatalf("DeleteMetricsBefore = %d, %v", n, err)
	}
	if err := s.InsertMetrics(ctx, "users", nil); err == nil {
		t.Fatal("非法表名应报错")
	}
}

func TestDeleteServerRemovesMetricsAndTraffic(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	sv := Server{Name: "a"}
	if err := s.CreateServer(ctx, &sv); err != nil {
		t.Fatal(err)
	}
	_ = s.InsertMetrics(ctx, Metrics1m, []MetricRow{{ServerID: sv.ID, Point: metric.Point{TS: 60}}})
	_ = s.SaveTraffic(ctx, []TrafficRow{{ServerID: sv.ID, Period: "2026-03-01", In: 1}})
	if err := s.DeleteServer(ctx, sv.ID); err != nil {
		t.Fatal(err)
	}
	pts, _ := s.QueryMetrics(ctx, Metrics1m, sv.ID, 0, 1000)
	if len(pts) != 0 {
		t.Fatal("删除服务器后历史数据应被清除")
	}
	if _, err := s.GetTraffic(ctx, sv.ID, "2026-03-01"); !errors.Is(err, ErrNotFound) {
		t.Fatal("删除服务器后流量数据应被清除")
	}
}

func TestTrafficGetSave(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if _, err := s.GetTraffic(ctx, "a", "2026-03-01"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("不存在时应 ErrNotFound，实际 %v", err)
	}
	row := TrafficRow{ServerID: "a", Period: "2026-03-01", In: 10, Out: 20, LastIn: 100, LastOut: 200}
	if err := s.SaveTraffic(ctx, []TrafficRow{row}); err != nil {
		t.Fatal(err)
	}
	row.In = 15
	if err := s.SaveTraffic(ctx, []TrafficRow{row}); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetTraffic(ctx, "a", "2026-03-01")
	if err != nil || got != row {
		t.Fatalf("GetTraffic = %+v, %v", got, err)
	}
}
