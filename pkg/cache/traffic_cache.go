package cache

import (
	"fmt"
	"pve-traffic-monitor/pkg/models"
	"sync"
	"time"
)

// TrafficCache 流量统计缓存
type TrafficCache struct {
	mu    sync.RWMutex
	cache map[string]*CachedStats
	ttl   time.Duration
}

// CachedStats 缓存的统计数据
type CachedStats struct {
	Stats       *models.TrafficStats
	LastUpdate  time.Time
	PeriodStart time.Time // 周期开始时间，用于检测周期切换
}

// NewTrafficCache 创建新的流量缓存
func NewTrafficCache(ttl time.Duration) *TrafficCache {
	if ttl == 0 {
		ttl = 5 * time.Minute // 默认5分钟
	}

	cache := &TrafficCache{
		cache: make(map[string]*CachedStats),
		ttl:   ttl,
	}

	// 启动定期清理过期缓存的goroutine
	go cache.cleanupLoop()

	return cache
}

// Get 获取缓存的统计数据
func (c *TrafficCache) Get(vmid int, period string, direction string, periodStart time.Time) (*models.TrafficStats, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := c.makeKey(vmid, period, direction)
	cached, exists := c.cache[key]
	if !exists {
		return nil, false
	}

	// 检查周期是否改变（如跨天、跨月）
	if !cached.PeriodStart.Equal(periodStart) {
		return nil, false
	}

	// 检查是否过期
	if time.Since(cached.LastUpdate) > c.ttl {
		return nil, false
	}

	return cached.Stats, true
}

// Set 设置缓存的统计数据
func (c *TrafficCache) Set(vmid int, period string, direction string, periodStart time.Time, stats *models.TrafficStats) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := c.makeKey(vmid, period, direction)
	c.cache[key] = &CachedStats{
		Stats:       stats,
		LastUpdate:  time.Now(),
		PeriodStart: periodStart,
	}
}

// Invalidate 使指定VM的缓存失效
func (c *TrafficCache) Invalidate(vmid int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 删除该VM的所有缓存
	for key := range c.cache {
		var keyVMID int
		fmt.Sscanf(key, "%d:", &keyVMID)
		if keyVMID == vmid {
			delete(c.cache, key)
		}
	}
}

// Clear 清空所有缓存
func (c *TrafficCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = make(map[string]*CachedStats)
}

// GetStats 获取缓存统计信息
func (c *TrafficCache) GetStats() (total, expired int) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total = len(c.cache)
	now := time.Now()

	for _, cached := range c.cache {
		if now.Sub(cached.LastUpdate) > c.ttl {
			expired++
		}
	}

	return total, expired
}

// makeKey 生成缓存键
func (c *TrafficCache) makeKey(vmid int, period string, direction string) string {
	return fmt.Sprintf("%d:%s:%s", vmid, period, direction)
}

// cleanupLoop 定期清理过期缓存
func (c *TrafficCache) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.cleanup()
	}
}

// cleanup 清理过期缓存
func (c *TrafficCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, cached := range c.cache {
		if now.Sub(cached.LastUpdate) > c.ttl {
			delete(c.cache, key)
		}
	}
}
