/**
 * 服务器信息格式化 Composable
 *
 * 职责：
 * - 统一管理服务器信息相关的格式化函数导入
 * - 提供一致的格式化函数访问接口
 * - 减少重复的导入语句
 *
 * 使用方式：
 * ```typescript
 * import { useServerInfoFormatting } from '@/composables/useServerInfoFormatting'
 *
 * const {
 *   readableBytes,
 *   formatUptime,
 *   formatPercent,
 *   formatLoad,
 *   getPercentColor
 * } = useServerInfoFormatting()
 * ```
 *
 * @author ruan
 */

import { readableBytes } from '@/utils/CommonUtil'
import { formatUptime, formatPercent, formatLoad } from '@/utils/formatters'
import { getPercentColor } from '@/utils/colorUtils'

/**
 * 服务器信息格式化工具集合
 * 统一导出所有格式化相关函数
 */
export function useServerInfoFormatting() {
  return {
    /**
     * 字节数格式化为易读字符串
     * @example readableBytes(1024) // "1.0 KB"
     */
    readableBytes,

    /**
     * 运行时间格式化（秒 → 天/小时/分钟）
     * @example formatUptime(86400) // "1天 0小时"
     */
    formatUptime,

    /**
     * 百分比格式化（保留整数）
     * @example formatPercent(75.6) // 76
     */
    formatPercent,

    /**
     * 系统负载格式化
     * @example formatLoad(1.2, 1.5, 1.9) // "1.2 / 1.5 / 1.9"
     */
    formatLoad,

    /**
     * 根据百分比返回对应颜色
     * @example getPercentColor(95) // "#ff4d4f" (红色)
     */
    getPercentColor
  }
}
