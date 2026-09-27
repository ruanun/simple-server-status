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
