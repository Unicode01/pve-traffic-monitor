package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"pve-traffic-monitor/pkg/models"
	"pve-traffic-monitor/pkg/pve"
	"pve-traffic-monitor/pkg/storage"
	"strconv"
	"sync"
	"time"
)

// Server API 服务器
type Server struct {
	config    *models.Config
	storage   storage.Interface
	pveClient *pve.Client
	mux       *http.ServeMux
	cache     *Cache
	perfStats *PerformanceStats // 性能统计
}

// PerformanceStats 性能统计
type PerformanceStats struct {
	mu               sync.RWMutex
	totalRequests    int64
	totalDuration    time.Duration
	requestDurations []time.Duration // 最近100个请求的处理时间
	maxDuration      time.Duration
	minDuration      time.Duration
}

// Cache 简单的内存缓存
type Cache struct {
	data map[string]*CacheEntry
	mu   sync.RWMutex
}

type CacheEntry struct {
	Data      interface{}
	ExpiresAt time.Time
}

// NewServer 创建新的 API 服务器
func NewServer(config *models.Config, storage storage.Interface, pveClient *pve.Client) *Server {
	s := &Server{
		config:    config,
		storage:   storage,
		pveClient: pveClient,
		mux:       http.NewServeMux(),
		cache: &Cache{
			data: make(map[string]*CacheEntry),
		},
		perfStats: &PerformanceStats{
			requestDurations: make([]time.Duration, 0, 100),
			minDuration:      time.Hour, // 初始值设大一些
		},
	}

	s.setupRoutes()

	// 启动缓存清理协程
	go s.cleanExpiredCache()

	return s
}

// recordRequest 记录请求性能
func (p *PerformanceStats) recordRequest(duration time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.totalRequests++
	p.totalDuration += duration

	// 保留最近100个请求的时间
	if len(p.requestDurations) >= 100 {
		p.requestDurations = p.requestDurations[1:]
	}
	p.requestDurations = append(p.requestDurations, duration)

	// 更新最大最小值
	if duration > p.maxDuration {
		p.maxDuration = duration
	}
	if duration < p.minDuration {
		p.minDuration = duration
	}
}

// getStats 获取性能统计
func (p *PerformanceStats) getStats() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	avgDuration := float64(0)
	if p.totalRequests > 0 {
		avgDuration = float64(p.totalDuration.Milliseconds()) / float64(p.totalRequests)
	}

	// 计算最近100个请求的平均值
	recent100Avg := float64(0)
	if len(p.requestDurations) > 0 {
		var sum time.Duration
		for _, d := range p.requestDurations {
			sum += d
		}
		recent100Avg = float64(sum.Milliseconds()) / float64(len(p.requestDurations))
	}

	return map[string]interface{}{
		"total_requests":   p.totalRequests,
		"avg_response_ms":  avgDuration,
		"recent100_avg_ms": recent100Avg,
		"max_response_ms":  p.maxDuration.Milliseconds(),
		"min_response_ms":  p.minDuration.Milliseconds(),
	}
}

// cleanExpiredCache 定期清理过期缓存
func (s *Server) cleanExpiredCache() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.cache.mu.Lock()
		now := time.Now()
		for key, entry := range s.cache.data {
			if now.After(entry.ExpiresAt) {
				delete(s.cache.data, key)
			}
		}
		s.cache.mu.Unlock()
	}
}

// getStats 获取缓存统计
func (c *Cache) getStats() (total int, expired int) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total = len(c.data)
	now := time.Now()

	for _, entry := range c.data {
		if now.After(entry.ExpiresAt) {
			expired++
		}
	}

	return total, expired
}

// getCache 获取缓存
func (s *Server) getCache(key string) (interface{}, bool) {
	s.cache.mu.RLock()
	defer s.cache.mu.RUnlock()

	entry, exists := s.cache.data[key]
	if !exists {
		return nil, false
	}

	if time.Now().After(entry.ExpiresAt) {
		return nil, false
	}

	return entry.Data, true
}

// setCache 设置缓存
func (s *Server) setCache(key string, data interface{}, ttl time.Duration) {
	s.cache.mu.Lock()
	defer s.cache.mu.Unlock()

	s.cache.data[key] = &CacheEntry{
		Data:      data,
		ExpiresAt: time.Now().Add(ttl),
	}
}

// performanceMiddleware 性能监控中间件
func (s *Server) performanceMiddleware(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		handler(w, r)
		duration := time.Since(start)
		s.perfStats.recordRequest(duration)
	}
}

