/**
 * WebSocket 消息类型定义
 *
 * 职责：
 * - 定义 WebSocket 消息的完整类型结构
 * - 提供类型安全的消息解析
 * - 统一消息格式规范
 *
 * @author ruan
 */

import type { ServerInfo } from '@/api/models'

/**
 * WebSocket 消息基础接口
 * 所有 WebSocket 消息都应该包含 type 字段
 */
export interface WebSocketMessage<T = any> {
  type: string
  data?: T
}

/**
 * 服务器状态更新消息
 * 从后端推送的服务器状态数据
 */
export interface ServerStatusUpdateMessage extends WebSocketMessage<ServerInfo[]> {
  type: 'server_status_update'
  data: ServerInfo[]
}

/**
 * Ping 消息（客户端发送）
 * 用于心跳保活
 */
export interface PingMessage extends WebSocketMessage {
  type: 'ping'
}

/**
 * Pong 消息（服务端响应）
 * 心跳响应
 */
export interface PongMessage extends WebSocketMessage {
  type: 'pong'
}

/**
 * 接收消息的联合类型
 * 包含所有可能从服务端接收到的消息类型
 */
export type IncomingMessage = ServerStatusUpdateMessage | PongMessage

/**
 * 发送消息的联合类型
 * 包含所有可能发送到服务端的消息类型
 */
export type OutgoingMessage = PingMessage

/**
 * 消息类型枚举
 * 用于类型检查和 switch 语句
 */
export const MessageType = {
  // 接收
  SERVER_STATUS_UPDATE: 'server_status_update',
  PONG: 'pong',
  // 发送
  PING: 'ping'
} as const

/**
 * 类型守卫：判断是否为服务器状态更新消息
 */
export function isServerStatusUpdateMessage(
  message: IncomingMessage
): message is ServerStatusUpdateMessage {
  return message.type === MessageType.SERVER_STATUS_UPDATE
}

/**
 * 类型守卫：判断是否为 Pong 消息
 */
export function isPongMessage(message: IncomingMessage): message is PongMessage {
  return message.type === MessageType.PONG
}
