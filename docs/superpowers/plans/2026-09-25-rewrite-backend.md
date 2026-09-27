# 后端重写（计划 1/3）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 用干净的新结构重写 Agent 与 Dashboard 后端（协议、采集、存储、实时状态、历史降采样、月流量、鉴权、全部 HTTP/WS 接口、命令行），做到可独立运行与测试。

**Architecture:** Agent 通过 WebSocket 长连接向 Dashboard 发送带版本号的 `hello`/`report` 消息，Dashboard 下发 `config`/`stop`。Dashboard 以内存 `hub` 保存实时状态，`history` 做 1 分钟 / 10 分钟分级降采样写入 SQLite，`traffic` 累计月流量；`api` 层（gin）提供公开接口、浏览器 WebSocket 与需 JWT 的管理接口，前端产物通过 `go:embed` 嵌入。

**Tech Stack:** Go 1.24、gin、coder/websocket、modernc.org/sqlite（database/sql）、slog + lumberjack、cobra、gopsutil v4、golang-jwt v5、bcrypt、yaml.v3。

**Spec:** `docs/superpowers/specs/2026-09-25-rewrite-design.md`

**后续计划：** 计划 2（前端）、计划 3（构建发布、安装脚本、Docker、文档、仓库清理）在本计划完成后编写。

## Global Constraints

- `go.mod` 保持 `go 1.24.0`；新增依赖固定版本：`github.com/spf13/cobra v1.10.1`、`github.com/coder/websocket v1.8.14`、`modernc.org/sqlite v1.44.3`；沿用 `gin v1.10.0`、`gopsutil/v4 v4.24.11`、`golang-jwt/jwt/v5 v5.3.0`、`lumberjack.v2 v2.2.1`、`yaml.v3 v3.0.1`、`golang.org/x/crypto v0.31.0`。
- 禁止引入：viper、zap、gin-contrib/zap、melody、gorilla/websocket、gorm、glebarez/sqlite。
- 代码注释、日志、错误信息使用简体中文；JSON 字段一律 snake_case；响应中的切片输出 `[]` 而不是 `null`。
- HTTP 响应：成功 `200 {"data": ...}`；失败 4xx/5xx `{"error":{"code":"snake_case","message":"中文说明"}}`。
- 环境变量前缀 `SSS_`，优先级：命令行 > 环境变量 > YAML（仅 Agent）> 默认值。
- 每个任务结束前运行 `go vet ./...` 与 `go test ./...`，均须通过；本机有 `golangci-lint` 时运行 `golangci-lint run --timeout=5m ./...` 并修复报告的问题。
- 允许提交（用户已授权），提交信息用简体中文；不得改动 `data/`、根目录 `sss-agent.yaml`。
- 所有命令在仓库根目录 `D:\Code\m\simple-server-status` 下用 Git Bash 执行。

## Review Focus

- **Agent 重连时旧连接的延迟退出**：新连接接入后，旧连接的清理不能把服务器标记为离线或删掉新连接的注册（Task 7 会话号测试、Task 11 顶替测试）。
- **Agent 重启导致累计流量计数器归零**：当月流量不能出现负数或丢失，归零后应以新计数值作为增量（Task 9）。
- **Dashboard 重启 / 服务器离线时的流量展示**：当月用量从数据库恢复，离线服务器仍显示已用流量（Task 9 恢复测试）。
- **从未上报或缺字段的服务器**：公开接口返回 `metrics:null`、`static:null`，分区等列表为 `[]`，不能报错（Task 1 Normalize、Task 12 未上报详情测试）。
- **隐藏服务器泄露**：列表、详情、历史、浏览器 WebSocket 四个入口在未登录时都不得返回隐藏服务器，公开响应不得含 `secret`、`last_ip`（Task 12）。

---

### Task 1: 清理旧后端并建立 proto、logx、cliutil 基础包

**Files:**
- Delete: `cmd/`、`internal/`、`pkg/`、`scripts/e2e-agent/`、`scripts/e2e-fixture/`、`scripts/config_yaml_test.go`、`scripts/install_agent_test.go`
- Create: `internal/proto/proto.go`、`internal/proto/proto_test.go`
- Create: `internal/logx/logx.go`、`internal/logx/logx_test.go`
- Create: `internal/cliutil/env.go`、`internal/cliutil/env_test.go`
- Modify: `go.mod`、`go.sum`

**Interfaces:**
- Produces:
  - `proto.Version = 1`；`proto.TypeHello/TypeConfig/TypeReport/TypeStop`；`proto.ErrVersion`
  - `type proto.Envelope{V int; Type string; Data json.RawMessage}`
  - `type proto.Hello`、`proto.Config`、`proto.Disk`、`proto.Report`、`proto.Stop`（字段见代码）
  - `func proto.Encode(typ string, data any) ([]byte, error)`；`func proto.Decode(b []byte) (proto.Envelope, error)`
  - `func (r *proto.Report) Normalize()`；`func (c *proto.Config) Normalize()`
  - `func logx.ParseLevel(s string) (slog.Level, error)`；`func logx.New(level, file string) (*slog.Logger, func() error, error)`
  - `func cliutil.ApplyEnv(fs *pflag.FlagSet, prefix string) error`

- [ ] **Step 1: 删除旧后端代码**

```bash
git rm -r -q cmd internal pkg scripts/e2e-agent scripts/e2e-fixture scripts/config_yaml_test.go scripts/install_agent_test.go
git ls-files '*.go'
```

Expected: 第二条命令无输出（仓库中已无 Go 文件）。

- [ ] **Step 2: 写 proto 的失败测试**

`internal/proto/proto_test.go`：

```go
package proto

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	b, err := Encode(TypeReport, Report{CPU: 12.5, Disks: []Disk{{Mount: "/", Total: 10, Used: 5}}})
	if err != nil {
		t.Fatal(err)
	}
	env, err := Decode(b)
	if err != nil {
		t.Fatal(err)
	}
	if env.V != Version || env.Type != TypeReport {
		t.Fatalf("外层结构错误: %+v", env)
	}
	var r Report
	if err := json.Unmarshal(env.Data, &r); err != nil {
		t.Fatal(err)
	}
	if r.CPU != 12.5 || len(r.Disks) != 1 || r.Disks[0].Mount != "/" {
		t.Fatalf("内容错误: %+v", r)
	}
}

func TestDecodeRejectsOtherVersion(t *testing.T) {
	_, err := Decode([]byte(`{"v":2,"type":"report","data":{}}`))
	if !errors.Is(err, ErrVersion) {
		t.Fatalf("期望 ErrVersion，实际 %v", err)
	}
}

func TestDecodeRejectsInvalidJSON(t *testing.T) {
	if _, err := Decode([]byte("not json")); err == nil {
		t.Fatal("期望解析错误")
	}
}

func TestFieldsAreSnakeCase(t *testing.T) {
	b, err := Encode(TypeHello, Hello{CPUModel: "x", MemTotal: 1})
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, key := range []string{`"cpu_model":"x"`, `"mem_total":1`, `"agent_version"`, `"v":1`} {
		if !strings.Contains(s, key) {
			t.Errorf("缺少字段 %s: %s", key, s)
		}
	}
}

func TestNormalizeOutputsEmptyArrays(t *testing.T) {
	var r Report
	r.Normalize()
	b, _ := json.Marshal(r)
	if !strings.Contains(string(b), `"disks":[]`) {
		t.Fatalf("disks 应为 []: %s", b)
	}
	var c Config
	c.Normalize()
	b, _ = json.Marshal(c)
	for _, key := range []string{`"nic_include":[]`, `"nic_exclude":[]`, `"mount_exclude":[]`} {
		if !strings.Contains(string(b), key) {
			t.Errorf("缺少 %s: %s", key, b)
		}
	}
}
```

- [ ] **Step 3: 运行测试确认失败**

Run: `go test ./internal/proto/`
Expected: FAIL，提示 `undefined: Encode` 等。

- [ ] **Step 4: 实现 proto**

`internal/proto/proto.go`：

```go
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
}

// Config Dashboard 下发给 Agent 的采集参数
type Config struct {
	ReportInterval int      `json:"report_interval"`
	NICInclude     []string `json:"nic_include"`
	NICExclude     []string `json:"nic_exclude"`
	MountExclude   []string `json:"mount_exclude"`
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
```

- [ ] **Step 5: 写 logx 与 cliutil 的失败测试**

`internal/logx/logx_test.go`：

```go
package logx

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseLevel(t *testing.T) {
	cases := map[string]slog.Level{
		"":        slog.LevelInfo,
		"info":    slog.LevelInfo,
		"DEBUG":   slog.LevelDebug,
		"warn":    slog.LevelWarn,
		"warning": slog.LevelWarn,
		"error":   slog.LevelError,
	}
	for in, want := range cases {
		got, err := ParseLevel(in)
		if err != nil || got != want {
			t.Errorf("ParseLevel(%q) = %v, %v；期望 %v", in, got, err, want)
		}
	}
	if _, err := ParseLevel("verbose"); err == nil {
		t.Error("未知级别应报错")
	}
}

func TestNewWritesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.log")
	log, closeFn, err := New("info", path)
	if err != nil {
		t.Fatal(err)
	}
	log.Info("你好")
	if err := closeFn(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "你好") {
		t.Fatalf("日志文件内容不含预期文本: %s", b)
	}
}

func TestNewStdoutOnly(t *testing.T) {
	_, closeFn, err := New("debug", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := closeFn(); err != nil {
		t.Fatal(err)
	}
}
```

`internal/cliutil/env_test.go`：

```go
package cliutil

import (
	"testing"

	"github.com/spf13/pflag"
)

func TestApplyEnvRespectsCommandLine(t *testing.T) {
	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	fs.String("listen", ":8900", "")
	fs.String("data-dir", "./data", "")
	if err := fs.Parse([]string{"--data-dir", "cli"}); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SSS_LISTEN", ":9000")
	t.Setenv("SSS_DATA_DIR", "env")
	if err := ApplyEnv(fs, "SSS"); err != nil {
		t.Fatal(err)
	}
	if v, _ := fs.GetString("listen"); v != ":9000" {
		t.Errorf("listen 应取环境变量，实际 %q", v)
	}
	if v, _ := fs.GetString("data-dir"); v != "cli" {
		t.Errorf("data-dir 应保留命令行值，实际 %q", v)
	}
	if !fs.Changed("listen") {
		t.Error("由环境变量设置的参数应标记为 Changed")
	}
}

func TestApplyEnvInvalidValue(t *testing.T) {
	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	fs.Int("port", 1, "")
	if err := fs.Parse(nil); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SSS_PORT", "abc")
	if err := ApplyEnv(fs, "SSS"); err == nil {
		t.Fatal("非法值应报错")
	}
}
```

- [ ] **Step 6: 运行测试确认失败**

Run: `go test ./internal/logx/ ./internal/cliutil/`
Expected: FAIL（`undefined: ParseLevel`、`undefined: ApplyEnv`，或缺少 pflag 依赖）。

- [ ] **Step 7: 实现 logx 与 cliutil**

`internal/logx/logx.go`：

```go
// Package logx 初始化 slog 日志，可选同时写入按大小轮转的文件。
package logx

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"gopkg.in/natefinch/lumberjack.v2"
)

// ParseLevel 解析日志级别，空字符串视为 info
func ParseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	}
	return slog.LevelInfo, fmt.Errorf("未知日志级别: %q", s)
}

// New 创建日志器；file 为空时只输出到标准输出。返回的关闭函数用于关闭日志文件
func New(level, file string) (*slog.Logger, func() error, error) {
	lv, err := ParseLevel(level)
	if err != nil {
		return nil, nil, err
	}
	var w io.Writer = os.Stdout
	closeFn := func() error { return nil }
	if file != "" {
		lj := &lumberjack.Logger{Filename: file, MaxSize: 10, MaxBackups: 5, MaxAge: 30, Compress: true}
		w = io.MultiWriter(os.Stdout, lj)
		closeFn = lj.Close
	}
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: lv})), closeFn, nil
}
```

`internal/cliutil/env.go`：

```go
// Package cliutil 提供命令行辅助函数。
package cliutil

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/pflag"
)

// ApplyEnv 对命令行未显式设置的参数，若存在环境变量 PREFIX_NAME（名称转大写，- 替换为 _），则用其值设置
func ApplyEnv(fs *pflag.FlagSet, prefix string) error {
	var firstErr error
	fs.VisitAll(func(f *pflag.Flag) {
		if f.Changed || firstErr != nil {
			return
		}
		key := prefix + "_" + strings.ToUpper(strings.ReplaceAll(f.Name, "-", "_"))
		if v, ok := os.LookupEnv(key); ok {
			if err := fs.Set(f.Name, v); err != nil {
				firstErr = fmt.Errorf("环境变量 %s 的值无效: %w", key, err)
			}
		}
	})
	return firstErr
}
```

- [ ] **Step 8: 整理依赖并运行测试**

```bash
go get github.com/spf13/cobra@v1.10.1 gopkg.in/natefinch/lumberjack.v2@v2.2.1
go mod tidy
go vet ./...
go test ./...
grep -E '^go |viper|zap|melody|gorilla|gorm|glebarez' go.mod
```

Expected: 测试 PASS；最后一条只输出 `go 1.24.0`（旧依赖已被 tidy 移除）。若 `cobra` 因暂未被引用而被 tidy 移除属正常，Task 4 会再次引入。

- [ ] **Step 9: 提交**

```bash
git add internal go.mod go.sum
git status --short
git commit -m "refactor: 移除旧后端代码，新增 proto/logx/cliutil 基础包"
```

---

### Task 2: Agent 指标采集（collect）

**Files:**
- Create: `internal/agent/collect/filter.go`、`internal/agent/collect/filter_test.go`
- Create: `internal/agent/collect/collector.go`、`internal/agent/collect/collector_test.go`
- Create: `internal/agent/collect/country.go`、`internal/agent/collect/country_test.go`

**Interfaces:**
- Consumes: `proto.Hello`、`proto.Report`、`proto.Disk`（Task 1）
- Produces:
  - `type collect.Filter struct{ NICInclude, NICExclude, MountExclude []string }`
  - `func (f collect.Filter) NICAllowed(name string) bool`；`func (f collect.Filter) MountAllowed(fstype, mount string) bool`
  - `var collect.DefaultNICExclude []string`
  - `type collect.Collector`；`func collect.New(log *slog.Logger) *collect.Collector`
  - `func (c *Collector) Static(version string) proto.Hello`；`func (c *Collector) Sample() proto.Report`；`func (c *Collector) SetFilter(f Filter)`
  - `var collect.TraceURL string`；`func collect.DetectCountry(ctx context.Context, client *http.Client, url string) (string, error)`；`func collect.ParseTraceLoc(body string) string`

- [ ] **Step 1: 写过滤规则与速率计算的失败测试**

`internal/agent/collect/filter_test.go`：

```go
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
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/agent/collect/`
Expected: FAIL（`undefined: Filter`、`undefined: rate`）。

- [ ] **Step 3: 实现过滤规则**

`internal/agent/collect/filter.go`：

```go
// Package collect 采集主机的静态信息与动态指标。
package collect

import "strings"

// DefaultNICExclude 默认排除的虚拟网卡名称前缀（参考 nezha）
var DefaultNICExclude = []string{"lo", "tun", "docker", "veth", "br-", "vmbr", "vnet", "kube"}

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
// 否则排除名称以 exclude 中任一前缀开头的网卡（exclude 为空时使用默认列表）
func (f Filter) NICAllowed(name string) bool {
	if len(f.NICInclude) > 0 {
		for _, p := range f.NICInclude {
			if strings.HasPrefix(name, p) {
				return true
			}
		}
		return false
	}
	exclude := f.NICExclude
	if len(exclude) == 0 {
		exclude = DefaultNICExclude
	}
	for _, p := range exclude {
		if strings.HasPrefix(name, p) {
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
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/agent/collect/ -run 'TestNICAllowed|TestMountAllowed|TestRate' -v`
Expected: PASS。

- [ ] **Step 5: 写采集器与国家探测的失败测试**

`internal/agent/collect/collector_test.go`：

```go
package collect

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestCollectorSmoke(t *testing.T) {
	c := New(slog.New(slog.DiscardHandler))
	h := c.Static("test")
	if h.AgentVersion != "test" || h.CPUCores <= 0 || h.MemTotal == 0 || h.Arch == "" {
		t.Fatalf("静态信息异常: %+v", h)
	}
	r := c.Sample()
	if r.TS == 0 || r.Disks == nil {
		t.Fatalf("上报异常: %+v", r)
	}
	if r.NetInSpeed != 0 || r.NetOutSpeed != 0 {
		t.Fatal("首次采样的网速应为 0")
	}
}

func TestWarnRateLimited(t *testing.T) {
	var buf bytes.Buffer
	c := New(slog.New(slog.NewTextHandler(&buf, nil)))
	now := time.Unix(0, 0)
	c.now = func() time.Time { return now }
	c.warnf("cpu", errors.New("x"))
	c.warnf("cpu", errors.New("x"))
	now = now.Add(11 * time.Minute)
	c.warnf("cpu", errors.New("x"))
	if n := strings.Count(buf.String(), "采集失败"); n != 2 {
		t.Fatalf("期望 2 条告警，实际 %d:\n%s", n, buf.String())
	}
}
```

`internal/agent/collect/country_test.go`：

```go
package collect

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseTraceLoc(t *testing.T) {
	cases := map[string]string{
		"fl=1\nloc=jp\nip=1.1.1.1": "JP",
		"loc=XX":                   "",
		"":                         "",
		"ip=1\r\nloc=US\r\n":       "US",
	}
	for in, want := range cases {
		if got := ParseTraceLoc(in); got != want {
			t.Errorf("ParseTraceLoc(%q) = %q，期望 %q", in, got, want)
		}
	}
}

func TestDetectCountry(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "ip=1.1.1.1\nloc=US\n")
	}))
	defer srv.Close()
	cc, err := DetectCountry(context.Background(), srv.Client(), srv.URL)
	if err != nil || cc != "US" {
		t.Fatalf("DetectCountry = %q, %v", cc, err)
	}
}
```

- [ ] **Step 6: 运行测试确认失败**

Run: `go test ./internal/agent/collect/`
Expected: FAIL（`undefined: New`、`undefined: ParseTraceLoc`）。

- [ ] **Step 7: 实现采集器**

`internal/agent/collect/collector.go`：

```go
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
```

`internal/agent/collect/country.go`：

```go
package collect

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// TraceURL Cloudflare trace 地址，返回内容中的 loc= 即国家代码
var TraceURL = "https://www.cloudflare.com/cdn-cgi/trace"

// DetectCountry 通过 Cloudflare trace 探测本机出口所在国家代码
func DetectCountry(ctx context.Context, client *http.Client, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求 trace 失败: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return "", fmt.Errorf("读取 trace 失败: %w", err)
	}
	return ParseTraceLoc(string(body)), nil
}

// ParseTraceLoc 从 trace 内容中提取两位国家代码，无效时返回空字符串
func ParseTraceLoc(body string) string {
	for _, line := range strings.Split(body, "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "loc="); ok {
			v = strings.ToUpper(strings.TrimSpace(v))
			if len(v) == 2 && v != "XX" {
				return v
			}
		}
	}
	return ""
}
```

- [ ] **Step 8: 整理依赖并运行测试**

```bash
go get github.com/shirou/gopsutil/v4@v4.24.11
go mod tidy
go vet ./...
go test ./internal/agent/collect/ -v
```

Expected: PASS（`TestCollectorSmoke` 在本机真实采集，Windows 与 Linux 均应通过）。

- [ ] **Step 9: 提交**

```bash
git add internal/agent/collect go.mod go.sum
git commit -m "feat(agent): 重写指标采集，修复空指针、越界与除零问题"
```

---

### Task 3: Agent 配置与连接客户端

**Files:**
- Create: `internal/agent/config.go`、`internal/agent/config_test.go`
- Create: `internal/agent/client.go`、`internal/agent/client_test.go`

**Interfaces:**
- Consumes: `proto.*`（Task 1）；`collect.Filter`（Task 2）
- Produces:
  - `type agent.Config struct{ Dashboard, ID, Secret string; DetectCountry bool; LogLevel, LogFile string }`（yaml 标签：`dashboard`、`id`、`secret`、`detect_country`、`log_level`、`log_file`）
  - `var agent.DefaultConfigPaths []string`；`func agent.DefaultConfig() agent.Config`；`func agent.LoadConfig(path string) (agent.Config, error)`；`func (c agent.Config) Validate() error`；`func agent.WSURL(dashboard string) (string, error)`
  - `type agent.Sampler interface{ Static(version string) proto.Hello; Sample() proto.Report; SetFilter(f collect.Filter) }`
  - `var agent.ErrStopped error`
  - `func agent.NewClient(cfg agent.Config, s agent.Sampler, log *slog.Logger, version string) (*agent.Client, error)`
  - `func (c *agent.Client) SetCountry(cc string)`；`func (c *agent.Client) Run(ctx context.Context) error`（ctx 结束返回 nil；收到 stop 返回 `ErrStopped`）
  - `func agent.Backoff(attempt int, unauthorized bool, rnd func() float64) time.Duration`

- [ ] **Step 1: 写配置的失败测试**

`internal/agent/config_test.go`：

```go
package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWSURL(t *testing.T) {
	cases := map[string]string{
		"http://a:8900":       "ws://a:8900/api/agent/ws",
		"https://a.com/sub/":  "wss://a.com/sub/api/agent/ws",
		"ws://a":              "ws://a/api/agent/ws",
		" https://a.com?x=1 ": "wss://a.com/api/agent/ws",
	}
	for in, want := range cases {
		got, err := WSURL(in)
		if err != nil || got != want {
			t.Errorf("WSURL(%q) = %q, %v；期望 %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"a.com", "ftp://a", "http://"} {
		if _, err := WSURL(bad); err == nil {
			t.Errorf("WSURL(%q) 应报错", bad)
		}
	}
}

func TestLoadConfigFromFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.yaml")
	if err := os.WriteFile(p, []byte("dashboard: http://x\nid: i\nsecret: s\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Dashboard != "http://x" || cfg.ID != "i" || cfg.Secret != "s" {
		t.Fatalf("配置内容错误: %+v", cfg)
	}
	if !cfg.DetectCountry || cfg.LogLevel != "info" {
		t.Fatalf("未配置的字段应保留默认值: %+v", cfg)
	}
	if _, err := LoadConfig(filepath.Join(dir, "missing.yaml")); err == nil {
		t.Fatal("显式指定的文件不存在时应报错")
	}
}

func TestLoadConfigDefaultPathsMissing(t *testing.T) {
	t.Chdir(t.TempDir())
	old := DefaultConfigPaths
	DefaultConfigPaths = []string{"sss-agent.yaml"}
	t.Cleanup(func() { DefaultConfigPaths = old })
	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg != DefaultConfig() {
		t.Fatalf("应返回默认配置: %+v", cfg)
	}
}

func TestValidate(t *testing.T) {
	ok := Config{Dashboard: "http://x", ID: "i", Secret: "s"}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []Config{
		{ID: "i", Secret: "s"},
		{Dashboard: "http://x", Secret: "s"},
		{Dashboard: "http://x", ID: "i"},
		{Dashboard: "x", ID: "i", Secret: "s"},
	} {
		if err := bad.Validate(); err == nil {
			t.Errorf("配置 %+v 应校验失败", bad)
		}
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/agent/`
Expected: FAIL（`undefined: WSURL` 等）。

- [ ] **Step 3: 实现配置**

`internal/agent/config.go`：

```go
// Package agent 实现探针 Agent：读取配置、连接 Dashboard 并周期上报。
package agent

import (
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config Agent 配置
type Config struct {
	Dashboard     string `yaml:"dashboard"`
	ID            string `yaml:"id"`
	Secret        string `yaml:"secret"`
	DetectCountry bool   `yaml:"detect_country"`
	LogLevel      string `yaml:"log_level"`
	LogFile       string `yaml:"log_file"`
}

// DefaultConfigPaths 未指定 --config 时依次查找的配置文件
var DefaultConfigPaths = []string{"sss-agent.yaml", "/etc/sss/sss-agent.yaml"}

// DefaultConfig 默认配置
func DefaultConfig() Config {
	return Config{DetectCountry: true, LogLevel: "info"}
}

// LoadConfig 读取 YAML 配置。path 为空时按默认路径查找，全部不存在则返回默认配置
func LoadConfig(path string) (Config, error) {
	cfg := DefaultConfig()
	paths := DefaultConfigPaths
	if path != "" {
		paths = []string{path}
	}
	for _, p := range paths {
		b, err := os.ReadFile(p) //nolint:gosec // 配置文件路径由用户指定
		if errors.Is(err, fs.ErrNotExist) && path == "" {
			continue
		}
		if err != nil {
			return cfg, fmt.Errorf("读取配置 %s 失败: %w", p, err)
		}
		if err := yaml.Unmarshal(b, &cfg); err != nil {
			return cfg, fmt.Errorf("解析配置 %s 失败: %w", p, err)
		}
		return cfg, nil
	}
	return cfg, nil
}

// Validate 校验必填项
func (c Config) Validate() error {
	if c.Dashboard == "" {
		return errors.New("缺少 dashboard 地址")
	}
	if c.ID == "" || c.Secret == "" {
		return errors.New("缺少 id 或 secret")
	}
	_, err := WSURL(c.Dashboard)
	return err
}

// WSURL 将 Dashboard 地址转换为 Agent WebSocket 地址
func WSURL(dashboard string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(dashboard))
	if err != nil {
		return "", fmt.Errorf("dashboard 地址无效: %w", err)
	}
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	case "ws", "wss":
	default:
		return "", fmt.Errorf("dashboard 地址需以 http:// 或 https:// 开头: %q", dashboard)
	}
	if u.Host == "" {
		return "", fmt.Errorf("dashboard 地址缺少主机: %q", dashboard)
	}
	u.Path = strings.TrimSuffix(u.Path, "/") + "/api/agent/ws"
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}
```

- [ ] **Step 4: 运行配置测试确认通过**

Run: `go get gopkg.in/yaml.v3@v3.0.1 && go mod tidy && go test ./internal/agent/ -v`
Expected: PASS。

- [ ] **Step 5: 写客户端的失败测试**

`internal/agent/client_test.go`：

