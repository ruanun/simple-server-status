package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ruanun/simple-server-status/internal/dashboard/auth"
)

type loginReq struct {
	Username       string `json:"username"`
	Password       string `json:"password"`
	CaptchaID      string `json:"captcha_id"`
	CaptchaCode    string `json:"captcha_code"`
	TurnstileToken string `json:"turnstile_token"`
}

func (a *API) login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "bad_request", "请求格式错误")
		return
	}
	key := auth.ClientKey(c.ClientIP())
	// 预占一次尝试，失败无需再计数
	if err := a.Limiter.Acquire(key); errors.Is(err, auth.ErrLoginBusy) {
		fail(c, http.StatusTooManyRequests, "login_busy", "登录尝试过于频繁，请稍后再试")
		return
	} else if err != nil {
		fail(c, http.StatusTooManyRequests, "too_many_attempts", "登录失败次数过多，请 5 分钟后再试")
		return
	}
	// 验证码在限流之后校验：填错验证码同样计为一次失败，避免被用来无限制地触发 Turnstile 核验请求
	if !a.checkCaptcha(c, req) {
		fail(c, http.StatusBadRequest, "captcha_invalid", "验证码错误或已过期")
		return
	}
	u, err := a.Store.GetUserByName(c.Request.Context(), req.Username)
	hash := auth.DummyHash() // 用户不存在时也做一次校验，保持耗时一致
	if err == nil {
		hash = u.PasswordHash
	}
	if !auth.CheckPassword(hash, req.Password) || err != nil {
		fail(c, http.StatusUnauthorized, "invalid_credentials", "用户名或密码错误")
		return
	}
	a.Limiter.Reset(key)
	tok, err := a.Auth.Issue(u.ID, u.TokenVersion)
	if err != nil {
		a.internal(c, "签发登录凭证失败", err)
		return
	}
	respond(c, gin.H{"token": tok, "username": u.Username})
}

func (a *API) me(c *gin.Context) {
	// Dashboard 版本只对登录用户可见，公开接口不返回
	respond(c, gin.H{"username": currentUser(c).Username, "version": a.Version})
}

type passwordReq struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

func (a *API) changePassword(c *gin.Context) {
	u := currentUser(c)
	var req passwordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "bad_request", "请求格式错误")
		return
	}
	if !auth.CheckPassword(u.PasswordHash, req.OldPassword) {
		fail(c, http.StatusBadRequest, "wrong_password", "原密码错误")
		return
	}
	if len(req.NewPassword) < auth.MinPasswordLen {
		fail(c, http.StatusBadRequest, "weak_password", "新密码至少 8 位")
		return
	}
	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		a.internal(c, "计算密码哈希失败", err)
		return
	}
	if err := a.Store.SetPassword(c.Request.Context(), u.ID, hash); err != nil {
		a.internal(c, "保存密码失败", err)
		return
	}
	// 旧 token 已失效，立即断开该用户已登录的浏览器推送连接（前端用新 token 重连）
	a.bc.kickUser(u.ID)
	tok, err := a.Auth.Issue(u.ID, u.TokenVersion+1)
	if err != nil {
		a.internal(c, "签发登录凭证失败", err)
		return
	}
	respond(c, gin.H{"token": tok})
}
