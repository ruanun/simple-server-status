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
