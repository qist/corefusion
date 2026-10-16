package portal

import (
	"encoding/json"
	"strings"
	"testing"
)

const sampleRenderMdJS = `var ref = Vue.ref
var renderMd = function(md) {
  if (!md) return ''
  try { return marked.parse(md, { breaks: true, gfm: true }) } catch(e) { return md }
}
var other = 1
`

const sampleTicketHTML = `<div class="ticket-msg-content" v-html="renderMd(msg.content)"></div>`

// 存量门户模板（存在数据库里）必须被打上 M7 补丁：工单消息（用户可提交）不再执行 HTML。
func TestPatchTemplateSecurityAppliesToStoredTemplate(t *testing.T) {
	html2, js2, changed := PatchTemplateSecurity(sampleTicketHTML, sampleRenderMdJS)
	if !changed {
		t.Fatal("含旧写法的模板应被判定为需要修改")
	}
	if strings.Contains(html2, oldTicketRender) {
		t.Error("工单消息仍在使用 renderMd（用户内容会执行 HTML）")
	}
	if !strings.Contains(html2, newTicketRender) {
		t.Error("工单消息应改为 renderMdUntrusted")
	}
	if !strings.Contains(js2, rendererPresence) {
		t.Error("应插入 renderMdUntrusted 渲染器")
	}
	if !strings.Contains(js2, "var escapeHtmlForMd = function(s)") {
		t.Error("应插入 HTML 转义器")
	}

	// ⚠ 关键：渲染器必须插在 renderMd 函数**之外**。插进函数体里它会变成局部变量，
	// 门户里 v-html="renderMdUntrusted(...)" 就取不到（undefined）✗。
	// 判据：renderMd 函数体内的 catch 行必须**在**渲染器之前出现。
	bodyIdx := strings.Index(js2, "catch(e) { return md }")
	rendererIdx := strings.Index(js2, rendererPresence)
	if bodyIdx < 0 || rendererIdx < 0 || bodyIdx > rendererIdx {
		t.Error("渲染器被插进了 renderMd 函数体内（应插在函数右花括号之后）")
	}
	// 原函数本体与其它代码不得被破坏（插入点在两者之间，所以整段不再连续，需分开断言）
	fnBody := "var renderMd = function(md) {\n" +
		"  if (!md) return ''\n" +
		"  try { return marked.parse(md, { breaks: true, gfm: true }) } catch(e) { return md }\n" +
		"}"
	if !strings.Contains(js2, fnBody) {
		t.Error("renderMd 函数本体应保持完整（只做精确插入，不改动它）")
	}
	if !strings.Contains(js2, "var other = 1") || !strings.Contains(js2, "var ref = Vue.ref") {
		t.Error("插入不得影响其它已有代码")
	}
	if strings.Count(js2, rendererPresence) != 1 {
		t.Error("渲染器只应插入一次")
	}
}

// 幂等：打过补丁的模板再跑一次不应再改（避免每次启动都写库/丢失管理员定制）。
func TestPatchTemplateSecurityIsIdempotent(t *testing.T) {
	html2, js2, _ := PatchTemplateSecurity(sampleTicketHTML, sampleRenderMdJS)
	html3, js3, changed := PatchTemplateSecurity(html2, js2)
	if changed {
		t.Error("第二次调用不应再判定为需要修改")
	}
	if html3 != html2 || js3 != js2 {
		t.Error("第二次调用不应改动任何内容")
	}
}

// 保守性：已定制的模板若本就没有旧写法，一个字都不能动（绝不覆盖管理员定制）。
func TestPatchTemplateSecurityLeavesCustomizedTemplateAlone(t *testing.T) {
	custom := `<div v-html="renderMdUntrusted(msg.content)"></div>`
	js := "var myOwnRenderer = 1\n"
	html2, js2, changed := PatchTemplateSecurity(custom, js)
	if changed {
		t.Error("已安全的模板不应被改动")
	}
	if html2 != custom || js2 != js {
		t.Error("内容必须原样返回")
	}

	// 只有 HTML 命中、JS 里找不到锚点 → 仍要修 HTML（不要因 JS 而整体放弃）
	html3, js3, changed3 := PatchTemplateSecurity(sampleTicketHTML, "var nothing = 1\n")
	if !changed3 || strings.Contains(html3, oldTicketRender) {
		t.Error("找不到 JS 锚点时也要修工单那一行")
	}
	if js3 != "var nothing = 1\n" {
		t.Error("找不到锚点时 JS 必须原样")
	}
}

