package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/ruanun/simple-server-status/internal/dashboard/auth"
	"github.com/ruanun/simple-server-status/internal/dashboard/store"
)

const (
	ctxUserKey   = "sss_user"
	ctxClaimsKey = "sss_claims"
)

// publicWSPath 浏览器 WebSocket 路径：浏览器无法为 WebSocket 设置请求头，只能通过 query 传 token
const publicWSPath = "/api/public/ws"

// bearerToken 从 Authorization 头读取 token；仅浏览器 WebSocket 允许使用 token 查询参数，
// 避免 token 出现在其他接口的 URL（日志、Referer）中
func bearerToken(c *gin.Context) string {
	if t, ok := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer "); ok {
		return t
	}
	if c.Request.URL.Path == publicWSPath {
		return c.Query("token")
	}
	return ""
}

// authenticate 校验 JWT，并确认 token 版本与数据库一致；通过时同时记录 token 载荷
func (a *API) authenticate(c *gin.Context) (store.User, bool) {
	tok := bearerToken(c)
	if tok == "" {
		return store.User{}, false
	}
	claims, err := a.Auth.Parse(tok)
	if err != nil {
		return store.User{}, false
	}
	u, err := a.Store.GetUser(c.Request.Context(), claims.UID)
	if err != nil || u.TokenVersion != claims.TV {
		return store.User{}, false
	}
	c.Set(ctxClaimsKey, claims)
	return u, true
}

// optionalAuth 有合法 token 时记录登录用户，否则按匿名处理
func (a *API) optionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if u, ok := a.authenticate(c); ok {
			c.Set(ctxUserKey, u)
		}
		c.Next()
	}
}

// requireAuth 未登录时返回 401
func (a *API) requireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		u, ok := a.authenticate(c)
		if !ok {
			fail(c, http.StatusUnauthorized, "unauthorized", "请先登录")
			return
		}
		c.Set(ctxUserKey, u)
		c.Next()
	}
}

func isAuthed(c *gin.Context) bool {
	_, ok := c.Get(ctxUserKey)
	return ok
}

// currentClaims 返回已通过校验的 token 载荷
func currentClaims(c *gin.Context) (auth.Claims, bool) {
	v, ok := c.Get(ctxClaimsKey)
	if !ok {
		return auth.Claims{}, false
	}
	cl, ok := v.(auth.Claims)
	return cl, ok
}

func currentUser(c *gin.Context) store.User {
	v, _ := c.Get(ctxUserKey)
	u, _ := v.(store.User)
	return u
}
