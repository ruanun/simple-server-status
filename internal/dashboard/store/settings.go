package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"strconv"
)

// DefaultInstallScriptBase 安装脚本默认下载地址前缀
const DefaultInstallScriptBase = "https://github.com/ruanun/simple-server-status/releases/latest/download"

// ReleaseDownloadBase 指定版本 Release 资产的下载地址前缀，后接版本标签（如 v2.0.0）
const ReleaseDownloadBase = "https://github.com/ruanun/simple-server-status/releases/download/"

// NotifySettings 通知设置：渠道（为空表示不启用）与四条规则的开关、阈值
type NotifySettings struct {
	WebhookURL     string `json:"webhook_url"`
	TelegramToken  string `json:"telegram_token"`
	TelegramChatID string `json:"telegram_chat_id"`
	Lang           string `json:"lang"`
	OfflineEnabled bool   `json:"offline_enabled"`
	OfflineMinutes int    `json:"offline_minutes"`
	LoadEnabled    bool   `json:"load_enabled"`
	LoadCPU        int    `json:"load_cpu"`
	LoadMem        int    `json:"load_mem"`
	LoadDisk       int    `json:"load_disk"`
	LoadMinutes    int    `json:"load_minutes"`
	ExpireEnabled  bool   `json:"expire_enabled"`
	ExpireDays     int    `json:"expire_days"`
	TrafficEnabled bool   `json:"traffic_enabled"`
	TrafficPercent int    `json:"traffic_percent"`
}

// DefaultNotifySettings 默认通知设置：规则全部开启，渠道未配置
func DefaultNotifySettings() NotifySettings {
	return NotifySettings{
		Lang:           "zh-CN",
		OfflineEnabled: true, OfflineMinutes: 3,
		LoadEnabled: true, LoadCPU: 90, LoadMem: 90, LoadDisk: 90, LoadMinutes: 5,
		ExpireEnabled: true, ExpireDays: 7,
		TrafficEnabled: true, TrafficPercent: 90,
	}
}

// 登录验证码模式
const (
	CaptchaNone      = "none"
	CaptchaImage     = "image"
	CaptchaTurnstile = "turnstile"
)

// CaptchaSettings 登录验证码：模式与 Cloudflare Turnstile 密钥（仅 turnstile 模式使用）
type CaptchaSettings struct {
	Mode             string `json:"mode"`
	TurnstileSiteKey string `json:"turnstile_site_key"`
	TurnstileSecret  string `json:"turnstile_secret"`
}

// Settings 运行期设置（后台修改后即时生效）
type Settings struct {
	SiteTitle             string          `json:"site_title"`
	ShowPrice             bool            `json:"show_price"`
	DefaultReportInterval int             `json:"default_report_interval"`
	InstallScriptBase     string          `json:"install_script_base"`
	Announcement          string          `json:"announcement"`
	Notify                NotifySettings  `json:"notify"`
	Captcha               CaptchaSettings `json:"captcha"`
}

// DefaultSettings 默认设置
func DefaultSettings() Settings {
	return Settings{
		SiteTitle:             "Simple Server Status",
		DefaultReportInterval: 2,
		InstallScriptBase:     DefaultInstallScriptBase,
		Notify:                DefaultNotifySettings(),
		Captcha:               CaptchaSettings{Mode: CaptchaNone},
	}
}

// settingField settings 表中的一个键与 Settings 字段的对应关系；ptr 为 *string、*bool 或 *int
type settingField struct {
	key string
	ptr any
}

