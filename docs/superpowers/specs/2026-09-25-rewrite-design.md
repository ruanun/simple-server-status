# Simple Server Status 整体重写设计

- 日期：2026-09-25
- 作者：ruan
- 分支：`dev-2.0`（重写前快照：`17314bc`，备份分支 `backup/dev-2.0-react`）

## 1. 目标与约束

**目标**：对 Agent、Dashboard、前端进行干净重写，得到一个"简洁但不简陋"的极简探针。

**用户明确的要求**

- 当前前端太难看，整体重写，不考虑兼容（尚未上线）。
- 参考：qoder 分支 web、`sss-dasboard/` 原型、pika、CF-Server-Monitor、m.3301921.xyz。
- 功能范围：标准档——实时状态 + 历史图表 + 月流量配额/到期/价格 + 分组/地区筛选 + 管理后台。
- 视觉：风格 A「极简单色」（参考 m.3301921）；详情页采用独立页面 `/server/:id`。
- 历史数据：分级降采样；公开页：公开 + 单机可隐藏。
- 架构：两个程序 + Agent 通过 WebSocket 长连接上报（方案 1）。
- 依赖：保留 gin（+ 所需 gin-contrib）和 lumberjack，其余按本文精简。

**成功标准**

1. 首页一眼可见全局状态（汇总卡）与每台机器的核心指标（2×2 指标卡片）。
2. 详情页可查看实时 / 1h / 6h / 24h / 7d 历史趋势。
3. 部署保持简单：Dashboard 单文件 + SQLite，Agent 单文件；全新 clone 后 `make build` 可直接成功。
4. 代码量明显少于当前实现，无死代码、无未使用依赖。

**非目标（YAGNI）**：告警、延迟/丢包监控、审计日志页面、多用户/RBAC、地图视图、WebSSH、主题商店、视觉截图回归测试。

## 2. 整体结构与依赖

### 2.1 依赖

| 用途 | 选型 |
|---|---|
| HTTP | gin（按需使用 gin-contrib，如 gzip） |
| WebSocket | `github.com/coder/websocket`（Agent 与浏览器共用） |
| 数据库 | `database/sql` + `modernc.org/sqlite`（纯 Go，无 CGO），手写 SQL |
| 日志 | 标准库 `log/slog`，文件输出使用 lumberjack 轮转 |
| 命令行 / 配置 | `spf13/cobra`（子命令与参数）；环境变量 `SSS_*` 映射为参数默认值；Agent 额外用 `gopkg.in/yaml.v3` 读取 YAML。优先级：命令行 > 环境变量 > YAML > 默认值 |
| 系统采集 | `gopsutil/v4` |
| 鉴权 | `golang-jwt/jwt/v5` + `golang.org/x/crypto/bcrypt` |

移除：viper、zap、gin-contrib/zap、melody、gorilla/websocket、gorm、glebarez/sqlite。

### 2.2 目录

```
cmd/
  sss-agent/main.go
  sss-dashboard/main.go
internal/
  proto/            消息类型与编解码（协议版本 v=1）
  logx/             slog + lumberjack 初始化（两端共用）
  agent/
    config.go       YAML/参数/环境变量
    collect/        指标采集（移植 gopsutil.go、network_stats.go 的过滤规则）
    client.go       连接、重连、应用下发参数、上报循环
  dashboard/
    config.go       启动参数
    store/          SQLite 访问与版本化迁移
    hub/            内存实时状态、在线判定、秒级环形缓冲、浏览器广播
    history/        降采样写入与过期清理
    traffic/        月流量累计
    auth/           JWT、bcrypt、登录限流
    api/            gin 路由与处理函数
    web/            go:embed 前端产物（dist/.gitkeep 提交入库）
web/                React 前端
scripts/            install-agent.sh / install-agent.ps1、构建脚本
deployments/docker/ Dockerfile 与 compose
```

### 2.3 删除清单（重写中执行）