```go
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/ruanun/simple-server-status/internal/agent/collect"
	"github.com/ruanun/simple-server-status/internal/proto"
)

type fakeSampler struct {
	mu     sync.Mutex
	filter collect.Filter
}

func (f *fakeSampler) Static(v string) proto.Hello { return proto.Hello{AgentVersion: v, CPUCores: 2} }
func (f *fakeSampler) Sample() proto.Report        { return proto.Report{CPU: 5} }
func (f *fakeSampler) SetFilter(x collect.Filter) {
	f.mu.Lock()
	f.filter = x
	f.mu.Unlock()
}

func TestClientHandshakeReportAndStop(t *testing.T) {
	gotAuth := make(chan string, 1)
	gotHello := make(chan proto.Hello, 1)
	gotReport := make(chan proto.Report, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth <- r.Header.Get("Authorization")
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		ctx := r.Context()
		_, b, err := conn.Read(ctx)
		if err != nil {
			return
		}
		env, _ := proto.Decode(b)
		var h proto.Hello
		_ = json.Unmarshal(env.Data, &h)
		gotHello <- h
		cfg, _ := proto.Encode(proto.TypeConfig, proto.Config{ReportInterval: 1, NICInclude: []string{"eth"}})
		_ = conn.Write(ctx, websocket.MessageText, cfg)
		_, b, err = conn.Read(ctx)
		if err != nil {
			return
		}
		env, _ = proto.Decode(b)
		var rep proto.Report
		_ = json.Unmarshal(env.Data, &rep)
		gotReport <- rep
		stop, _ := proto.Encode(proto.TypeStop, proto.Stop{Reason: "deleted"})
		_ = conn.Write(ctx, websocket.MessageText, stop)
		_, _, _ = conn.Read(ctx) // 等待客户端关闭连接
	}))
	defer srv.Close()

	fs := &fakeSampler{}
	c, err := NewClient(Config{Dashboard: srv.URL, ID: "id1", Secret: "sec"}, fs, slog.New(slog.DiscardHandler), "test")
	if err != nil {
		t.Fatal(err)
	}
	c.SetCountry("JP")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := c.Run(ctx); !errors.Is(err, ErrStopped) {
		t.Fatalf("期望 ErrStopped，实际 %v", err)
	}
	if a := <-gotAuth; a != "Bearer id1:sec" {
		t.Errorf("鉴权头错误: %q", a)
	}
	if h := <-gotHello; h.AgentVersion != "test" || h.Country != "JP" {
		t.Errorf("hello 内容错误: %+v", h)
	}
	if rep := <-gotReport; rep.CPU != 5 || rep.Disks == nil {
		t.Errorf("report 内容错误: %+v", rep)
	}
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if len(fs.filter.NICInclude) != 1 || fs.filter.NICInclude[0] != "eth" {
		t.Errorf("未应用下发的过滤规则: %+v", fs.filter)
	}
}

func TestSessionUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "no", http.StatusUnauthorized)
	}))
	defer srv.Close()
	c, err := NewClient(Config{Dashboard: srv.URL, ID: "a", Secret: "b"}, &fakeSampler{}, slog.New(slog.DiscardHandler), "t")
	if err != nil {
		t.Fatal(err)
	}
	err = c.session(context.Background())
	var ue unauthorizedError
	if !errors.As(err, &ue) {
		t.Fatalf("期望 unauthorizedError，实际 %v", err)
	}
}

func TestRunReturnsNilOnCancel(t *testing.T) {
	c, err := NewClient(Config{Dashboard: "http://127.0.0.1:1", ID: "a", Secret: "b"}, &fakeSampler{}, slog.New(slog.DiscardHandler), "t")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	if err := c.Run(ctx); err != nil {
		t.Fatalf("ctx 结束时应返回 nil，实际 %v", err)
	}
}

func TestBackoff(t *testing.T) {
	half := func() float64 { return 0.5 }
	if d := Backoff(0, false, half); d != time.Second {
		t.Errorf("attempt 0 = %v，期望 1s", d)
	}
	if d := Backoff(3, false, half); d != 8*time.Second {
		t.Errorf("attempt 3 = %v，期望 8s", d)
	}
	if d := Backoff(20, false, half); d != time.Minute {
		t.Errorf("attempt 20 = %v，期望 60s 封顶", d)
	}
	if d := Backoff(0, false, func() float64 { return 0 }); d != 800*time.Millisecond {
		t.Errorf("最小抖动 = %v，期望 800ms", d)
	}
	if d := Backoff(0, true, half); d != 5*time.Minute {
		t.Errorf("鉴权失败 = %v，期望 5m", d)
	}
}
```

- [ ] **Step 6: 运行测试确认失败**

Run: `go get github.com/coder/websocket@v1.8.14 && go test ./internal/agent/`
Expected: FAIL（`undefined: NewClient` 等）。

- [ ] **Step 7: 实现客户端**

`internal/agent/client.go`：

```go
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/ruanun/simple-server-status/internal/agent/collect"
	"github.com/ruanun/simple-server-status/internal/proto"
)

// Sampler 指标采集接口（collect.Collector 实现）
type Sampler interface {
	Static(version string) proto.Hello
	Sample() proto.Report
	SetFilter(f collect.Filter)
}

// ErrStopped Dashboard 通知 Agent 退出
var ErrStopped = errors.New("Dashboard 要求停止")

// unauthorizedError Dashboard 返回 401
type unauthorizedError struct{}

func (unauthorizedError) Error() string { return "鉴权失败（401），请检查 id 与 secret" }

// Client 负责与 Dashboard 保持连接并周期上报
type Client struct {
	cfg     Config
	url     string
	s       Sampler
	log     *slog.Logger
	version string
	country string
}

// NewClient 创建客户端
func NewClient(cfg Config, s Sampler, log *slog.Logger, version string) (*Client, error) {
	u, err := WSURL(cfg.Dashboard)
	if err != nil {
		return nil, err
	}
	return &Client{cfg: cfg, url: u, s: s, log: log, version: version}, nil
}

// SetCountry 设置随 hello 上报的国家代码
func (c *Client) SetCountry(cc string) { c.country = cc }

// Run 保持连接直到 ctx 结束（返回 nil）或收到 stop（返回 ErrStopped）
func (c *Client) Run(ctx context.Context) error {
	attempt := 0
	for {
		started := time.Now()
		err := c.session(ctx)
		if errors.Is(err, ErrStopped) {
			return err
		}
		if ctx.Err() != nil {
			return nil
		}
		if time.Since(started) > time.Minute {
			attempt = 0
		}
		var ue unauthorizedError
		wait := Backoff(attempt, errors.As(err, &ue), rand.Float64) //nolint:gosec // 重连抖动无需密码学随机数
		attempt++
		c.log.Warn("连接断开，稍后重连", "err", err, "wait", wait.String())
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(wait):
		}
	}
}

// Backoff 计算第 attempt 次重连的等待时间：1s 起指数增长、60s 封顶、±20% 抖动；鉴权失败固定 5 分钟
func Backoff(attempt int, unauthorized bool, rnd func() float64) time.Duration {
	if unauthorized {
		return 5 * time.Minute
	}
	d := time.Second << min(attempt, 6)
	if d > time.Minute {
		d = time.Minute
	}
	return time.Duration(float64(d) * (0.8 + 0.4*rnd()))
}

// session 建立一次连接并运行到断开
func (c *Client) session(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	h := http.Header{}
	h.Set("Authorization", "Bearer "+c.cfg.ID+":"+c.cfg.Secret)
	dctx, dcancel := context.WithTimeout(ctx, 15*time.Second)
	conn, resp, err := websocket.Dial(dctx, c.url, &websocket.DialOptions{HTTPHeader: h})
	dcancel()
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusUnauthorized {
			return unauthorizedError{}
		}
		return fmt.Errorf("连接失败: %w", err)
	}
	defer func() { _ = conn.CloseNow() }()
	conn.SetReadLimit(64 << 10)
	c.log.Info("已连接 Dashboard", "url", c.url)

	hello := c.s.Static(c.version)
	hello.Country = c.country
	if err := write(ctx, conn, proto.TypeHello, hello); err != nil {
		return err
	}

	cfgCh := make(chan proto.Config, 1)
	errCh := make(chan error, 1)
	go func() { errCh <- c.readLoop(ctx, conn, cfgCh) }()

	interval := 2 * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			_ = conn.Close(websocket.StatusNormalClosure, "")
			return ctx.Err()
		case err := <-errCh:
			return err
		case cfg := <-cfgCh:
			c.s.SetFilter(collect.Filter{NICInclude: cfg.NICInclude, NICExclude: cfg.NICExclude, MountExclude: cfg.MountExclude})
			if d := clampInterval(cfg.ReportInterval); d != interval {
				interval = d
				ticker.Reset(d)
			}
			c.log.Info("已应用 Dashboard 下发的参数", "interval", interval.String())
		case <-ticker.C:
			r := c.s.Sample()
			r.Normalize()
			if err := write(ctx, conn, proto.TypeReport, r); err != nil {
				return err
			}
		}
	}
}

// readLoop 处理 Dashboard 下发的消息
func (c *Client) readLoop(ctx context.Context, conn *websocket.Conn, cfgCh chan<- proto.Config) error {
	for {
		_, b, err := conn.Read(ctx)
		if err != nil {
			return fmt.Errorf("读取失败: %w", err)
		}
		env, err := proto.Decode(b)
		if err != nil {
			c.log.Warn("忽略无法解析的消息", "err", err)
			continue
		}
		switch env.Type {
		case proto.TypeConfig:
			var cfg proto.Config
			if err := json.Unmarshal(env.Data, &cfg); err != nil {
				c.log.Warn("忽略无效的 config 消息", "err", err)
				continue
			}
			select {
			case cfgCh <- cfg:
			case <-ctx.Done():
				return ctx.Err()
			}
		case proto.TypeStop:
			var s proto.Stop
			_ = json.Unmarshal(env.Data, &s)
			c.log.Warn("收到停止指令", "reason", s.Reason)
			return ErrStopped
		default:
			c.log.Debug("忽略未知消息", "type", env.Type)
		}
	}
}

// clampInterval 把下发的上报间隔限制在 1–60 秒，未设置时为 2 秒
func clampInterval(sec int) time.Duration {
	if sec < 1 {
		sec = 2
	}
	if sec > 60 {
		sec = 60
	}
	return time.Duration(sec) * time.Second
}

func write(ctx context.Context, conn *websocket.Conn, typ string, data any) error {
	b, err := proto.Encode(typ, data)
	if err != nil {
		return err
	}
	wctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := conn.Write(wctx, websocket.MessageText, b); err != nil {
		return fmt.Errorf("发送 %s 失败: %w", typ, err)
	}
	return nil
}
```

- [ ] **Step 8: 运行测试确认通过**

```bash
go mod tidy
go vet ./...
go test ./internal/agent/... -v
```

Expected: PASS。

- [ ] **Step 9: 提交**

```bash
git add internal/agent go.mod go.sum
git commit -m "feat(agent): 新增配置加载与 WebSocket 上报客户端"
```

---

### Task 4: sss-agent 命令行

**Files:**
- Create: `cmd/sss-agent/main.go`、`cmd/sss-agent/main_test.go`
- Modify: `configs/sss-agent.yaml.example`（整体替换）
- Modify: `Makefile`（`build-agent` 目标）

**Interfaces:**
- Consumes: `cliutil.ApplyEnv`、`logx.New`（Task 1）；`collect.New`、`collect.DetectCountry`、`collect.TraceURL`（Task 2）；`agent.*`（Task 3）
- Produces: `func newRootCmd() *cobra.Command`；`func resolveConfig(fs *pflag.FlagSet) (agent.Config, error)`；可执行文件 `sss-agent`（子命令 `run`（默认）、`version`）

- [ ] **Step 1: 写失败测试**

`cmd/sss-agent/main_test.go`：

```go
package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveConfigPriority(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.yaml")
	if err := os.WriteFile(p, []byte("dashboard: http://file\nid: file-id\nsecret: file-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SSS_ID", "env-id")
	t.Setenv("SSS_SECRET", "env-secret")
	fs := newRootCmd().PersistentFlags()
	if err := fs.Parse([]string{"--config", p, "--id", "cli-id"}); err != nil {
		t.Fatal(err)
	}
	cfg, err := resolveConfig(fs)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Dashboard != "http://file" {
		t.Errorf("dashboard 应来自文件，实际 %q", cfg.Dashboard)
	}
	if cfg.ID != "cli-id" {
		t.Errorf("id 应来自命令行，实际 %q", cfg.ID)
	}
	if cfg.Secret != "env-secret" {
		t.Errorf("secret 应来自环境变量，实际 %q", cfg.Secret)
	}
}

func TestResolveConfigRequiresDashboard(t *testing.T) {
	t.Chdir(t.TempDir())
	fs := newRootCmd().PersistentFlags()
	if err := fs.Parse([]string{"--config", ""}); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveConfig(fs); err == nil {
		t.Fatal("缺少 dashboard 时应报错")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./cmd/sss-agent/`
Expected: FAIL（`undefined: newRootCmd`）。

- [ ] **Step 3: 实现命令行**

`cmd/sss-agent/main.go`：

```go
// sss-agent 是 Simple Server Status 的探针程序。
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ruanun/simple-server-status/internal/agent"
	"github.com/ruanun/simple-server-status/internal/agent/collect"
	"github.com/ruanun/simple-server-status/internal/cliutil"
	"github.com/ruanun/simple-server-status/internal/logx"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// version 由构建时 -ldflags "-X main.version=..." 注入
var version = "dev"

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:          "sss-agent",
		Short:        "Simple Server Status 探针 Agent",
		SilenceUsage: true,
		RunE:         runAgent,
	}
	pf := root.PersistentFlags()
	pf.String("config", "", "配置文件路径（默认依次查找 ./sss-agent.yaml、/etc/sss/sss-agent.yaml）")
	pf.String("dashboard", "", "Dashboard 地址，如 https://status.example.com")
	pf.String("id", "", "服务器 ID")
	pf.String("secret", "", "服务器密钥")
	pf.Bool("detect-country", true, "是否通过 Cloudflare trace 探测国家代码")
	pf.String("log-level", "info", "日志级别：debug/info/warn/error")
	pf.String("log-file", "", "日志文件路径，留空只输出到标准输出")

	root.AddCommand(
		&cobra.Command{Use: "run", Short: "运行 Agent（默认）", RunE: runAgent},
		&cobra.Command{Use: "version", Short: "显示版本", Run: func(*cobra.Command, []string) { fmt.Println(version) }},
	)
	return root
}

// resolveConfig 合并配置：命令行 > 环境变量 > YAML > 默认值
func resolveConfig(fs *pflag.FlagSet) (agent.Config, error) {
	if err := cliutil.ApplyEnv(fs, "SSS"); err != nil {
		return agent.Config{}, err
	}
	path, _ := fs.GetString("config")
	cfg, err := agent.LoadConfig(path)
	if err != nil {
		return cfg, err
	}
	if fs.Changed("dashboard") {
		cfg.Dashboard, _ = fs.GetString("dashboard")
	}
	if fs.Changed("id") {
		cfg.ID, _ = fs.GetString("id")
	}
	if fs.Changed("secret") {
		cfg.Secret, _ = fs.GetString("secret")
	}
	if fs.Changed("detect-country") {
		cfg.DetectCountry, _ = fs.GetBool("detect-country")
	}
	if fs.Changed("log-level") {
		cfg.LogLevel, _ = fs.GetString("log-level")
	}
	if fs.Changed("log-file") {
		cfg.LogFile, _ = fs.GetString("log-file")
	}
	return cfg, cfg.Validate()
}

func runAgent(cmd *cobra.Command, _ []string) error {
	cfg, err := resolveConfig(cmd.Flags())
	if err != nil {
		return err
	}
	log, closeLog, err := logx.New(cfg.LogLevel, cfg.LogFile)
	if err != nil {
		return err
	}
	defer func() { _ = closeLog() }()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	client, err := agent.NewClient(cfg, collect.New(log), log, version)
	if err != nil {
		return err
	}
	if cfg.DetectCountry {
		dctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		cc, err := collect.DetectCountry(dctx, http.DefaultClient, collect.TraceURL)
		cancel()
		if err != nil {
			log.Warn("探测国家代码失败", "err", err)
		} else {
			client.SetCountry(cc)
		}
	}
	log.Info("Agent 启动", "version", version, "dashboard", cfg.Dashboard)
	err = client.Run(ctx)
	if errors.Is(err, agent.ErrStopped) {
		log.Info("Agent 已按 Dashboard 指令退出")
		return nil
	}
	return err
}
```

- [ ] **Step 4: 更新示例配置与 Makefile**

`configs/sss-agent.yaml.example` 整体替换为：

```yaml
# Simple Server Status Agent 配置
# 命令行参数与环境变量（SSS_DASHBOARD、SSS_ID、SSS_SECRET 等）优先于本文件

# Dashboard 地址
dashboard: "https://status.example.com"
# 在 Dashboard 后台创建服务器后获得
id: "your-server-id"
secret: "your-secret"
# 是否通过 Cloudflare trace 探测国家代码
detect_country: true
# 日志级别：debug/info/warn/error
log_level: "info"
# 日志文件路径，留空只输出到标准输出
log_file: ""
```

`Makefile` 中把

```makefile
	go build -o $(BIN_DIR)/$(BINARY_AGENT) ./cmd/agent
```

替换为

```makefile
	go build -o $(BIN_DIR)/$(BINARY_AGENT) ./cmd/sss-agent
```

- [ ] **Step 5: 运行测试并构建**

```bash
go get github.com/spf13/cobra@v1.10.1
go mod tidy
go vet ./...
go test ./...
go build -o bin/sss-agent$(go env GOEXE) ./cmd/sss-agent && ./bin/sss-agent$(go env GOEXE) version
```

Expected: 测试 PASS；最后输出 `dev`。

- [ ] **Step 6: 提交**

```bash
git add cmd/sss-agent configs/sss-agent.yaml.example Makefile go.mod go.sum
git commit -m "feat(agent): 新增基于 cobra 的 sss-agent 命令行"
```

---

### Task 5: Dashboard 存储：迁移、服务器、用户、设置

**Files:**
- Create: `internal/dashboard/randx/randx.go`、`internal/dashboard/randx/randx_test.go`
- Create: `internal/dashboard/store/store.go`、`internal/dashboard/store/migrations/001_init.sql`
- Create: `internal/dashboard/store/servers.go`、`internal/dashboard/store/users.go`、`internal/dashboard/store/settings.go`
- Test: `internal/dashboard/store/store_test.go`

**Interfaces:**
- Consumes: `proto.Hello`（Task 1）
- Produces:
  - `func randx.String(n int) string`（字母数字，crypto/rand）
  - `var store.ErrNotFound`；`func store.Open(path string) (*store.Store, error)`；`func (s *Store) Close() error`
  - `type store.Server`（字段与 JSON 标签见代码，`secret` 明文、`static_info *proto.Hello`）
  - `func (s *Store) ListServers(ctx) ([]Server, error)`（按 sort、name 排序，空时返回 `[]Server{}`）
  - `func (s *Store) GetServer(ctx, id string) (Server, error)`；`CreateServer(ctx, *Server) error`（自动生成 12 位 ID 与 32 位 secret）；`UpdateServer(ctx, Server) error`；`DeleteServer(ctx, id string) error`；`SetServerOrder(ctx, ids []string) error`；`ResetSecret(ctx, id string) (string, error)`；`SetStaticInfo(ctx, id, ip string, h proto.Hello) error`；`SetLastSeen(ctx, id string, ts int64) error`；`UpsertServers(ctx, list []Server) error`
  - `type store.User{ID int64; Username, PasswordHash string; TokenVersion int; CreatedAt int64}`；`CountUsers`、`CreateUser(ctx, *User)`、`GetUserByName`、`GetUser(ctx, id int64)`、`SetPassword(ctx, id int64, hash string) error`（token_version+1）
  - `type store.Settings{SiteTitle string; ShowPrice bool; DefaultReportInterval int; InstallScriptBase string}`（JSON：`site_title`、`show_price`、`default_report_interval`、`install_script_base`）；`const store.DefaultInstallScriptBase`；`func store.DefaultSettings() Settings`；`GetSettings(ctx) (Settings, error)`；`SaveSettings(ctx, Settings) error`；`JWTSecret(ctx) ([]byte, error)`

- [ ] **Step 1: 写失败测试**

`internal/dashboard/randx/randx_test.go`：

```go
package randx

import "testing"

func TestString(t *testing.T) {
	a, b := String(32), String(32)
	if len(a) != 32 || a == b {
		t.Fatalf("随机串异常: %q %q", a, b)
	}
	for _, r := range a {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			t.Fatalf("包含非字母数字字符: %q", a)
		}
	}
}
```

`internal/dashboard/store/store_test.go`：

```go
package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/ruanun/simple-server-status/internal/proto"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestOpenMigratesIdempotently(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var v int
	if err := s.db.QueryRow(`SELECT MAX(version) FROM schema_version`).Scan(&v); err != nil || v != 1 {
		t.Fatalf("schema_version = %d, %v", v, err)
	}
}

func TestServerCRUD(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	list, err := s.ListServers(ctx)
	if err != nil || list == nil || len(list) != 0 {
		t.Fatalf("空列表应为 []: %v %v", list, err)
	}

	price := 9.9
	a := Server{Name: "b-server", Price: &price}
	if err := s.CreateServer(ctx, &a); err != nil {
		t.Fatal(err)
	}
	if len(a.ID) != 12 || len(a.Secret) != 32 {
		t.Fatalf("ID/Secret 长度错误: %q %q", a.ID, a.Secret)
	}
	if a.TrafficMode != "sum" || a.TrafficResetDay != 1 || a.ReportInterval != 2 {
		t.Fatalf("默认值错误: %+v", a)
	}
	b := Server{Name: "a-server", Sort: 1}
	if err := s.CreateServer(ctx, &b); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetServer(ctx, a.ID)
	if err != nil || got.Name != "b-server" || got.Price == nil || *got.Price != 9.9 || got.NICInclude == nil {
		t.Fatalf("GetServer = %+v, %v", got, err)
	}

	got.Name = "renamed"
	got.Hidden = true
	got.NICInclude = []string{"eth"}
	if err := s.UpdateServer(ctx, got); err != nil {
		t.Fatal(err)
	}
	got, _ = s.GetServer(ctx, a.ID)
	if got.Name != "renamed" || !got.Hidden || len(got.NICInclude) != 1 {
		t.Fatalf("更新未生效: %+v", got)
	}

	if err := s.SetServerOrder(ctx, []string{b.ID, a.ID}); err != nil {
		t.Fatal(err)
	}
	list, _ = s.ListServers(ctx)
	if len(list) != 2 || list[0].ID != b.ID {
		t.Fatalf("排序错误: %+v", list)
	}

	sec, err := s.ResetSecret(ctx, a.ID)
	if err != nil || sec == a.Secret || len(sec) != 32 {
		t.Fatalf("ResetSecret = %q, %v", sec, err)
	}

	if err := s.DeleteServer(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetServer(ctx, a.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("删除后应 ErrNotFound，实际 %v", err)
	}
	if err := s.DeleteServer(ctx, a.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("重复删除应 ErrNotFound，实际 %v", err)
	}
	if err := s.UpdateServer(ctx, Server{ID: "missing", Name: "x"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("更新不存在的服务器应 ErrNotFound，实际 %v", err)
	}
}

func TestStaticInfoAndLastSeen(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	a := Server{Name: "a"}
	if err := s.CreateServer(ctx, &a); err != nil {
		t.Fatal(err)
	}
	if err := s.SetStaticInfo(ctx, a.ID, "1.2.3.4", proto.Hello{OS: "linux", MemTotal: 100}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetLastSeen(ctx, a.ID, 123); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetServer(ctx, a.ID)
	if got.StaticInfo == nil || got.StaticInfo.OS != "linux" || got.LastIP != "1.2.3.4" || got.LastSeen != 123 {
		t.Fatalf("静态信息错误: %+v", got)
	}
}

func TestUpsertServers(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	a := Server{Name: "a"}
	if err := s.CreateServer(ctx, &a); err != nil {
		t.Fatal(err)
	}
	_ = s.SetStaticInfo(ctx, a.ID, "ip", proto.Hello{OS: "linux"})
	a.Name = "a2"
	a.Secret = "imported-secret"
	if err := s.UpsertServers(ctx, []Server{a, {ID: "newid000001", Name: "b", Secret: "sb"}}); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetServer(ctx, a.ID)
	if got.Name != "a2" || got.Secret != "imported-secret" || got.StaticInfo == nil {
		t.Fatalf("覆盖结果错误（静态信息应保留）: %+v", got)
	}
	nb, err := s.GetServer(ctx, "newid000001")
	if err != nil || nb.Secret != "sb" || nb.CreatedAt == 0 {
		t.Fatalf("新增结果错误: %+v %v", nb, err)
	}
}

func TestUsers(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if n, _ := s.CountUsers(ctx); n != 0 {
		t.Fatalf("初始用户数应为 0，实际 %d", n)
	}
	u := User{Username: "admin", PasswordHash: "h1"}
	if err := s.CreateUser(ctx, &u); err != nil || u.ID == 0 {
		t.Fatalf("CreateUser: %+v %v", u, err)
	}
	if err := s.SetPassword(ctx, u.ID, "h2"); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetUserByName(ctx, "admin")
	if err != nil || got.PasswordHash != "h2" || got.TokenVersion != 1 {
		t.Fatalf("GetUserByName = %+v, %v", got, err)
	}
	if _, err := s.GetUser(ctx, 999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("不存在的用户应 ErrNotFound，实际 %v", err)
	}
}

func TestSettings(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	st, err := s.GetSettings(ctx)
	if err != nil || st != DefaultSettings() {
		t.Fatalf("默认设置错误: %+v %v", st, err)
	}
	st.SiteTitle = "我的探针"
	st.ShowPrice = true
	st.DefaultReportInterval = 5
	if err := s.SaveSettings(ctx, st); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetSettings(ctx)
	if got != st {
		t.Fatalf("保存后读取不一致: %+v", got)
	}
	k1, err := s.JWTSecret(ctx)
	if err != nil || len(k1) != 32 {
		t.Fatalf("JWTSecret = %x, %v", k1, err)
	}
	k2, _ := s.JWTSecret(ctx)
	if string(k1) != string(k2) {
		t.Fatal("JWTSecret 应保持不变")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/dashboard/...`
Expected: FAIL（`undefined: String`、`undefined: Open` 等）。

- [ ] **Step 3: 实现 randx 与迁移脚本**

`internal/dashboard/randx/randx.go`：

```go
// Package randx 生成密码学安全的随机字符串。
package randx

import "crypto/rand"

const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// String 生成长度为 n 的字母数字随机串
func String(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("读取系统随机数失败: " + err.Error())
	}
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b)
}
```

`internal/dashboard/store/migrations/001_init.sql`：

```sql
CREATE TABLE servers (
  id                TEXT PRIMARY KEY,
  name              TEXT NOT NULL,
  secret            TEXT NOT NULL,
  grp               TEXT NOT NULL DEFAULT '',
  country           TEXT NOT NULL DEFAULT '',
  sort              INTEGER NOT NULL DEFAULT 0,
  hidden            INTEGER NOT NULL DEFAULT 0,
  price             REAL,
  currency          TEXT NOT NULL DEFAULT '',
  billing_cycle     TEXT NOT NULL DEFAULT '',
  expire_at         INTEGER,
  traffic_limit     INTEGER,
  traffic_mode      TEXT NOT NULL DEFAULT 'sum',
  traffic_reset_day INTEGER NOT NULL DEFAULT 1,
  report_interval   INTEGER NOT NULL DEFAULT 2,
  nic_include       TEXT NOT NULL DEFAULT '[]',
  nic_exclude       TEXT NOT NULL DEFAULT '[]',
  mount_exclude     TEXT NOT NULL DEFAULT '[]',
  static_info       TEXT,
  last_ip           TEXT NOT NULL DEFAULT '',
  last_seen         INTEGER NOT NULL DEFAULT 0,
  created_at        INTEGER NOT NULL,
  updated_at        INTEGER NOT NULL
);

CREATE TABLE users (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  username      TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  token_version INTEGER NOT NULL DEFAULT 0,
  created_at    INTEGER NOT NULL
);

CREATE TABLE settings (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);

CREATE TABLE metrics_1m (
  server_id TEXT NOT NULL,
  ts        INTEGER NOT NULL,
  cpu       REAL NOT NULL,
  mem       REAL NOT NULL,
  disk      REAL NOT NULL,
  net_in    REAL NOT NULL,
  net_out   REAL NOT NULL,
  load1     REAL NOT NULL,
  tcp       REAL NOT NULL,
  PRIMARY KEY (server_id, ts)
) WITHOUT ROWID;

CREATE TABLE metrics_10m (
  server_id TEXT NOT NULL,
  ts        INTEGER NOT NULL,
  cpu       REAL NOT NULL,
  mem       REAL NOT NULL,
  disk      REAL NOT NULL,
  net_in    REAL NOT NULL,
  net_out   REAL NOT NULL,
  load1     REAL NOT NULL,
  tcp       REAL NOT NULL,
  PRIMARY KEY (server_id, ts)
) WITHOUT ROWID;

CREATE INDEX idx_metrics_1m_ts ON metrics_1m (ts);
CREATE INDEX idx_metrics_10m_ts ON metrics_10m (ts);

CREATE TABLE traffic_monthly (
  server_id      TEXT NOT NULL,
  period         TEXT NOT NULL,
  in_bytes       INTEGER NOT NULL DEFAULT 0,
  out_bytes      INTEGER NOT NULL DEFAULT 0,
  last_in_total  INTEGER NOT NULL DEFAULT 0,
  last_out_total INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (server_id, period)
) WITHOUT ROWID;
```

