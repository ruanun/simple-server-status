# WebSocket API 文档

> **作者**: ruan
> **最后更新**: 2025-11-05

## 概述

SimpleServerStatus 使用 WebSocket 实现实时双向通信。系统包含两个独立的 WebSocket 通道：

1. **Agent 通道** (`/ws-report`): Agent 上报监控数据到 Dashboard
2. **前端通道** (`/ws-frontend`): Dashboard 推送数据到前端展示

## Agent 通道 (/ws-report)

### 连接信息

- **URL**: `ws://dashboard-host:8900/ws-report`
- **协议**: WebSocket
- **认证**: HTTP Header 认证

### 连接方式

#### JavaScript

```javascript
const ws = new WebSocket('ws://localhost:8900/ws-report', {
  headers: {
    'X-AUTH-SECRET': 'your-secret-key',
    'X-SERVER-ID': 'your-server-id'
  }
});
```

#### Go

```go
import "github.com/gorilla/websocket"

headers := http.Header{
    "X-AUTH-SECRET": []string{"your-secret-key"},
    "X-SERVER-ID":   []string{"your-server-id"},
}

conn, _, err := websocket.DefaultDialer.Dial(
    "ws://localhost:8900/ws-report",
    headers,
)
```

#### curl (使用 websocat)

```bash
websocat ws://localhost:8900/ws-report \
  --header "X-AUTH-SECRET: your-secret-key" \
  --header "X-SERVER-ID: your-server-id"
```

### 认证

Agent 连接时必须在 HTTP Header 中提供认证信息：

| Header 名称 | 必填 | 说明 |
|-------------|------|------|
| X-AUTH-SECRET | 是 | 认证密钥，必须与 Dashboard 配置匹配 |
| X-SERVER-ID | 是 | 服务器ID，必须与 Dashboard 配置匹配 |

**认证失败**:

如果认证失败，连接将被立即关闭（Close Code: 1000）。

### 消息格式

#### Agent → Dashboard (上报数据)

**消息类型**: Text (JSON)

**完整消息示例**:

```json
{
  "serverId": "web-1",
  "serverName": "Web Server 1",
  "group": "production",
  "countryCode": "CN",
  "location": "Beijing, China",
  "ip": "123.45.67.89",
  "cpu": {
    "percent": 45.2,
    "cores": 8,
    "modelName": "Intel(R) Xeon(R) CPU E5-2680 v4"
  },
  "memory": {
    "total": 16777216000,
    "used": 8388608000,
    "available": 8388608000,
    "usedPercent": 50.0
  },
  "disk": [
    {
      "path": "/",
      "total": 107374182400,
      "used": 53687091200,
      "free": 53687091200,
      "usedPercent": 50.0,
      "readSpeed": 1048576,
      "writeSpeed": 524288
    }
  ],
  "network": {
    "interfaceName": "eth0",
    "bytesSent": 1073741824,
    "bytesRecv": 2147483648,
    "inSpeed": 1048576,
    "outSpeed": 524288
  },
  "system": {
    "hostname": "web-server-1",
    "os": "linux",
    "platform": "ubuntu",
    "arch": "amd64",
    "kernel": "5.15.0-58-generic",
    "uptime": 864000
  },
  "timestamp": 1699123456
}
```

**字段说明**:

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| serverId | string | 是 | 服务器ID |
| serverName | string | 是 | 服务器名称 |
| group | string | 否 | 分组名称 |
| countryCode | string | 否 | 国家代码（ISO 3166-1 alpha-2） |
| location | string | 否 | 地理位置描述 |
| ip | string | 否 | 公网IP地址 |
| cpu | CPUInfo | 是 | CPU信息 |
| memory | MemoryInfo | 是 | 内存信息 |
| disk | DiskInfo[] | 是 | 磁盘信息数组 |
| network | NetworkInfo | 是 | 网络信息 |
| system | SystemInfo | 是 | 系统信息 |
| timestamp | number | 是 | Unix时间戳（秒） |

#### Dashboard → Agent

Dashboard 不主动向 Agent 发送消息，只接收 Agent 上报的数据。

### 心跳机制

**Agent 端**:

- 每 30 秒发送一次 Ping 帧
- 如果 45 秒内未收到 Pong 响应，认为连接断开

**Dashboard 端**:

- 自动响应 Ping 帧（发送 Pong）
- 如果 60 秒内未收到任何消息（包括 Ping），断开连接

### 重连机制

**指数退避算法**:

```
初始间隔: 3 秒
最大间隔: 10 分钟
增长因子: 1.2

重连序列:
- 第1次: 3秒后重连
- 第2次: 3.6秒后重连
- 第3次: 4.32秒后重连
- ...
- 最大: 600秒（10分钟）后重连
```

### 连接生命周期

