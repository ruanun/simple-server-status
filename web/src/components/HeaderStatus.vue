<template>
  <div class="header-status">
    <!-- 统计信息 -->
    <div class="stats-info">
      <span class="stats-item">
        <cloud-server-outlined class="stats-icon" />
        <span class="stats-value total">{{ totalCount }}</span>
        <span class="stats-label">{{ t('header.servers') }}</span>
        <span class="stats-separator">/</span>
        <check-circle-outlined class="stats-icon online" />
        <span class="stats-value online">{{ onlineCount }}</span>
        <span class="stats-label">{{ t('header.online') }}</span>
      </span>
    </div>
    
    <!-- WebSocket状态指示器 -->
    <div class="connection-status">
      <div class="status-indicator">
        <div class="status-dot" :style="{ backgroundColor: statusColor }"></div>
        <span class="status-text">{{ statusText }}</span>
        <span class="connection-mode">({{ isWebSocketMode ? t('header.mode.websocket') : t('header.mode.http') }})</span>
      </div>
    </div>

    <!-- 控制按钮 -->
    <div class="status-controls">
      <a-dropdown placement="bottomRight">
        <template #overlay>
          <a-menu>
            <a-menu-item key="toggle" @click="handleToggleMode">
              <template #icon>
                <swap-outlined />
              </template>
              {{ isWebSocketMode ? t('header.actions.switchToHttp') : t('header.actions.switchToWebsocket') }}
            </a-menu-item>
            <a-menu-item
              v-if="isWebSocketMode && status !== WebSocketStatus.CONNECTED"
              key="reconnect"
              @click="handleReconnect"
            >
              <template #icon>
                <reload-outlined />
              </template>
              {{ t('header.actions.reconnect') }}
            </a-menu-item>
            <a-menu-divider />
            <a-menu-item-group :title="t('common.language')">
              <a-menu-item key="lang-zh" @click="changeLanguage('zh-CN')">
                <template #icon>
                  <check-outlined v-if="currentLocale === 'zh-CN'" />
                </template>
                简体中文
              </a-menu-item>
              <a-menu-item key="lang-en" @click="changeLanguage('en-US')">
                <template #icon>
                  <check-outlined v-if="currentLocale === 'en-US'" />
                </template>
                English
              </a-menu-item>
            </a-menu-item-group>
            <a-menu-divider />
            <a-menu-item-group :title="t('header.stats.title')">
              <a-menu-item key="messages" disabled>
                <template #icon>
                  <info-circle-outlined />
                </template>
                {{ t('header.stats.messages') }}: {{ connectionStats.messageCount }}
              </a-menu-item>
              <a-menu-item key="reconnections" disabled>
                <template #icon>
                  <reload-outlined />
                </template>
                {{ t('header.stats.reconnections') }}: {{ connectionStats.reconnectCount }}
              </a-menu-item>
              <a-menu-item v-if="connectionStats.connectTime" key="uptime" disabled>
                <template #icon>
                  <check-circle-outlined />
                </template>
                {{ t('header.stats.uptime') }}: {{ formatConnectionUptime() }}
              </a-menu-item>
            </a-menu-item-group>
          </a-menu>
        </template>
        <a-button type="text" size="small">
          <template #icon>
            <setting-outlined />
          </template>
        </a-button>
      </a-dropdown>
    </div>
  </div>
</template>

<script lang="ts" setup>
import { computed, ref } from 'vue'
import {
  SwapOutlined,
  ReloadOutlined,
  SettingOutlined,
  InfoCircleOutlined,
  CheckOutlined,
  CloudServerOutlined,
  CheckCircleOutlined
} from '@ant-design/icons-vue'
import { useWebSocket, WebSocketStatus } from '@/api/websocket'
import { useI18n } from 'vue-i18n'
import { setLocale, getLocale } from '@/locales'
import type { LocaleType } from '@/locales/types'
import { useConnectionStore } from '@/stores/connection'
import { useServerStore } from '@/stores/server'
import { storeToRefs } from 'pinia'
import { formatTimeInterval } from '@/utils/formatters'

