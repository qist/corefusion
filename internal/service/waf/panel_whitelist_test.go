package waf

import (
	"strings"
	"testing"
)

// 面板管理接口白名单必须覆盖「会携带 URL/备注/卡号等自由文本」的接口，
// 否则管理员保存支付站点（site_url=http://127.0.0.1:xxxx）会被 SSRF 规则
// 拦成 403（线上 waf 日志 rule_tag=http://127\.0\.0\.1）。
func TestPanelForwardWhitelistCoversAdminContentAPIs(t *testing.T) {
	required := []string{
		"^/api/v1/charge-sites/ url_attack,url_args,post",
		"^/api/v1/charge-cards/ url_attack,url_args,post",
		"^/api/v1/charge-orders/ url_attack,url_args,post",
		"^/api/v1/clients/ url_attack,url_args,post",
		"^/api/v1/plan-templates/ url_attack,url_args,post",
		"^/api/v1/plan-variants/ url_attack,url_args,post",
		"^/api/v1/traffic-packs/ url_attack,url_args,post",
		"^/api/v1/promotions/ url_attack,url_args,post",
		"^/api/v1/client-balances/ url_attack,url_args,post",
		"^/api/v1/tickets/ url_attack,url_args,post",
		"^/api/v1/articles/ url_attack,url_args,post",
	}
	for _, line := range required {
		if !strings.Contains(panelForwardWhitelist, line) {
			t.Errorf("面板白名单缺少：%s", line)
		}
	}
	// 白名单是「按接口前缀」而非整段 /api/ 放行（避免门户/回调也失去防护）
	if strings.Contains(panelForwardWhitelist, "\n^/api/ ") {
		t.Error("不应整段放行 /api/")
	}
	if strings.Contains(panelForwardWhitelist, "/api/v1/client-auth/") {
		t.Error("客户端接口不应加入管理端白名单")
	}
}
