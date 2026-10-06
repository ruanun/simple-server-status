package incident

import "github.com/ruanun/simple-server-status/internal/proto"

// Rebooted 比较新旧静态信息中的开机标识，变化即重启；任一方为空（首次上报、旧版 Agent 或不支持的平台）不判断。
// 开机标识由 Agent 所在系统生成，不依赖任何一方的时钟，时钟校准、漂移与上报延迟都不会造成误报
func Rebooted(old *proto.Hello, cur proto.Hello) bool {
	return old != nil && old.BootID != "" && cur.BootID != "" && old.BootID != cur.BootID
}

// IPChanges 比较新旧静态信息中的 IPv4 / IPv6，返回变化的项（键为 ipv4、ipv6，值为 [旧, 新]）。
// 任一方为空不算变化（公网地址探测失败时为空）
func IPChanges(old *proto.Hello, cur proto.Hello) map[string][2]string {
	out := map[string][2]string{}
	if old == nil {
		return out
	}
	for _, f := range []struct {
		key      string
		old, cur string
	}{{"ipv4", old.IPv4, cur.IPv4}, {"ipv6", old.IPv6, cur.IPv6}} {
		if f.old != "" && f.cur != "" && f.old != f.cur {
			out[f.key] = [2]string{f.old, f.cur}
		}
	}
	return out
}
