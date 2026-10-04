package incident

import "github.com/ruanun/simple-server-status/internal/proto"

// bootTolerance 开机时间的允许误差（秒）：上报延迟与时钟抖动不算重启
const bootTolerance = 120

// RebootDetail 重启事件的详情
type RebootDetail struct {
	BootAt int64 `json:"boot_at"`
}

// BootCheck 比较记录的开机时间 prev 与本次计算的 boot（均为 Unix 秒）。
// save 表示需要更新记录；rebooted 表示发生了重启（prev 为 0 时只记录，不算重启）
func BootCheck(prev, boot int64) (save, rebooted bool) {
	if prev != 0 && boot >= prev-bootTolerance && boot <= prev+bootTolerance {
		return false, false
	}
	return true, prev != 0 && boot > prev+bootTolerance
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
