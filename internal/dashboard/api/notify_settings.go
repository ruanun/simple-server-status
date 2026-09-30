package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/ruanun/simple-server-status/internal/dashboard/notify"
	"github.com/ruanun/simple-server-status/internal/dashboard/store"
)

// normalizeNotify 去除渠道配置首尾空白、补全语言并校验取值范围
func normalizeNotify(n *store.NotifySettings) error {
	n.WebhookURL = strings.TrimSpace(n.WebhookURL)
	if n.WebhookURL != "" {
		u, err := url.Parse(n.WebhookURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return errors.New("webhook 地址需以 http:// 或 https:// 开头")
		}
	}
	n.TelegramToken = strings.TrimSpace(n.TelegramToken)
	n.TelegramChatID = strings.TrimSpace(n.TelegramChatID)
	if (n.TelegramToken == "") != (n.TelegramChatID == "") {
		return errors.New("telegram Bot Token 与 Chat ID 需同时填写")
	}
	if n.Lang == "" {
		n.Lang = "zh-CN"
	}
	if n.Lang != "zh-CN" && n.Lang != "en-US" {
		return errors.New("通知语言只能是 zh-CN 或 en-US")
	}
	for _, c := range []struct {
		name     string
		v        int
		min, max int
	}{
		{"离线通知时长（分钟）", n.OfflineMinutes, 1, 1440},
		{"CPU 阈值", n.LoadCPU, 1, 100},
		{"内存阈值", n.LoadMem, 1, 100},
		{"硬盘阈值", n.LoadDisk, 1, 100},
		{"高负载持续时长（分钟）", n.LoadMinutes, 1, 10},
		{"到期提前天数", n.ExpireDays, 1, 90},
		{"流量阈值", n.TrafficPercent, 1, 100},
	} {
		if c.v < c.min || c.v > c.max {
			return fmt.Errorf("%s应在 %d–%d 之间", c.name, c.min, c.max)
		}
	}
	return nil
}

// adminNotifyTest 使用请求中的通知设置（可以尚未保存）同步发送测试消息
func (a *API) adminNotifyTest(c *gin.Context) {
	cfg := store.DefaultNotifySettings()
	if err := c.ShouldBindJSON(&cfg); err != nil {
		fail(c, http.StatusBadRequest, "bad_request", "请求格式错误")
		return
	}
	if err := normalizeNotify(&cfg); err != nil {
		fail(c, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	if len(notify.Channels(cfg, a.NotifyOptions)) == 0 {
		fail(c, http.StatusBadRequest, "invalid_input", "未配置任何通知渠道")
		return
	}
	respond(c, notify.SendTest(c.Request.Context(), cfg, a.NotifyOptions, a.Now(), a.Store))
}

// Servers 供通知模块读取当前服务器列表
func (a *API) Servers() []store.Server { return a.serverList() }

// NotifySettings 供通知模块读取当前通知设置
func (a *API) NotifySettings() store.NotifySettings { return a.currentSettings().Notify }

// RunNotifier 运行通知检查与发送，直到 ctx 结束
func (a *API) RunNotifier(ctx context.Context) { a.notifier.Run(ctx) }

// RunOutages 运行离线记录检查，直到 ctx 结束
func (a *API) RunOutages(ctx context.Context) { a.outages.Run(ctx) }
