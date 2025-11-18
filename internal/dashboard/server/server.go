package server

import (
	"net/http"
	"strings"

	"github.com/gin-contrib/static"
	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	internal "github.com/ruanun/simple-server-status/internal/dashboard"
	"github.com/ruanun/simple-server-status/internal/dashboard/config"
	"github.com/ruanun/simple-server-status/internal/dashboard/public"
)

func InitServer(cfg *config.DashboardConfig, logger *zap.SugaredLogger, errorHandler *internal.ErrorHandler) *gin.Engine {
	if !cfg.Debug {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()

	// 安全中间件
	r.Use(internal.SecurityMiddleware())

	// CORS中间件
	r.Use(internal.CORSMiddleware())

	// 使用自定义的错误处理中间件
	r.Use(internal.PanicRecoveryMiddleware(errorHandler))
	r.Use(internal.ErrorMiddleware(errorHandler))

	//gin使用zap日志
	r.Use(ginzap.GinzapWithConfig(logger.Desugar(), &ginzap.Config{TimeFormat: "2006-01-02 15:04:05.000", UTC: true, DefaultLevel: zapcore.DebugLevel}))

	//静态网页
	staticServer := static.Serve("/", static.EmbedFolder(public.Resource, "dist"))
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

	return r
}
