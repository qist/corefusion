package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// SettingStore provides access to system settings.
type SettingStore interface {
	GetByKey(ctx context.Context, key string) (string, error)
}

// Service handles OAuth2 authentication flows for GitHub and Google.
type Service struct {
	settingSvc SettingStore
	oAuthRepo  *repository.ClientOAuthBindingRepository
	clientRepo *repository.ClientRepository
}

// NewService creates a new OAuth service.
func NewService(settingSvc SettingStore, oAuthRepo *repository.ClientOAuthBindingRepository, clientRepo *repository.ClientRepository) *Service {
	return &Service{
		settingSvc: settingSvc,
		oAuthRepo:  oAuthRepo,
		clientRepo: clientRepo,
	}
}

// OAuthUserInfo contains the user information returned by an OAuth provider.
type OAuthUserInfo struct {
	Provider       string
	ProviderUserID string
	Email          string
	Name           string
	AvatarURL      string
}

// OAuthResult contains the result of an OAuth login/callback.
type OAuthResult struct {
	IsNewUser      bool
	ClientID       int64
	Email          string
	Provider       string
	ProviderUserID string
	ProviderName   string
}

// getGitHubConfig creates an OAuth2 config for GitHub using settings from the database.
func (s *Service) getGitHubConfig(ctx context.Context, redirectURL string) *oauth2.Config {
	clientID, _ := s.settingSvc.GetByKey(ctx, "oauth.github_client_id")
	clientSecret, _ := s.settingSvc.GetByKey(ctx, "oauth.github_client_secret")

	if clientID == "" || clientSecret == "" {
		return nil
	}

	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		// GitHub OAuth scopes: read:user grants access to profile data,
		// user:email grants access to email addresses (including private ones).
		// See: https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/scopes-for-oauth-apps
		Scopes:      []string{"read:user", "user:email"},
		Endpoint:    github.Endpoint,
		RedirectURL: redirectURL,
	}
}

// getGoogleConfig creates an OAuth2 config for Google using settings from the database.
func (s *Service) getGoogleConfig(ctx context.Context, redirectURL string) *oauth2.Config {
	clientID, _ := s.settingSvc.GetByKey(ctx, "oauth.google_client_id")
	clientSecret, _ := s.settingSvc.GetByKey(ctx, "oauth.google_client_secret")

	if clientID == "" || clientSecret == "" {
		return nil
	}

	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		// Google OpenID Connect scopes per latest spec:
		//   openid   — required for OIDC identity layer
		//   email    — grants access to email and email_verified
		//   profile  — grants access to name, picture, etc.
		// See: https://developers.google.com/identity/protocols/oauth2/web-server
		Scopes:      []string{"openid", "email", "profile"},
		Endpoint:    google.Endpoint,
		RedirectURL: redirectURL,
	}
}

// IsProviderEnabled checks if a given OAuth provider is enabled AND fully configured.
// A provider is only considered enabled when:
//  1. The _enabled setting is "true"
//  2. The client_id is non-empty
//  3. The client_secret is non-empty
//
// This prevents showing OAuth buttons that would fail on click.
func (s *Service) IsProviderEnabled(ctx context.Context, provider string) bool {
	enabled, _ := s.settingSvc.GetByKey(ctx, "oauth."+provider+"_enabled")
	if enabled != "true" {
		return false
	}
	clientID, _ := s.settingSvc.GetByKey(ctx, "oauth."+provider+"_client_id")
	clientSecret, _ := s.settingSvc.GetByKey(ctx, "oauth."+provider+"_client_secret")
	return clientID != "" && clientSecret != ""
}

// GetAuthURL returns the OAuth2 authorization URL for the given provider.
func (s *Service) GetAuthURL(ctx context.Context, provider, redirectURL, state string) (string, error) {
	var config *oauth2.Config
	switch provider {
	case "github":
		config = s.getGitHubConfig(ctx, redirectURL)
	case "google":
		config = s.getGoogleConfig(ctx, redirectURL)
	default:
		return "", fmt.Errorf("unsupported OAuth provider: %s", provider)
	}

	if config == nil {
		return "", fmt.Errorf("OAuth provider %s is not configured", provider)
	}

	return config.AuthCodeURL(state, oauth2.AccessTypeOffline), nil
}

// ClientTokenVersion 返回客户**当前**的令牌版本 ✓。
//
// ⚠ OAuth 回调签发 JWT 必须带对版本 ✗：不带（tv=0）而客户的 token_version 又是 1 时 ✗
// → /me 校验 `tv != DB` 直接 401 ✗ → 门户"显示登录成功却被弹回登录页" ✗
// （clients 列默认值是历史 schema 的 1 ✓，新号一样中招 ✓）。
// 查不到/出错返回 0 ✓（与历史行为一致 ✓，不让登录因此中断 ✓）。
func (s *Service) ClientTokenVersion(ctx context.Context, clientID int64) int {
	if clientID <= 0 {
		return 0
	}
	tv, err := s.clientRepo.GetTokenVersion(ctx, clientID)
	if err != nil {
		return 0
	}
	return tv
}

