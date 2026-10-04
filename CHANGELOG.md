# Changelog

本项目的所有重要变更都将记录在此文件中。

格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.0.0/)，
并且本项目遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

## [2.0.0] - 未发布

整体重写，不兼容 1.x 的配置与数据。

- Dashboard：gin + SQLite（纯 Go），分级降采样的历史数据（24 小时分钟级、7 天 10 分钟级），月流量累计与重置日，JWT 登录与限流，导入导出（含密钥，便于整机迁移）。
- Agent：WebSocket 长连接上报，参数由 Dashboard 下发，支持 Linux systemd 与 Windows 服务。
- 前端：React 19 + shadcn/ui 重写，极简单色风格，独立详情页与历史图表，中英文与亮暗主题。
- 部署：Docker 多架构镜像；安装脚本参数为 `--dashboard/--id/--secret`，后台可随时查看安装命令。
- 状态页与后台增强：在线率、每日流量、Agent 版本与升级入口、费用汇总与到期列表、备注、公告、卡片排序，以及离线 / 高负载 / 到期 / 流量通知（Webhook、Telegram）。
- 离线记录与通知记录（后台事件页、详情页最近离线），IPv4/IPv6 探测与标记，到期与流量提醒投递成功后才记为已提醒，CI 竞态检测。
- 公开页恢复页脚（项目与作者链接），后台侧栏底部显示 Dashboard 版本。
- 事件系统：检测与通知分离，离线、CPU / 内存 / 硬盘高负载、重启、公网 IP 变化无论是否配置通知渠道都会记为事件，后台事件页可按类型筛选；高负载阈值移到新的「检测规则」设置，通知设置新增重启与 IP 变化开关，Dashboard 重启后已推送离线的服务器恢复时仍会通知。数据库迁移会清空已有的离线记录与旧的负载阈值设置；`GET /api/admin/outages` 由 `GET /api/admin/events` 取代。
- 登录安全：可选登录验证码（图形验证码 / Cloudflare Turnstile，新增 `disable-captcha` 子命令兜底），限流按 IPv6 /64 网段计数并增加全站陌生来源限额；状态页卡片头部精简，长名称可完整显示。

## [1.2.0] - 2025-11-15

这是一个重大功能更新版本，包含全面的架构重构、功能增强和文档完善。

### ✨ 新增 (Added)

#### 后端功能
- 新增自适应数据收集机制，优化资源占用
- 新增内存池管理，提升性能
- 新增网络统计监控功能
- 新增共享基础设施模块 (`internal/shared/`)，包含日志、配置和错误处理
- 新增完整的单元测试覆盖（3,700+ 行测试代码）
  - Agent 验证器测试 (`internal/agent/validator_test.go`)
  - Dashboard 配置验证器测试 (`internal/dashboard/config_validator_test.go`)
  - 错误处理器测试 (`internal/shared/errors/handler_test.go`)
  - 日志系统测试 (`internal/shared/logger/logger_test.go`)
  - 配置加载器测试 (`internal/shared/config/loader_test.go`)
- 新增配置验证器，提供更详细的配置错误提示和警告
- 新增错误处理器，统一错误响应格式
- 新增 WebSocket 连接管理器改进版

#### 前端功能
- 新增国际化 (i18n) 支持，支持中文和英文自动切换
  - 中文语言包 (`web/src/locales/zh-CN.ts`)
  - 英文语言包 (`web/src/locales/en-US.ts`)
  - 语言自动检测和切换
  - 数字和日期格式化工具
- 新增 WebSocket 和 HTTP 轮询双模式连接切换
- 新增连接状态管理器和自动重连机制
- 新增多个 UI 组件
  - Logo 组件 (`web/src/components/Logo.vue`)
  - 状态指示器 (`web/src/components/StatusIndicator.vue`)
  - 头部状态显示 (`web/src/components/HeaderStatus.vue`)
- 新增 Pinia 状态管理
  - 服务器状态 store (`web/src/stores/serverStore.ts`)
  - 配置 store (`web/src/stores/settingsStore.ts`)
- 新增 Composables 复用逻辑
  - WebSocket 连接 (`web/src/composables/useWebSocket.ts`)
  - HTTP 轮询 (`web/src/composables/usePolling.ts`)
  - 服务器数据 (`web/src/composables/useServerData.ts`)
- 新增完整的 TypeScript 类型定义 (`web/src/types/`)

