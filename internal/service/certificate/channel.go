package certificate

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"corefusion/internal/model/entity"
	cert "corefusion/internal/runtime/cert"
	secretcrypto "corefusion/internal/security/crypto"
	"corefusion/internal/store/repository"
)

// SupportedDNSProviders returns the supported DNS providers and their required env vars.
func SupportedDNSProviders() map[string][]string {
	return map[string][]string{
		// 直接按**用户脚本里的写法**给字段 ✓（Cloudflare 文档/脚本就是 `CF_Token` + `CF_Account_ID` ✓）。
		// 服务端会把这些别名归一成 lego 认的 `CF_DNS_API_TOKEN` ✓（见 runtime/cert/dns_env.go ✓），
		// 所以标准名（`CF_DNS_API_TOKEN` ✓）与老式全局 Key（`CLOUDFLARE_API_KEY`+`CLOUDFLARE_EMAIL` ✓）
		// 也都能用 ✓。
		"cloudflare": {"CF_Token", "CF_Account_ID"},
		"alidns":     {"ALICLOUD_ACCESS_KEY", "ALICLOUD_SECRET_KEY"},
		"huawei":     {"HWC_ACCESS_KEY", "HWC_SECRET_KEY", "HWC_REGION", "HWC_PROJECT_ID"},
		"dnspod":     {"DNSPOD_API_KEY"},
		"route53":    {"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_REGION"},
		"google":     {"GCE_PROJECT", "GCE_SERVICE_ACCOUNT_FILE"},
		"godaddy":    {"GODADDY_API_KEY", "GODADDY_API_SECRET"},
		"namecheap":  {"NAMECHEAP_API_USER", "NAMECHEAP_API_KEY"},
	}
}

// DNSChannelService handles DNS channel business logic.
type DNSChannelService struct {
	repo *repository.DNSChannelRepository
}

// NewDNSChannelService creates a new DNS channel service.
func NewDNSChannelService(repo *repository.DNSChannelRepository) *DNSChannelService {
	return &DNSChannelService{repo: repo}
}

func (s *DNSChannelService) Create(ctx context.Context, name, provider string, envVars map[string]string) (*entity.DNSChannel, error) {
	encoded, err := encodeEnvVars(envVars)
	if err != nil {
		return nil, err
	}

	ch := &entity.DNSChannel{
		Name:     name,
		Provider: provider,
		EnvVars:  encoded,
		Status:   "active",
	}
	if err := s.repo.Create(ctx, ch); err != nil {
		return nil, err
	}
	return ch, nil
}

func (s *DNSChannelService) GetByID(ctx context.Context, id int64) (*entity.DNSChannel, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *DNSChannelService) List(ctx context.Context) ([]map[string]interface{}, error) {
	channels, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}

	var result []map[string]interface{}
	for _, ch := range channels {
		item := map[string]interface{}{
			"id":         ch.ID,
			"name":       ch.Name,
			"provider":   ch.Provider,
			"status":     ch.Status,
			"env_vars":   maskEnvVars(ch.EnvVars),
			"created_at": ch.CreatedAt,
			"updated_at": ch.UpdatedAt,
		}
		result = append(result, item)
	}
	return result, nil
}

func (s *DNSChannelService) Update(ctx context.Context, id int64, name, provider string, envVars map[string]string) (*entity.DNSChannel, error) {
	ch, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	ch.Name = name
	providerChanged := provider != "" && provider != ch.Provider
	ch.Provider = provider

	// ⚠ 前端编辑渠道时**不回填凭据**（列表接口只回掩码 ✓，见 maskEnvVars ✓），
	// 于是"只改名字"的提交会带着空 env_vars 进来 ✗ —— 旧实现 `envVars != nil` 判定
	// 对空 map 也成立 → 会把已配好的凭据**静默清空** ✗（改名即失效 ✗）。故：
	//   ① 空提交（且 provider 没换）→ 凭据原样保留 ✓；
	//   ② 非空提交 → 只覆盖"真正填了值的键" ✓，留空的键保留旧值 ✓；
	//   ③ 换了 provider → 旧凭据字段对不上，不合并（避免残留无用键 ✓）。
	switch {
	case len(envVars) > 0:
		merged := envVars
		if !providerChanged {
			if old, derr := decodeEnvVars(ch.EnvVars); derr == nil {
				merged = make(map[string]string, len(old)+len(envVars))
				for k, v := range old {
					merged[k] = v
				}
				for k, v := range envVars {
					if strings.TrimSpace(v) == "" {
						continue // 留空 = 不改这一项 ✓
					}
					merged[k] = v
				}
			}
		}
		encoded, eerr := encodeEnvVars(merged)
		if eerr != nil {
			return nil, eerr
		}
		ch.EnvVars = encoded
	case providerChanged:
		ch.EnvVars = ""
	}

	if err := s.repo.Update(ctx, ch); err != nil {
		return nil, err
	}
	return ch, nil
}

func (s *DNSChannelService) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}

