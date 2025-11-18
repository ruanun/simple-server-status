package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/olahol/melody"
	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/ruanun/simple-server-status/pkg/model"
)

// ServerStatusIterator 服务器状态迭代器接口
type ServerStatusIterator interface {
	IterBuffered() <-chan cmap.Tuple[string, *model.ServerInfo]
}

// FrontendWebSocketManager 前端 WebSocket 管理器
// 用于管理前端用户（浏览器）的连接，向前端推送服务器状态数据
type FrontendWebSocketManager struct {
	mu          sync.RWMutex
	connections map[*melody.Session]bool // 存储前端连接
	melody      *melody.Melody
	ctx         context.Context
	cancel      context.CancelFunc
	pushTicker  *time.Ticker
	logger      interface {
		Infof(string, ...interface{})
		Errorf(string, ...interface{})
		Info(...interface{})
	}
	errorHandler *ErrorHandler

	// 数据访问
	serverStatus ServerStatusIterator
	configAccess ConfigAccessor

	// 统计信息
	totalConnections    int64
	totalDisconnections int64
	totalMessages       int64
}

// NewFrontendWebSocketManager 创建新的前端WebSocket管理器
func NewFrontendWebSocketManager(
	logger interface {
		Infof(string, ...interface{})
		Errorf(string, ...interface{})
		Info(...interface{})
	},
	errorHandler *ErrorHandler,
	serverStatus ServerStatusIterator,
	configAccess ConfigAccessor,
) *FrontendWebSocketManager {
	ctx, cancel := context.WithCancel(context.Background())
	m := melody.New()
	m.Config.MaxMessageSize = 1024 * 10 // 10KB

	fwsm := &FrontendWebSocketManager{
		connections:  make(map[*melody.Session]bool),
		melody:       m,
		ctx:          ctx,
		cancel:       cancel,
		pushTicker:   time.NewTicker(1 * time.Second), // 每1秒推送一次数据（WebSocket实时模式）
		logger:       logger,
		errorHandler: errorHandler,
		serverStatus: serverStatus,
		configAccess: configAccess,
	}

	// 设置melody事件处理器
	m.HandleConnect(fwsm.handleConnect)
	m.HandleMessage(fwsm.handleMessage)
	m.HandleDisconnect(fwsm.handleDisconnect)
	m.HandleError(fwsm.handleError)

	// 启动数据推送循环
	go fwsm.dataPushLoop()

	return fwsm
}

// SetupFrontendRoutes 设置前端WebSocket路由
func (fwsm *FrontendWebSocketManager) SetupFrontendRoutes(r *gin.Engine) {
	r.GET("/ws-frontend", func(c *gin.Context) {
		// 前端连接不需要认证
		_ = fwsm.melody.HandleRequest(c.Writer, c.Request) // 忽略错误，melody 已经处理了响应
	})
}

// handleConnect 处理连接事件
func (fwsm *FrontendWebSocketManager) handleConnect(s *melody.Session) {
	fwsm.mu.Lock()
	fwsm.connections[s] = true
	fwsm.totalConnections++
	fwsm.mu.Unlock()

	fwsm.logger.Infof("前端WebSocket连接成功 - IP: %s", fwsm.getClientIP(s))

	// 立即发送当前服务器状态数据
	fwsm.sendCurrentData(s)
}

// handleMessage 处理消息事件
func (fwsm *FrontendWebSocketManager) handleMessage(s *melody.Session, msg []byte) {
	fwsm.mu.Lock()
	fwsm.totalMessages++
	fwsm.mu.Unlock()

	// 解析前端发送的消息（如心跳等）
	var message map[string]interface{}
	if err := json.Unmarshal(msg, &message); err != nil {
		// 记录消息格式错误
		msgErr := NewValidationError("前端WebSocket消息格式错误", fmt.Sprintf("IP: %s, Error: %v", fwsm.getClientIP(s), err))
		msgErr.IP = fwsm.getClientIP(s)
		if fwsm.errorHandler != nil {
			fwsm.errorHandler.RecordError(msgErr)
		}
		return
	}

	// 处理心跳消息
	if msgType, ok := message["type"]; ok && msgType == "ping" {
		response := map[string]interface{}{
			"type":      "pong",
			"timestamp": time.Now().Unix(),
		}
		if responseData, err := json.Marshal(response); err == nil {
			_ = s.Write(responseData) // 忽略写入错误，melody 会处理连接问题
		}
	}
}

