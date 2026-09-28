# Google OAuth 正式授权（应用验证）指南

> 适用范围：CoreFusion 门户的 Google 第三方登录（`internal/service/oauth`，scopes：`openid`、`email`、`profile`）。
> GitHub 对照说明见文末（GitHub 无需验证流程）。

## 1. 测试模式 vs 正式（发布）模式

Google OAuth 同意屏幕（OAuth consent screen）有两种发布状态：

| 状态 | 谁能登录 | 是否需要验证 |
|---|---|---|
| **测试（Testing）** | 仅"测试用户"列表里的 Google 账号 | 否 |
| **正式（In production）** | 任何 Google 账号 | **是**（见下） |

面板"系统设置 → OAuth"启用后，门户登录页会对所有访客显示 Google 按钮；
若此时应用仍是测试状态，未加入测试用户列表的访客会看到
`Error 403: access_denied`（应用未验证 / 您未被列为测试用户）。
要对外开放 Google 登录，**必须把应用切到正式状态并通过验证**。

## 2. 正式授权（验证）的硬性要求

Google 审核 OAuth 应用（非受限 scope 的"基础验证"）要求：

1. **应用主页（Home page）**：公共可访问的 URL，须与授权域名一致（同域或隶属域名）。
2. **隐私权政策（Privacy Policy）链接**：必须**公共可访问**（无需登录即可打开），
   说明你收集哪些数据、如何使用/存储/删除。Google 登录场景至少要覆盖：
   收集的 Google 账号信息（邮箱、姓名、头像）、用途（创建/登录门户账号）、不向第三方出售、
   用户可注销/删除账号及数据的途径。
3. **服务条款（Terms of Service）链接**：公共可访问，说明服务使用规则。
4. **授权域名（Authorized domains）**：在同意屏幕里登记应用所属域名（如 `example.com`），
   主页/隐私/条款链接的域名必须是其本身或其子域。
5. **应用徽标（可选但建议）**：120×120px。
6. **scopes 说明**：本项目只用 `openid / email / profile`（非受限、非敏感 scope），
   属于"基本验证"——不需要逐 scope 论证，也**不需要第三方安全评估**。
   （受限 scope 如 Gmail 全量读写才需要，本项目不涉及。）

