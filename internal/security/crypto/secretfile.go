package crypto

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SecretEnvFile 是**只放加密密钥**的独立隐藏文件（0600，与 .env 同目录）。
//
// 与 .env 分开的原因：.env 里有 SAFE_PATHS / CF_DB_DSN / JWT_SECRET 等日常要查看、
// 要改的配置，而 CF_SECRET_ENCRYPTION_KEY 一旦泄露就能解密库里所有 enc: 密文
// （DNS 通道凭据、支付密钥、2FA secret、CA 私钥）。分开后：
//   - 权限可以更严（只读、单独轮换）；
//   - 排查问题时 cat .env / 截图 / 贴日志不会带出密钥；
//   - 备份/打包规则可以只排除这一个文件。
//
// ⚠ 这只降低"误暴露面"，不改变威胁模型：拿到机器 root 的人仍能读它
// （也能读 /proc/<pid>/environ），本地文件方案防不了"已进机器"。
const SecretEnvFile = ".env.secret"

// SecretEncryptionKeyName 是加密主密钥的键名：只应出现在 SecretEnvFile，
// 不应写进 .env（否则会随 .env 被抄送/贴日志，且首次启动还会触发一次"搬家"）。
const SecretEncryptionKeyName = "CF_SECRET_ENCRYPTION_KEY"

// SecretEnvKeys 是允许存放在 SecretEnvFile 的键（目前只有加密密钥）。
var SecretEnvKeys = map[string]bool{
	SecretEncryptionKeyName: true,
}

// ParseEnvContent 解析 env 文本为 map（忽略空行与 # 注释）。
func ParseEnvContent(content string) map[string]string {
	out := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			out[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return out
}

// ReadEnvFileValue 从指定 env 文件读取单个键（文件不存在/无该键返回空串）。
func ReadEnvFileValue(path, key string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return ParseEnvContent(string(data))[key]
}

// ReadSecretEnvValue 从 SecretEnvFile 读取单个键。
func ReadSecretEnvValue(key string) string {
	return ReadEnvFileValue(SecretEnvFile, key)
}

// SaveSecretEnvValue 写入/更新 SecretEnvFile 中的单个键
// （0600，先写临时文件再 rename，避免半截文件）。
func SaveSecretEnvValue(key, value string) error {
	return SaveEnvFileValue(SecretEnvFile, key, value)
}

// SaveEnvFileValue 写入/更新指定 env 文件中的单个键
// （0600，先写临时文件再 rename，避免半截文件）。
func SaveEnvFileValue(path, key, value string) error {
	existing := ""
	if data, err := os.ReadFile(path); err == nil {
		existing = string(data)
	}
	lines := []string{}
	found := false
	for _, line := range strings.Split(existing, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) == 2 && strings.TrimSpace(parts[0]) == key {
				lines = append(lines, key+"="+value)
				found = true
				continue
			}
		}
		if line != "" || len(lines) > 0 {
			lines = append(lines, line)
		}
	}
	if !found {
		lines = append(lines, key+"="+value)
	}
	content := strings.Join(lines, "\n")
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0600); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0600); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// EnsureSecretKeyAt 确保 <dir>/.env.secret 里存在指定密钥并返回它。
//
// 用于**没有 .env 机制的进程**（节点上的 Agent）：首次启动生成 32 字节随机密钥
// 写入 0600 文件，之后一直复用；已有密钥时只读不写（幂等）。
//
// 密钥文件放在 <dir>（Agent 的 $INSTALL_DIR）而不是 data 目录里 ——
// 这样备份/打包 data 目录不会把密钥一起带走。
//
// ⚠ 它是**随机密钥**，不是从 hostname 派生。历史密文（hostname 派生加密）
// 仍由 DecryptSecret 的 legacy 回退解密，下次写入时自动迁移到新密钥。
func EnsureSecretKeyAt(dir, key string) (string, error) {
	if v := os.Getenv(key); v != "" {
		return v, nil
	}
	path := filepath.Join(dir, SecretEnvFile)
	if v := ReadEnvFileValue(path, key); v != "" {
		return v, nil
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	value := hex.EncodeToString(buf)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	if err := SaveEnvFileValue(path, key, value); err != nil {
		return "", err
	}
	return value, nil
}

// RemoveEnvValue 从指定 env 文件中删除某个键（保持其它行原样，0600）。
func RemoveEnvValue(path, key string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := []string{}
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) == 2 && strings.TrimSpace(parts[0]) == key {
				continue
			}
		}
		lines = append(lines, line)
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0600); err != nil {
		return err
	}
	return os.Chmod(path, 0600)
}

