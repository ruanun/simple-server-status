package api

import (
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

// spaHandler 提供前端静态资源；非 /api/ 且无扩展名的 GET 请求回退到 index.html
func spaHandler(webFS fs.FS) gin.HandlerFunc {
	fileServer := http.FileServer(http.FS(webFS))
	return func(c *gin.Context) {
		p := c.Request.URL.Path
		if (c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead) || strings.HasPrefix(p, "/api/") {
			fail(c, http.StatusNotFound, "not_found", "接口不存在")
			return
		}
		name := strings.TrimPrefix(path.Clean(p), "/")
		if name != "" && name != "index.html" {
			if st, err := fs.Stat(webFS, name); err == nil && !st.IsDir() {
				if strings.HasPrefix(name, "assets/") {
					c.Header("Cache-Control", "public, max-age=31536000, immutable")
				}
				fileServer.ServeHTTP(c.Writer, c.Request)
				return
			}
		}
		if path.Ext(name) != "" && name != "index.html" {
			c.String(http.StatusNotFound, "not found")
			return
		}
		index, err := fs.ReadFile(webFS, "index.html")
		if err != nil {
			c.String(http.StatusNotFound, "前端未构建：请先执行 make build-web")
			return
		}
		c.Header("Cache-Control", "no-cache")
		c.Data(http.StatusOK, "text/html; charset=utf-8", index)
	}
}
