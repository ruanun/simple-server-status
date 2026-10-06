package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ruanun/simple-server-status/internal/dashboard/store"
	"github.com/ruanun/simple-server-status/internal/proto"
)

func login(t *testing.T, e *testEnv, password string) (int, string) {
	t.Helper()
	code, body := e.do("POST", "/api/auth/login", "", gin.H{"username": "admin", "password": password})
	if code != http.StatusOK {
		return code, ""
	}
	return code, decodeData[struct {
		Token string `json:"token"`
	}](t, body).Token
}

func TestLoginFlowAndRateLimit(t *testing.T) {
	e := newTestEnv(t)
	e.adminToken() // 创建 admin/password123
	for i := 0; i < 5; i++ {
		if code, _ := login(t, e, "wrong"); code != http.StatusUnauthorized {
			t.Fatalf("第 %d 次错误密码应返回 401，实际 %d", i+1, code)
		}
	}
	if code, _ := login(t, e, "password123"); code != http.StatusTooManyRequests {
		t.Fatalf("连续失败后应限流，实际 %d", code)
	}
	e.clock.Add(6 * time.Minute)
	code, tok := login(t, e, "password123")
	if code != http.StatusOK || tok == "" {
		t.Fatalf("解锁后应能登录，实际 %d", code)
	}
	if code, body := e.do("GET", "/api/auth/me", tok, nil); code != http.StatusOK || !strings.Contains(string(body), "admin") {
		t.Fatalf("me = %d %s", code, body)
	}
}

// lockedBuffer 并发安全的日志缓冲（日志在服务端 goroutine 中写入）
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestLoginLogsAttempts(t *testing.T) {
	var out lockedBuffer
	e := newTestEnv(t, func(d *Deps) {
		d.Log = slog.New(slog.NewTextHandler(&out, &slog.HandlerOptions{Level: slog.LevelDebug}))
	})
	e.adminToken()
	if code, _ := login(t, e, "password123"); code != http.StatusOK {
		t.Fatalf("登录应成功，实际 %d", code)
	}
	if !strings.Contains(out.String(), `level=INFO msg=登录成功 ip=127.0.0.1 username=admin`) {
		t.Fatalf("登录成功应记录来源：\n%s", out.String())
	}
	long := strings.Repeat("中", 100)
	e.do("POST", "/api/auth/login", "", gin.H{"username": long, "password": "wrong"})
	for i := 0; i < 4; i++ {
		login(t, e, "wrong")
	}
	if code, _ := login(t, e, "password123"); code != http.StatusTooManyRequests {
		t.Fatalf("连续失败后应限流，实际 %d", code)
	}
	logs := out.String()
	if n := strings.Count(logs, `level=WARN msg=登录失败 ip=127.0.0.1`); n != 5 {
		t.Fatalf("5 次失败应各记一条 warn，实际 %d：\n%s", n, logs)
	}
	if strings.Contains(logs, long) || !strings.Contains(logs, strings.Repeat("中", 64)+"…") {
		t.Fatalf("超长用户名应截断：\n%s", logs)
	}
	if !strings.Contains(logs, `level=DEBUG msg=登录失败 ip=127.0.0.1 username=admin reason=too_many_attempts`) {
		t.Fatalf("被限流拒绝的请求应记 debug：\n%s", logs)
	}
}

func TestMeIncludesVersion(t *testing.T) {
	e := newTestEnv(t)
	e.api.Version = "2.0.0-beta.9"
	code, body := e.do("GET", "/api/auth/me", e.adminToken(), nil)
	me := decodeData[struct {
		Username string `json:"username"`
		Version  string `json:"version"`
	}](t, body)
	if code != http.StatusOK || me.Username != "admin" || me.Version != "2.0.0-beta.9" {
		t.Fatalf("me 应包含用户名与 Dashboard 版本：%d %s", code, body)
	}
	if _, body := e.do("GET", "/api/public/site", "", nil); strings.Contains(string(body), "2.0.0-beta.9") {
		t.Fatalf("公开接口不应包含 Dashboard 版本：%s", body)
	}
}

