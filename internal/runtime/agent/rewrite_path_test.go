package agent

import "testing"

// 证书路径改写必须幂等：绝对路径再拼一次 dataDir 会得到 /data/data/certs/...
// （真机事故：xray 启动直接失败，整个核心所有入站一起挂）。
func TestRewritePathToLocalIdempotent(t *testing.T) {
	dataDir := "/opt/corefusion/data"
	cases := map[string]string{
		"certs/example.com/fullchain.pem":                        "/opt/corefusion/data/certs/example.com/fullchain.pem",
		"/opt/corefusion/data/certs/example.com/fullchain.pem":   "/opt/corefusion/data/certs/example.com/fullchain.pem",
		"/opt/corefusion/data/opt/corefusion/data/certs/a.pem": "/opt/corefusion/data/certs/a.pem",
		"":                           "",
		"/etc/ssl/private/other.pem": "/etc/ssl/private/other.pem",
	}
	for in, want := range cases {
		if got := rewritePathToLocal(in, dataDir); got != want {
			t.Errorf("rewritePathToLocal(%q) = %q, want %q", in, got, want)
		}
	}
}
