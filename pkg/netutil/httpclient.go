package netutil

import (
	"bufio"
	"context"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// NewHTTPClient creates an *http.Client whose DNS resolver works around
// systemd-resolved incompatibilities. It reads nameservers from
// /etc/resolv.conf and dials them directly via UDP/TCP, avoiding the
// LLMNR/mDNS paths that can crash or hang when systemd-resolved is active.
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
			inner:        transport,
			accelDomain:  accelDomain,
		},
	}
}

// --- private helpers ---

// newDialContext returns a DialContext that resolves DNS using nameservers
// from /etc/resolv.conf directly, bypassing the Go default resolver which
// can conflict with systemd-resolved.
func newDialContext(dialTimeout time.Duration) func(ctx context.Context, network, addr string) (net.Conn, error) {
	nameservers := readNameservers()
	if len(nameservers) == 0 {
		// Fallback to system defaults
		dialer := &net.Dialer{Timeout: dialTimeout}
		return dialer.DialContext
	}

	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
			dialer := &net.Dialer{Timeout: 5 * time.Second}
			// Try each nameserver in order
			for _, ns := range nameservers {
				conn, err := dialer.DialContext(ctx, "udp", ns)
				if err == nil {
					return conn, nil
				}
			}
			// Last resort: try the first one, let the error propagate
			return dialer.DialContext(ctx, "udp", nameservers[0])
		},
	}

	dialer := &net.Dialer{
		Timeout:   dialTimeout,
		Resolver:  resolver,
		KeepAlive: 30 * time.Second,
	}
	return dialer.DialContext
}

// readNameservers parses /etc/resolv.conf and returns nameserver addresses
// in "host:port" format (port defaults to 53).
func readNameservers() []string {
	f, err := os.Open("/etc/resolv.conf")
	if err != nil {
		return nil
	}
	defer f.Close()

	var servers []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "nameserver" {
			ns := fields[1]
			if !strings.Contains(ns, ":") {
				ns = net.JoinHostPort(ns, "53")
			}
			servers = append(servers, ns)
		}
	}
	return servers
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
