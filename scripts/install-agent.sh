#!/usr/bin/env bash
# Simple Server Status Agent 安装脚本（Linux + systemd）
# 作者: ruan
# 用法:
#   curl -fsSL <脚本地址> | sudo bash -s -- --dashboard <地址> --id <ID> --secret <密钥>
#   curl -fsSL <脚本地址> | sudo bash -s -- --uninstall
set -euo pipefail

REPO="ruanun/simple-server-status"
BIN_PATH="/usr/local/bin/sss-agent"
CONFIG_DIR="/etc/sss"
CONFIG_FILE="${CONFIG_DIR}/sss-agent.yaml"
SERVICE_NAME="sss-agent"
UNIT_FILE="/etc/systemd/system/${SERVICE_NAME}.service"

DASHBOARD=""
ID=""
SECRET=""
VERSION="latest"
ACTION="install"
TMP_DIR=""

info() { printf '\033[0;32m[INFO]\033[0m %s\n' "$*"; }
die() {
  printf '\033[0;31m[ERROR]\033[0m %s\n' "$*" >&2
  exit 1
}

usage() {
  cat <<'EOF'
用法: install-agent.sh --dashboard <地址> --id <ID> --secret <密钥> [--version <版本>]
      install-agent.sh --uninstall

  --dashboard   Dashboard 地址，如 https://status.example.com
  --id          在后台创建服务器后得到的 ID
  --secret      服务器密钥
  --version     Agent 版本标签，如 v2.0.0（默认 latest）
  --uninstall   停止并删除 Agent 服务、程序与配置
EOF
}

# set_opt 记录一个带值参数
set_opt() {
  case "$1" in
    --dashboard) DASHBOARD="$2" ;;
    --id) ID="$2" ;;
    --secret) SECRET="$2" ;;
    --version) VERSION="$2" ;;
  esac
}

# parse_args 同时支持 "--key value" 与 "--key=value"
parse_args() {
  while [ $# -gt 0 ]; do
    case "$1" in
      --dashboard | --id | --secret | --version)
        [ $# -ge 2 ] || die "参数 $1 缺少取值"
        set_opt "$1" "$2"
        shift 2
        ;;
      --dashboard=* | --id=* | --secret=* | --version=*)
        set_opt "${1%%=*}" "${1#*=}"
        shift
        ;;
      --uninstall)
        ACTION="uninstall"
        shift
        ;;
      -h | --help)
        usage
        exit 0
        ;;
      *) die "未知参数: $1（使用 --help 查看用法）" ;;
    esac
  done
}

# validate_args 校验安装必填项，并去掉 Dashboard 地址末尾的斜杠
validate_args() {
  [ -n "$DASHBOARD" ] || die "缺少 --dashboard"
  [ -n "$ID" ] || die "缺少 --id"
  [ -n "$SECRET" ] || die "缺少 --secret"
  case "$DASHBOARD" in
    http://* | https://*) ;;
    *) die "Dashboard 地址需以 http:// 或 https:// 开头" ;;
  esac
  case "${DASHBOARD}${ID}${SECRET}" in
    *$'\n'*) die "参数中不能包含换行" ;;
  esac
  DASHBOARD="${DASHBOARD%/}"
}

# detect_arch 把 uname -m 映射为发布包中的架构名
detect_arch() {
  case "$1" in
    x86_64 | amd64) echo "amd64" ;;
    aarch64 | arm64) echo "arm64" ;;
    armv7*) echo "armv7" ;;
    *) return 1 ;;
  esac
}

# download_url 返回指定版本与架构的 Agent 压缩包地址
download_url() {
  local file="sss-agent_linux_$2.tar.gz"
  if [ "$1" = "latest" ]; then
    echo "https://github.com/${REPO}/releases/latest/download/${file}"
  else
    echo "https://github.com/${REPO}/releases/download/$1/${file}"
  fi
}

