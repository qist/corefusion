package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// 安全审计 v5 M5：自更新的校验和不能"没人填就跳过" ✗，全程自动：
// 面板下发优先 → 否则自动取 `downloadURL + ".sha256"` → 都拿不到就**拒绝更新**。
func TestNormalizeChecksum(t *testing.T) {
	const h = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	ok := []string{h, "  " + h + "  ", "sha256:" + h, "SHA256:" + h, "sha256: " + h}
	for _, s := range ok {
		if got := normalizeChecksum(s); got != h {
			t.Errorf("normalizeChecksum(%q) = %q，期望 %q", s, got, h)
		}
	}
	bad := []string{"", "not-a-hash", h[:63], h + "aa", "zzzz" + h[4:]}
	for _, s := range bad {
		if got := normalizeChecksum(s); got != "" {
			t.Errorf("normalizeChecksum(%q) 应为空，得到 %q", s, got)
		}
	}
}

func TestParseChecksumFile(t *testing.T) {
	const h = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	cases := []struct{ name, in, want string }{
		{"sha256sum 格式", h + "  corefusion-agent-linux-amd64\n", h},
		{"只有 hash", h + "\n", h},
		{"带 sha256: 前缀", "sha256:" + h + "\n", h},
		{"空文件", "", ""},
		{"404 HTML", "<html>Not Found</html>", ""},
	}
	for _, c := range cases {
		if got := parseChecksumFile([]byte(c.in)); got != c.want {
			t.Errorf("%s：得到 %q，期望 %q", c.name, got, c.want)
		}
	}
}

func TestVerifyChecksum(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "bin")
	content := []byte("hello-agent-binary")
	if err := os.WriteFile(p, content, 0600); err != nil {
		t.Fatal(err)
	}
	u := &SelfUpdater{dataDir: dir}

	// 真实校验和（"hello-agent-binary" 的 sha256）— 用工具函数算出来，避免手抄
	sum := sha256Hex(t, content)
	if err := u.verifyChecksum(p, sum); err != nil {
		t.Errorf("正确校验和应当通过：%v", err)
	}
	if err := u.verifyChecksum(p, "sha256:"+sum); err != nil {
		t.Errorf("带 sha256: 前缀应当通过：%v", err)
	}
	if err := u.verifyChecksum(p, sha256Hex(t, []byte("tampered"))); err == nil {
		t.Error("校验和不匹配必须报错（否则被篡改的二进制会被装上）")
	}
}

func TestResolveChecksumAutoFetchAndFailClosed(t *testing.T) {
	const h = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	u := &SelfUpdater{dataDir: t.TempDir()}

	// ① 面板已经带了 checksum → 直接用，不需要任何人工填写
	if got, err := u.resolveChecksum("https://example.invalid/a", "sha256:"+h); err != nil || got != h {
		t.Fatalf("下发的 checksum 应被直接采用：got=%q err=%v", got, err)
	}

	// ② 没带 → 自动取 `downloadURL + .sha256`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/agent.sha256" {
			_, _ = w.Write([]byte(h + "  corefusion-agent-linux-amd64\n"))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	got, err := u.resolveChecksum(srv.URL+"/agent", "")
	if err != nil || got != h {
		t.Fatalf("应自动从 .sha256 取到校验和：got=%q err=%v", got, err)
	}

	// ③ 旁挂文件不存在 → 必须报错（调用方据此拒绝更新，保持当前版本）
	if _, err := u.resolveChecksum(srv.URL+"/missing", ""); err == nil {
		t.Error("拿不到校验和时必须报错（不能静默跳过校验后直接替换二进制）")
	}

	// ④ 旁挂文件是垃圾内容 → 同样必须报错
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html>nope</html>"))
	}))
	defer srv2.Close()
	if _, err := u.resolveChecksum(srv2.URL+"/agent", ""); err == nil {
		t.Error("旁挂文件里没有合法 SHA-256 时必须报错")
	}
}

// 下载临时文件必须落在 Agent 自己的数据目录里（不再是全世界可写的 /tmp 固定名）。
func TestDownloadFileUsesDataDirTempWith0600(t *testing.T) {
	dir := t.TempDir()
	payload := []byte("fake-binary-bytes")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	u := &SelfUpdater{dataDir: dir}
	tmp, err := u.downloadFile(srv.URL + "/agent")
	if err != nil {
		t.Fatalf("downloadFile: %v", err)
	}
	defer os.Remove(tmp)

	if filepath.Dir(tmp) != dir {
		t.Errorf("临时文件应在数据目录内（%s），实际 %s", dir, tmp)
	}
	if filepath.Base(tmp) == "corefusion-agent-new" {
		t.Error("不应再使用可预测的固定文件名")
	}
	info, err := os.Stat(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("临时文件权限应为 0600，实际 %v", info.Mode().Perm())
	}
	got, err := os.ReadFile(tmp)
	if err != nil || string(got) != string(payload) {
		t.Errorf("内容不符：%q err=%v", got, err)
	}
}

func sha256Hex(t *testing.T, b []byte) string {
	t.Helper()
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
