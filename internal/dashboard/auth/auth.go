// Package auth 提供管理员登录所需的 JWT、密码哈希与登录限流。
package auth

import (
	"errors"
	"net/netip"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// MinPasswordLen 密码最小长度
const MinPasswordLen = 8

const tokenTTL = 7 * 24 * time.Hour

// maxTrackedKeys 触发清理的 key 总数阈值（fails、until 与 trusted 合计），防止内存无限增长
const maxTrackedKeys = 10000

// Claims JWT 载荷：用户 ID 与 token 版本（改密码后版本 +1，旧 token 失效）
type Claims struct {
	UID int64 `json:"uid"`
	TV  int   `json:"tv"`
	jwt.RegisteredClaims
}

// Manager 签发与校验 JWT
type Manager struct {
	secret []byte
	now    func() time.Time
}

// NewManager 创建 Manager
func NewManager(secret []byte, now func() time.Time) *Manager {
	return &Manager{secret: secret, now: now}
}

// Issue 签发 token
func (m *Manager) Issue(uid int64, tv int) (string, error) {
	now := m.now()
	c := Claims{UID: uid, TV: tv, RegisteredClaims: jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(tokenTTL)),
	}}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(m.secret)
}

// Parse 校验并解析 token
func (m *Manager) Parse(token string) (Claims, error) {
	var c Claims
	_, err := jwt.ParseWithClaims(token, &c, func(*jwt.Token) (any, error) { return m.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithTimeFunc(m.now),
		jwt.WithExpirationRequired())
	return c, err
}

// HashPassword 计算 bcrypt 哈希
func HashPassword(p string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(p), bcrypt.DefaultCost)
	return string(b), err
}

// CheckPassword 校验密码
func CheckPassword(hash, p string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(p)) == nil
}

// DummyHash 返回一个固定的 bcrypt 哈希：用户不存在时也用它做一次校验，
// 让响应耗时与用户存在时一致，避免据此判断用户名是否存在
var DummyHash = sync.OnceValue(func() string {
	h, _ := HashPassword("sss-dummy-password")
	return h
})

// ClientKey 把客户端 IP 转为限流 key：IPv4 取完整地址；IPv6 取所在 /64 网段，
// 因为同一用户通常能随意使用整个 /64 内的地址。无法解析时原样返回
func ClientKey(ip string) string {
	a, err := netip.ParseAddr(ip)
	if err != nil {
		return ip
	}
	a = a.Unmap()
	if a.Is4() {
		return a.String()
	}
	p, _ := a.Prefix(64)
	return p.String()
}

// 登录被拒绝的原因
var (
	// ErrTooManyAttempts 该来源失败次数过多，已被锁定
	ErrTooManyAttempts = errors.New("too many attempts")
	// ErrLoginBusy 全站登录尝试过多（疑似分布式爆破），暂时拒绝陌生来源
	ErrLoginBusy = errors.New("login busy")
)

// Limiter 登录限流，分两层：
//   - 单个 key：5 分钟内有 5 次未成功的尝试后锁定 5 分钟；
//   - 全局：陌生 key（近期未成功登录过）的尝试 5 分钟内合计超过 30 次后，暂停陌生 key 的尝试，
//     防止攻击者换 IP 绕过单 key 限流。成功登录过的 key 记为可信，不受全局限制，
//     这样攻击期间管理员仍能从常用地址登录
type Limiter struct {
	mu           sync.Mutex
	now          func() time.Time
	max          int
	window       time.Duration
	lock         time.Duration
	globalMax    int
	globalWindow time.Duration
	trustTTL     time.Duration
	fails        map[string][]time.Time
	until        map[string]time.Time
	trusted      map[string]time.Time // key -> 可信到期时间
	global       []time.Time          // 陌生 key 的尝试时间，最多保留 globalMax 个
}

// NewLimiter 创建 Limiter
func NewLimiter(now func() time.Time) *Limiter {
	return &Limiter{now: now, max: 5, window: 5 * time.Minute, lock: 5 * time.Minute,
		globalMax: 30, globalWindow: 5 * time.Minute, trustTTL: 30 * 24 * time.Hour,
		fails: map[string][]time.Time{}, until: map[string]time.Time{}, trusted: map[string]time.Time{}}
}

// Acquire 预占一次登录尝试：在同一把锁内检查锁定状态，并先把本次尝试计为一次失败
// （登录成功后由 Reset 清除），从而防止并发请求绕过限流。
// 允许时返回 nil，否则返回 ErrTooManyAttempts 或 ErrLoginBusy（被拒绝的请求不计数）
func (l *Limiter) Acquire(key string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if t, ok := l.until[key]; ok {
		if now.Before(t) {
			return ErrTooManyAttempts
		}
		delete(l.until, key)
	}
	trusted := false
	if t, ok := l.trusted[key]; ok {
		if now.Before(t) {
			trusted = true
		} else {
			delete(l.trusted, key)
		}
	}
	if !trusted {
		l.global = pruneBefore(l.global, now.Add(-l.globalWindow))
		if len(l.global) >= l.globalMax {
			return ErrLoginBusy
		}
		l.global = append(l.global, now)
	}
	recent := append(pruneBefore(l.fails[key], now.Add(-l.window)), now)
	if len(recent) >= l.max {
		l.until[key] = now.Add(l.lock)
		delete(l.fails, key)
	} else {
		l.fails[key] = recent
	}
	if len(l.fails)+len(l.until)+len(l.trusted) > maxTrackedKeys {
		l.sweep(now)
	}
	return nil
}

// Reset 登录成功后清除该 key 的失败记录，并把它记为可信
func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.fails, key)
	delete(l.until, key)
	l.trusted[key] = l.now().Add(l.trustTTL)
}

// pruneBefore 去掉按时间升序排列的 ts 中不晚于 cutoff 的部分
func pruneBefore(ts []time.Time, cutoff time.Time) []time.Time {
	i := 0
	for i < len(ts) && !ts[i].After(cutoff) {
		i++
	}
	return ts[i:]
}

// sweep 清理过期记录，防止内存无限增长（调用方需持有锁）
func (l *Limiter) sweep(now time.Time) {
	for k, ts := range l.fails {
		if len(ts) == 0 || now.Sub(ts[len(ts)-1]) >= l.window {
			delete(l.fails, k)
		}
	}
	for k, t := range l.until {
		if !now.Before(t) {
			delete(l.until, k)
		}
	}
	for k, t := range l.trusted {
		if !now.Before(t) {
			delete(l.trusted, k)
		}
	}
}