// HandleCallback exchanges the OAuth2 code for user info and performs login/register.
func (s *Service) HandleCallback(ctx context.Context, provider, code, redirectURL string) (*OAuthResult, error) {
	var config *oauth2.Config
	switch provider {
	case "github":
		config = s.getGitHubConfig(ctx, redirectURL)
	case "google":
		config = s.getGoogleConfig(ctx, redirectURL)
	default:
		return nil, fmt.Errorf("unsupported OAuth provider: %s", provider)
	}

	if config == nil {
		return nil, fmt.Errorf("OAuth provider %s is not configured", provider)
	}

	// Exchange code for token
	token, err := config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}

	// Fetch user info from provider
	var userInfo *OAuthUserInfo
	switch provider {
	case "github":
		userInfo, err = s.fetchGitHubUserInfo(ctx, config, token)
	case "google":
		userInfo, err = s.fetchGoogleUserInfo(ctx, config, token)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user info: %w", err)
	}

	// Check if already bound
	existing, _ := s.oAuthRepo.GetByProviderUserID(ctx, userInfo.Provider, userInfo.ProviderUserID)
	if existing != nil && existing.ID > 0 {
		// Already bound — login
		client, err := s.clientRepo.GetByID(ctx, existing.ClientID)
		if err != nil {
			return nil, fmt.Errorf("bound client not found: %w", err)
		}
		// Update name if changed
		if existing.ProviderName != userInfo.Name {
			existing.ProviderName = userInfo.Name
			_ = s.oAuthRepo.Create(ctx, existing) // best-effort update
		}
		return &OAuthResult{
			IsNewUser:      false,
			ClientID:       client.ID,
			Email:          client.Email,
			Provider:       userInfo.Provider,
			ProviderUserID: userInfo.ProviderUserID,
			ProviderName:   userInfo.Name,
		}, nil
	}

	// Not bound — auto-register a new client account
	email := userInfo.Email
	if email == "" {
		email = fmt.Sprintf("%s_%s@oauth.local", provider, userInfo.ProviderUserID)
	}

	// Check if email already taken
	// ⚠ GetByEmail 查无此人时返回 (零值对象, ErrRecordNotFound) ✗ —— 错误**绝不能吞** ✗：
	// 只判 `!= nil` 会把"没找到"当成"找到 ID=0 的客户" ✗ → 绑定挂到 client_id=0 ✗、
	// 给 client 0 签 JWT ✗（真机现象：回调 200 但登录是坏的，绑定表 client_id 全是 0 ✓）。
	existingClient, err := s.clientRepo.GetByEmail(ctx, email)
	if err == nil && existingClient != nil && existingClient.ID > 0 {
		// Email exists — bind OAuth to existing account
		binding := &entity.ClientOAuthBinding{
			ClientID:       existingClient.ID,
			Provider:       userInfo.Provider,
			ProviderUserID: userInfo.ProviderUserID,
			ProviderName:   userInfo.Name,
			BoundAt:        time.Now(),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		if err := s.oAuthRepo.Create(ctx, binding); err != nil {
			return nil, fmt.Errorf("failed to bind OAuth: %w", err)
		}
		return &OAuthResult{
			IsNewUser:      false,
			ClientID:       existingClient.ID,
			Email:          existingClient.Email,
			Provider:       userInfo.Provider,
			ProviderUserID: userInfo.ProviderUserID,
			ProviderName:   userInfo.Name,
		}, nil
	}

	// Create new client
	client := &entity.Client{
		Email:    email,
		Password: "", // No password for OAuth-only accounts
		Status:   "active",
	}
	if err := s.clientRepo.Create(ctx, client); err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	// Create OAuth binding
	binding := &entity.ClientOAuthBinding{
		ClientID:       client.ID,
		Provider:       userInfo.Provider,
		ProviderUserID: userInfo.ProviderUserID,
		ProviderName:   userInfo.Name,
		BoundAt:        time.Now(),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := s.oAuthRepo.Create(ctx, binding); err != nil {
		return nil, fmt.Errorf("failed to create OAuth binding: %w", err)
	}

	return &OAuthResult{
		IsNewUser:      true,
		ClientID:       client.ID,
		Email:          client.Email,
		Provider:       userInfo.Provider,
		ProviderUserID: userInfo.ProviderUserID,
		ProviderName:   userInfo.Name,
	}, nil
}

// fetchGitHubUserInfo fetches user info from GitHub REST API.
// Uses the latest API v3 endpoints with proper Accept headers.
// See: https://docs.github.com/en/rest/users/users?apiVersion=2022-11-28#get-the-authenticated-user
func (s *Service) fetchGitHubUserInfo(ctx context.Context, config *oauth2.Config, token *oauth2.Token) (*OAuthUserInfo, error) {
	client := config.Client(ctx, token)

	// Get user profile — GitHub API requires Accept: application/json for consistent JSON responses.
	// See: https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/authenticating-to-the-rest-api-with-an-oauth-app
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var ghUser struct {
		ID        json.Number `json:"id"` // GitHub may return int or string depending on API version
		Login     string      `json:"login"`
		Name      string      `json:"name"`
		Email     string      `json:"email"`
		AvatarURL string      `json:"avatar_url"`
	}
	if err := json.Unmarshal(body, &ghUser); err != nil {
		return nil, err
	}

	name := ghUser.Name
	if name == "" {
		name = ghUser.Login
	}

	email := ghUser.Email
	// If email is not public, fetch from /user/emails endpoint.
	// The user:email scope is required for this.
	if email == "" {
		email, _ = s.fetchGitHubPrimaryEmail(ctx, client)
	}

	// Use the string representation of the ID for portability
	providerUserID := ghUser.ID.String()
	if providerUserID == "" || providerUserID == "0" {
		return nil, fmt.Errorf("failed to get GitHub user ID")
	}

	return &OAuthUserInfo{
		Provider:       "github",
		ProviderUserID: providerUserID,
		Email:          email,
		Name:           name,
		AvatarURL:      ghUser.AvatarURL,
	}, nil
}

// fetchGitHubPrimaryEmail fetches the primary verified email from GitHub API.
// See: https://docs.github.com/en/rest/users/emails?apiVersion=2022-11-28#list-email-addresses-for-the-authenticated-user
func (s *Service) fetchGitHubPrimaryEmail(ctx context.Context, client *http.Client) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user/emails", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var emails []struct {
		Email      string `json:"email"`
		Primary    bool   `json:"primary"`
		Verified   bool   `json:"verified"`
		Visibility string `json:"visibility"`
	}
	if err := json.Unmarshal(body, &emails); err != nil {
		return "", err
	}

	// Prefer primary + verified email
	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email, nil
		}
	}

	// Fallback: any verified email
	for _, e := range emails {
		if e.Verified {
			return e.Email, nil
		}
	}

	// Fallback: primary email even if unverified
	for _, e := range emails {
		if e.Primary {
			return e.Email, nil
		}
	}

	// Last resort: first email
	if len(emails) > 0 {
		return emails[0].Email, nil
	}

	return "", nil
}

