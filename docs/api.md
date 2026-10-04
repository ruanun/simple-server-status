# API

Dashboard 对外提供三类接口：浏览器使用的 HTTP 接口、浏览器实时推送 `/api/public/ws`、Agent 上报 `/api/agent/ws`。字段名一律 snake_case，时间为 Unix 秒，流量与容量单位为字节。

## 响应格式

成功时 HTTP 200：

```json
{"data": ...}
```

失败时 HTTP 4xx / 5xx：

```json
{"error": {"code": "not_found", "message": "服务器不存在"}}
```

`message` 为中文说明，客户端应以 `code` 判断错误类型。列表字段为空时返回 `[]`，不返回 `null`。

| code | HTTP | 含义 |
|---|---|---|
| `bad_request` | 400 | 请求体不是合法 JSON 或格式不符（导入文件超过 10 MB 也返回此错误） |
| `invalid_input` | 400 | 字段校验失败，`message` 说明具体原因 |
| `bad_range` | 400 | `metrics` 的 `range` 不受支持 |
| `bad_version` | 400 | 导入文件的 `version` 不受支持 |
| `wrong_password` | 400 | 修改密码时原密码错误 |
| `weak_password` | 400 | 新密码少于 8 位 |
| `captcha_invalid` | 400 | 登录时验证码错误、已过期或未提交 |
| `turnstile_check_failed` | 400 | 保存设置时 Turnstile 核验未通过，`message` 说明原因 |
| `unauthorized` | 401 | 未登录、token 无效或已失效；Agent 鉴权失败 |
| `invalid_credentials` | 401 | 用户名或密码错误 |
| `not_found` | 404 | 服务器不存在，或接口不存在 |
| `too_many_attempts` | 429 | 登录失败次数过多 |
| `login_busy` | 429 | 全站登录尝试过多，暂时拒绝近期未成功登录过的来源 |
| `captcha_rate_limited` | 429 | 同一来源获取验证码过于频繁 |
| `internal` | 500 | 服务器内部错误 |
| `shutting_down` | 503 | 服务正在停止，拒绝新的 WebSocket 连接 |

## 鉴权

- `POST /api/auth/login` 返回 JWT（HS256，有效期 7 天），之后在请求头携带 `Authorization: Bearer <token>`。
- 修改密码或执行 `reset-password` 后，之前签发的 token 全部失效。
- 登录限流：
  - 同一来源 5 分钟内失败 5 次后锁定 5 分钟。来源按 IPv4 地址或 IPv6 的 /64 网段区分。
  - 近期（30 天内）未成功登录过的来源，5 分钟内合计尝试超过 30 次后，暂时拒绝这些来源（`login_busy`），防止换 IP 爆破；成功登录过的来源不受影响。以上记录保存在内存中，重启后清空。
  - 位于反向代理之后时需配置 `--trusted-proxies`，否则所有请求共用代理的 IP。
- 登录验证码：在后台「设置 → 账号与安全」中选择不启用、图形验证码或 Cloudflare Turnstile。启用后，登录页先调用 `GET /api/auth/captcha` 获取验证码，并随登录请求提交。验证码在限流之后校验，填错验证码同样计为一次登录失败。
  - 图形验证码保存在内存中，5 分钟内有效，无论对错只能使用一次，不区分大小写。同一来源每分钟最多获取 20 次，超出返回 `captcha_rate_limited`。
  - Turnstile 的 token 由 Dashboard 调用 Cloudflare 核验，因此 Dashboard 需要能访问 `challenges.cloudflare.com`。
  - 验证码配置有误导致无法登录时，执行 `sss-dashboard disable-captcha` 关闭验证码并重启 Dashboard。
- 公开接口不要求登录；携带有效 token 时可以看到隐藏服务器与价格。

