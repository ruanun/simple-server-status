# 重写计划 3：构建、部署、发布与清理 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让重写后的 Agent / Dashboard / 前端能被一键安装、构建、打包、发布，并清理仓库中的旧文件与文档。

**Architecture:** 构建入口统一为 Makefile + 前端 `pnpm build`（直接输出到 `internal/dashboard/web/dist`），删除旧的构建脚本；Dockerfile 移到 `deployments/docker/`；goreleaser 产出不带版本号的归档名，并把两个安装脚本作为 Release 资产上传，使后台默认的 `install_script_base`（`releases/latest/download`）可直接使用；安装脚本重写为与后台生成命令一致的参数，Agent 增加 Windows 服务支持。

**Tech Stack:** Go 1.24、golang.org/x/sys/windows/svc、Bash、PowerShell 5.1/7、Docker（buildx 多架构）、GoReleaser v2、GitHub Actions、Node 22 + pnpm 10。

**Spec:** `docs/superpowers/specs/2026-09-25-rewrite-design.md`（§2.2 目录、§4 运行参数、§8 构建发布与文档、§9 仓库清理）

## Global Constraints

- 代码注释、提交信息、文档一律简体中文；新文件需要作者时写 `ruan`。
- 允许 `git commit`；禁止 `git push` / `pull` / `merge` / `rebase` / `reset`。
- 不得修改或删除：`data/`、根目录 `sss-agent.yaml`（含真实密钥）、`.claude/`、`.idea/`、`.codex/`。
- 修改 Go 代码后必须通过：`golangci-lint run --timeout=5m ./...`（本机 v2.14.0）、`go vet ./...`、`go test ./...`。
- 修改前端后必须通过（在 `web/` 下）：`pnpm typecheck`、`pnpm lint`（0 错误）、`pnpm test`、`pnpm build`。
- 前端包管理器 pnpm 10；CI 与 Docker 使用 Node 22、pnpm 10。
- 行尾：`*.sh` 为 LF；`*.ps1` 为 CRLF 且**文件以 UTF-8 BOM 开头**（Windows PowerShell 5.1 对无 BOM 的 UTF-8 中文会乱码）。`.gitattributes` 已配置行尾，不要改动它。
- Agent 配置（YAML 键）：`dashboard`、`id`、`secret`、`detect_country`、`log_level`、`log_file`；参数：`--config`、`--dashboard`、`--id`、`--secret`、`--detect-country`、`--log-level`、`--log-file`；默认配置查找 `./sss-agent.yaml`、`/etc/sss/sss-agent.yaml`。
- Dashboard 参数 / 环境变量：`--listen`/`SSS_LISTEN`（`:8900`）、`--data-dir`/`SSS_DATA_DIR`（`./data`）、`--trusted-proxies`/`SSS_TRUSTED_PROXIES`（逗号分隔 IP/CIDR）、`--admin-password`/`SSS_ADMIN_PASSWORD`、`--log-level`、`--log-file`；子命令 `serve`（默认）、`reset-password`、`version`。
- 版本号通过 `-ldflags "-X main.version=<版本>"` 注入（两个 `cmd/*/main.go` 都只有 `main.version`）。
- **发布资产命名契约**（Task 2、3、6 共同依赖）：
  - Agent 归档：`sss-agent_<os>_<arch>.tar.gz`（Windows 为 `.zip`），`<arch>` ∈ `amd64`、`arm64`、`armv7`；归档**不包一层目录**，根目录直接是 `sss-agent`（Windows 为 `sss-agent.exe`）。
  - Dashboard 归档：`sss-dashboard_<os>_<arch>.tar.gz` / `.zip`，同样不包目录。
  - Release 额外资产：`install-agent.sh`、`install-agent.ps1`。
  - 下载地址：最新版 `https://github.com/ruanun/simple-server-status/releases/latest/download/<文件>`；指定版本 `https://github.com/ruanun/simple-server-status/releases/download/<tag>/<文件>`。
- **后台生成的安装命令格式**（`internal/dashboard/api/admin.go` 的 `installCommands`，本计划不改）：
  - Linux：`curl -fsSL '<base>/install-agent.sh' | sudo bash -s -- --dashboard '<地址>' --id '<ID>' --secret '<密钥>'`
  - Windows：`$f="$env:TEMP\install-agent.ps1"; iwr -useb '<base>/install-agent.ps1' -OutFile $f; & $f -Dashboard '<地址>' -Id '<ID>' -Secret '<密钥>'`

## Review Focus

1. 后台生成的安装命令原样执行（空格分隔的 `--dashboard 'x' --id 'y' --secret 'z'`）必须被两个安装脚本正确解析——Task 2、Task 3 的测试用后台命令的确切形式覆盖。
2. 密钥 / ID 含单引号、`$`、空格时，生成的 YAML 仍正确且不被 shell 展开——Task 2、Task 3 的测试覆盖 `a'b` 与 `$x y`。
3. 在已安装 Agent 的机器上重复运行安装脚本（升级 / 换密钥）应覆盖程序、配置和服务并重新启动，而不是报错退出——Task 2、Task 3 的实现步骤显式先停止旧服务；审查时重点看。
4. 用 Windows PowerShell 5.1（非 pwsh）运行含中文的 ps1 不乱码——Task 3 的测试检查 UTF-8 BOM，并分别用 `powershell` 与 `pwsh` 运行测试。
5. 全新 clone（没有预先构建的前端）执行 `make build` / `docker build` 必须成功，而 goreleaser 在前端未构建时必须失败而不是发布出空界面——Task 5 用干净上下文构建镜像，Task 6 的 CI 运行 `make build`，并在本机验证 goreleaser 的前置检查。

---

## 文件结构

| 路径 | 动作 | 职责 |
|---|---|---|
| `cmd/sss-agent/main.go` | 修改 | 拆出 `serve()`；在 Windows 服务管理器下改走服务模式 |
| `cmd/sss-agent/service_windows.go` / `service_other.go` / `service_windows_test.go` | 新建 | Windows 服务处理器（x/sys/windows/svc）与非 Windows 空实现 |
| `scripts/install-agent.sh` / `install-agent.test.sh` | 重写 / 新建 | Linux 安装（systemd）与函数级测试 |
| `scripts/install-agent.ps1` / `install-agent.test.ps1` | 重写 / 新建 | Windows 安装（Windows 服务）与函数级测试 |
| `web/scripts/check-bundle.mjs` / `check-bundle.test.mjs` | 新建 | 入口资源 gzip 预算检查 |
| `Makefile` | 修改 | 唯一构建入口 |
| `scripts/build-*.sh|ps1`、`scripts/check-web-bundle*`、`scripts/README.md` | 删除 | 被 Makefile / check-bundle 取代 |
| `deployments/docker/Dockerfile`、`.dockerignore`、`deployments/docker/docker-compose.yml`、`deployments/caddy/Caddyfile` | 新建 / 重写 | 镜像与部署示例 |
| 根目录 `Dockerfile`、`deployments/systemd/` | 删除 | 被新位置 / 安装脚本取代 |
| `.goreleaser.yml`、`.github/workflows/ci.yml`、`.github/workflows/release.yml` | 重写 | 打包、CI、发布 |
| `web/src/lib/api.ts`、`web/src/features/admin/settings-page.tsx` | 修改 | 前端两条留存小问题 |
| `README.md`、`docs/*`、`AGENTS.md`、`CHANGELOG.md`、`claude.md` | 重写 / 删除 | 文档 |

---

### Task 1: Agent 支持作为 Windows 服务运行

**Files:**
- Modify: `cmd/sss-agent/main.go`（`runAgent`，约 84–118 行）
- Create: `cmd/sss-agent/service_windows.go`
- Create: `cmd/sss-agent/service_other.go`
- Test: `cmd/sss-agent/service_windows_test.go`
- Modify: `go.mod` / `go.sum`（`go mod tidy` 后 `golang.org/x/sys` 变为直接依赖）

**Interfaces:**
- Produces: `func runAsService(run func(context.Context) error) (bool, error)` —— 当前进程由 Windows 服务管理器启动时以服务方式运行 `run` 并返回 `true`；其他情况返回 `false, nil`。服务收到 Stop/Shutdown 时取消 ctx 并等待 `run` 返回；`run` 返回 nil 时服务以退出码 0 停止，返回错误时以服务专用退出码 1 停止（配合 Task 3 的 `sc.exe failureflag` 触发自动重启）。
- Produces: `func serve(ctx context.Context, cfg agent.Config, log *slog.Logger) error` —— 原 `runAgent` 中「创建 client → 探测国家 → Run」部分；`agent.ErrStopped` 转为 nil。

- [ ] **Step 1: 写失败的测试**

`cmd/sss-agent/service_windows_test.go`：

