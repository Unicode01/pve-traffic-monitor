package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"pve-traffic-monitor/pkg/api"
	"pve-traffic-monitor/pkg/cache"
	"pve-traffic-monitor/pkg/chart"
	"pve-traffic-monitor/pkg/config"
	"pve-traffic-monitor/pkg/ipc"
	"pve-traffic-monitor/pkg/models"
	"pve-traffic-monitor/pkg/pve"
	"pve-traffic-monitor/pkg/recovery"
	"pve-traffic-monitor/pkg/storage"
	"strings"
	"sync"
	"syscall"
	"time"
)

var (
	configPath   = flag.String("config", "config.json", "配置文件路径")
	exportCmd    = flag.String("export", "", "导出图表 (格式: vm_id 或 all)")
	exportFormat = flag.String("format", "html", "导出格式 (json/png/html), 默认: html")
	useDarkTheme = flag.Bool("dark", false, "使用暗色主题 (仅html格式)")
	period       = flag.String("period", "hour", "聚合粒度 (minute/hour/day/month), 也用于确定默认时间范围")
	direction    = flag.String("direction", "both", "流量方向 (both/rx/tx)")
	startTime    = flag.String("start", "", "开始时间 (格式: 2006-01-02 或 2006-01-02T15:04:05)")
	endTime      = flag.String("end", "", "结束时间 (格式: 2006-01-02 或 2006-01-02T15:04:05)")
	exportDate   = flag.String("date", "", "指定日期 (格式: 2006-01-02, 导出某天的数据)")

	// 清除数据相关参数
	cleanupCmd = flag.String("cleanup", "", "清除历史数据 (range/vm/before)")
	vmID       = flag.Int("vmid", 0, "虚拟机ID (cleanup=vm时使用)")
	beforeDate = flag.String("before", "", "删除此日期之前的数据 (格式: 2006-01-02, cleanup=before时使用)")
	dryRun     = flag.Bool("dry-run", false, "仅显示将删除的数据，不实际执行")
)

type Monitor struct {
	configLoader    *config.Loader
	pveClient       *pve.Client
	storage         storage.Interface
	exporter        *chart.Exporter
	apiServer       *api.Server
	watcher         *config.Watcher
	recoveryManager *recovery.Manager
	trafficCache    *cache.TrafficCache // 流量统计缓存
	ipcServer       *ipc.Server         // IPC服务器
}

