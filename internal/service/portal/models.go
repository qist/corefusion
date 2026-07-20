package portal

import "encoding/json"

// NavigationItem represents a single navigation entry in the portal.
type NavigationItem struct {
	ID           string `json:"id"`
	Label        string `json:"label"`
	Icon         string `json:"icon"`
	Target       string `json:"target"`
	Href         string `json:"href"`
	Enabled      bool   `json:"enabled"`
	OpenExternal bool   `json:"open_external"`
}

// ModuleItem represents a single content module in the portal.
// Page links the module to a navigation target (e.g. "overview", "subscription").
type ModuleItem struct {
	ID      string                 `json:"id"`
	Type    string                 `json:"type"`
	Title   string                 `json:"title"`
	Page    string                 `json:"page"` // which nav page this module belongs to
	Visible bool                   `json:"visible"`
	Data    map[string]interface{} `json:"data"`
	Style   map[string]interface{} `json:"style"`
}

// PortalThemeConfig groups the visual/theme fields of the portal.
type PortalThemeConfig struct {
	SiteName            string `json:"site_name"`
	LogoURL             string `json:"logo_url"`
	PrimaryColor        string `json:"primary_color"`
	BackgroundColor     string `json:"background_color"`
	CardBackgroundColor string `json:"card_background_color"`
	TextColor           string `json:"text_color"`
	AccentColor         string `json:"accent_color"`
	FontFamily          string `json:"font_family"`
}

// RegionSource represents an editable portal region (e.g. auth/login page).
// Each region has HTML (Vue template), CSS, JS (setup function), and Data (configurable properties).
type RegionSource struct {
	HTML string                 `json:"html"`
	CSS  string                 `json:"css"`
	JS   string                 `json:"js"`
	Data map[string]interface{} `json:"data"`
}

// ParseNavigationItems parses a json.RawMessage into NavigationItem slice.
func ParseNavigationItems(raw json.RawMessage) []NavigationItem {
	if len(raw) == 0 || string(raw) == "null" || string(raw) == "[]" {
		return []NavigationItem{}
	}
	var items []NavigationItem
	if err := json.Unmarshal(raw, &items); err != nil {
		return []NavigationItem{}
	}
	return items
}

// ParseModuleItems parses a json.RawMessage into ModuleItem slice.
func ParseModuleItems(raw json.RawMessage) []ModuleItem {
	if len(raw) == 0 || string(raw) == "null" || string(raw) == "[]" {
		return []ModuleItem{}
	}
	var items []ModuleItem
	if err := json.Unmarshal(raw, &items); err != nil {
		return []ModuleItem{}
	}
	return items
}

// NavigationItemsToRaw converts NavigationItem slice to json.RawMessage.
func NavigationItemsToRaw(items []NavigationItem) json.RawMessage {
	if items == nil {
		return json.RawMessage(`[]`)
	}
	data, _ := json.Marshal(items)
	return data
}

// ModuleItemsToRaw converts ModuleItem slice to json.RawMessage.
func ModuleItemsToRaw(items []ModuleItem) json.RawMessage {
	if items == nil {
		return json.RawMessage(`[]`)
	}
	data, _ := json.Marshal(items)
	return data
}

// ParseRegions parses a json.RawMessage into a map of RegionSource.
func ParseRegions(raw json.RawMessage) map[string]RegionSource {
	if len(raw) == 0 || string(raw) == "null" || string(raw) == "{}" {
		return map[string]RegionSource{}
	}
	var regions map[string]RegionSource
	if err := json.Unmarshal(raw, &regions); err != nil {
		return map[string]RegionSource{}
	}
	return regions
}
