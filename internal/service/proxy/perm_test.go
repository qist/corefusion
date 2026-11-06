package proxy

import (
	"os"
	"path/filepath"
	"testing"
)

// 数据目录 0750 + lua 0600 时，EnsureRuntimePermissions 必须自动放开到
// nginx worker（非 root）可读，否则 WAF 会静默失效。这类问题不能靠手工 chmod。
func TestEnsureRuntimePermissionsFixesWAFReadability(t *testing.T) {
	dataDir := t.TempDir()
	// 模拟面板以 root 创建的 0750 数据目录
	if err := os.Chmod(dataDir, 0o750); err != nil {
		t.Fatal(err)
	}

	luaDir := filepath.Join(dataDir, "proxy", "lua", "waf")
	if err := os.MkdirAll(luaDir, 0o700); err != nil {
		t.Fatal(err)
	}
	luaFile := filepath.Join(luaDir, "access.lua")
	if err := os.WriteFile(luaFile, []byte("-- waf"), 0o600); err != nil {
		t.Fatal(err)
	}
	rulesDir := filepath.Join(dataDir, "proxy", "waf-rules")
	if err := os.MkdirAll(rulesDir, 0o700); err != nil {
		t.Fatal(err)
	}
	ruleFile := filepath.Join(rulesDir, "whiteip.rule")
	if err := os.WriteFile(ruleFile, []byte("1.2.3.4\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	logsDir := filepath.Join(dataDir, "logs")
	if err := os.MkdirAll(logsDir, 0o700); err != nil {
		t.Fatal(err)
	}

	if err := EnsureRuntimePermissions(dataDir); err != nil {
		t.Fatalf("EnsureRuntimePermissions: %v", err)
	}

	// 数据根目录：非 root 用户要能进入（o+x）
	if mode := statPerm(t, dataDir); mode&0o001 == 0 {
		t.Errorf("data dir mode = %o, want others execute", mode)
	}
	// WAF 脚本与规则：非 root 用户要能读（o+r）
	if mode := statPerm(t, luaFile); mode&0o004 == 0 {
		t.Errorf("access.lua mode = %o, want others read", mode)
	}
	if mode := statPerm(t, ruleFile); mode&0o004 == 0 {
		t.Errorf("whiteip.rule mode = %o, want others read", mode)
	}
	// 日志目录：非 root 用户要能进入并写（组/其他可写由 chown+0775 保证）
	if mode := statPerm(t, logsDir); mode&0o002 == 0 && mode&0o020 == 0 {
		t.Errorf("logs dir mode = %o, want writable by nginx group or others", mode)
	}
	// conf.d 目录要自动创建，方便迁移配置持久化
	if _, err := os.Stat(filepath.Join(dataDir, "proxy", "conf.d")); err != nil {
		t.Errorf("conf.d should be created: %v", err)
	}
}

func statPerm(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return info.Mode().Perm()
}
