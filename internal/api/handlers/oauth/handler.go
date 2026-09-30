package oauth

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"net/url"
	"strings"
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
	// portalPath 是登录成功后跳回的**门户页面路径** ✓：主面板引擎上门户挂在 /p ✓，
	// 独立门户端口引擎（NewPortalEngine）挂在根路径 / ✓ → 用 SetPortalBasePath 配置 ✓。
	portalPath string
}

// NewHandler creates a new OAuth handler.
func NewHandler(oauthSvc *oauthSvc.Service, jwtSecret string) *Handler {
	h := &Handler{
		oauthSvc:   oauthSvc,
		jwt:        jwt.New(jwtSecret, 24*time.Hour, 7*24*time.Hour),
		stateStore: make(map[string]stateEntry),
		portalPath: "/p", // 主面板引擎的默认挂载点 ✓
	}
	// Start cleanup goroutine for expired states
	go h.cleanupStates()
	return h
}

// SetPortalBasePath 设置登录成功后跳回的门户页面路径 ✓（独立门户端口引擎传 "/" ✓）。
// ⚠ "/" 是**合法值** ✗：先 Trim 再判空会把 "/" 削成空串而误拒 ✗
// （真机现象：门户引擎回调仍跳 /p → 门户域名下 /p 是 404 ✗）。
func (h *Handler) SetPortalBasePath(p string) {
	p = strings.TrimSpace(p)
	if p == "" {
		return
	}
	if p = strings.TrimSuffix(p, "/"); p == "" {
		p = "/"
	}
	h.portalPath = p
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
	// ⚠ 必须带**客户当前令牌版本** ✗：不带（tv=0）时 /me 校验 `tv != DB` 401 ✗
	// → 门户"显示登录成功却被弹回登录页" ✗（与密码登录 GenerateAccessTokenWithVersion 同口径 ✓）。
	tv := h.oauthSvc.ClientTokenVersion(c.Request.Context(), result.ClientID)

	// ── 自助解封分支（与密码登录 service.go Login 同语义）──
	// 账号 suspended 但符合自解封条件：**不签发**常规 token（M2 语义不破），
	// 改签 typ=self_unban 专用 token（中间件只放行 POST /self-unban），
	// 经 fragment 带回门户显示解封卡；解封成功后用户重新走 OAuth 登录即可。
	if result.RequireSelfUnban {
		suToken, terr := h.jwt.GenerateSelfUnbanToken(result.ClientID, result.Email, "client", tv)
		if terr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate tokens"})
			return
		}
		c.Redirect(http.StatusFound, h.portalPath+"#require_self_unban=1"+
			"&self_unban_token="+url.QueryEscape(suToken)+
			"&self_unban_hint="+url.QueryEscape(result.SelfUnbanHint))
		return
	}

	accessToken, err := h.jwt.GenerateAccessTokenWithVersion(result.ClientID, result.Email, "client", tv)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate tokens"})
		return
	}
	refreshToken, err := h.jwt.GenerateRefreshTokenWithVersion(result.ClientID, result.Email, "client", tv)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate tokens"})
		return
	}

	// ── 统一**整页跳回门户** ✓（门户 onMounted 会收 URL 里的 token 并进入登录态 ✓）──
	// ⚠ 不走 window.opener / postMessage 分支 ✗：门户标签可能本身是被别的页面打开的
	// （例如管理端预览、外链 _blank）✗ → opener 残留时 token 会 postMessage 给
	// **不监听的页面** 然后 window.close() ✗ → 标签直接关闭、登录凭空丢失 ✗
	// （真机现象：客户账号创建了、门户标签却关了 ✓）。
	// 门户侧登录按钮是整页跳转发起的 ✓ → 回调用 302 跳回门户是同一语义 ✓。
	// ⚠ 跳回路径按**引擎**配置 ✓：主面板引擎 → /p ✓；独立门户端口引擎 → / ✓。
	// 安全审计 L-5：token 改走 **fragment**（#access_token=..）下发 —— query 串里的
	// 长效 JWT 会残留进浏览器历史/访问日志/Referer ✗；fragment 只存在于客户端，
	// 不会发往服务器。门户模板 handleOAuthCallback 已同步改为读 location.hash
	// （存量模板由 PatchTemplateSecurity 幂等补丁升级）。
	c.Redirect(http.StatusFound, h.portalPath+"#access_token="+url.QueryEscape(accessToken)+
		"&refresh_token="+url.QueryEscape(refreshToken))
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
