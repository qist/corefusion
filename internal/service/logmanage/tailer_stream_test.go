package logmanage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// 日志查看器首连已经用普通接口加载过尾部快照，SSE 传 lines=0 时不能再回放历史行，
// 且必须立即进入持续 tail（小文件也照常追加），不能等日志攒够多少行才启动。
func TestTailLogStreamWithZeroSkipsHistoryAndTails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "probe.log")
	if err := os.WriteFile(path, []byte("old-1\nold-2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := &Service{dataDir: dir}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := make(chan string, 8)
	go func() { _ = svc.TailLogStreamWithType(ctx, path, "", 0, ch) }()

	select {
	case line := <-ch:
		t.Fatalf("lines=0 must not replay history, got %q", line)
	case <-time.After(300 * time.Millisecond):
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("new-1\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()

	select {
	case line := <-ch:
		if line != "new-1" {
			t.Fatalf("got %q, want new-1", line)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for appended line (tail did not start)")
	}
}
