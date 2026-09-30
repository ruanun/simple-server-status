package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ruanun/simple-server-status/internal/dashboard/store"
)

type challenge struct {
	Mode    string `json:"mode"`
	ID      string `json:"id"`
	Image   string `json:"image"`
	SiteKey string `json:"site_key"`
}

// fakeTurnstile 模拟 Cloudflare 核验接口：secret 为 good-secret 且 token 为 good 时通过
func fakeTurnstile(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		switch {
		case r.PostForm.Get("secret") != "good-secret":
			_, _ = w.Write([]byte(`{"success":false,"error-codes":["invalid-input-secret"]}`))
		case r.PostForm.Get("response") == "good":
			_, _ = w.Write([]byte(`{"success":true}`))
		default:
			_, _ = w.Write([]byte(`{"success":false,"error-codes":["invalid-input-response"]}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func withTurnstile(t *testing.T) func(*Deps) {
	u := fakeTurnstile(t).URL
	return func(d *Deps) { d.TurnstileURL = u }
}

func saveCaptcha(t *testing.T, e *testEnv, tok string, cs gin.H, turnstileToken string) (int, []byte) {
	t.Helper()
	body := gin.H{"site_title": "t", "default_report_interval": 2, "captcha": cs}
	if turnstileToken != "" {
		body["turnstile_token"] = turnstileToken
	}
	return e.do("PUT", "/api/admin/settings", tok, body)
}

func TestCaptchaNoneByDefault(t *testing.T) {
	e := newTestEnv(t)
	e.adminToken()
	_, body := e.do("GET", "/api/auth/captcha", "", nil)
	if c := decodeData[challenge](t, body); c.Mode != "none" {
		t.Fatalf("默认不启用验证码：%s", body)
	}
	if code, _ := login(t, e, "password123"); code != http.StatusOK {
		t.Fatalf("未启用验证码时应能直接登录，实际 %d", code)
	}
}

func TestImageCaptchaLogin(t *testing.T) {
	e := newTestEnv(t, func(d *Deps) { d.CaptchaCode = func() string { return "AB3C" } })
	tok := e.adminToken()
	if code, body := saveCaptcha(t, e, tok, gin.H{"mode": "image"}, ""); code != http.StatusOK {
		t.Fatalf("启用图形验证码失败：%d %s", code, body)
	}

	get := func() challenge {
		_, body := e.do("GET", "/api/auth/captcha", "", nil)
		c := decodeData[challenge](t, body)
		if c.Mode != "image" || c.ID == "" || !strings.HasPrefix(c.Image, "data:image/png;base64,") {
			t.Fatalf("应返回图形验证码：%s", body)
		}
		return c
	}
	try := func(id, code, pw string) (int, []byte) {
		return e.do("POST", "/api/auth/login", "", gin.H{"username": "admin", "password": pw, "captcha_id": id, "captcha_code": code})
	}

	if code, body := try("", "", "password123"); code != http.StatusBadRequest || errorCode(t, body) != "captcha_invalid" {
		t.Fatalf("缺少验证码应被拒绝：%d %s", code, body)
	}
	c := get()
	if code, _ := try(c.ID, "XXXX", "password123"); code != http.StatusBadRequest {
		t.Fatalf("验证码错误应被拒绝，实际 %d", code)
	}
	if code, _ := try(c.ID, "AB3C", "password123"); code != http.StatusBadRequest {
		t.Fatalf("答错后验证码应作废，实际 %d", code)
	}
	c = get()
	if code, body := try(c.ID, "ab3c", "password123"); code != http.StatusOK {
		t.Fatalf("正确的验证码与密码应能登录：%d %s", code, body)
	}
}

func TestCaptchaFailuresCountTowardsLimit(t *testing.T) {
	e := newTestEnv(t, withTurnstile(t))
	tok := e.adminToken()
	cs := gin.H{"mode": "turnstile", "turnstile_site_key": "site", "turnstile_secret": "good-secret"}
	if code, body := saveCaptcha(t, e, tok, cs, "good"); code != http.StatusOK {
		t.Fatalf("启用 Turnstile 失败：%d %s", code, body)
	}
	for i := range 5 {
		if code, _ := e.do("POST", "/api/auth/login", "", gin.H{"username": "admin", "password": "password123", "turnstile_token": "bad"}); code != http.StatusBadRequest {
			t.Fatalf("第 %d 次无效 token 应返回 400，实际 %d", i+1, code)
		}
	}
	if code, body := e.do("POST", "/api/auth/login", "", gin.H{"username": "admin", "password": "password123", "turnstile_token": "good"}); code != http.StatusTooManyRequests {
		t.Fatalf("验证码连续失败也应触发限流，避免无限制地请求 Cloudflare：%d %s", code, body)
	}
}

func TestCaptchaChallengeRateLimited(t *testing.T) {
	e := newTestEnv(t)
	tok := e.adminToken()
	if code, body := saveCaptcha(t, e, tok, gin.H{"mode": "image"}, ""); code != http.StatusOK {
		t.Fatalf("启用图形验证码失败：%d %s", code, body)
	}
	for i := range 20 {
		if code, _ := e.do("GET", "/api/auth/captcha", "", nil); code != http.StatusOK {
			t.Fatalf("第 %d 次获取应允许，实际 %d", i+1, code)
		}
	}
	if code, body := e.do("GET", "/api/auth/captcha", "", nil); code != http.StatusTooManyRequests || errorCode(t, body) != "captcha_rate_limited" {
		t.Fatalf("获取过于频繁应被拒绝：%d %s", code, body)
	}
}

func TestImportKeepsCaptchaWhenTurnstileUnverified(t *testing.T) {
	e := newTestEnv(t)
	tok := e.adminToken()
	if code, body := saveCaptcha(t, e, tok, gin.H{"mode": "image"}, ""); code != http.StatusOK {
		t.Fatalf("启用图形验证码失败：%d %s", code, body)
	}
	file := gin.H{"version": 1, "settings": gin.H{
		"site_title": "导入", "default_report_interval": 2,
		"captcha": gin.H{"mode": "turnstile", "turnstile_site_key": "old-site", "turnstile_secret": "old-secret"},
	}, "servers": []gin.H{}}
	code, body := e.do("POST", "/api/admin/import", tok, file)
	r := decodeData[struct {
		CaptchaKept bool `json:"captcha_kept"`
	}](t, body)
	if code != http.StatusOK || !r.CaptchaKept {
		t.Fatalf("导入未核验的 Turnstile 配置时应保留当前验证码设置并提示：%d %s", code, body)
	}
	st := e.api.currentSettings()
	if st.SiteTitle != "导入" || st.Captcha.Mode != store.CaptchaImage {
		t.Fatalf("其他设置应导入、验证码设置保持不变：%+v", st)
	}

	file["settings"] = gin.H{"site_title": "再次导入", "default_report_interval": 2, "captcha": gin.H{"mode": "none"}}
	_, body = e.do("POST", "/api/admin/import", tok, file)
	if r := decodeData[struct {
		CaptchaKept bool `json:"captcha_kept"`
	}](t, body); r.CaptchaKept || e.api.currentSettings().Captcha.Mode != store.CaptchaNone {
		t.Fatalf("非 Turnstile 的验证码设置照常导入：%s", body)
	}
}

func TestTurnstileLogin(t *testing.T) {
	e := newTestEnv(t, withTurnstile(t))
	tok := e.adminToken()
	cs := gin.H{"mode": "turnstile", "turnstile_site_key": "site", "turnstile_secret": "good-secret"}
	if code, body := saveCaptcha(t, e, tok, cs, "good"); code != http.StatusOK {
		t.Fatalf("启用 Turnstile 失败：%d %s", code, body)
	}
	_, body := e.do("GET", "/api/auth/captcha", "", nil)
	if c := decodeData[challenge](t, body); c.Mode != "turnstile" || c.SiteKey != "site" || strings.Contains(string(body), "good-secret") {
		t.Fatalf("应只返回 Site Key，不能泄露 Secret：%s", body)
	}
	if code, body := e.do("POST", "/api/auth/login", "", gin.H{"username": "admin", "password": "password123", "turnstile_token": "bad"}); code != http.StatusBadRequest || errorCode(t, body) != "captcha_invalid" {
		t.Fatalf("无效 token 应被拒绝：%d %s", code, body)
	}
	if code, body := e.do("POST", "/api/auth/login", "", gin.H{"username": "admin", "password": "password123", "turnstile_token": "good"}); code != http.StatusOK {
		t.Fatalf("有效 token 应能登录：%d %s", code, body)
	}
}

func TestSaveTurnstileRequiresVerifiedToken(t *testing.T) {
	e := newTestEnv(t, withTurnstile(t))
	tok := e.adminToken()

	if code, body := saveCaptcha(t, e, tok, gin.H{"mode": "turnstile"}, "good"); code != http.StatusBadRequest || errorCode(t, body) != "invalid_input" {
		t.Fatalf("缺少密钥应被拒绝：%d %s", code, body)
	}
	if code, body := saveCaptcha(t, e, tok, gin.H{"mode": "bogus"}, ""); code != http.StatusBadRequest || errorCode(t, body) != "invalid_input" {
		t.Fatalf("未知模式应被拒绝：%d %s", code, body)
	}
	cs := gin.H{"mode": "turnstile", "turnstile_site_key": "site", "turnstile_secret": "good-secret"}
	if code, body := saveCaptcha(t, e, tok, cs, ""); code != http.StatusBadRequest || errorCode(t, body) != "turnstile_check_failed" {
		t.Fatalf("未完成验证不应允许启用：%d %s", code, body)
	}
	bad := gin.H{"mode": "turnstile", "turnstile_site_key": "site", "turnstile_secret": "wrong-secret"}
	if code, body := saveCaptcha(t, e, tok, bad, "good"); code != http.StatusBadRequest || !strings.Contains(string(body), "invalid-input-secret") {
		t.Fatalf("Secret 错误时应带上 Cloudflare 的错误码：%d %s", code, body)
	}
	if e.api.currentSettings().Captcha.Mode != store.CaptchaNone {
		t.Fatal("核验失败时不应保存")
	}
	if code, body := saveCaptcha(t, e, tok, cs, "good"); code != http.StatusOK {
		t.Fatalf("核验通过后应保存：%d %s", code, body)
	}
	// 密钥未变时（如只修改站点标题）无需再次验证
	if code, body := saveCaptcha(t, e, tok, cs, ""); code != http.StatusOK {
		t.Fatalf("密钥未变时不应要求重新验证：%d %s", code, body)
	}
	if code, _ := saveCaptcha(t, e, tok, gin.H{"mode": "none", "turnstile_site_key": "site", "turnstile_secret": "good-secret"}, ""); code != http.StatusOK {
		t.Fatal("关闭验证码不需要验证")
	}
}
