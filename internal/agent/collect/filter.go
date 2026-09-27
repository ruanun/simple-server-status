// Package collect 采集主机的静态信息与动态指标。
package collect

import "strings"

// DefaultNICExclude 默认排除的虚拟网卡名称前缀（参考 nezha；vethernet、loopback 为 Windows 虚拟网卡），全部小写存放
var DefaultNICExclude = []string{"tun", "docker", "veth", "br-", "vmbr", "vnet", "kube", "vethernet", "loopback"}

// defaultNICExact 默认排除的回环网卡（Linux lo、macOS lo0），按名称精确匹配（不区分大小写），
// 避免前缀 lo 误伤 Windows 上的 "Local Area Connection" 等物理网卡；与默认前缀列表一同被自定义 exclude 替换
var defaultNICExact = []string{"lo", "lo0"}

// allowedFS 参与磁盘统计的文件系统类型
var allowedFS = []string{
	"apfs", "ext4", "ext3", "ext2", "f2fs", "reiserfs", "jfs", "btrfs",
	"fuseblk", "zfs", "simfs", "ntfs", "fat32", "exfat", "xfs", "fuse.rclone",
}

// Filter 网卡与挂载点过滤规则
type Filter struct {
	NICInclude   []string
	NICExclude   []string
	MountExclude []string
}

// NICAllowed 判断网卡是否计入流量：配置了 include 时仅统计名称以其开头的网卡；
// 否则排除名称以 exclude 中任一前缀开头的网卡（exclude 为空时使用默认列表）。前缀匹配不区分大小写
func (f Filter) NICAllowed(name string) bool {
	name = strings.ToLower(name)
	if len(f.NICInclude) > 0 {
		for _, p := range f.NICInclude {
			if strings.HasPrefix(name, strings.ToLower(p)) {
				return true
			}
		}
		return false
	}
	exclude := f.NICExclude
	if len(exclude) == 0 {
		for _, n := range defaultNICExact {
			if name == n {
				return false
			}
		}
		exclude = DefaultNICExclude
	}
	for _, p := range exclude {
		if strings.HasPrefix(name, strings.ToLower(p)) {
			return false
		}
	}
	return true
}

// MountAllowed 判断分区是否计入磁盘统计
func (f Filter) MountAllowed(fstype, mount string) bool {
	fs := strings.ToLower(fstype)
	known := false
	for _, t := range allowedFS {
		if fs == t {
			known = true
			break
		}
	}
	if !known || strings.Contains(mount, "/var/lib/kubelet") {
		return false
	}
	for _, m := range f.MountExclude {
		m = strings.TrimSuffix(m, "/")
		if mount == m || strings.HasPrefix(mount, m+"/") {
			return false
		}
	}
	return true
}

// rate 计算速率（字节/秒）；计数器回退或间隔非正时返回 0
func rate(prev, cur uint64, elapsed float64) uint64 {
	if elapsed <= 0 || cur < prev {
		return 0
	}
	return uint64(float64(cur-prev) / elapsed)
}
