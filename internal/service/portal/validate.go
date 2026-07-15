package portal

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Allowed module types — only these can be rendered by the portal template.
var allowedModuleTypes = map[string]bool{
	"hero":          true,
	"markdown":      true,
	"rich_text":     true,
	"notice":        true,
	"feature_cards": true,
	"account_stats": true,
	"subscription":  true,
	"custom_html":   true,
}

// Allowed navigation targets for internal pages.
var allowedNavTargets = map[string]bool{
	"overview":     true,
	"subscription": true,
	"security":     true,
	"logs":         true,
}

const (
	maxModules       = 20 // maximum number of modules allowed
	maxNavigation    = 10 // maximum number of navigation items allowed
	maxConfigSize    = 200 * 1024 // 200KB max config JSON size
	maxHTMLContent   = 50000      // max characters for HTML/Markdown content in a single module
	maxCardsPerModule = 6         // max feature cards per module
)

// dangerousScriptRe matches script tags and event handler attributes.
var dangerousScriptRe = regexp.MustCompile(`(?i)<\s*script[^>]*>[\s\S]*?<\s*/\s*script\s*>`)
var dangerousAttrRe = regexp.MustCompile(`(?i)\s+on\w+\s*=\s*["'][^"']*["']`)
var javascriptURLRe = regexp.MustCompile(`(?i)^javascript\s*:`)

// ValidatePortalConfig validates the portal config JSON string.
// It enforces type whitelists, size limits, and content safety rules.
// Returns cleaned config JSON or an error.
func ValidatePortalConfig(configStr string) (string, error) {
	if len(configStr) > maxConfigSize {
		return "", fmt.Errorf("portal config too large: %d bytes (max %d)", len(configStr), maxConfigSize)
	}

	var cfg map[string]json.RawMessage
	if err := json.Unmarshal([]byte(configStr), &cfg); err != nil {
		return "", fmt.Errorf("invalid portal config JSON: %w", err)
	}

	// Validate and clean navigation
	if navRaw, ok := cfg["navigation"]; ok {
		cleaned, err := validateNavigation(navRaw)
		if err != nil {
			return "", fmt.Errorf("navigation validation failed: %w", err)
		}
		cfg["navigation"] = cleaned
	}

	// Validate and clean modules
	if modRaw, ok := cfg["modules"]; ok {
		cleaned, err := validateModules(modRaw)
		if err != nil {
			return "", fmt.Errorf("modules validation failed: %w", err)
		}
		cfg["modules"] = cleaned
	}

	// Sanitize HTML content in modules
	if modRaw, ok := cfg["modules"]; ok {
		cleaned, err := sanitizeModuleHTML(modRaw)
		if err != nil {
			return "", fmt.Errorf("module content sanitization failed: %w", err)
		}
		cfg["modules"] = cleaned
	}

	// Ensure portal_editor_version is present and valid
	if v, ok := cfg["portal_editor_version"]; ok {
		var n int
		if json.Unmarshal(v, &n) != nil || n < 1 {
			cfg["portal_editor_version"] = json.RawMessage(`1`)
		}
	} else {
		cfg["portal_editor_version"] = json.RawMessage(`1`)
	}

	result, err := json.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("failed to marshal cleaned config: %w", err)
	}
	return string(result), nil
}

// validateNavigation validates navigation items.
func validateNavigation(navRaw json.RawMessage) (json.RawMessage, error) {
	var items []map[string]interface{}
	if err := json.Unmarshal(navRaw, &items); err != nil {
		return nil, fmt.Errorf("invalid navigation array: %w", err)
	}

	if len(items) > maxNavigation {
		return nil, fmt.Errorf("too many navigation items: %d (max %d)", len(items), maxNavigation)
	}

	for i, item := range items {
		// Validate label
		label, _ := item["label"].(string)
		if strings.TrimSpace(label) == "" {
			return nil, fmt.Errorf("navigation item %d: label is required", i)
		}

		// Validate target if present
		if target, ok := item["target"].(string); ok && target != "" {
			if !allowedNavTargets[target] {
				// Allow if it has an external href instead
				if href, _ := item["href"].(string); href == "" {
					return nil, fmt.Errorf("navigation item %d: invalid target %q", i, target)
				}
			}
		}

		// Validate href if present — block javascript: URLs
		if href, ok := item["href"].(string); ok && href != "" {
			if javascriptURLRe.MatchString(href) {
				return nil, fmt.Errorf("navigation item %d: javascript: URLs are not allowed", i)
			}
		}

		// Sanitize label and icon — strip any HTML tags
		item["label"] = stripHTMLTags(label)
		if icon, ok := item["icon"].(string); ok {
			item["icon"] = stripHTMLTags(icon)
		}
	}

	result, err := json.Marshal(items)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal navigation: %w", err)
	}
	return result, nil
}

// validateModules validates module items.
func validateModules(modRaw json.RawMessage) (json.RawMessage, error) {
	var modules []map[string]interface{}
	if err := json.Unmarshal(modRaw, &modules); err != nil {
		return nil, fmt.Errorf("invalid modules array: %w", err)
	}

	if len(modules) > maxModules {
		return nil, fmt.Errorf("too many modules: %d (max %d)", len(modules), maxModules)
	}

	for i, mod := range modules {
		// Validate type against whitelist
		modType, _ := mod["type"].(string)
		if !allowedModuleTypes[modType] {
			return nil, fmt.Errorf("module %d: unsupported type %q (allowed: %s)", i, modType, joinAllowedTypes())
		}

		// Validate title — strip HTML
		if title, ok := mod["title"].(string); ok {
			mod["title"] = stripHTMLTags(title)
		}

		// Module-specific validation
		switch modType {
		case "hero":
			if err := validateHeroModule(mod); err != nil {
				return nil, fmt.Errorf("module %d (hero): %w", i, err)
			}
		case "feature_cards":
			if err := validateFeatureCardsModule(mod); err != nil {
				return nil, fmt.Errorf("module %d (feature_cards): %w", i, err)
			}
		case "rich_text", "custom_html":
			if err := validateHTMLModule(mod); err != nil {
				return nil, fmt.Errorf("module %d (%s): %w", i, modType, err)
			}
		case "markdown", "notice":
			if err := validateMarkdownModule(mod); err != nil {
				return nil, fmt.Errorf("module %d (%s): %w", i, modType, err)
			}
		}
	}

	result, err := json.Marshal(modules)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal modules: %w", err)
	}
	return result, nil
}

