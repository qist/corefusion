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
// Titles and text fields use lang keys (e.g. "traffic.title") resolved by t() at runtime.
// Long custom content (notice_text, info_text, markdown) remains as editable content.
const defaultModulesJSON = `[
  {
    "id":"mod-hero","type":"hero","title":"hero.welcome","page":"overview","visible":true,
    "data":{"eyebrow":"WELCOME","heading":"hero.welcome_back","description":"hero.manage_sub","button_text":"hero.manage_sub","button_target":"store"},
    "style":{"variant":"primary","align":"left"}
  },
  {
    "id":"mod-account","type":"account_info","title":"account.title","page":"overview","visible":true,
    "data":{"columns":2},"style":{}
  },
  {
    "id":"mod-traffic","type":"traffic_ring","title":"traffic.title","page":"overview","visible":true,
    "data":{},"style":{}
  },
  {
    "id":"mod-client-download","type":"client_download","title":"client.title","page":"overview","visible":true,
    "data":{
      "platforms":[
        {"id":"windows","name":"Windows","icon":"windows","url":"/downloads/windows"},
        {"id":"ios","name":"iOS","icon":"apple","url":"/downloads/ios"},
        {"id":"android","name":"Android","icon":"android","url":"/downloads/android"},
        {"id":"mac","name":"Mac","icon":"desktop","url":"/downloads/mac"},
        {"id":"linux","name":"Linux","icon":"linux","url":"/downloads/linux"},
        {"id":"browser","name":"Browser","icon":"chrome","url":"/downloads/browser"},
        {"id":"other","name":"Other","icon":"ellipsis","url":"/downloads/other"}
      ],
      "select_text":"common.other",
      "hint_text":"tutorial.select_platform"
    },
    "style":{}
  },
  {
    "id":"mod-sub","type":"subscription_detail","title":"sub.title","page":"overview","visible":true,
    "data":{
      "description":"sub.description",
      "copy_text":"common.copy",
      "qr_text":"sub.qr_code",
      "show_status":true,
      "show_rotate":true,
      "actions":[
        {"text":"sub.buy_plan","type":"primary","action":"navigate","target":"store"},
        {"text":"sub.whitelist","type":"info","action":"dialog","target":"whitelist"},
        {"text":"sub.more_subs","type":"info","action":"dialog","target":"more_subs"},
        {"text":"sub.reset","type":"danger","action":"confirm","target":"reset_sub"}
      ]
    },
    "style":{}
  },
  {
    "id":"mod-streaming","type":"streaming_status","title":"streaming.title","page":"overview","visible":true,
    "data":{},
    "style":{}
  },
  {
    "id":"mod-plan-list","type":"plan_list","title":"plan.title","page":"store","visible":true,
    "data":{
      "notice_text":"plan.notice_text",
      "notice_subtext":"plan.notice_subtext",
      "show_price":true,
      "show_features":true,
      "show_quota":true,
      "button_text":"common.buy",
      "buy_text":"common.buy",
      "renew_text":"common.buy",
      "columns":2
    },
    "style":{}
  },
  {
    "id":"mod-orders","type":"order_list","title":"order.title","page":"billing","visible":true,
    "data":{
      "notice_text":"order.notice",
      "notice_highlight":"order.notice_highlight",
      "notice_subtext":"order.notice_subtext",
      "columns":["order.id","order.id","order.service","common.amount","order.date","order.expire","common.status"],
      "limit":20
    },
    "style":{}
  },
  {
    "id":"mod-nodes-list","type":"node_list","title":"node.title","page":"nodes","visible":true,
    "data":{
      "info_title":"node.info_title",
      "info_text":"node.info_text",
      "group_by":"region",
      "show_protocol":true,
      "show_tier":true,
      "show_qr":true,
      "show_details":true,
      "columns":3
    },
    "style":{}
  },
  {
    "id":"mod-tutorial-cards","type":"tutorial_cards","title":"tutorial.title","page":"tutorial","visible":true,
    "data":{
      "cards":[
        {"id":"windows","name":"Windows","icon":"windows","desc":"tutorial.desc_windows","color":"#0ea5e9","bg":"linear-gradient(135deg, #0ea5e9, #0284c7)"},
        {"id":"android","name":"Android","icon":"android","desc":"tutorial.desc_android","color":"#84cc16","bg":"linear-gradient(135deg, #84cc16, #65a30d)"},
        {"id":"apple","name":"Apple","icon":"apple","desc":"tutorial.desc_apple","color":"#1f2937","bg":"linear-gradient(135deg, #374151, #1f2937)"},
        {"id":"mac","name":"Mac","icon":"desktop","desc":"tutorial.desc_mac","color":"#6b7280","bg":"linear-gradient(135deg, #9ca3af, #6b7280)"},
        {"id":"linux","name":"Linux","icon":"linux","desc":"tutorial.desc_linux","color":"#f59e0b","bg":"linear-gradient(135deg, #f59e0b, #d97706)"},
        {"id":"faq","name":"FAQ","icon":"question","desc":"tutorial.desc_faq","color":"#8b5cf6","bg":"linear-gradient(135deg, #a78bfa, #8b5cf6)"},
        {"id":"update","name":"Update","icon":"sync","desc":"tutorial.desc_update","color":"#10b981","bg":"linear-gradient(135deg, #10b981, #059669)"}
      ]
    },
    "style":{}
  },
  {
    "id":"mod-tickets","type":"ticket_list","title":"ticket.title","page":"support","visible":true,
    "data":{"new_btn_text":"ticket.new","show_categories":true},
    "style":{}
  },
  {
    "id":"mod-referral","type":"referral","title":"referral.title","page":"referral","visible":true,
    "data":{"description":"referral.description","copy_btn_text":"referral.copy_link","show_records":true,"records_limit":10},
    "style":{}
  },
  {
    "id":"mod-sec","type":"security_settings","title":"security.title","page":"profile","visible":true,
    "data":{"description":"security.description","show_ip_whitelist":true,"show_password":true},
    "style":{}
  },
  {
    "id":"mod-log","type":"log_packages","title":"log.title","page":"logs","visible":true,
    "data":{"description":"log.description","btn_1h":"log.btn_1h","btn_6h":"log.btn_6h","btn_24h":"log.btn_24h"},
    "style":{}
  },
  {
    "id":"mod-log-info","type":"markdown","title":"log.access_log","page":"logs","visible":true,
    "data":{"markdown":"log.access_log_md"},
    "style":{"align":"left"}
  }
]`

