package model

import "github.com/shirou/gopsutil/v4/load"

type ServerInfo struct {
	Name           string `json:"name"`           //name展示
	Group          string `json:"group"`          //组
	Id             string `json:"id"`             //服务器id
	LastReportTime int64  `json:"lastReportTime"` //最后上报时间

	HostInfo          *HostInfo          `json:"hostInfo"`
	CpuInfo           *CpuInfo           `json:"cpuInfo"`
	VirtualMemoryInfo *VirtualMemoryInfo `json:"virtualMemoryInfo"`
	SwapMemoryInfo    *SwapMemoryInfo    `json:"swapMemoryInfo"`
	DiskInfo          *DiskInfo          `json:"diskInfo"`
	NetworkInfo       *NetworkInfo       `json:"networkInfo"`

	Ip  string `json:"ip"`
	Loc string `json:"loc"`
}
type CpuInfo struct {
	//Cores     int32   `json:"cores"`
	//ModelName string `json:"modelName"`
	//Mhz       float64 `json:"mhz"`
	/*cpu占用*/
	Percent float64 `json:"percent"`
	//cpu信息字符串描述
	Info []string `json:"info"`
}
type HostInfo struct {
	KernelArch           string `json:"kernelArch"` // native cpu architecture queried at runtime, as returned by `uname -m` or empty string in case of error
	KernelVersion        string `json:"kernelVersion"`
	VirtualizationSystem string `json:"virtualizationSystem"`
	Uptime               uint64 `json:"uptime"` //单位秒
	BootTime             uint64 `json:"bootTime"`
	//Procs                uint64 `json:"procs"`          // number of processes
	OS              string        `json:"os"`              // ex: freebsd, linux
	Platform        string        `json:"platform"`        // ex: ubuntu, linuxmint
	PlatformFamily  string        `json:"platformFamily"`  // ex: debian, rhel
	PlatformVersion string        `json:"platformVersion"` //具体版本
	AvgStat         *load.AvgStat `json:"avgStat"`
}
type VirtualMemoryInfo struct {
	Total uint64 `json:"total"`
	//Available   uint64  `json:"available"`
	Used        uint64  `json:"used"`
	UsedPercent float64 `json:"usedPercent"`
	//Free        uint64  `json:"free"`
}
type SwapMemoryInfo struct {
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	Free        uint64  `json:"free"`
	UsedPercent float64 `json:"usedPercent"`
}
type DiskInfo struct {
	Total       uint64       `json:"total"`
	Used        uint64       `json:"used"`
	UsedPercent float64      `json:"usedPercent"`
	Partitions  []*Partition `json:"partitions"`
}

// Partition /*磁盘分区信息*/
type Partition struct {
	MountPoint  string  `json:"mountPoint"`
	Fstype      string  `json:"fstype"`
	Total       uint64  `json:"total"`
	Free        uint64  `json:"free"`
	Used        uint64  `json:"used"`
	UsedPercent float64 `json:"usedPercent"`
}
type NetworkInfo struct {
	//下载速度
	NetInSpeed uint64 `json:"netInSpeed"`
	//上传速度
	NetOutSpeed uint64 `json:"netOutSpeed"`
	//下载
	NetInTransfer uint64 `json:"netInTransfer"`
	//上传
	NetOutTransfer uint64 `json:"netOutTransfer"`
}
