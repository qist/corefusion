package domain

import "testing"

func TestNormalizeTransport(t *testing.T) {
	cases := map[string]string{
		"tcp":       "raw",
		"raw":       "raw",
		"kcp":       "mkcp",
		"mkcp":      "mkcp",
		"ws":        "ws",
		"websocket": "ws",
		"xhttp":     "xhttp",
		"splithttp": "xhttp",
		"grpc":      "grpc",
		"MASQUE":    "masque",
		"":          "",
	}
	for in, want := range cases {
		if got := NormalizeTransport(in); got != want {
			t.Fatalf("NormalizeTransport(%q) = %q, want %q", in, got, want)
		}
	}
}

// 新核心只写 method（websocket 等全称），旧代码按 internal 短名继续工作。
func TestStreamTransport(t *testing.T) {
	cases := []struct {
		ss   map[string]interface{}
		want string
	}{
		{map[string]interface{}{"network": "ws"}, "ws"},
		{map[string]interface{}{"method": "websocket"}, "ws"},
		{map[string]interface{}{"method": "websocket", "network": "ws"}, "ws"},
		{map[string]interface{}{"method": "splithttp"}, "xhttp"},
		{map[string]interface{}{"method": "mkcp"}, "mkcp"},
		{map[string]interface{}{"method": "raw"}, "raw"},
		{nil, ""},
	}
	for _, tc := range cases {
		if got := StreamTransport(tc.ss); got != tc.want {
			t.Fatalf("StreamTransport(%v) = %q, want %q", tc.ss, got, tc.want)
		}
	}
}

func TestTransportMethod(t *testing.T) {
	if got := TransportMethod("ws"); got != "websocket" {
		t.Fatalf("ws method = %q", got)
	}
	if got := TransportMethod("websocket"); got != "websocket" {
		t.Fatalf("websocket method = %q", got)
	}
	if got := TransportMethod("tcp"); got != "raw" {
		t.Fatalf("tcp method = %q", got)
	}
	if got := TransportMethod("xhttp"); got != "xhttp" {
		t.Fatalf("xhttp method = %q", got)
	}
}
