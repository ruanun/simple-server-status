# 状态页与后台功能增强设计

- 日期：2026-09-29
- 作者：ruan
- 分支：`develop`（基于 v2.0.0-beta.1 之后的提交）
- 上位设计：`docs/superpowers/specs/2026-09-25-rewrite-design.md`（本文未提及之处仍以其为准）

## 1. 目标与范围

在保持「简洁但不简陋」的前提下补全状态页与后台的信息，并增加通知能力。完成后发布 `v2.0.0-beta.2`。

**用户已确认的 9 项功能**

1. 后台显示 Agent 版本并标出旧版本，提供「升级」入口（复用安装命令，不做远程自动升级）。
2. 详情页显示 1 / 5 / 15 分钟负载与交换分区使用。
3. 状态页卡片视图支持排序。
4. 详情页「当月每日流量」柱状图。
5. 在线率：公开页与后台都显示。
6. 后台费用汇总与即将到期列表。
7. 通知：离线 / 恢复、持续高负载 / 恢复、即将到期、月流量超阈值；渠道为 Webhook 与 Telegram；全局规则 + 单机静音。
8. 服务器备注（仅后台可见）。
9. 公开页公告（纯文本，自动识别链接）。

**不做（YAGNI）**：自定义通知模板、通知历史页、按服务器覆盖阈值、其他通知渠道（邮件、Bark 等，可经 Webhook 转发）、Agent 远程自动升级、汇率换算、上位设计中的其余非目标（延迟监控、多用户等）。

## 2. 数据与接口

### 2.1 数据库（迁移 `002`）

- `servers` 增加列：
  - `note TEXT NOT NULL DEFAULT ''`：备注，仅后台可见，最长 2000 字符。
  - `notify_muted INTEGER NOT NULL DEFAULT 0`：该服务器不发送任何通知。
- 新表 `traffic_daily`：
  ```sql
  CREATE TABLE traffic_daily (
    server_id TEXT NOT NULL,
    day       TEXT NOT NULL,          -- 本地日期 YYYY-MM-DD
    in_bytes  INTEGER NOT NULL DEFAULT 0,
    out_bytes INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (server_id, day)
  ) WITHOUT ROWID;
  ```
  流量累计器在累加月流量的同时累加当天增量（计数器归零、过滤口径变化的处理与月流量一致），与月流量一起落库；保留 400 天，由现有每分钟维护任务清理。删除服务器时一并删除。
- 新表 `notify_state`：
  ```sql
  CREATE TABLE notify_state (
    server_id TEXT NOT NULL,
    rule      TEXT NOT NULL,          -- expire / traffic
    key       TEXT NOT NULL,          -- 到期日时间戳 / 计费周期
    sent_at   INTEGER NOT NULL,
    PRIMARY KEY (server_id, rule, key)
  ) WITHOUT ROWID;
  ```
  记录「某到期日 / 某计费周期已提醒过」，重启后不重复提醒；删除服务器时一并删除。
- `settings` 表只新增键，不改结构：`announcement` 与 `notify_*` 一组（见 3.1）。

### 2.2 接口

- `ServerView`（公开列表、WS 快照 / delta）增加 `uptime_24h: number | null`（百分比，0–100，保留 1 位小数；无数据为 null）。`metrics` 已含 `load5`、`load15`、`swap_used`，前端直接使用，不改接口。
- 新增 `GET /api/public/servers/{id}/stats`（隐藏服务器对未登录用户 404，规则同现有详情接口）：
  ```json
  { "uptime_24h": 99.8, "uptime_7d": 99.5,
    "period_start": "2026-09-01", "period_end": "2026-09-30",
    "daily": [{ "day": "2026-09-01", "in": 123, "out": 45 }] }
  ```
  `daily` 覆盖当前计费周期（由 `traffic_reset_day` 决定）从开始到今天的每一天，缺失的日期补 0。
- `GET /api/public/site` 增加 `announcement: string`。
- `GET /api/admin/servers` 每项增加 `agent_version: string`（来自静态信息）、`outdated: bool`（Agent 版本低于 Dashboard 版本；任一方不是正式发布版本号时为 false）、`uptime_24h`；`note`、`notify_muted` 随 Server 字段返回。
- 服务器新建 / 编辑的输入增加 `note`、`notify_muted`。
- 新增 `GET /api/admin/overview`：
  ```json
  { "monthly_cost": [{ "currency": "$", "amount": 12.5, "servers": 3 }],
    "expiring": [{ "id": "…", "name": "…", "expire_at": 1790000000, "days": 5 }] }
  ```
  月均费用：`monthly` ×1、`quarterly` ÷3、`yearly` ÷12；`once`、未设置周期或未设价格的不计入。按币种原样分组（不做换算），币种为空归为一组。`expiring` 为 30 天内到期（含已过期但未超过 30 天）的服务器，按剩余天数升序。
