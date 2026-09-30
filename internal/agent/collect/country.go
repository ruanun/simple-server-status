package collect

import "strings"

// TraceURL Cloudflare trace 地址，返回内容中的 loc= 即国家代码
var TraceURL = "https://www.cloudflare.com/cdn-cgi/trace"

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
