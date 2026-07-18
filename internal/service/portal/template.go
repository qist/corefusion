package portal

// defaultPortalHTML is the built-in portal page template.
// It uses {{.SiteName}}, {{.LogoURL}}, {{.RegistrationEnabled}} variables.
const defaultPortalHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no">
  <title>{{.PortalName}} - 订阅管理</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/element-plus/dist/index.css">
  <link rel="icon" id="portal-favicon" href="">
  <style id="portal-custom-css"></style>
</head>
<body>
  <div id="app" v-cloak>
    <!-- Auth Screen -->
    <div v-if="!token" class="auth-wrapper">
      <div class="auth-bg"></div>
      <div class="auth-card glass">
        <div class="auth-logo">
          <img v-if="logoUrl" :src="logoUrl" class="logo-img" />
          <div v-else class="logo-icon">⚡</div>
        </div>
        <h1 class="auth-title">{{.PortalName}}</h1>
        <p class="auth-subtitle">{{registrationEnabledText}}管理您的订阅</p>
        <el-tabs v-model="authTab" class="auth-tabs">
          <el-tab-pane label="登录" name="login">
            <el-form :model="loginForm" class="auth-form">
              <el-form-item>
                <el-input v-model="loginForm.email" placeholder="邮箱" prefix-icon="Message" size="large" />
              </el-form-item>
              <el-form-item>
                <el-input v-model="loginForm.password" type="password" placeholder="密码" prefix-icon="Lock" show-password size="large" @keyup.enter="handleLogin" />
              </el-form-item>
              <el-button type="primary" class="btn-block" :loading="loading" @click="handleLogin" size="large">登录</el-button>
            </el-form>
          </el-tab-pane>
          <el-tab-pane v-if="registrationEnabled" label="注册" name="register">
            <el-form :model="registerForm" class="auth-form">
              <el-form-item>
                <el-input v-model="registerForm.email" placeholder="邮箱" prefix-icon="Message" size="large" />
              </el-form-item>
              <el-form-item>
                <el-input v-model="registerForm.password" type="password" placeholder="密码（至少6位）" prefix-icon="Lock" show-password size="large" @keyup.enter="handleRegister" />
              </el-form-item>
              <el-form-item>
                <el-input v-model="registerForm.referral_code" placeholder="邀请码（可选）" prefix-icon="Promotion" size="large" />
              </el-form-item>
              <el-button type="primary" class="btn-block" :loading="loading" @click="handleRegister" size="large">注册</el-button>
            </el-form>
          </el-tab-pane>
        </el-tabs>
      </div>
    </div>

    <!-- Main App (after login) -->
    <div v-else class="app-wrapper">
      <!-- Top Bar -->
      <header class="top-bar glass">
        <div class="top-bar-left">
          <img v-if="logoUrl" :src="logoUrl" class="top-logo" />
          <span class="top-title">{{.PortalName}}</span>
        </div>
        <div class="top-bar-right">
          <button class="theme-toggle" @click="toggleTheme" :title="isDark ? '切换到亮色模式' : '切换到暗色模式'">
            <span v-if="isDark">☀️</span><span v-else>🌙</span>
          </button>
          <el-dropdown trigger="click">
            <span class="user-badge">
              <span class="user-email">{{ client.email }}</span>
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item @click="showPasswordDialog = true">修改密码</el-dropdown-item>
                <el-dropdown-item divided @click="handleLogout">退出登录</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </header>

      <!-- Content Area -->
      <main class="main-content">
        <!-- Dynamic Modules — all content is module-driven, no hardcoded tabs -->
        <template v-for="mod in currentModules" :key="mod.id">
          <div class="card glass module-wrap" :id="'mod-' + mod.id" :class="mod.style && mod.style.custom_class || ''">
            <!-- Hero -->
            <div v-if="mod.type === 'hero'" class="module-hero" :style="{ textAlign: mod.style && mod.style.align || 'left' }">
              <div v-if="mod.data && mod.data.eyebrow" class="hero-eyebrow">{{ mod.data.eyebrow }}</div>
              <h2 v-if="mod.data && mod.data.heading" class="hero-heading">{{ mod.data.heading }}</h2>
              <p v-if="mod.data && mod.data.description" class="hero-desc">{{ mod.data.description }}</p>
              <button v-if="mod.data && mod.data.button_text" class="hero-btn" @click="activeTab = mod.data.button_target || 'subscription'">{{ mod.data.button_text }}</button>
            </div>
            <!-- Rich Text -->
            <div v-else-if="mod.type === 'rich_text'" class="module-rich-text" :style="{ textAlign: mod.style && mod.style.align || 'left' }" v-html="mod.data.html"></div>
            <!-- Notice -->
            <div v-else-if="mod.type === 'notice'" class="module-notice" :class="'notice-' + (mod.style && mod.style.variant || 'info')" :style="{ cursor: mod.data && mod.data.display_mode === 'popup' && mod.data.popup_trigger === 'manual' ? 'pointer' : 'default' }" @click="mod.data && mod.data.display_mode === 'popup' && mod.data.popup_trigger === 'manual' && showNoticePopup(mod)">
              <div v-if="mod.data && mod.data.title" class="notice-title">{{ mod.data.title }}</div>
              <div v-if="mod.data && mod.data.markdown" class="md-render" v-html="renderMd(mod.data.markdown)"></div>
              <div v-else-if="mod.data && mod.data.html" v-html="mod.data.html"></div>
              <div v-if="mod.data && mod.data.display_mode === 'popup' && mod.data.popup_trigger === 'manual'" class="notice-popup-hint">📍 点击查看详情</div>
            </div>
            <!-- Feature Cards -->
            <div v-else-if="mod.type === 'feature_cards'">
              <div v-if="mod.title" class="card-header"><span class="card-title">{{ mod.title }}</span></div>
              <div class="feature-cards-grid">
                <div v-for="(card, ci) in ((mod.data && mod.data.cards) || []).slice(0,6)" :key="ci" class="feature-card">
                  <span class="feature-card-icon">{{ card.icon || '⭐' }}</span>
                  <h4>{{ card.title }}</h4>
                  <p>{{ card.description }}</p>
                </div>
              </div>
            </div>
            <!-- Account Stats (simple) -->
            <div v-else-if="mod.type === 'account_stats'">
              <div v-if="mod.title" class="card-header"><span class="card-icon">📊</span><span class="card-title">{{ mod.title }}</span></div>
              <div class="traffic-stats">
                <div class="stat-item"><span class="stat-icon">↑</span><span class="stat-value">{{ formatBytes(client.upload_bytes) }}</span><span class="stat-label">上传</span></div>
                <div class="stat-divider"></div>
                <div class="stat-item"><span class="stat-icon">↓</span><span class="stat-value">{{ formatBytes(client.download_bytes) }}</span><span class="stat-label">下载</span></div>
                <div class="stat-divider"></div>
                <div class="stat-item"><span class="stat-icon">💾</span><span class="stat-value">{{ formatBytes(client.quota_bytes) }}</span><span class="stat-label">总量</span></div>
              </div>
            </div>
            <!-- Traffic Ring (with SVG ring) -->
            <div v-else-if="mod.type === 'traffic_ring'">
              <div class="card-header">
                <span class="card-icon">📊</span>
                <span class="card-title">{{ mod.title || '流量使用' }}</span>
                <span class="card-action" v-if="refreshing">刷新中...</span>
              </div>
              <div class="traffic-ring-wrap">
                <svg class="traffic-ring" viewBox="0 0 120 120">
                  <circle cx="60" cy="60" r="52" fill="none" class="ring-bg"/>
                  <circle cx="60" cy="60" r="52" fill="none" class="ring-fill" :stroke-dasharray="ringDash" :style="{stroke: usageColor}"/>
                </svg>
                <div class="ring-text">
                  <span class="ring-percent">{{ usagePercent }}%</span>
                  <span class="ring-label">已使用</span>
                </div>
              </div>
              <div class="traffic-stats">
                <div class="stat-item"><span class="stat-icon">↑</span><span class="stat-value">{{ formatBytes(client.upload_bytes) }}</span><span class="stat-label">上传</span></div>
                <div class="stat-divider"></div>
                <div class="stat-item"><span class="stat-icon">↓</span><span class="stat-value">{{ formatBytes(client.download_bytes) }}</span><span class="stat-label">下载</span></div>
                <div class="stat-divider"></div>
                <div class="stat-item"><span class="stat-icon">💾</span><span class="stat-value">{{ formatBytes(client.quota_bytes) }}</span><span class="stat-label">总量</span></div>
              </div>
            </div>
            <!-- Account Info (dynamic) -->
            <div v-else-if="mod.type === 'account_info'">
              <div class="card-header"><span class="card-icon">👤</span><span class="card-title">{{ mod.title || '账户信息' }}</span></div>
              <div class="info-grid">
                <div class="info-item"><span class="info-label">邮箱</span><span class="info-value">{{ client.email }}</span></div>
                <div class="info-item"><span class="info-label">套餐</span><span class="info-value">{{ client.plan_name || '未购买' }}</span></div>
                <div class="info-item"><span class="info-label">状态</span><span class="info-value"><span :class="'badge badge-' + client.status">{{ statusText }}</span></span></div>
                <div class="info-item"><span class="info-label">到期时间</span><span class="info-value">{{ client.expires_at ? formatDate(client.expires_at) : '无限制' }}</span></div>
              </div>
            </div>
            <!-- Subscription Detail (dynamic) -->
            <div v-else-if="mod.type === 'subscription_detail'">
              <div class="card-header"><span class="card-icon">📋</span><span class="card-title">{{ mod.title || '订阅信息' }}</span></div>
              <p v-if="mod.data && mod.data.description" class="card-desc">{{ mod.data.description }}</p>
              <div v-if="client.sub_token && client.sub_status !== 'revoked'">
                <div class="sub-link-box">
                  <code class="sub-link-text">{{ subUrl }}</code>
                  <div class="sub-link-actions">
                    <el-button size="small" type="primary" text @click="copyText(subUrl)">{{ (mod.data && mod.data.copy_text) || '复制' }}</el-button>
                    <el-button size="small" text @click="toggleQr">{{ showQr ? '隐藏' : ((mod.data && mod.data.qr_text) || '二维码') }}</el-button>
                  </div>
                </div>
                <div v-if="showQr && qrCodeUrl" class="qr-wrap">
                  <img :src="qrCodeUrl" class="qr-img" />
                  <span class="qr-hint">扫码导入订阅</span>
                </div>
                <div v-if="mod.data && mod.data.show_status !== false" class="info-grid" style="margin-top: 16px">
                  <div class="info-item"><span class="info-label">订阅状态</span><span class="info-value"><span :class="'badge badge-' + (client.sub_status || 'none')">{{ subStatusText }}</span></span></div>
                  <div class="info-item"><span class="info-label">最后使用</span><span class="info-value">{{ client.last_node_usage_at ? formatDate(client.last_node_usage_at) : '未使用' }}</span></div>
                  <div class="info-item"><span class="info-label">订阅更新</span><span class="info-value">{{ client.sub_updated_at ? formatDate(client.sub_updated_at) : '未更新' }}</span></div>
                </div>
                <div v-if="mod.data && mod.data.show_rotate !== false" class="card-actions">
                  <el-button type="warning" size="small" @click="rotateToken" :loading="rotating" v-if="client.sub_status === 'active'">轮换订阅 Token</el-button>
                </div>
              </div>
              <div v-else-if="client.sub_status === 'revoked'" class="empty-state">
                <span class="empty-icon">🚫</span><p class="empty-text danger">订阅已吊销</p><p class="empty-hint">请联系管理员重新分配</p>
              </div>
              <div v-else-if="client.status === 'active' && !client.sub_token" class="empty-state">
                <span class="empty-icon">📭</span><p class="empty-text">暂无订阅</p><p class="empty-hint">请联系管理员分配套餐</p>
              </div>
              <div v-else-if="client.status === 'suspended'" class="empty-state">
                <span class="empty-icon">⏸️</span><p class="empty-text danger">账户已暂停</p><p class="empty-hint">请续费后恢复使用</p>
              </div>
            </div>
            <!-- Subscription (simple button) -->
            <div v-else-if="mod.type === 'subscription'" class="text-center">
              <div v-if="mod.data && mod.data.title" class="card-header"><span class="card-icon">📋</span><span class="card-title">{{ mod.data.title }}</span></div>
              <p v-if="mod.data && mod.data.description" class="card-desc">{{ mod.data.description }}</p>
              <button v-if="mod.data && mod.data.button_text" class="hero-btn" @click="activeTab = mod.data.button_target || 'subscription'">{{ mod.data.button_text }}</button>
            </div>
            <!-- Security Settings (dynamic) -->
            <div v-else-if="mod.type === 'security_settings'">
              <div class="card-header"><span class="card-icon">🔒</span><span class="card-title">{{ mod.title || '安全设置' }}</span></div>
              <p v-if="mod.data && mod.data.description" class="card-desc">{{ mod.data.description }}</p>
              <el-form v-if="mod.data && mod.data.show_ip_whitelist !== false" label-position="top" size="small">
                <el-form-item label="IP 白名单（逗号分隔，留空不限制）">
                  <el-input v-model="settings.ip_whitelist" placeholder="例: 192.168.1.0/24,10.0.0.1" />
                </el-form-item>
                <el-form-item label="设备限制（0=不限制）">
                  <el-input-number v-model="settings.device_limit" :min="0" :max="100" />
                </el-form-item>
                <el-button type="primary" size="small" @click="saveSettings">保存设置</el-button>
              </el-form>
              <div v-if="mod.data && mod.data.show_password !== false">
                <div class="card-header" style="margin-top: 20px"><span class="card-icon">🔑</span><span class="card-title">修改密码</span></div>
                <el-form label-position="top" size="small">
                  <el-form-item label="旧密码"><el-input v-model="pwdForm.old_password" type="password" show-password /></el-form-item>
                  <el-form-item label="新密码"><el-input v-model="pwdForm.new_password" type="password" show-password /></el-form-item>
                  <el-form-item label="确认密码"><el-input v-model="pwdForm.confirm_password" type="password" show-password /></el-form-item>
                  <el-button type="primary" size="small" @click="changePassword">修改密码</el-button>
                </el-form>
              </div>
            </div>
            <!-- Log Packages (dynamic) -->
            <div v-else-if="mod.type === 'log_packages'">
              <div class="card-header"><span class="card-icon">📦</span><span class="card-title">{{ mod.title || '日志打包' }}</span></div>
              <p v-if="mod.data && mod.data.description" class="card-desc">{{ mod.data.description }}</p>
              <p v-else class="card-desc">请求打包最近日志，系统将在后台收集并压缩为可下载文件。</p>
              <div class="log-actions">
                <el-button type="primary" size="small" @click="createLogPackage(1)" :loading="logPacking">{{ (mod.data && mod.data.btn_1h) || '打包最近1小时' }}</el-button>
                <el-button size="small" @click="createLogPackage(6)" :loading="logPacking">{{ (mod.data && mod.data.btn_6h) || '最近6小时' }}</el-button>
                <el-button size="small" @click="createLogPackage(24)">{{ (mod.data && mod.data.btn_24h) || '最近24小时' }}</el-button>
              </div>
              <div v-if="logPackages.length > 0" style="margin-top: 16px">
                <div class="card-header"><span class="card-icon">📁</span><span class="card-title">日志包列表</span><span class="card-action" @click="fetchLogPackages">刷新</span></div>
                <div class="logpkg-list">
                  <div class="logpkg-item" v-for="pkg in logPackages" :key="pkg.id">
                    <div class="logpkg-info">
                      <span class="logpkg-id">#{{ pkg.id }}</span>
                      <span :class="'badge badge-' + pkg.status">{{ pkgStatusText(pkg.status) }}</span>
                      <span class="logpkg-time">{{ formatDate(pkg.created_at) }}</span>
                    </div>
                    <div class="logpkg-actions">
                      <span v-if="pkg.file_size" class="logpkg-size">{{ formatBytes(pkg.file_size) }}</span>
                      <el-button v-if="pkg.status === 'done'" size="small" type="primary" text @click="downloadLogPackage(pkg.id)">下载</el-button>
                      <span v-if="pkg.status === 'processing'" class="logpkg-hint">处理中...</span>
                      <span v-if="pkg.error" class="logpkg-error">{{ pkg.error }}</span>
                    </div>
                  </div>
                </div>
              </div>
            </div>
            <!-- Markdown -->
            <div v-else-if="mod.type === 'markdown'" class="module-markdown" :style="{ textAlign: mod.style && mod.style.align || 'left' }">
              <div v-if="mod.title" class="card-header"><span class="card-title">{{ mod.title }}</span></div>
              <div class="md-render" v-html="renderMd(mod.data && mod.data.markdown || '')"></div>
            </div>
            <!-- Custom HTML -->
            <div v-else-if="mod.type === 'custom_html'" v-html="(mod.data && mod.data.html) || ''"></div>
            <!-- Plan List (商业模块 P0) -->
            <div v-else-if="mod.type === 'plan_list'">
              <div class="card-header"><span class="card-icon">💎</span><span class="card-title">{{ mod.title || '套餐列表' }}</span></div>
              <p v-if="mod.data && mod.data.description" class="card-desc">{{ mod.data.description }}</p>
              <div v-if="plansLoading" class="empty-state"><span class="empty-icon">⏳</span><p class="empty-text">加载中...</p></div>
              <div v-else-if="plans.length === 0" class="empty-state"><span class="empty-icon">📭</span><p class="empty-text">暂无可用套餐</p></div>
              <div v-else class="plan-grid" :style="{ gridTemplateColumns: 'repeat(' + (mod.data && mod.data.columns || 2) + ', 1fr)' }">
                <div v-for="plan in plans" :key="plan.id" class="plan-card" :class="{ 'plan-popular': plan.variants && plan.variants.some(function(v){ return v.is_recommended }) }">
                  <h4 class="plan-name">{{ plan.name }}</h4>
                  <p v-if="plan.description" class="plan-desc">{{ plan.description }}</p>
                  <div v-for="v in plan.variants" :key="v.id" class="plan-variant">
                    <div v-if="mod.data && mod.data.show_price !== false" class="plan-price">
                      <span class="price-amount">{{ formatMoney(v.price) }}</span>
                      <span class="price-currency">{{ v.currency || 'CNY' }}</span>
                      <span v-if="v.original_price && v.original_price > v.price" class="price-original">{{ formatMoney(v.original_price) }}</span>
                    </div>
                    <div class="plan-meta">
                      <span v-if="v.billing_cycle" class="plan-cycle">{{ {monthly:'月付',quarterly:'季付',yearly:'年付'}[v.billing_cycle] || v.billing_cycle }}</span>
                      <span v-if="v.duration_days" class="plan-duration">{{ v.duration_days }}天</span>
                    </div>
                    <div v-if="mod.data && mod.data.show_quota !== false && v.quota_bytes" class="plan-quota">{{ formatBytes(v.quota_bytes) }} 流量</div>
                    <button class="hero-btn plan-btn" @click="purchaseVariant(v.id, plan, mod.data)" :disabled="orderBusy">
                      <span v-if="client.plan_id > 0 && client.plan_id === plan.id">{{ (mod.data && mod.data.renew_text) || '续费' }}</span>
                      <span v-else>{{ (mod.data && mod.data.buy_text) || '立即购买' }}</span>
                    </button>
                  </div>
                </div>
              </div>
            </div>
            <!-- Order Create (商业模块 P0) -->
            <div v-else-if="mod.type === 'order_create'">
              <div class="card-header"><span class="card-icon">🛒</span><span class="card-title">{{ mod.title || '创建订单' }}</span></div>
              <p v-if="mod.data && mod.data.description" class="card-desc">{{ mod.data.description }}</p>
              <el-form label-position="top" size="small">
                <el-form-item label="套餐">
                  <el-select v-model="orderForm.variant_id" placeholder="选择套餐" style="width: 100%">
                    <el-option v-for="p in plans" :key="p.id" :label="p.name" :value="p.id" />
                  </el-select>
                </el-form-item>
                <el-form-item label="订单类型">
                  <el-radio-group v-model="orderForm.order_type">
                    <el-radio value="new">新购</el-radio>
                    <el-radio value="renew">续费</el-radio>
                    <el-radio value="upgrade">升级</el-radio>
                  </el-radio-group>
                </el-form-item>
                <el-form-item label="优惠码">
                  <el-input v-model="orderForm.promotion_code" placeholder="输入优惠码（可选）" />
                </el-form-item>
                <el-form-item label="支付方式">
                  <el-radio-group v-model="orderForm.pay_method">
                    <el-radio value="balance">余额支付</el-radio>
                    <el-radio value="charge_site">在线支付</el-radio>
                  </el-radio-group>
                </el-form-item>
                <el-form-item v-if="orderForm.pay_method === 'charge_site'" label="充值站点">
                  <el-select v-model="orderForm.charge_site_id" placeholder="选择支付渠道" style="width: 100%">
                    <el-option v-for="s in chargeSites" :key="s.id" :label="s.name" :value="s.id" />
                  </el-select>
                </el-form-item>
                <el-button type="primary" size="small" @click="submitOrder" :loading="orderBusy">创建订单</el-button>
              </el-form>
            </div>
            <!-- Charge Panel (商业模块 P0) -->
            <div v-else-if="mod.type === 'charge_panel'">
              <div class="card-header"><span class="card-icon">💳</span><span class="card-title">{{ mod.title || '在线充值' }}</span></div>
              <p v-if="mod.data && mod.data.description" class="card-desc">{{ mod.data.description }}</p>
              <div class="charge-section">
                <h4 class="charge-subtitle">在线支付</h4>
                <el-form label-position="top" size="small">
                  <el-form-item label="充值站点">
                    <el-select v-model="chargeForm.site_id" placeholder="选择支付渠道" style="width: 100%">
                      <el-option v-for="s in chargeSites" :key="s.id" :label="s.name" :value="s.id" />
                    </el-select>
                  </el-form-item>
                  <el-form-item label="金额（元）">
                    <el-input-number v-model="chargeForm.amount_yuan" :min="1" :step="10" style="width: 100%" />
                  </el-form-item>
                  <el-button type="primary" size="small" @click="submitCharge" :loading="chargeBusy">立即充值</el-button>
                </el-form>
              </div>
              <div class="charge-section">
                <h4 class="charge-subtitle">充值卡兑换</h4>
                <div class="charge-card-row">
                  <el-input v-model="cardCode" :placeholder="(mod.data && mod.data.card_placeholder) || '输入充值卡号'" size="small" />
                  <el-button type="success" size="small" @click="redeemCard" :loading="redeemBusy">{{ (mod.data && mod.data.card_btn_text) || '兑换' }}</el-button>
                </div>
              </div>
              <div v-if="mod.data && mod.data.show_orders !== false && chargeOrders.length > 0" class="charge-section">
                <div class="card-header"><span class="card-icon">📋</span><span class="card-title">充值记录</span></div>
                <div class="logpkg-list">
                  <div class="logpkg-item" v-for="co in chargeOrders.slice(0, (mod.data && mod.data.orders_limit) || 5)" :key="co.id">
                    <div class="logpkg-info">
                      <span class="logpkg-id">{{ co.trade_no }}</span>
                      <span :class="'badge badge-' + co.status">{{ chargeStatusText(co.status) }}</span>
                      <span class="logpkg-time">{{ formatMoney(co.amount) }} {{ co.currency }}</span>
                    </div>
                    <div class="logpkg-actions">
                      <span class="logpkg-time">{{ formatDate(co.created_at) }}</span>
                      <a v-if="co.payment_url && co.status === 'pending'" :href="co.payment_url" target="_blank" class="card-action">去支付</a>
                    </div>
                  </div>
                </div>
              </div>
            </div>
            <!-- Balance Display (商业模块 P0) -->
            <div v-else-if="mod.type === 'balance_display'">
              <div class="card-header"><span class="card-icon">💰</span><span class="card-title">{{ mod.title || '账户余额' }}</span></div>
              <div class="balance-hero">
                <span class="balance-amount">{{ formatMoney(balance.balance) }}</span>
                <span v-if="balance.frozen > 0" class="balance-frozen">冻结: {{ formatMoney(balance.frozen) }}</span>
              </div>
              <div class="card-actions">
                <el-button type="primary" size="small" @click="activeTab = 'store'; scrollToModule('mod-charge')">{{ (mod.data && mod.data.charge_btn_text) || '去充值' }}</el-button>
              </div>
              <div v-if="mod.data && mod.data.show_transactions !== false && balanceTxns.length > 0" style="margin-top: 16px">
                <div class="card-header"><span class="card-icon">📄</span><span class="card-title">交易记录</span></div>
                <div class="logpkg-list">
                  <div class="logpkg-item" v-for="tx in balanceTxns.slice(0, (mod.data && mod.data.tx_limit) || 10)" :key="tx.id">
                    <div class="logpkg-info">
                      <span :class="'badge badge-' + (tx.amount >= 0 ? 'active' : 'expired')">{{ tx.amount >= 0 ? '+' : '' }}{{ formatMoney(tx.amount) }}</span>
                      <span class="logpkg-time">{{ tx.remark || tx.type }}</span>
                    </div>
                    <div class="logpkg-actions">
                      <span class="logpkg-time">{{ formatDate(tx.created_at) }}</span>
                    </div>
                  </div>
                </div>
              </div>
            </div>
            <!-- Referral (商业模块 P1) -->
            <div v-else-if="mod.type === 'referral'">
              <div class="card-header"><span class="card-icon">🎁</span><span class="card-title">{{ mod.title || '邀请返利' }}</span></div>
              <p v-if="mod.data && mod.data.description" class="card-desc">{{ mod.data.description }}</p>
              <div class="referral-link-box" v-if="referralCode">
                <code class="sub-link-text">{{ referralInviteUrl }}</code>
                <el-button size="small" type="primary" text @click="copyText(referralInviteUrl)">{{ (mod.data && mod.data.copy_btn_text) || '复制邀请链接' }}</el-button>
              </div>
              <div class="referral-stats" v-if="referralStats">
                <div class="stat-item"><span class="stat-value">{{ referralStats.total_invited }}</span><span class="stat-label">已邀请</span></div>
                <div class="stat-divider"></div>
                <div class="stat-item"><span class="stat-value">{{ referralStats.total_paid }}</span><span class="stat-label">已付费</span></div>
                <div class="stat-divider"></div>
                <div class="stat-item"><span class="stat-value">{{ formatMoney(referralStats.available_commission) }}</span><span class="stat-label">可提现</span></div>
                <div class="stat-divider"></div>
                <div class="stat-item"><span class="stat-value">{{ formatMoney(referralStats.total_commission) }}</span><span class="stat-label">总佣金</span></div>
              </div>
              <div v-if="mod.data && mod.data.show_records !== false && referralRecords.length > 0" style="margin-top: 16px">
                <div class="card-header"><span class="card-icon">📋</span><span class="card-title">邀请记录</span></div>
                <div class="logpkg-list">
                  <div class="logpkg-item" v-for="r in referralRecords.slice(0, (mod.data && mod.data.records_limit) || 10)" :key="r.id">
                    <div class="logpkg-info">
                      <span class="logpkg-time">{{ r.referee_email }}</span>
                      <span :class="'badge badge-' + (r.commission_status === 'available' ? 'active' : r.commission_status === 'pending' ? 'processing' : 'none')">{{ referralStatusText(r.commission_status) }}</span>
                    </div>
                    <div class="logpkg-actions">
                      <span v-if="r.commission_amount" class="logpkg-time">{{ formatMoney(r.commission_amount) }}</span>
                      <span class="logpkg-time">{{ formatDate(r.registered_at) }}</span>
                    </div>
                  </div>
                </div>
              </div>
            </div>
            <!-- Ticket List (商业模块 P1) -->
            <div v-else-if="mod.type === 'ticket_list'">
              <div class="card-header"><span class="card-icon">🎫</span><span class="card-title">{{ mod.title || '我的工单' }}</span><span class="card-action" @click="showTicketDialog = true">{{ (mod.data && mod.data.new_btn_text) || '新建工单' }}</span></div>
              <div v-if="tickets.length === 0" class="empty-state"><span class="empty-icon">📭</span><p class="empty-text">暂无工单</p></div>
              <div v-else class="logpkg-list">
                <div class="logpkg-item" v-for="t in tickets" :key="t.id" @click="openTicket(t.id)" style="cursor: pointer">
                  <div class="logpkg-info">
                    <span class="logpkg-id">#{{ t.id }}</span>
                    <span class="logpkg-time">{{ t.subject }}</span>
                    <span :class="'badge badge-' + ticketStatusBadge(t.status)">{{ ticketStatusText(t.status) }}</span>
                  </div>
                  <div class="logpkg-actions">
                    <span class="logpkg-time">{{ formatDate(t.updated_at) }}</span>
                  </div>
                </div>
              </div>
            </div>
            <!-- Article List (商业模块 P2) -->
            <div v-else-if="mod.type === 'article_list'">
              <div class="card-header"><span class="card-icon">📚</span><span class="card-title">{{ mod.title || '教程文章' }}</span></div>
              <div v-if="mod.data && mod.data.show_category_filter" class="article-filter">
                <el-radio-group v-model="articleCategory" size="small" @change="fetchArticles">
                  <el-radio-button value="">全部</el-radio-button>
                  <el-radio-button value="tutorial">教程</el-radio-button>
                  <el-radio-button value="faq">FAQ</el-radio-button>
                  <el-radio-button value="notice">公告</el-radio-button>
                </el-radio-group>
              </div>
              <div v-if="articles.length === 0" class="empty-state"><span class="empty-icon">📭</span><p class="empty-text">暂无文章</p></div>
              <div v-else class="article-list">
                <div class="article-item" v-for="a in articles" :key="a.id" @click="openArticle(a.slug)">
                  <span class="article-cat-badge">{{ articleCatText(a.category) }}</span>
                  <div class="article-info">
                    <span class="article-title">{{ a.title }}</span>
                    <span class="logpkg-time">{{ formatDate(a.published_at || a.updated_at) }}</span>
                  </div>
                  <span class="card-action">阅读</span>
                </div>
              </div>
            </div>
            <!-- Shared Accounts (商业模块 P2) -->
            <div v-else-if="mod.type === 'shared_accounts'">
              <div class="card-header"><span class="card-icon">🔑</span><span class="card-title">{{ mod.title || '共享账号' }}</span></div>
              <p v-if="mod.data && mod.data.description" class="card-desc">{{ mod.data.description }}</p>
              <div v-if="sharedAccounts.length === 0" class="empty-state"><span class="empty-icon">📭</span><p class="empty-text">暂无可用共享账号</p></div>
              <div v-else class="article-list">
                <div class="article-item" v-for="sa in sharedAccounts" :key="sa.id">
                  <img v-if="sa.icon_url" :src="sa.icon_url" class="sa-icon" />
                  <span v-else class="article-cat-badge">{{ sharedAcctTypeText(sa.type) }}</span>
                  <div class="article-info">
                    <span class="article-title">{{ sa.name }}</span>
                    <span class="logpkg-time">{{ sharedAcctTypeText(sa.type) }}</span>
                  </div>
                  <el-button size="small" type="primary" text @click="applySharedAccount(sa.id, mod.data)">{{ (mod.data && mod.data.apply_btn_text) || '查看凭据' }}</el-button>
                  <el-button size="small" text @click="repairSharedAccount(sa.id, mod.data)">{{ (mod.data && mod.data.repair_btn_text) || '请求修复' }}</el-button>
                </div>
              </div>
            </div>
            <!-- Freeform (HTML+CSS+JS custom component) -->
            <div v-else-if="mod.type === 'freeform'" class="module-freeform">
              <component :is="freeformComponents[mod.id]" v-if="freeformComponents[mod.id]"></component>
              <div v-else style="color:#f56c6c;padding:12px;font-size:13px">⚠️ 模块加载失败</div>
            </div>
          </div>
        </template>
        <!-- Empty state when no modules on current page -->
        <div v-if="currentModules.length === 0" class="card glass">
          <div class="empty-state">
            <span class="empty-icon">📋</span>
            <p class="empty-text">此页面暂无内容</p>
          </div>
        </div>
      </main>

      <!-- Bottom Navigation -->
      <nav class="bottom-nav glass">
        <div v-for="nav in portalNavItems" :key="nav.id" :class="'nav-item' + (activeTab === nav.target ? ' active' : '')" @click="onNavClick(nav)">
          <span class="nav-icon">{{ nav.icon || '🔗' }}</span>
          <span class="nav-label">{{ nav.label }}</span>
        </div>
      </nav>
    </div>

    <!-- Notice Popup Dialog -->
    <el-dialog
      v-if="activeNoticePopup"
      v-model="noticePopupVisible"
      :title="(activeNoticePopup.data && activeNoticePopup.data.title) || activeNoticePopup.title || '公告'"
      :width="(activeNoticePopup.data && activeNoticePopup.data.popup_width) || '480px'"
      :show-close="(activeNoticePopup.data && activeNoticePopup.data.popup_show_close !== false)"
      :close-on-click-modal="false"
      class="notice-popup-dialog"
      align-center
      append-to-body
    >
      <div class="notice-popup-body" :class="'notice-' + ((activeNoticePopup.style && activeNoticePopup.style.variant) || 'info')">
        <div v-if="activeNoticePopup.data && activeNoticePopup.data.markdown" class="md-render" v-html="renderMd(activeNoticePopup.data.markdown)"></div>
        <div v-if="activeNoticePopup.data && activeNoticePopup.data.html" v-html="activeNoticePopup.data.html"></div>
      </div>
      <template #footer>
        <div class="notice-popup-footer">
          <el-button
            v-if="activeNoticePopup.data && activeNoticePopup.data.popup_link_url"
            type="primary"
            plain
            @click="openNoticeLink(activeNoticePopup.data.popup_link_url, activeNoticePopup.data.popup_link_external !== false)"
          >
            {{ (activeNoticePopup.data && activeNoticePopup.data.popup_link_text) || '查看详情' }}
          </el-button>
          <el-button type="primary" @click="dismissNoticePopup">
            {{ (activeNoticePopup.data && activeNoticePopup.data.popup_button_text) || '我知道了' }}
          </el-button>
        </div>
      </template>
    </el-dialog>

    <!-- Ticket Detail Dialog -->
    <el-dialog v-model="showTicketDetail" title="工单详情" width="500px" align-center append-to-body>
      <div v-if="ticketDetail" class="ticket-detail">
        <div class="ticket-detail-header">
          <span class="logpkg-id">#{{ ticketDetail.ticket.id }}</span>
          <span :class="'badge badge-' + ticketStatusBadge(ticketDetail.ticket.status)">{{ ticketStatusText(ticketDetail.ticket.status) }}</span>
          <span class="logpkg-time">{{ formatDate(ticketDetail.ticket.created_at) }}</span>
        </div>
        <h3 class="ticket-subject">{{ ticketDetail.ticket.subject }}</h3>
        <div class="ticket-msgs">
          <div v-for="msg in ticketDetail.messages" :key="msg.id" class="ticket-msg" :class="'msg-' + msg.sender_type">
            <span class="ticket-msg-sender">{{ msg.sender_type === 'client' ? '我' : '客服' }}</span>
            <div class="ticket-msg-content" v-html="renderMd(msg.content)"></div>
            <span class="logpkg-time">{{ formatDate(msg.created_at) }}</span>
          </div>
        </div>
        <div v-if="ticketDetail.ticket.status !== 'closed'" class="ticket-reply">
          <el-input v-model="ticketReplyContent" type="textarea" :rows="2" placeholder="输入回复内容..." />
          <div class="card-actions" style="margin-top: 8px">
            <el-button type="primary" size="small" @click="replyTicket" :loading="ticketBusy">回复</el-button>
            <el-button size="small" @click="closeTicket" :loading="ticketBusy">关闭工单</el-button>
          </div>
        </div>
      </div>
    </el-dialog>

    <!-- New Ticket Dialog -->
    <el-dialog v-model="showTicketDialog" title="新建工单" width="500px" align-center append-to-body>
      <el-form label-position="top" size="small">
        <el-form-item label="主题"><el-input v-model="newTicketForm.subject" placeholder="简要描述问题" /></el-form-item>
        <el-form-item label="分类">
          <el-select v-model="newTicketForm.category" style="width: 100%">
            <el-option label="技术问题" value="technical" />
            <el-option label="账单问题" value="billing" />
            <el-option label="账号问题" value="account" />
            <el-option label="其他" value="other" />
          </el-select>
        </el-form-item>
        <el-form-item label="优先级">
          <el-radio-group v-model="newTicketForm.priority">
            <el-radio value="low">低</el-radio>
            <el-radio value="normal">普通</el-radio>
            <el-radio value="high">高</el-radio>
            <el-radio value="urgent">紧急</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="内容"><el-input v-model="newTicketForm.content" type="textarea" :rows="4" placeholder="详细描述您的问题..." /></el-form-item>
        <el-button type="primary" size="small" @click="createTicket" :loading="ticketBusy">提交工单</el-button>
      </el-form>
    </el-dialog>

    <!-- Article Detail Dialog -->
    <el-dialog v-model="showArticleDetail" :title="articleDetail ? articleDetail.title : '文章'" width="600px" align-center append-to-body class="article-dialog">
      <div v-if="articleDetail" class="md-render" v-html="renderMd(articleDetail.content)"></div>
    </el-dialog>

    <!-- Shared Account Credentials Dialog -->
    <el-dialog v-model="showCredentialsDialog" title="账号凭据" width="460px" align-center append-to-body>
      <div v-if="sharedAccountCreds" class="credential-detail">
        <div class="info-grid" style="margin-bottom: 12px">
          <div class="info-item"><span class="info-label">名称</span><span class="info-value">{{ sharedAccountCreds.name }}</span></div>
          <div class="info-item"><span class="info-label">类型</span><span class="info-value">{{ sharedAcctTypeText(sharedAccountCreds.type) }}</span></div>
        </div>
        <el-alert type="warning" :closable="false" show-icon style="margin-bottom: 12px">
          <span style="font-size: 12px">凭据信息请妥善保管，不要分享给他人。</span>
        </el-alert>
        <div class="referral-link-box">
          <code class="sub-link-text" style="white-space: pre-wrap; word-break: break-all">{{ sharedAccountCreds.credentials_enc }}</code>
          <el-button size="small" type="primary" text @click="copyText(sharedAccountCreds.credentials_enc)">复制</el-button>
        </div>
      </div>
    </el-dialog>
  </div>

  <script src="https://cdn.jsdelivr.net/npm/vue@3/dist/vue.global.prod.js"></script>
  <script src="https://cdn.jsdelivr.net/npm/element-plus/dist/index.full.min.js"></script>
  <script src="https://cdn.jsdelivr.net/npm/qrious@4/dist/qrious.min.js"></script>
  <script src="https://cdn.jsdelivr.net/npm/marked/marked.min.js"></script>
  <script id="portal-config"></script>
  <script id="portal-custom-js"></script>