```
1. 创建连接
   ↓
2. 发送认证信息（Header）
   ↓
3. 认证验证
   ├─ 成功 → 连接建立
   └─ 失败 → 连接关闭
   ↓
4. 数据上报循环
   - 每 5 秒上报一次数据
   - 每 30 秒发送心跳
   ↓
5. 连接断开
   - 网络错误
   - 心跳超时
   - 主动关闭
   ↓
6. 重连（指数退避）
```

### 错误处理

**Close Codes**:

| Code | 说明 |
|------|------|
| 1000 | 正常关闭 |
| 1001 | 端点离开 |
| 1002 | 协议错误 |
| 1003 | 不支持的数据类型 |
| 1006 | 异常关闭（连接丢失） |
| 1008 | 违反策略（认证失败） |
| 1011 | 内部错误 |

## 前端通道 (/ws-frontend)

### 连接信息

- **URL**: `ws://dashboard-host:8900/ws-frontend`
- **协议**: WebSocket
- **认证**: 无需认证

### 连接方式

#### JavaScript

```javascript
const ws = new WebSocket('ws://localhost:8900/ws-frontend');

ws.onopen = () => {
  console.log('WebSocket 连接成功');
};

ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log('收到服务器数据:', data);
  // 更新UI
};

ws.onerror = (error) => {
  console.error('WebSocket 错误:', error);
};

ws.onclose = () => {
  console.log('WebSocket 断开，3秒后重连');
  setTimeout(() => connectWebSocket(), 3000);
};
```

#### TypeScript (推荐)

```typescript
interface ServerInfo {
  serverId: string;
  serverName: string;
  cpu: CPUInfo;
  memory: MemoryInfo;
  // ... 其他字段
}

class WebSocketClient {
  private ws: WebSocket | null = null;
  private reconnectInterval = 3000;
  private url: string;

  constructor(url: string) {
    this.url = url;
  }

  connect() {
    this.ws = new WebSocket(this.url);

    this.ws.onopen = () => {
      console.log('WebSocket 连接成功');
    };

    this.ws.onmessage = (event: MessageEvent) => {
      try {
        const data: ServerInfo = JSON.parse(event.data);
        this.handleMessage(data);
      } catch (error) {
        console.error('JSON 解析失败:', error);
      }
    };

    this.ws.onerror = (error: Event) => {
      console.error('WebSocket 错误:', error);
    };

    this.ws.onclose = () => {
      console.log('WebSocket 断开，尝试重连...');
      setTimeout(() => this.connect(), this.reconnectInterval);
    };
  }

  private handleMessage(data: ServerInfo) {
    // 更新UI或触发事件
    console.log('收到数据:', data);
  }

  close() {
    if (this.ws) {
      this.ws.close();
    }
  }
}

// 使用
const client = new WebSocketClient('ws://localhost:8900/ws-frontend');
client.connect();
```

### 消息格式

#### Dashboard → 前端 (推送数据)

**消息类型**: Text (JSON)

**消息内容**: 与 Agent 上报的 ServerInfo 格式完全相同

```json
{
  "serverId": "web-1",
  "serverName": "Web Server 1",
  "cpu": { ... },
  "memory": { ... },
  "disk": [ ... ],
  "network": { ... },
  "system": { ... },
  "timestamp": 1699123456
}
```

**推送时机**:

- Dashboard 收到 Agent 上报数据时，立即广播到所有前端连接
- 实时性: 通常 <100ms 延迟

#### 前端 → Dashboard

前端不向 Dashboard 发送消息，只接收数据。

### 连接管理

**多连接支持**:

Dashboard 支持多个前端同时连接，每个连接独立接收所有服务器的数据。

**断线重连**:

```javascript
let reconnectAttempts = 0;
const maxReconnectDelay = 30000; // 最大30秒

function connect() {
  const ws = new WebSocket('ws://localhost:8900/ws-frontend');

  ws.onopen = () => {
    reconnectAttempts = 0; // 重置重连计数
    console.log('连接成功');
  };

  ws.onclose = () => {
    const delay = Math.min(3000 * Math.pow(1.5, reconnectAttempts), maxReconnectDelay);
    reconnectAttempts++;

    console.log(`连接断开，${delay/1000}秒后重连（第${reconnectAttempts}次）`);
    setTimeout(connect, delay);
  };
}
```

## 使用示例

### Vue 3 集成

