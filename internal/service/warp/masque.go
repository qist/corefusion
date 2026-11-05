package warp

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	crand "crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// MASQUE 注册与 WireGuard 注册并行存在：旧路径（Register）完全不动，
// 新路径（RegisterMasque）走 WARP 要求的两步流程。
//
// 背景（见 6Kmfi6HP/warp-reg-gw 的 internal/registration）：
//  1. POST /reg 用一次性 Curve25519 公钥创建设备，拿 id + token；
//  2. PATCH /reg/{id} 上传 ECDSA P-256 公钥并切到 tunnel_type=masque；
//     边缘地址、端口列表、边缘公钥只在第二步响应里出现。
//
// 响应是顶层对象，没有 {"result":{}} 外壳（套壳解析会拿不到 id）。
const (
	masqueAPIBase       = "https://api.cloudflareclient.com/v0a4471"
	masqueClientVersion = "a-6.35-4471"
	masqueUserAgent     = "WARP for Android"

	masqueWireGuard = "wireguard"
	masqueTunnel    = "masque"

	// masqueConnectSNI 是 WARP 官方客户端使用的 SNI（出站复用）。
	masqueConnectSNI = "consumer-masque.cloudflareclient.com"
)

// masqueRegisterRequest 是 POST /reg 的请求体（第一步，仍用 WireGuard 形态建号）。
type masqueRegisterRequest struct {
	Key         string `json:"key"`
	InstallID   string `json:"install_id"`
	FcmToken    string `json:"fcm_token"`
	Tos         string `json:"tos"`
	Model       string `json:"model"`
	Serial      string `json:"serial_number"`
	KeyType     string `json:"key_type"`
	TunnelType  string `json:"tunnel_type"`
	Locale      string `json:"locale"`
	WarpEnabled bool   `json:"warp_enabled"`
}

// masqueEnrollRequest 是 PATCH /reg/{id} 的请求体（第二步，登记 P-256 公钥）。
type masqueEnrollRequest struct {
	Key        string `json:"key"`
	KeyType    string `json:"key_type"`
	TunnelType string `json:"tunnel_type"`
}

// masqueRegisterResponse 覆盖两步响应的并集（字段缺失时为零值）。
type masqueRegisterResponse struct {
	ID         string `json:"id"`
	Token      string `json:"token"`
	KeyType    string `json:"key_type"`
	TunnelType string `json:"tunnel_type"`
	Account    struct {
		ID          string `json:"id"`
		AccountType string `json:"account_type"`
		License     string `json:"license"`
	} `json:"account"`
	Config struct {
		ClientID string `json:"client_id"`
		Peers    []struct {
			PublicKey string `json:"public_key"`
			Endpoint  struct {
				V4    string `json:"v4"`
				V6    string `json:"v6"`
				Host  string `json:"host"`
				Ports []int  `json:"ports"`
			} `json:"endpoint"`
		} `json:"peers"`
		Interface struct {
			Addresses struct {
				V4 string `json:"v4"`
				V6 string `json:"v6"`
			} `json:"addresses"`
		} `json:"interface"`
	} `json:"config"`
}

// RegisterMasque 注册一台 MASQUE 设备并落库（status=registered）。
//
// 旧数据不迁移：KeyType/TunnelType 为空的历史记录继续按 WireGuard 使用。
func (s *Service) RegisterMasque(serverID int64, coreType, remark string) (*WarpStatus, error) {
	reg, err := registerMasqueDevice()
	if err != nil {
		return nil, err
	}

	cfg := &entity.WarpConfig{
		ServerID:       serverID,
		CoreType:       coreType,
		Remark:         remark,
		DeviceID:       reg.DeviceID,
		Token:          reg.Token,
		PrivateKey:     reg.PrivateKey,
		PublicKey:      reg.PublicKey,
		AddressIPv4:    reg.AssignedIPv4,
		AddressIPv6:    reg.AssignedIPv6,
		Endpoint:       reg.Endpoint,
		KeyType:        "secp256r1",
		TunnelType:     masqueTunnel,
		EndpointPorts:  joinPorts(reg.EndpointPorts),
		AccountID:      reg.AccountID,
		AccountType:    reg.AccountType,
		LicenseKey:     reg.LicenseKey,
		RegistrationAt: time.Now(),
		Status:         "registered",
	}
	if err := s.repo.Create(nil, cfg); err != nil {
		return nil, fmt.Errorf("save masque config: %w", err)
	}
	// Create 会就地加密 private_key/token，返回值必须重新读取（解密）后构造，
	// 否则前端拿到的是 `enc:...`，直接拿去做出站会认证失败。
	if saved, err := s.repo.GetByID(nil, cfg.ID); err == nil && saved != nil {
		return s.configToStatus(saved), nil
	}
	return s.configToStatus(cfg), nil
}

