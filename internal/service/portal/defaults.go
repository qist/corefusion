package portal

// defaultNavigationJSON is the default navigation items for a new portal.
// Navigation structure follows qist's sidebar layout, with logs section retained.
const defaultNavigationJSON = `[
  {"id":"nav-home","label":"网站首页","icon":"🏠","target":"overview","href":"","enabled":true,"open_external":false},
  {"id":"nav-services","label":"购买服务","icon":"🛒","target":"store","href":"","enabled":true,"open_external":false},
  {"id":"nav-billing","label":"我的账单","icon":"💰","target":"billing","href":"","enabled":true,"open_external":false},
  {"id":"nav-nodes","label":"线路列表","icon":"📋","target":"nodes","href":"","enabled":true,"open_external":false},
  {"id":"nav-tutorial","label":"使用教程","icon":"📚","target":"tutorial","href":"","enabled":true,"open_external":false},
  {"id":"nav-tickets","label":"发起工单","icon":"🎫","target":"support","href":"","enabled":true,"open_external":false},
  {"id":"nav-referral","label":"推广邀请","icon":"🎁","target":"referral","href":"","enabled":true,"open_external":false},
  {"id":"nav-profile","label":"个人中心","icon":"👤","target":"profile","href":"","enabled":true,"open_external":false},
  {"id":"nav-logs","label":"日志","icon":"📦","target":"logs","href":"","enabled":true,"open_external":false}
]`

