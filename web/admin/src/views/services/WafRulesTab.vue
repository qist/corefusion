<template>
  <div>
    <!-- GitHub Source Config -->
    <el-card style="margin-bottom:20px">
      <template #header>
        <div style="display:flex;justify-content:space-between;align-items:center">
          <span style="font-weight:600">{{ t('proxy.wafGitHubSource') }}</span>
          <el-button size="small" type="primary" @click="saveGitHubSource" :loading="sourceSaving">{{ t('common.save') }}</el-button>
        </div>
      </template>
      <el-form label-width="120px" size="small">
        <el-form-item label="URL">
          <el-input v-model="githubSource.url" :placeholder="t('proxy.wafGitHubUrlHint')" />
          <div style="font-size:11px;color:#909399;margin-top:4px;line-height:1.6">
            {{ t('proxy.wafGitHubUrlDesc') }}
          </div>
        </el-form-item>
        <el-form-item :label="t('proxy.wafSyncCron')">
          <el-select v-model="githubSource.sync_cron" filterable allow-create style="width:100%">
            <el-option :label="t('common.disabled')" value="" />
            <el-option label="@every 1h" value="@every 1h" />
            <el-option label="@every 6h" value="@every 6h" />
            <el-option label="@every 12h" value="@every 12h" />
            <el-option label="@every 24h" value="@every 24h" />
            <el-option :label="t('service.rules.cronDaily3am')" value="0 3 * * *" />
            <el-option :label="t('service.rules.cronDailyMidnight')" value="0 0 * * *" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('proxy.wafAutoSync')">
          <el-switch v-model="githubSource.enabled" />
          <span style="font-size:11px;color:#909399;margin-left:8px">{{ t('proxy.wafAutoSyncDesc') }}</span>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- Action Bar -->
    <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:16px">
      <h3 style="margin:0">{{ t('proxy.wafGlobalRules') }}</h3>
      <div style="display:flex;gap:8px">
        <el-button size="small" @click="syncFromGitHub" :loading="syncing">
          <el-icon><Refresh /></el-icon> GitHub {{ t('common.refresh') }}
        </el-button>
        <el-button size="small" type="success" @click="pushToAllNodes" :loading="pushing">
          <el-icon><Promotion /></el-icon> {{ t('proxy.wafPushAllNodes') }}
        </el-button>
      </div>
    </div>

    <!-- Rules Table -->
    <el-card>
      <el-alert type="info" :closable="false" style="margin-bottom:12px">
        <div style="font-size:12px;line-height:1.6">
          <b>{{ t('proxy.wafMergeModeDesc') }}</b><br/>
          • <b>Replace</b>: {{ t('proxy.wafMergeReplaceDesc') }}<br/>
          • <b>Append</b>: {{ t('proxy.wafMergeAppendDesc') }}
        </div>
      </el-alert>
      <el-table :data="globalRules" stripe style="width:100%" @row-click="openRuleEditor" class="rules-table">
        <el-table-column prop="name" :label="t('proxy.wafRule')" width="180" />
        <el-table-column :label="t('proxy.wafMergeMode')" width="120">
          <template #default="{ row }">
            <el-tag :type="row.merge_mode === 'append' ? 'warning' : 'info'" size="small">
              {{ row.merge_mode || 'replace' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="t('proxy.wafRuleDesc')" min-width="200">
          <template #default="{ row }">{{ wafRuleDesc(row.name) }}</template>
        </el-table-column>
        <el-table-column :label="t('proxy.wafSize')" width="100">
          <template #default="{ row }">{{ row.content ? row.content.length + ' B' : '-' }}</template>
        </el-table-column>
        <el-table-column :label="t('proxy.wafHash')" width="120">
          <template #default="{ row }">
            <span style="font-family:monospace;font-size:11px;color:#909399">
              {{ row.hash ? row.hash.substring(0, 12) + '...' : '-' }}
            </span>
          </template>
        </el-table-column>
        <el-table-column prop="updated_at" :label="t('common.updatedAt')" width="180">
          <template #default="{ row }">{{ formatTime(row.updated_at) }}</template>
        </el-table-column>
        <el-table-column :label="t('common.actions')" width="100" fixed="right">
          <template #default="{ row }">
            <el-button size="small" text @click.stop="openRuleEditor(row)">{{ t('common.edit') }}</el-button>
            <el-button size="small" text type="danger" @click.stop="deleteRule(row)">{{ t('common.delete') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Rule Editor Dialog -->
    <el-dialog v-model="ruleEditorVisible" :title="`WAF Rule: ${editingRule.name}`" width="70%" @close="onEditorClose">
      <div style="margin-bottom:8px;display:flex;align-items:center;gap:12px">
        <span style="font-size:13px;color:#606266">Merge Mode:</span>
        <el-radio-group v-model="editingRule.merge_mode" size="small">
          <el-radio-button label="replace">Replace</el-radio-button>
          <el-radio-button label="append">Append</el-radio-button>
        </el-radio-group>
        <span v-if="editingRule.merge_mode === 'append'" style="font-size:11px;color:#909399">
          Node rules will be appended to this global rule
        </span>
        <span v-else style="font-size:11px;color:#909399">
          Node rules will replace this global rule entirely
        </span>
      </div>
      <el-alert v-if="editingRule.name === 'cdnip.rule'" type="info" :closable="false" style="margin-bottom:8px">
        <template #title>CDN/Trusted Proxy IP List</template>
        <div style="font-size:12px;line-height:1.6">
          Only IPs listed here are trusted for X-Forwarded-For / X-Real-IP headers (when <code>trust_proxy_headers=on</code>).<br/>
          Supports: IPv4 CIDR, IPv6 CIDR, wildcards, exact IPs.<br/>
          <b>Cloudflare:</b> <a href="https://www.cloudflare.com/ips/" target="_blank">https://www.cloudflare.com/ips/</a>
        </div>
      </el-alert>
      <el-input
        v-model="editingRule.content"
        type="textarea"
        :rows="20"
        style="font-family:monospace;font-size:13px"
        placeholder="Rule content..."
      />
      <template #footer>
        <el-button @click="ruleEditorVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="saveRule" :loading="saving">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { Refresh, Promotion } from '@element-plus/icons-vue'
import {
  listWafRules, getWafRule, saveWafRule, deleteWafRule,
  syncWafFromGitHub, getWafGitHubSource, saveWafGitHubSource, pushWafToAllNodes,
} from '../../api/waf'

const { t } = useI18n()

const globalRules = ref([])
const syncing = ref(false)
const pushing = ref(false)
const saving = ref(false)
const sourceSaving = ref(false)
const ruleEditorVisible = ref(false)
const editingRule = ref({ name: '', content: '', merge_mode: 'replace' })
const githubSource = ref({ url: '', sync_cron: '', enabled: false })

const ruleDescriptions = {
  'args.rule': 'URL argument rules',
  'url.rule': 'URL path rules',
  'cookie.rule': 'Cookie rules',
  'post.rule': 'POST body rules',
  'referer.rule': 'Referer rules',
  'useragent.rule': 'User-Agent rules',
  'fileext.rule': 'File extension rules',
  'whiteua.rule': 'Whitelist User-Agent rules',
  'blackip.rule': 'Blacklist IP rules (user-defined)',
  'whiteip.rule': 'Whitelist IP rules (user-defined)',
  'whiteurl.rule': 'Whitelist URL rules (user-defined)',
  'cdnip.rule': 'CDN/Trusted Proxy IP list',
}

function wafRuleDesc(name) {
  return ruleDescriptions[name] || 'Custom rule'
}

function formatTime(ts) {
  if (!ts) return '-'
  const d = new Date(ts)
  return d.toLocaleString()
}

async function loadRules() {
  try {
    const data = await listWafRules({ scope: 'global' })
    globalRules.value = data || []
  } catch (e) {
    // handled by interceptor
  }
}

async function loadGitHubSource() {
  try {
    const data = await getWafGitHubSource()
    githubSource.value = {
      url: data.url || '',
      sync_cron: data.sync_cron || '',
      enabled: data.enabled || false,
    }
  } catch (e) {
    // handled
  }
}

async function saveGitHubSource() {
  sourceSaving.value = true
  try {
    await saveWafGitHubSource(githubSource.value.url, githubSource.value.sync_cron, githubSource.value.enabled)
    ElMessage.success(t('common.saved'))
  } catch (e) {
    // handled
  } finally {
    sourceSaving.value = false
  }
}

async function openRuleEditor(row) {
  try {
    const data = await getWafRule(row.name)
    editingRule.value = {
      name: row.name,
      content: data.content || '',
      merge_mode: row.merge_mode || 'replace',
    }
    ruleEditorVisible.value = true
  } catch (e) {
    // handled
  }
}

function onEditorClose() {
  editingRule.value = { name: '', content: '', merge_mode: 'replace' }
}

async function saveRule() {
  saving.value = true
  try {
    await saveWafRule(editingRule.value.name, editingRule.value.content, editingRule.value.merge_mode)
    ElMessage.success(t('common.saved'))
    ruleEditorVisible.value = false
    await loadRules()
  } catch (e) {
    // handled
  } finally {
    saving.value = false
  }
}

async function deleteRule(row) {
  try {
    await ElMessageBox.confirm(`Delete rule "${row.name}"?`, 'Warning', { type: 'warning' })
    await deleteWafRule(row.name)
    ElMessage.success(t('common.deleted'))
    await loadRules()
  } catch (e) {
    // cancelled or error
  }
}

async function syncFromGitHub() {
  syncing.value = true
  try {
    await syncWafFromGitHub()
    ElMessage.success('Synced from GitHub & pushed to nodes')
    await loadRules()
  } catch (e) {
    // handled
  } finally {
    syncing.value = false
  }
}

async function pushToAllNodes() {
  pushing.value = true
  try {
    await pushWafToAllNodes()
    ElMessage.success(t('proxy.wafPushSuccess'))
  } catch (e) {
    // handled
  } finally {
    pushing.value = false
  }
}

onMounted(() => {
  loadRules()
  loadGitHubSource()
})
</script>

<style scoped>
.rules-table {
  cursor: pointer;
}
</style>
