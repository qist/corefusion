package agent

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// proxyCapabilities 探测代理二进制的可选编译能力（目前只对 nginx 有意义）。
//
// 背景：面板打包的 nginx 带 `-DNGX_STREAM_PP_VAR` 补丁，`proxy_protocol`
// 可以取变量（按 SNI 分别给不同后端发 v1/v2/off）。这决定了面板能否在同一
// 443 监听上让 xray 收 PROXY 头、sing-box REALITY 不收。老版本/官方 nginx
// 没有该补丁，面板必须回退「整组 proxy_protocol on 并跳过 sing-box 规则」。
//
// `nginx -V` 把编译参数输出到 **stderr**，所以用 CombinedOutput。
// 探测失败/超时返回 nil：调用方按「不支持」处理，回退旧行为，不影响状态查询。
func proxyCapabilities(proxyType, binPath, dataDir string) []string {
	if proxyType != "nginx" || strings.TrimSpace(binPath) == "" {
		return nil
	}
	path := binPath
	if !filepath.IsAbs(path) && dataDir != "" {
		path = filepath.Join(dataDir, path)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, path, "-V").CombinedOutput()
	if err != nil && len(out) == 0 {
		return nil
	}
	text := string(out)

	caps := make([]string, 0, 2)
	if strings.Contains(text, "NGX_STREAM_PP_VAR") {
		caps = append(caps, CapStreamPPVar)
	}
	if strings.Contains(text, "--with-stream_ssl_preread_module") {
		caps = append(caps, CapStreamSSLPreread)
	}
	return caps
}

// 代理能力标记（与 proto ProxyInfo.capabilities 的取值一致）。
const (
	// CapStreamPPVar：proxy_protocol 支持变量，可按 SNI 分别发送。
	CapStreamPPVar = "stream_pp_var"
	// CapStreamSSLPreread：编译了 stream_ssl_preread_module（SNI 分流前提）。
	CapStreamSSLPreread = "stream_ssl_preread"
)
