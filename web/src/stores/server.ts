/**
 * 服务器数据状态管理 Store
 *
 * 职责：
 * - 管理服务器数据的存储和更新
 * - 提供服务器分组和统计信息
 * - 支持 HTTP 轮询和 WebSocket 两种数据源
 *
 * 使用方式：
 * ```typescript
 * import { useServerStore } from '@/stores/server'
 * import { storeToRefs } from 'pinia'
 * const serverStore = useServerStore()
 *
 * // 解构响应式数据（必须使用 storeToRefs）
 * const { groupedServers, groupNames, totalCount, onlineCount } = storeToRefs(serverStore)
 *
 * // 更新数据（HTTP 轮询和 WebSocket 统一使用）
 * serverStore.setServerData(serverInfoArray)
 * ```
 *
 * @author ruan
 */

import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { ServerInfo } from '@/api/models'

/**
 * 服务器数据 Store
 * 管理服务器列表、分组和统计信息
 */
export const useServerStore = defineStore('server', () => {
  // ==================== 状态 ====================

  /**
   * 服务器数据原始存储
   * key: 分组名称
   * value: 该分组下的服务器列表
   */
  const serverData = ref<Map<string, ServerInfo[]>>(new Map())

  // ==================== Getters ====================

  /**
   * 分组后的服务器数据
   */
  const groupedServers = computed(() => serverData.value)

  /**
   * 所有分组名称列表
   */
  const groupNames = computed(() => {
    return Array.from(serverData.value.keys())
  })

  /**
   * 服务器总数
   */
  const totalCount = computed(() => {
    let total = 0
    serverData.value.forEach(servers => {
      total += servers.length
    })
    return total
  })

  /**
   * 在线服务器数量
   */
  const onlineCount = computed(() => {
    let online = 0
    serverData.value.forEach(servers => {
      online += servers.filter(server => server.isOnline).length
    })
    return online
  })

  // ==================== Actions ====================

  /**
   * 设置服务器数据（HTTP 轮询和 WebSocket 统一使用）
   * @param data 服务器信息数组
   */
  function setServerData(data: ServerInfo[]) {
    const map = new Map<string, ServerInfo[]>()

    data.forEach(item => {
      if (!map.has(item.group)) {
        map.set(item.group, [])
      }
      map.get(item.group)?.push(item)
    })

    serverData.value = map
  }

  return {
    // 状态
    serverData,
    // Getters
    groupedServers,
    groupNames,
    totalCount,
    onlineCount,
    // Actions
    setServerData
  }
})
