import axios from 'axios'

const request = axios.create({
  baseURL: '/api',
  timeout: 30000
})

request.interceptors.response.use(
  response => response.data,
  error => {
    console.error('API Error:', error)
    return Promise.reject(error)
  }
)

export const api = {
  // 获取所有虚拟机
  getVMs() {
    return request.get('/vms')
  },

  // 获取单个虚拟机详情
  getVM(vmid) {
    return request.get(`/vm/${vmid}`)
  },

  // 获取流量统计
  getStats(params) {
    return request.get('/stats', { params })
  },

  // 获取历史数据
  getHistory(vmid, params) {
    return request.get(`/history/${vmid}`, { params })
  },

  // 获取系统统计
  getSystemStats() {
    return request.get('/system/stats')
  },

  // 获取规则列表
  getRules() {
    return request.get('/rules')
  },

  // 获取日志
  getLogs(params) {
    return request.get('/logs', { params })
  }
}

export default api