```go
//go:build windows

package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"golang.org/x/sys/windows/svc"
)

// executeAsync 在后台运行服务处理器，返回请求通道与结果通道
func executeAsync(h *serviceHandler) (chan svc.ChangeRequest, chan [2]any) {
	req := make(chan svc.ChangeRequest)
	status := make(chan svc.Status, 16)
	done := make(chan [2]any, 1)
	go func() {
		ssec, code := h.Execute(nil, req, status)
		done <- [2]any{ssec, code}
	}()
	return req, done
}

func TestServiceHandlerStopCancelsRun(t *testing.T) {
	started := make(chan struct{})
	h := &serviceHandler{run: func(ctx context.Context) error {
		close(started)
		<-ctx.Done()
		return nil
	}}
	req, done := executeAsync(h)
	<-started
	req <- svc.ChangeRequest{Cmd: svc.Stop}
	select {
	case r := <-done:
		if r[0].(bool) || r[1].(uint32) != 0 {
			t.Fatalf("停止后应以 0 退出，得到 ssec=%v code=%v", r[0], r[1])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("收到停止请求后服务未退出")
	}
}

func TestServiceHandlerRunReturnsNil(t *testing.T) {
	h := &serviceHandler{run: func(context.Context) error { return nil }}
	_, done := executeAsync(h)
	select {
	case r := <-done:
		if r[0].(bool) || r[1].(uint32) != 0 {
			t.Fatalf("正常结束应以 0 退出，得到 ssec=%v code=%v", r[0], r[1])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("run 结束后服务未退出")
	}
}

func TestServiceHandlerRunError(t *testing.T) {
	h := &serviceHandler{run: func(context.Context) error { return errors.New("boom") }}
	_, done := executeAsync(h)
	select {
	case r := <-done:
		if !r[0].(bool) || r[1].(uint32) != 1 {
			t.Fatalf("出错应以服务专用退出码 1 退出，得到 ssec=%v code=%v", r[0], r[1])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("run 出错后服务未退出")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./cmd/sss-agent/ -run ServiceHandler -v`
Expected: 编译失败，`undefined: serviceHandler`

- [ ] **Step 3: 实现 Windows 服务处理器**

`cmd/sss-agent/service_windows.go`：

```go
//go:build windows

package main

import (
	"context"

	"golang.org/x/sys/windows/svc"
)

// serviceName Windows 服务名，与安装脚本保持一致
const serviceName = "sss-agent"

// runAsService 若当前进程由 Windows 服务管理器启动，则以服务方式运行 run 并返回 true
func runAsService(run func(context.Context) error) (bool, error) {
	isService, err := svc.IsWindowsService()
	if err != nil || !isService {
		return false, err
	}
	return true, svc.Run(serviceName, &serviceHandler{run: run})
}

// serviceHandler 把服务管理器的停止请求转换为 ctx 取消
type serviceHandler struct {
	run func(context.Context) error
}

// Execute 实现 svc.Handler；run 出错时返回服务专用退出码 1，便于服务管理器按失败策略重启
func (h *serviceHandler) Execute(_ []string, req <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	status <- svc.Status{State: svc.StartPending}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- h.run(ctx) }()
	status <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}

	for {
		select {
		case err := <-done:
			status <- svc.Status{State: svc.StopPending}
			if err != nil {
				return true, 1
			}
			return false, 0
		case c := <-req:
			switch c.Cmd {
			case svc.Interrogate:
				status <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				status <- svc.Status{State: svc.StopPending}
				cancel()
				<-done
				return false, 0
			}
		}
	}
}
```

`cmd/sss-agent/service_other.go`：

```go
//go:build !windows

package main

import "context"

// runAsService 非 Windows 平台没有服务管理器，始终返回 false
func runAsService(func(context.Context) error) (bool, error) {
	return false, nil
}
```

- [ ] **Step 4: 改造 runAgent**

把 `cmd/sss-agent/main.go` 中的 `runAgent` 替换为下面两个函数（其余代码不动；按需增删 import，如 `log/slog`）：

```go
func runAgent(cmd *cobra.Command, _ []string) error {
	cfg, err := resolveConfig(cmd.Flags())
	if err != nil {
		return err
	}
	log, closeLog, err := logx.New(cfg.LogLevel, cfg.LogFile)
	if err != nil {
		return err
	}
	defer func() { _ = closeLog() }()

	run := func(ctx context.Context) error { return serve(ctx, cfg, log) }
	if isService, err := runAsService(run); isService || err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return run(ctx)
}

// serve 连接 Dashboard 并周期上报，直到 ctx 结束或 Dashboard 要求停止
func serve(ctx context.Context, cfg agent.Config, log *slog.Logger) error {
	client, err := agent.NewClient(cfg, collect.New(log), log, version)
	if err != nil {
		return err
	}
	if cfg.DetectCountry {
		dctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		cc, err := collect.DetectCountry(dctx, http.DefaultClient, collect.TraceURL)
		cancel()
		if err != nil {
			log.Warn("探测国家代码失败", "err", err)
		} else {
			client.SetCountry(cc)
		}
	}
	log.Info("Agent 启动", "version", version, "dashboard", cfg.Dashboard)
	err = client.Run(ctx)
	if errors.Is(err, agent.ErrStopped) {
		log.Info("Agent 已按 Dashboard 指令退出")
		return nil
	}
	return err
}
```

（`logx.New` 返回的 logger 类型以实际代码为准；若不是 `*slog.Logger`，`serve` 的参数类型跟随它。）

- [ ] **Step 5: 整理依赖并运行测试**

Run:
```bash
go mod tidy
go test ./cmd/sss-agent/ -v
GOOS=linux go vet ./cmd/sss-agent/ && GOOS=darwin go build -o /dev/null ./cmd/sss-agent && GOOS=freebsd go build -o /dev/null ./cmd/sss-agent && GOOS=linux GOARCH=arm GOARM=7 go build -o /dev/null ./cmd/sss-agent
golangci-lint run --timeout=5m ./...
go test ./...
```
Expected: 3 个 ServiceHandler 测试 PASS；交叉编译全部成功；lint 0 issues；`go.mod` 中 `golang.org/x/sys` 不再带 `// indirect`。

- [ ] **Step 6: 提交**

```bash
git add cmd/sss-agent go.mod go.sum
git commit -m "feat(agent): 支持作为 Windows 服务运行"
```

---

### Task 2: 重写 Linux 安装脚本

**Files:**
- Rewrite: `scripts/install-agent.sh`
- Create: `scripts/install-agent.test.sh`

**Interfaces:**
- Consumes: 发布资产命名契约（Global Constraints）；后台 Linux 安装命令格式。
- Produces: 安装结果——程序 `/usr/local/bin/sss-agent`，配置 `/etc/sss/sss-agent.yaml`（权限 600，目录 700），服务 `/etc/systemd/system/sss-agent.service`（`Restart=on-failure`）；`--uninstall` 全部移除。脚本在 `SSS_INSTALL_LIB=1` 时只定义函数不执行（供测试 source）。

- [ ] **Step 1: 写失败的测试**

`scripts/install-agent.test.sh`（LF 行尾）：

```bash
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
check "ExecStart" yes "$(grep -qxF 'ExecStart=/usr/local/bin/sss-agent --config /etc/sss/sss-agent.yaml' <<<"$unit" && echo yes || echo no)"

exit "$failed"
```

- [ ] **Step 2: 运行测试确认失败**

Run: `bash scripts/install-agent.test.sh`
Expected: 失败（旧脚本没有 `SSS_INSTALL_LIB`，source 时直接执行 main 并报「缺少必选参数」退出）。

- [ ] **Step 3: 重写安装脚本**

`scripts/install-agent.sh`（整体替换，LF 行尾）：

```bash
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
```

- [ ] **Step 4: 运行测试确认通过**

Run: `bash scripts/install-agent.test.sh`
Expected: 全部 `ok`，退出码 0。若本机有 shellcheck 再运行 `shellcheck -x scripts/install-agent.sh scripts/install-agent.test.sh`（本机没有则跳过，CI 会运行）。
再确认行尾：`git ls-files --eol scripts/install-agent.sh scripts/install-agent.test.sh` 显示 `w/lf`（新文件先 `git add` 再查）。

- [ ] **Step 5: 提交**

```bash
git add scripts/install-agent.sh scripts/install-agent.test.sh
git commit -m "feat(scripts): 重写 Linux 安装脚本，参数与后台安装命令一致"
```

---

### Task 3: 重写 Windows 安装脚本

**Files:**
- Rewrite: `scripts/install-agent.ps1`（CRLF + UTF-8 BOM）
- Create: `scripts/install-agent.test.ps1`（CRLF + UTF-8 BOM）

