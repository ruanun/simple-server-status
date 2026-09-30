package notify

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

// queueCap 发送队列容量，满时丢弃最旧的一条
const queueCap = 100

// defaultDrainWait 退出时发送剩余通知的默认最长时间
const defaultDrainWait = 5 * time.Second

// 任务未能发送的原因
var (
	ErrDropped  = errors.New("队列已满，已丢弃")
	ErrShutdown = errors.New("退出时未能发送")
)

type job struct {
	ch      Channel
	ev      Event
	attempt int
	done    func(error)
}

// finish 报告任务的最终结果（每个任务恰好一次）
func (j job) finish(err error) {
	if j.done != nil {
		j.done(err)
	}
}

// retryItem 正在等待重试的任务及其定时器
type retryItem struct {
	t *time.Timer
	j job
}

// Sender 异步发送队列：单个工作协程依次发送，失败按 RetryDelays 延迟后重新入队
type Sender struct {
	o         Options
	log       *slog.Logger
	mu        sync.Mutex
	queue     []job
	retries   map[uint64]retryItem // 等待重试的任务，退出时由收尾接管
	nextRetry uint64
	closed    bool
	wake      chan struct{}
	drainWait time.Duration // 退出时发送剩余通知的最长时间，默认 defaultDrainWait；测试可缩短
}

// NewSender 创建发送队列；log 为 nil 时使用 slog.Default()
func NewSender(o Options, log *slog.Logger) *Sender {
	if log == nil {
		log = slog.Default()
	}
	return &Sender{o: o.withDefaults(), log: log, retries: map[uint64]retryItem{}, wake: make(chan struct{}, 1), drainWait: defaultDrainWait}
}

// Enqueue 把事件加入每个渠道的发送队列；done 在每个渠道任务最终成功或失败时各调用一次（可为 nil）。
// done 可能在不同协程中并发调用（发送协程 Run、重试定时器、以及队满丢弃时的 Enqueue 调用方本身），
// 调用方必须保证其并发安全；done 调用时不持有队列锁，可以在其中再次入队
func (s *Sender) Enqueue(chs []Channel, e Event, done func(channel string, err error)) {
	for _, ch := range chs {
		j := job{ch: ch, ev: e}
		if done != nil {
			name := ch.Name()
			j.done = func(err error) { done(name, err) }
		}
		s.push(j)
	}
}

// push 把任务加入队尾；回调一律在释放 s.mu 之后调用，避免回调中再次入队时死锁
func (s *Sender) push(j job) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		s.log.Warn("队列已关闭，丢弃通知", "channel", j.ch.Name(), "event", j.ev.Kind)
		j.finish(ErrShutdown)
		return
	}
	var dropped *job
	if len(s.queue) >= queueCap {
		old := s.queue[0]
		s.queue = s.queue[1:]
		dropped = &old
	}
	s.queue = append(s.queue, j)
	s.mu.Unlock()
	if dropped != nil {
		s.log.Warn("通知队列已满，丢弃最旧的一条", "channel", dropped.ch.Name(), "event", dropped.ev.Kind)
		dropped.finish(ErrDropped)
	}
	s.signal()
}

// pushFront 把任务放回队首（发送因退出被中断时使用）
func (s *Sender) pushFront(j job) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		j.finish(ErrShutdown)
		return
	}
	s.queue = append([]job{j}, s.queue...)
	s.mu.Unlock()
	s.signal()
}

func (s *Sender) signal() {
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

// retryLater 在 d 之后把任务重新入队；等待期间任务登记在 s.retries 中，
// 退出时由 drain 取走（定时器回调发现任务已被取走则不再入队），保证每个任务只结束一次
func (s *Sender) retryLater(d time.Duration, j job) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		j.finish(ErrShutdown)
		return
	}
	id := s.nextRetry
	s.nextRetry++
	t := time.AfterFunc(d, func() {
		s.mu.Lock()
		_, ok := s.retries[id]
		delete(s.retries, id)
		s.mu.Unlock()
		if ok {
			s.push(j)
		}
	})
	s.retries[id] = retryItem{t: t, j: j}
	s.mu.Unlock()
}

// Run 依次发送队列中的通知直到 ctx 结束；进行中的请求随 ctx 结束而中断并放回队首。
// 一旦 ctx 结束立即停止处理新通知（不会因为队列积压或重试不断回填而拖延退出），
// 改为在 drainWait 内尽量发送完剩余通知（不再重试）
func (s *Sender) Run(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			s.drain()
			return
		}
		if j, ok := s.pop(); ok {
			s.deliver(ctx, j, true)
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
// deliver 内部再按 s.o.Timeout 取较短者，保证单条请求也不会把整体退出时间拖到 drainWait 之外。
// 剩余任务包括队列中的与等待重试的；截止后仍未发送的以 ErrShutdown 结束
func (s *Sender) drain() {
	s.mu.Lock()
	s.closed = true
	rest := s.queue
	s.queue = nil
	for id, r := range s.retries {
		r.t.Stop()
		rest = append(rest, r.j)
		delete(s.retries, id)
	}
	s.mu.Unlock()
	deadline := time.Now().Add(s.drainWait)
	dctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	for i, j := range rest {
		if time.Now().After(deadline) {
			s.log.Warn("退出时仍有通知未发送", "count", len(rest)-i)
			for _, left := range rest[i:] {
				left.finish(ErrShutdown)
			}
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
		j.finish(nil)
		return
	}
	if retry && parent.Err() != nil {
		s.pushFront(j) // 因退出被中断：放回队首交给收尾，不计入重试次数
		return
	}
	if retry && j.attempt < len(s.o.RetryDelays) {
		d := s.o.RetryDelays[j.attempt]
		j.attempt++
		s.retryLater(d, j)
		return
	}
	if !retry && parent.Err() != nil {
		err = ErrShutdown // 收尾截止时间已到而中断：统一记为「退出时未能发送」
	}
	s.log.Warn("通知发送失败", "channel", j.ch.Name(), "event", j.ev.Kind, "server", j.ev.ServerID, "err", err)
	j.finish(err)
}
