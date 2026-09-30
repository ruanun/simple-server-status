package notify

import (
	"strings"
	"testing"
	"time"
)

func TestRender(t *testing.T) {
	exp := time.Date(2026, 10, 3, 0, 0, 0, 0, time.Local).Unix()
	cases := []struct {
		e     Event
		lang  string
		title string
		msg   string
	}{
		{Event{Kind: KindOffline, ServerName: "hk", Minutes: 75}, "zh-CN", "[离线] hk", "服务器 hk 已离线 1 小时 15 分钟"},
		{Event{Kind: KindOffline, ServerName: "hk", Minutes: 5}, "en-US", "[Offline] hk", "Server hk has been offline for 5m"},
		{Event{Kind: KindRecovered, ServerName: "hk", Minutes: 60}, "zh-CN", "[恢复] hk", "服务器 hk 已恢复在线，离线约 1 小时"},
		{Event{Kind: KindLoad, ServerName: "hk", Minutes: 5, Loads: []LoadValue{{"cpu", 95}, {"mem", 92.1}}}, "zh-CN", "[高负载] hk", "服务器 hk 最近 5 分钟平均负载过高：CPU 95.0%、内存 92.1%"},
		{Event{Kind: KindLoadRecovered, ServerName: "hk", Minutes: 5}, "en-US", "[Load recovered] hk", "Server hk load is back to normal over the last 5 minutes"},
		{Event{Kind: KindExpire, ServerName: "hk", Days: 4, ExpireAt: exp}, "zh-CN", "[即将到期] hk", "服务器 hk 将在 4 天后到期（2026-10-03）"},
		{Event{Kind: KindExpire, ServerName: "hk", Days: 0, ExpireAt: exp}, "en-US", "[Expired] hk", "Server hk expired on 2026-10-03"},
		{Event{Kind: KindExpire, ServerName: "hk", Days: -3, ExpireAt: exp}, "zh-CN", "[已到期] hk", "服务器 hk 已于 2026-10-03 到期"},
		{Event{Kind: KindExpire, ServerName: "hk", Days: 1, ExpireAt: exp}, "en-US", "[Expiring] hk", "Server hk expires in 1 day (2026-10-03)"},
		{Event{Kind: KindExpire, ServerName: "hk", Days: 4, ExpireAt: exp}, "en-US", "[Expiring] hk", "Server hk expires in 4 days (2026-10-03)"},
		{Event{Kind: KindTraffic, ServerName: "hk", Percent: 95, Used: 950 * 1024 * 1024 * 1024, Limit: 1024 * 1024 * 1024 * 1024}, "zh-CN", "[流量提醒] hk", "服务器 hk 本周期流量已用 95.0%（950 GB / 1.0 TB）"},
		{Event{Kind: KindTest}, "zh-CN", "[测试] Simple Server Status", "这是一条来自 Simple Server Status 的测试通知"},
	}
	for _, c := range cases {
		e := c.e
		Render(&e, c.lang)
		if e.Title != c.title || e.Message != c.msg {
			t.Errorf("%s/%s:\n得到 %q %q\n期望 %q %q", c.e.Kind, c.lang, e.Title, e.Message, c.title, c.msg)
		}
	}
	e := Event{Kind: KindOffline, ServerName: "hk", Minutes: 3}
	Render(&e, "")
	if !strings.HasPrefix(e.Title, "[离线]") {
		t.Errorf("未知语言应使用中文 %q", e.Title)
	}
}
