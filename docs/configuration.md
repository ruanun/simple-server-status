# 配置

Dashboard 与 Agent 的启动参数都可以用环境变量设置：变量名为 `SSS_` 加参数名大写、`-` 换成 `_`，例如 `--trusted-proxies` 对应 `SSS_TRUSTED_PROXIES`。命令行显式传入的参数优先于环境变量；环境变量取值无效（如布尔参数写成 `yes`）时程序直接报错退出。

站点标题、上报间隔等运行期设置不在启动参数中，而是在后台修改，保存后立即生效。

## Dashboard

子命令：

| 子命令 | 说明 |
|---|---|
| `serve` | 启动面板服务，不写子命令时默认执行 |
| `reset-password` | 为 `admin` 生成新的随机密码并打印，已登录的会话全部失效 |
| `version` | 显示版本 |

参数：

| 参数 | 环境变量 | 默认值 | 说明 |
|---|---|---|---|
| `--listen` | `SSS_LISTEN` | `:8900` | 监听地址 |
| `--data-dir` | `SSS_DATA_DIR` | `./data` | 数据目录，数据库文件为其中的 `sss.db`；`reset-password` 也按它定位数据库 |
| `--trusted-proxies` | `SSS_TRUSTED_PROXIES` | 空 | 可信反向代理的 IP 或 CIDR，逗号分隔。为空时不信任 `X-Forwarded-For`，客户端 IP 取连接地址 |
| `--admin-password` | `SSS_ADMIN_PASSWORD` | 空 | 首次初始化（数据库中还没有用户）时 `admin` 的密码；留空则随机生成并打印到日志。之后再设置不会生效 |
| `--log-level` | `SSS_LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |
| `--log-file` | `SSS_LOG_FILE` | 空 | 日志文件路径；留空只输出到标准输出。设置后同时输出到标准输出与文件，文件按 10 MB 轮转，保留 5 份、30 天 |

Docker 镜像已设置 `SSS_LISTEN=:8900`、`SSS_DATA_DIR=/app/data`、`TZ=Asia/Shanghai`。月流量周期按 Dashboard 所在时区计算。

示例：

```bash
sss-dashboard --listen 127.0.0.1:8900 --data-dir /var/lib/sss --trusted-proxies 127.0.0.1
SSS_DATA_DIR=/var/lib/sss sss-dashboard reset-password
```

## Agent

子命令：`run`（默认）、`version`。

配置可以来自 YAML 文件、环境变量或命令行参数，优先级：**命令行 > 环境变量 > YAML > 默认值**。

| YAML 键 | 参数 | 环境变量 | 默认值 | 说明 |
|---|---|---|---|---|
| — | `--config` | `SSS_CONFIG` | 空 | 配置文件路径，见下方查找顺序 |
| `dashboard` | `--dashboard` | `SSS_DASHBOARD` | 空（必填） | Dashboard 地址，以 `http://` 或 `https://` 开头，如 `https://status.example.com`。Agent 连接 `<地址>/api/agent/ws`，`https` 自动使用 `wss` |
| `id` | `--id` | `SSS_ID` | 空（必填） | 后台创建服务器后得到的 ID |
| `secret` | `--secret` | `SSS_SECRET` | 空（必填） | 服务器密钥 |
| `detect_country` | `--detect-country` | `SSS_DETECT_COUNTRY` | `true` | 启动时通过 Cloudflare trace 探测国家代码，用于显示国旗；后台填写了国家代码时以后台为准 |
| `log_level` | `--log-level` | `SSS_LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |
| `log_file` | `--log-file` | `SSS_LOG_FILE` | 空 | 日志文件路径，规则同 Dashboard |

上报间隔、网卡与挂载点过滤不在 Agent 配置中，由 Dashboard 在连接建立后下发，后台修改后立即推送给在线的 Agent。

### 配置文件查找顺序

1. 指定了 `--config`（或 `SSS_CONFIG`）时只读取该文件，文件不存在则报错。
2. 未指定时依次查找当前工作目录的 `./sss-agent.yaml`、`/etc/sss/sss-agent.yaml`，使用第一个存在的文件。
3. 都不存在时使用默认值，此时 `dashboard`、`id`、`secret` 必须通过参数或环境变量提供。

示例（与 `configs/sss-agent.yaml.example` 一致）：

```yaml
dashboard: "https://status.example.com"
id: "your-server-id"
secret: "your-secret"
detect_country: true
log_level: "info"
log_file: ""
```

安装脚本生成的配置位于 Linux `/etc/sss/sss-agent.yaml`、Windows `%ProgramFiles%\sss-agent\sss-agent.yaml`（Windows 额外设置了 `log_file` 为安装目录下的 `logs\agent.log`）。

## 后台设置

「设置」页的站点选项：

| 选项 | 默认值 | 说明 |
|---|---|---|
| 站点标题 | `Simple Server Status` | 显示在顶栏与浏览器标题；留空保存时恢复默认 |
| 公开显示价格 | 关闭 | 开启后未登录访客也能看到价格、币种与付费周期；登录后始终可见。到期日不受此项影响 |
| 默认上报间隔（秒） | `2` | 范围 1–60。新建或编辑服务器时「上报间隔」留空则使用此值（保存时写入该服务器，之后修改默认值不影响已有服务器） |
| 安装脚本地址 | `https://github.com/ruanun/simple-server-status/releases/latest/download` | 后台生成安装命令时使用的脚本下载前缀，需以 `http://` 或 `https://` 开头，末尾的 `/` 会被去掉 |

