# 部署

本文分为 Dashboard 部署、反向代理、Agent 安装、升级与备份、从源码构建五部分。参数的完整说明见 [配置](configuration.md)。

发布资产的下载地址：

- 最新版：`https://github.com/ruanun/simple-server-status/releases/latest/download/<文件>`
- 指定版本：`https://github.com/ruanun/simple-server-status/releases/download/<tag>/<文件>`

归档名不含版本号，也不包一层目录：

| 文件 | 说明 |
|---|---|
| `sss-dashboard_<os>_<arch>.tar.gz` | Dashboard，Windows 为 `.zip` |
| `sss-agent_<os>_<arch>.tar.gz` | Agent，Windows 为 `.zip` |
| `install-agent.sh` / `install-agent.ps1` | Agent 安装脚本 |

`<os>` 为 `linux`、`windows`、`darwin`、`freebsd`；`<arch>` 为 `amd64`、`arm64`、`armv7`（Windows 只有 `amd64`）。

## Dashboard：Docker

镜像 `ruanun/sssd` 支持 `linux/amd64`、`linux/arm64`、`linux/arm/v7`。容器内程序为 `/app/sss-dashboard`，以 UID 1000 运行，数据目录为 `/app/data`，监听 8900 端口。

```bash
docker run -d --name sss-dashboard --restart unless-stopped \
  -p 8900:8900 -v sss-data:/app/data ruanun/sssd:latest
```

镜像内置的环境变量：`TZ=Asia/Shanghai`、`SSS_LISTEN=:8900`、`SSS_DATA_DIR=/app/data`。月流量的重置时间按 `TZ` 计算，需要时用 `-e TZ=...` 覆盖。

如果把宿主机目录挂载到 `/app/data`，需先让 UID 1000 可写：

```bash
mkdir -p ./data && sudo chown 1000:1000 ./data
docker run -d --name sss-dashboard --restart unless-stopped \
  -p 8900:8900 -v "$PWD/data":/app/data ruanun/sssd:latest
```

### docker compose

仓库提供了 [deployments/docker/docker-compose.yml](../deployments/docker/docker-compose.yml)，数据保存在命名卷 `sss-data`（挂载到 `/app/data`）。在仓库根目录执行：

```bash
docker compose -f deployments/docker/docker-compose.yml up -d
docker logs sss-dashboard 2>&1 | grep password   # 首次启动的 admin 密码
```

文件中注释掉的 `SSS_ADMIN_PASSWORD`、`SSS_TRUSTED_PROXIES` 按需启用。

## Dashboard：二进制 + systemd

以 Linux amd64 为例：

```bash
curl -fsSL -o /tmp/sss-dashboard.tar.gz \
  https://github.com/ruanun/simple-server-status/releases/latest/download/sss-dashboard_linux_amd64.tar.gz
sudo tar -xzf /tmp/sss-dashboard.tar.gz -C /usr/local/bin sss-dashboard
sudo useradd --system --no-create-home --shell /usr/sbin/nologin sss
sudo install -d -o sss -g sss -m 0750 /var/lib/sss
```

写入 `/etc/systemd/system/sss-dashboard.service`：

```ini
[Unit]
Description=Simple Server Status Dashboard
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=sss
Group=sss
ExecStart=/usr/local/bin/sss-dashboard --data-dir /var/lib/sss
Restart=on-failure
RestartSec=5s
# 加固：除数据目录外文件系统只读
NoNewPrivileges=true
ProtectSystem=strict
ReadWritePaths=/var/lib/sss
ProtectHome=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

启动并查看首次生成的 admin 密码：

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now sss-dashboard
journalctl -u sss-dashboard | grep password
```

其他参数（如 `--listen 127.0.0.1:8900`、`--trusted-proxies`）直接追加到 `ExecStart`，或用 `Environment=SSS_TRUSTED_PROXIES=127.0.0.1` 设置。

重置密码时以同一用户、同一数据目录执行：

```bash
sudo -u sss /usr/local/bin/sss-dashboard reset-password --data-dir /var/lib/sss
```

## 反向代理

Dashboard 本身不处理 HTTPS，公网部署建议放在反向代理之后。浏览器实时推送（`/api/public/ws`）与 Agent 上报（`/api/agent/ws`）都是 WebSocket，代理必须支持协议升级。

**必须设置可信代理**：Dashboard 默认不信任 `X-Forwarded-For`，所有请求的客户端 IP 都会是代理的地址，登录限流（同一 IP 5 分钟内失败 5 次锁定 5 分钟）会误伤所有人。把代理所在的 IP 或网段填入 `--trusted-proxies`（或 `SSS_TRUSTED_PROXIES`），多个用逗号分隔，例如：

