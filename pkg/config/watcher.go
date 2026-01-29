package config

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Watcher 配置文件监视器
type Watcher struct {
	loader       *Loader
	signalChan   chan os.Signal
	stopChan     chan struct{}
	autoInterval time.Duration
}

// NewWatcher 创建配置监视器
func NewWatcher(loader *Loader) *Watcher {
	return &Watcher{
		loader:       loader,
		signalChan:   make(chan os.Signal, 1),
		stopChan:     make(chan struct{}),
		autoInterval: 30 * time.Second, // 默认 30 秒检查一次
	}
}

// SetAutoInterval 设置自动检查间隔
func (w *Watcher) SetAutoInterval(interval time.Duration) {
	w.autoInterval = interval
}

// Start 启动监视器
func (w *Watcher) Start() {
	// 监听 SIGHUP 信号（用于手动触发重载）
	signal.Notify(w.signalChan, syscall.SIGHUP)

	log.Printf("配置监视器已启动 (自动检查间隔: %v)", w.autoInterval)

	// 启动自动重载
	if w.autoInterval > 0 {
		w.loader.StartAutoReload(w.autoInterval)
	}

	// 监听信号
	go w.watchSignals()
}

// Stop 停止监视器
func (w *Watcher) Stop() {
	close(w.stopChan)
	signal.Stop(w.signalChan)
	log.Println("配置监视器已停止")
}

// watchSignals 监听信号
func (w *Watcher) watchSignals() {
	for {
		select {
		case <-w.signalChan:
			log.Println("收到 SIGHUP 信号，重载配置...")
			if err := w.loader.Reload(); err != nil {
				log.Printf("配置重载失败: %v", err)
			}
		case <-w.stopChan:
			return
		}
	}
}

// TriggerReload 手动触发重载
func (w *Watcher) TriggerReload() error {
	log.Println("手动触发配置重载")
	return w.loader.Reload()
}
