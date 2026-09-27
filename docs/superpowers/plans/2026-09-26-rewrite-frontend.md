# 前端重写（计划 2/3）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 删除旧 `web/`，用 React + shadcn/ui 从零构建新前端：公开首页（汇总卡、筛选、卡片/列表视图）、服务器详情页（信息网格、分区、历史图表）、登录页、后台（服务器管理、设置），产物输出到 `internal/dashboard/web/dist` 由 Go 嵌入。

**Architecture:** Vite 单页应用。数据层用 TanStack Query：`/api/public/servers` 快照 + `/api/public/ws` 推送（snapshot/delta 合并进同一个 Query 缓存），断线自动退回 5 秒轮询。登录态为 localStorage 中的 JWT，经 `useSyncExternalStore` 驱动路由守卫；任何非登录接口返回 401 即清除登录态。首页只加载轻量组件（手写 SVG 迷你曲线），详情页、登录页、后台按路由懒加载，Recharts 仅在详情页图表中加载。

**Tech Stack:** React 19、Vite 6、TypeScript 5.6（strict）、Tailwind CSS v4、shadcn/ui（zinc，CLI 生成）、Recharts 3、TanStack Query 5、React Router 7（react-router-dom）、i18next、react-hook-form + zod、@dnd-kit、sonner、flag-icons、Vitest + Testing Library、Playwright。

**Spec:** `docs/superpowers/specs/2026-09-25-rewrite-design.md`（第 5 节前端；3.6 浏览器接口契约）

## Global Constraints

- 包管理器 pnpm；所有前端命令在 `web/` 目录执行，Go 命令在仓库根目录执行（Git Bash）。
- 视觉：风格 A 极简单色——背景 zinc-50 / 暗色纯黑；卡片白底 + 1px 边框，圆角约 10px，无阴影；只有在线点（绿）、离线标记（红）、国旗、阈值进度条（<70% 前景色，70–90% 琥珀，≥90% 红）、临近到期（≤7 天琥珀、已过期红）使用彩色；数字使用 tabular-nums；图表灰阶。
- 主题：亮 / 暗 / 跟随系统（localStorage `sss.theme`）；语言：zh-CN / en-US（localStorage `sss.locale`，默认按浏览器语言）；视图模式记忆（localStorage `sss.view`）；登录 token 存 localStorage `sss.token`。
- 后端契约（不可更改）：成功 `{"data":...}`，失败 `{"error":{"code","message"}}`；公开 WS `/api/public/ws?token=`，消息 `{"type":"snapshot"|"delta","data":ServerView[]}`；历史 `GET /api/public/servers/{id}/metrics?range=realtime|1h|6h|24h|7d`；管理接口见 Task 4 的 `adminApi`。
- 构建产物输出到 `../internal/dashboard/web/dist`，构建后必须保留该目录下的 `.gitkeep`（它已入库）。产物本身被 `.gitignore` 忽略，不提交。
- 代码注释与用户可见文案使用简体中文（英文文案只出现在 `en-US` 词典中）；作者署名（如需）写 `ruan`。
- 每个任务结束前在 `web/` 运行 `pnpm typecheck`、`pnpm lint`、`pnpm test`，均须通过（lint 允许 shadcn 生成文件产生的 `react-refresh/only-export-components` 警告，不允许错误）。涉及 Go 的任务额外运行 `go vet ./...`、`go test ./...`。
- 允许提交（用户已授权），提交信息用简体中文；不得改动 `data/`、根目录 `sss-agent.yaml`；不得执行 push/pull/merge/rebase/reset。

## Review Focus

- **后端返回空值**：从未上报的服务器 `metrics:null`、`static:null`，`traffic.limit:null`，`expire_at:null`——卡片、列表、详情页均不得报错，分别显示 `—`、`∞`、`长期`（Task 7、Task 9 测试）。
- **实时连接断开与恢复**：WS 断开后立即开始 5 秒轮询，重连成功后停止轮询；页面隐藏时断开、重新可见时重连（Task 6 测试）。
- **登录态失效**：任意管理接口返回 401 后清除 token，路由守卫立即跳回登录页，登录成功后回到原页面（Task 5、Task 10 测试）。
- **隐藏服务器不串到公开视图**：登录与未登录使用不同的 Query 缓存键，退出登录后首页不得继续显示隐藏服务器（Task 6 测试）。
- **窄屏无横向滚动**：390px 宽度下首页与后台服务器页 `scrollWidth <= clientWidth`（Task 13 e2e）。

---

### Task 1: 后端公开视图去掉 filter_id

**Files:**
- Modify: `internal/proto/proto.go`（`Report.FilterID` 的 JSON 标签）
- Modify: `internal/dashboard/api/view.go`（`view` 中复制 metrics 并清空 FilterID）
- Test: `internal/dashboard/api/public_test.go`（新增测试）

**Interfaces:**
- Produces: 公开接口与浏览器 WS 中的 `metrics` 不再包含 `filter_id` 字段；Agent 协议照常传输 `filter_id`（空值时省略）。

- [ ] **Step 1: 写失败测试**

在 `internal/dashboard/api/public_test.go` 末尾追加（若文件已导入 `bytes`、`context`、`proto`、`store` 则无需重复导入）：

```go
func TestPublicViewHidesFilterID(t *testing.T) {
	e := newTestEnv(t)
	s := e.addServer(store.Server{Name: "a"})
	e.api.Hub.Connect(s.ID, 2)
	e.api.handleReport(context.Background(), s.ID, proto.Report{CPU: 1, FilterID: "abc123"})
	_, body := e.do("GET", "/api/public/servers", "", nil)
	if bytes.Contains(body, []byte("filter_id")) {
		t.Fatalf("公开响应不应包含 filter_id: %s", body)
	}
	if !bytes.Contains(body, []byte(`"cpu":1`)) {
		t.Fatalf("公开响应应包含指标: %s", body)
	}
}
```

注意：`e.api.Hub.Connect` 与 `e.api.handleReport` 的签名以当前代码为准（最终修复波次可能调整过，例如 Connect 返回值或 handleReport 参数）；如签名不同，按现有调用方式改写这两行，测试意图不变。

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/dashboard/api/ -run TestPublicViewHidesFilterID -v`
Expected: FAIL，提示响应中包含 filter_id。

- [ ] **Step 3: 实现**

`internal/proto/proto.go` 中把 `Report` 的字段标签改为：

```go
	FilterID string `json:"filter_id,omitempty"`
```

`internal/dashboard/api/view.go`：在 `view` 函数构造 `ServerView` 时，把 `Metrics: live.Report` 改为 `Metrics: publicMetrics(live.Report)`，并在文件末尾新增：

```go
// publicMetrics 复制上报数据并清空仅供内部统计使用的字段，避免泄露网卡过滤配置
func publicMetrics(r *proto.Report) *proto.Report {
	if r == nil {
		return nil
	}
	m := *r
	m.FilterID = ""
	return &m
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go vet ./... && go test ./... -count=1`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/proto/proto.go internal/dashboard/api/view.go internal/dashboard/api/public_test.go
git commit -m "fix(api): 公开视图去掉 filter_id"
```

---

### Task 2: 删除旧前端并搭建新脚手架

**Files:**
- Delete: 整个 `web/`（含 `node_modules`、`dist`、`e2e`、旧配置，全部不保留）
- Create: `web/package.json`、`web/pnpm-workspace.yaml`、`web/.gitignore`、`web/index.html`、`web/public/favicon.svg`
- Create: `web/vite.config.ts`、`web/tsconfig.json`、`web/tsconfig.app.json`、`web/tsconfig.node.json`、`web/eslint.config.js`、`web/components.json`
- Create: `web/src/index.css`、`web/src/lib/utils.ts`、`web/src/main.tsx`、`web/src/vite-env.d.ts`、`web/src/test/setup.ts`、`web/src/smoke.test.ts`
- Create（CLI 生成）: `web/src/components/ui/*.tsx`

**Interfaces:**
- Produces: 可运行的 Vite 工程；`@/` 路径别名指向 `web/src`；`cn()` 工具；shadcn 组件 `button badge input label textarea dialog alert-dialog dropdown-menu select switch table tabs sheet separator skeleton toggle-group`；Tailwind 主题色 `ok/warn/bad/online`（类名 `bg-warn`、`text-bad`、`bg-online` 等）；`pnpm build` 输出到 `../internal/dashboard/web/dist` 且保留 `.gitkeep`。

- [ ] **Step 1: 删除旧前端**

```bash
cd /d/Code/m/simple-server-status
git rm -r -q web
rm -rf web
git status --short web | head
```

Expected: 最后一条命令无输出（旧前端已从索引和磁盘移除）。

- [ ] **Step 2: 写工程配置文件**

`web/package.json`：

```json
{
  "name": "sss-web",
  "private": true,
  "version": "0.0.0",
  "type": "module",
  "scripts": {
    "dev": "vite --host",
    "typecheck": "tsc -p tsconfig.app.json --noEmit && tsc -p tsconfig.node.json --noEmit",
    "lint": "eslint src e2e --no-error-on-unmatched-pattern",
    "test": "vitest run",
    "test:watch": "vitest",
    "build": "pnpm typecheck && vite build",
    "test:e2e": "pnpm build && playwright test"
  },
  "dependencies": {
    "@dnd-kit/core": "^6.3.1",
    "@dnd-kit/sortable": "^10.0.0",
    "@dnd-kit/utilities": "^3.2.2",
    "@hookform/resolvers": "^5.4.0",
    "@tanstack/react-query": "^5.101.2",
    "class-variance-authority": "^0.7.1",
    "clsx": "^2.1.1",
    "flag-icons": "^7.5.0",
    "i18next": "^26.3.6",
    "lucide-react": "^1.25.0",
    "react": "^19.2.7",
    "react-dom": "^19.2.7",
    "react-hook-form": "^7.82.0",
    "react-i18next": "^17.0.10",
    "react-router-dom": "^7.18.1",
    "recharts": "^3.10.1",
    "sonner": "^2.0.8",
    "tailwind-merge": "^3.6.0",
    "zod": "^4.4.3"
  },
  "devDependencies": {
    "@eslint/js": "^10.0.1",
    "@playwright/test": "^1.61.1",
    "@tailwindcss/vite": "^4.3.3",
    "@testing-library/jest-dom": "^6.9.1",
    "@testing-library/react": "^16.3.2",
    "@testing-library/user-event": "^14.6.1",
    "@types/node": "^22.10.1",
    "@types/react": "^19.2.17",
    "@types/react-dom": "^19.2.3",
    "@vitejs/plugin-react": "^5.2.0",
    "eslint": "^10.7.0",
    "eslint-plugin-react-hooks": "^7.1.1",
    "eslint-plugin-react-refresh": "^0.5.3",
    "globals": "^17.7.0",
    "jsdom": "^29.1.1",
    "tailwindcss": "^4.3.3",
    "tw-animate-css": "^1.4.0",
    "typescript": "~5.6.2",
    "typescript-eslint": "^8.64.0",
    "vite": "^6.0.1",
    "vitest": "^4.1.10"
  }
}
```

`web/pnpm-workspace.yaml`：

```yaml
allowBuilds:
  esbuild: true
```

`web/.gitignore`：

```
node_modules
dist
test-results
playwright-report
*.local
```

`web/index.html`：

```html
<!doctype html>
<html lang="zh-CN">
  <head>
    <meta charset="UTF-8" />
    <link rel="icon" type="image/svg+xml" href="/favicon.svg" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>Simple Server Status</title>
    <script>
      // 在首帧渲染前应用主题，避免暗色模式闪白
      ;(function () {
        try {
          var t = localStorage.getItem('sss.theme') || 'system'
          var dark = t === 'dark' || (t === 'system' && window.matchMedia('(prefers-color-scheme: dark)').matches)
          if (dark) document.documentElement.classList.add('dark')
        } catch (e) {}
      })()
    </script>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

`web/public/favicon.svg`：

```svg
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><rect width="32" height="32" rx="7" fill="#09090b"/><rect x="7" y="9" width="18" height="5" rx="1.5" fill="#fafafa"/><rect x="7" y="18" width="18" height="5" rx="1.5" fill="#fafafa"/><circle cx="21.5" cy="11.5" r="1.2" fill="#22c55e"/><circle cx="21.5" cy="20.5" r="1.2" fill="#22c55e"/></svg>
```

`web/vite.config.ts`：

```ts
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import type { Plugin } from 'vite'
import { defineConfig } from 'vitest/config'

const root = path.dirname(fileURLToPath(import.meta.url))
const outDir = path.resolve(root, '../internal/dashboard/web/dist')
const dashboardTarget = process.env.SSS_DASHBOARD_TARGET ?? 'http://127.0.0.1:8900'

// keepGitkeep 构建会清空输出目录，结束后重新写入 .gitkeep，保证 Go embed 目录在仓库中始终存在
function keepGitkeep(): Plugin {
  return {
    name: 'keep-gitkeep',
    apply: 'build',
    closeBundle() {
      fs.writeFileSync(path.join(outDir, '.gitkeep'), '')
    },
  }
}

export default defineConfig({
  plugins: [react(), tailwindcss(), keepGitkeep()],
  resolve: { alias: { '@': path.resolve(root, 'src') } },
  build: { outDir, emptyOutDir: true, chunkSizeWarningLimit: 700 },
  server: {
    proxy: { '/api': { target: dashboardTarget, changeOrigin: true, ws: true } },
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    include: ['src/**/*.test.{ts,tsx}'],
    css: false,
    clearMocks: true,
  },
})
```

`web/tsconfig.json`：

```json
{
  "files": [],
  "references": [{ "path": "./tsconfig.app.json" }, { "path": "./tsconfig.node.json" }],
  "compilerOptions": {
    "baseUrl": ".",
    "paths": { "@/*": ["./src/*"] }
  }
}
```

`web/tsconfig.app.json`：

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "useDefineForClassFields": true,
    "lib": ["ES2023", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "skipLibCheck": true,
    "moduleResolution": "Bundler",
    "allowImportingTsExtensions": true,
    "isolatedModules": true,
    "moduleDetection": "force",
    "jsx": "react-jsx",
    "noEmit": true,
    "strict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noFallthroughCasesInSwitch": true,
    "types": ["vite/client"],
    "baseUrl": ".",
    "paths": { "@/*": ["./src/*"] }
  },
  "include": ["src"]
}
```

`web/tsconfig.node.json`：

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "lib": ["ES2023", "DOM"],
    "module": "ESNext",
    "skipLibCheck": true,
    "moduleResolution": "Bundler",
    "allowImportingTsExtensions": true,
    "isolatedModules": true,
    "moduleDetection": "force",
    "noEmit": true,
    "strict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "types": ["node"]
  },
  "include": ["vite.config.ts", "playwright.config.ts", "e2e"]
}
```

`web/eslint.config.js`：

```js
import eslint from '@eslint/js'
import reactHooks from 'eslint-plugin-react-hooks'
import reactRefresh from 'eslint-plugin-react-refresh'
import globals from 'globals'
import tseslint from 'typescript-eslint'

export default tseslint.config(
  { ignores: ['dist', 'node_modules', 'playwright-report', 'test-results'] },
  eslint.configs.recommended,
  ...tseslint.configs.recommended,
  {
    files: ['src/**/*.{ts,tsx}'],
    languageOptions: { ecmaVersion: 2022, globals: { ...globals.browser } },
    plugins: { 'react-hooks': reactHooks, 'react-refresh': reactRefresh },
    rules: {
      ...reactHooks.configs.flat.recommended.rules,
      'react-refresh/only-export-components': ['warn', { allowConstantExport: true }],
    },
  },
  {
    files: ['e2e/**/*.ts'],
    languageOptions: { globals: { ...globals.node } },
  },
)
```

`web/components.json`：

```json
{
  "$schema": "https://ui.shadcn.com/schema.json",
  "style": "new-york",
  "rsc": false,
  "tsx": true,
  "tailwind": {
    "config": "",
    "css": "src/index.css",
    "baseColor": "zinc",
    "cssVariables": true,
    "prefix": ""
  },
  "aliases": {
    "components": "@/components",
    "utils": "@/lib/utils",
    "ui": "@/components/ui",
    "lib": "@/lib",
    "hooks": "@/hooks"
  },
  "iconLibrary": "lucide"
}
```

- [ ] **Step 3: 写样式、入口与测试初始化**

`web/src/index.css`：

```css
@import 'tailwindcss';
@import 'tw-animate-css';
@import 'flag-icons/css/flag-icons.min.css';

@custom-variant dark (&:is(.dark *));

@theme inline {
  --radius-sm: calc(var(--radius) - 4px);
  --radius-md: calc(var(--radius) - 2px);
  --radius-lg: var(--radius);
  --radius-xl: calc(var(--radius) + 4px);
  --color-background: var(--background);
  --color-foreground: var(--foreground);
  --color-card: var(--card);
  --color-card-foreground: var(--card-foreground);
  --color-popover: var(--popover);
  --color-popover-foreground: var(--popover-foreground);
  --color-primary: var(--primary);
  --color-primary-foreground: var(--primary-foreground);
  --color-secondary: var(--secondary);
  --color-secondary-foreground: var(--secondary-foreground);
  --color-muted: var(--muted);
  --color-muted-foreground: var(--muted-foreground);
  --color-accent: var(--accent);
  --color-accent-foreground: var(--accent-foreground);
  --color-destructive: var(--destructive);
  --color-border: var(--border);
  --color-input: var(--input);
  --color-ring: var(--ring);
  --color-warn: var(--warn);
  --color-bad: var(--bad);
  --color-online: var(--online);
  --font-sans: ui-sans-serif, system-ui, -apple-system, 'Segoe UI', 'PingFang SC', 'Microsoft YaHei', sans-serif;
  --font-mono: ui-monospace, 'Cascadia Code', 'JetBrains Mono', Consolas, monospace;
}

:root {
  --radius: 0.625rem;
  --background: oklch(0.985 0 0);
  --foreground: oklch(0.141 0.005 285.823);
  --card: oklch(1 0 0);
  --card-foreground: oklch(0.141 0.005 285.823);
  --popover: oklch(1 0 0);
  --popover-foreground: oklch(0.141 0.005 285.823);
  --primary: oklch(0.21 0.006 285.885);
  --primary-foreground: oklch(0.985 0 0);
  --secondary: oklch(0.967 0.001 286.375);
  --secondary-foreground: oklch(0.21 0.006 285.885);
  --muted: oklch(0.967 0.001 286.375);
  --muted-foreground: oklch(0.552 0.016 285.938);
  --accent: oklch(0.967 0.001 286.375);
  --accent-foreground: oklch(0.21 0.006 285.885);
  --destructive: oklch(0.577 0.245 27.325);
  --border: oklch(0.92 0.004 286.32);
  --input: oklch(0.92 0.004 286.32);
  --ring: oklch(0.705 0.015 286.067);
  --warn: oklch(0.769 0.188 70.08);
  --bad: oklch(0.637 0.237 25.331);
  --online: oklch(0.723 0.219 149.579);
}

.dark {
  --background: oklch(0 0 0);
  --foreground: oklch(0.985 0 0);
  --card: oklch(0.141 0.005 285.823);
  --card-foreground: oklch(0.985 0 0);
  --popover: oklch(0.21 0.006 285.885);
  --popover-foreground: oklch(0.985 0 0);
  --primary: oklch(0.92 0.004 286.32);
  --primary-foreground: oklch(0.21 0.006 285.885);
  --secondary: oklch(0.274 0.006 286.033);
  --secondary-foreground: oklch(0.985 0 0);
  --muted: oklch(0.274 0.006 286.033);
  --muted-foreground: oklch(0.705 0.015 286.067);
  --accent: oklch(0.274 0.006 286.033);
  --accent-foreground: oklch(0.985 0 0);
  --destructive: oklch(0.704 0.191 22.216);
  --border: oklch(1 0 0 / 10%);
  --input: oklch(1 0 0 / 15%);
  --ring: oklch(0.552 0.016 285.938);
}

@layer base {
  * {
    @apply border-border outline-ring/50;
  }
  body {
    @apply bg-background font-sans text-foreground antialiased;
  }
}

@utility tabular {
  font-variant-numeric: tabular-nums;
}
```

`web/src/lib/utils.ts`：

```ts
import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

/** cn 合并 className，后者覆盖前者的 Tailwind 冲突类 */
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}
```

`web/src/vite-env.d.ts`：

```ts
/// <reference types="vite/client" />
```

`web/src/main.tsx`（临时入口，Task 5 会替换）：

```tsx
import '@/index.css'

import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <div className="p-6 text-sm">Simple Server Status</div>
  </StrictMode>,
)
```

`web/src/test/setup.ts`（Task 3、Task 6 会扩充）：

```ts
import '@testing-library/jest-dom/vitest'

import { cleanup } from '@testing-library/react'
import { afterEach, beforeEach, vi } from 'vitest'

class ResizeObserverStub {
  observe() {}
  unobserve() {}
  disconnect() {}
}

beforeEach(() => {
  localStorage.clear()
  vi.stubGlobal('ResizeObserver', ResizeObserverStub)
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addEventListener: () => {},
    removeEventListener: () => {},
    addListener: () => {},
    removeListener: () => {},
    dispatchEvent: () => false,
  }))
  // Radix 组件在 jsdom 中依赖以下 DOM API
  Element.prototype.scrollIntoView = () => {}
  Element.prototype.hasPointerCapture = () => false
  Element.prototype.releasePointerCapture = () => {}
})

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})
```

`web/src/smoke.test.ts`：

```ts
import { describe, expect, it } from 'vitest'

import { cn } from '@/lib/utils'

describe('cn', () => {
  it('合并并覆盖冲突的 Tailwind 类', () => {
    expect(cn('px-2 text-sm', 'px-4')).toBe('text-sm px-4')
  })
})
```

- [ ] **Step 4: 安装依赖并生成 shadcn 组件**

```bash
cd /d/Code/m/simple-server-status/web
pnpm install
pnpm dlx shadcn@4.21.0 add button badge input label textarea dialog alert-dialog dropdown-menu select switch table tabs sheet separator skeleton toggle-group --yes
ls src/components/ui
```

Expected: `src/components/ui` 下出现上述组件文件，`package.json` 被 CLI 追加了所需的 Radix 依赖。

若 CLI 拒绝 `components.json`（例如该版本不再支持 `new-york` 字段或要求先 init）：运行 `pnpm dlx shadcn@4.21.0 init --base-color zinc --yes` 完成初始化，然后**恢复**本任务写入的 `src/index.css`（CLI 可能改写它），再执行上面的 `add` 命令。在报告中写明实际执行的命令。生成的组件文件保持原样，不手动修改。

- [ ] **Step 5: 运行检查与构建**

```bash
pnpm typecheck
pnpm lint
pnpm test
pnpm exec vite build
ls ../internal/dashboard/web/dist
git -C .. status --short internal/dashboard/web/dist
```

Expected: 测试 PASS；构建成功；`dist` 下有 `index.html`、`assets/`、`.gitkeep`；最后一条命令无输出（`.gitkeep` 未被删除，产物被忽略）。

- [ ] **Step 6: 提交**

```bash
cd /d/Code/m/simple-server-status
git add -A web
git status --short web | head -40
git commit -m "chore(web): 删除旧前端并搭建 React + shadcn 新脚手架"
```

确认 `node_modules` 未被暂存（已被 `web/.gitignore` 忽略）。

---

### Task 3: 类型、格式化、指标计算与国际化

**Files:**
- Create: `web/src/lib/types.ts`、`web/src/lib/format.ts`、`web/src/lib/server.ts`
- Create: `web/src/i18n/zh-CN.ts`、`web/src/i18n/en-US.ts`、`web/src/i18n/index.ts`、`web/src/i18n/use-lang.ts`
- Create: `web/src/test/fixtures.ts`
- Modify: `web/src/test/setup.ts`（引入 i18n 并在每个测试前切回中文）
- Test: `web/src/lib/format.test.ts`、`web/src/lib/server.test.ts`

**Interfaces:**
- Produces:
  - 类型：`Hello`、`Disk`、`Metrics`、`Traffic`、`ServerView`、`Point`、`Site`、`Settings`、`AdminServer`、`ServerInput`、`Range`、`BillingCycle`、`TrafficMode`
  - `format.ts`：`type Lang = 'zh-CN' | 'en-US'`、`formatBytes(n)`、`formatSpeed(n)`、`percent(used, total)`、`formatPercent(p)`、`formatDuration(sec, lang)`、`daysUntil(expireAt, nowSec)`、`formatAgo(ts, nowSec, lang)`、`formatDate(ts, lang)`
  - `server.ts`：`type Level`、`usageLevel(p)`、`cpuPct(s)`、`memPct(s)`、`diskPct(s)`、`trafficPct(t)`、`expiryLevel(days)`、`osLabel(s)`、`sortServers(list)`
  - i18n：默认导出 i18next 实例；`setLang(lang)`；`useLang(): Lang`；词典类型 `Dict`
  - 测试：`makeServer(overrides)`、`makeMetrics(overrides)`

- [ ] **Step 1: 写类型定义**

`web/src/lib/types.ts`：

```ts
/** 与后端 JSON 契约一一对应的类型（字段均为 snake_case） */