- 新增 `POST /api/admin/notify/test`：使用请求体中的通知设置（未保存的也可测试）同步发送一条测试消息，返回 `{ "webhook": "ok" | 错误信息 | null, "telegram": … }`（未启用的渠道为 null）。
- 设置接口 `GET/PUT /api/admin/settings` 增加 `announcement` 与通知设置字段。
- 导入 / 导出包含 `note`、`notify_muted`、`announcement` 与全部通知设置（含 Telegram Token）；导出文件版本仍为 1，旧文件缺少的字段取默认值。

### 2.3 在线率

- 24 小时：`metrics_1m` 中窗口内有数据的分钟数 ÷ 窗口分钟数。
- 7 天：`metrics_10m` 同理。
- 窗口起点取「当前时间 − 窗口长度」与「服务器创建时间」中较晚者；窗口长度不足 1 个采样单位时返回 null。
- 结果上限 100。公开列表的 `uptime_24h` 在内存中缓存，每分钟随维护任务刷新一次，避免每次请求查库。

## 3. 通知模块

### 3.1 规则与设置

在「设置 → 通知」统一配置；每条规则独立开关。

| 规则 | 触发 | 恢复 | 默认 | 取值范围 |
|---|---|---|---|---|
| `offline` 离线 | 离线持续超过 N 分钟 | 恢复在线时通知，附离线时长 | 开，N=3 | 1–1440 |
| `load` 高负载 | CPU / 内存 / 硬盘任一项最近 M 分钟平均值 ≥ 各自阈值 | 所有项回落到阈值以下时通知 | 开，90/90/90%，M=5 | 阈值 1–100，M 1–10 |
| `expire` 即将到期 | 距到期 ≤ D 天 | 无；每个到期日只提醒一次 | 开，D=7 | 1–90 |
| `traffic` 月流量 | 设有配额且本周期用量 ≥ P% | 无；每个计费周期只提醒一次 | 开，P=90 | 1–100 |

设置键（均存于 `settings` 表）：`notify_webhook_url`、`notify_telegram_token`、`notify_telegram_chat_id`、`notify_lang`（`zh-CN` / `en-US`，默认 `zh-CN`）、`notify_offline_enabled`、`notify_offline_minutes`、`notify_load_enabled`、`notify_load_cpu`、`notify_load_mem`、`notify_load_disk`、`notify_load_minutes`、`notify_expire_enabled`、`notify_expire_days`、`notify_traffic_enabled`、`notify_traffic_percent`。Webhook URL 为空表示不启用 Webhook；Token 或 Chat ID 为空表示不启用 Telegram。

语义细节：

- 离线时长从最后一次上报时间（实时上报时间，缺失时取数据库 `last_seen`）算起；从未上报过的服务器不参与离线判断。
- 高负载使用 hub 中最近 10 分钟的逐次上报点（`Hub.Ring`），只取最近 M 分钟内的点计算平均；点数少于 2 时不判断。
- 离线与高负载是成对的「告警 / 恢复」状态，保存在内存中。Dashboard 启动后的第一轮检查只建立基线、不发送，避免重启时刷屏。
- 到期与流量的「已提醒」记录写入 `notify_state`；修改到期日后键不同，会重新提醒。
- `notify_muted` 的服务器跳过全部规则；隐藏服务器照常通知。删除服务器时清理其内存状态与 `notify_state`。

### 3.2 渠道与发送

- Webhook：`POST` JSON：`{ "event": "offline|recovered|load|load_recovered|expire|traffic|test", "server_id", "server_name", "title", "message", "time" }`，`Content-Type: application/json`。
- Telegram：`POST https://api.telegram.org/bot<token>/sendMessage`，`chat_id` + 纯文本 `text`（不使用 parse_mode）。
- 发送走后台队列：单次请求超时 10 秒；失败后按 5 秒、30 秒、2 分钟重试，共 3 次，最终失败记日志；队列容量 100，满时丢弃最旧一条并记日志。
- 测试接口同步发送、不重试，直接返回各渠道结果。
- Telegram Token 与 Webhook URL 不写入日志（日志中以「Webhook / Telegram 发送失败」加错误类型描述）。

### 3.3 代码结构

`internal/dashboard/notify/`：

- `rules.go`：纯函数。输入当前观测（在线与离线时长、最近负载点、到期日、流量用量）与上次状态，输出新状态与待发送事件。
- `message.go`：按语言生成标题与正文。
- `sender.go`：`Channel` 接口（Webhook、Telegram 两种实现）与带重试的发送队列。
- `notifier.go`：每 30 秒一轮的检查循环，组合 hub、流量累计器、服务器缓存、设置与 `notify_state`。

Notifier 随 Dashboard 启动，优雅退出时停止检查并尽量发送完队列中的消息（最多等待 5 秒）。

## 4. 界面

