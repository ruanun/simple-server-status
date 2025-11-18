# ============================================
# Simple Server Status Dashboard - 多阶段构建
# 作者: ruan
# 说明: 这个 Dockerfile 包含完整的前后端构建流程
# ============================================

# ============================================
# 阶段 1: 前端构建
# ============================================
FROM node:20-alpine AS frontend-builder

WORKDIR /build/web

# 启用 corepack 并安装 pnpm
RUN corepack enable && corepack prepare pnpm@latest --activate

# 复制前端依赖文件（利用 Docker 层缓存）
COPY web/package.json web/pnpm-lock.yaml ./

# 安装依赖（包括构建工具）
RUN pnpm install --frozen-lockfile

# 复制前端源码
COPY web/ ./

# 构建前端生产版本
RUN pnpm run build:prod

# ============================================
# 阶段 2: 后端构建
# ============================================
FROM golang:1.23-alpine AS backend-builder

# 安装构建依赖
RUN apk add --no-cache git make

WORKDIR /build

# 复制 Go 依赖文件（利用 Docker 层缓存）
COPY go.mod go.sum ./
RUN go mod download

# 复制后端源码
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY pkg/ ./pkg/

# 从前端构建阶段复制构建产物
COPY --from=frontend-builder /build/web/dist ./internal/dashboard/public/dist

# 构建参数（可在 docker build 时传入）
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

# 编译后端（静态链接，无 CGO）
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w \
    -X main.version=${VERSION} \
    -X main.commit=${COMMIT} \
    -X main.date=${BUILD_DATE}" \
    -trimpath \
    -o /build/sss-dashboard \
    ./cmd/dashboard

# ============================================
# 阶段 3: 最终运行时镜像
# ============================================
FROM alpine:latest

# 构建参数
ARG TZ="Asia/Shanghai"
ENV TZ=${TZ}

# 设置标签（OCI 标准）
LABEL org.opencontainers.image.title="Simple Server Status Dashboard"
LABEL org.opencontainers.image.description="极简服务器监控探针 - Dashboard"
LABEL org.opencontainers.image.authors="ruan"
LABEL org.opencontainers.image.url="https://github.com/ruanun/simple-server-status"
LABEL org.opencontainers.image.source="https://github.com/ruanun/simple-server-status"
LABEL org.opencontainers.image.licenses="MIT"

# 安装运行时依赖
RUN apk upgrade --no-cache && \
    apk add --no-cache \
        bash \
        tzdata \
        ca-certificates \
        wget && \
    ln -sf /usr/share/zoneinfo/${TZ} /etc/localtime && \
    echo ${TZ} > /etc/timezone && \
    rm -rf /var/cache/apk/*

WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=backend-builder /build/sss-dashboard ./sssd

# 创建非 root 用户（安全性最佳实践）
RUN addgroup -g 1000 sssd && \
    adduser -D -u 1000 -G sssd sssd && \
    chown -R sssd:sssd /app && \
    chmod +x /app/sssd

# 切换到非 root 用户
USER sssd

# 健康检查（每 30 秒检查一次）
HEALTHCHECK --interval=30s --timeout=10s --start-period=40s --retries=3 \
    CMD wget --quiet --tries=1 --spider http://localhost:8900/api/statistics || exit 1

# 环境变量
ENV CONFIG="sss-dashboard.yaml"

# 暴露端口
EXPOSE 8900

# 启动命令
CMD ["/app/sssd"]