`internal/shared/`、`pkg/model/`、现有 `internal/agent/*` 与 `internal/dashboard/*`（按需移植后删除）、`cmd/agent`、`cmd/dashboard`、`web/src/` 现有 UI（仅迁移 `RealtimeClient` 思路与 i18n 词条）、`web/e2e/` 视觉快照、`scripts/check-web-bundle*`、`scripts/e2e-*`（由新测试替代）、`docs/2.0-PLAN.md`、`docs/fix-plan-2025-11-22.md`、旧 specs/plans。

## 3. 协议与数据模型

### 3.1 Agent ↔ Dashboard

- 地址：`{dashboard}/api/agent/ws`（http→ws、https→wss 由 Agent 转换）。
- 鉴权：请求头 `Authorization: Bearer <server_id>:<secret>`。Dashboard 以常数时间比较 secret（`subtle.ConstantTimeCompare`）。
- 同一 server_id 新连接接入时，关闭旧连接。
- 所有消息：`{"v":1,"type":"<type>","data":{...}}`，字段 snake_case。未知 `type` 忽略并记 debug 日志；`v` 不匹配时 Dashboard 关闭连接并返回原因。

| 方向 | type | data |
|---|---|---|
| A→D | `hello` | `os, platform, platform_version, kernel, arch, virtualization, cpu_model, cpu_cores, mem_total, swap_total, disk_total, agent_version, country`（country 由 Agent 可选探测，可关闭） |
| D→A | `config` | `report_interval`（秒，默认 2，范围 1–60）、`nic_include[]`、`nic_exclude[]`、`mount_exclude[]` |
| A→D | `report` | `ts, cpu, load1, load5, load15, mem_used, swap_used, disk_used, disk_total, disks[{mount,fstype,total,used}], net_in_speed, net_out_speed, net_in_total, net_out_total, procs, tcp, udp, uptime` |
| D→A | `stop` | `reason`（`deleted`），服务器被删除时下发，Agent 收到后退出进程。重置密钥不发 stop，直接断开旧连接（旧密钥重连会得到 401） |

- 连接建立：Agent 发 `hello` → Dashboard 回 `config` → Agent 按间隔发 `report`。后台修改参数后 Dashboard 立即推送新的 `config`。
- 保活：Dashboard 每 20s ping，40s 无 pong 断开。
- 单条消息上限 64KB。

### 3.2 在线判定

在线 = 连接存在 且 最近一次 `report` 距今 ≤ 3 × report_interval（最小 6s）。连接断开立即离线。

### 3.3 SQLite 表

```sql
servers(
  id TEXT PRIMARY KEY,           -- 随机 12 位
  name TEXT NOT NULL, secret TEXT NOT NULL,
  grp TEXT NOT NULL DEFAULT '', country TEXT NOT NULL DEFAULT '',
  sort INTEGER NOT NULL DEFAULT 0, hidden INTEGER NOT NULL DEFAULT 0,
  price REAL, currency TEXT, billing_cycle TEXT,   -- monthly/quarterly/yearly/once
  expire_at INTEGER,                               -- unix 秒，NULL=长期
  traffic_limit INTEGER,                           -- 字节，NULL=不限
  traffic_mode TEXT NOT NULL DEFAULT 'sum',        -- sum/in/out
  traffic_reset_day INTEGER NOT NULL DEFAULT 1,    -- 1–28
  report_interval INTEGER NOT NULL DEFAULT 2,
  nic_include TEXT, nic_exclude TEXT, mount_exclude TEXT,  -- JSON 数组
  static_info TEXT,                                -- 最近一次 hello 的 JSON
  last_ip TEXT, last_seen INTEGER,
  created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)

users(id INTEGER PRIMARY KEY, username TEXT UNIQUE, password_hash TEXT,
      token_version INTEGER NOT NULL DEFAULT 0, created_at INTEGER)

settings(key TEXT PRIMARY KEY, value TEXT)
  -- site_title, show_price, default_report_interval, install_script_base, jwt_secret

metrics_1m(server_id, ts, cpu, mem, disk, net_in, net_out, load1, tcp, PRIMARY KEY(server_id, ts))
metrics_10m(同上结构)

traffic_monthly(server_id, period TEXT,  -- 周期起始日 'YYYY-MM-DD'
  in_bytes INTEGER, out_bytes INTEGER,
  last_in_total INTEGER, last_out_total INTEGER, PRIMARY KEY(server_id, period))

schema_version(version INTEGER)
```

