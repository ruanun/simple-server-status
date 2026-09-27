#!/usr/bin/env bash
# install-agent.sh 的函数级测试
# 作者: ruan
# 用法: bash scripts/install-agent.test.sh
set -euo pipefail
cd "$(dirname "$0")"

# shellcheck source=install-agent.sh
SSS_INSTALL_LIB=1 source ./install-agent.sh

failed=0
check() { # check <描述> <期望> <实际>
  if [ "$2" = "$3" ]; then
    echo "ok   $1"
  else
    echo "FAIL $1: 期望 [$2] 实际 [$3]"
    failed=1
  fi
}
succeeds() { # succeeds <命令...>：命令成功输出 yes，否则 no（在子 shell 中执行，die 的 exit 不影响本脚本）
  if ( "$@" ) >/dev/null 2>&1; then echo yes; else echo no; fi
}

# 后台生成命令的确切形式：bash -s -- --dashboard '<地址>' --id '<ID>' --secret '<密钥>'
parse_args --dashboard 'https://s.example.com/' --id 'abc' --secret '$x y'
check "空格分隔写法" 'https://s.example.com/|abc|$x y' "$DASHBOARD|$ID|$SECRET"
validate_args
check "去掉末尾斜杠" "https://s.example.com" "$DASHBOARD"

parse_args --dashboard=http://10.0.0.1:8900 --id=i2 --secret=s2 --version=v2.0.0
check "等号写法" "http://10.0.0.1:8900|i2|s2|v2.0.0" "$DASHBOARD|$ID|$SECRET|$VERSION"

check "未知参数报错" no "$(succeeds parse_args --foo)"
check "参数缺少取值报错" no "$(succeeds parse_args --id)"
check "缺少 dashboard 报错" no "$(succeeds bash -c 'SSS_INSTALL_LIB=1 source ./install-agent.sh; ID=a SECRET=b validate_args')"
check "缺少 secret 报错" no "$(succeeds bash -c 'SSS_INSTALL_LIB=1 source ./install-agent.sh; DASHBOARD=https://a ID=a validate_args')"
check "非 http 地址报错" no "$(succeeds bash -c 'SSS_INSTALL_LIB=1 source ./install-agent.sh; DASHBOARD=ftp://a ID=a SECRET=b validate_args')"

check "x86_64" amd64 "$(detect_arch x86_64)"
check "aarch64" arm64 "$(detect_arch aarch64)"
check "armv7l" armv7 "$(detect_arch armv7l)"
check "不支持的架构" no "$(succeeds detect_arch mips)"

check "latest 地址" \
  "https://github.com/ruanun/simple-server-status/releases/latest/download/sss-agent_linux_amd64.tar.gz" \
  "$(download_url latest amd64)"
check "指定版本地址" \
  "https://github.com/ruanun/simple-server-status/releases/download/v2.0.0/sss-agent_linux_arm64.tar.gz" \
  "$(download_url v2.0.0 arm64)"

check "YAML 单引号转义" "'it''s'" "$(yaml_quote "it's")"
DASHBOARD="https://s.example.com" ID="a'b" SECRET='$x y'
cfg="$(render_config)"
check "配置 id 行" yes "$(grep -qxF "id: 'a''b'" <<<"$cfg" && echo yes || echo no)"
check "配置 secret 行不展开" yes "$(grep -qxF "secret: '\$x y'" <<<"$cfg" && echo yes || echo no)"
check "配置 dashboard 行" yes "$(grep -qxF "dashboard: 'https://s.example.com'" <<<"$cfg" && echo yes || echo no)"

unit="$(render_unit)"
check "Restart=on-failure" yes "$(grep -qx 'Restart=on-failure' <<<"$unit" && echo yes || echo no)"
for opt in NoNewPrivileges=true ProtectSystem=strict ProtectHome=read-only LogsDirectory=sss-agent; do
  check "加固 $opt" yes "$(grep -qx "$opt" <<<"$unit" && echo yes || echo no)"
done
check "ExecStart" yes "$(grep -qxF 'ExecStart=/usr/local/bin/sss-agent --config /etc/sss/sss-agent.yaml' <<<"$unit" && echo yes || echo no)"

exit "$failed"
