#!/usr/bin/env bash

# ============================================
# Docker 本地构建测试脚本
# 作者: ruan
# 用途: 在本地测试 Docker 镜像构建
# ============================================

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 配置变量
IMAGE_NAME="sssd"
TAG="dev"
VERSION="dev-$(date +%Y%m%d-%H%M%S)"
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}Docker 本地构建测试${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo -e "${YELLOW}镜像信息:${NC}"
echo "  名称: ${IMAGE_NAME}"
echo "  标签: ${TAG}"
echo "  版本: ${VERSION}"
echo "  提交: ${COMMIT}"
echo "  构建时间: ${BUILD_DATE}"
echo ""

# 检查 Docker 是否安装
if ! command -v docker &> /dev/null; then
    echo -e "${RED}错误: Docker 未安装${NC}"
    exit 1
fi

# 构建选项
BUILD_PLATFORM="linux/amd64"
if [[ "$1" == "--multi-arch" ]]; then
    BUILD_PLATFORM="linux/amd64,linux/arm64,linux/arm/v7"
    echo -e "${YELLOW}多架构构建模式: ${BUILD_PLATFORM}${NC}"

    # 检查 buildx 是否可用
    if ! docker buildx version &> /dev/null; then
        echo -e "${RED}错误: Docker Buildx 未安装${NC}"
        echo "请运行: docker buildx install"
        exit 1
    fi
else
    echo -e "${YELLOW}单架构构建模式: ${BUILD_PLATFORM}${NC}"
fi

echo ""
echo -e "${GREEN}开始构建 Docker 镜像...${NC}"
echo ""

# 构建镜像
if [[ "$1" == "--multi-arch" ]]; then
    # 多架构构建
    docker buildx build \
        --platform ${BUILD_PLATFORM} \
        --build-arg VERSION="${VERSION}" \
        --build-arg COMMIT="${COMMIT}" \
        --build-arg BUILD_DATE="${BUILD_DATE}" \
        --build-arg TZ="Asia/Shanghai" \
        -t ${IMAGE_NAME}:${TAG} \
        -f Dockerfile \
        --load \
        .
else
    # 单架构构建
    docker build \
        --platform ${BUILD_PLATFORM} \
        --build-arg VERSION="${VERSION}" \
        --build-arg COMMIT="${COMMIT}" \
        --build-arg BUILD_DATE="${BUILD_DATE}" \
        --build-arg TZ="Asia/Shanghai" \
        -t ${IMAGE_NAME}:${TAG} \
        -f Dockerfile \
        .
fi

if [ $? -eq 0 ]; then
    echo ""
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}构建成功！${NC}"
    echo -e "${GREEN}========================================${NC}"
    echo ""

    # 显示镜像信息
    echo -e "${YELLOW}镜像详情:${NC}"
    docker images ${IMAGE_NAME}:${TAG}
    echo ""

    echo -e "${YELLOW}镜像大小分析:${NC}"
    docker image inspect ${IMAGE_NAME}:${TAG} --format='镜像大小: {{.Size}} bytes ({{ div .Size 1048576 }} MB)'
    echo ""

    # 提供运行命令
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}测试运行命令:${NC}"
    echo -e "${GREEN}========================================${NC}"
    echo ""
    echo "1. 使用示例配置运行:"
    echo -e "   ${YELLOW}docker run --rm -p 8900:8900 -v \$(pwd)/configs/sss-dashboard.yaml.example:/app/sss-dashboard.yaml ${IMAGE_NAME}:${TAG}${NC}"
    echo ""
    echo "2. 交互式运行（调试）:"
    echo -e "   ${YELLOW}docker run --rm -it -p 8900:8900 ${IMAGE_NAME}:${TAG} sh${NC}"
    echo ""
    echo "3. 后台运行:"
    echo -e "   ${YELLOW}docker run -d --name sssd-test -p 8900:8900 -v \$(pwd)/configs/sss-dashboard.yaml.example:/app/sss-dashboard.yaml ${IMAGE_NAME}:${TAG}${NC}"
    echo ""
    echo "4. 查看日志:"
    echo -e "   ${YELLOW}docker logs -f sssd-test${NC}"
    echo ""
    echo "5. 停止并删除容器:"
    echo -e "   ${YELLOW}docker stop sssd-test && docker rm sssd-test${NC}"
    echo ""
else
    echo ""
    echo -e "${RED}========================================${NC}"
    echo -e "${RED}构建失败！${NC}"
    echo -e "${RED}========================================${NC}"
    exit 1
fi
