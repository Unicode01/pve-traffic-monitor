package chart

import (
	"fmt"
	"os"
	"path/filepath"
	"pve-traffic-monitor/pkg/models"
	"pve-traffic-monitor/pkg/storage"
	"time"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/components"
	"github.com/go-echarts/go-echarts/v2/opts"
)

// ExportHTMLChart 导出HTML图表（使用go-echarts）
func (e *Exporter) ExportHTMLChart(vmid int, vmName string, records []models.TrafficRecord, isDark bool) (string, error) {
	return e.ExportHTMLChartWithRange(vmid, vmName, records, time.Time{}, time.Time{}, isDark)
}

// ExportHTMLChartWithRange 导出HTML图表（带时间范围信息）
func (e *Exporter) ExportHTMLChartWithRange(vmid int, vmName string, records []models.TrafficRecord, startTime, endTime time.Time, isDark bool) (string, error) {
	if len(records) == 0 {
		return "", fmt.Errorf("no traffic records")
	}

	// 使用storage包的正确聚合函数（计算增量而非累积值）
	aggregated := storage.AggregateTrafficByPeriod(records, "hour")

	if len(aggregated) == 0 {
		return "", fmt.Errorf("no aggregated data")
	}

	// 获取配色方案
	colors := models.GetChartColors(isDark)

	// 创建页面
	page := components.NewPage()
	page.PageTitle = fmt.Sprintf("VM %s (ID: %d) Traffic Statistics", vmName, vmid)
	page.AssetsHost = "https://cdn.jsdelivr.net/npm/echarts@5.4.3/dist/"

	// 准备数据
	var xAxis []string
	var rxData []opts.LineData
	var txData []opts.LineData
	var totalData []opts.LineData

	// 智能选择单位
	var maxBytes uint64
	for _, point := range aggregated {
		if point.TotalBytes > maxBytes {
			maxBytes = point.TotalBytes
		}
	}

	divisor := 1024.0 * 1024 * 1024 // 默认GB
	unitLabel := "GB"
	if maxBytes < 1024*1024 {
		divisor = 1024
		unitLabel = "KB"
	} else if maxBytes < 1024*1024*1024 {
		divisor = 1024 * 1024
		unitLabel = "MB"
	}

	for _, point := range aggregated {
		xAxis = append(xAxis, point.Timestamp.Format("01-02 15:04"))
		rxValue := float64(point.RXBytes) / divisor
		txValue := float64(point.TXBytes) / divisor
		totalValue := float64(point.TotalBytes) / divisor

		rxData = append(rxData, opts.LineData{Value: fmt.Sprintf("%.3f", rxValue)})
		txData = append(txData, opts.LineData{Value: fmt.Sprintf("%.3f", txValue)})
		totalData = append(totalData, opts.LineData{Value: fmt.Sprintf("%.3f", totalValue)})
	}

	// 创建折线图
	line := charts.NewLine()

	// 设置标题
	title := fmt.Sprintf("VM %s (ID: %d) Traffic Statistics", vmName, vmid)
	if !startTime.IsZero() && !endTime.IsZero() {
		title = fmt.Sprintf("VM %s (ID: %d) Traffic (%s - %s)",
			vmName, vmid,
			startTime.Format("01-02 15:04"),
			endTime.Format("01-02 15:04"))
	}

	// 获取主题
	theme := "light"
	if isDark {
		theme = "dark"
	}

	line.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			Width:  "1600px",
			Height: "800px",
			Theme:  theme,
		}),
		charts.WithTitleOpts(opts.Title{
			Title: title,
			Left:  "center",
		}),
		charts.WithTooltipOpts(opts.Tooltip{
			Trigger: "axis",
		}),
		charts.WithLegendOpts(opts.Legend{
			Top: "8%",
		}),
		charts.WithGridOpts(opts.Grid{
			Left:   "3%",
			Right:  "4%",
			Bottom: "15%", // 增加底部边距以显示旋转后的X轴标签
			Top:    "15%",
		}),
		charts.WithXAxisOpts(opts.XAxis{
			Name: "Time",
			AxisLabel: &opts.AxisLabel{
				Rotate: 45,
			},
		}),
		charts.WithYAxisOpts(opts.YAxis{
			Name: "Traffic (" + unitLabel + ")",
		}),
	)

	line.SetXAxis(xAxis).
		AddSeries("Download (RX)", rxData,
			charts.WithItemStyleOpts(opts.ItemStyle{
				Color: colors.Download,
			}),
		).
		AddSeries("Upload (TX)", txData,
			charts.WithItemStyleOpts(opts.ItemStyle{
				Color: colors.Upload,
			}),
		).
		AddSeries("Total", totalData,
			charts.WithItemStyleOpts(opts.ItemStyle{
				Color: colors.Total,
			}),
			charts.WithLineStyleOpts(opts.LineStyle{
				Width: 4,
			}),
		)

	page.AddCharts(line)

	// 生成文件名
	timestamp := time.Now().Format("20060102_150405")
	var filename string
	if !startTime.IsZero() && !endTime.IsZero() {
		dateRange := fmt.Sprintf("%s_to_%s",
			startTime.Format("20060102"),
			endTime.Format("20060102"))
		filename = filepath.Join(e.exportPath,
			fmt.Sprintf("vm_%d_traffic_%s_%s.html", vmid, dateRange, timestamp))
	} else {
		filename = filepath.Join(e.exportPath,
			fmt.Sprintf("vm_%d_traffic_%s.html", vmid, timestamp))
	}

	// 保存文件
	f, err := os.Create(filename)
	if err != nil {
		return "", fmt.Errorf("创建HTML文件失败: %w", err)
	}
	defer f.Close()

	if err := page.Render(f); err != nil {
		return "", fmt.Errorf("渲染HTML失败: %w", err)
	}

	return filename, nil
}

