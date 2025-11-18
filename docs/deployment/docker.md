# Docker 部署指南

> **作者**: ruan
> **最后更新**: 2025-11-05

## 概述

SimpleServerStatus 提供 Docker 镜像，支持快速部署和容器化运行。本文档介绍如何使用 Docker 和 Docker Compose 部署项目。

## 前置要求

### 必需软件

- **Docker**: 20.10+ 或更高版本
- **Docker Compose**: 2.0+ 或更高版本（可选）

### 安装 Docker

**Ubuntu/Debian**:
```bash
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER
```

**CentOS/RHEL**:
```bash
curl -fsSL https://get.docker.com | sh
sudo systemctl start docker
sudo systemctl enable docker
sudo usermod -aG docker $USER
```

**Windows/macOS**:
下载并安装 [Docker Desktop](https://www.docker.com/products/docker-desktop)

## 快速开始

### Dashboard 快速部署

```bash
# 1. 下载配置文件模板
wget https://raw.githubusercontent.com/ruanun/simple-server-status/main/configs/sss-dashboard.yaml.example -O sss-dashboard.yaml

# 2. 编辑配置文件
nano sss-dashboard.yaml

# 3. 运行 Dashboard
docker run -d \
  --name sss-dashboard \
  --restart=unless-stopped \
  -p 8900:8900 \
  -v $(pwd)/sss-dashboard.yaml:/app/sss-dashboard.yaml:ro \
  ruanun/sssd:latest

# 4. 查看日志
docker logs -f sss-dashboard

# 5. 访问 Dashboard
# 浏览器打开 http://localhost:8900
```

### Agent 快速部署

```bash
# 1. 下载配置文件模板
wget https://raw.githubusercontent.com/ruanun/simple-server-status/main/configs/sss-agent.yaml.example -O sss-agent.yaml

# 2. 编辑配置文件（填入 Dashboard 地址和认证信息）
nano sss-agent.yaml

# 3. 运行 Agent
docker run -d \
  --name sss-agent \
  --restart=unless-stopped \
  -v $(pwd)/sss-agent.yaml:/app/sss-agent.yaml:ro \
  ruanun/sss-agent:latest

# 4. 查看日志
docker logs -f sss-agent
```

## 使用 Docker Compose

### 方式 1: 仅部署 Dashboard

**docker-compose.yml**:

```yaml
version: '3.8'

services:
  dashboard:
    image: ruanun/sssd:latest
    container_name: sss-dashboard
    ports:
      - "8900:8900"
    volumes:
      - ./sss-dashboard.yaml:/app/sss-dashboard.yaml:ro
      - ./logs:/app/logs
    environment:
      - TZ=Asia/Shanghai
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "--quiet", "--tries=1", "--spider", "http://localhost:8900/api/statistics"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s
```

**启动服务**:

```bash
# 启动
docker-compose up -d

# 查看日志
docker-compose logs -f

# 停止
docker-compose down
```

### 方式 2: Dashboard + 反向代理（HTTPS）

如需配置 HTTPS 和反向代理，请参考：

- 📘 **[反向代理配置指南](proxy.md)** - Nginx/Caddy/Apache 完整配置和 SSL 证书
- 📂 **[部署配置示例](../../deployments/docker/docker-compose.yml)** - 包含 Caddy 的 Docker Compose 配置

**快速示例（Caddy）：**

```bash
# 使用 deployments/docker 中的配置
cd deployments/docker

# 准备 Caddyfile
cp ../caddy/Caddyfile ./Caddyfile
nano Caddyfile  # 修改域名

# 启动（包含 Caddy）
docker-compose --profile with-caddy up -d
```

详细的反向代理配置（包括 Nginx、Apache、Traefik 等）请参考 [proxy.md](proxy.md)

## 构建自定义镜像

### 从源码构建 Dashboard

**Dockerfile.dashboard** (已包含在 `deployments/docker/`):

```dockerfile
# 构建前端
FROM node:18-alpine AS frontend-builder
WORKDIR /web
COPY web/package*.json ./
RUN corepack enable && corepack prepare pnpm@latest --activate
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm run build:prod

# 构建后端
FROM golang:1.23-alpine AS backend-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend-builder /web/dist ./internal/dashboard/public/dist
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o sss-dashboard ./cmd/dashboard

# 运行时镜像
FROM alpine:latest
ARG TZ="Asia/Shanghai"
ENV TZ=${TZ}

RUN apk --no-cache add ca-certificates tzdata bash && \
    ln -sf /usr/share/zoneinfo/${TZ} /etc/localtime && \
    echo ${TZ} > /etc/timezone

WORKDIR /app
COPY --from=backend-builder /app/sss-dashboard .

EXPOSE 8900
CMD ["./sss-dashboard"]
```

**构建命令**:

```bash
# 在项目根目录执行
docker build -f deployments/docker/Dockerfile.dashboard -t ruanun/sssd:latest .
```

### 从源码构建 Agent

**Dockerfile.agent**:

```dockerfile
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o sss-agent ./cmd/agent

FROM alpine:latest
ARG TZ="Asia/Shanghai"
ENV TZ=${TZ}

RUN apk --no-cache add ca-certificates tzdata bash && \
    ln -sf /usr/share/zoneinfo/${TZ} /etc/localtime && \
    echo ${TZ} > /etc/timezone

WORKDIR /app
COPY --from=builder /app/sss-agent .

CMD ["./sss-agent"]
```

**构建命令**:

```bash
docker build -f deployments/docker/Dockerfile.agent -t ruanun/sss-agent:latest .
```

## 配置说明

### Dashboard 配置

**sss-dashboard.yaml**:

```yaml
# HTTP 服务配置
port: 8900
address: 0.0.0.0
webSocketPath: ws-report

# 授权的 Agent 列表
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

# 日志配置（可选）
logLevel: info
logPath: logs/dashboard.log
```

### Agent 配置

**sss-agent.yaml**:

```yaml
# Dashboard 地址（WebSocket）
serverAddr: ws://dashboard-host:8900/ws-report

# 服务器标识（必须与 Dashboard 配置匹配）
serverId: web-1

# 认证密钥（必须与 Dashboard 配置匹配）
authSecret: "your-secret-key-1"

# 日志配置（可选）
logLevel: info
logPath: logs/agent.log
```

## 环境变量

### Dashboard 环境变量

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `TZ` | 时区 | `UTC` |
| `CONFIG` | 配置文件路径 | `sss-dashboard.yaml` |

**使用环境变量**:

```bash
docker run -d \
  --name sss-dashboard \
  -e TZ=Asia/Shanghai \
  -e CONFIG=/app/config/dashboard.yaml \
  -v $(pwd)/dashboard.yaml:/app/config/dashboard.yaml:ro \
  -p 8900:8900 \
  ruanun/sssd:latest
```

### Agent 环境变量

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `TZ` | 时区 | `UTC` |
| `CONFIG` | 配置文件路径 | `sss-agent.yaml` |

## 数据持久化

### 日志持久化

**Dashboard**:

```bash
docker run -d \
  --name sss-dashboard \
  -v $(pwd)/logs:/app/logs \
  -p 8900:8900 \
  ruanun/sssd:latest
```

**Agent**:

```bash
docker run -d \
  --name sss-agent \
  -v $(pwd)/logs:/app/logs \
  ruanun/sss-agent:latest
```

### 使用 Docker Volume

```bash
# 创建 volume
docker volume create sss-dashboard-logs
docker volume create sss-agent-logs

# 使用 volume
docker run -d \
  --name sss-dashboard \
  -v sss-dashboard-logs:/app/logs \
  -p 8900:8900 \
  ruanun/sssd:latest
```

## 网络配置

### 创建自定义网络

```bash
# 创建网络
docker network create sss-network

# 启动 Dashboard（在自定义网络中）
docker run -d \
  --name sss-dashboard \
  --network sss-network \
  -p 8900:8900 \
  ruanun/sssd:latest

# 启动 Agent（在同一网络中，可以使用容器名连接）
docker run -d \
  --name sss-agent \
  --network sss-network \
  -v $(pwd)/sss-agent.yaml:/app/sss-agent.yaml:ro \
  ruanun/sss-agent:latest
```

**Agent 配置使用容器名**:

```yaml
# sss-agent.yaml
serverAddr: ws://sss-dashboard:8900/ws-report
```

## 多平台支持

### 构建多平台镜像

```bash
# 创建 buildx builder
docker buildx create --name multiplatform --use

# 构建并推送多平台镜像
docker buildx build \
  --platform linux/amd64,linux/arm64,linux/arm/v7 \
  -f deployments/docker/Dockerfile.dashboard \
  -t ruanun/sssd:latest \
  --push \
  .
```

### 拉取特定平台镜像

```bash
# ARM64 (如树莓派 4、Apple Silicon Mac)
docker pull --platform linux/arm64 ruanun/sssd:latest

# ARMv7 (如树莓派 3)
docker pull --platform linux/arm/v7 ruanun/sssd:latest

# AMD64 (普通 x86 服务器)
docker pull --platform linux/amd64 ruanun/sssd:latest
```

## 健康检查

### Dashboard 健康检查

```yaml
healthcheck:
  test: ["CMD", "wget", "--quiet", "--tries=1", "--spider", "http://localhost:8900/api/statistics"]
  interval: 30s
  timeout: 10s
  retries: 3
  start_period: 40s
```

### 查看健康状态

```bash
# 查看容器健康状态
docker ps

# 详细健康检查日志
docker inspect --format='{{json .State.Health}}' sss-dashboard | jq
```

## 日志管理

### 查看日志

```bash
# 实时查看日志
docker logs -f sss-dashboard

# 查看最近 100 行
docker logs --tail 100 sss-dashboard

# 查看带时间戳的日志
docker logs -t sss-dashboard
```

### 日志轮转

**docker-compose.yml**:

```yaml
services:
  dashboard:
    image: ruanun/sssd:latest
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"
```

## 资源限制

### 限制 CPU 和内存

```bash
docker run -d \
  --name sss-dashboard \
  --cpus="1.0" \
  --memory="256m" \
  --memory-swap="512m" \
  -p 8900:8900 \
  ruanun/sssd:latest
```

**docker-compose.yml**:

```yaml
services:
  dashboard:
    image: ruanun/sssd:latest
    deploy:
      resources:
        limits:
          cpus: '1.0'
          memory: 256M
        reservations:
          cpus: '0.5'
          memory: 128M
```

## 更新和维护

### 更新容器

```bash
# 1. 拉取最新镜像
docker pull ruanun/sssd:latest

# 2. 停止并删除旧容器
docker stop sss-dashboard
docker rm sss-dashboard

# 3. 启动新容器
docker run -d \
  --name sss-dashboard \
  -v $(pwd)/sss-dashboard.yaml:/app/sss-dashboard.yaml:ro \
  -p 8900:8900 \
  ruanun/sssd:latest
```

### 使用 Docker Compose 更新

```bash
# 拉取最新镜像
docker-compose pull

# 重新创建容器
docker-compose up -d
```

### 备份配置

```bash
# 备份配置文件
cp sss-dashboard.yaml sss-dashboard.yaml.backup

# 备份 Docker Volume
docker run --rm \
  -v sss-dashboard-logs:/source:ro \
  -v $(pwd):/backup \
  alpine tar czf /backup/dashboard-logs-backup.tar.gz -C /source .
```

## 故障排查

### 容器无法启动

**检查日志**:

```bash
docker logs sss-dashboard
```

**常见问题**:

1. **配置文件格式错误**
   ```bash
   # 验证 YAML 格式
   docker run --rm -v $(pwd)/sss-dashboard.yaml:/config.yaml:ro \
     alpine sh -c "cat /config.yaml"
   ```

2. **端口被占用**
   ```bash
   # 检查端口
   netstat -an | grep 8900
   # 或
   lsof -i :8900
   ```

3. **权限问题**
   ```bash
   # 检查配置文件权限
   ls -l sss-dashboard.yaml

   # 修改权限
   chmod 644 sss-dashboard.yaml
   ```

### 网络连接问题

**Agent 无法连接 Dashboard**:

```bash
# 1. 检查 Dashboard 是否运行
docker ps | grep sss-dashboard

# 2. 检查网络连通性
docker exec sss-agent ping sss-dashboard

# 3. 检查 WebSocket 端口
docker exec sss-agent wget -O- http://sss-dashboard:8900/api/statistics
```

### 性能问题

**查看资源使用**:

```bash
# 实时监控
docker stats

# 查看特定容器
docker stats sss-dashboard sss-agent
```

## 生产环境建议

### 安全配置

1. **使用强密钥**:
   ```yaml
   servers:
     - secret: "$(openssl rand -base64 32)"
   ```

2. **只读挂载配置文件**:
   ```bash
   -v $(pwd)/config.yaml:/app/config.yaml:ro
   ```

3. **使用非 root 用户**（Dockerfile 中配置）:
   ```dockerfile
   RUN adduser -D -u 1000 appuser
   USER appuser
   ```

### 监控和告警

1. **集成 Prometheus**:
   ```yaml
   # 添加 metrics 端点
   services:
     dashboard:
       labels:
         - "prometheus.scrape=true"
         - "prometheus.port=8900"
   ```

2. **日志收集**:
   ```yaml
   logging:
     driver: "fluentd"
     options:
       fluentd-address: "localhost:24224"
   ```

### 高可用部署

**使用 Docker Swarm**:

```bash
# 初始化 Swarm
docker swarm init

# 部署服务
docker stack deploy -c docker-compose.yml sss

# 扩展副本
docker service scale sss_dashboard=3
```

## 相关文档

- [systemd 部署](./systemd.md) - systemd 服务部署
- [开发环境搭建](../development/setup.md) - 本地开发
- [架构概览](../architecture/overview.md) - 系统架构

---

**版本**: 1.0
**作者**: ruan
**最后更新**: 2025-11-05