// masqueRegistration 是两步注册的中间结果。
type masqueRegistration struct {
	DeviceID      string
	Token         string
	PrivateKey    string // base64(SEC1 ECPrivateKey DER)
	PublicKey     string // base64(PKIX SubjectPublicKeyInfo DER)，边缘公钥
	Endpoint      string
	EndpointV4    string
	EndpointV6    string
	EndpointPorts []int
	AssignedIPv4  string
	AssignedIPv6  string
	AccountID     string
	AccountType   string
	LicenseKey    string
}

// registerMasqueDevice 执行 WARP 的两步注册，不落库（便于测试与导入）。
func registerMasqueDevice() (*masqueRegistration, error) {
	httpClient := &http.Client{Timeout: 30 * time.Second}

	// ---- 第一步：曲线 Curve25519 公钥创建设备 ----
	_, wgPub, err := generateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("generate wireguard key: %w", err)
	}
	step1 := masqueRegisterRequest{
		Key:         wgPub,
		InstallID:   randomString(22),
		FcmToken:    randomString(64) + ":APA91b" + randomString(80),
		Tos:         time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
		Model:       "PC",
		Serial:      randomHex(8),
		KeyType:     "curve25519",
		TunnelType:  masqueWireGuard,
		Locale:      "en_US",
		WarpEnabled: true,
	}
	body1, _ := json.Marshal(step1)
	resp1, err := masqueDo(httpClient, http.MethodPost, masqueAPIBase+"/reg", "", body1)
	if err != nil {
		return nil, err
	}
	var created masqueRegisterResponse
	if err := json.Unmarshal(resp1, &created); err != nil {
		return nil, fmt.Errorf("parse register response: %w, body=%s", err, truncateBody(resp1))
	}
	if created.ID == "" || created.Token == "" {
		return nil, fmt.Errorf("register response missing id/token: %s", truncateBody(resp1))
	}

	// ---- 第二步：登记 P-256 公钥，切到 masque ----
	priv, err := ecdsa.GenerateKey(elliptic.P256(), crand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate p256 key: %w", err)
	}
	privDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return nil, fmt.Errorf("marshal ec private key: %w", err)
	}
	pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("marshal pkix public key: %w", err)
	}
	step2 := masqueEnrollRequest{
		Key:        base64.StdEncoding.EncodeToString(pubDER),
		KeyType:    "secp256r1",
		TunnelType: masqueTunnel,
	}
	body2, _ := json.Marshal(step2)
	resp2, err := masqueDo(httpClient, http.MethodPatch, masqueAPIBase+"/reg/"+created.ID, created.Token, body2)
	if err != nil {
		return nil, err
	}
	var enrolled masqueRegisterResponse
	if err := json.Unmarshal(resp2, &enrolled); err != nil {
		return nil, fmt.Errorf("parse enroll response: %w, body=%s", err, truncateBody(resp2))
	}

	out := &masqueRegistration{
		DeviceID:     firstNonEmpty(enrolled.ID, created.ID),
		Token:        firstNonEmpty(enrolled.Token, created.Token),
		PrivateKey:   base64.StdEncoding.EncodeToString(privDER),
		AssignedIPv4: enrolled.Config.Interface.Addresses.V4,
		AssignedIPv6: enrolled.Config.Interface.Addresses.V6,
		AccountID:    firstNonEmpty(enrolled.Account.ID, created.Account.ID),
		AccountType:  firstNonEmpty(enrolled.Account.AccountType, created.Account.AccountType),
		LicenseKey:   firstNonEmpty(enrolled.Account.License, created.Account.License),
	}
	if len(enrolled.Config.Peers) > 0 {
		peer := enrolled.Config.Peers[0]
		out.EndpointPorts = peer.Endpoint.Ports
		v4, v6 := normalizeEndpoint(peer.Endpoint.V4), normalizeEndpoint(peer.Endpoint.V6)
		// 部分响应只给 host 字段（形如 "[2606:...]:443" 或 "162.159.198.2:443"）。
		if v4 == "" && v6 == "" {
			host := normalizeEndpoint(peer.Endpoint.Host)
			if host != "" {
				if strings.Contains(host, ":") {
					v6 = host
				} else {
					v4 = host
				}
			}
		}
		out.Endpoint = firstNonEmpty(v4, v6)
		out.EndpointV4, out.EndpointV6 = v4, v6
		if peer.PublicKey != "" {
			pk, err := pemPublicKeyToBase64(peer.PublicKey)
			if err != nil {
				return nil, fmt.Errorf("parse edge public key: %w", err)
			}
			out.PublicKey = pk
		}
	}
	if out.Endpoint == "" {
		return nil, fmt.Errorf("enroll response has no endpoint: %s", truncateBody(resp2))
	}
	if len(out.EndpointPorts) == 0 {
		out.EndpointPorts = []int{443}
	}
	if out.PrivateKey == "" || out.PublicKey == "" {
		return nil, fmt.Errorf("enroll response incomplete (private/public key missing)")
	}
	return out, nil
}