// 片段里不能有反引号：同一段代码还要插进 defaultPortalJS（Go 原始字符串），
// 有反引号会把 Go 字符串提前结束、直接编译失败（我踩过一次 ✗）。
func TestRendererSnippetHasNoBacktick(t *testing.T) {
	if strings.Contains(renderMdUntrustedJS, "`") {
		t.Fatal("渲染器片段不得包含反引号（会破坏 defaultPortalJS 这个 Go 原始字符串）")
	}
	// 默认模板（新装/重置时种子进库的那份）也必须带同样的渲染器
	if !strings.Contains(defaultPortalJS, rendererPresence) {
		t.Error("默认门户 JS 也应内置 renderMdUntrusted（新装门户直接安全）")
	}
}

// 客户端登出也必须对**存量模板**生效：旧 doLogout 只清本地存储（服务端仍认旧 token ✗），
// 补丁要把它换成"先调 /client-auth/logout 吊销（令牌版本 +1）"。
func TestPatchTemplateSecurityUpgradesLogout(t *testing.T) {
	js := `var ref = Vue.ref
    var doLogout = function() {
      token.value = ''
      refreshToken.value = ''
      localStorage.removeItem('client_token')
      localStorage.removeItem('client_refresh_token')
      client.value = {}
    }
var other = 1
`
	_, newJS, changed := PatchTemplateSecurity("", js)
	if !changed {
		t.Fatal("含旧 doLogout 的模板应被判定为需要修改")
	}
	if !strings.Contains(newJS, "/api/v1/client-auth/logout") {
		t.Error("登出应先在服务端吊销（否则旧实现只清本地存储、服务端照旧认 token）")
	}
	if !strings.Contains(newJS, "Authorization") {
		t.Error("吊销请求要带 Authorization 头")
	}
	if !strings.Contains(newJS, "localStorage.removeItem('client_token')") {
		t.Error("本地清理仍要保留（否则前端状态残留）")
	}
	if !strings.Contains(newJS, "var other = 1") {
		t.Error("其它代码不得被改动")
	}

	// 幂等：再跑一次不应再改
	_, again, changed2 := PatchTemplateSecurity("", newJS)
	if changed2 {
		t.Error("第二次调用不应再判定为需要修改")
	}
	if again != newJS {
		t.Error("第二次调用不应改动内容")
	}
}

// 存量门户模板（存在数据库里）的 account_info 模块必须被迁移为「充值按钮默认关闭」：
// 只有后台显式开启（show_charge_btn=true）才显示，否则默认隐藏。
func TestPatchChargeButtonConfigDefaultsOff(t *testing.T) {
	// 未设置 show_charge_btn + 旧条件 → 迁移为严格条件并补 false
	// 注意：条件含双引号，塞进 JSON 字符串需转义
	esc := strings.ReplaceAll(oldChargeBtnCond, `"`, `\"`)
	cfg := `{"modules":[{"id":"mod-account","type":"account_info","data":{"html":"<el-button ` + esc + `>充值</el-button>"}}]}`
	out, changed := PatchChargeButtonConfig(cfg)
	if !changed {
		t.Fatal("含旧充值按钮条件的存量模板应被判定为需要修改")
	}
	html, toggle := accountModuleOf(t, out)
	if strings.Contains(html, oldChargeBtnCond) {
		t.Error("旧的「未显式关闭就显示」条件应被替换")
	}
	if !strings.Contains(html, newChargeBtnCond) {
		t.Error("应改为「后台显式开启才显示」的严格条件")
	}
	if toggle != false {
		t.Errorf("未设置 show_charge_btn 时应补 false（默认关闭），实际 %v", toggle)
	}

	// 幂等：再跑一次不应再改
	out2, changed2 := PatchChargeButtonConfig(out)
	if changed2 {
		t.Error("第二次调用不应再判定为需要修改")
	}
	if out2 != out {
		t.Error("第二次调用不应改动内容")
	}

	// 管理员已显式开启 → 保留 true，仅迁移旧条件
	cfgOn := `{"modules":[{"id":"mod-account","type":"account_info","data":{"html":"<el-button ` + esc + `>充值</el-button>","show_charge_btn":true}}]}`
	on, changedOn := PatchChargeButtonConfig(cfgOn)
	if !changedOn {
		t.Fatal("旧条件仍应被迁移")
	}
	if _, toggleOn := accountModuleOf(t, on); toggleOn != true {
		t.Error("管理员已开启的 show_charge_btn=true 不应被改成 false")
	}
}

// accountModuleOf 解析 config JSON，返回 account_info 模块的 html 与 show_charge_btn 值。
func accountModuleOf(t *testing.T, configStr string) (html string, showChargeBtn bool) {
	t.Helper()
	var cfg struct {
		Modules []struct {
			Type string                 `json:"type"`
			Data map[string]interface{} `json:"data"`
		} `json:"modules"`
	}
	if err := json.Unmarshal([]byte(configStr), &cfg); err != nil {
		t.Fatalf("config JSON 非法: %v", err)
	}
	for _, m := range cfg.Modules {
		if m.Type != "account_info" {
			continue
		}
		html, _ = m.Data["html"].(string)
		showChargeBtn, _ = m.Data["show_charge_btn"].(bool)
		return html, showChargeBtn
	}
	t.Fatal("未找到 account_info 模块")
	return "", false
}

