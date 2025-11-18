import { createApp } from 'vue'
import App from './App.vue'
import 'ant-design-vue/dist/reset.css'
import i18n from './locales'
import { pinia } from './stores'

const app = createApp(App)

// 集成 Pinia 状态管理
app.use(pinia)

// 集成 i18n 国际化
app.use(i18n)

app.mount('#app')
