//go:build !linux

package collect

import (
	"github.com/ruanun/simple-server-status/internal/proto"
	gnet "github.com/shirou/gopsutil/v4/net"
)

// sampleConns 通过 gopsutil 统计 TCP/UDP 连接数（非 Linux 平台）
func (c *Collector) sampleConns(r *proto.Report) {
	if conns, err := gnet.Connections("tcp"); err != nil {
		c.warnf("tcp", err)
	} else {
		r.TCP = len(conns)
	}
	if conns, err := gnet.Connections("udp"); err != nil {
		c.warnf("udp", err)
	} else {
		r.UDP = len(conns)
	}
}
