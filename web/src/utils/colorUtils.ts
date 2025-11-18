/**
 * 颜色工具函数
 *
 * 职责：
 * - 定义百分比阈值和颜色常量
 * - 提供根据百分比返回颜色的工具函数
 * - 统一百分比颜色判断逻辑
 *
 * @author ruan
 */

/**
 * 百分比阈值常量
 * 用于判断状态的临界点
 */
export const PERCENT_THRESHOLDS = {
  /** 危险阈值 (>= 90%) */
  critical: 90,
  /** 警告阈值 (>= 70%) */
  warning: 70
} as const

/**
 * 百分比状态颜色常量
 * 基于 Ant Design 的色彩规范
 */
export const PERCENT_COLORS = {
  /** 危险状态 - 红色 */
  critical: '#ff4d4f',
  /** 警告状态 - 橙色 */
  warning: '#faad14',
  /** 正常状态 - 绿色 */
  normal: '#52c41a',
  /** 默认状态 - 空字符串（使用组件默认色） */
  default: ''
} as const

/**
 * 百分比状态 CSS 类名
 */
export const PERCENT_CLASSES = {
  critical: 'critical',
  warning: 'warning',
  normal: 'normal'
} as const

/**
 * 根据百分比返回对应的颜色
 * @param percent 百分比值 (0-100)
 * @param useDefault 是否在正常状态下返回空字符串（默认：true，兼容 Ant Design Progress 组件）
 * @returns 颜色值
 *
 * @example
 * ```typescript
 * getPercentColor(95) // '#ff4d4f' (红色)
 * getPercentColor(75) // '#faad14' (橙色)
 * getPercentColor(50) // '' (默认)
 * getPercentColor(50, false) // '#52c41a' (绿色)
 * ```
 */
export function getPercentColor(percent: number, useDefault: boolean = true): string {
  if (percent >= PERCENT_THRESHOLDS.critical) {
    return PERCENT_COLORS.critical
  }
  if (percent >= PERCENT_THRESHOLDS.warning) {
    return PERCENT_COLORS.warning
  }
  return useDefault ? PERCENT_COLORS.default : PERCENT_COLORS.normal
}

/**
 * 根据百分比返回对应的 CSS 类名
 * @param percent 百分比值 (0-100)
 * @returns CSS 类名
 *
 * @example
 * ```typescript
 * getPercentClass(95) // 'critical'
 * getPercentClass(75) // 'warning'
 * getPercentClass(50) // 'normal'
 * ```
 */
export function getPercentClass(percent: number): string {
  if (percent >= PERCENT_THRESHOLDS.critical) {
    return PERCENT_CLASSES.critical
  }
  if (percent >= PERCENT_THRESHOLDS.warning) {
    return PERCENT_CLASSES.warning
  }
  return PERCENT_CLASSES.normal
}

/**
 * 判断百分比是否处于危险状态
 * @param percent 百分比值 (0-100)
 * @returns 是否危险
 */
export function isCriticalPercent(percent: number): boolean {
  return percent >= PERCENT_THRESHOLDS.critical
}

/**
 * 判断百分比是否处于警告状态
 * @param percent 百分比值 (0-100)
 * @returns 是否警告
 */
export function isWarningPercent(percent: number): boolean {
  return percent >= PERCENT_THRESHOLDS.warning && percent < PERCENT_THRESHOLDS.critical
}

/**
 * 判断百分比是否处于正常状态
 * @param percent 百分比值 (0-100)
 * @returns 是否正常
 */
export function isNormalPercent(percent: number): boolean {
  return percent < PERCENT_THRESHOLDS.warning
}
