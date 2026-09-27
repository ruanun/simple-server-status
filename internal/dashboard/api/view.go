package api

import (
	"context"

	"github.com/ruanun/simple-server-status/internal/dashboard/store"
	"github.com/ruanun/simple-server-status/internal/proto"
)

// TrafficView 当前计费周期的流量
type TrafficView struct {
	In       int64  `json:"in"`
	Out      int64  `json:"out"`
	Used     int64  `json:"used"`
	Limit    *int64 `json:"limit"`
	Mode     string `json:"mode"`
	ResetDay int    `json:"reset_day"`
	Period   string `json:"period"`
}

// ServerView 前端展示用的服务器状态（不含 secret、IP）
type ServerView struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Group        string        `json:"group"`
	Country      string        `json:"country"`
	Sort         int           `json:"sort"`
	Hidden       bool          `json:"hidden"`
	Online       bool          `json:"online"`
	LastSeen     int64         `json:"last_seen"`
	Static       *proto.Hello  `json:"static"`
	Metrics      *proto.Report `json:"metrics"`
	Traffic      TrafficView   `json:"traffic"`
	ExpireAt     *int64        `json:"expire_at"`
	Price        *float64      `json:"price,omitempty"`
	Currency     string        `json:"currency,omitempty"`
	BillingCycle string        `json:"billing_cycle,omitempty"`
}

func trafficUsed(mode string, in, out int64) int64 {
	switch mode {
	case "in":
		return in
	case "out":
		return out
	}
	return in + out
}

// view 组合配置、实时状态与流量
func (a *API) view(ctx context.Context, srv store.Server, showPrice bool) ServerView {
	live := a.Hub.Get(srv.ID)
	v := ServerView{
		ID: srv.ID, Name: srv.Name, Group: srv.Group, Country: srv.Country, Sort: srv.Sort, Hidden: srv.Hidden,
		Online: live.Online, LastSeen: srv.LastSeen, Static: srv.StaticInfo, Metrics: publicMetrics(live.Report), ExpireAt: srv.ExpireAt,
	}
	if live.Static != nil {
		v.Static = live.Static
	}
	if live.LastReport > 0 {
		v.LastSeen = live.LastReport
	}
	if v.Country == "" && v.Static != nil {
		v.Country = v.Static.Country
	}
	in, out, period, err := a.Traffic.Usage(ctx, srv.ID, srv.TrafficResetDay)
	if err != nil {
		a.Log.Warn("读取流量失败", "id", srv.ID, "err", err)
	}
	v.Traffic = TrafficView{In: in, Out: out, Used: trafficUsed(srv.TrafficMode, in, out), Limit: srv.TrafficLimit,
		Mode: srv.TrafficMode, ResetDay: srv.TrafficResetDay, Period: period}
	if showPrice {
		v.Price, v.Currency, v.BillingCycle = srv.Price, srv.Currency, srv.BillingCycle
	}
	return v
}

// views 返回可见服务器的视图；登录用户可见隐藏服务器与价格
func (a *API) views(ctx context.Context, includeHidden bool) []ServerView {
	showPrice := includeHidden || a.currentSettings().ShowPrice
	list := []ServerView{}
	for _, srv := range a.serverList() {
		if srv.Hidden && !includeHidden {
			continue
		}
		list = append(list, a.view(ctx, srv, showPrice))
	}
	return list
}

// publicMetrics 复制上报数据并清空仅供内部统计使用的字段，避免泄露网卡过滤配置
func publicMetrics(r *proto.Report) *proto.Report {
	if r == nil {
		return nil
	}
	m := *r
	m.FilterID = ""
	return &m
}
