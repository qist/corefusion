package middleware

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// setTestIPBlacklist 直填缓存（绕过 DB 加载）供单测使用。
func setTestIPBlacklist(t *testing.T, raw string) {
	t.Helper()
	rules, ips := parseIPBlacklist(raw)
	globalIPBlacklist.mu.Lock()
	globalIPBlacklist.rules = rules
	globalIPBlacklist.ips = ips
	globalIPBlacklist.loadedAt = time.Now()
	globalIPBlacklist.mu.Unlock()
	t.Cleanup(func() {
		globalIPBlacklist.mu.Lock()
		globalIPBlacklist.rules = nil
		globalIPBlacklist.ips = nil
		globalIPBlacklist.loadedAt = time.Time{}
		globalIPBlacklist.mu.Unlock()
	})
}

func TestParseIPBlacklist(t *testing.T) {
	rules, ips := parseIPBlacklist("# comment\n203.0.113.10\n198.51.100.0/24\n\nnot-an-ip\n10.0.0.1, 2001:db8::1/64")
	if len(ips) != 2 {
		t.Fatalf("expected 2 single IPs, got %d", len(ips))
	}
	if len(rules) != 2 {
		t.Fatalf("expected 2 CIDR rules, got %d", len(rules))
	}
	if !ips[0].Equal(net.ParseIP("203.0.113.10")) {
		t.Errorf("unexpected ip %v", ips[0])
	}
}

func TestIsIPBlacklisted(t *testing.T) {
	setTestIPBlacklist(t, "203.0.113.10\n198.51.100.0/24")
	cases := map[string]bool{
		"203.0.113.10":  true,  // 精确命中
		"203.0.113.11":  false, // 同网段但只封了单 IP
		"198.51.100.55": true,  // CIDR 命中
		"198.51.101.1":  false, // CIDR 外
		"":              false,
		"not-an-ip":     false,
	}
	for ip, want := range cases {
		if got := isIPBlacklisted(ip); got != want {
			t.Errorf("isIPBlacklisted(%q) = %v, want %v", ip, got, want)
		}
	}
}

func TestIPBlacklistGuardBlocks(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setTestIPBlacklist(t, "203.0.113.7")
	r := gin.New()
	r.Use(IPBlacklistGuard(nil))
	r.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") })

	// 命中黑名单 → 403
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/ping", nil)
	req.RemoteAddr = "203.0.113.7:12345"
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for blacklisted IP, got %d", w.Code)
	}

	// 未命中 → 200
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("GET", "/ping", nil)
	req2.RemoteAddr = "8.8.4.4:12345"
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 for clean IP, got %d", w2.Code)
	}

	// 空黑名单 → 全放行
	setTestIPBlacklist(t, "")
	w3 := httptest.NewRecorder()
	req3 := httptest.NewRequest("GET", "/ping", nil)
	req3.RemoteAddr = "203.0.113.7:12345"
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Fatalf("expected 200 with empty blacklist, got %d", w3.Code)
	}
}
