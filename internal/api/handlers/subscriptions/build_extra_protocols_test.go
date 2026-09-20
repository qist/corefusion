package subscriptions

import "testing"

// 部分 sing-box 协议把参数放在**入站顶层**（不走 transport/tls），buildExtra 必须按
// 协议透传：少了它会丢可选参数，或让客户端出站缺必填字段（snell 的 version）。
func TestBuildExtraProtocolSpecificFields(t *testing.T) {
	ss := map[string]interface{}{}

	t.Run("snell version", func(t *testing.T) {
		extra := buildExtra(map[string]interface{}{"version": float64(4)}, ss, "snell")
		if extra["version"] != 4 {
			t.Fatalf("snell version 未透传: %v", extra["version"])
		}
	})

	t.Run("tuic", func(t *testing.T) {
		extra := buildExtra(map[string]interface{}{
			"congestion_control": "bbr",
			"udp_relay_mode":     "native",
			"zero_rtt_handshake": true,
		}, ss, "tuic")
		for k, want := range map[string]interface{}{
			"congestion-controller": "bbr",
			"udp-relay-mode":        "native",
			"zero-rtt-handshake":    true,
		} {
			if extra[k] != want {
				t.Errorf("tuic extra[%s] = %v, want %v", k, extra[k], want)
			}
		}
	})

	t.Run("anytls", func(t *testing.T) {
		extra := buildExtra(map[string]interface{}{
			"idle_session_check_interval": "30s",
			"idle_session_timeout":        "30s",
			"min_idle_session":            float64(5),
		}, ss, "anytls")
		for k, want := range map[string]interface{}{
			"idle-session-check-interval": "30s",
			"idle-session-timeout":        "30s",
			"min-idle-session":            5,
		} {
			if extra[k] != want {
				t.Errorf("anytls extra[%s] = %v, want %v", k, extra[k], want)
			}
		}
	})
}
