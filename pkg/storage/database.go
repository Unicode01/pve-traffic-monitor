package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"pve-traffic-monitor/pkg/models"
	"pve-traffic-monitor/pkg/utils"
	"sort"
	"time"

	_ "github.com/go-sql-driver/mysql" // MySQL driver
	_ "github.com/lib/pq"              // PostgreSQL driver
	_ "github.com/mattn/go-sqlite3"    // SQLite driver
)

// DatabaseStorage 数据库存储管理器(实现 Interface 接口)
type DatabaseStorage struct {
	db         *sql.DB
	driverType string // mysql, postgres, sqlite3
}

// NewDatabaseStorage 创建新的数据库存储管理器
func NewDatabaseStorage(driverType, dsn string, maxOpenConns, maxIdleConns, connMaxLifetime int) (*DatabaseStorage, error) {
	// 设置默认值
	if maxOpenConns <= 0 {
		maxOpenConns = 10
	}
	if maxIdleConns <= 0 {
		maxIdleConns = 5
	}
	if connMaxLifetime <= 0 {
		connMaxLifetime = 3600
	}

	// 连接数据库
	db, err := sql.Open(driverType, dsn)
	if err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	// 设置连接池参数
	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxIdleConns)
	db.SetConnMaxLifetime(time.Duration(connMaxLifetime) * time.Second)

	// 测试连接
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("数据库连接测试失败: %w", err)
	}

	storage := &DatabaseStorage{
		db:         db,
		driverType: driverType,
	}

	// 初始化表结构
	if err := storage.initTables(); err != nil {
		db.Close()
		return nil, fmt.Errorf("初始化数据库表失败: %w", err)
	}

	return storage, nil
}

// initTables 初始化数据库表
func (s *DatabaseStorage) initTables() error {
	// 流量记录表
	trafficRecordsTable := `
	CREATE TABLE IF NOT EXISTS traffic_records (
		id INTEGER PRIMARY KEY ` + s.autoIncrement() + `,
		vmid INTEGER NOT NULL,
		timestamp TIMESTAMP NOT NULL,
		rx_bytes BIGINT NOT NULL,
		tx_bytes BIGINT NOT NULL,
		total_bytes BIGINT NOT NULL,
		INDEX idx_vmid_timestamp (vmid, timestamp)
	)` + s.engine()

	// 操作日志表
	actionLogsTable := `
	CREATE TABLE IF NOT EXISTS action_logs (
		id INTEGER PRIMARY KEY ` + s.autoIncrement() + `,
		vmid INTEGER NOT NULL,
		rule_name VARCHAR(255) NOT NULL,
		action VARCHAR(50) NOT NULL,
		reason TEXT,
		timestamp TIMESTAMP NOT NULL,
		success BOOLEAN NOT NULL,
		error TEXT,
		INDEX idx_timestamp (timestamp)
	)` + s.engine()

	// VM状态表
	vmStatesTable := `
	CREATE TABLE IF NOT EXISTS vm_states (
		vmid INTEGER PRIMARY KEY,
		state_data TEXT NOT NULL,
		updated_at TIMESTAMP NOT NULL
	)` + s.engine()

	tables := []string{trafficRecordsTable, actionLogsTable, vmStatesTable}

	for _, table := range tables {
		if _, err := s.db.Exec(table); err != nil {
			return fmt.Errorf("创建表失败: %w", err)
		}
	}

	return nil
}

// autoIncrement 返回自增主键语法
func (s *DatabaseStorage) autoIncrement() string {
	switch s.driverType {
	case "mysql":
		return "AUTO_INCREMENT"
	case "postgres":
		return "" // PostgreSQL 使用 SERIAL
	case "sqlite3":
		return "AUTOINCREMENT"
	default:
		return ""
	}
}

// engine 返回存储引擎语法
func (s *DatabaseStorage) engine() string {
	if s.driverType == "mysql" {
		return " ENGINE=InnoDB DEFAULT CHARSET=utf8mb4"
	}
	return ""
}

