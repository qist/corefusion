package portal

// defaultPortalHTML is the built-in portal page template.
// It uses {{.SiteName}}, {{.LogoURL}}, {{.RegistrationEnabled}} variables.
const defaultPortalHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no">
  <title>{{.PortalName}}</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/element-plus/dist/index.css">
  <link rel="icon" id="portal-favicon" href="">
  <style id="portal-custom-css"></style>
</head>
<body>
  <div id="app" v-cloak>
    <!-- Auth Screen (dynamic region or fallback) -->
    <div v-if="!token">
      <component :is="authComponent" v-if="authComponent"></component>
      <div v-else class="auth-wrapper">
        <div class="auth-bg"></div>
        <div class="auth-card glass">
          <div class="auth-logo">
            <img v-if="logoUrl" :src="logoUrl" class="logo-img" />
            <div v-else class="logo-icon">⚡</div>
          </div>
          <h1 class="auth-title">{{.PortalName}}</h1>
          <p class="auth-subtitle">{{ registrationEnabled ? t('auth.welcome_login') : t('auth.login_title') }}</p>
          <el-tabs v-model="authTab" class="auth-tabs">
            <el-tab-pane :label="t('auth.login')" name="login">
              <el-form :model="loginForm" class="auth-form">
                <el-form-item>
                  <el-input v-model="loginForm.email" :placeholder="t('auth.email')" prefix-icon="Message" size="large" />
                </el-form-item>
                <el-form-item>
                  <el-input v-model="loginForm.password" type="password" :placeholder="t('auth.password')" prefix-icon="Lock" show-password size="large" @keyup.enter="handleLogin" />
                </el-form-item>
                <el-button type="primary" class="btn-block" :loading="loading" @click="handleLogin" size="large">{{ t('auth.login_btn') }}</el-button>
              </el-form>
            </el-tab-pane>
            <el-tab-pane v-if="registrationEnabled" :label="t('auth.register')" name="register">
              <el-form :model="registerForm" class="auth-form">
                <el-form-item>
                  <el-input v-model="registerForm.email" :placeholder="t('auth.email')" prefix-icon="Message" size="large" />
                </el-form-item>
                <el-form-item>
                  <el-input v-model="registerForm.password" type="password" :placeholder="t('auth.password')" prefix-icon="Lock" show-password size="large" @keyup.enter="handleRegister" />
                </el-form-item>
                <el-form-item>
                  <el-input v-model="registerForm.referral_code" :placeholder="t('auth.invite_code')" prefix-icon="Promotion" size="large" />
                </el-form-item>
                <el-button type="primary" class="btn-block" :loading="loading" @click="handleRegister" size="large">{{ t('auth.register_btn') }}</el-button>
              </el-form>
            </el-tab-pane>
          </el-tabs>
        </div>
      </div>
    </div>

    <!-- Main App (after login) -->
    <div v-else class="app-wrapper">
      <!-- Top Bar -->
      <header class="top-bar">
        <div class="top-bar-left">
          <img v-if="logoUrl" :src="logoUrl" class="top-logo" />
          <span class="top-title">{{.PortalName}}</span>
        </div>
        <div class="top-bar-right">
          <el-dropdown trigger="click" v-if="availableLocales.length > 1" class="locale-switcher">
            <button class="theme-toggle" :title="t('common.language')">
              <span>{{ currentLocaleFlag }}</span>
            </button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item v-for="loc in availableLocales" :key="loc.code" @click="switchLocale(loc.code)">
                  <span>{{ loc.flag }} {{ loc.label }}</span>
                  <el-icon v-if="loc.code === currentLocale" style="margin-left: 4px; color: var(--accent);"><Check /></el-icon>
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
          <button class="theme-toggle" @click="toggleTheme" :title="isDark ? t('theme.light') : t('theme.dark')">
            <span v-if="isDark">☀️</span><span v-else>🌙</span>
          </button>
          <el-dropdown trigger="click">
            <span class="user-badge">
              <span class="user-email">{{ client.email }}</span>
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item @click="showPasswordDialog = true">{{ t('security.password') }}</el-dropdown-item>
                <el-dropdown-item divided @click="handleLogout">{{ t('auth.logout') }}</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </header>

      <!-- Desktop Sidebar -->
      <aside class="app-sidebar">
        <div v-for="nav in portalNavItems" :key="nav.id" :class="'sidebar-item' + (activeTab === nav.target ? ' active' : '')" @click="onNavClick(nav)">
          <span class="sidebar-icon">{{ nav.icon || '🔗' }}</span>
          <span>{{ t(nav.label) }}</span>
        </div>
      </aside>

      <!-- Content Area -->
      <main class="main-content">
        <!-- Two-column layout for overview page: left main + right sidebar (qist style) -->
        <div v-if="activeTab === 'overview'" class="overview-layout">
          <!-- Left column: main modules (exclude notice type) -->
          <div class="overview-main">
            <template v-for="mod in currentModules.filter(function(m){ return m.type !== 'notice' })" :key="mod.id">
              <div class="card glass module-wrap" :id="'mod-' + mod.id" :class="mod.style && mod.style.custom_class || ''">
            <!-- Hero -->
            <div v-if="mod.type === 'hero'" class="module-hero" :style="{ textAlign: mod.style && mod.style.align || 'left' }">
              <div v-if="mod.data && mod.data.eyebrow" class="hero-eyebrow">{{ t(mod.data.eyebrow) }}</div>
              <h2 v-if="mod.data && mod.data.heading" class="hero-heading">{{ t(mod.data.heading) }}</h2>
              <p v-if="mod.data && mod.data.description" class="hero-desc">{{ t(mod.data.description) }}</p>
              <button v-if="mod.data && mod.data.button_text && mod.data.button_target !== 'overview'" class="hero-btn" @click="var target = mod.data.button_target || 'store'; activeTab = target; saveActiveTab(target); fetchPageData(target)">{{ t(mod.data.button_text) }}</button>
            </div>
            <!-- Rich Text -->
            <div v-else-if="mod.type === 'rich_text'" class="module-rich-text" :style="{ textAlign: mod.style && mod.style.align || 'left' }" v-html="mod.data.html"></div>
            <!-- Feature Cards -->
            <div v-else-if="mod.type === 'feature_cards'">
              <div v-if="mod.title" class="card-header"><span class="card-title">{{ t(mod.title) }}</span></div>
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
              <div v-if="mod.title" class="card-header"><span class="card-icon">📊</span><span class="card-title">{{ t(mod.title) }}</span></div>
              <div class="traffic-stats">
                <div class="stat-item"><span class="stat-icon">↑</span><span class="stat-value">{{ formatBytes(client.upload_bytes) }}</span><span class="stat-label">{{ t('traffic.upload') }}</span></div>
                <div class="stat-divider"></div>
                <div class="stat-item"><span class="stat-icon">↓</span><span class="stat-value">{{ formatBytes(client.download_bytes) }}</span><span class="stat-label">{{ t('traffic.download') }}</span></div>
                <div class="stat-divider"></div>
                <div class="stat-item"><span class="stat-icon">💾</span><span class="stat-value">{{ formatBytes(client.quota_bytes) }}</span><span class="stat-label">{{ t('traffic.total') }}</span></div>
              </div>
            </div>
            <!-- Traffic Ring (with SVG ring + horizontal bar) -->
            <div v-else-if="mod.type === 'traffic_ring'">
              <div class="card-header">
                <span class="card-icon">📊</span>
                <span class="card-title">{{ t(mod.title) }}</span>
                <span class="card-action" v-if="refreshing">{{ t('common.refreshing') }}</span>
              </div>
              <div class="traffic-ring-wrap">
                <svg class="traffic-ring" viewBox="0 0 120 120">
                  <circle cx="60" cy="60" r="52" fill="none" class="ring-bg"/>
                  <circle cx="60" cy="60" r="52" fill="none" class="ring-fill" :stroke-dasharray="ringDash" :style="{stroke: usageColor}"/>
                </svg>
                <div class="ring-text">
                  <span class="ring-percent">{{ usagePercent }}%</span>
                  <span class="ring-label">{{ t('traffic.used') }}</span>
                </div>
              </div>
              <div class="traffic-bar-wrap">
                <div class="traffic-bar-header">
                  <span class="bar-label">📊 {{ t('traffic.title') }}</span>
                  <span class="bar-nums">{{ formatBytes(client.used_bytes) }} <span style="color:var(--text-muted);">/</span> {{ formatBytes(client.quota_bytes) }}</span>
                </div>
                <div class="traffic-bar">
                  <div class="traffic-bar-fill" :class="{ warn: usagePercent > 70, danger: usagePercent > 90 }" :style="{ width: usagePercent + '%' }"></div>
                </div>
                <div class="traffic-bar-footer">
                  <span class="bar-hint">{{ t('traffic.upload') }} {{ formatBytes(client.upload_bytes) }} · {{ t('traffic.download') }} {{ formatBytes(client.download_bytes) }}</span>
                  <span class="bar-pct">{{ usagePercent }}%</span>
                </div>
              </div>
              <div class="traffic-stats">
                <div class="stat-item"><span class="stat-icon">↑</span><span class="stat-value">{{ formatBytes(client.upload_bytes) }}</span><span class="stat-label">{{ t('traffic.upload') }}</span></div>
                <div class="stat-divider"></div>
                <div class="stat-item"><span class="stat-icon">↓</span><span class="stat-value">{{ formatBytes(client.download_bytes) }}</span><span class="stat-label">{{ t('traffic.download') }}</span></div>
                <div class="stat-divider"></div>
                <div class="stat-item"><span class="stat-icon">💾</span><span class="stat-value">{{ formatBytes(client.quota_bytes) }}</span><span class="stat-label">{{ t('traffic.total') }}</span></div>
              </div>
            </div>
            <!-- Account Info (qist row style with icon+label+value+badge) -->
            <div v-else-if="mod.type === 'account_info'">
              <div class="card-header"><span class="card-icon">👤</span><span class="card-title">{{ t(mod.title) }}</span></div>
              <div class="acct-info-list" :class="{ 'two-column': mod.data && mod.data.columns === 2 }">
                <div class="acct-info-row">
                  <span class="acct-info-icon">🛡️</span>
                  <div class="acct-info-body">
                    <span class="acct-info-label">{{ t('account.status') }}</span>
                    <div class="acct-info-value-row">
                      <span class="acct-info-value">● {{ statusText }}</span>
                      <span v-if="client.status === 'active'" class="badge badge-active" style="margin-left:6px">{{ t('account.normal') }}</span>
                      <span v-else-if="client.status === 'suspended'" class="badge badge-suspended" style="margin-left:6px">{{ t('account.suspended') }}</span>
                      <span v-else-if="client.status === 'expired'" class="badge badge-expired" style="margin-left:6px">{{ t('common.expired') }}</span>
                      <span v-else class="badge badge-none" style="margin-left:6px">{{ t('common.unknown') }}</span>
                    </div>
                  </div>
                </div>
                <div class="acct-info-row">
                  <span class="acct-info-icon">⭐</span>
                  <div class="acct-info-body">
                    <span class="acct-info-label">{{ t('account.plan') }}</span>
                    <span class="acct-info-value">{{ client.plan_name || t('account.plan_default') }}</span>
                  </div>
                </div>
                <div class="acct-info-row">
                  <span class="acct-info-icon">📧</span>
                  <div class="acct-info-body">
                    <span class="acct-info-label">{{ t('account.email') }}</span>
                    <span class="acct-info-value">{{ client.email }}</span>
                  </div>
                </div>
                <div class="acct-info-row">
                  <span class="acct-info-icon">🕐</span>
                  <div class="acct-info-body">
                    <span class="acct-info-label">{{ t('account.register_time') }}</span>
                    <span class="acct-info-value">{{ client.created_at ? formatDate(client.created_at) : '-' }}</span>
                  </div>
                </div>
                <div class="acct-info-row">
                  <span class="acct-info-icon">🔄</span>
                  <div class="acct-info-body">
                    <span class="acct-info-label">{{ t('account.expire') }}</span>
                    <span class="acct-info-value">{{ client.expires_at ? formatDate(client.expires_at) : t('common.unlimited') }}</span>
                  </div>
                </div>
                <!-- balance display -->
                <div class="acct-info-row" v-if="mod.data && mod.data.show_balance !== false">
                  <span class="acct-info-icon">💰</span>
                  <div class="acct-info-body">
                    <span class="acct-info-label">{{ t('balance.available') }}</span>
                    <div class="acct-info-value-row">
                      <span class="acct-info-value">{{ formatMoney(balance.balance) }}</span>
                      <el-button type="primary" size="small" @click="showChargeDialog = true" style="margin-left: 8px;">{{ t('balance.charge_btn') }}</el-button>
                    </div>
                  </div>
                </div>
              </div>
              <!-- Traffic mini bar inside account info -->
              <div class="acct-traffic-section">
                <div class="acct-traffic-label">
                  <span>📊 {{ t('traffic.title') }}</span>
                  <span class="acct-traffic-pct">{{ usagePercent }}% / {{ formatBytes(client.quota_bytes) }}</span>
                </div>
                <div class="traffic-bar" style="height:8px;border-radius:6px;background:var(--ring-bg);overflow:hidden;margin-top:6px;">
                  <div class="traffic-bar-fill" :class="{ warn: usagePercent > 70, danger: usagePercent > 90 }" :style="{ width: usagePercent + '%' }"></div>
                </div>
                <div class="acct-traffic-hint" style="font-size:11px;color:var(--text-muted);margin-top:4px;">{{ t('traffic.remaining') }} {{ formatBytes(Math.max(0, client.quota_bytes - client.used_bytes)) }} {{ t('traffic.available') }}</div>
              </div>
            </div>
            <!-- Client Download -->
            <div v-else-if="mod.type === 'client_download'">
              <div class="card-header"><span class="card-icon">💻</span><span class="card-title">{{ t(mod.title) }}</span></div>
              <div class="client-platform-tabs">
                <div
                  v-for="p in (mod.data && mod.data.platforms) || []"
                  :key="p.id"
                  class="platform-tab"
                  :class="{ active: selectedPlatform === p.id }"
                  @click="selectPlatform(p.id)"
                >
                  <el-icon v-if="p.icon"><component :is="p.icon" /></el-icon>
                  <span>{{ p.name }}</span>
                </div>
              </div>
              <!-- Hint when no platform selected -->
              <div v-if="!selectedPlatform" class="client-download-hint">
                <el-icon><ArrowUp /></el-icon>
                <span>{{ t((mod.data && mod.data.hint_text) || 'tutorial.select_platform') }}</span>
                <el-icon><ArrowUp /></el-icon>
              </div>
              <!-- Content when platform selected -->
              <div v-else class="client-download-content">
                <div class="client-download-platform-header">
                  <span class="client-download-platform-name">{{ getSelectedPlatformName() }}</span>
                  <el-button text size="small" @click="navigateTo('tutorial')">{{ t('tutorial.view_all') }} →</el-button>
                </div>
                <!-- Download link -->
                <div v-if="getSelectedPlatformUrl(mod)" class="client-download-link-row">
                  <a :href="getSelectedPlatformUrl(mod)" target="_blank" rel="noopener" class="client-download-btn">
                    <el-icon><Download /></el-icon>
                    <span>{{ t('common.download') }} {{ getSelectedPlatformName() }} {{ t('client.title') }}</span>
                  </a>
                </div>
                <!-- Associated tutorial articles -->
                <div v-if="getPlatformArticles(selectedPlatform).length > 0" class="client-download-articles">
                  <div class="client-download-section-title">📖 {{ t('tutorial.related') }}</div>
                  <div class="article-list compact">
                    <div
                      class="article-item"
                      v-for="article in getPlatformArticles(selectedPlatform)"
                      :key="article.id"
                      @click="openArticle(article.slug)"
                    >
                      <span class="article-cat-badge">{{ articleCatText(article.category) }}</span>
                      <div class="article-info">
                        <span class="article-title">{{ article.title }}</span>
                        <span class="logpkg-time">{{ formatDate(article.published_at || article.updated_at) }}</span>
                      </div>
                      <el-icon class="article-arrow"><ArrowRight /></el-icon>
                    </div>
                  </div>
                </div>
                <!-- Empty state -->
                <div v-if="!getSelectedPlatformUrl(mod) && getPlatformArticles(selectedPlatform).length === 0" class="empty-state" style="padding: 24px;">
                  <span class="empty-icon">📦</span>
                  <p class="empty-text">{{ t('tutorial.no_content') }}</p>
                  <p class="empty-hint">{{ t('tutorial.no_content_hint') }}</p>
                </div>
              </div>
            </div>
            <!-- Subscription Detail (dynamic) -->
            <div v-else-if="mod.type === 'subscription_detail'">
              <div class="card-header"><span class="card-icon">🔗</span><span class="card-title">{{ t(mod.title) }}</span></div>
              <div v-if="client.sub_token && client.sub_status !== 'revoked'">
                <div v-if="client.status === 'suspended'" style="padding: 10px 12px; background: rgba(245,108,108,0.1); border: 1px solid rgba(245,108,108,0.3); border-radius: 8px; margin-bottom: 12px;">
                  <p style="color: var(--danger); margin: 0; font-size: 13px;">⚠️ {{ t('account.suspended') }}，{{ t('account.suspended_hint') }}</p>
                </div>
                <div class="sub-link-box">
                  <code class="sub-link-text">{{ showSubUrl && subUrl ? subUrl : '••••••••••••••••••••••••••' }}</code>
                  <div class="sub-link-actions">
                    <el-button size="small" text @click="showSubUrl = !showSubUrl">{{ showSubUrl ? '🙈' : '👁️' }}</el-button>
                    <el-button size="small" text @click="copyText(subUrl)" :disabled="!subUrl">📋</el-button>
                    <el-button size="small" text @click="showQrDialog" :disabled="!subUrl">{{ t('sub.qr_code') }}</el-button>
                  </div>
                </div>
                <div class="sub-actions-row" v-if="mod.data && mod.data.actions">
                  <el-button 
                    v-for="action in mod.data.actions" 
                    :key="action.target"
                    :type="action.type || 'default'"
                    size="small"
                    @click="handleSubAction(action)"
                  >
                    {{ t(action.text) }}
                  </el-button>
                </div>
              </div>
              <div v-else-if="client.sub_status === 'revoked'" class="empty-state">
                <span class="empty-icon">🚫</span><p class="empty-text danger">{{ t('sub.revoked') }}</p><p class="empty-hint">{{ t('sub.revoked_hint') }}</p>
              </div>
<div v-else-if="!client.sub_token" class="empty-state">
<span class="empty-icon">⏸️</span><p class="empty-text danger">{{ t('account.suspended') }}</p><p class="empty-hint">{{ t('account.suspended_hint') }}</p>
</div>
            </div>
            <!-- Streaming Status -->
            <div v-else-if="mod.type === 'streaming_status'">
              <div class="streaming-grid">
                <!-- Apple Shared ID -->
                <div class="streaming-card">
                  <div class="streaming-name">{{ t('shared.apple_id') }}</div>
                  <div class="streaming-icon"><span style="font-size: 32px;">🍎</span></div>
                  <div class="streaming-action" v-if="appleAccount" @click="applySharedAccount(appleAccount.id, mod.data)">{{ t('streaming.view_account') }}</div>
                  <div class="streaming-action" v-else style="color: var(--text-muted); cursor: default;">{{ t('streaming.none') }}</div>
                </div>
                <!-- Netflix Shared Account -->
                <div class="streaming-card">
                  <div class="streaming-name">{{ t('shared.netflix') }}</div>
                  <div class="streaming-icon"><span style="font-size: 32px;">🎬</span></div>
                  <div class="streaming-action" v-if="netflixAccount" @click="applySharedAccount(netflixAccount.id, mod.data)">{{ t('streaming.view_account') }}</div>
                  <div class="streaming-action" v-else style="color: var(--text-muted); cursor: default;">{{ t('streaming.none') }}</div>
                </div>
              </div>
            </div>
            <!-- Subscription (simple button) -->
            <div v-else-if="mod.type === 'subscription'" class="text-center">
              <div v-if="mod.data && mod.data.title" class="card-header"><span class="card-icon">📋</span><span class="card-title">{{ mod.data.title }}</span></div>
              <p v-if="mod.data && mod.data.description" class="card-desc">{{ t(mod.data.description) }}</p>
              <button v-if="mod.data && mod.data.button_text && mod.data.button_target !== 'overview'" class="hero-btn" @click="var target = mod.data.button_target || 'store'; activeTab = target; saveActiveTab(target); fetchPageData(target)">{{ t(mod.data.button_text) }}</button>
            </div>
            <!-- Security Settings (dynamic) -->
            <div v-else-if="mod.type === 'security_settings'">
              <div class="card-header"><span class="card-icon">🔒</span><span class="card-title">{{ t(mod.title) }}</span></div>
              <p v-if="mod.data && mod.data.description" class="card-desc">{{ t(mod.data.description) }}</p>
              <el-form v-if="mod.data && mod.data.show_ip_whitelist !== false" label-position="top" size="small">
                <el-form-item :label="t('security.ip_whitelist_full')">
                  <el-input v-model="settings.ip_whitelist" :placeholder="t('security.ip_example')" />
                </el-form-item>
                <el-form-item :label="t('security.device_limit_hint')">
                  <el-input-number v-model="settings.device_limit" :min="0" :max="100" />
                </el-form-item>
                <el-button type="primary" size="small" @click="saveSettings">{{ t('common.save') }}</el-button>
              </el-form>
              <div v-if="mod.data && mod.data.show_password !== false">
                <div class="card-header" style="margin-top: 20px"><span class="card-icon">🔑</span><span class="card-title">{{ t('security.password') }}</span></div>
                <el-form label-position="top" size="small">
<el-form-item :label="t('security.old_password')"><el-input v-model="pwdForm.old_password" type="password" show-password /></el-form-item>
<el-form-item :label="t('security.new_password')"><el-input v-model="pwdForm.new_password" type="password" show-password /></el-form-item>
<el-form-item :label="t('security.confirm_password')"><el-input v-model="pwdForm.confirm_password" type="password" show-password /></el-form-item>
                  <el-button type="primary" size="small" @click="changePassword">{{ t('security.password') }}</el-button>
                </el-form>
              </div>
            </div>
            <!-- Log Packages (dynamic) -->
            <div v-else-if="mod.type === 'log_packages'">
              <div class="card-header"><span class="card-icon">📦</span><span class="card-title">{{ t(mod.title) }}</span></div>
              <p v-if="mod.data && mod.data.description" class="card-desc">{{ t(mod.data.description) }}</p>
              <p v-else class="card-desc">{{ t('log.description') }}</p>
              <div class="log-actions">
                <el-button type="primary" size="small" @click="createLogPackage(1)" :loading="logPacking">{{ t((mod.data && mod.data.btn_1h) || 'log.btn_1h') }}</el-button>
                <el-button size="small" @click="createLogPackage(6)" :loading="logPacking">{{ t((mod.data && mod.data.btn_6h) || 'log.btn_6h') }}</el-button>
                <el-button size="small" @click="createLogPackage(24)">{{ t((mod.data && mod.data.btn_24h) || 'log.btn_24h') }}</el-button>
              </div>
              <div v-if="logPackages.length > 0" style="margin-top: 16px">
                <div class="card-header"><span class="card-icon">📁</span><span class="card-title">{{ t('log.package_list') }}</span><span class="card-action" @click="fetchLogPackages">{{ t('common.refresh') }}</span></div>
                <div class="logpkg-list">
                  <div class="logpkg-item" v-for="pkg in logPackages" :key="pkg.id">
                    <div class="logpkg-info">
                      <span class="logpkg-id">#{{ pkg.id }}</span>
                      <span :class="'badge badge-' + pkg.status">{{ pkgStatusText(pkg.status) }}</span>
                      <span class="logpkg-time">{{ formatDate(pkg.created_at) }}</span>
                    </div>
                    <div class="logpkg-actions">
                      <span v-if="pkg.file_size" class="logpkg-size">{{ formatBytes(pkg.file_size) }}</span>
                      <el-button v-if="pkg.status === 'done'" size="small" type="primary" text @click="downloadLogPackage(pkg.id)">{{ t('common.download') }}</el-button>
                      <span v-if="pkg.status === 'processing'" class="logpkg-hint">{{ t('common.processing') }}...</span>
                      <span v-if="pkg.error" class="logpkg-error">{{ pkg.error }}</span>
                    </div>
                  </div>
                </div>
              </div>
            </div>
            <!-- Markdown -->
            <div v-else-if="mod.type === 'markdown'" class="module-markdown" :style="{ textAlign: mod.style && mod.style.align || 'left' }">
              <div v-if="mod.title" class="card-header"><span class="card-title">{{ t(mod.title) }}</span></div>
              <div class="md-render" v-html="renderMd(t(mod.data && mod.data.markdown || ''))"></div>
            </div>
            <!-- Custom HTML -->
            <div v-else-if="mod.type === 'custom_html'" v-html="(mod.data && mod.data.html) || ''"></div>