func TestLoginRateLimitConcurrent(t *testing.T) {
	e := newTestEnv(t)
	e.adminToken()
	var wg sync.WaitGroup
	var checked atomic.Int32
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b, _ := json.Marshal(gin.H{"username": "admin", "password": "wrong"})
			resp, err := http.Post(e.srv.URL+"/api/auth/login", "application/json", bytes.NewReader(b))
			if err != nil {
				return
			}
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusTooManyRequests {
				checked.Add(1)
			}
		}()
	}
	wg.Wait()
	if n := checked.Load(); n > 5 {
		t.Fatalf("并发错误登录中进入密码校验的请求应不超过 5 个，实际 %d", n)
	}
}

func TestChangePasswordRevokesOldToken(t *testing.T) {
	e := newTestEnv(t)
	old := e.adminToken()
	if code, body := e.do("PUT", "/api/auth/password", old, gin.H{"old_password": "password123", "new_password": "short"}); code != http.StatusBadRequest || errorCode(t, body) != "weak_password" {
		t.Fatalf("弱密码应被拒绝: %d %s", code, body)
	}
	if code, body := e.do("PUT", "/api/auth/password", old, gin.H{"old_password": "bad", "new_password": "newpass123"}); code != http.StatusBadRequest || errorCode(t, body) != "wrong_password" {
		t.Fatalf("原密码错误应被拒绝: %d %s", code, body)
	}
	code, body := e.do("PUT", "/api/auth/password", old, gin.H{"old_password": "password123", "new_password": "newpass123"})
	if code != http.StatusOK {
		t.Fatalf("修改密码失败: %d %s", code, body)
	}
	fresh := decodeData[struct {
		Token string `json:"token"`
	}](t, body).Token
	if code, _ := e.do("GET", "/api/auth/me", old, nil); code != http.StatusUnauthorized {
		t.Fatalf("旧 token 应失效，实际 %d", code)
	}
	if code, _ := e.do("GET", "/api/auth/me", fresh, nil); code != http.StatusOK {
		t.Fatalf("新 token 应有效，实际 %d", code)
	}
}

func TestAdminRequiresAuth(t *testing.T) {
	e := newTestEnv(t)
	if code, body := e.do("GET", "/api/admin/servers", "", nil); code != http.StatusUnauthorized || errorCode(t, body) != "unauthorized" {
		t.Fatalf("未登录应返回 401，实际 %d %s", code, body)
	}
}

func createServer(t *testing.T, e *testEnv, tok string, body gin.H) store.Server {
	t.Helper()
	code, b := e.do("POST", "/api/admin/servers", tok, body)
	if code != http.StatusOK {
		t.Fatalf("创建服务器失败: %d %s", code, b)
	}
	return decodeData[store.Server](t, b)
}

