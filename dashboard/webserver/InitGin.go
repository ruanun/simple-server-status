package webserver

import (
	"github.com/gin-contrib/static"
	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"net/http"
	"simple-server-status/dashboard/common"
	"simple-server-status/dashboard/public"
	"simple-server-status/dashboard/route"
	"simple-server-status/dashboard/webserver/ginstatic"
	"simple-server-status/dashboard/zaplog"
	"strings"
)

func InitServer() *gin.Engine {
	r := gin.Default()

	//gin使用zap日志
	r.Use(ginzap.Ginzap(zaplog.Logger, "2006-01-02 15:04:05.000", true))
	r.Use(ginzap.RecoveryWithZap(zaplog.Logger, true))

	//静态网页
	staticServer := static.Serve("/", ginstatic.EmbedFolder(public.Resource, "dist"))
	r.Use(staticServer)

	r.NoRoute(func(c *gin.Context) {
		//是get请求，路径不是以api开头的跳转到首页
		if c.Request.Method == http.MethodGet &&
			!strings.ContainsRune(c.Request.URL.Path, '.') &&
			!strings.HasPrefix(c.Request.URL.Path, "/api/") {

			//这里直接响应到首页非跳转；转发
			//c.Request.URL.Path = "/"
			//staticServer(c)

			//这里301跳转
			c.Redirect(http.StatusMovedPermanently, "/")
		}
	})
	//配置api
	route.ApiRoute(r)

	//配置websocket
	common.SetWs(r)

	return r
}
