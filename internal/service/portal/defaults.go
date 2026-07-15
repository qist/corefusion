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
// These match what the frontend PortalManagement.vue uses as defaultModules.
const defaultModulesJSON = `[
  {
    "id":"mod-hero","type":"hero","title":"欢迎","visible":true,
    "data":{"eyebrow":"CORE FUSION","heading":"欢迎回来","description":"查看您的流量使用和订阅状态","button_text":"查看订阅","button_target":"subscription"},
    "style":{"variant":"primary","align":"left"}
  },
  {
    "id":"md-notice","type":"notice","title":"站点公告","visible":true,
    "data":{"title":"📢 公告","markdown":"欢迎使用 **CoreFusion** 订阅管理平台！\n\n- 支持 VLESS / VMess / Trojan / TUIC 等多种协议\n- 一键复制订阅链接\n- 流量使用实时查看\n"},
    "style":{"variant":"info"}
  },
  {
    "id":"mod-stats","type":"account_stats","title":"流量使用","visible":true,
    "data":{},
    "style":{"layout":"grid"}
  },
  {
    "id":"mod-sub","type":"subscription","title":"订阅信息","visible":true,
    "data":{"title":"订阅信息","description":"查看您的订阅链接和状态","button_text":"复制订阅链接"},
    "style":{}
  }
]`
