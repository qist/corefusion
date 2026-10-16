package netutil

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// publicIPEchoEndpoints 是用于"探测本机公网出口 IP"的第三方回显服务（按优先级尝试）。
// 它们只在响应体里回显"请求方被看到时的源 IP"，不做跳转。
var publicIPEchoEndpoints = []string{
	"https://ip.sb",
	"https://api.ipify.org",
	"https://ifconfig.me/ip",
	"https://icanhazip.com",
}

// DetectPublicIPDirect 在**本机直连**探测公网 IPv4 / IPv6 出口地址。
//
// ⚠ 关键：绝不经任何代理（Proxy=nil ✓）。
// 公网 IP 证书的 SAN、以及"本机对外提供服务时用的地址"，必须是**本机的真实出口 IP** ✓；
// 一旦走管理员配置的出网代理（或环境变量里的 HTTP_PROXY ✓），回显到的会是**代理的出口 IP** ✗ ——
// 写进证书就是一张指向别人的废证书 ✗（真机踩到过：面板配了代理，探测到的却是代理的 IP ✗）。
//
// 地址族用 tcp4 / tcp6 强制锁定 ✓：否则双栈机器上探测 IPv6 会命中 IPv4 出口 ✗。
// 两个地址族都失败时返回 error ✓。
func DetectPublicIPDirect(ctx context.Context, timeout time.Duration) (ipv4, ipv6 string, err error) {
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	ipv4 = probePublicIP(ctx, "tcp4", timeout)
	ipv6 = probePublicIP(ctx, "tcp6", timeout)
	if ipv4 == "" && ipv6 == "" {
		return "", "", fmt.Errorf("failed to detect public IP (both IPv4 and IPv6)")
	}
	return ipv4, ipv6, nil
}

// probePublicIP 用锁定到 network（tcp4 / tcp6 ✓）的直连客户端依次尝试各回显服务，
// 返回第一个**合法公网** IP ✓（内网/保留地址一律视为无效 ✓）。
func probePublicIP(ctx context.Context, network string, timeout time.Duration) string {
	dialer := &net.Dialer{Timeout: timeout}
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			// ⚠ 显式禁用代理 ✗：任何代理都会让我们回显到"代理的出口 IP" ✓→✗
			Proxy: nil,
			DialContext: func(ctx context.Context, _, addr string) (net.Conn, error) {
				return dialer.DialContext(ctx, network, addr)
			},
		},
	}
	for _, endpoint := range publicIPEchoEndpoints {
		if ip := fetchEchoIP(ctx, client, endpoint); ip != "" {
			return ip
		}
	}
	return ""
}

// fetchEchoIP 请求一个回显服务并解析出响应体里的 IP ✓；任何异常都返回空串（由上层尝试下一个 ✓）。
func fetchEchoIP(ctx context.Context, client *http.Client, endpoint string) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "corefusion")
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 128))
	if err != nil {
		return ""
	}
	ip := net.ParseIP(strings.TrimSpace(string(body)))
	if ip == nil {
		return ""
	}
	// 只接受公网地址 ✓：避免把 10.x / 169.254.x 之类的"回声"当成结果 ✗
	if IsPrivateIP(ip.String()) || IsReservedIP(ip.String()) {
		return ""
	}
	return ip.String()
}
