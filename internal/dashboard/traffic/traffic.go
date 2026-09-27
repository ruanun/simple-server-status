// Package traffic 按计费周期累计每台服务器的流量。
package traffic

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/ruanun/simple-server-status/internal/dashboard/store"
)

// Store 流量存储（store.Store 实现）
type Store interface {
	GetTraffic(ctx context.Context, serverID, period string) (store.TrafficRow, error)
	SaveTraffic(ctx context.Context, rows []store.TrafficRow) error
}

// PeriodStart 返回 t 所在计费周期的起点（resetDay 日 00:00，使用 t 的时区）；resetDay 超出 1–28 时按 1 处理
func PeriodStart(t time.Time, resetDay int) time.Time {
	if resetDay < 1 || resetDay > 28 {
		resetDay = 1
	}
	y, m, d := t.Date()
	if d < resetDay {
		m-- // time.Date 会把 0 月规范为上一年 12 月
	}
	return time.Date(y, m, resetDay, 0, 0, 0, 0, t.Location())
}

func periodKey(t time.Time, resetDay int) string {
	return PeriodStart(t, resetDay).Format("2006-01-02")
}

type entry struct {
	row   store.TrafficRow
	dirty bool
	// filterID 当前基线对应的 Agent 网卡过滤器摘要；为空表示尚未确定（刚启动或刚从库中加载）
	filterID string
}

// Accumulator 在内存中累计流量，定期落库
type Accumulator struct {
	st  Store
	now func() time.Time
	mu  sync.Mutex
	m   map[string]*entry
}

// New 创建 Accumulator
func New(st Store, now func() time.Time) *Accumulator {
	return &Accumulator{st: st, now: now, m: map[string]*entry{}}
}

// load 取得 id 在 period 的记录（调用方需持有锁）。
// 内存中是旧周期时先保存旧记录；新周期没有记录时继承旧记录的计数基线及其过滤器摘要
func (a *Accumulator) load(ctx context.Context, id, period string) (*entry, error) {
	old := a.m[id]
	if old != nil && old.row.Period == period {
		return old, nil
	}
	if old != nil && old.dirty {
		if err := a.st.SaveTraffic(ctx, []store.TrafficRow{old.row}); err != nil {
			return nil, err
		}
		old.dirty = false
	}
	e := &entry{}
	row, err := a.st.GetTraffic(ctx, id, period)
	if errors.Is(err, store.ErrNotFound) {
		row = store.TrafficRow{ServerID: id, Period: period, LastIn: -1, LastOut: -1}
		if old != nil {
			row.LastIn, row.LastOut = old.row.LastIn, old.row.LastOut
			e.filterID = old.filterID
		}
	} else if err != nil {
		return nil, err
	}
	e.row = row
	a.m[id] = e
	return e, nil
}

// Add 根据 Agent 上报的累计计数更新当前周期流量。
// filterID 为采集该计数时 Agent 使用的网卡过滤器摘要：与条目记录的不同时说明统计口径已变化，
// 此时只以本次计数重建基线、不累加，避免把口径变化造成的差值计入流量；条目尚未记录时直接采用
func (a *Accumulator) Add(ctx context.Context, id string, resetDay int, filterID string, inTotal, outTotal uint64) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	e, err := a.load(ctx, id, periodKey(a.now(), resetDay))
	if err != nil {
		return err
	}
	in, out := toInt64(inTotal), toInt64(outTotal)
	changed := e.filterID != "" && e.filterID != filterID
	e.filterID = filterID
	if e.row.LastIn >= 0 && !changed {
		e.row.In += delta(e.row.LastIn, in)
		e.row.Out += delta(e.row.LastOut, out)
	}
	e.row.LastIn, e.row.LastOut = in, out
	e.dirty = true
	return nil
}

// Usage 返回当前周期的入站、出站流量与周期起始日
func (a *Accumulator) Usage(ctx context.Context, id string, resetDay int) (in, out int64, period string, err error) {
	period = periodKey(a.now(), resetDay)
	a.mu.Lock()
	defer a.mu.Unlock()
	e, err := a.load(ctx, id, period)
	if err != nil {
		return 0, 0, period, err
	}
	return e.row.In, e.row.Out, period, nil
}

// Flush 保存所有有变化的记录
func (a *Accumulator) Flush(ctx context.Context) error {
	a.mu.Lock()
	rows := []store.TrafficRow{}
	for _, e := range a.m {
		if e.dirty {
			rows = append(rows, e.row)
			e.dirty = false
		}
	}
	a.mu.Unlock()
	if len(rows) == 0 {
		return nil
	}
	if err := a.st.SaveTraffic(ctx, rows); err != nil {
		a.mu.Lock()
		for _, r := range rows {
			if e := a.m[r.ServerID]; e != nil && e.row.Period == r.Period {
				e.dirty = true
			}
		}
		a.mu.Unlock()
		return err
	}
	return nil
}

// Forget 丢弃某台服务器的内存记录（删除服务器时调用）
func (a *Accumulator) Forget(id string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.m, id)
}

// Run 每分钟保存一次；ctx 结束时最后保存一次后返回
func (a *Accumulator) Run(ctx context.Context, log *slog.Logger) {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			fctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := a.Flush(fctx); err != nil {
				log.Error("退出时保存流量数据失败", "err", err)
			}
			cancel()
			return
		case <-t.C:
			if err := a.Flush(ctx); err != nil {
				log.Error("保存流量数据失败", "err", err)
			}
		}
	}
}

// delta 计算计数增量；计数器回退（重启归零）时以新值作为增量
func delta(last, cur int64) int64 {
	if cur < last {
		return cur
	}
	return cur - last
}

func toInt64(v uint64) int64 {
	if v > math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(v)
}
