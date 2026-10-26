package agent

import (
	"strings"
	"testing"
)

// 安全审计 v5 M6：安装脚本必须**自动**把面板 CA 交给 Agent（gRPC mTLS 心跳要用它校验面板），
// 并且把含 bootstrap token 的 config.yaml 收紧到 0600 —— 全程不需要人工填写/操作。
func TestInstallScriptShipsPanelCAAndProtectsConfig(t *testing.T) {
	caPEM := "-----BEGIN CERTIFICATE-----\nMIIBfakefakefake\n-----END CERTIFICATE-----"
	script, err := NewAgentInstallScript("panel.example.com", 8080).
		WithScheme("https").
		WithTLS(true).
		WithBootstrapToken("bootstrap-token-xyz").
		WithServerID(7).
		WithCACertPEM([]byte(caPEM)).
		Generate()
	if err != nil {
		t.Fatalf("生成安装脚本失败：%v", err)
	}

	if !strings.Contains(script, caPEM) {
		t.Error("安装脚本里必须带上面板 CA 证书（否则 Agent gRPC 心跳无法校验面板身份）")
	}
	if !strings.Contains(script, `CA_DIR="$INSTALL_DIR/data/certs"`) || !strings.Contains(script, `"$CA_DIR/ca-cert.pem"`) {
		t.Error("CA 证书要落到 Agent data_dir 下的 certs/ca-cert.pem（正是心跳读取的路径）")
	}
	if !strings.Contains(script, `chmod 600 "$CONFIG_FILE"`) {
		t.Error("config.yaml 里有 bootstrap token，必须 chmod 600")
	}
	if !strings.Contains(script, "bootstrap-token-xyz") {
		t.Error("脚本应包含 bootstrap token（由面板按需生成，属正常流程）")
	}
}

// 节点机需要 nft/tc 等系统命令才能做限速/防火墙，安装脚本必须自动补齐，
// 而不是让用户装完 Agent 才发现限速不可用。
func TestInstallScriptInstallsRuntimeDeps(t *testing.T) {
	script, err := NewAgentInstallScript("panel.example.com", 8080).
		WithBootstrapToken("t").
		WithServerID(2).
		Generate()
	if err != nil {
		t.Fatalf("生成失败：%v", err)
	}
	for _, want := range []string{
		"install_runtime_deps",
		"nftables",
		"iproute2",
		"conntrack-tools",
		"procps",
		"lsof",
		"openssl",
		"CF_SKIP_DEPS",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("安装脚本缺少运行时依赖相关内容：%q", want)
		}
	}
	// 依赖安装必须在下载/启动之前，否则 Agent 起来后限速仍然不可用
	depAt := strings.Index(script, "install_runtime_deps")
	startAt := strings.Index(script, "systemctl restart corefusion-agent")
	if depAt < 0 || startAt < 0 || depAt > startAt {
		t.Error("依赖安装应在启动服务之前")
	}
}

// 面板走 CDN 时，HTTP 入口是 CDN 域名，但 gRPC 必须直连源站 IP。
// 安装脚本必须把两者分开写入，Agent 才能自动连上，不需要人工改 YAML。
func TestInstallScriptUsesSeparateGRPCHost(t *testing.T) {
	script, err := NewAgentInstallScript("panel.example.com", 443).
		WithScheme("https").
		WithTLS(true).
		WithBootstrapEndpoint("203.0.113.10", 8080, false).
		WithGRPCHost("203.0.113.10").
		WithGRPCPort(18443).
		WithBootstrapToken("tok").
		WithServerID(3).
		Generate()
	if err != nil {
		t.Fatalf("生成安装脚本失败：%v", err)
	}
	for _, want := range []string{
		// 脚本/二进制仍从 CDN 下载
		`PANEL_SCHEME="https"`,
		`PANEL_HOST="panel.example.com"`,
		`PANEL_PORT="443"`,
		`PANEL_TLS=true`,
		`PANEL_DOWNLOAD_URL="${PANEL_SCHEME}://${PANEL_HOST}:${PANEL_PORT}/api/v1/agent/download`,
		// Agent 配置直连源站：引导 + gRPC
		`BOOTSTRAP_SCHEME="http"`,
		`BOOTSTRAP_HOST="203.0.113.10"`,
		`BOOTSTRAP_PORT="8080"`,
		`PANEL_GRPC_HOST="203.0.113.10"`,
		`  host: "$BOOTSTRAP_HOST"`,
		`  port: $BOOTSTRAP_PORT`,
		`  tls: $BOOTSTRAP_TLS`,
		`  public_host: "$PANEL_HOST"`,
		`  public_port: $PANEL_PORT`,
		`  public_tls: $PANEL_TLS`,
		`  grpc_host: "$PANEL_GRPC_HOST"`,
		`  grpc_port: $PANEL_GRPC_PORT`,
	} {
		if !strings.Contains(script, want) {
			t.Errorf("安装脚本缺少 %q，CDN 场景下 Agent 会连错地址", want)
		}
	}
}

