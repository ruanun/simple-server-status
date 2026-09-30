package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/ruanun/simple-server-status/internal/dashboard/store"
	"github.com/ruanun/simple-server-status/internal/proto"
)

func (a *API) registerAdmin(r *gin.Engine) {
	r.POST("/api/auth/login", a.login)
	r.GET("/api/auth/captcha", a.captchaChallenge)
	g := r.Group("/api", a.requireAuth())
	g.GET("/auth/me", a.me)
	g.PUT("/auth/password", a.changePassword)
	g.GET("/admin/servers", a.adminListServers)
	g.POST("/admin/servers", a.adminCreateServer)
	g.PUT("/admin/servers/:id", a.adminUpdateServer)
	g.DELETE("/admin/servers/:id", a.adminDeleteServer)
	g.POST("/admin/servers/:id/reset-secret", a.adminResetSecret)
	g.GET("/admin/servers/:id/install", a.adminInstall)
	g.PUT("/admin/server-order", a.adminServerOrder)
	g.GET("/admin/settings", a.adminGetSettings)
	g.PUT("/admin/settings", a.adminSaveSettings)
	g.POST("/admin/notify/test", a.adminNotifyTest)
	g.GET("/admin/export", a.adminExport)
	g.POST("/admin/import", a.adminImport)
	g.GET("/admin/overview", a.adminOverview)
	g.GET("/admin/outages", a.adminOutages)
	g.GET("/admin/notify-log", a.adminNotifyLog)
}

// serverInput 后台可编辑的服务器字段
type serverInput struct {
	Name            string   `json:"name"`
	Group           string   `json:"group"`
	Country         string   `json:"country"`
	Hidden          bool     `json:"hidden"`
	Price           *float64 `json:"price"`
	Currency        string   `json:"currency"`
	BillingCycle    string   `json:"billing_cycle"`
	ExpireAt        *int64   `json:"expire_at"`
	TrafficLimit    *int64   `json:"traffic_limit"`
	TrafficMode     string   `json:"traffic_mode"`
	TrafficResetDay int      `json:"traffic_reset_day"`
	ReportInterval  int      `json:"report_interval"`
	NICInclude      []string `json:"nic_include"`
	NICExclude      []string `json:"nic_exclude"`
	MountExclude    []string `json:"mount_exclude"`
	Note            string   `json:"note"`
	NotifyMuted     bool     `json:"notify_muted"`
}

var (
	trafficModes  = map[string]bool{"sum": true, "in": true, "out": true}
	billingCycles = map[string]bool{"": true, "monthly": true, "quarterly": true, "yearly": true, "once": true}
)

// normalize 补全默认值并校验
func (in *serverInput) normalize(defaultInterval int) error {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || utf8.RuneCountInString(in.Name) > 64 {
		return errors.New("名称不能为空且不超过 64 个字符")
	}
	in.Note = strings.TrimSpace(in.Note)
	if utf8.RuneCountInString(in.Note) > 2000 {
		return errors.New("备注不超过 2000 个字符")
	}
	in.Group = strings.TrimSpace(in.Group)
	in.Country = strings.ToUpper(strings.TrimSpace(in.Country))
	if in.Country != "" && len(in.Country) != 2 {
		return errors.New("国家代码应为两位字母")
	}
	if in.TrafficMode == "" {
		in.TrafficMode = "sum"
	}
	if !trafficModes[in.TrafficMode] {
		return errors.New("流量计算方式只能是 sum、in 或 out")
	}
	if in.TrafficResetDay == 0 {
		in.TrafficResetDay = 1
	}
	if in.TrafficResetDay < 1 || in.TrafficResetDay > 28 {
		return errors.New("流量重置日应在 1–28 之间")
	}
	if in.ReportInterval == 0 {
		in.ReportInterval = defaultInterval
	}
	if in.ReportInterval < 1 || in.ReportInterval > 60 {
		return errors.New("上报间隔应在 1–60 秒之间")
	}
	if !billingCycles[in.BillingCycle] {
		return errors.New("付费周期无效")
	}
	if in.Price != nil && *in.Price < 0 {
		return errors.New("价格不能为负数")
	}
	if in.TrafficLimit != nil && *in.TrafficLimit < 0 {
		return errors.New("流量配额不能为负数")
	}
	for _, l := range []*[]string{&in.NICInclude, &in.NICExclude, &in.MountExclude} {
		if *l == nil {
			*l = []string{}
		}
	}
	return nil
}

