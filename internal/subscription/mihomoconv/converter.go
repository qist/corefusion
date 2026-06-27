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
			if auth := urlHysteria2.User.String(); auth != "" {
				hysteria2["password"] = auth
			}
			hysteria2["fingerprint"] = query.Get("pinSHA256")
			hysteria2["down"] = query.Get("down")
			hysteria2["up"] = query.Get("up")
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
			network := strings.ToLower(query.Get("type"))
			if network != "" {
				trojan["network"] = network
			}
			switch network {
			case "ws":
				headers := make(map[string]any)
				wsOpts := make(map[string]any)
				headers["User-Agent"] = randUserAgent()
				wsOpts["path"] = query.Get("path")
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
				vmess["network"] = network
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
							med, err := strconv.Atoi(earlyData)
							if err == nil {
								switch network {
								case "ws":
									wsOpts["max-early-data"] = med
									wsOpts["early-data-header-name"] = "Sec-WebSocket-Protocol"
								case "httpupgrade":
									wsOpts["v2ray-http-upgrade-fast-open"] = true
								}
								query.Del("ed")
								pathURL.RawQuery = query.Encode()
								path = pathURL.String()
							}
						}
						if earlyDataHeader := query.Get("eh"); earlyDataHeader != "" {
							wsOpts["early-data-header-name"] = earlyDataHeader
						}
					}
					wsOpts["path"] = path
				}
				wsOpts["headers"] = headers
				vmess["ws-opts"] = wsOpts
			case "grpc":
				grpcOpts := make(map[string]any)
				grpcOpts["grpc-service-name"] = values["path"]
				vmess["grpc-opts"] = grpcOpts
			}
			proxies = append(proxies, vmess)

		case "ss":
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
			ss["type"] = scheme
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
			encodeStr := link.User.String()
			var username, password string
			if encodeStr != "" {
				decodeStr := string(DecodeBase64([]byte(encodeStr)))
				splitStr := strings.Split(decodeStr, ":")
				username = splitStr[0]
				if len(splitStr) == 2 {
					password = splitStr[1]
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
		proxy["reality-opts"] = map[string]any{
			"public-key": realityPublicKey,
			"short-id":   query.Get("sid"),
		}
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
	proxy["network"] = network

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
		wsOpts["path"] = query.Get("path")
		wsOpts["headers"] = headers
		if earlyData := query.Get("ed"); earlyData != "" {
			med, err := strconv.Atoi(earlyData)
			if err != nil {
				return fmt.Errorf("bad WebSocket max early data size: %v", err)
			}
			switch network {
			case "ws":
				wsOpts["max-early-data"] = med
				wsOpts["early-data-header-name"] = "Sec-WebSocket-Protocol"
			case "httpupgrade":
				wsOpts["v2ray-http-upgrade-fast-open"] = true
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
		proxy["xhttp-opts"] = xhttpOpts
	}

	return nil
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
