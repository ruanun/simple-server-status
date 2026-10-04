package api

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/ruanun/simple-server-status/internal/dashboard/hub"
	"github.com/ruanun/simple-server-status/internal/dashboard/incident"
	"github.com/ruanun/simple-server-status/internal/dashboard/store"
	"github.com/ruanun/simple-server-status/internal/proto"
)

func TestAgentRejectsBadSecret(t *testing.T) {
	e := newTestEnv(t)
	s := e.addServer(store.Server{Name: "a"})
	_, resp, err := e.dialAgent(s.ID, "wrong")
	if err == nil || resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("错误密钥应返回 401，实际 err=%v resp=%v", err, resp)
	}
	_, resp, _ = e.dialAgent("missing", "x")
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatal("不存在的服务器应返回 401")
	}
}

func TestAgentHelloConfigReport(t *testing.T) {
	e := newTestEnv(t)
	s := e.addServer(store.Server{Name: "a", ReportInterval: 3, NICInclude: []string{"eth"}})
	conn, _, err := e.dialAgent(s.ID, s.Secret)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()

	sendMsg(t, conn, proto.TypeHello, proto.Hello{MemTotal: 1000, Country: "JP"})
	env := readMsg(t, conn)
	var cfg proto.Config
	_ = json.Unmarshal(env.Data, &cfg)
	if env.Type != proto.TypeConfig || cfg.ReportInterval != 3 || len(cfg.NICInclude) != 1 || cfg.MountExclude == nil {
		t.Fatalf("config 错误: %s %+v", env.Type, cfg)
	}

	sendMsg(t, conn, proto.TypeReport, proto.Report{CPU: 42, MemUsed: 500, NetInTotal: 100, NetOutTotal: 100})
	sendMsg(t, conn, proto.TypeReport, proto.Report{CPU: 43, MemUsed: 500, NetInTotal: 300, NetOutTotal: 150})
	waitFor(t, func() bool {
		l := e.api.Hub.Get(s.ID)
		return l.Online && l.Report != nil && l.Report.NetInTotal == 300
	})
	in, out, _, _ := e.api.Traffic.Usage(context.Background(), s.ID, 1)
	if in != 200 || out != 50 {
		t.Fatalf("流量累计错误: %d %d", in, out)
	}
	got, _ := e.st.GetServer(context.Background(), s.ID)
	if got.StaticInfo == nil || got.StaticInfo.Country != "JP" || got.LastIP == "" {
		t.Fatalf("静态信息未保存: %+v", got)
	}
}

func TestAgentNewConnectionKicksOld(t *testing.T) {
	e := newTestEnv(t)
	s := e.addServer(store.Server{Name: "a"})
	c1, _, err := e.dialAgent(s.ID, s.Secret)
	if err != nil {
		t.Fatal(err)
	}
	defer c1.CloseNow()
	c2, _, err := e.dialAgent(s.ID, s.Secret)
	if err != nil {
		t.Fatal(err)
	}
	defer c2.CloseNow()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, _, err := c1.Read(ctx); err == nil {
		t.Fatal("旧连接应被断开")
	}
	sendMsg(t, c2, proto.TypeHello, proto.Hello{})
	if env := readMsg(t, c2); env.Type != proto.TypeConfig {
		t.Fatalf("新连接应正常工作，收到 %s", env.Type)
	}
	sendMsg(t, c2, proto.TypeReport, proto.Report{})
	waitFor(t, func() bool { return e.api.Hub.Get(s.ID).Online })
}

func TestAgentBadVersionClosed(t *testing.T) {
	e := newTestEnv(t)
	s := e.addServer(store.Server{Name: "a"})
	conn, _, err := e.dialAgent(s.ID, s.Secret)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = conn.Write(ctx, websocket.MessageText, []byte(`{"v":99,"type":"hello","data":{}}`))
	_, _, err = conn.Read(ctx)
	if websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
		t.Fatalf("协议版本不符应以 PolicyViolation 关闭，实际 %v", err)
	}
}

func TestAgentDisconnectMarksOffline(t *testing.T) {
	e := newTestEnv(t)
	s := e.addServer(store.Server{Name: "a"})
	conn, _, err := e.dialAgent(s.ID, s.Secret)
	if err != nil {
		t.Fatal(err)
	}
	sendMsg(t, conn, proto.TypeHello, proto.Hello{})
	readMsg(t, conn)
	sendMsg(t, conn, proto.TypeReport, proto.Report{})
	waitFor(t, func() bool { return e.api.Hub.Get(s.ID).Online })
	_ = conn.Close(websocket.StatusNormalClosure, "")
	waitFor(t, func() bool { return !e.api.Hub.Get(s.ID).Connected })
	waitFor(t, func() bool {
		got, _ := e.st.GetServer(context.Background(), s.ID)
		return got.LastSeen > 0
	})
}

