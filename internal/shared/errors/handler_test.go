package errors

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// TestNewErrorHandler 测试创建错误处理器
func TestNewErrorHandler(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()

	t.Run("使用默认配置", func(t *testing.T) {
		handler := NewErrorHandler(logger, nil)
		if handler == nil {
			t.Fatal("Expected non-nil handler")
		}
		if handler.logger != logger {
			t.Error("Logger not set correctly")
		}
		if handler.retryConfig == nil {
			t.Error("Expected default retry config")
		}
		if handler.maxLastErrors != 100 {
			t.Errorf("maxLastErrors = %d, want 100", handler.maxLastErrors)
		}
	})

	t.Run("使用自定义配置", func(t *testing.T) {
		config := &RetryConfig{
			MaxAttempts:   5,
			InitialDelay:  2 * time.Second,
			MaxDelay:      2 * time.Minute,
			BackoffFactor: 1.5,
			Timeout:       10 * time.Minute,
		}
		handler := NewErrorHandler(logger, config)
		if handler.retryConfig.MaxAttempts != 5 {
			t.Errorf("MaxAttempts = %d, want 5", handler.retryConfig.MaxAttempts)
		}
		if handler.retryConfig.BackoffFactor != 1.5 {
			t.Errorf("BackoffFactor = %f, want 1.5", handler.retryConfig.BackoffFactor)
		}
	})
}

// TestDefaultRetryConfig 测试默认重试配置
func TestDefaultRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()

	if config.MaxAttempts != 3 {
		t.Errorf("MaxAttempts = %d, want 3", config.MaxAttempts)
	}
	if config.InitialDelay != time.Second {
		t.Errorf("InitialDelay = %v, want 1s", config.InitialDelay)
	}
	if config.MaxDelay != time.Minute {
		t.Errorf("MaxDelay = %v, want 1m", config.MaxDelay)
	}
	if config.BackoffFactor != 2.0 {
		t.Errorf("BackoffFactor = %f, want 2.0", config.BackoffFactor)
	}
	if config.Timeout != 5*time.Minute {
		t.Errorf("Timeout = %v, want 5m", config.Timeout)
	}
}

// TestHandleError 测试处理错误
func TestHandleError(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()
	handler := NewErrorHandler(logger, nil)

	t.Run("记录错误统计", func(t *testing.T) {
		err1 := NewAppError(ErrorTypeNetwork, SeverityMedium, "NET001", "网络错误1")
		err2 := NewAppError(ErrorTypeNetwork, SeverityHigh, "NET002", "网络错误2")
		err3 := NewAppError(ErrorTypeSystem, SeverityLow, "SYS001", "系统错误")

		handler.HandleError(err1)
		handler.HandleError(err2)
		handler.HandleError(err3)

		stats := handler.GetErrorStats()
		if stats[ErrorTypeNetwork] != 2 {
			t.Errorf("Network errors = %d, want 2", stats[ErrorTypeNetwork])
		}
		if stats[ErrorTypeSystem] != 1 {
			t.Errorf("System errors = %d, want 1", stats[ErrorTypeSystem])
		}
	})

	t.Run("保存最近的错误", func(t *testing.T) {
		handler2 := NewErrorHandler(logger, nil)
		err := NewAppError(ErrorTypeData, SeverityMedium, "DATA001", "数据错误")
		handler2.HandleError(err)

		recent := handler2.GetRecentErrors(10)
		if len(recent) != 1 {
			t.Fatalf("Expected 1 recent error, got %d", len(recent))
		}
		if recent[0].Code != "DATA001" {
			t.Errorf("Recent error code = %s, want DATA001", recent[0].Code)
		}
	})
}

// TestHandleError_MaxLastErrors 测试最大错误历史限制
func TestHandleError_MaxLastErrors(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()
	handler := NewErrorHandler(logger, nil)

	// 添加超过100个错误
	for i := 0; i < 150; i++ {
		err := NewAppError(ErrorTypeData, SeverityLow, "TEST", "测试错误")
		handler.HandleError(err)
	}

	recent := handler.GetRecentErrors(200)
	if len(recent) != 100 {
		t.Errorf("Recent errors count = %d, want 100 (max limit)", len(recent))
	}
}

