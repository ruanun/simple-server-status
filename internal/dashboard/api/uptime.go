package api

import (
	"context"
	"time"

	"github.com/ruanun/simple-server-status/internal/dashboard/history"
)

// refreshUptime 重新计算所有服务器的 24 小时在线率并缓存，避免每次请求查库
func (a *API) refreshUptime(ctx context.Context) {
	now := a.Now()
	m := map[string]*float64{}
	for _, s := range a.serverList() {
		v, err := history.Uptime(ctx, a.Store, s.ID, s.CreatedAt, now, 24*time.Hour)
		if err != nil {
			a.Log.Warn("计算在线率失败", "id", s.ID, "err", err)
			m[s.ID] = a.cachedUptime(s.ID) // 保留上一次的值，避免短暂出错时在线率变为空
			continue
		}
		m[s.ID] = v
	}
	a.upMu.Lock()
	a.uptimeCache = m
	a.upMu.Unlock()
}

// cachedUptime 返回缓存的 24 小时在线率；无数据时为 nil
func (a *API) cachedUptime(id string) *float64 {
	a.upMu.RLock()
	defer a.upMu.RUnlock()
	return a.uptimeCache[id]
}
