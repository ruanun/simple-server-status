package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/ruanun/simple-server-status/internal/dashboard/store"
)

type wsClient struct {
	authed bool
	uid    int64     // 登录用户 ID（仅 authed）
	tv     int       // 登录时的 token 版本（仅 authed）
	exp    time.Time // token 过期时间（仅 authed）
	send   chan []byte
	cancel context.CancelFunc
}

type wsMessage struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

// broadcaster 每 2 秒向浏览器推送有变化的服务器
type broadcaster struct {
	a          *API
	mu         sync.Mutex
	closed     bool
	clients    map[*wsClient]struct{}
	lastSeq    uint64
	lastOnline map[string]bool
	resnap     atomic.Bool
	ticks      int // tick 计数，用于定期校验登录态（仅 tick 使用）
}

// verifyEvery 每隔多少次 tick 校验一次已登录客户端的凭证
const verifyEvery = 5

func newBroadcaster(a *API) *broadcaster {
	return &broadcaster{a: a, clients: map[*wsClient]struct{}{}, lastOnline: map[string]bool{}}
}

// add 登记客户端；已关闭（正在退出）时返回 false
func (b *broadcaster) add(c *wsClient) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return false
	}
	b.clients[c] = struct{}{}
	return true
}

func (b *broadcaster) remove(c *wsClient) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.clients, c)
}

// drop 移除并断开客户端（handler 退出时的 remove 可重复调用）
func (b *broadcaster) drop(c *wsClient) {
	b.mu.Lock()
	delete(b.clients, c)
	b.mu.Unlock()
	c.cancel()
}

// kickUser 断开某用户的全部已登录客户端（改密码后调用）
func (b *broadcaster) kickUser(uid int64) {
	for _, c := range b.snapshotClients() {
		if c.authed && c.uid == uid {
			b.drop(c)
		}
	}
}

// verifyAuthed 校验已登录客户端的凭证：token 已过期或版本与数据库不一致时断开，
// 客户端重连后按新凭证获得相应视图；数据库暂时出错时保留连接
func (b *broadcaster) verifyAuthed(ctx context.Context) {
	now := b.a.Now()
	versions := map[int64]int{}
	for _, c := range b.snapshotClients() {
		if !c.authed {
			continue
		}
		if !now.Before(c.exp) {
			b.drop(c)
			continue
		}
		tv, ok := versions[c.uid]
		if !ok {
			u, err := b.a.Store.GetUser(ctx, c.uid)
			switch {
			case errors.Is(err, store.ErrNotFound):
				tv = -1
			case err != nil:
				b.a.Log.Warn("校验浏览器连接登录态失败", "err", err)
				continue
			default:
				tv = u.TokenVersion
			}
			versions[c.uid] = tv
		}
		if tv != c.tv {
			b.drop(c)
		}
	}
}

func (b *broadcaster) closeAll() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	for c := range b.clients {
		c.cancel()
	}
}

// requestSnapshot 服务器列表或设置变化后，下一次推送改为完整快照
func (b *broadcaster) requestSnapshot() { b.resnap.Store(true) }

func (b *broadcaster) snapshotClients() []*wsClient {
	b.mu.Lock()
	defer b.mu.Unlock()
	list := make([]*wsClient, 0, len(b.clients))
	for c := range b.clients {
		list = append(list, c)
	}
	return list
}

func (b *broadcaster) encode(typ string, data any) []byte {
	msg, err := json.Marshal(wsMessage{Type: typ, Data: data})
	if err != nil {
		b.a.Log.Error("编码推送消息失败", "err", err)
		return nil
	}
	return msg
}

// push 非阻塞发送；客户端积压时断开，由其重连后重新获取快照
func push(c *wsClient, msg []byte) {
	if msg == nil {
		return
	}
	select {
	case c.send <- msg:
	default:
		c.cancel()
	}
}

