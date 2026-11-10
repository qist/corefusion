package router

import (
	"strings"
	"testing"
)

// 配了隐藏管理路径（webPath）后，根路径不得再提供前端静态文件：
// 否则任何人抓一份 /assets/*.js 或 /favicon.ico 就能确认这里跑的是 CoreFusion，
// 隐藏入口形同虚设（真机现象：直连源站 10111 根路径能拿到面板资源）。
func TestAdminStaticAssetPath(t *testing.T) {
	const wp = "/h7zgitxltt5z"
	cases := []struct {
		name    string
		path    string
		webPath string
		want    string
		ok      bool
	}{
		// 根部署（webPath 为空）：保持原行为，资源走根路径。
		{"根部署 assets", "/assets/index-abc.js", "", "assets/index-abc.js", true},
		{"根部署 favicon", "/favicon.ico", "", "favicon.ico", true},
		{"根部署 非资源路径", "/login", "", "", false},
		{"根部署 根路径", "/", "", "", false},

		// 隐藏路径部署：只认 <webPath>/ 下的资源。
		{"隐藏路径 assets", wp + "/assets/index-abc.js", wp, "assets/index-abc.js", true},
		{"隐藏路径 favicon", wp + "/favicon.ico", wp, "favicon.ico", true},
		{"隐藏路径 根级 png", wp + "/icon-192.png", wp, "icon-192.png", true},
		{"裸 assets 必须拒绝", "/assets/index-abc.js", wp, "", false},
		{"裸 favicon 必须拒绝", "/favicon.ico", wp, "", false},
		{"前缀相似必须拒绝", wp + "2/favicon.ico", wp, "", false},
		{"隐藏路径下多级非 assets 必须拒绝", wp + "/nested/file.js", wp, "", false},
		{"隐藏路径根本身不是资源", wp + "/", wp, "", false},

		// 目录穿越一律拒绝（embed.FS 也会拒绝，这里提前挡住）。
		{"穿越", wp + "/assets/../../etc/passwd", wp, "", false},
		{"穿越根部署", "/assets/../secret", "", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := adminStaticAssetPath(tc.path, tc.webPath)
			if ok != tc.ok || got != tc.want {
				t.Fatalf("adminStaticAssetPath(%q, %q) = (%q, %v)，期望 (%q, %v)",
					tc.path, tc.webPath, got, ok, tc.want, tc.ok)
			}
		})
	}
}

// index.html 里的根绝对引用必须全部改到隐藏路径下，否则浏览器会去请求已被
// 404 的根路径（favicon/图标/JS/CSS 全挂），外链与门户路径不受影响。
func TestRewriteIndexAssetRefs(t *testing.T) {
	const wp = "/h7zgitxltt5z"
	in := `<head>
  <link rel="icon" href="/favicon.ico" sizes="48x48" />
  <link rel="icon" type="image/png" sizes="192x192" href="/icon-192.png" />
  <link rel="apple-touch-icon" href="/apple-touch-icon.png" />
  <script type="module" crossorigin src="/assets/index-B8M_9sIt.js"></script>
  <link rel="stylesheet" crossorigin href="/assets/index-DPNC27CH.css">
  <link rel="preconnect" href="https://fonts.example.com">
  <img src="/p/qrcode.min.js">
</head>`

	got := rewriteIndexAssetRefs(in, wp)
	for _, want := range []string{
		`href="` + wp + `/favicon.ico"`,
		`href="` + wp + `/icon-192.png"`,
		`href="` + wp + `/apple-touch-icon.png"`,
		`src="` + wp + `/assets/index-B8M_9sIt.js"`,
		`href="` + wp + `/assets/index-DPNC27CH.css"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("index.html 缺少重写后的引用 %q\ngot:\n%s", want, got)
		}
	}
	for _, keep := range []string{
		`href="https://fonts.example.com"`, // 外链不能动
		`src="/p/qrcode.min.js"`,           // 门户路径不走 webPath
	} {
		if !strings.Contains(got, keep) {
			t.Errorf("不应改写的引用被改掉了：%q\ngot:\n%s", keep, got)
		}
	}
	if strings.Contains(got, `href="/favicon.ico"`) || strings.Contains(got, `src="/assets/`) {
		t.Errorf("仍有未重写的根路径引用：\n%s", got)
	}

	// 根部署（webPath 为空）时保持原样。
	if out := rewriteIndexAssetRefs(in, ""); out != in {
		t.Errorf("根部署不应改写 index.html")
	}
}