- 代理与 Dashboard 在同一台机器：`--trusted-proxies 127.0.0.1`
- 代理在宿主机、Dashboard 在 Docker 中：`-e SSS_TRUSTED_PROXIES=172.16.0.0/12`（Docker 默认网段）

### Caddy

示例见 [deployments/caddy/Caddyfile](../deployments/caddy/Caddyfile)，Caddy 会自动申请证书，WebSocket 无需额外配置：

```caddyfile
status.example.com {
	encode zstd gzip
	reverse_proxy 127.0.0.1:8900
}
```

### Nginx

```nginx
map $http_upgrade $connection_upgrade {
    default upgrade;
    ''      close;
}

server {
    listen 443 ssl;
    server_name status.example.com;
    # ssl_certificate     /path/to/fullchain.pem;
    # ssl_certificate_key /path/to/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:8900;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

## Agent 安装

### 一键脚本

在后台「服务器」页新建服务器或点击「安装命令」，复制生成的命令到目标机器执行即可，命令格式：

```bash
# Linux（root，需要 systemd）
curl -fsSL '<脚本地址>/install-agent.sh' | sudo bash -s -- --dashboard '<面板地址>' --id '<ID>' --secret '<密钥>'
```

```powershell
# Windows（管理员 PowerShell）
Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass -Force; $f="$env:TEMP\install-agent.ps1"; iwr -useb '<脚本地址>/install-agent.ps1' -OutFile $f; & $f -Dashboard '<面板地址>' -Id '<ID>' -Secret '<密钥>'
```

`<脚本地址>` 来自后台设置「安装脚本地址」，默认 `https://github.com/ruanun/simple-server-status/releases/latest/download`。

Dashboard 为正式发布版本（含 beta 等预发布版本）时，后台生成的命令会锁定与 Dashboard 相同的版本：默认脚本地址换成该版本的 Release 地址 `https://github.com/ruanun/simple-server-status/releases/download/<tag>`，并自动追加 `--version <tag>`（Windows 为 `-Version <tag>`），保证 Agent 与 Dashboard 版本一致。自行编译的开发版本仍使用 latest。

手动执行脚本时默认安装最新版 Agent，指定版本时追加 `--version`（Windows 为 `-Version`），取值为发布标签：

```bash
curl -fsSL '<脚本地址>/install-agent.sh' | sudo bash -s -- --dashboard '<面板地址>' --id '<ID>' --secret '<密钥>' --version v2.0.0
```

GitHub 的 latest 不指向预发布版本，手动安装预发布版本时需使用 `releases/download/<tag>` 下的脚本并追加 `--version <tag>`。

重复执行安装命令会覆盖程序与配置并重启服务，可用于升级或更换密钥。

**Linux** 安装结果：

| 项目 | 位置 |
|---|---|
| 程序 | `/usr/local/bin/sss-agent` |
| 配置 | `/etc/sss/sss-agent.yaml`（权限 600，含密钥） |
| 服务 | systemd `sss-agent`，`Restart=on-failure`，并启用 `ProtectSystem=strict` 等加固选项 |
| 日志 | `journalctl -u sss-agent -f` |

服务文件把文件系统设为只读。如需在配置中启用 `log_file`，请写到 `/var/log/sss-agent/` 下（由 `LogsDirectory` 创建并可写）。

支持的架构：`x86_64`、`aarch64`、`armv7`。

**Windows** 安装结果（仅 amd64）：

| 项目 | 位置 |
|---|---|
| 程序 | `%ProgramFiles%\sss-agent\sss-agent.exe` |
| 配置 | `%ProgramFiles%\sss-agent\sss-agent.yaml`（仅 SYSTEM 与 Administrators 可读） |
| 服务 | `sss-agent`，开机自动启动，异常退出后 5 秒自动重启 |
| 日志 | `%ProgramFiles%\sss-agent\logs\agent.log`；读取配置失败等启动前的错误写入事件查看器「Windows 日志 → 应用程序」，来源 `sss-agent` |

```powershell
Get-Service sss-agent
Get-Content "$env:ProgramFiles\sss-agent\logs\agent.log" -Tail 50
```

在后台删除服务器时，Dashboard 会通知 Agent 退出，Agent 以正常状态退出，服务不会被自动重启。

### 卸载

```bash
# Linux：停止并删除服务、程序与配置
curl -fsSL 'https://github.com/ruanun/simple-server-status/releases/latest/download/install-agent.sh' | sudo bash -s -- --uninstall
```

```powershell
# Windows：删除服务与整个安装目录
Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass -Force; $f="$env:TEMP\install-agent.ps1"; iwr -useb 'https://github.com/ruanun/simple-server-status/releases/latest/download/install-agent.ps1' -OutFile $f; & $f -Uninstall
```

### 手动安装

适用于没有 systemd 的系统、macOS、FreeBSD，或希望自行管理进程的场景。以 Linux amd64 为例：

