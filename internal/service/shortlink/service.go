package shortlink

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"corefusion/internal/model/entity"
	"corefusion/internal/security/crypto"
	"corefusion/internal/store/repository"
	"corefusion/internal/subscription/formatter"
)

const (
	// SettingKey 是全局短链开关（JSON）。
	SettingKey = "subscription.short_link"

	// CodeLength 12 位 Base32 已经足够；这里用 20 位，约 119 bit。
	CodeLength = 20

	codeAlphabet = "0123456789abcdefghjkmnpqrstvwxyz"
)

// Settings 是后台全局开关。
type Settings struct {
	Enabled             bool `json:"enabled"`
	AutoCreate          bool `json:"auto_create"`
	FollowTokenRotation bool `json:"follow_token_rotation"`
}

func DefaultSettings() Settings {
	return Settings{Enabled: false, AutoCreate: true, FollowTokenRotation: false}
}

// Service handles subscription short links.
type Service struct {
	repo         *repository.SubscriptionShortLinkRepository
	subRepo      *repository.SubscriptionRepository
	configRepo   *repository.MihomoConfigRepository
	fmtRepo      *repository.SubscriptionFormatRepository
	clientRepo   *repository.ClientRepository
	endpointRepo *repository.SubscriptionEndpointRepository
	settingRepo  *repository.SystemSettingRepository
	// revocationRepo 记录已撤销短码的 HMAC 指纹；未接线时为 nil（测试/只读场景）。
	revocationRepo *repository.SubscriptionShortLinkRevocationRepository
}

func NewService(
	repo *repository.SubscriptionShortLinkRepository,
	subRepo *repository.SubscriptionRepository,
	configRepo *repository.MihomoConfigRepository,
	fmtRepo *repository.SubscriptionFormatRepository,
	clientRepo *repository.ClientRepository,
	endpointRepo *repository.SubscriptionEndpointRepository,
	settingRepo *repository.SystemSettingRepository,
) *Service {
	return &Service{
		repo:         repo,
		subRepo:      subRepo,
		configRepo:   configRepo,
		fmtRepo:      fmtRepo,
		clientRepo:   clientRepo,
		endpointRepo: endpointRepo,
		settingRepo:  settingRepo,
	}
}

// SetRevocationRepository 接线撤销记录仓储（短码删除/轮换时写指纹）。
func (s *Service) SetRevocationRepository(repo *repository.SubscriptionShortLinkRevocationRepository) {
	s.revocationRepo = repo
}

// GetSettings 读取全局开关；未配置时返回默认值。
func (s *Service) GetSettings(ctx context.Context) Settings {
	cfg := DefaultSettings()
	if s.settingRepo == nil {
		return cfg
	}
	raw, err := s.settingRepo.GetByKey(ctx, SettingKey)
	if err != nil || strings.TrimSpace(raw) == "" {
		return cfg
	}
	var saved struct {
		Enabled             *bool `json:"enabled"`
		AutoCreate          *bool `json:"auto_create"`
		FollowTokenRotation *bool `json:"follow_token_rotation"`
	}
	if err := json.Unmarshal([]byte(raw), &saved); err != nil {
		return cfg
	}
	if saved.Enabled != nil {
		cfg.Enabled = *saved.Enabled
	}
	if saved.AutoCreate != nil {
		cfg.AutoCreate = *saved.AutoCreate
	}
	if saved.FollowTokenRotation != nil {
		cfg.FollowTokenRotation = *saved.FollowTokenRotation
	}
	return cfg
}

