package collect

import "testing"

func TestNICAllowed(t *testing.T) {
	cases := []struct {
		name string
		f    Filter
		nic  string
		want bool
	}{
		{"默认允许物理网卡", Filter{}, "eth0", true},
		{"默认排除回环", Filter{}, "lo", false},
		{"默认排除 docker", Filter{}, "docker0", false},
		{"前缀匹配不误伤 wlo1", Filter{}, "wlo1", true},
		{"include 命中", Filter{NICInclude: []string{"eth"}}, "eth1", true},
		{"include 未命中", Filter{NICInclude: []string{"eth"}}, "ens3", false},
		{"自定义 exclude 覆盖默认列表", Filter{NICExclude: []string{"ens"}}, "lo", true},
		{"自定义 exclude 命中", Filter{NICExclude: []string{"ens"}}, "ens3", false},
		{"默认排除 Windows Hyper-V 虚拟网卡", Filter{}, "vEthernet (WSL)", false},
		{"默认排除 Windows 回环", Filter{}, "Loopback Pseudo-Interface 1", false},
		{"默认允许 Windows 物理网卡", Filter{}, "Ethernet", true},
		{"默认允许 Local Area Connection", Filter{}, "Local Area Connection", true},
		{"默认允许 Local Area Connection* 1", Filter{}, "Local Area Connection* 1", true},
		{"默认排除 macOS 回环 lo0", Filter{}, "lo0", false},
		{"回环精确匹配不区分大小写", Filter{}, "LO", false},
		{"lo 为精确匹配而非前缀", Filter{}, "lan0", true},
		{"include 不区分大小写", Filter{NICInclude: []string{"eth"}}, "Ethernet 2", true},
		{"exclude 不区分大小写", Filter{NICExclude: []string{"WLAN"}}, "wlan0", false},
	}
	for _, c := range cases {
		if got := c.f.NICAllowed(c.nic); got != c.want {
			t.Errorf("%s: NICAllowed(%q) = %v，期望 %v", c.name, c.nic, got, c.want)
		}
	}
}

func TestMountAllowed(t *testing.T) {
	boot := Filter{MountExclude: []string{"/boot"}}
	cases := []struct {
		name   string
		f      Filter
		fstype string
		mount  string
		want   bool
	}{
		{"ext4 根分区", Filter{}, "ext4", "/", true},
		{"tmpfs 不统计", Filter{}, "tmpfs", "/run", false},
		{"Windows NTFS", Filter{}, "NTFS", "C:", true},
		{"排除 kubelet 挂载", Filter{}, "ext4", "/var/lib/kubelet/pods/1", false},
		{"排除指定挂载点", boot, "ext4", "/boot", false},
		{"排除挂载点的子目录", boot, "ext4", "/boot/efi", false},
		{"前缀相同但不是子目录", boot, "ext4", "/bootx", true},
	}
	for _, c := range cases {
		if got := c.f.MountAllowed(c.fstype, c.mount); got != c.want {
			t.Errorf("%s: MountAllowed(%q, %q) = %v，期望 %v", c.name, c.fstype, c.mount, got, c.want)
		}
	}
}

func TestRate(t *testing.T) {
	if got := rate(100, 300, 2); got != 100 {
		t.Errorf("rate = %d，期望 100", got)
	}
	if got := rate(300, 100, 2); got != 0 {
		t.Errorf("计数器回退时应为 0，实际 %d", got)
	}
	if got := rate(0, 100, 0); got != 0 {
		t.Errorf("间隔为 0 时应为 0，实际 %d", got)
	}
}
