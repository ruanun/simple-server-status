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

// Settings 运行期设置（后台修改后即时生效）
type Settings struct {
	SiteTitle             string `json:"site_title"`
	ShowPrice             bool   `json:"show_price"`
	DefaultReportInterval int    `json:"default_report_interval"`
	InstallScriptBase     string `json:"install_script_base"`
}

// DefaultSettings 默认设置
func DefaultSettings() Settings {
	return Settings{
		SiteTitle:             "Simple Server Status",
		DefaultReportInterval: 2,
		InstallScriptBase:     DefaultInstallScriptBase,
	}
}

// GetSettings 读取设置，缺失项使用默认值
func (s *Store) GetSettings(ctx context.Context) (Settings, error) {
	st := DefaultSettings()
	rows, err := s.db.QueryContext(ctx, `SELECT key, value FROM settings`)
	if err != nil {
		return st, err
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return st, err
		}
		switch k {
		case "site_title":
			st.SiteTitle = v
		case "show_price":
			st.ShowPrice = v == "true"
		case "default_report_interval":
			if n, err := strconv.Atoi(v); err == nil {
				st.DefaultReportInterval = n
			}
		case "install_script_base":
			st.InstallScriptBase = v
		}
	}
	return st, rows.Err()
}

// SaveSettings 保存设置
func (s *Store) SaveSettings(ctx context.Context, st Settings) error {
	kv := map[string]string{
		"site_title":              st.SiteTitle,
		"show_price":              strconv.FormatBool(st.ShowPrice),
		"default_report_interval": strconv.Itoa(st.DefaultReportInterval),
		"install_script_base":     st.InstallScriptBase,
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		for k, v := range kv {
			if err := upsertSetting(ctx, tx, k, v); err != nil {
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
