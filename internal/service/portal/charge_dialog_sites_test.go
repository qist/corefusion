package portal

import (
	"strings"
	"testing"
)

const oldChargeSitesJS = `    var fetchChargeSites = function() {
      api('GET', '/charge/sites').then(function(res) {
        chargeSites.value = Array.isArray(res) ? res : []
      }).catch(function() {})
    }

    // --- Submit charge (online payment) ---
`

// 存量模板：直接点「去充值」时 chargeSites 为空 → 只剩卡密。
// 补丁必须让打开弹窗就加载站点，并默认选中在线支付。
func TestPatchChargeDialogSites(t *testing.T) {
	out, changed := PatchChargeDialogSites(oldChargeSitesJS)
	if !changed {
		t.Fatal("旧模板应被补丁修改")
	}
	if !strings.Contains(out, "watch(showChargeDialog") {
		t.Error("缺少打开充值弹窗时加载站点的 watch")
	}
	if !strings.Contains(out, "return api('GET', '/charge/sites')") {
		t.Error("fetchChargeSites 应返回 promise，供调用方设置默认站点")
	}
	if !strings.Contains(out, "chargeForm.value.method = 'online'") {
		t.Error("拉到站点后应默认切到在线支付")
	}
	if !strings.Contains(out, "// --- Submit charge (online payment) ---") {
		t.Error("补丁不能破坏后续代码")
	}

	// 幂等
	again, changedAgain := PatchChargeDialogSites(out)
	if changedAgain || again != out {
		t.Error("重复打补丁应保持原样（幂等）")
	}
}

// 模板可能用 CRLF：跨行片段必须先归一化再匹配，且输出保持原换行风格。
func TestPatchChargeDialogSitesCRLF(t *testing.T) {
	crlf := strings.ReplaceAll(oldChargeSitesJS, "\n", "\r\n")
	out, changed := PatchChargeDialogSites(crlf)
	if !changed {
		t.Fatal("CRLF 模板也应匹配")
	}
	if !strings.Contains(out, "watch(showChargeDialog") {
		t.Error("CRLF 模板补丁后缺少 watch")
	}
	if !strings.Contains(out, "\r\n") {
		t.Error("应保持原有 CRLF 换行")
	}
	if strings.Contains(out, "\n") && !strings.Contains(out, "\r\n") {
		t.Error("换行风格混用")
	}
}

// 找不到旧片段（自定义模板）时不能改动内容，避免把别的模板搞坏。
func TestPatchChargeDialogSitesSkipsUnknownTemplate(t *testing.T) {
	const custom = "    var fetchChargeSites = function() { /* custom */ }\n"
	out, changed := PatchChargeDialogSites(custom)
	if changed {
		t.Error("未知模板不应被修改")
	}
	if out != custom {
		t.Error("未知模板内容应原样返回")
	}
}
