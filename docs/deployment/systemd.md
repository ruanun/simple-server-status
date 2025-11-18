# systemd 部署指南

> **作者**: ruan
> **最后更新**: 2025-11-05

## 概述

本文档介绍如何使用 systemd 在 Linux 系统上部署和管理 SimpleServerStatus 服务。systemd 是现代 Linux 发行版的标准服务管理器，支持自动启动、重启和日志管理。

## 前置要求

- **操作系统**: Linux 发行版（Ubuntu 16.04+, CentOS 7+, Debian 8+）
- **systemd**: 已安装并运行（大多数现代 Linux 发行版默认包含）
- **权限**: root 或 sudo 权限

## Dashboard 部署

### 1. 下载二进制文件

**方式 1: 从 GitHub Releases 下载**:

```bash
# 查看最新版本
LATEST_VERSION=$(curl -s https://api.github.com/repos/ruanun/simple-server-status/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

# 下载 Dashboard（以 Linux amd64 为例）
wget https://github.com/ruanun/simple-server-status/releases/download/${LATEST_VERSION}/sss-dashboard_${LATEST_VERSION}_linux_amd64.tar.gz

# 解压
tar -xzf sss-dashboard_${LATEST_VERSION}_linux_amd64.tar.gz

# 移动到系统目录
sudo mv sss-dashboard /usr/local/bin/
sudo chmod +x /usr/local/bin/sss-dashboard
```

**方式 2: 从源码编译**:

```bash
# 克隆项目
git clone https://github.com/ruanun/simple-server-status.git
cd simple-server-status

# 编译 Dashboard
go build -o sss-dashboard ./cmd/dashboard

# 移动到系统目录
sudo mv sss-dashboard /usr/local/bin/
sudo chmod +x /usr/local/bin/sss-dashboard
```

### 2. 创建配置文件

```bash
# 创建配置目录
sudo mkdir -p /etc/sss

# 下载配置模板
sudo wget https://raw.githubusercontent.com/ruanun/simple-server-status/main/configs/sss-dashboard.yaml.example \
  -O /etc/sss/sss-dashboard.yaml

# 编辑配置
sudo nano /etc/sss/sss-dashboard.yaml
```

**配置示例** (`/etc/sss/sss-dashboard.yaml`):

```yaml
port: 8900
address: 0.0.0.0
webSocketPath: /ws-report  # 非必填，推荐以 '/' 开头

servers:
  - name: Web Server 1
    id: web-1
    secret: "your-secret-key-1"
    group: production
    countryCode: CN

  - name: Database Server
    id: db-1
    secret: "your-secret-key-2"
    group: production
    countryCode: CN

logLevel: info
logPath: /var/log/sss/dashboard.log
```

### 3. 创建日志目录

```bash
# 创建日志目录
sudo mkdir -p /var/log/sss

# 设置权限（如果使用非 root 用户运行）
sudo chown -R sss:sss /var/log/sss
```

### 4. 创建 systemd 服务文件

**创建服务文件** (`/etc/systemd/system/sss-dashboard.service`):

```bash
sudo nano /etc/systemd/system/sss-dashboard.service
```

**服务配置**:

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
ExecStart=/usr/local/bin/sss-dashboard
ExecReload=/bin/kill -HUP $MAINPID
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
LimitNPROC=512

# 环境变量
Environment="CONFIG=/etc/sss/sss-dashboard.yaml"

# 日志配置
StandardOutput=journal
StandardError=journal
SyslogIdentifier=sss-dashboard

[Install]
WantedBy=multi-user.target
```

### 5. 启动服务

```bash
# 重新加载 systemd 配置
sudo systemctl daemon-reload

# 启动服务
sudo systemctl start sss-dashboard

# 查看状态
sudo systemctl status sss-dashboard

# 设置开机自启
sudo systemctl enable sss-dashboard

