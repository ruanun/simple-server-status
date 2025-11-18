package internal

import (
	"bytes"
	"encoding/json"
	"sync"
)

// BufferPool 字节缓冲池
type BufferPool struct {
	pool sync.Pool
}

// NewBufferPool 创建新的缓冲池
func NewBufferPool() *BufferPool {
	return &BufferPool{
		pool: sync.Pool{
			New: func() interface{} {
				return &bytes.Buffer{}
			},
		},
	}
}

// Get 从池中获取缓冲区
func (bp *BufferPool) Get() *bytes.Buffer {
	buf := bp.pool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

// Put 将缓冲区放回池中
func (bp *BufferPool) Put(buf *bytes.Buffer) {
	// 如果缓冲区太大，不放回池中，避免内存泄漏
	if buf.Cap() > 64*1024 { // 64KB
		return
	}
	bp.pool.Put(buf)
}

// MemoryPoolManager 内存池管理器
// 注意: json.Encoder 不适合池化,因为它绑定了特定的 io.Writer
// 我们只池化 bytes.Buffer,每次创建新的 Encoder
type MemoryPoolManager struct {
	bufferPool *BufferPool
	stats      PoolStats
	mu         sync.RWMutex
}

// NewMemoryPoolManager 创建新的内存池管理器
func NewMemoryPoolManager() *MemoryPoolManager {
	return &MemoryPoolManager{
		bufferPool: NewBufferPool(),
	}
}

// PoolStats 池统计信息
type PoolStats struct {
	BufferGets  int64
	BufferPuts  int64
	MemorySaved int64 // 估算节省的内存分配次数
}

// GetBuffer 获取缓冲区
func (mpm *MemoryPoolManager) GetBuffer() *bytes.Buffer {
	mpm.mu.Lock()
	mpm.stats.BufferGets++
	mpm.mu.Unlock()
	return mpm.bufferPool.Get()
}

// PutBuffer 归还缓冲区
func (mpm *MemoryPoolManager) PutBuffer(buf *bytes.Buffer) {
	mpm.mu.Lock()
	mpm.stats.BufferPuts++
	mpm.stats.MemorySaved++
	mpm.mu.Unlock()
	mpm.bufferPool.Put(buf)
}

// GetStats 获取池统计信息
func (mpm *MemoryPoolManager) GetStats() PoolStats {
	mpm.mu.RLock()
	defer mpm.mu.RUnlock()
	return mpm.stats
}

// LogStats 记录池统计信息
func (mpm *MemoryPoolManager) LogStats(logger interface{ Infof(string, ...interface{}) }) {
	if logger == nil {
		return
	}
	stats := mpm.GetStats()
	logger.Infof("Memory Pool Stats - Buffer Gets: %d, Puts: %d, Memory Saved: %d",
		stats.BufferGets, stats.BufferPuts, stats.MemorySaved)
}

// OptimizedJSONMarshal 使用内存池的优化JSON序列化
// 只池化 bytes.Buffer,每次创建新的 json.Encoder
func (mpm *MemoryPoolManager) OptimizedJSONMarshal(v interface{}) ([]byte, error) {
	// 从池中获取 buffer
	buf := mpm.GetBuffer()
	defer mpm.PutBuffer(buf)

	// 每次创建新的 encoder 使用池化的 buffer
	encoder := json.NewEncoder(buf)
	err := encoder.Encode(v)
	if err != nil {
		return nil, err
	}

	// 移除最后的换行符(Encode 会添加)
	data := buf.Bytes()
	if len(data) > 0 && data[len(data)-1] == '\n' {
		data = data[:len(data)-1]
	}

	// 复制数据,因为 buf 会被重用
	result := make([]byte, len(data))
	copy(result, data)
	return result, nil
}
