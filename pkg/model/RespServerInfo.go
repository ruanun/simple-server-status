package model

import (
	"strings"

	"github.com/shirou/gopsutil/v4/load"
)

type RespServerInfo struct {
	Name           string `json:"name"`           //name展示
	Group          string `json:"group"`          //组
	Id             string `json:"id"`             //服务器id
	LastReportTime int64  `json:"lastReportTime"` //最后上报时间

	Uptime   uint64 `json:"uptime"`   //服务器的uptime //单位秒
	Platform string `json:"platform"` //系统版型信息 ex: Windows 11 x64 ;platform+platformVersion

	CpuPercent  float64 `json:"cpuPercent"`  //cpu占用
	RAMPercent  float64 `json:"RAMPercent"`  //内存占用
	SWAPPercent float64 `json:"SWAPPercent"` //swap占用
	DiskPercent float64 `json:"diskPercent"` //硬盘占用

	NetInSpeed  uint64 `json:"netInSpeed"`  //下载速度
	NetOutSpeed uint64 `json:"netOutSpeed"` //上传速度

	Loc string `json:"loc"`

	HostInfo *RespHostData `json:"hostInfo"`

	IsOnline bool `json:"isOnline"` //是否在线
}

type RespHostData struct {
	CpuInfo []string      `json:"cpuInfo"` //cpu信息字符串描述
	AvgStat *load.AvgStat `json:"avgStat"` //load

	RAMTotal  uint64 `json:"RAMTotal"`
	RAMUsed   uint64 `json:"RAMUsed"`
	SwapTotal uint64 `json:"swapTotal"`
	SwapUsed  uint64 `json:"swapUsed"`

	DiskTotal      uint64       `json:"diskTotal"`      //总硬盘
	DiskUsed       uint64       `json:"diskUsed"`       //已使用
	DiskPartitions []*Partition `json:"diskPartitions"` //各个分区

	NetInTransfer  uint64 `json:"netInTransfer"`  //下载的流量
	NetOutTransfer uint64 `json:"netOutTransfer"` //上传的流量

	OS                   string `json:"os"`
	Platform             string `json:"platform"`
	PlatformVersion      string `json:"platformVersion"`
	VirtualizationSystem string `json:"virtualizationSystem"`
	KernelVersion        string `json:"kernelVersion"`
	KernelArch           string `json:"kernelArch"`
}

func isWin(serverInfo *ServerInfo) bool {
	return strings.Contains(serverInfo.HostInfo.Platform, "Windows")
}

func NewRespHostData(serverInfo *ServerInfo) *RespHostData {
	return &RespHostData{
		CpuInfo: serverInfo.CpuInfo.Info,
		AvgStat: serverInfo.HostInfo.AvgStat,

		RAMTotal:  serverInfo.VirtualMemoryInfo.Total,
		RAMUsed:   serverInfo.VirtualMemoryInfo.Used,
		SwapTotal: serverInfo.SwapMemoryInfo.Total,
		SwapUsed:  serverInfo.SwapMemoryInfo.Used,

		DiskTotal:      serverInfo.DiskInfo.Total,
		DiskUsed:       serverInfo.DiskInfo.Used,
		DiskPartitions: serverInfo.DiskInfo.Partitions,

		NetInTransfer:  serverInfo.NetworkInfo.NetInTransfer,
		NetOutTransfer: serverInfo.NetworkInfo.NetOutTransfer,

		OS:                   serverInfo.HostInfo.OS,
		Platform:             serverInfo.HostInfo.Platform,
		PlatformVersion:      serverInfo.HostInfo.PlatformVersion,
		VirtualizationSystem: serverInfo.HostInfo.VirtualizationSystem,
		KernelVersion:        serverInfo.HostInfo.KernelVersion,
		KernelArch:           serverInfo.HostInfo.KernelArch,
	}
}

func NewRespServerInfo(serverInfo *ServerInfo) *RespServerInfo {
	var platform string
	if isWin(serverInfo) {
		platform = serverInfo.HostInfo.Platform
	} else {
		platform = serverInfo.HostInfo.Platform + " " + serverInfo.HostInfo.PlatformVersion
	}
	return &RespServerInfo{
		Name:           serverInfo.Name,
		Group:          serverInfo.Group,
		Id:             serverInfo.Id,
		LastReportTime: serverInfo.LastReportTime,

		Uptime:   serverInfo.HostInfo.Uptime,
		Platform: platform,

		CpuPercent:  serverInfo.CpuInfo.Percent,
		RAMPercent:  serverInfo.VirtualMemoryInfo.UsedPercent,
		SWAPPercent: serverInfo.SwapMemoryInfo.UsedPercent,
		DiskPercent: serverInfo.DiskInfo.UsedPercent,
		NetInSpeed:  serverInfo.NetworkInfo.NetInSpeed,
		NetOutSpeed: serverInfo.NetworkInfo.NetOutSpeed,

		Loc: serverInfo.Loc,

		//其他信息
		HostInfo: NewRespHostData(serverInfo),
	}
}
