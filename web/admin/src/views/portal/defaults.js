// Default seed data for a fresh portal site (used when the server returns no
// navigation/modules). Kept in sync with backend defaults.go for new sites;
// once saved, the backend config JSON is the source of truth.

export function createDefaultNavigation(t) {
  return [
    { id: 'nav-overview', label: t('portal.seed.nav.overview'), icon: '📊', target: 'overview', href: '', enabled: true, open_external: false },
    { id: 'nav-subscription', label: t('portal.seed.nav.subscription'), icon: '📋', target: 'subscription', href: '', enabled: true, open_external: false },
    { id: 'nav-store', label: t('portal.seed.nav.store'), icon: '💎', target: 'store', href: '', enabled: true, open_external: false },
    { id: 'nav-security', label: t('portal.seed.nav.security'), icon: '🔒', target: 'security', href: '', enabled: true, open_external: false },
    { id: 'nav-support', label: t('portal.seed.nav.support'), icon: '💬', target: 'support', href: '', enabled: true, open_external: false },
    { id: 'nav-logs', label: t('portal.seed.nav.logs'), icon: '📦', target: 'logs', href: '', enabled: true, open_external: false },
  ]
}

export function createDefaultModules(t) {
  return [
    {
      id: 'mod-hero', type: 'hero', title: t('portal.seed.mod.hero.title'), page: 'overview', visible: true,
      data: { eyebrow: 'WELCOME BACK', heading: t('portal.seed.mod.hero.heading'), description: t('portal.seed.mod.hero.desc'), button_text: t('portal.seed.mod.hero.btn'), button_target: 'subscription' },
      style: { variant: 'primary', align: 'left' },
    },
    {
      id: 'mod-account', type: 'account_info', title: t('portal.seed.mod.account.title'), page: 'overview', visible: true,
      data: {}, style: {},
    },
    {
      id: 'mod-traffic', type: 'traffic_ring', title: t('portal.seed.mod.traffic.title'), page: 'overview', visible: true,
      data: {}, style: {},
    },
    {
      id: 'md-notice', type: 'notice', title: t('portal.seed.mod.notice.title'), page: 'overview', visible: true,
      data: { title: t('portal.seed.mod.notice.dataTitle'), markdown: t('portal.seed.mod.notice.markdown') },
      style: { variant: 'info' },
    },
    {
      id: 'mod-sub', type: 'subscription_detail', title: t('portal.seed.mod.sub.title'), page: 'subscription', visible: true,
      data: { description: t('portal.seed.mod.sub.desc'), copy_text: t('portal.seed.common.copy'), qr_text: t('portal.seed.common.qr'), show_status: true, show_rotate: true },
      style: {},
    },
    {
      id: 'mod-sec', type: 'security_settings', title: t('portal.seed.mod.sec.title'), page: 'security', visible: true,
      data: { description: t('portal.seed.mod.sec.desc'), show_ip_whitelist: true, show_password: true },
      style: {},
    },
    {
      id: 'mod-log', type: 'log_packages', title: t('portal.seed.mod.log.title'), page: 'logs', visible: true,
      data: { description: t('portal.seed.mod.log.desc'), btn_1h: t('portal.seed.common.btn1h'), btn_6h: t('portal.seed.common.btn6h'), btn_24h: t('portal.seed.common.btn24h') },
      style: {},
    },
    {
      id: 'mod-log-info', type: 'markdown', title: t('portal.seed.mod.logInfo.title'), page: 'logs', visible: true,
      data: { markdown: t('portal.seed.mod.logInfo.markdown') },
      style: { align: 'left' },
    },
    {
      id: 'mod-plan-list', type: 'plan_list', title: t('portal.seed.mod.plan.title'), page: 'store', visible: true,
      data: { show_price: true, show_features: true, show_quota: true, button_text: t('portal.seed.common.buy'), buy_text: t('portal.seed.common.buyNow'), renew_text: t('portal.seed.common.renew'), columns: 2 },
      style: {},
    },
    {
      id: 'mod-balance', type: 'balance_display', title: t('portal.seed.mod.balance.title'), page: 'store', visible: true,
      data: { show_transactions: true, charge_btn_text: t('portal.seed.common.charge'), tx_limit: 10 },
      style: {},
    },
    {
      id: 'mod-charge', type: 'charge_panel', title: t('portal.seed.mod.charge.title'), page: 'store', visible: false,
      data: { description: t('portal.seed.mod.charge.desc'), show_orders: true, orders_limit: 5, card_placeholder: t('portal.seed.mod.charge.cardPlaceholder'), card_btn_text: t('portal.seed.mod.charge.cardBtn') },
      style: {},
    },
    {
      id: 'mod-referral', type: 'referral', title: t('portal.seed.mod.referral.title'), page: 'support', visible: true,
      data: { description: t('portal.seed.mod.referral.desc'), copy_btn_text: t('portal.seed.mod.referral.copyBtn'), show_records: true, records_limit: 10 },
      style: {},
    },
    {
      id: 'mod-tickets', type: 'ticket_list', title: t('portal.seed.mod.tickets.title'), page: 'support', visible: true,
      data: { new_btn_text: t('portal.seed.mod.tickets.newBtn'), show_categories: true },
      style: {},
    },
    {
      id: 'mod-articles', type: 'article_list', title: t('portal.seed.mod.articles.title'), page: 'support', visible: true,
      data: { show_category_filter: true, default_category: '' },
      style: {},
    },
    {
      id: 'mod-shared-accts', type: 'shared_accounts', title: t('portal.seed.mod.shared.title'), page: 'support', visible: true,
      data: { description: t('portal.seed.mod.shared.desc'), apply_btn_text: t('portal.seed.mod.shared.applyBtn'), repair_btn_text: t('portal.seed.mod.shared.repairBtn') },
      style: {},
    },
  ]
}
