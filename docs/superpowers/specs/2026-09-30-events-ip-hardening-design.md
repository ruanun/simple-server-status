# 离线记录、通知记录、IPv4/IPv6 与稳定性完善设计

- 日期：2026-09-30
- 作者：ruan
- 分支：`develop`
- 上位设计：`docs/superpowers/specs/2026-09-25-rewrite-design.md`、`docs/superpowers/specs/2026-09-29-status-admin-enhancements-design.md`（本文未提及之处以其为准）

## 1. 目标与范围

用户确认的 8 项：

1. CI 增加竞态检测（`go test -race`）。
2. e2e 补充：设置页通知配置与发送测试、详情页在线率与每日流量、离线记录。
3. 「发送测试」两个渠道并行发送。
4. 在线率计算出错时保留上一次的值。
5. 退出时通知收尾总时长不超过 5 秒。
6. 通知记录，并借此让到期 / 流量提醒「投递成功后才算已提醒」。
7. 离线记录：后台「事件」页 + 公开详情页显示最近离线。
8. IPv4 / IPv6：Agent 探测公网地址；公开页只显示支持标记，后台显示完整地址。

完成后发布 `v2.0.0-beta.4`（包含已在本地提交的「公开页不显示 Agent 版本」）。

**不做**：离线记录的手动编辑 / 删除 / 导出；通知记录的一键重发；IP 变化历史；公开页显示 IP 地址。

## 2. 数据与协议

### 2.1 数据库（迁移 003）

```sql
CREATE TABLE outages (
  id        INTEGER PRIMARY KEY AUTOINCREMENT,
  server_id TEXT NOT NULL,
  start_at  INTEGER NOT NULL,   -- 最后一次上报时间（Unix 秒）
  end_at    INTEGER             -- 恢复后第一次上报时间；仍离线为 NULL
);
CREATE INDEX idx_outages_server ON outages (server_id, start_at);
CREATE INDEX idx_outages_start ON outages (start_at);

CREATE TABLE notify_log (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  server_id   TEXT NOT NULL DEFAULT '',   -- 测试消息为空
  server_name TEXT NOT NULL DEFAULT '',
  kind        TEXT NOT NULL,              -- offline / recovered / load / load_recovered / expire / traffic / test
  channel     TEXT NOT NULL,              -- webhook / telegram
  title       TEXT NOT NULL,
  message     TEXT NOT NULL,
  status      TEXT NOT NULL,              -- pending / sent / failed
  error       TEXT NOT NULL DEFAULT '',
  created_at  INTEGER NOT NULL,
  done_at     INTEGER                     -- 成功或最终失败的时间
);
CREATE INDEX idx_notify_log_created ON notify_log (created_at);
CREATE INDEX idx_notify_log_server ON notify_log (server_id, created_at);
```

两张表都保留 90 天，由现有维护任务每小时清理；删除服务器时删除其 `outages` 与 `notify_log` 记录。`error` 中不得包含 Telegram Token 或 Webhook 地址（沿用渠道层已有的错误清洗）。

### 2.2 协议

`proto.Hello` 增加 `IPv4 string \`json:"ipv4,omitempty"\``、`IPv6 string \`json:"ipv6,omitempty"\``：Agent 探测到的公网地址，探测失败为空。旧版 Agent 不发送，Dashboard 视为未知。

Agent 在连接中途可再次发送 `hello`（静态信息变化时）；Dashboard 的读循环已在任意时刻处理 `hello`，行为不变（更新静态信息与来源 IP 并同步缓存，再下发一次配置）。

### 2.3 接口

- `ServerView`（公开列表、详情、WS）：
  - 增加 `ipv4: boolean`、`ipv6: boolean`，表示是否探测到对应地址。
  - `static` 中的 `ipv4`、`ipv6` 地址与 `agent_version` 一样被清空。
- 新增 `GET /api/public/servers/{id}/outages`：
  - 返回该机最近 10 条离线记录 `[{start_at, end_at|null, duration}]`，按开始时间倒序；`duration` 为秒，进行中的按当前时间计算。
  - 隐藏服务器对未登录用户 404。
