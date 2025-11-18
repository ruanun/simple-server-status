package internal

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ruanun/simple-server-status/internal/agent/config"
	"go.uber.org/zap"
)

const (
	// retryCountMax WebSocket 客户端最大重试次数
	retryCountMax = 999
)

type WsClient struct {
	// 服务器地址
	ServerAddr string
	// 认证头
	AuthHeader http.Header
	// 重连次数
	RetryCountMax int
	// 链接
	conn *websocket.Conn
	// 连接状态管理
	connected    bool
	connMutex    sync.RWMutex
	reconnecting bool
	// 心跳管理
	heartbeatInterval time.Duration
	heartbeatTimeout  time.Duration
	lastPong          time.Time
	// 上下文管理
	ctx    context.Context
	cancel context.CancelFunc
	// 发送队列
	sendChan chan []byte
	closed   bool // channel 关闭标志,防止重复关闭
	// 连接统计
	connectionCount   int64
	reconnectionCount int64
	messagesSent      int64
	messagesReceived  int64
	// 依赖注入（移除全局变量）
	logger       *zap.SugaredLogger
	config       *config.AgentConfig
	errorHandler *ErrorHandler
	memoryPool   *MemoryPoolManager
	monitor      *PerformanceMonitor
}

func NewWsClient(
	cfg *config.AgentConfig,
	logger *zap.SugaredLogger,
	errorHandler *ErrorHandler,
	memoryPool *MemoryPoolManager,
	monitor *PerformanceMonitor,
) *WsClient {
	var AuthHeader = make(http.Header)
	AuthHeader.Add("X-AUTH-SECRET", cfg.AuthSecret)
	AuthHeader.Add("X-SERVER-ID", cfg.ServerId)

	ctx, cancel := context.WithCancel(context.Background())

	return &WsClient{
		AuthHeader:        AuthHeader,
		RetryCountMax:     retryCountMax,
		ServerAddr:        cfg.ServerAddr,
		connected:         false,
		reconnecting:      false,
		heartbeatInterval: time.Second * 30, // 30秒心跳间隔
		heartbeatTimeout:  time.Second * 45, // 45秒心跳超时
		ctx:               ctx,
		cancel:            cancel,
		sendChan:          make(chan []byte, 100), // 缓冲100条消息
		logger:            logger,
		config:            cfg,
		errorHandler:      errorHandler,
		memoryPool:        memoryPool,
		monitor:           monitor,
	}
}

// 返回下一次重试的等待时间（指数衰减算法）
func retryDelay(retryCount int) time.Duration {
	minDelay := 3 * time.Second
	maxDelay := 10 * time.Minute
	factor := 1.2

	delay := time.Duration(float64(minDelay) * math.Pow(factor, float64(retryCount)))
	if delay > maxDelay {
		delay = maxDelay
	}
	return delay
}

func (c *WsClient) CloseWs() {
	// 关闭WebSocket连接
	err := c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	if err != nil {
		// 使用统一错误处理
		closeErr := NewAppError(ErrorTypeNetwork, SeverityMedium, "WebSocket关闭失败", err)
		c.errorHandler.HandleError(closeErr)
		return
	}
	_ = c.conn.Close() // 忽略关闭错误，连接已经在关闭过程中
}

func (c *WsClient) SendJsonMsg(obj interface{}) {
	// 检查客户端是否已关闭
	c.connMutex.RLock()
	if c.closed {
		c.connMutex.RUnlock()
		return // 已关闭,直接返回,避免向已关闭的 channel 发送数据
	}
	c.connMutex.RUnlock()

	data, err := c.memoryPool.OptimizedJSONMarshal(obj)
	if err != nil {
		// 使用统一错误处理
		jsonErr := NewAppError(ErrorTypeData, SeverityMedium, "JSON序列化失败", err)
		c.errorHandler.HandleError(jsonErr)
		return
	}

	select {
	case c.sendChan <- data:
		// 消息已加入发送队列
	case <-time.After(time.Second * 5):
		// 使用统一错误处理
		timeoutErr := NewAppError(ErrorTypeNetwork, SeverityMedium, "发送队列已满，消息发送超时", nil)
		c.errorHandler.HandleError(timeoutErr)
	}
}

