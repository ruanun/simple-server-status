package internal

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/ruanun/simple-server-status/internal/dashboard/config"
	"github.com/ruanun/simple-server-status/internal/dashboard/handler"
	"github.com/ruanun/simple-server-status/pkg/model"
	"go.uber.org/zap"
)

// DashboardService 聚合所有 Dashboard 组件的服务
type DashboardService struct {
	// 配置和日志
	config *config.DashboardConfig
	logger *zap.SugaredLogger

	// 核心组件
	httpServer        *http.Server
	wsManager         *WebSocketManager
	frontendWsManager *FrontendWebSocketManager
	errorHandler      *ErrorHandler
	configValidator   *ConfigValidator
	ginEngine         *gin.Engine

	// 状态管理
	servers         cmap.ConcurrentMap[string, *config.ServerConfig] // 服务器配置 map
	serverStatusMap cmap.ConcurrentMap[string, *model.ServerInfo]    // 服务器状态 map

	// 生命周期管理
	ctx    context.Context
	cancel context.CancelFunc
}

// NewDashboardService 创建新的 Dashboard 服务
// 使用依赖注入模式，所有依赖通过参数传递
func NewDashboardService(cfg *config.DashboardConfig, logger *zap.SugaredLogger, ginEngine *gin.Engine, errorHandler *ErrorHandler) (*DashboardService, error) {
	if cfg == nil {
		return nil, fmt.Errorf("配置不能为空")
	}
	if logger == nil {
		return nil, fmt.Errorf("日志对象不能为空")
	}
	if ginEngine == nil {
		return nil, fmt.Errorf("gin 引擎不能为空")
	}
	if errorHandler == nil {
		return nil, fmt.Errorf("错误处理器不能为空")
	}

	// 创建服务上下文
	ctx, cancel := context.WithCancel(context.Background())

	// 创建服务实例
	service := &DashboardService{
		config:          cfg,
		logger:          logger,
		ginEngine:       ginEngine,
		errorHandler:    errorHandler,
		servers:         cmap.New[*config.ServerConfig](),
		serverStatusMap: cmap.New[*model.ServerInfo](),
		ctx:             ctx,
		cancel:          cancel,
	}

	// 从配置中加载服务器列表
	for _, server := range cfg.Servers {
		service.servers.Set(server.Id, server)
	}

	// 初始化组件
	if err := service.initComponents(); err != nil {
		cancel() // 清理上下文
		return nil, fmt.Errorf("初始化组件失败: %w", err)
	}

	return service, nil
}

// initComponents 初始化所有组件
func (s *DashboardService) initComponents() error {
	// 注意：errorHandler 和 ginEngine 已通过构造函数参数传入

	// 1. 初始化配置验证器
	s.configValidator = NewConfigValidator()
	s.logger.Info("配置验证器已初始化")

	// 3. 初始化 WebSocket 管理器（Agent 连接）
	serverConfigAdapter := &serverConfigAdapter{servers: s.servers}
	serverStatusAdapter := &serverStatusAdapter{statusMap: s.serverStatusMap}
	s.wsManager = NewWebSocketManager(s.logger, s.errorHandler, serverConfigAdapter, serverStatusAdapter, s)
	s.logger.Info("WebSocket 管理器已初始化")

	// 4. 初始化前端 WebSocket 管理器
	serverStatusIteratorAdapter := &serverStatusIteratorAdapter{statusMap: s.serverStatusMap}
	s.frontendWsManager = NewFrontendWebSocketManager(s.logger, s.errorHandler, serverStatusIteratorAdapter, s)
	s.logger.Info("前端 WebSocket 管理器已初始化")

	// 5. 设置 WebSocket 路由
	s.wsManager.SetupRoutes(s.ginEngine)
	s.frontendWsManager.SetupFrontendRoutes(s.ginEngine)

	// 6. 设置 API 路由（依赖 wsManager）
	s.setupAPIRoutes()

	// 7. 初始化 HTTP 服务器
	address := fmt.Sprintf("%s:%d", s.config.Address, s.config.Port)
	s.httpServer = &http.Server{
		Addr:              address,
		Handler:           s.ginEngine,
		ReadHeaderTimeout: 10 * time.Second, // 防止 Slowloris 攻击
		ReadTimeout:       30 * time.Second, // 读取整个请求的超时时间
		WriteTimeout:      30 * time.Second, // 写入响应的超时时间
		IdleTimeout:       60 * time.Second, // Keep-Alive 连接的空闲超时时间
	}
	s.logger.Infof("HTTP 服务器已初始化，监听地址: %s", address)

	return nil
}