#### 基础设施
- 新增多阶段 Dockerfile，支持前后端统一构建
  - 前端构建阶段（Node.js）
  - 后端构建阶段（Go）
  - 运行时阶段（Alpine Linux）
- 新增 Docker Compose 配置，集成 Caddy 反向代理
- 新增 GitHub Actions CI/CD 自动化工作流
  - 持续集成工作流 (`.github/workflows/ci.yml`)
  - 自动发布工作流 (`.github/workflows/release.yml`)
- 新增 GoReleaser 配置 (`.goreleaser.yml`)，支持跨平台自动发布
  - 支持 Linux (amd64, arm64, arm, 386)
  - 支持 macOS (amd64, arm64)
  - 支持 Windows (amd64, arm64, 386)
  - 支持 FreeBSD (amd64, arm64, arm)
- 新增 Makefile，支持多种构建和开发任务
  - `make build` - 构建所有二进制文件
  - `make build-agent` - 仅构建 agent
  - `make build-dashboard` - 仅构建 dashboard
  - `make build-web` - 仅构建前端
  - `make test` - 运行所有测试
  - `make docker-build` - 构建 Docker 镜像
  - `make clean` - 清理构建产物
- 新增构建脚本（Shell 和 PowerShell）
  - `scripts/build-web.sh` / `scripts/build-web.ps1` - 前端构建脚本
  - `scripts/build-dashboard.sh` - 完整构建脚本
  - `scripts/build-docker.sh` / `scripts/build-docker.ps1` - Docker 构建脚本
- 新增一键安装脚本
  - `scripts/install-agent.sh` - 支持 Linux/macOS/FreeBSD
  - `scripts/install-agent.ps1` - 支持 Windows PowerShell
  - 自动检测操作系统和架构
  - 自动下载最新版本
  - 自动配置 systemd 服务（Linux）
- 新增 golangci-lint 配置 (`.golangci.yml`)，包含 15+ 代码检查规则
  - errcheck - 错误检查
  - gosimple - 代码简化建议
  - govet - Go vet 检查
  - ineffassign - 无效赋值检查
  - staticcheck - 静态分析
  - unused - 未使用代码检查
  - 以及更多...
- 新增 `.dockerignore` 优化 Docker 镜像构建
  - 排除不必要的文件
  - 减小构建上下文大小
  - 加快构建速度

#### 代码质量与安全
- 全面修复 golangci-lint 检测的安全和代码质量问题（50+ 处）
  - **安全修复**
    - G112: 为 HTTP Server 添加超时配置（ReadHeaderTimeout、ReadTimeout、WriteTimeout、IdleTimeout），防止 Slowloris 攻击
    - G404: 为非安全场景的随机数生成器添加说明注释
    - G115: 安全处理 Unix 时间戳的 int64 到 uint64 转换
    - G101: 为 HTTP 头名称常量添加说明，消除硬编码凭证误报
    - G304: 为文件路径操作添加安全验证注释
    - G306: 将测试文件权限从 0644 改为更安全的 0600
  - **代码质量修复**
    - G104: 处理所有未检查的错误返回值（RegisterValidation、Sync()、Close() 等 20+ 处）
    - SA4003: 删除对 uint64 类型的无效负数检查
    - SA9003: 删除或重构空的 if 分支
    - ST1005: 统一错误信息格式为小写开头
    - unused: 导出 FormatFileSize 函数避免未使用警告
  - **代码格式化**
    - 使用 gofmt 格式化所有 Go 源文件（50+ 文件）
    - 使用 goimports 规范导入顺序和分组
- 升级 golangci-lint 到 v2 版本，优化检查规则配置
  - 启用 gofmt、goimports 格式化检查
  - 配置 gosec 安全扫描规则
  - 启用 staticcheck 静态分析
  - 优化 linter 配置减少误报噪音

#### 部署和配置
- 新增 Caddy 反向代理配置示例 (`deployments/caddy/Caddyfile`)
- 新增 systemd 服务配置（移至 `deployments/systemd/sssa.service`）
- 新增配置文件示例
  - `configs/sss-agent.yaml.example` - Agent 配置示例
  - `configs/sss-dashboard.yaml.example` - Dashboard 配置示例

#### 文档
新增完整的文档体系（8,500+ 行专业文档）：

**快速开始**
- `docs/getting-started.md` - 5 分钟快速开始指南

