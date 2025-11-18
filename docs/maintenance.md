# Simple Server Status 维护指南

> **作者**: ruan
> **最后更新**: 2025-11-15

本指南提供 Simple Server Status 的日常维护操作，包括更新、备份、迁移和卸载。

## 📋 目录

- [更新 Dashboard](#更新-dashboard)
- [更新 Agent](#更新-agent)
- [备份和恢复](#备份和恢复)
- [服务器迁移](#服务器迁移)
- [卸载 Dashboard](#卸载-dashboard)
- [卸载 Agent](#卸载-agent)

---

## 🔄 更新 Dashboard

### Docker 部署

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

# 4. 验证运行状态
docker ps | grep sssd
docker logs sssd

# 5. 检查版本
docker logs sssd | grep "版本\|version"
```

**使用 Docker Compose：**

```bash
# 1. 拉取最新镜像
docker-compose pull

# 2. 重新创建容器
docker-compose up -d

# 3. 验证
docker-compose ps
docker-compose logs -f dashboard
```

### 二进制部署

#### 自动更新（推荐）

如果有更新脚本，使用脚本更新：

```bash
# 下载更新脚本
wget https://raw.githubusercontent.com/ruanun/simple-server-status/main/scripts/update-dashboard.sh
chmod +x update-dashboard.sh

# 执行更新
sudo ./update-dashboard.sh
```

#### 手动更新

```bash
# 1. 下载最新版本
LATEST_VERSION=$(curl -s https://api.github.com/repos/ruanun/simple-server-status/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

wget https://github.com/ruanun/simple-server-status/releases/download/${LATEST_VERSION}/sss-dashboard_${LATEST_VERSION}_linux_amd64.tar.gz

# 2. 解压
tar -xzf sss-dashboard_${LATEST_VERSION}_linux_amd64.tar.gz

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

### 回滚到旧版本

如果新版本有问题，可以回滚到旧版本：

**Docker：**

```bash
# 使用指定版本的镜像
docker run --name sssd \
  --restart=unless-stopped \
  -d \
  -v ./sss-dashboard.yaml:/app/sss-dashboard.yaml \
  -p 8900:8900 \
  ruanun/sssd:v1.0.0  # 指定版本号
```

**二进制：**

```bash
# 使用备份的旧版本
sudo systemctl stop sss-dashboard
sudo mv /usr/local/bin/sss-dashboard.bak /usr/local/bin/sss-dashboard
sudo systemctl start sss-dashboard
```

---

## 📱 更新 Agent

### Linux / macOS / FreeBSD

**使用安装脚本更新（推荐）：**

```bash
# 重新运行安装脚本即可自动更新（会保留配置）
curl -fsSL https://raw.githubusercontent.com/ruanun/simple-server-status/main/scripts/install-agent.sh | sudo bash

# 验证版本
sss-agent --version

# 查看服务状态
sudo systemctl status sssa
```

**手动更新：**

```bash
# 1. 下载最新版本
LATEST_VERSION=$(curl -s https://api.github.com/repos/ruanun/simple-server-status/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

wget https://github.com/ruanun/simple-server-status/releases/download/${LATEST_VERSION}/sss-agent_${LATEST_VERSION}_linux_amd64.tar.gz

# 2. 解压
tar -xzf sss-agent_${LATEST_VERSION}_linux_amd64.tar.gz

# 3. 停止服务
sudo systemctl stop sssa

# 4. 备份旧版本
sudo cp /etc/sssa/sss-agent /etc/sssa/sss-agent.bak

# 5. 替换二进制文件
sudo mv sss-agent /etc/sssa/
sudo chmod +x /etc/sssa/sss-agent

# 6. 启动服务
sudo systemctl start sssa

# 7. 验证
sss-agent --version
sudo systemctl status sssa
```

### Windows

**使用安装脚本更新（推荐）：**

```powershell
# 以管理员身份运行 PowerShell
# 重新运行安装脚本
iwr -useb https://raw.githubusercontent.com/ruanun/simple-server-status/main/scripts/install-agent.ps1 | iex

# 验证版本
& "C:\Program Files\SSSA\sss-agent.exe" --version

# 查看服务状态
Get-Service -Name "SSSA"
```

**手动更新：**

```powershell
# 1. 停止服务
Stop-Service -Name "SSSA"

# 2. 下载最新版本
# 从 https://github.com/ruanun/simple-server-status/releases 下载对应版本

# 3. 备份旧版本
Copy-Item "C:\Program Files\SSSA\sss-agent.exe" "C:\Program Files\SSSA\sss-agent.exe.bak"

# 4. 解压并替换
Expand-Archive -Path "sss-agent_v1.x.x_windows_amd64.zip" -DestinationPath "$env:TEMP\sss-agent"
Copy-Item "$env:TEMP\sss-agent\sss-agent.exe" "C:\Program Files\SSSA\sss-agent.exe"

# 5. 启动服务
Start-Service -Name "SSSA"

# 6. 验证
& "C:\Program Files\SSSA\sss-agent.exe" --version
Get-Service -Name "SSSA"
```

### 批量更新多台 Agent

**Unix 批量更新脚本：**

```bash
#!/bin/bash
# batch-update.sh

# 服务器列表
SERVERS=(
    "192.168.1.10"
    "192.168.1.11"
    "192.168.1.12"
)

for server in "${SERVERS[@]}"; do
    echo "更新服务器: $server"

    ssh root@$server "curl -fsSL https://raw.githubusercontent.com/ruanun/simple-server-status/main/scripts/install-agent.sh | bash"

    if [ $? -eq 0 ]; then
        echo "✅ 服务器 $server 更新成功"
    else
        echo "❌ 服务器 $server 更新失败"
    fi
done
```

**使用 Ansible：**

```yaml
# update-agents.yml
---
- name: 更新 Simple Server Status Agent
  hosts: all
  become: yes
  tasks:
    - name: 下载安装脚本
      get_url:
        url: https://raw.githubusercontent.com/ruanun/simple-server-status/main/scripts/install-agent.sh
        dest: /tmp/install-agent.sh
        mode: '0755'

    - name: 运行安装脚本
      shell: /tmp/install-agent.sh

    - name: 验证服务状态
      systemd:
        name: sssa
        state: started
        enabled: yes

    - name: 检查版本
      command: sss-agent --version
      register: version_output

    - name: 显示版本
      debug:
        msg: "{{ version_output.stdout }}"
```

运行：

```bash
ansible-playbook -i inventory update-agents.yml
```

---

## 💾 备份和恢复

### 备份 Dashboard 配置

**Docker 部署：**

```bash
# 备份配置文件
cp sss-dashboard.yaml sss-dashboard.yaml.backup.$(date +%Y%m%d)

# 备份日志（如果使用 volume）
docker cp sssd:/app/.logs ./logs-backup-$(date +%Y%m%d)

# 压缩备份
tar -czf dashboard-backup-$(date +%Y%m%d).tar.gz sss-dashboard.yaml logs-backup-*
```

**二进制部署：**

```bash
# 创建备份目录
sudo mkdir -p /backup/sss

# 备份配置文件
sudo cp /etc/sss/sss-dashboard.yaml /backup/sss/sss-dashboard.yaml.$(date +%Y%m%d)

# 备份日志
sudo cp -r /var/log/sss /backup/sss/logs-$(date +%Y%m%d)

# 压缩备份
sudo tar -czf /backup/sss-dashboard-backup-$(date +%Y%m%d).tar.gz /backup/sss/
```

### 备份 Agent 配置

```bash
# Linux/macOS
sudo cp /etc/sssa/sss-agent.yaml /etc/sssa/sss-agent.yaml.backup.$(date +%Y%m%d)

# Windows
Copy-Item "C:\Program Files\SSSA\sss-agent.yaml" "C:\Program Files\SSSA\sss-agent.yaml.backup.$(Get-Date -Format 'yyyyMMdd')"
```

### 恢复配置

**Dashboard：**

```bash
# 停止服务
docker stop sssd  # 或 sudo systemctl stop sss-dashboard

# 恢复配置文件
cp sss-dashboard.yaml.backup.20250115 sss-dashboard.yaml

# 启动服务
docker start sssd  # 或 sudo systemctl start sss-dashboard
```

**Agent：**

```bash
# 停止服务
sudo systemctl stop sssa

# 恢复配置文件
sudo cp /etc/sssa/sss-agent.yaml.backup.20250115 /etc/sssa/sss-agent.yaml

# 启动服务
sudo systemctl start sssa
```

### 自动化备份

**使用 cron 定时备份（Linux）：**

```bash
# 编辑 crontab
sudo crontab -e

# 每天凌晨 2 点备份
0 2 * * * tar -czf /backup/sss-dashboard-$(date +\%Y\%m\%d).tar.gz /etc/sss/sss-dashboard.yaml /var/log/sss

# 保留最近 30 天的备份
0 3 * * * find /backup -name "sss-dashboard-*.tar.gz" -mtime +30 -delete
```

---

## 🔄 服务器迁移

### 迁移 Dashboard

#### 迁移到新服务器

**1. 在旧服务器上备份：**

```bash
# 导出配置文件
docker cp sssd:/app/sss-dashboard.yaml ./sss-dashboard.yaml

# 或 systemd 部署
sudo cp /etc/sss/sss-dashboard.yaml ./sss-dashboard.yaml

# 备份日志（可选）
docker cp sssd:/app/.logs ./logs
```

**2. 在新服务器上恢复：**

```bash
# 上传配置文件到新服务器
scp sss-dashboard.yaml user@new-server:~/

# 在新服务器上启动 Dashboard
ssh user@new-server
docker run --name sssd \
  --restart=unless-stopped \
  -d \
  -v ./sss-dashboard.yaml:/app/sss-dashboard.yaml \
  -p 8900:8900 \
  ruanun/sssd
```

**3. 更新所有 Agent 配置：**

```bash
# 在每台 Agent 服务器上修改配置
sudo nano /etc/sssa/sss-agent.yaml

# 修改 serverAddr
serverAddr: ws://NEW-DASHBOARD-IP:8900/ws-report

# 重启 Agent
sudo systemctl restart sssa
```

### 迁移 Agent

迁移到新服务器时，只需在新服务器上重新安装并使用相同的配置即可。

**重要提示：** `serverId` 必须保持不变，否则会在 Dashboard 上显示为新服务器。

---

## 🗑️ 卸载 Dashboard

### Docker 部署

```bash
# 停止并删除容器
docker stop sssd
docker rm sssd

# 删除镜像（可选）
docker rmi ruanun/sssd

# 删除配置文件和日志（可选）
rm sss-dashboard.yaml
rm -rf logs
```

### 二进制部署

#### Linux

```bash
# 停止并删除服务
sudo systemctl stop sss-dashboard
sudo systemctl disable sss-dashboard
sudo rm /etc/systemd/system/sss-dashboard.service
sudo systemctl daemon-reload

# 删除二进制文件
sudo rm /usr/local/bin/sss-dashboard

# 删除配置文件和日志
sudo rm -rf /etc/sss
sudo rm -rf /var/log/sss
```

#### macOS

```bash
# 停止服务
launchctl stop com.simple-server-status.dashboard
launchctl unload ~/Library/LaunchAgents/com.simple-server-status.dashboard.plist

# 删除文件
rm ~/Library/LaunchAgents/com.simple-server-status.dashboard.plist
sudo rm /usr/local/bin/sss-dashboard
sudo rm -rf /etc/sss
```

---

## 🗑️ 卸载 Agent

### 使用脚本卸载

#### Linux / macOS / FreeBSD

```bash
# 在线卸载
curl -fsSL https://raw.githubusercontent.com/ruanun/simple-server-status/main/scripts/install-agent.sh | sudo bash -s -- --uninstall

# 或下载脚本后卸载
wget https://raw.githubusercontent.com/ruanun/simple-server-status/main/scripts/install-agent.sh
chmod +x install-agent.sh
sudo ./install-agent.sh --uninstall
```

#### Windows

```powershell
# 以管理员身份运行 PowerShell
iwr -useb https://raw.githubusercontent.com/ruanun/simple-server-status/main/scripts/install-agent.ps1 | iex -ArgumentList "-Uninstall"

# 或下载脚本后卸载
.\install-agent.ps1 -Uninstall
```

### 手动卸载

#### Linux

```bash
# 停止并删除服务
sudo systemctl stop sssa
sudo systemctl disable sssa
sudo rm /etc/systemd/system/sssa.service
sudo systemctl daemon-reload

# 删除文件
sudo rm -rf /etc/sssa
sudo rm /usr/local/bin/sss-agent

# 删除日志（可选）
sudo rm -rf /var/log/sssa
```

#### macOS

```bash
# 停止服务
launchctl stop com.simple-server-status.agent
launchctl unload ~/Library/LaunchAgents/com.simple-server-status.agent.plist

# 删除文件
rm ~/Library/LaunchAgents/com.simple-server-status.agent.plist
sudo rm -rf /etc/sssa
sudo rm /usr/local/bin/sss-agent
```

#### FreeBSD

```bash
# 停止服务
sudo service sssa stop
sudo sysrc sssa_enable="NO"

# 删除文件
sudo rm /usr/local/etc/rc.d/sssa
sudo rm -rf /etc/sssa
sudo rm /usr/local/bin/sss-agent
```

#### Windows

```powershell
# 停止并删除服务
Stop-Service -Name "SSSA"
sc.exe delete "SSSA"

# 删除文件
Remove-Item "C:\Program Files\SSSA" -Recurse -Force

# 从 PATH 中移除（手动操作）
# 1. 右键"此电脑" -> 属性 -> 高级系统设置
# 2. 环境变量 -> 系统变量 -> Path
# 3. 删除 C:\Program Files\SSSA
```

### 批量卸载多台 Agent

```bash
#!/bin/bash
# batch-uninstall.sh

SERVERS=(
    "192.168.1.10"
    "192.168.1.11"
    "192.168.1.12"
)

for server in "${SERVERS[@]}"; do
    echo "卸载服务器: $server"

    ssh root@$server "curl -fsSL https://raw.githubusercontent.com/ruanun/simple-server-status/main/scripts/install-agent.sh | bash -s -- --uninstall"

    if [ $? -eq 0 ]; then
        echo "✅ 服务器 $server 卸载成功"
    else
        echo "❌ 服务器 $server 卸载失败"
    fi
done
```

---

## 📚 相关文档

- 📖 [快速开始指南](getting-started.md) - 部署和配置
- 🐛 [故障排除指南](troubleshooting.md) - 常见问题解决
- 🐳 [Docker 部署](deployment/docker.md) - Docker 详细配置
- ⚙️ [systemd 部署](deployment/systemd.md) - systemd 服务配置

---

**版本**: 1.0
**作者**: ruan
**最后更新**: 2025-11-15
