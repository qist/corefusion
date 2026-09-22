package agent

import (
	"os"
	"path/filepath"
	"testing"
)

// 回归安全审计 v5 H3：`ReadConfigFile` 会把 coreType 直接拼进路径 ✗ ——
// Agent 以 root 运行，`waf_rule:../../../etc/shadow` 之类能把任意文件读回面板。
func TestReadConfigFileRejectsTraversal(t *testing.T) {
	a := &LocalAgent{dataDir: t.TempDir()}

	attacks := []string{
		"waf_rule:../../../../etc/passwd",
		"caddy_waf_rule:../../../../etc/shadow",
		"waf_rule:foo/../../bar",
		"../../../../etc/passwd", // 默认分支（runtime/<coreType>.jsonc）
		"config/../../x",
	}
	for _, ct := range attacks {
		if _, _, err := a.ReadConfigFile(ct); err == nil {
			t.Errorf("ReadConfigFile(%q) 必须被拒（否则以 root 读任意文件）", ct)
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
