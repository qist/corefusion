package portal

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"
)

// cacheTTL bounds how long a cached entry may be served without checking the
// database. Same-instance writes invalidate instantly; this TTL is the safety
// net for changes that bypass the service (direct DB edits, or writes served
// by the OTHER service instance — the main panel engine and the standalone
// portal engine each hold their own cache).
const cacheTTL = 30 * time.Second

// CacheStats represents cache performance metrics.
type CacheStats struct {
	Cached       bool      `json:"cached"`
	CacheKey     string    `json:"cache_key"`
	BuiltAt      time.Time `json:"built_at"`
	HitCount     int64     `json:"hit_count"`
	MissCount    int64     `json:"miss_count"`
	DevMode      bool      `json:"dev_mode"`
	ConfigCached bool      `json:"config_cached"`
	ConfigHits   int64     `json:"config_hit_count"`
	ConfigMisses int64     `json:"config_miss_count"`
	TTLSeconds   int       `json:"ttl_seconds"`
}

// RenderCache holds the rendered portal page HTML and the lean runtime config
// JSON in memory. In production mode reads are served straight from memory
// (zero DB access, zero re-rendering) until a write invalidates the cache or
// the TTL elapses. Dev mode bypasses both caches entirely.
type RenderCache struct {
	cachedHTML string
	cacheKey   string
	builtAt    time.Time
	hitCount   atomic.Int64
	missCount  atomic.Int64

	configJSON    []byte
	configBuiltAt time.Time
	configHits    atomic.Int64
	configMisses  atomic.Int64

	devMode bool
	mu      sync.RWMutex
}

// NewRenderCache creates a new render cache.
func NewRenderCache() *RenderCache {
	return &RenderCache{}
}

// Get returns the cached HTML if it is fresh (and dev mode is off).
func (c *RenderCache) Get() (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.devMode || c.cachedHTML == "" {
		return "", false
	}
	if time.Since(c.builtAt) > cacheTTL {
		return "", false
	}
	c.hitCount.Add(1)
	return c.cachedHTML, true
}

// Set stores the rendered HTML in the cache.
func (c *RenderCache) Set(html string) {
	h := sha1.Sum([]byte(html))
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cachedHTML = html
	c.cacheKey = hex.EncodeToString(h[:])[:16]
	c.builtAt = time.Now()
	c.missCount.Add(1)
}

// GetConfig returns the cached runtime config JSON if it is fresh.
func (c *RenderCache) GetConfig() ([]byte, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.devMode || c.configJSON == nil {
		return nil, false
	}
	if time.Since(c.configBuiltAt) > cacheTTL {
		return nil, false
	}
	c.configHits.Add(1)
	return c.configJSON, true
}

// SetConfig stores the runtime config JSON response body in the cache.
func (c *RenderCache) SetConfig(data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.configJSON = data
	c.configBuiltAt = time.Now()
	c.configMisses.Add(1)
}

// Invalidate clears both caches (called on every portal write).
func (c *RenderCache) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cachedHTML = ""
	c.cacheKey = ""
	c.builtAt = time.Time{}
	c.configJSON = nil
	c.configBuiltAt = time.Time{}
}

// SetDevMode enables or disables dev mode (no caching).
func (c *RenderCache) SetDevMode(dev bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.devMode = dev
	c.InvalidateLocked()
}

// IsDevMode returns whether dev mode is enabled.
func (c *RenderCache) IsDevMode() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.devMode
}

// InvalidateLocked clears the cache without acquiring the lock (caller must hold it).
func (c *RenderCache) InvalidateLocked() {
	c.cachedHTML = ""
	c.cacheKey = ""
	c.builtAt = time.Time{}
	c.configJSON = nil
	c.configBuiltAt = time.Time{}
}

// Stats returns cache statistics.
func (c *RenderCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return CacheStats{
		Cached:       c.cachedHTML != "" && time.Since(c.builtAt) <= cacheTTL,
		CacheKey:     c.cacheKey,
		BuiltAt:      c.builtAt,
		HitCount:     c.hitCount.Load(),
		MissCount:    c.missCount.Load(),
		DevMode:      c.devMode,
		ConfigCached: c.configJSON != nil && time.Since(c.configBuiltAt) <= cacheTTL,
		ConfigHits:   c.configHits.Load(),
		ConfigMisses: c.configMisses.Load(),
		TTLSeconds:   int(cacheTTL / time.Second),
	}
}

// AssembleConfigResponse wraps a marshaled RenderConfig into the standard API
// envelope ({"data": ...}) once, so cache hits can be served as raw bytes.
func AssembleConfigResponse(cfg RenderConfig) ([]byte, error) {
	raw, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	body := make([]byte, 0, len(raw)+12)
	body = append(body, `{"data":`...)
	body = append(body, raw...)
	body = append(body, '}')
	return body, nil
}
