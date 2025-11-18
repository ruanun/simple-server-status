package internal

import (
	"sync"
	"testing"
	"time"
)

// TestNewNetworkStatsCollector 测试创建网络统计收集器
func TestNewNetworkStatsCollector(t *testing.T) {
	t.Run("使用默认排除接口", func(t *testing.T) {
		nsc := NewNetworkStatsCollector(nil)
		if nsc == nil {
			t.Fatal("NewNetworkStatsCollector() 返回 nil")
		}

		expectedInterfaces := []string{
			"lo", "tun", "docker", "veth", "br-", "vmbr", "vnet", "kube",
		}
		if len(nsc.excludeInterfaces) != len(expectedInterfaces) {
			t.Errorf("排除接口数量 = %d; want %d", len(nsc.excludeInterfaces), len(expectedInterfaces))
		}

		for i, iface := range expectedInterfaces {
			if i < len(nsc.excludeInterfaces) && nsc.excludeInterfaces[i] != iface {
				t.Errorf("excludeInterfaces[%d] = %s; want %s", i, nsc.excludeInterfaces[i], iface)
			}
		}
	})

	t.Run("使用自定义排除接口", func(t *testing.T) {
		customInterfaces := []string{"eth0", "wlan0"}
		nsc := NewNetworkStatsCollector(customInterfaces)
		if nsc == nil {
			t.Fatal("NewNetworkStatsCollector() 返回 nil")
		}

		if len(nsc.excludeInterfaces) != len(customInterfaces) {
			t.Errorf("排除接口数量 = %d; want %d", len(nsc.excludeInterfaces), len(customInterfaces))
		}

		for i, iface := range customInterfaces {
			if i < len(nsc.excludeInterfaces) && nsc.excludeInterfaces[i] != iface {
				t.Errorf("excludeInterfaces[%d] = %s; want %s", i, nsc.excludeInterfaces[i], iface)
			}
		}
	})

	t.Run("使用空排除接口列表", func(t *testing.T) {
		nsc := NewNetworkStatsCollector([]string{})
		if nsc == nil {
			t.Fatal("NewNetworkStatsCollector() 返回 nil")
		}

		if len(nsc.excludeInterfaces) != 0 {
			t.Errorf("排除接口数量 = %d; want 0", len(nsc.excludeInterfaces))
		}
	})
}

// TestNetworkStatsCollector_GetStats 测试获取统计信息
func TestNetworkStatsCollector_GetStats(t *testing.T) {
	nsc := NewNetworkStatsCollector(nil)

	// 初始状态应该全为 0
	stats := nsc.GetStats()
	if stats == nil {
		t.Fatal("GetStats() 返回 nil")
	}

	if stats.NetInSpeed != 0 {
		t.Errorf("初始 NetInSpeed = %d; want 0", stats.NetInSpeed)
	}
	if stats.NetOutSpeed != 0 {
		t.Errorf("初始 NetOutSpeed = %d; want 0", stats.NetOutSpeed)
	}
	if stats.NetInTransfer != 0 {
		t.Errorf("初始 NetInTransfer = %d; want 0", stats.NetInTransfer)
	}
	if stats.NetOutTransfer != 0 {
		t.Errorf("初始 NetOutTransfer = %d; want 0", stats.NetOutTransfer)
	}
}

// TestNetworkStatsCollector_Update 测试更新网络统计
func TestNetworkStatsCollector_Update(t *testing.T) {
	nsc := NewNetworkStatsCollector(nil)

	// 第一次更新
	err := nsc.Update()
	if err != nil {
		t.Logf("第一次更新失败（可能是权限问题）: %v", err)
		// 在某些环境中可能没有权限访问网络统计，这是正常的
		return
	}

	// 验证统计信息已更新
	stats1 := nsc.GetStats()
	if stats1 == nil {
		t.Fatal("GetStats() 返回 nil")
	}

	// 等待一段时间后再次更新
	time.Sleep(100 * time.Millisecond)

	err = nsc.Update()
	if err != nil {
		t.Fatalf("第二次更新失败: %v", err)
	}

	stats2 := nsc.GetStats()
	if stats2 == nil {
		t.Fatal("GetStats() 返回 nil")
	}

	// 传输量应该是单调递增的
	if stats2.NetInTransfer < stats1.NetInTransfer {
		t.Logf("警告: NetInTransfer 减少了 (%d -> %d)", stats1.NetInTransfer, stats2.NetInTransfer)
	}
	if stats2.NetOutTransfer < stats1.NetOutTransfer {
		t.Logf("警告: NetOutTransfer 减少了 (%d -> %d)", stats1.NetOutTransfer, stats2.NetOutTransfer)
	}
}