// fetchGoogleUserInfo fetches user info from Google OpenID Connect userinfo endpoint.
// Uses the latest v3 endpoint as recommended by Google.
// See: https://developers.google.com/identity/protocols/oauth2/web-server#callinganapi
// See: https://developers.google.com/identity/openid-connect/openid-connect#obtainuserinfo
func (s *Service) fetchGoogleUserInfo(ctx context.Context, config *oauth2.Config, token *oauth2.Token) (*OAuthUserInfo, error) {
	client := config.Client(ctx, token)

	// Use the v3 userinfo endpoint (latest recommended by Google).
	// The older v2 endpoint (googleapis.com/oauth2/v2/userinfo) is deprecated.
	req, err := http.NewRequestWithContext(ctx, "GET", "https://www.googleapis.com/oauth2/v3/userinfo", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var gUser struct {
		Sub           string `json:"sub"`            // Unique identifier (subject)
		Email         string `json:"email"`          // Email address
		EmailVerified bool   `json:"email_verified"` // Whether email is verified
		Name          string `json:"name"`           // Full name
		GivenName     string `json:"given_name"`     // First name
		FamilyName    string `json:"family_name"`    // Last name
		Picture       string `json:"picture"`        // Profile picture URL
		Locale        string `json:"locale"`         // Locale
	}
	if err := json.Unmarshal(body, &gUser); err != nil {
		return nil, err
	}

	// Use 'sub' as the unique provider user ID (OIDC standard)
	name := gUser.Name
	if name == "" {
		name = gUser.GivenName
	}
	if name == "" && gUser.Email != "" {
		// Fallback: use the part before @ as display name
		name = gUser.Email
	}

	return &OAuthUserInfo{
		Provider:       "google",
		ProviderUserID: gUser.Sub,
		Email:          gUser.Email,
		Name:           name,
		AvatarURL:      gUser.Picture,
	}, nil
}
