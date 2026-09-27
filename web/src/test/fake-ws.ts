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
