package recovery

import (
	"fmt"
	"log"
	"pve-traffic-monitor/pkg/models"
	periodcalc "pve-traffic-monitor/pkg/period"
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
	if state, exists := m.stateManager.GetState(vmid); exists &&
		state.NeedsRecovery &&
		action == models.ActionRateLimit &&
		state.ActionTaken == models.ActionRateLimit {
		return nil
	}

	// 获取当前虚拟机状态
	vmInfo, err := m.pveClient.GetVMStatus(vmid)
	if err != nil {
		return fmt.Errorf("获取虚拟机状态失败: %w", err)
	}

	// 获取当前速率限制（如果有）
	rateLimit := 0.0
	networkRates := map[string]float64{}
	networkLinks := map[string]bool{}
	config, err := m.pveClient.GetVMConfig(vmid)
	if err == nil {
		parsedRates, err := pve.NetworkRateLimitsFromConfig(config)
		if err == nil {
			networkRates = parsedRates
			if parsedRate, err := pve.NetworkRateLimitFromConfig(config); err == nil {
				rateLimit = parsedRate
			}
		}
		if parsedLinks, err := pve.NetworkLinkDownStatesFromConfig(config); err == nil {
			networkLinks = parsedLinks
		}
	}

	// 计算恢复时间（支持基于创建时间的周期）
	recoveryTime := calculateRecoveryTime(period, useCreationTime, creationTime)

	state := &models.VMState{
		VMID:              vmid,
		OriginalStatus:    vmInfo.Status,
		OriginalRateLimit: rateLimit,
		OriginalNetRates:  networkRates,
		OriginalNetLinks:  networkLinks,
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
		"original_net_rates":  state.OriginalNetRates,
		"original_net_links":  state.OriginalNetLinks,
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
		if len(state.OriginalNetLinks) > 0 {
			if err := m.pveClient.RestoreNetworkLinkStates(vmid, state.OriginalNetLinks); err != nil {
				return fmt.Errorf("恢复网络失败: %w", err)
			}
		} else if err := m.pveClient.ConnectNetwork(vmid); err != nil {
			return fmt.Errorf("恢复网络失败: %w", err)
		}

	case "rate_limit":
		// 恢复原始速率限制
		if len(state.OriginalNetRates) > 0 {
			if err := m.pveClient.RestoreNetworkRateLimits(vmid, state.OriginalNetRates); err != nil {
				return fmt.Errorf("恢复网络速率限制失败: %w", err)
			}
		} else if state.OriginalRateLimit == 0 {
			if err := m.pveClient.RemoveNetworkRateLimit(vmid); err != nil {
				return fmt.Errorf("移除网络速率限制失败: %w", err)
			}
		} else {
			if err := m.pveClient.SetNetworkRateLimit(vmid, state.OriginalRateLimit); err != nil {
				return fmt.Errorf("恢复网络速率限制失败: %w", err)
			}
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
	return periodcalc.CalculateNextCreationBasedPeriodStart(period, creationTime, now)
}
