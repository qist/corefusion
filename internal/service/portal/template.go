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
            <div v-else class="logo-icon"><el-icon size="48" color="var(--accent)"><Lightning /></el-icon></div>
          </div>
          <h1 class="auth-title">{{ siteName }}</h1>
          <p class="auth-subtitle">{{ registrationEnabled ? t('auth.welcome_login') : t('auth.login_title') }}</p>
          <!-- 自助解封卡 ✓：流量超额被自动封禁的账号登录时返回 require_self_unban，
               凭登录签发的受限专用 token（仅可调 /self-unban）一键恢复，无需联系客服 -->
          <div class="self-unban-card" v-if="selfUnban.visible">
            <el-alert type="warning" :title="t('auth.self_unban_title')" :description="selfUnban.hint || t('auth.self_unban_desc')" show-icon :closable="false" style="margin-bottom:14px" />
            <el-button type="warning" class="btn-block" :loading="selfUnban.loading" @click="handleSelfUnban" size="large">{{ t('auth.self_unban_btn') }}</el-button>
            <el-button class="btn-block" text @click="selfUnban.visible = false">{{ t('auth.self_unban_cancel') }}</el-button>
          </div>
          <el-tabs v-if="!selfUnban.visible" v-model="authTab" class="auth-tabs">
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
          <!-- OAuth 登录（第三方登录 ✓：后台启用哪个 provider 就显示哪个 ✓） -->
          <div class="oauth-block" v-if="oauthProviders && oauthProviders.length > 0">
            <div class="oauth-divider">或</div>
            <div class="oauth-buttons">
              <button v-if="isOAuthEnabled('github')" class="oauth-btn" @click="oauthLogin('github')">
                <svg viewBox="0 0 24 24" fill="currentColor"><path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.525.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.655.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z"/></svg>
                GitHub
              </button>
              <button v-if="isOAuthEnabled('google')" class="oauth-btn" @click="oauthLogin('google')">
                <svg viewBox="0 0 24 24"><path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 01-2.2 3.32v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.1z" fill="#4285F4"/><path d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" fill="#34A853"/><path d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z" fill="#FBBC05"/><path d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" fill="#EA4335"/></svg>
                Google
              </button>
            </div>
          </div>
          <!-- 合规文档链接 ✓（点击打开条款内容/外链 ✓；门户设置里配置后显示 ✓） -->
          <div class="auth-legal" v-if="hasLegalLinks">
            <a v-if="privacyUrl || privacyContent" href="javascript:void(0)" @click="openLegal('privacy')">{{ t('legal.privacy') }}</a>
            <span v-if="(privacyUrl || privacyContent) && (termsUrl || termsContent)" class="auth-legal-sep">·</span>
            <a v-if="termsUrl || termsContent" href="javascript:void(0)" @click="openLegal('terms')">{{ t('legal.terms') }}</a>
          </div>
        </div>
      </div>
    </div>

    <!-- Main App (after login) -->
    <div v-else class="app-wrapper">
      <!-- Top Bar -->
      <header class="top-bar">
        <div class="top-bar-left">
          <img v-if="logoUrl" :src="logoUrl" class="top-logo" />
          <span class="top-title">{{ siteName }}</span>
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
            <el-icon v-if="isDark" size="20"><Sunny /></el-icon><el-icon v-else size="20"><Moon /></el-icon>
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
          <svg class="sidebar-icon" viewBox="0 0 1024 1024" width="18" height="18" fill="currentColor" fill-rule="evenodd"><path :d="navIconPath(nav)"></path></svg>
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
            <component :is="moduleComponents[mod.id]" v-if="moduleComponents[mod.id]"></component>
            <div v-else style="color:#f56c6c;padding:12px;font-size:13px">⚠️ {{ t('error.load_failed') }}</div>
          </div>
        </template>
        <!-- Empty state for overview main column -->
        <div v-if="currentModules.filter(function(m){ return m.type !== 'notice' }).length === 0" class="card glass">
          <div class="empty-state">
            <el-icon class="empty-icon" size="48"><Document /></el-icon>
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
        <!-- Notice modules: special handling for announcements -->
        <div v-if="mod.type === 'notice'" class="announcement-sidebar-card" style="margin-bottom: 16px;">
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
        <!-- All other modules: dynamic component from database -->
        <component :is="moduleComponents[mod.id]" v-else-if="moduleComponents[mod.id]"></component>
        <div v-else style="color:#f56c6c;padding:12px;font-size:13px">⚠️ {{ t('error.load_failed') }}</div>
      </div>
    </template>
    <!-- Empty state for non-overview pages -->
    <div v-if="activeTab !== 'overview' && currentModules.length === 0" class="card glass">
      <div class="empty-state">
        <el-icon class="empty-icon" size="48"><Document /></el-icon>
        <p class="empty-text">{{ t('common.empty') }}</p>
      </div>
    </div>
      </main>
      <nav class="bottom-nav">
        <div v-for="nav in portalNavItems" :key="nav.id" :class="'nav-item' + (activeTab === nav.target ? ' active' : '')" @click="onNavClick(nav)">
          <svg class="nav-icon" viewBox="0 0 1024 1024" width="20" height="20" fill="currentColor" fill-rule="evenodd"><path :d="navIconPath(nav)"></path></svg>
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

<!-- 合规文档弹窗（隐私政策 / 服务条款 ✓：登录卡片链接点击打开 ✓） -->
<el-dialog v-model="legalDialogVisible" :title="legalDialogTitle" width="560px" align-center append-to-body>
  <div class="md-render" style="max-height:60vh;overflow-y:auto" v-html="legalDialogHtml"></div>
</el-dialog>

<!-- Node QR Code Dialog -->
<el-dialog v-model="showNodeQRDialog" :title="t('node.qr_code')" width="440px" align-center>
<div style="text-align: center; padding: 20px;">
<div ref="nodeQRContainer" style="width: 360px; height: 360px; margin: 0 auto;"></div>
</div>
</el-dialog>

<!-- Subscription QR Code Dialog -->
<el-dialog v-model="showSubQrDialog" :title="t('sub.qr_code')" width="400px" align-center>
<div style="text-align: center; padding: 20px;">
<img v-if="qrCodeUrl" :src="qrCodeUrl" style="width: 300px; height: 300px; border-radius: 8px;" />
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
                        <div class="ticket-msg-content" v-html="renderMdUntrusted(msg.content)"></div>
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

    <!-- Shared Account List Dialog (multiple accounts) -->
    <el-dialog v-model="showAccountListDialog" :title="accountListTitle" width="460px" align-center append-to-body>
      <div class="account-pick-list">
        <div v-for="(acct, idx) in accountListItems" :key="acct.id" class="account-pick-item" @click="pickSharedAccount(acct.id)">
          <div class="account-pick-info">
            <span class="account-pick-num">#{{ idx + 1 }}</span>
            <span class="account-pick-name">{{ acct.name }}</span>
          </div>
          <el-icon><ArrowRight /></el-icon>
        </div>
      </div>
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
            <el-input v-model="orderForm.promotion_code" :placeholder="t('plan.promo_placeholder')" clearable />
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
        <el-alert v-if="subInsecureHidden" type="warning" :closable="false" show-icon :title="t('sub.insecure_hidden')" style="margin-bottom:10px" />
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

/* ============================================
   Modern Design System - Glassmorphism Theme
   ============================================ */

