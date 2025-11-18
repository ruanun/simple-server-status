package internal

import (
	"fmt"
	"strings"

	"github.com/ruanun/simple-server-status/pkg/model"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
)

// GetServerInfo 获取服务器信息
// hostIp: 服务器IP地址（可选，传空字符串表示未设置）
// hostLocation: 服务器地理位置（可选，传空字符串表示未设置）
func GetServerInfo(hostIp, hostLocation string) *model.ServerInfo {
	return &model.ServerInfo{
		//Name:              "win",
		HostInfo:          getHostInfo(),
		CpuInfo:           getCpuInfo(),
		VirtualMemoryInfo: getMemInfo(),
		SwapMemoryInfo:    getSwapMemInfo(),
		DiskInfo:          getDiskInfo(),
		NetworkInfo:       getNetInfo(),

		Ip:  hostIp,
		Loc: hostLocation,
	}
}

// @brief：耗时统计函数
func timeCost() func() {
	//start := time.Now()
	return func() {
		//tc := time.Since(start)
		//fmt.Printf("time cost = %v\n", tc)
	}
}

func getHostInfo() *model.HostInfo {
	defer timeCost()()

	info, err := host.Info()
	if err != nil {
		fmt.Println("get host info fail, error: ", err)
	}
	var hostInfo model.HostInfo
	hostInfo.KernelArch = info.KernelArch
	hostInfo.KernelVersion = info.KernelVersion
	hostInfo.VirtualizationSystem = info.VirtualizationSystem
	hostInfo.Uptime = info.Uptime
	hostInfo.BootTime = info.BootTime
	hostInfo.OS = info.OS
	hostInfo.Platform = info.Platform
	hostInfo.PlatformVersion = info.PlatformVersion
	hostInfo.PlatformFamily = info.PlatformFamily

	loadInfo, err := load.Avg()
	if err != nil {
		fmt.Println("get average load fail. err: ", err)
	}
	hostInfo.AvgStat = loadInfo
	return &hostInfo
}

func getCpuInfo() *model.CpuInfo {
	defer timeCost()()

	var cpuInfo model.CpuInfo

	ci, err := cpu.Info()
	if err != nil {
		println("cpu.Info error:", err)
	} else {
		cpuModelCount := make(map[string]int)
		for i := 0; i < len(ci); i++ {
			cpuModelCount[ci[i].ModelName]++
		}
		for m, count := range cpuModelCount {
			cpuInfo.Info = append(cpuInfo.Info, fmt.Sprintf("%s x %d ", m, count))
		}
	}
	cpuPercent, _ := cpu.Percent(0, false)
	cpuInfo.Percent = cpuPercent[0]
	return &cpuInfo
}

func getMemInfo() *model.VirtualMemoryInfo {
	defer timeCost()()

	memInfo, err := mem.VirtualMemory()
	if err != nil {
		fmt.Println("get memory info fail. err： ", err)
	}

	var memRet model.VirtualMemoryInfo
	memRet.Total = memInfo.Total
	//memRet.Available = memInfo.Available
	//memRet.Free = memInfo.Free
	memRet.Used = memInfo.Used
	memRet.UsedPercent = memInfo.UsedPercent
	return &memRet
}

func getSwapMemInfo() *model.SwapMemoryInfo {
	defer timeCost()()

	ms, err := mem.SwapMemory()
	if err != nil {
		println("mem.SwapMemory error:", err)
	}

	var swapInfo model.SwapMemoryInfo
	swapInfo.Total = ms.Total
	swapInfo.Free = ms.Free
	swapInfo.Used = ms.Used
	swapInfo.UsedPercent = ms.UsedPercent
	return &swapInfo
}

func getDiskInfo() *model.DiskInfo {
	defer timeCost()()

	diskPart, err := disk.Partitions(false)
	if err != nil {
		fmt.Println(err)
	}
	var diskInfo model.DiskInfo
	var total, used uint64

	for _, dp := range diskPart {
		diskUsed, _ := disk.Usage(dp.Mountpoint)
		//fmt.Printf("%s %d %f %d \n", usage.Path, usage.Total, usage.UsedPercent, usage.Used)
		fsType := strings.ToLower(dp.Fstype)
		// 不统计 K8s 的虚拟挂载点：https://github.com/shirou/gopsutil/issues/1007
		if isListContainsStr(expectDiskFsTypes, fsType) && !strings.Contains(dp.Mountpoint, "/var/lib/kubelet") {
			p := model.Partition{
				MountPoint: dp.Mountpoint, Fstype: dp.Fstype,
				Total: diskUsed.Total, Free: diskUsed.Free,
				Used: diskUsed.Used, UsedPercent: diskUsed.UsedPercent,
			}
			diskInfo.Partitions = append(diskInfo.Partitions, &p)
			total += diskUsed.Total
			used += diskUsed.Used
		}
	}
	diskInfo.Used = used
	diskInfo.Total = total
	//计算占用百分比
	diskInfo.UsedPercent = float64(used) / float64(total) * 100
	return &diskInfo
}

// StatNetworkSpeed 更新网络统计（线程安全）
func StatNetworkSpeed() {
	defer timeCost()()

	if err := globalNetworkStats.Update(); err != nil {
		fmt.Println("更新网络统计失败: ", err)
	}
}

// 网络信息  参考 nezha（线程安全）
func getNetInfo() *model.NetworkInfo {
	return globalNetworkStats.GetStats()
}

// FormatFileSize 字节的单位转换 保留两位小数
// 导出此函数以供外部使用，避免 unused 警告
func FormatFileSize(fileSize uint64) (size string) {
	if fileSize < 1024 {
		//return strconv.FormatInt(fileSize, 10) + "B"
		return fmt.Sprintf("%.2fB", float64(fileSize)/float64(1))
	} else if fileSize < (1024 * 1024) {
		return fmt.Sprintf("%.2fKB", float64(fileSize)/float64(1024))
	} else if fileSize < (1024 * 1024 * 1024) {
		return fmt.Sprintf("%.2fMB", float64(fileSize)/float64(1024*1024))
	} else if fileSize < (1024 * 1024 * 1024 * 1024) {
		return fmt.Sprintf("%.2fGB", float64(fileSize)/float64(1024*1024*1024))
	} else if fileSize < (1024 * 1024 * 1024 * 1024 * 1024) {
		return fmt.Sprintf("%.2fTB", float64(fileSize)/float64(1024*1024*1024*1024))
	} else { //if fileSize < (1024 * 1024 * 1024 * 1024 * 1024 * 1024)
		return fmt.Sprintf("%.2fPB", float64(fileSize)/float64(1024*1024*1024*1024*1024))
	}
}

var excludeNetInterfaces = []string{
	"lo", "tun", "docker", "veth", "br-", "vmbr", "vnet", "kube",
}
var expectDiskFsTypes = []string{
	"apfs", "ext4", "ext3", "ext2", "f2fs", "reiserfs", "jfs", "btrfs",
	"fuseblk", "zfs", "simfs", "ntfs", "fat32", "exfat", "xfs", "fuse.rclone",
}

// 全局网络统计收集器（线程安全）
var globalNetworkStats = NewNetworkStatsCollector(excludeNetInterfaces)

func isListContainsStr(list []string, str string) bool {
	for i := 0; i < len(list); i++ {
		if strings.Contains(str, list[i]) {
			return true
		}
	}
	return false
}
