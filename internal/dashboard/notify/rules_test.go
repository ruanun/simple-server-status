package notify

import (
	"testing"
	"time"

	"github.com/ruanun/simple-server-status/internal/dashboard/metric"
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

func ring(now time.Time, cpu float64, n int) []metric.Point {
	pts := []metric.Point{}
	for i := 0; i < n; i++ {
		pts = append(pts, metric.Point{TS: now.Add(-time.Duration(i) * 30 * time.Second).Unix(), CPU: cpu, Mem: 10, Disk: 10})
	}
	return pts
}

func TestOfflineAndRecovery(t *testing.T) {
	cfg := store.DefaultNotifySettings()
	st := &State{}
	o := Observation{ID: "s", Name: "hk", Online: false, LastSeen: t0.Add(-2 * time.Minute).Unix()}
	if es, _ := Evaluate(t0, cfg, o, st, false, never); len(es) != 0 {
		t.Fatalf("离线未满 3 分钟不应通知 %v", kinds(es))
	}
	o.LastSeen = t0.Add(-4 * time.Minute).Unix()
	es, _ := Evaluate(t0, cfg, o, st, false, never)
	if !eq(kinds(es), []string{KindOffline}) || es[0].Minutes != 4 || !st.Offline {
		t.Fatalf("应发送离线通知 %+v", es)
	}
	if es, _ := Evaluate(t0.Add(time.Minute), cfg, o, st, false, never); len(es) != 0 {
		t.Fatalf("已告警不应重复 %v", kinds(es))
	}
	o.Online = true
	es, _ = Evaluate(t0.Add(10*time.Minute), cfg, o, st, false, never)
	if !eq(kinds(es), []string{KindRecovered}) || es[0].Minutes != 14 || st.Offline {
		t.Fatalf("应发送恢复通知 %+v", es)
	}
}

func TestBaselineSuppressesOfflineAndLoad(t *testing.T) {
	cfg := store.DefaultNotifySettings()
	st := &State{}
	o := Observation{ID: "s", Online: false, LastSeen: t0.Add(-time.Hour).Unix()}
	if es, _ := Evaluate(t0, cfg, o, st, true, never); len(es) != 0 || !st.Offline {
		t.Fatalf("首轮只建立基线 %v %+v", kinds(es), st)
	}
	if es, _ := Evaluate(t0.Add(30*time.Second), cfg, o, st, false, never); len(es) != 0 {
		t.Fatalf("基线后仍离线不应补发 %v", kinds(es))
	}
	o2 := Observation{ID: "x", Online: true, LastSeen: t0.Unix(), Ring: ring(t0, 99, 10)}
	st2 := &State{}
	if es, _ := Evaluate(t0, cfg, o2, st2, true, never); len(es) != 0 || !st2.Load {
		t.Fatalf("首轮高负载只建立基线 %v", kinds(es))
	}
}

func TestNeverSeenAndMuted(t *testing.T) {
	cfg := store.DefaultNotifySettings()
	if es, _ := Evaluate(t0, cfg, Observation{ID: "s"}, &State{}, false, never); len(es) != 0 {
		t.Fatalf("从未上报不参与离线判断 %v", kinds(es))
	}
	exp := t0.Add(24 * time.Hour).Unix()
	o := Observation{ID: "s", Muted: true, LastSeen: t0.Add(-time.Hour).Unix(), ExpireAt: &exp}
	if es, ms := Evaluate(t0, cfg, o, &State{}, false, never); len(es) != 0 || len(ms) != 0 {
		t.Fatalf("静音服务器不产生任何事件 %v", kinds(es))
	}
}

func TestLoadRule(t *testing.T) {
	cfg := store.DefaultNotifySettings()
	st := &State{}
	o := Observation{ID: "s", Online: true, LastSeen: t0.Unix(), Ring: ring(t0, 95, 1)}
	if es, _ := Evaluate(t0, cfg, o, st, false, never); len(es) != 0 {
		t.Fatalf("点数不足 2 不判断 %v", kinds(es))
	}
	o.Ring = ring(t0, 95, 10)
	es, _ := Evaluate(t0, cfg, o, st, false, never)
	if !eq(kinds(es), []string{KindLoad}) || len(es[0].Loads) != 1 || es[0].Loads[0] != (LoadValue{Metric: "cpu", Value: 95}) || es[0].Minutes != 5 {
		t.Fatalf("应发送高负载通知 %+v", es)
	}
	o.Ring = append(ring(t0.Add(time.Minute), 20, 10), ring(t0.Add(-10*time.Minute), 99, 5)...) // 窗口外的高值不计入
	es, _ = Evaluate(t0.Add(time.Minute), cfg, o, st, false, never)
	if !eq(kinds(es), []string{KindLoadRecovered}) || st.Load {
		t.Fatalf("应发送负载恢复通知 %+v", es)
	}
	cfg.LoadEnabled = false
	st.Load = true
	if es, _ := Evaluate(t0, cfg, o, st, false, never); len(es) != 0 || st.Load {
		t.Fatalf("关闭规则时静默清除状态 %v", kinds(es))
	}
}

func TestExpireAndTrafficOnce(t *testing.T) {
	cfg := store.DefaultNotifySettings()
	sentKeys := map[string]bool{}
	sent := func(rule, key string) bool { return sentKeys[rule+":"+key] }
	exp := t0.Add(3*24*time.Hour + time.Hour).Unix() // 剩余 4 天（向上取整）
	limit := int64(1000)
	o := Observation{ID: "s", Online: true, LastSeen: t0.Unix(), ExpireAt: &exp, TrafficUsed: 950, TrafficLimit: &limit, Period: "2026-09-01"}
	es, ms := Evaluate(t0, cfg, o, &State{}, true, sent) // 一次性提醒不受首轮基线影响
	if !eq(kinds(es), []string{KindExpire, KindTraffic}) || es[0].Days != 4 || es[1].Percent != 95 || len(ms) != 2 {
		t.Fatalf("应提醒到期与流量 %+v %+v", es, ms)
	}
	for _, m := range ms {
		sentKeys[m.Rule+":"+m.Key] = true
	}
	if es, _ := Evaluate(t0.Add(time.Hour), cfg, o, &State{}, false, sent); len(es) != 0 {
		t.Fatalf("同一到期日、同一周期只提醒一次 %v", kinds(es))
	}
	exp2 := exp + 86400 // 续费改了到期日
	o.ExpireAt = &exp2
	if es, _ := Evaluate(t0, cfg, o, &State{}, false, sent); !eq(kinds(es), []string{KindExpire}) {
		t.Fatalf("修改到期日后应重新提醒 %v", kinds(es))
	}
	o.ExpireAt = nil
	o.TrafficUsed = 899
	o.Period = "2026-10-01"
	if es, _ := Evaluate(t0, cfg, o, &State{}, false, sent); len(es) != 0 {
		t.Fatalf("未达阈值不提醒 %v", kinds(es))
	}
}

func TestMutedStillTracksState(t *testing.T) {
	cfg := store.DefaultNotifySettings()
	st := &State{}
	o := Observation{ID: "s", Online: false, LastSeen: t0.Add(-10 * time.Minute).Unix()}
	if es, _ := Evaluate(t0, cfg, o, st, false, never); !eq(kinds(es), []string{KindOffline}) {
		t.Fatalf("应发送离线通知 %v", kinds(es))
	}
	// 静音期间恢复：只更新状态
	o.Muted, o.Online, o.LastSeen = true, true, t0.Add(time.Minute).Unix()
	if es, ms := Evaluate(t0.Add(time.Minute), cfg, o, st, false, never); len(es) != 0 || len(ms) != 0 || st.Offline {
		t.Fatalf("静音期间只更新状态 %v %+v", kinds(es), st)
	}
	// 取消静音后不补发恢复
	o.Muted = false
	if es, _ := Evaluate(t0.Add(2*time.Minute), cfg, o, st, false, never); len(es) != 0 {
		t.Fatalf("取消静音后不应补发恢复 %v", kinds(es))
	}
	// 静音期间离线：记录状态，取消静音后仍离线不补发离线
	o.Muted, o.Online = true, false
	if es, _ := Evaluate(t0.Add(10*time.Minute), cfg, o, st, false, never); len(es) != 0 || !st.Offline {
		t.Fatalf("静音期间离线应记录状态 %v %+v", kinds(es), st)
	}
	o.Muted = false
	if es, _ := Evaluate(t0.Add(11*time.Minute), cfg, o, st, false, never); len(es) != 0 {
		t.Fatalf("取消静音后仍离线不应补发 %v", kinds(es))
	}
	// 静音期间高负载也只建立状态
	o2 := Observation{ID: "x", Muted: true, Online: true, LastSeen: t0.Unix(), Ring: ring(t0, 99, 10)}
	st2 := &State{}
	if es, _ := Evaluate(t0, cfg, o2, st2, false, never); len(es) != 0 || !st2.Load {
		t.Fatalf("静音期间高负载只建立状态 %v %+v", kinds(es), st2)
	}
}

func TestExpireSkipsLongExpired(t *testing.T) {
	cfg := store.DefaultNotifySettings()
	old := t0.Add(-31 * 24 * time.Hour).Unix()
	o := Observation{ID: "s", Online: true, LastSeen: t0.Unix(), ExpireAt: &old}
	if es, ms := Evaluate(t0, cfg, o, &State{}, false, never); len(es) != 0 || len(ms) != 0 {
		t.Fatalf("过期超过 30 天不提醒 %v", kinds(es))
	}
	recent := t0.Add(-30 * 24 * time.Hour).Unix()
	o.ExpireAt = &recent
	if es, _ := Evaluate(t0, cfg, o, &State{}, false, never); !eq(kinds(es), []string{KindExpire}) || es[0].Days != -30 {
		t.Fatalf("过期 30 天内应提醒 %+v", es)
	}
}