### 4.1 状态页

- **公告**：`announcement` 非空时在汇总卡上方显示细边框提示框，保留换行，`http(s)://` 链接自动转为 `<a target="_blank" rel="noopener noreferrer">`，其余内容一律作为文本渲染（不使用 `dangerouslySetInnerHTML`）。
- **卡片**：第二行右侧、到期信息之前显示「在线率 99.8%」（24h），离线卡片同样显示；< 99% 琥珀色，< 95% 红色；无数据不显示。
- **列表视图**：增加「在线率」列，可按表头排序。
- **排序**：筛选栏「卡片 / 列表」切换旁增加排序下拉：默认（后台顺序）、名称、CPU、内存、硬盘、流量、网速、到期、在线率；选择保存在 localStorage，与列表视图表头排序共用同一状态。

### 4.2 详情页

- 头部增加「在线率 24h 99.8% · 7d 99.5%」。
- 信息网格：
  - CPU 副行：「负载 0.50 / 0.42 / 0.38 · 进程 120 · TCP 30」。
  - 原「内存 / 硬盘 / 交换」合并项拆为三项：「内存」「交换」「硬盘」，均为 已用 / 总量 + 进度条；交换总量为 0 显示「未启用」。
  - 新增「网络累计」（开机以来 ↓ / ↑ 总量），网格为 3 × 3：系统、CPU、架构 · 虚拟化；内存、交换、硬盘；网络累计、本月流量、续费。
- 「当月每日流量」柱状图：位于历史趋势上方，灰阶堆叠显示入站与出站，标题显示周期范围（如「9/1 – 9/30」），设有配额时右上角显示「已用 400 GB / 1 TB」；数据来自 `/stats`，打开页面请求一次，每 5 分钟刷新。

### 4.3 后台

- **服务器列表**：
  - 上方总览条：「月均费用：$ 12.5 · ¥ 30」「30 天内到期 N 台」，点击后者展开列表（名称、剩余天数）。无数据时总览条对应项显示「—」。
  - 新增列「Agent」（版本号，`outdated` 时琥珀色并带提示）与「在线率」（24h）。
  - 名称单元格：有备注时显示备注图标，悬停显示全文；`notify_muted` 时显示静音图标（均带可读标签）。
  - 行操作菜单：`outdated` 时出现「升级」，打开安装命令弹窗，标题为「升级 Agent」，说明「在目标机器重新执行即可覆盖升级」。
- **服务器表单**：「基本」组增加「备注」（多行）；「采集」组增加「不发送通知」开关。
- **设置**：
  - 「站点」增加「公告」（多行，最长 1000 字符）。
  - 新增「通知」区块：Webhook URL；Telegram Bot Token（密码框，可切换显示）与 Chat ID；通知语言；四条规则的开关与阈值；「发送测试」按钮（使用表单当前值）。

## 5. 错误处理

- 通知检查中单台服务器出错只记日志并跳过，不影响本轮其他服务器与下一轮。
- 设置保存时校验：Webhook URL 为空或 `http(s)` 地址；Telegram Token 与 Chat ID 必须同时为空或同时非空；阈值在 3.1 的取值范围内；公告 ≤ 1000 字符。备注 ≤ 2000 字符。违规返回 `invalid_input`，消息指明字段。
- `/stats` 与 `/overview` 查询失败返回 `internal`；前端对应区域显示错误信息，页面其他部分正常。
- Telegram Token 属于敏感信息：公开接口不返回，日志不输出；后台设置接口返回以便编辑；导出包含。

## 6. 测试

- Go：
  - `notify/rules.go` 表驱动：阈值边界、恢复、静音、启动首轮不发送、到期 / 流量每周期一次、修改到期日后重新提醒、负载点不足不判断。
  - `notify/sender.go`：httptest 模拟 Webhook 与 Telegram，覆盖成功、失败重试、队列满丢弃最旧、日志不含 Token。
  - 在线率：新服务器、无数据、窗口边界。
  - 每日流量：累加、跨天、计数器归零、过滤口径变化、过期清理、删除服务器清理。
  - 月均费用折算与分组、到期列表排序。
  - API 集成：`/stats`、`/overview`、`/notify/test`、设置校验、服务器新字段、导入导出新字段、隐藏服务器 `/stats` 对未登录 404。
- 前端（Vitest）：公告链接识别与 XSS 输入按文本渲染；排序各字段与视图间共享；详情页在线率、三段负载、交换、每日流量图；后台总览条、Agent 旧版本与「升级」、备注与静音图标；设置页通知表单校验与测试按钮。
- e2e：现有用例保持通过；后台用例扩展为设置公告后首页可见。

## 7. 文档

更新 `docs/api.md`（新接口与字段）、`docs/configuration.md`（通知设置、公告、备注、静音）、README 特性列表；CHANGELOG 在 2.0.0 条目下补充。
