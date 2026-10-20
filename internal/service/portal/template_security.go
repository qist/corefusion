package portal

import (
	"context"
	"encoding/json"
	"log"
	"regexp"
	"strings"
)

// renderMdUntrustedJS 是"用户内容渲染器"的 JS 片段（先 HTML 转义、再走 marked）。
//
// ⚠ 片段里**不能出现反引号** —— 同一段代码还要插进 defaultPortalJS（Go 原始字符串）✗。
const renderMdUntrustedJS = `
    // 用户提交的内容（工单消息等）不能直接交给 marked + v-html：
    // marked 默认放行原始 HTML，而门户是 v-html 渲染，所以客户在工单里写 script 或
    // img onerror 就能在客服/管理员的浏览器里执行（跨角色存储型 XSS，审计 v5 M7）。
    // 这里先做 HTML 转义再走 markdown：Markdown 语法照常生效，标签只当文本显示。
    // 管理员自撰内容（公告/文章/freeform 模块）仍用 renderMd —— 那属于后台特权内容。
    var escapeHtmlForMd = function(s) {
      return String(s == null ? '' : s)
        .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;').replace(/'/g, '&#39;')
    }
    var renderMdUntrusted = function(md) {
      if (!md) return ''
      var safe = escapeHtmlForMd(md)
      try { return marked.parse(safe, { breaks: true, gfm: true }) } catch(e) { return safe }
    }
`

// 旧/新 doLogout：旧实现只清前端本地存储 ✗（服务端仍认那个 token，被窃取后有效期内照样可用），
// 新实现先调 `/api/v1/client-auth/logout` 让服务端把令牌版本 +1（此前签发的所有 token 立即失效）✓。
//
// 同样要处理**存量模板**：门户模板存在数据库里 ✗（真机验证过：改默认模板到不了已部署门户），
// 所以这里做一次精确片段替换，幂等、且不动管理员的其它定制。
const (
	oldLogoutSnippet = `    var doLogout = function() {
      token.value = ''
      refreshToken.value = ''
      localStorage.removeItem('client_token')
      localStorage.removeItem('client_refresh_token')
      client.value = {}
    }`

	newLogoutSnippet = `    var doLogout = function() {
      var done = function() {
        token.value = ''
        refreshToken.value = ''
        localStorage.removeItem('client_token')
        localStorage.removeItem('client_refresh_token')
        client.value = {}
      }
      var tk = token.value
      if (!tk) { done(); return }
      // 先在服务端吊销（令牌版本 +1 → 该客户端此前签发的所有 token 立即失效）。
      // 旧实现只清本地存储，服务端仍旧认那个 token（被窃取后有效期内还能用）。
      fetch('/api/v1/client-auth/logout', {
        method: 'POST',
        headers: { 'Authorization': 'Bearer ' + tk }
      }).then(function() { done() }).catch(function() { done() })
    }`
)

const (
	// 工单消息是**用户可提交**内容 → 必须用不可信渲染器
	oldTicketRender = `v-html="renderMd(msg.content)"`
	newTicketRender = `v-html="renderMdUntrusted(msg.content)"`

	renderMdAnchor   = `var renderMd = function(md) {`
	rendererPresence = `var renderMdUntrusted = function(md) {`
)

// L-5：OAuth 回调 token 由 query 下发改为 **fragment** 下发（token 不再进
// 访问日志/Referer/浏览器历史）。存量模板的 handleOAuthCallback 还在读
// location.search ✗ → 后端已改为 #fragment 跳回，读不到就登录失败 ——
// 这里把"读 query"的精确片段替换为"读 fragment"。幂等 ✓。
const (
	oldOAuthReadSnippet = `      var params = new URLSearchParams(window.location.search)
      var at = params.get('access_token')`

	newOAuthReadSnippet = `      var params = new URLSearchParams(window.location.hash.slice(1))
      var at = params.get('access_token')`
)

// P3-3：自助解封 —— 存量模板补齐解封卡。解封卡分 6 处（HTML 1 处 + JS 5 处），
// 每处独立判定、独立幂等；存量模板缺哪块补哪块，命中过的跳过 ✓。
// 注：片段里**不能出现反引号**（本文件是 Go 源码，JS 片段统一单引号 ✓）。
const (
	// JS-1：handleLogin 加 require_self_unban 分支（被封账号登录不再直接报错，
	// 改为显示解封卡；不发常规 token，M2 语义不破）
	oldSULoginSnippet = `      api('POST', '/login', loginForm.value).then(function(res) {
        token.value = res.access_token`

	newSULoginSnippet = `      api('POST', '/login', loginForm.value).then(function(res) {
        // 自助解封分支：被封但可自解 → 不发常规 token，显示解封卡
        if (res.require_self_unban) {
          selfUnban.value = { visible: true, token: res.self_unban_token || '', hint: res.self_unban_hint || '', loading: false }
          ElMessage.warning(res.self_unban_hint || t('auth.self_unban_desc'))
          return
        }
        token.value = res.access_token`

	// JS-2：handleOAuthCallback 加 require_self_unban 分支（OAuth 回调带解封
	// fragment → 显示解封卡，不进入登录态）
	oldSUOAuthSnippet = `    var handleOAuthCallback = function() {
      var params = new URLSearchParams(window.location.hash.slice(1))
      var at = params.get('access_token')`

	newSUOAuthSnippet = `    var handleOAuthCallback = function() {
      var params = new URLSearchParams(window.location.hash.slice(1))
      // 自助解封分支：OAuth 回调带 require_self_unban=1 → 显示解封卡，不进入登录态
      if (params.get('require_self_unban') === '1') {
        var suToken = params.get('self_unban_token') || ''
        var suHint = params.get('self_unban_hint') || ''
        window.history.replaceState({}, '', window.location.pathname)
        if (suToken) {
          selfUnban.value = { visible: true, token: suToken, hint: suHint, loading: false }
          ElMessage.warning(suHint || t('auth.self_unban_desc'))
        } else {
          ElMessage.error(t('auth.self_unban_failed'))
        }
        return
      }
      var at = params.get('access_token')`

	// JS-3：selfUnban ref + handleSelfUnban 函数体（插到 handleLogin 函数之后）
	suFuncPresence = `var handleSelfUnban = function() {`
	suFuncAnchor   = `var handleLogin = function() {`

	suFuncSnippet = `
    // ── 自助解封 ✓（登录/OAuth 返回 require_self_unban → 显示解封卡）──
    var selfUnban = ref({ visible: false, token: '', hint: '', loading: false })
    var handleSelfUnban = function() {
      selfUnban.value.loading = true
      fetch('/api/v1/client-auth/self-unban', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Authorization': 'Bearer ' + selfUnban.value.token }
      })
        .then(function(r) { return r.json().then(function(d) { return { ok: r.ok, data: d } }) })
        .then(function(x) {
          if (x.ok) {
            ElMessage.success(t('auth.self_unban_success'))
            selfUnban.value = { visible: false, token: '', hint: '', loading: false }
          } else {
            ElMessage.error((x.data && (x.data.message || x.data.error)) || t('auth.self_unban_failed'))
          }
        })
        .catch(function() { ElMessage.error(t('auth.self_unban_failed')) })
        .finally(function() { selfUnban.value.loading = false })
    }`

	// JS-4：auth 区域组件上下文透传（解封卡 HTML 在 auth 区域内编译，
	// 不透传 selfUnban/handleSelfUnban 渲染时直接 undefined ✗）
	oldSUAuthCtxSnippet = `        // Methods
        handleLogin: handleLogin,
        handleRegister: handleRegister,
        t: t,`

	newSUAuthCtxSnippet = `        // Methods
        handleLogin: handleLogin,
        handleRegister: handleRegister,
        // 自助解封卡 ✓（必须透传进 authCtx，否则渲染时 selfUnban undefined ✗）
        selfUnban: selfUnban,
        handleSelfUnban: handleSelfUnban,
        t: t,`

	// JS-5：主 setup return 透传（管理端若把卡片移出 auth 区域仍可用）
	oldSUReturnSnippet = `      handleLogin: handleLogin, handleRegister: handleRegister, handleLogout: handleLogout,`

	newSUReturnSnippet = `      handleLogin: handleLogin, handleRegister: handleRegister, handleLogout: handleLogout,
      selfUnban: selfUnban, handleSelfUnban: handleSelfUnban,`

	// HTML：登录卡 tabs 前插解封卡 + tabs 加 v-if（卡片显示时隐藏 tabs）
	oldSUTabsSnippet = `          <el-tabs v-model="authTab" class="auth-tabs">`

	newSUTabsSnippet = `          <!-- 自助解封卡 ✓：流量超额被自动封禁的账号登录时返回 require_self_unban，
               凭登录签发的受限专用 token（仅可调 /self-unban）一键恢复，无需联系客服 -->
          <div class="self-unban-card" v-if="selfUnban.visible">
            <el-alert type="warning" :title="t('auth.self_unban_title')" :description="selfUnban.hint || t('auth.self_unban_desc')" show-icon :closable="false" style="margin-bottom:14px" />
            <el-button type="warning" class="btn-block" :loading="selfUnban.loading" @click="handleSelfUnban" size="large">{{ t('auth.self_unban_btn') }}</el-button>
            <el-button class="btn-block" text @click="selfUnban.visible = false">{{ t('auth.self_unban_cancel') }}</el-button>
          </div>
          <el-tabs v-if="!selfUnban.visible" v-model="authTab" class="auth-tabs">`
)