// redactEnvKey 把 env 文本里指定键的值替换成占位符（写备份用）：
// 避免把密钥再复制到 .env.bak-* 这类容易被拷贝/发送的文件里。
func redactEnvKey(content, key string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) == 2 && strings.TrimSpace(parts[0]) == key {
			lines[i] = key + "=#redacted (moved to " + SecretEnvFile + ")"
		}
	}
	return strings.Join(lines, "\n")
}

// RelocateKeyToSecretFile 把老部署 `.env` 里的某个密钥键搬到 SecretEnvFile
// （**值不变**，所以现有密文仍能解密；幂等）。
//
// 顺序刻意保守：先写新文件 → 读回校验 → 备份 .env → 才从 .env 删除。
// 任一步失败就保持原样（最坏情况是密钥同时在两处，绝不会丢密钥）。
func RelocateKeyToSecretFile(key string) {
	data, err := os.ReadFile(".env")
	if err != nil {
		return
	}
	env := ParseEnvContent(string(data))
	value := env[key]
	if value == "" {
		return // 新部署：密钥本来就不在 .env
	}
	if existing := ReadSecretEnvValue(key); existing != "" {
		if existing == value {
			if err := RemoveEnvValue(".env", key); err != nil {
				log.Printf("[startup] warning: key already in %s but removing duplicate from .env failed: %v", SecretEnvFile, err)
				return
			}
			log.Printf("[startup] removed duplicate %s from .env (now only in %s)", key, SecretEnvFile)
		} else {
			// 两边值不同：以 .env 为准（历史行为），不动任何一边，只告警。
			log.Printf("[startup] warning: %s differs between .env and %s; keeping .env value, please reconcile manually", key, SecretEnvFile)
		}
		return
	}
	if err := SaveSecretEnvValue(key, value); err != nil {
		log.Printf("[startup] warning: move %s to %s failed (keeping it in .env): %v", key, SecretEnvFile, err)
		return
	}
	if ReadSecretEnvValue(key) != value {
		log.Printf("[startup] warning: %s verification failed; keeping key in .env", SecretEnvFile)
		return
	}
	// 备份 .env 后再删（出问题可手工恢复）。
	// 备份里**不回写密钥**：此时密钥已在 .env.secret 中，回滚只需恢复 .env，
	// 没必要再多一份含密钥的文件（.env 经常被拷贝/贴出来排查）。
	bak := ".env.bak-" + time.Now().Format("20060102-150405")
	if err := os.WriteFile(bak, []byte(redactEnvKey(string(data), key)), 0600); err != nil {
		log.Printf("[startup] warning: backup .env failed, keeping key in .env: %v", err)
		return
	}
	if err := RemoveEnvValue(".env", key); err != nil {
		log.Printf("[startup] warning: remove %s from .env failed (key exists in both files, harmless): %v", key, err)
		return
	}
	log.Printf("[startup] moved %s to %s (backup: %s)", key, SecretEnvFile, bak)
}

// ReencryptEnvFile 把 env 文件里「当前主密钥解不开、但 legacy 主机名派生密钥能解」
// 的 `enc:` 值，解出来后用当前主密钥重写（原子写，0600，幂等）。返回重写数量。
//
// 典型用途：`.env` 里的 `CF_DB_DSN`（mysql/pg 的 DSN 是加密存储的）。老部署若
// 曾在 hostname 派生密钥下写入，升级后只靠 DecryptSecret 的 legacy 回退能读；
// 这里把它真正迁到新密钥，避免换机/回退逻辑变化后永久解不开。
func ReencryptEnvFile(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	lines := strings.Split(string(data), "\n")
	n := 0
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		if !IsEncrypted(val) || CanDecryptWithPrimary(val) {
			continue
		}
		plain, derr := DecryptSecret(val)
		if derr != nil {
			continue
		}
		enc, eerr := EncryptSecret(plain)
		if eerr != nil || enc == "" {
			continue
		}
		lines[i] = key + "=" + enc
		n++
	}
	if n == 0 {
		return 0, nil
	}
	content := strings.Join(lines, "\n")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0600); err != nil {
		return 0, err
	}
	if err := os.Chmod(tmp, 0600); err != nil {
		os.Remove(tmp)
		return 0, err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return 0, err
	}
	return n, nil
}
