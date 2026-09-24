// Package twofa 实现基于 TOTP（RFC 6238）的双因素认证（2FA）。
//
// 设计约束（对齐 UI_DESIGN.md §9 认证与 2FA 交互设计）：
//   - 登录两步走：密码验证通过后若用户已启用 2FA，先返回一次性登录票据（pending login），
//     前端再提交 TOTP 动态码 / 恢复码换取正式 token —— 密码与动态码**必须分开两次请求**，
//     中间态票据 5 分钟过期、只能取一次；
//   - 恢复码一次性、只在下发时刻展示明文，数据库只存 sha256 摘要；
//   - TOTP secret 用 AES-256-GCM 加密落库（crypto.EncryptSecret），绝不通过 API 输出；
//   - 关闭 2FA / 重新生成恢复码必须再次验证动态码（防已登录会话被越权操作）。
//
// ⚠ 恢复路径（admin 2fa reset）在 cmd/corefusion/admin.go —— 与登录链路相互独立，
// 防止"唯一的超级管理员丢了认证器+恢复码就被永久锁在门外"。
package twofa

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"

	"corefusion/internal/model/entity"
	"corefusion/internal/security/crypto"
	"corefusion/internal/store/repository"
)

var (
	// ErrCodeInvalid 表示动态码/恢复码校验失败。
	ErrCodeInvalid = errors.New("invalid verification code")
	// ErrNotEnabled 表示用户当前未启用 2FA（关闭/重置等场景的防御校验）。
	ErrNotEnabled = errors.New("2FA is not enabled")

	// RecoveryCodeCount 每次生成恢复码的数量。
	RecoveryCodeCount = 10
)

// Service 提供 2FA 全部业务逻辑。
type Service struct {
	userRepo     *repository.UserRepository
	recoveryRepo *repository.RecoveryCodeRepository
	issuer       string // otpauth URI 里的 Issuer（面板品牌名）
	pending      *pendingLoginStore
}

// NewService 创建 2FA 服务。
func NewService(userRepo *repository.UserRepository, recoveryRepo *repository.RecoveryCodeRepository, issuer string) *Service {
	return &Service{
		userRepo:     userRepo,
		recoveryRepo: recoveryRepo,
		issuer:       issuer,
		pending:      newPendingLoginStore(),
	}
}

// --- 绑定 / 管理 ---

// BeginSetup 为用户生成新的 TOTP secret 与 otpauth URI（尚未持久化、尚未启用）。
// 返回的 secret 明文只此一次，用于前端展示 + 扫码二维码（otpauth URI）。
func (s *Service) BeginSetup(ctx context.Context, user *entity.User) (secret, otpauthURI string, err error) {
	if user.TOTPEnabled {
		return "", "", errors.New("2FA already enabled")
	}
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      s.issuer,
		AccountName: user.Username,
		Period:      30,
		SecretSize:  20, // 160bit，与 RFC 4226 推荐一致
	})
	if err != nil {
		return "", "", fmt.Errorf("generate totp secret: %w", err)
	}
	return key.Secret(), key.URL(), nil
}

// Enable 用动态码验证后持久化并启用 2FA，返回一次性恢复码明文列表。
func (s *Service) Enable(ctx context.Context, user *entity.User, code string) ([]string, error) {
	if user.TOTPEnabled {
		return nil, errors.New("2FA already enabled")
	}
	secret, err := crypto.DecryptSecret(user.TOTPSecret)
	if err != nil || secret == "" {
		// 用户还没走 setup（没有待启用密钥）——不允许直接 Enable。
		return nil, errors.New("no pending TOTP secret, call setup first")
	}
	if !s.validateTOTP(secret, code) {
		return nil, ErrCodeInvalid
	}
	if err := s.userRepo.SetTOTP(ctx, user.ID, true, user.TOTPSecret); err != nil {
		return nil, err
	}
	return s.issueRecoveryCodes(ctx, user.ID)
}

// Disable 关闭 2FA 并清空密钥与全部恢复码（调用方必须已确认身份）。
func (s *Service) Disable(ctx context.Context, user *entity.User) error {
	if !user.TOTPEnabled {
		return ErrNotEnabled
	}
	if err := s.userRepo.ClearTOTP(ctx, user.ID); err != nil {
		return err
	}
	return s.recoveryRepo.DeleteByUser(ctx, user.ID)
}

// RegenerateRecoveryCodes 重新生成恢复码（旧码全部作废）；调用方需已验证动态码。
func (s *Service) RegenerateRecoveryCodes(ctx context.Context, user *entity.User) ([]string, error) {
	if !user.TOTPEnabled {
		return nil, ErrNotEnabled
	}
	return s.issueRecoveryCodes(ctx, user.ID)
}

// Status 返回用户 2FA 状态：是否启用、是否已有密钥、未使用的恢复码数量。
func (s *Service) Status(ctx context.Context, userID int64) (enabled bool, unusedRecovery int64, err error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return false, 0, err
	}
	unused, err := s.recoveryRepo.CountUnused(ctx, userID)
	if err != nil {
		return false, 0, err
	}
	return user.TOTPEnabled, unused, nil
}

// VerifyManagedCode 在校验"管理侧"操作（关闭 2FA / 重新生成恢复码）时使用：
// 必须用 TOTP 动态码（恢复码不能用来管理操作），且 secret 必须真实存在。
func (s *Service) VerifyManagedCode(ctx context.Context, user *entity.User, code string) error {
	if !user.TOTPEnabled {
		return ErrNotEnabled
	}
	secret, err := crypto.DecryptSecret(user.TOTPSecret)
	if err != nil {
		return fmt.Errorf("decrypt totp secret: %w", err)
	}
	if secret == "" {
		return errors.New("totp secret missing")
	}
	if !s.validateTOTP(secret, code) {
		return ErrCodeInvalid
	}
	return nil
}