**部署指南**
- `docs/deployment/docker.md` - Docker 部署完整指南
- `docs/deployment/systemd.md` - systemd 服务配置指南
- `docs/deployment/manual.md` - 手动安装和配置指南
- `docs/deployment/proxy.md` - 反向代理配置（Nginx/Caddy/Apache）

**架构文档**
- `docs/architecture/overview.md` - 系统架构概览
- `docs/architecture/websocket.md` - WebSocket 通信设计
- `docs/architecture/data-flow.md` - 数据流程说明

**API 文档**
- `docs/api/rest-api.md` - REST API 接口文档
- `docs/api/websocket-api.md` - WebSocket API 协议文档

**开发指南**
- `docs/development/setup.md` - 开发环境搭建
- `docs/development/contributing.md` - 贡献指南
- `docs/development/docker-build.md` - Docker 构建说明
- `docs/development/testing.md` - 测试指南

**运维文档**
- `docs/troubleshooting.md` - 常见问题和故障排除
- `docs/maintenance.md` - 系统维护指南

**其他**
- `scripts/README.md` - 脚本使用说明文档
- 全面改进 `README.md`
  - 新增项目徽章（构建状态、许可证、版本等）
  - 新增特性列表和亮点介绍
  - 新增 5 分钟快速开始指南
  - 新增常见问题解答 (FAQ)
  - 新增文档导航和链接
  - 改进架构说明和技术栈介绍
  - 新增截图和演示
- 新增 `LICENSE` 文件（MIT License）

### 🔧 优化 (Changed)

#### 架构重构
- 重构项目结构为标准 Go 项目布局
  - 移动 `agent/` → `cmd/agent/` + `internal/agent/`
  - 移动 `dashboard/` → `cmd/dashboard/` + `internal/dashboard/`
  - 新增 `internal/shared/` 共享模块
  - 新增 `pkg/` 公共包（如有需要）
- 统一 agent 和 dashboard 的依赖管理
  - 删除 `go.work` 和 `go.work.sum`
  - 使用单一 `go.mod` 管理所有依赖
- 重构 WebSocket 管理器
  - 分离前端和后端通道管理
  - 改进连接状态跟踪
  - 优化心跳和重连机制
- 重构错误处理机制
  - 统一错误类型定义
  - 统一错误响应格式
  - 改进错误日志记录
- 优化配置加载逻辑
  - 移除 viper 库的冗余引用
  - 简化配置文件解析
  - 改进默认值处理
- 简化 API 路由结构
  - 合并冗余路由
  - 统一路由命名规范
  - 改进中间件组织

#### 依赖升级
- 升级 gopsutil 从 v3 到 v4
  - 适配新的 API 接口
  - 改进系统信息采集
  - 提升性能和稳定性
- 升级前端依赖包到最新稳定版本
  - Vue 3 相关包
  - Vite 构建工具
  - TypeScript 类型定义

#### CI/CD 优化
- 优化 GitHub Actions CI 工作流
  - 添加 Node.js 环境配置支持前端构建
  - 简化工作流配置减少重复代码
  - 更新 GoReleaser action 到 v6 版本
  - 改进构建缓存策略
- 改进 PowerShell 安装脚本
  - 优化错误处理和异常捕获
  - 改进跨平台兼容性检测
  - 增强用户体验和输出信息

#### UI/UX 改进
- 优化 `App.vue` 主布局
  - 改进响应式设计
  - 适配移动端显示
  - 使用 CSS Grid/Flexbox 布局
- 改进服务器信息展示组件
  - 更清晰的信息层级
  - 更好的视觉效果
  - 响应式卡片布局
- 使用 CSS 变量优化样式系统
  - 统一颜色方案
  - 支持主题切换（为未来功能做准备）
  - 改进可维护性
- 改进移动端适配
  - 响应式字体大小
  - 触摸友好的交互
  - 优化小屏幕布局

### 🐛 修复 (Fixed)

- 修复网络统计的并发安全问题
  - 添加互斥锁保护共享数据
  - 修复数据竞争条件
  - 改进线程安全性
- 修复 WebSocket 路径处理逻辑
  - 标准化路径格式（自动添加前导斜杠）
  - 改进路径验证
  - 向后兼容旧配置格式
- 修复配置验证的边界情况
  - 改进空值处理
  - 改进类型验证
  - 提供更友好的错误提示
- 标准化 API 响应属性命名
  - 统一使用驼峰命名
  - 保持一致的响应结构
  - 改进 JSON 序列化

### 🗑️ 移除 (Removed)

