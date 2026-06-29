<template>
  <div v-loading="loading">
    <el-tabs v-model="activeScope">
      <el-tab-pane :label="t('subscription.billingTemplateDefault')" name="default">
        <el-card>
          <el-form :model="defaultForm" label-width="120px">
            <el-form-item :label="t('subscription.billingRemainingTemplate')">
              <el-input v-model="defaultForm.billing_remaining_template" type="textarea" :rows="2" :placeholder="defaultTemplates.default_remaining_template" />
              <div class="hint">{{ t('subscription.billingTemplateDefaultHint') }}</div>
            </el-form-item>
            <el-form-item :label="t('subscription.billingResetTemplate')">
              <el-input v-model="defaultForm.billing_reset_template" type="textarea" :rows="2" :placeholder="defaultTemplates.default_reset_template" />
            </el-form-item>
            <el-form-item :label="t('subscription.billingExpiryTemplate')">
              <el-input v-model="defaultForm.billing_expiry_template" type="textarea" :rows="2" :placeholder="defaultTemplates.default_expiry_template" />
            </el-form-item>
            <el-form-item :label="t('subscription.billingTemplateVariables')">
              <div class="hint">{{ variablesText }}</div>
            </el-form-item>
          </el-form>
          <div class="actions">
            <el-button type="primary" :loading="savingDefault" @click="saveDefault">{{ t('common.save') }}</el-button>
          </div>
        </el-card>
      </el-tab-pane>
      <el-tab-pane :label="t('subscription.billingTemplatePlan')" name="plan">
        <div class="plan-layout">
          <el-card class="plan-list-card">
            <template #header>
              <div class="plan-list-header">
                <span>{{ t('subscription.billingTemplatePlanList') }}</span>
                <el-select v-model="selectedPlanId" filterable style="width:180px" :placeholder="t('subscription.billingTemplatePlanPlaceholder')" @change="selectPlanById">
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
            <el-form v-else :model="planForm" label-width="120px">
              <el-form-item :label="t('subscription.billingRemainingTemplate')">
                <el-input v-model="planForm.billing_remaining_template" type="textarea" :rows="2" :placeholder="defaultTemplates.default_remaining_template" />
                <div class="hint">{{ t('subscription.billingTemplateFallbackHint') }}</div>
              </el-form-item>
              <el-form-item :label="t('subscription.billingResetTemplate')">
                <el-input v-model="planForm.billing_reset_template" type="textarea" :rows="2" :placeholder="defaultTemplates.default_reset_template" />
              </el-form-item>
              <el-form-item :label="t('subscription.billingExpiryTemplate')">
                <el-input v-model="planForm.billing_expiry_template" type="textarea" :rows="2" :placeholder="defaultTemplates.default_expiry_template" />
              </el-form-item>
              <el-form-item :label="t('subscription.billingTemplateVariables')">
                <div class="hint">{{ variablesText }}</div>
              </el-form-item>
            </el-form>
            <div v-if="currentPlan" class="actions">
              <el-button @click="resetPlanForm">{{ t('subscription.billingTemplateUseDefault') }}</el-button>
              <el-button type="primary" :loading="savingPlan" @click="savePlan">{{ t('common.save') }}</el-button>
            </div>
          </el-card>
        </div>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup>
import { computed, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import client from '../../api/client'

const emit = defineEmits(['saved'])

const { t, locale } = useI18n()

const activeScope = ref('default')
const loading = ref(false)
const savingDefault = ref(false)
const savingPlan = ref(false)
const plans = ref([])
const selectedPlanId = ref(null)
const defaultTemplates = ref({
  default_remaining_template: '',
  default_reset_template: '',
  default_expiry_template: '',
  variables: []
})
const defaultForm = ref({
  billing_remaining_template: '',
  billing_reset_template: '',
  billing_expiry_template: ''
})
const planForm = ref({
  billing_remaining_template: '',
  billing_reset_template: '',
  billing_expiry_template: ''
})

const variablesText = computed(() => (defaultTemplates.value.variables || []).join(', '))
const currentPlan = computed(() => plans.value.find(plan => plan.id === selectedPlanId.value) || null)

function currentLang() {
  return String(locale.value || '').startsWith('zh') ? 'zh' : 'en'
}

function applyDefaultForm() {
  defaultForm.value = {
    billing_remaining_template: defaultTemplates.value.default_remaining_template || '',
    billing_reset_template: defaultTemplates.value.default_reset_template || '',
    billing_expiry_template: defaultTemplates.value.default_expiry_template || ''
  }
}

function planHasCustomTemplate(plan) {
  return !!(
    plan?.billing_remaining_template?.trim() ||
    plan?.billing_reset_template?.trim() ||
    plan?.billing_expiry_template?.trim()
  )
}

function selectPlanById(planId) {
  const plan = plans.value.find(item => item.id === planId) || null
  planForm.value = {
    billing_remaining_template: plan?.billing_remaining_template || '',
    billing_reset_template: plan?.billing_reset_template || '',
    billing_expiry_template: plan?.billing_expiry_template || ''
  }
}

function handlePlanRowChange(plan) {
  if (!plan) return
  selectedPlanId.value = plan.id
  selectPlanById(plan.id)
}

function resetPlanForm() {
  planForm.value = {
    billing_remaining_template: '',
    billing_reset_template: '',
    billing_expiry_template: ''
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
  } finally {
    loading.value = false
  }
}

async function saveDefault() {
  savingDefault.value = true
  try {
    await client.put('/system/subscription-billing-templates', {
      default_remaining_template: defaultForm.value.billing_remaining_template,
      default_reset_template: defaultForm.value.billing_reset_template,
      default_expiry_template: defaultForm.value.billing_expiry_template
    })
    defaultTemplates.value = {
      ...defaultTemplates.value,
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