// masqueDo 发请求并检查 2xx；返回响应体。
func masqueDo(client *http.Client, method, url, token string, body []byte) ([]byte, error) {
	req, err := http.NewRequest(method, url, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", masqueUserAgent)
	req.Header.Set("CF-Client-Version", masqueClientVersion)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", method, url, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s %s: HTTP %d - %s", method, url, resp.StatusCode, truncateBody(data))
	}
	return data, nil
}

// normalizeEndpoint 去掉端点里的端口与 IPv6 方括号，返回纯地址。
func normalizeEndpoint(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(v); err == nil {
		return host
	}
	return strings.Trim(v, "[]")
}

// pemPublicKeyToBase64 把 API 返回的 PEM 公钥转成 base64(PKIX DER)。
// 也接受直接给 base64 DER 的情况。
func pemPublicKeyToBase64(s string) (string, error) {
	s = strings.TrimSpace(s)
	if block, _ := pem.Decode([]byte(s)); block != nil {
		if _, err := x509.ParsePKIXPublicKey(block.Bytes); err != nil {
			return "", err
		}
		return base64.StdEncoding.EncodeToString(block.Bytes), nil
	}
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return "", err
	}
	if _, err := x509.ParsePKIXPublicKey(raw); err != nil {
		return "", err
	}
	return s, nil
}

func joinPorts(ports []int) string {
	if len(ports) == 0 {
		return ""
	}
	parts := make([]string, 0, len(ports))
	for _, p := range ports {
		if p > 0 {
			parts = append(parts, fmt.Sprintf("%d", p))
		}
	}
	return strings.Join(parts, ",")
}

// splitPorts 解析 EndpointPorts 列（逗号分隔）。
func splitPorts(s string) []int {
	var out []int
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		var n int
		if _, err := fmt.Sscanf(p, "%d", &n); err == nil && n > 0 && n < 65536 {
			out = append(out, n)
		}
	}
	return out
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := crand.Read(b); err != nil {
		return ""
	}
	return fmt.Sprintf("%x", b)
}

func truncateBody(b []byte) string {
	const max = 500
	if len(b) <= max {
		return string(b)
	}
	return string(b[:max]) + "..."
}

// ---------------------------------------------------------------------------
// 出站生成
// ---------------------------------------------------------------------------

// MasqueOutboundInput 是「用已注册的 MASQUE 设备直接创建出站」的入参。
type MasqueOutboundInput struct {
	Remark string `json:"remark"`
	Tag    string `json:"tag"`
	// ALPN 为空时按 h3（QUIC）；填 h2 走 TCP 回退。
	ALPN       []string `json:"alpn"`
	ServerName string   `json:"server_name"`
	// Port 为空时用注册返回的端口列表首个。
	Port int `json:"port"`
	// Enabled 默认 true（由 handler 兜底）。
	Enabled bool `json:"enabled"`
}

// MasqueOutboundResult 返回创建好的出站，前端据此提示路由 tag。
type MasqueOutboundResult struct {
	ID     int64                  `json:"id"`
	Tag    string                 `json:"tag"`
	Remark string                 `json:"remark"`
	Config map[string]interface{} `json:"config"`
}

// BuildMasqueOutboundConfig 用 WARP 注册数据生成 xray masque 出站配置。
func BuildMasqueOutboundConfig(cfg *entity.WarpConfig, in MasqueOutboundInput) (string, string, error) {
	if cfg == nil {
		return "", "", fmt.Errorf("warp config not found")
	}
	if !strings.EqualFold(cfg.TunnelType, masqueTunnel) {
		return "", "", fmt.Errorf("该 WARP 记录不是 MASQUE 设备（tunnel_type=%q），请重新注册 MASQUE", cfg.TunnelType)
	}
	if cfg.PrivateKey == "" || cfg.PublicKey == "" || cfg.Endpoint == "" {
		return "", "", fmt.Errorf("WARP 记录缺少 MASQUE 凭据（private/public key 或 endpoint）")
	}

	port := in.Port
	if port <= 0 {
		if ports := splitPorts(cfg.EndpointPorts); len(ports) > 0 {
			port = ports[0]
		}
	}
	if port <= 0 {
		port = 443
	}
	alpn := in.ALPN
	if len(alpn) == 0 {
		alpn = []string{"h3"}
	}
	sni := in.ServerName
	if sni == "" {
		sni = masqueConnectSNI
	}

	tag := strings.TrimSpace(in.Tag)
	if tag == "" {
		tag = fmt.Sprintf("warp-masque-%d", cfg.ID)
	}

	addresses := make([]string, 0, 2)
	if cfg.AddressIPv4 != "" {
		addresses = append(addresses, ensureCIDR(cfg.AddressIPv4, 32))
	}
	if cfg.AddressIPv6 != "" {
		addresses = append(addresses, ensureCIDR(cfg.AddressIPv6, 128))
	}

	// WARP 模式下 host/path 有官方默认值（cloudflareaccess.com / "/"）：
	// 显式写 RFC 9484 的模板路径会被边缘当作普通请求 404，导致隧道建立失败。
	path := "/"

	ob := map[string]interface{}{
		"tag":      tag,
		"protocol": "masque",
		"settings": map[string]interface{}{
			"address":   cfg.Endpoint,
			"port":      port,
			"remoteDNS": []string{"1.1.1.1", "1.0.0.1", "2606:4700:4700::1111", "2606:4700:4700::1001"},
		},
		"streamSettings": map[string]interface{}{
			"network": "masque",
			"masqueSettings": map[string]interface{}{
				"path": path,
				"warp": map[string]interface{}{
					"privateKey": cfg.PrivateKey,
					"publicKey":  cfg.PublicKey,
					"address":    addresses,
				},
			},
			"security": "tls",
			"tlsSettings": map[string]interface{}{
				"serverName": sni,
				"alpn":       alpn,
			},
		},
	}
	data, err := json.Marshal(ob)
	if err != nil {
		return "", "", err
	}
	return string(data), tag, nil
}