// SaveTrafficRecord 保存流量记录
func (s *DatabaseStorage) SaveTrafficRecord(record models.TrafficRecord) error {
	query := s.buildQuery(`INSERT INTO traffic_records (vmid, timestamp, rx_bytes, tx_bytes, total_bytes) 
			  VALUES (?, ?, ?, ?, ?)`, 5)

	_, err := s.db.Exec(query, record.VMID, record.Timestamp, record.RXBytes, record.TXBytes, record.TotalBytes)
	if err != nil {
		return fmt.Errorf("保存流量记录失败: %w", err)
	}

	return nil
}

// GetTrafficRecords 获取流量记录
func (s *DatabaseStorage) GetTrafficRecords(vmid int, startTime, endTime time.Time) ([]models.TrafficRecord, error) {
	query := s.buildQuery(`SELECT vmid, timestamp, rx_bytes, tx_bytes, total_bytes 
			  FROM traffic_records 
			  WHERE vmid = ? AND timestamp >= ? AND timestamp <= ?
			  ORDER BY timestamp ASC`, 3)

	rows, err := s.db.Query(query, vmid, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("查询流量记录失败: %w", err)
	}
	defer rows.Close()

	var records []models.TrafficRecord
	for rows.Next() {
		var record models.TrafficRecord
		if err := rows.Scan(&record.VMID, &record.Timestamp, &record.RXBytes, &record.TXBytes, &record.TotalBytes); err != nil {
			return nil, fmt.Errorf("扫描流量记录失败: %w", err)
		}
		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("迭代流量记录失败: %w", err)
	}

	return records, nil
}

// CalculateTrafficStats 计算流量统计
func (s *DatabaseStorage) CalculateTrafficStats(vmid int, period string) (*models.TrafficStats, error) {
	return s.CalculateTrafficStatsWithDirection(vmid, period, time.Time{}, false, models.DirectionBoth)
}

// CalculateTrafficStatsWithTime 使用指定时间计算流量统计
func (s *DatabaseStorage) CalculateTrafficStatsWithTime(vmid int, period string, creationTime time.Time, useCreationTime bool) (*models.TrafficStats, error) {
	return s.CalculateTrafficStatsWithDirection(vmid, period, creationTime, useCreationTime, models.DirectionBoth)
}

// CalculateTrafficStatsWithDirection 使用指定方向和时间计算流量统计
func (s *DatabaseStorage) CalculateTrafficStatsWithDirection(vmid int, period string, creationTime time.Time, useCreationTime bool, direction string) (*models.TrafficStats, error) {
	now := time.Now()
	var startTime time.Time

	if useCreationTime && !creationTime.IsZero() {
		// 使用基于创建时间的周期计算
		startTime = s.calculatePeriodStart(period, creationTime, now)
	} else {
		// 使用固定周期
		switch period {
		case models.PeriodMinute:
			// 确保有2个数据能够统计差值
			startTime = now.Add(-2 * time.Minute)
		case models.PeriodHour:
			startTime = time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())
		case models.PeriodDay:
			startTime = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		case models.PeriodMonth:
			startTime = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		default:
			return nil, fmt.Errorf("不支持的时间周期: %s", period)
		}
	}

	records, err := s.GetTrafficRecords(vmid, startTime, now)
	if err != nil {
		return nil, err
	}

	// 使用公共辅助函数构建统计结果
	return buildTrafficStats(vmid, period, startTime, now, direction, records), nil
}

// CalculateTrafficStatsWithTimeRange 使用自定义时间范围计算流量统计
func (s *DatabaseStorage) CalculateTrafficStatsWithTimeRange(vmid int, startTime, endTime time.Time, direction string) (*models.TrafficStats, error) {
	records, err := s.GetTrafficRecords(vmid, startTime, endTime)
	if err != nil {
		return nil, err
	}

	// 使用公共辅助函数构建统计结果（避免重复代码）
	return buildTrafficStats(vmid, "custom", startTime, endTime, direction, records), nil
}