const (
	// 历史版本仍直接遍历 portalNavItems（无需改动，本来就全显示）
	sampleNavHTML = `<div v-for="nav in portalNavItems" :key="nav.id" :class="'sidebar-item' + (activeTab === nav.target ? ' active' : '')" @click="onNavClick(nav)"></div>
<div v-for="nav in portalNavItems" :key="nav.id" :class="'nav-item' + (activeTab === nav.target ? ' active' : '')" @click="onNavClick(nav)"></div>`
	sampleNavJS = `    var portalNavItems = ref([])
    // 在线支付接口（充值站点）不可用时隐藏「购买服务 / 我的账单」导航（无数据、无意义）
    var chargeSitesLoaded = ref(false)
    var navItems = computed(function() {
      var items = portalNavItems.value
      if (!chargeSitesLoaded.value || chargeSites.value.length > 0) return items
      return items.filter(function(n) { return n.target !== 'store' && n.target !== 'billing' })
    })
      currentModules: currentModules, portalNavItems: portalNavItems, navItems: navItems, onNavClick: onNavClick,
`
)

// 存量模板里「按充值站点过滤导航」的历史写法应升级为透传（购买服务/我的账单 始终显示）。
func TestPatchChargeNavMenu(t *testing.T) {
	h2, j2, changed := PatchChargeNavMenu(sampleNavHTML, sampleNavJS)
	if !changed {
		t.Fatal("含历史「过滤导航」写法的模板应被判定为需要修改")
	}
	if strings.Contains(j2, "chargeSites.value.length > 0") {
		t.Error("导航不应再按充值站点过滤")
	}
	if !strings.Contains(j2, newNavItemsPassthrough) {
		t.Error("导航 computed 应升级为透传 portalNavItems")
	}

	// 幂等：再跑一次不应再改
	h3, j3, changed2 := PatchChargeNavMenu(h2, j2)
	if changed2 {
		t.Error("第二次调用不应再判定为需要修改")
	}
	if h3 != h2 || j3 != j2 {
		t.Error("第二次调用不应改动内容")
	}
}

// 存量门户导航「购买服务 / 我的账单」必须默认关闭（与 nav.logs 一致）。
func TestPatchNavDefaultsOff(t *testing.T) {
	const sample = `{"navigation":[{"id":"nav-services","label":"nav.store","enabled":true},{"id":"nav-billing","label":"nav.billing","enabled":true},{"id":"nav-home","label":"nav.home","enabled":true}]}`
	out, changed := PatchNavDefaultsOff(sample)
	if !changed {
		t.Fatal("购买服务/我的账单 enabled=true 应被判定为需要修改")
	}
	if !strings.Contains(out, `"id":"nav-services"`) || !strings.Contains(out, `"id":"nav-billing"`) {
		t.Error("导航项应保留")
	}
	// 两项都应为 false，nav-home 仍为 true
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(out), &cfg); err != nil {
		t.Fatalf("输出 JSON 非法: %v", err)
	}
	for _, ni := range cfg["navigation"].([]interface{}) {
		nav := ni.(map[string]interface{})
		enabled := nav["enabled"].(bool)
		switch nav["id"] {
		case "nav-services", "nav-billing":
			if enabled {
				t.Errorf("%v 应默认关闭", nav["id"])
			}
		case "nav-home":
			if !enabled {
				t.Error("nav-home 不应被改动")
			}
		}
	}
	// 幂等：再跑一次不应再改
	out2, changed2 := PatchNavDefaultsOff(out)
	if changed2 {
		t.Error("第二次调用不应再判定为需要修改")
	}
	if out2 != out {
		t.Error("第二次调用不应改动内容")
	}
}

// 订阅链接「购买套餐」必须随「购买服务」导航关闭而隐藏。
func TestPatchSubStoreActionConfig(t *testing.T) {
	const sample = `{"modules":[{"id":"mod-sub","type":"subscription_detail","data":{"html":"<el-button v-for=\"action in mod.data.actions\" :key=\"action.target\">x</el-button>"}},{"id":"mod-other","type":"hero","data":{"html":"v-for=\"action in mod.data.actions\""}}]}`
	out, changed := PatchSubStoreActionConfig(sample)
	if !changed {
		t.Fatal("订阅链接旧 v-for 源应被判定为需要修改")
	}
	if !strings.Contains(out, "visibleSubActions(mod.data.actions)") {
		t.Error("订阅链接 v-for 源应升级为 visibleSubActions")
	}
	// 只改 subscription_detail，不动其它模块
	if strings.Count(out, "visibleSubActions(mod.data.actions)") != 1 {
		t.Error("只应改写 subscription_detail 模块")
	}
	// 幂等
	out2, changed2 := PatchSubStoreActionConfig(out)
	if changed2 {
		t.Error("第二次调用不应再判定为需要修改")
	}
	if out2 != out {
		t.Error("第二次调用不应改动内容")
	}
}