验证入口：[Google Cloud Console](https://console.cloud.google.com/) →
**API 和服务 → OAuth 同意屏幕 → Publish App（切换到正式）→ Submit for verification**。
基础验证一般数日内完成；未验证期间仍可先用测试模式运营。

## 3. 本项目需要配置的位置

### 3.1 回调地址（Authorized redirect URI）

回调按"发起登录的域名"动态生成（见 `internal/api/handlers/oauth/handler.go` Authorize）：

```
https://<门户访问域名>/api/v1/client-auth/oauth/google/callback
```

- 从门户域名（如 `https://p.example.com`）发起登录 → 注册 `https://p.example.com/...` 的回调；
- 从面板域名 `/p` 发起 → 注册 `https://panel.example.com/...`。
- Google 允许配置**多个** redirect URI，两套域名都登记即可。
- ⚠ Google 强制要求 redirect URI 为 **https**（`http://localhost` 除外）；
  反代必须传对 `X-Forwarded-Host` / `X-Forwarded-Proto`，否则面板拼出的 redirect_uri
  会是 http 或内网地址 → Google 直接拒绝（`redirect_uri_mismatch`）。

面板里"系统设置 → OAuth → 门户回调域名"填上门户域名后，
页面显示的回调地址就是需要粘到 Google 后台的地址。

### 3.2 门户页上的隐私政策 / 服务条款链接

登录卡片底部自带"隐私政策 · 服务条款"链接（配置后显示 ✓，与常规站点一致 ✓）。

**配置入口（二选一，链接优先）**：

1. **内容方式（推荐，开箱即用）**：门户编辑器 → 设置 → 基础设置 → 合规文档 →
   把 §4 模板文本粘进"隐私政策内容"/"服务条款内容"（Markdown）→
   保存后登录卡片显示链接，点击**弹窗展示条款内容** ✓；
2. **外链方式**：填"隐私政策链接"/"服务条款链接"（指向你托管的公共页面）→
   点击新标签打开。

两种方式都会经 `/api/v1/public/portal-config` 下发（`privacy_policy_content` /
`terms_of_service_content` / `privacy_policy_url` / `terms_of_service_url`），
无需改模板源码。**要求：未登录也能打开**（内容方式天然满足 ✓）。

### 3.3 面板侧开关

`系统设置 → OAuth`：启用 Google、填 Client ID / Client Secret（Secret AES-256-GCM 加密落库）。
配置齐全后门户登录页才会显示 Google 按钮（`/api/v1/client-auth/oauth/providers` 返回 enabled）。

## 4. 隐私权政策 / 服务条款模板

> 以下为可直接改编的最小合规模板。使用前：替换 `【】` 占位、按实际业务增删条款、
> 由运营主体审阅后发布。本模板不构成法律意见。

### 4.1 隐私权政策（模板）

```markdown
# 隐私权政策

最后更新：【日期】

【运营主体名称】（"我们"）运营【门户名称】（"本服务"）。我们尊重您的隐私，
本政策说明我们收集哪些信息以及如何使用它们。

## 1. 我们收集的信息
- **账号信息**：您注册时提供的电子邮箱、密码（仅以单向哈希存储）；
- **Google 登录信息**：您选择"使用 Google 登录"时，我们经 Google 授权获取
  您的 Google 账号邮箱、昵称与头像 URL（scopes: openid, email, profile）；
- **服务必要数据**：订阅配置、流量用量、设备 IP 白名单、工单与订单记录。

## 2. 我们如何使用信息
仅用于：创建与登录您的账号、提供代理订阅服务、计费与客服、
安全风控（登录保护、IP 白名单）。我们**不会**出售您的个人信息。

## 3. 第三方服务
- **Google**：仅在您主动使用"Google 登录"时接收授权范围内的账号信息；
- 支付渠道：仅在您发起支付时接收订单信息。

## 4. 存储与安全
数据存储于我们的服务器，凭据经加密存储，传输全程 TLS。

## 5. 您的权利
您可以随时在门户内修改资料/密码、注销会话；如需删除账号及其全部数据，
请联系 【联系邮箱】，我们将在【N】个工作日内处理。

## 6. 政策变更
重大变更将在门户公告栏公示。
```

### 4.2 服务条款（模板）

```markdown
# 服务条款

最后更新：【日期】

## 1. 服务内容
本服务向注册用户提供【代理/网络接入】服务，具体套餐以商店页面为准。

## 2. 账号
- 您须对账号与密码安全负责；通过 Google 登录创建的账号，其凭据由 Google 托管；
- 禁止出借、转让账号，禁止将服务用于任何违法用途（包括但不限于：
  未经授权访问他人系统、分发违法内容、绕过司法管辖的强制规定）。

## 3. 付费与退款
套餐按【月/量】计费，流量与设备数以套餐说明为准；
【退款政策：如 7 天内未使用可全额退款 等】。

## 4. 服务变更与终止
我们可因不可抗力或您违反本条款而暂停/终止服务；账号删除时相关数据一并删除。

## 5. 免责声明
服务按"现状"提供；对因使用本服务造成的间接损失，我们在法律允许范围内不承担责任。

## 6. 其他
本条款受【司法辖区】法律管辖。联系：【联系邮箱】。
```

## 5. 上线前自查清单

- [ ] OAuth 同意屏幕：应用名称 / 徽标 / 主页 / 隐私政策 / 条款 五项齐全，域名已登记
- [ ] 隐私政策与服务条款页面**未登录可访问**（门户自定义页或独立静态页）
- [ ] 门户登录卡片/页脚可见两个链接
- [ ] redirect URI 与实际访问域名一致且为 https
- [ ] Publish App → 提交验证，通过后测试用户限制解除
- [ ] 面板 OAuth 设置：enabled=true 且 Client ID/Secret 已填（providers 接口返回 enabled:true）

## 6. GitHub 对照

GitHub OAuth App **没有**发布状态/验证流程：创建 App、填回调 URL 即可对外用。
仅需注意 GitHub OAuth App 的 Authorization callback URL 只能填**一个**（按前缀匹配），
多域名场景用 Google 那套多 URI 或为每个门户域名建独立 App。