// regions.auth（登录卡区域）是**真正渲染**的登录卡 —— 主 HTML 里的兜底卡只在
// regions 缺失时才用 ✗。存量模板的 config JSON 里 auth 区域没有解封卡 →
// 必须随 config 一起补齐（JSON 感知：unmarshal → 改 html/js → marshal）。
// 注意此处的锚点是 **JSON 反序列化后**的形态（真实换行/引号），与上面
// 主 HTML/JS 的原文锚点缩进不同 ✓。
const (
	// 区域 HTML：tabs 前插解封卡 + tabs 加 v-if（区域内 4 空格缩进）
	oldSURegionTabsSnippet = `    <el-tabs v-model="authTab" class="auth-tabs">`

	newSURegionTabsSnippet = `    <!-- 自助解封卡 ✓：流量超额被自动封禁的账号登录时返回 require_self_unban，
         凭登录签发的受限专用 token（仅可调 /self-unban）一键恢复，无需联系客服 -->
    <div class="self-unban-card" v-if="selfUnban.visible">
      <el-alert type="warning" :title="t('auth.self_unban_title')" :description="selfUnban.hint || t('auth.self_unban_desc')" show-icon :closable="false" style="margin-bottom:14px" />
      <el-button type="warning" class="btn-block" :loading="selfUnban.loading" @click="handleSelfUnban" size="large">{{ t('auth.self_unban_btn') }}</el-button>
      <el-button class="btn-block" text @click="selfUnban.visible = false">{{ t('auth.self_unban_cancel') }}</el-button>
    </div>
    <el-tabs v-if="!selfUnban.visible" v-model="authTab" class="auth-tabs">`

	// 区域 JS：setup return 透传 selfUnban/handleSelfUnban（ctx 来自主 JS 的 authCtx ✓）
	oldSURegionJSReturn = `  handleLogin,
  handleRegister,
  oauthProviders: ctx.oauthProviders,`

	newSURegionJSReturn = `  handleLogin,
  handleRegister,
  selfUnban: ctx.selfUnban,
  handleSelfUnban: ctx.handleSelfUnban,
  oauthProviders: ctx.oauthProviders,`
)

// PatchRegionsSecurity 对模板 config JSON 里的 regions.auth（登录卡区域源码）
// 打自助解封补丁。幂等：命中过的跳过；config 非法/无 regions 时原样返回。
// json.Marshal 会把 config 压缩成紧凑格式 —— 语义不变，编辑器读的是结构化字段 ✓。
func PatchRegionsSecurity(configStr string) (string, bool) {
	if strings.TrimSpace(configStr) == "" {
		return configStr, false
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(configStr), &cfg); err != nil {
		return configStr, false
	}
	regions, ok := cfg["regions"].(map[string]interface{})
	if !ok {
		return configStr, false
	}
	auth, ok := regions["auth"].(map[string]interface{})
	if !ok {
		return configStr, false
	}
	changed := false
	if html, ok := auth["html"].(string); ok && strings.Contains(html, oldSURegionTabsSnippet) {
		auth["html"] = strings.Replace(html, oldSURegionTabsSnippet, newSURegionTabsSnippet, 1)
		changed = true
	}
	if js, ok := auth["js"].(string); ok &&
		strings.Contains(js, oldSURegionJSReturn) && !strings.Contains(js, "selfUnban: ctx.selfUnban") {
		auth["js"] = strings.Replace(js, oldSURegionJSReturn, newSURegionJSReturn, 1)
		changed = true
	}
	if !changed {
		return configStr, false
	}
	out, err := json.Marshal(cfg)
	if err != nil {
		return configStr, false
	}
	return string(out), true
}

// 门户账户模块「充值按钮」默认关闭 —— 存量模板的 account_info 模块 HTML 里按钮仍是
// 「未显式关闭就显示」的旧写法 ✗（门户模板存在数据库里 → 只改默认模板到不了已部署门户 ✗，
// 与 M7/自助解封是同一个坑 ✓）。这里做**精确片段替换** + 未设置时补 false，使
// 「默认关闭、后台手动开启才显示」对存量模板同样生效；幂等，且不动管理员的其它定制 ✓。
const (
	oldChargeBtnCond = `v-if="!mod.data || mod.data.show_charge_btn !== false"`
	newChargeBtnCond = `v-if="mod.data && mod.data.show_charge_btn === true"`
)

// PatchChargeButtonConfig 对模板 config JSON 里的 account_info 模块打「充值按钮默认关闭」
// 补丁：把旧的「未显式关闭就显示」条件替换为「显式开启才显示」，并在 data 未设置
// show_charge_btn 时补 false（默认关闭 ✓）。幂等：命中过 / 无 account_info 时原样返回 ✓。
func PatchChargeButtonConfig(configStr string) (string, bool) {
	if strings.TrimSpace(configStr) == "" {
		return configStr, false
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(configStr), &cfg); err != nil {
		return configStr, false
	}
	modules, ok := cfg["modules"].([]interface{})
	if !ok {
		return configStr, false
	}
	changed := false
	for _, mi := range modules {
		mod, ok := mi.(map[string]interface{})
		if !ok || mod["type"] != "account_info" {
			continue
		}
		data, ok := mod["data"].(map[string]interface{})
		if !ok {
			data = map[string]interface{}{}
			mod["data"] = data
		}
		if html, ok := data["html"].(string); ok && strings.Contains(html, oldChargeBtnCond) {
			data["html"] = strings.Replace(html, oldChargeBtnCond, newChargeBtnCond, 1)
			changed = true
		}
		if _, ok := data["show_charge_btn"]; !ok {
			data["show_charge_btn"] = false
			changed = true
		}
	}
	if !changed {
		return configStr, false
	}
	out, err := json.Marshal(cfg)
	if err != nil {
		return configStr, false
	}
	return string(out), true
}

// 存量门户 hero 模块的「管理您的订阅与服务」按钮补「充值/余额」联动条件：
// 账户模块「显示余额行」或「显示充值按钮」任一关闭 → 按钮隐藏 ✓。
// 幂等：命中过 / 已是新版时原样返回 ✓（片段里**不能出现反引号**）。
const (
	oldHeroBtnCond = `v-if="mod.data && mod.data.button_text && mod.data.button_target !== 'overview'"`
	newHeroBtnCond = `v-if="mod.data && mod.data.button_text && mod.data.button_target !== 'overview' && chargeUiEnabled"`
)

