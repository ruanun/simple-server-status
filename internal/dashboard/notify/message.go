package notify

import (
	"fmt"
	"strings"
	"time"
)

// Render 按语言填充事件的标题与正文；lang 为 en-US 时使用英文，其余使用中文
func Render(e *Event, lang string) {
	en := lang == "en-US"
	name := e.ServerName
	pick := func(zh, enText string) string {
		if en {
			return enText
		}
		return zh
	}
	switch e.Kind {
	case KindOffline:
		d := duration(e.Minutes, en)
		e.Title = pick("[离线] ", "[Offline] ") + name
		e.Message = pick(fmt.Sprintf("服务器 %s 已离线 %s", name, d), fmt.Sprintf("Server %s has been offline for %s", name, d))
	case KindRecovered:
		d := duration(e.Minutes, en)
		e.Title = pick("[恢复] ", "[Recovered] ") + name
		e.Message = pick(fmt.Sprintf("服务器 %s 已恢复在线，离线约 %s", name, d), fmt.Sprintf("Server %s is back online after about %s", name, d))
	case KindLoad:
		e.Title = pick("[高负载] ", "[High load] ") + name
		e.Message = pick(
			fmt.Sprintf("服务器 %s 最近 %d 分钟平均负载过高：%s", name, e.Minutes, loads(e.Loads, en)),
			fmt.Sprintf("Server %s has high average load over the last %d minutes: %s", name, e.Minutes, loads(e.Loads, en)))
	case KindLoadRecovered:
		e.Title = pick("[负载恢复] ", "[Load recovered] ") + name
		e.Message = pick(fmt.Sprintf("服务器 %s 最近 %d 分钟的负载已恢复正常", name, e.Minutes),
			fmt.Sprintf("Server %s load is back to normal over the last %d minutes", name, e.Minutes))
	case KindExpire:
		date := time.Unix(e.ExpireAt, 0).Format(time.DateOnly)
		if e.Days > 0 {
			days := "days"
			if e.Days == 1 {
				days = "day"
			}
			e.Title = pick("[即将到期] ", "[Expiring] ") + name
			e.Message = pick(fmt.Sprintf("服务器 %s 将在 %d 天后到期（%s）", name, e.Days, date), fmt.Sprintf("Server %s expires in %d %s (%s)", name, e.Days, days, date))
		} else {
			e.Title = pick("[已到期] ", "[Expired] ") + name
			e.Message = pick(fmt.Sprintf("服务器 %s 已于 %s 到期", name, date), fmt.Sprintf("Server %s expired on %s", name, date))
		}
	case KindTraffic:
		e.Title = pick("[流量提醒] ", "[Traffic] ") + name
		e.Message = pick(
			fmt.Sprintf("服务器 %s 本周期流量已用 %.1f%%（%s / %s）", name, e.Percent, formatBytes(e.Used), formatBytes(e.Limit)),
			fmt.Sprintf("Server %s has used %.1f%% of its traffic this period (%s / %s)", name, e.Percent, formatBytes(e.Used), formatBytes(e.Limit)))
	case KindTest:
		e.Title = pick("[测试] ", "[Test] ") + "Simple Server Status"
		e.Message = pick("这是一条来自 Simple Server Status 的测试通知", "This is a test notification from Simple Server Status")
	}
}

// duration 把分钟数格式化为「1 小时 15 分钟」/「1h 15m」
func duration(minutes int64, en bool) string {
	h, m := minutes/60, minutes%60
	switch {
	case h == 0 && en:
		return fmt.Sprintf("%dm", m)
	case h == 0:
		return fmt.Sprintf("%d 分钟", m)
	case m == 0 && en:
		return fmt.Sprintf("%dh", h)
	case m == 0:
		return fmt.Sprintf("%d 小时", h)
	case en:
		return fmt.Sprintf("%dh %dm", h, m)
	}
	return fmt.Sprintf("%d 小时 %d 分钟", h, m)
}

func loads(vs []LoadValue, en bool) string {
	labels := map[string][2]string{"cpu": {"CPU", "CPU"}, "mem": {"内存", "memory"}, "disk": {"硬盘", "disk"}}
	parts := make([]string, 0, len(vs))
	for _, v := range vs {
		l := labels[v.Metric][0]
		if en {
			l = labels[v.Metric][1]
		}
		parts = append(parts, fmt.Sprintf("%s %.1f%%", l, v.Value))
	}
	if en {
		return strings.Join(parts, ", ")
	}
	return strings.Join(parts, "、")
}

// formatBytes 以 1024 进制格式化字节数，小于 100 保留 1 位小数
func formatBytes(n int64) string {
	units := []string{"B", "KB", "MB", "GB", "TB", "PB"}
	v := float64(n)
	i := 0
	for v >= 1024 && i < len(units)-1 {
		v /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%d B", n)
	}
	if v < 100 {
		return fmt.Sprintf("%.1f %s", v, units[i])
	}
	return fmt.Sprintf("%.0f %s", v, units[i])
}