func (s *Service) SaveSettings(ctx context.Context, cfg Settings) error {
	if s.settingRepo == nil {
		return errors.New("settings repository not configured")
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return s.settingRepo.Set(ctx, SettingKey, string(raw))
}

// Enabled 返回短链是否启用。直接读库，保证后台关闭后立刻生效。
func (s *Service) Enabled(ctx context.Context) bool {
	return s.GetSettings(ctx).Enabled
}

// CreateInput 是创建短链的入参。
type CreateInput struct {
	SubscriptionID      int64
	Format              string
	MihomoConfigID      *int64
	ConfigName          string
	Platform            string
	Providers           bool
	LockParams          bool
	FollowTokenRotation bool
	ExpiresAt           *time.Time
	CreatedBy           int64
}

var validFormats = map[string]bool{
	string(formatter.FormatBase64Aggregate): true,
	string(formatter.FormatRawURI):          true,
	string(formatter.FormatMihomoYAML):      true,
	string(formatter.FormatMihomoProxies):   true,
	string(formatter.FormatSingBoxJSON):     true,
	string(formatter.FormatXrayJSON):        true,
}

func NormalizeFormat(v string) (string, error) {
	v = strings.ToLower(strings.TrimSpace(v))
	switch v {
	case "", "clash", "mihomo", "clash.meta":
		if v == "" {
			return "", nil
		}
		return string(formatter.FormatMihomoYAML), nil
	case "xray", "v2ray":
		return string(formatter.FormatXrayJSON), nil
	case "singbox", "sing-box":
		return string(formatter.FormatSingBoxJSON), nil
	case "base64":
		return string(formatter.FormatBase64Aggregate), nil
	}
	if !validFormats[v] {
		return "", fmt.Errorf("unsupported format: %s", v)
	}
	return v, nil
}

// Create 创建一条短链。Code 由服务端生成，调用方不能指定。
func (s *Service) Create(ctx context.Context, in CreateInput) (*entity.SubscriptionShortLink, error) {
	if in.SubscriptionID <= 0 {
		return nil, errors.New("subscription_id is required")
	}
	sub, err := s.subRepo.GetByID(ctx, in.SubscriptionID)
	if err != nil {
		return nil, err
	}
	format, err := NormalizeFormat(in.Format)
	if err != nil {
		return nil, err
	}

	configName := strings.TrimSpace(in.ConfigName)
	configID := in.MihomoConfigID
	if configID != nil && *configID > 0 {
		if s.configRepo == nil {
			return nil, errors.New("mihomo config repository not configured")
		}
		cfg, cerr := s.configRepo.GetByID(ctx, *configID)
		if cerr != nil || cfg == nil {
			return nil, fmt.Errorf("mihomo config %d not found", *configID)
		}
		// 停用或格式不匹配的配置会让链接退化成「只有 proxies、没有规则」，
		// 与「无配置」是同一个后果，必须在创建时就拒绝。
		if !cfg.Enabled {
			return nil, fmt.Errorf("mihomo config %d is disabled", *configID)
		}
		if format != "" && !configUsableForFormat(cfg.Format, format) {
			return nil, fmt.Errorf("mihomo config %d format %q does not match %s", *configID, cfg.Format, format)
		}
		configName = cfg.Name
	} else if format == string(formatter.FormatMihomoYAML) && configName != "" {
		// 手填配置名时按名字定位到 ID：避免改名后链接打开变成「只有 proxies、没有规则」。
		cfg := s.findConfigByName(ctx, configName, format)
		if cfg == nil {
			return nil, fmt.Errorf("mihomo config %q not found", configName)
		}
		id := cfg.ID
		configID = &id
		configName = cfg.Name
	}
	if err := ValidateConfigTarget(format, configID, configName, in.Platform); err != nil {
		return nil, err
	}

	code, err := s.generateUniqueCode(ctx)
	if err != nil {
		return nil, err
	}

	link := &entity.SubscriptionShortLink{
		SubscriptionID:      in.SubscriptionID,
		Code:                code,
		Format:              format,
		MihomoConfigID:      configID,
		ConfigName:          configName,
		Platform:            strings.TrimSpace(in.Platform),
		Providers:           in.Providers,
		LockParams:          in.LockParams,
		Enabled:             true,
		ExpiresAt:           in.ExpiresAt,
		TokenVersion:        sub.TokenVersion,
		FollowTokenRotation: in.FollowTokenRotation,
		CreatedBy:           in.CreatedBy,
	}
	if err := s.repo.Create(ctx, link); err != nil {
		return nil, err
	}
	return link, nil
}

// ValidateConfigTarget 校验格式所需的配置目标：mihomo_yaml 必须能定位到一份配置
// （配置 ID / 配置名 / 平台三选一）。没有配置时只能输出 proxies，没有
// proxy-groups/rules，客户端拿到的不是一份可用的 Clash/Mihomo 配置。
func ValidateConfigTarget(format string, configID *int64, configName, platform string) error {
	f, err := NormalizeFormat(format)
	if err != nil {
		return err
	}
	if f != string(formatter.FormatMihomoYAML) {
		return nil
	}
	if (configID != nil && *configID > 0) ||
		strings.TrimSpace(configName) != "" ||
		strings.TrimSpace(platform) != "" {
		return nil
	}
	return errors.New("mihomo_yaml short link requires a config")
}

func (s *Service) findConfigByName(ctx context.Context, name, format string) *entity.MihomoConfig {
	if s.configRepo == nil {
		return nil
	}
	configs, err := s.configRepo.ListBrief(ctx)
	if err != nil {
		return nil
	}
	for i := range configs {
		cfg := &configs[i]
		if !cfg.Enabled || cfg.Name != name {
			continue
		}
		cfgFormat, ferr := NormalizeFormat(cfg.Format)
		if ferr != nil || cfgFormat == "" {
			cfgFormat = string(formatter.FormatMihomoYAML)
		}
		if cfgFormat == format {
			return cfg
		}
	}
	return nil
}

// configUsableForFormat 判断某份共享配置能否作为该格式短链的数据源。
// mihomo_proxies_yaml（provider 数据源）复用 mihomo_yaml 配置，与渲染侧
// mihomoConfigFormatMatches 的口径保持一致。
func configUsableForFormat(configFormat, linkFormat string) bool {
	cf, err := NormalizeFormat(configFormat)
	if err != nil || cf == "" {
		cf = string(formatter.FormatMihomoYAML)
	}
	lf, err := NormalizeFormat(linkFormat)
	if err != nil || lf == "" {
		return false
	}
	if cf == lf {
		return true
	}
	return lf == string(formatter.FormatMihomoProxies) && cf == string(formatter.FormatMihomoYAML)
}

// ValidateLinkConfig 校验一条短链的配置绑定与格式是否匹配（更新路径用）。
//
// 只校验格式匹配、不校验配置是否启用：配置被停用属于运营状态，历史链接仍可
// 切换 enabled/过期时间；但绑定到别的格式的配置会让链接打开后退化成
// 「只有 proxies、没有规则」，必须拒绝。
func (s *Service) ValidateLinkConfig(ctx context.Context, link *entity.SubscriptionShortLink) error {
	if link == nil {
		return errors.New("short link is required")
	}
	if err := ValidateConfigTarget(link.Format, link.MihomoConfigID, link.ConfigName, link.Platform); err != nil {
		return err
	}
	if link.MihomoConfigID == nil || *link.MihomoConfigID <= 0 || s.configRepo == nil {
		return nil
	}
	cfg, err := s.configRepo.GetByID(ctx, *link.MihomoConfigID)
	if err != nil || cfg == nil {
		return fmt.Errorf("mihomo config %d not found", *link.MihomoConfigID)
	}
	if link.Format != "" && !configUsableForFormat(cfg.Format, link.Format) {
		return fmt.Errorf("mihomo config %d format %q does not match %s", cfg.ID, cfg.Format, link.Format)
	}
	return nil
}

// PurgeUnconfiguredMihomo 清理没有绑定配置的 mihomo_yaml 短链。
// 这类链接打开后只有 proxies（无 proxy-groups/rules），不是可用的客户端配置。
//
// 删除前把短码指纹写进撤销表：短码一旦回收，绝不能再被重新签发，
// 否则客户手里的旧地址会静默指向另一条订阅。
func (s *Service) PurgeUnconfiguredMihomo(ctx context.Context, subID int64) int64 {
	links, err := s.repo.ListBySubscription(ctx, subID)
	if err != nil {
		log.Printf("[shortlink] purge unconfigured mihomo sub=%d list failed: %v", subID, err)
		return 0
	}
	var removed int64
	for i := range links {
		l := &links[i]
		if l.Format != string(formatter.FormatMihomoYAML) {
			continue
		}
		if (l.MihomoConfigID != nil && *l.MihomoConfigID > 0) ||
			strings.TrimSpace(l.ConfigName) != "" ||
			strings.TrimSpace(l.Platform) != "" {
			continue
		}
		s.revokeCode(ctx, l, "purge_unconfigured", 0)
		if err := s.repo.Delete(ctx, l.ID); err != nil {
			log.Printf("[shortlink] purge unconfigured mihomo id=%d failed: %v", l.ID, err)
			continue
		}
		removed++
	}
	return removed
}

// AutoCreateForSubscription 在新建订阅后按后台开关补齐全部「格式 × 配置」短链。
func (s *Service) AutoCreateForSubscription(ctx context.Context, sub *entity.Subscription) error {
	if sub == nil {
		return nil
	}
	cfg := s.GetSettings(ctx)
	if !cfg.Enabled || !cfg.AutoCreate {
		return nil
	}
	if _, err := s.EnsureLinks(ctx, sub.ID); err != nil {
		log.Printf("[shortlink] auto create for sub=%d failed: %v", sub.ID, err)
		return err
	}
	return nil
}

func (s *Service) GetByID(ctx context.Context, id int64) (*entity.SubscriptionShortLink, error) {
	return s.repo.GetByID(ctx, id)
}

// GetByCode 只按 code 查；是否可用由调用方结合订阅状态判断。
// 已撤销的短码即使行还在（例如轮换回写失败）也直接拒绝。
func (s *Service) GetByCode(ctx context.Context, code string) (*entity.SubscriptionShortLink, error) {
	code = strings.ToLower(strings.TrimSpace(code))
	link, err := s.repo.GetByCode(ctx, code)
	if err != nil {
		return nil, err
	}
	if s.revocationRepo != nil {
		fp, ferr := crypto.FingerprintSecret(code)
		if ferr != nil {
			return nil, ferr
		}
		revoked, rerr := s.revocationRepo.Exists(ctx, fp)
		if rerr != nil {
			return nil, rerr
		}
		if revoked {
			return nil, errors.New("short link code revoked")
		}
	}
	return link, nil
}

func (s *Service) ListBySubscription(ctx context.Context, subID int64) ([]entity.SubscriptionShortLink, error) {
	return s.repo.ListBySubscription(ctx, subID)
}

func (s *Service) Update(ctx context.Context, link *entity.SubscriptionShortLink) error {
	return s.repo.Update(ctx, link)
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	// 删除前把短码指纹写进撤销表，防止同码在删除后被重新签发。
	if link, err := s.repo.GetByID(ctx, id); err == nil {
		s.revokeCode(ctx, link, "delete", 0)
	}
	return s.repo.Delete(ctx, id)
}

func (s *Service) DeleteBySubscription(ctx context.Context, subID int64) error {
	// 订阅删除时短链一起删；顺手把每条短码指纹写入撤销表。
	if s.revocationRepo != nil {
		if links, err := s.repo.ListBySubscription(ctx, subID); err == nil {
			for i := range links {
				s.revokeCode(ctx, &links[i], "subscription_deleted", 0)
			}
		}
	}
	return s.repo.DeleteBySubscriptionID(ctx, subID)
}

func (s *Service) SyncTokenVersion(ctx context.Context, subID int64, version int) error {
	return s.repo.SyncTokenVersion(ctx, subID, version)
}

func (s *Service) RecordAccess(ctx context.Context, id int64, at time.Time) error {
	return s.repo.RecordAccess(ctx, id, at)
}

// RotateCode 重新生成 code（旧链接立即失效）。
func (s *Service) RotateCode(ctx context.Context, id int64) (*entity.SubscriptionShortLink, error) {
	link, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	// 旧 code 进入撤销表：轮换后即使有人拿到旧地址也不能再解析。
	s.revokeCode(ctx, link, "rotate", 0)
	code, err := s.generateUniqueCode(ctx)
	if err != nil {
		return nil, err
	}
	link.Code = code
	if err := s.repo.Update(ctx, link); err != nil {
		return nil, err
	}
	return link, nil
}

// revokeCode 记录短码指纹；没有接线撤销仓储时静默跳过。
func (s *Service) revokeCode(ctx context.Context, link *entity.SubscriptionShortLink, reason string, by int64) {
	if s.revocationRepo == nil || link == nil || link.Code == "" {
		return
	}
	fp, err := crypto.FingerprintSecret(link.Code)
	if err != nil {
		log.Printf("[shortlink] fingerprint code id=%d failed: %v", link.ID, err)
		return
	}
	if err := s.revocationRepo.Create(ctx, &entity.SubscriptionShortLinkRevocation{
		CodeHMAC:  fp,
		Reason:    reason,
		RevokedBy: by,
		RevokedAt: time.Now(),
	}); err != nil {
		log.Printf("[shortlink] record revocation id=%d failed: %v", link.ID, err)
	}
}

// Backfill 为存量 active 订阅补建默认短链（幂等）。
func (s *Service) Backfill(ctx context.Context, dryRun bool) (created, skipped, failed int, err error) {
	const pageSize = 500
	for offset := 0; ; offset += pageSize {
		subs, _, listErr := s.subRepo.List(ctx, "active", pageSize, offset)
		if listErr != nil {
			return created, skipped, failed, listErr
		}
		if len(subs) == 0 {
			break
		}
		for i := range subs {
			sub := &subs[i]
			existing, listErr := s.repo.ListBySubscription(ctx, sub.ID)
			if listErr != nil {
				failed++
				continue
			}
			if dryRun {
				now := time.Now()
				missing := 0
				for _, combo := range s.combosForSubscription(ctx, sub) {
					if matchUsableLink(existing, combo, sub, now) == nil {
						missing++
					}
				}
				if missing == 0 {
					skipped++
				} else {
					created += missing
				}
				continue
			}
			// 补齐全部组合（不是只补默认那条）。
			if _, ensureErr := s.EnsureLinks(ctx, sub.ID); ensureErr != nil {
				failed++
				continue
			}
			after, listErr := s.repo.ListBySubscription(ctx, sub.ID)
			if listErr != nil {
				failed++
				continue
			}
			if delta := len(after) - len(existing); delta > 0 {
				created += delta
			} else {
				skipped++
			}
		}
		if len(subs) < pageSize {
			break
		}
	}
	return created, skipped, failed, nil
}

// CreateForSubscription 按订阅默认参数建一条短链（backfill 与手动补建共用）。
func (s *Service) CreateForSubscription(ctx context.Context, sub *entity.Subscription, follow bool) (*entity.SubscriptionShortLink, error) {
	link, err := s.Create(ctx, CreateInput{
		SubscriptionID:      sub.ID,
		Format:              sub.Format,
		MihomoConfigID:      sub.MihomoConfigID,
		FollowTokenRotation: follow,
	})
	if err != nil && sub.MihomoConfigID != nil {
		// 配置可能已被删除；退化成“跟随订阅默认”，不要因此整条订阅补建失败。
		return s.Create(ctx, CreateInput{
			SubscriptionID:      sub.ID,
			Format:              sub.Format,
			FollowTokenRotation: follow,
		})
	}
	return link, err
}

func hasDefaultLink(links []entity.SubscriptionShortLink, sub *entity.Subscription) bool {
	for i := range links {
		l := links[i]
		if l.Format != "" && sub.Format != "" && l.Format != normalizeOrEmpty(sub.Format) {
			continue
		}
		if l.MihomoConfigID != nil && sub.MihomoConfigID != nil && *l.MihomoConfigID == *sub.MihomoConfigID {
			return true
		}
		if l.MihomoConfigID == nil && sub.MihomoConfigID == nil && l.Platform == "" && !l.Providers {
			return true
		}
	}
	return false
}

func normalizeOrEmpty(v string) string {
	f, _ := NormalizeFormat(v)
	return f
}

// Combo 是一条短链要固定的输出组合。
// 门户/客户端详情按「格式 × 配置 × providers」展开，每组合一条短链，
// URL 本身不再带任何 query 参数。
type Combo struct {
	Format     string
	ConfigID   *int64
	ConfigName string
	Platform   string
	Providers  bool
}

// EnsureLinks 幂等补齐该订阅所有可见组合的短链，返回补齐后的链接。
// 已有可用链接直接复用；缺的才新建，不会重复膨胀。
func (s *Service) EnsureLinks(ctx context.Context, subID int64) ([]entity.SubscriptionShortLink, error) {
	if !s.Enabled(ctx) {
		return nil, nil
	}
	sub, err := s.subRepo.GetByID(ctx, subID)
	if err != nil || sub == nil {
		return nil, err
	}
	return s.ensureCombos(ctx, sub, s.combosForSubscription(ctx, sub))
}

// combosForSubscription 计算门户需要的全部组合：
//   - 固定 base64 / raw_uri；
//   - 套餐可见的每个 Mihomo 配置（普通 + providers 变体）；
//   - 套餐可见的订阅格式（无配置）；
//   - 订阅自身默认格式（保证“默认链接”仍指向它）。
func (s *Service) combosForSubscription(ctx context.Context, sub *entity.Subscription) []Combo {
	planID := int64(0)
	if s.clientRepo != nil && sub.ClientID > 0 {
		if cl, err := s.clientRepo.GetByID(ctx, sub.ClientID); err == nil && cl != nil {
			planID = cl.PlanID
		}
	}

	combos := make([]Combo, 0, 12)
	seen := map[string]bool{}
	add := func(c Combo) {
		format, err := NormalizeFormat(c.Format)
		if err != nil || format == "" {
			return
		}
		key := format + "|" + c.ConfigName + "|" + c.Platform + "|"
		if c.ConfigID != nil {
			key += "id" + strconv.FormatInt(*c.ConfigID, 10)
		}
		key += "|" + strconv.FormatBool(c.Providers)
		if seen[key] {
			return
		}
		seen[key] = true
		c.Format = format
		combos = append(combos, c)
	}

	add(Combo{Format: string(formatter.FormatBase64Aggregate)})
	add(Combo{Format: string(formatter.FormatRawURI)})

	// 与门户 / 客户详情的展示口径保持一致：有 Mihomo 配置的格式只出「配置链接」，
	// 和同名订阅格式去重；没有配置的 sing-box / xray / mihomo_yaml 不再凭空多出一条空链接。
	configNames := map[string]bool{}
	if s.configRepo != nil {
		configs, err := s.configRepo.ListBrief(ctx)
		if err == nil {
			for i := range configs {
				cfg := &configs[i]
				if !cfg.Enabled {
					continue
				}
				if len(cfg.PlanIDs) > 0 && !containsID(cfg.PlanIDs, planID) {
					continue
				}
				format, ferr := NormalizeFormat(cfg.Format)
				if ferr != nil || format == "" {
					format = string(formatter.FormatMihomoYAML)
				}
				id := cfg.ID
				add(Combo{Format: format, ConfigID: &id, ConfigName: cfg.Name, Platform: cfg.Platform})
				if format == string(formatter.FormatMihomoYAML) {
					add(Combo{Format: format, ConfigID: &id, ConfigName: cfg.Name, Platform: cfg.Platform, Providers: true})
				}
				configNames[format+"|"+strings.TrimSpace(cfg.Name)] = true
			}
		}
	}

	if s.fmtRepo != nil {
		formats, err := s.fmtRepo.List(ctx)
		if err == nil {
			for i := range formats {
				f := &formats[i]
				if !f.Enabled {
					continue
				}
				if len(f.PlanIDs) > 0 && !containsID(f.PlanIDs, planID) {
					continue
				}
				format, ferr := NormalizeFormat(f.Format)
				if ferr != nil || format == "" {
					continue
				}
				// 同名配置已由上面的 Mihomo 配置覆盖（门户也做同名去重）。
				if configNames[format+"|"+strings.TrimSpace(f.Name)] {
					continue
				}
				// sing-box / Xray / mihomo_yaml 的链接必须绑定一份该格式的配置
				// （订阅格式里的 "Sing-Box/V2Ray JSON" 只是分类元数据；mihomo_yaml
				// 无配置只能输出 proxies，没有 proxy-groups/rules）。
				// 配置链接已在上面的 mihomo_configs 循环里生成，这里一律跳过。
				if format == string(formatter.FormatSingBoxJSON) ||
					format == string(formatter.FormatXrayJSON) ||
					format == string(formatter.FormatMihomoYAML) {
					continue
				}
				add(Combo{Format: format})
			}
		}
	}

	// 订阅默认格式：确保 DefaultURL 仍能选中它。
	// mihomo_yaml 没有配置时不再退化出一条「只有 proxies、没有规则」的链接。
	defaultCombo := Combo{Format: sub.Format}
	if sub.MihomoConfigID != nil {
		id := *sub.MihomoConfigID
		defaultCombo.ConfigID = &id
		if s.configRepo != nil {
			if cfg, err := s.configRepo.GetByID(ctx, id); err == nil && cfg != nil {
				defaultCombo.ConfigName = cfg.Name
				defaultCombo.Platform = cfg.Platform
			}
		}
	}
	if f, _ := NormalizeFormat(defaultCombo.Format); f != string(formatter.FormatMihomoYAML) || defaultCombo.ConfigID != nil {
		add(defaultCombo)
	}

	return combos
}

func (s *Service) ensureCombos(ctx context.Context, sub *entity.Subscription, combos []Combo) ([]entity.SubscriptionShortLink, error) {
	links, err := s.repo.ListBySubscription(ctx, sub.ID)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	follow := s.GetSettings(ctx).FollowTokenRotation
	out := make([]entity.SubscriptionShortLink, 0, len(combos))

	for _, combo := range combos {
		format, ferr := NormalizeFormat(combo.Format)
		if ferr != nil || format == "" {
			continue
		}
		combo.Format = format
		if existing := matchUsableLink(links, combo, sub, now); existing != nil {
			out = append(out, *existing)
			continue
		}
		link, cerr := s.Create(ctx, CreateInput{
			SubscriptionID:      sub.ID,
			Format:              format,
			MihomoConfigID:      combo.ConfigID,
			ConfigName:          combo.ConfigName,
			Platform:            combo.Platform,
			Providers:           combo.Providers,
			FollowTokenRotation: follow,
		})
		if cerr != nil {
			log.Printf("[shortlink] ensure sub=%d format=%s config=%s failed: %v", sub.ID, format, combo.ConfigName, cerr)
			continue
		}
		out = append(out, *link)
		links = append(links, *link)
	}
	return out, nil
}

// matchUsableLink 只认「完全一致 + 当前可用」的链接，避免把别的格式/配置顶上来。
func matchUsableLink(links []entity.SubscriptionShortLink, combo Combo, sub *entity.Subscription, now time.Time) *entity.SubscriptionShortLink {
	for i := range links {
		l := &links[i]
		if !l.Enabled || l.RevokedAt != nil || (l.ExpiresAt != nil && !l.ExpiresAt.After(now)) {
			continue
		}
		if !l.FollowTokenRotation && l.TokenVersion != sub.TokenVersion {
			continue
		}
		if l.Format != combo.Format {
			continue
		}
		if strings.TrimSpace(l.Platform) != strings.TrimSpace(combo.Platform) {
			continue
		}
		if l.Providers != combo.Providers {
			continue
		}
		if !sameConfigTarget(l, combo) {
			continue
		}
		return l
	}
	return nil
}

func sameConfigTarget(l *entity.SubscriptionShortLink, combo Combo) bool {
	if combo.ConfigID != nil && l.MihomoConfigID != nil {
		return *combo.ConfigID == *l.MihomoConfigID
	}
	lc := strings.TrimSpace(l.ConfigName)
	cc := strings.TrimSpace(combo.ConfigName)
	if lc == "" && cc == "" {
		return l.MihomoConfigID == nil && combo.ConfigID == nil
	}
	return strings.EqualFold(lc, cc)
}

func containsID(ids entity.Int64List, id int64) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

func (s *Service) generateUniqueCode(ctx context.Context) (string, error) {
	for i := 0; i < 8; i++ {
		code, err := randomCode(CodeLength)
		if err != nil {
			return "", err
		}
		exists, err := s.repo.CodeExists(ctx, code)
		if err != nil {
			return "", err
		}
		if exists {
			continue
		}
		if s.revocationRepo != nil {
			fp, ferr := crypto.FingerprintSecret(code)
			if ferr != nil {
				return "", ferr
			}
			revoked, rerr := s.revocationRepo.Exists(ctx, fp)
			if rerr != nil {
				return "", rerr
			}
			if revoked {
				continue
			}
		}
		return code, nil
	}
	return "", errors.New("failed to allocate unique short link code")
}

func randomCode(length int) (string, error) {
	if length <= 0 {
		length = CodeLength
	}
	max := big.NewInt(int64(len(codeAlphabet)))
	out := make([]byte, length)
	for i := range out {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		out[i] = codeAlphabet[n.Int64()]
	}
	return string(out), nil
}

// IsCode 判断一个路径段是否是短码（与 64 位 hex token 区分）。
// 大小写都接受：查询时会统一转小写，便于用户手动输入。
func IsCode(s string) bool {
	if len(s) != CodeLength {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		if !strings.ContainsRune(codeAlphabet, rune(c)) {
			return false
		}
	}
	return true
}

// CredentialURL 构造「订阅基址 + 凭证段」的完整地址。
// credential 可以是 token，也可以是短码；调用方保证不要把它写进日志。
func (s *Service) CredentialURL(ctx context.Context, credential, requestHost string) string {
	if credential == "" {
		return ""
	}
	if s.endpointRepo != nil {
		if endpoints, err := s.endpointRepo.GetEnabled(ctx); err == nil {
			var chosen *entity.SubscriptionEndpoint
			host := stripHostPort(requestHost)
			for i := range endpoints {
				ep := &endpoints[i]
				if strings.TrimSpace(ep.Domain) == "" {
					continue
				}
				if chosen == nil {
					chosen = ep
				}
				if host != "" && strings.EqualFold(strings.TrimSpace(ep.Domain), host) {
					chosen = ep
					break
				}
			}
			if chosen != nil {
				path := strings.TrimRight(strings.TrimSpace(chosen.Path), "/")
				if path == "" || path == "/" {
					return fmt.Sprintf("https://%s/%s", chosen.Domain, credential)
				}
				return fmt.Sprintf("https://%s%s/%s", chosen.Domain, path, credential)
			}
		}
	}
	return s.panelFallbackURL(ctx, credential, requestHost)
}

func (s *Service) panelFallbackURL(ctx context.Context, credential, requestHost string) string {
	scheme, host, port := "https", "", ""
	if s.settingRepo != nil {
		if raw, err := s.settingRepo.GetByKey(ctx, "panel.external_url"); err == nil {
			if u, err := url.Parse(strings.TrimSpace(raw)); err == nil && u.Host != "" {
				scheme = u.Scheme
				host = u.Hostname()
				port = u.Port()
			}
		}
	}
	if host == "" {
		host = stripHostPort(requestHost)
		if host == "" {
			// 门户 /me 没有请求域名（requestHost 为空）：宁可不给短链、
			// 让前端回退长链接，也不要下发 https://127.0.0.1/... 的死链。
			return ""
		}
	}
	if scheme == "" {
		scheme = "https"
	}
	// 独立订阅端口：优先替换端口；否则走面板 /sub。
	subPort := ""
	listenPort := ""
	if s.settingRepo != nil {
		subPort, _ = s.settingRepo.GetByKey(ctx, "panel.subscription_port")
		listenPort, _ = s.settingRepo.GetByKey(ctx, "panel.listen_port")
	}
	subPort = strings.TrimSpace(subPort)
	listenPort = strings.TrimSpace(listenPort)
	if subPort != "" && subPort != "0" && subPort != listenPort {
		return fmt.Sprintf("%s://%s:%s/%s", scheme, host, subPort, credential)
	}
	if port != "" && port != "80" && port != "443" {
		host = net.JoinHostPort(host, port)
	}
	return fmt.Sprintf("%s://%s/sub/%s", scheme, host, credential)
}

func stripHostPort(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	if i := strings.Index(host, ","); i >= 0 {
		host = host[:i]
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return host
}

// ManualLinkURL 供管理端补全 URL（短链未启用时也可预览）。
func (s *Service) ManualLinkURL(ctx context.Context, link *entity.SubscriptionShortLink, requestHost string) string {
	if link == nil {
		return ""
	}
	return s.CredentialURL(ctx, link.Code, requestHost)
}

// LinkView 是门户/客户端详情可展示的短链视图。
type LinkView struct {
	ID         int64  `json:"id"`
	URL        string `json:"url"`
	Format     string `json:"format"`
	ConfigName string `json:"config_name"`
	Platform   string `json:"platform"`
	Providers  bool   `json:"providers"`
	LockParams bool   `json:"lock_params"`
}

// ListViews 返回当前已存在的短链（不自动补齐；管理端列表用）。
func (s *Service) ListViews(ctx context.Context, subID int64) []LinkView {
	if !s.Enabled(ctx) {
		return nil
	}
	sub, err := s.subRepo.GetByID(ctx, subID)
	if err != nil || sub == nil {
		return nil
	}
	links, err := s.repo.ListBySubscription(ctx, subID)
	if err != nil {
		return nil
	}
	return s.viewsFromLinks(ctx, sub, links)
}

// EnsureViews 与 ListViews 同结构，但会先幂等补齐该订阅全部「格式 × 配置」组合。
// 门户 /me 用这个：前端拿到的每条短链都是干净的 URL，不需要再拼 query。
func (s *Service) EnsureViews(ctx context.Context, subID int64) []LinkView {
	if !s.Enabled(ctx) {
		return nil
	}
	// 门户 /me 是高频只读接口：后台关掉「自动创建」后不应再按组合插入新行，
	// 只返回已有链接（缺的由管理端显式补齐）。同时避免每次刷新 /me 都写库。
	if cfg := s.GetSettings(ctx); !cfg.AutoCreate {
		return s.ListViews(ctx, subID)
	}
	links, err := s.EnsureLinks(ctx, subID)
	if err != nil {
		return nil
	}
	sub, err := s.subRepo.GetByID(ctx, subID)
	if err != nil || sub == nil {
		return nil
	}
	return s.viewsFromLinks(ctx, sub, links)
}

func (s *Service) viewsFromLinks(ctx context.Context, sub *entity.Subscription, links []entity.SubscriptionShortLink) []LinkView {
	now := time.Now()
	out := make([]LinkView, 0, len(links))
	for i := range links {
		l := &links[i]
		if !l.Enabled || l.RevokedAt != nil || (l.ExpiresAt != nil && !l.ExpiresAt.After(now)) {
			continue
		}
		if !l.FollowTokenRotation && l.TokenVersion != sub.TokenVersion {
			continue
		}
		url := s.ManualLinkURL(ctx, l, "")
		if url == "" {
			// 拿不到对外域名（未配置 external_url/订阅端点）：不返回半成品，
			// 前端按「无短链」回退长链接。
			continue
		}
		out = append(out, LinkView{
			ID:         l.ID,
			URL:        url,
			Format:     l.Format,
			ConfigName: l.ConfigName,
			Platform:   l.Platform,
			Providers:  l.Providers,
			LockParams: l.LockParams,
		})
	}
	return out
}

// DefaultURL 返回默认短链（跟随订阅默认格式的那条，否则第一条可用短链）。
func (s *Service) DefaultURL(ctx context.Context, subID int64) string {
	return PickDefaultURL(s.ListViews(ctx, subID), s.defaultFormatOf(ctx, subID))
}

// EnsureDefaultURL 同 DefaultURL，但会先补齐组合。
func (s *Service) EnsureDefaultURL(ctx context.Context, subID int64) string {
	return PickDefaultURL(s.EnsureViews(ctx, subID), s.defaultFormatOf(ctx, subID))
}

func (s *Service) defaultFormatOf(ctx context.Context, subID int64) string {
	sub, err := s.subRepo.GetByID(ctx, subID)
	if err != nil || sub == nil {
		return ""
	}
	return sub.Format
}

// PickDefaultURL 在短链列表里挑「订阅默认格式、无配置、无 providers」的那条。
func PickDefaultURL(views []LinkView, format string) string {
	if len(views) == 0 {
		return ""
	}
	want, _ := NormalizeFormat(format)
	for i := range views {
		if views[i].Format == want && views[i].ConfigName == "" && views[i].Platform == "" && !views[i].Providers {
			return views[i].URL
		}
	}
	return views[0].URL
}

// FormatExtension 返回短链对应的下载扩展名（仅用于 Content-Disposition 优化）。
func FormatExtension(format string) string {
	switch format {
	case string(formatter.FormatMihomoYAML), string(formatter.FormatMihomoProxies):
		return ".yaml"
	case string(formatter.FormatSingBoxJSON), string(formatter.FormatXrayJSON):
		return ".json"
	default:
		return ".txt"
	}
}
