package main

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

func TestPanelGRPCAddressesFallbackOrder(t *testing.T) {
	cfg := &AgentConfig{}
	cfg.Panel.Host = "cdn.example.com"
	cfg.Panel.GRPCHost = "203.0.113.10"
	cfg.Panel.GRPCPort = 18443
	app := &AgentApp{cfg: cfg}
	app.setGRPCHostOverride("203.0.113.11")

	got := app.panelGRPCAddresses()
	want := []string{"203.0.113.11:18443", "203.0.113.10:18443", "cdn.example.com:18443"}
	if len(got) != len(want) {
		t.Fatalf("addresses = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("addresses = %v, want %v", got, want)
		}
	}

	// 上次握手成功的地址必须排在最前：坏掉的"发现源站"不能再拖慢/打死重连。
	app.setLastGoodGRPCAddr("cdn.example.com:18443")
	got = app.panelGRPCAddresses()
	if len(got) != 3 || got[0] != "cdn.example.com:18443" {
		t.Fatalf("addresses with last-good = %v", got)
	}
	app.setLastGoodGRPCAddr("")

	// A direct deployment/config must not produce duplicate attempts.
	cfg.Panel.Host = "203.0.113.10"
	app.setGRPCHostOverride("")
	if got := app.panelGRPCAddresses(); len(got) != 1 || got[0] != "203.0.113.10:18443" {
		t.Fatalf("deduplicated addresses = %v", got)
	}
}

func TestDiscoverPanelOriginUpdatesGRPCAddress(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/agent/origin" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":"OK","data":{"grpc_host":"203.0.113.10"}}`))
	}))
	defer srv.Close()

	host, portStr, err := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatal(err)
	}

	cfg := &AgentConfig{}
	cfg.Panel.Host = host
	cfg.Panel.Port = port
	// 只有配置域名时才做源站发现（CDN 场景）：这里用域名模拟 CDN。
	cfg.Panel.GRPCHost = "cdn.example.com"
	cfg.Panel.GRPCPort = 18443
	app := &AgentApp{cfg: cfg}

	app.discoverPanelOrigin(context.Background())

	if got := app.panelGRPCAddress(); got != "203.0.113.10:18443" {
		t.Fatalf("panelGRPCAddress() = %q, want discovered origin", got)
	}
}

// 配置的是 IP 时它本身就是源站，绝不能把连接换成面板探测到的公网出口 IP
// （节点 14/19 就是因为这个从 192.168.0.151 被换成了 23.254.233.185 而永久离线）。
func TestDiscoverPanelOriginSkippedForIPConfig(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		_, _ = w.Write([]byte(`{"code":"OK","data":{"grpc_host":"203.0.113.10"}}`))
	}))
	defer srv.Close()

	host, portStr, err := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatal(err)
	}

	cfg := &AgentConfig{}
	cfg.Panel.Host = host // IP
	cfg.Panel.Port = port
	cfg.Panel.GRPCHost = host // 与 config.go 默认行为一致：缺省回落到 panel.host
	cfg.Panel.GRPCPort = 18443
	app := &AgentApp{cfg: cfg}

	if app.shouldDiscoverOrigin() {
		t.Fatalf("shouldDiscoverOrigin() = true for IP config")
	}
	app.discoverPanelOrigin(context.Background())
	if n := atomic.LoadInt32(&hits); n != 0 {
		t.Fatalf("origin discovery hit the panel %d time(s), want 0", n)
	}
	if got := app.panelGRPCAddress(); got != net.JoinHostPort(host, "18443") {
		t.Fatalf("panelGRPCAddress() = %q, want configured address kept", got)
	}
}
