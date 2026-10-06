# 事件系统重构设计

- 日期：2026-10-03
- 作者：ruan
- 分支：`develop`
- 上位设计：`docs/superpowers/specs/2026-09-30-events-ip-hardening-design.md`（本文未提及之处以其为准）

## 1. 背景与目标

现状：检测与通知耦合。

- 离线由 `outage.Tracker` 记录（与渠道无关），写入 `outages` 表；`notify.Notifier` 又维护了一套离线状态机。
- 高负载只在 `Notifier` 中判断，而 `Notifier.Check` 在未配置渠道时直接返回，因此**不配置 Webhook / Telegram 时，高负载不留下任何记录**；配置了渠道时，也只以「每个渠道一条通知记录」的形式存在。

目标：拆成「检测 → 事件 → 通知」三层，每种状况只由一处判定。

### 1.1 概念划分

| 概念 | 含义 | 本文范围 |
|---|---|---|
| **事件** | 根据 Agent 数据观察到的运行状况：离线、高负载、重启、IP 变化 | 由检测器 / 上报处理判定，写入 `events`，无论是否配置渠道都记录 |
| **提醒** | 根据配置安排的提醒：即将到期、流量达到额度比例 | 保持现状，由 `Notifier` 判定并以 `notify_state` 去重 |
| **通知** | 把事件或提醒推送到渠道 | `Notifier` 订阅事件、产生提醒，统一经发送队列推送，结果写入 `notify_log` |

记录与推送的门槛相互独立：事件按检测规则记录（如离线 1 分钟），推送在事件之上再加通知维度的条件（如离线满 `OfflineMinutes` 才推送）。

### 1.2 范围

- 事件类型：`offline`、`load_cpu`、`load_mem`、`load_disk`、`reboot`、`ip_change`。
- 事件只在后台展示；公开详情页「最近离线」保持现状（数据改为读取 `kind='offline'`）。
- 架构上新增事件类型只需要增加判定和文案，暂不做的类型（挂载点空间、Agent 版本变化、系统信息变化等）以后按需加入。

**不做**：审计；事件的手动编辑 / 删除 / 导出；到期与流量改为事件。

## 2. 事件类型

| kind | 类型 | 开始 | 结束 | detail |
|---|---|---|---|---|
| `offline` | 时段 | 离线满 1 分钟（沿用现有门槛与 2 分钟启动宽限期），开始时间取最后一次上报 | 恢复后第一次上报 | `{}` |
| `load_cpu` / `load_mem` / `load_disk` | 时段 | 最近 `LoadMinutes` 分钟均值 ≥ 阈值 | 均值 < 阈值 | `{"threshold":90,"peak":97.2}`，峰值为事件期间窗口均值的最大值 |
| `reboot` | 瞬时 | hello 中的 `boot_id` 与上次不同 | 同开始 | `{}` |
| `ip_change` | 瞬时 | hello 中的 `IPv4` 或 `IPv6` 与上次不同 | 同开始 | `{"ipv4":["旧","新"]}`，只包含变化的一项 |

判定细节：

- 负载按指标拆分，各自开始、各自恢复；离线期间负载事件保持原状态（数据中断不等于恢复）。窗口内少于 2 个点时不判定，Dashboard 重启后会等数据攒够再判断。
- `reboot`：Agent 在 hello 中上报本次开机的唯一标识 `boot_id`（Linux `/proc/sys/kernel/random/boot_id`；Windows 注册表 `PrefetchParameters\BootId`，每次开机加一；macOS `kern.bootsessionuuid`；FreeBSD 为空），与上次保存的静态信息比较，变化即重启，任一方为空不判断。事件时间为发现时间。不依赖任何一方的时钟。
  - 最初版本（beta.7）用「Dashboard 时间 − uptime」推算开机时间并与记录值比较，Dashboard 校时后所有服务器同时被误判为重启，且事件时间取开机时间，开机超过 90 天的事件一写入即被清理；beta.8 改为 `boot_id`，迁移 005 删除 `servers.boot_at`。
- `ip_change`：旧值或新值为空时不算变化（公网探测失败）。hello 只在每次连接时发送一次，变化频率受重连约束，不需要去抖。
- 静音不影响记录，只影响推送。

## 3. 数据（迁移 004）

新建 `events` 表取代 `outages`。线上仅有极少量离线记录且未配置通知，旧数据不迁移，直接删除旧表：

