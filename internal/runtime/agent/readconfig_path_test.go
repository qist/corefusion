package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 回归安全审计 v5 H3：`ReadConfigFile` 会把 coreType 直接拼进路径 ✗ ——
// Agent 以 root 运行，`waf_rule:../../../<secret>` 之类能把数据目录之外的文件读回面板。
//
// ⚠ 判定方式很关键：**在 dataDir 的兄弟位置放一个"机密文件"**，越界读取一旦生效就会读到它。
// 只用 `/etc/passwd` 这类绝对路径不可靠 —— 临时目录上跳 N 层后落点未必存在，
// `ReadFile` 无论如何都返回 err → 测试会**永远通过**（我第一版就是这样，去掉修复后仍然 PASS ✗）。
func TestReadConfigFileRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	if err := os.MkdirAll(dataDir, 0750); err != nil {
		t.Fatal(err)
	}
	const secret = "TOP-SECRET-CONTENT"
	if err := os.WriteFile(filepath.Join(root, "secret.txt"), []byte(secret), 0640); err != nil {
		t.Fatal(err)
	}

	a := &LocalAgent{dataDir: dataDir}

	attacks := []string{
		"waf_rule:../../../secret.txt",       // 逃出 data/proxy/waf-rules
		"caddy_waf_rule:../../../secret.txt", // 同上（caddy 分支）
		"waf_rule:foo/../../x",               // 含分隔符
		"../../../secret.txt",                // 默认分支（runtime/<coreType>.jsonc）
	}
	for _, ct := range attacks {
		data, _, err := a.ReadConfigFile(ct)
		if err == nil {
			t.Errorf("ReadConfigFile(%q) 必须被拒（否则以 root 读任意文件）", ct)
		}
		if strings.Contains(string(data), secret) {
			t.Errorf("ReadConfigFile(%q) 读到了数据目录之外的文件内容 ✗", ct)
		}
	}
}

// 合法读取不能被影响。
func TestReadConfigFileStillWorksForLegitCoreType(t *testing.T) {
	a := &LocalAgent{dataDir: t.TempDir()}
	dir := filepath.Join(a.dataDir, "runtime")
	if err := os.MkdirAll(dir, 0750); err != nil {
		t.Fatal(err)
	}
	want := `{"ok":true}`
	if err := os.WriteFile(filepath.Join(dir, "xray.jsonc"), []byte(want), 0640); err != nil {
		t.Fatal(err)
	}
	data, p, err := a.ReadConfigFile("xray")
	if err != nil {
		t.Fatalf("合法 coreType 不应报错：%v", err)
	}
	if string(data) != want {
		t.Errorf("内容不对：%s", data)
	}
	if p != filepath.Join(dir, "xray.jsonc") {
		t.Errorf("路径不对：%s", p)
	}
}

// 合法 WAF 规则名（单纯文件名）仍要能读 —— 证明校验没有过度收紧。
func TestReadConfigFileStillWorksForLegitWafRule(t *testing.T) {
	a := &LocalAgent{dataDir: t.TempDir()}
	wafDir := filepath.Join(a.dataDir, "proxy", "waf-rules")
	if err := os.MkdirAll(wafDir, 0750); err != nil {
		t.Fatal(err)
	}
	want := `{"rule":"url.rule"}`
	if err := os.WriteFile(filepath.Join(wafDir, "url.rule"), []byte(want), 0640); err != nil {
		t.Fatal(err)
	}
	data, p, err := a.ReadConfigFile("waf_rule:url.rule")
	if err != nil {
		t.Fatalf("合法规则名不应报错：%v", err)
	}
	if string(data) != want || filepath.Base(p) != "url.rule" {
		t.Errorf("读取结果不对：%s %s", data, p)
	}
}