export interface Hello {
  os: string
  platform: string
  platform_version: string
  kernel: string
  arch: string
  virtualization: string
  cpu_model: string
  cpu_cores: number
  mem_total: number
  swap_total: number
  disk_total: number
  agent_version: string
  country: string
}

export interface Disk {
  mount: string
  fstype: string
  total: number
  used: number
}

export interface Metrics {
  ts: number
  cpu: number
  load1: number
  load5: number
  load15: number
  mem_used: number
  swap_used: number
  disk_used: number
  disk_total: number
  disks: Disk[]
  net_in_speed: number
  net_out_speed: number
  net_in_total: number
  net_out_total: number
  procs: number
  tcp: number
  udp: number
  uptime: number
}

export type TrafficMode = 'sum' | 'in' | 'out'

export interface Traffic {
  in: number
  out: number
  used: number
  limit: number | null
  mode: TrafficMode
  reset_day: number
  period: string
}

export type BillingCycle = '' | 'monthly' | 'quarterly' | 'yearly' | 'once'

export interface ServerView {
  id: string
  name: string
  group: string
  country: string
  sort: number
  hidden: boolean
  online: boolean
  last_seen: number
  static: Hello | null
  metrics: Metrics | null
  traffic: Traffic
  expire_at: number | null
  price?: number | null
  currency?: string
  billing_cycle?: BillingCycle
}

export interface Point {
  ts: number
  cpu: number
  mem: number
  disk: number
  net_in: number
  net_out: number
  load1: number
  tcp: number
}

export type Range = 'realtime' | '1h' | '6h' | '24h' | '7d'
export const RANGES: Range[] = ['realtime', '1h', '6h', '24h', '7d']

export interface Site {
  site_title: string
  show_price: boolean
}

export interface Settings {
  site_title: string
  show_price: boolean
  default_report_interval: number
  install_script_base: string
}

export interface ServerInput {
  name: string
  group: string
  country: string
  hidden: boolean
  price: number | null
  currency: string
  billing_cycle: BillingCycle
  expire_at: number | null
  traffic_limit: number | null
  traffic_mode: TrafficMode
  traffic_reset_day: number
  report_interval: number
  nic_include: string[]
  nic_exclude: string[]
  mount_exclude: string[]
}

export interface AdminServer extends ServerInput {
  id: string
  secret: string
  sort: number
  static_info: Hello | null
  last_ip: string
  last_seen: number
  created_at: number
  updated_at: number
  online: boolean
}
```

- [ ] **Step 2: 写格式化与指标计算的失败测试**

`web/src/test/fixtures.ts`：

```ts
import type { Hello, Metrics, ServerView } from '@/lib/types'

export function makeHello(overrides: Partial<Hello> = {}): Hello {
  return {
    os: 'linux',
    platform: 'debian',
    platform_version: '12',
    kernel: '6.1.0',
    arch: 'x86_64',
    virtualization: 'kvm',
    cpu_model: 'AMD EPYC 7B13',
    cpu_cores: 4,
    mem_total: 2 * 1024 ** 3,
    swap_total: 1024 ** 3,
    disk_total: 40 * 1024 ** 3,
    agent_version: '2.0.0',
    country: 'HK',
    ...overrides,
  }
}

export function makeMetrics(overrides: Partial<Metrics> = {}): Metrics {
  return {
    ts: 1_700_000_000,
    cpu: 12,
    load1: 0.5,
    load5: 0.4,
    load15: 0.3,
    mem_used: 1024 ** 3,
    swap_used: 0,
    disk_used: 10 * 1024 ** 3,
    disk_total: 40 * 1024 ** 3,
    disks: [{ mount: '/', fstype: 'ext4', total: 40 * 1024 ** 3, used: 10 * 1024 ** 3 }],
    net_in_speed: 1024,
    net_out_speed: 2048,
    net_in_total: 10 * 1024 ** 3,
    net_out_total: 5 * 1024 ** 3,
    procs: 120,
    tcp: 30,
    udp: 4,
    uptime: 3 * 86400 + 2 * 3600,
    ...overrides,
  }
}

export function makeServer(overrides: Partial<ServerView> = {}): ServerView {
  return {
    id: 'srv1',
    name: 'hk-01',
    group: 'HK',
    country: 'HK',
    sort: 0,
    hidden: false,
    online: true,
    last_seen: 1_700_000_000,
    static: makeHello(),
    metrics: makeMetrics(),
    traffic: { in: 300 * 1024 ** 3, out: 100 * 1024 ** 3, used: 400 * 1024 ** 3, limit: 1024 ** 4, mode: 'sum', reset_day: 1, period: '2026-09-01' },
    expire_at: null,
    ...overrides,
  }
}
```

`web/src/lib/format.test.ts`：

```ts
import { describe, expect, it } from 'vitest'

import { daysUntil, formatAgo, formatBytes, formatDuration, formatPercent, formatSpeed, percent } from './format'

describe('formatBytes', () => {
  it.each([
    [0, '0 B'],
    [-5, '0 B'],
    [512, '512 B'],
    [1024, '1.0 KB'],
    [1536, '1.5 KB'],
    [150 * 1024 ** 3, '150 GB'],
    [2.5 * 1024 ** 4, '2.5 TB'],
  ])('%d → %s', (n, want) => {
    expect(formatBytes(n)).toBe(want)
  })

  it('formatSpeed 附加 /s', () => {
    expect(formatSpeed(1024)).toBe('1.0 KB/s')
  })
})

describe('percent', () => {
  it('计算百分比并限制在 0–100', () => {
    expect(percent(50, 200)).toBe(25)
    expect(percent(1, 0)).toBe(0)
    expect(percent(300, 100)).toBe(100)
    expect(formatPercent(12.6)).toBe('13%')
  })
})

describe('formatDuration', () => {
  it('中文最多两级', () => {
    expect(formatDuration(23 * 86400 + 5 * 3600 + 10, 'zh-CN')).toBe('23 天 5 小时')
    expect(formatDuration(86400, 'zh-CN')).toBe('1 天')
    expect(formatDuration(2 * 3600 + 3 * 60, 'zh-CN')).toBe('2 小时 3 分')
    expect(formatDuration(59, 'zh-CN')).toBe('0 分钟')
  })
  it('英文缩写', () => {
    expect(formatDuration(23 * 86400 + 5 * 3600, 'en-US')).toBe('23d 5h')
    expect(formatDuration(3 * 60, 'en-US')).toBe('3m')
  })
})

describe('daysUntil', () => {
  it('向上取整，无到期时间返回 null', () => {
    expect(daysUntil(null, 0)).toBeNull()
    expect(daysUntil(2 * 86400, 0)).toBe(2)
    expect(daysUntil(100, 0)).toBe(1)
    expect(daysUntil(0, 86400)).toBe(-1)
  })
})

describe('formatAgo', () => {
  it('按量级输出', () => {
    expect(formatAgo(95, 100, 'zh-CN')).toBe('5 秒前')
    expect(formatAgo(0, 7200, 'en-US')).toBe('2h ago')
    expect(formatAgo(0, 3 * 86400, 'zh-CN')).toBe('3 天前')
  })
})
```

`web/src/lib/server.test.ts`：

```ts
import { describe, expect, it } from 'vitest'

import { makeHello, makeMetrics, makeServer } from '@/test/fixtures'

import { cpuPct, diskPct, expiryLevel, memPct, osLabel, sortServers, trafficPct, usageLevel } from './server'

describe('usageLevel', () => {
  it('70/90 为阈值', () => {
    expect(usageLevel(69.9)).toBe('ok')
    expect(usageLevel(70)).toBe('warn')
    expect(usageLevel(89.9)).toBe('warn')
    expect(usageLevel(90)).toBe('bad')
  })
})

describe('指标百分比', () => {
  it('根据静态信息与上报计算', () => {
    const s = makeServer({ static: makeHello({ mem_total: 1000 }), metrics: makeMetrics({ cpu: 150, mem_used: 250, disk_used: 30, disk_total: 120 }) })
    expect(cpuPct(s)).toBe(100)
    expect(memPct(s)).toBe(25)
    expect(diskPct(s)).toBe(25)
  })
  it('从未上报时为 0', () => {
    const s = makeServer({ static: null, metrics: null })
    expect(cpuPct(s)).toBe(0)
    expect(memPct(s)).toBe(0)
    expect(diskPct(s)).toBe(0)
  })
})

describe('trafficPct', () => {
  it('无配额返回 null', () => {
    const base = makeServer().traffic
    expect(trafficPct({ ...base, limit: null })).toBeNull()
    expect(trafficPct({ ...base, limit: 0 })).toBeNull()
    expect(trafficPct({ ...base, used: 50, limit: 200 })).toBe(25)
  })
})

describe('expiryLevel', () => {
  it('7 天内琥珀、过期红色、长期正常', () => {
    expect(expiryLevel(null)).toBe('ok')
    expect(expiryLevel(30)).toBe('ok')
    expect(expiryLevel(7)).toBe('warn')
    expect(expiryLevel(0)).toBe('bad')
  })
})

describe('osLabel', () => {
  it('系统 · 虚拟化 · 架构', () => {
    expect(osLabel(makeServer())).toBe('debian 12 · kvm · x86_64')
    expect(osLabel(makeServer({ static: makeHello({ platform: '', virtualization: '' }) }))).toBe('linux · x86_64')
    expect(osLabel(makeServer({ static: null }))).toBe('')
  })
})

describe('sortServers', () => {
  it('按 sort 再按名称排序且不修改原数组', () => {
    const list = [makeServer({ id: 'b', name: 'b', sort: 1 }), makeServer({ id: 'c', name: 'c', sort: 0 }), makeServer({ id: 'a', name: 'a', sort: 0 })]
    expect(sortServers(list).map((s) => s.id)).toEqual(['a', 'c', 'b'])
    expect(list[0].id).toBe('b')
  })
})
```

- [ ] **Step 3: 运行测试确认失败**

Run: `cd web && pnpm test`
Expected: FAIL（无法解析 `./format`、`./server`）。

- [ ] **Step 4: 实现 format 与 server**

`web/src/lib/format.ts`：

```ts
export type Lang = 'zh-CN' | 'en-US'

const UNITS = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']

/** formatBytes 把字节数格式化为带单位的字符串（1024 进制） */
export function formatBytes(n: number): string {
  if (!Number.isFinite(n) || n <= 0) return '0 B'
  const i = Math.min(Math.floor(Math.log(n) / Math.log(1024)), UNITS.length - 1)
  const v = n / 1024 ** i
  const digits = i === 0 || v >= 100 ? 0 : 1
  return `${v.toFixed(digits)} ${UNITS[i]}`
}

export function formatSpeed(n: number): string {
  return `${formatBytes(n)}/s`
}

/** percent 计算百分比并限制在 0–100，total 非正数时返回 0 */
export function percent(used: number, total: number): number {
  if (!Number.isFinite(used) || !Number.isFinite(total) || total <= 0) return 0
  return Math.min(100, Math.max(0, (used / total) * 100))
}

export function formatPercent(p: number): string {
  return `${Math.round(p)}%`
}

/** formatDuration 把秒数格式化为最多两级的时长 */
export function formatDuration(seconds: number, lang: Lang): string {
  const s = Math.max(0, Math.floor(seconds))
  const d = Math.floor(s / 86400)
  const h = Math.floor((s % 86400) / 3600)
  const m = Math.floor((s % 3600) / 60)
  const zh = lang === 'zh-CN'
  if (d > 0) return zh ? `${d} 天${h > 0 ? ` ${h} 小时` : ''}` : `${d}d${h > 0 ? ` ${h}h` : ''}`
  if (h > 0) return zh ? `${h} 小时${m > 0 ? ` ${m} 分` : ''}` : `${h}h${m > 0 ? ` ${m}m` : ''}`
  return zh ? `${m} 分钟` : `${m}m`
}

/** daysUntil 距到期的天数（向上取整，已过期为负数）；无到期时间返回 null */
export function daysUntil(expireAt: number | null, nowSec: number): number | null {
  if (expireAt == null) return null
  return Math.ceil((expireAt - nowSec) / 86400)
}

/** formatAgo 相对时间，如“12 秒前” */
export function formatAgo(ts: number, nowSec: number, lang: Lang): string {
  const diff = Math.max(0, Math.floor(nowSec - ts))
  const zh = lang === 'zh-CN'
  if (diff < 60) return zh ? `${diff} 秒前` : `${diff}s ago`
  if (diff < 3600) return zh ? `${Math.floor(diff / 60)} 分钟前` : `${Math.floor(diff / 60)}m ago`
  if (diff < 86400) return zh ? `${Math.floor(diff / 3600)} 小时前` : `${Math.floor(diff / 3600)}h ago`
  return zh ? `${Math.floor(diff / 86400)} 天前` : `${Math.floor(diff / 86400)}d ago`
}

export function formatDate(ts: number, lang: Lang): string {
  return new Date(ts * 1000).toLocaleDateString(lang)
}
```

`web/src/lib/server.ts`：

```ts
import { percent } from './format'
import type { ServerView, Traffic } from './types'

export type Level = 'ok' | 'warn' | 'bad'

/** usageLevel 用量阈值：<70 正常，70–90 警告，≥90 危险 */
export function usageLevel(p: number): Level {
  if (p >= 90) return 'bad'
  if (p >= 70) return 'warn'
  return 'ok'
}

export function cpuPct(s: ServerView): number {
  return s.metrics ? Math.min(100, Math.max(0, s.metrics.cpu)) : 0
}

export function memPct(s: ServerView): number {
  return s.metrics && s.static ? percent(s.metrics.mem_used, s.static.mem_total) : 0
}

export function diskPct(s: ServerView): number {
  return s.metrics ? percent(s.metrics.disk_used, s.metrics.disk_total) : 0
}

/** trafficPct 流量配额使用率；未设置配额返回 null（界面显示 ∞） */
export function trafficPct(t: Traffic): number | null {
  return t.limit != null && t.limit > 0 ? percent(t.used, t.limit) : null
}

/** expiryLevel 到期提醒级别：≤7 天警告，已到期危险 */
export function expiryLevel(days: number | null): Level {
  if (days == null) return 'ok'
  if (days <= 0) return 'bad'
  if (days <= 7) return 'warn'
  return 'ok'
}

/** osLabel 形如“debian 12 · kvm · x86_64” */
export function osLabel(s: ServerView): string {
  const st = s.static
  if (!st) return ''
  const os = st.platform ? `${st.platform}${st.platform_version ? ` ${st.platform_version}` : ''}` : st.os
  return [os, st.virtualization, st.arch].filter(Boolean).join(' · ')
}

/** sortServers 按后台排序值、再按名称排序，返回新数组 */
export function sortServers(list: ServerView[]): ServerView[] {
  return [...list].sort((a, b) => a.sort - b.sort || a.name.localeCompare(b.name))
}
```

- [ ] **Step 5: 实现国际化**

`web/src/i18n/zh-CN.ts`：

```ts
export const zhCN = {
  nav: { login: '登录', admin: '后台', status: '状态页', servers: '服务器', settings: '设置', logout: '退出登录', menu: '菜单', back: '返回' },
  theme: { toggle: '切换主题', light: '浅色', dark: '深色', system: '跟随系统' },
  lang: { switch: 'English' },
  conn: { polling: '实时连接已中断，正在每 5 秒刷新' },
  errors: { network: '网络错误，请检查连接', http: '请求失败（{{status}}）' },
  summary: { nodes: '节点', offline: '{{count}} 台离线', allOnline: '全部在线', busiest: '最忙节点', traffic: '本月流量', speed: '实时网速' },
  filter: { all: '全部', offline: '离线', search: '搜索名称' },
  view: { card: '卡片视图', list: '列表视图' },
  status: { online: '在线', offline: '离线', onlineFor: '在线 {{duration}}', lastSeen: '最后上报 {{ago}}', neverSeen: '尚未上报' },
  metric: { mem: '内存', disk: '硬盘', traffic: '流量', swap: '交换', cores: '{{count}} 核', thisMonth: '本月 {{value}}' },
  expire: { days: '{{count}} 天后到期', expired: '已过期', never: '长期' },
  empty: { servers: '还没有服务器', noMatch: '没有匹配的服务器' },
  table: { name: '名称', system: '系统', mem: '内存', disk: '硬盘', traffic: '流量', speed: '网速', uptime: '运行时长', expire: '到期' },
  detail: {
    notFound: '服务器不存在或已隐藏',
    system: '系统',
    cpu: 'CPU',
    memory: '内存 / 硬盘 / 交换',
    arch: '架构 · 虚拟化',
    traffic: '本月流量',
    resetDay: '每月 {{day}} 日重置',
    renew: '续费',
    expireAt: '{{date}} 到期',
    disks: '分区',
    history: '历史趋势',
    network: '网络',
    inbound: '下行',
    outbound: '上行',
    loadTcp: '负载 / TCP 连接',
    noData: '暂无数据',
  },
  range: { realtime: '实时', '1h': '1 小时', '6h': '6 小时', '24h': '24 小时', '7d': '7 天' },
  billing: { monthly: '月付', quarterly: '季付', yearly: '年付', once: '一次性' },
  login: { title: '登录后台', username: '用户名', password: '密码', submit: '登录' },
  admin: {
    title: '服务器',
    add: '新建服务器',
    empty: '还没有服务器，点击右上角新建',
    drag: '拖动排序',
    name: '名称',
    group: '分组',
    ip: 'IP',
    expire: '到期',
    interval: '上报间隔',
    actions: '操作',
    edit: '编辑',
    install: '安装命令',
    resetSecret: '重置密钥',
    delete: '删除',
    saved: '已保存',
    deleted: '已删除',
    secretReset: '密钥已重置',
    deleteTitle: '删除服务器 {{name}}？',
    deleteDesc: '将同时删除历史数据与流量记录，在线的 Agent 会收到停止指令。',
    resetTitle: '重置 {{name}} 的密钥？',
    resetDesc: '旧密钥立即失效，需要在服务器上用新的安装命令重新安装 Agent。',
  },
  form: {
    createTitle: '新建服务器',
    editTitle: '编辑服务器',
    basic: '基本',
    billing: '计费',
    traffic: '流量',
    collect: '采集',
    name: '名称',
    group: '分组',
    country: '国家代码',
    countryHint: '两位字母，如 HK；留空则使用 Agent 探测结果',
    hidden: '仅登录可见',
    price: '价格',
    currency: '币种',
    cycle: '付费周期',
    cycleNone: '不设置',
    expireAt: '到期日',
    trafficLimit: '流量配额（GB）',
    trafficLimitHint: '留空表示不限',
    trafficMode: '计量方式',
    modeSum: '双向合计',
    modeIn: '仅入站',
    modeOut: '仅出站',
    resetDay: '重置日（1–28）',
    interval: '上报间隔（秒）',
    intervalHint: '留空使用默认值',
    nicInclude: '仅统计网卡',
    nicExclude: '排除网卡',
    mountExclude: '排除挂载点',
    listHint: '多个用逗号分隔，按前缀匹配',
    cancel: '取消',
    save: '保存',
    nameRequired: '请输入名称',
    nameTooLong: '名称不超过 64 个字符',
    countryInvalid: '国家代码应为两位字母',
    numberInvalid: '请输入有效的数字',
    resetDayRange: '重置日应在 1–28 之间',
    intervalRange: '上报间隔应在 1–60 秒之间',
  },
  install: { title: '安装命令', desc: '在目标服务器上以管理员身份执行以下命令安装 Agent', copy: '复制', copied: '已复制', copyFailed: '复制失败，请手动选择复制' },
  settings: {
    title: '设置',
    site: '站点',
    siteTitle: '站点标题',
    showPrice: '公开显示价格',
    defaultInterval: '默认上报间隔（秒）',
    scriptBase: '安装脚本地址',
    save: '保存',
    saved: '设置已保存',
    password: '修改密码',
    oldPassword: '原密码',
    newPassword: '新密码',
    confirmPassword: '确认新密码',
    passwordMismatch: '两次输入的新密码不一致',
    passwordTooShort: '新密码至少 8 位',
    passwordChanged: '密码已修改',
    backup: '备份与迁移',
    backupDesc: '导出的文件包含全部服务器及其 Agent 密钥，在新面板导入后无需重装 Agent。',
    export: '导出 JSON',
    exportTitle: '导出文件包含密钥',
    exportDesc: '文件中包含所有服务器的 Agent 密钥，请妥善保管，不要分享给他人。',
    import: '导入 JSON',
    importDone: '已导入 {{count}} 台服务器',
    importInvalid: '文件不是有效的 JSON',
  },
  common: { cancel: '取消', confirm: '确定', loading: '加载中…' },
  notFound: { title: '页面不存在', back: '返回首页' },
}

type Widen<T> = { [K in keyof T]: T[K] extends string ? string : Widen<T[K]> }
export type Dict = Widen<typeof zhCN>
```

`web/src/i18n/en-US.ts`：

```ts
import type { Dict } from './zh-CN'