func main() {
	flag.Parse()

	// 检查是否为CLI模式（导出或清除命令）
	isCliMode := *exportCmd != "" || *cleanupCmd != ""

	// 加载配置（使用配置加载器）
	configLoader, err := config.NewLoader(*configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 创建监控器（CLI模式不启动API服务器）
	monitor, err := NewMonitor(configLoader, isCliMode)
	if err != nil {
		log.Fatalf("创建监控器失败: %v", err)
	}

	// 处理导出命令
	if *exportCmd != "" {
		if err := monitor.handleExport(*exportCmd, *period); err != nil {
			log.Fatalf("导出失败: %v", err)
		}
		return
	}

	// 处理清除数据命令
	if *cleanupCmd != "" {
		if err := monitor.handleCleanup(*cleanupCmd); err != nil {
			log.Fatalf("清除数据失败: %v", err)
		}

		// 清除完成后，通知主程序（如果在运行）
		monitor.notifyMainProgram("cleanup_done", map[string]interface{}{
			"type":    *cleanupCmd,
			"vmid":    *vmID,
			"success": true,
		})

		return
	}

	// 启动监控
	log.Println("启动 PVE 流量监控程序...")
	if err := monitor.Start(); err != nil {
		log.Fatalf("启动监控失败: %v", err)
	}
}

func NewMonitor(configLoader *config.Loader, isCliMode bool) (*Monitor, error) {
	cfg := configLoader.GetConfig()

	// 创建 PVE 客户端
	pveClient := pve.NewClient(cfg.PVE)
	if err := pveClient.Login(); err != nil {
		return nil, fmt.Errorf("登录 PVE 失败: %w", err)
	}

	// 创建存储管理器(使用工厂模式,支持多种存储类型)
	store, err := storage.NewStorageFromConfig(&cfg.Storage)
	if err != nil {
		return nil, fmt.Errorf("创建存储管理器失败: %w", err)
	}
	log.Printf("存储类型: %s", cfg.Storage.Type)

	// 创建图表导出器
	exporter, err := chart.NewExporter(cfg.Monitor.ExportPath)
	if err != nil {
		return nil, fmt.Errorf("创建图表导出器失败: %w", err)
	}

	// 创建配置监视器
	watcher := config.NewWatcher(configLoader)

	// 创建恢复管理器
	recoveryMgr := recovery.NewManager(pveClient, store)

	// 从存储加载状态
	if err := recoveryMgr.LoadStatesFromStorage(); err != nil {
		log.Printf("加载虚拟机状态失败: %v", err)
	}

	// 创建流量缓存（5分钟TTL）
	trafficCache := cache.NewTrafficCache(5 * time.Minute)

	// 创建IPC服务器（获取合适的socket路径）
	// CLI模式不创建IPC服务器
	var ipcServer *ipc.Server
	if !isCliMode {
		var socketBasePath string
		if cfg.Storage.Type == "file" || cfg.Storage.Type == "" {
			socketBasePath = cfg.Storage.FilePath
		} else {
			// 数据库存储，使用临时目录
			socketBasePath = filepath.Join(os.TempDir(), "pve-traffic-monitor")
		}

		socketPath := ipc.GetDefaultSocketPath(socketBasePath)
		ipcServer, err = ipc.NewServer(socketPath)
		if err != nil {
			return nil, fmt.Errorf("创建IPC服务器失败: %w", err)
		}
	}

	monitor := &Monitor{
		configLoader:    configLoader,
		pveClient:       pveClient,
		storage:         store,
		exporter:        exporter,
		watcher:         watcher,
		recoveryManager: recoveryMgr,
		trafficCache:    trafficCache,
		ipcServer:       ipcServer,
	}

	// 注册配置重载回调
	configLoader.OnReload(monitor.onConfigReload)

	// 如果启用了API服务器且非CLI模式，创建并启动
	if cfg.API.Enabled && !isCliMode {
		monitor.apiServer = api.NewServer(cfg, store, pveClient)
		go func() {
			if err := monitor.apiServer.Start(); err != nil {
				log.Printf("API 服务器错误: %v\n", err)
			}
		}()
	}

	return monitor, nil
}

// onConfigReload 配置重载回调函数
func (m *Monitor) onConfigReload(newConfig *models.Config) {
	log.Println("配置已重载")

	// 如果 PVE 连接信息改变，重新登录
	currentConfig := m.configLoader.GetConfig()
	if currentConfig.PVE.Host != newConfig.PVE.Host ||
		currentConfig.PVE.Port != newConfig.PVE.Port ||
		currentConfig.PVE.APITokenID != newConfig.PVE.APITokenID ||
		currentConfig.PVE.APITokenSecret != newConfig.PVE.APITokenSecret {
		log.Println("PVE 连接信息已更改，重新登录...")
		m.pveClient = pve.NewClient(newConfig.PVE)
		if err := m.pveClient.Login(); err != nil {
			log.Printf("重新登录失败: %v", err)
		}
	}

	// 如果 API 配置改变，重启 API 服务器
	if currentConfig.API.Enabled != newConfig.API.Enabled ||
		currentConfig.API.Port != newConfig.API.Port {
		if m.apiServer != nil {
			log.Println("警告: API 配置已更改，需要重启程序以应用")
		} else if newConfig.API.Enabled {
			log.Println("警告: API 服务器已启用，需要重启程序以应用")
		}
	}
}

func countEnabledRules(rules []models.Rule) int {
	count := 0
	for _, rule := range rules {
		if rule.Enabled {
			count++
		}
	}
	return count
}

func (m *Monitor) Start() error {
	cfg := m.configLoader.GetConfig()
	ticker := time.NewTicker(time.Duration(cfg.Monitor.IntervalSeconds) * time.Second)
	defer ticker.Stop()

	// 启动配置监视器
	m.watcher.Start()
	defer m.watcher.Stop()

	// 启动IPC服务器（如果已创建）
	if m.ipcServer != nil {
		if err := m.ipcServer.Start(); err != nil {
			log.Printf("警告: IPC服务器启动失败: %v", err)
		} else {
			defer m.ipcServer.Stop()

			// 注册消息处理器
			m.ipcServer.OnMessage("cleanup_done", m.handleCleanupNotification)
			m.ipcServer.OnMessage("reload_cache", m.handleReloadCacheNotification)
		}
	}

	// 处理信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Printf("监控已启动 [间隔:%ds PID:%d]", cfg.Monitor.IntervalSeconds, os.Getpid())

	// 立即执行一次
	if err := m.collectAndProcess(); err != nil {
		log.Printf("错误: %v\n", err)
	}

	// 用于动态调整 ticker 的通道
	tickerUpdateChan := make(chan time.Duration, 1)

	// 监听配置变化并更新 ticker
	m.configLoader.OnReload(func(newConfig *models.Config) {
		newInterval := time.Duration(newConfig.Monitor.IntervalSeconds) * time.Second
		tickerUpdateChan <- newInterval
	})

	// 定期检查恢复（每分钟检查一次）
	recoveryTicker := time.NewTicker(1 * time.Minute)
	defer recoveryTicker.Stop()

	// 定期清理旧数据（每天凌晨3点检查一次）
	cleanupTicker := time.NewTicker(1 * time.Hour)
	defer cleanupTicker.Stop()
	lastCleanupDay := time.Now().Day()

	for {
		select {
		case <-ticker.C:
			if err := m.collectAndProcess(); err != nil {
				log.Printf("错误: %v\n", err)
			}
		case <-recoveryTicker.C:
			// 检查是否有需要恢复的虚拟机
			if err := m.recoveryManager.CheckAndRecoverDue(); err != nil {
				log.Printf("检查恢复失败: %v\n", err)
			}
		case <-cleanupTicker.C:
			// 每天凌晨3点清理一次旧数据
			now := time.Now()
			if now.Hour() == 3 && now.Day() != lastCleanupDay {
				cfg := m.configLoader.GetConfig()
				if cfg.Monitor.DataRetentionDays > 0 {
					log.Printf("开始清理旧数据 (保留 %d 天)", cfg.Monitor.DataRetentionDays)
					if err := m.storage.CleanupOldData(cfg.Monitor.DataRetentionDays); err != nil {
						log.Printf("清理旧数据失败: %v", err)
					}
				}
				lastCleanupDay = now.Day()
			}
		case newInterval := <-tickerUpdateChan:
			// 更新监控间隔
			ticker.Stop()
			ticker = time.NewTicker(newInterval)
			log.Printf("监控间隔已更新为: %v\n", newInterval)
		case <-sigChan:
			log.Println("正在退出...")

			// 获取所有虚拟机
			cfg := m.configLoader.GetConfig()
			vms, err := m.pveClient.GetAllVMsWithFilter(cfg.Monitor.IncludeTemplates)
			if err == nil {
				m.recoveryManager.CleanupAllTags(vms)
			}

			// 恢复所有虚拟机
			m.recoveryManager.RecoverAll()

			// 关闭存储（保存计数器等）
			if err := m.storage.Close(); err != nil {
				log.Printf("关闭存储失败: %v", err)
			}

			log.Println("已退出")
			return nil
		}
	}
}

func (m *Monitor) collectAndProcess() error {
	// 获取所有虚拟机（根据配置决定是否包含模板）
	cfg := m.configLoader.GetConfig()
	vms, err := m.pveClient.GetAllVMsWithFilter(cfg.Monitor.IncludeTemplates)
	if err != nil {
		return fmt.Errorf("获取虚拟机列表失败: %w", err)
	}

	// 使用worker pool并发处理
	const maxWorkers = models.MaxWorkers
	vmChan := make(chan models.VMInfo, len(vms))

	var wg sync.WaitGroup

	// 启动worker池
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for vm := range vmChan {
				if err := m.processVM(vm); err != nil {
					log.Printf("虚拟机 %d 处理失败: %v", vm.VMID, err)
				}
			}
		}()
	}

	// 发送任务
	for _, vm := range vms {
		vmChan <- vm
	}
	close(vmChan)

	// 等待所有worker完成
	wg.Wait()

	return nil
}

