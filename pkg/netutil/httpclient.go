package netutil

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// NewHTTPClient creates an *http.Client whose DNS resolver works around
// systemd-resolved incompatibilities. It reads upstream nameservers directly
// (see resolvConfPaths) and dials them via UDP/TCP, avoiding both the
// LLMNR/mDNS paths that can crash or hang when systemd-resolved is active and
// the 127.0.0.53 stub that is unreachable when resolved is not listening.
func NewHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext:       newDialContext(15 * time.Second),
			DisableKeepAlives: false,
			MaxIdleConns:      10,
			IdleConnTimeout:   90 * time.Second,
		},
	}
}

// GitHubHTTPClient creates an HTTP client that automatically rewrites
// GitHub URLs through an acceleration proxy. Non-GitHub requests are
// unaffected. Pass accelDomain="" to disable acceleration while still
// benefiting from the DNS fix.
func GitHubHTTPClient(timeout time.Duration, accelDomain string) *http.Client {
	return GitHubHTTPClientWithProxy(timeout, accelDomain, "")
}

// GitHubHTTPClientWithProxy is like GitHubHTTPClient but additionally routes
// all requests through the given proxy URL (http://, https://, socks5://).
// An empty proxyURL disables proxying.
func GitHubHTTPClientWithProxy(timeout time.Duration, accelDomain, proxyURL string) *http.Client {
	transport := &http.Transport{
		DialContext:       newDialContext(15 * time.Second),
		DisableKeepAlives: false,
		MaxIdleConns:      10,
		IdleConnTimeout:   90 * time.Second,
	}
	if proxyURL != "" {
		if u, err := url.Parse(proxyURL); err == nil {
			transport.Proxy = http.ProxyURL(u)
		}
	}
	return &http.Client{
		Timeout: timeout,
		Transport: &githubTransport{
			inner:       transport,
			accelDomain: accelDomain,
		},
	}
}

// NewResolvedDialContext returns a DialContext that resolves hostnames through
// the local upstream nameservers (systemd-resolved aware), leaving every other
// transport decision (proxy, TLS, timeouts) to the caller. Use it to patch an
// existing http.Transport, e.g. one built by a third-party library.
func NewResolvedDialContext(timeout time.Duration) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return newResolvedDialer(timeout).DialContext
}

// NewDefaultHTTPClient returns a client that behaves like http.DefaultClient
// (honours HTTP(S)_PROXY/NO_PROXY, HTTP/2, default keep-alive settings) but
// resolves DNS through the local upstream nameservers instead of the system
// resolver. Needed by direct/ACME-style outbound calls on systemd-resolved
// hosts where /etc/resolv.conf only exposes the 127.0.0.53 stub.
func NewDefaultHTTPClient(timeout time.Duration) *http.Client {
	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		// 理论上不会发生（标准库 DefaultTransport 就是 *http.Transport）。
		return &http.Client{Timeout: timeout}
	}
	cloned := transport.Clone()
	cloned.DialContext = NewResolvedDialContext(30 * time.Second)
	return &http.Client{Timeout: timeout, Transport: cloned}
}

// --- private helpers ---

// newDialContext returns a DialContext that resolves DNS using the local
// upstream nameservers directly, bypassing the Go default resolver which can
// conflict with systemd-resolved.
func newDialContext(dialTimeout time.Duration) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return NewResolvedDialContext(dialTimeout)
}

// newResolvedDialer returns a net.Dialer that resolves hostnames through the
// nameservers returned by readNameservers. When none can be found it leaves
// Resolver nil, i.e. keeps the Go/system default behaviour.
//
// Only direct (proxy-less) traffic needs this: with a proxy the target host is
// resolved by the proxy, while every direct request has to go through the
// host resolver — which on systemd-resolved systems may be only the
// 127.0.0.53 stub.
func newResolvedDialer(timeout time.Duration) *net.Dialer {
	dialer := &net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second}
	if nameservers := readNameservers(); len(nameservers) > 0 {
		dialer.Resolver = newNameserverResolver(nameservers)
	}
	return dialer
}

// newNameserverResolver builds a Go resolver that talks to the given
// nameservers in order. network is the "udp"/"tcp" chosen by net.Resolver, so
// its TCP retry path also reaches the configured servers instead of always
// using UDP.
func newNameserverResolver(nameservers []string) *net.Resolver {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	return &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			lastErr := fmt.Errorf("no usable nameserver")
			for _, ns := range nameservers {
				conn, err := dialer.DialContext(ctx, network, ns)
				if err == nil {
					return conn, nil
				}
				lastErr = err
			}
			return nil, lastErr
		},
	}
}

// resolvConfPaths lists the nameserver sources in priority order.
//
// On systemd-resolved systems /etc/resolv.conf usually points at
// stub-resolv.conf and only contains the stub listener 127.0.0.53, while the
// real upstream servers live in /run/systemd/resolve/resolv.conf. Reading the
// upstream file first avoids depending on the stub being up (common failure in
// containers and on minimal installs); fall back to /etc/resolv.conf when it
// is missing or empty.
var resolvConfPaths = []string{
	"/run/systemd/resolve/resolv.conf",
	"/etc/resolv.conf",
}

// readNameservers returns nameserver addresses in "host:port" form (port
// defaults to 53) from the first resolvConfPaths entry that yields any.
func readNameservers() []string {
	for _, path := range resolvConfPaths {
		if servers := parseNameservers(path); len(servers) > 0 {
			return servers
		}
	}
	return nil
}

// parseNameservers parses a single resolv.conf file.
func parseNameservers(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var servers []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "nameserver" {
			servers = append(servers, normalizeNameserver(fields[1]))
		}
	}
	if err := scanner.Err(); err != nil {
		// 读取出错时宁可回退到下一个候选文件，也不要拿半份结果去解析。
		return nil
	}
	return servers
}

// normalizeNameserver turns a resolv.conf nameserver token into host:port.
// net.SplitHostPort distinguishes "already has a port" (8.8.8.8:5353,
// [::1]:53) from a bare address, so IPv6 entries like 2001:4860:4860::8888
// are not mistaken for "already has a port".
func normalizeNameserver(ns string) string {
	if _, _, err := net.SplitHostPort(ns); err == nil {
		return ns
	}
	return net.JoinHostPort(ns, "53")
}

// githubTransport is an http.RoundTripper that rewrites GitHub URLs through
// an acceleration proxy before sending them.
type githubTransport struct {
	inner       *http.Transport
	accelDomain string
}

func (t *githubTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.accelDomain != "" && IsGitHubURL(req.URL.String()) {
		accelURL := AccelerateGitHubURL(req.URL.String(), t.accelDomain)
		newReq := req.Clone(req.Context())
		parsed, err := parseURL(accelURL)
		if err == nil {
			newReq.URL = parsed
			newReq.Host = parsed.Host
		}
		return t.inner.RoundTrip(newReq)
	}
	return t.inner.RoundTrip(req)
}

// parseURL is a thin wrapper around url.Parse that never returns a
// relative-URL error (the stdlib rejects "scheme://host/path" when the
// receiver has no scheme, but we always pass absolute URLs).
func parseURL(raw string) (*url.URL, error) {
	return url.Parse(raw)
}