```sql
CREATE TABLE events (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  server_id    TEXT    NOT NULL,
  kind         TEXT    NOT NULL,
  start_at     INTEGER NOT NULL,
  end_at       INTEGER,                    -- 时段事件进行中为 NULL；瞬时事件等于 start_at
  detail       TEXT    NOT NULL DEFAULT '{}',
  notify_state INTEGER NOT NULL DEFAULT 0  -- 0 未决定，1 已推送，2 不推送（未开启 / 静音 / 无渠道 / 发生在渠道启用前），3 推送失败，4 发送中
);
CREATE INDEX idx_events_server ON events (server_id, start_at);
CREATE INDEX idx_events_kind   ON events (kind, start_at);
CREATE INDEX idx_events_start  ON events (start_at);
CREATE INDEX idx_events_open   ON events (server_id, kind) WHERE end_at IS NULL;  -- 启动时加载进行中的事件

DROP TABLE outages;
DELETE FROM settings WHERE key IN ('notify_load_cpu', 'notify_load_mem', 'notify_load_disk', 'notify_load_minutes');

ALTER TABLE notify_log ADD COLUMN event_id INTEGER;  -- 提醒与测试通知为 NULL
```

- `store/outages.go` 删除，由 `store/events.go` 取代；`Outage` 类型与相关方法随之移除。
- `notify_state` 持久化后，Dashboard 重启前已推送离线通知的服务器，恢复时仍会推送恢复通知（现状是重启后丢失）。
- 保留期 90 天与孤儿清理沿用 `cleanupEvents`。

## 4. 架构

```
handleHello ──ip_change──┐
handleReport ──reboot────┤
incident.Detector ───────┼─→ incident.Recorder ──写库──→ events
  （offline、load_*）     │        │
                          │        └─发布变更─→ Notifier ──→ Sender ──→ notify_log
                          │                       ↑
                          │              提醒（到期、流量）
```

### 4.1 `incident.Recorder`（与 Detector 同在新包 `internal/dashboard/incident`）

- 方法：`Open(serverID, kind, start, detail)`、`UpdateDetail(id, detail)`、`Close(id, end)`、`Instant(serverID, kind, at, detail)`、`SetNotifyState(id, state)`、`Ongoing(serverID, kind)`。
- 启动时从库加载进行中的事件（取代 `OpenOutages`），检测器与 Notifier 都从这里读取进行中的事件，不各自维护。
- 写库成功后向订阅者发布 `Change{Type: Opened|Closed|Instant, Event}`；发布为有缓冲的 channel，满时丢弃并警告（事件记录不受影响）。
- 删除服务器时 `Forget(id)` 清理内存状态。

### 4.2 `incident.Detector`（由 `outage.Tracker` 改造，`outage` 包删除）

- 每 30 秒一轮：离线、负载。负载判定函数 `loadOver` 从 `notify` 移到这里，**这是唯一的负载判定**。
- 离线门槛、检查间隔、启动宽限期的现有规则与测试全部保留。

### 4.3 上报处理

- `handleHello`：保存前用缓存中的旧 `StaticInfo` 比较 IP，变化时记录 `ip_change`。
- `handleHello`：保存前比较新旧 `boot_id`，变化时记录 `reboot`。

### 4.4 `Notifier`

删除 `State`、`Evaluate` 中的离线 / 负载状态机与基线期；保留提醒（到期、流量）的判定、`notify_state` 表去重、提醒的 `inflight`、发送队列与 `notify_log`。

事件推送规则（推送决定只做一次，结果写入事件的 `notify_state`）：

| 事件 | 决定时机 | 推送条件 |
|---|---|---|
| `offline` | 每 30 秒扫描进行中的离线事件，持续时间达到 `OfflineMinutes` 时 | 离线通知开启、未静音、有渠道，且事件由本进程在渠道启用之后开始跟踪（`Tracked.OpenedAt`；启动时从数据库加载的为零值，不推送） |
| `load_*` 开始 | 收到 Opened | 负载通知开启、未静音、有渠道 |
| `reboot`、`ip_change` | 收到 Instant | 对应开关开启、未静音、有渠道 |
| 时段事件结束 | 收到 Closed | 仅当该事件 `notify_state=1` 时推送恢复通知 |

