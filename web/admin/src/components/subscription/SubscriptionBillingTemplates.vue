<template>
  <div v-loading="loading">
    <!-- Variables Section (Top) -->
    <el-card style="margin-bottom: 16px">
      <template #header>
        <div style="display: flex; align-items: center; justify-content: space-between; cursor: pointer" @click="varsCollapsed = !varsCollapsed">
          <span>{{ t('subscription.billingTemplateVariables') }}</span>
          <el-icon style="font-size: 14px; color: #909399">
            <ArrowDown v-if="varsCollapsed" />
            <ArrowUp v-else />
          </el-icon>
        </div>
      </template>
      <el-table v-show="!varsCollapsed" :data="variableDescriptions" size="small" stripe>
        <el-table-column prop="var" label="Variable" width="200">
          <template #default="{ row }">
            <code style="font-size: 12px; background: #f5f7fa; padding: 2px 6px; border-radius: 3px">{{ row.var }}</code>
          </template>
        </el-table-column>
        <el-table-column prop="desc" :label="t('subscription.billingTemplateVarDesc')" />
      </el-table>
    </el-card>

    <el-tabs v-model="activeScope">
      <!-- Default Templates -->
      <el-tab-pane :label="t('subscription.billingTemplateDefault')" name="default">
        <el-card>
          <el-form :model="defaultForm" label-width="140px">
            <el-form-item :label="t('subscription.billingUsedTemplate')">
              <el-input v-model="defaultForm.billing_used_template" :placeholder="defaultTemplates.default_used_template" />
            </el-form-item>
            <el-form-item :label="t('subscription.billingRemainingTemplate')">
              <el-input v-model="defaultForm.billing_remaining_template" :placeholder="defaultTemplates.default_remaining_template" />
            </el-form-item>
            <el-form-item :label="t('subscription.billingResetTemplate')">
              <el-input v-model="defaultForm.billing_reset_template" :placeholder="defaultTemplates.default_reset_template" />
            </el-form-item>
            <el-form-item :label="t('subscription.billingExpiryTemplate')">
              <el-input v-model="defaultForm.billing_expiry_template" :placeholder="defaultTemplates.default_expiry_template" />
            </el-form-item>
          </el-form>
          <div class="actions">
            <el-button type="primary" :loading="savingDefault" @click="saveDefault">{{ t('common.save') }}</el-button>
          </div>
        </el-card>
      </el-tab-pane>

      <!-- Plan Override -->
      <el-tab-pane :label="t('subscription.billingTemplatePlan')" name="plan">
        <div class="plan-layout">
          <el-card class="plan-list-card">
            <template #header>
              <div class="plan-list-header">
                <span>{{ t('subscription.billingTemplatePlanList') }}</span>
                <el-select v-model="selectedPlanId" filterable style="width: 180px" :placeholder="t('subscription.billingTemplatePlanPlaceholder')" @change="selectPlanById">
                  <el-option v-for="plan in plans" :key="plan.id" :label="plan.name" :value="plan.id" />
                </el-select>
              </div>
            </template>
            <el-table :data="plans" height="420" highlight-current-row @current-change="handlePlanRowChange">
              <el-table-column prop="name" :label="t('subscription.billingTemplatePlan')" min-width="160" />
              <el-table-column :label="t('common.status')" width="120">
                <template #default="{ row }">
                  <el-tag :type="planHasCustomTemplate(row) ? 'warning' : 'info'" size="small">
                    {{ planHasCustomTemplate(row) ? t('subscription.billingTemplateCustom') : t('subscription.billingTemplateInherited') }}
                  </el-tag>
                </template>
              </el-table-column>
            </el-table>
          </el-card>
          <el-card class="plan-editor-card">
            <template #header>
              <div class="plan-editor-header">
                <span>{{ currentPlan?.name || t('subscription.billingTemplatePlan') }}</span>
                <el-tag v-if="currentPlan" :type="planHasCustomTemplate(currentPlan) ? 'warning' : 'info'" size="small">
                  {{ planHasCustomTemplate(currentPlan) ? t('subscription.billingTemplateCustom') : t('subscription.billingTemplateInherited') }}
                </el-tag>
              </div>
            </template>
            <el-empty v-if="!currentPlan" :description="t('subscription.billingTemplateNoPlanSelected')" />
            <template v-else>
              <el-form :model="planForm" label-width="140px">
                <el-form-item :label="t('subscription.billingUsedTemplate')">
                  <el-input v-model="planForm.billing_used_template" :placeholder="defaultTemplates.default_used_template" />
                </el-form-item>
                <el-form-item :label="t('subscription.billingRemainingTemplate')">
                  <el-input v-model="planForm.billing_remaining_template" :placeholder="defaultTemplates.default_remaining_template" />
                </el-form-item>
                <el-form-item :label="t('subscription.billingResetTemplate')">
                  <el-input v-model="planForm.billing_reset_template" :placeholder="defaultTemplates.default_reset_template" />
                </el-form-item>
                <el-form-item :label="t('subscription.billingExpiryTemplate')">
                  <el-input v-model="planForm.billing_expiry_template" :placeholder="defaultTemplates.default_expiry_template" />
                </el-form-item>
                <el-form-item :label="t('subscription.billingInboundNodes')">
                  <el-select v-model="planForm.billing_inbound_config_ids" multiple clearable filterable style="width: 100%" :placeholder="t('subscription.billingNodesPlaceholder')" :loading="nodeOptionsLoading">
                    <el-option v-for="opt in inboundOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
                  </el-select>
                </el-form-item>
                <el-form-item :label="t('subscription.billingExternalNodes')">
                  <el-select v-model="planForm.billing_external_node_ids" multiple clearable filterable style="width: 100%" :placeholder="t('subscription.billingNodesPlaceholder')" :loading="nodeOptionsLoading">
                    <el-option v-for="opt in externalOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
                  </el-select>
                  <div class="hint">{{ t('subscription.billingNodesHint') }}</div>
                </el-form-item>
              </el-form>
              <div class="actions">
                <el-button @click="resetPlanForm">{{ t('subscription.billingTemplateUseDefault') }}</el-button>
                <el-button type="primary" :loading="savingPlan" @click="savePlan">{{ t('common.save') }}</el-button>
              </div>
            </template>
          </el-card>
        </div>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup>
