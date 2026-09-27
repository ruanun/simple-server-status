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
