package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/ruanun/simple-server-status/internal/dashboard/auth"
	"github.com/ruanun/simple-server-status/internal/dashboard/captcha"
	"github.com/ruanun/simple-server-status/internal/dashboard/store"
)

// normalizeCaptcha 去除首尾空白并校验验证码设置
func normalizeCaptcha(c *store.CaptchaSettings) error {
	c.TurnstileSiteKey = strings.TrimSpace(c.TurnstileSiteKey)
	c.TurnstileSecret = strings.TrimSpace(c.TurnstileSecret)
	switch c.Mode {
	case "":
		c.Mode = store.CaptchaNone
	case store.CaptchaNone, store.CaptchaImage:
	case store.CaptchaTurnstile:
		if c.TurnstileSiteKey == "" || c.TurnstileSecret == "" {
			return errors.New("启用 Turnstile 需填写 Site Key 与 Secret Key")
		}
	default:
		return errors.New("验证码模式只能是 none、image 或 turnstile")
	}
	return nil
}

// captchaChallenge 登录页获取验证码：图形验证码返回新生成的图片，Turnstile 返回 Site Key
func (a *API) captchaChallenge(c *gin.Context) {
	cs := a.currentSettings().Captcha
	switch cs.Mode {
	case store.CaptchaImage:
		id, img, err := a.captchas.New(auth.ClientKey(c.ClientIP()))
		if errors.Is(err, captcha.ErrRateLimited) {
			fail(c, http.StatusTooManyRequests, "captcha_rate_limited", "获取验证码过于频繁，请稍后再试")
			return
		}
		if err != nil {
			a.internal(c, "生成验证码失败", err)
			return
		}
		respond(c, gin.H{"mode": cs.Mode, "id": id, "image": img})
	case store.CaptchaTurnstile:
		respond(c, gin.H{"mode": cs.Mode, "site_key": cs.TurnstileSiteKey})
	default:
		respond(c, gin.H{"mode": store.CaptchaNone})
	}
}

// checkCaptcha 按当前设置校验登录请求中的验证码；未启用时直接通过
func (a *API) checkCaptcha(c *gin.Context, req loginReq) bool {
	cs := a.currentSettings().Captcha
	switch cs.Mode {
	case store.CaptchaImage:
		return a.captchas.Verify(req.CaptchaID, req.CaptchaCode)
	case store.CaptchaTurnstile:
		return a.verifyTurnstile(c.Request.Context(), cs.TurnstileSecret, req.TurnstileToken, c.ClientIP())
	}
	return true
}

// verifyTurnstile 调用 Cloudflare 核验 token；token 无效属于正常失败，其余异常记录日志
func (a *API) verifyTurnstile(ctx context.Context, secret, token, ip string) bool {
	ok, err := a.turnstile.Verify(ctx, secret, token, ip)
	var rej *captcha.RejectedError
	if err != nil && !errors.As(err, &rej) {
		a.Log.Warn("Turnstile 核验失败", "err", err)
	}
	return ok
}

// sameTurnstile 两份设置是否都启用了 Turnstile 且密钥相同
func sameTurnstile(a, b store.CaptchaSettings) bool {
	return a.Mode == store.CaptchaTurnstile && b.Mode == store.CaptchaTurnstile &&
		a.TurnstileSiteKey == b.TurnstileSiteKey && a.TurnstileSecret == b.TurnstileSecret
}

// checkTurnstileOnSave 新启用 Turnstile 或修改其密钥时，要求提交一个用新密钥核验通过的 token，
// 确认 Site Key、Secret Key 与当前域名都配置正确，避免保存后无法登录
func (a *API) checkTurnstileOnSave(c *gin.Context, next store.CaptchaSettings, token string) bool {
	if next.Mode != store.CaptchaTurnstile {
		return true
	}
	if sameTurnstile(a.currentSettings().Captcha, next) {
		return true
	}
	ok, err := a.turnstile.Verify(c.Request.Context(), next.TurnstileSecret, token, c.ClientIP())
	if ok {
		return true
	}
	msg := "请先在下方完成 Turnstile 验证，以确认密钥可用"
	var rej *captcha.RejectedError
	if errors.As(err, &rej) {
		msg = "Turnstile 验证未通过（" + strings.Join(rej.Codes, ", ") + "），请检查 Site Key、Secret Key 与域名配置"
	} else if err != nil {
		msg = "无法连接 Turnstile 核验服务：" + err.Error()
	}
	fail(c, http.StatusBadRequest, "turnstile_check_failed", msg)
	return false
}