```typescript
// composables/useWebSocket.ts
import { ref, onMounted, onUnmounted } from 'vue'
import type { ServerInfo } from '@/types'

export function useWebSocket(url: string) {
  const servers = ref<Map<string, ServerInfo>>(new Map())
  const connected = ref(false)
  let ws: WebSocket | null = null

  function connect() {
    ws = new WebSocket(url)

    ws.onopen = () => {
      connected.value = true
      console.log('WebSocket 连接成功')
    }

    ws.onmessage = (event) => {
      const data: ServerInfo = JSON.parse(event.data)
      servers.value.set(data.serverId, data)
    }

    ws.onclose = () => {
      connected.value = false
      setTimeout(connect, 3000)
    }
  }

  onMounted(() => {
    connect()
  })

  onUnmounted(() => {
    if (ws) {
      ws.close()
    }
  })

  return {
    servers,
    connected
  }
}

// 使用
<script setup lang="ts">
import { useWebSocket } from '@/composables/useWebSocket'

const { servers, connected } = useWebSocket('ws://localhost:8900/ws-frontend')
</script>

<template>
  <div>
    <div v-if="connected" class="status online">在线</div>
    <div v-else class="status offline">离线</div>

    <div v-for="server in servers.values()" :key="server.serverId">
      <h3>{{ server.serverName }}</h3>
      <p>CPU: {{ server.cpu.percent.toFixed(1) }}%</p>
      <p>内存: {{ server.memory.usedPercent.toFixed(1) }}%</p>
    </div>
  </div>
</template>
```

### React 集成

```typescript
// hooks/useWebSocket.ts
import { useState, useEffect, useRef } from 'react'
import type { ServerInfo } from './types'

export function useWebSocket(url: string) {
  const [servers, setServers] = useState<Map<string, ServerInfo>>(new Map())
  const [connected, setConnected] = useState(false)
  const wsRef = useRef<WebSocket | null>(null)

  useEffect(() => {
    function connect() {
      const ws = new WebSocket(url)

      ws.onopen = () => {
        setConnected(true)
      }

      ws.onmessage = (event) => {
        const data: ServerInfo = JSON.parse(event.data)
        setServers(prev => new Map(prev).set(data.serverId, data))
      }

      ws.onclose = () => {
        setConnected(false)
        setTimeout(connect, 3000)
      }

      wsRef.current = ws
    }

    connect()

    return () => {
      wsRef.current?.close()
    }
  }, [url])

  return { servers, connected }
}
```

## 调试技巧

### 浏览器开发者工具

1. 打开开发者工具（F12）
2. 切换到 **Network** 标签
3. 筛选 **WS**（WebSocket）
4. 查看连接状态和消息

### 使用 wscat 测试

```bash
# 安装 wscat
pnpm add -g wscat

# 测试前端通道
wscat -c ws://localhost:8900/ws-frontend

# 测试 Agent 通道（需要认证）
wscat -c ws://localhost:8900/ws-report \
  -H "X-AUTH-SECRET: your-secret" \
  -H "X-SERVER-ID: your-id"
```

### 日志监控

```bash
# Dashboard 日志
sudo journalctl -u sss-dashboard -f | grep WebSocket

# 过滤连接事件
sudo journalctl -u sss-dashboard -f | grep "连接\|断开"
```

## 性能考虑

### 消息频率

- **Agent 上报**: 默认 5 秒/次（可配置）
- **前端推送**: 收到数据后立即推送（实时）
- **心跳**: Agent 30秒/次，Dashboard 自动响应

### 连接限制

- **并发连接数**: 理论无限制，实际受服务器资源限制
- **单个连接**: 支持长时间连接（数小时到数天）
- **重连频率**: 建议使用指数退避，避免DDoS

### 带宽估算

**单个 Agent**:

- 消息大小: ~2KB
- 频率: 5秒/次
- 带宽: ~0.4 KB/s

**100 个 Agent + 10 个前端**:

- Agent 上行: 40 KB/s
- 前端下行: 400 KB/s (10个连接)
- 总带宽: ~440 KB/s

## 故障排查

### 连接失败

**检查清单**:

1. ✅ Dashboard 是否运行
2. ✅ 端口是否开放（防火墙）
3. ✅ URL 是否正确（ws:// 或 wss://）
4. ✅ Agent 认证信息是否正确

### 连接断开

**常见原因**:

1. 网络不稳定
2. 心跳超时
3. Dashboard 重启
4. 代理服务器超时

**解决方案**:

- 实现自动重连（指数退避）
- 调整心跳间隔
- 配置代理超时（Nginx、Caddy）

### 数据不更新

**检查**:

1. WebSocket 连接状态（浏览器开发者工具）
2. Agent 是否正常上报（Dashboard 日志）
3. 前端是否正确处理消息（控制台）

## 安全建议

1. **使用 WSS**: 生产环境使用 wss:// (WebSocket Secure)
2. **强认证密钥**: Agent 使用强随机密钥
3. **限流**: 实现连接速率限制
4. **监控**: 监控异常连接和消息模式

## 相关文档

- [WebSocket 通信设计](../architecture/websocket.md) - 架构设计详解
- [REST API](./rest-api.md) - HTTP API 文档
- [数据流向](../architecture/data-flow.md) - 数据流转

---

**版本**: 1.0
**作者**: ruan
**最后更新**: 2025-11-05
