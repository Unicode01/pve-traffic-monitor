import axios from 'axios'

// 从 localStorage 获取 token
const getToken = () => {
  return localStorage.getItem('api_token') || ''
}

// 设置 token
export const setApiToken = (token) => {
  if (token) {
    localStorage.setItem('api_token', token)
  } else {
    localStorage.removeItem('api_token')
  }
}

// 获取当前 token
export const getApiToken = () => {
  return getToken()
}

const request = axios.create({
  baseURL: '/api',
  timeout: 30000
})

// 请求拦截器：添加 token
request.interceptors.request.use(
  config => {
    const token = getToken()
    if (token) {
      config.headers['X-API-Token'] = token
    }
    return config
  },
  error => {
    return Promise.reject(error)
  }
)

request.interceptors.response.use(
  response => response.data,
  error => {
    console.error('API Error:', error)
    // 如果是 401 错误，可能需要重新输入 token
    if (error.response && error.response.status === 401) {
      // 触发自定义事件，通知应用需要认证
      window.dispatchEvent(new CustomEvent('api-unauthorized'))
    }
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
