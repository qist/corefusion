package entity

import "time"

// PortalTemplate stores a customizable portal page template.
// The portal is a self-service web page where subscription users can
// log in, view their traffic usage, manage subscriptions, etc.
type PortalTemplate struct {
	ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string    `gorm:"size:255;not null" json:"name"`
	Description string    `gorm:"size:500" json:"description"`
	// HTMLContent is the full HTML page (with template variables like {{.SiteName}}).
	HTMLContent string `gorm:"type:longtext" json:"html_content"`
	// CSSContent is custom CSS injected into <style> tags.
	CSSContent string `gorm:"type:longtext" json:"css_content"`
	// JSContent is custom JS injected into <script> tags.
	JSContent string `gorm:"type:longtext" json:"js_content"`
	// Config is a JSON object with template variables (site_name, logo_url, primary_color, etc.).
	Config string `gorm:"type:longtext" json:"config"`
	// IsActive marks the currently active template (only one can be active).
	IsActive bool `gorm:"not null;default:false" json:"is_active"`
	// IsBuiltIn marks system-provided templates (cannot be deleted).
	IsBuiltIn bool `gorm:"not null;default:false" json:"is_built_in"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (PortalTemplate) TableName() string { return "portal_templates" }
