package portal

// defaultNavigationJSON is the default navigation items for a new portal.
// These match what the frontend PortalManagement.vue uses as defaultNavigation.
const defaultNavigationJSON = `[
  {"id":"nav-overview","label":"概览","icon":"📊","target":"overview","href":"","enabled":true,"open_external":false},
  {"id":"nav-subscription","label":"订阅","icon":"📋","target":"subscription","href":"","enabled":true,"open_external":false},
  {"id":"nav-store","label":"商城","icon":"💎","target":"store","href":"","enabled":true,"open_external":false},
  {"id":"nav-security","label":"安全","icon":"🔒","target":"security","href":"","enabled":true,"open_external":false},
  {"id":"nav-support","label":"支持","icon":"💬","target":"support","href":"","enabled":true,"open_external":false},
  {"id":"nav-logs","label":"日志","icon":"📦","target":"logs","href":"","enabled":true,"open_external":false}
]`

// defaultModulesJSON is the default module layout for a new portal.
// Each module has a "page" field linking it to a navigation target.
// All content is module-driven — no hardcoded tabs in the template.
const defaultModulesJSON = `[
  {
    "id":"mod-hero","type":"hero","title":"欢迎","page":"overview","visible":true,
    "data":{"eyebrow":"WELCOME BACK","heading":"欢迎回来","description":"管理您的订阅与服务","button_text":"查看订阅","button_target":"subscription"},
    "style":{"variant":"primary","align":"left"}
  },
  {
    "id":"mod-account","type":"account_info","title":"账户信息","page":"overview","visible":true,
    "data":{},"style":{}
  },
  {
    "id":"mod-traffic","type":"traffic_ring","title":"流量使用","page":"overview","visible":true,
    "data":{},"style":{}
  },
  {
    "id":"md-notice","type":"notice","title":"站点公告","page":"overview","visible":true,
    "data":{"title":"📢 公告","markdown":"欢迎使用 **CoreFusion** 订阅管理平台！\n\n- 支持 VLESS / VMess / Trojan / TUIC 等多种协议\n- 一键复制订阅链接\n- 流量使用实时查看\n"},
    "style":{"variant":"info"}
  },
  {
    "id":"mod-sub","type":"subscription_detail","title":"订阅信息","page":"subscription","visible":true,
    "data":{"description":"查看您的订阅链接和状态","copy_text":"复制","qr_text":"二维码","show_status":true,"show_rotate":true},
    "style":{}
  },
  {
    "id":"mod-sec","type":"security_settings","title":"安全设置","page":"security","visible":true,
    "data":{"description":"管理您的账户安全设置","show_ip_whitelist":true,"show_password":true},
    "style":{}
  },
  {
    "id":"mod-log","type":"log_packages","title":"日志打包","page":"logs","visible":true,
    "data":{"description":"请求打包最近日志，系统将在后台收集并压缩为可下载文件。","btn_1h":"打包最近1小时","btn_6h":"最近6小时","btn_24h":"最近24小时"},
    "style":{}
  },
  {
    "id":"mod-log-info","type":"markdown","title":"访问日志","page":"logs","visible":true,
    "data":{"markdown":"## 访问日志\n\n在此页面查看您的连接日志和访问记录。\n\n> 日志数据由客户端实时拉取。"},
    "style":{"align":"left"}
  },
  {
    "id":"mod-plan-list","type":"plan_list","title":"套餐列表","page":"store","visible":true,
    "data":{"show_price":true,"show_features":true,"show_quota":true,"button_text":"购买","buy_text":"立即购买","renew_text":"续费","columns":2},
    "style":{}
  },
  {
    "id":"mod-balance","type":"balance_display","title":"账户余额","page":"store","visible":true,
    "data":{"show_transactions":true,"charge_btn_text":"去充值","tx_limit":10},
    "style":{}
  },
  {
    "id":"mod-charge","type":"charge_panel","title":"在线充值","page":"store","visible":false,
    "data":{"description":"选择充值方式，输入金额后跳转到支付页面。","show_orders":true,"orders_limit":5,"card_placeholder":"输入充值卡号","card_btn_text":"兑换"},
    "style":{}
  },
  {
    "id":"mod-referral","type":"referral","title":"邀请返利","page":"support","visible":true,
    "data":{"description":"邀请好友注册并消费，即可获得返利佣金。","copy_btn_text":"复制邀请链接","show_records":true,"records_limit":10},
    "style":{}
  },
  {
    "id":"mod-tickets","type":"ticket_list","title":"我的工单","page":"support","visible":true,
    "data":{"new_btn_text":"新建工单","show_categories":true},
    "style":{}
  },
  {
    "id":"mod-articles","type":"article_list","title":"教程文章","page":"support","visible":true,
    "data":{"show_category_filter":true,"default_category":""},
    "style":{}
  },
  {
    "id":"mod-shared-accts","type":"shared_accounts","title":"共享账号","page":"support","visible":true,
    "data":{"description":"以下共享账号可供使用，点击查看凭据。","apply_btn_text":"查看凭据","repair_btn_text":"请求修复"},
    "style":{}
  }
]`