// setupAPIRoutes 设置 API 路由
func (s *DashboardService) setupAPIRoutes() {
	// 导入 handler 包以调用 InitApi
	// 创建适配器以满足接口要求
	serverStatusMapAdapter := &serverStatusMapAdapter{statusMap: s.serverStatusMap}
	serverConfigMapAdapter := &serverConfigMapAdapter{servers: s.servers}
	configValidatorAdapter := &configValidatorAdapter{validator: s.configValidator}
	handler.InitApi(s.ginEngine, s.wsManager, s, s.logger, serverStatusMapAdapter, serverConfigMapAdapter, configValidatorAdapter)
	s.logger.Info("API 路由已初始化")
}

// serverConfigAdapter 服务器配置适配器
// 用于将 ConcurrentMap 适配到 ServerConfigProvider 接口
type serverConfigAdapter struct {
	servers cmap.ConcurrentMap[string, *config.ServerConfig]
}

func (a *serverConfigAdapter) Get(key string) (*config.ServerConfig, bool) {
	return a.servers.Get(key)
}

// serverStatusAdapter 服务器状态适配器
// 用于将 ConcurrentMap 适配到 ServerStatusProvider 接口
type serverStatusAdapter struct {
	statusMap cmap.ConcurrentMap[string, *model.ServerInfo]
}

func (a *serverStatusAdapter) Set(key string, val *model.ServerInfo) {
	a.statusMap.Set(key, val)
}

// serverStatusIteratorAdapter 服务器状态迭代器适配器
// 用于将 ConcurrentMap 适配到 ServerStatusIterator 接口
type serverStatusIteratorAdapter struct {
	statusMap cmap.ConcurrentMap[string, *model.ServerInfo]
}

func (a *serverStatusIteratorAdapter) IterBuffered() <-chan cmap.Tuple[string, *model.ServerInfo] {
	return a.statusMap.IterBuffered()
}

// serverStatusMapAdapter 服务器状态 Map 适配器
// 用于将 ConcurrentMap 适配到 ServerStatusMapProvider 接口
type serverStatusMapAdapter struct {
	statusMap cmap.ConcurrentMap[string, *model.ServerInfo]
}

func (a *serverStatusMapAdapter) Count() int {
	return a.statusMap.Count()
}

func (a *serverStatusMapAdapter) Items() map[string]*model.ServerInfo {
	return a.statusMap.Items()
}

// serverConfigMapAdapter 服务器配置 Map 适配器
// 用于将 ConcurrentMap 适配到 ServerConfigMapProvider 接口
type serverConfigMapAdapter struct {
	servers cmap.ConcurrentMap[string, *config.ServerConfig]
}

func (a *serverConfigMapAdapter) Count() int {
	return a.servers.Count()
}

// configValidatorAdapter 配置验证器适配器
// 用于将 ConfigValidator 适配到 handler.ConfigValidatorProvider 接口
type configValidatorAdapter struct {
	validator *ConfigValidator
}

func (a *configValidatorAdapter) ValidateConfig(cfg *config.DashboardConfig) error {
	return a.validator.ValidateConfig(cfg)
}

func (a *configValidatorAdapter) GetValidationErrors() []handler.ConfigValidationError {
	errors := a.validator.GetValidationErrors()
	// 转换为 handler 包的类型
	result := make([]handler.ConfigValidationError, len(errors))
	for i, err := range errors {
		result[i] = handler.ConfigValidationError{
			Field:   err.Field,
			Value:   err.Value,
			Message: err.Message,
			Level:   err.Level,
		}
	}
	return result
}

func (a *configValidatorAdapter) GetErrorsByLevel(level string) []handler.ConfigValidationError {
	errors := a.validator.GetErrorsByLevel(level)
	// 转换为 handler 包的类型
	result := make([]handler.ConfigValidationError, len(errors))
	for i, err := range errors {
		result[i] = handler.ConfigValidationError{
			Field:   err.Field,
			Value:   err.Value,
			Message: err.Message,
			Level:   err.Level,
		}
	}
	return result
}

// Start 启动服务
func (s *DashboardService) Start() error {
	s.logger.Info("启动 Dashboard 服务...")

	// 在后台启动 HTTP 服务器
	go func() {
		s.logger.Infof("webserver start %s", s.httpServer.Addr)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Fatalf("webserver start failed: %v", err)
		}
	}()

	s.logger.Info("Dashboard 服务已启动")
	return nil
}

