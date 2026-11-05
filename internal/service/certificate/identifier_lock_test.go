package certificate

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// 同一组标识符换顺序/换大小写必须落同一个 key ✓
// （否则 ECC 与 RSA 传进来的顺序不同就绕过了串行化 ✗）。
func TestIdentifierLockKeyOrderAndCaseInsensitive(t *testing.T) {
	a := identifierLockKey([]string{"Example.com", " www.example.com "})
	b := identifierLockKey([]string{"www.example.com", "example.com"})
	if a != b {
		t.Fatalf("同一组标识符必须归一成同一个 key：%q != %q", a, b)
	}
	if c := identifierLockKey([]string{"example.com", "other.example.com"}); c == a {
		t.Fatalf("不同的标识符集合不能共用 key：%q", c)
	}
}

// 同一组标识符的两次签发必须**串行**（不能同时进入 ACME 流程）✓
func TestLockIdentifiersSerializesSameSet(t *testing.T) {
	var inFlight int32
	var maxInFlight int32
	var wg sync.WaitGroup
	start := make(chan struct{})

	work := func() {
		defer wg.Done()
		<-start
		unlock := lockIdentifiers([]string{"203.0.113.10", "2001:db8::1"})
		defer unlock()
		cur := atomic.AddInt32(&inFlight, 1)
		for {
			max := atomic.LoadInt32(&maxInFlight)
			if cur <= max || atomic.CompareAndSwapInt32(&maxInFlight, max, cur) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		atomic.AddInt32(&inFlight, -1)
	}

	wg.Add(2)
	go work()
	go work()
	close(start)
	wg.Wait()

	if got := atomic.LoadInt32(&maxInFlight); got != 1 {
		t.Fatalf("同一组标识符必须串行，实际并发度 = %d", got)
	}
}

// 不同标识符集合之间不能互相阻塞 ✓（各签各的 ✓）。
func TestLockIdentifiersDifferentSetsDoNotBlock(t *testing.T) {
	unlock := lockIdentifiers([]string{"a.example.com"})
	defer unlock()

	done := make(chan struct{})
	go func() {
		u := lockIdentifiers([]string{"b.example.com"})
		u()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("不同标识符集合不应互相阻塞")
	}
}
