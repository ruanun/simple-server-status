# 数据流向

> **作者**: ruan
> **最后更新**: 2025-11-07

## 概述

本文档描述 SimpleServerStatus 系统中数据的完整流转过程，从数据采集、传输、存储到展示的全链路。

## 完整数据流向

```
┌──────────────────────────────────────────────────────────────────┐
│                        数据流转全链路                             │
└──────────────────────────────────────────────────────────────────┘

1. 数据采集 (Agent)
   gopsutil → getXXXInfo → model.XXXInfo

2. 数据编码
   JSON Marshal (使用内存池优化)

3. 数据传输
   sendChan → sendLoop → WebSocket (/ws-report)

4. 数据接收 (Dashboard)
   Melody → JSON Unmarshal → 验证

5. 数据存储
   ConcurrentMap (内存存储)

6. 数据广播
   Broadcast → WebSocket (/ws-frontend)

7. 数据展示 (Web)
   onmessage → Vue State → DOM Update
```

## 1. 数据采集阶段

### 采集流程

**位置**: `internal/agent/gopsutil.go`, `internal/agent/report.go`

```go
// 定时采集循环
func reportInfo(ctx context.Context) {
    ticker := time.NewTicker(5 * time.Second)  // 默认 5 秒采集一次
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            // 采集各项指标
            serverInfo := &model.ServerInfo{
                ServerId:   global.AgentConfig.ServerId,
                ServerName: getHostname(),
                CPU:        getCpuInfo(),      // CPU 信息
                Memory:     getMemInfo(),      // 内存信息
                Disk:       getDiskInfo(),     // 磁盘信息
                Network:    getNetInfo(),      // 网络信息
                System:     getSystemInfo(),   // 系统信息
                Timestamp:  time.Now().Unix(),
            }

            // 发送数据
            wsClient.SendJsonMsg(serverInfo)
        }
    }
}
```

### 采集的数据类型

1. **CPU 信息** - 使用率、核心数、型号
2. **内存信息** - 总量、已用、可用、使用率
3. **磁盘信息** - 容量、使用率、读写速度
4. **网络信息** - 流量、上传/下载速度
5. **系统信息** - 主机名、操作系统、架构、运行时间

所有数据使用 `gopsutil` 库采集。

### 性能优化

#### 自适应采集频率

**位置**: `internal/agent/adaptive.go`

```go
// 根据 CPU 使用率动态调整采集频率
func (ac *AdaptiveCollector) AdjustInterval(cpuPercent float64) time.Duration {
    if cpuPercent > 70 {
        return 30 * time.Second  // 高负载，降低频率
    } else if cpuPercent < 30 {
        return 2 * time.Second   // 低负载，提高频率
    }
    return 5 * time.Second      // 正常频率
}
```

#### 内存池优化

**位置**: `internal/agent/mempool.go`

```go
// JSON 序列化使用内存池，减少内存分配
func OptimizedJSONMarshal(v interface{}) ([]byte, error) {
    buf := GlobalMemoryPool.GetBuffer()
    defer GlobalMemoryPool.PutBuffer(buf)

    encoder := json.NewEncoder(buf)
    err := encoder.Encode(v)
    return append([]byte(nil), buf.Bytes()...), err
}
```

## 2. 数据传输阶段

### 发送队列机制

**位置**: `internal/agent/ws.go`

```go
type WsClient struct {
    sendChan  chan []byte  // 缓冲 100 条消息
    conn      *websocket.Conn
}

// 非阻塞发送
func (c *WsClient) SendJsonMsg(v interface{}) error {
    data, err := OptimizedJSONMarshal(v)
    if err != nil {
        return err
    }

    select {
    case c.sendChan <- data:
        return nil
    default:
        return errors.New("send queue full")
    }
}

// 发送循环（独立 goroutine）
func (c *WsClient) sendLoop(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        case msg := <-c.sendChan:
            c.conn.WriteMessage(websocket.TextMessage, msg)
        }
    }
}
```

### 消息格式

**JSON 示例**:

```json
{
  "serverId": "server-1",
  "serverName": "Web Server 1",
  "cpu": {
    "percent": 45.2,
    "cores": 8
  },
  "memory": {
    "total": 16777216000,
    "used": 8388608000,
    "usedPercent": 50.0
  },
  "disk": [{
    "path": "/",
    "total": 107374182400,
    "used": 53687091200,
    "usedPercent": 50.0
  }],
  "network": {
    "interfaceName": "eth0",
    "inSpeed": 1048576,
    "outSpeed": 524288
  },
  "system": {
    "hostname": "web-server-1",
    "os": "linux",
    "platform": "ubuntu",
    "uptime": 864000
  },
  "timestamp": 1699123456
}
```

## 3. 数据接收和存储阶段

### Dashboard 接收流程

**位置**: `internal/dashboard/websocket_manager.go`

```go
// Melody 处理消息
func (wm *WebSocketManager) HandleMessage(s *melody.Session, msg []byte) {
    // 1. 反序列化
    var serverInfo model.ServerInfo
    if err := json.Unmarshal(msg, &serverInfo); err != nil {
        return
    }

    // 2. 验证数据
    if err := wm.validateServerInfo(&serverInfo); err != nil {
        return
    }

    // 3. 存储到内存
    wm.saveServerInfo(&serverInfo)

    // 4. 广播到前端
    wm.broadcastToFrontend(&serverInfo)

    // 5. 更新统计
    wm.updateStats(s, len(msg))
}
```

### 内存存储

**位置**: `internal/dashboard/global/global.go`

