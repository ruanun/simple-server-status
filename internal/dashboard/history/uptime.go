package history

import (
	"context"
	"math"
	"time"

	"github.com/ruanun/simple-server-status/internal/dashboard/store"
)

// UptimeStore 计算在线率所需的存储接口（store.Store 实现）
type UptimeStore interface {
	CountMetrics(ctx context.Context, table, serverID string, from, to int64) (int, error)
}

// Uptime 计算 window 内的在线率（百分比，保留 1 位小数，上限 100）：有历史数据的采样数 ÷ 窗口内的采样数。
// 窗口不超过 24 小时使用 1 分钟数据，否则使用 10 分钟数据；窗口起点不早于服务器创建时间并向上对齐到步长，
// 终点对齐到步长并排除最近一个可能尚未落库的单位；
// 窗口不足一个采样步长时返回 nil
func Uptime(ctx context.Context, st UptimeStore, id string, createdAt int64, now time.Time, window time.Duration) (*float64, error) {
	table, step := store.Metrics1m, int64(60)
	if window > 24*time.Hour {
		table, step = store.Metrics10m, 600
	}
	// 只统计已完成并落库的采样单位：排除当前未完成、上一个可能尚未落库的单位
	end := now.Unix()/step*step - step
	start := max(now.Unix()-int64(window/time.Second), createdAt)
	start = (start + step - 1) / step * step // 向上对齐到采样单位起点
	units := (end - start) / step
	if units < 1 {
		return nil, nil
	}
	n, err := st.CountMetrics(ctx, table, id, start, end)
	if err != nil {
		return nil, err
	}
	pct := math.Min(100, math.Round(float64(n)/float64(units)*1000)/10)
	return &pct, nil
}
