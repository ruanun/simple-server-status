package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ruanun/simple-server-status/internal/dashboard/store"
)

// exportFile 导出 / 导入文件格式（包含服务器 secret，便于整机迁移）
type exportFile struct {
	Version    int            `json:"version"`
	ExportedAt int64          `json:"exported_at"`
	Settings   store.Settings `json:"settings"`
	Servers    []store.Server `json:"servers"`
}

func (a *API) adminExport(c *gin.Context) {
	list := a.serverList()
	for i := range list {
		list[i].StaticInfo = nil
		list[i].LastIP = ""
		list[i].LastSeen = 0
	}
	respond(c, exportFile{Version: 1, ExportedAt: a.Now().Unix(), Settings: a.currentSettings(), Servers: list})
}

// maxImportSize 导入文件大小上限
const maxImportSize = 10 << 20

func (a *API) adminImport(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxImportSize)
	f := exportFile{Settings: store.DefaultSettings()}
	if err := c.ShouldBindJSON(&f); err != nil { // 超过大小上限时同样返回 bad_request
		fail(c, http.StatusBadRequest, "bad_request", "文件格式错误")
		return
	}
	if f.Version != 1 {
		fail(c, http.StatusBadRequest, "bad_version", "不支持的导出文件版本")
		return
	}
	if err := normalizeSettings(&f.Settings); err != nil {
		fail(c, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	for i := range f.Servers {
		s := &f.Servers[i]
		if s.ID == "" || s.Secret == "" {
			fail(c, http.StatusBadRequest, "invalid_input", "服务器缺少 id 或 secret")
			return
		}
		in := inputFrom(*s)
		if err := in.normalize(f.Settings.DefaultReportInterval); err != nil {
			fail(c, http.StatusBadRequest, "invalid_input", fmt.Sprintf("服务器 %s：%v", s.ID, err))
			return
		}
		in.apply(s)
	}
	// 记录 secret 将被覆盖的服务器，导入后断开其旧连接
	var changed []string
	for _, s := range f.Servers {
		if old, ok := a.server(s.ID); ok && old.Secret != s.Secret {
			changed = append(changed, s.ID)
		}
	}
	ctx := c.Request.Context()
	if err := a.Store.UpsertServers(ctx, f.Servers); err != nil {
		a.internal(c, "导入服务器失败", err)
		return
	}
	if err := a.Store.SaveSettings(ctx, f.Settings); err != nil {
		a.internal(c, "导入设置失败", err)
		return
	}
	a.afterServerChange(ctx)
	for _, id := range changed {
		a.agents.kick(id)
	}
	for _, s := range f.Servers {
		a.pushConfig(s.ID)
	}
	respond(c, gin.H{"servers": len(f.Servers)})
}
