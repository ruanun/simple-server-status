package collect

import (
	"strconv"
	"strings"
)

// parseSockstat 解析 /proc/net/sockstat 或 /proc/net/sockstat6 的内容，
// 返回 TCP/TCP6 与 UDP/UDP6 行中 inuse 字段的值（同一内容中出现多行时求和）
func parseSockstat(s string) (tcp, udp int) {
	for _, line := range strings.Split(s, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		var dst *int
		switch fields[0] {
		case "TCP:", "TCP6:":
			dst = &tcp
		case "UDP:", "UDP6:":
			dst = &udp
		default:
			continue
		}
		for i := 1; i+1 < len(fields); i++ {
			if fields[i] == "inuse" {
				if n, err := strconv.Atoi(fields[i+1]); err == nil {
					*dst += n
				}
				break
			}
		}
	}
	return tcp, udp
}
