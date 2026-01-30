package chart

import (
	"fmt"
	"os"
	"path/filepath"
	"pve-traffic-monitor/pkg/models"
	"pve-traffic-monitor/pkg/storage"
	"time"

	"github.com/wcharczuk/go-chart/v2"
	"github.com/wcharczuk/go-chart/v2/drawing"
)

// Exporter 图表导出器
type Exporter struct {
	exportPath string
}

// NewExporter 创建新的图表导出器
func NewExporter(exportPath string) (*Exporter, error) {
	if err := os.MkdirAll(exportPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create export directory: %w", err)
	}

	return &Exporter{
		exportPath: exportPath,
	}, nil
}

// ExportTrafficChart 导出流量图表（显示时间序列的流量变化趋势）
func (e *Exporter) ExportTrafficChart(vmid int, vmName string, records []models.TrafficRecord) (string, error) {
	return e.ExportTrafficChartWithRangeAndPeriod(vmid, vmName, records, time.Time{}, time.Time{}, "hour")
}

// ExportTrafficChartWithRange 导出流量图表（带时间范围信息，默认按小时聚合）
func (e *Exporter) ExportTrafficChartWithRange(vmid int, vmName string, records []models.TrafficRecord, startTime, endTime time.Time) (string, error) {
	return e.ExportTrafficChartWithRangeAndPeriod(vmid, vmName, records, startTime, endTime, "hour")
}