// setupRoutes 设置路由
func (s *Server) setupRoutes() {
	// API 路由
	s.mux.HandleFunc("/api/vms", s.performanceMiddleware(s.handleVMs))
	s.mux.HandleFunc("/api/vm/", s.performanceMiddleware(s.handleVM))
	s.mux.HandleFunc("/api/stats", s.performanceMiddleware(s.handleStats))
	s.mux.HandleFunc("/api/history/", s.performanceMiddleware(s.handleHistory))
	s.mux.HandleFunc("/api/logs", s.performanceMiddleware(s.handleLogs))
	s.mux.HandleFunc("/api/rules", s.performanceMiddleware(s.handleRules))
	s.mux.HandleFunc("/api/system/stats", s.performanceMiddleware(s.handleSystemStats))

	// 静态文件（前端）
	// 优先使用构建后的web/dist目录，如果不存在则使用内嵌的简化版本
	webDir := "./web/dist"
	if _, err := os.Stat(webDir); err == nil {
		// 存在构建后的前端文件，支持 SPA 路由
		s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			// 如果是 API 路径，返回 404
			if len(r.URL.Path) >= 5 && r.URL.Path[:5] == "/api/" {
				http.NotFound(w, r)
				return
			}

			// 尝试读取文件
			filePath := filepath.Join(webDir, r.URL.Path)
			info, err := os.Stat(filePath)

			// 如果文件不存在或是目录，返回 index.html（SPA 路由）
			if err != nil || info.IsDir() {
				http.ServeFile(w, r, filepath.Join(webDir, "index.html"))
				return
			}

			// 文件存在，直接返回
			http.ServeFile(w, r, filePath)
		})
	} else {
		// 使用内嵌的简化版本
		s.mux.HandleFunc("/", s.handleIndex)
	}
}

// Start 启动 API 服务器
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.API.Host, s.config.API.Port)
	log.Printf("API 服务器: http://%s\n", addr)
	return http.ListenAndServe(addr, s.corsMiddleware(s.mux))
}

