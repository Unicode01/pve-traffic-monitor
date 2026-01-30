<template>
  <div class="vm-detail">
    <el-page-header @back="handleBack" :title="t('vmDetail.title')">
      <template #content>
        <span class="vm-title">VM{{ vmid }} - {{ vmName }}</span>
      </template>
    </el-page-header>

    <el-card shadow="hover" class="controls-card">
      <el-form :inline="true" class="controls-form">
        <el-form-item :label="t('charts.timeMode')">
          <el-radio-group v-model="timeMode" @change="handleTimeModeChange">
            <el-radio-button value="preset">{{ t('charts.presetMode') }}</el-radio-button>
            <el-radio-button value="custom">{{ t('charts.customMode') }}</el-radio-button>
          </el-radio-group>
        </el-form-item>

        <template v-if="timeMode === 'preset'">
          <el-form-item :label="t('vmDetail.period')">
            <el-select v-model="period" @change="loadData" style="width: 180px;">
              <el-option :label="t('vmDetail.minute')" value="minute" />
              <el-option :label="t('vmDetail.hour')" value="hour" />
              <el-option :label="t('vmDetail.day')" value="day" />
              <el-option :label="t('vmDetail.month')" value="month" />
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
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
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
const timeMode = ref('preset')
const period = ref('day')
const granularity = ref('hour')
const dateTimeRange = ref(null)
const statsData = ref([])

const chartContainer = ref(null)
let chartInstance = null

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
  loadData()
}

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

    // 构建历史数据请求参数
    let params = {}
    if (timeMode.value === 'preset') {
      params.period = period.value
    } else if (dateTimeRange.value && dateTimeRange.value.length === 2) {
      params.granularity = granularity.value
      params.start = new Date(dateTimeRange.value[0]).toISOString()
      params.end = new Date(dateTimeRange.value[1]).toISOString()
    } else {
      params.period = 'day'
    }

    // 加载历史数据
    const historyRes = await api.getHistory(vmid.value, params)
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

:deep(.el-page-header) {
  margin-bottom: 20px;
}
</style>