// ExportTrafficChartWithRangeAndPeriod 导出流量图表（带时间范围和自定义聚合周期）
// period: minute/hour/day/month
func (e *Exporter) ExportTrafficChartWithRangeAndPeriod(vmid int, vmName string, records []models.TrafficRecord, startTime, endTime time.Time, period string) (string, error) {
	if len(records) == 0 {
		return "", fmt.Errorf("no traffic records")
	}

	// 验证 period 参数
	if period == "" {
		period = "hour"
	}

	// 按时间段聚合数据，显示趋势而不是累计值
	// 使用共用的聚合函数，与API保持一致
	aggregated := storage.AggregateTrafficByPeriod(records, period)

	if len(aggregated) == 0 {
		return "", fmt.Errorf("no aggregated data")
	}

	// 先找出最大流量值,决定使用什么单位
	var maxBytes uint64
	for _, point := range aggregated {
		if point.TotalBytes > maxBytes {
			maxBytes = point.TotalBytes
		}
	}

	// 如果最大值小于1GB,使用MB作为单位;否则使用GB
	useMB := maxBytes < 1024*1024*1024
	var divisor float64
	var unitLabel string
	if useMB {
		divisor = 1024 * 1024 // MB
		unitLabel = "MB"
	} else {
		divisor = 1024 * 1024 * 1024 // GB
		unitLabel = "GB"
	}

	// 准备图表数据 - 根据单位转换
	xValues := make([]time.Time, len(aggregated))
	yValuesRX := make([]float64, len(aggregated))
	yValuesTX := make([]float64, len(aggregated))
	yValuesTotal := make([]float64, len(aggregated))

	for i, point := range aggregated {
		xValues[i] = point.Timestamp
		yValuesRX[i] = float64(point.RXBytes) / divisor
		yValuesTX[i] = float64(point.TXBytes) / divisor
		yValuesTotal[i] = float64(point.TotalBytes) / divisor
	}

	// 创建图表标题（包含时间范围）
	title := fmt.Sprintf("VM %s (ID: %d) Traffic Statistics", vmName, vmid)
	if !startTime.IsZero() && !endTime.IsZero() {
		title = fmt.Sprintf("VM %s (ID: %d) Traffic (%s - %s)",
			vmName, vmid,
			startTime.Format("01-02 15:04"),
			endTime.Format("01-02 15:04"))
	}

	// 根据 period 选择时间显示格式
	var timeFormat string
	switch period {
	case "minute":
		timeFormat = "01-02 15:04"
	case "hour":
		timeFormat = "01-02 15:04"
	case "day":
		timeFormat = "01-02"
	case "month":
		timeFormat = "2006-01"
	default:
		timeFormat = "01-02 15:04"
	}

	// 现代化配色方案（与前端一致）
	colorDownload := drawing.Color{R: 54, G: 162, B: 235, A: 255}    // 蓝色
	colorUpload := drawing.Color{R: 75, G: 192, B: 192, A: 255}      // 青色
	colorTotal := drawing.Color{R: 255, G: 99, B: 132, A: 255}       // 红色
	colorDownloadFill := drawing.Color{R: 54, G: 162, B: 235, A: 50} // 半透明蓝
	colorUploadFill := drawing.Color{R: 75, G: 192, B: 192, A: 50}   // 半透明青
	colorGrid := drawing.Color{R: 230, G: 230, B: 230, A: 255}       // 浅灰网格
	colorBackground := drawing.Color{R: 250, G: 250, B: 250, A: 255} // 浅色背景

	// 创建现代化图表
	graph := chart.Chart{
		Title: title,
		TitleStyle: chart.Style{
			FontSize:  20,
			FontColor: drawing.Color{R: 44, G: 62, B: 80, A: 255}, // 深色标题
			Padding:   chart.Box{Top: 10, Bottom: 10},
		},
		Width:  1600, // 更宽的图表
		Height: 800,  // 更高的图表
		Background: chart.Style{
			FillColor: colorBackground,
			Padding:   chart.Box{Top: 50, Left: 50, Right: 50, Bottom: 50},
		},
		Canvas: chart.Style{
			FillColor: drawing.Color{R: 255, G: 255, B: 255, A: 255}, // 白色画布
		},
		XAxis: chart.XAxis{
			Name: "Time",
			NameStyle: chart.Style{
				FontSize:  14,
				FontColor: drawing.Color{R: 52, G: 73, B: 94, A: 255},
			},
			Style: chart.Style{
				FontSize:    11,
				FontColor:   drawing.Color{R: 100, G: 100, B: 100, A: 255},
				StrokeColor: drawing.Color{R: 200, G: 200, B: 200, A: 255},
				StrokeWidth: 1,
			},
			ValueFormatter: chart.TimeValueFormatterWithFormat(timeFormat),
			GridMajorStyle: chart.Style{
				StrokeColor: colorGrid,
				StrokeWidth: 1,
			},
			GridMinorStyle: chart.Style{
				StrokeColor: drawing.Color{R: 245, G: 245, B: 245, A: 255},
				StrokeWidth: 0.5,
			},
		},
		YAxis: chart.YAxis{
			Name: fmt.Sprintf("Traffic (%s)", unitLabel),
			NameStyle: chart.Style{
				FontSize:  14,
				FontColor: drawing.Color{R: 52, G: 73, B: 94, A: 255},
			},
			Style: chart.Style{
				FontSize:    11,
				FontColor:   drawing.Color{R: 100, G: 100, B: 100, A: 255},
				StrokeColor: drawing.Color{R: 200, G: 200, B: 200, A: 255},
				StrokeWidth: 1,
			},
			GridMajorStyle: chart.Style{
				StrokeColor: colorGrid,
				StrokeWidth: 1,
			},
			GridMinorStyle: chart.Style{
				StrokeColor: drawing.Color{R: 245, G: 245, B: 245, A: 255},
				StrokeWidth: 0.5,
			},
		},
		Series: []chart.Series{
			// Download 填充区域
			chart.TimeSeries{
				Style: chart.Style{
					StrokeWidth: 0,
					FillColor:   colorDownloadFill,
				},
				XValues: xValues,
				YValues: yValuesRX,
			},
			// Download 线条
			chart.TimeSeries{
				Name: "Download (RX)",
				Style: chart.Style{
					StrokeColor: colorDownload,
					StrokeWidth: 3,
					DotWidth:    4,
				},
				XValues: xValues,
				YValues: yValuesRX,
			},
			// Upload 填充区域
			chart.TimeSeries{
				Style: chart.Style{
					StrokeWidth: 0,
					FillColor:   colorUploadFill,
				},
				XValues: xValues,
				YValues: yValuesTX,
			},
			// Upload 线条
			chart.TimeSeries{
				Name: "Upload (TX)",
				Style: chart.Style{
					StrokeColor: colorUpload,
					StrokeWidth: 3,
					DotWidth:    4,
				},
				XValues: xValues,
				YValues: yValuesTX,
			},
			// Total 线条（无填充）
			chart.TimeSeries{
				Name: "Total",
				Style: chart.Style{
					StrokeColor: colorTotal,
					StrokeWidth: 4,
					DotWidth:    6,
				},
				XValues: xValues,
				YValues: yValuesTotal,
			},
		},
	}

	// 添加图例
	graph.Elements = []chart.Renderable{
		chart.Legend(&graph),
	}

	// 生成文件名（包含时间范围信息）
	timestamp := time.Now().Format("20060102_150405")
	var filename string
	if !startTime.IsZero() && !endTime.IsZero() {
		dateRange := fmt.Sprintf("%s_to_%s",
			startTime.Format("20060102"),
			endTime.Format("20060102"))
		filename = filepath.Join(e.exportPath,
			fmt.Sprintf("vm_%d_traffic_%s_%s.png", vmid, dateRange, timestamp))
	} else {
		filename = filepath.Join(e.exportPath,
			fmt.Sprintf("vm_%d_traffic_%s.png", vmid, timestamp))
	}

	// 保存图表
	f, err := os.Create(filename)
	if err != nil {
		return "", fmt.Errorf("创建图表文件失败: %w", err)
	}
	defer f.Close()

	if err := graph.Render(chart.PNG, f); err != nil {
		return "", fmt.Errorf("failed to render chart: %w", err)
	}

	return filename, nil
}