// PatchHeroButtonConfig 对模板 config JSON 里的 hero 模块打「充值/余额联动」补丁
// （只改按钮 v-if 条件，其它内容一字不动）；幂等：命中过 / 无 hero 时原样返回 ✓。
func PatchHeroButtonConfig(configStr string) (string, bool) {
	if strings.TrimSpace(configStr) == "" {
		return configStr, false
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(configStr), &cfg); err != nil {
		return configStr, false
	}
	modules, ok := cfg["modules"].([]interface{})
	if !ok {
		return configStr, false
	}
	changed := false
	for _, mi := range modules {
		mod, ok := mi.(map[string]interface{})
		if !ok || mod["type"] != "hero" {
			continue
		}
		data, ok := mod["data"].(map[string]interface{})
		if !ok {
			continue
		}
		html, ok := data["html"].(string)
		if !ok || strings.Contains(html, "chargeUiEnabled") || !strings.Contains(html, oldHeroBtnCond) {
			continue
		}
		data["html"] = strings.Replace(html, oldHeroBtnCond, newHeroBtnCond, 1)
		changed = true
	}
	if !changed {
		return configStr, false
	}
	out, err := json.Marshal(cfg)
	if err != nil {
		return configStr, false
	}
	return string(out), true
}

// 导航「购买服务 / 我的账单」**始终展示**（余额 / 充值卡仍可下单）；无可用充值站点时只隐藏
// 「在线支付」方式，不隐藏导航。历史版本曾按充值站点可用性过滤导航 → 刷新时会一闪而过，
// 这里把已迁移模板的导航 computed 升级回**透传**（幂等 ✓；片段里**不能出现反引号**）。
const (
	navItemsPresence = `var navItems = computed(function() {`
	// 历史版本：按充值站点可用性过滤导航（会导致刷新闪烁）
	oldNavItemsFilter = `var navItems = computed(function() {
      var items = portalNavItems.value
      if (!chargeSitesLoaded.value || chargeSites.value.length > 0) return items
      return items.filter(function(n) { return n.target !== 'store' && n.target !== 'billing' })
    })`
	// 升级后：透传全部导航（始终显示购买服务 / 我的账单）
	newNavItemsPassthrough = `var navItems = computed(function() {
      return portalNavItems.value
    })`
)

// PatchChargeNavMenu 对存量门户模板打「导航始终显示」补丁：把历史版本「按充值站点过滤
// 购买服务/我的账单」的导航 computed 升级回透传，避免刷新时导航一闪而过。
// 幂等：命中过 / 已是新版时原样返回 ✓。
func PatchChargeNavMenu(htmlContent, jsContent string) (newHTML, newJS string, changed bool) {
	newHTML, newJS = htmlContent, jsContent

	if strings.Contains(newJS, oldNavItemsFilter) {
		newJS = strings.Replace(newJS, oldNavItemsFilter, newNavItemsPassthrough, 1)
		changed = true
	}

	return newHTML, newJS, changed
}

// 存量门户模板的「套餐升级」改造补丁（2026-10-19 语义调整：升级 = 到期预约切换）。
// 三处必须一起改，缺一不可：
//  1. 跨套餐下单仍发 new → 后端以「已有生效套餐」拒绝，门户根本升不了级 ✗；
//  2. 回调返回 scheduled 时旧模板只认 completed → 用户看不到任何成功提示 ✗；
//  3. 订单状态表缺 scheduled → 「我的账单」显示原始英文 key ✗。
//
// 幂等：命中过 / 已是新版时原样返回 ✓（片段里**不能出现反引号**）。
const (
	oldPurchaseOrderType = `    var purchaseVariant = function(variantId, plan, modData) {
      var orderType = (client.value.plan_id > 0 && client.value.plan_id === plan.id) ? 'renew' : 'new'`

	newPurchaseOrderType = `    var purchaseVariant = function(variantId, plan, modData) {
      // 同套餐 = 续费（累加时长与配额）；跨套餐 = 升级预约（当前周期结束后才切换）。
      // 跨套餐若仍发 new，后端会以「已有生效套餐」拒绝，等于门户无法升级 ✗。
      var orderType = 'new'
      if (client.value.plan_id > 0) {
        orderType = (client.value.plan_id === plan.id) ? 'renew' : 'upgrade'
      }`

	oldOrderDoneBranch = `        showPaymentDialog.value = false
        if (res.status === 'completed') {
          ElMessage.success(t('plan.buy_success'))`

	newOrderDoneBranch = `        showPaymentDialog.value = false
        if (res.status === 'scheduled') {
          ElMessage.success(t('plan.upgrade_scheduled'))
          fetchMyOrders()
        } else if (res.status === 'completed') {
          ElMessage.success(t('plan.buy_success'))`

	oldOrderStatusText = `      return { pending: t('order.status_pending'), paid: t('order.status_paid'), completed: t('order.status_completed'), cancelled: t('order.status_cancelled'), refunded: t('order.status_refunded') }[s] || s || '-'`

	newOrderStatusText = `      return { pending: t('order.status_pending'), paid: t('order.status_paid'), scheduled: t('order.status_scheduled'), completed: t('order.status_completed'), cancelled: t('order.status_cancelled'), refunded: t('order.status_refunded') }[s] || s || '-'`

	oldOrderStatusBadge = `      return { pending: 'processing', paid: 'active', completed: 'active', cancelled: 'none', refunded: 'expired' }[s] || 'none'`

	newOrderStatusBadge = `      return { pending: 'processing', paid: 'active', scheduled: 'processing', completed: 'active', cancelled: 'none', refunded: 'expired' }[s] || 'none'`

	// 旧文案宣传「立即生效 + 差价换算」，与新语义相反 → 只在仍是原文时替换 ✓
	oldPlanNoticeZH = `套餐可叠加购买，剩余有效期自动叠加。如需升级请直接购买新套餐，剩余有效期会根据等级差价换算，然后再叠加(新套餐立即生效)。以下套餐只能个人使用，禁止倒卖/组团拼车购买等行为，发现则封号处理（如需多人使用请联系客服定制团队套餐）。`
	newPlanNoticeZH = `套餐可叠加购买，剩余有效期自动叠加。如需升级请直接购买更高档套餐：升级在当前套餐到期后生效，按新套餐一个完整周期计费，不会吞掉你已付费的剩余时长。以下套餐只能个人使用，禁止倒卖/组团拼车购买等行为，发现则封号处理（如需多人使用请联系客服定制团队套餐）。`

	oldPlanNoticeEN = `Plans can be stacked. Remaining validity period will be automatically extended. To upgrade, purchase a new plan directly — remaining time will be converted based on tier difference and stacked (new plan takes effect immediately). Plans are for personal use only. Reselling/group buying is prohibited and will result in account suspension (contact support for team plans).`
	newPlanNoticeEN = `Plans can be stacked. Remaining validity period will be automatically extended. To upgrade, purchase a higher-tier plan directly: the upgrade takes effect after the current plan expires and is charged for one full new cycle, so no paid time is lost. Plans are for personal use only. Reselling/group buying is prohibited and will result in account suspension (contact support for team plans).`
)

// PatchPlanUpgradeScheduledJS 对存量门户模板打「升级预约切换」补丁（只改上面三处精确片段）。
// 幂等：已是新版时原样返回 ✓。
func PatchPlanUpgradeScheduledJS(htmlContent, jsContent string) (newHTML, newJS string, changed bool) {
	newHTML, newJS = htmlContent, jsContent

	if strings.Contains(newJS, oldPurchaseOrderType) {
		newJS = strings.Replace(newJS, oldPurchaseOrderType, newPurchaseOrderType, 1)
		changed = true
	}
	if strings.Contains(newJS, oldOrderDoneBranch) {
		newJS = strings.Replace(newJS, oldOrderDoneBranch, newOrderDoneBranch, 1)
		changed = true
	}
	if strings.Contains(newJS, oldOrderStatusText) {
		newJS = strings.Replace(newJS, oldOrderStatusText, newOrderStatusText, 1)
		changed = true
	}
	if strings.Contains(newJS, oldOrderStatusBadge) {
		newJS = strings.Replace(newJS, oldOrderStatusBadge, newOrderStatusBadge, 1)
		changed = true
	}

	return newHTML, newJS, changed
}

