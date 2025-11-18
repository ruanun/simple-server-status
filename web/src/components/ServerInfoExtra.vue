<template>
    <a-popover trigger="click" placement="rightBottom">
        <template #content>
            <div style="line-height: 180%; min-width: 340px">
                <a-row>
                    <a-col :span="7" class="label-col">
                        <thunderbolt-outlined class="label-icon" />
                        <span>{{ t('serverInfo.labels.cpuInfo') }}</span>
                    </a-col>
                    <a-col :span="17">
                        <span>{{ item!.hostInfo.cpuInfo.toString() }}</span>
                    </a-col>
                </a-row>
                <a-row>
                    <a-col :span="7" class="label-col">
                        <database-outlined class="label-icon" />
                        <span>{{ t('serverInfo.labels.memoryDetails') }}</span>
                    </a-col>
                    <a-col :span="17">
                    <span>{{
                        readableBytes(item!.hostInfo.RAMUsed) + ' / ' + readableBytes(item!.hostInfo.RAMTotal)
                        }}</span>
                    </a-col>
                </a-row>
                <a-row>
                    <a-col :span="7" class="label-col">
                        <swap-outlined class="label-icon" />
                        <span>{{ t('serverInfo.labels.swapMemory') }}</span>
                    </a-col>
                    <a-col :span="17">
                        <span>{{
                            readableBytes(item!.hostInfo.swapUsed) + ' / ' + readableBytes(item!.hostInfo.swapTotal)
                            }}</span>
                    </a-col>
                </a-row>
                <a-row>
                    <a-col :span="7" class="label-col">
                        <dashboard-outlined class="label-icon" />
                        <span>{{ t('serverInfo.labels.systemLoad') }}</span>
                    </a-col>
                    <a-col :span="17">
                        <span>{{
                            formatLoad(
                                item!.hostInfo.avgStat.load1,
                                item!.hostInfo.avgStat.load5,
                                item!.hostInfo.avgStat.load15
                            )
                            }}</span>
                    </a-col>
                </a-row>
                <a-row>
                    <a-col :span="7" class="label-col">
                        <cloud-outlined class="label-icon" />
                        <span>{{ t('serverInfo.labels.totalTraffic') }}</span>
                    </a-col>
                    <a-col :span="17">
                        <a-row>
                            <a-col :span="12">
                                <arrow-down-outlined/>
                                <span>{{
                                    readableBytes(item!.hostInfo.netInTransfer)
                                    }}</span>
                            </a-col>
                            <a-col :span="12">
                                <arrow-up-outlined/>
                                <span>{{
                                    readableBytes(item!.hostInfo.netOutTransfer)
                                    }}</span>
                            </a-col>
                        </a-row>
                    </a-col>
                </a-row>
                <a-row>
                    <a-col :span="7" class="label-col">
                        <hdd-outlined class="label-icon" />
                        <span>{{ t('serverInfo.labels.diskUsage') }}</span>
                    </a-col>
                    <a-col :span="17">
                        <span>
                            {{
                            readableBytes(item!.hostInfo.diskUsed) + ' / ' + readableBytes(item!.hostInfo.diskTotal)
                            }}</span>
                    </a-col>
                </a-row>
                <a-row>
                    <a-col :span="7" class="label-col"></a-col>
                    <a-col :span="17" style="margin-top: 5px">
                        <a-table
                                :columns="columns"
                                :row-key="(record:DiskPartition) => record.mountPoint"
                                :data-source="item!.hostInfo.diskPartitions"
                                :pagination="false"
                                size="small"
                        >
                            <template #bodyCell="{ column, record }">
                                <template v-if="column.dataIndex === 'used'">
                                    {{ readableBytes(record.used) }}
                                </template>
                                <template v-else-if="column.dataIndex === 'total'">
                                    {{ readableBytes(record.total) }}
                                </template>
                            </template>
                        </a-table>
                    </a-col>
                </a-row>
            </div>
        </template>
        <more-outlined/>
    </a-popover>
</template>
<script lang="ts" setup>
import {
  ArrowDownOutlined,
  ArrowUpOutlined,
  MoreOutlined,
  ThunderboltOutlined,
  DatabaseOutlined,
  SwapOutlined,
  DashboardOutlined,
  CloudOutlined,
  HddOutlined
} from '@ant-design/icons-vue'
import type {DiskPartition, ServerInfo} from "@/api/models"
import { useServerInfoFormatting } from "@/composables/useServerInfoFormatting"
import { useI18n } from 'vue-i18n'
import { computed } from 'vue'

const { t } = useI18n()

// 使用统一的格式化工具
const { readableBytes, formatLoad } = useServerInfoFormatting()

// 使用 computed 使表格列名支持 i18n
const columns = computed(() => [
    {
        title: t('serverInfo.table.mountPoint'),
        dataIndex: 'mountPoint',
        width: '30%',
    },
    {
        title: t('serverInfo.table.used'),
        dataIndex: 'used',
    },
    {
        title: t('serverInfo.table.total'),
        dataIndex: 'total',
    },
])

defineProps<{
  item?: ServerInfo
}>()
</script>

<style scoped>
/* 标签列样式优化 */
.label-col {
  white-space: nowrap;    /* 防止换行 */
  /*padding-right: 8px;*/     /* 增加与数据列的间距 */
  display: flex;
  align-items: center;
}

.label-icon {
  margin-right: 4px;
  font-size: 13px;
  flex-shrink: 0;         /* 图标不缩小 */
}

/* 响应式设计 - 小屏幕调整 */
@media (max-width: 768px) {
  .label-icon {
    margin-right: 3px;
    font-size: 12px;
  }

  .label-col {
    padding-right: 6px;
  }
}
</style>
