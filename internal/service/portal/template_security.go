package portal

import (
	"context"
	"encoding/json"
	"log"
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

// 在线支付接口（充值站点）不可用时隐藏「购买服务 / 我的账单」导航 —— 存量模板同样需要迁移
// （门户模板存库 → 只改默认模板到不了已部署门户 ✗）。做法：导航渲染从 portalNavItems 改为按
// chargeSites 过滤的 navItems computed，并补齐 chargeSitesLoaded / fetchChargeSites / return。
// 每处独立判定、独立幂等 ✓（片段里**不能出现反引号**）。
const (
	oldNavSidebar = `v-for="nav in portalNavItems" :key="nav.id" :class="'sidebar-item'`
	newNavSidebar = `v-for="nav in navItems" :key="nav.id" :class="'sidebar-item'`
	oldNavBottom  = `v-for="nav in portalNavItems" :key="nav.id" :class="'nav-item'`
	newNavBottom  = `v-for="nav in navItems" :key="nav.id" :class="'nav-item'`

	navItemsAnchor   = `    var portalNavItems = ref([])`
	navItemsPresence = `var navItems = computed(function() {`
	navItemsSnippet  = `    var portalNavItems = ref([])
    // 在线支付接口（充值站点）不可用时隐藏「购买服务 / 我的账单」导航（无数据、无意义）
    var chargeSitesLoaded = ref(false)
    var navItems = computed(function() {
      var items = portalNavItems.value
      if (!chargeSitesLoaded.value || chargeSites.value.length > 0) return items
      return items.filter(function(n) { return n.target !== 'store' && n.target !== 'billing' })
    })`

	oldChargeSitesFetch = `        chargeSites.value = Array.isArray(res) ? res : []`
	newChargeSitesFetch = `        chargeSites.value = Array.isArray(res) ? res : []
        chargeSitesLoaded.value = true`
	chargeSitesLoadedFlag = `chargeSitesLoaded.value = true`
	oldNavInitFetch       = `        if (initHasPlanList || initHasCharge) fetchChargeSites()`
	newNavInitFetch       = `        fetchChargeSites()`
	oldNavReturn          = `      currentModules: currentModules, portalNavItems: portalNavItems, onNavClick: onNavClick,`
	newNavReturn          = `      currentModules: currentModules, portalNavItems: portalNavItems, navItems: navItems, onNavClick: onNavClick,`
)

// PatchChargeNavMenu 对存量门户模板打「无可用充值接口时隐藏购买服务/我的账单导航」补丁。
// 幂等：命中过 / 已是新版时原样返回 ✓。
func PatchChargeNavMenu(htmlContent, jsContent string) (newHTML, newJS string, changed bool) {
	newHTML, newJS = htmlContent, jsContent

	if strings.Contains(newHTML, oldNavSidebar) {
		newHTML = strings.Replace(newHTML, oldNavSidebar, newNavSidebar, 1)
		changed = true
	}
	if strings.Contains(newHTML, oldNavBottom) {
		newHTML = strings.Replace(newHTML, oldNavBottom, newNavBottom, 1)
		changed = true
	}

	// navItems computed + chargeSitesLoaded 声明
	if strings.Contains(newJS, navItemsAnchor) && !strings.Contains(newJS, navItemsPresence) {
		newJS = strings.Replace(newJS, navItemsAnchor, navItemsSnippet, 1)
		changed = true
	}
	// fetchChargeSites 成功后标记已加载
	if strings.Contains(newJS, oldChargeSitesFetch) && !strings.Contains(newJS, chargeSitesLoadedFlag) {
		newJS = strings.Replace(newJS, oldChargeSitesFetch, newChargeSitesFetch, 1)
		changed = true
	}
	// 进入门户即拉取充值站点（导航过滤依赖它）
	if strings.Contains(newJS, oldNavInitFetch) {
		newJS = strings.Replace(newJS, oldNavInitFetch, newNavInitFetch, 1)
		changed = true
	}
	// setup return 透传 navItems（旧片段是新片段前缀 → 必须同时查新片段防重复 ✓）
	if strings.Contains(newJS, oldNavReturn) && !strings.Contains(newJS, newNavReturn) {
		newJS = strings.Replace(newJS, oldNavReturn, newNavReturn, 1)
		changed = true
	}

	return newHTML, newJS, changed
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
	// 无可用充值接口（在线支付站点）时隐藏「购买服务 / 我的账单」导航 ✓
	if h2, j2, c2 := PatchChargeNavMenu(newHTML, newJS); c2 {
		newHTML, newJS = h2, j2
		changed = true
	}
	// regions.auth 是真正渲染的登录卡 → config 里的区域源码也要补（自助解封卡 ✓）
	newConfig, cfgChanged := PatchRegionsSecurity(tmpl.Config)
	// 存量模板的 account_info 模块 HTML 里「充值按钮」旧条件也要迁移（默认关闭 ✓）
	if c, cc := PatchChargeButtonConfig(newConfig); cc {
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
