package main

import (
	"os"
	"strings"
	"testing"
)

// 片段内容：必须是"只接管挑战路径"的独立 server 块 ✓，alias 对齐写入路径 ✓
func TestNginxSnippetContent(t *testing.T) {
	s := nginxSnippet("/var/lib/corefusion-agent/acme-challenge", []string{"test.example.com", "203.0.113.48"})
	for _, want := range []string{
		"listen 80;",
		"server_name test.example.com 203.0.113.48;",
		"location ^~ /.well-known/acme-challenge/ {",
		"alias /var/lib/corefusion-agent/acme-challenge/.well-known/acme-challenge/;",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("片段缺少 %q：\n%s", want, s)
		}
	}
	if !strings.Contains(s, "自动删除") {
		t.Fatalf("片段应标注是临时文件：\n%s", s)
	}
}

// 显式 webroot → 直接用，不做任何自动改动 ✓
func TestPrepareHTTP01ExplicitWebroot(t *testing.T) {
	root, cleanup, err := prepareHTTP01([]string{"a.com"}, t.TempDir(), "/apps/nginx/html")
	if err != nil {
		t.Fatal(err)
	}
	if root != "/apps/nginx/html" {
		t.Fatalf("应原样返回显式 webroot，实际 %q", root)
	}
	if cleanup != nil {
		cleanup()
	}
}

// 80 空闲 → 返回空 root（= 用内置临时 80 服务 ✓）
func TestPrepareHTTP01TempServerWhenPortFree(t *testing.T) {
	if !portIsFree(0) {
		t.Fatal("portIsFree(0) 应视为空闲（绑定到随机端口）")
	}
	// 80 的状态是**环境相关**的 ✓：空闲 → 走内置临时服务（root="") ✓；
	// 被占用 → 走自动配置（返回 root 或明确报错 ✓）——两种都算符合设计 ✓。
	root, cleanup, err := prepareHTTP01([]string{"a.com"}, t.TempDir(), "")
	if portIsFree(80) {
		if err != nil {
			t.Fatal(err)
		}
		if root != "" {
			t.Fatalf("80 空闲时应使用内置临时服务，实际给了 root=%q", root)
		}
	} else {
		// 占用时：要么给出可写目录 ✓，要么给出**明确**的错误（不静默 ✗）
		if err == nil && root == "" {
			t.Fatal("80 被占用时必须给出 webroot 或明确报错")
		}
		if err != nil && !strings.Contains(err.Error(), "port 80 is in use") {
			t.Fatalf("错误信息应说明 80 被占用：%v", err)
		}
	}
	cleanup()
}

// 挑战目录**绝不能**落在 /tmp ✗：nginx 服务单元常带 PrivateTmp=yes ✗ → 看不到 → 恒 404 ✗（真机踩过 ✓）。
func TestACMEChallengeDirNotUnderTmp(t *testing.T) {
	d := acmeChallengeDir("/tmp/some-data-dir")
	if strings.HasPrefix(d, "/tmp") {
		t.Fatalf("挑战目录不能放在 /tmp（PrivateTmp 会导致 404）：%s", d)
	}
	if !strings.HasPrefix(d, "/var/lib/") {
		t.Fatalf("默认应放在 web 服务器可见的 /var/lib 下：%s", d)
	}
	// 显式覆盖优先 ✓
	t.Setenv("COREFUSION_ACME_CHALLENGE_DIR", "/var/www/.acme-x")
	if got := acmeChallengeDir("/tmp/whatever"); got != "/var/www/.acme-x" {
		t.Fatalf("环境变量覆盖失效：%s", got)
	}
	_ = os.MkdirAll(d, 0755)
	_ = os.RemoveAll(d)
}