<!-- Plan List -->
            <div v-else-if="mod.type === 'plan_list'">
              <!-- Top Notice -->
              <div v-if="mod.data && mod.data.notice_text" class="plan-notice">
                <div class="plan-notice-content">{{ t(mod.data.notice_text) }}</div>
                <div v-if="mod.data.notice_subtext" class="plan-notice-sub">{{ t(mod.data.notice_subtext) }}</div>
              </div>
              
              <div v-if="plansLoading" class="empty-state"><span class="empty-icon">⏳</span><p class="empty-text">{{ t('common.loading') }}</p></div>
              <div v-else-if="plans.length === 0" class="empty-state"><span class="empty-icon">📭</span><p class="empty-text">{{ t('plan.empty') }}</p></div>
              <div v-else class="plan-table-wrap">
                <table class="plan-table">
                  <thead>
                    <tr>
                      <th class="plan-col-service">{{ t('plan.col_service') }}</th>
                      <th class="plan-col-desc">{{ t('plan.col_desc') }}</th>
                      <th class="plan-col-price">{{ t('plan.col_price') }}</th>
                      <th class="plan-col-action">{{ t('common.action') }}</th>
                    </tr>
                  </thead>
                  <tbody>
                    <template v-for="plan in plans" :key="plan.id">
                      <tr v-for="(v, vIdx) in (plan.variants || [])" :key="v.id" class="plan-row" :class="{ 'plan-current': client.plan_id > 0 && client.plan_id === plan.id }">
                        <!-- Service Column -->
                        <td v-if="vIdx === 0" :rowspan="plan.variants.length" class="plan-cell plan-cell-service">
                          <div class="plan-service-name">{{ plan.name }}</div>
                          <div class="plan-service-meta">
                            <div v-if="v.quota_bytes">{{ t('plan.included_traffic') }}: {{ formatBytes(v.quota_bytes) }}/{{ t('plan.per_month') }}</div>
                            <div v-if="v.speed_limit">{{ t('plan.max_bandwidth') }}: {{ v.speed_limit }}M</div>
                            <div v-if="v.duration_days">{{ t('plan.duration') }}: {{ v.duration_days }}{{ t('plan.days_unit') }}</div>
                          </div>
                        </td>
                        <!-- Description Column -->
                        <td class="plan-cell plan-cell-desc">
                          <div class="plan-desc-text">{{ plan.description || t('plan.default_desc') }}</div>
                        </td>
                        <!-- Price Column -->
                        <td class="plan-cell plan-cell-price">
                          <span class="plan-price-amount">{{ (v.currency || 'CNY') === 'CNY' ? '¥' : (v.currency === 'USD' ? '$' : v.currency) }}{{ formatMoney(v.price) }}</span>
                        </td>
                        <!-- Action Column -->
                        <td class="plan-cell plan-cell-action">
                          <el-button 
                            type="primary" 
                            size="small" 
                            @click="purchaseVariant(v.id, plan, mod.data)"
                            :disabled="orderBusy"
                          >
                            <span v-if="client.plan_id > 0 && client.plan_id === plan.id">{{ t((mod.data && mod.data.renew_text) || 'common.buy') }}</span>
                            <span v-else>{{ t((mod.data && mod.data.buy_text) || 'common.buy') }}</span>
                          </el-button>
                          <div v-if="client.plan_id > 0 && client.plan_id === plan.id" class="plan-current-badge">{{ t('plan.current_plan') }}</div>
                        </td>
                      </tr>
                    </template>
                  </tbody>
                </table>
              </div>
            </div>
            <!-- Order Create -->
            <div v-else-if="mod.type === 'order_create'">
              <div class="card-header"><span class="card-icon">🛒</span><span class="card-title">{{ t(mod.title) }}</span></div>
              <p v-if="mod.data && mod.data.description" class="card-desc">{{ t(mod.data.description) }}</p>
              <el-form label-position="top" size="small">
                <el-form-item :label="t('plan.title')">
                  <el-select v-model="orderForm.variant_id" :placeholder="t('plan.select_placeholder')" style="width: 100%">
                    <el-option v-for="p in plans" :key="p.id" :label="p.name" :value="p.id" />
                  </el-select>
                </el-form-item>
                <el-form-item :label="t('order.type')">
                  <el-radio-group v-model="orderForm.order_type">
                    <el-radio value="new">{{ t('order.type_new') }}</el-radio>
                    <el-radio value="renew">{{ t('common.renew') }}</el-radio>
                    <el-radio value="upgrade">{{ t('common.upgrade') }}</el-radio>
                  </el-radio-group>
                </el-form-item>
                <el-form-item :label="t('plan.promo_code')">
                  <el-input v-model="orderForm.promotion_code" :placeholder="t('plan.promo_placeholder')" />
                </el-form-item>
                <el-form-item :label="t('plan.select_payment')">
                  <el-radio-group v-model="orderForm.pay_method">
                    <el-radio value="balance">{{ t('plan.balance_pay') }}</el-radio>
                    <el-radio value="gift_card">{{ t('plan.card_pay') }}</el-radio>
                    <el-radio v-if="chargeSites.length > 0" value="charge_site">{{ t('plan.charge_pay') }}</el-radio>
                  </el-radio-group>
                </el-form-item>
                <el-form-item v-if="orderForm.pay_method === 'gift_card'" :label="t('plan.card_code')">
                  <el-input v-model="orderForm.gift_card_code" :placeholder="t('plan.card_code_placeholder')" />
                </el-form-item>
                <el-form-item v-if="orderForm.pay_method === 'charge_site' && chargeSites.length > 0" :label="t('charge.site_label')">
                  <el-select v-model="orderForm.charge_site_id" :placeholder="t('plan.select_channel_placeholder')" style="width: 100%">
                    <el-option v-for="s in chargeSites" :key="s.id" :label="s.name" :value="s.id" />
                  </el-select>
                </el-form-item>
                <el-button type="primary" size="small" @click="submitOrder" :loading="orderBusy">{{ t('order.create') }}</el-button>
              </el-form>
            </div>
            <!-- Charge Panel -->
            <div v-else-if="mod.type === 'charge_panel'">
              <div class="card-header"><span class="card-icon">💳</span><span class="card-title">{{ t(mod.title) }}</span></div>
              <p v-if="mod.data && mod.data.description" class="card-desc">{{ t(mod.data.description) }}</p>
              <div class="charge-section">
                <h4 class="charge-subtitle">{{ t('plan.charge_pay') }}</h4>
                <el-form label-position="top" size="small">
                  <el-form-item :label="t('charge.site_label')">
                    <el-select v-model="chargeForm.site_id" :placeholder="t('plan.select_channel_placeholder')" style="width: 100%">
                      <el-option v-for="s in chargeSites" :key="s.id" :label="s.name" :value="s.id" />
                    </el-select>
                  </el-form-item>
                  <el-form-item :label="t('charge.amount_yuan')">
                    <el-input-number v-model="chargeForm.amount_yuan" :min="1" :step="10" style="width: 100%" />
                  </el-form-item>
                  <el-button type="primary" size="small" @click="submitCharge" :loading="chargeBusy">{{ t('charge.submit_now') }}</el-button>
                </el-form>
              </div>
              <div class="charge-section">
                <h4 class="charge-subtitle">{{ t('charge.card_redeem') }}</h4>
                <div class="charge-card-row">
                  <el-input v-model="cardCode" :placeholder="(mod.data && mod.data.card_placeholder) || t('charge.card_code_placeholder')" size="small" />
                  <el-button type="success" size="small" @click="redeemCard" :loading="redeemBusy">{{ (mod.data && mod.data.card_btn_text) || t('charge.card_redeem_btn') }}</el-button>
                </div>
              </div>
              <div v-if="mod.data && mod.data.show_orders !== false && chargeOrders.length > 0" class="charge-section">
                <div class="card-header"><span class="card-icon">📋</span><span class="card-title">{{ t('charge.records') }}</span></div>
                <div class="logpkg-list">
                  <div class="logpkg-item" v-for="co in chargeOrders.slice(0, (mod.data && mod.data.orders_limit) || 5)" :key="co.id">
                    <div class="logpkg-info">
                      <span class="logpkg-id">{{ co.trade_no }}</span>
                      <span :class="'badge badge-' + co.status">{{ chargeStatusText(co.status) }}</span>
                      <span class="logpkg-time">{{ formatMoney(co.amount) }} {{ co.currency }}</span>
                    </div>
                    <div class="logpkg-actions">
                      <span class="logpkg-time">{{ formatDate(co.created_at) }}</span>
                      <a v-if="co.payment_url && co.status === 'pending'" :href="co.payment_url" target="_blank" class="card-action">{{ t('charge.go_pay') }}</a>
                    </div>
                  </div>
                </div>
              </div>
            </div>
            <!-- Balance Display -->
            <div v-else-if="mod.type === 'balance_display'">
              <div class="card-header"><span class="card-icon">💰</span><span class="card-title">{{ t(mod.title) }}</span></div>
              <div class="balance-hero">
                <span class="balance-amount">{{ formatMoney(balance.balance) }}</span>
                <span v-if="balance.frozen > 0" class="balance-frozen">{{ t('balance.frozen') }}: {{ formatMoney(balance.frozen) }}</span>
              </div>
              <div class="card-actions">
                <el-button type="primary" size="small" @click="activeTab = 'store'; scrollToModule('mod-charge')">{{ (mod.data && mod.data.charge_btn_text) || t('balance.charge_btn') }}</el-button>
              </div>
              <div v-if="mod.data && mod.data.show_transactions !== false && balanceTxns.length > 0" style="margin-top: 16px">
                <div class="card-header"><span class="card-icon">📄</span><span class="card-title">{{ t('balance.transactions') }}</span></div>
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
            <!-- Order List -->
            <div v-else-if="mod.type === 'order_list'">
              <!-- Top Notice -->
              <div class="billing-notice">
                <span class="billing-notice-text">{{ t((mod.data && mod.data.notice_text) || 'order.title') }}</span>
                <span v-if="mod.data.notice_highlight" class="billing-notice-highlight">{{ t(mod.data.notice_highlight) }}</span>
              </div>
              <div class="billing-notice-sub">
                {{ t((mod.data && mod.data.notice_subtext) || 'balance.tx_limit') }}
              </div>
              <!-- Order Table -->
              <div v-if="myOrdersLoading" class="empty-state">
                <span class="empty-icon">⏳</span>
                <p class="empty-text">{{ t('common.loading') }}</p>
              </div>
              <div v-else-if="myOrders.length === 0" class="empty-state">
                <span class="empty-icon">📭</span>
                <p class="empty-text">{{ t('order.empty') }}</p>
              </div>
              <div v-else class="billing-table-wrap">
                <table class="billing-table">
                  <thead>
                    <tr>
                      <th>#</th>
                      <th>{{ t('order.id') }}</th>
                      <th>{{ t('plan.col_service') }}</th>
                      <th>{{ t('common.amount') }}</th>
                      <th>{{ t('order.date') }}</th>
                      <th>{{ t('account.expire') }}</th>
                      <th>{{ t('common.status') }}</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="(order, idx) in myOrders.slice(0, (mod.data && mod.data.limit) || 20)" :key="order.id">
                      <td class="billing-col-index">{{ idx + 1 }}</td>
                      <td class="billing-col-id">#{{ order.id }}</td>
                      <td class="billing-col-service">{{ getOrderServiceName(order) }}</td>
                      <td class="billing-col-amount">{{ formatMoney(order.paid_amount || order.original_amount) }}</td>
                      <td class="billing-col-date">{{ formatDate(order.created_at) }}</td>
                      <td class="billing-col-expire">{{ order.effective_to ? formatDate(order.effective_to) : '-' }}</td>
                      <td class="billing-col-status"><span :class="'badge badge-' + orderStatusBadge(order.status)">{{ orderStatusText(order.status) }}</span></td>
                    </tr>
                  </tbody>
                </table>
              </div>
              <!-- Charge Records -->
              <div v-if="chargeOrders.length > 0" style="margin-top: 20px">
                <div class="card-header"><span class="card-icon">📋</span><span class="card-title">{{ t('charge.records') }}</span></div>
                <div class="billing-table-wrap">
                  <table class="billing-table">
                    <thead>
                      <tr>
                        <th>#</th>
                        <th>{{ t('order.trade_no') }}</th>
                        <th>{{ t('order.source') }}</th>
                        <th>{{ t('common.amount') }}</th>
                        <th>{{ t('common.time') }}</th>
                        <th>{{ t('common.status') }}</th>
                      </tr>
                    </thead>
                    <tbody>
                      <tr v-for="(co, idx) in chargeOrders.slice(0, 10)" :key="co.id">
                        <td class="billing-col-index">{{ idx + 1 }}</td>
                        <td class="billing-col-id">{{ co.trade_no }}</td>
                        <td>{{ co.source === 'card' ? t('charge.card_title') : (co.source === 'site' ? t('plan.charge_pay') : co.source) }}</td>
                        <td class="billing-col-amount">{{ formatMoney(co.amount) }} {{ co.currency }}</td>
                        <td class="billing-col-date">{{ formatDate(co.created_at) }}</td>
                        <td class="billing-col-status"><span :class="'badge badge-' + co.status">{{ chargeStatusText(co.status) }}</span></td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              </div>
              <!-- Transaction Flow -->
              <div v-if="balanceTxns.length > 0" style="margin-top: 20px">
                <div class="card-header"><span class="card-icon">📄</span><span class="card-title">{{ t('balance.balance_change') }}</span></div>
                <div class="billing-table-wrap">
                  <table class="billing-table">
                    <thead>
                      <tr>
                        <th>#</th>
                        <th>{{ t('common.type') }}</th>
                        <th>{{ t('common.amount') }}</th>
                        <th>{{ t('balance.balance_after') }}</th>
                        <th>{{ t('common.remark') }}</th>
                        <th>{{ t('common.time') }}</th>
                      </tr>
                    </thead>
                    <tbody>
                      <tr v-for="(tx, idx) in balanceTxns.slice(0, 10)" :key="tx.id">
                        <td class="billing-col-index">{{ idx + 1 }}</td>
                        <td><span :class="'badge badge-' + (tx.amount >= 0 ? 'active' : 'expired')">{{ tx.type === 'charge' ? t('balance.charge_btn') : (tx.type === 'consume' ? t('balance.type_consume') : (tx.type === 'refund' ? t('order.refund') : tx.type)) }}</span></td>
                        <td class="billing-col-amount" :style="{ color: tx.amount >= 0 ? '#67c23a' : '#f56c6c' }">{{ tx.amount >= 0 ? '+' : '' }}{{ formatMoney(tx.amount) }}</td>
                        <td class="billing-col-amount">{{ formatMoney(tx.balance_after) }}</td>
                        <td>{{ tx.remark || '-' }}</td>
                        <td class="billing-col-date">{{ formatDate(tx.created_at) }}</td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              </div>
            </div>
            <!-- Node List -->
            <div v-else-if="mod.type === 'node_list'" class="node-list-module">
              <div v-if="mod.data && mod.data.info_text" class="node-info-card">
                <div class="node-info-title">{{ t(mod.data.info_title || 'node.info_title') }}</div>
                <div class="node-info-content">{{ t(mod.data.info_text) }}</div>
              </div>
              <div v-if="plansLoading" class="empty-state">
                <span class="empty-icon">⏳</span>
                <p class="empty-text">{{ t('common.loading') }}</p>
              </div>
              <div v-else-if="allPlanNodes.length === 0" class="empty-state">
                <span class="empty-icon">📭</span>
                <p class="empty-text">{{ t('node.empty') }}</p>
              </div>
              <div v-else>
                <div v-for="planGroup in allPlanNodes" :key="planGroup.plan_id" class="plan-node-group">
                  <div class="plan-node-header">
                    <span class="plan-node-name">{{ planGroup.plan_name }}</span>
                    <span v-if="planGroup.active" class="badge badge-active">{{ t('account.plan') }}</span>
                    <span v-else class="badge badge-none">{{ t('node.not_purchased') }}</span>
                  </div>
                  <div class="node-grid" :style="{ gridTemplateColumns: 'repeat(' + ((mod.data && mod.data.columns) || 3) + ', 1fr)' }">
                    <div v-for="node in planGroup.nodes" :key="node.id" class="node-card" :class="{ disabled: !planGroup.active }">
                      <div class="node-name">{{ node.name }}</div>
                      <div class="node-tags">
                        <span v-if="node.protocol" class="node-tag tag-protocol">{{ node.protocol }}</span>
                        <span v-if="node.transport" class="node-tag tag-transport">{{ node.transport }}</span>
                      </div>
                      <div v-if="planGroup.active && node.link" class="node-actions">
                        <el-button size="small" text @click="showNodeDetail(node)">{{ t('common.view_detail') }}</el-button>
                        <el-button size="small" text @click="showNodeQR(node)">{{ t('sub.qr_code') }}</el-button>
                      </div>
                      <div v-else class="node-locked">
                        <span class="lock-hint">🔒 {{ t('node.need_plan') }}</span>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>
            <!-- Referral -->
            <div v-else-if="mod.type === 'referral'">
              <!-- Referral Link -->
              <div class="referral-header">
                <div class="referral-title">🎁 {{ t('referral.my_link') }}</div>
                <div class="referral-meta">
                  <span class="referral-rate">{{ t('referral.commission_rate') }}: {{ (referralStats && referralStats.commission_rate) ? referralStats.commission_rate + '%' : ((mod.data && mod.data.default_commission_rate) || '15') + '%' }}</span>
                  <span class="referral-divider">|</span>
                  <span class="referral-mode">{{ t('referral.commission_mode') }}: {{ (referralStats && referralStats.commission_mode === 'recurring') ? t('referral.mode_recurring') : ((mod.data && mod.data.default_commission_mode === 'recurring') ? t('referral.mode_recurring') : t('referral.mode_normal')) }}</span>
                </div>
              </div>
              <div class="referral-link-wrap" v-if="referralCode">
                <div class="referral-link">{{ referralInviteUrl }}</div>
                <el-button type="primary" size="small" @click="copyText(referralInviteUrl)" class="referral-copy-btn">
                  <el-icon><DocumentCopy /></el-icon> {{ t('common.copy') }}
                </el-button>
              </div>
              
              <!-- Invite Code -->
              <div class="referral-code-section" v-if="referralCode">
                <div class="referral-code-row">
                  <span class="referral-code-label">{{ t('referral.invite_code') }}</span>
                  <span class="referral-code-value">{{ referralCode.code }}</span>
                  <el-tag size="small" type="success" effect="dark" class="referral-status">{{ t('referral.code_available') }}</el-tag>
                </div>
              </div>

              <!-- Invite Records -->
              <div class="referral-section" v-if="referralRecords.length > 0">
                <div class="section-title">{{ t('referral.invite_records') }}</div>
                <div class="referral-table-wrap">
                  <table class="referral-table">
                    <thead>
                      <tr>
                        <th>#</th>
                        <th>{{ t('account.email') }}</th>
                        <th>{{ t('account.register_time') }}</th>
                      </tr>
                    </thead>
                    <tbody>
                      <tr v-for="(r, idx) in referralRecords.slice(0, (mod.data && mod.data.records_limit) || 10)" :key="r.id">
                        <td>{{ idx + 1 }}</td>
                        <td>{{ r.referee_email }}</td>
                        <td>{{ formatDateTime(r.registered_at) }}</td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              </div>

              <!-- Commission Records -->
              <div class="referral-section">
                <div class="section-header">
                  <span class="section-title">{{ t('referral.records') }}</span>
                  <span class="section-subtitle">{{ t('referral.withdraw_hint') }}</span>
                  <el-button type="danger" size="small" class="withdraw-btn" v-if="referralStats && referralStats.available_commission > 0">{{ t('referral.withdraw') }}</el-button>
                </div>
                <div class="referral-table-wrap">
                  <table class="referral-table">
                    <thead>
                      <tr>
                        <th>#</th>
                        <th>{{ t('referral.commission_time') }}</th>
                        <th>{{ t('referral.consumer') }}</th>
                        <th>{{ t('referral.consume_amount') }}</th>
                        <th>{{ t('referral.commission_amount') }}</th>
                        <th>{{ t('common.status') }}</th>
                      </tr>
                    </thead>
                    <tbody>
                      <tr v-for="(r, idx) in commissionRecords" :key="r.id">
                        <td>{{ idx + 1 }}</td>
                        <td>{{ formatDateTime(r.created_at) }}</td>
                        <td>{{ r.consumer_email }}</td>
                        <td>¥{{ r.amount }}</td>
                        <td>¥{{ r.commission }}</td>
                        <td>
                          <el-tag size="small" :type="r.status === 'available' ? 'success' : r.status === 'pending' ? 'warning' : 'info'" effect="plain">
                            {{ r.status === 'available' ? t('referral.status_available') : r.status === 'pending' ? t('referral.status_pending') : t('referral.status_withdrawn') }}
                          </el-tag>
                        </td>
                      </tr>
                    </tbody>
                  </table>
                </div>
                <div class="referral-summary" v-if="referralStats">
                  {{ t('referral.total_commission') }} {{ formatMoney(referralStats.total_commission) }}（{{ referralStats.total_paid }}{{ t('referral.times' ) }}），{{ t('referral.withdraw_threshold' ) }}
                </div>
              </div>

              <!-- Withdrawal Records -->
              <div class="referral-section" v-if="withdrawRecords && withdrawRecords.length > 0">
                <div class="section-title">{{ t('referral.withdraw_records') }}</div>
                <div class="referral-table-wrap">
                  <table class="referral-table">
                    <thead>
                      <tr>
                        <th>#</th>
                        <th>{{ t('referral.withdraw_date') }}</th>
                        <th>{{ t('referral.withdraw_amount') }}</th>
                        <th>{{ t('common.status') }}</th>
                      </tr>
                    </thead>
                    <tbody>
                      <tr v-for="(w, idx) in withdrawRecords" :key="w.id">
                        <td>{{ idx + 1 }}</td>
                        <td>{{ formatDateTime(w.created_at) }}</td>
                        <td>{{ w.amount }}</td>
                        <td>
                          <el-tag size="small" :type="w.status === 'completed' ? 'success' : w.status === 'pending' ? 'warning' : 'info'" effect="plain">
                            {{ w.status === 'completed' ? t('referral.status_paid') : w.status === 'pending' ? t('common.processing') : t('order.status_cancelled') }}
                          </el-tag>
                        </td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              </div>
            </div>
            <!-- Ticket List -->
            <div v-else-if="mod.type === 'ticket_list'">
              <div class="card-header"><span class="card-icon">🎫</span><span class="card-title">{{ t(mod.title) }}</span><span class="card-action" @click="showTicketDialog = true">{{ t((mod.data && mod.data.new_btn_text) || 'ticket.new') }}</span></div>
              <div v-if="tickets.length === 0" class="empty-state"><span class="empty-icon">📭</span><p class="empty-text">{{ t('ticket.empty') }}</p></div>
