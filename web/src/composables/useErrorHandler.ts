/**
 * 统一错误处理 Composable
 *
 * 职责：
 * - 提供统一的 HTTP 错误处理
 * - 提供统一的 WebSocket 错误处理
 * - 统一的错误日志和用户提示
 *
 * 使用方式：
 * ```typescript
 * import { useErrorHandler } from '@/composables/useErrorHandler'
 * const { handleHttpError, handleWebSocketError } = useErrorHandler()
 *
 * try {
 *   await someHttpRequest()
 * } catch (error) {
 *   handleHttpError(error, '获取数据')
 * }
 * ```
 *
 * @author ruan
 */

import { message } from 'ant-design-vue'

export function useErrorHandler() {
  /**
   * 处理 HTTP 请求错误
   * @param error 错误对象
   * @param context 操作上下文（用于错误消息）
   */
  function handleHttpError(error: any, context: string) {
    console.error(`${context}失败:`, error)
    message.error(`${context}失败，请稍后重试`)
  }

  /**
   * 处理 WebSocket 连接错误
   * @param error 错误对象
   */
  function handleWebSocketError(error: any) {
    console.error('WebSocket 错误:', error)
    message.warning('WebSocket 连接失败，已切换到 HTTP 轮询模式')
  }

  return {
    handleHttpError,
    handleWebSocketError
  }
}
