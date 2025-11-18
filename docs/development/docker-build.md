# Docker 构建指南

本项目使用多阶段构建 Dockerfile，支持完全自包含的前后端构建流程。

## 📋 目录

- [快速开始](#快速开始)
- [构建方式对比](#构建方式对比)
- [本地构建](#本地构建)
- [CI/CD 构建](#cicd-构建)
- [多架构支持](#多架构支持)
- [常见问题](#常见问题)

## 🚀 快速开始

### 方式 1：使用 Make 命令（推荐）

```bash
# 构建 Docker 镜像
make docker-build

# 运行容器
make docker-run

# 清理镜像
make docker-clean
```

### 方式 2：使用脚本

**Linux/macOS:**
```bash
# 单架构构建
bash scripts/build-docker.sh

# 多架构构建
bash scripts/build-docker.sh --multi-arch
```

**Windows (PowerShell):**
```powershell
# 单架构构建
.\scripts\build-docker.ps1

# 多架构构建
.\scripts\build-docker.ps1 --multi-arch
```

### 方式 3：直接使用 Docker 命令

```bash
# 基础构建
docker build -t sssd:dev -f Dockerfile .

# 带参数构建
docker build \
  --build-arg VERSION=v1.0.0 \
  --build-arg COMMIT=$(git rev-parse --short HEAD) \
  --build-arg BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ") \
  -t sssd:dev \
  -f Dockerfile \
  .
```

## 📊 构建方式对比

| 方式 | 优点 | 缺点 | 适用场景 |
|------|------|------|---------|
| **多阶段 Dockerfile** (当前) | 完全自包含、镜像小、易维护 | 构建时间稍长 | ✅ 推荐用于所有场景 |
| GoReleaser + Docker | 一体化发布流程 | 配置复杂 | CI/CD 自动化发布 |
| 简单 Dockerfile | 构建快速 | 依赖外部编译 | 已有编译产物 |

## 🛠️ 本地构建

### 构建参数说明

Dockerfile 支持以下构建参数：

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `VERSION` | `dev` | 版本号 |
| `COMMIT` | `unknown` | Git 提交哈希 |
| `BUILD_DATE` | `unknown` | 构建时间 |
| `TZ` | `Asia/Shanghai` | 时区设置 |

### 完整构建示例

```bash
docker build \
  --build-arg VERSION=v1.2.3 \
  --build-arg COMMIT=$(git rev-parse --short HEAD) \
  --build-arg BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ") \
  --build-arg TZ=Asia/Shanghai \
  -t sssd:v1.2.3 \
  -f Dockerfile \
  .
```

### 运行容器

**使用示例配置:**
```bash
docker run --rm -p 8900:8900 \
  -v $(pwd)/configs/sss-dashboard.yaml.example:/app/sss-dashboard.yaml \
  sssd:dev
```

**挂载自定义配置:**
```bash
docker run -d \
  --name sssd \
  -p 8900:8900 \
  -v /path/to/your/config.yaml:/app/sss-dashboard.yaml \
  -v /path/to/logs:/app/.logs \
  --restart=unless-stopped \
  sssd:dev
```

## 🔄 CI/CD 构建

项目使用 GitHub Actions 自动构建和推送 Docker 镜像。

### 工作流程

1. **触发条件**: 推送 tag（如 `v1.0.0`）
2. **构建流程**:
   - GoReleaser 编译二进制文件（多平台）
   - Docker Buildx 构建镜像（多架构）
   - 推送到 Docker Hub

### 自动构建的镜像标签

- `ruanun/sssd:v1.0.0` - 完整版本号
- `ruanun/sssd:1.0` - 主版本号
- `ruanun/sssd:1` - 大版本号
- `ruanun/sssd:latest` - 最新版本

### GitHub Actions 配置

参考 `.github/workflows/release.yml`:

```yaml
- name: 构建并推送 Docker 镜像
  uses: docker/build-push-action@v5
  with:
    context: .
    file: ./Dockerfile
    platforms: linux/amd64,linux/arm64,linux/arm/v7
    push: true
    build-args: |
      VERSION=${{ github.ref_name }}
      COMMIT=${{ github.sha }}
      BUILD_DATE=${{ github.event.repository.updated_at }}
```

## 🌐 多架构支持

项目支持以下平台架构：

- `linux/amd64` - x86_64 架构（PC、服务器）
- `linux/arm64` - ARM64 架构（Apple Silicon、ARM 服务器）
- `linux/arm/v7` - ARMv7 架构（树莓派 3/4）

### 使用 Buildx 构建多架构镜像

```bash
# 创建 builder（首次）
docker buildx create --name multiarch --use

# 构建并推送多架构镜像
docker buildx build \
  --platform linux/amd64,linux/arm64,linux/arm/v7 \
  --build-arg VERSION=v1.0.0 \
  -t username/sssd:v1.0.0 \
  -f Dockerfile \
  --push \
  .
```

## 📦 镜像优化

### 镜像大小

- **最终镜像大小**: ~30 MB
- **基础镜像**: Alpine Linux (轻量级)
- **优化措施**:
  - 多阶段构建（分离构建和运行环境）
  - 静态编译（无 CGO 依赖）
  - 清理不必要文件

### 安全性

- ✅ 使用非 root 用户运行
- ✅ 最小化运行时依赖
- ✅ 定期更新基础镜像
- ✅ 健康检查配置

### 健康检查

Dockerfile 内置健康检查：

```dockerfile
HEALTHCHECK --interval=30s --timeout=10s --start-period=40s --retries=3 \
    CMD wget --quiet --tries=1 --spider http://localhost:8900/api/statistics || exit 1
```

## ❓ 常见问题

### Q1: 构建失败：前端依赖安装错误

**解决方案:**
```bash
# 清理 web/node_modules
rm -rf web/node_modules

# 重新构建
docker build --no-cache -t sssd:dev -f Dockerfile .
```

### Q2: 镜像体积过大

**检查镜像层:**
```bash
docker history sssd:dev
```

**确保使用多阶段构建的最终阶段:**
```dockerfile
FROM alpine:latest  # 最终阶段
```

### Q3: 多架构构建失败

**安装 QEMU 模拟器:**
```bash
docker run --rm --privileged multiarch/qemu-user-static --reset -p yes
```

**重新创建 builder:**
```bash
docker buildx rm multiarch
docker buildx create --name multiarch --use
docker buildx inspect --bootstrap
```

### Q4: 容器启动后立即退出

**查看日志:**
```bash
docker logs <container_id>
```

**常见原因:**
- 配置文件路径错误
- 配置文件格式错误
- 端口被占用

**调试模式运行:**
```bash
docker run --rm -it sssd:dev sh
```

### Q5: 如何查看镜像构建参数

```bash
docker inspect sssd:dev | grep -A 10 "Labels"
```

## 📚 相关文档

- [Dockerfile 最佳实践](https://docs.docker.com/develop/develop-images/dockerfile_best-practices/)
- [多阶段构建](https://docs.docker.com/build/building/multi-stage/)
- [Docker Buildx](https://docs.docker.com/buildx/working-with-buildx/)
- [部署文档](../deployment/docker.md)

## 🤝 贡献

如果你有改进 Docker 构建的建议，欢迎提交 Issue 或 Pull Request！

---

**作者**: ruan
**更新时间**: 2025-01-15