</body>
</html>`

const defaultPortalCSS = `[v-cloak] { display: none; }

:root {
  --bg-gradient: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  --bg-solid: #f0f2f5;
  --glass-bg: rgba(255,255,255,0.78);
  --glass-border: rgba(255,255,255,0.3);
  --glass-shadow: 0 8px 32px rgba(31,38,135,0.12);
  --text-primary: #1a1a2e;
  --text-secondary: #6c7293;
  --text-muted: #9ca3af;
  --accent: {{.PrimaryColor}};
  --accent-light: rgba(102,126,234,0.12);
  --success: #67c23a;
  --danger: #f56c6c;
  --warning: #e6a23c;
  --info: #909399;
  --border: rgba(0,0,0,0.06);
  --ring-bg: #e8e8ef;
  --nav-bg: rgba(255,255,255,0.92);
  --font-family: {{.FontFamily}};
}

[data-theme="dark"] {
  --bg-gradient: linear-gradient(135deg, #0f0c29 0%, #302b63 50%, #24243e 100%);
  --bg-solid: #121220;
  --glass-bg: rgba(30,30,50,0.82);
  --glass-border: rgba(255,255,255,0.08);
  --glass-shadow: 0 8px 32px rgba(0,0,0,0.4);
  --text-primary: #e0e0f0;
  --text-secondary: #9ca3c0;
  --text-muted: #6c7293;
  --accent: {{.PrimaryColor}};
  --accent-light: rgba(102,126,234,0.2);
  --border: rgba(255,255,255,0.06);
  --ring-bg: #2a2a40;
  --nav-bg: rgba(20,20,35,0.95);
}

* { margin: 0; padding: 0; box-sizing: border-box; }
body {
  font-family: {{.FontFamily}};
  background: var(--bg-solid);
  color: var(--text-primary);
  min-height: 100vh;
  -webkit-font-smoothing: antialiased;
}

/* ---- Glass ---- */
.glass {
  background: var(--glass-bg);
  backdrop-filter: blur(16px);
  -webkit-backdrop-filter: blur(16px);
  border: 1px solid var(--glass-border);
  box-shadow: var(--glass-shadow);
}

/* ---- Auth ---- */
.auth-wrapper {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;
  padding: 20px;
}
.auth-bg {
  position: fixed; inset: 0;
  background: var(--bg-gradient);
  z-index: 0;
}
.auth-card {
  position: relative; z-index: 1;
  border-radius: 20px;
  padding: 40px 32px;
  width: 100%;
  max-width: 400px;
  text-align: center;
}
.auth-logo { margin-bottom: 8px; }
.logo-img { width: 64px; height: 64px; border-radius: 16px; }
.logo-icon { font-size: 48px; line-height: 1; }
.auth-title {
  font-size: 24px;
  font-weight: 700;
  color: var(--text-primary);
  margin-bottom: 4px;
}
.auth-subtitle {
  font-size: 14px;
  color: var(--text-secondary);
  margin-bottom: 24px;
}
.auth-tabs { margin-bottom: 0; }
.auth-form { margin-top: 16px; }
.btn-block { width: 100%; }

/* ---- App Layout ---- */
.app-wrapper {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  background: var(--bg-solid);
}

/* ---- Top Bar ---- */
.top-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 20px;
  position: sticky;
  top: 0;
  z-index: 100;
  border-bottom: 1px solid var(--border);
}
.top-bar-left { display: flex; align-items: center; gap: 10px; }
.top-logo { width: 28px; height: 28px; border-radius: 6px; }
.top-title { font-size: 16px; font-weight: 600; color: var(--text-primary); }
.top-bar-right { display: flex; align-items: center; gap: 12px; }
.theme-toggle {
  background: none; border: none; cursor: pointer;
  font-size: 18px; padding: 4px; border-radius: 8px;
  transition: background 0.2s;
}
.theme-toggle:hover { background: var(--accent-light); }
.user-badge { cursor: pointer; }
.user-email { font-size: 13px; color: var(--text-secondary); }

