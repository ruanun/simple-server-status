package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/ruanun/simple-server-status/internal/dashboard/store"
)

// Options 发送选项，零值字段使用默认值（测试可替换 HTTP 客户端、Telegram 地址与重试间隔）
type Options struct {
	Client       *http.Client
	TelegramBase string
	RetryDelays  []time.Duration
	Timeout      time.Duration
}

func (o Options) withDefaults() Options {
	if o.Client == nil {
		o.Client = &http.Client{}
	}
	if o.TelegramBase == "" {
		o.TelegramBase = "https://api.telegram.org"
	}
	if o.RetryDelays == nil {
		o.RetryDelays = []time.Duration{5 * time.Second, 30 * time.Second, 2 * time.Minute}
	}
	if o.Timeout == 0 {
		o.Timeout = 10 * time.Second
	}
	return o
}

// Channel 通知渠道
type Channel interface {
	Name() string
	Send(ctx context.Context, e Event) error
}

// Channels 按设置返回已启用的渠道
func Channels(cfg store.NotifySettings, o Options) []Channel {
	o = o.withDefaults()
	var chs []Channel
	if cfg.WebhookURL != "" {
		chs = append(chs, webhook{url: cfg.WebhookURL, client: o.Client})
	}
	if cfg.TelegramToken != "" && cfg.TelegramChatID != "" {
		chs = append(chs, telegram{base: o.TelegramBase, token: cfg.TelegramToken, chatID: cfg.TelegramChatID, client: o.Client})
	}
	return chs
}

type webhookPayload struct {
	Event      string `json:"event"`
	ServerID   string `json:"server_id"`
	ServerName string `json:"server_name"`
	Title      string `json:"title"`
	Message    string `json:"message"`
	Time       int64  `json:"time"`
}

type webhook struct {
	url    string
	client *http.Client
}

func (w webhook) Name() string { return "webhook" }

func (w webhook) Send(ctx context.Context, e Event) error {
	body, err := json.Marshal(webhookPayload{Event: e.Kind, ServerID: e.ServerID, ServerName: e.ServerName, Title: e.Title, Message: e.Message, Time: e.Time})
	if err != nil {
		return err
	}
	return post(ctx, w.client, w.url, body, func(resp *http.Response) error {
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("对方返回 HTTP %d", resp.StatusCode)
		}
		return nil
	})
}

type telegram struct {
	base, token, chatID string
	client              *http.Client
}

func (t telegram) Name() string { return "telegram" }

func (t telegram) Send(ctx context.Context, e Event) error {
	body, err := json.Marshal(map[string]string{"chat_id": t.chatID, "text": e.Title + "\n" + e.Message})
	if err != nil {
		return err
	}
	return post(ctx, t.client, t.base+"/bot"+t.token+"/sendMessage", body, func(resp *http.Response) error {
		var r struct {
			OK          bool   `json:"ok"`
			Description string `json:"description"`
		}
		_ = json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&r)
		if resp.StatusCode != http.StatusOK || !r.OK {
			return fmt.Errorf("对方返回 HTTP %d %s", resp.StatusCode, r.Description)
		}
		return nil
	})
}

// post 发送 JSON 并用 check 判断响应；网络错误去掉请求地址（可能含 Bot Token 或 Webhook 密钥）
func post(ctx context.Context, client *http.Client, target string, body []byte, check func(*http.Response) error) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return errors.New("请求地址无效")
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		var ue *url.Error
		if errors.As(err, &ue) {
			return fmt.Errorf("请求失败：%w", ue.Err)
		}
		return errors.New("请求失败")
	}
	defer resp.Body.Close()
	err = check(resp)
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	return err
}

// LogStore 通知记录存储（store.Store 实现）
type LogStore interface {
	AddNotifyLog(ctx context.Context, l store.NotifyLog) (int64, error)
	FinishNotifyLog(ctx context.Context, id int64, status, errMsg string, doneAt int64) error
}

// SendTest 同步、并行地向已启用的渠道各发送一条测试消息（不重试）；logs 非 nil 时写入通知记录。
// 返回值键为 webhook、telegram：nil 表示未启用，"ok" 表示成功，否则为错误信息
func SendTest(ctx context.Context, cfg store.NotifySettings, o Options, now time.Time, logs LogStore) map[string]*string {
	o = o.withDefaults()
	res := map[string]*string{"webhook": nil, "telegram": nil}
	e := Event{Kind: KindTest, Time: now.Unix()}
	Render(&e, cfg.Lang)
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, ch := range Channels(cfg, o) {
		var logID int64
		if logs != nil {
			id, err := logs.AddNotifyLog(ctx, store.NotifyLog{Kind: KindTest, Channel: ch.Name(), Title: e.Title, Message: e.Message, Status: store.LogPending, CreatedAt: e.Time})
			if err == nil {
				logID = id
			}
		}
		wg.Add(1)
		go func(ch Channel, logID int64) {
			defer wg.Done()
			cctx, cancel := context.WithTimeout(ctx, o.Timeout)
			err := ch.Send(cctx, e)
			cancel()
			msg, status := "ok", store.LogSent
			if err != nil {
				msg, status = err.Error(), store.LogFailed
			}
			if logs != nil && logID != 0 { // 请求被取消也要写入结果
				errText := ""
				if err != nil {
					errText = msg
				}
				_ = logs.FinishNotifyLog(context.WithoutCancel(ctx), logID, status, errText, time.Now().Unix())
			}
			mu.Lock()
			res[ch.Name()] = &msg
			mu.Unlock()
		}(ch, logID)
	}
	wg.Wait()
	return res
}