- [ ] **Step 4: 实现 store**

`internal/dashboard/store/store.go`：

```go
// Package store 封装 Dashboard 的 SQLite 存储。
package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"strconv"

	_ "modernc.org/sqlite" // 纯 Go SQLite 驱动
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// ErrNotFound 记录不存在
var ErrNotFound = errors.New("记录不存在")

// Store SQLite 存储
type Store struct {
	db *sql.DB
}

// execer 同时适配 *sql.DB 与 *sql.Tx
type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// Open 打开数据库并执行迁移
func Open(path string) (*Store, error) {
	dsn := path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// Close 关闭数据库
func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL)`); err != nil {
		return fmt.Errorf("创建版本表失败: %w", err)
	}
	var cur int
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_version`).Scan(&cur); err != nil {
		return fmt.Errorf("读取数据库版本失败: %w", err)
	}
	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return err
	}
	for _, e := range entries {
		name := e.Name()
		if len(name) < 4 {
			return fmt.Errorf("迁移文件名无效: %s", name)
		}
		v, err := strconv.Atoi(name[:3])
		if err != nil {
			return fmt.Errorf("迁移文件名无效: %s", name)
		}
		if v <= cur {
			continue
		}
		b, err := migrationFS.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, string(b)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("执行迁移 %s 失败: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_version (version) VALUES (?)`, v); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// withTx 在事务中执行 fn
func (s *Store) withTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// affected 把影响行数为 0 转换为 ErrNotFound
func affected(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
```

`internal/dashboard/store/servers.go`：

```go
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/ruanun/simple-server-status/internal/dashboard/randx"
	"github.com/ruanun/simple-server-status/internal/proto"
)

// Server 服务器配置与最近一次的静态信息
type Server struct {
	ID              string       `json:"id"`
	Name            string       `json:"name"`
	Secret          string       `json:"secret"`
	Group           string       `json:"group"`
	Country         string       `json:"country"`
	Sort            int          `json:"sort"`
	Hidden          bool         `json:"hidden"`
	Price           *float64     `json:"price"`
	Currency        string       `json:"currency"`
	BillingCycle    string       `json:"billing_cycle"`
	ExpireAt        *int64       `json:"expire_at"`
	TrafficLimit    *int64       `json:"traffic_limit"`
	TrafficMode     string       `json:"traffic_mode"`
	TrafficResetDay int          `json:"traffic_reset_day"`
	ReportInterval  int          `json:"report_interval"`
	NICInclude      []string     `json:"nic_include"`
	NICExclude      []string     `json:"nic_exclude"`
	MountExclude    []string     `json:"mount_exclude"`
	StaticInfo      *proto.Hello `json:"static_info"`
	LastIP          string       `json:"last_ip"`
	LastSeen        int64        `json:"last_seen"`
	CreatedAt       int64        `json:"created_at"`
	UpdatedAt       int64        `json:"updated_at"`
}

const selectCols = `id, name, secret, grp, country, sort, hidden, price, currency, billing_cycle,
 expire_at, traffic_limit, traffic_mode, traffic_reset_day, report_interval,
 nic_include, nic_exclude, mount_exclude, static_info, last_ip, last_seen, created_at, updated_at`

const insertSQL = `INSERT INTO servers (id, name, secret, grp, country, sort, hidden, price, currency, billing_cycle,
 expire_at, traffic_limit, traffic_mode, traffic_reset_day, report_interval,
 nic_include, nic_exclude, mount_exclude, created_at, updated_at)
 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

const upsertSQL = insertSQL + ` ON CONFLICT(id) DO UPDATE SET
 name=excluded.name, secret=excluded.secret, grp=excluded.grp, country=excluded.country, sort=excluded.sort,
 hidden=excluded.hidden, price=excluded.price, currency=excluded.currency, billing_cycle=excluded.billing_cycle,
 expire_at=excluded.expire_at, traffic_limit=excluded.traffic_limit, traffic_mode=excluded.traffic_mode,
 traffic_reset_day=excluded.traffic_reset_day, report_interval=excluded.report_interval,
 nic_include=excluded.nic_include, nic_exclude=excluded.nic_exclude, mount_exclude=excluded.mount_exclude,
 updated_at=excluded.updated_at`

type scanner interface {
	Scan(dest ...any) error
}

func scanServer(row scanner) (Server, error) {
	var (
		s                     Server
		hidden                int
		price                 sql.NullFloat64
		expire, limit         sql.NullInt64
		nicIn, nicEx, mountEx string
		static                sql.NullString
	)
	err := row.Scan(&s.ID, &s.Name, &s.Secret, &s.Group, &s.Country, &s.Sort, &hidden, &price, &s.Currency,
		&s.BillingCycle, &expire, &limit, &s.TrafficMode, &s.TrafficResetDay, &s.ReportInterval,
		&nicIn, &nicEx, &mountEx, &static, &s.LastIP, &s.LastSeen, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return s, err
	}
	s.Hidden = hidden != 0
	if price.Valid {
		v := price.Float64
		s.Price = &v
	}
	if expire.Valid {
		v := expire.Int64
		s.ExpireAt = &v
	}
	if limit.Valid {
		v := limit.Int64
		s.TrafficLimit = &v
	}
	s.NICInclude = decodeList(nicIn)
	s.NICExclude = decodeList(nicEx)
	s.MountExclude = decodeList(mountEx)
	if static.Valid && static.String != "" {
		var h proto.Hello
		if json.Unmarshal([]byte(static.String), &h) == nil {
			s.StaticInfo = &h
		}
	}
	return s, nil
}

func decodeList(s string) []string {
	var l []string
	_ = json.Unmarshal([]byte(s), &l)
	if l == nil {
		l = []string{}
	}
	return l
}

func encodeList(l []string) string {
	if l == nil {
		return "[]"
	}
	b, _ := json.Marshal(l)
	return string(b)
}

func nullFloat(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

func nullInt(p *int64) any {
	if p == nil {
		return nil
	}
	return *p
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// applyDefaults 补全缺省字段
func applyDefaults(s *Server) {
	if s.TrafficMode == "" {
		s.TrafficMode = "sum"
	}
	if s.TrafficResetDay < 1 || s.TrafficResetDay > 28 {
		s.TrafficResetDay = 1
	}
	if s.ReportInterval < 1 {
		s.ReportInterval = 2
	}
	if s.NICInclude == nil {
		s.NICInclude = []string{}
	}
	if s.NICExclude == nil {
		s.NICExclude = []string{}
	}
	if s.MountExclude == nil {
		s.MountExclude = []string{}
	}
}

func insertArgs(s Server) []any {
	return []any{s.ID, s.Name, s.Secret, s.Group, s.Country, s.Sort, boolInt(s.Hidden), nullFloat(s.Price),
		s.Currency, s.BillingCycle, nullInt(s.ExpireAt), nullInt(s.TrafficLimit), s.TrafficMode,
		s.TrafficResetDay, s.ReportInterval, encodeList(s.NICInclude), encodeList(s.NICExclude),
		encodeList(s.MountExclude), s.CreatedAt, s.UpdatedAt}
}

// ListServers 按 sort、name 返回全部服务器
func (s *Store) ListServers(ctx context.Context) ([]Server, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+selectCols+` FROM servers ORDER BY sort, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []Server{}
	for rows.Next() {
		sv, err := scanServer(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, sv)
	}
	return list, rows.Err()
}

// GetServer 按 ID 读取服务器
func (s *Store) GetServer(ctx context.Context, id string) (Server, error) {
	sv, err := scanServer(s.db.QueryRowContext(ctx, `SELECT `+selectCols+` FROM servers WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return sv, ErrNotFound
	}
	return sv, err
}

// CreateServer 新建服务器，自动生成 ID、密钥与时间戳
func (s *Store) CreateServer(ctx context.Context, sv *Server) error {
	now := time.Now().Unix()
	if sv.ID == "" {
		sv.ID = randx.String(12)
	}
	if sv.Secret == "" {
		sv.Secret = randx.String(32)
	}
	sv.CreatedAt, sv.UpdatedAt = now, now
	applyDefaults(sv)
	_, err := s.db.ExecContext(ctx, insertSQL, insertArgs(*sv)...)
	return err
}

// UpdateServer 更新可编辑字段（不含密钥、排序与静态信息）
func (s *Store) UpdateServer(ctx context.Context, sv Server) error {
	applyDefaults(&sv)
	return affected(s.db.ExecContext(ctx, `UPDATE servers SET name=?, grp=?, country=?, hidden=?, price=?,
 currency=?, billing_cycle=?, expire_at=?, traffic_limit=?, traffic_mode=?, traffic_reset_day=?, report_interval=?,
 nic_include=?, nic_exclude=?, mount_exclude=?, updated_at=? WHERE id=?`,
		sv.Name, sv.Group, sv.Country, boolInt(sv.Hidden), nullFloat(sv.Price), sv.Currency, sv.BillingCycle,
		nullInt(sv.ExpireAt), nullInt(sv.TrafficLimit), sv.TrafficMode, sv.TrafficResetDay, sv.ReportInterval,
		encodeList(sv.NICInclude), encodeList(sv.NICExclude), encodeList(sv.MountExclude), time.Now().Unix(), sv.ID))
}

// DeleteServer 删除服务器及其历史与流量数据
func (s *Store) DeleteServer(ctx context.Context, id string) error {
	return s.withTx(ctx, func(tx *sql.Tx) error {
		for _, q := range []string{
			`DELETE FROM metrics_1m WHERE server_id = ?`,
			`DELETE FROM metrics_10m WHERE server_id = ?`,
			`DELETE FROM traffic_monthly WHERE server_id = ?`,
		} {
			if _, err := tx.ExecContext(ctx, q, id); err != nil {
				return err
			}
		}
		return affected(tx.ExecContext(ctx, `DELETE FROM servers WHERE id = ?`, id))
	})
}

// SetServerOrder 按 ids 顺序重写排序值
func (s *Store) SetServerOrder(ctx context.Context, ids []string) error {
	return s.withTx(ctx, func(tx *sql.Tx) error {
		for i, id := range ids {
			if _, err := tx.ExecContext(ctx, `UPDATE servers SET sort = ? WHERE id = ?`, i, id); err != nil {
				return err
			}
		}
		return nil
	})
}

// ResetSecret 生成新密钥
func (s *Store) ResetSecret(ctx context.Context, id string) (string, error) {
	sec := randx.String(32)
	err := affected(s.db.ExecContext(ctx, `UPDATE servers SET secret = ?, updated_at = ? WHERE id = ?`, sec, time.Now().Unix(), id))
	return sec, err
}

// SetStaticInfo 保存 Agent 最近一次上报的静态信息与来源 IP
func (s *Store) SetStaticInfo(ctx context.Context, id, ip string, h proto.Hello) error {
	b, err := json.Marshal(h)
	if err != nil {
		return err
	}
	return affected(s.db.ExecContext(ctx, `UPDATE servers SET static_info = ?, last_ip = ? WHERE id = ?`, string(b), ip, id))
}

// SetLastSeen 记录最后在线时间
func (s *Store) SetLastSeen(ctx context.Context, id string, ts int64) error {
	return affected(s.db.ExecContext(ctx, `UPDATE servers SET last_seen = ? WHERE id = ?`, ts, id))
}

// UpsertServers 导入服务器：同 ID 覆盖配置（保留静态信息），新 ID 新增
func (s *Store) UpsertServers(ctx context.Context, list []Server) error {
	now := time.Now().Unix()
	return s.withTx(ctx, func(tx *sql.Tx) error {
		for _, sv := range list {
			applyDefaults(&sv)
			if sv.CreatedAt == 0 {
				sv.CreatedAt = now
			}
			sv.UpdatedAt = now
			if _, err := tx.ExecContext(ctx, upsertSQL, insertArgs(sv)...); err != nil {
				return err
			}
		}
		return nil
	})
}
```

`internal/dashboard/store/users.go`：

```go
package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// User 管理员账户
type User struct {
	ID           int64
	Username     string
	PasswordHash string
	TokenVersion int
	CreatedAt    int64
}

const userCols = `id, username, password_hash, token_version, created_at`

func scanUser(row scanner) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.TokenVersion, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return u, ErrNotFound
	}
	return u, err
}

// CountUsers 用户数量
func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

// CreateUser 新建用户并回填 ID
func (s *Store) CreateUser(ctx context.Context, u *User) error {
	u.CreatedAt = time.Now().Unix()
	res, err := s.db.ExecContext(ctx, `INSERT INTO users (username, password_hash, token_version, created_at) VALUES (?, ?, ?, ?)`,
		u.Username, u.PasswordHash, u.TokenVersion, u.CreatedAt)
	if err != nil {
		return err
	}
	u.ID, err = res.LastInsertId()
	return err
}

// GetUserByName 按用户名读取
func (s *Store) GetUserByName(ctx context.Context, name string) (User, error) {
	return scanUser(s.db.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE username = ?`, name))
}

// GetUser 按 ID 读取
func (s *Store) GetUser(ctx context.Context, id int64) (User, error) {
	return scanUser(s.db.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE id = ?`, id))
}

// SetPassword 修改密码并使旧 token 失效
func (s *Store) SetPassword(ctx context.Context, id int64, hash string) error {
	return affected(s.db.ExecContext(ctx, `UPDATE users SET password_hash = ?, token_version = token_version + 1 WHERE id = ?`, hash, id))
}
```

`internal/dashboard/store/settings.go`：

```go
package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"strconv"
)

// DefaultInstallScriptBase 安装脚本默认下载地址前缀
const DefaultInstallScriptBase = "https://github.com/ruanun/simple-server-status/releases/latest/download"

// Settings 运行期设置（后台修改后即时生效）
type Settings struct {
	SiteTitle             string `json:"site_title"`
	ShowPrice             bool   `json:"show_price"`
	DefaultReportInterval int    `json:"default_report_interval"`
	InstallScriptBase     string `json:"install_script_base"`
}

// DefaultSettings 默认设置
func DefaultSettings() Settings {
	return Settings{
		SiteTitle:             "Simple Server Status",
		DefaultReportInterval: 2,
		InstallScriptBase:     DefaultInstallScriptBase,
	}
}

// GetSettings 读取设置，缺失项使用默认值
func (s *Store) GetSettings(ctx context.Context) (Settings, error) {
	st := DefaultSettings()
	rows, err := s.db.QueryContext(ctx, `SELECT key, value FROM settings`)
	if err != nil {
		return st, err
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return st, err
		}
		switch k {
		case "site_title":
			st.SiteTitle = v
		case "show_price":
			st.ShowPrice = v == "true"
		case "default_report_interval":
			if n, err := strconv.Atoi(v); err == nil {
				st.DefaultReportInterval = n
			}
		case "install_script_base":
			st.InstallScriptBase = v
		}
	}
	return st, rows.Err()
}

// SaveSettings 保存设置
func (s *Store) SaveSettings(ctx context.Context, st Settings) error {
	kv := map[string]string{
		"site_title":              st.SiteTitle,
		"show_price":              strconv.FormatBool(st.ShowPrice),
		"default_report_interval": strconv.Itoa(st.DefaultReportInterval),
		"install_script_base":     st.InstallScriptBase,
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		for k, v := range kv {
			if err := upsertSetting(ctx, tx, k, v); err != nil {
				return err
			}
		}
		return nil
	})
}

func upsertSetting(ctx context.Context, e execer, k, v string) error {
	_, err := e.ExecContext(ctx, `INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, k, v)
	return err
}

// JWTSecret 读取 JWT 签名密钥，不存在时生成 32 字节随机密钥
func (s *Store) JWTSecret(ctx context.Context) ([]byte, error) {
	var v string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = 'jwt_secret'`).Scan(&v)
	if err == nil {
		return hex.DecodeString(v)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	if err := upsertSetting(ctx, s.db, "jwt_secret", hex.EncodeToString(b)); err != nil {
		return nil, err
	}
	return b, nil
}
```

- [ ] **Step 5: 运行测试确认通过**

```bash
go get modernc.org/sqlite@v1.44.3
go mod tidy
go vet ./...
go test ./internal/dashboard/... -v
grep -E '^go ' go.mod
```

Expected: PASS；go 版本仍为 `go 1.24.0`。

- [ ] **Step 6: 提交**

```bash
git add internal/dashboard go.mod go.sum
git commit -m "feat(dashboard): 新增 SQLite 存储（服务器、用户、设置）"
```

---

### Task 6: 指标点模型与历史、流量存储

**Files:**
- Create: `internal/dashboard/metric/metric.go`、`internal/dashboard/metric/metric_test.go`
- Create: `internal/dashboard/store/metrics.go`、`internal/dashboard/store/traffic.go`
- Test: `internal/dashboard/store/metrics_test.go`

**Interfaces:**
- Consumes: `proto.Report`（Task 1）；`store.Store`、`store.ErrNotFound`（Task 5）
- Produces:
  - `type metric.Point{TS int64; CPU, Mem, Disk, NetIn, NetOut, Load1, TCP float64}`（JSON：`ts`、`cpu`、`mem`、`disk`、`net_in`、`net_out`、`load1`、`tcp`）
  - `func metric.Percent(used, total uint64) float64`；`func metric.FromReport(r proto.Report, memTotal uint64) metric.Point`；`func metric.Average(ts int64, pts []metric.Point) metric.Point`；`func metric.Downsample(pts []metric.Point, bucket int64) []metric.Point`
  - `const store.Metrics1m = "metrics_1m"`、`store.Metrics10m = "metrics_10m"`
  - `type store.MetricRow struct{ ServerID string; metric.Point }`
  - `func (s *Store) InsertMetrics(ctx, table string, rows []MetricRow) error`；`QueryMetrics(ctx, table, serverID string, from, to int64) ([]metric.Point, error)`（`from <= ts < to`，升序，空为 `[]`）；`Rollup10m(ctx, from, to int64) error`；`DeleteMetricsBefore(ctx, table string, ts int64) (int64, error)`
  - `type store.TrafficRow{ServerID, Period string; In, Out, LastIn, LastOut int64}`；`GetTraffic(ctx, serverID, period string) (TrafficRow, error)`；`SaveTraffic(ctx, rows []TrafficRow) error`

- [ ] **Step 1: 写失败测试**

`internal/dashboard/metric/metric_test.go`：

```go
package metric

import (
	"testing"

	"github.com/ruanun/simple-server-status/internal/proto"
)

func TestPercent(t *testing.T) {
	if Percent(50, 200) != 25 || Percent(1, 0) != 0 {
		t.Fatal("Percent 计算错误")
	}
}

func TestFromReport(t *testing.T) {
	p := FromReport(proto.Report{TS: 10, CPU: 5, MemUsed: 512, DiskUsed: 25, DiskTotal: 100,
		NetInSpeed: 7, NetOutSpeed: 8, Load1: 0.5, TCP: 3}, 1024)
	want := Point{TS: 10, CPU: 5, Mem: 50, Disk: 25, NetIn: 7, NetOut: 8, Load1: 0.5, TCP: 3}
	if p != want {
		t.Fatalf("FromReport = %+v，期望 %+v", p, want)
	}
}

func TestAverage(t *testing.T) {
	got := Average(60, []Point{{CPU: 10, NetIn: 100}, {CPU: 20, NetIn: 300}})
	if got.TS != 60 || got.CPU != 15 || got.NetIn != 200 {
		t.Fatalf("Average = %+v", got)
	}
	if Average(0, nil) != (Point{}) {
		t.Fatal("空输入应返回零值")
	}
}

func TestDownsample(t *testing.T) {
	pts := []Point{{TS: 0, CPU: 1}, {TS: 100, CPU: 2}, {TS: 299, CPU: 3}, {TS: 300, CPU: 4}, {TS: 650, CPU: 5}}
	got := Downsample(pts, 300)
	want := []Point{{TS: 0, CPU: 2}, {TS: 300, CPU: 4}, {TS: 600, CPU: 5}}
	if len(got) != len(want) {
		t.Fatalf("Downsample = %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("第 %d 个点 = %+v，期望 %+v", i, got[i], want[i])
		}
	}
	if out := Downsample(nil, 300); out == nil || len(out) != 0 {
		t.Fatal("空输入应返回 []")
	}
}
```

`internal/dashboard/store/metrics_test.go`：

```go
package store

import (
	"context"
	"errors"
	"testing"

	"github.com/ruanun/simple-server-status/internal/dashboard/metric"
)

func TestMetricsInsertQueryRollupDelete(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	rows := []MetricRow{
		{ServerID: "a", Point: metric.Point{TS: 600, CPU: 10, NetIn: 100}},
		{ServerID: "a", Point: metric.Point{TS: 660, CPU: 30, NetIn: 300}},
		{ServerID: "b", Point: metric.Point{TS: 600, CPU: 50}},
	}
	if err := s.InsertMetrics(ctx, Metrics1m, rows); err != nil {
		t.Fatal(err)
	}
	// 同一时间点重复写入应覆盖
	if err := s.InsertMetrics(ctx, Metrics1m, []MetricRow{{ServerID: "a", Point: metric.Point{TS: 660, CPU: 30, NetIn: 300}}}); err != nil {
		t.Fatal(err)
	}
	pts, err := s.QueryMetrics(ctx, Metrics1m, "a", 0, 1000)
	if err != nil || len(pts) != 2 || pts[0].TS != 600 {
		t.Fatalf("QueryMetrics = %+v, %v", pts, err)
	}
	empty, _ := s.QueryMetrics(ctx, Metrics1m, "none", 0, 1000)
	if empty == nil || len(empty) != 0 {
		t.Fatal("无数据时应返回 []")
	}

	if err := s.Rollup10m(ctx, 600, 1200); err != nil {
		t.Fatal(err)
	}
	agg, _ := s.QueryMetrics(ctx, Metrics10m, "a", 0, 2000)
	if len(agg) != 1 || agg[0].TS != 600 || agg[0].CPU != 20 || agg[0].NetIn != 200 {
		t.Fatalf("Rollup10m 结果错误: %+v", agg)
	}

	n, err := s.DeleteMetricsBefore(ctx, Metrics1m, 650)
	if err != nil || n != 2 {
		t.Fatalf("DeleteMetricsBefore = %d, %v", n, err)
	}
	if err := s.InsertMetrics(ctx, "users", nil); err == nil {
		t.Fatal("非法表名应报错")
	}
}

func TestDeleteServerRemovesMetricsAndTraffic(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	sv := Server{Name: "a"}
	if err := s.CreateServer(ctx, &sv); err != nil {
		t.Fatal(err)
	}
	_ = s.InsertMetrics(ctx, Metrics1m, []MetricRow{{ServerID: sv.ID, Point: metric.Point{TS: 60}}})
	_ = s.SaveTraffic(ctx, []TrafficRow{{ServerID: sv.ID, Period: "2026-03-01", In: 1}})
	if err := s.DeleteServer(ctx, sv.ID); err != nil {
		t.Fatal(err)
	}
	pts, _ := s.QueryMetrics(ctx, Metrics1m, sv.ID, 0, 1000)
	if len(pts) != 0 {
		t.Fatal("删除服务器后历史数据应被清除")
	}
	if _, err := s.GetTraffic(ctx, sv.ID, "2026-03-01"); !errors.Is(err, ErrNotFound) {
		t.Fatal("删除服务器后流量数据应被清除")
	}
}

func TestTrafficGetSave(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if _, err := s.GetTraffic(ctx, "a", "2026-03-01"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("不存在时应 ErrNotFound，实际 %v", err)
	}
	row := TrafficRow{ServerID: "a", Period: "2026-03-01", In: 10, Out: 20, LastIn: 100, LastOut: 200}
	if err := s.SaveTraffic(ctx, []TrafficRow{row}); err != nil {
		t.Fatal(err)
	}
	row.In = 15
	if err := s.SaveTraffic(ctx, []TrafficRow{row}); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetTraffic(ctx, "a", "2026-03-01")
	if err != nil || got != row {
		t.Fatalf("GetTraffic = %+v, %v", got, err)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/dashboard/...`
Expected: FAIL（`undefined: Point`、`undefined: MetricRow` 等）。

- [ ] **Step 3: 实现 metric**

`internal/dashboard/metric/metric.go`：

```go
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
```

- [ ] **Step 4: 实现历史与流量存储**

`internal/dashboard/store/metrics.go`：

```go
package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/ruanun/simple-server-status/internal/dashboard/metric"
)

// 历史数据表
const (
	Metrics1m  = "metrics_1m"
	Metrics10m = "metrics_10m"
)

// MetricRow 某台服务器的一个指标点
type MetricRow struct {
	ServerID string
	metric.Point
}

// sqlFor 把模板中的 {t} 替换为经过校验的表名
func sqlFor(table, tmpl string) (string, error) {
	if table != Metrics1m && table != Metrics10m {
		return "", fmt.Errorf("非法的历史数据表: %q", table)
	}
	return strings.ReplaceAll(tmpl, "{t}", table), nil
}

// InsertMetrics 批量写入（同一服务器同一时间点覆盖）
func (s *Store) InsertMetrics(ctx context.Context, table string, rows []MetricRow) error {
	q, err := sqlFor(table, `INSERT OR REPLACE INTO {t} (server_id, ts, cpu, mem, disk, net_in, net_out, load1, tcp) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		stmt, err := tx.PrepareContext(ctx, q)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, r := range rows {
			if _, err := stmt.ExecContext(ctx, r.ServerID, r.TS, r.CPU, r.Mem, r.Disk, r.NetIn, r.NetOut, r.Load1, r.TCP); err != nil {
				return err
			}
		}
		return nil
	})
}

// QueryMetrics 查询 from <= ts < to 的数据，按时间升序
func (s *Store) QueryMetrics(ctx context.Context, table, serverID string, from, to int64) ([]metric.Point, error) {
	q, err := sqlFor(table, `SELECT ts, cpu, mem, disk, net_in, net_out, load1, tcp FROM {t} WHERE server_id = ? AND ts >= ? AND ts < ? ORDER BY ts`)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, q, serverID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	pts := []metric.Point{}
	for rows.Next() {
		var p metric.Point
		if err := rows.Scan(&p.TS, &p.CPU, &p.Mem, &p.Disk, &p.NetIn, &p.NetOut, &p.Load1, &p.TCP); err != nil {
			return nil, err
		}
		pts = append(pts, p)
	}
	return pts, rows.Err()
}

// Rollup10m 把 [from, to) 内的 1 分钟数据按服务器取平均，写入 10 分钟表（时间戳为 from）
func (s *Store) Rollup10m(ctx context.Context, from, to int64) error {
	_, err := s.db.ExecContext(ctx, `INSERT OR REPLACE INTO metrics_10m (server_id, ts, cpu, mem, disk, net_in, net_out, load1, tcp)
 SELECT server_id, ?, AVG(cpu), AVG(mem), AVG(disk), AVG(net_in), AVG(net_out), AVG(load1), AVG(tcp)
 FROM metrics_1m WHERE ts >= ? AND ts < ? GROUP BY server_id`, from, from, to)
	return err
}

