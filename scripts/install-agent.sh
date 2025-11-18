#!/bin/bash

# Simple Server Status Agent 安装脚本
# 支持 Linux, macOS, FreeBSD

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 项目信息
REPO="${REPO:-ruanun/simple-server-status}"
BINARY_NAME="sss-agent"
SERVICE_NAME="sssa"
INSTALL_DIR="/etc/sssa"
CONFIG_FILE="sss-agent.yaml"

# 函数：打印信息
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 函数：检查是否为root用户
check_root() {
    if [[ $EUID -ne 0 ]]; then
        print_error "此脚本需要root权限运行"
        print_info "请使用: sudo $0"
        exit 1
    fi
}

# 函数：检测系统信息
detect_system() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)
    
    case $OS in
        linux*)
            OS="linux"
            ;;
        darwin*)
            OS="darwin"
            ;;
        freebsd*)
            OS="freebsd"
            ;;
        *)
            print_error "不支持的操作系统: $OS"
            exit 1
            ;;
    esac
    
    case $ARCH in
        x86_64|amd64)
            ARCH="amd64"
            ;;
        aarch64|arm64)
            ARCH="arm64"
            ;;
        armv7l|armv6l)
            ARCH="arm"
            ;;
        *)
            print_error "不支持的架构: $ARCH"
            exit 1
            ;;
    esac
    
    print_info "检测到系统: $OS-$ARCH"
}