// GetEnvVars returns the decoded env vars for a DNS channel.
func (s *DNSChannelService) GetEnvVars(ctx context.Context, id int64) (map[string]string, error) {
	ch, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return decodeEnvVars(ch.EnvVars)
}

// GetEnvVarsRaw returns the raw decoded env vars string.
// ResolveDNSChannel 按通道 ID 解析成 dns-01 提供者配置 ✓（**凭据在这一步被解密** ✗→✓，调用方不得打印 ✗）。
//
// ⚠ 这是**唯一**的一条解析路径 ✗：签发 / 手动续期 / 自动续期都走它 ✓。
// 以前各调用方自己解析、甚至干脆不解析 ✗（真机 bug 的成因 ✓）→
// 口径一散就会出现"签发时按 dns-01、续期时退化成 http-01"的错位 ✗。
func (s *DNSChannelService) ResolveDNSChannel(ctx context.Context, id int64) (*cert.DNSProviderConfig, error) {
	if id <= 0 {
		return nil, fmt.Errorf("invalid dns channel id: %d", id)
	}
	ch, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("dns channel %d not found: %w", id, err)
	}
	vars, err := s.GetEnvVars(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("dns channel %d credentials: %w", id, err)
	}
	return &cert.DNSProviderConfig{Provider: cert.DNSProvider(ch.Provider), EnvVars: vars}, nil
}

func (s *DNSChannelService) GetEnvVarsRaw(encoded string) (map[string]string, error) {
	return decodeEnvVars(encoded)
}

// encodeEnvVars 把凭据 map 编码成**加密串**再落库 ✓。
//
// 格式：`enc:<base64 nonce>:<base64 ciphertext>`（明文是 base64(JSON) ✓），
// 由 `internal/security/crypto` 的 AES-256-GCM 提供 ✓ —— 与**充值卡**（charge 模块 ✓）、
// CA 私钥共用同一把密钥 ✓（`CF_SECRET_ENCRYPTION_KEY`，未设置时按 hostname 派生 ✓）。
//
// 为什么必须先 base64 再加密 ✗：JSON 里有换行/引号的字段（GCP 的 service account JSON ✓）
// 直接进 `enc:` 串没问题 ✓，但 base64 让"旧格式"与"新格式"在**同一列**里同构 ✓，
// 解码侧只需一次 `DecryptSecret` 就能同时吃下新旧两种数据 ✓（见 decodeEnvVars ✓）。
func encodeEnvVars(vars map[string]string) (string, error) {
	envJSON, err := json.Marshal(vars)
	if err != nil {
		return "", fmt.Errorf("encode env vars: %w", err)
	}
	enc, err := secretcrypto.EncryptSecret(base64.StdEncoding.EncodeToString(envJSON))
	if err != nil {
		return "", fmt.Errorf("encrypt env vars: %w", err)
	}
	return enc, nil
}

// decodeEnvVars 解出凭据 map ✓。
//
// 向后兼容 ✗：历史数据是**裸 base64(JSON)**（无 `enc:` 前缀 ✓）——
// `DecryptSecret` 对不带前缀的输入**原样返回** ✓，所以老渠道不用迁移就能继续用 ✓，
// 在被 Update 一次之后会自动换成加密格式 ✓。
func decodeEnvVars(encoded string) (map[string]string, error) {
	if encoded == "" {
		return nil, fmt.Errorf("no env vars configured")
	}
	plain, err := secretcrypto.DecryptSecret(encoded)
	if err != nil {
		return nil, fmt.Errorf("decrypt env vars: %w", err)
	}
	decoded, err := base64.StdEncoding.DecodeString(plain)
	if err != nil {
		return nil, fmt.Errorf("decode env vars: %w", err)
	}
	var vars map[string]string
	if err := json.Unmarshal(decoded, &vars); err != nil {
		return nil, fmt.Errorf("parse env vars: %w", err)
	}
	return vars, nil
}

// maskEnvVars masks the values of env vars for display.
func maskEnvVars(encoded string) map[string]string {
	vars, err := decodeEnvVars(encoded)
	if err != nil {
		return nil
	}
	masked := make(map[string]string)
	for k, v := range vars {
		if len(v) <= 8 {
			masked[k] = "***"
		} else {
			masked[k] = v[:4] + "***" + v[len(v)-4:]
		}
	}
	return masked
}

// GetProviderFields returns the required env var fields for a provider.
func GetProviderFields(provider string) []string {
	providers := SupportedDNSProviders()
	if fields, ok := providers[provider]; ok {
		return fields
	}
	return nil
}

// ValidateProvider validates that a provider is supported.
func ValidateProvider(provider string) bool {
	providers := SupportedDNSProviders()
	_, ok := providers[provider]
	return ok
}

// ValidateEnvVars validates that all required env vars are present.
func ValidateEnvVars(provider string, envVars map[string]string) error {
	required := GetProviderFields(provider)
	if required == nil {
		return fmt.Errorf("unsupported provider: %s", provider)
	}
	var missing []string
	for _, key := range required {
		if _, ok := envVars[key]; !ok {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required env vars: %s", strings.Join(missing, ", "))
	}
	return nil
}