// SaveActionLog 保存操作日志
func (s *DatabaseStorage) SaveActionLog(log models.ActionLog) error {
	query := s.buildQuery(`INSERT INTO action_logs (vmid, rule_name, action, reason, timestamp, success, error) 
			  VALUES (?, ?, ?, ?, ?, ?, ?)`, 7)

	_, err := s.db.Exec(query, log.VMID, log.RuleName, log.Action, log.Reason, log.Timestamp, log.Success, log.Error)
	if err != nil {
		return fmt.Errorf("保存操作日志失败: %w", err)
	}

	return nil
}

// GetActionLogs 获取操作日志
func (s *DatabaseStorage) GetActionLogs(startTime, endTime time.Time) ([]models.ActionLog, error) {
	query := s.buildQuery(`SELECT vmid, rule_name, action, reason, timestamp, success, error 
			  FROM action_logs 
			  WHERE timestamp >= ? AND timestamp <= ?
			  ORDER BY timestamp ASC`, 2)

	rows, err := s.db.Query(query, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("查询操作日志失败: %w", err)
	}
	defer rows.Close()

	var logs []models.ActionLog
	for rows.Next() {
		var log models.ActionLog
		var errorMsg sql.NullString
		if err := rows.Scan(&log.VMID, &log.RuleName, &log.Action, &log.Reason, &log.Timestamp, &log.Success, &errorMsg); err != nil {
			return nil, fmt.Errorf("扫描操作日志失败: %w", err)
		}
		if errorMsg.Valid {
			log.Error = errorMsg.String
		}
		logs = append(logs, log)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("迭代操作日志失败: %w", err)
	}

	return logs, nil
}

// SaveVMState 保存虚拟机状态
func (s *DatabaseStorage) SaveVMState(vmid int, state map[string]interface{}) error {
	stateData, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("序列化虚拟机状态失败: %w", err)
	}

	query := `INSERT INTO vm_states (vmid, state_data, updated_at) 
			  VALUES (?, ?, ?) 
			  ON DUPLICATE KEY UPDATE state_data = ?, updated_at = ?`

	if s.driverType == "postgres" {
		query = `INSERT INTO vm_states (vmid, state_data, updated_at) 
				 VALUES ($1, $2, $3)
				 ON CONFLICT (vmid) DO UPDATE 
				 SET state_data = $4, updated_at = $5`
	} else if s.driverType == "sqlite3" {
		query = `INSERT OR REPLACE INTO vm_states (vmid, state_data, updated_at) 
				 VALUES (?, ?, ?)`
	}

	now := time.Now()

	var err2 error
	if s.driverType == "sqlite3" {
		_, err2 = s.db.Exec(query, vmid, string(stateData), now)
	} else {
		_, err2 = s.db.Exec(query, vmid, string(stateData), now, string(stateData), now)
	}

	if err2 != nil {
		return fmt.Errorf("保存虚拟机状态失败: %w", err2)
	}

	return nil
}

// LoadVMState 加载虚拟机状态
func (s *DatabaseStorage) LoadVMState(vmid int) (map[string]interface{}, error) {
	query := `SELECT state_data FROM vm_states WHERE vmid = ?`

	if s.driverType == "postgres" {
		query = `SELECT state_data FROM vm_states WHERE vmid = $1`
	}

	var stateData string
	err := s.db.QueryRow(query, vmid).Scan(&stateData)
	if err != nil {
		if err == sql.ErrNoRows {
			return map[string]interface{}{}, nil
		}
		return nil, fmt.Errorf("查询虚拟机状态失败: %w", err)
	}

	var state map[string]interface{}
	if err := json.Unmarshal([]byte(stateData), &state); err != nil {
		return nil, fmt.Errorf("解析虚拟机状态失败: %w", err)
	}

	return state, nil
}

