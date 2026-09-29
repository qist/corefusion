<template>
  <div>
    <el-tabs v-model="activeTab" type="border-card">

      <!-- ════════════ 充值站点 Tab ════════════ -->
      <el-tab-pane :label="t('charge.sitesTab')" name="sites">
        <div style="display:flex;justify-content:space-between;margin-bottom:16px">
          <h3 style="margin:0">{{ t('charge.sitesTab') }}</h3>
          <el-button type="primary" size="small" @click="showCreateSite">+ {{ t('charge.addSite') }}</el-button>
        </div>
        <el-table :data="sites" v-loading="sitesLoading" stripe size="small">
          <el-table-column prop="id" label="ID" width="60" />
          <el-table-column prop="name" :label="t('common.name')" width="150" />
          <el-table-column prop="site_url" :label="t('charge.siteUrl')" show-overflow-tooltip />
          <el-table-column :label="t('charge.paymentMethods')" width="200">
            <template #default="{row}">
              <template v-if="parseMethods(row.supported_methods).length">
                <el-tag v-for="m in parseMethods(row.supported_methods)" :key="m" size="small" style="margin:2px">{{ m }}</el-tag>
              </template>
              <span v-else style="color:#999">—</span>
            </template>
          </el-table-column>
          <el-table-column :label="t('common.status')" width="80">
            <template #default="{row}">
              <el-tag :type="row.enabled?'success':'danger'" size="small">{{ row.enabled?t('common.enabled'):t('common.disabled') }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column :label="t('charge.verification')" width="80">
            <template #default="{row}">
              <el-tag :type="row.verified?'success':'danger'" size="small">{{ row.verified?t('charge.verified'):t('charge.notVerified') }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="currency" :label="t('charge.currency')" width="70" />
          <el-table-column prop="sort_order" :label="t('charge.sortOrder')" width="60" />
          <el-table-column :label="t('common.actions')" width="350">
            <template #default="{row}">
              <div style="display:flex;gap:2px;align-items:center;white-space:nowrap">
                <el-button size="small" @click="showEditSite(row)">{{ t('common.edit') }}</el-button>
                <el-button size="small" type="primary" @click="showVerifyDialog(row)">{{ t('charge.verify') }}</el-button>
                <el-button size="small" type="warning" @click="rotateSiteSecret(row)">{{ t('charge.rotateSecret') }}</el-button>
                <el-button size="small" type="info" @click="showIntegrationDoc(row)">{{ t('charge.doc') }}</el-button>
                <el-button size="small" :type="row.enabled?'warning':'success'" @click="toggleSite(row)">{{ row.enabled?t('common.disabled'):t('common.enabled') }}</el-button>
                <el-button size="small" type="danger" @click="deleteSite(row)">{{ t('common.delete') }}</el-button>
              </div>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>

      <!-- ════════════ 充值订单 Tab ════════════ -->
      <el-tab-pane :label="t('charge.ordersTab')" name="orders">
        <div style="display:flex;justify-content:space-between;margin-bottom:16px">
          <h3 style="margin:0">{{ t('charge.ordersTab') }}</h3>
          <div style="display:flex;gap:8px;align-items:center">
            <el-select v-model="orderStatusFilter" size="small" :placeholder="t('charge.statusFilter')" clearable style="width:120px">
              <el-option :label="t('charge.statusPending')" value="pending" />
              <el-option :label="t('charge.statusPaid')" value="paid" />
              <el-option :label="t('charge.statusFailed')" value="failed" />
              <el-option :label="t('charge.statusExpired')" value="expired" />
            </el-select>
            <el-button size="small" @click="fetchOrders">{{ t('common.search') }}</el-button>
            <el-button type="info" size="small" @click="doExportOrdersCSV">📥 {{ t('charge.exportCSV') }}</el-button>
          </div>
        </div>
        <el-table :data="orders" v-loading="ordersLoading" stripe size="small">
          <el-table-column prop="id" label="ID" width="60" />
          <el-table-column prop="client_id" :label="t('charge.clientId')" width="90" />
          <el-table-column prop="client_email" :label="t('charge.email')" min-width="180" show-overflow-tooltip />
          <el-table-column prop="trade_no" :label="t('charge.tradeNo')" width="200" show-overflow-tooltip />
          <el-table-column :label="t('charge.source')" width="80">
            <template #default="{row}">
              <el-tag :type="row.source==='site'?'':'success'" size="small">{{ sourceLabel(row.source) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column :label="t('common.type')" width="80">
            <template #default="{row}">
              <el-tag :type="row.order_type==='charge'?'':'warning'" size="small">{{ row.order_type==='charge'?t('charge.typeCharge'):t('charge.typePlan') }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column :label="t('currency.amount')" width="120">
            <template #default="{row}">{{ formatAmount(row.amount, row.currency || 'CNY') }}</template>
          </el-table-column>
          <el-table-column :label="t('charge.paymentMethods')" width="90">
            <template #default="{row}">{{ row.method || '—' }}</template>
          </el-table-column>
          <el-table-column :label="t('common.status')" width="80">
            <template #default="{row}">
              <el-tag :type="chargeOrderStatusColor(row.status)" size="small">{{ chargeOrderStatusLabel(row.status) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column :label="t('common.createdAt')" width="160">
            <template #default="{row}">{{ row.created_at?.substring(0,16).replace('T',' ') }}</template>
          </el-table-column>
          <el-table-column :label="t('charge.paidAt')" width="160">
            <template #default="{row}">{{ row.paid_at?.substring(0,16).replace('T',' ') || '—' }}</template>
          </el-table-column>
          <el-table-column :label="t('common.actions')" width="140">
            <template #default="{row}">
              <div style="display:flex;gap:2px;align-items:center;white-space:nowrap">
                <el-button size="small" @click="showOrderDetail(row)">{{ t('charge.detail') }}</el-button>
                <el-button size="small" type="danger" @click="refundOrder(row)" v-if="row.status==='paid'">{{ t('charge.refund') }}</el-button>
              </div>
            </template>
          </el-table-column>
        </el-table>
        <div style="display:flex;justify-content:center;margin-top:16px">
          <PaginationBar v-model:page="orderPage" v-model:page-size="orderPageSize" :total="orderTotal" @change="fetchOrders" />
        </div>
      </el-tab-pane>

      <!-- ════════════ 充值卡 Tab ════════════ -->
      <el-tab-pane :label="t('charge.cardsTab')" name="cards">
        <div style="display:flex;justify-content:space-between;margin-bottom:16px">
          <h3 style="margin:0">{{ t('charge.cardsTab') }}</h3>
          <div style="display:flex;gap:8px">
            <el-button type="primary" size="small" @click="showCreateCard">+ {{ t('charge.addCard') }}</el-button>
            <el-button type="success" size="small" @click="showBatchCreate">{{ t('charge.batchCreate') }}</el-button>
            <el-button type="info" size="small" @click="doExportCSV">📥 {{ t('charge.exportCSV') }}</el-button>
          </div>
        </div>
        <div style="display:flex;gap:8px;margin-bottom:12px;flex-wrap:wrap">
          <el-select v-model="cardStatusFilter" size="small" :placeholder="t('common.status')" clearable style="width:100px">
            <el-option :label="t('charge.cardActive')" value="active" />
            <el-option :label="t('charge.cardUsed')" value="used" />
            <el-option :label="t('charge.statusExpired')" value="expired" />
            <el-option :label="t('charge.cardDisabled')" value="disabled" />
          </el-select>
          <el-select v-model="cardTypeFilter" size="small" :placeholder="t('common.type')" clearable style="width:120px">
            <el-option :label="t('charge.cardTypeBalance')" value="balance" />
            <el-option :label="t('charge.cardTypePlan')" value="plan" />
            <el-option :label="t('charge.cardTypeTrafficPack')" value="traffic_pack" />
          </el-select>
          <el-select v-model="cardAgentFilter" size="small" :placeholder="t('charge.agentPlaceholder')" clearable style="width:140px">
            <el-option :label="t('charge.official')" :value="0" />
            <el-option v-for="u in agentUsers" :key="u.id" :label="u.username" :value="u.id" />
          </el-select>
          <el-button size="small" @click="fetchCards">{{ t('common.search') }}</el-button>
        </div>
        <el-table :data="cards" v-loading="cardsLoading" stripe size="small">
          <el-table-column prop="id" label="ID" width="60" />
          <el-table-column :label="t('charge.cardCode')" min-width="200">
            <template #default="{row}">
              <span style="font-family:monospace">{{ row.card_code_masked || row.card_code }}</span>
            </template>
          </el-table-column>
          <el-table-column :label="t('common.type')" width="100">
            <template #default="{row}">
              <el-tag :type="row.card_type==='balance'?'':row.card_type==='plan'?'warning':'success'" size="small">{{ cardTypeLabel(row.card_type) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column :label="t('charge.faceValue')" width="200" show-overflow-tooltip>
            <template #default="{row}">
              <span v-if="row.card_type==='balance'">{{ formatAmount(row.value, row.currency || 'CNY') }}</span>
              <el-tag v-else type="warning" size="small">{{ findVariantName(row.value) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column :label="t('charge.usage')" width="70">
            <template #default="{row}">
              <el-tag :type="row.status==='active'?'success':'info'" size="small">{{ row.status==='active'?t('charge.unused'):t('charge.used') }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="batch_id" :label="t('charge.batch')" width="140" show-overflow-tooltip />
          <el-table-column :label="t('charge.creator')" width="100">
            <template #default="{row}">
              <span v-if="row.created_by_admin_name">{{ row.created_by_admin_name }}</span>
              <span v-else style="color:#999">—</span>
            </template>
          </el-table-column>
          <el-table-column :label="t('charge.agent')" width="100">
            <template #default="{row}">
              <span v-if="row.agent_name">{{ row.agent_name }}</span>
              <span v-else style="color:#999">{{ t('charge.official') }}</span>
            </template>
          </el-table-column>
          <el-table-column :label="t('common.status')" width="80">
            <template #default="{row}">
              <el-tag :type="cardStatusColor(row.status)" size="small">{{ cardStatusLabel(row.status) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column :label="t('charge.expiresAt')" width="160">
            <template #default="{row}">{{ row.expires_at?.substring(0,10) || t('charge.neverExpires') }}</template>
          </el-table-column>
          <el-table-column :label="t('common.actions')" width="150">
            <template #default="{row}">
              <div style="display:flex;gap:2px;align-items:center;white-space:nowrap">
                <el-button size="small" @click="showCardUsages(row)">{{ t('charge.usageRecord') }}</el-button>
                <el-button size="small" type="danger" @click="deleteCard(row)">{{ t('common.delete') }}</el-button>
              </div>
            </template>
          </el-table-column>
        </el-table>
        <div style="display:flex;justify-content:center;margin-top:16px">
          <PaginationBar v-model:page="cardPage" v-model:page-size="cardPageSize" :total="cardTotal" @change="fetchCards" />
        </div>
      </el-tab-pane>

      <!-- ════════════ 客户余额 Tab ════════════ -->
      <el-tab-pane :label="t('charge.balancesTab')" name="balances">
        <div style="display:flex;justify-content:space-between;margin-bottom:16px">
          <h3 style="margin:0">{{ t('charge.balancesTab') }}</h3>
          <el-button type="info" size="small" @click="doExportBalanceTxCSV">📥 {{ t('charge.exportCSV') }}</el-button>
        </div>
        <el-table :data="balances" v-loading="balancesLoading" stripe size="small">
          <el-table-column prop="id" label="ID" width="60" />
          <el-table-column prop="client_id" :label="t('charge.clientId')" width="100" />
          <el-table-column prop="client_email" :label="t('charge.email')" min-width="180" show-overflow-tooltip />
          <el-table-column prop="client_nickname" :label="t('charge.nickname')" width="120" show-overflow-tooltip />
          <el-table-column :label="t('currency.balance')" width="140">
            <template #default="{row}">{{ formatAmount(row.balance, row.currency || 'CNY') }}</template>
          </el-table-column>
          <el-table-column :label="t('currency.frozen')" width="140">
            <template #default="{row}">{{ formatAmount(row.frozen, row.currency || 'CNY') }}</template>
          </el-table-column>
          <el-table-column :label="t('common.actions')" width="100">
            <template #default="{row}">
              <el-button size="small" @click="showTransactions(row)">{{ t('charge.transactions') }}</el-button>
            </template>
          </el-table-column>
        </el-table>
        <div style="display:flex;justify-content:center;margin-top:16px">
          <PaginationBar v-model:page="balancePage" v-model:page-size="balancePageSize" :total="balanceTotal" @change="fetchBalances" />
        </div>
      </el-tab-pane>

    </el-tabs>

    <!-- ═══════════════════════════════════════════════ -->
    <!-- 充值站点编辑弹窗 -->
    <el-dialog v-model="siteDialog" :title="editingSite?t('charge.editSite'):t('charge.createSite')" width="620px">
      <el-form :model="siteForm" label-width="110px" size="small">
        <el-form-item :label="t('common.name')"><el-input v-model="siteForm.name" /></el-form-item>
        <el-form-item :label="t('charge.siteUrl')"><el-input v-model="siteForm.site_url" placeholder="https://pay.example.com" /></el-form-item>
        <el-form-item label="API Key"><el-input v-model="siteForm.api_key" type="password" show-password /></el-form-item>
        <el-form-item :label="t('charge.callbackSecret')">
          <div style="display:flex;gap:8px;width:100%">
            <el-input v-model="siteForm.callback_secret" type="password" show-password :placeholder="t('charge.secretAutoGen')" style="flex:1" />
            <el-button @click="generateSecretField">🎲 {{ t('charge.autoGenerate') }}</el-button>
          </div>
          <div style="font-size:11px;color:#999;margin-top:4px">
            <span v-if="editingSite">{{ t('charge.secretEditHint') }}</span>
            <span v-else>{{ t('charge.secretCreateHint') }}</span>
          </div>
        </el-form-item>
        <el-form-item :label="t('charge.ipWhitelist')">
          <el-input v-model="siteForm.callback_whitelist" type="textarea" :rows="2" :placeholder="t('charge.ipWhitelistPlaceholder')" />
          <div style="font-size:11px;color:#999;margin-top:4px">{{ t('charge.ipWhitelistHint') }}</div>
        </el-form-item>
        <el-form-item :label="t('charge.currency')">
          <el-select v-model="siteForm.currency" style="width:100%">
            <el-option v-for="c in CURRENCIES" :key="c.code" :label="`${c.code} ${c.symbol} (${t(c.nameKey)})`" :value="c.code" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('charge.paymentMethods')">
          <el-select v-model="siteForm.supported_methods_arr" multiple filterable allow-create default-first-option style="width:100%" :placeholder="t('charge.selectPayMethod')">
            <el-option :label="t('charge.alipay')" value="alipay" />
            <el-option :label="t('charge.wechatPay')" value="wechat" />
            <el-option label="USDT" value="usdt" />
            <el-option label="PayPal" value="paypal" />
            <el-option :label="t('charge.creditCard')" value="credit_card" />
            <el-option :label="t('charge.debitCard')" value="debit_card" />
            <el-option :label="t('charge.bankTransfer')" value="bank_transfer" />
            <el-option label="QIWI" value="qiwi" />
            <el-option label="ЮMoney" value="yoomoney" />
            <el-option label="SberPay" value="sberpay" />
            <el-option label="Tinkoff" value="tinkoff" />
            <el-option label="Crypto" value="crypto" />
          </el-select>
          <div style="font-size:11px;color:#999;margin-top:4px">{{ t('charge.customPayMethodHint') }}</div>
        </el-form-item>
        <el-form-item :label="t('charge.iconUrl')"><el-input v-model="siteForm.icon_url" /></el-form-item>
        <el-form-item :label="t('charge.sortOrder')"><el-input-number v-model="siteForm.sort_order" :min="0" /></el-form-item>
        <el-form-item :label="t('common.enabled')"><el-switch v-model="siteForm.enabled" /></el-form-item>

        <!-- 回调地址展示 -->
        <el-divider content-position="left">{{ t('charge.callbackConfig') }}</el-divider>
        <el-form-item v-if="editingSite" :label="t('charge.callbackUrl')">
          <el-input :model-value="`/api/v1/charge/callback/${editingSite.id}`" readonly>
            <template #append><el-button @click="copyCallbackUrl(editingSite.id)">{{ t('common.copy') }}</el-button></template>
          </el-input>
          <div style="font-size:11px;color:#999;margin-top:4px">{{ t('charge.callbackHint') }}</div>
        </el-form-item>
        <el-form-item v-if="editingSite" :label="t('charge.verifyStatus')">
          <el-tag :type="editingSite.verified?'success':'danger'" size="small">{{ editingSite.verified?t('charge.verified'):t('charge.notVerified') }}</el-tag>
          <span style="margin-left:8px;font-size:12px;color:#909399">{{ t('charge.unverifiedWarning') }}</span>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="siteDialog=false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="saveSite">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- ═══════════════════════════════════════════════ -->
    <!-- 域名验证弹窗 -->
    <el-dialog v-model="verifyDialog" :title="t('charge.domainVerify')" width="600px">
      <div v-if="verifySite">
        <el-alert :type="verifySite.verified?'success':'warning'" :closable="false" style="margin-bottom:16px">
          <template #title>
            {{ verifySite.verified ? t('charge.domainVerified') : t('charge.domainNotVerified') }}
          </template>
        </el-alert>

        <el-steps :active="verifyStep" align-center style="margin-bottom:20px">
          <el-step :title="t('charge.genVerifyFile')" />
          <el-step :title="t('charge.deployVerify')" />
          <el-step :title="t('charge.verifyDomain')" />
        </el-steps>

        <div v-if="verifyData">
          <el-descriptions :column="1" border size="small">
            <el-descriptions-item :label="t('charge.verifyFileUrl')">
              <span style="word-break:break-all;font-family:monospace">{{ verifyData.verify_file_url }}</span>
            </el-descriptions-item>
            <el-descriptions-item :label="t('charge.fileContent')">
              <span style="font-family:monospace">{{ verifyData.verify_content }}</span>
            </el-descriptions-item>
            <el-descriptions-item :label="t('charge.dnsTxtName')">_corefusion-verify</el-descriptions-item>
            <el-descriptions-item :label="t('charge.dnsTxtValue')">
              <span style="font-family:monospace">{{ verifyData.verify_content }}</span>
            </el-descriptions-item>
          </el-descriptions>

          <div style="margin-top:16px;text-align:center">
            <el-button type="primary" :loading="verifyLoading" @click="doVerifyHTTP">{{ t('charge.verifyHttp') }}</el-button>
            <el-button type="success" :loading="verifyLoading" @click="doVerifyDNS">{{ t('charge.verifyDns') }}</el-button>
          </div>

          <div style="margin-top:12px;font-size:12px;color:#909399;text-align:center">
            {{ t('charge.verifyMethod1') }} <code>.well-known/</code> {{ t('charge.directory') }}<br>
            {{ t('charge.verifyMethod2') }} <code>_corefusion-verify.{{ t('charge.yourDomain') }}</code>
          </div>
        </div>

        <div v-else style="text-align:center;padding:20px">
          <el-button type="primary" :loading="verifyLoading" @click="doGenerateVerify">{{ t('charge.genVerifyFileBtn') }}</el-button>
        </div>
      </div>
    </el-dialog>

    <!-- ═══════════════════════════════════════════════ -->
    <!-- 集成文档弹窗 -->
    <el-dialog v-model="integrationDocDialog" :title="t('charge.integrationDoc')" width="650px">
      <div v-if="integrationDoc">
        <el-descriptions :column="1" border size="small">
          <el-descriptions-item :label="t('charge.siteId')">{{ integrationDoc.site_id }}</el-descriptions-item>
          <el-descriptions-item :label="t('charge.siteName')">{{ integrationDoc.site_name }}</el-descriptions-item>
          <el-descriptions-item :label="t('charge.callbackUrl')">
            <span style="font-family:monospace;color:#409eff">{{ integrationDoc.callback_url }}</span>
          </el-descriptions-item>
          <el-descriptions-item :label="t('charge.signAlgorithm')">{{ integrationDoc.signature_algorithm }}</el-descriptions-item>
          <el-descriptions-item :label="t('charge.signFields')">
            <el-tag v-for="f in integrationDoc.signature_fields" :key="f" size="small" style="margin:2px">{{ f }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item :label="t('charge.timestampTolerance')">{{ integrationDoc.timestamp_tolerance }} {{ t('charge.seconds') }}</el-descriptions-item>
          <el-descriptions-item :label="t('charge.nonceRequirement')">{{ integrationDoc.nonce_required ? t('charge.required') : t('charge.optional') }}</el-descriptions-item>
          <el-descriptions-item :label="t('charge.amountUnit')">{{ integrationDoc.amount_unit }}</el-descriptions-item>
          <el-descriptions-item :label="t('charge.ipWhitelist')">{{ integrationDoc.ip_whitelist || t('charge.notConfigured') }}</el-descriptions-item>
          <el-descriptions-item :label="t('charge.verifyStatus')">
            <el-tag :type="integrationDoc.verified?'success':'danger'" size="small">{{ integrationDoc.verified?t('charge.verified'):t('charge.notVerified') }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item :label="t('charge.callbackSecret')">{{ integrationDoc.callback_secret_set?t('charge.configured'):t('charge.notConfigured') }}</el-descriptions-item>
        </el-descriptions>

        <el-divider content-position="left">{{ t('charge.callbackExample') }}</el-divider>
        <pre style="background:#f5f7fa;padding:12px;border-radius:4px;font-size:12px;overflow-x:auto">{{ JSON.stringify(integrationDoc.example_callback, null, 2) }}</pre>

        <el-divider content-position="left">{{ t('charge.successResponse') }}</el-divider>
        <pre style="background:#f0f9eb;padding:12px;border-radius:4px;font-size:12px">{{ integrationDoc.response_format }}</pre>

        <el-divider content-position="left">{{ t('charge.signMethod') }}</el-divider>
        <div style="font-size:12px;color:#606266;line-height:1.8">
          <p>1. {{ t('charge.signStep1') }}</p>
          <p>2. {{ t('charge.signStep2') }}</p>
          <p>3. {{ t('charge.signStep3') }}</p>
        </div>
      </div>
    </el-dialog>

    <!-- 充值订单详情弹窗 -->
    <el-dialog v-model="orderDetailDialog" :title="t('charge.orderDetail')" width="600px">
      <el-descriptions :column="2" border size="small" v-if="currentOrder">
        <el-descriptions-item label="ID">{{ currentOrder.id }}</el-descriptions-item>
        <el-descriptions-item :label="t('charge.tradeNo')">{{ currentOrder.trade_no }}</el-descriptions-item>
        <el-descriptions-item :label="t('charge.clientId')">{{ currentOrder.client_id }}</el-descriptions-item>
        <el-descriptions-item :label="t('charge.source')">{{ sourceLabel(currentOrder.source) }}</el-descriptions-item>
        <el-descriptions-item :label="t('common.type')">{{ currentOrder.order_type==='charge'?t('charge.typeCharge'):t('charge.typePlan') }}</el-descriptions-item>
        <el-descriptions-item :label="t('currency.amount')">{{ formatAmount(currentOrder.amount || 0, currentOrder.currency || 'CNY') }}</el-descriptions-item>
        <el-descriptions-item :label="t('charge.paidCurrency')">{{ currentOrder.paid_currency || '—' }}</el-descriptions-item>
        <el-descriptions-item :label="t('charge.paidAmount')">{{ currentOrder.paid_amount ? formatAmount(currentOrder.paid_amount, currentOrder.paid_currency || 'CNY') : '—' }}</el-descriptions-item>
        <el-descriptions-item :label="t('charge.exchangeRate')">{{ currentOrder.exchange_rate || '—' }}</el-descriptions-item>
        <el-descriptions-item :label="t('charge.paymentMethods')">{{ currentOrder.method || '—' }}</el-descriptions-item>
        <el-descriptions-item :label="t('common.status')">
          <el-tag :type="chargeOrderStatusColor(currentOrder.status)" size="small">{{ chargeOrderStatusLabel(currentOrder.status) }}</el-tag>
        </el-descriptions-item>
        <el-descriptions-item :label="t('charge.siteTradeNo')">{{ currentOrder.site_trade_no || '—' }}</el-descriptions-item>
        <el-descriptions-item :label="t('common.createdAt')">{{ currentOrder.created_at?.replace('T',' ').substring(0,19) }}</el-descriptions-item>
        <el-descriptions-item :label="t('charge.paidAt')">{{ currentOrder.paid_at?.replace('T',' ').substring(0,19) || '—' }}</el-descriptions-item>
        <el-descriptions-item :label="t('charge.expiresAt')">{{ currentOrder.expires_at?.replace('T',' ').substring(0,19) || '—' }}</el-descriptions-item>
        <el-descriptions-item :label="t('charge.remark')">{{ currentOrder.remark || '—' }}</el-descriptions-item>
      </el-descriptions>
    </el-dialog>

    <!-- 新增充值卡弹窗 -->
    <el-dialog v-model="cardDialog" :title="t('charge.createCard')" width="560px">
      <el-form :model="cardForm" label-width="90px" size="small">
        <el-form-item :label="t('common.type')">
          <el-select v-model="cardForm.card_type" style="width:100%">
            <el-option :label="t('charge.cardTypeBalance')" value="balance" />
            <el-option :label="t('charge.cardTypePlan')" value="plan" />
            <el-option :label="t('charge.cardTypeTrafficPack')" value="traffic_pack" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="cardForm.card_type==='balance'" :label="t('currency.amount')">
          <div style="display:flex;gap:8px;width:100%">
            <el-select v-model="cardForm.currency" style="width:140px" @change="onCardCurrencyChange">
              <el-option v-for="c in CURRENCIES" :key="c.code" :label="`${c.code} ${c.symbol} (${t(c.nameKey)})`" :value="c.code" />
            </el-select>
            <el-input-number v-model="cardForm.value_yuan" :min="0.01" :step="getAmountStep(cardForm.currency)" :precision="getCurrencyDecimals(cardForm.currency)" style="flex:1" :placeholder="getValuePlaceholder(cardForm.currency)" />
          </div>
          <div style="font-size:11px;color:#999;margin-top:4px">{{ t('currency.amountHint') }}</div>
        </el-form-item>
        <el-form-item v-if="cardForm.card_type==='plan'" :label="t('charge.bindPlan')">
          <el-select v-model="cardForm.plan_variant_id" :placeholder="t('charge.selectPlanVariant')" filterable style="width:100%">
            <el-option v-for="v in planTypeVariants" :key="v.id" :label="variantDisplayName(v)" :value="v.id" />
          </el-select>
          <div style="font-size:12px;color:#909399;margin-top:4px">{{ t('charge.bindPlanHint') }}</div>
        </el-form-item>
        <el-form-item v-if="cardForm.card_type==='traffic_pack'" :label="t('charge.bindTrafficPack')">
          <el-select v-model="cardForm.plan_variant_id" :placeholder="t('charge.selectTrafficPack')" filterable style="width:100%">
            <el-option v-for="v in trafficPackVariants" :key="v.id" :label="variantDisplayName(v)" :value="v.id" />
          </el-select>
          <div style="font-size:12px;color:#909399;margin-top:4px">{{ t('charge.bindTrafficPackHint') }}</div>
        </el-form-item>
        <el-form-item :label="t('charge.cardPrefix')">
          <el-input v-model="cardForm.card_prefix" placeholder="CF" style="width:100%" />
          <div style="font-size:12px;color:#909399;margin-top:4px">{{ t('charge.cardPrefixHint') }}</div>
        </el-form-item>
        <el-form-item :label="t('charge.cardFormat')">
          <el-select v-model="cardForm.card_code_format" style="width:100%">
            <el-option v-for="f in cardCodeFormats" :key="f.value" :label="formatPreview(f.value, cardForm.card_prefix)" :value="f.value" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('charge.expiresAt')"><el-date-picker v-model="cardForm.expires_at" type="datetime" value-format="YYYY-MM-DDTHH:mm:ssZ" style="width:100%" /></el-form-item>
        <el-form-item :label="t('charge.agent')">
          <el-select v-model="cardForm.agent_id" :placeholder="t('charge.selectAgent')" clearable style="width:100%">
            <el-option :label="t('charge.official')" :value="null" />
            <el-option v-for="u in agentUsers" :key="u.id" :label="u.username" :value="u.id" />
          </el-select>
          <div style="font-size:12px;color:#909399;margin-top:4px">{{ t('charge.agentHint') }}</div>
        </el-form-item>
        <el-form-item :label="t('charge.remark')"><el-input v-model="cardForm.remark" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="cardDialog=false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="saveCard">{{ t('common.create') }}</el-button>
      </template>
    </el-dialog>

    <!-- 批量生成弹窗 -->
    <el-dialog v-model="batchDialog" :title="t('charge.batchCreateTitle')" width="560px">
      <el-form :model="batchForm" label-width="100px" size="small">
        <el-form-item :label="t('common.type')">
          <el-select v-model="batchForm.card_type" style="width:100%">
            <el-option :label="t('charge.cardTypeBalance')" value="balance" />
            <el-option :label="t('charge.cardTypePlan')" value="plan" />
            <el-option :label="t('charge.cardTypeTrafficPack')" value="traffic_pack" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="batchForm.card_type==='balance'" :label="t('currency.amount')">
          <div style="display:flex;gap:8px;width:100%">
            <el-select v-model="batchForm.currency" style="width:140px" @change="onBatchCurrencyChange">
              <el-option v-for="c in CURRENCIES" :key="c.code" :label="`${c.code} ${c.symbol} (${t(c.nameKey)})`" :value="c.code" />
            </el-select>
            <el-input-number v-model="batchForm.value_yuan" :min="0.01" :step="getAmountStep(batchForm.currency)" :precision="getCurrencyDecimals(batchForm.currency)" style="flex:1" :placeholder="getValuePlaceholder(batchForm.currency)" />
          </div>
          <div style="font-size:11px;color:#999;margin-top:4px">{{ t('currency.amountHint') }}</div>
        </el-form-item>
        <el-form-item v-if="batchForm.card_type==='plan'" :label="t('charge.bindPlan')">
          <el-select v-model="batchForm.plan_variant_id" :placeholder="t('charge.selectPlanVariant')" filterable style="width:100%">
            <el-option v-for="v in planTypeVariants" :key="v.id" :label="variantDisplayName(v)" :value="v.id" />
          </el-select>
          <div style="font-size:12px;color:#909399;margin-top:4px">{{ t('charge.bindPlanHint') }}</div>
        </el-form-item>
        <el-form-item v-if="batchForm.card_type==='traffic_pack'" :label="t('charge.bindTrafficPack')">
          <el-select v-model="batchForm.plan_variant_id" :placeholder="t('charge.selectTrafficPack')" filterable style="width:100%">
            <el-option v-for="v in trafficPackVariants" :key="v.id" :label="variantDisplayName(v)" :value="v.id" />
          </el-select>
          <div style="font-size:12px;color:#909399;margin-top:4px">{{ t('charge.bindTrafficPackHint') }}</div>
        </el-form-item>
        <el-form-item :label="t('charge.quantity')"><el-input-number v-model="batchForm.count" :min="1" :max="1000" style="width:100%" /></el-form-item>
        <el-form-item :label="t('charge.cardPrefix')">
          <el-input v-model="batchForm.card_prefix" placeholder="CF" style="width:100%" />
          <div style="font-size:12px;color:#909399;margin-top:4px">{{ t('charge.cardPrefixHint') }}</div>
        </el-form-item>
        <el-form-item :label="t('charge.cardFormat')">
          <el-select v-model="batchForm.card_code_format" style="width:100%">
            <el-option v-for="f in cardCodeFormats" :key="f.value" :label="formatPreview(f.value, batchForm.card_prefix)" :value="f.value" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('charge.expiresAt')"><el-date-picker v-model="batchForm.expires_at" type="datetime" value-format="YYYY-MM-DDTHH:mm:ssZ" style="width:100%" /></el-form-item>
        <el-form-item :label="t('charge.agent')">
          <el-select v-model="batchForm.agent_id" :placeholder="t('charge.selectAgent')" clearable style="width:100%">
            <el-option :label="t('charge.official')" :value="null" />
            <el-option v-for="u in agentUsers" :key="u.id" :label="u.username" :value="u.id" />
          </el-select>
          <div style="font-size:12px;color:#909399;margin-top:4px">{{ t('charge.agentHint') }}</div>
        </el-form-item>
        <el-form-item :label="t('charge.remark')"><el-input v-model="batchForm.remark" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="batchDialog=false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="batchSaving" @click="saveBatch">{{ t('charge.generate') }}</el-button>
      </template>
    </el-dialog>

    <!-- 余额流水弹窗 -->
    <el-dialog v-model="txDialog" :title="t('charge.txTitle', { id: txClientId })" width="800px">
      <el-table :data="transactions" v-loading="txLoading" stripe size="small">
        <el-table-column prop="id" label="ID" width="60" />
        <el-table-column :label="t('common.type')" width="80">
          <template #default="{row}">
            <el-tag :type="row.type==='charge'?'success':row.type==='consume'?'danger':'info'" size="small">{{ txTypeLabel(row.type) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="t('currency.amount')" width="120">
          <template #default="{row}">
            <span :style="{color: row.amount>0?'#67c23a':'#f56c6c', fontWeight:600}">
              {{ row.amount>0?'+':'' }}{{ formatAmount(Math.abs(row.amount), row.currency || 'CNY') }}
            </span>
          </template>
        </el-table-column>
        <el-table-column :label="t('charge.balance')" width="100">
          <template #default="{row}">{{ formatAmount(row.balance_after, row.currency || 'CNY') }}</template>
        </el-table-column>
        <el-table-column prop="remark" :label="t('charge.remark')" show-overflow-tooltip />
        <el-table-column :label="t('common.time')" width="160">
          <template #default="{row}">{{ row.created_at?.substring(0,16).replace('T',' ') }}</template>
        </el-table-column>
      </el-table>
    </el-dialog>

    <!-- ═══════════════════════════════════════════════ -->
    <!-- 密钥展示弹窗（仅显示一次） -->
    <el-dialog v-model="secretDialog" :title="t('charge.secretTitle')" width="560px" :close-on-click-modal="false">
      <el-alert type="warning" :closable="false" style="margin-bottom:16px">
        <template #title>
          ⚠ {{ t('charge.secretOnceWarning') }}
        </template>
      </el-alert>
      <el-descriptions :column="1" border size="small">
        <el-descriptions-item :label="t('charge.site')">{{ secretDialogSite }}</el-descriptions-item>
        <el-descriptions-item :label="t('charge.secretVersion')">v{{ secretDialogVersion }}</el-descriptions-item>
        <el-descriptions-item :label="t('charge.callbackSecret')">
          <div style="display:flex;gap:8px;align-items:center">
            <code style="font-family:monospace;font-size:13px;word-break:break-all;flex:1;background:#f5f7fa;padding:8px;border-radius:4px">{{ secretDialogValue }}</code>
            <el-button size="small" type="primary" @click="copySecret(secretDialogValue)">{{ t('common.copy') }}</el-button>
          </div>
        </el-descriptions-item>
        <el-descriptions-item :label="t('charge.callbackUrl')">
          <div style="display:flex;gap:8px;align-items:center">
            <code style="font-family:monospace;font-size:12px;word-break:break-all;flex:1">{{ secretDialogCallbackUrl }}</code>
            <el-button size="small" @click="copyText(secretDialogCallbackUrl)">{{ t('common.copy') }}</el-button>
          </div>
        </el-descriptions-item>
      </el-descriptions>
      <div style="margin-top:12px;font-size:12px;color:#909399">
        t('charge.secretUsageHint')
      </div>
      <template #footer>
        <el-button type="primary" @click="secretDialog=false">{{ t('charge.copiedSaved') }}</el-button>
      </template>
    </el-dialog>

    <!-- ═══════════════════════════════════════════════ -->
    <!-- 充值卡使用记录弹窗 -->
    <el-dialog v-model="usageDialog" :title="t('charge.usageRecordTitle', { id: usageCardId })" width="920px">
      <el-alert v-if="usageItems.length === 0 && !usageLoading" type="info" :closable="false">
        <template #title>{{ t('charge.cardNotUsed') }}</template>
      </el-alert>
      <el-table :data="usageItems" v-loading="usageLoading" stripe size="small" v-else>
        <el-table-column prop="id" :label="t('charge.orderId')" width="70" />
        <el-table-column prop="trade_no" :label="t('charge.tradeNo')" width="200" show-overflow-tooltip />
        <el-table-column prop="client_email" :label="t('charge.userEmail')" min-width="160" show-overflow-tooltip />
        <el-table-column prop="client_nickname" :label="t('charge.nickname')" width="100" show-overflow-tooltip />
        <el-table-column :label="t('currency.amount')" width="100">
          <template #default="{row}">{{ formatAmount(row.amount, row.currency || 'CNY') }}</template>
        </el-table-column>
        <el-table-column :label="t('common.status')" width="80">
          <template #default="{row}">
            <el-tag :type="chargeOrderStatusColor(row.status)" size="small">{{ chargeOrderStatusLabel(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="t('charge.usedAt')" width="160">
          <template #default="{row}">{{ row.created_at?.substring(0,16).replace('T',' ') }}</template>
        </el-table-column>
      </el-table>
      <template #footer>
        <el-button @click="usageDialog=false">{{ t('common.close') }}</el-button>
      </template>
    </el-dialog>

    <!-- ═══════════════════════════════════════════════ -->
    <!-- 卡号展示弹窗（仅显示一次） -->
    <el-dialog v-model="cardCodeDialog" :title="t('charge.cardCodeTitle')" width="500px" :close-on-click-modal="false">
      <el-alert type="warning" :closable="false" style="margin-bottom:16px">
        <template #title>
          ⚠ {{ t('charge.cardCodeOnceWarning') }}
        </template>
      </el-alert>
      <el-descriptions :column="1" border size="small">
        <el-descriptions-item v-if="cardCodeDialogBatchId" :label="t('charge.batchId')">{{ cardCodeDialogBatchId }}</el-descriptions-item>
        <el-descriptions-item :label="t('charge.cardCode')">
          <div style="display:flex;gap:8px;align-items:center">
            <code style="font-family:monospace;font-size:14px;word-break:break-all;flex:1;background:#f5f7fa;padding:8px;border-radius:4px">{{ cardCodeDialogValue }}</code>
            <el-button size="small" type="primary" @click="copyCardCode(cardCodeDialogValue)">{{ t('common.copy') }}</el-button>
          </div>
        </el-descriptions-item>
      </el-descriptions>
      <template #footer>
        <el-button type="primary" @click="cardCodeDialog=false">{{ t('charge.copiedSaved') }}</el-button>
      </template>
    </el-dialog>

    <!-- 批量生成结果弹窗 -->
    <el-dialog v-model="batchResultDialog" :title="t('charge.batchResult')" width="700px">
      <el-alert type="success" :closable="false" style="margin-bottom:12px">
        <template #title>{{ t('charge.batchResultMsg', { count: batchResult.length, batchId: batchResultBatchId }) }}</template>
      </el-alert>
      <el-table :data="batchResult" stripe size="small" max-height="400">
        <el-table-column prop="card_code" :label="t('charge.cardCode')" min-width="220" />
        <el-table-column :label="t('currency.amount')" width="200" show-overflow-tooltip>
          <template #default="{row}">{{ batchResultType==='balance' ? formatAmount(row.value, batchResultCurrency) : findVariantName(row.value) }}</template>
        </el-table-column>
        <el-table-column :label="t('common.status')" width="70">
          <template #default><el-tag type="success" size="small">{{ t('charge.unused') }}</el-tag></template>
        </el-table-column>
      </el-table>
      <template #footer>
        <el-button type="primary" @click="batchResultDialog=false">{{ t('common.confirm') }}</el-button>
      </template>
    </el-dialog>

  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import * as chargeApi from '../api/charge'
import * as planApi from '../api/plan'
import { CURRENCIES, getCurrencyInfo, getCurrencySymbol, getCurrencyDecimals, yuanToMinor, minorToYuan, formatAmount, getAmountStep, getValuePlaceholder } from '../utils/currency'
import { useQueryState } from '../composables/useQueryState'
import PaginationBar from '../components/PaginationBar.vue'

const { t, locale } = useI18n()

// Default currency - fetched from system settings
const defaultCurrency = ref('CNY')
async function fetchDefaultCurrency() {
  try {
    const client = (await import('../api/client')).default
    const d = await client.get('/system/settings', { params: { prefix: 'currency' } })
    if (d && d.items) {
      for (const item of d.items) {
        if (item.key === 'currency.default') defaultCurrency.value = item.value || 'CNY'
      }
    }
  } catch {}
}

const activeTab = useQueryState('tab', 'sites')

// ──────── Charge Sites ────────
const sites = ref([])
const sitesLoading = ref(false)
const siteDialog = ref(false)
const editingSite = ref(null)
const siteForm = ref({})

function emptySiteForm() {
  return { name: '', site_url: '', api_key: '', callback_secret: '', callback_whitelist: '', currency: defaultCurrency.value, supported_methods_arr: [], icon_url: '', sort_order: 0, enabled: true }
}
// v3.3: callback_secret 留空，后端自动生成强随机密钥
async function fetchSites() {
  sitesLoading.value = true
  try { sites.value = (await chargeApi.listChargeSites()) || [] } catch {} finally { sitesLoading.value = false }
}
function showCreateSite() {
  editingSite.value = null
  siteForm.value = emptySiteForm()
  siteDialog.value = true
}
function showEditSite(row) {
  editingSite.value = row
  let methods = []
  try { methods = JSON.parse(row.supported_methods || '[]') } catch {}
  siteForm.value = { ...row, supported_methods_arr: methods, enabled: !!row.enabled }
  siteDialog.value = true
}
function copyCallbackUrl(siteId) {
  const url = window.location.origin + `/api/v1/charge/callback/${siteId}`
  navigator.clipboard.writeText(url)
  ElMessage.success(t('charge.copiedCallbackUrl'))
}

// ──────── Secret Rotation & Generation (v3.3) ────────
const secretDialog = ref(false)
const secretDialogValue = ref('')
const secretDialogSite = ref('')
const secretDialogVersion = ref(1)
const secretDialogCallbackUrl = ref('')

function showSecretDialog(siteName, siteId, version, plaintextSecret) {
  secretDialogSite.value = siteName
  secretDialogVersion.value = version
  secretDialogValue.value = plaintextSecret
  secretDialogCallbackUrl.value = window.location.origin + `/api/v1/charge/callback/${siteId}`
  secretDialog.value = true
}

function copySecret(val) {
  navigator.clipboard.writeText(val)
  ElMessage.success(t('charge.secretCopied'))
}
function copyText(val) {
  navigator.clipboard.writeText(val)
  ElMessage.success(t('charge.copied'))
}

// 前端生成随机密钥字符串（用于编辑表单预填）
function generateSecretField() {
  const bytes = new Uint8Array(24)
  crypto.getRandomValues(bytes)
  const hex = Array.from(bytes).map(b => b.toString(16).padStart(2, '0')).join('')
  siteForm.value.callback_secret = 'cs_' + hex
  ElMessage.success(t('charge.randomSecretGenerated'))
}

// 轮换密钥 — 后端自动生成，返回明文（仅此一次）
async function rotateSiteSecret(row) {
  await ElMessageBox.confirm(
    t('charge.confirmRotateSecret', { name: row.name }),
    t('charge.rotateSecretConfirm'),
    { type: 'warning', confirmButtonText: t('charge.confirmRotate'), cancelButtonText: t('common.cancel') }
  )
  try {
    const r = await chargeApi.rotateSiteSecret(row.id) // 不传 newSecret，后端自动生成
    if (r && r.plaintext_secret) {
      showSecretDialog(row.name, row.id, r.new_version, r.plaintext_secret)
      ElMessage.success(t('charge.secretRotated'))
    } else {
      ElMessage.success(t('charge.secretRotated'))
    }
    fetchSites()
  } catch (e) {
    if (e?.response?.data?.message) ElMessage.error(e.response.data.message)
  }
}

// ──────── Card Code Display (v3.3: 仅显示一次) ────────
const cardCodeDialog = ref(false)
const cardCodeDialogValue = ref('')
const cardCodeDialogBatchId = ref('')

function showCardCodeDialog(plaintextCode, batchId) {
  cardCodeDialogValue.value = plaintextCode
  cardCodeDialogBatchId.value = batchId
  cardCodeDialog.value = true
}

function copyCardCode(val) {
  navigator.clipboard.writeText(val)
  ElMessage.success(t('charge.cardCodeCopied'))
}

// ──────── Domain Verification ────────
const verifyDialog = ref(false)
const verifySite = ref(null)
const verifyData = ref(null)
const verifyLoading = ref(false)
const verifyStep = ref(0)

async function showVerifyDialog(row) {
  verifySite.value = row
  verifyData.value = null
  verifyStep.value = 0
  verifyDialog.value = true
  // 如果已有 verify_token，展示已有数据
  if (row.verify_token) {
    verifyData.value = {
      verify_token: row.verify_token,
      verify_content: row.verify_content,
      verify_file_url: `${row.site_url.replace(/\/$/, '')}/.well-known/${row.verify_token}.txt`,
    }
    verifyStep.value = 1
  }
}
async function doGenerateVerify() {
  verifyLoading.value = true
  try {
    const r = await chargeApi.generateVerifyFile(verifySite.value.id)
    verifyData.value = r
    verifyStep.value = 1
    ElMessage.success(t('charge.verifyFileGenerated'))
    fetchSites()
  } catch {} finally { verifyLoading.value = false }
}
async function doVerifyHTTP() {
  verifyLoading.value = true
  try {
    await chargeApi.verifyDomainHTTP(verifySite.value.id)
    ElMessage.success(t('charge.httpVerifySuccess'))
    verifyStep.value = 3
    verifySite.value.verified = true
    fetchSites()
  } catch (e) {
    if (e?.response?.data?.message) ElMessage.error(e.response.data.message)
  } finally { verifyLoading.value = false }
}
async function doVerifyDNS() {
  verifyLoading.value = true
  try {
    await chargeApi.verifyDomainDNS(verifySite.value.id)
    ElMessage.success(t('charge.dnsVerifySuccess'))
    verifyStep.value = 3
    verifySite.value.verified = true
    fetchSites()
  } catch (e) {
    if (e?.response?.data?.message) ElMessage.error(e.response.data.message)
  } finally { verifyLoading.value = false }
}

// ──────── Integration Doc ────────
const integrationDocDialog = ref(false)
const integrationDoc = ref(null)

async function showIntegrationDoc(row) {
  integrationDocDialog.value = true
  integrationDoc.value = null
  try {
    integrationDoc.value = await chargeApi.getSiteIntegrationDoc(row.id)
  } catch {}
}
async function saveSite() {
  try {
    const payload = { ...siteForm.value, supported_methods: JSON.stringify(siteForm.value.supported_methods_arr || []) }
    delete payload.supported_methods_arr
    delete payload.id; delete payload.created_at; delete payload.updated_at
    if (editingSite.value) {
      await chargeApi.updateChargeSite(editingSite.value.id, payload)
      ElMessage.success(t('charge.saveSuccess'))
    } else {
      // v3.3: 创建时后端自动生成密钥，返回明文（仅此一次）
      const r = await chargeApi.createChargeSite(payload)
      if (r && r.plaintext_secret) {
        showSecretDialog(r.site?.name || siteForm.value.name, r.site?.id || 0, 1, r.plaintext_secret)
        ElMessage.success(t('charge.createSuccessWithSecret'))
      } else {
        ElMessage.success(t('charge.saveSuccess'))
      }
    }
    siteDialog.value = false
    fetchSites()
  } catch {}
}
async function toggleSite(row) {
  try {
    await chargeApi.updateChargeSite(row.id, { ...row, enabled: !row.enabled })
    ElMessage.success(row.enabled ? t('common.disabled') : t('common.enabled'))
    fetchSites()
  } catch {}
}
async function deleteSite(row) {
  await ElMessageBox.confirm(t('charge.confirmDeleteSite', { name: row.name }), t('common.confirm'))
  try { await chargeApi.deleteChargeSite(row.id); ElMessage.success(t('common.deleteSuccess')); fetchSites() } catch {}
}
function parseMethods(str) {
  try { return JSON.parse(str || '[]') } catch { return [] }
}

// ──────── Charge Orders ────────
const orders = ref([])
const ordersLoading = ref(false)
const orderStatusFilter = useQueryState('ostatus', '', { omit: (v) => v === '' })
const orderPage = useQueryState('opage', 1, { type: 'number' })
const orderPageSize = useQueryState('opageSize', 20, { type: 'number' })
const orderTotal = ref(0)
const orderDetailDialog = ref(false)
const currentOrder = ref(null)

async function fetchOrders() {
  ordersLoading.value = true
  try {
    const params = { limit: orderPageSize.value, offset: (orderPage.value - 1) * orderPageSize.value }
    if (orderStatusFilter.value) params.status = orderStatusFilter.value
    const r = await chargeApi.listChargeOrders(params)
    orders.value = r.items || []
    orderTotal.value = r.total || 0
  } catch {} finally { ordersLoading.value = false }
}
async function showOrderDetail(row) {
  try {
    currentOrder.value = await chargeApi.getChargeOrder(row.id)
    orderDetailDialog.value = true
  } catch {}
}

// ──────── Charge Cards ────────
const cards = ref([])
const cardsLoading = ref(false)
const cardStatusFilter = ref('')
const cardTypeFilter = ref('')
const cardAgentFilter = useQueryState('cagent', '', { omit: (v) => v === '' || v === null })
const cardPage = useQueryState('cpage', 1, { type: 'number' })
const cardPageSize = useQueryState('cpageSize', 20, { type: 'number' })
const cardTotal = ref(0)
const agentUsers = ref([])
const cardDialog = ref(false)
const cardForm = ref({})
const batchDialog = ref(false)
const batchForm = ref({})
const batchSaving = ref(false)
const batchResultDialog = ref(false)
const batchResult = ref([])
const batchResultBatchId = ref('')
const batchResultType = ref('balance')
const batchResultCurrency = ref('CNY')

async function fetchCards() {
  cardsLoading.value = true
  try {
    const params = { limit: cardPageSize.value, offset: (cardPage.value - 1) * cardPageSize.value }
    if (cardStatusFilter.value) params.status = cardStatusFilter.value
    if (cardTypeFilter.value) params.card_type = cardTypeFilter.value
    if (cardAgentFilter.value !== null && cardAgentFilter.value !== undefined) params.agent_id = cardAgentFilter.value
    const r = await chargeApi.listChargeCards(params)
    cards.value = r.items || []
    cardTotal.value = r.total || 0
  } catch {} finally { cardsLoading.value = false }
}

// v3.3: 加载代理员列表（管理员用户）
async function fetchAgentUsers() {
  try {
    const r = await chargeApi.listUsers({ limit: 1000 })
    agentUsers.value = (r.items || []).filter(u => u.status === 1)
  } catch {
    agentUsers.value = []
  }
}
function showCreateCard() {
  cardForm.value = {
    card_type: 'balance', value_yuan: 10, currency: defaultCurrency.value, plan_variant_id: null,
    card_prefix: loadSavedPrefix(), card_code_format: loadSavedFormat(),
    expires_at: null, remark: ''
  }
  cardDialog.value = true
}
function onCardCurrencyChange(code) {
  // 当货币改变时调整精度
  const info = getCurrencyInfo(code)
  cardForm.value.value_yuan = cardForm.value.value_yuan ? parseFloat(cardForm.value.value_yuan.toFixed(info.decimals)) : 0
}
async function saveCard() {
  try {
    const payload = { ...cardForm.value }
    if (payload.card_type === 'balance') {
      payload.value = yuanToMinor(payload.value_yuan, payload.currency)
    }
    if (payload.card_type === 'plan' && payload.plan_variant_id) {
      payload.value = payload.plan_variant_id
    }
    // 组合前缀+格式
    payload.card_code_format = buildCardCodeFormat(payload.card_code_format, payload.card_prefix)
    delete payload.plan_variant_id
    delete payload.card_prefix
    delete payload.value_yuan
    const r = await chargeApi.createChargeCard(payload)
    // 记住选择
    saveFormatAndPrefix(cardForm.value.card_code_format, cardForm.value.card_prefix)
    // v3.3: 后端返回明文卡号（仅此一次），弹窗展示
    if (r && r.plaintext_card_code) {
      showCardCodeDialog(r.plaintext_card_code, r.card?.batch_id || '')
    } else {
      ElMessage.success(t('charge.createSuccess'))
    }
    cardDialog.value = false
    fetchCards()
  } catch {}
}
function showBatchCreate() {
  batchForm.value = {
    card_type: 'balance', value_yuan: 10, currency: defaultCurrency.value, plan_variant_id: null, count: 10,
    expires_at: null, remark: '',
    card_prefix: loadSavedPrefix(), card_code_format: loadSavedFormat()
  }
  batchDialog.value = true
}
function onBatchCurrencyChange(code) {
  const info = getCurrencyInfo(code)
  batchForm.value.value_yuan = batchForm.value.value_yuan ? parseFloat(batchForm.value.value_yuan.toFixed(info.decimals)) : 0
}
async function saveBatch() {
  // 校验
  if (batchForm.value.card_type === 'plan' && !batchForm.value.plan_variant_id) {
    ElMessage.warning(t('charge.planVariantRequired'))
    return
  }
  if (batchForm.value.card_type === 'balance' && (!batchForm.value.value_yuan || batchForm.value.value_yuan <= 0)) {
    ElMessage.warning(t('charge.balanceValueRequired'))
    return
  }
  batchSaving.value = true
  try {
    const payload = { ...batchForm.value }
    if (payload.card_type === 'balance') {
      payload.value = yuanToMinor(payload.value_yuan, payload.currency)
    }
    if (payload.card_type === 'plan' && payload.plan_variant_id) {
      payload.plan_variant_id = payload.plan_variant_id
    }
    // 组合前缀+格式
    payload.card_code_format = buildCardCodeFormat(payload.card_code_format, payload.card_prefix)
    delete payload.card_prefix
    delete payload.value_yuan
    const r = await chargeApi.batchCreateChargeCards(payload)
    // 记住选择
    saveFormatAndPrefix(batchForm.value.card_code_format, batchForm.value.card_prefix)
    ElMessage.success(t('charge.batchCreateSuccess'))
    batchDialog.value = false
    batchResult.value = r.cards || []
    batchResultBatchId.value = r.batch_id || ''
    batchResultType.value = batchForm.value.card_type
    batchResultCurrency.value = batchForm.value.currency || defaultCurrency.value
    batchResultDialog.value = true
    fetchCards()
  } catch {} finally { batchSaving.value = false }
}
async function deleteCard(row) {
  await ElMessageBox.confirm(t('charge.confirmDeleteCard', { code: row.card_code_masked || row.card_code }), t('common.confirm'))
  try { await chargeApi.deleteChargeCard(row.id); ElMessage.success(t('common.deleteSuccess')); fetchCards() } catch {}
}

// ──────── Card Usage Records (v3.3) ────────
const usageDialog = ref(false)
const usageCardId = ref(0)
const usageItems = ref([])
const usageLoading = ref(false)

async function showCardUsages(row) {
  usageCardId.value = row.id
  usageDialog.value = true
  usageLoading.value = true
  usageItems.value = []
  try {
    const r = await chargeApi.getCardUsages(row.id)
    usageItems.value = r.items || []
  } catch { usageItems.value = [] } finally { usageLoading.value = false }
}

// ──────── CSV Export (v3.3) ────────
async function doExportCSV() {
  try {
    const params = {}
    if (cardStatusFilter.value) params.status = cardStatusFilter.value
    if (cardTypeFilter.value) params.card_type = cardTypeFilter.value
    if (cardAgentFilter.value !== null && cardAgentFilter.value !== undefined) params.agent_id = cardAgentFilter.value
    const blob = await chargeApi.exportCardsCSV(params)
    const url = window.URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `charge-cards-${new Date().toISOString().substring(0,10)}.csv`
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    window.URL.revokeObjectURL(url)
    ElMessage.success(t('charge.csvExportSuccess'))
  } catch (e) {
    if (e?.response?.data?.message) ElMessage.error(e.response.data.message)
  }
}

// ── 2026-09-23：订单 / 余额流水导出（原来只有列表，无法整体导出对账 ✗）──
async function doExportOrdersCSV() {
  try {
    const params = {}
    if (orderStatusFilter.value) params.status = orderStatusFilter.value
    const blob = await chargeApi.exportOrdersCSV(params)
    downloadBlob(blob, `charge-orders-${new Date().toISOString().substring(0, 10)}.csv`)
    ElMessage.success(t('charge.csvExportSuccess'))
  } catch (e) {
    if (e?.response?.data?.message) ElMessage.error(e.response.data.message)
  }
}

async function doExportBalanceTxCSV() {
  try {
    const blob = await chargeApi.exportBalanceTransactionsCSV({})
    downloadBlob(blob, `balance-transactions-${new Date().toISOString().substring(0, 10)}.csv`)
    ElMessage.success(t('charge.csvExportSuccess'))
  } catch (e) {
    if (e?.response?.data?.message) ElMessage.error(e.response.data.message)
  }
}

// downloadBlob 统一走一处，避免每次都抄一遍 createObjectURL 样板 ✓
function downloadBlob(blob, filename) {
  const url = window.URL.createObjectURL(new Blob([blob]))
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  window.URL.revokeObjectURL(url)
}

// ──────── Client Balances ────────
const balances = ref([])
const balancesLoading = ref(false)
const balancePage = useQueryState('bpage', 1, { type: 'number' })
const balancePageSize = useQueryState('bpageSize', 20, { type: 'number' })
const balanceTotal = ref(0)
const txDialog = ref(false)
const txClientId = ref(0)
const transactions = ref([])
const txLoading = ref(false)

async function fetchBalances() {
  balancesLoading.value = true
  try {
    const params = { limit: balancePageSize.value, offset: (balancePage.value - 1) * balancePageSize.value }
    const r = await chargeApi.listClientBalances(params)
    balances.value = r.items || []
    balanceTotal.value = r.total || 0
  } catch {} finally { balancesLoading.value = false }
}
async function showTransactions(row) {
  txClientId.value = row.client_id
  txDialog.value = true
  txLoading.value = true
  try {
    const r = await chargeApi.listBalanceTransactions({ client_id: row.client_id, limit: 100 })
    transactions.value = r.items || []
  } catch { transactions.value = [] } finally { txLoading.value = false }
}

// ──────── Refund ────────
async function refundOrder(row) {
  await ElMessageBox.confirm(t('charge.confirmRefund', { id: row.id, amount: formatAmount(row.amount, row.currency || 'CNY') }), t('charge.refundConfirm'), { type: 'warning' })
  try {
    await chargeApi.refundChargeOrder(row.id, { remark: t('charge.adminRefund') })
    ElMessage.success(t('charge.refundSuccess'))
    fetchOrders()
    fetchBalances()
  } catch {}
}

// ──────── Helpers ────────
const sourceLabel = (s) => ({ site: t('charge.sourceSite'), card: t('charge.sourceCard'), balance: t('charge.sourceBalance'), admin: t('charge.sourceAdmin') })[s] || s
const chargeOrderStatusLabel = (s) => ({ pending: t('charge.statusPending'), paid: t('charge.statusPaid'), failed: t('charge.statusFailed'), expired: t('charge.statusExpired'), refunded: t('charge.statusRefunded') })[s] || s
const chargeOrderStatusColor = (s) => ({ pending:'warning', paid:'success', failed:'danger', expired:'info', refunded:'danger' })[s] || ''
const cardStatusLabel = (s) => ({ active: t('charge.cardActive'), used: t('charge.cardUsedFull'), expired: t('charge.cardExpired'), disabled: t('charge.cardDisabled') })[s] || s
const cardStatusColor = (s) => ({ active:'success', used:'info', expired:'warning', disabled:'danger' })[s] || ''
const cardTypeLabel = (tp) => ({ balance: t('charge.typeBalance'), plan: t('charge.typePlan'), traffic_pack: t('charge.typeTrafficPack') })[tp] || tp
const txTypeLabel = (tp) => ({ charge: t('charge.txCharge'), consume: t('charge.txConsume'), refund: t('charge.txRefund'), gift: t('charge.txGift') })[tp] || tp

// ──────── Plan Data (for card variant selection) ────────
const planTemplates = ref([])
const planVariants = ref([])       // all variants across all templates, with template info
const variantsLoading = ref(false)

async function fetchPlanData() {
  try {
    const tplRes = await planApi.listPlanTemplates()
    planTemplates.value = tplRes?.items || tplRes || []
    const allVariants = []
    // 获取模板下的规格（套餐型）
    for (const tpl of planTemplates.value) {
      try {
        const vr = await planApi.listPlanVariants(tpl.id)
        const vars = vr?.items || vr || []
        for (const v of vars) {
          allVariants.push({ ...v, template_name: tpl.name, template_id: tpl.id })
        }
      } catch {}
    }
    // 获取独立的流量叠加包
    try {
      const trafficPacks = (await planApi.listTrafficPacks()) || []
      for (const v of trafficPacks) {
        allVariants.push({ ...v, template_name: t('charge.trafficPackLabel'), template_id: 0 })
      }
    } catch {}
    planVariants.value = allVariants
  } catch {}
}

// computed: only plan-type variants (not traffic_pack)
const planTypeVariants = computed(() => planVariants.value.filter(v => v.variant_type === 'plan' && v.enabled))

// computed: only traffic_pack-type variants
const trafficPackVariants = computed(() => planVariants.value.filter(v => v.variant_type === 'traffic_pack' && v.enabled))

function variantDisplayName(v) {
  return `${v.template_name} - ${v.name} (${cycleLabel(v.billing_cycle)}, ${formatAmount(v.price, v.currency || 'CNY')})`
}

function findVariantName(id) {
  const v = planVariants.value.find(v => v.id === id)
  return v ? `${v.template_name} - ${v.name}` : `#${id}`
}

const cycleLabel = (c) => ({ monthly: t('charge.cycleMonthly'), quarterly: t('charge.cycleQuarterly'), semi_annual: t('charge.cycleSemiAnnual'), annual: t('charge.cycleAnnual'), lifetime: t('charge.cycleLifetime'), one_time: t('charge.cycleOneTime') })[c] || c

// ──────── Card Code Format Presets ────────
const cardCodeFormats = [
  { label: t('charge.cardFormat4Seg'), value: '{8}-{8}-{8}-{8}' },
  { label: t('charge.cardFormat2Seg'), value: '{8}-{8}' },
  { label: t('charge.cardFormat4SegShort'), value: '{4}-{4}-{4}-{4}' },
  { label: t('charge.cardFormatNoSep32'), value: '{32}' },
  { label: t('charge.cardFormatNoSep16'), value: '{16}' },
]

// 根据前缀+格式生成预览文本
function formatPreview(formatVal, prefix) {
  let preview = formatVal.replace(/\{(\d+)\}/g, (_, n) => 'X'.repeat(parseInt(n)))
  if (prefix) {
    preview = prefix + '-' + preview
  }
  return preview
}

// 将前缀和格式组合成后端需要的完整格式字符串
function buildCardCodeFormat(formatVal, prefix) {
  if (!prefix && !formatVal) return ''
  if (!prefix) return formatVal
  if (!formatVal) return prefix + '-{8}-{8}-{8}-{8}'
  return prefix + '-' + formatVal
}

// ──────── Card Code Defaults (localStorage) ────────
const STORAGE_KEY_FORMAT = 'cf_card_code_format'
const STORAGE_KEY_PREFIX = 'cf_card_prefix'

function loadSavedFormat() {
  return localStorage.getItem(STORAGE_KEY_FORMAT) || '{8}-{8}-{8}-{8}'
}
function loadSavedPrefix() {
  return localStorage.getItem(STORAGE_KEY_PREFIX) || 'CF'
}
function saveFormatAndPrefix(format, prefix) {
  localStorage.setItem(STORAGE_KEY_FORMAT, format)
  localStorage.setItem(STORAGE_KEY_PREFIX, prefix)
}

onMounted(async () => {
  await fetchDefaultCurrency()
  fetchAgentUsers() // v3.3: 加载代理员列表
  fetchSites()
  fetchOrders()
  fetchCards()
  fetchBalances()
  fetchPlanData()
})
</script>

<style scoped>
.el-divider { margin: 12px 0; }
</style>
