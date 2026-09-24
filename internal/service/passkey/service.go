// Package passkey 实现基于 WebAuthn 的通行密钥（Passkey）登录与凭证管理。
//
// 设计约束（详见 .codebuddy/memory/2026-09-23.md 的 Passkey 设计修正）：
//   - RP ID 必须是可注册域名（IP 不行，localhost 例外），凭证绑定 RP ID；
//   - 不预设域名：白名单 `panel.passkey_rp_ids` 存于系统设置，命中规则
//     `host == rp || HasSuffix(host, "."+rp)`，且**登录（未认证）阶段只认白名单已有条目**；
//   - 密码登录永远保留，Passkey 只是增强入口 —— 任何部署形态都不会被锁死；
//   - 会话数据（SessionData）只存服务端内存，TTL 5 分钟、一次性；客户端拿到的是
//     不透明 token，拿不到也无法篡改挑战。
package passkey

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

const (
	// settingKeyRPIDs 是系统设置里 RP ID 白名单的配置键。
	settingKeyRPIDs = "panel.passkey_rp_ids"

	// sessionTTL 是一次 WebAuthn 仪式（begin→finish）的会话有效期。
	sessionTTL = 5 * time.Minute

	// userHandleLen 是 WebAuthn User Handle 的字节长度（32 字节随机数）。
	userHandleLen = 32
)

var (
	// ErrRPNotAllowed 表示当前请求的 Host 不在白名单内（或不是安全上下文）。
	ErrRPNotAllowed = errors.New("passkey is not available for this access address")
	// ErrCredentialNotFound 表示凭证不存在或不属于该用户。
	ErrCredentialNotFound = errors.New("credential not found")
)

// Service 是 Passkey 的业务逻辑入口。
type Service struct {
	userRepo    *repository.UserRepository
	credRepo    *repository.WebAuthnCredentialRepository
	settingRepo *repository.SystemSettingRepository
	sessions    *sessionStore
	displayName string // RP 显示名（浏览器授权弹窗里展示）
}

// NewService 创建 Passkey 服务。
func NewService(userRepo *repository.UserRepository, credRepo *repository.WebAuthnCredentialRepository,
	settingRepo *repository.SystemSettingRepository, displayName string) *Service {
	s := &Service{
		userRepo:    userRepo,
		credRepo:    credRepo,
		settingRepo: settingRepo,
		displayName: displayName,
		sessions:    newSessionStore(),
	}
	go s.sessions.runCleanup()
	return s
}

// --- RP ID 白名单 ---

