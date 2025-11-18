# Repository Guidelines

## Project Structure & Module Organization
- `cmd/agent`：Agent 入口；`cmd/dashboard`：Dashboard 入口。
- `internal/agent`、`internal/dashboard`：核心业务与适配层。
- `pkg/model`：共享数据模型。
- `web/`：前端（Vue 3 + TypeScript + Vite）。
- `configs/*.yaml.example`：配置模板（复制为根目录同名文件使用）。
- `scripts/`、`deployments/`、`docs/`：构建脚本、部署清单与技术文档。

## Build, Test, and Development Commands
- `make build`：构建 Agent 与 Dashboard 二进制产物。
- `make build-agent` / `make build-dashboard`：分别构建两端；Dashboard 构建会打包前端。
- `make dev-web`：启动前端开发（等同 `cd web && pnpm run dev`）。
- `make run-agent` / `make run-dashboard`：本地运行二进制。
- `make test` / `make test-coverage` / `make race`：测试、覆盖率与竞态检测。
- `make lint` / `make check` / `make tidy`：静态检查、综合检查与依赖整理。

## Coding Style & Naming Conventions
- Go：使用 `gofmt`/`goimports` 保持格式；`golangci-lint` 与 `gosec` 做静态与安全检查。
- 包/文件名小写且语义清晰；错误显式处理；使用 `zap` 记录结构化日志。
- 前端：2 空格缩进；组件 `PascalCase.vue`，模块 `kebab-case`；TypeScript 优先、类型完备。

## Testing Guidelines
- Go 标准测试：文件命名 `*_test.go`，优先表驱动测试；覆盖关键路径。
- 生成覆盖率：`make test-coverage`（输出 `coverage.html`）。
- 前端当前未配置测试框架，新增建议采用 Vitest。

## Commit & Pull Request Guidelines
- 建议遵循 Conventional Commits：如 `feat(agent): add NIC stats`、`fix(dashboard): ws reconnect`。
- PR 必须：清晰描述、关联 Issue、包含测试说明；UI 变更附截图；涉及公共接口/行为需更新文档与示例配置。

## Security & Configuration Tips
- 勿提交真实密钥/证书。复制示例为本地配置：
  - Linux/macOS：`cp configs/sss-agent.yaml.example sss-agent.yaml`
  - Windows：`Copy-Item configs\sss-agent.yaml.example sss-agent.yaml`
- 生产部署按需调整端口、日志与鉴权；以最小权限运行二进制。


