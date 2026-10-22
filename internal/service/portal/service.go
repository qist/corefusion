package portal

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// SettingStore provides access to system settings.
type SettingStore interface {
	GetByKey(ctx context.Context, key string) (string, error)
	SetByKey(ctx context.Context, key string, value string) error
}

// Service handles portal template business logic.
type Service struct {
	repo       *repository.PortalTemplateRepository
	settingSvc SettingStore
	cache      *RenderCache
}

// NewService creates a new portal template service.
func NewService(repo *repository.PortalTemplateRepository) *Service {
	return &Service{repo: repo, cache: NewRenderCache()}
}

// WithSettingStore injects a setting store for managing registration_enabled / portal_port.
func (s *Service) WithSettingStore(ss SettingStore) *Service {
	s.settingSvc = ss
	return s
}

// RenderPage renders the portal HTML page from the active template with caching.
// In dev mode (PORTAL_DEV_MODE=true), caching is bypassed and the page is
// re-rendered from the database on every request.
// In production mode, the fully rendered HTML is served straight from memory —
// re-rendering (DB read + template string substitution) only happens after a
// write invalidates the cache or the TTL safety net elapses.
func (s *Service) RenderPage(ctx context.Context) string {
	// Cache hit — zero DB access, zero re-rendering
	if html, ok := s.cache.Get(); ok {
		return html
	}

	// Cache miss — render from the database
	registrationEnabled := false
	if s.settingSvc != nil {
		if v, err := s.settingSvc.GetByKey(ctx, "panel.registration_enabled"); err == nil && v == "true" {
			registrationEnabled = true
		}
	}

	tmpl, err := s.repo.GetActive(ctx)
	if err != nil || tmpl == nil {
		return "<html><body><h1>Portal not configured</h1></body></html>"
	}

	cfg := MergeConfig(tmpl, registrationEnabled)
	html := renderPortalHTML(tmpl.HTMLContent, tmpl.CSSContent, tmpl.JSContent, tmpl.Config, cfg)

	s.cache.Set(html)

	return html
}

// InvalidateCache clears the render cache, forcing a rebuild on next request.
func (s *Service) InvalidateCache() {
	s.cache.Invalidate()
}

// RebuildCache forces a cache rebuild by rendering the page once and caching it.
func (s *Service) RebuildCache(ctx context.Context) error {
	s.cache.Invalidate()
	// Trigger a render which will populate the cache
	_ = s.RenderPage(ctx)
	return nil
}

// GetCacheStats returns diagnostic information about the render cache.
func (s *Service) GetCacheStats() CacheStats {
	return s.cache.Stats()
}

// SetDevMode enables or disables dev mode (cache bypass) at runtime.
func (s *Service) SetDevMode(dev bool) {
	s.cache.SetDevMode(dev)
}

// IsDevMode returns whether dev mode is currently active.
func (s *Service) IsDevMode() bool {
	return s.cache.IsDevMode()
}

// CreateInput is the input for creating a portal template.
type CreateInput struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	HTMLContent string `json:"html_content"`
	CSSContent  string `json:"css_content"`
	JSContent   string `json:"js_content"`
	Config      string `json:"config"`
}

// UpdateInput is the input for updating a portal template / active site.
// Supports both the raw config string and structured fields.
// If structured fields (navigation, modules, theme) are provided, they take
// precedence over the raw config string.
type UpdateInput struct {
	Name                *string `json:"name"`
	Description         *string `json:"description"`
	HTMLContent         *string `json:"html_content"`
	CSSContent          *string `json:"css_content"`
	JSContent           *string `json:"js_content"`
	Config              *string `json:"config"`
	RegistrationEnabled *bool   `json:"registration_enabled"`
	PortalPort          *string `json:"portal_port"`

	// Structured fields — when provided, merged into config JSON
	Navigation []NavigationItem   `json:"navigation"`
	Modules    []ModuleItem       `json:"modules"`
	Theme      *PortalThemeConfig `json:"theme"`
	Lang       json.RawMessage    `json:"lang"`
	Regions    json.RawMessage    `json:"regions"`
	// 合规文档链接 ✓（登录页展示 ✓；nil = 不修改 ✓）
	PrivacyPolicyURL  *string `json:"privacy_policy_url"`
	TermsOfServiceURL *string `json:"terms_of_service_url"`
	// 合规文档内容 ✓（按语言 markdown：{"zh-CN":"...","en":"..."}；nil = 不修改 ✓）
	PrivacyPolicyContent  *map[string]string `json:"privacy_policy_content"`
	TermsOfServiceContent *map[string]string `json:"terms_of_service_content"`
}

