# 开发环境搭建

> **作者**: ruan
> **最后更新**: 2025-11-05

## 环境要求

### 必需软件

- **Go**: 1.23.2 或更高版本
- **Node.js**: 18.x 或更高版本
- **pnpm**: 8.x 或更高版本
- **Git**: 最新版本

### 推荐工具

- **IDE**: GoLand、VS Code、Cursor 等
- **Go插件**: gopls (语言服务器)
- **代码检查**: golangci-lint
- **API测试**: Postman、curl
- **WebSocket测试**: wscat、浏览器开发者工具

## 快速开始

### 1. 克隆项目

```bash
git clone https://github.com/ruanun/simple-server-status.git
cd simple-server-status
```

### 2. 安装依赖

#### 后端依赖

```bash
# 下载 Go 依赖
go mod download

# 验证依赖
go mod verify

# 可选：整理依赖
go mod tidy
```

#### 前端依赖

```bash
# 进入前端目录
cd web

# 安装依赖
pnpm install

# 返回项目根目录
cd ..
```

### 3. 配置文件

#### Agent 配置

```bash
# 复制配置模板
cp configs/sss-agent.yaml.example sss-agent.yaml

# 编辑配置
nano sss-agent.yaml
```

最小配置示例：

```yaml
serverAddr: ws://localhost:8900/ws-report
serverId: dev-agent-1
authSecret: dev-secret-key
logLevel: debug
```

#### Dashboard 配置

```bash
# 复制配置模板
cp configs/sss-dashboard.yaml.example sss-dashboard.yaml

# 编辑配置
nano sss-dashboard.yaml
```

最小配置示例：

```yaml
port: 8900
address: 0.0.0.0
webSocketPath: ws-report
servers:
  - name: Dev Agent 1
    id: dev-agent-1
    secret: dev-secret-key
    group: development
```

### 4. 运行开发环境

#### 方式 1: 直接运行（推荐用于开发）

**终端 1 - 启动 Dashboard**:

```bash
# 在项目根目录
go run ./cmd/dashboard
```

输出示例：
```
INFO  Dashboard 启动中...
INFO  HTTP 服务器监听: 0.0.0.0:8900
INFO  WebSocket 路径: /ws-report
INFO  前端 WebSocket 路径: /ws-frontend
```

**终端 2 - 启动 Agent**:

```bash
# 在项目根目录
go run ./cmd/agent
```

输出示例：
```
INFO  Agent 启动中...
INFO  连接到 Dashboard: ws://localhost:8900/ws-report
INFO  WebSocket 连接成功
INFO  开始采集系统信息...
```

**终端 3 - 启动前端开发服务器**:

```bash
cd web
pnpm run dev
```

输出示例：
```
  VITE v6.0.0  ready in 500 ms

  ➜  Local:   http://localhost:5173/
  ➜  Network: use --host to expose
  ➜  press h + enter to show help
```

#### 方式 2: 使用 Makefile

```bash
# 构建所有模块
make build

# 只构建 Agent
make build-agent

# 只构建 Dashboard
make build-dashboard

# 运行 Agent
./bin/sss-agent

# 运行 Dashboard
./bin/sss-dashboard
```

#### 方式 3: 使用 goreleaser（多平台构建）

```bash
# 1. 构建前端
cd web
pnpm install
pnpm run build:prod
cd ..

# 2. 使用 goreleaser 构建
goreleaser release --snapshot --clean

# 3. 二进制文件输出到 dist/ 目录
ls dist/
```

## 项目结构说明

