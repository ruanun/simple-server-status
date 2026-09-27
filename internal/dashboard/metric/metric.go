// Package metric 定义历史图表使用的指标点及聚合方法。
package metric

import "github.com/ruanun/simple-server-status/internal/proto"

// Point 一个时间点的指标：cpu/mem/disk 为百分比，net_in/net_out 为字节每秒
type Point struct {
	TS     int64   `json:"ts"`
	CPU    float64 `json:"cpu"`
	Mem    float64 `json:"mem"`
	Disk   float64 `json:"disk"`
	NetIn  float64 `json:"net_in"`
	NetOut float64 `json:"net_out"`
	Load1  float64 `json:"load1"`
	TCP    float64 `json:"tcp"`
}

// Percent 安全计算百分比，total 为 0 时返回 0
func Percent(used, total uint64) float64 {
	if total == 0 {
		return 0
	}
	return float64(used) / float64(total) * 100
}

// FromReport 根据一次上报与内存总量计算指标点
func FromReport(r proto.Report, memTotal uint64) Point {
	return Point{
		TS:     r.TS,
		CPU:    r.CPU,
		Mem:    Percent(r.MemUsed, memTotal),
		Disk:   Percent(r.DiskUsed, r.DiskTotal),
		NetIn:  float64(r.NetInSpeed),
		NetOut: float64(r.NetOutSpeed),
		Load1:  r.Load1,
		TCP:    float64(r.TCP),
	}
}

// Average 对一组点逐字段取平均，结果时间戳为 ts
func Average(ts int64, pts []Point) Point {
	if len(pts) == 0 {
		return Point{}
	}
	var s Point
	for _, p := range pts {
		s.CPU += p.CPU
		s.Mem += p.Mem
		s.Disk += p.Disk
		s.NetIn += p.NetIn
		s.NetOut += p.NetOut
		s.Load1 += p.Load1
		s.TCP += p.TCP
	}
	n := float64(len(pts))
	return Point{TS: ts, CPU: s.CPU / n, Mem: s.Mem / n, Disk: s.Disk / n, NetIn: s.NetIn / n,
		NetOut: s.NetOut / n, Load1: s.Load1 / n, TCP: s.TCP / n}
}

// Downsample 按 bucket 秒分桶取平均；输入需按 ts 升序
func Downsample(pts []Point, bucket int64) []Point {
	out := []Point{}
	var cur []Point
	start := int64(-1)
	for _, p := range pts {
		b := p.TS - p.TS%bucket
		if b != start && len(cur) > 0 {
			out = append(out, Average(start, cur))
			cur = cur[:0]
		}
		start = b
		cur = append(cur, p)
	}
	if len(cur) > 0 {
		out = append(out, Average(start, cur))
	}
	return out
}