// PatchPlanUpgradeScheduledConfig 给模板 config 里的语言包补新键（scheduled 状态 / 预约提示），
// 并把仍在宣传「立即生效」的旧 notice_text 换成新语义文案。自定义过的文案不动 ✓。
// 幂等：第二次调用返回 changed=false ✓。
func PatchPlanUpgradeScheduledConfig(configStr string) (string, bool) {
	if strings.TrimSpace(configStr) == "" {
		return configStr, false
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(configStr), &cfg); err != nil {
		return configStr, false
	}
	lang, ok := cfg["lang"].(map[string]interface{})
	if !ok {
		return configStr, false
	}
	locales, ok := lang["locales"].(map[string]interface{})
	if !ok {
		return configStr, false
	}

	changed := false
	for code, raw := range locales {
		pack, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		switch code {
		case "zh-CN", "zh":
			if cur, _ := pack["plan.notice_text"].(string); cur == oldPlanNoticeZH {
				pack["plan.notice_text"] = newPlanNoticeZH
				changed = true
			}
			if _, exists := pack["plan.upgrade_scheduled"]; !exists {
				pack["plan.upgrade_scheduled"] = "升级已预约，将在当前套餐到期后生效"
				changed = true
			}
			if _, exists := pack["order.status_scheduled"]; !exists {
				pack["order.status_scheduled"] = "待生效"
				changed = true
			}
		default:
			if cur, _ := pack["plan.notice_text"].(string); cur == oldPlanNoticeEN {
				pack["plan.notice_text"] = newPlanNoticeEN
				changed = true
			}
			if _, exists := pack["plan.upgrade_scheduled"]; !exists {
				pack["plan.upgrade_scheduled"] = "Upgrade scheduled, it will take effect after your current plan expires"
				changed = true
			}
			if _, exists := pack["order.status_scheduled"]; !exists {
				pack["order.status_scheduled"] = "Scheduled"
				changed = true
			}
		}
	}
	if !changed {
		return configStr, false
	}
	out, err := json.Marshal(cfg)
	if err != nil {
		return configStr, false
	}
	return string(out), true
}

// 存量门户导航「购买服务(nav.store) / 我的账单(nav.billing)」默认关闭（与 nav.logs 一致）：
// 门户导航存在模板 config 里（数据库）→ 只改默认模板到不了已部署门户 ✗。这里把这两项
// enabled 置为 false（管理员仍可在编辑器里手动打开 ✓）。幂等：已是 false / 不存在时原样返回 ✓。
var defaultOffNavIDs = map[string]bool{
	"nav-services": true,
	"nav-billing":  true,
}

// PatchNavDefaultsOff 对模板 config JSON 里的导航打「购买服务 / 我的账单默认关闭」补丁。
// 幂等：命中过的第二次调用返回 changed=false ✓。
func PatchNavDefaultsOff(configStr string) (string, bool) {
	if strings.TrimSpace(configStr) == "" {
		return configStr, false
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(configStr), &cfg); err != nil {
		return configStr, false
	}
	navigation, ok := cfg["navigation"].([]interface{})
	if !ok {
		return configStr, false
	}
	changed := false
	for _, ni := range navigation {
		nav, ok := ni.(map[string]interface{})
		if !ok {
			continue
		}
		id, _ := nav["id"].(string)
		if !defaultOffNavIDs[id] {
			continue
		}
		if enabled, ok := nav["enabled"].(bool); !ok || enabled {
			nav["enabled"] = false
			changed = true
		}
	}
	if !changed {
		return configStr, false
	}
	out, err := json.Marshal(cfg)
	if err != nil {
		return configStr, false
	}
	return string(out), true
}

// 订阅链接（subscription_detail）里的「购买套餐」(target=store) 在「购买服务」导航关闭时
// 不应显示 —— 与导航开关联动 ✓。存量模板的模块 HTML 仍是 v-for="action in mod.data.actions"，
// 这里升级为 visibleSubActions(mod.data.actions)（购买服务关闭时剔除 store 动作）。
// 幂等：命中过 / 无 subscription_detail 时原样返回 ✓。
const (
	oldSubActionsLoop = `v-for="action in mod.data.actions"`
	newSubActionsLoop = `v-for="action in visibleSubActions(mod.data.actions)"`

	subStoreActionJSPresence = `var visibleSubActions = function(`
	subStoreActionJSSnippet  = `    var portalNavItems = ref([])
    // 「购买服务」导航（target=store）是否启用：关闭时订阅链接里的「购买套餐」入口不应显示。
    var storeEnabled = computed(function() {
      var navs = (typeof PORTAL_CONFIG !== 'undefined' && Array.isArray(PORTAL_CONFIG.navigation)) ? PORTAL_CONFIG.navigation : []
      for (var i = 0; i < navs.length; i++) {
        if (navs[i] && navs[i].target === 'store') return navs[i].enabled !== false
      }
      return true
    })
    var visibleSubActions = function(actions) {
      if (!Array.isArray(actions)) return actions
      if (storeEnabled.value) return actions
      return actions.filter(function(a) { return !(a && a.target === 'store') })
    }`
	subStoreActionExtExposed = `var ext = {
                storeEnabled: storeEnabled, visibleSubActions: visibleSubActions,`
)

// PatchSubStoreActionConfig 对模板 config JSON 里的 subscription_detail 模块打
// 「购买套餐随购买服务关闭而隐藏」补丁（只改动作按钮的 v-for 源）。幂等 ✓。
func PatchSubStoreActionConfig(configStr string) (string, bool) {
	if strings.TrimSpace(configStr) == "" {
		return configStr, false
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(configStr), &cfg); err != nil {
		return configStr, false
	}
	modules, ok := cfg["modules"].([]interface{})
	if !ok {
		return configStr, false
	}
	changed := false
	for _, mi := range modules {
		mod, ok := mi.(map[string]interface{})
		if !ok || mod["type"] != "subscription_detail" {
			continue
		}
		data, ok := mod["data"].(map[string]interface{})
		if !ok {
			continue
		}
		if html, ok := data["html"].(string); ok && strings.Contains(html, oldSubActionsLoop) {
			data["html"] = strings.Replace(html, oldSubActionsLoop, newSubActionsLoop, 1)
			changed = true
		}
	}
	if !changed {
		return configStr, false
	}
	out, err := json.Marshal(cfg)
	if err != nil {
		return configStr, false
	}
	return string(out), true
}

// PatchSubStoreActionJS 给存量门户 shell 补 storeEnabled computed + visibleSubActions 函数，
// 并透传到模块作用域（ext）。幂等：命中过 / 已是新版时原样返回 ✓（片段里**不能出现反引号**）。
func PatchSubStoreActionJS(htmlContent, jsContent string) (newHTML, newJS string, changed bool) {
	newHTML, newJS = htmlContent, jsContent
	if !strings.Contains(newJS, subStoreActionJSPresence) && strings.Contains(newJS, chargeUiAnchor) {
		newJS = strings.Replace(newJS, chargeUiAnchor, subStoreActionJSSnippet, 1)
		changed = true
	}
	if !strings.Contains(newJS, "storeEnabled: storeEnabled") && strings.Contains(newJS, chargeUiExtAnchor) {
		newJS = strings.Replace(newJS, chargeUiExtAnchor, subStoreActionExtExposed, 1)
		changed = true
	}
	return newHTML, newJS, changed
}

// 存量门户 shell 缺少 chargeUiEnabled computed：hero「管理您的订阅与服务」按钮的 v-if 引用了它，
// 未定义时恒为 falsy → 按钮恒隐藏 ✗（真机踩过 ✓）。这里给存量 shell 补上 computed，并透传到
// 模块作用域（ext 自动合并）——与默认模板同一机制 ✓（片段里**不能出现反引号**）。
const (
	chargeUiAnchor   = `    var portalNavItems = ref([])`
	chargeUiPresence = `var chargeUiEnabled = computed(function() {`
	chargeUiSnippet  = `    var portalNavItems = ref([])
    // 充值/余额 UI 开关：账户模块「显示余额行」或「显示充值按钮」任一关闭即视为关闭（hero 联动）
    var chargeUiEnabled = computed(function() {
      var acct = null
      for (var i = 0; i < portalModules.value.length; i++) {
        if (portalModules.value[i].type === 'account_info') { acct = portalModules.value[i]; break }
      }
      if (!acct || !acct.data) return true
      return !(acct.data.show_balance === false || acct.data.show_charge_btn === false)
    })`
	chargeUiExtAnchor  = `var ext = {`
	chargeUiExtExposed = `var ext = {
                chargeUiEnabled: chargeUiEnabled,`
)

