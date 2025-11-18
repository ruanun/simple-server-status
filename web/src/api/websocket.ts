/**
 * WebSocket 客户端模块
 *
 * 职责：
 * - 管理 WebSocket 连接生命周期（连接、断开、重连）
 * - 处理心跳保活机制
 * - 提供连接状态和统计信息
 * - 发布消息事件供外部订阅
 *
 * 设计原则：
 * - 单一职责：只负责连接管理，不处理业务数据
 * - 事件驱动：通过事件发布消息，由 Store 层处理
 * - 自动重连：支持指数退避的自动重连策略
 *
 * @author ruan
 */

import { ref, reactive } from 'vue'
import type { ServerInfo } from '@/api/models'
import { message } from 'ant-design-vue'
import type { IncomingMessage } from '@/types/websocket'
import { isServerStatusUpdateMessage, isPongMessage } from '@/types/websocket'

// WebSocket连接状态枚举
enum WebSocketStatus {
  CONNECTING = 'connecting',
  CONNECTED = 'connected',
  DISCONNECTED = 'disconnected',
  RECONNECTING = 'reconnecting',
  ERROR = 'error'
}

// WebSocket事件类型
interface WebSocketEvents {
  onStatusUpdate: (data: ServerInfo[]) => void
  onConnectionChange: (status: WebSocketStatus) => void
  onError: (error: Event) => void
}

// WebSocket客户端类
class WebSocketClient {
  private ws: WebSocket | null = null
  private url: string
  private reconnectAttempts = 0
  private maxReconnectAttempts = 5
  private reconnectInterval = 3000 // 3秒
  private heartbeatInterval = 30000 // 30秒心跳
  private heartbeatTimer: number | null = null
  private reconnectTimer: number | null = null
  private isManualClose = false

  // 响应式状态
  public status = ref<WebSocketStatus>(WebSocketStatus.DISCONNECTED)
  public connectionStats = reactive({
    connectTime: null as Date | null,
    reconnectCount: 0,
    messageCount: 0,
    lastMessageTime: null as Date | null
  })

  // 事件回调 - 支持多个监听器
  private events: Map<string, Set<Function>> = new Map()

  constructor() {
    // 根据当前协议构建WebSocket URL
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    // 使用当前页面的 host，在开发环境下 Vite 会自动代理到后端
    const host = window.location.host
    this.url = `${protocol}//${host}/ws-frontend`

    // 调试日志：输出连接信息
    console.log('[WebSocket] 初始化连接配置')
    console.log('[WebSocket] 页面地址:', window.location.href)
    console.log('[WebSocket] 连接地址:', this.url)
    console.log('[WebSocket] 开发模式:', import.meta.env.DEV)
  }

  // 注册事件监听器（支持多个监听器）
  on<K extends keyof WebSocketEvents>(event: K, callback: WebSocketEvents[K]) {
    if (!this.events.has(event)) {
      this.events.set(event, new Set())
    }
    this.events.get(event)!.add(callback as Function)
  }

  // 移除事件监听器
  off<K extends keyof WebSocketEvents>(event: K, callback?: WebSocketEvents[K]) {
    if (!callback) {
      // 如果没有指定回调，移除该事件的所有监听器
      this.events.delete(event)
    } else {
      // 移除特定的回调函数
      this.events.get(event)?.delete(callback as Function)
    }
  }

  // 触发事件（私有方法）
  private emit<K extends keyof WebSocketEvents>(
    event: K,
    data: Parameters<WebSocketEvents[K]>[0]
  ) {
    const callbacks = this.events.get(event)
    if (callbacks) {
      callbacks.forEach(callback => {
        try {
          callback(data)
        } catch (error) {
          console.error(`Error in ${event} callback:`, error)
        }
      })
    }
  }

  // 连接WebSocket
  connect(): Promise<void> {
    return new Promise((resolve, reject) => {
      if (this.ws && this.ws.readyState === WebSocket.OPEN) {
        resolve()
        return
      }

      this.isManualClose = false
      this.updateStatus(WebSocketStatus.CONNECTING)

      try {
        this.ws = new WebSocket(this.url)

        this.ws.onopen = () => {
          console.log('[WebSocket] 连接已建立')
          this.updateStatus(WebSocketStatus.CONNECTED)
          this.connectionStats.connectTime = new Date()
          this.reconnectAttempts = 0
          this.startHeartbeat()
          resolve()
        }

        this.ws.onmessage = (event) => {
          this.handleMessage(event)
        }

        this.ws.onclose = (event) => {
          this.handleClose(event)
        }

        this.ws.onerror = (event) => {
          console.error('[WebSocket] 连接错误:', event)
          console.error('[WebSocket] 尝试连接的地址:', this.url)
          this.updateStatus(WebSocketStatus.ERROR)
          this.emit('onError', event)
          reject(event)
        }

      } catch (error) {
        console.error('[WebSocket] 连接失败:', error)
        console.error('[WebSocket] 尝试连接的地址:', this.url)
        this.updateStatus(WebSocketStatus.ERROR)
        reject(error)
      }
    })
  }

