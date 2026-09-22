package subscription

import (
	"fmt"
	"time"

	"corefusion/internal/domain"
)

// Subscription represents a server-side subscription credential.
type Subscription struct {
	ID                      int64             `json:"id"`
	ClientID                int64             `json:"client_id"`
	Format                  string            `json:"format"`
	Token                   string            `json:"token"`
	TokenVersion            int               `json:"token_version"`
	Status                  string            `json:"status"`
	SecurityMode            string            `json:"security_mode"`
	Signature               string            `json:"signature,omitempty"`
	BindMode                string            `json:"bind_mode"`
	BindValue               string            `json:"bind_value,omitempty"`
	ActivatedAt             *time.Time        `json:"activated_at,omitempty"`
	MaxActiveDays           *int              `json:"max_active_days,omitempty"`
	ExpiresAt               *time.Time        `json:"expires_at,omitempty"`
	QuotaBytes              int64             `json:"quota_bytes"`
	UsedBytesCache          int64             `json:"used_bytes_cache"`
	QuotaResetCycle         string            `json:"quota_reset_cycle"`
	QuotaResetDay           *int              `json:"quota_reset_day,omitempty"`
	LastResetAt             *time.Time        `json:"last_reset_at,omitempty"`
	NextResetAt             *time.Time        `json:"next_reset_at,omitempty"`
	SuspendOnQuotaExhausted bool              `json:"suspend_on_quota_exhausted"`
	QuotaState              domain.QuotaState `json:"quota_state"`
	SuspendedAt             *time.Time        `json:"suspended_at,omitempty"`
	RevokedAt               *time.Time        `json:"revoked_at,omitempty"`
	LastAccessAt            *time.Time        `json:"last_access_at,omitempty"`
	AccessCount             int64             `json:"access_count"`
	LastIP                  string            `json:"last_ip,omitempty"`
	LastUserAgent           string            `json:"last_user_agent,omitempty"`
	CreatedAt               time.Time         `json:"created_at"`
	UpdatedAt               time.Time         `json:"updated_at"`
}

// Subscription status constants.
const (
	StatusActive   = "active"
	StatusRevoked  = "revoked"
	StatusExpired  = "expired"
	StatusDisabled = "disabled"
)

// SecurityMode constants.
//
// ⚠ **只有 `token_only` 是真正实现了的** —— 其余三个是"设计文档写了、代码里没有任何执行点"的漂移
// （安全审计 v5：`SecurityMode` / `BindMode` / `Signature` 字段从未被校验）。也就是说：
// 把它们存进库**不会带来任何保护** ✗，而管理员/调用方会以为订阅被签名/绑定校验保护了 ✗ ——
// 这比"没这个功能"更危险（虚假的安全感）。
//
// 因此 `ValidateSecurityFields` 在写入时**直接拒绝**未实现的取值（失败关闭 ✓）：
// 要么真的实现（需要单独设计协议兼容策略），要么就别让人以为已经生效。
const (
	SecurityModeTokenOnly          = "token_only"
	SecurityModeTokenWithExpiry    = "token_with_expiry"    // 未实现（写入即拒绝）
	SecurityModeTokenWithSignature = "token_with_signature" // 未实现（写入即拒绝）
	SecurityModeTokenWithBinding   = "token_with_binding"   // 未实现（写入即拒绝）
)

// BindModeNone 是唯一已实现的绑定模式（订阅 token 不做 IP/设备绑定校验）。
const BindModeNone = "none"

// ValidateSecurityFields 校验订阅的 security_mode / bind_mode 是否**已实现**。
//
// 返回错误即表示"这个模式我们其实没有执行" ✗ —— 落库只会让使用者误以为受保护。
// 空串按默认值处理（调用方已填默认值，这里再兜一次 ✓）。
func ValidateSecurityFields(securityMode, bindMode string) error {
	switch securityMode {
	case "", SecurityModeTokenOnly:
		// ok
	case SecurityModeTokenWithExpiry, SecurityModeTokenWithSignature, SecurityModeTokenWithBinding:
		return fmt.Errorf("security_mode %q is not implemented yet; only %q is supported",
			securityMode, SecurityModeTokenOnly)
	default:
		return fmt.Errorf("unknown security_mode %q", securityMode)
	}

	switch bindMode {
	case "", BindModeNone:
		// ok
	default:
		return fmt.Errorf("bind_mode %q is not implemented yet; only %q is supported",
			bindMode, BindModeNone)
	}
	return nil
}

// ServerShareProfile represents a server-side node share profile.
type ServerShareProfile struct {
	ID                  int64      `json:"id"`
	Name                string     `json:"name"`
	ClientID            *int64     `json:"client_id,omitempty"`
	TargetClientType    string     `json:"target_client_type"`
	OutputFormat        string     `json:"output_format"`
	DeliveryMode        string     `json:"delivery_mode"`
	TemplateID          *int64     `json:"template_id,omitempty"`
	RouteTemplateID     *int64     `json:"route_template_id,omitempty"`
	DefaultOutboundMode string     `json:"default_outbound_mode"`
	SecurityMode        string     `json:"security_mode"`
	BindMode            string     `json:"bind_mode"`
	IncludeRuleURLs     bool       `json:"include_rule_urls"`
	IncludeCustomRules  bool       `json:"include_custom_rules"`
	HTTPSRequired       bool       `json:"https_required"`
	Status              string     `json:"status"`
	LastPreviewAt       *time.Time `json:"last_preview_at,omitempty"`
	LastPublishedAt     *time.Time `json:"last_published_at,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}
