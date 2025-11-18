/**
 * 连接状态管理 Store
 *
 * 职责：
 * - 管理 WebSocket/HTTP 连接模式切换
 * - 提供连接状态和统计信息的响应式访问
 * - 处理重连逻辑
 *
 * 使用方式：
 * ```typescript
 * import { useConnectionStore } from '@/stores/connection'
 * const connectionStore = useConnectionStore()
 *
 * // 读取状态
 * console.log(connectionStore.mode) // 'websocket' | 'http'
 * console.log(connectionStore.isWebSocketMode)
 *
 * // 切换模式
 * connectionStore.toggleMode()
 *
 * // 重连
 * await connectionStore.reconnect()
 * ```
 *
 * @author ruan
 */

import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { useWebSocket } from '@/api/websocket'
import { CONNECTION_MODES, type ConnectionMode } from '@/constants/connectionModes'

// 获取 WebSocket 实例
const ws = useWebSocket()

/**
 * 连接状态 Store
 * 管理 WebSocket/HTTP 连接模式和状态
 */
export const useConnectionStore = defineStore('connection', () => {
  // ==================== WebSocket 响应式状态 ====================

  /**
   * WebSocket 连接状态（直接引用全局实例）
   */
  const status = ws.status

  /**
   * WebSocket 连接统计信息（直接引用全局实例）
   */
  const connectionStats = ws.connectionStats

  // ==================== 状态 ====================

  /**
   * 当前连接模式
   * - websocket: WebSocket 实时模式
   * - http: HTTP 轮询模式
   */
  const mode = ref<ConnectionMode>(CONNECTION_MODES.WEBSOCKET)

  // ==================== Getters ====================

  /**
   * WebSocket 连接状态
   */
  const websocketStatus = computed(() => status.value)

  /**
   * WebSocket 连接统计信息
   */
  const stats = computed(() => connectionStats)

  /**
   * 是否为 WebSocket 模式
   */
  const isWebSocketMode = computed(() => mode.value === CONNECTION_MODES.WEBSOCKET)

  // ==================== Actions ====================

  /**
   * 切换连接模式（WebSocket ↔ HTTP）
   * 注意：此方法仅切换模式状态，实际的连接/断开逻辑由组件监听状态变化后执行
   */
  function toggleMode() {
    mode.value = mode.value === CONNECTION_MODES.WEBSOCKET
      ? CONNECTION_MODES.HTTP
      : CONNECTION_MODES.WEBSOCKET
  }

  /**
   * 设置连接模式
   * @param newMode 新的连接模式
   */
  function setMode(newMode: ConnectionMode) {
    mode.value = newMode
  }

  /**
   * 重连 WebSocket
   * @returns Promise，resolve 表示成功，reject 表示失败
   */
  async function reconnect() {
    return await ws.connect()
  }

  return {
    // 状态
    mode,
    // Getters
    websocketStatus,
    stats,
    isWebSocketMode,
    // Actions
    toggleMode,
    setMode,
    reconnect
  }
})