// PatchChargeUiEnabled 给存量门户 shell 补 chargeUiEnabled（computed + ext 透传），
// 使 hero 按钮的充值/余额联动在存量模板上同样生效。幂等：命中过 / 已是新版时原样返回 ✓。
func PatchChargeUiEnabled(htmlContent, jsContent string) (newHTML, newJS string, changed bool) {
	newHTML, newJS = htmlContent, jsContent

	if !strings.Contains(newJS, chargeUiPresence) && strings.Contains(newJS, chargeUiAnchor) {
		newJS = strings.Replace(newJS, chargeUiAnchor, chargeUiSnippet, 1)
		changed = true
	}
	if !strings.Contains(newJS, "chargeUiEnabled: chargeUiEnabled") && strings.Contains(newJS, chargeUiExtAnchor) {
		newJS = strings.Replace(newJS, chargeUiExtAnchor, chargeUiExtExposed, 1)
		changed = true
	}

	return newHTML, newJS, changed
}

// ─────────────────────────────────────────────────────────────────────────────
// 客户端面收尾（C-1 支付后轮询 / C-2 推荐提现 / C-4 订阅 IP 白名单管理）
//
// 门户模板存在数据库里 → 只改默认模板到不了已部署门户（与 M7/自助解封是同一个坑）。
// 这里对存量模板做精确、幂等的片段补齐：命中旧写法才改，已具备的跳过，
// 管理员的其它定制一字不动 ✓。片段里不能出现反引号（同一段代码还要进 defaultPortalJS）。
// ─────────────────────────────────────────────────────────────────────────────
const (
	clientFeatureRefsAnchor  = "var withdrawRecords = ref([])"
	clientFeatureRefsSnippet = `var showWithdrawDialog = ref(false)
var withdrawBusy = ref(false)
var withdrawForm = ref({ amount_yuan: 0, withdraw_type: 'balance', wallet_address: '' })
var ipWhitelistEntries = ref([])
var whitelistBusy = ref(false)`

	clientFeatureJSAnchor  = "var chargeStatusText = function(s) { return { pending: t('order.status_pending'), paid: t('order.status_paid'), processing: t('common.processing'), expired: t('common.expired'), failed: t('common.failed') }[s] || s || '-' }"
	clientFeatureJSSnippet = `    // --- C-1: 支付后轮询（套餐订单 / 充值订单共用） ---
    // 支付页在 window.open 的新标签里，门户这边只能轮询状态；到账后自动刷新余额/订单/套餐。
    var paymentPollTimer = null
    var stopPaymentPolling = function() {
      if (paymentPollTimer) { clearInterval(paymentPollTimer); paymentPollTimer = null }
    }
    var startPaymentPolling = function(res) {
      var tradeNo = res && res.trade_no
      var orderId = res && res.order_id
      if (!tradeNo && !orderId) return
      stopPaymentPolling()
      var startedAt = Date.now()
      var finish = function() {
        stopPaymentPolling()
        ElMessage.success(t('order.payment_arrived'))
        fetchMe(); fetchBalance(); fetchBalanceTxns(); fetchMyOrders(); fetchChargeOrders()
      }
      paymentPollTimer = setInterval(function() {
        if (Date.now() - startedAt > 300000) { stopPaymentPolling(); return }
        if (tradeNo) {
          api('GET', '/charge/status/' + encodeURIComponent(tradeNo)).then(function(o) {
            var st = o && o.status
            if (st === 'paid') finish()
            else if (st === 'failed' || st === 'expired' || st === 'refunded') stopPaymentPolling()
          }).catch(function() {})
        } else {
          api('GET', '/orders/' + orderId).then(function(o) {
            var st = o && o.status
            if (st === 'paid' || st === 'completed' || st === 'scheduled') finish()
            else if (st === 'cancelled' || st === 'refunded') stopPaymentPolling()
          }).catch(function() {})
        }
      }, 3000)
    }

    // --- C-2: 推荐提现申请 ---
    var openWithdrawDialog = function() {
      var stats = referralStats.value || {}
      withdrawForm.value = { amount_yuan: Math.floor((stats.available_commission || 0) / 100), withdraw_type: 'balance', wallet_address: '' }
      showWithdrawDialog.value = true
    }
    var submitWithdraw = function() {
      var f = withdrawForm.value || {}
      var cents = Math.round((Number(f.amount_yuan) || 0) * 100)
      var min = (referralStats.value && referralStats.min_withdraw_amount) || 0
      if (cents <= 0) { ElMessage.error(t('referral.withdraw_amount_invalid')); return }
      if (min > 0 && cents < min) { ElMessage.error(t('referral.withdraw_min_hint').replace('{amount}', (min / 100).toFixed(0))); return }
      if (referralStats.value && cents > referralStats.value.available_commission) { ElMessage.error(t('referral.withdraw_exceeds')); return }
      if (f.withdraw_type === 'usdt' && !f.wallet_address) { ElMessage.error(t('referral.wallet_required')); return }
      withdrawBusy.value = true
      api('POST', '/referral/withdraw', { amount: cents, withdraw_type: f.withdraw_type, wallet_address: f.wallet_address || '' }).then(function() {
        ElMessage.success(t('referral.withdraw_submitted'))
        showWithdrawDialog.value = false
        fetchReferral()
      }).catch(function(e) { ElMessage.error(e.message) }).finally(function() { withdrawBusy.value = false })
    }

    // --- C-4: 订阅 IP 白名单列表管理 ---
    var fetchWhitelistEntries = function() {
      api('GET', '/settings/whitelist').then(function(res) {
        ipWhitelistEntries.value = Array.isArray(res) ? res : []
      }).catch(function() {})
    }
    var autoAddCurrentIP = function() {
      whitelistBusy.value = true
      api('POST', '/settings/auto-add-ip', {}).then(function() {
        ElMessage.success(t('security.ip_added'))
        fetchWhitelistEntries(); fetchMe()
      }).catch(function(e) { ElMessage.error(e.message) }).finally(function() { whitelistBusy.value = false })
    }
    var removeWhitelistEntry = function(id) {
      whitelistBusy.value = true
      api('DELETE', '/settings/whitelist/' + id).then(function() {
        ElMessage.success(t('security.ip_removed'))
        fetchWhitelistEntries(); fetchMe()
      }).catch(function(e) { ElMessage.error(e.message) }).finally(function() { whitelistBusy.value = false })
    }`

	clientFeatureReferralOld = `      api('GET', '/referral/records?limit=20').then(function(res) {
        referralRecords.value = (res && res.items) ? res.items : []
      }).catch(function() {})
    }`
	clientFeatureReferralNew = `      api('GET', '/referral/records?limit=20').then(function(res) {
        referralRecords.value = (res && res.items) ? res.items : []
      }).catch(function() {})
      api('GET', '/referral/withdrawals?limit=20').then(function(res) {
        withdrawRecords.value = (res && res.items) ? res.items : []
      }).catch(function() {})
    }`

	clientFeatureInitOld = `      if (initHasReferral) fetchReferral()`
	clientFeatureInitNew = `      if (initHasReferral) fetchReferral()
      if (initMods.some(function(m) { return m.type === 'security_settings' })) fetchWhitelistEntries()`

	clientFeaturePageModsOld = `      var hasReferral = pageMods.some(function(m) { return m.type === 'referral' })`
	clientFeaturePageModsNew = `      var hasReferral = pageMods.some(function(m) { return m.type === 'referral' })
      var hasSecurity = pageMods.some(function(m) { return m.type === 'security_settings' })`

	clientFeaturePageFetchOld = `      if (hasReferral) fetchReferral()
      if (hasTickets) fetchTickets()`
	clientFeaturePageFetchNew = `      if (hasReferral) fetchReferral()
      if (hasSecurity) fetchWhitelistEntries()
      if (hasTickets) fetchTickets()`

	clientFeatureUnmountOld = `    onUnmounted(function() {
      if (refreshTimer) clearInterval(refreshTimer)
      window.removeEventListener('message', oauthMsgHandler)
    })

    watch(token, function(v) { if (v) { startAutoRefresh() } else { if (refreshTimer) clearInterval(refreshTimer) } })`
	clientFeatureUnmountNew = `    onUnmounted(function() {
      if (refreshTimer) clearInterval(refreshTimer)
      stopPaymentPolling()
      window.removeEventListener('message', oauthMsgHandler)
    })

    watch(token, function(v) { if (v) { startAutoRefresh() } else { if (refreshTimer) clearInterval(refreshTimer); stopPaymentPolling() } })`

	clientFeatureModuleCtxOld = `          referralCode: referralCode, referralStats: referralStats, referralRecords: referralRecords,`
	clientFeatureModuleCtxNew = `          referralCode: referralCode, referralStats: referralStats, referralRecords: referralRecords,
          showWithdrawDialog: showWithdrawDialog, withdrawForm: withdrawForm, withdrawBusy: withdrawBusy,
          openWithdrawDialog: openWithdrawDialog, submitWithdraw: submitWithdraw,
          ipWhitelistEntries: ipWhitelistEntries, whitelistBusy: whitelistBusy,
          fetchWhitelistEntries: fetchWhitelistEntries, autoAddCurrentIP: autoAddCurrentIP, removeWhitelistEntry: removeWhitelistEntry,`

	clientFeatureExtOld = `                commissionRecords: commissionRecords, withdrawRecords: withdrawRecords,`
	clientFeatureExtNew = `                commissionRecords: commissionRecords, withdrawRecords: withdrawRecords,
                showWithdrawDialog: showWithdrawDialog, withdrawForm: withdrawForm, withdrawBusy: withdrawBusy,
                openWithdrawDialog: openWithdrawDialog, submitWithdraw: submitWithdraw,
                ipWhitelistEntries: ipWhitelistEntries, whitelistBusy: whitelistBusy,
                fetchWhitelistEntries: fetchWhitelistEntries, autoAddCurrentIP: autoAddCurrentIP, removeWhitelistEntry: removeWhitelistEntry,`

	clientFeatureReturnOld = `      iconPaths: iconPaths, navIconPath: navIconPath, authComponent: authComponent,`
	clientFeatureReturnNew = `      iconPaths: iconPaths, navIconPath: navIconPath, authComponent: authComponent,
      // Referral withdraw
      referralStats: referralStats,
      showWithdrawDialog: showWithdrawDialog, withdrawForm: withdrawForm, withdrawBusy: withdrawBusy,
      openWithdrawDialog: openWithdrawDialog, submitWithdraw: submitWithdraw,`

	clientFeatureDialogHTML = `    <!-- Withdraw Dialog -->
    <el-dialog v-model="showWithdrawDialog" :title="t('referral.withdraw')" width="420px" align-center append-to-body>
      <div class="payment-dialog-content">
        <div class="pay-extra-label">{{ t('referral.withdraw_amount') }}（{{ t('charge.yuan') }}）</div>
        <el-input-number v-model="withdrawForm.amount_yuan" :min="0" :step="10" style="width: 100%" />
        <div class="pay-hint" style="margin-top:8px">{{ t('referral.withdrawable') }} {{ formatMoney((referralStats && referralStats.available_commission) || 0) }}</div>
        <div class="pay-hint">{{ t('referral.withdraw_min_hint').replace('{amount}', ((referralStats && referralStats.min_withdraw_amount) || 0) / 100) }}</div>
        <div class="pay-extra-label" style="margin-top:14px">{{ t('referral.withdraw_type') }}</div>
        <el-radio-group v-model="withdrawForm.withdraw_type">
          <el-radio label="balance">{{ t('referral.withdraw_type_balance') }}</el-radio>
          <el-radio label="usdt">{{ t('referral.withdraw_type_usdt') }}</el-radio>
        </el-radio-group>
        <div v-if="withdrawForm.withdraw_type === 'usdt'" style="margin-top:12px">
          <div class="pay-extra-label">{{ t('referral.wallet_address') }}</div>
          <el-input v-model="withdrawForm.wallet_address" :placeholder="t('referral.wallet_address_placeholder')" />
        </div>
      </div>
      <template #footer>
        <el-button @click="showWithdrawDialog = false">{{ t('common.cancel') }}</el-button>
        <el-button type="danger" :loading="withdrawBusy" @click="submitWithdraw">{{ t('referral.withdraw_submit') }}</el-button>
      </template>
    </el-dialog>

`

	clientFeatureWithdrawBtnOld = `<el-button type="danger" size="small" class="withdraw-btn" v-if="referralStats && referralStats.available_commission > 0">`
	clientFeatureWithdrawBtnNew = `<el-button type="danger" size="small" class="withdraw-btn" v-if="referralStats && referralStats.available_commission > 0" @click="openWithdrawDialog">`

	clientFeatureSecAnchor = `                <el-form-item :label="t('security.ip_whitelist_full')">
                  <el-input v-model="settings.ip_whitelist" :placeholder="t('security.ip_example')" />
                </el-form-item>`
	clientFeatureSecList = `                <div class="whitelist-manage" style="margin: 4px 0 12px">
                  <div style="display:flex;align-items:center;justify-content:space-between;gap:8px">
                    <span style="font-size:13px;font-weight:600">{{ t('security.ip_whitelist_list') }}</span>
                    <el-button link type="primary" size="small" :loading="whitelistBusy" @click="autoAddCurrentIP">{{ t('security.ip_add_current') }}</el-button>
                  </div>
                  <div v-if="!ipWhitelistEntries || ipWhitelistEntries.length === 0" class="card-desc" style="margin:6px 0 0">{{ t('security.ip_whitelist_empty') }}</div>
                  <div v-else style="margin-top:6px;display:flex;flex-direction:column;gap:6px">
                    <div v-for="entry in ipWhitelistEntries" :key="entry.id" style="display:flex;align-items:center;gap:8px;padding:6px 8px;border:1px solid var(--border);border-radius:6px">
                      <span style="flex:1;font-family:monospace;font-size:13px">{{ entry.ip_address }}</span>
                      <el-tag size="small" effect="plain">{{ entry.source === 'auto' ? t('security.ip_source_auto') : t('security.ip_source_manual') }}</el-tag>
                      <el-button link type="danger" size="small" :loading="whitelistBusy" @click="removeWhitelistEntry(entry.id)">{{ t('common.delete') }}</el-button>
                    </div>
                  </div>
                </div>`
)

