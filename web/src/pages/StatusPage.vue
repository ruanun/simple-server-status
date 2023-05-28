<template>
  <a-collapse :bordered="false" v-model:activeKey="groupNameList" style="background-color: #f0f1f2">
    <template #expandIcon="{ isActive }">
      <caret-right-outlined :rotate="isActive ? 90 : 0"/>
    </template>
    <a-collapse-panel :key="key" :header="key" v-for="(value,key,index) in serverGroup "
                      style="background: #f7f7f7;border-radius: 4px;margin-bottom: 2px;border: 0;">
      <a-list
          :grid="{ gutter: 16, xs: 1, sm: 1, md: 2, lg: 2, xl: 3, xxl: 4, xxxl: 5 }"
          :data-source="value"
      >
        <template #renderItem="{ item }">
          <a-list-item>
            <a-card :bordered="true" hoverable>
              <template #title>
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
import http from "@/api";
import ServerInfoContent from "@/components/ServerInfoContent.vue";
import ServerInfoExtra from "@/components/ServerInfoExtra.vue";
import type {ServerInfo} from "@/api/models";

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

function getServerStatusInfo() {
  http.get<Map<String, Array<ServerInfo>>>("/statusInfo")
      .then(value => {
        // console.log(value)
        const resp = value.data;
        serverGroup.value = resp

        // console.log(start)
        let keys = Object.keys(resp);
        if (start.size == 0) {
          //初次请求，初始化全部
          start = new Set<String>(keys)
          groupNameList.value = Array.from(keys)
        }
        for (let key of keys) {
          if (!start.has(key)) {
            //只放如第一次时不存在的
            groupNameList.value.push(key)
          }
        }

      })
}

onMounted(() => {

  getServerStatusInfo()

  setInterval(() => {
    getServerStatusInfo();
  }, 2 * 1000)
})

</script>