// defaultAuthRegionJSON is the default source code for the auth (login) region.
// Data fields use lang keys resolved by t() at runtime.
const defaultAuthRegionJSON = `{
  "html": "<div class=\"auth-wrapper\">\n  <div class=\"auth-bg\"></div>\n  <div class=\"auth-card glass\">\n    <div class=\"auth-logo\">\n      <img v-if=\"logoUrl\" :src=\"logoUrl\" class=\"logo-img\" />\n      <div v-else class=\"logo-icon\">⚡</div>\n    </div>\n    <h1 class=\"auth-title\">{{ siteName }}</h1>\n    <p class=\"auth-subtitle\">{{ t(data.subtitle) || t('auth.welcome_login') }}</p>\n    <el-tabs v-model=\"authTab\" class=\"auth-tabs\">\n      <el-tab-pane :label=\"t(data.login_label) || t('auth.login')\" name=\"login\">\n        <el-form class=\"auth-form\" @submit.prevent=\"handleLogin\">\n          <el-form-item>\n            <el-input v-model=\"loginEmail\" :placeholder=\"t(data.email_placeholder) || t('auth.email')\" prefix-icon=\"Message\" size=\"large\" />\n          </el-form-item>\n          <el-form-item>\n            <el-input v-model=\"loginPassword\" type=\"password\" :placeholder=\"t(data.password_placeholder) || t('auth.password')\" prefix-icon=\"Lock\" show-password size=\"large\" />\n          </el-form-item>\n          <el-button type=\"primary\" class=\"btn-block\" :loading=\"loading\" native-type=\"submit\" size=\"large\">{{ t(data.login_btn_text) || t('auth.login_btn') }}</el-button>\n        </el-form>\n      </el-tab-pane>\n      <el-tab-pane v-if=\"registrationEnabled\" :label=\"t(data.register_label) || t('auth.register')\" name=\"register\">\n        <el-form class=\"auth-form\" @submit.prevent=\"handleRegister\">\n          <el-form-item>\n            <el-input v-model=\"registerEmail\" :placeholder=\"t(data.email_placeholder) || t('auth.email')\" prefix-icon=\"Message\" size=\"large\" />\n          </el-form-item>\n          <el-form-item>\n            <el-input v-model=\"registerPassword\" type=\"password\" :placeholder=\"t(data.password_placeholder) || t('auth.password')\" prefix-icon=\"Lock\" show-password size=\"large\" />\n          </el-form-item>\n          <el-form-item>\n            <el-input v-model=\"registerReferral\" :placeholder=\"t(data.invite_placeholder) || t('auth.invite_code')\" prefix-icon=\"Promotion\" size=\"large\" />\n          </el-form-item>\n          <el-button type=\"primary\" class=\"btn-block\" :loading=\"loading\" native-type=\"submit\" size=\"large\">{{ t(data.register_btn_text) || t('auth.register_btn') }}</el-button>\n        </el-form>\n      </el-tab-pane>\n    </el-tabs>\n    <p v-if=\"data.footer_text\" class=\"auth-footer\">{{ data.footer_text }}</p>\n  </div>\n</div>\n",
  "css": ".auth-wrapper {\n  min-height: 100vh;\n  display: flex;\n  align-items: center;\n  justify-content: center;\n  position: relative;\n  padding: 20px;\n}\n.auth-bg {\n  position: fixed; inset: 0;\n  background: var(--bg-gradient);\n  z-index: 0;\n}\n.auth-card {\n  position: relative; z-index: 1;\n  border-radius: 20px;\n  padding: 40px 32px;\n  width: 100%;\n  max-width: 400px;\n  text-align: center;\n}\n.auth-logo { margin-bottom: 8px; }\n.logo-img { width: 64px; height: 64px; border-radius: 16px; }\n.logo-icon { font-size: 48px; line-height: 1; }\n.auth-title {\n  font-size: 24px;\n  font-weight: 700;\n  color: var(--text-primary);\n  margin-bottom: 4px;\n}\n.auth-subtitle {\n  font-size: 14px;\n  color: var(--text-secondary);\n  margin-bottom: 24px;\n}\n.auth-tabs { margin-bottom: 0; }\n.auth-form { margin-top: 16px; }\n.btn-block {\n  width: 100%;\n  background: linear-gradient(135deg, var(--accent), var(--accent-dark)) !important;\n  border: none !important;\n  font-weight: 700 !important;\n  box-shadow: 0 4px 14px rgba(37,99,235,0.25) !important;\n  transition: transform 0.15s, box-shadow 0.15s !important;\n}\n.btn-block:hover {\n  transform: translateY(-2px);\n  box-shadow: 0 6px 20px rgba(37,99,235,0.35) !important;\n}\n.auth-footer {\n  margin-top: 16px;\n  font-size: 12px;\n  color: var(--text-muted);\n}\n",
  "js": "const { computed, ref } = ctx;\n\n// Sync authTab both ways\nconst authTab = computed({\n  get: () => ctx.authTab.value,\n  set: (v) => { ctx.authTab.value = v; }\n});\n\n// Create local reactive refs for form fields\nconst loginEmail = ref('');\nconst loginPassword = ref('');\nconst registerEmail = ref('');\nconst registerPassword = ref('');\nconst registerReferral = ref('');\n\n// Sync local refs with parent form on submit\nconst handleLogin = function() {\n  ctx.loginForm.value.email = loginEmail.value;\n  ctx.loginForm.value.password = loginPassword.value;\n  ctx.handleLogin();\n};\nconst handleRegister = function() {\n  ctx.registerForm.value.email = registerEmail.value;\n  ctx.registerForm.value.password = registerPassword.value;\n  ctx.registerForm.value.referral_code = registerReferral.value;\n  ctx.handleRegister();\n};\n\nreturn {\n  loginEmail,\n  loginPassword,\n  registerEmail,\n  registerPassword,\n  registerReferral,\n  authTab,\n  loading: ctx.loading,\n  registrationEnabled: ctx.registrationEnabled,\n  logoUrl: ctx.logoUrl,\n  siteName: ctx.siteName,\n  handleLogin,\n  handleRegister,\n  t: ctx.t,\n  data: ctx.data\n};\n",
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
