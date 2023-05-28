package service

import (
	"simple-server-status/agent/common"
	"time"
)

func InitWsService() {
	//定时上报信息
	go reportInfo()
	//接收服务器发送的消息
	go handleMessage()
}

func handleMessage() {
	for {
		_, message, err := common.Conn.ReadMessage()
		if err != nil {
			time.Sleep(time.Second * 10)
			continue
		}
		common.LOG.Info("Received message: %s\n", message)
	}
}

func reportInfo() {
	defer common.LOG.Error("reportInfo exit!")

	for {
		serverInfo := common.GetServerInfo()
		//发送
		common.SendJsonMsg(serverInfo)
		//间隔
		time.Sleep(time.Second * time.Duration(common.AgentConfig.ReportTimeInterval))
	}
}
