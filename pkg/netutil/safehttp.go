package netutil

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"syscall"
	"time"
)

// ErrBlockedAddress 表示目标地址指向内网/环回/保留地址，已按 SSRF 防护拒绝。
var ErrBlockedAddress = errors.New("blocked address")

// DefaultSafeFetchMaxBytes 是安全抓取的默认响应体上限（1 MiB）。
const DefaultSafeFetchMaxBytes = 1 << 20

// blockedIP 判断一个 IP 是否**不允许**作为服务端出站目标。
// 覆盖：环回、未指定、多播、链路本地、以及私有/保留网段（含云元数据 169.254.169.254）。
func blockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsUnspecified() || ip.IsMulticast() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast() {
		return true
	}
	s := ip.String()
	return IsPrivateIP(s) || IsReservedIP(s)
}

// newSafeDialControl 返回 net.Dialer.Control —— 它在下层**已解析出 IP 之后**才被调用，
// 因此每次连接都会对**真实目标 IP**校验，天然规避"先校验域名、再请求"的 TOCTOU /
// DNS 重绑定（同一个域名两次解析结果不同）★这是比"请求前 LookupIP 预检"更可靠的写法。
func newSafeDialControl(allowPrivate bool) func(network, address string, c syscall.RawConn) error {
	return func(network, address string, _ syscall.RawConn) error {
		if allowPrivate {
			return nil
		}
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return fmt.Errorf("%w: malformed address %q", ErrBlockedAddress, address)
		}
		ip := net.ParseIP(host)
		if ip == nil {
			return fmt.Errorf("%w: not an IP %q", ErrBlockedAddress, host)
		}
		if blockedIP(ip) {
			return fmt.Errorf("%w: %s is loopback/private/reserved", ErrBlockedAddress, host)
		}
		return nil
	}
}

// NewSafeHTTPClient 构造面向**用户可控 URL** 的 HTTP 客户端：
//   - 超时可控（默认调用方传）；
//   - **不跟随重定向**（`ErrUseLastResponse`：返回 3xx 本身，由调用方决定是否接受）——
//     否则"公网域名 302 到 169.254.169.254"就能绕过一切前置校验；
//   - 拨号层拒绝环回/内网/保留地址（见 newSafeDialControl），除非 allowPrivate=true（仅供测试）。
//
// 需要访问本机/内网服务的内部调用（面板自身的 gRPC 转 HTTP 等）不要用这个客户端。
func NewSafeHTTPClient(timeout time.Duration, allowPrivate bool) *http.Client {
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
		Control:   newSafeDialControl(allowPrivate),
	}
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext:           dialer.DialContext,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
			ForceAttemptHTTP2:     true,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// ReadLimited 读取 r 的全部内容，但**最多 maxBytes**，超出即报错。
//
// ⚠ 不要用 `io.LimitReader` 静默截断 ✗ —— 那会让"下载到一半的二进制/JSON"被当成完整内容使用
// （二进制校验和会算错、JSON 解析会给出误导性错误）。这里明确报错。
func ReadLimited(r io.Reader, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		return nil, fmt.Errorf("ReadLimited: maxBytes must be > 0")
	}
	data, err := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("response exceeds %d bytes", maxBytes)
	}
	return data, nil
}

// CopyLimited 把 src 复制到 dst，最多 maxBytes，超出即报错（用于"落盘 + 期望知道实际大小"的场景）。
func CopyLimited(dst io.Writer, src io.Reader, maxBytes int64) (int64, error) {
	if maxBytes <= 0 {
		return 0, fmt.Errorf("CopyLimited: maxBytes must be > 0")
	}
	n, err := io.Copy(dst, io.LimitReader(src, maxBytes+1))
	if err != nil {
		return n, err
	}
	if n > maxBytes {
		return n, fmt.Errorf("response exceeds %d bytes", maxBytes)
	}
	return n, nil
}

// SafeFetchGET 抓取一个用户可控的 URL：限制协议、禁止跳转、拒绝内网地址、限制响应体大小。
//
// 返回的 error 在"地址被拒"时可用 errors.Is(err, ErrBlockedAddress) 判定。
func SafeFetchGET(ctx context.Context, rawURL string, timeout time.Duration, maxBytes int64) ([]byte, error) {
	return safeFetchGETWithClient(ctx, NewSafeHTTPClient(timeout, false), rawURL, maxBytes)
}

func safeFetchGETWithClient(ctx context.Context, client *http.Client, rawURL string, maxBytes int64) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("unsupported url scheme %q (only http/https)", u.Scheme)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("invalid url: empty host")
	}
	if maxBytes <= 0 {
		maxBytes = DefaultSafeFetchMaxBytes
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		// 客户端配了 CheckRedirect=ErrUseLastResponse，重定向不会被跟随；
		// 这里直接拒绝，避免调用方把 3xx 的响应体当成内容解析。
		return nil, fmt.Errorf("redirect not followed (status %d, location %q)", resp.StatusCode, resp.Header.Get("Location"))
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	return ReadLimited(resp.Body, maxBytes)
}

// 各类响应的大小上限。取值都留了充足余量（远大于正常情况），只为拦住异常/恶意响应把内存或磁盘撑爆。
const (
	// MaxJSONResponseBytes：GitHub API 之类的 JSON 响应上限。
	MaxJSONResponseBytes int64 = 8 << 20 // 8 MiB
	// MaxChecksumBytes：`.sha256` 校验和文件上限。
	MaxChecksumBytes int64 = 64 << 10 // 64 KiB
	// MaxBinaryBytes：核心/Agent 二进制上限（整块读进内存做格式校验时使用）。
	MaxBinaryBytes int64 = 1 << 30 // 1 GiB
	// MaxGeoRuleBytes：geo 规则文件落盘上限。
	MaxGeoRuleBytes int64 = 1 << 30 // 1 GiB
)
