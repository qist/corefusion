package oauth

import "testing"

// SetPortalBasePath("/") 必须真的设成 "/" ✓：
// ⚠ 以前 TrimSuffix("/", "/") 把 "/" 削成空串、再被"空值不设置"守卫拒掉 ✗
// → 门户引擎沿用默认 /p ✗ → 独立门户域名回调跳到 域名/p（404 ✗）。
func TestSetPortalBasePath(t *testing.T) {
	h := NewHandler(nil, "test-secret")
	if h.portalPath != "/p" {
		t.Fatalf("默认应为 /p（主面板引擎 ✓），实际 %q", h.portalPath)
	}
	h.SetPortalBasePath("/")
	if h.portalPath != "/" {
		t.Fatalf(`SetPortalBasePath("/") 应设为 "/" ✓（独立门户引擎根路径 ✓），实际 %q`, h.portalPath)
	}
	h.SetPortalBasePath("/portal/")
	if h.portalPath != "/portal" {
		t.Fatalf("应去尾斜杠 ✓，实际 %q", h.portalPath)
	}
	h.SetPortalBasePath(" /p ")
	if h.portalPath != "/p" {
		t.Fatalf("应去空白 ✓，实际 %q", h.portalPath)
	}
	h.SetPortalBasePath("")
	if h.portalPath != "/p" {
		t.Fatalf("空值应保持现状 ✓，实际 %q", h.portalPath)
	}
}