// DeleteMetricsBefore 删除早于 ts 的数据，返回删除行数
func (s *Store) DeleteMetricsBefore(ctx context.Context, table string, ts int64) (int64, error) {
	q, err := sqlFor(table, `DELETE FROM {t} WHERE ts < ?`)
	if err != nil {
		return 0, err
	}
	res, err := s.db.ExecContext(ctx, q, ts)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
```

`internal/dashboard/store/traffic.go`：

```go
package store

import (
	"context"
	"database/sql"
	"errors"
)

// TrafficRow 某台服务器在某个计费周期内的流量
type TrafficRow struct {
	ServerID string
	Period   string // 周期起始日 YYYY-MM-DD
	In       int64
	Out      int64
	LastIn   int64 // 最近一次读到的累计计数，用于计算增量
	LastOut  int64
}

// GetTraffic 读取某周期的流量记录
func (s *Store) GetTraffic(ctx context.Context, serverID, period string) (TrafficRow, error) {
	r := TrafficRow{ServerID: serverID, Period: period}
	err := s.db.QueryRowContext(ctx, `SELECT in_bytes, out_bytes, last_in_total, last_out_total FROM traffic_monthly WHERE server_id = ? AND period = ?`,
		serverID, period).Scan(&r.In, &r.Out, &r.LastIn, &r.LastOut)
	if errors.Is(err, sql.ErrNoRows) {
		return r, ErrNotFound
	}
	return r, err
}

// SaveTraffic 批量保存流量记录
func (s *Store) SaveTraffic(ctx context.Context, rows []TrafficRow) error {
	return s.withTx(ctx, func(tx *sql.Tx) error {
		for _, r := range rows {
			if _, err := tx.ExecContext(ctx, `INSERT INTO traffic_monthly (server_id, period, in_bytes, out_bytes, last_in_total, last_out_total)
 VALUES (?, ?, ?, ?, ?, ?) ON CONFLICT(server_id, period) DO UPDATE SET in_bytes = excluded.in_bytes,
 out_bytes = excluded.out_bytes, last_in_total = excluded.last_in_total, last_out_total = excluded.last_out_total`,
				r.ServerID, r.Period, r.In, r.Out, r.LastIn, r.LastOut); err != nil {
				return err
			}
		}
		return nil
	})
}
```

- [ ] **Step 5: 运行测试确认通过**

Run: `go vet ./... && go test ./internal/dashboard/... -v`
Expected: PASS。

- [ ] **Step 6: 提交**

```bash
git add internal/dashboard
git commit -m "feat(dashboard): 新增指标点模型与历史、流量存储"
```

---

### Task 7: 实时状态中心（hub）

**Files:**
- Create: `internal/dashboard/hub/hub.go`、`internal/dashboard/hub/hub_test.go`

**Interfaces:**
- Consumes: `proto.Hello`、`proto.Report`（Task 1）；`metric.Point`、`metric.FromReport`（Task 6）
- Produces:
  - `type hub.Live struct{ Online, Connected bool; LastReport int64; Report *proto.Report; Static *proto.Hello }`
  - `func hub.New(now func() time.Time) *hub.Hub`
  - `func (h *Hub) Connect(id string, interval int) uint64`；`Disconnect(id string, session uint64)`；`SetInterval(id string, interval int)`；`SetStatic(id string, hello proto.Hello)`；`Report(id string, r proto.Report) metric.Point`（以服务端时间覆盖 `r.TS`）；`Get(id string) Live`；`Ring(id string) []metric.Point`；`Remove(id string)`；`Changed(since uint64) ([]string, uint64)`
  - 在线判定：已连接 且 距最近一次上报 ≤ max(3×interval, 6) 秒

- [ ] **Step 1: 写失败测试**

`internal/dashboard/hub/hub_test.go`：

```go
package hub

import (
	"testing"
	"time"

	"github.com/ruanun/simple-server-status/internal/proto"
)

type clock struct{ t time.Time }

func (c *clock) Now() time.Time { return c.t }

func newHub() (*Hub, *clock) {
	c := &clock{t: time.Unix(1000, 0)}
	return New(c.Now), c
}

func TestOnlineRequiresRecentReport(t *testing.T) {
	h, c := newHub()
	h.Connect("a", 2)
	if h.Get("a").Online {
		t.Fatal("未上报前不应在线")
	}
	h.Report("a", proto.Report{CPU: 1})
	if !h.Get("a").Online {
		t.Fatal("上报后应在线")
	}
	c.t = c.t.Add(6 * time.Second)
	if !h.Get("a").Online {
		t.Fatal("6 秒内应在线")
	}
	c.t = c.t.Add(time.Second)
	if h.Get("a").Online {
		t.Fatal("超过 6 秒应离线")
	}
}

func TestOnlineThresholdScalesWithInterval(t *testing.T) {
	h, c := newHub()
	h.Connect("a", 10)
	h.Report("a", proto.Report{})
	c.t = c.t.Add(30 * time.Second)
	if !h.Get("a").Online {
		t.Fatal("间隔 10 秒时 30 秒内应在线")
	}
	c.t = c.t.Add(time.Second)
	if h.Get("a").Online {
		t.Fatal("超过 30 秒应离线")
	}
}

func TestDisconnectIgnoresStaleSession(t *testing.T) {
	h, _ := newHub()
	s1 := h.Connect("a", 2)
	s2 := h.Connect("a", 2)
	h.Report("a", proto.Report{})
	h.Disconnect("a", s1)
	if !h.Get("a").Online {
		t.Fatal("旧会话断开不应影响新连接")
	}
	h.Disconnect("a", s2)
	if l := h.Get("a"); l.Online || l.Connected {
		t.Fatalf("当前会话断开后应离线: %+v", l)
	}
}

func TestReportUsesServerTimeAndStatic(t *testing.T) {
	h, c := newHub()
	h.SetStatic("a", proto.Hello{MemTotal: 1000})
	p := h.Report("a", proto.Report{TS: 1, MemUsed: 250})
	if p.Mem != 25 || p.TS != c.t.Unix() {
		t.Fatalf("指标点错误: %+v", p)
	}
	l := h.Get("a")
	if l.Report == nil || l.Report.TS != c.t.Unix() || l.LastReport != c.t.Unix() || l.Static == nil {
		t.Fatalf("实时状态错误: %+v", l)
	}
}

func TestRingKeepsTenMinutes(t *testing.T) {
	h, c := newHub()
	for i := 0; i < 700; i++ {
		h.Report("a", proto.Report{})
		c.t = c.t.Add(time.Second)
	}
	ring := h.Ring("a")
	if len(ring) != 601 || ring[0].TS != 1099 {
		t.Fatalf("环形缓冲长度 %d，首点 %d", len(ring), ring[0].TS)
	}
	if r := h.Ring("none"); r == nil || len(r) != 0 {
		t.Fatal("未知服务器应返回 []")
	}
}

func TestChangedAndRemove(t *testing.T) {
	h, _ := newHub()
	_, s0 := h.Changed(0)
	h.Report("a", proto.Report{})
	ids, s1 := h.Changed(s0)
	if len(ids) != 1 || ids[0] != "a" {
		t.Fatalf("Changed = %v", ids)
	}
	if ids, _ := h.Changed(s1); len(ids) != 0 {
		t.Fatalf("无变化时应为空: %v", ids)
	}
	h.Remove("a")
	if l := h.Get("a"); l.Report != nil || l.Online {
		t.Fatalf("删除后应无状态: %+v", l)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/dashboard/hub/`
Expected: FAIL（`undefined: New`）。

- [ ] **Step 3: 实现 hub**

`internal/dashboard/hub/hub.go`：

```go
// Package hub 在内存中维护各服务器的实时状态。
package hub

import (
	"sync"
	"time"

	"github.com/ruanun/simple-server-status/internal/dashboard/metric"
	"github.com/ruanun/simple-server-status/internal/proto"
)

// ringWindow 秒级环形缓冲保留时长
const ringWindow = 10 * time.Minute

// Live 某台服务器的实时状态
type Live struct {
	Online     bool
	Connected  bool
	LastReport int64 // unix 秒，0 表示本次运行中尚未上报
	Report     *proto.Report
	Static     *proto.Hello
}

type state struct {
	session   uint64
	connected bool
	interval  int
	last      time.Time
	report    *proto.Report
	static    *proto.Hello
	ring      []metric.Point
	seq       uint64
}

// Hub 实时状态中心，并发安全
type Hub struct {
	mu      sync.RWMutex
	now     func() time.Time
	nextSes uint64
	seq     uint64
	servers map[string]*state
}

// New 创建 Hub
func New(now func() time.Time) *Hub {
	return &Hub{now: now, servers: map[string]*state{}}
}

func (h *Hub) get(id string) *state {
	s := h.servers[id]
	if s == nil {
		s = &state{interval: 2}
		h.servers[id] = s
	}
	return s
}

func (h *Hub) bump(s *state) {
	h.seq++
	s.seq = h.seq
}

// Connect 记录新连接，返回会话号
func (h *Hub) Connect(id string, interval int) uint64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	s := h.get(id)
	h.nextSes++
	s.session = h.nextSes
	s.connected = true
	s.interval = interval
	h.bump(s)
	return s.session
}

// Disconnect 仅当会话号匹配时标记断开，避免旧连接的退出影响新连接
func (h *Hub) Disconnect(id string, session uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s := h.servers[id]
	if s == nil || s.session != session {
		return
	}
	s.connected = false
	h.bump(s)
}

// SetInterval 更新上报间隔（影响在线判定）
func (h *Hub) SetInterval(id string, interval int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.get(id).interval = interval
}

// SetStatic 更新静态信息
func (h *Hub) SetStatic(id string, hello proto.Hello) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s := h.get(id)
	s.static = &hello
	h.bump(s)
}

// Report 记录一次上报（时间戳以服务端为准），返回对应的指标点
func (h *Hub) Report(id string, r proto.Report) metric.Point {
	h.mu.Lock()
	defer h.mu.Unlock()
	s := h.get(id)
	now := h.now()
	r.TS = now.Unix()
	s.last = now
	s.report = &r
	var memTotal uint64
	if s.static != nil {
		memTotal = s.static.MemTotal
	}
	p := metric.FromReport(r, memTotal)
	s.ring = append(s.ring, p)
	cut := now.Add(-ringWindow).Unix()
	i := 0
	for i < len(s.ring) && s.ring[i].TS < cut {
		i++
	}
	s.ring = s.ring[i:]
	h.bump(s)
	return p
}

func (h *Hub) online(s *state, now time.Time) bool {
	if !s.connected || s.last.IsZero() {
		return false
	}
	limit := time.Duration(3*s.interval) * time.Second
	if limit < 6*time.Second {
		limit = 6 * time.Second
	}
	return now.Sub(s.last) <= limit
}

// Get 返回实时状态；未知服务器返回零值
func (h *Hub) Get(id string) Live {
	h.mu.RLock()
	defer h.mu.RUnlock()
	s := h.servers[id]
	if s == nil {
		return Live{}
	}
	l := Live{Online: h.online(s, h.now()), Connected: s.connected, Report: s.report, Static: s.static}
	if !s.last.IsZero() {
		l.LastReport = s.last.Unix()
	}
	return l
}

// Ring 返回最近 10 分钟的秒级指标点副本
func (h *Hub) Ring(id string) []metric.Point {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := []metric.Point{}
	if s := h.servers[id]; s != nil {
		out = append(out, s.ring...)
	}
	return out
}

// Remove 删除服务器的全部实时状态
func (h *Hub) Remove(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.servers, id)
	h.seq++
}

// Changed 返回序号大于 since 的服务器 ID，以及当前序号
func (h *Hub) Changed(since uint64) ([]string, uint64) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	ids := []string{}
	for id, s := range h.servers {
		if s.seq > since {
			ids = append(ids, id)
		}
	}
	return ids, h.seq
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go vet ./... && go test ./internal/dashboard/hub/ -v`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/dashboard/hub
git commit -m "feat(dashboard): 新增实时状态中心与在线判定"
```

---

### Task 8: 历史降采样与查询（history）

**Files:**
- Create: `internal/dashboard/history/history.go`、`internal/dashboard/history/history_test.go`

**Interfaces:**
- Consumes: `metric.Point`、`metric.Average`、`metric.Downsample`（Task 6）；`store.MetricRow`、`store.Metrics1m/Metrics10m`、`store.Open`（Task 5/6）
- Produces:
  - `type history.MetricStore interface{ InsertMetrics; QueryMetrics; Rollup10m; DeleteMetricsBefore }`（签名同 store）
  - `func history.NewRecorder(st MetricStore, now func() time.Time, log *slog.Logger) *history.Recorder`
  - `func (r *Recorder) Add(id string, p metric.Point)`；`Flush(ctx, all bool) error`；`Tick(ctx)`；`Run(ctx)`（每分钟 Tick，ctx 结束时写入全部剩余数据后返回）
  - `const history.RangeRealtime/Range1h/Range6h/Range24h/Range7d`（`"realtime"`、`"1h"`、`"6h"`、`"24h"`、`"7d"`）；`var history.ErrBadRange`
  - `func history.Query(ctx, st MetricStore, ring func(id string) []metric.Point, id, rng string, now time.Time) ([]metric.Point, error)`

- [ ] **Step 1: 写失败测试**

`internal/dashboard/history/history_test.go`：

```go
package history

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/ruanun/simple-server-status/internal/dashboard/metric"
	"github.com/ruanun/simple-server-status/internal/dashboard/store"
)

type clock struct{ t time.Time }

func (c *clock) Now() time.Time { return c.t }

func newRecorder(t *testing.T, at int64) (*Recorder, *store.Store, *clock) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "h.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	c := &clock{t: time.Unix(at, 0)}
	return NewRecorder(st, c.Now, slog.New(slog.DiscardHandler)), st, c
}

func query(t *testing.T, st *store.Store, table string) []metric.Point {
	t.Helper()
	pts, err := st.QueryMetrics(context.Background(), table, "a", 0, 1<<40)
	if err != nil {
		t.Fatal(err)
	}
	return pts
}

func TestAddAggregatesMinute(t *testing.T) {
	r, st, _ := newRecorder(t, 120)
	r.Add("a", metric.Point{TS: 60, CPU: 10})
	r.Add("a", metric.Point{TS: 90, CPU: 30})
	r.Add("a", metric.Point{TS: 125, CPU: 50}) // 进入下一分钟
	if err := r.Flush(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	pts := query(t, st, store.Metrics1m)
	if len(pts) != 1 || pts[0].TS != 60 || pts[0].CPU != 20 {
		t.Fatalf("分钟聚合错误: %+v", pts)
	}
}

func TestFlushWritesStaleBucket(t *testing.T) {
	r, st, c := newRecorder(t, 100)
	r.Add("a", metric.Point{TS: 70, CPU: 10})
	_ = r.Flush(context.Background(), false)
	if pts := query(t, st, store.Metrics1m); len(pts) != 0 {
		t.Fatalf("当前分钟未结束不应写入: %+v", pts)
	}
	c.t = time.Unix(130, 0)
	_ = r.Flush(context.Background(), false)
	if pts := query(t, st, store.Metrics1m); len(pts) != 1 || pts[0].TS != 60 {
		t.Fatalf("已结束的分钟应写入（即使服务器已无新上报）: %+v", pts)
	}
}

func TestFlushAllIncludesCurrentMinute(t *testing.T) {
	r, st, _ := newRecorder(t, 80)
	r.Add("a", metric.Point{TS: 70, CPU: 10})
	if err := r.Flush(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	if pts := query(t, st, store.Metrics1m); len(pts) != 1 {
		t.Fatalf("退出时应写入当前分钟: %+v", pts)
	}
}

func TestTickRollsUpTenMinutes(t *testing.T) {
	r, st, _ := newRecorder(t, 1250)
	_ = st.InsertMetrics(context.Background(), store.Metrics1m, []store.MetricRow{
		{ServerID: "a", Point: metric.Point{TS: 600, CPU: 10}},
		{ServerID: "a", Point: metric.Point{TS: 660, CPU: 30}},
	})
	r.Tick(context.Background())
	pts := query(t, st, store.Metrics10m)
	if len(pts) != 1 || pts[0].TS != 600 || pts[0].CPU != 20 {
		t.Fatalf("10 分钟聚合错误: %+v", pts)
	}
}

func TestTickCleansUpExpired(t *testing.T) {
	r, st, _ := newRecorder(t, 100000)
	_ = st.InsertMetrics(context.Background(), store.Metrics1m, []store.MetricRow{
		{ServerID: "a", Point: metric.Point{TS: 10}},
		{ServerID: "a", Point: metric.Point{TS: 99990}},
	})
	r.Tick(context.Background())
	pts := query(t, st, store.Metrics1m)
	if len(pts) != 1 || pts[0].TS != 99990 {
		t.Fatalf("应只保留 24 小时内的数据: %+v", pts)
	}
}

func TestQueryRanges(t *testing.T) {
	ctx := context.Background()
	_, st, _ := newRecorder(t, 0)
	now := time.Unix(100000, 0)
	_ = st.InsertMetrics(ctx, store.Metrics1m, []store.MetricRow{
		{ServerID: "a", Point: metric.Point{TS: 50000, CPU: 99}},
		{ServerID: "a", Point: metric.Point{TS: 99600, CPU: 10}},
		{ServerID: "a", Point: metric.Point{TS: 99660, CPU: 20}},
		{ServerID: "a", Point: metric.Point{TS: 99900, CPU: 30}},
	})
	_ = st.InsertMetrics(ctx, store.Metrics10m, []store.MetricRow{{ServerID: "a", Point: metric.Point{TS: 99000, CPU: 40}}})
	ring := func(string) []metric.Point { return []metric.Point{{TS: 1}} }

	if pts, err := Query(ctx, st, ring, "a", Range1h, now); err != nil || len(pts) != 3 {
		t.Fatalf("1h = %+v, %v", pts, err)
	}
	pts, err := Query(ctx, st, ring, "a", Range24h, now)
	if err != nil || len(pts) != 3 || pts[1] != (metric.Point{TS: 99600, CPU: 15}) {
		t.Fatalf("24h 应聚合为 5 分钟: %+v, %v", pts, err)
	}
	if pts, _ := Query(ctx, st, ring, "a", Range7d, now); len(pts) != 1 || pts[0].CPU != 40 {
		t.Fatalf("7d = %+v", pts)
	}
	if pts, _ := Query(ctx, st, ring, "a", RangeRealtime, now); len(pts) != 1 {
		t.Fatalf("realtime = %+v", pts)
	}
	if _, err := Query(ctx, st, ring, "a", "bad", now); !errors.Is(err, ErrBadRange) {
		t.Fatalf("非法范围应返回 ErrBadRange，实际 %v", err)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/dashboard/history/`
Expected: FAIL（`undefined: NewRecorder`）。

- [ ] **Step 3: 实现 history**

`internal/dashboard/history/history.go`：

```go
// Package history 把实时指标降采样写入数据库，并按时间范围查询。
package history

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/ruanun/simple-server-status/internal/dashboard/metric"
	"github.com/ruanun/simple-server-status/internal/dashboard/store"
)

// MetricStore 历史数据存储（store.Store 实现）
type MetricStore interface {
	InsertMetrics(ctx context.Context, table string, rows []store.MetricRow) error
	QueryMetrics(ctx context.Context, table, serverID string, from, to int64) ([]metric.Point, error)
	Rollup10m(ctx context.Context, from, to int64) error
	DeleteMetricsBefore(ctx context.Context, table string, ts int64) (int64, error)
}

// maxPending 写库失败时最多暂存的分钟数据条数
const maxPending = 10000

type bucket struct {
	start int64
	pts   []metric.Point
}

// Recorder 把每台服务器的上报聚合为 1 分钟数据，并定期生成 10 分钟数据、清理过期数据
type Recorder struct {
	st  MetricStore
	now func() time.Time
	log *slog.Logger

	mu      sync.Mutex
	cur     map[string]*bucket
	pending []store.MetricRow

	lastRollup  int64
	lastCleanup time.Time
}

// NewRecorder 创建 Recorder
func NewRecorder(st MetricStore, now func() time.Time, log *slog.Logger) *Recorder {
	return &Recorder{st: st, now: now, log: log, cur: map[string]*bucket{}}
}

// Add 把一个点累积到所属分钟；分钟切换时把上一分钟的平均值放入待写队列
func (r *Recorder) Add(id string, p metric.Point) {
	start := p.TS - p.TS%60
	r.mu.Lock()
	defer r.mu.Unlock()
	b := r.cur[id]
	if b != nil && b.start != start {
		r.pending = append(r.pending, store.MetricRow{ServerID: id, Point: metric.Average(b.start, b.pts)})
		b = nil
	}
	if b == nil {
		b = &bucket{start: start}
		r.cur[id] = b
	}
	b.pts = append(b.pts, p)
}

// Flush 写入已结束的分钟；all 为 true 时连同当前未结束的分钟一起写入（退出时使用）
func (r *Recorder) Flush(ctx context.Context, all bool) error {
	r.mu.Lock()
	rows := r.pending
	r.pending = nil
	now := r.now().Unix()
	curMin := now - now%60
	for id, b := range r.cur {
		if all || b.start < curMin {
			rows = append(rows, store.MetricRow{ServerID: id, Point: metric.Average(b.start, b.pts)})
			delete(r.cur, id)
		}
	}
	r.mu.Unlock()
	if len(rows) == 0 {
		return nil
	}
	if err := r.st.InsertMetrics(ctx, store.Metrics1m, rows); err != nil {
		r.mu.Lock()
		r.pending = append(rows, r.pending...)
		if n := len(r.pending); n > maxPending {
			r.pending = r.pending[n-maxPending:]
		}
		r.mu.Unlock()
		return err
	}
	return nil
}

// Tick 每分钟调用：写入 1 分钟数据、生成上一个 10 分钟窗口的数据、每小时清理过期数据
func (r *Recorder) Tick(ctx context.Context) {
	if err := r.Flush(ctx, false); err != nil {
		r.log.Error("写入 1 分钟数据失败", "err", err)
	}
	now := r.now()
	end := now.Unix() - now.Unix()%600
	if end > r.lastRollup {
		if err := r.st.Rollup10m(ctx, end-600, end); err != nil {
			r.log.Error("生成 10 分钟数据失败", "err", err)
		} else {
			r.lastRollup = end
		}
	}
	if now.Sub(r.lastCleanup) >= time.Hour {
		if _, err := r.st.DeleteMetricsBefore(ctx, store.Metrics1m, now.Add(-24*time.Hour).Unix()); err != nil {
			r.log.Error("清理 1 分钟数据失败", "err", err)
		}
		if _, err := r.st.DeleteMetricsBefore(ctx, store.Metrics10m, now.Add(-7*24*time.Hour).Unix()); err != nil {
			r.log.Error("清理 10 分钟数据失败", "err", err)
		}
		r.lastCleanup = now
	}
}

// Run 每分钟执行 Tick；ctx 结束时写入全部剩余数据后返回
func (r *Recorder) Run(ctx context.Context) {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			fctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := r.Flush(fctx, true); err != nil {
				r.log.Error("退出时写入历史数据失败", "err", err)
			}
			cancel()
			return
		case <-t.C:
			r.Tick(ctx)
		}
	}
}

// 支持的时间范围
const (
	RangeRealtime = "realtime"
	Range1h       = "1h"
	Range6h       = "6h"
	Range24h      = "24h"
	Range7d       = "7d"
)

// ErrBadRange 不支持的时间范围
var ErrBadRange = errors.New("不支持的时间范围")

// Query 按范围返回某台服务器的历史点
func Query(ctx context.Context, st MetricStore, ring func(id string) []metric.Point, id, rng string, now time.Time) ([]metric.Point, error) {
	end := now.Unix() + 1
	switch rng {
	case RangeRealtime:
		return ring(id), nil
	case Range1h:
		return st.QueryMetrics(ctx, store.Metrics1m, id, end-3600, end)
	case Range6h:
		return st.QueryMetrics(ctx, store.Metrics1m, id, end-6*3600, end)
	case Range24h:
		pts, err := st.QueryMetrics(ctx, store.Metrics1m, id, end-24*3600, end)
		if err != nil {
			return nil, err
		}
		return metric.Downsample(pts, 300), nil
	case Range7d:
		return st.QueryMetrics(ctx, store.Metrics10m, id, end-7*24*3600, end)
	}
	return nil, ErrBadRange
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go vet ./... && go test ./internal/dashboard/history/ -v`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/dashboard/history
git commit -m "feat(dashboard): 新增历史数据分级降采样与查询"
```

---

### Task 9: 月流量累计（traffic）

**Files:**
- Create: `internal/dashboard/traffic/traffic.go`、`internal/dashboard/traffic/traffic_test.go`

**Interfaces:**
- Consumes: `store.TrafficRow`、`store.ErrNotFound`、`store.Open`（Task 5/6）
- Produces:
  - `type traffic.Store interface{ GetTraffic(ctx, serverID, period string) (store.TrafficRow, error); SaveTraffic(ctx, rows []store.TrafficRow) error }`
  - `func traffic.PeriodStart(t time.Time, resetDay int) time.Time`
  - `func traffic.New(st traffic.Store, now func() time.Time) *traffic.Accumulator`
  - `func (a *Accumulator) Add(ctx, id string, resetDay int, inTotal, outTotal uint64) error`
  - `func (a *Accumulator) Usage(ctx, id string, resetDay int) (in, out int64, period string, err error)`
  - `func (a *Accumulator) Flush(ctx) error`；`Forget(id string)`；`Run(ctx, log *slog.Logger)`（每分钟 Flush，ctx 结束时最后 Flush 一次）

- [ ] **Step 1: 写失败测试**

`internal/dashboard/traffic/traffic_test.go`：

```go
package traffic

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/ruanun/simple-server-status/internal/dashboard/store"
)

var cst = time.FixedZone("CST", 8*3600)

type clock struct{ t time.Time }

func (c *clock) Now() time.Time { return c.t }

func newAcc(t *testing.T, at time.Time) (*Accumulator, *store.Store, *clock) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	c := &clock{t: at}
	return New(st, c.Now), st, c
}

func mustAdd(t *testing.T, a *Accumulator, in, out uint64) {
	t.Helper()
	if err := a.Add(context.Background(), "s", 1, in, out); err != nil {
		t.Fatal(err)
	}
}

func usage(t *testing.T, a *Accumulator) (int64, int64, string) {
	t.Helper()
	in, out, period, err := a.Usage(context.Background(), "s", 1)
	if err != nil {
		t.Fatal(err)
	}
	return in, out, period
}

func TestPeriodStart(t *testing.T) {
	cases := []struct {
		now  time.Time
		day  int
		want string
	}{
		{time.Date(2026, 3, 15, 10, 0, 0, 0, cst), 1, "2026-03-01"},
		{time.Date(2026, 3, 15, 10, 0, 0, 0, cst), 20, "2026-02-20"},
		{time.Date(2026, 1, 5, 0, 0, 0, 0, cst), 10, "2025-12-10"},
		{time.Date(2026, 3, 20, 0, 0, 0, 0, cst), 20, "2026-03-20"},
		{time.Date(2026, 3, 15, 0, 0, 0, 0, cst), 0, "2026-03-01"},
		{time.Date(2026, 3, 15, 0, 0, 0, 0, cst), 31, "2026-03-01"},
	}
	for _, c := range cases {
		if got := PeriodStart(c.now, c.day).Format("2006-01-02"); got != c.want {
			t.Errorf("PeriodStart(%v, %d) = %s，期望 %s", c.now, c.day, got, c.want)
		}
	}
}

func TestAddCountsDeltasAndCounterReset(t *testing.T) {
	a, _, _ := newAcc(t, time.Date(2026, 3, 15, 12, 0, 0, 0, cst))
	mustAdd(t, a, 1000, 500) // 首次上报只建立基线
	if in, out, period := usage(t, a); in != 0 || out != 0 || period != "2026-03-01" {
		t.Fatalf("首次上报后 = %d %d %s", in, out, period)
	}
	mustAdd(t, a, 1600, 700)
	if in, out, _ := usage(t, a); in != 600 || out != 200 {
		t.Fatalf("增量错误: %d %d", in, out)
	}
	mustAdd(t, a, 100, 50) // Agent 重启，计数器归零
	if in, out, _ := usage(t, a); in != 700 || out != 250 {
		t.Fatalf("计数器归零后应以新值为增量: %d %d", in, out)
	}
}