// 存量门户 shell 必须补 storeEnabled/visibleSubActions（含 ext 透传）。
func TestPatchSubStoreActionJS(t *testing.T) {
	const sampleJS = `    var portalModules = ref([])
    var portalNavItems = ref([])
    var navItems = computed(function() { return portalNavItems.value })
              var ext = {
                refreshing: refreshing,
              }
`
	h2, j2, changed := PatchSubStoreActionJS("", sampleJS)
	if !changed {
		t.Fatal("缺少 storeEnabled 的存量 shell 应被判定为需要修改")
	}
	if !strings.Contains(j2, subStoreActionJSPresence) {
		t.Error("应补 visibleSubActions（含 storeEnabled）")
	}
	if !strings.Contains(j2, "storeEnabled: storeEnabled") {
		t.Error("应在 ext 里透传 storeEnabled/visibleSubActions")
	}
	// 幂等
	h3, j3, changed2 := PatchSubStoreActionJS(h2, j2)
	if changed2 {
		t.Error("第二次调用不应再判定为需要修改")
	}
	if h3 != h2 || j3 != j2 {
		t.Error("第二次调用不应改动内容")
	}
}

// 默认门户模板的导航必须始终显示（不按充值站点过滤）。
func TestDefaultPortalNavAlwaysVisible(t *testing.T) {
	if !strings.Contains(defaultPortalHTML, `v-for="nav in navItems"`) {
		t.Error("默认门户 HTML 导航应使用 navItems")
	}
	if !strings.Contains(defaultPortalJS, newNavItemsPassthrough) {
		t.Error("默认门户 JS 的 navItems 应为透传（不按充值站点过滤）")
	}
	if strings.Contains(defaultPortalJS, "chargeSitesLoaded") {
		t.Error("默认门户 JS 不应再有 chargeSitesLoaded 导航过滤逻辑")
	}
}

// 存量门户 hero 的「管理您的订阅与服务」按钮必须补「充值/余额」联动条件。
func TestPatchHeroButtonConfig(t *testing.T) {
	const sample = `{"modules":[{"id":"mod-hero","type":"hero","data":{"button_text":"hero.manage_sub","button_target":"store","html":"<button v-if=\"mod.data && mod.data.button_text && mod.data.button_target !== 'overview'\" class=\"hero-btn\">x</button>"}}]}`
	out, changed := PatchHeroButtonConfig(sample)
	if !changed {
		t.Fatal("hero 旧条件应被判定为需要修改")
	}
	if !strings.Contains(out, "chargeUiEnabled") {
		t.Error("hero 按钮条件应补 chargeUiEnabled")
	}
	// 幂等：再跑一次不应再改
	out2, changed2 := PatchHeroButtonConfig(out)
	if changed2 {
		t.Error("第二次调用不应再判定为需要修改")
	}
	if out2 != out {
		t.Error("第二次调用不应改动内容")
	}
}

// 默认门户模板的 hero 按钮必须带 chargeUiEnabled 联动条件。
func TestDefaultPortalHeroHasChargeGate(t *testing.T) {
	if !strings.Contains(defaultModulesJSON, "chargeUiEnabled") {
		t.Error("默认门户 hero 按钮应带 chargeUiEnabled 联动条件")
	}
}

// 存量门户 shell 必须补 chargeUiEnabled（computed + ext 透传），否则 hero 按钮恒隐藏。
func TestPatchChargeUiEnabled(t *testing.T) {
	const sampleJS = `    var portalModules = ref([])
    var portalNavItems = ref([])
    var navItems = computed(function() { return portalNavItems.value })
              var ext = {
                refreshing: refreshing, rotating: rotating,
              }
`
	h2, j2, changed := PatchChargeUiEnabled("", sampleJS)
	if !changed {
		t.Fatal("缺少 chargeUiEnabled 的存量 shell 应被判定为需要修改")
	}
	if !strings.Contains(j2, chargeUiPresence) {
		t.Error("应补 chargeUiEnabled computed")
	}
	if !strings.Contains(j2, "chargeUiEnabled: chargeUiEnabled") {
		t.Error("应在 ext 里透传 chargeUiEnabled（模块作用域可见）")
	}

	// 幂等：再跑一次不应再改
	h3, j3, changed2 := PatchChargeUiEnabled(h2, j2)
	if changed2 {
		t.Error("第二次调用不应再判定为需要修改")
	}
	if h3 != h2 || j3 != j2 {
		t.Error("第二次调用不应改动内容")
	}
}