- 后台服务器列表：`static_info` 中包含 `ipv4`、`ipv6` 地址；`last_ip`（连接来源）保留。
- 新增 `GET /api/admin/outages?server_id=&page=&size=`：
  - 分页，`size` 默认 50、最大 100，按 `start_at` 倒序。
  - 返回 `{items: [{id, server_id, server_name, start_at, end_at, duration}], total}`；`server_name` 取当前名称，服务器已删除的记录随删除一并清理。
- 新增 `GET /api/admin/notify-log?server_id=&status=&page=&size=`：
  - 分页规则同上，按 `created_at` 倒序。
  - 返回 `{items: [{id, server_id, server_name, kind, channel, title, message, status, error, created_at, done_at}], total}`。
- 导入导出不包含两类记录。

## 3. 后台行为

### 3.1 离线记录（`internal/dashboard/outage`）

- 每 30 秒检查一次所有服务器：
  - 服务器离线、距最后一次上报 ≥ 60 秒、且没有未结束记录时，新建记录，`start_at` 为最后一次上报时间（实时上报时间，缺失时取数据库 `last_seen`）。
  - 服务器在线且有未结束记录时，`end_at` 设为恢复后的第一次上报时间。实现上取在线期间 hub 的最近上报时间，第一次检测到恢复时写入。
- 未结束记录保存在数据库中。Dashboard 重启后第一轮检查读取所有未结束记录，按上面的规则关闭或保持。因此 Dashboard 停机时段计入离线时长，界面与文档须说明。
- 从未上报过的服务器不记录。
- 门槛（60 秒）与检查间隔（30 秒）作为 `outage.Tracker` 的参数。Dashboard 增加隐藏的启动参数 `--outage-test-threshold`，仅供 e2e 缩短：接受 Go duration，同时把检查间隔设为门槛的一半；帮助信息中隐藏（`MarkHidden`），文档不提。

### 3.2 通知记录与投递可靠性（`internal/dashboard/notify`）

- 入队时每个 (事件, 渠道) 写一条 `pending` 记录，得到记录 ID 随任务携带。发送成功更新为 `sent`；重试全部失败（或退出时未能发出）更新为 `failed` 与错误原因。
- 到期与流量提醒（有 `Mark` 的事件）：
  - 不再在入队前 `MarkNotified`，改为任一渠道发送成功后才写入 `notify_state`。
  - 投递进行中时，Notifier 在内存中记录 `(server_id, rule, key)` 为「处理中」，`sent()` 对处理中的键返回 true，避免下一轮重复入队。
  - 该事件所有渠道均最终失败后移除「处理中」，下一轮会重新提醒。
- 离线 / 恢复 / 高负载类事件只记录结果，不重发。
- 「发送测试」同样写记录（`server_id` 为空、`kind=test`），同步发送，结果也更新到记录。
- 通知记录写库失败只记日志，不影响发送。

### 3.3 IPv4 / IPv6 探测（Agent）

- 新增 `collect.DetectNetwork(ctx, trace URL) (Network{Country, IPv4, IPv6}, error)`：
  - 分别用只走 `tcp4` 与只走 `tcp6` 的 HTTP 客户端请求 Cloudflare trace（现有 `TraceURL`），各自超时 5 秒。
  - 从返回中解析 `ip=` 与 `loc=`；地址须是对应协议的合法 IP，否则视为失败。
  - `Country` 取任一成功响应的 `loc`（优先 IPv4）。两者都失败时返回错误，由调用方记日志。
- 探测时机：
  - Agent 启动时探测一次，替代现在只探测国家的调用；
  - 之后每 6 小时探测一次。结果与上次不同就更新 Client 的静态信息，并在当前连接上重新发送 `hello`；未连接时在下次连接的 `hello` 中带上。
- 配置 `detect_country` 为 false 时不探测国家，也不探测 IP（不访问外网）。

### 3.4 顺带的稳定性项

- `SendTest` 并行发送各渠道，总时长约等于单个渠道超时（10 秒）。
- 在线率缓存：单台计算出错时保留该台上一次的值。
- 发送队列退出：
  - `Run` 中进行中的 `deliver` 使用从 `Run` 的 ctx 派生的上下文，ctx 结束时立即中断该请求，并把任务放回队列交给收尾；
  - 收尾阶段仍受 5 秒截止时间约束，未发出的记为 `failed`（原因「退出时未能发送」）。