// PortalSiteConfig is the response for the active portal site.
// It wraps the template entity with extra system-level settings and
// structured navigation/modules/theme fields parsed from the config JSON.
type PortalSiteConfig struct {
	*entity.PortalTemplate
	RegistrationEnabled bool   `json:"registration_enabled"`
	PortalPort          string `json:"portal_port"`
	// 门户对外地址 ✓（后台生成「合规文档独立页」/ 预览链接用：门户配置了域名用门户域名，
	// 否则回退面板地址）：
	//   PortalExternalURL = oauth.callback_domain（门户回调域名）> panel.external_url；均未配置为 ""（前端回退当前 origin）
	//   PortalPagePath    = 门户在本域名下的挂载路径（独立门户端口为 ""，否则 "/p"）
	PortalExternalURL string `json:"portal_external_url"`
	PortalPagePath    string `json:"portal_page_path"`

	// Structured fields parsed from the config JSON string.
	// These are read-only in the response; to update them, use
	// the navigation/modules/theme fields in UpdateInput.
	SiteName            string           `json:"site_name"`
	LogoURL             string           `json:"logo_url"`
	PrimaryColor        string           `json:"primary_color"`
	BackgroundColor     string           `json:"background_color"`
	CardBackgroundColor string           `json:"card_background_color"`
	TextColor           string           `json:"text_color"`
	AccentColor         string           `json:"accent_color"`
	FontFamily          string           `json:"font_family"`
	PortalEditorVersion int              `json:"portal_editor_version"`
	Navigation          []NavigationItem `json:"navigation"`
	Modules             []ModuleItem     `json:"modules"`
	Lang                json.RawMessage  `json:"lang"`
	Regions             json.RawMessage  `json:"regions"`
	// 合规文档链接 ✓（登录页展示 ✓；编辑器"基础设置"里配置 ✓）
	PrivacyPolicyURL  string `json:"privacy_policy_url"`
	TermsOfServiceURL string `json:"terms_of_service_url"`
	// 合规文档内容 ✓（按语言 markdown：{"zh-CN":"...","en":"..."}）
	PrivacyPolicyContent  map[string]string `json:"privacy_policy_content"`
	TermsOfServiceContent map[string]string `json:"terms_of_service_content"`
}

// parseConfigToSiteConfig parses the config JSON string and populates the
// structured fields on a PortalSiteConfig.
func parseConfigToSiteConfig(configStr string, cfg *PortalSiteConfig) {
	if configStr == "" {
		return
	}
	var stored map[string]json.RawMessage
	if json.Unmarshal([]byte(configStr), &stored) != nil {
		return
	}
	// Theme fields
	stringFields := map[string]*string{
		"site_name":             &cfg.SiteName,
		"logo_url":              &cfg.LogoURL,
		"primary_color":         &cfg.PrimaryColor,
		"background_color":      &cfg.BackgroundColor,
		"card_background_color": &cfg.CardBackgroundColor,
		"text_color":            &cfg.TextColor,
		"accent_color":          &cfg.AccentColor,
		"font_family":           &cfg.FontFamily,
		"privacy_policy_url":    &cfg.PrivacyPolicyURL,
		"terms_of_service_url":  &cfg.TermsOfServiceURL,
	}
	for key, ptr := range stringFields {
		if v, ok := stored[key]; ok {
			var s string
			if json.Unmarshal(v, &s) == nil {
				*ptr = s
			}
		}
	}
	// 合规文档内容 ✓（按语言 markdown：{"zh-CN":"...","en":"..."}）
	if v, ok := stored["privacy_policy_content"]; ok {
		if m := parseLegalContent(v); m != nil {
			cfg.PrivacyPolicyContent = m
		}
	}
	if v, ok := stored["terms_of_service_content"]; ok {
		if m := parseLegalContent(v); m != nil {
			cfg.TermsOfServiceContent = m
		}
	}
	// Version
	if v, ok := stored["portal_editor_version"]; ok {
		var n int
		if json.Unmarshal(v, &n) == nil {
			cfg.PortalEditorVersion = n
		}
	}
	// Navigation
	if v, ok := stored["navigation"]; ok {
		cfg.Navigation = ParseNavigationItems(v)
	}
	// Modules
	if v, ok := stored["modules"]; ok {
		cfg.Modules = ParseModuleItems(v)
	}
	// Lang (multi-language pack)
	// 库里的 lang 可能只是 diff（与内置包相同的文案已剥离）→ 这里补回内置兜底，
	// 让编辑器拿到完整文案（保存时 ValidatePortalConfig 会再剥离 ✓）。
	if v, ok := stored["lang"]; ok && len(v) > 0 && string(v) != "null" {
		cfg.Lang = MergeBuiltinLangFallback(v)
	} else {
		cfg.Lang = json.RawMessage(defaultMultiLangJSON)
	}
	// Regions
	if v, ok := stored["regions"]; ok && len(v) > 0 && string(v) != "null" && string(v) != "{}" {
		cfg.Regions = v
	} else {
		cfg.Regions = json.RawMessage(defaultRegionsJSON)
	}
}

