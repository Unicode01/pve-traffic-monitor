package models

import "time"

// VMState 虚拟机状态记录
type VMState struct {
	VMID              int       `json:"vmid"`
	OriginalStatus    string    `json:"original_status"`     // 原始运行状态 (running/stopped)
	OriginalRateLimit float64   `json:"original_rate_limit"` // 原始速率限制 MB/s (0表示无限制)
	ActionTaken       string    `json:"action_taken"`        // 执行的操作 (shutdown/rate_limit)
	ActionTime        time.Time `json:"action_time"`         // 操作执行时间
	Period            string    `json:"period"`              // 记录周期 (hour/day/month)
	RuleName          string    `json:"rule_name"`           // 触发的规则名称
	NeedsRecovery     bool      `json:"needs_recovery"`      // 是否需要恢复
	RecoveryTime      time.Time `json:"recovery_time"`       // 计划恢复时间
}

// VMStateManager 虚拟机状态管理器
type VMStateManager struct {
	States map[int]*VMState `json:"states"` // VMID -> VMState
}

// NewVMStateManager 创建新的状态管理器
func NewVMStateManager() *VMStateManager {
	return &VMStateManager{
		States: make(map[int]*VMState),
	}
}

// RecordState 记录虚拟机状态
func (m *VMStateManager) RecordState(state *VMState) {
	m.States[state.VMID] = state
}

// GetState 获取虚拟机状态
func (m *VMStateManager) GetState(vmid int) (*VMState, bool) {
	state, exists := m.States[vmid]
	return state, exists
}

// RemoveState 移除虚拟机状态记录
func (m *VMStateManager) RemoveState(vmid int) {
	delete(m.States, vmid)
}

// GetAllStates 获取所有需要恢复的虚拟机状态
func (m *VMStateManager) GetAllStates() []*VMState {
	states := make([]*VMState, 0, len(m.States))
	for _, state := range m.States {
		states = append(states, state)
	}
	return states
}

// GetRecoveryDueStates 获取到期需要恢复的虚拟机状态
func (m *VMStateManager) GetRecoveryDueStates() []*VMState {
	now := time.Now()
	states := make([]*VMState, 0)
	for _, state := range m.States {
		if state.NeedsRecovery && now.After(state.RecoveryTime) {
			states = append(states, state)
		}
	}
	return states
}