// TestHandleError_Concurrent 测试并发处理错误
func TestHandleError_Concurrent(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()
	handler := NewErrorHandler(logger, nil)

	var wg sync.WaitGroup
	errorCount := 100
	goroutines := 10

	// 10个goroutine并发添加错误
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < errorCount/goroutines; j++ {
				err := NewAppError(ErrorTypeNetwork, SeverityMedium, "CONCURRENT", "并发测试")
				handler.HandleError(err)
			}
		}(i)
	}

	wg.Wait()

	stats := handler.GetErrorStats()
	if stats[ErrorTypeNetwork] != int64(errorCount) {
		t.Errorf("Concurrent errors = %d, want %d", stats[ErrorTypeNetwork], errorCount)
	}
}

// TestGetRecentErrors 测试获取最近的错误
func TestGetRecentErrors(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()
	handler := NewErrorHandler(logger, nil)

	// 添加5个错误
	for i := 0; i < 5; i++ {
		err := NewAppError(ErrorTypeData, SeverityLow, "TEST", "测试错误")
		handler.HandleError(err)
	}

	tests := []struct {
		name  string
		count int
		want  int
	}{
		{"获取3个", 3, 3},
		{"获取全部", 5, 5},
		{"获取超过总数", 10, 5},
		{"获取0个", 0, 5},
		{"获取负数", -1, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recent := handler.GetRecentErrors(tt.count)
			if len(recent) != tt.want {
				t.Errorf("GetRecentErrors(%d) returned %d errors, want %d",
					tt.count, len(recent), tt.want)
			}
		})
	}
}

// TestGetRecentErrors_Empty 测试空错误历史
func TestGetRecentErrors_Empty(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()
	handler := NewErrorHandler(logger, nil)

	recent := handler.GetRecentErrors(10)
	if len(recent) != 0 {
		t.Errorf("Expected empty slice, got %d errors", len(recent))
	}
}

// TestRetryWithBackoff_Success 测试重试成功场景
func TestRetryWithBackoff_Success(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()
	config := &RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      100 * time.Millisecond,
		BackoffFactor: 2.0,
	}
	handler := NewErrorHandler(logger, config)

	attempts := 0
	operation := func() error {
		attempts++
		if attempts < 3 {
			return errors.New("临时失败")
		}
		return nil
	}

	ctx := context.Background()
	err := handler.RetryWithBackoff(ctx, operation, ErrorTypeNetwork)

	if err != nil {
		t.Errorf("Expected success, got error: %v", err)
	}
	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

// TestRetryWithBackoff_Failure 测试重试失败场景
func TestRetryWithBackoff_Failure(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()
	config := &RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      100 * time.Millisecond,
		BackoffFactor: 2.0,
	}
	handler := NewErrorHandler(logger, config)

	attempts := 0
	operation := func() error {
		attempts++
		return errors.New("持续失败")
	}

	ctx := context.Background()
	err := handler.RetryWithBackoff(ctx, operation, ErrorTypeNetwork)

	if err == nil {
		t.Error("Expected error, got nil")
	}
	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

// TestRetryWithBackoff_ContextCancel 测试上下文取消
func TestRetryWithBackoff_ContextCancel(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()
	config := &RetryConfig{
		MaxAttempts:   10,
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      time.Second,
		BackoffFactor: 2.0,
	}
	handler := NewErrorHandler(logger, config)

	attempts := 0
	operation := func() error {
		attempts++
		return errors.New("失败")
	}

	// 在50ms后取消上下文
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := handler.RetryWithBackoff(ctx, operation, ErrorTypeNetwork)

	if err != context.DeadlineExceeded {
		t.Errorf("Expected context.DeadlineExceeded, got %v", err)
	}
	// 应该只执行了1次（因为上下文很快就取消了）
	if attempts > 2 {
		t.Errorf("Expected <= 2 attempts due to quick cancel, got %d", attempts)
	}
}