// Start 启动WebSocket客户端
func (c *WsClient) Start() {
	go c.connectLoop()
	go c.sendLoop()
	go c.heartbeatLoop()
}

// connectLoop 连接循环，处理自动重连
func (c *WsClient) connectLoop() {
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			if !c.IsConnected() {
				c.attemptConnection()
			}
			time.Sleep(time.Second * 5) // 每5秒检查一次连接状态
		}
	}
}

// attemptConnection 尝试建立连接
func (c *WsClient) attemptConnection() {
	c.connMutex.Lock()
	if c.reconnecting {
		c.connMutex.Unlock()
		return
	}
	c.reconnecting = true
	c.connMutex.Unlock()

	defer func() {
		c.connMutex.Lock()
		c.reconnecting = false
		c.connMutex.Unlock()
	}()

	c.logger.Info("开始尝试连接服务器...")
	c.logger.Info("服务器地址：", c.ServerAddr)

	retryCount := 0
	for retryCount <= c.RetryCountMax {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		// 尝试建立WebSocket连接
		conn, _, err := websocket.DefaultDialer.Dial(c.ServerAddr, c.AuthHeader)
		if err == nil {
			c.setConnection(conn)
			c.logger.Info("连接成功")
			c.connectionCount++
			if c.connectionCount > 1 {
				c.reconnectionCount++
			}
			// 启动消息处理
			go c.handleMessage()
			return
		}

		// 连接失败，等待重试
		delay := retryDelay(retryCount)
		// 使用统一错误处理
		connErr := NewAppError(ErrorTypeNetwork, SeverityMedium,
			fmt.Sprintf("WebSocket连接失败 (将在%.1fs后重试)", delay.Seconds()), err)
		c.errorHandler.HandleError(connErr)
		retryCount++

		if retryCount > c.RetryCountMax {
			// 使用统一错误处理
			maxRetryErr := NewAppError(ErrorTypeNetwork, SeverityHigh, "WebSocket连接失败: 超过最大重试次数", nil)
			c.errorHandler.HandleError(maxRetryErr)
			return
		}

		select {
		case <-c.ctx.Done():
			return
		case <-time.After(delay):
			continue
		}
	}
}

// setConnection 设置连接
func (c *WsClient) setConnection(conn *websocket.Conn) {
	c.connMutex.Lock()
	defer c.connMutex.Unlock()

	// 设置pong处理器
	conn.SetPongHandler(func(appData string) error {
		c.connMutex.Lock()
		c.lastPong = time.Now()
		c.connMutex.Unlock()
		return nil
	})

	if c.conn != nil {
		_ = c.conn.Close() // 忽略关闭错误，连接即将被替换
	}

	c.conn = conn
	c.connected = true
	c.lastPong = time.Now() // 初始化lastPong时间

	// 设置pong处理器
	c.conn.SetPongHandler(func(string) error {
		c.connMutex.Lock()
		c.lastPong = time.Now()
		c.connMutex.Unlock()
		return nil
	})
}

// IsConnected 检查连接状态
func (c *WsClient) IsConnected() bool {
	c.connMutex.RLock()
	defer c.connMutex.RUnlock()
	return c.connected
}

// sendLoop 发送循环
func (c *WsClient) sendLoop() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case data := <-c.sendChan:
			c.sendMessage(data)
		}
	}
}