- secret 明文存储（数据库文件即敏感资产，需做好 `data/` 目录权限）：后台可随时查看安装命令；重置密钥后旧 Agent 连接立即断开。
- 迁移：`store/migrations/NNN_*.sql` 按版本顺序执行。
- SQLite 开 WAL、`busy_timeout=5000`，单写连接。

### 3.4 历史降采样

- 内存：每台服务器保留最近 10 分钟的原始 `report`（环形缓冲），供「实时」图和汇总卡 sparkline。
- 每满 1 分钟：对该分钟原始数据取平均写入 `metrics_1m`（cpu/mem/disk 为百分比，net 为字节/秒平均值）。
- 每满 10 分钟：由 `metrics_1m` 聚合写入 `metrics_10m`。
- 清理：每小时删除 `metrics_1m` 中 >24h、`metrics_10m` 中 >7d 的数据。
- 查询（点数均 ≤ 1008）：

| range | 数据源 | 返回粒度 |
|---|---|---|
| realtime | 内存环形缓冲 | 原始（约 300 点） |
| 1h | metrics_1m | 1 分钟（60 点） |
| 6h | metrics_1m | 1 分钟（360 点） |
| 24h | metrics_1m | 服务端聚合为 5 分钟（288 点） |
| 7d | metrics_10m | 10 分钟（1008 点） |

### 3.5 月流量

- 周期起点由 `traffic_reset_day` 决定（每月该日 00:00，按 Dashboard 本地时区）。
- 每次 `report`：`delta = total - last_total`；若 `delta < 0`（Agent 重启或计数器归零），`delta = total`。累加到当前周期；更新 `last_total`。
- 写库节流：内存累加，每 60s 或连接断开时落库。
- 使用量：`sum` = in+out，`in` = in，`out` = out；百分比 = 使用量 / traffic_limit。

### 3.6 浏览器 ↔ Dashboard

