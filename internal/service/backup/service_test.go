package backup

import (
	"strings"
	"testing"
	"time"

	"corefusion/internal/security/crypto"
)

// 存储密钥加密落库 → 读取解密 → 对外掩码 的完整往返行为锁。
func TestStorageSecretRoundTrip(t *testing.T) {
	orig := `{"endpoint":"http://minio:9000","bucket":"bk","access_key":"AKIA","secret_key":"super-secret"}`
	enc := encryptStorageConfig(orig)
	if !strings.Contains(enc, "enc:") {
		t.Fatalf("secret_key 未加密落库: %s", enc)
	}
	if strings.Contains(enc, "super-secret") {
		t.Fatalf("加密后仍含明文: %s", enc)
	}
	// 幂等：二次加密不叠加
	if again := encryptStorageConfig(enc); again != enc {
		t.Fatalf("重复加密破坏幂等:\n got: %s\nwant: %s", again, enc)
	}
	// 执行路径解密
	var cfg S3Config
	if err := ParseStorageConfig(enc, &cfg); err != nil {
		t.Fatalf("ParseStorageConfig: %v", err)
	}
	if cfg.SecretKey != "super-secret" {
		t.Fatalf("解密后应得到明文，got %q", cfg.SecretKey)
	}
}

func TestStoragePasswordDecrypt(t *testing.T) {
	enc := encryptStorageConfig(`{"host":"ftp.example.com","username":"u","password":"p@ss"}`)
	var cfg FTPConfig
	if err := ParseStorageConfig(enc, &cfg); err != nil {
		t.Fatalf("ParseStorageConfig: %v", err)
	}
	if cfg.Password != "p@ss" {
		t.Fatalf("password 解密失败，got %q", cfg.Password)
	}
	// 存量明文直接解析（兼容）
	var legacy FTPConfig
	if err := ParseStorageConfig(`{"password":"old-plain"}`, &legacy); err != nil {
		t.Fatalf("legacy parse: %v", err)
	}
	if legacy.Password != "old-plain" {
		t.Fatalf("存量明文应原样可用，got %q", legacy.Password)
	}
}

// UpdateStorage 的 ******** 哨兵逻辑单测（不经 DB，验证 encrypt 前的合并行为由 handler 层保证）。
func TestCryptoRoundTripBasic(t *testing.T) {
	enc, err := crypto.EncryptSecret("token-123")
	if err != nil {
		t.Fatalf("EncryptSecret: %v", err)
	}
	if !crypto.IsEncrypted(enc) {
		t.Fatalf("密文应带 enc: 前缀")
	}
	plain, err := crypto.DecryptSecret(enc)
	if err != nil || plain != "token-123" {
		t.Fatalf("往返失败: %q, %v", plain, err)
	}
}

// 还原成功后进程会 exec 重启：状态必须落盘，重启后仍能读到 done，
// 否则客户端轮询只会看到 idle，误以为还原超时/没执行。
func TestRestoreStatusPersistedAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	s := &Service{dataDir: dir}
	s.restoreMu.Lock()
	s.nextRestoreID = 3
	s.restoreStatus = &RestoreStatus{ID: 3, Status: "done", Message: "restore completed successfully"}
	s.persistRestoreStatusLocked()
	s.restoreMu.Unlock()

	loaded := (&Service{dataDir: dir}).loadRestoreStatusFile()
	if loaded == nil {
		t.Fatal("重启后未读到还原状态")
	}
	if loaded.ID != 3 || loaded.Status != "done" {
		t.Fatalf("loaded = %+v, want id=3 done", loaded)
	}
}

// 还原进行中被重启：不能永远停在 restoring，应按中断标记 error。
func TestRestoreStatusInterruptedMarkedError(t *testing.T) {
	dir := t.TempDir()
	s := &Service{dataDir: dir}
	s.restoreMu.Lock()
	s.restoreStatus = &RestoreStatus{ID: 4, Status: "restoring", Message: "restoring"}
	s.persistRestoreStatusLocked()
	s.restoreMu.Unlock()

	loaded := (&Service{dataDir: dir}).loadRestoreStatusFile()
	if loaded == nil || loaded.Status != "error" {
		t.Fatalf("中断状态应为 error，实际 %+v", loaded)
	}
}

// 定时任务必须按 cron 表达式判定，而不是旧的 24h 固定间隔。
func TestScheduleDueCron(t *testing.T) {
	loc := time.Local
	last3am := time.Date(2026, 11, 23, 3, 0, 0, 0, loc)
	cases := []struct {
		name    string
		cron    string
		last    time.Time
		created time.Time
		now     time.Time
		want    bool
	}{
		{"daily 未到点", "0 3 * * *", last3am, time.Time{}, last3am.Add(7 * time.Hour), false},
		{"daily 到点", "0 3 * * *", last3am, time.Time{}, last3am.Add(24 * time.Hour), true},
		{"daily 新建不立即跑", "0 3 * * *", time.Time{}, last3am.Add(10 * time.Hour), last3am.Add(10 * time.Hour), false},
		{"每分钟到点", "* * * * *", last3am, time.Time{}, last3am.Add(61 * time.Second), true},
		{"每分钟未到点", "* * * * *", last3am, time.Time{}, last3am.Add(30 * time.Second), false},
		{"非法cron首次执行", "bad cron", time.Time{}, time.Time{}, last3am, true},
		{"非法cron 24h内不重复", "bad cron", last3am, time.Time{}, last3am.Add(time.Hour), false},
		{"非法cron 超24h", "bad cron", last3am, time.Time{}, last3am.Add(25 * time.Hour), true},
	}
	for _, c := range cases {
		if got := scheduleDue(c.cron, c.last, c.created, c.now); got != c.want {
			t.Errorf("%s: scheduleDue(%q,%v,%v,%v)=%v want %v", c.name, c.cron, c.last, c.created, c.now, got, c.want)
		}
	}
}