  // 断开连接
  disconnect() {
    this.isManualClose = true
    this.stopHeartbeat()
    this.stopReconnect()
    
    if (this.ws) {
      this.ws.close(1000, '手动断开连接')
      this.ws = null
    }
    
    this.updateStatus(WebSocketStatus.DISCONNECTED)
  }

  // 发送消息
  send(data: any) {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(data))
    } else {
      console.warn('WebSocket未连接，无法发送消息')
    }
  }

  // 处理接收到的消息
  private handleMessage(event: MessageEvent) {
    this.connectionStats.messageCount++
    this.connectionStats.lastMessageTime = new Date()

    try {
      const message = JSON.parse(event.data) as IncomingMessage

      // 使用类型守卫处理不同类型的消息
      if (isPongMessage(message)) {
        // 心跳响应，无需处理
        return
      }

      if (isServerStatusUpdateMessage(message)) {
        // 触发状态更新回调，由 Store 处理数据
        this.emit('onStatusUpdate', message.data)
        return
      }

      // 未知消息类型
      console.warn('未知的 WebSocket 消息类型:', message)
    } catch (error) {
      console.error('WebSocket消息解析失败:', error)
      this.emit('onError', event as any)
    }
  }

  // 处理连接关闭
  private handleClose(event: CloseEvent) {
    console.log('WebSocket连接已关闭:', event.code, event.reason)
    this.stopHeartbeat()
    
    if (!this.isManualClose) {
      this.updateStatus(WebSocketStatus.DISCONNECTED)
      this.attemptReconnect()
    }
  }

  // 尝试重连
  private attemptReconnect() {
    if (this.reconnectAttempts >= this.maxReconnectAttempts) {
      console.error('达到最大重连次数，停止重连')
      message.error('WebSocket连接失败，自动切换到 HTTP 轮询模式')
      // 设置状态为 ERROR，触发自动故障转移
      this.updateStatus(WebSocketStatus.ERROR)
      return
    }

    this.reconnectAttempts++
    this.connectionStats.reconnectCount++
    this.updateStatus(WebSocketStatus.RECONNECTING)

    console.log(`尝试第${this.reconnectAttempts}次重连...`)

    this.reconnectTimer = window.setTimeout(() => {
      this.connect().catch(() => {
        // 重连失败，继续尝试
        this.attemptReconnect()
      })
    }, this.reconnectInterval * this.reconnectAttempts) // 指数退避
  }

  // 开始心跳
  private startHeartbeat() {
    this.stopHeartbeat()
    this.heartbeatTimer = window.setInterval(() => {
      if (this.ws && this.ws.readyState === WebSocket.OPEN) {
        this.send({ type: 'ping' })
      }
    }, this.heartbeatInterval)
  }

  // 停止心跳
  private stopHeartbeat() {
    if (this.heartbeatTimer) {
      clearInterval(this.heartbeatTimer)
      this.heartbeatTimer = null
    }
  }

  // 停止重连
  private stopReconnect() {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }
  }

  // 更新连接状态
  private updateStatus(newStatus: WebSocketStatus) {
    this.status.value = newStatus
    this.emit('onConnectionChange', newStatus)
  }

  // 获取连接状态
  getStatus(): WebSocketStatus {
    return this.status.value
  }

  // 检查是否已连接
  isConnected(): boolean {
    return this.status.value === WebSocketStatus.CONNECTED
  }

  // 获取连接统计信息
  getStats() {
    return {
      ...this.connectionStats,
      status: this.status.value,
      reconnectAttempts: this.reconnectAttempts
    }
  }
}

// ==================== 全局实例和导出 ====================

/**
 * WebSocket 全局实例（内部使用）
 * 外部应该通过 useWebSocket() 访问
 */
const websocketClient = new WebSocketClient()

/**
 * WebSocket 组合式 API
 * 统一的 WebSocket 访问入口
 *
 * @example
 * ```typescript
 * import { useWebSocket } from '@/api/websocket'
 *
 * const ws = useWebSocket()
 * await ws.connect()
 * console.log(ws.status.value)
 * ```
 */
export function useWebSocket() {
  return {
    client: websocketClient,
    status: websocketClient.status,
    connectionStats: websocketClient.connectionStats,
    connect: () => websocketClient.connect(),
    disconnect: () => websocketClient.disconnect(),
    isConnected: () => websocketClient.isConnected(),
    getStats: () => websocketClient.getStats()
  }
}

// ==================== 导出类型 ====================

// 导出类型和枚举供外部使用
export { WebSocketStatus, WebSocketClient }
export type { WebSocketEvents }