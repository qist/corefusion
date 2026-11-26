package ratelimit

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestNftKernelUnsupported(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{fmt.Errorf(`nft add table inet x: exit status 1 (Error: Could not process rule: Operation not supported)`), true},
		{fmt.Errorf(`nft list set inet x y: exit status 1 (Error: No such file or directory)`), false},
		{errors.New("protocol not supported"), true},
		{errors.New("permission denied"), true},
		{nil, false},
	}
	for _, c := range cases {
		if got := nftKernelUnsupported(c.err); got != c.want {
			t.Fatalf("nftKernelUnsupported(%v) = %v, want %v", c.err, got, c.want)
		}
	}
}

// 没有 nft 命令时应立刻返回 ErrNftUnavailable，面板据此停止下发。
func TestApplyIPBlacklistWithoutNft(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	err := ApplyIPBlacklist([]string{"192.0.2.1"})
	if !errors.Is(err, ErrNftUnavailable) {
		t.Fatalf("应返回 ErrNftUnavailable，实际 %v", err)
	}
	if !strings.Contains(err.Error(), "NFT_UNAVAILABLE") {
		t.Fatalf("错误文案需带 NFT_UNAVAILABLE 标记（远端按字符串识别）：%v", err)
	}
}

func TestErrSkippedIsDistinct(t *testing.T) {
	if errors.Is(ErrSkipped, ErrNftUnavailable) || errors.Is(ErrNftUnavailable, ErrSkipped) {
		t.Fatal("ErrSkipped 与 ErrNftUnavailable 不应互相匹配")
	}
}
