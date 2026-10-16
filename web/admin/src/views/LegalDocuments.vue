<template>
  <div class="legal-page" v-loading="loading">
    <div class="legal-head">
      <div class="legal-head-text">
        <h2>{{ t('legal.title') }}</h2>
        <p>{{ t('legal.subtitle') }}</p>
      </div>
      <el-button type="primary" :loading="saving" :disabled="!dirty" @click="save">{{ t('common.save') }}</el-button>
    </div>

    <el-card shadow="never" class="legal-card">
      <!-- 文档类型 -->
      <el-radio-group v-model="activeDoc" class="legal-doc-tabs">
        <el-radio-button value="privacy">{{ t('legal.privacyTab') }}</el-radio-button>
        <el-radio-button value="terms">{{ t('legal.termsTab') }}</el-radio-button>
      </el-radio-group>

      <!-- 语言栏 -->
      <div class="legal-lang-bar">
        <span class="legal-lang-label">{{ t('legal.editLocale') }}</span>
        <el-select
          v-model="editLocale"
          size="small"
          filterable
          allow-create
          default-first-option
          style="width: 180px"
          @change="onLocaleChange"
        >
          <el-option v-for="c in locales" :key="c" :label="c" :value="c" />
        </el-select>
        <el-button size="small" plain type="danger" :disabled="locales.length <= 1" @click="removeLocale">
          {{ t('legal.removeLocale') }}
        </el-button>
        <span class="legal-lang-hint">{{ t('legal.localeHint') }}</span>
      </div>

      <!-- 编辑 / 预览 -->
      <div class="legal-editor-wrap">
        <div class="legal-editor-col">
          <div class="legal-col-label">{{ t('legal.editor') }}</div>
          <CodeEditor v-model="currentContent" language="markdown" height="520px" @change="dirty = true" />
        </div>
        <div class="legal-preview-col">
          <div class="legal-col-label">{{ t('legal.preview') }}</div>
          <div class="legal-preview md" v-html="previewHtml"></div>
        </div>
      </div>

      <!-- 链接 -->
      <el-divider content-position="left">{{ t('legal.pageLinks') }}</el-divider>
      <div class="legal-links">
        <div class="legal-link-row">
          <span class="legal-link-label">{{ t('legal.privacyTab') }}</span>
          <el-input
            v-model="docs.privacy_policy_url"
            size="small"
            :placeholder="t('legal.externalUrlPlaceholder')"
            style="max-width: 360px"
            @input="dirty = true"
          />
          <span class="legal-builtin-label">{{ t('legal.builtinLink') }}</span>
          <code>{{ builtinPrivacyUrl }}</code>
          <el-button text size="small" @click="copy(builtinPrivacyUrl)">{{ t('portal.copyLink') }}</el-button>
        </div>
        <div class="legal-link-row">
          <span class="legal-link-label">{{ t('legal.termsTab') }}</span>
          <el-input
            v-model="docs.terms_of_service_url"
            size="small"
            :placeholder="t('legal.externalUrlPlaceholder')"
            style="max-width: 360px"
            @input="dirty = true"
          />
          <span class="legal-builtin-label">{{ t('legal.builtinLink') }}</span>
          <code>{{ builtinTermsUrl }}</code>
          <el-button text size="small" @click="copy(builtinTermsUrl)">{{ t('portal.copyLink') }}</el-button>
        </div>
        <div class="legal-links-hint">{{ t('legal.pageLinksHint') }}</div>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import { getActivePortal, updateActivePortal } from '../api/portal'
import CodeEditor from '../components/common/CodeEditor.vue'

const { t } = useI18n()

const loading = ref(false)
const saving = ref(false)
const dirty = ref(false)

const activeDoc = ref('privacy')
const editLocale = ref('zh-CN')
const locales = ref(['zh-CN'])
// 门户对外地址（只读，来自后端）：门户配置了域名用门户域名，否则回退当前 origin
const portalExternalUrl = ref('')
const portalPagePath = ref('/p')
const portalPort = ref('')

const docs = reactive({
  privacy_policy_url: '',
  terms_of_service_url: '',
  privacy_policy_content: {},
  terms_of_service_content: {},
})

// 当前文档对应的内容字段名（按语言 markdown：{"zh-CN":"...","en":"..."}）
const docKey = computed(() =>
  activeDoc.value === 'privacy' ? 'privacy_policy_content' : 'terms_of_service_content',
)

const currentContent = computed({
  get: () => docs[docKey.value][editLocale.value] || '',
  set: (v) => {
    docs[docKey.value][editLocale.value] = v
  },
})

const previewHtml = computed(() => {
  const md = currentContent.value
  if (!md) return `<p class="legal-empty">${t('legal.placeholder')}</p>`
  return DOMPurify.sanitize(marked.parse(md))
})

// 门户对外基址（含挂载路径）：门户配置了域名 → 用门户域名；否则回退当前面板 origin。
// portal_page_path：独立门户端口为 ""（根路径，不回退 /p），否则 "/p"。
const portalPublicBase = computed(() => {
  const path = portalPagePath.value === undefined || portalPagePath.value === null ? '/p' : portalPagePath.value
  const ext = (portalExternalUrl.value || '').replace(/\/$/, '')
  if (ext) return ext + path
  // 未配置门户域名：独立门户端口 → 当前主机 + 门户端口（根路径）；否则回退当前 origin + 路径
  const pp = String(portalPort.value || '').trim()
  if (pp && pp !== '0') return window.location.protocol + '//' + window.location.hostname + ':' + pp
  return window.location.origin + path
})
const builtinPrivacyUrl = computed(() => portalPublicBase.value + '/legal/privacy')
const builtinTermsUrl = computed(() => portalPublicBase.value + '/legal/terms')

