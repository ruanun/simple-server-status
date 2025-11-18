# WebSocket 通信设计

> **作者**: ruan
> **最后更新**: 2025-11-05

## 概述

SimpleServerStatus 使用 **双通道 WebSocket** 架构实现实时通信：

1. **Agent 通道** (`/ws-report`): Agent 连接到 Dashboard 上报监控数据
2. **前端通道** (`/ws-frontend`): 浏览器连接接收实时数据推送

这种设计实现了数据采集、传输、展示的完全解耦。

## 架构设计

### 数据流向

```
┌────────────┐                ┌─────────────┐                ┌──────────┐
│   Agent    │   WebSocket    │  Dashboard  │   WebSocket    │   Web    │
│  (采集端)   │ ────────────►  │  (服务端)   │ ────────────►  │ (展示端)  │
└────────────┘     /ws-report └─────────────┘   /ws-frontend └──────────┘
       │                             │                             │
       │                             │                             │
    采集数据                      数据管理                     实时展示
   上报数据                      连接管理                     状态更新
   指数退避                      心跳检测                     断线重连
```

### 通道对比

| 特性 | Agent 通道 (`/ws-report`) | 前端通道 (`/ws-frontend`) |
|------|---------------------------|---------------------------|
| **用途** | Agent 上报监控数据 | 推送数据到前端展示 |
| **认证** | ✅ Header 认证 | ❌ 无需认证 |
| **实现库** | Melody | gorilla/websocket |
| **连接数** | 数十到数百 | 数个 |
| **消息频率** | 高频（秒级） | 高频（秒级） |
| **重连机制** | 指数退避算法 | 前端简单重连 |
| **心跳** | 30秒 ping，45秒超时 | 60秒无响应断开 |

## Agent 通道详解

### 连接流程

```
1. Agent 启动
   ↓
2. 创建 WebSocket 连接
   ↓
3. 发送认证信息（Header）
   - X-AUTH-SECRET: 认证密钥
   - X-SERVER-ID: 服务器 ID
   ↓
4. Dashboard 验证
   ├─ ✅ 验证通过 → 建立连接
   └─ ❌ 验证失败 → 断开连接
   ↓
5. 进入数据上报循环
   ↓
6. 心跳保持连接
```

### 认证机制

**Agent 端** (`internal/agent/ws.go`):

```go
// 连接时发送认证信息
headers := http.Header{
    "X-AUTH-SECRET": []string{c.authSecret},
    "X-SERVER-ID":   []string{c.serverID},
}

conn, _, err := websocket.DefaultDialer.Dial(c.serverAddr, headers)
```

**Dashboard 端** (`internal/dashboard/websocket_manager.go`):

```go
// 验证 Agent 认证信息
func (wm *WebSocketManager) HandleConnect(s *melody.Session) {
    // 从 HTTP 请求中获取认证信息
    authSecret := s.Request.Header.Get("X-AUTH-SECRET")
    serverID := s.Request.Header.Get("X-SERVER-ID")

    // 验证服务器配置
    server, exists := wm.getServerConfig(serverID)
    if !exists || server.Secret != authSecret {
        s.Close()
        return
    }

    // 建立连接映射
    wm.registerConnection(serverID, s)
}
```

### 重连机制

**指数退避算法** (`internal/agent/ws.go`):

```go
type BackoffConfig struct {
    InitialInterval time.Duration  // 初始重连间隔：3秒
    MaxInterval     time.Duration  // 最大重连间隔：10分钟
    Multiplier      float64        // 增长因子：1.2
    MaxRetries      int            // 最大重试次数：无限(-1)
}

// 计算下次重连间隔
nextInterval = currentInterval * Multiplier
if nextInterval > MaxInterval {
    nextInterval = MaxInterval
}
```

**重连流程**:

```
连接断开
   ↓
等待 3 秒 ────────► 重连失败
   ↓                    │
等待 3.6 秒 ◄───────────┘
   ↓
等待 4.32 秒
   ↓
...
   ↓
等待最多 10 分钟
```

### 心跳机制

**Agent 端**:

```go
// 每 30 秒发送一次 ping
ticker := time.NewTicker(30 * time.Second)
for {
    select {
    case <-ticker.C:
        c.conn.WriteMessage(websocket.PingMessage, nil)
    case <-c.ctx.Done():
        return
    }
}

// 设置读超时：45 秒
c.conn.SetReadDeadline(time.Now().Add(45 * time.Second))
```

**Dashboard 端**:

```go
// Melody 自动处理 pong 响应
melody.HandlePong(func(s *melody.Session) {
    // 更新最后活跃时间
    wm.updateLastSeen(s)
})

// 定期检查超时连接（60秒无响应断开）
ticker := time.NewTicker(30 * time.Second)
for range ticker.C {
    wm.checkTimeouts()
}
```

### Goroutine 管理

Agent WebSocket 客户端使用 **3 个独立的 goroutine**:

1. **connectLoop**: 连接和重连循环
2. **sendLoop**: 发送消息队列处理
3. **heartbeatLoop**: 心跳保持连接

所有 goroutine 支持 **Context 取消**，确保优雅退出：

