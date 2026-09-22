package agent

import "testing"

// mihomoHy2ProxyJSON 是**真机看到的外部 hy2 节点**的 `proxy_json`（来自 external_nodes.id=3）。
// 关键特征（正是让旧实现出问题的地方）：
//   - **没有 `tls` 键** ✗（hy2 天生 TLS，mihomo 不写）→ 只看 pm["tls"] 会生成不带 tls 的出站
//   - `sni` / `alpn` / `client-fingerprint` 都在顶层
//   - obfs 是**扁平**的 `obfs: "salamander"` + `obfs-password`（不是嵌套 map）
//   - `port: 0` + `ports: "40000-50000"`（端口跳跃）
var mihomoHy2ProxyJSON = map[string]interface{}{
	"alpn":                 []interface{}{"h3"},
	"bbr-profile":          "conservative",
	"client-fingerprint":   "chrome",
	"down":                 "300 mbps",
	"fast-open":            true,
	"hop-interval":         "5-10",
	"name":                 "hysteria2",
	"network":              "tcp",
	"obfs":                 "salamander",
	"obfs-max-packet-size": float64(1200),
	"obfs-min-packet-size": float64(512),
	"obfs-password":        "mrMdT3L9AH2VVwPI",
	"password":             "fdcfb24d-5883-4b96-fd09-666293bab439",
	"port":                 float64(0),
	"ports":                "40000-50000",
	"server":               "rem5ogoe5m.swsixtwo.top",
	"skip-cert-verify":     false,
	"sni":                  "rem5ogoe5m.swsixtwo.top",
	"type":                 "hysteria2",
	"up":                   "300 mbps",
}

// TestBuildStreamFromProxyMapHysteria2WithoutTLSKey hy2 的映射里**没有 `tls` 键**，
// 但出站**必须**带 tls（sing-box 缺了直接 `FATAL TLS required`）→ 协议名兜底必须生效。
func TestBuildStreamFromProxyMapHysteria2WithoutTLSKey(t *testing.T) {
	ss := buildStreamFromProxyMap(mihomoHy2ProxyJSON, "hysteria2")

	if ss["security"] != "tls" {
		t.Fatalf("hy2 必须判为 TLS（否则 sing-box FATAL TLS required），得到 security=%v", ss["security"])
	}
	if ss["serverName"] != "rem5ogoe5m.swsixtwo.top" {
		t.Errorf("SNI 应取自 sni 键，得到 %v", ss["serverName"])
	}
	if alpn, _ := ss["alpn"].([]interface{}); len(alpn) != 1 || alpn[0] != "h3" {
		t.Errorf("alpn 应原样带过，得到 %v", ss["alpn"])
	}
	if ss["fingerprint"] != "chrome" {
		t.Errorf("fingerprint 应取自 client-fingerprint，得到 %v", ss["fingerprint"])
	}
}

// 同一个映射若按 vless 解析（没有 tls 键）→ 不该凭空加 TLS。
func TestBuildStreamFromProxyMapNoTLSKeyStaysPlainForNonTLSTProtocol(t *testing.T) {
	pm := map[string]interface{}{"network": "tcp", "port": float64(443)}
	if ss := buildStreamFromProxyMap(pm, "shadowsocks"); ss["security"] != nil {
		t.Errorf("非 TLS 协议且无 tls 键时不应设置 security，得到 %v", ss["security"])
	}
}

