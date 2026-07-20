package portal

import (
	"context"
	"encoding/json"
	"fmt"
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
}

// NewService creates a new portal template service.
func NewService(repo *repository.PortalTemplateRepository) *Service {
	return &Service{repo: repo}
}

// WithSettingStore injects a setting store for managing registration_enabled / portal_port.
func (s *Service) WithSettingStore(ss SettingStore) *Service {
	s.settingSvc = ss
	return s
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
	Name                *string          `json:"name"`
	Description         *string          `json:"description"`
	HTMLContent         *string          `json:"html_content"`
	CSSContent          *string          `json:"css_content"`
	JSContent           *string          `json:"js_content"`
	Config              *string          `json:"config"`
	RegistrationEnabled *bool            `json:"registration_enabled"`
	PortalPort          *string          `json:"portal_port"`

	// Structured fields — when provided, merged into config JSON
	Navigation []NavigationItem `json:"navigation"`
	Modules    []ModuleItem     `json:"modules"`
	Theme      *PortalThemeConfig `json:"theme"`
	Lang       json.RawMessage   `json:"lang"`
	Regions    json.RawMessage   `json:"regions"`
}

// PortalSiteConfig is the response for the active portal site.
// It wraps the template entity with extra system-level settings and
// structured navigation/modules/theme fields parsed from the config JSON.
type PortalSiteConfig struct {
	*entity.PortalTemplate
	RegistrationEnabled bool              `json:"registration_enabled"`
	PortalPort          string            `json:"portal_port"`

	// Structured fields parsed from the config JSON string.
	// These are read-only in the response; to update them, use
	// the navigation/modules/theme fields in UpdateInput.
	SiteName            string            `json:"site_name"`
	LogoURL             string            `json:"logo_url"`
	PrimaryColor        string            `json:"primary_color"`
	BackgroundColor     string            `json:"background_color"`
	CardBackgroundColor string            `json:"card_background_color"`
	TextColor           string            `json:"text_color"`
	AccentColor         string            `json:"accent_color"`
	FontFamily          string            `json:"font_family"`
	PortalEditorVersion int               `json:"portal_editor_version"`
	Navigation          []NavigationItem  `json:"navigation"`
	Modules             []ModuleItem       `json:"modules"`
	Lang                json.RawMessage   `json:"lang"`
	Regions             json.RawMessage   `json:"regions"`
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
		"site_name":              &cfg.SiteName,
		"logo_url":              &cfg.LogoURL,
		"primary_color":         &cfg.PrimaryColor,
		"background_color":      &cfg.BackgroundColor,
		"card_background_color": &cfg.CardBackgroundColor,
		"text_color":             &cfg.TextColor,
		"accent_color":          &cfg.AccentColor,
		"font_family":           &cfg.FontFamily,
	}
	for key, ptr := range stringFields {
		if v, ok := stored[key]; ok {
			var s string
			if json.Unmarshal(v, &s) == nil {
				*ptr = s
			}
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
	if v, ok := stored["lang"]; ok && len(v) > 0 && string(v) != "null" {
		cfg.Lang = v
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
	tmpl := &entity.PortalTemplate{
		Name:        input.Name,
		Description: input.Description,
		HTMLContent: input.HTMLContent,
		CSSContent:  input.CSSContent,
		JSContent:   input.JSContent,
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

	if err := s.repo.Update(ctx, tmpl); err != nil {
		return nil, err
	}
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
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		return err
	}
	return s.repo.Activate(ctx, id)
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

	// Parse the config JSON string into structured fields
	parseConfigToSiteConfig(tmpl.Config, cfg)

	return cfg, nil
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
	if input.Config != nil || len(input.Navigation) > 0 || len(input.Modules) > 0 || input.Theme != nil || len(input.Regions) > 0 {
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
	defaultConfig, _ := json.Marshal(DefaultRenderConfig())
	tmpl.Name = "Default"
	tmpl.Description = "Built-in default portal template"
	tmpl.HTMLContent = defaultPortalHTML
	tmpl.CSSContent = defaultPortalCSS
	tmpl.JSContent = defaultPortalJS
	tmpl.Config = string(defaultConfig)
	tmpl.IsBuiltIn = true
	tmpl.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, tmpl.PortalTemplate); err != nil {
		return nil, err
	}

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
	Navigation         json.RawMessage `json:"navigation,omitempty"`
	Modules             json.RawMessage `json:"modules,omitempty"`
	Lang                json.RawMessage `json:"lang,omitempty"`
	Regions             json.RawMessage `json:"regions,omitempty"`
}

// DefaultRenderConfig returns default render configuration.
func DefaultRenderConfig() RenderConfig {
	return RenderConfig{
		SiteName:            "CoreFusion",
		PrimaryColor:        "#667eea",
		BackgroundColor:     "#f0f2f5",
		CardBackgroundColor: "#ffffff",
		TextColor:           "#303133",
		AccentColor:         "#67c23a",
		FontFamily:          "system-ui, -apple-system, sans-serif",
		PortalEditorVersion: 1,
		Navigation:         json.RawMessage(defaultNavigationJSON),
		Modules:             json.RawMessage(defaultModulesJSON),
		Lang:                json.RawMessage(defaultMultiLangJSON),
		Regions:             json.RawMessage(defaultRegionsJSON),
	}
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
		// Pass through lang (i18n multi-language pack)
		if v, ok := stored["lang"]; ok && len(v) > 0 && string(v) != "null" {
			// Check if it's the new multi-language format or old flat format
			// Both are passed through as-is; the frontend handles both
			cfg.Lang = v
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

	defaultConfig, _ := json.Marshal(DefaultRenderConfig())

	tmpl := &entity.PortalTemplate{
		Name:        "Default",
		Description: "Built-in default portal template",
		HTMLContent: defaultPortalHTML,
		CSSContent:  defaultPortalCSS,
		JSContent:   defaultPortalJS,
		Config:      string(defaultConfig),
		IsActive:    true,
		IsBuiltIn:   true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	return s.repo.Create(ctx, tmpl)
}
