/**
 * 连接模式常量定义
 *
 * 职责：
 * - 定义连接模式的标准常量
 * - 提供连接模式的显示名称
 * - 统一连接模式的类型定义
 *
 * @author ruan
 */

/**
 * 连接模式常量
 * 使用 as const 确保类型安全
 */
export const CONNECTION_MODES = {
  /** WebSocket 实时模式 */
  WEBSOCKET: 'websocket',
  /** HTTP 轮询模式 */
  HTTP: 'http'
} as const

/**
 * 连接模式类型
 * 从 CONNECTION_MODES 推导出联合类型
 */
export type ConnectionMode = typeof CONNECTION_MODES[keyof typeof CONNECTION_MODES]

/**
 * 连接模式显示名称（中文）
 */
export const CONNECTION_MODE_LABELS_ZH = {
  [CONNECTION_MODES.WEBSOCKET]: 'WebSocket 实时模式',
  [CONNECTION_MODES.HTTP]: 'HTTP 轮询模式'
} as const

/**
 * 连接模式显示名称（英文）
 */
export const CONNECTION_MODE_LABELS_EN = {
  [CONNECTION_MODES.WEBSOCKET]: 'WebSocket Real-time',
  [CONNECTION_MODES.HTTP]: 'HTTP Polling'
} as const

/**
 * 根据语言获取连接模式的显示名称
 * @param mode 连接模式
 * @param locale 语言环境，默认为中文
 * @returns 显示名称
 */
export function getConnectionModeLabel(mode: ConnectionMode, locale: string = 'zh-CN'): string {
  const labels = locale === 'zh-CN' ? CONNECTION_MODE_LABELS_ZH : CONNECTION_MODE_LABELS_EN
  return labels[mode]
}
