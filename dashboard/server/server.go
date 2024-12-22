package server

import (
	"github.com/gin-contrib/static"
	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap/zapcore"
	"net/http"
	"simple-server-status/dashboard/global"
	"simple-server-status/dashboard/public"
	"simple-server-status/dashboard/router"
	v2 "simple-server-status/dashboard/router/v2"
	"strings"
)

func InitServer() *gin.Engine {
	if !global.CONFIG.Debug {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	//r.Use(gin.Recovery())
	//gin使用zap日志
	//r.Use(ginzap.Ginzap(global.LOG.Desugar(), "2006-01-02 15:04:05.000", true))
	r.Use(ginzap.GinzapWithConfig(global.LOG.Desugar(), &ginzap.Config{TimeFormat: "2006-01-02 15:04:05.000", UTC: true, DefaultLevel: zapcore.DebugLevel}))
	r.Use(ginzap.RecoveryWithZap(global.LOG.Desugar(), true))

	//静态网页
	staticServer := static.Serve("/", static.EmbedFolder(public.Resource, "dist"))
	r.Use(staticServer)

	//配置websocket
	InitWebSocket(r)

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
	router.InitApi(r)
	v2.InitApi(r)
	return r
}
