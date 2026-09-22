package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

// 票据作用域 —— 票据只对**特定用途**有效（见 middleware 里的"路径 → 作用域"表）：
// 即使日志票据泄漏，也不能拿去下载备份。
const (
	TicketScopeLogs           = "logs"            // 管理端：核心/代理日志（含 SSE 流）
	TicketScopeBackupDownload = "backup_download" // 管理端：备份文件下载
	TicketScopeClientDownload = "client_download" // 客户端：日志包下载
)

// 票据主体类型。
const (
	TicketKindAdmin  = "admin"
	TicketKindClient = "client"
)

// Ticket 是一次性短时票据（安全审计 v5 M10）。
//
// 为什么要它：浏览器的 `EventSource`（SSE）与 `window.open`（文件下载）**无法自定义
// Authorization 头** ✗，所以前端只能把凭证塞进 URL ✗ —— 而 URL 会进访问日志、Referer、
// 浏览器历史、代理日志，等于把**长期有效的 JWT**到处散播 ✗。
//
// 票据的三个关键性质：
//  1. **短时**：默认 60 秒（够一次跳转/一次建流）；
//  2. **一次性**：用完立即作废（`Consume` 无论校验是否通过都删除 → 防重放）；
//  3. **限作用域 + 限路径**：票据只在其作用域对应的路径上被接受（middleware 里按路径选 scope）。
type Ticket struct {
	Scope        string
	Kind         string
	UserID       int64
	Username     string
	Role         string
	TokenVersion int
	ClientID     int64
	ExpiresAt    time.Time
}

// TicketOptions 是签发票据的入参（零值即"未设置"）。
type TicketOptions struct {
	Scope        string
	Kind         string
	UserID       int64
	Username     string
	Role         string
	TokenVersion int
	ClientID     int64
}

// TicketStore 是进程内的票据仓库。
//
// 故意放内存（不放 DB）：票据生命周期只有几十秒、重启即失效是**期望行为**，
// 且避免每次 SSE 建流都写库。多副本部署时注意：票据需落在同一个实例上
// （当前面板是单实例，够用；将来多副本需换成共享存储）。
type TicketStore struct {
	mu         sync.Mutex
	ttl        time.Duration
	now        func() time.Time
	tickets    map[string]Ticket
	maxEntries int
}

// NewTicketStore 创建票据仓库；ttl <= 0 时用默认 60 秒。
func NewTicketStore(ttl time.Duration) *TicketStore {
	if ttl <= 0 {
		ttl = 60 * time.Second
	}
	return &TicketStore{
		ttl:        ttl,
		now:        time.Now,
		tickets:    make(map[string]Ticket),
		maxEntries: 4096,
	}
}

// TTL 返回票据有效期。
func (s *TicketStore) TTL() time.Duration {
	if s == nil {
		return 0
	}
	return s.ttl
}

// Issue 签发一张票据（32 字节 CSPRNG 随机数，hex 编码）。
func (s *TicketStore) Issue(opts TicketOptions) (string, time.Time, error) {
	if s == nil {
		return "", time.Time{}, errors.New("ticket store not configured")
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", time.Time{}, err
	}
	token := hex.EncodeToString(buf)
	expires := s.now().Add(s.ttl)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.gcLocked()
	if len(s.tickets) >= s.maxEntries {
		return "", time.Time{}, errors.New("too many outstanding tickets")
	}
	s.tickets[token] = Ticket{
		Scope:        opts.Scope,
		Kind:         opts.Kind,
		UserID:       opts.UserID,
		Username:     opts.Username,
		Role:         opts.Role,
		TokenVersion: opts.TokenVersion,
		ClientID:     opts.ClientID,
		ExpiresAt:    expires,
	}
	return token, expires, nil
}

// Consume 校验并**作废**票据。scope 必须完全匹配（票据只对一种用途有效）。
//
// 注意删除时机：只要票据存在就先删除 ✗→ 这样即使随后因过期/作用域不符而失败，
// 攻击者也无法把同一张票据反复试（防重放）。
func (s *TicketStore) Consume(token, scope string) (Ticket, bool) {
	if s == nil || token == "" {
		return Ticket{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tickets[token]
	if !ok {
		return Ticket{}, false
	}
	delete(s.tickets, token)
	if s.now().After(t.ExpiresAt) {
		return Ticket{}, false
	}
	if scope == "" || t.Scope != scope {
		return Ticket{}, false
	}
	return t, true
}

// gcLocked 清理过期票据（调用方须持锁）。
func (s *TicketStore) gcLocked() {
	now := s.now()
	for k, v := range s.tickets {
		if now.After(v.ExpiresAt) {
			delete(s.tickets, k)
		}
	}
}

// Len 返回当前未过期票据数量（测试/观测用）。
func (s *TicketStore) Len() int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gcLocked()
	return len(s.tickets)
}
