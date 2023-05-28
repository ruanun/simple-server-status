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
            <div>
                <a href="./">
                    <a-button type="link"
                              style="color: #000000d9; font-weight: bold; font-size: 18px;padding-left: 10vw">
                        Simple{{ ' ' }}Server{{ ' ' }}Status
                    </a-button>
                </a>
            </div>
        </a-layout-header>
        <a-layout-content style="padding: 10px 10vw">
            <status-page/>
        </a-layout-content>
        <a-layout-footer style="text-align: center">
            <a style="color: black;" href="https://github.com/ruanun/simple-server-status">Simple Server Status</a>
            ©2023 Created by <a style="color: black;" href="https://github.com/ruanun">Ruan</a>
        </a-layout-footer>
    </a-layout>
</template>

<style>

</style>
