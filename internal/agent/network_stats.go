package internal

import (
	"fmt"
	"sync"
	"time"

	"github.com/ruanun/simple-server-status/pkg/model"

	"github.com/shirou/gopsutil/v4/net"
)

// NetworkStatsCollector 线程安全的网络统计收集器
type NetworkStatsCollector struct {
	mu                 sync.RWMutex
	netInSpeed         uint64
	netOutSpeed        uint64
	netInTransfer      uint64
	netOutTransfer     uint64
	lastUpdateNetStats uint64
	excludeInterfaces  []string
}

// NewNetworkStatsCollector 创建网络统计收集器
func NewNetworkStatsCollector(excludeInterfaces []string) *NetworkStatsCollector {
	if excludeInterfaces == nil {
		excludeInterfaces = []string{
			"lo", "tun", "docker", "veth", "br-", "vmbr", "vnet", "kube",
		}
	}
	return &NetworkStatsCollector{
		excludeInterfaces: excludeInterfaces,
	}
}

// Update 更新网络统计（在单独的 goroutine 中调用）
func (nsc *NetworkStatsCollector) Update() error {
	netIOs, err := net.IOCounters(true)
	if err != nil {
		return fmt.Errorf("获取网络IO统计失败: %w", err)
	}

	var innerNetInTransfer, innerNetOutTransfer uint64
	for _, v := range netIOs {
		if isListContainsStr(nsc.excludeInterfaces, v.Name) {
			continue
		}
		innerNetInTransfer += v.BytesRecv
		innerNetOutTransfer += v.BytesSent
	}

	// time.Now().Unix() 返回的 int64 时间戳在正常情况下总是正数（自1970年以来的秒数）
	// 因此转换为 uint64 是安全的
	//nolint:gosec // G115: Unix时间戳转换为uint64是安全的，时间戳始终为正数
	now := uint64(time.Now().Unix())

	// 使用写锁保护并发写入
	nsc.mu.Lock()
	defer nsc.mu.Unlock()

	diff := now - nsc.lastUpdateNetStats
	if diff > 0 {
		// 检测计数器回绕或网络接口重置
		if innerNetInTransfer >= nsc.netInTransfer {
			nsc.netInSpeed = (innerNetInTransfer - nsc.netInTransfer) / diff
		} else {
			// 发生回绕或重置，从新值开始计算
			nsc.netInSpeed = 0
		}

		if innerNetOutTransfer >= nsc.netOutTransfer {
			nsc.netOutSpeed = (innerNetOutTransfer - nsc.netOutTransfer) / diff
		} else {
			// 发生回绕或重置，从新值开始计算
			nsc.netOutSpeed = 0
		}
	}
	nsc.netInTransfer = innerNetInTransfer
	nsc.netOutTransfer = innerNetOutTransfer
	nsc.lastUpdateNetStats = now

	return nil
}

// GetStats 获取当前网络统计（线程安全）
func (nsc *NetworkStatsCollector) GetStats() *model.NetworkInfo {
	// 使用读锁允许并发读取
	nsc.mu.RLock()
	defer nsc.mu.RUnlock()

	return &model.NetworkInfo{
		NetInSpeed:     nsc.netInSpeed,
		NetOutSpeed:    nsc.netOutSpeed,
		NetInTransfer:  nsc.netInTransfer,
		NetOutTransfer: nsc.netOutTransfer,
	}
}