func (m *Monitor) processVM(vm models.VMInfo) error {
	// 再次检查是否为模板（双重保险）
	if vm.IsTemplate() {
		return nil
	}

	// 获取最新状态
	status, err := m.pveClient.GetVMStatus(vm.VMID)
	if err != nil {
		return err
	}

	// 保存流量记录
	now := time.Now()
	record := models.TrafficRecord{
		VMID:       vm.VMID,
		Timestamp:  now,
		RXBytes:    status.NetworkRX,
		TXBytes:    status.NetworkTX,
		TotalBytes: status.NetworkRX + status.NetworkTX,
	}

	if err := m.storage.SaveTrafficRecord(record); err != nil {
		return fmt.Errorf("保存流量记录失败: %w", err)
	}

	// 检查并应用规则
	// 只有匹配规则的虚拟机才会被打标签
	if err := m.applyRules(vm); err != nil {
		log.Printf("应用规则失败 (VM %d): %v\n", vm.VMID, err)
	}

	return nil
}

func (m *Monitor) applyRules(vm models.VMInfo) error {
	cfg := m.configLoader.GetConfig()

	// 1. 收集该VM匹配的所有规则
	var matchedRules []models.Rule
	for _, rule := range cfg.Rules {
		if !rule.Enabled {
			continue
		}

		if !m.vmMatchesRule(vm, rule) {
			continue
		}

		matchedRules = append(matchedRules, rule)
	}

	if len(matchedRules) == 0 {
		return nil
	}

	// 2. 按 (period, direction, useCreationTime) 分组，避免重复计算
	type StatsKey struct {
		Period          string
		Direction       string
		UseCreationTime bool
	}

	statsMap := make(map[StatsKey]*models.TrafficStats)
	var vmCreationTime time.Time

	// 3. 对每组只计算一次
	for _, rule := range matchedRules {
		direction := "both"
		if rule.TrafficDirection != "" {
			direction = rule.TrafficDirection
		}

		key := StatsKey{
			Period:          rule.Period,
			Direction:       direction,
			UseCreationTime: rule.UseCreationTime,
		}

		// 如果已经计算过这个组合，跳过
		if _, exists := statsMap[key]; exists {
			continue
		}

		// 计算流量统计
		stats, err := m.calculateTrafficStatsWithCache(vm.VMID, rule.Period, direction, rule.UseCreationTime, &vmCreationTime)
		if err != nil {
			log.Printf("计算流量统计失败 (VM %d): %v", vm.VMID, err)
			continue
		}

		statsMap[key] = stats
	}

	// 4. 使用计算结果检查所有匹配的规则
	for _, rule := range matchedRules {
		direction := "both"
		if rule.TrafficDirection != "" {
			direction = rule.TrafficDirection
		}

		key := StatsKey{
			Period:          rule.Period,
			Direction:       direction,
			UseCreationTime: rule.UseCreationTime,
		}

		stats, exists := statsMap[key]
		if !exists {
			continue
		}

		// 为每个匹配的规则打独立的流量状态标签
		if err := m.pveClient.AutoTagByTrafficWithRule(vm.VMID, stats.TotalGB, rule.LimitGB, rule.Name); err != nil {
			debugLog("自动打流量标签失败 (VM %d, 规则 %s): %v", vm.VMID, rule.Name, err)
		}

		// 检查是否超出限制
		if stats.TotalGB > rule.LimitGB {
			directionText := getDirectionText(stats.Direction)
			log.Printf("VM%d 超%s流量限制 %.2f/%.2f GB [%s]",
				vm.VMID, directionText, stats.TotalGB, rule.LimitGB, rule.Name)

			// 执行操作（传递创建时间信息）
			if err := m.executeAction(vm, rule, stats, vmCreationTime); err != nil {
				log.Printf("执行操作失败: %v", err)
				// 继续执行其他规则
			}
		}
	}

	return nil
}

