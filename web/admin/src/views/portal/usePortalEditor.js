import { ref, reactive, computed, watch, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import {
  getActivePortal, updateActivePortal, resetPortal,
  getPortalCacheStatus, clearPortalCache, rebuildPortalCache, setPortalDevMode,
  getBuiltinPortalLang,
} from '../../api/portal'
import { useQueryState } from '../../composables/useQueryState'
import { CONTENT_EDITOR_TYPES, moduleTypeLabel, moduleGroups, createModule, genId } from './moduleDefs'
import { createDefaultNavigation, createDefaultModules } from './defaults'
import { buildApiGroups } from './apiDocs'

marked.setOptions({ breaks: true, gfm: true })

// Shared editor store for the portal builder. Created once in the root view
// and provided to all child panels via inject('portalEditor'). The returned
// reactive bag unwraps refs/computeds on access — always use `ed.xxx`,
// never destructure.
export function usePortalEditor() {
  const { t } = useI18n()

  // ---- UI state ----
  const loading = ref(false)
  const saving = ref(false)
  const editorMode = useQueryState('mode', 'builder')
  const settingsTab = useQueryState('stab', 'basic')
  const codeTab = useQueryState('ctab', 'html')
  const langSearch = ref('')
  const previewDevice = ref('desktop')
  const hasUnsavedChanges = ref(false)
  const registrationEnabled = ref(false)
  const portalPort = ref('')
  const navDialogVisible = ref(false)
  const activeApiNames = ref([0])

  const form = reactive({ name: '', description: '', html_content: '', css_content: '', js_content: '' })
  const langPack = ref({ default: 'zh-CN', locales: {} })
  const langLocale = ref('zh-CN')
  const langData = ref({})
  // 内置语言包（后端 locales/*.json 经 go:embed 提供）——用于缺失 key 检测 / 补齐
  const builtinLang = ref({ default: 'zh-CN', locales: {} })
  const config = reactive({
    site_name: 'CoreFusion', logo_url: '', primary_color: '#667eea', background_color: '#f0f2f5',
    card_background_color: '#ffffff', text_color: '#303133', accent_color: '#67c23a',
    font_family: 'system-ui, -apple-system, sans-serif',
    // 合规文档链接 ✓（Google OAuth 验证要求登录页展示 ✓）
    privacy_policy_url: '', terms_of_service_url: '',
    // 合规文档内容 ✓（按语言 markdown：{"zh-CN":"...","en":"..."}）
    privacy_policy_content: {}, terms_of_service_content: {},
    // 门户对外地址 ✓（只读，来自后端：门户配置了域名用门户域名，否则回退面板地址；不参与保存）
    portal_external_url: '', portal_page_path: '/p',
  })

  const navigation = ref([])
  const modules = ref([])
  const selectedNavId = ref('')
  const selectedModuleId = ref('')

  // Regions (editable system pages like auth/login)
  const regions = ref({})
  const selectedRegionType = ref('')
  const regionViewMode = useQueryState('rview', 'preview')

  const newNav = reactive({ label: '', icon: '', target: 'overview', href: '', enabled: true, open_external: false })

  const currentPageNavId = ref('')
  const centerViewMode = useQueryState('cview', 'preview')
  const freeformTab = useQueryState('ftab', 'html')
  const moduleCodeTab = useQueryState('mctab', 'html')

  const cacheStatus = ref({ cached: false, dev_mode: false, hit_count: 0, miss_count: 0 })

  // ---- Computed ----
  const selectedNav = computed(() => navigation.value.find((n) => n.id === selectedNavId.value))
  const selectedModule = computed(() => modules.value.find((m) => m.id === selectedModuleId.value))
  const currentNav = computed(() => navigation.value.find((n) => n.id === currentPageNavId.value))
  const currentPageTarget = computed(() => currentNav.value?.target || '')

  const hasContentEditor = computed(() => {
    if (!selectedModule.value) return false
    return CONTENT_EDITOR_TYPES.includes(selectedModule.value.type)
  })

  const currentPageModules = computed(() => {
    if (!currentPageTarget.value) return []
    return modules.value.filter((m) => (m.page || 'overview') === currentPageTarget.value)
  })

  const selectedRegion = computed(() => {
    if (!selectedRegionType.value) return null
    return regions.value[selectedRegionType.value] || null
  })

  // 可视化区 iframe 基址：固定用当前面板地址（面板自身提供 /p 门户页），
  // **不走门户对外域名** —— 门户域名未就绪/不可达时 iframe 会「拒绝连接」。
  const portalLocalBase = computed(() => window.location.origin + '/p')

  // 对外基址（预览按钮/新标签打开）：配了门户域名就用域名（含挂载路径），否则回退本地。
  const portalPublicBase = computed(() => {
    const ext = (config.portal_external_url || '').replace(/\/$/, '')
    if (!ext) return portalLocalBase.value
    const path = config.portal_page_path === undefined || config.portal_page_path === null
      ? '/p'
      : config.portal_page_path
    return ext + path
  })

  const previewUrl = computed(() => portalLocalBase.value + '?preview=1&screen=auth')

  // Preview iframe control — key forces reload when config changes
  const previewKey = ref(0)
  const pagePreviewUrl = computed(() => {
    const base = portalLocalBase.value + '?preview=1'
    // Append current page target so the preview shows the right page
    if (currentPageTarget.value) {
      return base + '&page=' + encodeURIComponent(currentPageTarget.value)
    }
    return base
  })

  function refreshPreview() {
    previewKey.value++
  }

  function onPreviewLoad() {
    // Preview iframe loaded — could sync scroll position or highlight selected module
  }

  const apiGroups = computed(() => buildApiGroups(t))
  const addModuleGroups = computed(() => moduleGroups())

  // ---- Normalizers ----
  // 模块代码模型（统一）：data.html / data.css / data.js 均为字符串。
  // 不再有 style.custom_css（已移除）。
  function ensureModuleCode(mod) {
    if (!mod) return
    if (!mod.data) mod.data = {}
    if (typeof mod.data.html !== 'string') mod.data.html = ''
    if (typeof mod.data.css !== 'string') mod.data.css = ''
    if (typeof mod.data.js !== 'string') mod.data.js = ''
    if (!mod.style) mod.style = {}
    if (typeof mod.style.custom_class !== 'string') mod.style.custom_class = ''
  }

  function ensureNoticeData(mod) {
    if (!mod || mod.type !== 'notice') return
    if (!mod.data) mod.data = {}
    const d = mod.data
    if (typeof d.display_mode !== 'string') d.display_mode = 'inline'
    if (typeof d.popup_trigger !== 'string') d.popup_trigger = 'on_load'
    if (typeof d.popup_dismiss !== 'string') d.popup_dismiss = 'once'
    if (typeof d.popup_width !== 'string') d.popup_width = '480px'
    if (typeof d.popup_button_text !== 'string') d.popup_button_text = t('portal.api.ex.defaultPopupButton')
    if (typeof d.popup_show_close !== 'boolean') d.popup_show_close = true
    if (typeof d.popup_link_text !== 'string') d.popup_link_text = ''
    if (typeof d.popup_link_url !== 'string') d.popup_link_url = ''
    if (typeof d.popup_link_external !== 'boolean') d.popup_link_external = true
  }

  const normalizeModule = (m) => {
    const mod = { ...m, page: m.page || 'overview' }
    ensureModuleCode(mod)
    ensureNoticeData(mod)
    return mod
  }

  function normalizeLangPack(raw) {
    if (raw && typeof raw === 'object') {
      if (raw.locales && typeof raw.locales === 'object') return raw
      // Legacy flat format — wrap it
      return { default: 'zh-CN', locales: { 'zh-CN': raw } }
    }
    return { default: 'zh-CN', locales: {} }
  }

  // ---- Helpers ----
  function copyText(text) {
    navigator.clipboard.writeText(text)
      .then(() => ElMessage.success(t('portal.copiedToClipboard')))
      .catch(() => ElMessage.error(t('portal.copyFailed')))
  }

  function renderMd(md) {
    if (!md) return ''
    const raw = marked.parse(md)
    return DOMPurify.sanitize(raw, { ADD_TAGS: ['input'], ADD_ATTR: ['type', 'checked', 'disabled'] })
  }

  function sanitizeHtml(html) {
    return DOMPurify.sanitize(html)
  }

  const typeLabel = (type) => moduleTypeLabel(type, t)

  function currentPageModuleCount(target) {
    return modules.value.filter((m) => (m.page || 'overview') === target).length
  }

  function onCodeChange() {
    hasUnsavedChanges.value = true
  }

  // ---- Selection ----
  function selectNav(id) {
    currentPageNavId.value = id
    selectedNavId.value = id
    selectedModuleId.value = ''
    selectedRegionType.value = ''
  }

  function selectModule(id) {
    selectedModuleId.value = id
    selectedNavId.value = ''
    selectedRegionType.value = ''
    const mod = modules.value.find((m) => m.id === id)
    if (mod) {
      ensureModuleCode(mod)
      ensureNoticeData(mod)
    }
    autoCenterViewMode()
  }

  function selectRegion(type) {
    // Lazily initialize the region so the props panel never binds into
    // undefined nested fields (backend payloads may omit data/html/css/js)
    const existing = regions.value[type] || {}
    regions.value[type] = {
      html: existing.html || '',
      css: existing.css || '',
      js: existing.js || '',
      data: existing.data || {},
    }
    selectedRegionType.value = type
    selectedNavId.value = ''
    selectedModuleId.value = ''
    currentPageNavId.value = ''
    regionViewMode.value = 'preview'
  }

  function autoCenterViewMode() {
    centerViewMode.value = hasContentEditor.value ? 'editor' : 'preview'
  }

  watch(centerViewMode, (val) => {
    // 旧的 'css' 视图已合并进「模块代码」（右侧属性面板 / 全屏）
    if (val === 'css') centerViewMode.value = 'preview'
  })

  // ---- Navigation CRUD ----
  function openNavDialog() {
    Object.assign(newNav, { label: '', icon: '', target: '', href: '', enabled: true, open_external: false })
    navDialogVisible.value = true
  }

  function confirmAddNav() {
    if (!newNav.label) { ElMessage.error(t('portal.pleaseEnterName')); return }
    const target = newNav.target || newNav.label.trim().toLowerCase().replace(/[^a-z0-9一-龥]+/g, '-').replace(/^-+|-+$/g, '') || 'page-' + Date.now()
    const navId = genId('nav')
    navigation.value.push({ ...newNav, id: navId, target })
    navDialogVisible.value = false
    selectNav(navId)
    hasUnsavedChanges.value = true
  }

  function removeNav(idx) {
    ElMessageBox.confirm(t('portal.confirmDeleteNav'), t('common.confirm'), { type: 'warning' }).then(() => {
      const removedId = navigation.value[idx].id
      if (selectedNavId.value === removedId) selectedNavId.value = ''
      if (currentPageNavId.value === removedId) {
        const next = navigation.value[idx + 1] || navigation.value[idx - 1]
        currentPageNavId.value = next ? next.id : ''
      }
      navigation.value.splice(idx, 1)
      hasUnsavedChanges.value = true
    }).catch(() => {})
  }

  function moveItem(list, idx, dir) {
    const ni = idx + dir
    if (ni < 0 || ni >= list.length) return
    const tmp = list[idx]; list[idx] = list[ni]; list[ni] = tmp
    hasUnsavedChanges.value = true
  }

  // ---- Module CRUD ----
  function addModule(type) {
    const mod = createModule(type, currentPageTarget.value || 'overview', t)
    modules.value.push(mod)
    selectModule(mod.id)
    hasUnsavedChanges.value = true
  }

  function duplicateModuleByIdx(pageIdx) {
    const mod = currentPageModules.value[pageIdx]
    if (!mod) return
    const realIdx = modules.value.indexOf(mod)
    const copy = JSON.parse(JSON.stringify(mod))
    copy.id = genId('mod')
    copy.title = (mod.title || '') + t('portal.copySuffix')
    modules.value.splice(realIdx + 1, 0, copy)
    hasUnsavedChanges.value = true
  }

  function removeModuleByIdx(pageIdx) {
    const mod = currentPageModules.value[pageIdx]
    if (!mod) return
    ElMessageBox.confirm(t('portal.confirmDeleteModule'), t('common.confirm'), { type: 'warning' }).then(() => {
      const realIdx = modules.value.indexOf(mod)
      if (realIdx >= 0) {
        if (selectedModuleId.value === mod.id) selectedModuleId.value = ''
        modules.value.splice(realIdx, 1)
        hasUnsavedChanges.value = true
      }
    }).catch(() => {})
  }

  function movePageModule(pageIdx, dir) {
    const mod = currentPageModules.value[pageIdx]
    if (!mod) return
    const realIdx = modules.value.indexOf(mod)
    const samePageIndices = modules.value.map((m, i) => ((m.page || 'overview') === mod.page ? i : -1)).filter((i) => i >= 0)
    const posInPage = samePageIndices.indexOf(realIdx)
    const targetPos = posInPage + dir
    if (targetPos < 0 || targetPos >= samePageIndices.length) return
    const targetRealIdx = samePageIndices[targetPos]
    const tmp = modules.value[realIdx]
    modules.value[realIdx] = modules.value[targetRealIdx]
    modules.value[targetRealIdx] = tmp
    hasUnsavedChanges.value = true
  }

  // ---- Load / save ----
  const applyData = (d) => {
    if (!d) return
    form.name = d.name || ''
    form.description = d.description || ''
    form.html_content = d.html_content || ''
    form.css_content = d.css_content || ''
    form.js_content = d.js_content || ''
    registrationEnabled.value = d.registration_enabled || false
    portalPort.value = d.portal_port || ''

    // Theme fields (parsed server-side from the config JSON)
    config.site_name = d.site_name || 'CoreFusion'
    config.logo_url = d.logo_url || ''
    config.primary_color = d.primary_color || '#667eea'
    config.background_color = d.background_color || '#f0f2f5'
    config.card_background_color = d.card_background_color || '#ffffff'
    config.text_color = d.text_color || '#303133'
    config.accent_color = d.accent_color || '#67c23a'
    config.font_family = d.font_family || "-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif"
    config.privacy_policy_url = d.privacy_policy_url || ''
    config.terms_of_service_url = d.terms_of_service_url || ''
    config.privacy_policy_content = d.privacy_policy_content || {}
    config.terms_of_service_content = d.terms_of_service_content || {}
    // 门户对外地址（只读）：门户配置了域名用门户域名，否则前端回退当前 origin
    config.portal_external_url = d.portal_external_url || ''
    // portal_page_path：独立门户端口为 ""（根路径）—— 不能用 || 回退，否则 "" 会变成 "/p"
    config.portal_page_path = d.portal_page_path === undefined || d.portal_page_path === null
      ? '/p'
      : d.portal_page_path

    // Config string is only a fallback source for legacy payloads
    let cfgObj = null
    if (d.config && typeof d.config === 'string') {
      try { cfgObj = JSON.parse(d.config) } catch { cfgObj = null }
    }

    let nav = Array.isArray(d.navigation) && d.navigation.length > 0 ? d.navigation : null
    let mods = Array.isArray(d.modules) && d.modules.length > 0 ? d.modules : null
    if (cfgObj) {
      if (!nav && Array.isArray(cfgObj.navigation) && cfgObj.navigation.length > 0) nav = cfgObj.navigation
      if (!mods && Array.isArray(cfgObj.modules) && cfgObj.modules.length > 0) mods = cfgObj.modules
    }
    navigation.value = nav || createDefaultNavigation(t)
    modules.value = (mods || createDefaultModules(t)).map(normalizeModule)

    // Lang pack
    const rawLang = (d.lang && typeof d.lang === 'object') ? d.lang : (cfgObj && cfgObj.lang) || null
    langPack.value = normalizeLangPack(rawLang)
    langLocale.value = langPack.value.default || 'zh-CN'
    if (!langPack.value.locales[langLocale.value]) {
      langPack.value.locales[langLocale.value] = {}
    }
    langData.value = langPack.value.locales[langLocale.value]

    // Regions
    if (d.regions && typeof d.regions === 'object') {
      regions.value = d.regions
    } else if (cfgObj && cfgObj.regions && typeof cfgObj.regions === 'object') {
      regions.value = cfgObj.regions
    } else {
      regions.value = {}
    }

    // Auto-select first nav if none selected
    if (navigation.value.length > 0) {
      if (!currentPageNavId.value) currentPageNavId.value = navigation.value[0].id
      if (!selectedNavId.value) selectedNavId.value = navigation.value[0].id
    }
    hasUnsavedChanges.value = false
  }

  const fetchConfig = async () => {
    loading.value = true
    try {
      const data = await getActivePortal()
      applyData(data)
    } catch {
      ElMessage.error(t('portal.loadConfigFailed'))
    } finally {
      loading.value = false
    }
  }

  const handleSave = async () => {
    saving.value = true
    try {
      const data = {
        name: form.name, description: form.description,
        html_content: form.html_content, css_content: form.css_content, js_content: form.js_content,
        navigation: navigation.value,
        modules: modules.value,
        theme: {
          site_name: config.site_name,
          logo_url: config.logo_url,
          primary_color: config.primary_color,
          background_color: config.background_color,
          card_background_color: config.card_background_color,
          text_color: config.text_color,
          accent_color: config.accent_color,
          font_family: config.font_family,
        },
        lang: Object.keys(langPack.value.locales).length > 0 ? langPack.value : undefined,
        registration_enabled: registrationEnabled.value,
        portal_port: portalPort.value,
        // 合规文档链接/内容 ✓（登录卡片展示 ✓）
        privacy_policy_url: config.privacy_policy_url || '',
        terms_of_service_url: config.terms_of_service_url || '',
        privacy_policy_content: config.privacy_policy_content || {},
        terms_of_service_content: config.terms_of_service_content || {},
        regions: Object.keys(regions.value).length > 0 ? regions.value : undefined,
      }
      const res = await updateActivePortal(data)
      applyData(res)
      // Warm the render cache (best-effort; the runtime also fetches fresh config)
      try { await rebuildPortalCache() } catch { /* best-effort */ }
      ElMessage.success(t('portal.saveSuccess'))
      hasUnsavedChanges.value = false
    } catch (e) {
      ElMessage.error(e.response?.data?.message || t('portal.saveFailed'))
    } finally {
      saving.value = false
    }
  }

  const handlePreview = () => { window.open(portalPublicBase.value, '_blank') }

  const handleReset = async () => {
    try {
      await ElMessageBox.confirm(t('portal.resetConfirmMsg'), t('portal.resetConfirmTitle'), { type: 'warning' })
      const res = await resetPortal()
      applyData(res)
      try { await rebuildPortalCache() } catch { /* best-effort */ }
      ElMessage.success(t('portal.resetSuccess'))
    } catch (e) {
      if (e !== 'cancel') ElMessage.error(t('portal.resetFailed'))
    }
  }

  // ---- Lang editor ----
  const langAvailableLocales = computed(() => {
    return Object.keys(langPack.value.locales).map((code) => {
      const meta = langPack.value.locales[code]?._meta || {}
      return { code, label: meta.label || code, flag: meta.flag || '🌐' }
    })
  })

  const filteredLangEntries = computed(() => {
    const entries = []
    const search = langSearch.value.toLowerCase().trim()
    for (const key of Object.keys(langData.value)) {
      if (key.startsWith('_')) continue
      const val = String(langData.value[key] || '')
      const prefix = key.split('.')[0] || 'other'
      if (!search || key.toLowerCase().includes(search) || val.toLowerCase().includes(search)) {
        entries.push({ key, val, prefix })
      }
    }
    entries.sort((a, b) => a.key.localeCompare(b.key))
    return entries
  })

  // 当前语言对应的内置语言包（精确 → 前缀 → 默认兜底）
  const builtinLangForCurrent = computed(() => {
    const locales = builtinLang.value.locales || {}
    const code = langLocale.value
    if (locales[code]) return locales[code]
    const prefix = String(code).split('-')[0]
    for (const c of Object.keys(locales)) {
      if (String(c).split('-')[0] === prefix) return locales[c]
    }
    return locales[builtinLang.value.default] || {}
  })

  // 缺失 key：内置语言包里有，但当前语言未填 / 为空
  const missingLangKeys = computed(() => {
    const pack = builtinLangForCurrent.value || {}
    const cur = langData.value || {}
    return Object.keys(pack).filter((k) => !k.startsWith('_') && (cur[k] === undefined || cur[k] === null || cur[k] === ''))
  })

  async function fetchBuiltinLang() {
    try {
      const data = await getBuiltinPortalLang()
      if (data && data.locales) builtinLang.value = data
    } catch { /* best-effort；失败则不显示缺失检测 */ }
  }

  function fillMissingLangKeys() {
    const pack = builtinLangForCurrent.value || {}
    if (!pack || Object.keys(pack).length === 0) {
      ElMessage.warning(t('portal.builtinLangUnavailable'))
      return
    }
    const keys = missingLangKeys.value
    if (keys.length === 0) {
      ElMessage.info(t('portal.noMissingLangKeys'))
      return
    }
    const next = { ...langData.value }
    keys.forEach((k) => { next[k] = pack[k] })
    langData.value = next
    langPack.value.locales[langLocale.value] = next
    hasUnsavedChanges.value = true
    ElMessage.success(t('portal.filledMissingLangKeys', { count: keys.length }))
  }

  function switchLangLocale(code) {
    langPack.value.locales[langLocale.value] = langData.value
    langLocale.value = code
    if (!langPack.value.locales[code]) langPack.value.locales[code] = {}
    langData.value = langPack.value.locales[code]
    hasUnsavedChanges.value = true
  }

  function addLocale() {
    ElMessageBox.prompt(t('portal.enterLangCode'), t('portal.addLocaleTitle'), {
      confirmButtonText: t('common.create'),
      cancelButtonText: t('common.cancel'),
      inputPattern: /^[a-z]{2}(-[A-Z]{2})?$/,
      inputErrorMessage: t('portal.langCodeFormat'),
    }).then(({ value }) => {
      if (langPack.value.locales[value]) { ElMessage.warning(t('portal.langAlreadyExists')); return }
      const newLocale = {}
      for (const key of Object.keys(langData.value)) {
        if (key === '_meta') continue
        newLocale[key] = ''
      }
      newLocale._meta = { language: value, label: value, flag: '🌐' }
      langPack.value.locales[value] = newLocale
      langPack.value = { ...langPack.value }
      ElMessage.success(t('portal.langAdded', { code: value }))
    }).catch(() => {})
  }

  function removeLocale(code) {
    if (code === langPack.value.default) { ElMessage.warning(t('portal.cannotDeleteDefaultLang')); return }
    ElMessageBox.confirm(t('portal.confirmDeleteLang', { code }), t('common.confirm'), { type: 'warning' }).then(() => {
      delete langPack.value.locales[code]
      langPack.value = { ...langPack.value }
      if (langLocale.value === code) {
        langLocale.value = langPack.value.default || 'zh-CN'
        langData.value = langPack.value.locales[langLocale.value] || {}
      }
      hasUnsavedChanges.value = true
      ElMessage.success(t('portal.deleted'))
    }).catch(() => {})
  }

  function addLangKey() {
    ElMessageBox.prompt(t('portal.enterLangKey'), t('portal.addLangEntryTitle'), {
      confirmButtonText: t('common.create'),
      cancelButtonText: t('common.cancel'),
      inputPattern: /^[a-z_]+\.[a-z_]+$/,
      inputErrorMessage: t('portal.langKeyFormat'),
    }).then(({ value }) => {
      if (langData.value[value] !== undefined) {
        ElMessage.warning(t('portal.langKeyExists'))
        return
      }
      langData.value[value] = ''
      langData.value = { ...langData.value }
      hasUnsavedChanges.value = true
      ElMessage.success(t('portal.langKeyAdded'))
    }).catch(() => {})
  }

  function deleteLangKey(key) {
    ElMessageBox.confirm(t('portal.confirmDeleteLangKey', { key }), t('common.confirm'), { type: 'warning' }).then(() => {
      delete langData.value[key]
      langData.value = { ...langData.value }
      hasUnsavedChanges.value = true
      ElMessage.success(t('portal.deleted'))
    }).catch(() => {})
  }

  function exportLang() {
    langPack.value.locales[langLocale.value] = langData.value
    const json = JSON.stringify(langPack.value, null, 2)
    const blob = new Blob([json], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = 'portal-lang.json'
    a.click()
    URL.revokeObjectURL(url)
  }

  function importLang(file) {
    const reader = new FileReader()
    reader.onload = (e) => {
      try {
        const data = JSON.parse(e.target.result)
        if (data && data.locales && typeof data.locales === 'object') {
          langPack.value = data
          langLocale.value = data.default || Object.keys(data.locales)[0] || 'zh-CN'
          langData.value = langPack.value.locales[langLocale.value] || {}
          hasUnsavedChanges.value = true
          ElMessage.success(t('portal.langPackImported'))
        } else if (typeof data === 'object' && data !== null) {
          langData.value = { ...langData.value, ...data }
          langPack.value.locales[langLocale.value] = langData.value
          hasUnsavedChanges.value = true
          ElMessage.success(t('portal.langImportedMerged'))
        } else {
          ElMessage.error(t('portal.invalidFileFormat'))
        }
      } catch {
        ElMessage.error(t('portal.jsonParseFailed'))
      }
    }
    reader.readAsText(file)
    return false
  }

  // ---- Render cache management ----
  async function fetchCacheStatus() {
    try {
      const data = await getPortalCacheStatus()
      cacheStatus.value = data
    } catch { /* ignore */ }
  }

  async function handleCacheCommand(cmd) {
    if (cmd === 'status') {
      await fetchCacheStatus()
      const s = cacheStatus.value
      const msg = t('portal.cacheStatusCached', { status: s.cached ? t('portal.cachedStatus') : t('portal.notCachedStatus') }) + '\n' +
        t('portal.cacheModeLabel') + '：' + (s.dev_mode ? t('portal.devModeNoCache') : t('portal.prodModeCached')) + '\n' +
        t('portal.hitCount') + '：' + s.hit_count + '\n' +
        t('portal.missCount') + '：' + s.miss_count +
        (s.built_at ? '\n' + t('portal.cacheTime') + '：' + s.built_at : '')
      ElMessageBox.alert(msg, t('portal.cacheStatusTitle'), { confirmButtonText: t('common.close') })
    } else if (cmd === 'rebuild') {
      try {
        await rebuildPortalCache()
        await fetchCacheStatus()
        ElMessage.success(t('portal.cacheRebuilt'))
      } catch { ElMessage.error(t('portal.rebuildCacheFailed')) }
    } else if (cmd === 'clear') {
      try {
        await clearPortalCache()
        await fetchCacheStatus()
        ElMessage.success(t('portal.cacheCleared'))
      } catch { ElMessage.error(t('portal.clearCacheFailed')) }
    } else if (cmd === 'dev') {
      const newMode = !cacheStatus.value.dev_mode
      try {
        await setPortalDevMode(newMode)
        await fetchCacheStatus()
        ElMessage.success(newMode ? t('portal.switchedToDevMode') : t('portal.switchedToProdMode'))
      } catch { ElMessage.error(t('portal.switchModeFailed')) }
    }
  }

  onMounted(() => { fetchConfig(); fetchCacheStatus(); fetchBuiltinLang() })

  // The reactive bag unwraps refs on access — children use ed.xxx everywhere.
  return reactive({
    // UI state
    loading, saving, editorMode, settingsTab, codeTab, langSearch, previewDevice,
    hasUnsavedChanges, registrationEnabled, portalPort, navDialogVisible, activeApiNames,
    form, langPack, langLocale, langData, config,
    navigation, modules, selectedNavId, selectedModuleId,
    regions, selectedRegionType, regionViewMode, newNav,
    currentPageNavId, centerViewMode, freeformTab, moduleCodeTab, cacheStatus,
    // Computed
    selectedNav, selectedModule, currentNav, currentPageTarget, hasContentEditor,
    currentPageModules, selectedRegion, previewUrl,
    previewKey, pagePreviewUrl, refreshPreview, onPreviewLoad,
    apiGroups, addModuleGroups,
    langAvailableLocales, filteredLangEntries,
    // Helpers
    copyText, renderMd, sanitizeHtml, typeLabel, currentPageModuleCount,
    onCodeChange, ensureModuleCode,
    // Selection
    selectNav, selectModule, selectRegion,
    // Nav & modules
    openNavDialog, confirmAddNav, removeNav, moveItem,
    addModule, duplicateModuleByIdx, removeModuleByIdx, movePageModule,
    // Load/save
    fetchConfig, handleSave, handlePreview, handleReset,
    // Lang editor
    builtinLang, missingLangKeys, fillMissingLangKeys, fetchBuiltinLang,
    switchLangLocale, addLocale, removeLocale, addLangKey, deleteLangKey, exportLang, importLang,
    // Cache
    fetchCacheStatus, handleCacheCommand,
  })
}
