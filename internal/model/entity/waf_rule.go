package entity

import "time"

// WafRule is the GORM model for the waf_rules table.
// Stores WAF rule files (args.rule, url.rule, blackip.rule, etc.)
// shared by both NginxGuard and CaddyGuard.
//
// Multi-level rules (three layers, merged in order):
//   - rule_scope="global", server_id=0, domain=""     → all nodes, all domains
//   - rule_scope="node",   server_id=N, domain=""     → only node N, all domains on it
//   - rule_scope="domain", server_id=N, domain="xxx"  → only domain xxx on node N
//
// Merge modes (how a layer combines with its parent):
//   - merge_mode="append"   → content appended to parent (e.g. blackip.rule)
//   - merge_mode="replace"  → content replaces parent entirely (e.g. args.rule)
//
// Merge order: global → node → domain
//   For a given domain on a given node, each rule file is resolved by
//   taking the global content, then applying node-level (append/replace),
//   then applying domain-level (append/replace).
//
// Users can create any rule file name, not limited to the default set.
type WafRule struct {
	ID        int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Name      string    `json:"name" gorm:"size:128;not null"`                      // e.g. "args.rule", "blackip.rule"
	RuleScope string    `json:"rule_scope" gorm:"size:16;not null;default:global"`  // global | node | domain
	ServerID  int64     `json:"server_id" gorm:"not null;default:0;index"`          // 0=global, >0=node-specific
	Domain    string    `json:"domain" gorm:"size:255;not null;default:''"`         // empty for global/node, domain name for domain-scope
	MergeMode string    `json:"merge_mode" gorm:"size:16;not null;default:replace"`  // append | replace
	Content   string    `json:"content" gorm:"type:longtext"`                       // rule content
	Hash      string    `json:"hash" gorm:"size:64;index"`                         // SHA-256 of content
	Enabled   bool      `json:"enabled" gorm:"not null;default:true"`
	UpdatedAt time.Time `json:"updated_at" gorm:"not null"`
}

func (WafRule) TableName() string { return "waf_rules" }

// WafConfig is the GORM model for the waf_configs table.
// Stores WAF configuration (config.json for Caddy, config.lua for Nginx).
// server_id=0 means global default; server_id>0 means per-node override.
type WafConfig struct {
	ID        int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	ServerID  int64     `json:"server_id" gorm:"not null;default:0;index"` // 0=global, >0=per-node
	ProxyType string    `json:"proxy_type" gorm:"size:16;not null"`         // "caddy" or "nginx"
	Content   string    `json:"content" gorm:"type:longtext"`               // config.json or config.lua content
	Hash      string    `json:"hash" gorm:"size:64"`
	UpdatedAt time.Time `json:"updated_at" gorm:"not null"`
}

func (WafConfig) TableName() string { return "waf_configs" }

// WafDomainConfig is the GORM model for the waf_domain_configs table.
// Stores per-domain WAF configuration (equivalent to entries in domain.json).
type WafDomainConfig struct {
	ID        int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	ServerID  int64     `json:"server_id" gorm:"not null;default:0;index"` // 0=global, >0=per-node
	Domain    string    `json:"domain" gorm:"size:255;not null"`
	Config    string    `json:"config" gorm:"type:longtext"` // JSON config for this domain
	UpdatedAt time.Time `json:"updated_at" gorm:"not null"`
}

func (WafDomainConfig) TableName() string { return "waf_domain_configs" }
