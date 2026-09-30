package captcha

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// TurnstileVerifyURL Cloudflare Turnstile 服务端核验地址
const TurnstileVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

// Turnstile 调用 Cloudflare 核验前端拿到的 token
type Turnstile struct {
	URL    string       // 为空使用 TurnstileVerifyURL（测试可替换）
	Client *http.Client // 为空使用 10 秒超时的默认客户端
}

// Verify 核验 token；token 无效时返回 false 与 Cloudflare 给出的错误码，网络等异常返回 error
func (t Turnstile) Verify(ctx context.Context, secret, token, remoteIP string) (bool, error) {
	if token == "" {
		return false, nil
	}
	u := t.URL
	if u == "" {
		u = TurnstileVerifyURL
	}
	c := t.Client
	if c == nil {
		c = &http.Client{Timeout: 10 * time.Second}
	}
	form := url.Values{"secret": {secret}, "response": {token}}
	if remoteIP != "" {
		form.Set("remoteip", remoteIP)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(form.Encode()))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.Do(req)
	if err != nil {
		return false, fmt.Errorf("请求 Turnstile 核验接口失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("turnstile 核验接口返回 %d", resp.StatusCode)
	}
	var r struct {
		Success    bool     `json:"success"`
		ErrorCodes []string `json:"error-codes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return false, fmt.Errorf("解析 Turnstile 核验结果失败: %w", err)
	}
	if !r.Success && len(r.ErrorCodes) > 0 {
		return false, &RejectedError{Codes: r.ErrorCodes}
	}
	return r.Success, nil
}

// RejectedError Cloudflare 拒绝了 token，Codes 为其返回的错误码（如 invalid-input-secret）
type RejectedError struct{ Codes []string }

func (e *RejectedError) Error() string {
	return "turnstile 核验未通过: " + strings.Join(e.Codes, ", ")
}
