package main

import (
	"fmt"
	"os"
	"os/signal"
	"simple-server-status/agent/global"
	"simple-server-status/agent/internal"
)

func main() {
	//print build var
	fmt.Printf("build variable %s %s %s %s\n", global.GitCommit, global.Version, global.BuiltAt, global.GoVersion)

	global.VP = internal.InitConfig()
	global.LOG = internal.InitLog()

	wsClient := internal.InitWs()
	internal.StartTask(wsClient)

	//等待程序退出信号
	signalChan := make(chan os.Signal, 1)
	// 捕获指定信号
	signal.Notify(signalChan, os.Interrupt, os.Kill)
	// 阻塞直到接收到信号
	sig := <-signalChan
	fmt.Printf("\nReceived signal: %s\n", sig)
	wsClient.CloseWs()
}