// sendMessage 发送消息
func (c *WsClient) sendMessage(data []byte) {
	c.connMutex.RLock()
	conn := c.conn
	connected := c.connected
	c.connMutex.RUnlock()

	if !connected || conn == nil {
		// 使用统一错误处理
		noConnErr := NewAppError(ErrorTypeNetwork, SeverityMedium, "连接未建立，消息发送失败", nil)
		c.errorHandler.HandleError(noConnErr)
		return
	}

	err := conn.WriteMessage(websocket.TextMessage, data)
	if err != nil {
		// 使用统一错误处理
		sendErr := NewAppError(ErrorTypeNetwork, SeverityMedium, "发送消息失败", err)
		c.errorHandler.HandleError(sendErr)
		c.markDisconnected()
		c.monitor.IncrementError()
		return
	}

	c.messagesSent++
	// 记录WebSocket消息发送事件
	c.monitor.IncrementWebSocketMessage()
}

// markDisconnected 标记为断开连接
func (c *WsClient) markDisconnected() {
	c.connMutex.Lock()
	defer c.connMutex.Unlock()
	c.connected = false
	if c.conn != nil {
		_ = c.conn.Close() // 忽略关闭错误，连接即将被置空
		c.conn = nil
	}
}

// heartbeatLoop 心跳循环
func (c *WsClient) heartbeatLoop() {
	ticker := time.NewTicker(c.heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.sendHeartbeat()
			c.checkHeartbeat()
		}
	}
}

// sendHeartbeat 发送心跳
func (c *WsClient) sendHeartbeat() {
	c.connMutex.RLock()
	conn := c.conn
	connected := c.connected
	c.connMutex.RUnlock()

	if !connected || conn == nil {
		return
	}

	err := conn.WriteMessage(websocket.PingMessage, []byte{})
	if err != nil {
		// 使用统一错误处理
		heartbeatErr := NewAppError(ErrorTypeNetwork, SeverityMedium, "发送心跳失败", err)
		c.errorHandler.HandleError(heartbeatErr)
		c.markDisconnected()
	}
}

// checkHeartbeat 检查心跳超时
func (c *WsClient) checkHeartbeat() {
	c.connMutex.RLock()
	lastPong := c.lastPong
	connected := c.connected
	c.connMutex.RUnlock()

	if connected && time.Since(lastPong) > c.heartbeatTimeout {
		c.logger.Warn("心跳超时，断开连接")
		c.markDisconnected()
	}
}

// handleMessage 处理接收到的消息
func (c *WsClient) handleMessage() {
	for {
		c.connMutex.RLock()
		conn := c.conn
		connected := c.connected
		c.connMutex.RUnlock()

		if !connected || conn == nil {
			return
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				// 使用统一错误处理
				readErr := NewAppError(ErrorTypeNetwork, SeverityMedium, "WebSocket读取错误", err)
				c.errorHandler.HandleError(readErr)
			}
			c.markDisconnected()
			return
		}

		c.messagesReceived++
		c.logger.Debug("收到消息:", string(message))
	}
}

// GetStats 获取连接统计信息
func (c *WsClient) GetStats() map[string]int64 {
	c.connMutex.RLock()
	defer c.connMutex.RUnlock()
	return map[string]int64{
		"connections":       c.connectionCount,
		"reconnections":     c.reconnectionCount,
		"messages_sent":     c.messagesSent,
		"messages_received": c.messagesReceived,
	}
}

// Close 关闭WebSocket客户端
func (c *WsClient) Close() {
	c.connMutex.Lock()
	// 检查是否已关闭,避免重复关闭
	if c.closed {
		c.connMutex.Unlock()
		return
	}
	c.closed = true
	c.connMutex.Unlock()

	// 1. 先发送取消信号,通知所有 goroutine 停止
	c.cancel()

	// 2. 等待一小段时间让 goroutine 处理取消信号
	//    这确保 sendLoop 能够正常退出,不会在 channel 关闭后继续读取
	time.Sleep(time.Millisecond * 100)

	// 3. 标记连接断开
	c.markDisconnected()

	// 4. 安全关闭 sendChan
	//    此时 sendLoop 应该已经退出,不会再从 channel 读取
	close(c.sendChan)
}