<div v-else class="logpkg-list">
<div class="logpkg-item" v-for="tk in tickets" :key="tk.id" @click="openTicket(tk.id)" style="cursor: pointer">
<div class="logpkg-info">
<span class="logpkg-id">#{{ tk.id }}</span>
<span class="logpkg-time">{{ tk.subject }}</span>
<span :class="'badge badge-' + ticketStatusBadge(tk.status)">{{ ticketStatusText(tk.status) }}</span>
<span v-if="tk.status === 'replied'" class="ticket-new-badge">{{ t('ticket.new_reply') }}</span>
</div>
<div class="logpkg-actions">
<span class="logpkg-time">{{ formatDate(tk.updated_at) }}</span>
</div>
</div>
</div>
            </div>
            <!-- Tutorial Cards -->
            <div v-else-if="mod.type === 'tutorial_cards'">
              <div class="card-header"><span class="card-icon">📥</span><span class="card-title">{{ t(mod.title) }}</span></div>
              <!-- Platform Tabs -->
              <div class="tutorial-platform-tabs">
                <div
                  v-for="card in (mod.data && mod.data.cards) || []"
                  :key="card.id"
                  class="tutorial-tab"
                  :class="{ active: selectedPlatform === card.id }"
                  @click="selectPlatform(card.id)"
                >
                  {{ card.name }}
                </div>
              </div>
              <!-- Platform Grid -->
              <div class="tutorial-grid">
                <div
                  v-for="card in (mod.data && mod.data.cards) || []"
                  :key="card.id"
                  class="tutorial-card"
                  :class="{ active: selectedPlatform === card.id }"
                  :style="{ background: card.bg || card.color }"
                  @click="selectPlatform(card.id)"
                >
                  <div class="tutorial-card-icon">{{ getTutorialIcon(card.icon) }}</div>
                  <div class="tutorial-card-name">{{ card.name }}</div>
                  <div class="tutorial-card-desc">{{ t(card.desc) }}</div>
                </div>
              </div>
              <!-- Related Articles -->
              <div v-if="selectedPlatform" class="tutorial-articles">
                <div class="tutorial-articles-header">
                  <span class="tutorial-platform-name">{{ getSelectedPlatformName() }}</span>
                  <span class="tutorial-articles-count">{{ getPlatformArticles(selectedPlatform).length }} {{ selectedPlatform === 'faq' ? t('article.faq_count') : t('tutorial.platform_articles') }}</span>
                </div>
                <div v-if="getPlatformArticles(selectedPlatform).length === 0" class="empty-state" style="padding: 24px;">
                  <span class="empty-icon">📝</span>
                  <p class="empty-text">{{ selectedPlatform === 'faq' ? t('tutorial.no_faq') : t('tutorial.no_articles') }}</p>
                </div>
                <div v-else class="article-list tutorial-article-list">
                  <div
                    class="article-item"
                    v-for="article in getPlatformArticles(selectedPlatform)"
                    :key="article.id"
                    @click="openArticle(article.slug)"
                  >
                    <span class="article-cat-badge">{{ articleCatText(article.category) }}</span>
                    <div class="article-info">
                      <span class="article-title">{{ article.title }}</span>
                      <span class="logpkg-time">{{ formatDate(article.published_at || article.updated_at) }}</span>
                    </div>
                    <span class="card-action">{{ t('article.read') }}</span>
                  </div>
                </div>
              </div>
              <div v-else class="tutorial-select-hint">
                <span class="empty-icon">👆</span>
                <p class="empty-text">{{ t('tutorial.select_platform') }}</p>
              </div>
            </div>
            <!-- Article List -->
            <div v-else-if="mod.type === 'article_list'">
              <div class="card-header"><span class="card-icon">📚</span><span class="card-title">{{ t(mod.title) }}</span></div>
              <div v-if="mod.data && mod.data.show_category_filter" class="article-filter">
                <el-radio-group v-model="articleCategory" size="small" @change="fetchArticles">
                  <el-radio-button value="">{{ t('common.all') }}</el-radio-button>
                  <el-radio-button value="tutorial">{{ t('article.tutorial') }}</el-radio-button>
                  <el-radio-button value="faq">FAQ</el-radio-button>
                  <el-radio-button value="notice">{{ t('article.notice') }}</el-radio-button>
                </el-radio-group>
              </div>
              <div v-if="articles.length === 0" class="empty-state"><span class="empty-icon">📭</span><p class="empty-text">{{ t('article.empty') }}</p></div>
              <div v-else class="article-list">
                <div class="article-item" v-for="a in articles" :key="a.id" @click="openArticle(a.slug)">
                  <span class="article-cat-badge">{{ articleCatText(a.category) }}</span>
                  <div class="article-info">
                    <span class="article-title">{{ a.title }}</span>
                    <span class="logpkg-time">{{ formatDate(a.published_at || a.updated_at) }}</span>
                  </div>
                  <span class="card-action">{{ t('article.read') }}</span>
                </div>
              </div>
            </div>
            <!-- Shared Accounts -->
            <div v-else-if="mod.type === 'shared_accounts'">
              <div class="card-header"><span class="card-icon">🔑</span><span class="card-title">{{ t(mod.title) }}</span></div>
              <p v-if="mod.data && mod.data.description" class="card-desc">{{ t(mod.data.description) }}</p>
              <!-- No Plan Access -->
              <div v-if="!client.plan_id" class="empty-state">
                <span class="empty-icon">🔒</span>
                <p class="empty-text">{{ t('shared.need_plan') }}</p>
                <el-button type="primary" size="small" @click="activeTab = 'services'" style="margin-top: 12px">{{ t('plan.go_subscribe') }}</el-button>
              </div>
              <!-- Has Plan, No Accounts -->
              <div v-else-if="sharedAccounts.length === 0" class="empty-state"><span class="empty-icon">📭</span><p class="empty-text">{{ t('shared.empty') }}</p></div>
              <!-- Has Plan And Accounts -->
              <div v-else class="article-list">
                <div class="article-item" v-for="sa in sharedAccounts" :key="sa.id">
                  <img v-if="sa.icon_url" :src="sa.icon_url" class="sa-icon" />
                  <span v-else class="article-cat-badge">{{ sharedAcctTypeText(sa.type) }}</span>
                  <div class="article-info">
                    <span class="article-title">{{ sa.name }}</span>
                    <span class="logpkg-time">{{ sharedAcctTypeText(sa.type) }}</span>
                  </div>
                  <el-button size="small" type="primary" text @click="applySharedAccount(sa.id, mod.data)">{{ t((mod.data && mod.data.apply_btn_text) || 'shared.view_credential') }}</el-button>
                  <el-button size="small" text @click="repairSharedAccount(sa.id, mod.data)">{{ t((mod.data && mod.data.repair_btn_text) || 'shared.repair') }}</el-button>
                </div>
              </div>
            </div>
            <!-- Freeform (HTML+CSS+JS custom component) -->
            <div v-else-if="mod.type === 'freeform'" class="module-freeform">
              <component :is="freeformComponents[mod.id]" v-if="freeformComponents[mod.id]"></component>
              <div v-else style="color:#f56c6c;padding:12px;font-size:13px">⚠️ {{ t('error.load_failed') }}</div>
            </div>
          </div>
        </template>
        <!-- Empty state for overview main column -->
        <div v-if="currentModules.filter(function(m){ return m.type !== 'notice' }).length === 0" class="card glass">
          <div class="empty-state">
            <span class="empty-icon">📋</span>
            <p class="empty-text">{{ t('common.empty') }}</p>
          </div>
        </div>
      </div>

      <!-- Right column: announcement list -->
      <aside class="overview-sidebar">
        <!-- Announcements -->
        <div v-if="announcements.length > 0" class="card glass announcement-sidebar-card">
          <div class="card-header"><span class="card-icon">📢</span><span class="card-title">{{ t('announcement.title') }}</span></div>
          <div class="announcement-sidebar-list">
            <div v-for="ann in announcements.slice(0, 10)" :key="ann.id" class="announcement-sidebar-item" @click="openAnnouncementPopup(ann)">
              <span v-if="ann.is_popup" class="announcement-popup-badge">📌</span>
              <span class="announcement-sidebar-date">[{{ formatDate(ann.published_at || ann.updated_at).slice(5,10) }}]</span>
              <span class="announcement-sidebar-text">{{ ann.title }}</span>
            </div>
          </div>
        </div>
      </aside>
    </div>

    <!-- Non-overview pages: single column layout -->
    <template v-if="activeTab !== 'overview'" v-for="mod in currentModules" :key="mod.id">
      <div class="card glass module-wrap" :id="'mod-' + mod.id" :class="mod.style && mod.style.custom_class || ''">
        <!-- Hero -->
        <div v-if="mod.type === 'hero'" class="module-hero" :style="{ textAlign: mod.style && mod.style.align || 'left' }">
          <div v-if="mod.data && mod.data.eyebrow" class="hero-eyebrow">{{ t(mod.data.eyebrow) }}</div>
          <h2 v-if="mod.data && mod.data.heading" class="hero-heading">{{ t(mod.data.heading) }}</h2>
          <p v-if="mod.data && mod.data.description" class="hero-desc">{{ t(mod.data.description) }}</p>
          <button v-if="mod.data && mod.data.button_text && mod.data.button_target !== 'overview'" class="hero-btn" @click="var target = mod.data.button_target || 'store'; activeTab = target; saveActiveTab(target); fetchPageData(target)">{{ t(mod.data.button_text) }}</button>
        </div>
        <!-- Rich Text -->
        <div v-else-if="mod.type === 'rich_text'" class="module-rich-text" :style="{ textAlign: mod.style && mod.style.align || 'left' }" v-html="mod.data.html"></div>
            <!-- Announcement list (for non-overview pages) -->
            <div v-else-if="mod.type === 'notice'" class="card glass announcement-sidebar-card" style="margin-bottom: 16px;">
              <div class="card-header"><span class="card-icon">📢</span><span class="card-title">{{ t(mod.title) }}</span></div>
              <div class="announcement-sidebar-list">
                <div v-for="ann in announcements.slice(0, 10)" :key="ann.id" class="announcement-sidebar-item" @click="openAnnouncementPopup(ann)">
                  <span v-if="ann.is_popup" class="announcement-popup-badge">📌</span>
                  <span class="announcement-sidebar-date">[{{ formatDate(ann.published_at || ann.updated_at).slice(5,10) }}]</span>
                  <span class="announcement-sidebar-text">{{ ann.title }}</span>
                </div>
                <div v-if="announcements.length === 0" style="padding:16px;text-align:center;color:var(--text-muted);font-size:13px">{{ t('announcement.empty') }}</div>
              </div>
            </div>
        <!-- Feature Cards -->
        <div v-else-if="mod.type === 'feature_cards'">
          <div v-if="mod.title" class="card-header"><span class="card-title">{{ t(mod.title) }}</span></div>
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
          <div v-if="mod.title" class="card-header"><span class="card-icon">📊</span><span class="card-title">{{ t(mod.title) }}</span></div>
          <div class="traffic-stats">
            <div class="stat-item"><span class="stat-icon">↑</span><span class="stat-value">{{ formatBytes(client.upload_bytes) }}</span><span class="stat-label">{{ t('traffic.upload') }}</span></div>
            <div class="stat-divider"></div>
            <div class="stat-item"><span class="stat-icon">↓</span><span class="stat-value">{{ formatBytes(client.download_bytes) }}</span><span class="stat-label">{{ t('traffic.download') }}</span></div>
            <div class="stat-divider"></div>
            <div class="stat-item"><span class="stat-icon">💾</span><span class="stat-value">{{ formatBytes(client.quota_bytes) }}</span><span class="stat-label">{{ t('traffic.total') }}</span></div>
          </div>
        </div>
        <!-- Traffic Ring -->
        <div v-else-if="mod.type === 'traffic_ring'">
          <div class="card-header">
            <span class="card-icon">📊</span>
            <span class="card-title">{{ t(mod.title) }}</span>
            <span class="card-action" v-if="refreshing">{{ t('common.refreshing') }}</span>
          </div>
          <div class="traffic-ring-wrap">
            <svg class="traffic-ring" viewBox="0 0 120 120">
              <circle cx="60" cy="60" r="52" fill="none" class="ring-bg"/>
              <circle cx="60" cy="60" r="52" fill="none" class="ring-fill" :stroke-dasharray="ringDash" :style="{stroke: usageColor}"/>
            </svg>
            <div class="ring-text">
              <span class="ring-percent">{{ usagePercent }}%</span>
              <span class="ring-label">{{ t('traffic.used') }}</span>
            </div>
          </div>
          <div class="traffic-bar-wrap">
            <div class="traffic-bar-header">
              <span class="bar-label">📊 {{ t('traffic.title') }}</span>
              <span class="bar-nums">{{ formatBytes(client.used_bytes) }} <span style="color:var(--text-muted);">/</span> {{ formatBytes(client.quota_bytes) }}</span>
            </div>
            <div class="traffic-bar">
              <div class="traffic-bar-fill" :class="{ warn: usagePercent > 70, danger: usagePercent > 90 }" :style="{ width: usagePercent + '%' }"></div>
            </div>
            <div class="traffic-bar-footer">
              <span class="bar-hint">{{ t('traffic.upload') }} {{ formatBytes(client.upload_bytes) }} · {{ t('traffic.download') }} {{ formatBytes(client.download_bytes) }}</span>
              <span class="bar-pct">{{ usagePercent }}%</span>
            </div>
          </div>
          <div class="traffic-stats">
            <div class="stat-item"><span class="stat-icon">↑</span><span class="stat-value">{{ formatBytes(client.upload_bytes) }}</span><span class="stat-label">{{ t('traffic.upload') }}</span></div>
            <div class="stat-divider"></div>
            <div class="stat-item"><span class="stat-icon">↓</span><span class="stat-value">{{ formatBytes(client.download_bytes) }}</span><span class="stat-label">{{ t('traffic.download') }}</span></div>
            <div class="stat-divider"></div>
            <div class="stat-item"><span class="stat-icon">💾</span><span class="stat-value">{{ formatBytes(client.quota_bytes) }}</span><span class="stat-label">{{ t('traffic.total') }}</span></div>
          </div>
        </div>
        <!-- Account Info (qist style with icon+label+value+badge) -->
        <div v-else-if="mod.type === 'account_info'">
          <div class="card-header"><span class="card-icon">👤</span><span class="card-title">{{ t(mod.title) }}</span></div>
          <div class="acct-info-list" :class="{ 'two-column': mod.data && mod.data.columns === 2 }">
            <div class="acct-info-row">
              <span class="acct-info-icon">🛡️</span>
              <div class="acct-info-body">
                <span class="acct-info-label">{{ t('account.status') }}</span>
                <div class="acct-info-value-row">
                  <span class="acct-info-value">● {{ statusText }}</span>
                  <span v-if="client.status === 'active'" class="badge badge-active" style="margin-left:6px">{{ t('account.normal') }}</span>
                  <span v-else-if="client.status === 'suspended'" class="badge badge-suspended" style="margin-left:6px">{{ t('account.suspended') }}</span>
                  <span v-else-if="client.status === 'expired'" class="badge badge-expired" style="margin-left:6px">{{ t('common.expired') }}</span>
                  <span v-else class="badge badge-none" style="margin-left:6px">{{ t('common.unknown') }}</span>
                </div>
              </div>
            </div>
            <div class="acct-info-row">
              <span class="acct-info-icon">⭐</span>
              <div class="acct-info-body">
                <span class="acct-info-label">{{ t('account.plan') }}</span>
                <span class="acct-info-value">{{ client.plan_name || t('account.plan_default') }}</span>
              </div>
            </div>
            <div class="acct-info-row">
              <span class="acct-info-icon">🕐</span>
              <div class="acct-info-body">
                <span class="acct-info-label">{{ t('account.register_time') }}</span>
                <span class="acct-info-value">{{ client.created_at ? formatDate(client.created_at) : '-' }}</span>
              </div>
            </div>
            <div class="acct-info-row">
              <span class="acct-info-icon">🔄</span>
              <div class="acct-info-body">
                <span class="acct-info-label">{{ t('account.expire') }}</span>
                <span class="acct-info-value">{{ client.expires_at ? formatDate(client.expires_at) : t('common.unlimited') }}</span>
              </div>
            </div>
            <!-- Balance Display -->
            <div class="acct-info-row" v-if="mod.data && mod.data.show_balance !== false">
              <span class="acct-info-icon">💰</span>
              <div class="acct-info-body">
                <span class="acct-info-label">{{ t('balance.available') }}</span>
                <div class="acct-info-value-row">
                  <span class="acct-info-value">{{ formatMoney(balance.balance) }}</span>
                  <el-button type="primary" size="small" @click="showChargeDialog = true" style="margin-left: 8px;">{{ t('balance.charge_btn') }}</el-button>
                </div>
              </div>
            </div>
          </div>
          <!-- Traffic mini bar inside account info -->
          <div class="acct-traffic-section">
            <div class="acct-traffic-label">
              <span>📊 {{ t('traffic.title') }}</span>
              <span class="acct-traffic-pct">{{ usagePercent }}% / {{ formatBytes(client.quota_bytes) }}</span>
            </div>
            <div class="traffic-bar" style="height:8px;border-radius:6px;background:var(--ring-bg);overflow:hidden;margin-top:6px;">
              <div class="traffic-bar-fill" :class="{ warn: usagePercent > 70, danger: usagePercent > 90 }" :style="{ width: usagePercent + '%' }"></div>
            </div>
            <div class="acct-traffic-hint" style="font-size:11px;color:var(--text-muted);margin-top:4px;">{{ t('traffic.remaining') }} {{ formatBytes(Math.max(0, client.quota_bytes - client.used_bytes)) }} {{ t('traffic.available') }}</div>
          </div>
        </div>
        <!-- Client Download -->
        <div v-else-if="mod.type === 'client_download'">
          <div class="card-header"><span class="card-icon">💻</span><span class="card-title">{{ t(mod.title) }}</span></div>
          <div class="client-platform-tabs">
            <div
              v-for="p in (mod.data && mod.data.platforms) || []"
              :key="p.id"
              class="platform-tab"
              :class="{ active: selectedPlatform === p.id }"
              @click="selectPlatform(p.id)"
            >
              <el-icon v-if="p.icon"><component :is="p.icon" /></el-icon>
              <span>{{ p.name }}</span>
            </div>
          </div>
          <!-- Hint when no platform selected -->
          <div v-if="!selectedPlatform" class="client-download-hint">
            <el-icon><ArrowUp /></el-icon>
            <span>{{ t((mod.data && mod.data.hint_text) || 'tutorial.select_platform') }}</span>
            <el-icon><ArrowUp /></el-icon>
          </div>
          <!-- Content when platform selected -->
          <div v-else class="client-download-content">
            <div class="client-download-platform-header">
              <span class="client-download-platform-name">{{ getSelectedPlatformName() }}</span>
              <el-button text size="small" @click="navigateTo('tutorial')">{{ t('tutorial.view_all') }} →</el-button>
            </div>
            <!-- Download link -->
            <div v-if="getSelectedPlatformUrl(mod)" class="client-download-link-row">
              <a :href="getSelectedPlatformUrl(mod)" target="_blank" rel="noopener" class="client-download-btn">
                <el-icon><Download /></el-icon>
                <span>{{ t('common.download') }} {{ getSelectedPlatformName() }} {{ t('client.title') }}</span>
              </a>
            </div>
            <!-- Associated tutorial articles -->
            <div v-if="getPlatformArticles(selectedPlatform).length > 0" class="client-download-articles">
              <div class="client-download-section-title">📖 {{ t('tutorial.related') }}</div>
              <div class="article-list compact">
                <div
                  class="article-item"
                  v-for="article in getPlatformArticles(selectedPlatform)"
                  :key="article.id"
                  @click="openArticle(article.slug)"
                >
                  <span class="article-cat-badge">{{ articleCatText(article.category) }}</span>
                  <div class="article-info">
                    <span class="article-title">{{ article.title }}</span>
                    <span class="logpkg-time">{{ formatDate(article.published_at || article.updated_at) }}</span>
                  </div>
                  <el-icon class="article-arrow"><ArrowRight /></el-icon>
                </div>
              </div>
            </div>
            <!-- Empty state -->
            <div v-if="!getSelectedPlatformUrl(mod) && getPlatformArticles(selectedPlatform).length === 0" class="empty-state" style="padding: 24px;">
              <span class="empty-icon">📦</span>
              <p class="empty-text">{{ t('tutorial.no_content') }}</p>
              <p class="empty-hint">{{ t('tutorial.no_content_hint') }}</p>
            </div>
          </div>
        </div>
        <!-- Subscription Detail -->
        <div v-else-if="mod.type === 'subscription_detail'">
          <div class="card-header"><span class="card-icon">🔗</span><span class="card-title">{{ t(mod.title) }}</span></div>
          <div v-if="client.sub_token && client.sub_status !== 'revoked'">
                <div class="sub-link-box">
                  <code class="sub-link-text">{{ subUrl }}</code>
                  <div class="sub-link-actions">
                    <el-button size="small" text @click="copyText(subUrl)">{{ (mod.data && mod.data.copy_text) || t('common.copy') }}</el-button>
                    <el-button size="small" text @click="showQrDialog">{{ (mod.data && mod.data.qr_text) || t('sub.qr_code') }}</el-button>
                  </div>
                </div>
            <div class="sub-actions-row" v-if="mod.data && mod.data.actions">
              <el-button 
                v-for="action in mod.data.actions" 
                :key="action.target"
                :type="action.type || 'default'"
                size="small"
                @click="handleSubAction(action)"
              >
                {{ t(action.text) }}
              </el-button>
            </div>
          </div>
          <div v-else-if="client.sub_status === 'revoked'" class="empty-state">
            <span class="empty-icon">🚫</span><p class="empty-text danger">{{ t('sub.revoked') }}</p><p class="empty-hint">{{ t('sub.revoked_hint') }}</p>
          </div>
<div v-else-if="client.status === 'active' && !client.sub_token" class="empty-state">
<span class="empty-icon">⏸️</span><p class="empty-text danger">{{ t('account.suspended') }}</p><p class="empty-hint">{{ t('account.suspended_hint') }}</p>
</div>
          <div v-else-if="client.status === 'suspended'" class="empty-state">
            <span class="empty-icon">⏸️</span><p class="empty-text danger">{{ t('account.suspended') }}</p><p class="empty-hint">{{ t('account.renew_hint') }}</p>
          </div>
        </div>
        <!-- Subscription (simple button) -->
        <div v-else-if="mod.type === 'subscription'" class="text-center">
          <div v-if="mod.data && mod.data.title" class="card-header"><span class="card-icon">📋</span><span class="card-title">{{ mod.data.title }}</span></div>
          <p v-if="mod.data && mod.data.description" class="card-desc">{{ t(mod.data.description) }}</p>
          <button v-if="mod.data && mod.data.button_text && mod.data.button_target !== 'overview'" class="hero-btn" @click="var target = mod.data.button_target || 'store'; activeTab = target; saveActiveTab(target); fetchPageData(target)">{{ t(mod.data.button_text) }}</button>
        </div>
        <!-- Streaming Status -->
        <div v-else-if="mod.type === 'streaming_status'">
          <div class="streaming-grid">
            <!-- Apple Shared ID -->
            <div class="streaming-card">
              <div class="streaming-name">{{ t('shared.apple_id') }}</div>
              <div class="streaming-icon"><span style="font-size: 32px;">🍎</span></div>
              <div class="streaming-action" v-if="appleAccount" @click="applySharedAccount(appleAccount.id, mod.data)">{{ t('streaming.view_account') }}</div>
              <div class="streaming-action" v-else style="color: var(--text-muted); cursor: default;">{{ t('streaming.none') }}</div>
            </div>
            <!-- Netflix Shared Account -->
            <div class="streaming-card">
              <div class="streaming-name">{{ t('shared.netflix') }}</div>
              <div class="streaming-icon"><span style="font-size: 32px;">🎬</span></div>
              <div class="streaming-action" v-if="netflixAccount" @click="applySharedAccount(netflixAccount.id, mod.data)">{{ t('streaming.view_account') }}</div>
              <div class="streaming-action" v-else style="color: var(--text-muted); cursor: default;">{{ t('streaming.none') }}</div>
            </div>
          </div>
        </div>
        <!-- Security Settings -->
        <div v-else-if="mod.type === 'security_settings'">
          <div class="card-header"><span class="card-icon">🔒</span><span class="card-title">{{ t(mod.title) }}</span></div>
          <p v-if="mod.data && mod.data.description" class="card-desc">{{ t(mod.data.description) }}</p>
          <el-form v-if="mod.data && mod.data.show_ip_whitelist !== false" label-position="top" size="small">
            <el-form-item :label="t('security.ip_whitelist_full')">
              <el-input v-model="settings.ip_whitelist" :placeholder="t('security.ip_example')" />
            </el-form-item>
            <el-form-item :label="t('security.device_limit_hint')">
              <el-input-number v-model="settings.device_limit" :min="0" :max="100" />
            </el-form-item>
            <el-button type="primary" size="small" @click="saveSettings">{{ t('common.save') }}</el-button>
          </el-form>
          <div v-if="mod.data && mod.data.show_password !== false">
            <div class="card-header" style="margin-top: 20px"><span class="card-icon">🔑</span><span class="card-title">{{ t('security.password') }}</span></div>
            <el-form label-position="top" size="small">
<el-form-item :label="t('security.old_password')"><el-input v-model="pwdForm.old_password" type="password" show-password /></el-form-item>
<el-form-item :label="t('security.new_password')"><el-input v-model="pwdForm.new_password" type="password" show-password /></el-form-item>
<el-form-item :label="t('security.confirm_password')"><el-input v-model="pwdForm.confirm_password" type="password" show-password /></el-form-item>
              <el-button type="primary" size="small" @click="changePassword">{{ t('security.password') }}</el-button>
            </el-form>
          </div>
        </div>
        <!-- Log Packages -->
        <div v-else-if="mod.type === 'log_packages'">
          <div class="card-header"><span class="card-icon">📦</span><span class="card-title">{{ t(mod.title) }}</span></div>
          <p v-if="mod.data && mod.data.description" class="card-desc">{{ t(mod.data.description) }}</p>
          <p v-else class="card-desc">{{ t('log.description') }}</p>
          <div class="log-actions">
            <el-button type="primary" size="small" @click="createLogPackage(1)" :loading="logPacking">{{ t((mod.data && mod.data.btn_1h) || 'log.btn_1h') }}</el-button>
            <el-button size="small" @click="createLogPackage(6)" :loading="logPacking">{{ t((mod.data && mod.data.btn_6h) || 'log.btn_6h') }}</el-button>
            <el-button size="small" @click="createLogPackage(24)">{{ t((mod.data && mod.data.btn_24h) || 'log.btn_24h') }}</el-button>
          </div>
          <div v-if="logPackages.length > 0" style="margin-top: 16px">
            <div class="card-header"><span class="card-icon">📁</span><span class="card-title">{{ t('log.package_list') }}</span><span class="card-action" @click="fetchLogPackages">{{ t('common.refresh') }}</span></div>
            <div class="logpkg-list">
              <div class="logpkg-item" v-for="pkg in logPackages" :key="pkg.id">
                <div class="logpkg-info">
                  <span class="logpkg-id">#{{ pkg.id }}</span>
                  <span :class="'badge badge-' + pkg.status">{{ pkgStatusText(pkg.status) }}</span>
                  <span class="logpkg-time">{{ formatDate(pkg.created_at) }}</span>
                </div>
                <div class="logpkg-actions">
                  <span v-if="pkg.file_size" class="logpkg-size">{{ formatBytes(pkg.file_size) }}</span>
                  <el-button v-if="pkg.status === 'done'" size="small" type="primary" text @click="downloadLogPackage(pkg.id)">{{ t('common.download') }}</el-button>
                  <span v-if="pkg.status === 'processing'" class="logpkg-hint">{{ t('common.processing') }}...</span>
                  <span v-if="pkg.error" class="logpkg-error">{{ pkg.error }}</span>
                </div>
              </div>
            </div>
          </div>
        </div>
        <!-- Markdown -->
        <div v-else-if="mod.type === 'markdown'" class="module-markdown" :style="{ textAlign: mod.style && mod.style.align || 'left' }">
          <div v-if="mod.title" class="card-header"><span class="card-title">{{ t(mod.title) }}</span></div>
          <div class="md-render" v-html="renderMd(t(mod.data && mod.data.markdown || ''))"></div>
        </div>
        <!-- Custom HTML -->
        <div v-else-if="mod.type === 'custom_html'" v-html="(mod.data && mod.data.html) || ''"></div>
        <!-- Plan List (qist table style) -->
        <div v-else-if="mod.type === 'plan_list'">
          <!-- Top Notice -->
          <div v-if="mod.data && mod.data.notice_text" class="plan-notice">
            <div class="plan-notice-content">{{ t(mod.data.notice_text) }}</div>
            <div v-if="mod.data.notice_subtext" class="plan-notice-sub">{{ t(mod.data.notice_subtext) }}</div>
          </div>
          
          <div v-if="plansLoading" class="empty-state"><span class="empty-icon">⏳</span><p class="empty-text">{{ t('common.loading') }}</p></div>
          <div v-else-if="plans.length === 0" class="empty-state"><span class="empty-icon">📭</span><p class="empty-text">{{ t('plan.empty') }}</p></div>
          <div v-else class="plan-table-wrap">
            <table class="plan-table">
              <thead>
                <tr>
                  <th class="plan-col-service">{{ t('plan.col_service') }}</th>
                  <th class="plan-col-desc">{{ t('plan.col_desc') }}</th>
                  <th class="plan-col-price">{{ t('plan.col_price') }}</th>
                  <th class="plan-col-action">{{ t('common.action') }}</th>
                </tr>
              </thead>
              <tbody>
                <template v-for="plan in plans" :key="plan.id">
                  <tr v-for="(v, vIdx) in (plan.variants || [])" :key="v.id" class="plan-row" :class="{ 'plan-current': client.plan_id > 0 && client.plan_id === plan.id }">
                    <!-- Service Column -->
                    <td v-if="vIdx === 0" :rowspan="plan.variants.length" class="plan-cell plan-cell-service">
                      <div class="plan-service-name">{{ plan.name }}</div>
                      <div class="plan-service-meta">
                        <div v-if="v.quota_bytes">{{ t('plan.included_traffic') }}: {{ formatBytes(v.quota_bytes) }}/{{ t('plan.per_month') }}</div>
                        <div v-if="v.speed_limit">{{ t('plan.max_bandwidth') }}: {{ v.speed_limit }}M</div>
                        <div v-if="v.duration_days">{{ t('plan.duration') }}: {{ v.duration_days }}{{ t('plan.days_unit') }}</div>
                      </div>
                    </td>
                    <!-- Description Column -->
                    <td class="plan-cell plan-cell-desc">
                      <div class="plan-desc-text">{{ plan.description || t('plan.default_desc') }}</div>
                    </td>
                    <!-- Price Column -->
                    <td class="plan-cell plan-cell-price">
                      <span class="plan-price-amount">{{ (v.currency || 'CNY') === 'CNY' ? '¥' : (v.currency === 'USD' ? '$' : v.currency) }}{{ formatMoney(v.price) }}</span>
                    </td>
                    <!-- Action Column -->
                    <td class="plan-cell plan-cell-action">
                      <el-button 
                        type="primary" 
                        size="small" 
                        @click="purchaseVariant(v.id, plan, mod.data)"
                        :disabled="orderBusy"
                      >
                        <span v-if="client.plan_id > 0 && client.plan_id === plan.id">{{ t((mod.data && mod.data.renew_text) || 'common.buy') }}</span>
                        <span v-else>{{ t((mod.data && mod.data.buy_text) || 'common.buy') }}</span>
                      </el-button>
                      <div v-if="client.plan_id > 0 && client.plan_id === plan.id" class="plan-current-badge">{{ t('plan.current_plan') }}</div>
                    </td>
                  </tr>
                </template>
              </tbody>
            </table>
          </div>
        </div>
        <!-- Order Create -->
        <div v-else-if="mod.type === 'order_create'">
          <div class="card-header"><span class="card-icon">🛒</span><span class="card-title">{{ t(mod.title) }}</span></div>
          <p v-if="mod.data && mod.data.description" class="card-desc">{{ t(mod.data.description) }}</p>
          <el-form label-position="top" size="small">
            <el-form-item :label="t('plan.title')">
              <el-select v-model="orderForm.variant_id" :placeholder="t('plan.select_placeholder')" style="width: 100%">
                <el-option v-for="p in plans" :key="p.id" :label="p.name" :value="p.id" />
              </el-select>
            </el-form-item>
            <el-form-item :label="t('order.type')">
              <el-radio-group v-model="orderForm.order_type">
                <el-radio value="new">{{ t('order.type_new') }}</el-radio>
                <el-radio value="renew">{{ t('common.renew') }}</el-radio>
                <el-radio value="upgrade">{{ t('common.upgrade') }}</el-radio>
              </el-radio-group>
            </el-form-item>
            <el-form-item :label="t('plan.promo_code')">
              <el-input v-model="orderForm.promotion_code" :placeholder="t('plan.promo_placeholder')" />
            </el-form-item>
            <el-form-item :label="t('plan.select_payment')">
              <el-radio-group v-model="orderForm.pay_method">
                <el-radio value="balance">{{ t('plan.balance_pay') }}</el-radio>
                <el-radio value="gift_card">{{ t('plan.card_pay') }}</el-radio>
                <el-radio v-if="chargeSites.length > 0" value="charge_site">{{ t('plan.charge_pay') }}</el-radio>
              </el-radio-group>
            </el-form-item>
            <el-form-item v-if="orderForm.pay_method === 'gift_card'" :label="t('plan.card_code')">
              <el-input v-model="orderForm.gift_card_code" :placeholder="t('plan.card_code_placeholder')" />
            </el-form-item>
            <el-form-item v-if="orderForm.pay_method === 'charge_site' && chargeSites.length > 0" :label="t('charge.site_label')">
              <el-select v-model="orderForm.charge_site_id" :placeholder="t('plan.select_channel_placeholder')" style="width: 100%">
                <el-option v-for="s in chargeSites" :key="s.id" :label="s.name" :value="s.id" />
              </el-select>
            </el-form-item>
            <el-button type="primary" size="small" @click="submitOrder" :loading="orderBusy">{{ t('order.create') }}</el-button>
          </el-form>
        </div>
        <!-- Charge Panel -->
        <div v-else-if="mod.type === 'charge_panel'">
          <div class="card-header"><span class="card-icon">💳</span><span class="card-title">{{ t(mod.title) }}</span></div>
          <p v-if="mod.data && mod.data.description" class="card-desc">{{ t(mod.data.description) }}</p>
          <div class="charge-section">
            <h4 class="charge-subtitle">{{ t('plan.charge_pay') }}</h4>
            <el-form label-position="top" size="small">
              <el-form-item :label="t('charge.site_label')">
                <el-select v-model="chargeForm.site_id" :placeholder="t('plan.select_channel_placeholder')" style="width: 100%">
                  <el-option v-for="s in chargeSites" :key="s.id" :label="s.name" :value="s.id" />
                </el-select>
              </el-form-item>
              <el-form-item :label="t('charge.amount_yuan')">
                <el-input-number v-model="chargeForm.amount_yuan" :min="1" :step="10" style="width: 100%" />
              </el-form-item>
              <el-button type="primary" size="small" @click="submitCharge" :loading="chargeBusy">{{ t('charge.submit_now') }}</el-button>
            </el-form>
          </div>
          <div class="charge-section">
            <h4 class="charge-subtitle">{{ t('charge.card_redeem') }}</h4>
            <div class="charge-card-row">
              <el-input v-model="cardCode" :placeholder="(mod.data && mod.data.card_placeholder) || t('charge.card_code_placeholder')" size="small" />
              <el-button type="success" size="small" @click="redeemCard" :loading="redeemBusy">{{ (mod.data && mod.data.card_btn_text) || t('charge.card_redeem_btn') }}</el-button>
            </div>
          </div>
          <div v-if="mod.data && mod.data.show_orders !== false && chargeOrders.length > 0" class="charge-section">
            <div class="card-header"><span class="card-icon">📋</span><span class="card-title">{{ t('charge.records') }}</span></div>
            <div class="logpkg-list">
              <div class="logpkg-item" v-for="co in chargeOrders.slice(0, (mod.data && mod.data.orders_limit) || 5)" :key="co.id">
                <div class="logpkg-info">
                  <span class="logpkg-id">{{ co.trade_no }}</span>
                  <span :class="'badge badge-' + co.status">{{ chargeStatusText(co.status) }}</span>
                  <span class="logpkg-time">{{ formatMoney(co.amount) }} {{ co.currency }}</span>
                </div>
                <div class="logpkg-actions">
                  <span class="logpkg-time">{{ formatDate(co.created_at) }}</span>
                  <a v-if="co.payment_url && co.status === 'pending'" :href="co.payment_url" target="_blank" class="card-action">{{ t('charge.go_pay') }}</a>
                </div>
              </div>
            </div>
          </div>
        </div>
        <!-- Balance Display -->
        <div v-else-if="mod.type === 'balance_display'">
          <div class="card-header"><span class="card-icon">💰</span><span class="card-title">{{ t(mod.title) }}</span></div>
          <div class="balance-hero">
            <span class="balance-amount">{{ formatMoney(balance.balance) }}</span>
            <span v-if="balance.frozen > 0" class="balance-frozen">{{ t('balance.frozen') }}: {{ formatMoney(balance.frozen) }}</span>
          </div>
          <div class="card-actions">
            <el-button type="primary" size="small" @click="activeTab = 'store'; scrollToModule('mod-charge')">{{ (mod.data && mod.data.charge_btn_text) || t('balance.charge_btn') }}</el-button>
          </div>
          <div v-if="mod.data && mod.data.show_transactions !== false && balanceTxns.length > 0" style="margin-top: 16px">
            <div class="card-header"><span class="card-icon">📄</span><span class="card-title">{{ t('balance.transactions') }}</span></div>
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
        <!-- Order List -->
        <div v-else-if="mod.type === 'order_list'">
          <div class="billing-notice">
            <span class="billing-notice-text">{{ t((mod.data && mod.data.notice_text) || 'order.title') }}</span>
            <span v-if="mod.data.notice_highlight" class="billing-notice-highlight">{{ t(mod.data.notice_highlight) }}</span>
          </div>
          <div class="billing-notice-sub">
            {{ t((mod.data && mod.data.notice_subtext) || 'balance.tx_limit') }}
          </div>
          <div v-if="myOrdersLoading" class="empty-state">
            <span class="empty-icon">⏳</span>
            <p class="empty-text">{{ t('common.loading') }}</p>
          </div>
          <div v-else-if="myOrders.length === 0" class="empty-state">
            <span class="empty-icon">📭</span>
            <p class="empty-text">{{ t('order.empty') }}</p>
          </div>
          <div v-else class="billing-table-wrap">
                <table class="billing-table">
                  <thead>
                    <tr>
                      <th>#</th>
                      <th>{{ t('order.id') }}</th>
                      <th>{{ t('plan.col_service') }}</th>
                      <th>{{ t('common.amount') }}</th>
                      <th>{{ t('order.date') }}</th>
                      <th>{{ t('account.expire') }}</th>
                      <th>{{ t('common.status') }}</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="(order, idx) in myOrders.slice(0, (mod.data && mod.data.limit) || 20)" :key="order.id">
                      <td class="billing-col-index">{{ idx + 1 }}</td>
                      <td class="billing-col-id">#{{ order.id }}</td>
                      <td class="billing-col-service">{{ getOrderServiceName(order) }}</td>
                      <td class="billing-col-amount">{{ formatMoney(order.paid_amount || order.original_amount) }}</td>
                      <td class="billing-col-date">{{ formatDate(order.created_at) }}</td>
                      <td class="billing-col-expire">{{ order.effective_to ? formatDate(order.effective_to) : '-' }}</td>
                      <td class="billing-col-status"><span :class="'badge badge-' + orderStatusBadge(order.status)">{{ orderStatusText(order.status) }}</span></td>
                    </tr>
                  </tbody>
                </table>
              </div>
              <!-- Charge Records -->
              <div v-if="chargeOrders.length > 0" style="margin-top: 20px">
                <div class="card-header"><span class="card-icon">📋</span><span class="card-title">{{ t('charge.records') }}</span></div>
                <div class="billing-table-wrap">
                  <table class="billing-table">
                    <thead>
                      <tr>
                        <th>#</th>
                        <th>{{ t('order.trade_no') }}</th>
                        <th>{{ t('order.source') }}</th>
                        <th>{{ t('common.amount') }}</th>
                        <th>{{ t('common.time') }}</th>
                        <th>{{ t('common.status') }}</th>
                      </tr>
                    </thead>
                    <tbody>
                      <tr v-for="(co, idx) in chargeOrders.slice(0, 10)" :key="co.id">
                        <td class="billing-col-index">{{ idx + 1 }}</td>
                        <td class="billing-col-id">{{ co.trade_no }}</td>
                        <td>{{ co.source === 'card' ? t('charge.card_title') : (co.source === 'site' ? t('plan.charge_pay') : co.source) }}</td>
                        <td class="billing-col-amount">{{ formatMoney(co.amount) }} {{ co.currency }}</td>
                        <td class="billing-col-date">{{ formatDate(co.created_at) }}</td>
                        <td class="billing-col-status"><span :class="'badge badge-' + co.status">{{ chargeStatusText(co.status) }}</span></td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              </div>
              <!-- Transaction Flow -->
              <div v-if="balanceTxns.length > 0" style="margin-top: 20px">
                <div class="card-header"><span class="card-icon">📄</span><span class="card-title">{{ t('balance.balance_change') }}</span></div>
                <div class="billing-table-wrap">
                  <table class="billing-table">
                    <thead>
                      <tr>
                        <th>#</th>
                        <th>{{ t('common.type') }}</th>
                        <th>{{ t('common.amount') }}</th>
                        <th>{{ t('balance.balance_after') }}</th>
                        <th>{{ t('common.remark') }}</th>
                        <th>{{ t('common.time') }}</th>
                      </tr>
                    </thead>
                    <tbody>
                      <tr v-for="(tx, idx) in balanceTxns.slice(0, 10)" :key="tx.id">
                        <td class="billing-col-index">{{ idx + 1 }}</td>
                        <td><span :class="'badge badge-' + (tx.amount >= 0 ? 'active' : 'expired')">{{ tx.type === 'charge' ? t('balance.charge_btn') : (tx.type === 'consume' ? t('balance.type_consume') : (tx.type === 'refund' ? t('order.refund') : tx.type)) }}</span></td>
                        <td class="billing-col-amount" :style="{ color: tx.amount >= 0 ? '#67c23a' : '#f56c6c' }">{{ tx.amount >= 0 ? '+' : '' }}{{ formatMoney(tx.amount) }}</td>
                        <td class="billing-col-amount">{{ formatMoney(tx.balance_after) }}</td>
                        <td>{{ tx.remark || '-' }}</td>
                        <td class="billing-col-date">{{ formatDate(tx.created_at) }}</td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              </div>
        </div>
        <!-- Node List (Mobile) -->
        <div v-else-if="mod.type === 'node_list'" class="node-list-module">
          <div v-if="mod.data && mod.data.info_text" class="node-info-card">
            <div class="node-info-title">{{ t(mod.data.info_title || 'node.info_title') }}</div>
            <div class="node-info-content">{{ t(mod.data.info_text) }}</div>
          </div>
          <div v-if="plansLoading" class="empty-state">
            <span class="empty-icon">⏳</span>
            <p class="empty-text">{{ t('common.loading') }}</p>
          </div>
          <div v-else-if="allPlanNodes.length === 0" class="empty-state">
            <span class="empty-icon">📭</span>
            <p class="empty-text">{{ t('node.empty') }}</p>
          </div>
          <div v-else>
            <div v-for="planGroup in allPlanNodes" :key="planGroup.plan_id" class="plan-node-group">
              <div class="plan-node-header">
                <span class="plan-node-name">{{ planGroup.plan_name }}</span>
                <span v-if="planGroup.active" class="badge badge-active">{{ t('account.plan') }}</span>
                <span v-else class="badge badge-none">{{ t('node.not_purchased') }}</span>
              </div>
              <div class="node-grid" :style="{ gridTemplateColumns: 'repeat(' + ((mod.data && mod.data.columns) || 2) + ', 1fr)' }">
                <div v-for="node in planGroup.nodes" :key="node.id" class="node-card" :class="{ disabled: !planGroup.active }">
                  <div class="node-name">{{ node.name }}</div>
                  <div class="node-tags">
                    <span v-if="node.protocol" class="node-tag tag-protocol">{{ node.protocol }}</span>
                    <span v-if="node.transport" class="node-tag tag-transport">{{ node.transport }}</span>
                  </div>
                  <div v-if="planGroup.active && node.link" class="node-actions">
                    <el-button size="small" text @click="showNodeDetail(node)">{{ t('common.view_detail') }}</el-button>
                    <el-button size="small" text @click="showNodeQR(node)">{{ t('sub.qr_code') }}</el-button>
                  </div>
                  <div v-else class="node-locked">
                    <span class="lock-hint">🔒 {{ t('node.need_plan') }}</span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
        <!-- Referral -->
        <div v-else-if="mod.type === 'referral'">
          <!-- Referral Link -->
          <div class="referral-header">
            <div class="referral-title">🎁 {{ t('referral.my_link') }}</div>
            <div class="referral-meta">
              <span class="referral-rate">{{ t('referral.commission_rate') }}: {{ (referralStats && referralStats.commission_rate) ? referralStats.commission_rate + '%' : ((mod.data && mod.data.default_commission_rate) || '15') + '%' }}</span>
              <span class="referral-divider">|</span>
              <span class="referral-mode">{{ t('referral.commission_mode') }}: {{ (referralStats && referralStats.commission_mode === 'recurring') ? t('referral.mode_recurring') : ((mod.data && mod.data.default_commission_mode === 'recu rring') ? t('referral.mode_recurring') : t('referral.mode_normal')) }}</span>
            </div>
          </div>
          <div class="referral-link-wrap" v-if="referralCode">
            <div class="referral-link">{{ referralInviteUrl }}</div>
            <el-button type="primary" size="small" @click="copyText(referralInviteUrl)" class="referral-copy-btn">
              <el-icon><DocumentCopy /></el-icon> {{ t('common.copy') }}
            </el-button>
          </div>
          