# yaml_quote 用 YAML 单引号包裹字符串，内部单引号写成两个
yaml_quote() {
  printf "'%s'" "${1//\'/\'\'}"
}

render_config() {
  cat <<EOF
# Simple Server Status Agent 配置（由安装脚本生成）
dashboard: $(yaml_quote "$DASHBOARD")
id: $(yaml_quote "$ID")
secret: $(yaml_quote "$SECRET")
detect_country: true
log_level: info
EOF
}

render_unit() {
  cat <<EOF
[Unit]
Description=Simple Server Status Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=${BIN_PATH} --config ${CONFIG_FILE}
Restart=on-failure
RestartSec=5s
# 加固：Agent 只读取系统信息，文件系统只读；若在配置中启用 log_file，请写到 /var/log/sss-agent/ 下
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=read-only
PrivateTmp=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
RestrictSUIDSGID=true
LockPersonality=true
LogsDirectory=sss-agent

[Install]
WantedBy=multi-user.target
EOF
}

# fetch 下载 URL 到文件，优先 curl，其次 wget
fetch() {
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL -o "$2" "$1"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "$2" "$1"
  else
    die "需要 curl 或 wget"
  fi
}

require_root() {
  [ "$(id -u)" -eq 0 ] || die "请使用 root 运行（sudo）"
  command -v systemctl >/dev/null 2>&1 || die "未检测到 systemd"
}

cleanup() {
  if [ -n "$TMP_DIR" ]; then rm -rf "$TMP_DIR"; fi
}

install_agent() {
  require_root
  local arch url
  arch="$(detect_arch "$(uname -m)")" || die "不支持的架构: $(uname -m)"
  url="$(download_url "$VERSION" "$arch")"
  TMP_DIR="$(mktemp -d)"
  trap cleanup EXIT

  info "下载 Agent：${url}"
  fetch "$url" "$TMP_DIR/agent.tar.gz" || die "下载失败：${url}"
  tar -xzf "$TMP_DIR/agent.tar.gz" -C "$TMP_DIR" sss-agent || die "解压失败"

  # 已安装时先停止旧服务，再覆盖程序与配置
  systemctl stop "$SERVICE_NAME" >/dev/null 2>&1 || true
  install -m 0755 "$TMP_DIR/sss-agent" "$BIN_PATH"
  install -d -m 0700 "$CONFIG_DIR"
  (
    umask 077
    render_config >"$CONFIG_FILE"
  )
  # 重装时配置文件可能已存在且权限不同，覆盖内容后强制恢复为 600
  chmod 600 "$CONFIG_FILE"
  render_unit >"$UNIT_FILE"
  systemctl daemon-reload
  systemctl enable "$SERVICE_NAME" >/dev/null 2>&1
  systemctl restart "$SERVICE_NAME"

  sleep 2
  systemctl is-active --quiet "$SERVICE_NAME" ||
    die "Agent 未能启动，请查看日志：journalctl -u ${SERVICE_NAME} -e"
  info "安装完成，Agent 已启动（$("$BIN_PATH" version)）"
  echo "  查看状态: systemctl status ${SERVICE_NAME}"
  echo "  查看日志: journalctl -u ${SERVICE_NAME} -f"
  echo "  卸载:     curl -fsSL <脚本地址> | sudo bash -s -- --uninstall"
}

uninstall_agent() {
  require_root
  systemctl disable --now "$SERVICE_NAME" >/dev/null 2>&1 || true
  rm -f "$UNIT_FILE" "$BIN_PATH" "$CONFIG_FILE"
  rmdir "$CONFIG_DIR" 2>/dev/null || true
  systemctl daemon-reload
  info "Agent 已卸载"
}

main() {
  parse_args "$@"
  if [ "$ACTION" = "uninstall" ]; then
    uninstall_agent
    return
  fi
  validate_args
  install_agent
}

# 测试时设置 SSS_INSTALL_LIB=1，只加载函数不执行
if [ "${SSS_INSTALL_LIB:-}" != "1" ]; then
  main "$@"
fi