// Stop 停止服务
func (s *DashboardService) Stop(timeout time.Duration) error {
	s.logger.Info("停止 Dashboard 服务...")

	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// 1. 发送取消信号
	s.cancel()

	// 2. 关闭 WebSocket 管理器
	s.logger.Info("关闭 WebSocket 管理器...")
	if s.wsManager != nil {
		s.wsManager.Close()
	}
	if s.frontendWsManager != nil {
		s.frontendWsManager.Close()
	}

	// 3. 关闭 HTTP 服务器
	s.logger.Info("关闭 HTTP 服务器...")
	if err := s.httpServer.Shutdown(ctx); err != nil {
		s.logger.Errorf("HTTP 服务器关闭失败: %v", err)
		return err
	}

	// 4. 记录错误统计（如果有）
	// TODO: 添加错误统计记录方法

	s.logger.Info("Dashboard 服务已停止")
	return nil
}

// GetHTTPServer 获取 HTTP 服务器（用于测试）
func (s *DashboardService) GetHTTPServer() *http.Server {
	return s.httpServer
}

// GetWSManager 获取 WebSocket 管理器（用于外部访问）
func (s *DashboardService) GetWSManager() *WebSocketManager {
	return s.wsManager
}

// GetFrontendWSManager 获取前端 WebSocket 管理器（用于外部访问）
func (s *DashboardService) GetFrontendWSManager() *FrontendWebSocketManager {
	return s.frontendWsManager
}

// GetConfig 获取配置（用于外部访问）
func (s *DashboardService) GetConfig() *config.DashboardConfig {
	return s.config
}

// GetServers 获取服务器配置 map（用于外部访问）
func (s *DashboardService) GetServers() cmap.ConcurrentMap[string, *config.ServerConfig] {
	return s.servers
}

// ReloadServers 重新加载服务器配置（用于配置热加载）
func (s *DashboardService) ReloadServers(newServers []*config.ServerConfig) {
	// 1. 构建新服务器 ID 集合
	newServerIDs := make(map[string]bool)
	for _, server := range newServers {
		newServerIDs[server.Id] = true
	}

	// 2. 找出被删除的服务器 ID
	oldServerIDs := s.servers.Keys()
	var removedServerIDs []string
	for _, oldID := range oldServerIDs {
		if !newServerIDs[oldID] {
			removedServerIDs = append(removedServerIDs, oldID)
		}
	}

	// 3. 断开被删除服务器的 WebSocket 连接
	for _, serverID := range removedServerIDs {
		s.wsManager.DelByServerId(serverID)
		s.logger.Infof("配置热加载：断开服务器 %s 的 WebSocket 连接", serverID)
	}

	// 4. 清理被删除服务器的状态数据
	for _, serverID := range removedServerIDs {
		s.serverStatusMap.Remove(serverID)
		s.logger.Infof("配置热加载：删除服务器 %s 的状态数据", serverID)
	}

	// 5. 更新服务器配置 map
	for _, key := range oldServerIDs {
		s.servers.Remove(key)
	}
	for _, server := range newServers {
		s.servers.Set(server.Id, server)
	}

	s.logger.Infof("已重新加载 %d 个服务器配置，删除 %d 个废弃服务器",
		len(newServers), len(removedServerIDs))
}

// GetServerStatusMap 获取服务器状态 map（用于外部访问）
func (s *DashboardService) GetServerStatusMap() cmap.ConcurrentMap[string, *model.ServerInfo] {
	return s.serverStatusMap
}

// GetStats 获取服务统计信息
func (s *DashboardService) GetStats() map[string]interface{} {
	stats := make(map[string]interface{})

	// Agent WebSocket 统计
	if s.wsManager != nil {
		stats["agent_ws_stats"] = s.wsManager.GetStats()
	}

	// 前端 WebSocket 统计
	if s.frontendWsManager != nil {
		stats["frontend_ws_stats"] = s.frontendWsManager.GetStats()
	}

	// 错误统计
	if s.errorHandler != nil {
		stats["error_stats"] = s.errorHandler.GetErrorStats()
	}

	// 服务器统计
	serverStats := map[string]interface{}{
		"total_servers":     s.servers.Count(),
		"monitored_servers": s.serverStatusMap.Count(),
	}

	// 计算在线/离线服务器
	onlineCount := 0
	now := time.Now().Unix()
	for item := range s.serverStatusMap.IterBuffered() {
		if now-item.Val.LastReportTime <= int64(s.config.ReportTimeIntervalMax) {
			onlineCount++
		}
	}
	serverStats["online_servers"] = onlineCount
	serverStats["offline_servers"] = s.servers.Count() - onlineCount
	stats["server_stats"] = serverStats

	return stats
}
