# Repository Guidelines

## Project Structure & Module Organization
- `cmd/sss-agent`：Agent 入口（cobra，含 Windows 服务支持）；`cmd/sss-dashboard`：Dashboard 入口（`serve`、`reset-password`、`version`）。
- `internal/proto`：Agent 与 Dashboard 的 WebSocket 消息协议；`internal/logx`：slog 日志初始化；`internal/cliutil`：`SSS_*` 环境变量映射到命令行参数。
- `internal/agent`：Agent 配置与连接上报，`internal/agent/collect` 为系统指标采集（gopsutil）。
- `internal/dashboard`：`api`（gin 路由、浏览器与 Agent WebSocket）、`store`（SQLite 与迁移）、`hub`（实时状态与在线判定）、`history`（降采样）、`traffic`（月流量）、`auth`（JWT 与登录限流）、`web`（通过 embed 嵌入前端产物 `internal/dashboard/web/dist`）。
- `web/`：前端（React 19 + TypeScript + Vite + Tailwind v4 + shadcn/ui），构建产物输出到 `internal/dashboard/web/dist`；`src/features` 按页面划分，`src/components/ui` 为 shadcn 组件，`e2e/` 为 Playwright 用例。
- `scripts/install-agent.sh`、`scripts/install-agent.ps1`：Agent 一键安装脚本（附带测试脚本）。
- `deployments/`：Dockerfile、docker compose 与 Caddy 示例；`configs/sss-agent.yaml.example`：Agent 配置模板；`docs/`：快速开始、部署、配置、API 四篇文档。

## Build, Test, and Development Commands
- `make build`：构建前端并编译 `bin/sss-agent`、`bin/sss-dashboard`。
- `make check`：`go vet`、`golangci-lint`、`go test` 与前端检查（等于 `make vet lint test test-web`）。
- `make test-web`：在 `web/` 下执行 `pnpm typecheck && pnpm lint && pnpm test && pnpm test:scripts`。
- `make e2e`：构建前端后运行 Playwright 端到端测试。
- `make dev-web`：启动 Vite 开发服务器，`/api`（含 WebSocket）代理到 `SSS_DASHBOARD_TARGET` 或 `http://127.0.0.1:8900`；配合 `make run-dashboard` 使用。
- 其他命令见 `make help`。Windows 没有 make 时：`cd web && pnpm build`，`go build ./cmd/sss-agent`，`go build ./cmd/sss-dashboard`。

## Coding Style & Naming Conventions
- Go：`gofmt` 格式化，`golangci-lint`（v2）必须零告警；日志统一使用标准库 `log/slog`；错误显式处理并用 `%w` 包装。
- 前端：TypeScript strict，ESLint 零错误；文件名（含组件）一律 kebab-case，如 `server-card.tsx`；界面文案放在 `src/i18n` 的 zh-CN / en-US 中。
- 代码注释、日志与文档使用简体中文。

## Testing Guidelines
- Go：`*_test.go`，优先表驱动测试；API 测试使用 httptest + 临时 SQLite。
- 前端：Vitest + Testing Library，测试文件与源码同目录 `*.test.ts(x)`；端到端测试使用 Playwright（`web/e2e`），会编译并启动真实的 Dashboard 与 Agent。
- 安装脚本：`scripts/install-agent.test.sh`、`scripts/install-agent.test.ps1`。

## Commit & Pull Request Guidelines
- 遵循 Conventional Commits，描述使用中文，如 `feat(agent): 支持按前缀过滤网卡`、`fix(web): 修复离线卡片的时间显示`。
- 提交前确保 `make check` 通过；修改参数、接口或部署方式时同步更新 `docs/` 与示例配置。

## Security & Configuration Tips
- 勿提交真实密钥：根目录 `sss-agent.yaml`、`data/` 仅用于本地调试。复制模板使用：
  - Linux/macOS：`cp configs/sss-agent.yaml.example sss-agent.yaml`
  - Windows：`Copy-Item configs\sss-agent.yaml.example sss-agent.yaml`
- 数据库 `data/sss.db` 明文保存服务器密钥，部署时注意数据目录权限；导出文件同样包含密钥。