:root {
  /* Background - Modern gradient */
  --bg-gradient: linear-gradient(135deg, #667eea 0%, #764ba2 50%, #f093fb 100%);
  --bg-solid: #f8fafc;
  --bg-elevated: #ffffff;
  
  /* Glassmorphism Cards */
  --glass-bg: rgba(255, 255, 255, 0.85);
  --glass-bg-hover: rgba(255, 255, 255, 0.95);
  --glass-border: rgba(255, 255, 255, 0.5);
  --glass-shadow: 
    0 4px 6px -1px rgba(0, 0, 0, 0.05),
    0 10px 15px -3px rgba(0, 0, 0, 0.08),
    0 20px 25px -5px rgba(0, 0, 0, 0.05);
  --glass-shadow-hover:
    0 10px 15px -3px rgba(0, 0, 0, 0.08),
    0 20px 25px -5px rgba(0, 0, 0, 0.1),
    0 0 0 1px rgba(255, 255, 255, 0.5) inset;
  
  /* Text Colors */
  --text-primary: #0f172a;
  --text-secondary: #475569;
  --text-muted: #94a3b8;
  
  /* Accent - Purple to Blue gradient */
  --accent: #6366f1;
  --accent-dark: #4f46e5;
  --accent-light: rgba(99, 102, 241, 0.1);
  --accent-border: rgba(99, 102, 241, 0.2);
  --accent-gradient: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  
  /* Status Colors */
  --success: #10b981;
  --success-light: rgba(16, 185, 129, 0.1);
  --danger: #ef4444;
  --danger-light: rgba(239, 68, 68, 0.1);
  --warning: #f59e0b;
  --warning-light: rgba(245, 158, 11, 0.1);
  --info: #3b82f6;
  
  /* UI Elements */
  --border: rgba(148, 163, 184, 0.2);
  --ring-bg: rgba(99, 102, 241, 0.1);
  --nav-bg: rgba(255, 255, 255, 0.9);
  --sidebar-bg: rgba(255, 255, 255, 0.95);
  --sidebar-width: 260px;
  
  /* Typography */
  --font-family: {{.FontFamily}};
  --font-mono: 'SF Mono', 'Fira Code', 'JetBrains Mono', monospace;
  
  /* Spacing Scale */
  --space-1: 0.25rem;
  --space-2: 0.5rem;
  --space-3: 0.75rem;
  --space-4: 1rem;
  --space-6: 1.5rem;
  --space-8: 2rem;
  
  /* Border Radius */
  --radius-sm: 6px;
  --radius-md: 10px;
  --radius-lg: 16px;
  --radius-xl: 24px;
  --radius-full: 9999px;
  
  /* Transitions */
  --transition-fast: 150ms ease;
  --transition-base: 250ms ease;
  --transition-slow: 350ms ease;
}

[data-theme="dark"] {
  --bg-gradient: linear-gradient(135deg, #1a1a2e 0%, #16213e 50%, #0f3460 100%);
  --bg-solid: #0f172a;
  --bg-elevated: #1e293b;
  
  --glass-bg: rgba(30, 41, 59, 0.8);
  --glass-bg-hover: rgba(30, 41, 59, 0.9);
  --glass-border: rgba(255, 255, 255, 0.1);
  --glass-shadow: 
    0 4px 6px -1px rgba(0, 0, 0, 0.3),
    0 10px 15px -3px rgba(0, 0, 0, 0.4),
    0 0 0 1px rgba(255, 255, 255, 0.05) inset;
  --glass-shadow-hover:
    0 10px 15px -3px rgba(0, 0, 0, 0.4),
    0 20px 25px -5px rgba(0, 0, 0, 0.5),
    0 0 0 1px rgba(255, 255, 255, 0.1) inset;
  
  --text-primary: #f1f5f9;
  --text-secondary: #cbd5e1;
  --text-muted: #64748b;
  
  --accent: #818cf8;
  --accent-dark: #6366f1;
  --accent-light: rgba(129, 140, 248, 0.15);
  --accent-border: rgba(129, 140, 248, 0.25);
  --accent-gradient: linear-gradient(135deg, #818cf8 0%, #a78bfa 100%);
  
  --border: rgba(255, 255, 255, 0.1);
  --ring-bg: rgba(129, 140, 248, 0.15);
  --nav-bg: rgba(15, 23, 42, 0.9);
  --sidebar-bg: rgba(15, 23, 42, 0.95);
}

/* ============================================
   Base Styles & Reset
   ============================================ */

*, *::before, *::after { 
  margin: 0; 
  padding: 0; 
  box-sizing: border-box;
}

html {
  scroll-behavior: smooth;
}

body {
  font-family: var(--font-family);
  background: var(--bg-solid);
  color: var(--text-primary);
  min-height: 100vh;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
  line-height: 1.6;
  overflow-x: hidden;
}

/* Selection styling */
::selection {
  background: var(--accent);
  color: white;
}

/* Scrollbar styling */
::-webkit-scrollbar {
  width: 8px;
  height: 8px;
}

::-webkit-scrollbar-track {
  background: transparent;
}

::-webkit-scrollbar-thumb {
  background: var(--border);
  border-radius: var(--radius-full);
}

::-webkit-scrollbar-thumb:hover {
  background: var(--text-muted);
}

/* Focus styles */
:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}

/* ============================================
   Glassmorphism Components
   ============================================ */

.glass {
  background: var(--glass-bg);
  border: 1px solid var(--glass-border);
  box-shadow: var(--glass-shadow);
  border-radius: var(--radius-lg);
  backdrop-filter: blur(20px);
  -webkit-backdrop-filter: blur(20px);
  transition: transform var(--transition-base), 
              box-shadow var(--transition-base),
              background var(--transition-base);
}

.glass:hover {
  background: var(--glass-bg-hover);
  box-shadow: var(--glass-shadow-hover);
}

/* Card component */
.card {
  composes: glass;
  padding: var(--space-6);
  margin-bottom: var(--space-4);
  border-top: 3px solid transparent;
  border-image: var(--accent-gradient) 1;
  border-image-slice: 3 0 0 0;
}

.card-header {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  margin-bottom: var(--space-4);
}

.card-icon { 
  font-size: 1.25rem;
  color: var(--accent);
  transition: transform var(--transition-fast), color var(--transition-fast);
}

.card:hover .card-icon {
  transform: scale(1.1) rotate(-5deg);
}

.card-title { 
  font-size: 1.125rem; 
  font-weight: 700; 
  background: var(--accent-gradient);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

.card-action { 
  font-size: 0.875rem; 
  color: var(--accent); 
  cursor: pointer; 
  margin-left: auto; 
  font-weight: 600;
  transition: opacity var(--transition-fast);
}

.card-action:hover {
  opacity: 0.8;
}

.card-desc { 
  font-size: 0.875rem; 
  color: var(--text-secondary); 
  margin-bottom: var(--space-3); 
  line-height: 1.6; 
}

.card-actions { 
  margin-top: var(--space-4); 
  display: flex; 
  gap: var(--space-2); 
  flex-wrap: wrap; 
}

/* ============================================
   Auth / Login Page
   ============================================ */

.auth-wrapper {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;
  padding: var(--space-4);
  background: var(--bg-gradient);
  background-attachment: fixed;
}

.auth-wrapper::before {
  content: '';
  position: fixed;
  inset: 0;
  background: 
    radial-gradient(circle at 20% 80%, rgba(120, 119, 198, 0.3) 0%, transparent 50%),
    radial-gradient(circle at 80% 20%, rgba(255, 119, 198, 0.3) 0%, transparent 50%);
  pointer-events: none;
  z-index: 0;
}

.auth-bg {
  position: fixed; 
  inset: 0;
  background: var(--bg-gradient);
  z-index: 0;
}

.auth-card {
  position: relative; 
  z-index: 1;
  border-radius: var(--radius-xl);
  padding: var(--space-8) var(--space-6);
  width: 100%;
  max-width: 420px;
  text-align: center;
  background: var(--glass-bg);
  border: 1px solid var(--glass-border);
  box-shadow: 
    0 25px 50px -12px rgba(0, 0, 0, 0.25),
    0 0 0 1px rgba(255, 255, 255, 0.1) inset;
  backdrop-filter: blur(20px);
  -webkit-backdrop-filter: blur(20px);
  animation: authCardEnter 0.6s ease-out;
}

@keyframes authCardEnter {
  from {
    opacity: 0;
    transform: translateY(20px) scale(0.98);
  }
  to {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
}

.auth-logo { 
  margin-bottom: var(--space-4);
  animation: logoPulse 2s ease-in-out infinite;
}

@keyframes logoPulse {
  0%, 100% { transform: scale(1); }
  50% { transform: scale(1.05); }
}

.logo-img { 
  width: 72px; 
  height: 72px; 
  border-radius: var(--radius-lg);
  box-shadow: 0 8px 32px rgba(99, 102, 241, 0.3);
}

.logo-icon { 
  display: flex;
  align-items: center;
  justify-content: center;
  animation: logoPulse 2s ease-in-out infinite;
}

.logo-icon .el-icon {
  filter: drop-shadow(0 4px 8px rgba(99, 102, 241, 0.3));
}

.auth-title {
  font-size: 1.75rem;
  font-weight: 800;
  background: var(--accent-gradient);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  margin-bottom: var(--space-1);
  letter-spacing: -0.025em;
}

.auth-subtitle {
  font-size: 0.9375rem;
  color: var(--text-secondary);
  margin-bottom: var(--space-6);
  font-weight: 500;
}

.auth-tabs { 
  margin-bottom: 0; 
}

.auth-tabs :deep(.el-tabs__nav-wrap::after) {
  background: var(--border);
  height: 1px;
}

.auth-tabs :deep(.el-tabs__item) {
  font-weight: 600;
  font-size: 0.9375rem;
  color: var(--text-muted);
  transition: color var(--transition-fast);
}

.auth-tabs :deep(.el-tabs__item.is-active) {
  color: var(--accent);
}

.auth-tabs :deep(.el-tabs__active-bar) {
  background: var(--accent-gradient);
  height: 3px;
  border-radius: var(--radius-full);
}

.auth-form { 
  margin-top: var(--space-4); 
}

.auth-form :deep(.el-input__wrapper) {
  background: var(--bg-solid);
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  box-shadow: none;
  transition: all var(--transition-fast);
}

.auth-form :deep(.el-input__wrapper:hover) {
  border-color: var(--accent-border);
}

.auth-form :deep(.el-input__wrapper.is-focus) {
  border-color: var(--accent);
  box-shadow: 0 0 0 3px var(--accent-light);
}

/* OAuth 登录按钮（第三方登录 ✓：启用哪个显示哪个 ✓） */
.oauth-divider { display: flex; align-items: center; margin: 20px 0 16px; color: var(--text-muted); font-size: 13px; }
.oauth-divider::before, .oauth-divider::after { content: ''; flex: 1; border-bottom: 1px solid var(--border); }
.oauth-divider::before { margin-right: 12px; }
.oauth-divider::after { margin-left: 12px; }
.oauth-buttons { display: flex; gap: 12px; }
.oauth-btn { flex: 1; display: flex; align-items: center; justify-content: center; gap: 8px; padding: 10px 16px; border: 1px solid var(--border); border-radius: 10px; background: var(--glass-bg); cursor: pointer; font-size: 14px; font-weight: 500; color: var(--text-primary); transition: all 0.2s; }
.oauth-btn:hover { border-color: var(--accent); background: var(--accent-light); }
.oauth-btn svg { width: 20px; height: 20px; flex-shrink: 0; }

/* 合规文档链接（Google 验证要求登录页可见 ✓） */
.auth-legal { margin-top: 14px; font-size: 12px; color: var(--text-muted); }
.auth-legal a { color: var(--text-secondary); text-decoration: none; }
.auth-legal a:hover { color: var(--accent); text-decoration: underline; }
.auth-legal-sep { margin: 0 8px; }

.btn-block {
  width: 100%;
  background: var(--accent-gradient) !important;
  border: none !important;
  font-weight: 700 !important;
  font-size: 1rem;
  padding: var(--space-3) var(--space-4) !important;
  border-radius: var(--radius-md) !important;
  box-shadow: 
    0 4px 14px rgba(99, 102, 241, 0.4),
    0 0 0 1px rgba(255, 255, 255, 0.1) inset !important;
  transition: all var(--transition-base) !important;
  position: relative;
  overflow: hidden;
}

.btn-block::before {
  content: '';
  position: absolute;
  inset: 0;
  background: linear-gradient(135deg, rgba(255,255,255,0.2) 0%, transparent 50%);
  pointer-events: none;
}

.btn-block:hover {
  transform: translateY(-2px);
  box-shadow: 
    0 8px 25px rgba(99, 102, 241, 0.5),
    0 0 0 1px rgba(255, 255, 255, 0.1) inset !important;
}

.btn-block:active {
  transform: translateY(0);
}

/* ============================================
   App Layout & Navigation
   ============================================ */

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
  padding: 0 var(--space-6);
  height: 64px;
  position: sticky;
  top: 0;
  z-index: 100;
  background: var(--nav-bg);
  border-bottom: 1px solid var(--border);
  box-shadow: 0 1px 3px rgba(0,0,0,0.05);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
}

.top-bar-left { 
  display: flex; 
  align-items: center; 
  gap: var(--space-3);
}

.top-logo { 
  width: 36px; 
  height: 36px; 
  border-radius: var(--radius-md);
  box-shadow: 0 2px 8px rgba(99, 102, 241, 0.2);
}

.top-title { 
  font-size: 1.125rem;
  font-weight: 800;
  background: var(--accent-gradient);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  letter-spacing: -0.025em;
}

.top-bar-right { 
  display: flex; 
  align-items: center; 
  gap: var(--space-2);
}

.theme-toggle {
  background: none;
  border: none;
  cursor: pointer;
  font-size: 1.25rem;
  padding: var(--space-2);
  border-radius: var(--radius-md);
  transition: all var(--transition-fast);
  display: flex;
  align-items: center;
  justify-content: center;
}

.theme-toggle:hover { 
  background: var(--accent-light);
  transform: scale(1.05);
}

.locale-switcher { 
  display: inline-flex; 
}

.locale-switcher .theme-toggle { 
  display: inline-flex; 
  align-items: center; 
  justify-content: center;
  font-size: 1rem;
}

.locale-switcher :deep(.el-dropdown-menu__item) { 
  display: flex; 
  align-items: center; 
  justify-content: space-between;
}

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
  height: 22px;
  flex-shrink: 0;
  margin-right: 12px;
}

.app-sidebar .sidebar-icon svg {
  width: 18px;
  height: 18px;
}

.sidebar-item:hover .sidebar-icon,
.sidebar-item.active .sidebar-icon {
  color: var(--accent);
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

/* Gift Section */
.acct-gift-section {
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid var(--border);
}
.acct-gift-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary);
}
.acct-gift-count {
  font-size: 11px;
  font-weight: 700;
  color: #fff;
  background: var(--accent);
  padding: 1px 8px;
  border-radius: 20px;
}
.acct-gift-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.acct-gift-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 8px 12px;
  background: var(--accent-light);
  border: 1px solid var(--accent-border);
  border-radius: 10px;
}
.acct-gift-info {
  display: flex;
  align-items: center;
  gap: 6px;
  flex: 1;
  min-width: 0;
}
.acct-gift-type {
  font-size: 12px;
  font-weight: 600;
  color: var(--accent);
  background: var(--glass-bg);
  padding: 2px 8px;
  border-radius: 6px;
  flex-shrink: 0;
}
.acct-gift-value {
  font-size: 13px;
  font-weight: 700;
  color: var(--text-primary);
}
.acct-gift-source {
  font-size: 11px;
  color: var(--text-muted);
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
.badge-ready { background: #d1fae5; color: #065f46; }
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

/* Account pick list */
.account-pick-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.account-pick-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  background: var(--card-bg);
  border: 1px solid var(--border);
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s;
}
.account-pick-item:hover {
  border-color: var(--accent);
  background: var(--accent-light);
}
.account-pick-info {
  display: flex;
  align-items: center;
  gap: 10px;
}
.account-pick-num {
  font-size: 13px;
  font-weight: 700;
  color: var(--accent);
  min-width: 24px;
}
.account-pick-name {
  font-size: 14px;
  color: var(--text-primary);
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
.more-subs-qr-img { width: 220px; height: 220px; border-radius: 8px; }
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
.nav-icon { 
  font-size: 20px; 
  width: 20px;
  height: 20px;
  margin-bottom: 4px;
  transition: color var(--transition-fast), transform var(--transition-fast);
}

.nav-icon svg {
  width: 20px;
  height: 20px;
}
.nav-label { font-size: 11px; color: var(--text-muted); }

/* ============================================
   Responsive Design - Mobile First
   ============================================ */

/* Mobile (default) */
.app-sidebar { display: none; }
.bottom-nav { display: flex; }
.main-content {
  padding: var(--space-4);
  max-width: 100%;
  width: 100%;
  margin: 0 auto;
}
.overview-layout { flex-direction: column; }
.overview-sidebar { width: 100%; }
.info-grid { grid-template-columns: 1fr 1fr; }
.feature-cards-grid { grid-template-columns: repeat(auto-fill, minmax(140px, 1fr)); }

/* Tablet */
@media (min-width: 640px) {
  .main-content {
    padding: var(--space-6);
  }
  .info-grid { grid-template-columns: repeat(3, 1fr); }
  .feature-cards-grid { grid-template-columns: repeat(auto-fill, minmax(150px, 1fr)); }
}

/* Desktop */
@media (min-width: 768px) {
  .app-sidebar { display: block; }
  .bottom-nav { display: none; }
  .main-content {
    margin-left: var(--sidebar-width);
    padding: var(--space-6) var(--space-8) var(--space-8);
    max-width: calc(100vw - var(--sidebar-width));
    width: 100%;
  }
  .overview-layout { flex-direction: row; }
  .overview-sidebar { display: flex; width: 280px; flex-shrink: 0; }
  .info-grid { grid-template-columns: repeat(2, 1fr); }
  .feature-cards-grid { grid-template-columns: repeat(auto-fill, minmax(160px, 1fr)); }
}

/* Large Desktop */
@media (min-width: 1024px) {
  .main-content {
    max-width: calc(1200px + var(--sidebar-width));
    padding: var(--space-6) var(--space-8) var(--space-8);
  }
  .info-grid { grid-template-columns: repeat(4, 1fr); }
}

/* Extra Large */
@media (min-width: 1280px) {
  .main-content {
    max-width: calc(1400px + var(--sidebar-width));
  }
}

/* Mobile landscape optimization */
@media (max-width: 767px) and (orientation: landscape) {
  .bottom-nav {
    padding: 4px 0;
  }
  .nav-item {
    padding: 4px 12px;
  }
  .nav-label {
    font-size: 10px;
  }
}

/* Safe area for notched phones */
@supports (padding: max(0px)) {
  .bottom-nav {
    padding-bottom: max(8px, env(safe-area-inset-bottom));
  }
  .main-content {
    padding-bottom: max(80px, env(safe-area-inset-bottom) + 80px);
  }
  @media (min-width: 768px) {
    .main-content {
      padding-bottom: var(--space-8);
    }
  }
}

/* ---- Element Plus Overrides ---- */
.el-button--primary { --el-color-primary: #2563eb; }
.el-button--primary:not(.is-plain) {
  background: linear-gradient(135deg, var(--accent), var(--accent-dark)) !important;
  border: none !important;
  font-weight: 600 !important;
  box-shadow: 0 2px 8px rgba(37,99,235,0.18) !important;
  color: #fff !important;
}
.el-button--primary:not(.is-plain):hover {
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(37,99,235,0.28) !important;
}
/* Plain button styles */
.el-button--primary.is-plain {
  background: var(--accent-light) !important;
  border: 1px solid var(--accent-border) !important;
  color: var(--accent) !important;
  font-weight: 600 !important;
}
.el-button--primary.is-plain:hover {
  background: var(--accent) !important;
  color: #fff !important;
  border-color: var(--accent) !important;
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
.hero-eyebrow { font-size: 11px; text-transform: uppercase; letter-spacing: 1px; color: var(--accent); margin-bottom: 4px; }
.hero-heading { font-size: 22px; font-weight: 700; color: var(--text-primary); margin: 4px 0; }
.hero-desc { font-size: 14px; color: var(--text-secondary); margin-bottom: 12px; }
.hero-btn {
  display: inline-flex; align-items: center; gap: 7px;
  padding: 9px 22px; border-radius: 8px;
  background: linear-gradient(135deg, var(--accent), var(--accent-dark));
  color: #fff; font-weight: 700; font-size: 14px;
  text-decoration: none; cursor: pointer; border: none;
  box-shadow: 0 2px 8px rgba(37,99,235,0.22);
  transition: transform 0.15s, box-shadow 0.15s;
}
.hero-btn:hover {
  transform: translateY(-2px);
  box-shadow: 0 5px 16px rgba(37,99,235,0.32);
}
.rich-text-content { font-size: 14px; line-height: 1.7; color: var(--text-primary); }
.rich-text-content h2 { font-size: 18px; margin: 8px 0; }
.rich-text-content h3 { font-size: 16px; margin: 8px 0; }
.rich-text-content p { margin: 6px 0; }
.rich-text-content blockquote { border-left: 3px solid var(--accent); padding-left: 12px; margin: 8px 0; color: var(--text-secondary); }
.rich-text-content ul, .rich-text-content ol { padding-left: 20px; margin: 6px 0; }
.notice-content { border-left: 4px solid; padding: 12px 14px; border-radius: 8px; font-size: 13px; line-height: 1.6; }
.notice-info { border-color: var(--accent); background: var(--accent-light); }
.notice-success { border-color: var(--success); background: #ecfdf5; }
.notice-warning { border-color: var(--warning); background: #fffbeb; }
.notice-error { border-color: var(--danger); background: #fef2f2; }
.notice-content .notice-title { font-weight: 600; margin-bottom: 4px; }
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
.markdown-content { padding: 4px 0; }
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

/* ============================================
   Page & Content Transitions
   ============================================ */

/* Page switch animation */
.page-enter-active,
.page-leave-active {
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.page-enter-from {
  opacity: 0;
  transform: translateX(20px);
}

.page-leave-to {
  opacity: 0;
  transform: translateX(-20px);
}

/* Card stagger animation */
@keyframes cardSlideUp {
  from {
    opacity: 0;
    transform: translateY(30px) scale(0.95);
  }
  to {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
}

.card {
  animation: cardSlideUp 0.5s cubic-bezier(0.4, 0, 0.2, 1) backwards;
}

.card:nth-child(1) { animation-delay: 0ms; }
.card:nth-child(2) { animation-delay: 50ms; }
.card:nth-child(3) { animation-delay: 100ms; }
.card:nth-child(4) { animation-delay: 150ms; }
.card:nth-child(5) { animation-delay: 200ms; }
.card:nth-child(6) { animation-delay: 250ms; }

/* Card hover lift effect */
.card {
  transition: transform var(--transition-base), 
              box-shadow var(--transition-base),
              background var(--transition-base);
}

.card:hover {
  transform: translateY(-4px);
  box-shadow: var(--glass-shadow-hover), 0 20px 40px -10px rgba(0, 0, 0, 0.15);
}

/* Button ripple effect */
.hero-btn,
.btn-block {
  position: relative;
  overflow: hidden;
}

.hero-btn::after,
.btn-block::after {
  content: '';
  position: absolute;
  inset: 0;
  background: radial-gradient(circle at var(--x, 50%) var(--y, 50%), 
    rgba(255, 255, 255, 0.3) 0%, 
    transparent 60%);
  opacity: 0;
  transition: opacity 0.3s;
}

.hero-btn:active::after,
.btn-block:active::after {
  opacity: 1;
}

/* Sidebar item animation */
.sidebar-item {
  transition: all var(--transition-fast);
  position: relative;
}

.sidebar-item::before {
  content: '';
  position: absolute;
  left: 0;
  top: 50%;
  transform: translateY(-50%) scaleY(0);
  width: 3px;
  height: 20px;
  background: var(--accent-gradient);
  border-radius: 0 3px 3px 0;
  transition: transform var(--transition-fast);
}

.sidebar-item.active::before,
.sidebar-item:hover::before {
  transform: translateY(-50%) scaleY(1);
}

.sidebar-item:hover {
  background: var(--accent-light);
  padding-left: calc(var(--space-3) + 4px);
}

/* Bottom nav item animation */
.nav-item {
  transition: all var(--transition-fast);
}

.nav-item:hover {
  transform: translateY(-2px);
}

.nav-item.active {
  transform: translateY(-4px);
}

.nav-item.active .nav-icon {
  animation: navIconBounce 0.5s ease;
}

@keyframes navIconBounce {
  0%, 100% { transform: scale(1); }
  50% { transform: scale(1.2); }
}

/* Skeleton loading animation */
@keyframes skeletonShimmer {
  0% { background-position: -200% 0; }
  100% { background-position: 200% 0; }
}

.skeleton {
  background: linear-gradient(
    90deg,
    var(--glass-bg) 25%,
    var(--glass-bg-hover) 50%,
    var(--glass-bg) 75%
  );
  background-size: 200% 100%;
  animation: skeletonShimmer 1.5s infinite;
  border-radius: var(--radius-md);
}

/* Empty state animation */
.empty-state {
  animation: emptyStateFadeIn 0.5s ease;
}

@keyframes emptyStateFadeIn {
  from {
    opacity: 0;
    transform: scale(0.9);
  }
  to {
    opacity: 1;
    transform: scale(1);
  }
}

.empty-icon {
  animation: emptyIconFloat 3s ease-in-out infinite;
  color: var(--accent);
}

.empty-icon.is-loading {
  animation: emptyIconFloat 3s ease-in-out infinite, rotate 1s linear infinite;
}

@keyframes emptyIconFloat {
  0%, 100% { transform: translateY(0); }
  50% { transform: translateY(-10px); }
}

@keyframes rotate {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

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

// Icon SVG paths for Element Plus icons
// Icon set for the portal navigation. Rendered with fill-rule="evenodd", so
// nested sub-paths punch holes (outline style) instead of merging into a blob.
var iconPaths = {
  'HomeFilled': 'M512 96 96 448h96v416h240V608h160v256h240V448h96L512 96z',
  'DataAnalysis': 'M160 800h704v64H160v-64zM224 576h96v192h-96V576zm160-128h96v320h-96V448zm160-192h96v512h-96V256zm160 256h96v256h-96V512z',
  'ShoppingCart': 'M64 96h160l128 512h448v64H320L192 160H64V96zm320 704a64 64 0 1 0 128 0 64 64 0 0 0-128 0zm320 0a64 64 0 1 0 128 0 64 64 0 0 0-128 0zM288 224h672l-96 320H384l-96-320zm84 64 58 192h386l58-192H372z',
  'Diamond': 'M320 128h384l192 256-384 512L128 384 320 128zm34 64L212 384h600L670 192H354z',
  'Wallet': 'M64 256a64 64 0 0 1 64-64h704a64 64 0 0 1 64 64v64h-64v-64H128v448h704v-64h64v64a64 64 0 0 1-64 64H128a64 64 0 0 1-64-64V256zm576 128h320v256H640a128 128 0 0 1 0-256zm0 64a64 64 0 0 0 0 128h256V448H640zm32 32a32 32 0 1 1 0 64 32 32 0 0 1 0-64z',
  'Ticket': 'M96 256h832v192a64 64 0 0 0 0 128v192H96V576a64 64 0 0 0 0-128V256zm64 64v82a128 128 0 0 1 0 220v82h704v-82a128 128 0 0 1 0-220v-82H160zm192 32h64v320h-64V352z',
  'Server': 'M128 160h768v256H128V160zm64 64v128h640V224H192zm576 32a32 32 0 1 1 0 64 32 32 0 0 1 0-64zM128 608h768v256H128V608zm64 64v128h640V672H192zm576 32a32 32 0 1 1 0 64 32 32 0 0 1 0-64z',
  'List': 'M128 256h768v64H128v-64zm0 256h768v64H128v-64zm0 256h768v64H128v-64z',
  'Reading': 'M96 224c128-48 256-40 384 24v576c-128-64-256-72-384-24V224zm64 86v426c96-20 192-10 288 30V340c-96-40-192-50-288-30zm384 162c128-64 256-72 384-24v576c-128-48-256-40-384 24V472zm64 40v426c96-40 192-50 288-30V482c-96-20-192-10-288 30z',
  'Service': 'M512 96C299 96 128 267 128 480v160h64V480c0-177 143-320 320-320s320 143 320 320v160h64V480C896 267 725 96 512 96zM96 576h160v288H96V576zm64 64v160h32V640h-32zm608-64h160v288H768V576zm64 64v160h32V640h-32z',
  'Gift': 'M96 288h832v160H96V288zm64 64v32h704v-32H160zm0 160h704v384H160V512zm64 64v256h576V576H224zm256-64h64v384h-64V512zM348 96c76 0 132 60 164 160h-164a80 80 0 0 1 0-160zm0 64a16 16 0 0 0 0 32h74c-24-22-49-32-74-32zm328-64a80 80 0 0 1 0 160H512c32-100 88-160 164-160zm0 64c-25 0-50 10-74 32h74a16 16 0 0 0 0-32z',
  'User': 'M512 128a192 192 0 1 1 0 384 192 192 0 0 1 0-384zm0 64a128 128 0 1 0 0 256 128 128 0 0 0 0-256zM192 896c0-176 143-320 320-320s320 144 320 320h-64c0-141-115-256-256-256s-256 115-256 256h-64z',
  'Lock': 'M320 448V320a192 192 0 0 1 384 0v128h-64V320a128 128 0 0 0-256 0v128h-64zM192 480h640v416H192V480zm64 64v288h512V544H256zm224 64h64v160h-64V608z',
  'Box': 'M96 192h832v192H96V192zm64 64v64h704v-64H160zm0 192h704v416H160V448zm64 64v288h576V512H224zm192 64h192v64H416v-64z',
  'Bell': 'M512 96a256 256 0 0 1 256 256v208l96 128v32H160v-32l96-128V352A256 256 0 0 1 512 96zm0 64a192 192 0 0 0-192 192v230l-56 74h496l-56-74V352a192 192 0 0 0-192-192zm-96 704h192a96 96 0 0 1-192 0z',
  'Globe': 'M512 96a416 416 0 1 0 0 832 416 416 0 0 0 0-832zm0 64a352 352 0 1 1 0 704 352 352 0 0 1 0-704zM128 480h768v64H128v-64zM512 160c-88 96-136 216-136 352s48 256 136 352c88-96 136-216 136-352S600 256 512 160zm0 118c40 74 62 155 62 234s-22 160-62 234c-40-74-62-155-62-234s22-160 62-234z',
  'Document': 'M192 96h432l208 208v624H192V96zm64 64v704h512V352H576V160H256zm384 26v102h102L640 186zM320 480h384v64H320v-64zm0 160h384v64H320v-64z',
  'Link': 'M576 128c-141 0-256 115-256 256h64c0-106 86-192 192-192s192 86 192 192-86 192-192 192h-64v64h64c141 0 256-115 256-256S717 128 576 128zm-320 320c-141 0-256 115-256 256s115 256 256 256 256-115 256-256h-64c0 106-86 192-192 192s-192-86-192-192 86-192 192-192h64v-64h-64zM384 448v64h256v-64H384z'
}

// Navigation icons are resolved by target first: the stored config historically
// uses emoji (e.g. "🏠") which never matched an iconPaths key, so every item fell
// back to the same 'Link' glyph. Matching on target keeps each page distinct
// regardless of what the stored icon field contains.
var navTargetIcons = {
  'overview': 'HomeFilled', 'dashboard': 'DataAnalysis', 'home': 'HomeFilled',
  'subscription': 'Ticket', 'subscribe': 'Ticket',
  'store': 'ShoppingCart', 'shop': 'ShoppingCart', 'plan': 'Diamond', 'plans': 'Diamond',
  'billing': 'Wallet', 'order': 'Wallet', 'orders': 'Wallet', 'finance': 'Wallet',
  'nodes': 'Server', 'node': 'Server',
  'tutorial': 'Reading', 'docs': 'Reading', 'help': 'Reading',
  'support': 'Service', 'ticket': 'Service', 'tickets': 'Service',
  'referral': 'Gift', 'invite': 'Gift',
  'profile': 'User', 'account': 'User', 'security': 'Lock',
  'logs': 'Box', 'notice': 'Bell', 'announcement': 'Bell'
}

// Emoji fallback, used for custom/external nav items that have no known target.
var navEmojiIcons = {
  '🏠': 'HomeFilled', '🏡': 'HomeFilled', '📊': 'DataAnalysis', '📈': 'DataAnalysis',
  '🛒': 'ShoppingCart', '🛍': 'ShoppingCart', '💎': 'Diamond',
  '💰': 'Wallet', '💵': 'Wallet', '💳': 'Wallet',
  '📋': 'Ticket', '🎫': 'Ticket', '🎟': 'Ticket',
  '🖥': 'Server', '🗄': 'Server', '🌐': 'Globe', '🔗': 'Link',
  '📚': 'Reading', '📖': 'Reading', '📄': 'Document',
  '🎧': 'Service', '💬': 'Service', '🛎': 'Service',
  '🎁': 'Gift', '🎉': 'Gift', '👤': 'User', '🙍': 'User',
  '🔒': 'Lock', '🔐': 'Lock', '📦': 'Box', '🔔': 'Bell', '📢': 'Bell'
}

var navIconPath = function(nav) {
  if (!nav) return iconPaths['Link']
  var key = nav.icon
  if (key && iconPaths[key]) return iconPaths[key]
  var byTarget = nav.target && navTargetIcons[String(nav.target).toLowerCase()]
  if (byTarget && iconPaths[byTarget]) return iconPaths[byTarget]
  var byEmoji = key && navEmojiIcons[key]
  if (byEmoji && iconPaths[byEmoji]) return iconPaths[byEmoji]
  if (nav.href) return iconPaths['Globe']
  return iconPaths['Link']
}

var app = createApp({
  setup: function() {
    // Preview mode: simulate logged-in state so the builder can preview pages
    var urlParams = new URLSearchParams(window.location.search)
    var isPreviewMode = urlParams.get('preview') === '1'
    var token = ref(isPreviewMode ? 'preview-token' : (localStorage.getItem('client_token') || ''))
    var refreshToken = ref(localStorage.getItem('client_refresh_token') || '')
    var authTab = ref('login')
    var loading = ref(false)
    var rotating = ref(false)
    var refreshing = ref(false)
    // Preview mode: ?page=xxx forces the active tab (used by the admin builder)
    var urlParams = new URLSearchParams(window.location.search)
    var previewPage = urlParams.get('page')
    var isPreviewMode = urlParams.get('preview') === '1'
    var activeTab = ref(previewPage || localStorage.getItem('portal_active_tab') || 'overview')
    var showPasswordDialog = ref(false)
var showQr = ref(false)
var showSubQrDialog = ref(false)
var showSubUrl = ref(false)
    var selectedPlatform = ref('')
var announcementPopupVisible = ref(false)
var activeAnnouncement = ref(null)
    var logoUrl = ref((typeof PORTAL_CONFIG !== 'undefined' && PORTAL_CONFIG.logo_url) || '')
    var siteName = ref((typeof PORTAL_CONFIG !== 'undefined' && PORTAL_CONFIG.site_name) || 'CoreFusion')
    // 合规文档链接 ✓（Google OAuth 验证要求登录页可见隐私政策/服务条款链接 ✓）
    var privacyUrl = ref((typeof PORTAL_CONFIG !== 'undefined' && PORTAL_CONFIG.privacy_policy_url) || '')
    var termsUrl = ref((typeof PORTAL_CONFIG !== 'undefined' && PORTAL_CONFIG.terms_of_service_url) || '')
    // 合规文档内容 ✓（点击链接弹窗展示 markdown ✓；URL 优先 ✓）
    var privacyContent = ref((typeof PORTAL_CONFIG !== 'undefined' && PORTAL_CONFIG.privacy_policy_content) || '')
    var termsContent = ref((typeof PORTAL_CONFIG !== 'undefined' && PORTAL_CONFIG.terms_of_service_content) || '')
    var isDark = ref(localStorage.getItem('portal_theme') === 'dark' || (!localStorage.getItem('portal_theme') && window.matchMedia('(prefers-color-scheme: dark)').matches))

    // Latest effective portal config — refreshed on every hot fetch from the DB.
    var latestPortalConfig = (typeof PORTAL_CONFIG !== 'undefined' && PORTAL_CONFIG) || {}

    // --- Theme color engine: apply builder theme (DB config) as CSS variables ---
    var hexToRgbParts = function(hex) {
      if (typeof hex !== 'string') return null
      var m = hex.trim().replace(/^#/, '')
      if (m.length === 3) m = m[0] + m[0] + m[1] + m[1] + m[2] + m[2]
      if (!/^[0-9a-fA-F]{6}$/.test(m)) return null
      return { r: parseInt(m.slice(0, 2), 16), g: parseInt(m.slice(2, 4), 16), b: parseInt(m.slice(4, 6), 16) }
    }
    var rgbaFromHex = function(hex, alpha) {
      var c = hexToRgbParts(hex)
      if (!c) return null
      return 'rgba(' + c.r + ', ' + c.g + ', ' + c.b + ', ' + alpha + ')'
    }
    var darkenHex = function(hex, ratio) {
      var c = hexToRgbParts(hex)
      if (!c) return null
      var to2 = function(v) {
        var n = Math.max(0, Math.min(255, Math.round(v * (1 - ratio))))
        var s = n.toString(16)
        return s.length === 1 ? '0' + s : s
      }
      return '#' + to2(c.r) + to2(c.g) + to2(c.b)
    }
    // Color vars customized by the builder — removed again in dark mode so the
    // tuned dark palette is not clobbered by light-mode colors.
    var customThemeVarNames = ['--accent', '--accent-dark', '--accent-light', '--accent-border', '--accent-gradient', '--ring-bg', '--bg-solid', '--bg-elevated', '--glass-bg', '--glass-bg-hover', '--text-primary', '--success', '--success-light']
    var applyThemeVars = function(cfg) {
      if (!cfg || typeof cfg !== 'object') return
      var style = document.documentElement.style
      if (cfg.font_family) style.setProperty('--font-family', cfg.font_family)
      if (isDark.value) {
        customThemeVarNames.forEach(function(n) { style.removeProperty(n) })
        return
      }
      var set = function(name, val) { if (val) style.setProperty(name, val) }
      var primary = cfg.primary_color
      if (primary) {
        set('--accent', primary)
        set('--accent-dark', darkenHex(primary, 0.12))
        set('--accent-light', rgbaFromHex(primary, 0.1))
        set('--accent-border', rgbaFromHex(primary, 0.2))
        set('--ring-bg', rgbaFromHex(primary, 0.1))
        var gradEnd = darkenHex(primary, 0.25)
        if (gradEnd) set('--accent-gradient', 'linear-gradient(135deg, ' + primary + ' 0%, ' + gradEnd + ' 100%)')
      }
      set('--bg-solid', cfg.background_color)
      var cardBg = cfg.card_background_color
      if (cardBg) {
        set('--bg-elevated', cardBg)
        set('--glass-bg', rgbaFromHex(cardBg, 0.85))
        set('--glass-bg-hover', rgbaFromHex(cardBg, 0.95))
      }
      set('--text-primary', cfg.text_color)
      if (cfg.accent_color) {
        set('--success', cfg.accent_color)
        set('--success-light', rgbaFromHex(cfg.accent_color, 0.1))
      }
    }

    var loginForm = ref({ email: '', password: '' })
    var registerForm = ref({ email: '', password: '', referral_code: '' })
    var pwdForm = ref({ old_password: '', new_password: '', confirm_password: '' })
    var settings = ref({ ip_whitelist: '', device_limit: 0 })
    var client = ref({})

    var logPackages = ref([])
    var logPacking = ref(false)
    var hasEntryNodes = ref(true)

    // --- Commercial module state ---
var plans = ref([])
var plansLoading = ref(false)
var trafficPacks = ref([])
var trafficPacksLoading = ref(false)
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
// --- Gifts ---
var gifts = ref([])
var giftClaiming = ref(false)
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
    var showAccountListDialog = ref(false)
    var accountListItems = ref([])
    var accountListTitle = ref('')
    var showChargeDialog = ref(false)
    var showPaymentDialog = ref(false) // v3.3: payment method dialog
var showPromoInput = ref(false) // promo code fold

    var registrationEnabled = ref(false)
    if (typeof PORTAL_CONFIG !== 'undefined' && PORTAL_CONFIG.registration_enabled !== undefined) {
      registrationEnabled.value = PORTAL_CONFIG.registration_enabled
    }

    // OAuth 登录（第三方登录 ✓）：后台启用哪个 provider，登录页就显示哪个按钮 ✓
    var oauthProviders = ref([])

    // Portal editor: modules & navigation from PORTAL_CONFIG
    var portalModules = ref([])
    var portalNavItems = ref([])
    // Markdown renderer (using marked.js loaded from CDN)
        var renderMd = function(md) {
      if (!md) return ''
      try { return marked.parse(md, { breaks: true, gfm: true }) } catch(e) { return md }
    }
    // 用户提交的内容（工单消息等）不能直接交给 marked + v-html：
    // marked 默认放行原始 HTML，而门户是 v-html 渲染，所以客户在工单里写 script 或
    // img onerror 就能在**客服/管理员的浏览器**里执行（跨角色存储型 XSS，审计 v5 M7）。
    // 这里先做 HTML 转义再走 markdown：Markdown 语法照常生效，标签只当文本显示。
    // 管理员自撰内容（公告/文章/freeform 模块）仍用 renderMd —— 那属于后台特权内容。
    var escapeHtmlForMd = function(s) {
      return String(s == null ? '' : s)
        .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;').replace(/'/g, '&#39;')
    }
    var renderMdUntrusted = function(md) {
      if (!md) return ''
      var safe = escapeHtmlForMd(md)
      try { return marked.parse(safe, { breaks: true, gfm: true }) } catch(e) { return safe }
    }
    if (typeof PORTAL_CONFIG !== 'undefined') {
      if (Array.isArray(PORTAL_CONFIG.modules)) {
        portalModules.value = PORTAL_CONFIG.modules.filter(function(m) { return m.visible !== false })
      }
      if (Array.isArray(PORTAL_CONFIG.navigation)) {
        portalNavItems.value = PORTAL_CONFIG.navigation.filter(function(n) { return n.enabled !== false })
      }
    }

    // --- Auth Region (editable login page) ---
    var authComponent = ref(null)
    
    // Compile auth region from portal config regions (re-callable on hot config updates)
    var compileAuthRegion = function(regions) {
      var regionSource = regions || (typeof PORTAL_CONFIG !== 'undefined' ? PORTAL_CONFIG.regions : null) || {}
      var region = regionSource.auth || null
      if (!region || !region.html) {
        authComponent.value = null
        return
      }
      
      // Build context for auth region
      var authCtx = {
        // Vue APIs
        ref: Vue.ref,
        computed: Vue.computed,
        watch: Vue.watch,
        onMounted: Vue.onMounted,
        onUnmounted: Vue.onUnmounted,
        
        // Reactive state
        loginForm: loginForm,
        registerForm: registerForm,
        authTab: authTab,
        loading: loading,
        registrationEnabled: registrationEnabled,
        logoUrl: logoUrl,
        isDark: isDark,
        
        // Site info (reactive site name from latest config)
        siteName: siteName.value,
        
        // Methods
        handleLogin: handleLogin,
        handleRegister: handleRegister,
        // 自助解封卡 ✓（解封卡 HTML 在 auth 区域内编译，ref/方法必须透传进 authCtx，
        // 否则渲染时 selfUnban undefined 直接报错 ✗）
        selfUnban: selfUnban,
        handleSelfUnban: handleSelfUnban,
        t: t,

        // OAuth（第三方登录 ✓：登录按钮按后台启用状态显隐 ✓，ref 透传保响应式 ✓）
        oauthProviders: oauthProviders,
        isOAuthEnabled: isOAuthEnabled,
        oauthLogin: oauthLogin,

        // 合规文档链接 ✓（Google 验证要求登录页展示隐私政策/服务条款链接 ✓）
        privacyUrl: privacyUrl,
        termsUrl: termsUrl,
        privacyContent: privacyContent,
        termsContent: termsContent,
        openLegal: openLegal,
        hasLegalLinks: hasLegalLinks,

        // Config data
        data: region.data || {},
        
        // Utils
        ElMessage: ElMessage,
        ElMessageBox: ElMessageBox
      }
      
      // Compile JS setup function
      // JS should be an object literal or return statement
      // Examples: "{ foo: 1 }" or "return { foo: 1 }" or "console.log('init'); return {}"
      var setupFn
      if (region.js) {
        try {
          var jsCode = region.js.trim()
          // Wrap in function if it's an object literal (starts with {)
          if (jsCode.charAt(0) === '{' && jsCode.charAt(jsCode.length - 1) === '}') {
            jsCode = 'return ' + jsCode
          }
          // Create factory function with ctx in scope
          var factory = new Function('ctx', jsCode)
          setupFn = function() { 
            try {
              var result = factory(authCtx)
              return result || {}
            } catch(e) {
              console.error('Auth region JS runtime error:', e)
              return { __error: e.message }
            }
          }
        } catch(e) {
          console.error('Auth region JS compile error:', e)
          setupFn = function() { return { __error: e.message } }
        }
      } else {
        setupFn = function() { return authCtx }
      }
      
      // Inject CSS
      if (region.css) {
        var styleId = 'auth-region-css'
        var existing = document.getElementById(styleId)
        if (existing) existing.remove()
        var styleEl = document.createElement('style')
        styleEl.id = styleId
        styleEl.textContent = region.css
        document.head.appendChild(styleEl)
      }
      
      // Create component - call setupFn to get the setup result
      var setupResult = setupFn()
      authComponent.value = {
        template: region.html,
        setup: function() { return setupResult },
        // Bind methods from setup result to ensure they're accessible in template
        methods: {
          handleLogin: setupResult.handleLogin,
          handleRegister: setupResult.handleRegister
        }
      }
    }
    
    // --- i18n / multi-language pack ---
    var normalizeLangPack = function(raw) {
      if (raw && raw.locales && typeof raw.locales === 'object') return raw
      if (raw && typeof raw === 'object') return { default: 'zh-CN', locales: { 'zh-CN': raw } }
      return { default: 'zh-CN', locales: {} }
    }
    // Reactive lang pack — can be hot-swapped when the builder saves new translations
    var multiLang = ref(normalizeLangPack(typeof PORTAL_CONFIG !== 'undefined' ? PORTAL_CONFIG.lang : null))
    // Detect preferred locale
    var savedLocale = localStorage.getItem('portal_locale')
    var browserLang = (navigator.language || navigator.userLanguage || 'zh-CN').toLowerCase()
    var detectedLocale = savedLocale
    if (!detectedLocale || !multiLang.value.locales[detectedLocale]) {
      if (multiLang.value.locales[browserLang]) {
        detectedLocale = browserLang
      } else {
        var prefix = browserLang.split('-')[0]
        var found = false
        for (var code in multiLang.value.locales) {
          if (code.split('-')[0] === prefix) { detectedLocale = code; found = true; break }
        }
        if (!found) detectedLocale = multiLang.value.default || 'zh-CN'
      }
    }
    var currentLocale = ref(detectedLocale)
    var availableLocales = computed(function() {
      var list = []
      for (var code in multiLang.value.locales) {
        var meta = (multiLang.value.locales[code] && multiLang.value.locales[code]._meta) || {}
        list.push({ code: code, label: meta.label || code, flag: meta.flag || '🌐' })
      }
      return list
    })
    var currentLocaleFlag = computed(function() {
      var loc = multiLang.value.locales[currentLocale.value]
      if (loc && loc._meta && loc._meta.flag) return loc._meta.flag
      return '🌐'
    })
    var lang = computed(function() {
      return multiLang.value.locales[currentLocale.value] || multiLang.value.locales[multiLang.value.default] || {}
    })
    var t = function(key) {
      if (!key) return ''
      var v = lang.value[key]
      if (v === undefined || v === null || v === '') {
        var defaultPack = multiLang.value.locales[multiLang.value.default] || {}
        v = defaultPack[key]
      }
      if (v === undefined || v === null || v === '') return key
      return v
    }
    var switchLocale = function(code) {
      if (multiLang.value.locales[code]) {
        currentLocale.value = code
        localStorage.setItem('portal_locale', code)
        document.documentElement.lang = code
      }
    }
    
    // Fallback to default nav if none configured
    if (portalNavItems.value.length === 0) {
      portalNavItems.value = [
        { id: 'nav-home', label: t('nav.home'), icon: 'HomeFilled', target: 'overview', enabled: true },
        { id: 'nav-services', label: t('nav.store'), icon: 'ShoppingCart', target: 'store', enabled: true },
        { id: 'nav-billing', label: t('nav.billing'), icon: 'Wallet', target: 'billing', enabled: true },
        { id: 'nav-nodes', label: t('nav.nodes'), icon: 'List', target: 'nodes', enabled: true },
        { id: 'nav-tutorial', label: t('nav.tutorial'), icon: 'Reading', target: 'tutorial', enabled: true },
        { id: 'nav-tickets', label: t('nav.support'), icon: 'Service', target: 'support', enabled: true },
        { id: 'nav-referral', label: t('nav.referral'), icon: 'Gift', target: 'referral', enabled: false },
        { id: 'nav-profile', label: t('nav.profile'), icon: 'User', target: 'profile', enabled: true },
        { id: 'nav-logs', label: t('nav.logs'), icon: 'Document', target: 'logs', enabled: false }
      ]
    }
    // Modules for the current page (filtered by activeTab)
    // ⚠ visible 开关必须在这里生效 ✓：编辑器里把模块"隐藏"（visible=false）后，
    // 运行时也要真的不渲染 ✓（此前只按 page 过滤 ✗ → 隐藏开关形同虚设 ✗）。
    var currentModules = computed(function() {
      return portalModules.value.filter(function(m) { return (m.page || 'overview') === activeTab.value && m.visible !== false })
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
      if (hasLogMods) { fetchLogPackages(); checkEntryNodes() }
      // Auto-fetch commercial module data based on page
      var pageMods = portalModules.value.filter(function(m) { return (m.page || 'overview') === pageTarget })
      var hasPlanList = pageMods.some(function(m) { return m.type === 'plan_list' || m.type === 'order_create' })
      var hasTrafficPacks = pageMods.some(function(m) { return m.type === 'traffic_pack_list' })
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
      if (hasTrafficPacks || hasPlanList) fetchTrafficPacks()
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
      // Re-apply (or clear, in dark mode) builder theme colors from latest config
      applyThemeVars(latestPortalConfig)
    }
    applyTheme()

    // Favicon follows the configured logo (including hot config updates)
    var applyFavicon = function() {
      var link = document.getElementById('portal-favicon')
      if (link && logoUrl.value) link.href = logoUrl.value
    }
    watch(logoUrl, applyFavicon)
    applyFavicon()
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
      var done = function() {
        token.value = ''
        refreshToken.value = ''
        localStorage.removeItem('client_token')
        localStorage.removeItem('client_refresh_token')
        client.value = {}
      }
      var tk = token.value
      if (!tk) { done(); return }
      // 先在**服务端**吊销（令牌版本 +1 → 该客户端此前签发的所有 token 立即失效）。
      // 旧实现只清本地存储，服务端仍旧认那个 token（被窃取后有效期内还能用）。
      fetch('/api/v1/client-auth/logout', {
        method: 'POST',
        headers: { 'Authorization': 'Bearer ' + tk }
      }).then(function() { done() }).catch(function() { done() })
    }

    // ── 自助解封 ✓ ────────────────────────────────────────────────────────────
    // 登录返回 require_self_unban（仅"流量超额自动封禁"+功能开启时）→ 显示解封卡，
    // 点击后凭专用 token 调 POST /self-unban 恢复账号与节点凭据，成功后重新登录。
    var selfUnban = ref({ visible: false, token: '', hint: '', loading: false })
    var handleSelfUnban = function() {
      selfUnban.value.loading = true
      fetch('/api/v1/client-auth/self-unban', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Authorization': 'Bearer ' + selfUnban.value.token }
      })
        .then(function(r) { return r.json().then(function(d) { return { ok: r.ok, data: d } }) })
        .then(function(x) {
          if (x.ok) {
            ElMessage.success(t('auth.self_unban_success'))
            selfUnban.value = { visible: false, token: '', hint: '', loading: false }
          } else {
            ElMessage.error((x.data && (x.data.message || x.data.error)) || t('auth.self_unban_failed'))
          }
        })
        .catch(function() { ElMessage.error(t('auth.self_unban_failed')) })
        .finally(function() { selfUnban.value.loading = false })
    }

    var handleLogin = function() {
      loading.value = true
      api('POST', '/login', loginForm.value).then(function(res) {
        // 自助解封分支：被封但可自解 → 不发常规 token，显示解封卡
        if (res.require_self_unban) {
          selfUnban.value = { visible: true, token: res.self_unban_token || '', hint: res.self_unban_hint || '', loading: false }
          ElMessage.warning(res.self_unban_hint || t('auth.self_unban_desc'))
          return
        }
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

    // ── OAuth 登录（第三方登录 ✓）────────────────────────────────────────────
    // 后台"系统设置 → OAuth"启用 GitHub/Google 后 ✓，登录页自动出现对应按钮 ✓。
    // 流程：跳 /oauth/<provider> → CA 验证 → 回调页带回 token（postMessage 或 URL）✓。
    var fetchOAuthProviders = function() {
      fetch('/api/v1/client-auth/oauth/providers').then(function(r) { return r.json() }).then(function(d) {
        var list = (d.data && d.data.providers) || d.providers || []
        oauthProviders.value = list.filter(function(p) { return p.enabled })
      }).catch(function() {})
    }
    var isOAuthEnabled = function(provider) {
      return oauthProviders.value.some(function(p) { return p.provider === provider })
    }
    var oauthLogin = function(provider) {
      window.location.href = '/api/v1/client-auth/oauth/' + provider
    }

    // ── 合规文档弹窗 ✓（登录卡片"隐私政策/服务条款"点击打开 ✓，与常规站点一致 ✓）──
    // 展示优先级：配了 URL → 新标签打开外链 ✓；配了内容 → 弹窗渲染 markdown ✓。
    var legalDialogVisible = ref(false)
    var legalDialogTitle = ref('')
    var legalDialogHtml = ref('')
    var openLegal = function(type) {
      var url = type === 'privacy' ? privacyUrl.value : termsUrl.value
      if (url) { window.open(url, '_blank', 'noopener'); return }
      var content = type === 'privacy' ? privacyContent.value : termsContent.value
      if (!content) return
      legalDialogTitle.value = t(type === 'privacy' ? 'legal.privacy' : 'legal.terms')
      legalDialogHtml.value = renderMd(content)
      legalDialogVisible.value = true
    }
    // 登录卡片链接是否显示 ✓：URL 或内容任一配置 ✓
    var hasLegalLinks = computed(function() {
      return !!(privacyUrl.value || privacyContent.value || termsUrl.value || termsContent.value)
    })
    // 兜底：回调以 /p#access_token=... 方式回来时（无 opener 的整页跳转 ✓）直接收 token ✓
    // 安全审计 L-5：token 走 URL **fragment** 下发 —— fragment 不会进访问日志/Referer/
    // 浏览器历史，query 串会把长效 JWT 散播到各处 ✗。
    var handleOAuthCallback = function() {
      var params = new URLSearchParams(window.location.hash.slice(1))
      // 自助解封分支：OAuth 回调带 require_self_unban=1（账号 suspended 但符合自解封条件，
      // 后端只发了 typ=self_unban 专用 token）→ 清理地址栏并显示解封卡，**不进入登录态**。
      // 解封成功后用户重新点 OAuth 登录即可（此时账号已 active）。
      if (params.get('require_self_unban') === '1') {
        var suToken = params.get('self_unban_token') || ''
        var suHint = params.get('self_unban_hint') || ''
        window.history.replaceState({}, '', window.location.pathname)
        if (suToken) {
          selfUnban.value = { visible: true, token: suToken, hint: suHint, loading: false }
          ElMessage.warning(suHint || t('auth.self_unban_desc'))
        } else {
          ElMessage.error(t('auth.self_unban_failed'))
        }
        return
      }
      var at = params.get('access_token')
      if (!at) return
      var rt = params.get('refresh_token') || ''
      token.value = at
      refreshToken.value = rt
      localStorage.setItem('client_token', at)
      if (rt) localStorage.setItem('client_refresh_token', rt)
      window.history.replaceState({}, '', window.location.pathname)
      ElMessage.success(t('auth.login_success'))
      fetchMe(); fetchEndpoints()
    }
    // 弹窗回调：回调页通过 postMessage 把 token 发回本页 ✓（只认同源消息 ✓，防 token 外泄 ✓）
    var oauthMsgHandler = function(event) {
      if (event.origin !== window.location.origin) return
      if (!event.data || event.data.type !== 'oauth-callback' || !event.data.data) return
      var d = event.data.data
      token.value = d.access_token
      refreshToken.value = d.refresh_token || ''
      localStorage.setItem('client_token', d.access_token)
      if (d.refresh_token) localStorage.setItem('client_refresh_token', d.refresh_token)
      ElMessage.success(t('auth.login_success'))
      fetchMe(); fetchEndpoints()
    }

    // Check for referral code in URL (before compileAuthRegion so auth module can pick it up)
    var urlParams = new URLSearchParams(window.location.search)
    var refCode = urlParams.get('ref')
    if (refCode) {
      registerForm.value.referral_code = refCode
      authTab.value = 'register'
    }

    // Compile auth region after handleLogin and handleRegister are defined
    compileAuthRegion()

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
    var appleAccounts = computed(function() {
      return sharedAccounts.value.filter(function(sa) { return sa.type === 'apple_id' && sa.enabled !== false })
    })
    var netflixAccounts = computed(function() {
      return sharedAccounts.value.filter(function(sa) { return sa.type === 'netflix' && sa.enabled !== false })
    })
    var appleAccount = computed(function() {
      return appleAccounts.value[0] || null
    })
    var netflixAccount = computed(function() {
      return netflixAccounts.value[0] || null
    })

    var endpoints = ref([])
    var fetchEndpoints = function() {
      fetch('/api/v1/public/subscription-endpoints').then(function(r) { return r.json() }).then(function(d) {
        // backend may return a plain array (d.data) or a paginated wrapper (d.data.items)
        var list = []
        if (Array.isArray(d.data)) list = d.data
        else if (d.data && Array.isArray(d.data.items)) list = d.data.items
        endpoints.value = list
      }).catch(function() {})
    }

    var hostIsPublicIP = function(host) {
      if (!host) return false
      var h = String(host).replace('[', '').replace(']', '')
      var m = h.split('.')
      if (m.length === 4) {
        var a = parseInt(m[0], 10), b = parseInt(m[1], 10)
        if (isNaN(a) || isNaN(b)) return false
        var priv = a === 10 || a === 127 || a === 0 || (a === 172 && b >= 16 && b <= 31) || (a === 192 && b === 168) || (a === 169 && b === 254) || (a === 100 && b >= 64 && b <= 127)
        return !priv
      }
      if (h.indexOf(':') >= 0) {
        if (h === '::1') return false
        var p = h.toLowerCase()
        if (p.indexOf('fc') === 0 || p.indexOf('fd') === 0 || p.indexOf('fe80') === 0) return false
        return true
      }
      return false
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
      // 非 HTTPS + 公网地址 → 不外发订阅链接（不安全）；内网地址仍可展示。
      if (window.location.protocol !== 'https:' && hostIsPublicIP(window.location.hostname)) {
        return ''
      }
      return window.location.origin + '/sub/' + t
    })
    var subInsecureHidden = computed(function() {
      return !!client.value.sub_token && !subUrl.value
    })

    var qrCodeUrl = ref('')
    var generateQr = function() {
      if (!subUrl.value) { qrCodeUrl.value = ''; return }
      try {
        var canvas = document.createElement('canvas')
        new QRious({ element: canvas, value: subUrl.value, size: 300, background: isDark.value ? '#1a1a2e' : '#ffffff', foreground: isDark.value ? '#e0e0f0' : '#000000', level: 'M' })
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
    if (Array.isArray(d.data)) clientConfigs.value = d.data
    else if (d.data && Array.isArray(d.data.items)) clientConfigs.value = d.data.items
    else clientConfigs.value = []
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
    new QRious({ element: canvas, value: url, size: 220, background: isDark.value ? '#1a1a2e' : '#ffffff', foreground: isDark.value ? '#e0e0f0' : '#000000', level: 'M' })
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

    var checkEntryNodes = function() {
      api('GET', '/logs/types').then(function(res) {
        hasEntryNodes.value = !!res.has_entry_nodes
      }).catch(function() { hasEntryNodes.value = true })
    }

    var confirmCreateLogPackage = function() {
      if (!hasEntryNodes.value) {
        ElMessage.warning(t('log.no_nodes'))
        return
      }
      var dialogHtml = '<div style="margin-bottom:12px">' + t('log.confirm_desc') + '</div>' +
        '<div style="display:flex;gap:12px">' +
          '<label style="cursor:pointer"><input type="radio" name="logPackHours" value="1" checked style="margin-right:4px">1h</label>' +
          '<label style="cursor:pointer"><input type="radio" name="logPackHours" value="6" style="margin-right:4px">6h</label>' +
          '<label style="cursor:pointer"><input type="radio" name="logPackHours" value="24" style="margin-right:4px">24h</label>' +
        '</div>'
      ElMessageBox.confirm(dialogHtml, t('log.btn_pack'), {
        dangerouslyUseHTMLString: true,
        confirmButtonText: t('common.confirm'),
        cancelButtonText: t('common.cancel')
      }).then(function() {
        var checked = document.querySelector('input[name="logPackHours"]:checked')
        var hours = checked ? parseInt(checked.value) : 1
        createLogPackage(hours)
      }).catch(function() {})
    }

    var logPackHours = ref(1)

    var createLogPackage = function(hours) {
      logPacking.value = true
      api('POST', '/logs/package', { hours: hours }).then(function(res) {
        ElMessage.success(t('log.request_sent'))
        fetchLogPackages()
      }).catch(function(e) { ElMessage.error(e.message) }).finally(function() { logPacking.value = false })
    }

    var downloadLogPackage = function(id) {
      // 一次性短时票据（M10）：新窗口带不了 Authorization 头；长期 token 进 URL 会残留在
      // 日志/历史/Referer 里，而且客户端中间件**只读 Header**，所以以前这个下载一直是 401。
      fetch('/api/v1/client-auth/ticket?scope=client_download', {
        method: 'POST',
        headers: { 'Authorization': 'Bearer ' + token.value }
      }).then(function(r) { return r.json() }).then(function(d) {
        var tk = (d && d.data && d.data.ticket) || (d && d.ticket) || ''
        if (!tk) { ElMessage.error('下载凭证获取失败'); return }
        window.open('/api/v1/client-auth/logs/packages/' + id + '/download?ticket=' + encodeURIComponent(tk), '_blank')
      })
    }

    var deleteLogPackage = function(id) {
      ElMessageBox.confirm(t('log.delete_confirm'), t('common.confirm'), {
        confirmButtonText: t('common.confirm'),
        cancelButtonText: t('common.cancel'),
        type: 'warning'
      }).then(function() {
        api('DELETE', '/logs/packages/' + id).then(function() {
          ElMessage.success(t('log.deleted'))
          fetchLogPackages()
        }).catch(function(e) { ElMessage.error(e.message) })
      }).catch(function() {})
    }

    var pkgStatusText = function(s) { return { pending: t('common.processing'), processing: t('common.processing'), ready: t('order.status_completed'), failed: t('common.failed') }[s] || s || '-' }

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

    // --- Traffic packs: fetch traffic packs ---
    var fetchTrafficPacks = function() {
      trafficPacksLoading.value = true
      api('GET', '/traffic-packs').then(function(res) {
        trafficPacks.value = Array.isArray(res) ? res : []
      }).catch(function() {}).finally(function() { trafficPacksLoading.value = false })
    }

    // --- Buy a traffic pack (reuses payment dialog) ---
    var buyTrafficPack = function(pack) {
      if (client.value.plan_id === 0 || !client.value.plan_id) {
        ElMessage.warning(t('traffic_pack.need_plan'))
        return
      }
      orderForm.value = {
        variant_id: pack.id,
        order_type: 'new',
        promotion_code: '',
        pay_method: 'balance',
        gift_card_code: '',
        charge_site_id: null,
        plan_name: pack.name,
        variant_name: pack.name,
        price: pack.price,
        currency: pack.currency || 'CNY',
        duration_days: pack.duration_days || 0,
        quota_bytes: pack.quota_bytes || 0,
        is_traffic_pack: true
      }
      showPromoInput.value = false
      fetchBalance()
      if (chargeSites.value.length === 0) fetchChargeSites()
      showPaymentDialog.value = true
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
      var isTrafficPack = !!orderForm.value.is_traffic_pack
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

      // Traffic pack uses a dedicated buy endpoint with a simpler payload
      var apiPath = isTrafficPack ? '/traffic-packs/buy' : '/orders'
      var apiPayload = isTrafficPack ? {
        variant_id: orderForm.value.variant_id,
        pay_method: orderForm.value.pay_method,
        charge_site_id: orderForm.value.pay_method === 'charge_site' ? orderForm.value.charge_site_id : null
      } : payload

      api('POST', apiPath, apiPayload).then(function(res) {
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

    // --- Gifts ---
    var fetchGifts = function() {
      api('GET', '/gifts').then(function(res) {
        gifts.value = Array.isArray(res) ? res : []
      }).catch(function() {})
    }
    var claimGift = function(giftId) {
      giftClaiming.value = true
      api('POST', '/gifts/' + giftId + '/claim').then(function(res) {
        ElMessage.success(t('gift.claim_success'))
        fetchGifts()
        // Refresh client data to reflect quota/expiry changes
        fetchMe()
      }).catch(function(e) { ElMessage.error(e.message) }).finally(function() { giftClaiming.value = false })
    }
    var giftTypeText = function(type) { return { days: t('gift.type_days'), traffic: t('gift.type_traffic') }[type] || type }
    var giftSourceText = function(s) { return { fault_compensation: t('gift.src_fault'), monthly_bonus: t('gift.src_monthly'), promotion: t('gift.src_promo'), admin: t('gift.src_admin') }[s] || s }

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
      // Special handling for Update card: show general category articles (cross-platform guides)
      if (platformId === 'update') {
        return articles.value.filter(function(a) {
          return a.category === 'general' && a.is_published
        }).sort(function(a, b) { return (a.sort_order || 0) - (b.sort_order || 0) })
      }
      // Map platform id to article platform field
      var platformMap = {
        windows: 'windows',
        android: 'android',
        apple: 'ios',
        mac: 'mac',
        linux: 'linux',
        browser: 'other',
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
    // Show list of accounts by type, user picks one to view credentials
    var showAccountList = function(accounts, title) {
      if (!accounts || accounts.length === 0) return
      if (accounts.length === 1) {
        applySharedAccount(accounts[0].id)
      } else {
        accountListItems.value = accounts
        accountListTitle.value = title
        showAccountListDialog.value = true
      }
    }
    var pickSharedAccount = function(id) {
      showAccountListDialog.value = false
      applySharedAccount(id)
    }
    // Click handler for streaming cards: show list or directly view
    var viewAppleAccounts = function() { showAccountList(appleAccounts.value, t('shared.apple_id')) }
    var viewNetflixAccounts = function() { showAccountList(netflixAccounts.value, t('shared.netflix')) }
    var repairSharedAccount = function(id, modData) {
      api('POST', '/shared-accounts/' + id + '/repair').then(function(res) {
        ElMessage.success(t('shared.repair_sent'))
      }).catch(function(e) { ElMessage.error(e.message) })
    }
    var sharedAcctTypeText = function(type) { if (type === 'apple_id') return 'Apple ID'; if (type === 'netflix') return 'Netflix'; if (type === 'other') return t('common.other'); return type || '-' }

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
      new QRious({ element: canvas, value: text, size: 360, background: isDark.value ? '#1a1a2e' : '#ffffff', foreground: isDark.value ? '#e0e0f0' : '#000000', level: 'M' })
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

    // --- Portal config hot-load (builder changes render directly from DB) ---
    // Apply a fresh portal config to the running page: theme, site name,
    // navigation, modules, language pack and editable regions. Everything is
    // reactive, so the page re-renders in place without a full reload.
    var applyPortalConfig = function(cfg, opts) {
      if (!cfg || typeof cfg !== 'object') return
      latestPortalConfig = cfg
      if (cfg.site_name) {
        siteName.value = cfg.site_name
        try { document.title = cfg.site_name } catch(e) {}
      }
      if (cfg.logo_url !== undefined) logoUrl.value = cfg.logo_url
      if (cfg.privacy_policy_url !== undefined) privacyUrl.value = cfg.privacy_policy_url
      if (cfg.terms_of_service_url !== undefined) termsUrl.value = cfg.terms_of_service_url
      if (cfg.privacy_policy_content !== undefined) privacyContent.value = cfg.privacy_policy_content
      if (cfg.terms_of_service_content !== undefined) termsContent.value = cfg.terms_of_service_content
      if (cfg.registration_enabled !== undefined) registrationEnabled.value = !!cfg.registration_enabled
      if (Array.isArray(cfg.modules)) {
        portalModules.value = cfg.modules.filter(function(m) { return m.visible !== false })
      }
      if (Array.isArray(cfg.navigation)) {
        portalNavItems.value = cfg.navigation.filter(function(n) { return n.enabled !== false })
      }
      if (cfg.lang) multiLang.value = normalizeLangPack(cfg.lang)
      if (cfg.regions && cfg.regions.auth) compileAuthRegion(cfg.regions)
      applyThemeVars(cfg)
      // reload page data after module update (only for hot refreshes, not initial load)
      if (opts && opts.reloadData) fetchPageData(activeTab.value)
    }

    // Fetch the lean portal runtime config — always a fresh DB read server-side.
    var fetchPortalConfig = function(reloadData) {
      fetch('/api/v1/public/portal-config').then(function(r) { return r.json() }).then(function(d) {
        var cfg = (d && d.data) || d
        applyPortalConfig(cfg, { reloadData: reloadData })
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
      fetchPortalConfig(false)
      // OAuth：拉启用列表（显隐按钮 ✓）+ 收回调 token + 监听弹窗 postMessage ✓
      fetchOAuthProviders()
      handleOAuthCallback()
      window.addEventListener('message', oauthMsgHandler)
      // Refetch config when the tab becomes visible again so builder changes
      // show up without a manual reload
      document.addEventListener('visibilitychange', function() {
        if (!document.hidden) fetchPortalConfig(true)
      })
      // Skip API data loading in preview mode (no real token)
      if (isPreviewMode) {
        // Just load the config and render the page structure
        return
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
var initHasTrafficPacks = initMods.some(function(m) { return m.type === 'traffic_pack_list' })
var initHasReferral = initMods.some(function(m) { return m.type === 'referral' })
        var initHasTickets = initMods.some(function(m) { return m.type === 'ticket_list' })
        var initHasArticles = initMods.some(function(m) { return m.type === 'article_list' || m.type === 'tutorial_cards' || m.type === 'client_download' })
        var initHasSharedAccts = initMods.some(function(m) { return m.type === 'shared_accounts' || m.type === 'streaming_status' })
        var initHasOrderList = initMods.some(function(m) { return m.type === 'order_list' })
        var initHasNodeList = initMods.some(function(m) { return m.type === 'node_list' })
if (initHasPlanList || initHasNodeList) fetchPlans()
if (initHasTrafficPacks || initHasPlanList) fetchTrafficPacks()
if (initHasNodeList) fetchNodes()
        if (initHasPlanList || initHasCharge) fetchChargeSites()
        if (initHasCharge) fetchChargeOrders()
      if (initHasBalance) { fetchBalance(); fetchBalanceTxns() }
      if (initHasBalance || initMods.some(function(m) { return m.type === 'account_info' })) fetchGifts()
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

    onUnmounted(function() {
      if (refreshTimer) clearInterval(refreshTimer)
      window.removeEventListener('message', oauthMsgHandler)
    })

    watch(token, function(v) { if (v) { startAutoRefresh() } else { if (refreshTimer) clearInterval(refreshTimer) } })

    // --- Module component runtime (all module types) ---
    var moduleComponents = ref({})

    // Navigation helper for modules
    var goToPage = function(target) { activeTab.value = target; saveActiveTab(target); fetchPageData(target) }

    // Generic API call for module ctx (supports query params)
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

    var registerModuleComponents = function() {
      var comps = {}
      portalModules.value.forEach(function(mod) {
        // Skip modules without html/js code (notice modules are handled separately in template)
        if (!mod.data || (!mod.data.html && !mod.data.js)) return
        var html = (mod.data && mod.data.html) || '<div></div>'
        var js = (mod.data && mod.data.js) || ''
        var css = (mod.data && mod.data.css) || ''

        // Build module context with i18n, data, and utilities
        var modCtx = {
          // Vue APIs
          ref: Vue.ref, computed: Vue.computed, watch: Vue.watch,
          onMounted: Vue.onMounted, onUnmounted: Vue.onUnmounted,
          // Module data
          data: mod.data,
          mod: mod,
          // Global reactive data
          client: client, token: token, isDark: isDark,
          settings: settings, endpoints: endpoints,
          announcements: announcements,
          plans: plans, plansLoading: plansLoading,
          chargeSites: chargeSites, chargeOrders: chargeOrders,
          balance: balance, balanceTxns: balanceTxns,
          myOrders: myOrders, myOrdersLoading: myOrdersLoading,
          allPlanNodes: allPlanNodes,
          referralCode: referralCode, referralStats: referralStats, referralRecords: referralRecords,
          tickets: tickets,
          articles: articles,
          sharedAccounts: sharedAccounts,
          logPackages: logPackages,
          gifts: gifts, giftClaiming: giftClaiming, claimGift: claimGift,
          giftTypeText: giftTypeText, giftSourceText: giftSourceText, fetchGifts: fetchGifts,
          // Computed values
          usagePercent: usagePercent, usageColor: usageColor, ringDash: ringDash,
          subUrl: subUrl, subStatusText: subStatusText, statusText: statusText,
          // i18n
          t: t, lang: lang, currentLocale: currentLocale, switchLocale: switchLocale,
          // API
          api: {
            get: function(p, q) { return apiRaw('GET', p, null, q) },
            post: function(p, b) { return apiRaw('POST', p, b) },
            put: function(p, b) { return apiRaw('PUT', p, b) },
            delete: function(p) { return apiRaw('DELETE', p, null) },
            raw: function(m, p, b) { return apiRaw(m, p, b) }
          },
          // Utilities
          formatBytes: formatBytes, formatDate: formatDate, formatDateTime: formatDateTime,
          formatMoney: formatMoney, copyText: copyText, elMessage: ElMessage, renderMd: renderMd, renderMdUntrusted: renderMdUntrusted,
          // Navigation
          goToPage: goToPage,
          activeTab: activeTab,
          // Config
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
              var result = factory(modCtx) || {}
              // Auto-merge common bindings so templates can access them
              if (!result.t) result.t = t
              if (!result.mod) result.mod = mod
              if (!result.client) result.client = client
              if (!result.token) result.token = token
              if (!result.isDark) result.isDark = isDark
              if (!result.formatBytes) result.formatBytes = formatBytes
              if (!result.formatDate) result.formatDate = formatDate
              if (!result.formatDateTime) result.formatDateTime = formatDateTime
              if (!result.formatMoney) result.formatMoney = formatMoney
              if (!result.copyText) result.copyText = copyText
              if (!result.usagePercent) result.usagePercent = usagePercent
              if (!result.usageColor) result.usageColor = usageColor
              if (!result.ringDash) result.ringDash = ringDash
              if (!result.subUrl) result.subUrl = subUrl
              if (!result.subStatusText) result.subStatusText = subStatusText
              if (!result.statusText) result.statusText = statusText
              if (!result.announcements) result.announcements = announcements
              if (!result.settings) result.settings = settings
              if (!result.endpoints) result.endpoints = endpoints
              if (!result.plans) result.plans = plans
              if (!result.chargeSites) result.chargeSites = chargeSites
              if (!result.chargeOrders) result.chargeOrders = chargeOrders
              if (!result.balance) result.balance = balance
              if (!result.balanceTxns) result.balanceTxns = balanceTxns
              if (!result.myOrders) result.myOrders = myOrders
              if (!result.allPlanNodes) result.allPlanNodes = allPlanNodes
              if (!result.referralCode) result.referralCode = referralCode
              if (!result.referralStats) result.referralStats = referralStats
              if (!result.referralRecords) result.referralRecords = referralRecords
              if (!result.tickets) result.tickets = tickets
              if (!result.articles) result.articles = articles
              if (!result.sharedAccounts) result.sharedAccounts = sharedAccounts
              if (!result.logPackages) result.logPackages = logPackages
              if (!result.renderMd) result.renderMd = renderMd
              if (!result.goToPage) result.goToPage = goToPage
              if (!result.activeTab) result.activeTab = activeTab
              if (!result.lang) result.lang = lang
              // Extended auto-merge: expose all main-app state & functions to module templates
              var ext = {
                refreshing: refreshing, rotating: rotating, logoUrl: logoUrl, registrationEnabled: registrationEnabled,
                selectedPlatform: selectedPlatform, selectPlatform: selectPlatform,
                getSelectedPlatformName: getSelectedPlatformName, getSelectedPlatformUrl: getSelectedPlatformUrl,
                getPlatformArticles: getPlatformArticles, getTutorialIcon: getTutorialIcon,
                showSubUrl: showSubUrl, showQrDialog: showQrDialog, showSubQrDialog: showSubQrDialog,
                handleSubAction: handleSubAction, rotateToken: rotateToken,
                showNodeQRDialog: showNodeQRDialog, showNodeQR: showNodeQR,
                showNodeDetailDialog: showNodeDetailDialog, currentNodeDetail: currentNodeDetail, showNodeDetail: showNodeDetail,
                showTicketDialog: showTicketDialog, showTicketDetail: showTicketDetail,
                openTicket: openTicket, newTicketForm: newTicketForm, ticketReplyContent: ticketReplyContent,
                ticketBusy: ticketBusy, createTicket: createTicket, replyTicket: replyTicket, closeTicket: closeTicket,
                ticketStatusText: ticketStatusText, ticketStatusBadge: ticketStatusBadge,
                articleCategory: articleCategory, showArticleDetail: showArticleDetail, articleDetail: articleDetail,
                fetchArticles: fetchArticles, openArticle: openArticle, articleCatText: articleCatText,
                plansLoading: plansLoading, myOrdersLoading: myOrdersLoading,
                purchaseVariant: purchaseVariant, submitOrder: submitOrder,
                buyTrafficPack: buyTrafficPack, trafficPacks: trafficPacks, trafficPacksLoading: trafficPacksLoading,
                orderForm: orderForm, orderBusy: orderBusy,
                showChargeDialog: showChargeDialog, submitCharge: submitCharge,
                chargeForm: chargeForm, chargeBusy: chargeBusy, chargeStatusText: chargeStatusText,
                cardCode: cardCode, redeemBusy: redeemBusy, redeemCard: redeemCard,
                showPaymentDialog: showPaymentDialog, confirmPayment: confirmPayment, showPromoInput: showPromoInput,
                confirmCreateLogPackage: confirmCreateLogPackage, fetchLogPackages: fetchLogPackages,
                downloadLogPackage: downloadLogPackage, deleteLogPackage: deleteLogPackage,
                logPacking: logPacking, hasEntryNodes: hasEntryNodes, pkgStatusText: pkgStatusText,
                createLogPackage: createLogPackage, checkEntryNodes: checkEntryNodes,
                pwdForm: pwdForm, changePassword: changePassword, saveSettings: saveSettings,
                navigateTo: navigateTo, scrollToModule: scrollToModule,
                referralInviteUrl: referralInviteUrl, referralStatusText: referralStatusText,
                commissionRecords: commissionRecords, withdrawRecords: withdrawRecords,
                appleAccount: appleAccount, netflixAccount: netflixAccount,
                appleAccounts: appleAccounts, netflixAccounts: netflixAccounts,
                applySharedAccount: applySharedAccount, repairSharedAccount: repairSharedAccount,
                viewAppleAccounts: viewAppleAccounts, viewNetflixAccounts: viewNetflixAccounts,
                pickSharedAccount: pickSharedAccount, showAccountListDialog: showAccountListDialog,
                accountListItems: accountListItems, accountListTitle: accountListTitle,
                sharedAcctTypeText: sharedAcctTypeText, sharedAccountCreds: sharedAccountCreds,
                showCredentialsDialog: showCredentialsDialog,
                orderStatusText: orderStatusText, orderStatusBadge: orderStatusBadge, getOrderServiceName: getOrderServiceName,
                openAnnouncementPopup: openAnnouncementPopup, announcementPopupVisible: announcementPopupVisible,
                activeAnnouncement: activeAnnouncement,
                toggleTheme: toggleTheme, showStreamingAccount: showStreamingAccount,
                qrCodeUrl: qrCodeUrl, toggleQr: toggleQr,
                onNavClick: onNavClick, fetchPageData: fetchPageData, saveActiveTab: saveActiveTab,
                currentLocale: currentLocale, availableLocales: availableLocales, currentLocaleFlag: currentLocaleFlag,
                switchLocale: switchLocale,
                moreSubsVisible: moreSubsVisible, moreSubFormats: moreSubFormats,
                showMoreSubQrDialog: showMoreSubQrDialog, moreSubQrDialogUrl: moreSubQrDialogUrl,
                moreSubQrDialogVisible: moreSubQrDialogVisible, buildSubLink: buildSubLink,
                showNoticePanel: showNoticePanel, iconPaths: iconPaths, navIconPath: navIconPath,
                gifts: gifts, giftClaiming: giftClaiming, claimGift: claimGift,
                giftTypeText: giftTypeText, giftSourceText: giftSourceText, fetchGifts: fetchGifts
              }
              for (var k in ext) { if (!(k in result)) result[k] = ext[k] }
              return result
            } catch(e) {
              console.error('[Module ' + mod.id + ']', e)
              return { __error: e.message, t: t, mod: mod }
            }
          }
        } catch(e) {
          setupFn = function() { return { __error: e.message, t: t, mod: mod } }
        }

        // Inject CSS
        if (css) {
          var styleId = 'module-css-' + mod.id
          var existing = document.getElementById(styleId)
          if (existing) existing.remove()
          var styleEl = document.createElement('style')
          styleEl.id = styleId
          styleEl.textContent = css
          document.head.appendChild(styleEl)
        }

        comps[mod.id] = { template: html, setup: setupFn }
      })
      moduleComponents.value = comps
    }

    watch(portalModules, registerModuleComponents, { immediate: true })

return {
      // Auth & shell
      token: token, authTab: authTab, loading: loading,
      activeTab: activeTab, showPasswordDialog: showPasswordDialog,
      showSubQrDialog: showSubQrDialog, logoUrl: logoUrl, siteName: siteName,
      isDark: isDark, registrationEnabled: registrationEnabled,
      loginForm: loginForm, registerForm: registerForm, client: client,
      handleLogin: handleLogin, handleRegister: handleRegister, handleLogout: handleLogout,
      selfUnban: selfUnban, handleSelfUnban: handleSelfUnban,
      toggleTheme: toggleTheme,
      // OAuth（第三方登录 ✓：登录页按启用 provider 显隐按钮 ✓）
      oauthProviders: oauthProviders, isOAuthEnabled: isOAuthEnabled, oauthLogin: oauthLogin,
      // 合规文档链接 ✓（Google 验证要求登录页展示 ✓）
      privacyUrl: privacyUrl, termsUrl: termsUrl,
      // 合规文档弹窗 ✓（点击链接打开条款内容 ✓）
      openLegal: openLegal, legalDialogVisible: legalDialogVisible,
      legalDialogTitle: legalDialogTitle, legalDialogHtml: legalDialogHtml,
      hasLegalLinks: hasLegalLinks,
      // Navigation & modules
      currentModules: currentModules, portalNavItems: portalNavItems, onNavClick: onNavClick,
      renderMd: renderMd, renderMdUntrusted: renderMdUntrusted, moduleComponents: moduleComponents,
      // Shared dialogs
      showNodeQRDialog: showNodeQRDialog, nodeQRContainer: nodeQRContainer,
      showNodeDetailDialog: showNodeDetailDialog, currentNodeDetail: currentNodeDetail,
      announcementPopupVisible: announcementPopupVisible, activeAnnouncement: activeAnnouncement,
      openAnnouncementPopup: openAnnouncementPopup, announcements: announcements,
      // Payment dialogs
      orderForm: orderForm, orderBusy: orderBusy,
      trafficPacks: trafficPacks, trafficPacksLoading: trafficPacksLoading, buyTrafficPack: buyTrafficPack,
      chargeSites: chargeSites, chargeForm: chargeForm, chargeBusy: chargeBusy,
      cardCode: cardCode, redeemBusy: redeemBusy, balance: balance,
      showChargeDialog: showChargeDialog, showPaymentDialog: showPaymentDialog,
      confirmPayment: confirmPayment, showPromoInput: showPromoInput,
      redeemCard: redeemCard, submitCharge: submitCharge,
      // Ticket dialogs
      showTicketDialog: showTicketDialog, showTicketDetail: showTicketDetail,
      ticketDetail: ticketDetail, ticketReplyContent: ticketReplyContent, ticketBusy: ticketBusy,
      newTicketForm: newTicketForm, createTicket: createTicket, replyTicket: replyTicket, closeTicket: closeTicket,
      ticketStatusText: ticketStatusText, ticketStatusBadge: ticketStatusBadge,
      // Article & shared account dialogs
      showArticleDetail: showArticleDetail, articleDetail: articleDetail,
      showCredentialsDialog: showCredentialsDialog, sharedAccountCreds: sharedAccountCreds,
      showAccountListDialog: showAccountListDialog, accountListItems: accountListItems, accountListTitle: accountListTitle,
      pickSharedAccount: pickSharedAccount,
      sharedAcctTypeText: sharedAcctTypeText,
      // i18n
      t: t, currentLocale: currentLocale, availableLocales: availableLocales,
      currentLocaleFlag: currentLocaleFlag, switchLocale: switchLocale,
      // Subscription dialogs
      qrCodeUrl: qrCodeUrl, moreSubsVisible: moreSubsVisible, moreSubFormats: moreSubFormats,
      moreSubQrDialogVisible: moreSubQrDialogVisible, moreSubQrDialogUrl: moreSubQrDialogUrl,
      buildSubLink: buildSubLink, showMoreSubQrDialog: showMoreSubQrDialog,
      subUrl: subUrl, subInsecureHidden: subInsecureHidden,
      // Utils & icons
      copyText: copyText, formatBytes: formatBytes, formatDate: formatDate, formatMoney: formatMoney,
      iconPaths: iconPaths, navIconPath: navIconPath, authComponent: authComponent,
      // Gifts
      gifts: gifts, giftClaiming: giftClaiming, claimGift: claimGift,
      giftTypeText: giftTypeText, giftSourceText: giftSourceText, fetchGifts: fetchGifts
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
