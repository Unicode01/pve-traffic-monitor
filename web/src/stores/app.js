import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useAppStore = defineStore('app', () => {
  const loading = ref(false)
  const vms = ref([])
  const stats = ref({
    totalVMs: 0,
    runningVMs: 0,
    totalTraffic: '0 B',
    totalSamples: 0,
    apiAvgTime: '0 ms'
  })

  const setLoading = (val) => {
    loading.value = val
  }

  const setVMs = (data) => {
    vms.value = data
  }

  const setStats = (data) => {
    stats.value = data
  }

  return {
    loading,
    vms,
    stats,
    setLoading,
    setVMs,
    setStats
  }
})
