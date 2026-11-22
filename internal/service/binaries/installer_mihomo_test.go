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

// 滚动 tag（Prerelease-Alpha）的资产名带 commit 后缀：按版本号拼 URL 会 404，
// 必须从 release 的资产列表里挑真实资产，并优先 compatible 变体。
func TestPickReleaseAssetRollingTag(t *testing.T) {
	const compatDigest = "ea6d8fed8fc3b197bfc840089cdfde06ac265997b6992ca928f194b5941b8465"
	releases := []GitHubRelease{
		{
			TagName: "Prerelease-Alpha",
			Assets: []GitHubAsset{
				{Name: "mihomo-linux-amd64-alpha-100ff73.gz", BrowserDownloadURL: "https://example.com/plain.gz", Digest: "sha256:" + compatDigest},
				{Name: "mihomo-linux-amd64-v3-alpha-100ff73.gz", BrowserDownloadURL: "https://example.com/v3.gz", Digest: "sha256:" + compatDigest},
				{Name: "mihomo-linux-amd64-v1-go123-alpha-100ff73.gz", BrowserDownloadURL: "https://example.com/v1go.gz", Digest: "sha256:" + compatDigest},
				{Name: "mihomo-linux-amd64-compatible-alpha-100ff73.gz", BrowserDownloadURL: "https://example.com/compat.gz", Digest: "sha256:" + compatDigest},
				{Name: "mihomo-linux-amd64-alpha-100ff73.deb", BrowserDownloadURL: "https://example.com/plain.deb"},
			},
		},
	}
	asset, ok := pickReleaseAsset(releases, "mihomo", "Prerelease-Alpha", "linux", "amd64")
	if !ok {
		t.Fatal("应挑出 amd64 资产")
	}
	if asset.Name != "mihomo-linux-amd64-compatible-alpha-100ff73.gz" {
		t.Fatalf("应优先 compatible 资产，得到 %q", asset.Name)
	}
	if asset.BrowserDownloadURL != "https://example.com/compat.gz" {
		t.Fatalf("下载地址应取资产真实地址，得到 %q", asset.BrowserDownloadURL)
	}
	if got := normalizeDigest(asset.Digest); got != compatDigest {
		t.Fatalf("digest 归一化失败：%q", got)
	}

	// 找不到对应 tag 时必须返回 false（调用方回退到拼 URL）。
	if _, ok := pickReleaseAsset(releases, "mihomo", "v9.9.9", "linux", "amd64"); ok {
		t.Fatal("不存在的 tag 不应挑出资产")
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
