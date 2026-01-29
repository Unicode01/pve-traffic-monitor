package models

// ColorScheme 统一的配色方案
type ColorScheme struct {
	Primary    string
	Success    string
	Warning    string
	Danger     string
	Info       string
	Download   string
	Upload     string
	Total      string
	Background string
	Text       string
	Border     string
	GridLine   string
}

// LightTheme 亮色主题
var LightTheme = ColorScheme{
	Primary:    "#3498db", // 主色调 - 蓝色
	Success:    "#27ae60", // 成功 - 绿色
	Warning:    "#f39c12", // 警告 - 橙色
	Danger:     "#e74c3c", // 危险 - 红色
	Info:       "#34495e", // 信息 - 深灰
	Download:   "#36a2eb", // 下载 - 蓝色 (rgb(54, 162, 235))
	Upload:     "#4bc0c0", // 上传 - 青色 (rgb(75, 192, 192))
	Total:      "#ff6384", // 总计 - 红色 (rgb(255, 99, 132))
	Background: "#ffffff", // 背景 - 白色
	Text:       "#2c3e50", // 文字 - 深蓝灰
	Border:     "#dcdfe6", // 边框 - 浅灰
	GridLine:   "#e6e6e6", // 网格线 - 浅灰
}

// DarkTheme 暗色主题
var DarkTheme = ColorScheme{
	Primary:    "#409eff", // 主色调 - 亮蓝色
	Success:    "#67c23a", // 成功 - 亮绿色
	Warning:    "#e6a23c", // 警告 - 亮橙色
	Danger:     "#f56c6c", // 危险 - 亮红色
	Info:       "#909399", // 信息 - 中灰
	Download:   "#5cb3ff", // 下载 - 亮蓝色
	Upload:     "#5cd3d3", // 上传 - 亮青色
	Total:      "#ff8fa3", // 总计 - 亮红色
	Background: "#1f1f1f", // 背景 - 深灰
	Text:       "#e4e7ed", // 文字 - 浅灰
	Border:     "#4c4d4f", // 边框 - 灰色
	GridLine:   "#3a3a3a", // 网格线 - 深灰
}

// ChartColors ECharts 图表配色方案
type ChartColors struct {
	Download   string   // 下载颜色
	Upload     string   // 上传颜色
	Total      string   // 总计颜色
	BarColors  []string // 柱状图颜色组
	LineColors []string // 折线图颜色组
	Background string   // 背景色
	TextColor  string   // 文字颜色
	AxisLine   string   // 坐标轴线颜色
	SplitLine  string   // 分割线颜色
	TooltipBg  string   // 提示框背景
}

// GetChartColors 获取图表配色（支持主题）
func GetChartColors(isDark bool) ChartColors {
	if isDark {
		return ChartColors{
			Download:   "#5cb3ff",
			Upload:     "#5cd3d3",
			Total:      "#ff8fa3",
			BarColors:  []string{"#5cb3ff", "#5cd3d3", "#ff8fa3", "#a0d911", "#ffc53d"},
			LineColors: []string{"#5cb3ff", "#5cd3d3", "#ff8fa3"},
			Background: "#1f1f1f",
			TextColor:  "#e4e7ed",
			AxisLine:   "#4c4d4f",
			SplitLine:  "#3a3a3a",
			TooltipBg:  "#2d2d2d",
		}
	}
	return ChartColors{
		Download:   "#36a2eb",
		Upload:     "#4bc0c0",
		Total:      "#ff6384",
		BarColors:  []string{"#36a2eb", "#4bc0c0", "#ff6384", "#8bc34a", "#ffc107"},
		LineColors: []string{"#36a2eb", "#4bc0c0", "#ff6384"},
		Background: "#ffffff",
		TextColor:  "#2c3e50",
		AxisLine:   "#dcdfe6",
		SplitLine:  "#e6e6e6",
		TooltipBg:  "#ffffff",
	}
}
