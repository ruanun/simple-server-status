/**
 * 服务器数据服务层
 *
 * 职责：
 * - 封装服务器数据的 HTTP API 调用
 * - 提供统一的数据获取接口
 * - 处理 API 错误
 *
 * 使用方式：
 * ```typescript
 * import { serverService } from '@/services/serverService'
 *
 * try {
 *   const data = await serverService.fetchServerInfo()
 *   // 处理数据
 * } catch (error) {
 *   // 处理错误
 * }
 * ```
 *
 * @author ruan
 */

import http from '@/api'
import type { ServerInfo } from '@/api/models'

/**
 * 服务器数据服务类
 */
export class ServerService {
  /**
   * 获取所有服务器状态信息
   * @returns Promise<ServerInfo[]> 服务器信息数组
   * @throws Error 当 HTTP 请求失败时抛出
   */
  async fetchServerInfo(): Promise<ServerInfo[]> {
    try {
      const response = await http.get<Array<ServerInfo>>("/server/statusInfo")
      return response.data
    } catch (error) {
      console.error('Failed to fetch server info:', error)
      throw error
    }
  }
}

/**
 * 服务器数据服务单例实例
 */
export const serverService = new ServerService()
