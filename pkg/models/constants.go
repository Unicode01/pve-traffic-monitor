package models

import "time"

// 常量定义 - 避免magic number
const (
	// 字节单位转换
	BytesPerKB = 1024
	BytesPerMB = 1024 * 1024
	BytesPerGB = 1024 * 1024 * 1024

	// 缓存配置
	DefaultCacheTTL      = 5 * time.Minute
	CacheCleanupInterval = 5 * time.Minute

	// 性能配置
	MaxRecentRequests = 100
	MaxWorkers        = 10

	// 时间格式
	TimeFormatMinute = "2006-01-02 15:04"
	TimeFormatHour   = "2006-01-02 15:00"
	TimeFormatDay    = "2006-01-02"
	TimeFormatMonth  = "2006-01"

	// 流量方向
	DirectionBoth     = "both"
	DirectionUpload   = "upload"
	DirectionTX       = "tx"
	DirectionDownload = "download"
	DirectionRX       = "rx"

	// 周期类型
	PeriodMinute = "minute"
	PeriodHour   = "hour"
	PeriodDay    = "day"
	PeriodMonth  = "month"

	// 操作类型
	ActionShutdown   = "shutdown"
	ActionStop       = "stop"
	ActionDisconnect = "disconnect"
	ActionRateLimit  = "rate_limit"

	// 标签前缀
	TagTrafficLimit      = "traffic-limit"
	TagTrafficShutdown   = "traffic-exceeded-shutdown"
	TagTrafficDisconnect = "traffic-exceeded-disconnected"
	TagTrafficLimited    = "traffic-exceeded-limited"
)
