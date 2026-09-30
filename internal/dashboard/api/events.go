package api

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ruanun/simple-server-status/internal/dashboard/store"
)

// 事件记录的保留期与分页设置
const (
	eventRetention    = 90 * 24 * time.Hour
	publicOutageLimit = 10
	defaultPageSize   = 50
	maxPageSize       = 100
)

// parsePage 解析 page（默认 1）与 size（默认 50，超过 100 按 100）；非法时已输出 400
func parsePage(c *gin.Context) (limit, offset int, ok bool) {
	page, size := 1, defaultPageSize
	for _, p := range []struct {
		key string
		dst *int
	}{{"page", &page}, {"size", &size}} {
		if v := c.Query(p.key); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 {
				fail(c, http.StatusBadRequest, "invalid_input", "分页参数无效")
				return 0, 0, false
			}
			*p.dst = n
		}
	}
	size = min(size, maxPageSize)
	return size, (page - 1) * size, true
}

// outageDuration 离线时长（秒）；进行中的按当前时间计算
func (a *API) outageDuration(o store.Outage) int64 {
	end := a.Now().Unix()
	if o.EndAt != nil {
		end = *o.EndAt
	}
	return max(0, end-o.StartAt)
}

type publicOutage struct {
	StartAt  int64  `json:"start_at"`
	EndAt    *int64 `json:"end_at"`
	Duration int64  `json:"duration"`
}

type adminOutage struct {
	ID         int64  `json:"id"`
	ServerID   string `json:"server_id"`
	ServerName string `json:"server_name"`
	publicOutage
}

func (a *API) publicOutages(c *gin.Context) {
	srv, found := a.visibleServer(c)
	if !found {
		return
	}
	list, _, err := a.Store.ListOutages(c.Request.Context(), srv.ID, publicOutageLimit, 0)
	if err != nil {
		a.internal(c, "读取离线记录失败", err)
		return
	}
	out := make([]publicOutage, 0, len(list))
	for _, o := range list {
		out = append(out, publicOutage{StartAt: o.StartAt, EndAt: o.EndAt, Duration: a.outageDuration(o)})
	}
	respond(c, out)
}

func (a *API) adminOutages(c *gin.Context) {
	limit, offset, ok := parsePage(c)
	if !ok {
		return
	}
	list, total, err := a.Store.ListOutages(c.Request.Context(), c.Query("server_id"), limit, offset)
	if err != nil {
		a.internal(c, "读取离线记录失败", err)
		return
	}
	items := make([]adminOutage, 0, len(list))
	for _, o := range list {
		name := ""
		if srv, found := a.server(o.ServerID); found {
			name = srv.Name
		}
		items = append(items, adminOutage{ID: o.ID, ServerID: o.ServerID, ServerName: name,
			publicOutage: publicOutage{StartAt: o.StartAt, EndAt: o.EndAt, Duration: a.outageDuration(o)}})
	}
	respond(c, gin.H{"items": items, "total": total})
}

var logStatuses = map[string]bool{"": true, store.LogPending: true, store.LogSent: true, store.LogFailed: true}

func (a *API) adminNotifyLog(c *gin.Context) {
	limit, offset, ok := parsePage(c)
	if !ok {
		return
	}
	status := c.Query("status")
	if !logStatuses[status] {
		fail(c, http.StatusBadRequest, "invalid_input", "状态筛选无效")
		return
	}
	list, total, err := a.Store.ListNotifyLog(c.Request.Context(), c.Query("server_id"), status, limit, offset)
	if err != nil {
		a.internal(c, "读取通知记录失败", err)
		return
	}
	respond(c, gin.H{"items": list, "total": total})
}

// cleanupEvents 删除保留期之前的离线记录与通知记录，以及所属服务器已删除的记录
// （删除服务器与离线检查、通知发送并发时可能在删除之后写入）
func (a *API) cleanupEvents(ctx context.Context) {
	cut := a.Now().Add(-eventRetention).Unix()
	if _, err := a.Store.DeleteOutagesBefore(ctx, cut); err != nil {
		a.Log.Warn("清理离线记录失败", "err", err)
	}
	if _, err := a.Store.DeleteOrphanOutages(ctx); err != nil {
		a.Log.Warn("清理孤儿离线记录失败", "err", err)
	}
	if _, err := a.Store.DeleteNotifyLogBefore(ctx, cut); err != nil {
		a.Log.Warn("清理通知记录失败", "err", err)
	}
	if _, err := a.Store.DeleteOrphanNotifyLog(ctx); err != nil {
		a.Log.Warn("清理孤儿通知记录失败", "err", err)
	}
}