## 公开接口

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/public/site` | 站点信息：`{"site_title", "show_price", "announcement"}` |
| GET | `/api/public/servers` | 所有可见服务器的 `ServerView` 数组 |
| GET | `/api/public/servers/:id` | 单台服务器的 `ServerView`；不存在或未登录访问隐藏服务器时返回 404 |
| GET | `/api/public/servers/:id/metrics?range=<range>` | 历史数据点数组，`range` 默认 `1h` |
| GET | `/api/public/servers/:id/stats` | 在线率与当月每日流量，见下文；不存在或未登录访问隐藏服务器时返回 404 |
| GET | `/api/public/servers/:id/outages` | 最近离线记录，见下文；不存在或未登录访问隐藏服务器时返回 404 |
| GET | `/api/public/ws` | 浏览器实时推送，见下文 |

可见性规则：隐藏服务器只对已登录用户返回；价格字段（`price`、`currency`、`billing_cycle`）在设置「公开显示价格」关闭时只对已登录用户返回；任何公开接口都不返回 IP 与密钥。

### ServerView

| 字段 | 说明 |
|---|---|
| `id`、`name`、`group`、`country`、`sort`、`hidden` | 服务器配置；`country` 为空时取 Agent 探测结果 |
| `online` | 是否在线：连接存在且最近一次上报距今不超过 3 倍上报间隔（最少 6 秒） |
| `last_seen` | 最近一次上报（或断开）时间 |
| `uptime_24h` | 最近 24 小时在线率（百分比，0–100，保留 1 位小数）；窗口不足一个采样周期或服务器刚创建时为 `null`，每分钟随维护任务刷新一次 |
| `ipv4`、`ipv6` | 是否探测到对应公网地址的布尔标记（地址本身不返回）；从未连接或 Agent 未上报时为 `false` |
| `static` | 静态信息，结构同 Agent 的 `hello`，但 `agent_version` 与 `ipv4`、`ipv6` 地址始终为空（Agent 版本只在后台服务器列表中提供，IP 地址是否存在见上面的 `ipv4`/`ipv6` 标记）；从未连接过时为 `null` |
| `metrics` | 最新一次上报，结构同 Agent 的 `report`（不含 `filter_id`）；无数据时为 `null` |
| `traffic` | 当前周期流量：`in`、`out`、`used`（按 `mode` 计算）、`limit`（`null` 表示不限）、`mode`（`sum`/`in`/`out`）、`reset_day`、`period`（周期起始日 `YYYY-MM-DD`） |
| `expire_at` | 到期时间，`null` 表示长期 |
| `price`、`currency`、`billing_cycle` | 仅在可见时出现 |

### metrics 的 range

| range | 数据来源 | 粒度 |
|---|---|---|
| `realtime` | 内存中最近 10 分钟的原始上报 | 每次上报一个点 |
| `1h` | 分钟表 | 1 分钟 |
| `6h` | 分钟表 | 1 分钟 |
| `24h` | 分钟表，服务端聚合 | 5 分钟 |
| `7d` | 10 分钟表 | 10 分钟 |

分钟级数据保留 24 小时，10 分钟级数据保留 7 天。每个点：

```json
{"ts": 1790000000, "cpu": 12.5, "mem": 43.1, "disk": 61.0, "net_in": 1024, "net_out": 2048, "load1": 0.3, "tcp": 25}
```

`cpu`、`mem`、`disk` 为百分比，`net_in`、`net_out` 为字节每秒。

### GET /api/public/servers/:id/stats

```json
{
  "uptime_24h": 99.8, "uptime_7d": 99.5,
  "period_start": "2026-09-01", "period_end": "2026-09-30",
  "daily": [{"day": "2026-09-01", "in": 123, "out": 45}]
}
```

`uptime_24h`、`uptime_7d` 含义同 `ServerView.uptime_24h`，窗口分别为 24 小时、7 天（7 天在线率取 10 分钟级数据）。`period_start`、`period_end` 为当前计费周期（由服务器的 `traffic_reset_day` 决定）的起止日期（Dashboard 本地时区 `YYYY-MM-DD`）。`daily` 覆盖该周期从开始到今天的每一天，缺失的日期补 0，`in`、`out` 为当天入站、出站字节数。

### GET /api/public/servers/:id/outages

```json
[{"start_at": 1790000000, "end_at": 1790000300, "duration": 300}]
```

最近 10 条离线记录（即类型为 `offline` 的事件），按开始时间倒序；`end_at` 为 `null` 表示仍在离线中，此时 `duration` 按当前时间计算。离线门槛（持续 60 秒才记录）与保留期（90 天）见 [配置 - 事件](configuration.md#事件)。公开接口只提供离线记录，其他类型的事件仅在后台可见。

## 登录与管理接口

除 `POST /api/auth/login` 与 `GET /api/auth/captcha` 外均需要 `Authorization: Bearer <token>`。

| 方法 | 路径 | 请求 | 响应 `data` |
|---|---|---|---|
| POST | `/api/auth/login` | `{"username", "password"}`；启用图形验证码时加 `captcha_id`、`captcha_code`，启用 Turnstile 时加 `turnstile_token` | `{"token", "username"}` |
| GET | `/api/auth/captcha` | — | 未启用：`{"mode": "none"}`；图形验证码：`{"mode": "image", "id", "image"}`（`image` 为 PNG 的 data URL，每次调用生成新的验证码）；Turnstile：`{"mode": "turnstile", "site_key"}` |
| GET | `/api/auth/me` | — | `{"username", "version"}`（`version` 为 Dashboard 版本，仅登录后可见） |
| PUT | `/api/auth/password` | `{"old_password", "new_password"}` | `{"token"}`（新 token，旧 token 失效） |
| GET | `/api/admin/servers` | — | `Server` 数组，每项额外带 `online`、`agent_version`（当前连接或最近一次连接的 Agent 版本，未连接过为空）、`outdated`（Agent 版本低于 Dashboard 版本；任一方不是正式发布版本号时为 `false`）、`uptime_24h`（同 `ServerView`） |
| POST | `/api/admin/servers` | `ServerInput` | 新建的 `Server`（含自动生成的 `id` 与 `secret`） |
| PUT | `/api/admin/servers/:id` | `ServerInput` | 更新后的 `Server`；在线的 Agent 会立即收到新的 `config` |
| DELETE | `/api/admin/servers/:id` | — | `{}`；同时删除历史、流量、通知状态、事件与通知记录，在线的 Agent 收到 `stop` 后退出 |
| POST | `/api/admin/servers/:id/reset-secret` | — | `{"secret"}`；旧密钥立即失效，当前连接被断开 |
| GET | `/api/admin/servers/:id/install?dashboard=<面板地址>` | — | `{"linux", "windows"}` 两条安装命令 |
| PUT | `/api/admin/server-order` | `{"ids": [...]}` | `{}`；按数组顺序设置 `sort` |
| GET | `/api/admin/settings` | — | `Settings` |
| PUT | `/api/admin/settings` | `Settings`；新启用 Turnstile 或修改其密钥时加 `turnstile_token`（见下文） | 保存后的 `Settings` |
| POST | `/api/admin/notify/test` | `NotifySettings` | `{"webhook": "ok" \| 错误信息 \| null, "telegram": ...}`，见下文；每个已启用渠道都会写入一条通知记录 |
| GET | `/api/admin/export` | — | 导出文件（见下文） |
| POST | `/api/admin/import` | 导出文件 | `{"servers": <导入数量>, "captcha_kept": <是否保留了当前验证码设置>}` |
| GET | `/api/admin/overview` | — | `{"monthly_cost": [...], "expiring": [...]}`，见下文 |
| GET | `/api/admin/events` | 查询参数：`server_id`（可选）、`kind`（可选，逗号分隔多个）、`page`、`size` | `{"items": [AdminEvent, ...], "total": N}`，见下文 |
| GET | `/api/admin/notify-log` | 查询参数：`server_id`（可选）、`status`（可选，`pending`/`sent`/`failed`）、`page`、`size` | `{"items": [NotifyLog, ...], "total": N}`，见下文 |

`install` 的 `dashboard` 参数必须是 `http(s)://主机[:端口][/路径]` 形式，否则返回 `invalid_input`。

