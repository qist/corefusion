package certificate

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"corefusion/internal/model/entity"
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
	// Encode env vars to base64
	envJSON, _ := json.Marshal(envVars)
	encoded := base64.StdEncoding.EncodeToString(envJSON)

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
	ch.Provider = provider
	if envVars != nil {
		envJSON, _ := json.Marshal(envVars)
		ch.EnvVars = base64.StdEncoding.EncodeToString(envJSON)
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
func (s *DNSChannelService) GetEnvVarsRaw(encoded string) (map[string]string, error) {
	return decodeEnvVars(encoded)
}

func decodeEnvVars(encoded string) (map[string]string, error) {
	if encoded == "" {
		return nil, fmt.Errorf("no env vars configured")
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
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
