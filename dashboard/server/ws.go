package server

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/olahol/melody"
	"net"
	"net/http"
	"simple-server-status/dashboard/global"
	"simple-server-status/dashboard/global/constant"
	"simple-server-status/dashboard/internal"
	"simple-server-status/dashboard/pkg/model"
	"strings"
	"time"
)

// InitWebSocket 处理websocket
func InitWebSocket(r *gin.Engine) {
	internal.WsSessionMgr = internal.NewSessionMgr()
	m := melody.New()
	m.Config.MaxMessageSize = 1024 * 10 //10kb ;单位 字节 默认 512

	r.GET(global.CONFIG.WebSocketPath, func(c *gin.Context) {
		secret := c.GetHeader(constant.HeaderSecret)
		serverId := c.GetHeader(constant.HeaderId)

		if Authentication(secret, serverId) {
			//处理websocket
			m.HandleRequest(c.Writer, c.Request)
		} else {
			global.LOG.Info("未授权连接！Headers: ", c.Request.Header)
		}
	})
	//收到消息
	m.HandleMessage(handleMessage)
	//连接成功
	m.HandleConnect(handleConnect)

	m.HandleDisconnect(func(s *melody.Session) {
		serverId, _ := internal.WsSessionMgr.GetServerId(s)
		global.LOG.Infof("断开连接 serverId:%s ip: %s", serverId, GetIP(s.Request))
		//删除绑定session
		internal.WsSessionMgr.DelBySession(s)
	})
}

func handleMessage(s *melody.Session, msg []byte) {
	var serverStatusInfo model.ServerInfo
	err := json.Unmarshal(msg, &serverStatusInfo)
	if err != nil {
		global.LOG.Infof("发的消息格式错误！serverId: %s ip: %s", string(msg), GetIP(s.Request))
		return
	}
	//通过session获取服务器id
	serverId, b := internal.WsSessionMgr.GetServerId(s)
	if !b {
		global.LOG.Infof("未授权连接！serverId: %s ip: %s", serverId, GetIP(s.Request))
		return
	}

	server, _ := global.SERVERS.Get(serverId)
	serverStatusInfo.Name = server.Name
	serverStatusInfo.Group = server.Group
	serverStatusInfo.Id = server.Id
	serverStatusInfo.LastReportTime = time.Now().Unix()
	if server.CountryCode != "" {
		serverStatusInfo.Loc = server.CountryCode
	}
	//转换为小写字符
	serverStatusInfo.Loc = strings.ToLower(serverStatusInfo.Loc)

	global.ServerStatusInfoMap.Set(serverId, &serverStatusInfo)
}

func handleConnect(s *melody.Session) {
	secret := s.Request.Header.Get(constant.HeaderSecret)
	serverId := s.Request.Header.Get(constant.HeaderId)
	if !Authentication(secret, serverId) {
		global.LOG.Info("未授权连接！", s.Request.Header)
		s.CloseWithMsg([]byte("未授权连接！"))
	}
	global.LOG.Infof("连接成功 serverId: %s  ip: %s", serverId, GetIP(s.Request))

	//绑定session
	internal.WsSessionMgr.Add(serverId, s)
}

func GetIP(r *http.Request) string {
	ip := r.Header.Get("X-Real-IP")
	if net.ParseIP(ip) != nil {
		return ip
	}

	ip = r.Header.Get("X-Forward-For")
	for _, i := range strings.Split(ip, ",") {
		if net.ParseIP(i) != nil {
			return i
		}
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return ""
	}

	if net.ParseIP(ip) != nil {
		return ip
	}

	return ""
}
func Authentication(secret string, id string) bool {
	if secret == "" || id == "" {
		return false
	}

	if s, ok := global.SERVERS.Get(id); ok {
		if s.Secret == secret {
			return true
		}
	}
	return false
}
