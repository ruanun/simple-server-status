// Package auth 提供管理员登录所需的 JWT、密码哈希与登录限流。
package auth

import (
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// MinPasswordLen 密码最小长度
const MinPasswordLen = 8

const tokenTTL = 7 * 24 * time.Hour

// maxTrackedKeys 触发清理的 key 总数阈值（fails 与 until 合计），防止内存无限增长
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

// Limiter 登录限流：同一 key 在 5 分钟内有 5 次未成功的尝试后锁定 5 分钟
type Limiter struct {
	mu     sync.Mutex
	now    func() time.Time
	max    int
	window time.Duration
	lock   time.Duration
	fails  map[string][]time.Time
	until  map[string]time.Time
}

// NewLimiter 创建 Limiter
func NewLimiter(now func() time.Time) *Limiter {
	return &Limiter{now: now, max: 5, window: 5 * time.Minute, lock: 5 * time.Minute,
		fails: map[string][]time.Time{}, until: map[string]time.Time{}}
}

// Acquire 预占一次登录尝试：在同一把锁内检查锁定状态，并先把本次尝试计为一次失败
// （登录成功后由 Reset 清除），从而防止并发请求绕过限流；返回是否允许本次尝试
func (l *Limiter) Acquire(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if t, ok := l.until[key]; ok {
		if now.Before(t) {
			return false
		}
		delete(l.until, key)
	}
	var recent []time.Time
	for _, t := range l.fails[key] {
		if now.Sub(t) < l.window {
			recent = append(recent, t)
		}
	}
	recent = append(recent, now)
	if len(recent) >= l.max {
		l.until[key] = now.Add(l.lock)
		delete(l.fails, key)
	} else {
		l.fails[key] = recent
	}
	if len(l.fails)+len(l.until) > maxTrackedKeys {
		l.sweep(now)
	}
	return true
}

// Reset 登录成功后清除记录
func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.fails, key)
	delete(l.until, key)
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
}