func TestAdminServerCRUD(t *testing.T) {
	e := newTestEnv(t)
	tok := e.adminToken()
	if code, body := e.do("POST", "/api/admin/servers", tok, gin.H{"name": "n1", "traffic_reset_day": 40}); code != http.StatusBadRequest || errorCode(t, body) != "invalid_input" {
		t.Fatalf("非法重置日应返回 400，实际 %d %s", code, body)
	}
	a := createServer(t, e, tok, gin.H{"name": " n1 ", "group": "HK", "country": "hk"})
	if a.Name != "n1" || a.Country != "HK" || len(a.Secret) != 32 || a.ReportInterval != 2 || a.TrafficMode != "sum" {
		t.Fatalf("创建结果错误: %+v", a)
	}
	b := createServer(t, e, tok, gin.H{"name": "n0"})

	_, body := e.do("GET", "/api/admin/servers", tok, nil)
	list := decodeData[[]struct {
		store.Server
		Online bool `json:"online"`
	}](t, body)
	if len(list) != 2 || list[0].ID != a.ID || list[0].Secret == "" {
		t.Fatalf("列表错误（应按创建顺序排序且包含 secret）: %+v", list)
	}

	code, body := e.do("PUT", "/api/admin/servers/"+a.ID, tok, gin.H{"name": "n2", "hidden": true})
	if updated := decodeData[store.Server](t, body); code != http.StatusOK || updated.Name != "n2" || !updated.Hidden {
		t.Fatalf("更新失败: %d %s", code, body)
	}
	if code, _ := e.do("PUT", "/api/admin/servers/missing", tok, gin.H{"name": "x"}); code != http.StatusNotFound {
		t.Fatalf("更新不存在的服务器应返回 404，实际 %d", code)
	}

	if code, _ := e.do("PUT", "/api/admin/server-order", tok, gin.H{"ids": []string{b.ID, a.ID}}); code != http.StatusOK {
		t.Fatalf("排序失败: %d", code)
	}
	_, body = e.do("GET", "/api/admin/servers", tok, nil)
	if list := decodeData[[]store.Server](t, body); list[0].ID != b.ID {
		t.Fatalf("排序未生效: %+v", list)
	}

	if code, _ := e.do("DELETE", "/api/admin/servers/"+a.ID, tok, nil); code != http.StatusOK {
		t.Fatalf("删除失败: %d", code)
	}
	if code, _ := e.do("DELETE", "/api/admin/servers/"+a.ID, tok, nil); code != http.StatusNotFound {
		t.Fatalf("重复删除应返回 404，实际 %d", code)
	}
}

func TestAdminUpdatePushesConfig(t *testing.T) {
	e := newTestEnv(t)
	tok := e.adminToken()
	s := createServer(t, e, tok, gin.H{"name": "a"})
	conn, _, err := e.dialAgent(s.ID, s.Secret)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	sendMsg(t, conn, proto.TypeHello, proto.Hello{})
	readMsg(t, conn)
	e.do("PUT", "/api/admin/servers/"+s.ID, tok, gin.H{"name": "a", "report_interval": 5})
	env := readMsg(t, conn)
	var cfg proto.Config
	_ = json.Unmarshal(env.Data, &cfg)
	if env.Type != proto.TypeConfig || cfg.ReportInterval != 5 {
		t.Fatalf("更新后应推送新参数: %s %+v", env.Type, cfg)
	}
}

func TestAdminDeleteSendsStop(t *testing.T) {
	e := newTestEnv(t)
	tok := e.adminToken()
	s := createServer(t, e, tok, gin.H{"name": "a"})
	conn, _, err := e.dialAgent(s.ID, s.Secret)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	sendMsg(t, conn, proto.TypeHello, proto.Hello{})
	readMsg(t, conn)
	e.do("DELETE", "/api/admin/servers/"+s.ID, tok, nil)
	env := readMsg(t, conn)
	var stop proto.Stop
	_ = json.Unmarshal(env.Data, &stop)
	if env.Type != proto.TypeStop || stop.Reason != "deleted" {
		t.Fatalf("删除后应下发 stop: %s %+v", env.Type, stop)
	}
}

func TestAdminResetSecretKicksAgent(t *testing.T) {
	e := newTestEnv(t)
	tok := e.adminToken()
	s := createServer(t, e, tok, gin.H{"name": "a"})
	conn, _, err := e.dialAgent(s.ID, s.Secret)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	sendMsg(t, conn, proto.TypeHello, proto.Hello{})
	readMsg(t, conn)
	code, body := e.do("POST", "/api/admin/servers/"+s.ID+"/reset-secret", tok, nil)
	sec := decodeData[struct {
		Secret string `json:"secret"`
	}](t, body).Secret
	if code != http.StatusOK || sec == "" || sec == s.Secret {
		t.Fatalf("重置密钥失败: %d %s", code, body)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, _, err := conn.Read(ctx); err == nil {
		t.Fatal("重置密钥后旧连接应被断开")
	}
	if _, resp, _ := e.dialAgent(s.ID, s.Secret); resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatal("旧密钥应被拒绝")
	}
	c2, _, err := e.dialAgent(s.ID, sec)
	if err != nil {
		t.Fatalf("新密钥应可连接: %v", err)
	}
	c2.CloseNow()
}