# 查看日志
sudo journalctl -u sss-dashboard -f
```

## Agent 部署

### 1. 下载二进制文件

```bash
# 查看最新版本
LATEST_VERSION=$(curl -s https://api.github.com/repos/ruanun/simple-server-status/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

# 下载 Agent
wget https://github.com/ruanun/simple-server-status/releases/download/${LATEST_VERSION}/sss-agent_${LATEST_VERSION}_linux_amd64.tar.gz

# 解压
tar -xzf sss-agent_${LATEST_VERSION}_linux_amd64.tar.gz

# 移动到系统目录
sudo mv sss-agent /usr/local/bin/
sudo chmod +x /usr/local/bin/sss-agent
```

### 2. 创建配置文件

```bash
# 创建配置目录
sudo mkdir -p /etc/sss

# 下载配置模板
sudo wget https://raw.githubusercontent.com/ruanun/simple-server-status/main/configs/sss-agent.yaml.example \
  -O /etc/sss/sss-agent.yaml

# 编辑配置
sudo nano /etc/sss/sss-agent.yaml
```

**配置示例** (`/etc/sss/sss-agent.yaml`):

```yaml
# Dashboard 地址
serverAddr: ws://dashboard-host:8900/ws-report

# 服务器标识（必须与 Dashboard 配置匹配）
serverId: web-1

# 认证密钥（必须与 Dashboard 配置匹配）
authSecret: "your-secret-key-1"

# 日志配置
logLevel: info
logPath: /var/log/sss/agent.log
```

### 3. 创建 systemd 服务文件

**创建服务文件** (`/etc/systemd/system/sss-agent.service`):

```bash
sudo nano /etc/systemd/system/sss-agent.service
```

**服务配置**:

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
WorkingDirectory=/etc/sss
ExecStart=/usr/local/bin/sss-agent
ExecReload=/bin/kill -HUP $MAINPID
Restart=always
RestartSec=5s

# 安全加固
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/log/sss /etc/sss

# 资源限制
LimitNOFILE=65536
LimitNPROC=512

# 环境变量
Environment="CONFIG=/etc/sss/sss-agent.yaml"

# 日志配置
StandardOutput=journal
StandardError=journal
SyslogIdentifier=sss-agent

[Install]
WantedBy=multi-user.target
```

### 4. 启动服务

```bash
# 重新加载 systemd 配置
sudo systemctl daemon-reload

# 启动服务
sudo systemctl start sss-agent

# 查看状态
sudo systemctl status sss-agent

# 设置开机自启
sudo systemctl enable sss-agent

# 查看日志
sudo journalctl -u sss-agent -f
```

## 服务管理

### 基本命令

```bash
# 启动服务
sudo systemctl start sss-dashboard
sudo systemctl start sss-agent

# 停止服务
sudo systemctl stop sss-dashboard
sudo systemctl stop sss-agent

# 重启服务
sudo systemctl restart sss-dashboard
sudo systemctl restart sss-agent

# 重新加载配置（不重启服务）
sudo systemctl reload sss-dashboard
sudo systemctl reload sss-agent

# 查看状态
sudo systemctl status sss-dashboard
sudo systemctl status sss-agent

# 开机自启
sudo systemctl enable sss-dashboard
sudo systemctl enable sss-agent

# 禁用开机自启
sudo systemctl disable sss-dashboard
sudo systemctl disable sss-agent

# 查看是否启用开机自启
sudo systemctl is-enabled sss-dashboard
sudo systemctl is-enabled sss-agent
```

### 查看日志

```bash
# 实时查看日志
sudo journalctl -u sss-dashboard -f
sudo journalctl -u sss-agent -f

# 查看最近 100 行日志
sudo journalctl -u sss-dashboard -n 100
sudo journalctl -u sss-agent -n 100

# 查看今天的日志
sudo journalctl -u sss-dashboard --since today
sudo journalctl -u sss-agent --since today

# 查看最近 1 小时的日志
sudo journalctl -u sss-dashboard --since "1 hour ago"
sudo journalctl -u sss-agent --since "1 hour ago"

# 查看指定时间范围的日志
sudo journalctl -u sss-dashboard --since "2025-11-05 00:00:00" --until "2025-11-05 23:59:59"

# 导出日志到文件
sudo journalctl -u sss-dashboard > dashboard.log
sudo journalctl -u sss-agent > agent.log
```

## 使用非 root 用户运行

### 创建专用用户

```bash
# 创建系统用户
sudo useradd -r -s /bin/false sss

# 创建必要目录
sudo mkdir -p /etc/sss /var/log/sss

# 设置目录权限
sudo chown -R sss:sss /etc/sss /var/log/sss
sudo chmod 755 /etc/sss
sudo chmod 755 /var/log/sss

# 设置配置文件权限
sudo chown sss:sss /etc/sss/sss-dashboard.yaml
sudo chmod 600 /etc/sss/sss-dashboard.yaml
```

### 修改服务文件

```ini
[Service]
User=sss
Group=sss
# ... 其他配置保持不变
```

### 重新启动服务

```bash
sudo systemctl daemon-reload
sudo systemctl restart sss-dashboard
```

## 日志轮转

### 使用 logrotate

**创建配置文件** (`/etc/logrotate.d/sss`):

```bash
sudo nano /etc/logrotate.d/sss
```

**配置内容**:

```
/var/log/sss/*.log {
    daily
    rotate 7
    compress
    delaycompress
    missingok
    notifempty
    create 0640 sss sss
    sharedscripts
    postrotate
        systemctl reload sss-dashboard > /dev/null 2>&1 || true
        systemctl reload sss-agent > /dev/null 2>&1 || true
    endscript
}
```

**测试配置**:

```bash
sudo logrotate -d /etc/logrotate.d/sss
```

## 防火墙配置

### UFW (Ubuntu/Debian)

```bash
# 允许 Dashboard 端口
sudo ufw allow 8900/tcp

# 查看规则
sudo ufw status

# 启用防火墙
sudo ufw enable
```

### firewalld (CentOS/RHEL)

```bash
# 允许 Dashboard 端口
sudo firewall-cmd --permanent --add-port=8900/tcp

# 重新加载
sudo firewall-cmd --reload

# 查看规则
sudo firewall-cmd --list-all
```

### iptables

```bash
# 允许 Dashboard 端口
sudo iptables -A INPUT -p tcp --dport 8900 -j ACCEPT

# 保存规则
sudo iptables-save > /etc/iptables/rules.v4

# 或者（CentOS/RHEL）
sudo service iptables save
```

## 反向代理配置

如果需要配置 HTTPS 或使用域名访问，建议使用反向代理（Nginx 或 Caddy）。

详细配置请参考：[反向代理配置指南](proxy.md)

---

## 更新和维护

### 更新服务

```bash
# 1. 下载新版本二进制文件
wget https://github.com/ruanun/simple-server-status/releases/download/v1.1.0/sss-dashboard_v1.1.0_linux_amd64.tar.gz

# 2. 解压
tar -xzf sss-dashboard_v1.1.0_linux_amd64.tar.gz

# 3. 停止服务
sudo systemctl stop sss-dashboard

# 4. 备份旧版本
sudo cp /usr/local/bin/sss-dashboard /usr/local/bin/sss-dashboard.bak

# 5. 替换二进制文件
sudo mv sss-dashboard /usr/local/bin/
sudo chmod +x /usr/local/bin/sss-dashboard

# 6. 启动服务
sudo systemctl start sss-dashboard

# 7. 验证版本
/usr/local/bin/sss-dashboard --version

# 8. 查看日志
sudo journalctl -u sss-dashboard -f
```

### 备份配置

```bash
# 备份配置文件
sudo cp /etc/sss/sss-dashboard.yaml /etc/sss/sss-dashboard.yaml.backup

# 备份日志
sudo tar -czf sss-logs-$(date +%Y%m%d).tar.gz /var/log/sss/
```

### 回滚版本

```bash
# 停止服务
sudo systemctl stop sss-dashboard

# 恢复备份
sudo mv /usr/local/bin/sss-dashboard.bak /usr/local/bin/sss-dashboard

# 启动服务
sudo systemctl start sss-dashboard
```

## 监控和告警

### 使用 systemd 监控

**创建监控脚本** (`/usr/local/bin/sss-monitor.sh`):

```bash
#!/bin/bash

# 检查服务状态
if ! systemctl is-active --quiet sss-dashboard; then
    echo "Dashboard 服务已停止，尝试重启..."
    systemctl start sss-dashboard

    # 发送告警（示例：发送邮件）
    echo "Dashboard service was down and has been restarted" | \
        mail -s "SSS Dashboard Alert" admin@example.com
fi

if ! systemctl is-active --quiet sss-agent; then
    echo "Agent 服务已停止，尝试重启..."
    systemctl start sss-agent
fi
```

**创建 cron 任务**:

```bash
# 编辑 crontab
sudo crontab -e

# 每 5 分钟检查一次
*/5 * * * * /usr/local/bin/sss-monitor.sh >> /var/log/sss/monitor.log 2>&1
```

### 集成 Prometheus

**导出 systemd 指标**:

```bash
# 安装 node_exporter
wget https://github.com/prometheus/node_exporter/releases/download/v1.6.1/node_exporter-1.6.1.linux-amd64.tar.gz
tar -xzf node_exporter-1.6.1.linux-amd64.tar.gz
sudo mv node_exporter-1.6.1.linux-amd64/node_exporter /usr/local/bin/

# 创建 systemd 服务
sudo nano /etc/systemd/system/node_exporter.service
```

**node_exporter.service**:

```ini
[Unit]
Description=Prometheus Node Exporter
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/node_exporter --collector.systemd

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl start node_exporter
sudo systemctl enable node_exporter
```

## 故障排查

### 服务无法启动

**查看详细状态**:

```bash
sudo systemctl status sss-dashboard -l
```

**查看启动日志**:

```bash
sudo journalctl -u sss-dashboard -b
```

**常见问题**:

1. **配置文件格式错误**
   ```bash
   # 验证 YAML 格式
   python3 -c "import yaml; yaml.safe_load(open('/etc/sss/sss-dashboard.yaml'))"
   ```

2. **端口被占用**
   ```bash
   sudo netstat -tulpn | grep 8900
   # 或
   sudo lsof -i :8900
   ```

3. **权限问题**
   ```bash
   # 检查二进制文件权限
   ls -l /usr/local/bin/sss-dashboard

   # 检查配置文件权限
   ls -l /etc/sss/sss-dashboard.yaml

   # 检查日志目录权限
   ls -ld /var/log/sss
   ```

### 服务频繁重启

**查看重启次数**:

```bash
systemctl show sss-dashboard | grep NRestarts
```

**查看重启原因**:

```bash
sudo journalctl -u sss-dashboard | grep "Started\|Stopped"
```

### 性能问题

**查看资源使用**:

```bash
# CPU 和内存使用
systemctl status sss-dashboard sss-agent

# 详细资源统计
systemd-cgtop
```

## 生产环境建议

### 安全加固

1. **最小权限原则**: 使用非 root 用户运行
2. **配置文件权限**: 600 (只有所有者可读写)
3. **使用 HTTPS**: 配置反向代理启用 TLS
4. **防火墙规则**: 只开放必要端口
5. **定期更新**: 及时更新到最新版本

### 高可用配置

**使用 Keepalived 实现高可用**:

```bash
# 安装 Keepalived
sudo apt-get install keepalived

# 配置虚拟 IP
sudo nano /etc/keepalived/keepalived.conf
```

### 性能优化

**调整系统参数** (`/etc/sysctl.conf`):

```conf
# 增加文件描述符限制
fs.file-max = 65536

# 优化网络参数
net.core.somaxconn = 65535
net.ipv4.tcp_max_syn_backlog = 8192
```

```bash
# 应用配置
sudo sysctl -p
```

## 相关文档

- [Docker 部署](./docker.md) - Docker 容器化部署
- [开发环境搭建](../development/setup.md) - 本地开发
- [架构概览](../architecture/overview.md) - 系统架构

---

**版本**: 1.0
**作者**: ruan
**最后更新**: 2025-11-05
