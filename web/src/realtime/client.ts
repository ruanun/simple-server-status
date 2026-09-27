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
