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

/** localizeError 中文界面优先使用后端原文（后端错误为中文且带细节）；否则按错误码翻译，无译文时回退后端原文 */
function localizeError(code: string, serverMessage: string | undefined, fallback: string): string {
  if (serverMessage && i18n.language.startsWith('zh')) return serverMessage
  const key = `errors.codes.${code}`
  if (i18n.exists(key)) return i18n.t(key)
  return serverMessage ?? fallback
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
    throw new ApiError(0, 'network', localizeError('network', undefined, i18n.t('errors.network')))
  }
  const json = parse(await res.text())
  if (!res.ok) {
    if (res.status === 401 && path !== '/api/auth/login') tokenStore.clear()
    const code = json?.error?.code ?? `http_${res.status}`
    throw new ApiError(res.status, code, localizeError(code, json?.error?.message, i18n.t('errors.http', { status: res.status })))
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