// PatchClientFeatures 对存量门户模板补 C-1/C-2/C-4 的 shell JS/HTML：
//   - 支付页打开后轮询到账 + 到账刷新；
//   - 推荐提现弹窗与提交、提现记录拉取；
//   - 安全设置页的白名单条目管理（自动添加当前 IP / 删除）。
//
// 幂等：每处独立判定，已具备的跳过；找不到锚点保持原样，绝不覆盖管理员定制 ✓。
func PatchClientFeatures(htmlContent, jsContent string) (newHTML, newJS string, changed bool) {
	newHTML, newJS = htmlContent, jsContent

	// 1) 新增响应式状态（提现弹窗 + 白名单列表）
	if !strings.Contains(newJS, "var showWithdrawDialog = ref(false)") && strings.Contains(newJS, clientFeatureRefsAnchor) {
		newJS = strings.Replace(newJS, clientFeatureRefsAnchor, clientFeatureRefsAnchor+"\n"+clientFeatureRefsSnippet, 1)
		changed = true
	}

	// 2) 轮询 / 提现 / 白名单函数（插在 chargeStatusText 之后）
	if !strings.Contains(newJS, "var startPaymentPolling = function") && strings.Contains(newJS, clientFeatureJSAnchor) {
		newJS = strings.Replace(newJS, clientFeatureJSAnchor, clientFeatureJSAnchor+"\n\n"+clientFeatureJSSnippet, 1)
		changed = true
	}

	// 3) 三处支付跳转后启动轮询（套餐下单 / 升级 / 充值）
	if !strings.Contains(newJS, "startPaymentPolling(res)") && strings.Contains(newJS, "window.open(res.payment_url, '_blank')") {
		newJS = strings.ReplaceAll(newJS, "window.open(res.payment_url, '_blank')", "window.open(res.payment_url, '_blank')\n          startPaymentPolling(res)")
		changed = true
	}

	// 4) 推荐页同时拉取提现记录（否则弹窗提交后列表仍旧空白）
	if strings.Contains(newJS, clientFeatureReferralOld) && !strings.Contains(newJS, "/referral/withdrawals") {
		newJS = strings.Replace(newJS, clientFeatureReferralOld, clientFeatureReferralNew, 1)
		changed = true
	}

	// 5) 安全设置模块首次加载时拉白名单条目
	if !strings.Contains(newJS, clientFeatureInitNew) && strings.Contains(newJS, clientFeatureInitOld) {
		newJS = strings.Replace(newJS, clientFeatureInitOld, clientFeatureInitNew, 1)
		changed = true
	}
	// 5b) 切到「安全设置」页时也拉一次（首次登录默认在 overview，init 分支不会命中）
	if !strings.Contains(newJS, "var hasSecurity = pageMods.some") && strings.Contains(newJS, clientFeaturePageModsOld) {
		newJS = strings.Replace(newJS, clientFeaturePageModsOld, clientFeaturePageModsNew, 1)
		changed = true
	}
	if !strings.Contains(newJS, clientFeaturePageFetchNew) && strings.Contains(newJS, clientFeaturePageFetchOld) {
		newJS = strings.Replace(newJS, clientFeaturePageFetchOld, clientFeaturePageFetchNew, 1)
		changed = true
	}

	// 6) 卸载 / 登出时停止轮询（避免定时器泄漏）
	if !strings.Contains(newJS, clientFeatureUnmountNew) && strings.Contains(newJS, clientFeatureUnmountOld) {
		newJS = strings.Replace(newJS, clientFeatureUnmountOld, clientFeatureUnmountNew, 1)
		changed = true
	}

	// 7) 模块上下文透传（模块模板只能看到 ctx + ext 显式字段）
	if !strings.Contains(newJS, clientFeatureModuleCtxNew) && strings.Contains(newJS, clientFeatureModuleCtxOld) {
		newJS = strings.Replace(newJS, clientFeatureModuleCtxOld, clientFeatureModuleCtxNew, 1)
		changed = true
	}
	if !strings.Contains(newJS, clientFeatureExtNew) && strings.Contains(newJS, clientFeatureExtOld) {
		newJS = strings.Replace(newJS, clientFeatureExtOld, clientFeatureExtNew, 1)
		changed = true
	}
	// 主 app 的 setup return（shell HTML 里的提现弹窗需要这些状态/函数）
	if !strings.Contains(newJS, clientFeatureReturnNew) && strings.Contains(newJS, clientFeatureReturnOld) {
		newJS = strings.Replace(newJS, clientFeatureReturnOld, clientFeatureReturnNew, 1)
		changed = true
	}

	// 8) shell HTML 补提现弹窗
	if !strings.Contains(newHTML, "<!-- Withdraw Dialog -->") && strings.Contains(newHTML, "    <!-- More Subscriptions Dialog -->") {
		newHTML = strings.Replace(newHTML, "    <!-- More Subscriptions Dialog -->", clientFeatureDialogHTML+"    <!-- More Subscriptions Dialog -->", 1)
		changed = true
	}

	return newHTML, newJS, changed
}

