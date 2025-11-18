# Simple Server Status 反向代理配置指南

> **作者**: ruan
> **最后更新**: 2025-11-15

本指南提供使用反向代理（Nginx、Caddy、Apache、Traefik）配置 HTTPS 访问的详细步骤。

**使用反向代理的好处：**
- ✅ 自动 HTTPS 证书（Let's Encrypt）
- ✅ 域名访问更友好
- ✅ 统一管理多个服务
- ✅ 负载均衡和高可用
- ✅ 访问控制和安全加固

## 📋 目录

- [Nginx 配置](#nginx-配置)
- [Caddy 配置](#caddy-配置)
- [Apache 配置](#apache-配置)
- [Traefik 配置](#traefik-配置)
- [SSL 证书配置](#ssl-证书配置)
- [WebSocket 路径配置](#websocket-路径配置)
- [Agent 配置更新](#agent-配置更新)

---

## 🔧 Nginx 配置

### 基础 HTTP 代理

#### 安装 Nginx

```bash
# Ubuntu/Debian
sudo apt update && sudo apt install nginx -y

# CentOS/RHEL
sudo yum install nginx -y

# macOS
brew install nginx
```

#### 配置文件

创建配置文件 `/etc/nginx/sites-available/sss` 或 `/etc/nginx/conf.d/sss.conf`：

```nginx
upstream sssd {
    server 127.0.0.1:8900;
}

# WebSocket 升级映射
map $http_upgrade $connection_upgrade {
    default upgrade;
    '' close;
}

server {
    listen 80;
    server_name status.example.com;  # 替换为你的域名

    # 主站点代理
    location / {
        proxy_pass http://sssd;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # WebSocket 代理（路径需与 Dashboard 的 webSocketPath 一致）
    location /ws-report {
        proxy_pass http://sssd;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;
        proxy_read_timeout 300s;
        proxy_send_timeout 300s;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Host $host:$server_port;
        proxy_set_header X-Forwarded-Server $host;
        proxy_set_header X-Forwarded-Port $server_port;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # 前端 WebSocket (如果需要)
    location /ws-frontend {
        proxy_pass http://sssd;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;
        proxy_read_timeout 86400;
        proxy_send_timeout 86400;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

#### 启用配置

```bash
# 如果使用 sites-available/sites-enabled 结构
sudo ln -s /etc/nginx/sites-available/sss /etc/nginx/sites-enabled/

# 测试配置
sudo nginx -t

# 重新加载
sudo systemctl reload nginx
```

### HTTPS 配置

#### 使用 Let's Encrypt（推荐）

```bash
# 安装 certbot
# Ubuntu/Debian
sudo apt install certbot python3-certbot-nginx -y

# CentOS/RHEL
sudo yum install certbot python3-certbot-nginx -y

# 申请证书并自动配置 Nginx
sudo certbot --nginx -d status.example.com

# certbot 会自动：
# 1. 申请 Let's Encrypt 证书
# 2. 修改 Nginx 配置启用 HTTPS
# 3. 配置自动续期

# 手动续期（自动续期已配置好，一般不需要手动执行）
sudo certbot renew
```

#### 手动 HTTPS 配置

```nginx
server {
    listen 443 ssl http2;
    server_name status.example.com;

    # SSL 证书配置
    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    # SSL 安全配置（推荐）
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers 'ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256';
    ssl_prefer_server_ciphers on;
    ssl_session_cache shared:SSL:10m;
    ssl_session_timeout 10m;

    # HSTS（可选，强制 HTTPS）
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;

    # 其他安全头
    add_header X-Frame-Options "DENY" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;

    location / {
        proxy_pass http://127.0.0.1:8900;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location /ws-report {
        proxy_pass http://127.0.0.1:8900;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_read_timeout 86400;
    }

    location /ws-frontend {
        proxy_pass http://127.0.0.1:8900;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_read_timeout 86400;
    }
}

# HTTP 重定向到 HTTPS
server {
    listen 80;
    server_name status.example.com;
    return 301 https://$server_name$request_uri;
}
```

### 高级配置

#### 访问控制

```nginx
server {
    listen 443 ssl http2;
    server_name status.example.com;

    # ... SSL 配置 ...

    # 基于 IP 的访问控制
    allow 192.168.1.0/24;  # 允许内网访问
    allow 203.0.113.0/24;  # 允许特定 IP 段
    deny all;               # 拒绝其他所有 IP

    # 或使用 Basic Auth
    auth_basic "Restricted Access";
    auth_basic_user_file /etc/nginx/.htpasswd;

    location / {
        proxy_pass http://127.0.0.1:8900;
        # ...
    }
}
```

创建 Basic Auth 用户：

```bash
# 安装 htpasswd
sudo apt install apache2-utils -y

# 创建用户
sudo htpasswd -c /etc/nginx/.htpasswd admin
# 输入密码
```

#### 请求限速

```nginx
# 在 http 块中配置
http {
    # 限制请求速率：每个 IP 每秒最多 10 个请求
    limit_req_zone $binary_remote_addr zone=sss_limit:10m rate=10r/s;

    server {
        # ...

        location / {
            limit_req zone=sss_limit burst=20 nodelay;
            proxy_pass http://127.0.0.1:8900;
            # ...
        }
    }
}
```

---

## 🚀 Caddy 配置

Caddy 是现代化的 Web 服务器，自动配置 HTTPS 证书，配置极其简单。

### 安装 Caddy

```bash
# Ubuntu/Debian
sudo apt install -y debian-keyring debian-archive-keyring apt-transport-https
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list
sudo apt update
sudo apt install caddy

# macOS
brew install caddy

# 或使用安装脚本
curl https://getcaddy.com | bash -s personal
```

### 基础配置（自动 HTTPS）

编辑 `/etc/caddy/Caddyfile`：

```caddyfile
status.example.com {
    # 自动申请 Let's Encrypt 证书并配置 HTTPS
    reverse_proxy localhost:8900

    # 可选：启用压缩
    encode gzip

    # 可选：访问日志
    log {
        output file /var/log/caddy/sssd.log
    }
}
```

**就这么简单！** Caddy 会自动：
- 申请 Let's Encrypt 证书
- 配置 HTTPS
- 配置 HTTP 到 HTTPS 重定向
- 处理 WebSocket 连接
- 自动续期证书

### 高级配置

#### 自定义 TLS 配置

```caddyfile
status.example.com {
    reverse_proxy localhost:8900

    # 自定义 TLS 配置
    tls {
        protocols tls1.2 tls1.3
        ciphers TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256
    }

    # 安全头
    header {
        # 启用 HSTS
        Strict-Transport-Security "max-age=31536000; includeSubDomains; preload"
        # 防止点击劫持
        X-Frame-Options "DENY"
        # 防止 MIME 类型嗅探
        X-Content-Type-Options "nosniff"
        # XSS 保护
        X-XSS-Protection "1; mode=block"
    }

    # 启用 gzip 压缩
    encode gzip
}
```

#### 访问控制

```caddyfile
status.example.com {
    # IP 白名单
    @allowed {
        remote_ip 192.168.1.0/24 203.0.113.0/24
    }
    handle @allowed {
        reverse_proxy localhost:8900
    }
    handle {
        abort
    }

    # 或使用 Basic Auth
    basicauth {
        admin $2a$14$Zkx19XLiW6VYouLHR5NmfOFU0z2GTNmpkT/5qqR7hx7wNQIWxTR.e
    }

    reverse_proxy localhost:8900
}
```

生成 Basic Auth 密码：

```bash
caddy hash-password
# 输入密码，获得加密后的哈希值
```

#### 使用自签名证书（开发环境）

```caddyfile
status.example.com {
    tls internal  # 使用 Caddy 内置的自签名证书

    reverse_proxy localhost:8900
}
```

### 启动 Caddy

```bash
# 测试配置
sudo caddy validate --config /etc/caddy/Caddyfile

# 启动服务
sudo systemctl start caddy
sudo systemctl enable caddy

# 查看状态
sudo systemctl status caddy

# 查看日志
sudo journalctl -u caddy -f
```

---

## 🌐 Apache 配置

### 安装 Apache

```bash
# Ubuntu/Debian
sudo apt install apache2 -y

# CentOS/RHEL
sudo yum install httpd -y
```

### 启用必要模块

```bash
# 启用代理和 WebSocket 模块
sudo a2enmod proxy proxy_http proxy_wstunnel ssl rewrite headers

# 重启 Apache
sudo systemctl restart apache2
```

### 配置文件

创建 `/etc/apache2/sites-available/sss.conf`：

```apache
<VirtualHost *:80>
    ServerName status.example.com

    # 代理到 Dashboard
    ProxyPreserveHost On
    ProxyPass / http://127.0.0.1:8900/
    ProxyPassReverse / http://127.0.0.1:8900/

    # WebSocket 支持
    RewriteEngine On
    RewriteCond %{HTTP:Upgrade} websocket [NC]
    RewriteCond %{HTTP:Connection} upgrade [NC]
    RewriteRule ^/?(.*) "ws://127.0.0.1:8900/$1" [P,L]

    # 日志
    ErrorLog ${APACHE_LOG_DIR}/sss_error.log
    CustomLog ${APACHE_LOG_DIR}/sss_access.log combined
</VirtualHost>
```

### HTTPS 配置

```apache
<VirtualHost *:443>
    ServerName status.example.com

    # SSL 证书
    SSLEngine on
    SSLCertificateFile /path/to/cert.pem
    SSLCertificateKeyFile /path/to/key.pem

    # SSL 安全配置
    SSLProtocol all -SSLv2 -SSLv3 -TLSv1 -TLSv1.1
    SSLCipherSuite HIGH:!aNULL:!MD5

    # 安全头
    Header always set Strict-Transport-Security "max-age=31536000; includeSubDomains"
    Header always set X-Frame-Options "DENY"
    Header always set X-Content-Type-Options "nosniff"

    # 代理配置
    ProxyPreserveHost On
    ProxyPass / http://127.0.0.1:8900/
    ProxyPassReverse / http://127.0.0.1:8900/

    # WebSocket 支持
    RewriteEngine On
    RewriteCond %{HTTP:Upgrade} websocket [NC]
    RewriteCond %{HTTP:Connection} upgrade [NC]
    RewriteRule ^/?(.*) "ws://127.0.0.1:8900/$1" [P,L]
</VirtualHost>

# HTTP 重定向到 HTTPS
<VirtualHost *:80>
    ServerName status.example.com
    Redirect permanent / https://status.example.com/
</VirtualHost>
```

### 启用配置

```bash
# 启用站点
sudo a2ensite sss

# 测试配置
sudo apachectl configtest

# 重新加载
sudo systemctl reload apache2
```

---

## 🐋 Traefik 配置

Traefik 是云原生的反向代理，特别适合 Docker 和 Kubernetes 环境。

### Docker Compose 配置

`docker-compose.yml`：

```yaml
version: '3.8'

services:
  traefik:
    image: traefik:latest
    command:
      - "--api.insecure=true"
      - "--providers.docker=true"
      - "--entrypoints.web.address=:80"
      - "--entrypoints.websecure.address=:443"
      - "--certificatesresolvers.myresolver.acme.tlschallenge=true"
      - "--certificatesresolvers.myresolver.acme.email=your-email@example.com"
      - "--certificatesresolvers.myresolver.acme.storage=/letsencrypt/acme.json"
    ports:
      - "80:80"
      - "443:443"
      - "8080:8080"  # Traefik Dashboard
    volumes:
      - "/var/run/docker.sock:/var/run/docker.sock:ro"
      - "./letsencrypt:/letsencrypt"
    networks:
      - web

  dashboard:
    image: ruanun/sssd:latest
    volumes:
      - ./sss-dashboard.yaml:/app/sss-dashboard.yaml
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.sss.rule=Host(`status.example.com`)"
      - "traefik.http.routers.sss.entrypoints=websecure"
      - "traefik.http.routers.sss.tls.certresolver=myresolver"
      - "traefik.http.services.sss.loadbalancer.server.port=8900"
    networks:
      - web

networks:
  web:
    driver: bridge
```

---

## 🔐 SSL 证书配置

### Let's Encrypt（推荐）

**免费、自动化、受信任**

#### 使用 Certbot（独立模式）

```bash
# 安装 certbot
sudo apt install certbot -y

# 申请证书（需要停止 Dashboard 或反向代理）
sudo certbot certonly --standalone -d status.example.com

# 证书路径：
# /etc/letsencrypt/live/status.example.com/fullchain.pem
# /etc/letsencrypt/live/status.example.com/privkey.pem

# 自动续期（已自动配置）
sudo certbot renew --dry-run
```

#### 使用 acme.sh

```bash
# 安装 acme.sh
curl https://get.acme.sh | sh

# 申请证书
~/.acme.sh/acme.sh --issue -d status.example.com --webroot /var/www/html

# 安装证书
~/.acme.sh/acme.sh --install-cert -d status.example.com \
  --key-file /etc/ssl/private/status.example.com.key \
  --fullchain-file /etc/ssl/certs/status.example.com.crt \
  --reloadcmd "sudo systemctl reload nginx"
```

### 自签名证书（开发环境）

```bash
# 生成自签名证书
sudo openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout /etc/ssl/private/selfsigned.key \
  -out /etc/ssl/certs/selfsigned.crt

# 在配置中使用
# ssl_certificate /etc/ssl/certs/selfsigned.crt;
# ssl_certificate_key /etc/ssl/private/selfsigned.key;
```

---

## 🔄 WebSocket 路径配置

### 默认路径

Dashboard 默认使用 `/ws-report` 作为 Agent 上报的 WebSocket 路径。

### 自定义路径

#### 1. 修改 Dashboard 配置

```yaml
# Dashboard 配置 (sss-dashboard.yaml)
webSocketPath: /custom-path  # 自定义路径，必须以 '/' 开头
```

#### 2. 修改反向代理配置

**Nginx：**

```nginx
location /custom-path {
    proxy_pass http://127.0.0.1:8900;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    # ...
}
```

**Caddy：**

Caddy 自动处理 WebSocket，无需特殊配置。

#### 3. 修改所有 Agent 配置

```yaml
# Agent 配置 (sss-agent.yaml)
serverAddr: ws://status.example.com/custom-path  # 或 wss://
```

#### 4. 重启所有服务

```bash
# Dashboard
docker restart sssd  # 或 sudo systemctl restart sss-dashboard

# 反向代理
sudo systemctl reload nginx  # 或 caddy

# 所有 Agent
sudo systemctl restart sssa
```

---

## 🔐 Agent 配置更新

配置反向代理后，需要更新所有 Agent 的 `serverAddr`。

### HTTP → HTTPS

**原配置：**

```yaml
serverAddr: ws://192.168.1.100:8900/ws-report
```

**新配置：**

```yaml
serverAddr: wss://status.example.com/ws-report  # 注意使用 wss://
```

### 批量更新 Agent

**脚本示例：**

```bash
#!/bin/bash
# update-agent-url.sh

SERVERS=(
    "192.168.1.10"
    "192.168.1.11"
    "192.168.1.12"
)

NEW_URL="wss://status.example.com/ws-report"

for server in "${SERVERS[@]}"; do
    echo "更新服务器: $server"

    ssh root@$server "sed -i 's|serverAddr:.*|serverAddr: $NEW_URL|' /etc/sssa/sss-agent.yaml"
    ssh root@$server "systemctl restart sssa"

    echo "✅ 服务器 $server 更新完成"
done
```

---

## ✅ 验证配置

### 测试 HTTPS

```bash
# 测试 HTTPS 连接
curl -I https://status.example.com

# 测试 WebSocket (需要 websocat 或类似工具)
websocat wss://status.example.com/ws-report

# 检查证书
openssl s_client -connect status.example.com:443 -servername status.example.com
```

### 测试 Agent 连接

```bash
# 查看 Agent 日志
sudo journalctl -u sssa -f

# 应该看到 "连接成功" 或 "WebSocket connected"
```

### 浏览器访问

访问 `https://status.example.com`，检查：
- ✅ HTTPS 证书有效（绿色锁图标）
- ✅ 页面正常加载
- ✅ 服务器数据实时更新

---

## 📚 相关文档

- 📖 [快速开始指南](../getting-started.md) - 基本部署
- 🐳 [Docker 部署](docker.md) - Docker 容器化部署
- ⚙️ [systemd 部署](systemd.md) - systemd 服务配置
- 🐛 [故障排除](../troubleshooting.md) - 常见问题解决

---

**版本**: 1.0
**作者**: ruan
**最后更新**: 2025-11-15
