package jwt

import (
	"crypto/rand"
	"encoding/hex"
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
	// TokenVersion 是签发时的"用户令牌版本"。登出/吊销时服务端把该用户版本 +1，
	// 旧 token 的 tv 就对不上 → 立即失效（见 middleware.AuthRequired 的校验）。
	// 历史 token 没有该声明，解出来是 0，与库里默认值 0 一致 → 升级部署不会把大家踢下线。
	TokenVersion int `json:"tv"`
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
	return j.generate(userID, username, role, 0, TokenTypeAccess, j.accessTTL)
}

// GenerateAccessTokenWithVersion generates an access token carrying the user's token version.
func (j *JWT) GenerateAccessTokenWithVersion(userID int64, username, role string, tokenVersion int) (string, error) {
	return j.generate(userID, username, role, tokenVersion, TokenTypeAccess, j.accessTTL)
}

// GenerateRefreshToken generates a refresh token.
func (j *JWT) GenerateRefreshToken(userID int64, username, role string) (string, error) {
	return j.generate(userID, username, role, 0, TokenTypeRefresh, j.refreshTTL)
}

// GenerateRefreshTokenWithVersion generates a refresh token carrying the user's token version.
func (j *JWT) GenerateRefreshTokenWithVersion(userID int64, username, role string, tokenVersion int) (string, error) {
	return j.generate(userID, username, role, tokenVersion, TokenTypeRefresh, j.refreshTTL)
}

func (j *JWT) generate(userID int64, username, role string, tokenVersion int, tokenType string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:       userID,
		Username:     username,
		Role:         role,
		TokenType:    tokenType,
		TokenVersion: tokenVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			// jti：随机唯一 ID。refresh 轮换的一次性检测依赖它（安全审计 L-4）。
			ID:        newJTI(),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(j.secret)
}

// newJTI 生成 16 字节 CSPRNG 随机数的 hex 编码（失败时退化为纳秒时间戳，仍唯一）。
func newJTI() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return hex.EncodeToString([]byte(time.Now().Format("20060102150405.000000000")))
	}
	return hex.EncodeToString(b)
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
