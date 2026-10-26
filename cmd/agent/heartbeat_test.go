package main

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
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
	cfg.Panel.GRPCPort = 18443
	app := &AgentApp{cfg: cfg}

	app.discoverPanelOrigin(context.Background())

	if got := app.panelGRPCAddress(); got != "203.0.113.10:18443" {
		t.Fatalf("panelGRPCAddress() = %q, want discovered origin", got)
	}
}
