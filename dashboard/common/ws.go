package common

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/olahol/melody"
	"net"
	"net/http"
	"simple-server-status/model"
	"strings"
	"time"
)

// SetWs 处理websocket
func SetWs(r *gin.Engine) {
	m := melody.New()
	m.Config.MaxMessageSize = 1024 * 10 //10kb ;单位 字节 默认 512

	r.GET(CONFIG.WebSocketPath, func(c *gin.Context) {
		secret := c.GetHeader(HeaderSecret)
		serverId := c.GetHeader(HeaderId)

		if ServerAuthentication(secret, serverId) {
			//处理websocket
			m.HandleRequest(c.Writer, c.Request)
		} else {
			LOG.Info("未授权连接！Headers: ", c.Request.Header)
		}
	})
	//收到消息
	m.HandleMessage(handleMessage)
	//连接成功
	m.HandleConnect(handleConnect)

	m.HandleDisconnect(func(s *melody.Session) {
		LOG.Infof("断开连接 serverId:%s ip: %s", SessionServerIdMap[s], GetIP(s.Request))
		//删除绑定session
		id := SessionServerIdMap[s]
		delete(SessionServerIdMap, s)
		delete(ServerIdSessionMap, id)
	})
}

func handleMessage(s *melody.Session, msg []byte) {
	var serverStatusInfo model.ServerInfo
	err := json.Unmarshal(msg, &serverStatusInfo)
	if err != nil {
		LOG.Infof("发的消息格式错误！serverId: %s ip: %s", string(msg), GetIP(s.Request))
		return
	}
	//通过session获取服务器id
	serverId := SessionServerIdMap[s]

	server, _ := SERVERS.Get(serverId)
	serverStatusInfo.Name = server.Name
	serverStatusInfo.Group = server.Group
	serverStatusInfo.Id = server.Id
	serverStatusInfo.LastReportTime = time.Now().Unix()

	ServerStatusInfoMap.Set(serverId, &serverStatusInfo)
}

func handleConnect(s *melody.Session) {
	secret := s.Request.Header.Get(HeaderSecret)
	serverId := s.Request.Header.Get(HeaderId)
	if !ServerAuthentication(secret, serverId) {
		LOG.Info("未授权连接！", s.Request.Header)
		s.CloseWithMsg([]byte("未授权连接！"))
	}
	LOG.Infof("连接成功 serverId: %s  ip: %s", serverId, GetIP(s.Request))

	//绑定session
	ServerIdSessionMap[serverId] = s
	SessionServerIdMap[s] = serverId
}

func getHeader(s *melody.Session, key string) string {
	return s.Request.Header.Get(key)
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