**Interfaces:**
- Consumes: 发布资产命名契约；后台 Windows 安装命令格式；Task 1 的 Windows 服务支持（服务名 `sss-agent`，run 出错时服务专用退出码 1）。
- Produces: 安装到 `%ProgramFiles%\sss-agent\`（`sss-agent.exe`、`sss-agent.yaml`、`logs\agent.log`）；服务 `sss-agent` 自动启动，失败自动重启；配置文件 ACL 仅 SYSTEM 与 Administrators。以点号方式加载（`. .\install-agent.ps1`）时只定义函数。

- [ ] **Step 1: 写失败的测试**

`scripts/install-agent.test.ps1`：

```powershell
# install-agent.ps1 的函数级测试
# 作者: ruan
# 用法: pwsh -File scripts/install-agent.test.ps1（也可用 powershell -File 在 Windows PowerShell 5.1 下运行）
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'install-agent.ps1')

$script:failed = 0
function Check([string]$Name, $Expected, $Actual) {
    if ("$Expected" -eq "$Actual") {
        Write-Host "ok   $Name"
    } else {
        Write-Host "FAIL ${Name}: 期望 [$Expected] 实际 [$Actual]"
        $script:failed = 1
    }
}
function Throws([scriptblock]$Block) {
    try { & $Block | Out-Null; return 'no' } catch { return 'yes' }
}

Check 'latest 地址' 'https://github.com/ruanun/simple-server-status/releases/latest/download/sss-agent_windows_amd64.zip' (Get-DownloadUrl 'latest' 'amd64')
Check '指定版本地址' 'https://github.com/ruanun/simple-server-status/releases/download/v2.0.0/sss-agent_windows_amd64.zip' (Get-DownloadUrl 'v2.0.0' 'amd64')

Check 'YAML 单引号转义' "'it''s'" (ConvertTo-YamlQuoted "it's")
$lines = (Get-AgentConfig 'https://s.example.com' "a'b" '$x y' 'C:\Program Files\sss-agent\logs\agent.log') -split "`n"
Check '配置 dashboard 行' 'True' ($lines -contains "dashboard: 'https://s.example.com'")
Check '配置 id 行' 'True' ($lines -contains "id: 'a''b'")
Check '配置 secret 行不展开' 'True' ($lines -contains "secret: '`$x y'")
Check '配置 log_file 行' 'True' ($lines -contains "log_file: 'C:\Program Files\sss-agent\logs\agent.log'")

# 后台生成的命令：& $f -Dashboard '<地址>' -Id '<ID>' -Secret '<密钥>'
Check '去掉末尾斜杠' 'https://s.example.com' (Assert-InstallArgs 'https://s.example.com/' 'abc' 'x')
Check '缺少 Dashboard 报错' 'yes' (Throws { Assert-InstallArgs '' 'abc' 'x' })
Check '缺少 Id 报错' 'yes' (Throws { Assert-InstallArgs 'https://a' '' 'x' })
Check '缺少 Secret 报错' 'yes' (Throws { Assert-InstallArgs 'https://a' 'abc' '' })
Check '非 http 地址报错' 'yes' (Throws { Assert-InstallArgs 'ftp://a' 'abc' 'x' })

# 两个脚本都必须以 UTF-8 BOM 开头，否则 Windows PowerShell 5.1 会按本地编码解析中文
foreach ($name in 'install-agent.ps1', 'install-agent.test.ps1') {
    $bytes = [IO.File]::ReadAllBytes((Join-Path $PSScriptRoot $name))
    Check "$name 带 UTF-8 BOM" '239,187,191' ($bytes[0..2] -join ',')
}

exit $script:failed
```

- [ ] **Step 2: 运行测试确认失败**

Run: `pwsh -NoProfile -File scripts/install-agent.test.ps1`
Expected: 失败（旧脚本没有 `Get-DownloadUrl` 等函数）。

- [ ] **Step 3: 重写安装脚本**

`scripts/install-agent.ps1`（整体替换；保存为 **UTF-8 带 BOM**、CRLF）：

```powershell
# Simple Server Status Agent 安装脚本（Windows 服务）
# 作者: ruan
# 用法（管理员 PowerShell）:
#   $f="$env:TEMP\install-agent.ps1"; iwr -useb <脚本地址> -OutFile $f; & $f -Dashboard <地址> -Id <ID> -Secret <密钥>
#   & $f -Uninstall
param(
    [string]$Dashboard,
    [string]$Id,
    [string]$Secret,
    [string]$Version = 'latest',
    [switch]$Uninstall
)

$ErrorActionPreference = 'Stop'
$Repo = 'ruanun/simple-server-status'
$ServiceName = 'sss-agent'
$InstallDir = Join-Path $env:ProgramFiles 'sss-agent'
$BinPath = Join-Path $InstallDir 'sss-agent.exe'
$ConfigPath = Join-Path $InstallDir 'sss-agent.yaml'
$LogPath = Join-Path $InstallDir 'logs\agent.log'

# Get-DownloadUrl 返回指定版本与架构的 Agent 压缩包地址
function Get-DownloadUrl([string]$Ver, [string]$Arch) {
    $file = "sss-agent_windows_$Arch.zip"
    if ($Ver -eq 'latest') { return "https://github.com/$Repo/releases/latest/download/$file" }
    return "https://github.com/$Repo/releases/download/$Ver/$file"
}

# ConvertTo-YamlQuoted 用 YAML 单引号包裹字符串，内部单引号写成两个
function ConvertTo-YamlQuoted([string]$Value) {
    return "'" + $Value.Replace("'", "''") + "'"
}

function Get-AgentConfig([string]$DashboardUrl, [string]$ServerId, [string]$ServerSecret, [string]$LogFile) {
    return @(
        '# Simple Server Status Agent 配置（由安装脚本生成）'
        "dashboard: $(ConvertTo-YamlQuoted $DashboardUrl)"
        "id: $(ConvertTo-YamlQuoted $ServerId)"
        "secret: $(ConvertTo-YamlQuoted $ServerSecret)"
        'detect_country: true'
        'log_level: info'
        "log_file: $(ConvertTo-YamlQuoted $LogFile)"
    ) -join "`n"
}

# Assert-InstallArgs 校验必填参数，返回去掉末尾斜杠的 Dashboard 地址
function Assert-InstallArgs([string]$DashboardUrl, [string]$ServerId, [string]$ServerSecret) {
    if (-not $DashboardUrl) { throw '缺少 -Dashboard' }
    if (-not $ServerId) { throw '缺少 -Id' }
    if (-not $ServerSecret) { throw '缺少 -Secret' }
    if ($DashboardUrl -notmatch '^https?://') { throw 'Dashboard 地址需以 http:// 或 https:// 开头' }
    return $DashboardUrl.TrimEnd('/')
}

function Assert-Admin {
    $principal = [Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()
    if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
        throw '请在管理员 PowerShell 中运行'
    }
}

# Remove-AgentService 停止并删除已有服务，等待服务管理器完成删除
function Remove-AgentService {
    if (-not (Get-Service -Name $ServiceName -ErrorAction SilentlyContinue)) { return }
    Stop-Service -Name $ServiceName -Force -ErrorAction SilentlyContinue
    sc.exe delete $ServiceName | Out-Null
    for ($i = 0; $i -lt 20 -and (Get-Service -Name $ServiceName -ErrorAction SilentlyContinue); $i++) {
        Start-Sleep -Milliseconds 500
    }
}

