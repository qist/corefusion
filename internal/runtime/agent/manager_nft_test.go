package agent

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"corefusion/internal/runtime/agent/ratelimit"
)

func TestErrIsNftUnavailable(t *testing.T) {
	if errIsNftUnavailable(nil) {
		t.Fatal("nil 不应判定为 nft 缺失")
	}
	// 远端 Agent 的失败原因只透传字符串
	if !errIsNftUnavailable(fmt.Errorf("NFT_UNAVAILABLE: nft command not found")) {
		t.Fatal("应识别远端透传的 NFT_UNAVAILABLE 标记")
	}
	// 本地节点走 errors.Is
	if !errIsNftUnavailable(fmt.Errorf("apply: %w", ratelimit.ErrNftUnavailable)) {
		t.Fatal("应识别包装后的 ErrNftUnavailable")
	}
	if errIsNftUnavailable(errors.New("command timed out after 15s")) {
		t.Fatal("普通错误不应误判")
	}
}

func TestNftUnsupportedCache(t *testing.T) {
	m := &NodeServerManager{}
	const id = int64(19)
	if m.nftUnsupportedRecently(id) {
		t.Fatal("初始状态不应命中")
	}
	m.markNftUnsupported(id)
	if !m.nftUnsupportedRecently(id) {
		t.Fatal("标记后应命中并跳过下发")
	}
	// 其他节点不受影响
	if m.nftUnsupportedRecently(20) {
		t.Fatal("不应影响其他节点")
	}
	// 过期后自动重试
	m.nftUnsupportedMu.Lock()
	m.nftUnsupported[id] = time.Now().Add(-nftUnsupportedTTL - time.Minute)
	m.nftUnsupportedMu.Unlock()
	if m.nftUnsupportedRecently(id) {
		t.Fatal("超过 TTL 后应重新尝试")
	}
	// 成功推送后清除标记
	m.markNftUnsupported(id)
	m.clearNftUnsupported(id)
	if m.nftUnsupportedRecently(id) {
		t.Fatal("清除后不应命中")
	}
}

// 兼容旧 Agent（还没有 NFT_UNAVAILABLE 标记）的原始 exec 错误。
func TestErrIsNftUnavailableLegacy(t *testing.T) {
	legacy := errors.New(`nft set ip_blacklist unavailable (create failed?): exec: "nft": executable file not found in $PATH`)
	if !errIsNftUnavailable(legacy) {
		t.Fatal("旧 Agent 的原始 nft 缺失错误也应识别")
	}
	if errIsNftUnavailable(errors.New(`nft add element: exit status 1`)) {
		t.Fatal("普通 nft 失败（如元素非法）不应被当成不支持")
	}
}
