package subscription

import (
	"errors"
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

// ErrUnsupportedOption 表示"该订阅选项在当前版本里根本没有执行点"，因此**拒绝写入**（fail-closed）。
//
// 它是**调用方的输入错误**（应回 HTTP 400 ✓），不是服务端故障 ✗ ——
// handler 必须用 errors.Is 判断后回 400；否则会被包成 500，让人误以为面板坏了、
// 也不会让调用方意识到"是自己传了一个未实现的选项"。
var ErrUnsupportedOption = errors.New("unsupported subscription option")

// ValidateSecurityFields 校验订阅的 security_mode / bind_mode 是否**已实现**。
//
// 返回错误即表示"这个模式我们其实没有执行" ✗ —— 落库只会让使用者误以为受保护。
// 空串按默认值处理（调用方已填默认值，这里再兜一次 ✓）。
func ValidateSecurityFields(securityMode, bindMode string) error {
	switch securityMode {
	case "", SecurityModeTokenOnly:
		// ok
	case SecurityModeTokenWithExpiry, SecurityModeTokenWithSignature, SecurityModeTokenWithBinding:
		return fmt.Errorf("%w: security_mode %q is not implemented yet; only %q is supported",
			ErrUnsupportedOption, securityMode, SecurityModeTokenOnly)
	default:
		return fmt.Errorf("%w: unknown security_mode %q", ErrUnsupportedOption, securityMode)
	}

	switch bindMode {
	case "", BindModeNone:
		// ok
	default:
		return fmt.Errorf("%w: bind_mode %q is not implemented yet; only %q is supported",
			ErrUnsupportedOption, bindMode, BindModeNone)
	}
	return nil
}

// ValidateActivationWindow 校验"激活后有效天数"（`max_active_days`）—— **该能力已决议不做**。
//
// 背景（2026-09-23 决议）：订阅有效期的**唯一事实来源是套餐那边的 `expires_at`** ✗ ——
// 购买、续费、升级、试用（`plan/trial.go`）、卡密兑换（`GiftCardCode`）、手工建客户端，
// 全部由套餐变体的 `duration_days`（默认 30）或表单到期日决定 ✓。
//
// `activated_at` + `max_active_days` 是设计文档里的**第二条时间轴**（"首次使用起算"），
// 但代码里从来没有入口：套餐流转不写、前端不暴露、业务不读（实测全库 0 条数据 ✓）。
// 也就是"声明了却不生效"的半套功能 —— 保留只会制造虚假预期（管理员以为设了天数会生效，
// 实际什么都不会发生），排障时还多一个要同时核对的到期来源。
//
// 因此与 `security_mode` 同样处理：**写入即拒绝**（fail-closed）✓ ——
// 将来真要做这条时间轴时，应先在**卡密兑换 / 套餐变体**上设计出真正的入口
// （那时它才有明确语义），再来放开这里。库列保留不删（避免迁移风险）。
func ValidateActivationWindow(maxActiveDays *int) error {
	if maxActiveDays != nil && *maxActiveDays != 0 {
		return fmt.Errorf("%w: max_active_days is not supported: subscription expiry is driven by the plan's expires_at (variant duration_days); this field has no effect, so it is rejected instead of silently stored", ErrUnsupportedOption)
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
