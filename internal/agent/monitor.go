package internal

import (
	"context"
	"runtime"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
)

// PerformanceMetrics 性能指标结构
type PerformanceMetrics struct {
	// 系统指标
	CPUUsage    float64 `json:"cpu_usage"`
	MemoryUsage float64 `json:"memory_usage"`
	Goroutines  int     `json:"goroutines"`

	// 网络指标
	NetworkSent     uint64 `json:"network_sent"`
	NetworkReceived uint64 `json:"network_received"`

	// 应用指标
	DataCollections   int64     `json:"data_collections"`
	WebSocketMessages int64     `json:"websocket_messages"`
	Errors            int64     `json:"errors"`
	Uptime            float64   `json:"uptime_seconds"`
	LastUpdate        time.Time `json:"last_update"`
}

// PerformanceMonitor 性能监控器
type PerformanceMonitor struct {
	mu              sync.RWMutex
	metrics         *PerformanceMetrics
	startTime       time.Time
	ctx             context.Context
	cancel          context.CancelFunc
	collectInterval time.Duration
	logInterval     time.Duration
	logger          interface{ Infof(string, ...interface{}) }

	// 计数器
	dataCollectionCount   int64
	webSocketMessageCount int64
	errorCount            int64

	// 网络基线
	lastNetworkSent     uint64
	lastNetworkReceived uint64
}

// NewPerformanceMonitor 创建新的性能监控器
func NewPerformanceMonitor(logger interface{ Infof(string, ...interface{}) }) *PerformanceMonitor {
	ctx, cancel := context.WithCancel(context.Background())
	pm := &PerformanceMonitor{
		metrics: &PerformanceMetrics{
			LastUpdate: time.Now(),
		},
		startTime:       time.Now(),
		ctx:             ctx,
		cancel:          cancel,
		collectInterval: time.Second * 30, // 每30秒收集一次指标
		logInterval:     time.Minute * 5,  // 每5分钟记录一次日志
		logger:          logger,
	}

	// 启动监控
	go pm.start()
	if logger != nil {
		logger.Infof("性能监控器已启动")
	}
	return pm
}

// start 启动监控循环
func (pm *PerformanceMonitor) start() {
	collectTicker := time.NewTicker(pm.collectInterval)
	logTicker := time.NewTicker(pm.logInterval)
	defer collectTicker.Stop()
	defer logTicker.Stop()

	for {
		select {
		case <-pm.ctx.Done():
			return
		case <-collectTicker.C:
			pm.collectMetrics()
		case <-logTicker.C:
			pm.logMetrics()
		}
	}
}

// collectMetrics 收集性能指标
func (pm *PerformanceMonitor) collectMetrics() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// 收集CPU使用率
	cpuPercent, err := cpu.Percent(time.Second, false)
	if err == nil && len(cpuPercent) > 0 {
		pm.metrics.CPUUsage = cpuPercent[0]
	}

	// 收集内存使用率
	vmStat, err := mem.VirtualMemory()
	if err == nil {
		pm.metrics.MemoryUsage = vmStat.UsedPercent
	}

	// 收集Goroutine数量
	pm.metrics.Goroutines = runtime.NumGoroutine()

	// 收集网络统计
	netStats, err := net.IOCounters(false)
	if err == nil && len(netStats) > 0 {
		currentSent := netStats[0].BytesSent
		currentReceived := netStats[0].BytesRecv

		if pm.lastNetworkSent > 0 {
			pm.metrics.NetworkSent = currentSent - pm.lastNetworkSent
			pm.metrics.NetworkReceived = currentReceived - pm.lastNetworkReceived
		}

		pm.lastNetworkSent = currentSent
		pm.lastNetworkReceived = currentReceived
	}

	// 更新应用指标
	pm.metrics.DataCollections = pm.dataCollectionCount
	pm.metrics.WebSocketMessages = pm.webSocketMessageCount
	pm.metrics.Errors = pm.errorCount
	pm.metrics.Uptime = time.Since(pm.startTime).Seconds()
	pm.metrics.LastUpdate = time.Now()
}

// logMetrics 记录性能指标到日志
func (pm *PerformanceMonitor) logMetrics() {
	if pm.logger == nil {
		return
	}

	pm.mu.RLock()
	metrics := *pm.metrics // 复制一份避免长时间持锁
	pm.mu.RUnlock()

	pm.logger.Infof("性能指标 - CPU: %.2f%%, 内存: %.2f%%, Goroutines: %d, 运行时间: %.0fs",
		metrics.CPUUsage, metrics.MemoryUsage, metrics.Goroutines, metrics.Uptime)

	pm.logger.Infof("应用指标 - 数据收集: %d次, WebSocket消息: %d条, 错误: %d个",
		metrics.DataCollections, metrics.WebSocketMessages, metrics.Errors)

	if metrics.NetworkSent > 0 || metrics.NetworkReceived > 0 {
		pm.logger.Infof("网络指标 - 发送: %d字节, 接收: %d字节",
			metrics.NetworkSent, metrics.NetworkReceived)
	}
}

// GetMetrics 获取当前性能指标
func (pm *PerformanceMonitor) GetMetrics() *PerformanceMetrics {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	// 返回指标的副本
	metricsCopy := *pm.metrics
	return &metricsCopy
}

// IncrementDataCollection 增加数据收集计数
func (pm *PerformanceMonitor) IncrementDataCollection() {
	pm.mu.Lock()
	pm.dataCollectionCount++
	pm.mu.Unlock()
}

// IncrementWebSocketMessage 增加WebSocket消息计数
func (pm *PerformanceMonitor) IncrementWebSocketMessage() {
	pm.mu.Lock()
	pm.webSocketMessageCount++
	pm.mu.Unlock()
}

// IncrementError 增加错误计数
func (pm *PerformanceMonitor) IncrementError() {
	pm.mu.Lock()
	pm.errorCount++
	pm.mu.Unlock()
}

// Close 关闭性能监控器
func (pm *PerformanceMonitor) Close() {
	pm.cancel()
	if pm.logger != nil {
		pm.logger.Infof("性能监控器已关闭")
	}
}
