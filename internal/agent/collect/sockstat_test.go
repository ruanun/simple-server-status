package collect

import "testing"

func TestParseSockstat(t *testing.T) {
	v4 := `sockets: used 234
TCP: inuse 5 orphan 0 tw 3 alloc 7 mem 1
UDP: inuse 3 mem 2
UDPLITE: inuse 9
RAW: inuse 0
FRAG: inuse 0 memory 0
`
	v6 := `TCP6: inuse 2
UDP6: inuse 1
UDPLITE6: inuse 4
RAW6: inuse 0
FRAG6: inuse 0 memory 0
`
	if tcp, udp := parseSockstat(v4); tcp != 5 || udp != 3 {
		t.Fatalf("sockstat = %d %d，期望 5 3", tcp, udp)
	}
	if tcp, udp := parseSockstat(v6); tcp != 2 || udp != 1 {
		t.Fatalf("sockstat6 = %d %d，期望 2 1", tcp, udp)
	}
	if tcp, udp := parseSockstat(v4 + v6); tcp != 7 || udp != 4 {
		t.Fatalf("合并 = %d %d，期望 7 4", tcp, udp)
	}
	if tcp, udp := parseSockstat("garbage\nTCP: inuse x\n"); tcp != 0 || udp != 0 {
		t.Fatalf("异常内容应为 0: %d %d", tcp, udp)
	}
}