func settingFields(st *Settings) []settingField {
	n := &st.Notify
	return []settingField{
		{"site_title", &st.SiteTitle},
		{"show_price", &st.ShowPrice},
		{"default_report_interval", &st.DefaultReportInterval},
		{"install_script_base", &st.InstallScriptBase},
		{"announcement", &st.Announcement},
		{"notify_webhook_url", &n.WebhookURL},
		{"notify_telegram_token", &n.TelegramToken},
		{"notify_telegram_chat_id", &n.TelegramChatID},
		{"notify_lang", &n.Lang},
		{"notify_offline_enabled", &n.OfflineEnabled},
		{"notify_offline_minutes", &n.OfflineMinutes},
		{"notify_load_enabled", &n.LoadEnabled},
		{"notify_load_cpu", &n.LoadCPU},
		{"notify_load_mem", &n.LoadMem},
		{"notify_load_disk", &n.LoadDisk},
		{"notify_load_minutes", &n.LoadMinutes},
		{"notify_expire_enabled", &n.ExpireEnabled},
		{"notify_expire_days", &n.ExpireDays},
		{"notify_traffic_enabled", &n.TrafficEnabled},
		{"notify_traffic_percent", &n.TrafficPercent},
		{"captcha_mode", &st.Captcha.Mode},
		{"captcha_turnstile_site_key", &st.Captcha.TurnstileSiteKey},
		{"captcha_turnstile_secret", &st.Captcha.TurnstileSecret},
	}
}

func (f settingField) set(v string) {
	switch p := f.ptr.(type) {
	case *string:
		*p = v
	case *bool:
		*p = v == "true"
	case *int:
		if n, err := strconv.Atoi(v); err == nil {
			*p = n
		}
	}
}

func (f settingField) get() string {
	switch p := f.ptr.(type) {
	case *string:
		return *p
	case *bool:
		return strconv.FormatBool(*p)
	case *int:
		return strconv.Itoa(*p)
	}
	return ""
}

// GetSettings 读取设置，缺失项使用默认值
func (s *Store) GetSettings(ctx context.Context) (Settings, error) {
	st := DefaultSettings()
	rows, err := s.db.QueryContext(ctx, `SELECT key, value FROM settings`)
	if err != nil {
		return st, err
	}
	defer rows.Close()
	vals := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return st, err
		}
		vals[k] = v
	}
	if err := rows.Err(); err != nil {
		return st, err
	}
	for _, f := range settingFields(&st) {
		if v, ok := vals[f.key]; ok {
			f.set(v)
		}
	}
	return st, nil
}

// SaveSettings 保存设置
func (s *Store) SaveSettings(ctx context.Context, st Settings) error {
	return s.withTx(ctx, func(tx *sql.Tx) error {
		for _, f := range settingFields(&st) {
			if err := upsertSetting(ctx, tx, f.key, f.get()); err != nil {
				return err
			}
		}
		return nil
	})
}

func upsertSetting(ctx context.Context, e execer, k, v string) error {
	_, err := e.ExecContext(ctx, `INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, k, v)
	return err
}

// JWTSecret 读取 JWT 签名密钥，不存在时生成 32 字节随机密钥。
// 生成后使用 INSERT ... ON CONFLICT DO NOTHING 写入并重新读回，
// 避免并发调用各自生成随机值、后写者覆盖先写者导致返回不一致密钥的竞态。
func (s *Store) JWTSecret(ctx context.Context) ([]byte, error) {
	v, err := s.readJWTSecret(ctx)
	if err == nil {
		return hex.DecodeString(v)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO settings (key, value) VALUES ('jwt_secret', ?) ON CONFLICT(key) DO NOTHING`,
		hex.EncodeToString(b)); err != nil {
		return nil, err
	}
	// 无论本次写入是否生效（可能已被并发调用抢先写入），
	// 都以数据库中的最终值为准，保证所有调用方返回同一密钥。
	v, err = s.readJWTSecret(ctx)
	if err != nil {
		return nil, err
	}
	return hex.DecodeString(v)
}

// readJWTSecret 读取已保存的 JWT 密钥（十六进制字符串）
func (s *Store) readJWTSecret(ctx context.Context) (string, error) {
	var v string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = 'jwt_secret'`).Scan(&v)
	return v, err
}
