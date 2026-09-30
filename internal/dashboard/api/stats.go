package api

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ruanun/simple-server-status/internal/dashboard/history"
	"github.com/ruanun/simple-server-status/internal/dashboard/store"
	"github.com/ruanun/simple-server-status/internal/dashboard/traffic"
)

// statsView 详情页统计：在线率与当前计费周期的每日流量
type statsView struct {
	Uptime24h   *float64             `json:"uptime_24h"`
	Uptime7d    *float64             `json:"uptime_7d"`
	PeriodStart string               `json:"period_start"`
	PeriodEnd   string               `json:"period_end"`
	Daily       []store.DailyTraffic `json:"daily"`
}

func (a *API) publicStats(c *gin.Context) {
	srv, found := a.visibleServer(c)
	if !found {
		return
	}
	ctx := c.Request.Context()
	now := a.Now()
	up24, err := history.Uptime(ctx, a.Store, srv.ID, srv.CreatedAt, now, 24*time.Hour)
	if err != nil {
		a.internal(c, "计算在线率失败", err)
		return
	}
	up7d, err := history.Uptime(ctx, a.Store, srv.ID, srv.CreatedAt, now, 7*24*time.Hour)
	if err != nil {
		a.internal(c, "计算在线率失败", err)
		return
	}
	start := traffic.PeriodStart(now, srv.TrafficResetDay)
	daily, err := a.Traffic.Daily(ctx, srv.ID, start.Format(time.DateOnly), now.Format(time.DateOnly))
	if err != nil {
		a.internal(c, "读取每日流量失败", err)
		return
	}
	respond(c, statsView{Uptime24h: up24, Uptime7d: up7d, PeriodStart: start.Format(time.DateOnly),
		PeriodEnd: start.AddDate(0, 1, -1).Format(time.DateOnly), Daily: daily})
}
