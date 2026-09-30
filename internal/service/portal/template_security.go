package portal

import (
	"context"
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
	if !changed {
		return nil
	}
	tmpl.HTMLContent = newHTML
	tmpl.JSContent = newJS
	if err := s.repo.Update(ctx, tmpl); err != nil {
		return err
	}
	s.InvalidateCache()
	log.Printf("[portal] security patch applied to template %d (M7: 用户内容不再执行 HTML)", tmpl.ID)
	return nil
}