```go
func (c *WsClient) Start() {
    c.ctx, c.cancel = context.WithCancel(context.Background())

    go c.connectLoop(c.ctx)    // 可取消
    go c.sendLoop(c.ctx)       // 可取消
    go c.heartbeatLoop(c.ctx)  // 可取消
}

func (c *WsClient) Close() {
    c.cancel()  // 触发所有 goroutine 退出
}
```

### 并发安全

**发送队列设计**:

```go
type WsClient struct {
    sendChan   chan []byte        // 缓冲 100 条消息
    connMutex  sync.RWMutex       // 保护连接状态
    connected  bool               // 连接状态标志
    closed     bool               // 关闭状态标志
}

// 发送消息（非阻塞）
func (c *WsClient) SendJsonMsg(v interface{}) error {
    // 检查状态
    c.connMutex.RLock()
    if c.closed || !c.connected {
        c.connMutex.RUnlock()
        return errors.New("connection not ready")
    }
    c.connMutex.RUnlock()

    // 非阻塞发送
    select {
    case c.sendChan <- data:
        return nil
    default:
        return errors.New("send queue full")
    }
}
```

### 优雅关闭

```go
func (c *WsClient) Close() {
    c.connMutex.Lock()
    if c.closed {
        c.connMutex.Unlock()
        return  // 防止重复关闭
    }
    c.closed = true
    c.connMutex.Unlock()

    // 1. 发送取消信号
    c.cancel()

    // 2. 等待 goroutine 退出
    time.Sleep(time.Millisecond * 100)

    // 3. 标记连接断开
    c.markDisconnected()

    // 4. 安全关闭 channel
    close(c.sendChan)
}
```

## 前端通道详解

### 连接流程

```
1. 浏览器访问 Dashboard
   ↓
2. 前端 JavaScript 创建 WebSocket 连接
   ws://dashboard-host:8900/ws-frontend
   ↓
3. Dashboard 接受连接（无需认证）
   ↓
4. Dashboard 将所有 Agent 数据推送到前端
   ↓
5. 前端实时更新界面
```

### 前端实现

**WebSocket 客户端** (`web/src/api/websocket.ts`):

```typescript
class WebSocketClient {
    private ws: WebSocket | null = null;
    private reconnectInterval = 3000;

    connect(url: string) {
        this.ws = new WebSocket(url);

        this.ws.onopen = () => {
            console.log('WebSocket 连接成功');
        };

        this.ws.onmessage = (event) => {
            const data = JSON.parse(event.data);
            this.handleMessage(data);
        };

        this.ws.onerror = (error) => {
            console.error('WebSocket 错误:', error);
        };

        this.ws.onclose = () => {
            console.log('WebSocket 断开，3秒后重连');
            setTimeout(() => this.connect(url), this.reconnectInterval);
        };
    }
}
```

### Dashboard 广播机制

**前端 WebSocket 管理器** (`internal/dashboard/frontend_websocket_manager.go`):

```go
// 广播到所有前端连接
func (fwm *FrontendWebSocketManager) BroadcastServerInfo(info *model.ServerInfo) {
    fwm.mu.RLock()
    defer fwm.mu.RUnlock()

    data, _ := json.Marshal(info)

    for conn := range fwm.connections {
        conn.WriteMessage(websocket.TextMessage, data)
    }
}
```

**触发时机**:

```go
// Agent 上报数据时，立即广播到前端
func (wm *WebSocketManager) HandleMessage(s *melody.Session, msg []byte) {
    var serverInfo model.ServerInfo
    json.Unmarshal(msg, &serverInfo)

    // 保存到内存
    wm.saveServerInfo(&serverInfo)

    // 广播到所有前端连接
    frontendWM.BroadcastServerInfo(&serverInfo)
}
```

## 性能优化

### Agent 端优化

**1. 内存池优化** (`internal/agent/mempool.go`):

```go
// 池化 bytes.Buffer，减少 GC 压力
var GlobalMemoryPool = NewMemoryPoolManager()

func OptimizedJSONMarshal(v interface{}) ([]byte, error) {
    buf := GlobalMemoryPool.GetBuffer()  // 从池中获取
    defer GlobalMemoryPool.PutBuffer(buf) // 归还到池

    encoder := json.NewEncoder(buf)
    err := encoder.Encode(v)
    return buf.Bytes(), err
}
```

**2. 自适应采集** (`internal/agent/adaptive.go`):

```go
// 根据系统负载动态调整采集频率
type AdaptiveCollector struct {
    baseInterval time.Duration  // 基础间隔：5秒
    minInterval  time.Duration  // 最小间隔：2秒
    maxInterval  time.Duration  // 最大间隔：30秒
}

// CPU 使用率高 → 降低采集频率
// CPU 使用率低 → 提高采集频率
```

**3. 并发安全的网络统计** (`internal/agent/network_stats.go`):

