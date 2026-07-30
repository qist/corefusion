// Module type registry — single source of truth for the portal builder.
// Every module type allowed by the backend (internal/service/portal/validate.go)
// must be registered here so the add-menu, property forms and default values
// can never drift out of sync.

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
      data: { eyebrow: '', heading: t('portal.dft.hero.heading'), description: '', button_text: '', button_target: 'subscription' },
      style: { variant: 'primary', align: 'left', custom_css: '' },
    }),
  },
  markdown: {
    icon: '📝', labelKey: 'portal.modMarkdown', category: 'content',
    defaults: (t) => ({ data: { markdown: t('portal.dft.markdown') }, style: { align: 'left', custom_css: '' } }),
  },
  rich_text: {
    icon: '🖊️', labelKey: 'portal.modRichText', category: 'content',
    defaults: (t) => ({ data: { html: t('portal.dft.richText') }, style: { align: 'left', custom_css: '' } }),
  },
  notice: {
    icon: '📢', labelKey: 'portal.modNotice', category: 'content',
    defaults: (t) => ({
      data: {
        title: t('portal.dft.notice.title'), markdown: t('portal.dft.notice.markdown'),
        display_mode: 'inline', popup_trigger: 'on_load', popup_dismiss: 'once', popup_width: '480px',
        popup_button_text: t('portal.api.ex.defaultPopupButton'), popup_show_close: true,
        popup_link_text: '', popup_link_url: '', popup_link_external: true,
      },
      style: { variant: 'info', custom_css: '' },
    }),
  },
  custom_html: {
    icon: '🔧', labelKey: 'portal.modCustomHtml', category: 'content',
    defaults: (t) => ({ data: { html: t('portal.dft.customHtml') }, style: { custom_css: '' } }),
  },
  freeform: {
    icon: '🧩', labelKey: 'portal.modFreeform', category: 'content',
    defaults: (t) => ({
      data: {
        html: t('portal.dft.freeform.html'),
        css: '.my-module {\n  padding: 20px;\n  border-radius: 12px;\n  background: var(--accent-light);\n}\n.my-module h3 {\n  color: var(--accent);\n  margin-bottom: 8px;\n}\n.my-module button {\n  padding: 6px 16px;\n  border: none;\n  border-radius: 6px;\n  background: var(--accent);\n  color: #fff;\n  cursor: pointer;\n}',
        js: t('portal.dft.freeform.js'),
      },
      style: { custom_css: '', custom_class: '' },
    }),
  },
  feature_cards: {
    icon: '⭐', labelKey: 'portal.modFeatureCards', category: 'content',
    defaults: (t) => ({
      data: { cards: [{ icon: '⭐', title: t('portal.dft.featureCards.title'), description: t('portal.dft.featureCards.desc') }] },
      style: { custom_css: '' },
    }),
  },
  tutorial_cards: {
    icon: '📚', labelKey: 'portal.modTutorialCards', category: 'content',
    defaults: () => ({ data: {}, style: { custom_css: '' } }),
  },
  article_list: {
    icon: '📖', labelKey: 'portal.modArticleList', category: 'content',
    defaults: () => ({ data: { show_category_filter: true, default_category: '' }, style: { custom_css: '' } }),
  },
  // --- Account & subscription ---
  account_info: {
    icon: '👤', labelKey: 'portal.modAccountInfo', category: 'account',
    defaults: () => ({ data: {}, style: { custom_css: '' } }),
  },
  traffic_ring: {
    icon: '📊', labelKey: 'portal.modTrafficRing', category: 'account',
    defaults: () => ({ data: {}, style: { custom_css: '' } }),
  },
  subscription_detail: {
    icon: '🧾', labelKey: 'portal.modSubDetail', category: 'account',
    defaults: (t) => ({
      data: { description: '', copy_text: t('portal.seed.common.copy'), qr_text: t('portal.seed.common.qr'), show_status: true, show_rotate: true },
      style: { custom_css: '' },
    }),
  },
  client_download: {
    icon: '💻', labelKey: 'portal.modClientDownload', category: 'account',
    defaults: () => ({ data: {}, style: { custom_css: '' } }),
  },
  streaming_status: {
    icon: '🎬', labelKey: 'portal.modStreamingStatus', category: 'account',
    defaults: () => ({ data: {}, style: { custom_css: '' } }),
  },
  shared_accounts: {
    icon: '🔑', labelKey: 'portal.modSharedAccounts', category: 'account',
    defaults: (t) => ({
      data: { description: t('portal.seed.mod.shared.desc'), apply_btn_text: t('portal.seed.mod.shared.applyBtn'), repair_btn_text: t('portal.seed.mod.shared.repairBtn') },
      style: { custom_css: '' },
    }),
  },
  // --- Business ---
  plan_list: {
    icon: '📦', labelKey: 'portal.modPlanList', category: 'business',
    defaults: (t) => ({
      data: { show_price: true, show_features: true, show_quota: true, button_text: t('portal.seed.common.buy'), buy_text: t('portal.seed.common.buyNow'), renew_text: t('portal.seed.common.renew'), columns: 2 },
      style: { custom_css: '' },
    }),
  },
  order_create: {
    icon: '🛒', labelKey: 'portal.modOrderCreate', category: 'business',
    defaults: (t) => ({ data: { description: t('portal.dft.orderCreate.desc') }, style: { custom_css: '' } }),
  },
  charge_panel: {
    icon: '💳', labelKey: 'portal.modChargePanel', category: 'business',
    defaults: (t) => ({
      data: { description: t('portal.seed.mod.charge.desc'), show_orders: true, orders_limit: 5, card_placeholder: t('portal.seed.mod.charge.cardPlaceholder'), card_btn_text: t('portal.seed.mod.charge.cardBtn') },
      style: { custom_css: '' },
    }),
  },
  balance_display: {
    icon: '💰', labelKey: 'portal.modBalanceDisplay', category: 'business',
    defaults: (t) => ({ data: { show_transactions: true, charge_btn_text: t('portal.seed.common.charge'), tx_limit: 10 }, style: { custom_css: '' } }),
  },
  order_list: {
    icon: '🧾', labelKey: 'portal.modOrderList', category: 'business',
    defaults: () => ({ data: {}, style: { custom_css: '' } }),
  },
  traffic_pack_list: {
    icon: '📦', labelKey: 'portal.modTrafficPackList', category: 'business',
    defaults: (t) => ({
      data: { notice_text: '', button_text: t('portal.seed.common.buy'), html: '', css: '', js: 'return {}' },
      style: { custom_css: '' },
    }),
  },
  referral: {
    icon: '🎁', labelKey: 'portal.modReferral', category: 'business',
    defaults: (t) => ({
      data: { description: t('portal.seed.mod.referral.desc'), copy_btn_text: t('portal.seed.mod.referral.copyBtn'), show_records: true, records_limit: 10 },
      style: { custom_css: '' },
    }),
  },
  // --- System ---
  node_list: {
    icon: '📋', labelKey: 'portal.modNodeList', category: 'system',
    defaults: () => ({ data: {}, style: { custom_css: '' } }),
  },
  ticket_list: {
    icon: '🎫', labelKey: 'portal.modTicketList', category: 'system',
    defaults: (t) => ({ data: { new_btn_text: t('portal.seed.mod.tickets.newBtn'), show_categories: true }, style: { custom_css: '' } }),
  },
  security_settings: {
    icon: '🔒', labelKey: 'portal.modSecuritySettings', category: 'system',
    defaults: () => ({ data: { description: '', show_ip_whitelist: true, show_password: true }, style: { custom_css: '' } }),
  },
  log_packages: {
    icon: '🗂️', labelKey: 'portal.modLogPackages', category: 'system',
    defaults: (t) => ({
      data: { description: '', btn_1h: t('portal.seed.common.btn1h'), btn_6h: t('portal.seed.common.btn6h'), btn_24h: t('portal.seed.common.btn24h') },
      style: { custom_css: '' },
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
  const base = def ? def.defaults(t) : { data: {}, style: { custom_css: '' } }
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
