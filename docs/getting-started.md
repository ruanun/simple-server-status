# Simple Server Status 快速开始指南

> **作者**: ruan
> **最后更新**: 2025-11-15

本指南将帮助你在 **5-10 分钟内**完成 Simple Server Status 的完整部署，从零开始搭建服务器监控系统。

## 📋 部署概览

Simple Server Status 包含两个核心组件：

- **Dashboard（监控面板）** - 在一台服务器上部署，提供 Web 界面显示所有服务器状态
- **Agent（监控客户端）** - 在每台被监控服务器上部署，收集并上报系统信息

**部署流程：**
1. 部署 Dashboard（约 2 分钟）
2. 部署 Agent（约 3 分钟/台服务器）
3. 验证连接（约 1 分钟）

---

## 🚀 第一步：部署 Dashboard

### 方式 1：使用 Docker（推荐）

#### 1.1 准备配置文件

```bash
# 下载配置文件模板
wget https://raw.githubusercontent.com/ruanun/simple-server-status/main/configs/sss-dashboard.yaml.example -O sss-dashboard.yaml

# 编辑配置文件
nano sss-dashboard.yaml
```

**最小化配置示例：**

```yaml
port: 8900
address: 0.0.0.0
webSocketPath: /ws-report

servers:
  - name: Web Server 1
    id: web-server-01
    secret: "your-strong-secret-key-here"
    group: production
    countryCode: CN

  - name: Database Server
    id: db-server-01
    secret: "another-strong-secret-key"
    group: production
    countryCode: US
```

**配置说明：**
- `servers.id` - 服务器唯一标识符（3-50个字符，仅允许字母、数字、下划线、连字符）
- `servers.secret` - 认证密钥，**必须使用强密码**
- `servers.name` - 在 Dashboard 上显示的服务器名称
- `servers.group` - 服务器分组（可选）
- `servers.countryCode` - 国家代码（可选，2位字母）

**密钥生成建议：**

```bash
# Linux/macOS
openssl rand -base64 32
# 或
pwgen -s 32 1

# Windows PowerShell
-join ((65..90) + (97..122) + (48..57) | Get-Random -Count 32 | % {[char]$_})
```

#### 1.2 启动 Dashboard

```bash
docker run --name sssd \
  --restart=unless-stopped \
  -d \
  -v ./sss-dashboard.yaml:/app/sss-dashboard.yaml \
  -p 8900:8900 \
  ruanun/sssd
```

#### 1.3 验证 Dashboard 是否正常运行

```bash
# 查看容器状态
docker ps | grep sssd

# 查看日志
docker logs sssd

# 检查是否正常监听
curl http://localhost:8900/api/statistics
```

#### 1.4 配置防火墙

```bash
# Ubuntu/Debian
sudo ufw allow 8900/tcp

# CentOS/RHEL
sudo firewall-cmd --add-port=8900/tcp --permanent
sudo firewall-cmd --reload
```

#### 1.5 访问 Dashboard

在浏览器中打开：`http://your-server-ip:8900`

### 方式 2：使用二进制文件

#### 2.1 下载 Dashboard

```bash
# 查看最新版本
LATEST_VERSION=$(curl -s https://api.github.com/repos/ruanun/simple-server-status/releases/latest | grep '\"tag_name\":' | sed -E 's/.*\"([^\"]+)\".*/\1/')

# 下载（以 Linux amd64 为例）
wget https://github.com/ruanun/simple-server-status/releases/download/${LATEST_VERSION}/sss-dashboard_${LATEST_VERSION}_linux_amd64.tar.gz

# 解压
tar -xzf sss-dashboard_${LATEST_VERSION}_linux_amd64.tar.gz

# 移动到系统目录
sudo mv sss-dashboard /usr/local/bin/
sudo chmod +x /usr/local/bin/sss-dashboard
```

#### 2.2 创建配置文件

```bash
# 创建配置目录
sudo mkdir -p /etc/sss

# 下载配置模板
sudo wget https://raw.githubusercontent.com/ruanun/simple-server-status/main/configs/sss-dashboard.yaml.example \
  -O /etc/sss/sss-dashboard.yaml

# 编辑配置
sudo nano /etc/sss/sss-dashboard.yaml
```

#### 2.3 创建 systemd 服务（Linux）

创建服务文件 `/etc/systemd/system/sss-dashboard.service`：

```ini
[Unit]
Description=Simple Server Status Dashboard
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
Group=root
WorkingDirectory=/etc/sss
ExecStart=/usr/local/bin/sss-dashboard
Restart=on-failure
RestartSec=5s
Environment="CONFIG=/etc/sss/sss-dashboard.yaml"

[Install]
WantedBy=multi-user.target
```

