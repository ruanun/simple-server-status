# Simple Server Status Dashboard 镜像（多阶段构建）
# 作者: ruan
# 构建（在仓库根目录）：docker build -f deployments/docker/Dockerfile -t sssd:dev .

# 前端与 Go 编译都在构建机原生架构上执行，多架构构建时只交叉编译 Go
FROM --platform=$BUILDPLATFORM node:22-alpine AS web
RUN npm install -g pnpm@10
WORKDIR /src/web
COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm build

FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS go
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY --from=web /src/internal/dashboard/web/dist ./internal/dashboard/web/dist
ARG VERSION=dev
ARG TARGETOS TARGETARCH TARGETVARIANT
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOARM=${TARGETVARIANT#v} \
    go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/sss-dashboard ./cmd/sss-dashboard

FROM alpine:3.22
RUN apk add --no-cache tzdata ca-certificates \
    && addgroup -S -g 1000 sss && adduser -S -D -H -u 1000 -G sss sss \
    && mkdir -p /app/data && chown sss:sss /app/data
ENV TZ=Asia/Shanghai \
    SSS_LISTEN=:8900 \
    SSS_DATA_DIR=/app/data
WORKDIR /app
COPY --from=go /out/sss-dashboard /app/sss-dashboard
USER sss
VOLUME ["/app/data"]
EXPOSE 8900
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -q --spider http://127.0.0.1:8900/api/public/site || exit 1
ENTRYPOINT ["/app/sss-dashboard"]
