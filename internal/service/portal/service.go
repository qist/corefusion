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
type UpdateInput struct {
	Name                *string `json:"name"`
	Description         *string `json:"description"`
	HTMLContent         *string `json:"html_content"`
	CSSContent          *string `json:"css_content"`
	JSContent           *string `json:"js_content"`
	Config              *string `json:"config"`
	RegistrationEnabled *bool   `json:"registration_enabled"`
	PortalPort          *string `json:"portal_port"`
}

// PortalSiteConfig is the response for the active portal site.
// It wraps the template entity with extra system-level settings.
type PortalSiteConfig struct {
	*entity.PortalTemplate
	RegistrationEnabled bool   `json:"registration_enabled"`
	PortalPort          string `json:"portal_port"`
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
	tmpl := &entity.PortalTemplate{
		Name:        input.Name,
		Description: input.Description,
		HTMLContent: input.HTMLContent,
		CSSContent:  input.CSSContent,
		JSContent:   input.JSContent,
		Config:      input.Config,
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
		tmpl.Config = *input.Config
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

	return cfg, nil
}

// UpdateActiveConfig updates the currently active portal configuration.
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
	if input.Config != nil {
		tmpl.Config = *input.Config
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
	return tmpl, nil
}

// RenderConfig represents the configuration variables available in a portal template.
type RenderConfig struct {
	SiteName            string `json:"site_name"`
	LogoURL             string `json:"logo_url"`
	PrimaryColor        string `json:"primary_color"`
	BackgroundColor     string `json:"background_color"`
	CardBackgroundColor string `json:"card_background_color"`
	TextColor           string `json:"text_color"`
	AccentColor         string `json:"accent_color"`
	FontFamily          string `json:"font_family"`
	RegistrationEnabled bool   `json:"registration_enabled"`
}

// DefaultRenderConfig returns default render configuration.
func DefaultRenderConfig() RenderConfig {
	return RenderConfig{
		SiteName:            "CoreFusion",
		PrimaryColor:        "#409eff",
		BackgroundColor:     "#f5f7fa",
		CardBackgroundColor: "#ffffff",
		TextColor:           "#303133",
		AccentColor:         "#67c23a",
		FontFamily:          "-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif",
	}
}

// MergeConfig merges a template's stored config JSON with defaults.
func MergeConfig(tmpl *entity.PortalTemplate, registrationEnabled bool) RenderConfig {
	cfg := DefaultRenderConfig()
	cfg.RegistrationEnabled = registrationEnabled

	if tmpl != nil && tmpl.Config != "" {
		var stored map[string]interface{}
		if json.Unmarshal([]byte(tmpl.Config), &stored) == nil {
			if v, ok := stored["site_name"].(string); ok && v != "" {
				cfg.SiteName = v
			}
			if v, ok := stored["logo_url"].(string); ok {
				cfg.LogoURL = v
			}
			if v, ok := stored["primary_color"].(string); ok && v != "" {
				cfg.PrimaryColor = v
			}
			if v, ok := stored["background_color"].(string); ok && v != "" {
				cfg.BackgroundColor = v
			}
			if v, ok := stored["card_background_color"].(string); ok && v != "" {
				cfg.CardBackgroundColor = v
			}
			if v, ok := stored["text_color"].(string); ok && v != "" {
				cfg.TextColor = v
			}
			if v, ok := stored["accent_color"].(string); ok && v != "" {
				cfg.AccentColor = v
			}
			if v, ok := stored["font_family"].(string); ok && v != "" {
				cfg.FontFamily = v
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