启动服务：

```bash
sudo systemctl daemon-reload
sudo systemctl start sss-dashboard
sudo systemctl enable sss-dashboard
sudo systemctl status sss-dashboard
```

---

## 📱 第二步：部署 Agent

### 方式 1：使用安装脚本（推荐）

#### Linux / macOS / FreeBSD

```bash
# 一键安装
curl -fsSL https://raw.githubusercontent.com/ruanun/simple-server-status/main/scripts/install-agent.sh | sudo bash
```

安装完成后，编辑配置文件：

```bash
sudo nano /etc/sssa/sss-agent.yaml
```

**必须修改的配置项：**

```yaml
# Dashboard 地址（替换为你的 Dashboard IP 或域名）
serverAddr: ws://your-dashboard-ip:8900/ws-report

# 服务器唯一标识符（必须与 Dashboard 配置中的 servers.id 一致）
serverId: web-server-01

# 认证密钥（必须与 Dashboard 配置中的 servers.secret 一致）
authSecret: "your-strong-secret-key-here"

# 可选配置
logLevel: info
disableIP2Region: false
```

启动 Agent：

```bash
sudo systemctl start sssa
sudo systemctl enable sssa  # 设置开机自启

# 查看运行状态
sudo systemctl status sssa

# 查看日志
sudo journalctl -u sssa -f
```

#### Windows

```powershell
# 以管理员身份运行 PowerShell

# 一键安装
iwr -useb https://raw.githubusercontent.com/ruanun/simple-server-status/main/scripts/install-agent.ps1 | iex
```

安装完成后，编辑配置文件：

```powershell
notepad "C:\Program Files\SSSA\sss-agent.yaml"
```

修改配置后，通过服务管理器启动 SSSA 服务：

```powershell
# 启动服务
Start-Service -Name "SSSA"

# 查看服务状态
Get-Service -Name "SSSA"
```

### 方式 2：手动安装

手动安装的详细步骤请参考：[手动安装指南](deployment/manual.md)

---

## ✅ 第三步：验证连接

### 3.1 检查 Dashboard

1. 访问 Dashboard Web 界面：`http://your-server-ip:8900`
2. 检查服务器是否显示为**在线**状态
3. 查看是否有实时数据更新

### 3.2 检查 Agent

**Linux / macOS / FreeBSD:**

```bash
# 查看 Agent 状态
sudo systemctl status sssa

# 查看日志（应该看到 "连接成功" 的消息）
sudo journalctl -u sssa -n 50
```

**Windows:**

```powershell
# 查看服务状态
Get-Service -Name "SSSA"

# 查看 Windows 事件日志
Get-EventLog -LogName Application -Source "SSSA" -Newest 20
```

### 3.3 连接失败排查

如果服务器显示离线，请按以下步骤排查：

#### ① 检查 Dashboard 是否正常运行

```bash
# Docker
docker ps | grep sssd
docker logs sssd

# 二进制
sudo systemctl status sss-dashboard
sudo journalctl -u sss-dashboard -n 50
```

#### ② 检查 Agent 是否正常运行

```bash
# Linux
sudo systemctl status sssa
sudo journalctl -u sssa -n 50

# Windows
Get-Service -Name "SSSA"
```

#### ③ 验证配置文件

确认以下配置项完全一致：

- Dashboard 的 `servers.id` ↔ Agent 的 `serverId`
- Dashboard 的 `servers.secret` ↔ Agent 的 `authSecret`
- Agent 的 `serverAddr` 格式正确：`ws://dashboard-ip:8900/ws-report`
- `serverAddr` 路径与 Dashboard 的 `webSocketPath` 一致（默认 `/ws-report`）

#### ④ 检查网络连接

```bash
# 测试 Dashboard 端口是否可访问
telnet your-dashboard-ip 8900
# 或
nc -zv your-dashboard-ip 8900
```

#### ⑤ 检查防火墙

```bash
# Ubuntu/Debian
sudo ufw status
sudo ufw allow 8900

# CentOS/RHEL
sudo firewall-cmd --list-all
sudo firewall-cmd --add-port=8900/tcp --permanent
sudo firewall-cmd --reload
```

更多故障排除信息，请参考：[故障排除完整指南](../troubleshooting.md)

---

## 🔧 快速命令参考

### Dashboard 管理（Docker）

