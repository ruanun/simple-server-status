package router

import (
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"simple-server-status/dashboard/global"
	"simple-server-status/dashboard/internal"
	"simple-server-status/dashboard/pkg/model"
	"simple-server-status/dashboard/pkg/model/result"
	"sort"
	"time"
)

func InitApi(r *gin.Engine) {
	group := r.Group("/api")
	{
		//基础信息
		group.GET("/statusInfo", StatusInfo)

		//统计信息
		group.GET("/statistics", func(c *gin.Context) {
			result.OkWithData(c, gin.H{
				"onlineIds":        internal.WsSessionMgr.GetAllServerId(),
				"sessionMapLen":    len(internal.WsSessionMgr.SessionServerIdMap),
				"reportMapLen":     global.ServerStatusInfoMap.Count(),
				"configServersLen": global.SERVERS.Count(),
			})
		})
	}
}

func StatusInfo(c *gin.Context) {
	// 处理数据结构并返回
	values := lo.Values(global.ServerStatusInfoMap.Items())
	//转换
	baseServerInfos := lo.Map(values, func(item *model.ServerInfo, index int) *model.RespServerInfo {
		info := model.NewRespServerInfo(item)
		isOnline := time.Now().Unix()-info.LastReportTime <= int64(global.CONFIG.ReportTimeIntervalMax)
		info.IsOnline = isOnline
		return info
	})
	sort.Slice(baseServerInfos, func(i, j int) bool {
		return baseServerInfos[i].Id < baseServerInfos[j].Id
	})
	groupMap := lo.GroupBy(baseServerInfos, func(item *model.RespServerInfo) string {
		return item.Group
	})

	result.OkWithData(c, groupMap)
}
