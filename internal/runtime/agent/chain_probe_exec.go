package agent

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	agentpb "corefusion/internal/runtime/agent/proto"
)

const (
	chainProbeDefaultTimeout = 15 * time.Second
	chainProbeMaxBody        = 1 << 20
	chainProbeStartupWait    = 8 * time.Second
)

// RunChainProbe 在节点本机跑一个临时核心，完成一轮链路探测。
//
// 配置由面板生成（出站/路由/网卡绑定都已写好），但入站/listener 的端口留 0：
// 由 Agent 分配本机空闲端口并回填，避免面板与节点端口冲突。
func (a *LocalAgent) RunChainProbe(ctx context.Context, req *agentpb.ChainProbeRequest) (*agentpb.ChainProbeResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("empty request")
	}
	dialer := strings.ToLower(strings.TrimSpace(req.GetDialer()))
	switch dialer {
	case "xray", "singbox", "mihomo":
	default:
		return nil, fmt.Errorf("unsupported dialer %q", dialer)
	}
	if strings.TrimSpace(req.GetConfigJson()) == "" {
		return nil, fmt.Errorf("empty config")
	}

	var conf map[string]interface{}
	if err := json.Unmarshal([]byte(req.GetConfigJson()), &conf); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	count := chainProbeListenerCount(conf, dialer)
	if count == 0 {
		return nil, fmt.Errorf("config has no probe listener")
	}
	rawTargets := req.GetRawTargets()
	if len(rawTargets) > 0 && count != 1 {
		return nil, fmt.Errorf("raw probe requires exactly one listener, got %d", count)
	}
	ports, err := uniqueProbePorts(count)
	if err != nil {
		return nil, err
	}
	if err := assignChainProbePorts(conf, dialer, ports); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(conf)
	if err != nil {
		return nil, err
	}

	binPath, err := a.resolveBinaryPath(dialer)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(a.dataDir, "probe", fmt.Sprintf("agent-run-%d", time.Now().UnixNano()))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	cfgPath := filepath.Join(dir, "config.json")
	if err := os.WriteFile(cfgPath, raw, 0o600); err != nil {
		return nil, err
	}

	args := []string{"run", "-c", cfgPath}
	if dialer == "mihomo" {
		args = []string{"-f", cfgPath, "-d", dir}
	}
	cmd := exec.Command(binPath, args...)
	cmd.Dir = dir
	var coreOut strings.Builder
	cmd.Stdout = &coreOut
	cmd.Stderr = &coreOut
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", dialer, err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()

	addrs := make([]string, len(ports))
	for i, p := range ports {
		addrs[i] = net.JoinHostPort("127.0.0.1", strconv.Itoa(p))
	}
	if !waitProbePorts(addrs, chainProbeStartupWait) {
		msg := strings.TrimSpace(coreOut.String())
		if msg == "" {
			msg = "no output"
		}
		return nil, fmt.Errorf("%s failed to open SOCKS ports in %s: %s", dialer, chainProbeStartupWait, truncateProbeErr(msg))
	}

	timeout := time.Duration(req.GetTimeoutMs()) * time.Millisecond
	if timeout <= 0 {
		timeout = chainProbeDefaultTimeout
	}
	if timeout > 30*time.Second {
		timeout = 30 * time.Second
	}
	probeURL := strings.TrimSpace(req.GetProbeUrl())
	if probeURL == "" {
		probeURL = "https://www.cloudflare.com/cdn-cgi/trace"
	}

	concurrency := int(req.GetConcurrency())
	if concurrency < 1 {
		concurrency = 1
	}
	if concurrency > 8 {
		concurrency = 8
	}

	// 优选 IP / 域名只需要 TCP 可达或 TLS 握手，不需要发 HTTP。
	if len(rawTargets) > 0 {
		return runRawChainProbe(ctx, addrs[0], rawTargets, timeout, concurrency), nil
	}

	results := make([]*agentpb.ChainProbeItemResult, len(ports))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for i, socksAddr := range addrs {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, addr string) {
			defer wg.Done()
			defer func() { <-sem }()
			status, up, down, latency, perr := probeThroughSocks(ctx, addr, probeURL, timeout)
			item := &agentpb.ChainProbeItemResult{
				HttpStatus: int32(status),
				UpBytes:    up,
				DownBytes:  down,
				LatencyMs:  int32(latency),
			}
			switch {
			case perr != nil:
				item.Error = truncateProbeErr(perr.Error())
			case status < 200 || status >= 400:
				item.Error = fmt.Sprintf("HTTP %d", status)
			case up <= 0 || down <= 0:
				item.Error = "no payload"
			default:
				item.Success = true
			}
			results[idx] = item
		}(i, socksAddr)
	}
	wg.Wait()

	return &agentpb.ChainProbeResponse{Success: true, Results: results}, nil
}