```go
type NetworkStatsCollector struct {
    mu           sync.RWMutex
    netInSpeed   uint64
    netOutSpeed  uint64
}

// Update 使用写锁
func (nsc *NetworkStatsCollector) Update() {
    nsc.mu.Lock()
    defer nsc.mu.Unlock()
    // 更新统计
}

// GetStats 使用读锁
func (nsc *NetworkStatsCollector) GetStats() (uint64, uint64) {
    nsc.mu.RLock()
    defer nsc.mu.RUnlock()
    return nsc.netInSpeed, nsc.netOutSpeed
}
```

### Dashboard 端优化

**1. 连接池管理**:

```go
// Melody 内置连接池
melody.Config{
    WriteWait:         10 * time.Second,
    PongWait:          60 * time.Second,
    PingPeriod:        54 * time.Second,
    MaxMessageSize:    512,
    MessageBufferSize: 256,
}
```

**2. 并发映射**:

```go
// 使用 concurrent-map 管理连接
type WebSocketManager struct {
    sessionToServer  cmap.ConcurrentMap[*melody.Session, string]
    serverToSession  cmap.ConcurrentMap[string, *melody.Session]
}
```

## 错误处理

### Agent 端错误处理

```go
// 统一错误处理器
type ErrorHandler struct {
    logger     *zap.SugaredLogger
    errorStats map[ErrorType]int
    errorHistory []ErrorRecord
}

// 按类型处理错误
func (eh *ErrorHandler) HandleError(err error, errType ErrorType, severity ErrorSeverity) {
    eh.logError(err, errType, severity)
    eh.updateStats(errType)
    eh.recordHistory(err, errType, severity)
}
```

### Dashboard 端错误处理

```go
// WebSocket 错误处理
melody.HandleError(func(s *melody.Session, err error) {
    log.Errorf("WebSocket 错误: %v", err)
    wm.handleConnectionError(s, err)
})

// 连接断开处理
melody.HandleDisconnect(func(s *melody.Session) {
    serverID := wm.getServerID(s)
    wm.unregisterConnection(serverID)
    log.Infof("Agent 断开: %s", serverID)
})
```

## 调试技巧

### 查看 WebSocket 通信

**Dashboard 日志**:
```bash
# 查看 Agent 连接状态
tail -f logs/dashboard.log | grep "WebSocket"

# 输出示例
INFO  WebSocket 连接建立: serverID=server-1
INFO  收到消息: serverID=server-1, size=1024
WARN  心跳超时: serverID=server-2
ERROR 认证失败: serverID=unknown, secret=invalid
```

**Agent 日志**:
```bash
# 查看连接和上报状态
tail -f logs/agent.log | grep "WebSocket"

# 输出示例
INFO  WebSocket 连接成功: ws://dashboard:8900/ws-report
INFO  发送消息: size=1024
WARN  连接断开，3秒后重连
INFO  重连成功，重试次数: 3
```

**前端浏览器控制台**:
```javascript
// 查看 WebSocket 连接状态
console.log('WebSocket 状态:', ws.readyState);
// 0: CONNECTING
// 1: OPEN
// 2: CLOSING
// 3: CLOSED

// 监听消息
ws.onmessage = (event) => {
    console.log('收到消息:', JSON.parse(event.data));
};
```

## 常见问题排查

### Agent 无法连接 Dashboard

**检查清单**:
1. ✅ serverAddr 配置正确（注意协议 ws:// 或 wss://）
2. ✅ serverId 和 authSecret 与 Dashboard 配置匹配
3. ✅ Dashboard 已启动且端口未被占用
4. ✅ 防火墙允许 WebSocket 连接
5. ✅ 查看 Dashboard 日志是否有认证失败信息

**错误示例**:
```
ERROR 连接失败: dial tcp: lookup dashboard: no such host
→ 检查 serverAddr 配置

ERROR 认证失败: invalid secret
→ 检查 authSecret 配置

ERROR 连接超时: dial tcp 192.168.1.100:8900: i/o timeout
→ 检查防火墙和网络连接
```

### 前端不显示数据

**检查清单**:
1. ✅ 浏览器控制台 WebSocket 连接状态（OPEN）
2. ✅ Agent 成功连接到 Dashboard
3. ✅ Dashboard 正确配置了 servers 列表
4. ✅ 前端 WebSocket URL 正确

**调试代码**:
```typescript
// 检查 WebSocket 连接
const ws = new WebSocket('ws://dashboard:8900/ws-frontend');
ws.onopen = () => console.log('连接成功');
ws.onmessage = (e) => console.log('收到数据:', e.data);
ws.onerror = (e) => console.error('连接错误:', e);
```

### 连接频繁断开

**可能原因**:
1. 网络不稳定
2. 心跳超时设置过短
3. Dashboard 负载过高
4. 代理服务器（Nginx、Caddy）超时设置

**Nginx 配置示例**:
```nginx
location /ws-report {
    proxy_pass http://dashboard:8900;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_read_timeout 86400;  # 24小时超时
}
```

## 相关文档

- [架构概览](./overview.md) - 系统整体架构
- [数据流向](./data-flow.md) - 数据流转过程
- [WebSocket API 文档](../api/websocket-api.md) - 消息格式说明

---

**版本**: 1.0
**作者**: ruan
**最后更新**: 2025-11-05
