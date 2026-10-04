package incident

import (
	"context"
	"encoding/json"
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/ruanun/simple-server-status/internal/dashboard/hub"
	"github.com/ruanun/simple-server-status/internal/dashboard/metric"
	"github.com/ruanun/simple-server-status/internal/dashboard/store"
)

// DefaultThreshold 离线持续多久才记录
const DefaultThreshold = time.Minute

// DefaultGrace 启动宽限期（门槛的 2 倍），从首轮检查开始计算。
// Dashboard 刚启动时 Agent 还没来得及重连（重连退避上限约 72 秒），最后上报时间停留在停机前，
// 宽限期内不新建离线事件，避免每次重启都给所有服务器多记一条；
// 宽限期结束后仍离线的照常新建，开始时间仍取最后上报时间（停机时段计入时长）。
const DefaultGrace = 2 * DefaultThreshold

// Source 当前服务器列表与检测规则（由 api.API 提供）
type Source interface {
	Servers() []store.Server
	EventSettings() store.EventSettings
}

// LoadDetail 负载事件的详情
type LoadDetail struct {
	Threshold int     `json:"threshold"`
	Minutes   int     `json:"minutes"`
	Peak      float64 `json:"peak"` // 事件期间窗口均值的最大值
}

// Detector 周期检测离线与高负载，结果交给 Recorder
type Detector struct {
	src       Source
	hub       *hub.Hub
	rec       *Recorder
	now       func() time.Time
	log       *slog.Logger
	threshold time.Duration
	interval  time.Duration
	grace     time.Duration

	mu      sync.Mutex
	started time.Time // 首轮检查的时间，宽限期由此开始
}

// NewDetector 创建 Detector；threshold ≤ 0 时使用 DefaultThreshold，检查间隔为门槛的一半，启动宽限期为门槛的 2 倍
func NewDetector(src Source, h *hub.Hub, rec *Recorder, now func() time.Time, log *slog.Logger, threshold time.Duration) *Detector {
	if threshold <= 0 {
		threshold = DefaultThreshold
	}
	return &Detector{src: src, hub: h, rec: rec, now: now, log: log, threshold: threshold, interval: threshold / 2, grace: 2 * threshold}
}

// Check 执行一轮检测；首轮先加载数据库中进行中的事件
func (d *Detector) Check(ctx context.Context) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.started.IsZero() {
		d.started = d.now()
	}
	if err := d.rec.Load(ctx); err != nil {
		d.log.Warn("读取进行中的事件失败", "err", err)
		return
	}
	nowT := d.now()
	inGrace := nowT.Sub(d.started) < d.grace
	cfg := d.src.EventSettings()
	seen := map[string]bool{}
	for _, srv := range d.src.Servers() {
		seen[srv.ID] = true
		live := d.hub.Get(srv.ID)
		d.checkOffline(ctx, srv, live, nowT, inGrace)
		if live.Online { // 离线期间负载事件保持原状态
			d.checkLoad(ctx, srv.ID, cfg, d.hub.Ring(srv.ID), nowT)
		}
	}
	d.rec.retain(seen)
}

func (d *Detector) checkOffline(ctx context.Context, srv store.Server, live hub.Live, nowT time.Time, inGrace bool) {
	last := live.LastReport
	if last == 0 {
		last = srv.LastSeen
	}
	_, open := d.rec.Ongoing(srv.ID, KindOffline)
	switch {
	case live.Online && open:
		if err := d.rec.Close(ctx, srv.ID, KindOffline, live.LastReport); err != nil {
			d.log.Warn("结束离线事件失败", "id", srv.ID, "err", err)
		}
	case !live.Online && !open && !inGrace && last > 0 && time.Duration(nowT.Unix()-last)*time.Second >= d.threshold:
		if err := d.rec.Open(ctx, srv.ID, KindOffline, last, nil); err != nil {
			d.log.Warn("新建离线事件失败", "id", srv.ID, "err", err)
		}
	}
}

func (d *Detector) checkLoad(ctx context.Context, id string, cfg store.EventSettings, ring []metric.Point, nowT time.Time) {
	avg, ok := averages(nowT, cfg.LoadMinutes, ring)
	if !ok {
		return
	}
	now := nowT.Unix()
	for _, m := range []struct {
		kind      string
		value     float64
		threshold int
	}{
		{KindLoadCPU, avg.CPU, cfg.LoadCPU},
		{KindLoadMem, avg.Mem, cfg.LoadMem},
		{KindLoadDisk, avg.Disk, cfg.LoadDisk},
	} {
		v := math.Round(m.value*10) / 10
		over := v >= float64(m.threshold)
		detail := LoadDetail{Threshold: m.threshold, Minutes: cfg.LoadMinutes, Peak: v}
		cur, open := d.rec.Ongoing(id, m.kind)
		var err error
		switch {
		case over && !open:
			err = d.rec.Open(ctx, id, m.kind, now, detail)
		case over && v > peakOf(cur.Event):
			err = d.rec.Update(ctx, id, m.kind, detail)
		case !over && open:
			err = d.rec.Close(ctx, id, m.kind, now)
		}
		if err != nil {
			d.log.Warn("记录负载事件失败", "id", id, "kind", m.kind, "err", err)
		}
	}
}

func peakOf(e store.Event) float64 {
	var ld LoadDetail
	_ = json.Unmarshal(e.Detail, &ld)
	return ld.Peak
}

// averages 计算最近 minutes 分钟内各项指标的平均值；点数不足 2 时 ok 为 false
func averages(now time.Time, minutes int, ring []metric.Point) (metric.Point, bool) {
	cut := now.Unix() - int64(minutes)*60
	var pts []metric.Point
	for _, p := range ring {
		if p.TS >= cut {
			pts = append(pts, p)
		}
	}
	if len(pts) < 2 {
		return metric.Point{}, false
	}
	return metric.Average(now.Unix(), pts), true
}

// Run 启动时检测一次，之后按间隔检测，直到 ctx 结束
func (d *Detector) Run(ctx context.Context) {
	d.Check(ctx)
	tk := time.NewTicker(d.interval)
	defer tk.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tk.C:
			d.Check(ctx)
		}
	}
}
