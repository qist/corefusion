package agent

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"testing"

	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
)

func ctxWithCert(ous ...string) context.Context {
	cert := &x509.Certificate{Subject: pkix.Name{Organization: []string{"CoreFusion"}, OrganizationalUnit: ous}}
	return peer.NewContext(context.Background(), &peer.Peer{
		AuthInfo: credentials.TLSInfo{State: tls.ConnectionState{
			VerifiedChains: [][]*x509.Certificate{{cert}},
		}},
	})
}

func TestCertServerID(t *testing.T) {
	cases := []struct {
		name string
		ous  []string
		want int64
		ok   bool
	}{
		{"标准形状", []string{"server-7"}, 7, true},
		{"多个 OU 里挑出身份", []string{"ops", "server-42", "x"}, 42, true},
		{"前缀但非法数字", []string{"server-abc"}, 0, false},
		{"前缀但 0", []string{"server-0"}, 0, false},
		{"没有身份 OU", []string{"ops"}, 0, false},
		{"空", nil, 0, false},
	}
	for _, c := range cases {
		got, ok := certServerID(&x509.Certificate{Subject: pkix.Name{OrganizationalUnit: c.ous}})
		if got != c.want || ok != c.ok {
			t.Errorf("%s：得到 (%d,%v)，期望 (%d,%v)", c.name, got, ok, c.want, c.ok)
		}
	}
	if _, ok := certServerID(nil); ok {
		t.Error("nil 证书必须返回 false")
	}
}

// 安全审计 v5 M3：Agent 上报的 server_id 必须与证书身份一致，否则任一节点可冒充其它节点。
func TestVerifyPeerServerID(t *testing.T) {
	// ① 一致 → 通过
	if err := verifyPeerServerID(ctxWithCert("server-5"), 5); err != nil {
		t.Errorf("证书与上报一致时应通过，得到 %v", err)
	}

	// ② 不一致（冒充）→ 必须拒绝
	err := verifyPeerServerID(ctxWithCert("server-5"), 6)
	if err == nil {
		t.Fatal("证书说 server-5、上报 server-6 必须被拒（否则可冒充其它节点）")
	}

	// ③ 取不到身份（无对端信息）→ 失败关闭
	if err := verifyPeerServerID(context.Background(), 5); err == nil {
		t.Error("无法确定身份时必须拒绝建流（失败关闭）")
	}
	// ④ 证书里没有身份 OU → 拒绝
	if err := verifyPeerServerID(ctxWithCert("ops"), 5); err == nil {
		t.Error("证书缺少身份 OU 时必须拒绝")
	}
}
