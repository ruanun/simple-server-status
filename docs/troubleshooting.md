# Simple Server Status 故障排除指南

> **作者**: ruan
> **最后更新**: 2025-11-15

本指南提供详细的故障排除步骤和常见问题解答，帮助你快速解决部署和运行中遇到的问题。

## 📋 目录

- [连接问题](#连接问题)
- [认证问题](#认证问题)
- [服务启动问题](#服务启动问题)
- [配置问题](#配置问题)
- [网络和防火墙问题](#网络和防火墙问题)
- [更新问题](#更新问题)
- [性能问题](#性能问题)
- [平台特定问题](#平台特定问题)
- [调试模式](#调试模式)

---

## 🔌 连接问题

### Agent 无法连接到 Dashboard

**症状：** Dashboard 显示服务器离线，或 Agent 日志显示连接失败

#### 排查步骤

**① 检查 Dashboard 是否正常运行**

```bash
# Docker 部署
docker ps | grep sssd  # 应该看到运行中的容器
docker logs sssd       # 查看 Dashboard 日志

# 二进制/systemd 部署
sudo systemctl status sss-dashboard
sudo journalctl -u sss-dashboard -n 50
```

**预期结果：**
- Docker 容器状态为 `Up`
- systemd 服务状态为 `active (running)`
- 日志中显示 "服务器启动成功" 或类似消息

**② 检查 Agent 是否正常运行**

```bash
# Linux/macOS
sudo systemctl status sssa
sudo journalctl -u sssa -n 50

# Windows
Get-Service -Name "SSSA"
Get-EventLog -LogName Application -Source "SSSA" -Newest 20
```

**预期结果：**
- 服务状态为 `active (running)`
- 日志中显示 "连接成功" 或 "WebSocket connected"

**③ 验证配置文件**

检查以下配置项是否完全一致：

| Dashboard 配置 | Agent 配置 | 说明 |
|---------------|-----------|------|
| `servers.id` | `serverId` | 必须完全一致（区分大小写） |
| `servers.secret` | `authSecret` | 必须完全一致（区分大小写） |
| - | `serverAddr` | 格式：`ws://dashboard-ip:8900/ws-report` |

**Dashboard 配置文件检查：**

```bash
# Docker
cat sss-dashboard.yaml | grep -A 3 "servers:"

# systemd
sudo cat /etc/sss/sss-dashboard.yaml | grep -A 3 "servers:"
```

**Agent 配置文件检查：**

```bash
# Linux/macOS
sudo cat /etc/sssa/sss-agent.yaml

# Windows
type "C:\Program Files\SSSA\sss-agent.yaml"
```

**④ 检查网络连通性**

```bash
# 测试 Dashboard 端口是否可访问
telnet your-dashboard-ip 8900

# 或使用 nc
nc -zv your-dashboard-ip 8900

# 或使用 curl
curl -I http://your-dashboard-ip:8900
```

**预期结果：**
- `Connected to ...` 或 `succeeded!`
- 如果失败，说明网络不通或防火墙阻止

**⑤ 检查防火墙设置**

```bash
# Ubuntu/Debian
sudo ufw status
sudo ufw allow 8900/tcp
sudo ufw reload

# CentOS/RHEL
sudo firewall-cmd --list-all
sudo firewall-cmd --add-port=8900/tcp --permanent
sudo firewall-cmd --reload

# 检查 iptables
sudo iptables -L -n | grep 8900
```

**⑥ 检查 WebSocket 路径配置**

Dashboard 的 `webSocketPath` 和 Agent 的 `serverAddr` 路径必须一致。

**Dashboard 配置：**

```yaml
webSocketPath: /ws-report  # 默认值，推荐以 '/' 开头
```

**Agent 配置：**

```yaml
serverAddr: ws://192.168.1.100:8900/ws-report  # 路径必须匹配
```

**注意：** 旧格式 `ws-report`（无前导斜杠）会自动兼容，但建议使用新格式 `/ws-report`。

### WebSocket 连接断开

**症状：** Agent 日志显示 "连接已断开" 或频繁重连

**可能原因：**
1. 网络不稳定
2. Dashboard 重启
3. 反向代理超时配置不当
4. 防火墙关闭了长连接

**解决方法：**

1. **检查网络稳定性**

```bash
# 持续 ping Dashboard
ping -c 100 your-dashboard-ip

# 查看网络延迟
mtr your-dashboard-ip
```

2. **如果使用反向代理，增加超时时间**

**Nginx 配置：**

```nginx
location /ws-report {
    proxy_pass http://localhost:8900;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_read_timeout 86400;  # 24小时
    proxy_send_timeout 86400;  # 24小时
}
```

**Caddy 配置：**

```caddyfile
your-domain.com {
    reverse_proxy localhost:8900 {
        transport http {
            read_timeout 24h
            write_timeout 24h
        }
    }
}
```

3. **检查 Dashboard 日志**

```bash
# Docker
docker logs -f sssd | grep -i "disconnect\|error"

# systemd
sudo journalctl -u sss-dashboard -f | grep -i "disconnect\|error"
```

---

## 🔐 认证问题

### 认证失败

**症状：** Agent 日志显示 "认证失败" 或 "authentication failed"

**原因：** `serverId` 或 `authSecret` 配置不匹配

#### 解决步骤

**① 检查 Dashboard 配置**

```bash
# 查看 Dashboard 配置的服务器列表
docker logs sssd 2>&1 | grep -A 5 "已配置的服务器"
# 或
sudo journalctl -u sss-dashboard | grep -A 5 "已配置的服务器"
```

**② 检查 Agent 配置**

```bash
# Linux/macOS
sudo cat /etc/sssa/sss-agent.yaml | grep -E "serverId|authSecret"

# Windows
type "C:\Program Files\SSSA\sss-agent.yaml" | findstr /I "serverId authSecret"
```

**③ 确认配置一致性**

| 项目 | Dashboard | Agent | 状态 |
|------|-----------|-------|------|
| ID | `servers.id: "web-server-01"` | `serverId: "web-server-01"` | ✅ |
| 密钥 | `servers.secret: "abc123"` | `authSecret: "abc123"` | ✅ |

**注意事项：**
- ID 和密钥**区分大小写**
- 不要有多余的空格或引号
- YAML 格式要求密钥用引号包裹

**④ 重启 Agent 使配置生效**

```bash
# Linux/macOS
sudo systemctl restart sssa
sudo journalctl -u sssa -f

# Windows
Restart-Service -Name "SSSA"
Get-EventLog -LogName Application -Source "SSSA" -Newest 5
```

### 密钥被泄露需要更换

**步骤：**

1. 生成新密钥

```bash
# Linux/macOS
openssl rand -base64 32

# Windows PowerShell
-join ((65..90) + (97..122) + (48..57) | Get-Random -Count 32 | % {[char]$_})
```

2. 更新 Dashboard 配置

```bash
# 编辑配置文件
nano sss-dashboard.yaml  # 或 sudo nano /etc/sss/sss-dashboard.yaml

# 修改对应服务器的 secret
servers:
  - id: "web-server-01"
    secret: "NEW-SECRET-KEY-HERE"  # 更新为新密钥
```

3. 重启 Dashboard

```bash
# Docker
docker restart sssd

# systemd
sudo systemctl restart sss-dashboard
```

4. 更新所有 Agent 配置

```bash
# 在每台 Agent 服务器上
sudo nano /etc/sssa/sss-agent.yaml

# 修改 authSecret
authSecret: "NEW-SECRET-KEY-HERE"

# 重启 Agent
sudo systemctl restart sssa
```

---

## 🚀 服务启动问题

### Dashboard 无法启动

#### Docker 容器无法启动

**症状：** `docker ps` 没有显示 sssd 容器

**排查步骤：**

```bash
# 查看容器状态（包括已停止的）
docker ps -a | grep sssd

# 查看容器日志
docker logs sssd

# 检查端口是否被占用
sudo netstat -tulpn | grep 8900
# 或
sudo lsof -i :8900
```

**常见原因和解决方法：**

**1. 端口被占用**

```bash
# 查看占用 8900 端口的进程
sudo lsof -i :8900

# 终止占用进程（谨慎操作）
sudo kill -9 <PID>

# 或修改 Dashboard 端口
nano sss-dashboard.yaml
# 修改 port: 8901

# 重新启动容器
docker rm sssd
docker run --name sssd -d -p 8901:8901 ...
```

**2. 配置文件格式错误**

```bash
# 验证 YAML 格式
python3 -c "import yaml; yaml.safe_load(open('sss-dashboard.yaml'))"

# 如果报错，检查：
# - 缩进是否正确（使用空格，不要用 Tab）
# - 引号是否配对
# - 字段名是否拼写正确
```

**3. 配置文件路径不对**

```bash
# 确认配置文件存在
ls -la ./sss-dashboard.yaml

# 重新挂载配置文件
docker run --name sssd -d \
  -v $(pwd)/sss-dashboard.yaml:/app/sss-dashboard.yaml \
  -p 8900:8900 \
  ruanun/sssd
```

**4. 权限问题**

```bash
# 检查配置文件权限
ls -l ./sss-dashboard.yaml

# 如果权限不足，修改权限
chmod 644 ./sss-dashboard.yaml
```

#### systemd 服务无法启动

**症状：** `systemctl status sss-dashboard` 显示 `failed` 或 `inactive`

**排查步骤：**

```bash
# 查看详细状态
sudo systemctl status sss-dashboard -l

# 查看启动日志
sudo journalctl -u sss-dashboard -b

# 查看最近的错误日志
sudo journalctl -u sss-dashboard --since "10 minutes ago"
```

**常见原因和解决方法：**

**1. 二进制文件权限问题**

```bash
# 检查权限
ls -l /usr/local/bin/sss-dashboard

# 添加执行权限
sudo chmod +x /usr/local/bin/sss-dashboard
```

**2. 配置文件路径错误**

```bash
# 检查服务文件中的配置路径
sudo cat /etc/systemd/system/sss-dashboard.service | grep CONFIG

# 确认配置文件存在
ls -l /etc/sss/sss-dashboard.yaml

# 修改服务文件
sudo nano /etc/systemd/system/sss-dashboard.service

# 重新加载并启动
sudo systemctl daemon-reload
sudo systemctl restart sss-dashboard
```

**3. 端口被占用**

```bash
# 查看占用端口的进程
sudo netstat -tulpn | grep 8900

# 修改配置文件中的端口
sudo nano /etc/sss/sss-dashboard.yaml
# port: 8901

# 重启服务
sudo systemctl restart sss-dashboard
```

### Agent 无法启动

#### Linux/macOS Agent 无法启动

**症状：** `systemctl status sssa` 显示 `failed`

**排查步骤：**

```bash
# 查看详细状态
sudo systemctl status sssa -l

# 查看启动日志
sudo journalctl -u sssa -b

# 手动运行 Agent 查看错误
sudo /etc/sssa/sss-agent -c /etc/sssa/sss-agent.yaml
```

**常见原因：**

**1. 配置文件格式错误**

```bash
# 检查 YAML 格式
python3 -c "import yaml; yaml.safe_load(open('/etc/sssa/sss-agent.yaml'))"

# 常见错误：
# - serverAddr 缺少引号
# - 缩进不正确
# - serverId 拼写错误
```

**2. 二进制文件权限问题**

```bash
# 添加执行权限
sudo chmod +x /etc/sssa/sss-agent
sudo chmod +x /usr/local/bin/sss-agent
```

**3. 网络问题**

```bash
# 测试能否访问 Dashboard
curl -I http://your-dashboard-ip:8900

# 如果需要代理
export HTTP_PROXY=http://proxy:8080
export HTTPS_PROXY=http://proxy:8080
sudo -E systemctl restart sssa
```

#### Windows Agent 无法启动

**症状：** 服务管理器中 SSSA 服务状态为 "已停止"

**排查步骤：**

```powershell
# 查看服务状态
Get-Service -Name "SSSA"

# 查看 Windows 事件日志
Get-EventLog -LogName Application -Source "SSSA" -Newest 20

# 手动运行 Agent 查看错误
& "C:\Program Files\SSSA\sss-agent.exe" -c "C:\Program Files\SSSA\sss-agent.yaml"
```

**常见原因：**

**1. 配置文件路径问题**

```powershell
# 检查配置文件是否存在
Test-Path "C:\Program Files\SSSA\sss-agent.yaml"

# 查看配置文件内容
Get-Content "C:\Program Files\SSSA\sss-agent.yaml"
```

**2. 权限不足**

```powershell
# 以管理员身份运行 PowerShell
Start-Process powershell -Verb runAs

# 重新安装服务
sc.exe delete "SSSA"
.\install-agent.ps1  # 重新运行安装脚本
```

**3. 防火墙阻止**

```powershell
# 检查 Windows 防火墙规则
Get-NetFirewallRule | Where-Object {$_.DisplayName -like "*SSSA*"}

# 添加防火墙规则允许出站连接
New-NetFirewallRule -DisplayName "SSSA Agent" -Direction Outbound -Action Allow -Program "C:\Program Files\SSSA\sss-agent.exe"
```

---

## ⚙️ 配置问题

### 配置文件修改后不生效

**原因：** 没有重启服务

**解决方法：**

```bash
# Dashboard (Docker)
docker restart sssd

# Dashboard (systemd)
sudo systemctl restart sss-dashboard

# Agent (Linux/macOS)
sudo systemctl restart sssa

# Agent (Windows)
Restart-Service -Name "SSSA"
```

### 不确定配置是否正确

**验证配置文件格式：**

```bash
# 验证 YAML 格式
python3 -c "import yaml; yaml.safe_load(open('sss-dashboard.yaml'))"

# 如果命令成功执行（无输出），说明格式正确
# 如果有错误，会显示具体的错误行号和原因
```

**检查必填字段：**

**Dashboard 配置：**
- ✅ `port`
- ✅ `servers` (至少一个)
- ✅ `servers.id`
- ✅ `servers.name`
- ✅ `servers.secret`

**Agent 配置：**
- ✅ `serverAddr`
- ✅ `serverId`
- ✅ `authSecret`

### 自定义 WebSocket 路径

如果需要修改默认的 `/ws-report` 路径：

**1. 修改 Dashboard 配置**

```yaml
webSocketPath: /custom-path  # 自定义路径，必须以 '/' 开头
```

**2. 修改所有 Agent 配置**

```yaml
serverAddr: ws://your-dashboard-ip:8900/custom-path  # 路径必须匹配
```

**3. 如果使用反向代理，同步修改**

```nginx
# Nginx
location /custom-path {
    proxy_pass http://localhost:8900;
    # ...
}
```

**4. 重启所有服务**

```bash
# Dashboard
docker restart sssd

# 所有 Agent
sudo systemctl restart sssa
```

---

## 🌐 网络和防火墙问题

### 云服务器安全组配置

**阿里云/腾讯云/AWS 等云服务器，需要在安全组中开放端口：**

**入站规则：**
- 协议：TCP
- 端口：8900
- 来源：0.0.0.0/0（或指定 IP）

**出站规则：**
- 通常默认允许所有出站流量

### Docker 网络问题

**症状：** Dashboard 容器运行正常，但外部无法访问

**检查 Docker 端口映射：**

```bash
# 查看端口映射
docker port sssd

# 应该显示：
# 8900/tcp -> 0.0.0.0:8900
```

**检查 Docker 网络：**

```bash
# 查看容器网络
docker inspect sssd | grep -A 10 "Networks"

# 确认 HostPort 正确
docker inspect sssd | grep "HostPort"
```

**解决方法：**

```bash
# 重新创建容器，确保端口映射正确
docker stop sssd
docker rm sssd

docker run --name sssd -d \
  -p 8900:8900 \  # 主机端口:容器端口
  -v ./sss-dashboard.yaml:/app/sss-dashboard.yaml \
  ruanun/sssd
```

### IPv6 配置

如果需要使用 IPv6：

**Agent 配置：**

```yaml
serverAddr: ws://[2001:db8::1]:8900/ws-report  # IPv6 地址用方括号包裹
```

**Dashboard 配置：**

```yaml
address: "::"  # 监听所有 IPv6 地址
# 或
address: "0.0.0.0"  # 同时支持 IPv4 和 IPv6
```

---

## 🔄 更新问题

### 如何更新到最新版本？

#### 更新 Dashboard

**Docker 部署：**

```bash
# 1. 拉取最新镜像
docker pull ruanun/sssd:latest

# 2. 停止并删除旧容器
docker stop sssd
docker rm sssd

# 3. 使用新镜像启动（使用相同的配置文件）
docker run --name sssd \
  --restart=unless-stopped \
  -d \
  -v ./sss-dashboard.yaml:/app/sss-dashboard.yaml \
  -p 8900:8900 \
  ruanun/sssd:latest

# 4. 验证版本
docker logs sssd | grep "版本"
```

**二进制部署：**

```bash
# 1. 下载最新版本
LATEST_VERSION=$(curl -s https://api.github.com/repos/ruanun/simple-server-status/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
wget https://github.com/ruanun/simple-server-status/releases/download/${LATEST_VERSION}/sss-dashboard_${LATEST_VERSION}_linux_amd64.tar.gz

# 2. 停止服务
sudo systemctl stop sss-dashboard

# 3. 备份旧版本
sudo cp /usr/local/bin/sss-dashboard /usr/local/bin/sss-dashboard.bak

# 4. 解压并替换
tar -xzf sss-dashboard_${LATEST_VERSION}_linux_amd64.tar.gz
sudo mv sss-dashboard /usr/local/bin/
sudo chmod +x /usr/local/bin/sss-dashboard

# 5. 启动服务
sudo systemctl start sss-dashboard

# 6. 验证
/usr/local/bin/sss-dashboard --version
sudo systemctl status sss-dashboard
```

#### 更新 Agent

**Linux/macOS/FreeBSD：**

```bash
# 重新运行安装脚本即可自动更新（会保留配置）
curl -fsSL https://raw.githubusercontent.com/ruanun/simple-server-status/main/scripts/install-agent.sh | sudo bash

# 验证版本
sss-agent --version
sudo systemctl status sssa
```

**Windows：**

```powershell
# 重新运行安装脚本
iwr -useb https://raw.githubusercontent.com/ruanun/simple-server-status/main/scripts/install-agent.ps1 | iex

# 验证版本
& "C:\Program Files\SSSA\sss-agent.exe" --version
Get-Service -Name "SSSA"
```

### 更新后服务无法启动

**可能原因：** 配置文件格式与新版本不兼容

**解决方法：**

```bash
# 1. 查看更新日志
# https://github.com/ruanun/simple-server-status/releases

# 2. 对比配置文件模板
wget https://raw.githubusercontent.com/ruanun/simple-server-status/main/configs/sss-dashboard.yaml.example -O sss-dashboard.yaml.new

# 3. 合并配置（保留旧配置的 ID 和密钥，使用新模板的格式）
diff sss-dashboard.yaml sss-dashboard.yaml.new

# 4. 重启服务
docker restart sssd  # 或 sudo systemctl restart sss-dashboard
```

---

## ⚡ 性能问题

### Dashboard CPU 占用过高

**症状：** Dashboard CPU 持续 > 50%

**原因分析：**
1. 监控的服务器数量过多
2. Agent 上报频率过高
3. WebSocket 连接不稳定导致频繁重连

**解决方法：**

**1. 调整 Agent 上报频率**

```yaml
# Agent 配置文件
collectInterval: 5s  # 从 2s 增加到 5s
```

**2. 检查是否有大量无效连接**

```bash
# 查看 WebSocket 连接数
docker logs sssd | grep -c "WebSocket connected"

# 如果连接数远超配置的服务器数量，说明有频繁重连
```

**3. 增加 Dashboard 资源限制**

```bash
# Docker 设置 CPU 和内存限制
docker run --name sssd -d \
  --cpus="1.0" \
  --memory="512m" \
  -v ./sss-dashboard.yaml:/app/sss-dashboard.yaml \
  -p 8900:8900 \
  ruanun/sssd
```

### Agent 内存占用持续增长

**症状：** Agent 内存从 10MB 增长到 100MB+

**原因：** 可能存在内存泄漏

**解决方法：**

**1. 重启 Agent**

```bash
sudo systemctl restart sssa
```

**2. 禁用 IP 地理位置查询**

```yaml
# Agent 配置文件
disableIP2Region: true  # 减少内存占用
```

**3. 降低日志级别**

```yaml
# Agent 配置文件
logLevel: warn  # 从 info 或 debug 改为 warn
```

**4. 更新到最新版本**

```bash
# 最新版本可能已修复内存泄漏问题
curl -fsSL https://raw.githubusercontent.com/ruanun/simple-server-status/main/scripts/install-agent.sh | sudo bash
```

**5. 提交 Issue**

如果问题持续存在，请提交 Issue 并附上：
- Agent 版本
- 操作系统和架构
- 运行时间和内存增长曲线
- 配置文件（去除敏感信息）

### 数据更新延迟

**症状：** Dashboard 显示的数据比实际延迟几秒甚至几分钟

**原因分析：**
1. 网络延迟过高
2. Agent collectInterval 设置过大
3. Dashboard 处理能力不足

**解决方法：**

**1. 检查网络延迟**

```bash
# 从 Agent 服务器 ping Dashboard
ping -c 10 your-dashboard-ip

# 如果平均延迟 > 100ms，考虑：
# - 使用更近的数据中心
# - 优化网络路由
```

**2. 调整 Agent 采集频率**

```yaml
# Agent 配置文件
collectInterval: 2s  # 降低延迟（会增加资源占用）
```

**3. 检查 Dashboard 负载**

```bash
# 查看 Dashboard CPU 和内存
docker stats sssd

# 如果负载过高，考虑：
# - 减少监控的服务器数量
# - 增加 Dashboard 资源配置
# - 部署多个 Dashboard 分散负载
```

---

## 🖥️ 平台特定问题

### macOS 特定问题

#### 安装脚本执行失败

**症状：** `permission denied` 或 `command not found`

**解决方法：**

```bash
# 检查是否安装了 curl
which curl

# 如果未安装
brew install curl

# 使用 sudo 执行安装脚本
curl -fsSL https://raw.githubusercontent.com/ruanun/simple-server-status/main/scripts/install-agent.sh | sudo bash
```

#### 没有 systemd

**解决方法：** macOS 不使用 systemd，需要使用 launchd 或手动运行

**使用 launchd：**

创建 `~/Library/LaunchAgents/com.simple-server-status.agent.plist`：

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.simple-server-status.agent</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/sss-agent</string>
        <string>-c</string>
        <string>/etc/sssa/sss-agent.yaml</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
</dict>
</plist>
```

加载服务：

```bash
launchctl load ~/Library/LaunchAgents/com.simple-server-status.agent.plist
launchctl start com.simple-server-status.agent
```

**手动运行：**

```bash
sudo /usr/local/bin/sss-agent -c /etc/sssa/sss-agent.yaml &
```

### FreeBSD 特定问题

#### 包管理器不支持

**解决方法：** 使用 `pkg` 安装依赖

```bash
# 安装 curl
sudo pkg install curl

# 运行安装脚本
curl -fsSL https://raw.githubusercontent.com/ruanun/simple-server-status/main/scripts/install-agent.sh | sudo bash
```

#### rc.d 服务配置

创建 `/usr/local/etc/rc.d/sssa`：

```bash
#!/bin/sh
#
# PROVIDE: sssa
# REQUIRE: NETWORKING
# KEYWORD: shutdown

. /etc/rc.subr

name="sssa"
rcvar=sssa_enable
command="/usr/local/bin/sss-agent"
command_args="-c /etc/sssa/sss-agent.yaml"

load_rc_config $name
run_rc_command "$1"
```

启用服务：

```bash
sudo chmod +x /usr/local/etc/rc.d/sssa
sudo sysrc sssa_enable="YES"
sudo service sssa start
```

### Windows 特定问题

#### PowerShell 执行策略限制

**症状：** `无法加载脚本` 或 `execution policy`

**解决方法：**

```powershell
# 临时允许（推荐）
Set-ExecutionPolicy -ExecutionPolicy Bypass -Scope Process
iwr -useb https://raw.githubusercontent.com/ruanun/simple-server-status/main/scripts/install-agent.ps1 | iex

# 或永久允许（需要管理员权限）
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
```

#### 服务安装失败

**症状：** `Access denied` 或 `Service installation failed`

**解决方法：**

```powershell
# 1. 以管理员身份运行 PowerShell
Start-Process powershell -Verb runAs

# 2. 检查是否有权限
([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole] "Administrator")

# 应该返回 True

# 3. 重新运行安装脚本
.\install-agent.ps1
```

#### Windows Defender 阻止

**症状：** 安装文件被删除或隔离

**解决方法：**

```powershell
# 添加排除项
Add-MpPreference -ExclusionPath "C:\Program Files\SSSA"

# 或临时禁用实时保护（谨慎操作）
Set-MpPreference -DisableRealtimeMonitoring $true
# 安装完成后重新启用
Set-MpPreference -DisableRealtimeMonitoring $false
```

---

## 🐛 调试模式

### 启用详细日志

#### Dashboard 调试

**Docker：**

```bash
# 编辑配置文件
nano sss-dashboard.yaml

# 修改日志级别
logLevel: debug

# 重启容器
docker restart sssd

# 查看详细日志
docker logs -f sssd
```

**systemd：**

```bash
# 编辑配置文件
sudo nano /etc/sss/sss-dashboard.yaml

# 修改日志级别
logLevel: debug

# 重启服务
sudo systemctl restart sss-dashboard

# 查看详细日志
sudo journalctl -u sss-dashboard -f
```

#### Agent 调试

```bash
# 编辑配置文件
sudo nano /etc/sssa/sss-agent.yaml

# 修改日志级别
logLevel: debug

# 重启服务
sudo systemctl restart sssa

# 查看详细日志
sudo journalctl -u sssa -f
```

### 手动运行查看错误

#### Dashboard 手动运行

```bash
# Docker
docker stop sssd
docker run -it --rm \
  -v ./sss-dashboard.yaml:/app/sss-dashboard.yaml \
  -p 8900:8900 \
  ruanun/sssd

# 二进制
sudo systemctl stop sss-dashboard
sudo /usr/local/bin/sss-dashboard -c /etc/sss/sss-dashboard.yaml
```

#### Agent 手动运行

```bash
# Linux/macOS
sudo systemctl stop sssa
sudo /etc/sssa/sss-agent -c /etc/sssa/sss-agent.yaml

# Windows
Stop-Service -Name "SSSA"
& "C:\Program Files\SSSA\sss-agent.exe" -c "C:\Program Files\SSSA\sss-agent.yaml"
```

### 网络抓包调试

**抓取 WebSocket 通信：**

```bash
# 安装 tcpdump
sudo apt install tcpdump  # Ubuntu/Debian
sudo yum install tcpdump  # CentOS/RHEL

# 抓取 8900 端口的流量
sudo tcpdump -i any -nn port 8900 -A

# 或使用 Wireshark 进行更详细的分析
```

---

## 📞 获取帮助

如果以上方法都无法解决问题，可以通过以下方式获取帮助：

### 提交 Issue

在提交 Issue 前，请准备以下信息：

1. **环境信息**
   - 操作系统和版本
   - Dashboard 部署方式（Docker/二进制）
   - Agent 部署方式（脚本/手动）
   - 版本号

2. **配置文件**（去除敏感信息）
   ```yaml
   # Dashboard 配置
   port: 8900
   servers:
     - id: "server-01"
       name: "服务器1"
       secret: "***已隐藏***"

   # Agent 配置
   serverAddr: ws://xxx.xxx.xxx.xxx:8900/ws-report
   serverId: "server-01"
   authSecret: "***已隐藏***"
   ```

3. **错误日志**
   ```bash
   # Dashboard 日志
   docker logs sssd --tail 100

   # Agent 日志
   sudo journalctl -u sssa -n 100
   ```

4. **复现步骤**
   - 详细描述如何触发问题
   - 预期行为和实际行为

### 社区支持

- 🐛 [GitHub Issues](https://github.com/ruanun/simple-server-status/issues) - 报告 Bug 和功能请求
- 💬 [GitHub Discussions](https://github.com/ruanun/simple-server-status/discussions) - 社区讨论和问答
- 📖 [文档](https://github.com/ruanun/simple-server-status/tree/main/docs) - 查看完整文档

---

**版本**: 1.0
**作者**: ruan
**最后更新**: 2025-11-15
