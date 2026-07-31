package portal

// defaultNavigationJSON is the default navigation items for a new portal.
// Labels use lang keys (e.g. "nav.home") which are resolved by t() at runtime.
// If a label is not a lang key, t() returns it as-is (backward compatible).
const defaultNavigationJSON = `[
  {"id":"nav-home","label":"nav.home","icon":"🏠","target":"overview","href":"","enabled":true,"open_external":false},
  {"id":"nav-services","label":"nav.store","icon":"🛒","target":"store","href":"","enabled":true,"open_external":false},
  {"id":"nav-billing","label":"nav.billing","icon":"💰","target":"billing","href":"","enabled":true,"open_external":false},
  {"id":"nav-nodes","label":"nav.nodes","icon":"📋","target":"nodes","href":"","enabled":true,"open_external":false},
  {"id":"nav-tutorial","label":"nav.tutorial","icon":"📚","target":"tutorial","href":"","enabled":true,"open_external":false},
  {"id":"nav-tickets","label":"nav.support","icon":"🎫","target":"support","href":"","enabled":true,"open_external":false},
  {"id":"nav-referral","label":"nav.referral","icon":"🎁","target":"referral","href":"","enabled":true,"open_external":false},
  {"id":"nav-profile","label":"nav.profile","icon":"👤","target":"profile","href":"","enabled":true,"open_external":false},
  {"id":"nav-logs","label":"nav.logs","icon":"📦","target":"logs","href":"","enabled":false,"open_external":false}
]`

