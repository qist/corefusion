package binaries

import (
	"strings"
	"testing"
)

// mihomo 的资产命名和其它核心不同：amd64 用 -compatible 变体，非 Windows 是
// **单个二进制的 .gz**（不是 tar.gz），Windows 是 .zip。URL 拼错就会 404。
func TestBuildDownloadURLMihomo(t *testing.T) {
	cases := []struct {
		osName, arch, want string
	}{
		{"linux", "amd64", "mihomo-linux-amd64-compatible-v1.19.34.gz"},
		{"linux", "arm64", "mihomo-linux-arm64-v1.19.34.gz"},
		{"darwin", "amd64", "mihomo-darwin-amd64-compatible-v1.19.34.gz"},
		{"windows", "amd64", "mihomo-windows-amd64-compatible-v1.19.34.zip"},
	}
	for _, c := range cases {
		got := buildDownloadURL("mihomo", "MetaCubeX/mihomo", "v1.19.34", c.osName, c.arch)
		if !strings.HasSuffix(got, "/"+c.want) {
			t.Errorf("%s/%s: got %q, want suffix %q", c.osName, c.arch, got, c.want)
		}
	}
}

// 版本列表只保留真正有目标平台资产的 release（避免前端列出装不上的版本）。
func TestFilterAssetMihomo(t *testing.T) {
	if !filterAsset("mihomo", "mihomo-linux-amd64-compatible-v1.19.34.gz", "linux", "amd64") {
		t.Fatal("mihomo linux/amd64 资产应被识别")
	}
	if filterAsset("mihomo", "mihomo-linux-arm64-v1.19.34.gz", "linux", "amd64") {
		t.Fatal("arm64 资产不应匹配 amd64")
	}
	if filterAsset("mihomo", "mihomo-linux-amd64-v1.19.34.deb", "linux", "amd64") {
		t.Fatal("deb 包不应被当成可直接运行的二进制")
	}
}