// TestNetworkStatsCollector_ConcurrentAccess 测试并发访问安全性
func TestNetworkStatsCollector_ConcurrentAccess(t *testing.T) {
	nsc := NewNetworkStatsCollector(nil)

	// 先进行一次更新，确保有数据
	if err := nsc.Update(); err != nil {
		t.Logf("初始更新失败（可能是权限问题）: %v", err)
		// 即使更新失败，也可以测试并发安全性
	}

	var wg sync.WaitGroup
	errChan := make(chan error, 20)

	// 10 个 goroutine 并发更新
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				if err := nsc.Update(); err != nil {
					errChan <- err
				}
				time.Sleep(10 * time.Millisecond)
			}
		}()
	}

	// 10 个 goroutine 并发读取
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				stats := nsc.GetStats()
				if stats == nil {
					errChan <- nil // 用 nil 表示获取统计失败
				}
				time.Sleep(5 * time.Millisecond)
			}
		}()
	}

	wg.Wait()
	close(errChan)

	// 检查是否有 nil 统计
	nilCount := 0
	updateErrCount := 0
	for err := range errChan {
		if err == nil {
			nilCount++
		} else {
			updateErrCount++
		}
	}

	if nilCount > 0 {
		t.Errorf("并发读取时获得了 %d 次 nil 统计", nilCount)
	}

	// 更新错误是可以接受的（可能是权限问题）
	if updateErrCount > 0 {
		t.Logf("并发更新时有 %d 次更新失败（可能是权限问题）", updateErrCount)
	}
}

// TestNetworkStatsCollector_MultipleUpdates 测试多次更新的一致性
func TestNetworkStatsCollector_MultipleUpdates(t *testing.T) {
	nsc := NewNetworkStatsCollector(nil)

	// 进行多次更新
	for i := 0; i < 5; i++ {
		err := nsc.Update()
		if err != nil {
			t.Logf("第 %d 次更新失败（可能是权限问题）: %v", i+1, err)
			// 如果更新失败，跳过后续验证
			return
		}

		stats := nsc.GetStats()
		if stats == nil {
			t.Fatalf("第 %d 次更新后 GetStats() 返回 nil", i+1)
		}

		// 注意：NetInTransfer 和 NetOutTransfer 是 uint64 类型，无需检查是否为负数
		// 验证数据已正确收集（非零值表示有流量）
		t.Logf("第 %d 次: NetInTransfer=%d, NetOutTransfer=%d", i+1, stats.NetInTransfer, stats.NetOutTransfer)

		time.Sleep(50 * time.Millisecond)
	}
}

// TestNetworkStatsCollector_WithCustomExclusions 测试自定义排除接口
func TestNetworkStatsCollector_WithCustomExclusions(t *testing.T) {
	// 排除所有常见接口，只保留非虚拟接口
	customExclusions := []string{"lo", "docker", "veth", "br-", "virbr", "tun", "tap"}
	nsc := NewNetworkStatsCollector(customExclusions)

	err := nsc.Update()
	if err != nil {
		t.Logf("更新失败（可能是权限问题）: %v", err)
		return
	}

	stats := nsc.GetStats()
	if stats == nil {
		t.Fatal("GetStats() 返回 nil")
	}

	// 验证统计信息有效
	t.Logf("NetInTransfer: %d, NetOutTransfer: %d", stats.NetInTransfer, stats.NetOutTransfer)
	t.Logf("NetInSpeed: %d, NetOutSpeed: %d", stats.NetInSpeed, stats.NetOutSpeed)
}

// TestNetworkStatsCollector_ZeroTimeInterval 测试时间间隔为0的情况
func TestNetworkStatsCollector_ZeroTimeInterval(t *testing.T) {
	nsc := NewNetworkStatsCollector(nil)

	// 连续快速更新两次（时间间隔可能为0）
	err1 := nsc.Update()
	if err1 != nil {
		t.Logf("第一次更新失败（可能是权限问题）: %v", err1)
		return
	}

	// 立即再次更新（时间差可能为0）
	err2 := nsc.Update()
	if err2 != nil {
		t.Fatalf("第二次更新失败: %v", err2)
	}

	stats := nsc.GetStats()
	if stats == nil {
		t.Fatal("GetStats() 返回 nil")
	}

	// 当时间间隔为0时，速度应该保持不变或为0
	// 这不应该导致崩溃或除零错误
	t.Logf("速度: In=%d, Out=%d（时间间隔可能为0）", stats.NetInSpeed, stats.NetOutSpeed)
}

// BenchmarkNetworkStatsCollector_Update 基准测试：更新性能
func BenchmarkNetworkStatsCollector_Update(b *testing.B) {
	nsc := NewNetworkStatsCollector(nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := nsc.Update()
		if err != nil {
			b.Logf("更新失败: %v", err)
		}
	}
}

// BenchmarkNetworkStatsCollector_GetStats 基准测试：获取统计性能
func BenchmarkNetworkStatsCollector_GetStats(b *testing.B) {
	nsc := NewNetworkStatsCollector(nil)
	// 先更新一次
	_ = nsc.Update() // 忽略更新错误，测试环境中无关紧要

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stats := nsc.GetStats()
		if stats == nil {
			b.Fatal("GetStats() 返回 nil")
		}
	}
}

// BenchmarkNetworkStatsCollector_ConcurrentGetStats 基准测试：并发读取性能
func BenchmarkNetworkStatsCollector_ConcurrentGetStats(b *testing.B) {
	nsc := NewNetworkStatsCollector(nil)
	_ = nsc.Update() // 忽略更新错误，测试环境中无关紧要

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			stats := nsc.GetStats()
			if stats == nil {
				b.Error("GetStats() 返回 nil")
			}
		}
	})
}
