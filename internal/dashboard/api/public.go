package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ruanun/simple-server-status/internal/dashboard/history"
	"github.com/ruanun/simple-server-status/internal/dashboard/store"
)

func (a *API) registerPublic(r *gin.Engine) {
	g := r.Group("/api/public", a.optionalAuth())
	g.GET("/site", a.publicSite)
	g.GET("/servers", a.publicServers)
	g.GET("/servers/:id", a.publicServer)
	g.GET("/servers/:id/metrics", a.publicMetrics)
	g.GET("/ws", a.publicWS)
}

func (a *API) publicSite(c *gin.Context) {
	st := a.currentSettings()
	respond(c, gin.H{"site_title": st.SiteTitle, "show_price": st.ShowPrice})
}

func (a *API) publicServers(c *gin.Context) {
	respond(c, a.views(c.Request.Context(), isAuthed(c)))
}

// visibleServer 读取调用方可见的服务器，不可见时输出 404
func (a *API) visibleServer(c *gin.Context) (store.Server, bool) {
	srv, found := a.server(c.Param("id"))
	if !found || (srv.Hidden && !isAuthed(c)) {
		fail(c, http.StatusNotFound, "not_found", "服务器不存在")
		return srv, false
	}
	return srv, true
}

func (a *API) publicServer(c *gin.Context) {
	srv, found := a.visibleServer(c)
	if !found {
		return
	}
	respond(c, a.view(c.Request.Context(), srv, isAuthed(c) || a.currentSettings().ShowPrice))
}

func (a *API) publicMetrics(c *gin.Context) {
	srv, found := a.visibleServer(c)
	if !found {
		return
	}
	pts, err := history.Query(c.Request.Context(), a.Store, a.Hub.Ring, srv.ID, c.DefaultQuery("range", history.Range1h), a.Now())
	if errors.Is(err, history.ErrBadRange) {
		fail(c, http.StatusBadRequest, "bad_range", "不支持的时间范围")
		return
	}
	if err != nil {
		a.internal(c, "查询历史数据失败", err)
		return
	}
	respond(c, pts)
}
