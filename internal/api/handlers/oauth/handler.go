package oauth

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"corefusion/internal/auth/jwt"
	oauthSvc "corefusion/internal/service/oauth"
)

// stateEntry stores a state parameter with its creation time.
type stateEntry struct {
	State     string
	CreatedAt time.Time
}

// Handler handles OAuth2 HTTP requests.
type Handler struct {
	oauthSvc *oauthSvc.Service
	jwt      *jwt.JWT
	// In-memory state store (for single-instance; use Redis for multi-instance)
	stateStore   map[string]stateEntry
	stateStoreMu sync.RWMutex
}

// NewHandler creates a new OAuth handler.
func NewHandler(oauthSvc *oauthSvc.Service, jwtSecret string) *Handler {
	h := &Handler{
		oauthSvc:   oauthSvc,
		jwt:        jwt.New(jwtSecret, 24*time.Hour, 7*24*time.Hour),
		stateStore: make(map[string]stateEntry),
	}
	// Start cleanup goroutine for expired states
	go h.cleanupStates()
	return h
}

// cleanupStates removes expired state entries periodically.
func (h *Handler) cleanupStates() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		h.stateStoreMu.Lock()
		now := time.Now()
		for k, v := range h.stateStore {
			if now.Sub(v.CreatedAt) > 10*time.Minute {
				delete(h.stateStore, k)
			}
		}
		h.stateStoreMu.Unlock()
	}
}

// generateState creates a random state parameter and stores it.
func (h *Handler) generateState() string {
	b := make([]byte, 16)
	rand.Read(b)
	state := hex.EncodeToString(b)
	h.stateStoreMu.Lock()
	h.stateStore[state] = stateEntry{State: state, CreatedAt: time.Now()}
	h.stateStoreMu.Unlock()
	return state
}

// validateState checks if a state parameter is valid and removes it.
func (h *Handler) validateState(state string) bool {
	h.stateStoreMu.Lock()
	defer h.stateStoreMu.Unlock()
	entry, ok := h.stateStore[state]
	if !ok {
		return false
	}
	delete(h.stateStore, state)
	return time.Since(entry.CreatedAt) < 10*time.Minute
}

// Authorize handles GET /api/v1/client-auth/oauth/:provider
// Redirects the user to the OAuth provider's authorization page.
func (h *Handler) Authorize(c *gin.Context) {
	provider := c.Param("provider")

	// Check if provider is enabled
	if !h.oauthSvc.IsProviderEnabled(c.Request.Context(), provider) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "OAuth provider not enabled: " + provider})
		return
	}

	// Generate state for CSRF protection
	state := h.generateState()

	// Build redirect URL for callback
	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	host := c.GetHeader("X-Forwarded-Host")
	if host == "" {
		host = c.Request.Host
	}
	redirectURL := scheme + "://" + host + "/api/v1/client-auth/oauth/" + provider + "/callback"

	authURL, err := h.oauthSvc.GetAuthURL(c.Request.Context(), provider, redirectURL, state)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Redirect(http.StatusTemporaryRedirect, authURL)
}

// Callback handles GET /api/v1/client-auth/oauth/:provider/callback
// Exchanges the OAuth2 code for user info and performs login/register.
func (h *Handler) Callback(c *gin.Context) {
	provider := c.Param("provider")

	// Check if provider is enabled
	if !h.oauthSvc.IsProviderEnabled(c.Request.Context(), provider) {
		c.HTML(http.StatusBadRequest, "", "<html><body><script>alert('OAuth provider not enabled'); window.location.href='/p';</script></body></html>")
		return
	}

	code := c.Query("code")
	state := c.Query("state")

	// Validate state
	if !h.validateState(state) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid or expired state parameter"})
		return
	}

	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing authorization code"})
		return
	}

	// Build redirect URL (same as in Authorize)
	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	host := c.GetHeader("X-Forwarded-Host")
	if host == "" {
		host = c.Request.Host
	}
	redirectURL := scheme + "://" + host + "/api/v1/client-auth/oauth/" + provider + "/callback"

	// Process OAuth callback
	result, err := h.oauthSvc.HandleCallback(c.Request.Context(), provider, code, redirectURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Generate JWT tokens for the client
	accessToken, err := h.jwt.GenerateAccessToken(result.ClientID, result.Email, "client")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate tokens"})
		return
	}
	refreshToken, err := h.jwt.GenerateRefreshToken(result.ClientID, result.Email, "client")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate tokens"})
		return
	}

	// Return HTML page that posts the tokens to the parent window (for popup-based OAuth)
	// or redirect to the portal with tokens as query params.
	// S-6 fix: restrict postMessage target origin to the panel's own origin
	// to prevent token leakage to malicious opener windows.
	panelOrigin := scheme + "://" + host
	responseHTML := `<!DOCTYPE html>
<html>
<head><title>OAuth Login</title></head>
<body>
<script>
(function() {
  var data = ` + jsonEscape(accessToken, refreshToken, result) + `;
  var targetOrigin = ` + fmt.Sprintf("%q", panelOrigin) + `;
  if (window.opener) {
    window.opener.postMessage({ type: 'oauth-callback', data: data }, targetOrigin);
    window.close();
  } else {
    window.location.href = '/p?access_token=' + encodeURIComponent(data.access_token) + '&refresh_token=' + encodeURIComponent(data.refresh_token);
  }
})();
</script>
</body>
</html>`

	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(responseHTML))
}

// jsonEscape safely produces a JSON string for embedding in HTML.
func jsonEscape(accessToken, refreshToken string, result *oauthSvc.OAuthResult) string {
	data := gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"is_new_user":   result.IsNewUser,
		"client_id":     result.ClientID,
		"email":         result.Email,
		"provider":      result.Provider,
		"provider_name": result.ProviderName,
	}
	b, _ := json.Marshal(data)
	return string(b)
}

// ProvidersStatus handles GET /api/v1/client-auth/oauth/providers
// Returns the list of OAuth providers with their enabled status.
// Used by the Portal login page to conditionally show OAuth buttons.
func (h *Handler) ProvidersStatus(c *gin.Context) {
	providers := []gin.H{}

	for _, provider := range []string{"github", "google"} {
		enabled := h.oauthSvc.IsProviderEnabled(c.Request.Context(), provider)
		providers = append(providers, gin.H{
			"provider": provider,
			"enabled":  enabled,
		})
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"providers": providers}})
}
