package netutil

import (
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

// withResolvConfPaths 临时替换候选文件列表，避免测试读到本机真实的 resolv.conf。
func withResolvConfPaths(t *testing.T, paths ...string) {
	t.Helper()
	old := resolvConfPaths
	resolvConfPaths = paths
	t.Cleanup(func() { resolvConfPaths = old })
}

func writeTempFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

// systemd-resolved 的上游文件必须优先于 /etc/resolv.conf 里的 stub 地址。
func TestReadNameserversPrefersSystemdResolvedUpstream(t *testing.T) {
	dir := t.TempDir()
	upstream := writeTempFile(t, dir, "resolved.conf", "# upstream\nnameserver 9.9.9.9\nnameserver 2001:4860:4860::8888\n")
	etc := writeTempFile(t, dir, "resolv.conf", "nameserver 127.0.0.53\n")
	withResolvConfPaths(t, upstream, etc)

	got := readNameservers()
	want := []string{"9.9.9.9:53", "[2001:4860:4860::8888]:53"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("readNameservers() = %v, want %v", got, want)
	}
}

func TestReadNameserversFallsBackToEtc(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing.conf")
	etc := writeTempFile(t, dir, "resolv.conf", "nameserver 192.168.2.84\nnameserver 192.168.2.89\n")
	withResolvConfPaths(t, missing, etc)

	got := readNameservers()
	want := []string{"192.168.2.84:53", "192.168.2.89:53"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("readNameservers() = %v, want %v", got, want)
	}
}

// 上游文件存在但没有 nameserver（只有注释/空行）时也要回退。
func TestReadNameserversFallsBackWhenUpstreamEmpty(t *testing.T) {
	dir := t.TempDir()
	empty := writeTempFile(t, dir, "resolved.conf", "# nothing here\n; nor here\n\n")
	etc := writeTempFile(t, dir, "resolv.conf", "nameserver 1.1.1.1\n")
	withResolvConfPaths(t, empty, etc)

	if got, want := readNameservers(), []string{"1.1.1.1:53"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("readNameservers() = %v, want %v", got, want)
	}
}

func TestReadNameserversNone(t *testing.T) {
	dir := t.TempDir()
	withResolvConfPaths(t, filepath.Join(dir, "missing1.conf"), filepath.Join(dir, "missing2.conf"))

	if got := readNameservers(); len(got) != 0 {
		t.Fatalf("readNameservers() = %v, want empty", got)
	}
}

func TestParseNameserversNormalizesAddresses(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "resolv.conf", `
nameserver 8.8.8.8
nameserver 8.8.4.4:5353
nameserver 2001:4860:4860::8888
nameserver [2606:4700:4700::1111]:53
# nameserver 127.0.0.1
`)
	got := parseNameservers(path)
	want := []string{
		"8.8.8.8:53",
		"8.8.4.4:5353",
		"[2001:4860:4860::8888]:53",
		"[2606:4700:4700::1111]:53",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseNameservers() = %v, want %v", got, want)
	}
}

// 默认客户端必须保留环境代理语义，只替换 DNS 解析。
func TestNewDefaultHTTPClientKeepsProxyAndPatchesDNS(t *testing.T) {
	client := NewDefaultHTTPClient(5 * time.Second)
	if client.Timeout != 5*time.Second {
		t.Fatalf("Timeout = %v, want 5s", client.Timeout)
	}
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Transport 类型 = %T, want *http.Transport", client.Transport)
	}
	if transport.Proxy == nil {
		t.Fatal("必须保留默认的 ProxyFromEnvironment 行为")
	}
	if transport.DialContext == nil {
		t.Fatal("DialContext 未设置")
	}
	// 必须是克隆出来的独立 transport（方法值的代码指针无法区分 receiver，故比较对象本身）。
	if transport == http.DefaultTransport {
		t.Fatal("不能直接修改 http.DefaultTransport，必须克隆")
	}
}

// 没有任何可用 nameserver 时必须保持系统默认解析，而不是把 Resolver 设为空实现。
func TestNewResolvedDialerFallsBackToSystemResolver(t *testing.T) {
	dir := t.TempDir()
	withResolvConfPaths(t, filepath.Join(dir, "missing.conf"))

	if d := newResolvedDialer(1); d.Resolver != nil {
		t.Fatalf("expected nil Resolver fallback, got %#v", d.Resolver)
	}
}
