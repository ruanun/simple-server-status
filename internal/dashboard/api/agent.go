package api

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/ruanun/simple-server-status/internal/dashboard/store"
	"github.com/ruanun/simple-server-status/internal/proto"
)

type agentConn struct {
	id      string
	send    chan []byte
	cancel  context.CancelFunc
	session uint64 // Hub 会话号，由 add 在锁内设置
}

// agentRegistry 当前在线的 Agent 连接，每台服务器最多一条
type agentRegistry struct {
	mu     sync.Mutex
	closed bool
	conns  map[string]*agentConn
}

func newAgentRegistry() *agentRegistry {
	return &agentRegistry{conns: map[string]*agentConn{}}
}

// add 注册新连接，并断开同一服务器的旧连接；connect 在同一把锁内执行（Hub.Connect），
// 保证 registry 中的当前连接与 Hub 会话号一致。已关闭（正在退出）时返回 false
func (r *agentRegistry) add(c *agentConn, connect func() uint64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return false
	}
	if old := r.conns[c.id]; old != nil {
		old.cancel()
	}
	c.session = connect()
	r.conns[c.id] = c
	return true
}

// remove 仅当仍是当前连接时移除
func (r *agentRegistry) remove(c *agentConn) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.conns[c.id] == c {
		delete(r.conns, c.id)
	}
}

// send 非阻塞发送；无连接或缓冲已满时返回 false
func (r *agentRegistry) send(id string, msg []byte) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	c := r.conns[id]
	if c == nil {
		return false
	}
	select {
	case c.send <- msg:
		return true
	default:
		return false
	}
}

func (r *agentRegistry) kick(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c := r.conns[id]; c != nil {
		c.cancel()
	}
}

func (r *agentRegistry) closeAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	for _, c := range r.conns {
		c.cancel()
	}
}

// parseAgentAuth 解析 "Bearer <id>:<secret>"
func parseAgentAuth(h string) (id, secret string, ok bool) {
	rest, found := strings.CutPrefix(h, "Bearer ")
	if !found {
		return "", "", false
	}
	id, secret, found = strings.Cut(rest, ":")
	return id, secret, found && id != "" && secret != ""
}

// agentWS Agent 的 WebSocket 入口
func (a *API) agentWS(c *gin.Context) {
	if !a.conns.enter() {
		fail(c, http.StatusServiceUnavailable, "shutting_down", "服务正在停止")
		return
	}
	defer a.conns.leave()
	id, secret, okAuth := parseAgentAuth(c.GetHeader("Authorization"))
	srv, found := a.server(id)
	if !okAuth || !found || subtle.ConstantTimeCompare([]byte(secret), []byte(srv.Secret)) != 1 {
		fail(c, http.StatusUnauthorized, "unauthorized", "Agent 鉴权失败")
		return
	}
	conn, err := websocket.Accept(c.Writer, c.Request, nil)
	if err != nil {
		a.Log.Warn("Agent 握手失败", "id", id, "err", err)
		return
	}
	conn.SetReadLimit(64 << 10)

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()
	ac := &agentConn{id: id, send: make(chan []byte, 8), cancel: cancel}
	if !a.agents.add(ac, func() uint64 { return a.Hub.Connect(id, srv.ReportInterval) }) {
		_ = conn.Close(websocket.StatusGoingAway, "服务正在停止")
		return
	}
	ip := c.ClientIP()
	a.Log.Info("Agent 已连接", "id", id, "name", srv.Name, "ip", ip)

	defer func() {
		a.agents.remove(ac)
		a.Hub.Disconnect(id, ac.session)
		_ = conn.CloseNow()
		sctx, scancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer scancel()
		a.saveLastSeen(sctx, id, a.Now().Unix())
		if err := a.Traffic.Flush(sctx); err != nil {
			a.Log.Warn("保存流量数据失败", "err", err)
		}
		a.Log.Info("Agent 已断开", "id", id)
	}()

	go a.agentWriter(ctx, conn, ac)
	go a.agentPinger(ctx, conn, ac)
	a.agentReadLoop(ctx, conn, id, ip)
}

