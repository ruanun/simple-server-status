<template>
    <a-popover trigger="click" placement="rightBottom">
        <template #content>
            <div style="line-height: 180%; min-width: 280px">
                <a-row>
                    <a-col :span="6">CPU</a-col>
                    <a-col :span="18">
                        <span>{{ item!.hostInfo.cpuInfo.toString() }}</span>
                    </a-col>
                </a-row>
                <a-row>
                    <a-col :span="6">RAM</a-col>
                    <a-col :span="18">
                    <span>{{
                        readableBytes(item!.hostInfo.RAMUsed) + ' / ' + readableBytes(item!.hostInfo.RAMTotal)
                        }}</span>
                    </a-col>
                </a-row>
                <a-row>
                    <a-col :span="6">Swap</a-col>
                    <a-col :span="18">
                        <span>{{
                            readableBytes(item!.hostInfo.swapUsed) + ' / ' + readableBytes(item!.hostInfo.swapTotal)
                            }}</span>
                    </a-col>
                </a-row>
                <a-row>
                    <a-col :span="6">Load</a-col>
                    <a-col :span="18">
                        <span>{{
                            item!.hostInfo.avgStat.load1.toFixed(2) + ' / ' + item!.hostInfo.avgStat.load5.toFixed(2) + ' / ' + item!.hostInfo.avgStat.load15.toFixed(2)
                            }}</span>
                    </a-col>
                </a-row>
                <a-row>
                    <a-col :span="6">DataTraffic</a-col>
                    <a-col :span="18">
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
                    <a-col :span="6">DiskInfo</a-col>
                    <a-col :span="18">
                        <span>
                            {{
                            readableBytes(item!.hostInfo.diskUsed) + ' / ' + readableBytes(item!.hostInfo.diskTotal)
                            }}
                        </span>
                    </a-col>
                </a-row>
                <a-row>
                    <a-col :span="6"></a-col>
                    <a-col :span="18" style="margin-top: 5px">
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
import {readableBytes} from "@/utils/CommonUtil";
import {ArrowDownOutlined, ArrowUpOutlined, MoreOutlined} from '@ant-design/icons-vue';
import type {DiskPartition, ServerInfo} from "@/api/models";

const columns = [
    {
        title: 'Mount',
        dataIndex: 'mountPoint',
        width: '20%',
    },
    {
        title: 'Used',
        dataIndex: 'used',
    },
    {
        title: 'Total',
        dataIndex: 'total',
    },
];
const props = defineProps<{
    item?: ServerInfo
}>();

</script>