<script lang="ts" setup>
import StatusPage from "@/pages/StatusPage.vue";

import {onMounted} from 'vue'

onMounted(() => {
  console.log(`App. the component is now mounted.`)
})

</script>

<template>
  <a-layout style="min-height: 100%">
    <a-layout-header style="background-color: white; box-shadow: 0 2px 8px #f0f1f2;padding-left: 0">
      <div style="padding-left: 10vw">
        <a href="./">
          <a-button type="link"
                    style="color: #000000d9; font-weight: bold; font-size: 18px; height: 100%;padding-left: 0">
            Simple{{ ' ' }}Server{{ ' ' }}Status
          </a-button>
        </a>
      </div>
    </a-layout-header>
    <a-layout-content style="padding: 10px 10vw;background-color: #f0f2f5">
      <status-page/>
    </a-layout-content>
    <a-layout-footer style="text-align: center;background-color: #f0f2f5">
      <a style="color: black;" href="https://github.com/ruanun/simple-server-status">Simple Server Status</a>
      &copy;{{ new Date().getFullYear() }} Created by <a style="color: black;" href="https://github.com/ruanun">Ruan</a>
    </a-layout-footer>
  </a-layout>
</template>

<style>

</style>