// ExportStatsChart 导出统计图表（柱状图）
// direction: both(全部)/rx(下载)/tx(上传)
func (e *Exporter) ExportStatsChart(stats []models.TrafficStats, direction string) (string, error) {
	if len(stats) == 0 {
		return "", fmt.Errorf("no statistics data")
	}

	// 按流量大小排序（从大到小）
	sortedStats := make([]models.TrafficStats, len(stats))
	copy(sortedStats, stats)

	switch direction {
	case "rx":
		// 按下载流量排序
		for i := 0; i < len(sortedStats)-1; i++ {
			for j := i + 1; j < len(sortedStats); j++ {
				if sortedStats[j].RXBytes > sortedStats[i].RXBytes {
					sortedStats[i], sortedStats[j] = sortedStats[j], sortedStats[i]
				}
			}
		}
	case "tx":
		// 按上传流量排序
		for i := 0; i < len(sortedStats)-1; i++ {
			for j := i + 1; j < len(sortedStats); j++ {
				if sortedStats[j].TXBytes > sortedStats[i].TXBytes {
					sortedStats[i], sortedStats[j] = sortedStats[j], sortedStats[i]
				}
			}
		}
	default:
		// 按总流量排序
		for i := 0; i < len(sortedStats)-1; i++ {
			for j := i + 1; j < len(sortedStats); j++ {
				if sortedStats[j].TotalBytes > sortedStats[i].TotalBytes {
					sortedStats[i], sortedStats[j] = sortedStats[j], sortedStats[i]
				}
			}
		}
	}

	// 先找出最大流量值,决定使用什么单位
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

	// 如果最大值小于1GB,使用MB作为单位;否则使用GB
	useMB := maxBytes < 1024*1024*1024
	var divisor float64
	var unitLabel string
	if useMB {
		divisor = 1024 * 1024 // MB
		unitLabel = "MB"
	} else {
		divisor = 1024 * 1024 * 1024 // GB
		unitLabel = "GB"
	}

	// 准备柱状图数据
	bars := make([]chart.Value, 0)

	// 为每个VM创建柱状条（根据方向参数决定显示哪些柱子）
	for _, stat := range sortedStats {
		var bytes uint64
		var fillColor, strokeColor drawing.Color

		switch direction {
		case "both":
			bytes = stat.TotalBytes
			fillColor = drawing.Color{R: 255, G: 99, B: 132, A: 255} // 完全不透明的红色
			strokeColor = drawing.Color{R: 220, G: 50, B: 90, A: 255}
		case "rx":
			bytes = stat.RXBytes
			fillColor = drawing.Color{R: 54, G: 162, B: 235, A: 255} // 完全不透明的蓝色
			strokeColor = drawing.Color{R: 30, G: 130, B: 200, A: 255}
		case "tx":
			bytes = stat.TXBytes
			fillColor = drawing.Color{R: 75, G: 192, B: 192, A: 255} // 完全不透明的青色
			strokeColor = drawing.Color{R: 40, G: 160, B: 160, A: 255}
		}

		value := float64(bytes) / divisor
		var label string

		// 根据数值大小调整显示精度
		if useMB {
			if value >= 100 {
				label = fmt.Sprintf("VM%d\n%.1f %s", stat.VMID, value, unitLabel)
			} else if value >= 10 {
				label = fmt.Sprintf("VM%d\n%.2f %s", stat.VMID, value, unitLabel)
			} else {
				label = fmt.Sprintf("VM%d\n%.3f %s", stat.VMID, value, unitLabel)
			}
		} else {
			if value >= 10 {
				label = fmt.Sprintf("VM%d\n%.2f %s", stat.VMID, value, unitLabel)
			} else {
				label = fmt.Sprintf("VM%d\n%.3f %s", stat.VMID, value, unitLabel)
			}
		}

		bars = append(bars, chart.Value{
			Label: label,
			Value: value,
			Style: chart.Style{
				FillColor:   fillColor,
				StrokeColor: strokeColor,
				StrokeWidth: 2,
			},
		})
	}

	// 现代化配色方案 - 网格线使用合适的颜色和透明度
	colorGrid := drawing.Color{R: 220, G: 220, B: 220, A: 120} // 半透明网格线,清晰但不遮挡
	colorBackground := drawing.Color{R: 250, G: 250, B: 250, A: 255}

	// 根据方向生成标题
	var title string
	switch direction {
	case "rx":
		title = "VM Traffic Statistics - Download (RX)"
	case "tx":
		title = "VM Traffic Statistics - Upload (TX)"
	default:
		title = "VM Traffic Statistics Summary"
	}

	// Y轴标签
	yAxisName := fmt.Sprintf("Traffic (%s)", unitLabel)

	// 根据VM数量动态调整柱子宽度和图表宽度
	vmCount := len(bars)
	barWidth := 80 // 增加柱子宽度,让柱子更醒目
	if vmCount > 10 {
		barWidth = 60
	} else if vmCount > 20 {
		barWidth = 50
	}

	// 动态计算图表宽度,确保有足够空间
	chartWidth := vmCount*(barWidth+20) + 200
	if chartWidth < 1200 {
		chartWidth = 1200
	}
	if chartWidth > 3000 {
		chartWidth = 3000
		barWidth = (chartWidth-200)/vmCount - 20
	}

	// 创建柱状图
	graph := chart.BarChart{
		Title: title,
		TitleStyle: chart.Style{
			FontSize:  24,
			FontColor: drawing.Color{R: 44, G: 62, B: 80, A: 255},
			Padding:   chart.Box{Top: 15, Bottom: 15},
		},
		Width:  chartWidth,
		Height: 700, // 减小高度,从900降到700
		Background: chart.Style{
			FillColor: colorBackground,
			Padding:   chart.Box{Top: 50, Left: 100, Right: 50, Bottom: 50}, // 减小底部内边距
		},
		Canvas: chart.Style{
			FillColor: drawing.Color{R: 255, G: 255, B: 255, A: 255},
		},
		XAxis: chart.Style{
			FontSize:    12,
			FontColor:   drawing.Color{R: 80, G: 80, B: 80, A: 255},
			StrokeColor: drawing.Color{R: 180, G: 180, B: 180, A: 255},
			StrokeWidth: 1.5,
		},
		YAxis: chart.YAxis{
			Name: yAxisName,
			NameStyle: chart.Style{
				FontSize:  16,
				FontColor: drawing.Color{R: 52, G: 73, B: 94, A: 255},
			},
			Style: chart.Style{
				FontSize:    13,
				FontColor:   drawing.Color{R: 80, G: 80, B: 80, A: 255},
				StrokeColor: drawing.Color{R: 180, G: 180, B: 180, A: 255},
				StrokeWidth: 1.5,
			},
			GridMajorStyle: chart.Style{
				StrokeColor: colorGrid,
				StrokeWidth: 1.0, // 适中的网格线宽度
			},
		},
		BarWidth: barWidth,
		Bars:     bars,
	}

	// 生成文件名（包含方向信息）
	timestamp := time.Now().Format("20060102_150405")
	var filename string
	if direction == "both" {
		filename = filepath.Join(e.exportPath, fmt.Sprintf("traffic_stats_%s.png", timestamp))
	} else {
		filename = filepath.Join(e.exportPath, fmt.Sprintf("traffic_stats_%s_%s.png", direction, timestamp))
	}

	// 保存图表
	f, err := os.Create(filename)
	if err != nil {
		return "", fmt.Errorf("failed to create chart file: %w", err)
	}
	defer f.Close()

	if err := graph.Render(chart.PNG, f); err != nil {
		return "", fmt.Errorf("failed to render chart: %w", err)
	}

	return filename, nil
}