```bash
curl -fsSL -o /tmp/sss-agent.tar.gz \
  https://github.com/ruanun/simple-server-status/releases/latest/download/sss-agent_linux_amd64.tar.gz
sudo tar -xzf /tmp/sss-agent.tar.gz -C /usr/local/bin sss-agent

sudo install -d -m 0700 /etc/sss
sudo tee /etc/sss/sss-agent.yaml >/dev/null <<'EOF'
dashboard: 'https://status.example.com'
id: '<ID>'
secret: '<密钥>'
EOF
sudo chmod 600 /etc/sss/sss-agent.yaml

# 前台运行，确认能连上后再交给进程管理器
sudo sss-agent --config /etc/sss/sss-agent.yaml
```

不指定 `--config` 时，Agent 会依次查找 `./sss-agent.yaml`、`/etc/sss/sss-agent.yaml`。也可以不用配置文件，直接传参数：

```bash
sss-agent --dashboard https://status.example.com --id '<ID>' --secret '<密钥>'
```

归档中附带配置模板 `configs/sss-agent.yaml.example`，所有配置项见 [配置](configuration.md#agent)。

有 systemd 但不方便用一键脚本（如离线环境）时，可以照脚本的做法写入 `/etc/systemd/system/sss-agent.service`，之后按 [服务管理](#服务管理systemd) 操作：

```ini
[Unit]
Description=Simple Server Status Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/sss-agent --config /etc/sss/sss-agent.yaml
Restart=on-failure
RestartSec=5s
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=read-only
PrivateTmp=true
LogsDirectory=sss-agent

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now sss-agent
```

## 服务管理（systemd）

Dashboard 服务名为 `sss-dashboard`，Agent 为 `sss-agent`，下面以 Agent 为例，Dashboard 把服务名替换即可。

| 操作 | 命令 |
|---|---|
| 查看状态 | `systemctl status sss-agent` |
| 启动 / 停止 / 重启 | `sudo systemctl start sss-agent` / `sudo systemctl stop sss-agent` / `sudo systemctl restart sss-agent` |
| 开机自启 / 取消自启 | `sudo systemctl enable sss-agent` / `sudo systemctl disable sss-agent` |
| 查看日志 | `journalctl -u sss-agent -f`，更多见 [查看日志](#查看日志) |

两个服务都设置了 `Restart=on-failure`：异常退出后 5 秒自动重启；正常退出（如在后台删除了服务器，Agent 收到通知后退出）不会重启，此时 `systemctl status` 显示 `inactive (dead)`。

**修改配置后需要重启才生效：**

- Agent：编辑 `/etc/sss/sss-agent.yaml` 后执行 `sudo systemctl restart sss-agent`。采集间隔、网卡与挂载点过滤等参数由 Dashboard 下发，在后台修改即可，无需改配置或重启。
- Dashboard：不要直接改服务文件，用 `sudo systemctl edit sss-dashboard` 写入覆盖配置，保存后重启：

  ```ini
  [Service]
  Environment=SSS_LISTEN=127.0.0.1:8900
  Environment=SSS_TRUSTED_PROXIES=127.0.0.1
  ```

  ```bash
  sudo systemctl restart sss-dashboard
  ```

  覆盖配置保存在 `/etc/systemd/system/sss-dashboard.service.d/override.conf`，升级时替换程序文件不会影响它。启动参数与对应的环境变量见 [配置](configuration.md#dashboard)。

服务文件启用了 `ProtectSystem=strict`，文件系统除指定目录外只读。Dashboard 如需 `--log-file`，请写到数据目录 `/var/lib/sss` 下；Agent 的 `log_file` 请写到 `/var/log/sss-agent/` 下。

## 查看日志

Dashboard 与 Agent 默认只输出到标准输出，不写文件，日志位置取决于部署方式。

**Dashboard**

| 部署方式 | 命令 |
|---|---|
| Docker | `docker logs -f sss-dashboard`（最近 100 行：`docker logs --tail 100 sss-dashboard`） |
| Docker Compose | `docker compose -f deployments/docker/docker-compose.yml logs -f` |
| systemd | `journalctl -u sss-dashboard -f`（最近 100 行：`journalctl -u sss-dashboard -n 100 --no-pager`） |
| 前台运行 | 直接输出在终端 |

首次启动生成的 admin 密码也在日志中：`docker logs sss-dashboard 2>&1 | grep password` 或 `journalctl -u sss-dashboard | grep password`。

登录记录同样在日志中：每次登录成功与失败都会记下来源 IP、用户名与失败原因，用 `docker logs sss-dashboard 2>&1 | grep 登录` 或 `journalctl -u sss-dashboard | grep 登录` 查看。被限流拒绝的请求（`too_many_attempts`、`login_busy`）只在 `debug` 级别记录，避免被攻击时刷屏。容器删除重建（如升级）后 `docker logs` 中的旧日志会丢失，需要长期留存时按下文设置日志文件。

需要日志文件时设置 `--log-file`（或 `SSS_LOG_FILE`），设置后同时输出到标准输出与文件，文件按 10 MB 轮转，保留 5 份、30 天：

- systemd：路径放在 `/var/lib/sss` 下，用 `sudo systemctl edit sss-dashboard` 写入 `Environment=SSS_LOG_FILE=/var/lib/sss/dashboard.log` 后重启。
- Docker：路径放在 `/app/data` 下（如 `-e SSS_LOG_FILE=/app/data/dashboard.log`），日志随数据卷保存。

**Agent**

| 部署方式 | 查看方式 |
|---|---|
| Linux（一键脚本 / systemd） | `journalctl -u sss-agent -f`（最近 100 行：`journalctl -u sss-agent -n 100 --no-pager`，本次开机以来：`journalctl -u sss-agent -b`） |
| Windows（一键脚本） | `Get-Content "$env:ProgramFiles\sss-agent\logs\agent.log" -Tail 50 -Wait`；读取配置失败等启动前的错误在事件查看器「Windows 日志 → 应用程序」，来源 `sss-agent` |
| 前台运行 | 直接输出在终端 |

Linux 需要日志文件时，在 `/etc/sss/sss-agent.yaml` 中设置 `log_file: /var/log/sss-agent/agent.log` 后重启服务（systemd 下只有该目录可写）。

排查问题时可把日志级别调为 `debug`：Dashboard 用 `--log-level debug`（或 `SSS_LOG_LEVEL=debug`），Agent 在配置中设置 `log_level: debug`，改完重启。Agent 日志中出现 401 表示 ID 或密钥不对（例如密钥已在后台重置），重新执行后台的安装命令即可。

## 升级与备份

**升级**

- Docker：`docker pull ruanun/sssd:latest` 后删除并用原参数重新创建容器（数据在卷中，不受影响）；compose 部署执行 `docker compose -f deployments/docker/docker-compose.yml pull && docker compose -f deployments/docker/docker-compose.yml up -d`。
- 二进制：下载新版归档，替换 `/usr/local/bin/sss-dashboard` 后 `sudo systemctl restart sss-dashboard`。
- Agent：重新执行后台的安装命令即可（Agent 的采集参数由 Dashboard 下发，无需改配置）。

从 1.x 升级后，旧版目录不会被自动清理，可手动删除：Linux 为 `/opt/sss-agent`，Windows（若存在）为 `C:\sss-agent`。

**备份**

所有数据在数据目录（Docker 为 `/app/data`，默认 `./data`）中的 SQLite 数据库 `sss.db`（以及同目录的 `sss.db-wal`、`sss.db-shm`）。停止 Dashboard 后复制整个目录即可：

```bash
# 二进制部署
sudo systemctl stop sss-dashboard
sudo tar -czf sss-backup.tar.gz -C /var/lib/sss .
sudo systemctl start sss-dashboard

# Docker 命名卷
docker stop sss-dashboard
docker run --rm -v sss-data:/data -v "$PWD":/backup alpine tar -czf /backup/sss-backup.tar.gz -C /data .
docker start sss-dashboard
```

也可以在后台「设置 → 备份与迁移」导出 JSON，内容为站点设置和全部服务器（含 Agent 密钥），不含历史数据与流量统计。在新 Dashboard 导入后，服务器 ID 与密钥保持不变：面板地址不变（如沿用同一域名）时 Agent 无需任何改动；地址变化时修改 Agent 配置中的 `dashboard` 并重启，或重新执行安装命令。**导出文件包含密钥，请妥善保管。**

## 从源码构建

需要 Go 1.24+、Node.js 22+、pnpm 10。

```bash
make build
```

依次构建前端（输出到 `internal/dashboard/web/dist`，嵌入 Dashboard）和两个程序，产物为 `bin/sss-agent`、`bin/sss-dashboard`。版本号默认取 `git describe`，可用 `make build VERSION=v2.0.0` 指定。

Windows 没有 make 时，等价命令为：

```powershell
cd web
pnpm install --frozen-lockfile
pnpm build
cd ..
go build -trimpath -ldflags "-s -w -X main.version=dev" -o bin/sss-agent.exe ./cmd/sss-agent
go build -trimpath -ldflags "-s -w -X main.version=dev" -o bin/sss-dashboard.exe ./cmd/sss-dashboard
```

构建 Docker 镜像（在仓库根目录）：

```bash
docker build -f deployments/docker/Dockerfile -t sssd:dev .
```

推送 `v*` 标签会触发 GitHub Actions，用 goreleaser 发布各平台归档与安装脚本，并推送多架构镜像。本地试打包：`make release`。
