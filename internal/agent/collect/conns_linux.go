//go:build linux

package collect

import (
	"errors"
	"io/fs"
	"os"

	"github.com/ruanun/simple-server-status/internal/proto"
)

// sampleConns 读取 /proc/net/sockstat(6) 统计 TCP/UDP 连接数，避免每次遍历 /proc 下所有进程的 fd
func (c *Collector) sampleConns(r *proto.Report) {
	b, err := os.ReadFile("/proc/net/sockstat")
	if err != nil {
		c.warnf("conns", err)
		return
	}
	r.TCP, r.UDP = parseSockstat(string(b))
	b6, err := os.ReadFile("/proc/net/sockstat6")
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) { // 未启用 IPv6 时文件不存在
			c.warnf("conns6", err)
		}
		return
	}
	tcp6, udp6 := parseSockstat(string(b6))
	r.TCP += tcp6
	r.UDP += udp6
}
