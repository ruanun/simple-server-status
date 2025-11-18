<template>
  <div style="margin: -10px; line-height: 180%">
    <!-- 操作系统 -->
    <a-row>
      <a-col :span="8">
        <desktop-outlined class="label-icon" />
        <span>{{ t('serverInfo.labels.system') }}</span>
      </a-col>
      <a-col :span="16">
        <a-typography-text
            style="width: 100%"
            :content="data?.platform"
            :ellipsis="{ tooltip: data?.platform }"
        />
      </a-col>
    </a-row>

    <!-- CPU 使用率 -->
    <a-row>
      <a-col :span="8">
        <thunderbolt-outlined class="label-icon" />
        <span>{{ t('serverInfo.labels.cpuUsage') }}</span>
      </a-col>
      <a-col :span="16">
        <a-progress
            style="margin-bottom: 0"
            :strokeColor="getPercentColor(data?.cpuPercent || 0)"
            :percent="formatPercent(data?.cpuPercent || 0)"
            :success="{parent:100,strokeColor:'red'}">
          <template #format="percent">
            <span style="color: black">{{ percent }}%</span>
          </template>
        </a-progress>
      </a-col>
    </a-row>

    <!-- 内存使用率 -->
    <a-row>
      <a-col :span="8">
        <database-outlined class="label-icon" />
        <span>{{ t('serverInfo.labels.memoryUsage') }}</span>
      </a-col>
      <a-col :span="16">
        <a-progress
            style="margin-bottom: 0"
            :strokeColor="getPercentColor(data?.RAMPercent || 0)"
            :percent="formatPercent(data?.RAMPercent || 0)"
            :success="{parent:100,strokeColor:'red'}">
          <template #format="percent">
            <span style="color: black">{{ percent }}%</span>
          </template>
        </a-progress>
      </a-col>
    </a-row>

    <!-- 交换空间 -->
    <a-row>
      <a-col :span="8">
        <swap-outlined class="label-icon" />
        <span>{{ t('serverInfo.labels.swapMemory') }}</span>
      </a-col>
      <a-col :span="16">
        <a-progress
            style="margin-bottom: 0"
            :strokeColor="getPercentColor(data?.SWAPPercent || 0)"
            :percent="formatPercent(data?.SWAPPercent || 0)"
            :success="{parent:100,strokeColor:'red'}">
          <template #format="percent">
            <span style="color: black">{{ percent }}%</span>
          </template>
        </a-progress>
      </a-col>
    </a-row>

    <!-- 网络速率 -->
    <a-row>
      <a-col :span="8">
        <cloud-outlined class="label-icon" />
        <span>{{ t('serverInfo.labels.networkSpeed') }}</span>
      </a-col>
      <a-col :span="16">
        <a-row>
          <a-col :span="12">
            <arrow-down-outlined/>
            <span>{{ readableBytes(data?.netInSpeed || 0) }}/s</span>
          </a-col>
          <a-col :span="12">
            <arrow-up-outlined/>
            <span>{{ readableBytes(data?.netOutSpeed || 0) }}/s</span>
          </a-col>
        </a-row>
      </a-col>
    </a-row>

    <!-- 运行时间 -->
    <a-row>
      <a-col :span="8">
        <clock-circle-outlined class="label-icon" />
        <span>{{ t('serverInfo.labels.uptime') }}</span>
      </a-col>
      <a-col :span="16">{{ formatUptime(data?.uptime || 0) }}</a-col>
    </a-row>

    <!-- 最后更新 -->
    <a-row>
      <a-col :span="8">
        <sync-outlined class="label-icon" />
        <span>{{ t('serverInfo.labels.lastUpdate') }}</span>
      </a-col>
      <a-col :span="16">{{ formatDate(data?.lastReportTime || 0) }}</a-col>
    </a-row>
  </div>
</template>
<script lang="ts" setup>
import {
  ArrowDownOutlined,
  ArrowUpOutlined,
  DesktopOutlined,
  ThunderboltOutlined,
  DatabaseOutlined,
  SwapOutlined,
  CloudOutlined,
  ClockCircleOutlined,
  SyncOutlined
} from '@ant-design/icons-vue'
import dayjs from "dayjs"
import type {ServerInfo} from "@/api/models"
import { useServerInfoFormatting } from "@/composables/useServerInfoFormatting"
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

// 使用统一的格式化工具
const { readableBytes, formatUptime, formatPercent, getPercentColor } = useServerInfoFormatting()

defineProps<{
  data?: ServerInfo
}>()

function formatDate(t: number) {
  return dayjs.unix(t).format("YYYY-MM-DD HH:mm:ss")
}
</script>

<style scoped>
.label-icon {
  margin-right: 6px;
  font-size: 14px;
}

/* 响应式设计 - 小屏幕图标调整 */
@media (max-width: 768px) {
  .label-icon {
    margin-right: 4px;
    font-size: 13px;
  }
}
</style>
