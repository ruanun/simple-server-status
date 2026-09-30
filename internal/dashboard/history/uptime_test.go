package history

import (
	"context"
	"testing"
	"time"

	"github.com/ruanun/simple-server-status/internal/dashboard/store"
)

type countStore struct {
	n        int
	table    string
	from, to int64
}

func (c *countStore) CountMetrics(_ context.Context, table, _ string, from, to int64) (int, error) {
	c.table, c.from, c.to = table, from, to
	return c.n, nil
}

func TestUptime(t *testing.T) {
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	long := now.Add(-30 * 24 * time.Hour).Unix()
	// 窗口终点排除当前未完成、上一个可能尚未落库的采样单位：24 小时共 1439 个单位，7 天共 1007 个单位
	end1m, end10m := now.Add(-time.Minute).Unix(), now.Add(-10*time.Minute).Unix()
	cases := []struct {
		name      string
		created   int64
		window    time.Duration
		n         int
		want      *float64
		wantTable string
		wantFrom  int64
		wantTo    int64
	}{
		{"满 24 小时", long, 24 * time.Hour, 1439, ptr(100), store.Metrics1m, now.Add(-24 * time.Hour).Unix(), end1m},
		{"一半", long, 24 * time.Hour, 720, ptr(50), store.Metrics1m, now.Add(-24 * time.Hour).Unix(), end1m},
		{"保留 1 位小数", long, 24 * time.Hour, 1438, ptr(99.9), store.Metrics1m, now.Add(-24 * time.Hour).Unix(), end1m},
		{"新服务器从创建时间算起", now.Add(-30 * time.Minute).Unix(), 24 * time.Hour, 29, ptr(100), store.Metrics1m, now.Add(-30 * time.Minute).Unix(), end1m},
		{"创建时间向上对齐到步长", now.Add(-30*time.Minute + 20*time.Second).Unix(), 24 * time.Hour, 28, ptr(100), store.Metrics1m, now.Add(-29 * time.Minute).Unix(), end1m},
		{"不足一个步长", now.Add(-30 * time.Second).Unix(), 24 * time.Hour, 0, nil, "", 0, 0},
		{"只有未落库的单位", now.Add(-90 * time.Second).Unix(), 24 * time.Hour, 0, nil, "", 0, 0},
		{"7 天用 10 分钟表", long, 7 * 24 * time.Hour, 1007, ptr(100), store.Metrics10m, now.Add(-7 * 24 * time.Hour).Unix(), end10m},
		{"超过 100 按 100", long, 24 * time.Hour, 1441, ptr(100), store.Metrics1m, now.Add(-24 * time.Hour).Unix(), end1m},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cs := &countStore{n: tc.n}
			got, err := Uptime(context.Background(), cs, "s", tc.created, now, tc.window)
			if err != nil {
				t.Fatal(err)
			}
			if (got == nil) != (tc.want == nil) || (got != nil && *got != *tc.want) {
				t.Fatalf("得到 %v，期望 %v", deref(got), deref(tc.want))
			}
			if tc.want != nil && (cs.table != tc.wantTable || cs.from != tc.wantFrom || cs.to != tc.wantTo) {
				t.Fatalf("查询参数错误 %s %d %d", cs.table, cs.from, cs.to)
			}
		})
	}
}

// tsStore 按真实时间戳计数（ts >= from 且 ts < to，与 store.CountMetrics 一致）
type tsStore struct{ ts []int64 }

func (s *tsStore) CountMetrics(_ context.Context, _, _ string, from, to int64) (int, error) {
	n := 0
	for _, v := range s.ts {
		if v >= from && v < to {
			n++
		}
	}
	return n, nil
}

// 一直在线的服务器：按 Recorder 真实写入时序（每个采样单位起点为 ts，只写到已完成的单位，
// 最近一个完成的单位可能尚未落库）构造数据，在线率应为 100
func TestUptimeAlwaysOnline(t *testing.T) {
	now := time.Date(2026, 9, 29, 12, 0, 30, 0, time.UTC) // 未对齐步长
	cases := []struct {
		name    string
		created int64
		window  time.Duration
		step    int64
	}{
		{"24 小时", now.Add(-30 * 24 * time.Hour).Unix(), 24 * time.Hour, 60},
		{"新服务器", now.Add(-40*time.Minute - 10*time.Second).Unix(), 24 * time.Hour, 60},
		{"7 天", now.Add(-30 * 24 * time.Hour).Unix(), 7 * 24 * time.Hour, 600},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// 已落库：从创建时间所在单位起，到上上个单位为止（上一个单位尚未写入）
			last := now.Unix()/tc.step*tc.step - 2*tc.step
			s := &tsStore{}
			for ts := tc.created / tc.step * tc.step; ts <= last; ts += tc.step {
				s.ts = append(s.ts, ts)
			}
			got, err := Uptime(context.Background(), s, "s", tc.created, now, tc.window)
			if err != nil {
				t.Fatal(err)
			}
			if got == nil || *got != 100 {
				t.Fatalf("一直在线应为 100，得到 %v", deref(got))
			}
		})
	}
}

func ptr(v float64) *float64 { return &v }

func deref(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}
