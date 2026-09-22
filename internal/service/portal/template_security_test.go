package portal

import (
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
