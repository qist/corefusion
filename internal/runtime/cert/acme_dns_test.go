package cert

import (
	"context"
	"net"
	"net/http"
	"reflect"
	"testing"
	"time"
)

// ACME 出网必须用本地上游 nameserver 解析，同时不能改动 lego 的 proxy/TLS 配置。
func TestApplyResolvedDNSKeepsTransportSettings(t *testing.T) {
	originalDial := func(context.Context, string, string) (net.Conn, error) { return nil, nil }
	original := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		DialContext:         originalDial,
		TLSHandshakeTimeout: 12 * time.Second,
		MaxIdleConns:        7,
	}
	client := &http.Client{Timeout: 2 * time.Minute, Transport: original}

	applyResolvedDNS(client)

	patched, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Transport 类型 = %T, want *http.Transport", client.Transport)
	}
	if patched == original {
		t.Fatal("必须克隆 transport，不能原地修改共享对象")
	}
	if patched.Proxy == nil {
		t.Fatal("Proxy 设置必须保留（环境代理语义不能变）")
	}
	if patched.TLSHandshakeTimeout != 12*time.Second || patched.MaxIdleConns != 7 {
		t.Fatalf("其它 transport 设置丢失: %+v", patched)
	}
	if reflect.ValueOf(patched.DialContext).Pointer() == reflect.ValueOf(originalDial).Pointer() {
		t.Fatal("DialContext 未被替换，DNS 修复不生效")
	}
	if reflect.ValueOf(original.DialContext).Pointer() != reflect.ValueOf(originalDial).Pointer() {
		t.Fatal("原 transport 的 DialContext 被修改")
	}
	if client.Timeout != 2*time.Minute {
		t.Fatalf("client.Timeout = %v, want 2m", client.Timeout)
	}
}

func TestApplyResolvedDNSIgnoresNilAndUnknownTransport(t *testing.T) {
	applyResolvedDNS(nil)

	rt := &fakeRoundTripper{}
	client := &http.Client{Transport: rt}
	applyResolvedDNS(client)
	if client.Transport != rt {
		t.Fatal("非 *http.Transport 的 transport 不应被替换")
	}
}

type fakeRoundTripper struct{}

func (f *fakeRoundTripper) RoundTrip(*http.Request) (*http.Response, error) { return nil, nil }
