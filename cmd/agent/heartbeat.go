package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"corefusion/internal/constants"
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
			log.Printf("[Agent] panel connection error: %v", err)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}

// dialPanel establishes a gRPC connection to the Panel using mTLS.
func (a *AgentApp) dialPanel() (*grpc.ClientConn, error) {
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

	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		RootCAs:            caPool,
		MinVersion:         tls.VersionTLS13,
		InsecureSkipVerify: true, // Panel cert CN is hostname, relax for development
	}

	addr := fmt.Sprintf("%s:%d", a.cfg.Panel.Host, a.cfg.Panel.GRPCPort)
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	if err != nil {
		return nil, fmt.Errorf("dial panel: %w", err)
	}
	return conn, nil
}

// connectAndStream opens a heartbeat stream with the Panel and runs the
// send/receive loop until the stream breaks or the context is cancelled.
func (a *AgentApp) connectAndStream(ctx context.Context) error {
	conn, err := a.dialPanel()
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

	// Receive loop (Panel commands via stream)
	recvErr := make(chan error, 1)
	go func() {
		for {
			cmd, err := stream.Recv()
			if err != nil {
				recvErr <- err
				return
			}
			a.handlePanelCommand(cmd, stream)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-recvErr:
			return err
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
		}
	}
}

// sendHeartbeat sends a periodic heartbeat report with comprehensive system metrics.
func (a *AgentApp) sendHeartbeat(stream agentpb.HeartbeatService_StreamClient) error {
	m := getSystemMetrics()
	a.streamSendMu.Lock()
	defer a.streamSendMu.Unlock()
	return stream.Send(&agentpb.AgentReport{
		Type:            agentpb.AgentReport_HEARTBEAT,
		ServerId:        a.cfg.Panel.ServerID,
		Timestamp:       time.Now().Unix(),
		AgentVersion:    constants.Version,

		// CPU
		CpuUsage:   m.CPUUsage,
		CpuCores:   int32(m.CPUCores),

		// Memory
		MemTotalBytes:   m.MemTotalBytes,
		MemUsageBytes:   m.MemUsedBytes,
		MemUsage:        m.MemUsage,

		// Swap
		SwapTotalBytes:  m.SwapTotalBytes,
		SwapUsedBytes:   m.SwapUsedBytes,
		SwapUsage:       m.SwapUsage,

		// Disk
		DiskTotalBytes:  m.DiskTotalBytes,
		DiskUsageBytes:  m.DiskUsedBytes,
		DiskUsage:       m.DiskUsage,

		// Disk I/O rate
		DiskReadBytesSec:  m.DiskReadBytesSec,
		DiskWriteBytesSec: m.DiskWriteBytesSec,

		// Network counters
		NetRxBytes:      m.NetRxBytesTotal,
		NetTxBytes:      m.NetTxBytesTotal,

		// Network rate
		NetRxBytesSec:   m.NetRxBytesSec,
		NetTxBytesSec:   m.NetTxBytesSec,

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
		EgressRxBytes:  m.EgressRxBytes,
		EgressTxBytes:  m.EgressTxBytes,
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