// tick 计算变化并推送（仅由 run 或测试调用，不并发）
func (b *broadcaster) tick(ctx context.Context) {
	b.ticks++
	if b.ticks%verifyEvery == 0 {
		b.verifyAuthed(ctx)
	}
	ids, seq := b.a.Hub.Changed(b.lastSeq)
	b.lastSeq = seq
	changed := make(map[string]bool, len(ids))
	for _, id := range ids {
		changed[id] = true
	}
	servers := b.a.serverList()
	// 超时离线不会产生序号变化，需要单独比较
	for _, srv := range servers {
		on := b.a.Hub.Get(srv.ID).Online
		if b.lastOnline[srv.ID] != on {
			b.lastOnline[srv.ID] = on
			changed[srv.ID] = true
		}
	}
	clients := b.snapshotClients()
	full := b.resnap.Swap(false)
	if len(clients) == 0 {
		return
	}
	if full {
		pub := b.encode("snapshot", b.a.views(ctx, false))
		all := b.encode("snapshot", b.a.views(ctx, true))
		for _, c := range clients {
			if c.authed {
				push(c, all)
			} else {
				push(c, pub)
			}
		}
		return
	}
	if len(changed) == 0 {
		return
	}
	showPrice := b.a.currentSettings().ShowPrice
	pubList, allList := []ServerView{}, []ServerView{}
	for _, srv := range servers {
		if !changed[srv.ID] {
			continue
		}
		allList = append(allList, b.a.view(ctx, srv, true))
		if !srv.Hidden {
			pubList = append(pubList, b.a.view(ctx, srv, showPrice))
		}
	}
	if len(allList) == 0 {
		return
	}
	all := b.encode("delta", allList)
	var pub []byte
	if len(pubList) > 0 {
		pub = b.encode("delta", pubList)
	}
	for _, c := range clients {
		if c.authed {
			push(c, all)
		} else {
			push(c, pub)
		}
	}
}

func (b *broadcaster) run(ctx context.Context) {
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			b.tick(ctx)
		}
	}
}

// RunBroadcaster 启动浏览器推送循环，直到 ctx 结束
func (a *API) RunBroadcaster(ctx context.Context) { a.bc.run(ctx) }

// publicWS 浏览器 WebSocket：连接后先推快照，之后接收 delta
func (a *API) publicWS(c *gin.Context) {
	if !a.conns.enter() {
		fail(c, http.StatusServiceUnavailable, "shutting_down", "服务正在停止")
		return
	}
	defer a.conns.leave()
	// 跳过同源校验：推送的公开数据本就公开；登录态通过 query token 传递，浏览器不会自动附带，
	// 跨源页面无法借用户身份获取隐藏数据，因此同源校验没有安全价值，反而会导致反向代理部署失败
	conn, err := websocket.Accept(c.Writer, c.Request, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		a.Log.Debug("浏览器 WebSocket 握手失败", "err", err)
		return
	}
	defer func() { _ = conn.CloseNow() }()
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()
	ctx = conn.CloseRead(ctx)

	cl := &wsClient{send: make(chan []byte, 16), cancel: cancel}
	if claims, ok := currentClaims(c); ok && isAuthed(c) && claims.ExpiresAt != nil {
		cl.authed, cl.uid, cl.tv, cl.exp = true, claims.UID, claims.TV, claims.ExpiresAt.Time
	}
	if snap := a.bc.encode("snapshot", a.views(ctx, cl.authed)); snap != nil {
		cl.send <- snap
	}
	if !a.bc.add(cl) {
		_ = conn.Close(websocket.StatusGoingAway, "服务正在停止")
		return
	}
	defer a.bc.remove(cl)

	ping := time.NewTicker(30 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-ctx.Done():
			_ = conn.Close(websocket.StatusNormalClosure, "")
			return
		case <-ping.C:
			pctx, pcancel := context.WithTimeout(ctx, 10*time.Second)
			err := conn.Ping(pctx)
			pcancel()
			if err != nil {
				return
			}
		case msg := <-cl.send:
			if ctx.Err() != nil { // 已被断开（如登录态失效）时不再发送积压消息
				return
			}
			wctx, wcancel := context.WithTimeout(ctx, 10*time.Second)
			err := conn.Write(wctx, websocket.MessageText, msg)
			wcancel()
			if err != nil {
				return
			}
		}
	}
}
