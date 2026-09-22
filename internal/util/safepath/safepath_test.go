package safepath

import (
	"path/filepath"
	"strings"
	"testing"
)

// 这些用例直接对应安全审计 v5 的 H2/H3/H4：Agent 以 root 运行，面板下发的文件名/路径片段
// 一旦含 `..` 或分隔符，`filepath.Join` 就会逃出目标目录 → 以 root 读写任意文件。
func TestSafeName(t *testing.T) {
	ok := []string{"xray", "singbox", "url.rule", "domain.json", "config.lua", "a-b_c.d"}
	for _, s := range ok {
		if !SafeName(s) {
			t.Errorf("SafeName(%q) 应为 true（是合法的单独名字）", s)
		}
	}
	bad := []string{
		"", ".", "..",
		"../../../etc/passwd",
		"a/b",
		`a\b`,
		"/etc/passwd",
		"foo/..",
		"bad\x00name",
	}
	for _, s := range bad {
		if SafeName(s) {
			t.Errorf("SafeName(%q) 应为 false（可逃出目标目录或非法）", s)
		}
	}
}

func TestCheckName(t *testing.T) {
	if err := CheckName("geo rule file name", "url.rule"); err != nil {
		t.Errorf("合法名字不应报错：%v", err)
	}
	err := CheckName("geo rule file name", "../../../etc/cron.d/pwn")
	if err == nil {
		t.Fatal("含上跳的名字必须被拒")
	}
	if !strings.Contains(err.Error(), "geo rule file name") {
		t.Errorf("错误信息应带上下文，得到 %v", err)
	}
}

func TestRejectTraversal(t *testing.T) {
	// 合法：普通相对路径、以及管理员配置的绝对路径（例如 nginx 自己的证书目录）
	ok := []string{"certs/example.com/cert.pem", "/etc/nginx/certs/cert.pem", "/opt/agent/data/certs/x.pem"}
	for _, p := range ok {
		if err := RejectTraversal("cert path", p); err != nil {
			t.Errorf("RejectTraversal(%q) 不应报错（合法用法不能被限制）：%v", p, err)
		}
	}
	bad := []string{"", "../x", "certs/../../etc/shadow", "/etc/../etc/shadow", "a/b/..", "x\x00y"}
	for _, p := range bad {
		if err := RejectTraversal("cert path", p); err == nil {
			t.Errorf("RejectTraversal(%q) 必须报错", p)
		}
	}
}

func TestResolveUnder(t *testing.T) {
	root := "/opt/agent/data"

	got, err := ResolveUnder(root, "certs/a.pem")
	if err != nil || got != filepath.Join(root, "certs", "a.pem") {
		t.Errorf("相对路径应解析到 root 之下，得到 %q err=%v", got, err)
	}

	got, err = ResolveUnder(root, filepath.Join(root, "certs", "b.pem"))
	if err != nil || got != filepath.Join(root, "certs", "b.pem") {
		t.Errorf("root 内的绝对路径应通过，得到 %q err=%v", got, err)
	}

	// 逃逸必须被拒：相对上跳、绝对路径跑到 root 之外
	for _, p := range []string{"../../etc/passwd", "/etc/passwd", "certs/../../../etc/passwd"} {
		if _, err := ResolveUnder(root, p); err == nil {
			t.Errorf("ResolveUnder(%q) 必须拒绝（会逃出 %s）", p, root)
		}
	}
}
