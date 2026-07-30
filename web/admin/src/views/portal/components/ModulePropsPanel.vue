<template>
  <div class="col-properties">
    <!-- Region Properties -->
    <div v-if="ed.selectedRegion" class="props-panel">
      <h3 class="props-title">{{ t('portal.editLoginPage') }}</h3>
      <el-form label-position="top" size="small">
        <el-divider content-position="left">{{ t('portal.basicSettings') }}</el-divider>
        <el-form-item :label="t('portal.subtitle')">
          <el-input v-model="ed.selectedRegion.data.subtitle" @input="ed.onCodeChange" />
        </el-form-item>
        <el-form-item :label="t('portal.logoEmoji')">
          <el-input v-model="ed.selectedRegion.data.logo_emoji" @input="ed.onCodeChange" />
        </el-form-item>

        <el-divider content-position="left">{{ t('portal.formText') }}</el-divider>
        <el-form-item :label="t('portal.loginLabel')">
          <el-input v-model="ed.selectedRegion.data.login_label" @input="ed.onCodeChange" />
        </el-form-item>
        <el-form-item :label="t('portal.registerLabel')">
          <el-input v-model="ed.selectedRegion.data.register_label" @input="ed.onCodeChange" />
        </el-form-item>
        <el-form-item :label="t('portal.loginBtn')">
          <el-input v-model="ed.selectedRegion.data.login_btn_text" @input="ed.onCodeChange" />
        </el-form-item>
        <el-form-item :label="t('portal.registerBtn')">
          <el-input v-model="ed.selectedRegion.data.register_btn_text" @input="ed.onCodeChange" />
        </el-form-item>
        <el-form-item :label="t('portal.emailPlaceholder')">
          <el-input v-model="ed.selectedRegion.data.email_placeholder" @input="ed.onCodeChange" />
        </el-form-item>
        <el-form-item :label="t('portal.passwordPlaceholder')">
          <el-input v-model="ed.selectedRegion.data.password_placeholder" @input="ed.onCodeChange" />
        </el-form-item>
        <el-form-item :label="t('portal.invitePlaceholder')">
          <el-input v-model="ed.selectedRegion.data.invite_placeholder" @input="ed.onCodeChange" />
        </el-form-item>

        <el-divider content-position="left">{{ t('portal.footerArea') }}</el-divider>
        <el-form-item :label="t('portal.footerText')">
          <el-input v-model="ed.selectedRegion.data.footer_text" @input="ed.onCodeChange" />
        </el-form-item>

        <el-divider content-position="left">{{ t('portal.jsCodingSpec') }}</el-divider>
        <el-alert type="info" :closable="false" show-icon>
          <span style="font-size: 11px">
            <b>{{ t('portal.jsObjectLiteral') }}</b>：<code>{ customTitle: '...' }</code><br>
            <b>{{ t('portal.jsReturnStatement') }}</b>：<code>return { customTitle: '...' }</code><br>
            <b>{{ t('portal.jsFullFunction') }}</b>：<code>console.log('init'); return {}</code><br>
            <b>{{ t('portal.jsAvailableCtx') }}</b>：loginForm, registerForm, authTab, loading, handleLogin, handleRegister, t, data, ElMessage
          </span>
        </el-alert>
      </el-form>
    </div>

    <!-- Navigation Properties -->
    <div v-else-if="ed.selectedNavId && ed.selectedNav" class="props-panel">
      <h3 class="props-title">{{ t('portal.editNav') }}</h3>
      <el-form label-position="top" size="small">
        <el-form-item :label="t('portal.displayName')"><el-input v-model="ed.selectedNav.label" :placeholder="t('portal.displayNamePlaceholder')" @input="ed.onCodeChange" /></el-form-item>
        <el-form-item :label="t('portal.navIcon')"><el-input v-model="ed.selectedNav.icon" :placeholder="t('portal.navIconPlaceholder')" @input="ed.onCodeChange" /></el-form-item>
        <el-form-item :label="t('portal.pageTarget')"><el-input v-model="ed.selectedNav.target" placeholder="auto" disabled /></el-form-item>
        <el-form-item :label="t('portal.externalLink')"><el-input v-model="ed.selectedNav.href" :placeholder="t('portal.externalLinkPlaceholder')" @input="ed.onCodeChange" /></el-form-item>
        <el-form-item :label="t('portal.openInNewWindow')"><el-switch v-model="ed.selectedNav.open_external" @change="ed.onCodeChange" /></el-form-item>
        <el-form-item :label="t('portal.showNav')"><el-switch v-model="ed.selectedNav.enabled" @change="ed.onCodeChange" /></el-form-item>
      </el-form>
    </div>

    <!-- Module Properties -->
    <div v-else-if="ed.selectedModuleId && ed.selectedModule" class="props-panel">
      <h3 class="props-title">{{ t('portal.editModule', { type: ed.typeLabel(ed.selectedModule.type) }) }}</h3>
      <el-form label-position="top" size="small">
        <el-form-item :label="t('portal.moduleTitle')"><el-input v-model="ed.selectedModule.title" :placeholder="t('portal.moduleTitle')" @input="ed.onCodeChange" /></el-form-item>
        <el-form-item :label="t('portal.showModule')"><el-switch v-model="ed.selectedModule.visible" @change="ed.onCodeChange" /></el-form-item>

        <!-- Hero Properties -->
        <template v-if="ed.selectedModule.type === 'hero'">
          <el-form-item :label="t('portal.heroEyebrow')"><el-input v-model="ed.selectedModule.data.eyebrow" :placeholder="t('portal.heroEyebrowPlaceholder')" /></el-form-item>
          <el-form-item :label="t('portal.heroHeading')"><el-input v-model="ed.selectedModule.data.heading" :placeholder="t('portal.heroHeadingPlaceholder')" /></el-form-item>
          <el-form-item :label="t('portal.heroDescription')"><el-input v-model="ed.selectedModule.data.description" type="textarea" :rows="2" /></el-form-item>
          <el-form-item :label="t('portal.heroButtonText')"><el-input v-model="ed.selectedModule.data.button_text" :placeholder="t('portal.heroButtonPlaceholder')" /></el-form-item>
          <el-form-item :label="t('portal.heroButtonTarget')">
            <el-select v-model="ed.selectedModule.data.button_target" style="width: 100%">
              <el-option :label="t('portal.targetOverview')" value="overview" /><el-option :label="t('portal.targetSubscription')" value="subscription" /><el-option :label="t('portal.targetSecurity')" value="security" /><el-option :label="t('portal.targetLogs')" value="logs" />
            </el-select>
          </el-form-item>
          <el-form-item :label="t('portal.alignment')">
            <el-radio-group v-model="ed.selectedModule.style.align">
              <el-radio value="left">{{ t('portal.alignLeft') }}</el-radio><el-radio value="center">{{ t('portal.alignCenter') }}</el-radio>
            </el-radio-group>
          </el-form-item>
          <el-form-item :label="t('portal.styleVariant')">
            <el-select v-model="ed.selectedModule.style.variant" style="width: 100%">
              <el-option :label="t('portal.variantPrimary')" value="primary" /><el-option :label="t('portal.variantGradient')" value="gradient" /><el-option :label="t('portal.variantMinimal')" value="minimal" />
            </el-select>
          </el-form-item>
        </template>

        <!-- Markdown / Rich Text Properties (editor is in center column) -->
        <template v-if="ed.selectedModule.type === 'markdown' || ed.selectedModule.type === 'rich_text'">
          <el-form-item :label="t('portal.alignment')">
            <el-radio-group v-model="ed.selectedModule.style.align">
              <el-radio value="left">{{ t('portal.alignLeft') }}</el-radio><el-radio value="center">{{ t('portal.alignCenter') }}</el-radio>
            </el-radio-group>
          </el-form-item>
        </template>

        <!-- Notice Properties -->
        <template v-if="ed.selectedModule.type === 'notice'">
          <el-form-item :label="t('portal.statusColor')">
            <el-select v-model="ed.selectedModule.style.variant" style="width: 100%">
              <el-option :label="t('portal.statusInfo')" value="info" /><el-option :label="t('portal.statusSuccess')" value="success" /><el-option :label="t('portal.statusWarning')" value="warning" /><el-option :label="t('portal.statusError')" value="error" />
            </el-select>
          </el-form-item>
          <el-divider content-position="left">{{ t('portal.popupSettings') }}</el-divider>
          <el-form-item :label="t('portal.displayMode')">
            <el-radio-group v-model="ed.selectedModule.data.display_mode">
              <el-radio value="inline">{{ t('portal.modeInline') }}</el-radio>
              <el-radio value="popup">{{ t('portal.modePopup') }}</el-radio>
            </el-radio-group>
          </el-form-item>
          <template v-if="ed.selectedModule.data.display_mode === 'popup'">
            <el-form-item :label="t('portal.popupTrigger')">
              <el-radio-group v-model="ed.selectedModule.data.popup_trigger">
                <el-radio value="on_load">{{ t('portal.triggerOnLoad') }}</el-radio>
                <el-radio value="manual">{{ t('portal.triggerManual') }}</el-radio>
              </el-radio-group>
            </el-form-item>
            <el-form-item :label="t('portal.popupDismiss')">
              <el-radio-group v-model="ed.selectedModule.data.popup_dismiss">
                <el-radio value="once">{{ t('portal.dismissOnce') }}</el-radio>
                <el-radio value="always">{{ t('portal.dismissAlways') }}</el-radio>
              </el-radio-group>
            </el-form-item>
            <el-form-item :label="t('portal.popupWidth')">
              <el-select v-model="ed.selectedModule.data.popup_width" style="width: 100%">
                <el-option :label="t('portal.widthNarrow')" value="360px" />
                <el-option :label="t('portal.widthStandard')" value="480px" />
                <el-option :label="t('portal.widthWide')" value="640px" />
                <el-option :label="t('portal.widthExtraWide')" value="800px" />
              </el-select>
            </el-form-item>
            <el-form-item :label="t('portal.popupCloseBtn')">
              <el-input v-model="ed.selectedModule.data.popup_button_text" :placeholder="t('portal.popupCloseBtnPlaceholder')" />
            </el-form-item>
            <el-form-item :label="t('portal.popupShowClose')">
              <el-switch v-model="ed.selectedModule.data.popup_show_close" :active-value="true" :inactive-value="false" />
            </el-form-item>
            <el-divider content-position="left">{{ t('portal.popupLinkSection') }}</el-divider>
            <el-form-item :label="t('portal.heroButtonText')">
              <el-input v-model="ed.selectedModule.data.popup_link_text" :placeholder="t('portal.popupLinkTextPlaceholder')" />
            </el-form-item>
            <el-form-item :label="t('portal.popupLinkUrl')">
              <el-input v-model="ed.selectedModule.data.popup_link_url" placeholder="https://example.com" />
            </el-form-item>
            <el-form-item v-if="ed.selectedModule.data.popup_link_url" :label="t('portal.openInNewWindow')">
              <el-switch v-model="ed.selectedModule.data.popup_link_external" :active-value="true" :inactive-value="false" />
            </el-form-item>
          </template>
          <el-alert v-if="ed.selectedModule.data.display_mode === 'popup'" type="info" :closable="false" show-icon style="margin-top: 4px">
            <span style="font-size: 12px">{{ t('portal.noticePopupHint') }}</span>
          </el-alert>
        </template>

        <!-- Feature Cards Properties -->
        <template v-if="ed.selectedModule.type === 'feature_cards'">
          <div v-for="(card, ci) in ed.selectedModule.data.cards" :key="ci" class="card-edit-row">
            <el-input v-model="card.icon" :placeholder="t('portal.cardIcon')" style="width: 60px" />
            <el-input v-model="card.title" :placeholder="t('portal.cardTitle')" />
            <el-input v-model="card.description" :placeholder="t('portal.cardDesc')" />
            <el-button text type="danger" :icon="Delete" @click="ed.selectedModule.data.cards.splice(ci, 1)" />
          </div>
          <el-button text :icon="Plus" @click="ed.selectedModule.data.cards.push({ icon: '⭐', title: '', description: '' })">{{ t('portal.addCard') }}</el-button>
        </template>

        <!-- Account Stats Properties -->
        <template v-if="ed.selectedModule.type === 'account_stats'">
          <el-form-item :label="t('portal.layout')">
            <el-radio-group v-model="ed.selectedModule.style.layout">
              <el-radio value="grid">{{ t('portal.layoutGrid') }}</el-radio><el-radio value="row">{{ t('portal.layoutRow') }}</el-radio>
            </el-radio-group>
          </el-form-item>
        </template>

        <!-- Subscription Properties -->
        <template v-if="ed.selectedModule.type === 'subscription'">
          <el-form-item :label="t('portal.heroHeading')"><el-input v-model="ed.selectedModule.data.title" :placeholder="t('portal.subInfoPlaceholder')" /></el-form-item>
          <el-form-item :label="t('portal.description')"><el-input v-model="ed.selectedModule.data.description" type="textarea" :rows="2" /></el-form-item>
          <el-form-item :label="t('portal.heroButtonText')"><el-input v-model="ed.selectedModule.data.button_text" :placeholder="t('portal.copySubLinkPlaceholder')" /></el-form-item>
        </template>

        <!-- Custom HTML Properties (editor is in center column) -->
        <template v-if="ed.selectedModule.type === 'custom_html'">
          <el-alert type="info" :closable="false" show-icon style="margin-bottom: 8px">
            <span style="font-size: 12px">{{ t('portal.codeEditorAreaHint') }}</span>
          </el-alert>
        </template>

        <!-- Subscription Detail Properties -->
        <template v-if="ed.selectedModule.type === 'subscription_detail'">
          <el-form-item :label="t('portal.description')"><el-input v-model="ed.selectedModule.data.description" type="textarea" :rows="2" :placeholder="t('portal.subDetailDescPlaceholder')" /></el-form-item>
          <el-form-item :label="t('portal.copyBtnText')"><el-input v-model="ed.selectedModule.data.copy_text" :placeholder="t('portal.copyPlaceholder')" /></el-form-item>
          <el-form-item :label="t('portal.qrBtnText')"><el-input v-model="ed.selectedModule.data.qr_text" :placeholder="t('portal.qrPlaceholder')" /></el-form-item>
          <el-form-item :label="t('portal.showSubStatus')"><el-switch v-model="ed.selectedModule.data.show_status" /></el-form-item>
          <el-form-item :label="t('portal.showRotateToken')"><el-switch v-model="ed.selectedModule.data.show_rotate" /></el-form-item>
        </template>

        <!-- Security Settings Properties -->
        <template v-if="ed.selectedModule.type === 'security_settings'">
          <el-form-item :label="t('portal.description')"><el-input v-model="ed.selectedModule.data.description" type="textarea" :rows="2" :placeholder="t('portal.securityDescPlaceholder')" /></el-form-item>
          <el-form-item :label="t('portal.showIpWhitelist')"><el-switch v-model="ed.selectedModule.data.show_ip_whitelist" /></el-form-item>
          <el-form-item :label="t('portal.showChangePassword')"><el-switch v-model="ed.selectedModule.data.show_password" /></el-form-item>
        </template>

        <!-- Log Packages Properties -->
        <template v-if="ed.selectedModule.type === 'log_packages'">
          <el-form-item :label="t('portal.description')"><el-input v-model="ed.selectedModule.data.description" type="textarea" :rows="2" :placeholder="t('portal.logPkgDescPlaceholder')" /></el-form-item>
          <el-form-item :label="t('portal.btn1h')"><el-input v-model="ed.selectedModule.data.btn_1h" :placeholder="t('portal.btn1hPlaceholder')" /></el-form-item>
          <el-form-item :label="t('portal.btn6h')"><el-input v-model="ed.selectedModule.data.btn_6h" :placeholder="t('portal.btn6hPlaceholder')" /></el-form-item>
          <el-form-item :label="t('portal.btn24h')"><el-input v-model="ed.selectedModule.data.btn_24h" :placeholder="t('portal.btn24hPlaceholder')" /></el-form-item>
        </template>

        <!-- Plan List Properties -->
        <template v-if="ed.selectedModule.type === 'plan_list'">
          <el-form-item :label="t('portal.description')"><el-input v-model="ed.selectedModule.data.description" type="textarea" :rows="2" :placeholder="t('portal.optionalPlaceholder')" /></el-form-item>
          <el-form-item :label="t('portal.showPrice')"><el-switch v-model="ed.selectedModule.data.show_price" /></el-form-item>
          <el-form-item :label="t('portal.showQuota')"><el-switch v-model="ed.selectedModule.data.show_quota" /></el-form-item>
          <el-form-item :label="t('portal.buyBtnText')"><el-input v-model="ed.selectedModule.data.buy_text" :placeholder="t('portal.buyBtnPlaceholder')" /></el-form-item>
          <el-form-item :label="t('portal.renewBtnText')"><el-input v-model="ed.selectedModule.data.renew_text" :placeholder="t('portal.renewBtnPlaceholder')" /></el-form-item>
          <el-form-item :label="t('portal.columns')"><el-input-number v-model="ed.selectedModule.data.columns" :min="1" :max="4" /></el-form-item>
        </template>

        <!-- Order Create Properties -->
        <template v-if="ed.selectedModule.type === 'order_create'">
          <el-form-item :label="t('portal.description')"><el-input v-model="ed.selectedModule.data.description" type="textarea" :rows="2" /></el-form-item>
        </template>

        <!-- Charge Panel Properties -->
        <template v-if="ed.selectedModule.type === 'charge_panel'">
          <el-form-item :label="t('portal.description')"><el-input v-model="ed.selectedModule.data.description" type="textarea" :rows="2" /></el-form-item>
          <el-form-item :label="t('portal.showChargeRecords')"><el-switch v-model="ed.selectedModule.data.show_orders" /></el-form-item>
          <el-form-item :label="t('portal.recordsLimit')"><el-input-number v-model="ed.selectedModule.data.orders_limit" :min="1" :max="20" /></el-form-item>
          <el-form-item :label="t('portal.cardPlaceholder')"><el-input v-model="ed.selectedModule.data.card_placeholder" :placeholder="t('portal.cardPlaceholderText')" /></el-form-item>
          <el-form-item :label="t('portal.redeemBtnText')"><el-input v-model="ed.selectedModule.data.card_btn_text" :placeholder="t('portal.redeemPlaceholder')" /></el-form-item>
        </template>

        <!-- Balance Display Properties -->
        <template v-if="ed.selectedModule.type === 'balance_display'">
          <el-form-item :label="t('portal.showTransactions')"><el-switch v-model="ed.selectedModule.data.show_transactions" /></el-form-item>
          <el-form-item :label="t('portal.txLimit')"><el-input-number v-model="ed.selectedModule.data.tx_limit" :min="1" :max="50" /></el-form-item>
          <el-form-item :label="t('portal.chargeBtnText2')"><el-input v-model="ed.selectedModule.data.charge_btn_text" :placeholder="t('portal.chargeBtnPlaceholder')" /></el-form-item>
        </template>

        <!-- Referral Properties -->
        <template v-if="ed.selectedModule.type === 'referral'">
          <el-form-item :label="t('portal.description')"><el-input v-model="ed.selectedModule.data.description" type="textarea" :rows="2" /></el-form-item>
          <el-form-item :label="t('portal.copyBtnText')"><el-input v-model="ed.selectedModule.data.copy_btn_text" :placeholder="t('portal.copyInvitePlaceholder')" /></el-form-item>
          <el-form-item :label="t('portal.showInviteRecords')"><el-switch v-model="ed.selectedModule.data.show_records" /></el-form-item>
          <el-form-item :label="t('portal.recordsLimit')"><el-input-number v-model="ed.selectedModule.data.records_limit" :min="1" :max="50" /></el-form-item>
        </template>

        <!-- Ticket List Properties -->
        <template v-if="ed.selectedModule.type === 'ticket_list'">
          <el-form-item :label="t('portal.newTicketBtn')"><el-input v-model="ed.selectedModule.data.new_btn_text" :placeholder="t('portal.newTicketPlaceholder')" /></el-form-item>
        </template>

        <!-- Article List Properties -->
        <template v-if="ed.selectedModule.type === 'article_list'">
          <el-form-item :label="t('portal.showCategoryFilter')"><el-switch v-model="ed.selectedModule.data.show_category_filter" /></el-form-item>
          <el-form-item :label="t('portal.defaultCategory')">
            <el-select v-model="ed.selectedModule.data.default_category" style="width: 100%">
              <el-option :label="t('portal.catAll')" value="" /><el-option :label="t('portal.catTutorial')" value="tutorial" /><el-option :label="t('portal.catFaq')" value="faq" /><el-option :label="t('portal.catNotice')" value="notice" />
            </el-select>
          </el-form-item>
        </template>

        <!-- Shared Accounts Properties -->
        <template v-if="ed.selectedModule.type === 'shared_accounts'">
          <el-form-item :label="t('portal.description')"><el-input v-model="ed.selectedModule.data.description" type="textarea" :rows="2" /></el-form-item>
          <el-form-item :label="t('portal.viewCredBtn')"><el-input v-model="ed.selectedModule.data.apply_btn_text" :placeholder="t('portal.viewCredPlaceholder')" /></el-form-item>
          <el-form-item :label="t('portal.repairBtn')"><el-input v-model="ed.selectedModule.data.repair_btn_text" :placeholder="t('portal.repairBtnPlaceholder')" /></el-form-item>
        </template>

        <!-- Types without configurable props -->
        <el-alert v-if="!hasPropsForm" type="info" :closable="false" show-icon>
          <span style="font-size: 12px">{{ t('portal.noPropsHint') }}</span>
        </el-alert>
      </el-form>
    </div>

    <!-- No Selection -->
    <div v-else class="props-empty">
      <span class="props-empty-icon">⚙️</span>
      <p>{{ t('portal.selectToEdit') }}</p>
    </div>
  </div>
