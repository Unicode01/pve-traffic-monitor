package models

import "time"

// Config 主配置结构
type Config struct {
	PVE     PVEConfig     `json:"pve"`
	Monitor MonitorConfig `json:"monitor"`
	Storage StorageConfig `json:"storage"`
	Rules   []Rule        `json:"rules"`
	API     APIConfig     `json:"api"`
}

// PVEConfig PVE 连接配置（使用API Token认证）
type PVEConfig struct {
	Host           string `json:"host"`             // PVE主机地址（默认 localhost）
	Port           int    `json:"port"`             // PVE端口（默认 8006）
	Node           string `json:"node"`             // 节点名称
	APITokenID     string `json:"api_token_id"`     // API Token ID (格式: user@realm!tokenid)
	APITokenSecret string `json:"api_token_secret"` // API Token Secret (UUID格式)
}

// MonitorConfig 监控配置
type MonitorConfig struct {
	IntervalSeconds   int    `json:"interval_seconds"`
	ExportPath        string `json:"export_path"`
	IncludeTemplates  bool   `json:"include_templates,omitempty"`   // 是否包含模板虚拟机（默认 false）
	DataRetentionDays int    `json:"data_retention_days,omitempty"` // 数据保留天数（0=永久保留，默认90天）
}

// Rule 流量规则
type Rule struct {
	Name             string   `json:"name"`
	Enabled          bool     `json:"enabled"`
	Period           string   `json:"period"`                      // hour, day, month
	UseCreationTime  bool     `json:"use_creation_time,omitempty"` // 是否使用虚拟机创建时间作为周期基准
	TrafficDirection string   `json:"traffic_direction,omitempty"` // both, upload, download (默认 both)
	LimitGB          float64  `json:"limit_gb"`
	Action           string   `json:"action"`                  // shutdown, stop, disconnect, rate_limit
	ForceStop        bool     `json:"force_stop,omitempty"`    // 是否强制停止（仅当 action=shutdown 时有效）
	RateLimitMB      float64  `json:"rate_limit_mb,omitempty"` // 限速值 MB/s（用于 rate_limit，支持小数）
	VMIDs            []int    `json:"vm_ids"`
	VMTags           []string `json:"vm_tags"`
	ExcludeVMIDs     []int    `json:"exclude_vm_ids"`
}

// StorageConfig 存储配置
type StorageConfig struct {
	Type string `json:"type"` // 存储类型: file, mysql, postgresql, sqlite
	// 文件存储配置
	FilePath string `json:"file_path,omitempty"` // 文件存储路径(type=file时使用)
	// 数据库存储配置
	DSN             string `json:"dsn,omitempty"`               // 数据库连接字符串
	MaxOpenConns    int    `json:"max_open_conns,omitempty"`    // 最大打开连接数(默认10)
	MaxIdleConns    int    `json:"max_idle_conns,omitempty"`    // 最大空闲连接数(默认5)
	ConnMaxLifetime int    `json:"conn_max_lifetime,omitempty"` // 连接最大生命周期(秒,默认3600)
}

// APIConfig API 服务器配置
type APIConfig struct {
	Enabled bool   `json:"enabled"` // 是否启用 API 服务器
	Host    string `json:"host"`    // API 监听地址
	Port    int    `json:"port"`    // API 监听端口
}

// VMInfo 虚拟机信息
type VMInfo struct {
	VMID         int       `json:"vmid"`
	Name         string    `json:"name"`
	Status       string    `json:"status"`
	Tags         []string  `json:"tags"`
	MatchedRules []string  `json:"matched_rules"` // 匹配的规则名称列表
	NetworkRX    uint64    `json:"netrx"`         // 接收字节数
	NetworkTX    uint64    `json:"nettx"`         // 发送字节数
	LastUpdated  time.Time `json:"last_updated"`
	CreationTime time.Time `json:"creation_time"` // 虚拟机创建时间
	Template     bool      `json:"template"`      // 是否为模板虚拟机
}

// IsTemplate 检查是否为模板虚拟机
func (v *VMInfo) IsTemplate() bool {
	return v.Template
}

// IsMonitorable 检查虚拟机是否可监控（非模板）
func (v *VMInfo) IsMonitorable() bool {
	return !v.Template
}

// TrafficRecord 流量记录
type TrafficRecord struct {
	VMID       int       `json:"vmid"`
	Timestamp  time.Time `json:"timestamp"`
	RXBytes    uint64    `json:"rx_bytes"`
	TXBytes    uint64    `json:"tx_bytes"`
	TotalBytes uint64    `json:"total_bytes"`
}

// TrafficStats 流量统计
type TrafficStats struct {
	VMID       int       `json:"vmid"`
	Name       string    `json:"name"` // 虚拟机名称
	Period     string    `json:"period"`
	StartTime  time.Time `json:"start_time"`
	EndTime    time.Time `json:"end_time"`
	Direction  string    `json:"direction"` // both, upload, download
	RXBytes    uint64    `json:"rx_bytes"`  // 接收字节数
	TXBytes    uint64    `json:"tx_bytes"`  // 发送字节数
	TotalBytes uint64    `json:"total_bytes"`
	TotalGB    float64   `json:"total_gb"`
}

// AggregatedPoint 聚合的流量数据点
type AggregatedPoint struct {
	Timestamp  time.Time `json:"timestamp"`
	RXBytes    uint64    `json:"rx_bytes"`
	TXBytes    uint64    `json:"tx_bytes"`
	TotalBytes uint64    `json:"total_bytes"`
}

// ActionLog 操作日志
type ActionLog struct {
	VMID      int       `json:"vmid"`
	RuleName  string    `json:"rule_name"`
	Action    string    `json:"action"`
	Reason    string    `json:"reason"`
	Timestamp time.Time `json:"timestamp"`
	Success   bool      `json:"success"`
	Error     string    `json:"error,omitempty"`
}
