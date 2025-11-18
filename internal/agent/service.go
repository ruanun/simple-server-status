package internal

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ruanun/simple-server-status/internal/agent/config"
	"go.uber.org/zap"
)

// AgentService 聚合所有 Agent 组件的服务
type AgentService struct {
	// 配置和日志
	config *config.AgentConfig
	logger *zap.SugaredLogger

	// 核心组件
	wsClient     *WsClient
	monitor      *PerformanceMonitor
	memoryPool   *MemoryPoolManager
	errorHandler *ErrorHandler

	// 服务器信息
	hostIp       string // 服务器IP地址
	hostLocation string // 服务器地理位置

	// 生命周期管理
	ctx    context.Context
	cancel context.CancelFunc
}

// NewAgentService 创建新的 Agent 服务
// 使用依赖注入模式，所有依赖通过参数传递
func NewAgentService(cfg *config.AgentConfig, logger *zap.SugaredLogger) (*AgentService, error) {
	if cfg == nil {
		return nil, fmt.Errorf("配置不能为空")
	}
	if logger == nil {
		return nil, fmt.Errorf("日志对象不能为空")
	}

	// 创建服务上下文
	ctx, cancel := context.WithCancel(context.Background())

	// 创建服务实例
	service := &AgentService{
		config: cfg,
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
	}

	// 初始化组件
	if err := service.initComponents(); err != nil {
		cancel() // 清理上下文
		return nil, fmt.Errorf("初始化组件失败: %w", err)
	}

	return service, nil
}

// initComponents 初始化所有组件
func (s *AgentService) initComponents() error {
	// 1. 初始化内存池（无依赖）
	s.memoryPool = NewMemoryPoolManager()
	s.logger.Info("内存池已初始化")

	// 2. 初始化性能监控器（依赖 logger）
	s.monitor = NewPerformanceMonitor(s.logger)
	s.logger.Info("性能监控器已初始化")

	// 3. 初始化错误处理器（依赖 logger, monitor）
	s.errorHandler = NewErrorHandler(s.logger, s.monitor)
	s.logger.Info("错误处理器已初始化")

	// 4. 初始化 WebSocket 客户端（依赖所有组件）
	s.wsClient = NewWsClient(s.config, s.logger, s.errorHandler, s.memoryPool, s.monitor)
	s.logger.Info("WebSocket 客户端已初始化")

	return nil
}

// Start 启动服务
func (s *AgentService) Start() error {
	s.logger.Info("启动 Agent 服务...")

	// 启动 WebSocket 客户端
	s.wsClient.Start()

	// 启动业务任务（数据收集和上报）
	go s.startTasks()

	s.logger.Info("Agent 服务已启动")
	return nil
}

// startTasks 启动业务任务
func (s *AgentService) startTasks() {
	// 获取服务器 IP 和位置
	if !s.config.DisableIP2Region {
		go s.getServerLocAndIp()
	}

	// 定时统计网络速度、流量信息
	go s.statNetInfo()

	// 定时上报信息
	go s.reportInfo()
}

// statNetInfo 统计网络信息
func (s *AgentService) statNetInfo() {
	defer func() {
		if err := recover(); err != nil {
			s.logger.Error("StatNetworkSpeed panic: ", err)
		}
	}()

	ticker := time.NewTicker(time.Second * 1)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			s.logger.Info("网络统计 goroutine 正常退出")
			return
		case <-ticker.C:
			StatNetworkSpeed()
		}
	}
}

// getServerLocAndIp 获取服务器位置和 IP
func (s *AgentService) getServerLocAndIp() {
	defer func() {
		if err := recover(); err != nil {
			s.logger.Errorf("getServerLocAndIp panic: %v", err)
		}
	}()

	s.logger.Debug("getServerLocAndIp start")

	// 随机选择一个 URL
	urls := []string{
		"https://cloudflare.com/cdn-cgi/trace",
		"https://developers.cloudflare.com/cdn-cgi/trace",
		"https://blog.cloudflare.com/cdn-cgi/trace",
		"https://info.cloudflare.com/cdn-cgi/trace",
		"https://store.ubi.com/cdn-cgi/trace",
	}
	url := urls[RandomIntInRange(0, len(urls)-1)]
	s.logger.Debugf("getServerLocAndIp url: %s", url)

	// 创建带超时的 HTTP 客户端
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// 发送 GET 请求
	resp, err := client.Get(url)
	if err != nil {
		s.logger.Warnf("Failed to fetch IP location from %s: %v", url, err)
		return
	}
	defer resp.Body.Close()

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		s.logger.Warnf("Failed to read response body: %v", err)
		return
	}

	// 解析响应 (格式: key=value，每行一个)
	lines := strings.Split(string(body), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "ip":
			s.hostIp = value
			s.logger.Infof("Server IP detected: %s", value)
		case "loc":
			s.hostLocation = value
			s.logger.Infof("Server location detected: %s", value)
		}
	}

	if s.hostIp == "" {
		s.logger.Warn("Failed to parse IP from response")
	}
	if s.hostLocation == "" {
		s.logger.Warn("Failed to parse location from response")
	}
}