// chainProbeListenerCount 返回配置里用于探测的本地 SOCKS 入口数量。
func chainProbeListenerCount(conf map[string]interface{}, dialer string) int {
	key := "inbounds"
	if dialer == "mihomo" {
		key = "listeners"
	}
	list, _ := conf[key].([]interface{})
	return len(list)
}

// assignChainProbePorts 把 Agent 分配到的端口写回配置。
func assignChainProbePorts(conf map[string]interface{}, dialer string, ports []int) error {
	key := "inbounds"
	if dialer == "mihomo" {
		key = "listeners"
	}
	list, _ := conf[key].([]interface{})
	if len(list) != len(ports) {
		return fmt.Errorf("listener count mismatch: config=%d ports=%d", len(list), len(ports))
	}
	for i, item := range list {
		m, _ := item.(map[string]interface{})
		if m == nil {
			return fmt.Errorf("listener %d is not an object", i)
		}
		if dialer == "singbox" {
			m["listen_port"] = ports[i]
		} else {
			m["port"] = ports[i]
		}
	}
	return nil
}

func uniqueProbePorts(n int) ([]int, error) {
	seen := make(map[int]struct{}, n)
	out := make([]int, 0, n)
	for len(out) < n {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return nil, err
		}
		port := l.Addr().(*net.TCPAddr).Port
		_ = l.Close()
		if _, ok := seen[port]; ok {
			continue
		}
		seen[port] = struct{}{}
		out = append(out, port)
	}
	return out, nil
}

func waitProbePorts(addrs []string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		ok := true
		for _, addr := range addrs {
			c, err := net.DialTimeout("tcp", addr, 300*time.Millisecond)
			if err != nil {
				ok = false
				break
			}
			_ = c.Close()
		}
		if ok {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

// probeThroughSocks 通过本地 SOCKS5 入口发起一次 HTTP 请求，统计上下行字节。
func probeThroughSocks(ctx context.Context, socksAddr, probeURL string, timeout time.Duration) (status int, up, down int64, latencyMs int, err error) {
	var u, d int64
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			raw, derr := socks5DialContext(ctx, socksAddr, addr, timeout)
			if derr != nil {
				return nil, derr
			}
			return &countingProbeConn{Conn: raw, up: &u, down: &d}, nil
		},
		DisableKeepAlives: true,
		ForceAttemptHTTP2: false,
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12},
	}
	defer transport.CloseIdleConnections()

	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	start := time.Now()
	req, rerr := http.NewRequestWithContext(reqCtx, http.MethodGet, probeURL, nil)
	if rerr != nil {
		return 0, 0, 0, 0, rerr
	}
	req.Header.Set("User-Agent", "CoreFusion-ChainProbe/1.0")
	req.Header.Set("Cache-Control", "no-store")

	client := &http.Client{Transport: transport, Timeout: timeout}
	resp, derr := client.Do(req)
	latencyMs = int(time.Since(start) / time.Millisecond)
	if derr != nil {
		return 0, atomic.LoadInt64(&u), atomic.LoadInt64(&d), latencyMs, derr
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, chainProbeMaxBody))
	return resp.StatusCode, atomic.LoadInt64(&u), atomic.LoadInt64(&d), latencyMs, nil
}

