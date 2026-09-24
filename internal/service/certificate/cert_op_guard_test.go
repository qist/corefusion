package certificate

import (
	"context"
	"testing"
	"time"
)

type certOpCtxTestKey string

// TestDetachedCertOpContextSurvivesClientDisconnect 覆盖"证书签出来了、库里却没有"的核心机制 ✓。
//
// 真机 2026-09-23 的现场 ✓：dns-01 等 DNS 传播 ~2 分钟 ✗ → 前端/反代断开 ✗ →
// `c.Request.Context()` 被取消 ✗ → CA 已经把证书签出来 ✓（日志有 `Server responded with a certificate` ✓、
// 磁盘也有产物 ✓），**紧接着的落库那一步却直接失败** ✗ → 接口 500、DB 里没有这张证书 ✗。
// 修复口径：签发/续期开跑后，上下文**只保留值、丢掉取消** ✓ → 一定要落库 ✓。
func TestDetachedCertOpContextSurvivesClientDisconnect(t *testing.T) {
	// 模拟"断开"：父上下文先被取消 ✓（值仍然挂着 ✓，审计/追踪要靠它 ✓）
	parent, cancelReq := context.WithCancel(context.Background())
	parent = context.WithValue(parent, certOpCtxTestKey("audit"), "trace-id")
	cancelReq()

	opCtx, cancelOp := detachedCertOpContext(parent)
	defer cancelOp()

	if err := opCtx.Err(); err != nil {
		t.Fatalf("客户端断开后，签发/续期上下文必须仍然可用（否则证书签出来也落不了库 ✓），实际 err=%v", err)
	}
	if got := opCtx.Value(certOpCtxTestKey("audit")); got != "trace-id" {
		t.Fatalf("必须保留上下文里的值 ✓，实际 %v", got)
	}

	// 兜底超时：防止任务永远挂着 ✓（但不是"请求一断就取消" ✓）
	deadline, ok := opCtx.Deadline()
	if !ok {
		t.Fatal("必须带兜底超时（否则一个卡死的签发会永久占着 goroutine）")
	}
	if remaining := time.Until(deadline); remaining <= 0 || remaining > 16*time.Minute {
		t.Fatalf("兜底超时应为 15 分钟量级 ✓，实际剩余 %v", remaining)
	}

	// 兜底超时只影响任务本身 ✓：显式取消仍然有效 ✓（任务跑完时 defer cancel() 会立刻释放 ✓）
	cancelOp()
	if opCtx.Err() == nil {
		t.Fatal("显式 cancel 之后必须结束（defer cancel 是正常收尾）")
	}
}

// TestBeginCertOpRejectsConcurrentSameCert 覆盖"同一张证书不允许两个签发/续期并发" ✓。
//
// 为什么必须有它 ✗：签发/续期被刻意脱离请求上下文后 ✓，客户端断开任务仍在跑 ✓ ——
// 用户看界面没反应又点一次 ✓ / 自动续期与手动续期撞上 ✓，就会变成两个续期同时
// 写同一份证书文件 + 各自 Update 同一行 ✗（落库内容与磁盘/最终证书可能对不上 ✗，还白耗 CA 配额 ✗）。
func TestBeginCertOpRejectsConcurrentSameCert(t *testing.T) {
	s := &Service{}

	done, ok := s.beginCertOp("renew:20")
	if !ok {
		t.Fatal("第一次操作必须能上锁 ✓")
	}
	if _, again := s.beginCertOp("renew:20"); again {
		t.Fatal("同一张证书的第二次操作必须被拒 ✓（否则两边都写文件、都落库 ✗）")
	}

	// 不同证书之间不能互相阻塞 ✓（各签各的 ✓）
	otherDone, okOther := s.beginCertOp("renew:21")
	if !okOther {
		t.Fatal("不同证书的操作不应互相阻塞 ✓")
	}
	otherDone()

	// 释放之后必须能再次上锁 ✓（否则一次失败会把这张证书永久锁死 ✗）
	done()
	if _, retry := s.beginCertOp("renew:20"); !retry {
		t.Fatal("释放后必须能再次上锁 ✓")
	}
}
