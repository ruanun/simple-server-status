<template>
  <div style="margin: -10px; line-height: 180%">
    <a-row>
      <a-col :span="8">System</a-col>
      <a-col :span="16">
        <a-typography-text
            style="width: 100%"
            :content="data?.platform"
            :ellipsis="{ tooltip: data?.platform }"
        />
      </a-col>
    </a-row>
    <a-row>
      <a-col :span="8">CPU</a-col>
      <a-col :span="16">
        <a-progress :strokeColor="getPercentColor(data!.cpuPercent)"
                    :percent="Number.parseFloat(data!.cpuPercent.toFixed())"
                    :success="{parent:100,strokeColor:'red'}">
          <template #format="percent">
            <span style="color: black">{{ percent }}%</span>
          </template>
        </a-progress>
      </a-col>
    </a-row>
    <a-row>
      <a-col :span="8">RAM</a-col>
      <a-col :span="16">
        <a-progress :strokeColor="getPercentColor(data!.RAMPercent)"
                    :percent="Number.parseFloat(data!.RAMPercent.toFixed())"
                    :success="{parent:100,strokeColor:'red'}">
          <template #format="percent">
            <span style="color: black">{{ percent }}%</span>
          </template>
        </a-progress>
      </a-col>
    </a-row>
    <a-row>
      <a-col :span="8">Swap</a-col>
      <a-col :span="16">
        <a-progress :strokeColor="getPercentColor(data!.SWAPPercent)"
                    :percent="Number.parseFloat(data!.SWAPPercent.toFixed())"
                    :success="{parent:100,strokeColor:'red'}"
        >
          <template #format="percent">
            <span style="color: black">{{ percent }}%</span>
          </template>
        </a-progress>
      </a-col>
    </a-row>
    <a-row>
      <a-col :span="8">Network</a-col>
      <a-col :span="16">
        <a-row>
          <a-col :span="12">
            <arrow-down-outlined/>
            <span>{{ readableBytes(data!.netInSpeed) }}</span>
          </a-col>
          <a-col :span="12">
            <arrow-up-outlined/>
            <span>{{ readableBytes(data!.netOutSpeed) }}</span>
          </a-col>
        </a-row>
      </a-col>
    </a-row>
    <a-row>
      <a-col :span="8">Uptime</a-col>
      <a-col :span="16">{{ getDays(data!.uptime) }}</a-col>
    </a-row>
    <a-row>
      <a-col :span="8">ReportTime</a-col>
      <a-col :span="16">{{ formatDate(data!.lastReportTime) }}</a-col>
    </a-row>
  </div>
</template>
<script lang="ts" setup>
import {ArrowDownOutlined, ArrowUpOutlined} from '@ant-design/icons-vue';
import dayjs from "dayjs";
import type {ServerInfo} from "@/api/models";
import {readableBytes} from "@/utils/CommonUtil";

const props = defineProps<{
  data?: ServerInfo
}>();

function formatDate(t: number) {
  return dayjs.unix(t).format("YYYY-MM-DD HH:mm:ss")
}

function getPercentColor(percent: number) {
  if (percent > 90) {
    return "red"
  }
  if (percent > 70) {
    return "#faad14"
  }
  return ""
}

function getDays(uptime: number) {
  return `${(uptime / (60 * 60 * 24)).toFixed()} days`
}

</script>