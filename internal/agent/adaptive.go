package internal

import (
	"math"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
)

// AdaptiveCollector 自适应数据收集器
type AdaptiveCollector struct {
	mu                  sync.RWMutex
	currentInterval     time.Duration
	baseInterval        time.Duration
	maxInterval         time.Duration
	minInterval         time.Duration
	lastCPUUsage        float64
	lastMemUsage        float64
	consecutiveHighLoad int
	consecutiveLowLoad  int
	highLoadThreshold   float64
	lowLoadThreshold    float64
	adjustmentFactor    float64
	logger              interface {
		Warn(...interface{})
		Infof(string, ...interface{})
		Info(...interface{})
	}
}

// NewAdaptiveCollector 创建新的自适应收集器
func NewAdaptiveCollector(reportInterval int, logger interface {
	Warn(...interface{})
	Infof(string, ...interface{})
	Info(...interface{})
}) *AdaptiveCollector {
	baseInterval := time.Duration(reportInterval) * time.Second
	return &AdaptiveCollector{
		currentInterval:   baseInterval,
		baseInterval:      baseInterval,
		maxInterval:       (baseInterval * 5) / 2, // 最大间隔为基础间隔的2.5倍（5秒）
		minInterval:       time.Second * 1,        // 最小间隔1秒
		highLoadThreshold: 80.0,                   // CPU或内存使用率超过80%认为是高负载
		lowLoadThreshold:  30.0,                   // CPU或内存使用率低于30%认为是低负载
		adjustmentFactor:  1.2,                    // 调整因子
		logger:            logger,
	}
}

// GetCurrentInterval 获取当前收集间隔
func (ac *AdaptiveCollector) GetCurrentInterval() time.Duration {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	return ac.currentInterval
}

// UpdateInterval 根据系统负载更新收集间隔
func (ac *AdaptiveCollector) UpdateInterval() {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	// 获取CPU使用率
	cpuPercent, err := cpu.Percent(time.Second, false)
	if err != nil {
		ac.logger.Warn("Failed to get CPU usage:", err)
		return
	}

	// 获取内存使用率
	memInfo, err := mem.VirtualMemory()
	if err != nil {
		ac.logger.Warn("Failed to get memory usage:", err)
		return
	}

	currentCPU := cpuPercent[0]
	currentMem := memInfo.UsedPercent

	// 计算系统负载（CPU和内存使用率的最大值）
	systemLoad := math.Max(currentCPU, currentMem)

	// 根据负载调整间隔
	if systemLoad > ac.highLoadThreshold {
		// 高负载：增加收集间隔，减少系统压力
		ac.consecutiveHighLoad++
		ac.consecutiveLowLoad = 0

		if ac.consecutiveHighLoad >= 3 { // 连续3次高负载才调整
			newInterval := time.Duration(float64(ac.currentInterval) * ac.adjustmentFactor)
			if newInterval <= ac.maxInterval {
				ac.currentInterval = newInterval
				ac.logger.Infof("High load detected (%.2f%%), increasing interval to %v", systemLoad, ac.currentInterval)
			}
		}
	} else if systemLoad < ac.lowLoadThreshold {
		// 低负载：减少收集间隔，提高数据精度
		ac.consecutiveLowLoad++
		ac.consecutiveHighLoad = 0

		if ac.consecutiveLowLoad >= 5 { // 连续5次低负载才调整
			newInterval := time.Duration(float64(ac.currentInterval) / ac.adjustmentFactor)
			if newInterval >= ac.minInterval {
				ac.currentInterval = newInterval
				ac.logger.Infof("Low load detected (%.2f%%), decreasing interval to %v", systemLoad, ac.currentInterval)
			}
		}
	} else {
		// 中等负载：重置计数器，逐渐回归基础间隔
		ac.consecutiveHighLoad = 0
		ac.consecutiveLowLoad = 0

		// 如果当前间隔偏离基础间隔太多，逐渐调整回去
		if ac.currentInterval > ac.baseInterval {
			newInterval := time.Duration(float64(ac.currentInterval) * 0.95)
			if newInterval >= ac.baseInterval {
				ac.currentInterval = newInterval
			} else {
				ac.currentInterval = ac.baseInterval
			}
		} else if ac.currentInterval < ac.baseInterval {
			newInterval := time.Duration(float64(ac.currentInterval) * 1.05)
			if newInterval <= ac.baseInterval {
				ac.currentInterval = newInterval
			} else {
				ac.currentInterval = ac.baseInterval
			}
		}
	}

	// 更新历史数据
	ac.lastCPUUsage = currentCPU
	ac.lastMemUsage = currentMem
}

// GetLoadInfo 获取当前负载信息（用于调试）
func (ac *AdaptiveCollector) GetLoadInfo() (float64, float64, time.Duration) {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	return ac.lastCPUUsage, ac.lastMemUsage, ac.currentInterval
}

// ResetToBase 重置到基础间隔
func (ac *AdaptiveCollector) ResetToBase() {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	ac.currentInterval = ac.baseInterval
	ac.consecutiveHighLoad = 0
	ac.consecutiveLowLoad = 0
	ac.logger.Info("Reset collection interval to base:", ac.baseInterval)
}
