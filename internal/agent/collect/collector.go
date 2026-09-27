package collect

import (
	"log/slog"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/ruanun/simple-server-status/internal/proto"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	gnet "github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
)

// Collector 采集主机指标；每项指标独立获取，失败时记 0 并限频告警
type Collector struct {
	mu      sync.Mutex
	filter  Filter
	lastIn  uint64
	lastOut uint64
	lastAt  time.Time
	now     func() time.Time
	log     *slog.Logger
	warned  map[string]time.Time
}

// New 创建采集器
func New(log *slog.Logger) *Collector {
	return &Collector{now: time.Now, log: log, warned: map[string]time.Time{}}
}

// SetFilter 更新过滤规则，并重置网速基线
func (c *Collector) SetFilter(f Filter) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.filter = f
	c.lastAt = time.Time{}
}

// warnf 同一采集项每 10 分钟最多告警一次（调用方需持有 c.mu 或处于单线程测试中）
func (c *Collector) warnf(item string, err error) {
	now := c.now()
	if t, ok := c.warned[item]; ok && now.Sub(t) < 10*time.Minute {
		return
	}
	c.warned[item] = now
	c.log.Warn("采集失败", "item", item, "err", err)
}

// Static 采集静态信息
func (c *Collector) Static(version string) proto.Hello {
	c.mu.Lock()
	defer c.mu.Unlock()
	h := proto.Hello{AgentVersion: version, Arch: runtime.GOARCH}
	if info, err := host.Info(); err != nil {
		c.warnf("host", err)
	} else {
		h.OS = info.OS
		h.Platform = info.Platform
		h.PlatformVersion = info.PlatformVersion
		h.Kernel = info.KernelVersion
		h.Virtualization = info.VirtualizationSystem
		if info.KernelArch != "" {
			h.Arch = info.KernelArch
		}
	}
	if infos, err := cpu.Info(); err != nil {
		c.warnf("cpu_info", err)
	} else if len(infos) > 0 {
		h.CPUModel = strings.TrimSpace(infos[0].ModelName)
	}
	if n, err := cpu.Counts(true); err != nil {
		c.warnf("cpu_count", err)
	} else {
		h.CPUCores = n
	}
	if vm, err := mem.VirtualMemory(); err != nil {
		c.warnf("mem", err)
	} else {
		h.MemTotal = vm.Total
	}
	if sw, err := mem.SwapMemory(); err != nil {
		c.warnf("swap", err)
	} else {
		h.SwapTotal = sw.Total
	}
	_, h.DiskTotal, _ = c.disks()
	return h
}

// Sample 采集一次动态指标
func (c *Collector) Sample() proto.Report {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	r := proto.Report{TS: now.Unix()}
	if ps, err := cpu.Percent(0, false); err != nil {
		c.warnf("cpu", err)
	} else if len(ps) > 0 {
		r.CPU = ps[0]
	}
	if l, err := load.Avg(); err != nil {
		c.warnf("load", err)
	} else {
		r.Load1, r.Load5, r.Load15 = l.Load1, l.Load5, l.Load15
	}
	if vm, err := mem.VirtualMemory(); err != nil {
		c.warnf("mem", err)
	} else {
		r.MemUsed = vm.Used
	}
	if sw, err := mem.SwapMemory(); err != nil {
		c.warnf("swap", err)
	} else {
		r.SwapUsed = sw.Used
	}
	r.Disks, r.DiskTotal, r.DiskUsed = c.disks()
	if up, err := host.Uptime(); err != nil {
		c.warnf("uptime", err)
	} else {
		r.Uptime = up
	}
	if pids, err := process.Pids(); err != nil {
		c.warnf("procs", err)
	} else {
		r.Procs = len(pids)
	}
	c.sampleConns(&r)
	c.sampleNet(now, &r)
	return r
}

// disks 返回允许统计的分区列表及总量、已用量（按设备与挂载点去重）
func (c *Collector) disks() (list []proto.Disk, total, used uint64) {
	list = []proto.Disk{}
	parts, err := disk.Partitions(false)
	if err != nil {
		c.warnf("disk", err)
		return list, 0, 0
	}
	seenMount := map[string]bool{}
	seenDevice := map[string]bool{}
	for _, p := range parts {
		if !c.filter.MountAllowed(p.Fstype, p.Mountpoint) || seenMount[p.Mountpoint] || (p.Device != "" && seenDevice[p.Device]) {
			continue
		}
		u, err := disk.Usage(p.Mountpoint)
		if err != nil {
			c.warnf("disk_usage:"+p.Mountpoint, err)
			continue
		}
		seenMount[p.Mountpoint] = true
		if p.Device != "" {
			seenDevice[p.Device] = true
		}
		list = append(list, proto.Disk{Mount: p.Mountpoint, FSType: p.Fstype, Total: u.Total, Used: u.Used})
		total += u.Total
		used += u.Used
	}
	return list, total, used
}

// sampleNet 统计允许网卡的累计流量，并根据上次采样计算速率
func (c *Collector) sampleNet(now time.Time, r *proto.Report) {
	ios, err := gnet.IOCounters(true)
	if err != nil {
		c.warnf("net", err)
		return
	}
	var in, out uint64
	for _, s := range ios {
		if c.filter.NICAllowed(s.Name) {
			in += s.BytesRecv
			out += s.BytesSent
		}
	}
	if !c.lastAt.IsZero() {
		elapsed := now.Sub(c.lastAt).Seconds()
		r.NetInSpeed = rate(c.lastIn, in, elapsed)
		r.NetOutSpeed = rate(c.lastOut, out, elapsed)
	}
	c.lastIn, c.lastOut, c.lastAt = in, out, now
	r.NetInTotal, r.NetOutTotal = in, out
}
