/**
 * 连接管理 Composable
 *
 * 职责：
 * - 管理 WebSocket/HTTP 两种连接模式
 * - 处理连接初始化、切换、清理
 * - 管理 HTTP 轮询定时器
 * - 订阅 WebSocket 事件
 *
 * 使用方式：
 * ```typescript
 * import { useConnectionManager } from '@/composables/useConnectionManager'
 * const { initializeConnection, switchToHTTP, switchToWebSocket, cleanup } = useConnectionManager()
 *
 * // 初始化连接
 * onMounted(() => initializeConnection())
 *
 * // 清理资源
 * onUnmounted(() => cleanup())
 * ```
 *
 * @author ruan
 */

import { onMounted, onUnmounted } from 'vue'
import { message } from 'ant-design-vue'
import { useWebSocket, WebSocketStatus } from '@/api/websocket'
import { useConnectionStore } from '@/stores/connection'
import { useServerStore } from '@/stores/server'
import { serverService } from '@/services/serverService'
import { useErrorHandler } from '@/composables/useErrorHandler'
import { CONNECTION_MODES } from '@/constants/connectionModes'
import type { ServerInfo } from '@/api/models'
import { storeToRefs } from 'pinia'

export function useConnectionManager() {
  const connectionStore = useConnectionStore()
  const serverStore = useServerStore()
  const { isWebSocketMode } = storeToRefs(connectionStore)
  const { client, connect, disconnect } = useWebSocket()
  const { handleHttpError } = useErrorHandler()

  // 轮询定时器
  let pollingTimer: number | null = null

  // ==================== 数据获取 ====================

  /**
   * 获取服务器数据并更新 Store
   */
  async function fetchServerData() {
    try {
      const data = await serverService.fetchServerInfo()
      serverStore.setServerData(data)
    } catch (error) {
      handleHttpError(error, '获取服务器信息')
    }
  }

  // ==================== HTTP 轮询管理 ====================

  /**
   * 启动 HTTP 轮询
   * @param skipFirstLoad 是否跳过首次加载（默认：false）
   */
  function startPolling(skipFirstLoad = false) {
    stopPolling() // 先停止现有轮询，避免重复
    if (!skipFirstLoad) {
      fetchServerData()
    }
    pollingTimer = window.setInterval(fetchServerData, 2000)
  }

  /**
   * 停止 HTTP 轮询
   */
  function stopPolling() {
    if (pollingTimer) {
      clearInterval(pollingTimer)
      pollingTimer = null
    }
  }

  // ==================== 连接模式切换 ====================

  /**
   * 切换到 HTTP 轮询模式
   */
  function switchToHTTP() {
    disconnect()
    stopPolling()
    startPolling()
    message.info('已切换到 HTTP 轮询模式')
  }

  /**
   * 切换到 WebSocket 实时模式
   */
  async function switchToWebSocket() {
    stopPolling()
    try {
      await connect()
      message.success('已切换到 WebSocket 实时模式')
    } catch (error) {
      message.error('WebSocket 连接失败，回退到 HTTP 轮询模式')
      connectionStore.setMode(CONNECTION_MODES.HTTP)
      startPolling()
    }
  }

  // ==================== 连接初始化 ====================

  /**
   * 初始化连接
   * 根据 Connection Store 的模式决定使用 WebSocket 或 HTTP 轮询
   */
  async function initializeConnection() {
    if (isWebSocketMode.value) {
      try {
        await connect()
      } catch (error) {
        console.error('WebSocket 连接失败，回退到 HTTP 轮询:', error)
        connectionStore.setMode(CONNECTION_MODES.HTTP)
        await fetchServerData()
        startPolling(true) // 跳过首次加载，因为上面已经加载过了
      }
    } else {
      await fetchServerData()
      startPolling(true) // 跳过首次加载，因为上面已经加载过了
    }
  }

  // ==================== 资源清理 ====================

  /**
   * 清理所有资源
   * 包括停止轮询、断开 WebSocket、移除事件监听
   */
  function cleanup() {
    stopPolling()
    disconnect()
    // 移除所有事件监听器
    client.off('onStatusUpdate')
    client.off('onConnectionChange')
  }

  // ==================== WebSocket 事件订阅 ====================

  /**
   * 设置 WebSocket 事件监听
   * 在组件挂载时自动注册
   */
  onMounted(() => {
    // 监听数据更新
    client.on('onStatusUpdate', (data: ServerInfo[]) => {
      serverStore.setServerData(data)
    })

    // 监听连接状态变化
    client.on('onConnectionChange', (newStatus: WebSocketStatus) => {
      if (newStatus === WebSocketStatus.ERROR && isWebSocketMode.value) {
        // WebSocket 连接失败，自动切换到 HTTP 轮询模式
        message.warning('WebSocket 连接失败，自动切换到 HTTP 轮询模式')
        connectionStore.setMode(CONNECTION_MODES.HTTP)
        startPolling()
      }
    })
  })

  /**
   * 组件卸载时自动清理
   */
  onUnmounted(() => {
    cleanup()
  })

  // ==================== 导出 API ====================

  return {
    // 连接管理
    initializeConnection,
    switchToHTTP,
    switchToWebSocket,
    cleanup,
    // 数据获取
    fetchServerData,
    // 轮询管理
    startPolling,
    stopPolling
  }
}
