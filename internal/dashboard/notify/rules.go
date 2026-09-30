// Package notify 按规则检测服务器状态变化，并通过 Webhook、Telegram 发送通知。
package notify

import (
	"math"
	"strconv"
	"time"

	"github.com/ruanun/simple-server-status/internal/dashboard/metric"
	"github.com/ruanun/simple-server-status/internal/dashboard/store"
)

// 事件类型（同时作为 Webhook 的 event 字段）
const (
	KindOffline       = "offline"
	KindRecovered     = "recovered"
	KindLoad          = "load"
	KindLoadRecovered = "load_recovered"
	KindExpire        = "expire"
	KindTraffic       = "traffic"
	KindTest          = "test"
)

// expireGraceDays 已过期超过该天数的服务器不再提醒到期
const expireGraceDays = 30

// 一次性提醒的规则名（notify_state.rule）
const (
	RuleExpire  = "expire"
	RuleTraffic = "traffic"
)

// LoadValue 超过阈值的负载指标；Metric 为 cpu、mem 或 disk
type LoadValue struct {
	Metric string
	Value  float64
}

// Event 一条待发送的通知；Title、Message 由 Render 填充
type Event struct {
	Kind       string
	ServerID   string
	ServerName string
	Time       int64
	Minutes    int64       // offline：已离线分钟数；recovered：离线时长；load / load_recovered：统计窗口分钟数
	Loads      []LoadValue // load：超过阈值的指标
	Days       int         // expire：剩余天数（≤0 表示已过期）
	ExpireAt   int64       // expire：到期时间
	Percent    float64     // traffic：已用百分比
	Used       int64       // traffic：已用字节
	Limit      int64       // traffic：配额字节
	Title      string
	Message    string
	Mark       *Mark // 到期 / 流量等一次性提醒：投递成功后需记录的去重键
}

// Observation 一台服务器当前的观测数据
type Observation struct {
	ID           string
	Name         string
	Muted        bool
	Online       bool
	LastSeen     int64 // 最后一次上报的 Unix 秒，0 表示从未上报
	Ring         []metric.Point
	ExpireAt     *int64
	TrafficUsed  int64
	TrafficLimit *int64
	Period       string // 当前计费周期（流量提醒的去重键）
}

// State 一台服务器的离线、高负载告警状态，在内存中跨轮次保存
type State struct {
	Offline      bool
	OfflineSince int64
	Load         bool
}

// Mark 需要记录为已发送的一次性提醒
type Mark struct {
	Rule string
	Key  string
}

// Evaluate 按规则评估一台服务器，更新 st 并返回待发送的事件（一次性提醒携带 Mark）。
// baseline 为 true 时（启动后的基线期）离线、高负载只建立状态、不产生事件；到期与流量提醒以 sent 去重，不受 baseline 影响。
// 静音的服务器按基线方式更新离线、高负载状态（避免取消静音后补发陈旧的通知），且不产生一次性提醒
func Evaluate(now time.Time, cfg store.NotifySettings, o Observation, st *State, baseline bool, sent func(rule, key string) bool) []Event {
	if o.Muted {
		baseline = true
	}
	var events []Event
	emit := func(kind string, fill func(e *Event)) {
		e := Event{Kind: kind, ServerID: o.ID, ServerName: o.Name, Time: now.Unix()}
		fill(&e)
		events = append(events, e)
	}

	// 离线 / 恢复
	switch {
	case !cfg.OfflineEnabled:
		st.Offline = false
	case o.LastSeen == 0:
	case !o.Online && !st.Offline && now.Unix()-o.LastSeen >= int64(cfg.OfflineMinutes)*60:
		st.Offline, st.OfflineSince = true, o.LastSeen
		if !baseline {
			emit(KindOffline, func(e *Event) { e.Minutes = (now.Unix() - o.LastSeen) / 60 })
		}
	case o.Online && st.Offline:
		st.Offline = false
		if !baseline {
			emit(KindRecovered, func(e *Event) { e.Minutes = (now.Unix() - st.OfflineSince) / 60 })
		}
	}

	// 高负载 / 恢复（离线期间保持原状态）
	if !cfg.LoadEnabled {
		st.Load = false
	} else if o.Online {
		if over, ok := loadOver(now, cfg, o.Ring); ok {
			switch {
			case len(over) > 0 && !st.Load:
				st.Load = true
				if !baseline {
					emit(KindLoad, func(e *Event) { e.Minutes, e.Loads = int64(cfg.LoadMinutes), over })
				}
			case len(over) == 0 && st.Load:
				st.Load = false
				if !baseline {
					emit(KindLoadRecovered, func(e *Event) { e.Minutes = int64(cfg.LoadMinutes) })
				}
			}
		}
	}

	if o.Muted {
		return events
	}

	// 即将到期：每个到期日只提醒一次；过期超过 expireGraceDays 天的不再提醒（与后台总览一致）
	if cfg.ExpireEnabled && o.ExpireAt != nil {
		days := int(math.Ceil(float64(*o.ExpireAt-now.Unix()) / 86400))
		key := strconv.FormatInt(*o.ExpireAt, 10)
		if days <= cfg.ExpireDays && days >= -expireGraceDays && !sent(RuleExpire, key) {
			emit(KindExpire, func(e *Event) {
				e.Days, e.ExpireAt = days, *o.ExpireAt
				e.Mark = &Mark{Rule: RuleExpire, Key: key}
			})
		}
	}

	// 月流量：每个计费周期只提醒一次
	if cfg.TrafficEnabled && o.TrafficLimit != nil && *o.TrafficLimit > 0 {
		pct := float64(o.TrafficUsed) / float64(*o.TrafficLimit) * 100
		if pct >= float64(cfg.TrafficPercent) && !sent(RuleTraffic, o.Period) {
			emit(KindTraffic, func(e *Event) {
				e.Percent, e.Used, e.Limit = math.Round(pct*10)/10, o.TrafficUsed, *o.TrafficLimit
				e.Mark = &Mark{Rule: RuleTraffic, Key: o.Period}
			})
		}
	}
	return events
}

// loadOver 计算最近 LoadMinutes 分钟内各项平均值，返回超过阈值的指标；点数不足 2 时 ok 为 false
func loadOver(now time.Time, cfg store.NotifySettings, ring []metric.Point) ([]LoadValue, bool) {
	cut := now.Unix() - int64(cfg.LoadMinutes)*60
	var n int
	var cpu, mem, disk float64
	for _, p := range ring {
		if p.TS < cut {
			continue
		}
		n++
		cpu += p.CPU
		mem += p.Mem
		disk += p.Disk
	}
	if n < 2 {
		return nil, false
	}
	over := []LoadValue{}
	for _, v := range []struct {
		metric    string
		avg       float64
		threshold int
	}{
		{"cpu", cpu / float64(n), cfg.LoadCPU},
		{"mem", mem / float64(n), cfg.LoadMem},
		{"disk", disk / float64(n), cfg.LoadDisk},
	} {
		if v.avg >= float64(v.threshold) {
			over = append(over, LoadValue{Metric: v.metric, Value: math.Round(v.avg*10) / 10})
		}
	}
	return over, true
}
