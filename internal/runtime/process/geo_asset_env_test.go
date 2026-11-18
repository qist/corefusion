package process

import (
	"os"
	"path/filepath"
	"testing"
)

// xray 的 ext:xxx.dat 统一靠 XRAY_LOCATION_ASSET 解析（不再建软链）：
// 必须写入**绝对路径**，否则核心子进程 cwd 不同会解析错。
func TestEnsureXrayAssetEnvSetsAbsolutePath(t *testing.T) {
	t.Setenv("XRAY_LOCATION_ASSET", "")
	base := t.TempDir()
	t.Chdir(base)

	EnsureXrayAssetEnv("data/rules")

	got := os.Getenv("XRAY_LOCATION_ASSET")
	want := filepath.Join(base, "data", "rules")
	if filepath.Clean(got) != filepath.Clean(want) {
		t.Fatalf("XRAY_LOCATION_ASSET = %q, want %q", got, want)
	}
	if fi, err := os.Stat(got); err != nil || !fi.IsDir() {
		t.Fatalf("资产目录未创建: %v", err)
	}
}

// 运维显式配置了资产目录时不能覆盖。
func TestEnsureXrayAssetEnvKeepsExisting(t *testing.T) {
	custom := t.TempDir()
	t.Setenv("XRAY_LOCATION_ASSET", custom)

	EnsureXrayAssetEnv(t.TempDir())

	if got := os.Getenv("XRAY_LOCATION_ASSET"); got != custom {
		t.Fatalf("不应覆盖已配置的资产目录: %q", got)
	}
}

// 升级后要清掉旧版本在可执行目录/cwd 里留下的规则软链（只删指向 rules 的）。
func TestRemoveRuleAssetLinksCleansLegacyLinks(t *testing.T) {
	rulesDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(rulesDir, "geosite_IR.dat"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()
	t.Chdir(work)

	legacy := filepath.Join(work, "geosite_IR.dat")
	if err := os.Symlink(filepath.Join(rulesDir, "geosite_IR.dat"), legacy); err != nil {
		t.Fatal(err)
	}
	// 真实文件不能删
	realFile := filepath.Join(work, "geoip.dat")
	if err := os.WriteFile(realFile, []byte("real"), 0o644); err != nil {
		t.Fatal(err)
	}
	// 指向别处的软链不能删
	other := filepath.Join(work, "other.dat")
	if err := os.Symlink("/etc/hostname", other); err != nil {
		t.Fatal(err)
	}

	RemoveRuleAssetLinks(rulesDir)

	if _, err := os.Lstat(legacy); !os.IsNotExist(err) {
		t.Errorf("旧规则软链应被清理")
	}
	if fi, err := os.Lstat(realFile); err != nil || fi.Mode()&os.ModeSymlink != 0 {
		t.Errorf("真实文件不应被动")
	}
	if _, err := os.Lstat(other); err != nil {
		t.Errorf("无关软链不应被删: %v", err)
	}
}
