package binaries

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// CoreValidator 用「真实核心二进制」校验配置文件内容。
//
// 为什么需要它：面板是按分段（core_configs）拼装配置的，一旦写入了该版本核心
// 已移除/未知的字段，只有真实二进制才能发现——sing-box 是严格解码（未知字段直接
// FATAL 退出，核心起不来），而纯 JSON 语法检查完全看不出来。典型例子：
// dns.servers[].address（1.12 移除）、顶层 dns.fakeip/hosts（1.14 移除）、
// route 规则上的 strategy/domain_resolver、port 用字符串数组等。
//
// 注意各核心的严格程度不同：
//   - sing-box：严格，未知字段 FATAL（最有价值）
//   - xray：宽松，未知字段只告警不报错，能挡住语法/语义错误
type CoreValidator struct {
	resolver *Resolver
	timeout  time.Duration
}

// NewValidator 创建验证器。resolver 用于定位各核心的二进制。
func NewValidator(resolver *Resolver) *CoreValidator {
	return &CoreValidator{resolver: resolver, timeout: 30 * time.Second}
}

// checkArgs 返回各核心的「只检查不启动」命令参数；返回 nil 表示该核心不支持校验。
func checkArgs(coreType, configPath string) []string {
	switch coreType {
	case "singbox":
		return []string{"check", "-c", configPath}
	case "xray":
		return []string{"-test", "-config", configPath}
	default:
		return nil
	}
}

// ValidateConfig 用核心二进制校验配置。
//
// 返回值语义（供 coreconfig.Service 透传到 API）：
//   - checked=false：未能/无需校验（核心未安装、或该核心不支持检查命令），err 为 nil
//   - checked=true 且 err == nil：校验通过
//   - err != nil：校验失败，output 为核心输出的关键行
func (v *CoreValidator) ValidateConfig(coreType string, configJSON []byte) (checked bool, version string, output string, err error) {
	var args []string
	if args = checkArgs(coreType, ""); args == nil {
		return false, "", "核心 " + coreType + " 不支持配置检查命令，已跳过", nil
	}

	binaryPath, ver, resolveErr := v.resolver.Resolve(coreType)
	if resolveErr != nil {
		return false, "", "核心未安装，无法校验（" + resolveErr.Error() + "）", nil
	}

	// 配置文件写到核心自己的运行目录下：配置里的相对路径（如 certs/xxx.pem）
	// 依赖核心进程的工作目录，用该目录做 CWD 才能得到与真实启动一致的结论。
	dir := filepath.Join(v.resolver.dataDir, "tmp")
	if mkErr := os.MkdirAll(dir, 0o750); mkErr != nil {
		dir = os.TempDir()
	}
	f, createErr := os.CreateTemp(dir, "validate-*.json")
	if createErr != nil {
		return false, ver, "无法创建临时配置文件：" + createErr.Error(), nil
	}
	tmpPath := f.Name()
	defer os.Remove(tmpPath)
	if _, writeErr := f.Write(configJSON); writeErr != nil {
		f.Close()
		return false, ver, "无法写入临时配置文件：" + writeErr.Error(), nil
	}
	f.Close()

	ctx, cancel := context.WithTimeout(context.Background(), v.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath, checkArgs(coreType, tmpPath)...)
	cmd.Dir = v.resolver.dataDir
	cmd.Env = os.Environ()
	// xray 通过 XRAY_LOCATION_ASSET 查找 geoip.dat/geosite.dat，
	// 与 internal/runtime/process 启动核心时保持一致，否则会因找不到 geo 文件误判。
	if coreType == "xray" {
		if dir := v.xrayAssetDir(); dir != "" {
			cmd.Env = append(cmd.Env, "XRAY_LOCATION_ASSET="+dir)
		}
	}
	raw, runErr := cmd.CombinedOutput()
	out := sanitizeCoreOutput(string(raw), tmpPath)

	if ctx.Err() == context.DeadlineExceeded {
		return true, ver, out, fmt.Errorf("校验超时（%s）", v.timeout)
	}
	if runErr != nil {
		if out == "" {
			out = runErr.Error()
		}
		// 校验是在面板主机跑的，节点上的证书/规则文件在面板本机不一定存在，
		// 这类「缺少文件」不是配置字段问题，明确提示避免误判。
		hint := ""
		if missingFilePattern.MatchString(out) {
			hint = "（报错为缺少文件/证书：校验在面板主机执行，节点本地的文件在面板上可能不存在，不一定是配置问题）"
		}
		return true, ver, out, fmt.Errorf("%s 配置校验失败%s", coreType, hint)
	}
	return true, ver, out, nil
}

// missingFilePattern 匹配「打开文件失败」类错误（用于给出面板侧校验的提示）。
var missingFilePattern = regexp.MustCompile(`(no such file or directory|permission denied|cannot find the file|file does not exist)`)

// xrayAssetDir 返回 xray 的 geo 资源目录（geoip.dat/geosite.dat 所在）。
// 先看运行环境已有的 XRAY_LOCATION_ASSET，再依次探测面板的几个常见存放位置。
func (v *CoreValidator) xrayAssetDir() string {
	if p := os.Getenv("XRAY_LOCATION_ASSET"); p != "" {
		return p
	}
	for _, dir := range []string{
		filepath.Join(v.resolver.dataDir, "rules"),
		filepath.Join(v.resolver.dataDir, "binaries", "active", "corefusion"),
		filepath.Join(v.resolver.dataDir, "binaries", "staging"),
	} {
		if _, err := os.Stat(filepath.Join(dir, "geoip.dat")); err == nil {
			return dir
		}
	}
	return ""
}

// sanitizeCoreOutput 去掉 ANSI 颜色、临时文件路径噪音，并只保留关键行。
func sanitizeCoreOutput(raw, tmpPath string) string {
	raw = strings.ReplaceAll(raw, tmpPath, "<config>")
	var kept []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimRight(stripANSI(line), " \t")
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		if strings.Contains(lower, "fatal") || strings.Contains(lower, "error") || strings.Contains(lower, "unknown field") {
			kept = append(kept, line)
		}
	}
	if len(kept) == 0 {
		// 没有明确的错误行时，保留末尾几行作为上下文
		all := strings.Split(strings.TrimSpace(raw), "\n")
		for i := len(all) - 1; i >= 0 && len(kept) < 3; i-- {
			if l := strings.TrimSpace(stripANSI(all[i])); l != "" {
				kept = append([]string{l}, kept...)
			}
		}
	}
	if len(kept) > 8 {
		kept = kept[:8]
	}
	joined := strings.Join(kept, "\n")
	if len(joined) > 1500 {
		joined = joined[:1500] + "…"
	}
	return joined
}

func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b {
			// 跳过 ESC [ ... m 序列
			if i+1 < len(s) && s[i+1] == '[' {
				j := i + 2
				for j < len(s) && s[j] != 'm' {
					j++
				}
				i = j
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
