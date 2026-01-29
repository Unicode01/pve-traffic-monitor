package storage

import (
	"pve-traffic-monitor/pkg/models"
	"time"
)

// Interface 存储接口定义
// 所有存储后端都需要实现这个接口
type Interface interface {
	// SaveTrafficRecord 保存流量记录
	SaveTrafficRecord(record models.TrafficRecord) error

	// GetTrafficRecords 获取流量记录
	GetTrafficRecords(vmid int, startTime, endTime time.Time) ([]models.TrafficRecord, error)

	// CalculateTrafficStats 计算流量统计
	CalculateTrafficStats(vmid int, period string) (*models.TrafficStats, error)

	// CalculateTrafficStatsWithTime 使用指定时间计算流量统计
	CalculateTrafficStatsWithTime(vmid int, period string, creationTime time.Time, useCreationTime bool) (*models.TrafficStats, error)

	// CalculateTrafficStatsWithDirection 使用指定方向和时间计算流量统计
	CalculateTrafficStatsWithDirection(vmid int, period string, creationTime time.Time, useCreationTime bool, direction string) (*models.TrafficStats, error)

	// CalculateTrafficStatsWithTimeRange 使用自定义时间范围计算流量统计
	CalculateTrafficStatsWithTimeRange(vmid int, startTime, endTime time.Time, direction string) (*models.TrafficStats, error)

	// SaveActionLog 保存操作日志
	SaveActionLog(log models.ActionLog) error

	// GetActionLogs 获取操作日志
	GetActionLogs(startTime, endTime time.Time) ([]models.ActionLog, error)

	// SaveVMState 保存虚拟机状态
	SaveVMState(vmid int, state map[string]interface{}) error

	// LoadVMState 加载虚拟机状态
	LoadVMState(vmid int) (map[string]interface{}, error)

	// CleanupOldData 清理旧数据
	CleanupOldData(retentionDays int) error

	// GetTotalRecordCount 获取总采样点数
	GetTotalRecordCount() (int64, error)

	// DeleteRecordsInRange 删除指定时间范围内的记录
	// vmid=0 表示删除所有VM的记录
	DeleteRecordsInRange(vmid int, startTime, endTime time.Time) (int64, error)

	// CountRecordsInRange 统计指定时间范围内的记录数
	// vmid=0 表示统计所有VM的记录
	CountRecordsInRange(vmid int, startTime, endTime time.Time) (int64, error)

	// DeleteRecordsBefore 删除指定日期之前的所有记录
	DeleteRecordsBefore(beforeTime time.Time) (int64, error)

	// CountRecordsBefore 统计指定日期之前的记录数
	CountRecordsBefore(beforeTime time.Time) (int64, error)

	// Close 关闭存储连接
	Close() error
}
