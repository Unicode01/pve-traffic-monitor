/**
 * 格式化字节数
 */
export function formatBytes(bytes) {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

/**
 * 智能格式化字节数（带精度控制）
 */
export function formatBytesWithPrecision(bytes) {
  if (bytes === 0) return { value: 0, unit: 'B', formatted: '0 B' }

  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  const value = bytes / Math.pow(k, i)

  // 根据大小调整精度
  let decimals = 2
  if (value < 10) {
    decimals = 3
  } else if (value < 100) {
    decimals = 2
  } else {
    decimals = 1
  }

  return {
    value: parseFloat(value.toFixed(decimals)),
    unit: sizes[i],
    formatted: parseFloat(value.toFixed(decimals)) + ' ' + sizes[i]
  }
}

/**
 * 格式化时间
 */
export function formatTime(timestamp) {
  const date = new Date(timestamp)
  return date.toLocaleString()
}

/**
 * 格式化时长
 */
export function formatDuration(ms) {
  if (ms < 1000) return ms.toFixed(0) + ' ms'
  const seconds = ms / 1000
  if (seconds < 60) return seconds.toFixed(1) + ' s'
  const minutes = seconds / 60
  if (minutes < 60) return minutes.toFixed(1) + ' min'
  const hours = minutes / 60
  return hours.toFixed(1) + ' h'
}

/**
 * 格式化数字（千分位）
 */
export function formatNumber(num) {
  if (num >= 1000000) {
    return (num / 1000000).toFixed(2) + 'M'
  } else if (num >= 1000) {
    return (num / 1000).toFixed(1) + 'K'
  } else {
    return num.toString()
  }
}