<!-- Invite Code -->
<div class="referral-code-section" v-if="referralCode">
<div class="referral-code-row">
<span class="referral-code-label">{{ t('referral.invite_code') }}</span>
<span class="referral-code-value">{{ referralCode.code }}</span>
<el-tag size="small" type="success" effect="dark" class="referral-status">{{ t('referral.code_available') }}</el-tag>
</div>
</div>

          <!-- Invite Records -->
          <div class="referral-section" v-if="referralRecords.length > 0">
            <div class="section-title">{{ t('referral.invite_records') }}</div>
            <div class="referral-table-wrap">
              <table class="referral-table">
                <thead>
                  <tr>
                    <th>#</th>
                    <th>{{ t('account.email') }}</th>
                    <th>{{ t('account.register_time') }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="(r, idx) in referralRecords.slice(0, (mod.data && mod.data.records_limit) || 10)" :key="r.id">
                    <td>{{ idx + 1 }}</td>
                    <td>{{ r.referee_email }}</td>
                    <td>{{ formatDateTime(r.registered_at) }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>

          <!-- Commission Records -->
          <div class="referral-section">
            <div class="section-header">
              <span class="section-title">{{ t('referral.records') }}</span>
              <span class="section-subtitle">{{ t('referral.withdraw_hint') }}</span>
              <el-button type="danger" size="small" class="withdraw-btn" v-if="referralStats && referralStats.available_commission > 0">{{ t('referral.withdraw') }}</el-button>
            </div>
            <div class="referral-table-wrap">
              <table class="referral-table">
                <thead>
                  <tr>
                    <th>#</th>
                    <th>{{ t('referral.commission_time') }}</th>
                    <th>{{ t('referral.consumer') }}</th>
                    <th>{{ t('referral.consume_amount') }}</th>
                    <th>{{ t('referral.commission_amount') }}</th>
                    <th>{{ t('common.status') }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="(r, idx) in commissionRecords" :key="r.id">
                    <td>{{ idx + 1 }}</td>
                    <td>{{ formatDateTime(r.created_at) }}</td>
                    <td>{{ r.consumer_email }}</td>
                    <td>¥{{ r.amount }}</td>
                    <td>¥{{ r.commission }}</td>
                    <td>
                      <el-tag size="small" :type="r.status === 'available' ? 'success' : r.status === 'pending' ? 'warning' : 'info'" effect="plain">
                        {{ r.status === 'available' ? t('referral.status_available') : r.status === 'pending' ? t('referral.status_pending') : t('referral.status_withdrawn') }}
                      </el-tag>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
            <div class="referral-summary" v-if="referralStats">
              {{ t('referral.total_commission') }} {{ formatMoney(referralStats.total_commission) }}（{{ referralStats.total_paid }}{{ t('referral.times' ) }}），{{ t('referral.withdraw_threshold' ) }}
            </div>
          </div>

          <!-- Withdrawal Records -->
          <div class="referral-section" v-if="withdrawRecords && withdrawRecords.length > 0">
            <div class="section-title">{{ t('referral.withdraw_records') }}</div>
            <div class="referral-table-wrap">
              <table class="referral-table">
                <thead>
                  <tr>
                    <th>#</th>
                    <th>{{ t('referral.withdraw_date') }}</th>
                    <th>{{ t('referral.withdraw_amount') }}</th>
                    <th>{{ t('common.status') }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="(w, idx) in withdrawRecords" :key="w.id">
                    <td>{{ idx + 1 }}</td>
                    <td>{{ formatDateTime(w.created_at) }}</td>
                    <td>{{ w.amount }}</td>
                    <td>
                      <el-tag size="small" :type="w.status === 'completed' ? 'success' : w.status === 'pending' ? 'warning' : 'info'" effect="plain">
                        {{ w.status === 'completed' ? t('referral.status_paid') : w.status === 'pending' ? t('common.processing') : t('order.status_cancelled') }}
                      </el-tag>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>
        <!-- Ticket List -->
        <div v-else-if="mod.type === 'ticket_list'">
          <div class="card-header"><span class="card-icon">🎫</span><span class="card-title">{{ t(mod.title) }}</span><span class="card-action" @click="showTicketDialog = true">{{ t((mod.data && mod.data.new_btn_text) || 'ticket.new') }}</span></div>
          <div v-if="tickets.length === 0" class="empty-state"><span class="empty-icon">📭</span><p class="empty-text">{{ t('ticket.empty') }}</p></div>
<div v-else class="logpkg-list">
<div class="logpkg-item" v-for="tk in tickets" :key="tk.id" @click="openTicket(tk.id)" style="cursor: pointer">
<div class="logpkg-info">
<span class="logpkg-id">#{{ tk.id }}</span>
<span class="logpkg-time">{{ tk.subject }}</span>
<span :class="'badge badge-' + ticketStatusBadge(tk.status)">{{ ticketStatusText(tk.status) }}</span>
<span v-if="tk.status === 'replied'" class="ticket-new-badge">{{ t('ticket.new_reply') }}</span>
</div>
<div class="logpkg-actions">
<span class="logpkg-time">{{ formatDate(tk.updated_at) }}</span>
</div>
</div>
</div>
        </div>
        <!-- Tutorial Cards (Mobile) -->
        <div v-else-if="mod.type === 'tutorial_cards'">
          <div class="card-header"><span class="card-icon">📥</span><span class="card-title">{{ t(mod.title) }}</span></div>
          <!-- Platform Tabs -->
          <div class="tutorial-platform-tabs">
            <div
              v-for="card in (mod.data && mod.data.cards) || []"
              :key="card.id"
              class="tutorial-tab"
              :class="{ active: selectedPlatform === card.id }"
              @click="selectPlatform(card.id)"
            >
              {{ card.name }}
            </div>
          </div>
          <!-- Platform Grid -->
          <div class="tutorial-grid">
            <div
              v-for="card in (mod.data && mod.data.cards) || []"
              :key="card.id"
              class="tutorial-card"
              :class="{ active: selectedPlatform === card.id }"
              :style="{ background: card.bg || card.color }"
              @click="selectPlatform(card.id)"
            >
              <div class="tutorial-card-icon">{{ getTutorialIcon(card.icon) }}</div>
              <div class="tutorial-card-name">{{ card.name }}</div>
              <div class="tutorial-card-desc">{{ t(card.desc) }}</div>
            </div>
          </div>
          <!-- Related Articles -->
          <div v-if="selectedPlatform" class="tutorial-articles">
            <div class="tutorial-articles-header">
              <span class="tutorial-platform-name">{{ getSelectedPlatformName() }}</span>
              <span class="tutorial-articles-count">{{ getPlatformArticles(selectedPlatform).length }} {{ selectedPlatform === 'faq' ? t('article.faq_count') : t('tutorial.platform_articles') }}</span>
            </div>
            <div v-if="getPlatformArticles(selectedPlatform).length === 0" class="empty-state" style="padding: 24px;">
              <span class="empty-icon">📝</span>
              <p class="empty-text">{{ selectedPlatform === 'faq' ? t('tutorial.no_faq') : t('tutorial.no_articles') }}</p>
            </div>
            <div v-else class="article-list tutorial-article-list">
              <div
                class="article-item"
                v-for="article in getPlatformArticles(selectedPlatform)"
                :key="article.id"
                @click="openArticle(article.slug)"
              >
                <span class="article-cat-badge">{{ articleCatText(article.category) }}</span>
                <div class="article-info">
                  <span class="article-title">{{ article.title }}</span>
                  <span class="logpkg-time">{{ formatDate(article.published_at || article.updated_at) }}</span>
                </div>
                <span class="card-action">{{ t('article.read') }}</span>
              </div>
            </div>
          </div>
          <div v-else class="tutorial-select-hint">
            <span class="empty-icon">👆</span>
            <p class="empty-text">{{ t('tutorial.select_platform') }}</p>
          </div>
        </div>
        <!-- Article List -->
        <div v-else-if="mod.type === 'article_list'">
          <div class="card-header"><span class="card-icon">📚</span><span class="card-title">{{ t(mod.title) }}</span></div>
          <div v-if="mod.data && mod.data.show_category_filter" class="article-filter">
            <el-radio-group v-model="articleCategory" size="small" @change="fetchArticles">
              <el-radio-button value="">{{ t('common.all') }}</el-radio-button>
              <el-radio-button value="tutorial">{{ t('article.tutorial') }}</el-radio-button>
              <el-radio-button value="faq">FAQ</el-radio-button>
              <el-radio-button value="notice">{{ t('article.notice') }}</el-radio-button>
            </el-radio-group>
          </div>
          <div v-if="articles.length === 0" class="empty-state"><span class="empty-icon">📭</span><p class="empty-text">{{ t('article.empty') }}</p></div>
          <div v-else class="article-list">
            <div class="article-item" v-for="a in articles" :key="a.id" @click="openArticle(a.slug)">
              <span class="article-cat-badge">{{ articleCatText(a.category) }}</span>
              <div class="article-info">
                <span class="article-title">{{ a.title }}</span>
                <span class="logpkg-time">{{ formatDate(a.published_at || a.updated_at) }}</span>
              </div>
              <span class="card-action">{{ t('article.read') }}</span>
            </div>
          </div>
        </div>
        <!-- Shared Accounts -->
        <div v-else-if="mod.type === 'shared_accounts'">
          <div class="card-header"><span class="card-icon">🔑</span><span class="card-title">{{ t(mod.title) }}</span></div>
          <p v-if="mod.data && mod.data.description" class="card-desc">{{ t(mod.data.description) }}</p>
          <!-- No Plan Access -->
          <div v-if="!client.plan_id" class="empty-state">
            <span class="empty-icon">🔒</span>
            <p class="empty-text">{{ t('shared.need_plan') }}</p>
            <el-button type="primary" size="small" @click="activeTab = 'services'" style="margin-top: 12px">{{ t('plan.go_subscribe') }}</el-button>
          </div>
          <!-- Has Plan, No Accounts -->
          <div v-else-if="sharedAccounts.length === 0" class="empty-state"><span class="empty-icon">📭</span><p class="empty-text">{{ t('shared.empty') }}</p></div>
          <!-- Has Plan And Accounts -->
          <div v-else class="article-list">
            <div class="article-item" v-for="sa in sharedAccounts" :key="sa.id">
              <img v-if="sa.icon_url" :src="sa.icon_url" class="sa-icon" />
              <span v-else class="article-cat-badge">{{ sharedAcctTypeText(sa.type) }}</span>
              <div class="article-info">
                <span class="article-title">{{ sa.name }}</span>
                <span class="logpkg-time">{{ sharedAcctTypeText(sa.type) }}</span>
              </div>
              <el-button size="small" type="primary" text @click="applySharedAccount(sa.id, mod.data)">{{ t((mod.data && mod.data.apply_btn_text) || 'shared.view_credential') }}</el-button>
              <el-button size="small" text @click="repairSharedAccount(sa.id, mod.data)">{{ t((mod.data && mod.data.repair_btn_text) || 'shared.repair') }}</el-button>
            </div>
          </div>
        </div>
        <!-- Freeform -->
        <div v-else-if="mod.type === 'freeform'" class="module-freeform">
          <component :is="freeformComponents[mod.id]" v-if="freeformComponents[mod.id]"></component>
          <div v-else style="color:#f56c6c;padding:12px;font-size:13px">⚠️ {{ t('error.load_failed') }}</div>
        </div>
      </div>
    </template>
    <!-- Empty state for non-overview pages -->
    <div v-if="activeTab !== 'overview' && currentModules.length === 0" class="card glass">
      <div class="empty-state">
        <span class="empty-icon">📋</span>
        <p class="empty-text">{{ t('common.empty') }}</p>
      </div>
    </div>
      </main>
      <nav class="bottom-nav">
        <div v-for="nav in portalNavItems" :key="nav.id" :class="'nav-item' + (activeTab === nav.target ? ' active' : '')" @click="onNavClick(nav)">
          <span class="nav-icon">{{ nav.icon || '🔗' }}</span>
          <span class="nav-label">{{ t(nav.label) }}</span>
        </div>
      </nav>
    </div>

    <!-- Announcement Popup Dialog -->
    <el-dialog
      v-if="activeAnnouncement"
      v-model="announcementPopupVisible"
      :title="activeAnnouncement.title || t('announcement.popup_title')"
      width="500px"
      :close-on-click-modal="true"
      class="notice-popup-dialog"
      align-center
      append-to-body
    >
      <div class="notice-popup-body notice-info">
        <div class="md-render" v-html="renderMd(activeAnnouncement.content || '')"></div>
      </div>
      <template #footer>
        <div class="notice-popup-footer">
          <el-button type="primary" @click="announcementPopupVisible = false">{{ t('announcement.got_it') }}</el-button>
        </div>
      </template>
    </el-dialog>

<!-- Node QR Code Dialog -->
<el-dialog v-model="showNodeQRDialog" :title="t('node.qr_code')" width="300px" align-center>
<div style="text-align: center; padding: 20px;">
<div ref="nodeQRContainer" style="width: 200px; height: 200px; margin: 0 auto;"></div>
</div>
</el-dialog>

<!-- Subscription QR Code Dialog -->
<el-dialog v-model="showSubQrDialog" :title="t('sub.qr_code')" width="300px" align-center>
<div style="text-align: center; padding: 20px;">
<img v-if="qrCodeUrl" :src="qrCodeUrl" style="width: 200px; height: 200px; border-radius: 8px;" />
<span v-else style="color: var(--text-muted)">Loading...</span>
<div style="margin-top: 12px; font-size: 12px; color: var(--text-muted);">{{ t('sub.scan_import') }}</div>
</div>
</el-dialog>

    <!-- Node Detail Dialog -->
    <el-dialog v-model="showNodeDetailDialog" :title="currentNodeDetail.name || t('node.detail')" width="450px" align-center append-to-body>
      <div v-if="currentNodeDetail.link" class="node-detail-content">
        <div class="node-detail-tags">
          <span v-if="currentNodeDetail.protocol" class="node-tag tag-protocol">{{ currentNodeDetail.protocol }}</span>
          <span v-if="currentNodeDetail.transport" class="node-tag tag-transport">{{ currentNodeDetail.transport }}</span>
        </div>
        <div class="node-detail-label">{{ t('node.share_link') }}</div>
        <div class="node-detail-link-box">
          <code class="node-detail-link-text">{{ currentNodeDetail.link }}</code>
          <el-button type="primary" size="small" @click="copyText(currentNodeDetail.link)">{{ t('node.copy_link') }}</el-button>
        </div>
      </div>
    </el-dialog>

    <!-- Ticket Detail Dialog -->
    <el-dialog v-model="showTicketDetail" :title="t('ticket.detail')" width="500px" align-center append-to-body class="ticket-detail-dialog">
      <div v-if="ticketDetail" class="ticket-detail">
        <div class="ticket-detail-header">
          <span class="logpkg-id">#{{ ticketDetail.ticket.id }}</span>
          <span :class="'badge badge-' + ticketStatusBadge(ticketDetail.ticket.status)">{{ ticketStatusText(ticketDetail.ticket.status) }}</span>
          <span class="logpkg-time">{{ formatDate(ticketDetail.ticket.created_at) }}</span>
        </div>
        <h3 class="ticket-subject">{{ ticketDetail.ticket.subject }}</h3>
        <div class="ticket-msgs">
          <div v-for="msg in ticketDetail.messages" :key="msg.id" class="ticket-msg" :class="'msg-' + msg.sender_type">
            <span class="ticket-msg-sender">{{ msg.sender_type === 'client' ? t('ticket.sender_me') : t('ticket.sender_staff') }}</span>
            <div class="ticket-msg-content" v-html="renderMd(msg.content)"></div>
            <span class="logpkg-time">{{ formatDate(msg.created_at) }}</span>
          </div>
        </div>
        <div v-if="ticketDetail.ticket && ticketDetail.ticket.status !== 'closed'" class="ticket-reply">
          <el-input v-model="ticketReplyContent" type="textarea" :rows="3" :placeholder="t('ticket.reply_placeholder')"></el-input>
        </div>
        <div v-if="ticketDetail.ticket && ticketDetail.ticket.status !== 'closed'" style="margin-top: 12px; text-align: right;">
          <el-button size="small" @click="closeTicket" :loading="ticketBusy">{{ t('ticket.close') }}</el-button>
          <el-button type="primary" size="small" @click="replyTicket" :loading="ticketBusy">{{ t('ticket.send_reply') }}</el-button>
        </div>
        <div v-if="ticketDetail.ticket && ticketDetail.ticket.status === 'closed'" style="margin-top: 12px; text-align: center; color: var(--text-muted);">
          {{ t('ticket.closed_notice') }}
        </div>
      </div>
    </el-dialog>

<!-- New Ticket Dialog -->
<el-dialog v-model="showTicketDialog" :title="t('ticket.new')" width="500px" align-center append-to-body class="ticket-dialog">
  <el-form label-position="top" size="default">
    <el-form-item :label="t('ticket.category')">
      <el-radio-group v-model="newTicketForm.category" class="ticket-type-group">
        <el-radio-button value="technical">{{ t('ticket.technical') }}</el-radio-button>
        <el-radio-button value="billing">{{ t('ticket.billing') }}</el-radio-button>
        <el-radio-button value="account">{{ t('ticket.account') }}</el-radio-button>
        <el-radio-button value="other">{{ t('common.other') }}</el-radio-button>
      </el-radio-group>
    </el-form-item>
    <el-form-item :label="t('ticket.subject')">
      <el-input v-model="newTicketForm.subject" :placeholder="t('ticket.subject_placeholder')" />
    </el-form-item>
    <el-form-item :label="t('ticket.content_label')">
      <el-input v-model="newTicketForm.content" type="textarea" :rows="5" :placeholder="t('ticket.content_placeholder')" />
    </el-form-item>
    <div class="ticket-submit-wrap">
      <el-button type="primary" size="large" @click="createTicket" :loading="ticketBusy" class="ticket-submit-btn">{{ t('ticket.submit') }}</el-button>
    </div>
  </el-form>
</el-dialog>

    <!-- Article Detail Dialog -->
    <el-dialog v-model="showArticleDetail" :title="articleDetail ? articleDetail.title : t('article.title')" width="600px" align-center append-to-body class="article-dialog">
      <div v-if="articleDetail" class="md-render" v-html="renderMd(articleDetail.content)"></div>
    </el-dialog>

    <!-- Shared Account Credentials Dialog -->
    <el-dialog v-model="showCredentialsDialog" :title="t('shared.credential_title')" width="460px" align-center append-to-body>
      <div v-if="sharedAccountCreds" class="credential-detail">
        <div class="info-grid" style="margin-bottom: 12px">
          <div class="info-item"><span class="info-label">{{ t('common.name') }}</span><span class="info-value">{{ sharedAccountCreds.name }}</span></div>
          <div class="info-item"><span class="info-label">{{ t('common.type') }}</span><span class="info-value">{{ sharedAcctTypeText(sharedAccountCreds.type) }}</span></div>
        </div>
        <el-alert type="warning" :closable="false" show-icon style="margin-bottom: 12px">
          <span style="font-size: 12px">{{ t('shared.credential_hint') }}</span>
        </el-alert>
        <div class="referral-link-box">
          <code class="sub-link-text" style="white-space: pre-wrap; word-break: break-all">{{ sharedAccountCreds.credentials_enc }}</code>
          <el-button size="small" type="primary" text @click="copyText(sharedAccountCreds.credentials_enc)">{{ t('common.copy') }}</el-button>
        </div>
      </div>
    </el-dialog>

    <!-- Charge Dialog -->
    <el-dialog v-model="showChargeDialog" :title="t('charge.dialog_title')" width="440px" align-center append-to-body class="payment-dialog">
      <div class="payment-dialog-content">
        <!-- Charge Method -->
        <div class="pay-section-title">{{ t('charge.select_method') }}</div>
        <div class="pay-method-list">
          <div 
            v-for="site in (chargeSites || [])" 
            :key="'cs-'+site.id"
            class="pay-method-card"
            :class="{ active: chargeForm.method !== 'card' && chargeForm.site_id === site.id }"
            @click="chargeForm.site_id = site.id; chargeForm.method = 'online'"
          >
            <div class="pay-method-icon pay-method-icon-online">
              <el-icon><Money /></el-icon>
            </div>
            <div class="pay-method-info">
              <div class="pay-method-name">{{ site.name || t('plan.charge_pay') }}</div>
              <div class="pay-method-desc">{{ t('charge.alipay_wechat') }}</div>
            </div>
            <div class="pay-method-check" v-if="chargeForm.method !== 'card' && chargeForm.site_id === site.id">
              <el-icon><Check /></el-icon>
            </div>
          </div>

          <div 
            class="pay-method-card"
            :class="{ active: chargeForm.method === 'card' }"
            @click="chargeForm.method = 'card'; chargeForm.site_id = null"
          >
            <div class="pay-method-icon pay-method-icon-card">
              <el-icon><Ticket /></el-icon>
            </div>
            <div class="pay-method-info">
              <div class="pay-method-name">{{ t('charge.card_title') }}</div>
              <div class="pay-method-desc">{{ t('charge.use_card_redeem') }}</div>
            </div>
            <div class="pay-method-check" v-if="chargeForm.method === 'card'">
              <el-icon><Check /></el-icon>
            </div>
          </div>
        </div>

        <!-- Online Payment Form -->
        <transition name="pay-fade">
          <div v-if="chargeForm.method !== 'card' && chargeSites && chargeSites.length > 0" class="pay-extra-form">
            <div class="pay-extra-label">{{ t('charge.amount_label') }}</div>
            <div class="charge-amount-grid">
              <div 
                v-for="amt in [1, 2, 5, 10, 20, 50, 100, 300, 500]" 
                :key="amt"
                class="charge-amount-btn"
                :class="{ active: chargeForm.amount_yuan === amt && !chargeForm.custom_amount }"
                @click="chargeForm.amount_yuan = amt; chargeForm.custom_amount = false"
              >
                <span class="charge-amount-num">{{ amt }}</span><span class="charge-amount-unit">{{ t('charge.yuan') }}</span>
              </div>
              <div 
                class="charge-amount-btn charge-amount-custom"
                :class="{ active: chargeForm.custom_amount }"
                @click="chargeForm.custom_amount = true"
              >
                {{ t('charge.custom') }}
              </div>
            </div>
            <el-input-number 
              v-if="chargeForm.custom_amount" 
              v-model="chargeForm.amount_yuan" 
              :min="1" 
              :step="1" 
              style="width: 100%; margin-top: 12px;" 
              :placeholder="t('charge.amount_placeholder')"
            />
          </div>
        </transition>

        <!-- Card Form -->
        <transition name="pay-fade">
          <div v-if="chargeForm.method === 'card'" class="pay-extra-form">
            <div class="pay-extra-label">{{ t('charge.card_password') }}</div>
            <el-input v-model="cardCode" :placeholder="t('charge.card_password_placeholder')" prefix-icon="Ticket" clearable />
            <div class="pay-hint">{{ t('charge.card_hint') }}</div>
          </div>
        </transition>
      </div>
      <template #footer>
        <div class="pay-dialog-footer">
          <el-button @click="showChargeDialog = false" size="large">{{ t('common.cancel') }}</el-button>
          <el-button 
            v-if="chargeForm.method === 'card'" 
            type="primary" 
            @click="redeemCard"
            :loading="redeemBusy"
            size="large"
            class="pay-confirm-btn"
          >{{ t('charge.card_redeem_btn') }}</el-button>
          <el-button 
            v-else 
            type="primary" 
            @click="submitCharge"
            :loading="chargeBusy"
            size="large"
            class="pay-confirm-btn"
          >{{ t('charge.go_pay') }}</el-button>
        </div>
      </template>
    </el-dialog>

    <!-- Payment Method Dialog -->
    <el-dialog v-model="showPaymentDialog" :title="t('order.confirm')" width="440px" align-center append-to-body :close-on-click-modal="false" class="payment-dialog">
      <div class="payment-dialog-content">
        <!-- Order Summary -->
        <div class="pay-summary">
          <div class="pay-summary-row">
            <span class="pay-summary-label">{{ t('plan.title') }}</span>
            <span class="pay-summary-value">{{ orderForm.plan_name || t('plan.not_selected') }}</span>
          </div>
          <div class="pay-summary-row" v-if="orderForm.variant_name">
            <span class="pay-summary-label">{{ t('order.spec') }}</span>
            <span class="pay-summary-value">{{ orderForm.variant_name }}</span>
          </div>
          <div class="pay-summary-row" v-if="orderForm.duration_days">
            <span class="pay-summary-label">{{ t('order.duration') }}</span>
            <span class="pay-summary-value">{{ orderForm.duration_days }} {{ t('plan.days_unit') }}</span>
          </div>
          <div class="pay-summary-row" v-if="orderForm.quota_bytes">
            <span class="pay-summary-label">{{ t('order.traffic') }}</span>
            <span class="pay-summary-value">{{ formatBytes(orderForm.quota_bytes) }}/{{ t('plan.per_month') }}</span>
          </div>
          <div class="pay-summary-row">
            <span class="pay-summary-label">{{ t('common.type') }}</span>
            <span class="pay-summary-value">
              <span :class="'pay-type-tag pay-type-' + orderForm.order_type">{{ orderForm.order_type === 'renew' ? t('common.renew') : orderForm.order_type === 'upgrade' ? t('common.upgrade') : t('order.type_new') }}</span>
            </span>
          </div>
          <div class="pay-summary-row pay-summary-price">
            <span class="pay-summary-label">{{ t('order.amount_due') }}</span>
            <span class="pay-summary-value pay-price-num">{{ orderForm.currency === 'USD' ? '$' : '¥' }}{{ formatMoney(orderForm.price) }}</span>
          </div>
          <div class="pay-summary-row" v-if="orderForm.pay_method === 'balance'">
            <span class="pay-summary-label">{{ t('balance.available') }}</span>
            <span class="pay-summary-value" :class="{ 'text-danger': balance && balance.balance < orderForm.price }">{{ formatMoney(balance.balance) }}</span>
          </div>
        </div>

        <!-- Payment Methods -->
        <div class="pay-section-title">{{ t('plan.select_payment') }}</div>
        <div class="pay-method-list">
          <div 
            class="pay-method-card"
            :class="{ active: orderForm.pay_method === 'balance' }"
            @click="orderForm.pay_method = 'balance'"
          >
            <div class="pay-method-icon pay-method-icon-balance">
              <el-icon><Wallet /></el-icon>
            </div>
            <div class="pay-method-info">
              <div class="pay-method-name">{{ t('plan.balance_pay') }}</div>
              <div class="pay-method-desc">{{ t('balance.available') }} {{ formatMoney(balance.balance) }}</div>
            </div>
            <div class="pay-method-check" v-if="orderForm.pay_method === 'balance'">
              <el-icon><Check /></el-icon>
            </div>
          </div>

          <div 
            class="pay-method-card"
            :class="{ active: orderForm.pay_method === 'gift_card' }"
            @click="orderForm.pay_method = 'gift_card'"
          >
            <div class="pay-method-icon pay-method-icon-card">
              <el-icon><Ticket /></el-icon>
            </div>
            <div class="pay-method-info">
              <div class="pay-method-name">{{ t('plan.card_pay') }}</div>
              <div class="pay-method-desc">{{ t('charge.use_card_redeem') }}</div>
            </div>
            <div class="pay-method-check" v-if="orderForm.pay_method === 'gift_card'">
              <el-icon><Check /></el-icon>
            </div>
          </div>

          <div 
            v-if="chargeSites.length > 0"
            class="pay-method-card"
            :class="{ active: orderForm.pay_method === 'charge_site' }"
            @click="orderForm.pay_method = 'charge_site'"
          >
            <div class="pay-method-icon pay-method-icon-online">
              <el-icon><Money /></el-icon>
            </div>
            <div class="pay-method-info">
              <div class="pay-method-name">{{ t('plan.charge_pay') }}</div>
              <div class="pay-method-desc">{{ t('charge.alipay_wechat') }}</div>
            </div>
            <div class="pay-method-check" v-if="orderForm.pay_method === 'charge_site'">
              <el-icon><Check /></el-icon>
            </div>
          </div>
        </div>

        <!-- Expanded Form Area -->
        <transition name="pay-fade">
          <div v-if="orderForm.pay_method === 'gift_card'" class="pay-extra-form">
            <div class="pay-extra-label">{{ t('plan.card_code') }}</div>
            <el-input v-model="orderForm.gift_card_code" :placeholder="t('plan.card_code_placeholder')" prefix-icon="Ticket" clearable />
          </div>
        </transition>

        <transition name="pay-fade">
          <div v-if="orderForm.pay_method === 'charge_site' && chargeSites.length > 0" class="pay-extra-form">
            <div class="pay-extra-label">{{ t('charge.channel') }}</div>
            <el-select v-model="orderForm.charge_site_id" :placeholder="t('plan.select_channel_placeholder')" style="width: 100%" size="large">
              <el-option v-for="s in chargeSites" :key="s.id" :label="s.name" :value="s.id" />
            </el-select>
          </div>
        </transition>

        <transition name="pay-fade">
          <div v-if="orderForm.pay_method === 'balance' && balance && balance.balance < orderForm.price" class="pay-balance-warn">
            <el-icon><WarningFilled /></el-icon>
            <span>{{ t('plan.balance_insufficient') }}</span>
          </div>
        </transition>

        <!-- Promo Code (Collapsible) -->
        <div class="pay-promo-toggle" @click="showPromoInput = !showPromoInput">
          <span>{{ showPromoInput ? t('charge.hide_promo') : (orderForm.promotion_code ? (t('plan.promo_code') + ': ' + orderForm.promotion_code) : t('charge.use_promo')) }}</span>
          <el-icon class="pay-promo-arrow" :class="{ rotated: showPromoInput }"><ArrowDown /></el-icon>
        </div>
        <transition name="pay-fade">
          <div v-if="showPromoInput" class="pay-extra-form" style="margin-top: 8px;">
            <el-input v-model="orderForm.promotion_code" placeholder="t('plan.promo_placeholder')" clearable />
          </div>
        </transition>
      </div>
      <template #footer>
        <div class="pay-dialog-footer">
          <el-button @click="showPaymentDialog = false" size="large">{{ t('common.cancel') }}</el-button>
          <el-button type="primary" @click="confirmPayment" :loading="orderBusy" size="large" class="pay-confirm-btn">
            <span v-if="orderForm.pay_method === 'balance'">{{ t('charge.confirm_pay') }}</span>
            <span v-else-if="orderForm.pay_method === 'gift_card'">{{ t('charge.card_pay') }}</span>
            <span v-else-if="orderForm.pay_method === 'charge_site'">{{ t('charge.go_pay') }}</span>
            <span v-else>{{ t('charge.confirm_pay') }}</span>
          </el-button>
        </div>
      </template>
    </el-dialog>

    <!-- More Subscriptions Dialog -->
    <el-dialog v-model="moreSubsVisible" :title="t('sub.more_subs')" width="480px" align-center append-to-body class="more-subs-dialog">
      <div class="more-subs-list">
        <div v-if="moreSubFormats.length === 0" class="more-subs-empty">{{ t('common.empty') }}</div>
        <!-- Format Cards -->
        <div v-for="item in moreSubFormats" :key="item.key" class="more-subs-card">
          <div class="more-subs-card-header">
            <span class="more-subs-card-icon">{{ item.icon }}</span>
            <span class="more-subs-card-title">{{ item.label }}</span>
            <span class="more-subs-card-desc">{{ item.desc }}</span>
          </div>
          <!-- Config List -->
          <div class="more-subs-config-list">
            <!-- Single format without configs -->
            <div v-if="!item.configs || item.configs.length === 0" class="more-subs-config-row">
              <span class="more-subs-config-label">{{ t('sub.default_config') }}</span>
              <div class="more-subs-config-actions">
                <el-button size="small" type="primary" plain @click="copyText(buildSubLink(item.key, '', ''))" :disabled="!client.sub_token">{{ t('sub.copy_link') }}</el-button>
                <el-button size="small" @click="showMoreSubQrDialog(item.key, '', '')" :disabled="!client.sub_token">{{ t('sub.qr_code') }}</el-button>
              </div>
            </div>
            <!-- Multiple configs -->
            <div v-for="cfg in item.configs" :key="cfg.name + '-' + cfg.type" class="more-subs-config-row">
              <div class="more-subs-config-info">
                <span class="more-subs-config-name">{{ cfg.name }}</span>
                <el-tag v-if="cfg.platform" size="small" type="info" effect="plain" class="more-subs-config-tag">{{ cfg.platform }}</el-tag>
              </div>
              <div class="more-subs-config-actions">
                <el-button size="small" type="primary" plain @click="copyText(buildSubLink(item.key, cfg.type === 'config' ? cfg.name : '', cfg.platform || ''))" :disabled="!client.sub_token">{{ t('sub.copy_link') }}</el-button>
                <el-button size="small" @click="showMoreSubQrDialog(item.key, cfg.type === 'config' ? cfg.name : '', cfg.platform || '')" :disabled="!client.sub_token">{{ t('sub.qr_code') }}</el-button>
              </div>
            </div>
          </div>
        </div>
      </div>
      <template #footer>
        <el-button @click="moreSubsVisible = false">{{ t('common.close') }}</el-button>
      </template>
    </el-dialog>

    <!-- More Subscriptions QR Code Dialog -->
    <el-dialog v-model="moreSubQrDialogVisible" :title="t('sub.qr_code')" width="300px" align-center append-to-body>
      <div style="text-align: center; padding: 20px;">
        <img v-if="moreSubQrDialogUrl" :src="moreSubQrDialogUrl" style="width: 200px; height: 200px; border-radius: 8px;" />
        <span v-else style="color: var(--text-muted)">Loading...</span>
        <div style="margin-top: 12px; font-size: 12px; color: var(--text-muted);">{{ t('sub.scan_import') }}</div>
      </div>
    </el-dialog>
  </div>

<script src="https://cdn.jsdelivr.net/npm/vue@3/dist/vue.global.prod.js"></script>
<script src="https://cdn.jsdelivr.net/npm/element-plus/dist/index.full.min.js"></script>
<script src="https://cdn.jsdelivr.net/npm/@element-plus/icons-vue"></script>
<script src="https://cdn.jsdelivr.net/npm/qrious@4/dist/qrious.min.js"></script>
<script src="https://cdn.jsdelivr.net/npm/marked/marked.min.js"></script>
  <script id="portal-config"></script>
  <script id="portal-custom-js"></script>
</body>
</html>`

const defaultPortalCSS = `[v-cloak] { display: none; }

:root {
  --bg-gradient: linear-gradient(135deg, #2563eb 0%, #1e40af 100%);
  --bg-solid: #f0f2f5;
  --glass-bg: #ffffff;
  --glass-border: #e8f0fe;
  --glass-shadow: 0 2px 12px rgba(37,99,235,0.06);
  --text-primary: #1e293b;
  --text-secondary: #4b5563;
  --text-muted: #9ca3af;
  --accent: #2563eb;
  --accent-dark: #1e40af;
  --accent-light: #eff6ff;
  --accent-border: #dbeafe;
  --success: #059669;
  --danger: #ef4444;
  --warning: #f59e0b;
  --info: #6b7280;
  --purple: #7c3aed;
  --purple-dark: #5b21b6;
  --border: #e8f0fe;
  --ring-bg: #e8f0fe;
  --nav-bg: #ffffff;
  --sidebar-bg: #ffffff;
  --sidebar-width: 250px;
  --font-family: {{.FontFamily}};
}

[data-theme="dark"] {
  --bg-gradient: linear-gradient(135deg, #1e3a5f 0%, #0f172a 100%);
  --bg-solid: #0f172a;
  --glass-bg: #1e293b;
  --glass-border: #334155;
  --glass-shadow: 0 2px 12px rgba(0,0,0,0.3);
  --text-primary: #e2e8f0;
  --text-secondary: #94a3b8;
  --text-muted: #64748b;
  --accent: #3b82f6;
  --accent-dark: #2563eb;
  --accent-light: rgba(59,130,246,0.15);
  --accent-border: rgba(59,130,246,0.2);
  --border: #334155;
  --ring-bg: #334155;
  --nav-bg: #1e293b;
  --sidebar-bg: #1e293b;
}

* { margin: 0; padding: 0; box-sizing: border-box; }
body {
  font-family: {{.FontFamily}};
  background: var(--bg-solid);
  color: var(--text-primary);
  min-height: 100vh;
  -webkit-font-smoothing: antialiased;
}

/* ---- Glass (now portlet-style) ---- */
.glass {
  background: var(--glass-bg);
  border: 1px solid var(--glass-border);
  box-shadow: var(--glass-shadow);
  border-radius: 12px;
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
.btn-block {
  width: 100%;
  background: linear-gradient(135deg, var(--accent), var(--accent-dark)) !important;
  border: none !important;
  font-weight: 700 !important;
  box-shadow: 0 4px 14px rgba(37,99,235,0.25) !important;
  transition: transform 0.15s, box-shadow 0.15s !important;
}
.btn-block:hover {
  transform: translateY(-2px);
  box-shadow: 0 6px 20px rgba(37,99,235,0.35) !important;
}

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
  padding: 0 20px;
  height: 56px;
  position: sticky;
  top: 0;
  z-index: 100;
  background: var(--glass-bg);
  border-bottom: 1px solid var(--border);
  box-shadow: 0 1px 4px rgba(0,0,0,0.04);
}
.top-bar-left { display: flex; align-items: center; gap: 10px; }
.top-logo { width: 30px; height: 30px; border-radius: 8px; }
.top-title { font-size: 17px; font-weight: 700; color: var(--accent); }
.top-bar-right { display: flex; align-items: center; gap: 12px; }
.theme-toggle {
  background: none; border: none; cursor: pointer;
  font-size: 18px; padding: 6px; border-radius: 8px;
  transition: background 0.2s;
}
.theme-toggle:hover { background: var(--accent-light); }
.locale-switcher { display: inline-flex; }
.locale-switcher .theme-toggle { display: inline-flex; align-items: center; justify-content: center; }
.locale-switcher :deep(.el-dropdown-menu__item) { display: flex; align-items: center; justify-content: space-between; }

/* ---- Notice Bell (top bar) ---- */
.notice-bell-wrap { position: relative; display: inline-flex; }
.notice-bell {
  position: relative;
  background: none; border: none; cursor: pointer;
  font-size: 18px; padding: 6px; border-radius: 8px;
  transition: background 0.2s;
  display: inline-flex; align-items: center;
}
.notice-bell:hover { background: var(--accent-light); }
.bell-badge {
  position: absolute; top: 0px; right: 0px;
  min-width: 16px; height: 16px; padding: 0 4px;
  background: linear-gradient(135deg, #ef4444, #dc2626);
  color: #fff; font-size: 10px; font-weight: 700;
  border-radius: 10px; display: flex; align-items: center; justify-content: center;
  line-height: 1; box-shadow: 0 1px 4px rgba(239,68,68,0.4);
}

/* ---- Notice Dropdown Panel (absolute positioned) ---- */
.notice-backdrop {
  position: fixed; inset: 0; z-index: 998;
}
.notice-panel-dropdown {
  position: absolute;
  top: calc(100% + 8px);
  right: 0;
  width: 340px;
  background: var(--glass-bg);
  border: 1px solid var(--border);
  border-radius: 12px;
  box-shadow: 0 8px 32px rgba(0,0,0,0.12);
  z-index: 999;
  overflow: hidden;
}
.notice-panel-header {
  display: flex; align-items: center; justify-content: space-between;
  padding: 12px 16px;
  border-bottom: 1px solid var(--border);
  background: var(--accent-light);
}
.notice-panel-title { font-size: 14px; font-weight: 700; color: var(--accent); }
.notice-panel-count { font-size: 12px; color: var(--text-muted); }
.notice-panel-empty { padding: 32px 16px; text-align: center; color: var(--text-muted); font-size: 13px; }
.notice-panel-list { max-height: 360px; overflow-y: auto; }
.notice-panel-item {
  display: flex; align-items: center; gap: 10px;
  padding: 10px 16px; cursor: pointer; transition: background 0.15s;
  border-bottom: 1px solid var(--border);
}
.notice-panel-item:last-child { border-bottom: none; }
.notice-panel-item:hover { background: var(--accent-light); }
.notice-panel-date {
  font-size: 11px; font-weight: 700; color: var(--accent);
  background: var(--glass-bg); border: 1px solid var(--accent-border);
  padding: 2px 6px; border-radius: 6px; flex-shrink: 0;
}
.notice-panel-text { font-size: 13px; color: var(--text-primary); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; flex: 1; }

.user-badge { cursor: pointer; }
.user-email { font-size: 13px; color: var(--text-secondary); }

/* ---- Desktop Sidebar (qist Metronic style) ---- */
.app-sidebar {
  display: none;
  position: fixed;
  top: 56px;
  left: 0;
  bottom: 0;
  width: var(--sidebar-width);
  background: var(--sidebar-bg);
  border-right: 1px solid var(--border);
  overflow-y: auto;
  z-index: 90;
  padding: 8px 0 24px;
}
.app-sidebar .sidebar-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 22px;
  cursor: pointer;
  font-size: 14px;
  font-weight: 500;
  color: var(--text-secondary);
  transition: all 0.15s;
  border-left: 3px solid transparent;
  position: relative;
  text-decoration: none;
}
.app-sidebar .sidebar-item:hover {
  background: var(--accent-light);
  color: var(--accent);
}
.app-sidebar .sidebar-item.active {
  background: var(--accent-light);
  color: var(--accent);
  border-left-color: var(--accent);
  font-weight: 600;
}
.app-sidebar .sidebar-item.active::after {
  content: '';
  position: absolute;
  right: 16px;
  top: 50%;
  transform: translateY(-50%);
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--accent);
}
.app-sidebar .sidebar-icon {
  font-size: 18px;
  width: 22px;
  text-align: center;
  flex-shrink: 0;
}
.app-sidebar .sidebar-section-label {
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.8px;
  color: var(--text-muted);
  padding: 16px 22px 6px;
  border-top: 1px solid var(--border);
  margin-top: 8px;
}
.app-sidebar .sidebar-section-label:first-child {
  border-top: none;
  margin-top: 0;
}

/* ---- Main Content ---- */
.main-content {
  flex: 1;
  padding: 16px 16px 80px;
  max-width: 1200px;
  width: 100%;
  margin: 0 auto;
}

/* ---- Overview Two-Column Layout (qist style) ---- */
.overview-layout {
  display: flex;
  gap: 20px;
  align-items: flex-start;
}
.overview-main {
  flex: 1;
  min-width: 0; /* prevent overflow */
}
.overview-sidebar {
  width: 280px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.notice-sidebar-card { padding: 14px !important; }
.announcement-sidebar-card { padding: 14px !important; }
.announcement-sidebar-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-top: 8px;
}
.announcement-sidebar-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 7px 10px;
  border-radius: 8px;
  cursor: pointer;
  transition: background 0.15s;
  font-size: 13px;
}
.announcement-sidebar-item:hover {
  background: var(--accent-light);
}
.announcement-popup-badge {
  font-size: 12px;
  flex-shrink: 0;
}
.announcement-sidebar-date {
  font-size: 11px;
  font-weight: 700;
  color: var(--accent);
  white-space: nowrap;
  flex-shrink: 0;
}
.announcement-sidebar-text {
  color: var(--text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* ---- Account Info (qist row style) ---- */
.acct-info-list {
display: flex;
flex-direction: column;
gap: 2px;
}
.acct-info-list.two-column {
display: grid;
grid-template-columns: repeat(2, 1fr);
gap: 8px;
}
.acct-info-list.two-column .acct-info-row {
border-bottom: none;
padding: 8px;
background: var(--glass-bg);
border-radius: 8px;
}
.acct-info-row {
display: flex;
align-items: center;
gap: 12px;
padding: 10px 0;
border-bottom: 1px solid var(--border);
}
.acct-info-row:last-child { border-bottom: none; }
.acct-info-icon {
  font-size: 20px;
  width: 32px;
  text-align: center;
  flex-shrink: 0;
}
.acct-info-body {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
  min-width: 0;
}
.acct-info-label {
  font-size: 11px;
  color: var(--text-muted);
  font-weight: 500;
}
.acct-info-value-row {
  display: flex;
  align-items: center;
  gap: 6px;
}
.acct-info-value {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary);
}
.acct-traffic-section {
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid var(--border);
}
.acct-traffic-label {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 4px;
}
.acct-traffic-pct {
  font-size: 12px;
  font-weight: 700;
  color: var(--accent);
}

/* ---- Cards (portlet style) ---- */
.card {
  border-radius: 12px;
  padding: 20px;
  margin-bottom: 16px;
  border-top: 3px solid var(--accent);
}
.card-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 16px;
}
.card-icon { font-size: 16px; }
.card-title { font-size: 15px; font-weight: 700; color: var(--accent); }
.card-action { font-size: 12px; color: var(--accent); cursor: pointer; margin-left: auto; font-weight: 600; }
.card-desc { font-size: 13px; color: var(--text-secondary); margin-bottom: 12px; line-height: 1.6; }
.card-actions { margin-top: 16px; display: flex; gap: 8px; flex-wrap: wrap; }

/* ---- Info Grid (qist acct-item style) ---- */
.info-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
}
.info-item {
  display: flex;
  flex-direction: column;
  gap: 3px;
  background: var(--accent-light);
  border: 1px solid var(--accent-border);
  border-radius: 10px;
  padding: 11px 13px;
  transition: box-shadow 0.2s, transform 0.15s;
}
.info-item:hover {
  box-shadow: 0 3px 12px rgba(37,99,235,0.09);
  transform: translateY(-1px);
}
.info-label {
  font-size: 11px; color: var(--text-muted);
  font-weight: 500;
  text-transform: uppercase; letter-spacing: 0.4px;
}
.info-value {
  font-size: 13px; font-weight: 600;
  color: var(--text-primary); line-height: 1.4;
  word-break: break-all;
}

/* ---- Badge (qist style) ---- */
.badge {
  display: inline-flex; align-items: center; gap: 5px;
  padding: 2px 9px; border-radius: 20px;
  font-size: 12px; font-weight: 700; letter-spacing: 0.3px;
}
.badge-active { background: #d1fae5; color: #065f46; }
.badge-suspended { background: #fee2e2; color: #991b1b; }
.badge-expired { background: #fef3c7; color: #92400e; }
.badge-revoked { background: #fee2e2; color: #991b1b; }
.badge-done { background: #d1fae5; color: #065f46; }
.badge-processing { background: #dbeafe; color: #1e40af; }
.badge-failed { background: #fee2e2; color: #991b1b; }
.badge-none { background: #f3f4f6; color: #6b7280; }

/* ---- Traffic Ring + Bar (qist style) ---- */
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
.ring-percent { font-size: 28px; font-weight: 700; color: var(--accent); line-height: 1; }
.ring-label { font-size: 12px; color: var(--text-muted); margin-top: 2px; }

/* Horizontal traffic bar (qist style) */
.traffic-bar-wrap { padding: 4px 0 8px; }
.traffic-bar-header {
  display: flex; align-items: center; justify-content: space-between;
  margin-bottom: 6px;
}
.traffic-bar-header .bar-label {
  font-size: 12px; font-weight: 600; color: var(--text-secondary);
  display: flex; align-items: center; gap: 4px;
}
.traffic-bar-header .bar-nums {
  font-size: 12px; color: var(--text-muted);
}
.traffic-bar {
  height: 9px; border-radius: 10px;
  background: var(--ring-bg); overflow: hidden;
}
.traffic-bar-fill {
  height: 100%; border-radius: 10px;
  background: linear-gradient(90deg, var(--accent) 0%, #60a5fa 100%);
  transition: width 0.7s cubic-bezier(.4,0,.2,1);
}
.traffic-bar-fill.warn { background: linear-gradient(90deg, #f59e0b 0%, #fcd34d 100%); }
.traffic-bar-fill.danger { background: linear-gradient(90deg, #ef4444 0%, #f87171 100%); }
.traffic-bar-footer {
  display: flex; align-items: center; justify-content: space-between;
  margin-top: 5px;
}
.traffic-bar-footer .bar-hint { font-size: 11px; color: var(--text-muted); }
.traffic-bar-footer .bar-pct { font-size: 12px; font-weight: 700; color: var(--accent); }

.traffic-stats {
  display: flex;
  align-items: center;
  justify-content: space-around;
  padding: 12px 0;
  margin-top: 8px;
  border-top: 1px solid var(--border);
}
.stat-item {
  display: flex; flex-direction: column; align-items: center; gap: 2px;
}
.stat-icon { font-size: 14px; color: var(--accent); }
.stat-value { font-size: 15px; font-weight: 700; color: var(--text-primary); }
.stat-label { font-size: 11px; color: var(--text-muted); }
.stat-divider { width: 1px; height: 32px; background: var(--border); }

/* ---- Subscription (qist sub-card style) ---- */
.sub-link-box {
  background: var(--accent-light);
  border: 1px solid var(--accent-border);
  border-radius: 12px;
  padding: 14px 16px;
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
.qr-img { max-width: 180px; border-radius: 10px; border: 2px solid var(--accent-border); }
.qr-hint { display: block; font-size: 12px; color: var(--text-muted); margin-top: 8px; }

/* ---- Empty State ---- */
.empty-state { text-align: center; padding: 24px 0; }
.empty-icon { font-size: 40px; display: block; margin-bottom: 8px; }
.empty-text { font-size: 15px; font-weight: 500; color: var(--text-primary); }
.empty-text.danger { color: var(--danger); }
.empty-hint { font-size: 13px; color: var(--text-muted); margin-top: 4px; }

/* ---- Logs (qist style) ---- */
.log-actions { display: flex; gap: 8px; flex-wrap: wrap; }
.logpkg-list { display: flex; flex-direction: column; gap: 10px; }
.logpkg-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 12px;
  background: var(--accent-light);
  border: 1px solid var(--accent-border);
  border-radius: 10px;
}
.logpkg-info { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.logpkg-id { font-size: 13px; font-weight: 600; color: var(--accent); }
.logpkg-time { font-size: 12px; color: var(--text-muted); }
.logpkg-actions { display: flex; align-items: center; gap: 8px; }
.logpkg-size { font-size: 12px; color: var(--text-secondary); }
.logpkg-hint { font-size: 12px; color: var(--accent); }
.logpkg-error { font-size: 12px; color: var(--danger); max-width: 120px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

/* ---- Billing ---- */
.billing-notice {
  padding: 12px 16px;
  margin-bottom: 8px;
  border-radius: 8px;
  font-size: 13px;
  line-height: 1.6;
  color: #dc2626;
  background: rgba(220, 38, 38, 0.06);
  border: 1px solid rgba(220, 38, 38, 0.15);
  word-break: break-word;
}
[data-theme="dark"] .billing-notice {
  background: rgba(220, 38, 38, 0.1);
  border-color: rgba(220, 38, 38, 0.25);
}
.billing-notice-text { color: #dc2626; }
.billing-notice-highlight {
  color: #dc2626;
  font-weight: 700;
}
.billing-notice-sub {
  padding: 8px 16px;
  margin-bottom: 12px;
  border-radius: 8px;
  font-size: 13px;
  line-height: 1.5;
  color: #dc2626;
  background: rgba(220, 38, 38, 0.04);
}
.billing-table-wrap {
  overflow-x: auto;
  border-radius: 10px;
  border: 1px solid var(--border);
}
.billing-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
  min-width: 600px;
}
.billing-table thead th {
  padding: 10px 14px;
  text-align: left;
  font-weight: 600;
  font-size: 12px;
  color: var(--text-muted);
  background: var(--accent-light);
  border-bottom: 1px solid var(--border);
  white-space: nowrap;
}
.billing-table tbody td {
  padding: 10px 14px;
  border-bottom: 1px solid var(--border);
  color: var(--text-primary);
  white-space: nowrap;
  vertical-align: middle;
}
.billing-table tbody tr:last-child td { border-bottom: none; }
.billing-table tbody tr:hover td { background: var(--accent-light); }
.billing-col-index { width: 40px; text-align: center; color: var(--text-muted) !important; }
.billing-col-id { width: 60px; font-weight: 600; color: var(--accent); }
.billing-col-service { min-width: 100px; }
.billing-col-amount { width: 80px; font-weight: 600; }
.billing-col-date { width: 110px; }
.billing-col-expire { width: 110px; }
.billing-col-status { width: 80px; text-align: center; }

/* ---- Node List ---- */
.node-info-card {
  background: linear-gradient(135deg, rgba(56, 189, 248, 0.08), rgba(14, 165, 233, 0.04));
  border: 1px solid rgba(56, 189, 248, 0.15);
  border-radius: 12px;
  padding: 16px;
  margin-bottom: 20px;
}
.node-info-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--accent);
  margin-bottom: 10px;
}
.node-info-content {
  font-size: 12px;
  line-height: 1.8;
  color: var(--text-secondary);
  white-space: pre-line;
}
.node-grid {
  display: grid;
  gap: 12px;
}
.node-card {
  background: var(--glass-bg);
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 12px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.node-name {
  font-size: 13px;
  font-weight: 500;
  color: var(--text-primary);
  word-break: break-all;
}
.node-actions {
  display: flex;
  gap: 8px;
  justify-content: flex-end;
}
.node-card.disabled {
  opacity: 0.6;
  background: var(--accent-light);
}
.node-locked {
  display: flex;
  justify-content: center;
  padding: 4px 0;
}
.plan-node-group {
  margin-bottom: 20px;
}
.plan-node-group:last-child {
  margin-bottom: 0;
}
.plan-node-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 10px;
  padding-bottom: 6px;
  border-bottom: 1px solid var(--border);
}
.plan-node-name {
  font-size: 15px;
  font-weight: 600;
  color: var(--text-primary);
}
.node-region {
  font-size: 11px;
  color: var(--text-secondary);
}
.node-tags {
  display: flex;
  gap: 4px;
  flex-wrap: wrap;
}
.node-tag {
  font-size: 10px;
  padding: 1px 6px;
  border-radius: 4px;
  font-weight: 500;
}
.tag-protocol {
  background: rgba(37, 99, 235, 0.12);
  color: #2563eb;
}
.tag-transport {
  background: rgba(16, 185, 129, 0.12);
  color: #10b981;
}
.node-detail-content {
  padding: 10px 0;
}
.node-detail-tags {
  display: flex;
  gap: 6px;
  margin-bottom: 16px;
}
.node-detail-label {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary);
  margin-bottom: 8px;
}
.node-detail-link-box {
  display: flex;
  gap: 8px;
  align-items: flex-start;
}
.node-detail-link-text {
  flex: 1;
  font-size: 12px;
  background: var(--accent-light, #f5f5f5);
  padding: 8px 12px;
  border-radius: 8px;
  word-break: break-all;
  line-height: 1.6;
  font-family: monospace;
}
.lock-hint {
  font-size: 12px;
  color: var(--text-secondary);
}

/* ---- Tutorial Cards ---- */
.tutorial-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 16px;
}
@media (min-width: 768px) {
  .tutorial-grid {
    grid-template-columns: repeat(3, 1fr);
  }
}
.tutorial-card {
  border-radius: 12px;
  padding: 24px 16px;
  text-align: center;
  color: #fff;
  cursor: pointer;
  transition: transform 0.15s, box-shadow 0.15s;
  position: relative;
  overflow: hidden;
}
.tutorial-card:hover {
  transform: translateY(-4px);
  box-shadow: 0 8px 24px rgba(0,0,0,0.15);
}
.tutorial-card::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: linear-gradient(135deg, rgba(255,255,255,0.1), transparent);
  pointer-events: none;
}
.tutorial-card-icon {
  font-size: 48px;
  line-height: 1;
  margin-bottom: 12px;
  filter: drop-shadow(0 2px 4px rgba(0,0,0,0.2));
}
.tutorial-card-name {
  font-size: 18px;
  font-weight: 600;
  margin-bottom: 8px;
  text-shadow: 0 1px 2px rgba(0,0,0,0.2);
}
.tutorial-card-desc {
  font-size: 12px;
  opacity: 0.9;
  line-height: 1.5;
}

/* ---- Tutorial Platform Tabs ---- */
.tutorial-platform-tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 16px;
  padding: 0 4px;
}
.tutorial-tab {
  padding: 8px 16px;
  border-radius: 20px;
  background: var(--card-bg);
  border: 1px solid var(--border);
  color: var(--text-secondary);
  font-size: 13px;
  cursor: pointer;
  transition: all 0.2s;
  white-space: nowrap;
}
.tutorial-tab:hover {
  background: var(--accent-light);
  color: var(--accent);
}
.tutorial-tab.active {
  background: var(--accent);
  color: #fff;
  border-color: var(--accent);
}
.tutorial-card.active {
  box-shadow: 0 0 0 3px var(--accent), 0 8px 24px rgba(0,0,0,0.15);
}

/* ---- Tutorial Articles ---- */
.tutorial-articles {
  margin-top: 20px;
  padding-top: 20px;
  border-top: 1px solid var(--border);
}
.tutorial-articles-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
  padding: 0 4px;
}
.tutorial-platform-name {
  font-size: 16px;
  font-weight: 600;
  color: var(--text-primary);
}
.tutorial-articles-count {
  font-size: 13px;
  color: var(--text-secondary);
  background: var(--accent-light);
  padding: 4px 10px;
  border-radius: 12px;
}
.tutorial-article-list {
  max-height: 400px;
  overflow-y: auto;
}
.tutorial-select-hint {
  text-align: center;
  padding: 40px 20px;
  color: var(--text-secondary);
}

/* ---- Ticket Dialog ---- */
.ticket-dialog .el-dialog__body {
  padding: 20px 24px;
}
.ticket-type-group {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.ticket-type-group .el-radio-button__inner {
  border-radius: 8px !important;
  border: 1px solid var(--border);
  padding: 8px 16px;
  font-size: 13px;
}
.ticket-type-group .el-radio-button.is-active .el-radio-button__inner {
  background: var(--accent);
  border-color: var(--accent);
  box-shadow: none;
}
.ticket-submit-wrap {
  margin-top: 24px;
  text-align: center;
}
.ticket-submit-btn {
  min-width: 200px;
  border-radius: 8px;
}

/* ---- Referral ---- */
.referral-header {
  margin-bottom: 16px;
}
.referral-title {
  font-size: 18px;
  font-weight: 600;
  margin-bottom: 8px;
  color: var(--text);
}
.referral-meta {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 13px;
  color: var(--text-secondary);
}
.referral-rate, .referral-mode {
  color: var(--accent);
  font-weight: 500;
}
.referral-divider {
  color: var(--border);
}
.referral-link-wrap {
  display: flex;
  align-items: center;
  gap: 12px;
  background: var(--card-bg);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 12px 16px;
  margin-bottom: 16px;
}
.referral-link {
  flex: 1;
  font-size: 13px;
  color: var(--text-secondary);
  word-break: break-all;
  font-family: monospace;
}
.referral-copy-btn {
  flex-shrink: 0;
}
.referral-code-section {
  background: var(--card-bg);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 16px;
  margin-bottom: 24px;
}
.referral-code-row {
  display: flex;
  align-items: center;
  gap: 16px;
}
.referral-code-label {
  font-size: 14px;
  color: var(--text-secondary);
  min-width: 60px;
}
.referral-code-value {
  font-size: 16px;
  font-weight: 600;
  color: var(--accent);
  font-family: monospace;
  flex: 1;
}
.referral-status {
  flex-shrink: 0;
}
.referral-section {
  margin-bottom: 24px;
}
.section-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 12px;
  flex-wrap: wrap;
}
.section-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--text);
}
.section-subtitle {
  font-size: 12px;
  color: var(--text-secondary);
  flex: 1;
}
.withdraw-btn {
  flex-shrink: 0;
}
.referral-table-wrap {
  overflow-x: auto;
  background: var(--card-bg);
  border: 1px solid var(--border);
  border-radius: 8px;
}
.referral-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}
.referral-table th,
.referral-table td {
  padding: 12px 16px;
  text-align: left;
  border-bottom: 1px solid var(--border);
}
.referral-table th {
  background: var(--bg);
  font-weight: 600;
  color: var(--text-secondary);
  font-size: 12px;
  text-transform: uppercase;
}
.referral-table tr:last-child td {
  border-bottom: none;
}
.referral-table tbody tr:hover {
  background: var(--bg);
}
.referral-summary {
  margin-top: 12px;
  font-size: 13px;
  color: var(--text-secondary);
  text-align: right;
}

/* ---- Client Download ---- */
.client-platform-tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 16px;
}
.platform-tab {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 16px;
  border-radius: 6px;
  background: var(--card-bg);
  border: 1px solid var(--border);
  cursor: pointer;
  transition: all 0.2s;
  font-size: 13px;
}
.platform-tab:hover {
  border-color: var(--accent);
}
.platform-tab.active {
  background: var(--accent);
  color: white;
  border-color: var(--accent);
}
.client-download-hint {
  text-align: center;
  color: var(--text-secondary);
  font-size: 13px;
  padding: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
}
.client-download-content {
  padding: 4px 0;
}
.client-download-platform-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}
.client-download-platform-name {
  font-size: 15px;
  font-weight: 600;
  color: var(--text-primary);
}
.client-download-link-row {
  margin-bottom: 16px;
}
.client-download-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 12px 20px;
  border-radius: 10px;
  background: linear-gradient(135deg, var(--accent), var(--accent-dark, var(--accent)));
  color: #fff;
  text-decoration: none;
  font-size: 14px;
  font-weight: 500;
  transition: all 0.2s;
}
.client-download-btn:hover {
  opacity: 0.9;
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(0,0,0,0.15);
}
.client-download-articles {
  margin-top: 4px;
}
.client-download-section-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-secondary);
  margin-bottom: 8px;
  padding-bottom: 6px;
  border-bottom: 1px solid var(--border);
}
.article-list.compact .article-item {
  padding: 8px 10px;
}
.article-list.compact .article-title {
  font-size: 13px;
}

/* ---- Subscription Actions ---- */
.sub-actions-row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid var(--border);
}

/* ---- More Subscriptions Dialog ---- */
.more-subs-list { max-height: 60vh; overflow-y: auto; padding: 8px 0; }
.more-subs-card {
  background: var(--bg-secondary);
  border-radius: 12px;
  padding: 16px;
  margin-bottom: 12px;
  border: 1px solid var(--border);
}
.more-subs-card:last-child { margin-bottom: 0; }
.more-subs-card-header { display: flex; align-items: center; gap: 8px; margin-bottom: 12px; }
.more-subs-card-icon { font-size: 20px; }
.more-subs-card-title { font-size: 15px; font-weight: 600; color: var(--text-primary); }
.more-subs-card-desc { font-size: 12px; color: var(--text-muted); margin-left: auto; }
.more-subs-config-list { display: flex; flex-direction: column; gap: 8px; }
.more-subs-config-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 12px;
  background: var(--bg);
  border-radius: 8px;
  border: 1px solid var(--border);
}
.more-subs-config-info { display: flex; align-items: center; gap: 8px; flex: 1; min-width: 0; }
.more-subs-config-name { font-size: 14px; font-weight: 500; color: var(--text-primary); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.more-subs-config-label { font-size: 13px; color: var(--text-secondary); }
.more-subs-config-tag { font-size: 11px; text-transform: capitalize; }
.more-subs-config-actions { display: flex; gap: 6px; flex-shrink: 0; }
.more-subs-qr-section { text-align: center; margin-top: 12px; padding: 16px; background: var(--bg); border-radius: 8px; border: 1px dashed var(--border); }
.more-subs-qr-img { width: 180px; height: 180px; border-radius: 8px; }
.more-subs-qr-hint { display: block; font-size: 12px; color: var(--text-muted); margin-top: 8px; }
.more-subs-empty { text-align: center; padding: 40px; color: var(--text-muted); }
.more-subs-dialog .el-dialog__body { padding: 16px 20px; }

/* ---- Streaming Status ---- */
.streaming-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 16px;
}
.streaming-card {
  background: var(--card-bg);
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 20px;
  text-align: center;
  transition: all 0.2s;
}
.streaming-card:hover {
  border-color: var(--accent);
  box-shadow: 0 4px 12px rgba(0,0,0,0.1);
}
.streaming-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--text);
  margin-bottom: 16px;
}
.streaming-icon {
  width: 64px;
  height: 64px;
  margin: 0 auto 16px;
  display: flex;
  align-items: center;
  justify-content: center;
}
.streaming-icon img {
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
}
.streaming-action {
  color: var(--accent);
  font-size: 13px;
  cursor: pointer;
  text-decoration: underline;
}
.streaming-action:hover {
  opacity: 0.8;
}

