# 快速开始

从零部署一个 Dashboard 并接入第一台服务器。更多部署方式见 [部署](deployment.md)，参数说明见 [配置](configuration.md)。

## 1. 启动 Dashboard

```bash
docker run -d --name sss-dashboard --restart unless-stopped \
  -p 8900:8900 -v sss-data:/app/data ruanun/sssd:latest
```

浏览器打开 `http://<服务器IP>:8900` 即可看到状态页。

## 2. 获取管理员密码

首次启动时会创建管理员账户 `admin`，随机密码只在日志中打印一次：

```bash
docker logs sss-dashboard 2>&1 | grep password
```

也可以在首次启动时通过环境变量预设密码（只在数据库中还没有用户时生效）：

```bash
docker run -d --name sss-dashboard --restart unless-stopped \
  -p 8900:8900 -v sss-data:/app/data \
  -e SSS_ADMIN_PASSWORD='change-me-please' ruanun/sssd:latest
```

登录后可在「设置」中修改密码（至少 8 位）。

## 3. 添加服务器并安装 Agent

1. 打开 `http://<服务器IP>:8900/login`，用 `admin` 登录，进入后台。
2. 在「服务器」页点击「新建服务器」，填写名称后保存。保存后会自动弹出安装命令；之后也可以在行操作的「安装命令」中随时查看。
3. 确认弹窗中的「面板地址」是被监控机器能访问到的 Dashboard 地址，然后复制对应系统的命令，到目标机器上执行（以下为示意，实际以弹窗为准；正式发布版本的命令会锁定为与 Dashboard 相同的版本）：
   - Linux（需要 root 与 systemd）：

     ```bash
     curl -fsSL 'https://github.com/ruanun/simple-server-status/releases/latest/download/install-agent.sh' | sudo bash -s -- --dashboard 'http://<面板地址>' --id '<ID>' --secret '<密钥>'
     ```

   - Windows（管理员 PowerShell）：

     ```powershell
     Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass -Force; $f="$env:TEMP\install-agent.ps1"; iwr -useb 'https://github.com/ruanun/simple-server-status/releases/latest/download/install-agent.ps1' -OutFile $f; & $f -Dashboard 'http://<面板地址>' -Id '<ID>' -Secret '<密钥>'
     ```

   以上仅为格式示例，请直接复制后台生成的命令，其中已填好 ID 与密钥。

## 4. 查看状态

回到首页，几秒内即可看到该服务器上线。点击卡片进入详情页查看历史趋势。

如果一直显示离线，在目标机器上查看 Agent 日志：

- Linux：`journalctl -u sss-agent -f`
- Windows：`Get-Content "$env:ProgramFiles\sss-agent\logs\agent.log" -Tail 50`

日志中出现 401 表示 ID 或密钥不对（例如密钥已在后台重置），请在后台重新复制安装命令执行。

## 5. 忘记密码

用 `reset-password` 子命令为 `admin` 生成新的随机密码，已登录的会话同时失效：

```bash
# Docker
docker exec sss-dashboard /app/sss-dashboard reset-password

# 二进制部署（以运行服务的用户执行，--data-dir 与运行时一致）
sudo -u sss /usr/local/bin/sss-dashboard reset-password --data-dir /var/lib/sss
```