```
simple-server-status/
├── cmd/                          # 程序入口
│   ├── agent/main.go             # Agent 启动入口
│   └── dashboard/main.go         # Dashboard 启动入口
│
├── internal/                     # 内部包（项目内部使用）
│   ├── agent/                    # Agent 实现
│   │   ├── config/               # Agent 配置
│   │   ├── global/               # Agent 全局变量
│   │   ├── adaptive.go           # 自适应采集
│   │   ├── errorhandler.go       # 错误处理
│   │   ├── gopsutil.go           # 系统信息采集
│   │   ├── mempool.go            # 内存池
│   │   ├── monitor.go            # 性能监控
│   │   ├── network_stats.go      # 网络统计（并发安全）
│   │   ├── report.go             # 数据上报
│   │   ├── validator.go          # 配置验证
│   │   └── ws.go                 # WebSocket客户端
│   │
│   ├── dashboard/                # Dashboard 实现
│   │   ├── config/               # Dashboard 配置
│   │   ├── global/               # Dashboard 全局变量
│   │   ├── handler/              # HTTP 处理器
│   │   ├── response/             # HTTP 响应封装
│   │   ├── server/               # HTTP 服务器
│   │   ├── public/               # 前端静态文件嵌入
│   │   ├── config_validator.go   # 配置验证
│   │   ├── error_handler.go      # 错误处理
│   │   ├── frontend_websocket_manager.go  # 前端 WebSocket 管理
│   │   ├── middleware.go         # 中间件
│   │   └── websocket_manager.go  # Agent WebSocket 管理
│   │
│   └── shared/                   # 共享基础设施
│       ├── logging/              # 日志初始化
│       │   └── logger.go
│       ├── config/               # 配置加载器
│       │   └── loader.go
│       └── errors/               # 错误类型和处理
│           ├── types.go
│           └── handler.go
│
├── pkg/                          # 公共包（可被外部引用）
│   └── model/                    # 共享数据模型
│       ├── server.go             # 服务器信息
│       ├── cpu.go                # CPU 信息
│       ├── memory.go             # 内存信息
│       ├── disk.go               # 磁盘信息
│       └── network.go            # 网络信息
│
├── configs/                      # 配置文件示例
│   ├── sss-agent.yaml.example
│   └── sss-dashboard.yaml.example
│
├── web/                          # Vue 3 前端
│   ├── src/
│   │   ├── api/                  # API 和 WebSocket 客户端
│   │   ├── components/           # Vue 组件
│   │   ├── pages/                # 页面
│   │   ├── stores/               # 状态管理
│   │   └── utils/                # 工具函数
│   ├── package.json
│   └── vite.config.ts
│
├── deployments/                  # 部署配置
├── docs/                         # 文档
├── scripts/                      # 脚本
├── go.mod                        # 统一的 Go 模块定义
├── Makefile                      # 构建任务
└── .goreleaser.yaml              # 多平台构建配置
```

## 开发指南

### 后端开发

#### 修改 Agent 代码

1. 修改 `internal/agent/` 下的文件
2. 运行 `go run ./cmd/agent` 测试
3. 查看日志输出验证功能

#### 修改 Dashboard 代码

1. 修改 `internal/dashboard/` 下的文件
2. 运行 `go run ./cmd/dashboard` 测试
3. 使用 API 测试工具验证接口

#### 修改共享包

1. 修改 `internal/shared/` 或 `pkg/model/` 下的文件
2. 同时测试 Agent 和 Dashboard
3. 确保向后兼容

### 前端开发

#### 开发模式

```bash
cd web

# 启动开发服务器（自动热重载）
pnpm run dev

# 访问 http://localhost:5173
```

#### 修改前端代码

1. 修改 `web/src/` 下的 Vue 组件
2. Vite 自动热重载，浏览器立即更新
3. 打开浏览器开发者工具查看 WebSocket 通信

#### 构建生产版本

```bash
cd web

# 类型检查
pnpm run type-check

# 构建生产版本
pnpm run build:prod

# 输出到 web/dist/
```

### 代码规范

#### Go 代码规范

使用 `golangci-lint` 进行代码检查：

```bash
# 安装 golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# 运行代码检查
golangci-lint run

# 或使用 Makefile
make lint
```

项目使用的 linters（`.golangci.yml`）：

- `errcheck` - 检查未处理的错误
- `gosimple` - 简化代码建议
- `govet` - 静态分析
- `ineffassign` - 检测无效的赋值
- `staticcheck` - 高级静态分析
- `unused` - 检查未使用的代码
- `gofmt` - 格式化检查
- `goimports` - 导入排序检查
- `misspell` - 拼写检查
- `unconvert` - 不必要的类型转换
- 等等...