func TestAdminInstallCommand(t *testing.T) {
	e := newTestEnv(t)
	tok := e.adminToken()
	s := createServer(t, e, tok, gin.H{"name": "a"})
	if code, body := e.do("GET", "/api/admin/servers/"+s.ID+"/install", tok, nil); code != http.StatusBadRequest || errorCode(t, body) != "invalid_input" {
		t.Fatalf("缺少 dashboard 参数应返回 400 invalid_input，实际 %d %s", code, body)
	}
	code, body := e.do("GET", "/api/admin/servers/"+s.ID+"/install?dashboard="+url.QueryEscape("https://status.example.com/"), tok, nil)
	cmds := decodeData[map[string]string](t, body)
	wantLinux := "curl -fsSL '" + store.DefaultInstallScriptBase + "/install-agent.sh' | sudo bash -s -- --dashboard 'https://status.example.com' --id '" + s.ID + "' --secret '" + s.Secret + "'"
	if code != http.StatusOK || cmds["linux"] != wantLinux {
		t.Fatalf("Linux 命令错误:\n%s\n期望:\n%s", cmds["linux"], wantLinux)
	}
	wantWindows := `Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass -Force; $f="$env:TEMP\install-agent.ps1"; iwr -useb '` +
		store.DefaultInstallScriptBase + "/install-agent.ps1' -OutFile $f; & $f -Dashboard 'https://status.example.com' -Id '" + s.ID + "' -Secret '" + s.Secret + "'"
	if code != http.StatusOK || cmds["windows"] != wantWindows {
		t.Fatalf("Windows 命令错误:\n%s\n期望:\n%s", cmds["windows"], wantWindows)
	}
}

func TestAdminInstallCommandRejectsShellMetachars(t *testing.T) {
	e := newTestEnv(t)
	tok := e.adminToken()
	s := createServer(t, e, tok, gin.H{"name": "a"})
	for _, dashboard := range []string{
		"https://evil.com/foo%26%26touch%20x",
		"https://evil.com/a;b",
		"https://evil.com/$(id)",
	} {
		if code, body := e.do("GET", "/api/admin/servers/"+s.ID+"/install?dashboard="+dashboard, tok, nil); code != http.StatusBadRequest || errorCode(t, body) != "invalid_input" {
			t.Fatalf("dashboard=%s 应返回 400，实际 %d %s", dashboard, code, body)
		}
	}
}

func TestShQuoteAndPsQuote(t *testing.T) {
	if got := shQuote("a'b"); got != `'a'\''b'` {
		t.Fatalf("shQuote 错误: %s", got)
	}
	if got := psQuote("a'b"); got != "'a''b'" {
		t.Fatalf("psQuote 错误: %s", got)
	}
}

func TestAdminSettings(t *testing.T) {
	e := newTestEnv(t)
	tok := e.adminToken()
	bad := store.Settings{SiteTitle: "x", DefaultReportInterval: 0, InstallScriptBase: "https://x"}
	if code, _ := e.do("PUT", "/api/admin/settings", tok, bad); code != http.StatusBadRequest {
		t.Fatalf("非法间隔应返回 400，实际 %d", code)
	}
	good := store.Settings{SiteTitle: "我的探针", ShowPrice: true, DefaultReportInterval: 5, InstallScriptBase: "https://x/",
		Notify: store.DefaultNotifySettings(), Events: store.DefaultEventSettings()}
	code, body := e.do("PUT", "/api/admin/settings", tok, good)
	saved := decodeData[store.Settings](t, body)
	if code != http.StatusOK || saved.InstallScriptBase != "https://x" {
		t.Fatalf("保存设置失败: %d %s", code, body)
	}
	_, body = e.do("GET", "/api/public/site", "", nil)
	if site := decodeData[map[string]any](t, body); site["site_title"] != "我的探针" || site["show_price"] != true {
		t.Fatalf("公开站点信息未更新: %v", site)
	}
	if s := createServer(t, e, tok, gin.H{"name": "a"}); s.ReportInterval != 5 {
		t.Fatalf("新服务器应使用默认上报间隔 5，实际 %d", s.ReportInterval)
	}
}

