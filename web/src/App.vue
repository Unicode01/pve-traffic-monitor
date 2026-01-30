<template>
  <el-config-provider :locale="locale">
    <!-- Token 输入对话框 -->
    <el-dialog
      v-model="showTokenDialog"
      :title="t('auth.tokenRequired')"
      width="400px"
      :close-on-click-modal="false"
      :close-on-press-escape="false"
      :show-close="tokenConfigured"
    >
      <el-form @submit.prevent="handleTokenSubmit">
        <el-form-item :label="t('auth.apiToken')">
          <el-input
            v-model="tokenInput"
            :placeholder="t('auth.enterToken')"
            type="password"
            show-password
          />
        </el-form-item>
        <el-form-item>
          <el-text type="info" size="small">{{ t('auth.tokenHint') }}</el-text>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button v-if="tokenConfigured" @click="showTokenDialog = false">
          {{ t('common.cancel') }}
        </el-button>
        <el-button type="primary" @click="handleTokenSubmit">
          {{ t('auth.confirm') }}
        </el-button>
      </template>
    </el-dialog>

    <router-view />
  </el-config-provider>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import zhCn from 'element-plus/dist/locale/zh-cn.mjs'
import en from 'element-plus/dist/locale/en.mjs'
import { ElMessage } from 'element-plus'
import { getApiToken, setApiToken } from '@/api'

const { locale: i18nLocale, t } = useI18n()

const locale = computed(() => {
  return i18nLocale.value === 'zh-CN' ? zhCn : en
})

const showTokenDialog = ref(false)
const tokenInput = ref('')
const tokenConfigured = ref(false)

// 检查是否已有 token
onMounted(() => {
  const existingToken = getApiToken()
  tokenConfigured.value = !!existingToken

  // 监听未授权事件
  window.addEventListener('api-unauthorized', handleUnauthorized)

  // 监听显示 token 设置对话框事件
  window.addEventListener('show-token-dialog', handleShowTokenDialog)
})

onUnmounted(() => {
  window.removeEventListener('api-unauthorized', handleUnauthorized)
  window.removeEventListener('show-token-dialog', handleShowTokenDialog)
})

const handleUnauthorized = () => {
  showTokenDialog.value = true
}

const handleShowTokenDialog = () => {
  tokenInput.value = getApiToken()
  showTokenDialog.value = true
}

const handleTokenSubmit = () => {
  setApiToken(tokenInput.value)
  tokenConfigured.value = !!tokenInput.value
  showTokenDialog.value = false

  if (tokenInput.value) {
    ElMessage.success(t('auth.tokenSaved'))
    // 刷新数据
    window.dispatchEvent(new Event('refresh-data'))
  } else {
    ElMessage.info(t('auth.tokenCleared'))
  }
}
</script>

<style>
* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

body {
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
}

#app {
  width: 100vw;
  height: 100vh;
}
</style>