```go
// 使用并发安全的 Map 存储
var ServerStatusInfoMap = cmap.New[*model.ServerInfo]()

// 保存数据
func (wm *WebSocketManager) saveServerInfo(info *model.ServerInfo) {
    ServerStatusInfoMap.Set(info.ServerId, info)
}

// 查询数据
func GetServerInfo(serverID string) (*model.ServerInfo, bool) {
    return ServerStatusInfoMap.Get(serverID)
}
```

### 连接状态跟踪

```go
type WebSocketManager struct {
    sessionToServer cmap.ConcurrentMap  // Session → ServerID
    serverToSession cmap.ConcurrentMap  // ServerID → Session
    connections     cmap.ConcurrentMap  // 连接信息
}

type ConnectionInfo struct {
    ServerID      string
    ConnectAt     time.Time
    LastSeen      time.Time
    MessageCount  int64
    BytesReceived int64
}
```

## 4. 数据广播阶段

### 前端 WebSocket 管理

**位置**: `internal/dashboard/frontend_websocket_manager.go`

```go
// 广播到所有前端连接
func (fwm *FrontendWebSocketManager) BroadcastServerInfo(info *model.ServerInfo) {
    data, err := json.Marshal(info)
    if err != nil {
        return
    }

    fwm.mu.RLock()
    defer fwm.mu.RUnlock()

    for conn := range fwm.connections {
        go func(c *websocket.Conn) {
            c.WriteMessage(websocket.TextMessage, data)
        }(conn)
    }
}
```

### 触发时机

```
Agent 上报数据
     ↓
Dashboard 接收
     ↓
立即广播到前端  ←─ 实时性保证
     ↓
所有前端连接收到更新
```

## 5. 前端展示阶段

### WebSocket 客户端

**位置**: `web/src/api/websocket.ts`

```typescript
class WebSocketClient {
    private ws: WebSocket | null = null;

    connect(url: string) {
        this.ws = new WebSocket(url);

        this.ws.onmessage = (event) => {
            const data = JSON.parse(event.data);
            // 更新 Vue 状态
            store.updateServerInfo(data);
        };
    }
}
```

### Vue 状态更新

**位置**: `web/src/stores/serverStore.ts`

```typescript
export const useServerStore = defineStore('server', {
    state: () => ({
        servers: new Map<string, ServerInfo>(),
    }),

    actions: {
        updateServerInfo(info: ServerInfo) {
            // 更新服务器信息，触发响应式更新
            this.servers.set(info.serverId, info);
        },

        getAllServers() {
            return Array.from(this.servers.values());
        },
    },
});
```

### 界面渲染

**位置**: `web/src/components/ServerCard.vue`

```vue
<template>
  <div class="server-card">
    <h3>{{ server.serverName }}</h3>
    <div class="stats">
      <div>CPU: {{ server.cpu.percent.toFixed(1) }}%</div>
      <div>内存: {{ server.memory.usedPercent.toFixed(1) }}%</div>
    </div>
  </div>
</template>

<script setup lang="ts">
const store = useServerStore();
const server = computed(() => store.getServer(props.serverId));
</script>
```

## 数据流时序图

```
时间线 →

T0: Agent 采集数据 (gopsutil)
    ↓ <1ms
T1: 数据序列化 (JSON)
    ↓ <1ms
T2: 加入发送队列 (sendChan)
    ↓ <1ms
T3: WebSocket 发送 (/ws-report)
    ↓ 网络延迟 (1-50ms)
T4: Dashboard 接收 (Melody)
    ↓ <1ms
T5: 数据反序列化和验证
    ↓ <1ms
T6: 存储到内存 (ConcurrentMap)
    ↓ <1ms
T7: 广播到前端 (/ws-frontend)
    ↓ 网络延迟 (1-50ms)
T8: 前端接收并更新状态
    ↓ <1ms
T9: Vue 响应式渲染
    ↓ <16ms (60fps)

总延迟: 20-120ms（端到端）
```

## 数据一致性保证

### 时间戳机制

```go
// Agent 发送时添加时间戳
serverInfo.Timestamp = time.Now().Unix()

// Dashboard 接收时检查时间戳
if time.Now().Unix() - serverInfo.Timestamp > 60 {
    log.Warn("数据过期")
}
```

### 顺序保证

WebSocket 是有序传输协议，消息按发送顺序到达，不需要额外的序列号。

### 数据校验

```go
func (wm *WebSocketManager) validateServerInfo(info *model.ServerInfo) error {
    if info.ServerId == "" {
        return errors.New("serverId 不能为空")
    }
    if info.Timestamp == 0 {
        return errors.New("timestamp 不能为空")
    }
    return nil
}
```

## 性能指标

### 典型性能

| 指标 | 数值 |
|------|------|
| **采集频率** | 5 秒/次（可配置） |
| **单次采集耗时** | <100ms |
| **JSON 序列化** | <1ms |
| **WebSocket 发送** | <1ms（本地队列） |
| **网络延迟** | 1-50ms（取决于网络） |
| **Dashboard 处理** | <5ms |
| **前端渲染** | <16ms (60fps) |
| **端到端延迟** | 20-120ms |

### 吞吐量

```
单个 Agent:
  - 采集频率: 5秒/次
  - 消息大小: ~2KB
  - 吞吐量: 0.4 KB/s

100 个 Agent:
  - 总吞吐量: 40 KB/s
  - Dashboard CPU: <5%
  - Dashboard 内存: <50MB
```

## 相关文档

- [架构概览](./overview.md) - 系统整体架构
- [WebSocket 通信设计](./websocket.md) - WebSocket 详细设计
- [API 文档](../api/websocket-api.md) - 消息格式规范

---

**版本**: 2.0
**作者**: ruan
**最后更新**: 2025-11-07
