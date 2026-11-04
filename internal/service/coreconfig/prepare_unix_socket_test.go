package coreconfig

import "testing"

// Unix socket 入站不需要 port：xray 会把 PortList 置 nil（见 infra/conf/xray.go），
// 运行配置里带 port 只是噪音，容易让人误以为在监听 TCP 端口。
func TestCleanInboundFieldsDropsPortForUnixSocket(t *testing.T) {
	cases := []struct {
		listen      string
		wantPort    bool
		wantPortVal float64
	}{
		{"/dev/shm/trojan.sock,0666", false, 0},
		{"/dev/shm/VLESS_TCP.sock", false, 0},
		{"unix:/dev/shm/x.sock", false, 0},
		{"@vless-grpc", false, 0},
		{"127.0.0.1", true, 20001},
		{"0.0.0.0", true, 20001},
		{"", true, 20001},
	}
	for _, c := range cases {
		inbound := map[string]interface{}{"tag": "t", "port": float64(9999)}
		CleanInboundFields(inbound, "t", c.listen, "vless", 20001, "xray")
		v, ok := inbound["port"]
		if ok != c.wantPort {
			t.Fatalf("listen=%q: port present=%v, want %v (inbound=%v)", c.listen, ok, c.wantPort, inbound)
		}
		if c.wantPort {
			var got int
			switch v := v.(type) {
			case int:
				got = v
			case float64:
				got = int(v)
			default:
				t.Fatalf("listen=%q: unexpected port type %T", c.listen, v)
			}
			if got != int(c.wantPortVal) {
				t.Fatalf("listen=%q: port=%v, want %v", c.listen, got, c.wantPortVal)
			}
		}
	}
}

// 内部字段 _listenType（面板回显用）不能出现在运行配置里。
func TestCleanInboundFieldsStripsListenType(t *testing.T) {
	inbound := map[string]interface{}{"tag": "t", "_listenType": "sock", "listen": "/dev/shm/x.sock"}
	CleanInboundFields(inbound, "t", "/dev/shm/x.sock", "vless", 0, "xray")
	if _, ok := inbound["_listenType"]; ok {
		t.Fatalf("_listenType 应被剥离：%v", inbound)
	}
}
