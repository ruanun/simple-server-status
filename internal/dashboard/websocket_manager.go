package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/olahol/melody"
	"github.com/ruanun/simple-server-status/internal/dashboard/config"
	"github.com/ruanun/simple-server-status/internal/dashboard/global/constant"
	"github.com/ruanun/simple-server-status/pkg/model"
)

// ServerConfigProvider 服务器配置提供者接口
type ServerConfigProvider interface {
	Get(key string) (*config.ServerConfig, bool)
}

// ServerStatusProvider 服务器状态提供者接口
type ServerStatusProvider interface {
	Set(key string, val *model.ServerInfo)
}

// ConfigAccessor 配置访问器接口
type ConfigAccessor interface {
	GetConfig() *config.DashboardConfig
}

// ConnectionStatus 连接状态
type ConnectionStatus int

const (
	Connected ConnectionStatus = iota
	Disconnected
	Reconnecting
)

// ConnectionInfo 连接信息
type ConnectionInfo struct {
	ServerID      string           `json:"server_id"`
	Session       *melody.Session  `json:"-"`
	Status        ConnectionStatus `json:"status"`
	ConnectedAt   time.Time        `json:"connected_at"`
	LastHeartbeat time.Time        `json:"last_heartbeat"`
	LastMessage   time.Time        `json:"last_message"`
	IP            string           `json:"ip"`
	MessageCount  int64            `json:"message_count"`
	ErrorCount    int64            `json:"error_count"`
}

// WebSocketManager Agent 端 WebSocket 管理器
// 用于管理 Agent 到 Dashboard 的连接，接收服务器状态上报数据
type WebSocketManager struct {
	mu                sync.RWMutex
	connections       map[string]*ConnectionInfo // serverID -> ConnectionInfo
	sessions          map[*melody.Session]string // session -> serverID
	melody            *melody.Melody
	ctx               context.Context
	cancel            context.CancelFunc
	heartbeatInterval time.Duration
	heartbeatTimeout  time.Duration
	maxMessageSize    int64
	logger            interface {
		Infof(string, ...interface{})
		Warnf(string, ...interface{})
		Debugf(string, ...interface{})
		Info(...interface{})
	}
	errorHandler *ErrorHandler

	// 数据访问
	serverConfigs ServerConfigProvider
	serverStatus  ServerStatusProvider
	configAccess  ConfigAccessor

	// 统计信息
	totalConnections    int64
	totalDisconnections int64
	totalMessages       int64
	totalErrors         int64
}

// NewWebSocketManager 创建新的WebSocket管理器
func NewWebSocketManager(logger interface {
	Infof(string, ...interface{})
	Warnf(string, ...interface{})
	Debugf(string, ...interface{})
	Info(...interface{})
}, errorHandler *ErrorHandler, serverConfigs ServerConfigProvider, serverStatus ServerStatusProvider, configAccess ConfigAccessor) *WebSocketManager {
	ctx, cancel := context.WithCancel(context.Background())
	m := melody.New()
	m.Config.MaxMessageSize = 1024 * 10 // 10KB

	wsm := &WebSocketManager{
		connections:       make(map[string]*ConnectionInfo),
		sessions:          make(map[*melody.Session]string),
		melody:            m,
		ctx:               ctx,
		cancel:            cancel,
		heartbeatInterval: time.Second * 30, // 30秒心跳间隔
		heartbeatTimeout:  time.Second * 60, // 60秒心跳超时
		maxMessageSize:    1024 * 10,
		logger:            logger,
		errorHandler:      errorHandler,
		serverConfigs:     serverConfigs,
		serverStatus:      serverStatus,
		configAccess:      configAccess,
	}

	// 设置melody事件处理器
	m.HandleConnect(wsm.handleConnect)
	m.HandleMessage(wsm.handleMessage)
	m.HandleDisconnect(wsm.handleDisconnect)
	m.HandleError(wsm.handleError)
	// 添加ping/pong处理
	m.HandlePong(wsm.handlePong)

	// 启动心跳检测
	go wsm.heartbeatLoop()

	return wsm
}

// SetupRoutes 设置WebSocket路由
func (wsm *WebSocketManager) SetupRoutes(r *gin.Engine) {
	r.GET(wsm.configAccess.GetConfig().WebSocketPath, func(c *gin.Context) {
		secret := c.GetHeader(constant.HeaderSecret)
		serverID := c.GetHeader(constant.HeaderId)

		if wsm.authenticate(secret, serverID) {
			_ = wsm.melody.HandleRequest(c.Writer, c.Request) // 忽略错误，melody 已经处理了响应
		} else {
			wsm.logger.Warnf("未授权连接尝试 - ServerID: %s, IP: %s", serverID, c.ClientIP())
			c.JSON(401, gin.H{"error": "未授权连接"})
		}
	})
}