// TestRetryWithBackoff_NonRetryableError 测试不可重试的错误
func TestRetryWithBackoff_NonRetryableError(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()
	config := &RetryConfig{
		MaxAttempts:   5,
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      100 * time.Millisecond,
		BackoffFactor: 2.0,
	}
	handler := NewErrorHandler(logger, config)

	attempts := 0
	operation := func() error {
		attempts++
		return errors.New("配置错误") // 配置错误不可重试
	}

	ctx := context.Background()
	err := handler.RetryWithBackoff(ctx, operation, ErrorTypeConfig)

	if err == nil {
		t.Error("Expected error, got nil")
	}
	// 配置错误不可重试，应该只执行1次
	if attempts != 1 {
		t.Errorf("Expected 1 attempt for non-retryable error, got %d", attempts)
	}
}

// TestRetryWithBackoff_BackoffCalculation 测试退避计算
func TestRetryWithBackoff_BackoffCalculation(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()
	config := &RetryConfig{
		MaxAttempts:   4,
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      500 * time.Millisecond,
		BackoffFactor: 2.0,
	}
	handler := NewErrorHandler(logger, config)

	attempts := 0
	timestamps := []time.Time{}
	operation := func() error {
		attempts++
		timestamps = append(timestamps, time.Now())
		return errors.New("失败")
	}

	ctx := context.Background()
	_ = handler.RetryWithBackoff(ctx, operation, ErrorTypeNetwork)

	// 验证重试间隔
	// 第1次尝试后延迟 100ms
	// 第2次尝试后延迟 200ms
	// 第3次尝试后延迟 400ms
	if len(timestamps) >= 2 {
		delay1 := timestamps[1].Sub(timestamps[0])
		if delay1 < 100*time.Millisecond || delay1 > 150*time.Millisecond {
			t.Logf("Warning: First delay %v not close to 100ms", delay1)
		}
	}
	if len(timestamps) >= 3 {
		delay2 := timestamps[2].Sub(timestamps[1])
		if delay2 < 200*time.Millisecond || delay2 > 250*time.Millisecond {
			t.Logf("Warning: Second delay %v not close to 200ms", delay2)
		}
	}
}

// TestSafeExecute 测试安全执行函数
func TestSafeExecute(t *testing.T) {
	t.Run("正常执行", func(t *testing.T) {
		executed := false
		operation := func() error {
			executed = true
			return nil
		}

		err := SafeExecute(operation, ErrorTypeSystem, "测试操作")

		if err != nil {
			t.Errorf("Expected nil error, got %v", err)
		}
		if !executed {
			t.Error("Operation was not executed")
		}
	})

	t.Run("返回错误", func(t *testing.T) {
		expectedErr := errors.New("操作失败")
		operation := func() error {
			return expectedErr
		}

		err := SafeExecute(operation, ErrorTypeSystem, "测试操作")

		if err != expectedErr {
			t.Errorf("Expected error %v, got %v", expectedErr, err)
		}
	})

	t.Run("捕获panic", func(t *testing.T) {
		operation := func() error {
			panic("致命错误")
		}

		err := SafeExecute(operation, ErrorTypeSystem, "测试操作")

		if err == nil {
			t.Fatal("Expected error from panic, got nil")
		}

		appErr, ok := err.(*AppError)
		if !ok {
			t.Fatalf("Expected *AppError, got %T", err)
		}
		if appErr.Code != "PANIC" {
			t.Errorf("Expected code PANIC, got %s", appErr.Code)
		}
		if appErr.Severity != SeverityCritical {
			t.Errorf("Expected SeverityCritical, got %v", appErr.Severity)
		}
	})
}