// socks5DialContext 是最小 SOCKS5 CONNECT 客户端（只支持无认证）。
func socks5DialContext(ctx context.Context, socksAddr, target string, timeout time.Duration) (net.Conn, error) {
	d := net.Dialer{Timeout: timeout}
	conn, err := d.DialContext(ctx, "tcp", socksAddr)
	if err != nil {
		return nil, err
	}
	ok := false
	defer func() {
		if !ok {
			_ = conn.Close()
		}
	}()

	_ = conn.SetDeadline(time.Now().Add(timeout))
	if _, err := conn.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		return nil, err
	}
	buf := make([]byte, 2)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return nil, err
	}
	if buf[0] != 0x05 || buf[1] != 0x00 {
		return nil, fmt.Errorf("socks5 auth rejected")
	}

	host, portStr, err := net.SplitHostPort(target)
	if err != nil {
		return nil, err
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, err
	}
	req := []byte{0x05, 0x01, 0x00}
	if ip := net.ParseIP(host); ip != nil {
		if ip4 := ip.To4(); ip4 != nil {
			req = append(req, 0x01)
			req = append(req, ip4...)
		} else {
			req = append(req, 0x04)
			req = append(req, ip.To16()...)
		}
	} else {
		if len(host) > 255 {
			return nil, fmt.Errorf("host too long")
		}
		req = append(req, 0x03, byte(len(host)))
		req = append(req, []byte(host)...)
	}
	req = append(req, byte(port>>8), byte(port))
	if _, err := conn.Write(req); err != nil {
		return nil, err
	}

	head := make([]byte, 4)
	if _, err := io.ReadFull(conn, head); err != nil {
		return nil, err
	}
	if head[0] != 0x05 || head[1] != 0x00 {
		return nil, fmt.Errorf("socks5 connect failed: code %d", head[1])
	}
	switch head[3] {
	case 0x01:
		_, err = io.ReadFull(conn, make([]byte, 6))
	case 0x04:
		_, err = io.ReadFull(conn, make([]byte, 18))
	case 0x03:
		l := make([]byte, 1)
		if _, err = io.ReadFull(conn, l); err == nil {
			_, err = io.ReadFull(conn, make([]byte, int(l[0])+2))
		}
	default:
		err = fmt.Errorf("socks5 unknown address type %d", head[3])
	}
	if err != nil {
		return nil, err
	}
	_ = conn.SetDeadline(time.Time{})
	ok = true
	return conn, nil
}

type countingProbeConn struct {
	net.Conn
	up   *int64
	down *int64
}

func (c *countingProbeConn) Read(b []byte) (int, error) {
	n, err := c.Conn.Read(b)
	if n > 0 {
		atomic.AddInt64(c.down, int64(n))
	}
	return n, err
}

func (c *countingProbeConn) Write(b []byte) (int, error) {
	n, err := c.Conn.Write(b)
	if n > 0 {
		atomic.AddInt64(c.up, int64(n))
	}
	return n, err
}

func truncateProbeErr(s string) string {
	s = strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
	if len(s) > 300 {
		return s[:300] + "..."
	}
	return s
}

// runRawChainProbe 通过本地 SOCKS 入口做 TCP 连接 / TLS 握手，用于优选探测。
// 结果与 raw_targets 一一对应；不做 HTTP 请求，也不统计上下行字节。
func runRawChainProbe(ctx context.Context, socksAddr string, targets []*agentpb.ChainProbeRawTarget, timeout time.Duration, concurrency int) *agentpb.ChainProbeResponse {
	results := make([]*agentpb.ChainProbeItemResult, len(targets))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for i, target := range targets {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, t *agentpb.ChainProbeRawTarget) {
			defer wg.Done()
			defer func() { <-sem }()
			item := &agentpb.ChainProbeItemResult{}
			host := strings.TrimSpace(t.GetHost())
			port := int(t.GetPort())
			if host == "" || port <= 0 || port > 65535 {
				item.Error = "invalid target"
				results[idx] = item
				return
			}
			start := time.Now()
			conn, err := socks5DialContext(ctx, socksAddr, net.JoinHostPort(host, strconv.Itoa(port)), timeout)
			if err == nil {
				if strings.EqualFold(strings.TrimSpace(t.GetMode()), "tls") {
					sni := strings.TrimSpace(t.GetSni())
					if sni == "" {
						sni = host
					}
					tc := tls.Client(conn, &tls.Config{ServerName: sni, InsecureSkipVerify: true, MinVersion: tls.VersionTLS12})
					_ = tc.SetDeadline(time.Now().Add(timeout))
					err = tc.HandshakeContext(ctx)
					_ = tc.Close()
				} else {
					_ = conn.Close()
				}
			}
			item.LatencyMs = int32(time.Since(start) / time.Millisecond)
			if err != nil {
				item.Error = truncateProbeErr(err.Error())
			} else {
				item.Success = true
			}
			results[idx] = item
		}(i, target)
	}
	wg.Wait()
	return &agentpb.ChainProbeResponse{Success: true, Results: results}
}
