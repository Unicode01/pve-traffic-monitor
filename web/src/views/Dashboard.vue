<template>
  <div class="dashboard">
    <el-row :gutter="20" class="stats-row">
      <el-col :xs="24" :sm="12" :md="12" :lg="8" :xl="4">
        <el-card shadow="hover">
          <div class="stat-card">
            <div class="stat-label">{{ t('dashboard.totalVMs') }}</div>
            <div class="stat-value">{{ appStore.stats.totalVMs }}</div>
          </div>
        </el-card>
      </el-col>
      
      <el-col :xs="24" :sm="12" :md="12" :lg="8" :xl="4">
        <el-card shadow="hover">
          <div class="stat-card">
            <div class="stat-label">{{ t('dashboard.runningVMs') }}</div>
            <div class="stat-value success">{{ appStore.stats.runningVMs }}</div>
          </div>
        </el-card>
      </el-col>
      
      <el-col :xs="24" :sm="12" :md="12" :lg="8" :xl="6">
        <el-card shadow="hover">
          <div class="stat-card">
            <div class="stat-label">{{ t('dashboard.todayTraffic') }}</div>
            <div class="stat-value primary">{{ appStore.stats.totalTraffic }}</div>
          </div>
        </el-card>
      </el-col>
      
      <el-col :xs="24" :sm="12" :md="12" :lg="12" :xl="5">
        <el-card shadow="hover">
          <div class="stat-card">
            <div class="stat-label">{{ t('dashboard.totalSamples') }}</div>
            <div class="stat-value">{{ appStore.stats.totalSamples }}</div>
          </div>
        </el-card>
      </el-col>
      
      <el-col :xs="24" :sm="12" :md="12" :lg="12" :xl="5">
        <el-card shadow="hover">
          <div class="stat-card">
            <div class="stat-label">{{ t('dashboard.apiAvgTime') }}</div>
            <div class="stat-value warning">{{ appStore.stats.apiAvgTime }}</div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <el-card shadow="hover" class="vm-card">
      <template #header>
        <div class="card-header">
          <span>{{ t('dashboard.title') }}</span>
        </div>
      </template>

      <el-table
        v-loading="appStore.loading"
        :data="appStore.vms"
        stripe
        style="width: 100%"
        :empty-text="t('common.noData')"
        :default-sort="{ prop: 'vmid', order: 'ascending' }"
      >
        <el-table-column
          prop="vmid"
          :label="t('dashboard.vmId')"
          sortable
          width="80"
        />
        
        <el-table-column
          prop="name"
          :label="t('dashboard.name')"
          min-width="120"
        />
        
        <el-table-column
          prop="status"
          :label="t('dashboard.status')"
          width="100"
        >
          <template #default="{ row }">
            <el-tag :type="row.status === 'running' ? 'success' : 'info'" size="small">
              {{ row.status === 'running' ? t('dashboard.running') : t('dashboard.stopped') }}
            </el-tag>
          </template>
        </el-table-column>
        
        <el-table-column
          prop="matched_rules"
          :label="t('dashboard.matchedRules')"
          min-width="150"
        >
          <template #default="{ row }">
            <el-tag v-for="rule in row.matched_rules" :key="rule" size="small" style="margin-right: 4px;">
              {{ rule }}
            </el-tag>
            <span v-if="!row.matched_rules || row.matched_rules.length === 0">-</span>
          </template>
        </el-table-column>
        
        <el-table-column
          :label="t('dashboard.download')"
          width="120"
          sortable
          :sort-method="(a, b) => (a.netrx || 0) - (b.netrx || 0)"
        >
          <template #default="{ row }">
            <span class="traffic">{{ formatBytes(row.netrx || 0) }}</span>
          </template>
        </el-table-column>
        
        <el-table-column
          :label="t('dashboard.upload')"
          width="120"
          sortable
          :sort-method="(a, b) => (a.nettx || 0) - (b.nettx || 0)"
        >
          <template #default="{ row }">
            <span class="traffic">{{ formatBytes(row.nettx || 0) }}</span>
          </template>
        </el-table-column>
        
        <el-table-column
          :label="t('dashboard.total')"
          width="120"
          sortable
          :sort-method="(a, b) => ((a.netrx || 0) + (a.nettx || 0)) - ((b.netrx || 0) + (b.nettx || 0))"
        >
          <template #default="{ row }">
            <span class="traffic total">{{ formatBytes((row.netrx || 0) + (row.nettx || 0)) }}</span>
          </template>
        </el-table-column>
        
        <el-table-column
          :label="t('dashboard.action')"
          width="100"
          fixed="right"
        >
          <template #default="{ row }">
            <el-button type="primary" size="small" @click="handleViewDetails(row.vmid)">
              {{ t('dashboard.details') }}
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup>
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { api } from '@/api'
import { formatBytes, formatNumber } from '@/utils/format'

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()

const loadData = async () => {
  appStore.setLoading(true)
  
  try {
    // 加载虚拟机列表
    const vmsRes = await api.getVMs()
    if (vmsRes.success) {
      appStore.setVMs(vmsRes.data || [])
      
      // 计算统计数据
      const total = vmsRes.data.length
      const running = vmsRes.data.filter(vm => vm.status === 'running').length
      
      // 获取今日流量
      const statsRes = await api.getStats({ period: 'day', direction: 'both' })
      let totalTraffic = 0
      if (statsRes.success && statsRes.data) {
        totalTraffic = statsRes.data.reduce((sum, stat) => sum + stat.total_bytes, 0)
      }
      
      // 获取系统统计
      const sysStatsRes = await api.getSystemStats()
      let totalSamples = 0
      let apiAvgTime = 0
      
      if (sysStatsRes.success && sysStatsRes.data) {
        totalSamples = sysStatsRes.data.total_records || 0
        apiAvgTime = sysStatsRes.data.api_performance?.recent100_avg_ms || 
                     sysStatsRes.data.api_performance?.avg_response_ms || 0
      }
      
      appStore.setStats({
        totalVMs: total,
        runningVMs: running,
        totalTraffic: formatBytes(totalTraffic),
        totalSamples: formatNumber(totalSamples),
        apiAvgTime: apiAvgTime.toFixed(1) + ' ms'
      })
    }
  } catch (error) {
    console.error('Failed to load data:', error)
  } finally {
    appStore.setLoading(false)
  }
}

const handleViewDetails = (vmid) => {
  router.push(`/vm/${vmid}`)
}

onMounted(() => {
  loadData()
  
  // 监听手动刷新事件
  window.addEventListener('refresh-data', loadData)
  
  // 自动刷新
  const interval = setInterval(loadData, 30000)
  
  return () => {
    clearInterval(interval)
    window.removeEventListener('refresh-data', loadData)
  }
})
</script>

<style scoped>
.dashboard {
  width: 100%;
}

.stats-row {
  margin-bottom: 20px;
}

.stat-card {
  text-align: center;
  padding: 10px 0;
}

.stat-label {
  font-size: 14px;
  color: var(--el-text-color-secondary);
  margin-bottom: 8px;
}

.stat-value {
  font-size: 28px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.stat-value.success {
  color: var(--el-color-success);
}

.stat-value.primary {
  color: var(--el-color-primary);
}

.stat-value.warning {
  color: var(--el-color-warning);
}

.vm-card {
  margin-bottom: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-weight: 600;
}

.traffic {
  color: var(--el-color-primary);
  font-weight: 500;
}

.traffic.total {
  font-weight: 600;
}
</style>
