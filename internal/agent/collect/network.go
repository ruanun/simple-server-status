package collect

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// probeTimeout 单次探测超时
const probeTimeout = 5 * time.Second

// Network 本机出口的国家代码与公网地址；探测失败的项为空
type Network struct {
	Country string
	IPv4    string
	IPv6    string
}

// DetectNetwork 分别只走 IPv4、只走 IPv6 请求 trace 地址（两路并行），得到对应的公网地址；
// 国家代码取任一成功响应（优先 IPv4）。两者都失败时返回错误
func DetectNetwork(ctx context.Context, v4URL, v6URL string) (Network, error) {
	var (
		wg    sync.WaitGroup
		body6 string
		err6  error
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		body6, err6 = traceVia(ctx, "tcp6", v6URL)
	}()
	body4, err4 := traceVia(ctx, "tcp4", v4URL)
	wg.Wait()

	var n Network
	if err4 == nil {
		n.IPv4 = parseTraceIP(body4, false)
		// 地址解析失败（协议族不符或无效）视为该次探测失败，国家代码也不采信
		if n.IPv4 != "" {
			n.Country = ParseTraceLoc(body4)
		}
	}
	if err6 == nil {
		n.IPv6 = parseTraceIP(body6, true)
		if n.IPv6 != "" && n.Country == "" {
			n.Country = ParseTraceLoc(body6)
		}
	}
	if n.IPv4 == "" && n.IPv6 == "" {
		return n, fmt.Errorf("IPv4 与 IPv6 探测均失败: %w", errors.Join(err4, err6))
	}
	return n, nil
}

// traceVia 只通过指定网络（tcp4 / tcp6）请求 trace 地址并返回内容
func traceVia(ctx context.Context, network, url string) (string, error) {
	var d net.Dialer
	client := &http.Client{
		Timeout: probeTimeout,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, addr string) (net.Conn, error) {
				return d.DialContext(ctx, network, addr)
			},
		},
	}
	defer client.CloseIdleConnections()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return string(b), err
}

// parseTraceIP 从 trace 内容中取出 ip=，要求与探测的协议一致，否则返回空
func parseTraceIP(body string, v6 bool) string {
	for _, line := range strings.Split(body, "\n") {
		v, ok := strings.CutPrefix(strings.TrimSpace(line), "ip=")
		if !ok {
			continue
		}
		ip := net.ParseIP(strings.TrimSpace(v))
		if ip == nil || (ip.To4() == nil) != v6 {
			return ""
		}
		return ip.String()
	}
	return ""
}