export const enUS: Dict = {
  nav: { login: 'Log in', admin: 'Admin', status: 'Status', servers: 'Servers', settings: 'Settings', logout: 'Log out', menu: 'Menu', back: 'Back' },
  theme: { toggle: 'Toggle theme', light: 'Light', dark: 'Dark', system: 'System' },
  lang: { switch: '中文' },
  conn: { polling: 'Live connection lost, refreshing every 5 seconds' },
  errors: { network: 'Network error, please check your connection', http: 'Request failed ({{status}})' },
  summary: { nodes: 'Nodes', offline: '{{count}} offline', allOnline: 'All online', busiest: 'Busiest', traffic: 'Traffic this month', speed: 'Throughput' },
  filter: { all: 'All', offline: 'Offline', search: 'Search name' },
  view: { card: 'Card view', list: 'List view' },
  status: { online: 'Online', offline: 'Offline', onlineFor: 'Up {{duration}}', lastSeen: 'Last seen {{ago}}', neverSeen: 'Never reported' },
  metric: { mem: 'Memory', disk: 'Disk', traffic: 'Traffic', swap: 'Swap', cores: '{{count}} cores', thisMonth: '{{value}} this month' },
  expire: { days: 'Expires in {{count}}d', expired: 'Expired', never: 'No expiry' },
  empty: { servers: 'No servers yet', noMatch: 'No matching servers' },
  table: { name: 'Name', system: 'System', mem: 'Memory', disk: 'Disk', traffic: 'Traffic', speed: 'Speed', uptime: 'Uptime', expire: 'Expiry' },
  detail: {
    notFound: 'Server not found or hidden',
    system: 'System',
    cpu: 'CPU',
    memory: 'Memory / Disk / Swap',
    arch: 'Arch · Virtualization',
    traffic: 'Traffic this month',
    resetDay: 'Resets on day {{day}}',
    renew: 'Renewal',
    expireAt: 'Expires {{date}}',
    disks: 'Partitions',
    history: 'History',
    network: 'Network',
    inbound: 'In',
    outbound: 'Out',
    loadTcp: 'Load / TCP',
    noData: 'No data',
  },
  range: { realtime: 'Live', '1h': '1 hour', '6h': '6 hours', '24h': '24 hours', '7d': '7 days' },
  billing: { monthly: 'Monthly', quarterly: 'Quarterly', yearly: 'Yearly', once: 'One-time' },
  login: { title: 'Admin login', username: 'Username', password: 'Password', submit: 'Log in' },
  admin: {
    title: 'Servers',
    add: 'New server',
    empty: 'No servers yet. Create one from the top right.',
    drag: 'Drag to reorder',
    name: 'Name',
    group: 'Group',
    ip: 'IP',
    expire: 'Expiry',
    interval: 'Interval',
    actions: 'Actions',
    edit: 'Edit',
    install: 'Install command',
    resetSecret: 'Reset secret',
    delete: 'Delete',
    saved: 'Saved',
    deleted: 'Deleted',
    secretReset: 'Secret reset',
    deleteTitle: 'Delete server {{name}}?',
    deleteDesc: 'History and traffic records are deleted too; a connected agent will be told to stop.',
    resetTitle: 'Reset the secret of {{name}}?',
    resetDesc: 'The old secret stops working immediately; reinstall the agent with the new command.',
  },
  form: {
    createTitle: 'New server',
    editTitle: 'Edit server',
    basic: 'Basic',
    billing: 'Billing',
    traffic: 'Traffic',
    collect: 'Collection',
    name: 'Name',
    group: 'Group',
    country: 'Country code',
    countryHint: 'Two letters such as HK; leave empty to use the agent’s detection',
    hidden: 'Visible to admins only',
    price: 'Price',
    currency: 'Currency',
    cycle: 'Billing cycle',
    cycleNone: 'Not set',
    expireAt: 'Expiry date',
    trafficLimit: 'Traffic quota (GB)',
    trafficLimitHint: 'Leave empty for unlimited',
    trafficMode: 'Metering',
    modeSum: 'In + out',
    modeIn: 'Inbound only',
    modeOut: 'Outbound only',
    resetDay: 'Reset day (1–28)',
    interval: 'Report interval (s)',
    intervalHint: 'Leave empty to use the default',
    nicInclude: 'Only these NICs',
    nicExclude: 'Exclude NICs',
    mountExclude: 'Exclude mounts',
    listHint: 'Comma separated, prefix match',
    cancel: 'Cancel',
    save: 'Save',
    nameRequired: 'Name is required',
    nameTooLong: 'Name must be at most 64 characters',
    countryInvalid: 'Country code must be two letters',
    numberInvalid: 'Enter a valid number',
    resetDayRange: 'Reset day must be between 1 and 28',
    intervalRange: 'Interval must be between 1 and 60 seconds',
  },
  install: { title: 'Install command', desc: 'Run this command as administrator on the target server', copy: 'Copy', copied: 'Copied', copyFailed: 'Copy failed, please select and copy manually' },
  settings: {
    title: 'Settings',
    site: 'Site',
    siteTitle: 'Site title',
    showPrice: 'Show prices publicly',
    defaultInterval: 'Default report interval (s)',
    scriptBase: 'Install script URL',
    save: 'Save',
    saved: 'Settings saved',
    password: 'Change password',
    oldPassword: 'Current password',
    newPassword: 'New password',
    confirmPassword: 'Confirm new password',
    passwordMismatch: 'The new passwords do not match',
    passwordTooShort: 'The new password needs at least 8 characters',
    passwordChanged: 'Password changed',
    backup: 'Backup & migration',
    backupDesc: 'The export contains every server and its agent secret; import it on a new panel without reinstalling agents.',
    export: 'Export JSON',
    exportTitle: 'The export contains secrets',
    exportDesc: 'The file includes every agent secret. Keep it safe and do not share it.',
    import: 'Import JSON',
    importDone: 'Imported {{count}} servers',
    importInvalid: 'The file is not valid JSON',
  },
  common: { cancel: 'Cancel', confirm: 'OK', loading: 'Loading…' },
  notFound: { title: 'Page not found', back: 'Back to home' },
}
```

`web/src/i18n/index.ts`：

```ts
import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'

import type { Lang } from '@/lib/format'

import { enUS } from './en-US'
import { zhCN } from './zh-CN'

const KEY = 'sss.locale'

/** detectLang 优先使用已保存的语言，否则按浏览器语言选择 */
export function detectLang(): Lang {
  const saved = localStorage.getItem(KEY)
  if (saved === 'zh-CN' || saved === 'en-US') return saved
  return navigator.language.toLowerCase().startsWith('zh') ? 'zh-CN' : 'en-US'
}

void i18n.use(initReactI18next).init({
  resources: { 'zh-CN': { translation: zhCN }, 'en-US': { translation: enUS } },
  lng: detectLang(),
  fallbackLng: 'zh-CN',
  interpolation: { escapeValue: false },
})

i18n.on('languageChanged', (lng) => {
  document.documentElement.lang = lng
})

/** setLang 切换并保存界面语言 */
export function setLang(lang: Lang) {
  localStorage.setItem(KEY, lang)
  void i18n.changeLanguage(lang)
}

export default i18n
```

`web/src/i18n/use-lang.ts`：

```ts
import { useTranslation } from 'react-i18next'

import type { Lang } from '@/lib/format'

/** useLang 当前界面语言（随切换重新渲染） */
export function useLang(): Lang {
  const { i18n } = useTranslation()
  return i18n.language === 'en-US' ? 'en-US' : 'zh-CN'
}
```

在 `web/src/test/setup.ts` 顶部导入区加入 `import i18n from '@/i18n'`，并在 `beforeEach` 回调的第一行加入：

```ts
  void i18n.changeLanguage('zh-CN')
```

- [ ] **Step 6: 运行检查**

Run: `cd web && pnpm typecheck && pnpm lint && pnpm test`
Expected: 全部通过。

- [ ] **Step 7: 提交**

```bash
git add web/src
git commit -m "feat(web): 新增类型定义、格式化工具、指标计算与中英文词典"
```

---

### Task 4: API 客户端、登录态与管理接口封装

**Files:**
- Create: `web/src/lib/api.ts`、`web/src/lib/auth.ts`、`web/src/lib/admin-api.ts`
- Create: `web/src/test/fetch.ts`
- Test: `web/src/lib/api.test.ts`

**Interfaces:**
- Consumes: `i18n`（Task 3）、类型（Task 3）
- Produces:
  - `class ApiError extends Error { status: number; code: string }`
  - `tokenStore: { get(): string | null; set(t): void; clear(): void; subscribe(l): () => void }`
  - `request<T>(method, path, body?)`、`api.get/post/put/del<T>`、`errorMessage(e): string`
  - `useToken(): string | null`（`lib/auth.ts`）
  - `adminKeys`、`adminApi`（`lib/admin-api.ts`，方法见代码）
  - 测试：`mockFetch(routes)`，路由键为 `"METHOD /path"`（可带 query），值为数据或 `(body) => ({ status?, data?, error? })`

- [ ] **Step 1: 写测试工具与失败测试**

`web/src/test/fetch.ts`：

```ts
import { vi } from 'vitest'

export interface MockResult {
  status?: number
  data?: unknown
  error?: { code: string; message: string }
}
export type MockHandler = (body: unknown, url: string) => MockResult

/** mockFetch 按 "METHOD /path" 匹配请求（先匹配完整路径含 query，再匹配去掉 query 的路径），返回后端信封格式的响应 */
export function mockFetch(routes: Record<string, unknown>) {
  const fn = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? `${input.pathname}${input.search}` : input.url
    const method = init?.method ?? 'GET'
    const route = routes[`${method} ${url}`] ?? routes[`${method} ${url.split('?')[0]}`]
    if (route === undefined) {
      return new Response(JSON.stringify({ error: { code: 'not_found', message: `未模拟的请求 ${method} ${url}` } }), { status: 404 })
    }
    const body: unknown = typeof init?.body === 'string' ? JSON.parse(init.body) : undefined
    const result: MockResult = typeof route === 'function' ? (route as MockHandler)(body, url) : { data: route }
    const status = result.status ?? 200
    const payload = status < 400 ? { data: result.data ?? null } : { error: result.error ?? { code: 'error', message: '错误' } }
    return new Response(JSON.stringify(payload), { status, headers: { 'Content-Type': 'application/json' } })
  })
  vi.stubGlobal('fetch', fn)
  return fn
}
```

`web/src/lib/api.test.ts`：

```ts
import { describe, expect, it, vi } from 'vitest'

import { mockFetch } from '@/test/fetch'

import { api, ApiError, errorMessage, tokenStore } from './api'

describe('api', () => {
  it('解包 data 并携带 JSON 请求体与 token', async () => {
    tokenStore.set('tok')
    const fetchFn = mockFetch({ 'POST /api/x': (body: unknown) => ({ data: { echo: body } }) })
    const r = await api.post<{ echo: unknown }>('/api/x', { a: 1 })
    expect(r.echo).toEqual({ a: 1 })
    const init = fetchFn.mock.calls[0][1] as RequestInit
    const headers = init.headers as Record<string, string>
    expect(headers.Authorization).toBe('Bearer tok')
    expect(headers['Content-Type']).toBe('application/json')
  })

  it('错误信封转换为 ApiError', async () => {
    mockFetch({ 'GET /api/x': () => ({ status: 400, error: { code: 'invalid_input', message: '名称不能为空' } }) })
    const err = await api.get('/api/x').catch((e: unknown) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect((err as ApiError).code).toBe('invalid_input')
    expect((err as ApiError).status).toBe(400)
    expect(errorMessage(err)).toBe('名称不能为空')
  })

  it('非登录接口 401 时清除 token 并通知订阅者', async () => {
    tokenStore.set('tok')
    const listener = vi.fn()
    const unsubscribe = tokenStore.subscribe(listener)
    mockFetch({ 'GET /api/admin/servers': () => ({ status: 401, error: { code: 'unauthorized', message: '请先登录' } }) })
    await expect(api.get('/api/admin/servers')).rejects.toThrow('请先登录')
    expect(tokenStore.get()).toBeNull()
    expect(listener).toHaveBeenCalled()
    unsubscribe()
  })

  it('登录接口 401 不清除已有 token', async () => {
    tokenStore.set('tok')
    mockFetch({ 'POST /api/auth/login': () => ({ status: 401, error: { code: 'invalid_credentials', message: '用户名或密码错误' } }) })
    await expect(api.post('/api/auth/login', {})).rejects.toThrow('用户名或密码错误')
    expect(tokenStore.get()).toBe('tok')
  })

  it('网络错误转换为 network', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => { throw new TypeError('failed') }))
    const err = await api.get('/api/x').catch((e: unknown) => e)
    expect((err as ApiError).code).toBe('network')
    expect((err as ApiError).message).toBe('网络错误，请检查连接')
  })
})
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd web && pnpm test src/lib/api.test.ts`
Expected: FAIL（无法解析 `./api`）。

- [ ] **Step 3: 实现**

`web/src/lib/api.ts`：

```ts
import i18n from '@/i18n'

/** ApiError 后端错误信封或网络错误 */
export class ApiError extends Error {
  readonly status: number
  readonly code: string

  constructor(status: number, code: string, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
  }
}

const TOKEN_KEY = 'sss.token'
const listeners = new Set<() => void>()

function notify() {
  listeners.forEach((l) => l())
}

/** tokenStore 登录 token 的唯一存取入口（可订阅变化） */
export const tokenStore = {
  get(): string | null {
    return localStorage.getItem(TOKEN_KEY)
  },
  set(token: string) {
    localStorage.setItem(TOKEN_KEY, token)
    notify()
  },
  clear() {
    localStorage.removeItem(TOKEN_KEY)
    notify()
  },
  subscribe(listener: () => void): () => void {
    listeners.add(listener)
    return () => {
      listeners.delete(listener)
    }
  },
}

interface Envelope {
  data?: unknown
  error?: { code?: string; message?: string }
}

function parse(text: string): Envelope | null {
  if (!text) return null
  try {
    return JSON.parse(text) as Envelope
  } catch {
    return null
  }
}

/** request 调用后端接口并解析 {data}/{error} 信封；非登录接口返回 401 时清除本地登录态 */
export async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = {}
  const init: RequestInit = { method, headers }
  if (body !== undefined) {
    headers['Content-Type'] = 'application/json'
    init.body = JSON.stringify(body)
  }
  const token = tokenStore.get()
  if (token) headers.Authorization = `Bearer ${token}`

  let res: Response
  try {
    res = await fetch(path, init)
  } catch {
    throw new ApiError(0, 'network', i18n.t('errors.network'))
  }
  const json = parse(await res.text())
  if (!res.ok) {
    if (res.status === 401 && path !== '/api/auth/login') tokenStore.clear()
    throw new ApiError(res.status, json?.error?.code ?? `http_${res.status}`, json?.error?.message ?? i18n.t('errors.http', { status: res.status }))
  }
  return (json?.data ?? null) as T
}

export const api = {
  get: <T>(path: string) => request<T>('GET', path),
  post: <T>(path: string, body?: unknown) => request<T>('POST', path, body),
  put: <T>(path: string, body?: unknown) => request<T>('PUT', path, body),
  del: <T>(path: string) => request<T>('DELETE', path),
}

export function errorMessage(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}
```

`web/src/lib/auth.ts`：

```ts
import { useSyncExternalStore } from 'react'

import { tokenStore } from './api'

/** useToken 当前登录 token；登录、退出或 401 失效时自动重新渲染 */
export function useToken(): string | null {
  return useSyncExternalStore(tokenStore.subscribe, tokenStore.get, () => null)
}
```

`web/src/lib/admin-api.ts`：

```ts
import { api } from './api'
import type { AdminServer, ServerInput, Settings } from './types'

export const adminKeys = {
  servers: ['admin', 'servers'] as const,
  settings: ['admin', 'settings'] as const,
  me: ['admin', 'me'] as const,
}

export interface InstallCommands {
  linux: string
  windows: string
}

const enc = encodeURIComponent

/** adminApi 管理接口（均需登录） */
export const adminApi = {
  me: () => api.get<{ username: string }>('/api/auth/me'),
  login: (username: string, password: string) => api.post<{ token: string; username: string }>('/api/auth/login', { username, password }),
  changePassword: (oldPassword: string, newPassword: string) =>
    api.put<{ token: string }>('/api/auth/password', { old_password: oldPassword, new_password: newPassword }),
  servers: () => api.get<AdminServer[]>('/api/admin/servers'),
  create: (input: ServerInput) => api.post<AdminServer>('/api/admin/servers', input),
  update: (id: string, input: ServerInput) => api.put<AdminServer>(`/api/admin/servers/${enc(id)}`, input),
  remove: (id: string) => api.del<unknown>(`/api/admin/servers/${enc(id)}`),
  resetSecret: (id: string) => api.post<{ secret: string }>(`/api/admin/servers/${enc(id)}/reset-secret`),
  install: (id: string, dashboard: string) => api.get<InstallCommands>(`/api/admin/servers/${enc(id)}/install?dashboard=${enc(dashboard)}`),
  order: (ids: string[]) => api.put<unknown>('/api/admin/server-order', { ids }),
  settings: () => api.get<Settings>('/api/admin/settings'),
  saveSettings: (s: Settings) => api.put<Settings>('/api/admin/settings', s),
  exportData: () => api.get<unknown>('/api/admin/export'),
  importData: (file: unknown) => api.post<{ servers: number }>('/api/admin/import', file),
}
```

- [ ] **Step 4: 运行检查**

Run: `cd web && pnpm typecheck && pnpm lint && pnpm test`
Expected: 全部通过。

- [ ] **Step 5: 提交**

```bash
git add web/src
git commit -m "feat(web): 新增 API 客户端、登录态存储与管理接口封装"
```

---

### Task 5: 应用外壳：主题、语言、站点头部、路由

**Files:**
- Create: `web/src/components/theme.tsx`、`web/src/components/header-controls.tsx`、`web/src/components/site-header.tsx`、`web/src/components/app-toaster.tsx`
- Create: `web/src/features/site/use-site.ts`
- Create: `web/src/app/router.tsx`、`web/src/app/route-error.tsx`、`web/src/app/not-found.tsx`、`web/src/features/auth/require-auth.tsx`
- Create（占位，后续任务替换）: `web/src/features/status/status-page.tsx`、`web/src/features/detail/detail-page.tsx`、`web/src/features/auth/login-page.tsx`、`web/src/features/admin/admin-layout.tsx`、`web/src/features/admin/servers-page.tsx`、`web/src/features/admin/settings-page.tsx`
- Modify: `web/src/main.tsx`（整体替换）
- Create: `web/src/test/render.tsx`
- Test: `web/src/components/theme.test.tsx`、`web/src/features/auth/require-auth.test.tsx`

**Interfaces:**
- Consumes: `tokenStore`、`useToken`、`api`（Task 4）；`setLang`、`useLang`（Task 3）
- Produces:
  - `ThemeProvider`、`useTheme(): { theme, resolved, setTheme }`、`type Theme = 'light' | 'dark' | 'system'`
  - `ThemeToggle`、`LangToggle`、`SiteHeader`、`AppToaster`
  - `useSite(): Site`
  - `RequireAuth`（未登录时跳转 `/login`，`state.from` 为原路径）
  - `router`：`/`、`/server/:id`、`/login`、`/admin`（→ `/admin/servers`）、`/admin/servers`、`/admin/settings`、`*`；各页面组件为具名导出：`StatusPage`、`DetailPage`、`LoginPage`、`AdminLayout`、`ServersPage`、`SettingsPage`
  - 测试：`renderWithProviders(ui, { route, path })` 返回 `{ client, ...RenderResult }`

- [ ] **Step 1: 写失败测试**

`web/src/test/render.tsx`：

```tsx
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render } from '@testing-library/react'
import type { ReactElement } from 'react'
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom'

import { ThemeProvider } from '@/components/theme'

function LocationProbe() {
  const location = useLocation()
  return <div data-testid="location">{location.pathname}</div>
}

/** renderWithProviders 在主题、Query 与内存路由中渲染组件；path 为该组件挂载的路由模式 */
export function renderWithProviders(ui: ReactElement, { route = '/', path = '/' }: { route?: string; path?: string } = {}) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  const result = render(
    <ThemeProvider>
      <QueryClientProvider client={client}>
        <MemoryRouter initialEntries={[route]}>
          <Routes>
            <Route path={path} element={ui} />
            <Route path="*" element={<div data-testid="elsewhere" />} />
          </Routes>
          <LocationProbe />
        </MemoryRouter>
      </QueryClientProvider>
    </ThemeProvider>,
  )
  return { client, ...result }
}
```

`web/src/components/theme.test.tsx`：

```tsx
import { act, render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { ThemeProvider, useTheme } from './theme'

function Probe() {
  const { theme, resolved, setTheme } = useTheme()
  return (
    <button type="button" onClick={() => setTheme('dark')}>
      {theme}/{resolved}
    </button>
  )
}

describe('ThemeProvider', () => {
  it('默认跟随系统，切换后写入 localStorage 并设置 dark 类', () => {
    render(
      <ThemeProvider>
        <Probe />
      </ThemeProvider>,
    )
    expect(screen.getByRole('button')).toHaveTextContent('system/light')
    act(() => screen.getByRole('button').click())
    expect(screen.getByRole('button')).toHaveTextContent('dark/dark')
    expect(localStorage.getItem('sss.theme')).toBe('dark')
    expect(document.documentElement.classList.contains('dark')).toBe(true)
  })
})
```

`web/src/features/auth/require-auth.test.tsx`：

```tsx
import { act, render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom'
import { describe, expect, it } from 'vitest'

import { tokenStore } from '@/lib/api'

import { RequireAuth } from './require-auth'

function LoginProbe() {
  const location = useLocation()
  return <div>login from {(location.state as { from?: string } | null)?.from}</div>
}

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/login" element={<LoginProbe />} />
        <Route element={<RequireAuth />}>
          <Route path="/admin/servers" element={<div>admin page</div>} />
        </Route>
      </Routes>
    </MemoryRouter>,
  )
}

describe('RequireAuth', () => {
  it('未登录时跳转登录页并记录来源', () => {
    renderAt('/admin/servers')
    expect(screen.getByText('login from /admin/servers')).toBeInTheDocument()
  })

  it('已登录时渲染子路由，token 被清除后立即跳转', () => {
    tokenStore.set('tok')
    renderAt('/admin/servers')
    expect(screen.getByText('admin page')).toBeInTheDocument()
    act(() => tokenStore.clear())
    expect(screen.getByText('login from /admin/servers')).toBeInTheDocument()
  })
})
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd web && pnpm test`
Expected: FAIL（无法解析 `./theme`、`./require-auth`）。

- [ ] **Step 3: 实现主题、头部与守卫**

`web/src/components/theme.tsx`：

```tsx
import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'

export type Theme = 'light' | 'dark' | 'system'

interface ThemeState {
  theme: Theme
  resolved: 'light' | 'dark'
  setTheme: (t: Theme) => void
}

const KEY = 'sss.theme'
const QUERY = '(prefers-color-scheme: dark)'
const ThemeContext = createContext<ThemeState | null>(null)

function readTheme(): Theme {
  const v = localStorage.getItem(KEY)
  return v === 'light' || v === 'dark' ? v : 'system'
}

/** ThemeProvider 管理亮/暗/跟随系统主题，并同步到 <html> 的 dark 类 */
export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setThemeState] = useState<Theme>(readTheme)
  const [systemDark, setSystemDark] = useState(() => window.matchMedia(QUERY).matches)

  useEffect(() => {
    const mq = window.matchMedia(QUERY)
    const onChange = () => setSystemDark(mq.matches)
    mq.addEventListener('change', onChange)
    return () => mq.removeEventListener('change', onChange)
  }, [])

  const resolved: 'light' | 'dark' = theme === 'system' ? (systemDark ? 'dark' : 'light') : theme

  useEffect(() => {
    document.documentElement.classList.toggle('dark', resolved === 'dark')
  }, [resolved])

  const value = useMemo<ThemeState>(
    () => ({
      theme,
      resolved,
      setTheme: (t) => {
        localStorage.setItem(KEY, t)
        setThemeState(t)
      },
    }),
    [theme, resolved],
  )
  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>
}

export function useTheme(): ThemeState {
  const v = useContext(ThemeContext)
  if (!v) throw new Error('useTheme 必须在 ThemeProvider 内使用')
  return v
}
```

`web/src/components/header-controls.tsx`：

```tsx
import { Moon, Sun } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { setLang } from '@/i18n'
import { useLang } from '@/i18n/use-lang'

import { useTheme, type Theme } from './theme'

