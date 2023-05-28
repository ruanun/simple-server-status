package common

import (
	"github.com/olahol/melody"
	cmap "github.com/orcaman/concurrent-map/v2"
	"go.uber.org/zap"
	"simple-server-status/dashboard/config"
	"simple-server-status/dashboard/zaplog"
	"simple-server-status/model"
)

var (
	BuiltAt   string
	GitCommit string
	Version   string = "dev"
	GoVersion string
)

var LOG *zap.SugaredLogger

var (
	// key：服务器id 配置文件； value: session
	ServerIdSessionMap map[string]*melody.Session
	// key：session  value: 服务器id；
	SessionServerIdMap map[*melody.Session]string
)

var (
	// 控制台配置文件
	CONFIG *config.DashboardConfig
	// key: 服务器id; value: 服务器配置
	//SERVERS map[string]*config.ServerConfig
	SERVERS cmap.ConcurrentMap[string, *config.ServerConfig]
)

// agent上报的信息;  key: 服务器id; value: 上报的信息
// var ServerStatusInfoMap = make(map[string]*model.ServerInfo)
var ServerStatusInfoMap = cmap.New[*model.ServerInfo]()

const HeaderSecret = "X-AUTH-SECRET"
const HeaderId = "X-SERVER-ID"

// InitGlobal 初始化配置文件，日志等
func InitGlobal() {
	//init log
	LOG = zaplog.InitLog()

	//build var
	LOG.Infof("build variable %s %s %s %s", GitCommit, Version, BuiltAt, GoVersion)

	//server
	ServerIdSessionMap = make(map[string]*melody.Session)
	SessionServerIdMap = make(map[*melody.Session]string)
	SERVERS = cmap.New[*config.ServerConfig]()

	//初始化配置文件
	InitConfig()
}

func ServerAuthentication(secret string, id string) bool {
	if secret == "" || id == "" {
		return false
	}

	if s, ok := SERVERS.Get(id); ok {
		if s.Secret == secret {
			return true
		}
	}
	return false
}