// handleConnect 处理连接事件
func (wsm *WebSocketManager) handleConnect(s *melody.Session) {
	secret := s.Request.Header.Get(constant.HeaderSecret)
	serverID := s.Request.Header.Get(constant.HeaderId)

	if !wsm.authenticate(secret, serverID) {
		// 记录认证错误
		authErr := NewAuthenticationError("WebSocket连接认证失败", fmt.Sprintf("ServerID: %s", serverID))
		authErr.IP = wsm.getClientIP(s)
		if wsm.errorHandler != nil {
			wsm.errorHandler.RecordError(authErr)
		}

		_ = s.CloseWithMsg([]byte("未授权连接")) // 忽略关闭错误，连接将被断开
		return
	}

	ip := wsm.getClientIP(s)
	now := time.Now()

	wsm.mu.Lock()
	defer wsm.mu.Unlock()

	// 如果已存在连接，先关闭旧连接
	if oldConn, exists := wsm.connections[serverID]; exists {
		wsm.logger.Infof("服务器 %s 重新连接，关闭旧连接", serverID)
		if oldConn.Session != nil {
			_ = oldConn.Session.Close() // 忽略关闭错误，连接即将被替换
		}
		delete(wsm.sessions, oldConn.Session)
	}

	// 创建新连接信息
	connInfo := &ConnectionInfo{
		ServerID:      serverID,
		Session:       s,
		Status:        Connected,
		ConnectedAt:   now,
		LastHeartbeat: now,
		LastMessage:   now,
		IP:            ip,
		MessageCount:  0,
		ErrorCount:    0,
	}

	wsm.connections[serverID] = connInfo
	wsm.sessions[s] = serverID
	wsm.totalConnections++

	wsm.logger.Infof("服务器连接成功 - ServerID: %s, IP: %s", serverID, ip)
}

// handleMessage 处理消息事件
func (wsm *WebSocketManager) handleMessage(s *melody.Session, msg []byte) {
	wsm.mu.Lock()
	serverID, exists := wsm.sessions[s]
	if !exists {
		wsm.mu.Unlock()
		// 记录未知会话错误
		unknownErr := NewWebSocketError("收到未知会话的消息", "会话未在管理器中注册")
		if wsm.errorHandler != nil {
			wsm.errorHandler.RecordError(unknownErr)
		}
		return
	}

	connInfo := wsm.connections[serverID]
	connInfo.LastMessage = time.Now()
	connInfo.MessageCount++
	wsm.totalMessages++
	wsm.mu.Unlock()

	// 解析服务器状态信息
	var serverStatusInfo model.ServerInfo
	err := json.Unmarshal(msg, &serverStatusInfo)
	if err != nil {
		// 记录消息格式错误
		msgErr := NewValidationError("WebSocket消息格式错误", fmt.Sprintf("ServerID: %s, Error: %v", serverID, err))
		if wsm.errorHandler != nil {
			wsm.errorHandler.RecordError(msgErr)
		}
		wsm.incrementErrorCount(serverID)
		return
	}

	// 获取服务器配置信息
	server, exists := wsm.serverConfigs.Get(serverID)
	if !exists {
		// 记录配置未找到错误
		configErr := NewConfigError("未找到服务器配置", fmt.Sprintf("ServerID: %s", serverID))
		if wsm.errorHandler != nil {
			wsm.errorHandler.RecordError(configErr)
		}
		wsm.incrementErrorCount(serverID)
		return
	}

	// 更新服务器状态信息
	serverStatusInfo.Name = server.Name
	serverStatusInfo.Group = server.Group
	serverStatusInfo.Id = server.Id
	serverStatusInfo.LastReportTime = time.Now().Unix()
	if server.CountryCode != "" {
		serverStatusInfo.Loc = server.CountryCode
	}
	serverStatusInfo.Loc = strings.ToLower(serverStatusInfo.Loc)

	// 存储到全局状态映射
	wsm.serverStatus.Set(serverID, &serverStatusInfo)
}

