<script lang="ts" setup>
import StatusPage from "@/pages/StatusPage.vue";
import HeaderStatus from "@/components/HeaderStatus.vue";
import Logo from "@/components/Logo.vue";
import { onMounted } from 'vue'

onMounted(() => {
  if (import.meta.env.DEV) {
    console.log(`App. the component is now mounted.`)
  }
})

</script>

<template>
  <a-layout style="min-height: 100%">

    <!--  Header   -->
    <a-layout-header role="banner" aria-label="网站头部" class="app-header">
      <div class="header-container">
        <!-- 左侧标题 -->
        <div class="app-title">
          <a href="./">
            <Logo :size="32" class="logo-desktop" />
            <Logo :size="24" class="logo-mobile" />
            <h1 class="title-text">Simple Server Status</h1>
          </a>
        </div>

        <!-- 右侧状态组件 -->
        <div>
          <HeaderStatus />
        </div>
      </div>
    </a-layout-header>

    <!--  Content   -->
    <a-layout-content role="main" aria-label="主要内容" class="app-content">
      <status-page/>
    </a-layout-content>

    <!--  Footer   -->
    <a-layout-footer role="contentinfo" aria-label="网站页脚" class="app-footer">
      <div class="footer-content">
        <a
          href="https://github.com/ruanun/simple-server-status"
          target="_blank"
          rel="noopener noreferrer"
          class="footer-link"
        >
          Simple Server Status
        </a>
        <span class="footer-separator">|</span>
        <span class="footer-copyright">
          &copy;{{ new Date().getFullYear() }} Created by
          <a
            href="https://github.com/ruanun"
            target="_blank"
            rel="noopener noreferrer"
            class="footer-link"
          >
            Ruan
          </a>
        </span>
      </div>
    </a-layout-footer>
  </a-layout>
</template>

<style>
/* ===================================== */
/* 全局 CSS 变量系统 */
/* ===================================== */
:root {
  /* 颜色变量 */
  --app-bg-primary: #ffffff;
  --app-bg-secondary: #f0f2f5;
  --app-shadow-color: #f0f1f2;
  --app-text-primary: #000000d9;
  --app-text-secondary: rgba(0, 0, 0, 0.65);
  --app-text-tertiary: rgba(0, 0, 0, 0.85);
  --app-border-color: rgba(0, 0, 0, 0.25);
  --app-link-color: #1890ff;
  --app-link-hover: #40a9ff;

  /* 间距变量 */
  --app-padding-horizontal: 10vw;
  --app-padding-horizontal-mobile: 4vw;
  --app-padding-vertical: 10px;
  --app-gap-large: 12px;
  --app-gap-medium: 8px;
  --app-gap-small: 6px;

  /* 字体大小变量 */
  --app-font-title-desktop: 18px;
  --app-font-title-tablet: 15px;
  --app-font-title-mobile: 14px;
  --app-font-body: 14px;
  --app-font-small: 13px;
  --app-font-tiny: 12px;

  /* Footer 变量 */
  --app-footer-padding-desktop: 24px;
  --app-footer-padding-tablet: 20px;
  --app-footer-padding-mobile: 16px;
}

/* ===================================== */
/* 暗色模式支持 */
/* ===================================== */
@media (prefers-color-scheme: dark) {
  :root {
    --app-bg-primary: #1f1f1f;
    --app-bg-secondary: #141414;
    --app-shadow-color: rgba(0, 0, 0, 0.5);
    --app-text-primary: rgba(255, 255, 255, 0.85);
    --app-text-secondary: rgba(255, 255, 255, 0.65);
    --app-text-tertiary: rgba(255, 255, 255, 0.85);
    --app-border-color: rgba(255, 255, 255, 0.25);
    --app-link-color: #40a9ff;
    --app-link-hover: #69c0ff;
  }
}
</style>

<style scoped>
.app-header {
  background-color: var(--app-bg-primary);
  box-shadow: 0 2px 8px var(--app-shadow-color);
  padding: 0 var(--app-padding-horizontal);
}

.header-container {
  display: flex;
  justify-content: space-between;
  align-items: center;
  height: 100%;
}

.app-content {
  padding: var(--app-padding-vertical) var(--app-padding-horizontal);
  background-color: var(--app-bg-secondary);
}

.app-title a {
  display: flex;
  align-items: center;
  gap: var(--app-gap-large);
  text-decoration: none;
  color: var(--app-text-primary);
  font-weight: bold;
  font-size: var(--app-font-title-desktop);
  height: 100%;
}

.title-text {
  white-space: nowrap;
  margin: 0;
  font-size: var(--app-font-title-desktop);
  font-weight: bold;
}

/* 默认显示桌面端 Logo */
.logo-desktop {
  display: block;
}

.logo-mobile {
  display: none;
}

/* ===================================== */
/* Footer 样式 */
/* ===================================== */
.app-footer {
  text-align: center;
  background-color: var(--app-bg-secondary);
  padding: var(--app-footer-padding-desktop) var(--app-padding-horizontal);
  color: var(--app-text-secondary);
}

.footer-content {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  align-items: center;
  gap: var(--app-gap-large);
  line-height: 1.6;
  font-size: var(--app-font-body);
}

.footer-link {
  color: var(--app-text-tertiary);
  text-decoration: none;
  position: relative;
  transition: color 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.footer-link:hover {
  color: var(--app-link-hover);
}

/* 下划线动画 */
.footer-link::after {
  content: '';
  position: absolute;
  bottom: -2px;
  left: 0;
  width: 0;
  height: 1px;
  background-color: var(--app-link-hover);
  transition: width 0.3s ease;
}

.footer-link:hover::after {
  width: 100%;
}

.footer-separator {
  color: var(--app-border-color);
  margin: 0 4px;
  user-select: none;
}

.footer-copyright {
  font-size: var(--app-font-body);
}

/* ===================================== */
/* 响应式设计 - 统一媒体查询 */
/* ===================================== */

/* 平板/小屏幕 (≤768px) */
@media (max-width: 768px) {
  /* Header & Content */
  .app-header,
  .app-content {
    padding-left: var(--app-padding-horizontal-mobile);
    padding-right: var(--app-padding-horizontal-mobile);
  }

  .app-title a {
    font-size: var(--app-font-title-tablet);
    gap: var(--app-gap-medium);
  }

  .title-text {
    font-size: var(--app-font-title-tablet);
  }

  /* Logo 切换 */
  .logo-desktop {
    display: none;
  }

  .logo-mobile {
    display: block;
  }

  /* Footer */
  .app-footer {
    padding: var(--app-footer-padding-tablet) var(--app-padding-horizontal-mobile);
    font-size: var(--app-font-small);
  }

  .footer-content {
    gap: var(--app-gap-medium);
    line-height: 1.8;
    font-size: var(--app-font-small);
  }

  .footer-separator {
    display: none;
  }

  .footer-copyright {
    font-size: var(--app-font-small);
  }
}

/* 手机 (≤480px) */
@media (max-width: 480px) {
  /* Header */
  .app-title a {
    font-size: var(--app-font-title-mobile);
    gap: var(--app-gap-small);
  }

  .title-text {
    font-size: var(--app-font-title-mobile);
  }

  /* Footer */
  .app-footer {
    padding: var(--app-footer-padding-mobile) var(--app-padding-horizontal-mobile);
    font-size: var(--app-font-tiny);
  }

  .footer-content {
    flex-direction: column;
    gap: var(--app-gap-small);
    font-size: var(--app-font-tiny);
  }

  .footer-copyright {
    font-size: var(--app-font-tiny);
  }
}
</style>
