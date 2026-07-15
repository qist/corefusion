package portal

// defaultNavigationJSON is the default navigation items for a new portal.
// These match what the frontend PortalManagement.vue uses as defaultNavigation.
const defaultNavigationJSON = `[
  {"id":"nav-overview","label":"概览","icon":"📊","target":"overview","href":"","enabled":true,"open_external":false},
  {"id":"nav-subscription","label":"订阅","icon":"📋","target":"subscription","href":"","enabled":true,"open_external":false},
  {"id":"nav-security","label":"安全","icon":"🔒","target":"security","href":"","enabled":true,"open_external":false},
  {"id":"nav-logs","label":"日志","icon":"📦","target":"logs","href":"","enabled":true,"open_external":false}
]`

// defaultModulesJSON is the default module layout for a new portal.
// Each module has a "page" field linking it to a navigation target.
// All content is module-driven — no hardcoded tabs in the template.
const defaultModulesJSON = `[
  {
    "id":"mod-hero","type":"hero","title":"欢迎","page":"overview","visible":true,
    "data":{"eyebrow":"CORE FUSION","heading":"欢迎回来","description":"查看您的流量使用和订阅状态","button_text":"查看订阅","button_target":"subscription"},
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
  }
]`