// calculateTrafficStatsWithCache 带缓存的流量统计计算
func (m *Monitor) calculateTrafficStatsWithCache(vmid int, period string, direction string, useCreationTime bool, vmCreationTime *time.Time) (*models.TrafficStats, error) {
	now := time.Now()
	var startTime time.Time
	var creationTime time.Time

	// 计算周期开始时间
	if useCreationTime {
		ct, err := m.pveClient.GetVMCreationTime(vmid)
		if err == nil {
			creationTime = ct
			*vmCreationTime = ct
			startTime = m.calculatePeriodStart(period, creationTime, now)
		} else {
			startTime = m.calculateFixedPeriodStart(period, now)
		}
	} else {
		startTime = m.calculateFixedPeriodStart(period, now)
	}

	// 尝试从缓存获取
	if cachedStats, ok := m.trafficCache.Get(vmid, period, direction, startTime); ok {
		debugLog("缓存命中: VM%d period=%s direction=%s", vmid, period, direction)
		return cachedStats, nil
	}

	debugLog("缓存未命中: VM%d period=%s direction=%s", vmid, period, direction)

	// 缓存未命中，计算统计
	var stats *models.TrafficStats
	var err error

	if useCreationTime && !creationTime.IsZero() {
		stats, err = m.storage.CalculateTrafficStatsWithDirection(vmid, period, creationTime, true, direction)
	} else {
		stats, err = m.storage.CalculateTrafficStatsWithDirection(vmid, period, time.Time{}, false, direction)
	}

	if err != nil {
		return nil, err
	}

	// 存入缓存
	m.trafficCache.Set(vmid, period, direction, startTime, stats)

	return stats, nil
}

// calculateFixedPeriodStart 计算固定周期的开始时间
func (m *Monitor) calculateFixedPeriodStart(period string, now time.Time) time.Time {
	switch period {
	case models.PeriodHour:
		return time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())
	case models.PeriodDay:
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	case models.PeriodMonth:
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	default:
		return now
	}
}

// calculatePeriodStart 计算基于创建时间的周期开始时间（简化版）
func (m *Monitor) calculatePeriodStart(period string, creationTime, now time.Time) time.Time {
	// 这里简化处理，实际逻辑在storage层
	return m.calculateFixedPeriodStart(period, now)
}