function recomputeLocales() {
  const set = new Set()
  Object.keys(docs.privacy_policy_content || {}).forEach((c) => set.add(c))
  Object.keys(docs.terms_of_service_content || {}).forEach((c) => set.add(c))
  if (!set.has('zh-CN')) set.add('zh-CN')
  locales.value = Array.from(set)
}

function onLocaleChange(code) {
  if (!code) return
  if (!locales.value.includes(code)) locales.value.push(code)
  if (!(code in docs[docKey.value])) docs[docKey.value][code] = ''
}

async function removeLocale() {
  const code = editLocale.value
  try {
    await ElMessageBox.confirm(t('legal.removeLocaleConfirm', { code }), t('common.confirm'), {
      type: 'warning',
    })
  } catch {
    return
  }
  delete docs.privacy_policy_content[code]
  delete docs.terms_of_service_content[code]
  recomputeLocales()
  editLocale.value = locales.value[0]
  dirty.value = true
}

function copy(text) {
  navigator.clipboard
    .writeText(text)
    .then(() => ElMessage.success(t('portal.copiedToClipboard')))
    .catch(() => ElMessage.error(t('portal.copyFailed')))
}

async function load() {
  loading.value = true
  try {
    const d = await getActivePortal()
    docs.privacy_policy_url = d.privacy_policy_url || ''
    docs.terms_of_service_url = d.terms_of_service_url || ''
    docs.privacy_policy_content = d.privacy_policy_content || {}
    docs.terms_of_service_content = d.terms_of_service_content || {}
    portalExternalUrl.value = d.portal_external_url || ''
    portalPort.value = d.portal_port || ''
    portalPagePath.value = d.portal_page_path === undefined || d.portal_page_path === null ? '/p' : d.portal_page_path
    recomputeLocales()
    editLocale.value = locales.value[0]
    dirty.value = false
  } catch {
    ElMessage.error(t('legal.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function save() {
  saving.value = true
  try {
    await updateActivePortal({
      privacy_policy_url: docs.privacy_policy_url || '',
      terms_of_service_url: docs.terms_of_service_url || '',
      privacy_policy_content: docs.privacy_policy_content || {},
      terms_of_service_content: docs.terms_of_service_content || {},
    })
    dirty.value = false
    ElMessage.success(t('legal.saved'))
  } catch (e) {
    ElMessage.error(e.response?.data?.message || t('legal.saveFailed'))
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.legal-page { padding: 16px; max-width: 1200px; margin: 0 auto; }
.legal-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; margin-bottom: 16px; }
.legal-head-text h2 { margin: 0 0 4px; font-size: 20px; color: #303133; }
.legal-head-text p { margin: 0; font-size: 13px; color: #909399; }
.legal-card :deep(.el-card__body) { padding: 20px; }
.legal-doc-tabs { margin-bottom: 16px; }
.legal-lang-bar { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; margin-bottom: 14px; }
.legal-lang-label { font-size: 13px; color: #606266; }
.legal-lang-hint { font-size: 12px; color: #a8abb2; }
.legal-editor-wrap { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }
.legal-editor-col, .legal-preview-col { min-width: 0; }
.legal-col-label { font-size: 12px; font-weight: 600; color: #909399; text-transform: uppercase; letter-spacing: .5px; margin-bottom: 6px; }
.legal-preview {
  height: 520px; overflow: auto; padding: 18px 22px;
  border: 1px solid #dcdfe6; border-radius: 6px; background: #fff;
  font-size: 14px; line-height: 1.75; color: #374151;
}
.legal-preview :deep(h1) { font-size: 22px; margin: 0 0 14px; color: #111827; }
.legal-preview :deep(h2) { font-size: 17px; margin: 22px 0 8px; color: #111827; }
.legal-preview :deep(h3) { font-size: 15px; margin: 18px 0 6px; }
.legal-preview :deep(p) { margin: 8px 0; }
.legal-preview :deep(ul), .legal-preview :deep(ol) { padding-left: 22px; margin: 8px 0; }
.legal-preview :deep(blockquote) { margin: 10px 0; padding: 4px 12px; border-left: 3px solid #4f46e5; background: #f5f6ff; color: #4b5563; }
.legal-preview :deep(code) { background: rgba(17, 24, 39, .06); padding: 2px 6px; border-radius: 4px; font-size: 13px; }
.legal-preview :deep(a) { color: #4f46e5; }
.legal-preview :deep(.legal-empty) { color: #c0c4cc; }
.legal-links { display: flex; flex-direction: column; gap: 10px; }
.legal-link-row { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.legal-link-label { flex: none; width: 64px; font-size: 13px; color: #606266; }
.legal-builtin-label { font-size: 12px; color: #a8abb2; }
.legal-link-row code {
  font-size: 12px; color: #4f46e5; background: #f5f6ff; border: 1px solid #e5e7ff;
  border-radius: 6px; padding: 3px 8px; white-space: nowrap; overflow-x: auto; max-width: 420px;
}
.legal-links-hint { font-size: 12px; color: #909399; line-height: 1.6; }
@media (max-width: 900px) {
  .legal-editor-wrap { grid-template-columns: 1fr; }
}
</style>
