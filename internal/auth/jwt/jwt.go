package jwt

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrTokenExpired  = errors.New("token expired")
	ErrTokenInvalid  = errors.New("token invalid")
	ErrWrongTokenUse = errors.New("token used for wrong purpose")
)

// Token type values carried in the "typ" claim. Without this an access token
// and a refresh token are structurally identical, which would let a leaked
// access token be exchanged for fresh credentials indefinitely.
const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

// Claims represents JWT claims.
type Claims struct {
	UserID    int64  `json:"user_id"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	TokenType string `json:"typ"`
	jwt.RegisteredClaims
}

// JWT handles JWT token operations.
type JWT struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

// New creates a new JWT handler.
func New(secret string, accessTTL, refreshTTL time.Duration) *JWT {
	return &JWT{
		secret:     []byte(secret),
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

// GenerateAccessToken generates an access token.
func (j *JWT) GenerateAccessToken(userID int64, username, role string) (string, error) {
	return j.generate(userID, username, role, TokenTypeAccess, j.accessTTL)
}

// GenerateRefreshToken generates a refresh token.
func (j *JWT) GenerateRefreshToken(userID int64, username, role string) (string, error) {
	return j.generate(userID, username, role, TokenTypeRefresh, j.refreshTTL)
}

func (j *JWT) generate(userID int64, username, role, tokenType string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:    userID,
		Username:  username,
		Role:      role,
		TokenType: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(j.secret)
}

// Validate validates a token and returns the claims.
func (j *JWT) Validate(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return j.secret, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrTokenInvalid
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrTokenInvalid
	}

	return claims, nil
}

// ValidateTyped validates a token and additionally asserts its "typ" claim.
//
// Tokens issued before the "typ" claim existed carry an empty type. Those are
// accepted only where an access token is expected, so that in-flight sessions
// survive the upgrade; refresh explicitly requires the new claim, which is the
// half that actually grants new credentials.
func (j *JWT) ValidateTyped(tokenStr, expectedType string) (*Claims, error) {
	claims, err := j.Validate(tokenStr)
	if err != nil {
		return nil, err
	}

	if claims.TokenType == expectedType {
		return claims, nil
	}
	if claims.TokenType == "" && expectedType == TokenTypeAccess {
		return claims, nil
	}

	return nil, ErrWrongTokenUse
}