export function ThemeToggle() {
  const { theme, resolved, setTheme } = useTheme()
  const { t } = useTranslation()
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" aria-label={t('theme.toggle')}>
          {resolved === 'dark' ? <Moon className="size-4" /> : <Sun className="size-4" />}
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuRadioGroup value={theme} onValueChange={(v) => setTheme(v as Theme)}>
          <DropdownMenuRadioItem value="light">{t('theme.light')}</DropdownMenuRadioItem>
          <DropdownMenuRadioItem value="dark">{t('theme.dark')}</DropdownMenuRadioItem>
          <DropdownMenuRadioItem value="system">{t('theme.system')}</DropdownMenuRadioItem>
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

export function LangToggle() {
  const lang = useLang()
  const { t } = useTranslation()
  return (
    <Button variant="ghost" size="sm" onClick={() => setLang(lang === 'zh-CN' ? 'en-US' : 'zh-CN')}>
      {t('lang.switch')}
    </Button>
  )
}
```

`web/src/features/site/use-site.ts`：

```ts
import { useQuery } from '@tanstack/react-query'
import { useEffect } from 'react'

import { api } from '@/lib/api'
import type { Site } from '@/lib/types'

const FALLBACK: Site = { site_title: 'Simple Server Status', show_price: false }

/** useSite 站点标题与价格可见性，并同步浏览器标题 */
export function useSite(): Site {
  const { data } = useQuery({ queryKey: ['site'], queryFn: () => api.get<Site>('/api/public/site'), staleTime: 60_000 })
  useEffect(() => {
    if (data) document.title = data.site_title
  }, [data])
  return data ?? FALLBACK
}
```

`web/src/components/site-header.tsx`：

```tsx
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router-dom'

import { Button } from '@/components/ui/button'
import { useSite } from '@/features/site/use-site'
import { useToken } from '@/lib/auth'

import { LangToggle, ThemeToggle } from './header-controls'

export function SiteHeader() {
  const site = useSite()
  const token = useToken()
  const { t } = useTranslation()
  return (
    <header className="sticky top-0 z-30 border-b bg-background/80 backdrop-blur">
      <div className="mx-auto flex h-14 max-w-[1400px] items-center justify-between gap-2 px-4">
        <Link to="/" className="truncate font-semibold tracking-tight">
          {site.site_title}
        </Link>
        <div className="flex shrink-0 items-center gap-1">
          <LangToggle />
          <ThemeToggle />
          <Button asChild variant="ghost" size="sm">
            <Link to={token ? '/admin' : '/login'}>{token ? t('nav.admin') : t('nav.login')}</Link>
          </Button>
        </div>
      </div>
    </header>
  )
}
```

`web/src/components/app-toaster.tsx`：

```tsx
import { Toaster } from 'sonner'

import { useTheme } from './theme'

export function AppToaster() {
  const { resolved } = useTheme()
  return <Toaster theme={resolved} position="top-center" richColors />
}
```

`web/src/features/auth/require-auth.tsx`：

```tsx
import { Navigate, Outlet, useLocation } from 'react-router-dom'

import { useToken } from '@/lib/auth'

/** RequireAuth 未登录时跳转登录页，并记录来源路径以便登录后返回 */
export function RequireAuth() {
  const token = useToken()
  const location = useLocation()
  if (!token) return <Navigate to="/login" replace state={{ from: location.pathname }} />
  return <Outlet />
}
```

`web/src/app/not-found.tsx`：

```tsx
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router-dom'

import { Button } from '@/components/ui/button'

export function NotFound() {
  const { t } = useTranslation()
  return (
    <main className="flex min-h-svh flex-col items-center justify-center gap-4 px-4">
      <p className="text-sm text-muted-foreground">{t('notFound.title')}</p>
      <Button asChild variant="outline" size="sm">
        <Link to="/">{t('notFound.back')}</Link>
      </Button>
    </main>
  )
}
```

`web/src/app/route-error.tsx`：

```tsx
import { useRouteError } from 'react-router-dom'

import { errorMessage } from '@/lib/api'

export function RouteError() {
  const error = useRouteError()
  return (
    <main className="flex min-h-svh items-center justify-center px-4">
      <pre className="max-w-lg whitespace-pre-wrap text-sm text-bad">{errorMessage(error)}</pre>
    </main>
  )
}
```

六个页面占位文件（后续任务整体替换），内容格式相同，以 `status-page.tsx` 为例：

```tsx
export function StatusPage() {
  return <div>StatusPage</div>
}
```

其余五个文件分别导出 `DetailPage`（`features/detail/detail-page.tsx`）、`LoginPage`（`features/auth/login-page.tsx`）、`AdminLayout`（`features/admin/admin-layout.tsx`，返回 `<Outlet />`，需 `import { Outlet } from 'react-router-dom'`）、`ServersPage`（`features/admin/servers-page.tsx`）、`SettingsPage`（`features/admin/settings-page.tsx`），函数体返回各自名称的 `<div>`。

`web/src/app/router.tsx`：

```tsx
import { createBrowserRouter, Navigate } from 'react-router-dom'

import { RequireAuth } from '@/features/auth/require-auth'
import { StatusPage } from '@/features/status/status-page'

import { NotFound } from './not-found'
import { RouteError } from './route-error'

export const router = createBrowserRouter([
  {
    errorElement: <RouteError />,
    children: [
      { path: '/', element: <StatusPage /> },
      { path: '/server/:id', lazy: async () => ({ Component: (await import('@/features/detail/detail-page')).DetailPage }) },
      { path: '/login', lazy: async () => ({ Component: (await import('@/features/auth/login-page')).LoginPage }) },
      {
        path: '/admin',
        element: <RequireAuth />,
        children: [
          {
            lazy: async () => ({ Component: (await import('@/features/admin/admin-layout')).AdminLayout }),
            children: [
              { index: true, element: <Navigate to="servers" replace /> },
              { path: 'servers', lazy: async () => ({ Component: (await import('@/features/admin/servers-page')).ServersPage }) },
              { path: 'settings', lazy: async () => ({ Component: (await import('@/features/admin/settings-page')).SettingsPage }) },
            ],
          },
        ],
      },
      { path: '*', element: <NotFound /> },
    ],
  },
])
```

`web/src/main.tsx`（整体替换）：

```tsx
import '@/index.css'
import '@/i18n'

import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { RouterProvider } from 'react-router-dom'

import { router } from '@/app/router'
import { AppToaster } from '@/components/app-toaster'
import { ThemeProvider } from '@/components/theme'

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: 1, refetchOnWindowFocus: false } },
})

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <ThemeProvider>
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
        <AppToaster />
      </QueryClientProvider>
    </ThemeProvider>
  </StrictMode>,
)
```

- [ ] **Step 4: 运行检查**

Run: `cd web && pnpm typecheck && pnpm lint && pnpm test && pnpm exec vite build`
Expected: 全部通过；构建产物中详情页、登录页、后台页为独立 chunk。

- [ ] **Step 5: 提交**

```bash
git add web/src
git commit -m "feat(web): 新增主题、语言切换、站点头部与路由骨架"
```

---

### Task 6: 实时数据层（WebSocket + 轮询回退）

**Files:**
- Create: `web/src/realtime/client.ts`、`web/src/realtime/speed-history.ts`、`web/src/realtime/use-live-servers.ts`、`web/src/hooks/use-now.ts`
- Create: `web/src/test/fake-ws.ts`
- Modify: `web/src/test/setup.ts`（每个测试替换全局 WebSocket 并重置网速历史）
- Test: `web/src/realtime/client.test.ts`、`web/src/realtime/live.test.ts`

**Interfaces:**
- Consumes: `api`、`tokenStore`、`useToken`（Task 4）；`sortServers`（Task 3）
- Produces:
  - `type RealtimeStatus = 'connecting' | 'open' | 'polling'`、`interface LiveMessage { type: 'snapshot' | 'delta'; data: ServerView[] }`
  - `backoff(attempt, random?)`、`parseMessage(raw)`、`class RealtimeClient { start(); stop() }`
  - `serversKey(authed)`、`applyMessage(prev, msg)`、`wsUrl(token, loc?)`、`useLiveServers(): { servers, isLoading, error, status }`
  - `recordSpeed(servers, nowSec?)`、`useSpeedHistory(): SpeedSample[]`、`resetSpeedHistory()`、`type SpeedSample = { ts; in; out }`
  - `useNow(intervalMs?): number`（Unix 秒）
  - 测试：`FakeWebSocket`（静态 `instances`、`reset()`、`latest()`；实例方法 `open()`、`receive(data)`、`drop()`）

- [ ] **Step 1: 写测试替身与失败测试**

`web/src/test/fake-ws.ts`：

```ts
/** FakeWebSocket 测试用 WebSocket 替身：不会自动连接，由测试手动触发事件 */
export class FakeWebSocket {
  static instances: FakeWebSocket[] = []

  onopen: (() => void) | null = null
  onmessage: ((ev: { data: unknown }) => void) | null = null
  onclose: (() => void) | null = null
  closed = false
  readonly url: string

  constructor(url: string) {
    this.url = url
    FakeWebSocket.instances.push(this)
  }

  close() {
    this.closed = true
  }

  open() {
    this.onopen?.()
  }

  receive(data: unknown) {
    this.onmessage?.({ data: typeof data === 'string' ? data : JSON.stringify(data) })
  }

  drop() {
    this.onclose?.()
  }

  static reset() {
    FakeWebSocket.instances = []
  }

  static latest(): FakeWebSocket {
    const ws = FakeWebSocket.instances.at(-1)
    if (!ws) throw new Error('没有创建 WebSocket')
    return ws
  }
}
```

`web/src/realtime/client.test.ts`：

```ts
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { makeServer } from '@/test/fixtures'
import { FakeWebSocket } from '@/test/fake-ws'

import { backoff, parseMessage, RealtimeClient, type RealtimeStatus } from './client'

function setup() {
  const statuses: RealtimeStatus[] = []
  const messages: unknown[] = []
  const poll = vi.fn()
  const client = new RealtimeClient({
    url: () => 'ws://test/api/public/ws',
    onMessage: (m) => messages.push(m),
    onStatus: (s) => statuses.push(s),
    poll,
    random: () => 0.5,
    createSocket: (u) => new FakeWebSocket(u) as unknown as WebSocket,
  })
  return { client, statuses, messages, poll }
}

describe('backoff', () => {
  it('1s 起翻倍、30s 封顶、带抖动', () => {
    expect(backoff(0, () => 0.5)).toBe(1000)
    expect(backoff(3, () => 0.5)).toBe(8000)
    expect(backoff(5, () => 0.5)).toBe(30000)
    expect(backoff(20, () => 0.5)).toBe(30000)
    expect(backoff(0, () => 0)).toBe(800)
  })
})

describe('parseMessage', () => {
  it('只接受 snapshot/delta 且 data 为数组', () => {
    expect(parseMessage(JSON.stringify({ type: 'delta', data: [] }))).toEqual({ type: 'delta', data: [] })
    expect(parseMessage(JSON.stringify({ type: 'other', data: [] }))).toBeNull()
    expect(parseMessage('not json')).toBeNull()
    expect(parseMessage(42)).toBeNull()
  })
})

describe('RealtimeClient', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => vi.useRealTimers())

  it('连接成功后分发消息，忽略非法消息', () => {
    const { client, statuses, messages } = setup()
    client.start()
    expect(FakeWebSocket.latest().url).toBe('ws://test/api/public/ws')
    FakeWebSocket.latest().open()
    FakeWebSocket.latest().receive({ type: 'snapshot', data: [makeServer()] })
    FakeWebSocket.latest().receive('garbage')
    expect(statuses).toEqual(['connecting', 'open'])
    expect(messages).toHaveLength(1)
    client.stop()
  })

  it('断开后立即轮询、按退避重连，恢复后停止轮询', () => {
    const { client, statuses, poll } = setup()
    client.start()
    FakeWebSocket.latest().open()
    FakeWebSocket.latest().drop()
    expect(statuses.at(-1)).toBe('polling')
    expect(poll).toHaveBeenCalledTimes(1)
    vi.advanceTimersByTime(999)
    expect(FakeWebSocket.instances).toHaveLength(1)
    vi.advanceTimersByTime(1)
    expect(FakeWebSocket.instances).toHaveLength(2)
    vi.advanceTimersByTime(5000)
    expect(poll).toHaveBeenCalledTimes(2)
    FakeWebSocket.latest().open()
    expect(statuses.at(-1)).toBe('open')
    vi.advanceTimersByTime(20000)
    expect(poll).toHaveBeenCalledTimes(2)
    client.stop()
  })

  it('stop 后关闭连接且不再重连', () => {
    const { client } = setup()
    client.start()
    const ws = FakeWebSocket.latest()
    client.stop()
    expect(ws.closed).toBe(true)
    ws.drop()
    vi.advanceTimersByTime(60000)
    expect(FakeWebSocket.instances).toHaveLength(1)
  })
})
```

`web/src/realtime/live.test.ts`：

```ts
import { describe, expect, it } from 'vitest'

import { makeServer } from '@/test/fixtures'

import { recordSpeed, resetSpeedHistory, useSpeedHistory } from './speed-history'
import { applyMessage, serversKey, wsUrl } from './use-live-servers'

describe('applyMessage', () => {
  it('snapshot 整体替换并排序', () => {
    const next = applyMessage([makeServer({ id: 'x' })], { type: 'snapshot', data: [makeServer({ id: 'b', name: 'b' }), makeServer({ id: 'a', name: 'a' })] })
    expect(next.map((s) => s.id)).toEqual(['a', 'b'])
  })

  it('delta 按 id 覆盖、保留其余', () => {
    const prev = [makeServer({ id: 'a', name: 'a', online: true }), makeServer({ id: 'b', name: 'b' })]
    const next = applyMessage(prev, { type: 'delta', data: [makeServer({ id: 'a', name: 'a', online: false })] })
    expect(next).toHaveLength(2)
    expect(next.find((s) => s.id === 'a')?.online).toBe(false)
  })
})

describe('serversKey / wsUrl', () => {
  it('登录与未登录使用不同缓存键', () => {
    expect(serversKey(true)).not.toEqual(serversKey(false))
  })

  it('按协议选择 ws/wss 并附带 token', () => {
    expect(wsUrl(null, { protocol: 'http:', host: 'a:8900' })).toBe('ws://a:8900/api/public/ws')
    expect(wsUrl('t k', { protocol: 'https:', host: 'a.com' })).toBe('wss://a.com/api/public/ws?token=t%20k')
  })
})

describe('recordSpeed', () => {
  it('汇总在线服务器网速，同一秒只保留最后一个样本', () => {
    resetSpeedHistory()
    recordSpeed([makeServer(), makeServer({ id: 'off', online: false })], 100)
    recordSpeed([makeServer()], 100)
    recordSpeed([makeServer()], 101)
    const samples = useSpeedHistory.getSnapshot()
    expect(samples).toHaveLength(2)
    expect(samples[0]).toEqual({ ts: 100, in: 1024, out: 2048 })
  })
})
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd web && pnpm test src/realtime`
Expected: FAIL（无法解析 `./client` 等）。

- [ ] **Step 3: 实现**

`web/src/realtime/client.ts`：

```ts
import type { ServerView } from '@/lib/types'

export type RealtimeStatus = 'connecting' | 'open' | 'polling'

export interface LiveMessage {
  type: 'snapshot' | 'delta'
  data: ServerView[]
}

export interface RealtimeOptions {
  url: () => string
  onMessage: (m: LiveMessage) => void
  onStatus: (s: RealtimeStatus) => void
  /** poll 断线期间的轮询回调（立即执行一次，之后每 pollInterval 毫秒一次） */
  poll: () => void
  createSocket?: (url: string) => WebSocket
  pollInterval?: number
  random?: () => number
}

/** backoff 第 attempt 次重连的等待毫秒数：1s 起翻倍、30s 封顶、±20% 抖动 */
export function backoff(attempt: number, random: () => number = Math.random): number {
  const base = Math.min(30_000, 1000 * 2 ** Math.min(attempt, 5))
  return Math.round(base * (0.8 + 0.4 * random()))
}

/** parseMessage 解析推送消息，非法消息返回 null */
export function parseMessage(raw: unknown): LiveMessage | null {
  if (typeof raw !== 'string') return null
  try {
    const m = JSON.parse(raw) as { type?: unknown; data?: unknown }
    if ((m.type === 'snapshot' || m.type === 'delta') && Array.isArray(m.data)) {
      return { type: m.type, data: m.data as ServerView[] }
    }
  } catch {
    // 忽略无法解析的消息
  }
  return null
}

/** RealtimeClient 维护浏览器 WebSocket：断线时退回轮询并按退避重连 */
export class RealtimeClient {
  private ws: WebSocket | null = null
  private attempt = 0
  private running = false
  private retryTimer: ReturnType<typeof setTimeout> | undefined
  private pollTimer: ReturnType<typeof setInterval> | undefined
  private readonly opts: RealtimeOptions

  constructor(opts: RealtimeOptions) {
    this.opts = opts
  }

  start(): void {
    if (this.running) return
    this.running = true
    this.attempt = 0
    this.opts.onStatus('connecting')
    this.connect()
  }

  stop(): void {
    this.running = false
    clearTimeout(this.retryTimer)
    this.stopPolling()
    const ws = this.ws
    this.ws = null
    if (ws) {
      ws.onopen = null
      ws.onmessage = null
      ws.onclose = null
      ws.close()
    }
  }

  private connect(): void {
    const create = this.opts.createSocket ?? ((u: string) => new WebSocket(u))
    let ws: WebSocket
    try {
      ws = create(this.opts.url())
    } catch {
      this.handleClose()
      return
    }
    this.ws = ws
    ws.onopen = () => {
      this.attempt = 0
      this.stopPolling()
      this.opts.onStatus('open')
    }
    ws.onmessage = (ev: MessageEvent) => {
      const m = parseMessage(ev.data)
      if (m) this.opts.onMessage(m)
    }
    ws.onclose = () => {
      if (this.ws !== ws) return
      this.ws = null
      this.handleClose()
    }
  }

  private handleClose(): void {
    if (!this.running) return
    this.opts.onStatus('polling')
    this.startPolling()
    const delay = backoff(this.attempt++, this.opts.random)
    this.retryTimer = setTimeout(() => {
      if (this.running) this.connect()
    }, delay)
  }

  private startPolling(): void {
    if (this.pollTimer !== undefined) return
    this.opts.poll()
    this.pollTimer = setInterval(() => this.opts.poll(), this.opts.pollInterval ?? 5000)
  }

  private stopPolling(): void {
    if (this.pollTimer === undefined) return
    clearInterval(this.pollTimer)
    this.pollTimer = undefined
  }
}
```

`web/src/realtime/speed-history.ts`：

```ts
import { useSyncExternalStore } from 'react'

import type { ServerView } from '@/lib/types'

export interface SpeedSample {
  ts: number
  in: number
  out: number
}

const MAX = 300
let samples: SpeedSample[] = []
const listeners = new Set<() => void>()

function subscribe(listener: () => void) {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}

function getSnapshot() {
  return samples
}

/** recordSpeed 记录一次全部在线服务器的总网速（最多保留 300 个样本，约 10 分钟） */
export function recordSpeed(servers: ServerView[], nowSec = Math.floor(Date.now() / 1000)): void {
  let inSum = 0
  let outSum = 0
  for (const s of servers) {
    if (s.online && s.metrics) {
      inSum += s.metrics.net_in_speed
      outSum += s.metrics.net_out_speed
    }
  }
  const sample = { ts: nowSec, in: inSum, out: outSum }
  const last = samples.at(-1)
  samples = last && last.ts === nowSec ? [...samples.slice(0, -1), sample] : [...samples, sample].slice(-MAX)
  listeners.forEach((l) => l())
}

export function resetSpeedHistory(): void {
  samples = []
  listeners.forEach((l) => l())
}

/** useSpeedHistory 汇总卡迷你曲线所用的网速历史 */
export function useSpeedHistory(): SpeedSample[] {
  return useSyncExternalStore(subscribe, getSnapshot)
}
useSpeedHistory.getSnapshot = getSnapshot
```

`web/src/realtime/use-live-servers.ts`：

```ts
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useState } from 'react'

import { api } from '@/lib/api'
import { useToken } from '@/lib/auth'
import { sortServers } from '@/lib/server'
import type { ServerView } from '@/lib/types'

import { RealtimeClient, type LiveMessage, type RealtimeStatus } from './client'
import { recordSpeed } from './speed-history'

/** serversKey 登录与未登录的服务器列表使用不同缓存，避免隐藏服务器残留在公开视图 */
export function serversKey(authed: boolean) {
  return ['servers', authed ? 'authed' : 'public'] as const
}

/** applyMessage 把推送消息合并进现有列表 */
export function applyMessage(prev: ServerView[] | undefined, m: LiveMessage): ServerView[] {
  if (m.type === 'snapshot') return sortServers(m.data)
  const map = new Map((prev ?? []).map((s) => [s.id, s]))
  for (const s of m.data) map.set(s.id, s)
  return sortServers([...map.values()])
}

export function wsUrl(token: string | null, loc: Pick<Location, 'protocol' | 'host'> = window.location): string {
  const scheme = loc.protocol === 'https:' ? 'wss:' : 'ws:'
  const query = token ? `?token=${encodeURIComponent(token)}` : ''
  return `${scheme}//${loc.host}/api/public/ws${query}`
}

/** useLiveServers 服务器实时列表：HTTP 快照 + WebSocket 推送，断线时每 5 秒轮询；页面隐藏时断开 */
export function useLiveServers() {
  const token = useToken()
  const authed = token !== null
  const qc = useQueryClient()
  const [status, setStatus] = useState<RealtimeStatus>('connecting')

  const query = useQuery({
    queryKey: serversKey(authed),
    queryFn: async () => {
      const list = sortServers(await api.get<ServerView[]>('/api/public/servers'))
      recordSpeed(list)
      return list
    },
  })

  useEffect(() => {
    const key = serversKey(authed)
    const client = new RealtimeClient({
      url: () => wsUrl(token),
      onMessage: (m) =>
        qc.setQueryData<ServerView[]>(key, (prev) => {
          const next = applyMessage(prev, m)
          recordSpeed(next)
          return next
        }),
      onStatus: setStatus,
      poll: () => {
        void qc.invalidateQueries({ queryKey: key })
      },
    })
    client.start()
    const onVisibility = () => {
      if (document.hidden) client.stop()
      else client.start()
    }
    document.addEventListener('visibilitychange', onVisibility)
    return () => {
      document.removeEventListener('visibilitychange', onVisibility)
      client.stop()
    }
  }, [token, authed, qc])

  return { servers: query.data ?? [], isLoading: query.isLoading, error: query.error, status }
}
```

`web/src/hooks/use-now.ts`：

```ts
import { useEffect, useState } from 'react'

/** useNow 当前 Unix 秒，按间隔刷新（用于“x 秒前”“x 天后到期”） */
export function useNow(intervalMs = 1000): number {
  const [now, setNow] = useState(() => Math.floor(Date.now() / 1000))
  useEffect(() => {
    const id = setInterval(() => setNow(Math.floor(Date.now() / 1000)), intervalMs)
    return () => clearInterval(id)
  }, [intervalMs])
  return now
}
```

在 `web/src/test/setup.ts` 顶部加入：

```ts
import { resetSpeedHistory } from '@/realtime/speed-history'

import { FakeWebSocket } from './fake-ws'
```

并在 `beforeEach` 回调末尾加入：

```ts
  FakeWebSocket.reset()
  vi.stubGlobal('WebSocket', FakeWebSocket)
  resetSpeedHistory()
