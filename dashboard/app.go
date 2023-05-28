package main

import (
	"fmt"
	"simple-server-status/dashboard/common"
	"simple-server-status/dashboard/webserver"
)

func main() {
	common.InitGlobal()

	address := fmt.Sprintf("%s:%d", common.CONFIG.Address, common.CONFIG.Port)
	err := webserver.InitServer().Run(address)
	if err != nil {
		common.LOG.Fatal("webserver initiate error ", err)
		return
	}
}
