<template>
  <div class="status-page">
    <!-- 服务器状态面板 -->
    <a-collapse :bordered="false" v-model:activeKey="activeKeys" style="background-color: transparent">
      <template #expandIcon="{ isActive }">
        <caret-right-outlined :rotate="isActive ? 90 : 0"/>
      </template>
      <a-collapse-panel :key="groupName" :header="groupName + ' (' + servers.length + ')' " v-for="([groupName, servers]) in groupedServers "
                        class="collapse-panel-style-a">
        <a-list
            :grid="{ gutter: 16, xs: 1, sm: 1, md: 2, lg: 2, xl: 3, xxl: 4, xxxl: 5 }"
            :data-source="servers"
        >
          <template #renderItem="{ item }">
            <a-list-item style="padding: 0 0 ;">
              <a-card :bordered="true" hoverable style="border-radius: 7px;background-color: #ffffff">
                <template #title>
                  <FlagIcon v-if="item.loc" :countryCode="item.loc"/>
                  {{ item.name }}
                  <StatusIndicator
                    :is-online="item.isOnline"
                    online-text="Online"
                    offline-text="Offline"
                    variant="full"
                  />
                </template>
                <template #extra>
                  <ServerInfoExtra :item="item"/>
                </template>
                <ServerInfoContent :data="item"/>
              </a-card>
            </a-list-item>
          </template>
        </a-list>
      </a-collapse-panel>
    </a-collapse>
  </div>

</template>

<script lang="ts" setup>
import {CaretRightOutlined} from '@ant-design/icons-vue';
import {onMounted, ref, watch} from 'vue';
import ServerInfoContent from "@/components/ServerInfoContent.vue";
import ServerInfoExtra from "@/components/ServerInfoExtra.vue";
import FlagIcon from "@/components/FlagIcon.vue";
import StatusIndicator from "@/components/StatusIndicator.vue";
import { useConnectionStore } from '@/stores/connection';
import { useServerStore } from '@/stores/server';
import { storeToRefs } from 'pinia';
import { useConnectionManager } from '@/composables/useConnectionManager';
import { CONNECTION_MODES } from '@/constants/connectionModes';

// Stores
const connectionStore = useConnectionStore()
const serverStore = useServerStore()
const { groupedServers, groupNames } = storeToRefs(serverStore)

// 本地可写状态：折叠面板的展开状态
const activeKeys = ref<string[]>([])

// 智能监听 groupNames 变化，只处理真正的增删，保留用户状态
watch(groupNames, (newGroupNames, oldGroupNames) => {
  if (!oldGroupNames || oldGroupNames.length === 0) {
    // 首次初始化：展开所有分组
    activeKeys.value = [...newGroupNames]
    return
  }

  // 检测新增的分组
  const addedGroups = newGroupNames.filter(group => !oldGroupNames.includes(group))

  // 检测删除的分组
  const removedGroups = oldGroupNames.filter(group => !newGroupNames.includes(group))

  // 只在有真正变化时更新
  if (addedGroups.length > 0 || removedGroups.length > 0) {
    // 移除已消失的分组
    activeKeys.value = activeKeys.value.filter(key => !removedGroups.includes(key))

    // 添加新分组（默认展开）
    activeKeys.value = [...activeKeys.value, ...addedGroups]
  }
  // 如果分组内容没变化，不更新 activeKeys，保留用户操作
}, { immediate: true })

// 连接管理
const { initializeConnection, switchToHTTP, switchToWebSocket } = useConnectionManager()

// 监听模式变化（响应 HeaderStatus 的切换操作）
watch(() => connectionStore.mode, async (newMode, oldMode) => {
  // 避免初始化时触发
  if (!oldMode) return

  if (newMode === CONNECTION_MODES.HTTP && oldMode === CONNECTION_MODES.WEBSOCKET) {
    switchToHTTP()
  } else if (newMode === CONNECTION_MODES.WEBSOCKET && oldMode === CONNECTION_MODES.HTTP) {
    await switchToWebSocket()
  }
})

// 生命周期
onMounted(() => initializeConnection())

</script>

<style scoped>
.status-page {
  padding: 0;
}

/* ===================================== */
/* 方案 A：现代白卡片风格 */
/* ===================================== */
.collapse-panel-style-a {
  background: #ffffff;
  border-radius: 8px !important;
  margin-bottom: 12px;
  border: 0;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
  overflow: hidden;
}

/* Panel 头部样式 - 保留上圆角 */
.collapse-panel-style-a :deep(.ant-collapse-header) {
  background-color: #ffffff !important;
  border-bottom: 1px solid #f0f0f0;
  padding: 12px 16px;
  border-radius: 8px 8px 0 0 !important;
}

/* Panel 内容区域 - 保留下圆角 */
.collapse-panel-style-a :deep(.ant-collapse-content) {
  background-color: #fafafa !important;
  border-radius: 0 0 8px 8px !important;
}

.collapse-panel-style-a :deep(.ant-collapse-content-box) {
  padding: 16px;
}

/* ===================================== */
/* 方案 C：极简全白风格 */
/* ===================================== */
.collapse-panel-style-c {
  background: #ffffff;
  border-radius: 8px !important;
  margin-bottom: 12px;
  border: 0;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.08);
  overflow: hidden;
}

/* Panel 头部样式 - 保留上圆角 */
.collapse-panel-style-c :deep(.ant-collapse-header) {
  background-color: #ffffff !important;
  border-bottom: 1px solid #e8e8e8;
  padding: 12px 16px;
  border-radius: 8px 8px 0 0 !important;
}

/* Panel 内容区域 - 保留下圆角 */
.collapse-panel-style-c :deep(.ant-collapse-content) {
  background-color: #ffffff !important;
  border-radius: 0 0 8px 8px !important;
}

.collapse-panel-style-c :deep(.ant-collapse-content-box) {
  padding: 16px;
}

/* Card 添加边框（仅方案C） */
.collapse-panel-style-c :deep(.ant-card) {
  border: 1px solid #f0f0f0 !important;
  box-shadow: none !important;
}
</style>