func TestSettingsNotifyValidation(t *testing.T) {
	e := newTestEnv(t)
	tok := e.adminToken()
	base := func(mod func(n gin.H)) gin.H {
		n := gin.H{"webhook_url": "", "telegram_token": "", "telegram_chat_id": "", "lang": "zh-CN",
			"offline_enabled": true, "offline_minutes": 3, "load_enabled": true, "reboot_enabled": true, "ip_change_enabled": true,
			"expire_enabled": true, "expire_days": 7, "traffic_enabled": true, "traffic_percent": 90}
		ev := gin.H{"load_cpu": 90, "load_mem": 90, "load_disk": 90, "load_minutes": 5}
		mod(n)
		if v, ok := n["events"]; ok {
			ev = v.(gin.H)
			delete(n, "events")
		}
		return gin.H{"site_title": "t", "default_report_interval": 2, "install_script_base": "https://x.example.com", "notify": n, "events": ev}
	}
	if code, body := e.do("PUT", "/api/admin/settings", tok, base(func(gin.H) {})); code != http.StatusOK {
		t.Fatalf("合法设置应保存成功，实际 %d %s", code, body)
	}
	bad := []func(n gin.H){
		func(n gin.H) { n["webhook_url"] = "ftp://x" },
		func(n gin.H) { n["telegram_token"] = "123:abc" },
		func(n gin.H) { n["lang"] = "fr" },
		func(n gin.H) { n["offline_minutes"] = 0 },
		func(n gin.H) {
			n["events"] = gin.H{"load_cpu": 90, "load_mem": 90, "load_disk": 90, "load_minutes": 11}
		},
		func(n gin.H) { n["events"] = gin.H{"load_cpu": 0, "load_mem": 90, "load_disk": 90, "load_minutes": 5} },
		func(n gin.H) { n["expire_days"] = 91 },
		func(n gin.H) { n["traffic_percent"] = 101 },
	}
	for i, mod := range bad {
		if code, body := e.do("PUT", "/api/admin/settings", tok, base(mod)); code != http.StatusBadRequest || errorCode(t, body) != "invalid_input" {
			t.Errorf("第 %d 个非法设置应返回 invalid_input，实际 %d %s", i, code, body)
		}
	}
	if code, body := e.do("PUT", "/api/admin/settings", tok, gin.H{"site_title": "t", "default_report_interval": 2, "announcement": strings.Repeat("字", 1001)}); code != http.StatusBadRequest {
		t.Errorf("超长公告应返回 400，实际 %d %s", code, body)
	}
	ok := base(func(n gin.H) { n["telegram_token"], n["telegram_chat_id"] = "123:SECRET", "42" })
	ok["announcement"] = "  维护通知  "
	if code, body := e.do("PUT", "/api/admin/settings", tok, ok); code != http.StatusOK {
		t.Fatalf("合法设置保存失败 %d %s", code, body)
	}
	code, body := e.do("GET", "/api/public/site", "", nil)
	if code != http.StatusOK || !strings.Contains(string(body), `"announcement":"维护通知"`) || strings.Contains(string(body), "SECRET") || strings.Contains(string(body), "notify") {
		t.Fatalf("公开站点信息错误或泄露通知设置: %s", body)
	}
}