// PatchClientFeaturesConfig 对模板 config JSON 里的模块 HTML 补 C-2/C-4 入口：
//   - 推荐模块的「提现」按钮绑定 openWithdrawDialog；
//   - 安全模块的 IP 白名单输入框下补条目列表（自动添加当前 IP / 删除）。
//
// 幂等：命中过的跳过；config 非法/无对应模块时原样返回 ✓。
func PatchClientFeaturesConfig(configStr string) (string, bool) {
	if strings.TrimSpace(configStr) == "" {
		return configStr, false
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(configStr), &cfg); err != nil {
		return configStr, false
	}
	modules, ok := cfg["modules"].([]interface{})
	if !ok {
		return configStr, false
	}
	changed := false
	for _, mi := range modules {
		mod, ok := mi.(map[string]interface{})
		if !ok {
			continue
		}
		data, ok := mod["data"].(map[string]interface{})
		if !ok {
			continue
		}
		html, ok := data["html"].(string)
		if !ok {
			continue
		}
		switch mod["id"] {
		case "mod-referral":
			if strings.Contains(html, clientFeatureWithdrawBtnOld) {
				data["html"] = strings.Replace(html, clientFeatureWithdrawBtnOld, clientFeatureWithdrawBtnNew, 1)
				changed = true
			}
		case "mod-sec":
			if !strings.Contains(html, "security.ip_whitelist_list") && strings.Contains(html, clientFeatureSecAnchor) {
				data["html"] = strings.Replace(html, clientFeatureSecAnchor, clientFeatureSecAnchor+"\n"+clientFeatureSecList, 1)
				changed = true
			}
		}
	}
	if !changed {
		return configStr, false
	}
	out, err := json.Marshal(cfg)
	if err != nil {
		return configStr, false
	}
	return string(out), true
}

// PatchTemplateSecurity 对**存量门户模板**做幂等的安全补丁（安全审计 v5 M7）。
//
// 为什么需要：门户模板存在数据库里、可在后台编辑，所以只改默认模板**到不了**已部署的门户
// （真机验证：默认模板已修，库里那份仍旧写 renderMd(msg.content) ✗）。做法刻意保守：
//   - 只做**精确片段替换**，命中才改；其它内容一字不动（绝不覆盖管理员的定制 ✗）；
//   - 幂等：已打过补丁的模板第二次调用返回 changed=false；
//   - 渲染器插到 `var renderMd = function...` 这个函数的**右花括号之后**（按花括号配对定位 ✓
//     —— 插在函数体内会让它变成局部变量，门户里就取不到了 ✗）。
func PatchTemplateSecurity(htmlContent, jsContent string) (newHTML, newJS string, changed bool) {
	newHTML, newJS = htmlContent, jsContent

	if strings.Contains(newHTML, oldTicketRender) {
		newHTML = strings.ReplaceAll(newHTML, oldTicketRender, newTicketRender)
		changed = true
	}

	if !strings.Contains(newJS, rendererPresence) {
		if patched, ok := insertAfterFunction(newJS, renderMdAnchor, renderMdUntrustedJS); ok {
			newJS = patched
			changed = true
		}
	}

	// 登出：只清本地存储 → 先在服务端吊销（令牌版本 +1）✓
	if strings.Contains(newJS, oldLogoutSnippet) {
		newJS = strings.Replace(newJS, oldLogoutSnippet, newLogoutSnippet, 1)
		changed = true
	}

	// OAuth 回调：token 从 URL query 改为 fragment 读取（安全审计 L-5）✓
	if strings.Contains(newJS, oldOAuthReadSnippet) {
		newJS = strings.Replace(newJS, oldOAuthReadSnippet, newOAuthReadSnippet, 1)
		changed = true
	}

	// ── P3-3：自助解封卡补齐（每处独立判定、独立幂等 ✓）──
	// JS-1：登录响应处理加 require_self_unban 分支
	if strings.Contains(newJS, oldSULoginSnippet) {
		newJS = strings.Replace(newJS, oldSULoginSnippet, newSULoginSnippet, 1)
		changed = true
	}
	// JS-2：OAuth 回调加 require_self_unban 分支
	if strings.Contains(newJS, oldSUOAuthSnippet) {
		newJS = strings.Replace(newJS, oldSUOAuthSnippet, newSUOAuthSnippet, 1)
		changed = true
	}
	// JS-3：selfUnban ref + handleSelfUnban（插到 handleLogin 函数之后）
	if !strings.Contains(newJS, suFuncPresence) {
		if patched, ok := insertAfterFunction(newJS, suFuncAnchor, suFuncSnippet); ok {
			newJS = patched
			changed = true
		}
	}
	// JS-4：auth 区域组件上下文透传 selfUnban/handleSelfUnban
	if strings.Contains(newJS, oldSUAuthCtxSnippet) {
		newJS = strings.Replace(newJS, oldSUAuthCtxSnippet, newSUAuthCtxSnippet, 1)
		changed = true
	}
	// JS-5：主 setup return 透传（旧片段是新片段的前缀 → 必须同时查新片段防重复追加 ✗）
	if strings.Contains(newJS, oldSUReturnSnippet) && !strings.Contains(newJS, newSUReturnSnippet) {
		newJS = strings.Replace(newJS, oldSUReturnSnippet, newSUReturnSnippet, 1)
		changed = true
	}
	// HTML：tabs 前插解封卡 + tabs 加 v-if
	if strings.Contains(newHTML, oldSUTabsSnippet) {
		newHTML = strings.Replace(newHTML, oldSUTabsSnippet, newSUTabsSnippet, 1)
		changed = true
	}

	return newHTML, newJS, changed
}

