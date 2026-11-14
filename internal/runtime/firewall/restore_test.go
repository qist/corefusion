package firewall

import (
	"reflect"
	"testing"
)

// 不同核心/客户端格式的动态端口都要能识别出来，供启动恢复时放行端口段。
func TestDynamicPortSpecs(t *testing.T) {
	cases := []struct {
		name   string
		config string
		port   int
		want   []string
	}{
		{
			name:   "面板 _portHop",
			config: `{"_portHop":{"enabled":true,"mode":"preset","range":"20000-29999"},"port":4443}`,
			port:   4443,
			want:   []string{"20000-29999"},
		},
		{
			name:   "面板 _portHop 手动段",
			config: `{"_portHop":{"enabled":true,"mode":"manual","start":30000,"end":39999},"port":4443}`,
			port:   4443,
			want:   []string{"30000-39999"},
		},
		{
			name:   "xray hysteriaSettings.hopPorts 多段",
			config: `{"streamSettings":{"hysteriaSettings":{"hopPorts":"20000-29999,40000-49999"}}}`,
			port:   4443,
			want:   []string{"20000-29999", "40000-49999"},
		},
		{
			name:   "xray udpHop.ports",
			config: `{"streamSettings":{"hysteriaSettings":{"udpHop":{"ports":"20000-29999"}}}}`,
			port:   4443,
			want:   []string{"20000-29999"},
		},
		{
			name:   "finalmask 顶层 udpHop",
			config: `{"finalmask":{"quicParams":{"udpHop":{"ports":"10000-19999"}}}}`,
			port:   4443,
			want:   []string{"10000-19999"},
		},
		{
			name:   "streamSettings.finalmask udpHop",
			config: `{"streamSettings":{"finalmask":{"quicParams":{"udpHop":{"ports":"10000-19999"}}}}}`,
			port:   4443,
			want:   []string{"10000-19999"},
		},
		{
			name:   "sing-box server_ports 数组（冒号写法）",
			config: `{"server_ports":["20000:29999","30000:39999"]}`,
			port:   4443,
			want:   []string{"20000-29999", "30000-39999"},
		},
		{
			name:   "mihomo ports 单段",
			config: `{"ports":"20000-29999"}`,
			port:   4443,
			want:   []string{"20000-29999"},
		},
		{
			name:   "去重 + 忽略非法项",
			config: `{"ports":"20000-29999, 20000-29999, {{RANGE}}","server_ports":["20000:29999"]}`,
			port:   4443,
			want:   []string{"20000-29999"},
		},
		{
			name:   "无动态端口",
			config: `{"port":4443}`,
			port:   4443,
			want:   nil,
		},
		{
			name:   "非法 JSON",
			config: `not-json`,
			port:   4443,
			want:   nil,
		},
	}
	for _, tc := range cases {
		got := DynamicPortSpecs(tc.config, tc.port)
		if !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("%s: DynamicPortSpecs() = %v, want %v", tc.name, got, tc.want)
		}
	}
}
