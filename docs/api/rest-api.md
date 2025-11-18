# REST API 文档

> **作者**: ruan
> **最后更新**: 2025-11-05

## 概述

SimpleServerStatus Dashboard 提供 REST API 用于查询服务器状态、统计信息和配置管理。所有 API 均返回 JSON 格式数据。

## 基础信息

- **Base URL**: `http://dashboard-host:8900/api`
- **Content-Type**: `application/json`
- **认证**: 当前版本无需认证（后续版本可能添加）

## 通用响应格式

### 成功响应

```json
{
  "code": 0,
  "message": "success",
  "data": { ... }
}
```

### 错误响应

```json
{
  "code": 1001,
  "message": "error message",
  "data": null
}
```

### 错误码

| 错误码 | 说明 |
|--------|------|
| 0 | 成功 |
| 1001 | 参数错误 |
| 1002 | 服务器不存在 |
| 1003 | 内部错误 |

## API 端点

### 1. 获取服务器列表

获取所有已连接服务器的状态信息。

**请求**:

```http
GET /api/server/statusInfo
```

**响应示例**:

```json
{
  "code": 0,
  "message": "success",
  "data": [
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
  ]
}
```

### 2. 获取统计信息

获取系统统计信息（连接数、消息数等）。

**请求**:

```http
GET /api/statistics
```

**响应示例**:

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "totalServers": 10,
    "onlineServers": 8,
    "offlineServers": 2,
    "totalMessages": 123456,
    "totalErrors": 12,
    "uptime": 86400
  }
}
```

## 数据模型

### ServerInfo

服务器完整信息对象。

```typescript
interface ServerInfo {
  serverId: string;          // 服务器ID
  serverName: string;        // 服务器名称
  group?: string;            // 分组
  countryCode?: string;      // 国家代码
  location?: string;         // 地理位置
  ip?: string;               // 公网IP
  cpu: CPUInfo;              // CPU信息
  memory: MemoryInfo;        // 内存信息
  disk: DiskInfo[];          // 磁盘信息
  network: NetworkInfo;      // 网络信息
  system: SystemInfo;        // 系统信息
  timestamp: number;         // 时间戳（Unix秒）
}
```

### CPUInfo

```typescript
interface CPUInfo {
  percent: number;           // CPU使用率（百分比）
  cores: number;             // CPU核心数
  modelName?: string;        // CPU型号
}
```

### MemoryInfo

```typescript
interface MemoryInfo {
  total: number;             // 总内存（字节）
  used: number;              // 已用内存（字节）
  available: number;         // 可用内存（字节）
  usedPercent: number;       // 使用率（百分比）
}
```

### DiskInfo

```typescript
interface DiskInfo {
  path: string;              // 挂载路径
  total: number;             // 总容量（字节）
  used: number;              // 已用容量（字节）
  free: number;              // 剩余容量（字节）
  usedPercent: number;       // 使用率（百分比）
  readSpeed?: number;        // 读取速度（字节/秒）
  writeSpeed?: number;       // 写入速度（字节/秒）
}
```

### NetworkInfo

```typescript
interface NetworkInfo {
  interfaceName: string;     // 网卡名称
  bytesSent: number;         // 发送总字节数
  bytesRecv: number;         // 接收总字节数
  inSpeed: number;           // 下载速度（字节/秒）
  outSpeed: number;          // 上传速度（字节/秒）
}
```

### SystemInfo

```typescript
interface SystemInfo {
  hostname: string;          // 主机名
  os: string;                // 操作系统
  platform: string;          // 平台
  arch: string;              // 架构
  kernel: string;            // 内核版本
  uptime: number;            // 运行时间（秒）
}
```

## 使用示例

### JavaScript/TypeScript

```typescript
// 获取服务器列表
async function getServers() {
  const response = await fetch('http://localhost:8900/api/server/statusInfo');
  const result = await response.json();

  if (result.code === 0) {
    console.log('服务器列表:', result.data);
    return result.data;
  } else {
    console.error('获取失败:', result.message);
    return [];
  }
}

// 获取统计信息
async function getStatistics() {
  const response = await fetch('http://localhost:8900/api/statistics');
  const result = await response.json();
  return result.data;
}
```

### curl

```bash
# 获取服务器列表
curl http://localhost:8900/api/server/statusInfo

# 获取统计信息
curl http://localhost:8900/api/statistics

# 验证配置
curl -X POST http://localhost:8900/api/config/validate \
  -H "Content-Type: application/json" \
  -d '{"port":8900,"servers":[{"name":"Test","id":"test-1","secret":"secret"}]}'
```

### Python

```python
import requests

# 获取服务器列表
def get_servers():
    response = requests.get('http://localhost:8900/api/server/statusInfo')
    result = response.json()

    if result['code'] == 0:
        return result['data']
    else:
        print(f"Error: {result['message']}")
        return []

# 获取统计信息
def get_statistics():
    response = requests.get('http://localhost:8900/api/statistics')
    return response.json()['data']
```

## 错误处理

建议在客户端实现以下错误处理策略：

1. **网络错误**: 实现重试机制（指数退避）
2. **超时**: 设置合理的超时时间（建议 10 秒）
3. **错误码处理**: 根据错误码进行相应处理
4. **数据验证**: 验证返回数据的完整性

**示例**:

```typescript
async function fetchWithRetry(url: string, maxRetries = 3) {
  let lastError;

  for (let i = 0; i < maxRetries; i++) {
    try {
      const response = await fetch(url, { timeout: 10000 });
      const result = await response.json();

      if (result.code === 0) {
        return result.data;
      } else {
        throw new Error(result.message);
      }
    } catch (error) {
      lastError = error;
      await new Promise(resolve => setTimeout(resolve, Math.pow(2, i) * 1000));
    }
  }

  throw lastError;
}
```

## 相关文档

- [WebSocket API](./websocket-api.md) - WebSocket 消息格式
- [数据流向](../architecture/data-flow.md) - 数据流转过程
- [架构概览](../architecture/overview.md) - 系统架构

---

**版本**: 1.0
**作者**: ruan
**最后更新**: 2025-11-05