func TestSettingsMissingNotifyUsesDefaults(t *testing.T) {
	e := newTestEnv(t)
	tok := e.adminToken()
	if code, body := e.do("PUT", "/api/admin/settings", tok, gin.H{"site_title": "t", "default_report_interval": 2}); code != http.StatusOK {
		t.Fatalf("缺少 notify 时应使用默认值保存 %d %s", code, body)
	}
	if got := e.api.currentSettings().Notify; got != store.DefaultNotifySettings() {
		t.Fatalf("通知设置应为默认值 %+v", got)
	}
	old := gin.H{"version": 1, "settings": gin.H{"site_title": "旧", "default_report_interval": 2}, "servers": []gin.H{}}
	if code, body := e.do("POST", "/api/admin/import", tok, old); code != http.StatusOK {
		t.Fatalf("旧导出文件应能导入 %d %s", code, body)
	}
	if got := e.api.currentSettings(); got.SiteTitle != "旧" || got.Notify != store.DefaultNotifySettings() {
		t.Fatalf("导入旧文件后设置错误 %+v", got)
	}
}

func TestImportChangedSecretKicksAgent(t *testing.T) {
	e := newTestEnv(t)
	tok := e.adminToken()
	s := createServer(t, e, tok, gin.H{"name": "a"})
	same := createServer(t, e, tok, gin.H{"name": "b"})
	conn, _, err := e.dialAgent(s.ID, s.Secret)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	sendMsg(t, conn, proto.TypeHello, proto.Hello{})
	readMsg(t, conn)
	keep, _, err := e.dialAgent(same.ID, same.Secret)
	if err != nil {
		t.Fatal(err)
	}
	defer keep.CloseNow()
	sendMsg(t, keep, proto.TypeHello, proto.Hello{})
	readMsg(t, keep)

	payload := gin.H{"version": 1, "settings": store.DefaultSettings(), "servers": []gin.H{
		{"id": s.ID, "secret": "new-secret-value", "name": "a"},
		{"id": same.ID, "secret": same.Secret, "name": "b2"},
	}}
	if code, body := e.do("POST", "/api/admin/import", tok, payload); code != http.StatusOK {
		t.Fatalf("导入失败: %d %s", code, body)
	}
	expectClosed(t, conn, "secret 被覆盖后旧连接应被断开")
	// secret 未变的服务器保持连接，并收到新的参数
	if env := readMsg(t, keep); env.Type != proto.TypeConfig {
		t.Fatalf("secret 未变的连接应保持并收到 config，实际 %s", env.Type)
	}
}

func TestExportImport(t *testing.T) {
	src := newTestEnv(t)
	tok := src.adminToken()
	a := createServer(t, src, tok, gin.H{"name": "a", "price": 9.9, "traffic_limit": 1000})
	code, body := src.do("GET", "/api/admin/export", tok, nil)
	if code != http.StatusOK {
		t.Fatalf("导出失败: %d", code)
	}
	var wrapper struct {
		Data json.RawMessage `json:"data"`
	}
	_ = json.Unmarshal(body, &wrapper)
	if !strings.Contains(string(wrapper.Data), a.Secret) {
		t.Fatal("导出内容应包含 secret")
	}

	dst := newTestEnv(t)
	dtok := dst.adminToken()
	var payload map[string]any
	_ = json.Unmarshal(wrapper.Data, &payload)
	if code, body := dst.do("POST", "/api/admin/import", dtok, payload); code != http.StatusOK {
		t.Fatalf("导入失败: %d %s", code, body)
	}
	_, body = dst.do("GET", "/api/admin/servers", dtok, nil)
	list := decodeData[[]store.Server](t, body)
	if len(list) != 1 || list[0].ID != a.ID || list[0].Secret != a.Secret || list[0].Price == nil || *list[0].Price != 9.9 {
		t.Fatalf("导入结果错误: %+v", list)
	}

	payload["version"] = 2
	if code, body := dst.do("POST", "/api/admin/import", dtok, payload); code != http.StatusBadRequest || errorCode(t, body) != "bad_version" {
		t.Fatalf("不支持的版本应返回 400 bad_version，实际 %d %s", code, body)
	}
}