/* ---- Plan Table ---- */
.plan-notice {
  background: #fff5f5;
  border: 1px solid #feb2b2;
  border-radius: 8px;
  padding: 16px;
  margin-bottom: 20px;
}
.plan-notice-content {
  color: #c53030;
  font-size: 13px;
  line-height: 1.6;
  margin-bottom: 8px;
}
.plan-notice-sub {
  color: #9b2c2c;
  font-size: 12px;
  line-height: 1.5;
}
.plan-table-wrap {
  overflow-x: auto;
  background: var(--card-bg);
  border: 1px solid var(--border);
  border-radius: 8px;
}
.plan-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}
.plan-table th {
  background: var(--bg);
  padding: 12px 16px;
  text-align: left;
  font-weight: 600;
  color: var(--text-secondary);
  border-bottom: 2px solid var(--border);
}
.plan-table td {
  padding: 16px;
  border-bottom: 1px solid var(--border);
  vertical-align: top;
}
.plan-table tr:last-child td {
  border-bottom: none;
}
.plan-table tr:hover {
  background: var(--bg);
}
.plan-col-service {
  width: 25%;
  min-width: 180px;
}
.plan-col-desc {
  width: 45%;
}
.plan-col-price {
  width: 15%;
  text-align: center;
}
.plan-col-action {
  width: 15%;
  text-align: center;
}
.plan-service-name {
  font-size: 15px;
  font-weight: 600;
  color: var(--text);
  margin-bottom: 8px;
}
.plan-service-meta {
  font-size: 12px;
  color: var(--text-secondary);
  line-height: 1.6;
}
.plan-service-meta div {
  margin-bottom: 2px;
}
.plan-desc-text {
  color: var(--text-secondary);
  font-size: 13px;
  line-height: 1.5;
}
.plan-price-amount {
  font-size: 16px;
  font-weight: 600;
  color: var(--text);
}
.plan-current-badge {
  margin-top: 8px;
  font-size: 11px;
  color: var(--accent);
}
.plan-current .plan-service-name {
  color: var(--accent);
}