// parseLegalContent 解析合规文档内容（隐私政策 / 服务条款）。
// 新格式：{"zh-CN":"...","en":"..."}；旧格式：单语言纯字符串。
// 旧字符串归一到默认语言下，避免历史内容丢失。
func parseLegalContent(raw json.RawMessage) map[string]string {
	if len(raw) == 0 {
		return nil
	}
	var m map[string]string
	if json.Unmarshal(raw, &m) == nil {
		return m
	}
	var s string
	if json.Unmarshal(raw, &s) == nil && strings.TrimSpace(s) != "" {
		return map[string]string{defaultLangCode: s}
	}
	return nil
}

// List returns all portal templates.
func (s *Service) List(ctx context.Context) ([]entity.PortalTemplate, error) {
	return s.repo.List(ctx)
}

// GetByID returns a portal template by ID.
func (s *Service) GetByID(ctx context.Context, id int64) (*entity.PortalTemplate, error) {
	return s.repo.GetByID(ctx, id)
}

// GetActive returns the currently active portal template.
func (s *Service) GetActive(ctx context.Context) (*entity.PortalTemplate, error) {
	return s.repo.GetActive(ctx)
}

// Create creates a new portal template.
func (s *Service) Create(ctx context.Context, input CreateInput) (*entity.PortalTemplate, error) {
	configStr := input.Config
	if configStr != "" {
		cleaned, err := ValidatePortalConfig(configStr)
		if err != nil {
			return nil, fmt.Errorf("config validation failed: %w", err)
		}
		configStr = cleaned
	}
	// 新建模板同样过一遍安全补丁（M7）：用户内容（工单消息）不允许直接执行 HTML。
	// 幂等 —— 已经是安全写法时原样保留，不会改动管理员的定制。
	safeHTML, safeJS, _ := PatchTemplateSecurity(input.HTMLContent, input.JSContent)
	tmpl := &entity.PortalTemplate{
		Name:        input.Name,
		Description: input.Description,
		HTMLContent: safeHTML,
		CSSContent:  input.CSSContent,
		JSContent:   safeJS,
		Config:      configStr,
		IsActive:    false,
		IsBuiltIn:   false,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := s.repo.Create(ctx, tmpl); err != nil {
		return nil, err
	}
	return tmpl, nil
}

// Update updates an existing portal template.
func (s *Service) Update(ctx context.Context, id int64, input UpdateInput) (*entity.PortalTemplate, error) {
	tmpl, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if input.Name != nil {
		tmpl.Name = *input.Name
	}
	if input.Description != nil {
		tmpl.Description = *input.Description
	}
	if input.HTMLContent != nil {
		tmpl.HTMLContent = *input.HTMLContent
	}
	if input.CSSContent != nil {
		tmpl.CSSContent = *input.CSSContent
	}
	if input.JSContent != nil {
		tmpl.JSContent = *input.JSContent
	}
	if input.Config != nil {
		cleaned, err := ValidatePortalConfig(*input.Config)
		if err != nil {
			return nil, fmt.Errorf("config validation failed: %w", err)
		}
		tmpl.Config = cleaned
	}
	tmpl.UpdatedAt = time.Now()

	// 保存时再兜一次（M7）：哪怕管理员从旧备份粘回旧写法，也不会把 XSS 带回来。
	if safeHTML, safeJS, changed := PatchTemplateSecurity(tmpl.HTMLContent, tmpl.JSContent); changed {
		tmpl.HTMLContent = safeHTML
		tmpl.JSContent = safeJS
		log.Printf("[portal] security patch applied on save (M7: 用户内容不再执行 HTML)")
	}
	// 升级 = 到期预约切换：旧模板粘回来时同样纠正（跨套餐发 upgrade / 认 scheduled）✓
	if h, j, changed := PatchPlanUpgradeScheduledJS(tmpl.HTMLContent, tmpl.JSContent); changed {
		tmpl.HTMLContent = h
		tmpl.JSContent = j
	}
	// regions.auth（登录卡区域）在 config 里 → 保存时一并补齐（自助解封卡 ✓）
	if newConfig, cfgChanged := PatchRegionsSecurity(tmpl.Config); cfgChanged {
		tmpl.Config = newConfig
	}
	// 语言包：升级预约文案 + scheduled 状态键 ✓
	if newConfig, cfgChanged := PatchPlanUpgradeScheduledConfig(tmpl.Config); cfgChanged {
		tmpl.Config = newConfig
	}

	if err := s.repo.Update(ctx, tmpl); err != nil {
		return nil, err
	}
	// Invalidate cache so the next request re-renders with the new content
	s.cache.Invalidate()
	return tmpl, nil
}

// Delete deletes a portal template by ID.
func (s *Service) Delete(ctx context.Context, id int64) error {
	tmpl, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if tmpl.IsBuiltIn {
		return fmt.Errorf("cannot delete built-in template")
	}
	if tmpl.IsActive {
		return fmt.Errorf("cannot delete active template, deactivate first")
	}
	return s.repo.Delete(ctx, id)
}

// Activate sets a template as the active one.
func (s *Service) Activate(ctx context.Context, id int64) error {
	tmpl, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	// 安全审计 L-2：激活旧模板是把未打补丁的 HTML/JS 原样带回门户的入口 ——
	// 落库激活前统一补丁（幂等，与 Update/EnsureSecurityPatches 同一实现）。
	newHTML, newJS, changed := PatchTemplateSecurity(tmpl.HTMLContent, tmpl.JSContent)
	// 升级 = 到期预约切换（跨套餐发 upgrade / 认 scheduled 回调 / 状态表补键）✓
	if h, j, c := PatchPlanUpgradeScheduledJS(newHTML, newJS); c {
		newHTML, newJS = h, j
		changed = true
	}
	// regions.auth（登录卡区域）在 config JSON 里 → 一并补齐（自助解封卡 ✓）
	newConfig, cfgChanged := PatchRegionsSecurity(tmpl.Config)
	// 存量模板语言包：升级预约文案 + scheduled 状态键 ✓
	if c, cc := PatchPlanUpgradeScheduledConfig(newConfig); cc {
		newConfig = c
		cfgChanged = true
	}
	if changed || cfgChanged {
		tmpl.HTMLContent = newHTML
		tmpl.JSContent = newJS
		tmpl.Config = newConfig
		if err := s.repo.Update(ctx, tmpl); err != nil {
			return err
		}
		log.Printf("[portal] security patch applied before activation of template %d (M7)", tmpl.ID)
	}
	if err := s.repo.Activate(ctx, id); err != nil {
		return err
	}
	s.cache.Invalidate()
	return nil
}

// GetActiveConfig returns the currently active portal site's full configuration.
// If no active template exists, it seeds the built-in default first.
// The config JSON string is parsed into structured navigation, modules, and theme fields.
func (s *Service) GetActiveConfig(ctx context.Context) (*PortalSiteConfig, error) {
	tmpl, err := s.repo.GetActive(ctx)
	if err != nil || tmpl == nil {
		// No active template — seed the built-in default and return it
		if err := s.SeedBuiltInTemplate(ctx); err != nil {
			return nil, fmt.Errorf("failed to seed default template: %w", err)
		}
		tmpl, err = s.repo.GetActive(ctx)
		if err != nil {
			return nil, err
		}
	}

	cfg := &PortalSiteConfig{PortalTemplate: tmpl}

	// Read system-level settings
	if s.settingSvc != nil {
		if v, err := s.settingSvc.GetByKey(ctx, "panel.registration_enabled"); err == nil {
			cfg.RegistrationEnabled = v == "true"
		}
		if v, err := s.settingSvc.GetByKey(ctx, "panel.portal_port"); err == nil {
			cfg.PortalPort = v
		}
	}

	// 门户对外地址（供后台生成合规文档独立页 / 预览链接）：门户域名优先，未配置回退面板地址。
	cfg.PortalExternalURL = s.resolvePortalExternalURL(ctx)
	cfg.PortalPagePath = s.portalPagePath(ctx)

	// Parse the config JSON string into structured fields
	parseConfigToSiteConfig(tmpl.Config, cfg)

	return cfg, nil
}

// resolvePortalExternalURL 返回门户对外基址（scheme + host，不含路径/查询）。
// 优先级：oauth.callback_domain（门户回调域名）> panel.external_url；均未配置返回 ""。
func (s *Service) resolvePortalExternalURL(ctx context.Context) string {
	if s.settingSvc == nil {
		return ""
	}
	if d, _ := s.settingSvc.GetByKey(ctx, "oauth.callback_domain"); strings.TrimSpace(d) != "" {
		base := strings.TrimSpace(d)
		if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
			base = "https://" + base
		}
		return strings.TrimRight(base, "/")
	}
	if base, _ := s.settingSvc.GetByKey(ctx, "panel.external_url"); strings.TrimSpace(base) != "" {
		return strings.TrimRight(strings.TrimSpace(base), "/")
	}
	return ""
}