// CleanupOldData 清理旧数据
func (s *DatabaseStorage) CleanupOldData(retentionDays int) error {
	if retentionDays <= 0 {
		return nil // 不清理
	}

	cutoffTime := time.Now().AddDate(0, 0, -retentionDays)

	query := `DELETE FROM traffic_records WHERE timestamp < ?`
	if s.driverType == "postgres" {
		query = `DELETE FROM traffic_records WHERE timestamp < $1`
	}

	result, err := s.db.Exec(query, cutoffTime)
	if err != nil {
		return fmt.Errorf("清理旧流量记录失败: %w", err)
	}

	deletedCount, _ := result.RowsAffected()
	if deletedCount > 0 {
		utils.DebugLog("数据清理完成: 删除 %d 条过期记录 (保留天数: %d)", deletedCount, retentionDays)
	}

	return nil
}

// Close 关闭数据库连接
func (s *DatabaseStorage) Close() error {
	return s.db.Close()
}

// GetTotalRecordCount 获取总采样点数（数据库实现）
func (s *DatabaseStorage) GetTotalRecordCount() (int64, error) {
	query := `SELECT COUNT(*) FROM traffic_records`

	var count int64
	err := s.db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("查询总记录数失败: %w", err)
	}

	return count, nil
}

// calculatePeriodStart 基于创建时间计算周期开始时间
func (s *DatabaseStorage) calculatePeriodStart(period string, creationTime, now time.Time) time.Time {
	creation := creationTime

	switch period {
	case models.PeriodHour:
		hoursSinceCreation := int(now.Sub(creation).Hours())
		return creation.Add(time.Duration(hoursSinceCreation) * time.Hour)

	case models.PeriodDay:
		daysSinceCreation := int(now.Sub(creation).Hours() / 24)
		return creation.AddDate(0, 0, daysSinceCreation)

	case models.PeriodMonth:
		creationDay := creation.Day()
		creationHour := creation.Hour()

		monthsSinceCreation := (now.Year()-creation.Year())*12 + int(now.Month()-creation.Month())

		if now.Day() < creationDay || (now.Day() == creationDay && now.Hour() < creationHour) {
			monthsSinceCreation--
		}

		periodStart := creation.AddDate(0, monthsSinceCreation, 0)
		return time.Date(periodStart.Year(), periodStart.Month(), creationDay,
			creationHour, 0, 0, 0, periodStart.Location())

	default:
		return time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())
	}
}

// GetActionLogsByVMID 获取指定VM的操作日志(辅助方法)
func (s *DatabaseStorage) GetActionLogsByVMID(vmid int, startTime, endTime time.Time) ([]models.ActionLog, error) {
	query := `SELECT vmid, rule_name, action, reason, timestamp, success, error 
			  FROM action_logs 
			  WHERE vmid = ? AND timestamp >= ? AND timestamp <= ?
			  ORDER BY timestamp ASC`

	if s.driverType == "postgres" {
		query = `SELECT vmid, rule_name, action, reason, timestamp, success, error 
				 FROM action_logs 
				 WHERE vmid = $1 AND timestamp >= $2 AND timestamp <= $3
				 ORDER BY timestamp ASC`
	}

	rows, err := s.db.Query(query, vmid, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("查询操作日志失败: %w", err)
	}
	defer rows.Close()

	var logs []models.ActionLog
	for rows.Next() {
		var log models.ActionLog
		var errorMsg sql.NullString
		if err := rows.Scan(&log.VMID, &log.RuleName, &log.Action, &log.Reason, &log.Timestamp, &log.Success, &errorMsg); err != nil {
			return nil, fmt.Errorf("扫描操作日志失败: %w", err)
		}
		if errorMsg.Valid {
			log.Error = errorMsg.String
		}
		logs = append(logs, log)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("迭代操作日志失败: %w", err)
	}

	// 按时间排序
	sort.Slice(logs, func(i, j int) bool {
		return logs[i].Timestamp.Before(logs[j].Timestamp)
	})

	return logs, nil
}

