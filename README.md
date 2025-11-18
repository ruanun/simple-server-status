# Simple Server Status

<div align="center">

一款**极简探针**，云探针、多服务器探针。基于 Golang + Vue 实现。

[![GitHub release](https://img.shields.io/github/release/ruanun/simple-server-status.svg)](https://github.com/ruanun/simple-server-status/releases)
[![Build Status](https://github.com/ruanun/simple-server-status/workflows/CI/badge.svg)](https://github.com/ruanun/simple-server-status/actions)
[![codecov](https://codecov.io/gh/ruanun/simple-server-status/branch/master/graph/badge.svg)](https://codecov.io/gh/ruanun/simple-server-status)
[![Go Report Card](https://goreportcard.com/badge/github.com/ruanun/simple-server-status)](https://goreportcard.com/report/github.com/ruanun/simple-server-status)
[![Docker Pulls](https://img.shields.io/docker/pulls/ruanun/sssd)](https://hub.docker.com/r/ruanun/sssd)
[![GitHub license](https://img.shields.io/github/license/ruanun/simple-server-status.svg)](https://github.com/ruanun/simple-server-status/blob/main/LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/ruanun/simple-server-status)](https://github.com/ruanun/simple-server-status/blob/master/go.mod)

**演示地址：** [https://sssd.ions.top](https://sssd.ions.top/)

</div>

## ✨ 特性

- 🚀 **极简设计** - 简洁美观的 Web 界面
- 📊 **实时监控** - 实时显示服务器状态信息
- 🌐 **多平台支持** - 支持 Linux、Windows、macOS、FreeBSD
- 📱 **响应式设计** - 完美适配桌面和移动设备
- 🔒 **安全可靠** - WebSocket 加密传输，支持认证
- 🐳 **容器化部署** - 支持 Docker 一键部署
- 📦 **轻量级** - 单文件部署，资源占用极低
- 🔧 **易于配置** - YAML 配置文件，简单易懂

## 📊 监控指标

- **系统信息** - 操作系统、架构、内核版本
- **CPU 使用率** - 实时 CPU 占用情况
- **内存使用** - 内存使用率和详细信息
- **磁盘空间** - 磁盘使用情况和 I/O 统计
- **网络流量** - 网络接口流量统计
- **系统负载** - 系统平均负载
- **运行时间** - 系统运行时间
- **进程数量** - 系统进程统计

## 🚀 5分钟快速开始

### 步骤 1：部署 Dashboard（监控面板）

```bash
# 1. 下载配置文件
wget https://raw.githubusercontent.com/ruanun/simple-server-status/main/configs/sss-dashboard.yaml.example -O sss-dashboard.yaml

# 2. 编辑配置（设置服务器ID和密钥）
nano sss-dashboard.yaml

# 3. 启动 Dashboard
docker run --name sssd --restart=unless-stopped -d \
  -v ./sss-dashboard.yaml:/app/sss-dashboard.yaml \
  -p 8900:8900 \
  ruanun/sssd

# 4. 访问 http://your-server-ip:8900
```

### 步骤 2：部署 Agent（被监控服务器）

**Linux/macOS/FreeBSD:**

```bash
# 一键安装
curl -fsSL https://raw.githubusercontent.com/ruanun/simple-server-status/main/scripts/install-agent.sh | sudo bash

# 编辑配置
sudo nano /etc/sssa/sss-agent.yaml
# 修改 serverAddr, serverId, authSecret

# 启动服务
sudo systemctl start sssa
sudo systemctl enable sssa
```

**Windows (PowerShell 管理员模式):**

```powershell
# 一键安装
iwr -useb https://raw.githubusercontent.com/ruanun/simple-server-status/main/scripts/install-agent.ps1 | iex

# 配置文件位置: C:\Program Files\SSSA\sss-agent.yaml
# 配置后启动 SSSA 服务
```

### 步骤 3：验证

- 访问 Dashboard：`http://your-server-ip:8900`
- 检查服务器是否显示为在线状态
- 查看实时数据更新

**遇到问题？** 参考 [故障排除指南](docs/troubleshooting.md)

---

## 📖 文档导航

### 快速开始

- 📥 **[完整部署指南](docs/getting-started.md)** - 从零开始的详细部署步骤
- 🛠️ **[脚本使用说明](scripts/README.md)** - 安装脚本和构建脚本详解
- 📦 **[Release 下载](https://github.com/ruanun/simple-server-status/releases)** - 预编译二进制文件

### 部署方式

- 🐳 **[Docker 部署](docs/deployment/docker.md)** - Docker 和 Docker Compose 完整指南
- ⚙️ **[systemd 部署](docs/deployment/systemd.md)** - Linux systemd 服务配置
- 🔧 **[手动安装](docs/deployment/manual.md)** - 不使用脚本的手动安装步骤
- 🌐 **[反向代理配置](docs/deployment/proxy.md)** - Nginx/Caddy/Apache HTTPS 配置

### 维护和故障排除

- 🐛 **[故障排除指南](docs/troubleshooting.md)** - 常见问题和详细解决方案
- 🔄 **[维护指南](docs/maintenance.md)** - 更新、备份、迁移、卸载

### 架构和开发

- 🏗️ **[架构概览](docs/architecture/overview.md)** - 系统整体架构和技术栈
- 🔌 **[WebSocket 通信设计](docs/architecture/websocket.md)** - 双通道 WebSocket 实现详解
- 🔄 **[数据流向](docs/architecture/data-flow.md)** - 完整数据流转过程
- 💻 **[开发环境搭建](docs/development/setup.md)** - 本地开发环境配置
- 🤝 **[贡献指南](docs/development/contributing.md)** - 如何贡献代码

### API 文档

- 🌐 **[REST API](docs/api/rest-api.md)** - HTTP API 接口说明
- 💬 **[WebSocket API](docs/api/websocket-api.md)** - WebSocket 消息格式和协议

---

## ❓ 常见问题

<details>
<summary><b>Q1: 如何监控多台服务器？</b></summary>

在 Dashboard 配置中添加多个服务器：

```yaml
servers:
  - id: "server-01"
    name: "生产服务器-1"
    secret: "your-secret-1"
  - id: "server-02"
    name: "生产服务器-2"
    secret: "your-secret-2"
```

在每台服务器上安装 Agent 并配置对应的 ID 和密钥。详见 [完整部署指南](docs/getting-started.md)

</details>

<details>
<summary><b>Q2: 如何配置 HTTPS？</b></summary>

使用反向代理（Nginx 或 Caddy）配置 HTTPS：

```bash
# Caddy（推荐，自动 HTTPS）
status.example.com {
    reverse_proxy localhost:8900
}
```

Agent 配置改为：`serverAddr: wss://status.example.com/ws-report`

详细配置参考 [反向代理指南](docs/deployment/proxy.md)

</details>

<details>
<summary><b>Q3: 支持哪些操作系统？</b></summary>

**完全支持：**
- Linux（x86_64, ARM64, ARMv7）
- Windows（x86_64, ARM64）
- macOS（x86_64, ARM64/Apple Silicon）
- FreeBSD（x86_64）

**已测试的 Linux 发行版：**
- Ubuntu 18.04+, Debian 10+, CentOS 7+, Rocky Linux 8+, Arch Linux, Alpine Linux

</details>

<details>
<summary><b>Q4: 资源占用情况如何？</b></summary>

**Agent（单个实例）：**
- 内存：约 8-15 MB
- CPU：< 0.5%（采集间隔 2s）
- 磁盘：约 5 MB

**Dashboard（监控 10 台服务器）：**
- 内存：约 30-50 MB
- CPU：< 2%
- 磁盘：约 20 MB

✅ 非常轻量，适合资源受限的环境

</details>

**更多问题？** 查看 [故障排除指南](docs/troubleshooting.md)

---

## 🏗️ 架构说明

本项目采用 **Monorepo 单仓库架构**，前后端分离设计：

- **Agent** - 轻量级监控客户端，部署在被监控服务器上
- **Dashboard** - 监控面板服务端，提供 Web 界面和数据收集
- **Web** - 前端界面，基于 Vue 3 开发
- **pkg/model** - 共享数据模型，Agent 和 Dashboard 共用
- **internal/shared** - 共享基础设施（日志、配置、错误处理）

### 技术栈

#### 后端技术
- **Go 1.23+** - 高性能编译型语言，跨平台支持
- **Gin** - 轻量级 HTTP Web 框架，高性能路由
- **Melody** - 优雅的 WebSocket 服务端库
- **Gorilla WebSocket** - 成熟的 WebSocket 客户端实现
- **Viper** - 灵活的配置管理，支持热加载
- **Zap** - 高性能结构化日志库
- **gopsutil** - 跨平台系统信息采集库

#### 前端技术
- **Vue 3** - 渐进式 JavaScript 框架（Composition API）
- **TypeScript** - 类型安全的 JavaScript 超集
- **Ant Design Vue** - 企业级 UI 组件库
- **Vite** - 下一代前端构建工具，开发体验极佳
- **Axios** - Promise 基于的 HTTP 客户端

#### 架构设计
- **Monorepo** - 单仓库多模块管理，统一依赖
- **标准 Go 项目布局** - cmd/、internal/、pkg/ 清晰分离
- **依赖注入** - 松耦合设计，易于测试和扩展
- **WebSocket 双通道** - 实时双向通信，低延迟

---

## 📊 系统要求

### Agent
- **内存**: 最低 10MB
- **CPU**: 最低 0.1%
- **磁盘**: 最低 5MB
- **网络**: 支持 WebSocket 连接

### Dashboard
- **内存**: 最低 20MB
- **CPU**: 最低 0.5%
- **磁盘**: 最低 10MB
- **端口**: 默认 8900（可配置）

---

## 🛠️ 开发构建

### 环境要求

- Go 1.23+
- Node.js 20+
- pnpm（推荐）或 npm

### 构建步骤

```bash
# 克隆项目
git clone https://github.com/ruanun/simple-server-status.git
cd simple-server-status

# 使用 Makefile（推荐）
make build-web       # 构建前端
make build-agent     # 构建 Agent
make build-dashboard # 构建 Dashboard（包含前端）
make build           # 构建所有模块

# 或使用构建脚本
bash scripts/build-web.sh
bash scripts/build-dashboard.sh
```

**详细构建说明：** [scripts/README.md](scripts/README.md)

### 开发模式

```bash
# 前端开发（热重载）
make dev-web

# 后端开发
make build-dashboard-only
./bin/sss-dashboard
```

### 项目结构

```
simple-server-status/
├── cmd/                    # 程序入口
│   ├── agent/             # Agent 启动入口
│   └── dashboard/         # Dashboard 启动入口
├── internal/              # 内部包
│   ├── agent/             # Agent 实现
│   ├── dashboard/         # Dashboard 实现
│   └── shared/            # 共享基础设施
├── pkg/model/             # 共享数据模型
├── configs/               # 配置文件示例
├── deployments/           # 部署配置
├── web/                   # Vue 3 前端
└── go.mod                 # 统一依赖管理
```

---

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

1. Fork 本项目
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 开启 Pull Request

详见 [贡献指南](docs/development/contributing.md)

---

## 📄 许可证

本项目采用 MIT 许可证。详情请参阅 [LICENSE](LICENSE) 文件。

---

## ⭐ Star History

如果这个项目对你有帮助，请给个 Star ⭐

---

<div align="center">

**[🏠 首页](https://github.com/ruanun/simple-server-status)** • **[📖 文档](docs/getting-started.md)** • **[🚀 演示](https://sssd.ions.top/)** • **[📦 下载](https://github.com/ruanun/simple-server-status/releases)**

</div>
