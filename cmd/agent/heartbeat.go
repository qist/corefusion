package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"

	"corefusion/internal/constants"
	"corefusion/internal/coredata"
	agentpb "corefusion/internal/runtime/agent/proto"
)

// RunHeartbeatLoop connects to the Panel gRPC server and maintains a persistent
// heartbeat stream. It auto-reconnects on failure with a 5-second backoff.
func (a *AgentApp) RunHeartbeatLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := a.connectAndStream(ctx); err != nil {
			log.Printf("[Agent] panel connection error (%s): %v", a.panelGRPCAddress(), err)
			// Older configs only have panel.host (possibly a CDN domain) and
			// no grpc_host. Try to discover the origin through the public HTTP
			// endpoint so those Agents recover without a reinstall.
			a.discoverPanelOrigin(ctx)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}

// panelGRPCAddress returns the panel address used for the mTLS heartbeat.
// panel.host is kept for the HTTP(S) bootstrap/download path (it may be a CDN
// domain), while panel.grpc_host is the address that can actually serve the
// gRPC port (normally the origin IP). Older configs without grpc_host fall
// back to panel.host.
func (a *AgentApp) panelGRPCAddress() string {
	addrs := a.panelGRPCAddresses()
	if len(addrs) > 0 {
		return addrs[0]
	}
	return net.JoinHostPort(a.cfg.Panel.Host, strconv.Itoa(a.cfg.Panel.GRPCPort))
}

// panelGRPCAddresses returns the addresses to try for the mTLS heartbeat, in
// priority order: a runtime-discovered origin, grpc_host, then panel.host.
// The last entry is important: if a discovered origin is temporarily
// unreachable, a CDN that does proxy the gRPC port can still be used; the
// panel's CA verification makes trying both safe.
func (a *AgentApp) panelGRPCAddresses() []string {
	a.grpcHostMu.RLock()
	override := strings.TrimSpace(a.grpcHostOverride)
	a.grpcHostMu.RUnlock()

	hosts := []string{override, strings.TrimSpace(a.cfg.Panel.GRPCHost), strings.TrimSpace(a.cfg.Panel.Host)}
	seen := make(map[string]struct{}, len(hosts))
	addrs := make([]string, 0, len(hosts))
	for _, host := range hosts {
		host = strings.Trim(host, "[]")
		if host == "" {
			continue
		}
		addr := net.JoinHostPort(host, strconv.Itoa(a.cfg.Panel.GRPCPort))
		if _, ok := seen[addr]; ok {
			continue
		}
		seen[addr] = struct{}{}
		addrs = append(addrs, addr)
	}
	return addrs
}

func (a *AgentApp) setGRPCHostOverride(host string) {
	a.grpcHostMu.Lock()
	a.grpcHostOverride = host
	a.grpcHostMu.Unlock()
}

const panelOriginDiscoveryInterval = 2 * time.Minute

