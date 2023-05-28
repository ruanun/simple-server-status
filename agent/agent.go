package main

import (
	"os"
	"os/signal"
	"simple-server-status/agent/common"
	"simple-server-status/agent/service"
	"time"
)

func main() {
	common.InitGlobal()
	//初始化websocket客户端
	common.InitWs()
	//上报信息
	service.InitWsService()

	// 等待程序退出信号
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	done := make(chan struct{})

	// 循环，等待程序退出信号通道关闭
	for {
		select {
		case <-interrupt:
			common.LOG.Info("interrupt")
			// 关闭WebSocket连接
			common.CloseWs(done)
			// 等待服务器响应并关闭连接
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			return
		}
	}
}
