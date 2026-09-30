package captcha

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type clock struct{ t time.Time }

func (c *clock) Now() time.Time { return c.t }

func fixedCode(code string) func() string { return func() string { return code } }

func TestRandomCode(t *testing.T) {
	c := randomCode()
	if len(c) != CodeLen {
		t.Fatalf("长度应为 %d：%q", CodeLen, c)
	}
	for _, r := range c {
		if !strings.ContainsRune(codeAlphabet, r) {
			t.Fatalf("包含字符集以外的字符：%q", c)
		}
	}
}

func TestNewReturnsPNG(t *testing.T) {
	s := NewStore(time.Now, nil)
	id, img, err := s.New("ip")
	if err != nil || id == "" {
		t.Fatalf("New 失败：%v", err)
	}
	b64, ok := strings.CutPrefix(img, "data:image/png;base64,")
	if !ok {
		t.Fatalf("应返回 PNG data URL：%.40s", img)
	}
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(raw))
	if err != nil || cfg.Width != imgW || cfg.Height != imgH {
		t.Fatalf("图片无效：%v %+v", err, cfg)
	}
}

func TestVerifyOnceAndCaseInsensitive(t *testing.T) {
	s := NewStore(time.Now, fixedCode("AB3C"))
	id, _, _ := s.New("ip")
	if !s.Verify(id, " ab3c ") {
		t.Fatal("忽略大小写与首尾空白后应校验通过")
	}
	if s.Verify(id, "AB3C") {
		t.Fatal("验证码只能使用一次")
	}
	id, _, _ = s.New("ip")
	if s.Verify(id, "XXXX") || s.Verify(id, "AB3C") {
		t.Fatal("答错一次后验证码应作废")
	}
	if s.Verify("unknown", "AB3C") {
		t.Fatal("不存在的 id 不应通过")
	}
}

func TestVerifyExpires(t *testing.T) {
	c := &clock{t: time.Unix(0, 0)}
	s := NewStore(c.Now, fixedCode("AB3C"))
	id, _, _ := s.New("ip")
	c.t = c.t.Add(ttl)
	if s.Verify(id, "AB3C") {
		t.Fatal("过期的验证码不应通过")
	}
}

func TestStoreEvictsOldest(t *testing.T) {
	c := &clock{t: time.Unix(0, 0)}
	s := NewStore(c.Now, fixedCode("AB3C"))
	first, _, _ := s.New("ip")
	s.mu.Lock()
	for i := range maxItems {
		id := string(rune('a'+i%26)) + strings.Repeat("x", i/26+1)
		s.items[id] = item{code: "AB3C", expires: c.t.Add(ttl)}
		s.order = append(s.order, id)
	}
	s.mu.Unlock()
	last, _, _ := s.New("ip")
	if len(s.items) > maxItems {
		t.Fatalf("数量应不超过上限，实际 %d", len(s.items))
	}
	if s.Verify(first, "AB3C") {
		t.Fatal("超出上限时应淘汰最早的验证码")
	}
	if !s.Verify(last, "AB3C") {
		t.Fatal("新生成的验证码应可用")
	}
}

func TestTurnstileVerify(t *testing.T) {
	var got struct{ secret, response, ip string }
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		got.secret, got.response, got.ip = r.PostForm.Get("secret"), r.PostForm.Get("response"), r.PostForm.Get("remoteip")
		switch r.PostForm.Get("response") {
		case "good":
			_, _ = w.Write([]byte(`{"success":true}`))
		case "boom":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			_, _ = w.Write([]byte(`{"success":false,"error-codes":["invalid-input-response"]}`))
		}
	}))
	defer srv.Close()
	ts := Turnstile{URL: srv.URL}
	ctx := context.Background()

	if ok, err := ts.Verify(ctx, "sec", "good", "1.2.3.4"); !ok || err != nil {
		t.Fatalf("有效 token 应通过：%v %v", ok, err)
	}
	if got.secret != "sec" || got.response != "good" || got.ip != "1.2.3.4" {
		t.Fatalf("提交的参数不对：%+v", got)
	}
	ok, err := ts.Verify(ctx, "sec", "bad", "")
	var rej *RejectedError
	if ok || !errors.As(err, &rej) || rej.Codes[0] != "invalid-input-response" {
		t.Fatalf("无效 token 应被拒绝并带错误码：%v %v", ok, err)
	}
	if ok, err := ts.Verify(ctx, "sec", "boom", ""); ok || err == nil || errors.As(err, &rej) {
		t.Fatalf("接口异常应返回普通错误：%v %v", ok, err)
	}
	if ok, err := ts.Verify(ctx, "sec", "", ""); ok || err != nil {
		t.Fatalf("空 token 直接判定不通过：%v %v", ok, err)
	}
}

func TestNewRateLimitedPerKey(t *testing.T) {
	c := &clock{t: time.Unix(0, 0)}
	s := NewStore(c.Now, fixedCode("AB3C"))
	for i := range rateMax {
		if _, _, err := s.New("ip"); err != nil {
			t.Fatalf("第 %d 次应允许：%v", i+1, err)
		}
	}
	if _, _, err := s.New("ip"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("超过频率上限应拒绝，实际 %v", err)
	}
	if _, _, err := s.New("other"); err != nil {
		t.Fatalf("其他来源不受影响：%v", err)
	}
	c.t = c.t.Add(rateWindow)
	if _, _, err := s.New("ip"); err != nil {
		t.Fatalf("窗口过后应恢复：%v", err)
	}
}