func TestNICFilterID(t *testing.T) {
	base := nicFilterID([]string{"eth"}, nil)
	if len(base) != 12 || base != nicFilterID([]string{"eth"}, []string{}) {
		t.Fatalf("摘要应为 12 位且 nil 与空切片一致: %q", base)
	}
	if base == nicFilterID([]string{"ens"}, nil) || base == nicFilterID(nil, []string{"eth"}) {
		t.Fatal("网卡参数不同时摘要应不同")
	}
}

func TestAgentConfigFilterIDAndTrafficRebaseline(t *testing.T) {
	e := newTestEnv(t)
	tok := e.adminToken()
	s := createServer(t, e, tok, map[string]any{"name": "a", "nic_include": []string{"eth"}})
	conn, _, err := e.dialAgent(s.ID, s.Secret)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	sendMsg(t, conn, proto.TypeHello, proto.Hello{})
	var cfg proto.Config
	_ = json.Unmarshal(readMsg(t, conn).Data, &cfg)
	if cfg.FilterID != nicFilterID([]string{"eth"}, nil) {
		t.Fatalf("config 应携带网卡过滤摘要: %+v", cfg)
	}
	sendMsg(t, conn, proto.TypeReport, proto.Report{NetInTotal: 1000, NetOutTotal: 1000, FilterID: cfg.FilterID})
	sendMsg(t, conn, proto.TypeReport, proto.Report{NetInTotal: 1500, NetOutTotal: 1200, FilterID: cfg.FilterID})

	// 只改 mount_exclude 不影响摘要
	e.do("PUT", "/api/admin/servers/"+s.ID, tok, map[string]any{"name": "a", "nic_include": []string{"eth"}, "mount_exclude": []string{"/boot"}})
	var cfg2 proto.Config
	_ = json.Unmarshal(readMsg(t, conn).Data, &cfg2)
	if cfg2.FilterID != cfg.FilterID {
		t.Fatalf("mount_exclude 不应影响摘要: %s %s", cfg.FilterID, cfg2.FilterID)
	}
	// 改网卡过滤后，新口径的计数只重建基线
	e.do("PUT", "/api/admin/servers/"+s.ID, tok, map[string]any{"name": "a", "nic_include": []string{"ens"}})
	var cfg3 proto.Config
	_ = json.Unmarshal(readMsg(t, conn).Data, &cfg3)
	if cfg3.FilterID == cfg.FilterID {
		t.Fatal("网卡过滤变化后摘要应变化")
	}
	sendMsg(t, conn, proto.TypeReport, proto.Report{NetInTotal: 100, NetOutTotal: 100, FilterID: cfg3.FilterID})
	sendMsg(t, conn, proto.TypeReport, proto.Report{NetInTotal: 130, NetOutTotal: 110, FilterID: cfg3.FilterID})
	var in, out int64
	waitFor(t, func() bool {
		in, out, _, _ = e.api.Traffic.Usage(context.Background(), s.ID, 1)
		return in >= 530
	})
	if in != 530 || out != 210 {
		t.Fatalf("过滤器变化后流量应只按新基线累加: %d %d", in, out)
	}
}

func TestShutdownWaitsForAgentCleanup(t *testing.T) {
	e := newTestEnv(t)
	s := e.addServer(store.Server{Name: "a"})
	conn, _, err := e.dialAgent(s.ID, s.Secret)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	sendMsg(t, conn, proto.TypeHello, proto.Hello{})
	readMsg(t, conn)
	sendMsg(t, conn, proto.TypeReport, proto.Report{})
	waitFor(t, func() bool { return e.api.Hub.Get(s.ID).Online })
	e.api.Shutdown()
	got, _ := e.st.GetServer(context.Background(), s.ID)
	if got.LastSeen != e.clock.Now().Unix() {
		t.Fatalf("Shutdown 返回时应已写入最后在线时间，实际 %d", got.LastSeen)
	}
	if _, resp, _ := e.dialAgent(s.ID, s.Secret); resp != nil && resp.StatusCode == http.StatusSwitchingProtocols {
		t.Fatal("Shutdown 后不应再接受新的 Agent 连接")
	}
}