func TestQueryTokenOnlyForPublicWS(t *testing.T) {
	e := newTestEnv(t)
	e.addServer(store.Server{Name: "hid", Hidden: true})
	tok := e.adminToken()
	if code, _ := e.do("GET", "/api/auth/me?token="+tok, "", nil); code != http.StatusUnauthorized {
		t.Fatalf("普通接口不应接受 query token，实际 %d", code)
	}
	_, body := e.do("GET", "/api/public/servers?token="+tok, "", nil)
	if list := decodeData[[]store.Server](t, body); len(list) != 0 {
		t.Fatalf("公开接口不应通过 query token 登录: %+v", list)
	}
}

func TestPanicReturnsErrorEnvelope(t *testing.T) {
	e := newTestEnv(t)
	eng, err := e.api.Router(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	eng.GET("/panic", func(*gin.Context) { panic("boom") })
	w := httptest.NewRecorder()
	eng.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/panic", nil))
	if w.Code != http.StatusInternalServerError || errorCode(t, w.Body.Bytes()) != "internal" {
		t.Fatalf("panic 应返回 500 错误信封，实际 %d %s", w.Code, w.Body.String())
	}
}

func TestImportRejectsOversizedBody(t *testing.T) {
	e := newTestEnv(t)
	tok := e.adminToken()
	big := `{"version":1,"servers":[],"pad":"` + strings.Repeat("x", 10<<20) + `"}`
	req, _ := http.NewRequest("POST", e.srv.URL+"/api/admin/import", strings.NewReader(big))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusBadRequest || errorCode(t, body) != "bad_request" {
		t.Fatalf("超过 10MB 的导入请求应返回 400 bad_request，实际 %d %s", resp.StatusCode, body)
	}
}

func TestInstallCommandsPinVersion(t *testing.T) {
	const custom = "https://mirror.example.com/sss"
	cases := []struct {
		name, base, version, wantScript, wantLinuxVer, wantWinVer string
	}{
		{"开发版使用 latest", store.DefaultInstallScriptBase, "dev", store.DefaultInstallScriptBase, "", ""},
		{"未设置版本使用 latest", store.DefaultInstallScriptBase, "", store.DefaultInstallScriptBase, "", ""},
		{"git describe 版本视为开发版", store.DefaultInstallScriptBase, "v2.0.0-beta.1-3-gabc123", store.DefaultInstallScriptBase, "", ""},
		{"发布版锁定默认地址与版本", store.DefaultInstallScriptBase, "2.0.0-beta.1",
			store.ReleaseDownloadBase + "v2.0.0-beta.1", " --version 'v2.0.0-beta.1'", " -Version 'v2.0.0-beta.1'"},
		{"自定义地址保留，仅追加版本", custom, "v2.1.0", custom, " --version 'v2.1.0'", " -Version 'v2.1.0'"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := installCommands(tc.base, tc.version, "https://s.example.com", "id1", "sec1")
			wantLinux := "curl -fsSL '" + tc.wantScript + "/install-agent.sh' | sudo bash -s -- --dashboard 'https://s.example.com' --id 'id1' --secret 'sec1'" + tc.wantLinuxVer
			wantWin := `Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass -Force; $f="$env:TEMP\install-agent.ps1"; iwr -useb '` +
				tc.wantScript + "/install-agent.ps1' -OutFile $f; & $f -Dashboard 'https://s.example.com' -Id 'id1' -Secret 'sec1'" + tc.wantWinVer
			if got["linux"] != wantLinux {
				t.Errorf("linux 命令\n得到 %v\n期望 %s", got["linux"], wantLinux)
			}
			if got["windows"] != wantWin {
				t.Errorf("windows 命令\n得到 %v\n期望 %s", got["windows"], wantWin)
			}
		})
	}
}

