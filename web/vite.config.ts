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
  // 国旗 SVG 始终作为独立文件输出，避免被内联进 JS/CSS
  build: { outDir, emptyOutDir: true, chunkSizeWarningLimit: 700, assetsInlineLimit: (file) => (file.endsWith('.svg') ? false : undefined) },
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