// corsMiddleware CORS 中间件
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// handleIndex 首页
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	html := `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>PVE Traffic Monitor</title>
    <script src="https://cdn.jsdelivr.net/npm/chart.js@4.4.1/dist/chart.umd.min.js"></script>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #f5f5f5; }
        .container { max-width: 1400px; margin: 0 auto; padding: 20px; }
        header { background: #2c3e50; color: white; padding: 20px; border-radius: 8px; margin-bottom: 20px; display: flex; justify-content: space-between; align-items: center; }
        h1 { font-size: 24px; }
        .tabs { display: flex; gap: 10px; }
        .tab { background: rgba(255,255,255,0.2); color: white; border: none; padding: 8px 16px; border-radius: 4px; cursor: pointer; transition: all 0.3s; }
        .tab:hover, .tab.active { background: rgba(255,255,255,0.3); }
        .stats { display: grid; grid-template-columns: repeat(auto-fit, minmax(250px, 1fr)); gap: 20px; margin-bottom: 20px; }
        .stat-card { background: white; padding: 20px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .stat-card h3 { color: #7f8c8d; font-size: 14px; margin-bottom: 10px; }
        .stat-card .value { font-size: 32px; font-weight: bold; color: #2c3e50; }
        .card { background: white; border-radius: 8px; padding: 20px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); margin-bottom: 20px; }
        .controls { display: flex; gap: 10px; margin-bottom: 20px; align-items: center; }
        .controls select, .controls button { padding: 8px 16px; border: 1px solid #ddd; border-radius: 4px; background: white; cursor: pointer; }
        .controls button { background: #3498db; color: white; border: none; }
        .controls button:hover { background: #2980b9; }
        table { width: 100%; border-collapse: collapse; }
        th, td { padding: 12px; text-align: left; border-bottom: 1px solid #ecf0f1; }
        th { background: #34495e; color: white; font-weight: 500; }
        tr:hover { background: #f8f9fa; }
        tr.clickable { cursor: pointer; }
        .status { display: inline-block; padding: 4px 12px; border-radius: 12px; font-size: 12px; font-weight: 500; }
        .status.running { background: #d4edda; color: #155724; }
        .status.stopped { background: #f8d7da; color: #721c24; }
        .traffic { color: #3498db; font-weight: 500; }
        .loading { text-align: center; padding: 40px; color: #7f8c8d; }
        .page { display: none; }
        .page.active { display: block; }
        .chart-container { position: relative; height: 400px; }
        .modal { display: none; position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.5); z-index: 1000; }
        .modal.active { display: flex; align-items: center; justify-content: center; }
        .modal-content { background: white; border-radius: 8px; padding: 20px; max-width: 90%; max-height: 90%; overflow: auto; }
        .modal-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; }
        .modal-close { background: #e74c3c; color: white; border: none; padding: 8px 16px; border-radius: 4px; cursor: pointer; }
    </style>
</head>
<body>
    <div class="container">
        <header>
            <h1>PVE Traffic Monitor</h1>
            <div class="tabs">
                <button class="tab active" onclick="switchTab('overview')">Overview</button>
                <button class="tab" onclick="switchTab('charts')">Charts</button>
            </div>
        </header>
        
        <div id="overview-page" class="page active">
            <button class="controls button" onclick="loadData()" style="background: #3498db; color: white; border: none; padding: 10px 20px; border-radius: 4px; cursor: pointer; margin-bottom: 20px;">Refresh</button>
            
            <div class="stats" id="stats">
                <div class="stat-card">
                    <h3>Total VMs</h3>
                    <div class="value" id="total-vms">-</div>
                </div>
                <div class="stat-card">
                    <h3>Running</h3>
                    <div class="value" id="running-vms">-</div>
                </div>
                <div class="stat-card">
                    <h3>Today's Traffic</h3>
                    <div class="value" id="total-traffic">-</div>
                </div>
                <div class="stat-card">
                    <h3>Total Samples</h3>
                    <div class="value" id="total-samples">-</div>
                </div>
                <div class="stat-card">
                    <h3>API Avg Response</h3>
                    <div class="value" id="api-avg-time">-</div>
                </div>
            </div>
            
            <div class="card">
                <h2 style="margin-bottom: 20px;">Virtual Machines</h2>
                <div id="vm-list" class="loading">Loading...</div>
            </div>
        </div>

        <div id="charts-page" class="page">
            <div class="controls">
                <label>Period:</label>
                <select id="chart-period">
                    <option value="minute">Current Minute</option>
                    <option value="hour">Current Hour</option>
                    <option value="day" selected>Today</option>
                    <option value="month">Current Month</option>
                </select>
                <label>Direction:</label>
                <select id="chart-direction">
                    <option value="both" selected>Both</option>
                    <option value="download">Download</option>
                    <option value="upload">Upload</option>
                </select>
                <button onclick="loadChartData()">Update Chart</button>
            </div>

            <div class="card">
                <h2 style="margin-bottom: 20px;">Traffic Overview</h2>
                <div class="chart-container">
                    <canvas id="overview-chart"></canvas>
                </div>
            </div>

            <div class="card">
                <h2 style="margin-bottom: 20px;">Top 10 VMs by Traffic</h2>
                <div class="chart-container">
                    <canvas id="top-vms-chart"></canvas>
                </div>
            </div>
        </div>
    </div>

    <!-- VM Detail Modal -->
    <div id="vm-modal" class="modal">
        <div class="modal-content" style="width: 90%; max-width: 1200px;">
            <div class="modal-header">
                <h2 id="modal-title">VM Details</h2>
                <button class="modal-close" onclick="closeModal()">Close</button>
            </div>
            <div class="controls" style="margin-bottom: 20px;">
                <label>Period:</label>
                <select id="vm-detail-period">
                    <option value="minute">Minute (Last Hour)</option>
                    <option value="hour">Hour</option>
                    <option value="day" selected>Day</option>
                    <option value="month">Month</option>
                </select>
                <button onclick="reloadVMDetail()">Update</button>
            </div>
            <div class="chart-container" style="height: 400px;">
                <canvas id="vm-detail-chart"></canvas>
            </div>
            <div id="vm-detail-stats" style="margin-top: 20px;"></div>
        </div>
    </div>
    
    <script>
        let overviewChart = null;
        let topVMsChart = null;
        let vmDetailChart = null;

        function formatBytes(bytes) {
            if (bytes === 0) return '0 B';
            const k = 1024;
            const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
            const i = Math.floor(Math.log(bytes) / Math.log(k));
            return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
        }

        function switchTab(tab) {
            document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
            document.querySelectorAll('.page').forEach(p => p.classList.remove('active'));
            
            if (tab === 'overview') {
                document.querySelector('.tab:nth-child(1)').classList.add('active');
                document.getElementById('overview-page').classList.add('active');
                loadData();
            } else if (tab === 'charts') {
                document.querySelector('.tab:nth-child(2)').classList.add('active');
                document.getElementById('charts-page').classList.add('active');
                loadChartData();
            }
        }
        
        async function loadData() {
            try {
                const response = await fetch('/api/vms');
                const data = await response.json();
                
                if (data.success) {
                    cachedVMs = data.data;
                    updateStats(data.data);
                    renderVMs(data.data);
                }
            } catch (error) {
                console.error('Failed to load data:', error);
                document.getElementById('vm-list').innerHTML = '<p style="color: red;">Failed to load data</p>';
            }
        }
        
        async function updateStats(vms) {
            const total = vms.length;
            const running = vms.filter(vm => vm.status === 'running').length;
            
            try {
                // 获取流量统计
                const statsResp = await fetch('/api/stats?period=day&direction=both');
                const statsData = await statsResp.json();
                let totalTrafficBytes = 0;
                
                if (statsData.success && statsData.data) {
                    totalTrafficBytes = statsData.data.reduce((sum, stat) => sum + stat.total_bytes, 0);
                }
                
                // 获取系统统计
                const sysStatsResp = await fetch('/api/system/stats');
                const sysStatsData = await sysStatsResp.json();
                
                document.getElementById('total-vms').textContent = total;
                document.getElementById('running-vms').textContent = running;
                document.getElementById('total-traffic').textContent = formatBytes(totalTrafficBytes);
                
                if (sysStatsData.success && sysStatsData.data) {
                    const data = sysStatsData.data;
                    
                    // 显示总采样点数
                    const totalRecords = data.total_records || 0;
                    if (totalRecords >= 1000000) {
                        document.getElementById('total-samples').textContent = (totalRecords / 1000000).toFixed(2) + 'M';
                    } else if (totalRecords >= 1000) {
                        document.getElementById('total-samples').textContent = (totalRecords / 1000).toFixed(1) + 'K';
                    } else {
                        document.getElementById('total-samples').textContent = totalRecords;
                    }
                    
                    // 显示API平均响应时间
                    const avgTime = data.api_performance?.recent100_avg_ms || data.api_performance?.avg_response_ms || 0;
                    document.getElementById('api-avg-time').textContent = avgTime.toFixed(1) + ' ms';
                    
                    // 存储系统统计供其他地方使用
                    window.systemStats = data;
                }
            } catch (error) {
                console.error('Failed to update stats:', error);
                document.getElementById('total-vms').textContent = total;
                document.getElementById('running-vms').textContent = running;
                document.getElementById('total-traffic').textContent = 'Loading...';
                document.getElementById('total-samples').textContent = '-';
                document.getElementById('api-avg-time').textContent = '-';
            }
        }
        
        function renderVMs(vms) {
            // 按VM ID排序（升序），确保每次显示顺序一致
            const sortedVMs = [...vms].sort((a, b) => a.vmid - b.vmid);
            
            const html = '<table><thead><tr><th onclick="sortVMs(\'vmid\')">ID ▲</th><th onclick="sortVMs(\'name\')">Name</th><th onclick="sortVMs(\'status\')">Status</th><th>Matched Rules</th><th onclick="sortVMs(\'download\')">Download</th><th onclick="sortVMs(\'upload\')">Upload</th><th onclick="sortVMs(\'total\')">Total</th><th>Action</th></tr></thead><tbody>' +
                sortedVMs.map(vm => '<tr class="clickable">' +
                    '<td>' + vm.vmid + '</td>' +
                    '<td>' + vm.name + '</td>' +
                    '<td><span class="status ' + vm.status + '">' + (vm.status === 'running' ? 'Running' : 'Stopped') + '</span></td>' +
                    '<td>' + (vm.matched_rules || []).join(', ') + '</td>' +
                    '<td class="traffic">' + formatBytes(vm.netrx || 0) + '</td>' +
                    '<td class="traffic">' + formatBytes(vm.nettx || 0) + '</td>' +
                    '<td class="traffic"><strong>' + formatBytes((vm.netrx || 0) + (vm.nettx || 0)) + '</strong></td>' +
                    '<td><button onclick="showVMDetail(' + vm.vmid + ', \'' + vm.name + '\'); event.stopPropagation();" style="padding: 4px 12px; background: #3498db; color: white; border: none; border-radius: 4px; cursor: pointer;">Details</button></td>' +
                '</tr>').join('') +
                '</tbody></table>';
            
            document.getElementById('vm-list').innerHTML = html;
        }

        let currentSort = { field: 'vmid', order: 'asc' };
        let cachedVMs = [];

        function sortVMs(field) {
            if (currentSort.field === field) {
                currentSort.order = currentSort.order === 'asc' ? 'desc' : 'asc';
            } else {
                currentSort.field = field;
                currentSort.order = 'asc';
            }

            const sorted = [...cachedVMs].sort((a, b) => {
                let valA, valB;
                switch(field) {
                    case 'vmid':
                        valA = a.vmid;
                        valB = b.vmid;
                        break;
                    case 'name':
                        valA = (a.name || '').toLowerCase();
                        valB = (b.name || '').toLowerCase();
                        break;
                    case 'status':
                        valA = a.status;
                        valB = b.status;
                        break;
                    case 'download':
                        valA = a.netrx || 0;
                        valB = b.netrx || 0;
                        break;
                    case 'upload':
                        valA = a.nettx || 0;
                        valB = b.nettx || 0;
                        break;
                    case 'total':
                        valA = (a.netrx || 0) + (a.nettx || 0);
                        valB = (b.netrx || 0) + (b.nettx || 0);
                        break;
                    default:
                        return 0;
                }
                
                if (currentSort.order === 'asc') {
                    return valA > valB ? 1 : valA < valB ? -1 : 0;
                } else {
                    return valA < valB ? 1 : valA > valB ? -1 : 0;
                }
            });

            renderVMsFromCache(sorted);
        }

        function renderVMsFromCache(vms) {
            const html = '<table><thead><tr>' +
                '<th onclick="sortVMs(\'vmid\')" style="cursor: pointer;">ID ' + (currentSort.field === 'vmid' ? (currentSort.order === 'asc' ? '▲' : '▼') : '') + '</th>' +
                '<th onclick="sortVMs(\'name\')" style="cursor: pointer;">Name ' + (currentSort.field === 'name' ? (currentSort.order === 'asc' ? '▲' : '▼') : '') + '</th>' +
                '<th onclick="sortVMs(\'status\')" style="cursor: pointer;">Status ' + (currentSort.field === 'status' ? (currentSort.order === 'asc' ? '▲' : '▼') : '') + '</th>' +
                '<th>Matched Rules</th>' +
                '<th onclick="sortVMs(\'download\')" style="cursor: pointer;">Download ' + (currentSort.field === 'download' ? (currentSort.order === 'asc' ? '▲' : '▼') : '') + '</th>' +
                '<th onclick="sortVMs(\'upload\')" style="cursor: pointer;">Upload ' + (currentSort.field === 'upload' ? (currentSort.order === 'asc' ? '▲' : '▼') : '') + '</th>' +
                '<th onclick="sortVMs(\'total\')" style="cursor: pointer;">Total ' + (currentSort.field === 'total' ? (currentSort.order === 'asc' ? '▲' : '▼') : '') + '</th>' +
                '<th>Action</th>' +
                '</tr></thead><tbody>' +
                vms.map(vm => '<tr class="clickable">' +
                    '<td>' + vm.vmid + '</td>' +
                    '<td>' + vm.name + '</td>' +
                    '<td><span class="status ' + vm.status + '">' + (vm.status === 'running' ? 'Running' : 'Stopped') + '</span></td>' +
                    '<td>' + (vm.matched_rules || []).join(', ') + '</td>' +
                    '<td class="traffic">' + formatBytes(vm.netrx || 0) + '</td>' +
                    '<td class="traffic">' + formatBytes(vm.nettx || 0) + '</td>' +
                    '<td class="traffic"><strong>' + formatBytes((vm.netrx || 0) + (vm.nettx || 0)) + '</strong></td>' +
                    '<td><button onclick="showVMDetail(' + vm.vmid + ', \'' + vm.name + '\'); event.stopPropagation();" style="padding: 4px 12px; background: #3498db; color: white; border: none; border-radius: 4px; cursor: pointer;">Details</button></td>' +
                '</tr>').join('') +
                '</tbody></table>';
            
            document.getElementById('vm-list').innerHTML = html;
        }

        async function loadChartData() {
            const period = document.getElementById('chart-period').value;
            const direction = document.getElementById('chart-direction').value;

            try {
                const response = await fetch('/api/stats?period=' + period + '&direction=' + direction);
                const data = await response.json();

                if (data.success && data.data) {
                    renderOverviewChart(data.data);
                    renderTopVMsChart(data.data);
                }
            } catch (error) {
                console.error('Failed to load chart data:', error);
            }
        }

        function renderOverviewChart(stats) {
            const ctx = document.getElementById('overview-chart').getContext('2d');
            
            // 按VMID升序排序
            const sorted = [...stats].sort((a, b) => a.vmid - b.vmid);
            
            const labels = sorted.map(s => 'VM' + s.vmid + ' (' + s.name + ')');
            const rxData = sorted.map(s => s.rx_bytes / (1024 * 1024 * 1024)); // 转换为GB显示
            const txData = sorted.map(s => s.tx_bytes / (1024 * 1024 * 1024));
            const totalData = sorted.map(s => s.total_bytes / (1024 * 1024 * 1024));
            
            if (overviewChart) overviewChart.destroy();
            
            overviewChart = new Chart(ctx, {
                type: 'line',
                data: {
                    labels: labels,
                    datasets: [{
                        label: 'Download (GB)',
                        data: rxData,
                        borderColor: 'rgb(54, 162, 235)',
                        backgroundColor: 'rgba(54, 162, 235, 0.1)',
                        tension: 0.2,
                        fill: true,
                        pointRadius: 2
                    }, {
                        label: 'Upload (GB)',
                        data: txData,
                        borderColor: 'rgb(75, 192, 192)',
                        backgroundColor: 'rgba(75, 192, 192, 0.1)',
                        tension: 0.2,
                        fill: true,
                        pointRadius: 2
                    }, {
                        label: 'Total (GB)',
                        data: totalData,
                        borderColor: 'rgb(255, 99, 132)',
                        backgroundColor: 'rgba(255, 99, 132, 0.1)',
                        tension: 0.2,
                        fill: false,
                        borderWidth: 2,
                        pointRadius: 3
                    }]
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    interaction: {
                        mode: 'index',
                        intersect: false
                    },
                    plugins: {
                        title: { display: true, text: 'All VMs Traffic' },
                        legend: { position: 'top' },
                        tooltip: {
                            callbacks: {
                                label: function(context) {
                                    const bytes = context.parsed.y * 1024 * 1024 * 1024;
                                    return context.dataset.label + ': ' + formatBytes(bytes);
                                }
                            }
                        }
                    },
                    scales: {
                        y: { beginAtZero: true, title: { display: true, text: 'Traffic (GB)' } }
                    }
                }
            });
        }

        function renderTopVMsChart(stats) {
            const ctx = document.getElementById('top-vms-chart').getContext('2d');
            
            const sorted = [...stats].sort((a, b) => b.total_bytes - a.total_bytes).slice(0, 10);
            const labels = sorted.map(s => 'VM' + s.vmid + ' (' + s.name + ')');
            const data = sorted.map(s => s.total_bytes / (1024 * 1024 * 1024)); // 转换为GB显示
            
            if (topVMsChart) topVMsChart.destroy();
            
            topVMsChart = new Chart(ctx, {
                type: 'bar',
                data: {
                    labels: labels,
                    datasets: [{
                        label: 'Total Traffic (GB)',
                        data: data,
                        backgroundColor: 'rgba(231, 76, 60, 0.8)',
                        borderColor: 'rgb(231, 76, 60)',
                        borderWidth: 1
                    }]
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    plugins: {
                        title: { display: true, text: 'Top 10 VMs by Traffic' },
                        legend: { display: false },
                        tooltip: {
                            callbacks: {
                                label: function(context) {
                                    const bytes = context.parsed.y * 1024 * 1024 * 1024;
                                    return 'Total Traffic: ' + formatBytes(bytes);
                                }
                            }
                        }
                    },
                    scales: {
                        y: { beginAtZero: true, title: { display: true, text: 'Traffic (GB)' } }
                    }
                }
            });
        }

        let currentVMID = null;
        let currentVMName = '';

        async function showVMDetail(vmid, vmName) {
            currentVMID = vmid;
            currentVMName = vmName;
            document.getElementById('modal-title').textContent = 'VM' + vmid + ' - ' + vmName;
            document.getElementById('vm-modal').classList.add('active');
            
            await reloadVMDetail();
        }

        async function reloadVMDetail() {
            if (!currentVMID) return;

            const period = document.getElementById('vm-detail-period').value;

            try {
                // 加载统计信息
                const statsResponse = await fetch('/api/vm/' + currentVMID);
                const statsData = await statsResponse.json();

                // 加载历史数据
                const historyResponse = await fetch('/api/history/' + currentVMID + '?period=' + period);
                const historyData = await historyResponse.json();

                if (statsData.success && historyData.success) {
                    renderVMDetailChart(historyData.data);
                    renderVMDetailStats(statsData.data.stats);
                }
            } catch (error) {
                console.error('Failed to load VM details:', error);
            }
        }

        function renderVMDetailChart(historyPoints) {
            const ctx = document.getElementById('vm-detail-chart').getContext('2d');
            
            if (vmDetailChart) vmDetailChart.destroy();

            if (!historyPoints || historyPoints.length === 0) {
                ctx.font = '16px sans-serif';
                ctx.fillStyle = '#7f8c8d';
                ctx.textAlign = 'center';
                ctx.fillText('No data available', ctx.canvas.width / 2, ctx.canvas.height / 2);
                return;
            }

            // 后端已经处理了重启，直接使用累积值
            const period = document.getElementById('vm-detail-period').value;
            const labels = historyPoints.map(p => p.timestamp);
            
            // 根据粒度自动选择合适的单位
            let rxData, txData, totalData;
            let yAxisLabel, unit, divisor;
            
            // 计算平均流量大小来决定单位
            const avgBytes = historyPoints.reduce((sum, p) => sum + p.total_bytes, 0) / historyPoints.length;
            
            if (avgBytes < 1024 * 1024 * 100) { // 小于100MB，用MB
                divisor = 1024 * 1024;
                unit = 'MB';
                yAxisLabel = 'Traffic Usage (MB)';
            } else { // 否则用GB
                divisor = 1024 * 1024 * 1024;
                unit = 'GB';
                yAxisLabel = 'Traffic Usage (GB)';
            }
            
            rxData = historyPoints.map(p => p.rx_bytes / divisor);
            txData = historyPoints.map(p => p.tx_bytes / divisor);
            totalData = historyPoints.map(p => p.total_bytes / divisor);
            
            vmDetailChart = new Chart(ctx, {
                type: 'line',
                data: {
                    labels: labels,
                    datasets: [{
                        label: 'Download (GB)',
                        data: rxData,
                        borderColor: 'rgb(54, 162, 235)',
                        backgroundColor: 'rgba(54, 162, 235, 0.1)',
                        tension: 0.2,
                        fill: true,
                        pointRadius: 2
                    }, {
                        label: 'Upload (GB)',
                        data: txData,
                        borderColor: 'rgb(75, 192, 192)',
                        backgroundColor: 'rgba(75, 192, 192, 0.1)',
                        tension: 0.2,
                        fill: true,
                        pointRadius: 2
                    }, {
                        label: 'Total (GB)',
                        data: totalData,
                        borderColor: 'rgb(255, 99, 132)',
                        backgroundColor: 'rgba(255, 99, 132, 0.1)',
                        tension: 0.2,
                        fill: false,
                        borderWidth: 2,
                        pointRadius: 3
                    }]
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    interaction: {
                        mode: 'index',
                        intersect: false
                    },
                    plugins: {
                        title: { display: true, text: 'Traffic Usage Pattern' },
                        legend: { position: 'top' },
                        tooltip: {
                            callbacks: {
                                label: function(context) {
                                    const bytes = context.parsed.y * divisor;
                                    return context.dataset.label + ': ' + formatBytes(bytes);
                                }
                            }
                        }
                    },
                    scales: {
                        x: {
                            display: true,
                            title: { display: true, text: 'Time' },
                            ticks: {
                                maxRotation: 45,
                                minRotation: 45,
                                maxTicksLimit: period === 'minute' ? 30 : 20
                            }
                        },
                        y: {
                            beginAtZero: true,
                            title: { display: true, text: yAxisLabel }
                        }
                    }
                }
            });
        }

        function renderVMDetailStats(stats) {
            const html = '<table><thead><tr><th>Period</th><th>Download</th><th>Upload</th><th>Total</th></tr></thead><tbody>' +
                Object.keys(stats).map(period => {
                    const s = stats[period];
                    return '<tr>' +
                        '<td style="text-transform: capitalize;">' + period + '</td>' +
                        '<td>' + formatBytes(s.rx_bytes || 0) + '</td>' +
                        '<td>' + formatBytes(s.tx_bytes || 0) + '</td>' +
                        '<td><strong>' + formatBytes(s.total_bytes || 0) + '</strong></td>' +
                    '</tr>';
                }).join('') +
                '</tbody></table>';
            
            document.getElementById('vm-detail-stats').innerHTML = html;
        }

        function closeModal() {
            document.getElementById('vm-modal').classList.remove('active');
        }

        // Close modal on outside click
        document.getElementById('vm-modal').addEventListener('click', function(e) {
            if (e.target === this) closeModal();
        });
        
        // Auto refresh
        setInterval(() => {
            const currentTab = document.querySelector('.tab.active');
            if (currentTab && currentTab.textContent === 'Overview') {
                loadData();
            }
        }, 30000);
        
        // Initial load
        loadData();
    </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

// handleVMs 获取所有虚拟机
func (s *Server) handleVMs(w http.ResponseWriter, r *http.Request) {
	vms, err := s.pveClient.GetAllVMs()
	if err != nil {
		s.sendError(w, "获取虚拟机列表失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 应用规则匹配（统一在一处完成）
	vmsWithRules := pve.ApplyRulesToVMs(vms, s.config.Rules)

	s.sendJSON(w, map[string]interface{}{
		"success": true,
		"data":    vmsWithRules,
	})
}

// handleVM 获取单个虚拟机信息
func (s *Server) handleVM(w http.ResponseWriter, r *http.Request) {
	vmidStr := r.URL.Path[len("/api/vm/"):]
	vmid, err := strconv.Atoi(vmidStr)
	if err != nil {
		s.sendError(w, "无效的虚拟机 ID", http.StatusBadRequest)
		return
	}

	vm, err := s.pveClient.GetVMStatus(vmid)
	if err != nil {
		s.sendError(w, "获取虚拟机信息失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 获取流量统计（包含上传/下载分别统计）
	stats := make(map[string]interface{})
	for _, period := range []string{"hour", "day", "month"} {
		stat, err := s.storage.CalculateTrafficStatsWithDirection(vmid, period, time.Time{}, false, "both")
		if err == nil {
			stats[period] = map[string]interface{}{
				"total_bytes": stat.TotalBytes,
				"rx_bytes":    stat.RXBytes,
				"tx_bytes":    stat.TXBytes,
				"start_time":  stat.StartTime,
				"end_time":    stat.EndTime,
			}
		}
	}

	s.sendJSON(w, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"vm":    vm,
			"stats": stats,
		},
	})
}

// handleStats 获取统计信息
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "day"
	}

	direction := r.URL.Query().Get("direction")
	if direction == "" {
		direction = "both"
	}

	vms, err := s.pveClient.GetAllVMsWithFilter(false)
	if err != nil {
		s.sendError(w, "获取虚拟机列表失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	type VMStatsResponse struct {
		VMID       int       `json:"vmid"`
		Name       string    `json:"name"`
		Period     string    `json:"period"`
		Direction  string    `json:"direction"`
		StartTime  time.Time `json:"start_time"`
		EndTime    time.Time `json:"end_time"`
		TotalBytes uint64    `json:"total_bytes"`
		RXBytes    uint64    `json:"rx_bytes"`
		TXBytes    uint64    `json:"tx_bytes"`
	}

	var allStats []VMStatsResponse
	for _, vm := range vms {
		stats, err := s.storage.CalculateTrafficStatsWithDirection(vm.VMID, period, time.Time{}, false, direction)
		if err == nil {
			allStats = append(allStats, VMStatsResponse{
				VMID:       vm.VMID,
				Name:       vm.Name,
				Period:     stats.Period,
				Direction:  stats.Direction,
				StartTime:  stats.StartTime,
				EndTime:    stats.EndTime,
				TotalBytes: stats.TotalBytes,
				RXBytes:    stats.RXBytes,
				TXBytes:    stats.TXBytes,
			})
		}
	}

	s.sendJSON(w, map[string]interface{}{
		"success":   true,
		"data":      allStats,
		"period":    period,
		"direction": direction,
	})
}

// handleLogs 获取操作日志
func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	// 默认获取最近 7 天的日志
	endTime := time.Now()
	startTime := endTime.AddDate(0, 0, -7)

	// 支持自定义时间范围
	if start := r.URL.Query().Get("start"); start != "" {
		if t, err := time.Parse(time.RFC3339, start); err == nil {
			startTime = t
		}
	}
	if end := r.URL.Query().Get("end"); end != "" {
		if t, err := time.Parse(time.RFC3339, end); err == nil {
			endTime = t
		}
	}

	logs, err := s.storage.GetActionLogs(startTime, endTime)
	if err != nil {
		s.sendError(w, "获取日志失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	s.sendJSON(w, map[string]interface{}{
		"success": true,
		"data":    logs,
	})
}

// handleHistory 获取虚拟机历史流量数据（用于图表，按时间段聚合，带缓存）
func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	vmidStr := r.URL.Path[len("/api/history/"):]
	vmid, err := strconv.Atoi(vmidStr)
	if err != nil {
		s.sendError(w, "Invalid VM ID", http.StatusBadRequest)
		return
	}

	period := r.URL.Query().Get("period")
	if period == "" {
		period = "day"
	}

	// 检查缓存
	cacheKey := fmt.Sprintf("history_%d_%s", vmid, period)
	cached, ok := s.getCache(cacheKey)
	if ok {
		s.sendJSON(w, map[string]interface{}{
			"success": true,
			"data":    cached,
			"period":  period,
			"cached":  true,
		})
		return
	}

	// 计算时间范围（限制查询范围以优化性能）
	now := time.Now()
	var startTime time.Time
	var cacheTTL time.Duration

	switch period {
	case models.PeriodMinute:
		startTime = now.Add(-1 * time.Hour) // 最近1小时，用于分钟粒度
		cacheTTL = 30 * time.Second         // 缓存30秒
	case models.PeriodHour:
		startTime = now.Add(-24 * time.Hour) // 最近24小时
		cacheTTL = 1 * time.Minute           // 缓存1分钟
	case models.PeriodDay:
		startTime = now.AddDate(0, 0, -30) // 最近30天
		cacheTTL = 5 * time.Minute         // 缓存5分钟
	case models.PeriodMonth:
		startTime = now.AddDate(0, -12, 0) // 最近12个月
		cacheTTL = 15 * time.Minute        // 缓存15分钟
	default:
		s.sendError(w, "Invalid period", http.StatusBadRequest)
		return
	}

	// 获取历史记录
	records, err := s.storage.GetTrafficRecords(vmid, startTime, now)
	if err != nil {
		s.sendError(w, "Failed to get traffic records: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if len(records) == 0 {
		emptyResult := []map[string]interface{}{}
		s.sendJSON(w, map[string]interface{}{
			"success": true,
			"data":    emptyResult,
			"period":  period,
		})
		return
	}

	// 按时间段聚合数据 - 使用共用的聚合函数
	aggregatedPoints := storage.AggregateTrafficByPeriod(records, period)

	// 转换为API响应格式
	aggregated := make([]map[string]interface{}, len(aggregatedPoints))
	for i, point := range aggregatedPoints {
		aggregated[i] = map[string]interface{}{
			"timestamp":   point.Timestamp.Format(getTimeFormat(period)),
			"rx_bytes":    point.RXBytes,
			"tx_bytes":    point.TXBytes,
			"total_bytes": point.TotalBytes,
		}
	}

	// 缓存结果
	s.setCache(cacheKey, aggregated, cacheTTL)

	s.sendJSON(w, map[string]interface{}{
		"success": true,
		"data":    aggregated,
		"period":  period,
		"cached":  false,
	})
}

// getTimeFormat 根据period获取时间格式
func getTimeFormat(period string) string {
	switch period {
	case models.PeriodMinute, models.PeriodHour:
		return models.TimeFormatMinute
	case models.PeriodDay:
		return models.TimeFormatDay
	case models.PeriodMonth:
		return models.TimeFormatMonth
	default:
		return models.TimeFormatDay
	}
}

// handleRules 获取规则列表
func (s *Server) handleRules(w http.ResponseWriter, r *http.Request) {
	s.sendJSON(w, map[string]interface{}{
		"success": true,
		"data":    s.config.Rules,
	})
}

// handleSystemStats 获取系统统计信息
func (s *Server) handleSystemStats(w http.ResponseWriter, r *http.Request) {
	// 获取总采样点数
	totalRecords, err := s.storage.GetTotalRecordCount()
	if err != nil {
		log.Printf("获取总记录数失败: %v", err)
		totalRecords = 0
	}

	// 获取性能统计
	perfStats := s.perfStats.getStats()

	// 获取缓存统计
	cacheTotal, cacheExpired := s.cache.getStats()

	s.sendJSON(w, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"total_records":    totalRecords,
			"api_performance":  perfStats,
			"cache_total":      cacheTotal,
			"cache_expired":    cacheExpired,
			"storage_type":     s.config.Storage.Type,
			"monitor_interval": s.config.Monitor.IntervalSeconds,
			"data_retention":   s.config.Monitor.DataRetentionDays,
		},
	})
}

// sendJSON 发送 JSON 响应
func (s *Server) sendJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// sendError 发送错误响应
func (s *Server) sendError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"error":   message,
	})
}
