<template>
  <div>
    <el-tabs v-model="activeTab" type="border-card">

      <!-- ════════════ 流量叠加包 Tab ════════════ -->
      <el-tab-pane label="流量叠加包" name="traffic_packs">
        <div style="display:flex;justify-content:space-between;margin-bottom:16px">
          <h3 style="margin:0">流量叠加包</h3>
        </div>
        <el-table :data="allTrafficPacks" v-loading="trafficPacksLoading" stripe size="small">
          <el-table-column prop="id" label="ID" width="60" />
          <el-table-column prop="name" label="名称" width="120" />
          <el-table-column label="所属模板" width="150">
            <template #default="{row}">{{ templateName(row.template_id) }}</template>
          </el-table-column>
          <el-table-column label="价格" width="100">
            <template #default="{row}">¥{{ (row.price/100).toFixed(2) }}</template>
          </el-table-column>
          <el-table-column label="流量" width="100">
            <template #default="{row}">{{ row.quota_bytes > 0 ? formatBytes(row.quota_bytes) : '继承模板' }}</template>
          </el-table-column>
          <el-table-column prop="duration_days" label="天数" width="80" />
          <el-table-column label="状态" width="80">
            <template #default="{row}">
              <el-tag :type="row.enabled?'success':'danger'" size="small">{{ row.enabled?'启用':'禁用' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="120">
            <template #default="{row}">
              <el-button size="small" @click="showEditTrafficPackVariant(row)">编辑</el-button>
              <el-button size="small" type="danger" @click="deleteTrafficPackVariant(row)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>

      <!-- ════════════ 套餐模板 Tab ════════════ -->
      <el-tab-pane label="套餐模板" name="templates">
        <div style="display:flex;justify-content:space-between;margin-bottom:16px">
          <h3 style="margin:0">套餐模板</h3>
          <div style="display:flex;gap:8px">
            <el-button type="warning" size="small" :loading="syncAllLoading" @click="handleSyncAll">全量同步账号</el-button>
            <el-button type="primary" size="small" @click="showCreateTemplate">+ 新增模板</el-button>
          </div>
        </div>
        <el-row :gutter="16">
          <el-col v-for="tpl in templates" :key="tpl.id" :span="8" style="margin-bottom:16px">
            <el-card shadow="hover">
              <template #header>
                <div style="display:flex;justify-content:space-between;align-items:center">
                  <span style="font-weight:600">{{ tpl.name }}</span>
                  <el-tag :type="tpl.enabled?'success':'danger'" size="small">{{ tpl.enabled?'启用':'禁用' }}</el-tag>
                </div>
              </template>
              <div style="font-size:13px;line-height:2;color:#606266">
                <div>分类: <el-tag size="small" :type="categoryType(tpl.category)">{{ categoryLabel(tpl.category) }}</el-tag></div>
                <div>流量: {{ formatBytes(tpl.default_quota_bytes) }} / {{ tpl.default_quota_reset_cycle || '不限' }}</div>
                <div>设备: {{ tpl.device_limit || '不限' }} 台 | 限速: {{ tpl.speed_limit_mbps || '不限' }} Mbps</div>
                <div v-if="tpl.is_trial" style="color:#e6a23c">
                  <el-tag type="warning" size="small">试用</el-tag> {{ tpl.trial_duration_hours }}小时
                </div>
                <div>节点分组: {{ (tpl.node_group_ids||[]).length }} 个 | 计费节点: {{ (tpl.billing_node_ids||[]).length }} 个</div>
                <div style="color:#909399;font-size:12px">{{ tpl.description }}</div>
              </div>
              <div v-if="templateNodes[tpl.id] && templateNodes[tpl.id].length" style="margin-top:8px;border-top:1px solid #ebeef5;padding-top:8px">
                <div style="font-size:12px;color:#909399;margin-bottom:4px">关联节点（主机 - 入口/节点名称）</div>
                <div v-for="node in templateNodes[tpl.id].slice(0,5)" :key="node.member_id" style="font-size:12px;line-height:1.8">
                  <el-tag :type="node.type==='inbound'?'':'success'" size="small" style="margin-right:4px">{{ node.type==='inbound'?'入口':'外部' }}</el-tag>
                  <span style="color:#409eff">{{ node.server_name }}</span> - {{ node.node_name }}
                  <span style="color:#909399">({{ node.protocol }}:{{ node.port }})</span>
                </div>
                <div v-if="templateNodes[tpl.id].length > 5" style="font-size:12px;color:#909399;margin-top:4px">...共 {{ templateNodes[tpl.id].length }} 个节点</div>
              </div>
              <div style="margin-top:12px;display:flex;gap:4px;flex-wrap:wrap">
                <el-button size="small" @click="showVariantsDialog(tpl)">管理规格</el-button>
                <el-button size="small" @click="showNodeGroupsDialog(tpl)">管理节点</el-button>
                <el-button size="small" @click="showBillingNodesDialog(tpl)">计费节点</el-button>
                <el-button size="small" type="warning" :loading="syncLoading[tpl.id]" @click="handleSyncTemplate(tpl)">同步账号</el-button>
                <el-button size="small" @click="showEditTemplate(tpl)">编辑</el-button>
                <el-button size="small" type="danger" @click="deleteTemplate(tpl)">删除</el-button>
              </div>
            </el-card>
          </el-col>
        </el-row>
      </el-tab-pane>

      <!-- ════════════ 节点分组 Tab ════════════ -->
      <el-tab-pane label="节点分组" name="node-groups">
        <div style="display:flex;justify-content:space-between;margin-bottom:16px">
          <h3 style="margin:0">节点分组</h3>
          <el-button type="primary" size="small" @click="showCreateGroup">+ 新增分组</el-button>
        </div>
        <el-table :data="nodeGroups" v-loading="groupLoading" stripe size="small">
          <el-table-column prop="id" label="ID" width="50" />
          <el-table-column prop="name" label="名称" width="150" />
          <el-table-column label="等级" width="100">
            <template #default="{row}">
              <el-tag :type="tierType(row.tier)" size="small">{{ tierLabel(row.tier) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="region" label="区域" width="80" />
          <el-table-column prop="description" label="描述" show-overflow-tooltip />
          <el-table-column label="状态" width="70">
            <template #default="{row}">
              <el-tag :type="row.enabled?'success':'danger'" size="small">{{ row.enabled?'✓':'✗' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="200">
            <template #default="{row}">
              <el-button size="small" @click="showMembersDialog(row)">成员管理</el-button>
              <el-button size="small" @click="showEditGroup(row)">编辑</el-button>
              <el-button size="small" type="danger" @click="deleteGroup(row)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>

      <!-- ════════════ 优惠码 Tab ════════════ -->
      <el-tab-pane label="优惠码" name="promotions">
        <div style="display:flex;justify-content:space-between;margin-bottom:16px">
          <h3 style="margin:0">促销优惠</h3>
          <el-button type="primary" size="small" @click="showCreatePromo">+ 新增优惠码</el-button>
        </div>
        <el-table :data="promotions" v-loading="promoLoading" stripe size="small">
          <el-table-column prop="id" label="ID" width="50" />
          <el-table-column prop="code" label="优惠码" width="150" />
          <el-table-column prop="name" label="名称" width="150" />
          <el-table-column label="类型" width="100">
            <template #default="{row}">
              {{ row.discount_type === 'percentage' ? row.discount_value + '%' : '¥' + (row.discount_value/100).toFixed(2) }}
            </template>
          </el-table-column>
          <el-table-column label="适用范围" width="120">
            <template #default="{row}">
              <el-tag size="small">{{ scopeLabel(row.applicable_scope) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="使用次数" width="100">
            <template #default="{row}">{{ row.used_count }}{{ row.max_uses > 0 ? '/' + row.max_uses : '/∞' }}</template>
          </el-table-column>
          <el-table-column label="有效期" width="200">
            <template #default="{row}">
              <span style="font-size:12px">
                {{ row.starts_at?.substring(0,10) || '—' }} ~ {{ row.ends_at?.substring(0,10) || '永久' }}
              </span>
            </template>
          </el-table-column>
          <el-table-column label="状态" width="70">
            <template #default="{row}">
              <el-tag :type="row.enabled?'success':'danger'" size="small">{{ row.enabled?'✓':'✗' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="120">
            <template #default="{row}">
              <el-button size="small" @click="showEditPromo(row)">编辑</el-button>
              <el-button size="small" type="danger" @click="deletePromo(row)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>

      <!-- ════════════ 订单记录 Tab ════════════ -->
      <el-tab-pane label="订单记录" name="orders">
        <div style="display:flex;justify-content:space-between;margin-bottom:16px">
          <div style="display:flex;gap:8px;align-items:center">
            <h3 style="margin:0">订单记录</h3>
            <el-input v-model="orderClientFilter" size="small" placeholder="输入客户端ID" style="width:150px" @keyup.enter="fetchOrders" clearable />
            <el-button size="small" @click="fetchOrders">查询</el-button>
          </div>
        </div>
        <el-table :data="orders" v-loading="orderLoading" stripe size="small">
          <el-table-column prop="id" label="ID" width="60" />
          <el-table-column prop="client_id" label="客户端ID" width="80" />
          <el-table-column label="类型" width="80">
            <template #default="{row}">
              <el-tag :type="orderTypeColor(row.order_type)" size="small">{{ orderTypeLabel(row.order_type) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="to_variant_id" label="规格ID" width="70" />
          <el-table-column label="金额" width="180">
            <template #default="{row}">
              <span style="font-size:12px">
                ¥{{ (row.paid_amount/100).toFixed(2) }}
                <span v-if="row.discount_amount > 0" style="color:#f56c6c;text-decoration:line-through;font-size:11px">
                  ¥{{ (row.original_amount/100).toFixed(2) }}
                </span>
              </span>
            </template>
          </el-table-column>
          <el-table-column prop="promotion_code" label="优惠码" width="120" />
          <el-table-column label="生效时间" width="160">
            <template #default="{row}">{{ row.effective_from?.substring(0,16).replace('T',' ') }}</template>
          </el-table-column>
          <el-table-column label="到期时间" width="160">
            <template #default="{row}">{{ row.effective_to?.substring(0,16).replace('T',' ') || '—' }}</template>
          </el-table-column>
          <el-table-column label="状态" width="80">
            <template #default="{row}">
              <el-tag :type="orderStatusColor(row.status)" size="small">{{ orderStatusLabel(row.status) }}</el-tag>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>

    </el-tabs>

    <!-- ═══════════════════════════════════════════════ -->
    <!-- 模板编辑弹窗 -->
    <el-dialog v-model="templateDialog" :title="editingTemplate?'编辑模板':'新增模板'" width="650px">
      <el-form :model="templateForm" label-width="120px" size="small">
        <el-divider content-position="left">基本信息</el-divider>
        <el-form-item label="名称"><el-input v-model="templateForm.name" /></el-form-item>
        <el-form-item label="描述"><el-input v-model="templateForm.description" type="textarea" :rows="2" /></el-form-item>
        <el-form-item label="分类">
          <el-select v-model="templateForm.category" style="width:100%">
            <el-option label="基础版" value="basic" />
            <el-option label="标准版" value="standard" />
            <el-option label="高级版" value="premium" />
            <el-option label="试用版" value="trial" />
            <el-option label="自定义" value="custom" />
          </el-select>
        </el-form-item>
        <el-divider content-position="left">功能特性</el-divider>
        <el-form-item label="设备数限制">
          <el-input-number v-model="templateForm.device_limit" :min="0" /> <span style="color:#909399;font-size:12px">0=不限</span>
        </el-form-item>
        <el-form-item label="限速 (Mbps)">
          <el-input-number v-model="templateForm.speed_limit_mbps" :min="0" /> <span style="color:#909399;font-size:12px">0=不限</span>
        </el-form-item>
        <el-form-item label="并发连接限制">
          <el-input-number v-model="templateForm.concurrent_limit" :min="0" /> <span style="color:#909399;font-size:12px">0=不限</span>
        </el-form-item>
        <el-divider content-position="left">配额默认值</el-divider>
        <el-form-item label="默认流量 (GB)">
          <el-input-number v-model="templateForm.default_quota_gb" :min="0" :step="0.1" /> <span style="color:#909399;font-size:12px">0=不限</span>
        </el-form-item>
        <el-form-item label="配额重置周期">
          <el-select v-model="templateForm.default_quota_reset_cycle" style="width:100%">
            <el-option label="不重置" value="none" />
            <el-option label="每月" value="monthly" />
          </el-select>
        </el-form-item>
        <el-form-item label="超量暂停">
          <el-switch v-model="templateForm.suspend_on_quota_exhausted" :active-value="1" :inactive-value="0" />
        </el-form-item>
        <el-divider content-position="left">试用配置</el-divider>
        <el-form-item label="是否试用">
          <el-switch v-model="templateForm.is_trial" />
        </el-form-item>
        <template v-if="templateForm.is_trial">
          <el-form-item label="试用时长 (小时)">
            <el-input-number v-model="templateForm.trial_duration_hours" :min="1" />
          </el-form-item>
        </template>
        <el-divider content-position="left">显示设置</el-divider>
        <el-form-item label="排序"><el-input-number v-model="templateForm.sort_order" :min="0" /></el-form-item>
        <el-form-item label="启用"><el-switch v-model="templateForm.enabled" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="templateDialog=false">取消</el-button>
        <el-button type="primary" @click="saveTemplate">保存</el-button>
      </template>
    </el-dialog>

    <!-- 规格管理弹窗 -->
    <el-dialog v-model="variantsDialog" :title="currentTemplate?.name + ' - 规格管理'" width="750px">
      <div style="margin-bottom:12px;text-align:right">
        <el-button type="primary" size="small" @click="showCreateVariant">+ 添加规格</el-button>
      </div>
      <el-table :data="variants" stripe size="small">
        <el-table-column prop="name" label="名称" width="80" />
        <el-table-column label="类型" width="90">
          <template #default="{row}">
            <el-tag :type="row.variant_type==='traffic_pack'?'warning':''" size="small">{{ row.variant_type==='traffic_pack'?'流量包':'套餐' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="周期" width="80">
          <template #default="{row}">{{ cycleLabel(row.billing_cycle) }}</template>
        </el-table-column>
        <el-table-column label="价格" width="100">
          <template #default="{row}">¥{{ (row.price/100).toFixed(2) }}</template>
        </el-table-column>
        <el-table-column label="原价" width="100">
          <template #default="{row}">{{ row.original_price > 0 ? '¥' + (row.original_price/100).toFixed(2) : '—' }}</template>
        </el-table-column>
        <el-table-column prop="duration_days" label="天数" width="60" />
        <el-table-column label="配额" width="100">
          <template #default="{row}">{{ row.quota_bytes > 0 ? formatBytes(row.quota_bytes) : '继承模板' }}</template>
        </el-table-column>
        <el-table-column label="标记" width="80">
          <template #default="{row}">
            <el-tag v-if="row.is_default" type="success" size="small">默认</el-tag>
            <el-tag v-else-if="row.is_recommended" type="warning" size="small">推荐</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="120">
          <template #default="{row}">
            <el-button size="small" @click="showEditVariant(row)">编辑</el-button>
            <el-button size="small" type="danger" @click="deleteVariant(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-dialog>

    <!-- 规格编辑弹窗 -->
    <el-dialog v-model="variantDialog" :title="editingVariant?'编辑规格':'添加规格'" width="500px" append-to-body>
      <el-form :model="variantForm" label-width="100px" size="small">
        <el-form-item label="规格类型">
          <el-select v-model="variantForm.variant_type" style="width:100%">
            <el-option label="常规套餐" value="plan" />
            <el-option label="流量叠加包" value="traffic_pack" />
          </el-select>
        </el-form-item>
        <el-form-item label="名称"><el-input v-model="variantForm.name" placeholder="如: 月付" /></el-form-item>
        <el-form-item label="计费周期">
          <el-select v-model="variantForm.billing_cycle" style="width:100%">
            <el-option label="月付" value="monthly" />
            <el-option label="季付" value="quarterly" />
            <el-option label="半年付" value="semi_annual" />
            <el-option label="年付" value="annual" />
            <el-option label="终身" value="lifetime" />
            <el-option label="一次性" value="one_time" />
          </el-select>
        </el-form-item>
        <el-form-item label="有效天数"><el-input-number v-model="variantForm.duration_days" :min="1" /></el-form-item>
        <el-form-item label="价格 (元)"><el-input-number v-model="variantForm.price_yuan" :min="0" :step="0.01" /></el-form-item>
        <el-form-item label="原价 (元)"><el-input-number v-model="variantForm.original_price_yuan" :min="0" :step="0.01" /></el-form-item>
        <el-form-item label="流量 (GB)"><el-input-number v-model="variantForm.quota_gb" :min="0" :step="0.1" /> <span style="font-size:12px;color:#909399">0=继承模板</span></el-form-item>
        <el-form-item label="默认规格"><el-switch v-model="variantForm.is_default" /></el-form-item>
        <el-form-item label="推荐"><el-switch v-model="variantForm.is_recommended" /></el-form-item>
        <el-form-item label="启用"><el-switch v-model="variantForm.enabled" /></el-form-item>
        <el-form-item label="排序"><el-input-number v-model="variantForm.sort_order" :min="0" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="variantDialog=false">取消</el-button>
        <el-button type="primary" @click="saveVariant">保存</el-button>
      </template>
    </el-dialog>

    <!-- 模板节点分组管理弹窗 -->
    <el-dialog v-model="templateNodeGroupsDialog" :title="currentTemplate?.name + ' - 节点分组'" width="500px">
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
        <el-button @click="templateNodeGroupsDialog=false">取消</el-button>
        <el-button type="primary" @click="saveTemplateNodeGroups">保存</el-button>
      </template>
    </el-dialog>

    <!-- 计费节点选择弹窗 -->
    <el-dialog v-model="billingNodesDialog" :title="currentTemplate?.name + ' - 计费节点选择'" width="600px">
      <div v-loading="billingNodesLoading">
        <div style="margin-bottom:8px;font-size:13px;color:#606266">
          选择用于渲染计费信息的节点（显示：主机节点 - 入口/节点名称）。不选则不显示计费信息。
        </div>
        <el-checkbox-group v-model="selectedBillingNodeIds">
          <div v-for="opt in billingNodeOptions" :key="opt.member_id" style="margin-bottom:8px">
            <el-checkbox :value="opt.member_id">
              <el-tag :type="opt.type==='inbound'?'':'success'" size="small" style="margin-right:4px">{{ opt.type==='inbound'?'入口':'外部' }}</el-tag>
              <span style="color:#409eff">{{ opt.server_name }}</span> - {{ opt.node_name }}
              <span style="color:#909399;font-size:12px">({{ opt.protocol }}:{{ opt.port }})</span>
            </el-checkbox>
          </div>
        </el-checkbox-group>
        <el-empty v-if="!billingNodesLoading && billingNodeOptions.length === 0" description="暂无关联节点，请先为模板分配节点分组并添加成员" />
      </div>
      <template #footer>
        <el-button @click="billingNodesDialog=false">取消</el-button>
        <el-button type="primary" @click="saveBillingNodes">保存</el-button>
      </template>
    </el-dialog>

    <!-- 分组编辑弹窗 -->
    <el-dialog v-model="groupDialog" :title="editingGroup?'编辑分组':'新增分组'" width="450px">
      <el-form :model="groupForm" label-width="80px" size="small">
        <el-form-item label="名称"><el-input v-model="groupForm.name" /></el-form-item>
        <el-form-item label="描述"><el-input v-model="groupForm.description" type="textarea" :rows="2" /></el-form-item>
        <el-form-item label="区域">
          <el-select v-model="groupForm.region" style="width:100%" clearable>
            <el-option label="香港" value="HK" />
            <el-option label="日本" value="JP" />
            <el-option label="美国" value="US" />
            <el-option label="新加坡" value="SG" />
            <el-option label="欧洲" value="EU" />
            <el-option label="中国" value="CN" />
          </el-select>
        </el-form-item>
        <el-form-item label="等级">
          <el-select v-model="groupForm.tier" style="width:100%">
            <el-option label="标准节点" value="standard" />
            <el-option label="高级节点" value="premium" />
            <el-option label="独享节点" value="exclusive" />
          </el-select>
        </el-form-item>
        <el-form-item label="排序"><el-input-number v-model="groupForm.sort_order" :min="0" /></el-form-item>
        <el-form-item label="启用"><el-switch v-model="groupForm.enabled" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="groupDialog=false">取消</el-button>
        <el-button type="primary" @click="saveGroup">保存</el-button>
      </template>
    </el-dialog>

    <!-- 分组成员管理弹窗 -->
    <el-dialog v-model="membersDialog" :title="currentGroup?.name + ' - 成员管理'" width="650px">
      <div style="margin-bottom:12px;display:flex;gap:8px;flex-wrap:wrap">
        <el-select v-model="newMemberInboundId" size="small" filterable placeholder="选择入站配置" style="width:320px">
          <el-option v-for="ib in inbounds" :key="ib.id" :label="(ib.server_name || '本地节点') + ' - ' + (ib.remark || ib.tag) + ' (' + ib.protocol + ':' + ib.port + ')'" :value="ib.id" />
        </el-select>
        <el-button size="small" type="primary" @click="addInboundMember">添加入站</el-button>
        <el-select v-model="newMemberExtNodeId" size="small" filterable placeholder="选择外部节点" style="width:260px">
          <el-option v-for="en in extNodes" :key="en.id" :label="en.name + ' (' + en.host + ':' + en.port + ')'" :value="en.id" />
        </el-select>
        <el-button size="small" type="success" @click="addExtNodeMember">添加外部节点</el-button>
      </div>
      <el-table :data="groupMembers" stripe size="small">
        <el-table-column prop="id" label="ID" width="50" />
        <el-table-column label="主机节点" width="120">
          <template #default="{row}">
            <template v-if="row.inbound_config_id">{{ inboundServerName(row.inbound_config_id) }}</template>
            <template v-else-if="row.external_node_id">外部节点</template>
            <template v-else>—</template>
          </template>
        </el-table-column>
        <el-table-column label="入口/节点名称">
          <template #default="{row}">
            <template v-if="row.inbound_config_id">{{ inboundName(row.inbound_config_id) }}</template>
            <template v-else-if="row.external_node_id">{{ extNodeName(row.external_node_id) }}</template>
            <template v-else>—</template>
          </template>
        </el-table-column>
        <el-table-column label="类型" width="80">
          <template #default="{row}">
            <el-tag v-if="row.inbound_config_id" size="small">入站</el-tag>
            <el-tag v-else-if="row.external_node_id" size="small" type="success">外部</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="80">
          <template #default="{row}">
            <el-button size="small" type="danger" @click="removeMember(row)">移除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-dialog>

    <!-- 优惠码编辑弹窗 -->
    <el-dialog v-model="promoDialog" :title="editingPromo?'编辑优惠码':'新增优惠码'" width="550px">
      <el-form :model="promoForm" label-width="100px" size="small">
        <el-form-item label="优惠码"><el-input v-model="promoForm.code" :disabled="!!editingPromo" /></el-form-item>
        <el-form-item label="名称"><el-input v-model="promoForm.name" /></el-form-item>
        <el-form-item label="描述"><el-input v-model="promoForm.description" type="textarea" :rows="2" /></el-form-item>
        <el-form-item label="优惠类型">
          <el-select v-model="promoForm.discount_type" style="width:100%">
            <el-option label="百分比折扣" value="percentage" />
            <el-option label="固定金额减免" value="fixed_amount" />
          </el-select>
        </el-form-item>
        <el-form-item label="优惠值">
          <el-input-number v-model="promoForm.discount_value" :min="0" />
          <span style="font-size:12px;color:#909399;margin-left:8px">
            {{ promoForm.discount_type === 'percentage' ? '百分比 (1-100)' : '金额（分）' }}
          </span>
        </el-form-item>
        <el-form-item v-if="promoForm.discount_type === 'percentage'" label="最大减免">
          <el-input-number v-model="promoForm.max_discount_amount" :min="0" />
          <span style="font-size:12px;color:#909399;margin-left:8px">0=不限（分）</span>
        </el-form-item>
        <el-form-item label="适用范围">
          <el-select v-model="promoForm.applicable_scope" style="width:100%">
            <el-option label="全部" value="all" />
            <el-option label="指定模板" value="template" />
            <el-option label="指定规格" value="specific_variant" />
          </el-select>
        </el-form-item>
        <el-form-item label="总使用次数"><el-input-number v-model="promoForm.max_uses" :min="0" /> <span style="font-size:12px;color:#909399">0=不限</span></el-form-item>
        <el-form-item label="每用户次数"><el-input-number v-model="promoForm.max_uses_per_user" :min="1" /></el-form-item>
        <el-form-item label="生效时间"><el-date-picker v-model="promoForm.starts_at" type="datetime" value-format="YYYY-MM-DDTHH:mm:ssZ" style="width:100%" /></el-form-item>
        <el-form-item label="结束时间"><el-date-picker v-model="promoForm.ends_at" type="datetime" value-format="YYYY-MM-DDTHH:mm:ssZ" style="width:100%" /></el-form-item>
        <el-form-item label="启用"><el-switch v-model="promoForm.enabled" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="promoDialog=false">取消</el-button>
        <el-button type="primary" @click="savePromo">保存</el-button>
      </template>
    </el-dialog>

  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import client from '../api/client'
import * as planApi from '../api/plan'

const activeTab = ref('templates')

// ──────── Templates ────────
const templates = ref([])
const templateDialog = ref(false)
const editingTemplate = ref(null)
const templateForm = ref({})

function emptyTemplateForm() {
  return {
    name: '', description: '', category: 'standard',
    device_limit: 0, speed_limit_mbps: 0, concurrent_limit: 0,
    default_quota_gb: 0, default_quota_reset_cycle: 'none',
    suspend_on_quota_exhausted: 1, is_trial: false, trial_duration_hours: 0,
    sort_order: 0, enabled: true,
  }
}
async function fetchTemplates() {
  try { templates.value = (await planApi.listPlanTemplates()) || [] } catch {}
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
    ElMessage.success('保存成功')
    templateDialog.value = false
    fetchTemplates()
  } catch {}
}
async function deleteTemplate(row) {
  await ElMessageBox.confirm(`确定删除模板「${row.name}」？`, '确认')
  try {
    await planApi.deletePlanTemplate(row.id)
    ElMessage.success('删除成功')
    fetchTemplates()
  } catch {}
}

// ──────── Account Sync ────────
const syncLoading = ref({})
const syncAllLoading = ref(false)

async function handleSyncTemplate(tpl) {
  try {
    await ElMessageBox.confirm(
      `确定为套餐「${tpl.name}」的所有已绑定客户端在所有节点上同步创建账号？\n\n此操作会检查每个节点上的 inbound config，为缺少账号的客户端自动创建。已存在的账号不会被重复创建。`,
      '同步账号确认',
      { type: 'warning' }
    )
  } catch {
    return
  }
  syncLoading.value[tpl.id] = true
  try {
    const res = await planApi.syncAccountsForTemplate(tpl.id)
    ElMessage.success(`同步完成，处理了 ${res.inbounds_processed || 0} 个节点`)
  } catch {
    ElMessage.error('同步失败')
  } finally {
    syncLoading.value[tpl.id] = false
  }
}

async function handleSyncAll() {
  try {
    await ElMessageBox.confirm(
      '确定为所有已启用套餐的所有已绑定客户端在所有节点上同步创建账号？\n\n此操作会检查每个节点上的 inbound config，为缺少账号的客户端自动创建。已存在的账号不会被重复创建。',
      '全量同步账号确认',
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
    ElMessage.success(`全量同步完成，${Object.keys(templates).length} 个套餐，共处理 ${total} 个节点`)
  } catch {
    ElMessage.error('全量同步失败')
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
  try { variants.value = (await planApi.listPlanVariants(tpl.id)) || [] } catch { variants.value = [] }
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
    ElMessage.success('保存成功')
    variantDialog.value = false
    showVariantsDialog(currentTemplate.value)
  } catch {}
}
async function deleteVariant(row) {
  await ElMessageBox.confirm(`确定删除规格「${row.name}」？`, '确认')
  try {
    await planApi.deletePlanVariant(row.id)
    ElMessage.success('删除成功')
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
    ElMessage.success('保存成功')
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
    ElMessage.success('保存成功')
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
    ElMessage.success('保存成功')
    groupDialog.value = false
    fetchNodeGroups()
  } catch {}
}
async function deleteGroup(row) {
  await ElMessageBox.confirm(`确定删除分组「${row.name}」？`, '确认')
  try {
    await planApi.deleteNodeGroup(row.id)
    ElMessage.success('删除成功')
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
    ElMessage.success('添加成功')
    newMemberInboundId.value = null
    showMembersDialog(currentGroup.value)
  } catch {}
}
async function addExtNodeMember() {
  if (!newMemberExtNodeId.value) return
  try {
    await planApi.addGroupMember(currentGroup.value.id, { external_node_id: newMemberExtNodeId.value })
    ElMessage.success('添加成功')
    newMemberExtNodeId.value = null
    showMembersDialog(currentGroup.value)
  } catch {}
}
async function removeMember(row) {
  try {
    await planApi.removeGroupMember(currentGroup.value.id, row.id)
    ElMessage.success('移除成功')
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
    ElMessage.success('保存成功')
    promoDialog.value = false
    fetchPromotions()
  } catch {}
}
async function deletePromo(row) {
  await ElMessageBox.confirm(`确定删除优惠码「${row.code}」？`, '确认')
  try {
    await planApi.deletePromotion(row.id)
    ElMessage.success('删除成功')
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

// ──────── Traffic Packs ────────
const allTrafficPacks = computed(() => {
  // Collect all traffic_pack variants across all templates
  const packs = []
  for (const tpl of templates.value) {
    // We need to load variants for each template to show traffic packs
    // Since variants are loaded on-demand, use a separate fetch for traffic packs
  }
  return trafficPackVariants.value
})
const trafficPacksLoading = ref(false)
const trafficPackVariants = ref([])

async function fetchTrafficPacks() {
  trafficPacksLoading.value = true
  try {
    const tpls = templates.value
    const allVariants = []
    for (const tpl of tpls) {
      try {
        const vars = (await planApi.listPlanVariants(tpl.id)) || []
        for (const v of vars) {
          if (v.variant_type === 'traffic_pack') {
            allVariants.push({ ...v, template_id: tpl.id, template_name: tpl.name })
          }
        }
      } catch {}
    }
    trafficPackVariants.value = allVariants
  } finally {
    trafficPacksLoading.value = false
  }
}

function templateName(templateId) {
  const tpl = templates.value.find(t => t.id === templateId)
  return tpl ? tpl.name : '#'+templateId
}

async function showEditTrafficPackVariant(row) {
  const tpl = templates.value.find(t => t.id === row.template_id)
  if (tpl) {
    currentTemplate.value = tpl
    showEditVariant(row)
  }
}

async function deleteTrafficPackVariant(row) {
  await ElMessageBox.confirm(`确定删除流量包「${row.name}」？`, '确认')
  try {
    await planApi.deletePlanVariant(row.id)
    ElMessage.success('删除成功')
    fetchTemplates()
  } catch {}
}

// ──────── Helpers ────────
const formatBytes = (b) => {
  if (!b || b <= 0) return '不限'
  if (b >= 1073741824) return (b / 1073741824).toFixed(1) + ' GB'
  if (b >= 1048576) return (b / 1048576).toFixed(0) + ' MB'
  return (b / 1024).toFixed(0) + ' KB'
}
const categoryLabel = (c) => ({ basic:'基础版', standard:'标准版', premium:'高级版', trial:'试用版', custom:'自定义' })[c] || c
const categoryType = (c) => ({ basic:'info', standard:'', premium:'warning', trial:'danger', custom:'success' })[c] || ''
const tierLabel = (t) => ({ standard:'标准', premium:'高级', exclusive:'独享' })[t] || t
const tierType = (t) => ({ standard:'', premium:'warning', exclusive:'danger' })[t] || ''
const cycleLabel = (c) => ({ monthly:'月付', quarterly:'季付', semi_annual:'半年付', annual:'年付', lifetime:'终身', one_time:'一次性' })[c] || c
const scopeLabel = (s) => ({ all:'全部', template:'指定模板', specific_variant:'指定规格' })[s] || s
const orderTypeLabel = (t) => ({ new:'新购', renew:'续费', upgrade:'升级', downgrade:'降级', trial_convert:'试用转正' })[t] || t
const orderTypeColor = (t) => ({ new:'success', renew:'', upgrade:'warning', downgrade:'info', trial_convert:'danger' })[t] || ''
const orderStatusLabel = (s) => ({ pending:'待支付', paid:'已支付', completed:'已完成', cancelled:'已取消', refunded:'已退款' })[s] || s
const orderStatusColor = (s) => ({ pending:'warning', paid:'info', completed:'success', cancelled:'danger', refunded:'danger' })[s] || ''
const inboundName = (id) => { const ib = inbounds.value.find(i => i.id === id); return ib ? (ib.remark || ib.tag) : '#' + id }
const inboundServerName = (id) => { const ib = inbounds.value.find(i => i.id === id); return ib ? (ib.server_name || '本地节点') : '—' }
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
