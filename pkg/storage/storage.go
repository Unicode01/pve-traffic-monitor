package storage

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"pve-traffic-monitor/pkg/models"
	"pve-traffic-monitor/pkg/utils"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// FileStorage 文件存储管理器(实现 Interface 接口)
type FileStorage struct {
	basePath      string
	recordCounter *RecordCounter // 记录计数器（用于快速统计）
}

// RecordCounter 记录计数器（带缓存）
type RecordCounter struct {
	mu           sync.RWMutex
	cachedCount  int64
	lastUpdate   time.Time
	cacheTTL     time.Duration
	counterFile  string
	needsRebuild bool
}

// NewFileStorage 创建新的文件存储管理器
func NewFileStorage(basePath string) (*FileStorage, error) {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("创建存储目录失败: %w", err)
	}

	counterFile := filepath.Join(basePath, ".record_count")

	fs := &FileStorage{
		basePath: basePath,
		recordCounter: &RecordCounter{
			counterFile:  counterFile,
			cacheTTL:     5 * time.Minute, // 缓存5分钟
			needsRebuild: false,
		},
	}

	// 尝试加载计数器文件
	if err := fs.recordCounter.load(); err != nil {
		// 如果加载失败，标记需要重建
		fs.recordCounter.needsRebuild = true
		utils.DebugLog("计数器文件不存在或损坏，将在后台重建")

		// 启动后台重建
		go fs.rebuildCounter()
	}

	return fs, nil
}

// Close 关闭文件存储(保存计数器并清理资源)
func (s *FileStorage) Close() error {
	// 在关闭前保存计数器，确保退出时数据准确
	if s.recordCounter != nil {
		if err := s.recordCounter.save(); err != nil {
			utils.DebugLog("保存计数器失败: %v", err)
		} else {
			utils.DebugLog("程序退出前已保存计数器: %d", s.recordCounter.cachedCount)
		}
	}
	return nil
}

// GetTotalRecordCount 获取总采样点数（优化版：使用缓存）
func (s *FileStorage) GetTotalRecordCount() (int64, error) {
	// 尝试从缓存获取
	if count, ok := s.recordCounter.get(); ok {
		return count, nil
	}

	// 缓存未命中，执行实际统计
	count, err := s.countRecordsActual()
	if err != nil {
		return 0, err
	}

	// 更新缓存和计数器文件
	s.recordCounter.set(count)
	s.recordCounter.save()

	return count, nil
}

// countRecordsActual 实际统计记录数（内部方法）
func (s *FileStorage) countRecordsActual() (int64, error) {
	var totalCount int64 = 0

	// 遍历所有VM目录
	entries, err := os.ReadDir(s.basePath)
	if err != nil {
		return 0, fmt.Errorf("读取存储目录失败: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "vm_") {
			continue
		}

		vmDir := filepath.Join(s.basePath, entry.Name())
		files, err := os.ReadDir(vmDir)
		if err != nil {
			continue
		}

		// 统计所有流量记录文件中的记录数
		for _, file := range files {
			if file.IsDir() {
				continue
			}

			name := file.Name()
			if !strings.HasPrefix(name, "traffic_") {
				continue
			}

			filePath := filepath.Join(vmDir, name)

			// 优化：使用更快的行数统计方法
			if strings.HasSuffix(name, ".jsonl") {
				count, err := s.countLinesInFile(filePath)
				if err == nil {
					totalCount += count
				}
			} else if strings.HasSuffix(name, ".json") {
				// JSON格式，解析数组长度
				data, err := os.ReadFile(filePath)
				if err != nil {
					continue
				}
				var records []models.TrafficRecord
				if err := json.Unmarshal(data, &records); err == nil {
					totalCount += int64(len(records))
				}
			}
		}
	}

	return totalCount, nil
}

// countLinesInFile 快速统计文件行数（优化版）
func (s *FileStorage) countLinesInFile(filePath string) (int64, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	var count int64 = 0
	scanner := bufio.NewScanner(file)

	// 增大缓冲区以提升性能
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		if len(strings.TrimSpace(scanner.Text())) > 0 {
			count++
		}
	}

	if err := scanner.Err(); err != nil {
		return 0, err
	}

	return count, nil
}