// validateHeroModule validates hero module data fields.
func validateHeroModule(mod map[string]interface{}) error {
	data, ok := mod["data"].(map[string]interface{})
	if !ok {
		return nil // data is optional
	}
	// Strip HTML from text fields
	for _, key := range []string{"eyebrow", "heading", "description", "button_text"} {
		if v, ok := data[key].(string); ok {
			data[key] = stripHTMLTags(v)
		}
	}
	// Validate button_target
	if target, ok := data["button_target"].(string); ok && target != "" {
		if !allowedNavTargets[target] {
			data["button_target"] = "subscription" // safe default
		}
	}
	return nil
}

// validateFeatureCardsModule validates feature cards module.
func validateFeatureCardsModule(mod map[string]interface{}) error {
	data, ok := mod["data"].(map[string]interface{})
	if !ok {
		return nil
	}
	cards, ok := data["cards"].([]interface{})
	if !ok {
		return nil
	}
	if len(cards) > maxCardsPerModule {
		data["cards"] = cards[:maxCardsPerModule] // truncate
	}
	// Strip HTML from card text fields
	for _, c := range data["cards"].([]interface{}) {
		if card, ok := c.(map[string]interface{}); ok {
			for _, key := range []string{"title", "description"} {
				if v, ok := card[key].(string); ok {
					card[key] = stripHTMLTags(v)
				}
			}
			if icon, ok := card["icon"].(string); ok {
				card["icon"] = stripHTMLTags(icon)
			}
		}
	}
	return nil
}

// validateHTMLModule validates HTML content size.
func validateHTMLModule(mod map[string]interface{}) error {
	data, ok := mod["data"].(map[string]interface{})
	if !ok {
		return nil
	}
	if html, ok := data["html"].(string); ok {
		if len(html) > maxHTMLContent {
			return fmt.Errorf("HTML content too large: %d chars (max %d)", len(html), maxHTMLContent)
		}
	}
	return nil
}

// validateMarkdownModule validates markdown content size.
func validateMarkdownModule(mod map[string]interface{}) error {
	data, ok := mod["data"].(map[string]interface{})
	if !ok {
		return nil
	}
	if md, ok := data["markdown"].(string); ok {
		if len(md) > maxHTMLContent {
			return fmt.Errorf("markdown content too large: %d chars (max %d)", len(md), maxHTMLContent)
		}
	}
	if title, ok := data["title"].(string); ok {
		data["title"] = stripHTMLTags(title)
	}
	return nil
}

// sanitizeModuleHTML removes dangerous HTML from module content.
// This is a backend-level safety net — the frontend also sanitizes with DOMPurify.
func sanitizeModuleHTML(modRaw json.RawMessage) (json.RawMessage, error) {
	var modules []map[string]interface{}
	if err := json.Unmarshal(modRaw, &modules); err != nil {
		return modRaw, nil // return as-is if can't parse
	}

	for _, mod := range modules {
		data, ok := mod["data"].(map[string]interface{})
		if !ok {
			continue
		}
		// Clean HTML fields
		if html, ok := data["html"].(string); ok {
			data["html"] = sanitizeHTMLString(html)
		}
		// Markdown is rendered client-side; we strip script tags as a safety net
		if md, ok := data["markdown"].(string); ok {
			data["markdown"] = sanitizeMarkdownString(md)
		}
	}

	result, err := json.Marshal(modules)
	if err != nil {
		return modRaw, nil
	}
	return result, nil
}

// sanitizeHTMLString removes dangerous elements from HTML strings.
// This is a simple backend sanitizer — DOMPurify handles the real filtering client-side.
func sanitizeHTMLString(html string) string {
	// Remove <script> tags and their content
	html = dangerousScriptRe.ReplaceAllString(html, "")
	// Remove event handler attributes (onclick, onload, etc.)
	html = dangerousAttrRe.ReplaceAllString(html, "")
	return html
}

// sanitizeMarkdownString removes dangerous content from Markdown.
// Markdown itself can't contain executable scripts, but embedded HTML can.
func sanitizeMarkdownString(md string) string {
	// Remove <script> tags if someone embeds HTML in Markdown
	md = dangerousScriptRe.ReplaceAllString(md, "")
	// Remove event handler attributes
	md = dangerousAttrRe.ReplaceAllString(md, "")
	return md
}

// stripHTMLTags removes all HTML tags from a string, returning plain text.
var htmlTagRe = regexp.MustCompile(`<[^>]*>`)

func stripHTMLTags(s string) string {
	return htmlTagRe.ReplaceAllString(s, "")
}

// joinAllowedTypes returns a comma-separated list of allowed module types.
func joinAllowedTypes() string {
	types := make([]string, 0, len(allowedModuleTypes))
	for t := range allowedModuleTypes {
		types = append(types, t)
	}
	return strings.Join(types, ", ")
}
