<template>
  <div>
    <el-tabs v-model="activeTab" type="border-card">

      <!-- ════════════ 流量叠加包 Tab ════════════ -->
      <el-tab-pane :label="t('plan.trafficPacksTab')" name="traffic_packs">
        <div style="display:flex;justify-content:space-between;margin-bottom:16px">
          <h3 style="margin:0">{{ t('plan.trafficPacksTab') }}</h3>
          <el-button type="primary" size="small" @click="showCreateTrafficPack">{{ t('plan.createTrafficPack') }}</el-button>
        </div>
        <el-table :data="trafficPackVariants" v-loading="trafficPacksLoading" stripe size="small">
          <el-table-column prop="id" label="ID" width="60" />
          <el-table-column prop="name" :label="t('common.name')" width="150" />
          <el-table-column :label="t('plan.trafficPackDesc')" show-overflow-tooltip>
            <template #default="{row}">{{ row.description || '—' }}</template>
          </el-table-column>
          <el-table-column :label="t('plan.applicableScope')" width="120">
            <template #default=""><el-tag size="small" type="success">{{ t('plan.allPlans') }}</el-tag></template>
          </el-table-column>
          <el-table-column :label="t('plan.price')" width="100">
            <template #default="{row}">¥{{ (row.price/100).toFixed(2) }}</template>
          </el-table-column>
          <el-table-column :label="t('plan.traffic')" width="120">
            <template #default="{row}">{{ row.quota_bytes > 0 ? formatBytes(row.quota_bytes) : '—' }}</template>
          </el-table-column>
          <el-table-column :label="t('plan.expiryMode')" width="160">
            <template #default="{row}">
              <el-tag v-if="row.expiry_mode === 'extend_plan'" type="warning" size="small">{{ t('plan.extendPlan', { days: row.duration_days }) }}</el-tag>
              <el-tag v-else size="small">{{ t('plan.modeSamePlan') }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="sort_order" :label="t('plan.sortOrder')" width="80" />
          <el-table-column :label="t('common.status')" width="80">
            <template #default="{row}">
              <el-tag :type="row.enabled?'success':'danger'" size="small">{{ row.enabled?t('common.enabled'):t('common.disabled') }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column :label="t('common.actions')" width="120">
            <template #default="{row}">
              <el-button size="small" @click="showEditTrafficPack(row)">{{ t('common.edit') }}</el-button>
              <el-button size="small" type="danger" @click="deleteTrafficPack(row)">{{ t('common.delete') }}</el-button>
            </template>
          </el-table-column>
        </el-table>
        <PaginationBar
          v-model:page="trafficPage"
          v-model:page-size="trafficSize"
          :total="trafficTotal"
          @change="fetchTrafficPacks"
        />
      </el-tab-pane>

      <!-- ════════════ 套餐模板 Tab ════════════ -->
      <el-tab-pane :label="t('plan.templatesTab')" name="templates">
        <div style="display:flex;justify-content:space-between;margin-bottom:16px">
          <h3 style="margin:0">{{ t('plan.templatesTab') }}</h3>
          <div style="display:flex;gap:8px">
            <el-button type="warning" size="small" :loading="syncAllLoading" @click="handleSyncAll">{{ t('plan.syncAllAccounts') }}</el-button>
            <el-button type="primary" size="small" @click="showCreateTemplate">{{ t('plan.createTemplate') }}</el-button>
          </div>
        </div>
        <el-row :gutter="16">
          <el-col v-for="tpl in templates" :key="tpl.id" :span="8" style="margin-bottom:16px">
            <el-card shadow="hover">
              <template #header>
                <div style="display:flex;justify-content:space-between;align-items:center">
                  <span style="font-weight:600">{{ tpl.name }}</span>
                  <el-tag :type="tpl.enabled?'success':'danger'" size="small">{{ tpl.enabled?t('common.enabled'):t('common.disabled') }}</el-tag>
                </div>
              </template>
              <div style="font-size:13px;line-height:2;color:#606266">
                <div>{{ t('plan.category') }}: <el-tag size="small" :type="categoryType(tpl.category)">{{ categoryLabel(tpl.category) }}</el-tag></div>
                <div>{{ t('plan.traffic') }}: {{ formatBytes(tpl.default_quota_bytes) }} / {{ tpl.default_quota_reset_cycle || t('common.unlimited') }}</div>
                <div>{{ t('plan.deviceLimit') }}: {{ tpl.device_limit || t('common.unlimited') }} {{ t('plan.unitDevices') }} | {{ t('plan.downloadSpeed') }}: {{ tpl.speed_limit_mbps || t('common.unlimited') }} Mbps | {{ t('plan.uploadSpeed') }}: {{ tpl.speed_limit_up_mbps || t('common.unlimited') }} Mbps</div>
                <div v-if="tpl.is_trial" style="color:#e6a23c">
                  <el-tag type="warning" size="small">{{ t('plan.categoryTrial') }}</el-tag> {{ tpl.trial_duration_hours }}{{ t('plan.unitHours') }}
                </div>
                <div>{{ t('plan.nodeGroupsTab') }}: {{ (tpl.node_group_ids||[]).length }} {{ t('plan.unitCount') }} | {{ t('plan.billingNodes') }}: {{ (tpl.billing_node_ids||[]).length }} {{ t('plan.unitCount') }}</div>
                <div style="color:#909399;font-size:12px">{{ tpl.description }}</div>
              </div>
              <div v-if="templateNodes[tpl.id] && templateNodes[tpl.id].length" style="margin-top:8px;border-top:1px solid #ebeef5;padding-top:8px">
                <div style="font-size:12px;color:#909399;margin-bottom:4px">{{ t('plan.relatedNodes') }}</div>
<div v-for="node in templateNodes[tpl.id].slice(0,5)" :key="node.member_id" style="font-size:12px;line-height:1.8">
<el-tag :type="node.type==='inbound'?'':'success'" size="small" style="margin-right:4px">{{ node.type==='inbound'?t('plan.nodeTypeInbound'):t('plan.nodeTypeExternal') }}</el-tag>
<el-tag v-if="node.has_mirror" type="warning" size="small" style="margin-right:4px">{{ t('plan.mirrored') }}</el-tag>
<span style="color:#409eff">{{ node.server_name }}</span> - {{ node.node_name }}
<span style="color:#909399">({{ node.protocol }}:{{ node.port }})</span>
</div>
                <div v-if="templateNodes[tpl.id].length > 5" style="font-size:12px;color:#909399;margin-top:4px">...{{ t('plan.totalCount') }} {{ templateNodes[tpl.id].length }} {{ t('plan.unitNodes') }}</div>
              </div>
              <div style="margin-top:12px;display:flex;gap:4px;flex-wrap:wrap">
                <el-button size="small" @click="showVariantsDialog(tpl)">{{ t('plan.manageVariants') }}</el-button>
                <el-button size="small" @click="showNodeGroupsDialog(tpl)">{{ t('plan.manageNodes') }}</el-button>
                <el-button size="small" @click="showBillingNodesDialog(tpl)">{{ t('plan.billingNodes') }}</el-button>
                <el-button size="small" type="warning" :loading="syncLoading[tpl.id]" @click="handleSyncTemplate(tpl)">{{ t('plan.syncAccounts') }}</el-button>
                <el-button size="small" @click="showEditTemplate(tpl)">{{ t('common.edit') }}</el-button>
                <el-button size="small" type="danger" @click="deleteTemplate(tpl)">{{ t('common.delete') }}</el-button>
              </div>
            </el-card>
          </el-col>
        </el-row>
      </el-tab-pane>

      <!-- ════════════ 节点分组 Tab ════════════ -->
      <el-tab-pane :label="t('plan.nodeGroupsTab')" name="node-groups">
        <div style="display:flex;justify-content:space-between;margin-bottom:16px">
          <h3 style="margin:0">{{ t('plan.nodeGroupsTab') }}</h3>
          <el-button type="primary" size="small" @click="showCreateGroup">{{ t('plan.createGroup') }}</el-button>
        </div>
        <el-table :data="nodeGroups" v-loading="groupLoading" stripe size="small">
          <el-table-column prop="id" label="ID" width="50" />
          <el-table-column prop="name" :label="t('common.name')" width="150" />
          <el-table-column :label="t('plan.category')" width="100">
            <template #default="{row}">
              <el-tag :type="tierType(row.tier)" size="small">{{ tierLabel(row.tier) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="region" :label="t('plan.region')" width="80" />
          <el-table-column prop="description" :label="t('plan.description')" show-overflow-tooltip />
          <el-table-column :label="t('common.status')" width="70">
            <template #default="{row}">
              <el-tag :type="row.enabled?'success':'danger'" size="small">{{ row.enabled?'✓':'✗' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column :label="t('common.actions')" width="200">
            <template #default="{row}">
              <el-button size="small" @click="showMembersDialog(row)">{{ t('plan.memberManagement') }}</el-button>
              <el-button size="small" @click="showEditGroup(row)">{{ t('common.edit') }}</el-button>
              <el-button size="small" type="danger" @click="deleteGroup(row)">{{ t('common.delete') }}</el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>

      <!-- ════════════ 优惠码 Tab ════════════ -->
      <el-tab-pane :label="t('plan.promotionsTab')" name="promotions">
        <div style="display:flex;justify-content:space-between;margin-bottom:16px">
          <h3 style="margin:0">{{ t('plan.promotions') }}</h3>
          <el-button type="primary" size="small" @click="showCreatePromo">{{ t('plan.createPromotion') }}</el-button>
        </div>
        <el-table :data="promotions" v-loading="promoLoading" stripe size="small">
          <el-table-column prop="id" label="ID" width="50" />
          <el-table-column prop="code" :label="t('plan.promotionsTab')" width="150" />
          <el-table-column prop="name" :label="t('common.name')" width="150" />
          <el-table-column :label="t('common.type')" width="100">
            <template #default="{row}">
              {{ row.discount_type === 'percentage' ? row.discount_value + '%' : '¥' + (row.discount_value/100).toFixed(2) }}
            </template>
          </el-table-column>
          <el-table-column :label="t('plan.applicableScope')" width="120">
            <template #default="{row}">
              <el-tag size="small">{{ scopeLabel(row.applicable_scope) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column :label="t('plan.usageLimit')" width="100">
            <template #default="{row}">{{ row.used_count }}{{ row.max_uses > 0 ? '/' + row.max_uses : '/∞' }}</template>
          </el-table-column>
          <el-table-column :label="t('plan.perUserLimit')" width="90">
            <template #default="{row}">{{ row.max_uses_per_user > 0 ? row.max_uses_per_user : '∞' }}</template>
          </el-table-column>
          <el-table-column :label="t('plan.validPeriod')" width="200">
            <template #default="{row}">
              <span style="font-size:12px">
                {{ row.starts_at?.substring(0,10) || '—' }} ~ {{ row.ends_at?.substring(0,10) || t('plan.permanent') }}
              </span>
            </template>
          </el-table-column>
          <el-table-column :label="t('common.status')" width="70">
            <template #default="{row}">
              <el-tag :type="row.enabled?'success':'danger'" size="small">{{ row.enabled?'✓':'✗' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column :label="t('common.actions')" width="120">
            <template #default="{row}">
              <el-button size="small" @click="showEditPromo(row)">{{ t('common.edit') }}</el-button>
              <el-button size="small" type="danger" @click="deletePromo(row)">{{ t('common.delete') }}</el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>

      <!-- ════════════ 订单记录 Tab ════════════ -->
      <el-tab-pane :label="t('plan.ordersTab')" name="orders">
        <div style="display:flex;justify-content:space-between;margin-bottom:16px">
          <div style="display:flex;gap:8px;align-items:center">
            <h3 style="margin:0">{{ t('plan.ordersTab') }}</h3>
            <el-input v-model="orderClientFilter" size="small" :placeholder="t('plan.clientIdPlaceholder')" style="width:150px" @keyup.enter="fetchOrders" clearable />
            <el-button size="small" @click="fetchOrders">{{ t('common.search') }}</el-button>
          </div>
        </div>
        <el-table :data="orders" v-loading="orderLoading" stripe size="small">
          <el-table-column prop="id" label="ID" width="60" />
          <el-table-column prop="client_id" :label="t('plan.clientId')" width="80" />
          <el-table-column :label="t('common.type')" width="80">
            <template #default="{row}">
              <el-tag :type="orderTypeColor(row.order_type)" size="small">{{ orderTypeLabel(row.order_type) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="to_variant_id" :label="t('plan.variantId')" width="70" />
          <el-table-column :label="t('plan.amount')" width="180">
            <template #default="{row}">
              <span style="font-size:12px">
                ¥{{ (row.paid_amount/100).toFixed(2) }}
                <span v-if="row.discount_amount > 0" style="color:#f56c6c;text-decoration:line-through;font-size:11px">
                  ¥{{ (row.original_amount/100).toFixed(2) }}
                </span>
              </span>
            </template>
          </el-table-column>
          <el-table-column prop="promotion_code" :label="t('plan.promotionsTab')" width="120" />
          <el-table-column :label="t('plan.effectiveTime')" width="160">
            <template #default="{row}">{{ row.effective_from?.substring(0,16).replace('T',' ') }}</template>
          </el-table-column>
          <el-table-column :label="t('plan.expiryTime')" width="160">
            <template #default="{row}">{{ row.effective_to?.substring(0,16).replace('T',' ') || '—' }}</template>
          </el-table-column>
          <el-table-column :label="t('common.status')" width="80">
            <template #default="{row}">
              <el-tag :type="orderStatusColor(row.status)" size="small">{{ orderStatusLabel(row.status) }}</el-tag>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>

    </el-tabs>

    <!-- ═══════════════════════════════════════════════ -->
    <!-- 模板编辑弹窗 -->
    <el-dialog v-model="templateDialog" :title="editingTemplate ? t('plan.editTemplate') : t('plan.createTemplate')" width="650px">
      <el-form :model="templateForm" label-width="120px" size="small">
        <el-divider content-position="left">{{ t('plan.basicInfo') }}</el-divider>
        <el-form-item :label="t('common.name')"><el-input v-model="templateForm.name" /></el-form-item>
        <el-form-item :label="t('plan.description')"><el-input v-model="templateForm.description" type="textarea" :rows="2" /></el-form-item>
        <el-form-item :label="t('plan.category')">
          <el-select v-model="templateForm.category" style="width:100%">
            <el-option :label="t('plan.categoryBasic')" value="basic" />
            <el-option :label="t('plan.categoryStandard')" value="standard" />
            <el-option :label="t('plan.categoryPremium')" value="premium" />
            <el-option :label="t('plan.categoryTrial')" value="trial" />
            <el-option :label="t('plan.categoryCustom')" value="custom" />
          </el-select>
        </el-form-item>
        <el-divider content-position="left">{{ t('plan.features') }}</el-divider>
        <el-form-item :label="t('plan.deviceLimit')">
          <el-input-number v-model="templateForm.device_limit" :min="0" /> <span style="color:#909399;font-size:12px">{{ t('plan.zeroUnlimited') }}</span>
        </el-form-item>
        <el-form-item :label="t('plan.downloadSpeedLimit')">
          <el-input-number v-model="templateForm.speed_limit_mbps" :min="0" /> <span style="color:#909399;font-size:12px">{{ t('plan.zeroUnlimited') }}</span>
        </el-form-item>
        <el-form-item :label="t('plan.uploadSpeedLimit')">
          <el-input-number v-model="templateForm.speed_limit_up_mbps" :min="0" /> <span style="color:#909399;font-size:12px">{{ t('plan.zeroUnlimited') }}</span>
        </el-form-item>
        <el-form-item :label="t('plan.concurrentLimit')">
          <el-input-number v-model="templateForm.concurrent_limit" :min="0" /> <span style="color:#909399;font-size:12px">{{ t('plan.zeroUnlimited') }}</span>
        </el-form-item>
        <el-divider content-position="left">{{ t('plan.quotaDefaults') }}</el-divider>
        <el-form-item :label="t('plan.defaultTraffic')">
          <el-input-number v-model="templateForm.default_quota_gb" :min="0" :step="0.1" /> <span style="color:#909399;font-size:12px">{{ t('plan.zeroUnlimited') }}</span>
        </el-form-item>
        <el-form-item :label="t('plan.quotaResetCycle')">
          <el-select v-model="templateForm.default_quota_reset_cycle" style="width:100%">
            <el-option :label="t('plan.noReset')" value="none" />
            <el-option :label="t('plan.cycleMonthly')" value="monthly" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('plan.suspendOnExhausted')">
          <el-switch v-model="templateForm.suspend_on_quota_exhausted" :active-value="1" :inactive-value="0" />
        </el-form-item>
        <el-divider content-position="left">{{ t('plan.trialConfig') }}</el-divider>
        <el-form-item :label="t('plan.enableTrial')">
          <el-switch v-model="templateForm.is_trial" />
        </el-form-item>
        <template v-if="templateForm.is_trial">
          <el-form-item :label="t('plan.trialDuration')">
            <el-input-number v-model="templateForm.trial_duration_hours" :min="1" />
            <div class="form-hint">{{ t('plan.trialExpiryHint') }}</div>
          </el-form-item>
        </template>
        <el-divider content-position="left">{{ t('plan.displaySettings') }}</el-divider>
        <el-form-item :label="t('plan.sortOrder')"><el-input-number v-model="templateForm.sort_order" :min="0" /></el-form-item>
        <el-form-item :label="t('common.enabled')"><el-switch v-model="templateForm.enabled" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="templateDialog=false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="saveTemplate">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- 规格管理弹窗 -->
    <el-dialog v-model="variantsDialog" :title="currentTemplate?.name + ' - ' + t('plan.manageVariants')" width="750px">
      <div style="margin-bottom:12px;text-align:right">
        <el-button type="primary" size="small" @click="showCreateVariant">{{ t('plan.addVariant') }}</el-button>
      </div>
      <el-table :data="variants" stripe size="small">
        <el-table-column prop="name" :label="t('common.name')" width="80" />
        <el-table-column :label="t('common.type')" width="90">
          <template #default="{row}">
            <el-tag :type="row.variant_type==='traffic_pack'?'warning':''" size="small">{{ row.variant_type==='traffic_pack'?t('plan.trafficPack'):t('plan.planType') }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="t('plan.billingCycle')" width="80">
          <template #default="{row}">{{ cycleLabel(row.billing_cycle) }}</template>
        </el-table-column>
        <el-table-column :label="t('plan.price')" width="100">
          <template #default="{row}">¥{{ (row.price/100).toFixed(2) }}</template>
        </el-table-column>
        <el-table-column :label="t('plan.originalPrice')" width="100">
          <template #default="{row}">{{ row.original_price > 0 ? '¥' + (row.original_price/100).toFixed(2) : '—' }}</template>
        </el-table-column>
        <el-table-column prop="duration_days" :label="t('plan.durationDays')" width="60" />
        <el-table-column :label="t('plan.quota')" width="100">
          <template #default="{row}">{{ row.quota_bytes > 0 ? formatBytes(row.quota_bytes) : t('plan.modeSamePlan') }}</template>
        </el-table-column>
        <el-table-column :label="t('plan.tag')" width="80">
          <template #default="{row}">
            <el-tag v-if="row.is_default" type="success" size="small">{{ t('plan.default') }}</el-tag>
            <el-tag v-else-if="row.is_recommended" type="warning" size="small">{{ t('plan.recommended') }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="t('common.actions')" width="120">
          <template #default="{row}">
            <el-button size="small" @click="showEditVariant(row)">{{ t('common.edit') }}</el-button>
            <el-button size="small" type="danger" @click="deleteVariant(row)">{{ t('common.delete') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-dialog>

    <!-- 规格编辑弹窗 -->
    <el-dialog v-model="variantDialog" :title="editingVariant ? t('plan.editVariant') : t('plan.addVariant')" width="500px" append-to-body>
      <el-form :model="variantForm" label-width="100px" size="small">
        <el-form-item :label="t('plan.variantType')">
          <el-select v-model="variantForm.variant_type" style="width:100%">
            <el-option :label="t('plan.regularPlan')" value="plan" />
            <el-option :label="t('plan.trafficPacksTab')" value="traffic_pack" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('common.name')"><el-input v-model="variantForm.name" :placeholder="t('plan.variantNamePlaceholder')" /></el-form-item>
        <el-form-item :label="t('plan.billingCycle')">
          <el-select v-model="variantForm.billing_cycle" style="width:100%">
            <el-option :label="t('plan.cycleMonthly')" value="monthly" />
            <el-option :label="t('plan.cycleQuarterly')" value="quarterly" />
            <el-option :label="t('plan.cycleSemiAnnual')" value="semi_annual" />
            <el-option :label="t('plan.cycleAnnual')" value="annual" />
            <el-option :label="t('plan.cycleLifetime')" value="lifetime" />
            <el-option :label="t('plan.cycleOneTime')" value="one_time" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('plan.effectiveDays')"><el-input-number v-model="variantForm.duration_days" :min="1" /></el-form-item>
        <el-form-item :label="t('plan.priceYuan')"><el-input-number v-model="variantForm.price_yuan" :min="0" :step="0.01" /></el-form-item>
        <el-form-item :label="t('plan.originalPriceYuan')"><el-input-number v-model="variantForm.original_price_yuan" :min="0" :step="0.01" /></el-form-item>
        <el-form-item :label="t('plan.trafficGB')"><el-input-number v-model="variantForm.quota_gb" :min="0" :step="0.1" /> <span style="font-size:12px;color:#909399">{{ t('plan.inheritTemplate') }}</span></el-form-item>
        <el-form-item :label="t('plan.defaultVariant')"><el-switch v-model="variantForm.is_default" /></el-form-item>
        <el-form-item :label="t('plan.recommended')"><el-switch v-model="variantForm.is_recommended" /></el-form-item>
        <el-form-item :label="t('common.enabled')"><el-switch v-model="variantForm.enabled" /></el-form-item>
        <el-form-item :label="t('plan.sortOrder')"><el-input-number v-model="variantForm.sort_order" :min="0" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="variantDialog=false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="saveVariant">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- 模板节点分组管理弹窗 -->
    <el-dialog v-model="templateNodeGroupsDialog" :title="currentTemplate?.name + ' - ' + t('plan.nodeGroupsTab')" width="500px">
      <el-checkbox-group v-model="selectedNodeGroupIds">
        <div v-for="g in nodeGroups" :key="g.id" style="margin-bottom:8px">
          <el-checkbox :value="g.id" :disabled="!g.enabled">
            {{ g.name }}
            <el-tag :type="tierType(g.tier)" size="small" style="margin-left:4px">{{ tierLabel(g.tier) }}</el-tag>
            <span v-if="g.region" style="color:#909399;font-size:12px;margin-left:4px">{{ g.region }}</span>
          </el-checkbox>
        </div>
      </el-checkbox-group>
      <template #footer>
        <el-button @click="templateNodeGroupsDialog=false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="saveTemplateNodeGroups">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- 计费节点选择弹窗 -->
    <el-dialog v-model="billingNodesDialog" :title="currentTemplate?.name + ' - ' + t('plan.billingNodeSelect')" width="600px">
      <div v-loading="billingNodesLoading">
        <div style="margin-bottom:8px;font-size:13px;color:#606266">
          {{ t('plan.billingNodeSelectHint') }}
        </div>
        <el-checkbox-group v-model="selectedBillingNodeIds">
<div v-for="opt in billingNodeOptions" :key="opt.member_id" style="margin-bottom:8px">
<el-checkbox :value="opt.member_id">
<el-tag :type="opt.type==='inbound'?'':'success'" size="small" style="margin-right:4px">{{ opt.type==='inbound'?t('plan.nodeTypeInbound'):t('plan.nodeTypeExternal') }}</el-tag>
<el-tag v-if="opt.has_mirror" type="warning" size="small" style="margin-right:4px">{{ t('plan.mirrored') }}</el-tag>
<span style="color:#409eff">{{ opt.server_name }}</span> - {{ opt.node_name }}
<span style="color:#909399;font-size:12px">({{ opt.protocol }}:{{ opt.port }})</span>
</el-checkbox>
</div>
        </el-checkbox-group>
        <el-empty v-if="!billingNodesLoading && billingNodeOptions.length === 0" description="{{ t('plan.noRelatedNodesHint') }}" />
      </div>
      <template #footer>
        <el-button @click="billingNodesDialog=false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="saveBillingNodes">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- 分组编辑弹窗 -->
    <el-dialog v-model="groupDialog" :title="editingGroup ? t('plan.editGroup') : t('plan.createGroup')" width="450px">
      <el-form :model="groupForm" label-width="80px" size="small">
        <el-form-item :label="t('common.name')"><el-input v-model="groupForm.name" /></el-form-item>
        <el-form-item :label="t('plan.description')"><el-input v-model="groupForm.description" type="textarea" :rows="2" /></el-form-item>
        <el-form-item :label="t('plan.region')">
          <el-select v-model="groupForm.region" style="width:100%" clearable>
            <el-option :label="t('plan.regionHK')" value="HK" />
            <el-option :label="t('plan.regionJP')" value="JP" />
            <el-option :label="t('plan.regionUS')" value="US" />
            <el-option :label="t('plan.regionSG')" value="SG" />
            <el-option :label="t('plan.regionEU')" value="EU" />
            <el-option :label="t('plan.regionCN')" value="CN" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('plan.category')">
          <el-select v-model="groupForm.tier" style="width:100%">
            <el-option :label="t('plan.tierStandard')" value="standard" />
            <el-option :label="t('plan.tierPremium')" value="premium" />
            <el-option :label="t('plan.tierExclusive')" value="exclusive" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('plan.sortOrder')"><el-input-number v-model="groupForm.sort_order" :min="0" /></el-form-item>
        <el-form-item :label="t('common.enabled')"><el-switch v-model="groupForm.enabled" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="groupDialog=false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="saveGroup">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- 分组成员管理弹窗 -->
    <el-dialog v-model="membersDialog" :title="currentGroup?.name + ' - ' + t('plan.memberManagement')" width="650px">
      <div style="margin-bottom:12px;display:flex;gap:8px;flex-wrap:wrap">
        <el-select v-model="newMemberInboundId" size="small" filterable :placeholder="t('plan.selectInbound')" style="width:360px">
          <!-- 按节点排序；标签里带上「核心 · 协议 · 传输 · 安全」，避免同协议条目看起来一样 -->
          <el-option v-for="ib in sortedInbounds" :key="ib.id" :value="ib.id" :label="inboundLabel(ib)">
            <span style="font-weight:600">{{ ib.remark || ib.tag }}</span>
            <span style="color:#909399;font-size:12px;margin-left:8px">
              {{ ib.server_name || t('plan.localNode') }} · {{ inboundDetail(ib) }} :{{ ib.port }}
            </span>
          </el-option>
        </el-select>
        <el-button size="small" type="primary" @click="addInboundMember">{{ t('plan.addInbound') }}</el-button>
        <el-select v-model="newMemberExtNodeId" size="small" filterable :placeholder="t('plan.selectExtNode')" style="width:260px">
          <el-option v-for="en in extNodes" :key="en.id" :label="en.name + ' (' + en.host + ':' + en.port + ')'" :value="en.id" />
        </el-select>
        <el-button size="small" type="success" @click="addExtNodeMember">{{ t('plan.addExtNode') }}</el-button>
      </div>
      <el-table :data="groupMembers" stripe size="small">
        <el-table-column prop="id" label="ID" width="50" />
        <el-table-column :label="t('plan.hostNode')" width="120">
          <template #default="{row}">
            <template v-if="row.inbound_config_id">{{ inboundServerName(row.inbound_config_id) }}</template>
            <template v-else-if="row.external_node_id">{{ t('plan.nodeTypeExternal') }}</template>
            <template v-else>—</template>
          </template>
        </el-table-column>
        <el-table-column :label="t('plan.inboundNodeName')">
          <template #default="{row}">
            <template v-if="row.inbound_config_id">{{ inboundName(row.inbound_config_id) }}</template>
            <template v-else-if="row.external_node_id">{{ extNodeName(row.external_node_id) }}</template>
            <template v-else>—</template>
          </template>
        </el-table-column>
        <el-table-column :label="t('common.protocol')" width="180">
          <template #default="{row}">
            <span v-if="row.inbound_config_id" style="font-size:12px">{{ inboundDetailById(row.inbound_config_id) }}</span>
            <span v-else style="color:#c0c4cc">—</span>
          </template>
        </el-table-column>
        <el-table-column :label="t('common.type')" width="80">
          <template #default="{row}">
            <el-tag v-if="row.inbound_config_id" size="small">{{ t('plan.nodeTypeInbound') }}</el-tag>
            <el-tag v-else-if="row.external_node_id" size="small" type="success">{{ t('plan.nodeTypeExternal') }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="t('common.actions')" width="80">
          <template #default="{row}">
            <el-button size="small" type="danger" @click="removeMember(row)">{{ t('common.remove') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-dialog>

    <!-- 优惠码编辑弹窗 -->
    <el-dialog v-model="promoDialog" :title="editingPromo ? t('plan.editPromotion') : t('plan.createPromotion')" width="550px">
      <el-form :model="promoForm" label-width="100px" size="small">
        <el-form-item :label="t('plan.promotionsTab')"><el-input v-model="promoForm.code" :disabled="!!editingPromo" /></el-form-item>
        <el-form-item :label="t('common.name')"><el-input v-model="promoForm.name" /></el-form-item>
        <el-form-item :label="t('plan.description')"><el-input v-model="promoForm.description" type="textarea" :rows="2" /></el-form-item>
        <el-form-item :label="t('plan.promotionType')">
          <el-select v-model="promoForm.discount_type" style="width:100%">
            <el-option :label="t('plan.typePercent')" value="percentage" />
            <el-option :label="t('plan.typeFixed')" value="fixed_amount" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('plan.discountValue')">
          <el-input-number v-model="promoForm.discount_value" :min="0" />
          <span style="font-size:12px;color:#909399;margin-left:8px">
            {{ promoForm.discount_type === 'percentage' ? t('plan.percentHint') : t('plan.amountCents') }}
          </span>
        </el-form-item>
        <el-form-item v-if="promoForm.discount_type === 'percentage'" :label="t('plan.maxDiscount')">
          <el-input-number v-model="promoForm.max_discount_amount" :min="0" />
          <span style="font-size:12px;color:#909399;margin-left:8px">{{ t('plan.zeroUnlimitedCents') }}</span>
        </el-form-item>
        <el-form-item :label="t('plan.applicableScope')">
          <el-select v-model="promoForm.applicable_scope" style="width:100%">
            <el-option :label="t('plan.allPlansLabel')" value="all" />
            <el-option :label="t('plan.specificTemplate')" value="template" />
            <el-option :label="t('plan.specificVariant')" value="specific_variant" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('plan.totalUsageLimit')"><el-input-number v-model="promoForm.max_uses" :min="0" /> <span style="font-size:12px;color:#909399">{{ t('plan.zeroUnlimited') }}</span></el-form-item>
        <el-form-item :label="t('plan.perUserLimit')"><el-input-number v-model="promoForm.max_uses_per_user" :min="1" /></el-form-item>
        <el-form-item :label="t('plan.effectiveTime')"><el-date-picker v-model="promoForm.starts_at" type="datetime" value-format="YYYY-MM-DDTHH:mm:ssZ" style="width:100%" /></el-form-item>
        <el-form-item :label="t('plan.endTime')"><el-date-picker v-model="promoForm.ends_at" type="datetime" value-format="YYYY-MM-DDTHH:mm:ssZ" style="width:100%" /></el-form-item>
        <el-form-item :label="t('common.enabled')"><el-switch v-model="promoForm.enabled" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="promoDialog=false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="savePromo">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- 流量叠加包编辑弹窗 -->
    <el-dialog v-model="trafficPackDialog" :title="editingTrafficPack ? t('plan.editTrafficPack') : t('plan.createTrafficPack')" width="520px">
      <el-form :model="trafficPackForm" label-width="100px" size="small">
        <el-form-item :label="t('common.name')"><el-input v-model="trafficPackForm.name" :placeholder="t('plan.trafficPackNamePlaceholder')" /></el-form-item>
        <el-form-item :label="t('plan.trafficPackDesc')"><el-input v-model="trafficPackForm.description" type="textarea" :rows="2" :placeholder="t('plan.trafficPackDescPlaceholder')" /></el-form-item>
        <el-form-item :label="t('plan.priceYuan')"><el-input-number v-model="trafficPackForm.price_yuan" :min="0" :step="0.01" /></el-form-item>
        <el-form-item :label="t('plan.originalPriceYuan')"><el-input-number v-model="trafficPackForm.original_price_yuan" :min="0" :step="0.01" /></el-form-item>
        <el-form-item :label="t('plan.trafficGB')"><el-input-number v-model="trafficPackForm.quota_gb" :min="0.1" :step="0.1" /></el-form-item>
        <el-divider content-position="left">{{ t('plan.expiryTime') }}</el-divider>
        <el-form-item :label="t('plan.expiryMode')">
          <el-radio-group v-model="trafficPackForm.expiry_mode">
            <el-radio value="align_with_plan">{{ t('plan.modeSamePlan') }}</el-radio>
            <el-radio value="extend_plan">{{ t('plan.modeExtend') }}</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="trafficPackForm.expiry_mode === 'extend_plan'" :label="t('plan.modeExtend')">
          <el-input-number v-model="trafficPackForm.duration_days" :min="1" :max="365" />
          <span style="font-size:12px;color:#909399;margin-left:8px">{{ t('plan.extendPlanHint') }}</span>
        </el-form-item>
        <el-divider content-position="left">{{ t('plan.displaySettings') }}</el-divider>
        <el-form-item :label="t('plan.recommended')"><el-switch v-model="trafficPackForm.is_recommended" /></el-form-item>
        <el-form-item :label="t('common.enabled')"><el-switch v-model="trafficPackForm.enabled" /></el-form-item>
        <el-form-item :label="t('plan.sortOrder')"><el-input-number v-model="trafficPackForm.sort_order" :min="0" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="trafficPackDialog=false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="saveTrafficPack">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>

  </div>
</template>

<script setup>
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
import { ElMessage, ElMessageBox } from 'element-plus'
import client from '../api/client'
import * as planApi from '../api/plan'
import { useQueryState } from '../composables/useQueryState'
import { usePagination } from '../composables/usePagination'
import PaginationBar from '../components/PaginationBar.vue'

const activeTab = useQueryState('tab', 'templates')

// ──────── Templates ────────
const templates = ref([])
const templateDialog = ref(false)
const editingTemplate = ref(null)
const templateForm = ref({})

// Non-trial templates available as the auto-convert target for a trial template.
function emptyTemplateForm() {
  return {
    name: '', description: '', category: 'standard',
    device_limit: 0, speed_limit_mbps: 0, speed_limit_up_mbps: 0, concurrent_limit: 0,
    default_quota_gb: 0, default_quota_reset_cycle: 'none',
    suspend_on_quota_exhausted: 1, is_trial: false, trial_duration_hours: 0,
    sort_order: 0, enabled: true,
  }
}
async function fetchTemplates() {
  try { const r = await planApi.listPlanTemplates(); templates.value = r?.items || r || [] } catch {}
  fetchTemplateNodes()
}
function showCreateTemplate() {
  editingTemplate.value = null
  templateForm.value = emptyTemplateForm()
  templateDialog.value = true
}
function showEditTemplate(row) {
  editingTemplate.value = row
  templateForm.value = {
    ...row,
    default_quota_gb: row.default_quota_bytes / 1073741824,
    is_trial: !!row.is_trial,
    enabled: !!row.enabled,
    suspend_on_quota_exhausted: row.suspend_on_quota_exhausted,
  }
  templateDialog.value = true
}
async function saveTemplate() {
  try {
    const payload = { ...templateForm.value }
    payload.default_quota_bytes = Math.round((payload.default_quota_gb || 0) * 1073741824)
    delete payload.default_quota_gb
    // billing_node_ids and node_group_ids are managed via separate dialogs,
    // never send them through the template edit form to avoid accidental overrides
    delete payload.billing_node_ids
    delete payload.node_group_ids
    delete payload.id
    delete payload.created_at
    delete payload.updated_at
    if (editingTemplate.value) {
      await planApi.updatePlanTemplate(editingTemplate.value.id, payload)
    } else {
      await planApi.createPlanTemplate(payload)
    }
    ElMessage.success(t('plan.saveSuccess'))
    templateDialog.value = false
    fetchTemplates()
  } catch {}
}
async function deleteTemplate(row) {
  await ElMessageBox.confirm(t('plan.deleteTemplateConfirm', { name: row.name }), t('common.confirm'))
  try {
    await planApi.deletePlanTemplate(row.id)
    ElMessage.success(t('plan.deleteSuccess'))
    fetchTemplates()
  } catch {}
}

// ──────── Account Sync ────────
const syncLoading = ref({})
const syncAllLoading = ref(false)

async function handleSyncTemplate(tpl) {
  try {
    await ElMessageBox.confirm(
      t('plan.syncAccountConfirm', { name: tpl.name }),
      t('plan.syncAccountConfirmTitle'),
      { type: 'warning' }
    )
  } catch {
    return
  }
  syncLoading.value[tpl.id] = true
  try {
    const res = await planApi.syncAccountsForTemplate(tpl.id)
    ElMessage.success(t('plan.syncComplete', { count: res.inbounds_processed || 0 }))
  } catch {
    ElMessage.error(t('plan.syncFailed'))
  } finally {
    syncLoading.value[tpl.id] = false
  }
}

async function handleSyncAll() {
  try {
    await ElMessageBox.confirm(
      t('plan.syncAllConfirm'),
      t('plan.syncAllConfirmTitle'),
      { type: 'warning' }
    )
  } catch {
    return
  }
  syncAllLoading.value = true
  try {
    const res = await planApi.syncAllAccounts()
    const templates = res.templates || {}
    const total = Object.values(templates).reduce((s, v) => s + v, 0)
    ElMessage.success(t('plan.syncAllComplete', { planCount: Object.keys(templates).length, nodeCount: total }))
  } catch {
    ElMessage.error(t('plan.syncAllFailed'))
  } finally {
    syncAllLoading.value = false
  }
}

// ──────── Variants ────────
const variantsDialog = ref(false)
const currentTemplate = ref(null)
const variants = ref([])
const variantDialog = ref(false)
const editingVariant = ref(null)
const variantForm = ref({})

function emptyVariantForm() {
  return {
    variant_type: 'plan', name: '', billing_cycle: 'monthly', duration_days: 30,
    price_yuan: 0, original_price_yuan: 0, quota_gb: 0,
    is_default: false, is_recommended: false, enabled: true, sort_order: 0,
  }
}
async function showVariantsDialog(tpl) {
  currentTemplate.value = tpl
  variantsDialog.value = true
  try { const r = await planApi.listPlanVariants(tpl.id); variants.value = r?.items || r || [] } catch { variants.value = [] }
}
function showCreateVariant() {
  editingVariant.value = null
  variantForm.value = emptyVariantForm()
  variantDialog.value = true
}
function showEditVariant(row) {
  editingVariant.value = row
  variantForm.value = {
    ...row,
    price_yuan: row.price / 100,
    original_price_yuan: row.original_price / 100,
    quota_gb: row.quota_bytes / 1073741824,
    is_default: !!row.is_default,
    is_recommended: !!row.is_recommended,
    enabled: !!row.enabled,
  }
  variantDialog.value = true
}
async function saveVariant() {
  try {
    const payload = { ...variantForm.value }
    payload.price = Math.round(payload.price_yuan * 100)
    payload.original_price = Math.round(payload.original_price_yuan * 100)
    payload.quota_bytes = Math.round(payload.quota_gb * 1073741824)
    delete payload.price_yuan
    delete payload.original_price_yuan
    delete payload.quota_gb
    if (editingVariant.value) {
      await planApi.updatePlanVariant(editingVariant.value.id, payload)
    } else {
      await planApi.createPlanVariant(currentTemplate.value.id, payload)
    }
    ElMessage.success(t('plan.saveSuccess'))
    variantDialog.value = false
    showVariantsDialog(currentTemplate.value)
  } catch {}
}
async function deleteVariant(row) {
  await ElMessageBox.confirm(t('plan.deleteVariantConfirm', { name: row.name }), t('common.confirm'))
  try {
    await planApi.deletePlanVariant(row.id)
    ElMessage.success(t('plan.deleteSuccess'))
    showVariantsDialog(currentTemplate.value)
  } catch {}
}

// ──────── Template Node Groups ────────
const templateNodeGroupsDialog = ref(false)
const selectedNodeGroupIds = ref([])

async function showNodeGroupsDialog(tpl) {
  currentTemplate.value = tpl
  selectedNodeGroupIds.value = [...(tpl.node_group_ids || [])]
  await fetchNodeGroups()
  templateNodeGroupsDialog.value = true
}
async function saveTemplateNodeGroups() {
  try {
    await planApi.setTemplateNodeGroups(currentTemplate.value.id, selectedNodeGroupIds.value)
    ElMessage.success(t('plan.saveSuccess'))
    templateNodeGroupsDialog.value = false
    fetchTemplates()
    fetchTemplateNodes()
  } catch {}
}

// ──────── Template Billing Nodes ────────
const billingNodesDialog = ref(false)
const billingNodeOptions = ref([])
const selectedBillingNodeIds = ref([])
const billingNodesLoading = ref(false)
const templateNodes = ref({})

async function fetchTemplateNodes() {
  for (const tpl of templates.value) {
    try {
      const res = await planApi.getBillingNodeOptions(tpl.id)
      templateNodes.value[tpl.id] = Array.isArray(res) ? res : []
    } catch {
      templateNodes.value[tpl.id] = []
    }
  }
}

async function showBillingNodesDialog(tpl) {
  currentTemplate.value = tpl
  selectedBillingNodeIds.value = [...(tpl.billing_node_ids || [])]
  billingNodesLoading.value = true
  billingNodesDialog.value = true
  try {
    const res = await planApi.getBillingNodeOptions(tpl.id)
    billingNodeOptions.value = Array.isArray(res) ? res : []
  } catch {
    billingNodeOptions.value = []
  } finally {
    billingNodesLoading.value = false
  }
}

async function saveBillingNodes() {
  try {
    await planApi.updatePlanTemplate(currentTemplate.value.id, { billing_node_ids: selectedBillingNodeIds.value })
    ElMessage.success(t('plan.saveSuccess'))
    billingNodesDialog.value = false
    fetchTemplates()
  } catch {}
}

// ──────── Node Groups ────────
const nodeGroups = ref([])
const groupLoading = ref(false)
const groupDialog = ref(false)
const editingGroup = ref(null)
const groupForm = ref({})
const membersDialog = ref(false)
const currentGroup = ref(null)
const groupMembers = ref([])
const inbounds = ref([])
const extNodes = ref([])
const newMemberInboundId = ref(null)
const newMemberExtNodeId = ref(null)

async function fetchNodeGroups() {
  groupLoading.value = true
  try { nodeGroups.value = (await planApi.listNodeGroups()) || [] } catch {} finally { groupLoading.value = false }
}
function showCreateGroup() {
  editingGroup.value = null
  groupForm.value = { name: '', description: '', region: '', tier: 'standard', sort_order: 0, enabled: true }
  groupDialog.value = true
}
function showEditGroup(row) {
  editingGroup.value = row
  groupForm.value = { ...row, enabled: !!row.enabled }
  groupDialog.value = true
}
async function saveGroup() {
  try {
    if (editingGroup.value) {
      await planApi.updateNodeGroup(editingGroup.value.id, groupForm.value)
    } else {
      await planApi.createNodeGroup(groupForm.value)
    }
    ElMessage.success(t('plan.saveSuccess'))
    groupDialog.value = false
    fetchNodeGroups()
  } catch {}
}
async function deleteGroup(row) {
  await ElMessageBox.confirm(t('plan.deleteGroupConfirm', { name: row.name }), t('common.confirm'))
  try {
    await planApi.deleteNodeGroup(row.id)
    ElMessage.success(t('plan.deleteSuccess'))
    fetchNodeGroups()
  } catch {}
}
async function showMembersDialog(group) {
  currentGroup.value = group
  membersDialog.value = true
  try { groupMembers.value = (await planApi.listGroupMembers(group.id)) || [] } catch { groupMembers.value = [] }
}
async function addInboundMember() {
  if (!newMemberInboundId.value) return
  try {
    await planApi.addGroupMember(currentGroup.value.id, { inbound_config_id: newMemberInboundId.value })
    ElMessage.success(t('plan.addSuccess'))
    newMemberInboundId.value = null
    showMembersDialog(currentGroup.value)
  } catch {}
}
async function addExtNodeMember() {
  if (!newMemberExtNodeId.value) return
  try {
    await planApi.addGroupMember(currentGroup.value.id, { external_node_id: newMemberExtNodeId.value })
    ElMessage.success(t('plan.addSuccess'))
    newMemberExtNodeId.value = null
    showMembersDialog(currentGroup.value)
  } catch {}
}
async function removeMember(row) {
  try {
    await planApi.removeGroupMember(currentGroup.value.id, row.id)
    ElMessage.success(t('common.removeSuccess'))
    showMembersDialog(currentGroup.value)
  } catch {}
}

// ──────── Promotions ────────
const promotions = ref([])
const promoLoading = ref(false)
const promoDialog = ref(false)
const editingPromo = ref(null)
const promoForm = ref({})

async function fetchPromotions() {
  promoLoading.value = true
  try {
    const r = await planApi.listPromotions({ limit: 100 })
    promotions.value = r.items || []
  } catch {} finally { promoLoading.value = false }
}
function showCreatePromo() {
  editingPromo.value = null
  promoForm.value = {
    code: '', name: '', description: '',
    discount_type: 'percentage', discount_value: 20, max_discount_amount: 0,
    applicable_scope: 'all', max_uses: 0, max_uses_per_user: 1,
    starts_at: null, ends_at: null, enabled: true,
  }
  promoDialog.value = true
}
function showEditPromo(row) {
  editingPromo.value = row
  promoForm.value = { ...row, enabled: !!row.enabled }
  promoDialog.value = true
}
async function savePromo() {
  try {
    if (editingPromo.value) {
      await planApi.updatePromotion(editingPromo.value.id, promoForm.value)
    } else {
      await planApi.createPromotion(promoForm.value)
    }
    ElMessage.success(t('plan.saveSuccess'))
    promoDialog.value = false
    fetchPromotions()
  } catch {}
}
async function deletePromo(row) {
  await ElMessageBox.confirm(t('plan.deletePromoConfirm', { code: row.code }), t('common.confirm'))
  try {
    await planApi.deletePromotion(row.id)
    ElMessage.success(t('plan.deleteSuccess'))
    fetchPromotions()
  } catch {}
}

// ──────── Orders ────────
const orders = ref([])
const orderLoading = ref(false)
const orderClientFilter = ref('')

async function fetchOrders() {
  if (!orderClientFilter.value) return
  orderLoading.value = true
  try {
    const r = await planApi.listClientOrders(orderClientFilter.value, { limit: 100 })
    orders.value = r.items || []
  } catch {} finally { orderLoading.value = false }
}

// ──────── Shared data ────────
async function fetchInbounds() {
  try {
    inbounds.value = (await planApi.listAllInboundsWithServer()) || []
  } catch {}
}
async function fetchExtNodes() {
  try { extNodes.value = (await client.get('/external-nodes')) || [] } catch {}
}

// ──────── Traffic Packs (独立流量叠加包) ────────
const trafficPacksLoading = ref(false)
const trafficPackVariants = ref([])
const { page: trafficPage, pageSize: trafficSize, total: trafficTotal, limit: trafficLimit, offset: trafficOffset } = usePagination({ pageKey: 'trafPage', pageSizeKey: 'trafSize' })
const trafficPackDialog = ref(false)
const editingTrafficPack = ref(null)
const trafficPackForm = ref({})

function emptyTrafficPackForm() {
  return {
    name: '', description: '', price_yuan: 0, original_price_yuan: 0, quota_gb: 1,
    expiry_mode: 'align_with_plan', duration_days: 7,
    is_recommended: false, enabled: true, sort_order: 0,
  }
}

async function fetchTrafficPacks() {
  trafficPacksLoading.value = true
  try {
    const r = await planApi.listTrafficPacks({ limit: trafficLimit.value, offset: trafficOffset.value })
    trafficPackVariants.value = Array.isArray(r?.items) ? r.items : (Array.isArray(r) ? r : [])
    trafficTotal.value = r?.total ?? trafficPackVariants.value.length
  } catch {
    trafficPackVariants.value = []
  } finally {
    trafficPacksLoading.value = false
  }
}

function showCreateTrafficPack() {
  editingTrafficPack.value = null
  trafficPackForm.value = emptyTrafficPackForm()
  trafficPackDialog.value = true
}

function showEditTrafficPack(row) {
  editingTrafficPack.value = row
  trafficPackForm.value = {
    ...row,
    price_yuan: row.price / 100,
    original_price_yuan: row.original_price / 100,
    quota_gb: row.quota_bytes / 1073741824,
    expiry_mode: row.expiry_mode || 'align_with_plan',
    duration_days: row.duration_days || 7,
    is_recommended: !!row.is_recommended,
    enabled: !!row.enabled,
  }
  trafficPackDialog.value = true
}

async function saveTrafficPack() {
  try {
    const payload = {
      name: trafficPackForm.value.name,
      description: trafficPackForm.value.description,
      price: Math.round(trafficPackForm.value.price_yuan * 100),
      original_price: Math.round(trafficPackForm.value.original_price_yuan * 100),
      quota_bytes: Math.round(trafficPackForm.value.quota_gb * 1073741824),
      expiry_mode: trafficPackForm.value.expiry_mode,
      duration_days: trafficPackForm.value.expiry_mode === 'extend_plan' ? trafficPackForm.value.duration_days : 0,
      is_recommended: trafficPackForm.value.is_recommended,
      enabled: trafficPackForm.value.enabled,
      sort_order: trafficPackForm.value.sort_order,
    }
    if (editingTrafficPack.value) {
      await planApi.updateTrafficPack(editingTrafficPack.value.id, payload)
    } else {
      await planApi.createTrafficPack(payload)
    }
    ElMessage.success(t('plan.saveSuccess'))
    trafficPackDialog.value = false
    fetchTrafficPacks()
  } catch {}
}

async function deleteTrafficPack(row) {
  await ElMessageBox.confirm(t('plan.deleteTrafficPackConfirm', { name: row.name }), t('common.confirm'))
  try {
    await planApi.deleteTrafficPack(row.id)
    ElMessage.success(t('plan.deleteSuccess'))
    fetchTrafficPacks()
  } catch {}
}

// ──────── Helpers ────────
const formatBytes = (b) => {
  if (!b || b <= 0) return t('common.unlimited')
  if (b >= 1073741824) return (b / 1073741824).toFixed(1) + ' GB'
  if (b >= 1048576) return (b / 1048576).toFixed(0) + ' MB'
  return (b / 1024).toFixed(0) + ' KB'
}
const categoryLabel = (c) => ({ basic: t('plan.categoryBasic'), standard: t('plan.categoryStandard'), premium: t('plan.categoryPremium'), trial: t('plan.categoryTrial'), custom: t('plan.categoryCustom') })[c] || c
const categoryType = (c) => ({ basic:'info', standard:'', premium:'warning', trial:'danger', custom:'success' })[c] || ''
const tierLabel = (tv) => ({ standard: t('plan.tierStandardLabel'), premium: t('plan.tierPremiumLabel'), exclusive: t('plan.tierExclusiveLabel') })[tv] || tv
const tierType = (t) => ({ standard:'', premium:'warning', exclusive:'danger' })[t] || ''
const cycleLabel = (c) => ({ monthly: t('plan.cycleMonthly'), quarterly: t('plan.cycleQuarterly'), semi_annual: t('plan.cycleSemiAnnual'), annual: t('plan.cycleAnnual'), lifetime: t('plan.cycleLifetime'), one_time: t('plan.cycleOneTime') })[c] || c
const scopeLabel = (s) => ({ all: t('plan.allPlansLabel'), template: t('plan.specificTemplate'), specific_variant: t('plan.specificVariant') })[s] || s
const orderTypeLabel = (tv) => ({ new: t('plan.orderNew'), renew: t('plan.orderRenew'), upgrade: t('plan.orderUpgrade'), downgrade: t('plan.orderDowngrade'), trial_convert: t('plan.orderTrialConvert') })[tv] || tv
const orderTypeColor = (t) => ({ new:'success', renew:'', upgrade:'warning', downgrade:'info', trial_convert:'danger' })[t] || ''
const orderStatusLabel = (s) => ({ pending: t('plan.orderPending'), paid: t('plan.orderPaid'), completed: t('plan.orderCompleted'), cancelled: t('plan.orderCancelled'), refunded: t('plan.orderRefunded') })[s] || s
const orderStatusColor = (s) => ({ pending:'warning', paid:'info', completed:'success', cancelled:'danger', refunded:'danger' })[s] || ''
const inboundName = (id) => { const ib = inbounds.value.find(i => i.id === id); return ib ? (ib.remark || ib.tag) : '#' + id }
// sortedInbounds：按「节点 → 入站」排序，让同节点条目聚在一起（后端已按 sort_order 排序，这里再兜底一次）
const sortedInbounds = computed(() => {
  const list = [...(inbounds.value || [])]
  return list.sort((a, b) => {
    const an = a.server_name || '', bn = b.server_name || ''
    if (an !== bn) return an.localeCompare(bn)
    if ((a.server_id || 0) !== (b.server_id || 0)) return (a.server_id || 0) - (b.server_id || 0)
    return (a.tag || '').localeCompare(b.tag || '')
  })
})

// inboundDetail：核心 · 协议 · 传输 · 安全（空值自动省略）
function inboundDetail(ib) {
  if (!ib) return '—'
  return [ib.core_type, ib.protocol, ib.transport, ib.security].filter(Boolean).join(' · ') || '—'
}
function inboundDetailById(id) {
  const ib = inbounds.value.find(i => i.id === id)
  return ib ? inboundDetail(ib) : '—'
}
// inboundLabel：下拉的纯文本标签（供搜索过滤）
function inboundLabel(ib) {
  return `${ib.server_name || t('plan.localNode')} · ${inboundDetail(ib)} :${ib.port} — ${ib.remark || ib.tag}`
}

const inboundServerName = (id) => { const ib = inbounds.value.find(i => i.id === id); return ib ? (ib.server_name || t('plan.localNode')) : '—' }
const extNodeName = (id) => { const n = extNodes.value.find(n => n.id === id); return n ? n.name : '#' + id }

onMounted(() => {
  fetchTemplates()
  fetchNodeGroups()
  fetchPromotions()
  fetchInbounds()
  fetchExtNodes()
  fetchTrafficPacks()
})
</script>

<style scoped>
.el-divider { margin: 12px 0;
}
</style>
