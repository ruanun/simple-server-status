package common

import (
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"net/http"
	"simple-server-status/agent/config"
	"simple-server-status/agent/zaplog"
)

var (
	BuiltAt   string
	GitCommit string
	Version   string = "dev"
	GoVersion string
)

var Conn *websocket.Conn

var retryCountMax = 999

var LOG *zap.SugaredLogger

var AuthHeader http.Header

var AgentConfig *config.AgentConfig

func InitGlobal() {
	//日志
	LOG = zaplog.InitLog()
	//build var
	LOG.Infof("build variable %s %s %s %s", GitCommit, Version, BuiltAt, GoVersion)

	//初始配置
	InitConfig()
	// 创建HTTP请求头
	InitHeader()
}

func InitHeader() {
	AuthHeader = http.Header{}
	AuthHeader.Add("X-AUTH-SECRET", AgentConfig.AuthSecret)
	AuthHeader.Add("X-SERVER-ID", AgentConfig.ServerId)
}
