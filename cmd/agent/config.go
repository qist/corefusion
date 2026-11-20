package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"corefusion/internal/constants"
	"gopkg.in/yaml.v3"
)

// AgentConfig defines the YAML configuration for the Agent daemon.
type AgentConfig struct {
	Panel struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"` // Panel HTTP port (for bootstrap REST API)
		// GRPCHost is the address used for the mTLS heartbeat connection.
		// It defaults to Host. When the panel is fronted by a normal CDN
		// (which only proxies HTTP/HTTPS), Host is the CDN domain and cannot
		// serve the gRPC port; the panel fills this with its origin IP so no
		// manual edit is needed.
		GRPCHost string `yaml:"grpc_host"`
		GRPCPort int    `yaml:"grpc_port"` // Panel gRPC port (for mTLS heartbeat)
		ServerID int64  `yaml:"server_id"`
		TLS      bool   `yaml:"tls"` // Use HTTPS for bootstrap API (production behind Caddy/Nginx)

		// PublicHost/PublicPort/PublicTLS describe the panel's CDN/reverse-proxy
		// HTTP endpoint. Host/Port/TLS point at the panel's direct listener
		// (used first); when that listener is not exposed to the node, bootstrap
		// falls back to this public endpoint. The returned Agent certificate is
		// still verified against the panel CA, so a TLS-terminating CDN cannot
		// substitute its own certificate.
		PublicHost string `yaml:"public_host"`
		PublicPort int    `yaml:"public_port"`
		PublicTLS  bool   `yaml:"public_tls"`
	} `yaml:"panel"`
	Bootstrap struct {
		Token string `yaml:"token"`
		// PanelCertFingerprint 是面板 TLS 证书链**根（最后一环）**的 SHA-256（hex，可带冒号）。
		// 设置后引导阶段严格比对：即使没有面板 CA，也不会信任首次连接的证书。
		// 安全审计 v7 L-5：原来没有 CA 时会自动 pin 首连证书 → 首装时处于网络中间位置
		// 的人可以固定自己的证书并截获 bootstrap token。
		PanelCertFingerprint string `yaml:"panel_cert_fingerprint"`
		// AllowTOFU 显式允许「首连同证书自动固定」（trust-on-first-use）。
		// 默认关闭：安装脚本会随带面板 CA，正常安装无需开启；仅为无 CA 的历史手工安装保留。
		AllowTOFU bool `yaml:"allow_tofu"`
	} `yaml:"bootstrap"`
	Agent struct {
		LogLevel string `yaml:"log_level"`
		DataDir  string `yaml:"data_dir"`
	} `yaml:"agent"`
	// Update 段：Agent 自更新签名校验（安全审计 v7 L-6）。
	// public_key 是面板更新签名公钥（PEM），由安装脚本写入；设置后拒绝未签名/签名不符的更新。
	Update struct {
		PublicKey string `yaml:"public_key"`
	} `yaml:"update"`
}

// 说明 ✓：Agent **没有** acme 配置段。
// ACME 账号邮箱、CA、挑战方式这些统统由**面板**在 `IssuePublicCertCommand` 里下发 ✓
// （面板本来就有 ACME 账号 ✓：`acme_account_configs`，见证书页的"邮箱"下拉 ✓），
// 挑战路径也由面板自己掌握 ✓（面板生成节点 web 配置时就带 `/.well-known/acme-challenge/` ✓）。
// 让每个节点各填一份 yaml 属于重复配置 ✗，节点侧应当零配置 ✓。

