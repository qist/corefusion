package coreconfig

import "testing"

// 外部 hy2 节点（经 external_node_servers 注册后选为出口）由 agent.buildExternalOutbound
// 派生出的中间形状 → sing-box 出站。这一组断言对应真机验证中发现的 4 个缺陷：
//   - 端口跳跃必须用 server_ports（`40000-50000` → `"40000:50000"`），且**不能再写 server_port**：
//     mihomo 的 `port` 是 0，`server_port: 0` 能过 `sing-box check`（core 不校验端口 ✗）但运行期连不上
//   - tls.server_name / alpn / utls 都要带过来
//   - allowInsecure → insecure（外部节点的 skip-cert-verify）
func TestConvertChainOutboundsToSingbox_ExternalHysteria2PortHopping(t *testing.T) {
	in := []interface{}{
		map[string]interface{}{
			"tag":      "chain-118-ext3",
			"protocol": "hysteria2",
			"settings": map[string]interface{}{
				"servers": []interface{}{
					map[string]interface{}{
						"address":     "rem5ogoe5m.swsixtwo.top",
						"port":        float64(0), // mihomo 的 port（端口跳跃时为 0）
						"serverPorts": []interface{}{"40000:50000"},
						"password":    "fdcfb24d-5883-4b96-fd09-666293bab439",
						"obfs": map[string]interface{}{
							"type": "salamander", "password": "mrMdT3L9AH2VVwPI",
						},
					},
				},
			},
			"streamSettings": map[string]interface{}{
				"network":       "tcp",
				"security":      "tls",
				"serverName":    "rem5ogoe5m.swsixtwo.top",
				"alpn":          []interface{}{"h3"},
				"fingerprint":   "chrome",
				"allowInsecure": true,
			},
		},
	}

	out := convertChainOutboundsToSingbox(in)
	if len(out) != 1 {
		t.Fatalf("期望 1 条出站，得到 %d", len(out))
	}
	ob, _ := out[0].(map[string]interface{})
	if ob["type"] != "hysteria2" || ob["server"] != "rem5ogoe5m.swsixtwo.top" {
		t.Fatalf("出站字段不对：%v", ob)
	}

	// 端口跳跃：只能用 server_ports，绝不能写 server_port（否则静默连不上 ✗）
	if v, has := ob["server_port"]; has {
		t.Errorf("端口跳跃时不能写 server_port（得到 %v）—— mihomo 的 port 是 0，写 0 不会报错但连不上", v)
	}
	ports, _ := ob["server_ports"].([]interface{})
	if len(ports) != 1 || ports[0] != "40000:50000" {
		t.Errorf("server_ports 应为 [\"40000:50000\"]，得到 %v", ob["server_ports"])
	}

	tls, _ := ob["tls"].(map[string]interface{})
	if tls == nil || tls["enabled"] != true {
		t.Fatalf("hy2 必须带 tls（缺了 sing-box 直接 FATAL TLS required）：%v", ob["tls"])
	}
	if tls["server_name"] != "rem5ogoe5m.swsixtwo.top" {
		t.Errorf("SNI 丢失：%v", tls)
	}
	if alpn, _ := tls["alpn"].([]interface{}); len(alpn) != 1 || alpn[0] != "h3" {
		t.Errorf("alpn 丢失：%v", tls)
	}
	if utls, _ := tls["utls"].(map[string]interface{}); utls == nil || utls["fingerprint"] != "chrome" {
		t.Errorf("utls/fingerprint 丢失：%v", tls)
	}
	if tls["insecure"] != true {
		t.Errorf("skip-cert-verify=true 应转成 tls.insecure：%v", tls)
	}

	obfs, _ := ob["obfs"].(map[string]interface{})
	if obfs == nil || obfs["type"] != "salamander" || obfs["password"] != "mrMdT3L9AH2VVwPI" {
		t.Errorf("obfs 丢失（服务端开了 obfs 就连不上）：%v", ob["obfs"])
	}
}

// 没有 serverPorts 时仍按 port 写 server_port（集群出口的普通 hy2 不受影响）。
func TestConvertChainOutboundsToSingbox_Hysteria2KeepsServerPort(t *testing.T) {
	in := []interface{}{
		map[string]interface{}{
			"tag":      "chain-118-1",
			"protocol": "hysteria2",
			"settings": map[string]interface{}{
				"servers": []interface{}{
					map[string]interface{}{"address": "a.example.com", "port": 47180, "password": "p"},
				},
			},
			"streamSettings": map[string]interface{}{"network": "tcp", "security": "tls"},
		},
	}
	ob, _ := convertChainOutboundsToSingbox(in)[0].(map[string]interface{})
	// 注意：集群路径的 port 在进程内是 int（未经 JSON 往返）
	if ob["server_port"] != 47180 {
		t.Errorf("server_port 应为 47180，得到 %v(%T)", ob["server_port"], ob["server_port"])
	}
}