const { t } = useI18n()

// Connection Store
const connectionStore = useConnectionStore()
const { isWebSocketMode } = storeToRefs(connectionStore)

// Server Store
const serverStore = useServerStore()
const { totalCount, onlineCount } = storeToRefs(serverStore)

const { status, connectionStats } = useWebSocket()

// 当前语言
const currentLocale = ref<LocaleType>(getLocale())

// 切换语言
function changeLanguage(locale: LocaleType) {
  setLocale(locale)
  currentLocale.value = locale
}

// 格式化连接时长
function formatConnectionUptime(): string {
  return formatTimeInterval(connectionStats.connectTime, currentLocale.value)
}

// WebSocket状态相关
const statusColor = computed(() => {
  if (!isWebSocketMode.value) return '#52c41a' // HTTP模式显示绿色

  switch (status.value) {
    case WebSocketStatus.CONNECTED:
      return '#52c41a' // 绿色
    case WebSocketStatus.CONNECTING:
    case WebSocketStatus.RECONNECTING:
      return '#faad14' // 黄色
    case WebSocketStatus.DISCONNECTED:
    case WebSocketStatus.ERROR:
      return '#ff4d4f' // 红色
    default:
      return '#d9d9d9' // 灰色
  }
})

const statusText = computed(() => {
  if (!isWebSocketMode.value) return t('header.status.httpPolling')

  switch (status.value) {
    case WebSocketStatus.CONNECTED:
      return t('header.status.connected')
    case WebSocketStatus.CONNECTING:
      return t('header.status.connecting')
    case WebSocketStatus.RECONNECTING:
      return t('header.status.reconnecting')
    case WebSocketStatus.DISCONNECTED:
      return t('header.status.disconnected')
    case WebSocketStatus.ERROR:
      return t('header.status.error')
    default:
      return t('header.status.unknown')
  }
})

// 切换连接模式
function handleToggleMode() {
  connectionStore.toggleMode()
}

// 重连 WebSocket
async function handleReconnect() {
  try {
    await connectionStore.reconnect()
  } catch (error) {
    // 错误已在 Store 中处理
  }
}
</script>

<style scoped>
.header-status {
  display: flex;
  align-items: center;
  gap: 16px;
  height: 100%;
}

.stats-info {
  display: flex;
  align-items: center;
}

.stats-item {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
}

.stats-icon {
  font-size: 13px;
  color: #1890ff;
}

.stats-icon.online {
  color: #52c41a;
}

.stats-label {
  color: #8c8c8c;
  font-weight: 500;
  font-size: 11px;
}

.stats-value {
  font-weight: 600;
  font-size: 13px;
}

.stats-value.total {
  color: #1890ff;
}

.stats-value.online {
  color: #52c41a;
}

.stats-separator {
  color: #d9d9d9;
  margin: 0 1px;
}

.connection-status {
  display: flex;
  align-items: center;
}

.status-indicator {
  display: flex;
  align-items: center;
  gap: 6px;
}

.status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  transition: background-color 0.3s ease;
}

.status-text {
  font-size: 12px;
  font-weight: 500;
  color: #262626;
}

.connection-mode {
  color: #8c8c8c;
  font-size: 11px;
}

.status-controls {
  display: flex;
  align-items: center;
}

/* 响应式设计 */
@media (max-width: 768px) {
  .header-status {
    gap: 8px;
  }
  
  .stats-info {
    display: none; /* 在小屏幕上隐藏统计信息 */
  }
  
  .status-text {
    display: none; /* 在小屏幕上只显示状态点和按钮 */
  }
  
  .connection-mode {
    display: none;
  }
}

@media (max-width: 480px) {
  .header-status {
    gap: 4px;
  }
}
</style>