func (m *Monitor) vmMatchesRule(vm models.VMInfo, rule models.Rule) bool {
	// 使用统一的规则匹配函数
	return pve.VMMatchesRule(vm, rule)
}

func (m *Monitor) executeAction(vm models.VMInfo, rule models.Rule, stats *models.TrafficStats, creationTime time.Time) error {
	actionLog := models.ActionLog{
		VMID:      vm.VMID,
		RuleName:  rule.Name,
		Action:    rule.Action,
		Reason:    fmt.Sprintf("超出流量限制: %.2f GB / %.2f GB", stats.TotalGB, rule.LimitGB),
		Timestamp: time.Now(),
	}

	// 检查是否已经执行过该操作（通过检查对应的标签）
	actionTag := ""
	switch rule.Action {
	case models.ActionShutdown, models.ActionStop:
		actionTag = models.TagTrafficShutdown
	case models.ActionDisconnect:
		actionTag = models.TagTrafficDisconnect
	case models.ActionRateLimit:
		actionTag = models.TagTrafficLimited
	}

	// 如果已经有对应的标签，说明操作已执行，跳过
	if actionTag != "" {
		tags, err := m.pveClient.GetVMTags(vm.VMID)
		if err == nil {
			for _, tag := range tags {
				if strings.ToLower(tag) == actionTag {
					debugLog("VM%d 已执行过操作 %s，跳过重复执行", vm.VMID, rule.Action)
					return nil
				}
			}
		}
	}

	// 先记录虚拟机状态（在执行操作前，传递创建时间信息）
	if err := m.recoveryManager.RecordVMState(vm.VMID, rule.Action, rule.Period, rule.Name, rule.UseCreationTime, creationTime); err != nil {
		log.Printf("记录虚拟机状态失败 (VM %d): %v\n", vm.VMID, err)
	}

	var err error
	switch rule.Action {
	case models.ActionShutdown:
		if rule.ForceStop {
			log.Printf("执行操作: VM%d 强制停止", vm.VMID)
			err = m.pveClient.StopVM(vm.VMID)
		} else {
			log.Printf("执行操作: VM%d 关机", vm.VMID)
			err = m.pveClient.ShutdownVM(vm.VMID)
		}

		if err == nil {
			m.pveClient.AddVMTag(vm.VMID, models.TagTrafficShutdown)
		}

	case models.ActionStop:
		log.Printf("执行操作: VM%d 强制停止", vm.VMID)
		err = m.pveClient.StopVM(vm.VMID)

		if err == nil {
			m.pveClient.AddVMTag(vm.VMID, models.TagTrafficShutdown)
		}

	case models.ActionDisconnect:
		log.Printf("执行操作: VM%d 断网", vm.VMID)
		err = m.pveClient.DisconnectNetwork(vm.VMID)

		if err == nil {
			m.pveClient.AddVMTag(vm.VMID, models.TagTrafficDisconnect)
		}

	case models.ActionRateLimit:
		log.Printf("执行操作: VM%d 限速至 %.2fMB/s", vm.VMID, rule.RateLimitMB)
		err = m.pveClient.SetNetworkRateLimit(vm.VMID, rule.RateLimitMB)

		if err == nil {
			m.pveClient.AddVMTag(vm.VMID, models.TagTrafficLimited)
		}

	default:
		err = fmt.Errorf("未知操作: %s", rule.Action)
	}

	if err != nil {
		actionLog.Success = false
		actionLog.Error = err.Error()
		log.Printf("操作失败: %v", err)
	} else {
		actionLog.Success = true
	}

	// 保存操作日志
	m.storage.SaveActionLog(actionLog)

	return err
}

func (m *Monitor) handleExport(vmidStr string, period string) error {
	if vmidStr == "all" {
		return m.exportAllVMs(period)
	}

	// 导出单个虚拟机
	var vmid int
	if _, err := fmt.Sscanf(vmidStr, "%d", &vmid); err != nil {
		return fmt.Errorf("无效的虚拟机 ID: %s", vmidStr)
	}

	return m.exportVM(vmid, period)
}

