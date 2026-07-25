package portal

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// RenderCache provides an in-memory cache for the compiled portal HTML page.
//
// In production mode (default), the fully-rendered HTML is cached and served
// directly on every request without touching the database — near-zero overhead.
// The cache is automatically invalidated when the template config, HTML/CSS/JS
// content, or registration_enabled flag changes.
//
// In dev mode (PORTAL_DEV_MODE=true), the cache is bypassed entirely so every
// request re-renders from the database — useful for live editing and testing.
type RenderCache struct {
	// cachedHTML is the fully rendered HTML page (after variable substitution,
	// CSS injection, PORTAL_CONFIG injection).
	cachedHTML string
	// cacheKey is the hash of all inputs that affect the rendered output.
	// If this changes, the cache is considered stale.
	cacheKey string
	// builtAt records when the cache was last populated.
	builtAt time.Time

	// hitCount / missCount are for diagnostics (exposed via API).
	hitCount  int64
	missCount int64

	// devMode controls whether caching is active.
	// When true, every Get() returns a miss, forcing re-render.
	devMode bool

	mu sync.RWMutex
}

// NewRenderCache creates a new RenderCache instance.
// Dev mode is auto-detected from the PORTAL_DEV_MODE environment variable.
func NewRenderCache() *RenderCache {
	dev := false
	switch strings.ToLower(os.Getenv("PORTAL_DEV_MODE")) {
	case "true", "1", "yes", "on":
		dev = true
	}
	return &RenderCache{devMode: dev}
}

// cacheInput captures every field that can change the rendered portal page.
// If any of these differ from when the cache was built, it's a miss.
type cacheInput struct {
	TemplateID          int64
	UpdatedAt           time.Time
	RegistrationEnabled bool
	HTMLContent         string
	CSSContent          string
	JSContent           string
	Config              string
}

// computeKey derives a deterministic cache key from the inputs.
func (ci cacheInput) computeKey() string {
	h := sha256.New()
	fmt.Fprintf(h, "%d|%d|%v", ci.TemplateID, ci.UpdatedAt.UnixNano(), ci.RegistrationEnabled)
	h.Write([]byte(ci.HTMLContent))
	h.Write([]byte(ci.CSSContent))
	h.Write([]byte(ci.JSContent))
	h.Write([]byte(ci.Config))
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// Get returns the cached HTML if the cache key matches and we are not in dev mode.
// Returns ok=false on miss (caller should rebuild).
func (rc *RenderCache) Get(key string) (html string, ok bool) {
	if rc.IsDevMode() {
		atomic.AddInt64(&rc.missCount, 1)
		return "", false
	}

	rc.mu.RLock()
	defer rc.mu.RUnlock()

	if rc.cacheKey == key && rc.cachedHTML != "" {
		atomic.AddInt64(&rc.hitCount, 1)
		return rc.cachedHTML, true
	}

	atomic.AddInt64(&rc.missCount, 1)
	return "", false
}

// Set stores the rendered HTML with the given key.
func (rc *RenderCache) Set(key, html string) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.cachedHTML = html
	rc.cacheKey = key
	rc.builtAt = time.Now()
}

// Invalidate clears the cache, forcing a rebuild on next request.
func (rc *RenderCache) Invalidate() {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.cachedHTML = ""
	rc.cacheKey = ""
}

// IsDevMode returns whether dev mode (no caching) is active.
func (rc *RenderCache) IsDevMode() bool {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.devMode
}

// SetDevMode enables or disables dev mode at runtime (via API).
func (rc *RenderCache) SetDevMode(dev bool) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.devMode = dev
	// Clear cache so the mode change takes effect immediately
	rc.cachedHTML = ""
	rc.cacheKey = ""
}

// Stats returns diagnostic information about the cache.
type CacheStats struct {
	Cached       bool   `json:"cached"`
	CacheKey     string `json:"cache_key,omitempty"`
	BuiltAt      string `json:"built_at,omitempty"`
	HitCount     int64  `json:"hit_count"`
	MissCount    int64  `json:"miss_count"`
	DevMode      bool   `json:"dev_mode"`
}

// Stats returns current cache statistics for the admin API.
func (rc *RenderCache) Stats() CacheStats {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	return CacheStats{
		Cached:    rc.cachedHTML != "",
		CacheKey:  rc.cacheKey,
		BuiltAt:   func() string { if rc.builtAt.IsZero() { return "" }; return rc.builtAt.Format(time.RFC3339) }(),
		HitCount:  atomic.LoadInt64(&rc.hitCount),
		MissCount: atomic.LoadInt64(&rc.missCount),
		DevMode:   rc.devMode,
	}
}

// renderPortalHTML performs the actual rendering: variable substitution,
// CSS injection, PORTAL_CONFIG injection. This is the "compile" step.
// It is called on cache miss.
func renderPortalHTML(htmlContent, cssContent, jsContent, config string, cfg RenderConfig) string {
	html := htmlContent

	// --- HTML variable substitution ---
	html = strings.ReplaceAll(html, "{{.PortalName}}", htmlEscape(cfg.SiteName))
	html = strings.ReplaceAll(html, "{{.SiteName}}", htmlEscape(cfg.SiteName))
	html = strings.ReplaceAll(html, "{{.LogoURL}}", htmlEscape(cfg.LogoURL))
	html = strings.ReplaceAll(html, "{{.RegistrationEnabled}}", fmt.Sprintf("%v", cfg.RegistrationEnabled))
	html = strings.ReplaceAll(html, "{{registrationEnabledText}}", map[bool]string{true: "", false: ""}[cfg.RegistrationEnabled])

	// --- CSS variable substitution ---
	css := cssContent
	css = strings.ReplaceAll(css, "{{.FontFamily}}", cfg.FontFamily)
	css = strings.ReplaceAll(css, "{{.BackgroundColor}}", cfg.BackgroundColor)
	css = strings.ReplaceAll(css, "{{.CardBackgroundColor}}", cfg.CardBackgroundColor)
	css = strings.ReplaceAll(css, "{{.TextColor}}", cfg.TextColor)
	css = strings.ReplaceAll(css, "{{.PrimaryColor}}", cfg.PrimaryColor)
	css = strings.ReplaceAll(css, "{{.AccentColor}}", cfg.AccentColor)

	// --- Inject CSS ---
	if css != "" {
		html = strings.Replace(html, `<style id="portal-custom-css"></style>`,
			`<style id="portal-custom-css">`+css+`</style>`, 1)
	}

	// --- Inject PORTAL_CONFIG as JSON ---
	configJSON, _ := json.Marshal(cfg)
	html = strings.Replace(html, `<script id="portal-config"></script>`,
		`<script id="portal-config">window.PORTAL_CONFIG=`+string(configJSON)+`;if(PORTAL_CONFIG.logo_url){document.getElementById('portal-favicon').href=PORTAL_CONFIG.logo_url;}</script>`, 1)

	// --- Inject custom JS ---
	if jsContent != "" {
		html = strings.Replace(html, `<script id="portal-custom-js"></script>`,
			`<script id="portal-custom-js">`+jsContent+`</script>`, 1)
	}

	return html
}

// htmlEscape sanitizes a string for safe inclusion in HTML.
func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&#34;")
	return s
}