// handleDisconnect 处理断开连接事件
func (wsm *WebSocketManager) handleDisconnect(s *melody.Session) {
	// 使用写锁保护所有读写操作
	wsm.mu.Lock()
	serverID, exists := wsm.sessions[s]
	if !exists {
		wsm.mu.Unlock()
		return
	}

	connInfo := wsm.connections[serverID]
	connInfo.Status = Disconnected

	// 保存日志需要的信息
	ip := connInfo.IP

	// 删除连接
	delete(wsm.sessions, s)
	delete(wsm.connections, serverID)

	// 更新统计信息
	wsm.totalDisconnections++
	wsm.mu.Unlock()

	wsm.logger.Infof("服务器断开连接 - ServerID: %s, IP: %s", serverID, ip)
}

// handleError 处理错误事件
func (wsm *WebSocketManager) handleError(s *melody.Session, err error) {
	wsm.mu.RLock()
	serverID, exists := wsm.sessions[s]
	wsm.mu.RUnlock()

	if exists {
		// 记录已知会话的WebSocket错误
		wsErr := NewWebSocketError("WebSocket连接错误", fmt.Sprintf("ServerID: %s, Error: %v", serverID, err))
		wsErr.IP = wsm.getClientIP(s)
		if wsm.errorHandler != nil {
			wsm.errorHandler.RecordError(wsErr)
		}
		wsm.incrementErrorCount(serverID)
	} else {
		// 记录未知会话的WebSocket错误
		unknownErr := NewWebSocketError("WebSocket未知会话错误", fmt.Sprintf("Error: %v", err))
		if wsm.errorHandler != nil {
			wsm.errorHandler.RecordError(unknownErr)
		}
	}

	wsm.mu.Lock()
	wsm.totalErrors++
	wsm.mu.Unlock()
}

// handlePong 处理pong消息
func (wsm *WebSocketManager) handlePong(s *melody.Session) {
	wsm.mu.RLock()
	serverID, exists := wsm.sessions[s]
	wsm.mu.RUnlock()

	if !exists {
		return
	}

	wsm.mu.Lock()
	if connInfo, exists := wsm.connections[serverID]; exists {
		connInfo.LastHeartbeat = time.Now()
	}
	wsm.mu.Unlock()

	wsm.logger.Debugf("收到心跳响应 - ServerID: %s", serverID)
}

// heartbeatLoop 心跳检测循环
func (wsm *WebSocketManager) heartbeatLoop() {
	ticker := time.NewTicker(wsm.heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-wsm.ctx.Done():
			return
		case <-ticker.C:
			wsm.checkHeartbeats()
		}
	}
}

// checkHeartbeats 检查心跳超时
func (wsm *WebSocketManager) checkHeartbeats() {
	wsm.mu.Lock()
	defer wsm.mu.Unlock()

	now := time.Now()
	var timeoutSessions []*melody.Session

	for serverID, connInfo := range wsm.connections {
		if now.Sub(connInfo.LastMessage) > wsm.heartbeatTimeout {
			wsm.logger.Warnf("服务器心跳超时 - ServerID: %s, 最后消息时间: %v", serverID, connInfo.LastMessage)
			timeoutSessions = append(timeoutSessions, connInfo.Session)
		}
	}

	// 关闭超时的连接
	for _, session := range timeoutSessions {
		_ = session.Close() // 忽略关闭错误，会话即将被清理
	}
}

// authenticate 认证
func (wsm *WebSocketManager) authenticate(secret, serverID string) bool {
	if secret == "" || serverID == "" {
		return false
	}

	server, exists := wsm.serverConfigs.Get(serverID)
	if !exists {
		return false
	}

	return server.Secret == secret
}

// getClientIP 获取客户端IP
func (wsm *WebSocketManager) getClientIP(s *melody.Session) string {
	// 尝试从X-Real-IP头获取
	if ip := s.Request.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}

	// 尝试从X-Forwarded-For头获取
	if ip := s.Request.Header.Get("X-Forwarded-For"); ip != "" {
		parts := strings.Split(ip, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}

	// 从RemoteAddr获取
	if ip := s.Request.RemoteAddr; ip != "" {
		parts := strings.Split(ip, ":")
		if len(parts) > 0 {
			return parts[0]
		}
	}

	return "unknown"
}

// incrementErrorCount 增加错误计数
func (wsm *WebSocketManager) incrementErrorCount(serverID string) {
	wsm.mu.Lock()
	defer wsm.mu.Unlock()

	if connInfo, exists := wsm.connections[serverID]; exists {
		connInfo.ErrorCount++
	}
}

