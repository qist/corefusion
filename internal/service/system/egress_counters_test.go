package system

import "testing"

const procNetDevFixture = `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
    lo: 1000 1 0 0 0 0 0 0 2000 1 0 0 0 0 0 0
  eth0: 111 2 0 0 0 0 0 0 222 2 0 0 0 0 0 0
  eth1: 333 3 0 0 0 0 0 0 444 3 0 0 0 0 0 0
`

const procNetRouteFixture = `Iface	Destination	Gateway 	Flags	RefCnt	Use	Metric	Mask		MTU	Window	IRTT
eth0	00000000	0102A8C0	0003	0	0	0	00000000	0	0	0
eth1	0002A8C0	00000000	0001	0	0	0	00FFFFFF	0	0	0
`

// 默认路由网卡：Destination=0、Mask=0、RTF_UP。
func TestParseDefaultRouteInterface(t *testing.T) {
	if got := parseDefaultRouteInterface(procNetRouteFixture); got != "eth0" {
		t.Fatalf("default route interface = %q, want eth0", got)
	}
	if got := parseDefaultRouteInterface(""); got != "" {
		t.Fatalf("no routes should return empty, got %q", got)
	}
}

// 指定网卡取对应的累计 rx/tx；不存在的网卡返回 ok=false。
func TestParseInterfaceCounters(t *testing.T) {
	rx, tx, ok := parseInterfaceCounters(procNetDevFixture, "eth1")
	if !ok || rx != 333 || tx != 444 {
		t.Fatalf("eth1 counters = (%d,%d,%v), want (333,444,true)", rx, tx, ok)
	}
	if _, _, ok := parseInterfaceCounters(procNetDevFixture, "eth9"); ok {
		t.Fatal("unknown interface should not be found")
	}
}

// 回退汇总只算非回环网卡。
func TestSumNonLoopbackCounters(t *testing.T) {
	rx, tx := sumNonLoopbackCounters(procNetDevFixture)
	if rx != 111+333 || tx != 222+444 {
		t.Fatalf("sum = (%d,%d), want (%d,%d)", rx, tx, 111+333, 222+444)
	}
}