// portalPagePath 返回门户页面在本域名下的挂载路径（"" 或 "/p"）。
// 与 proxy portal_forward / 充值 return_url 的生成规则一致：
// panel.portal_port 已配置（独立门户端口）→ 门户整域名挂在根路径 ""；否则共用主端口挂在 "/p"。
func (s *Service) portalPagePath(ctx context.Context) string {
	if s.settingSvc != nil {
		if p, _ := s.settingSvc.GetByKey(ctx, "panel.portal_port"); p != "" && p != "0" {
			return ""
		}
	}
	return "/p"
}

// UpdateActiveConfig updates the currently active portal configuration.
// It supports both the raw config string and structured fields.
// If structured fields (navigation, modules, theme) are provided, they are
// merged into the config JSON, taking precedence over the raw config string.
func (s *Service) UpdateActiveConfig(ctx context.Context, input UpdateInput) (*PortalSiteConfig, error) {
	tmpl, err := s.GetActiveConfig(ctx)
	if err != nil {
		return nil, err
	}
	// Apply template updates
	if input.Name != nil {
		tmpl.Name = *input.Name
	}
	if input.Description != nil {
		tmpl.Description = *input.Description
	}
	if input.HTMLContent != nil {
		tmpl.HTMLContent = *input.HTMLContent
	}
	if input.CSSContent != nil {
		tmpl.CSSContent = *input.CSSContent
	}
	if input.JSContent != nil {
		tmpl.JSContent = *input.JSContent
	}

	// Build the config JSON from the existing config + structured inputs
	if input.Config != nil || len(input.Navigation) > 0 || len(input.Modules) > 0 || input.Theme != nil || len(input.Regions) > 0 ||
		len(input.Lang) > 0 ||
		input.PrivacyPolicyURL != nil || input.TermsOfServiceURL != nil ||
		input.PrivacyPolicyContent != nil || input.TermsOfServiceContent != nil {
		// Start from existing config or fresh map
		configMap := make(map[string]interface{})
		if tmpl.Config != "" {
			_ = json.Unmarshal([]byte(tmpl.Config), &configMap)
		}

		// If raw config string provided, use it as base
		if input.Config != nil {
			configMap = make(map[string]interface{})
			_ = json.Unmarshal([]byte(*input.Config), &configMap)
		}

		// Merge theme fields
		if input.Theme != nil {
			configMap["site_name"] = input.Theme.SiteName
			configMap["logo_url"] = input.Theme.LogoURL
			configMap["primary_color"] = input.Theme.PrimaryColor
			configMap["background_color"] = input.Theme.BackgroundColor
			configMap["card_background_color"] = input.Theme.CardBackgroundColor
			configMap["text_color"] = input.Theme.TextColor
			configMap["accent_color"] = input.Theme.AccentColor
			configMap["font_family"] = input.Theme.FontFamily
		}

		// Merge navigation (if provided, replaces entire array)
		if len(input.Navigation) > 0 {
			configMap["navigation"] = input.Navigation
		}

		// Merge modules (if provided, replaces entire array)
		if len(input.Modules) > 0 {
			configMap["modules"] = input.Modules
		}

		// Merge lang (i18n text pack, if provided)
		if len(input.Lang) > 0 && string(input.Lang) != "null" {
			configMap["lang"] = json.RawMessage(input.Lang)
		}

		// Merge regions (editable system pages, if provided)
		if len(input.Regions) > 0 && string(input.Regions) != "null" && string(input.Regions) != "{}" {
			configMap["regions"] = json.RawMessage(input.Regions)
		}

		// Ensure editor version is set
		configMap["portal_editor_version"] = 1

		// 合规文档链接 ✓（nil = 不修改 ✓；空串 = 清除 ✓）
		if input.PrivacyPolicyURL != nil {
			configMap["privacy_policy_url"] = *input.PrivacyPolicyURL
		}
		if input.TermsOfServiceURL != nil {
			configMap["terms_of_service_url"] = *input.TermsOfServiceURL
		}
		if input.PrivacyPolicyContent != nil {
			configMap["privacy_policy_content"] = *input.PrivacyPolicyContent
		}
		if input.TermsOfServiceContent != nil {
			configMap["terms_of_service_content"] = *input.TermsOfServiceContent
		}

		// Serialize back to JSON string
		mergedConfig, marshalErr := json.Marshal(configMap)
		if marshalErr != nil {
			return nil, fmt.Errorf("failed to serialize merged config: %w", marshalErr)
		}

		// Validate and sanitize the merged config
		cleanedConfig, validateErr := ValidatePortalConfig(string(mergedConfig))
		if validateErr != nil {
			return nil, fmt.Errorf("config validation failed: %w", validateErr)
		}
		tmpl.Config = cleanedConfig
	}
	tmpl.UpdatedAt = time.Now()

	// 安全审计 L-2：UpdateActiveConfig 可直接改写 HTML/JS —— 落库前统一打安全补丁，
	// 防止编辑器/旧备份粘回 renderMd(msg.content) 之类的 XSS 写法。
	if newHTML, newJS, changed := PatchTemplateSecurity(tmpl.HTMLContent, tmpl.JSContent); changed {
		tmpl.HTMLContent = newHTML
		tmpl.JSContent = newJS
	}
	// regions.auth（登录卡区域）在 config 里 → 编辑器保存也一并补齐（自助解封卡 ✓）
	if newConfig, cfgChanged := PatchRegionsSecurity(tmpl.Config); cfgChanged {
		tmpl.Config = newConfig
	}

	if err := s.repo.Update(ctx, tmpl.PortalTemplate); err != nil {
		return nil, err
	}

	// Sync system-level settings
	if s.settingSvc != nil {
		if input.RegistrationEnabled != nil {
			val := "false"
			if *input.RegistrationEnabled {
				val = "true"
			}
			_ = s.settingSvc.SetByKey(ctx, "panel.registration_enabled", val)
			tmpl.RegistrationEnabled = *input.RegistrationEnabled
		}
		if input.PortalPort != nil {
			_ = s.settingSvc.SetByKey(ctx, "panel.portal_port", *input.PortalPort)
			tmpl.PortalPort = *input.PortalPort
		}
	}

	// Invalidate render cache since the active template changed
	s.cache.Invalidate()

	// Re-parse structured fields from the updated config
	parseConfigToSiteConfig(tmpl.Config, tmpl)

	return tmpl, nil
}