# 函数：获取最新版本
get_latest_version() {
    print_info "获取最新版本信息..."
    
    if command -v curl >/dev/null 2>&1; then
        VERSION=$(curl -s "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    elif command -v wget >/dev/null 2>&1; then
        VERSION=$(wget -qO- "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    else
        print_error "需要安装 curl 或 wget"
        exit 1
    fi
    
    if [ -z "$VERSION" ]; then
        print_error "无法获取最新版本信息"
        exit 1
    fi
    
    print_info "最新版本: $VERSION"
}

# 函数：下载文件
download_file() {
    local url=$1
    local output=$2
    
    print_info "下载: $url"
    
    if command -v curl >/dev/null 2>&1; then
        curl -L -o "$output" "$url"
    elif command -v wget >/dev/null 2>&1; then
        wget -O "$output" "$url"
    else
        print_error "需要安装 curl 或 wget"
        exit 1
    fi
}

# 函数：下载并安装
download_and_install() {
    # 构建下载URL
    if [ "$OS" = "windows" ]; then
        ARCHIVE_EXT="zip"
    else
        ARCHIVE_EXT="tar.gz"
    fi

    # 处理 ARM 架构命名（armv7l/armv6l → armv7）
    if [ "$ARCH" = "arm" ]; then
        ARCH="armv7"
    fi

    # 使用简化的命名格式（与 GoReleaser 一致）
    ARCHIVE_NAME="sss-agent_${VERSION}_${OS}_${ARCH}.${ARCHIVE_EXT}"
    DOWNLOAD_URL="https://github.com/$REPO/releases/download/$VERSION/$ARCHIVE_NAME"
    
    # 创建临时目录
    TEMP_DIR=$(mktemp -d)
    cd "$TEMP_DIR"
    
    # 下载文件
    download_file "$DOWNLOAD_URL" "$ARCHIVE_NAME"
    
    # 解压文件
    print_info "解压文件..."
    if [ "$ARCHIVE_EXT" = "zip" ]; then
        unzip -q "$ARCHIVE_NAME"
    else
        tar -xzf "$ARCHIVE_NAME"
    fi
    
    # 查找解压后的目录
    EXTRACT_DIR=$(find . -maxdepth 1 -type d -name "sss-agent*" | head -1)
    if [ -z "$EXTRACT_DIR" ]; then
        print_error "无法找到解压后的目录"
        exit 1
    fi
    
    cd "$EXTRACT_DIR"
    
    # 创建安装目录
    print_info "创建安装目录: $INSTALL_DIR"
    mkdir -p "$INSTALL_DIR"
    
    # 复制二进制文件
    print_info "安装二进制文件..."
    cp "$BINARY_NAME" "$INSTALL_DIR/"
    chmod +x "$INSTALL_DIR/$BINARY_NAME"
    
    # 复制配置文件示例
    if [ -f "configs/sss-agent.yaml.example" ]; then
        if [ ! -f "$INSTALL_DIR/$CONFIG_FILE" ]; then
            print_info "复制配置文件示例..."
            cp "configs/sss-agent.yaml.example" "$INSTALL_DIR/$CONFIG_FILE"
            print_warning "请编辑配置文件: $INSTALL_DIR/$CONFIG_FILE"
        else
            print_info "配置文件已存在，跳过复制"
        fi
    fi
    
    # 安装systemd服务 (仅Linux)
    if [ "$OS" = "linux" ] && [ -f "deployments/systemd/sssa.service" ]; then
        print_info "安装systemd服务..."
        cp "deployments/systemd/sssa.service" "/etc/systemd/system/"
        systemctl daemon-reload
        print_info "服务已安装，使用以下命令管理:"
        print_info "  启动服务: systemctl start $SERVICE_NAME"
        print_info "  开机自启: systemctl enable $SERVICE_NAME"
        print_info "  查看状态: systemctl status $SERVICE_NAME"
    fi
    
    # 创建符号链接
    if [ ! -L "/usr/local/bin/sss-agent" ]; then
        print_info "创建符号链接..."
        ln -sf "$INSTALL_DIR/$BINARY_NAME" "/usr/local/bin/sss-agent"
    fi
    
    # 清理临时文件
    cd /
    rm -rf "$TEMP_DIR"
    
    print_success "安装完成！"
}

# 函数：显示使用说明
show_usage() {
    print_info "安装完成后的使用说明:"
    echo
    print_info "1. 编辑配置文件:"
    echo "   sudo nano $INSTALL_DIR/$CONFIG_FILE"
    echo
    print_info "2. 配置说明:"
    echo "   - serverAddr: Dashboard服务器WebSocket地址"
    echo "   - serverId: 服务器ID (在Dashboard中配置)"
    echo "   - authSecret: 认证密钥 (与Dashboard配置一致)"
    echo
    if [ "$OS" = "linux" ]; then
        print_info "3. 启动服务:"
        echo "   sudo systemctl start $SERVICE_NAME"
        echo "   sudo systemctl enable $SERVICE_NAME"
        echo
        print_info "4. 查看日志:"
        echo "   sudo journalctl -u $SERVICE_NAME -f"
    else
        print_info "3. 手动启动:"
        echo "   sudo $INSTALL_DIR/$BINARY_NAME -c $INSTALL_DIR/$CONFIG_FILE"
    fi
    echo
    print_info "5. 验证安装:"
    echo "   sss-agent --version"
}

# 函数：卸载
uninstall() {
    print_info "开始卸载 Simple Server Status Agent..."
    
    # 停止服务
    if [ "$OS" = "linux" ] && systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
        print_info "停止服务..."
        systemctl stop "$SERVICE_NAME"
        systemctl disable "$SERVICE_NAME"
    fi
    
    # 删除服务文件
    if [ -f "/etc/systemd/system/$SERVICE_NAME.service" ]; then
        print_info "删除服务文件..."
        rm -f "/etc/systemd/system/$SERVICE_NAME.service"
        systemctl daemon-reload
    fi
    
    # 删除符号链接
    if [ -L "/usr/local/bin/sss-agent" ]; then
        print_info "删除符号链接..."
        rm -f "/usr/local/bin/sss-agent"
    fi
    
    # 删除安装目录 (保留配置文件)
    if [ -d "$INSTALL_DIR" ]; then
        print_warning "是否删除安装目录 $INSTALL_DIR ? (配置文件将被删除) [y/N]"
        read -r response
        if [[ "$response" =~ ^[Yy]$ ]]; then
            rm -rf "$INSTALL_DIR"
            print_info "安装目录已删除"
        else
            rm -f "$INSTALL_DIR/$BINARY_NAME"
            print_info "仅删除二进制文件，配置文件已保留"
        fi
    fi
    
    print_success "卸载完成！"
}

# 主函数
main() {
    echo "Simple Server Status Agent 安装脚本"
    echo "===================================="
    echo
    
    # 解析命令行参数
    case "${1:-}" in
        --uninstall|-u)
            check_root
            detect_system
            uninstall
            exit 0
            ;;
        --help|-h)
            echo "用法: $0 [选项]"
            echo
            echo "选项:"
            echo "  --uninstall, -u    卸载 Simple Server Status Agent"
            echo "  --help, -h         显示此帮助信息"
            echo
            echo "环境变量:"
            echo "  REPO               GitHub仓库 (默认: $REPO)"
            echo "  VERSION            指定版本 (默认: 最新版本)"
            exit 0
            ;;
        "")
            # 默认安装
            ;;
        *)
            print_error "未知选项: $1"
            print_info "使用 $0 --help 查看帮助"
            exit 1
            ;;
    esac
    
    # 检查权限
    check_root
    
    # 检测系统
    detect_system
    
    # 获取版本信息
    if [ -z "${VERSION:-}" ]; then
        get_latest_version
    else
        print_info "使用指定版本: $VERSION"
    fi
    
    # 下载并安装
    download_and_install
    
    # 显示使用说明
    show_usage
}

# 执行主函数
main "$@"