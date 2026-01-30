<template>
  <div class="charts">
    <el-card shadow="hover" class="controls-card">
      <el-form :inline="true" class="controls-form">
        <el-form-item :label="t('charts.timeMode')">
          <el-radio-group v-model="timeMode" @change="handleTimeModeChange">
            <el-radio-button value="preset">{{ t('charts.presetMode') }}</el-radio-button>
            <el-radio-button value="custom">{{ t('charts.customMode') }}</el-radio-button>
          </el-radio-group>
        </el-form-item>

        <template v-if="timeMode === 'preset'">
          <el-form-item :label="t('charts.period')">
            <el-select v-model="period" @change="loadChartData" style="width: 150px;">
              <el-option :label="t('charts.currentMinute')" value="minute" />
              <el-option :label="t('charts.currentHour')" value="hour" />
              <el-option :label="t('charts.today')" value="day" />
              <el-option :label="t('charts.currentMonth')" value="month" />
            </el-select>
          </el-form-item>
        </template>

        <template v-else>
          <el-form-item :label="t('charts.granularity')">
            <el-select v-model="granularity" style="width: 120px;">
              <el-option :label="t('charts.byMinute')" value="minute" />
              <el-option :label="t('charts.byHour')" value="hour" />
              <el-option :label="t('charts.byDay')" value="day" />
              <el-option :label="t('charts.byMonth')" value="month" />
            </el-select>
          </el-form-item>

          <el-form-item :label="t('charts.timeRange')">
            <el-date-picker
              v-model="dateTimeRange"
              :type="datePickerType"
              :format="datePickerFormat"
              :value-format="datePickerValueFormat"
              range-separator="-"
              :start-placeholder="t('charts.startTime')"
              :end-placeholder="t('charts.endTime')"
              :shortcuts="dateShortcuts"
              style="width: 380px;"
            />
          </el-form-item>
        </template>

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
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useThemeStore } from '@/stores/theme'
import { api } from '@/api'
import * as echarts from 'echarts'
import { createTrafficLineChart, createTopVMsBarChart } from '@/utils/chart'
import { Refresh } from '@element-plus/icons-vue'

const { t } = useI18n()
const themeStore = useThemeStore()

const timeMode = ref('preset')
const period = ref('day')
const direction = ref('both')
const granularity = ref('hour')
const dateTimeRange = ref(null)

const overviewChart = ref(null)
const topVMsChart = ref(null)

let overviewChartInstance = null
let topVMsChartInstance = null

// 根据粒度计算日期选择器类型
const datePickerType = computed(() => {
  switch (granularity.value) {
    case 'minute':
    case 'hour':
      return 'datetimerange'
    case 'day':
      return 'daterange'
    case 'month':
      return 'monthrange'
    default:
      return 'datetimerange'
  }
})

// 日期选择器显示格式
const datePickerFormat = computed(() => {
  switch (granularity.value) {
    case 'minute':
    case 'hour':
      return 'YYYY-MM-DD HH:mm'
    case 'day':
      return 'YYYY-MM-DD'
    case 'month':
      return 'YYYY-MM'
    default:
      return 'YYYY-MM-DD HH:mm'
  }
})

// 日期选择器值格式
const datePickerValueFormat = computed(() => {
  return 'YYYY-MM-DD HH:mm:ss'
})

// 快捷选项
const dateShortcuts = computed(() => {
  const now = new Date()
  return [
    {
      text: t('charts.lastHour'),
      value: () => {
        const end = new Date()
        const start = new Date()
        start.setTime(start.getTime() - 3600 * 1000)
        return [start, end]
      }
    },
    {
      text: t('charts.last24Hours'),
      value: () => {
        const end = new Date()
        const start = new Date()
        start.setTime(start.getTime() - 3600 * 1000 * 24)
        return [start, end]
      }
    },
    {
      text: t('charts.last7Days'),
      value: () => {
        const end = new Date()
        const start = new Date()
        start.setTime(start.getTime() - 3600 * 1000 * 24 * 7)
        return [start, end]
      }
    },
    {
      text: t('charts.last30Days'),
      value: () => {
        const end = new Date()
        const start = new Date()
        start.setTime(start.getTime() - 3600 * 1000 * 24 * 30)
        return [start, end]
      }
    },
    {
      text: t('charts.thisMonth'),
      value: () => {
        const end = new Date()
        const start = new Date(now.getFullYear(), now.getMonth(), 1)
        return [start, end]
      }
    },
    {
      text: t('charts.lastMonth'),
      value: () => {
        const start = new Date(now.getFullYear(), now.getMonth() - 1, 1)
        const end = new Date(now.getFullYear(), now.getMonth(), 0, 23, 59, 59)
        return [start, end]
      }
    }
  ]
})

const handleTimeModeChange = () => {
  if (timeMode.value === 'custom' && !dateTimeRange.value) {
    // 默认设置为最近24小时
    const end = new Date()
    const start = new Date()
    start.setTime(start.getTime() - 3600 * 1000 * 24)
    dateTimeRange.value = [start, end]
  }
  loadChartData()
}

const loadChartData = async () => {
  try {
    let params = { direction: direction.value }

    if (timeMode.value === 'preset') {
      params.period = period.value
    } else if (dateTimeRange.value && dateTimeRange.value.length === 2) {
      params.granularity = granularity.value
      params.start = new Date(dateTimeRange.value[0]).toISOString()
      params.end = new Date(dateTimeRange.value[1]).toISOString()
    } else {
      params.period = 'day'
    }

    console.log('Loading chart data with:', params)
    const res = await api.getStats(params)

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
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}

.controls-form :deep(.el-form-item) {
  margin-bottom: 0;
  margin-right: 0;
}

.controls-form :deep(.el-radio-group) {
  display: flex;
}

@media (max-width: 768px) {
  .controls-form :deep(.el-date-editor) {
    width: 100% !important;
  }
}
</style>