// CreateMasqueOutbound 用已注册的 MASQUE 设备直接创建一条 xray 出站。
//
// 旧 WireGuard 记录不参与：非 masque 的 WARP 记录会明确报错，调用方继续用
// 原有「生成 WireGuard 配置」路径。
func (s *Service) CreateMasqueOutbound(serverID, warpID int64, in MasqueOutboundInput) (*MasqueOutboundResult, error) {
	if s.outboundRepo == nil {
		return nil, fmt.Errorf("outbound repository not configured")
	}
	cfg, err := s.repo.GetByID(nil, warpID)
	if err != nil || cfg == nil {
		return nil, fmt.Errorf("warp config %d not found", warpID)
	}
	if cfg.ServerID != serverID {
		return nil, fmt.Errorf("warp config %d belongs to server %d", warpID, cfg.ServerID)
	}
	if cfg.CoreType != "" && !strings.EqualFold(cfg.CoreType, "xray") {
		return nil, fmt.Errorf("masque 出站只能用于 xray（该记录 core_type=%s）", cfg.CoreType)
	}

	configJSON, tag, err := BuildMasqueOutboundConfig(cfg, in)
	if err != nil {
		return nil, err
	}

	// tag 冲突时追加序号，保证唯一（outbound_configs 上有 server+tag 唯一索引）。
	tag = uniqueOutboundTag(s.outboundRepo, serverID, tag)

	var configMap map[string]interface{}
	if err := json.Unmarshal([]byte(configJSON), &configMap); err != nil {
		return nil, err
	}
	// 出站里的 tag 以最终唯一 tag 为准。
	configMap["tag"] = tag

	remark := strings.TrimSpace(in.Remark)
	if remark == "" {
		remark = "WARP MASQUE"
	}
	maxOrder, _ := s.outboundRepo.GetMaxSortOrder(nil, serverID, "xray")
	ob := &entity.OutboundConfig{
		ServerID:   serverID,
		CoreType:   "xray",
		Remark:     remark,
		Protocol:   "masque",
		Tag:        tag,
		ConfigJSON: string(mustJSON(configMap)),
		Enabled:    true,
		IsDefault:  false,
		SortOrder:  maxOrder + 1,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	if err := s.outboundRepo.Create(nil, ob); err != nil {
		return nil, fmt.Errorf("create masque outbound: %w", err)
	}
	return &MasqueOutboundResult{
		ID:     ob.ID,
		Tag:    tag,
		Remark: remark,
		Config: configMap,
	}, nil
}

// uniqueOutboundTag 在 tag 已存在时追加 -2、-3 …。
func uniqueOutboundTag(repo *repository.OutboundConfigRepository, serverID int64, tag string) string {
	if repo == nil {
		return tag
	}
	if _, err := repo.GetByTag(nil, serverID, tag); err != nil {
		return tag
	}
	for i := 2; i < 100; i++ {
		candidate := fmt.Sprintf("%s-%d", tag, i)
		if _, err := repo.GetByTag(nil, serverID, candidate); err != nil {
			return candidate
		}
	}
	return fmt.Sprintf("%s-%d", tag, time.Now().Unix())
}

func mustJSON(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}

// ensureCIDR 给裸地址补前缀长度（WARP 分配的是 /32、/128）。
func ensureCIDR(addr string, bits int) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ""
	}
	if strings.Contains(addr, "/") {
		return addr
	}
	return fmt.Sprintf("%s/%d", addr, bits)
}