```

- [ ] **Step 4: 运行检查**

Run: `cd web && pnpm typecheck && pnpm lint && pnpm test`
Expected: 全部通过。若 react-hooks 规则对 `useSpeedHistory.getSnapshot = getSnapshot` 这一赋值报错，改为额外导出 `export const speedSnapshot = getSnapshot`，并把测试中的 `useSpeedHistory.getSnapshot()` 换成 `speedSnapshot()`，在报告中说明。

- [ ] **Step 5: 提交**

```bash
git add web/src
git commit -m "feat(web): 新增实时数据层（WebSocket 推送与轮询回退）"
```

---

### Task 7: 服务器卡片与基础展示组件

**Files:**
- Create: `web/src/components/usage-bar.tsx`、`web/src/components/flag.tsx`、`web/src/components/status-bits.tsx`
- Create: `web/src/features/status/server-card.tsx`
- Test: `web/src/features/status/server-card.test.tsx`

**Interfaces:**
- Consumes: `format.ts`、`server.ts`（Task 3）；`useLang`（Task 3）
- Produces:
  - `UsageBar({ value: number | null; className? })`（role=progressbar，按阈值着色，null 时不填充）
  - `Flag({ code; className? })`（非两位字母返回 null）
  - `OnlineDot({ online })`、`StatusPill({ server })`、`ExpiryText({ days })`
  - `ServerCard({ server, now })`（整卡为指向 `/server/:id` 的链接）

- [ ] **Step 1: 写失败测试**

`web/src/features/status/server-card.test.tsx`：

```tsx
import { screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { makeMetrics, makeServer } from '@/test/fixtures'
import { renderWithProviders } from '@/test/render'

import { ServerCard } from './server-card'

const NOW = 1_700_000_000

describe('ServerCard', () => {
  it('显示名称、系统、2×2 指标与网速，并链接到详情页', () => {
    renderWithProviders(<ServerCard server={makeServer({ expire_at: NOW + 3 * 86400 })} now={NOW} />)
    const link = screen.getByRole('link')
    expect(link).toHaveAttribute('href', '/server/srv1')
    expect(screen.getByText('hk-01')).toBeInTheDocument()
    expect(screen.getByText('debian 12 · kvm · x86_64')).toBeInTheDocument()
    expect(screen.getByText('3 天后到期')).toHaveClass('text-warn')
    expect(screen.getByText('在线 3 天 2 小时')).toBeInTheDocument()
    expect(screen.getByText('12%')).toBeInTheDocument()
    expect(screen.getByText('50%')).toBeInTheDocument()
    expect(screen.getByText('400 GB / 1.0 TB')).toBeInTheDocument()
    expect(screen.getByText(/↓ 1\.0 KB\/s/)).toBeInTheDocument()
    expect(screen.getAllByRole('progressbar')).toHaveLength(4)
  })

  it('从未上报、无配额、长期的服务器显示占位而不报错', () => {
    const s = makeServer({
      online: false,
      metrics: null,
      static: null,
      last_seen: 0,
      traffic: { in: 0, out: 0, used: 0, limit: null, mode: 'sum', reset_day: 1, period: '2026-09-01' },
    })
    renderWithProviders(<ServerCard server={s} now={NOW} />)
    expect(screen.getByText('离线')).toBeInTheDocument()
    expect(screen.getByText('长期')).toBeInTheDocument()
    expect(screen.getByText('∞')).toBeInTheDocument()
    expect(screen.getAllByText('—')).toHaveLength(3)
    expect(screen.getByText('尚未上报')).toBeInTheDocument()
  })

  it('高负载时进度条变红', () => {
    renderWithProviders(<ServerCard server={makeServer({ metrics: makeMetrics({ cpu: 95 }) })} now={NOW} />)
    const cpuBar = screen.getAllByRole('progressbar')[0]
    expect(cpuBar.firstElementChild).toHaveClass('bg-bad')
  })
})
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd web && pnpm test src/features/status`
Expected: FAIL（无法解析 `./server-card`）。

- [ ] **Step 3: 实现**

`web/src/components/usage-bar.tsx`：

```tsx
import { usageLevel, type Level } from '@/lib/server'
import { cn } from '@/lib/utils'

const COLOR: Record<Level, string> = { ok: 'bg-foreground', warn: 'bg-warn', bad: 'bg-bad' }

/** UsageBar 细进度条：<70% 前景色、70–90% 琥珀、≥90% 红；value 为 null 时只显示轨道 */
export function UsageBar({ value, className }: { value: number | null; className?: string }) {
  const v = value ?? 0
  return (
    <div
      role="progressbar"
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={Math.round(v)}
      className={cn('h-1 w-full overflow-hidden rounded-full bg-muted', className)}
    >
      {value != null && <div className={cn('h-full rounded-full transition-[width] duration-500', COLOR[usageLevel(v)])} style={{ width: `${v}%` }} />}
    </div>
  )
}
```

`web/src/components/flag.tsx`：

```tsx
import { cn } from '@/lib/utils'

/** Flag 国旗图标（flag-icons SVG），非两位字母代码时不渲染 */
export function Flag({ code, className }: { code: string; className?: string }) {
  if (!/^[A-Za-z]{2}$/.test(code)) return null
  const upper = code.toUpperCase()
  return <span className={cn('fi shrink-0 rounded-[2px]', `fi-${code.toLowerCase()}`, className)} title={upper} aria-label={upper} role="img" />
}
```

`web/src/components/status-bits.tsx`：

```tsx
import { useTranslation } from 'react-i18next'

import { useLang } from '@/i18n/use-lang'
import { formatDuration } from '@/lib/format'
import { expiryLevel } from '@/lib/server'
import type { ServerView } from '@/lib/types'
import { cn } from '@/lib/utils'

export function OnlineDot({ online }: { online: boolean }) {
  return <span className={cn('inline-block size-1.5 shrink-0 rounded-full', online ? 'bg-online' : 'bg-bad')} />
}

/** StatusPill 在线显示运行时长，离线显示红色标记 */
export function StatusPill({ server }: { server: ServerView }) {
  const { t } = useTranslation()
  const lang = useLang()
  if (!server.online) {
    return (
      <span className="inline-flex shrink-0 items-center gap-1 rounded-full border border-bad/30 px-2 py-0.5 text-[11px] text-bad">
        <OnlineDot online={false} />
        {t('status.offline')}
      </span>
    )
  }
  return (
    <span className="inline-flex shrink-0 items-center gap-1 rounded-full border px-2 py-0.5 text-[11px]">
      <OnlineDot online />
      {server.metrics ? t('status.onlineFor', { duration: formatDuration(server.metrics.uptime, lang) }) : t('status.online')}
    </span>
  )
}

/** ExpiryText 到期提示：长期 / N 天后到期（≤7 天琥珀）/ 已过期（红） */
export function ExpiryText({ days }: { days: number | null }) {
  const { t } = useTranslation()
  if (days == null) return <span>{t('expire.never')}</span>
  const level = expiryLevel(days)
  return (
    <span className={cn(level === 'warn' && 'text-warn', level === 'bad' && 'text-bad')}>
      {days <= 0 ? t('expire.expired') : t('expire.days', { count: days })}
    </span>
  )
}
```

`web/src/features/status/server-card.tsx`：

```tsx
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router-dom'

import { Flag } from '@/components/flag'
import { ExpiryText, StatusPill } from '@/components/status-bits'
import { UsageBar } from '@/components/usage-bar'
import { useLang } from '@/i18n/use-lang'
import { daysUntil, formatAgo, formatBytes, formatPercent, formatSpeed } from '@/lib/format'
import { cpuPct, diskPct, memPct, osLabel, trafficPct } from '@/lib/server'
import type { ServerView } from '@/lib/types'
import { cn } from '@/lib/utils'

function Metric({ label, extra, value, valueText, detail }: { label: string; extra?: string; value: number | null; valueText?: string; detail: string }) {
  return (
    <div className="min-w-0">
      <div className="flex items-baseline justify-between gap-2 text-xs">
        <span className="truncate">
          {label}
          {extra && <span className="ml-1 text-muted-foreground">{extra}</span>}
        </span>
        <span className="font-medium tabular">{valueText ?? (value == null ? '—' : formatPercent(value))}</span>
      </div>
      <UsageBar value={value} className="my-1.5" />
      <div className="truncate text-[11px] text-muted-foreground tabular">{detail || ' '}</div>
    </div>
  )
}

/** ServerCard 首页卡片：头部状态、2×2 指标、底部网速与累计流量 */
export function ServerCard({ server: s, now }: { server: ServerView; now: number }) {
  const { t } = useTranslation()
  const lang = useLang()
  const m = s.metrics
  const tp = trafficPct(s.traffic)

  return (
    <Link
      to={`/server/${s.id}`}
      className={cn('block rounded-xl border bg-card p-4 text-card-foreground transition-colors hover:border-foreground/25', !s.online && 'opacity-60')}
    >
      <div className="flex items-start justify-between gap-2">
        <div className="flex min-w-0 items-center gap-1.5">
          <span className="truncate font-semibold">{s.name}</span>
          <Flag code={s.country} />
        </div>
        <StatusPill server={s} />
      </div>
      <div className="mt-1 flex justify-between gap-2 text-xs text-muted-foreground">
        <span className="truncate">{osLabel(s)}</span>
        <span className="shrink-0">
          <ExpiryText days={daysUntil(s.expire_at, now)} />
        </span>
      </div>

      <div className="mt-4 grid grid-cols-2 gap-x-4 gap-y-3">
        <Metric
          label="CPU"
          extra={s.static?.cpu_cores ? t('metric.cores', { count: s.static.cpu_cores }) : undefined}
          value={m ? cpuPct(s) : null}
          detail={m ? `load ${m.load1.toFixed(2)} ${m.load5.toFixed(2)} ${m.load15.toFixed(2)}` : ''}
        />
        <Metric
          label={t('metric.mem')}
          value={m ? memPct(s) : null}
          detail={m && s.static ? `${formatBytes(m.mem_used)} / ${formatBytes(s.static.mem_total)}` : ''}
        />
        <Metric label={t('metric.disk')} value={m ? diskPct(s) : null} detail={m ? `${formatBytes(m.disk_used)} / ${formatBytes(m.disk_total)}` : ''} />
        <Metric
          label={t('metric.traffic')}
          value={tp}
          valueText={tp == null ? '∞' : undefined}
          detail={s.traffic.limit ? `${formatBytes(s.traffic.used)} / ${formatBytes(s.traffic.limit)}` : t('metric.thisMonth', { value: formatBytes(s.traffic.used) })}
        />
      </div>

      <div className="mt-3 flex justify-between gap-2 border-t pt-2.5 text-xs tabular">
        {s.online && m ? (
          <span>
            ↓ {formatSpeed(m.net_in_speed)}　↑ {formatSpeed(m.net_out_speed)}
          </span>
        ) : (
          <span className="text-muted-foreground">{s.last_seen ? t('status.lastSeen', { ago: formatAgo(s.last_seen, now, lang) }) : t('status.neverSeen')}</span>
        )}
        {m && (
          <span className="truncate text-muted-foreground">
            ↓ {formatBytes(m.net_in_total)} ↑ {formatBytes(m.net_out_total)}
          </span>
        )}
      </div>
    </Link>
  )
}
```

- [ ] **Step 4: 运行检查**

Run: `cd web && pnpm typecheck && pnpm lint && pnpm test`
Expected: 全部通过。

- [ ] **Step 5: 提交**

```bash
git add web/src
git commit -m "feat(web): 新增服务器卡片与进度条、国旗、状态组件"
```

---

### Task 8: 公开首页（汇总卡、筛选、卡片 / 列表视图）

**Files:**
- Create: `web/src/features/status/filters.ts`、`web/src/features/status/summary-cards.tsx`、`web/src/features/status/sparkline.tsx`、`web/src/features/status/filter-bar.tsx`、`web/src/features/status/server-table.tsx`、`web/src/components/connection-banner.tsx`
- Modify: `web/src/features/status/status-page.tsx`（整体替换占位）
- Test: `web/src/features/status/filters.test.ts`、`web/src/features/status/status-page.test.tsx`

**Interfaces:**
- Consumes: `useLiveServers`、`useSpeedHistory`、`useNow`（Task 6）；`ServerCard`、`UsageBar`、`Flag`、`OnlineDot`、`ExpiryText`（Task 7）；`SiteHeader`（Task 5）
- Produces:
  - `type Filter = { kind: 'all' } | { kind: 'group'; value: string } | { kind: 'country'; value: string } | { kind: 'offline' }`
  - `buildChips(servers)`、`applyFilter(servers, filter, query)`、`sameFilter(a, b)`、`type SortKey`、`sortBy(list, key, desc)`
  - `StatusPage`（具名导出，替换占位）

- [ ] **Step 1: 写失败测试**

`web/src/features/status/filters.test.ts`：

```ts
import { describe, expect, it } from 'vitest'

import { makeMetrics, makeServer } from '@/test/fixtures'

import { applyFilter, buildChips, sameFilter, sortBy } from './filters'

const servers = [
  makeServer({ id: 'a', name: 'alpha', group: 'HK', country: 'HK' }),
  makeServer({ id: 'b', name: 'beta', group: 'HK', country: 'JP', online: false }),
  makeServer({ id: 'c', name: 'gamma', group: '', country: '' }),
]

describe('buildChips', () => {
  it('统计分组、地区与离线数量（空值不计）', () => {
    const chips = buildChips(servers)
    expect(chips.groups).toEqual([{ value: 'HK', count: 2 }])
    expect(chips.countries).toEqual([
      { value: 'HK', count: 1 },
      { value: 'JP', count: 1 },
    ])
    expect(chips.offline).toBe(1)
  })
})

describe('applyFilter', () => {
  it('按分组、地区、离线与名称搜索过滤', () => {
    expect(applyFilter(servers, { kind: 'group', value: 'HK' }, '').map((s) => s.id)).toEqual(['a', 'b'])
    expect(applyFilter(servers, { kind: 'country', value: 'JP' }, '').map((s) => s.id)).toEqual(['b'])
    expect(applyFilter(servers, { kind: 'offline' }, '').map((s) => s.id)).toEqual(['b'])
    expect(applyFilter(servers, { kind: 'all' }, ' AL ').map((s) => s.id)).toEqual(['a'])
  })

  it('sameFilter 比较种类与值', () => {
    expect(sameFilter({ kind: 'group', value: 'HK' }, { kind: 'group', value: 'HK' })).toBe(true)
    expect(sameFilter({ kind: 'group', value: 'HK' }, { kind: 'country', value: 'HK' })).toBe(false)
    expect(sameFilter({ kind: 'all' }, { kind: 'all' })).toBe(true)
  })
})

describe('sortBy', () => {
  it('按指标排序，无数据的服务器排在升序末尾', () => {
    const list = [
      makeServer({ id: 'x', name: 'x', metrics: makeMetrics({ cpu: 50 }) }),
      makeServer({ id: 'y', name: 'y', metrics: null }),
      makeServer({ id: 'z', name: 'z', metrics: makeMetrics({ cpu: 90 }) }),
    ]
    expect(sortBy(list, 'cpu', true).map((s) => s.id)).toEqual(['z', 'x', 'y'])
    expect(sortBy(list, 'name', false).map((s) => s.id)).toEqual(['x', 'y', 'z'])
    expect(sortBy(list, 'expire', false).map((s) => s.id)).toEqual(['x', 'y', 'z'])
  })
})
```

`web/src/features/status/status-page.test.tsx`：

```tsx
import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'

import { mockFetch } from '@/test/fetch'
import { makeServer } from '@/test/fixtures'
import { renderWithProviders } from '@/test/render'

import { StatusPage } from './status-page'

const SITE = { site_title: '我的探针', show_price: false }

describe('StatusPage', () => {
  it('显示汇总，支持离线筛选、搜索与切换列表视图', async () => {
    mockFetch({
      'GET /api/public/site': SITE,
      'GET /api/public/servers': [
        makeServer({ id: 'a', name: 'alpha', group: 'HK', country: 'HK' }),
        makeServer({ id: 'b', name: 'beta', group: 'JP', country: 'JP', online: false }),
      ],
    })
    const user = userEvent.setup()
    renderWithProviders(<StatusPage />)

    expect(await screen.findByText('alpha')).toBeInTheDocument()
    expect(screen.getByText('我的探针')).toBeInTheDocument()
    expect(screen.getByText('1 台离线')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: /^离线/ }))
    expect(screen.queryByText('alpha')).not.toBeInTheDocument()
    expect(screen.getByText('beta')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: /^全部/ }))
    await user.type(screen.getByPlaceholderText('搜索名称'), 'alp')
    expect(screen.queryByText('beta')).not.toBeInTheDocument()

    await user.click(screen.getByRole('radio', { name: '列表视图' }))
    expect(screen.getByRole('table')).toBeInTheDocument()
    expect(localStorage.getItem('sss.view')).toBe('list')
  })

  it('没有服务器时显示提示', async () => {
    mockFetch({ 'GET /api/public/site': SITE, 'GET /api/public/servers': [] })
    renderWithProviders(<StatusPage />)
    expect(await screen.findByText('还没有服务器')).toBeInTheDocument()
  })
})
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd web && pnpm test src/features/status`
Expected: FAIL（无法解析 `./filters`；StatusPage 仍为占位）。

- [ ] **Step 3: 实现筛选与排序**

`web/src/features/status/filters.ts`：

```ts
import { cpuPct, diskPct, memPct } from '@/lib/server'
import type { ServerView } from '@/lib/types'

export type Filter = { kind: 'all' } | { kind: 'group'; value: string } | { kind: 'country'; value: string } | { kind: 'offline' }

export interface ChipCount {
  value: string
  count: number
}

function toList(m: Map<string, number>): ChipCount[] {
  return [...m.entries()].map(([value, count]) => ({ value, count })).sort((a, b) => b.count - a.count || a.value.localeCompare(b.value))
}

/** buildChips 统计筛选标签：分组、地区（空值不计）与离线数量 */
export function buildChips(servers: ServerView[]) {
  const groups = new Map<string, number>()
  const countries = new Map<string, number>()
  let offline = 0
  for (const s of servers) {
    if (s.group) groups.set(s.group, (groups.get(s.group) ?? 0) + 1)
    if (s.country) countries.set(s.country, (countries.get(s.country) ?? 0) + 1)
    if (!s.online) offline++
  }
  return { groups: toList(groups), countries: toList(countries), offline }
}

export function applyFilter(servers: ServerView[], filter: Filter, query: string): ServerView[] {
  const q = query.trim().toLowerCase()
  return servers.filter((s) => {
    if (filter.kind === 'group' && s.group !== filter.value) return false
    if (filter.kind === 'country' && s.country !== filter.value) return false
    if (filter.kind === 'offline' && s.online) return false
    return !q || s.name.toLowerCase().includes(q)
  })
}

export function sameFilter(a: Filter, b: Filter): boolean {
  if (a.kind !== b.kind) return false
  if ((a.kind === 'group' || a.kind === 'country') && (b.kind === 'group' || b.kind === 'country')) return a.value === b.value
  return true
}

export type SortKey = 'name' | 'cpu' | 'mem' | 'disk' | 'traffic' | 'speed' | 'uptime' | 'expire'

const MISSING = -1
const NEVER = Number.MAX_SAFE_INTEGER

function sortValue(s: ServerView, key: SortKey): number | string {
  const m = s.metrics
  switch (key) {
    case 'name':
      return s.name
    case 'cpu':
      return m ? cpuPct(s) : MISSING
    case 'mem':
      return m ? memPct(s) : MISSING
    case 'disk':
      return m ? diskPct(s) : MISSING
    case 'traffic':
      return s.traffic.used
    case 'speed':
      return m && s.online ? m.net_in_speed + m.net_out_speed : MISSING
    case 'uptime':
      return m ? m.uptime : MISSING
    case 'expire':
      return s.expire_at ?? NEVER
  }
}

/** sortBy 列表视图表头排序，返回新数组 */
export function sortBy(list: ServerView[], key: SortKey, desc: boolean): ServerView[] {
  return [...list].sort((a, b) => {
    const va = sortValue(a, key)
    const vb = sortValue(b, key)
    const r = typeof va === 'string' && typeof vb === 'string' ? va.localeCompare(vb) : Number(va) - Number(vb)
    return desc ? -r : r
  })
}
```

注意：`sortBy(list, 'cpu', true)` 期望 `['z','x','y']`（缺数据的 -1 在降序时排最后）；`sortBy(list, 'expire', false)` 三者都为 NEVER，保持原顺序（Array.prototype.sort 稳定）。

- [ ] **Step 4: 实现首页组件**

`web/src/features/status/sparkline.tsx`：

```tsx
import type { SpeedSample } from '@/realtime/speed-history'

/** Sparkline 汇总卡中的迷你双线图（下行实线、上行浅色），纯 SVG 避免首页加载图表库 */
export function Sparkline({ samples }: { samples: SpeedSample[] }) {
  if (samples.length < 2) return <div className="h-5" />
  const max = Math.max(1, ...samples.map((s) => Math.max(s.in, s.out)))
  const line = (pick: (s: SpeedSample) => number) =>
    samples.map((s, i) => `${((i / (samples.length - 1)) * 100).toFixed(2)},${(19 - (pick(s) / max) * 18).toFixed(2)}`).join(' ')
  return (
    <svg viewBox="0 0 100 20" preserveAspectRatio="none" className="h-5 w-full" aria-hidden>
      <polyline fill="none" stroke="currentColor" strokeWidth="1.2" vectorEffect="non-scaling-stroke" className="text-foreground" points={line((s) => s.in)} />
      <polyline fill="none" stroke="currentColor" strokeWidth="1.2" vectorEffect="non-scaling-stroke" className="text-muted-foreground/60" points={line((s) => s.out)} />
    </svg>
  )
}
```

`web/src/features/status/summary-cards.tsx`：

```tsx
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { formatBytes, formatPercent, formatSpeed } from '@/lib/format'
import { cpuPct } from '@/lib/server'
import type { ServerView } from '@/lib/types'
import { useSpeedHistory } from '@/realtime/speed-history'

import { Sparkline } from './sparkline'

function Stat({ label, value, sub }: { label: string; value: ReactNode; sub: ReactNode }) {
  return (
    <div className="min-w-0 rounded-xl border bg-card p-4">
      <div className="text-xs text-muted-foreground">{label}</div>
      <div className="mt-1 truncate text-2xl font-semibold tracking-tight tabular">{value}</div>
      <div className="mt-1 truncate text-xs text-muted-foreground tabular">{sub}</div>
    </div>
  )
}

/** SummaryCards 首页四张汇总卡：在线数、最忙节点、本月流量、实时网速 */
export function SummaryCards({ servers }: { servers: ServerView[] }) {
  const { t } = useTranslation()
  const history = useSpeedHistory()
  const online = servers.filter((s) => s.online)
  const offline = servers.length - online.length
  const busiest = online.reduce<ServerView | null>((best, s) => (!best || cpuPct(s) > cpuPct(best) ? s : best), null)
  const traffic = servers.reduce((a, s) => ({ in: a.in + s.traffic.in, out: a.out + s.traffic.out }), { in: 0, out: 0 })
  const speed = online.reduce((a, s) => ({ in: a.in + (s.metrics?.net_in_speed ?? 0), out: a.out + (s.metrics?.net_out_speed ?? 0) }), { in: 0, out: 0 })

  return (
    <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
      <Stat
        label={t('summary.nodes')}
        value={
          <>
            {online.length}
            <span className="text-base font-normal text-muted-foreground">/{servers.length}</span>
          </>
        }
        sub={offline > 0 ? t('summary.offline', { count: offline }) : t('summary.allOnline')}
      />
      <Stat label={t('summary.busiest')} value={busiest ? formatPercent(cpuPct(busiest)) : '—'} sub={busiest ? `${busiest.name} · CPU` : '—'} />
      <Stat label={t('summary.traffic')} value={formatBytes(traffic.in + traffic.out)} sub={`↓ ${formatBytes(traffic.in)}　↑ ${formatBytes(traffic.out)}`} />
      <Stat label={t('summary.speed')} value={formatSpeed(speed.in + speed.out)} sub={<Sparkline samples={history} />} />
    </div>
  )
}
```

`web/src/features/status/filter-bar.tsx`：

```tsx
import { LayoutGrid, List, Search } from 'lucide-react'
import { useMemo, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Flag } from '@/components/flag'
import { Input } from '@/components/ui/input'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import type { ServerView } from '@/lib/types'
import { cn } from '@/lib/utils'

import { buildChips, sameFilter, type Filter } from './filters'

export type ViewMode = 'card' | 'list'

interface Props {
  servers: ServerView[]
  filter: Filter
  onFilter: (f: Filter) => void
  query: string
  onQuery: (q: string) => void
  view: ViewMode
  onView: (v: ViewMode) => void
}

/** FilterBar 筛选标签（全部 / 分组 / 地区 / 离线）、名称搜索与视图切换 */
export function FilterBar({ servers, filter, onFilter, query, onQuery, view, onView }: Props) {
  const { t } = useTranslation()
  const chips = useMemo(() => buildChips(servers), [servers])

  const chip = (key: string, label: ReactNode, count: number, f: Filter) => (
    <button
      key={key}
      type="button"
      onClick={() => onFilter(f)}
      className={cn(
        'inline-flex items-center gap-1.5 rounded-full border px-3 py-1 text-xs transition-colors',
        sameFilter(filter, f) ? 'border-foreground bg-foreground text-background' : 'bg-card hover:bg-accent',
      )}
    >
      {label}
      <span className="tabular opacity-60">{count}</span>
    </button>
  )

  return (
    <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
      <div className="flex flex-wrap gap-1.5">
        {chip('all', t('filter.all'), servers.length, { kind: 'all' })}
        {chips.groups.map((g) => chip(`g:${g.value}`, g.value, g.count, { kind: 'group', value: g.value }))}
        {chips.countries.map((c) =>
          chip(
            `c:${c.value}`,
            <>
              <Flag code={c.value} />
              {c.value}
            </>,
            c.count,
            { kind: 'country', value: c.value },
          ),
        )}
        {chips.offline > 0 && chip('offline', t('filter.offline'), chips.offline, { kind: 'offline' })}
      </div>
      <div className="flex items-center gap-2">
        <div className="relative min-w-0 flex-1 md:flex-none">
          <Search className="pointer-events-none absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
          <Input value={query} onChange={(e) => onQuery(e.target.value)} placeholder={t('filter.search')} className="h-8 w-full pl-8 text-sm md:w-48" />
        </div>
        <ToggleGroup type="single" variant="outline" size="sm" value={view} onValueChange={(v) => v && onView(v as ViewMode)}>
          <ToggleGroupItem value="card" aria-label={t('view.card')}>
            <LayoutGrid className="size-4" />
          </ToggleGroupItem>
          <ToggleGroupItem value="list" aria-label={t('view.list')}>
            <List className="size-4" />
          </ToggleGroupItem>
        </ToggleGroup>
      </div>
    </div>
  )
}
```

`web/src/features/status/server-table.tsx`：

```tsx
import { ChevronDown, ChevronUp } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router-dom'

import { Flag } from '@/components/flag'
import { ExpiryText, OnlineDot } from '@/components/status-bits'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { UsageBar } from '@/components/usage-bar'
import { useLang } from '@/i18n/use-lang'
import { daysUntil, formatBytes, formatDuration, formatPercent, formatSpeed } from '@/lib/format'
import { cpuPct, diskPct, memPct, osLabel, trafficPct } from '@/lib/server'
import type { ServerView } from '@/lib/types'
import { cn } from '@/lib/utils'

import { sortBy, type SortKey } from './filters'

function BarCell({ value, text }: { value: number | null; text?: string }) {
  return (
    <TableCell className="w-28">
      <div className="text-xs tabular">{text ?? (value == null ? '—' : formatPercent(value))}</div>
      {value != null && <UsageBar value={value} className="mt-1" />}
    </TableCell>
  )
}

