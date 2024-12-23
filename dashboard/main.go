package main

import (
	"fmt"
	"simple-server-status/dashboard/global"
	"simple-server-status/dashboard/internal"
	"simple-server-status/dashboard/server"
)

func main() {
	//print build var
	fmt.Printf("build variable %s %s %s %s\n", global.GitCommit, global.Version, global.BuiltAt, global.GoVersion)

	global.VP = internal.InitConfig()
	global.LOG = internal.InitLog()

	e := server.InitServer()
	address := fmt.Sprintf("%s:%d", global.CONFIG.Address, global.CONFIG.Port)
	global.LOG.Info("webserver start ", address)
	err := e.Run(address)
	if err != nil {
		global.LOG.Fatal("webserver start failed ", err)
	}
}