// defaultModulesJSON is the default module layout for a new portal.
// Each module includes html/css/js code in its data field, so the database
// always stores complete, editable module code from the start.
// The HTML templates are extracted from the original hardcoded template.go
// to preserve all functionality and i18n keys.
// Titles and text fields use lang keys (e.g. "traffic.title") resolved by t() at runtime.
const defaultModulesJSON = `[
  {
    "id": "mod-hero",
    "type": "hero",
    "title": "hero.welcome",
    "page": "overview",
    "visible": true,
    "data": {
      "eyebrow": "WELCOME",
      "heading": "hero.welcome_back",
      "description": "hero.manage_sub",
      "button_text": "hero.manage_sub",
      "button_target": "store",
      "html": "<div v-if=\"mod.data && mod.data.eyebrow\" class=\"hero-eyebrow\">{{ t(mod.data.eyebrow) }}</div>\n              <h2 v-if=\"mod.data && mod.data.heading\" class=\"hero-heading\">{{ t(mod.data.heading) }}</h2>\n              <p v-if=\"mod.data && mod.data.description\" class=\"hero-desc\">{{ t(mod.data.description) }}</p>\n              <button v-if=\"mod.data && mod.data.button_text && mod.data.button_target !== 'overview'\" class=\"hero-btn\" @click=\"var target = mod.data.button_target || 'store'; activeTab = target; saveActiveTab(target); fetchPageData(target)\">{{ t(mod.data.button_text) }}</button>",
      "css": "",
      "js": "return {}"
    },
    "style": {
      "variant": "primary",
      "align": "left"
    }
  },
  {
    "id": "mod-account",
    "type": "account_info",
    "title": "account.title",
    "page": "overview",
    "visible": true,
    "data": {
      "columns": 2,
      "html": "<div class=\"card-header\"><el-icon class=\"card-icon\" size=\"20\"><User /></el-icon><span class=\"card-title\">{{ t(mod.title) }}</span></div>\n              <div class=\"acct-info-list\" :class=\"{ 'two-column': mod.data && mod.data.columns === 2 }\">\n                <div class=\"acct-info-row\">\n                  <span class=\"acct-info-icon\">🛡️</span>\n                  <div class=\"acct-info-body\">\n                    <span class=\"acct-info-label\">{{ t('account.status') }}</span>\n                    <div class=\"acct-info-value-row\">\n                      <span class=\"acct-info-value\">● {{ statusText }}</span>\n                      <span v-if=\"client.status === 'active'\" class=\"badge badge-active\" style=\"margin-left:6px\">{{ t('account.normal') }}</span>\n                      <span v-else-if=\"client.status === 'suspended'\" class=\"badge badge-suspended\" style=\"margin-left:6px\">{{ t('account.suspended') }}</span>\n                      <span v-else-if=\"client.status === 'expired'\" class=\"badge badge-expired\" style=\"margin-left:6px\">{{ t('common.expired') }}</span>\n                      <span v-else class=\"badge badge-none\" style=\"margin-left:6px\">{{ t('common.unknown') }}</span>\n                    </div>\n                  </div>\n                </div>\n                <div class=\"acct-info-row\">\n                  <span class=\"acct-info-icon\">⭐</span>\n                  <div class=\"acct-info-body\">\n                    <span class=\"acct-info-label\">{{ t('account.plan') }}</span>\n                    <span class=\"acct-info-value\">{{ client.plan_name || t('account.plan_default') }}</span>\n                  </div>\n                </div>\n                <div class=\"acct-info-row\">\n                  <span class=\"acct-info-icon\">📧</span>\n                  <div class=\"acct-info-body\">\n                    <span class=\"acct-info-label\">{{ t('account.email') }}</span>\n                    <span class=\"acct-info-value\">{{ client.email }}</span>\n                  </div>\n                </div>\n                <div class=\"acct-info-row\">\n                  <span class=\"acct-info-icon\">🕐</span>\n                  <div class=\"acct-info-body\">\n                    <span class=\"acct-info-label\">{{ t('account.register_time') }}</span>\n                    <span class=\"acct-info-value\">{{ client.created_at ? formatDate(client.created_at) : '-' }}</span>\n                  </div>\n                </div>\n                <div class=\"acct-info-row\">\n                  <span class=\"acct-info-icon\">🔄</span>\n                  <div class=\"acct-info-body\">\n                    <span class=\"acct-info-label\">{{ t('account.expire') }}</span>\n                    <span class=\"acct-info-value\">{{ client.expires_at ? formatDate(client.expires_at) : t('common.unlimited') }}</span>\n                  </div>\n                </div>\n                <!-- balance display -->\n                <div class=\"acct-info-row\" v-if=\"mod.data && mod.data.show_balance !== false\">\n                  <el-icon class=\"acct-info-icon\" size=\"16\"><Wallet /></el-icon>\n                  <div class=\"acct-info-body\">\n                    <span class=\"acct-info-label\">{{ t('balance.available') }}</span>\n                    <div class=\"acct-info-value-row\">\n                      <span class=\"acct-info-value\">{{ formatMoney(balance.balance) }}</span>\n                      <el-button type=\"primary\" size=\"small\" @click=\"showChargeDialog = true\" style=\"margin-left: 8px;\">{{ t('balance.charge_btn') }}</el-button>\n                    </div>\n                  </div>\n                </div>\n              </div>\n              <!-- Traffic mini bar inside account info -->\n              <div class=\"acct-traffic-section\">\n                <div class=\"acct-traffic-label\">\n                  <span>📊 {{ t('traffic.title') }}</span>\n                  <span class=\"acct-traffic-pct\">{{ usagePercent }}% / {{ formatBytes(client.quota_bytes) }}</span>\n                </div>\n                <div class=\"traffic-bar\" style=\"height:8px;border-radius:6px;background:var(--ring-bg);overflow:hidden;margin-top:6px;\">\n                  <div class=\"traffic-bar-fill\" :class=\"{ warn: usagePercent > 70, danger: usagePercent > 90 }\" :style=\"{ width: usagePercent + '%' }\"></div>\n                </div>\n                <div class=\"acct-traffic-hint\" style=\"font-size:11px;color:var(--text-muted);margin-top:4px;\">{{ t('traffic.remaining') }} {{ formatBytes(Math.max(0, client.quota_bytes - client.used_bytes)) }} {{ t('traffic.available') }}</div>\n              </div>\n              <!-- Gift claim section -->\n              <div class=\"acct-gift-section\" v-if=\"gifts && gifts.length > 0\">\n                <div class=\"acct-gift-header\">\n                  <span>🎁 {{ t('gift.title') }}</span>\n                  <span class=\"acct-gift-count\" v-if=\"gifts.length > 0\">{{ gifts.length }}</span>\n                </div>\n                <div class=\"acct-gift-list\">\n                  <div v-for=\"g in gifts\" :key=\"g.id\" class=\"acct-gift-item\">\n                    <div class=\"acct-gift-info\">\n                      <span class=\"acct-gift-type\">{{ giftTypeText(g.type) }}</span>\n                      <span class=\"acct-gift-value\">{{ g.type === 'days' ? g.value + ' ' + t('gift.days') : formatBytes(g.value) }}</span>\n                      <span class=\"acct-gift-source\" v-if=\"g.source\">· {{ giftSourceText(g.source) }}</span>\n                    </div>\n                    <el-button type=\"success\" size=\"small\" :loading=\"giftClaiming\" @click=\"claimGift(g.id)\">{{ t('gift.claim_btn') }}</el-button>\n                  </div>\n                </div>\n              </div>",
      "css": "",
      "js": "return {}"
    },
    "style": {}
  },
  {
    "id": "mod-traffic",
    "type": "traffic_ring",
    "title": "traffic.title",
    "page": "overview",
    "visible": true,
    "data": {
      "html": "<div class=\"card-header\">\n                <span class=\"card-icon\">📊</span>\n                <span class=\"card-title\">{{ t(mod.title) }}</span>\n                <span class=\"card-action\" v-if=\"refreshing\">{{ t('common.refreshing') }}</span>\n              </div>\n              <div class=\"traffic-ring-wrap\">\n                <svg class=\"traffic-ring\" viewBox=\"0 0 120 120\">\n                  <circle cx=\"60\" cy=\"60\" r=\"52\" fill=\"none\" class=\"ring-bg\"/>\n                  <circle cx=\"60\" cy=\"60\" r=\"52\" fill=\"none\" class=\"ring-fill\" :stroke-dasharray=\"ringDash\" :style=\"{stroke: usageColor}\"/>\n                </svg>\n                <div class=\"ring-text\">\n                  <span class=\"ring-percent\">{{ usagePercent }}%</span>\n                  <span class=\"ring-label\">{{ t('traffic.used') }}</span>\n                </div>\n              </div>\n              <div class=\"traffic-bar-wrap\">\n                <div class=\"traffic-bar-header\">\n                  <span class=\"bar-label\">📊 {{ t('traffic.title') }}</span>\n                  <span class=\"bar-nums\">{{ formatBytes(client.used_bytes) }} <span style=\"color:var(--text-muted);\">/</span> {{ formatBytes(client.quota_bytes) }}</span>\n                </div>\n                <div class=\"traffic-bar\">\n                  <div class=\"traffic-bar-fill\" :class=\"{ warn: usagePercent > 70, danger: usagePercent > 90 }\" :style=\"{ width: usagePercent + '%' }\"></div>\n                </div>\n                <div class=\"traffic-bar-footer\">\n                  <span class=\"bar-hint\">{{ t('traffic.upload') }} {{ formatBytes(client.upload_bytes) }} · {{ t('traffic.download') }} {{ formatBytes(client.download_bytes) }}</span>\n                  <span class=\"bar-pct\">{{ usagePercent }}%</span>\n                </div>\n              </div>\n              <div class=\"traffic-stats\">\n                <div class=\"stat-item\"><span class=\"stat-icon\">↑</span><span class=\"stat-value\">{{ formatBytes(client.upload_bytes) }}</span><span class=\"stat-label\">{{ t('traffic.upload') }}</span></div>\n                <div class=\"stat-divider\"></div>\n                <div class=\"stat-item\"><span class=\"stat-icon\">↓</span><span class=\"stat-value\">{{ formatBytes(client.download_bytes) }}</span><span class=\"stat-label\">{{ t('traffic.download') }}</span></div>\n                <div class=\"stat-divider\"></div>\n                <div class=\"stat-item\"><span class=\"stat-icon\">💾</span><span class=\"stat-value\">{{ formatBytes(client.quota_bytes) }}</span><span class=\"stat-label\">{{ t('traffic.total') }}</span></div>\n              </div>",
      "css": "",
      "js": "return {}"
    },
    "style": {}
  },
  {
    "id": "mod-traffic-history",
    "type": "traffic_history",
    "title": "traffic_history.title",
    "page": "overview",
    "visible": true,
    "data": {
      "months": 6,
      "html": "<div class=\"th-card\"><div class=\"th-head\"><span class=\"th-icon\">📈</span><span class=\"th-title\">{{ t(mod.title) }}</span></div><div v-if=\"loading\" class=\"th-state\">{{ t('traffic_history.loading') }}</div><div v-else-if=\"error\" class=\"th-state th-error\">{{ error }}</div><div v-else-if=\"history.length === 0\" class=\"th-state\">{{ t('traffic_history.empty') }}</div><div v-else class=\"th-list\"><div class=\"th-row\" v-for=\"item in history\" :key=\"item.period_start\"><div class=\"th-period\">{{ fmtMonth(item.period_start) }}</div><div class=\"th-bar-wrap\"><div class=\"th-bar\" :style=\"{ width: barWidth(item) }\"></div></div><div class=\"th-bytes\">{{ formatBytes(item.total_bytes) }}</div></div></div></div>",
      "css": ".th-card{padding:16px}.th-head{display:flex;align-items:center;gap:8px;margin-bottom:12px}.th-icon{font-size:18px}.th-title{font-weight:600;font-size:15px}.th-state{color:var(--text-muted,#999);font-size:13px;padding:8px 0}.th-error{color:#f56c6c}.th-list{display:flex;flex-direction:column;gap:8px}.th-row{display:flex;align-items:center;gap:10px;font-size:13px}.th-period{width:56px;color:var(--text-muted,#999);flex:none}.th-bar-wrap{flex:1;height:8px;background:var(--bg-muted,#eee);border-radius:4px;overflow:hidden}.th-bar{height:100%;background:var(--accent,#409eff);border-radius:4px;transition:width .3s}.th-bytes{width:72px;text-align:right;flex:none;font-variant-numeric:tabular-nums}",
      "js": "const { ref, computed } = ctx; const history = ref([]); const loading = ref(true); const error = ref(''); const months = (ctx.mod && ctx.mod.data && ctx.mod.data.months) || 6; ctx.api.get('/traffic-history', { months: months }).then(function (res) { history.value = (res && res.history) || []; }).catch(function (e) { error.value = (e && (e.message || e.error)) || t('traffic_history.error'); }).finally(function () { loading.value = false; }); const maxBytes = computed(function () { let m = 0; history.value.forEach(function (h) { if ((h.total_bytes || 0) > m) m = h.total_bytes; }); return m || 1; }); const barWidth = function (item) { return Math.max(6, Math.round(((item.total_bytes || 0) / maxBytes.value) * 100)) + '%'; }; const fmtMonth = function (s) { if (!s) return ''; return ('' + s).slice(0, 7); }; return { history: history, loading: loading, error: error, months: months, barWidth: barWidth, fmtMonth: fmtMonth, formatBytes: ctx.formatBytes, t: ctx.t };"
    },
    "style": {}
  },
  {
    "id": "mod-client-download",
    "type": "client_download",
    "title": "client.title",
    "page": "overview",
    "visible": true,
    "data": {
      "platforms": [
        {
          "id": "windows",
          "name": "Windows",
          "icon": "windows",
          "url": ""
        },
        {
          "id": "ios",
          "name": "iOS",
          "icon": "apple",
          "url": ""
        },
        {
          "id": "android",
          "name": "Android",
          "icon": "android",
          "url": ""
        },
        {
          "id": "mac",
          "name": "Mac",
          "icon": "desktop",
          "url": ""
        },
        {
          "id": "linux",
          "name": "Linux",
          "icon": "linux",
          "url": ""
        },
        {
          "id": "browser",
          "name": "Browser",
          "icon": "chrome",
          "url": ""
        },
        {
          "id": "other",
          "name": "Other",
          "icon": "ellipsis",
          "url": ""
        }
      ],
      "select_text": "common.other",
      "hint_text": "tutorial.select_platform",
      "html": "<div class=\"card-header\"><span class=\"card-icon\">💻</span><span class=\"card-title\">{{ t(mod.title) }}</span></div>\n              <div class=\"client-platform-tabs\">\n                <div\n                  v-for=\"p in (mod.data && mod.data.platforms) || []\"\n                  :key=\"p.id\"\n                  class=\"platform-tab\"\n                  :class=\"{ active: selectedPlatform === p.id }\"\n                  @click=\"selectPlatform(p.id)\"\n                >\n                  <el-icon v-if=\"p.icon\"><component :is=\"p.icon\" /></el-icon>\n                  <span>{{ p.name }}</span>\n                </div>\n              </div>\n              <!-- Hint when no platform selected -->\n              <div v-if=\"!selectedPlatform\" class=\"client-download-hint\">\n                <el-icon><ArrowUp /></el-icon>\n                <span>{{ t((mod.data && mod.data.hint_text) || 'tutorial.select_platform') }}</span>\n                <el-icon><ArrowUp /></el-icon>\n              </div>\n              <!-- Content when platform selected -->\n              <div v-else class=\"client-download-content\">\n                <div class=\"client-download-platform-header\">\n                  <span class=\"client-download-platform-name\">{{ getSelectedPlatformName() }}</span>\n                  <el-button text size=\"small\" @click=\"navigateTo('tutorial')\">{{ t('tutorial.view_all') }} →</el-button>\n                </div>\n                <!-- Tutorial articles (contains download links) -->\n                <div v-if=\"getPlatformArticles(selectedPlatform).length > 0\" class=\"client-download-articles\">\n                  <div class=\"client-download-section-title\">📖 {{ t('tutorial.related') }}</div>\n                  <div class=\"article-list compact\">\n                    <div\n                      class=\"article-item\"\n                      v-for=\"article in getPlatformArticles(selectedPlatform)\"\n                      :key=\"article.id\"\n                      @click=\"openArticle(article.slug)\"\n                    >\n                      <span class=\"article-cat-badge\">{{ articleCatText(article.category) }}</span>\n                      <div class=\"article-info\">\n                        <span class=\"article-title\">{{ article.title }}</span>\n                        <span class=\"logpkg-time\">{{ formatDate(article.published_at || article.updated_at) }}</span>\n                      </div>\n                      <el-icon class=\"article-arrow\"><ArrowRight /></el-icon>\n                    </div>\n                  </div>\n                </div>\n                <!-- Empty state -->\n                <div v-if=\"getPlatformArticles(selectedPlatform).length === 0\" class=\"empty-state\" style=\"padding: 24px;\">\n                  <el-icon class=\"empty-icon\" size=\"48\"><Box /></el-icon>\n                  <p class=\"empty-text\">{{ t('tutorial.no_content') }}</p>\n                  <p class=\"empty-hint\">{{ t('tutorial.no_content_hint') }}</p>\n                </div>\n              </div>",
      "css": "",
      "js": "return {}"
    },
    "style": {}
  },
  {
    "id": "mod-sub",
    "type": "subscription_detail",
    "title": "sub.title",
    "page": "overview",
    "visible": true,
    "data": {
      "description": "sub.description",
      "copy_text": "common.copy",
      "qr_text": "sub.qr_code",
      "show_status": true,
      "show_rotate": true,
      "actions": [
        {
          "text": "sub.buy_plan",
          "type": "primary",
          "action": "navigate",
          "target": "store"
        },
        {
          "text": "sub.whitelist",
          "type": "info",
          "action": "dialog",
          "target": "whitelist"
        },
        {
          "text": "sub.more_subs",
          "type": "info",
          "action": "dialog",
          "target": "more_subs"
        },
        {
          "text": "sub.reset",
          "type": "danger",
          "action": "confirm",
          "target": "reset_sub"
        }
      ],
      "html": "<div class=\"card-header\"><span class=\"card-icon\">🔗</span><span class=\"card-title\">{{ t(mod.title) }}</span></div>\n              <div v-if=\"client.sub_token && client.sub_status !== 'revoked'\">\n                <div v-if=\"client.status === 'suspended'\" style=\"padding: 10px 12px; background: rgba(245,108,108,0.1); border: 1px solid rgba(245,108,108,0.3); border-radius: 8px; margin-bottom: 12px;\">\n                  <p style=\"color: var(--danger); margin: 0; font-size: 13px;\">⚠️ {{ t('account.suspended') }}，{{ t('account.suspended_hint') }}</p>\n                </div>\n                <div class=\"sub-link-box\">\n                  <code class=\"sub-link-text\">{{ showSubUrl && subUrl ? subUrl : '••••••••••••••••••••••••••' }}</code>\n                  <div class=\"sub-link-actions\">\n                    <el-button size=\"small\" text @click=\"showSubUrl = !showSubUrl\"><el-icon><View v-if=\"!showSubUrl\" /><Hide v-else /></el-icon></el-button>\n                    <el-button size=\"small\" text @click=\"copyText(subUrl)\" :disabled=\"!subUrl\"><el-icon><DocumentCopy /></el-icon></el-button>\n                    <el-button size=\"small\" text @click=\"showQrDialog\" :disabled=\"!subUrl\">{{ t('sub.qr_code') }}</el-button>\n                  </div>\n                </div>\n                <div class=\"sub-actions-row\" v-if=\"mod.data && mod.data.actions\">\n                  <el-button \n                    v-for=\"action in mod.data.actions\" \n                    :key=\"action.target\"\n                    :type=\"action.type || 'default'\"\n                    size=\"small\"\n                    @click=\"handleSubAction(action)\"\n                  >\n                    {{ t(action.text) }}\n                  </el-button>\n                </div>\n              </div>\n              <div v-else-if=\"client.sub_status === 'revoked'\" class=\"empty-state\">\n                <el-icon class=\"empty-icon\" size=\"48\" color=\"var(--danger)\"><CircleClose /></el-icon><p class=\"empty-text danger\">{{ t('sub.revoked') }}</p><p class=\"empty-hint\">{{ t('sub.revoked_hint') }}</p>\n              </div>\n<div v-else-if=\"!client.sub_token\" class=\"empty-state\">\n<el-icon class=\"empty-icon\" size=\"48\" color=\"var(--warning)\"><Warning /></el-icon><p class=\"empty-text danger\">{{ t('account.suspended') }}</p><p class=\"empty-hint\">{{ t('account.suspended_hint') }}</p>\n</div>",
      "css": "",
      "js": "return {}"
    },
    "style": {}
  },
  {
    "id": "mod-streaming",
    "type": "streaming_status",
    "title": "streaming.title",
    "page": "overview",
    "visible": true,
    "data": {
      "html": "<div class=\"streaming-grid\">\n                <!-- Apple Shared ID -->\n                <div class=\"streaming-card\">\n                  <div class=\"streaming-name\">{{ t('shared.apple_id') }}</div>\n                  <div class=\"streaming-icon\"><span style=\"font-size: 32px;\">🍎</span></div>\n                  <div class=\"streaming-action\" v-if=\"appleAccounts.length > 0\" @click=\"viewAppleAccounts()\">{{ t('streaming.view_account') }}</div>\n                  <div class=\"streaming-action\" v-else style=\"color: var(--text-muted); cursor: default;\">{{ t('streaming.none') }}</div>\n                </div>\n                <!-- Netflix Shared Account -->\n                <div class=\"streaming-card\">\n                  <div class=\"streaming-name\">{{ t('shared.netflix') }}</div>\n                  <div class=\"streaming-icon\"><span style=\"font-size: 32px;\">🎬</span></div>\n                  <div class=\"streaming-action\" v-if=\"netflixAccounts.length > 0\" @click=\"viewNetflixAccounts()\">{{ t('streaming.view_account') }}</div>\n                  <div class=\"streaming-action\" v-else style=\"color: var(--text-muted); cursor: default;\">{{ t('streaming.none') }}</div>\n                </div>\n              </div>",
      "css": "",
      "js": "return {}"
    },
    "style": {}
  },
  {
    "id": "mod-plan-list",
    "type": "plan_list",
    "title": "plan.title",
    "page": "store",
    "visible": true,
    "data": {
      "notice_text": "plan.notice_text",
      "notice_subtext": "plan.notice_subtext",
      "show_price": true,
      "show_features": true,
      "show_quota": true,
      "button_text": "common.buy",
      "buy_text": "common.buy",
      "renew_text": "common.buy",
      "columns": 2,
      "html": "<!-- Top Notice -->\n              <div v-if=\"mod.data && mod.data.notice_text\" class=\"plan-notice\">\n                <div class=\"plan-notice-content\">{{ t(mod.data.notice_text) }}</div>\n                <div v-if=\"mod.data.notice_subtext\" class=\"plan-notice-sub\">{{ t(mod.data.notice_subtext) }}</div>\n              </div>\n              \n              <div v-if=\"plansLoading\" class=\"empty-state\"><el-icon class=\"empty-icon\" size=\"48\" class=\"is-loading\"><Loading /></el-icon><p class=\"empty-text\">{{ t('common.loading') }}</p></div>\n              <div v-else-if=\"plans.length === 0\" class=\"empty-state\"><el-icon class=\"empty-icon\" size=\"48\"><Inbox /></el-icon><p class=\"empty-text\">{{ t('plan.empty') }}</p></div>\n              <div v-else class=\"plan-table-wrap\">\n                <table class=\"plan-table\">\n                  <thead>\n                    <tr>\n                      <th class=\"plan-col-service\">{{ t('plan.col_service') }}</th>\n                      <th class=\"plan-col-desc\">{{ t('plan.col_desc') }}</th>\n                      <th class=\"plan-col-price\">{{ t('plan.col_price') }}</th>\n                      <th class=\"plan-col-action\">{{ t('common.action') }}</th>\n                    </tr>\n                  </thead>\n                  <tbody>\n                    <template v-for=\"plan in plans\" :key=\"plan.id\">\n                      <tr v-for=\"(v, vIdx) in (plan.variants || [])\" :key=\"v.id\" class=\"plan-row\" :class=\"{ 'plan-current': client.plan_id > 0 && client.plan_id === plan.id }\">\n                        <!-- Service Column -->\n                        <td v-if=\"vIdx === 0\" :rowspan=\"plan.variants.length\" class=\"plan-cell plan-cell-service\">\n                          <div class=\"plan-service-name\">{{ plan.name }}</div>\n                          <div class=\"plan-service-meta\">\n                            <div v-if=\"v.quota_bytes\">{{ t('plan.included_traffic') }}: {{ formatBytes(v.quota_bytes) }}/{{ t('plan.per_month') }}</div>\n                            <div v-if=\"v.speed_limit\">{{ t('plan.max_bandwidth') }}: {{ v.speed_limit }}M</div>\n                            <div v-if=\"v.duration_days\">{{ t('plan.duration') }}: {{ v.duration_days }}{{ t('plan.days_unit') }}</div>\n                          </div>\n                        </td>\n                        <!-- Description Column -->\n                        <td class=\"plan-cell plan-cell-desc\">\n                          <div class=\"plan-desc-text\">{{ plan.description || t('plan.default_desc') }}</div>\n                        </td>\n                        <!-- Price Column -->\n                        <td class=\"plan-cell plan-cell-price\">\n                          <span class=\"plan-price-amount\">{{ (v.currency || 'CNY') === 'CNY' ? '¥' : (v.currency === 'USD' ? '$' : v.currency) }}{{ formatMoney(v.price) }}</span>\n                        </td>\n                        <!-- Action Column -->\n                        <td class=\"plan-cell plan-cell-action\">\n                          <el-button \n                            type=\"primary\" \n                            size=\"small\" \n                            @click=\"purchaseVariant(v.id, plan, mod.data)\"\n                            :disabled=\"orderBusy\"\n                          >\n                            <span v-if=\"client.plan_id > 0 && client.plan_id === plan.id\">{{ t((mod.data && mod.data.renew_text) || 'common.buy') }}</span>\n                            <span v-else>{{ t((mod.data && mod.data.buy_text) || 'common.buy') }}</span>\n                          </el-button>\n                          <div v-if=\"client.plan_id > 0 && client.plan_id === plan.id\" class=\"plan-current-badge\">{{ t('plan.current_plan') }}</div>\n                        </td>\n                      </tr>\n                    </template>\n                  </tbody>\n                </table>\n              </div>",
      "css": "",
      "js": "return {}"
    },
    "style": {}
  },
  {
    "id": "mod-traffic-packs",
    "type": "traffic_pack_list",
    "title": "traffic_pack.title",
    "page": "store",
    "visible": true,
    "data": {
      "notice_text": "traffic_pack.notice",
      "button_text": "common.buy",
      "html": "<div class=\"card-header\"><span class=\"card-icon\">📦</span><span class=\"card-title\">{{ t(mod.title) }}</span></div>\n              <div v-if=\"mod.data && mod.data.notice_text\" class=\"plan-notice\" style=\"margin-bottom:12px\">\n                <div class=\"plan-notice-content\">{{ t(mod.data.notice_text) }}</div>\n              </div>\n              <div v-if=\"trafficPacksLoading\" class=\"empty-state\"><el-icon class=\"empty-icon\" size=\"48\" class=\"is-loading\"><Loading /></el-icon><p class=\"empty-text\">{{ t('common.loading') }}</p></div>\n              <div v-else-if=\"trafficPacks.length === 0\" class=\"empty-state\"><el-icon class=\"empty-icon\" size=\"48\"><Inbox /></el-icon><p class=\"empty-text\">{{ t('traffic_pack.empty') }}</p></div>\n              <div v-else class=\"plan-table-wrap\">\n                <table class=\"plan-table\">\n                  <thead>\n                    <tr>\n                      <th class=\"plan-col-service\">{{ t('traffic_pack.col_name') }}</th>\n                      <th class=\"plan-col-desc\">{{ t('traffic_pack.col_desc') }}</th>\n                      <th class=\"plan-col-price\">{{ t('plan.col_price') }}</th>\n                      <th class=\"plan-col-action\">{{ t('common.action') }}</th>\n                    </tr>\n                  </thead>\n                  <tbody>\n                    <tr v-for=\"pack in trafficPacks\" :key=\"pack.id\" class=\"plan-row\">\n                      <td class=\"plan-cell plan-cell-service\">\n                        <div class=\"plan-service-name\">{{ pack.name }}</div>\n                        <div class=\"plan-service-meta\">\n                          <div v-if=\"pack.quota_bytes\">{{ t('traffic_pack.traffic_amount') }}: {{ formatBytes(pack.quota_bytes) }}</div>\n                          <div v-if=\"pack.expiry_mode === 'extend_plan' && pack.duration_days\">{{ t('traffic_pack.extend_hint') }}: +{{ pack.duration_days }}{{ t('plan.days_unit') }}</div>\n                          <div v-else>{{ t('traffic_pack.align_hint') }}</div>\n                        </div>\n                      </td>\n                      <td class=\"plan-cell plan-cell-desc\">\n                        <div class=\"plan-desc-text\">{{ pack.description || t('traffic_pack.default_desc') }}</div>\n                      </td>\n                      <td class=\"plan-cell plan-cell-price\">\n                        <span class=\"plan-price-amount\">{{ (pack.currency || 'CNY') === 'CNY' ? '¥' : (pack.currency === 'USD' ? '$' : pack.currency) }}{{ formatMoney(pack.price) }}</span>\n                      </td>\n                      <td class=\"plan-cell plan-cell-action\">\n                        <el-button type=\"primary\" size=\"small\" @click=\"buyTrafficPack(pack)\" :disabled=\"orderBusy || !client.plan_id\">\n                          {{ t((mod.data && mod.data.button_text) || 'common.buy') }}\n                        </el-button>\n                        <div v-if=\"!client.plan_id\" class=\"plan-current-badge\" style=\"color:var(--text-muted)\">{{ t('traffic_pack.need_plan') }}</div>\n                      </td>\n                    </tr>\n                  </tbody>\n                </table>\n              </div>",
      "css": "",
      "js": "return {}"
    },
    "style": {}
  },
  {
    "id": "mod-orders",
    "type": "order_list",
    "title": "order.title",
    "page": "billing",
    "visible": true,
    "data": {
      "notice_text": "order.notice",
      "notice_highlight": "order.notice_highlight",
      "notice_subtext": "order.notice_subtext",
      "columns": [
        "order.id",
        "order.id",
        "order.service",
        "common.amount",
        "order.date",
        "order.expire",
        "common.status"
      ],
      "limit": 20,
      "html": "<!-- Top Notice -->\n              <div class=\"billing-notice\">\n                <span class=\"billing-notice-text\">{{ t((mod.data && mod.data.notice_text) || 'order.title') }}</span>\n                <span v-if=\"mod.data.notice_highlight\" class=\"billing-notice-highlight\">{{ t(mod.data.notice_highlight) }}</span>\n              </div>\n              <div class=\"billing-notice-sub\">\n                {{ t((mod.data && mod.data.notice_subtext) || 'balance.tx_limit') }}\n              </div>\n              <!-- Order Table -->\n              <div v-if=\"myOrdersLoading\" class=\"empty-state\">\n                <el-icon class=\"empty-icon\" size=\"48\" class=\"is-loading\"><Loading /></el-icon>\n                <p class=\"empty-text\">{{ t('common.loading') }}</p>\n              </div>\n              <div v-else-if=\"myOrders.length === 0\" class=\"empty-state\">\n                <el-icon class=\"empty-icon\" size=\"48\"><Inbox /></el-icon>\n                <p class=\"empty-text\">{{ t('order.empty') }}</p>\n              </div>\n              <div v-else class=\"billing-table-wrap\">\n                <table class=\"billing-table\">\n                  <thead>\n                    <tr>\n                      <th>#</th>\n                      <th>{{ t('order.id') }}</th>\n                      <th>{{ t('plan.col_service') }}</th>\n                      <th>{{ t('common.amount') }}</th>\n                      <th>{{ t('order.date') }}</th>\n                      <th>{{ t('account.expire') }}</th>\n                      <th>{{ t('common.status') }}</th>\n                    </tr>\n                  </thead>\n                  <tbody>\n                    <tr v-for=\"(order, idx) in myOrders.slice(0, (mod.data && mod.data.limit) || 20)\" :key=\"order.id\">\n                      <td class=\"billing-col-index\">{{ idx + 1 }}</td>\n                      <td class=\"billing-col-id\">#{{ order.id }}</td>\n                      <td class=\"billing-col-service\">{{ getOrderServiceName(order) }}</td>\n                      <td class=\"billing-col-amount\">{{ formatMoney(order.paid_amount || order.original_amount) }}</td>\n                      <td class=\"billing-col-date\">{{ formatDate(order.created_at) }}</td>\n                      <td class=\"billing-col-expire\">{{ order.effective_to ? formatDate(order.effective_to) : '-' }}</td>\n                      <td class=\"billing-col-status\"><span :class=\"'badge badge-' + orderStatusBadge(order.status)\">{{ orderStatusText(order.status) }}</span></td>\n                    </tr>\n                  </tbody>\n                </table>\n              </div>\n              <!-- Charge Records -->\n              <div v-if=\"chargeOrders.length > 0\" style=\"margin-top: 20px\">\n                <div class=\"card-header\"><el-icon class=\"card-icon\" size=\"20\"><Document /></el-icon><span class=\"card-title\">{{ t('charge.records') }}</span></div>\n                <div class=\"billing-table-wrap\">\n                  <table class=\"billing-table\">\n                    <thead>\n                      <tr>\n                        <th>#</th>\n                        <th>{{ t('order.trade_no') }}</th>\n                        <th>{{ t('order.source') }}</th>\n                        <th>{{ t('common.amount') }}</th>\n                        <th>{{ t('common.time') }}</th>\n                        <th>{{ t('common.status') }}</th>\n                      </tr>\n                    </thead>\n                    <tbody>\n                      <tr v-for=\"(co, idx) in chargeOrders.slice(0, 10)\" :key=\"co.id\">\n                        <td class=\"billing-col-index\">{{ idx + 1 }}</td>\n                        <td class=\"billing-col-id\">{{ co.trade_no }}</td>\n                        <td>{{ co.source === 'card' ? t('charge.card_title') : (co.source === 'site' ? t('plan.charge_pay') : co.source) }}</td>\n                        <td class=\"billing-col-amount\">{{ formatMoney(co.amount) }} {{ co.currency }}</td>\n                        <td class=\"billing-col-date\">{{ formatDate(co.created_at) }}</td>\n                        <td class=\"billing-col-status\"><span :class=\"'badge badge-' + co.status\">{{ chargeStatusText(co.status) }}</span></td>\n                      </tr>\n                    </tbody>\n                  </table>\n                </div>\n              </div>\n              <!-- Transaction Flow -->\n              <div v-if=\"balanceTxns.length > 0\" style=\"margin-top: 20px\">\n                <div class=\"card-header\"><span class=\"card-icon\">📄</span><span class=\"card-title\">{{ t('balance.balance_change') }}</span></div>\n                <div class=\"billing-table-wrap\">\n                  <table class=\"billing-table\">\n                    <thead>\n                      <tr>\n                        <th>#</th>\n                        <th>{{ t('common.type') }}</th>\n                        <th>{{ t('common.amount') }}</th>\n                        <th>{{ t('balance.balance_after') }}</th>\n                        <th>{{ t('common.remark') }}</th>\n                        <th>{{ t('common.time') }}</th>\n                      </tr>\n                    </thead>\n                    <tbody>\n                      <tr v-for=\"(tx, idx) in balanceTxns.slice(0, 10)\" :key=\"tx.id\">\n                        <td class=\"billing-col-index\">{{ idx + 1 }}</td>\n                        <td><span :class=\"'badge badge-' + (tx.amount >= 0 ? 'active' : 'expired')\">{{ tx.type === 'charge' ? t('balance.charge_btn') : (tx.type === 'consume' ? t('balance.type_consume') : (tx.type === 'refund' ? t('order.refund') : tx.type)) }}</span></td>\n                        <td class=\"billing-col-amount\" :style=\"{ color: tx.amount >= 0 ? '#67c23a' : '#f56c6c' }\">{{ tx.amount >= 0 ? '+' : '' }}{{ formatMoney(tx.amount) }}</td>\n                        <td class=\"billing-col-amount\">{{ formatMoney(tx.balance_after) }}</td>\n                        <td>{{ tx.remark || '-' }}</td>\n                        <td class=\"billing-col-date\">{{ formatDate(tx.created_at) }}</td>\n                      </tr>\n                    </tbody>\n                  </table>\n                </div>\n              </div>",
      "css": "",
      "js": "return {}"
    },
    "style": {}
  },
  {
    "id": "mod-nodes-list",
    "type": "node_list",
    "title": "node.title",
    "page": "nodes",
    "visible": true,
    "data": {
      "info_title": "node.info_title",
      "info_text": "node.info_text",
      "group_by": "region",
      "show_protocol": true,
      "show_tier": true,
      "show_qr": true,
      "show_details": true,
      "columns": 3,
      "html": "<div v-if=\"mod.data && mod.data.info_text\" class=\"node-info-card\">\n                <div class=\"node-info-title\">{{ t(mod.data.info_title || 'node.info_title') }}</div>\n                <div class=\"node-info-content\">{{ t(mod.data.info_text) }}</div>\n              </div>\n              <div v-if=\"plansLoading\" class=\"empty-state\">\n                <el-icon class=\"empty-icon\" size=\"48\" class=\"is-loading\"><Loading /></el-icon>\n                <p class=\"empty-text\">{{ t('common.loading') }}</p>\n              </div>\n              <div v-else-if=\"allPlanNodes.length === 0\" class=\"empty-state\">\n                <el-icon class=\"empty-icon\" size=\"48\"><Inbox /></el-icon>\n                <p class=\"empty-text\">{{ t('node.empty') }}</p>\n              </div>\n              <div v-else>\n                <div v-for=\"planGroup in allPlanNodes\" :key=\"planGroup.plan_id\" class=\"plan-node-group\">\n                  <div class=\"plan-node-header\">\n                    <span class=\"plan-node-name\">{{ planGroup.plan_name }}</span>\n                    <span v-if=\"planGroup.active\" class=\"badge badge-active\">{{ t('account.plan') }}</span>\n                    <span v-else class=\"badge badge-none\">{{ t('node.not_purchased') }}</span>\n                  </div>\n                  <div class=\"node-grid\" :style=\"{ gridTemplateColumns: 'repeat(' + ((mod.data && mod.data.columns) || 3) + ', 1fr)' }\">\n                    <div v-for=\"node in planGroup.nodes\" :key=\"node.id\" class=\"node-card\" :class=\"{ disabled: !planGroup.active }\">\n                      <div class=\"node-name\">{{ node.name }}</div>\n                      <div class=\"node-tags\">\n                        <span v-if=\"node.protocol\" class=\"node-tag tag-protocol\">{{ node.protocol }}</span>\n                        <span v-if=\"node.transport\" class=\"node-tag tag-transport\">{{ node.transport }}</span>\n                      </div>\n                      <div v-if=\"planGroup.active && node.link\" class=\"node-actions\">\n                        <el-button size=\"small\" text @click=\"showNodeDetail(node)\">{{ t('common.view_detail') }}</el-button>\n                        <el-button size=\"small\" text @click=\"showNodeQR(node)\">{{ t('sub.qr_code') }}</el-button>\n                      </div>\n                      <div v-else class=\"node-locked\">\n                        <span class=\"lock-hint\">🔒 {{ t('node.need_plan') }}</span>\n                      </div>\n                    </div>\n                  </div>\n                </div>\n              </div>",
      "css": "",
      "js": "return {}"
    },
    "style": {}
  },
  {
    "id": "mod-tutorial-cards",
    "type": "tutorial_cards",
    "title": "tutorial.title",
    "page": "tutorial",
    "visible": true,
    "data": {
      "cards": [
        {
          "id": "windows",
          "name": "Windows",
          "icon": "windows",
          "desc": "tutorial.desc_windows",
          "color": "#0ea5e9",
          "bg": "linear-gradient(135deg, #0ea5e9, #0284c7)"
        },
        {
          "id": "android",
          "name": "Android",
          "icon": "android",
          "desc": "tutorial.desc_android",
          "color": "#84cc16",
          "bg": "linear-gradient(135deg, #84cc16, #65a30d)"
        },
        {
          "id": "apple",
          "name": "Apple",
          "icon": "apple",
          "desc": "tutorial.desc_apple",
          "color": "#1f2937",
          "bg": "linear-gradient(135deg, #374151, #1f2937)"
        },
        {
          "id": "mac",
          "name": "Mac",
          "icon": "desktop",
          "desc": "tutorial.desc_mac",
          "color": "#6b7280",
          "bg": "linear-gradient(135deg, #9ca3af, #6b7280)"
        },
        {
          "id": "linux",
          "name": "Linux",
          "icon": "linux",
          "desc": "tutorial.desc_linux",
          "color": "#f59e0b",
          "bg": "linear-gradient(135deg, #f59e0b, #d97706)"
        },
        {
          "id": "faq",
          "name": "FAQ",
          "icon": "question",
          "desc": "tutorial.desc_faq",
          "color": "#8b5cf6",
          "bg": "linear-gradient(135deg, #a78bfa, #8b5cf6)"
        },
        {
          "id": "update",
          "name": "Update",
          "icon": "sync",
          "desc": "tutorial.desc_update",
          "color": "#10b981",
          "bg": "linear-gradient(135deg, #10b981, #059669)"
        }
      ],
      "html": "<div class=\"card-header\"><span class=\"card-icon\">📥</span><span class=\"card-title\">{{ t(mod.title) }}</span></div>\n              <!-- Platform Tabs -->\n              <div class=\"tutorial-platform-tabs\">\n                <div\n                  v-for=\"card in (mod.data && mod.data.cards) || []\"\n                  :key=\"card.id\"\n                  class=\"tutorial-tab\"\n                  :class=\"{ active: selectedPlatform === card.id }\"\n                  @click=\"selectPlatform(card.id)\"\n                >\n                  {{ card.name }}\n                </div>\n              </div>\n              <!-- Platform Grid -->\n              <div class=\"tutorial-grid\">\n                <div\n                  v-for=\"card in (mod.data && mod.data.cards) || []\"\n                  :key=\"card.id\"\n                  class=\"tutorial-card\"\n                  :class=\"{ active: selectedPlatform === card.id }\"\n                  :style=\"{ background: card.bg || card.color }\"\n                  @click=\"selectPlatform(card.id)\"\n                >\n                  <div class=\"tutorial-card-icon\">{{ getTutorialIcon(card.icon) }}</div>\n                  <div class=\"tutorial-card-name\">{{ card.name }}</div>\n                  <div class=\"tutorial-card-desc\">{{ t(card.desc) }}</div>\n                </div>\n              </div>\n              <!-- Related Articles -->\n              <div v-if=\"selectedPlatform\" class=\"tutorial-articles\">\n                <div class=\"tutorial-articles-header\">\n                  <span class=\"tutorial-platform-name\">{{ getSelectedPlatformName() }}</span>\n                  <span class=\"tutorial-articles-count\">{{ getPlatformArticles(selectedPlatform).length }} {{ selectedPlatform === 'faq' ? t('article.faq_count') : t('tutorial.platform_articles') }}</span>\n                </div>\n                <div v-if=\"getPlatformArticles(selectedPlatform).length === 0\" class=\"empty-state\" style=\"padding: 24px;\">\n                  <span class=\"empty-icon\">📝</span>\n                  <p class=\"empty-text\">{{ selectedPlatform === 'faq' ? t('tutorial.no_faq') : t('tutorial.no_articles') }}</p>\n                </div>\n                <div v-else class=\"article-list tutorial-article-list\">\n                  <div\n                    class=\"article-item\"\n                    v-for=\"article in getPlatformArticles(selectedPlatform)\"\n                    :key=\"article.id\"\n                    @click=\"openArticle(article.slug)\"\n                  >\n                    <span class=\"article-cat-badge\">{{ articleCatText(article.category) }}</span>\n                    <div class=\"article-info\">\n                      <span class=\"article-title\">{{ article.title }}</span>\n                      <span class=\"logpkg-time\">{{ formatDate(article.published_at || article.updated_at) }}</span>\n                    </div>\n                    <span class=\"card-action\">{{ t('article.read') }}</span>\n                  </div>\n                </div>\n              </div>\n              <div v-else class=\"tutorial-select-hint\">\n                <span class=\"empty-icon\">👆</span>\n                <p class=\"empty-text\">{{ t('tutorial.select_platform') }}</p>\n              </div>",
      "css": "",
      "js": "return {}"
    },
    "style": {}
  },
  {
    "id": "mod-tickets",
    "type": "ticket_list",
    "title": "ticket.title",
    "page": "support",
    "visible": true,
    "data": {
      "new_btn_text": "ticket.new",
      "show_categories": true,
      "html": "<div class=\"card-header\"><el-icon class=\"card-icon\" size=\"20\"><Ticket /></el-icon><span class=\"card-title\">{{ t(mod.title) }}</span><span class=\"card-action\" @click=\"showTicketDialog = true\">{{ t((mod.data && mod.data.new_btn_text) || 'ticket.new') }}</span></div>\n              <div v-if=\"tickets.length === 0\" class=\"empty-state\"><el-icon class=\"empty-icon\" size=\"48\"><Inbox /></el-icon><p class=\"empty-text\">{{ t('ticket.empty') }}</p></div>\n<div v-else class=\"logpkg-list\">\n<div class=\"logpkg-item\" v-for=\"tk in tickets\" :key=\"tk.id\" @click=\"openTicket(tk.id)\" style=\"cursor: pointer\">\n<div class=\"logpkg-info\">\n<span class=\"logpkg-id\">#{{ tk.id }}</span>\n<span class=\"logpkg-time\">{{ tk.subject }}</span>\n<span :class=\"'badge badge-' + ticketStatusBadge(tk.status)\">{{ ticketStatusText(tk.status) }}</span>\n<span v-if=\"tk.status === 'replied'\" class=\"ticket-new-badge\">{{ t('ticket.new_reply') }}</span>\n</div>\n<div class=\"logpkg-actions\">\n<span class=\"logpkg-time\">{{ formatDate(tk.updated_at) }}</span>\n</div>\n</div>\n</div>",
      "css": "",
      "js": "return {}"
    },
    "style": {}
  },
  {
    "id": "mod-referral",
    "type": "referral",
    "title": "referral.title",
    "page": "referral",
    "visible": true,
    "data": {
      "description": "referral.description",
      "copy_btn_text": "referral.copy_link",
      "show_records": true,
      "records_limit": 10,
      "html": "<!-- Referral Link -->\n              <div class=\"referral-header\">\n                <div class=\"referral-title\"><el-icon size=\"20\" style=\"margin-right: 8px;\"><Gift /></el-icon>{{ t('referral.my_link') }}</div>\n                <div class=\"referral-meta\">\n                  <span class=\"referral-rate\">{{ t('referral.commission_rate') }}: {{ (referralStats && referralStats.commission_rate) ? referralStats.commission_rate + '%' : ((mod.data && mod.data.default_commission_rate) || '15') + '%' }}</span>\n                  <span class=\"referral-divider\">|</span>\n                  <span class=\"referral-mode\">{{ t('referral.commission_mode') }}: {{ (referralStats && referralStats.commission_mode === 'recurring') ? t('referral.mode_recurring') : ((mod.data && mod.data.default_commission_mode === 'recurring') ? t('referral.mode_recurring') : t('referral.mode_normal')) }}</span>\n                </div>\n              </div>\n              <div class=\"referral-link-wrap\" v-if=\"referralCode\">\n                <div class=\"referral-link\">{{ referralInviteUrl }}</div>\n                <el-button type=\"primary\" size=\"small\" @click=\"copyText(referralInviteUrl)\" class=\"referral-copy-btn\">\n                  <el-icon><DocumentCopy /></el-icon> {{ t('common.copy') }}\n                </el-button>\n              </div>\n              \n              <!-- Invite Code -->\n              <div class=\"referral-code-section\" v-if=\"referralCode\">\n                <div class=\"referral-code-row\">\n                  <span class=\"referral-code-label\">{{ t('referral.invite_code') }}</span>\n                  <span class=\"referral-code-value\">{{ referralCode.code }}</span>\n                  <el-tag size=\"small\" type=\"success\" effect=\"dark\" class=\"referral-status\">{{ t('referral.code_available') }}</el-tag>\n                </div>\n              </div>\n\n              <!-- Invite Records -->\n              <div class=\"referral-section\" v-if=\"referralRecords.length > 0\">\n                <div class=\"section-title\">{{ t('referral.invite_records') }}</div>\n                <div class=\"referral-table-wrap\">\n                  <table class=\"referral-table\">\n                    <thead>\n                      <tr>\n                        <th>#</th>\n                        <th>{{ t('account.email') }}</th>\n                        <th>{{ t('account.register_time') }}</th>\n                      </tr>\n                    </thead>\n                    <tbody>\n                      <tr v-for=\"(r, idx) in referralRecords.slice(0, (mod.data && mod.data.records_limit) || 10)\" :key=\"r.id\">\n                        <td>{{ idx + 1 }}</td>\n                        <td>{{ r.referee_email }}</td>\n                        <td>{{ formatDateTime(r.registered_at) }}</td>\n                      </tr>\n                    </tbody>\n                  </table>\n                </div>\n              </div>\n\n              <!-- Commission Records -->\n              <div class=\"referral-section\">\n                <div class=\"section-header\">\n                  <span class=\"section-title\">{{ t('referral.records') }}</span>\n                  <span class=\"section-subtitle\">{{ t('referral.withdraw_hint') }}</span>\n                  <el-button type=\"danger\" size=\"small\" class=\"withdraw-btn\" v-if=\"referralStats && referralStats.available_commission > 0\">{{ t('referral.withdraw') }}</el-button>\n                </div>\n                <div class=\"referral-table-wrap\">\n                  <table class=\"referral-table\">\n                    <thead>\n                      <tr>\n                        <th>#</th>\n                        <th>{{ t('referral.commission_time') }}</th>\n                        <th>{{ t('referral.consumer') }}</th>\n                        <th>{{ t('referral.consume_amount') }}</th>\n                        <th>{{ t('referral.commission_amount') }}</th>\n                        <th>{{ t('common.status') }}</th>\n                      </tr>\n                    </thead>\n                    <tbody>\n                      <tr v-for=\"(r, idx) in commissionRecords\" :key=\"r.id\">\n                        <td>{{ idx + 1 }}</td>\n                        <td>{{ formatDateTime(r.created_at) }}</td>\n                        <td>{{ r.consumer_email }}</td>\n                        <td>¥{{ r.amount }}</td>\n                        <td>¥{{ r.commission }}</td>\n                        <td>\n                          <el-tag size=\"small\" :type=\"r.status === 'available' ? 'success' : r.status === 'pending' ? 'warning' : 'info'\" effect=\"plain\">\n                            {{ r.status === 'available' ? t('referral.status_available') : r.status === 'pending' ? t('referral.status_pending') : t('referral.status_withdrawn') }}\n                          </el-tag>\n                        </td>\n                      </tr>\n                    </tbody>\n                  </table>\n                </div>\n                <div class=\"referral-summary\" v-if=\"referralStats\">\n                  {{ t('referral.total_commission') }} {{ formatMoney(referralStats.total_commission) }}（{{ referralStats.total_paid }}{{ t('referral.times' ) }}），满{{ (referralStats.min_withdraw_amount / 100).toFixed(0) }}元可提现\n                </div>\n              </div>\n\n              <!-- Withdrawal Records -->\n              <div class=\"referral-section\" v-if=\"withdrawRecords && withdrawRecords.length > 0\">\n                <div class=\"section-title\">{{ t('referral.withdraw_records') }}</div>\n                <div class=\"referral-table-wrap\">\n                  <table class=\"referral-table\">\n                    <thead>\n                      <tr>\n                        <th>#</th>\n                        <th>{{ t('referral.withdraw_date') }}</th>\n                        <th>{{ t('referral.withdraw_amount') }}</th>\n                        <th>{{ t('common.status') }}</th>\n                      </tr>\n                    </thead>\n                    <tbody>\n                      <tr v-for=\"(w, idx) in withdrawRecords\" :key=\"w.id\">\n                        <td>{{ idx + 1 }}</td>\n                        <td>{{ formatDateTime(w.created_at) }}</td>\n                        <td>{{ w.amount }}</td>\n                        <td>\n                          <el-tag size=\"small\" :type=\"w.status === 'completed' ? 'success' : w.status === 'pending' ? 'warning' : 'info'\" effect=\"plain\">\n                            {{ w.status === 'completed' ? t('referral.status_paid') : w.status === 'pending' ? t('common.processing') : t('order.status_cancelled') }}\n                          </el-tag>\n                        </td>\n                      </tr>\n                    </tbody>\n                  </table>\n                </div>\n              </div>",
      "css": "",
      "js": "return {}"
    },
    "style": {}
  },
  {
    "id": "mod-sec",
    "type": "security_settings",
    "title": "security.title",
    "page": "profile",
    "visible": true,
    "data": {
      "description": "security.description",
      "show_ip_whitelist": true,
      "show_password": true,
      "html": "<div class=\"card-header\"><span class=\"card-icon\">🔒</span><span class=\"card-title\">{{ t(mod.title) }}</span></div>\n              <p v-if=\"mod.data && mod.data.description\" class=\"card-desc\">{{ t(mod.data.description) }}</p>\n              <el-form v-if=\"mod.data && mod.data.show_ip_whitelist !== false\" label-position=\"top\" size=\"small\">\n                <el-form-item :label=\"t('security.ip_whitelist_full')\">\n                  <el-input v-model=\"settings.ip_whitelist\" :placeholder=\"t('security.ip_example')\" />\n                </el-form-item>\n                <el-form-item :label=\"t('security.device_limit_hint')\">\n                  <el-input-number v-model=\"settings.device_limit\" :min=\"0\" :max=\"100\" />\n                </el-form-item>\n                <el-button type=\"primary\" size=\"small\" @click=\"saveSettings\">{{ t('common.save') }}</el-button>\n              </el-form>\n              <div v-if=\"mod.data && mod.data.show_password !== false\">\n                <div class=\"card-header\" style=\"margin-top: 20px\"><span class=\"card-icon\">🔑</span><span class=\"card-title\">{{ t('security.password') }}</span></div>\n                <el-form label-position=\"top\" size=\"small\">\n<el-form-item :label=\"t('security.old_password')\"><el-input v-model=\"pwdForm.old_password\" type=\"password\" show-password /></el-form-item>\n<el-form-item :label=\"t('security.new_password')\"><el-input v-model=\"pwdForm.new_password\" type=\"password\" show-password /></el-form-item>\n<el-form-item :label=\"t('security.confirm_password')\"><el-input v-model=\"pwdForm.confirm_password\" type=\"password\" show-password /></el-form-item>\n                  <el-button type=\"primary\" size=\"small\" @click=\"changePassword\">{{ t('security.password') }}</el-button>\n                </el-form>\n              </div>",
      "css": "",
      "js": "return {}"
    },
    "style": {}
  },
  {
    "id": "mod-log",
    "type": "log_packages",
    "title": "log.title",
    "page": "logs",
    "visible": true,
    "data": {
      "description": "log.description",
      "btn_1h": "log.btn_1h",
      "btn_6h": "log.btn_6h",
      "btn_24h": "log.btn_24h",
      "html": "<div class=\"card-header\"><el-icon class=\"card-icon\" size=\"20\"><Box /></el-icon><span class=\"card-title\">{{ t(mod.title) }}</span></div>\n              <p v-if=\"mod.data && mod.data.description\" class=\"card-desc\">{{ t(mod.data.description) }}</p>\n              <p v-else class=\"card-desc\">{{ t('log.description') }}</p>\n              <div class=\"log-actions\">\n                <el-button type=\"primary\" size=\"small\" @click=\"confirmCreateLogPackage()\" :loading=\"logPacking\" :disabled=\"!hasEntryNodes\">{{ t('log.btn_pack') }}</el-button>\n                <span v-if=\"!hasEntryNodes\" class=\"logpkg-hint\" style=\"margin-left:8px\">{{ t('log.no_nodes') }}</span>\n              </div>\n              <div v-if=\"logPackages.length > 0\" style=\"margin-top: 16px\">\n                <div class=\"card-header\"><span class=\"card-icon\">📁</span><span class=\"card-title\">{{ t('log.package_list') }}</span><span class=\"card-action\" @click=\"fetchLogPackages\">{{ t('common.refresh') }}</span></div>\n                <div class=\"logpkg-list\">\n                  <div class=\"logpkg-item\" v-for=\"pkg in logPackages\" :key=\"pkg.id\">\n                    <div class=\"logpkg-info\">\n                      <span class=\"logpkg-id\">#{{ pkg.id }}</span>\n                      <span :class=\"'badge badge-' + pkg.status\">{{ pkgStatusText(pkg.status) }}</span>\n                      <span class=\"logpkg-time\">{{ formatDate(pkg.created_at) }}</span>\n                    </div>\n                    <div class=\"logpkg-actions\">\n                      <span v-if=\"pkg.file_size\" class=\"logpkg-size\">{{ formatBytes(pkg.file_size) }}</span>\n                      <el-button v-if=\"pkg.status === 'ready'\" size=\"small\" type=\"primary\" text @click=\"downloadLogPackage(pkg.id)\">{{ t('common.download') }}</el-button>\n                      <span v-if=\"pkg.status === 'processing'\" class=\"logpkg-hint\">{{ t('common.processing') }}...</span>\n                      <span v-if=\"pkg.error\" class=\"logpkg-error\">{{ pkg.error }}</span>\n                      <el-button v-if=\"pkg.status === 'ready' || pkg.status === 'failed'\" size=\"small\" type=\"danger\" text @click=\"deleteLogPackage(pkg.id)\">{{ t('common.delete') }}</el-button>\n                    </div>\n                  </div>\n                </div>\n              </div>",
      "css": "",
      "js": "return {}"
    },
    "style": {}
  },
  {
    "id": "mod-log-info",
    "type": "markdown",
    "title": "log.access_log",
    "page": "logs",
    "visible": true,
    "data": {
      "markdown": "log.access_log_md",
      "html": "<div v-if=\"mod.title\" class=\"card-header\"><span class=\"card-title\">{{ t(mod.title) }}</span></div>\n              <div class=\"md-render\" v-html=\"renderMd(t(mod.data && mod.data.markdown || ''))\"></div>",
      "css": "",
      "js": "return {}"
    },
    "style": {
      "align": "left"
    }
  }
]`

