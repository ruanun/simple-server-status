package internal

import (
	"math/rand"
	"time"
)

// RandomIntInRange 生成指定范围内的随机整数 [min, max]
// 注意：此函数用于非安全场景（如负载均衡URL选择），使用 math/rand 足够
//
//nolint:gosec // G404: 此处使用弱随机数生成器是可接受的，仅用于负载均衡URL选择，非安全敏感场景
func RandomIntInRange(min, max int) int {
	// 使用当前时间创建随机数生成器的种子源
	source := rand.NewSource(time.Now().UnixNano())
	rng := rand.New(source) // 创建新的随机数生成器
	// 生成随机整数，范围是 [min, max]
	return rng.Intn(max-min+1) + min
}
