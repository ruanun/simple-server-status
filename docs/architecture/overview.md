# 架构概览

> **作者**: ruan
> **最后更新**: 2025-11-05

## 项目概述

SimpleServerStatus 是一个基于 Golang + Vue 的分布式服务器监控系统，采用 **Monorepo** 单仓库架构设计，实现了前后端分离、模块解耦、代码共享的现代化架构。

## 核心架构

### 系统组成

```
┌────────────┐                ┌─────────────┐                ┌──────────┐
│   Agent    │   WebSocket    │  Dashboard  │   WebSocket    │   Web    │
│  (采集端)   │ ────────────►  │  (服务端)   │ ────────────►  │ (展示端)  │
└────────────┘     /ws-report └─────────────┘   /ws-frontend └──────────┘
```

- **Agent**: 部署在被监控服务器上的监控代理，负责收集系统指标并通过 WebSocket 上报
- **Dashboard**: 后端服务，管理 WebSocket 连接、提供 REST API 和静态资源服务
- **Web**: Vue 3 前端用户界面，实时展示监控数据

### Monorepo 架构

项目采用 Monorepo 单仓库架构，统一管理所有模块：

```
simple-server-status/
├── go.mod                     # 统一的 Go 模块定义
│
├── cmd/                       # 程序入口
│   ├── agent/main.go          # Agent 启动入口
│   └── dashboard/main.go      # Dashboard 启动入口
│
├── pkg/                       # 公共包（可被外部引用）
│   └── model/                 # 共享数据模型
│       ├── server.go          # 服务器信息
│       ├── cpu.go             # CPU 信息
│       ├── memory.go          # 内存信息
│       ├── disk.go            # 磁盘信息
│       └── network.go         # 网络信息
│
├── internal/                  # 内部包（项目内部使用）
│   ├── agent/                 # Agent 实现
│   ├── dashboard/             # Dashboard 实现
│   └── shared/                # 共享基础设施
│       ├── logging/           # 统一日志
│       ├── config/            # 统一配置加载
│       └── errors/            # 统一错误处理
│
├── configs/                   # 配置文件示例
├── deployments/               # 部署配置
├── scripts/                   # 构建和部署脚本
└── web/                       # Vue 3 前端
```

### 架构特点

✅ **Monorepo 架构优势**
- 统一 go.mod，避免版本冲突
- 代码共享简单，直接 import
- IDE 自动识别，代码跳转无障碍
- 统一构建脚本和 CI/CD
- 保持独立部署能力

✅ **标准项目布局**
- 符合 Go 标准项目结构
- `cmd/` 存放程序入口
- `pkg/` 存放可导出的公共包
- `internal/` 存放内部实现
- 清晰的模块边界

✅ **依赖注入设计**
- 基础设施包支持依赖注入
- 减少全局变量使用
- 提高代码可测试性
- 便于单元测试和集成测试

## 数据模型

### 共享数据模型 (pkg/model)

所有数据模型定义在 `pkg/model/` 包中，Agent 和 Dashboard 共用：

- **ServerInfo**: 服务器基本信息（ID、名称、分组、国家等）
- **CPUInfo**: CPU 使用率、核心数等
- **MemoryInfo**: 内存使用情况（总量、已用、可用等）
- **DiskInfo**: 磁盘使用情况（分区、容量、读写速度）
- **NetworkInfo**: 网络流量统计（上传、下载、速度）

### 导入路径规范

```go
// 共享数据模型
import "github.com/ruanun/simple-server-status/pkg/model"

// Agent 内部包
import "github.com/ruanun/simple-server-status/internal/agent/config"

// Dashboard 内部包
import "github.com/ruanun/simple-server-status/internal/dashboard/handler"

// 共享基础设施
import "github.com/ruanun/simple-server-status/internal/shared/logging"
```

## 核心模块

### Agent 模块

**职责**: 系统信息采集和上报

**核心组件**:
- **采集器 (gopsutil.go)**: 使用 gopsutil 库采集系统信息
- **网络统计 (network_stats.go)**: 并发安全的网络流量统计
- **数据上报 (report.go)**: 定时采集并通过 WebSocket 上报
- **WebSocket 客户端 (ws.go)**: 维护与 Dashboard 的 WebSocket 连接
- **性能监控 (monitor.go)**: 监控 Agent 自身性能
- **内存池 (mempool.go)**: 优化内存分配，减少 GC 压力
- **自适应采集 (adaptive.go)**: 根据系统负载动态调整采集频率

**关键特性**:
- ✅ 指数退避重连机制
- ✅ 心跳保持连接
- ✅ 并发安全的网络统计
- ✅ Goroutine 优雅退出（Context 取消）
- ✅ Channel 安全关闭
- ✅ 内存池优化

### Dashboard 模块