// handleDisconnect 处理断开连接事件
func (fwsm *FrontendWebSocketManager) handleDisconnect(s *melody.Session) {
	fwsm.mu.Lock()
	delete(fwsm.connections, s)
	fwsm.totalDisconnections++
	fwsm.mu.Unlock()

	fwsm.logger.Infof("前端WebSocket连接断开 - IP: %s", fwsm.getClientIP(s))
}

// handleError 处理错误事件
func (fwsm *FrontendWebSocketManager) handleError(s *melody.Session, err error) {
	// 记录前端WebSocket错误
	wsErr := NewWebSocketError("前端WebSocket连接错误", fmt.Sprintf("IP: %s, Error: %v", fwsm.getClientIP(s), err))
	wsErr.IP = fwsm.getClientIP(s)
	if fwsm.errorHandler != nil {
		fwsm.errorHandler.RecordError(wsErr)
	}
}

// dataPushLoop 数据推送循环
func (fwsm *FrontendWebSocketManager) dataPushLoop() {
	for {
		select {
		case <-fwsm.ctx.Done():
			return
		case <-fwsm.pushTicker.C:
			fwsm.broadcastServerData()
		}
	}
}

// buildServerStatusMessage 构建服务器状态消息
func (fwsm *FrontendWebSocketManager) buildServerStatusMessage() ([]byte, error) {
	serverData := fwsm.getAllServerData()
	message := map[string]interface{}{
		"type":      "server_status_update",
		"data":      serverData,
		"timestamp": time.Now().Unix(),
	}
	return json.Marshal(message)
}

// broadcastServerData 广播服务器数据给所有前端连接
func (fwsm *FrontendWebSocketManager) broadcastServerData() {
	fwsm.mu.RLock()
	connectionCount := len(fwsm.connections)
	fwsm.mu.RUnlock()

	if connectionCount == 0 {
		return // 没有连接的前端客户端
	}

	msgData, err := fwsm.buildServerStatusMessage()
	if err != nil {
		fwsm.logger.Errorf("构建服务器状态消息失败: %v", err)
		return
	}

	// 广播给所有连接的前端客户端
	_ = fwsm.melody.Broadcast(msgData) // 忽略广播错误，melody 会处理连接问题
}

// sendCurrentData 发送当前数据给指定连接
func (fwsm *FrontendWebSocketManager) sendCurrentData(s *melody.Session) {
	msgData, err := fwsm.buildServerStatusMessage()
	if err != nil {
		fwsm.logger.Errorf("构建服务器状态消息失败: %v", err)
		return
	}
	_ = s.Write(msgData) // 忽略写入错误，melody 会处理连接问题
}

// getAllServerData 获取所有服务器状态数据
func (fwsm *FrontendWebSocketManager) getAllServerData() []*model.RespServerInfo {
	// 获取所有服务器状态信息并转换为RespServerInfo
	var respServerInfos []*model.RespServerInfo
	for item := range fwsm.serverStatus.IterBuffered() {
		info := model.NewRespServerInfo(item.Val)
		// 检查是否在线
		isOnline := time.Now().Unix()-info.LastReportTime <= int64(fwsm.configAccess.GetConfig().ReportTimeIntervalMax)
		info.IsOnline = isOnline
		respServerInfos = append(respServerInfos, info)
	}

	return respServerInfos
}

// getClientIP 获取客户端IP
func (fwsm *FrontendWebSocketManager) getClientIP(s *melody.Session) string {
	// 尝试从X-Real-IP头获取
	if ip := s.Request.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}

	// 尝试从X-Forwarded-For头获取
	if ip := s.Request.Header.Get("X-Forwarded-For"); ip != "" {
		return ip
	}

	// 从RemoteAddr获取
	if ip := s.Request.RemoteAddr; ip != "" {
		return ip
	}

	return "unknown"
}

// GetStats 获取统计信息
func (fwsm *FrontendWebSocketManager) GetStats() map[string]interface{} {
	fwsm.mu.RLock()
	defer fwsm.mu.RUnlock()

	return map[string]interface{}{
		"active_connections":   len(fwsm.connections),
		"total_connections":    fwsm.totalConnections,
		"total_disconnections": fwsm.totalDisconnections,
		"total_messages":       fwsm.totalMessages,
	}
}

// Close 关闭前端WebSocket管理器
func (fwsm *FrontendWebSocketManager) Close() {
	fwsm.cancel()
	fwsm.pushTicker.Stop()
	_ = fwsm.melody.Close() // 忽略关闭错误，管理器即将销毁
	fwsm.logger.Info("前端WebSocket管理器已关闭")
}
