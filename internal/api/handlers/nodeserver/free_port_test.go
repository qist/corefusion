package nodeserver

import (
	"encoding/json"
	"testing"
)

// 入站/端点 JSON 里的端口字段有多种写法（数字、字符串、json.Number、portMap），
// 漏掉任何一种都会让该端口被重复分配。
func TestCollectPortsParsesAllForms(t *testing.T) {
	cases := []struct {
		name string
		json string
		want []int
	}{
		{
			name: "sing-box inbound listen_port（数字）",
			json: `{"type":"vless","tag":"in-1","listen_port":10443}`,
			want: []int{10443},
		},
		{
			name: "xray inbound port（数字）",
			json: `{"tag":"in-2","port":8443}`,
			want: []int{8443},
		},
		{
			name: "端口写成字符串",
			json: `{"tag":"in-3","listen_port":"2096"}`,
			want: []int{2096},
		},
		{
			name: "嵌套 settings.port",
			json: `{"tag":"in-4","settings":{"port":12345,"network":"tcp"}}`,
			want: []int{12345},
		},
		{
			name: "端点 wireguard listen_port",
			json: `{"type":"wireguard","tag":"wg-ep-1","listen_port":51820}`,
			want: []int{51820},
		},
		{
			name: "端点 openvpn-server listen_port（remote_port 是对端，不计入）",
			json: `{"type":"openvpn-server","tag":"ovpn-s-1","listen_port":21194,"remote_port":1194}`,
			want: []int{21194},
		},
		{
			// 这些字段都是「远端/转发目标」端口，若被当成已占用会误伤合法端口
			name: "远端/转发目标端口不计入",
			json: `{"peer_port":1194,"remote_port":1194,"server_port":1194,"override_port":1194,"target_port":1194}`,
			want: nil,
		},
		{
			name: "xray dokodemo-door portMap",
			json: `{"tag":"in-5","settings":{"portMap":{"80":"1.2.3.4:8080","443":"1.2.3.4:8443"}}}`,
			want: []int{80, 443},
		},
		{
			name: "数组（inbounds 段）",
			json: `[{"tag":"a","listen_port":10001},{"tag":"b","port":10002}]`,
			want: []int{10001, 10002},
		},
		{
			name: "非法/越界端口忽略",
			json: `{"listen_port":0,"port":70000,"listen":-1}`,
			want: nil,
		},
		{
			name: "端口范围字符串不当作单端口",
			json: `{"tag":"in-6","port_range":"1000:2000"}`,
			want: nil,
		},
	}

	for _, tc := range cases {
		used := map[int]bool{}
		addPortsFromJSON(used, json.RawMessage(tc.json))
		if len(used) != len(tc.want) {
			t.Errorf("%s: 收集到 %d 个端口 %v, want %v", tc.name, len(used), used, tc.want)
			continue
		}
		for _, p := range tc.want {
			if !used[p] {
				t.Errorf("%s: 缺少端口 %d（收集到 %v）", tc.name, p, used)
			}
		}
	}
}

// 同一端口无论被 TCP 还是 UDP 形态的入站占用，都必须排除（用户诉求：
// 创建入站时不要重复 udp/tcp —— 端口号层面共享，不做协议区分）。
func TestPickFreePortSkipsAnyUsedPort(t *testing.T) {
	// 造一个几乎占满候选区间的 used 集合，只留一个空位
	used := map[int]bool{}
	for p := freePortMin; p <= freePortMax; p++ {
		used[p] = true
	}
	const want = 23456
	delete(used, want)

	got := pickFreePort(used, false)
	if got != want {
		t.Fatalf("pickFreePort = %d, want %d（唯一空位）", got, want)
	}
}

// 多次取端口：返回的端口既不能落在已占用集合里，也必须落在候选区间内。
func TestPickFreePortAvoidsUsedSet(t *testing.T) {
	used := map[int]bool{}
	// 占用前一半区间
	for p := freePortMin; p < freePortMin+(freePortMax-freePortMin)/2; p++ {
		used[p] = true
	}
	for i := 0; i < 200; i++ {
		got := pickFreePort(used, false)
		if got < freePortMin || got > freePortMax {
			t.Fatalf("返回端口越界: %d", got)
		}
		if used[got] {
			t.Fatalf("返回了已占用端口: %d", got)
		}
	}
}

// 保留端口（面板自身、核心 API）不允许被分配。
func TestReservedPortsAreExcluded(t *testing.T) {
	used := map[int]bool{}
	for p := range reservedPorts {
		used[p] = true
	}
	if used[8080] != true || used[10085] != true || used[10086] != true || used[18443] != true {
		t.Fatal("保留端口集合缺少面板/核心关键端口")
	}
}