func TestPeriodRollover(t *testing.T) {
	a, st, c := newAcc(t, time.Date(2026, 3, 15, 12, 0, 0, 0, cst))
	mustAdd(t, a, 1000, 1000)
	mustAdd(t, a, 2000, 2000)
	c.t = time.Date(2026, 4, 1, 1, 0, 0, 0, cst)
	mustAdd(t, a, 2500, 2600)
	if in, out, period := usage(t, a); in != 500 || out != 600 || period != "2026-04-01" {
		t.Fatalf("新周期 = %d %d %s", in, out, period)
	}
	old, err := st.GetTraffic(context.Background(), "s", "2026-03-01")
	if err != nil || old.In != 1000 || old.Out != 1000 {
		t.Fatalf("旧周期应已保存: %+v %v", old, err)
	}
}

func TestFlushAndRestore(t *testing.T) {
	a, st, c := newAcc(t, time.Date(2026, 3, 15, 12, 0, 0, 0, cst))
	mustAdd(t, a, 1000, 500)
	mustAdd(t, a, 1600, 700)
	if err := a.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	b := New(st, c.Now) // 模拟 Dashboard 重启
	if in, out, _ := usage(t, b); in != 600 || out != 200 {
		t.Fatalf("重启后应从数据库恢复: %d %d", in, out)
	}
	mustAdd(t, b, 1700, 800)
	if in, out, _ := usage(t, b); in != 700 || out != 300 {
		t.Fatalf("重启后应沿用基线继续累计: %d %d", in, out)
	}
}

func TestUsageUnknownServer(t *testing.T) {
	a, _, _ := newAcc(t, time.Date(2026, 3, 15, 12, 0, 0, 0, cst))
	if in, out, _ := usage(t, a); in != 0 || out != 0 {
		t.Fatalf("未知服务器应为 0: %d %d", in, out)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/dashboard/traffic/`
Expected: FAIL（`undefined: New`）。

- [ ] **Step 3: 实现 traffic**

`internal/dashboard/traffic/traffic.go`：

```go
// Package traffic 按计费周期累计每台服务器的流量。
package traffic

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/ruanun/simple-server-status/internal/dashboard/store"
)

// Store 流量存储（store.Store 实现）
type Store interface {
	GetTraffic(ctx context.Context, serverID, period string) (store.TrafficRow, error)
	SaveTraffic(ctx context.Context, rows []store.TrafficRow) error
}

// PeriodStart 返回 t 所在计费周期的起点（resetDay 日 00:00，使用 t 的时区）；resetDay 超出 1–28 时按 1 处理
func PeriodStart(t time.Time, resetDay int) time.Time {
	if resetDay < 1 || resetDay > 28 {
		resetDay = 1
	}
	y, m, d := t.Date()
	if d < resetDay {
		m-- // time.Date 会把 0 月规范为上一年 12 月
	}
	return time.Date(y, m, resetDay, 0, 0, 0, 0, t.Location())
}

func periodKey(t time.Time, resetDay int) string {
	return PeriodStart(t, resetDay).Format("2006-01-02")
}

type entry struct {
	row   store.TrafficRow
	dirty bool
}

// Accumulator 在内存中累计流量，定期落库
type Accumulator struct {
	st  Store
	now func() time.Time
	mu  sync.Mutex
	m   map[string]*entry
}

// New 创建 Accumulator
func New(st Store, now func() time.Time) *Accumulator {
	return &Accumulator{st: st, now: now, m: map[string]*entry{}}
}

// load 取得 id 在 period 的记录（调用方需持有锁）。
// 内存中是旧周期时先保存旧记录；新周期没有记录时继承旧记录的计数基线
func (a *Accumulator) load(ctx context.Context, id, period string) (*entry, error) {
	old := a.m[id]
	if old != nil && old.row.Period == period {
		return old, nil
	}
	if old != nil && old.dirty {
		if err := a.st.SaveTraffic(ctx, []store.TrafficRow{old.row}); err != nil {
			return nil, err
		}
		old.dirty = false
	}
	row, err := a.st.GetTraffic(ctx, id, period)
	if errors.Is(err, store.ErrNotFound) {
		row = store.TrafficRow{ServerID: id, Period: period, LastIn: -1, LastOut: -1}
		if old != nil {
			row.LastIn, row.LastOut = old.row.LastIn, old.row.LastOut
		}
	} else if err != nil {
		return nil, err
	}
	e := &entry{row: row}
	a.m[id] = e
	return e, nil
}

// Add 根据 Agent 上报的累计计数更新当前周期流量
func (a *Accumulator) Add(ctx context.Context, id string, resetDay int, inTotal, outTotal uint64) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	e, err := a.load(ctx, id, periodKey(a.now(), resetDay))
	if err != nil {
		return err
	}
	in, out := toInt64(inTotal), toInt64(outTotal)
	if e.row.LastIn >= 0 {
		e.row.In += delta(e.row.LastIn, in)
		e.row.Out += delta(e.row.LastOut, out)
	}
	e.row.LastIn, e.row.LastOut = in, out
	e.dirty = true
	return nil
}

// Usage 返回当前周期的入站、出站流量与周期起始日
func (a *Accumulator) Usage(ctx context.Context, id string, resetDay int) (in, out int64, period string, err error) {
	period = periodKey(a.now(), resetDay)
	a.mu.Lock()
	defer a.mu.Unlock()
	e, err := a.load(ctx, id, period)
	if err != nil {
		return 0, 0, period, err
	}
	return e.row.In, e.row.Out, period, nil
}

// Flush 保存所有有变化的记录
func (a *Accumulator) Flush(ctx context.Context) error {
	a.mu.Lock()
	rows := []store.TrafficRow{}
	for _, e := range a.m {
		if e.dirty {
			rows = append(rows, e.row)
			e.dirty = false
		}
	}
	a.mu.Unlock()
	if len(rows) == 0 {
		return nil
	}
	if err := a.st.SaveTraffic(ctx, rows); err != nil {
		a.mu.Lock()
		for _, r := range rows {
			if e := a.m[r.ServerID]; e != nil && e.row.Period == r.Period {
				e.dirty = true
			}
		}
		a.mu.Unlock()
		return err
	}
	return nil
}

// Forget 丢弃某台服务器的内存记录（删除服务器时调用）
func (a *Accumulator) Forget(id string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.m, id)
}

// Run 每分钟保存一次；ctx 结束时最后保存一次后返回
func (a *Accumulator) Run(ctx context.Context, log *slog.Logger) {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			fctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := a.Flush(fctx); err != nil {
				log.Error("退出时保存流量数据失败", "err", err)
			}
			cancel()
			return
		case <-t.C:
			if err := a.Flush(ctx); err != nil {
				log.Error("保存流量数据失败", "err", err)
			}
		}
	}
}

// delta 计算计数增量；计数器回退（重启归零）时以新值作为增量
func delta(last, cur int64) int64 {
	if cur < last {
		return cur
	}
	return cur - last
}

func toInt64(v uint64) int64 {
	if v > math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(v)
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go vet ./... && go test ./internal/dashboard/traffic/ -v`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/dashboard/traffic
git commit -m "feat(dashboard): 新增按计费周期的月流量累计"
```

---

### Task 10: 鉴权（JWT、密码、登录限流）

**Files:**
- Create: `internal/dashboard/auth/auth.go`、`internal/dashboard/auth/auth_test.go`

**Interfaces:**
- Produces:
  - `const auth.MinPasswordLen = 8`
  - `type auth.Claims struct{ UID int64; TV int; jwt.RegisteredClaims }`
  - `func auth.NewManager(secret []byte, now func() time.Time) *auth.Manager`；`(m *Manager) Issue(uid int64, tv int) (string, error)`；`(m *Manager) Parse(token string) (Claims, error)`（HS256，7 天有效）
  - `func auth.HashPassword(p string) (string, error)`；`func auth.CheckPassword(hash, p string) bool`
  - `func auth.NewLimiter(now func() time.Time) *auth.Limiter`；`Allowed(key string) bool`；`Fail(key string)`；`Reset(key string)`（5 分钟内失败 5 次锁定 5 分钟）

- [ ] **Step 1: 写失败测试**

`internal/dashboard/auth/auth_test.go`：

```go
package auth

import (
	"testing"
	"time"
)

type clock struct{ t time.Time }

func (c *clock) Now() time.Time { return c.t }

func TestIssueAndParse(t *testing.T) {
	c := &clock{t: time.Unix(1_700_000_000, 0)}
	m := NewManager([]byte("k"), c.Now)
	tok, err := m.Issue(7, 3)
	if err != nil {
		t.Fatal(err)
	}
	cl, err := m.Parse(tok)
	if err != nil || cl.UID != 7 || cl.TV != 3 {
		t.Fatalf("Parse = %+v, %v", cl, err)
	}
	c.t = c.t.Add(8 * 24 * time.Hour)
	if _, err := m.Parse(tok); err == nil {
		t.Fatal("过期 token 应被拒绝")
	}
}

func TestParseRejectsForeignToken(t *testing.T) {
	c := &clock{t: time.Unix(1_700_000_000, 0)}
	tok, _ := NewManager([]byte("a"), c.Now).Issue(1, 0)
	if _, err := NewManager([]byte("b"), c.Now).Parse(tok); err == nil {
		t.Fatal("其他密钥签发的 token 应被拒绝")
	}
	if _, err := NewManager([]byte("a"), c.Now).Parse("garbage"); err == nil {
		t.Fatal("非法 token 应被拒绝")
	}
}

func TestPassword(t *testing.T) {
	h, err := HashPassword("password123")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPassword(h, "password123") || CheckPassword(h, "wrong") {
		t.Fatal("密码校验错误")
	}
}

func TestLimiterLocksAfterFiveFailures(t *testing.T) {
	c := &clock{t: time.Unix(0, 0)}
	l := NewLimiter(c.Now)
	for i := 0; i < 4; i++ {
		l.Fail("ip")
	}
	if !l.Allowed("ip") {
		t.Fatal("失败 4 次仍应允许")
	}
	l.Fail("ip")
	if l.Allowed("ip") {
		t.Fatal("失败 5 次应锁定")
	}
	if !l.Allowed("other") {
		t.Fatal("不同 key 不应受影响")
	}
	c.t = c.t.Add(5 * time.Minute)
	if !l.Allowed("ip") {
		t.Fatal("锁定 5 分钟后应解除")
	}
}

func TestLimiterWindowAndReset(t *testing.T) {
	c := &clock{t: time.Unix(0, 0)}
	l := NewLimiter(c.Now)
	for i := 0; i < 4; i++ {
		l.Fail("ip")
	}
	c.t = c.t.Add(6 * time.Minute)
	l.Fail("ip")
	if !l.Allowed("ip") {
		t.Fatal("窗口外的失败不应累计")
	}
	for i := 0; i < 4; i++ {
		l.Fail("ip")
	}
	l.Reset("ip")
	l.Fail("ip")
	if !l.Allowed("ip") {
		t.Fatal("Reset 后应重新计数")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/dashboard/auth/`
Expected: FAIL（`undefined: NewManager`）。

- [ ] **Step 3: 实现 auth**

`internal/dashboard/auth/auth.go`：

```go
// Package auth 提供管理员登录所需的 JWT、密码哈希与登录限流。
package auth

import (
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// MinPasswordLen 密码最小长度
const MinPasswordLen = 8

const tokenTTL = 7 * 24 * time.Hour

// Claims JWT 载荷：用户 ID 与 token 版本（改密码后版本 +1，旧 token 失效）
type Claims struct {
	UID int64 `json:"uid"`
	TV  int   `json:"tv"`
	jwt.RegisteredClaims
}

// Manager 签发与校验 JWT
type Manager struct {
	secret []byte
	now    func() time.Time
}

// NewManager 创建 Manager
func NewManager(secret []byte, now func() time.Time) *Manager {
	return &Manager{secret: secret, now: now}
}

// Issue 签发 token
func (m *Manager) Issue(uid int64, tv int) (string, error) {
	now := m.now()
	c := Claims{UID: uid, TV: tv, RegisteredClaims: jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(tokenTTL)),
	}}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(m.secret)
}

// Parse 校验并解析 token
func (m *Manager) Parse(token string) (Claims, error) {
	var c Claims
	_, err := jwt.ParseWithClaims(token, &c, func(*jwt.Token) (any, error) { return m.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithTimeFunc(m.now),
		jwt.WithExpirationRequired())
	return c, err
}

// HashPassword 计算 bcrypt 哈希
func HashPassword(p string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(p), bcrypt.DefaultCost)
	return string(b), err
}

// CheckPassword 校验密码
func CheckPassword(hash, p string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(p)) == nil
}

// Limiter 登录失败限流：同一 key 在 5 分钟内失败 5 次后锁定 5 分钟
type Limiter struct {
	mu     sync.Mutex
	now    func() time.Time
	max    int
	window time.Duration
	lock   time.Duration
	fails  map[string][]time.Time
	until  map[string]time.Time
}

// NewLimiter 创建 Limiter
func NewLimiter(now func() time.Time) *Limiter {
	return &Limiter{now: now, max: 5, window: 5 * time.Minute, lock: 5 * time.Minute,
		fails: map[string][]time.Time{}, until: map[string]time.Time{}}
}

// Allowed 是否允许尝试登录
func (l *Limiter) Allowed(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if t, ok := l.until[key]; ok {
		if l.now().Before(t) {
			return false
		}
		delete(l.until, key)
	}
	return true
}

// Fail 记录一次失败
func (l *Limiter) Fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	var recent []time.Time
	for _, t := range l.fails[key] {
		if now.Sub(t) < l.window {
			recent = append(recent, t)
		}
	}
	recent = append(recent, now)
	if len(recent) >= l.max {
		l.until[key] = now.Add(l.lock)
		delete(l.fails, key)
		return
	}
	l.fails[key] = recent
	if len(l.fails) > 10000 {
		l.sweep(now)
	}
}

// Reset 登录成功后清除记录
func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.fails, key)
	delete(l.until, key)
}

// sweep 清理过期记录，防止内存无限增长（调用方需持有锁）
func (l *Limiter) sweep(now time.Time) {
	for k, ts := range l.fails {
		if len(ts) == 0 || now.Sub(ts[len(ts)-1]) >= l.window {
			delete(l.fails, k)
		}
	}
	for k, t := range l.until {
		if !now.Before(t) {
			delete(l.until, k)
		}
	}
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
go get github.com/golang-jwt/jwt/v5@v5.3.0 golang.org/x/crypto@v0.31.0
go mod tidy
go vet ./...
go test ./internal/dashboard/auth/ -v
```

Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/dashboard/auth go.mod go.sum
git commit -m "feat(dashboard): 新增 JWT、密码哈希与登录限流"
```

---

### Task 11: API 骨架与 Agent 接入

**Files:**
- Create: `internal/dashboard/api/api.go`、`internal/dashboard/api/resp.go`、`internal/dashboard/api/agent.go`
- Test: `internal/dashboard/api/helpers_test.go`、`internal/dashboard/api/agent_test.go`

**Interfaces:**
- Consumes: `store`（Task 5/6）、`hub`（Task 7）、`history.Recorder`（Task 8）、`traffic.Accumulator`（Task 9）、`auth`（Task 10）、`proto`（Task 1）
- Produces:
  - `type api.Deps struct{ Store *store.Store; Hub *hub.Hub; History *history.Recorder; Traffic *traffic.Accumulator; Auth *auth.Manager; Limiter *auth.Limiter; Log *slog.Logger; Now func() time.Time }`
  - `func api.New(ctx context.Context, d api.Deps) (*api.API, error)`；`func (a *API) Router(webFS fs.FS, trustedProxies []string) (*gin.Engine, error)`；`func (a *API) Shutdown()`
  - 内部：`a.reload(ctx) error`、`a.server(id) (store.Server, bool)`、`a.serverList() []store.Server`、`a.currentSettings() store.Settings`、`a.pushConfig(id string)`、`a.handleReport(ctx, id string, r proto.Report)`、`respond(c, data)`、`fail(c, status, code, msg)`、`a.internal(c, msg, err)`
  - 路由：`GET /api/agent/ws`（鉴权失败返回 401 JSON）

- [ ] **Step 1: 写测试辅助与失败测试**

`internal/dashboard/api/helpers_test.go`：

```go
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/coder/websocket"
	"github.com/ruanun/simple-server-status/internal/dashboard/auth"
	"github.com/ruanun/simple-server-status/internal/dashboard/history"
	"github.com/ruanun/simple-server-status/internal/dashboard/hub"
	"github.com/ruanun/simple-server-status/internal/dashboard/store"
	"github.com/ruanun/simple-server-status/internal/dashboard/traffic"
	"github.com/ruanun/simple-server-status/internal/proto"
)

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Add(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

type testEnv struct {
	t     *testing.T
	api   *API
	srv   *httptest.Server
	st    *store.Store
	clock *fakeClock
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	clock := &fakeClock{t: time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)}
	log := slog.New(slog.DiscardHandler)
	a, err := New(context.Background(), Deps{
		Store:   st,
		Hub:     hub.New(clock.Now),
		History: history.NewRecorder(st, clock.Now, log),
		Traffic: traffic.New(st, clock.Now),
		Auth:    auth.NewManager([]byte("test-secret"), clock.Now),
		Limiter: auth.NewLimiter(clock.Now),
		Log:     log,
		Now:     clock.Now,
	})
	if err != nil {
		t.Fatal(err)
	}
	eng, err := a.Router(fstest.MapFS{"index.html": {Data: []byte("<html>app</html>")}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(eng)
	t.Cleanup(func() {
		a.Shutdown()
		srv.Close()
	})
	return &testEnv{t: t, api: a, srv: srv, st: st, clock: clock}
}

// addServer 直接在库中创建服务器并刷新缓存
func (e *testEnv) addServer(s store.Server) store.Server {
	e.t.Helper()
	if err := e.st.CreateServer(context.Background(), &s); err != nil {
		e.t.Fatal(err)
	}
	if err := e.api.reload(context.Background()); err != nil {
		e.t.Fatal(err)
	}
	return s
}

// adminToken 确保存在 admin/password123 并返回其 token
func (e *testEnv) adminToken() string {
	e.t.Helper()
	ctx := context.Background()
	u, err := e.st.GetUserByName(ctx, "admin")
	if errors.Is(err, store.ErrNotFound) {
		hash, _ := auth.HashPassword("password123")
		u = store.User{Username: "admin", PasswordHash: hash}
		if err := e.st.CreateUser(ctx, &u); err != nil {
			e.t.Fatal(err)
		}
	} else if err != nil {
		e.t.Fatal(err)
	}
	tok, err := e.api.Auth.Issue(u.ID, u.TokenVersion)
	if err != nil {
		e.t.Fatal(err)
	}
	return tok
}

// do 发送 JSON 请求，返回状态码与响应体
func (e *testEnv) do(method, path, token string, body any) (int, []byte) {
	e.t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, e.srv.URL+path, r)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

// decodeData 解析 {"data": ...} 响应
func decodeData[T any](t *testing.T, b []byte) T {
	t.Helper()
	var w struct {
		Data T `json:"data"`
	}
	if err := json.Unmarshal(b, &w); err != nil {
		t.Fatalf("解析响应失败: %v, body=%s", err, b)
	}
	return w.Data
}

// errorCode 解析 {"error":{"code":...}} 响应
func errorCode(t *testing.T, b []byte) string {
	t.Helper()
	var w struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(b, &w); err != nil {
		t.Fatalf("解析错误响应失败: %v, body=%s", err, b)
	}
	return w.Error.Code
}

func (e *testEnv) wsURL(path string) string {
	return "ws" + strings.TrimPrefix(e.srv.URL, "http") + path
}

// dialAgent 以 Agent 身份连接
func (e *testEnv) dialAgent(id, secret string) (*websocket.Conn, *http.Response, error) {
	h := http.Header{}
	h.Set("Authorization", "Bearer "+id+":"+secret)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return websocket.Dial(ctx, e.wsURL("/api/agent/ws"), &websocket.DialOptions{HTTPHeader: h})
}

func sendMsg(t *testing.T, conn *websocket.Conn, typ string, data any) {
	t.Helper()
	b, err := proto.Encode(typ, data)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := conn.Write(ctx, websocket.MessageText, b); err != nil {
		t.Fatal(err)
	}
}

func readMsg(t *testing.T, conn *websocket.Conn) proto.Envelope {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, b, err := conn.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	env, err := proto.Decode(b)
	if err != nil {
		t.Fatal(err)
	}
	return env
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("等待条件超时")
}
```

`internal/dashboard/api/agent_test.go`：

```go
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/ruanun/simple-server-status/internal/dashboard/store"
	"github.com/ruanun/simple-server-status/internal/proto"
)

func TestAgentRejectsBadSecret(t *testing.T) {
	e := newTestEnv(t)
	s := e.addServer(store.Server{Name: "a"})
	_, resp, err := e.dialAgent(s.ID, "wrong")
	if err == nil || resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("错误密钥应返回 401，实际 err=%v resp=%v", err, resp)
	}
	_, resp, _ = e.dialAgent("missing", "x")
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatal("不存在的服务器应返回 401")
	}
}

func TestAgentHelloConfigReport(t *testing.T) {
	e := newTestEnv(t)
	s := e.addServer(store.Server{Name: "a", ReportInterval: 3, NICInclude: []string{"eth"}})
	conn, _, err := e.dialAgent(s.ID, s.Secret)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()

	sendMsg(t, conn, proto.TypeHello, proto.Hello{MemTotal: 1000, Country: "JP"})
	env := readMsg(t, conn)
	var cfg proto.Config
	_ = json.Unmarshal(env.Data, &cfg)
	if env.Type != proto.TypeConfig || cfg.ReportInterval != 3 || len(cfg.NICInclude) != 1 || cfg.MountExclude == nil {
		t.Fatalf("config 错误: %s %+v", env.Type, cfg)
	}

	sendMsg(t, conn, proto.TypeReport, proto.Report{CPU: 42, MemUsed: 500, NetInTotal: 100, NetOutTotal: 100})
	sendMsg(t, conn, proto.TypeReport, proto.Report{CPU: 43, MemUsed: 500, NetInTotal: 300, NetOutTotal: 150})
	waitFor(t, func() bool {
		l := e.api.Hub.Get(s.ID)
		return l.Online && l.Report != nil && l.Report.NetInTotal == 300
	})
	in, out, _, _ := e.api.Traffic.Usage(context.Background(), s.ID, 1)
	if in != 200 || out != 50 {
		t.Fatalf("流量累计错误: %d %d", in, out)
	}
	got, _ := e.st.GetServer(context.Background(), s.ID)
	if got.StaticInfo == nil || got.StaticInfo.Country != "JP" || got.LastIP == "" {
		t.Fatalf("静态信息未保存: %+v", got)
	}
}

func TestAgentNewConnectionKicksOld(t *testing.T) {
	e := newTestEnv(t)
	s := e.addServer(store.Server{Name: "a"})
	c1, _, err := e.dialAgent(s.ID, s.Secret)
	if err != nil {
		t.Fatal(err)
	}
	defer c1.CloseNow()
	c2, _, err := e.dialAgent(s.ID, s.Secret)
	if err != nil {
		t.Fatal(err)
	}
	defer c2.CloseNow()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, _, err := c1.Read(ctx); err == nil {
		t.Fatal("旧连接应被断开")
	}
	sendMsg(t, c2, proto.TypeHello, proto.Hello{})
	if env := readMsg(t, c2); env.Type != proto.TypeConfig {
		t.Fatalf("新连接应正常工作，收到 %s", env.Type)
	}
	sendMsg(t, c2, proto.TypeReport, proto.Report{})
	waitFor(t, func() bool { return e.api.Hub.Get(s.ID).Online })
}

func TestAgentBadVersionClosed(t *testing.T) {
	e := newTestEnv(t)
	s := e.addServer(store.Server{Name: "a"})
	conn, _, err := e.dialAgent(s.ID, s.Secret)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = conn.Write(ctx, websocket.MessageText, []byte(`{"v":99,"type":"hello","data":{}}`))
	_, _, err = conn.Read(ctx)
	if websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
		t.Fatalf("协议版本不符应以 PolicyViolation 关闭，实际 %v", err)
	}
}

func TestAgentDisconnectMarksOffline(t *testing.T) {
	e := newTestEnv(t)
	s := e.addServer(store.Server{Name: "a"})
	conn, _, err := e.dialAgent(s.ID, s.Secret)
	if err != nil {
		t.Fatal(err)
	}
	sendMsg(t, conn, proto.TypeHello, proto.Hello{})
	readMsg(t, conn)
	sendMsg(t, conn, proto.TypeReport, proto.Report{})
	waitFor(t, func() bool { return e.api.Hub.Get(s.ID).Online })
	_ = conn.Close(websocket.StatusNormalClosure, "")
	waitFor(t, func() bool { return !e.api.Hub.Get(s.ID).Connected })
	waitFor(t, func() bool {
		got, _ := e.st.GetServer(context.Background(), s.ID)
		return got.LastSeen > 0
	})
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go get github.com/gin-gonic/gin@v1.10.0 && go test ./internal/dashboard/api/`
Expected: FAIL（`undefined: New`、`undefined: Deps`）。

- [ ] **Step 3: 实现 API 骨架**

`internal/dashboard/api/resp.go`：

```go
package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// respond 输出成功响应 {"data": ...}
func respond(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"data": data})
}

// fail 输出错误响应 {"error": {...}} 并中止后续处理
func fail(c *gin.Context, status int, code, msg string) {
	c.AbortWithStatusJSON(status, gin.H{"error": apiError{Code: code, Message: msg}})
}

// internal 记录错误日志并返回 500
func (a *API) internal(c *gin.Context, msg string, err error) {
	a.Log.Error(msg, "err", err)
	fail(c, http.StatusInternalServerError, "internal", msg)
}
```

`internal/dashboard/api/api.go`：

```go
// Package api 提供 Dashboard 的 HTTP 与 WebSocket 接口。
package api

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ruanun/simple-server-status/internal/dashboard/auth"
	"github.com/ruanun/simple-server-status/internal/dashboard/history"
	"github.com/ruanun/simple-server-status/internal/dashboard/hub"
	"github.com/ruanun/simple-server-status/internal/dashboard/store"
	"github.com/ruanun/simple-server-status/internal/dashboard/traffic"
)

// Deps API 依赖的组件
type Deps struct {
	Store   *store.Store
	Hub     *hub.Hub
	History *history.Recorder
	Traffic *traffic.Accumulator
	Auth    *auth.Manager
	Limiter *auth.Limiter
	Log     *slog.Logger
	Now     func() time.Time
}

// API HTTP 接口集合，缓存服务器列表与设置（修改后立即刷新）
type API struct {
	Deps
	mu       sync.RWMutex
	list     []store.Server
	byID     map[string]store.Server
	settings store.Settings
	agents   *agentRegistry
}

// New 创建 API 并加载缓存
func New(ctx context.Context, d Deps) (*API, error) {
	a := &API{Deps: d, agents: newAgentRegistry()}
	if err := a.reload(ctx); err != nil {
		return nil, err
	}
	return a, nil
}

// reload 从数据库刷新服务器与设置缓存
func (a *API) reload(ctx context.Context) error {
	list, err := a.Store.ListServers(ctx)
	if err != nil {
		return fmt.Errorf("读取服务器列表失败: %w", err)
	}
	st, err := a.Store.GetSettings(ctx)
	if err != nil {
		return fmt.Errorf("读取设置失败: %w", err)
	}
	byID := make(map[string]store.Server, len(list))
	for _, s := range list {
		byID[s.ID] = s
	}
	a.mu.Lock()
	a.list, a.byID, a.settings = list, byID, st
	a.mu.Unlock()
	return nil
}

func (a *API) server(id string) (store.Server, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	s, ok := a.byID[id]
	return s, ok
}

func (a *API) serverList() []store.Server {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return slices.Clone(a.list)
}

func (a *API) currentSettings() store.Settings {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.settings
}

// Shutdown 断开所有长连接
func (a *API) Shutdown() {
	a.agents.closeAll()
}