// ResetToDefault resets the active portal to the built-in default template.
func (s *Service) ResetToDefault(ctx context.Context) (*PortalSiteConfig, error) {
	tmpl, err := s.GetActiveConfig(ctx)
	if err != nil {
		return nil, err
	}
	tmpl.Name = "Default"
	tmpl.Description = "Built-in default portal template"
	tmpl.HTMLContent = defaultPortalHTML
	tmpl.CSSContent = defaultPortalCSS
	tmpl.JSContent = defaultPortalJS
	tmpl.Config = defaultStoredConfigJSON()
	tmpl.IsBuiltIn = true
	tmpl.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, tmpl.PortalTemplate); err != nil {
		return nil, err
	}

	// Invalidate render cache since the template was reset
	s.cache.Invalidate()

	// Re-parse structured fields from the reset config
	parseConfigToSiteConfig(tmpl.Config, tmpl)

	return tmpl, nil
}

// RenderConfig represents the configuration variables available in a portal template.
type RenderConfig struct {
	SiteName            string          `json:"site_name"`
	LogoURL             string          `json:"logo_url"`
	PrimaryColor        string          `json:"primary_color"`
	BackgroundColor     string          `json:"background_color"`
	CardBackgroundColor string          `json:"card_background_color"`
	TextColor           string          `json:"text_color"`
	AccentColor         string          `json:"accent_color"`
	FontFamily          string          `json:"font_family"`
	RegistrationEnabled bool            `json:"registration_enabled"`
	PortalEditorVersion int             `json:"portal_editor_version,omitempty"`
	Navigation          json.RawMessage `json:"navigation,omitempty"`
	Modules             json.RawMessage `json:"modules,omitempty"`
	Lang                json.RawMessage `json:"lang,omitempty"`
	Regions             json.RawMessage `json:"regions,omitempty"`
	// 合规文档链接 ✓（Google OAuth 验证要求登录页展示隐私政策/服务条款链接 ✓；空 = 不显示 ✓）
	PrivacyPolicyURL  string `json:"privacy_policy_url,omitempty"`
	TermsOfServiceURL string `json:"terms_of_service_url,omitempty"`
	// 合规文档**内容** ✓（按语言 markdown：{"zh-CN":"...","en":"..."}；
	// 登录卡片弹窗 + 独立页面 /legal/* 均使用 ✓；URL 与内容二选一 ✓，URL 优先 ✓）
	PrivacyPolicyContent  map[string]string `json:"privacy_policy_content,omitempty"`
	TermsOfServiceContent map[string]string `json:"terms_of_service_content,omitempty"`
}

