package recovery

import (
	"fmt"
	"log"
	"pve-traffic-monitor/pkg/models"
	"pve-traffic-monitor/pkg/pve"
	"pve-traffic-monitor/pkg/storage"
	"time"
)

// Manager 恢复管理器
type Manager struct {
	pveClient    *pve.Client
	storage      storage.Interface
	stateManager *models.VMStateManager
}

// NewManager 创建恢复管理器
func NewManager(pveClient *pve.Client, storage storage.Interface) *Manager {
	return &Manager{
		pveClient:    pveClient,
		storage:      storage,
		stateManager: models.NewVMStateManager(),
	}
}

// RecordVMState 记录虚拟机状态（在执行操作前）
func (m *Manager) RecordVMState(vmid int, action, period, ruleName string, useCreationTime bool, creationTime time.Time) error {
	// 获取当前虚拟机状态
	vmInfo, err := m.pveClient.GetVMStatus(vmid)
	if err != nil {
		return fmt.Errorf("获取虚拟机状态失败: %w", err)
	}

	// 获取当前速率限制（如果有）
	rateLimit := 0.0
	config, err := m.pveClient.GetVMConfig(vmid)
	if err == nil {
		// 尝试从配置中解析速率限制
		for key, value := range config {
			if key == "net0" || key == "net1" { // 检查网络接口
				if netConfig, ok := value.(string); ok {
					// 简单解析，实际可能需要更复杂的逻辑
					_ = netConfig // TODO: 解析速率限制
				}
			}
		}
	}

	// 计算恢复时间（支持基于创建时间的周期）
	recoveryTime := calculateRecoveryTime(period, useCreationTime, creationTime)

	state := &models.VMState{
		VMID:              vmid,
		OriginalStatus:    vmInfo.Status,
		OriginalRateLimit: rateLimit,
		ActionTaken:       action,
		ActionTime:        time.Now(),
		Period:            period,
		RuleName:          ruleName,
		NeedsRecovery:     true,
		RecoveryTime:      recoveryTime,
	}

	m.stateManager.RecordState(state)

	// 持久化到存储
	if err := m.storage.SaveVMState(vmid, map[string]interface{}{
		"original_status":     state.OriginalStatus,
		"original_rate_limit": state.OriginalRateLimit,
		"action_taken":        state.ActionTaken,
		"action_time":         state.ActionTime,
		"period":              state.Period,
		"rule_name":           state.RuleName,
		"needs_recovery":      state.NeedsRecovery,
		"recovery_time":       state.RecoveryTime,
	}); err != nil {
		log.Printf("保存虚拟机状态失败: %v", err)
	}

	return nil
}

// RecoverVM 恢复单个虚拟机
func (m *Manager) RecoverVM(vmid int) error {
	state, exists := m.stateManager.GetState(vmid)
	if !exists {
		return fmt.Errorf("虚拟机 %d 没有状态记录", vmid)
	}

	log.Printf("恢复 VM%d [%s→%s]", vmid, state.ActionTaken, state.OriginalStatus)

	// 根据原始状态恢复
	switch state.ActionTaken {
	case "shutdown", "stop":
		// 如果原本是运行状态，重新启动
		if state.OriginalStatus == "running" {
			if err := m.pveClient.StartVM(vmid); err != nil {
				return fmt.Errorf("启动失败: %w", err)
			}
		}

	case "disconnect":
		// 恢复网络连接
		if err := m.pveClient.ConnectNetwork(vmid); err != nil {
			return fmt.Errorf("恢复网络失败: %w", err)
		}

	case "rate_limit":
		// 恢复原始速率限制
		if state.OriginalRateLimit == 0 {
			m.pveClient.RemoveNetworkRateLimit(vmid)
		} else {
			m.pveClient.SetNetworkRateLimit(vmid, state.OriginalRateLimit)
		}
	}

	// 清理所有 traffic- 开头的标签
	tags, err := m.pveClient.GetVMTags(vmid)
	if err == nil {
		for _, tag := range tags {
			if len(tag) >= 8 && tag[:8] == "traffic-" {
				m.pveClient.RemoveVMTag(vmid, tag)
			}
		}
	}

	// 移除状态记录
	m.stateManager.RemoveState(vmid)
	m.storage.SaveVMState(vmid, map[string]interface{}{
		"needs_recovery": false,
		"recovered_at":   time.Now(),
	})

	return nil
}

