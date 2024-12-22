package global

import (
	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"simple-server-status/dashboard/config"
	"simple-server-status/dashboard/pkg/model"
)

var (
	BuiltAt   string
	GitCommit string
	Version   string = "dev"
	GoVersion string
)

var (
	CONFIG *config.DashboardConfig
	VP     *viper.Viper
	LOG    *zap.SugaredLogger
)

// SERVERS 服务器信息 key: 服务器id; value: 服务器配置
var SERVERS cmap.ConcurrentMap[string, *config.ServerConfig] = cmap.New[*config.ServerConfig]()

// ServerStatusInfoMap agent上报的信息;  key: 服务器id; value: 上报的信息
var ServerStatusInfoMap = cmap.New[*model.ServerInfo]()