```bash
docker start sssd      # 启动
docker stop sssd       # 停止
docker restart sssd    # 重启
docker logs sssd       # 查看日志
docker logs -f sssd    # 实时日志
```

### Dashboard 管理（systemd）

```bash
sudo systemctl start sss-dashboard    # 启动
sudo systemctl stop sss-dashboard     # 停止
sudo systemctl restart sss-dashboard  # 重启
sudo systemctl status sss-dashboard   # 查看状态
sudo journalctl -u sss-dashboard -f   # 实时日志
```

### Agent 管理（Linux / macOS）

```bash
sudo systemctl start sssa      # 启动
sudo systemctl stop sssa       # 停止
sudo systemctl restart sssa    # 重启
sudo systemctl status sssa     # 查看状态
sudo journalctl -u sssa -f     # 实时日志
```

### Agent 管理（Windows）

```powershell
Start-Service -Name "SSSA"     # 启动
Stop-Service -Name "SSSA"      # 停止
Restart-Service -Name "SSSA"   # 重启
Get-Service -Name "SSSA"       # 查看状态
```

---

## ❓ 常见问题

### Q1: 如何监控多台服务器？

1. 在 Dashboard 配置中添加多个服务器：

```yaml
servers:
  - id: "server-01"
    name: "生产服务器-1"
    secret: "secret-key-1"
  - id: "server-02"
    name: "生产服务器-2"
    secret: "secret-key-2"
  - id: "server-03"
    name: "开发服务器"
    secret: "secret-key-3"
```

2. 在每台服务器上安装 Agent 并配置对应的 ID 和密钥
3. 所有 Agent 的 `serverAddr` 指向同一个 Dashboard

### Q2: 如何配置 HTTPS 访问？

使用反向代理（Nginx 或 Caddy）配置 HTTPS。详细配置请参考：

- [反向代理配置指南](deployment/proxy.md)
- [Nginx 配置示例](deployment/proxy.md#nginx)
- [Caddy 配置示例](deployment/proxy.md#caddy)

配置反向代理后，Agent 的 `serverAddr` 需要改为：

```yaml
serverAddr: wss://your-domain.com/ws-report  # 注意使用 wss://
```

### Q3: 数据收集频率可以调整吗？

可以。在 Agent 配置文件中调整：

```yaml
collectInterval: 2s  # 数据收集间隔，默认 2 秒
```

**建议值：**
- 高频监控：1s - 2s
- 标准监控：3s - 5s（推荐）
- 低频监控：10s - 30s

⚠️ 注意：间隔越短，CPU 占用越高。

### Q4: 支持哪些操作系统？

**完全支持：**
- Linux（x86_64, ARM64, ARMv7）
- Windows（x86_64, ARM64）
- macOS（x86_64, ARM64/Apple Silicon）
- FreeBSD（x86_64）

**已测试的 Linux 发行版：**
- Ubuntu 18.04+, Debian 10+, CentOS 7+, Rocky Linux 8+, Arch Linux, Alpine Linux

### Q5: 资源占用情况如何？

**Agent（单个实例）：**
- 内存：约 8-15 MB
- CPU：< 0.5%（采集间隔 2s）
- 磁盘：约 5 MB
- 网络：约 1-2 KB/s

**Dashboard（监控 10 台服务器）：**
- 内存：约 30-50 MB
- CPU：< 2%
- 磁盘：约 20 MB
- 网络：约 10-20 KB/s

✅ 非常轻量，适合资源受限的环境。

---

## 📚 下一步

完成快速开始后，你可以：

- 📖 [配置 HTTPS 反向代理](deployment/proxy.md) - 使用 Nginx/Caddy 配置 HTTPS
- 🐳 [Docker 完整部署指南](deployment/docker.md) - Docker Compose、网络配置等
- ⚙️ [systemd 部署指南](deployment/systemd.md) - 生产环境 systemd 服务配置
- 🔧 [手动安装指南](deployment/manual.md) - 不使用脚本的手动安装步骤
- 🐛 [故障排除指南](../troubleshooting.md) - 详细的故障排除和调试方法
- 🔄 [维护指南](../maintenance.md) - 更新、备份、卸载等维护操作

---

## 🆘 获取帮助

如果遇到问题，可以通过以下方式获取帮助：

- 📖 查看 [故障排除指南](../troubleshooting.md)
- 🐛 提交 [GitHub Issue](https://github.com/ruanun/simple-server-status/issues)
- 💬 参与 [GitHub Discussions](https://github.com/ruanun/simple-server-status/discussions)

---

**版本**: 1.0
**作者**: ruan
**最后更新**: 2025-11-15