// TestBuildStreamFromProxyMapRealityOpts 外部 reality 节点：`reality-opts` 必须被转成
// realitySettings —— 旧实现完全忽略它，会把 reality 当成**普通 TLS**，连不上（且核心不报错）。
func TestBuildStreamFromProxyMapRealityOpts(t *testing.T) {
	pm := map[string]interface{}{
		"type":               "vless",
		"network":            "grpc",
		"tls":                true,
		"servername":         "nuvexo.com",
		"client-fingerprint": "chrome",
		"reality-opts": map[string]interface{}{
			"public-key": "LSmdyYMFOWUsw2tv5eteIY6zn_tXQYP8ndKH-VEAFGM",
			"short-id":   "03e822599b2634",
		},
		"grpc-opts": map[string]interface{}{"grpc-service-name": "svc"},
	}
	ss := buildStreamFromProxyMap(pm, "vless")

	if ss["security"] != "reality" {
		t.Fatalf("有 reality-opts 时必须判为 reality，得到 %v", ss["security"])
	}
	re, _ := ss["realitySettings"].(map[string]interface{})
	if re == nil {
		t.Fatal("必须生成 realitySettings（否则 reality 退化成普通 TLS → 连不上）")
	}
	if re["publicKey"] != "LSmdyYMFOWUsw2tv5eteIY6zn_tXQYP8ndKH-VEAFGM" {
		t.Errorf("publicKey 丢失：%v", re)
	}
	if re["shortId"] != "03e822599b2634" {
		t.Errorf("shortId 丢失：%v", re)
	}
	if re["serverName"] != "nuvexo.com" {
		t.Errorf("reality 的 SNI 应写进 realitySettings.serverName：%v", re)
	}
	if re["fingerprint"] != "chrome" {
		t.Errorf("reality 客户端需要指纹：%v", re)
	}
}

// TestMihomoHy2Obfs obfs 在 mihomo 里是**扁平**的 `obfs` + `obfs-password` ✗（不是嵌套 map），
// 旧实现只找嵌套 map → 静默丢掉 obfs（服务端开了 obfs 就连不上）。
func TestMihomoHy2Obfs(t *testing.T) {
	// ① 真实（扁平）形状
	got := mihomoHy2Obfs(mihomoHy2ProxyJSON)
	if got["type"] != "salamander" || got["password"] != "mrMdT3L9AH2VVwPI" {
		t.Errorf("扁平形状解析错：%v", got)
	}
	// ② 嵌套形状（部分来源会这么给）
	nested := mihomoHy2Obfs(map[string]interface{}{
		"obfs": map[string]interface{}{"type": "salamander", "password": "p"},
	})
	if nested["type"] != "salamander" || nested["password"] != "p" {
		t.Errorf("嵌套形状解析错：%v", nested)
	}
	// ③ 没有 obfs
	if got := mihomoHy2Obfs(map[string]interface{}{"type": "hysteria2"}); got != nil {
		t.Errorf("没有 obfs 时应返回 nil，得到 %v", got)
	}
}

// TestMihomoPortRanges 端口跳跃：mihomo 用 `40000-50000`，sing-box 的 `server_ports` 用冒号。
func TestMihomoPortRanges(t *testing.T) {
	cases := []struct {
		in   interface{}
		want []string
	}{
		{"40000-50000", []string{"40000:50000"}},
		{"40000-50000,51000", []string{"40000:50000", "51000"}},
		{"  40000-50000 , 51000  ", []string{"40000:50000", "51000"}},
		{"", nil},
		{float64(443), nil}, // 不是字符串 → 不处理
		{nil, nil},
	}
	for _, c := range cases {
		got := mihomoPortRanges(map[string]interface{}{"ports": c.in})
		if len(got) != len(c.want) {
			t.Errorf("ports=%v：期望 %v，得到 %v", c.in, c.want, got)
			continue
		}
		for i := range c.want {
			if got[i] != c.want[i] {
				t.Errorf("ports=%v：第 %d 项期望 %q，得到 %v", c.in, i, c.want[i], got[i])
			}
		}
	}
}

// 常规分支不回归：trojan + ws + tls。
func TestBuildStreamFromProxyMapTrojanWS(t *testing.T) {
	pm := map[string]interface{}{
		"type":     "trojan",
		"network":  "ws",
		"tls":      true,
		"sni":      "sweet-bonus-aeb7.boown.com",
		"ws-opts":  map[string]interface{}{"path": "/?ed=2560"},
		"password": "p",
	}
	ss := buildStreamFromProxyMap(pm, "trojan")
	if ss["security"] != "tls" || ss["serverName"] != "sweet-bonus-aeb7.boown.com" {
		t.Errorf("trojan tls 解析错：%v", ss)
	}
	ws, _ := ss["wsSettings"].(map[string]interface{})
	if ws == nil || ws["path"] != "/?ed=2560" {
		t.Errorf("wsSettings 解析错：%v", ss["wsSettings"])
	}
}