function Install-Agent {
    $dash = Assert-InstallArgs $Dashboard $Id $Secret
    Assert-Admin
    if ($env:PROCESSOR_ARCHITECTURE -ne 'AMD64') { throw "不支持的架构: $env:PROCESSOR_ARCHITECTURE" }

    [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
    $ProgressPreference = 'SilentlyContinue'
    $url = Get-DownloadUrl $Version 'amd64'
    $tmp = Join-Path ([IO.Path]::GetTempPath()) ("sss-agent-" + [guid]::NewGuid())
    New-Item -ItemType Directory -Path $tmp | Out-Null
    try {
        Write-Host "下载 Agent：$url"
        Invoke-WebRequest -UseBasicParsing -Uri $url -OutFile (Join-Path $tmp 'agent.zip')
        Expand-Archive -Path (Join-Path $tmp 'agent.zip') -DestinationPath $tmp -Force

        # 已安装时先删除旧服务，再覆盖程序与配置
        Remove-AgentService
        New-Item -ItemType Directory -Path (Split-Path $LogPath) -Force | Out-Null
        Copy-Item (Join-Path $tmp 'sss-agent.exe') $BinPath -Force
    } finally {
        Remove-Item $tmp -Recurse -Force -ErrorAction SilentlyContinue
    }

    [IO.File]::WriteAllText($ConfigPath, (Get-AgentConfig $dash $Id $Secret $LogPath), (New-Object Text.UTF8Encoding $false))
    # 配置含密钥：仅 SYSTEM（S-1-5-18）与 Administrators（S-1-5-32-544）可访问，用 SID 避免中文系统下组名不同
    icacls $ConfigPath /inheritance:r /grant:r '*S-1-5-18:F' '*S-1-5-32-544:F' | Out-Null

    New-Service -Name $ServiceName -DisplayName 'Simple Server Status Agent' -StartupType Automatic `
        -BinaryPathName "`"$BinPath`" --config `"$ConfigPath`"" | Out-Null
    # 异常退出（含非 0 退出码）时 5 秒后自动重启；Dashboard 要求停止时以 0 退出，不会被重启
    sc.exe failure $ServiceName reset= 86400 actions= restart/5000/restart/5000/restart/5000 | Out-Null
    sc.exe failureflag $ServiceName 1 | Out-Null
    Start-Service -Name $ServiceName

    Start-Sleep -Seconds 2
    if ((Get-Service -Name $ServiceName).Status -ne 'Running') {
        throw "Agent 未能启动，请查看日志：$LogPath"
    }
    Write-Host '安装完成，Agent 已启动' -ForegroundColor Green
    Write-Host "  查看状态: Get-Service $ServiceName"
    Write-Host "  查看日志: Get-Content '$LogPath' -Tail 50"
    Write-Host '  卸载:     & $f -Uninstall'
}

function Uninstall-Agent {
    Assert-Admin
    Remove-AgentService
    Remove-Item $InstallDir -Recurse -Force -ErrorAction SilentlyContinue
    Write-Host 'Agent 已卸载' -ForegroundColor Green
}

# 以点号方式加载（测试）时只定义函数
if ($MyInvocation.InvocationName -ne '.') {
    if ($Uninstall) { Uninstall-Agent } else { Install-Agent }
}
```

写入 BOM 的方法（任选其一，确保结果以 `EF BB BF` 开头、行尾为 CRLF）：

```powershell
pwsh -NoProfile -Command "foreach (`$f in 'scripts/install-agent.ps1','scripts/install-agent.test.ps1') { `$t = [IO.File]::ReadAllText(`$f) -replace '(?<!\r)\n', \"`r`n\"; [IO.File]::WriteAllText(`$f, `$t, (New-Object Text.UTF8Encoding `$true)) }"
```

- [ ] **Step 4: 运行测试确认通过**

Run:
```bash
pwsh -NoProfile -File scripts/install-agent.test.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/install-agent.test.ps1
```
Expected: 两种 PowerShell 下全部 `ok`、中文无乱码、退出码 0。
再确认：`git add` 后 `git ls-files --eol scripts/install-agent.ps1 scripts/install-agent.test.ps1` 显示 `i/lf w/crlf`（或 `i/crlf`，与仓库现有 ps1 一致即可）。

- [ ] **Step 5: 提交**

```bash
git add scripts/install-agent.ps1 scripts/install-agent.test.ps1
git commit -m "feat(scripts): 重写 Windows 安装脚本，注册为 Windows 服务"
```

---

### Task 4: 统一构建入口与包体预算

**Files:**
- Create: `web/scripts/check-bundle.mjs`
- Test: `web/scripts/check-bundle.test.mjs`
- Modify: `web/package.json`（scripts）
- Modify: `Makefile`
- Delete: `scripts/build-web.sh`、`scripts/build-web.ps1`、`scripts/build-dashboard.sh`、`scripts/build-docker.sh`、`scripts/build-docker.ps1`、`scripts/check-web-bundle.mjs`、`scripts/check-web-bundle.test.mjs`、`scripts/README.md`

**Interfaces:**
- Produces: `web/package.json` 脚本 `check:bundle`（检查 `internal/dashboard/web/dist` 入口资源 gzip 预算：JS ≤ 240 KB、CSS ≤ 16 KB）、`test:scripts`（`node --test` 运行 `web/scripts/*.test.mjs`）；删除 `build:prod`。
- Produces: Makefile 目标 `build-web`、`test-web`、`e2e`、`build-agent`、`build-dashboard`、`build`、`docker-build`（Task 5 的 Dockerfile 路径 `deployments/docker/Dockerfile`）、`VERSION` 变量注入 `main.version`。Task 6 的 CI 运行 `make build`。

- [ ] **Step 1: 写失败的测试**

`web/scripts/check-bundle.test.mjs`：

```js
// check-bundle.mjs 的测试：node --test scripts/check-bundle.test.mjs
import assert from 'node:assert/strict'
import { mkdirSync, mkdtempSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import path from 'node:path'
import { test } from 'node:test'

import { checkBundle, entryAssets } from './check-bundle.mjs'

function makeDist(html, files) {
  const dir = mkdtempSync(path.join(tmpdir(), 'sss-bundle-'))
  mkdirSync(path.join(dir, 'assets'))
  writeFileSync(path.join(dir, 'index.html'), html)
  for (const [name, content] of Object.entries(files)) writeFileSync(path.join(dir, name), content)
  return dir
}

const HTML = `<!doctype html><html><head>
<script type="module" crossorigin src="/assets/index-a.js"></script>
<link rel="modulepreload" crossorigin href="/assets/vendor-b.js">
<link rel="stylesheet" crossorigin href="/assets/index-c.css">
</head></html>`

test('entryAssets 只提取入口脚本与样式，属性顺序无关', () => {
  assert.deepEqual(entryAssets(HTML), { js: ['/assets/index-a.js'], css: ['/assets/index-c.css'] })
  assert.deepEqual(entryAssets('<script src="/x.js" type="module"></script><link href="/y.css" rel="stylesheet">'), {
    js: ['/x.js'],
    css: ['/y.css'],
  })
})

test('checkBundle 计算 gzip 大小并与预算比较', () => {
  const dir = makeDist(HTML, { 'assets/index-a.js': 'console.log(1)\n'.repeat(100), 'assets/index-c.css': 'a{}' })
  const ok = checkBundle(dir, { js: 10_000, css: 10_000 })
  assert.equal(ok.js.ok, true)
  assert.ok(ok.js.gzip > 0 && ok.js.gzip < 1600)
  const over = checkBundle(dir, { js: 1, css: 10_000 })
  assert.equal(over.js.ok, false)
  assert.equal(over.css.ok, true)
})

test('index.html 没有入口脚本时报错', () => {
  const dir = makeDist('<html></html>', {})
  assert.throws(() => checkBundle(dir), /入口脚本/)
})
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd web && node --test scripts/check-bundle.test.mjs`
Expected: FAIL，`Cannot find module .../check-bundle.mjs`

- [ ] **Step 3: 实现检查脚本**

`web/scripts/check-bundle.mjs`：

```js
// 入口包体预算检查：读取构建产物 index.html 引用的入口 JS / CSS，按 gzip 大小与预算比较
// 作者: ruan
// 用法：node scripts/check-bundle.mjs [dist 目录]
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'
import { gzipSync } from 'node:zlib'

/** 入口资源 gzip 预算（字节） */
export const BUDGETS = { js: 240 * 1024, css: 16 * 1024 }

function attr(tag, name) {
  return tag.match(new RegExp(`\\s${name}="([^"]+)"`))?.[1]
}

/** entryAssets 从 index.html 中提取入口脚本（type=module）与样式表路径 */
export function entryAssets(html) {
  const js = []
  const css = []
  for (const [tag] of html.matchAll(/<script\b[^>]*>/g)) {
    const src = attr(tag, 'src')
    if (attr(tag, 'type') === 'module' && src) js.push(src)
  }
  for (const [tag] of html.matchAll(/<link\b[^>]*>/g)) {
    const href = attr(tag, 'href')
    if (attr(tag, 'rel') === 'stylesheet' && href) css.push(href)
  }
  return { js, css }
}

/** checkBundle 返回入口 JS / CSS 的 gzip 总大小及是否在预算内 */
export function checkBundle(distDir, budgets = BUDGETS) {
  const { js, css } = entryAssets(readFileSync(path.join(distDir, 'index.html'), 'utf8'))
  if (js.length === 0) throw new Error('index.html 中未找到入口脚本')
  const measure = (files, budget) => {
    const gzip = files.reduce((n, f) => n + gzipSync(readFileSync(path.join(distDir, f.replace(/^\//, '')))).byteLength, 0)
    return { files, gzip, budget, ok: gzip <= budget }
  }
  return { js: measure(js, budgets.js), css: measure(css, budgets.css) }
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const here = path.dirname(fileURLToPath(import.meta.url))
  const dist = process.argv[2] ?? path.resolve(here, '../../internal/dashboard/web/dist')
  const result = checkBundle(dist)
  let ok = true
  for (const [kind, r] of Object.entries(result)) {
    console.log(`${kind}: ${(r.gzip / 1024).toFixed(1)} KB gzip / 预算 ${(r.budget / 1024).toFixed(0)} KB ${r.ok ? '✓' : '✗'}`)
    ok &&= r.ok
  }
  if (!ok) {
    console.error('入口资源超出预算')
    process.exit(1)
  }
}
```

- [ ] **Step 4: 更新 package.json 脚本**

`web/package.json` 的 `scripts`：删除 `"build:prod"`，新增两项（其余不变）：

```json
    "check:bundle": "node scripts/check-bundle.mjs",
    "test:scripts": "node --test \"scripts/*.test.mjs\"",
```

- [ ] **Step 5: 运行测试与真实构建检查**

Run: `cd web && pnpm test:scripts && pnpm build && pnpm check:bundle`
Expected: 3 个测试 PASS；check:bundle 输出 js 约 204 KB、css 约 10 KB，均 ✓，退出码 0。

- [ ] **Step 6: 重写 Makefile**

`Makefile` 整体替换为：

```makefile
# Simple Server Status Makefile
# 作者: ruan

.PHONY: help lint lint-fix fmt vet test test-coverage race build-web test-web e2e \
	build-agent build-dashboard build-dashboard-only build run-agent run-dashboard dev-web \
	clean tidy check gosec release docker-build docker-run

.DEFAULT_GOAL := help

GREEN  := \033[0;32m
YELLOW := \033[0;33m
NC     := \033[0m

BIN_DIR := bin
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
ifeq ($(OS),Windows_NT)
EXE := .exe
endif

help: ## 显示帮助信息
	@echo "$(GREEN)Simple Server Status - 可用命令:$(NC)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(YELLOW)%-20s$(NC) %s\n", $$1, $$2}'

lint: ## Go 代码检查
	golangci-lint run --timeout=5m ./...

lint-fix: ## Go 代码检查并自动修复
	golangci-lint run --timeout=5m --fix ./...

fmt: ## 格式化 Go 代码
	gofmt -s -w .

vet: ## 运行 go vet
	go vet ./...

test: ## 运行 Go 测试
	go test ./...

test-coverage: ## 运行 Go 测试并生成覆盖率报告
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

race: ## 运行竞态检测
	go test -race ./...

build-web: ## 构建前端（输出到 internal/dashboard/web/dist）
	cd web && pnpm install --frozen-lockfile && pnpm build

test-web: ## 前端类型检查、Lint 与单元测试
	cd web && pnpm typecheck && pnpm lint && pnpm test && pnpm test:scripts

e2e: ## 前端端到端测试（会构建前端与 Dashboard）
	cd web && pnpm test:e2e

build-agent: ## 构建 Agent
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/sss-agent$(EXE) ./cmd/sss-agent

build-dashboard: build-web build-dashboard-only ## 构建 Dashboard（包含前端）

build-dashboard-only: ## 仅构建 Dashboard（使用已有前端产物）
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/sss-dashboard$(EXE) ./cmd/sss-dashboard

build: build-agent build-dashboard ## 构建全部

run-agent: build-agent ## 运行 Agent（读取 ./sss-agent.yaml）
	./$(BIN_DIR)/sss-agent$(EXE)

run-dashboard: build-dashboard ## 运行 Dashboard
	./$(BIN_DIR)/sss-dashboard$(EXE)

dev-web: ## 启动前端开发服务器（代理到 127.0.0.1:8900）
	cd web && pnpm dev

clean: ## 清理构建产物
	rm -rf $(BIN_DIR) dist coverage.out coverage.html
	find internal/dashboard/web/dist -mindepth 1 ! -name '.gitkeep' -delete 2>/dev/null || true

tidy: ## 整理 Go 依赖
	go mod tidy

check: vet lint test test-web ## 运行全部检查

gosec: ## 安全扫描
	gosec ./...

release: ## 本地试打包（不发布）
	goreleaser release --snapshot --clean

docker-build: ## 构建 Docker 镜像 sssd:dev
	docker build -f deployments/docker/Dockerfile --build-arg VERSION=$(VERSION) -t sssd:dev .

docker-run: ## 运行 Docker 镜像（数据卷 sss-data）
	docker run --rm -p 8900:8900 -v sss-data:/app/data sssd:dev
```

- [ ] **Step 7: 删除旧构建脚本**

```bash
git rm scripts/build-web.sh scripts/build-web.ps1 scripts/build-dashboard.sh scripts/build-docker.sh scripts/build-docker.ps1 scripts/check-web-bundle.mjs scripts/check-web-bundle.test.mjs scripts/README.md
grep -rn "build-web\.\|build-dashboard\.sh\|build-docker\.\|check-web-bundle\|build:prod" --exclude-dir=node_modules --exclude-dir=.git --exclude-dir=docs . || echo "无残留引用"
```
Expected: 除 `.github/workflows/*.yml`、根目录 `Dockerfile`（分别由 Task 6、Task 5 处理）外无残留引用。

- [ ] **Step 8: 验证 Makefile 命令**

本机没有 make：逐条手动运行 `build-web`、`build-agent`、`build-dashboard-only` 目标中的命令（把 `$(VERSION)` 换成 `dev`、`$(EXE)` 换成 `.exe`），确认都成功，并运行 `bin/sss-agent.exe version` 输出 `dev`。完整的 `make build` 由 Task 6 的 CI 验证。

- [ ] **Step 9: 提交**

```bash
git add Makefile web/package.json web/scripts
git commit -m "build: 构建入口统一为 Makefile，新增前端入口包体预算检查，删除旧构建脚本"
```

---

### Task 5: Docker 镜像与部署示例

**Files:**
- Create: `deployments/docker/Dockerfile`
- Delete: 根目录 `Dockerfile`、`deployments/systemd/sssa.service`
- Rewrite: `.dockerignore`、`deployments/docker/docker-compose.yml`、`deployments/caddy/Caddyfile`

**Interfaces:**
- Consumes: 前端 `pnpm build` 输出到 `../internal/dashboard/web/dist`（相对 `web/`）。
- Produces: 镜像内程序 `/app/sss-dashboard`（ENTRYPOINT），数据卷 `/app/data`，端口 8900，构建参数 `VERSION`；支持 buildx 多架构（`TARGETOS`/`TARGETARCH`/`TARGETVARIANT`）。Task 6 的 release 工作流使用此 Dockerfile。

- [ ] **Step 1: 写 Dockerfile**

`deployments/docker/Dockerfile`：

```dockerfile
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
```

- [ ] **Step 2: 重写 .dockerignore（白名单）**

```
# Docker 构建上下文白名单：只发送构建所需文件
*
!go.mod
!go.sum
!cmd
!internal
!web
web/node_modules
web/test-results
web/playwright-report
internal/dashboard/web/dist
**/*_test.go
```

- [ ] **Step 3: 重写 docker-compose.yml 与 Caddyfile**

`deployments/docker/docker-compose.yml`：

```yaml
# Simple Server Status Dashboard
# 用法（仓库根目录）：docker compose -f deployments/docker/docker-compose.yml up -d
# 首次启动会在日志中打印 admin 随机密码：docker logs sss-dashboard
services:
  dashboard:
    image: ruanun/sssd:latest
    # 从源码构建时改用：
    # build:
    #   context: ../..
    #   dockerfile: deployments/docker/Dockerfile
    container_name: sss-dashboard
    restart: unless-stopped
    ports:
      - "8900:8900"
    environment:
      TZ: Asia/Shanghai
      # 首次初始化时使用的管理员密码（不设置则随机生成）
      # SSS_ADMIN_PASSWORD: "change-me-please"
      # 位于反向代理之后时填写代理的 IP 或 CIDR，才能正确识别客户端 IP（登录限流依赖它）
      # SSS_TRUSTED_PROXIES: "172.16.0.0/12"
    volumes:
      - sss-data:/app/data

volumes:
  sss-data:
```

`deployments/caddy/Caddyfile`：

```
# Caddy 反向代理示例：自动申请 HTTPS 证书，WebSocket 无需额外配置
# 把 status.example.com 换成你的域名；Dashboard 需设置 --trusted-proxies（或 SSS_TRUSTED_PROXIES）为 Caddy 所在地址
status.example.com {
	encode zstd gzip
	reverse_proxy 127.0.0.1:8900
}
```

- [ ] **Step 4: 删除旧文件**

```bash
git rm Dockerfile deployments/systemd/sssa.service
```

- [ ] **Step 5: 用干净上下文构建并运行镜像**

先清掉本地前端产物，证明镜像不依赖预先构建（Review Focus 5）：

```bash
find internal/dashboard/web/dist -mindepth 1 ! -name '.gitkeep' -delete
docker build -f deployments/docker/Dockerfile --build-arg VERSION=test -t sssd:plan3 .
docker run -d --name sss-plan3 -p 18900:8900 sssd:plan3
sleep 5
curl -fsS http://127.0.0.1:18900/api/public/site
curl -fsS http://127.0.0.1:18900/ | grep -q '<div id="root">' && echo "首页 OK"
docker logs sss-plan3 2>&1 | grep -i "密码"
docker exec sss-plan3 /app/sss-dashboard version
docker inspect --format '{{.State.Health.Status}}' sss-plan3   # 等待 start-period 后应为 healthy
docker rm -f sss-plan3 && docker rmi sssd:plan3
```
Expected: `/api/public/site` 返回 JSON；首页包含 root 节点（HTML 实际根节点 id 以 `web/index.html` 为准）；日志中能看到首次生成的 admin 密码；`version` 输出 `test`；健康检查 healthy。结束后删除容器与镜像，再运行 `cd web && pnpm build` 恢复本地前端产物。
若本机 Docker 不可用，报告 DONE_WITH_CONCERNS 并说明未验证。

- [ ] **Step 6: 提交**

```bash
git add .dockerignore deployments
git commit -m "build(docker): Dockerfile 移至 deployments/docker 并适配新目录，精简部署示例"
```

---

### Task 6: GoReleaser、CI 与发布工作流

**Files:**
- Rewrite: `.goreleaser.yml`
- Rewrite: `.github/workflows/ci.yml`
- Rewrite: `.github/workflows/release.yml`

**Interfaces:**
- Consumes: 发布资产命名契约；Task 2/3 的测试脚本；Task 4 的 `pnpm test:scripts`、`pnpm check:bundle`、`make build`；Task 5 的 Dockerfile。
- Produces: 推送 `v*` 标签即发布：Agent / Dashboard 各平台归档、`checksums.txt`、`install-agent.sh`、`install-agent.ps1`，以及多架构镜像 `ruanun/sssd`。

- [ ] **Step 1: 重写 .goreleaser.yml**

```yaml
# GoReleaser 配置：https://goreleaser.com
# 作者: ruan
# 发布前必须先构建前端（make build-web），前端产物会嵌入 Dashboard
version: 2

before:
  hooks:
    # 防止前端未构建就发布出没有界面的 Dashboard
    - sh -c 'test -f internal/dashboard/web/dist/index.html || { echo "缺少前端产物，请先运行 make build-web" >&2; exit 1; }'

builds:
  - id: agent
    main: ./cmd/sss-agent
    binary: sss-agent
    env: [CGO_ENABLED=0]
    goos: [linux, windows, darwin, freebsd]
    goarch: [amd64, arm64, arm]
    goarm: ["7"]
    ignore:
      - { goos: windows, goarch: arm }
      - { goos: windows, goarch: arm64 }
      - { goos: darwin, goarch: arm }
      - { goos: freebsd, goarch: arm }
    flags: [-trimpath]
    ldflags: [-s -w -X main.version={{ .Version }}]

  - id: dashboard
    main: ./cmd/sss-dashboard
    binary: sss-dashboard
    env: [CGO_ENABLED=0]
    goos: [linux, windows, darwin, freebsd]
    goarch: [amd64, arm64, arm]
    goarm: ["7"]
    ignore:
      - { goos: windows, goarch: arm }
      - { goos: windows, goarch: arm64 }
      - { goos: darwin, goarch: arm }
      - { goos: freebsd, goarch: arm }
    flags: [-trimpath]
    ldflags: [-s -w -X main.version={{ .Version }}]

# 归档名不含版本号且不包目录，安装脚本可直接使用 releases/latest/download/<文件名>
archives:
  - id: agent
    ids: [agent]
    formats: [tar.gz]
    format_overrides:
      - { goos: windows, formats: [zip] }
    name_template: "sss-agent_{{ .Os }}_{{ .Arch }}{{ if .Arm }}v{{ .Arm }}{{ end }}"
    wrap_in_directory: false
    files: [LICENSE, configs/sss-agent.yaml.example]

  - id: dashboard
    ids: [dashboard]
    formats: [tar.gz]
    format_overrides:
      - { goos: windows, formats: [zip] }
    name_template: "sss-dashboard_{{ .Os }}_{{ .Arch }}{{ if .Arm }}v{{ .Arm }}{{ end }}"
    wrap_in_directory: false
    files: [LICENSE]

checksum:
  name_template: checksums.txt
  algorithm: sha256

snapshot:
  version_template: "{{ incpatch .Version }}-next"

changelog:
  sort: asc
  use: github
  filters:
    exclude: ['^docs:', '^test:', '^chore:', '^ci:', 'Merge pull request', 'Merge branch']
  groups:
    - { title: 新功能, regexp: '^.*?feat(\([[:word:]]+\))??!?:.+$', order: 0 }
    - { title: 问题修复, regexp: '^.*?fix(\([[:word:]]+\))??!?:.+$', order: 1 }
    - { title: 性能优化, regexp: '^.*?perf(\([[:word:]]+\))??!?:.+$', order: 2 }
    - { title: 其他, order: 999 }

release:
  github:
    owner: ruanun
    name: simple-server-status
  prerelease: auto
  mode: replace
  name_template: "v{{ .Version }}"
  extra_files:
    - glob: scripts/install-agent.sh
    - glob: scripts/install-agent.ps1
  header: |
    ## 安装

    **Dashboard（Docker）**
    ```bash
    docker run -d --name sss-dashboard -p 8900:8900 -v sss-data:/app/data ruanun/sssd:{{ .Version }}
    docker logs sss-dashboard   # 查看首次生成的 admin 密码
    ```

    **Agent**：登录后台 → 服务器 → 新建或点「安装命令」，复制 Linux / Windows 一键命令到目标机器执行。

    完整文档见 [README](https://github.com/ruanun/simple-server-status/blob/master/README.md)。
```

- [ ] **Step 2: 本机验证 goreleaser**

```bash
goreleaser check
find internal/dashboard/web/dist -mindepth 1 ! -name '.gitkeep' -delete
goreleaser release --snapshot --clean --skip=publish 2>&1 | tail -5   # 期望：因前置检查失败而退出，提示「缺少前端产物」
(cd web && pnpm build)
goreleaser release --snapshot --clean --skip=publish
ls dist/*.tar.gz dist/*.zip
tar -tzf dist/sss-agent_linux_amd64.tar.gz
tar -tzf dist/sss-agent_linux_armv7.tar.gz | grep -x sss-agent
rm -rf dist
```
Expected: `check` 通过；无前端产物时失败；有产物时生成 `sss-agent_linux_amd64.tar.gz`、`sss-agent_linux_armv7.tar.gz`、`sss-agent_windows_amd64.zip`、`sss-dashboard_*` 等，归档根目录直接是 `sss-agent`。`sh -c` 前置钩子在 Windows 本机依赖 Git Bash 的 `sh`；若本机钩子因 shell 不可用而失败，把失败原因写进报告（CI 在 Linux 运行，不受影响），不要改写钩子。验证后删除 `dist/`。

- [ ] **Step 3: 重写 ci.yml**

```yaml
name: CI

on:
  push:
    branches: [master, dev-2.0]
  pull_request:
    branches: [master, dev-2.0]

permissions:
  contents: read

jobs:
  go:
    name: Go 检查与测试
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24.x'
      - name: 安装 golangci-lint
        run: curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b "$(go env GOPATH)/bin" v2.14.0
      - name: golangci-lint
        run: golangci-lint run --timeout=5m ./...
      - name: go vet
        run: go vet ./...
      - name: go test
        run: go test ./...
      - name: 交叉编译 Agent
        run: |
          GOOS=windows GOARCH=amd64 go build -o /dev/null ./cmd/sss-agent
          GOOS=darwin GOARCH=arm64 go build -o /dev/null ./cmd/sss-agent
          GOOS=freebsd GOARCH=amd64 go build -o /dev/null ./cmd/sss-agent
          GOOS=linux GOARCH=arm GOARM=7 go build -o /dev/null ./cmd/sss-agent

  web:
    name: 前端检查、构建与端到端测试
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24.x'
      - uses: pnpm/action-setup@v4
        with:
          version: 10
      - uses: actions/setup-node@v4
        with:
          node-version: '22'
          cache: pnpm
          cache-dependency-path: web/pnpm-lock.yaml
      - name: 安装依赖
        working-directory: web
        run: pnpm install --frozen-lockfile
      - name: 类型检查、Lint、单元测试
        working-directory: web
        run: pnpm typecheck && pnpm lint && pnpm test && pnpm test:scripts
      - name: 全新构建（make build）
        run: make build
      - name: 入口包体预算
        working-directory: web
        run: pnpm check:bundle
      - name: 安装 Playwright Chromium
        working-directory: web
        run: pnpm exec playwright install --with-deps chromium
      - name: 端到端测试
        working-directory: web
        run: pnpm test:e2e
      - name: 上传 Playwright 报告
        if: failure()
        uses: actions/upload-artifact@v4
        with:
          name: playwright-report
          path: web/playwright-report
          retention-days: 7

  scripts:
    name: 安装脚本（Linux）
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: shellcheck
        run: shellcheck -x scripts/install-agent.sh scripts/install-agent.test.sh
      - name: 函数级测试
        run: bash scripts/install-agent.test.sh

  windows:
    name: Windows（Agent 服务与安装脚本）
    runs-on: windows-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24.x'
      - name: Agent 测试
        run: go test ./cmd/sss-agent/...
      - name: 安装脚本测试（PowerShell 7）
        shell: pwsh
        run: ./scripts/install-agent.test.ps1
      - name: 安装脚本测试（Windows PowerShell 5.1）
        shell: powershell
        run: ./scripts/install-agent.test.ps1

  release-config:
    name: 检查 GoReleaser 配置
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: goreleaser/goreleaser-action@v6
        with:
          version: '~> v2'
          args: check

  security:
    name: 安全扫描
    runs-on: ubuntu-latest
    permissions:
      contents: read
      security-events: write
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24.x'
      - name: gosec
        uses: securego/gosec@master
        with:
          args: '-fmt sarif -out results.sarif ./...'
        continue-on-error: true
      - name: 上传 SARIF
        if: always()
        uses: github/codeql-action/upload-sarif@v4
        with:
          sarif_file: results.sarif
```

- [ ] **Step 4: 重写 release.yml**

```yaml
name: Release

on:
  push:
    tags: ['v*']

permissions:
  contents: write

jobs:
  release:
    name: 发布二进制
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24.x'
      - uses: pnpm/action-setup@v4
        with:
          version: 10
      - uses: actions/setup-node@v4
        with:
          node-version: '22'
          cache: pnpm
          cache-dependency-path: web/pnpm-lock.yaml
      - name: 构建前端
        run: make build-web
      - uses: goreleaser/goreleaser-action@v6
        with:
          version: '~> v2'
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}

  docker:
    name: 发布镜像
    runs-on: ubuntu-latest
    needs: release
    steps:
      - uses: actions/checkout@v4
      - uses: docker/setup-qemu-action@v3
      - uses: docker/setup-buildx-action@v3
      - uses: docker/login-action@v3
        with:
          username: ${{ secrets.DOCKER_USERNAME }}
          password: ${{ secrets.DOCKER_PASSWORD }}
      - id: meta
        uses: docker/metadata-action@v5
        with:
          images: ruanun/sssd
          tags: |
            type=semver,pattern={{version}}
            type=semver,pattern={{major}}.{{minor}}
            type=semver,pattern={{major}}
      - uses: docker/build-push-action@v6
        with:
          context: .
          file: deployments/docker/Dockerfile
          platforms: linux/amd64,linux/arm64,linux/arm/v7
          push: true
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
          build-args: VERSION=${{ steps.meta.outputs.version }}
          cache-from: type=gha
          cache-to: type=gha,mode=max
```

（`type=semver,pattern={{major}}` 对 `v0.x` 不生成标签属正常；正式版会同时带 `latest`，因为 metadata-action 对 semver 标签默认 `latest=auto`。）

- [ ] **Step 5: 校验 YAML 并提交**

Run: `pwsh -NoProfile -Command "Get-Content .github/workflows/ci.yml,.github/workflows/release.yml,.goreleaser.yml -Raw | Out-Null"`，并用 `goreleaser check` 再确认一次；若本机有 `actionlint` 则运行 `actionlint`。

```bash
git add .goreleaser.yml .github/workflows
git commit -m "ci: 适配新目录，发布无版本号归档与安装脚本，CI 覆盖前端、安装脚本与 Windows 服务"
```

---

### Task 7: 前端两条留存小问题

**Files:**
- Modify: `web/src/lib/api.ts:58-62, 79, 85`
- Modify: `web/src/features/admin/settings-page.tsx:145-220`（BackupPanel）
- Test: `web/src/lib/api.test.ts`

**Interfaces:**
- Consumes: 现有 `request()`、`localizeError()`、BackupPanel。
- Produces: 中文界面下，后端返回的中文错误原文（带细节）优先展示；英文界面仍按错误码翻译、无译文时回退原文。

- [ ] **Step 1: 写失败的测试**

在 `web/src/lib/api.test.ts` 的 `describe('api', ...)` 中追加：

```ts
  it('中文界面优先使用后端原文，保留错误细节', async () => {
    await i18n.changeLanguage('zh-CN')
    mockFetch({ 'POST /api/admin/import': () => ({ status: 500, error: { code: 'internal', message: '导入服务器失败' } }) })
    const err = await api.post('/api/admin/import', {}).catch((e: unknown) => e)
    expect((err as ApiError).code).toBe('internal')
    expect(errorMessage(err)).toBe('导入服务器失败')
  })

  it('中文界面后端无原文时仍按错误码翻译', async () => {
    await i18n.changeLanguage('zh-CN')
    mockFetch({ 'GET /api/x': () => ({ status: 404, error: { code: 'not_found' } }) })
    const err = await api.get('/api/x').catch((e: unknown) => e)
    expect(errorMessage(err)).toBe('请求的内容不存在')
  })
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd web && pnpm vitest run src/lib/api.test.ts`
Expected: 第一条 FAIL（期望「导入服务器失败」，实际「服务器内部错误」）。

- [ ] **Step 3: 修改 localizeError**

`web/src/lib/api.ts`：

```ts
/** localizeError 中文界面优先使用后端原文（后端错误为中文且带细节）；否则按错误码翻译，无译文时回退后端原文 */
function localizeError(code: string, serverMessage: string | undefined, fallback: string): string {
  if (serverMessage && i18n.language.startsWith('zh')) return serverMessage
  const key = `errors.codes.${code}`
  if (i18n.exists(key)) return i18n.t(key)
  return serverMessage ?? fallback
}
```

两处调用改为：

```ts
    throw new ApiError(0, 'network', localizeError('network', undefined, i18n.t('errors.network')))
```

```ts
    throw new ApiError(res.status, code, localizeError(code, json?.error?.message, i18n.t('errors.http', { status: res.status })))
```

- [ ] **Step 4: 修复导入确认弹窗关闭时数字闪为 0**

`settings-page.tsx` 的 BackupPanel：弹窗开关与待导入数据分开，关闭时保留数据直到下次选择文件。

```tsx
  // 解析成功、等待用户确认的导入内容；关闭弹窗时保留，避免关闭动画中数字变为 0
  const [pendingImport, setPendingImport] = useState<{ data: unknown; count: number } | null>(null)
  const [importOpen, setImportOpen] = useState(false)
```

`onFile` 末尾：

```tsx
    setPendingImport({ data: parsed, count: Array.isArray(servers) ? servers.length : 0 })
    setImportOpen(true)
```

`doImport`：

```tsx
  const doImport = async () => {
    if (!pendingImport) return
    setImportOpen(false)
    try {
      const r = await adminApi.importData(pendingImport.data)
      await qc.invalidateQueries()
      toast.success(t('settings.importDone', { count: r.servers }))
    } catch (err) {
      toast.error(errorMessage(err))
    }
  }
```

弹窗：

```tsx
      <AlertDialog open={importOpen} onOpenChange={setImportOpen}>
```

（描述行 `pendingImport?.count ?? 0` 不变。）jsdom 中没有关闭动画，此项不新增测试，靠现有 settings-page 测试保证行为不回退。

- [ ] **Step 5: 运行检查**

Run: `cd web && pnpm typecheck && pnpm lint && pnpm test && pnpm build`
Expected: 全部通过（测试数 89）。

- [ ] **Step 6: 提交**

```bash
git add web/src/lib/api.ts web/src/lib/api.test.ts web/src/features/admin/settings-page.tsx
git commit -m "fix(web): 中文界面保留后端错误细节，导入确认弹窗关闭时不再闪 0"
```

---

### Task 8: 文档

**Files:**
- Rewrite: `README.md`
- Create: `docs/getting-started.md`（覆盖旧文件）、`docs/deployment.md`、`docs/configuration.md`、`docs/api.md`
- Delete: `docs/2.0-PLAN.md`、`docs/fix-plan-2025-11-22.md`、`docs/maintenance.md`、`docs/troubleshooting.md`、`docs/api/`、`docs/architecture/`、`docs/deployment/`、`docs/development/`、`docs/superpowers/specs/2026-07-19-2.0-release-readiness-design.md`、`docs/superpowers/specs/2026-07-20-frontend-react-rewrite-design.md`、`docs/superpowers/plans/2026-07-19-2.0-release-readiness.md`、`docs/superpowers/plans/2026-07-20-frontend-react-rewrite.md`
- Rewrite: `AGENTS.md`
- Modify: `CHANGELOG.md`（顶部新增 2.0.0 条目）
- Modify: `claude.md`（未跟踪文件，即项目的 CLAUDE.md；只更新内容，**不要删除、不要 git add**）

**Interfaces:**
- Consumes: 前面所有任务的最终命令、路径、参数；Global Constraints 中的参数表；后端路由 `internal/dashboard/api/*.go`；协议 `internal/proto`；spec §3.6（浏览器 WS）。

所有文档内容必须以代码为准：写每个参数、路由、字段前先在代码中确认（`cmd/*/main.go`、`internal/agent/config.go`、`internal/dashboard/store/settings.go`、`internal/dashboard/api/*.go`、`internal/proto/*.go`）。风格：简洁、中文、命令可直接复制；不写营销语，不加 emoji。

- [ ] **Step 1: 删除旧文档**

```bash
git rm -r docs/2.0-PLAN.md docs/fix-plan-2025-11-22.md docs/maintenance.md docs/troubleshooting.md docs/api docs/architecture docs/deployment docs/development \
  docs/superpowers/specs/2026-07-19-2.0-release-readiness-design.md docs/superpowers/specs/2026-07-20-frontend-react-rewrite-design.md \
  docs/superpowers/plans/2026-07-19-2.0-release-readiness.md docs/superpowers/plans/2026-07-20-frontend-react-rewrite.md
```
（`docs/getting-started.md` 保留路径、下一步整体重写。）

- [ ] **Step 2: 写 4 篇文档**

各篇必须包含：

- `docs/getting-started.md` 快速开始：
  1. 用 Docker 启动 Dashboard 的一条命令；
  2. 用 `docker logs` 找首次 admin 密码，或用 `SSS_ADMIN_PASSWORD` 预设；
  3. 登录后台 → 新建服务器 → 复制安装命令（Linux / Windows）到目标机执行；
  4. 回到首页看到上线；
  5. 忘记密码用 `reset-password`（Docker 下是 `docker exec sss-dashboard /app/sss-dashboard reset-password`）。
- `docs/deployment.md` 部署：
  1. Docker / docker compose，数据卷 `/app/data`；
  2. 二进制 + systemd 部署 Dashboard，给出完整 unit 示例：`ExecStart=/usr/local/bin/sss-dashboard --data-dir /var/lib/sss`，`Restart=on-failure`；
  3. 反向代理：Caddy 示例引用 `deployments/caddy/Caddyfile`；Nginx 示例必须包含 WebSocket 升级头 `Upgrade`、`Connection`；说明要设置 `--trusted-proxies`；
  4. Agent 安装：一键脚本、`--version`、卸载、手动安装（下载归档 + 写 `/etc/sss/sss-agent.yaml` + 运行）；Windows 服务说明（安装位置、日志位置）；
  5. 升级与备份：替换二进制或镜像；备份 `data/`，或用后台导出（导出文件含密钥）；
  6. 从源码构建：`make build`，Windows 无 make 时列出等价的 pnpm / go 命令。
- `docs/configuration.md` 配置：
  1. Dashboard 参数、环境变量与默认值表；
  2. Agent YAML 键 / 参数 / 环境变量（`SSS_DASHBOARD` 等）表，写明优先级：命令行 > 环境变量 > YAML > 默认值；
  3. 默认配置文件查找顺序；
  4. 后台「设置」各项的含义：站点标题、公开价格、默认上报间隔、安装脚本地址（默认 `releases/latest/download`，可改为自建镜像地址，要求该地址下有 `install-agent.sh`、`install-agent.ps1`）；
  5. 服务器编辑弹窗四组字段的含义：基本 / 计费 / 流量 / 采集。
- `docs/api.md` API：
  1. 响应信封 `{"data"}` / `{"error":{"code","message"}}` 与错误码列表；
  2. 公开接口、登录与管理接口（方法、路径、鉴权、主要参数，如 metrics 的 `range`）；
  3. 浏览器 WebSocket `/api/public/ws` 的消息（snapshot / delta）；
  4. Agent WebSocket `/api/agent/ws` 协议信封 `{"v":1,"type","data"}` 与 hello / config / report / stop。

- [ ] **Step 3: 重写 README.md**

结构：
1. 一句话介绍 + 特性列表：实时状态、历史趋势（实时 / 1h / 6h / 24h / 7d）、月流量配额与到期 / 价格、分组与地区筛选、公开页可隐藏单机、管理后台、导入导出、亮 / 暗主题、中英文；
2. 快速开始：Docker 一条命令 + 添加 Agent 三步，链接 getting-started；
3. 文档链接（4 篇）；
4. 本地开发：`make help`，`make dev-web` 配合本地 Dashboard，`make check`，`make e2e`；
5. 技术栈一行；
6. License。

不写旧版本 / 1.x 内容，不放不存在的截图。

- [ ] **Step 4: 更新 AGENTS.md 与 claude.md**

- `AGENTS.md`：沿用现有 5 节结构，按新目录重写：
  - 目录：`cmd/sss-agent`、`cmd/sss-dashboard`、`internal/{proto,logx,cliutil,agent,dashboard/...}`、`web/`（React 19 + shadcn + Vite）、`scripts/install-agent.*`、`deployments/`；
  - 命令：`make build`、`make check`、`make test-web`、`make e2e`、`make dev-web`；
  - 风格：slog、前端组件 kebab-case 文件名、Vitest + Playwright；
  - 提交：Conventional Commits、中文。
- `claude.md`：更新「项目概述」为 Go + React。
  - 「代码提交前检查」改为：`golangci-lint run --timeout=5m ./...`、`go test ./...`、在 `web/` 下 `pnpm typecheck && pnpm lint && pnpm test`、构建 `make build`，Windows 无 make 时写出等价命令 `cd web && pnpm build`、`go build ./cmd/sss-agent`、`go build ./cmd/sss-dashboard`。
  - 「技术栈」按 spec §2.1 / §5.1 重写，去掉 Melody / Viper / Vue / Ant Design。
  - 分支改为 `master` / `dev-2.0`。
  - 保留「其他注意事项」中的 pnpm、简体中文、`.claude` 不提交三条。

- [ ] **Step 5: CHANGELOG 新增条目**

在 `CHANGELOG.md` 标题下方、最新条目之前插入：

```markdown
## [2.0.0] - 未发布

整体重写，不兼容 1.x 的配置与数据。

- Dashboard：gin + SQLite（纯 Go），分级降采样的历史数据（24 小时分钟级、7 天 10 分钟级），月流量累计与重置日，JWT 登录与限流，导入导出（含密钥，便于整机迁移）。
- Agent：WebSocket 长连接上报，参数由 Dashboard 下发，支持 Linux systemd 与 Windows 服务。
- 前端：React 19 + shadcn/ui 重写，极简单色风格，独立详情页与历史图表，中英文与亮暗主题。
- 部署：Docker 多架构镜像；安装脚本参数为 `--dashboard/--id/--secret`，后台可随时查看安装命令。
```

（若现有 CHANGELOG 的格式与此不同，按其现有标题层级调整。）

- [ ] **Step 6: 检查链接与残留引用**

```bash
grep -rn "cmd/agent\b\|cmd/dashboard\b\|pkg/model\|Vue\|Melody\|viper\|build-web\.sh\|public/dist\|serverAddr\|authSecret" README.md AGENTS.md docs/getting-started.md docs/deployment.md docs/configuration.md docs/api.md claude.md || echo "无残留"
grep -on "](\./[^)]*\|](docs/[^)]*" README.md docs/*.md
```
逐个确认相对链接指向的文件存在。

- [ ] **Step 7: 提交**

```bash
git add README.md AGENTS.md CHANGELOG.md docs
git commit -m "docs: 按重写后的结构重写 README 与文档，精简为四篇"
```
（`claude.md` 为未跟踪文件，不提交。）

---

### Task 9: 仓库清理（控制者执行，清单已经用户确认）

**Files:**
- Modify: `.gitignore`
- 删除未跟踪 / 被忽略的本地文件（不涉及 git 历史）

- [ ] **Step 1: 删除清单中的本地文件**

在仓库根目录执行（仅删除下列路径）：

```bash
rm -f agent.exe dashboard.exe agent-run.log dashboard-run.log dashboard.log coverage.out
rm -rf .logs .cache sss-dasboard internal/dashboard/public web/test-results bin
```

保留：`data/`、`sss-agent.yaml`、`.idea/`、`.codex/`、`.claude/`、`claude.md`（即项目 CLAUDE.md，Windows 下文件名大小写不敏感，spec §9 中「未跟踪 claude.md」指的就是它，因此不删）。

- [ ] **Step 2: 整理 .gitignore**

- 删除已不存在路径的规则：`web/dist-react/`、`web/.tmp/`、`sss-dashboard.yaml`。
- 新增：`.cache/`、`dist/`（goreleaser 本地输出）。

- [ ] **Step 3: 验证与提交**

```bash
git status --short --ignored | grep -v node_modules
cd web && pnpm build && cd .. && go build ./... && go test ./...
git add .gitignore
git commit -m "chore: 整理 .gitignore"
```
Expected: 忽略列表只剩 `.claude/`、`.idea/`、`.superpowers/`、`data/`、`sss-agent.yaml`、`internal/dashboard/web/dist/*` 产物、`.codex/` 下数据与 `web/node_modules`；构建与测试通过。