import { computed, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { ArrowDown, ArrowUp } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'
import i18n from '../../locales'
import client from '../../api/client'

const emit = defineEmits(['saved'])

const { t, locale } = useI18n()

const activeScope = ref('default')
const loading = ref(false)
const savingDefault = ref(false)
const savingPlan = ref(false)
const varsCollapsed = ref(true)
const plans = ref([])
const selectedPlanId = ref(null)
const nodeOptionsLoading = ref(false)
const inboundConfigs = ref([])
const externalNodes = ref([])

const defaultTemplates = ref({
  default_used_template: '',
  default_remaining_template: '',
  default_reset_template: '',
  default_expiry_template: '',
  variables: []
})
const defaultForm = ref({
  billing_used_template: '',
  billing_remaining_template: '',
  billing_reset_template: '',
  billing_expiry_template: ''
})
const planForm = ref({
  billing_used_template: '',
  billing_remaining_template: '',
  billing_reset_template: '',
  billing_expiry_template: '',
  billing_inbound_config_ids: [],
  billing_external_node_ids: []
})

const variableDescriptions = computed(() => {
  const vars = defaultTemplates.value.variables || []
  const isZh = String(i18n.global.locale.value).startsWith('zh')
  const descMap = isZh ? {
    remaining_human: '剩余用量（人类可读，如 7.00 GB）',
    remaining_bytes: '剩余用量（字节）',
    remaining_gb: '剩余用量（GB 数值）',
    used_human: '已用流量（人类可读）',
    used_bytes: '已用流量（字节）',
    used_gb: '已用流量（GB 数值）',
    quota_human: '总配额（人类可读）',
    quota_bytes: '总配额（字节）',
    quota_gb: '总配额（GB 数值）',
    reset_human: '重置倒计时（人类可读，如 15 Days）',
    reset_days: '距下次重置天数',
    reset_at: '下次重置时间（ISO 格式）',
    reset_date: '下次重置日期（2006-01-02）',
    expires_human: '到期时间（人类可读）',
    expires_at: '到期时间（ISO 格式）',
    expires_date: '到期日期（2006-01-02）',
    expires_unix: '到期时间（Unix 时间戳）',
    quota_state: '配额状态（normal/near_limit/over_limit 等）',
  } : {
    remaining_human: 'Remaining usage (human readable, e.g. 7.00 GB)',
    remaining_bytes: 'Remaining usage (bytes)',
    remaining_gb: 'Remaining usage (GB value)',
    used_human: 'Used traffic (human readable)',
    used_bytes: 'Used traffic (bytes)',
    used_gb: 'Used traffic (GB value)',
    quota_human: 'Total quota (human readable)',
    quota_bytes: 'Total quota (bytes)',
    quota_gb: 'Total quota (GB value)',
    reset_human: 'Reset countdown (human readable, e.g. 15 Days)',
    reset_days: 'Days until next reset',
    reset_at: 'Next reset time (ISO format)',
    reset_date: 'Next reset date (2006-01-02)',
    expires_human: 'Expiration time (human readable)',
    expires_at: 'Expiration time (ISO format)',
    expires_date: 'Expiration date (2006-01-02)',
    expires_unix: 'Expiration time (Unix timestamp)',
    quota_state: 'Quota state (normal/near_limit/over_limit etc.)',
  }
  
  return vars.map(v => {
    const key = v.replace(/[{}]/g, '')
    return {
      var: v,
      desc: descMap[key] || v
    }
  })
})

const inboundOptions = computed(() => {
  return inboundConfigs.value
    .filter(c => c.enabled !== false)
    .map(c => ({
      value: c.id,
      label: `${c.remark || c.tag} (${c.protocol}:${c.port})`
    }))
})

const externalOptions = computed(() => {
  return externalNodes.value
    .filter(n => n.enabled !== false)
    .map(n => ({
      value: n.id,
      label: `${n.name} (${n.host}:${n.port})`
    }))
})

const currentPlan = computed(() => plans.value.find(plan => plan.id === selectedPlanId.value) || null)

function currentLang() {
  return String(locale.value || '').startsWith('zh') ? 'zh' : 'en'
}

function applyDefaultForm() {
  defaultForm.value = {
    billing_used_template: defaultTemplates.value.default_used_template || '',
    billing_remaining_template: defaultTemplates.value.default_remaining_template || '',
    billing_reset_template: defaultTemplates.value.default_reset_template || '',
    billing_expiry_template: defaultTemplates.value.default_expiry_template || ''
  }
}

function planHasCustomTemplate(plan) {
  return !!(
    plan?.billing_used_template?.trim() ||
    plan?.billing_remaining_template?.trim() ||
    plan?.billing_reset_template?.trim() ||
    plan?.billing_expiry_template?.trim() ||
    (plan?.billing_inbound_config_ids && plan.billing_inbound_config_ids.length > 0) ||
    (plan?.billing_external_node_ids && plan.billing_external_node_ids.length > 0)
  )
}

function selectPlanById(planId) {
  const plan = plans.value.find(item => item.id === planId) || null
  planForm.value = {
    billing_used_template: plan?.billing_used_template || '',
    billing_remaining_template: plan?.billing_remaining_template || '',
    billing_reset_template: plan?.billing_reset_template || '',
    billing_expiry_template: plan?.billing_expiry_template || '',
    billing_inbound_config_ids: Array.isArray(plan?.billing_inbound_config_ids) ? [...plan.billing_inbound_config_ids] : [],
    billing_external_node_ids: Array.isArray(plan?.billing_external_node_ids) ? [...plan.billing_external_node_ids] : []
  }
}

function handlePlanRowChange(plan) {
  if (!plan) return
  selectedPlanId.value = plan.id
  selectPlanById(plan.id)
}

function resetPlanForm() {
  planForm.value = {
    billing_used_template: '',
    billing_remaining_template: '',
    billing_reset_template: '',
    billing_expiry_template: '',
    billing_inbound_config_ids: [],
    billing_external_node_ids: []
  }
}

async function loadNodeOptions() {
  if (inboundConfigs.value.length && externalNodes.value.length) return
  nodeOptionsLoading.value = true
  try {
    const [inboundRes, extRes] = await Promise.all([
      inboundConfigs.value.length ? Promise.resolve(inboundConfigs.value) : client.get('/inbound-configs').catch(() => []),
      externalNodes.value.length ? Promise.resolve(externalNodes.value) : client.get('/external-nodes').catch(() => [])
    ])
    const inbounds = Array.isArray(inboundRes) ? inboundRes : (inboundRes?.data || [])
    inboundConfigs.value = inbounds
    const exts = Array.isArray(extRes) ? extRes : []
    externalNodes.value = exts
  } finally {
    nodeOptionsLoading.value = false
  }
}

async function loadData() {
  loading.value = true
  try {
    const [templateRes, planRes] = await Promise.all([
      client.get('/system/subscription-billing-templates', { params: { lang: currentLang() } }),
      client.get('/plans')
    ])
    defaultTemplates.value = {
      default_used_template: templateRes?.default_used_template || '',
      default_remaining_template: templateRes?.default_remaining_template || '',
      default_reset_template: templateRes?.default_reset_template || '',
      default_expiry_template: templateRes?.default_expiry_template || '',
      variables: Array.isArray(templateRes?.variables) ? templateRes.variables : []
    }
    plans.value = Array.isArray(planRes) ? planRes : []
    applyDefaultForm()
    if (!selectedPlanId.value && plans.value.length > 0) {
      selectedPlanId.value = plans.value[0].id
    }
    selectPlanById(selectedPlanId.value)
    await loadNodeOptions()
  } finally {
    loading.value = false
  }
}

async function saveDefault() {
  savingDefault.value = true
  try {
    await client.put('/system/subscription-billing-templates', {
      default_used_template: defaultForm.value.billing_used_template,
      default_remaining_template: defaultForm.value.billing_remaining_template,
      default_reset_template: defaultForm.value.billing_reset_template,
      default_expiry_template: defaultForm.value.billing_expiry_template
    })
    defaultTemplates.value = {
      ...defaultTemplates.value,
      default_used_template: defaultForm.value.billing_used_template,
      default_remaining_template: defaultForm.value.billing_remaining_template,
      default_reset_template: defaultForm.value.billing_reset_template,
      default_expiry_template: defaultForm.value.billing_expiry_template
    }
    ElMessage.success(t('common.success'))
    emit('saved')
  } finally {
    savingDefault.value = false
  }
}

async function savePlan() {
  if (!selectedPlanId.value) return
  savingPlan.value = true
  try {
    await client.put(`/plans/${selectedPlanId.value}`, { ...planForm.value })
    const target = plans.value.find(item => item.id === selectedPlanId.value)
    if (target) {
      Object.assign(target, planForm.value)
    }
    ElMessage.success(t('common.success'))
    emit('saved')
  } finally {
    savingPlan.value = false
  }
}

loadData()
</script>

<style scoped>
.plan-layout {
  display: grid;
  grid-template-columns: 320px minmax(0, 1fr);
  gap: 16px;
}

.plan-list-card,
.plan-editor-card {
  min-height: 520px;
}

.plan-list-header,
.plan-editor-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.hint {
  font-size: 12px;
  color: #909399;
  margin-top: 4px;
  word-break: break-word;
}
</style>