// VerifyLoginCode 登录场景的验证：优先恢复码（命中即标记一次性使用），否则 TOTP 动态码。
func (s *Service) VerifyLoginCode(ctx context.Context, user *entity.User, code string) error {
	if !user.TOTPEnabled {
		return ErrNotEnabled
	}
	// 1) 恢复码 —— 大小写不敏感、去空格，便于手输
	normalized := strings.ToUpper(strings.ReplaceAll(code, " ", ""))
	if rec, err := s.recoveryRepo.FindUnusedByHash(ctx, user.ID, hashCode(normalized)); err == nil {
		return s.recoveryRepo.MarkUsed(ctx, rec.ID)
	}
	// 2) TOTP 动态码
	secret, err := crypto.DecryptSecret(user.TOTPSecret)
	if err != nil {
		return fmt.Errorf("decrypt totp secret: %w", err)
	}
	if secret == "" {
		return errors.New("totp secret missing")
	}
	if s.validateTOTP(secret, code) {
		return nil
	}
	return ErrCodeInvalid
}

// --- pending login（密码通过 → 待 2FA） ---

// PendingLogin 是一次完成密码验证但尚未完成 2FA 的登录中间态。
type PendingLogin struct {
	UserID    int64
	Username  string
	ExpiresAt time.Time
}

// CreatePendingLogin 创建一张一次性且 5 分钟过期的登录票据，返回 token。
func (s *Service) CreatePendingLogin(userID int64, username string) string {
	return s.pending.create(userID, username, 5*time.Minute)
}

// TakePendingLogin 取出（并作废）一张票据；不存在/过期返回 false。
func (s *Service) TakePendingLogin(token string) (PendingLogin, bool) {
	return s.pending.take(token)
}

// PeekPendingLogin 查询票据但**不消费**。登录第二步先 peek 校验动态码，
// 校验通过后再 Take 消费 —— 动态码输错不会作废票据（无需重新输密码）。
func (s *Service) PeekPendingLogin(token string) (PendingLogin, bool) {
	return s.pending.peek(token)
}

// --- 内部工具 ---

// validateTOTP 使用 30s 周期、±1 时钟窗口、6 位 SHA1（主流认证器兼容）校验。
func (s *Service) validateTOTP(secret, code string) bool {
	code = strings.TrimSpace(code)
	if len(code) != 6 {
		return false
	}
	ok, err := totp.ValidateCustom(code, secret, time.Now().UTC(), totp.ValidateOpts{
		Period:    30,
		Skew:      1,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil {
		return false
	}
	return ok
}

// issueRecoveryCodes 生成一组新恢复码并落库，返回明文（一次性展示）。
func (s *Service) issueRecoveryCodes(ctx context.Context, userID int64) ([]string, error) {
	plains := make([]string, 0, RecoveryCodeCount)
	hashes := make([]string, 0, RecoveryCodeCount)
	for i := 0; i < RecoveryCodeCount; i++ {
		plain := randomRecoveryCode()
		plains = append(plains, plain)
		hashes = append(hashes, hashCode(plain))
	}
	if err := s.recoveryRepo.ReplaceByUser(ctx, userID, hashes); err != nil {
		return nil, err
	}
	return plains, nil
}

// randomRecoveryCode 生成 XXXX-XXXX-XXXX 格式的恢复码（12 字符，48bit 十六进制）。
func randomRecoveryCode() string {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		panic("crypto/rand failed: " + err.Error()) // 系统熵源故障，无法保证安全，直接崩溃
	}
	hexStr := strings.ToUpper(hex.EncodeToString(buf))
	return hexStr[0:4] + "-" + hexStr[4:8] + "-" + hexStr[8:12]
}

func hashCode(plain string) string {
	// 统一转大写后再 hash：无论生成/校验传什么大小写，摘要恒一致。
	sum := sha256.Sum256([]byte(strings.ToUpper(plain)))
	return hex.EncodeToString(sum[:])
}

// --- pending login store（服务端内存、TTL、一次性） ---

type pendingLoginStore struct {
	mu    sync.Mutex
	items map[string]PendingLogin
}

func newPendingLoginStore() *pendingLoginStore {
	return &pendingLoginStore{items: make(map[string]PendingLogin)}
}

func (p *pendingLoginStore) create(userID int64, username string, ttl time.Duration) string {
	token, err := randomToken()
	if err != nil {
		return ""
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.gcLocked(time.Now())
	p.items[token] = PendingLogin{UserID: userID, Username: username, ExpiresAt: time.Now().Add(ttl)}
	return token
}

func (p *pendingLoginStore) take(token string) (PendingLogin, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	item, ok := p.items[token]
	if !ok {
		return PendingLogin{}, false
	}
	delete(p.items, token) // 一次性：取走即作废
	if time.Now().After(item.ExpiresAt) {
		return PendingLogin{}, false
	}
	return item, true
}

// peek 只查询不删除（供登录第二步先校验动态码、成功后再消费）。
func (p *pendingLoginStore) peek(token string) (PendingLogin, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	item, ok := p.items[token]
	if !ok || time.Now().After(item.ExpiresAt) {
		return PendingLogin{}, false
	}
	return item, true
}

// gcLocked 惰性清理过期条目（调用方需持锁）。
func (p *pendingLoginStore) gcLocked(now time.Time) {
	for k, v := range p.items {
		if now.After(v.ExpiresAt) {
			delete(p.items, k)
		}
	}
}

// randomToken 生成 32 字节随机 token（hex）。
func randomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}