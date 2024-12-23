package internal

import (
	"encoding/json"
	"github.com/gorilla/websocket"
	"log"
	"math"
	"net/http"
	"simple-server-status/agent/config"
	"simple-server-status/agent/global"
	"time"
)

type WsClient struct {
	// 服务器地址
	ServerAddr string
	// 认证头
	AuthHeader http.Header
	// 重连次数
	RetryCountMax int
	// 链接
	conn *websocket.Conn
}

func NewWsClient(AgentConfig *config.AgentConfig) *WsClient {
	var AuthHeader = make(http.Header)
	AuthHeader.Add("X-AUTH-SECRET", AgentConfig.AuthSecret)
	AuthHeader.Add("X-SERVER-ID", AgentConfig.ServerId)

	return &WsClient{
		AuthHeader:    AuthHeader,
		RetryCountMax: global.RetryCountMax,
		ServerAddr:    AgentConfig.ServerAddr,
	}
}

func (c *WsClient) connect() *websocket.Conn {
	global.LOG.Info("开始尝试连接服务器..。")
	global.LOG.Info("服务器地址：", c.ServerAddr)
	retryCount := 0
	for {
		// 尝试建立WebSocket连接
		err := func() error {
			t, _, err := websocket.DefaultDialer.Dial(c.ServerAddr, c.AuthHeader)
			if err != nil {
				return err
			}
			c.conn = t
			return nil
		}()

		// 如果连接成功，则退出重试循环
		if err == nil {
			global.LOG.Info("连接成功")
			break
		}

		// 如果连接失败，则等待一段时间后重新尝试连接
		delay := retryDelay(retryCount)
		global.LOG.Infof("delay %f s", delay.Seconds())
		global.LOG.Infof("WebSocket dial failed: %v (retry after %fs)", err, delay.Seconds())
		retryCount++
		if retryCount > c.RetryCountMax {
			log.Fatal("WebSocket dial failed: max retries exceeded")
		}
		global.LOG.Info("重连次数：", retryCount)
		time.Sleep(delay)
	}
	return c.conn
}

// 返回下一次重试的等待时间（指数衰减算法）
func retryDelay(retryCount int) time.Duration {
	minDelay := 3 * time.Second
	maxDelay := 10 * time.Minute
	factor := 1.2

	delay := time.Duration(float64(minDelay) * math.Pow(factor, float64(retryCount)))
	if delay > maxDelay {
		delay = maxDelay
	}
	return delay
}

func (c *WsClient) CloseWs() {
	// 关闭WebSocket连接
	err := c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	if err != nil {
		global.LOG.Error("write close:", err)
		return
	}
	c.conn.Close()
}

func (c *WsClient) SendJsonMsg(obj interface{}) {
	bytes, _ := json.Marshal(obj)

	err := c.conn.WriteMessage(websocket.TextMessage, bytes)
	if err != nil {
		global.LOG.Error("发送失败 ==== ", err)
		time.Sleep(time.Second * 2)
		//重连
		c.connect()
	}
}

func handleMessage(client *WsClient) {
	for {
		_, message, err := client.conn.ReadMessage()
		if err != nil {
			time.Sleep(time.Second * 10)
			continue
		}
		global.LOG.Info("Received message: %s\n", message)
	}
}
func InitWs() *WsClient {
	//初始化连接
	wsClient := NewWsClient(global.AgentConfig)
	wsClient.connect()
	//接收服务器发送的消息
	go handleMessage(wsClient)
	return wsClient
}
