<template>
  <div class="settings-mode">
    <el-tabs v-model="ed.settingsTab" type="border-card">
      <el-tab-pane :label="t('portal.basicSettingsTab')" name="basic">
        <el-form :model="ed.form" label-width="140px" style="max-width: 700px; padding: 20px 0">
          <el-form-item :label="t('portal.siteName')"><el-input v-model="ed.form.name" :placeholder="t('portal.siteNamePlaceholder')" @input="ed.onCodeChange" /></el-form-item>
          <el-form-item :label="t('portal.siteDescription')"><el-input v-model="ed.form.description" type="textarea" :rows="2" @input="ed.onCodeChange" /></el-form-item>
          <el-form-item :label="t('portal.openRegistration')"><el-switch v-model="ed.registrationEnabled" @change="ed.onCodeChange" /></el-form-item>
          <el-form-item :label="t('portal.portalPath')"><el-input value="/p" disabled /></el-form-item>
          <el-form-item :label="t('portal.portalPort')"><el-input v-model="ed.portalPort" :placeholder="t('portal.portalPortPlaceholder')" @input="ed.onCodeChange" /></el-form-item>
          <el-divider content-position="left">{{ t('portal.legalDocsTab') }}</el-divider>
          <el-form-item :label="t('portal.privacyPolicyUrl')"><el-input v-model="ed.config.privacy_policy_url" placeholder="https://..." @input="ed.onCodeChange" /></el-form-item>
          <el-form-item :label="t('portal.privacyPolicyContent')"><el-input v-model="ed.config.privacy_policy_content" type="textarea" :rows="4" :placeholder="t('portal.legalContentPlaceholder')" @input="ed.onCodeChange" /></el-form-item>
          <el-form-item :label="t('portal.termsUrl')"><el-input v-model="ed.config.terms_of_service_url" placeholder="https://..." @input="ed.onCodeChange" /></el-form-item>
          <el-form-item :label="t('portal.termsContent')"><el-input v-model="ed.config.terms_of_service_content" type="textarea" :rows="4" :placeholder="t('portal.legalContentPlaceholder')" @input="ed.onCodeChange" /></el-form-item>
          <div style="font-size:11px;color:#909399;margin:-8px 0 8px 140px">{{ t('portal.legalDocsHint') }}</div>
        </el-form>
      </el-tab-pane>
      <el-tab-pane :label="t('portal.appearanceTab')" name="appearance">
        <el-form :model="ed.config" label-width="140px" style="max-width: 700px; padding: 20px 0">
          <el-form-item :label="t('portal.siteTitle')"><el-input v-model="ed.config.site_name" placeholder="CoreFusion" @input="ed.onCodeChange" /></el-form-item>
          <el-form-item :label="t('portal.logoUrl')"><el-input v-model="ed.config.logo_url" placeholder="https://..." @input="ed.onCodeChange" /></el-form-item>
          <el-form-item :label="t('portal.primaryColor')"><div class="color-row"><el-color-picker v-model="ed.config.primary_color" @change="ed.onCodeChange" /><el-input v-model="ed.config.primary_color" style="width: 120px" @input="ed.onCodeChange" /></div></el-form-item>
          <el-form-item :label="t('portal.backgroundColor')"><div class="color-row"><el-color-picker v-model="ed.config.background_color" @change="ed.onCodeChange" /><el-input v-model="ed.config.background_color" style="width: 120px" @input="ed.onCodeChange" /></div></el-form-item>
          <el-form-item :label="t('portal.cardBgColor')"><div class="color-row"><el-color-picker v-model="ed.config.card_background_color" @change="ed.onCodeChange" /><el-input v-model="ed.config.card_background_color" style="width: 120px" @input="ed.onCodeChange" /></div></el-form-item>
          <el-form-item :label="t('portal.textColor')"><div class="color-row"><el-color-picker v-model="ed.config.text_color" @change="ed.onCodeChange" /><el-input v-model="ed.config.text_color" style="width: 120px" @input="ed.onCodeChange" /></div></el-form-item>
          <el-form-item :label="t('portal.accentColor')"><div class="color-row"><el-color-picker v-model="ed.config.accent_color" @change="ed.onCodeChange" /><el-input v-model="ed.config.accent_color" style="width: 120px" @input="ed.onCodeChange" /></div></el-form-item>
          <el-form-item :label="t('portal.fontFamily')"><el-input v-model="ed.config.font_family" @input="ed.onCodeChange" /></el-form-item>
        </el-form>
      </el-tab-pane>
      <el-tab-pane :label="t('portal.apiRefTab')" name="api">
        <div class="api-ref-container">
          <el-alert type="info" :closable="false" show-icon style="margin-bottom: 8px">
            <span style="font-size: 13px">{{ t('portal.apiRefHint') }}</span>
          </el-alert>
          <el-alert type="success" :closable="false" show-icon style="margin-bottom: 16px">
            <span style="font-size: 13px">{{ t('portal.apiFullRef') }}
              <a href="https://github.com/qist/corefusion/blob/main/docs/api/portal-api.md" target="_blank" rel="noopener" style="color:#409eff">docs/api/portal-api.md</a>
            </span>
          </el-alert>
          <el-collapse v-model="ed.activeApiNames" class="api-collapse">
            <el-collapse-item v-for="(group, gi) in ed.apiGroups" :key="gi" :name="gi" :title="group.title">
              <div v-for="(api, ai) in group.apis" :key="ai" class="api-item">
                <div class="api-item-header">
                  <span class="api-method" :class="'method-' + api.method.toLowerCase()">{{ api.method }}</span>
                  <code class="api-path">{{ api.path }}</code>
                  <span class="api-desc">{{ api.desc }}</span>
                </div>
                <div v-if="api.auth" class="api-auth">
                  <span class="api-auth-label">{{ t('portal.authLabel') }}</span>
                  <span class="api-auth-value">{{ api.auth }}</span>
                </div>
                <div v-if="api.requestBody" class="api-section-block">
                  <div class="api-block-label">{{ t('portal.requestExample') }}</div>
                  <div class="api-copy-bar">
                    <el-button text size="small" :icon="CopyDocument" @click="ed.copyText(api.requestBody)">{{ t('common.copy') }}</el-button>
                  </div>
                  <pre class="api-code-block"><code>{{ api.requestBody }}</code></pre>
                </div>
                <div v-if="api.response" class="api-section-block">
                  <div class="api-block-label">{{ t('portal.responseExample') }}</div>
                  <pre class="api-code-block"><code>{{ api.response }}</code></pre>
                </div>
              </div>
            </el-collapse-item>
          </el-collapse>
        </div>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup>
