#!/bin/bash
# Dashboard 完整构建脚本（包含前端）
# 作者: ruan
# 说明: 先构建前端，再构建 Dashboard Go 程序

set -e  # 遇到错误立即退出

# 颜色定义
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 获取脚本所在目录的父目录（项目根目录）
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo -e "${BLUE}================================${NC}"
echo -e "${BLUE}  Dashboard 完整构建流程${NC}"
echo -e "${BLUE}================================${NC}"
echo ""

# 步骤 1: 构建前端
echo -e "${GREEN}📦 步骤 1/2: 构建前端项目${NC}"
echo ""

if [ -f "$SCRIPT_DIR/build-web.sh" ]; then
    bash "$SCRIPT_DIR/build-web.sh"
else
    echo -e "${RED}❌ 错误: 未找到 build-web.sh 脚本${NC}"
    exit 1
fi

echo ""
echo -e "${GREEN}✓ 前端构建完成${NC}"
echo ""

# 步骤 2: 构建 Dashboard Go 程序
echo -e "${GREEN}🔧 步骤 2/2: 构建 Dashboard 二进制文件${NC}"
echo ""

# 检查 Go 是否安装
if ! command -v go &> /dev/null; then
    echo -e "${RED}❌ 错误: 未找到 Go${NC}"
    echo -e "${YELLOW}请先安装 Go: https://golang.org/${NC}"
    exit 1
fi

echo -e "${GREEN}✓ Go 版本: $(go version)${NC}"

# 进入项目根目录
cd "$PROJECT_ROOT"

# 创建 bin 目录
BIN_DIR="$PROJECT_ROOT/bin"
mkdir -p "$BIN_DIR"

# 构建 Dashboard
echo -e "${YELLOW}🔨 编译 Dashboard...${NC}"
go build -v -o "$BIN_DIR/sss-dashboard" ./cmd/dashboard

# 验证构建结果
if [ -f "$BIN_DIR/sss-dashboard" ]; then
    echo -e "${GREEN}✅ Dashboard 构建成功！${NC}"
    echo ""
    echo -e "${BLUE}================================${NC}"
    echo -e "${BLUE}  构建完成${NC}"
    echo -e "${BLUE}================================${NC}"
    echo -e "${GREEN}  二进制文件: $BIN_DIR/sss-dashboard${NC}"

    # 显示文件大小
    FILE_SIZE=$(du -h "$BIN_DIR/sss-dashboard" | cut -f1)
    echo -e "${GREEN}  文件大小: $FILE_SIZE${NC}"

    # 设置可执行权限
    chmod +x "$BIN_DIR/sss-dashboard"

    echo ""
    echo -e "${YELLOW}运行方式:${NC}"
    echo -e "  ${GREEN}./bin/sss-dashboard${NC}"
    echo ""
else
    echo -e "${RED}❌ 构建失败${NC}"
    exit 1
fi