// reportInfo 上报信息
func (s *AgentService) reportInfo() {
	defer func() {
		if err := recover(); err != nil {
			panicErr := NewAppError(ErrorTypeSystem, SeverityCritical, "reportInfo panic", fmt.Errorf("%v", err))
			s.errorHandler.HandleError(panicErr)
			s.monitor.IncrementError()
		}
	}()
	defer s.logger.Info("reportInfo 正常退出")

	s.logger.Debug("reportInfo start")

	// 创建自适应收集器
	adaptiveCollector := NewAdaptiveCollector(s.config.ReportTimeInterval, s.logger)
	s.logger.Info("Adaptive collection strategy enabled")

	// 定期更新收集间隔的 goroutine
	go func() {
		ticker := time.NewTicker(time.Second * 10)
		defer ticker.Stop()

		for {
			select {
			case <-s.ctx.Done():
				s.logger.Info("自适应收集器更新 goroutine 正常退出")
				return
			case <-ticker.C:
				adaptiveCollector.UpdateInterval()
			}
		}
	}()

	// 主上报循环
	ticker := time.NewTicker(time.Second * 1)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			s.logger.Info("数据上报 goroutine 正常退出")
			return
		case <-ticker.C:
			serverInfo := GetServerInfo(s.hostIp, s.hostLocation)

			// 记录数据收集事件
			s.monitor.IncrementDataCollection()

			// 通过 WebSocket 发送
			s.wsClient.SendJsonMsg(serverInfo)

			// 记录发送事件
			s.monitor.IncrementWebSocketMessage()

			// 使用自适应间隔，重置 ticker
			currentInterval := adaptiveCollector.GetCurrentInterval()
			ticker.Reset(currentInterval)
		}
	}
}

// Stop 停止服务
func (s *AgentService) Stop(timeout time.Duration) error {
	s.logger.Info("停止 Agent 服务...")

	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// 1. 发送取消信号
	s.cancel()

	// 2. 等待一小段时间让 goroutine 处理取消信号
	time.Sleep(time.Millisecond * 200)

	// 3. 关闭 WebSocket 客户端
	done := make(chan struct{})
	go func() {
		if s.wsClient != nil {
			s.wsClient.Close()
		}
		close(done)
	}()

	select {
	case <-done:
		s.logger.Info("WebSocket 客户端已关闭")
	case <-ctx.Done():
		s.logger.Warn("WebSocket 客户端关闭超时")
	}

	// 4. 关闭性能监控器
	if s.monitor != nil {
		s.monitor.Close()
	}

	// 5. 记录内存池统计
	if s.memoryPool != nil {
		s.memoryPool.LogStats(s.logger)
	}

	// 6. 记录错误统计
	if s.errorHandler != nil {
		s.errorHandler.LogErrorStats()
	}

	s.logger.Info("Agent 服务已停止")
	return nil
}

// GetMetrics 获取性能指标（用于外部监控）
func (s *AgentService) GetMetrics() *PerformanceMetrics {
	if s.monitor != nil {
		return s.monitor.GetMetrics()
	}
	return nil
}

// GetErrorStats 获取错误统计（用于外部监控）
func (s *AgentService) GetErrorStats() map[ErrorType]int64 {
	if s.errorHandler != nil {
		return s.errorHandler.GetErrorStats()
	}
	return nil
}

// GetMemoryPoolStats 获取内存池统计（用于外部监控）
func (s *AgentService) GetMemoryPoolStats() PoolStats {
	if s.memoryPool != nil {
		return s.memoryPool.GetStats()
	}
	return PoolStats{}
}

// GetWSStats 获取 WebSocket 统计（用于外部监控）
func (s *AgentService) GetWSStats() map[string]int64 {
	if s.wsClient != nil {
		return s.wsClient.GetStats()
	}
	return nil
}
