package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/ruanun/simple-server-status/internal/agent/collect"
	"github.com/ruanun/simple-server-status/internal/proto"
)

// Sampler 指标采集接口（collect.Collector 实现）
type Sampler interface {
	Static(version string) proto.Hello
	Sample() proto.Report
	SetFilter(f collect.Filter)
}

// ErrStopped Dashboard 通知 Agent 退出
var ErrStopped = errors.New("dashboard 要求停止")

// unauthorizedError Dashboard 返回 401
type unauthorizedError struct{}

func (unauthorizedError) Error() string { return "鉴权失败（401），请检查 id 与 secret" }

// Client 负责与 Dashboard 保持连接并周期上报
type Client struct {
	cfg     Config
	url     string
	s       Sampler
	log     *slog.Logger
	version string
	country string
	// filterID 当前已应用到采集器的网卡过滤器摘要；仅由 Run 所在协程访问，跨重连保留（采集器的过滤规则同样保留）
	filterID string
}

// NewClient 创建客户端
func NewClient(cfg Config, s Sampler, log *slog.Logger, version string) (*Client, error) {
	u, err := WSURL(cfg.Dashboard)
	if err != nil {
		return nil, err
	}
	return &Client{cfg: cfg, url: u, s: s, log: log, version: version}, nil
}

// SetCountry 设置随 hello 上报的国家代码
func (c *Client) SetCountry(cc string) { c.country = cc }

// Run 保持连接直到 ctx 结束（返回 nil）或收到 stop（返回 ErrStopped）
func (c *Client) Run(ctx context.Context) error {
	attempt := 0
	for {
		started := time.Now()
		err := c.session(ctx)
		if errors.Is(err, ErrStopped) {
			return err
		}
		if ctx.Err() != nil {
			return nil
		}
		if time.Since(started) > time.Minute {
			attempt = 0
		}
		var ue unauthorizedError
		wait := Backoff(attempt, errors.As(err, &ue), rand.Float64) //nolint:gosec // 重连抖动无需密码学随机数
		attempt++
		c.log.Warn("连接断开，稍后重连", "err", err, "wait", wait.String())
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(wait):
		}
	}
}

// Backoff 计算第 attempt 次重连的等待时间：1s 起指数增长、60s 封顶、±20% 抖动；鉴权失败固定 5 分钟
func Backoff(attempt int, unauthorized bool, rnd func() float64) time.Duration {
	if unauthorized {
		return 5 * time.Minute
	}
	d := time.Second << min(attempt, 6)
	if d > time.Minute {
		d = time.Minute
	}
	return time.Duration(float64(d) * (0.8 + 0.4*rnd()))
}

// session 建立一次连接并运行到断开
func (c *Client) session(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	h := http.Header{}
	h.Set("Authorization", "Bearer "+c.cfg.ID+":"+c.cfg.Secret)
	dctx, dcancel := context.WithTimeout(ctx, 15*time.Second)
	conn, resp, err := websocket.Dial(dctx, c.url, &websocket.DialOptions{HTTPHeader: h})
	dcancel()
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusUnauthorized {
			return unauthorizedError{}
		}
		return fmt.Errorf("连接失败: %w", err)
	}
	defer func() { _ = conn.CloseNow() }()
	conn.SetReadLimit(64 << 10)
	c.log.Info("已连接 Dashboard", "url", c.url)

	hello := c.s.Static(c.version)
	hello.Country = c.country
	if err := write(ctx, conn, proto.TypeHello, hello); err != nil {
		return err
	}

	cfgCh := make(chan proto.Config, 1)
	errCh := make(chan error, 1)
	go func() { errCh <- c.readLoop(ctx, conn, cfgCh) }()

	interval := 2 * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			_ = conn.Close(websocket.StatusNormalClosure, "")
			return ctx.Err()
		case err := <-errCh:
			return err
		case cfg := <-cfgCh:
			c.s.SetFilter(collect.Filter{NICInclude: cfg.NICInclude, NICExclude: cfg.NICExclude, MountExclude: cfg.MountExclude})
			c.filterID = cfg.FilterID
			if d := clampInterval(cfg.ReportInterval); d != interval {
				interval = d
				ticker.Reset(d)
			}
			c.log.Info("已应用 Dashboard 下发的参数", "interval", interval.String())
		case <-ticker.C:
			// SetFilter 与 Sample 在同一协程中顺序执行，因此 filterID 与本次采集实际使用的过滤器一致
			r := c.s.Sample()
			r.FilterID = c.filterID
			r.Normalize()
			if err := write(ctx, conn, proto.TypeReport, r); err != nil {
				return err
			}
		}
	}
}

// readLoop 处理 Dashboard 下发的消息
func (c *Client) readLoop(ctx context.Context, conn *websocket.Conn, cfgCh chan<- proto.Config) error {
	for {
		_, b, err := conn.Read(ctx)
		if err != nil {
			return fmt.Errorf("读取失败: %w", err)
		}
		env, err := proto.Decode(b)
		if err != nil {
			c.log.Warn("忽略无法解析的消息", "err", err)
			continue
		}
		switch env.Type {
		case proto.TypeConfig:
			var cfg proto.Config
			if err := json.Unmarshal(env.Data, &cfg); err != nil {
				c.log.Warn("忽略无效的 config 消息", "err", err)
				continue
			}
			select {
			case cfgCh <- cfg:
			case <-ctx.Done():
				return ctx.Err()
			}
		case proto.TypeStop:
			var s proto.Stop
			_ = json.Unmarshal(env.Data, &s)
			c.log.Warn("收到停止指令", "reason", s.Reason)
			return ErrStopped
		default:
			c.log.Debug("忽略未知消息", "type", env.Type)
		}
	}
}

// clampInterval 把下发的上报间隔限制在 1–60 秒，未设置时为 2 秒
func clampInterval(sec int) time.Duration {
	if sec < 1 {
		sec = 2
	}
	if sec > 60 {
		sec = 60
	}
	return time.Duration(sec) * time.Second
}

func write(ctx context.Context, conn *websocket.Conn, typ string, data any) error {
	b, err := proto.Encode(typ, data)
	if err != nil {
		return err
	}
	wctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := conn.Write(wctx, websocket.MessageText, b); err != nil {
		return fmt.Errorf("发送 %s 失败: %w", typ, err)
	}
	return nil
}