import { inject } from 'vue'
import { useI18n } from 'vue-i18n'
import { CopyDocument } from '@element-plus/icons-vue'

const { t } = useI18n()
const ed = inject('portalEditor')
</script>

<style scoped>
.settings-mode { padding: 16px; overflow-y: auto; height: 100%; }
.color-row { display: flex; align-items: center; gap: 8px; }

/* ===== API Reference ===== */
.api-ref-container { padding: 16px 0; max-width: 900px; }
.api-collapse { border: none; }
.api-collapse :deep(.el-collapse-item__header) { font-size: 14px; font-weight: 600; color: #303133; }
.api-collapse :deep(.el-collapse-item__content) { padding-bottom: 0; }
.api-item { border: 1px solid #ebeef5; border-radius: 8px; padding: 12px 16px; margin-bottom: 12px; background: #fafbfc; }
.api-item-header { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.api-method {
  display: inline-block; padding: 2px 8px; border-radius: 4px; font-size: 11px; font-weight: 700;
  color: #fff; min-width: 52px; text-align: center;
}
.method-get { background: #409eff; }
.method-post { background: #67c23a; }
.method-put { background: #e6a23c; }
.method-delete { background: #f56c6c; }
.api-path { font-family: 'SF Mono', 'Fira Code', Consolas, monospace; font-size: 13px; color: #303133; font-weight: 600; }
.api-desc { font-size: 12px; color: #909399; }
.api-auth { font-size: 12px; color: #606266; margin-top: 6px; }
.api-auth-label { font-weight: 600; }
.api-auth-value { font-family: 'SF Mono', 'Fira Code', Consolas, monospace; color: #409eff; }
.api-section-block { margin-top: 8px; }
.api-block-label { font-size: 11px; font-weight: 600; color: #909399; margin-bottom: 4px; text-transform: uppercase; letter-spacing: 0.5px; }
.api-copy-bar { display: flex; justify-content: flex-end; margin: -4px 0 2px; }
.api-code-block {
  background: #1e1e2e; color: #cdd6f4; padding: 12px 16px; border-radius: 6px;
  overflow-x: auto; font-family: 'SF Mono', 'Fira Code', Consolas, monospace;
  font-size: 12px; line-height: 1.6; margin: 0;
}
.api-code-block code { color: inherit; }
</style>
