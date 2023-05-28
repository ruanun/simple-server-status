package route

import (
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"simple-server-status/dashboard/common"
	"simple-server-status/model"
	"simple-server-status/model/result"
	"sort"
	"time"
)

func ApiRoute(r *gin.Engine) {
	group := r.Group("/api")
	{
		//基础信息
		group.GET("/statusInfo", func(c *gin.Context) {
			// 处理数据结构并返回
			values := lo.Values(common.ServerStatusInfoMap.Items())
			//转换
			baseServerInfos := lo.Map(values, func(item *model.ServerInfo, index int) *model.RespServerInfo {
				info := model.NewRespServerInfo(item)
				isOnline := time.Now().Unix()-info.LastReportTime <= int64(common.CONFIG.ReportTimeIntervalMax)
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
		})

		//统计信息
		group.GET("/statistics", func(c *gin.Context) {
			result.OkWithData(c, gin.H{
				"onlineIds":        lo.Keys(common.ServerIdSessionMap),
				"sessionMapLen":    len(common.SessionServerIdMap),
				"reportMapLen":     common.ServerStatusInfoMap.Count(),
				"configServersLen": common.SERVERS.Count(),
			})
		})
	}
}
