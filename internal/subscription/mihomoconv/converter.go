// Package mihomoconv converts V2Ray/xray share links to mihomo proxy maps.
// Ported from github.com/metacubex/mihomo/common/convert with minimal dependencies.
package mihomoconv

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/url"
	"strconv"
	"strings"
)

var encRaw = base64.RawStdEncoding
var enc = base64.StdEncoding

// ConvertsV2Ray converts V2Ray subscribe proxies data to mihomo proxies config.
func ConvertsV2Ray(buf []byte) ([]map[string]any, error) {
	data := DecodeBase64(buf)
	arr := strings.Split(string(data), "\n")
	proxies := make([]map[string]any, 0, len(arr))
	names := make(map[string]int, 200)

	for _, line := range arr {
		line = strings.TrimRight(line, " \r")
		if line == "" {
			continue
		}
		scheme, body, found := strings.Cut(line, "://")
		if !found {
			continue
		}
		scheme = strings.ToLower(scheme)
		switch scheme {
		case "hysteria":
			urlHysteria, err := url.Parse(line)
			if err != nil {
				continue
			}
			query := urlHysteria.Query()
			name := uniqueName(names, urlHysteria.Fragment)
			hysteria := make(map[string]any, 20)
			hysteria["name"] = name
			hysteria["type"] = scheme
			hysteria["server"] = urlHysteria.Hostname()
			hysteria["port"] = urlHysteria.Port()
			hysteria["sni"] = query.Get("peer")
			hysteria["obfs"] = query.Get("obfs")
			if alpn := query.Get("alpn"); alpn != "" {
				hysteria["alpn"] = strings.Split(alpn, ",")
			}
			hysteria["auth_str"] = query.Get("auth")
			hysteria["protocol"] = query.Get("protocol")
			up := query.Get("up")
			down := query.Get("down")
			if up == "" {
				up = query.Get("upmbps")
			}
			if down == "" {
				down = query.Get("downmbps")
			}
			hysteria["down"] = down
			hysteria["up"] = up
			hysteria["skip-cert-verify"], _ = strconv.ParseBool(query.Get("insecure"))
			proxies = append(proxies, hysteria)

		case "anytls", "any-tls":
			// anytls://password@host:port/?sni=..&insecure=1&fp=..&alpn=..（官方 URI 规范）
			urlAnyTLS, err := url.Parse(line)
			if err != nil {
				continue
			}
			query := urlAnyTLS.Query()
			anytls := map[string]any{
				"name":   uniqueName(names, urlAnyTLS.Fragment),
				"type":   "anytls",
				"server": urlAnyTLS.Hostname(),
				"udp":    true,
			}
			if port := urlAnyTLS.Port(); port != "" {
				anytls["port"] = port
			} else {
				anytls["port"] = "443"
			}
			// 注意用 Username()（已解码）：User.String() 返回的是**转义后**的 userinfo，
			// 密码里的 %40 / %2B 之类会原样带进配置 → 认证失败。
			if auth := urlAnyTLS.User.Username(); auth != "" {
				anytls["password"] = auth
			}
			if sni := query.Get("sni"); sni != "" {
				anytls["sni"] = sni
			}
			if insecure := query.Get("insecure"); insecure == "1" || insecure == "true" {
				anytls["skip-cert-verify"] = true
			}
			if fp := query.Get("fp"); fp != "" {
				anytls["client-fingerprint"] = fp
			}
			if alpn := query.Get("alpn"); alpn != "" {
				anytls["alpn"] = strings.Split(alpn, ",")
			}
			proxies = append(proxies, anytls)

		case "hysteria2", "hy2":
			urlHysteria2, err := url.Parse(line)
			if err != nil {
				continue
			}
			query := urlHysteria2.Query()
			name := uniqueName(names, urlHysteria2.Fragment)
			hysteria2 := make(map[string]any, 20)
			hysteria2["name"] = name
			hysteria2["type"] = "hysteria2"
			hysteria2["server"] = urlHysteria2.Hostname()
			if port := urlHysteria2.Port(); port != "" {
				hysteria2["port"] = port
			} else {
				hysteria2["port"] = "443"
			}
			hysteria2["obfs"] = query.Get("obfs")
			hysteria2["obfs-password"] = query.Get("obfs-password")
			hysteria2["sni"] = query.Get("sni")
			hysteria2["skip-cert-verify"], _ = strconv.ParseBool(query.Get("insecure"))
			if alpn := query.Get("alpn"); alpn != "" {
				hysteria2["alpn"] = strings.Split(alpn, ",")
			}
			// 同 anytls：必须用已解码的 Username()，否则 base64 密码里的 + / = 会变成
			// %2B / %2F / %3D 带进 mihomo 配置 → 认证必然失败。
			if auth := urlHysteria2.User.Username(); auth != "" {
				hysteria2["password"] = auth
			}
			// TLS (hysteria2 always uses TLS)
			security := strings.ToLower(query.Get("security"))
			if security == "tls" || security == "" {
				hysteria2["tls"] = true
			}
			// client-fingerprint (browser fingerprint)
			if fp := query.Get("fp"); fp != "" {
				hysteria2["client-fingerprint"] = fp
			}
			// fingerprint (TLS certificate pinning)
			if pin := query.Get("pinSHA256"); pin != "" {
				hysteria2["fingerprint"] = pin
			}
			// 带宽：链接可能用 up/down（xray 口径）或 upmbps/downmbps（hysteria 官方口径，
			// sing-box 侧就用这个），两种都认。
			up := query.Get("up")
			if up == "" {
				up = query.Get("upmbps")
			}
			down := query.Get("down")
			if down == "" {
				down = query.Get("downmbps")
			}
			if up != "" {
				hysteria2["up"] = up
			}
			if down != "" {
				hysteria2["down"] = down
			}
			// 动态端口：链接以 mport 下发跳跃段（兼容 ports 写法）
			if ports := query.Get("mport"); ports != "" {
				hysteria2["ports"] = ports
			} else if ports := query.Get("ports"); ports != "" {
				hysteria2["ports"] = ports
			}
			if hop := query.Get("hop-interval"); hop != "" {
				hysteria2["hop-interval"] = hop
			}
			// bbr-profile：sing-box 侧以独立链接参数下发（不在 fm 里），漏读就丢拥塞控制
			if bbr := query.Get("bbr-profile"); bbr != "" && bbr != "standard" {
				hysteria2["bbr-profile"] = bbr
			}

			// Parse fm param for quicParams
			if fmStr := query.Get("fm"); fmStr != "" {
				var fm map[string]any
				if err := json.Unmarshal([]byte(fmStr), &fm); err == nil {
					// Obfs from udp (salamander / gecko)
					if udp, ok := fm["udp"].([]any); ok {
						for _, item := range udp {
							if m, ok := item.(map[string]any); ok {
								obfsType, _ := m["type"].(string)
								s, _ := m["settings"].(map[string]any)
								if s == nil {
									continue
								}
								switch obfsType {
								case "salamander":
									if hysteria2["obfs"] == nil || hysteria2["obfs"] == "" {
										if pwd, ok := s["password"].(string); ok && pwd != "" {
											hysteria2["obfs"] = "salamander"
											hysteria2["obfs-password"] = pwd
										}
									}
								case "gecko":
									if hysteria2["obfs"] == nil || hysteria2["obfs"] == "" {
										hysteria2["obfs"] = "gecko"
										if pwd, ok := s["password"].(string); ok && pwd != "" {
											hysteria2["obfs-password"] = pwd
										}
										if ps, ok := s["packetSize"].(string); ok && ps != "" {
											parts := strings.SplitN(ps, "-", 2)
											if len(parts) == 2 {
												if min, err := strconv.Atoi(strings.TrimSpace(parts[0])); err == nil && min > 0 {
													hysteria2["obfs-min-packet-size"] = min
												}
												if max, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil && max > 0 {
													hysteria2["obfs-max-packet-size"] = max
												}
											}
										}
									}
								case "realm":
									realmOpts := map[string]any{"enable": true}
									if url, ok := s["url"].(string); ok && url != "" {
										realmOpts["server-url"] = url
									}
									if token, ok := s["token"].(string); ok && token != "" {
										realmOpts["token"] = token
									}
									if realmID, ok := s["realm-id"].(string); ok && realmID != "" {
										realmOpts["realm-id"] = realmID
									}
									if stunServers, ok := s["stunServers"].([]any); ok && len(stunServers) > 0 {
										var servers []string
										for _, ss := range stunServers {
											if s, ok := ss.(string); ok {
												servers = append(servers, s)
											}
										}
										if len(servers) > 0 {
											realmOpts["stun-servers"] = servers
										}
									}
									if len(realmOpts) > 1 { // more than just "enable"
										hysteria2["realm-opts"] = realmOpts
									}
								}
							}
						}
					}
					// quicParams
					if qp, ok := fm["quicParams"].(map[string]any); ok {
						// ports and hop-interval from udpHop
						if hop, ok := qp["udpHop"].(map[string]any); ok {
							if ports, ok := hop["ports"].(string); ok && ports != "" {
								hysteria2["ports"] = ports
							}
							if interval, ok := hop["interval"].(string); ok && interval != "" {
								hysteria2["hop-interval"] = interval
							}
						}
						// bbr-profile
						if bp, ok := qp["bbrProfile"].(string); ok && bp != "" && bp != "standard" {
							hysteria2["bbr-profile"] = bp
						}
						// up/down from brutalUp/brutalDown (only for brutal/force-brutal congestion)
						congestion, _ := qp["congestion"].(string)
						if congestion == "brutal" || congestion == "force-brutal" {
							if up, ok := qp["brutalUp"].(string); ok && up != "" {
								hysteria2["up"] = up
							}
							if down, ok := qp["brutalDown"]; ok {
								switch v := down.(type) {
								case string:
									if v != "" {
										hysteria2["down"] = v
									}
								case float64:
									if v > 0 {
										hysteria2["down"] = fmt.Sprintf("%v Mbps", v)
									}
								}
							}
						}
						// QUIC window settings
						if v, ok := qp["initStreamReceiveWindow"].(float64); ok && v > 0 {
							hysteria2["initial-stream-receive-window"] = int(v)
						}
						if v, ok := qp["maxStreamReceiveWindow"].(float64); ok && v > 0 {
							hysteria2["max-stream-receive-window"] = int(v)
						}
						if v, ok := qp["initConnectionReceiveWindow"].(float64); ok && v > 0 {
							hysteria2["initial-connection-receive-window"] = int(v)
						}
						if v, ok := qp["maxConnectionReceiveWindow"].(float64); ok && v > 0 {
							hysteria2["max-connection-receive-window"] = int(v)
						}
						if v, ok := qp["maxIdleTimeout"].(float64); ok && v > 0 {
							hysteria2["max-idle-timeout"] = int(v)
						}
						if v, ok := qp["keepAlivePeriod"].(float64); ok && v > 0 {
							hysteria2["keep-alive-period"] = int(v)
						}
						if v, ok := qp["disablePathMTUDiscovery"].(bool); ok && v {
							hysteria2["disable-path-mtu-discovery"] = true
						}
						if v, ok := qp["maxIncomingStreams"].(float64); ok && v > 0 {
							hysteria2["max-incoming-streams"] = int(v)
						}
					}
				}
			}

			// mport param (alternative to fm)
			if mport := query.Get("mport"); mport != "" {
				if _, exists := hysteria2["ports"]; !exists {
					hysteria2["ports"] = mport
				}
			}

			proxies = append(proxies, hysteria2)

		case "tuic":
			urlTUIC, err := url.Parse(line)
			if err != nil {
				continue
			}
			query := urlTUIC.Query()
			tuic := make(map[string]any, 20)
			tuic["name"] = uniqueName(names, urlTUIC.Fragment)
			tuic["type"] = scheme
			tuic["server"] = urlTUIC.Hostname()
			tuic["port"] = urlTUIC.Port()
			tuic["udp"] = true
			password, v5 := urlTUIC.User.Password()
			if v5 {
				tuic["uuid"] = urlTUIC.User.Username()
				tuic["password"] = password
			} else {
				tuic["token"] = urlTUIC.User.Username()
			}
			if cc := query.Get("congestion_control"); cc != "" {
				tuic["congestion-controller"] = cc
			}
			if alpn := query.Get("alpn"); alpn != "" {
				tuic["alpn"] = strings.Split(alpn, ",")
			}
			if sni := query.Get("sni"); sni != "" {
				tuic["sni"] = sni
			}
			if query.Get("disable_sni") == "1" {
				tuic["disable-sni"] = true
			}
			if udpRelayMode := query.Get("udp_relay_mode"); udpRelayMode != "" {
				tuic["udp-relay-mode"] = udpRelayMode
			}
			proxies = append(proxies, tuic)

		case "trojan":
			urlTrojan, err := url.Parse(line)
			if err != nil {
				continue
			}
			query := urlTrojan.Query()
			name := uniqueName(names, urlTrojan.Fragment)
			trojan := make(map[string]any, 20)
			trojan["name"] = name
			trojan["type"] = scheme
			trojan["server"] = urlTrojan.Hostname()
			trojan["port"] = urlTrojan.Port()
			trojan["password"] = urlTrojan.User.Username()
			trojan["udp"] = true
			trojan["skip-cert-verify"], _ = strconv.ParseBool(query.Get("allowInsecure"))
			if sni := query.Get("sni"); sni != "" {
				trojan["sni"] = sni
			}
			if alpn := query.Get("alpn"); alpn != "" {
				trojan["alpn"] = strings.Split(alpn, ",")
			}
			// TLS / Reality
			security := strings.ToLower(query.Get("security"))
			if strings.HasSuffix(security, "tls") || security == "reality" {
				trojan["tls"] = true
			}
			if security == "reality" {
				if pbk := query.Get("pbk"); pbk != "" {
					ro := map[string]any{
						"public-key": pbk,
						"short-id":   query.Get("sid"),
					}
					// REALITY 一律保留 MLKEM 密钥共享（原因见下方 vless 分支的说明：
					// mihomo 默认删掉 MLKEM 段会导致 xray 26 服务端认证失败）
					ro["support-x25519mlkem768"] = true
					trojan["reality-opts"] = ro
				}
			}
			network := strings.ToLower(query.Get("type"))
			// 裸 TCP（""/tcp/raw）不写 network —— mihomo 的 network 里没有这些取值
			if network != "" && network != "tcp" && network != "raw" {
				trojan["network"] = network
			}
			switch network {
			case "ws", "httpupgrade":
				headers := make(map[string]any)
				wsOpts := make(map[string]any)
				headers["User-Agent"] = randUserAgent()
				// ⚠ Host 必须带上：服务端限制 Host 的 ws 入站，缺这个头会直接 404
				// （vless/vmess 分支一直有，trojan 分支漏了 —— e2e 实测 trojan+ws 只有 mihomo 挂）
				if host := query.Get("host"); host != "" {
					headers["Host"] = host
				}
				path := query.Get("path")
				// Detect early data in path (e.g. /?ed=2560)
				if idx := strings.Index(path, "?ed="); idx >= 0 {
					edStr := path[idx+4:]
					if ampIdx := strings.Index(edStr, "&"); ampIdx >= 0 {
						edStr = edStr[:ampIdx]
					}
					if _, err := strconv.Atoi(edStr); err == nil {
						wsOpts["max-early-data"] = 4096
						wsOpts["early-data-header-name"] = "Sec-WebSocket-Protocol"
					}
				}
				if path == "" {
					path = "/"
				}
				wsOpts["path"] = path
				wsOpts["headers"] = headers
				trojan["ws-opts"] = wsOpts
			case "grpc":
				grpcOpts := make(map[string]any)
				grpcOpts["grpc-service-name"] = query.Get("serviceName")
				trojan["grpc-opts"] = grpcOpts
			}
			if fingerprint := query.Get("fp"); fingerprint == "" {
				trojan["client-fingerprint"] = "chrome"
			} else {
				trojan["client-fingerprint"] = fingerprint
			}
			if pcs := query.Get("pcs"); pcs != "" {
				trojan["fingerprint"] = pcs
			}
			proxies = append(proxies, trojan)

		case "vless":
			urlVLess, err := url.Parse(line)
			if err != nil {
				continue
			}
			if decodedHost, err := tryDecodeBase64([]byte(urlVLess.Host)); err == nil {
				urlVLess.Host = string(decodedHost)
			}
			query := urlVLess.Query()
			vless := make(map[string]any, 20)
			err = handleVShareLink(names, urlVLess, scheme, vless)
			if err != nil {
				continue
			}
			if flow := query.Get("flow"); flow != "" {
				vless["flow"] = strings.ToLower(flow)
			}
			if encryption := query.Get("encryption"); encryption != "" {
				vless["encryption"] = encryption
			}
			proxies = append(proxies, vless)

		case "vmess":
			dcBuf, err := tryDecodeBase64([]byte(body))
			if err != nil {
				urlVMess, err := url.Parse(line)
				if err != nil {
					continue
				}
				query := urlVMess.Query()
				vmess := make(map[string]any, 20)
				err = handleVShareLink(names, urlVMess, scheme, vmess)
				if err != nil {
					continue
				}
				vmess["alterId"] = 0
				vmess["cipher"] = "auto"
				if encryption := query.Get("encryption"); encryption != "" {
					vmess["cipher"] = encryption
				}
				proxies = append(proxies, vmess)
				continue
			}

			jsonDc := json.NewDecoder(bytes.NewReader(dcBuf))
			values := make(map[string]any, 20)
			if jsonDc.Decode(&values) != nil {
				continue
			}
			tempName, ok := values["ps"].(string)
			if !ok {
				continue
			}
			name := uniqueName(names, tempName)
			vmess := make(map[string]any, 20)
			vmess["name"] = name
			vmess["type"] = scheme
			vmess["server"] = values["add"]
			vmess["port"] = values["port"]
			vmess["uuid"] = values["id"]
			if alterId, ok := values["aid"]; ok {
				vmess["alterId"] = alterId
			} else {
				vmess["alterId"] = 0
			}
			vmess["udp"] = true
			vmess["xudp"] = true
			vmess["tls"] = false
			vmess["skip-cert-verify"] = false
			vmess["cipher"] = "auto"
			if cipher, ok := values["scy"].(string); ok && cipher != "" {
				vmess["cipher"] = cipher
			}
			if sni, ok := values["sni"].(string); ok && sni != "" {
				vmess["servername"] = sni
			}
			network, ok := values["net"].(string)
			if ok {
				network = strings.ToLower(network)
				if values["type"] == "http" {
					network = "http"
				} else if network == "http" {
					network = "h2"
				}
				// mihomo 的 network 里没有 "tcp"/"raw"：裸 TCP 就**不写**这个字段
				// （xray 的 "raw" 与 "tcp" 都是裸 TCP，mihomo 那边省略即可）。
				if network != "" && network != "tcp" && network != "raw" {
					vmess["network"] = network
				}
			}
			tls, ok := values["tls"].(string)
			if ok {
				tls = strings.ToLower(tls)
				if strings.HasSuffix(tls, "tls") {
					vmess["tls"] = true
				}
				if alpn, ok := values["alpn"].(string); ok {
					vmess["alpn"] = strings.Split(alpn, ",")
				}
			}
			switch network {
			case "http":
				headers := make(map[string]any)
				httpOpts := make(map[string]any)
				if host, ok := values["host"].(string); ok && host != "" {
					headers["Host"] = []string{host}
				}
				httpOpts["path"] = []string{"/"}
				if path, ok := values["path"].(string); ok && path != "" {
					httpOpts["path"] = []string{path}
				}
				httpOpts["headers"] = headers
				vmess["http-opts"] = httpOpts
			case "h2":
				h2Opts := make(map[string]any)
				h2Opts["path"] = "/"
				if path, ok := values["path"].(string); ok && path != "" {
					h2Opts["path"] = path
				}
				if host, ok := values["host"].(string); ok && host != "" {
					h2Opts["host"] = []string{host}
				}
				vmess["h2-opts"] = h2Opts
			case "ws", "httpupgrade":
				headers := make(map[string]any)
				wsOpts := make(map[string]any)
				wsOpts["path"] = "/"
				if host, ok := values["host"].(string); ok && host != "" {
					headers["Host"] = host
				}
				if path, ok := values["path"].(string); ok && path != "" {
					pathURL, err := url.Parse(path)
					if err == nil {
						query := pathURL.Query()
						if earlyData := query.Get("ed"); earlyData != "" {
							if _, err := strconv.Atoi(earlyData); err == nil {
								switch network {
								case "ws":
									wsOpts["max-early-data"] = 4096
									wsOpts["early-data-header-name"] = "Sec-WebSocket-Protocol"
								case "httpupgrade":
									wsOpts["v2ray-http-upgrade-fast-open"] = true
								}
							}
						}
						if earlyDataHeader := query.Get("eh"); earlyDataHeader != "" {
							wsOpts["early-data-header-name"] = earlyDataHeader
						}
					}
					wsOpts["path"] = path
				}
				if network == "httpupgrade" {
					wsOpts["v2ray-http-upgrade"] = true
				}
				wsOpts["headers"] = headers
				vmess["ws-opts"] = wsOpts
			case "grpc":
				grpcOpts := make(map[string]any)
				grpcOpts["grpc-service-name"] = values["path"]
				vmess["grpc-opts"] = grpcOpts
			}
			proxies = append(proxies, vmess)

		// shadowsocks / shadowsocks2022：同一标准 URI 的别名。只认 "ss" 时，
		// 面板早期用 shadowsocks:// 生成的链接会被静默丢弃（整批 ss 节点丢出订阅）。
		case "ss", "shadowsocks", "shadowsocks2022":
			urlSS, err := url.Parse(line)
			if err != nil {
				continue
			}
			name := uniqueName(names, urlSS.Fragment)
			port := urlSS.Port()
			if port == "" {
				dcBuf, err := encRaw.DecodeString(urlSS.Host)
				if err != nil {
					continue
				}
				urlSS, err = url.Parse("ss://" + string(dcBuf))
				if err != nil {
					continue
				}
			}
			cipherRaw := urlSS.User.Username()
			cipher := cipherRaw
			password := ""
			var found bool
			if password, found = urlSS.User.Password(); !found {
				dcBuf, err := base64.RawURLEncoding.DecodeString(cipherRaw)
				if err != nil {
					dcBuf, _ = enc.DecodeString(cipherRaw)
				}
				cipher, password, found = strings.Cut(string(dcBuf), ":")
				if !found {
					continue
				}
			}
			ss := make(map[string]any, 10)
			ss["name"] = name
			// mihomo 的出站类型只有 "ss"（别名 shadowsocks / shadowsocks2022 不是合法
			// 类型名，照抄 scheme 会被 mihomo 判为未知类型），统一收敛成 ss。
			ss["type"] = "ss"
			ss["server"] = urlSS.Hostname()
			ss["port"] = urlSS.Port()
			ss["cipher"] = cipher
			ss["password"] = password
			ss["udp"] = true
			proxies = append(proxies, ss)

		case "socks", "socks5", "socks5h", "http", "https":
			link, err := url.Parse(line)
			if err != nil {
				continue
			}
			server := link.Hostname()
			if server == "" {
				continue
			}
			portStr := link.Port()
			if portStr == "" {
				continue
			}
			remarks := link.Fragment
			if remarks == "" {
				remarks = fmt.Sprintf("%s:%s", server, portStr)
			}
			name := uniqueName(names, remarks)
			// 面板生成的是明文 user:pass（用户名 / 密码分别转义）；也兼容
			// `socks://base64(user:pass)@` 这种历史/第三方写法。
			// 注意不能用 link.User.String()：它返回**未解码**的串，会把用户名里的
			// %40 / %23 之类原样留下 → mihomo 拿错误的用户名去认证。
			var username, password string
			username = link.User.Username()
			if p, ok := link.User.Password(); ok {
				password = p
			} else if username != "" && !strings.Contains(username, "@") {
				// 没有明文密码 → 可能整段是 base64(user:pass)
				if dec := string(DecodeBase64([]byte(username))); strings.Count(dec, ":") == 1 {
					username, password, _ = strings.Cut(dec, ":")
				}
			}
			socks := make(map[string]any, 10)
			socks["name"] = name
			socks["type"] = func() string {
				switch scheme {
				case "socks", "socks5", "socks5h":
					return "socks5"
				case "http", "https":
					return "http"
				}
				return scheme
			}()
			socks["server"] = server
			socks["port"] = portStr
			socks["username"] = username
			socks["password"] = password
			socks["skip-cert-verify"] = true
			if scheme == "https" {
				socks["tls"] = true
			}
			proxies = append(proxies, socks)
		}
	}

	if len(proxies) == 0 {
		return nil, fmt.Errorf("convert v2ray subscribe error: format invalid")
	}
	return proxies, nil
}