// insertAfterFunction 在 anchor 所指函数（按花括号配对找到其结束的 `}`）所在行的行尾之后
// 插入 snippet。找不到 anchor 或花括号不配对时返回 ok=false（保持原样 ✗）。
func insertAfterFunction(js, anchor, snippet string) (string, bool) {
	idx := strings.Index(js, anchor)
	if idx < 0 {
		return js, false
	}
	depth := 0
	started := false
	for i := idx; i < len(js); i++ {
		switch js[i] {
		case '{':
			depth++
			started = true
		case '}':
			depth--
			if started && depth == 0 {
				if nl := strings.Index(js[i:], "\n"); nl >= 0 {
					at := i + nl
					return js[:at] + "\n" + snippet + js[at:], true
				}
				return js + "\n" + snippet, true
			}
		}
	}
	return js, false
}

// selfClosingCustomTagRe 匹配模板里的自闭合自定义组件标签（如 <el-alert ... />、<Lightning />）。
// 门户 shell 是**通过 #app innerHTML 编译的 in-DOM 模板**：HTML 解析器不认自定义元素的自闭合写法，
// 会把 `<el-alert ... />` 当成开始标签，并把它后面的兄弟节点全部吞成自己的子节点 →
// 当该组件带 v-if 且条件为假时，后面的内容（弹窗空态 / 格式卡片 / 按钮）整块不渲染 ✗。
var selfClosingCustomTagRe = regexp.MustCompile(`<([A-Za-z][A-Za-z0-9-]*)\b[^<>]*?/>`)

// PatchInDOMSelfClosingComponents 把 in-DOM 模板里"后面还跟着兄弟节点"的自闭合自定义组件展开成
// 显式 `<tag ...></tag>`。幂等：仅命中紧跟兄弟内容的写法；紧跟 `</...>` 的（后面本无兄弟）保持原样。
func PatchInDOMSelfClosingComponents(html string) (string, bool) {
	matches := selfClosingCustomTagRe.FindAllStringSubmatchIndex(html, -1)
	if len(matches) == 0 {
		return html, false
	}
	var b strings.Builder
	last := 0
	changed := false
	for _, m := range matches {
		tag := html[m[2]:m[3]]
		if !isCustomComponentTag(tag) {
			continue
		}
		// 标签后的下一个非空白字符：若是结束标签 `</`，说明它后面本来就没有兄弟节点，安全跳过。
		j := m[1]
		for j < len(html) && (html[j] == ' ' || html[j] == '\t' || html[j] == '\n' || html[j] == '\r') {
			j++
		}
		if j+1 < len(html) && html[j] == '<' && html[j+1] == '/' {
			continue
		}
		b.WriteString(html[last : m[1]-2]) // 去掉结尾的 "/>"
		b.WriteString("></" + tag + ">")
		last = m[1]
		changed = true
	}
	if !changed {
		return html, false
	}
	b.WriteString(html[last:])
	return b.String(), true
}

// isCustomComponentTag 判断是否为自定义组件标签：带连字符（如 el-alert）或以大写开头（如 Lightning 图标）。
// 原生 void 元素（img/br/path 等）为小写且不含连字符，会被排除，不会误改。
func isCustomComponentTag(tag string) bool {
	if tag == "" {
		return false
	}
	return strings.Contains(tag, "-") || (tag[0] >= 'A' && tag[0] <= 'Z')
}

// EnsureSecurityPatches 给"当前生效的门户模板"打安全补丁（进程启动时调用一次）。
//
// 全程自动：不需要管理员做任何操作；只在命中旧写法时改写，并把结果落库 + 清渲染缓存。
func (s *Service) EnsureSecurityPatches(ctx context.Context) error {
	if s == nil || s.repo == nil {
		return nil
	}
	tmpl, err := s.repo.GetActive(ctx)
	if err != nil || tmpl == nil {
		return nil
	}
	newHTML, newJS, changed := PatchTemplateSecurity(tmpl.HTMLContent, tmpl.JSContent)
	// in-DOM shell 的自闭合自定义组件展开：HTML 解析器会把 `<el-alert ... />` 当成开始标签，
	// 把它后面的兄弟节点全部吞成子节点 → v-if 为假时「更多订阅」弹窗的格式卡片整块不渲染 ✗。
	if h1, c1 := PatchInDOMSelfClosingComponents(newHTML); c1 {
		newHTML = h1
		changed = true
	}
	// 导航「购买服务 / 我的账单」始终显示（历史版本按充值站点过滤的写法升级回透传）✓
	if h2, j2, c2 := PatchChargeNavMenu(newHTML, newJS); c2 {
		newHTML, newJS = h2, j2
		changed = true
	}
	// 存量 shell 补 chargeUiEnabled（hero「管理您的订阅与服务」充值/余额联动依赖）✓
	if h3, j3, c3 := PatchChargeUiEnabled(newHTML, newJS); c3 {
		newHTML, newJS = h3, j3
		changed = true
	}
	// 存量 shell 补 storeEnabled/visibleSubActions（订阅链接「购买套餐」随购买服务关闭而隐藏）✓
	if h4, j4, c4 := PatchSubStoreActionJS(newHTML, newJS); c4 {
		newHTML, newJS = h4, j4
		changed = true
	}
	// 升级 = 到期预约切换：跨套餐发 upgrade、认 scheduled 回调、订单状态补 scheduled ✓
	if h5, j5, c5 := PatchPlanUpgradeScheduledJS(newHTML, newJS); c5 {
		newHTML, newJS = h5, j5
		changed = true
	}
	// 客户端面：支付后轮询（C-1）/ 推荐提现（C-2）/ 订阅 IP 白名单管理（C-4）✓
	if h6, j6, c6 := PatchClientFeatures(newHTML, newJS); c6 {
		newHTML, newJS = h6, j6
		changed = true
	}
	// regions.auth 是真正渲染的登录卡 → config 里的区域源码也要补（自助解封卡 ✓）
	newConfig, cfgChanged := PatchRegionsSecurity(tmpl.Config)
	// 存量模板的 account_info 模块 HTML 里「充值按钮」旧条件也要迁移（默认关闭 ✓）
	if c, cc := PatchChargeButtonConfig(newConfig); cc {
		newConfig = c
		cfgChanged = true
	}
	// 存量模板 hero 的「管理您的订阅与服务」按钮补「充值/余额」联动条件 ✓
	if c, cc := PatchHeroButtonConfig(newConfig); cc {
		newConfig = c
		cfgChanged = true
	}
	// 存量模板导航「购买服务 / 我的账单」默认关闭（与 nav.logs 一致）✓
	if c, cc := PatchNavDefaultsOff(newConfig); cc {
		newConfig = c
		cfgChanged = true
	}
	// 存量模板订阅链接模块的动作按钮源升级（购买服务关闭时隐藏「购买套餐」）——
	// 仅在 shell 已具备 visibleSubActions 时才改写模块 HTML，避免引用未定义函数 ✗。
	if strings.Contains(newJS, subStoreActionJSPresence) {
		if c, cc := PatchSubStoreActionConfig(newConfig); cc {
			newConfig = c
			cfgChanged = true
		}
	}
	// 存量模板语言包：升级预约文案 + scheduled 状态键 ✓
	if c, cc := PatchPlanUpgradeScheduledConfig(newConfig); cc {
		newConfig = c
		cfgChanged = true
	}
	// 存量模板模块 HTML：推荐提现按钮 + 安全设置白名单列表（C-2/C-4）✓
	if c, cc := PatchClientFeaturesConfig(newConfig); cc {
		newConfig = c
		cfgChanged = true
	}
	if !changed && !cfgChanged {
		return nil
	}
	tmpl.HTMLContent = newHTML
	tmpl.JSContent = newJS
	tmpl.Config = newConfig
	if err := s.repo.Update(ctx, tmpl); err != nil {
		return err
	}
	s.InvalidateCache()
	log.Printf("[portal] patches applied to template %d (security / charge nav menu)", tmpl.ID)
	return nil
}