// discoverPanelOrigin asks the panel's public HTTP endpoint for its gRPC
// origin. It is used only when the existing mTLS connection fails, so old
// installations (which have just panel.host = CDN domain) can recover without
// editing config.yaml. The discovered address is still verified by mTLS
// against the panel CA, so a forged HTTP response cannot impersonate the panel.
func (a *AgentApp) discoverPanelOrigin(ctx context.Context) {
	// An explicitly configured hostname (e.g. a gRPC load balancer) is
	// authoritative and must not be replaced by the panel's egress IP. Only
	// old configs without grpc_host, or configs using an IP, are discovered.
	if explicit := strings.Trim(strings.TrimSpace(a.cfg.Panel.GRPCHost), "[]"); explicit != "" && net.ParseIP(explicit) == nil {
		return
	}
	a.discoveryMu.Lock()
	if !a.lastDiscovery.IsZero() && time.Since(a.lastDiscovery) < panelOriginDiscoveryInterval {
		a.discoveryMu.Unlock()
		return
	}
	a.lastDiscovery = time.Now()
	a.discoveryMu.Unlock()

	url := a.panelHTTPBaseURL() + "/api/v1/agent/origin?grpc_port=" + strconv.Itoa(a.cfg.Panel.GRPCPort)
	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		log.Printf("[Agent] origin discovery request failed: %v", err)
		return
	}
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		log.Printf("[Agent] origin discovery request failed: %v", err)
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != http.StatusOK {
		log.Printf("[Agent] origin discovery failed (HTTP %d): %v %s", resp.StatusCode, err, strings.TrimSpace(string(body)))
		return
	}

	var apiResp struct {
		Code string `json:"code"`
		Data struct {
			GRPCHost string `json:"grpc_host"`
		} `json:"data"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		log.Printf("[Agent] origin discovery returned invalid JSON: %v", err)
		return
	}
	if apiResp.Code != "" && apiResp.Code != "OK" {
		log.Printf("[Agent] origin discovery rejected: %s - %s", apiResp.Code, apiResp.Message)
		return
	}

	host := strings.Trim(strings.TrimSpace(apiResp.Data.GRPCHost), "[]")
	if ip := net.ParseIP(host); ip != nil {
		host = ip.String()
	} else {
		log.Printf("[Agent] origin discovery returned invalid host %q", apiResp.Data.GRPCHost)
		return
	}
	a.grpcHostMu.RLock()
	current := strings.TrimSpace(a.grpcHostOverride)
	if current == "" {
		current = strings.TrimSpace(a.cfg.Panel.GRPCHost)
	}
	a.grpcHostMu.RUnlock()
	if host == current {
		return
	}
	a.setGRPCHostOverride(host)
	log.Printf("[Agent] discovered panel gRPC origin %s (was %q)", host, current)
}

// panelHTTPBaseURL returns the HTTP(S) base URL used for relative downloads
// (Agent self-update, geo rule files). Prefer the public CDN endpoint when it
// is configured: the panel's direct HTTP port may be firewalled from nodes,
// while the CDN is always reachable at the same address that served the
// install script.
func (a *AgentApp) panelHTTPBaseURL() string {
	host := a.cfg.Panel.Host
	port := a.cfg.Panel.Port
	useTLS := a.cfg.Panel.TLS
	if publicHost := strings.TrimSpace(a.cfg.Panel.PublicHost); publicHost != "" {
		host = publicHost
		port = a.cfg.Panel.PublicPort
		if port <= 0 {
			port = a.cfg.Panel.Port
		}
		useTLS = a.cfg.Panel.PublicTLS
	}
	scheme := "http"
	if useTLS {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s", scheme, net.JoinHostPort(host, strconv.Itoa(port)))
}

// dialPanel establishes a gRPC connection to the Panel using mTLS.
func (a *AgentApp) dialPanel(ctx context.Context) (*grpc.ClientConn, error) {
	cert, err := tls.LoadX509KeyPair(a.certPath, a.keyPath)
	if err != nil {
		return nil, fmt.Errorf("load cert: %w", err)
	}

	caPEM, err := os.ReadFile(a.caPath)
	if err != nil {
		return nil, fmt.Errorf("read CA: %w", err)
	}
	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("failed to parse CA cert")
	}

	tlsConfig := buildPanelTLSConfig(cert, caPool)

	addr := a.panelGRPCAddress()
	// grpc.NewClient（非阻塞，Dial 的推荐替代）。创建时不拨号，需显式等待 Ready。
	// 启用 keepalive：每 10 秒发送一次 PING，5 秒内无 PONG 响应则判定连接死亡，
	// 避免 TCP 静默断开时长时间等待 OS 级 TCP 超时（默认可达 60+ 秒）。
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second,
			Timeout:             5 * time.Second,
			PermitWithoutStream: true,
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("create panel client: %w", err)
	}
	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := coredata.WaitForReady(dialCtx, conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("dial panel: %w", err)
	}
	return conn, nil
}

// panelCertServerName 是面板 gRPC 服务端证书的名字：`IssueServerCertificate`
// （internal/runtime/agent/ca.go）把它同时写进 CommonName 与 DNSNames。
// 面板证书在**内存里签发、不落盘**，而 Agent 连的地址通常是 IP 或任意主机名 →
// 必须显式指定 ServerName，否则标准校验无法通过。
const panelCertServerName = "corefusion-panel"

// buildPanelTLSConfig 组装「Agent → 面板」的 mTLS 客户端配置。
//
// ⚠ 这里**绝不能**用 `InsecureSkipVerify: true` ✗：Agent 不校验对端身份时，任何能 MITM 的人
// （DNS 劫持 / ARP 欺骗 / 内网中间人）都能用一张自签证书冒充面板，而面板可以指挥 Agent
// 以 **root** 写任意路径、执行命令——
// 相当于把整条控制面交出去。`RootCAs` 本来就已由 bootstrap 落盘的 CA 建好，
// 这里改成正常校验 + 显式 ServerName 即可，不需要任何"开发用"放松。
func buildPanelTLSConfig(cert tls.Certificate, caPool *x509.CertPool) *tls.Config {
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caPool,
		MinVersion:   tls.VersionTLS13,
		ServerName:   panelCertServerName,
	}
}

// connectAndStream opens a heartbeat stream with the Panel and runs the
// send/receive loop until the stream breaks or the context is cancelled.
//
// Dead-connection detection:
//
// gRPC keepalive PING/PONG handles TCP-level silent drops, but a "half-open"
// state can still occur: the panel's Stream() handler returns (e.g. due to a
// send error on its side) and calls markOffline, while the Agent's stream.Send
// succeeds (data goes into the local TCP buffer) and stream.Recv blocks forever.
//
// To detect this, we track lastRecvTime — the last time we received ANY message
// from the panel (PanelCommand). If no message arrives within recvTimeout
// (90s), we proactively close the stream and reconnect.
func (a *AgentApp) connectAndStream(ctx context.Context) error {
	conn, err := a.dialPanel(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	hbClient := agentpb.NewHeartbeatServiceClient(conn)
	stream, err := hbClient.Stream(ctx)
	if err != nil {
		return fmt.Errorf("open stream: %w", err)
	}

	// Send first heartbeat
	if err := a.sendHeartbeat(stream); err != nil {
		return fmt.Errorf("first heartbeat: %w", err)
	}
	log.Printf("[Agent] connected to panel, heartbeat stream active")

	heartbeatTicker := time.NewTicker(15 * time.Second)
	defer heartbeatTicker.Stop()
	statusTicker := time.NewTicker(30 * time.Second)
	defer statusTicker.Stop()
	trafficTicker := time.NewTicker(30 * time.Second)
	defer trafficTicker.Stop()
	onlineTicker := time.NewTicker(15 * time.Second)
	defer onlineTicker.Stop()

	// Dead-connection watchdog: if no message from panel in 90s, force reconnect.
	const recvTimeout = 90 * time.Second
	a.lastRecvMu.Lock()
	a.lastRecvTime = time.Now()
	a.lastRecvMu.Unlock()
	watchdogTicker := time.NewTicker(15 * time.Second)

	// ── 节点自续期 ✓（面板只管触发 ✓、证书由节点持有并由节点自己续 ✓）──────────
	// ⚠ 决不能在下面的 select 里同步跑 ✗：一次 ACME 签发常要 1~2 分钟 ✓，
	// 会把这期间的心跳饿死 ✗ → 面板把节点判成离线 ✗。
	// 所以独立 goroutine 干活 ✓，结果通过 channel 回到这里发 ✓（沿用 stream ✓）。
	certRenewReports := make(chan *agentpb.PublicCertIssuedReport, 8)
	go a.runCertRenewLoop(ctx, certRenewReports)
	defer watchdogTicker.Stop()

	// Receive loop (Panel commands via stream)
	recvErr := make(chan error, 1)
	go func() {
		for {
			cmd, err := stream.Recv()
			if err != nil {
				recvErr <- err
				return
			}
			// Update last-received time — any message from panel means the
			// connection is alive (including keepalive PONG frames which
			// surface as empty messages at the HTTP/2 level).
			a.lastRecvMu.Lock()
			a.lastRecvTime = time.Now()
			a.lastRecvMu.Unlock()
			a.handlePanelCommand(cmd, stream)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-recvErr:
			return err
		case rep := <-certRenewReports:
			// 自续期结果上报 ✓：面板据此刷新**元数据**（到期时间/签发者 ✓），
			// 不再把 PEM/私钥落到面板 ✗（面板不持有节点证书 ✓）。
			a.streamSendMu.Lock()
			serr := stream.Send(a.publicCertReport(rep))
			a.streamSendMu.Unlock()
			if serr != nil {
				return fmt.Errorf("cert renew report: %w", serr)
			}
		case <-watchdogTicker.C:
			// Check if we haven't heard from the panel in too long.
			a.lastRecvMu.RLock()
			lr := a.lastRecvTime
			a.lastRecvMu.RUnlock()
			if time.Since(lr) > recvTimeout {
				return fmt.Errorf("no panel messages received in %v, forcing reconnect", recvTimeout)
			}
			// Also check the gRPC transport state — if it's not READY/IDLE,
			// the connection is broken but stream.Send might not have noticed.
			st := conn.GetState()
			if st == connectivity.TransientFailure || st == connectivity.Shutdown {
				return fmt.Errorf("gRPC connection state %s, forcing reconnect", st.String())
			}
		case <-heartbeatTicker.C:
			if err := a.sendHeartbeat(stream); err != nil {
				return fmt.Errorf("heartbeat: %w", err)
			}
		case <-statusTicker.C:
			if err := a.sendStatusReport(stream); err != nil {
				return fmt.Errorf("status report: %w", err)
			}
		case <-trafficTicker.C:
			// Traffic report failures should not break the heartbeat stream.
			// Errors are logged inside sendTrafficReport.
			if err := a.sendTrafficReport(stream); err != nil {
				log.Printf("[Agent] traffic report error: %v", err)
			}
		case <-onlineTicker.C:
			// Online-users report failures should not break the heartbeat stream.
			if err := a.sendOnlineUsersReport(stream); err != nil {
				log.Printf("[Agent] online users report error: %v", err)
			}
		}
	}
}

// sendHeartbeat sends a periodic heartbeat report with comprehensive system metrics.
func (a *AgentApp) sendHeartbeat(stream agentpb.HeartbeatService_StreamClient) error {
	m := getSystemMetrics()
	a.streamSendMu.Lock()
	defer a.streamSendMu.Unlock()
	return stream.Send(&agentpb.AgentReport{
		Type:         agentpb.AgentReport_HEARTBEAT,
		ServerId:     a.cfg.Panel.ServerID,
		Timestamp:    time.Now().Unix(),
		AgentVersion: constants.Version,

		// CPU
		CpuUsage: m.CPUUsage,
		CpuCores: int32(m.CPUCores),

		// Memory
		MemTotalBytes: m.MemTotalBytes,
		MemUsageBytes: m.MemUsedBytes,
		MemUsage:      m.MemUsage,

		// Swap
		SwapTotalBytes: m.SwapTotalBytes,
		SwapUsedBytes:  m.SwapUsedBytes,
		SwapUsage:      m.SwapUsage,

		// Disk
		DiskTotalBytes: m.DiskTotalBytes,
		DiskUsageBytes: m.DiskUsedBytes,
		DiskUsage:      m.DiskUsage,

		// Disk I/O rate
		DiskReadBytesSec:  m.DiskReadBytesSec,
		DiskWriteBytesSec: m.DiskWriteBytesSec,

		// Network counters
		NetRxBytes: m.NetRxBytesTotal,
		NetTxBytes: m.NetTxBytesTotal,

		// Network rate
		NetRxBytesSec: m.NetRxBytesSec,
		NetTxBytesSec: m.NetTxBytesSec,

		// System
		Os:             runtime.GOOS,
		Arch:           runtime.GOARCH,
		Distro:         detectDistro(),
		Hostname:       m.Hostname,
		UptimeSeconds:  m.UptimeSeconds,
		Load_1M:        m.Load1m,
		TcpConnections: int32(m.TCPConnections),

		// Egress interface traffic (出口网卡累计流量，用于月度配额)
		EgressInterface: m.EgressInterface,
		EgressRxBytes:   m.EgressRxBytes,
		EgressTxBytes:   m.EgressTxBytes,
	})
}

// detectDistro reads /etc/os-release to determine the Linux distribution.
// Returns a normalized lower-case distro identifier (e.g. "ubuntu", "debian",
// "redhat", "alpine"). On non-Linux systems or when detection fails, returns "".
func detectDistro() string {
	if runtime.GOOS != "linux" {
		return ""
	}
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "ID=") {
			id := strings.Trim(strings.TrimPrefix(line, "ID="), `"'`)
			id = strings.ToLower(strings.TrimSpace(id))
			// Normalize common variants
			switch id {
			case "rhel", "centos", "rocky", "rockylinux", "almalinux", "fedora":
				return "redhat"
			case "ubuntu", "debian", "alpine", "opensuse", "opensuse-leap", "sles", "arch":
				return id
			default:
				if id != "" {
					return id
				}
			}
		}
	}
	return ""
}

// sendStatusReport collects core process statuses and sends them as a STATUS report.
func (a *AgentApp) sendStatusReport(stream agentpb.HeartbeatService_StreamClient) error {
	// Get core states from LocalAgent → process.Manager.ListAll()
	resp, err := a.localAgent.GetStatus(context.Background())
	if err != nil {
		return err
	}
	a.streamSendMu.Lock()
	defer a.streamSendMu.Unlock()
	return stream.Send(&agentpb.AgentReport{
		Type:      agentpb.AgentReport_STATUS,
		ServerId:  a.cfg.Panel.ServerID,
		Timestamp: time.Now().Unix(),
		Cores:     resp.Cores,
	})
}
