package auth

import (
	"errors"
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
		if l.Acquire("ip") != nil {
			t.Fatalf("第 %d 次尝试应允许", i+1)
		}
	}
	if err := l.Acquire("ip"); !errors.Is(err, ErrTooManyAttempts) {
		t.Fatalf("5 次未成功的尝试后应锁定，实际 %v", err)
	}
	if l.Acquire("other") != nil {
		t.Fatal("不同 key 不应受影响")
	}
	c.t = c.t.Add(5 * time.Minute)
	if l.Acquire("ip") != nil {
		t.Fatal("锁定 5 分钟后应解除")
	}
}

func TestLimiterWindowAndReset(t *testing.T) {
	c := &clock{t: time.Unix(0, 0)}
	l := NewLimiter(c.Now)
	for i := 0; i < 4; i++ {
		_ = l.Acquire("ip")
	}
	c.t = c.t.Add(6 * time.Minute)
	for i := 0; i < 4; i++ {
		_ = l.Acquire("ip")
	}
	if l.Acquire("ip") != nil {
		t.Fatal("窗口外的尝试不应累计")
	}
	l.Reset("ip") // 登录成功
	for i := 0; i < 4; i++ {
		_ = l.Acquire("ip")
	}
	if l.Acquire("ip") != nil {
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
			if l.Acquire("ip") == nil {
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
	_ = l.Acquire("x")
	if len(l.until) >= maxTrackedKeys {
		t.Fatalf("大量已过期锁定记录应被清理，len(until)=%d", len(l.until))
	}
}

func TestLimiterGlobalLimitAndTrustedKey(t *testing.T) {
	c := &clock{t: time.Unix(0, 0)}
	l := NewLimiter(c.Now)
	_ = l.Acquire("admin")
	l.Reset("admin") // 成功登录过，记为可信；这次尝试本身也计入全局
	for i := 0; i < 29; i++ {
		if err := l.Acquire(fmt.Sprintf("attacker-%d", i)); err != nil {
			t.Fatalf("第 %d 个陌生来源应允许：%v", i+1, err)
		}
	}
	if err := l.Acquire("attacker-new"); !errors.Is(err, ErrLoginBusy) {
		t.Fatalf("全站尝试过多后应拒绝陌生来源，实际 %v", err)
	}
	if err := l.Acquire("admin"); err != nil {
		t.Fatalf("可信来源不应受全局限制：%v", err)
	}
	if len(l.fails["attacker-new"]) != 0 {
		t.Fatal("被全局拒绝的请求不应计入该来源的失败次数")
	}
	c.t = c.t.Add(5 * time.Minute)
	if err := l.Acquire("attacker-new"); err != nil {
		t.Fatalf("全局窗口过后应恢复：%v", err)
	}
}

func TestLimiterTrustExpires(t *testing.T) {
	c := &clock{t: time.Unix(0, 0)}
	l := NewLimiter(c.Now)
	l.Reset("admin")
	c.t = c.t.Add(31 * 24 * time.Hour)
	for i := 0; i < 30; i++ {
		_ = l.Acquire(fmt.Sprintf("attacker-%d", i))
	}
	if err := l.Acquire("admin"); !errors.Is(err, ErrLoginBusy) {
		t.Fatalf("可信期过后应按陌生来源处理，实际 %v", err)
	}
}

func TestClientKey(t *testing.T) {
	cases := map[string]string{
		"1.2.3.4":              "1.2.3.4",
		"::ffff:1.2.3.4":       "1.2.3.4",
		"2001:db8:1:2:3:4:5:6": "2001:db8:1:2::/64",
		"2001:db8:1:2:ffff::1": "2001:db8:1:2::/64",
		"not-an-ip":            "not-an-ip",
	}
	for in, want := range cases {
		if got := ClientKey(in); got != want {
			t.Errorf("ClientKey(%q) = %q，期望 %q", in, got, want)
		}
	}
}

func TestDummyHashRejects(t *testing.T) {
	if CheckPassword(DummyHash(), "") || CheckPassword(DummyHash(), "password123") {
		t.Fatal("DummyHash 不应匹配常见密码")
	}
}