// defaultStoredConfigJSON 返回「默认模板落库用」的 config JSON。
// 与 DefaultRenderConfig() 的区别：语言包已剥离与内置包逐字相同的文案（只留 diff），
// 体积从 ~506KB 降到 ~85KB —— 否则新建/重置的模板一落库就超过 maxConfigSize，
// 且内置文案升级对存量模板失效 ✗。渲染时由 MergeBuiltinLangFallback 补齐 ✓。
func defaultStoredConfigJSON() string {
	b, err := json.Marshal(DefaultRenderConfig())
	if err != nil {
		return ""
	}
	compact, _ := CompactConfigLang(string(b))
	return compact
}

// DefaultRenderConfig returns default render configuration.
func DefaultRenderConfig() RenderConfig {
	return RenderConfig{
		SiteName:              "CoreFusion",
		PrimaryColor:          "#667eea",
		BackgroundColor:       "#f0f2f5",
		CardBackgroundColor:   "#ffffff",
		TextColor:             "#303133",
		AccentColor:           "#67c23a",
		FontFamily:            "system-ui, -apple-system, sans-serif",
		PrivacyPolicyContent:  defaultLegalMap(defaultPrivacyPolicyContent, defaultPrivacyPolicyContentEN),
		TermsOfServiceContent: defaultLegalMap(defaultTermsOfServiceContent, defaultTermsOfServiceContentEN),
		PortalEditorVersion:   1,
		Navigation:            json.RawMessage(defaultNavigationJSON),
		Modules:               json.RawMessage(defaultModulesJSON),
		Lang:                  json.RawMessage(defaultMultiLangJSON),
		Regions:               json.RawMessage(defaultRegionsJSON),
	}
}