func TestTouchOnlineWritesLastSeen(t *testing.T) {
	e := newTestEnv(t)
	on := e.addServer(store.Server{Name: "on"})
	off := e.addServer(store.Server{Name: "off"})
	e.api.Hub.Connect(on.ID, 2)
	e.api.handleReport(context.Background(), on.ID, proto.Report{}, false)
	e.api.touchOnline(context.Background())
	got, _ := e.st.GetServer(context.Background(), on.ID)
	if got.LastSeen != e.clock.Now().Unix() {
		t.Fatalf("在线服务器应写入最后在线时间，实际 %d", got.LastSeen)
	}
	if cached, _ := e.api.server(on.ID); cached.LastSeen != e.clock.Now().Unix() {
		t.Fatalf("缓存中的最后在线时间未同步，实际 %d", cached.LastSeen)
	}
	if got, _ := e.st.GetServer(context.Background(), off.ID); got.LastSeen != 0 {
		t.Fatalf("离线服务器不应写入，实际 %d", got.LastSeen)
	}
}

func TestAgentRegistryAddConnectsHubAtomically(t *testing.T) {
	h := hub.New(time.Now)
	r := newAgentRegistry()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, cancel := context.WithCancel(context.Background())
			r.add(&agentConn{id: "s", send: make(chan []byte, 1), cancel: cancel}, func() uint64 { return h.Connect("s", 2) })
		}()
	}
	wg.Wait()
	cur := r.conns["s"]
	h.Disconnect("s", cur.session)
	if h.Get("s").Connected {
		t.Fatal("registry 中当前连接的会话号应与 Hub 一致")
	}
}

func TestAgentHelloUpdatesAdminList(t *testing.T) {
	e := newTestEnv(t)
	s := e.addServer(store.Server{Name: "a"})
	conn, _, err := e.dialAgent(s.ID, s.Secret)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	sendMsg(t, conn, proto.TypeHello, proto.Hello{OS: "linux", Country: "JP"})
	readMsg(t, conn) // config

	code, body := e.do("GET", "/api/admin/servers", e.adminToken(), nil)
	if code != http.StatusOK {
		t.Fatalf("状态码 %d", code)
	}
	list := decodeData[[]struct {
		ID         string       `json:"id"`
		LastIP     string       `json:"last_ip"`
		StaticInfo *proto.Hello `json:"static_info"`
	}](t, body)
	if len(list) != 1 || list[0].LastIP == "" || list[0].StaticInfo == nil || list[0].StaticInfo.Country != "JP" {
		t.Fatalf("后台列表未反映 Agent 上报的 IP 与静态信息: %+v", list)
	}
}

func TestRebootAndIPChangeEvents(t *testing.T) {
	e := newTestEnv(t)
	s := e.addServer(store.Server{Name: "a"})
	ctx := context.Background()
	events := func(kind string) []store.Event {
		list, _, _ := e.st.ListEvents(ctx, store.EventFilter{Kinds: []string{kind}}, 10, 0)
		return list
	}

	e.api.handleHello(ctx, s.ID, "9.9.9.9", proto.Hello{IPv4: "1.1.1.1"})
	e.api.handleHello(ctx, s.ID, "9.9.9.9", proto.Hello{IPv4: "1.1.1.1"})
	if len(events(incident.KindIPChange)) != 0 {
		t.Fatal("首次上报与未变化不应记录 IP 变化")
	}
	e.api.handleHello(ctx, s.ID, "9.9.9.9", proto.Hello{IPv4: "2.2.2.2"})
	if list := events(incident.KindIPChange); len(list) != 1 || string(list[0].Detail) != `{"ipv4":["1.1.1.1","2.2.2.2"]}` {
		t.Fatalf("应记录 IPv4 变化 %+v", list)
	}

	e.api.Hub.Connect(s.ID, 2)
	e.api.handleReport(ctx, s.ID, proto.Report{Uptime: 100}, true)
	if got, _ := e.st.GetServer(ctx, s.ID); got.BootAt != e.clock.Now().Unix()-100 || len(events(incident.KindReboot)) != 0 {
		t.Fatalf("首次只记录开机时间 %d", got.BootAt)
	}
	e.clock.Add(time.Hour)
	e.api.handleReport(ctx, s.ID, proto.Report{Uptime: 30}, false)
	if len(events(incident.KindReboot)) != 0 {
		t.Fatal("非连接后的第一次上报不判断重启")
	}
	e.api.handleReport(ctx, s.ID, proto.Report{Uptime: 60}, true)
	boot := e.clock.Now().Unix() - 60
	if list := events(incident.KindReboot); len(list) != 1 || list[0].StartAt != boot || list[0].EndAt == nil || *list[0].EndAt != boot {
		t.Fatalf("应记录重启，时间为开机时间 %+v", list)
	}
	if cached, _ := e.api.server(s.ID); cached.BootAt != boot {
		t.Fatalf("缓存中的开机时间未同步 %d", cached.BootAt)
	}
}
