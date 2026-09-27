// Package history 把实时指标降采样写入数据库，并按时间范围查询。
package history

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/ruanun/simple-server-status/internal/dashboard/metric"
	"github.com/ruanun/simple-server-status/internal/dashboard/store"
)

// MetricStore 历史数据存储（store.Store 实现）
type MetricStore interface {
	InsertMetrics(ctx context.Context, table string, rows []store.MetricRow) error
	QueryMetrics(ctx context.Context, table, serverID string, from, to int64) ([]metric.Point, error)
	Rollup10m(ctx context.Context, from, to int64) error
	DeleteMetricsBefore(ctx context.Context, table string, ts int64) (int64, error)
}

// maxPending 写库失败时最多暂存的分钟数据条数
const maxPending = 10000

type bucket struct {
	start int64
	pts   []metric.Point
}

// Recorder 把每台服务器的上报聚合为 1 分钟数据，并定期生成 10 分钟数据、清理过期数据
type Recorder struct {
	st  MetricStore
	now func() time.Time
	log *slog.Logger

	mu      sync.Mutex
	cur     map[string]*bucket
	pending []store.MetricRow

	lastRollup  int64
	lastCleanup time.Time
}

// NewRecorder 创建 Recorder
func NewRecorder(st MetricStore, now func() time.Time, log *slog.Logger) *Recorder {
	return &Recorder{st: st, now: now, log: log, cur: map[string]*bucket{}}
}

// Add 把一个点累积到所属分钟；分钟切换时把上一分钟的平均值放入待写队列
func (r *Recorder) Add(id string, p metric.Point) {
	start := p.TS - p.TS%60
	r.mu.Lock()
	defer r.mu.Unlock()
	b := r.cur[id]
	if b != nil && b.start != start {
		r.pending = append(r.pending, store.MetricRow{ServerID: id, Point: metric.Average(b.start, b.pts)})
		b = nil
	}
	if b == nil {
		b = &bucket{start: start}
		r.cur[id] = b
	}
	b.pts = append(b.pts, p)
}

// Forget 丢弃某台服务器当前分钟的累积数据与待写队列中的行（删除服务器时调用）
func (r *Recorder) Forget(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.cur, id)
	kept := r.pending[:0]
	for _, row := range r.pending {
		if row.ServerID != id {
			kept = append(kept, row)
		}
	}
	r.pending = kept
}

// Flush 写入已结束的分钟；all 为 true 时连同当前未结束的分钟一起写入（退出时使用）
func (r *Recorder) Flush(ctx context.Context, all bool) error {
	r.mu.Lock()
	rows := r.pending
	r.pending = nil
	now := r.now().Unix()
	curMin := now - now%60
	for id, b := range r.cur {
		if all || b.start < curMin {
			rows = append(rows, store.MetricRow{ServerID: id, Point: metric.Average(b.start, b.pts)})
			delete(r.cur, id)
		}
	}
	r.mu.Unlock()
	if len(rows) == 0 {
		return nil
	}
	if err := r.st.InsertMetrics(ctx, store.Metrics1m, rows); err != nil {
		r.mu.Lock()
		r.pending = append(rows, r.pending...)
		if n := len(r.pending); n > maxPending {
			r.pending = r.pending[n-maxPending:]
		}
		r.mu.Unlock()
		return err
	}
	return nil
}

// Tick 每分钟调用：写入 1 分钟数据、生成上一个 10 分钟窗口的数据、每小时清理过期数据
func (r *Recorder) Tick(ctx context.Context) {
	if err := r.Flush(ctx, false); err != nil {
		r.log.Error("写入 1 分钟数据失败", "err", err)
	}
	now := r.now()
	end := now.Unix() - now.Unix()%600
	if end > r.lastRollup {
		if err := r.st.Rollup10m(ctx, end-600, end); err != nil {
			r.log.Error("生成 10 分钟数据失败", "err", err)
		} else {
			r.lastRollup = end
		}
	}
	if now.Sub(r.lastCleanup) >= time.Hour {
		if _, err := r.st.DeleteMetricsBefore(ctx, store.Metrics1m, now.Add(-24*time.Hour).Unix()); err != nil {
			r.log.Error("清理 1 分钟数据失败", "err", err)
		}
		if _, err := r.st.DeleteMetricsBefore(ctx, store.Metrics10m, now.Add(-7*24*time.Hour).Unix()); err != nil {
			r.log.Error("清理 10 分钟数据失败", "err", err)
		}
		r.lastCleanup = now
	}
}

// Run 每分钟执行 Tick；ctx 结束时写入全部剩余数据后返回
func (r *Recorder) Run(ctx context.Context) {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			fctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := r.Flush(fctx, true); err != nil {
				r.log.Error("退出时写入历史数据失败", "err", err)
			}
			cancel()
			return
		case <-t.C:
			r.Tick(ctx)
		}
	}
}

// 支持的时间范围
const (
	RangeRealtime = "realtime"
	Range1h       = "1h"
	Range6h       = "6h"
	Range24h      = "24h"
	Range7d       = "7d"
)

// ErrBadRange 不支持的时间范围
var ErrBadRange = errors.New("不支持的时间范围")

// Query 按范围返回某台服务器的历史点
func Query(ctx context.Context, st MetricStore, ring func(id string) []metric.Point, id, rng string, now time.Time) ([]metric.Point, error) {
	end := now.Unix() + 1
	switch rng {
	case RangeRealtime:
		return ring(id), nil
	case Range1h:
		return st.QueryMetrics(ctx, store.Metrics1m, id, end-3600, end)
	case Range6h:
		return st.QueryMetrics(ctx, store.Metrics1m, id, end-6*3600, end)
	case Range24h:
		pts, err := st.QueryMetrics(ctx, store.Metrics1m, id, end-24*3600, end)
		if err != nil {
			return nil, err
		}
		return metric.Downsample(pts, 300), nil
	case Range7d:
		return st.QueryMetrics(ctx, store.Metrics10m, id, end-7*24*3600, end)
	}
	return nil, ErrBadRange
}