// GetPublicConfig returns the effective runtime configuration for the portal
// page. It is the same shape as the config injected into the page at render
// time (window.PORTAL_CONFIG), but always freshly read from the database.
// The portal runtime fetches this on boot so that builder changes take effect
// immediately without relying on the render cache.
func (s *Service) GetPublicConfig(ctx context.Context) (RenderConfig, error) {
	registrationEnabled := false
	if s.settingSvc != nil {
		if v, err := s.settingSvc.GetByKey(ctx, "panel.registration_enabled"); err == nil && v == "true" {
			registrationEnabled = true
		}
	}

	tmpl, err := s.repo.GetActive(ctx)
	if err != nil || tmpl == nil {
		// No active template — seed the built-in default and use it
		if err := s.SeedBuiltInTemplate(ctx); err != nil {
			return RenderConfig{}, fmt.Errorf("failed to seed default template: %w", err)
		}
		tmpl, err = s.repo.GetActive(ctx)
		if err != nil {
			return RenderConfig{}, err
		}
	}

	return MergeConfig(tmpl, registrationEnabled), nil
}

// GetPublicConfigJSON returns the portal runtime config as a ready-to-serve
// JSON response body ({"data": ...}). In production mode the marshaled body is
// served from the in-memory cache — the DB read + config merge + JSON marshal
// only happens again after a write invalidates the cache or the TTL elapses.
func (s *Service) GetPublicConfigJSON(ctx context.Context) ([]byte, error) {
	if body, ok := s.cache.GetConfig(); ok {
		return body, nil
	}

	cfg, err := s.GetPublicConfig(ctx)
	if err != nil {
		return nil, err
	}
	body, err := AssembleConfigResponse(cfg)
	if err != nil {
		return nil, err
	}
	s.cache.SetConfig(body)
	return body, nil
}