/** ServerTable 首页列表视图：表头点击排序，整行点击进入详情 */
export function ServerTable({ servers, now }: { servers: ServerView[]; now: number }) {
  const { t } = useTranslation()
  const lang = useLang()
  const navigate = useNavigate()
  const [sort, setSort] = useState<{ key: SortKey; desc: boolean } | null>(null)
  const rows = useMemo(() => (sort ? sortBy(servers, sort.key, sort.desc) : servers), [servers, sort])

  const head = (key: SortKey, label: string, className?: string) => (
    <TableHead className={className}>
      <button
        type="button"
        className="inline-flex items-center gap-1"
        onClick={() => setSort((p) => (p?.key === key ? { key, desc: !p.desc } : { key, desc: key !== 'name' }))}
      >
        {label}
        {sort?.key === key && (sort.desc ? <ChevronDown className="size-3" /> : <ChevronUp className="size-3" />)}
      </button>
    </TableHead>
  )

  return (
    <div className="overflow-x-auto rounded-xl border bg-card">
      <Table>
        <TableHeader>
          <TableRow>
            {head('name', t('table.name'))}
            <TableHead className="hidden xl:table-cell">{t('table.system')}</TableHead>
            {head('cpu', 'CPU')}
            {head('mem', t('table.mem'))}
            {head('disk', t('table.disk'), 'hidden sm:table-cell')}
            {head('traffic', t('table.traffic'), 'hidden sm:table-cell')}
            {head('speed', t('table.speed'), 'hidden md:table-cell')}
            {head('uptime', t('table.uptime'), 'hidden lg:table-cell')}
            {head('expire', t('table.expire'), 'hidden lg:table-cell')}
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((s) => {
            const m = s.metrics
            const tp = trafficPct(s.traffic)
            return (
              <TableRow key={s.id} className={cn('cursor-pointer', !s.online && 'opacity-60')} onClick={() => navigate(`/server/${s.id}`)}>
                <TableCell>
                  <div className="flex items-center gap-2">
                    <OnlineDot online={s.online} />
                    <span className="font-medium">{s.name}</span>
                    <Flag code={s.country} />
                  </div>
                </TableCell>
                <TableCell className="hidden text-xs text-muted-foreground xl:table-cell">{osLabel(s)}</TableCell>
                <BarCell value={m ? cpuPct(s) : null} />
                <BarCell value={m ? memPct(s) : null} />
                <TableCell className="hidden w-28 sm:table-cell">
                  <div className="text-xs tabular">{m ? formatPercent(diskPct(s)) : '—'}</div>
                  {m && <UsageBar value={diskPct(s)} className="mt-1" />}
                </TableCell>
                <TableCell className="hidden w-28 sm:table-cell">
                  <div className="text-xs tabular">{tp == null ? formatBytes(s.traffic.used) : formatPercent(tp)}</div>
                  {tp != null && <UsageBar value={tp} className="mt-1" />}
                </TableCell>
                <TableCell className="hidden whitespace-nowrap text-xs tabular md:table-cell">
                  {m && s.online ? `↓ ${formatSpeed(m.net_in_speed)} ↑ ${formatSpeed(m.net_out_speed)}` : '—'}
                </TableCell>
                <TableCell className="hidden text-xs lg:table-cell">{m ? formatDuration(m.uptime, lang) : '—'}</TableCell>
                <TableCell className="hidden text-xs lg:table-cell">
                  <ExpiryText days={daysUntil(s.expire_at, now)} />
                </TableCell>
              </TableRow>
            )
          })}
        </TableBody>
      </Table>
    </div>
  )
}
```

`web/src/components/connection-banner.tsx`：

```tsx
import { useTranslation } from 'react-i18next'

export function ConnectionBanner() {
  const { t } = useTranslation()
  return <div className="border-b bg-warn/10 px-4 py-1.5 text-center text-xs">{t('conn.polling')}</div>
}
```

`web/src/features/status/status-page.tsx`（整体替换）：

```tsx
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ConnectionBanner } from '@/components/connection-banner'
import { SiteHeader } from '@/components/site-header'
import { Skeleton } from '@/components/ui/skeleton'
import { useNow } from '@/hooks/use-now'
import { errorMessage } from '@/lib/api'
import { useLiveServers } from '@/realtime/use-live-servers'

import { FilterBar, type ViewMode } from './filter-bar'
import { applyFilter, type Filter } from './filters'
import { ServerCard } from './server-card'
import { ServerTable } from './server-table'
import { SummaryCards } from './summary-cards'

const VIEW_KEY = 'sss.view'

export function StatusPage() {
  const { t } = useTranslation()
  const { servers, isLoading, error, status } = useLiveServers()
  const now = useNow()
  const [filter, setFilter] = useState<Filter>({ kind: 'all' })
  const [query, setQuery] = useState('')
  const [view, setView] = useState<ViewMode>(() => (localStorage.getItem(VIEW_KEY) === 'list' ? 'list' : 'card'))
  const visible = useMemo(() => applyFilter(servers, filter, query), [servers, filter, query])

  const changeView = (v: ViewMode) => {
    localStorage.setItem(VIEW_KEY, v)
    setView(v)
  }

  let content
  if (isLoading) {
    content = (
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
        {[0, 1, 2].map((i) => (
          <Skeleton key={i} className="h-52 rounded-xl" />
        ))}
      </div>
    )
  } else if (error) {
    content = <p className="text-sm text-bad">{errorMessage(error)}</p>
  } else if (visible.length === 0) {
    content = <p className="py-16 text-center text-sm text-muted-foreground">{servers.length ? t('empty.noMatch') : t('empty.servers')}</p>
  } else if (view === 'card') {
    content = (
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
        {visible.map((s) => (
          <ServerCard key={s.id} server={s} now={now} />
        ))}
      </div>
    )
  } else {
    content = <ServerTable servers={visible} now={now} />
  }

  return (
    <>
      <SiteHeader />
      {status === 'polling' && <ConnectionBanner />}
      <main className="mx-auto max-w-[1400px] space-y-4 px-4 py-6">
        <SummaryCards servers={servers} />
        <FilterBar servers={servers} filter={filter} onFilter={setFilter} query={query} onQuery={setQuery} view={view} onView={changeView} />
        {content}
      </main>
    </>
  )
}
```

- [ ] **Step 5: 运行检查**

Run: `cd web && pnpm typecheck && pnpm lint && pnpm test`
Expected: 全部通过。

- [ ] **Step 6: 提交**

```bash
git add web/src
git commit -m "feat(web): 新增公开首页（汇总卡、筛选、卡片与列表视图）"
```

---

### Task 9: 服务器详情页

**Files:**
- Modify: `web/src/features/detail/detail-page.tsx`（整体替换占位）
- Create: `web/src/features/detail/info-grid.tsx`、`web/src/features/detail/disk-list.tsx`、`web/src/features/detail/metric-charts.tsx`
- Test: `web/src/features/detail/detail-page.test.tsx`

**Interfaces:**
- Consumes: `useLiveServers`、`useNow`（Task 6）；`SiteHeader`、`useSite`（Task 5）；`Flag`、`StatusPill`、`ExpiryText`、`UsageBar`（Task 7）
- Produces: `DetailPage`（具名导出）；`metric-charts.tsx` 默认导出 `MetricCharts({ points })`（懒加载，唯一引用 Recharts 的文件）

- [ ] **Step 1: 写失败测试**

`web/src/features/detail/detail-page.test.tsx`：

```tsx
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'

import type { Point } from '@/lib/types'
import { mockFetch } from '@/test/fetch'
import { makeServer } from '@/test/fixtures'
import { renderWithProviders } from '@/test/render'

import { DetailPage } from './detail-page'

vi.mock('./metric-charts', () => ({
  default: ({ points }: { points: Point[] }) => <div data-testid="charts">{points.length}</div>,
}))

const point: Point = { ts: 1, cpu: 1, mem: 1, disk: 1, net_in: 1, net_out: 1, load1: 1, tcp: 1 }
const SITE = { site_title: '我的探针', show_price: true }

describe('DetailPage', () => {
  it('显示信息网格、分区与历史图表，并可切换时间范围', async () => {
    mockFetch({
      'GET /api/public/site': SITE,
      'GET /api/public/servers': [makeServer({ price: 9.9, currency: '$', billing_cycle: 'yearly', expire_at: 1_900_000_000 })],
      'GET /api/public/servers/srv1/metrics?range=realtime': [point],
      'GET /api/public/servers/srv1/metrics?range=1h': [point, point],
    })
    const user = userEvent.setup()
    renderWithProviders(<DetailPage />, { route: '/server/srv1', path: '/server/:id' })

    expect(await screen.findByRole('heading', { name: 'hk-01' })).toBeInTheDocument()
    expect(screen.getByText('AMD EPYC 7B13 × 4')).toBeInTheDocument()
    expect(screen.getByText('每月 1 日重置')).toBeInTheDocument()
    expect(screen.getByText('$9.9 / 年付')).toBeInTheDocument()
    expect(screen.getByText('Agent 2.0.0')).toBeInTheDocument()
    expect(screen.getByText('/')).toBeInTheDocument()
    expect(await screen.findByTestId('charts')).toHaveTextContent('1')

    await user.click(screen.getByRole('tab', { name: '1 小时' }))
    await waitFor(() => expect(screen.getByTestId('charts')).toHaveTextContent('2'))
  })

  it('服务器不存在或已隐藏时提示', async () => {
    mockFetch({ 'GET /api/public/site': SITE, 'GET /api/public/servers': [] })
    renderWithProviders(<DetailPage />, { route: '/server/none', path: '/server/:id' })
    expect(await screen.findByText('服务器不存在或已隐藏')).toBeInTheDocument()
  })
})
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd web && pnpm test src/features/detail`
Expected: FAIL（占位页面没有标题等内容）。

- [ ] **Step 3: 实现**

`web/src/features/detail/info-grid.tsx`：

```tsx
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { ExpiryText } from '@/components/status-bits'
import type { Lang } from '@/lib/format'
import { daysUntil, formatBytes, formatDate } from '@/lib/format'
import type { ServerView } from '@/lib/types'

interface Item {
  label: string
  value: ReactNode
  sub?: ReactNode
}

/** InfoGrid 详情页顶部的系统、硬件、流量与续费信息 */
export function InfoGrid({ server: s, now, lang }: { server: ServerView; now: number; lang: Lang }) {
  const { t } = useTranslation()
  const st = s.static
  const items: Item[] = [
    { label: t('detail.system'), value: st ? `${st.platform || st.os} ${st.platform_version}`.trim() : '—', sub: st?.kernel },
    {
      label: t('detail.cpu'),
      value: st?.cpu_model ? `${st.cpu_model} × ${st.cpu_cores}` : '—',
      sub: s.metrics ? `load ${s.metrics.load1.toFixed(2)} · ${s.metrics.procs} procs · TCP ${s.metrics.tcp}` : undefined,
    },
    { label: t('detail.memory'), value: st ? `${formatBytes(st.mem_total)} / ${formatBytes(st.disk_total)} / ${formatBytes(st.swap_total)}` : '—' },
    { label: t('detail.arch'), value: st ? [st.arch, st.virtualization].filter(Boolean).join(' · ') || '—' : '—' },
    {
      label: t('detail.traffic'),
      value: `${formatBytes(s.traffic.used)}${s.traffic.limit ? ` / ${formatBytes(s.traffic.limit)}` : ''}`,
      sub: t('detail.resetDay', { day: s.traffic.reset_day }),
    },
    {
      label: t('detail.renew'),
      value:
        s.price != null ? (
          `${s.currency ?? ''}${s.price} / ${s.billing_cycle ? t(`billing.${s.billing_cycle}`) : '—'}`
        ) : (
          <ExpiryText days={daysUntil(s.expire_at, now)} />
        ),
      sub: s.expire_at ? t('detail.expireAt', { date: formatDate(s.expire_at, lang) }) : undefined,
    },
  ]
  return (
    <div className="grid gap-4 rounded-xl border bg-card p-4 sm:grid-cols-2 lg:grid-cols-3">
      {items.map((i) => (
        <div key={i.label} className="min-w-0">
          <div className="text-xs text-muted-foreground">{i.label}</div>
          <div className="truncate text-sm font-medium tabular">{i.value}</div>
          {i.sub && <div className="truncate text-xs text-muted-foreground">{i.sub}</div>}
        </div>
      ))}
    </div>
  )
}
```

`web/src/features/detail/disk-list.tsx`：

```tsx
import { useTranslation } from 'react-i18next'

import { UsageBar } from '@/components/usage-bar'
import { formatBytes, formatPercent, percent } from '@/lib/format'
import type { Disk } from '@/lib/types'