// rebuildCounter 后台重建计数器
func (s *FileStorage) rebuildCounter() {
	utils.DebugLog("[计数器] 开始后台重建...")

	count, err := s.countRecordsActual()
	if err != nil {
		utils.DebugLog("[计数器] 重建失败: %v", err)
		return
	}

	s.recordCounter.set(count)
	s.recordCounter.save()
	s.recordCounter.needsRebuild = false

	utils.DebugLog("[计数器] 重建完成，总记录数: %d", count)
}

// Storage 存储管理器(已弃用,保留以兼容旧代码)
// 推荐使用 FileStorage
type Storage = FileStorage

// NewStorage 创建新的存储管理器(已弃用,保留以兼容旧代码)
// 推荐使用 NewFileStorage
func NewStorage(basePath string) (*Storage, error) {
	return NewFileStorage(basePath)
}

// get 从缓存获取计数（如果有效）
func (c *RecordCounter) get() (int64, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 如果需要重建，返回未命中
	if c.needsRebuild {
		return 0, false
	}

	// 检查缓存是否过期
	if time.Since(c.lastUpdate) > c.cacheTTL {
		return 0, false
	}

	return c.cachedCount, true
}

// set 设置缓存计数
func (c *RecordCounter) set(count int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cachedCount = count
	c.lastUpdate = time.Now()
}

// increment 增加计数（保存记录时调用）
func (c *RecordCounter) increment() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cachedCount++
	c.lastUpdate = time.Now()
}

// load 从文件加载计数器
func (c *RecordCounter) load() error {
	data, err := os.ReadFile(c.counterFile)
	if err != nil {
		return err
	}

	count, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return err
	}

	c.mu.Lock()
	c.cachedCount = count
	c.lastUpdate = time.Now()
	c.mu.Unlock()

	return nil
}

// save 保存计数器到文件
func (c *RecordCounter) save() error {
	c.mu.RLock()
	count := c.cachedCount
	c.mu.RUnlock()

	data := []byte(fmt.Sprintf("%d\n", count))
	return os.WriteFile(c.counterFile, data, 0644)
}

// SaveTrafficRecord 保存流量记录（优化版：追加模式 + 计数器更新）
func (s *FileStorage) SaveTrafficRecord(record models.TrafficRecord) error {
	vmDir := filepath.Join(s.basePath, fmt.Sprintf("vm_%d", record.VMID))
	if err := os.MkdirAll(vmDir, 0755); err != nil {
		return fmt.Errorf("创建虚拟机目录失败: %w", err)
	}

	// 按日期组织文件（改为按天，减少单文件大小）
	dateStr := record.Timestamp.Format("2006-01-02")
	filename := filepath.Join(vmDir, fmt.Sprintf("traffic_%s.jsonl", dateStr))

	// 使用JSONL格式追加（每行一个JSON对象）
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("序列化流量记录失败: %w", err)
	}

	// 追加模式写入，避免读取整个文件
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("打开流量记录文件失败: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("写入流量记录失败: %w", err)
	}

	// 增加计数器
	s.recordCounter.increment()

	// 定期保存计数器（每100次保存一次，减少IO）
	// 注意：程序退出时Close()方法会保存最终值
	s.recordCounter.mu.RLock()
	count := s.recordCounter.cachedCount
	s.recordCounter.mu.RUnlock()

	if count%100 == 0 {
		go s.recordCounter.save()
	}

	return nil
}

// GetTrafficRecords 获取流量记录（优化版：支持JSONL格式）
func (s *FileStorage) GetTrafficRecords(vmid int, startTime, endTime time.Time) ([]models.TrafficRecord, error) {
	vmDir := filepath.Join(s.basePath, fmt.Sprintf("vm_%d", vmid))
	if _, err := os.Stat(vmDir); os.IsNotExist(err) {
		return []models.TrafficRecord{}, nil
	}

	var allRecords []models.TrafficRecord

	// 遍历可能的日期文件（改为按天）
	current := startTime
	for current.Before(endTime.AddDate(0, 0, 1)) {
		dateStr := current.Format("2006-01-02")

		// 尝试读取JSONL格式（新格式）
		jsonlFile := filepath.Join(vmDir, fmt.Sprintf("traffic_%s.jsonl", dateStr))
		if records, err := s.readJSONLFile(jsonlFile, startTime, endTime); err == nil {
			allRecords = append(allRecords, records...)
		} else {
			// 兼容旧的JSON格式
			jsonFile := filepath.Join(vmDir, fmt.Sprintf("traffic_%s.json", dateStr))
			if records, err := s.readJSONFile(jsonFile, startTime, endTime); err == nil {
				allRecords = append(allRecords, records...)
			}
		}

		// 移动到下一天
		current = current.AddDate(0, 0, 1)
	}

	// 排序
	sort.Slice(allRecords, func(i, j int) bool {
		return allRecords[i].Timestamp.Before(allRecords[j].Timestamp)
	})

	return allRecords, nil
}