/* ---- Main Content ---- */
.main-content {
  flex: 1;
  padding: 16px 16px 80px;
  max-width: 600px;
  width: 100%;
  margin: 0 auto;
}

/* ---- Cards ---- */
.card {
  border-radius: 16px;
  padding: 20px;
  margin-bottom: 16px;
}
.card-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 16px;
}
.card-icon { font-size: 18px; }
.card-title { font-size: 15px; font-weight: 600; color: var(--text-primary); }
.card-action { font-size: 12px; color: var(--accent); cursor: pointer; margin-left: auto; }
.card-desc { font-size: 13px; color: var(--text-secondary); margin-bottom: 12px; }
.card-actions { margin-top: 16px; display: flex; gap: 8px; flex-wrap: wrap; }

/* ---- Info Grid ---- */
.info-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}
.info-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.info-label { font-size: 12px; color: var(--text-muted); }
.info-value { font-size: 14px; font-weight: 500; color: var(--text-primary); word-break: break-all; }

/* ---- Badge ---- */
.badge {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 6px;
  font-size: 12px;
  font-weight: 500;
}
.badge-active { background: rgba(103,194,58,0.12); color: var(--success); }
.badge-suspended { background: rgba(245,108,108,0.12); color: var(--danger); }
.badge-expired { background: rgba(230,162,60,0.12); color: var(--warning); }
.badge-revoked { background: rgba(245,108,108,0.12); color: var(--danger); }
.badge-done { background: rgba(103,194,58,0.12); color: var(--success); }
.badge-processing { background: rgba(102,126,234,0.12); color: var(--accent); }
.badge-failed { background: rgba(245,108,108,0.12); color: var(--danger); }
.badge-none { background: rgba(144,147,153,0.12); color: var(--info); }

