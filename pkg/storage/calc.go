package storage

import (
	"pve-traffic-monitor/pkg/models"
	"pve-traffic-monitor/pkg/utils"
	"sort"
	"time"
)

// buildTrafficStats 构建流量统计结果（公共辅助函数，避免代码重复）
func buildTrafficStats(vmid int, period string, startTime, endTime time.Time, direction string, records []models.TrafficRecord) *models.TrafficStats {
	// 设置默认方向
	if direction == "" {
		direction = models.DirectionBoth
	}

	// 如果没有记录，返回零值统计
	if len(records) == 0 {
		return &models.TrafficStats{
			VMID:       vmid,
			Period:     period,
			StartTime:  startTime,
			EndTime:    endTime,
			Direction:  direction,
			RXBytes:    0,
			TXBytes:    0,
			TotalBytes: 0,
			TotalGB:    0,
		}
	}

	// 计算流量（正确处理虚拟机重启的情况）
	totalRXBytes, totalTXBytes := calculateTraffic(vmid, records)

	// 根据方向计算总流量
	var totalBytes uint64
	var totalGB float64

	switch direction {
	case models.DirectionUpload, models.DirectionTX:
		totalBytes = totalTXBytes
		totalGB = float64(totalTXBytes) / models.BytesPerGB
	case models.DirectionDownload, models.DirectionRX:
		totalBytes = totalRXBytes
		totalGB = float64(totalRXBytes) / models.BytesPerGB
	default: // "both"
		totalBytes = totalRXBytes + totalTXBytes
		totalGB = float64(totalRXBytes+totalTXBytes) / models.BytesPerGB
	}

	return &models.TrafficStats{
		VMID:       vmid,
		Period:     period,
		StartTime:  startTime,
		EndTime:    endTime,
		Direction:  direction,
		RXBytes:    totalRXBytes,
		TXBytes:    totalTXBytes,
		TotalBytes: totalBytes,
		TotalGB:    totalGB,
	}
}

// calculateTraffic 计算流量统计（正确处理VM重启的情况）
// 这是一个公共函数，供 FileStorage 和 DatabaseStorage 共用
//
// 算法说明:
// 1. 如果第一、二条记录之间检测到重启 → 忽略第一条记录，从第二条(重启后)开始计算
// 2. 如果周期内发生重启 → 重启前的流量累加，重启后从0开始重新累计
// 3. 正常情况 → 最后一条减第一条就是周期内的增量
func calculateTraffic(vmid int, records []models.TrafficRecord) (totalRXBytes, totalTXBytes uint64) {
	if len(records) == 0 {
		return 0, 0
	}

	// 如果只有一条记录，无法计算增量
	if len(records) == 1 {
		return 0, 0
	}

	var totalRX, totalTX uint64

	// 检查第一、二条记录之间是否有重启
	firstRXRestart := records[1].RXBytes < records[0].RXBytes
	firstTXRestart := records[1].TXBytes < records[0].TXBytes

	// RX流量计算：确定初始基准值
	var rxSegmentStart uint64
	if firstRXRestart {
		// 第一条记录后就重启了，忽略第一条记录，从第二条(重启后)开始
		utils.DebugLog("[流量统计] 虚拟机 %d 在周期开始时重启(RX)，从重启后记录开始计算",
			vmid)
		rxSegmentStart = 0 // 重启后从0开始
	} else {
		// 正常情况，第一条作为基准
		rxSegmentStart = records[0].RXBytes
	}

	// TX流量计算：确定初始基准值
	var txSegmentStart uint64
	if firstTXRestart {
		// 第一条记录后就重启了，忽略第一条记录，从第二条(重启后)开始
		utils.DebugLog("[流量统计] 虚拟机 %d 在周期开始时重启(TX)，从重启后记录开始计算",
			vmid)
		txSegmentStart = 0 // 重启后从0开始
	} else {
		// 正常情况，第一条作为基准
		txSegmentStart = records[0].TXBytes
	}

	// 遍历记录，检测后续的重启（从第2条记录开始检查）
	for i := 1; i < len(records); i++ {
		// 跳过已经处理的第一次重启
		if i == 1 && (firstRXRestart || firstTXRestart) {
			continue
		}

		currentRX := records[i].RXBytes
		previousRX := records[i-1].RXBytes
		currentTX := records[i].TXBytes
		previousTX := records[i-1].TXBytes

		// 检测RX重启（流量计数器重置）
		if currentRX < previousRX {
			utils.DebugLog("[流量统计] 检测到虚拟机 %d 在 %s RX重启，调整流量计算",
				vmid, records[i].Timestamp.Format("2006-01-02 15:04:05"))

			// 累加重启前这一段的流量
			totalRX += previousRX - rxSegmentStart

			// 重启后从0开始（下一个记录的值就是从0开始的累计）
			rxSegmentStart = 0
		}

		// 检测TX重启（流量计数器重置）
		if currentTX < previousTX {
			utils.DebugLog("[流量统计] 检测到虚拟机 %d 在 %s TX重启，调整流量计算",
				vmid, records[i].Timestamp.Format("2006-01-02 15:04:05"))

			// 累加重启前这一段的流量
			totalTX += previousTX - txSegmentStart

			// 重启后从0开始（下一个记录的值就是从0开始的累计）
			txSegmentStart = 0
		}
	}

	// 加上当前段的流量（从最后一次重启/开始 到 最后一条记录）
	lastRecord := records[len(records)-1]
	totalRX += lastRecord.RXBytes - rxSegmentStart
	totalTX += lastRecord.TXBytes - txSegmentStart

	return totalRX, totalTX
}