// readJSONLFile 读取JSONL格式文件（每行一个JSON对象）
func (s *FileStorage) readJSONLFile(filename string, startTime, endTime time.Time) ([]models.TrafficRecord, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var records []models.TrafficRecord
	lines := strings.Split(string(data), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var record models.TrafficRecord
		if err := json.Unmarshal([]byte(line), &record); err == nil {
			if (record.Timestamp.After(startTime) || record.Timestamp.Equal(startTime)) &&
				(record.Timestamp.Before(endTime) || record.Timestamp.Equal(endTime)) {
				records = append(records, record)
			}
		}
	}

	return records, nil
}

// readJSONFile 读取JSON格式文件（兼容旧格式）
func (s *FileStorage) readJSONFile(filename string, startTime, endTime time.Time) ([]models.TrafficRecord, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var allRecords []models.TrafficRecord
	if err := json.Unmarshal(data, &allRecords); err != nil {
		return nil, err
	}

	var records []models.TrafficRecord
	for _, record := range allRecords {
		if (record.Timestamp.After(startTime) || record.Timestamp.Equal(startTime)) &&
			(record.Timestamp.Before(endTime) || record.Timestamp.Equal(endTime)) {
			records = append(records, record)
		}
	}

	return records, nil
}

// CalculateTrafficStats 计算流量统计
func (s *FileStorage) CalculateTrafficStats(vmid int, period string) (*models.TrafficStats, error) {
	return s.CalculateTrafficStatsWithDirection(vmid, period, time.Time{}, false, "both")
}

// CalculateTrafficStatsWithTime 使用指定时间计算流量统计（兼容旧接口）
func (s *FileStorage) CalculateTrafficStatsWithTime(vmid int, period string, creationTime time.Time, useCreationTime bool) (*models.TrafficStats, error) {
	return s.CalculateTrafficStatsWithDirection(vmid, period, creationTime, useCreationTime, "both")
}

