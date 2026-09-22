package auth

import (
	"testing"
	"time"
)

// 安全审计 v5 M10：浏览器无法给 SSE/下载带 Authorization 头 → 前端只能把凭证放 URL ✗。
// 票据必须同时满足：短时、一次性、限作用域。
func TestTicketIssueAndConsume(t *testing.T) {
	s := NewTicketStore(time.Minute)
	tok, exp, err := s.Issue(TicketOptions{
		Scope: TicketScopeLogs, Kind: TicketKindAdmin,
		UserID: 7, Username: "admin", Role: "admin", TokenVersion: 3,
	})
	if err != nil {
		t.Fatalf("签发失败：%v", err)
	}
	if len(tok) != 64 {
		t.Errorf("票据应为 32 字节 hex（64 字符），实际 %d 字符", len(tok))
	}
	if time.Until(exp) <= 0 || time.Until(exp) > time.Minute+time.Second {
		t.Errorf("过期时间不合理：%v", exp)
	}

	got, ok := s.Consume(tok, TicketScopeLogs)
	if !ok {
		t.Fatal("首次使用应通过")
	}
	if got.UserID != 7 || got.Role != "admin" || got.TokenVersion != 3 || got.Kind != TicketKindAdmin {
		t.Errorf("票据内容不符：%+v", got)
	}

	// 一次性：同一票据不能再用
	if _, ok := s.Consume(tok, TicketScopeLogs); ok {
		t.Error("票据必须是一次性的（可重放就等于长期凭证）")
	}
}

func TestTicketScopeAndExpiry(t *testing.T) {
	s := NewTicketStore(time.Minute)

	// 作用域必须完全匹配：日志票据不能拿来下备份
	logTicket, _, err := s.Issue(TicketOptions{Scope: TicketScopeLogs, Kind: TicketKindAdmin, UserID: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Consume(logTicket, TicketScopeBackupDownload); ok {
		t.Error("作用域不符必须拒绝（否则日志票据能下载备份）")
	}
	// 上面那次失败尝试已把票据作废 → 正确作用域也不能再用（防重放）
	if _, ok := s.Consume(logTicket, TicketScopeLogs); ok {
		t.Error("校验失败的尝试也必须作废票据（防重放）")
	}

	// 未知票据
	if _, ok := s.Consume("deadbeef", TicketScopeLogs); ok {
		t.Error("不存在的票据必须拒绝")
	}
	// 空票据 / 空作用域
	if _, ok := s.Consume("", TicketScopeLogs); ok {
		t.Error("空票据必须拒绝")
	}
	empty, _, _ := s.Issue(TicketOptions{Scope: TicketScopeLogs})
	if _, ok := s.Consume(empty, ""); ok {
		t.Error("空作用域必须拒绝（否则票据可用于任何路径）")
	}

	// 过期
	expired := NewTicketStore(time.Minute)
	past := time.Now().Add(-2 * time.Minute)
	expired.now = func() time.Time { return past }
	old, _, err := expired.Issue(TicketOptions{Scope: TicketScopeLogs, UserID: 1})
	if err != nil {
		t.Fatal(err)
	}
	expired.now = time.Now
	if _, ok := expired.Consume(old, TicketScopeLogs); ok {
		t.Error("过期票据必须拒绝")
	}
}

func TestTicketTTLAndLimit(t *testing.T) {
	if got := NewTicketStore(0).TTL(); got != 60*time.Second {
		t.Errorf("默认 TTL 应为 60s，实际 %v", got)
	}
	if got := NewTicketStore(90 * time.Second).TTL(); got != 90*time.Second {
		t.Errorf("TTL 应可配置，实际 %v", got)
	}

	// 未消费的票据不能被无限堆积
	s := NewTicketStore(time.Minute)
	for i := 0; i < 4096; i++ {
		if _, _, err := s.Issue(TicketOptions{Scope: TicketScopeLogs, UserID: int64(i)}); err != nil {
			t.Fatalf("第 %d 张就失败了：%v", i, err)
		}
	}
	if _, _, err := s.Issue(TicketOptions{Scope: TicketScopeLogs, UserID: 1}); err == nil {
		t.Error("超过上限应拒绝（防止无限堆积）")
	}
	if s.Len() == 0 {
		t.Error("Len 应反映当前票据数")
	}

	// 过期票据会被回收
	s2 := NewTicketStore(time.Minute)
	var past = time.Now().Add(-2 * time.Minute)
	s2.now = func() time.Time { return past }
	if _, _, err := s2.Issue(TicketOptions{Scope: TicketScopeLogs}); err != nil {
		t.Fatal(err)
	}
	s2.now = time.Now
	if got := s2.Len(); got != 0 {
		t.Errorf("过期票据应被回收，实际剩 %d 张", got)
	}
}

// 客户端票据（日志包下载）与管理端票据共用仓库但作用域不同。
func TestTicketClientKind(t *testing.T) {
	s := NewTicketStore(time.Minute)
	tok, _, err := s.Issue(TicketOptions{Scope: TicketScopeClientDownload, Kind: TicketKindClient, ClientID: 42})
	if err != nil {
		t.Fatal(err)
	}
	got, ok := s.Consume(tok, TicketScopeClientDownload)
	if !ok || got.ClientID != 42 || got.Kind != TicketKindClient {
		t.Errorf("客户端票据不符：ok=%v %+v", ok, got)
	}
	if _, ok := s.Consume(tok, TicketScopeBackupDownload); ok {
		t.Error("客户端票据不得用于管理端下载")
	}
}