/* ---- Bottom Nav (mobile only) ---- */
.bottom-nav {
  position: fixed;
  bottom: 0; left: 0; right: 0;
  display: flex;
  justify-content: space-around;
  padding: 6px 0 max(6px, env(safe-area-inset-bottom));
  border-top: 1px solid var(--border);
  background: var(--nav-bg);
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

/* ---- Responsive: Desktop shows sidebar, mobile shows bottom nav ---- */
@media (min-width: 768px) {
  .app-sidebar { display: block; }
  .bottom-nav { display: none; }
  .main-content {
    margin-left: var(--sidebar-width);
    padding: 24px 32px 32px;
    max-width: 1200px;
  }
  /* Two-column overview layout on desktop */
  .overview-layout { flex-direction: row; }
  .overview-sidebar { display: flex; width: 280px; }
  .info-grid { grid-template-columns: 1fr 1fr 1fr 1fr; }
  .feature-cards-grid { grid-template-columns: repeat(auto-fill, minmax(160px, 1fr)); }
}

@media (max-width: 767px) {
  .app-sidebar { display: none; }
  .bottom-nav { display: flex; }
  /* Single column on mobile - sidebar moves below main content */
  .overview-layout { flex-direction: column; }
  .overview-sidebar { width: 100%; }
}

/* ---- Element Plus Overrides ---- */
.el-button--primary { --el-color-primary: #2563eb; }
.el-button--primary {
  background: linear-gradient(135deg, var(--accent), var(--accent-dark)) !important;
  border: none !important;
  font-weight: 600 !important;
  box-shadow: 0 2px 8px rgba(37,99,235,0.18) !important;
}
.el-button--primary:hover {
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(37,99,235,0.28) !important;
}
.el-button--success {
  background: linear-gradient(135deg, #059669, #047857) !important;
  border: none !important;
}
.el-button--warning {
  background: linear-gradient(135deg, #f59e0b, #d97706) !important;
  border: none !important;
}
[data-theme="dark"] .el-input__wrapper { background: rgba(30,41,59,0.5); }
[data-theme="dark"] .el-tabs__item { color: var(--text-secondary); }
[data-theme="dark"] .el-tabs__item.is-active { color: var(--accent); }
[data-theme="dark"] .el-form-item__label { color: var(--text-secondary); }
[data-theme="dark"] .el-dropdown-menu { background: var(--glass-bg); border: 1px solid var(--border); }
[data-theme="dark"] .el-select__wrapper { background: rgba(30,41,59,0.5); }
[data-theme="dark"] .el-dialog { background: var(--glass-bg); border: 1px solid var(--border); }

/* ---- Module Styles (Portal Editor) ---- */
.module-wrap { margin-bottom: 16px; border-top: 3px solid var(--accent); }
.module-hero { padding: 8px 0; }
.module-hero .hero-eyebrow { font-size: 11px; text-transform: uppercase; letter-spacing: 1px; color: var(--accent); margin-bottom: 4px; }
.module-hero .hero-heading { font-size: 22px; font-weight: 700; color: var(--text-primary); margin: 4px 0; }
.module-hero .hero-desc { font-size: 14px; color: var(--text-secondary); margin-bottom: 12px; }
.module-hero .hero-btn {
  display: inline-flex; align-items: center; gap: 7px;
  padding: 9px 22px; border-radius: 8px;
  background: linear-gradient(135deg, var(--accent), var(--accent-dark));
  color: #fff; font-weight: 700; font-size: 14px;
  text-decoration: none; cursor: pointer; border: none;
  box-shadow: 0 2px 8px rgba(37,99,235,0.22);
  transition: transform 0.15s, box-shadow 0.15s;
}
.module-hero .hero-btn:hover {
  transform: translateY(-2px);
  box-shadow: 0 5px 16px rgba(37,99,235,0.32);
}
.module-rich-text { font-size: 14px; line-height: 1.7; color: var(--text-primary); }
.module-rich-text h2 { font-size: 18px; margin: 8px 0; }
.module-rich-text h3 { font-size: 16px; margin: 8px 0; }
.module-rich-text p { margin: 6px 0; }
.module-rich-text blockquote { border-left: 3px solid var(--accent); padding-left: 12px; margin: 8px 0; color: var(--text-secondary); }
.module-rich-text ul, .module-rich-text ol { padding-left: 20px; margin: 6px 0; }
.module-notice { border-left: 4px solid; padding: 12px 14px; border-radius: 8px; font-size: 13px; line-height: 1.6; }
.notice-info { border-color: var(--accent); background: var(--accent-light); }
.notice-success { border-color: var(--success); background: #ecfdf5; }
.notice-warning { border-color: var(--warning); background: #fffbeb; }
.notice-error { border-color: var(--danger); background: #fef2f2; }
.module-notice .notice-title { font-weight: 600; margin-bottom: 4px; }
.notice-popup-hint { font-size: 11px; color: var(--text-muted); margin-top: 6px; }
.notice-article-preview { margin: 8px 0; }
.notice-article-desc { font-size: 13px; color: var(--text-secondary); line-height: 1.5; margin: 0; }
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
.feature-card { padding: 14px; background: var(--accent-light); border: 1px solid var(--accent-border); border-radius: 12px; text-align: center; transition: transform 0.15s, box-shadow 0.15s; }
.feature-card:hover { transform: translateY(-2px); box-shadow: 0 4px 16px rgba(37,99,235,0.08); }
.feature-card-icon { font-size: 28px; display: block; margin-bottom: 6px; }
.feature-card h4 { font-size: 13px; margin: 2px 0; color: var(--text-primary); font-weight: 700; }
.feature-card p { font-size: 12px; color: var(--text-muted); line-height: 1.5; }
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
.md-render code { background: var(--accent-light); padding: 2px 6px; border-radius: 4px; font-size: 13px; font-family: 'SF Mono','Fira Code',Consolas,monospace; color: var(--accent); }
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
/* Plan List (qist horizontal row style) */
.plan-grid { display: flex; flex-direction: column; gap: 10px; margin-top: 8px; }
.plan-card {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 14px 18px;
  background: var(--glass-bg);
  border: 1px solid var(--border);
  border-radius: 12px;
  transition: box-shadow 0.2s, transform 0.15s;
  border-left: 4px solid var(--accent);
}
.plan-card:hover { box-shadow: 0 3px 16px rgba(37,99,235,0.08); transform: translateY(-1px); }
.plan-card.plan-popular { border-left-color: #f59e0b; border-color: rgba(245,158,11,0.2); }
/* Plan info section (left) */
.plan-info { flex: 1; min-width: 0; }
.plan-name { font-size: 15px; font-weight: 700; color: var(--text-primary); margin-bottom: 2px; }
.plan-name .plan-badge {
  display: inline-block; font-size: 10px; font-weight: 700; color: #fff;
  background: linear-gradient(135deg, #f59e0b, #d97706);
  padding: 1px 7px; border-radius: 8px; margin-left: 6px; vertical-align: middle;
}
.plan-desc { font-size: 12px; color: var(--text-muted); margin-bottom: 4px; line-height: 1.5; }
.plan-meta-row { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; }
.plan-meta { display: flex; justify-content: flex-start; gap: 6px; }
.plan-cycle, .plan-duration {
  font-size: 11px; color: var(--text-secondary); background: var(--accent-light);
  padding: 1px 7px; border-radius: 6px; border: 1px solid var(--accent-border);
}
.plan-quota { font-size: 11px; color: var(--text-muted); }
/* Plan price + action section (right) */
-plan-price-section {
  display: flex; flex-direction: column; align-items: flex-end; gap: 4px;
  flex-shrink: 0; min-width: 120px;
}
.plan-price { display: flex; align-items: baseline; gap: 2px; margin-bottom: 2px; }
.price-amount { font-size: 22px; font-weight: 800; color: var(--accent); line-height: 1; }
.price-currency { font-size: 12px; color: var(--text-muted); }
.price-original { font-size: 11px; color: var(--text-muted); text-decoration: line-through; margin-left: 4px; }
.plan-btn {
  display: inline-flex; align-items: center; justify-content: center; gap: 5px;
  padding: 7px 20px; border-radius: 8px; border: none; cursor: pointer;
  background: linear-gradient(135deg, var(--accent), var(--accent-dark));
  color: #fff !important; font-size: 13px; font-weight: 700;
  white-space: nowrap; letter-spacing: 0.2px;
  box-shadow: 0 2px 8px rgba(37,99,235,0.25);
  transition: transform 0.15s, box-shadow 0.15s;
}
.plan-btn:hover { transform: translateY(-1px); box-shadow: 0 4px 14px rgba(37,99,235,0.35); }
.plan-variant { border-top: none; padding: 0; }

/* Charge Panel */
.charge-section { margin-top: 16px; padding-top: 16px; border-top: 1px solid var(--border); }
.charge-section:first-child { margin-top: 0; padding-top: 0; border-top: none; }
.charge-subtitle { font-size: 14px; font-weight: 700; color: var(--text-primary); margin-bottom: 10px; }
.charge-card-row { display: flex; gap: 8px; }

/* ===== Payment/Charge Dialog (v3.4) ===== */
.payment-dialog .el-dialog__header { padding: 20px 24px 12px; margin-right: 0; }
.payment-dialog .el-dialog__title { font-size: 17px; font-weight: 700; }
.payment-dialog .el-dialog__body { padding: 8px 24px 16px; }
.payment-dialog .el-dialog__footer { padding: 12px 24px 20px; }
.payment-dialog-content { }

/* Order Summary */
.pay-summary {
  background: var(--bg-secondary, rgba(0,0,0,0.03)); border-radius: 12px; padding: 14px 16px; margin-bottom: 20px;
  display: flex; flex-direction: column; gap: 10px;
}
.pay-summary-row { display: flex; align-items: center; justify-content: space-between; }
.pay-summary-label { font-size: 13px; color: var(--text-muted, #999); }
.pay-summary-value { font-size: 14px; font-weight: 600; color: var(--text-primary, #333); }
.pay-summary-value.text-danger { color: var(--danger, #ef4444); }
.pay-summary-price { padding-top: 10px; margin-top: 4px; border-top: 1px dashed var(--border, #e5e7eb); }
.pay-price-num { font-size: 20px; font-weight: 800; color: var(--accent, #2563eb); }

/* Order Type Badge */
.pay-type-tag {
  display: inline-block; padding: 2px 10px; border-radius: 6px; font-size: 12px; font-weight: 600;
}
.pay-type-new { background: rgba(37,99,235,0.12); color: #2563eb; }
.pay-type-renew { background: rgba(16,185,129,0.12); color: #10b981; }
.pay-type-upgrade { background: rgba(245,158,11,0.12); color: #f59e0b; }

/* Section Title */
.pay-section-title {
  font-size: 14px; font-weight: 700; color: var(--text-primary, #333); margin-bottom: 12px;
  display: flex; align-items: center; gap: 6px;
}
.pay-section-title::before {
  content: ''; display: inline-block; width: 3px; height: 14px; border-radius: 2px;
  background: var(--accent, #2563eb);
}

/* Payment Method List */
.pay-method-list { display: flex; flex-direction: column; gap: 10px; margin-bottom: 16px; }
.pay-method-card {
  display: flex; align-items: center; gap: 12px; padding: 14px 16px;
  border: 2px solid var(--border, #e5e7eb); border-radius: 12px; cursor: pointer;
  transition: all 0.2s ease; background: var(--bg-card, transparent); position: relative;
}
.pay-method-card:hover { border-color: rgba(37,99,235,0.4); background: rgba(37,99,235,0.04); }
.pay-method-card.active {
  border-color: var(--accent, #2563eb); background: rgba(37,99,235,0.06);
  box-shadow: 0 2px 12px rgba(37,99,235,0.12);
}

/* Payment Method Icon */
.pay-method-icon {
  width: 42px; height: 42px; border-radius: 10px; display: flex; align-items: center; justify-content: center;
  font-size: 20px; flex-shrink: 0;
}
.pay-method-icon-balance { background: rgba(37,99,235,0.1); color: #2563eb; }
.pay-method-icon-card { background: rgba(16,185,129,0.1); color: #10b981; }
.pay-method-icon-online { background: rgba(245,158,11,0.1); color: #f59e0b; }

/* Payment Method Info */
.pay-method-info { flex: 1; min-width: 0; }
.pay-method-name { font-size: 15px; font-weight: 600; color: var(--text-primary, #333); margin-bottom: 2px; }
.pay-method-desc { font-size: 12px; color: var(--text-muted, #999); }

/* Selected Check */
.pay-method-check {
  width: 24px; height: 24px; border-radius: 50%; background: var(--accent, #2563eb); color: #fff;
  display: flex; align-items: center; justify-content: center; font-size: 14px; flex-shrink: 0;
  animation: pay-check-pop 0.2s ease;
}
@keyframes pay-check-pop { 0% { transform: scale(0); } 80% { transform: scale(1.15); } 100% { transform: scale(1); } }

/* Expanded Form */
.pay-extra-form { margin-bottom: 16px; }
.pay-extra-label { font-size: 13px; font-weight: 600; color: var(--text-muted, #666); margin-bottom: 8px; }
.pay-hint { font-size: 12px; color: var(--text-muted, #999); margin-top: 8px; }

/* Insufficient Balance Warning */
.pay-balance-warn {
  display: flex; align-items: center; gap: 6px; padding: 10px 14px; margin-bottom: 16px;
  background: rgba(239,68,68,0.08); border: 1px solid rgba(239,68,68,0.2); border-radius: 10px;
  color: var(--danger, #ef4444); font-size: 13px;
}

/* Promo Code Fold */
.pay-promo-toggle {
  display: flex; align-items: center; justify-content: space-between; padding: 10px 14px;
  border-radius: 10px; cursor: pointer; font-size: 13px; color: var(--text-muted, #666);
  background: var(--bg-secondary, rgba(0,0,0,0.03)); transition: background 0.15s;
}
.pay-promo-toggle:hover { background: rgba(0,0,0,0.06); }
.pay-promo-arrow { transition: transform 0.2s; font-size: 12px; }
.pay-promo-arrow.rotated { transform: rotate(180deg); }

/* Charge Amount Grid */
.charge-amount-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 8px; }
.charge-amount-btn {
  display: flex; align-items: baseline; justify-content: center; gap: 1px;
  padding: 12px 8px; border-radius: 10px; border: 2px solid var(--border, #e5e7eb);
  cursor: pointer; font-size: 14px; transition: all 0.15s; background: var(--bg-card, transparent);
}
.charge-amount-btn:hover { border-color: rgba(37,99,235,0.4); }
.charge-amount-btn.active {
  border-color: var(--accent, #2563eb); background: rgba(37,99,235,0.08); color: var(--accent, #2563eb);
  font-weight: 700;
}
.charge-amount-num { font-size: 16px; font-weight: 700; }
.charge-amount-unit { font-size: 12px; opacity: 0.8; }
.charge-amount-custom { font-size: 13px; color: var(--text-muted, #666); }

/* Bottom Button */
.pay-dialog-footer { display: flex; gap: 10px; justify-content: flex-end; }
.pay-confirm-btn { min-width: 120px; }

/* Transition Animation */
.pay-fade-enter-active, .pay-fade-leave-active { transition: all 0.25s ease; }
.pay-fade-enter-from { opacity: 0; transform: translateY(-8px); }
.pay-fade-leave-to { opacity: 0; transform: translateY(-4px); }

/* Dark mode */
[data-theme="dark"] .pay-method-card { background: rgba(255,255,255,0.03); }
[data-theme="dark"] .pay-method-card:hover { background: rgba(37,99,235,0.08); }
[data-theme="dark"] .pay-method-card.active { background: rgba(37,99,235,0.1); }
[data-theme="dark"] .pay-summary { background: rgba(255,255,255,0.05); }
[data-theme="dark"] .pay-promo-toggle { background: rgba(255,255,255,0.05); }
[data-theme="dark"] .pay-promo-toggle:hover { background: rgba(255,255,255,0.08); }
[data-theme="dark"] .charge-amount-btn { background: rgba(255,255,255,0.03); }
[data-theme="dark"] .charge-amount-btn:hover { border-color: rgba(37,99,235,0.4); }
[data-theme="dark"] .charge-amount-btn.active { background: rgba(37,99,235,0.12); }

/* Balance Display */
.balance-hero { text-align: center; padding: 20px 0; }
.balance-amount { font-size: 36px; font-weight: 700; color: var(--accent); }
.balance-frozen { display: block; font-size: 13px; color: var(--text-muted); margin-top: 4px; }

/* Referral */
.referral-link-box {
  background: var(--accent-light); border: 1px solid var(--accent-border); border-radius: 12px; padding: 14px 16px;
  display: flex; align-items: center; gap: 8px; margin-bottom: 16px;
}
.referral-stats {
  display: flex; align-items: center; justify-content: space-around; padding: 16px 0;
  background: var(--accent-light); border: 1px solid var(--accent-border); border-radius: 12px;
}

/* Article List */
.article-filter { margin-bottom: 12px; }
.article-list { display: flex; flex-direction: column; gap: 8px; }
.article-item {
  display: flex; align-items: center; gap: 10px; padding: 10px 12px;
  background: var(--accent-light); border: 1px solid var(--accent-border); border-radius: 10px; cursor: pointer; transition: background 0.2s, transform 0.15s;
}
.article-item:hover { background: var(--accent-border); transform: translateY(-1px); }
.article-cat-badge {
  font-size: 11px; font-weight: 700; color: var(--accent);
  background: var(--glass-bg); padding: 2px 10px; border-radius: 20px; flex-shrink: 0;
  border: 1px solid var(--accent-border);
}
.article-info { flex: 1; display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.article-title { font-size: 14px; font-weight: 500; color: var(--text-primary); }
.article-arrow { color: var(--text-muted); flex-shrink: 0; font-size: 14px; }
.sa-icon { width: 32px; height: 32px; border-radius: 8px; flex-shrink: 0; }

/* Ticket Detail */
.ticket-detail-header { display: flex; align-items: center; gap: 8px; margin-bottom: 8px; }
.ticket-subject { font-size: 16px; font-weight: 700; color: var(--text-primary); margin-bottom: 12px; }
.ticket-msgs { max-height: 300px; overflow-y: auto; display: flex; flex-direction: column; gap: 12px; }
.ticket-msg { padding: 10px 12px; border-radius: 10px; max-width: 80%; }
.ticket-msg.msg-client { background: var(--accent-light); border: 1px solid var(--accent-border); align-self: flex-end; text-align: right; }
.ticket-msg.msg-admin { background: var(--glass-bg); border: 1px solid var(--border); align-self: flex-start; text-align: left; }
.ticket-msg-sender { font-size: 12px; font-weight: 700; color: var(--accent); display: block; margin-bottom: 4px; }
.ticket-msg-content { font-size: 14px; color: var(--text-primary); line-height: 1.6; }
.ticket-reply { margin-top: 16px; border-top: 1px solid var(--border); padding-top: 12px; min-height: 100px; }
.ticket-new-badge { background: #f56c6c; color: #fff; font-size: 11px; padding: 2px 6px; border-radius: 10px; margin-left: 8px; }
.ticket-detail-dialog .el-dialog__body { max-height: 80vh; overflow-y: auto; padding-bottom: 20px; }
.ticket-detail { display: flex; flex-direction: column; }
.ticket-msgs { flex: 1; overflow-y: auto; max-height: 400px; }

/* Article Dialog */
.article-dialog .el-dialog__body { max-height: 60vh; overflow-y: auto; }`

const defaultPortalJS = `var createApp = Vue.createApp
var ref = Vue.ref
var computed = Vue.computed
var onMounted = Vue.onMounted
var onUnmounted = Vue.onUnmounted
var watch = Vue.watch
var ElMessage = ElementPlus.ElMessage
var ElMessageBox = ElementPlus.ElMessageBox

var app = createApp({
  setup: function() {
    var token = ref(localStorage.getItem('client_token') || '')
    var refreshToken = ref(localStorage.getItem('client_refresh_token') || '')
    var authTab = ref('login')
    var loading = ref(false)
    var rotating = ref(false)
    var refreshing = ref(false)
    var activeTab = ref(localStorage.getItem('portal_active_tab') || 'overview')
    var showPasswordDialog = ref(false)
var showQr = ref(false)
var showSubQrDialog = ref(false)
var showSubUrl = ref(false)
    var selectedPlatform = ref('')
var announcementPopupVisible = ref(false)
var activeAnnouncement = ref(null)
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
    var orderForm = ref({ variant_id: null, order_type: 'new', promotion_code: '', pay_method: 'balance', charge_site_id: null, gift_card_code: '' })
    var orderBusy = ref(false)
    var chargeSites = ref([])
    var chargeForm = ref({ site_id: null, amount_yuan: 10, method: 'online', custom_amount: false })
    // if no charge sites, default to card
    watch(chargeSites, function(sites) {
      if (!sites || sites.length === 0) {
        chargeForm.value.method = 'card'
      }
    }, { immediate: true })
    var chargeBusy = ref(false)
    var chargeOrders = ref([])
    var cardCode = ref('')
    var redeemBusy = ref(false)
var balance = ref({ balance: 0, frozen: 0 })
var balanceTxns = ref([])
// --- Order list ---
var myOrders = ref([])
var myOrdersLoading = ref(false)
var referralCode = ref(null)
var referralStats = ref(null)
var referralRecords = ref([])
var commissionRecords = ref([])
var withdrawRecords = ref([])
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
    var announcements = ref([])
    var showNoticePanel = ref(false)
    var sharedAccounts = ref([])
var showCredentialsDialog = ref(false)
    var sharedAccountCreds = ref(null)
    var showChargeDialog = ref(false)
    var showPaymentDialog = ref(false) // v3.3: payment method dialog
var showPromoInput = ref(false) // promo code fold

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
    // --- i18n / multi-language pack ---
    var multiLang = { default: 'zh-CN', locales: {} }
    if (typeof PORTAL_CONFIG !== 'undefined' && PORTAL_CONFIG.lang) {
      var rawLang = PORTAL_CONFIG.lang
      if (rawLang && rawLang.locales && typeof rawLang.locales === 'object') {
        multiLang = rawLang
      } else if (rawLang && typeof rawLang === 'object') {
        multiLang = { default: 'zh-CN', locales: { 'zh-CN': rawLang } }
      }
    }
    // Detect preferred locale
    var savedLocale = localStorage.getItem('portal_locale')
    var browserLang = (navigator.language || navigator.userLanguage || 'zh-CN').toLowerCase()
    var detectedLocale = savedLocale
    if (!detectedLocale || !multiLang.locales[detectedLocale]) {
      if (multiLang.locales[browserLang]) {
        detectedLocale = browserLang
      } else {
        var prefix = browserLang.split('-')[0]
        var found = false
        for (var code in multiLang.locales) {
          if (code.split('-')[0] === prefix) { detectedLocale = code; found = true; break }
        }
        if (!found) detectedLocale = multiLang.default || 'zh-CN'
      }
    }
    var currentLocale = ref(detectedLocale)
    var availableLocales = computed(function() {
      var list = []
      for (var code in multiLang.locales) {
        var meta = (multiLang.locales[code] && multiLang.locales[code]._meta) || {}
        list.push({ code: code, label: meta.label || code, flag: meta.flag || '🌐' })
      }
      return list
    })
    var currentLocaleFlag = computed(function() {
      var loc = multiLang.locales[currentLocale.value]
      if (loc && loc._meta && loc._meta.flag) return loc._meta.flag
      return '🌐'
    })
    var lang = computed(function() {
      return multiLang.locales[currentLocale.value] || multiLang.locales[multiLang.default] || {}
    })
    var t = function(key) {
      if (!key) return ''
      var v = lang.value[key]
      if (v === undefined || v === null || v === '') {
        var defaultPack = multiLang.locales[multiLang.default] || {}
        v = defaultPack[key]
      }
      if (v === undefined || v === null || v === '') return key
      return v
    }
    var switchLocale = function(code) {
      if (multiLang.locales[code]) {
        currentLocale.value = code
        localStorage.setItem('portal_locale', code)
        document.documentElement.lang = code
      }
    }
    // Fallback to default nav if none configured
    if (portalNavItems.value.length === 0) {
      portalNavItems.value = [
        { id: 'nav-home', label: t('nav.home'), icon: '🏠', target: 'overview', enabled: true },
        { id: 'nav-services', label: t('nav.store'), icon: '🛒', target: 'store', enabled: true },
        { id: 'nav-billing', label: t('nav.billing'), icon: '💰', target: 'billing', enabled: true },
        { id: 'nav-nodes', label: t('nav.nodes'), icon: '📋', target: 'nodes', enabled: true },
        { id: 'nav-tutorial', label: t('nav.tutorial'), icon: '📚', target: 'tutorial', enabled: true },
        { id: 'nav-tickets', label: t('nav.support'), icon: '🎫', target: 'support', enabled: true },
        { id: 'nav-referral', label: t('nav.referral'), icon: '🎁', target: 'referral', enabled: true },
        { id: 'nav-profile', label: t('nav.profile'), icon: '👤', target: 'profile', enabled: true },
        { id: 'nav-logs', label: t('nav.logs'), icon: '📦', target: 'logs', enabled: true }
      ]
    }
    // Modules for the current page (filtered by activeTab)
    var currentModules = computed(function() {
      return portalModules.value.filter(function(m) { return (m.page || 'overview') === activeTab.value })
    })

    // Client visible nodes (from /nodes API — only for user's current plan, provides links)
    var clientNodes = ref([])
    var fetchNodes = function() {
      api('GET', '/nodes').then(function(res) {
        clientNodes.value = (res && res.items) ? res.items : (Array.isArray(res) ? res : [])
      }).catch(function() {})
    }

    // Build a link lookup from clientNodes: map inbound_id/external_id -> link
    // /plans returns inbound_config.id and external_node.id as node.id,
    // while /nodes returns MemberID as id and inbound_config.id as inbound_id.
    // So we must key by inbound_id and external_id to match.
    var nodeLinkMap = computed(function() {
      var map = {}
      clientNodes.value.forEach(function(n) {
        if (n.link) {
          if (n.inbound_id) map['ib_' + n.inbound_id] = n.link
          if (n.external_id) map['ext_' + n.external_id] = n.link
          // Also key by name as fallback
          if (n.name) map['name_' + n.name] = n.link
        }
      })
      return map
    })

    // Determine if user's subscription is active (not expired/suspended)
    var userPlanActive = computed(function() {
      var c = client.value
      if (!c || !c.plan_id) return false
      if (c.status === 'suspended' || c.status === 'expired') return false
      return true
    })

    // All plan nodes grouped by plan — shows all plans with their nodes,
    // only the user's active plan shows detail/QR buttons
    var allPlanNodes = computed(function() {
      var userPlanId = client.value && client.value.plan_id
      var active = userPlanActive.value
      var linkMap = nodeLinkMap.value
      var result = []
      plans.value.forEach(function(plan) {
        var isUserPlan = (plan.id === userPlanId)
        var planNodes = []
        if (plan.nodes && Array.isArray(plan.nodes)) {
          plan.nodes.forEach(function(node) {
            // Build key matching nodeLinkMap format: ib_<id> or ext_<id>
            var prefix = node.type === 'external' ? 'ext_' : 'ib_'
            var nodeKey = prefix + node.id
            var nameKey = 'name_' + (node.name || '')
            var link = null
            if (isUserPlan && active) {
              link = linkMap[nodeKey] || linkMap[nameKey] || null
            }
            planNodes.push({
              id: node.id,
              name: node.name || (t('node.hash_prefix') + (node.id || '?')),
              protocol: node.protocol || '',
              transport: node.transport || '',
              region: node.region || '',
              link: link,
              hasAccess: isUserPlan && active
            })
          })
        }
        result.push({
          plan_id: plan.id,
          plan_name: plan.name,
          isUserPlan: isUserPlan,
          active: isUserPlan && active,
          nodes: planNodes
        })
      })
      return result
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

    var saveActiveTab = function(tab) {
      try { localStorage.setItem('portal_active_tab', tab) } catch(e) {}
    }
    // Fetch data for a specific page
    var fetchPageData = function(pageTarget) {
      // Auto-fetch log packages if the page has log_packages modules
      var hasLogMods = portalModules.value.some(function(m) {
        return (m.page || 'overview') === pageTarget && m.type === 'log_packages'
      })
      if (hasLogMods) fetchLogPackages()
      // Auto-fetch commercial module data based on page
      var pageMods = portalModules.value.filter(function(m) { return (m.page || 'overview') === pageTarget })
      var hasPlanList = pageMods.some(function(m) { return m.type === 'plan_list' || m.type === 'order_create' })
      var hasCharge = pageMods.some(function(m) { return m.type === 'charge_panel' })
      var hasBalance = pageMods.some(function(m) { return m.type === 'balance_display' || m.type === 'account_info' })
      var hasReferral = pageMods.some(function(m) { return m.type === 'referral' })
      var hasTickets = pageMods.some(function(m) { return m.type === 'ticket_list' })
      var hasArticles = pageMods.some(function(m) { return m.type === 'article_list' })
      var hasTutorialCards = pageMods.some(function(m) { return m.type === 'tutorial_cards' })
      var hasClientDownload = pageMods.some(function(m) { return m.type === 'client_download' })
      var hasSharedAccts = pageMods.some(function(m) { return m.type === 'shared_accounts' })
      var hasStreaming = pageMods.some(function(m) { return m.type === 'streaming_status' })
      var hasOrderList = pageMods.some(function(m) { return m.type === 'order_list' })
      var hasNodeList = pageMods.some(function(m) { return m.type === 'node_list' })
      if (hasPlanList || hasNodeList) fetchPlans()
      if (hasNodeList) fetchNodes()
      if (hasPlanList || hasCharge) fetchChargeSites()
      if (hasCharge) fetchChargeOrders()
      if (hasBalance) { fetchBalance(); fetchBalanceTxns() }
      if (hasReferral) fetchReferral()
      if (hasTickets) fetchTickets()
      if (hasArticles || hasTutorialCards || hasClientDownload) fetchArticles()
      if (hasSharedAccts || hasStreaming) fetchSharedAccounts()
      if (hasOrderList) { fetchPlans(); fetchMyOrders(); fetchChargeOrders(); fetchBalanceTxns() }
    }
    var onNavClick = function(nav) {
      if (nav.href) {
        if (nav.open_external) window.open(nav.href, '_blank')
        else window.location.href = nav.href
        return
      }
      activeTab.value = nav.target
      saveActiveTab(nav.target)
      fetchPageData(nav.target)
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
        ElMessage.success(t('auth.login_success'))
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
        ElMessage.success(t('auth.register_success'))
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
        ElMessage.success(t('sub.reset_success'))
      }).catch(function(e) { ElMessage.error(e.message) }).finally(function() { rotating.value = false })
    }

    var saveSettings = function() {
      api('PUT', '/settings', settings.value).then(function() {
        ElMessage.success(t('common.saved'))
      }).catch(function(e) { ElMessage.error(e.message) })
    }

    var changePassword = function() {
      if (!pwdForm.value.old_password || !pwdForm.value.new_password) { ElMessage.error(t('common.fill_required')); return }
      if (pwdForm.value.new_password.length < 6) { ElMessage.error(t('security.password_min_length')); return }
      if (pwdForm.value.new_password !== pwdForm.value.confirm_password) { ElMessage.error(t('security.password_mismatch')); return }
      api('POST', '/change-password', { old_password: pwdForm.value.old_password, new_password: pwdForm.value.new_password }).then(function() {
        ElMessage.success(t('security.password_changed'))
        pwdForm.value = { old_password: '', new_password: '', confirm_password: '' }
      }).catch(function(e) { ElMessage.error(e.message) })
    }

    var handleLogout = function() { doLogout() }

    var copyText = function(text) {
      if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(text).then(function() { ElMessage.success(t('common.copied')) }).catch(function() { fallbackCopy(text) })
      } else { fallbackCopy(text) }
    }
    var fallbackCopy = function(text) {
      var ta = document.createElement('textarea')
      ta.value = text; ta.style.position = 'fixed'; ta.style.left = '-9999px'
      document.body.appendChild(ta); ta.select(); document.execCommand('copy')
      document.body.removeChild(ta); ElMessage.success(t('common.copied'))
    }

    var formatBytes = function(b) {
      if (!b || b <= 0) return '0 B'
      if (b >= 1073741824) return (b / 1073741824).toFixed(1) + ' GB'
      if (b >= 1048576) return (b / 1048576).toFixed(0) + ' MB'
      return (b / 1024).toFixed(0) + ' KB'
    }
    var formatDate = function(d) { if (!d) return '-'; return new Date(d).toLocaleString('zh-CN') }
var within30Days = function(d) { if (!d) return false; var t = new Date(d).getTime(); if (isNaN(t)) return false; return t >= Date.now() - 30 * 24 * 60 * 60 * 1000 }
var formatDateTime = function(d) { if (!d) return '-'; var date = new Date(d); return date.getFullYear() + '-' + String(date.getMonth()+1).padStart(2,'0') + '-' + String(date.getDate()).padStart(2,'0') + ' ' + String(date.getHours()).padStart(2,'0') + ':' + String(date.getMinutes()).padStart(2,'0') + ':' + String(date.getSeconds()).padStart(2,'0') }

    var usagePercent = computed(function() { return client.value.quota_bytes > 0 ? Math.min(100, Math.round(client.value.used_bytes / client.value.quota_bytes * 100)) : 0 })
    var usageColor = computed(function() { return usagePercent.value > 90 ? '#ef4444' : usagePercent.value > 70 ? '#f59e0b' : '#2563eb' })
    var ringDash = computed(function() {
      var circumference = 2 * Math.PI * 52
      var fill = circumference * usagePercent.value / 100
      return fill + ' ' + (circumference - fill)
    })
var statusText = computed(function() { return { active: t('account.normal'), suspended: t('account.suspended'), expired: t('common.expired') }[client.value.status] || client.value.status || '-' })
var subStatusText = computed(function() { return { active: t('account.normal'), revoked: t('sub.status_revoked'), expired: t('common.expired'), suspended: t('account.suspended') }[client.value.sub_status] || client.value.sub_status || '-' })
    // Streaming accounts from shared accounts API
    var appleAccount = computed(function() {
      return sharedAccounts.value.find(function(sa) { return sa.type === 'apple_id' && sa.enabled !== false })
    })
    var netflixAccount = computed(function() {
      return sharedAccounts.value.find(function(sa) { return sa.type === 'netflix' && sa.enabled !== false })
    })

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
var showQrDialog = function() { if (!qrCodeUrl.value) generateQr(); showSubQrDialog.value = true }

// --- More Subscriptions Dialog ---
var moreSubsVisible = ref(false)
var clientConfigs = ref([])
var moreSubQrKey = ref('')
var moreSubQrConfig = ref('')
var moreSubQrUrl = ref('')
var moreSubQrDialogVisible = ref(false)
var moreSubQrDialogUrl = ref('')

var fetchClientConfigs = function() {
  var pid = client.value && client.value.plan_id ? client.value.plan_id : ''
  var url = '/api/v1/public/client-configs'
  if (pid) url += '?plan_id=' + pid
  fetch(url).then(function(r) { return r.json() }).then(function(d) {
    clientConfigs.value = Array.isArray(d.data) ? d.data : []
  }).catch(function() { clientConfigs.value = [] })
}

var buildSubLink = function(formatKey, configName, platform) {
  var base = subUrl.value
  if (!base) return ''
  var sep = base.indexOf('?') >= 0 ? '&' : '?'
  var link = base + sep + 'format=' + encodeURIComponent(formatKey)
  if (configName) link += '&config=' + encodeURIComponent(configName)
  if (platform) link += '&platform=' + encodeURIComponent(platform)
  return link
}

var moreSubFormats = computed(function() {
  // Separate MihomoConfigs and SubscriptionFormats
  var mihomoCfgs = {}  // keyed by format
  var subFmtCfgs = {}  // keyed by format
  clientConfigs.value.forEach(function(c) {
    var fmt = c.format || 'mihomo_yaml'
    var entry = {
      id: c.id,
      name: c.name,
      description: c.description || '',
      platform: c.platform || '',
      type: c.type || 'config',
      client_type: c.client_type || '',
      sort_order: c.sort_order || 0
    }
    if (c.type === 'config') {
      if (!mihomoCfgs[fmt]) mihomoCfgs[fmt] = []
      mihomoCfgs[fmt].push(entry)
    } else {
      if (!subFmtCfgs[fmt]) subFmtCfgs[fmt] = []
      subFmtCfgs[fmt].push(entry)
    }
  })

  // Build format groups with icons
  var allFormats = [
    { key: 'base64_uri', label: 'Base64', desc: t('sub.fmt_base64'), icon: '📋' },
    { key: 'raw_uri', label: 'Raw URI', desc: t('sub.fmt_raw'), icon: '🔗' },
    { key: 'mihomo_yaml', label: 'Clash / Mihomo', desc: t('sub.fmt_clash'), icon: '🐱' },
    { key: 'singbox_json', label: 'Sing-box', desc: t('sub.fmt_singbox'), icon: '📦' },
    { key: 'xray_json', label: 'Xray', desc: t('sub.fmt_xray'), icon: '🚀' }
  ]
  return allFormats.map(function(f) {
    var item = { key: f.key, label: f.label, desc: f.desc, icon: f.icon, configs: [] }
    // Priority: MihomoConfig > SubscriptionFormat (deduplicate by name)
    var configs = mihomoCfgs[f.key] || []
    var seenNames = {}
    configs.forEach(function(c) { seenNames[c.name] = true })
    // Add SubscriptionFormats only if no MihomoConfig with same name exists
    var subFmts = subFmtCfgs[f.key] || []
    subFmts.forEach(function(c) {
      if (!seenNames[c.name]) configs.push(c)
    })
    // Sort by sort_order then name
    if (configs.length > 0) {
      item.configs = configs.slice().sort(function(a, b) { return a.sort_order - b.sort_order || a.name.localeCompare(b.name) })
    }
    return item
  }).filter(function(f) { return f.configs.length > 0 || ['base64_uri', 'raw_uri'].indexOf(f.key) >= 0 })
})

var generateMoreSubQr = function(url) {
  try {
    var canvas = document.createElement('canvas')
    new QRious({ element: canvas, value: url, size: 200, background: isDark.value ? '#1a1a2e' : '#ffffff', foreground: isDark.value ? '#e0e0f0' : '#000000', level: 'M' })
    return canvas.toDataURL('image/png')
  } catch(e) { return '' }
}

var toggleMoreSubQr = function(formatKey, configName, platform) {
if (moreSubQrKey.value === formatKey && moreSubQrConfig.value === configName) {
moreSubQrKey.value = ''
moreSubQrConfig.value = ''
moreSubQrUrl.value = ''
return
}
moreSubQrKey.value = formatKey
moreSubQrConfig.value = configName
var url = buildSubLink(formatKey, configName, platform)
moreSubQrUrl.value = url ? generateMoreSubQr(url) : ''
}
var showMoreSubQrDialog = function(formatKey, configName, platform) {
var url = buildSubLink(formatKey, configName, platform)
moreSubQrDialogUrl.value = url ? generateMoreSubQr(url) : ''
moreSubQrDialogVisible.value = true
}
// --- End More Subscriptions ---
var handleSubAction = function(action) {
  if (action.action === 'navigate') {
    activeTab.value = action.target
    try { localStorage.setItem('portal_active_tab', action.target) } catch(e) {}
  } else if (action.action === 'dialog') {
    // handle dialog actions
    if (action.target === 'whitelist') {
      // navigate to security settings
      activeTab.value = 'profile'
      try { localStorage.setItem('portal_active_tab', 'profile') } catch(e) {}
      ElMessage.info(t('security.ip_whitelist_manage'))
    } else if (action.target === 'more_subs') {
      moreSubsVisible.value = true
      fetchClientConfigs()
    }
  } else if (action.action === 'confirm') {
    // handle confirm
    if (action.target === 'reset_sub') {
      // rotate token
      ElMessageBox.confirm(t('sub.reset_confirm'), t('common.confirm'), {
        confirmButtonText: t('sub.reset_confirm_btn'),
        cancelButtonText: t('common.cancel'),
        type: 'warning',
        confirmButtonClass: 'el-button--danger'
      }).then(function() {
        rotateToken()
      }).catch(function() {})
    }
  }
}
var showStreamingAccount = function(service) {
  ElMessage.info(service.name + ' ' + t('shared.coming_soon'))
}

// Log packages
    var fetchLogPackages = function() {
      api('GET', '/logs/packages?limit=10').then(function(res) {
        logPackages.value = res.items || []
      }).catch(function() {})
    }

    var createLogPackage = function(hours) {
      logPacking.value = true
      api('POST', '/logs/package', { hours: hours }).then(function(res) {
        ElMessage.success(t('log.request_sent'))
        fetchLogPackages()
      }).catch(function(e) { ElMessage.error(e.message) }).finally(function() { logPacking.value = false })
    }

    var downloadLogPackage = function(id) {
      window.open('/api/v1/client-auth/logs/packages/' + id + '/download?token=' + encodeURIComponent(token.value), '_blank')
    }

    var pkgStatusText = function(s) { return { pending: t('common.processing'), processing: t('common.processing'), done: t('order.status_completed'), failed: t('common.failed') }[s] || s || '-' }

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
    // v3.3: show payment method dialog
    var purchaseVariant = function(variantId, plan, modData) {
      var orderType = (client.value.plan_id > 0 && client.value.plan_id === plan.id) ? 'renew' : 'new'
      // find selected variant for dialog
      var variant = null
      if (plan && plan.variants) {
        for (var i = 0; i < plan.variants.length; i++) {
          if (plan.variants[i].id === variantId) { variant = plan.variants[i]; break }
        }
      }
      // set order form defaults
      orderForm.value = {
        variant_id: variantId,
        order_type: orderType,
        promotion_code: '',
        pay_method: 'balance', // default: balance payment
        gift_card_code: '',
        charge_site_id: null,
        // dialog display info
        plan_name: plan ? plan.name : '',
        variant_name: variant ? (variant.name || '') : '',
        price: variant ? variant.price : 0,       // cents
        currency: variant ? (variant.currency || 'CNY') : 'CNY',
        duration_days: variant ? (variant.duration_days || 0) : 0,
        quota_bytes: variant ? (variant.quota_bytes || 0) : 0
      }
      showPromoInput.value = false
      // ensure balance and charge sites are fresh
      fetchBalance()
      if (chargeSites.value.length === 0) fetchChargeSites()
      // show payment dialog
      showPaymentDialog.value = true
    }

    // get plan name
    var getPlanName = function(planId) {
      if (!planId) return t('plan.not_selected')
      var p = plans.value.find(function(x) { return x.id === planId })
      return p ? p.name : t('plan.hash_prefix') + planId
    }
    // get plan price
    var getPlanPrice = function(planId) {
      if (!planId) return 0
      var p = plans.value.find(function(x) { return x.id === planId })
      if (!p) return 0
      if (p.variants && p.variants.length > 0) {
        var minPrice = Infinity
        p.variants.forEach(function(v) {
          if (v.price < minPrice) minPrice = v.price
        })
        return minPrice === Infinity ? 0 : minPrice / 100
      }
      return (p.price || 0) / 100
    }

    // v3.3: confirm payment
    var confirmPayment = function() {
      if (!orderForm.value.pay_method) {
        ElMessage.error(t('plan.select_payment'))
        return
      }
      if (orderForm.value.pay_method === 'gift_card' && !orderForm.value.gift_card_code) {
        ElMessage.error(t('plan.card_code_placeholder'))
        return
      }
      if (orderForm.value.pay_method === 'charge_site' && !orderForm.value.charge_site_id) {
        ElMessage.error(t('plan.select_channel_placeholder'))
        return
      }

      orderBusy.value = true
      var payload = {
        variant_id: orderForm.value.variant_id,
        order_type: orderForm.value.order_type,
        promotion_code: orderForm.value.promotion_code,
        pay_method: orderForm.value.pay_method
      }
      if (orderForm.value.pay_method === 'charge_site' && orderForm.value.charge_site_id) {
        payload.charge_site_id = orderForm.value.charge_site_id
      }
      if (orderForm.value.pay_method === 'gift_card' && orderForm.value.gift_card_code) {
        payload.gift_card_code = orderForm.value.gift_card_code
      }

      api('POST', '/orders', payload).then(function(res) {
        showPaymentDialog.value = false
        if (res.status === 'completed') {
          ElMessage.success(t('plan.buy_success'))
          fetchMe(); fetchBalance(); fetchBalanceTxns()
        } else if (res.payment_url) {
          ElMessage.info(t('plan.redirecting_payment'))
          window.open(res.payment_url, '_blank')
        } else {
          ElMessage.success(t('plan.order_created'))
        }
      }).catch(function(e) {
        // v3.3: handle insufficient balance
        if (e.code === 'INSUFFICIENT_BALANCE' || (e.message && e.message.includes('INSUFFICIENT_BALANCE'))) {
          ElMessage.warning(t('plan.balance_insufficient_alt'))
        } else {
          ElMessage.error(e.message || t('plan.order_create_failed'))
          showPaymentDialog.value = false
        }
      }).finally(function() { orderBusy.value = false })
    }

    // --- Submit order form ---
    var submitOrder = function() {
      if (!orderForm.value.variant_id) { ElMessage.error(t('plan.select_placeholder')); return }
      if (orderForm.value.pay_method === 'gift_card' && !orderForm.value.gift_card_code) { ElMessage.error(t('plan.card_code_placeholder')); return }
      orderBusy.value = true
      var payload = {
        variant_id: orderForm.value.variant_id, order_type: orderForm.value.order_type,
        promotion_code: orderForm.value.promotion_code, pay_method: orderForm.value.pay_method
      }
      if (orderForm.value.pay_method === 'charge_site' && orderForm.value.charge_site_id) {
        payload.charge_site_id = orderForm.value.charge_site_id
      }
      if (orderForm.value.pay_method === 'gift_card' && orderForm.value.gift_card_code) {
        payload.gift_card_code = orderForm.value.gift_card_code
      }
      api('POST', '/orders', payload).then(function(res) {
        if (res.status === 'completed') {
          ElMessage.success(t('order.completed'))
          fetchMe(); fetchBalance(); fetchBalanceTxns()
        } else if (res.payment_url) {
          ElMessage.info(t('plan.redirecting_payment'))
          window.open(res.payment_url, '_blank')
        } else {
          ElMessage.success(t('plan.order_created'))
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
      if (!chargeForm.value.site_id) { ElMessage.error(t('charge.select_site')); return }
      chargeBusy.value = true
      var amountCents = Math.round(chargeForm.value.amount_yuan * 100)
      api('POST', '/charge/create', { site_id: chargeForm.value.site_id, amount: amountCents }).then(function(res) {
        if (res.payment_url) {
          ElMessage.info(t('plan.redirecting_payment'))
          window.open(res.payment_url, '_blank')
        } else {
          ElMessage.success(t('charge.order_created'))
        }
        fetchChargeOrders()
      }).catch(function(e) { ElMessage.error(e.message) }).finally(function() { chargeBusy.value = false })
    }

    // --- Redeem charge card ---
    var redeemCard = function() {
      if (!cardCode.value) { ElMessage.error(t('charge.enter_card_code')); return }
      redeemBusy.value = true
      api('POST', '/charge/redeem-card', { card_code: cardCode.value }).then(function(res) {
        ElMessage.success(t('charge.card_success'))
        cardCode.value = ''
        fetchBalance(); fetchBalanceTxns()
        fetchChargeOrders()
      }).catch(function(e) { ElMessage.error(e.message) }).finally(function() { redeemBusy.value = false })
    }

    // --- Charge orders history ---
    var fetchChargeOrders = function() {
api('GET', '/charge/orders?limit=50').then(function(res) {
chargeOrders.value = ((res && res.items) ? res.items : []).filter(function(co) { return within30Days(co.created_at) })
}).catch(function() {})
    }

    var chargeStatusText = function(s) { return { pending: t('order.status_pending'), paid: t('order.status_paid'), processing: t('common.processing'), expired: t('common.expired'), failed: t('common.failed') }[s] || s || '-' }

    // --- My Orders ---
    var fetchMyOrders = function() {
      myOrdersLoading.value = true
api('GET', '/orders?limit=50').then(function(res) {
myOrders.value = ((res && res.items) ? res.items : (Array.isArray(res) ? res : [])).filter(function(o) { return within30Days(o.created_at) })
}).catch(function() {}).finally(function() { myOrdersLoading.value = false })
    }
    var orderStatusText = function(s) {
      return { pending: t('order.status_pending'), paid: t('order.status_paid'), completed: t('order.status_completed'), cancelled: t('order.status_cancelled'), refunded: t('order.status_refunded') }[s] || s || '-'
    }
    var orderStatusBadge = function(s) {
      return { pending: 'processing', paid: 'active', completed: 'active', cancelled: 'none', refunded: 'expired' }[s] || 'none'
    }
    // Resolve service name from plan cache by to_variant_id -> variant -> plan name
    var getOrderServiceName = function(order) {
      if (!order.to_variant_id) return { new: t('order.type_new'), renew: t('common.renew'), upgrade: t('common.upgrade'), downgrade: t('order.type_downgrade') }[order.order_type] || order.order_type || '-'
      for (var i = 0; i < plans.value.length; i++) {
        var p = plans.value[i]
        if (p.variants) {
          for (var j = 0; j < p.variants.length; j++) {
            if (p.variants[j].id === order.to_variant_id) return p.name
          }
        }
      }
      return { new: t('order.type_new'), renew: t('common.renew'), upgrade: t('common.upgrade'), downgrade: t('order.type_downgrade') }[order.order_type] || order.order_type || '-'
    }

    // --- Balance ---
    var fetchBalance = function() {
      api('GET', '/balance').then(function(res) {
        balance.value = res || { balance: 0, frozen: 0 }
      }).catch(function() {})
    }
    var fetchBalanceTxns = function() {
api('GET', '/balance/transactions?limit=50').then(function(res) {
balanceTxns.value = ((res && res.items) ? res.items : []).filter(function(tx) { return within30Days(tx.created_at) })
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
    var referralStatusText = function(s) { return { pending: t('referral.status_pending'), available: t('referral.status_available'), withdrawn: t('referral.status_withdrawn') }[s] || s || '-' }

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
      if (!newTicketForm.value.subject || !newTicketForm.value.content) { ElMessage.error(t('ticket.fill_subject_content')); return }
      ticketBusy.value = true
      api('POST', '/tickets', newTicketForm.value).then(function(res) {
        ElMessage.success(t('ticket.created'))
        showTicketDialog.value = false
        newTicketForm.value = { subject: '', category: 'other', priority: 'normal', content: '' }
        fetchTickets()
      }).catch(function(e) { ElMessage.error(e.message) }).finally(function() { ticketBusy.value = false })
    }
    var replyTicket = function() {
      if (!ticketReplyContent.value) { ElMessage.error(t('ticket.reply_placeholder')); return }
      ticketBusy.value = true
      var tid = ticketDetail.value.ticket.id
      api('POST', '/tickets/' + tid + '/reply', { content: ticketReplyContent.value }).then(function() {
        ElMessage.success(t('ticket.reply_success'))
        ticketReplyContent.value = ''
        // reload messages, keep dialog open
        api('GET', '/tickets/' + tid).then(function(res) {
          ticketDetail.value = res
        })
      }).catch(function(e) { ElMessage.error(e.message) }).finally(function() { ticketBusy.value = false })
    }
    var closeTicket = function() {
      if (!ticketDetail.value) return
      ticketBusy.value = true
      var tid = ticketDetail.value.ticket.id
      api('POST', '/tickets/' + tid + '/close').then(function() {
        ElMessage.success(t('ticket.closed'))
        showTicketDetail.value = false
        fetchTickets()
      }).catch(function(e) { ElMessage.error(e.message) }).finally(function() { ticketBusy.value = false })
    }
    var ticketStatusText = function(s) { return { open: t('ticket.status_open'), replied: t('ticket.status_replied'), closed: t('ticket.status_closed') }[s] || s || '-' }
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
    var articleCatText = function(c) { return { tutorial: t('article.tutorial'), faq: 'FAQ', notice: t('article.notice'), general: t('article.general') }[c] || c || '-' }

    // --- Tutorial cards helpers ---
    var getTutorialIcon = function(icon) {
      var icons = { windows: '🪟', android: '🤖', apple: '🍎', desktop: '💻', linux: '🐧', question: '❓', sync: '🔄' }
      return icons[icon] || '📄'
    }
    var openTutorial = function(card) {
      // Open article by slug or external link
      if (card.article_slug) {
        openArticle(card.article_slug)
      } else if (card.link) {
        window.open(card.link, '_blank')
      } else {
        ElMessage.info(t('tutorial.coming_soon'))
      }
    }
    // Platform selection for tutorial cards
    var selectPlatform = function(platformId) {
      selectedPlatform.value = platformId
      // Fetch articles for this platform if not already loaded
      fetchArticlesByPlatform(platformId)
    }
    var getSelectedPlatformName = function() {
      // Find platform name from tutorial_cards or client_download module
      var tutorialMod = currentModules.value.find(function(m) { return m.type === 'tutorial_cards' })
      var cards = (tutorialMod && tutorialMod.data && tutorialMod.data.cards) || []
      var card = cards.find(function(c) { return c.id === selectedPlatform.value })
      if (card) return card.name
      // Fallback: search client_download module platforms
      var dlMod = currentModules.value.find(function(m) { return m.type === 'client_download' })
      var platforms = (dlMod && dlMod.data && dlMod.data.platforms) || []
      var p = platforms.find(function(pl) { return pl.id === selectedPlatform.value })
      return p ? p.name : selectedPlatform.value
    }
    var getSelectedPlatformUrl = function(mod) {
      if (!mod || !mod.data || !mod.data.platforms) return ''
      var p = mod.data.platforms.find(function(pl) { return pl.id === selectedPlatform.value })
      return (p && p.url) || ''
    }
    var navigateTo = function(target) {
      var nav = portalNavItems.value.find(function(n) { return n.target === target })
      if (nav) onNavClick(nav)
    }
    var getPlatformArticles = function(platformId) {
      // Special handling for FAQ card
      if (platformId === 'faq') {
        return articles.value.filter(function(a) {
          return a.category === 'faq' && a.is_published
        }).sort(function(a, b) { return (a.sort_order || 0) - (b.sort_order || 0) })
      }
      // Map platform id to article platform field
      var platformMap = {
        windows: 'windows',
        android: 'android',
        apple: 'ios',
        mac: 'mac',
        linux: 'linux',
        game: 'game',
        other: 'other'
      }
      var articlePlatform = platformMap[platformId] || platformId
      return articles.value.filter(function(a) {
        return a.platform === articlePlatform && a.category === 'tutorial' && a.is_published
      }).sort(function(a, b) { return (a.sort_order || 0) - (b.sort_order || 0) })
    }
    var fetchArticlesByPlatform = function(platformId) {
      // Articles are already loaded by fetchArticles, just filtered in getPlatformArticles
      // If articles not loaded yet, fetch them
      if (articles.value.length === 0) {
        fetchArticles()
      }
    }

    // --- Announcements (notice-category articles for top bar bell) ---
    var fetchAnnouncements = function() {
      api('GET', '/articles?category=notice').then(function(res) {
        announcements.value = Array.isArray(res) ? res : []
        // check auto popup after data load
        checkAutoAnnouncementPopup()
      }).catch(function() {})
    }

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
        ElMessage.success(t('shared.repair_sent'))
      }).catch(function(e) { ElMessage.error(e.message) })
    }
    var sharedAcctTypeText = function(t) { return { apple_id: 'Apple ID', netflix: 'Netflix', other: t('common.other') }[t] || t || '-' }

    // --- Scroll to a module by ID ---
    var scrollToModule = function(modId) {
      setTimeout(function() {
        var el = document.getElementById('mod-' + modId)
        if (el) el.scrollIntoView({ behavior: 'smooth', block: 'center' })
      }, 100)
    }

    var switchToLogs = function() { activeTab.value = 'logs'; fetchLogPackages() }

    // Node QR Code dialog
    var showNodeQRDialog = ref(false)
    var currentNodeQR = ref('')
    var nodeQRContainer = ref(null)
    var showNodeQR = function(node) {
      if (!node.link) return
      currentNodeQR.value = node.link
      showNodeQRDialog.value = true
      // Generate QR code after dialog opens
      nextTick(function() {
        generateNodeQR(node.link)
      })
    }

    // Node detail dialog (shows share link + copy)
    var showNodeDetailDialog = ref(false)
    var currentNodeDetail = ref({})
    var showNodeDetail = function(node) {
      currentNodeDetail.value = node
      showNodeDetailDialog.value = true
    }
    var generateNodeQR = function(text) {
      var container = nodeQRContainer.value
      if (!container || typeof QRious === 'undefined') return
      container.innerHTML = ''
      var canvas = document.createElement('canvas')
      new QRious({ element: canvas, value: text, size: 200, background: isDark.value ? '#1a1a2e' : '#ffffff', foreground: isDark.value ? '#e0e0f0' : '#000000', level: 'M' })
      container.appendChild(canvas)
    }
    var nextTick = function(fn) {
      if (typeof Vue !== 'undefined' && Vue.nextTick) {
        Vue.nextTick(fn)
      } else {
        setTimeout(fn, 0)
      }
    }

    // Auto-refresh every 30s
    var refreshTimer = null
    var startAutoRefresh = function() {
      if (refreshTimer) clearInterval(refreshTimer)
      refreshTimer = setInterval(function() { if (token.value) fetchMe() }, 30000)
    }

    // Public settings
    var fetchPublicSettings = function() {
      fetch('/api/v1/public/settings').then(function(r) { return r.json() }).then(function(d) {
        var data = d.data || d
        if (data && data.registration_enabled !== undefined) registrationEnabled.value = data.registration_enabled
        // v3.4: hot-reload portal modules/nav
        if (data && data.portal_config) {
          var cfg = data.portal_config
          if (Array.isArray(cfg.modules)) {
            portalModules.value = cfg.modules.filter(function(m) { return m.visible !== false })
          }
          if (Array.isArray(cfg.navigation)) {
            portalNavItems.value = cfg.navigation.filter(function(n) { return n.enabled !== false })
          }
          // reload page data after module update
          fetchPageData(activeTab.value)
        }
      }).catch(function() {})
    }

    // --- Announcement popup logic ---
    var openAnnouncementPopup = function(ann) {
      activeAnnouncement.value = ann
      announcementPopupVisible.value = true
    }

    // auto-popup announcements (once per day)
    var checkAutoAnnouncementPopup = function() {
      if (!token.value || !announcements.value || announcements.value.length === 0) return
      var today = new Date().toDateString()
      var popupKey = 'ann_popup_seen_' + today
      // already popped today
      if (localStorage.getItem(popupKey)) return
      // find first popup announcement
      for (var i = 0; i < announcements.value.length; i++) {
        if (announcements.value[i].is_popup) {
          openAnnouncementPopup(announcements.value[i])
          localStorage.setItem(popupKey, '1')
          break
        }
      }
    }

    onMounted(function() {
      fetchPublicSettings()
      // Check for referral code in URL
      var urlParams = new URLSearchParams(window.location.search)
      var refCode = urlParams.get('ref')
      if (refCode) {
        registerForm.value.referral_code = refCode
        authTab.value = 'register'
      }
      if (token.value) {
        fetchMe(); fetchEndpoints(); startAutoRefresh()
        fetchAnnouncements()
        setTimeout(checkAutoAnnouncementPopup, 800)
        // Auto-load commercial module data for the default page
        var initMods = portalModules.value.filter(function(m) { return (m.page || 'overview') === activeTab.value })
        var initHasCharge = initMods.some(function(m) { return m.type === 'charge_panel' })
        var initHasBalance = initMods.some(function(m) { return m.type === 'balance_display' || m.type === 'account_info' })
        var initHasPlanList = initMods.some(function(m) { return m.type === 'plan_list' || m.type === 'order_create' })
        var initHasReferral = initMods.some(function(m) { return m.type === 'referral' })
        var initHasTickets = initMods.some(function(m) { return m.type === 'ticket_list' })
        var initHasArticles = initMods.some(function(m) { return m.type === 'article_list' || m.type === 'tutorial_cards' || m.type === 'client_download' })
        var initHasSharedAccts = initMods.some(function(m) { return m.type === 'shared_accounts' })
        var initHasOrderList = initMods.some(function(m) { return m.type === 'order_list' })
        var initHasNodeList = initMods.some(function(m) { return m.type === 'node_list' })
        if (initHasPlanList || initHasNodeList) fetchPlans()
        if (initHasNodeList) fetchNodes()
        if (initHasPlanList || initHasCharge) fetchChargeSites()
        if (initHasCharge) fetchChargeOrders()
        if (initHasBalance) { fetchBalance(); fetchBalanceTxns() }
        if (initHasReferral) fetchReferral()
        if (initHasTickets) fetchTickets()
        if (initHasArticles) fetchArticles()
        if (initHasSharedAccts) fetchSharedAccounts()
        if (initHasOrderList) { fetchPlans(); fetchMyOrders(); fetchChargeOrders(); fetchBalanceTxns() }
      }
    })

    // Check for popups when switching pages
    watch(activeTab, function() {
      if (token.value) setTimeout(checkAutoAnnouncementPopup, 300)
    })

    // Check for popups after login
    watch(token, function(v) {
      if (v) { setTimeout(checkAutoAnnouncementPopup, 800); fetchAnnouncements() }
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
          activeTab: activeTab, showPasswordDialog: showPasswordDialog, showQr: showQr, showSubQrDialog: showSubQrDialog, showSubUrl: showSubUrl, selectedPlatform: selectedPlatform, logoUrl: logoUrl,
      isDark: isDark, registrationEnabled: registrationEnabled,
      loginForm: loginForm, registerForm: registerForm, pwdForm: pwdForm,
      settings: settings, client: client, endpoints: endpoints,
      logPackages: logPackages, logPacking: logPacking,
      qrCodeUrl: qrCodeUrl,
      handleLogin: handleLogin, handleRegister: handleRegister, rotateToken: rotateToken,
      saveSettings: saveSettings, changePassword: changePassword, handleLogout: handleLogout,
      copyText: copyText, formatBytes: formatBytes, formatDate: formatDate, formatDateTime: formatDateTime, formatMoney: formatMoney,
      usagePercent: usagePercent, usageColor: usageColor, ringDash: ringDash,
      statusText: statusText, subStatusText: subStatusText, subUrl: subUrl,
      toggleQr: toggleQr, showQrDialog: showQrDialog, handleSubAction: handleSubAction, showStreamingAccount: showStreamingAccount, toggleTheme: toggleTheme,
      createLogPackage: createLogPackage, fetchLogPackages: fetchLogPackages,
      downloadLogPackage: downloadLogPackage, pkgStatusText: pkgStatusText,
      switchToLogs: switchToLogs, scrollToModule: scrollToModule,
      portalModules: portalModules, currentModules: currentModules, portalNavItems: portalNavItems, onNavClick: onNavClick, renderMd: renderMd, fetchPageData: fetchPageData, saveActiveTab: saveActiveTab,
      freeformComponents: freeformComponents,
      showNodeQRDialog: showNodeQRDialog, currentNodeQR: currentNodeQR, nodeQRContainer: nodeQRContainer, showNodeQR: showNodeQR,
      showNodeDetailDialog: showNodeDetailDialog, currentNodeDetail: currentNodeDetail, showNodeDetail: showNodeDetail,
      announcementPopupVisible: announcementPopupVisible, activeAnnouncement: activeAnnouncement,
      openAnnouncementPopup: openAnnouncementPopup,
      // Commercial modules
      plans: plans, plansLoading: plansLoading, orderForm: orderForm, orderBusy: orderBusy,
      chargeSites: chargeSites, chargeForm: chargeForm, chargeBusy: chargeBusy, chargeOrders: chargeOrders,
      cardCode: cardCode, redeemBusy: redeemBusy,
      balance: balance, balanceTxns: balanceTxns,
      myOrders: myOrders, myOrdersLoading: myOrdersLoading,
      allPlanNodes: allPlanNodes,
referralCode: referralCode, referralStats: referralStats, referralRecords: referralRecords,
              commissionRecords: commissionRecords, withdrawRecords: withdrawRecords,
              referralInviteUrl: referralInviteUrl, referralStatusText: referralStatusText,
      tickets: tickets, showTicketDialog: showTicketDialog, showTicketDetail: showTicketDetail,
      ticketDetail: ticketDetail, ticketReplyContent: ticketReplyContent, ticketBusy: ticketBusy,
      newTicketForm: newTicketForm,
      articles: articles, articleCategory: articleCategory,
      showArticleDetail: showArticleDetail, articleDetail: articleDetail,
      sharedAccounts: sharedAccounts, showCredentialsDialog: showCredentialsDialog, sharedAccountCreds: sharedAccountCreds,
      appleAccount: appleAccount, netflixAccount: netflixAccount,
      fetchPlans: fetchPlans, purchaseVariant: purchaseVariant, submitOrder: submitOrder,
      showChargeDialog: showChargeDialog, // charge dialog
      showPaymentDialog: showPaymentDialog, confirmPayment: confirmPayment, showPromoInput: showPromoInput, getPlanName: getPlanName, getPlanPrice: getPlanPrice, // v3.3: payment method dialog
      fetchChargeSites: fetchChargeSites, submitCharge: submitCharge, redeemCard: redeemCard,
      fetchChargeOrders: fetchChargeOrders, chargeStatusText: chargeStatusText,
      fetchMyOrders: fetchMyOrders, orderStatusText: orderStatusText, orderStatusBadge: orderStatusBadge, getOrderServiceName: getOrderServiceName,
      fetchBalance: fetchBalance, fetchBalanceTxns: fetchBalanceTxns, fetchReferral: fetchReferral,
      fetchTickets: fetchTickets, openTicket: openTicket, createTicket: createTicket,
      replyTicket: replyTicket, closeTicket: closeTicket,
      ticketStatusText: ticketStatusText, ticketStatusBadge: ticketStatusBadge,
      fetchArticles: fetchArticles, openArticle: openArticle, articleCatText: articleCatText,
      getTutorialIcon: getTutorialIcon, openTutorial: openTutorial,
      selectedPlatform: selectedPlatform, selectPlatform: selectPlatform,
      getSelectedPlatformName: getSelectedPlatformName, getPlatformArticles: getPlatformArticles,
      getSelectedPlatformUrl: getSelectedPlatformUrl, navigateTo: navigateTo,
      announcements: announcements, fetchAnnouncements: fetchAnnouncements,
      showNoticePanel: showNoticePanel,
      fetchSharedAccounts: fetchSharedAccounts, applySharedAccount: applySharedAccount,
      repairSharedAccount: repairSharedAccount, sharedAcctTypeText: sharedAcctTypeText,
      lang: lang, t: t,
      currentLocale: currentLocale, availableLocales: availableLocales, currentLocaleFlag: currentLocaleFlag, switchLocale: switchLocale,
      moreSubsVisible: moreSubsVisible, moreSubFormats: moreSubFormats, moreSubQrKey: moreSubQrKey, moreSubQrConfig: moreSubQrConfig, moreSubQrUrl: moreSubQrUrl, moreSubQrDialogVisible: moreSubQrDialogVisible, moreSubQrDialogUrl: moreSubQrDialogUrl, buildSubLink: buildSubLink, toggleMoreSubQr: toggleMoreSubQr, showMoreSubQrDialog: showMoreSubQrDialog
    }
  }
})

app.use(ElementPlus)
// register Element Plus icons
if (typeof ElementPlusIconsVue !== 'undefined') {
  for (var iconName in ElementPlusIconsVue) {
    app.component(iconName, ElementPlusIconsVue[iconName])
  }
}
app.mount('#app')`