// CalculateTrafficStatsWithDirection 使用指定方向和时间计算流量统计
func (s *FileStorage) CalculateTrafficStatsWithDirection(vmid int, period string, creationTime time.Time, useCreationTime bool, direction string) (*models.TrafficStats, error) {
	now := time.Now()
	var startTime time.Time

	if useCreationTime && !creationTime.IsZero() {
		// 使用基于创建时间的周期计算
		startTime = s.calculatePeriodStart(period, creationTime, now)
	} else {
		// 使用固定周期
		switch period {
		case "minute":
			// minute周期：查询最近5分钟（确保有足够的数据点）
			// 采集间隔60秒，5分钟内约有5条记录
			startTime = now.Add(-5 * time.Minute)
		case "hour":
			startTime = time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())
		case "day":
			startTime = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		case "month":
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
func (s *FileStorage) CalculateTrafficStatsWithTimeRange(vmid int, startTime, endTime time.Time, direction string) (*models.TrafficStats, error) {
	records, err := s.GetTrafficRecords(vmid, startTime, endTime)
	if err != nil {
		return nil, err
	}

	// 使用公共辅助函数构建统计结果（避免重复代码）
	return buildTrafficStats(vmid, "custom", startTime, endTime, direction, records), nil
}

// SaveActionLog 保存操作日志
func (s *FileStorage) SaveActionLog(log models.ActionLog) error {
	logDir := filepath.Join(s.basePath, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("创建日志目录失败: %w", err)
	}

	dateStr := log.Timestamp.Format("2006-01-02")
	filename := filepath.Join(logDir, fmt.Sprintf("actions_%s.json", dateStr))

	// 读取现有日志
	var logs []models.ActionLog
	if data, err := os.ReadFile(filename); err == nil {
		json.Unmarshal(data, &logs)
	}

	// 添加新日志
	logs = append(logs, log)

	// 保存
	data, err := json.MarshalIndent(logs, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化操作日志失败: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("保存操作日志失败: %w", err)
	}

	return nil
}

// GetActionLogs 获取操作日志
func (s *FileStorage) GetActionLogs(startTime, endTime time.Time) ([]models.ActionLog, error) {
	logDir := filepath.Join(s.basePath, "logs")
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		return []models.ActionLog{}, nil
	}

	var allLogs []models.ActionLog

	// 遍历可能的日期文件
	current := startTime
	for current.Before(endTime) || current.Equal(endTime) {
		dateStr := current.Format("2006-01-02")
		filename := filepath.Join(logDir, fmt.Sprintf("actions_%s.json", dateStr))

		if data, err := os.ReadFile(filename); err == nil {
			var logs []models.ActionLog
			if err := json.Unmarshal(data, &logs); err == nil {
				for _, log := range logs {
					if (log.Timestamp.After(startTime) || log.Timestamp.Equal(startTime)) &&
						(log.Timestamp.Before(endTime) || log.Timestamp.Equal(endTime)) {
						allLogs = append(allLogs, log)
					}
				}
			}
		}

		current = current.AddDate(0, 0, 1)
	}

	// 排序
	sort.Slice(allLogs, func(i, j int) bool {
		return allLogs[i].Timestamp.Before(allLogs[j].Timestamp)
	})

	return allLogs, nil
}

// SaveVMState 保存虚拟机状态
func (s *FileStorage) SaveVMState(vmid int, state map[string]interface{}) error {
	stateDir := filepath.Join(s.basePath, "states")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return fmt.Errorf("创建状态目录失败: %w", err)
	}

	filename := filepath.Join(stateDir, fmt.Sprintf("vm_%d_state.json", vmid))

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化虚拟机状态失败: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("保存虚拟机状态失败: %w", err)
	}

	return nil
}

// LoadVMState 加载虚拟机状态
func (s *FileStorage) LoadVMState(vmid int) (map[string]interface{}, error) {
	filename := filepath.Join(s.basePath, "states", fmt.Sprintf("vm_%d_state.json", vmid))

	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]interface{}{}, nil
		}
		return nil, fmt.Errorf("读取虚拟机状态失败: %w", err)
	}

	var state map[string]interface{}
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("解析虚拟机状态失败: %w", err)
	}

	return state, nil
}

// CleanupOldData 清理旧数据（删除超过保留期的文件）
func (s *FileStorage) CleanupOldData(retentionDays int) error {
	if retentionDays <= 0 {
		return nil // 不清理
	}

	cutoffTime := time.Now().AddDate(0, 0, -retentionDays)
	cutoffDate := cutoffTime.Format("2006-01-02")

	// 遍历所有VM目录
	entries, err := os.ReadDir(s.basePath)
	if err != nil {
		return fmt.Errorf("读取存储目录失败: %w", err)
	}

	deletedCount := 0
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "vm_") {
			continue
		}

		vmDir := filepath.Join(s.basePath, entry.Name())
		files, err := os.ReadDir(vmDir)
		if err != nil {
			continue
		}

		// 删除过期文件
		for _, file := range files {
			if file.IsDir() {
				continue
			}

			// 从文件名提取日期
			name := file.Name()
			if !strings.HasPrefix(name, "traffic_") {
				continue
			}

			// traffic_2006-01-02.jsonl 或 traffic_2006-01-02.json
			dateStr := strings.TrimPrefix(name, "traffic_")
			dateStr = strings.TrimSuffix(dateStr, ".jsonl")
			dateStr = strings.TrimSuffix(dateStr, ".json")

			// 比较日期
			if dateStr < cutoffDate {
				filePath := filepath.Join(vmDir, name)
				if err := os.Remove(filePath); err == nil {
					deletedCount++
					utils.DebugLog("删除过期数据文件: %s", filePath)
				}
			}
		}
	}

	if deletedCount > 0 {
		utils.DebugLog("数据清理完成: 删除 %d 个过期文件 (保留天数: %d)", deletedCount, retentionDays)
	}

	return nil
}