// RPIDs 返回当前白名单（去重、去空、小写）。
func (s *Service) RPIDs(ctx context.Context) []string {
	raw, err := s.settingRepo.GetByKey(ctx, settingKeyRPIDs)
	if err != nil || strings.TrimSpace(raw) == "" {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	for _, part := range strings.Split(raw, ",") {
		p := strings.ToLower(strings.TrimSpace(part))
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

// AddRPID 把一个 RP ID 加入白名单（幂等）。
func (s *Service) AddRPID(ctx context.Context, rpid string) error {
	rpid = strings.ToLower(strings.TrimSpace(rpid))
	if rpid == "" {
		return fmt.Errorf("rpid must not be empty")
	}
	cur := s.RPIDs(ctx)
	for _, r := range cur {
		if r == rpid {
			return nil // already present
		}
	}
	cur = append(cur, rpid)
	return s.settingRepo.Set(ctx, settingKeyRPIDs, strings.Join(cur, ","))
}

// RemoveRPID 从白名单移除一个 RP ID。
// 已注册的凭证仍保留在库里（若域名恢复可再次使用），只是该入口不再可用。
func (s *Service) RemoveRPID(ctx context.Context, rpid string) error {
	rpid = strings.ToLower(strings.TrimSpace(rpid))
	cur := s.RPIDs(ctx)
	var out []string
	for _, r := range cur {
		if r != rpid {
			out = append(out, r)
		}
	}
	return s.settingRepo.Set(ctx, settingKeyRPIDs, strings.Join(out, ","))
}

// ResolveRPID 按设计规则把请求 Host 解析为允许的 RP ID。
// （返回空串表示命中失败 —— 不安全的 Host / IP / 不在白名单。）
func (s *Service) ResolveRPID(ctx context.Context, host string) string {
	return resolveRPIDFromList(s.RPIDs(ctx), host)
}

// resolveRPIDFromList 是纯解析函数（便于单测）：
//   - 命中规则 `host == rp || HasSuffix(host, "."+rp)`；
//   - **精确匹配优先**：管理员显式加入白名单的域名优先绑定（符合"把当前访问域名加白名单"的语义）；
//   - 无精确命中时，在子域命中条目中取**最短**的那个（最通用的可注册域）→ 多子域共用一把 passkey。
func resolveRPIDFromList(rpids []string, host string) string {
	if len(rpids) == 0 {
		return ""
	}
	h := normalizeHost(host)
	if h == "" {
		return ""
	}
	for _, rp := range rpids {
		if h == rp {
			return rp
		}
	}
	best := ""
	bestLen := int(^uint(0) >> 1)
	for _, rp := range rpids {
		if strings.HasSuffix(h, "."+rp) {
			if len(rp) < bestLen {
				best, bestLen = rp, len(rp)
			}
		}
	}
	return best
}

// normalizeHost 去掉端口与括号（IPv6 字面量），只保留 host。
func normalizeHost(host string) string {
	host = strings.TrimSpace(strings.ToLower(host))
	if host == "" {
		return ""
	}
	// 去掉末端的 . （FQDN 写法）
	host = strings.TrimSuffix(host, ".")
	if strings.HasPrefix(host, "[") {
		// [::1]:8080 → ::1
		if i := strings.Index(host, "]"); i > 0 {
			return host[1:i]
		}
		return strings.Trim(host, "[]")
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return host
}

// --- 工具函数（导出给 handler 用） ---

// IsSecureContext 判断是否安全上下文（HTTPS 或 localhost —— WebAuthn 的硬性前提）。
func IsSecureContext(scheme, host string) bool {
	if scheme == "https" {
		return true
	}
	h := normalizeHost(host)
	return h == "localhost" || h == "127.0.0.1" || h == "::1"
}

// newUserHandle 生成新的随机 User Handle（base64url）。
func newUserHandle() (string, error) {
	b := make([]byte, userHandleLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// ensureUserHandle 为用户懒生成并持久化 WebAuthn Handle（幂等）。
func (s *Service) ensureUserHandle(ctx context.Context, user *entity.User) (string, error) {
	if user.WebAuthnHandle != "" {
		return user.WebAuthnHandle, nil
	}
	handle, err := newUserHandle()
	if err != nil {
		return "", err
	}
	if err := s.userRepo.SetWebAuthnHandle(ctx, user.ID, handle); err != nil {
		return "", err
	}
	user.WebAuthnHandle = handle
	return handle, nil
}

// --- webauthn.User 适配 ---

// webUser 把 entity.User + 凭证列表适配为 webauthn.User 接口。
type webUser struct {
	id    []byte
	name  string
	creds []webauthn.Credential
}

func (u *webUser) WebAuthnID() []byte               { return u.id }
func (u *webUser) WebAuthnName() string             { return u.name }
func (u *webUser) WebAuthnDisplayName() string      { return u.name }
func (u *webUser) WebAuthnCredentials() []webauthn.Credential { return u.creds }

// loadWebUser 加载用户的凭证适配列表。
func (s *Service) loadWebUser(ctx context.Context, user *entity.User) (*webUser, error) {
	handle, err := s.ensureUserHandle(ctx, user)
	if err != nil {
		return nil, err
	}
	id, err := base64.RawURLEncoding.DecodeString(handle)
	if err != nil {
		return nil, fmt.Errorf("decode webauthn handle: %w", err)
	}
	creds, err := s.loadCredentials(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	return &webUser{id: id, name: user.Username, creds: creds}, nil
}

// loadCredentials 把该用户所有凭证从 JSON 反序列化。
func (s *Service) loadCredentials(ctx context.Context, userID int64) ([]webauthn.Credential, error) {
	rows, err := s.credRepo.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]webauthn.Credential, 0, len(rows))
	for i := range rows {
		var c webauthn.Credential
		if err := json.Unmarshal([]byte(rows[i].CredentialJSON), &c); err != nil {
			return nil, fmt.Errorf("decode credential %d: %w", rows[i].ID, err)
		}
		out = append(out, c)
	}
	return out, nil
}

// saveUpdatedCredential 把 FinishLogin 后的凭证（SignCount/flags 已刷新）整体回写。
func (s *Service) saveUpdatedCredential(ctx context.Context, userID int64, rpid string, updated *webauthn.Credential) error {
	credID := base64.RawURLEncoding.EncodeToString(updated.ID)
	existing, err := s.credRepo.GetByCredentialID(ctx, rpid, credID)
	if err != nil {
		return err
	}
	if existing.UserID != userID {
		return ErrCredentialNotFound
	}
	blob, err := json.Marshal(updated)
	if err != nil {
		return err
	}
	now := time.Now()
	existing.CredentialJSON = string(blob)
	existing.SignCount = updated.Authenticator.SignCount
	existing.Transport = transportsToString(updated.Transport)
	existing.Attachment = string(updated.Authenticator.Attachment)
	existing.LastUsedAt = &now
	return s.credRepo.Update(ctx, existing)
}

func transportsToString(ts []protocol.AuthenticatorTransport) string {
	parts := make([]string, len(ts))
	for i, t := range ts {
		parts[i] = string(t)
	}
	return strings.Join(parts, ",")
}

// newWebAuthn 按 (rpid, origin) 创建一次性 WebAuthn 实例。
func (s *Service) newWebAuthn(rpid, origin string) (*webauthn.WebAuthn, error) {
	return webauthn.New(&webauthn.Config{
		RPDisplayName: s.displayName,
		RPID:          rpid,
		RPOrigins:     []string{origin},
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			ResidentKey:     protocol.ResidentKeyRequirementRequired, // 强制 discoverable → 支持无用户名登录
			UserVerification: protocol.VerificationPreferred,
		},
		Timeouts: webauthn.TimeoutsConfig{
			Login:        webauthn.TimeoutConfig{Enforce: true, Timeout: 3 * time.Minute},
			Registration: webauthn.TimeoutConfig{Enforce: true, Timeout: 3 * time.Minute},
		},
	})
}

// --- 注册（需已登录） ---

// BeginRegistration 为用户发起注册仪式，返回给前端的创建选项与一次性会话 token。
func (s *Service) BeginRegistration(ctx context.Context, user *entity.User, host, origin string) (*protocol.CredentialCreation, string, error) {
	rpid := s.ResolveRPID(ctx, host)
	if rpid == "" {
		return nil, "", ErrRPNotAllowed
	}
	web, err := s.newWebAuthn(rpid, origin)
	if err != nil {
		return nil, "", err
	}
	wu, err := s.loadWebUser(ctx, user)
	if err != nil {
		return nil, "", err
	}
	creds, err := s.loadCredentials(ctx, user.ID)
	if err != nil {
		return nil, "", err
	}
	creation, session, err := web.BeginRegistration(wu,
		webauthn.WithResidentKeyRequirement(protocol.ResidentKeyRequirementRequired),
		webauthn.WithExclusions(webauthn.Credentials(creds).CredentialDescriptors()),
	)
	if err != nil {
		return nil, "", err
	}
	token := s.sessions.store(&ceremonySession{
		session:   *session,
		kind:      "register",
		userID:    user.ID,
		rpid:      rpid,
		origin:    origin,
		createdAt: time.Now(),
	})
	return creation, token, nil
}

// FinishRegistration 校验认证器响应并落库新凭证。
func (s *Service) FinishRegistration(ctx context.Context, user *entity.User, token string, rawJSON []byte, name string) (*entity.WebAuthnCredential, error) {
	cs, ok := s.sessions.take(token)
	if !ok || cs.kind != "register" || cs.userID != user.ID {
		return nil, fmt.Errorf("invalid or expired passkey registration session")
	}
	web, err := s.newWebAuthn(cs.rpid, cs.origin)
	if err != nil {
		return nil, err
	}
	wu, err := s.loadWebUser(ctx, user)
	if err != nil {
		return nil, err
	}
	cred, err := web.FinishRegistration(wu, cs.session, jsonBodyRequest(rawJSON))
	if err != nil {
		return nil, err
	}
	blob, err := json.Marshal(cred)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(name) == "" {
		name = fmt.Sprintf("%s 的 Passkey", user.Username)
	}
	row := &entity.WebAuthnCredential{
		UserID:         user.ID,
		RPID:           cs.rpid,
		CredentialID:   base64.RawURLEncoding.EncodeToString(cred.ID),
		CredentialJSON: string(blob),
		SignCount:      cred.Authenticator.SignCount,
		Transport:      transportsToString(cred.Transport),
		Attachment:     string(cred.Authenticator.Attachment),
		Name:           name,
	}
	if err := s.credRepo.Create(ctx, row); err != nil {
		return nil, err
	}
	return row, nil
}

// --- 登录（无需认证） ---

// BeginLogin 发起 discoverable（无用户名）登录仪式，返回断言选项与会话 token。
// 未认证阶段只认白名单已有条目 —— Host 伪造在这里拿不到任何东西。
func (s *Service) BeginLogin(ctx context.Context, host, origin string) (*protocol.CredentialAssertion, string, error) {
	rpid := s.ResolveRPID(ctx, host)
	if rpid == "" {
		return nil, "", ErrRPNotAllowed
	}
	web, err := s.newWebAuthn(rpid, origin)
	if err != nil {
		return nil, "", err
	}
	assertion, session, err := web.BeginDiscoverableLogin()
	if err != nil {
		return nil, "", err
	}
	token := s.sessions.store(&ceremonySession{
		session:   *session,
		kind:      "login",
		rpid:      rpid,
		origin:    origin,
		createdAt: time.Now(),
	})
	return assertion, token, nil
}

// FinishLogin 校验登录断言，返回被验证的用户（由 handler 随后签发 token）。
func (s *Service) FinishLogin(ctx context.Context, token string, rawJSON []byte) (*entity.User, error) {
	cs, ok := s.sessions.take(token)
	if !ok || cs.kind != "login" {
		return nil, fmt.Errorf("invalid or expired passkey login session")
	}
	web, err := s.newWebAuthn(cs.rpid, cs.origin)
	if err != nil {
		return nil, err
	}

	handler := func(rawID, userHandle []byte) (webauthn.User, error) {
		handle := base64.RawURLEncoding.EncodeToString(userHandle)
		user, err := s.userRepo.GetUserByWebAuthnHandle(ctx, handle)
		if err != nil {
			return nil, errors.New("unknown user handle")
		}
		if user.Status != 1 {
			return nil, errors.New("user disabled")
		}
		return s.loadWebUser(ctx, user)
	}

	validatedUser, validatedCred, err := web.FinishPasskeyLogin(handler, cs.session, jsonBodyRequest(rawJSON))
	if err != nil {
		return nil, err
	}
	user, ok := validatedUser.(*webUser)
	if !ok {
		return nil, fmt.Errorf("unexpected user type returned by handler")
	}
	// 反查真实用户实体（handler 内已确认可用）
	entityUser, err := s.userRepo.GetUserByWebAuthnHandle(ctx, base64.RawURLEncoding.EncodeToString(user.id))
	if err != nil {
		return nil, err
	}
	// 凭证整体回写（SignCount / flags 已随断言更新）
	if err := s.saveUpdatedCredential(ctx, entityUser.ID, cs.rpid, validatedCred); err != nil {
		return nil, err
	}
	return entityUser, nil
}

// --- 凭证管理（需已登录） ---

// ListByUser 返回用户的所有凭证（管理页面展示）。
func (s *Service) ListByUser(ctx context.Context, userID int64) ([]entity.WebAuthnCredential, error) {
	return s.credRepo.ListByUser(ctx, userID)
}

// DeleteCredential 删除用户的一把凭证。
func (s *Service) DeleteCredential(ctx context.Context, userID, id int64) error {
	if err := s.credRepo.Delete(ctx, id, userID); err != nil {
		return err
	}
	return nil
}

// --- 会话存储 ---

type ceremonySession struct {
	session   webauthn.SessionData
	kind      string // register | login
	userID    int64  // register 时绑定发起人
	rpid      string
	origin    string
	createdAt time.Time
}

type sessionStore struct {
	mu   sync.Mutex
	data map[string]*ceremonySession
}

func newSessionStore() *sessionStore {
	return &sessionStore{data: map[string]*ceremonySession{}}
}

// store 生成一次性随机 token 并保存会话。
func (st *sessionStore) store(cs *ceremonySession) string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	token := hex.EncodeToString(b)
	st.mu.Lock()
	st.data[token] = cs
	st.mu.Unlock()
	return token
}

// take 取出并删除会话（一次性）。过期同样视为无效。
func (st *sessionStore) take(token string) (*ceremonySession, bool) {
	st.mu.Lock()
	defer st.mu.Unlock()
	cs, ok := st.data[token]
	if !ok {
		return nil, false
	}
	delete(st.data, token)
	if time.Since(cs.createdAt) > sessionTTL {
		return nil, false
	}
	return cs, true
}

// runCleanup 周期性清理过期会话（防累积）。
func (st *sessionStore) runCleanup() {
	ticker := time.NewTicker(time.Minute)
	for range ticker.C {
		st.mu.Lock()
		for k, cs := range st.data {
			if time.Since(cs.createdAt) > sessionTTL {
				delete(st.data, k)
			}
		}
		st.mu.Unlock()
	}
}

// jsonBodyRequest 把客户端提交的原始 JSON 包装成 http.Request 供库解析用。
func jsonBodyRequest(raw []byte) *http.Request {
	return &http.Request{
		Body:   io.NopCloser(bytes.NewReader(raw)),
		Header: http.Header{},
	}
}