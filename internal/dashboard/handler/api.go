package handler

import (
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ruanun/simple-server-status/internal/dashboard/response"
	"github.com/ruanun/simple-server-status/pkg/model"
	"github.com/samber/lo"
)

// WebSocketStatsProvider 定义 WebSocket 统计信息提供者接口
// 用于避免循环导入 internal/dashboard 包
type WebSocketStatsProvider interface {
	GetAllServerId() []string
	SessionLength() int
}

// ServerStatusMapProvider 服务器状态 Map 提供者接口
type ServerStatusMapProvider interface {
	Count() int
	Items() map[string]*model.ServerInfo
}

// ServerConfigMapProvider 服务器配置 Map 提供者接口
type ServerConfigMapProvider interface {
	Count() int
}

// InitApi 初始化 API 路由
// wsManager: Agent WebSocket 管理器，用于获取连接统计信息
// configProvider: 配置提供者
// logger: 日志记录器
// serverStatusMap: 服务器状态 Map 提供者
// serverConfigMap: 服务器配置 Map 提供者
// configValidator: 配置验证器提供者
func InitApi(
	r *gin.Engine,
	wsManager WebSocketStatsProvider,
	configProvider ConfigProvider,
	logger LoggerProvider,
	serverStatusMap ServerStatusMapProvider,
	serverConfigMap ServerConfigMapProvider,
	configValidator ConfigValidatorProvider,
) {
	group := r.Group("/api")

	{
		group.GET("/server/statusInfo", StatusInfo(serverStatusMap, configProvider))
		//统计信息
		group.GET("/statistics", func(c *gin.Context) {
			response.Success(c, gin.H{
				"onlineIds":        wsManager.GetAllServerId(),
				"sessionMapLen":    wsManager.SessionLength(),
				"reportMapLen":     serverStatusMap.Count(),
				"configServersLen": serverConfigMap.Count(),
			})
		})

		// 初始化配置相关API TODO 暂不使用
		//InitConfigAPI(group, configProvider, logger, configValidator)

	}
}

// StatusInfo 获取服务器状态信息（工厂函数）
func StatusInfo(serverStatusMap ServerStatusMapProvider, configProvider ConfigProvider) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 处理数据结构并返回
		values := lo.Values(serverStatusMap.Items())
		//转换
		baseServerInfos := lo.Map(values, func(item *model.ServerInfo, index int) *model.RespServerInfo {
			info := model.NewRespServerInfo(item)
			isOnline := time.Now().Unix()-info.LastReportTime <= int64(configProvider.GetConfig().ReportTimeIntervalMax)
			info.IsOnline = isOnline
			return info
		})
		sort.Slice(baseServerInfos, func(i, j int) bool {
			return baseServerInfos[i].Id < baseServerInfos[j].Id
		})
		response.Success(c, baseServerInfos)
	}
}
