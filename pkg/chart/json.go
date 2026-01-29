package chart

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"pve-traffic-monitor/pkg/models"
	"pve-traffic-monitor/pkg/storage"
	"time"
)

// ExportJSONData 导出JSON格式数据
func (e *Exporter) ExportJSONData(vmid int, vmName string, records []models.TrafficRecord, startTime, endTime time.Time) (string, error) {
	if len(records) == 0 {
		return "", fmt.Errorf("no traffic records")
	}

	// 使用storage包的正确聚合函数（计算增量而非累积值）
	aggregated := storage.AggregateTrafficByPeriod(records, "hour")

	if len(aggregated) == 0 {
		return "", fmt.Errorf("no aggregated data")
	}

	// 准备导出数据
	exportData := map[string]interface{}{
		"vmid":        vmid,
		"vm_name":     vmName,
		"start_time":  startTime.Format(time.RFC3339),
		"end_time":    endTime.Format(time.RFC3339),
		"data_points": len(aggregated),
		"records":     aggregated,
		"summary": map[string]interface{}{
			"total_rx_bytes": func() uint64 {
				var sum uint64
				for _, p := range aggregated {
					sum += p.RXBytes
				}
				return sum
			}(),
			"total_tx_bytes": func() uint64 {
				var sum uint64
				for _, p := range aggregated {
					sum += p.TXBytes
				}
				return sum
			}(),
			"total_bytes": func() uint64 {
				var sum uint64
				for _, p := range aggregated {
					sum += p.TotalBytes
				}
				return sum
			}(),
		},
	}

	// 生成文件名
	timestamp := time.Now().Format("20060102_150405")
	var filename string
	if !startTime.IsZero() && !endTime.IsZero() {
		dateRange := fmt.Sprintf("%s_to_%s",
			startTime.Format("20060102"),
			endTime.Format("20060102"))
		filename = filepath.Join(e.exportPath,
			fmt.Sprintf("vm_%d_traffic_%s_%s.json", vmid, dateRange, timestamp))
	} else {
		filename = filepath.Join(e.exportPath,
			fmt.Sprintf("vm_%d_traffic_%s.json", vmid, timestamp))
	}

	// 保存JSON文件
	f, err := os.Create(filename)
	if err != nil {
		return "", fmt.Errorf("创建JSON文件失败: %w", err)
	}
	defer f.Close()

	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(exportData); err != nil {
		return "", fmt.Errorf("写入JSON失败: %w", err)
	}

	return filename, nil
}

// ExportStatsJSONData 导出统计JSON数据
func (e *Exporter) ExportStatsJSONData(stats []models.TrafficStats, direction string) (string, error) {
	if len(stats) == 0 {
		return "", fmt.Errorf("no statistics data")
	}

	// 排序
	sortedStats := make([]models.TrafficStats, len(stats))
	copy(sortedStats, stats)

	switch direction {
	case "rx":
		for i := 0; i < len(sortedStats)-1; i++ {
			for j := i + 1; j < len(sortedStats); j++ {
				if sortedStats[j].RXBytes > sortedStats[i].RXBytes {
					sortedStats[i], sortedStats[j] = sortedStats[j], sortedStats[i]
				}
			}
		}
	case "tx":
		for i := 0; i < len(sortedStats)-1; i++ {
			for j := i + 1; j < len(sortedStats); j++ {
				if sortedStats[j].TXBytes > sortedStats[i].TXBytes {
					sortedStats[i], sortedStats[j] = sortedStats[j], sortedStats[i]
				}
			}
		}
	default:
		for i := 0; i < len(sortedStats)-1; i++ {
			for j := i + 1; j < len(sortedStats); j++ {
				if sortedStats[j].TotalBytes > sortedStats[i].TotalBytes {
					sortedStats[i], sortedStats[j] = sortedStats[j], sortedStats[i]
				}
			}
		}
	}

	// 计算汇总信息
	var totalRX, totalTX, totalBytes uint64
	for _, stat := range sortedStats {
		totalRX += stat.RXBytes
		totalTX += stat.TXBytes
		totalBytes += stat.TotalBytes
	}

	exportData := map[string]interface{}{
		"direction":  direction,
		"vm_count":   len(sortedStats),
		"statistics": sortedStats,
		"summary": map[string]interface{}{
			"total_rx_bytes": totalRX,
			"total_tx_bytes": totalTX,
			"total_bytes":    totalBytes,
		},
	}

	// 生成文件名
	timestamp := time.Now().Format("20060102_150405")
	var filename string
	if direction == "both" {
		filename = filepath.Join(e.exportPath, fmt.Sprintf("traffic_stats_%s.json", timestamp))
	} else {
		filename = filepath.Join(e.exportPath, fmt.Sprintf("traffic_stats_%s_%s.json", direction, timestamp))
	}

	// 保存JSON文件
	f, err := os.Create(filename)
	if err != nil {
		return "", fmt.Errorf("创建JSON文件失败: %w", err)
	}
	defer f.Close()

	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(exportData); err != nil {
		return "", fmt.Errorf("写入JSON失败: %w", err)
	}

	return filename, nil
}
