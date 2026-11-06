package router

import "testing"

// 子路径部署（webPath=/h7zgitxltt5z）时，直接访问无尾斜杠的 webPath 或站点根，
// 必须跳到 "<webPath>/"：否则 SPA fallback 返回的 index.html 里 vue-router 的 base
// 是 "<webPath>/"，当前 path 匹配不上 base → 路由无匹配 → 整页白屏。
// 真机现象：https://httpdns.tycng.com/h7zgitxltt5z 转发面板"不跳转、打不开"。
func TestAdminBaseRedirectTarget(t *testing.T) {
	cases := []struct {
		name    string
		path    string
		webPath string
		want    string
		ok      bool
	}{
		{"无尾斜杠的 webPath", "/h7zgitxltt5z", "/h7zgitxltt5z", "/h7zgitxltt5z/", true},
		{"站点根", "/", "/h7zgitxltt5z", "/h7zgitxltt5z/", true},
		{"已带尾斜杠（必须放过，否则 302 到自己 → 无限重定向）", "/h7zgitxltt5z/", "/h7zgitxltt5z", "", false},
		{"子路由", "/h7zgitxltt5z/login", "/h7zgitxltt5z", "", false},
		{"根部署时根路径不跳", "/", "", "", false},
		{"根部署时业务路径不跳", "/login", "", "", false},
		{"前缀相似但不同的路径不跳", "/h7zgitxltt5z2", "/h7zgitxltt5z", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := adminBaseRedirectTarget(tc.path, tc.webPath)
			if ok != tc.ok || got != tc.want {
				t.Fatalf("adminBaseRedirectTarget(%q, %q) = (%q, %v)，期望 (%q, %v)",
					tc.path, tc.webPath, got, ok, tc.want, tc.ok)
			}
		})
	}
}