func (in serverInput) apply(s *store.Server) {
	s.Name, s.Group, s.Country, s.Hidden = in.Name, in.Group, in.Country, in.Hidden
	s.Price, s.Currency, s.BillingCycle, s.ExpireAt = in.Price, in.Currency, in.BillingCycle, in.ExpireAt
	s.TrafficLimit, s.TrafficMode, s.TrafficResetDay = in.TrafficLimit, in.TrafficMode, in.TrafficResetDay
	s.ReportInterval, s.NICInclude, s.NICExclude, s.MountExclude = in.ReportInterval, in.NICInclude, in.NICExclude, in.MountExclude
	s.Note, s.NotifyMuted = in.Note, in.NotifyMuted
}

func inputFrom(s store.Server) serverInput {
	return serverInput{Name: s.Name, Group: s.Group, Country: s.Country, Hidden: s.Hidden, Price: s.Price,
		Currency: s.Currency, BillingCycle: s.BillingCycle, ExpireAt: s.ExpireAt, TrafficLimit: s.TrafficLimit,
		TrafficMode: s.TrafficMode, TrafficResetDay: s.TrafficResetDay, ReportInterval: s.ReportInterval,
		NICInclude: s.NICInclude, NICExclude: s.NICExclude, MountExclude: s.MountExclude,
		Note: s.Note, NotifyMuted: s.NotifyMuted}
}

// bindServerInput 解析并校验请求体，失败时已输出 400
func (a *API) bindServerInput(c *gin.Context) (serverInput, bool) {
	var in serverInput
	if err := c.ShouldBindJSON(&in); err != nil {
		fail(c, http.StatusBadRequest, "bad_request", "请求格式错误")
		return in, false
	}
	if err := in.normalize(a.currentSettings().DefaultReportInterval); err != nil {
		fail(c, http.StatusBadRequest, "invalid_input", err.Error())
		return in, false
	}
	return in, true
}

// afterServerChange 刷新缓存，并让浏览器下一次收到完整快照
func (a *API) afterServerChange(ctx context.Context) {
	if err := a.reload(ctx); err != nil {
		a.Log.Error("刷新服务器缓存失败", "err", err)
	}
	a.bc.requestSnapshot()
}

type adminServer struct {
	store.Server
	Online       bool     `json:"online"`
	AgentVersion string   `json:"agent_version"`
	Outdated     bool     `json:"outdated"`
	Uptime24h    *float64 `json:"uptime_24h"`
}

func (a *API) adminListServers(c *gin.Context) {
	list := []adminServer{}
	for _, s := range a.serverList() {
		live := a.Hub.Get(s.ID)
		// 与公开视图一致：有实时上报时 last_seen 取最近一次上报时间
		if live.LastReport > 0 {
			s.LastSeen = live.LastReport
		}
		static := s.StaticInfo
		if live.Static != nil {
			static = live.Static
		}
		ver := ""
		if static != nil {
			ver = static.AgentVersion
		}
		list = append(list, adminServer{Server: s, Online: live.Online, AgentVersion: ver,
			Outdated: agentOutdated(ver, a.Version), Uptime24h: a.cachedUptime(s.ID)})
	}
	respond(c, list)
}

func (a *API) adminCreateServer(c *gin.Context) {
	in, ok := a.bindServerInput(c)
	if !ok {
		return
	}
	s := store.Server{Sort: len(a.serverList())}
	in.apply(&s)
	if err := a.Store.CreateServer(c.Request.Context(), &s); err != nil {
		a.internal(c, "创建服务器失败", err)
		return
	}
	a.afterServerChange(c.Request.Context())
	respond(c, s)
}