func (m *Monitor) exportVM(vmid int, period string) error {
	// 验证导出格式
	format := *exportFormat
	if format != "json" && format != "png" && format != "html" {
		return fmt.Errorf("无效的导出格式: %s (支持: json/png/html)", format)
	}

	// 验证 period 参数
	validPeriods := map[string]bool{"minute": true, "hour": true, "day": true, "month": true}
	if !validPeriods[period] {
		return fmt.Errorf("无效的聚合周期: %s (支持: minute/hour/day/month)", period)
	}

	// 计算时间范围
	now := time.Now()
	var start, end time.Time
	var err error

	// 优先使用自定义时间范围
	if *startTime != "" && *endTime != "" {
		// 自定义时间范围
		start, err = m.parseTimeParam(*startTime)
		if err != nil {
			return fmt.Errorf("解析开始时间失败: %w", err)
		}
		end, err = m.parseTimeParam(*endTime)
		if err != nil {
			return fmt.Errorf("解析结束时间失败: %w", err)
		}
		// 自定义时间范围时，period 参数用于控制聚合粒度
		log.Printf("使用自定义时间范围，聚合粒度: %s\n", period)
	} else if *exportDate != "" {
		// 指定日期（导出某天的数据）
		date, err := time.ParseInLocation("2006-01-02", *exportDate, time.Local)
		if err != nil {
			return fmt.Errorf("解析日期失败: %w (格式应为: 2006-01-02)", err)
		}
		start = time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
		end = start.Add(24 * time.Hour)
	} else {
		// 使用period参数确定时间范围
		switch period {
		case "minute":
			start = now.Add(-1 * time.Hour) // 最近1小时，按分钟聚合
		case "hour":
			start = now.Add(-24 * time.Hour) // 最近24小时，按小时聚合
		case "day":
			start = now.AddDate(0, 0, -30) // 最近30天，按天聚合
		case "month":
			start = now.AddDate(-1, 0, 0) // 最近1年，按月聚合
		}
		end = now
	}

	// 获取流量记录
	records, err := m.storage.GetTrafficRecords(vmid, start, end)
	if err != nil {
		return fmt.Errorf("获取流量记录失败: %w", err)
	}

	if len(records) == 0 {
		return fmt.Errorf("没有找到流量记录 (时间范围: %s - %s)", start.Format("2006-01-02 15:04"), end.Format("2006-01-02 15:04"))
	}

	// 获取虚拟机信息
	vmInfo, err := m.pveClient.GetVMStatus(vmid)
	if err != nil {
		return fmt.Errorf("获取虚拟机信息失败: %w", err)
	}

	var filename string

	// 根据格式导出（使用带 period 参数的新函数）
	switch format {
	case "json":
		filename, err = m.exporter.ExportJSONData(vmid, vmInfo.Name, records, start, end)
		if err != nil {
			return fmt.Errorf("导出JSON失败: %w", err)
		}
	case "png":
		filename, err = m.exporter.ExportTrafficChartWithRangeAndPeriod(vmid, vmInfo.Name, records, start, end, period)
		if err != nil {
			return fmt.Errorf("导出PNG图表失败: %w", err)
		}
	case "html":
		filename, err = m.exporter.ExportHTMLChartWithRangeAndPeriod(vmid, vmInfo.Name, records, start, end, period, *useDarkTheme)
		if err != nil {
			return fmt.Errorf("导出HTML图表失败: %w", err)
		}
	}

	log.Printf("已导出 (%s): %s\n", format, filename)
	log.Printf("时间范围: %s - %s\n", start.Format("2006-01-02 15:04"), end.Format("2006-01-02 15:04"))
	log.Printf("聚合粒度: %s\n", period)
	log.Printf("原始数据点数: %d\n", len(records))
	return nil
}

