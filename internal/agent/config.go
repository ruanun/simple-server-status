// Package agent 实现探针 Agent：读取配置、连接 Dashboard 并周期上报。
package agent

import (
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config Agent 配置
type Config struct {
	Dashboard     string `yaml:"dashboard"`
	ID            string `yaml:"id"`
	Secret        string `yaml:"secret"`
	DetectCountry bool   `yaml:"detect_country"`
	LogLevel      string `yaml:"log_level"`
	LogFile       string `yaml:"log_file"`
}

// DefaultConfigPaths 未指定 --config 时依次查找的配置文件
var DefaultConfigPaths = []string{"sss-agent.yaml", "/etc/sss/sss-agent.yaml"}

// DefaultConfig 默认配置
func DefaultConfig() Config {
	return Config{DetectCountry: true, LogLevel: "info"}
}

// LoadConfig 读取 YAML 配置。path 为空时按默认路径查找，全部不存在则返回默认配置
func LoadConfig(path string) (Config, error) {
	cfg := DefaultConfig()
	paths := DefaultConfigPaths
	if path != "" {
		paths = []string{path}
	}
	for _, p := range paths {
		b, err := os.ReadFile(p) //nolint:gosec // 配置文件路径由用户指定
		if errors.Is(err, fs.ErrNotExist) && path == "" {
			continue
		}
		if err != nil {
			return cfg, fmt.Errorf("读取配置 %s 失败: %w", p, err)
		}
		if err := yaml.Unmarshal(b, &cfg); err != nil {
			return cfg, fmt.Errorf("解析配置 %s 失败: %w", p, err)
		}
		return cfg, nil
	}
	return cfg, nil
}

// Validate 校验必填项
func (c Config) Validate() error {
	if c.Dashboard == "" {
		return errors.New("缺少 dashboard 地址")
	}
	if c.ID == "" || c.Secret == "" {
		return errors.New("缺少 id 或 secret")
	}
	_, err := WSURL(c.Dashboard)
	return err
}

// WSURL 将 Dashboard 地址转换为 Agent WebSocket 地址
func WSURL(dashboard string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(dashboard))
	if err != nil {
		return "", fmt.Errorf("dashboard 地址无效: %w", err)
	}
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	case "ws", "wss":
	default:
		return "", fmt.Errorf("dashboard 地址需以 http:// 或 https:// 开头: %q", dashboard)
	}
	if u.Host == "" {
		return "", fmt.Errorf("dashboard 地址缺少主机: %q", dashboard)
	}
	u.Path = strings.TrimSuffix(u.Path, "/") + "/api/agent/ws"
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}