</template>

<script setup>
import { inject, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Plus, Delete } from '@element-plus/icons-vue'

const { t } = useI18n()
const ed = inject('portalEditor')

const TYPES_WITH_PROPS = [
  'hero', 'markdown', 'rich_text', 'notice', 'feature_cards', 'account_stats',
  'subscription', 'custom_html', 'subscription_detail', 'security_settings',
  'log_packages', 'plan_list', 'order_create', 'charge_panel', 'balance_display',
  'referral', 'ticket_list', 'article_list', 'shared_accounts', 'freeform',
]

const hasPropsForm = computed(() => {
  if (!ed.selectedModule) return true
  if (TYPES_WITH_PROPS.includes(ed.selectedModule.type)) return true
  return false
})
</script>

<style scoped>
.col-properties {
  width: 280px;
  background: #fff;
  overflow-y: auto;
  border-left: 1px solid #e8e8e8;
  flex-shrink: 0;
  padding: 16px;
}
.props-title { font-size: 14px; font-weight: 600; margin-bottom: 16px; color: #303133; }
.props-empty { text-align: center; padding: 40px 20px; color: #909399; }
.props-empty-icon { font-size: 32px; display: block; margin-bottom: 8px; }
.card-edit-row { display: flex; gap: 4px; margin-bottom: 8px; }

@media (max-width: 1400px) {
  .col-properties { width: 260px; }
}
@media (max-width: 1100px) {
  .col-properties { width: 100%; max-height: 250px; }
}
</style>