### ServerInput

| 字段 | 类型 | 说明 |
|---|---|---|
| `name` | string | 必填，不超过 64 个字符 |
| `group` | string | 分组 |
| `country` | string | 空或两位字母，保存为大写 |
| `hidden` | bool | 仅登录可见 |
| `price` | number \| null | 非负 |
| `currency` | string | 币种 |
| `billing_cycle` | string | `""`、`monthly`、`quarterly`、`yearly`、`once` |
| `expire_at` | number \| null | 到期时间 |
| `traffic_limit` | number \| null | 流量配额（字节），`null` 为不限 |
| `traffic_mode` | string | `sum`（默认）、`in`、`out` |
| `traffic_reset_day` | number | 1–28，传 0 视为 1 |
| `report_interval` | number | 1–60 秒，传 0 使用设置中的默认上报间隔 |
| `nic_include`、`nic_exclude`、`mount_exclude` | string[] | 网卡 / 挂载点过滤，含义见 [配置](configuration.md#服务器字段) |
| `note` | string | 备注，仅后台可见，不超过 2000 个字符 |
| `notify_muted` | bool | 开启后该服务器跳过全部通知规则 |

`Server` 在 `ServerInput` 的基础上增加 `id`、`secret`、`sort`、`static_info`、`last_ip`、`last_seen`、`created_at`、`updated_at`。其中 `last_seen` 与公开接口含义相同：有实时上报时为最近一次上报时间，否则为断开或最后记录的在线时间；为 0 表示从未上报。

### Settings

```json
{
  "site_title": "Simple Server Status",
  "show_price": false,
  "default_report_interval": 2,
  "install_script_base": "https://github.com/ruanun/simple-server-status/releases/latest/download",
  "announcement": "",
  "notify": {
    "webhook_url": "", "telegram_token": "", "telegram_chat_id": "", "lang": "zh-CN",
    "offline_enabled": true, "offline_minutes": 3,
    "load_enabled": true, "reboot_enabled": true, "ip_change_enabled": true,
    "expire_enabled": true, "expire_days": 7,
    "traffic_enabled": true, "traffic_percent": 90
  },
  "events": { "load_cpu": 90, "load_mem": 90, "load_disk": 90, "load_minutes": 5 },
  "captcha": { "mode": "none", "turnstile_site_key": "", "turnstile_secret": "" }
}
```

`announcement` 显示在状态页顶部，不超过 1000 个字符，留空不显示；换行保留，`http(s)://` 链接自动转为可点击链接。`telegram_token` 属于敏感信息，公开接口不返回、日志不输出，仅登录后台可读写。

`notify`（`NotifySettings`）字段：

| 字段 | 类型 | 默认值 | 取值范围 | 说明 |
|---|---|---|---|---|
| `webhook_url` | string | `""` | 空或 `http(s)://` 地址 | 为空表示不启用 Webhook |
| `telegram_token`、`telegram_chat_id` | string | `""` | — | 需同时为空或同时非空；均非空时启用 Telegram |
| `lang` | string | `zh-CN` | `zh-CN`、`en-US` | 通知标题与正文使用的语言 |
| `offline_enabled`、`offline_minutes` | bool、number | `true`、`3` | 分钟 1–1440 | 离线持续超过该时长即通知，恢复在线时通知并附带离线时长 |
| `load_enabled` | bool | `true` | — | 高负载事件开始时通知，结束时通知恢复；阈值见下方 `events` |
| `reboot_enabled` | bool | `true` | — | 服务器重启时通知 |
| `ip_change_enabled` | bool | `true` | — | 公网 IPv4 / IPv6 变化时通知 |
| `expire_enabled`、`expire_days` | bool、number | `true`、`7` | 天 1–90 | 距到期不超过该天数时通知一次；每个到期日只提醒一次 |
| `traffic_enabled`、`traffic_percent` | bool、number | `true`、`90` | 百分比 1–100 | 本计费周期流量用量达到该比例时通知一次；每个计费周期只提醒一次 |

`events`（`EventSettings`，检测规则）决定是否记录高负载事件，与是否配置通知渠道无关：

| 字段 | 类型 | 默认值 | 取值范围 | 说明 |
|---|---|---|---|---|
| `load_cpu`、`load_mem`、`load_disk` | number | `90` | 1–100 | CPU / 内存 / 硬盘最近 `load_minutes` 分钟平均值达到阈值时记录对应的高负载事件，回落到阈值以下时结束 |
| `load_minutes` | number | `5` | 1–10 | 计算平均值的时长（分钟） |

`captcha`（`CaptchaSettings`）：`mode` 为 `none`（默认）、`image` 或 `turnstile`；`turnstile_site_key`、`turnstile_secret` 为 Cloudflare Turnstile 的 Site Key 与 Secret Key，`mode` 为 `turnstile` 时必填。`turnstile_secret` 与 `telegram_token` 一样属于敏感信息，公开接口不返回。新启用 Turnstile 或修改其密钥时，请求体需额外带上 `turnstile_token`：前端用新的 Site Key 完成一次验证得到的 token。Dashboard 用新的 Secret Key 核验通过后才会保存，以确认密钥与域名配置正确；核验失败返回 `turnstile_check_failed`。

保存 `Settings` 时的校验：`webhook_url` 为空或 `http(s)` 地址；`telegram_token` 与 `telegram_chat_id` 需同时为空或同时非空；上表阈值需在取值范围内；`announcement` ≤ 1000 字符。违规返回 `invalid_input`，`message` 说明具体字段。

### GET /api/admin/overview

```json
{
  "monthly_cost": [{"currency": "$", "amount": 12.5, "servers": 3}],
  "expiring": [{"id": "...", "name": "...", "expire_at": 1790000000, "days": 5}]
}
```

`monthly_cost` 按币种汇总月均费用：`monthly` 原价、`quarterly` 除以 3、`yearly` 除以 12，结果保留 2 位小数；`once`、未设置付费周期或未设价格的服务器不计入；按币种原样分组（不做汇率换算），币种为空归为一组。`expiring` 为 30 天内到期（含已过期但未超过 30 天）的服务器，按剩余天数升序、同天按名称排序；`days` 为负数表示已过期。

### POST /api/admin/notify/test

请求体为 `NotifySettings`（可以是尚未保存的值）。同步向其中已启用的渠道各发送一条测试消息，不重试、不经过发送队列，响应示例：

```json
{"webhook": "ok", "telegram": null}
```

未启用的渠道为 `null`；已启用且发送成功为 `"ok"`，失败为错误信息字符串。未配置任何渠道时返回 `invalid_input`。此接口不经过发送队列，但仍会给每个已启用渠道各写入一条 `kind: "test"` 的通知记录，可在 `/api/admin/notify-log` 中查到。

### GET /api/admin/events、GET /api/admin/notify-log

后台「事件」页使用的事件与通知记录列表，均支持分页：`page` 默认 1，`size` 默认 50、超过 100 按 100 处理；`page`、`size` 不是正整数时返回 `invalid_input`。`server_id` 留空表示不筛选服务器；`kind` 留空表示不筛选类型，含未知类型时返回 `invalid_input`；`status` 留空表示不筛选状态，非 `pending`/`sent`/`failed` 时返回 `invalid_input`。两类记录均保留 90 天，到期自动清理（进行中的事件保留）。

`AdminEvent`：

```json
{"id": 1, "server_id": "...", "server_name": "...", "kind": "load_cpu", "start_at": 1790000000, "end_at": null, "duration": 120,
 "detail": {"threshold": 90, "minutes": 5, "peak": 97.2}}
```

| kind | 类型 | `detail` |
|---|---|---|
| `offline` | 时段 | `{}` |
| `load_cpu`、`load_mem`、`load_disk` | 时段 | `threshold` 阈值、`minutes` 统计分钟数、`peak` 事件期间窗口平均值的最大值 |
| `reboot` | 瞬时 | `boot_at` 开机时间 |
| `ip_change` | 瞬时 | `ipv4`、`ipv6`：`[旧地址, 新地址]`，只包含变化的一项 |

时段事件进行中时 `end_at` 为 `null`，`duration` 按当前时间计算；瞬时事件 `end_at` 等于 `start_at`，`duration` 为 0。

`NotifyLog`：

```json
{
  "id": 1, "server_id": "...", "server_name": "...", "event_id": 3, "kind": "offline", "channel": "webhook",
  "title": "[离线] node-1", "message": "服务器 node-1 已离线 5 分钟",
  "status": "sent", "error": "", "created_at": 1790000000, "done_at": 1790000003
}
```

`kind` 取值同 Webhook 负载的 `event`（见下文「通知」一节），「发送测试」产生的记录 `kind` 为 `test`、`server_id`/`server_name` 为空。`event_id` 为触发该通知的事件，到期、流量提醒与测试通知为 `null`。`status` 为 `pending`（发送中）、`sent`（成功）、`failed`（失败，`error` 说明原因）。`error` 不包含 Telegram Token 或 Webhook 地址。

### 导出文件

```json
{"version": 1, "exported_at": 1790000000, "settings": {...}, "servers": [Server, ...]}
```

包含全部服务器的 `secret`、`note`、`notify_muted`，以及 `settings` 中的 `announcement`、全部 `notify` 字段（含 Telegram Token）、`events` 字段与 `captcha` 字段（含 Turnstile Secret Key）；不包含静态信息、最后在线 IP 与时间、历史数据和流量。导入时按 `id` 覆盖已有服务器、新增不存在的服务器，并用文件中的设置覆盖当前设置。例外是文件中的 Turnstile 配置与当前不同时：它没有在当前域名下核验过，直接启用可能导致无法登录，因此保留当前的验证码设置，并在响应中返回 `"captcha_kept": true`；密钥发生变化的服务器会断开旧连接。导出文件版本固定为 1；导入旧版本文件中缺少的字段取默认值。

## 浏览器 WebSocket

地址：`/api/public/ws`。浏览器无法为 WebSocket 设置请求头，登录用户通过查询参数传 token：`/api/public/ws?token=<jwt>`（仅此接口接受查询参数中的 token）。token 缺失或无效时按匿名处理。

服务端只推送文本消息，客户端无需发送任何内容：

```json
{"type": "snapshot", "data": [ServerView, ...]}
{"type": "delta", "data": [ServerView, ...]}
```

- 连接建立后立即推送一次 `snapshot`，内容与 `GET /api/public/servers` 相同。
- 之后每 2 秒检查一次，把有变化的服务器（有新上报、上线或离线）以 `delta` 推送，客户端按 `id` 合并。
- 服务器被新建、删除、编辑、排序，或设置被修改后，下一次推送改为完整的 `snapshot`，客户端应整体替换列表。
- 可见性规则与 HTTP 接口一致。已登录连接每 10 秒校验一次 token，token 过期或因改密码失效时服务端主动断开，客户端重连即可。
- 服务端每 30 秒发送一次 ping；客户端接收过慢导致积压时会被断开。

## Agent WebSocket

地址：`/api/agent/ws`（Agent 把配置中的 `http`/`https` 地址转换为 `ws`/`wss`）。

握手请求头：

```
Authorization: Bearer <server_id>:<secret>
```

ID 不存在或密钥错误时返回 HTTP 401，Agent 随后每 5 分钟重试一次；其他断线按 1 秒起指数退避重连，最长 60 秒。同一服务器建立新连接时，旧连接会被关闭。

所有消息使用统一信封：

```json
{"v": 1, "type": "<类型>", "data": {...}}
```

`v` 不等于 1 时 Dashboard 以状态码 1008 关闭连接；未知的 `type` 被忽略。单条消息不超过 64 KB。Dashboard 每 20 秒发送一次 ping，20 秒内未收到 pong 即断开。

交互流程：Agent 连接后发送 `hello` → Dashboard 回复 `config` → Agent 按 `report_interval` 周期发送 `report`。后台修改服务器参数或导入数据后，Dashboard 立即推送新的 `config`。

| 方向 | type | data |
|---|---|---|
| Agent → Dashboard | `hello` | 静态信息：`os`、`platform`、`platform_version`、`kernel`、`arch`、`virtualization`、`cpu_model`、`cpu_cores`、`mem_total`、`swap_total`、`disk_total`、`agent_version`、`country`、`ipv4`、`ipv6` |
| Dashboard → Agent | `config` | 采集参数：`report_interval`（秒）、`nic_include`、`nic_exclude`、`mount_exclude`、`filter_id` |
| Agent → Dashboard | `report` | 动态指标，见下表 |
| Dashboard → Agent | `stop` | `{"reason": "deleted"}`：服务器已被删除，Agent 收到后退出进程且不再重连 |

`report` 字段：

| 字段 | 说明 |
|---|---|
| `ts` | 采集时间 |
| `cpu` | CPU 使用率（百分比） |
| `load1`、`load5`、`load15` | 系统负载 |
| `mem_used`、`swap_used` | 已用内存、交换 |
| `disk_used`、`disk_total` | 参与统计的分区合计 |
| `disks` | 分区数组：`mount`、`fstype`、`total`、`used` |
| `net_in_speed`、`net_out_speed` | 网速（字节每秒） |
| `net_in_total`、`net_out_total` | 网卡累计收发字节，Dashboard 据此计算月流量 |
| `procs`、`tcp`、`udp` | 进程数、TCP / UDP 连接数 |
| `uptime` | 开机时长（秒） |
| `filter_id` | 采集时使用的网卡过滤摘要，原样取自最近一次 `config`，用于识别流量统计口径的变化 |

示例：

```json
{"v":1,"type":"config","data":{"report_interval":2,"nic_include":[],"nic_exclude":[],"mount_exclude":["/boot"],"filter_id":"3f1a9c0b7e2d"}}
```

## 通知（Webhook / Telegram）

规则、渠道配置与默认值见 [配置 - 后台设置](configuration.md#通知)。事件发生或提醒触发时由后台的发送队列推送，单次请求超时 10 秒，失败后按 5 秒、30 秒、2 分钟重试共 3 次，最终失败只记日志；`POST /api/admin/notify/test` 不经过该队列，同步发送且不重试。

Webhook：`POST`，`Content-Type: application/json`：

```json
{"event": "offline", "server_id": "...", "server_name": "...", "title": "[离线] node-1", "message": "服务器 node-1 已离线 5 分钟", "time": 1790000000}
```

`event` 取值：

| event | 触发时机 |
|---|---|
| `offline` | 离线持续超过设定时长 |
| `recovered` | 离线后恢复在线 |
| `load` | CPU / 内存 / 硬盘持续高负载（每项指标单独通知） |
| `load_recovered` | 该项指标的高负载恢复正常 |
| `reboot` | 服务器重启 |
| `ip_change` | 公网 IPv4 / IPv6 变化 |
| `expire` | 距到期不超过设定天数 |
| `traffic` | 本周期流量用量达到设定比例 |
| `test` | 「发送测试」触发 |

Telegram：`POST https://api.telegram.org/bot<token>/sendMessage`，请求体 `{"chat_id": "...", "text": "<title>\n<message>"}`，不使用 `parse_mode`。

Telegram Token 与 Webhook 地址均不写入日志，发送失败时日志只记录渠道与错误类型。
