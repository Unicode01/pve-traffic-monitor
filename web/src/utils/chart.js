/**
 * 图表配色方案
 */

// 亮色主题配色
export const lightColors = {
  download: '#36a2eb',
  upload: '#4bc0c0',
  total: '#ff6384',
  barColors: ['#36a2eb', '#4bc0c0', '#ff6384', '#8bc34a', '#ffc107'],
  lineColors: ['#36a2eb', '#4bc0c0', '#ff6384'],
  background: '#ffffff',
  textColor: '#2c3e50',
  axisLine: '#dcdfe6',
  splitLine: '#e6e6e6',
  tooltipBg: '#ffffff'
}

// 暗色主题配色
export const darkColors = {
  download: '#5cb3ff',
  upload: '#5cd3d3',
  total: '#ff8fa3',
  barColors: ['#5cb3ff', '#5cd3d3', '#ff8fa3', '#a0d911', '#ffc53d'],
  lineColors: ['#5cb3ff', '#5cd3d3', '#ff8fa3'],
  background: '#1f1f1f',
  textColor: '#e4e7ed',
  axisLine: '#4c4d4f',
  splitLine: '#3a3a3a',
  tooltipBg: '#2d2d2d'
}

/**
 * 获取图表颜色方案
 */
export function getChartColors(isDark) {
  return isDark ? darkColors : lightColors
}

/**
 * 智能选择单位和转换数据
 */
function smartConvertBytes(data) {
  // 找出最大值决定使用什么单位
  const maxBytes = Math.max(...data.map(item => item.total_bytes || 0))

  let divisor, unit
  if (maxBytes < 1024 * 1024) {
    // 小于1MB，使用KB
    divisor = 1024
    unit = 'KB'
  } else if (maxBytes < 1024 * 1024 * 1024) {
    // 小于1GB，使用MB
    divisor = 1024 * 1024
    unit = 'MB'
  } else {
    // 使用GB
    divisor = 1024 * 1024 * 1024
    unit = 'GB'
  }

  return { divisor, unit }
}

/**
 * 创建流量折线图配置（用于统计页面 - VM汇总）
 */
export function createTrafficLineChart(data, isDark, t) {
  const colors = getChartColors(isDark)

  // 按vmid排序
  const sortedData = [...data].sort((a, b) => (a.vmid || 0) - (b.vmid || 0))

  const labels = sortedData.map(item => {
    const vmid = item.vmid || 'N/A'
    const name = item.name || 'N/A'
    return `VM${vmid} (${name})`
  })

  const { divisor, unit } = smartConvertBytes(sortedData)

  const rxData = sortedData.map(item => Number((item.rx_bytes / divisor).toFixed(3)))
  const txData = sortedData.map(item => Number((item.tx_bytes / divisor).toFixed(3)))
  const totalData = sortedData.map(item => Number((item.total_bytes / divisor).toFixed(3)))

  return {
    backgroundColor: 'transparent',
    textStyle: {
      color: colors.textColor
    },
    grid: {
      left: '3%',
      right: '4%',
      bottom: '3%',
      top: '15%',
      containLabel: true
    },
    tooltip: {
      trigger: 'axis',
      backgroundColor: colors.tooltipBg,
      borderColor: colors.axisLine,
      textStyle: {
        color: colors.textColor
      },
      formatter: function (params) {
        if (!params || params.length === 0) return ''
        let result = params[0].axisValue + '<br/>'
        params.forEach(item => {
          const value = Number(item.value)
          result += item.marker + ' ' + item.seriesName + ': ' + value.toFixed(3) + ' ' + unit + '<br/>'
        })
        return result
      }
    },
    legend: {
      textStyle: {
        color: colors.textColor
      }
    },
    xAxis: {
      type: 'category',
      data: labels,
      axisLine: {
        lineStyle: {
          color: colors.axisLine
        }
      },
      axisLabel: {
        color: colors.textColor,
        rotate: 45
      }
    },
    yAxis: {
      type: 'value',
      name: `Traffic (${unit})`,
      axisLine: {
        lineStyle: {
          color: colors.axisLine
        }
      },
      axisLabel: {
        color: colors.textColor,
        formatter: '{value}'
      },
      splitLine: {
        lineStyle: {
          color: colors.splitLine
        }
      }
    },
    series: [
      {
        name: t('charts.download'),
        type: 'line',
        data: rxData,
        smooth: true,
        itemStyle: {
          color: colors.download
        },
        areaStyle: {
          color: {
            type: 'linear',
            x: 0,
            y: 0,
            x2: 0,
            y2: 1,
            colorStops: [{
              offset: 0,
              color: colors.download + '40'
            }, {
              offset: 1,
              color: colors.download + '10'
            }]
          }
        }
      },
      {
        name: t('charts.upload'),
        type: 'line',
        data: txData,
        smooth: true,
        itemStyle: {
          color: colors.upload
        },
        areaStyle: {
          color: {
            type: 'linear',
            x: 0,
            y: 0,
            x2: 0,
            y2: 1,
            colorStops: [{
              offset: 0,
              color: colors.upload + '40'
            }, {
              offset: 1,
              color: colors.upload + '10'
            }]
          }
        }
      },
      {
        name: t('dashboard.total'),
        type: 'line',
        data: totalData,
        smooth: true,
        itemStyle: {
          color: colors.total
        },
        lineStyle: {
          width: 3
        }
      }
    ]
  }
}

/**
 * 创建VM详情时间序列图表配置（横坐标为时间）
 */
