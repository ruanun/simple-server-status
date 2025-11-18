<!--
  Server Status Logo Component
  @author ruan
-->
<template>
  <svg
    :width="size"
    :height="size"
    viewBox="0 0 32 32"
    fill="none"
    xmlns="http://www.w3.org/2000/svg"
    class="logo"
  >
    <!-- 服务器机架背景 -->
    <rect
      x="6"
      y="4"
      width="20"
      height="24"
      rx="2"
      fill="#1890ff"
      opacity="0.08"
    />

    <!-- 服务器机架轮廓 -->
    <rect
      x="6"
      y="4"
      width="20"
      height="24"
      rx="2"
      fill="none"
      stroke="#1890ff"
      stroke-width="2"
    />

    <!-- 服务器分层线 -->
    <line
      x1="8"
      y1="11"
      x2="24"
      y2="11"
      stroke="#1890ff"
      stroke-width="1.5"
      opacity="0.6"
    />
    <line
      x1="8"
      y1="18"
      x2="24"
      y2="18"
      stroke="#1890ff"
      stroke-width="1.5"
      opacity="0.6"
    />

    <!-- 服务器状态指示灯 - 第一层（在线） -->
    <circle cx="10" cy="7.5" r="1.5" fill="#52c41a" />
    <circle cx="10" cy="7.5" r="1.5" fill="#52c41a" opacity="0.3" />

    <!-- 服务器状态指示灯 - 第二层（在线） -->
    <circle cx="10" cy="14.5" r="1.5" fill="#52c41a" />

    <!-- 服务器状态指示灯 - 第三层（运行中） -->
    <circle cx="10" cy="21.5" r="1.5" fill="#1890ff" />

    <!-- 右侧指示器 -->
    <rect x="20" y="7" width="2" height="1" rx="0.5" fill="#52c41a" opacity="0.6"/>
    <rect x="20" y="14" width="2" height="1" rx="0.5" fill="#52c41a" opacity="0.6"/>
    <rect x="20" y="21" width="2" height="1" rx="0.5" fill="#1890ff" opacity="0.6"/>
  </svg>
</template>

<script setup lang="ts">
/**
 * Logo 组件属性
 */
defineProps<{
  /** Logo 尺寸，默认 32 */
  size?: number | string
}>()
</script>

<script lang="ts">
export default {
  name: 'Logo'
}
</script>

<style scoped>
.logo {
  display: inline-block;
  vertical-align: middle;
  flex-shrink: 0;
}
</style>