// 兼容直接部署（没有 CDN）：未显式给 grpc_host 时应回退到 panel.host。
func TestInstallScriptGRPCHostFallsBackToPanelHost(t *testing.T) {
	script, err := NewAgentInstallScript("panel.example.com", 8080).
		WithBootstrapToken("tok").
		WithServerID(4).
		Generate()
	if err != nil {
		t.Fatalf("生成安装脚本失败：%v", err)
	}
	if !strings.Contains(script, `PANEL_GRPC_HOST="panel.example.com"`) {
		t.Error("未设置 grpc_host 时应回退到 panel.host")
	}
	if !strings.Contains(script, `BOOTSTRAP_HOST="panel.example.com"`) {
		t.Error("未设置 bootstrap endpoint 时应回退到 panel.host/port/tls")
	}
	if !strings.Contains(script, `  port: $BOOTSTRAP_PORT`) || !strings.Contains(script, `BOOTSTRAP_PORT="8080"`) {
		t.Error("未设置 bootstrap endpoint 时应回退到 panel.port")
	}
}

// 没提供 CA（理论上走不到，防御性）时脚本仍要能生成，只是不带 CA 段落。
func TestInstallScriptWithoutCAStillGenerates(t *testing.T) {
	script, err := NewAgentInstallScript("panel.example.com", 8080).
		WithBootstrapToken("t").
		WithServerID(1).
		Generate()
	if err != nil {
		t.Fatalf("生成失败：%v", err)
	}
	if strings.Contains(script, "BEGIN CERTIFICATE") {
		t.Error("没提供 CA 时不应凭空出现证书内容")
	}
	if !strings.Contains(script, `chmod 600 "$CONFIG_FILE"`) {
		t.Error("config 权限收紧与 CA 无关，必须始终生效")
	}
}

// 回归：安装脚本写出的 config.yaml 必须是**合法 YAML** ✓。
// 真机上踩到过：`data_dir` 前面多了 2 个空格 ✗ → Agent 一启动就
// `parse config: yaml: line 13: mapping values are not allowed in this context` ✗ 起不来 ✓。
func TestInstallScriptConfigYAMLIndent(t *testing.T) {
	script, err := NewAgentInstallScript("panel.example.com", 8080).
		WithBootstrapToken("tok").
		WithServerID(19).
		Generate()
	if err != nil {
		t.Fatal(err)
	}
	// 取 cat > "$CONFIG_FILE" << EOF ... EOF 之间的内容 ✓
	start := strings.Index(script, `cat > "$CONFIG_FILE" << EOF`)
	if start < 0 {
		t.Fatal("脚本里找不到写 config 的片段")
	}
	body := script[start:]
	if end := strings.Index(body, "\nEOF"); end > 0 {
		body = body[:end]
	}
	// 顶层键下的子键必须是 2 空格缩进 ✓；出现 4 空格说明写歪了 ✗
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "    data_dir:") || strings.HasPrefix(line, "    log_level:") {
			t.Fatalf("config.yaml 缩进过深（应为 2 空格）: %q", line)
		}
	}
	if !strings.Contains(body, "\n  data_dir: ") {
		t.Fatalf("config.yaml 缺少正确缩进的 data_dir：\n%s", body)
	}
}