func (a *API) agentWriter(ctx context.Context, conn *websocket.Conn, ac *agentConn) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-ac.send:
			wctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := conn.Write(wctx, websocket.MessageText, msg)
			cancel()
			if err != nil {
				ac.cancel()
				return
			}
		}
	}
}

// agentPinger 每 20 秒 ping 一次，20 秒内无 pong 则断开
func (a *API) agentPinger(ctx context.Context, conn *websocket.Conn, ac *agentConn) {
	t := time.NewTicker(20 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			pctx, cancel := context.WithTimeout(ctx, 20*time.Second)
			err := conn.Ping(pctx)
			cancel()
			if err != nil {
				ac.cancel()
				return
			}
		}
	}
}

func (a *API) agentReadLoop(ctx context.Context, conn *websocket.Conn, id, ip string) {
	for {
		_, b, err := conn.Read(ctx)
		if err != nil {
			return
		}
		env, err := proto.Decode(b)
		if errors.Is(err, proto.ErrVersion) {
			_ = conn.Close(websocket.StatusPolicyViolation, "不支持的协议版本")
			return
		}
		if err != nil {
			a.Log.Warn("忽略无法解析的 Agent 消息", "id", id, "err", err)
			continue
		}
		switch env.Type {
		case proto.TypeHello:
			var h proto.Hello
			if err := json.Unmarshal(env.Data, &h); err != nil {
				a.Log.Warn("忽略无效的 hello", "id", id, "err", err)
				continue
			}
			a.handleHello(ctx, id, ip, h)
		case proto.TypeReport:
			var r proto.Report
			if err := json.Unmarshal(env.Data, &r); err != nil {
				a.Log.Warn("忽略无效的 report", "id", id, "err", err)
				continue
			}
			a.handleReport(ctx, id, r)
		default:
			a.Log.Debug("忽略未知 Agent 消息", "id", id, "type", env.Type)
		}
	}
}

func (a *API) handleHello(ctx context.Context, id, ip string, h proto.Hello) {
	if _, ok := a.server(id); !ok {
		return
	}
	a.Hub.SetStatic(id, h)
	if err := a.Store.SetStaticInfo(ctx, id, ip, h); err != nil {
		a.Log.Warn("保存静态信息失败", "id", id, "err", err)
	} else {
		a.updateCached(id, func(s *store.Server) {
			s.StaticInfo = &h
			s.LastIP = ip
		})
	}
	a.pushConfig(id)
}

// nicFilterID 计算网卡过滤参数的稳定摘要（规范化 JSON 的 sha256 前 12 位 hex）；
// 只包含影响流量统计口径的 NIC 字段，mount_exclude 不参与
func nicFilterID(include, exclude []string) string {
	if include == nil {
		include = []string{}
	}
	if exclude == nil {
		exclude = []string{}
	}
	b, _ := json.Marshal(struct {
		Include []string `json:"nic_include"`
		Exclude []string `json:"nic_exclude"`
	}{include, exclude})
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])[:12]
}

// pushConfig 向在线的 Agent 推送当前采集参数
func (a *API) pushConfig(id string) {
	srv, ok := a.server(id)
	if !ok {
		return
	}
	cfg := proto.Config{ReportInterval: srv.ReportInterval, NICInclude: srv.NICInclude, NICExclude: srv.NICExclude, MountExclude: srv.MountExclude}
	cfg.Normalize()
	cfg.FilterID = nicFilterID(cfg.NICInclude, cfg.NICExclude)
	b, err := proto.Encode(proto.TypeConfig, cfg)
	if err != nil {
		a.Log.Error("编码 config 失败", "err", err)
		return
	}
	a.Hub.SetInterval(id, srv.ReportInterval)
	a.agents.send(id, b)
}

// handleReport 处理一次上报：更新实时状态、历史与流量
func (a *API) handleReport(ctx context.Context, id string, r proto.Report) {
	srv, ok := a.server(id)
	if !ok {
		return
	}
	r.Normalize()
	p := a.Hub.Report(id, r)
	a.History.Add(id, p)
	if err := a.Traffic.Add(ctx, id, srv.TrafficResetDay, r.FilterID, r.NetInTotal, r.NetOutTotal); err != nil {
		a.Log.Warn("累计流量失败", "id", id, "err", err)
	}
}
