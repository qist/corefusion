package jwt

import (
	"errors"
	"testing"
	"time"
)

func newTestJWT() *JWT {
	return New("test-secret-key", time.Hour, 24*time.Hour)
}

// An access token must not be usable in place of a refresh token, otherwise a
// leaked access token could be exchanged for fresh credentials indefinitely.
func TestAccessTokenRejectedAsRefresh(t *testing.T) {
	j := newTestJWT()

	access, err := j.GenerateAccessToken(1, "admin", "super_admin")
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}

	if _, err := j.ValidateTyped(access, TokenTypeRefresh); !errors.Is(err, ErrWrongTokenUse) {
		t.Fatalf("expected ErrWrongTokenUse, got %v", err)
	}
}

func TestRefreshTokenRejectedAsAccess(t *testing.T) {
	j := newTestJWT()

	refresh, err := j.GenerateRefreshToken(1, "admin", "super_admin")
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}

	if _, err := j.ValidateTyped(refresh, TokenTypeAccess); !errors.Is(err, ErrWrongTokenUse) {
		t.Fatalf("expected ErrWrongTokenUse, got %v", err)
	}
}

func TestTokensValidateForOwnType(t *testing.T) {
	j := newTestJWT()

	access, _ := j.GenerateAccessToken(7, "alice", "operator")
	claims, err := j.ValidateTyped(access, TokenTypeAccess)
	if err != nil {
		t.Fatalf("access token should validate as access: %v", err)
	}
	if claims.UserID != 7 || claims.Username != "alice" || claims.Role != "operator" {
		t.Fatalf("claims not round-tripped: %+v", claims)
	}

	refresh, _ := j.GenerateRefreshToken(7, "alice", "operator")
	if _, err := j.ValidateTyped(refresh, TokenTypeRefresh); err != nil {
		t.Fatalf("refresh token should validate as refresh: %v", err)
	}
}

// Tokens minted before the "typ" claim existed must keep working for access,
// so that upgrading the binary does not log every active session out.
func TestLegacyTokenAcceptedAsAccessOnly(t *testing.T) {
	j := newTestJWT()

	legacy, err := j.generate(3, "bob", "viewer", "", time.Hour)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	if _, err := j.ValidateTyped(legacy, TokenTypeAccess); err != nil {
		t.Fatalf("legacy token should be accepted as access: %v", err)
	}

	// ...but must not be accepted where a refresh token is required.
	if _, err := j.ValidateTyped(legacy, TokenTypeRefresh); !errors.Is(err, ErrWrongTokenUse) {
		t.Fatalf("expected ErrWrongTokenUse for legacy-as-refresh, got %v", err)
	}
}

func TestExpiredTokenRejected(t *testing.T) {
	j := New("test-secret-key", -time.Minute, time.Hour)

	access, _ := j.GenerateAccessToken(1, "admin", "super_admin")
	if _, err := j.ValidateTyped(access, TokenTypeAccess); err == nil {
		t.Fatal("expected expired token to be rejected")
	}
}

func TestTokenSignedWithOtherSecretRejected(t *testing.T) {
	issuer := New("secret-a", time.Hour, time.Hour)
	verifier := New("secret-b", time.Hour, time.Hour)

	access, _ := issuer.GenerateAccessToken(1, "admin", "super_admin")
	if _, err := verifier.ValidateTyped(access, TokenTypeAccess); err == nil {
		t.Fatal("expected token signed with a different secret to be rejected")
	}
}