func (a *API) adminUpdateServer(c *gin.Context) {
	srv, found := a.server(c.Param("id"))
	if !found {
		fail(c, http.StatusNotFound, "not_found", "服务器不存在")
		return
	}
	in, ok := a.bindServerInput(c)
	if !ok {
		return
	}
	in.apply(&srv)
	if err := a.Store.UpdateServer(c.Request.Context(), srv); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			fail(c, http.StatusNotFound, "not_found", "服务器不存在")
			return
		}
		a.internal(c, "更新服务器失败", err)
		return
	}
	a.afterServerChange(c.Request.Context())
	a.pushConfig(srv.ID)
	updated, _ := a.server(srv.ID)
	respond(c, updated)
}

func (a *API) adminDeleteServer(c *gin.Context) {
	id := c.Param("id")
	if err := a.Store.DeleteServer(c.Request.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			fail(c, http.StatusNotFound, "not_found", "服务器不存在")
			return
		}
		a.internal(c, "删除服务器失败", err)
		return
	}
	if msg, err := proto.Encode(proto.TypeStop, proto.Stop{Reason: "deleted"}); err == nil {
		a.agents.send(id, msg)
	}
	// 留出时间让 stop 消息送达，之后强制断开
	time.AfterFunc(2*time.Second, func() { a.agents.kick(id) })
	// 先从缓存移除，之后到达的上报会因找不到服务器而被忽略，再清理各组件中的残留数据
	a.afterServerChange(c.Request.Context())
	a.Hub.Remove(id)
	a.Traffic.Forget(id)
	a.History.Forget(id)
	a.notifier.Forget(id)
	a.outages.Forget(id)
	respond(c, gin.H{})
}

func (a *API) adminResetSecret(c *gin.Context) {
	id := c.Param("id")
	sec, err := a.Store.ResetSecret(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			fail(c, http.StatusNotFound, "not_found", "服务器不存在")
			return
		}
		a.internal(c, "重置密钥失败", err)
		return
	}
	a.afterServerChange(c.Request.Context())
	a.agents.kick(id)
	respond(c, gin.H{"secret": sec})
}

func (a *API) adminServerOrder(c *gin.Context) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "bad_request", "请求格式错误")
		return
	}
	if err := a.Store.SetServerOrder(c.Request.Context(), req.IDs); err != nil {
		a.internal(c, "保存排序失败", err)
		return
	}
	a.afterServerChange(c.Request.Context())
	respond(c, gin.H{})
}

// dashboardPattern dashboard 参数白名单：仅允许 http(s)、主机、可选端口与安全路径字符，
// 阻止 shell 元字符（;、&、$ 等）注入到一键安装命令中
var dashboardPattern = regexp.MustCompile(`^https?://[A-Za-z0-9.\-]+(:[0-9]+)?(/[A-Za-z0-9._~\-/]*)?$`)

// shQuote 按 POSIX 规则用单引号包裹字符串，内部单引号替换为转义序列，防止 shell 注入
func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// psQuote 按 PowerShell 规则用单引号包裹字符串，内部单引号替换为两个单引号，防止命令注入
func psQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// releaseVersionPattern 正式发布版本号（可带预发布后缀，如 2.0.0-beta.1）；git describe 生成的开发版本不匹配
var releaseVersionPattern = regexp.MustCompile(`^v?\d+\.\d+\.\d+(-[0-9A-Za-z.]+)?$`)

// releaseTag 返回 Dashboard 版本对应的 Release 标签（带 v 前缀）；开发版本返回空串
func releaseTag(version string) string {
	if !releaseVersionPattern.MatchString(version) {
		return ""
	}
	return "v" + strings.TrimPrefix(version, "v")
}