/* ---- Traffic Ring ---- */
.traffic-ring-wrap {
  position: relative;
  width: 140px; height: 140px;
  margin: 0 auto 16px;
}
.traffic-ring { width: 100%; height: 100%; transform: rotate(-90deg); }
.ring-bg { stroke: var(--ring-bg); stroke-width: 10; }
.ring-fill { stroke-width: 10; stroke-linecap: round; transition: stroke-dasharray 0.6s ease; }
.ring-text {
  position: absolute; inset: 0;
  display: flex; flex-direction: column;
  align-items: center; justify-content: center;
}
.ring-percent { font-size: 28px; font-weight: 700; color: var(--text-primary); line-height: 1; }
.ring-label { font-size: 12px; color: var(--text-muted); margin-top: 2px; }

.traffic-stats {
  display: flex;
  align-items: center;
  justify-content: space-around;
  padding: 12px 0;
}
.stat-item {
  display: flex; flex-direction: column; align-items: center; gap: 2px;
}
.stat-icon { font-size: 14px; }
.stat-value { font-size: 15px; font-weight: 600; color: var(--text-primary); }
.stat-label { font-size: 11px; color: var(--text-muted); }
.stat-divider { width: 1px; height: 32px; background: var(--border); }

/* ---- Subscription ---- */
.sub-link-box {
  background: var(--accent-light);
  border-radius: 10px;
  padding: 12px;
  display: flex;
  align-items: center;
  gap: 8px;
}
.sub-link-text {
  flex: 1;
  font-size: 12px;
  word-break: break-all;
  color: var(--text-primary);
  font-family: 'SF Mono', 'Fira Code', monospace;
}
.sub-link-actions { display: flex; gap: 4px; flex-shrink: 0; }

.qr-wrap {
  text-align: center;
  margin-top: 16px;
}
.qr-img { max-width: 180px; border-radius: 10px; border: 1px solid var(--border); }
.qr-hint { display: block; font-size: 12px; color: var(--text-muted); margin-top: 8px; }

/* ---- Empty State ---- */
.empty-state { text-align: center; padding: 24px 0; }
.empty-icon { font-size: 40px; display: block; margin-bottom: 8px; }
.empty-text { font-size: 15px; font-weight: 500; color: var(--text-primary); }
.empty-text.danger { color: var(--danger); }
.empty-hint { font-size: 13px; color: var(--text-muted); margin-top: 4px; }

