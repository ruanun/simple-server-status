package collect

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

// traceServer 在指定地址上模拟 Cloudflare trace，返回请求方地址与国家
func traceServer(t *testing.T, network, addr, loc string) string {
	t.Helper()
	ln, err := net.Listen(network, addr)
	if err != nil {
		t.Skipf("无法监听 %s %s：%v", network, addr, err)
	}
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, _ := net.SplitHostPort(r.RemoteAddr)
		fmt.Fprintf(w, "fl=1\nip=%s\nloc=%s\n", host, loc)
	}))
	srv.Listener = ln
	srv.Start()
	t.Cleanup(srv.Close)
	return srv.URL + "/cdn-cgi/trace"
}

func TestDetectNetworkBoth(t *testing.T) {
	v4 := traceServer(t, "tcp4", "127.0.0.1:0", "JP")
	v6 := traceServer(t, "tcp6", "[::1]:0", "JP")
	n, err := DetectNetwork(context.Background(), v4, v6)
	if err != nil || n.IPv4 != "127.0.0.1" || n.IPv6 != "::1" || n.Country != "JP" {
		t.Fatalf("%+v %v", n, err)
	}
}

func TestDetectNetworkOnlyV4(t *testing.T) {
	v4 := traceServer(t, "tcp4", "127.0.0.1:0", "HK")
	// v6 地址指向只监听 IPv4 的服务：强制 tcp6 连接会失败
	n, err := DetectNetwork(context.Background(), v4, v4)
	if err != nil || n.IPv4 != "127.0.0.1" || n.IPv6 != "" || n.Country != "HK" {
		t.Fatalf("只有 IPv4 时应返回 IPv4 与国家 %+v %v", n, err)
	}
}

func TestDetectNetworkNone(t *testing.T) {
	if _, err := DetectNetwork(context.Background(), "http://127.0.0.1:1/x", "http://127.0.0.1:1/x"); err == nil {
		t.Fatal("都失败时应返回错误")
	}
}

// TestDetectNetworkIgnoresCountryFromFailedProbe 验证某一路探测的地址协议族不符（视为该路失败）时，
// 不会采信它返回的国家代码
func TestDetectNetworkIgnoresCountryFromFailedProbe(t *testing.T) {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("无法监听 tcp4：%v", err)
	}
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// 故意返回一个 IPv6 地址：与本次 tcp4 探测的协议族不符，parseTraceIP 会判定为失败
		fmt.Fprint(w, "fl=1\nip=::1\nloc=HK\n")
	}))
	srv.Listener = ln
	srv.Start()
	t.Cleanup(srv.Close)
	v4URL := srv.URL + "/cdn-cgi/trace"

	n, err := DetectNetwork(context.Background(), v4URL, "http://127.0.0.1:1/x")
	if err == nil {
		t.Fatalf("两路都拿不到合法地址时应返回错误: %+v", n)
	}
	if n.Country != "" {
		t.Fatalf("地址解析失败的探测不应采信其国家代码: %+v", n)
	}
}

func TestParseTraceIPRejectsWrongFamily(t *testing.T) {
	if ip := parseTraceIP("ip=::1\n", false); ip != "" {
		t.Fatalf("IPv4 探测不应接受 IPv6 地址 %q", ip)
	}
	if ip := parseTraceIP("ip=1.2.3.4\n", true); ip != "" {
		t.Fatalf("IPv6 探测不应接受 IPv4 地址 %q", ip)
	}
	if ip := parseTraceIP("ip=2001:db8::1\n", true); ip != "2001:db8::1" {
		t.Fatalf("应接受合法 IPv6 %q", ip)
	}
}
