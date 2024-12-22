package internal

import (
	"io"
	"math/rand"
	"net/http"
	"simple-server-status/agent/global"
	"strings"
	"time"
)

var httpClient = http.Client{Timeout: 10 * time.Second}

func StartTask(client *WsClient) {
	//获取服务器ip和位置
	go getServerLocAndIp()
	//定时统计网络速度，流量信息
	go statNetInfo()
	//定时上报信息
	go reportInfo(client)
}

func statNetInfo() {
	defer func() {
		if err := recover(); err != nil {
			global.LOG.Error("StatNetworkSpeed panic: ", err)
		}
	}()
	for {
		StatNetworkSpeed()
		time.Sleep(time.Second * 1)
	}
}

var urls = []string{
	"https://cloudflare.com/cdn-cgi/trace",
	"https://developers.cloudflare.com/cdn-cgi/trace",
	"https://blog.cloudflare.com/cdn-cgi/trace",
	"https://info.cloudflare.com/cdn-cgi/trace",
	"https://store.ubi.com/cdn-cgi/trace",
}

func RandomIntInRange(min, max int) int {
	// 使用当前时间创建随机数生成器的种子源
	source := rand.NewSource(time.Now().UnixNano())
	rng := rand.New(source) // 创建新的随机数生成器
	// 生成随机整数，范围是 [min, max]
	return rng.Intn(max-min+1) + min
}
func getServerLocAndIp() {
	defer func() {
		if err := recover(); err != nil {
			global.LOG.Error("getServerLocAndIp panic: ", err)
		}
	}()
	global.LOG.Debug("getServerLocAndIp start")

	//随机一个url
	url := urls[RandomIntInRange(0, len(urls)-1)]
	global.LOG.Debug("getServerLocAndIp url: ", url)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		global.LOG.Error(err)
	}
	req.Header.Add("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/104.0.5112.79 Safari/537.36")
	resp, err := httpClient.Do(req)
	if err != nil {
		global.LOG.Error(err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		global.LOG.Error("getServerLocAndIp fail status code: ", resp.StatusCode)
		return
	}
	bodyStr, err := io.ReadAll(resp.Body)
	if err != nil {
		global.LOG.Error(err)
	}
	lines := strings.Split(string(bodyStr), "\n")

	for _, line := range lines {
		parts := strings.Split(line, "=")
		if len(parts) == 2 {
			switch parts[0] {
			case "ip":
				global.HostIp = parts[1]
			case "loc":
				global.HostLocation = parts[1]
			}
		}
	}
	global.LOG.Debugf("getServerLocAndIp end ip: %s loc: %s", global.HostIp, global.HostLocation)
	//sleep
	time.Sleep(time.Hour * 1)
}

func reportInfo(client *WsClient) {
	defer func() {
		if err := recover(); err != nil {
			global.LOG.Error("reportInfo panic: ", err)
		}
	}()
	defer global.LOG.Error("reportInfo exit!")
	global.LOG.Debug("reportInfo start")

	for {
		serverInfo := GetServerInfo()
		//发送
		client.SendJsonMsg(serverInfo)
		//间隔
		time.Sleep(time.Second * time.Duration(global.AgentConfig.ReportTimeInterval))
	}
}