/* ---- Logs ---- */
.log-actions { display: flex; gap: 8px; flex-wrap: wrap; }
.logpkg-list { display: flex; flex-direction: column; gap: 10px; }
.logpkg-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 12px;
  background: var(--accent-light);
  border-radius: 10px;
}
.logpkg-info { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.logpkg-id { font-size: 13px; font-weight: 600; color: var(--text-primary); }
.logpkg-time { font-size: 12px; color: var(--text-muted); }
.logpkg-actions { display: flex; align-items: center; gap: 8px; }
.logpkg-size { font-size: 12px; color: var(--text-secondary); }
.logpkg-hint { font-size: 12px; color: var(--accent); }
.logpkg-error { font-size: 12px; color: var(--danger); max-width: 120px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

/* ---- Bottom Nav ---- */
.bottom-nav {
  position: fixed;
  bottom: 0; left: 0; right: 0;
  display: flex;
  justify-content: space-around;
  padding: 6px 0 max(6px, env(safe-area-inset-bottom));
  border-top: 1px solid var(--border);
  z-index: 100;
}
.nav-item {
  display: flex; flex-direction: column;
  align-items: center; gap: 2px;
  padding: 6px 16px;
  cursor: pointer;
  border-radius: 12px;
  transition: background 0.2s;
}
.nav-item:hover { background: var(--accent-light); }
.nav-item.active { background: var(--accent-light); }
.nav-item.active .nav-icon,
.nav-item.active .nav-label { color: var(--accent); }
.nav-icon { font-size: 20px; line-height: 1; }
.nav-label { font-size: 11px; color: var(--text-muted); }

/* ---- Responsive ---- */
@media (min-width: 768px) {
  .main-content { padding: 24px 24px 24px; }
  .info-grid { grid-template-columns: 1fr 1fr 1fr 1fr; }
}

/* ---- Element Plus Overrides ---- */
.el-button--primary { --el-color-primary: {{.PrimaryColor}}; }
[data-theme="dark"] .el-input__wrapper { background: rgba(30,30,50,0.5); }
[data-theme="dark"] .el-tabs__item { color: var(--text-secondary); }
[data-theme="dark"] .el-tabs__item.is-active { color: var(--accent); }
[data-theme="dark"] .el-form-item__label { color: var(--text-secondary); }
[data-theme="dark"] .el-dropdown-menu { background: var(--glass-bg); }

/* ---- Module Styles (Portal Editor) ---- */
.module-wrap { margin-bottom: 16px; }
.module-hero { padding: 8px 0; }
.module-hero .hero-eyebrow { font-size: 11px; text-transform: uppercase; letter-spacing: 1px; color: var(--accent); margin-bottom: 4px; }
.module-hero .hero-heading { font-size: 22px; font-weight: 700; color: var(--text-primary); margin: 4px 0; }
.module-hero .hero-desc { font-size: 14px; color: var(--text-secondary); margin-bottom: 12px; }
.module-hero .hero-btn { padding: 8px 20px; border: none; border-radius: 8px; background: var(--accent); color: #fff; font-weight: 600; cursor: pointer; font-size: 13px; }
.module-rich-text { font-size: 14px; line-height: 1.7; color: var(--text-primary); }
.module-rich-text h2 { font-size: 18px; margin: 8px 0; }
.module-rich-text h3 { font-size: 16px; margin: 8px 0; }
.module-rich-text p { margin: 6px 0; }
.module-rich-text blockquote { border-left: 3px solid var(--accent); padding-left: 12px; margin: 8px 0; color: var(--text-secondary); }
.module-rich-text ul, .module-rich-text ol { padding-left: 20px; margin: 6px 0; }
.module-notice { border-left: 4px solid; padding: 12px; border-radius: 8px; }
.notice-info { border-color: var(--accent); background: var(--accent-light); }
.notice-success { border-color: var(--success); background: rgba(103,194,58,0.12); }
.notice-warning { border-color: var(--warning); background: rgba(230,162,60,0.12); }
.notice-error { border-color: var(--danger); background: rgba(245,108,108,0.12); }
.module-notice .notice-title { font-weight: 600; margin-bottom: 4px; }
.notice-popup-hint { font-size: 11px; color: var(--text-muted); margin-top: 6px; }
.notice-popup-dialog .el-dialog__body { padding: 0; }
.notice-popup-body { padding: 20px 24px; }
.notice-popup-body.info { border-top: 3px solid var(--accent); }
.notice-popup-body.success { border-top: 3px solid var(--success); }
.notice-popup-body.warning { border-top: 3px solid var(--warning); }
.notice-popup-body.error { border-top: 3px solid var(--danger); }
.notice-popup-footer { display: flex; justify-content: flex-end; gap: 12px; }
.notice-popup-dialog .el-dialog { border-radius: 16px; overflow: hidden; }
.notice-popup-dialog .el-dialog__header { padding: 16px 24px; border-bottom: 1px solid var(--border); }
.notice-popup-dialog .el-dialog__title { font-size: 16px; font-weight: 600; }
.feature-cards-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(130px, 1fr)); gap: 12px; }
.feature-card { padding: 12px; background: var(--accent-light); border-radius: 10px; text-align: center; }
.feature-card-icon { font-size: 24px; display: block; margin-bottom: 4px; }
.feature-card h4 { font-size: 13px; margin: 2px 0; color: var(--text-primary); }
.feature-card p { font-size: 12px; color: var(--text-muted); }
.text-center { text-align: center; }
/* ---- Markdown Render Styles ---- */
.module-markdown { padding: 4px 0; }
.md-render { font-size: 14px; line-height: 1.7; color: var(--text-primary); }
.md-render h1 { font-size: 22px; font-weight: 700; margin: 16px 0 8px; padding-bottom: 6px; border-bottom: 1px solid var(--border); }
.md-render h2 { font-size: 18px; font-weight: 700; margin: 14px 0 6px; padding-bottom: 4px; border-bottom: 1px solid var(--border); }
.md-render h3 { font-size: 16px; font-weight: 600; margin: 12px 0 4px; }
.md-render h4 { font-size: 14px; font-weight: 600; margin: 10px 0 4px; }
.md-render p { margin: 6px 0; }
.md-render a { color: var(--accent); text-decoration: none; }
.md-render a:hover { text-decoration: underline; }
.md-render strong { font-weight: 600; }
.md-render blockquote { border-left: 3px solid var(--accent); padding: 4px 12px; margin: 8px 0; color: var(--text-secondary); background: var(--accent-light); border-radius: 0 6px 6px 0; }
.md-render code { background: rgba(0,0,0,0.06); padding: 2px 6px; border-radius: 4px; font-size: 13px; font-family: 'SF Mono','Fira Code',Consolas,monospace; }
.md-render pre { background: #1e1e2e; color: #cdd6f4; padding: 14px; border-radius: 8px; overflow-x: auto; margin: 10px 0; }
.md-render pre code { background: none; padding: 0; color: inherit; }
.md-render ul, .md-render ol { padding-left: 20px; margin: 6px 0; }
.md-render li { margin: 3px 0; }
.md-render table { border-collapse: collapse; width: 100%; margin: 10px 0; font-size: 13px; }
.md-render th, .md-render td { border: 1px solid var(--border); padding: 6px 10px; text-align: left; }
.md-render th { background: var(--accent-light); font-weight: 600; }
.md-render hr { border: none; border-top: 1px solid var(--border); margin: 16px 0; }
.md-render img { max-width: 100%; border-radius: 8px; }

/* ---- Commercial Modules ---- */
/* Plan List */
.plan-grid { display: grid; gap: 12px; margin-top: 8px; }
.plan-card {
  padding: 16px; background: var(--accent-light); border-radius: 12px;
  text-align: center; transition: transform 0.2s, box-shadow 0.2s; border: 2px solid transparent;
}
.plan-card:hover { transform: translateY(-2px); box-shadow: 0 4px 16px rgba(0,0,0,0.08); }
.plan-card.plan-popular { border-color: var(--accent); }
.plan-name { font-size: 16px; font-weight: 600; color: var(--text-primary); margin-bottom: 4px; }
.plan-desc { font-size: 12px; color: var(--text-muted); margin-bottom: 8px; }
.plan-variant { padding: 10px 0; border-top: 1px solid var(--border); }
.plan-variant:first-of-type { border-top: none; }
.plan-price { margin-bottom: 6px; }
.price-amount { font-size: 26px; font-weight: 700; color: var(--accent); }
.price-currency { font-size: 13px; color: var(--text-muted); margin-left: 4px; }
.price-original { font-size: 13px; color: var(--text-muted); text-decoration: line-through; margin-left: 6px; }
.plan-meta { display: flex; justify-content: center; gap: 8px; margin-bottom: 4px; }
.plan-cycle, .plan-duration { font-size: 12px; color: var(--text-secondary); background: var(--glass-bg); padding: 2px 8px; border-radius: 6px; }
.plan-quota { font-size: 12px; color: var(--text-secondary); margin-bottom: 8px; }
.plan-btn { width: 100%; }

/* Charge Panel */
.charge-section { margin-top: 16px; padding-top: 16px; border-top: 1px solid var(--border); }
.charge-section:first-child { margin-top: 0; padding-top: 0; border-top: none; }
.charge-subtitle { font-size: 14px; font-weight: 600; color: var(--text-primary); margin-bottom: 10px; }
.charge-card-row { display: flex; gap: 8px; }

/* Balance Display */
.balance-hero { text-align: center; padding: 20px 0; }
.balance-amount { font-size: 36px; font-weight: 700; color: var(--accent); }
.balance-frozen { display: block; font-size: 13px; color: var(--text-muted); margin-top: 4px; }

/* Referral */
.referral-link-box {
  background: var(--accent-light); border-radius: 10px; padding: 12px;
  display: flex; align-items: center; gap: 8px; margin-bottom: 16px;
}
.referral-stats {
  display: flex; align-items: center; justify-content: space-around; padding: 16px 0;
  background: var(--accent-light); border-radius: 12px;
}

/* Article List */
.article-filter { margin-bottom: 12px; }
.article-list { display: flex; flex-direction: column; gap: 8px; }
.article-item {
  display: flex; align-items: center; gap: 10px; padding: 10px 12px;
  background: var(--accent-light); border-radius: 10px; cursor: pointer; transition: background 0.2s;
}
.article-item:hover { background: var(--glass-border); }
.article-cat-badge {
  font-size: 11px; font-weight: 600; color: var(--accent);
  background: var(--glass-bg); padding: 2px 8px; border-radius: 6px; flex-shrink: 0;
}
.article-info { flex: 1; display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.article-title { font-size: 14px; font-weight: 500; color: var(--text-primary); }
.sa-icon { width: 32px; height: 32px; border-radius: 8px; flex-shrink: 0; }

/* Ticket Detail */
.ticket-detail-header { display: flex; align-items: center; gap: 8px; margin-bottom: 8px; }
.ticket-subject { font-size: 16px; font-weight: 600; color: var(--text-primary); margin-bottom: 12px; }
.ticket-msgs { max-height: 300px; overflow-y: auto; display: flex; flex-direction: column; gap: 12px; }
.ticket-msg { padding: 10px 12px; border-radius: 10px; }
.ticket-msg.msg-client { background: var(--accent-light); }
.ticket-msg.msg-admin { background: var(--glass-bg); border: 1px solid var(--border); }
.ticket-msg-sender { font-size: 12px; font-weight: 600; color: var(--text-secondary); display: block; margin-bottom: 4px; }
.ticket-msg-content { font-size: 14px; color: var(--text-primary); line-height: 1.6; }
.ticket-reply { margin-top: 16px; border-top: 1px solid var(--border); padding-top: 12px; }

/* Article Dialog */
.article-dialog .el-dialog__body { max-height: 60vh; overflow-y: auto; }`

const defaultPortalJS = `var createApp = Vue.createApp
var ref = Vue.ref
var computed = Vue.computed
var onMounted = Vue.onMounted
var onUnmounted = Vue.onUnmounted
var watch = Vue.watch
var ElMessage = ElementPlus.ElMessage

var app = createApp({
  setup: function() {
    var token = ref(localStorage.getItem('client_token') || '')
    var refreshToken = ref(localStorage.getItem('client_refresh_token') || '')
    var authTab = ref('login')
    var loading = ref(false)
    var rotating = ref(false)
    var refreshing = ref(false)
    var activeTab = ref('overview')
    var showPasswordDialog = ref(false)
    var showQr = ref(false)
    var noticePopupVisible = ref(false)
    var activeNoticePopup = ref(null)
    var logoUrl = ref((typeof PORTAL_CONFIG !== 'undefined' && PORTAL_CONFIG.logo_url) || '')
    var isDark = ref(localStorage.getItem('portal_theme') === 'dark' || (!localStorage.getItem('portal_theme') && window.matchMedia('(prefers-color-scheme: dark)').matches))

    var loginForm = ref({ email: '', password: '' })
    var registerForm = ref({ email: '', password: '', referral_code: '' })
    var pwdForm = ref({ old_password: '', new_password: '', confirm_password: '' })
    var settings = ref({ ip_whitelist: '', device_limit: 0 })
    var client = ref({})

    var logPackages = ref([])
    var logPacking = ref(false)

    // --- Commercial module state ---
    var plans = ref([])
    var plansLoading = ref(false)
    var orderForm = ref({ variant_id: null, order_type: 'new', promotion_code: '', pay_method: 'balance', charge_site_id: null })
    var orderBusy = ref(false)
    var chargeSites = ref([])
    var chargeForm = ref({ site_id: null, amount_yuan: 10 })
    var chargeBusy = ref(false)
    var chargeOrders = ref([])
    var cardCode = ref('')
    var redeemBusy = ref(false)
    var balance = ref({ balance: 0, frozen: 0 })
    var balanceTxns = ref([])
    var referralCode = ref(null)
    var referralStats = ref(null)
    var referralRecords = ref([])
    var tickets = ref([])
    var showTicketDialog = ref(false)
    var showTicketDetail = ref(false)
    var ticketDetail = ref(null)
    var ticketReplyContent = ref('')
    var ticketBusy = ref(false)
    var newTicketForm = ref({ subject: '', category: 'other', priority: 'normal', content: '' })
    var articles = ref([])
    var articleCategory = ref('')
    var showArticleDetail = ref(false)
    var articleDetail = ref(null)
    var sharedAccounts = ref([])
    var showCredentialsDialog = ref(false)
    var sharedAccountCreds = ref(null)

    var registrationEnabled = ref(false)
    if (typeof PORTAL_CONFIG !== 'undefined' && PORTAL_CONFIG.registration_enabled !== undefined) {
      registrationEnabled.value = PORTAL_CONFIG.registration_enabled
    }

    // Portal editor: modules & navigation from PORTAL_CONFIG
    var portalModules = ref([])
    var portalNavItems = ref([])
    // Markdown renderer (using marked.js loaded from CDN)
    var renderMd = function(md) {
      if (!md) return ''
      try { return marked.parse(md, { breaks: true, gfm: true }) } catch(e) { return md }
    }
    if (typeof PORTAL_CONFIG !== 'undefined') {
      if (Array.isArray(PORTAL_CONFIG.modules)) {
        portalModules.value = PORTAL_CONFIG.modules.filter(function(m) { return m.visible !== false })
      }
      if (Array.isArray(PORTAL_CONFIG.navigation)) {
        portalNavItems.value = PORTAL_CONFIG.navigation.filter(function(n) { return n.enabled !== false })
      }
    }
    // Fallback to default nav if none configured
    if (portalNavItems.value.length === 0) {
      portalNavItems.value = [
        { id: 'nav-ov', label: '概览', icon: '📊', target: 'overview', enabled: true },
        { id: 'nav-sub', label: '订阅', icon: '📋', target: 'subscription', enabled: true },
        { id: 'nav-sec', label: '安全', icon: '🔒', target: 'security', enabled: true },
        { id: 'nav-log', label: '日志', icon: '📦', target: 'logs', enabled: true }
      ]
    }
    // Modules for the current page (filtered by activeTab)
    var currentModules = computed(function() {
      return portalModules.value.filter(function(m) { return (m.page || 'overview') === activeTab.value })
    })

    // Inject per-module custom CSS into a <style> tag
    var injectModuleCss = function() {
      var existing = document.getElementById('portal-module-css')
      if (existing) existing.remove()
      var cssParts = []
      portalModules.value.forEach(function(m) {
        if (m.style && m.style.custom_css) {
          var scoped = m.style.custom_css.replace(/([^{}]+)\{/g, function(match, selectors) {
            var scoped2 = selectors.split(',').map(function(s) {
              var t = s.trim()
              if (!t || t.charAt(0) === '@') return t
              return '#mod-' + m.id + ' ' + t
            }).join(', ')
            return scoped2 + ' {'
          })
          cssParts.push('/* Module: ' + (m.title || m.id) + ' */\n' + scoped)
        }
      })
      if (cssParts.length > 0) {
        var style = document.createElement('style')
        style.id = 'portal-module-css'
        style.textContent = cssParts.join('\n')
        document.head.appendChild(style)
      }
    }
    watch(portalModules, injectModuleCss, { immediate: true })

    var onNavClick = function(nav) {
      if (nav.href) {
        if (nav.open_external) window.open(nav.href, '_blank')
        else window.location.href = nav.href
        return
      }
      activeTab.value = nav.target
      // Auto-fetch log packages if the page has log_packages modules
      var hasLogMods = portalModules.value.some(function(m) {
        return (m.page || 'overview') === nav.target && m.type === 'log_packages'
      })
      if (hasLogMods) fetchLogPackages()
      // Auto-fetch commercial module data based on page
      var pageMods = portalModules.value.filter(function(m) { return (m.page || 'overview') === nav.target })
      var hasPlanList = pageMods.some(function(m) { return m.type === 'plan_list' || m.type === 'order_create' })
      var hasCharge = pageMods.some(function(m) { return m.type === 'charge_panel' })
      var hasBalance = pageMods.some(function(m) { return m.type === 'balance_display' })
      var hasReferral = pageMods.some(function(m) { return m.type === 'referral' })
      var hasTickets = pageMods.some(function(m) { return m.type === 'ticket_list' })
      var hasArticles = pageMods.some(function(m) { return m.type === 'article_list' })
      var hasSharedAccts = pageMods.some(function(m) { return m.type === 'shared_accounts' })
      if (hasPlanList) fetchPlans()
      if (hasPlanList || hasCharge) fetchChargeSites()
      if (hasCharge) fetchChargeOrders()
      if (hasBalance) { fetchBalance(); fetchBalanceTxns() }
      if (hasReferral) fetchReferral()
      if (hasTickets) fetchTickets()
      if (hasArticles) fetchArticles()
      if (hasSharedAccts) fetchSharedAccounts()
    }

    // Apply theme
    var applyTheme = function() {
      document.documentElement.setAttribute('data-theme', isDark.value ? 'dark' : 'light')
      localStorage.setItem('portal_theme', isDark.value ? 'dark' : 'light')
    }
    applyTheme()
    var toggleTheme = function() { isDark.value = !isDark.value; applyTheme() }

    // API helper with auto token refresh
    var isRefreshing = false
    var api = function(method, path, body) {
      var headers = { 'Content-Type': 'application/json' }
      if (token.value) headers['Authorization'] = 'Bearer ' + token.value
      return fetch('/api/v1/client-auth' + path, { method: method, headers: headers, body: body ? JSON.stringify(body) : undefined })
        .then(function(res) {
          if (res.status === 401 && refreshToken.value && !isRefreshing) {
            isRefreshing = true
            return fetch('/api/v1/client-auth/refresh', {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({ refresh_token: refreshToken.value })
            }).then(function(refRes) {
              isRefreshing = false
              if (refRes.ok) return refRes.json().then(function(d) {
                token.value = d.data.access_token
                refreshToken.value = d.data.refresh_token
                localStorage.setItem('client_token', token.value)
                localStorage.setItem('client_refresh_token', refreshToken.value)
                // Retry original request
                headers['Authorization'] = 'Bearer ' + token.value
                return fetch('/api/v1/client-auth' + path, { method: method, headers: headers, body: body ? JSON.stringify(body) : undefined })
              })
              doLogout()
              throw new Error('Session expired')
            })
          }
          return res
        })
        .then(function(res) { return res.json().then(function(data) { return { ok: res.ok, status: res.status, data: data } }) })
        .then(function(result) {
          if (!result.ok) throw new Error(result.data.message || result.data.error || 'Error')
          return result.data.data || result.data
        })
    }

    var doLogout = function() {
      token.value = ''
      refreshToken.value = ''
      localStorage.removeItem('client_token')
      localStorage.removeItem('client_refresh_token')
      client.value = {}
    }

    var handleLogin = function() {
      loading.value = true
      api('POST', '/login', loginForm.value).then(function(res) {
        token.value = res.access_token
        refreshToken.value = res.refresh_token
        localStorage.setItem('client_token', token.value)
        localStorage.setItem('client_refresh_token', refreshToken.value)
        client.value = res.client || {}
        settings.value.ip_whitelist = client.value.ip_whitelist || ''
        settings.value.device_limit = client.value.device_limit || 0
        ElMessage.success('登录成功')
      }).catch(function(e) { ElMessage.error(e.message) }).finally(function() { loading.value = false })
    }

    var handleRegister = function() {
      loading.value = true
      api('POST', '/register', registerForm.value).then(function(res) {
        token.value = res.access_token
        refreshToken.value = res.refresh_token
        localStorage.setItem('client_token', token.value)
        localStorage.setItem('client_refresh_token', refreshToken.value)
        client.value = res.client || {}
        ElMessage.success('注册成功')
      }).catch(function(e) { ElMessage.error(e.message) }).finally(function() { loading.value = false })
    }

    var fetchMe = function() {
      refreshing.value = true
      api('GET', '/me').then(function(res) {
        client.value = res
        settings.value.ip_whitelist = res.ip_whitelist || ''
        settings.value.device_limit = res.device_limit || 0
      }).catch(function(e) {
        if (e.message.indexOf('invalid') >= 0 || e.message.indexOf('expired') >= 0 || e.message.indexOf('Session') >= 0) {
          doLogout()
        }
      }).finally(function() { refreshing.value = false })
    }

    var rotateToken = function() {
      rotating.value = true
      api('POST', '/rotate-token').then(function(res) {
        client.value.sub_token = res.token
        ElMessage.success('Token 已轮换')
      }).catch(function(e) { ElMessage.error(e.message) }).finally(function() { rotating.value = false })
    }

    var saveSettings = function() {
      api('PUT', '/settings', settings.value).then(function() {
        ElMessage.success('设置已保存')
      }).catch(function(e) { ElMessage.error(e.message) })
    }

    var changePassword = function() {
      if (!pwdForm.value.old_password || !pwdForm.value.new_password) { ElMessage.error('请填写完整'); return }
      if (pwdForm.value.new_password.length < 6) { ElMessage.error('新密码至少6位'); return }
      if (pwdForm.value.new_password !== pwdForm.value.confirm_password) { ElMessage.error('两次输入的密码不一致'); return }
      api('POST', '/change-password', { old_password: pwdForm.value.old_password, new_password: pwdForm.value.new_password }).then(function() {
        ElMessage.success('密码已修改')
        pwdForm.value = { old_password: '', new_password: '', confirm_password: '' }
      }).catch(function(e) { ElMessage.error(e.message) })
    }

    var handleLogout = function() { doLogout() }

    var copyText = function(text) {
      if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(text).then(function() { ElMessage.success('已复制') }).catch(function() { fallbackCopy(text) })
      } else { fallbackCopy(text) }
    }
    var fallbackCopy = function(text) {
      var ta = document.createElement('textarea')
      ta.value = text; ta.style.position = 'fixed'; ta.style.left = '-9999px'
      document.body.appendChild(ta); ta.select(); document.execCommand('copy')
      document.body.removeChild(ta); ElMessage.success('已复制')
    }

    var formatBytes = function(b) {
      if (!b || b <= 0) return '0 B'
      if (b >= 1073741824) return (b / 1073741824).toFixed(1) + ' GB'
      if (b >= 1048576) return (b / 1048576).toFixed(0) + ' MB'
      return (b / 1024).toFixed(0) + ' KB'
    }
    var formatDate = function(d) { if (!d) return '-'; return new Date(d).toLocaleString('zh-CN') }

    var usagePercent = computed(function() { return client.value.quota_bytes > 0 ? Math.min(100, Math.round(client.value.used_bytes / client.value.quota_bytes * 100)) : 0 })
    var usageColor = computed(function() { return usagePercent.value > 90 ? '#f56c6c' : usagePercent.value > 70 ? '#e6a23c' : '#67c23a' })
    var ringDash = computed(function() {
      var circumference = 2 * Math.PI * 52
      var fill = circumference * usagePercent.value / 100
      return fill + ' ' + (circumference - fill)
    })
    var statusText = computed(function() { return { active: '正常', suspended: '已暂停', expired: '已过期' }[client.value.status] || client.value.status || '-' })
    var subStatusText = computed(function() { return { active: '正常', revoked: '已吊销', expired: '已过期', suspended: '已暂停' }[client.value.sub_status] || client.value.sub_status || '-' })

    var endpoints = ref([])
    var fetchEndpoints = function() {
      fetch('/api/v1/public/subscription-endpoints').then(function(r) { return r.json() }).then(function(d) {
        endpoints.value = Array.isArray(d.data) ? d.data : []
      }).catch(function() {})
    }

    var subUrl = computed(function() {
      if (!client.value.sub_token) return ''
      var t = client.value.sub_token
      var ep = endpoints.value.find(function(e) { return e.enabled }) || endpoints.value[0]
      if (ep && ep.domain) {
        var host = ep.domain
        var path = ep.path || '/'
        var basePath = path.endsWith('/') ? path : path + '/'
        return 'https://' + host + basePath + t
      }
      return window.location.origin + '/sub/' + t
    })

    var qrCodeUrl = ref('')
    var generateQr = function() {
      if (!subUrl.value) { qrCodeUrl.value = ''; return }
      try {
        var canvas = document.createElement('canvas')
        new QRious({ element: canvas, value: subUrl.value, size: 200, background: isDark.value ? '#1a1a2e' : '#ffffff', foreground: isDark.value ? '#e0e0f0' : '#000000', level: 'M' })
        qrCodeUrl.value = canvas.toDataURL('image/png')
      } catch(e) { qrCodeUrl.value = '' }
    }
    var toggleQr = function() { showQr.value = !showQr.value; if (showQr.value && !qrCodeUrl.value) generateQr() }

    // Log packages
    var fetchLogPackages = function() {
      api('GET', '/logs/packages?limit=10').then(function(res) {
        logPackages.value = res.items || []
      }).catch(function() {})
    }

    var createLogPackage = function(hours) {
      logPacking.value = true
      api('POST', '/logs/package', { hours: hours }).then(function(res) {
        ElMessage.success('日志打包任务已创建')
        fetchLogPackages()
      }).catch(function(e) { ElMessage.error(e.message) }).finally(function() { logPacking.value = false })
    }

    var downloadLogPackage = function(id) {
      window.open('/api/v1/client-auth/logs/packages/' + id + '/download?token=' + encodeURIComponent(token.value), '_blank')
    }

    var pkgStatusText = function(s) { return { pending: '等待中', processing: '处理中', done: '已完成', failed: '失败' }[s] || s || '-' }

    // --- formatMoney: converts cents to yuan string ---
    var formatMoney = function(cents) {
      if (!cents || cents === 0) return '0.00'
      return (cents / 100).toFixed(2)
    }

    // --- Plan list: fetch plans ---
    var fetchPlans = function() {
      plansLoading.value = true
      api('GET', '/plans').then(function(res) {
        plans.value = Array.isArray(res) ? res : []
      }).catch(function() {}).finally(function() { plansLoading.value = false })
    }

    // --- Purchase a plan variant ---
    var purchaseVariant = function(variantId, plan, modData) {
      var orderType = (client.value.plan_id > 0 && client.value.plan_id === plan.id) ? 'renew' : 'new'
      orderBusy.value = true
      api('POST', '/orders', { variant_id: variantId, order_type: orderType, pay_method: 'balance' }).then(function(res) {
        if (res.status === 'completed') {
          ElMessage.success('购买成功')
          fetchMe()
        } else if (res.payment_url) {
          ElMessage.info('正在跳转到支付页面...')
          window.open(res.payment_url, '_blank')
        } else {
          ElMessage.success('订单已创建，状态: ' + res.status)
        }
      }).catch(function(e) { ElMessage.error(e.message) }).finally(function() { orderBusy.value = false })
    }

    // --- Submit order form ---
    var submitOrder = function() {
      if (!orderForm.value.variant_id) { ElMessage.error('请选择套餐'); return }
      orderBusy.value = true
      var payload = {
        variant_id: orderForm.value.variant_id, order_type: orderForm.value.order_type,
        promotion_code: orderForm.value.promotion_code, pay_method: orderForm.value.pay_method
      }
      if (orderForm.value.pay_method === 'charge_site' && orderForm.value.charge_site_id) {
        payload.charge_site_id = orderForm.value.charge_site_id
      }
      api('POST', '/orders', payload).then(function(res) {
        if (res.status === 'completed') {
          ElMessage.success('订单完成')
          fetchMe()
        } else if (res.payment_url) {
          ElMessage.info('正在跳转到支付页面...')
          window.open(res.payment_url, '_blank')
        } else {
          ElMessage.success('订单已创建')
        }
      }).catch(function(e) { ElMessage.error(e.message) }).finally(function() { orderBusy.value = false })
    }

    // --- Charge sites ---
    var fetchChargeSites = function() {
      api('GET', '/charge/sites').then(function(res) {
        chargeSites.value = Array.isArray(res) ? res : []
      }).catch(function() {})
    }

    // --- Submit charge (online payment) ---
    var submitCharge = function() {
      if (!chargeForm.value.site_id) { ElMessage.error('请选择充值站点'); return }
      chargeBusy.value = true
      var amountCents = Math.round(chargeForm.value.amount_yuan * 100)
      api('POST', '/charge/create', { site_id: chargeForm.value.site_id, amount: amountCents }).then(function(res) {
        if (res.payment_url) {
          ElMessage.info('正在跳转到支付页面...')
          window.open(res.payment_url, '_blank')
        } else {
          ElMessage.success('充值订单已创建')
        }
        fetchChargeOrders()
      }).catch(function(e) { ElMessage.error(e.message) }).finally(function() { chargeBusy.value = false })
    }

    // --- Redeem charge card ---
    var redeemCard = function() {
      if (!cardCode.value) { ElMessage.error('请输入卡号'); return }
      redeemBusy.value = true
      api('POST', '/charge/redeem-card', { card_code: cardCode.value }).then(function(res) {
        ElMessage.success('充值卡兑换成功')
        cardCode.value = ''
        fetchBalance()
        fetchChargeOrders()
      }).catch(function(e) { ElMessage.error(e.message) }).finally(function() { redeemBusy.value = false })
    }

    // --- Charge orders history ---
    var fetchChargeOrders = function() {
      api('GET', '/charge/orders?limit=10').then(function(res) {
        chargeOrders.value = (res && res.items) ? res.items : []
      }).catch(function() {})
    }

    var chargeStatusText = function(s) { return { pending: '待支付', paid: '已支付', processing: '处理中', expired: '已过期', failed: '失败' }[s] || s || '-' }

    // --- Balance ---
    var fetchBalance = function() {
      api('GET', '/balance').then(function(res) {
        balance.value = res || { balance: 0, frozen: 0 }
      }).catch(function() {})
    }
    var fetchBalanceTxns = function() {
      api('GET', '/balance/transactions?limit=20').then(function(res) {
        balanceTxns.value = (res && res.items) ? res.items : []
      }).catch(function() {})
    }

    // --- Referral ---
    var fetchReferral = function() {
      api('GET', '/referral/code').then(function(res) { referralCode.value = res }).catch(function() {})
      api('GET', '/referral/stats').then(function(res) { referralStats.value = res }).catch(function() {})
      api('GET', '/referral/records?limit=20').then(function(res) {
        referralRecords.value = (res && res.items) ? res.items : []
      }).catch(function() {})
    }
    var referralInviteUrl = computed(function() {
      if (!referralCode.value || !referralCode.value.code) return ''
      return window.location.origin + '/p?ref=' + referralCode.value.code
    })
    var referralStatusText = function(s) { return { pending: '待结算', available: '可提现', withdrawn: '已提现' }[s] || s || '-' }

    // --- Tickets ---
    var fetchTickets = function() {
      api('GET', '/tickets?limit=20').then(function(res) {
        tickets.value = (res && res.items) ? res.items : []
      }).catch(function() {})
    }
    var openTicket = function(id) {
      ticketBusy.value = true
      api('GET', '/tickets/' + id).then(function(res) {
        ticketDetail.value = res
        showTicketDetail.value = true
      }).catch(function(e) { ElMessage.error(e.message) }).finally(function() { ticketBusy.value = false })
    }
    var createTicket = function() {
      if (!newTicketForm.value.subject || !newTicketForm.value.content) { ElMessage.error('请填写主题和内容'); return }
      ticketBusy.value = true
      api('POST', '/tickets', newTicketForm.value).then(function(res) {
        ElMessage.success('工单已创建')
        showTicketDialog.value = false
        newTicketForm.value = { subject: '', category: 'other', priority: 'normal', content: '' }
        fetchTickets()
      }).catch(function(e) { ElMessage.error(e.message) }).finally(function() { ticketBusy.value = false })
    }
    var replyTicket = function() {
      if (!ticketReplyContent.value) { ElMessage.error('请输入回复内容'); return }
      ticketBusy.value = true
      var tid = ticketDetail.value.ticket.id
      api('POST', '/tickets/' + tid + '/reply', { content: ticketReplyContent.value }).then(function() {
        ElMessage.success('回复成功')
        ticketReplyContent.value = ''
        openTicket(tid)
      }).catch(function(e) { ElMessage.error(e.message) }).finally(function() { ticketBusy.value = false })
    }
    var closeTicket = function() {
      if (!ticketDetail.value) return
      ticketBusy.value = true
      var tid = ticketDetail.value.ticket.id
      api('POST', '/tickets/' + tid + '/close').then(function() {
        ElMessage.success('工单已关闭')
        showTicketDetail.value = false
        fetchTickets()
      }).catch(function(e) { ElMessage.error(e.message) }).finally(function() { ticketBusy.value = false })
    }
    var ticketStatusText = function(s) { return { open: '待回复', replied: '已回复', closed: '已关闭' }[s] || s || '-' }
    var ticketStatusBadge = function(s) { return { open: 'processing', replied: 'active', closed: 'none' }[s] || 'none' }

    // --- Articles ---
    var fetchArticles = function() {
      var path = '/articles'
      if (articleCategory.value) path += '?category=' + encodeURIComponent(articleCategory.value)
      api('GET', path).then(function(res) {
        articles.value = Array.isArray(res) ? res : []
      }).catch(function() {})
    }
    var openArticle = function(slug) {
      api('GET', '/articles/' + slug).then(function(res) {
        articleDetail.value = res
        showArticleDetail.value = true
      }).catch(function(e) { ElMessage.error(e.message) })
    }
    var articleCatText = function(c) { return { tutorial: '教程', faq: 'FAQ', notice: '公告', general: '综合' }[c] || c || '-' }

    // --- Shared accounts ---
    var fetchSharedAccounts = function() {
      api('GET', '/shared-accounts').then(function(res) {
        sharedAccounts.value = Array.isArray(res) ? res : []
      }).catch(function() {})
    }
    var applySharedAccount = function(id, modData) {
      api('POST', '/shared-accounts/' + id + '/apply').then(function(res) {
        sharedAccountCreds.value = res
        showCredentialsDialog.value = true
      }).catch(function(e) { ElMessage.error(e.message) })
    }
    var repairSharedAccount = function(id, modData) {
      api('POST', '/shared-accounts/' + id + '/repair').then(function(res) {
        ElMessage.success('修复请求已提交，请稍后查看')
      }).catch(function(e) { ElMessage.error(e.message) })
    }
    var sharedAcctTypeText = function(t) { return { apple_id: 'Apple ID', netflix: 'Netflix', other: '其他' }[t] || t || '-' }

    // --- Scroll to a module by ID ---
    var scrollToModule = function(modId) {
      setTimeout(function() {
        var el = document.getElementById('mod-' + modId)
        if (el) el.scrollIntoView({ behavior: 'smooth', block: 'center' })
      }, 100)
    }

    var switchToLogs = function() { activeTab.value = 'logs'; fetchLogPackages() }

    // Auto-refresh every 30s
    var refreshTimer = null
    var startAutoRefresh = function() {
      if (refreshTimer) clearInterval(refreshTimer)
      refreshTimer = setInterval(function() { if (token.value) fetchMe() }, 30000)
    }

    // Public settings
    var fetchPublicSettings = function() {
      fetch('/api/v1/public/settings').then(function(r) { return r.json() }).then(function(d) {
        if (d.data && d.data.registration_enabled !== undefined) registrationEnabled.value = d.data.registration_enabled
      }).catch(function() {})
    }

    // --- Notice popup logic ---
    var showNoticePopup = function(mod) {
      // Check if dismissed before (for 'once' mode)
      if (mod.data && mod.data.popup_dismiss === 'once') {
        var key = 'notice_dismissed_' + mod.id
        if (localStorage.getItem(key)) return
      }
      activeNoticePopup.value = mod
      noticePopupVisible.value = true
    }

    var dismissNoticePopup = function() {
      var mod = activeNoticePopup.value
      if (mod && mod.data && mod.data.popup_dismiss === 'once') {
        localStorage.setItem('notice_dismissed_' + mod.id, '1')
      }
      noticePopupVisible.value = false
    }

    var openNoticeLink = function(url, external) {
      if (external) window.open(url, '_blank')
      else window.location.href = url
    }

    var checkAutoNoticePopups = function() {
      if (!token.value) return // Only show when logged in
      var pageMods = portalModules.value.filter(function(m) {
        return (m.page || 'overview') === activeTab.value && m.type === 'notice'
      })
      for (var i = 0; i < pageMods.length; i++) {
        var mod = pageMods[i]
        if (mod.data && mod.data.display_mode === 'popup' && mod.data.popup_trigger === 'on_load') {
          if (mod.data.popup_dismiss === 'once') {
            var key = 'notice_dismissed_' + mod.id
            if (localStorage.getItem(key)) continue
          }
          showNoticePopup(mod)
          break // Only show first matching popup
        }
      }
    }

    onMounted(function() {
      fetchPublicSettings()
      if (token.value) {
        fetchMe(); fetchEndpoints(); startAutoRefresh()
        setTimeout(checkAutoNoticePopups, 800)
      }
    })

    // Check for popups when switching pages
    watch(activeTab, function() {
      if (token.value) setTimeout(checkAutoNoticePopups, 300)
    })

    // Check for popups after login
    watch(token, function(v) {
      if (v) setTimeout(checkAutoNoticePopups, 800)
    })

    onUnmounted(function() { if (refreshTimer) clearInterval(refreshTimer) })

    watch(token, function(v) { if (v) { startAutoRefresh() } else { if (refreshTimer) clearInterval(refreshTimer) } })

    // --- Freeform module runtime ---
    var freeformComponents = ref({})

    // Generic API call for freeform ctx (supports query params)
    var apiRaw = function(method, path, body, query) {
      var url = '/api/v1/client-auth' + path
      if (query) {
        var qs = Object.keys(query).map(function(k) { return k + '=' + encodeURIComponent(query[k]) }).join('&')
        if (qs) url += (url.indexOf('?') >= 0 ? '&' : '?') + qs
      }
      var headers = { 'Content-Type': 'application/json' }
      if (token.value) headers['Authorization'] = 'Bearer ' + token.value
      return fetch(url, { method: method, headers: headers, body: body ? JSON.stringify(body) : undefined })
        .then(function(res) {
          if (res.status === 401 && refreshToken.value && !isRefreshing) {
            isRefreshing = true
            return fetch('/api/v1/client-auth/refresh', {
              method: 'POST', headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({ refresh_token: refreshToken.value })
            }).then(function(refRes) {
              isRefreshing = false
              if (refRes.ok) return refRes.json().then(function(d) {
                token.value = d.data.access_token
                refreshToken.value = d.data.refresh_token
                localStorage.setItem('client_token', token.value)
                localStorage.setItem('client_refresh_token', refreshToken.value)
                headers['Authorization'] = 'Bearer ' + token.value
                return fetch(url, { method: method, headers: headers, body: body ? JSON.stringify(body) : undefined })
              })
              doLogout()
              throw new Error('Session expired')
            })
          }
          return res
        })
        .then(function(res) { return res.json().then(function(data) { return { ok: res.ok, status: res.status, data: data } }) })
        .then(function(result) {
          if (!result.ok) throw new Error(result.data.message || result.data.error || 'Error')
          return result.data.data || result.data
        })
    }

    var registerFreeformModules = function() {
      var comps = {}
      portalModules.value.forEach(function(mod) {
        if (mod.type !== 'freeform') return
        var html = (mod.data && mod.data.html) || '<div></div>'
        var js = (mod.data && mod.data.js) || ''
        var css = (mod.data && mod.data.css) || ''

        // Build module context
        var modCtx = {
          ref: Vue.ref, computed: Vue.computed, watch: Vue.watch,
          onMounted: Vue.onMounted, onUnmounted: Vue.onUnmounted,
          data: { client: client, token: token, isDark: isDark },
          api: {
            get: function(p, q) { return apiRaw('GET', p, null, q) },
            post: function(p, b) { return apiRaw('POST', p, b) },
            put: function(p, b) { return apiRaw('PUT', p, b) },
            delete: function(p) { return apiRaw('DELETE', p, null) },
            raw: function(m, p, b) { return apiRaw(m, p, b) }
          },
          formatBytes: formatBytes,
          formatDate: formatDate,
          copyText: copyText,
          elMessage: ElMessage,
          config: (typeof PORTAL_CONFIG !== 'undefined') ? PORTAL_CONFIG : {},
          moduleId: mod.id,
          moduleTitle: mod.title || ''
        }

        // Compile JS via new Function sandbox
        var setupFn
        try {
          var factory = new Function('ctx', js)
          setupFn = function() {
            try {
              var result = factory(modCtx)
              return result || {}
            } catch(e) {
              console.error('[Freeform ' + mod.id + ']', e)
              return { __error: e.message }
            }
          }
        } catch(e) {
          setupFn = function() { return { __error: e.message } }
        }

        // Inject CSS
        if (css) {
          var styleId = 'freeform-css-' + mod.id
          var existing = document.getElementById(styleId)
          if (existing) existing.remove()
          var styleEl = document.createElement('style')
          styleEl.id = styleId
          styleEl.textContent = css
          document.head.appendChild(styleEl)
        }

        comps[mod.id] = { template: html, setup: setupFn }
      })
      freeformComponents.value = comps
    }

    watch(portalModules, registerFreeformModules, { immediate: true })

    return {
      token: token, authTab: authTab, loading: loading, rotating: rotating, refreshing: refreshing,
      activeTab: activeTab, showPasswordDialog: showPasswordDialog, showQr: showQr, logoUrl: logoUrl,
      isDark: isDark, registrationEnabled: registrationEnabled,
      loginForm: loginForm, registerForm: registerForm, pwdForm: pwdForm,
      settings: settings, client: client, endpoints: endpoints,
      logPackages: logPackages, logPacking: logPacking,
      qrCodeUrl: qrCodeUrl,
      handleLogin: handleLogin, handleRegister: handleRegister, rotateToken: rotateToken,
      saveSettings: saveSettings, changePassword: changePassword, handleLogout: handleLogout,
      copyText: copyText, formatBytes: formatBytes, formatDate: formatDate, formatMoney: formatMoney,
      usagePercent: usagePercent, usageColor: usageColor, ringDash: ringDash,
      statusText: statusText, subStatusText: subStatusText, subUrl: subUrl,
      toggleQr: toggleQr, toggleTheme: toggleTheme,
      createLogPackage: createLogPackage, fetchLogPackages: fetchLogPackages,
      downloadLogPackage: downloadLogPackage, pkgStatusText: pkgStatusText,
      switchToLogs: switchToLogs, scrollToModule: scrollToModule,
      portalModules: portalModules, currentModules: currentModules, portalNavItems: portalNavItems, onNavClick: onNavClick, renderMd: renderMd,
      freeformComponents: freeformComponents,
      noticePopupVisible: noticePopupVisible, activeNoticePopup: activeNoticePopup,
      showNoticePopup: showNoticePopup, dismissNoticePopup: dismissNoticePopup, openNoticeLink: openNoticeLink,
      // Commercial modules
      plans: plans, plansLoading: plansLoading, orderForm: orderForm, orderBusy: orderBusy,
      chargeSites: chargeSites, chargeForm: chargeForm, chargeBusy: chargeBusy, chargeOrders: chargeOrders,
      cardCode: cardCode, redeemBusy: redeemBusy,
      balance: balance, balanceTxns: balanceTxns,
      referralCode: referralCode, referralStats: referralStats, referralRecords: referralRecords,
      referralInviteUrl: referralInviteUrl, referralStatusText: referralStatusText,
      tickets: tickets, showTicketDialog: showTicketDialog, showTicketDetail: showTicketDetail,
      ticketDetail: ticketDetail, ticketReplyContent: ticketReplyContent, ticketBusy: ticketBusy,
      newTicketForm: newTicketForm,
      articles: articles, articleCategory: articleCategory,
      showArticleDetail: showArticleDetail, articleDetail: articleDetail,
      sharedAccounts: sharedAccounts, showCredentialsDialog: showCredentialsDialog, sharedAccountCreds: sharedAccountCreds,
      fetchPlans: fetchPlans, purchaseVariant: purchaseVariant, submitOrder: submitOrder,
      fetchChargeSites: fetchChargeSites, submitCharge: submitCharge, redeemCard: redeemCard,
      fetchChargeOrders: fetchChargeOrders, chargeStatusText: chargeStatusText,
      fetchBalance: fetchBalance, fetchBalanceTxns: fetchBalanceTxns, fetchReferral: fetchReferral,
      fetchTickets: fetchTickets, openTicket: openTicket, createTicket: createTicket,
      replyTicket: replyTicket, closeTicket: closeTicket,
      ticketStatusText: ticketStatusText, ticketStatusBadge: ticketStatusBadge,
      fetchArticles: fetchArticles, openArticle: openArticle, articleCatText: articleCatText,
      fetchSharedAccounts: fetchSharedAccounts, applySharedAccount: applySharedAccount,
      repairSharedAccount: repairSharedAccount, sharedAcctTypeText: sharedAcctTypeText
    }
  }
})

app.use(ElementPlus)
app.mount('#app')`