// DeleteRecordsInRange 删除指定时间范围内的记录
func (s *DatabaseStorage) DeleteRecordsInRange(vmid int, startTime, endTime time.Time) (int64, error) {
	var result sql.Result
	var err error

	if vmid == 0 {
		// 删除所有VM在时间范围内的记录
		if s.driverType == "postgres" {
			result, err = s.db.Exec(
				"DELETE FROM traffic_records WHERE timestamp >= $1 AND timestamp <= $2",
				startTime, endTime,
			)
		} else {
			result, err = s.db.Exec(
				"DELETE FROM traffic_records WHERE timestamp >= ? AND timestamp <= ?",
				startTime, endTime,
			)
		}
	} else {
		// 删除指定VM在时间范围内的记录
		if s.driverType == "postgres" {
			result, err = s.db.Exec(
				"DELETE FROM traffic_records WHERE vmid = $1 AND timestamp >= $2 AND timestamp <= $3",
				vmid, startTime, endTime,
			)
		} else {
			result, err = s.db.Exec(
				"DELETE FROM traffic_records WHERE vmid = ? AND timestamp >= ? AND timestamp <= ?",
				vmid, startTime, endTime,
			)
		}
	}

	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// CountRecordsInRange 统计指定时间范围内的记录数
func (s *DatabaseStorage) CountRecordsInRange(vmid int, startTime, endTime time.Time) (int64, error) {
	var count int64
	var err error

	if vmid == 0 {
		// 统计所有VM在时间范围内的记录
		if s.driverType == "postgres" {
			err = s.db.QueryRow(
				"SELECT COUNT(*) FROM traffic_records WHERE timestamp >= $1 AND timestamp <= $2",
				startTime, endTime,
			).Scan(&count)
		} else {
			err = s.db.QueryRow(
				"SELECT COUNT(*) FROM traffic_records WHERE timestamp >= ? AND timestamp <= ?",
				startTime, endTime,
			).Scan(&count)
		}
	} else {
		// 统计指定VM在时间范围内的记录
		if s.driverType == "postgres" {
			err = s.db.QueryRow(
				"SELECT COUNT(*) FROM traffic_records WHERE vmid = $1 AND timestamp >= $2 AND timestamp <= $3",
				vmid, startTime, endTime,
			).Scan(&count)
		} else {
			err = s.db.QueryRow(
				"SELECT COUNT(*) FROM traffic_records WHERE vmid = ? AND timestamp >= ? AND timestamp <= ?",
				vmid, startTime, endTime,
			).Scan(&count)
		}
	}

	if err != nil {
		return 0, err
	}

	return count, nil
}

// DeleteRecordsBefore 删除指定日期之前的所有记录
func (s *DatabaseStorage) DeleteRecordsBefore(beforeTime time.Time) (int64, error) {
	var result sql.Result
	var err error

	if s.driverType == "postgres" {
		result, err = s.db.Exec(
			"DELETE FROM traffic_records WHERE timestamp < $1",
			beforeTime,
		)
	} else {
		result, err = s.db.Exec(
			"DELETE FROM traffic_records WHERE timestamp < ?",
			beforeTime,
		)
	}

	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// CountRecordsBefore 统计指定日期之前的记录数
func (s *DatabaseStorage) CountRecordsBefore(beforeTime time.Time) (int64, error) {
	var count int64
	var err error

	if s.driverType == "postgres" {
		err = s.db.QueryRow(
			"SELECT COUNT(*) FROM traffic_records WHERE timestamp < $1",
			beforeTime,
		).Scan(&count)
	} else {
		err = s.db.QueryRow(
			"SELECT COUNT(*) FROM traffic_records WHERE timestamp < ?",
			beforeTime,
		).Scan(&count)
	}

	if err != nil {
		return 0, err
	}

	return count, nil
}
