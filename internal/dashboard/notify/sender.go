package notify

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// queueCap 发送队列容量，满时丢弃最旧的一条
const queueCap = 100

// defaultDrainWait 退出时发送剩余通知的默认最长时间
const defaultDrainWait = 5 * time.Second

type job struct {
	ch      Channel
	ev      Event
	attempt int
}

// Sender 异步发送队列：单个工作协程依次发送，失败按 RetryDelays 延迟后重新入队
type Sender struct {
	o         Options
	log       *slog.Logger
	mu        sync.Mutex
	queue     []job
	closed    bool
	wake      chan struct{}
	drainWait time.Duration // 退出时发送剩余通知的最长时间，默认 defaultDrainWait；测试可缩短
}

// NewSender 创建发送队列；log 为 nil 时使用 slog.Default()
func NewSender(o Options, log *slog.Logger) *Sender {
	if log == nil {
		log = slog.Default()
	}
	return &Sender{o: o.withDefaults(), log: log, wake: make(chan struct{}, 1), drainWait: defaultDrainWait}
}

// Enqueue 把事件加入每个渠道的发送队列
func (s *Sender) Enqueue(chs []Channel, e Event) {
	for _, ch := range chs {
		s.push(job{ch: ch, ev: e})
	}
}

func (s *Sender) push(j job) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		s.log.Warn("队列已关闭，丢弃通知", "channel", j.ch.Name(), "event", j.ev.Kind)
		return
	}
	if len(s.queue) >= queueCap {
		old := s.queue[0]
		s.queue = s.queue[1:]
		s.log.Warn("通知队列已满，丢弃最旧的一条", "channel", old.ch.Name(), "event", old.ev.Kind)
	}
	s.queue = append(s.queue, j)
	s.mu.Unlock()
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *Sender) pop() (job, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.queue) == 0 {
		return job{}, false
	}
	j := s.queue[0]
	s.queue = s.queue[1:]
	return j, true
}

// Run 依次发送队列中的通知直到 ctx 结束；一旦 ctx 结束立即停止处理新通知（不会因为队列积压或重试不断
// 回填而拖延退出），改为在 drainWait 内尽量发送完剩余通知（不再重试）
func (s *Sender) Run(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			s.drain()
			return
		}
		if j, ok := s.pop(); ok {
			s.deliver(context.Background(), j, true)
			continue
		}
		select {
		case <-ctx.Done():
			s.drain()
			return
		case <-s.wake:
		}
	}
}

// drain 用统一的截止时间（now + drainWait）派生 dctx，作为剩余通知发送的父 context；
// deliver 内部再按 s.o.Timeout 取较短者，保证单条请求也不会把整体退出时间拖到 drainWait 之外
func (s *Sender) drain() {
	s.mu.Lock()
	s.closed = true
	rest := s.queue
	s.queue = nil
	s.mu.Unlock()
	deadline := time.Now().Add(s.drainWait)
	dctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	for i, j := range rest {
		if time.Now().After(deadline) {
			s.log.Warn("退出时仍有通知未发送", "count", len(rest)-i)
			return
		}
		s.deliver(dctx, j, false)
	}
}

// deliver 发送一条通知；parent 决定单次请求的最长等待时间（取 s.o.Timeout 与 parent 剩余时间中较短者，
// 即 context.WithTimeout(parent, s.o.Timeout)）。失败时按重试间隔重新入队，重试用尽后记录日志（错误中不含请求地址）
func (s *Sender) deliver(parent context.Context, j job, retry bool) {
	ctx, cancel := context.WithTimeout(parent, s.o.Timeout)
	err := j.ch.Send(ctx, j.ev)
	cancel()
	if err == nil {
		return
	}
	if retry && j.attempt < len(s.o.RetryDelays) {
		d := s.o.RetryDelays[j.attempt]
		j.attempt++
		time.AfterFunc(d, func() { s.push(j) })
		return
	}
	s.log.Warn("通知发送失败", "channel", j.ch.Name(), "event", j.ev.Kind, "server", j.ev.ServerID, "err", err)
}