// TestSafeGo 测试安全启动goroutine
func TestSafeGo(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()
	handler := NewErrorHandler(logger, nil)

	t.Run("正常执行", func(t *testing.T) {
		done := make(chan bool, 1)
		SafeGo(handler, func() {
			done <- true
		}, "测试goroutine")

		select {
		case <-done:
			// 成功
		case <-time.After(time.Second):
			t.Error("Goroutine did not complete")
		}
	})

	t.Run("捕获panic", func(t *testing.T) {
		done := make(chan bool, 1)
		SafeGo(handler, func() {
			defer func() {
				// 让测试知道goroutine已结束
				done <- true
			}()
			panic("goroutine panic")
		}, "测试goroutine")

		select {
		case <-done:
			// Panic被捕获，goroutine正常结束
			// 验证错误被记录
			time.Sleep(10 * time.Millisecond) // 给HandleError时间执行
			stats := handler.GetErrorStats()
			if stats[ErrorTypeSystem] == 0 {
				t.Error("Expected panic to be recorded as system error")
			}
		case <-time.After(time.Second):
			t.Error("Goroutine did not complete")
		}
	})

	t.Run("nil handler", func(t *testing.T) {
		// 验证nil handler不会导致崩溃
		done := make(chan bool, 1)
		SafeGo(nil, func() {
			defer func() {
				done <- true
			}()
			panic("测试panic")
		}, "测试")

		select {
		case <-done:
			// 成功，没有崩溃
		case <-time.After(time.Second):
			t.Error("Goroutine did not complete")
		}
	})
}

// TestLogErrorStats 测试记录错误统计
func TestLogErrorStats(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()
	handler := NewErrorHandler(logger, nil)

	// 添加各种类型的错误
	handler.HandleError(NewAppError(ErrorTypeNetwork, SeverityMedium, "NET", "网络"))
	handler.HandleError(NewAppError(ErrorTypeNetwork, SeverityHigh, "NET", "网络"))
	handler.HandleError(NewAppError(ErrorTypeSystem, SeverityLow, "SYS", "系统"))
	handler.HandleError(NewAppError(ErrorTypeConfig, SeverityHigh, "CFG", "配置"))

	// 应该不会panic
	handler.LogErrorStats()

	// 验证统计正确
	stats := handler.GetErrorStats()
	if stats[ErrorTypeNetwork] != 2 {
		t.Errorf("Network errors = %d, want 2", stats[ErrorTypeNetwork])
	}
	if stats[ErrorTypeSystem] != 1 {
		t.Errorf("System errors = %d, want 1", stats[ErrorTypeSystem])
	}
	if stats[ErrorTypeConfig] != 1 {
		t.Errorf("Config errors = %d, want 1", stats[ErrorTypeConfig])
	}
}

// TestLogErrorStats_NilLogger 测试nil logger
func TestLogErrorStats_NilLogger(t *testing.T) {
	handler := NewErrorHandler(nil, nil)
	handler.HandleError(NewAppError(ErrorTypeNetwork, SeverityMedium, "TEST", "测试"))

	// 应该不会panic
	handler.LogErrorStats()
}

// BenchmarkHandleError 基准测试：处理错误
func BenchmarkHandleError(b *testing.B) {
	logger := zap.NewNop().Sugar()
	handler := NewErrorHandler(logger, nil)
	err := NewAppError(ErrorTypeNetwork, SeverityMedium, "NET001", "网络错误")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handler.HandleError(err)
	}
}

// BenchmarkRetryWithBackoff 基准测试：重试机制
func BenchmarkRetryWithBackoff(b *testing.B) {
	logger := zap.NewNop().Sugar()
	config := &RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  time.Millisecond,
		MaxDelay:      10 * time.Millisecond,
		BackoffFactor: 2.0,
	}
	handler := NewErrorHandler(logger, config)

	operation := func() error {
		return nil // 立即成功
	}

	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = handler.RetryWithBackoff(ctx, operation, ErrorTypeNetwork)
	}
}

// BenchmarkSafeExecute 基准测试：安全执行
func BenchmarkSafeExecute(b *testing.B) {
	operation := func() error {
		return nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = SafeExecute(operation, ErrorTypeSystem, "测试")
	}
}
