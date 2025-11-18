<!--
  状态指示器组件

  职责：
  - 显示在线/离线状态
  - 支持不同的样式变体（简单/完整）
  - 提供动画效果（脉冲）

  使用方式：
  ```vue
  <StatusIndicator
    :is-online="true"
    online-text="在线"
    offline-text="离线"
    variant="full"
  />
  ```

  @author ruan
-->
<template>
  <div class="status-indicator" :class="[variantClass]">
    <div
      class="status-dot"
      :class="statusClass"
      :style="customColor ? { backgroundColor: customColor } : {}"
    ></div>
    <span v-if="showText" class="status-text" :class="statusClass">
      {{ displayText }}
    </span>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

/**
 * 组件属性
 */
interface Props {
  /** 是否在线 */
  isOnline: boolean
  /** 在线状态文本 */
  onlineText?: string
  /** 离线状态文本 */
  offlineText?: string
  /** 是否显示文本 */
  showText?: boolean
  /** 自定义颜色（覆盖默认颜色） */
  customColor?: string
  /**
   * 变体样式
   * - simple: 简单样式（小点，无动画，用于 Header）
   * - full: 完整样式（大点，有动画，用于卡片）
   */
  variant?: 'simple' | 'full'
}

const props = withDefaults(defineProps<Props>(), {
  onlineText: 'Online',
  offlineText: 'Offline',
  showText: true,
  variant: 'full'
})

// 状态类名
const statusClass = computed(() => props.isOnline ? 'online' : 'offline')

// 变体类名
const variantClass = computed(() => `variant-${props.variant}`)

// 显示文本
const displayText = computed(() =>
  props.isOnline ? props.onlineText : props.offlineText
)
</script>

<style scoped>
/* ==================== 基础样式 ==================== */
.status-indicator {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  transition: all 0.3s ease;
}

.status-dot {
  border-radius: 50%;
  position: relative;
  transition: all 0.3s ease;
}

.status-text {
  font-size: 12px;
  font-weight: 500;
}

/* ==================== Simple 变体（用于 Header）==================== */
.variant-simple .status-dot {
  width: 6px;
  height: 6px;
}

.variant-simple .status-text {
  color: #262626;
}

/* ==================== Full 变体（用于卡片）==================== */
.variant-full {
  padding: 4px 8px;
  border-radius: 12px;
}

.variant-full .status-dot {
  width: 8px;
  height: 8px;
}

/* 在线状态样式 */
.variant-full .status-dot.online {
  background-color: #52c41a;
  box-shadow: 0 0 0 2px rgba(82, 196, 26, 0.2);
}

/* 在线状态脉冲动画 */
.variant-full .status-dot.online::before {
  content: '';
  position: absolute;
  top: -2px;
  left: -2px;
  right: -2px;
  bottom: -2px;
  border-radius: 50%;
  background-color: rgba(82, 196, 26, 0.3);
  animation: pulse 2s infinite;
}

/* 离线状态样式 */
.variant-full .status-dot.offline {
  background-color: #ff4d4f;
  box-shadow: 0 0 0 2px rgba(255, 77, 79, 0.2);
}

/* 文本颜色 */
.variant-full .status-text.online {
  color: #52c41a;
}

.variant-full .status-text.offline {
  color: #ff4d4f;
}

/* ==================== 动画 ==================== */
@keyframes pulse {
  0% {
    transform: scale(1);
    opacity: 1;
  }
  50% {
    transform: scale(1.2);
    opacity: 0.7;
  }
  100% {
    transform: scale(1);
    opacity: 1;
  }
}

/* ==================== 响应式 ==================== */
@media (max-width: 768px) {
  .variant-simple .status-text {
    display: none; /* 在小屏幕上隐藏文本 */
  }
}
</style>
