package cert

import (
	"path"
	"path/filepath"
	"strings"
)

// CertPaths 是"一张证书落在某个目录下"的三个文件相对路径（相对 dataDir ✓）。
type CertPaths struct {
	Dir       string
	Cert      string
	Key       string
	Fullchain string
}

func certPathsForDir(dir string) CertPaths {
	return CertPaths{
		Dir:       dir,
		Cert:      path.Join(dir, "cert.pem"),
		Key:       path.Join(dir, "key.pem"),
		Fullchain: path.Join(dir, "fullchain.pem"),
	}
}

// CertDirAliases 返回一张证书在磁盘上应当写入的**所有**相对目录（去重、顺序稳定）。
//
// 规范目录由域名决定：certs/<domain>[/<keytype>]。
// 但历史/迁移前创建的证书，站点配置里引用的是"名称"目录（例如 name=gg、
// domain=example.com 时配置写 certs/gg/cert.pem）。只按域名分发会出现
// "证书签出来了、节点上也写了 certs/example.com/，但站点一直读 certs/gg/ 的旧证书"。
// 因此分发 / 落地 / 同步匹配都必须同时认这两个目录。
func CertDirAliases(name, domain, keyType string) []string {
	dirs := make([]string, 0, 2)
	seen := make(map[string]bool, 2)
	add := func(n string) {
		n = sanitizeCertDirName(n)
		if n == "" {
			return
		}
		rel, _, _ := CertRelPaths(n, keyType)
		dir := filepath.Dir(rel)
		if dir == "" || dir == "." || seen[dir] {
			return
		}
		seen[dir] = true
		dirs = append(dirs, dir)
	}
	add(domain)
	// 名称与域名不同时，名称目录是站点历史引用（如 certs/gg），必须一起写。
	if !strings.EqualFold(strings.TrimSpace(name), strings.TrimSpace(domain)) {
		add(name)
	}
	return dirs
}

// CertPathVariants 返回一张证书可能被引用/需要落地的**全部**路径形态（去重、顺序稳定）：
//
//  1. 域名目录（规范路径，certs/<domain>[/<keytype>]）；
//  2. 历史名称目录（name != domain 时，站点可能仍写 certs/<name>/）；
//  3. 库记录里当前路径所在目录（更早版本按名称/消毒名落盘的位置）。
//
// 分发、面板本机落地、SyncData 匹配都必须用同一份列表，否则会出现
// "签出来了、某一处写了新证书，但站点引用的另一个目录一直是旧证书"。
func CertPathVariants(name, domain, keyType, storedCertPath string) []CertPaths {
	dirs := CertDirAliases(name, domain, keyType)
	seen := make(map[string]bool, len(dirs)+1)
	for _, d := range dirs {
		seen[d] = true
	}
	if storedDir := sanitizeCertRelDir(path.Dir(filepath.ToSlash(strings.TrimSpace(storedCertPath)))); storedDir != "" && !seen[storedDir] {
		dirs = append(dirs, storedDir)
	}
	out := make([]CertPaths, 0, len(dirs))
	for _, d := range dirs {
		out = append(out, certPathsForDir(d))
	}
	return out
}

// sanitizeCertRelDir 校验并归一化一个**相对**证书目录（拒绝绝对路径与 ".."）。
func sanitizeCertRelDir(dir string) string {
	dir = strings.TrimSpace(filepath.ToSlash(dir))
	dir = strings.TrimPrefix(dir, "./")
	dir = strings.TrimSuffix(dir, "/")
	if dir == "" || dir == "." || strings.HasPrefix(dir, "/") {
		return ""
	}
	for _, seg := range strings.Split(dir, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return ""
		}
	}
	return dir
}

// sanitizeCertDirName 只保留最后一段，拒绝 "." / ".." / 空名（防目录穿越）。
func sanitizeCertDirName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	name = filepath.Base(name)
	if name == "." || name == ".." || name == string(filepath.Separator) {
		return ""
	}
	return name
}
