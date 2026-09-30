package captcha

import (
	"crypto/subtle"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/ruanun/simple-server-status/internal/dashboard/randx"
)

const (
	ttl      = 5 * time.Minute
	maxItems = 10000 // 超出时淘汰最早生成的验证码，防止被刷接口占满内存

	rateWindow = time.Minute
	rateMax    = 20 // 同一来源每分钟最多获取的验证码数量，防止刷接口挤掉他人手中的验证码
)

// ErrRateLimited 同一来源获取验证码过于频繁
var ErrRateLimited = errors.New("captcha rate limited")

type item struct {
	code    string
	expires time.Time
}

// Store 保存待校验的图形验证码（内存中，一次性使用，5 分钟过期）
type Store struct {
	mu    sync.Mutex
	now   func() time.Time
	gen   func() string
	items map[string]item
	order []string               // 按生成顺序排列的 id；有效期固定，因此也是过期顺序
	gets  map[string][]time.Time // 来源 -> 最近一分钟内获取验证码的时间
}

// NewStore 创建 Store；gen 为验证码生成函数，传 nil 使用随机验证码（测试可注入固定值）
func NewStore(now func() time.Time, gen func() string) *Store {
	if gen == nil {
		gen = randomCode
	}
	return &Store{now: now, gen: gen, items: map[string]item{}, gets: map[string][]time.Time{}}
}

// New 为来源 key（客户端 IP 的限流 key）生成一个验证码，返回 id 与 PNG 图片的 data URL；
// 同一来源获取过于频繁时返回 ErrRateLimited
func (s *Store) New(key string) (id, image string, err error) {
	if !s.allow(key) {
		return "", "", ErrRateLimited
	}
	code := s.gen()
	if image, err = render(code); err != nil {
		return "", "", err
	}
	id = randx.String(24)
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	for len(s.order) > 0 {
		head := s.order[0]
		it, ok := s.items[head]
		if ok && now.Before(it.expires) && len(s.items) < maxItems {
			break
		}
		delete(s.items, head)
		s.order = s.order[1:]
	}
	s.items[id] = item{code: code, expires: now.Add(ttl)}
	s.order = append(s.order, id)
	return id, image, nil
}

// Verify 校验验证码（不区分大小写）；无论对错都会作废该验证码，防止重复猜测
func (s *Store) Verify(id, code string) bool {
	s.mu.Lock()
	it, ok := s.items[id]
	delete(s.items, id)
	s.mu.Unlock()
	if !ok || !s.now().Before(it.expires) {
		return false
	}
	got := strings.ToUpper(strings.TrimSpace(code))
	return subtle.ConstantTimeCompare([]byte(got), []byte(it.code)) == 1
}

// allow 记录来源 key 的一次获取，返回是否未超过频率上限
func (s *Store) allow(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	recent := s.gets[key][:0:0]
	for _, t := range s.gets[key] {
		if now.Sub(t) < rateWindow {
			recent = append(recent, t)
		}
	}
	if len(recent) >= rateMax {
		s.gets[key] = recent
		return false
	}
	s.gets[key] = append(recent, now)
	if len(s.gets) > maxItems {
		for k, ts := range s.gets {
			if now.Sub(ts[len(ts)-1]) >= rateWindow {
				delete(s.gets, k)
			}
		}
	}
	return true
}
