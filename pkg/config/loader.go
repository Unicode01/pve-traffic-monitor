package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"pve-traffic-monitor/pkg/models"
	"sync"
	"time"
)

// Loader 配置加载器
type Loader struct {
	configPath   string
	config       *models.Config
	mu           sync.RWMutex
	lastModified time.Time
	callbacks    []func(*models.Config)
}

// NewLoader 创建配置加载器
func NewLoader(configPath string) (*Loader, error) {
	loader := &Loader{
		configPath: configPath,
		callbacks:  make([]func(*models.Config), 0),
	}

	// 首次加载配置
	if err := loader.Reload(); err != nil {
		return nil, fmt.Errorf("加载配置失败: %w", err)
	}

	return loader, nil
}

// Reload 重新加载配置
func (l *Loader) Reload() error {
	// 检查文件是否修改
	fileInfo, err := os.Stat(l.configPath)
	if err != nil {
		return fmt.Errorf("获取配置文件信息失败: %w", err)
	}

	// 如果文件没有修改，跳过重载
	if !l.lastModified.IsZero() && !fileInfo.ModTime().After(l.lastModified) {
		return nil
	}

	// 读取配置文件
	data, err := os.ReadFile(l.configPath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	// 解析配置
	var newConfig models.Config
	if err := json.Unmarshal(data, &newConfig); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 验证配置
	if err := l.validateConfig(&newConfig); err != nil {
		return fmt.Errorf("配置验证失败: %w", err)
	}

	// 更新配置
	l.mu.Lock()
	l.config = &newConfig
	l.lastModified = fileInfo.ModTime()
	l.mu.Unlock()

	log.Println("配置文件已重载")

	// 通知所有回调函数
	l.notifyCallbacks(&newConfig)

	return nil
}

// GetConfig 获取当前配置（线程安全）
func (l *Loader) GetConfig() *models.Config {
	l.mu.RLock()
	defer l.mu.RUnlock()

	// 返回配置的副本，避免外部修改
	configCopy := *l.config
	return &configCopy
}

// OnReload 注册配置重载回调函数
func (l *Loader) OnReload(callback func(*models.Config)) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.callbacks = append(l.callbacks, callback)
}

// notifyCallbacks 通知所有回调函数
func (l *Loader) notifyCallbacks(config *models.Config) {
	l.mu.RLock()
	callbacks := make([]func(*models.Config), len(l.callbacks))
	copy(callbacks, l.callbacks)
	l.mu.RUnlock()

	for _, callback := range callbacks {
		go func(cb func(*models.Config)) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("配置重载回调函数执行失败: %v\n", r)
				}
			}()
			cb(config)
		}(callback)
	}
}

// StartAutoReload 启动自动重载（定期检查文件修改）
func (l *Loader) StartAutoReload(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			if err := l.Reload(); err != nil {
				log.Printf("自动重载配置失败: %v", err)
			}
		}
	}()
}

// validateConfig 验证配置
func (l *Loader) validateConfig(config *models.Config) error {
	// 验证 PVE 配置
	if config.PVE.Host == "" {
		return fmt.Errorf("PVE 主机地址不能为空")
	}
	if config.PVE.Port <= 0 || config.PVE.Port > 65535 {
		return fmt.Errorf("PVE 端口无效: %d", config.PVE.Port)
	}
	if config.PVE.Node == "" {
		return fmt.Errorf("PVE 节点名称不能为空")
	}

	// 验证监控配置
	if config.Monitor.IntervalSeconds <= 0 {
		return fmt.Errorf("监控间隔必须大于 0")
	}

	// 验证存储配置
	if config.Storage.Type == "" {
		config.Storage.Type = "file" // 默认使用文件存储
	}

	storageType := config.Storage.Type
	switch storageType {
	case "file":
		if config.Storage.FilePath == "" {
			return fmt.Errorf("文件存储路径不能为空")
		}
	case "mysql", "postgres", "postgresql", "sqlite", "sqlite3":
		if config.Storage.DSN == "" {
			return fmt.Errorf("数据库连接字符串不能为空")
		}
	default:
		return fmt.Errorf("不支持的存储类型: %s (支持: file, mysql, postgresql, sqlite)", storageType)
	}

	// 验证规则
	for i, rule := range config.Rules {
		if rule.Name == "" {
			return fmt.Errorf("规则 #%d 名称不能为空", i)
		}
		if rule.Period != "hour" && rule.Period != "day" && rule.Period != "month" {
			return fmt.Errorf("规则 %s 周期无效: %s", rule.Name, rule.Period)
		}
		if rule.LimitGB <= 0 {
			return fmt.Errorf("规则 %s 流量限制必须大于 0", rule.Name)
		}
		// 验证操作类型
		validActions := map[string]bool{
			"shutdown":   true,
			"stop":       true,
			"disconnect": true,
			"rate_limit": true,
		}
		if !validActions[rule.Action] {
			return fmt.Errorf("规则 %s 操作无效: %s (支持: shutdown, stop, disconnect, rate_limit)", rule.Name, rule.Action)
		}

		// 验证限速值
		if rule.Action == "rate_limit" && rule.RateLimitMB <= 0 {
			return fmt.Errorf("规则 %s 限速值必须大于 0 MB/s", rule.Name)
		}

		// 验证流量方向
		if rule.TrafficDirection != "" {
			validDirections := map[string]bool{
				"both":     true,
				"upload":   true,
				"download": true,
				"tx":       true,
				"rx":       true,
			}
			if !validDirections[rule.TrafficDirection] {
				return fmt.Errorf("规则 %s 流量方向无效: %s (支持: both, upload, download)", rule.Name, rule.TrafficDirection)
			}
		}
	}

	return nil
}

// GetLastModified 获取配置文件最后修改时间
func (l *Loader) GetLastModified() time.Time {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.lastModified
}
