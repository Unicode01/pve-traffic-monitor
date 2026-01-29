<template>
  <div class="charts">
    <el-card shadow="hover" class="controls-card">
      <el-form :inline="true" class="controls-form">
        <el-form-item :label="t('charts.period')">
          <el-select v-model="period" @change="loadChartData" style="width: 150px;">
            <el-option :label="t('charts.currentMinute')" value="minute" />
            <el-option :label="t('charts.currentHour')" value="hour" />
            <el-option :label="t('charts.today')" value="day" />
            <el-option :label="t('charts.currentMonth')" value="month" />
          </el-select>
        </el-form-item>
        
        <el-form-item :label="t('charts.direction')">
          <el-select v-model="direction" @change="loadChartData" style="width: 120px;">
            <el-option :label="t('charts.both')" value="both" />
            <el-option :label="t('charts.download')" value="rx" />
            <el-option :label="t('charts.upload')" value="tx" />
          </el-select>
        </el-form-item>
        
        <el-form-item>
          <el-button type="primary" @click="loadChartData" :icon="Refresh">
            {{ t('charts.update') }}
          </el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <el-row :gutter="20">
      <el-col :span="24">
        <el-card shadow="hover" class="chart-card">
          <template #header>
            <div class="card-header">
              <span>{{ t('charts.trafficOverview') }}</span>
            </div>
          </template>
          <div ref="overviewChart" class="chart-container"></div>
        </el-card>
      </el-col>
      
      <el-col :span="24">
        <el-card shadow="hover" class="chart-card">
          <template #header>
            <div class="card-header">
              <span>{{ t('charts.topVMs') }}</span>
            </div>
          </template>
          <div ref="topVMsChart" class="chart-container"></div>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useThemeStore } from '@/stores/theme'
import { api } from '@/api'
import * as echarts from 'echarts'
import { createTrafficLineChart, createTopVMsBarChart } from '@/utils/chart'
import { Refresh } from '@element-plus/icons-vue'

const { t } = useI18n()
const themeStore = useThemeStore()

const period = ref('day')
const direction = ref('both')

const overviewChart = ref(null)
const topVMsChart = ref(null)

let overviewChartInstance = null
let topVMsChartInstance = null

const loadChartData = async () => {
  try {
    console.log('Loading chart data with:', { period: period.value, direction: direction.value })
    const res = await api.getStats({ period: period.value, direction: direction.value })
    
    console.log('Stats API response:', res)
    
    if (res.success && res.data && res.data.length > 0) {
      renderOverviewChart(res.data)
      renderTopVMsChart(res.data)
    } else {
      console.warn('No data received from API')
    }
  } catch (error) {
    console.error('Failed to load chart data:', error)
  }
}

const renderOverviewChart = (data) => {
  if (!overviewChartInstance) {
    overviewChartInstance = echarts.init(overviewChart.value)
  }
  
  const option = createTrafficLineChart(data, themeStore.isDark, t)
  overviewChartInstance.setOption(option)
}

const renderTopVMsChart = (data) => {
  if (!topVMsChartInstance) {
    topVMsChartInstance = echarts.init(topVMsChart.value)
  }
  
  const option = createTopVMsBarChart(data, themeStore.isDark, t)
  topVMsChartInstance.setOption(option)
}

const resizeCharts = () => {
  overviewChartInstance?.resize()
  topVMsChartInstance?.resize()
}

// 监听主题变化
watch(() => themeStore.isDark, () => {
  loadChartData()
})

onMounted(() => {
  loadChartData()
  
  // 监听手动刷新事件
  window.addEventListener('refresh-data', loadChartData)
  window.addEventListener('resize', resizeCharts)
})

onUnmounted(() => {
  window.removeEventListener('refresh-data', loadChartData)
  window.removeEventListener('resize', resizeCharts)
  overviewChartInstance?.dispose()
  topVMsChartInstance?.dispose()
})
</script>

<style scoped>
.charts {
  width: 100%;
}

.controls-card {
  margin-bottom: 20px;
}

.chart-card {
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
</style>