- 响应：成功 `200 {"data": ...}`；失败 4xx/5xx `{"error":{"code":"snake_case_code","message":"..."}}`。Go 端所有切片序列化为 `[]`，不输出 `null`。
- 公开接口（隐藏服务器仅在携带有效 JWT 时返回；从不返回 IP；`show_price=false` 时不返回价格字段）：

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/public/site` | 站点标题、show_price |
| GET | `/api/public/servers` | 全量快照：静态信息 + 最新指标 + 本月流量 + 在线状态 |
| GET | `/api/public/servers/{id}` | 单台详情（含分区） |
| GET | `/api/public/servers/{id}/metrics?range=realtime\|1h\|6h\|24h\|7d` | 历史序列 |
| GET | `/api/public/ws?token=` | 实时推送：连接后发 `{"type":"snapshot","data":[ServerView]}`；之后每 2s 发 `{"type":"delta","data":[ServerView]}`（仅有变化的服务器，含上下线）；服务器列表或设置变更后推送新的 snapshot |

- 管理接口（`Authorization: Bearer <jwt>`）：

| 方法 | 路径 |
|---|---|
| POST | `/api/auth/login` |
| GET | `/api/auth/me` |
| PUT | `/api/auth/password` |
| GET/POST | `/api/admin/servers` |
| PUT/DELETE | `/api/admin/servers/{id}` |
| PUT | `/api/admin/server-order`（批量排序，请求体 `{"ids":[...]}`） |
| POST | `/api/admin/servers/{id}/reset-secret` |
| GET | `/api/admin/servers/{id}/install?dashboard=<面板地址>`（返回 Linux / Windows 安装命令） |
| GET/PUT | `/api/admin/settings` |
| GET | `/api/admin/export` |
| POST | `/api/admin/import` |

- 静态资源：embed 的前端；非 `/api` 且无扩展名的 GET 返回 `index.html`（200）。

### 3.7 鉴权

- JWT HS256，有效期 7 天，payload 含 `uid` 与 `tv`（token_version）；改密码时 `token_version+1`，旧 token 失效。
- 首次启动：生成 `jwt_secret` 与 `admin` 随机密码，仅打印一次到日志；支持 `sss-dashboard reset-password` 子命令重置（生成新随机密码并使旧 token 失效）。
- 登录限流：同一 IP 5 分钟内失败 5 次后锁定 5 分钟。
- 客户端 IP：默认取 `RemoteAddr`；仅当 `--trusted-proxies` 配置时信任 `X-Forwarded-For`。

## 4. 后端运行参数

**Dashboard**（Cobra 子命令：默认 `serve`、`reset-password`、`version`；参数 / 环境变量）：`--listen`（`SSS_LISTEN`，默认 `:8900`）、`--data-dir`（`SSS_DATA_DIR`，默认 `./data`）、`--trusted-proxies`（`SSS_TRUSTED_PROXIES`）、`--admin-password`（`SSS_ADMIN_PASSWORD`，仅首次初始化管理员时使用，留空随机生成）、`--log-level`、`--log-file`（为空则只输出到 stdout）。运行期设置全部在后台修改并即时生效。

**Agent**（Cobra 子命令：默认 `run`、`version`；`--config` 指定 YAML，默认依次查找 `./sss-agent.yaml`、`/etc/sss/sss-agent.yaml`；参数 / 环境变量可覆盖 YAML）：`dashboard`、`id`、`secret`、`detect_country`（默认 true，通过 Cloudflare trace）、`log_level`、`log_file`。

## 5. 前端

### 5.1 技术栈

React 19、Vite、TypeScript（strict）、Tailwind v4、shadcn/ui（zinc，按需添加组件）、Recharts、TanStack Query、React Router、i18next（zh-CN / en-US）、`flag-icons`（SVG 国旗）。主题：亮 / 暗 / 跟随系统。

### 5.2 视觉规范（风格 A）

- 背景 zinc-50 / 暗色纯黑；卡片白底 + 1px zinc-200 边框，圆角 10px，无阴影。
- 仅以下元素使用彩色：在线点（绿）、离线标记（红）、国旗、阈值进度条（<70% 前景色，70–90% 琥珀，≥90% 红）、临近到期（≤7 天琥珀）。
- 数字使用 tabular-nums；图表灰阶。

### 5.3 页面

**`/` 首页**

- 顶栏：站点标题；右侧主题、语言、登录（登录后为「后台」）。实时连接中断时显示提示条。
- 汇总卡 ×4：在线/总数；最忙节点（CPU 最高）；本月流量（↓↑）；实时总网速 + 10 分钟 sparkline。
- 筛选栏：分组 / 地区 / 离线 chips（带计数）+ 搜索 + 卡片/列表切换（localStorage 记忆）。
- 卡片视图（1–4 列自适应）：名称 + 国旗 + 在线时长徽章；`OS · 虚拟化 · 架构` + 到期天数（长期显示 ∞）；2×2：CPU（核数、load）、内存（已用/总量）、硬盘（已用/总量）、流量（已用/配额，不限显示 ∞）；底部实时速率与累计流量。离线卡片半透明并显示最后上报时间。整卡可点进入详情。
- 列表视图：状态点、名称/国旗、系统、CPU、内存、硬盘、流量（细进度条）、↓↑ 速率、在线时长、到期；表头排序；窄屏降级为紧凑行。
- 排序：默认按 `sort`，其次名称。

**`/server/:id` 详情页**

- 头部：名称、国旗、在线徽章、Agent 版本、返回。
- 信息网格：系统/内核、CPU 型号×核数、内存/硬盘/交换、架构·虚拟化、本月流量 + 重置日、续费信息（受 show_price 控制）。
- 分区列表：挂载点、文件系统、进度条、已用/总量。
- 历史趋势：分段控件 实时 / 1h / 6h / 24h / 7d；单列 5 图：CPU、内存、网络（↓↑ 两线）、硬盘、负载 + TCP。

**`/login`**、**`/admin/servers`**、**`/admin/settings`**

- 后台：左侧窄侧栏（移动端抽屉）。
- 服务器：表格（拖拽排序、在线状态）；新建/编辑弹窗分「基本 / 计费 / 流量 / 采集」四组；创建与重置密钥后自动弹出安装命令，行操作中也可随时查看（Linux / Windows，一键复制）；删除二次确认。
- 设置：站点标题、公开价格、默认上报间隔、安装脚本地址；修改密码；导出 / 导入 JSON（包含服务器 secret，便于整机迁移，Agent 无需重装；导出前弹窗提示文件含密钥需妥善保管；导入时同 id 覆盖、新 id 新增）。

### 5.4 数据层

- `RealtimeClient`：WebSocket 指数退避重连，失败时降级 5s 轮询 `/api/public/servers`，页面隐藏时暂停。
- 快照与 delta 合并进 TanStack Query 缓存；历史数据按 range 查询，`realtime` 模式使用 WS 数据追加。

## 6. 错误处理

- Agent 采集：每项指标独立获取，失败记 0 并按项限频告警（每 10 分钟一次）；修复已知缺陷（主机信息 nil 解引用、CPU 空切片越界、除零 NaN）。
- Agent 重连：1s 起指数退避至 60s，±20% 抖动；401 时退避 5 分钟。
- Dashboard：后台任务（降采样、清理、流量落库）独立 goroutine，错误仅记日志；优雅退出时刷新流量与待写数据。
- 前端：统一 `ApiError`；401（非登录请求）跳转登录；空数据兼容。

## 7. 测试

- Go 单元测试（表驱动）：proto 编解码、在线判定、降采样聚合、月流量（计数器归零、跨周期、重置日）、限流、JWT token_version 失效。
- Go 集成测试：`api` 使用 httptest + 临时 SQLite；端到端冒烟：启动 Dashboard + 模拟 Agent，校验快照、详情、metrics 接口与 WS 推送。
- 前端 Vitest：格式化、阈值颜色、流量百分比、筛选/排序、快照合并。
- Playwright：首页 → 详情页；登录 → 新建服务器 → 显示安装命令；后端使用 Go 端的模拟 Agent。

## 8. 构建、发布与文档

- `make build`：构建 web → 输出到 `internal/dashboard/web/dist` → 构建两个 Go 程序。`dist/.gitkeep` 入库，全新 clone 可直接 `go build`。
- CI：golangci-lint → go test → web test → build。
- Dockerfile、docker-compose、goreleaser 按新目录调整。
- 安装脚本：`install-agent.sh --dashboard <url> --id <id> --secret <secret>`（ps1 同名参数），生成 YAML 与 systemd / Windows 服务。后台安装命令中的脚本地址来自设置 `install_script_base`，默认指向对应版本的 GitHub Release 资产。
- 文档：README + `docs/` 仅保留快速开始、部署、配置、API 四篇；更新 `CLAUDE.md`、`AGENTS.md`。

## 9. 仓库清理

执行前列出清单请用户确认。

- 删除：根目录与 `bin/` 下 exe、`*.log`、`coverage.out`、`.cache/`、`web/.tmp/`、`web/dist-react/`、`web/playwright-report/`、`web/test-results/`、`web/vite-review.*`、`sss-dasboard/`、未跟踪 `claude.md`。
- 保留：`data/`、`sss-agent.yaml`、`.idea/`、`.codex/`。