// installCommands 生成 Linux / Windows 一键安装命令；所有插值参数均加引号，防止 shell/PowerShell 注入。
// Dashboard 为正式发布版本时锁定同版本：默认脚本地址改为该版本的 Release 资产，并让脚本安装同版本 Agent
func installCommands(base, version, dashboard, id, secret string) gin.H {
	base = strings.TrimRight(base, "/")
	linuxVer, winVer := "", ""
	if tag := releaseTag(version); tag != "" {
		if base == store.DefaultInstallScriptBase {
			base = store.ReleaseDownloadBase + tag
		}
		linuxVer = " --version " + shQuote(tag)
		winVer = " -Version " + psQuote(tag)
	}
	linux := fmt.Sprintf("curl -fsSL %s | sudo bash -s -- --dashboard %s --id %s --secret %s%s",
		shQuote(base+"/install-agent.sh"), shQuote(dashboard), shQuote(id), shQuote(secret), linuxVer)
	windows := fmt.Sprintf(`Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass -Force; $f="$env:TEMP\install-agent.ps1"; iwr -useb %s -OutFile $f; & $f -Dashboard %s -Id %s -Secret %s%s`,
		psQuote(base+"/install-agent.ps1"), psQuote(dashboard), psQuote(id), psQuote(secret), winVer)
	return gin.H{"linux": linux, "windows": windows}
}

func (a *API) adminInstall(c *gin.Context) {
	srv, found := a.server(c.Param("id"))
	if !found {
		fail(c, http.StatusNotFound, "not_found", "服务器不存在")
		return
	}
	u, err := url.Parse(strings.TrimSpace(c.Query("dashboard")))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		fail(c, http.StatusBadRequest, "invalid_input", "缺少或无效的 dashboard 参数")
		return
	}
	dash := strings.TrimRight(u.Scheme+"://"+u.Host+u.EscapedPath(), "/")
	if !dashboardPattern.MatchString(dash) {
		fail(c, http.StatusBadRequest, "invalid_input", "缺少或无效的 dashboard 参数")
		return
	}
	respond(c, installCommands(a.currentSettings().InstallScriptBase, a.Version, dash, srv.ID, srv.Secret))
}

// normalizeSettings 补全默认值并校验设置
func normalizeSettings(st *store.Settings) error {
	st.SiteTitle = strings.TrimSpace(st.SiteTitle)
	if st.SiteTitle == "" {
		st.SiteTitle = store.DefaultSettings().SiteTitle
	}
	if st.DefaultReportInterval < 1 || st.DefaultReportInterval > 60 {
		return errors.New("默认上报间隔应在 1–60 秒之间")
	}
	st.InstallScriptBase = strings.TrimRight(strings.TrimSpace(st.InstallScriptBase), "/")
	if st.InstallScriptBase == "" {
		st.InstallScriptBase = store.DefaultInstallScriptBase
	}
	if !strings.HasPrefix(st.InstallScriptBase, "http://") && !strings.HasPrefix(st.InstallScriptBase, "https://") {
		return errors.New("安装脚本地址需以 http:// 或 https:// 开头")
	}
	st.Announcement = strings.TrimSpace(st.Announcement)
	if utf8.RuneCountInString(st.Announcement) > 1000 {
		return errors.New("公告不超过 1000 个字符")
	}
	if err := normalizeCaptcha(&st.Captcha); err != nil {
		return err
	}
	return normalizeNotify(&st.Notify)
}

func (a *API) adminGetSettings(c *gin.Context) {
	respond(c, a.currentSettings())
}

// saveSettingsReq 保存设置的请求体：设置本身，外加新启用 Turnstile 时用于确认密钥可用的 token
type saveSettingsReq struct {
	store.Settings
	TurnstileToken string `json:"turnstile_token"`
}

func (a *API) adminSaveSettings(c *gin.Context) {
	req := saveSettingsReq{Settings: store.DefaultSettings()}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "bad_request", "请求格式错误")
		return
	}
	st := req.Settings
	if err := normalizeSettings(&st); err != nil {
		fail(c, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	if !a.checkTurnstileOnSave(c, st.Captcha, req.TurnstileToken) {
		return
	}
	if err := a.Store.SaveSettings(c.Request.Context(), st); err != nil {
		a.internal(c, "保存设置失败", err)
		return
	}
	a.afterServerChange(c.Request.Context())
	respond(c, st)
}
