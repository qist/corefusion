package agent

import (
	"os"
	"path/filepath"
	"testing"
)

// 能力探测靠 `nginx -V`（输出在 stderr）。这里用脚本模拟，避免依赖真机安装。
func TestProxyCapabilitiesParsesNginxV(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "nginx")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' 'nginx version: nginx/1.31.9' 'built by gcc' " +
		"'-DNGX_STREAM_PP_VAR' '--with-stream_ssl_preread_module' >&2\n" +
		"exit 0\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	caps := proxyCapabilities("nginx", bin, "")
	got := map[string]bool{}
	for _, c := range caps {
		got[c] = true
	}
	if !got[CapStreamPPVar] {
		t.Fatalf("应识别 NGX_STREAM_PP_VAR，实际 %v", caps)
	}
	if !got[CapStreamSSLPreread] {
		t.Fatalf("应识别 --with-stream_ssl_preread_module，实际 %v", caps)
	}
}

// 没有补丁的官方 nginx：只报 preread，不报 pp_var。
func TestProxyCapabilitiesWithoutPatch(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "nginx")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' 'nginx version: nginx/1.26.0' '--with-stream_ssl_preread_module' >&2\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	for _, c := range proxyCapabilities("nginx", bin, "") {
		if c == CapStreamPPVar {
			t.Fatalf("无补丁的 nginx 不应报 %s", CapStreamPPVar)
		}
	}
}

// 非 nginx 类型 / 空路径直接返回 nil，不执行任何命令。
func TestProxyCapabilitiesSkipsNonNginx(t *testing.T) {
	if caps := proxyCapabilities("caddy", "/bin/true", ""); len(caps) != 0 {
		t.Fatalf("caddy 不应探测能力: %v", caps)
	}
	if caps := proxyCapabilities("nginx", "", ""); len(caps) != 0 {
		t.Fatalf("空路径不应探测: %v", caps)
	}
}
