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
type ModuleItem struct {
	ID     string                 `json:"id"`
	Type   string                 `json:"type"`
	Title  string                 `json:"title"`
	Visible bool                  `json:"visible"`
	Data   map[string]interface{} `json:"data"`
	Style  map[string]interface{} `json:"style"`
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
