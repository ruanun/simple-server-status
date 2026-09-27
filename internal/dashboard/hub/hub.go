// Package hub 在内存中维护各服务器的实时状态。
package hub

import (
	"sync"
	"time"

	"github.com/ruanun/simple-server-status/internal/dashboard/metric"
	"github.com/ruanun/simple-server-status/internal/proto"
)

// ringWindow 秒级环形缓冲保留时长
const ringWindow = 10 * time.Minute

// Live 某台服务器的实时状态
type Live struct {
	Online     bool
	Connected  bool
	LastReport int64 // unix 秒，0 表示本次运行中尚未上报
	Report     *proto.Report
	Static     *proto.Hello
}

type state struct {
	session   uint64
	connected bool
	interval  int
	last      time.Time
	report    *proto.Report
	static    *proto.Hello
	ring      []metric.Point
	seq       uint64
}

// Hub 实时状态中心，并发安全
type Hub struct {
	mu      sync.RWMutex
	now     func() time.Time
	nextSes uint64
	seq     uint64
	servers map[string]*state
}

// New 创建 Hub
func New(now func() time.Time) *Hub {
	return &Hub{now: now, servers: map[string]*state{}}
}

func (h *Hub) get(id string) *state {
	s := h.servers[id]
	if s == nil {
		s = &state{interval: 2}
		h.servers[id] = s
	}
	return s
}

func (h *Hub) bump(s *state) {
	h.seq++
	s.seq = h.seq
}

// Connect 记录新连接，返回会话号
func (h *Hub) Connect(id string, interval int) uint64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	s := h.get(id)
	h.nextSes++
	s.session = h.nextSes
	s.connected = true
	s.interval = interval
	h.bump(s)
	return s.session
}

// Disconnect 仅当会话号匹配时标记断开，避免旧连接的退出影响新连接
func (h *Hub) Disconnect(id string, session uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s := h.servers[id]
	if s == nil || s.session != session {
		return
	}
	s.connected = false
	h.bump(s)
}

// SetInterval 更新上报间隔（影响在线判定）
func (h *Hub) SetInterval(id string, interval int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.get(id).interval = interval
}

// SetStatic 更新静态信息
func (h *Hub) SetStatic(id string, hello proto.Hello) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s := h.get(id)
	s.static = &hello
	h.bump(s)
}

// Report 记录一次上报（时间戳以服务端为准），返回对应的指标点
func (h *Hub) Report(id string, r proto.Report) metric.Point {
	h.mu.Lock()
	defer h.mu.Unlock()
	s := h.get(id)
	now := h.now()
	r.TS = now.Unix()
	s.last = now
	s.report = &r
	var memTotal uint64
	if s.static != nil {
		memTotal = s.static.MemTotal
	}
	p := metric.FromReport(r, memTotal)
	s.ring = append(s.ring, p)
	cut := now.Add(-ringWindow).Unix()
	i := 0
	for i < len(s.ring) && s.ring[i].TS < cut {
		i++
	}
	s.ring = s.ring[i:]
	h.bump(s)
	return p
}

func (h *Hub) online(s *state, now time.Time) bool {
	if !s.connected || s.last.IsZero() {
		return false
	}
	limit := time.Duration(3*s.interval) * time.Second
	if limit < 6*time.Second {
		limit = 6 * time.Second
	}
	return now.Sub(s.last) <= limit
}

// Get 返回实时状态；未知服务器返回零值
func (h *Hub) Get(id string) Live {
	h.mu.RLock()
	defer h.mu.RUnlock()
	s := h.servers[id]
	if s == nil {
		return Live{}
	}
	l := Live{Online: h.online(s, h.now()), Connected: s.connected, Report: s.report, Static: s.static}
	if !s.last.IsZero() {
		l.LastReport = s.last.Unix()
	}
	return l
}

// Ring 返回最近 10 分钟的秒级指标点副本
func (h *Hub) Ring(id string) []metric.Point {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := []metric.Point{}
	if s := h.servers[id]; s != nil {
		out = append(out, s.ring...)
	}
	return out
}

// Remove 删除服务器的全部实时状态
func (h *Hub) Remove(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.servers, id)
	h.seq++
}

// Changed 返回序号大于 since 的服务器 ID，以及当前序号
func (h *Hub) Changed(since uint64) ([]string, uint64) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	ids := []string{}
	for id, s := range h.servers {
		if s.seq > since {
			ids = append(ids, id)
		}
	}
	return ids, h.seq
}