// defaultModulesJSON is the default module layout for a new portal.
// Each module has a "page" field linking it to a navigation target.
// All content is module-driven — no hardcoded tabs in the template.
const defaultModulesJSON = `[
  {
    "id":"mod-hero","type":"hero","title":"欢迎","page":"overview","visible":true,
    "data":{"eyebrow":"WELCOME","heading":"欢迎回来","description":"管理您的订阅与服务","button_text":"查看订阅","button_target":"store"},
    "style":{"variant":"primary","align":"left"}
  },
  {
    "id":"mod-account","type":"account_info","title":"帐号信息","page":"overview","visible":true,
    "data":{"columns":2},"style":{}
  },
  {
    "id":"mod-traffic","type":"traffic_ring","title":"流量使用","page":"overview","visible":true,
    "data":{},"style":{}
  },
  {
    "id":"mod-client-download","type":"client_download","title":"客户端下载","page":"overview","visible":true,
    "data":{
      "platforms":[
        {"id":"windows","name":"Windows","icon":"windows","url":"/downloads/windows"},
        {"id":"ios","name":"iOS","icon":"apple","url":"/downloads/ios"},
        {"id":"android","name":"安卓","icon":"android","url":"/downloads/android"},
        {"id":"mac","name":"Mac","icon":"desktop","url":"/downloads/mac"},
        {"id":"linux","name":"Linux","icon":"linux","url":"/downloads/linux"},
        {"id":"browser","name":"浏览器","icon":"chrome","url":"/downloads/browser"},
        {"id":"other","name":"其他","icon":"ellipsis","url":"/downloads/other"}
      ],
      "select_text":"选择平台",
      "hint_text":"请点击上方标签选择你的操作系统"
    },
    "style":{}
  },
  {
    "id":"mod-sub","type":"subscription_detail","title":"订阅链接","page":"overview","visible":true,
    "data":{
      "description":"查看您的订阅链接和状态",
      "copy_text":"复制",
      "qr_text":"二维码",
      "show_status":true,
      "show_rotate":true,
      "actions":[
        {"text":"购买套餐","type":"primary","action":"navigate","target":"store"},
        {"text":"白名单管理","type":"info","action":"dialog","target":"whitelist"},
        {"text":"更多订阅","type":"info","action":"dialog","target":"more_subs"},
        {"text":"重置订阅","type":"danger","action":"confirm","target":"reset_sub"}
      ]
    },
    "style":{}
  },
  {
    "id":"mod-streaming","type":"streaming_status","title":"流媒体解锁","page":"overview","visible":true,
    "data":{},
    "style":{}
  },
  {
    "id":"mod-plan-list","type":"plan_list","title":"套餐列表","page":"store","visible":true,
    "data":{
      "notice_text":"套餐可叠加购买，剩余有效期自动叠加。如需升级请直接购买新套餐，剩余有效期会根据等级差价换算，然后再叠加(新套餐立即生效)。以下套餐只能个人使用，禁止倒卖/组团拼车购买等行为，发现则封号处理（如需多人使用请联系客服定制团队套餐）。",
      "notice_subtext":"福利：每月赠送10-60G应急流量（结合VIP等级和账户累计续费天数而增加），首页点领福利可获取",
      "show_price":true,
      "show_features":true,
      "show_quota":true,
      "button_text":"购买",
      "buy_text":"购买",
      "renew_text":"购买",
      "columns":2
    },
    "style":{}
  },
  {
    "id":"mod-orders","type":"order_list","title":"我的账单","page":"billing","visible":true,
    "data":{
      "notice_text":"支付后自动开通，帐号信息在网站首页，新用户请到网站首页查看使用教程。续费用户请在客户端墙上重新订阅即可！",
      "notice_highlight":"请保护好自己！",
      "notice_subtext":"付款后5分钟未有显示已发行，直接联系在线客服补单",
      "columns":["编号","编号","服务","金额","购买日期","到期时间","状态"],
      "limit":20
    },
    "style":{}
  },
  {
    "id":"mod-nodes-list","type":"node_list","title":"线路列表","page":"nodes","visible":true,
    "data":{
      "info_title":"线路列表",
      "info_text":"线路名字说明：\n【主打线路】R系列：主力线路(中转|网速快|推荐)\n【特殊线路】V系列：IEPL专线；S系列：IPLC低延迟游戏级专线\n【特殊线路】D系列：直连线路(R/V/S系列只能从大陆连接，D系列境外可访问)\n【流量倍率】名字末尾带x1.5为1.5倍率，不带x的为默认1倍率，计算方法：账号使用流量 = 实际流量 * 倍率\n【Netflix解锁】全部香港/新加坡/美国线路均可解锁非自制剧\n【OpenAI解锁】美国/香港均可解锁，其中IPLC专线采用独立通道解锁，响应更快。请勿用于批量注册\n【Gemini解锁】美国均可解锁\n【其他AI解锁】美国均可解锁\n【Disney+解锁】港/美基本都可解锁",
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
    "id":"mod-tutorial-cards","type":"tutorial_cards","title":"下载与教程","page":"tutorial","visible":true,
    "data":{
      "cards":[
        {"id":"windows","name":"Windows","icon":"windows","desc":"适用于 Windows 10 及以上版本","color":"#0ea5e9","bg":"linear-gradient(135deg, #0ea5e9, #0284c7)"},
        {"id":"android","name":"Android","icon":"android","desc":"适用于 Android 5.0 及以上版本，安卓手机和平板设备","color":"#84cc16","bg":"linear-gradient(135deg, #84cc16, #65a30d)"},
        {"id":"apple","name":"Apple","icon":"apple","desc":"适用于 iOS/iPadOS/","color":"#1f2937","bg":"linear-gradient(135deg, #374151, #1f2937)"},
        {"id":"mac","name":"Mac","icon":"desktop","desc":"适用于 M 系列芯片及 macOS 12+","color":"#6b7280","bg":"linear-gradient(135deg, #9ca3af, #6b7280)"},
        {"id":"linux","name":"Linux","icon":"linux","desc":"适用于所有主流 Linux 发行版","color":"#f59e0b","bg":"linear-gradient(135deg, #f59e0b, #d97706)"},
        {"id":"faq","name":"FAQ","icon":"question","desc":"常见问题与操作指南，助您快速上手","color":"#8b5cf6","bg":"linear-gradient(135deg, #a78bfa, #8b5cf6)"},
        {"id":"update","name":"更新订阅","icon":"sync","desc":"所有客户端更新订阅方法汇总，一页查清","color":"#10b981","bg":"linear-gradient(135deg, #10b981, #059669)"}
      ]
    },
    "style":{}
  },
  {
    "id":"mod-tickets","type":"ticket_list","title":"我的工单","page":"support","visible":true,
    "data":{"new_btn_text":"新建工单","show_categories":true},
    "style":{}
  },
  {
    "id":"mod-referral","type":"referral","title":"推广邀请","page":"referral","visible":true,
    "data":{"description":"邀请好友注册并消费，即可获得返利佣金。","copy_btn_text":"复制邀请链接","show_records":true,"records_limit":10},
    "style":{}
  },
  {
    "id":"mod-sec","type":"security_settings","title":"安全设置","page":"profile","visible":true,
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