export function DiskList({ disks }: { disks: Disk[] }) {
  const { t } = useTranslation()
  return (
    <div className="rounded-xl border bg-card p-4">
      <div className="mb-3 text-sm font-medium">{t('detail.disks')}</div>
      <div className="grid gap-4 sm:grid-cols-2">
        {disks.map((d) => {
          const p = percent(d.used, d.total)
          return (
            <div key={d.mount} className="min-w-0">
              <div className="flex justify-between gap-2 text-xs">
                <span className="truncate">
                  <span className="font-medium">{d.mount}</span> <span className="text-muted-foreground">{d.fstype}</span>
                </span>
                <span className="tabular">{formatPercent(p)}</span>
              </div>
              <UsageBar value={p} className="my-1.5" />
              <div className="text-[11px] text-muted-foreground tabular">
                {formatBytes(d.used)} / {formatBytes(d.total)}
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}
```

`web/src/features/detail/metric-charts.tsx`：

```tsx
import { useTranslation } from 'react-i18next'
import { Area, AreaChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'

import { useLang } from '@/i18n/use-lang'
import { formatSpeed } from '@/lib/format'
import type { Point } from '@/lib/types'

type Unit = 'percent' | 'speed' | 'number'

interface Line {
  key: keyof Point
  label: string
  muted?: boolean
}

const AXIS_TICK = { fontSize: 11, fill: 'var(--muted-foreground)' }

function formatter(unit: Unit): (v: number) => string {
  if (unit === 'percent') return (v) => `${Math.round(v)}%`
  if (unit === 'speed') return formatSpeed
  return (v) => String(Math.round(v * 100) / 100)
}

function ChartCard({ title, points, lines, unit, tick }: { title: string; points: Point[]; lines: Line[]; unit: Unit; tick: (ts: number) => string }) {
  const fmt = formatter(unit)
  return (
    <div className="rounded-xl border bg-card p-4">
      <div className="mb-2 text-sm font-medium">{title}</div>
      <div className="h-40">
        <ResponsiveContainer width="100%" height="100%">
          <AreaChart data={points} margin={{ top: 4, right: 4, left: 0, bottom: 0 }}>
            <CartesianGrid vertical={false} stroke="var(--border)" />
            <XAxis dataKey="ts" tickFormatter={tick} tick={AXIS_TICK} tickLine={false} axisLine={false} minTickGap={48} />
            <YAxis tickFormatter={fmt} tick={AXIS_TICK} tickLine={false} axisLine={false} width={64} domain={unit === 'percent' ? [0, 100] : [0, 'auto']} />
            <Tooltip
              labelFormatter={(v) => tick(Number(v))}
              formatter={(v) => fmt(Number(v))}
              contentStyle={{ background: 'var(--popover)', border: '1px solid var(--border)', borderRadius: 8, fontSize: 12 }}
            />
            {lines.map((l) => (
              <Area
                key={l.key}
                type="monotone"
                dataKey={l.key}
                name={l.label}
                stroke={l.muted ? 'var(--muted-foreground)' : 'var(--foreground)'}
                fill={l.muted ? 'transparent' : 'var(--muted)'}
                strokeWidth={1.5}
                dot={false}
                isAnimationActive={false}
              />
            ))}
          </AreaChart>
        </ResponsiveContainer>
      </div>
    </div>
  )
}

/** MetricCharts 详情页 5 张灰阶历史图（懒加载，避免首页引入图表库） */
export default function MetricCharts({ points }: { points: Point[] }) {
  const { t } = useTranslation()
  const lang = useLang()
  const tick = (ts: number) => new Date(ts * 1000).toLocaleTimeString(lang, { hour: '2-digit', minute: '2-digit' })
  return (
    <div className="grid gap-3">
      <ChartCard title="CPU" points={points} lines={[{ key: 'cpu', label: 'CPU' }]} unit="percent" tick={tick} />
      <ChartCard title={t('metric.mem')} points={points} lines={[{ key: 'mem', label: t('metric.mem') }]} unit="percent" tick={tick} />
      <ChartCard
        title={t('detail.network')}
        points={points}
        lines={[
          { key: 'net_in', label: t('detail.inbound') },
          { key: 'net_out', label: t('detail.outbound'), muted: true },
        ]}
        unit="speed"
        tick={tick}
      />
      <ChartCard title={t('metric.disk')} points={points} lines={[{ key: 'disk', label: t('metric.disk') }]} unit="percent" tick={tick} />
      <ChartCard
        title={t('detail.loadTcp')}
        points={points}
        lines={[
          { key: 'load1', label: 'load1' },
          { key: 'tcp', label: 'TCP', muted: true },
        ]}
        unit="number"
        tick={tick}
      />
    </div>
  )
}
```

`web/src/features/detail/detail-page.tsx`（整体替换）：

```tsx
import { useQuery } from '@tanstack/react-query'
import { ArrowLeft } from 'lucide-react'
import { lazy, Suspense, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useParams } from 'react-router-dom'

import { Flag } from '@/components/flag'
import { SiteHeader } from '@/components/site-header'
import { StatusPill } from '@/components/status-bits'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { useNow } from '@/hooks/use-now'
import { useLang } from '@/i18n/use-lang'
import { api } from '@/lib/api'
import { RANGES, type Point, type Range } from '@/lib/types'
import { useLiveServers } from '@/realtime/use-live-servers'

import { DiskList } from './disk-list'
import { InfoGrid } from './info-grid'

const MetricCharts = lazy(() => import('./metric-charts'))

export function DetailPage() {
  const { id = '' } = useParams()
  const { t } = useTranslation()
  const lang = useLang()
  const now = useNow()
  const { servers, isLoading } = useLiveServers()
  const [range, setRange] = useState<Range>('realtime')
  const server = servers.find((s) => s.id === id)

  const points = useQuery({
    queryKey: ['metrics', id, range],
    queryFn: () => api.get<Point[]>(`/api/public/servers/${encodeURIComponent(id)}/metrics?range=${range}`),
    enabled: server !== undefined,
    refetchInterval: range === 'realtime' ? 2000 : 60_000,
  })

  let body
  if (isLoading) {
    body = <Skeleton className="h-48 w-full rounded-xl" />
  } else if (!server) {
    body = <p className="py-16 text-center text-sm text-muted-foreground">{t('detail.notFound')}</p>
  } else {
    body = (
      <>
        <div className="flex flex-wrap items-center gap-2">
          <h1 className="text-xl font-semibold tracking-tight">{server.name}</h1>
          <Flag code={server.country} />
          <StatusPill server={server} />
          {server.static?.agent_version && (
            <Badge variant="outline" className="font-normal">
              Agent {server.static.agent_version}
            </Badge>
          )}
        </div>
        <InfoGrid server={server} now={now} lang={lang} />
        {server.metrics && server.metrics.disks.length > 0 && <DiskList disks={server.metrics.disks} />}
        <section className="space-y-3">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <h2 className="text-sm font-medium">{t('detail.history')}</h2>
            <Tabs value={range} onValueChange={(v) => setRange(v as Range)}>
              <TabsList>
                {RANGES.map((r) => (
                  <TabsTrigger key={r} value={r}>
                    {t(`range.${r}`)}
                  </TabsTrigger>
                ))}
              </TabsList>
            </Tabs>
          </div>
          {points.data && points.data.length > 0 ? (
            <Suspense fallback={<Skeleton className="h-40 w-full rounded-xl" />}>
              <MetricCharts points={points.data} />
            </Suspense>
          ) : (
            <p className="rounded-xl border bg-card py-12 text-center text-sm text-muted-foreground">
              {points.isLoading ? t('common.loading') : t('detail.noData')}
            </p>
          )}
        </section>
      </>
    )
  }

  return (
    <>
      <SiteHeader />
      <main className="mx-auto max-w-[1400px] space-y-4 px-4 py-6">
        <Button asChild variant="ghost" size="sm" className="-ml-2">
          <Link to="/">
            <ArrowLeft className="size-4" />
            {t('nav.back')}
          </Link>
        </Button>
        {body}
      </main>
    </>
  )
}
```

- [ ] **Step 4: 运行检查**

Run: `cd web && pnpm typecheck && pnpm lint && pnpm test && pnpm exec vite build`
Expected: 全部通过；构建产物中 Recharts 只出现在详情页相关 chunk（入口 chunk 不含 `recharts`，可用 `grep -l recharts ../internal/dashboard/web/dist/assets/*.js` 查看，入口 `index-*.js` 不在结果中）。

- [ ] **Step 5: 提交**

```bash
git add web/src
git commit -m "feat(web): 新增服务器详情页与历史趋势图"
```

---

### Task 10: 登录页与后台布局

**Files:**
- Modify: `web/src/features/auth/login-page.tsx`、`web/src/features/admin/admin-layout.tsx`（整体替换占位）
- Test: `web/src/features/auth/login-page.test.tsx`

**Interfaces:**
- Consumes: `adminApi.login`、`adminApi.me`、`adminKeys`（Task 4）；`tokenStore`、`useToken`（Task 4）；`SiteHeader`、`ThemeToggle`、`LangToggle`（Task 5）
- Produces: `LoginPage`（登录成功写入 token，回到 `state.from` 或 `/admin`）；`AdminLayout`（桌面侧栏 + 移动端抽屉，含退出登录）

- [ ] **Step 1: 写失败测试**

`web/src/features/auth/login-page.test.tsx`：

```tsx
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'

import { mockFetch } from '@/test/fetch'
import { renderWithProviders } from '@/test/render'

import { LoginPage } from './login-page'

describe('LoginPage', () => {
  it('失败时显示后端消息，成功后保存 token 并跳转后台', async () => {
    mockFetch({
      'GET /api/public/site': { site_title: '我的探针', show_price: false },
      'POST /api/auth/login': (body: unknown) =>
        (body as { password: string }).password === 'password123'
          ? { data: { token: 'tok', username: 'admin' } }
          : { status: 401, error: { code: 'invalid_credentials', message: '用户名或密码错误' } },
    })
    const user = userEvent.setup()
    renderWithProviders(<LoginPage />, { route: '/login', path: '/login' })

    await user.type(screen.getByLabelText('密码'), 'wrong')
    await user.click(screen.getByRole('button', { name: '登录' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('用户名或密码错误')

    await user.clear(screen.getByLabelText('密码'))
    await user.type(screen.getByLabelText('密码'), 'password123')
    await user.click(screen.getByRole('button', { name: '登录' }))
    await waitFor(() => expect(screen.getByTestId('location')).toHaveTextContent('/admin'))
    expect(localStorage.getItem('sss.token')).toBe('tok')
  })
})
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd web && pnpm test src/features/auth/login-page.test.tsx`
Expected: FAIL（占位页面没有表单）。

- [ ] **Step 3: 实现**

`web/src/features/auth/login-page.tsx`（整体替换）：

```tsx
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Navigate, useLocation, useNavigate } from 'react-router-dom'

import { SiteHeader } from '@/components/site-header'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { adminApi } from '@/lib/admin-api'
import { errorMessage, tokenStore } from '@/lib/api'
import { useToken } from '@/lib/auth'

export function LoginPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const location = useLocation()
  const token = useToken()
  const [username, setUsername] = useState('admin')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [pending, setPending] = useState(false)
  const from = (location.state as { from?: string } | null)?.from ?? '/admin'

  if (token) return <Navigate to={from} replace />

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setPending(true)
    setError('')
    try {
      const r = await adminApi.login(username, password)
      tokenStore.set(r.token)
      navigate(from, { replace: true })
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setPending(false)
    }
  }

  return (
    <>
      <SiteHeader />
      <main className="mx-auto flex max-w-sm flex-col px-4 py-16">
        <form onSubmit={(e) => void submit(e)} className="space-y-4 rounded-xl border bg-card p-6">
          <h1 className="text-lg font-semibold">{t('login.title')}</h1>
          <div className="space-y-1.5">
            <Label htmlFor="username">{t('login.username')}</Label>
            <Input id="username" autoComplete="username" value={username} onChange={(e) => setUsername(e.target.value)} required />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="password">{t('login.password')}</Label>
            <Input id="password" type="password" autoComplete="current-password" value={password} onChange={(e) => setPassword(e.target.value)} required />
          </div>
          {error && (
            <p role="alert" className="text-sm text-bad">
              {error}
            </p>
          )}
          <Button type="submit" className="w-full" disabled={pending}>
            {t('login.submit')}
          </Button>
        </form>
      </main>
    </>
  )
}
```

`web/src/features/admin/admin-layout.tsx`（整体替换）：

```tsx
import { useQuery } from '@tanstack/react-query'
import { ArrowLeft, LogOut, Menu, Server, Settings } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, NavLink, Outlet } from 'react-router-dom'

import { LangToggle, ThemeToggle } from '@/components/header-controls'
import { Button } from '@/components/ui/button'
import { Sheet, SheetContent, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { adminApi, adminKeys } from '@/lib/admin-api'
import { tokenStore } from '@/lib/api'
import { cn } from '@/lib/utils'

/** AdminLayout 后台布局：桌面左侧窄侧栏，移动端抽屉 */
export function AdminLayout() {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  // 进入后台即校验 token；失效时接口返回 401，登录态被清除，路由守卫自动跳转登录页
  const me = useQuery({ queryKey: adminKeys.me, queryFn: adminApi.me })

  const links = [
    { to: '/admin/servers', icon: Server, label: t('nav.servers') },
    { to: '/admin/settings', icon: Settings, label: t('nav.settings') },
  ]
  const item = 'flex w-full items-center gap-2 rounded-md px-3 py-2 text-sm'

  const sidebar = (
    <>
      <nav className="flex flex-col gap-1">
        {links.map((l) => (
          <NavLink
            key={l.to}
            to={l.to}
            onClick={() => setOpen(false)}
            className={({ isActive }) => cn(item, isActive ? 'bg-accent font-medium' : 'text-muted-foreground hover:bg-accent/60')}
          >
            <l.icon className="size-4" />
            {l.label}
          </NavLink>
        ))}
      </nav>
      <div className="mt-auto flex flex-col gap-1">
        <Link to="/" className={cn(item, 'text-muted-foreground hover:bg-accent/60')}>
          <ArrowLeft className="size-4" />
          {t('nav.status')}
        </Link>
        <button type="button" onClick={() => tokenStore.clear()} className={cn(item, 'text-muted-foreground hover:bg-accent/60')}>
          <LogOut className="size-4" />
          {t('nav.logout')}
          {me.data && <span className="ml-auto truncate text-xs">{me.data.username}</span>}
        </button>
      </div>
    </>
  )

  return (
    <div className="flex min-h-svh">
      <aside className="hidden w-56 shrink-0 flex-col gap-4 border-r bg-card p-3 md:flex">
        <div className="px-3 py-2 font-semibold tracking-tight">Simple Server Status</div>
        {sidebar}
      </aside>
      <div className="flex min-w-0 flex-1 flex-col">
        <header className="flex h-14 items-center gap-2 border-b px-4">
          <Button variant="ghost" size="icon" className="md:hidden" onClick={() => setOpen(true)} aria-label={t('nav.menu')}>
            <Menu className="size-4" />
          </Button>
          <div className="ml-auto flex items-center gap-1">
            <LangToggle />
            <ThemeToggle />
          </div>
        </header>
        <main className="min-w-0 flex-1 p-4 md:p-6">
          <Outlet />
        </main>
      </div>
      <Sheet open={open} onOpenChange={setOpen}>
        <SheetContent side="left" className="flex w-64 flex-col gap-4 p-3">
          <SheetHeader>
            <SheetTitle>Simple Server Status</SheetTitle>
          </SheetHeader>
          {sidebar}
        </SheetContent>
      </Sheet>
    </div>
  )
}
```

- [ ] **Step 4: 运行检查**

Run: `cd web && pnpm typecheck && pnpm lint && pnpm test`
Expected: 全部通过。

- [ ] **Step 5: 提交**

```bash
git add web/src
git commit -m "feat(web): 新增登录页与后台布局"
```

---

### Task 11: 后台服务器管理

**Files:**
- Create: `web/src/features/admin/form-model.ts`、`web/src/features/admin/server-form.tsx`、`web/src/features/admin/install-dialog.tsx`
- Modify: `web/src/features/admin/servers-page.tsx`（整体替换占位）
- Modify: `web/src/test/fixtures.ts`（追加 `makeAdminServer`）
- Test: `web/src/features/admin/form-model.test.ts`、`web/src/features/admin/servers-page.test.tsx`

**Interfaces:**
- Consumes: `adminApi`、`adminKeys`（Task 4）；`Flag`、`OnlineDot`（Task 7）；`useNow`（Task 6）
- Produces:
  - `formSchema`、`type FormValues`、`fromServer(s | null)`、`toInput(values)`（错误信息为 i18n 键）
  - `ServerFormDialog({ server, onClose, onSaved(saved, created) })`
  - `InstallDialog({ server, onClose })`（`dashboard` 参数取 `window.location.origin`）
  - `ServersPage`（具名导出）
  - 测试：`makeAdminServer(overrides)`

- [ ] **Step 1: 追加测试数据并写失败测试**

在 `web/src/test/fixtures.ts` 末尾追加（并在文件顶部的类型导入中加入 `AdminServer`）：

```ts
export function makeAdminServer(overrides: Partial<AdminServer> = {}): AdminServer {
  return {
    id: 'srv1',
    name: 'hk-01',
    secret: 'secret-1',
    group: 'HK',
    country: 'HK',
    sort: 0,
    hidden: false,
    price: null,
    currency: '',
    billing_cycle: '',
    expire_at: null,
    traffic_limit: null,
    traffic_mode: 'sum',
    traffic_reset_day: 1,
    report_interval: 2,
    nic_include: [],
    nic_exclude: [],
    mount_exclude: [],
    static_info: makeHello(),
    last_ip: '1.2.3.4',
    last_seen: 1_700_000_000,
    created_at: 1_700_000_000,
    updated_at: 1_700_000_000,
    online: true,
    ...overrides,
  }
}
```

`web/src/features/admin/form-model.test.ts`：

```ts
import { describe, expect, it } from 'vitest'

import { makeAdminServer } from '@/test/fixtures'

import { formSchema, fromServer, toInput } from './form-model'

describe('表单模型', () => {
  it('服务器 → 表单 → 提交数据往返一致', () => {
    const expire = Math.floor(new Date('2027-01-15T00:00:00').getTime() / 1000)
    const s = makeAdminServer({
      price: 9.9,
      currency: 'USD',
      billing_cycle: 'yearly',
      expire_at: expire,
      traffic_limit: 1024 ** 4,
      nic_include: ['eth0', 'ens'],
      report_interval: 5,
    })
    const v = fromServer(s)
    expect(v.traffic_limit_gb).toBe('1024')
    expect(v.expire_at).toBe('2027-01-15')
    expect(v.nic_include).toBe('eth0, ens')
    const input = toInput(v)
    expect(input).toMatchObject({ price: 9.9, billing_cycle: 'yearly', expire_at: expire, traffic_limit: 1024 ** 4, nic_include: ['eth0', 'ens'], report_interval: 5 })
  })

  it('新服务器使用默认值（间隔 0 表示由后端取默认）', () => {
    expect(toInput(fromServer(null))).toMatchObject({
      name: '',
      country: '',
      price: null,
      traffic_limit: null,
      billing_cycle: '',
      expire_at: null,
      traffic_reset_day: 1,
      report_interval: 0,
      nic_include: [],
    })
  })

  it('校验失败时返回 i18n 键', () => {
    const r = formSchema.safeParse({ ...fromServer(null), name: ' ', country: 'HKG', price: 'abc', traffic_reset_day: '29', report_interval: '61' })
    expect(r.success).toBe(false)
    const messages = r.success ? [] : r.error.issues.map((i) => i.message)
    expect(messages).toEqual(expect.arrayContaining(['form.nameRequired', 'form.countryInvalid', 'form.numberInvalid', 'form.resetDayRange', 'form.intervalRange']))
  })
})
```

`web/src/features/admin/servers-page.test.tsx`：

```tsx
import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'

import type { AdminServer } from '@/lib/types'
import { mockFetch } from '@/test/fetch'
import { makeAdminServer } from '@/test/fixtures'
import { renderWithProviders } from '@/test/render'

import { ServersPage } from './servers-page'

describe('ServersPage', () => {
  it('列出服务器，新建成功后弹出安装命令', async () => {
    let list: AdminServer[] = [makeAdminServer()]
    const created = makeAdminServer({ id: 'new1', name: 'n1', online: false })
    mockFetch({
      'GET /api/admin/servers': () => ({ data: list }),
      'POST /api/admin/servers': (body: unknown) => {
        list = [...list, { ...created, ...(body as object) }]
        return { data: created }
      },
      'GET /api/admin/servers/new1/install': { linux: 'curl -fsSL x | sudo bash -s -- --dashboard http://localhost:3000', windows: 'iwr x' },
    })
    const user = userEvent.setup()
    renderWithProviders(<ServersPage />, { route: '/admin/servers', path: '/admin/servers' })

    expect(await screen.findByText('hk-01')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '新建服务器' }))
    await user.type(screen.getByLabelText('名称'), 'n1')
    await user.click(screen.getByRole('button', { name: '保存' }))

    const dialog = await screen.findByRole('dialog', { name: '安装命令' })
    expect(await within(dialog).findByText(/--dashboard/)).toBeInTheDocument()
  })

  it('名称为空时显示校验错误且不提交', async () => {
    const fetchFn = mockFetch({ 'GET /api/admin/servers': [] })
    const user = userEvent.setup()
    renderWithProviders(<ServersPage />, { route: '/admin/servers', path: '/admin/servers' })
    expect(await screen.findByText('还没有服务器，点击右上角新建')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '新建服务器' }))
    await user.click(screen.getByRole('button', { name: '保存' }))
    expect(await screen.findByText('请输入名称')).toBeInTheDocument()
    expect(fetchFn.mock.calls.some(([, init]) => (init as RequestInit | undefined)?.method === 'POST')).toBe(false)
  })
})
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd web && pnpm test src/features/admin`
Expected: FAIL（无法解析 `./form-model`；ServersPage 为占位）。

- [ ] **Step 3: 实现表单模型**

`web/src/features/admin/form-model.ts`：

```ts
import { z } from 'zod'

import type { AdminServer, BillingCycle, ServerInput } from '@/lib/types'

const DECIMAL = /^(\d+(\.\d+)?)?$/
const GB = 1024 ** 3

/** formSchema 服务器表单校验；错误信息为 i18n 键，由界面翻译 */
export const formSchema = z.object({
  name: z.string().trim().min(1, 'form.nameRequired').max(64, 'form.nameTooLong'),
  group: z.string(),
  country: z.string().trim().regex(/^([A-Za-z]{2})?$/, 'form.countryInvalid'),
  hidden: z.boolean(),
  price: z.string().trim().regex(DECIMAL, 'form.numberInvalid'),
  currency: z.string(),
  billing_cycle: z.enum(['none', 'monthly', 'quarterly', 'yearly', 'once']),
  expire_at: z.string(),
  traffic_limit_gb: z.string().trim().regex(DECIMAL, 'form.numberInvalid'),
  traffic_mode: z.enum(['sum', 'in', 'out']),
  traffic_reset_day: z
    .string()
    .trim()
    .refine((v) => /^\d+$/.test(v) && Number(v) >= 1 && Number(v) <= 28, 'form.resetDayRange'),
  report_interval: z
    .string()
    .trim()
    .refine((v) => v === '' || (/^\d+$/.test(v) && Number(v) >= 1 && Number(v) <= 60), 'form.intervalRange'),
  nic_include: z.string(),
  nic_exclude: z.string(),
  mount_exclude: z.string(),
})

export type FormValues = z.infer<typeof formSchema>

function pad(n: number) {
  return String(n).padStart(2, '0')
}

/** toDateInput Unix 秒 → 本地日期 yyyy-mm-dd */
function toDateInput(ts: number | null): string {
  if (ts == null) return ''
  const d = new Date(ts * 1000)
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

function splitList(s: string): string[] {
  return s
    .split(',')
    .map((x) => x.trim())
    .filter(Boolean)
}

export function fromServer(s: AdminServer | null): FormValues {
  return {
    name: s?.name ?? '',
    group: s?.group ?? '',
    country: s?.country ?? '',
    hidden: s?.hidden ?? false,
    price: s?.price != null ? String(s.price) : '',
    currency: s?.currency ?? '',
    billing_cycle: s?.billing_cycle ? s.billing_cycle : 'none',
    expire_at: toDateInput(s?.expire_at ?? null),
    traffic_limit_gb: s?.traffic_limit != null ? String(Math.round((s.traffic_limit / GB) * 100) / 100) : '',
    traffic_mode: s?.traffic_mode ?? 'sum',
    traffic_reset_day: String(s?.traffic_reset_day ?? 1),
    report_interval: s ? String(s.report_interval) : '',
    nic_include: (s?.nic_include ?? []).join(', '),
    nic_exclude: (s?.nic_exclude ?? []).join(', '),
    mount_exclude: (s?.mount_exclude ?? []).join(', '),
  }
}

/** toInput 表单值 → 后端提交数据；到期日按本地零点换算，配额 GB → 字节，间隔为空时传 0 由后端取默认 */
export function toInput(v: FormValues): ServerInput {
  return {
    name: v.name.trim(),
    group: v.group.trim(),
    country: v.country.trim().toUpperCase(),
    hidden: v.hidden,
    price: v.price ? Number(v.price) : null,
    currency: v.currency.trim(),
    billing_cycle: v.billing_cycle === 'none' ? '' : (v.billing_cycle as BillingCycle),
    expire_at: v.expire_at ? Math.floor(new Date(`${v.expire_at}T00:00:00`).getTime() / 1000) : null,
    traffic_limit: v.traffic_limit_gb ? Math.round(Number(v.traffic_limit_gb) * GB) : null,
    traffic_mode: v.traffic_mode,
    traffic_reset_day: Number(v.traffic_reset_day),
    report_interval: v.report_interval ? Number(v.report_interval) : 0,
    nic_include: splitList(v.nic_include),
    nic_exclude: splitList(v.nic_exclude),
    mount_exclude: splitList(v.mount_exclude),
  }
}
```

- [ ] **Step 4: 实现表单弹窗、安装命令弹窗与服务器页**

`web/src/features/admin/server-form.tsx`：

```tsx
import { zodResolver } from '@hookform/resolvers/zod'
import { useQueryClient } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { Controller, useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { adminApi, adminKeys } from '@/lib/admin-api'
import { errorMessage } from '@/lib/api'
import type { AdminServer } from '@/lib/types'

import { formSchema, fromServer, toInput, type FormValues } from './form-model'

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <fieldset className="space-y-3">
      <legend className="text-sm font-medium">{title}</legend>
      <div className="grid gap-3 sm:grid-cols-2">{children}</div>
    </fieldset>
  )
}

function Field({ id, label, error, hint, children }: { id: string; label: string; error?: string; hint?: string; children: ReactNode }) {
  const { t } = useTranslation()
  return (
    <div className="space-y-1.5">
      <Label htmlFor={id}>{label}</Label>
      {children}
      {error ? <p className="text-xs text-bad">{t(error)}</p> : hint ? <p className="text-xs text-muted-foreground">{hint}</p> : null}
    </div>
  )
}

interface Props {
  server: AdminServer | null
  onClose: () => void
  onSaved: (saved: AdminServer, created: boolean) => void
}

/** ServerFormDialog 新建 / 编辑服务器，字段分为基本、计费、流量、采集四组 */
export function ServerFormDialog({ server, onClose, onSaved }: Props) {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const {
    register,
    control,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<FormValues>({ resolver: zodResolver(formSchema), defaultValues: fromServer(server) })

  const submit = handleSubmit(async (values) => {
    try {
      const input = toInput(values)
      const saved = server ? await adminApi.update(server.id, input) : await adminApi.create(input)
      await qc.invalidateQueries({ queryKey: adminKeys.servers })
      toast.success(t('admin.saved'))
      onSaved(saved, server === null)
    } catch (err) {
      toast.error(errorMessage(err))
    }
  })

  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-h-[90svh] overflow-y-auto sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{server ? t('form.editTitle') : t('form.createTitle')}</DialogTitle>
        </DialogHeader>
        <form onSubmit={(e) => void submit(e)} className="space-y-6" noValidate>
          <Section title={t('form.basic')}>
            <Field id="name" label={t('form.name')} error={errors.name?.message}>
              <Input id="name" {...register('name')} />
            </Field>
            <Field id="group" label={t('form.group')}>
              <Input id="group" {...register('group')} />
            </Field>
            <Field id="country" label={t('form.country')} error={errors.country?.message} hint={t('form.countryHint')}>
              <Input id="country" maxLength={2} {...register('country')} />
            </Field>
            <div className="flex items-center gap-2 sm:self-center">
              <Controller control={control} name="hidden" render={({ field }) => <Switch id="hidden" checked={field.value} onCheckedChange={field.onChange} />} />
              <Label htmlFor="hidden">{t('form.hidden')}</Label>
            </div>
          </Section>

          <Section title={t('form.billing')}>
            <Field id="price" label={t('form.price')} error={errors.price?.message}>
              <Input id="price" inputMode="decimal" {...register('price')} />
            </Field>
            <Field id="currency" label={t('form.currency')}>
              <Input id="currency" placeholder="$ / ¥ / USD" {...register('currency')} />
            </Field>
            <Field id="billing_cycle" label={t('form.cycle')}>
              <Controller
                control={control}
                name="billing_cycle"
                render={({ field }) => (
                  <Select value={field.value} onValueChange={field.onChange}>
                    <SelectTrigger id="billing_cycle" className="w-full">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="none">{t('form.cycleNone')}</SelectItem>
                      {(['monthly', 'quarterly', 'yearly', 'once'] as const).map((c) => (
                        <SelectItem key={c} value={c}>
                          {t(`billing.${c}`)}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                )}
              />
            </Field>
            <Field id="expire_at" label={t('form.expireAt')}>
              <Input id="expire_at" type="date" {...register('expire_at')} />
            </Field>
          </Section>

          <Section title={t('form.traffic')}>
            <Field id="traffic_limit_gb" label={t('form.trafficLimit')} error={errors.traffic_limit_gb?.message} hint={t('form.trafficLimitHint')}>
              <Input id="traffic_limit_gb" inputMode="decimal" {...register('traffic_limit_gb')} />
            </Field>
            <Field id="traffic_mode" label={t('form.trafficMode')}>
              <Controller
                control={control}
                name="traffic_mode"
                render={({ field }) => (
                  <Select value={field.value} onValueChange={field.onChange}>
                    <SelectTrigger id="traffic_mode" className="w-full">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="sum">{t('form.modeSum')}</SelectItem>
                      <SelectItem value="in">{t('form.modeIn')}</SelectItem>
                      <SelectItem value="out">{t('form.modeOut')}</SelectItem>
                    </SelectContent>
                  </Select>
                )}
              />
            </Field>
            <Field id="traffic_reset_day" label={t('form.resetDay')} error={errors.traffic_reset_day?.message}>
              <Input id="traffic_reset_day" inputMode="numeric" {...register('traffic_reset_day')} />
            </Field>
          </Section>

          <Section title={t('form.collect')}>
            <Field id="report_interval" label={t('form.interval')} error={errors.report_interval?.message} hint={t('form.intervalHint')}>
              <Input id="report_interval" inputMode="numeric" {...register('report_interval')} />
            </Field>
            <Field id="nic_include" label={t('form.nicInclude')} hint={t('form.listHint')}>
              <Input id="nic_include" placeholder="eth0, ens" {...register('nic_include')} />
            </Field>
            <Field id="nic_exclude" label={t('form.nicExclude')} hint={t('form.listHint')}>
              <Input id="nic_exclude" {...register('nic_exclude')} />
            </Field>
            <Field id="mount_exclude" label={t('form.mountExclude')} hint={t('form.listHint')}>
              <Input id="mount_exclude" placeholder="/boot" {...register('mount_exclude')} />
            </Field>
          </Section>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              {t('form.cancel')}
            </Button>
            <Button type="submit" disabled={isSubmitting}>
              {t('form.save')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
```

`web/src/features/admin/install-dialog.tsx`：

```tsx
import { useQuery } from '@tanstack/react-query'
import { Copy } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { adminApi } from '@/lib/admin-api'
import { errorMessage } from '@/lib/api'
import type { AdminServer } from '@/lib/types'

function CopyBlock({ text }: { text: string }) {
  const { t } = useTranslation()
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(text)
      toast.success(t('install.copied'))
    } catch {
      toast.error(t('install.copyFailed'))
    }
  }
  return (
    <div className="relative">
      <pre className="max-h-56 overflow-auto rounded-lg bg-muted p-3 pr-12 font-mono text-xs break-all whitespace-pre-wrap">{text}</pre>
      <Button size="icon" variant="ghost" className="absolute top-1.5 right-1.5 size-7" onClick={() => void copy()} aria-label={t('install.copy')}>
        <Copy className="size-3.5" />
      </Button>
    </div>
  )
}

/** InstallDialog 显示一键安装命令（每次打开重新获取，重置密钥后即为新命令） */
export function InstallDialog({ server, onClose }: { server: AdminServer | null; onClose: () => void }) {
  const { t } = useTranslation()
  const q = useQuery({
    queryKey: ['admin', 'install', server?.id],
    queryFn: () => (server ? adminApi.install(server.id, window.location.origin) : Promise.resolve(null)),
    enabled: server !== null,
    gcTime: 0,
    staleTime: 0,
  })

  return (
    <Dialog open={server !== null} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{t('install.title')}</DialogTitle>
          <DialogDescription>{t('install.desc')}</DialogDescription>
        </DialogHeader>
        {q.error ? (
          <p className="text-sm text-bad">{errorMessage(q.error)}</p>
        ) : !q.data ? (
          <Skeleton className="h-24" />
        ) : (
          <Tabs defaultValue="linux">
            <TabsList>
              <TabsTrigger value="linux">Linux</TabsTrigger>
              <TabsTrigger value="windows">Windows</TabsTrigger>
            </TabsList>
            <TabsContent value="linux">
              <CopyBlock text={q.data.linux} />
            </TabsContent>
            <TabsContent value="windows">
              <CopyBlock text={q.data.windows} />
            </TabsContent>
          </Tabs>
        )}
      </DialogContent>
    </Dialog>
  )
}
```

`web/src/features/admin/servers-page.tsx`（整体替换）：

```tsx
import { closestCenter, DndContext, KeyboardSensor, PointerSensor, useSensor, useSensors, type DragEndEvent } from '@dnd-kit/core'
import { arrayMove, SortableContext, sortableKeyboardCoordinates, useSortable, verticalListSortingStrategy } from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { EyeOff, GripVertical, MoreHorizontal, Plus } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Flag } from '@/components/flag'
import { OnlineDot } from '@/components/status-bits'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { useLang } from '@/i18n/use-lang'
import { adminApi, adminKeys } from '@/lib/admin-api'
import { errorMessage } from '@/lib/api'
import type { Lang } from '@/lib/format'
import { formatDate } from '@/lib/format'
import type { AdminServer } from '@/lib/types'
import { cn } from '@/lib/utils'

import { InstallDialog } from './install-dialog'
import { ServerFormDialog } from './server-form'

interface RowProps {
  server: AdminServer
  lang: Lang
  onEdit: () => void
  onInstall: () => void
  onReset: () => void
  onDelete: () => void
}

function SortableRow({ server: s, lang, onEdit, onInstall, onReset, onDelete }: RowProps) {
  const { t } = useTranslation()
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id: s.id })
  return (
    <TableRow ref={setNodeRef} style={{ transform: CSS.Transform.toString(transform), transition }} className={cn(isDragging && 'relative z-10 bg-accent')}>
      <TableCell className="w-8">
        <button type="button" className="cursor-grab touch-none text-muted-foreground" aria-label={t('admin.drag')} {...attributes} {...listeners}>
          <GripVertical className="size-4" />
        </button>
      </TableCell>
      <TableCell>
        <div className="flex min-w-0 items-center gap-2">
          <OnlineDot online={s.online} />
          <span className="truncate font-medium">{s.name}</span>
          <Flag code={s.country} />
          {s.hidden && <EyeOff className="size-3.5 text-muted-foreground" />}
        </div>
      </TableCell>
      <TableCell className="hidden sm:table-cell">{s.group || '—'}</TableCell>
      <TableCell className="hidden font-mono text-xs md:table-cell">{s.last_ip || '—'}</TableCell>
      <TableCell className="hidden text-xs md:table-cell">{s.expire_at ? formatDate(s.expire_at, lang) : t('expire.never')}</TableCell>
      <TableCell className="hidden text-xs tabular lg:table-cell">{s.report_interval}s</TableCell>
      <TableCell className="w-10 text-right">
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon" className="size-8" aria-label={t('admin.actions')}>
              <MoreHorizontal className="size-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem onSelect={onEdit}>{t('admin.edit')}</DropdownMenuItem>
            <DropdownMenuItem onSelect={onInstall}>{t('admin.install')}</DropdownMenuItem>
            <DropdownMenuItem onSelect={onReset}>{t('admin.resetSecret')}</DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem onSelect={onDelete} className="text-bad focus:text-bad">
              {t('admin.delete')}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </TableCell>
    </TableRow>
  )
}

type Confirm = { kind: 'delete' | 'reset'; server: AdminServer }

export function ServersPage() {
  const { t } = useTranslation()
  const lang = useLang()
  const qc = useQueryClient()
  const { data, isLoading, error } = useQuery({ queryKey: adminKeys.servers, queryFn: adminApi.servers, refetchInterval: 10_000 })
  const [editing, setEditing] = useState<AdminServer | 'new' | null>(null)
  const [installFor, setInstallFor] = useState<AdminServer | null>(null)
  const [confirm, setConfirm] = useState<Confirm | null>(null)
  const [order, setOrder] = useState<string[] | null>(null)

  // 拖拽后先按本地顺序显示，保存完成并刷新列表后再使用服务端顺序
  const servers = useMemo(() => {
    const list = data ?? []
    if (!order) return list
    const byId = new Map(list.map((s) => [s.id, s]))
    return order.map((id) => byId.get(id)).filter((s): s is AdminServer => s !== undefined)
  }, [data, order])

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 4 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  )

  const onDragEnd = ({ active, over }: DragEndEvent) => {
    if (!over || active.id === over.id) return
    const ids = servers.map((s) => s.id)
    const next = arrayMove(ids, ids.indexOf(String(active.id)), ids.indexOf(String(over.id)))
    setOrder(next)
    adminApi
      .order(next)
      .then(() => qc.invalidateQueries({ queryKey: adminKeys.servers }))
      .catch((err: unknown) => toast.error(errorMessage(err)))
      .finally(() => setOrder(null))
  }

  const runConfirm = async () => {
    if (!confirm) return
    const { kind, server } = confirm
    setConfirm(null)
    try {
      if (kind === 'delete') {
        await adminApi.remove(server.id)
        toast.success(t('admin.deleted'))
      } else {
        await adminApi.resetSecret(server.id)
        toast.success(t('admin.secretReset'))
        setInstallFor(server)
      }
      await qc.invalidateQueries({ queryKey: adminKeys.servers })
    } catch (err) {
      toast.error(errorMessage(err))
    }
  }

  let body
  if (isLoading) body = <Skeleton className="h-40 rounded-xl" />
  else if (error) body = <p className="text-sm text-bad">{errorMessage(error)}</p>
  else if (servers.length === 0) body = <p className="rounded-xl border bg-card py-16 text-center text-sm text-muted-foreground">{t('admin.empty')}</p>
  else
    body = (
      <div className="overflow-x-auto rounded-xl border bg-card">
        <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={onDragEnd}>
          <SortableContext items={servers.map((s) => s.id)} strategy={verticalListSortingStrategy}>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-8" />
                  <TableHead>{t('admin.name')}</TableHead>
                  <TableHead className="hidden sm:table-cell">{t('admin.group')}</TableHead>
                  <TableHead className="hidden md:table-cell">{t('admin.ip')}</TableHead>
                  <TableHead className="hidden md:table-cell">{t('admin.expire')}</TableHead>
                  <TableHead className="hidden lg:table-cell">{t('admin.interval')}</TableHead>
                  <TableHead className="w-10">
                    <span className="sr-only">{t('admin.actions')}</span>
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {servers.map((s) => (
                  <SortableRow
                    key={s.id}
                    server={s}
                    lang={lang}
                    onEdit={() => setEditing(s)}
                    onInstall={() => setInstallFor(s)}
                    onReset={() => setConfirm({ kind: 'reset', server: s })}
                    onDelete={() => setConfirm({ kind: 'delete', server: s })}
                  />
                ))}
              </TableBody>
            </Table>
          </SortableContext>
        </DndContext>
      </div>
    )

  return (
    <div className="mx-auto max-w-6xl space-y-4">
      <div className="flex items-center justify-between gap-2">
        <h1 className="text-lg font-semibold">{t('admin.title')}</h1>
        <Button onClick={() => setEditing('new')}>
          <Plus className="size-4" />
          {t('admin.add')}
        </Button>
      </div>
      {body}

      {editing !== null && (
        <ServerFormDialog
          key={editing === 'new' ? 'new' : editing.id}
          server={editing === 'new' ? null : editing}
          onClose={() => setEditing(null)}
          onSaved={(saved, created) => {
            setEditing(null)
            if (created) setInstallFor(saved)
          }}
        />
      )}
      <InstallDialog server={installFor} onClose={() => setInstallFor(null)} />
      <AlertDialog open={confirm !== null} onOpenChange={(o) => !o && setConfirm(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{confirm && t(confirm.kind === 'delete' ? 'admin.deleteTitle' : 'admin.resetTitle', { name: confirm.server.name })}</AlertDialogTitle>
            <AlertDialogDescription>{confirm && t(confirm.kind === 'delete' ? 'admin.deleteDesc' : 'admin.resetDesc')}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
            <AlertDialogAction onClick={() => void runConfirm()} className={confirm?.kind === 'delete' ? 'bg-destructive text-white hover:bg-destructive/90' : undefined}>
              {t('common.confirm')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
```

- [ ] **Step 5: 运行检查**

Run: `cd web && pnpm typecheck && pnpm lint && pnpm test`
Expected: 全部通过。若 shadcn 生成的 `TableRow` 不接受 `ref`（React 19 下函数组件把 `ref` 作为普通 prop 透传，通常可用），把 `SortableRow` 的最外层改为原生 `<tr>` 并复用 `TableRow` 的 className，在报告中说明。

- [ ] **Step 6: 提交**

```bash
git add web/src
git commit -m "feat(web): 新增后台服务器管理（拖拽排序、表单、安装命令）"
```

---

### Task 12: 后台设置页

**Files:**
- Modify: `web/src/features/admin/settings-page.tsx`（整体替换占位）
- Create: `web/src/lib/download.ts`
- Test: `web/src/features/admin/settings-page.test.tsx`

**Interfaces:**
- Consumes: `adminApi`、`adminKeys`（Task 4）；`tokenStore`（Task 4）
- Produces: `SettingsPage`（站点设置、修改密码、导出 / 导入）；`downloadJson(name, data)`

- [ ] **Step 1: 写失败测试**

`web/src/features/admin/settings-page.test.tsx`：

```tsx
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'

import type { Settings } from '@/lib/types'
import { mockFetch } from '@/test/fetch'
import { renderWithProviders } from '@/test/render'

import { SettingsPage } from './settings-page'

const SETTINGS: Settings = { site_title: 'Simple Server Status', show_price: false, default_report_interval: 2, install_script_base: 'https://example.com/dl' }

describe('SettingsPage', () => {
  it('修改站点标题并保存', async () => {
    let saved: unknown = null
    mockFetch({
      'GET /api/admin/settings': SETTINGS,
      'PUT /api/admin/settings': (body: unknown) => {
        saved = body
        return { data: body }
      },
    })
    const user = userEvent.setup()
    renderWithProviders(<SettingsPage />, { route: '/admin/settings', path: '/admin/settings' })
    const input = await screen.findByLabelText('站点标题')
    await user.clear(input)
    await user.type(input, '新标题')
    await user.click(screen.getByRole('button', { name: '保存' }))
    await waitFor(() => expect(saved).toMatchObject({ site_title: '新标题', default_report_interval: 2 }))
  })

  it('两次新密码不一致时提示且不发送请求', async () => {
    const fetchFn = mockFetch({ 'GET /api/admin/settings': SETTINGS })
    const user = userEvent.setup()
    renderWithProviders(<SettingsPage />, { route: '/admin/settings', path: '/admin/settings' })
    await user.type(screen.getByLabelText('原密码'), 'password123')
    await user.type(screen.getByLabelText('新密码'), 'newpass123')
    await user.type(screen.getByLabelText('确认新密码'), 'different1')
    await user.click(screen.getByRole('button', { name: '修改密码' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('两次输入的新密码不一致')
    expect(fetchFn.mock.calls.some(([url]) => String(url) === '/api/auth/password')).toBe(false)
  })

  it('修改密码成功后更新本地 token', async () => {
    mockFetch({ 'GET /api/admin/settings': SETTINGS, 'PUT /api/auth/password': { token: 'new-token' } })
    const user = userEvent.setup()
    renderWithProviders(<SettingsPage />, { route: '/admin/settings', path: '/admin/settings' })
    await user.type(screen.getByLabelText('原密码'), 'password123')
    await user.type(screen.getByLabelText('新密码'), 'newpass123')
    await user.type(screen.getByLabelText('确认新密码'), 'newpass123')
    await user.click(screen.getByRole('button', { name: '修改密码' }))
    await waitFor(() => expect(localStorage.getItem('sss.token')).toBe('new-token'))
  })
})
```

注意：`getByLabelText('新密码')` 按完整文本精确匹配，不会命中“确认新密码”。

- [ ] **Step 2: 运行测试确认失败**

Run: `cd web && pnpm test src/features/admin/settings-page.test.tsx`
Expected: FAIL（占位页面）。

- [ ] **Step 3: 实现**

`web/src/lib/download.ts`：

```ts
/** downloadJson 在浏览器中把数据保存为 JSON 文件 */
export function downloadJson(name: string, data: unknown): void {
  const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = name
  a.click()
  URL.revokeObjectURL(url)
}
```

`web/src/features/admin/settings-page.tsx`（整体替换）：

```tsx
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useRef, useState, type ChangeEvent, type FormEvent, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import { adminApi, adminKeys } from '@/lib/admin-api'
import { errorMessage, tokenStore } from '@/lib/api'
import { downloadJson } from '@/lib/download'
import type { Settings } from '@/lib/types'

function Card({ title, desc, children }: { title: string; desc?: string; children: ReactNode }) {
  return (
    <section className="space-y-4 rounded-xl border bg-card p-5">
      <div>
        <h2 className="text-sm font-medium">{title}</h2>
        {desc && <p className="mt-1 text-xs text-muted-foreground">{desc}</p>}
      </div>
      {children}
    </section>
  )
}

function SiteSettingsForm({ initial }: { initial: Settings }) {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const [s, setS] = useState(initial)
  const [pending, setPending] = useState(false)

  const save = async (e: FormEvent) => {
    e.preventDefault()
    setPending(true)
    try {
      await adminApi.saveSettings({ ...s, default_report_interval: Number(s.default_report_interval) })
      await Promise.all([qc.invalidateQueries({ queryKey: adminKeys.settings }), qc.invalidateQueries({ queryKey: ['site'] })])
      toast.success(t('settings.saved'))
    } catch (err) {
      toast.error(errorMessage(err))
    } finally {
      setPending(false)
    }
  }

  return (
    <form onSubmit={(e) => void save(e)} className="space-y-4">
      <div className="space-y-1.5">
        <Label htmlFor="site_title">{t('settings.siteTitle')}</Label>
        <Input id="site_title" value={s.site_title} onChange={(e) => setS({ ...s, site_title: e.target.value })} />
      </div>
      <div className="flex items-center gap-2">
        <Switch id="show_price" checked={s.show_price} onCheckedChange={(v) => setS({ ...s, show_price: v })} />
        <Label htmlFor="show_price">{t('settings.showPrice')}</Label>
      </div>
      <div className="space-y-1.5">
        <Label htmlFor="default_report_interval">{t('settings.defaultInterval')}</Label>
        <Input
          id="default_report_interval"
          type="number"
          min={1}
          max={60}
          value={s.default_report_interval}
          onChange={(e) => setS({ ...s, default_report_interval: Number(e.target.value) })}
        />
      </div>
      <div className="space-y-1.5">
        <Label htmlFor="install_script_base">{t('settings.scriptBase')}</Label>
        <Input id="install_script_base" value={s.install_script_base} onChange={(e) => setS({ ...s, install_script_base: e.target.value })} />
      </div>
      <Button type="submit" disabled={pending}>
        {t('settings.save')}
      </Button>
    </form>
  )
}

function PasswordForm() {
  const { t } = useTranslation()
  const [oldPw, setOldPw] = useState('')
  const [newPw, setNewPw] = useState('')
  const [confirmPw, setConfirmPw] = useState('')
  const [error, setError] = useState('')
  const [pending, setPending] = useState(false)

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    if (newPw.length < 8) return setError(t('settings.passwordTooShort'))
    if (newPw !== confirmPw) return setError(t('settings.passwordMismatch'))
    setError('')
    setPending(true)
    try {
      const r = await adminApi.changePassword(oldPw, newPw)
      tokenStore.set(r.token)
      setOldPw('')
      setNewPw('')
      setConfirmPw('')
      toast.success(t('settings.passwordChanged'))
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setPending(false)
    }
  }

  return (
    <form onSubmit={(e) => void submit(e)} className="space-y-4">
      <div className="space-y-1.5">
        <Label htmlFor="old_password">{t('settings.oldPassword')}</Label>
        <Input id="old_password" type="password" autoComplete="current-password" value={oldPw} onChange={(e) => setOldPw(e.target.value)} />
      </div>
      <div className="space-y-1.5">
        <Label htmlFor="new_password">{t('settings.newPassword')}</Label>
        <Input id="new_password" type="password" autoComplete="new-password" value={newPw} onChange={(e) => setNewPw(e.target.value)} />
      </div>
      <div className="space-y-1.5">
        <Label htmlFor="confirm_password">{t('settings.confirmPassword')}</Label>
        <Input id="confirm_password" type="password" autoComplete="new-password" value={confirmPw} onChange={(e) => setConfirmPw(e.target.value)} />
      </div>
      {error && (
        <p role="alert" className="text-sm text-bad">
          {error}
        </p>
      )}
      <Button type="submit" disabled={pending}>
        {t('settings.password')}
      </Button>
    </form>
  )
}

function BackupPanel() {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const fileRef = useRef<HTMLInputElement>(null)
  const [confirmOpen, setConfirmOpen] = useState(false)

  const doExport = async () => {
    try {
      const data = await adminApi.exportData()
      downloadJson(`sss-export-${new Date().toISOString().slice(0, 10)}.json`, data)
    } catch (err) {
      toast.error(errorMessage(err))
    }
  }

  const onFile = async (e: ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    e.target.value = ''
    if (!file) return
    let parsed: unknown
    try {
      parsed = JSON.parse(await file.text())
    } catch {
      toast.error(t('settings.importInvalid'))
      return
    }
    try {
      const r = await adminApi.importData(parsed)
      await qc.invalidateQueries()
      toast.success(t('settings.importDone', { count: r.servers }))
    } catch (err) {
      toast.error(errorMessage(err))
    }
  }

  return (
    <div className="flex flex-wrap gap-2">
      <Button variant="outline" onClick={() => setConfirmOpen(true)}>
        {t('settings.export')}
      </Button>
      <Button variant="outline" onClick={() => fileRef.current?.click()}>
        {t('settings.import')}
      </Button>
      <input ref={fileRef} type="file" accept="application/json,.json" className="hidden" onChange={(e) => void onFile(e)} />
      <AlertDialog open={confirmOpen} onOpenChange={setConfirmOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('settings.exportTitle')}</AlertDialogTitle>
            <AlertDialogDescription>{t('settings.exportDesc')}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
            <AlertDialogAction onClick={() => void doExport()}>{t('settings.export')}</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}

export function SettingsPage() {
  const { t } = useTranslation()
  const q = useQuery({ queryKey: adminKeys.settings, queryFn: adminApi.settings })
  return (
    <div className="mx-auto max-w-2xl space-y-6">
      <h1 className="text-lg font-semibold">{t('settings.title')}</h1>
      <Card title={t('settings.site')}>
        {q.data ? (
          <SiteSettingsForm key={JSON.stringify(q.data)} initial={q.data} />
        ) : q.error ? (
          <p className="text-sm text-bad">{errorMessage(q.error)}</p>
        ) : (
          <Skeleton className="h-48" />
        )}
      </Card>
      <Card title={t('settings.password')}>
        <PasswordForm />
      </Card>
      <Card title={t('settings.backup')} desc={t('settings.backupDesc')}>
        <BackupPanel />
      </Card>
    </div>
  )
}
```

注意：卡片标题“修改密码”是 `h2`，提交按钮同名但角色为 button，测试用 `getByRole('button', { name: '修改密码' })` 不冲突。

- [ ] **Step 4: 运行检查**

Run: `cd web && pnpm typecheck && pnpm lint && pnpm test`
Expected: 全部通过。

- [ ] **Step 5: 提交**

```bash
git add web/src
git commit -m "feat(web): 新增后台设置页（站点、密码、导入导出）"
```

---

### Task 13: 端到端测试与收尾

**Files:**
- Create: `web/playwright.config.ts`、`web/e2e/global-setup.ts`、`web/e2e/status.spec.ts`、`web/e2e/admin.spec.ts`、`web/e2e/mobile.spec.ts`

**Interfaces:**
- Consumes: 全部前端；Go 端 `./cmd/sss-dashboard`、`./cmd/sss-agent`（计划 1）
- Produces: `pnpm test:e2e`（先构建前端，再由全局初始化编译并启动 Dashboard 与真实 Agent，测试结束后停止并清理临时目录）

- [ ] **Step 1: 写 Playwright 配置与全局初始化**

`web/playwright.config.ts`：

```ts
import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: './e2e',
  timeout: 60_000,
  workers: 1,
  retries: 0,
  globalSetup: './e2e/global-setup.ts',
  use: { baseURL: 'http://127.0.0.1:18990', locale: 'zh-CN', trace: 'retain-on-failure' },
  projects: [{ name: 'chromium', use: { browserName: 'chromium' } }],
})
```

`web/e2e/global-setup.ts`：

```ts
import { execFileSync, spawn, type ChildProcess } from 'node:child_process'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const REPO = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..')
const BASE = 'http://127.0.0.1:18990'
const EXE = process.platform === 'win32' ? '.exe' : ''

async function waitFor(check: () => Promise<boolean>, timeoutMs: number, what: string) {
  const end = Date.now() + timeoutMs
  while (Date.now() < end) {
    try {
      if (await check()) return
    } catch {
      // 服务尚未就绪，继续等待
    }
    await new Promise((r) => setTimeout(r, 500))
  }
  throw new Error(`等待超时：${what}`)
}

async function call<T>(method: string, p: string, token?: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (token) headers.Authorization = `Bearer ${token}`
  const res = await fetch(BASE + p, { method, headers, body: body === undefined ? undefined : JSON.stringify(body) })
  if (!res.ok) throw new Error(`${method} ${p} 返回 ${res.status}`)
  return ((await res.json()) as { data: T }).data
}

/** 编译并启动 Dashboard（嵌入刚构建的前端）与一个真实 Agent，返回清理函数 */
export default async function globalSetup() {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'sss-e2e-'))
  const dashboard = path.join(tmp, `sss-dashboard${EXE}`)
  const agent = path.join(tmp, `sss-agent${EXE}`)
  execFileSync('go', ['build', '-o', dashboard, './cmd/sss-dashboard'], { cwd: REPO, stdio: 'inherit' })
  execFileSync('go', ['build', '-o', agent, './cmd/sss-agent'], { cwd: REPO, stdio: 'inherit' })

  const procs: ChildProcess[] = []
  procs.push(spawn(dashboard, ['--listen', '127.0.0.1:18990', '--data-dir', path.join(tmp, 'data'), '--admin-password', 'password123'], { cwd: tmp, stdio: 'ignore' }))
  await waitFor(async () => (await fetch(`${BASE}/api/public/site`)).ok, 30_000, 'Dashboard 启动')

  const { token } = await call<{ token: string }>('POST', '/api/auth/login', undefined, { username: 'admin', password: 'password123' })
  const srv = await call<{ id: string; secret: string }>('POST', '/api/admin/servers', token, { name: 'e2e-node', group: 'E2E' })
  procs.push(spawn(agent, ['--dashboard', BASE, '--id', srv.id, '--secret', srv.secret, '--detect-country=false'], { cwd: tmp, stdio: 'ignore' }))
  await waitFor(async () => (await call<{ online: boolean }[]>('GET', '/api/public/servers')).some((s) => s.online), 30_000, 'Agent 上线')

  return async () => {
    for (const p of procs) p.kill()
    await new Promise((r) => setTimeout(r, 1000))
    fs.rmSync(tmp, { recursive: true, force: true, maxRetries: 5, retryDelay: 500 })
  }
}
```

- [ ] **Step 2: 写 e2e 用例**

`web/e2e/status.spec.ts`：

```ts
import { expect, test } from '@playwright/test'

test('首页显示在线服务器，可进入详情页并切换时间范围', async ({ page }) => {
  await page.goto('/')
  const card = page.getByRole('link', { name: /e2e-node/ })
  await expect(card).toBeVisible()
  await expect(card.getByText(/^在线/)).toBeVisible()
  await card.click()
  await expect(page).toHaveURL(/\/server\//)
  await expect(page.getByRole('heading', { name: 'e2e-node' })).toBeVisible()
  await expect(page.getByText('历史趋势')).toBeVisible()
  await page.getByRole('tab', { name: '1 小时' }).click()
  await expect(page.getByRole('tab', { name: '1 小时' })).toHaveAttribute('data-state', 'active')
})
```

`web/e2e/admin.spec.ts`：

```ts
import { expect, test } from '@playwright/test'

test('登录后新建服务器并显示安装命令', async ({ page }) => {
  await page.goto('/login')
  await page.getByLabel('密码', { exact: true }).fill('password123')
  await page.getByRole('button', { name: '登录' }).click()
  await expect(page).toHaveURL(/\/admin\/servers/)
  await expect(page.getByText('e2e-node')).toBeVisible()

  await page.getByRole('button', { name: '新建服务器' }).click()
  await page.getByLabel('名称', { exact: true }).fill('e2e-new')
  await page.getByRole('button', { name: '保存' }).click()

  const dialog = page.getByRole('dialog', { name: '安装命令' })
  await expect(dialog).toBeVisible()
  await expect(dialog.getByText(/--dashboard/)).toBeVisible()
})
```

`web/e2e/mobile.spec.ts`：

```ts
import { expect, test, type Page } from '@playwright/test'

test.use({ viewport: { width: 390, height: 844 } })

async function horizontalOverflow(page: Page): Promise<number> {
  return page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)
}

test('390px 宽度下首页与后台无横向滚动', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByRole('link', { name: /e2e-node/ })).toBeVisible()
  expect(await horizontalOverflow(page)).toBeLessThanOrEqual(0)

  await page.goto('/login')
  await page.getByLabel('密码', { exact: true }).fill('password123')
  await page.getByRole('button', { name: '登录' }).click()
  await expect(page).toHaveURL(/\/admin\/servers/)
  await expect(page.getByText('e2e-node')).toBeVisible()
  expect(await horizontalOverflow(page)).toBeLessThanOrEqual(0)
})
```

- [ ] **Step 3: 运行端到端测试**

```bash
cd /d/Code/m/simple-server-status/web
pnpm exec playwright install chromium
pnpm test:e2e
```

Expected: 3 个用例全部 PASS。若失败，按报错定位到对应任务的组件修复（不要放宽断言），修复后重跑并在报告中说明。

- [ ] **Step 4: 全量检查**

```bash
cd /d/Code/m/simple-server-status/web
pnpm typecheck
pnpm lint
pnpm test
pnpm build
ls ../internal/dashboard/web/dist
cd ..
git status --short internal/dashboard/web/dist
go vet ./...
go test ./... -count=1
go build -o bin/sss-dashboard$(go env GOEXE) ./cmd/sss-dashboard
```

Expected:
- 前端 typecheck、lint（无错误）、单元测试、构建全部通过；
- `dist` 下有 `index.html`、`assets/`、`.gitkeep`，`git status` 对该目录无输出；
- Go vet / test 通过，嵌入新前端的 `sss-dashboard` 构建成功。

- [ ] **Step 5: 提交**

```bash
git add web/playwright.config.ts web/e2e
git commit -m "test(web): 新增首页、后台与窄屏的端到端测试"
```
