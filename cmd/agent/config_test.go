package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeAgentConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

// 旧版 config.yaml 只有 panel.host：升级后必须继续用它做 gRPC 地址，
// 不能因为新增了 grpc_host 字段就断掉已有节点。
func TestLoadConfigGRPCHostDefaultsToHost(t *testing.T) {
	path := writeAgentConfig(t, `
panel:
  host: "panel.example.com"
  port: 443
  grpc_port: 18443
  server_id: 7
  tls: true
agent:
  data_dir: "/tmp/agent"
`)
	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Panel.GRPCHost != "panel.example.com" {
		t.Fatalf("grpc_host 应回退到 panel.host，得到 %q", cfg.Panel.GRPCHost)
	}
}

// CDN 场景：HTTP 走 CDN 域名，gRPC 走源站 IP，两者必须互不覆盖。
func TestLoadConfigKeepsSeparateGRPCHost(t *testing.T) {
	path := writeAgentConfig(t, `
panel:
  host: "panel.example.com"
  port: 443
  grpc_host: "203.0.113.10"
  grpc_port: 18443
  server_id: 8
  tls: true
`)
	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Panel.Host != "panel.example.com" {
		t.Fatalf("panel.host 被改动了：%q", cfg.Panel.Host)
	}
	if cfg.Panel.GRPCHost != "203.0.113.10" {
		t.Fatalf("grpc_host 未保留：%q", cfg.Panel.GRPCHost)
	}
}

// CDN 源站 HTTP 端口被防火墙挡住时，Agent 需要回退到公网域名引导；
// public_* 字段必须完整保留。
func TestLoadConfigPublicBootstrapFallback(t *testing.T) {
	path := writeAgentConfig(t, `
panel:
  host: "203.0.113.10"
  port: 8080
  grpc_host: "203.0.113.10"
  grpc_port: 18443
  public_host: "panel.example.com"
  public_port: 443
  public_tls: true
  server_id: 9
  tls: false
`)
	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Panel.PublicHost != "panel.example.com" || cfg.Panel.PublicPort != 443 || !cfg.Panel.PublicTLS {
		t.Fatalf("public_* 未保留：host=%q port=%d tls=%v",
			cfg.Panel.PublicHost, cfg.Panel.PublicPort, cfg.Panel.PublicTLS)
	}
}
