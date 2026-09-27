package auth

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type clock struct{ t time.Time }

func (c *clock) Now() time.Time { return c.t }

func TestIssueAndParse(t *testing.T) {
	c := &clock{t: time.Unix(1_700_000_000, 0)}
	m := NewManager([]byte("k"), c.Now)
	tok, err := m.Issue(7, 3)
	if err != nil {
		t.Fatal(err)
	}
	cl, err := m.Parse(tok)
	if err != nil || cl.UID != 7 || cl.TV != 3 {
		t.Fatalf("Parse = %+v, %v", cl, err)
	}
	c.t = c.t.Add(8 * 24 * time.Hour)
	if _, err := m.Parse(tok); err == nil {
		t.Fatal("过期 token 应被拒绝")
	}
}

func TestParseRejectsForeignToken(t *testing.T) {
	c := &clock{t: time.Unix(1_700_000_000, 0)}
	tok, _ := NewManager([]byte("a"), c.Now).Issue(1, 0)
	if _, err := NewManager([]byte("b"), c.Now).Parse(tok); err == nil {
		t.Fatal("其他密钥签发的 token 应被拒绝")
	}
	if _, err := NewManager([]byte("a"), c.Now).Parse("garbage"); err == nil {
		t.Fatal("非法 token 应被拒绝")
	}
}

func TestPassword(t *testing.T) {
	h, err := HashPassword("password123")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPassword(h, "password123") || CheckPassword(h, "wrong") {
		t.Fatal("密码校验错误")
	}
}

func TestLimiterLocksAfterFiveAttempts(t *testing.T) {
	c := &clock{t: time.Unix(0, 0)}
	l := NewLimiter(c.Now)
	for i := 0; i < 5; i++ {
		if !l.Acquire("ip") {
			t.Fatalf("第 %d 次尝试应允许", i+1)
		}
	}
	if l.Acquire("ip") {
		t.Fatal("5 次未成功的尝试后应锁定")
	}
	if !l.Acquire("other") {
		t.Fatal("不同 key 不应受影响")
	}
	c.t = c.t.Add(5 * time.Minute)
	if !l.Acquire("ip") {
		t.Fatal("锁定 5 分钟后应解除")
	}
}

func TestLimiterWindowAndReset(t *testing.T) {
	c := &clock{t: time.Unix(0, 0)}
	l := NewLimiter(c.Now)
	for i := 0; i < 4; i++ {
		l.Acquire("ip")
	}
	c.t = c.t.Add(6 * time.Minute)
	for i := 0; i < 4; i++ {
		l.Acquire("ip")
	}
	if !l.Acquire("ip") {
		t.Fatal("窗口外的尝试不应累计")
	}
	l.Reset("ip") // 登录成功
	for i := 0; i < 4; i++ {
		l.Acquire("ip")
	}
	if !l.Acquire("ip") {
		t.Fatal("Reset 后应重新计数")
	}
}

func TestLimiterAcquireConcurrent(t *testing.T) {
	l := NewLimiter(func() time.Time { return time.Unix(0, 0) })
	var wg sync.WaitGroup
	var allowed atomic.Int32
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if l.Acquire("ip") {
				allowed.Add(1)
			}
		}()
	}
	wg.Wait()
	if n := allowed.Load(); n != 5 {
		t.Fatalf("并发尝试中应恰好允许 5 次，实际 %d", n)
	}
}

func TestAcquireSweepsExpiredLockoutsWhenUntilGrowsLarge(t *testing.T) {
	c := &clock{t: time.Unix(0, 0)}
	l := NewLimiter(c.Now)
	past := c.t.Add(-time.Hour)
	for i := 0; i < maxTrackedKeys; i++ {
		l.until[fmt.Sprintf("locked-%d", i)] = past
	}
	l.Acquire("x")
	if len(l.until) >= maxTrackedKeys {
		t.Fatalf("大量已过期锁定记录应被清理，len(until)=%d", len(l.until))
	}
}