func TestAdminListLastSeenUsesLiveReport(t *testing.T) {
	e := newTestEnv(t)
	on := e.addServer(store.Server{Name: "on"})
	off := e.addServer(store.Server{Name: "off"})
	if err := e.st.SetLastSeen(context.Background(), off.ID, 1000); err != nil {
		t.Fatal(err)
	}
	if err := e.api.reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	e.api.Hub.Connect(on.ID, 2)
	e.api.handleReport(context.Background(), on.ID, proto.Report{})

	code, body := e.do("GET", "/api/admin/servers", e.adminToken(), nil)
	if code != http.StatusOK {
		t.Fatalf("状态码 %d", code)
	}
	got := map[string]int64{}
	for _, s := range decodeData[[]struct {
		ID       string `json:"id"`
		LastSeen int64  `json:"last_seen"`
	}](t, body) {
		got[s.ID] = s.LastSeen
	}
	if got[on.ID] != e.clock.Now().Unix() {
		t.Fatalf("在线服务器应返回实时上报时间，得到 %d", got[on.ID])
	}
	if got[off.ID] != 1000 {
		t.Fatalf("离线服务器应返回数据库中的最后在线时间，得到 %d", got[off.ID])
	}
}

func TestAdminServerNoteAndMuted(t *testing.T) {
	e := newTestEnv(t)
	tok := e.adminToken()
	s := createServer(t, e, tok, gin.H{"name": "n1", "note": "  购买于 A 商家  ", "notify_muted": true})
	if s.Note != "购买于 A 商家" || !s.NotifyMuted {
		t.Fatalf("新字段未保存或未去除首尾空白: %+v", s)
	}
	long := strings.Repeat("字", 2001)
	if code, body := e.do("POST", "/api/admin/servers", tok, gin.H{"name": "n2", "note": long}); code != http.StatusBadRequest || errorCode(t, body) != "invalid_input" {
		t.Fatalf("超长备注应返回 invalid_input，实际 %d %s", code, body)
	}
	code, body := e.do("GET", "/api/public/servers", "", nil)
	if code != http.StatusOK || strings.Contains(string(body), "购买于") {
		t.Fatalf("公开接口不应包含备注: %d %s", code, body)
	}
}

func TestAdminNotifyTest(t *testing.T) {
	e := newTestEnv(t)
	tok := e.adminToken()
	var hits atomic.Int32
	hook := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer hook.Close()
	body := gin.H{"webhook_url": hook.URL, "lang": "zh-CN", "offline_minutes": 3, "expire_days": 7, "traffic_percent": 90}
	code, b := e.do("POST", "/api/admin/notify/test", tok, body)
	res := decodeData[map[string]*string](t, b)
	if code != http.StatusOK || res["webhook"] == nil || *res["webhook"] != "ok" || res["telegram"] != nil || hits.Load() != 1 {
		t.Fatalf("%d %s", code, b)
	}
	body["webhook_url"] = ""
	if code, b := e.do("POST", "/api/admin/notify/test", tok, body); code != http.StatusBadRequest || errorCode(t, b) != "invalid_input" {
		t.Fatalf("未配置渠道应返回 invalid_input，实际 %d %s", code, b)
	}
}

func TestAdminListAgentVersion(t *testing.T) {
	e := newTestEnv(t)
	e.api.Version = "2.0.0-beta.2"
	old := e.addServer(store.Server{Name: "old"})
	cur := e.addServer(store.Server{Name: "cur"})
	none := e.addServer(store.Server{Name: "none"})
	e.api.Hub.SetStatic(old.ID, proto.Hello{AgentVersion: "2.0.0-beta.1"})
	e.api.Hub.SetStatic(cur.ID, proto.Hello{AgentVersion: "2.0.0-beta.2"})
	code, body := e.do("GET", "/api/admin/servers", e.adminToken(), nil)
	if code != http.StatusOK {
		t.Fatal(code)
	}
	got := map[string][2]any{}
	for _, s := range decodeData[[]struct {
		ID           string `json:"id"`
		AgentVersion string `json:"agent_version"`
		Outdated     bool   `json:"outdated"`
	}](t, body) {
		got[s.ID] = [2]any{s.AgentVersion, s.Outdated}
	}
	if got[old.ID] != [2]any{"2.0.0-beta.1", true} || got[cur.ID] != [2]any{"2.0.0-beta.2", false} || got[none.ID] != [2]any{"", false} {
		t.Fatalf("版本信息错误 %+v", got)
	}
}
