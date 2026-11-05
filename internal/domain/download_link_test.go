package domain

import "testing"

func TestDownloadTargetID(t *testing.T) {
	cases := []struct {
		name string
		json string
		want int64
	}{
		{"number", `{"streamSettings":{"xhttpSettings":{"extra":{"_downloadInboundId":7}}}}`, 7},
		{"string", `{"streamSettings":{"xhttpSettings":{"extra":{"_downloadInboundId":"12"}}}}`, 12},
		{"missing", `{"streamSettings":{"xhttpSettings":{"extra":{}}}}`, 0},
		{"bad json", `{`, 0},
		{"no xhttp", `{"streamSettings":{"network":"raw"}}`, 0},
	}
	for _, tc := range cases {
		if got := DownloadTargetID(tc.json); got != tc.want {
			t.Errorf("%s: DownloadTargetID = %d, want %d", tc.name, got, tc.want)
		}
	}
}

func TestIsXHTTPInbound(t *testing.T) {
	if !IsXHTTPInbound(`{"streamSettings":{"network":"xhttp"}}`) {
		t.Error("xhttp inbound not detected")
	}
	if IsXHTTPInbound(`{"streamSettings":{"network":"raw"}}`) {
		t.Error("raw inbound misdetected as xhttp")
	}
	if IsXHTTPInbound(`not json`) {
		t.Error("invalid json should return false")
	}
}

func TestNormalizeClientEmail(t *testing.T) {
	// 面板生成的入站后缀是单段（协议+端口），只切最后一段即可。
	// 若后缀本身含下划线（历史/手工 tag），只能切到最后一段，属已知限制。
	cases := map[string]string{
		"a@x.com_vless20296":   "a@x.com",
		"a_b@x.com_vless20296": "a_b@x.com",
		"a@x.com":              "a@x.com",
		"plain_without_at":     "plain_without_at",
		"a@x.com_trojan_8443":  "a@x.com_trojan",
	}
	for in, want := range cases {
		if got := NormalizeClientEmail(in); got != want {
			t.Errorf("NormalizeClientEmail(%q) = %q, want %q", in, got, want)
		}
	}
}