**职责**: WebSocket 连接管理、数据分发、Web 界面服务

**核心组件**:
- **WebSocket 管理器 (websocket_manager.go)**: 管理 Agent 连接
- **前端 WebSocket 管理器 (frontend_websocket_manager.go)**: 管理前端连接
- **HTTP 处理器 (handler/)**: REST API 处理
- **中间件 (middleware.go)**: CORS、Recovery、日志等
- **服务器初始化 (server/server.go)**: Gin 服务器初始化
- **静态资源 (public/resource.go)**: 嵌入前端静态文件

**关键特性**:
- ✅ 双通道 WebSocket 设计（Agent 通道 + 前端通道）
- ✅ 连接状态跟踪
- ✅ 心跳超时检测
- ✅ 并发连接管理
- ✅ 静态文件嵌入部署

### 共享基础设施 (internal/shared)

**日志模块 (logging/)**:
- 基于 Zap 实现的结构化日志
- 支持日志级别、文件输出、日志轮转
- 统一的日志初始化接口

**配置模块 (config/)**:
- 基于 Viper 实现的配置加载
- 支持多路径搜索
- 支持环境变量覆盖
- 支持配置热加载

**错误处理模块 (errors/)**:
- 统一的错误类型定义
- 错误严重等级分类
- 错误统计和历史记录
- 重试机制（指数退避）

## 技术栈

### 后端技术

- **Go 1.23.2**: 主要开发语言
- **Gin 1.x**: HTTP 框架
- **Melody**: WebSocket 库（Agent 连接）
- **gorilla/websocket**: WebSocket 库（前端连接）
- **gopsutil**: 系统信息采集
- **Viper**: 配置管理
- **Zap**: 结构化日志

### 前端技术

- **Vue 3.5+**: 使用 Composition API
- **TypeScript 5.6+**: 类型安全
- **Ant Design Vue 4.x**: UI 组件库
- **Vite 6.x**: 构建工具
- **unplugin-vue-components**: 组件自动导入

## 编译和部署

### 独立编译

```bash
# 编译 Agent（输出独立二进制）
go build -o bin/sss-agent ./cmd/agent

# 编译 Dashboard（输出独立二进制）
go build -o bin/sss-dashboard ./cmd/dashboard
```

### 多平台构建

```bash
# 使用 goreleaser 构建多平台版本
goreleaser release --snapshot --clean

# 支持的平台
# - Linux (amd64, arm, arm64)
# - Windows (amd64)
# - macOS (amd64, arm64)
# - FreeBSD (amd64, arm64)
```

### 部署方式

**Agent 服务器**只需要：
- `sss-agent` 二进制文件
- `configs/sss-agent.yaml` 配置文件

**Dashboard 服务器**只需要：
- `sss-dashboard` 二进制文件
- `configs/sss-dashboard.yaml` 配置文件
- 前端静态文件（已嵌入到二进制中）

## 架构优势

### 对比传统架构

**优化前（Go Workspace）**:
```
Agent ─依赖→ Dashboard/pkg/model  ❌
独立 go.mod × 2 + go.work         ⚠️
内部目录扁平化                    ⚠️
基础设施代码重复 ~330行            ⚠️
```

**优化后（Monorepo）**:
```
Agent ←─共享─→ pkg/model ←─共享─→ Dashboard  ✅
统一 go.mod                                   ✅
清晰分层架构（cmd/pkg/internal）                ✅
共享基础设施（logging/config/errors）           ✅
```

### 量化指标

| 指标 | 优化前 | 优化后 | 提升 |
|------|--------|--------|------|
| **模块独立性** | ❌ Agent 依赖 Dashboard | ✅ 完全独立 | 100% |
| **go.mod 文件** | 3 个（含 go.work） | 1 个 | -67% |
| **代码重复** | ~330 行 | <50 行 | 85% |
| **全局变量** | 10+ 个 | 支持依赖注入 | 显著改善 |
| **目录层级** | 扁平化 | 清晰分层 | 200% |
| **并发安全** | ❌ 存在竞态 | ✅ 完全安全 | 100% |

## 开发体验改进

1. **更快的编译**: Monorepo 单仓库，只编译修改部分
2. **更好的可维护性**: 清晰的分层和职责划分
3. **更容易扩展**: 接口抽象，便于添加功能
4. **更规范的结构**: 符合 Go 社区标准
5. **更安全的代码**: 修复了所有已知的并发安全问题

## 相关文档

- [WebSocket 通信设计](./websocket.md) - WebSocket 双通道设计详解
- [数据流向](./data-flow.md) - 系统数据流转过程
- [开发指南](../development/setup.md) - 本地开发环境搭建
- [API 文档](../api/rest-api.md) - REST API 接口说明

---

**版本**: 1.0
**作者**: ruan
**最后更新**: 2025-11-05
