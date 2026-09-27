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
| `unauthorized` | 401 | 未登录、token 无效或已失效；Agent 鉴权失败 |
| `invalid_credentials` | 401 | 用户名或密码错误 |
| `not_found` | 404 | 服务器不存在，或接口不存在 |
| `too_many_attempts` | 429 | 登录失败次数过多 |
| `internal` | 500 | 服务器内部错误 |
| `shutting_down` | 503 | 服务正在停止，拒绝新的 WebSocket 连接 |

## 鉴权

- `POST /api/auth/login` 返回 JWT（HS256，有效期 7 天），之后在请求头携带 `Authorization: Bearer <token>`。
- 修改密码或执行 `reset-password` 后，之前签发的 token 全部失效。
- 登录限流：同一客户端 IP 5 分钟内失败 5 次后锁定 5 分钟。位于反向代理之后时需配置 `--trusted-proxies`，否则所有请求共用代理的 IP。
- 公开接口不要求登录；携带有效 token 时可以看到隐藏服务器与价格。

## 公开接口

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/public/site` | 站点信息：`{"site_title", "show_price"}` |
| GET | `/api/public/servers` | 所有可见服务器的 `ServerView` 数组 |
| GET | `/api/public/servers/:id` | 单台服务器的 `ServerView`；不存在或未登录访问隐藏服务器时返回 404 |
| GET | `/api/public/servers/:id/metrics?range=<range>` | 历史数据点数组，`range` 默认 `1h` |
| GET | `/api/public/ws` | 浏览器实时推送，见下文 |

可见性规则：隐藏服务器只对已登录用户返回；价格字段（`price`、`currency`、`billing_cycle`）在设置「公开显示价格」关闭时只对已登录用户返回；任何公开接口都不返回 IP 与密钥。

### ServerView

| 字段 | 说明 |
|---|---|
| `id`、`name`、`group`、`country`、`sort`、`hidden` | 服务器配置；`country` 为空时取 Agent 探测结果 |
| `online` | 是否在线：连接存在且最近一次上报距今不超过 3 倍上报间隔（最少 6 秒） |
| `last_seen` | 最近一次上报（或断开）时间 |
| `static` | 静态信息，结构同 Agent 的 `hello`；从未连接过时为 `null` |
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

## 登录与管理接口

除 `POST /api/auth/login` 外均需要 `Authorization: Bearer <token>`。

| 方法 | 路径 | 请求 | 响应 `data` |
|---|---|---|---|
| POST | `/api/auth/login` | `{"username", "password"}` | `{"token", "username"}` |
| GET | `/api/auth/me` | — | `{"username"}` |
| PUT | `/api/auth/password` | `{"old_password", "new_password"}` | `{"token"}`（新 token，旧 token 失效） |
| GET | `/api/admin/servers` | — | `Server` 数组，每项额外带 `online` |
| POST | `/api/admin/servers` | `ServerInput` | 新建的 `Server`（含自动生成的 `id` 与 `secret`） |
| PUT | `/api/admin/servers/:id` | `ServerInput` | 更新后的 `Server`；在线的 Agent 会立即收到新的 `config` |
| DELETE | `/api/admin/servers/:id` | — | `{}`；同时删除历史与流量数据，在线的 Agent 收到 `stop` 后退出 |
| POST | `/api/admin/servers/:id/reset-secret` | — | `{"secret"}`；旧密钥立即失效，当前连接被断开 |
| GET | `/api/admin/servers/:id/install?dashboard=<面板地址>` | — | `{"linux", "windows"}` 两条安装命令 |
| PUT | `/api/admin/server-order` | `{"ids": [...]}` | `{}`；按数组顺序设置 `sort` |
| GET | `/api/admin/settings` | — | `Settings` |
| PUT | `/api/admin/settings` | `Settings` | 保存后的 `Settings` |
| GET | `/api/admin/export` | — | 导出文件（见下文） |
| POST | `/api/admin/import` | 导出文件 | `{"servers": <导入数量>}` |

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

`Server` 在 `ServerInput` 的基础上增加 `id`、`secret`、`sort`、`static_info`、`last_ip`、`last_seen`、`created_at`、`updated_at`。

### Settings

```json
{
  "site_title": "Simple Server Status",
  "show_price": false,
  "default_report_interval": 2,
  "install_script_base": "https://github.com/ruanun/simple-server-status/releases/latest/download"
}
```

### 导出文件

```json
{"version": 1, "exported_at": 1790000000, "settings": {...}, "servers": [Server, ...]}
```

包含全部服务器的 `secret`，不包含静态信息、最后在线 IP 与时间、历史数据和流量。导入时按 `id` 覆盖已有服务器、新增不存在的服务器，并用文件中的设置覆盖当前设置；密钥发生变化的服务器会断开旧连接。

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
| Agent → Dashboard | `hello` | 静态信息：`os`、`platform`、`platform_version`、`kernel`、`arch`、`virtualization`、`cpu_model`、`cpu_cores`、`mem_total`、`swap_total`、`disk_total`、`agent_version`、`country` |
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