// 外部 gRPC 出口：`grpcSettings.serviceName` 必须转成 sing-box 的 `transport.service_name`。
// 旧实现只读顶层 `ss["serviceName"]` ✗ → 服务名被静默丢掉 → 对不上服务端 → 连不上，
// 而 `sing-box check` 照样通过（该字段可选）✗。
func TestConvertChainOutboundsToSingbox_ExternalGRPCServiceName(t *testing.T) {
	in := []interface{}{
		map[string]interface{}{
			"tag":      "chain-4-ext3",
			"protocol": "vless",
			"settings": map[string]interface{}{
				"vnext": []interface{}{
					map[string]interface{}{
						"address": "exit.example.com", "port": 443,
						"users": []interface{}{map[string]interface{}{"id": "uuid-1", "encryption": "none"}},
					},
				},
			},
			"streamSettings": map[string]interface{}{
				"network":      "grpc",
				"security":     "reality",
				"serverName":   "nuvexo.com",
				"fingerprint":  "chrome",
				"grpcSettings": map[string]interface{}{"serviceName": "s7cKJaIE095Mtd3B0EpH0JXy31YR6GHF"},
				"realitySettings": map[string]interface{}{
					"publicKey": "PUBKEY", "shortId": "03e822599b2634",
					"serverName": "nuvexo.com", "fingerprint": "chrome",
				},
			},
		},
	}
	ob, _ := convertChainOutboundsToSingbox(in)[0].(map[string]interface{})
	tr, _ := ob["transport"].(map[string]interface{})
	if tr == nil || tr["type"] != "grpc" {
		t.Fatalf("transport 不对：%v", ob["transport"])
	}
	if tr["service_name"] != "s7cKJaIE095Mtd3B0EpH0JXy31YR6GHF" {
		t.Errorf("gRPC 服务名丢了（对不上服务端就连不上）：%v", tr)
	}
	tls, _ := ob["tls"].(map[string]interface{})
	if tls == nil || tls["server_name"] != "nuvexo.com" {
		t.Errorf("reality 的 SNI 丢了：%v", ob["tls"])
	}
	re, _ := tls["reality"].(map[string]interface{})
	if re == nil || re["public_key"] != "PUBKEY" || re["short_id"] != "03e822599b2634" {
		t.Errorf("reality 参数丢了：%v", tls["reality"])
	}
	if utls, _ := tls["utls"].(map[string]interface{}); utls == nil || utls["fingerprint"] != "chrome" {
		t.Errorf("reality 必须有 uTLS：%v", tls)
	}
}

// 外部 ws 出口：`wsSettings.path` 与 `wsSettings.headers.Host` 必须转过来
// （旧实现只读顶层 path ✗ → ws 落到默认路径 → 连不上）。
func TestConvertChainOutboundsToSingbox_ExternalWSPathAndHost(t *testing.T) {
	in := []interface{}{
		map[string]interface{}{
			"tag":      "chain-118-ext2",
			"protocol": "vless",
			"settings": map[string]interface{}{
				"vnext": []interface{}{
					map[string]interface{}{
						"address": "dns.example.com", "port": 443,
						"users": []interface{}{map[string]interface{}{"id": "uuid-2", "encryption": "none"}},
					},
				},
			},
			"streamSettings": map[string]interface{}{
				"network":    "ws",
				"security":   "tls",
				"serverName": "maimai.example.com",
				"wsSettings": map[string]interface{}{
					"path":    "/?ed=2560",
					"headers": map[string]interface{}{"Host": "maimai.example.com"},
				},
			},
		},
	}
	ob, _ := convertChainOutboundsToSingbox(in)[0].(map[string]interface{})
	tr, _ := ob["transport"].(map[string]interface{})
	if tr == nil || tr["type"] != "ws" {
		t.Fatalf("transport 不对：%v", ob["transport"])
	}
	if tr["path"] != "/?ed=2560" {
		t.Errorf("ws path 丢了：%v", tr)
	}
	// ⚠ sing-box 的 ws transport 没有 `host` 字段（写了核心直接 FATAL）→ 必须进 headers.Host
	if hdrs, _ := tr["headers"].(map[string]interface{}); hdrs == nil || hdrs["Host"] != "maimai.example.com" {
		t.Errorf("ws Host 丢了（须写进 headers.Host，不能写顶层 host）：%v", tr)
	}
	if _, bad := tr["host"]; bad {
		t.Errorf("不能写 transport.host —— sing-box 会报 unknown field 并拒绝启动：%v", tr)
	}
}

// 顶层写法仍要兼容（少数派生路径写在顶层）。
func TestConvertChainOutboundsToSingbox_TopLevelTransportFallback(t *testing.T) {
	in := []interface{}{
		map[string]interface{}{
			"tag":      "chain-1-2",
			"protocol": "vless",
			"settings": map[string]interface{}{
				"vnext": []interface{}{
					map[string]interface{}{
						"address": "a.example.com", "port": 443,
						"users": []interface{}{map[string]interface{}{"id": "uuid-3", "encryption": "none"}},
					},
				},
			},
			"streamSettings": map[string]interface{}{
				"network": "ws", "security": "none",
				"path": "/legacy", "serviceName": "svc-legacy",
			},
		},
	}
	ob, _ := convertChainOutboundsToSingbox(in)[0].(map[string]interface{})
	tr, _ := ob["transport"].(map[string]interface{})
	if tr == nil || tr["path"] != "/legacy" || tr["service_name"] != "svc-legacy" {
		t.Errorf("顶层回落失效：%v", ob["transport"])
	}
}

// port 为 0 且没有 serverPorts（异常数据）→ 不写 server_port: 0。
func TestConvertChainOutboundsToSingbox_Hysteria2OmitsZeroPort(t *testing.T) {
	in := []interface{}{
		map[string]interface{}{
			"tag":      "chain-118-1",
			"protocol": "hysteria2",
			"settings": map[string]interface{}{
				"servers": []interface{}{
					map[string]interface{}{"address": "a.example.com", "port": float64(0), "password": "p"},
				},
			},
			"streamSettings": map[string]interface{}{"network": "tcp", "security": "tls"},
		},
	}
	ob, _ := convertChainOutboundsToSingbox(in)[0].(map[string]interface{})
	if v, has := ob["server_port"]; has {
		t.Errorf("port=0 时不应写 server_port（写 0 不会被 check 拦下但连不上），得到 %v", v)
	}
}
