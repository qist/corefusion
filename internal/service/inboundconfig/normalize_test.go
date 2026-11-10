package inboundconfig

import (
	"errors"
	"strings"
	"testing"
)

// vless 必须有 decryption（xray 强制要求）；缺了就补，避免整机核心起不来。
func TestNormalizeXrayInboundAddsVlessDecryption(t *testing.T) {
	out, err := normalizeXrayInboundConfig("xray", "vless",
		`{"settings":{"clients":[{"id":"11111111-1111-1111-1111-111111111111"}]}}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, `"decryption":"none"`) {
		t.Fatalf("decryption 未被补上：%s", out)
	}
}

// shadowsocks 的 method/password 残留在 vless 里是真实事故来源：必须拦住。
func TestNormalizeXrayInboundRejectsShadowsocksInVless(t *testing.T) {
	_, err := normalizeXrayInboundConfig("xray", "vless",
		`{"settings":{"method":"2022-blake3-aes-256-gcm","password":"abc","network":"tcp,udp"}}`)
	if !errors.Is(err, ErrProtocolSettingsMismatch) {
		t.Fatalf("expected ErrProtocolSettingsMismatch, got %v", err)
	}
}

func TestNormalizeXrayInboundShadowsocksSingleUserOK(t *testing.T) {
	cfg := `{"settings":{"method":"aes-256-gcm","password":"abc","network":"tcp,udp"}}`
	out, err := normalizeXrayInboundConfig("xray", "shadowsocks", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != cfg {
		t.Fatalf("单用户 shadowsocks 不应被改写：%s", out)
	}
}

func TestNormalizeXrayInboundShadowsocksMissingCredentials(t *testing.T) {
	if _, err := normalizeXrayInboundConfig("xray", "shadowsocks", `{"settings":{}}`); !errors.Is(err, ErrProtocolSettingsMismatch) {
		t.Fatalf("expected ErrProtocolSettingsMismatch, got %v", err)
	}
}

// 其它核心（sing-box）不参与 xray 规则。
func TestNormalizeInboundSkipsOtherCores(t *testing.T) {
	cfg := `{"settings":{"method":"x"}}`
	if out, err := normalizeXrayInboundConfig("sing-box", "vless", cfg); err != nil || out != cfg {
		t.Fatalf("sing-box 不应被校验：out=%s err=%v", out, err)
	}
}

// vless/vmess 的客户端必须有 id；只有 password（trojan 结构）时 xray 会报
// "invalid UUID:" 并让整个核心启动失败。
func TestNormalizeXrayInboundRejectsMissingClientID(t *testing.T) {
	_, err := normalizeXrayInboundConfig("xray", "vless",
		`{"settings":{"clients":[{"password":"p1","email":"a"}],"decryption":"none"}}`)
	if !errors.Is(err, ErrProtocolSettingsMismatch) {
		t.Fatalf("expected ErrProtocolSettingsMismatch, got %v", err)
	}
}

// trojan 客户端必须有 password。
func TestNormalizeXrayInboundRejectsTrojanWithoutPassword(t *testing.T) {
	_, err := normalizeXrayInboundConfig("xray", "trojan",
		`{"settings":{"clients":[{"id":"11111111-1111-1111-1111-111111111111"}]}}`)
	if !errors.Is(err, ErrProtocolSettingsMismatch) {
		t.Fatalf("expected ErrProtocolSettingsMismatch, got %v", err)
	}
}

func TestNormalizeXrayInboundAcceptsValidTrojan(t *testing.T) {
	cfg := `{"settings":{"clients":[{"password":"p1","email":"a"}]}}`
	if out, err := normalizeXrayInboundConfig("xray", "trojan", cfg); err != nil || out != cfg {
		t.Fatalf("valid trojan rejected: out=%s err=%v", out, err)
	}
}

// xver>0 只对「本机 unix socket / 回环地址」的 REALITY dest 有意义；
// dest 是公网站点时会让所有连接（含已认证）被判定 invalid，必须强制归零。
func TestNormalizeXrayInboundForcesXverZeroForPublicDest(t *testing.T) {
	out, err := normalizeXrayInboundConfig("xray", "vless",
		`{"settings":{"clients":[{"id":"11111111-1111-1111-1111-111111111111"}]},`+
			`"streamSettings":{"security":"reality","realitySettings":{"dest":"www.apple.com:443","xver":1}}}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, `"xver":0`) {
		t.Fatalf("公网 dest 的 xver 应被改成 0：%s", out)
	}
}

// 本机 unix socket（面板 nginx.sock）必须保留用户配置的 xver=1。
func TestNormalizeXrayInboundKeepsXverForLocalUnixTarget(t *testing.T) {
	out, err := normalizeXrayInboundConfig("xray", "vless",
		`{"settings":{"clients":[{"id":"11111111-1111-1111-1111-111111111111"}]},`+
			`"streamSettings":{"security":"reality","realitySettings":{"target":"/dev/shm/nginx.sock","xver":1}}}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, `"xver":1`) {
		t.Fatalf("本机 unix target 的 xver 不应被改：%s", out)
	}
}

// 127.0.0.1:<port> 的本地反代同样保留 xver。
func TestNormalizeXrayInboundKeepsXverForLoopbackTarget(t *testing.T) {
	out, err := normalizeXrayInboundConfig("xray", "vless",
		`{"settings":{"clients":[{"id":"11111111-1111-1111-1111-111111111111"}]},`+
			`"streamSettings":{"security":"reality","realitySettings":{"target":"127.0.0.1:8443","xver":2}}}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, `"xver":2`) {
		t.Fatalf("回环 target 的 xver 不应被改：%s", out)
	}
}