// ExportStatsHTMLChart 导出统计HTML图表（柱状图）
func (e *Exporter) ExportStatsHTMLChart(stats []models.TrafficStats, direction string, isDark bool) (string, error) {
	return e.ExportStatsHTMLChartWithRange(stats, direction, time.Time{}, time.Time{}, isDark)
}

// ExportStatsHTMLChartWithRange 导出统计HTML图表（带时间范围信息）
func (e *Exporter) ExportStatsHTMLChartWithRange(stats []models.TrafficStats, direction string, startTime, endTime time.Time, isDark bool) (string, error) {
	if len(stats) == 0 {
		return "", fmt.Errorf("no statistics data")
	}

	// 获取配色方案
	colors := models.GetChartColors(isDark)

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

	// 准备数据
	var xAxis []string
	var barData []opts.BarData
	var color string

	switch direction {
	case "both":
		color = colors.Total
	case "rx":
		color = colors.Download
	case "tx":
		color = colors.Upload
	default:
		color = colors.Total
	}

	// 智能选择单位
	var maxBytes uint64
	for _, stat := range sortedStats {
		var bytes uint64
		switch direction {
		case "rx":
			bytes = stat.RXBytes
		case "tx":
			bytes = stat.TXBytes
		default:
			bytes = stat.TotalBytes
		}
		if bytes > maxBytes {
			maxBytes = bytes
		}
	}

	var divisor float64
	var unitLabel string
	if maxBytes < 1024*1024 {
		divisor = 1024
		unitLabel = "KB"
	} else if maxBytes < 1024*1024*1024 {
		divisor = 1024 * 1024
		unitLabel = "MB"
	} else {
		divisor = 1024 * 1024 * 1024
		unitLabel = "GB"
	}

	for _, stat := range sortedStats {
		var bytes uint64
		switch direction {
		case "rx":
			bytes = stat.RXBytes
		case "tx":
			bytes = stat.TXBytes
		default:
			bytes = stat.TotalBytes
		}

		value := float64(bytes) / divisor
		xAxis = append(xAxis, fmt.Sprintf("VM%d (%s)", stat.VMID, stat.Name))
		barData = append(barData, opts.BarData{Value: fmt.Sprintf("%.3f", value)})
	}

	// 创建页面
	page := components.NewPage()
	page.PageTitle = "VM Traffic Statistics Summary"
	page.AssetsHost = "https://cdn.jsdelivr.net/npm/echarts@5.4.3/dist/"

	// 创建柱状图
	bar := charts.NewBar()

	// 构建标题
	var title string
	switch direction {
	case "rx":
		title = "VM Traffic Statistics - Download (RX)"
	case "tx":
		title = "VM Traffic Statistics - Upload (TX)"
	default:
		title = "VM Traffic Statistics Summary"
	}

	// 添加时间范围信息
	if !startTime.IsZero() && !endTime.IsZero() {
		title = fmt.Sprintf("%s (%s - %s)",
			title,
			startTime.Format("01-02 15:04"),
			endTime.Format("01-02 15:04"))
	}

	// 获取主题
	theme := "light"
	if isDark {
		theme = "dark"
	}

	bar.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			Width:  "1600px",
			Height: "900px",
			Theme:  theme,
		}),
		charts.WithTitleOpts(opts.Title{
			Title: title,
			Left:  "center",
			Top:   "2%",
		}),
		charts.WithTooltipOpts(opts.Tooltip{
			Trigger: "axis",
		}),
		charts.WithLegendOpts(opts.Legend{
			Top: "8%", // 设置图例位置在标题下方
		}),
		charts.WithGridOpts(opts.Grid{
			Left:   "10%",
			Right:  "5%",
			Bottom: "20%", // 增加底部边距以显示旋转后的X轴标签
			Top:    "15%", // 增加顶部边距以适应标题和图例
		}),
		charts.WithXAxisOpts(opts.XAxis{
			AxisLabel: &opts.AxisLabel{
				Rotate: 45,
			},
		}),
		charts.WithYAxisOpts(opts.YAxis{
			Name: fmt.Sprintf("Traffic (%s)", unitLabel),
		}),
	)

	bar.SetXAxis(xAxis).
		AddSeries("Total Traffic", barData,
			charts.WithItemStyleOpts(opts.ItemStyle{
				Color: color,
			}),
		)

	page.AddCharts(bar)

	// 生成文件名
	timestamp := time.Now().Format("20060102_150405")
	var filename string

	// 根据是否有时间范围生成不同的文件名
	if !startTime.IsZero() && !endTime.IsZero() {
		dateRange := fmt.Sprintf("%s_to_%s",
			startTime.Format("20060102"),
			endTime.Format("20060102"))
		if direction == "both" {
			filename = filepath.Join(e.exportPath, fmt.Sprintf("traffic_stats_%s_%s.html", dateRange, timestamp))
		} else {
			filename = filepath.Join(e.exportPath, fmt.Sprintf("traffic_stats_%s_%s_%s.html", direction, dateRange, timestamp))
		}
	} else {
		if direction == "both" {
			filename = filepath.Join(e.exportPath, fmt.Sprintf("traffic_stats_%s.html", timestamp))
		} else {
			filename = filepath.Join(e.exportPath, fmt.Sprintf("traffic_stats_%s_%s.html", direction, timestamp))
		}
	}

	// 保存文件
	f, err := os.Create(filename)
	if err != nil {
		return "", fmt.Errorf("创建HTML文件失败: %w", err)
	}
	defer f.Close()

	if err := page.Render(f); err != nil {
		return "", fmt.Errorf("渲染HTML失败: %w", err)
	}

	return filename, nil
}

// 注意：AggregateTrafficByPeriod 函数已移除
// 现在使用 storage.AggregateTrafficByPeriod，它能正确计算流量增量并处理VM重启的情况