// handleVShareLink handles Xray VMessAEAD / VLESS share link standard.
func handleVShareLink(names map[string]int, u *url.URL, scheme string, proxy map[string]any) error {
	query := u.Query()
	proxy["name"] = uniqueName(names, u.Fragment)
	if u.Hostname() == "" {
		return fmt.Errorf("url.Hostname() is empty")
	}
	if u.Port() == "" {
		return fmt.Errorf("url.Port() is empty")
	}
	proxy["type"] = scheme
	proxy["server"] = u.Hostname()
	proxy["port"] = u.Port()
	proxy["uuid"] = u.User.Username()
	proxy["udp"] = true

	tls := strings.ToLower(query.Get("security"))
	if strings.HasSuffix(tls, "tls") || tls == "reality" {
		proxy["tls"] = true
		if fingerprint := query.Get("fp"); fingerprint == "" {
			proxy["client-fingerprint"] = "chrome"
		} else {
			proxy["client-fingerprint"] = fingerprint
		}
		if alpn := query.Get("alpn"); alpn != "" {
			proxy["alpn"] = strings.Split(alpn, ",")
		}
		if pcs := query.Get("pcs"); pcs != "" {
			proxy["fingerprint"] = pcs
		}
	}
	if sni := query.Get("sni"); sni != "" {
		proxy["servername"] = sni
	}
	if realityPublicKey := query.Get("pbk"); realityPublicKey != "" {
		ro := map[string]any{
			"public-key": realityPublicKey,
			"short-id":   query.Get("sid"),
		}
		// ⚠ mihomo 的 REALITY 必须显式保留 X25519MLKEM768 密钥共享，否则**认证失败**：
		// mihomo 默认会把 ClientHello 里的 MLKEM 段删掉（component/tls/reality.go 的
		// BuildRemovedX25519MLKEM768HandshakeState，注释写着 "for X25519MLKEM768 does not work
		// properly with the old reality server"），而 xray 26 的 REALITY 服务端按带 MLKEM 的
		// 指纹校验 → 删掉就认证不过。e2e 实测（面板自建 reality 入站）：不加这行 mihomo 报
		// `REALITY authentication failed`，加上立即可通；xray / sing-box 客户端不受影响。
		// 代价：对**很老**的 reality 服务端（不支持 MLKEM）可能反而不通 —— 本项目节点跑的是
		// xray 26.x，以实测为准。
		ro["support-x25519mlkem768"] = true
		proxy["reality-opts"] = ro
	}

	switch query.Get("packetEncoding") {
	case "none":
	case "packet":
		proxy["packet-addr"] = true
	default:
		proxy["xudp"] = true
	}

	network := strings.ToLower(query.Get("type"))
	if network == "" || network == "raw" {
		network = "tcp"
	}
	fakeType := strings.ToLower(query.Get("headerType"))
	if network == "tcp" && fakeType == "http" {
		network = "http"
	} else if network == "http" {
		network = "h2"
	}
	// mihomo 的 network 取值里**没有** "tcp"/"raw"（裸 TCP 就不写这个字段）：
	// 写了 mihomo 会当未知值忽略，部分 Clash 系客户端还会直接校验报错。
	if network != "tcp" {
		proxy["network"] = network
	}

	switch network {
	case "tcp":
	case "http":
		headers := make(map[string]any)
		httpOpts := make(map[string]any)
		httpOpts["path"] = []string{"/"}
		if host := query.Get("host"); host != "" {
			headers["Host"] = []string{host}
		}
		if method := query.Get("method"); method != "" {
			httpOpts["method"] = method
		}
		if path := query.Get("path"); path != "" {
			httpOpts["path"] = []string{path}
		}
		httpOpts["headers"] = headers
		proxy["http-opts"] = httpOpts

	case "h2":
		h2Opts := make(map[string]any)
		h2Opts["path"] = "/"
		if path := query.Get("path"); path != "" {
			h2Opts["path"] = path
		}
		if host := query.Get("host"); host != "" {
			h2Opts["host"] = []string{host}
		}
		proxy["h2-opts"] = h2Opts

	case "ws", "httpupgrade":
		headers := make(map[string]any)
		wsOpts := make(map[string]any)
		headers["User-Agent"] = randUserAgent()
		headers["Host"] = query.Get("host")
		path := query.Get("path")
		// Detect early data in path (e.g. /?ed=2560) — set max-early-data to 4096, keep path as-is
		if idx := strings.Index(path, "?ed="); idx >= 0 {
			edStr := path[idx+4:]
			if ampIdx := strings.Index(edStr, "&"); ampIdx >= 0 {
				edStr = edStr[:ampIdx]
			}
			if _, err := strconv.Atoi(edStr); err == nil {
				wsOpts["max-early-data"] = 4096
				wsOpts["early-data-header-name"] = "Sec-WebSocket-Protocol"
			}
		}
		if path == "" {
			path = "/"
		}
		wsOpts["path"] = path
		wsOpts["headers"] = headers
		if network == "httpupgrade" {
			wsOpts["v2ray-http-upgrade"] = true
		}
		// Also check for ed as separate query param
		if _, exists := wsOpts["max-early-data"]; !exists {
			if edStr := query.Get("ed"); edStr != "" {
				if _, err := strconv.Atoi(edStr); err == nil {
					wsOpts["max-early-data"] = 4096
					wsOpts["early-data-header-name"] = "Sec-WebSocket-Protocol"
				}
			}
		}
		if earlyDataHeader := query.Get("eh"); earlyDataHeader != "" {
			wsOpts["early-data-header-name"] = earlyDataHeader
		}
		proxy["ws-opts"] = wsOpts

	case "grpc":
		grpcOpts := make(map[string]any)
		grpcOpts["grpc-service-name"] = query.Get("serviceName")
		proxy["grpc-opts"] = grpcOpts

	case "xhttp":
		proxy["network"] = "xhttp"
		xhttpOpts := make(map[string]any)
		if path := query.Get("path"); path != "" {
			xhttpOpts["path"] = path
		}
		if host := query.Get("host"); host != "" {
			xhttpOpts["host"] = host
		}
		if mode := query.Get("mode"); mode != "" {
			xhttpOpts["mode"] = mode
		}
		// Parse extra JSON for xhttp advanced options
		if extraStr := query.Get("extra"); extraStr != "" {
			var extraMap map[string]any
			if err := json.Unmarshal([]byte(extraStr), &extraMap); err == nil {
				parseXHTTPExtra(extraMap, xhttpOpts)
			}
		}
		if len(xhttpOpts) > 0 {
			proxy["xhttp-opts"] = xhttpOpts
		}
	}

	return nil
}

