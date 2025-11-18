#!/bin/bash
# 前端构建脚本
# 作者: ruan
# 说明: 构建前端项目并复制到 embed 目录

set -e  # 遇到错误立即退出

# 颜色定义
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# 获取脚本所在目录的父目录（项目根目录）
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo -e "${GREEN}📦 开始构建前端项目...${NC}"

# 检查 Node.js 是否安装
if ! command -v node &> /dev/null; then
    echo -e "${RED}❌ 错误: 未找到 Node.js${NC}"
    echo -e "${YELLOW}请先安装 Node.js: https://nodejs.org/${NC}"
    exit 1
fi

# 检查 pnpm 是否安装
if ! command -v pnpm &> /dev/null; then
    echo -e "${RED}❌ 错误: 未找到 pnpm${NC}"
    echo -e "${YELLOW}请先安装 pnpm: npm install -g pnpm 或 corepack enable${NC}"
    exit 1
fi

echo -e "${GREEN}✓ Node.js 版本: $(node --version)${NC}"
echo -e "${GREEN}✓ pnpm 版本: $(pnpm --version)${NC}"

# 进入 web 目录
cd "$PROJECT_ROOT/web"

# 检查 package.json 是否存在
if [ ! -f "package.json" ]; then
    echo -e "${RED}❌ 错误: 未找到 package.json${NC}"
    exit 1
fi

# 安装依赖（仅在 node_modules 不存在时）
if [ ! -d "node_modules" ]; then
    echo -e "${YELLOW}📥 安装前端依赖...${NC}"
    pnpm install --frozen-lockfile
else
    echo -e "${GREEN}✓ 依赖已存在，跳过安装${NC}"
fi

# 构建前端项目
echo -e "${YELLOW}🔨 构建前端项目（生产模式）...${NC}"
pnpm run build:prod

# 检查构建产物是否存在
if [ ! -d "dist" ]; then
    echo -e "${RED}❌ 错误: 构建失败，未找到 dist 目录${NC}"
    exit 1
fi

# 返回项目根目录
cd "$PROJECT_ROOT"

# 目标目录
EMBED_DIR="$PROJECT_ROOT/internal/dashboard/public/dist"

# 创建目标目录（如果不存在）
mkdir -p "$EMBED_DIR"

# 清空目标目录（保留 .gitkeep 或 README.md）
echo -e "${YELLOW}🗑️  清理 embed 目录...${NC}"
find "$EMBED_DIR" -mindepth 1 ! -name '.gitkeep' ! -name 'README.md' -delete

# 复制构建产物
echo -e "${YELLOW}📋 复制构建产物到 embed 目录...${NC}"
cp -r web/dist/* "$EMBED_DIR/"

# 验证复制结果
if [ -d "$EMBED_DIR/assets" ]; then
    echo -e "${GREEN}✅ 前端构建完成！${NC}"
    echo -e "${GREEN}   输出目录: $EMBED_DIR${NC}"

    # 显示文件统计
    FILE_COUNT=$(find "$EMBED_DIR" -type f | wc -l)
    echo -e "${GREEN}   文件数量: $FILE_COUNT${NC}"
else
    echo -e "${RED}❌ 错误: 复制失败，未找到 assets 目录${NC}"
    exit 1
fi
