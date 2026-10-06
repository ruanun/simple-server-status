package notify

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/ruanun/simple-server-status/internal/dashboard/incident"
	"github.com/ruanun/simple-server-status/internal/dashboard/store"
)

var t0 = time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)

func never(string, string) bool { return false }

func kinds(es []Event) []string {
	out := []string{}
	for _, e := range es {
		out = append(out, e.Kind)
	}
	return out
}

func eq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestRemindersMuted(t *testing.T) {
	cfg := store.DefaultNotifySettings()
	exp := t0.Add(24 * time.Hour).Unix()
	if es := Reminders(t0, cfg, Observation{ID: "s", Muted: true, ExpireAt: &exp}, never); len(es) != 0 {
		t.Fatalf("静音服务器不提醒 %v", kinds(es))
	}
}

func TestExpireAndTrafficOnce(t *testing.T) {
	cfg := store.DefaultNotifySettings()
	sentKeys := map[string]bool{}
	sent := func(rule, key string) bool { return sentKeys[rule+":"+key] }
	exp := t0.Add(3*24*time.Hour + time.Hour).Unix() // 剩余 4 天（向上取整）
	limit := int64(1000)
	o := Observation{ID: "s", ExpireAt: &exp, TrafficUsed: 950, TrafficLimit: &limit, Period: "2026-09-01"}
	es := Reminders(t0, cfg, o, sent)
	if !eq(kinds(es), []string{KindExpire, KindTraffic}) || es[0].Days != 4 || es[1].Percent != 95 || len(marksOf(es)) != 2 {
		t.Fatalf("应提醒到期与流量 %+v %+v", es, marksOf(es))
	}
	for _, m := range marksOf(es) {
		sentKeys[m.Rule+":"+m.Key] = true
	}
	if es := Reminders(t0.Add(time.Hour), cfg, o, sent); len(es) != 0 {
		t.Fatalf("同一到期日、同一周期只提醒一次 %v", kinds(es))
	}
	exp2 := exp + 86400 // 续费改了到期日
	o.ExpireAt = &exp2
	if es := Reminders(t0, cfg, o, sent); !eq(kinds(es), []string{KindExpire}) {
		t.Fatalf("修改到期日后应重新提醒 %v", kinds(es))
	}
	o.ExpireAt = nil
	o.TrafficUsed = 899
	o.Period = "2026-10-01"
	if es := Reminders(t0, cfg, o, sent); len(es) != 0 {
		t.Fatalf("未达阈值不提醒 %v", kinds(es))
	}
}

func TestExpireSkipsLongExpired(t *testing.T) {
	cfg := store.DefaultNotifySettings()
	old := t0.Add(-31 * 24 * time.Hour).Unix()
	o := Observation{ID: "s", ExpireAt: &old}
	if es := Reminders(t0, cfg, o, never); len(es) != 0 {
		t.Fatalf("过期超过 30 天不提醒 %v", kinds(es))
	}
	recent := t0.Add(-30 * 24 * time.Hour).Unix()
	o.ExpireAt = &recent
	if es := Reminders(t0, cfg, o, never); !eq(kinds(es), []string{KindExpire}) || es[0].Days != -30 {
		t.Fatalf("过期 30 天内应提醒 %+v", es)
	}
}

func TestFromIncident(t *testing.T) {
	srv := store.Server{ID: "s", Name: "hk"}
	end := t0.Unix()
	start := t0.Add(-75 * time.Minute).Unix()
	offline := store.Event{ID: 1, ServerID: "s", Kind: incident.KindOffline, StartAt: start}
	if e := fromIncident(offline, srv, t0, false); e.Kind != KindOffline || e.Minutes != 75 || e.EventID != 1 || e.ServerName != "hk" {
		t.Fatalf("离线 %+v", e)
	}
	offline.EndAt = &end
	if e := fromIncident(offline, srv, t0, true); e.Kind != KindRecovered || e.Minutes != 75 {
		t.Fatalf("恢复 %+v", e)
	}
	detail, _ := json.Marshal(incident.LoadDetail{Threshold: 90, Minutes: 5, Peak: 97.5})
	load := store.Event{ID: 2, ServerID: "s", Kind: incident.KindLoadMem, StartAt: start, Detail: detail}
	if e := fromIncident(load, srv, t0, false); e.Kind != KindLoad || e.Minutes != 5 || e.Load != (LoadValue{"mem", 97.5}) {
		t.Fatalf("高负载 %+v", e)
	}
	if e := fromIncident(load, srv, t0, true); e.Kind != KindLoadRecovered || e.Load.Metric != "mem" {
		t.Fatalf("负载恢复 %+v", e)
	}
	reboot := store.Event{ServerID: "s", Kind: incident.KindReboot}
	if e := fromIncident(reboot, srv, t0, false); e.Kind != KindReboot || e.ServerName != "hk" {
		t.Fatalf("重启 %+v", e)
	}
	ip := store.Event{ServerID: "s", Kind: incident.KindIPChange, Detail: json.RawMessage(`{"ipv4":["1.1.1.1","2.2.2.2"]}`)}
	if e := fromIncident(ip, srv, t0, false); e.Kind != KindIPChange || e.IPs["ipv4"] != [2]string{"1.1.1.1", "2.2.2.2"} {
		t.Fatalf("IP 变化 %+v", e)
	}
}

func marksOf(es []Event) []Mark {
	out := []Mark{}
	for _, e := range es {
		if e.Mark != nil {
			out = append(out, *e.Mark)
		}
	}
	return out
}