// Router 构建路由；webFS 为前端产物，trustedProxies 为可信反向代理（为空时不信任任何 X-Forwarded-For）
func (a *API) Router(webFS fs.FS, trustedProxies []string) (*gin.Engine, error) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	if err := r.SetTrustedProxies(trustedProxies); err != nil {
		return nil, fmt.Errorf("可信代理配置无效: %w", err)
	}
	r.Use(gin.Recovery())
	r.GET("/api/agent/ws", a.agentWS)
	r.NoRoute(func(c *gin.Context) { fail(c, http.StatusNotFound, "not_found", "接口不存在") })
	return r, nil
}
```

`internal/dashboard/api/agent.go`：

```go
package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/ruanun/simple-server-status/internal/dashboard/store"
	"github.com/ruanun/simple-server-status/internal/proto"
)

type agentConn struct {
	id     string
	send   chan []byte
	cancel context.CancelFunc
}

// agentRegistry 当前在线的 Agent 连接，每台服务器最多一条
type agentRegistry struct {
	mu    sync.Mutex
	conns map[string]*agentConn
}

func newAgentRegistry() *agentRegistry {
	return &agentRegistry{conns: map[string]*agentConn{}}
}

// add 注册新连接，并断开同一服务器的旧连接
func (r *agentRegistry) add(c *agentConn) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if old := r.conns[c.id]; old != nil {
		old.cancel()
	}
	r.conns[c.id] = c
}

// remove 仅当仍是当前连接时移除
func (r *agentRegistry) remove(c *agentConn) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.conns[c.id] == c {
		delete(r.conns, c.id)
	}
}

// send 非阻塞发送；无连接或缓冲已满时返回 false
func (r *agentRegistry) send(id string, msg []byte) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	c := r.conns[id]
	if c == nil {
		return false
	}
	select {
	case c.send <- msg:
		return true
	default:
		return false
	}
}

func (r *agentRegistry) kick(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c := r.conns[id]; c != nil {
		c.cancel()
	}
}

func (r *agentRegistry) closeAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range r.conns {
		c.cancel()
	}
}

// parseAgentAuth 解析 "Bearer <id>:<secret>"
func parseAgentAuth(h string) (id, secret string, ok bool) {
	rest, found := strings.CutPrefix(h, "Bearer ")
	if !found {
		return "", "", false
	}
	id, secret, found = strings.Cut(rest, ":")
	return id, secret, found && id != "" && secret != ""
}

// agentWS Agent 的 WebSocket 入口
func (a *API) agentWS(c *gin.Context) {
	id, secret, okAuth := parseAgentAuth(c.GetHeader("Authorization"))
	srv, found := a.server(id)
	if !okAuth || !found || subtle.ConstantTimeCompare([]byte(secret), []byte(srv.Secret)) != 1 {
		fail(c, http.StatusUnauthorized, "unauthorized", "Agent 鉴权失败")
		return
	}
	conn, err := websocket.Accept(c.Writer, c.Request, nil)
	if err != nil {
		a.Log.Warn("Agent 握手失败", "id", id, "err", err)
		return
	}
	conn.SetReadLimit(64 << 10)

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()
	ac := &agentConn{id: id, send: make(chan []byte, 8), cancel: cancel}
	a.agents.add(ac)
	session := a.Hub.Connect(id, srv.ReportInterval)
	ip := c.ClientIP()
	a.Log.Info("Agent 已连接", "id", id, "name", srv.Name, "ip", ip)

	defer func() {
		a.agents.remove(ac)
		a.Hub.Disconnect(id, session)
		_ = conn.CloseNow()
		sctx, scancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer scancel()
		if err := a.Store.SetLastSeen(sctx, id, a.Now().Unix()); err != nil && !errors.Is(err, store.ErrNotFound) {
			a.Log.Warn("记录最后在线时间失败", "id", id, "err", err)
		}
		if err := a.Traffic.Flush(sctx); err != nil {
			a.Log.Warn("保存流量数据失败", "err", err)
		}
		a.Log.Info("Agent 已断开", "id", id)
	}()

	go a.agentWriter(ctx, conn, ac)
	go a.agentPinger(ctx, conn, ac)
	a.agentReadLoop(ctx, conn, id, ip)
}

func (a *API) agentWriter(ctx context.Context, conn *websocket.Conn, ac *agentConn) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-ac.send:
			wctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := conn.Write(wctx, websocket.MessageText, msg)
			cancel()
			if err != nil {
				ac.cancel()
				return
			}
		}
	}
}

// agentPinger 每 20 秒 ping 一次，20 秒内无 pong 则断开
func (a *API) agentPinger(ctx context.Context, conn *websocket.Conn, ac *agentConn) {
	t := time.NewTicker(20 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			pctx, cancel := context.WithTimeout(ctx, 20*time.Second)
			err := conn.Ping(pctx)
			cancel()
			if err != nil {
				ac.cancel()
				return
			}
		}
	}
}

func (a *API) agentReadLoop(ctx context.Context, conn *websocket.Conn, id, ip string) {
	for {
		_, b, err := conn.Read(ctx)
		if err != nil {
			return
		}
		env, err := proto.Decode(b)
		if errors.Is(err, proto.ErrVersion) {
			_ = conn.Close(websocket.StatusPolicyViolation, "不支持的协议版本")
			return
		}
		if err != nil {
			a.Log.Warn("忽略无法解析的 Agent 消息", "id", id, "err", err)
			continue
		}
		switch env.Type {
		case proto.TypeHello:
			var h proto.Hello
			if err := json.Unmarshal(env.Data, &h); err != nil {
				a.Log.Warn("忽略无效的 hello", "id", id, "err", err)
				continue
			}
			a.handleHello(ctx, id, ip, h)
		case proto.TypeReport:
			var r proto.Report
			if err := json.Unmarshal(env.Data, &r); err != nil {
				a.Log.Warn("忽略无效的 report", "id", id, "err", err)
				continue
			}
			a.handleReport(ctx, id, r)
		default:
			a.Log.Debug("忽略未知 Agent 消息", "id", id, "type", env.Type)
		}
	}
}

func (a *API) handleHello(ctx context.Context, id, ip string, h proto.Hello) {
	if _, ok := a.server(id); !ok {
		return
	}
	a.Hub.SetStatic(id, h)
	if err := a.Store.SetStaticInfo(ctx, id, ip, h); err != nil {
		a.Log.Warn("保存静态信息失败", "id", id, "err", err)
	}
	a.pushConfig(id)
}

// pushConfig 向在线的 Agent 推送当前采集参数
func (a *API) pushConfig(id string) {
	srv, ok := a.server(id)
	if !ok {
		return
	}
	cfg := proto.Config{ReportInterval: srv.ReportInterval, NICInclude: srv.NICInclude, NICExclude: srv.NICExclude, MountExclude: srv.MountExclude}
	cfg.Normalize()
	b, err := proto.Encode(proto.TypeConfig, cfg)
	if err != nil {
		a.Log.Error("编码 config 失败", "err", err)
		return
	}
	a.Hub.SetInterval(id, srv.ReportInterval)
	a.agents.send(id, b)
}