// GetConnectionInfo 获取连接信息
func (wsm *WebSocketManager) GetConnectionInfo(serverID string) (*ConnectionInfo, bool) {
	wsm.mu.RLock()
	defer wsm.mu.RUnlock()

	connInfo, exists := wsm.connections[serverID]
	if !exists {
		return nil, false
	}

	// 返回副本以避免并发问题
	copy := *connInfo
	copy.Session = nil // 不暴露session
	return &copy, true
}

// GetAllConnections 获取所有连接信息
func (wsm *WebSocketManager) GetAllConnections() map[string]*ConnectionInfo {
	wsm.mu.RLock()
	defer wsm.mu.RUnlock()

	result := make(map[string]*ConnectionInfo)
	for serverID, connInfo := range wsm.connections {
		copy := *connInfo
		copy.Session = nil // 不暴露session
		result[serverID] = &copy
	}

	return result
}

// GetStats 获取统计信息
func (wsm *WebSocketManager) GetStats() map[string]interface{} {
	wsm.mu.RLock()
	defer wsm.mu.RUnlock()

	return map[string]interface{}{
		"active_connections":   len(wsm.connections),
		"total_connections":    wsm.totalConnections,
		"total_disconnections": wsm.totalDisconnections,
		"total_messages":       wsm.totalMessages,
		"total_errors":         wsm.totalErrors,
	}
}

// BroadcastToServer 向特定服务器发送消息
func (wsm *WebSocketManager) BroadcastToServer(serverID string, message []byte) error {
	wsm.mu.RLock()
	connInfo, exists := wsm.connections[serverID]
	wsm.mu.RUnlock()

	if !exists || connInfo.Session == nil {
		return ErrServerNotConnected
	}

	return connInfo.Session.Write(message)
}

// BroadcastToAll 向所有连接的服务器广播消息
func (wsm *WebSocketManager) BroadcastToAll(message []byte) {
	_ = wsm.melody.Broadcast(message) // 忽略广播错误，melody 会处理连接问题
}

// Close 关闭WebSocket管理器
func (wsm *WebSocketManager) Close() {
	wsm.cancel()
	_ = wsm.melody.Close() // 忽略关闭错误，管理器即将销毁
	wsm.logger.Info("WebSocket管理器已关闭")
}

// 错误定义
var (
	ErrServerNotConnected = fmt.Errorf("服务器未连接")
)

// 为了兼容性，保留旧的SessionMgr接口
func (wsm *WebSocketManager) GetServerId(session *melody.Session) (string, bool) {
	wsm.mu.RLock()
	defer wsm.mu.RUnlock()
	serverID, exists := wsm.sessions[session]
	return serverID, exists
}

func (wsm *WebSocketManager) GetSession(serverID string) (*melody.Session, bool) {
	wsm.mu.RLock()
	defer wsm.mu.RUnlock()
	connInfo, exists := wsm.connections[serverID]
	if !exists {
		return nil, false
	}
	return connInfo.Session, true
}

func (wsm *WebSocketManager) SessionLength() int {
	wsm.mu.RLock()
	defer wsm.mu.RUnlock()
	return len(wsm.connections)
}

func (wsm *WebSocketManager) GetAllServerId() []string {
	wsm.mu.RLock()
	defer wsm.mu.RUnlock()

	var serverIDs []string
	for serverID := range wsm.connections {
		serverIDs = append(serverIDs, serverID)
	}
	return serverIDs
}

// 会话管理方法

// DelByServerId 通过服务器ID删除连接
func (wsm *WebSocketManager) DelByServerId(serverID string) {
	wsm.mu.Lock()
	defer wsm.mu.Unlock()

	if connInfo, exists := wsm.connections[serverID]; exists {
		if connInfo.Session != nil {
			_ = connInfo.Session.Close() // 忽略关闭错误，连接即将被清理
		}
		delete(wsm.connections, serverID)
		delete(wsm.sessions, connInfo.Session)
	}
}

// DelBySession 通过会话删除连接
func (wsm *WebSocketManager) DelBySession(session *melody.Session) {
	wsm.mu.Lock()
	defer wsm.mu.Unlock()

	if serverID, exists := wsm.sessions[session]; exists {
		delete(wsm.sessions, session)
		delete(wsm.connections, serverID)
	}
}

// ServerIdLength 获取当前连接的服务器数量（兼容性方法）
func (wsm *WebSocketManager) ServerIdLength() int {
	return wsm.SessionLength()
}
