package models

import (
	"errors"
	"fmt"
)

// Validate 验证配置的有效性
func (c *Config) Validate() error {
	// 验证 PVE 配置
	if err := c.PVE.Validate(); err != nil {
		return fmt.Errorf("PVE配置错误: %w", err)
	}

	// 验证监控配置
	if err := c.Monitor.Validate(); err != nil {
		return fmt.Errorf("监控配置错误: %w", err)
	}

	// 验证存储配置
	if err := c.Storage.Validate(); err != nil {
		return fmt.Errorf("存储配置错误: %w", err)
	}

	// 验证API配置
	if err := c.API.Validate(); err != nil {
		return fmt.Errorf("API配置错误: %w", err)
	}

	// 验证规则配置
	for i, rule := range c.Rules {
		if err := rule.Validate(); err != nil {
			return fmt.Errorf("规则 #%d (%s) 配置错误: %w", i+1, rule.Name, err)
		}
	}

	return nil
}

// Validate 验证 PVE 配置
func (p *PVEConfig) Validate() error {
	if p.Host == "" {
		return errors.New("host不能为空")
	}

	if p.Port <= 0 || p.Port > 65535 {
		return fmt.Errorf("port必须在1-65535之间，当前值: %d", p.Port)
	}

	if p.Node == "" {
		return errors.New("node不能为空")
	}

	if p.APITokenID == "" {
		return errors.New("api_token_id不能为空")
	}

	if p.APITokenSecret == "" {
		return errors.New("api_token_secret不能为空")
	}

	return nil
}

// Validate 验证监控配置
func (m *MonitorConfig) Validate() error {
	if m.IntervalSeconds <= 0 {
		return fmt.Errorf("interval_seconds必须大于0，当前值: %d", m.IntervalSeconds)
	}

	if m.IntervalSeconds < 10 {
		return fmt.Errorf("interval_seconds建议不小于10秒，当前值: %d", m.IntervalSeconds)
	}

	if m.ExportPath == "" {
		return errors.New("export_path不能为空")
	}

	if m.DataRetentionDays < 0 {
		return fmt.Errorf("data_retention_days不能为负数，当前值: %d", m.DataRetentionDays)
	}

	return nil
}

// Validate 验证存储配置
func (s *StorageConfig) Validate() error {
	if s.Type == "" {
		return errors.New("type不能为空")
	}

	validTypes := map[string]bool{
		"file":       true,
		"mysql":      true,
		"postgresql": true,
		"sqlite":     true,
	}

	if !validTypes[s.Type] {
		return fmt.Errorf("不支持的存储类型: %s (支持: file, mysql, postgresql, sqlite)", s.Type)
	}

	// 验证文件存储配置
	if s.Type == "file" && s.FilePath == "" {
		return errors.New("文件存储需要指定file_path")
	}

	// 验证数据库存储配置
	if s.Type != "file" && s.DSN == "" {
		return fmt.Errorf("%s存储需要指定dsn", s.Type)
	}

	return nil
}

// Validate 验证 API 配置
func (a *APIConfig) Validate() error {
	if a.Enabled {
		if a.Port <= 0 || a.Port > 65535 {
			return fmt.Errorf("port必须在1-65535之间，当前值: %d", a.Port)
		}

		if a.Host == "" {
			return errors.New("host不能为空")
		}
	}

	return nil
}

// Validate 验证规则配置
func (r *Rule) Validate() error {
	if r.Name == "" {
		return errors.New("name不能为空")
	}

	// 验证周期
	validPeriods := map[string]bool{
		PeriodHour:  true,
		PeriodDay:   true,
		PeriodMonth: true,
	}

	if !validPeriods[r.Period] {
		return fmt.Errorf("不支持的周期: %s (支持: hour, day, month)", r.Period)
	}

	// 验证流量方向
	if r.TrafficDirection != "" {
		validDirections := map[string]bool{
			DirectionBoth:     true,
			DirectionUpload:   true,
			DirectionTX:       true,
			DirectionDownload: true,
			DirectionRX:       true,
		}

		if !validDirections[r.TrafficDirection] {
			return fmt.Errorf("不支持的流量方向: %s (支持: both, upload, download, tx, rx)", r.TrafficDirection)
		}
	}

	// 验证限制值
	if r.LimitGB <= 0 {
		return fmt.Errorf("limit_gb必须大于0，当前值: %.2f", r.LimitGB)
	}

	// 验证操作
	validActions := map[string]bool{
		ActionShutdown:   true,
		ActionStop:       true,
		ActionDisconnect: true,
		ActionRateLimit:  true,
	}

	if !validActions[r.Action] {
		return fmt.Errorf("不支持的操作: %s (支持: shutdown, stop, disconnect, rate_limit)", r.Action)
	}

	// 验证限速值
	if r.Action == ActionRateLimit && r.RateLimitMB <= 0 {
		return fmt.Errorf("rate_limit操作需要指定rate_limit_mb且必��大于0，当前值: %.2f", r.RateLimitMB)
	}

	// 至少要有一个匹配条件
	if len(r.VMIDs) == 0 && len(r.VMTags) == 0 {
		return errors.New("至少需要指定vm_ids或vm_tags之一")
	}

	return nil
}