// handleReport 处理一次上报：更新实时状态、历史与流量
func (a *API) handleReport(ctx context.Context, id string, r proto.Report) {
	srv, ok := a.server(id)
	if !ok {
		return
	}
	r.Normalize()
	p := a.Hub.Report(id, r)
	a.History.Add(id, p)
	if err := a.Traffic.Add(ctx, id, srv.TrafficResetDay, r.NetInTotal, r.NetOutTotal); err != nil {
		a.Log.Warn("累计流量失败", "id", id, "err", err)
	}
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
go mod tidy
go vet ./...
go test ./internal/dashboard/api/ -v
```

Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/dashboard/api go.mod go.sum
git commit -m "feat(dashboard): 新增 API 骨架与 Agent WebSocket 接入"
```

---

### Task 12: 公开接口、登录态中间件与浏览器实时推送

**Files:**
- Create: `internal/dashboard/api/auth_mw.go`、`internal/dashboard/api/view.go`、`internal/dashboard/api/public.go`、`internal/dashboard/api/ws.go`
- Modify: `internal/dashboard/api/api.go`
- Test: `internal/dashboard/api/public_test.go`

**Interfaces:**
- Consumes: Task 11 的 API 内部方法；`history.Query`、`history.ErrBadRange`、`history.Range1h`（Task 8）
- Produces:
  - `type api.ServerView`（JSON：`id, name, group, country, sort, hidden, online, last_seen, static, metrics, traffic, expire_at, price?, currency?, billing_cycle?`）；`type api.TrafficView`（`in, out, used, limit, mode, reset_day, period`）
  - 中间件：`a.optionalAuth()`、`a.requireAuth()`；`isAuthed(c) bool`；`currentUser(c) store.User`
  - 路由：`GET /api/public/site`、`GET /api/public/servers`、`GET /api/public/servers/:id`、`GET /api/public/servers/:id/metrics?range=`、`GET /api/public/ws?token=`
  - 浏览器推送消息：`{"type":"snapshot","data":[ServerView...]}`（连接时与服务器列表变更时），`{"type":"delta","data":[ServerView...]}`（每 2 秒，仅有变化的服务器）
  - `func (a *API) RunBroadcaster(ctx context.Context)`；内部 `a.bc.requestSnapshot()`、`a.bc.tick(ctx)`、`a.views(ctx, includeHidden bool) []ServerView`、`a.view(ctx, srv, showPrice bool) ServerView`

- [ ] **Step 1: 写失败测试**

`internal/dashboard/api/public_test.go`：

```go
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/ruanun/simple-server-status/internal/dashboard/metric"
	"github.com/ruanun/simple-server-status/internal/dashboard/store"
	"github.com/ruanun/simple-server-status/internal/proto"
)

func TestPublicListHidesHiddenAndSecrets(t *testing.T) {
	e := newTestEnv(t)
	price := 9.9
	e.addServer(store.Server{Name: "pub", Price: &price, Currency: "USD"})
	e.addServer(store.Server{Name: "hid", Hidden: true})

	code, body := e.do("GET", "/api/public/servers", "", nil)
	if code != http.StatusOK {
		t.Fatalf("状态码 %d", code)
	}
	list := decodeData[[]ServerView](t, body)
	if len(list) != 1 || list[0].Name != "pub" || list[0].Price != nil {
		t.Fatalf("公开列表错误: %+v", list)
	}
	for _, bad := range []string{`"secret"`, `"last_ip"`, `"hid"`, `"price"`} {
		if bytes.Contains(body, []byte(bad)) {
			t.Errorf("公开响应不应包含 %s: %s", bad, body)
		}
	}

	_, body = e.do("GET", "/api/public/servers", e.adminToken(), nil)
	list = decodeData[[]ServerView](t, body)
	if len(list) != 2 {
		t.Fatalf("登录后应看到隐藏服务器: %+v", list)
	}
}

func TestPublicShowPriceSetting(t *testing.T) {
	e := newTestEnv(t)
	price := 9.9
	e.addServer(store.Server{Name: "pub", Price: &price, Currency: "USD"})
	st := store.DefaultSettings()
	st.ShowPrice = true
	if err := e.st.SaveSettings(context.Background(), st); err != nil {
		t.Fatal(err)
	}
	_ = e.api.reload(context.Background())
	_, body := e.do("GET", "/api/public/servers", "", nil)
	list := decodeData[[]ServerView](t, body)
	if list[0].Price == nil || *list[0].Price != 9.9 || list[0].Currency != "USD" {
		t.Fatalf("开启公开价格后应返回价格: %+v", list[0])
	}
}

func TestPublicHiddenServerNotReachable(t *testing.T) {
	e := newTestEnv(t)
	h := e.addServer(store.Server{Name: "hid", Hidden: true})
	for _, path := range []string{"/api/public/servers/" + h.ID, "/api/public/servers/" + h.ID + "/metrics?range=1h"} {
		if code, body := e.do("GET", path, "", nil); code != http.StatusNotFound || errorCode(t, body) != "not_found" {
			t.Errorf("%s 未登录应返回 404，实际 %d %s", path, code, body)
		}
		if code, _ := e.do("GET", path, e.adminToken(), nil); code != http.StatusOK {
			t.Errorf("%s 登录后应返回 200，实际 %d", path, code)
		}
	}
}

func TestPublicDetailNeverReported(t *testing.T) {
	e := newTestEnv(t)
	s := e.addServer(store.Server{Name: "a"})
	code, body := e.do("GET", "/api/public/servers/"+s.ID, "", nil)
	if code != http.StatusOK {
		t.Fatalf("状态码 %d", code)
	}
	for _, want := range []string{`"metrics":null`, `"static":null`, `"online":false`, `"limit":null`} {
		if !bytes.Contains(body, []byte(want)) {
			t.Errorf("响应应包含 %s: %s", want, body)
		}
	}
}

func TestPublicMetrics(t *testing.T) {
	e := newTestEnv(t)
	s := e.addServer(store.Server{Name: "a"})
	ts := e.clock.Now().Unix() - 60
	_ = e.st.InsertMetrics(context.Background(), store.Metrics1m, []store.MetricRow{{ServerID: s.ID, Point: metric.Point{TS: ts, CPU: 12}}})
	code, body := e.do("GET", "/api/public/servers/"+s.ID+"/metrics?range=1h", "", nil)
	pts := decodeData[[]metric.Point](t, body)
	if code != http.StatusOK || len(pts) != 1 || pts[0].CPU != 12 {
		t.Fatalf("历史数据错误: %d %s", code, body)
	}
	code, body = e.do("GET", "/api/public/servers/"+s.ID+"/metrics?range=bad", "", nil)
	if code != http.StatusBadRequest || errorCode(t, body) != "bad_range" {
		t.Fatalf("非法范围应返回 400 bad_range，实际 %d %s", code, body)
	}
}

func TestPublicSite(t *testing.T) {
	e := newTestEnv(t)
	_, body := e.do("GET", "/api/public/site", "", nil)
	site := decodeData[map[string]any](t, body)
	if site["site_title"] != "Simple Server Status" || site["show_price"] != false {
		t.Fatalf("站点信息错误: %v", site)
	}
}

type wsEnvelope struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

func readWS(t *testing.T, conn *websocket.Conn) (string, []ServerView) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, b, err := conn.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var env wsEnvelope
	if err := json.Unmarshal(b, &env); err != nil {
		t.Fatal(err)
	}
	var list []ServerView
	if err := json.Unmarshal(env.Data, &list); err != nil {
		t.Fatal(err)
	}
	return env.Type, list
}

func TestPublicWSSnapshotAndDelta(t *testing.T) {
	e := newTestEnv(t)
	s := e.addServer(store.Server{Name: "pub"})
	h := e.addServer(store.Server{Name: "hid", Hidden: true})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, e.wsURL("/api/public/ws"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()

	typ, list := readWS(t, conn)
	if typ != "snapshot" || len(list) != 1 || list[0].ID != s.ID {
		t.Fatalf("首条应为只含公开服务器的快照: %s %+v", typ, list)
	}

	e.api.Hub.Connect(s.ID, 2)
	e.api.handleReport(ctx, s.ID, proto.Report{CPU: 77})
	e.api.Hub.Connect(h.ID, 2)
	e.api.handleReport(ctx, h.ID, proto.Report{CPU: 1})
	e.api.bc.tick(ctx)

	typ, list = readWS(t, conn)
	if typ != "delta" || len(list) != 1 || list[0].ID != s.ID || list[0].Metrics == nil || list[0].Metrics.CPU != 77 || !list[0].Online {
		t.Fatalf("delta 错误: %s %+v", typ, list)
	}

	e.api.bc.requestSnapshot()
	e.api.bc.tick(ctx)
	if typ, _ := readWS(t, conn); typ != "snapshot" {
		t.Fatalf("请求快照后应推送 snapshot，实际 %s", typ)
	}
}

func TestPublicWSWithTokenIncludesHidden(t *testing.T) {
	e := newTestEnv(t)
	e.addServer(store.Server{Name: "hid", Hidden: true})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, e.wsURL("/api/public/ws?token="+e.adminToken()), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	if _, list := readWS(t, conn); len(list) != 1 {
		t.Fatalf("登录后快照应包含隐藏服务器: %+v", list)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/dashboard/api/`
Expected: FAIL（`undefined: ServerView`、`e.api.bc undefined` 等）。

- [ ] **Step 3: 实现登录态中间件与视图**

`internal/dashboard/api/auth_mw.go`：

```go
package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/ruanun/simple-server-status/internal/dashboard/store"
)

const ctxUserKey = "sss_user"

// bearerToken 从 Authorization 头或 token 查询参数（WebSocket 使用）读取 token
func bearerToken(c *gin.Context) string {
	if t, ok := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer "); ok {
		return t
	}
	return c.Query("token")
}

// authenticate 校验 JWT，并确认 token 版本与数据库一致
func (a *API) authenticate(c *gin.Context) (store.User, bool) {
	tok := bearerToken(c)
	if tok == "" {
		return store.User{}, false
	}
	claims, err := a.Auth.Parse(tok)
	if err != nil {
		return store.User{}, false
	}
	u, err := a.Store.GetUser(c.Request.Context(), claims.UID)
	if err != nil || u.TokenVersion != claims.TV {
		return store.User{}, false
	}
	return u, true
}

// optionalAuth 有合法 token 时记录登录用户，否则按匿名处理
func (a *API) optionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if u, ok := a.authenticate(c); ok {
			c.Set(ctxUserKey, u)
		}
		c.Next()
	}
}

// requireAuth 未登录时返回 401
func (a *API) requireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		u, ok := a.authenticate(c)
		if !ok {
			fail(c, http.StatusUnauthorized, "unauthorized", "请先登录")
			return
		}
		c.Set(ctxUserKey, u)
		c.Next()
	}
}

func isAuthed(c *gin.Context) bool {
	_, ok := c.Get(ctxUserKey)
	return ok
}

func currentUser(c *gin.Context) store.User {
	v, _ := c.Get(ctxUserKey)
	u, _ := v.(store.User)
	return u
}
```

`internal/dashboard/api/view.go`：

```go
package api

import (
	"context"

	"github.com/ruanun/simple-server-status/internal/dashboard/store"
	"github.com/ruanun/simple-server-status/internal/proto"
)

// TrafficView 当前计费周期的流量
type TrafficView struct {
	In       int64  `json:"in"`
	Out      int64  `json:"out"`
	Used     int64  `json:"used"`
	Limit    *int64 `json:"limit"`
	Mode     string `json:"mode"`
	ResetDay int    `json:"reset_day"`
	Period   string `json:"period"`
}

// ServerView 前端展示用的服务器状态（不含 secret、IP）
type ServerView struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Group        string        `json:"group"`
	Country      string        `json:"country"`
	Sort         int           `json:"sort"`
	Hidden       bool          `json:"hidden"`
	Online       bool          `json:"online"`
	LastSeen     int64         `json:"last_seen"`
	Static       *proto.Hello  `json:"static"`
	Metrics      *proto.Report `json:"metrics"`
	Traffic      TrafficView   `json:"traffic"`
	ExpireAt     *int64        `json:"expire_at"`
	Price        *float64      `json:"price,omitempty"`
	Currency     string        `json:"currency,omitempty"`
	BillingCycle string        `json:"billing_cycle,omitempty"`
}

func trafficUsed(mode string, in, out int64) int64 {
	switch mode {
	case "in":
		return in
	case "out":
		return out
	}
	return in + out
}

// view 组合配置、实时状态与流量
func (a *API) view(ctx context.Context, srv store.Server, showPrice bool) ServerView {
	live := a.Hub.Get(srv.ID)
	v := ServerView{
		ID: srv.ID, Name: srv.Name, Group: srv.Group, Country: srv.Country, Sort: srv.Sort, Hidden: srv.Hidden,
		Online: live.Online, LastSeen: srv.LastSeen, Static: srv.StaticInfo, Metrics: live.Report, ExpireAt: srv.ExpireAt,
	}
	if live.Static != nil {
		v.Static = live.Static
	}
	if live.LastReport > 0 {
		v.LastSeen = live.LastReport
	}
	if v.Country == "" && v.Static != nil {
		v.Country = v.Static.Country
	}
	in, out, period, err := a.Traffic.Usage(ctx, srv.ID, srv.TrafficResetDay)
	if err != nil {
		a.Log.Warn("读取流量失败", "id", srv.ID, "err", err)
	}
	v.Traffic = TrafficView{In: in, Out: out, Used: trafficUsed(srv.TrafficMode, in, out), Limit: srv.TrafficLimit,
		Mode: srv.TrafficMode, ResetDay: srv.TrafficResetDay, Period: period}
	if showPrice {
		v.Price, v.Currency, v.BillingCycle = srv.Price, srv.Currency, srv.BillingCycle
	}
	return v
}

// views 返回可见服务器的视图；登录用户可见隐藏服务器与价格
func (a *API) views(ctx context.Context, includeHidden bool) []ServerView {
	showPrice := includeHidden || a.currentSettings().ShowPrice
	list := []ServerView{}
	for _, srv := range a.serverList() {
		if srv.Hidden && !includeHidden {
			continue
		}
		list = append(list, a.view(ctx, srv, showPrice))
	}
	return list
}
```

- [ ] **Step 4: 实现公开接口与浏览器推送**

`internal/dashboard/api/public.go`：

```go
package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ruanun/simple-server-status/internal/dashboard/history"
	"github.com/ruanun/simple-server-status/internal/dashboard/store"
)

func (a *API) registerPublic(r *gin.Engine) {
	g := r.Group("/api/public", a.optionalAuth())
	g.GET("/site", a.publicSite)
	g.GET("/servers", a.publicServers)
	g.GET("/servers/:id", a.publicServer)
	g.GET("/servers/:id/metrics", a.publicMetrics)
	g.GET("/ws", a.publicWS)
}

func (a *API) publicSite(c *gin.Context) {
	st := a.currentSettings()
	respond(c, gin.H{"site_title": st.SiteTitle, "show_price": st.ShowPrice})
}

func (a *API) publicServers(c *gin.Context) {
	respond(c, a.views(c.Request.Context(), isAuthed(c)))
}

// visibleServer 读取调用方可见的服务器，不可见时输出 404
func (a *API) visibleServer(c *gin.Context) (store.Server, bool) {
	srv, found := a.server(c.Param("id"))
	if !found || (srv.Hidden && !isAuthed(c)) {
		fail(c, http.StatusNotFound, "not_found", "服务器不存在")
		return srv, false
	}
	return srv, true
}

func (a *API) publicServer(c *gin.Context) {
	srv, found := a.visibleServer(c)
	if !found {
		return
	}
	respond(c, a.view(c.Request.Context(), srv, isAuthed(c) || a.currentSettings().ShowPrice))
}

func (a *API) publicMetrics(c *gin.Context) {
	srv, found := a.visibleServer(c)
	if !found {
		return
	}
	pts, err := history.Query(c.Request.Context(), a.Store, a.Hub.Ring, srv.ID, c.DefaultQuery("range", history.Range1h), a.Now())
	if errors.Is(err, history.ErrBadRange) {
		fail(c, http.StatusBadRequest, "bad_range", "不支持的时间范围")
		return
	}
	if err != nil {
		a.internal(c, "查询历史数据失败", err)
		return
	}
	respond(c, pts)
}
```

`internal/dashboard/api/ws.go`：

```go
package api

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"
)

type wsClient struct {
	authed bool
	send   chan []byte
	cancel context.CancelFunc
}

type wsMessage struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

// broadcaster 每 2 秒向浏览器推送有变化的服务器
type broadcaster struct {
	a          *API
	mu         sync.Mutex
	clients    map[*wsClient]struct{}
	lastSeq    uint64
	lastOnline map[string]bool
	resnap     atomic.Bool
}

func newBroadcaster(a *API) *broadcaster {
	return &broadcaster{a: a, clients: map[*wsClient]struct{}{}, lastOnline: map[string]bool{}}
}

func (b *broadcaster) add(c *wsClient) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.clients[c] = struct{}{}
}

func (b *broadcaster) remove(c *wsClient) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.clients, c)
}

func (b *broadcaster) closeAll() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for c := range b.clients {
		c.cancel()
	}
}

// requestSnapshot 服务器列表或设置变化后，下一次推送改为完整快照
func (b *broadcaster) requestSnapshot() { b.resnap.Store(true) }

func (b *broadcaster) snapshotClients() []*wsClient {
	b.mu.Lock()
	defer b.mu.Unlock()
	list := make([]*wsClient, 0, len(b.clients))
	for c := range b.clients {
		list = append(list, c)
	}
	return list
}

func (b *broadcaster) encode(typ string, data any) []byte {
	msg, err := json.Marshal(wsMessage{Type: typ, Data: data})
	if err != nil {
		b.a.Log.Error("编码推送消息失败", "err", err)
		return nil
	}
	return msg
}

// push 非阻塞发送；客户端积压时断开，由其重连后重新获取快照
func push(c *wsClient, msg []byte) {
	if msg == nil {
		return
	}
	select {
	case c.send <- msg:
	default:
		c.cancel()
	}
}

// tick 计算变化并推送（仅由 run 或测试调用，不并发）
func (b *broadcaster) tick(ctx context.Context) {
	ids, seq := b.a.Hub.Changed(b.lastSeq)
	b.lastSeq = seq
	changed := make(map[string]bool, len(ids))
	for _, id := range ids {
		changed[id] = true
	}
	servers := b.a.serverList()
	// 超时离线不会产生序号变化，需要单独比较
	for _, srv := range servers {
		on := b.a.Hub.Get(srv.ID).Online
		if b.lastOnline[srv.ID] != on {
			b.lastOnline[srv.ID] = on
			changed[srv.ID] = true
		}
	}
	clients := b.snapshotClients()
	full := b.resnap.Swap(false)
	if len(clients) == 0 {
		return
	}
	if full {
		pub := b.encode("snapshot", b.a.views(ctx, false))
		all := b.encode("snapshot", b.a.views(ctx, true))
		for _, c := range clients {
			if c.authed {
				push(c, all)
			} else {
				push(c, pub)
			}
		}
		return
	}
	if len(changed) == 0 {
		return
	}
	showPrice := b.a.currentSettings().ShowPrice
	pubList, allList := []ServerView{}, []ServerView{}
	for _, srv := range servers {
		if !changed[srv.ID] {
			continue
		}
		allList = append(allList, b.a.view(ctx, srv, true))
		if !srv.Hidden {
			pubList = append(pubList, b.a.view(ctx, srv, showPrice))
		}
	}
	if len(allList) == 0 {
		return
	}
	all := b.encode("delta", allList)
	var pub []byte
	if len(pubList) > 0 {
		pub = b.encode("delta", pubList)
	}
	for _, c := range clients {
		if c.authed {
			push(c, all)
		} else {
			push(c, pub)
		}
	}
}

func (b *broadcaster) run(ctx context.Context) {
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			b.tick(ctx)
		}
	}
}

// RunBroadcaster 启动浏览器推送循环，直到 ctx 结束
func (a *API) RunBroadcaster(ctx context.Context) { a.bc.run(ctx) }

// publicWS 浏览器 WebSocket：连接后先推快照，之后接收 delta
func (a *API) publicWS(c *gin.Context) {
	conn, err := websocket.Accept(c.Writer, c.Request, nil)
	if err != nil {
		a.Log.Debug("浏览器 WebSocket 握手失败", "err", err)
		return
	}
	defer func() { _ = conn.CloseNow() }()
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()
	ctx = conn.CloseRead(ctx)

	cl := &wsClient{authed: isAuthed(c), send: make(chan []byte, 16), cancel: cancel}
	if snap := a.bc.encode("snapshot", a.views(ctx, cl.authed)); snap != nil {
		cl.send <- snap
	}
	a.bc.add(cl)
	defer a.bc.remove(cl)

	ping := time.NewTicker(30 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-ctx.Done():
			_ = conn.Close(websocket.StatusNormalClosure, "")
			return
		case <-ping.C:
			pctx, pcancel := context.WithTimeout(ctx, 10*time.Second)
			err := conn.Ping(pctx)
			pcancel()
			if err != nil {
				return
			}
		case msg := <-cl.send:
			wctx, wcancel := context.WithTimeout(ctx, 10*time.Second)
			err := conn.Write(wctx, websocket.MessageText, msg)
			wcancel()
			if err != nil {
				return
			}
		}
	}
}
```

- [ ] **Step 5: 在 API 中接入 broadcaster 与公开路由**

`internal/dashboard/api/api.go` 做以下四处修改：

结构体增加字段：

```go
	agents   *agentRegistry
	bc       *broadcaster
}
```

`New` 中初始化：

```go
	a := &API{Deps: d, agents: newAgentRegistry()}
	a.bc = newBroadcaster(a)
```

`Shutdown` 改为：

```go
// Shutdown 断开所有长连接
func (a *API) Shutdown() {
	a.agents.closeAll()
	a.bc.closeAll()
}
```

`Router` 中在 `r.GET("/api/agent/ws", a.agentWS)` 之后加入：

```go
	a.registerPublic(r)
```

- [ ] **Step 6: 运行测试确认通过**

Run: `go vet ./... && go test ./internal/dashboard/api/ -v`
Expected: PASS（Task 11 的测试同样通过）。

- [ ] **Step 7: 提交**

```bash
git add internal/dashboard/api
git commit -m "feat(dashboard): 新增公开接口与浏览器实时推送"
```

---

### Task 13: 登录与管理接口

**Files:**
- Create: `internal/dashboard/api/auth_handlers.go`、`internal/dashboard/api/admin.go`、`internal/dashboard/api/admin_io.go`
- Modify: `internal/dashboard/api/api.go`（`Router` 注册管理路由）
- Test: `internal/dashboard/api/admin_test.go`

**Interfaces:**
- Consumes: Task 11/12 的内部方法与中间件；`auth.CheckPassword/HashPassword/MinPasswordLen`（Task 10）
- Produces 路由（除登录外均需 `Authorization: Bearer <jwt>`）：
  - `POST /api/auth/login` `{username,password}` → `{token,username}`；错误 401 `invalid_credentials`，限流 429 `too_many_attempts`
  - `GET /api/auth/me` → `{username}`；`PUT /api/auth/password` `{old_password,new_password}` → `{token}`
  - `GET /api/admin/servers` → `[]store.Server` + `online`；`POST /api/admin/servers`；`PUT /api/admin/servers/:id`；`DELETE /api/admin/servers/:id`
  - `POST /api/admin/servers/:id/reset-secret` → `{secret}`；`GET /api/admin/servers/:id/install?dashboard=<url>` → `{linux,windows}`
  - `PUT /api/admin/server-order` `{ids:[]}`
  - `GET /api/admin/settings`、`PUT /api/admin/settings`（`store.Settings`）
  - `GET /api/admin/export` → `{version:1, exported_at, settings, servers(含 secret)}`；`POST /api/admin/import`（请求体同导出的 data）→ `{servers:n}`
  - 输入校验失败统一 400 `invalid_input`；JSON 格式错误 400 `bad_request`；不存在 404 `not_found`

- [ ] **Step 1: 写失败测试**

`internal/dashboard/api/admin_test.go`：

```go
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ruanun/simple-server-status/internal/dashboard/store"
	"github.com/ruanun/simple-server-status/internal/proto"
)

func login(t *testing.T, e *testEnv, password string) (int, string) {
	t.Helper()
	code, body := e.do("POST", "/api/auth/login", "", gin.H{"username": "admin", "password": password})
	if code != http.StatusOK {
		return code, ""
	}
	return code, decodeData[struct {
		Token string `json:"token"`
	}](t, body).Token
}

func TestLoginFlowAndRateLimit(t *testing.T) {
	e := newTestEnv(t)
	e.adminToken() // 创建 admin/password123
	for i := 0; i < 5; i++ {
		if code, _ := login(t, e, "wrong"); code != http.StatusUnauthorized {
			t.Fatalf("第 %d 次错误密码应返回 401，实际 %d", i+1, code)
		}
	}
	if code, _ := login(t, e, "password123"); code != http.StatusTooManyRequests {
		t.Fatalf("连续失败后应限流，实际 %d", code)
	}
	e.clock.Add(6 * time.Minute)
	code, tok := login(t, e, "password123")
	if code != http.StatusOK || tok == "" {
		t.Fatalf("解锁后应能登录，实际 %d", code)
	}
	if code, body := e.do("GET", "/api/auth/me", tok, nil); code != http.StatusOK || !strings.Contains(string(body), "admin") {
		t.Fatalf("me = %d %s", code, body)
	}
}

func TestChangePasswordRevokesOldToken(t *testing.T) {
	e := newTestEnv(t)
	old := e.adminToken()
	if code, body := e.do("PUT", "/api/auth/password", old, gin.H{"old_password": "password123", "new_password": "short"}); code != http.StatusBadRequest || errorCode(t, body) != "weak_password" {
		t.Fatalf("弱密码应被拒绝: %d %s", code, body)
	}
	if code, body := e.do("PUT", "/api/auth/password", old, gin.H{"old_password": "bad", "new_password": "newpass123"}); code != http.StatusBadRequest || errorCode(t, body) != "wrong_password" {
		t.Fatalf("原密码错误应被拒绝: %d %s", code, body)
	}
	code, body := e.do("PUT", "/api/auth/password", old, gin.H{"old_password": "password123", "new_password": "newpass123"})
	if code != http.StatusOK {
		t.Fatalf("修改密码失败: %d %s", code, body)
	}
	fresh := decodeData[struct {
		Token string `json:"token"`
	}](t, body).Token
	if code, _ := e.do("GET", "/api/auth/me", old, nil); code != http.StatusUnauthorized {
		t.Fatalf("旧 token 应失效，实际 %d", code)
	}
	if code, _ := e.do("GET", "/api/auth/me", fresh, nil); code != http.StatusOK {
		t.Fatalf("新 token 应有效，实际 %d", code)
	}
}

func TestAdminRequiresAuth(t *testing.T) {
	e := newTestEnv(t)
	if code, body := e.do("GET", "/api/admin/servers", "", nil); code != http.StatusUnauthorized || errorCode(t, body) != "unauthorized" {
		t.Fatalf("未登录应返回 401，实际 %d %s", code, body)
	}
}

func createServer(t *testing.T, e *testEnv, tok string, body gin.H) store.Server {
	t.Helper()
	code, b := e.do("POST", "/api/admin/servers", tok, body)
	if code != http.StatusOK {
		t.Fatalf("创建服务器失败: %d %s", code, b)
	}
	return decodeData[store.Server](t, b)
}

func TestAdminServerCRUD(t *testing.T) {
	e := newTestEnv(t)
	tok := e.adminToken()
	if code, body := e.do("POST", "/api/admin/servers", tok, gin.H{"name": "n1", "traffic_reset_day": 40}); code != http.StatusBadRequest || errorCode(t, body) != "invalid_input" {
		t.Fatalf("非法重置日应返回 400，实际 %d %s", code, body)
	}
	a := createServer(t, e, tok, gin.H{"name": " n1 ", "group": "HK", "country": "hk"})
	if a.Name != "n1" || a.Country != "HK" || len(a.Secret) != 32 || a.ReportInterval != 2 || a.TrafficMode != "sum" {
		t.Fatalf("创建结果错误: %+v", a)
	}
	b := createServer(t, e, tok, gin.H{"name": "n0"})

	_, body := e.do("GET", "/api/admin/servers", tok, nil)
	list := decodeData[[]struct {
		store.Server
		Online bool `json:"online"`
	}](t, body)
	if len(list) != 2 || list[0].ID != a.ID || list[0].Secret == "" {
		t.Fatalf("列表错误（应按创建顺序排序且包含 secret）: %+v", list)
	}

	code, body := e.do("PUT", "/api/admin/servers/"+a.ID, tok, gin.H{"name": "n2", "hidden": true})
	if updated := decodeData[store.Server](t, body); code != http.StatusOK || updated.Name != "n2" || !updated.Hidden {
		t.Fatalf("更新失败: %d %s", code, body)
	}
	if code, _ := e.do("PUT", "/api/admin/servers/missing", tok, gin.H{"name": "x"}); code != http.StatusNotFound {
		t.Fatalf("更新不存在的服务器应返回 404，实际 %d", code)
	}

	if code, _ := e.do("PUT", "/api/admin/server-order", tok, gin.H{"ids": []string{b.ID, a.ID}}); code != http.StatusOK {
		t.Fatalf("排序失败: %d", code)
	}
	_, body = e.do("GET", "/api/admin/servers", tok, nil)
	if list := decodeData[[]store.Server](t, body); list[0].ID != b.ID {
		t.Fatalf("排序未生效: %+v", list)
	}

	if code, _ := e.do("DELETE", "/api/admin/servers/"+a.ID, tok, nil); code != http.StatusOK {
		t.Fatalf("删除失败: %d", code)
	}
	if code, _ := e.do("DELETE", "/api/admin/servers/"+a.ID, tok, nil); code != http.StatusNotFound {
		t.Fatalf("重复删除应返回 404，实际 %d", code)
	}
}

func TestAdminUpdatePushesConfig(t *testing.T) {
	e := newTestEnv(t)
	tok := e.adminToken()
	s := createServer(t, e, tok, gin.H{"name": "a"})
	conn, _, err := e.dialAgent(s.ID, s.Secret)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	sendMsg(t, conn, proto.TypeHello, proto.Hello{})
	readMsg(t, conn)
	e.do("PUT", "/api/admin/servers/"+s.ID, tok, gin.H{"name": "a", "report_interval": 5})
	env := readMsg(t, conn)
	var cfg proto.Config
	_ = json.Unmarshal(env.Data, &cfg)
	if env.Type != proto.TypeConfig || cfg.ReportInterval != 5 {
		t.Fatalf("更新后应推送新参数: %s %+v", env.Type, cfg)
	}
}

func TestAdminDeleteSendsStop(t *testing.T) {
	e := newTestEnv(t)
	tok := e.adminToken()
	s := createServer(t, e, tok, gin.H{"name": "a"})
	conn, _, err := e.dialAgent(s.ID, s.Secret)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	sendMsg(t, conn, proto.TypeHello, proto.Hello{})
	readMsg(t, conn)
	e.do("DELETE", "/api/admin/servers/"+s.ID, tok, nil)
	env := readMsg(t, conn)
	var stop proto.Stop
	_ = json.Unmarshal(env.Data, &stop)
	if env.Type != proto.TypeStop || stop.Reason != "deleted" {
		t.Fatalf("删除后应下发 stop: %s %+v", env.Type, stop)
	}
}

func TestAdminResetSecretKicksAgent(t *testing.T) {
	e := newTestEnv(t)
	tok := e.adminToken()
	s := createServer(t, e, tok, gin.H{"name": "a"})
	conn, _, err := e.dialAgent(s.ID, s.Secret)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	sendMsg(t, conn, proto.TypeHello, proto.Hello{})
	readMsg(t, conn)
	code, body := e.do("POST", "/api/admin/servers/"+s.ID+"/reset-secret", tok, nil)
	sec := decodeData[struct {
		Secret string `json:"secret"`
	}](t, body).Secret
	if code != http.StatusOK || sec == "" || sec == s.Secret {
		t.Fatalf("重置密钥失败: %d %s", code, body)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, _, err := conn.Read(ctx); err == nil {
		t.Fatal("重置密钥后旧连接应被断开")
	}
	if _, resp, _ := e.dialAgent(s.ID, s.Secret); resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatal("旧密钥应被拒绝")
	}
	c2, _, err := e.dialAgent(s.ID, sec)
	if err != nil {
		t.Fatalf("新密钥应可连接: %v", err)
	}
	c2.CloseNow()
}

func TestAdminInstallCommand(t *testing.T) {
	e := newTestEnv(t)
	tok := e.adminToken()
	s := createServer(t, e, tok, gin.H{"name": "a"})
	if code, _ := e.do("GET", "/api/admin/servers/"+s.ID+"/install", tok, nil); code != http.StatusBadRequest {
		t.Fatalf("缺少 dashboard 参数应返回 400，实际 %d", code)
	}
	code, body := e.do("GET", "/api/admin/servers/"+s.ID+"/install?dashboard="+url.QueryEscape("https://status.example.com/"), tok, nil)
	cmds := decodeData[map[string]string](t, body)
	wantLinux := "curl -fsSL " + store.DefaultInstallScriptBase + "/install-agent.sh | sudo bash -s -- --dashboard https://status.example.com --id " + s.ID + " --secret " + s.Secret
	if code != http.StatusOK || cmds["linux"] != wantLinux {
		t.Fatalf("Linux 命令错误:\n%s\n期望:\n%s", cmds["linux"], wantLinux)
	}
	if !strings.Contains(cmds["windows"], "-Dashboard https://status.example.com -Id "+s.ID+" -Secret "+s.Secret) {
		t.Fatalf("Windows 命令错误: %s", cmds["windows"])
	}
}

func TestAdminSettings(t *testing.T) {
	e := newTestEnv(t)
	tok := e.adminToken()
	bad := store.Settings{SiteTitle: "x", DefaultReportInterval: 0, InstallScriptBase: "https://x"}
	if code, _ := e.do("PUT", "/api/admin/settings", tok, bad); code != http.StatusBadRequest {
		t.Fatalf("非法间隔应返回 400，实际 %d", code)
	}
	good := store.Settings{SiteTitle: "我的探针", ShowPrice: true, DefaultReportInterval: 5, InstallScriptBase: "https://x/"}
	code, body := e.do("PUT", "/api/admin/settings", tok, good)
	saved := decodeData[store.Settings](t, body)
	if code != http.StatusOK || saved.InstallScriptBase != "https://x" {
		t.Fatalf("保存设置失败: %d %s", code, body)
	}
	_, body = e.do("GET", "/api/public/site", "", nil)
	if site := decodeData[map[string]any](t, body); site["site_title"] != "我的探针" || site["show_price"] != true {
		t.Fatalf("公开站点信息未更新: %v", site)
	}
	if s := createServer(t, e, tok, gin.H{"name": "a"}); s.ReportInterval != 5 {
		t.Fatalf("新服务器应使用默认上报间隔 5，实际 %d", s.ReportInterval)
	}
}

func TestExportImport(t *testing.T) {
	src := newTestEnv(t)
	tok := src.adminToken()
	a := createServer(t, src, tok, gin.H{"name": "a", "price": 9.9, "traffic_limit": 1000})
	code, body := src.do("GET", "/api/admin/export", tok, nil)
	if code != http.StatusOK {
		t.Fatalf("导出失败: %d", code)
	}
	var wrapper struct {
		Data json.RawMessage `json:"data"`
	}
	_ = json.Unmarshal(body, &wrapper)
	if !strings.Contains(string(wrapper.Data), a.Secret) {
		t.Fatal("导出内容应包含 secret")
	}

	dst := newTestEnv(t)
	dtok := dst.adminToken()
	var payload map[string]any
	_ = json.Unmarshal(wrapper.Data, &payload)
	if code, body := dst.do("POST", "/api/admin/import", dtok, payload); code != http.StatusOK {
		t.Fatalf("导入失败: %d %s", code, body)
	}
	_, body = dst.do("GET", "/api/admin/servers", dtok, nil)
	list := decodeData[[]store.Server](t, body)
	if len(list) != 1 || list[0].ID != a.ID || list[0].Secret != a.Secret || list[0].Price == nil || *list[0].Price != 9.9 {
		t.Fatalf("导入结果错误: %+v", list)
	}

	payload["version"] = 2
	if code, body := dst.do("POST", "/api/admin/import", dtok, payload); code != http.StatusBadRequest || errorCode(t, body) != "bad_version" {
		t.Fatalf("不支持的版本应返回 400 bad_version，实际 %d %s", code, body)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/dashboard/api/`
Expected: FAIL（`/api/auth/login` 等返回 404；编译期若无错误则为断言失败）。

- [ ] **Step 3: 实现登录接口**

`internal/dashboard/api/auth_handlers.go`：

```go
package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ruanun/simple-server-status/internal/dashboard/auth"
)

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (a *API) login(c *gin.Context) {
	key := c.ClientIP()
	if !a.Limiter.Allowed(key) {
		fail(c, http.StatusTooManyRequests, "too_many_attempts", "登录失败次数过多，请 5 分钟后再试")
		return
	}
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "bad_request", "请求格式错误")
		return
	}
	u, err := a.Store.GetUserByName(c.Request.Context(), req.Username)
	if err != nil || !auth.CheckPassword(u.PasswordHash, req.Password) {
		a.Limiter.Fail(key)
		fail(c, http.StatusUnauthorized, "invalid_credentials", "用户名或密码错误")
		return
	}
	a.Limiter.Reset(key)
	tok, err := a.Auth.Issue(u.ID, u.TokenVersion)
	if err != nil {
		a.internal(c, "签发登录凭证失败", err)
		return
	}
	respond(c, gin.H{"token": tok, "username": u.Username})
}

func (a *API) me(c *gin.Context) {
	respond(c, gin.H{"username": currentUser(c).Username})
}

type passwordReq struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

func (a *API) changePassword(c *gin.Context) {
	u := currentUser(c)
	var req passwordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "bad_request", "请求格式错误")
		return
	}
	if !auth.CheckPassword(u.PasswordHash, req.OldPassword) {
		fail(c, http.StatusBadRequest, "wrong_password", "原密码错误")
		return
	}
	if len(req.NewPassword) < auth.MinPasswordLen {
		fail(c, http.StatusBadRequest, "weak_password", "新密码至少 8 位")
		return
	}
	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		a.internal(c, "计算密码哈希失败", err)
		return
	}
	if err := a.Store.SetPassword(c.Request.Context(), u.ID, hash); err != nil {
		a.internal(c, "保存密码失败", err)
		return
	}
	tok, err := a.Auth.Issue(u.ID, u.TokenVersion+1)
	if err != nil {
		a.internal(c, "签发登录凭证失败", err)
		return
	}
	respond(c, gin.H{"token": tok})
}
```

- [ ] **Step 4: 实现服务器管理与设置接口**

`internal/dashboard/api/admin.go`：

```go
package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/ruanun/simple-server-status/internal/dashboard/store"
	"github.com/ruanun/simple-server-status/internal/proto"
)

func (a *API) registerAdmin(r *gin.Engine) {
	r.POST("/api/auth/login", a.login)
	g := r.Group("/api", a.requireAuth())
	g.GET("/auth/me", a.me)
	g.PUT("/auth/password", a.changePassword)
	g.GET("/admin/servers", a.adminListServers)
	g.POST("/admin/servers", a.adminCreateServer)
	g.PUT("/admin/servers/:id", a.adminUpdateServer)
	g.DELETE("/admin/servers/:id", a.adminDeleteServer)
	g.POST("/admin/servers/:id/reset-secret", a.adminResetSecret)
	g.GET("/admin/servers/:id/install", a.adminInstall)
	g.PUT("/admin/server-order", a.adminServerOrder)
	g.GET("/admin/settings", a.adminGetSettings)
	g.PUT("/admin/settings", a.adminSaveSettings)
	g.GET("/admin/export", a.adminExport)
	g.POST("/admin/import", a.adminImport)
}

// serverInput 后台可编辑的服务器字段
type serverInput struct {
	Name            string   `json:"name"`
	Group           string   `json:"group"`
	Country         string   `json:"country"`
	Hidden          bool     `json:"hidden"`
	Price           *float64 `json:"price"`
	Currency        string   `json:"currency"`
	BillingCycle    string   `json:"billing_cycle"`
	ExpireAt        *int64   `json:"expire_at"`
	TrafficLimit    *int64   `json:"traffic_limit"`
	TrafficMode     string   `json:"traffic_mode"`
	TrafficResetDay int      `json:"traffic_reset_day"`
	ReportInterval  int      `json:"report_interval"`
	NICInclude      []string `json:"nic_include"`
	NICExclude      []string `json:"nic_exclude"`
	MountExclude    []string `json:"mount_exclude"`
}

var (
	trafficModes  = map[string]bool{"sum": true, "in": true, "out": true}
	billingCycles = map[string]bool{"": true, "monthly": true, "quarterly": true, "yearly": true, "once": true}
)

// normalize 补全默认值并校验
func (in *serverInput) normalize(defaultInterval int) error {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || utf8.RuneCountInString(in.Name) > 64 {
		return errors.New("名称不能为空且不超过 64 个字符")
	}
	in.Group = strings.TrimSpace(in.Group)
	in.Country = strings.ToUpper(strings.TrimSpace(in.Country))
	if in.Country != "" && len(in.Country) != 2 {
		return errors.New("国家代码应为两位字母")
	}
	if in.TrafficMode == "" {
		in.TrafficMode = "sum"
	}
	if !trafficModes[in.TrafficMode] {
		return errors.New("流量计算方式只能是 sum、in 或 out")
	}
	if in.TrafficResetDay == 0 {
		in.TrafficResetDay = 1
	}
	if in.TrafficResetDay < 1 || in.TrafficResetDay > 28 {
		return errors.New("流量重置日应在 1–28 之间")
	}
	if in.ReportInterval == 0 {
		in.ReportInterval = defaultInterval
	}
	if in.ReportInterval < 1 || in.ReportInterval > 60 {
		return errors.New("上报间隔应在 1–60 秒之间")
	}
	if !billingCycles[in.BillingCycle] {
		return errors.New("付费周期无效")
	}
	if in.Price != nil && *in.Price < 0 {
		return errors.New("价格不能为负数")
	}
	if in.TrafficLimit != nil && *in.TrafficLimit < 0 {
		return errors.New("流量配额不能为负数")
	}
	for _, l := range []*[]string{&in.NICInclude, &in.NICExclude, &in.MountExclude} {
		if *l == nil {
			*l = []string{}
		}
	}
	return nil
}

func (in serverInput) apply(s *store.Server) {
	s.Name, s.Group, s.Country, s.Hidden = in.Name, in.Group, in.Country, in.Hidden
	s.Price, s.Currency, s.BillingCycle, s.ExpireAt = in.Price, in.Currency, in.BillingCycle, in.ExpireAt
	s.TrafficLimit, s.TrafficMode, s.TrafficResetDay = in.TrafficLimit, in.TrafficMode, in.TrafficResetDay
	s.ReportInterval, s.NICInclude, s.NICExclude, s.MountExclude = in.ReportInterval, in.NICInclude, in.NICExclude, in.MountExclude
}

func inputFrom(s store.Server) serverInput {
	return serverInput{Name: s.Name, Group: s.Group, Country: s.Country, Hidden: s.Hidden, Price: s.Price,
		Currency: s.Currency, BillingCycle: s.BillingCycle, ExpireAt: s.ExpireAt, TrafficLimit: s.TrafficLimit,
		TrafficMode: s.TrafficMode, TrafficResetDay: s.TrafficResetDay, ReportInterval: s.ReportInterval,
		NICInclude: s.NICInclude, NICExclude: s.NICExclude, MountExclude: s.MountExclude}
}

// bindServerInput 解析并校验请求体，失败时已输出 400
func (a *API) bindServerInput(c *gin.Context) (serverInput, bool) {
	var in serverInput
	if err := c.ShouldBindJSON(&in); err != nil {
		fail(c, http.StatusBadRequest, "bad_request", "请求格式错误")
		return in, false
	}
	if err := in.normalize(a.currentSettings().DefaultReportInterval); err != nil {
		fail(c, http.StatusBadRequest, "invalid_input", err.Error())
		return in, false
	}
	return in, true
}

// afterServerChange 刷新缓存，并让浏览器下一次收到完整快照
func (a *API) afterServerChange(ctx context.Context) {
	if err := a.reload(ctx); err != nil {
		a.Log.Error("刷新服务器缓存失败", "err", err)
	}
	a.bc.requestSnapshot()
}

type adminServer struct {
	store.Server
	Online bool `json:"online"`
}

func (a *API) adminListServers(c *gin.Context) {
	list := []adminServer{}
	for _, s := range a.serverList() {
		list = append(list, adminServer{Server: s, Online: a.Hub.Get(s.ID).Online})
	}
	respond(c, list)
}

func (a *API) adminCreateServer(c *gin.Context) {
	in, ok := a.bindServerInput(c)
	if !ok {
		return
	}
	s := store.Server{Sort: len(a.serverList())}
	in.apply(&s)
	if err := a.Store.CreateServer(c.Request.Context(), &s); err != nil {
		a.internal(c, "创建服务器失败", err)
		return
	}
	a.afterServerChange(c.Request.Context())
	respond(c, s)
}

func (a *API) adminUpdateServer(c *gin.Context) {
	srv, found := a.server(c.Param("id"))
	if !found {
		fail(c, http.StatusNotFound, "not_found", "服务器不存在")
		return
	}
	in, ok := a.bindServerInput(c)
	if !ok {
		return
	}
	in.apply(&srv)
	if err := a.Store.UpdateServer(c.Request.Context(), srv); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			fail(c, http.StatusNotFound, "not_found", "服务器不存在")
			return
		}
		a.internal(c, "更新服务器失败", err)
		return
	}
	a.afterServerChange(c.Request.Context())
	a.pushConfig(srv.ID)
	updated, _ := a.server(srv.ID)
	respond(c, updated)
}

func (a *API) adminDeleteServer(c *gin.Context) {
	id := c.Param("id")
	if err := a.Store.DeleteServer(c.Request.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			fail(c, http.StatusNotFound, "not_found", "服务器不存在")
			return
		}
		a.internal(c, "删除服务器失败", err)
		return
	}
	if msg, err := proto.Encode(proto.TypeStop, proto.Stop{Reason: "deleted"}); err == nil {
		a.agents.send(id, msg)
	}
	// 留出时间让 stop 消息送达，之后强制断开
	time.AfterFunc(2*time.Second, func() { a.agents.kick(id) })
	a.Hub.Remove(id)
	a.Traffic.Forget(id)
	a.afterServerChange(c.Request.Context())
	respond(c, gin.H{})
}

func (a *API) adminResetSecret(c *gin.Context) {
	id := c.Param("id")
	sec, err := a.Store.ResetSecret(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			fail(c, http.StatusNotFound, "not_found", "服务器不存在")
			return
		}
		a.internal(c, "重置密钥失败", err)
		return
	}
	a.afterServerChange(c.Request.Context())
	a.agents.kick(id)
	respond(c, gin.H{"secret": sec})
}

func (a *API) adminServerOrder(c *gin.Context) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "bad_request", "请求格式错误")
		return
	}
	if err := a.Store.SetServerOrder(c.Request.Context(), req.IDs); err != nil {
		a.internal(c, "保存排序失败", err)
		return
	}
	a.afterServerChange(c.Request.Context())
	respond(c, gin.H{})
}

// installCommands 生成 Linux / Windows 一键安装命令
func installCommands(base, dashboard, id, secret string) gin.H {
	base = strings.TrimRight(base, "/")
	linux := fmt.Sprintf("curl -fsSL %s/install-agent.sh | sudo bash -s -- --dashboard %s --id %s --secret %s", base, dashboard, id, secret)
	windows := fmt.Sprintf(`$f="$env:TEMP\install-agent.ps1"; iwr -useb %s/install-agent.ps1 -OutFile $f; & $f -Dashboard %s -Id %s -Secret %s`, base, dashboard, id, secret)
	return gin.H{"linux": linux, "windows": windows}
}

func (a *API) adminInstall(c *gin.Context) {
	srv, found := a.server(c.Param("id"))
	if !found {
		fail(c, http.StatusNotFound, "not_found", "服务器不存在")
		return
	}
	u, err := url.Parse(strings.TrimSpace(c.Query("dashboard")))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		fail(c, http.StatusBadRequest, "bad_request", "缺少或无效的 dashboard 参数")
		return
	}
	dash := strings.TrimRight(u.Scheme+"://"+u.Host+u.EscapedPath(), "/")
	respond(c, installCommands(a.currentSettings().InstallScriptBase, dash, srv.ID, srv.Secret))
}

// normalizeSettings 补全默认值并校验设置
func normalizeSettings(st *store.Settings) error {
	st.SiteTitle = strings.TrimSpace(st.SiteTitle)
	if st.SiteTitle == "" {
		st.SiteTitle = store.DefaultSettings().SiteTitle
	}
	if st.DefaultReportInterval < 1 || st.DefaultReportInterval > 60 {
		return errors.New("默认上报间隔应在 1–60 秒之间")
	}
	st.InstallScriptBase = strings.TrimRight(strings.TrimSpace(st.InstallScriptBase), "/")
	if st.InstallScriptBase == "" {
		st.InstallScriptBase = store.DefaultInstallScriptBase
	}
	if !strings.HasPrefix(st.InstallScriptBase, "http://") && !strings.HasPrefix(st.InstallScriptBase, "https://") {
		return errors.New("安装脚本地址需以 http:// 或 https:// 开头")
	}
	return nil
}

func (a *API) adminGetSettings(c *gin.Context) {
	respond(c, a.currentSettings())
}

func (a *API) adminSaveSettings(c *gin.Context) {
	var st store.Settings
	if err := c.ShouldBindJSON(&st); err != nil {
		fail(c, http.StatusBadRequest, "bad_request", "请求格式错误")
		return
	}
	if err := normalizeSettings(&st); err != nil {
		fail(c, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	if err := a.Store.SaveSettings(c.Request.Context(), st); err != nil {
		a.internal(c, "保存设置失败", err)
		return
	}
	a.afterServerChange(c.Request.Context())
	respond(c, st)
}
```

`internal/dashboard/api/admin_io.go`：

```go
package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ruanun/simple-server-status/internal/dashboard/store"
)

// exportFile 导出 / 导入文件格式（包含服务器 secret，便于整机迁移）
type exportFile struct {
	Version    int            `json:"version"`
	ExportedAt int64          `json:"exported_at"`
	Settings   store.Settings `json:"settings"`
	Servers    []store.Server `json:"servers"`
}

func (a *API) adminExport(c *gin.Context) {
	list := a.serverList()
	for i := range list {
		list[i].StaticInfo = nil
		list[i].LastIP = ""
		list[i].LastSeen = 0
	}
	respond(c, exportFile{Version: 1, ExportedAt: a.Now().Unix(), Settings: a.currentSettings(), Servers: list})
}

func (a *API) adminImport(c *gin.Context) {
	var f exportFile
	if err := c.ShouldBindJSON(&f); err != nil {
		fail(c, http.StatusBadRequest, "bad_request", "文件格式错误")
		return
	}
	if f.Version != 1 {
		fail(c, http.StatusBadRequest, "bad_version", "不支持的导出文件版本")
		return
	}
	if err := normalizeSettings(&f.Settings); err != nil {
		fail(c, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	for i := range f.Servers {
		s := &f.Servers[i]
		if s.ID == "" || s.Secret == "" {
			fail(c, http.StatusBadRequest, "invalid_input", "服务器缺少 id 或 secret")
			return
		}
		in := inputFrom(*s)
		if err := in.normalize(f.Settings.DefaultReportInterval); err != nil {
			fail(c, http.StatusBadRequest, "invalid_input", fmt.Sprintf("服务器 %s：%v", s.ID, err))
			return
		}
		in.apply(s)
	}
	ctx := c.Request.Context()
	if err := a.Store.UpsertServers(ctx, f.Servers); err != nil {
		a.internal(c, "导入服务器失败", err)
		return
	}
	if err := a.Store.SaveSettings(ctx, f.Settings); err != nil {
		a.internal(c, "导入设置失败", err)
		return
	}
	a.afterServerChange(ctx)
	for _, s := range f.Servers {
		a.pushConfig(s.ID)
	}
	respond(c, gin.H{"servers": len(f.Servers)})
}
```

- [ ] **Step 5: 注册管理路由**

`internal/dashboard/api/api.go` 的 `Router` 中，在 `a.registerPublic(r)` 之后加入：

```go
	a.registerAdmin(r)
```

- [ ] **Step 6: 运行测试确认通过**

Run: `go vet ./... && go test ./internal/dashboard/api/ -v`
Expected: PASS。

- [ ] **Step 7: 提交**

```bash
git add internal/dashboard/api
git commit -m "feat(dashboard): 新增登录、服务器管理、设置与导入导出接口"
```

---

### Task 14: 前端嵌入、启动流程与 sss-dashboard 命令行

**Files:**
- Create: `internal/dashboard/web/web.go`、`internal/dashboard/web/dist/.gitkeep`
- Create: `internal/dashboard/api/static.go`、`internal/dashboard/api/static_test.go`
- Modify: `internal/dashboard/api/api.go`（`NoRoute` 改为前端回退）
- Create: `internal/dashboard/app.go`、`internal/dashboard/app_test.go`
- Create: `cmd/sss-dashboard/main.go`、`cmd/sss-dashboard/main_test.go`
- Modify: `.gitignore`、`Makefile`

**Interfaces:**
- Consumes: 前述全部包
- Produces:
  - `func web.FS() fs.FS`（嵌入的 `dist` 目录）
  - `func spaHandler(webFS fs.FS) gin.HandlerFunc`（非 `/api/` 且无扩展名的 GET 回退 `index.html`；`assets/` 下文件加长期缓存头）
  - `type dashboard.Options struct{ Listen string; Listener net.Listener; DataDir string; TrustedProxies []string; AdminPassword string; Log *slog.Logger; Version string; WebFS fs.FS }`
  - `const dashboard.DBFile = "sss.db"`；`func dashboard.Run(ctx, o Options) error`（ctx 结束后优雅退出并写入剩余数据）；`func dashboard.ResetPassword(ctx, dataDir string) (string, error)`
  - 可执行文件 `sss-dashboard`：默认 `serve`，子命令 `reset-password`、`version`；参数 `--listen`、`--data-dir`、`--trusted-proxies`、`--admin-password`、`--log-level`、`--log-file`（环境变量 `SSS_*`）

- [ ] **Step 1: 写失败测试**

`internal/dashboard/api/static_test.go`：

```go
package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gin-gonic/gin"
)

func serveStatic(t *testing.T, fsys fstest.MapFS, path string) *http.Response {
	t.Helper()
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.NoRoute(spaHandler(fsys))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	return w.Result()
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}

func TestSPAHandler(t *testing.T) {
	fsys := fstest.MapFS{
		"index.html":    {Data: []byte("<html>app</html>")},
		"assets/app.js": {Data: []byte("console.log(1)")},
		"favicon.svg":   {Data: []byte("<svg/>")},
	}
	for _, p := range []string{"/", "/server/abc", "/admin/servers"} {
		resp := serveStatic(t, fsys, p)
		if resp.StatusCode != http.StatusOK || !strings.Contains(readBody(t, resp), "app") {
			t.Errorf("%s 应返回 index.html，实际 %d", p, resp.StatusCode)
		}
	}
	resp := serveStatic(t, fsys, "/assets/app.js")
	if resp.StatusCode != http.StatusOK || readBody(t, resp) != "console.log(1)" || !strings.Contains(resp.Header.Get("Cache-Control"), "immutable") {
		t.Errorf("静态资源错误: %d %q", resp.StatusCode, resp.Header.Get("Cache-Control"))
	}
	if resp := serveStatic(t, fsys, "/favicon.svg"); resp.StatusCode != http.StatusOK || resp.Header.Get("Cache-Control") != "" {
		t.Errorf("非 assets 文件不应长期缓存: %d %q", resp.StatusCode, resp.Header.Get("Cache-Control"))
	}
	if resp := serveStatic(t, fsys, "/missing.js"); resp.StatusCode != http.StatusNotFound {
		t.Errorf("不存在的带扩展名文件应返回 404，实际 %d", resp.StatusCode)
	}
	resp = serveStatic(t, fsys, "/api/unknown")
	if resp.StatusCode != http.StatusNotFound || !strings.Contains(readBody(t, resp), "not_found") {
		t.Errorf("未知 API 应返回 JSON 404，实际 %d", resp.StatusCode)
	}
	resp = serveStatic(t, fstest.MapFS{}, "/")
	if resp.StatusCode != http.StatusNotFound || !strings.Contains(readBody(t, resp), "前端未构建") {
		t.Errorf("未构建前端时应提示，实际 %d", resp.StatusCode)
	}
}
```

`internal/dashboard/app_test.go`：

```go
package dashboard

import (
	"context"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/ruanun/simple-server-status/internal/dashboard/auth"
	"github.com/ruanun/simple-server-status/internal/dashboard/store"
)

func TestBootstrapAdminAndResetPassword(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, DBFile))
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.DiscardHandler)
	if err := bootstrapAdmin(ctx, st, "password123", log); err != nil {
		t.Fatal(err)
	}
	if err := bootstrapAdmin(ctx, st, "other-password", log); err != nil {
		t.Fatal(err)
	}
	u, err := st.GetUserByName(ctx, "admin")
	if err != nil || !auth.CheckPassword(u.PasswordHash, "password123") {
		t.Fatalf("应只在首次创建管理员: %+v %v", u, err)
	}
	_ = st.Close()

	pw, err := ResetPassword(ctx, dir)
	if err != nil || len(pw) != 16 {
		t.Fatalf("ResetPassword = %q, %v", pw, err)
	}
	st, _ = store.Open(filepath.Join(dir, DBFile))
	defer st.Close()
	u, _ = st.GetUserByName(ctx, "admin")
	if !auth.CheckPassword(u.PasswordHash, pw) || u.TokenVersion != 1 {
		t.Fatalf("重置后密码或 token 版本错误: %+v", u)
	}
}
```

`cmd/sss-dashboard/main_test.go`：

```go
package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionCommand(t *testing.T) {
	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"version"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out.String()) != "dev" {
		t.Fatalf("version 输出 %q", out.String())
	}
}

func TestServeFlagsRegistered(t *testing.T) {
	root := newRootCmd()
	for _, name := range []string{"listen", "trusted-proxies", "admin-password", "data-dir", "log-level", "log-file"} {
		if root.Flags().Lookup(name) == nil && root.PersistentFlags().Lookup(name) == nil {
			t.Errorf("缺少参数 --%s", name)
		}
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/dashboard/ ./internal/dashboard/api/ ./cmd/sss-dashboard/`
Expected: FAIL（`undefined: spaHandler`、`undefined: bootstrapAdmin`、`undefined: newRootCmd`）。

- [ ] **Step 3: 实现前端嵌入与回退**

创建空文件 `internal/dashboard/web/dist/.gitkeep`。

`internal/dashboard/web/web.go`：

```go
// Package web 嵌入前端构建产物（由 web/ 构建输出到 dist 目录）。
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// FS 返回前端产物根目录
func FS() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}
```

`internal/dashboard/api/static.go`：

```go
package api

import (
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

// spaHandler 提供前端静态资源；非 /api/ 且无扩展名的 GET 请求回退到 index.html
func spaHandler(webFS fs.FS) gin.HandlerFunc {
	fileServer := http.FileServer(http.FS(webFS))
	return func(c *gin.Context) {
		p := c.Request.URL.Path
		if (c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead) || strings.HasPrefix(p, "/api/") {
			fail(c, http.StatusNotFound, "not_found", "接口不存在")
			return
		}
		name := strings.TrimPrefix(path.Clean(p), "/")
		if name != "" && name != "index.html" {
			if st, err := fs.Stat(webFS, name); err == nil && !st.IsDir() {
				if strings.HasPrefix(name, "assets/") {
					c.Header("Cache-Control", "public, max-age=31536000, immutable")
				}
				fileServer.ServeHTTP(c.Writer, c.Request)
				return
			}
		}
		if path.Ext(name) != "" && name != "index.html" {
			c.String(http.StatusNotFound, "not found")
			return
		}
		index, err := fs.ReadFile(webFS, "index.html")
		if err != nil {
			c.String(http.StatusNotFound, "前端未构建：请先执行 make build-web")
			return
		}
		c.Header("Cache-Control", "no-cache")
		c.Data(http.StatusOK, "text/html; charset=utf-8", index)
	}
}
```

`internal/dashboard/api/api.go` 的 `Router` 中，把

```go
	r.NoRoute(func(c *gin.Context) { fail(c, http.StatusNotFound, "not_found", "接口不存在") })
```

替换为

```go
	r.NoRoute(spaHandler(webFS))
```

替换后若 `net/http` 在 `api.go` 中不再被使用，从该文件的 import 中删除。

- [ ] **Step 4: 实现启动流程**

`internal/dashboard/app.go`：

```go
// Package dashboard 组装并运行 Dashboard 服务。
package dashboard

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ruanun/simple-server-status/internal/dashboard/api"
	"github.com/ruanun/simple-server-status/internal/dashboard/auth"
	"github.com/ruanun/simple-server-status/internal/dashboard/history"
	"github.com/ruanun/simple-server-status/internal/dashboard/hub"
	"github.com/ruanun/simple-server-status/internal/dashboard/randx"
	"github.com/ruanun/simple-server-status/internal/dashboard/store"
	"github.com/ruanun/simple-server-status/internal/dashboard/traffic"
)

// DBFile 数据库文件名
const DBFile = "sss.db"

// Options 启动参数
type Options struct {
	Listen         string
	Listener       net.Listener // 非空时忽略 Listen（测试用）
	DataDir        string
	TrustedProxies []string
	AdminPassword  string // 仅首次初始化时使用，为空则随机生成
	Log            *slog.Logger
	Version        string
	WebFS          fs.FS
}

// Run 启动服务，直到 ctx 结束后优雅退出
func Run(ctx context.Context, o Options) error {
	if o.Log == nil {
		o.Log = slog.Default()
	}
	if err := os.MkdirAll(o.DataDir, 0o750); err != nil {
		return fmt.Errorf("创建数据目录失败: %w", err)
	}
	st, err := store.Open(filepath.Join(o.DataDir, DBFile))
	if err != nil {
		return err
	}
	defer func() { _ = st.Close() }()
	if err := bootstrapAdmin(ctx, st, o.AdminPassword, o.Log); err != nil {
		return err
	}
	secret, err := st.JWTSecret(ctx)
	if err != nil {
		return fmt.Errorf("读取 JWT 密钥失败: %w", err)
	}

	now := time.Now
	rec := history.NewRecorder(st, now, o.Log)
	tr := traffic.New(st, now)
	a, err := api.New(ctx, api.Deps{
		Store: st, Hub: hub.New(now), History: rec, Traffic: tr,
		Auth: auth.NewManager(secret, now), Limiter: auth.NewLimiter(now), Log: o.Log, Now: now,
	})
	if err != nil {
		return err
	}
	eng, err := a.Router(o.WebFS, o.TrustedProxies)
	if err != nil {
		return err
	}
	ln := o.Listener
	if ln == nil {
		if ln, err = net.Listen("tcp", o.Listen); err != nil {
			return fmt.Errorf("监听 %s 失败: %w", o.Listen, err)
		}
	}
	srv := &http.Server{Handler: eng, ReadHeaderTimeout: 10 * time.Second}

	bgCtx, bgCancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(3)
	go func() { defer wg.Done(); rec.Run(bgCtx) }()
	go func() { defer wg.Done(); tr.Run(bgCtx, o.Log) }()
	go func() { defer wg.Done(); a.RunBroadcaster(bgCtx) }()

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()
	o.Log.Info("Dashboard 已启动", "addr", ln.Addr().String(), "version", o.Version, "data_dir", o.DataDir)

	var serveErr error
	select {
	case <-ctx.Done():
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			serveErr = err
		}
	}
	a.Shutdown()
	sctx, scancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer scancel()
	_ = srv.Shutdown(sctx)
	bgCancel()
	wg.Wait() // 各后台任务退出前会写入剩余数据
	o.Log.Info("Dashboard 已停止")
	return serveErr
}

// bootstrapAdmin 数据库中没有用户时创建 admin
func bootstrapAdmin(ctx context.Context, st *store.Store, password string, log *slog.Logger) error {
	n, err := st.CountUsers(ctx)
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	generated := password == ""
	if generated {
		password = randx.String(16)
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	if err := st.CreateUser(ctx, &store.User{Username: "admin", PasswordHash: hash}); err != nil {
		return fmt.Errorf("创建管理员失败: %w", err)
	}
	if generated {
		log.Warn("已创建管理员账户，请登录后立即修改密码", "username", "admin", "password", password)
	} else {
		log.Info("已使用指定密码创建管理员账户", "username", "admin")
	}
	return nil
}

// ResetPassword 为 admin 生成新随机密码并使旧登录失效，返回新密码
func ResetPassword(ctx context.Context, dataDir string) (string, error) {
	st, err := store.Open(filepath.Join(dataDir, DBFile))
	if err != nil {
		return "", err
	}
	defer func() { _ = st.Close() }()
	pw := randx.String(16)
	hash, err := auth.HashPassword(pw)
	if err != nil {
		return "", err
	}
	u, err := st.GetUserByName(ctx, "admin")
	if errors.Is(err, store.ErrNotFound) {
		return pw, st.CreateUser(ctx, &store.User{Username: "admin", PasswordHash: hash})
	}
	if err != nil {
		return "", err
	}
	return pw, st.SetPassword(ctx, u.ID, hash)
}
```

- [ ] **Step 5: 实现命令行**

`cmd/sss-dashboard/main.go`：

```go
// sss-dashboard 是 Simple Server Status 的面板服务。
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/ruanun/simple-server-status/internal/cliutil"
	"github.com/ruanun/simple-server-status/internal/dashboard"
	"github.com/ruanun/simple-server-status/internal/dashboard/web"
	"github.com/ruanun/simple-server-status/internal/logx"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// version 由构建时 -ldflags "-X main.version=..." 注入
var version = "dev"

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:          "sss-dashboard",
		Short:        "Simple Server Status 面板",
		SilenceUsage: true,
		RunE:         runServe,
	}
	pf := root.PersistentFlags()
	pf.String("data-dir", "./data", "数据目录（SQLite 数据库所在位置）")
	pf.String("log-level", "info", "日志级别：debug/info/warn/error")
	pf.String("log-file", "", "日志文件路径，留空只输出到标准输出")
	addServeFlags(root.Flags())

	serve := &cobra.Command{Use: "serve", Short: "启动面板服务（默认）", RunE: runServe}
	addServeFlags(serve.Flags())
	reset := &cobra.Command{Use: "reset-password", Short: "重置 admin 密码并使已登录会话失效", RunE: runResetPassword}
	ver := &cobra.Command{Use: "version", Short: "显示版本", Run: func(cmd *cobra.Command, _ []string) {
		fmt.Fprintln(cmd.OutOrStdout(), version)
	}}
	root.AddCommand(serve, reset, ver)
	return root
}

func addServeFlags(fs *pflag.FlagSet) {
	fs.String("listen", ":8900", "监听地址")
	fs.StringSlice("trusted-proxies", nil, "可信反向代理的 IP 或 CIDR，逗号分隔")
	fs.String("admin-password", "", "首次初始化时的管理员密码，留空则随机生成并打印到日志")
}

func runServe(cmd *cobra.Command, _ []string) error {
	fs := cmd.Flags()
	if err := cliutil.ApplyEnv(fs, "SSS"); err != nil {
		return err
	}
	dataDir, _ := fs.GetString("data-dir")
	listen, _ := fs.GetString("listen")
	proxies, _ := fs.GetStringSlice("trusted-proxies")
	adminPw, _ := fs.GetString("admin-password")
	level, _ := fs.GetString("log-level")
	file, _ := fs.GetString("log-file")

	log, closeLog, err := logx.New(level, file)
	if err != nil {
		return err
	}
	defer func() { _ = closeLog() }()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return dashboard.Run(ctx, dashboard.Options{
		Listen: listen, DataDir: dataDir, TrustedProxies: proxies, AdminPassword: adminPw,
		Log: log, Version: version, WebFS: web.FS(),
	})
}

func runResetPassword(cmd *cobra.Command, _ []string) error {
	fs := cmd.Flags()
	if err := cliutil.ApplyEnv(fs, "SSS"); err != nil {
		return err
	}
	dataDir, _ := fs.GetString("data-dir")
	pw, err := dashboard.ResetPassword(cmd.Context(), dataDir)
	if err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "admin 新密码：%s\n", pw)
	return nil
}
```

- [ ] **Step 6: 更新 .gitignore 与 Makefile**

`.gitignore` 中把

```
!dashboard/public/dist/README.md
```

替换为

```
!internal/dashboard/web/dist
internal/dashboard/web/dist/*
!internal/dashboard/web/dist/.gitkeep
```

`Makefile` 中做以下替换：

- `go build -o $(BIN_DIR)/$(BINARY_DASHBOARD) ./cmd/dashboard`（出现两处）全部替换为 `go build -o $(BIN_DIR)/$(BINARY_DASHBOARD) ./cmd/sss-dashboard`
- `find internal/dashboard/public/dist -mindepth 1 ! -name '.gitkeep' ! -name 'README.md' -delete 2>/dev/null || true`（出现两处）全部替换为 `find internal/dashboard/web/dist -mindepth 1 ! -name '.gitkeep' -delete 2>/dev/null || true`

- [ ] **Step 7: 运行测试并构建**

```bash
go mod tidy
go vet ./...
go test ./...
git check-ignore -v internal/dashboard/web/dist/.gitkeep; echo "exit=$?"
go build -o bin/sss-dashboard$(go env GOEXE) ./cmd/sss-dashboard && ./bin/sss-dashboard$(go env GOEXE) version
```

Expected: 测试 PASS；`git check-ignore` 无输出且 `exit=1`（`.gitkeep` 未被忽略）；最后输出 `dev`。

- [ ] **Step 8: 本地冒烟运行**

```bash
./bin/sss-dashboard$(go env GOEXE) --listen 127.0.0.1:18900 --data-dir .tmp-smoke --admin-password password123 &
sleep 2
curl -s http://127.0.0.1:18900/api/public/site
curl -s -o /dev/null -w "%{http_code}\n" http://127.0.0.1:18900/
kill %1
rm -rf .tmp-smoke
```

Expected: 第一条输出 `{"data":{"show_price":false,"site_title":"Simple Server Status"}}`；第二条输出 `404`（前端尚未构建，提示"前端未构建"）。

- [ ] **Step 9: 提交**

```bash
git add internal/dashboard cmd/sss-dashboard .gitignore Makefile go.mod go.sum
git commit -m "feat(dashboard): 新增前端嵌入、启动流程与 sss-dashboard 命令行"
```

---

### Task 15: 端到端冒烟测试与收尾检查

**Files:**
- Create: `internal/dashboard/e2e_test.go`
- Modify: `.golangci.yml`

**Interfaces:**
- Consumes: `dashboard.Run`（Task 14）；`agent.NewClient`、`agent.ErrStopped`（Task 3）；`collect.New`（Task 2）

- [ ] **Step 1: 写端到端测试**

`internal/dashboard/e2e_test.go`：

```go
package dashboard_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"testing/fstest"
	"time"

	"github.com/coder/websocket"
	"github.com/ruanun/simple-server-status/internal/agent"
	"github.com/ruanun/simple-server-status/internal/agent/collect"
	"github.com/ruanun/simple-server-status/internal/dashboard"
)

func call[T any](t *testing.T, method, url, token string, body any) T {
	t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, url, r)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var w struct {
		Data T `json:"data"`
	}
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s %s 返回 %d: %s", method, url, resp.StatusCode, b)
	}
	if err := json.Unmarshal(b, &w); err != nil {
		t.Fatalf("解析响应失败: %v %s", err, b)
	}
	return w.Data
}

func waitUntil(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("等待条件超时")
}

func TestEndToEnd(t *testing.T) {
	dataDir := t.TempDir()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	base := "http://" + ln.Addr().String()
	log := slog.New(slog.DiscardHandler)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- dashboard.Run(ctx, dashboard.Options{
			Listener: ln, DataDir: dataDir, AdminPassword: "password123", Log: log, Version: "e2e",
			WebFS: fstest.MapFS{"index.html": {Data: []byte("<html></html>")}},
		})
	}()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("Run 返回错误: %v", err)
		}
	})

	waitUntil(t, 5*time.Second, func() bool {
		resp, err := http.Get(base + "/api/public/site")
		if err != nil {
			return false
		}
		resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	})

	tok := call[struct {
		Token string `json:"token"`
	}](t, "POST", base+"/api/auth/login", "", map[string]string{"username": "admin", "password": "password123"}).Token
	srv := call[struct {
		ID     string `json:"id"`
		Secret string `json:"secret"`
	}](t, "POST", base+"/api/admin/servers", tok, map[string]any{"name": "e2e"})

	agentCtx, agentCancel := context.WithCancel(ctx)
	defer agentCancel()
	client, err := agent.NewClient(agent.Config{Dashboard: base, ID: srv.ID, Secret: srv.Secret}, collect.New(log), log, "e2e")
	if err != nil {
		t.Fatal(err)
	}
	agentDone := make(chan error, 1)
	go func() { agentDone <- client.Run(agentCtx) }()

	waitUntil(t, 15*time.Second, func() bool {
		list := call[[]map[string]any](t, "GET", base+"/api/public/servers", "", nil)
		return len(list) == 1 && list[0]["online"] == true && list[0]["metrics"] != nil && list[0]["static"] != nil
	})
	pts := call[[]map[string]any](t, "GET", base+"/api/public/servers/"+srv.ID+"/metrics?range=realtime", "", nil)
	if len(pts) == 0 {
		t.Fatal("实时历史应至少有 1 个点")
	}

	wsCtx, wsCancel := context.WithTimeout(ctx, 5*time.Second)
	defer wsCancel()
	ws, _, err := websocket.Dial(wsCtx, "ws://"+ln.Addr().String()+"/api/public/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, b, err := ws.Read(wsCtx)
	if err != nil || !bytes.Contains(b, []byte(`"type":"snapshot"`)) {
		t.Fatalf("浏览器首条消息应为快照: %s %v", b, err)
	}
	ws.CloseNow()

	call[map[string]any](t, "DELETE", base+"/api/admin/servers/"+srv.ID, tok, nil)
	select {
	case err := <-agentDone:
		if !errors.Is(err, agent.ErrStopped) {
			t.Fatalf("删除服务器后 Agent 应收到停止指令，实际 %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Agent 未在 10 秒内退出")
	}
}
```

- [ ] **Step 2: 运行端到端测试**

Run: `go test ./internal/dashboard/ -run TestEndToEnd -v -timeout 120s`
Expected: PASS。若失败，按报错定位到对应任务的代码修复后重跑。

- [ ] **Step 3: 更新 lint 配置中的旧依赖引用**

`.golangci.yml` 中把

```yaml
      - (*github.com/gorilla/websocket.Conn).Close
```

替换为

```yaml
      - (*github.com/coder/websocket.Conn).Close
```

- [ ] **Step 4: 全量检查**

```bash
go mod tidy
git diff --exit-code go.mod go.sum && echo "go.mod 已整洁"
grep -E 'viper|go.uber.org/zap|melody|gorilla|gorm|glebarez' go.mod || echo "无禁用依赖"
grep -E '^go ' go.mod
gofmt -l ./cmd ./internal
go vet ./...
go test ./... -count=1
golangci-lint run --timeout=5m ./... || echo "本机未安装 golangci-lint 时跳过，交由 CI 检查"
go build -o bin/sss-agent$(go env GOEXE) ./cmd/sss-agent
go build -o bin/sss-dashboard$(go env GOEXE) ./cmd/sss-dashboard
```

Expected:
- `go.mod 已整洁`、`无禁用依赖`、`go 1.24.0`
- `gofmt -l` 无输出
- vet、test 全部通过；golangci-lint 无问题（或未安装）
- 两个二进制构建成功

若 golangci-lint 报告问题（常见为 gosec 的 G104/G115），按提示修正代码而不是关闭规则，然后重新运行本步骤。

- [ ] **Step 5: 提交**

```bash
git add internal/dashboard/e2e_test.go .golangci.yml go.mod go.sum
git commit -m "test: 新增 Agent 与 Dashboard 端到端冒烟测试"
```
