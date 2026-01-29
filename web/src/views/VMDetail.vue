<template>
  <div class="vm-detail">
    <el-page-header @back="handleBack" :title="t('vmDetail.title')">
      <template #content>
        <span class="vm-title">VM{{ vmid }} - {{ vmName }}</span>
      </template>
    </el-page-header>

    <el-card shadow="hover" class="controls-card">
      <el-form :inline="true" class="controls-form">
        <el-form-item :label="t('vmDetail.period')">
          <el-select v-model="period" @change="loadData" style="width: 180px;">
            <el-option :label="t('vmDetail.minute')" value="minute" />
            <el-option :label="t('vmDetail.hour')" value="hour" />
            <el-option :label="t('vmDetail.day')" value="day" />
            <el-option :label="t('vmDetail.month')" value="month" />
          </el-select>
        </el-form-item>
        
        <el-form-item>
          <el-button type="primary" @click="loadData" :icon="Refresh">
            {{ t('vmDetail.update') }}
          </el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <el-card shadow="hover" class="chart-card">
      <template #header>
        <div class="card-header">
          <span>{{ t('vmDetail.trafficPattern') }}</span>
        </div>
      </template>
      <div ref="chartContainer" class="chart-container"></div>
    </el-card>

    <el-card shadow="hover" class="stats-card">
      <template #header>
        <div class="card-header">
          <span>{{ t('vmDetail.stats') }}</span>
        </div>
      </template>
      
      <el-table :data="statsData" stripe>
        <el-table-column prop="period" :label="t('vmDetail.period')" width="120">
          <template #default="{ row }">
            <span style="text-transform: capitalize;">{{ row.period }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="t('charts.download')">
          <template #default="{ row }">
            {{ formatBytes(row.rx_bytes || 0) }}
          </template>
        </el-table-column>
        <el-table-column :label="t('charts.upload')">
          <template #default="{ row }">
            {{ formatBytes(row.tx_bytes || 0) }}
          </template>
        </el-table-column>
        <el-table-column :label="t('dashboard.total')">
          <template #default="{ row }">
            <strong>{{ formatBytes(row.total_bytes || 0) }}</strong>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useThemeStore } from '@/stores/theme'
import { api } from '@/api'
import * as echarts from 'echarts'
import { createVMTimeSeriesChart } from '@/utils/chart'
import { formatBytes } from '@/utils/format'
import { Refresh } from '@element-plus/icons-vue'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const themeStore = useThemeStore()

const vmid = ref(route.params.id)
const vmName = ref('')
const period = ref('day')
const statsData = ref([])

const chartContainer = ref(null)
let chartInstance = null

const loadData = async () => {
  try {
    // 加载VM详情
    const vmRes = await api.getVM(vmid.value)
    if (vmRes.success && vmRes.data) {
      vmName.value = vmRes.data.vm.name
      
      // 转换统计数据为表格格式
      if (vmRes.data.stats) {
        statsData.value = Object.keys(vmRes.data.stats).map(key => ({
          period: key,
          ...vmRes.data.stats[key]
        }))
      }
    }
    
    // 加载历史数据
    const historyRes = await api.getHistory(vmid.value, { period: period.value })
    if (historyRes.success && historyRes.data) {
      renderChart(historyRes.data)
    }
  } catch (error) {
    console.error('Failed to load VM details:', error)
  }
}

const renderChart = (data) => {
  if (!chartInstance) {
    chartInstance = echarts.init(chartContainer.value)
  }
  
  if (!data || data.length === 0) {
    chartInstance.clear()
    return
  }
  
  const option = createVMTimeSeriesChart(data, themeStore.isDark, t)
  chartInstance.setOption(option)
}

const resizeChart = () => {
  chartInstance?.resize()
}

const handleBack = () => {
  router.back()
}

// 监听主题变化
watch(() => themeStore.isDark, () => {
  loadData()
})

onMounted(() => {
  loadData()
  
  // 监听手动刷新事件
  window.addEventListener('refresh-data', loadData)
  window.addEventListener('resize', resizeChart)
})

onUnmounted(() => {
  window.removeEventListener('refresh-data', loadData)
  window.removeEventListener('resize', resizeChart)
  chartInstance?.dispose()
})
</script>

<style scoped>
.vm-detail {
  width: 100%;
}

.vm-title {
  font-size: 18px;
  font-weight: 600;
}

.controls-card {
  margin: 20px 0;
}

.chart-card {
  margin-bottom: 20px;
}

.stats-card {
  margin-bottom: 20px;
}

.card-header {
  font-weight: 600;
}

.chart-container {
  width: 100%;
  height: 400px;
}

.controls-form {
  margin: 0;
}

.controls-form :deep(.el-form-item) {
  margin-bottom: 0;
}

:deep(.el-page-header) {
  margin-bottom: 20px;
}
</style>