// parseXHTTPExtra maps xray-core extra JSON fields to mihomo xhttp-opts fields.
func parseXHTTPExtra(extra map[string]any, opts map[string]any) {
	// xmux → reuse-settings
	xmuxToReuse := func(xmux map[string]any) map[string]any {
		reuse := make(map[string]any)
		setStr := func(src, dst string) {
			if v, ok := xmux[src]; ok {
				switch val := v.(type) {
				case string:
					if val != "" {
						reuse[dst] = val
					}
				case float64:
					reuse[dst] = strconv.FormatInt(int64(val), 10)
				}
			}
		}
		setStr("maxConnections", "max-connections")
		setStr("maxConcurrency", "max-concurrency")
		setStr("cMaxReuseTimes", "c-max-reuse-times")
		setStr("hMaxRequestTimes", "h-max-request-times")
		setStr("hMaxReusableSecs", "h-max-reusable-secs")
		if v, ok := xmux["hKeepAlivePeriod"].(float64); ok {
			reuse["h-keep-alive-period"] = int(v)
		}
		return reuse
	}

	setStr := func(src, dst string) {
		if v, ok := extra[src]; ok {
			if s, ok := v.(string); ok && s != "" {
				opts[dst] = s
			}
		}
	}
	setBool := func(src, dst string) {
		if v, ok := extra[src]; ok {
			if b, ok := v.(bool); ok && b {
				opts[dst] = true
			}
		}
	}
	setInt := func(src, dst string) {
		if v, ok := extra[src]; ok {
			switch val := v.(type) {
			case float64:
				if val > 0 {
					opts[dst] = int(val)
				}
			case int:
				if val > 0 {
					opts[dst] = val
				}
			}
		}
	}

	setStr("xPaddingBytes", "x-padding-bytes")
	setBool("noGRPCHeader", "no-grpc-header")
	setBool("xPaddingObfsMode", "x-padding-obfs-mode")
	setStr("xPaddingKey", "x-padding-key")
	setStr("xPaddingHeader", "x-padding-header")
	setStr("xPaddingPlacement", "x-padding-placement")
	setStr("xPaddingMethod", "x-padding-method")
	setStr("uplinkHttpMethod", "uplink-http-method")

	// session placement/key
	if v, ok := extra["sessionIDPlacement"].(string); ok && v != "" {
		opts["session-placement"] = v
	} else if v, ok := extra["sessionPlacement"].(string); ok && v != "" {
		opts["session-placement"] = v
	}
	if v, ok := extra["sessionIDKey"].(string); ok && v != "" {
		opts["session-key"] = v
	} else if v, ok := extra["sessionKey"].(string); ok && v != "" {
		opts["session-key"] = v
	}

	setStr("seqPlacement", "seq-placement")
	setStr("seqKey", "seq-key")
	setStr("uplinkDataPlacement", "uplink-data-placement")
	setStr("uplinkDataKey", "uplink-data-key")
	setInt("uplinkChunkSize", "uplink-chunk-size")
	setInt("scMaxEachPostBytes", "sc-max-each-post-bytes")
	setInt("scMinPostsIntervalMs", "sc-min-posts-interval-ms")

	// xmux → reuse-settings
	if xmuxAny, ok := extra["xmux"].(map[string]any); ok && len(xmuxAny) > 0 {
		if reuse := xmuxToReuse(xmuxAny); len(reuse) > 0 {
			opts["reuse-settings"] = reuse
		}
	}
}

func uniqueName(names map[string]int, name string) string {
	if index, ok := names[name]; ok {
		index++
		names[name] = index
		name = fmt.Sprintf("%s-%02d", name, index)
	} else {
		index = 0
		names[name] = index
	}
	return name
}

func DecodeBase64(buf []byte) []byte {
	result, err := tryDecodeBase64(buf)
	if err != nil {
		return buf
	}
	return result
}

func tryDecodeBase64(buf []byte) ([]byte, error) {
	dBuf := make([]byte, encRaw.DecodedLen(len(buf)))
	n, err := encRaw.Decode(dBuf, buf)
	if err != nil {
		n, err = enc.Decode(dBuf, buf)
		if err != nil {
			return nil, err
		}
	}
	return dBuf[:n], nil
}

func randUserAgent() string {
	agents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	}
	return agents[rand.Intn(len(agents))]
}
