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
