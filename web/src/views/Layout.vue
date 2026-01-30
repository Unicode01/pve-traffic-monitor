<template>
  <el-container class="layout">
    <el-header class="header">
      <div class="header-left">
        <h1>{{ t('app.title') }}</h1>
      </div>
      
      <div class="header-center">
        <el-menu
          :default-active="activeMenu"
          mode="horizontal"
          @select="handleMenuSelect"
          :ellipsis="false"
        >
          <el-menu-item index="/dashboard">
            <el-icon><HomeFilled /></el-icon>
            <span>{{ t('nav.dashboard') }}</span>
          </el-menu-item>
          <el-menu-item index="/charts">
            <el-icon><TrendCharts /></el-icon>
            <span>{{ t('nav.charts') }}</span>
          </el-menu-item>
        </el-menu>
      </div>

      <div class="header-right">
        <el-button @click="handleRefresh" :icon="Refresh" circle />

        <el-button @click="handleTokenSetting" :icon="Key" circle :title="t('auth.setToken')" />

        <el-dropdown @command="handleLangChange">
          <el-button :icon="Promotion" circle />
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="zh-CN">简体中文</el-dropdown-item>
              <el-dropdown-item command="en-US">English</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>

        <el-switch
          v-model="isDark"
          :active-action-icon="Moon"
          :inactive-action-icon="Sunny"
          @change="themeStore.toggleTheme"
          inline-prompt
          style="--el-switch-on-color: #2c3e50; --el-switch-off-color: #409eff"
        />
      </div>
    </el-header>

    <el-main class="main">
      <router-view />
    </el-main>
  </el-container>
</template>

<script setup>
import { computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useThemeStore } from '@/stores/theme'
import { HomeFilled, TrendCharts, Refresh, Promotion, Moon, Sunny, Key } from '@element-plus/icons-vue'

const router = useRouter()
const route = useRoute()
const { t, locale } = useI18n()
const themeStore = useThemeStore()

const isDark = computed({
  get: () => themeStore.isDark,
  set: () => {}
})

const activeMenu = computed(() => route.path)

const handleMenuSelect = (index) => {
  router.push(index)
}

const handleRefresh = () => {
  // 触发数据刷新事件
  window.dispatchEvent(new Event('refresh-data'))
}

const handleTokenSetting = () => {
  // 触发显示 token 设置对话框事件
  window.dispatchEvent(new Event('show-token-dialog'))
}

const handleLangChange = (lang) => {
  locale.value = lang
  localStorage.setItem('locale', lang)
}
</script>

<style scoped>
.layout {
  width: 100%;
  height: 100vh;
}

.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 20px;
  background: var(--el-bg-color);
  border-bottom: 1px solid var(--el-border-color);
}

.header-left h1 {
  font-size: 20px;
  font-weight: 600;
  color: var(--el-text-color-primary);
  margin: 0;
}

.header-center {
  flex: 1;
  display: flex;
  justify-content: center;
}

.header-right {
  display: flex;
  align-items: center;
  gap: 12px;
}

.main {
  background: var(--el-bg-color-page);
  padding: 20px;
  overflow-y: auto;
}

:deep(.el-menu--horizontal) {
  border-bottom: none;
}
</style>
