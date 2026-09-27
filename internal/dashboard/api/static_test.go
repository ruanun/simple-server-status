package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gin-gonic/gin"
)

func serveStatic(t *testing.T, fsys fstest.MapFS, path string) *http.Response {
	t.Helper()
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.NoRoute(spaHandler(fsys))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	return w.Result()
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}

func TestSPAHandler(t *testing.T) {
	fsys := fstest.MapFS{
		"index.html":    {Data: []byte("<html>app</html>")},
		"assets/app.js": {Data: []byte("console.log(1)")},
		"favicon.svg":   {Data: []byte("<svg/>")},
	}
	for _, p := range []string{"/", "/server/abc", "/admin/servers"} {
		resp := serveStatic(t, fsys, p)
		if resp.StatusCode != http.StatusOK || !strings.Contains(readBody(t, resp), "app") {
			t.Errorf("%s 应返回 index.html，实际 %d", p, resp.StatusCode)
		}
	}
	resp := serveStatic(t, fsys, "/assets/app.js")
	if resp.StatusCode != http.StatusOK || readBody(t, resp) != "console.log(1)" || !strings.Contains(resp.Header.Get("Cache-Control"), "immutable") {
		t.Errorf("静态资源错误: %d %q", resp.StatusCode, resp.Header.Get("Cache-Control"))
	}
	if resp := serveStatic(t, fsys, "/favicon.svg"); resp.StatusCode != http.StatusOK || resp.Header.Get("Cache-Control") != "" {
		t.Errorf("非 assets 文件不应长期缓存: %d %q", resp.StatusCode, resp.Header.Get("Cache-Control"))
	}
	if resp := serveStatic(t, fsys, "/missing.js"); resp.StatusCode != http.StatusNotFound {
		t.Errorf("不存在的带扩展名文件应返回 404，实际 %d", resp.StatusCode)
	}
	resp = serveStatic(t, fsys, "/api/unknown")
	if resp.StatusCode != http.StatusNotFound || !strings.Contains(readBody(t, resp), "not_found") {
		t.Errorf("未知 API 应返回 JSON 404，实际 %d", resp.StatusCode)
	}
	resp = serveStatic(t, fstest.MapFS{}, "/")
	if resp.StatusCode != http.StatusNotFound || !strings.Contains(readBody(t, resp), "前端未构建") {
		t.Errorf("未构建前端时应提示，实际 %d", resp.StatusCode)
	}
}