// parseTimeParam 解析时间参数（支持多种格式）
func (m *Monitor) parseTimeParam(timeStr string) (time.Time, error) {
	// 尝试多种时间格式
	formats := []string{
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04",
		"2006-01-02 15:04",
		"2006-01-02",
	}

	for _, format := range formats {
		if t, err := time.ParseInLocation(format, timeStr, time.Local); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("无效的时间格式: %s (支持格式: 2006-01-02 或 2006-01-02T15:04:05)", timeStr)
}

func (m *Monitor) exportAllVMs(period string) error {
	// 验证方向参数
	dir := *direction
	if dir != "both" && dir != "rx" && dir != "tx" {
		return fmt.Errorf("无效的流量方向: %s (支持: both/rx/tx)", dir)
	}

	// 验证导出格式
	format := *exportFormat
	if format != "json" && format != "png" && format != "html" {
		return fmt.Errorf("无效的导出格式: %s (支持: json/png/html)", format)
	}

	// 获取所有虚拟机
	vms, err := m.pveClient.GetAllVMs()
	if err != nil {
		return fmt.Errorf("获取虚拟机列表失败: %w", err)
	}

	// 处理时间范围参数（支持start/end参数或period参数）
	var start, end time.Time
	var usePeriod bool

	// 优先使用start/end参数
	if *startTime != "" && *endTime != "" {
		start, err = m.parseTimeParam(*startTime)
		if err != nil {
			return fmt.Errorf("解析开始时间失败: %w", err)
		}
		end, err = m.parseTimeParam(*endTime)
		if err != nil {
			return fmt.Errorf("解析结束时间失败: %w", err)
		}
		if start.After(end) {
			return fmt.Errorf("开始时间不能晚于结束时间")
		}
		usePeriod = false
	} else {
		// 使用period参数
		usePeriod = true
	}

	// 收集统计信息 - 使用与API一致的方法
	var stats []models.TrafficStats
	for _, vm := range vms {
		var stat *models.TrafficStats

		if usePeriod {
			// 使用周期统计
			stat, err = m.storage.CalculateTrafficStatsWithDirection(vm.VMID, period, time.Time{}, false, dir)
			if err != nil {
				log.Printf("计算虚拟机 %d 统计失败: %v\n", vm.VMID, err)
				continue
			}
		} else {
			// 使用时间范围统计
			stat, err = m.storage.CalculateTrafficStatsWithTimeRange(vm.VMID, start, end, dir)
			if err != nil {
				log.Printf("计算虚拟机 %d 统计失败: %v\n", vm.VMID, err)
				continue
			}
		}

		stat.Name = vm.Name // 设置VM名称
		stats = append(stats, *stat)
	}

	if len(stats) == 0 {
		return fmt.Errorf("没有统计数据")
	}

	var filename string

	// 根据格式导出
	switch format {
	case "json":
		filename, err = m.exporter.ExportStatsJSONData(stats, dir)
		if err != nil {
			return fmt.Errorf("导出JSON失败: %w", err)
		}
	case "png":
		filename, err = m.exporter.ExportStatsChart(stats, dir)
		if err != nil {
			return fmt.Errorf("导出PNG图表失败: %w", err)
		}
	case "html":
		filename, err = m.exporter.ExportStatsHTMLChartWithRange(stats, dir, start, end, *useDarkTheme)
		if err != nil {
			return fmt.Errorf("导出HTML图表失败: %w", err)
		}
	}

	if usePeriod {
		log.Printf("汇总已导出 (%s): %s\n", format, filename)
		log.Printf("统计周期: %s, 流量方向: %s, 虚拟机数量: %d\n", period, dir, len(stats))
	} else {
		log.Printf("汇总已导出 (%s): %s\n", format, filename)
		log.Printf("时间范围: %s - %s, 流量方向: %s, 虚拟机数量: %d\n",
			start.Format("2006-01-02 15:04"), end.Format("2006-01-02 15:04"), dir, len(stats))
	}

	return nil
}

func loadConfig(path string) (*models.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var config models.Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 验证配置
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	return &config, nil
}

// getDirectionText 获取流量方向的文本描述
func getDirectionText(direction string) string {
	switch direction {
	case models.DirectionUpload, models.DirectionTX:
		return "上传"
	case models.DirectionDownload, models.DirectionRX:
		return "下载"
	default:
		return "总"
	}
}

// notifyMainProgram 通知主程序
func (m *Monitor) notifyMainProgram(msgType string, data map[string]interface{}) {
	cfg := m.configLoader.GetConfig()

	var socketBasePath string
	if cfg.Storage.Type == "file" || cfg.Storage.Type == "" {
		socketBasePath = cfg.Storage.FilePath
	} else {
		socketBasePath = filepath.Join(os.TempDir(), "pve-traffic-monitor")
	}

	socketPath := ipc.GetDefaultSocketPath(socketBasePath)
	client := ipc.NewClient(socketPath)

	msg := ipc.Message{
		Type:      msgType,
		Timestamp: time.Now(),
		Data:      data,
	}

	if err := client.SendMessage(msg); err != nil {
		// 主程序可能未运行，这是正常的
		log.Printf("通知主程序失败（主程序可能未运行）: %v", err)
	} else {
		log.Printf("已通知主程序: %s", msgType)
	}
}

// handleCleanupNotification 处理清除数据通知
func (m *Monitor) handleCleanupNotification(msg ipc.Message) {
	log.Printf("收到数据清除通知: type=%v, vmid=%v", msg.Data["type"], msg.Data["vmid"])

	// 清除流量缓存
	m.trafficCache.Clear()
	log.Println("已清除流量缓存")

	// 如果API服务器在运行，也应该清除它的缓存
	// API服务器的缓存会自动过期，这里主要是清除monitor的缓存
}

// handleReloadCacheNotification 处理重载缓存通知
func (m *Monitor) handleReloadCacheNotification(msg ipc.Message) {
	log.Println("收到重载缓存通知")
	m.trafficCache.Clear()
	log.Println("已清除流量缓存")
}

// handleCleanup 处理清除数据命令
func (m *Monitor) handleCleanup(cleanupType string) error {
	switch cleanupType {
	case "range":
		// 清除指定时间段的数据
		return m.cleanupRange()
	case "vm":
		// 清除指定VM指定日期的数据
		return m.cleanupVM()
	case "before":
		// 清除指定日期之前的数据
		return m.cleanupBefore()
	default:
		return fmt.Errorf("无效的清除类型: %s (支持: range/vm/before)", cleanupType)
	}
}

// cleanupRange 清除指定时间段的数据
func (m *Monitor) cleanupRange() error {
	if *startTime == "" || *endTime == "" {
		return fmt.Errorf("清除时间段数据需要指定 -start 和 -end 参数")
	}

	start, err := m.parseTimeParam(*startTime)
	if err != nil {
		return fmt.Errorf("解析开始时间失败: %w", err)
	}

	end, err := m.parseTimeParam(*endTime)
	if err != nil {
		return fmt.Errorf("解析结束时间失败: %w", err)
	}

	if start.After(end) {
		return fmt.Errorf("开始时间不能晚于结束时间")
	}

	log.Printf("准备清除时间段数据: %s 至 %s\n", start.Format("2006-01-02 15:04:05"), end.Format("2006-01-02 15:04:05"))

	if *dryRun {
		count, err := m.storage.CountRecordsInRange(0, start, end)
		if err != nil {
			return fmt.Errorf("统计记录数失败: %w", err)
		}
		log.Printf("[DRY RUN] 将删除 %d 条记录\n", count)
		return nil
	}

	deleted, err := m.storage.DeleteRecordsInRange(0, start, end)
	if err != nil {
		return fmt.Errorf("删除记录失败: %w", err)
	}

	log.Printf("成功删除 %d 条记录\n", deleted)
	return nil
}

// cleanupVM 清除指定VM指定日期的数据
func (m *Monitor) cleanupVM() error {
	if *vmID == 0 {
		return fmt.Errorf("清除VM数据需要指定 -vmid 参数")
	}

	var start, end time.Time
	var err error

	if *exportDate != "" {
		// 清除指定日期的数据
		date, err := time.ParseInLocation("2006-01-02", *exportDate, time.Local)
		if err != nil {
			return fmt.Errorf("解析日期失败: %w", err)
		}
		start = time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
		end = start.Add(24 * time.Hour)
	} else if *startTime != "" && *endTime != "" {
		// 清除指定时间段的数据
		start, err = m.parseTimeParam(*startTime)
		if err != nil {
			return fmt.Errorf("解析开始时间失败: %w", err)
		}
		end, err = m.parseTimeParam(*endTime)
		if err != nil {
			return fmt.Errorf("解析结束时间失败: %w", err)
		}
	} else {
		return fmt.Errorf("清除VM数据需要指定 -date 或 (-start 和 -end) 参数")
	}

	log.Printf("准备清除 VM%d 的数据: %s 至 %s\n", *vmID, start.Format("2006-01-02 15:04:05"), end.Format("2006-01-02 15:04:05"))

	if *dryRun {
		count, err := m.storage.CountRecordsInRange(*vmID, start, end)
		if err != nil {
			return fmt.Errorf("统计记录数失败: %w", err)
		}
		log.Printf("[DRY RUN] 将删除 VM%d 的 %d 条记录\n", *vmID, count)
		return nil
	}

	deleted, err := m.storage.DeleteRecordsInRange(*vmID, start, end)
	if err != nil {
		return fmt.Errorf("删除记录失败: %w", err)
	}

	log.Printf("成功删除 VM%d 的 %d 条记录\n", *vmID, deleted)
	return nil
}

// cleanupBefore 清除指定日期之前的数据
func (m *Monitor) cleanupBefore() error {
	if *beforeDate == "" {
		return fmt.Errorf("清除历史数据需要指定 -before 参数")
	}

	date, err := time.ParseInLocation("2006-01-02", *beforeDate, time.Local)
	if err != nil {
		return fmt.Errorf("解析日期失败: %w", err)
	}

	beforeTime := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	log.Printf("准备清除 %s 之前的所有数据\n", beforeTime.Format("2006-01-02"))

	if *dryRun {
		count, err := m.storage.CountRecordsBefore(beforeTime)
		if err != nil {
			return fmt.Errorf("统计记录数失败: %w", err)
		}
		log.Printf("[DRY RUN] 将删除 %d 条记录\n", count)
		return nil
	}

	deleted, err := m.storage.DeleteRecordsBefore(beforeTime)
	if err != nil {
		return fmt.Errorf("删除记录失败: %w", err)
	}

	log.Printf("成功删除 %d 条记录\n", deleted)
	return nil
}
