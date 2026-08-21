<template>
  <div class="waf-rules-page">
    <div class="page-header">
      <div>
        <h2 style="margin:0">WAF {{ t('proxy.wafRules') }}</h2>
        <span style="font-size:12px;color:#909399">{{ t('proxy.wafRuleStoreHint') }}</span>
      </div>
      <div style="display:flex;gap:8px">
        <el-button size="small" @click="syncFromGitHub" :loading="syncing">
          <el-icon><Refresh /></el-icon> GitHub {{ t('common.refresh') }}
        </el-button>
        <el-button size="small" type="primary" @click="syncAll" :loading="syncingAll">
          <el-icon><Promotion /></el-icon> {{ t('proxy.syncAllNodes') }}
        </el-button>
      </div>
    </div>

    <!-- Global Rules Table -->
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

    <!-- Add New Rule -->
    <div style="margin-top:16px;display:flex;gap:8px;align-items:center">
      <el-input v-model="newRuleName" placeholder="custom.rule" style="width:200px" />
      <el-select v-model="newRuleMergeMode" style="width:140px">
        <el-option label="Replace" value="replace" />
        <el-option label="Append" value="append" />
      </el-select>
      <el-button size="small" type="primary" @click="addRule">{{ t('common.add') }}</el-button>
    </div>

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
          Supports: IPv4 CIDR (1.2.3.0/24), IPv6 CIDR (2001:db8::/32), wildcards (192.168.*), exact IPs.<br/>
          Hot-reloaded via mtime check, no restart needed.<br/>
          <b>Major CDN providers reference:</b>
          <ul style="margin:4px 0 0 16px;padding:0">
            <li>Cloudflare: <a href="https://www.cloudflare.com/ips/" target="_blank">https://www.cloudflare.com/ips/</a></li>
            <li>Alibaba CDN: 47.0.0.0/8, 120.0.0.0/8 (approximate, check official)</li>
            <li>Tencent CDN: 101.0.0.0/8, 111.0.0.0/8 (approximate, check official)</li>
            <li>Amazon CloudFront: <a href="https://d7uri8nf7uskq.cloudfront.net/tools/list-cloudfront-ips" target="_blank">CloudFront IP list</a></li>
            <li>Google Cloud: 35.132.0.0/14 (check official)</li>
          </ul>
          This file is <b>not created by default</b>. Only create it when you need CDN trust.
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
import { ref, onMounted, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { Refresh, Promotion } from '@element-plus/icons-vue'
import {
  listWafRules, getWafRule, saveWafRule, deleteWafRule,
  syncWafFromGitHub, syncWafAll,
} from '../api/waf'

const { t } = useI18n()

const globalRules = ref([])
const syncing = ref(false)
const syncingAll = ref(false)
const saving = ref(false)
const ruleEditorVisible = ref(false)
const editingRule = ref({ name: '', content: '', merge_mode: 'replace' })
const newRuleName = ref('')
const newRuleMergeMode = ref('replace')

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
  'cdnip.rule': 'CDN/Trusted Proxy IP list — only these IPs are trusted for X-Forwarded-For',
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
    // Error already handled by interceptor
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
    // handled by interceptor
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
    // handled by interceptor
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

function addRule() {
  if (!newRuleName.value.trim()) {
    ElMessage.warning('Please enter a rule name')
    return
  }
  editingRule.value = {
    name: newRuleName.value.trim(),
    content: '',
    merge_mode: newRuleMergeMode.value,
  }
  newRuleName.value = ''
  ruleEditorVisible.value = true
}

async function syncFromGitHub() {
  syncing.value = true
  try {
    await syncWafFromGitHub()
    ElMessage.success('Synced from GitHub')
    await loadRules()
  } catch (e) {
    // handled
  } finally {
    syncing.value = false
  }
}

async function syncAll() {
  syncingAll.value = true
  try {
    await syncWafAll()
    ElMessage.success('Synced to all nodes')
  } catch (e) {
    // handled
  } finally {
    syncingAll.value = false
  }
}

onMounted(() => {
  loadRules()
})
</script>

<style scoped>
.waf-rules-page {
  padding: 20px;
}
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}
.rules-table {
  cursor: pointer;
}
</style>