#### TypeScript 代码规范

```bash
cd web

# 类型检查
pnpm run type-check

# Lint 检查（如果配置了）
pnpm run lint

# 格式化代码（如果配置了）
pnpm run format
```

### 调试技巧

#### Go 调试

**使用 delve**:

```bash
# 安装 delve
go install github.com/go-delve/delve/cmd/dlv@latest

# 调试 Agent
dlv debug ./cmd/agent

# 调试 Dashboard
dlv debug ./cmd/dashboard
```

**VS Code 调试配置** (`.vscode/launch.json`):

```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Debug Agent",
      "type": "go",
      "request": "launch",
      "mode": "debug",
      "program": "${workspaceFolder}/cmd/agent",
      "cwd": "${workspaceFolder}"
    },
    {
      "name": "Debug Dashboard",
      "type": "go",
      "request": "launch",
      "mode": "debug",
      "program": "${workspaceFolder}/cmd/dashboard",
      "cwd": "${workspaceFolder}"
    }
  ]
}
```

#### 查看日志

**Agent 日志**:

```bash
# 实时查看
tail -f logs/agent.log

# 过滤错误
tail -f logs/agent.log | grep ERROR

# 过滤 WebSocket 相关
tail -f logs/agent.log | grep WebSocket
```

**Dashboard 日志**:

```bash
# 实时查看
tail -f logs/dashboard.log

# 查看连接事件
tail -f logs/dashboard.log | grep "连接\|断开"

# 查看错误
tail -f logs/dashboard.log | grep ERROR
```

#### WebSocket 调试

**使用 wscat**:

```bash
# 安装 wscat
pnpm add -g wscat

# 连接到 Dashboard（需要认证）
wscat -c ws://localhost:8900/ws-report \
  -H "X-AUTH-SECRET: dev-secret-key" \
  -H "X-SERVER-ID: dev-agent-1"

# 发送测试消息
> {"serverId":"dev-agent-1","serverName":"Test"}
```

**浏览器开发者工具**:

1. 打开浏览器开发者工具（F12）
2. 切换到 "Network" 标签
3. 过滤 "WS"（WebSocket）
4. 查看 WebSocket 连接和消息

### 常见问题

#### 1. Agent 无法连接 Dashboard

**检查清单**:

```bash
# 1. Dashboard 是否启动
ps aux | grep sss-dashboard

# 2. 端口是否监听
netstat -an | grep 8900
# 或
lsof -i :8900

# 3. 配置是否正确
cat sss-agent.yaml
cat sss-dashboard.yaml

# 4. 防火墙是否阻止
sudo ufw status
```

#### 2. 前端无法连接 WebSocket

**检查**:

1. Dashboard 是否启动在 0.0.0.0:8900
2. 浏览器控制台是否有错误
3. WebSocket URL 是否正确（ws://localhost:8900/ws-frontend）

#### 3. 编译错误

```bash
# 清理缓存
go clean -cache

# 重新下载依赖
rm go.sum
go mod tidy

# 验证依赖
go mod verify
```

#### 4. 前端构建失败

```bash
# 清理缓存
cd web
rm -rf node_modules dist
pnpm install

# 重新构建
pnpm run build:prod
```

## 性能分析

### Go 性能分析

**CPU Profile**:

```bash
# Agent
go run ./cmd/agent -cpuprofile=cpu.prof

# 分析
go tool pprof cpu.prof
```

**内存 Profile**:

```bash
# Agent
go run ./cmd/agent -memprofile=mem.prof

# 分析
go tool pprof mem.prof
```

**pprof Web 界面**:

```bash
go tool pprof -http=:8080 cpu.prof
```

### 前端性能分析

使用浏览器开发者工具：

1. Performance 标签 - 记录性能
2. Memory 标签 - 内存快照
3. Network 标签 - 网络请求分析

## 下一步

- [贡献指南](./contributing.md) - 如何贡献代码
- [架构文档](../architecture/overview.md) - 了解系统架构

---

**版本**: 1.0
**作者**: ruan
**最后更新**: 2025-11-05