## 4. 界面

### 4.1 公开页

- **卡片与列表**：名称后（国旗之后）显示小标记「v4」「v6」，只显示支持的项；均为 false 时不显示。
- **详情页**：
  - 头部显示同样的标记。
  - 「每日流量」下方新增「最近离线」区块：来自 `/outages`，列出最多 10 条，格式「9/28 03:12 – 03:20 · 8 分钟」；进行中显示「进行中」；无记录显示「最近 90 天无离线记录」。
  - 该区块附注「时间以面板记录为准，包含面板自身停机时段」。
  - 请求失败时只在该区块显示错误。

### 4.2 后台

- **服务器列表的 IP 列**：
  - 两行显示 IPv4 / IPv6，缺失显示「—」；悬停显示「连接来源：{last_ip}」。
  - 两者都未上报（旧版 Agent）时显示 `last_ip`。
- **侧栏新增「事件」**（`/admin/events`），两个标签「离线记录」「通知记录」，当前标签写入 `?tab=`：
  - **离线记录**：服务器筛选（下拉，含「全部」）；列为服务器、开始、结束（进行中）、时长；分页每页 50。
  - **通知记录**：服务器与状态（全部 / 成功 / 失败 / 发送中）筛选；列为时间、服务器（测试消息显示「测试」）、类型、渠道、状态、失败原因；点击行展开标题与正文；分页每页 50。
  - 窄屏隐藏次要列（渠道、结束时间等）。
  - 筛选条件也写入网址（`?server=`、`?status=`）。
- 服务器行菜单新增「查看事件」→ `/admin/events?server={id}`。
- 设置页「发送测试」结果旁显示链接「查看通知记录」→ `/admin/events?tab=notify`。

## 5. 错误处理

- 离线记录、通知记录写库失败只记日志，不影响检查与发送。
- 分页参数非法（非数字、小于 1）返回 `invalid_input`；`size` 超过 100 按 100 处理。
- IP 探测失败不影响 Agent 上报，只记一次日志（与国家探测一致）。

## 6. 测试与 CI

- CI：Go 检查任务增加 `go test -race ./...`。实现前先在 Go 官方容器中运行一次；若发现既有竞态，记录并修复（报告中说明涉及的已有逻辑）。
- 单元测试：
  - **离线记录**：满 60 秒才记录；恢复补结束时间；重启后关闭未结束记录；从未上报不记录；删除清理；90 天清理；公开 / 后台接口分页、筛选、隐藏服务器 404。
  - **通知记录**：pending → sent / failed；到期提醒失败后下一轮重发；处理中不重复；测试消息写记录；记录中无 Token / URL。
  - **IP 探测**：用本地测试服务器分别监听 IPv4 与 IPv6 回环地址，模拟两者都通、只有 v4、都不通；结果变化时重发 hello；`detect_country=false` 不探测。
  - **公开视图**：只有布尔标记、不含地址；旧版 Agent 缺字段兼容。
  - **稳定性项**：在线率出错保留旧值；`SendTest` 并行；退出总时长 ≤ 5 秒且未发出的记为 failed。
- 前端：
  - IP 标记显示规则；
  - 详情页「最近离线」的各状态；
  - 事件页两个标签的筛选、分页、网址同步；
  - 后台 IP 列两行与回退；
  - 「查看事件」「查看通知记录」跳转。
- e2e 新增：
  1. 设置页「通知」标签填 Webhook（指向 e2e 进程内的接收器），保存后刷新仍在；「发送测试」后接收器收到消息，事件页通知记录出现一条成功记录。
  2. 详情页显示在线率文字与每日流量图。
  3. 使用 `--outage-test-threshold` 启动的 Dashboard，停止 e2e Agent 后，事件页离线记录出现进行中的记录。

## 7. 文档

更新 `docs/api.md`（新接口与字段、`hello` 新字段）、`docs/configuration.md`（离线记录说明、通知记录、`detect_country` 同时控制 IP 探测）、README 特性列表、CHANGELOG。
