// Package proto 定义 Agent 与 Dashboard 之间的消息协议。
package proto

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Version 当前协议版本
const Version = 1

// 消息类型
const (
	TypeHello  = "hello"
	TypeConfig = "config"
	TypeReport = "report"
	TypeStop   = "stop"
)

// ErrVersion 协议版本不匹配
var ErrVersion = errors.New("不支持的协议版本")

// Envelope 所有消息的外层结构
type Envelope struct {
	V    int             `json:"v"`
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// Hello Agent 建立连接后上报的静态信息
type Hello struct {
	OS              string `json:"os"`
	Platform        string `json:"platform"`
	PlatformVersion string `json:"platform_version"`
	Kernel          string `json:"kernel"`
	Arch            string `json:"arch"`
	Virtualization  string `json:"virtualization"`
	CPUModel        string `json:"cpu_model"`
	CPUCores        int    `json:"cpu_cores"`
	MemTotal        uint64 `json:"mem_total"`
	SwapTotal       uint64 `json:"swap_total"`
	DiskTotal       uint64 `json:"disk_total"`
	AgentVersion    string `json:"agent_version"`
	Country         string `json:"country"`
	IPv4            string `json:"ipv4,omitempty"`
	IPv6            string `json:"ipv6,omitempty"`
	BootID          string `json:"boot_id,omitempty"` // 本次开机的唯一标识，变化即重启；不支持的平台为空
}

// Config Dashboard 下发给 Agent 的采集参数
type Config struct {
	ReportInterval int      `json:"report_interval"`
	NICInclude     []string `json:"nic_include"`
	NICExclude     []string `json:"nic_exclude"`
	MountExclude   []string `json:"mount_exclude"`
	// FilterID 网卡过滤参数（nic_include / nic_exclude）的摘要，Agent 在之后的 report 中原样带回
	FilterID string `json:"filter_id"`
}

// Disk 单个分区的用量
type Disk struct {
	Mount  string `json:"mount"`
	FSType string `json:"fstype"`
	Total  uint64 `json:"total"`
	Used   uint64 `json:"used"`
}

// Report 周期上报的动态指标
type Report struct {
	TS          int64   `json:"ts"`
	CPU         float64 `json:"cpu"`
	Load1       float64 `json:"load1"`
	Load5       float64 `json:"load5"`
	Load15      float64 `json:"load15"`
	MemUsed     uint64  `json:"mem_used"`
	SwapUsed    uint64  `json:"swap_used"`
	DiskUsed    uint64  `json:"disk_used"`
	DiskTotal   uint64  `json:"disk_total"`
	Disks       []Disk  `json:"disks"`
	NetInSpeed  uint64  `json:"net_in_speed"`
	NetOutSpeed uint64  `json:"net_out_speed"`
	NetInTotal  uint64  `json:"net_in_total"`
	NetOutTotal uint64  `json:"net_out_total"`
	Procs       int     `json:"procs"`
	TCP         int     `json:"tcp"`
	UDP         int     `json:"udp"`
	Uptime      uint64  `json:"uptime"`
	// FilterID 采集本条数据时实际使用的网卡过滤器摘要（来自 config），用于识别流量统计口径变化
	FilterID string `json:"filter_id,omitempty"`
}

// Stop 通知 Agent 退出
type Stop struct {
	Reason string `json:"reason"`
}

// Normalize 把 nil 切片替换为空切片，保证 JSON 输出为 []
func (r *Report) Normalize() {
	if r.Disks == nil {
		r.Disks = []Disk{}
	}
}

// Normalize 把 nil 切片替换为空切片，保证 JSON 输出为 []
func (c *Config) Normalize() {
	if c.NICInclude == nil {
		c.NICInclude = []string{}
	}
	if c.NICExclude == nil {
		c.NICExclude = []string{}
	}
	if c.MountExclude == nil {
		c.MountExclude = []string{}
	}
}

// Encode 将消息编码为带外层结构的 JSON
func Encode(typ string, data any) ([]byte, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("编码 %s 消息失败: %w", typ, err)
	}
	return json.Marshal(Envelope{V: Version, Type: typ, Data: raw})
}

// Decode 解析外层结构并校验协议版本
func Decode(b []byte) (Envelope, error) {
	var env Envelope
	if err := json.Unmarshal(b, &env); err != nil {
		return env, fmt.Errorf("解析消息失败: %w", err)
	}
	if env.V != Version {
		return env, ErrVersion
	}
	return env, nil
}