// MergeConfig merges a template's stored config JSON with defaults.
// It preserves navigation and modules fields for the portal runtime.
func MergeConfig(tmpl *entity.PortalTemplate, registrationEnabled bool) RenderConfig {
	cfg := DefaultRenderConfig()
	cfg.RegistrationEnabled = registrationEnabled

	if tmpl != nil && tmpl.Config != "" {
		var stored map[string]json.RawMessage
		if json.Unmarshal([]byte(tmpl.Config), &stored) == nil {
			if v, ok := stored["site_name"]; ok {
				var s string
				if json.Unmarshal(v, &s) == nil && s != "" {
					cfg.SiteName = s
				}
			}
			if v, ok := stored["logo_url"]; ok {
				var s string
				if json.Unmarshal(v, &s) == nil {
					cfg.LogoURL = s
				}
			}
			if v, ok := stored["primary_color"]; ok {
				var s string
				if json.Unmarshal(v, &s) == nil && s != "" {
					cfg.PrimaryColor = s
				}
			}
			if v, ok := stored["background_color"]; ok {
				var s string
				if json.Unmarshal(v, &s) == nil && s != "" {
					cfg.BackgroundColor = s
				}
			}
			if v, ok := stored["card_background_color"]; ok {
				var s string
				if json.Unmarshal(v, &s) == nil && s != "" {
					cfg.CardBackgroundColor = s
				}
			}
			if v, ok := stored["text_color"]; ok {
				var s string
				if json.Unmarshal(v, &s) == nil && s != "" {
					cfg.TextColor = s
				}
			}
			if v, ok := stored["accent_color"]; ok {
				var s string
				if json.Unmarshal(v, &s) == nil && s != "" {
					cfg.AccentColor = s
				}
			}
			if v, ok := stored["font_family"]; ok {
				var s string
				if json.Unmarshal(v, &s) == nil && s != "" {
					cfg.FontFamily = s
				}
			}
			// Pass through navigation and modules
			if v, ok := stored["navigation"]; ok && len(v) > 0 && string(v) != "null" {
				cfg.Navigation = v
			}
			if v, ok := stored["modules"]; ok && len(v) > 0 && string(v) != "null" {
				cfg.Modules = v
			}
			// Pass through portal_editor_version
			if v, ok := stored["portal_editor_version"]; ok {
				var n int
				if json.Unmarshal(v, &n) == nil && n > 0 {
					cfg.PortalEditorVersion = n
				}
			}
			// 合规文档链接 ✓（登录页展示 ✓）
			if v, ok := stored["privacy_policy_url"]; ok {
				var s string
				if json.Unmarshal(v, &s) == nil {
					cfg.PrivacyPolicyURL = s
				}
			}
			if v, ok := stored["terms_of_service_url"]; ok {
				var s string
				if json.Unmarshal(v, &s) == nil {
					cfg.TermsOfServiceURL = s
				}
			}
			if v, ok := stored["privacy_policy_content"]; ok {
				if m := parseLegalContent(v); m != nil {
					cfg.PrivacyPolicyContent = m
				}
			}
			if v, ok := stored["terms_of_service_content"]; ok {
				if m := parseLegalContent(v); m != nil {
					cfg.TermsOfServiceContent = m
				}
			}
			// Pass through lang (i18n multi-language pack)
			if v, ok := stored["lang"]; ok && len(v) > 0 && string(v) != "null" {
				// Check if it's the new multi-language format or old flat format
				// Both are passed through as-is; the frontend handles both
				// ⚠ 自定义语言包缺新增内置 key（如自助解封卡文案）时显示原始 key ✗
				// → 内置包作兜底层合并（自定义 key 优先，仅补缺失 ✓）
				cfg.Lang = MergeBuiltinLangFallback(v)
			} else {
				cfg.Lang = json.RawMessage(defaultMultiLangJSON)
			}
			// Pass through regions (editable system pages)
			if v, ok := stored["regions"]; ok && len(v) > 0 && string(v) != "null" && string(v) != "{}" {
				cfg.Regions = v
			} else {
				cfg.Regions = json.RawMessage(defaultRegionsJSON)
			}
		}
	}

	return cfg
}

// SeedBuiltInTemplate seeds the default built-in portal template if none exist.
func (s *Service) SeedBuiltInTemplate(ctx context.Context) error {
	templates, err := s.repo.List(ctx)
	if err != nil {
		return err
	}
	if len(templates) > 0 {
		return nil
	}

	defaultConfig := defaultStoredConfigJSON()

	tmpl := &entity.PortalTemplate{
		Name:        "Default",
		Description: "Built-in default portal template",
		HTMLContent: defaultPortalHTML,
		CSSContent:  defaultPortalCSS,
		JSContent:   defaultPortalJS,
		Config:      defaultConfig,
		IsActive:    true,
		IsBuiltIn:   true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	return s.repo.Create(ctx, tmpl)
}
