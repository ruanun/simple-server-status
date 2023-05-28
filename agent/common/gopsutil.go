package common

import (
	"fmt"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
	"simple-server-status/model"
	"strings"
	"time"
)

func GetServerInfo() *model.ServerInfo {
	return &model.ServerInfo{
		//Name:              "win",
		HostInfo:          getHostInfo(),
		CpuInfo:           getCpuInfo(),
		VirtualMemoryInfo: getMemInfo(),
		SwapMemoryInfo:    getSwapMemInfo(),
		DiskInfo:          getDiskInfo(),
		NetworkInfo:       getNetInfo(),
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
	hostInfo.AvgStat = &model.AvgStat{loadInfo.Load1, loadInfo.Load5, loadInfo.Load15}
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
	cpuPercent, _ := cpu.Percent(time.Second, true)
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

// 网络信息  参考 nezha
func getNetInfo() *model.NetworkInfo {
	defer timeCost()()

	netIOs, err := net.IOCounters(true)
	if err != nil {
		fmt.Println("get net io counters failed: ", err)
		return nil
	}
	var innerNetInTransfer, innerNetOutTransfer uint64

	for _, v := range netIOs {
		if isListContainsStr(excludeNetInterfaces, v.Name) {
			continue
		}
		innerNetInTransfer += v.BytesRecv
		innerNetOutTransfer += v.BytesSent
	}
	now := uint64(time.Now().Unix())
	diff := now - lastUpdateNetStats
	if diff > 0 {
		//计算速度
		netInSpeed = (innerNetInTransfer - netInTransfer) / diff
		netOutSpeed = (innerNetOutTransfer - netOutTransfer) / diff
	}
	netInTransfer = innerNetInTransfer
	netOutTransfer = innerNetOutTransfer
	lastUpdateNetStats = now

	//fmt.Println("===================================================")
	//fmt.Println("netInTransfer: " + formatFileSize(netInTransfer))
	//fmt.Println("netOutTransfer: " + formatFileSize(netOutTransfer))
	//fmt.Println("netInSpeed: " + formatFileSize(netInSpeed))
	//fmt.Println("netOutSpeed: " + formatFileSize(netOutSpeed))
	netInfo := model.NetworkInfo{netInSpeed, netOutSpeed, netInTransfer, netOutTransfer}
	return &netInfo
}

// 以1024作为基数
func ByteCountIEC(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB",
		float64(b)/float64(div), "KMGTPE"[exp])
}

// 字节的单位转换 保留两位小数
func formatFileSize(fileSize uint64) (size string) {
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
var (
	netInSpeed, netOutSpeed, netInTransfer, netOutTransfer, lastUpdateNetStats uint64
)

func isListContainsStr(list []string, str string) bool {
	for i := 0; i < len(list); i++ {
		if strings.Contains(str, list[i]) {
			return true
		}
	}
	return false
}
