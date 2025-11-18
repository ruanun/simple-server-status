/**
 * HTTP API 类型定义
 *
 * 职责：
 * - 定义 HTTP 响应的标准格式
 * - 提供响应码枚举
 * - 统一错误类型定义
 *
 * @author ruan
 */

/**
 * API 响应码枚举
 * 标准的 HTTP 状态码
 */
export enum ResponseCode {
  /** 成功 */
  SUCCESS = 200,
  /** 客户端错误 */
  BAD_REQUEST = 400,
  /** 未授权 */
  UNAUTHORIZED = 401,
  /** 未找到 */
  NOT_FOUND = 404,
  /** 服务器错误 */
  INTERNAL_ERROR = 500,
  /** 请求超时 */
  TIMEOUT = 30000
}

/**
 * API 响应基础接口
 * 不包含 data 字段
 */
export interface ApiResult {
  code: ResponseCode
  message: string
}

/**
 * API 响应接口（包含 data）
 * 所有成功的 API 响应都应该包含 data 字段
 */
export interface ApiResponse<T = any> extends ApiResult {
  data?: T
}

/**
 * API 错误接口
 * 统一的错误格式
 */
export interface ApiError {
  code: ResponseCode
  message: string
  originalError?: any
}

/**
 * 类型守卫：判断响应是否成功
 */
export function isSuccessResponse(response: ApiResponse): boolean {
  return response.code === ResponseCode.SUCCESS
}

/**
 * 类型守卫：判断是否为 ApiError
 */
export function isApiError(error: any): error is ApiError {
  return error && typeof error.code === 'number' && typeof error.message === 'string'
}