// calculatePeriodStart 基于创建时间计算周期开始时间
func (s *FileStorage) calculatePeriodStart(period string, creationTime, now time.Time) time.Time {
	creation := creationTime

	switch period {
	case "hour":
		hoursSinceCreation := int(now.Sub(creation).Hours())
		return creation.Add(time.Duration(hoursSinceCreation) * time.Hour)

	case "day":
		daysSinceCreation := int(now.Sub(creation).Hours() / 24)
		return creation.AddDate(0, 0, daysSinceCreation)

	case "month":
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

// DeleteRecordsInRange 删除指定时间范围内的记录
func (s *FileStorage) DeleteRecordsInRange(vmid int, startTime, endTime time.Time) (int64, error) {
	var deletedCount int64

	if vmid == 0 {
		// 删除所有VM在时间范围内的记录
		vmDirs, err := filepath.Glob(filepath.Join(s.basePath, "vm_*"))
		if err != nil {
			return 0, err
		}

		for _, vmDir := range vmDirs {
			count, err := s.deleteRecordsInRangeForVM(vmDir, startTime, endTime)
			if err != nil {
				utils.DebugLog("删除VM目录 %s 的记录失败: %v", vmDir, err)
				continue
			}
			deletedCount += count
		}
	} else {
		// 删除指定VM在时间范围内的记录
		vmDir := filepath.Join(s.basePath, fmt.Sprintf("vm_%d", vmid))
		count, err := s.deleteRecordsInRangeForVM(vmDir, startTime, endTime)
		if err != nil {
			return 0, err
		}
		deletedCount = count
	}

	// 更新计数器
	if deletedCount > 0 {
		s.recordCounter.mu.Lock()
		s.recordCounter.cachedCount -= deletedCount
		s.recordCounter.mu.Unlock()
		s.recordCounter.save()
	}

	return deletedCount, nil
}

// deleteRecordsInRangeForVM 删除指定VM目录下时间范围内的记录
func (s *FileStorage) deleteRecordsInRangeForVM(vmDir string, startTime, endTime time.Time) (int64, error) {
	if _, err := os.Stat(vmDir); os.IsNotExist(err) {
		return 0, nil
	}

	var deletedCount int64

	// 遍历可能的日期文件
	current := startTime
	for current.Before(endTime.AddDate(0, 0, 1)) {
		dateStr := current.Format("2006-01-02")
		jsonlFile := filepath.Join(vmDir, fmt.Sprintf("traffic_%s.jsonl", dateStr))

		// 读取文件
		records, err := s.readJSONLFileAll(jsonlFile)
		if err != nil {
			current = current.AddDate(0, 0, 1)
			continue
		}

		// 过滤记录
		filtered := []models.TrafficRecord{}
		for _, record := range records {
			if record.Timestamp.Before(startTime) || record.Timestamp.After(endTime) {
				filtered = append(filtered, record)
			} else {
				deletedCount++
			}
		}

		// 重写文件或删除空文件
		if len(filtered) == 0 {
			os.Remove(jsonlFile)
		} else if len(filtered) != len(records) {
			if err := s.writeJSONLFile(jsonlFile, filtered); err != nil {
				return deletedCount, err
			}
		}

		current = current.AddDate(0, 0, 1)
	}

	return deletedCount, nil
}

// readJSONLFileAll 读取整个JSONL文件
func (s *FileStorage) readJSONLFileAll(filename string) ([]models.TrafficRecord, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var records []models.TrafficRecord
	lines := strings.Split(string(data), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var record models.TrafficRecord
		if err := json.Unmarshal([]byte(line), &record); err == nil {
			records = append(records, record)
		}
	}

	return records, nil
}

// writeJSONLFile 写入JSONL文件
func (s *FileStorage) writeJSONLFile(filename string, records []models.TrafficRecord) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, record := range records {
		data, err := json.Marshal(record)
		if err != nil {
			return err
		}
		if _, err := f.Write(append(data, '\n')); err != nil {
			return err
		}
	}

	return nil
}

// CountRecordsInRange 统计指定时间范围内的记录数
func (s *FileStorage) CountRecordsInRange(vmid int, startTime, endTime time.Time) (int64, error) {
	var count int64

	if vmid == 0 {
		// 统计所有VM在时间范围内的记录
		vmDirs, err := filepath.Glob(filepath.Join(s.basePath, "vm_*"))
		if err != nil {
			return 0, err
		}

		for _, vmDir := range vmDirs {
			c, err := s.countRecordsInRangeForVM(vmDir, startTime, endTime)
			if err != nil {
				continue
			}
			count += c
		}
	} else {
		// 统计指定VM在时间范围内的记录
		vmDir := filepath.Join(s.basePath, fmt.Sprintf("vm_%d", vmid))
		c, err := s.countRecordsInRangeForVM(vmDir, startTime, endTime)
		if err != nil {
			return 0, err
		}
		count = c
	}

	return count, nil
}

// countRecordsInRangeForVM 统计指定VM目录下时间范围内的记录数
func (s *FileStorage) countRecordsInRangeForVM(vmDir string, startTime, endTime time.Time) (int64, error) {
	if _, err := os.Stat(vmDir); os.IsNotExist(err) {
		return 0, nil
	}

	var count int64

	// 遍历可能的日期文件
	current := startTime
	for current.Before(endTime.AddDate(0, 0, 1)) {
		dateStr := current.Format("2006-01-02")
		jsonlFile := filepath.Join(vmDir, fmt.Sprintf("traffic_%s.jsonl", dateStr))

		records, err := s.readJSONLFileAll(jsonlFile)
		if err != nil {
			current = current.AddDate(0, 0, 1)
			continue
		}

		for _, record := range records {
			if !record.Timestamp.Before(startTime) && !record.Timestamp.After(endTime) {
				count++
			}
		}

		current = current.AddDate(0, 0, 1)
	}

	return count, nil
}

// DeleteRecordsBefore 删除指定日期之前的所有记录
func (s *FileStorage) DeleteRecordsBefore(beforeTime time.Time) (int64, error) {
	var deletedCount int64

	vmDirs, err := filepath.Glob(filepath.Join(s.basePath, "vm_*"))
	if err != nil {
		return 0, err
	}

	for _, vmDir := range vmDirs {
		// 遍历该VM的所有日期文件
		files, err := filepath.Glob(filepath.Join(vmDir, "traffic_*.jsonl"))
		if err != nil {
			continue
		}

		for _, file := range files {
			// 从文件名提取日期
			basename := filepath.Base(file)
			dateStr := strings.TrimPrefix(basename, "traffic_")
			dateStr = strings.TrimSuffix(dateStr, ".jsonl")

			fileDate, err := time.ParseInLocation("2006-01-02", dateStr, time.Local)
			if err != nil {
				continue
			}

			// 如果整个文件的日期都在beforeTime之前，直接删除文件
			if fileDate.Before(beforeTime) {
				records, err := s.readJSONLFileAll(file)
				if err == nil {
					deletedCount += int64(len(records))
					os.Remove(file)
				}
			} else {
				// 否则需要过滤文件内容
				records, err := s.readJSONLFileAll(file)
				if err != nil {
					continue
				}

				filtered := []models.TrafficRecord{}
				for _, record := range records {
					if record.Timestamp.Before(beforeTime) {
						deletedCount++
					} else {
						filtered = append(filtered, record)
					}
				}

				if len(filtered) == 0 {
					os.Remove(file)
				} else if len(filtered) != len(records) {
					s.writeJSONLFile(file, filtered)
				}
			}
		}
	}

	// 更新计数器
	if deletedCount > 0 {
		s.recordCounter.mu.Lock()
		s.recordCounter.cachedCount -= deletedCount
		s.recordCounter.mu.Unlock()
		s.recordCounter.save()
	}

	return deletedCount, nil
}

// CountRecordsBefore 统计指定日期之前的记录数
func (s *FileStorage) CountRecordsBefore(beforeTime time.Time) (int64, error) {
	var count int64

	vmDirs, err := filepath.Glob(filepath.Join(s.basePath, "vm_*"))
	if err != nil {
		return 0, err
	}

	for _, vmDir := range vmDirs {
		files, err := filepath.Glob(filepath.Join(vmDir, "traffic_*.jsonl"))
		if err != nil {
			continue
		}

		for _, file := range files {
			records, err := s.readJSONLFileAll(file)
			if err != nil {
				continue
			}

			for _, record := range records {
				if record.Timestamp.Before(beforeTime) {
					count++
				}
			}
		}
	}

	return count, nil
}