Dashboard 为正式发布版本时，若「安装脚本地址」保持默认，生成的命令会改用该版本的 Release 地址并追加 `--version`；改为其他地址时只追加 `--version`。「安装脚本地址」可改为自建镜像地址，要求该地址下能直接下载 `install-agent.sh` 与 `install-agent.ps1`（生成的命令为 `<地址>/install-agent.sh`、`<地址>/install-agent.ps1`）。注意脚本本身仍从 GitHub Release 下载 Agent 归档，目标机器需要能访问 GitHub。

同一页还提供修改密码（新密码至少 8 位，修改后其他已登录会话失效）以及导出 / 导入 JSON。导入时站点设置被文件覆盖，ID 相同的服务器被覆盖，其余新增。

## 服务器字段

后台「服务器」页新建 / 编辑弹窗分四组。

**基本**

| 字段 | 说明 |
|---|---|
| 名称 | 必填，不超过 64 个字符 |
| 分组 | 可选，首页可按分组筛选 |
| 国家代码 | 两位字母，如 `HK`，用于国旗与地区筛选；留空则使用 Agent 探测的结果 |
| 仅登录可见 | 开启后该服务器不出现在公开页面与公开接口中，登录后可见 |

**计费**

| 字段 | 说明 |
|---|---|
| 价格 | 非负数字，可留空 |
| 币种 | 自由文本，如 `$`、`¥`、`USD` |
| 付费周期 | 不设置 / 月付 / 季付 / 年付 / 一次性 |
| 到期日 | 可留空表示长期（显示为 ∞）；7 天内到期会高亮提示 |

价格、币种、付费周期是否对访客公开，由设置中的「公开显示价格」控制。

**流量**

| 字段 | 说明 |
|---|---|
| 流量配额（GB） | 每个周期的配额，1 GB 按 1024³ 字节计；留空表示不限 |
| 计量方式 | 双向合计 / 仅入站 / 仅出站，决定「已用流量」的计算方式 |
| 重置日（1–28） | 每月该日 00:00（Dashboard 时区）开始新的流量周期，默认 1 |

月流量由 Dashboard 根据 Agent 上报的网卡累计计数计算，Agent 重启或计数器归零不会丢失已统计的流量。

**采集**

| 字段 | 说明 |
|---|---|
| 上报间隔（秒） | 1–60，留空使用设置中的默认上报间隔 |
| 仅统计网卡 | 填写后只统计名称以这些前缀开头的网卡 |
| 排除网卡 | 未填写「仅统计网卡」时生效：排除名称以这些前缀开头的网卡。留空使用内置列表（`lo`、`lo0` 以及 `tun`、`docker`、`veth`、`br-`、`vmbr`、`vnet`、`kube`、`vethernet`、`loopback` 前缀），填写后替换内置列表 |
| 排除挂载点 | 不计入磁盘统计的挂载点，同时排除其下的子路径，如 `/boot` |

以上列表多个值用逗号分隔，网卡名匹配不区分大小写。修改网卡过滤后，月流量从新的统计口径继续累计，不会把口径变化造成的差值计入。

在线判定：Agent 连接存在，且最近一次上报距今不超过 3 倍上报间隔（最少 6 秒）。
