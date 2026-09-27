package collect

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// TraceURL Cloudflare trace 地址，返回内容中的 loc= 即国家代码
var TraceURL = "https://www.cloudflare.com/cdn-cgi/trace"

// DetectCountry 通过 Cloudflare trace 探测本机出口所在国家代码
func DetectCountry(ctx context.Context, client *http.Client, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求 trace 失败: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return "", fmt.Errorf("读取 trace 失败: %w", err)
	}
	return ParseTraceLoc(string(body)), nil
}

// ParseTraceLoc 从 trace 内容中提取两位国家代码，无效时返回空字符串
func ParseTraceLoc(body string) string {
	for _, line := range strings.Split(body, "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "loc="); ok {
			v = strings.ToUpper(strings.TrimSpace(v))
			if len(v) == 2 && v != "XX" {
				return v
			}
		}
	}
	return ""
}
