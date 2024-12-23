<template>
  <a-collapse :bordered="false" v-model:activeKey="groupNameList" style="background-color: #f0f1f2">
    <template #expandIcon="{ isActive }">
      <caret-right-outlined :rotate="isActive ? 90 : 0"/>
    </template>
    <a-collapse-panel :key="value[0]" :header="value[0] + ' (' + value[1].length + ')'" v-for="(value) in serverGroup "
                      style="background: #f7f7f7;border-radius: 6px;margin-bottom: 2px;border: 0;">
      <a-list
          :grid="{ gutter: 16, xs: 1, sm: 1, md: 2, lg: 2, xl: 3, xxl: 4, xxxl: 5 }"
          :data-source="value[1]"
      >
        <template #renderItem="{ item }">
          <a-list-item style="padding: 0 0 ;">
            <a-card :bordered="true" hoverable style="border-radius: 7px;background-color: #ffffff">
              <template #title>
                <FlagIcon v-if="item.loc" :countryCode="item.loc"/>
                {{ item.name }}
                <a-tag :color="getOnlineColor(item)">
                  {{ item.isOnline ? 'online' : 'offline' }}
                </a-tag>
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

</template>

<script lang="ts" setup>
import {CaretRightOutlined} from '@ant-design/icons-vue';
import {onMounted, ref} from 'vue';
import http from "../api";
import ServerInfoContent from "@/components/ServerInfoContent.vue";
import ServerInfoExtra from "@/components/ServerInfoExtra.vue";
import type {ServerInfo} from "@/api/models";
import FlagIcon from "@/components/FlagIcon.vue";

function getOnlineColor(item: ServerInfo) {
  if (item.isOnline) {
    return "success"
  }
  return "error"
}

const groupNameList = ref<Array<String>>([])
const serverGroup = ref<Map<String, Array<ServerInfo>>>()
//初始时的group
let start = new Set<String>();

async function getServerStatusInfo() {
  const resultData = await http.get<Array<ServerInfo>>("/v2/server/statusInfo")
  const map = new Map<String, Array<ServerInfo>>()
  resultData.data.forEach(item => {
    if (!start.has(item.group)) {
      groupNameList.value.push(item.group)
      start.add(item.group)
    }
    if (!map.has(item.group)) {
      map.set(item.group, [])
    }
    map.get(item.group)?.push(item)
  })
  serverGroup.value = map
}

onMounted(() => {
  getServerStatusInfo()
  setInterval(() => {
    getServerStatusInfo();
  }, 2 * 1000)
})

</script>