// CheckAndRecoverDue 检查并恢复到期的虚拟机
func (m *Manager) CheckAndRecoverDue() error {
	dueStates := m.stateManager.GetRecoveryDueStates()

	if len(dueStates) == 0 {
		return nil
	}

	log.Printf("自动恢复 %d 个虚拟机", len(dueStates))

	for _, state := range dueStates {
		if err := m.RecoverVM(state.VMID); err != nil {
			log.Printf("VM%d 恢复失败: %v", state.VMID, err)
		}
	}

	return nil
}

// RecoverAll 恢复所有虚拟机（程序退出时调用）
func (m *Manager) RecoverAll() error {
	states := m.stateManager.GetAllStates()

	if len(states) == 0 {
		return nil
	}

	log.Printf("恢复 %d 个虚拟机", len(states))

	for _, state := range states {
		if err := m.RecoverVM(state.VMID); err != nil {
			log.Printf("VM%d 恢复失败: %v", state.VMID, err)
		}
	}

	return nil
}

// CleanupAllTags 清理所有虚拟机的流量标签
func (m *Manager) CleanupAllTags(vms []models.VMInfo) error {
	for _, vm := range vms {
		// 获取虚拟机的所有标签
		tags, err := m.pveClient.GetVMTags(vm.VMID)
		if err != nil {
			log.Printf("获取虚拟机 %d 标签失败: %v", vm.VMID, err)
			continue
		}

		// 移除所有以 traffic- 开头的标签（包括规则特定的标签）
		for _, tag := range tags {
			if len(tag) >= 8 && tag[:8] == "traffic-" {
				m.pveClient.RemoveVMTag(vm.VMID, tag)
			}
		}
	}

	return nil
}

// LoadStatesFromStorage 从存储加载状态（程序启动时）
func (m *Manager) LoadStatesFromStorage() error {
	// TODO: 从存储中加载所有虚拟机状态
	return nil
}

// calculateRecoveryTime 计算恢复时间
func calculateRecoveryTime(period string, useCreationTime bool, creationTime time.Time) time.Time {
	now := time.Now()

	if useCreationTime && !creationTime.IsZero() {
		// 基于创建时间计算下一个周期开始时间
		return calculateNextPeriodStart(period, creationTime, now)
	}

	// 使用固定周期
	switch period {
	case "hour":
		// 下一个小时的开始
		return time.Date(now.Year(), now.Month(), now.Day(), now.Hour()+1, 0, 0, 0, now.Location())
	case "day":
		// 明天的开始
		return time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	case "month":
		// 下个月的开始
		return time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location())
	default:
		// 默认一小时后
		return now.Add(1 * time.Hour)
	}
}

// calculateNextPeriodStart 计算基于创建时间的下一个周期开始时间
func calculateNextPeriodStart(period string, creationTime, now time.Time) time.Time {
	creation := creationTime

	switch period {
	case "hour":
		// 计算从创建到现在经过了多少小时，然后加1得到下一个周期
		hoursSinceCreation := int(now.Sub(creation).Hours())
		return creation.Add(time.Duration(hoursSinceCreation+1) * time.Hour)

	case "day":
		// 计算从创建到现在经过了多少天，然后加1得到下一个周期
		daysSinceCreation := int(now.Sub(creation).Hours() / 24)
		return creation.AddDate(0, 0, daysSinceCreation+1)

	case "month":
		// 计算下一个周期（下个月的同一天同一时刻）
		creationDay := creation.Day()
		creationHour := creation.Hour()
		creationMinute := creation.Minute()

		monthsSinceCreation := (now.Year()-creation.Year())*12 + int(now.Month()-creation.Month())

		// 如果当前还没到创建时刻，说明还在当前周期
		if now.Day() < creationDay || (now.Day() == creationDay && now.Hour() < creationHour) {
			// 下一个周期就是当前月的创建日期
			monthsSinceCreation++
		} else {
			// 下一个周期是下个月的创建日期
			monthsSinceCreation++
		}

		nextPeriod := creation.AddDate(0, monthsSinceCreation, 0)

		// 处理月份天数不同的情况
		if nextPeriod.Month() != creation.AddDate(0, monthsSinceCreation, 0).Month() {
			// 如果目标月份没有那么多天，使用该月最后一天
			nextPeriod = time.Date(nextPeriod.Year(), nextPeriod.Month()+1, 0,
				creationHour, creationMinute, 0, 0, nextPeriod.Location())
		} else {
			nextPeriod = time.Date(nextPeriod.Year(), nextPeriod.Month(), creationDay,
				creationHour, creationMinute, 0, 0, nextPeriod.Location())
		}

		return nextPeriod

	default:
		return now.Add(1 * time.Hour)
	}
}