// CalculateTrafficExample 计算示例说明
/*
示例1: 无重启的情况
记录1: RX=0,   TX=0    (基准)
记录2: RX=100, TX=50   (增量: RX=100, TX=50)
记录3: RX=200, TX=120  (增量: RX=100, TX=70)

计算过程:
- rxSegmentStart = 0, txSegmentStart = 0
- 无重启检测
- 最后: totalRX = 200-0 = 200, totalTX = 120-0 = 120 ✅

示例2: 有重启的情况
记录1: RX=0,   TX=0    (基准)
记录2: RX=100, TX=50
记录3: RX=200, TX=120
记录4: RX=300, TX=180
记录5: RX=50,  TX=30   (检测到重启! 50 < 300, 30 < 180)
记录6: RX=150, TX=100

计算过程:
- 初始: rxSegmentStart=0, txSegmentStart=0
- i=1,2,3: 无重启
- i=4 (记录5):
  * 检测到RX重启 (50 < 300)
    totalRX += 300 - 0 = 300
    rxSegmentStart = 50
  * 检测到TX重启 (30 < 180)
    totalTX += 180 - 0 = 180
    txSegmentStart = 30
- i=5 (记录6): 无重启
- 最后:
  totalRX += 150 - 50 = 100, totalRX = 400 ✅
  totalTX += 100 - 30 = 70,  totalTX = 250 ✅

示例3: 多次重启
记录1: RX=0
记录2: RX=100
记录3: RX=50   (第1次重启)
记录4: RX=200
记录5: RX=80   (第2次重启)
记录6: RX=150

计算过程:
- 初始: rxSegmentStart=0
- i=1: 正常
- i=2 (记录3):
  检测重启, totalRX += 100-0 = 100, rxSegmentStart=50
- i=3: 正常
- i=4 (记录5):
  检测重启, totalRX += 200-50 = 150, totalRX=250, rxSegmentStart=80
- i=5: 正常
- 最后: totalRX += 150-80 = 70, totalRX = 320 ✅

总流量 = 100(第1段) + 150(第2段) + 70(第3段) = 320
*/

// AggregatedPoint 聚合后的流量数据点
type AggregatedPoint struct {
	Timestamp  time.Time
	RXBytes    uint64
	TXBytes    uint64
	TotalBytes uint64
}

// AggregateTrafficByPeriod 按时间段聚合流量数据（通用函数，API和图表都可使用）
func AggregateTrafficByPeriod(records []models.TrafficRecord, period string) []AggregatedPoint {
	if len(records) == 0 {
		return []AggregatedPoint{}
	}

	// 按时间段分组
	type GroupData struct {
		RXBytes uint64
		TXBytes uint64
		Count   int
	}
	groups := make(map[string]*GroupData)

	// 获取时间段的key
	getKey := func(t time.Time) string {
		switch period {
		case models.PeriodMinute:
			return t.Format(models.TimeFormatMinute)
		case models.PeriodHour:
			return t.Format(models.TimeFormatHour)
		case models.PeriodDay:
			return t.Format(models.TimeFormatDay)
		case models.PeriodMonth:
			return t.Format(models.TimeFormatMonth)
		default:
			return t.Format(models.TimeFormatDay)
		}
	}

	// 计算每个采集点的增量，然后聚合到时间段
	var prevRX, prevTX uint64

	for i, record := range records {
		if i == 0 {
			prevRX = record.RXBytes
			prevTX = record.TXBytes
			continue
		}

		// 检测重启并计算增量
		var deltaRX, deltaTX uint64
		if record.RXBytes < prevRX {
			deltaRX = record.RXBytes // 重启
		} else {
			deltaRX = record.RXBytes - prevRX
		}
		if record.TXBytes < prevTX {
			deltaTX = record.TXBytes // 重启
		} else {
			deltaTX = record.TXBytes - prevTX
		}

		// 聚合到对应的时间段
		key := getKey(record.Timestamp)
		if groups[key] == nil {
			groups[key] = &GroupData{}
		}
		groups[key].RXBytes += deltaRX
		groups[key].TXBytes += deltaTX
		groups[key].Count++

		prevRX = record.RXBytes
		prevTX = record.TXBytes
	}

	// 转换为数组
	var result []AggregatedPoint
	for timeStr, data := range groups {
		// 根据period选择正确的时间格式
		var format string
		switch period {
		case models.PeriodMinute, models.PeriodHour:
			format = models.TimeFormatMinute
		case models.PeriodDay:
			format = models.TimeFormatDay
		case models.PeriodMonth:
			format = models.TimeFormatMonth
		default:
			format = models.TimeFormatDay
		}

		timestamp, err := time.ParseInLocation(format, timeStr, time.Local)
		if err != nil {
			// 解析失败，跳过这个数据点
			utils.DebugLog("[聚合] 时间解析失败: %s, 格式: %s", timeStr, format)
			continue
		}

		result = append(result, AggregatedPoint{
			Timestamp:  timestamp,
			RXBytes:    data.RXBytes,
			TXBytes:    data.TXBytes,
			TotalBytes: data.RXBytes + data.TXBytes,
		})
	}

	// 使用标准库排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.Before(result[j].Timestamp)
	})

	return result
}