- 推送成功后 `notify_state` 置 1；所有渠道在发送队列重试后仍失败时置 3，不再重试（避免渠道配置错误时每 30 秒重发），也不推送恢复通知。
- 推送状态是唯一的状态来源：入队前先记为发送中（4），Check 据此不重复推送。`Recorder.SetNotifyState` 与 `Close` 在同一把锁下执行，并返回事件当前的结束时间：结束时状态已是「已推送」的，由结束变更推送恢复；结束时仍在发送中的，由发送成功的回调发现事件已结束后推送恢复，失败则不推送。进程被强制结束时遗留的发送中事件不再推送。
- 不满足条件的置 2，之后取消静音或配置渠道都不会补发旧事件。
- 「渠道启用时间」保存在内存中，从「无渠道」变为「有渠道」时更新，取代原来的基线期。用跟踪开始时间而非事件开始时间比较：面板停机期间宕机的服务器，宽限期后新建的离线事件开始时间早于面板启动，但仍应推送。
- `notify_log.event_id` 关联到对应事件。

## 5. 设置

负载阈值决定的是「是否记录事件」，从通知设置移到新的「检测规则」分组：

| 新键 | 原键 | 默认 |
|---|---|---|
| `event_load_cpu` | `notify_load_cpu` | 90 |
| `event_load_mem` | `notify_load_mem` | 90 |
| `event_load_disk` | `notify_load_disk` | 90 |
| `event_load_minutes` | `notify_load_minutes` | 5 |

旧键在迁移 004 中删除，新键缺省时取默认值（不保留旧值）。

通知设置保留渠道与语言；规则为：离线（开关 + 延迟分钟数）、负载（开关）、重启（开关，默认开启）、IP 变化（开关，默认开启）、到期（开关 + 天数）、流量（开关 + 百分比）。

## 6. 接口

- 新增 `GET /api/admin/events`：查询参数 `server_id`、`kind`（逗号分隔多个）、`page`、`size`；返回 `{"items":[AdminEvent],"total":N}`，`AdminEvent` 为 `id, server_id, server_name, kind, start_at, end_at, duration, detail`。
- 删除 `GET /api/admin/outages`（只有后台前端在用，CHANGELOG 注明）。
- `GET /api/public/servers/:id/outages` 字段与行为不变，改为查询 `kind='offline'`。
- `notify_log` 条目增加 `event_id`。
- 设置接口增加 `events` 分组；通知分组增加 `reboot_enabled`、`ip_change_enabled`，去掉负载阈值字段。

## 7. 前端

- 后台「事件」页：「离线记录」标签改为「事件」，增加类型筛选（单选，三类负载合为「高负载」一项，网址参数 `type`）；每种类型以颜色圆点区分，按类型显示 detail（如「峰值 97.2%（阈值 90%，5 分钟均值）」「IPv4 1.2.3.4 → 5.6.7.8」）；瞬时事件的结束时间与时长显示「—」；进行中沿用现有标记。
- 设置页：新增「检测规则」（负载阈值与窗口）；通知设置增加重启、IP 变化开关，负载只保留开关。
- 详情页不变。
- zh-CN / en-US 文案。

## 8. 实施步骤

步骤 1～3 同时完成后才发布，中间状态不单独发布（避免出现两处负载判定的过渡版本）。

1. **存储与 Recorder**：迁移 004（`004_incidents.sql`）、store 方法、`incident.Recorder`；`outage.Tracker` 改为 `incident.Detector` 并通过 Recorder 写入离线。
2. **检测与通知**：Detector 增加负载判定；Notifier 改为订阅事件，删除离线 / 负载状态机；检测规则设置。现有通知测试改写后保持覆盖：离线延迟、恢复只在推送过后发送、静音不补发、渠道启用前的事件不补发、Dashboard 重启后恢复通知仍能发出。
3. **重启与 IP 变化**：hello 中的判定及推送开关。
4. **接口、前端与文档**：`/api/admin/events`、事件页、设置页、`docs/api.md`、`docs/configuration.md`、CHANGELOG。

每一步都跑 CLAUDE.md 要求的全部检查。

## 9. 风险

- **迁移不可逆**：迁移 004 删除 `outages` 表与旧的负载阈值设置，已有离线记录清空，降级到旧版本会因缺表无法读取离线记录。CHANGELOG 注明。
- **通知行为变化**：Notifier 是本次改动风险最大的部分，以第 4.4 节的表格为准逐条写测试；行为上与现状的差异只有两点：Dashboard 重启后恢复通知不再丢失；负载按指标分别通知。