export function createVMTimeSeriesChart(historyData, isDark, t) {
  const colors = getChartColors(isDark)

  if (!historyData || historyData.length === 0) {
    return {}
  }

  // 横坐标：时间
  const labels = historyData.map(item => item.timestamp)

  // 智能选择单位
  const { divisor, unit } = smartConvertBytes(historyData)

  const rxData = historyData.map(item => Number((item.rx_bytes / divisor).toFixed(3)))
  const txData = historyData.map(item => Number((item.tx_bytes / divisor).toFixed(3)))
  const totalData = historyData.map(item => Number((item.total_bytes / divisor).toFixed(3)))

  return {
    backgroundColor: 'transparent',
    textStyle: {
      color: colors.textColor
    },
    grid: {
      left: '3%',
      right: '4%',
      bottom: '10%',
      top: '12%',
      containLabel: true
    },
    tooltip: {
      trigger: 'axis',
      backgroundColor: colors.tooltipBg,
      borderColor: colors.axisLine,
      textStyle: {
        color: colors.textColor
      },
      formatter: function (params) {
        if (!params || params.length === 0) return ''
        let result = params[0].axisValue + '<br/>'
        params.forEach(item => {
          const value = Number(item.value)
          result += item.marker + ' ' + item.seriesName + ': ' + value.toFixed(3) + ' ' + unit + '<br/>'
        })
        return result
      }
    },
    legend: {
      top: '2%',
      textStyle: {
        color: colors.textColor
      }
    },
    xAxis: {
      type: 'category',
      data: labels,
      axisLine: {
        lineStyle: {
          color: colors.axisLine
        }
      },
      axisLabel: {
        color: colors.textColor,
        rotate: 45
      }
    },
    yAxis: {
      type: 'value',
      name: `Traffic (${unit})`,
      axisLine: {
        lineStyle: {
          color: colors.axisLine
        }
      },
      axisLabel: {
        color: colors.textColor,
        formatter: '{value}'
      },
      splitLine: {
        lineStyle: {
          color: colors.splitLine
        }
      }
    },
    series: [
      {
        name: t('charts.download'),
        type: 'line',
        data: rxData,
        smooth: true,
        itemStyle: {
          color: colors.download
        },
        areaStyle: {
          color: {
            type: 'linear',
            x: 0,
            y: 0,
            x2: 0,
            y2: 1,
            colorStops: [{
              offset: 0,
              color: colors.download + '40'
            }, {
              offset: 1,
              color: colors.download + '10'
            }]
          }
        }
      },
      {
        name: t('charts.upload'),
        type: 'line',
        data: txData,
        smooth: true,
        itemStyle: {
          color: colors.upload
        },
        areaStyle: {
          color: {
            type: 'linear',
            x: 0,
            y: 0,
            x2: 0,
            y2: 1,
            colorStops: [{
              offset: 0,
              color: colors.upload + '40'
            }, {
              offset: 1,
              color: colors.upload + '10'
            }]
          }
        }
      },
      {
        name: t('dashboard.total'),
        type: 'line',
        data: totalData,
        smooth: true,
        itemStyle: {
          color: colors.total
        },
        lineStyle: {
          width: 3
        }
      }
    ]
  }
}

/**
 * 创建Top虚拟机柱状图配置
 */
export function createTopVMsBarChart(data, isDark, t) {
  const colors = getChartColors(isDark)

  console.log('Top VMs chart data:', data)

  const sorted = [...data].sort((a, b) => b.total_bytes - a.total_bytes).slice(0, 10)

  const { divisor, unit } = smartConvertBytes(sorted)

  const labels = sorted.map(s => {
    const vmid = s.vmid || 'N/A'
    const name = s.name || 'N/A'
    return `VM${vmid} (${name})`
  })
  const values = sorted.map(s => Number((s.total_bytes / divisor).toFixed(3)))

  return {
    backgroundColor: 'transparent',
    textStyle: {
      color: colors.textColor
    },
    grid: {
      left: '3%',
      right: '4%',
      bottom: '3%',
      top: '15%',
      containLabel: true
    },
    tooltip: {
      trigger: 'axis',
      backgroundColor: colors.tooltipBg,
      borderColor: colors.axisLine,
      textStyle: {
        color: colors.textColor
      },
      formatter: function (params) {
        if (!params || params.length === 0) return ''
        const item = params[0]
        const value = Number(item.value)
        return item.axisValue + '<br/>' +
          item.marker + ' ' + item.seriesName + ': ' + value.toFixed(3) + ' ' + unit
      }
    },
    legend: {
      textStyle: {
        color: colors.textColor
      }
    },
    xAxis: {
      type: 'category',
      data: labels,
      axisLine: {
        lineStyle: {
          color: colors.axisLine
        }
      },
      axisLabel: {
        color: colors.textColor,
        rotate: 45
      }
    },
    yAxis: {
      type: 'value',
      name: t('dashboard.total') + ` (${unit})`,
      axisLine: {
        lineStyle: {
          color: colors.axisLine
        }
      },
      axisLabel: {
        color: colors.textColor,
        formatter: '{value}'
      },
      splitLine: {
        lineStyle: {
          color: colors.splitLine
        }
      }
    },
    series: [
      {
        name: t('dashboard.total'),
        type: 'bar',
        data: values,
        itemStyle: {
          color: {
            type: 'linear',
            x: 0,
            y: 0,
            x2: 0,
            y2: 1,
            colorStops: [{
              offset: 0,
              color: colors.total
            }, {
              offset: 1,
              color: colors.total + '80'
            }]
          }
        },
        barWidth: '60%'
      }
    ]
  }
}