func loadConfig(path string) (*AgentConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg AgentConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if cfg.Agent.DataDir == "" {
		cfg.Agent.DataDir = "/opt/corefusion-agent/data"
	}
	if cfg.Panel.GRPCPort == 0 {
		cfg.Panel.GRPCPort = constants.DefaultAgentGRPCPort
	}
	if cfg.Panel.Port == 0 {
		cfg.Panel.Port = 8080
	}
	cfg.Panel.PublicHost = strings.TrimSpace(cfg.Panel.PublicHost)
	if cfg.Panel.PublicHost != "" && (cfg.Panel.PublicPort <= 0 || cfg.Panel.PublicPort > 65535) {
		cfg.Panel.PublicPort = cfg.Panel.Port
	}
	// Backward compatible: older configs only have panel.host. Keep using it
	// for gRPC unless the panel explicitly installed a separate grpc_host.
	if strings.TrimSpace(cfg.Panel.GRPCHost) == "" {
		cfg.Panel.GRPCHost = cfg.Panel.Host
	} else {
		cfg.Panel.GRPCHost = strings.TrimSpace(cfg.Panel.GRPCHost)
	}
	if fp := normalizeCertFingerprint(cfg.Bootstrap.PanelCertFingerprint); fp != "" {
		if len(fp) != 64 {
			return nil, fmt.Errorf("bootstrap.panel_cert_fingerprint 必须是 64 位 hex（SHA-256），当前为 %q", cfg.Bootstrap.PanelCertFingerprint)
		}
		cfg.Bootstrap.PanelCertFingerprint = fp
	} else {
		cfg.Bootstrap.PanelCertFingerprint = ""
	}
	return &cfg, nil
}

// normalizeCertFingerprint 去掉指纹里的空格、冒号与大小写差异（sha256sum/fingerprint 两种写法都收）。
func normalizeCertFingerprint(fp string) string {
	fp = strings.ToLower(strings.TrimSpace(fp))
	fp = strings.ReplaceAll(fp, ":", "")
	fp = strings.ReplaceAll(fp, " ", "")
	return fp
}

// WriteUpdatePublicKey 把面板更新签名公钥写进 config.yaml 的 update.public_key。
//
// 为什么需要：安装脚本只在**首次安装/重装**时写这一段；老节点用自更新升级后
// 配置文件原样保留（没有 update 段）→ Agent 只校验 SHA-256、不校验 ed25519，
// 安全审计 v7 L-6 的保护形同虚设。新版 Agent 连上面板后调用 WriteUpdatePublicKey
// 补齐，之后强制校验签名。
//
// 用文本方式改写而不是 yaml.Marshal：config.yaml 里有注释与字段顺序，重新序列化
// 会把它们全部丢掉。写入走临时文件 + rename，避免写一半断电留下坏配置。
func WriteUpdatePublicKey(configPath, pem string) error {
	pem = strings.TrimSpace(pem)
	if pem == "" {
		return fmt.Errorf("empty update public key")
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")

	block := []string{"update:", "  public_key: |"}
	for _, ln := range strings.Split(pem, "\n") {
		block = append(block, "    "+strings.TrimRight(ln, "\r"))
	}

	out := make([]string, 0, len(lines)+len(block)+2)
	start := -1
	for i, ln := range lines {
		if strings.TrimRight(ln, " \t\r") == "update:" {
			start = i
			break
		}
	}
	if start >= 0 {
		end := len(lines)
		for j := start + 1; j < len(lines); j++ {
			t := lines[j]
			if strings.TrimSpace(t) == "" || strings.HasPrefix(strings.TrimSpace(t), "#") {
				continue
			}
			if !strings.HasPrefix(t, " ") && !strings.HasPrefix(t, "\t") {
				end = j
				break
			}
		}
		out = append(out, lines[:start]...)
		out = append(out, block...)
		if end < len(lines) {
			out = append(out, "")
			out = append(out, lines[end:]...)
		}
	} else {
		out = append(out, lines...)
		if len(out) > 0 && strings.TrimSpace(out[len(out)-1]) != "" {
			out = append(out, "")
		}
		out = append(out, block...)
		out = append(out, "")
	}

	tmp := filepath.Join(filepath.Dir(configPath), "."+filepath.Base(configPath)+".tmp")
	if err := os.WriteFile(tmp, []byte(strings.Join(out, "\n")), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, configPath)
}
