// Module type registry — single source of truth for the portal builder.
// Every module type allowed by the backend (internal/service/portal/validate.go)
// must be registered here so the add-menu, property forms and default values
// can never drift out of sync.
//
// 模块代码模型（统一）：每个模块的 HTML/CSS/JS 都存在 `data.html` / `data.css` /
// `data.js`；不再使用 `style.custom_css`（已移除）。

export const MODULE_CATEGORIES = [
  { key: 'content', labelKey: 'portal.catContent' },
  { key: 'account', labelKey: 'portal.catAccount' },
  { key: 'business', labelKey: 'portal.catBusiness' },
  { key: 'system', labelKey: 'portal.catSystem' },
]

// Module types that have a center-column content editor
export const CONTENT_EDITOR_TYPES = ['markdown', 'rich_text', 'notice', 'custom_html', 'freeform']

// Each def: icon + i18n label key + category + factory producing default
// data/style. Factories receive vue-i18n's t() so seeds are localized.
export const MODULE_DEFS = {
  // --- Content ---
  hero: {
    icon: '🚀', labelKey: 'portal.modHero', category: 'content',
    defaults: (t) => ({
      data: { eyebrow: '', heading: t('portal.dft.hero.heading'), description: '', button_text: '', button_target: 'subscription', css: '', js: '' },
      style: { variant: 'primary', align: 'left' },
    }),
  },
  markdown: {
    icon: '📝', labelKey: 'portal.modMarkdown', category: 'content',
    defaults: (t) => ({ data: { markdown: t('portal.dft.markdown'), css: '', js: '' }, style: { align: 'left' } }),
  },
  rich_text: {
    icon: '🖊️', labelKey: 'portal.modRichText', category: 'content',
    defaults: (t) => ({ data: { html: t('portal.dft.richText'), css: '', js: '' }, style: { align: 'left' } }),
  },
  notice: {
    icon: '📢', labelKey: 'portal.modNotice', category: 'content',
    defaults: (t) => ({
      data: {
        title: t('portal.dft.notice.title'), markdown: t('portal.dft.notice.markdown'),
        display_mode: 'inline', popup_trigger: 'on_load', popup_dismiss: 'once', popup_width: '480px',
        popup_button_text: t('portal.api.ex.defaultPopupButton'), popup_show_close: true,
        popup_link_text: '', popup_link_url: '', popup_link_external: true,
        css: '', js: '',
      },
      style: { variant: 'info' },
    }),
  },
  custom_html: {
    icon: '🔧', labelKey: 'portal.modCustomHtml', category: 'content',
    defaults: (t) => ({ data: { html: t('portal.dft.customHtml'), css: '', js: '' }, style: {} }),
  },
  freeform: {
    icon: '🧩', labelKey: 'portal.modFreeform', category: 'content',
    defaults: (t) => ({
      data: {
        html: t('portal.dft.freeform.html'),
        css: '.my-module {\n  padding: 20px;\n  border-radius: 12px;\n  background: var(--accent-light);\n}\n.my-module h3 {\n  color: var(--accent);\n  margin-bottom: 8px;\n}\n.my-module button {\n  padding: 6px 16px;\n  border: none;\n  border-radius: 6px;\n  background: var(--accent);\n  color: #fff;\n  cursor: pointer;\n}',
        js: t('portal.dft.freeform.js'),
      },
      style: { custom_class: '' },
    }),
  },
  feature_cards: {
    icon: '⭐', labelKey: 'portal.modFeatureCards', category: 'content',
    defaults: (t) => ({
      data: { cards: [{ icon: '⭐', title: t('portal.dft.featureCards.title'), description: t('portal.dft.featureCards.desc') }], css: '', js: '' },
      style: {},
    }),
  },
  tutorial_cards: {
    icon: '📚', labelKey: 'portal.modTutorialCards', category: 'content',
    defaults: () => ({ data: { css: '', js: '' }, style: {} }),
  },
  article_list: {
    icon: '📖', labelKey: 'portal.modArticleList', category: 'content',
    defaults: () => ({ data: { show_category_filter: true, default_category: '', css: '', js: '' }, style: {} }),
  },
  // --- Account & subscription ---
  account_info: {
    icon: '👤', labelKey: 'portal.modAccountInfo', category: 'account',
    defaults: () => ({ data: { css: '', js: '' }, style: {} }),
  },
  traffic_ring: {
    icon: '📊', labelKey: 'portal.modTrafficRing', category: 'account',
    defaults: () => ({ data: { css: '', js: '' }, style: {} }),
  },
  traffic_history: {
    icon: '📈', labelKey: 'portal.modTrafficHistory', category: 'account',
    defaults: () => ({
      data: {
        months: 6,
        html: `<div class="th-card"><div class="th-head"><span class="th-icon">📈</span><span class="th-title">{{ t(mod.title) }}</span></div><div v-if="loading" class="th-state">{{ t('traffic_history.loading') }}</div><div v-else-if="error" class="th-state th-error">{{ error }}</div><div v-else-if="history.length === 0" class="th-state">{{ t('traffic_history.empty') }}</div><div v-else class="th-list"><div class="th-row" v-for="item in history" :key="item.period_start"><div class="th-period">{{ fmtMonth(item.period_start) }}</div><div class="th-bar-wrap"><div class="th-bar" :style="{ width: barWidth(item) }"></div></div><div class="th-bytes">{{ formatBytes(item.total_bytes) }}</div></div></div></div>`,
        css: `.th-card{padding:16px}.th-head{display:flex;align-items:center;gap:8px;margin-bottom:12px}.th-icon{font-size:18px}.th-title{font-weight:600;font-size:15px}.th-state{color:var(--text-muted,#999);font-size:13px;padding:8px 0}.th-error{color:#f56c6c}.th-list{display:flex;flex-direction:column;gap:8px}.th-row{display:flex;align-items:center;gap:10px;font-size:13px}.th-period{width:56px;color:var(--text-muted,#999);flex:none}.th-bar-wrap{flex:1;height:8px;background:var(--bg-muted,#eee);border-radius:4px;overflow:hidden}.th-bar{height:100%;background:var(--accent,#409eff);border-radius:4px;transition:width .3s}.th-bytes{width:72px;text-align:right;flex:none;font-variant-numeric:tabular-nums}`,
        js: `const { ref, computed } = ctx; const history = ref([]); const loading = ref(true); const error = ref(''); const months = (ctx.mod && ctx.mod.data && ctx.mod.data.months) || 6; ctx.api.get('/traffic-history', { months: months }).then(function (res) { history.value = (res && res.history) || []; }).catch(function (e) { error.value = (e && (e.message || e.error)) || t('traffic_history.error'); }).finally(function () { loading.value = false; }); const maxBytes = computed(function () { let m = 0; history.value.forEach(function (h) { if ((h.total_bytes || 0) > m) m = h.total_bytes; }); return m || 1; }); const barWidth = function (item) { return Math.max(6, Math.round(((item.total_bytes || 0) / maxBytes.value) * 100)) + '%'; }; const fmtMonth = function (s) { if (!s) return ''; return ('' + s).slice(0, 7); }; return { history: history, loading: loading, error: error, months: months, barWidth: barWidth, fmtMonth: fmtMonth, formatBytes: ctx.formatBytes, t: ctx.t };`,
      },
      style: {},
    }),
  },
  subscription_detail: {
    icon: '🧾', labelKey: 'portal.modSubDetail', category: 'account',
    defaults: (t) => ({
      data: { description: '', copy_text: t('portal.seed.common.copy'), qr_text: t('portal.seed.common.qr'), show_status: true, show_rotate: true, css: '', js: '' },
      style: {},
    }),
  },
  client_download: {
    icon: '💻', labelKey: 'portal.modClientDownload', category: 'account',
    defaults: () => ({ data: { css: '', js: '' }, style: {} }),
  },
  streaming_status: {
    icon: '🎬', labelKey: 'portal.modStreamingStatus', category: 'account',
    defaults: () => ({ data: { css: '', js: '' }, style: {} }),
  },
  shared_accounts: {
    icon: '🔑', labelKey: 'portal.modSharedAccounts', category: 'account',
    defaults: (t) => ({
      data: { description: t('portal.seed.mod.shared.desc'), apply_btn_text: t('portal.seed.mod.shared.applyBtn'), repair_btn_text: t('portal.seed.mod.shared.repairBtn'), css: '', js: '' },
      style: {},
    }),
  },
  // --- Business ---
  plan_list: {
    icon: '📦', labelKey: 'portal.modPlanList', category: 'business',
    defaults: (t) => ({
      data: { show_price: true, show_features: true, show_quota: true, button_text: t('portal.seed.common.buy'), buy_text: t('portal.seed.common.buyNow'), renew_text: t('portal.seed.common.renew'), columns: 2, css: '', js: '' },
      style: {},
    }),
  },
  order_create: {
    icon: '🛒', labelKey: 'portal.modOrderCreate', category: 'business',
    defaults: (t) => ({ data: { description: t('portal.dft.orderCreate.desc'), css: '', js: '' }, style: {} }),
  },
  charge_panel: {
    icon: '💳', labelKey: 'portal.modChargePanel', category: 'business',
    defaults: (t) => ({
      data: { description: t('portal.seed.mod.charge.desc'), show_orders: true, orders_limit: 5, card_placeholder: t('portal.seed.mod.charge.cardPlaceholder'), card_btn_text: t('portal.seed.mod.charge.cardBtn'), css: '', js: '' },
      style: {},
    }),
  },
  balance_display: {
    icon: '💰', labelKey: 'portal.modBalanceDisplay', category: 'business',
    defaults: () => ({ data: { show_transactions: true, tx_limit: 10, css: '', js: '' }, style: {} }),
  },
  order_list: {
    icon: '🧾', labelKey: 'portal.modOrderList', category: 'business',
    defaults: () => ({ data: { css: '', js: '' }, style: {} }),
  },
  traffic_pack_list: {
    icon: '📦', labelKey: 'portal.modTrafficPackList', category: 'business',
    defaults: (t) => ({
      data: { notice_text: '', button_text: t('portal.seed.common.buy'), html: '', css: '', js: 'return {}' },
      style: {},
    }),
  },
  referral: {
    icon: '🎁', labelKey: 'portal.modReferral', category: 'business',
    defaults: (t) => ({
      data: { description: t('portal.seed.mod.referral.desc'), copy_btn_text: t('portal.seed.mod.referral.copyBtn'), show_records: true, records_limit: 10, css: '', js: '' },
      style: {},
    }),
  },
  // --- System ---
  node_list: {
    icon: '📋', labelKey: 'portal.modNodeList', category: 'system',
    defaults: () => ({ data: { css: '', js: '' }, style: {} }),
  },
  ticket_list: {
    icon: '🎫', labelKey: 'portal.modTicketList', category: 'system',
    defaults: (t) => ({ data: { new_btn_text: t('portal.seed.mod.tickets.newBtn'), show_categories: true, css: '', js: '' }, style: {} }),
  },
  security_settings: {
    icon: '🔒', labelKey: 'portal.modSecuritySettings', category: 'system',
    defaults: () => ({ data: { description: '', show_ip_whitelist: true, show_password: true, css: '', js: '' }, style: {} }),
  },
  log_packages: {
    icon: '🗂️', labelKey: 'portal.modLogPackages', category: 'system',
    defaults: (t) => ({
      data: { description: '', btn_1h: t('portal.seed.common.btn1h'), btn_6h: t('portal.seed.common.btn6h'), btn_24h: t('portal.seed.common.btn24h'), css: '', js: '' },
      style: {},
    }),
  },
}

export function moduleTypeLabel(type, t) {
  const def = MODULE_DEFS[type]
  return def ? t(def.labelKey) : type
}

// Grouped module types for the "add module" dropdown
export function moduleGroups() {
  return MODULE_CATEGORIES.map((cat) => ({
    ...cat,
    types: Object.entries(MODULE_DEFS)
      .filter(([, def]) => def.category === cat.key)
      .map(([type, def]) => ({ type, icon: def.icon, labelKey: def.labelKey })),
  }))
}

export function genId(prefix) {
  return (prefix || 'id') + '-' + Math.random().toString(36).slice(2, 9)
}

// Create a new module instance with localized defaults. Never throws for
// unknown types — falls back to empty data/style so the UI stays stable.
export function createModule(type, page, t) {
  const def = MODULE_DEFS[type]
  const base = def ? def.defaults(t) : { data: { css: '', js: '' }, style: {} }
  return {
    id: genId('mod'),
    type,
    title: moduleTypeLabel(type, t),
    page: page || 'overview',
    visible: true,
    data: base.data,
    style: base.style,
  }
}
