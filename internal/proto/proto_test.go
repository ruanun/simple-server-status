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
