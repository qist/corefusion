package agent

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

func newDownloadTestHandler(t *testing.T) (*Handler, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	agentDir := filepath.Join(dir, "binaries", "active", "corefusion-agent")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	bin := make([]byte, 2<<20)
	copy(bin, []byte{0x7f, 'E', 'L', 'F'})
	if err := os.WriteFile(filepath.Join(agentDir, "corefusion-agent-linux-amd64"), bin, 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	if err := os.WriteFile(filepath.Join(agentDir, "VERSION"), []byte("v1.0.0\n"), 0o644); err != nil {
		t.Fatalf("write version: %v", err)
	}
	return &Handler{dataDir: dir}, agentDir
}

func requestDownload(h *Handler, query string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/agent/download"+query, nil)
	h.DownloadAgent(c)
	return w
}

// 本地有同版本二进制 → 直接下发（面板优先，内网/开发环境走这条）。
func TestDownloadAgentServesMatchingVersion(t *testing.T) {
	h, _ := newDownloadTestHandler(t)
	w := requestDownload(h, "?os=linux&arch=amd64&version=v1.0.0")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if w.Body.Len() < 1<<20 {
		t.Errorf("下载内容过小: %d", w.Body.Len())
	}
}

// 本地版本不匹配 → 404，让安装脚本回退 GitHub（避免新旧 Agent 协议不兼容）。
func TestDownloadAgentRejectsVersionMismatch(t *testing.T) {
	h, _ := newDownloadTestHandler(t)
	w := requestDownload(h, "?os=linux&arch=amd64&version=v9.9.9")
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", w.Code, w.Body.String())
	}
	if !contains(w.Body.String(), "version mismatch") {
		t.Errorf("应返回版本不匹配原因，实际 %s", w.Body.String())
	}
}

// 文件不是 ELF（例如被 CDN 拦成 HTML）→ 视为不存在，脚本会换 GitHub。
func TestDownloadAgentRejectsNonELF(t *testing.T) {
	h, agentDir := newDownloadTestHandler(t)
	if err := os.WriteFile(filepath.Join(agentDir, "corefusion-agent-linux-amd64"), []byte("<html>blocked</html>"), 0o755); err != nil {
		t.Fatalf("overwrite: %v", err)
	}
	w := requestDownload(h, "?os=linux&arch=amd64")
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", w.Code, w.Body.String())
	}
}

// 不带 version（旧脚本/手工下载）仍按「本地有就给」处理。
func TestDownloadAgentWithoutVersionServesLocal(t *testing.T) {
	h, _ := newDownloadTestHandler(t)
	w := requestDownload(h, "?os=linux&arch=amd64")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
