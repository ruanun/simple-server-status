// Package notify 把事件与提醒通过 Webhook、Telegram 推送出去。
// 事件（离线、高负载、重启、IP 变化）由 incident 检测并记录，本包只决定是否推送；
// 提醒（即将到期、流量达到额度比例）由本包按配置判定。
package notify

import (
	"encoding/json"
	"math"
	"strconv"
	"time"

	"github.com/ruanun/simple-server-status/internal/dashboard/incident"
	"github.com/ruanun/simple-server-status/internal/dashboard/store"
)

// 通知类型（同时作为 Webhook 的 event 字段）
const (
	KindOffline       = "offline"
	KindRecovered     = "recovered"
	KindLoad          = "load"
	KindLoadRecovered = "load_recovered"
	KindReboot        = "reboot"
	KindIPChange      = "ip_change"
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

// LoadValue 负载指标；Metric 为 cpu、mem 或 disk
type LoadValue struct {
	Metric string
	Value  float64
}

// Event 一条待发送的通知；Title、Message 由 Render 填充
type Event struct {
	Kind       string
	ServerID   string
	ServerName string
	EventID    int64 // 关联的事件，0 表示提醒或测试通知
	Time       int64
	Minutes    int64                // offline：已离线分钟数；recovered：离线时长；load / load_recovered：统计窗口分钟数
	Load       LoadValue            // load：超过阈值的指标与峰值；load_recovered：恢复的指标
	IPs        map[string][2]string // ip_change：变化的地址，键为 ipv4、ipv6，值为 [旧, 新]
	Days       int                  // expire：剩余天数（≤0 表示已过期）
	ExpireAt   int64                // expire：到期时间
	Percent    float64              // traffic：已用百分比
	Used       int64                // traffic：已用字节
	Limit      int64                // traffic：配额字节
	Title      string
	Message    string
	Mark       *Mark // 到期 / 流量等一次性提醒：投递成功后需记录的去重键
}

// Observation 一台服务器提醒所需的数据
type Observation struct {
	ID           string
	Name         string
	Muted        bool
	ExpireAt     *int64
	TrafficUsed  int64
	TrafficLimit *int64
	Period       string // 当前计费周期（流量提醒的去重键）
}

// Mark 需要记录为已发送的一次性提醒
type Mark struct {
	Rule string
	Key  string
}

// Reminders 按规则评估一台服务器的到期与流量提醒，以 sent 去重；静音的服务器不提醒
func Reminders(now time.Time, cfg store.NotifySettings, o Observation, sent func(rule, key string) bool) []Event {
	if o.Muted {
		return nil
	}
	var events []Event
	emit := func(kind string, fill func(e *Event)) {
		e := Event{Kind: kind, ServerID: o.ID, ServerName: o.Name, Time: now.Unix()}
		fill(&e)
		events = append(events, e)
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

// enabled 事件类型的推送开关是否开启
func enabled(cfg store.NotifySettings, kind string) bool {
	if _, load := incident.LoadMetric[kind]; load {
		return cfg.LoadEnabled
	}
	switch kind {
	case incident.KindOffline:
		return cfg.OfflineEnabled
	case incident.KindReboot:
		return cfg.RebootEnabled
	case incident.KindIPChange:
		return cfg.IPChangeEnabled
	}
	return false
}

// fromIncident 把事件转换为通知；recovered 为 true 时生成对应的恢复通知
func fromIncident(ev store.Event, srv store.Server, now time.Time, recovered bool) Event {
	e := Event{ServerID: ev.ServerID, ServerName: srv.Name, EventID: ev.ID, Time: now.Unix()}
	metric, load := incident.LoadMetric[ev.Kind]
	switch {
	case ev.Kind == incident.KindOffline && recovered:
		e.Kind, e.Minutes = KindRecovered, (*ev.EndAt-ev.StartAt)/60
	case ev.Kind == incident.KindOffline:
		e.Kind, e.Minutes = KindOffline, (now.Unix()-ev.StartAt)/60
	case load:
		var d incident.LoadDetail
		_ = json.Unmarshal(ev.Detail, &d)
		e.Kind, e.Minutes, e.Load = KindLoad, int64(d.Minutes), LoadValue{Metric: metric, Value: d.Peak}
		if recovered {
			e.Kind = KindLoadRecovered
		}
	case ev.Kind == incident.KindReboot:
		e.Kind = KindReboot
	case ev.Kind == incident.KindIPChange:
		_ = json.Unmarshal(ev.Detail, &e.IPs)
		e.Kind = KindIPChange
	}
	return e
}