- 移除旧的 monorepo 结构
  - 删除顶层 `agent/` 目录
  - 删除顶层 `dashboard/` 目录
  - 合并到统一的项目结构
- 移除各模块独立的 goreleaser 配置
  - 删除 `agent/.goreleaser.yml`
  - 删除 `dashboard/.goreleaser.yml`
  - 使用统一的 `.goreleaser.yml`
- 移除旧的构建脚本
  - 删除 `build.sh`
  - 使用新的 Makefile 和构建脚本
- 移除 Go workspace 配置
  - 删除 `go.work`
  - 删除 `go.work.sum`
- 移除冗余的配置文件
  - 清理不再使用的配置示例
  - 整合配置到 `configs/` 目录

### 📊 代码统计

- **变更文件**: 143 个（核心功能）+ 25 个（代码质量改进）
- **新增代码**: 26,343 行
- **删除代码**: 2,159 行
- **净增代码**: 24,184 行
- **新增测试**: 3,700 行（8 个测试文件）
- **新增文档**: 8,500+ 行（20+ 个文档文件）
- **代码质量改进**:
  - 修复的 linter 问题: 50+ 处
  - 格式化的源文件: 50+ 个
  - 安全问题修复: 10+ 种类型
- **测试文件**:
  - `internal/agent/validator_test.go`
  - `internal/agent/network_stats_test.go`
  - `internal/dashboard/config_validator_test.go`
  - `internal/shared/errors/handler_test.go`
  - `internal/shared/errors/types_test.go`
  - `internal/shared/logger/logger_test.go`
  - `internal/shared/config/loader_test.go`
  - `internal/shared/config/validator_test.go`

### 🔄 迁移指南

**向后兼容性**: ✅ 本次更新保持完全向后兼容

- ✅ 配置文件格式保持兼容
- ✅ API 接口保持兼容
- ✅ WebSocket 协议保持兼容
- ✅ 数据模型保持兼容

**推荐操作**：

1. **使用 Docker 部署的用户**
   ```bash
   docker pull ruanun/sssd:v1.2.0
   docker-compose up -d
   ```

2. **使用二进制部署的用户**
   - 下载最新的二进制文件
   - 替换旧版本文件
   - 重启服务

3. **新用户**
   - 使用一键安装脚本快速部署
     ```bash
     # Linux/macOS/FreeBSD
     curl -fsSL https://raw.githubusercontent.com/ruanun/simple-server-status/main/scripts/install-agent.sh | sudo bash

     # Windows (PowerShell 管理员)
     iwr -useb https://raw.githubusercontent.com/ruanun/simple-server-status/main/scripts/install-agent.ps1 | iex
     ```
   - 参考快速开始指南 5 分钟完成部署

**可选配置**：
- 查看新的配置示例文件 (`configs/*.yaml.example`)
- 参考文档启用新特性（如国际化）
- 配置反向代理（参考 `docs/deployment/proxy.md`）

**注意事项**：
- 无需修改现有配置文件
- 建议查看新文档了解改进的功能
- 建议运行测试确保系统正常工作

### 📚 文档链接

- [📖 快速开始](docs/getting-started.md) - 5 分钟部署指南
- [🐳 Docker 部署](docs/deployment/docker.md) - 容器化部署完整指南
- [🔧 systemd 配置](docs/deployment/systemd.md) - Linux 服务配置
- [🌐 反向代理](docs/deployment/proxy.md) - Nginx/Caddy/Apache 配置
- [🏗️ 架构概览](docs/architecture/overview.md) - 系统设计说明
- [🔌 WebSocket 设计](docs/architecture/websocket.md) - 实时通信架构
- [📡 REST API](docs/api/rest-api.md) - HTTP API 接口文档
- [💬 WebSocket API](docs/api/websocket-api.md) - WebSocket 协议文档
- [💻 开发环境搭建](docs/development/setup.md) - 开发者指南
- [🤝 贡献指南](docs/development/contributing.md) - 如何参与贡献
- [🔍 故障排除](docs/troubleshooting.md) - 常见问题解决
- [🛠️ 维护指南](docs/maintenance.md) - 系统维护说明

### 🙏 致谢

感谢所有为本项目做出贡献的开发者和用户！

---

## [1.1.0] - 之前版本

（早期版本的变更记录可以从 git 历史中补充）

---

## [1.0.0] - 初始版本

（早期版本的变更记录可以从 git 历史中补充）
