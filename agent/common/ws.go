package common

import (
	"encoding/json"
	"github.com/gorilla/websocket"
	"log"
	"math"
	"time"
)

func CloseWs(done chan struct{}) {
	defer close(done)

	// 关闭WebSocket连接
	err := Conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	if err != nil {
		LOG.Error("write close:", err)
		return
	}
}

func SendJsonMsg(obj interface{}) {
	bytes, _ := json.Marshal(obj)

	err := Conn.WriteMessage(websocket.TextMessage, bytes)
	if err != nil {
		LOG.Error("发送失败 ==== ", err)
		time.Sleep(time.Second * 2)
		ReConnect()
	}
}

func ReConnect() {
	Conn = Connect()
}

func Connect() *websocket.Conn {
	retryCount := 0

	var conn *websocket.Conn
	for {
		// 尝试建立WebSocket连接
		err := func() error {
			LOG.Info("服务器地址：", AgentConfig.ServerAddr)
			c, _, err := websocket.DefaultDialer.Dial(AgentConfig.ServerAddr, AuthHeader)
			if err != nil {
				return err
			}
			conn = c
			return nil
		}()

		// 如果连接成功，则退出重试循环
		if err == nil {
			LOG.Info("连接成功")
			break
		}

		// 如果连接失败，则等待一段时间后重新尝试连接
		delay := retryDelay(retryCount)
		LOG.Infof("delay %f s\n", delay.Seconds())
		LOG.Infof("WebSocket dial failed: %v (retry after %fs)\n", err, delay.Seconds())
		retryCount++
		time.Sleep(delay)
		if retryCount > retryCountMax {
			log.Fatal("WebSocket dial failed: max retries exceeded")
		}
	}
	return conn
}

// 返回下一次重试的等待时间（指数衰减算法）
func retryDelay(retryCount int) time.Duration {
	minDelay := 5 * time.Second
	maxDelay := 10 * time.Minute
	factor := 1.5

	delay := time.Duration(float64(minDelay) * math.Pow(factor, float64(retryCount)))
	if delay > maxDelay {
		delay = maxDelay
	}
	return delay
}

func InitWs() {
	//初始化连接
	Conn = Connect()
}