// defaultAuthRegionJSON is the default source code for the auth (login) region.
// Data fields use lang keys resolved by t() at runtime.
const defaultAuthRegionJSON = `{
  "html": "<div class=\"auth-wrapper\">\n  <div class=\"auth-bg\"></div>\n  <div class=\"auth-card glass\">\n    <div class=\"auth-logo\">\n      <img v-if=\"logoUrl\" :src=\"logoUrl\" class=\"logo-img\" />\n      <div v-else class=\"logo-icon\">⚡</div>\n    </div>\n    <h1 class=\"auth-title\">{{ siteName }}</h1>\n    <p class=\"auth-subtitle\">{{ t(data.subtitle) || t('auth.welcome_login') }}</p>\n    <el-tabs v-model=\"authTab\" class=\"auth-tabs\">\n      <el-tab-pane :label=\"t(data.login_label) || t('auth.login')\" name=\"login\">\n        <el-form class=\"auth-form\" @submit.prevent=\"handleLogin\">\n          <el-form-item>\n            <el-input v-model=\"loginEmail\" :placeholder=\"t(data.email_placeholder) || t('auth.email')\" prefix-icon=\"Message\" size=\"large\" />\n          </el-form-item>\n          <el-form-item>\n            <el-input v-model=\"loginPassword\" type=\"password\" :placeholder=\"t(data.password_placeholder) || t('auth.password')\" prefix-icon=\"Lock\" show-password size=\"large\" />\n          </el-form-item>\n          <el-button type=\"primary\" class=\"btn-block\" :loading=\"loading\" native-type=\"submit\" size=\"large\">{{ t(data.login_btn_text) || t('auth.login_btn') }}</el-button>\n        </el-form>\n      </el-tab-pane>\n      <el-tab-pane v-if=\"registrationEnabled\" :label=\"t(data.register_label) || t('auth.register')\" name=\"register\">\n        <el-form class=\"auth-form\" @submit.prevent=\"handleRegister\">\n          <el-form-item>\n            <el-input v-model=\"registerEmail\" :placeholder=\"t(data.email_placeholder) || t('auth.email')\" prefix-icon=\"Message\" size=\"large\" />\n          </el-form-item>\n          <el-form-item>\n            <el-input v-model=\"registerPassword\" type=\"password\" :placeholder=\"t(data.password_placeholder) || t('auth.password')\" prefix-icon=\"Lock\" show-password size=\"large\" />\n          </el-form-item>\n          <el-form-item>\n            <el-input v-model=\"registerReferral\" :placeholder=\"t(data.invite_placeholder) || t('auth.invite_code')\" prefix-icon=\"Promotion\" size=\"large\" />\n          </el-form-item>\n          <el-button type=\"primary\" class=\"btn-block\" :loading=\"loading\" native-type=\"submit\" size=\"large\">{{ t(data.register_btn_text) || t('auth.register_btn') }}</el-button>\n        </el-form>\n      </el-tab-pane>\n    </el-tabs>\n    <p v-if=\"data.footer_text\" class=\"auth-footer\">{{ data.footer_text }}</p>\n  </div>\n</div>\n",
  "css": ".auth-wrapper {\n  min-height: 100vh;\n  display: flex;\n  align-items: center;\n  justify-content: center;\n  position: relative;\n  padding: 20px;\n}\n.auth-bg {\n  position: fixed; inset: 0;\n  background: var(--bg-gradient);\n  z-index: 0;\n}\n.auth-card {\n  position: relative; z-index: 1;\n  border-radius: 20px;\n  padding: 40px 32px;\n  width: 100%;\n  max-width: 400px;\n  text-align: center;\n}\n.auth-logo { margin-bottom: 8px; }\n.logo-img { width: 64px; height: 64px; border-radius: 16px; }\n.logo-icon { font-size: 48px; line-height: 1; }\n.auth-title {\n  font-size: 24px;\n  font-weight: 700;\n  color: var(--text-primary);\n  margin-bottom: 4px;\n}\n.auth-subtitle {\n  font-size: 14px;\n  color: var(--text-secondary);\n  margin-bottom: 24px;\n}\n.auth-tabs { margin-bottom: 0; }\n.auth-form { margin-top: 16px; }\n.btn-block {\n  width: 100%;\n  background: linear-gradient(135deg, var(--accent), var(--accent-dark)) !important;\n  border: none !important;\n  font-weight: 700 !important;\n  box-shadow: 0 4px 14px rgba(37,99,235,0.25) !important;\n  transition: transform 0.15s, box-shadow 0.15s !important;\n}\n.btn-block:hover {\n  transform: translateY(-2px);\n  box-shadow: 0 6px 20px rgba(37,99,235,0.35) !important;\n}\n.auth-footer {\n  margin-top: 16px;\n  font-size: 12px;\n  color: var(--text-muted);\n}\n",
  "js": "const { computed, ref } = ctx;\n\n// Sync authTab both ways\nconst authTab = computed({\n  get: () => ctx.authTab.value,\n  set: (v) => { ctx.authTab.value = v; }\n});\n\n// Create local reactive refs for form fields\nconst loginEmail = ref('');\nconst loginPassword = ref('');\nconst registerEmail = ref('');\nconst registerPassword = ref('');\nconst registerReferral = ref(ctx.registerForm.value.referral_code || '');\n\n// Sync local refs with parent form on submit\nconst handleLogin = function() {\n  ctx.loginForm.value.email = loginEmail.value;\n  ctx.loginForm.value.password = loginPassword.value;\n  ctx.handleLogin();\n};\nconst handleRegister = function() {\n  ctx.registerForm.value.email = registerEmail.value;\n  ctx.registerForm.value.password = registerPassword.value;\n  ctx.registerForm.value.referral_code = registerReferral.value;\n  ctx.handleRegister();\n};\n\nreturn {\n  loginEmail,\n  loginPassword,\n  registerEmail,\n  registerPassword,\n  registerReferral,\n  authTab,\n  loading: ctx.loading,\n  registrationEnabled: ctx.registrationEnabled,\n  logoUrl: ctx.logoUrl,\n  siteName: ctx.siteName,\n  handleLogin,\n  handleRegister,\n  t: ctx.t,\n  data: ctx.data\n};\n",
  "data": {
    "subtitle": "auth.welcome_login",
    "logo_emoji": "⚡",
    "login_label": "auth.login",
    "register_label": "auth.register",
    "login_btn_text": "auth.login_btn",
    "register_btn_text": "auth.register_btn",
    "email_placeholder": "auth.email",
    "password_placeholder": "auth.password",
    "invite_placeholder": "auth.invite_code",
    "footer_text": ""
  }
}`

// defaultRegionsJSON wraps all editable regions into a single JSON object.
// Currently only "auth" (login page) is supported; more regions can be added.
const defaultRegionsJSON = `{"auth":` + defaultAuthRegionJSON + `}`
