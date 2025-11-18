# Simple Server Status 手动安装指南

> **作者**: ruan
> **最后更新**: 2025-11-15

本指南提供不使用安装脚本或 Docker 的手动安装步骤，适合需要自定义安装或在特殊环境下部署的用户。

**推荐方式：**
- 新用户和标准部署：建议使用[快速开始指南](../getting-started.md)中的安装脚本或 Docker 方式
- 生产环境和高级用户：可以参考本手动安装指南

## 📋 目录

- [前置准备](#前置准备)
- [手动安装 Agent](#手动安装-agent)
- [手动安装 Dashboard](#手动安装-dashboard)
- [配置服务自动启动](#配置服务自动启动)
- [验证安装](#验证安装)

---

## 📦 前置准备

### 系统要求

**Agent:**
- 操作系统：Linux, Windows, macOS, FreeBSD
- 内存：最低 10MB
- CPU：最低 0.1%
- 磁盘：最低 5MB
- 网络：支持 WebSocket 连接

**Dashboard:**
- 操作系统：Linux, Windows, macOS, FreeBSD
- 内存：最低 20MB
- CPU：最低 0.5%
- 磁盘：最低 10MB
- 端口：默认 8900（可配置）

### 下载地址

从 [GitHub Releases](https://github.com/ruanun/simple-server-status/releases) 页面下载对应系统的二进制文件。

**命名格式：**
- Agent: `sss-agent_{version}_{os}_{arch}.tar.gz` 或 `.zip`
- Dashboard: `sss-dashboard_{version}_{os}_{arch}.tar.gz` 或 `.zip`

**示例：**
- Linux AMD64 Agent: `sss-agent_v1.0.0_linux_amd64.tar.gz`
- Windows AMD64 Agent: `sss-agent_v1.0.0_windows_amd64.zip`
- Linux AMD64 Dashboard: `sss-dashboard_v1.0.0_linux_amd64.tar.gz`

---

## 📱 手动安装 Agent

### Linux

#### 1. 下载二进制文件

```bash
# 查看最新版本
LATEST_VERSION=$(curl -s https://api.github.com/repos/ruanun/simple-server-status/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

# 下载（根据你的架构选择）
wget https://github.com/ruanun/simple-server-status/releases/download/${LATEST_VERSION}/sss-agent_${LATEST_VERSION}_linux_amd64.tar.gz

# 其他架构：
# ARM64: sss-agent_${LATEST_VERSION}_linux_arm64.tar.gz
# ARMv7: sss-agent_${LATEST_VERSION}_linux_armv7.tar.gz
```

#### 2. 解压并安装

```bash
# 解压文件
tar -xzf sss-agent_${LATEST_VERSION}_linux_amd64.tar.gz
cd sss-agent_${LATEST_VERSION}_linux_amd64

# 创建安装目录
sudo mkdir -p /etc/sssa

# 复制二进制文件
sudo cp sss-agent /etc/sssa/
sudo chmod +x /etc/sssa/sss-agent

# 创建符号链接（可选，方便命令行调用）
sudo ln -sf /etc/sssa/sss-agent /usr/local/bin/sss-agent
```

#### 3. 创建配置文件

```bash
# 下载配置模板
sudo wget https://raw.githubusercontent.com/ruanun/simple-server-status/main/configs/sss-agent.yaml.example \
  -O /etc/sssa/sss-agent.yaml

# 或手动创建配置文件
sudo nano /etc/sssa/sss-agent.yaml
```

**配置文件示例：**

```yaml
# Dashboard 地址（替换为实际的 Dashboard IP 或域名）
serverAddr: ws://192.168.1.100:8900/ws-report

# 服务器唯一标识符（必须与 Dashboard 配置中的 servers.id 一致）
serverId: web-server-01

# 认证密钥（必须与 Dashboard 配置中的 servers.secret 一致）
authSecret: "your-strong-secret-key-here"

# 可选配置
logLevel: info
disableIP2Region: false
```

#### 4. 设置权限

```bash
# 设置配置文件权限
sudo chmod 600 /etc/sssa/sss-agent.yaml
sudo chown root:root /etc/sssa/sss-agent.yaml

# 设置二进制文件权限
sudo chmod 755 /etc/sssa/sss-agent
```

#### 5. 测试运行

```bash
# 手动运行测试
sudo /etc/sssa/sss-agent -c /etc/sssa/sss-agent.yaml

# 如果看到 "连接成功" 或 "WebSocket connected" 消息，说明配置正确
# 按 Ctrl+C 停止
```

#### 6. 配置 systemd 服务（推荐）

参考[配置服务自动启动](#配置-systemd-服务linux)章节。

### macOS

#### 1. 下载二进制文件

```bash
# 查看最新版本
LATEST_VERSION=$(curl -s https://api.github.com/repos/ruanun/simple-server-status/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

# 下载（根据你的架构选择）
# Apple Silicon (M1/M2/M3):
wget https://github.com/ruanun/simple-server-status/releases/download/${LATEST_VERSION}/sss-agent_${LATEST_VERSION}_darwin_arm64.tar.gz

# Intel Mac:
# wget https://github.com/ruanun/simple-server-status/releases/download/${LATEST_VERSION}/sss-agent_${LATEST_VERSION}_darwin_amd64.tar.gz
```

#### 2. 解压并安装

```bash
# 解压文件
tar -xzf sss-agent_${LATEST_VERSION}_darwin_arm64.tar.gz
cd sss-agent_${LATEST_VERSION}_darwin_arm64

# 创建安装目录
sudo mkdir -p /etc/sssa

# 复制二进制文件
sudo cp sss-agent /etc/sssa/
sudo chmod +x /etc/sssa/sss-agent

# 创建符号链接
sudo ln -sf /etc/sssa/sss-agent /usr/local/bin/sss-agent
```

#### 3. 创建配置文件

```bash
# 下载配置模板
sudo curl -fsSL https://raw.githubusercontent.com/ruanun/simple-server-status/main/configs/sss-agent.yaml.example \
  -o /etc/sssa/sss-agent.yaml

# 编辑配置
sudo nano /etc/sssa/sss-agent.yaml
```

配置内容参考 Linux 章节。

#### 4. 配置 launchd 服务（可选）

参考[配置服务自动启动](#配置-launchd-服务macos)章节。

### FreeBSD

#### 1. 下载二进制文件

```bash
# 查看最新版本
LATEST_VERSION=$(curl -s https://api.github.com/repos/ruanun/simple-server-status/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

# 下载
fetch https://github.com/ruanun/simple-server-status/releases/download/${LATEST_VERSION}/sss-agent_${LATEST_VERSION}_freebsd_amd64.tar.gz
```

#### 2. 解压并安装

```bash
# 解压文件
tar -xzf sss-agent_${LATEST_VERSION}_freebsd_amd64.tar.gz
cd sss-agent_${LATEST_VERSION}_freebsd_amd64

# 创建安装目录
sudo mkdir -p /etc/sssa

# 复制二进制文件
sudo cp sss-agent /etc/sssa/
sudo chmod +x /etc/sssa/sss-agent

# 创建符号链接
sudo ln -sf /etc/sssa/sss-agent /usr/local/bin/sss-agent
```

#### 3. 创建配置文件

```bash
# 下载配置模板
sudo fetch https://raw.githubusercontent.com/ruanun/simple-server-status/main/configs/sss-agent.yaml.example \
  -o /etc/sssa/sss-agent.yaml

# 编辑配置
sudo ee /etc/sssa/sss-agent.yaml
```

#### 4. 配置 rc.d 服务（可选）

参考[配置服务自动启动](#配置-rcd-服务freebsd)章节。

### Windows

#### 1. 下载二进制文件

1. 访问 [GitHub Releases](https://github.com/ruanun/simple-server-status/releases)
2. 下载对应架构的 Windows 版本：
   - x64: `sss-agent_v1.0.0_windows_amd64.zip`
   - x86: `sss-agent_v1.0.0_windows_386.zip`
   - ARM64: `sss-agent_v1.0.0_windows_arm64.zip`

#### 2. 解压并安装

```powershell
# 解压文件到临时目录
Expand-Archive -Path "sss-agent_v1.0.0_windows_amd64.zip" -DestinationPath "$env:TEMP\sss-agent"

# 创建安装目录
New-Item -ItemType Directory -Path "C:\Program Files\SSSA" -Force

# 复制文件
Copy-Item "$env:TEMP\sss-agent\sss-agent_v1.0.0_windows_amd64\sss-agent.exe" "C:\Program Files\SSSA\"
```

#### 3. 创建配置文件

```powershell
# 下载配置模板
Invoke-WebRequest -Uri "https://raw.githubusercontent.com/ruanun/simple-server-status/main/configs/sss-agent.yaml.example" -OutFile "C:\Program Files\SSSA\sss-agent.yaml"

# 编辑配置
notepad "C:\Program Files\SSSA\sss-agent.yaml"
```

配置内容参考 Linux 章节。

#### 4. 添加到系统 PATH（可选）

```powershell
# 获取当前 PATH
$currentPath = [Environment]::GetEnvironmentVariable("Path", "Machine")

# 添加 SSSA 目录
[Environment]::SetEnvironmentVariable("Path", "$currentPath;C:\Program Files\SSSA", "Machine")

# 验证
$env:Path = [Environment]::GetEnvironmentVariable("Path", "Machine")
sss-agent --version
```

#### 5. 配置 Windows 服务（可选）

参考[配置服务自动启动](#配置-windows-服务)章节。

---

## 🖥️ 手动安装 Dashboard

### Linux

#### 1. 下载二进制文件

```bash
# 查看最新版本
LATEST_VERSION=$(curl -s https://api.github.com/repos/ruanun/simple-server-status/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

# 下载
wget https://github.com/ruanun/simple-server-status/releases/download/${LATEST_VERSION}/sss-dashboard_${LATEST_VERSION}_linux_amd64.tar.gz
```

#### 2. 解压并安装

```bash
# 解压文件
tar -xzf sss-dashboard_${LATEST_VERSION}_linux_amd64.tar.gz

# 移动到系统目录
sudo mv sss-dashboard /usr/local/bin/
sudo chmod +x /usr/local/bin/sss-dashboard
```

#### 3. 创建配置文件

```bash
# 创建配置目录
sudo mkdir -p /etc/sss

# 下载配置模板
sudo wget https://raw.githubusercontent.com/ruanun/simple-server-status/main/configs/sss-dashboard.yaml.example \
  -O /etc/sss/sss-dashboard.yaml

# 编辑配置
sudo nano /etc/sss/sss-dashboard.yaml
```

**配置文件示例：**

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

logLevel: info
logPath: /var/log/sss/dashboard.log
```

#### 4. 创建日志目录

```bash
# 创建日志目录
sudo mkdir -p /var/log/sss

# 设置权限
sudo chmod 755 /var/log/sss
```

#### 5. 测试运行

```bash
# 手动运行测试
sudo /usr/local/bin/sss-dashboard -c /etc/sss/sss-dashboard.yaml

# 在另一个终端测试访问
curl http://localhost:8900/api/statistics

# 按 Ctrl+C 停止
```

#### 6. 配置 systemd 服务（推荐）

参考[配置服务自动启动](#配置-systemd-服务linux-1)章节。

### Windows

#### 1. 下载并安装

```powershell
# 下载（从 GitHub Releases 页面）
# 解压到临时目录
Expand-Archive -Path "sss-dashboard_v1.0.0_windows_amd64.zip" -DestinationPath "$env:TEMP\sss-dashboard"

# 创建安装目录
New-Item -ItemType Directory -Path "C:\Program Files\SSSD" -Force

# 复制文件
Copy-Item "$env:TEMP\sss-dashboard\sss-dashboard_v1.0.0_windows_amd64\sss-dashboard.exe" "C:\Program Files\SSSD\"
```

#### 2. 创建配置文件

```powershell
# 下载配置模板
Invoke-WebRequest -Uri "https://raw.githubusercontent.com/ruanun/simple-server-status/main/configs/sss-dashboard.yaml.example" -OutFile "C:\Program Files\SSSD\sss-dashboard.yaml"

# 编辑配置
notepad "C:\Program Files\SSSD\sss-dashboard.yaml"
```

#### 3. 创建日志目录

```powershell
New-Item -ItemType Directory -Path "C:\Program Files\SSSD\logs" -Force
```

#### 4. 配置 Windows 服务（可选）

使用 NSSM 或其他工具将 Dashboard 注册为 Windows 服务。

---

## ⚙️ 配置服务自动启动

### 配置 systemd 服务（Linux）

#### Agent 服务

创建服务文件 `/etc/systemd/system/sssa.service`：

```bash
sudo nano /etc/systemd/system/sssa.service
```

**服务配置：**

```ini
[Unit]
Description=Simple Server Status Agent
Documentation=https://github.com/ruanun/simple-server-status
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
Group=root
WorkingDirectory=/etc/sssa
ExecStart=/etc/sssa/sss-agent -c /etc/sssa/sss-agent.yaml
Restart=always
RestartSec=5s

# 安全加固
NoNewPrivileges=true
PrivateTmp=true

# 资源限制
LimitNOFILE=65536

# 环境变量
Environment="CONFIG=/etc/sssa/sss-agent.yaml"

[Install]
WantedBy=multi-user.target
```

**启动服务：**

```bash
sudo systemctl daemon-reload
sudo systemctl start sssa
sudo systemctl enable sssa
sudo systemctl status sssa
```

#### Dashboard 服务

创建服务文件 `/etc/systemd/system/sss-dashboard.service`：

```bash
sudo nano /etc/systemd/system/sss-dashboard.service
```

**服务配置：**

```ini
[Unit]
Description=Simple Server Status Dashboard
Documentation=https://github.com/ruanun/simple-server-status
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
Group=root
WorkingDirectory=/etc/sss
ExecStart=/usr/local/bin/sss-dashboard -c /etc/sss/sss-dashboard.yaml
Restart=on-failure
RestartSec=5s

# 安全加固
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/log/sss /etc/sss

# 资源限制
LimitNOFILE=65536

# 环境变量
Environment="CONFIG=/etc/sss/sss-dashboard.yaml"

[Install]
WantedBy=multi-user.target
```

**启动服务：**

```bash
sudo systemctl daemon-reload
sudo systemctl start sss-dashboard
sudo systemctl enable sss-dashboard
sudo systemctl status sss-dashboard
```

### 配置 launchd 服务（macOS）

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
        <string>/etc/sssa/sss-agent</string>
        <string>-c</string>
        <string>/etc/sssa/sss-agent.yaml</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardErrorPath</key>
    <string>/tmp/sss-agent.err</string>
    <key>StandardOutPath</key>
    <string>/tmp/sss-agent.out</string>
</dict>
</plist>
```

**加载服务：**

```bash
launchctl load ~/Library/LaunchAgents/com.simple-server-status.agent.plist
launchctl start com.simple-server-status.agent

# 查看状态
launchctl list | grep simple-server-status
```

### 配置 rc.d 服务（FreeBSD）

创建 `/usr/local/etc/rc.d/sssa`：

```bash
sudo ee /usr/local/etc/rc.d/sssa
```

**服务脚本：**

```sh
#!/bin/sh
#
# PROVIDE: sssa
# REQUIRE: NETWORKING
# KEYWORD: shutdown

. /etc/rc.subr

name="sssa"
rcvar=sssa_enable
command="/etc/sssa/sss-agent"
command_args="-c /etc/sssa/sss-agent.yaml"
pidfile="/var/run/${name}.pid"

load_rc_config $name
run_rc_command "$1"
```

**启用服务：**

```bash
sudo chmod +x /usr/local/etc/rc.d/sssa
sudo sysrc sssa_enable="YES"
sudo service sssa start
sudo service sssa status
```

### 配置 Windows 服务

#### 使用 NSSM（推荐）

```powershell
# 下载 NSSM
# https://nssm.cc/download

# 安装 Agent 服务
nssm install SSSA "C:\Program Files\SSSA\sss-agent.exe" "-c" "C:\Program Files\SSSA\sss-agent.yaml"
nssm set SSSA DisplayName "Simple Server Status Agent"
nssm set SSSA Description "监控客户端"
nssm set SSSA Start SERVICE_AUTO_START

# 启动服务
nssm start SSSA

# 查看状态
nssm status SSSA
```

#### 使用 sc 命令

```powershell
# 创建服务
sc.exe create SSSA binPath= "C:\Program Files\SSSA\sss-agent.exe -c C:\Program Files\SSSA\sss-agent.yaml" start= auto

# 启动服务
sc.exe start SSSA

# 查看状态
sc.exe query SSSA
```

---

## ✅ 验证安装

### 验证 Agent

```bash
# 检查版本
sss-agent --version

# 检查服务状态
# Linux
sudo systemctl status sssa
sudo journalctl -u sssa -n 50

# macOS
launchctl list | grep simple-server-status
tail -f /tmp/sss-agent.out

# FreeBSD
sudo service sssa status

# Windows
Get-Service -Name "SSSA"
```

**预期结果：**
- 服务状态为 `active (running)`
- 日志中显示 "连接成功" 或 "WebSocket connected"

### 验证 Dashboard

```bash
# 检查版本
sss-dashboard --version

# 检查服务状态
sudo systemctl status sss-dashboard
sudo journalctl -u sss-dashboard -n 50

# 测试 HTTP 接口
curl http://localhost:8900/api/statistics

# 访问 Web 界面
# 浏览器打开: http://your-server-ip:8900
```

**预期结果：**
- 服务状态为 `active (running)`
- curl 返回 JSON 数据
- Web 界面可以正常访问

### 验证连接

在 Dashboard Web 界面中检查：
1. 服务器是否显示为在线状态（绿色）
2. 是否有实时数据更新
3. CPU、内存、网络等指标是否正常显示

---

## 🔍 故障排除

如果遇到问题，请参考：
- [故障排除指南](../troubleshooting.md)
- [快速开始指南](../getting-started.md)

---

## 📚 相关文档

- 📖 [快速开始指南](../getting-started.md) - 使用脚本或 Docker 快速部署
- 🐳 [Docker 部署](docker.md) - 使用 Docker 容器化部署
- ⚙️ [systemd 部署](systemd.md) - 生产环境 systemd 服务配置
- 🐛 [故障排除](../troubleshooting.md) - 常见问题解决

---

**版本**: 1.0
**作者**: ruan
**最后更新**: 2025-11-15
