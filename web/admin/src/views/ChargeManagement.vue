<template>
  <div>
    <el-tabs v-model="activeTab" type="border-card">

      <!-- ════════════ 充值站点 Tab ════════════ -->
      <el-tab-pane label="充值站点" name="sites">
        <div style="display:flex;justify-content:space-between;margin-bottom:16px">
          <h3 style="margin:0">充值站点</h3>
          <el-button type="primary" size="small" @click="showCreateSite">+ 新增站点</el-button>
        </div>
        <el-table :data="sites" v-loading="sitesLoading" stripe size="small">
          <el-table-column prop="id" label="ID" width="60" />
          <el-table-column prop="name" label="名称" width="150" />
          <el-table-column prop="site_url" label="站点URL" show-overflow-tooltip />
          <el-table-column label="支付方式" width="200">
            <template #default="{row}">
              <template v-if="parseMethods(row.supported_methods).length">
                <el-tag v-for="m in parseMethods(row.supported_methods)" :key="m" size="small" style="margin:2px">{{ m }}</el-tag>
              </template>
              <span v-else style="color:#999">—</span>
            </template>
          </el-table-column>
          <el-table-column label="状态" width="80">
            <template #default="{row}">
              <el-tag :type="row.enabled?'success':'danger'" size="small">{{ row.enabled?'启用':'禁用' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="验证" width="80">
            <template #default="{row}">
              <el-tag :type="row.verified?'success':'danger'" size="small">{{ row.verified?'已验证':'未验证' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="currency" label="货币" width="70" />
          <el-table-column prop="sort_order" label="排序" width="60" />
          <el-table-column label="操作" width="350">
            <template #default="{row}">
              <el-button size="small" @click="showEditSite(row)">编辑</el-button>
              <el-button size="small" type="primary" @click="showVerifyDialog(row)">验证</el-button>
              <el-button size="small" type="warning" @click="rotateSiteSecret(row)">轮换密钥</el-button>
              <el-button size="small" type="info" @click="showIntegrationDoc(row)">文档</el-button>
              <el-button size="small" :type="row.enabled?'warning':'success'" @click="toggleSite(row)">{{ row.enabled?'禁用':'启用' }}</el-button>
              <el-button size="small" type="danger" @click="deleteSite(row)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>

      <!-- ════════════ 充值订单 Tab ════════════ -->
      <el-tab-pane label="充值订单" name="orders">
        <div style="display:flex;justify-content:space-between;margin-bottom:16px">
          <h3 style="margin:0">充值订单</h3>
          <div style="display:flex;gap:8px;align-items:center">
            <el-select v-model="orderStatusFilter" size="small" placeholder="状态筛选" clearable style="width:120px">
              <el-option label="待支付" value="pending" />
              <el-option label="已支付" value="paid" />
              <el-option label="已失败" value="failed" />
              <el-option label="已过期" value="expired" />
            </el-select>
            <el-button size="small" @click="fetchOrders">查询</el-button>
          </div>
        </div>
        <el-table :data="orders" v-loading="ordersLoading" stripe size="small">
          <el-table-column prop="id" label="ID" width="60" />
          <el-table-column prop="client_id" label="客户端ID" width="90" />
          <el-table-column prop="client_email" label="邮箱" min-width="180" show-overflow-tooltip />
          <el-table-column prop="trade_no" label="交易号" width="200" show-overflow-tooltip />
          <el-table-column label="来源" width="80">
            <template #default="{row}">
              <el-tag :type="row.source==='site'?'':'success'" size="small">{{ sourceLabel(row.source) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="类型" width="80">
            <template #default="{row}">
              <el-tag :type="row.order_type==='charge'?'':'warning'" size="small">{{ row.order_type==='charge'?'充值':'购套' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column :label="t('currency.amount')" width="120">
            <template #default="{row}">{{ formatAmount(row.amount, row.currency || 'CNY') }}</template>
          </el-table-column>
          <el-table-column label="支付方式" width="90">
            <template #default="{row}">{{ row.method || '—' }}</template>
          </el-table-column>
          <el-table-column label="状态" width="80">
            <template #default="{row}">
              <el-tag :type="chargeOrderStatusColor(row.status)" size="small">{{ chargeOrderStatusLabel(row.status) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="创建时间" width="160">
            <template #default="{row}">{{ row.created_at?.substring(0,16).replace('T',' ') }}</template>
          </el-table-column>
          <el-table-column label="支付时间" width="160">
            <template #default="{row}">{{ row.paid_at?.substring(0,16).replace('T',' ') || '—' }}</template>
          </el-table-column>
          <el-table-column label="操作" width="140">
            <template #default="{row}">
              <el-button size="small" @click="showOrderDetail(row)">详情</el-button>
              <el-button size="small" type="danger" @click="refundOrder(row)" v-if="row.status==='paid'">退款</el-button>
            </template>
          </el-table-column>
        </el-table>
        <div style="display:flex;justify-content:center;margin-top:16px">
          <el-pagination v-model:current-page="orderPage" :page-size="20" :total="orderTotal" layout="prev, pager, next" @current-change="fetchOrders" />
        </div>
      </el-tab-pane>

      <!-- ════════════ 充值卡 Tab ════════════ -->
      <el-tab-pane label="充值卡" name="cards">
        <div style="display:flex;justify-content:space-between;margin-bottom:16px">
          <h3 style="margin:0">充值卡</h3>
          <div style="display:flex;gap:8px">
            <el-button type="primary" size="small" @click="showCreateCard">+ 新增卡</el-button>
            <el-button type="success" size="small" @click="showBatchCreate">批量生成</el-button>
            <el-button type="info" size="small" @click="doExportCSV">📥 导出CSV</el-button>
          </div>
        </div>
        <div style="display:flex;gap:8px;margin-bottom:12px;flex-wrap:wrap">
          <el-select v-model="cardStatusFilter" size="small" placeholder="状态" clearable style="width:100px">
            <el-option label="可用" value="active" />
            <el-option label="已用" value="used" />
            <el-option label="已过期" value="expired" />
            <el-option label="已禁用" value="disabled" />
          </el-select>
          <el-select v-model="cardTypeFilter" size="small" placeholder="类型" clearable style="width:120px">
            <el-option label="金额型" value="balance" />
            <el-option label="套餐型" value="plan" />
            <el-option label="流量叠加包" value="traffic_pack" />
          </el-select>
          <el-select v-model="cardAgentFilter" size="small" placeholder="代理员" clearable style="width:140px">
            <el-option label="官方" :value="0" />
            <el-option v-for="u in agentUsers" :key="u.id" :label="u.username" :value="u.id" />
          </el-select>
          <el-button size="small" @click="fetchCards">查询</el-button>
        </div>
        <el-table :data="cards" v-loading="cardsLoading" stripe size="small">
          <el-table-column prop="id" label="ID" width="60" />
          <el-table-column label="卡号" width="200">
            <template #default="{row}">
              <span style="font-family:monospace">{{ row.card_code_masked || row.card_code }}</span>
            </template>
          </el-table-column>
          <el-table-column label="类型" width="100">
            <template #default="{row}">
              <el-tag :type="row.card_type==='balance'?'':row.card_type==='plan'?'warning':'success'" size="small">{{ cardTypeLabel(row.card_type) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="面值/套餐" width="200" show-overflow-tooltip>
            <template #default="{row}">
              <span v-if="row.card_type==='balance'">{{ formatAmount(row.value, row.currency || 'CNY') }}</span>
              <el-tag v-else type="warning" size="small">{{ findVariantName(row.value) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="使用" width="70">
            <template #default="{row}">
              <el-tag :type="row.status==='active'?'success':'info'" size="small">{{ row.status==='active'?'未用':'已用' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="batch_id" label="批次" width="140" show-overflow-tooltip />
          <el-table-column label="创建者" width="100">
            <template #default="{row}">
              <span v-if="row.created_by_admin_name">{{ row.created_by_admin_name }}</span>
              <span v-else style="color:#999">—</span>
            </template>
          </el-table-column>
          <el-table-column label="代理员" width="100">
            <template #default="{row}">
              <span v-if="row.agent_name">{{ row.agent_name }}</span>
              <span v-else style="color:#999">官方</span>
            </template>
          </el-table-column>
          <el-table-column label="状态" width="80">
            <template #default="{row}">
              <el-tag :type="cardStatusColor(row.status)" size="small">{{ cardStatusLabel(row.status) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="过期时间" width="160">
            <template #default="{row}">{{ row.expires_at?.substring(0,10) || '永不过期' }}</template>
          </el-table-column>
          <el-table-column label="操作" width="150">
            <template #default="{row}">
              <el-button size="small" @click="showCardUsages(row)">使用记录</el-button>
              <el-button size="small" type="danger" @click="deleteCard(row)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>
        <div style="display:flex;justify-content:center;margin-top:16px">
          <el-pagination v-model:current-page="cardPage" :page-size="20" :total="cardTotal" layout="prev, pager, next" @current-change="fetchCards" />
        </div>
      </el-tab-pane>

      <!-- ════════════ 客户余额 Tab ════════════ -->
      <el-tab-pane label="客户余额" name="balances">
        <div style="display:flex;justify-content:space-between;margin-bottom:16px">
          <h3 style="margin:0">客户余额</h3>
        </div>
        <el-table :data="balances" v-loading="balancesLoading" stripe size="small">
          <el-table-column prop="id" label="ID" width="60" />
          <el-table-column prop="client_id" label="客户端ID" width="100" />
          <el-table-column prop="client_email" label="邮箱" min-width="180" show-overflow-tooltip />
          <el-table-column prop="client_nickname" label="昵称" width="120" show-overflow-tooltip />
          <el-table-column :label="t('currency.balance')" width="140">
            <template #default="{row}">{{ formatAmount(row.balance, row.currency || 'CNY') }}</template>
          </el-table-column>
          <el-table-column :label="t('currency.frozen')" width="140">
            <template #default="{row}">{{ formatAmount(row.frozen, row.currency || 'CNY') }}</template>
          </el-table-column>
          <el-table-column label="操作" width="100">
            <template #default="{row}">
              <el-button size="small" @click="showTransactions(row)">流水</el-button>
            </template>
          </el-table-column>
        </el-table>
        <div style="display:flex;justify-content:center;margin-top:16px">
          <el-pagination v-model:current-page="balancePage" :page-size="20" :total="balanceTotal" layout="prev, pager, next" @current-change="fetchBalances" />
        </div>
      </el-tab-pane>

    </el-tabs>

    <!-- ═══════════════════════════════════════════════ -->
    <!-- 充值站点编辑弹窗 -->
    <el-dialog v-model="siteDialog" :title="editingSite?'编辑充值站点':'新增充值站点'" width="620px">
      <el-form :model="siteForm" label-width="110px" size="small">
        <el-form-item label="名称"><el-input v-model="siteForm.name" /></el-form-item>
        <el-form-item label="站点URL"><el-input v-model="siteForm.site_url" placeholder="https://pay.example.com" /></el-form-item>
        <el-form-item label="API Key"><el-input v-model="siteForm.api_key" type="password" show-password /></el-form-item>
        <el-form-item label="回调密钥">
          <div style="display:flex;gap:8px;width:100%">
            <el-input v-model="siteForm.callback_secret" type="password" show-password placeholder="留空将自动生成" style="flex:1" />
            <el-button @click="generateSecretField">🎲 自动生成</el-button>
          </div>
          <div style="font-size:11px;color:#999;margin-top:4px">
            <span v-if="editingSite">留空不修改现有密钥。点击「轮换密钥」可生成新密钥</span>
            <span v-else>留空将自动生成强随机密钥（推荐）</span>
          </div>
        </el-form-item>
        <el-form-item label="IP白名单">
          <el-input v-model="siteForm.callback_whitelist" type="textarea" :rows="2" placeholder='JSON格式，如 ["1.2.3.4","10.0.0.0/24"]' />
          <div style="font-size:11px;color:#999;margin-top:4px">留空则不校验IP。支持单IP和CIDR网段</div>
        </el-form-item>
        <el-form-item label="货币">
          <el-select v-model="siteForm.currency" style="width:100%">
            <el-option v-for="c in CURRENCIES" :key="c.code" :label="`${c.code} ${c.symbol} (${t(c.nameKey)})`" :value="c.code" />
          </el-select>
        </el-form-item>
        <el-form-item label="支付方式">
          <el-select v-model="siteForm.supported_methods_arr" multiple filterable allow-create default-first-option style="width:100%" placeholder="选择或输入自定义支付方式">
            <el-option label="支付宝" value="alipay" />
            <el-option label="微信支付" value="wechat" />
            <el-option label="USDT" value="usdt" />
            <el-option label="PayPal" value="paypal" />
            <el-option label="信用卡" value="credit_card" />
            <el-option label="借记卡" value="debit_card" />
            <el-option label="银行转账" value="bank_transfer" />
            <el-option label="QIWI" value="qiwi" />
            <el-option label="ЮMoney" value="yoomoney" />
            <el-option label="SberPay" value="sberpay" />
            <el-option label="Tinkoff" value="tinkoff" />
            <el-option label="Crypto" value="crypto" />
          </el-select>
          <div style="font-size:11px;color:#999;margin-top:4px">可直接输入自定义支付方式，回车添加</div>
        </el-form-item>
        <el-form-item label="图标URL"><el-input v-model="siteForm.icon_url" /></el-form-item>
        <el-form-item label="排序"><el-input-number v-model="siteForm.sort_order" :min="0" /></el-form-item>
        <el-form-item label="启用"><el-switch v-model="siteForm.enabled" /></el-form-item>

        <!-- 回调地址展示 -->
        <el-divider content-position="left">回调配置</el-divider>
        <el-form-item v-if="editingSite" label="回调地址">
          <el-input :model-value="`/api/v1/charge/callback/${editingSite.id}`" readonly>
            <template #append><el-button @click="copyCallbackUrl(editingSite.id)">复制</el-button></template>
          </el-input>
          <div style="font-size:11px;color:#999;margin-top:4px">第三方站点需向此地址发送 POST 回调</div>
        </el-form-item>
        <el-form-item v-if="editingSite" label="验证状态">
          <el-tag :type="editingSite.verified?'success':'danger'" size="small">{{ editingSite.verified?'已验证':'未验证' }}</el-tag>
          <span style="margin-left:8px;font-size:12px;color:#909399">未验证站点的回调将被拒绝</span>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="siteDialog=false">取消</el-button>
        <el-button type="primary" @click="saveSite">保存</el-button>
      </template>
    </el-dialog>

    <!-- ═══════════════════════════════════════════════ -->
    <!-- 域名验证弹窗 -->
    <el-dialog v-model="verifyDialog" title="域名验证" width="600px">
      <div v-if="verifySite">
        <el-alert :type="verifySite.verified?'success':'warning'" :closable="false" style="margin-bottom:16px">
          <template #title>
            {{ verifySite.verified ? '✓ 域名已验证' : '⚠ 域名未验证，回调将被拒绝' }}
          </template>
        </el-alert>

        <el-steps :active="verifyStep" align-center style="margin-bottom:20px">
          <el-step title="生成验证文件" />
          <el-step title="部署验证" />
          <el-step title="验证域名" />
        </el-steps>

        <div v-if="verifyData">
          <el-descriptions :column="1" border size="small">
            <el-descriptions-item label="验证文件URL">
              <span style="word-break:break-all;font-family:monospace">{{ verifyData.verify_file_url }}</span>
            </el-descriptions-item>
            <el-descriptions-item label="文件内容">
              <span style="font-family:monospace">{{ verifyData.verify_content }}</span>
            </el-descriptions-item>
            <el-descriptions-item label="DNS TXT 记录名">_corefusion-verify</el-descriptions-item>
            <el-descriptions-item label="DNS TXT 记录值">
              <span style="font-family:monospace">{{ verifyData.verify_content }}</span>
            </el-descriptions-item>
          </el-descriptions>

          <div style="margin-top:16px;text-align:center">
            <el-button type="primary" :loading="verifyLoading" @click="doVerifyHTTP">验证 HTTP 文件</el-button>
            <el-button type="success" :loading="verifyLoading" @click="doVerifyDNS">验证 DNS TXT</el-button>
          </div>

          <div style="margin-top:12px;font-size:12px;color:#909399;text-align:center">
            方法一：将验证文件放在站点 <code>.well-known/</code> 目录下<br>
            方法二：添加 DNS TXT 记录 <code>_corefusion-verify.您的域名</code>
          </div>
        </div>

        <div v-else style="text-align:center;padding:20px">
          <el-button type="primary" :loading="verifyLoading" @click="doGenerateVerify">生成验证文件</el-button>
        </div>
      </div>
    </el-dialog>

    <!-- ═══════════════════════════════════════════════ -->
    <!-- 集成文档弹窗 -->
    <el-dialog v-model="integrationDocDialog" title="对接文档" width="650px">
      <div v-if="integrationDoc">
        <el-descriptions :column="1" border size="small">
          <el-descriptions-item label="站点ID">{{ integrationDoc.site_id }}</el-descriptions-item>
          <el-descriptions-item label="站点名称">{{ integrationDoc.site_name }}</el-descriptions-item>
          <el-descriptions-item label="回调地址">
            <span style="font-family:monospace;color:#409eff">{{ integrationDoc.callback_url }}</span>
          </el-descriptions-item>
          <el-descriptions-item label="签名算法">{{ integrationDoc.signature_algorithm }}</el-descriptions-item>
          <el-descriptions-item label="签名字段">
            <el-tag v-for="f in integrationDoc.signature_fields" :key="f" size="small" style="margin:2px">{{ f }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="时间戳容差">{{ integrationDoc.timestamp_tolerance }} 秒</el-descriptions-item>
          <el-descriptions-item label="Nonce 要求">{{ integrationDoc.nonce_required ? '必须提供' : '可选' }}</el-descriptions-item>
          <el-descriptions-item label="金额单位">{{ integrationDoc.amount_unit }}</el-descriptions-item>
          <el-descriptions-item label="IP白名单">{{ integrationDoc.ip_whitelist || '未配置' }}</el-descriptions-item>
          <el-descriptions-item label="验证状态">
            <el-tag :type="integrationDoc.verified?'success':'danger'" size="small">{{ integrationDoc.verified?'已验证':'未验证' }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="回调密钥">{{ integrationDoc.callback_secret_set?'已配置':'未配置' }}</el-descriptions-item>
        </el-descriptions>

        <el-divider content-position="left">回调请求示例 (JSON)</el-divider>
        <pre style="background:#f5f7fa;padding:12px;border-radius:4px;font-size:12px;overflow-x:auto">{{ JSON.stringify(integrationDoc.example_callback, null, 2) }}</pre>

        <el-divider content-position="left">成功响应</el-divider>
        <pre style="background:#f0f9eb;padding:12px;border-radius:4px;font-size:12px">{{ integrationDoc.response_format }}</pre>

        <el-divider content-position="left">签名计算方法</el-divider>
        <div style="font-size:12px;color:#606266;line-height:1.8">
          <p>1. 将签名字段按 key 字典序排列，组成 JSON 字符串</p>
          <p>2. 使用 <code>callback_secret</code> 作为密钥，计算 <code>HMAC-SHA256</code></p>
          <p>3. 将结果转为十六进制字符串，放入 <code>sign</code> 字段</p>
        </div>
      </div>
    </el-dialog>

    <!-- 充值订单详情弹窗 -->
    <el-dialog v-model="orderDetailDialog" title="充值订单详情" width="600px">
      <el-descriptions :column="2" border size="small" v-if="currentOrder">
        <el-descriptions-item label="ID">{{ currentOrder.id }}</el-descriptions-item>
        <el-descriptions-item label="交易号">{{ currentOrder.trade_no }}</el-descriptions-item>
        <el-descriptions-item label="客户端ID">{{ currentOrder.client_id }}</el-descriptions-item>
        <el-descriptions-item label="来源">{{ sourceLabel(currentOrder.source) }}</el-descriptions-item>
        <el-descriptions-item label="类型">{{ currentOrder.order_type==='charge'?'充值':'购套' }}</el-descriptions-item>
        <el-descriptions-item :label="t('currency.amount')">{{ formatAmount(currentOrder.amount || 0, currentOrder.currency || 'CNY') }}</el-descriptions-item>
        <el-descriptions-item label="支付币种">{{ currentOrder.paid_currency || '—' }}</el-descriptions-item>
        <el-descriptions-item label="支付金额">{{ currentOrder.paid_amount ? formatAmount(currentOrder.paid_amount, currentOrder.paid_currency || 'CNY') : '—' }}</el-descriptions-item>
        <el-descriptions-item label="汇率">{{ currentOrder.exchange_rate || '—' }}</el-descriptions-item>
        <el-descriptions-item label="支付方式">{{ currentOrder.method || '—' }}</el-descriptions-item>
        <el-descriptions-item label="状态">
          <el-tag :type="chargeOrderStatusColor(currentOrder.status)" size="small">{{ chargeOrderStatusLabel(currentOrder.status) }}</el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="站点交易号">{{ currentOrder.site_trade_no || '—' }}</el-descriptions-item>
        <el-descriptions-item label="创建时间">{{ currentOrder.created_at?.replace('T',' ').substring(0,19) }}</el-descriptions-item>
        <el-descriptions-item label="支付时间">{{ currentOrder.paid_at?.replace('T',' ').substring(0,19) || '—' }}</el-descriptions-item>
        <el-descriptions-item label="过期时间">{{ currentOrder.expires_at?.replace('T',' ').substring(0,19) || '—' }}</el-descriptions-item>
        <el-descriptions-item label="备注">{{ currentOrder.remark || '—' }}</el-descriptions-item>
      </el-descriptions>
    </el-dialog>

    <!-- 新增充值卡弹窗 -->
    <el-dialog v-model="cardDialog" title="新增充值卡" width="560px">
      <el-form :model="cardForm" label-width="90px" size="small">
        <el-form-item label="类型">
          <el-select v-model="cardForm.card_type" style="width:100%">
            <el-option label="金额型" value="balance" />
            <el-option label="套餐型" value="plan" />
            <el-option label="流量叠加包" value="traffic_pack" />
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
        <el-form-item v-if="cardForm.card_type==='plan'" label="绑定套餐">
          <el-select v-model="cardForm.plan_variant_id" placeholder="选择套餐规格" filterable style="width:100%">
            <el-option v-for="v in planTypeVariants" :key="v.id" :label="variantDisplayName(v)" :value="v.id" />
          </el-select>
          <div style="font-size:12px;color:#909399;margin-top:4px">选择要绑定的套餐规格，兑换时自动分配该套餐</div>
        </el-form-item>
        <el-form-item v-if="cardForm.card_type==='traffic_pack'" label="绑定流量包">
          <el-select v-model="cardForm.plan_variant_id" placeholder="选择流量叠加包规格" filterable style="width:100%">
            <el-option v-for="v in trafficPackVariants" :key="v.id" :label="variantDisplayName(v)" :value="v.id" />
          </el-select>
          <div style="font-size:12px;color:#909399;margin-top:4px">选择要绑定的流量叠加包规格，兑换时自动增加流量</div>
        </el-form-item>
        <el-form-item label="卡号前缀">
          <el-input v-model="cardForm.card_prefix" placeholder="CF" style="width:100%" />
          <div style="font-size:12px;color:#909399;margin-top:4px">留空则无前缀，如 CF、VIP、GIFT 等</div>
        </el-form-item>
        <el-form-item label="卡号格式">
          <el-select v-model="cardForm.card_code_format" style="width:100%">
            <el-option v-for="f in cardCodeFormats" :key="f.value" :label="formatPreview(f.value, cardForm.card_prefix)" :value="f.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="过期时间"><el-date-picker v-model="cardForm.expires_at" type="datetime" value-format="YYYY-MM-DDTHH:mm:ssZ" style="width:100%" /></el-form-item>
        <el-form-item label="代理员">
          <el-select v-model="cardForm.agent_id" placeholder="选择代理员（默认官方）" clearable style="width:100%">
            <el-option label="官方" :value="null" />
            <el-option v-for="u in agentUsers" :key="u.id" :label="u.username" :value="u.id" />
          </el-select>
          <div style="font-size:12px;color:#909399;margin-top:4px">选择销售代理员，用于追踪业绩（默认官方）</div>
        </el-form-item>
        <el-form-item label="备注"><el-input v-model="cardForm.remark" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="cardDialog=false">取消</el-button>
        <el-button type="primary" @click="saveCard">创建</el-button>
      </template>
    </el-dialog>

    <!-- 批量生成弹窗 -->
    <el-dialog v-model="batchDialog" title="批量生成充值卡" width="560px">
      <el-form :model="batchForm" label-width="100px" size="small">
        <el-form-item label="类型">
          <el-select v-model="batchForm.card_type" style="width:100%">
            <el-option label="金额型" value="balance" />
            <el-option label="套餐型" value="plan" />
            <el-option label="流量叠加包" value="traffic_pack" />
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
        <el-form-item v-if="batchForm.card_type==='plan'" label="绑定套餐">
          <el-select v-model="batchForm.plan_variant_id" placeholder="选择套餐规格" filterable style="width:100%">
            <el-option v-for="v in planTypeVariants" :key="v.id" :label="variantDisplayName(v)" :value="v.id" />
          </el-select>
          <div style="font-size:12px;color:#909399;margin-top:4px">选择要绑定的套餐规格，兑换时自动分配该套餐</div>
        </el-form-item>
        <el-form-item v-if="batchForm.card_type==='traffic_pack'" label="绑定流量包">
          <el-select v-model="batchForm.plan_variant_id" placeholder="选择流量叠加包规格" filterable style="width:100%">
            <el-option v-for="v in trafficPackVariants" :key="v.id" :label="variantDisplayName(v)" :value="v.id" />
          </el-select>
          <div style="font-size:12px;color:#909399;margin-top:4px">选择要绑定的流量叠加包规格，兑换时自动增加流量</div>
        </el-form-item>
        <el-form-item label="数量"><el-input-number v-model="batchForm.count" :min="1" :max="1000" style="width:100%" /></el-form-item>
        <el-form-item label="卡号前缀">
          <el-input v-model="batchForm.card_prefix" placeholder="CF" style="width:100%" />
          <div style="font-size:12px;color:#909399;margin-top:4px">留空则无前缀，如 CF、VIP、GIFT 等</div>
        </el-form-item>
        <el-form-item label="卡号格式">
          <el-select v-model="batchForm.card_code_format" style="width:100%">
            <el-option v-for="f in cardCodeFormats" :key="f.value" :label="formatPreview(f.value, batchForm.card_prefix)" :value="f.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="过期时间"><el-date-picker v-model="batchForm.expires_at" type="datetime" value-format="YYYY-MM-DDTHH:mm:ssZ" style="width:100%" /></el-form-item>
        <el-form-item label="代理员">
          <el-select v-model="batchForm.agent_id" placeholder="选择代理员（默认官方）" clearable style="width:100%">
            <el-option label="官方" :value="null" />
            <el-option v-for="u in agentUsers" :key="u.id" :label="u.username" :value="u.id" />
          </el-select>
          <div style="font-size:12px;color:#909399;margin-top:4px">选择销售代理员，用于追踪业绩（默认官方）</div>
        </el-form-item>
        <el-form-item label="备注"><el-input v-model="batchForm.remark" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="batchDialog=false">取消</el-button>
        <el-button type="primary" :loading="batchSaving" @click="saveBatch">生成</el-button>
      </template>
    </el-dialog>

    <!-- 余额流水弹窗 -->
    <el-dialog v-model="txDialog" :title="'余额流水 - 客户#'+txClientId" width="800px">
      <el-table :data="transactions" v-loading="txLoading" stripe size="small">
        <el-table-column prop="id" label="ID" width="60" />
        <el-table-column label="类型" width="80">
          <template #default="{row}">
            <el-tag :type="row.type==='charge'?'success':row.type==='consume'?'danger':'info'" size="small">{{ txTypeLabel(row.type) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="金额" width="120">
          <template #default="{row}">
            <span :style="{color: row.amount>0?'#67c23a':'#f56c6c', fontWeight:600}">
              {{ row.amount>0?'+':'' }}{{ formatAmount(Math.abs(row.amount), row.currency || 'CNY') }}
            </span>
          </template>
        </el-table-column>
        <el-table-column label="余额" width="100">
          <template #default="{row}">{{ formatAmount(row.balance_after, row.currency || 'CNY') }}</template>
        </el-table-column>
        <el-table-column prop="remark" label="备注" show-overflow-tooltip />
        <el-table-column label="时间" width="160">
          <template #default="{row}">{{ row.created_at?.substring(0,16).replace('T',' ') }}</template>
        </el-table-column>
      </el-table>
    </el-dialog>

    <!-- ═══════════════════════════════════════════════ -->
    <!-- 密钥展示弹窗（仅显示一次） -->
    <el-dialog v-model="secretDialog" title="🔑 回调密钥（仅显示一次）" width="560px" :close-on-click-modal="false">
      <el-alert type="warning" :closable="false" style="margin-bottom:16px">
        <template #title>
          ⚠ 此密钥仅显示一次！请立即复制并配置到支付平台，关闭后无法再次查看。
        </template>
      </el-alert>
      <el-descriptions :column="1" border size="small">
        <el-descriptions-item label="站点">{{ secretDialogSite }}</el-descriptions-item>
        <el-descriptions-item label="密钥版本">v{{ secretDialogVersion }}</el-descriptions-item>
        <el-descriptions-item label="回调密钥">
          <div style="display:flex;gap:8px;align-items:center">
            <code style="font-family:monospace;font-size:13px;word-break:break-all;flex:1;background:#f5f7fa;padding:8px;border-radius:4px">{{ secretDialogValue }}</code>
            <el-button size="small" type="primary" @click="copySecret(secretDialogValue)">复制</el-button>
          </div>
        </el-descriptions-item>
        <el-descriptions-item label="回调地址">
          <div style="display:flex;gap:8px;align-items:center">
            <code style="font-family:monospace;font-size:12px;word-break:break-all;flex:1">{{ secretDialogCallbackUrl }}</code>
            <el-button size="small" @click="copyText(secretDialogCallbackUrl)">复制</el-button>
          </div>
        </el-descriptions-item>
      </el-descriptions>
      <div style="margin-top:12px;font-size:12px;color:#909399">
        此密钥用于 HMAC-SHA256 签名验证。支付平台回调请求需使用此密钥计算签名，CoreFusion 服务端会验证签名是否正确。
      </div>
      <template #footer>
        <el-button type="primary" @click="secretDialog=false">我已复制保存</el-button>
      </template>
    </el-dialog>

    <!-- ═══════════════════════════════════════════════ -->
    <!-- 充值卡使用记录弹窗 -->
    <el-dialog v-model="usageDialog" :title="'使用记录 - 卡#' + usageCardId" width="750px">
      <el-alert v-if="usageItems.length === 0 && !usageLoading" type="info" :closable="false">
        <template #title>此卡尚未被使用</template>
      </el-alert>
      <el-table :data="usageItems" v-loading="usageLoading" stripe size="small" v-else>
        <el-table-column prop="id" label="订单ID" width="70" />
        <el-table-column prop="trade_no" label="交易号" width="200" show-overflow-tooltip />
        <el-table-column prop="client_email" label="使用者邮箱" min-width="160" show-overflow-tooltip />
        <el-table-column prop="client_nickname" label="昵称" width="100" show-overflow-tooltip />
        <el-table-column label="金额" width="100">
          <template #default="{row}">{{ formatAmount(row.amount, row.currency || 'CNY') }}</template>
        </el-table-column>
        <el-table-column label="状态" width="80">
          <template #default="{row}">
            <el-tag :type="chargeOrderStatusColor(row.status)" size="small">{{ chargeOrderStatusLabel(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="使用时间" width="160">
          <template #default="{row}">{{ row.created_at?.substring(0,16).replace('T',' ') }}</template>
        </el-table-column>
      </el-table>
      <template #footer>
        <el-button @click="usageDialog=false">关闭</el-button>
      </template>
    </el-dialog>

    <!-- ═══════════════════════════════════════════════ -->
    <!-- 卡号展示弹窗（仅显示一次） -->
    <el-dialog v-model="cardCodeDialog" title="🎫 充值卡号（仅显示一次）" width="500px" :close-on-click-modal="false">
      <el-alert type="warning" :closable="false" style="margin-bottom:16px">
        <template #title>
          ⚠ 此卡号仅显示一次！请立即复制保存，关闭后列表中将显示脱敏卡号。
        </template>
      </el-alert>
      <el-descriptions :column="1" border size="small">
        <el-descriptions-item v-if="cardCodeDialogBatchId" label="批次号">{{ cardCodeDialogBatchId }}</el-descriptions-item>
        <el-descriptions-item label="卡号">
          <div style="display:flex;gap:8px;align-items:center">
            <code style="font-family:monospace;font-size:14px;word-break:break-all;flex:1;background:#f5f7fa;padding:8px;border-radius:4px">{{ cardCodeDialogValue }}</code>
            <el-button size="small" type="primary" @click="copyCardCode(cardCodeDialogValue)">复制</el-button>
          </div>
        </el-descriptions-item>
      </el-descriptions>
      <template #footer>
        <el-button type="primary" @click="cardCodeDialog=false">我已复制保存</el-button>
      </template>
    </el-dialog>

    <!-- 批量生成结果弹窗 -->
    <el-dialog v-model="batchResultDialog" title="批量生成结果" width="700px">
      <el-alert type="success" :closable="false" style="margin-bottom:12px">
        <template #title>成功生成 {{ batchResult.length }} 张充值卡，批次号: {{ batchResultBatchId }}</template>
      </el-alert>
      <el-table :data="batchResult" stripe size="small" max-height="400">
        <el-table-column prop="card_code" label="卡号" width="220" />
        <el-table-column :label="t('currency.amount')" width="200" show-overflow-tooltip>
          <template #default="{row}">{{ batchResultType==='balance' ? formatAmount(row.value, batchResultCurrency) : findVariantName(row.value) }}</template>
        </el-table-column>
        <el-table-column label="状态" width="70">
          <template #default><el-tag type="success" size="small">未用</el-tag></template>
        </el-table-column>
      </el-table>
      <template #footer>
        <el-button type="primary" @click="batchResultDialog=false">确定</el-button>
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

const activeTab = ref('sites')

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
  ElMessage.success('已复制回调地址')
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
  ElMessage.success('密钥已复制')
}
function copyText(val) {
  navigator.clipboard.writeText(val)
  ElMessage.success('已复制')
}

// 前端生成随机密钥字符串（用于编辑表单预填）
function generateSecretField() {
  const bytes = new Uint8Array(24)
  crypto.getRandomValues(bytes)
  const hex = Array.from(bytes).map(b => b.toString(16).padStart(2, '0')).join('')
  siteForm.value.callback_secret = 'cs_' + hex
  ElMessage.success('已生成随机密钥')
}

// 轮换密钥 — 后端自动生成，返回明文（仅此一次）
async function rotateSiteSecret(row) {
  await ElMessageBox.confirm(
    `确定轮换站点「${row.name}」的回调密钥？\n\n轮换后旧密钥将变为过渡态（过渡期内仍可验签），新密钥立即生效。\n请确保已准备好将新密钥配置到支付平台。`,
    '轮换密钥确认',
    { type: 'warning', confirmButtonText: '确认轮换', cancelButtonText: '取消' }
  )
  try {
    const r = await chargeApi.rotateSiteSecret(row.id) // 不传 newSecret，后端自动生成
    if (r && r.plaintext_secret) {
      showSecretDialog(row.name, row.id, r.new_version, r.plaintext_secret)
      ElMessage.success('密钥已轮换')
    } else {
      ElMessage.success('密钥已轮换')
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
  ElMessage.success('卡号已复制')
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
    ElMessage.success('验证文件已生成')
    fetchSites()
  } catch {} finally { verifyLoading.value = false }
}
async function doVerifyHTTP() {
  verifyLoading.value = true
  try {
    await chargeApi.verifyDomainHTTP(verifySite.value.id)
    ElMessage.success('HTTP 验证成功！')
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
    ElMessage.success('DNS 验证成功！')
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
      ElMessage.success('保存成功')
    } else {
      // v3.3: 创建时后端自动生成密钥，返回明文（仅此一次）
      const r = await chargeApi.createChargeSite(payload)
      if (r && r.plaintext_secret) {
        showSecretDialog(r.site?.name || siteForm.value.name, r.site?.id || 0, 1, r.plaintext_secret)
        ElMessage.success('创建成功，密钥已生成')
      } else {
        ElMessage.success('保存成功')
      }
    }
    siteDialog.value = false
    fetchSites()
  } catch {}
}
async function toggleSite(row) {
  try {
    await chargeApi.updateChargeSite(row.id, { ...row, enabled: !row.enabled })
    ElMessage.success(row.enabled ? '已禁用' : '已启用')
    fetchSites()
  } catch {}
}
async function deleteSite(row) {
  await ElMessageBox.confirm(`确定删除充值站点「${row.name}」？`, '确认')
  try { await chargeApi.deleteChargeSite(row.id); ElMessage.success('删除成功'); fetchSites() } catch {}
}
function parseMethods(str) {
  try { return JSON.parse(str || '[]') } catch { return [] }
}

// ──────── Charge Orders ────────
const orders = ref([])
const ordersLoading = ref(false)
const orderStatusFilter = ref('')
const orderPage = ref(1)
const orderTotal = ref(0)
const orderDetailDialog = ref(false)
const currentOrder = ref(null)

async function fetchOrders() {
  ordersLoading.value = true
  try {
    const params = { limit: 20, offset: (orderPage.value - 1) * 20 }
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
const cardAgentFilter = ref(null)
const cardPage = ref(1)
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
    const params = { limit: 20, offset: (cardPage.value - 1) * 20 }
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
      ElMessage.success('创建成功')
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
    ElMessage.warning('套餐型充值卡必须选择绑定的套餐规格')
    return
  }
  if (batchForm.value.card_type === 'balance' && (!batchForm.value.value_yuan || batchForm.value.value_yuan <= 0)) {
    ElMessage.warning('金额型充值卡必须填写面值')
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
    ElMessage.success('批量生成成功')
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
  await ElMessageBox.confirm(`确定删除充值卡「${row.card_code_masked || row.card_code}」？`, '确认')
  try { await chargeApi.deleteChargeCard(row.id); ElMessage.success('删除成功'); fetchCards() } catch {}
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
    ElMessage.success('CSV 导出成功')
  } catch (e) {
    if (e?.response?.data?.message) ElMessage.error(e.response.data.message)
  }
}

// ──────── Client Balances ────────
const balances = ref([])
const balancesLoading = ref(false)
const balancePage = ref(1)
const balanceTotal = ref(0)
const txDialog = ref(false)
const txClientId = ref(0)
const transactions = ref([])
const txLoading = ref(false)

async function fetchBalances() {
  balancesLoading.value = true
  try {
    const params = { limit: 20, offset: (balancePage.value - 1) * 20 }
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
  await ElMessageBox.confirm(`确定对订单 #${row.id}（${formatAmount(row.amount, row.currency || 'CNY')}）执行退款？金额将退回客户余额。`, '退款确认', { type: 'warning' })
  try {
    await chargeApi.refundChargeOrder(row.id, { remark: '管理员退款' })
    ElMessage.success('退款成功')
    fetchOrders()
    fetchBalances()
  } catch {}
}

// ──────── Helpers ────────
const sourceLabel = (s) => ({ site:'支付站', card:'充值卡', balance:'余额', admin:'管理员' })[s] || s
const chargeOrderStatusLabel = (s) => ({ pending:'待支付', paid:'已支付', failed:'已失败', expired:'已过期', refunded:'已退款' })[s] || s
const chargeOrderStatusColor = (s) => ({ pending:'warning', paid:'success', failed:'danger', expired:'info', refunded:'danger' })[s] || ''
const cardStatusLabel = (s) => ({ active:'可用', used:'已用完', expired:'已过期', disabled:'已禁用' })[s] || s
const cardStatusColor = (s) => ({ active:'success', used:'info', expired:'warning', disabled:'danger' })[s] || ''
const cardTypeLabel = (t) => ({ balance:'金额', plan:'套餐', traffic_pack:'流量包' })[t] || t
const txTypeLabel = (t) => ({ charge:'充值', consume:'消费', refund:'退款', gift:'赠送' })[t] || t

// ──────── Plan Data (for card variant selection) ────────
const planTemplates = ref([])
const planVariants = ref([])       // all variants across all templates, with template info
const variantsLoading = ref(false)

async function fetchPlanData() {
  try {
    planTemplates.value = (await planApi.listPlanTemplates()) || []
    const allVariants = []
    // 获取模板下的规格（套餐型）
    for (const tpl of planTemplates.value) {
      try {
        const vars = (await planApi.listPlanVariants(tpl.id)) || []
        for (const v of vars) {
          allVariants.push({ ...v, template_name: tpl.name, template_id: tpl.id })
        }
      } catch {}
    }
    // 获取独立的流量叠加包
    try {
      const trafficPacks = (await planApi.listTrafficPacks()) || []
      for (const v of trafficPacks) {
        allVariants.push({ ...v, template_name: '流量叠加包', template_id: 0 })
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
  return `${v.template_name} - ${v.name} (${cycleLabel(v.billing_cycle)}, ${formatAmount(v.price, 'CNY')})`
}

function findVariantName(id) {
  const v = planVariants.value.find(v => v.id === id)
  return v ? `${v.template_name} - ${v.name}` : `#${id}`
}

const cycleLabel = (c) => ({ monthly:'月付', quarterly:'季付', semi_annual:'半年付', annual:'年付', lifetime:'终身', one_time:'一次性' })[c] || c

// ──────── Card Code Format Presets ────────
const cardCodeFormats = [
  { label: 'XXXXXXXX-XXXXXXXX-XXXXXXXX-XXXXXXXX (4段)', value: '{8}-{8}-{8}-{8}' },
  { label: 'XXXXXXXX-XXXXXXXX (2段)', value: '{8}-{8}' },
  { label: 'XXXX-XXXX-XXXX-XXXX (4段短)', value: '{4}-{4}-{4}-{4}' },
  { label: 'XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX (无分隔)', value: '{32}' },
  { label: 'XXXXXXXXXXXXXXXX (16位无分隔)', value: '{16}' },
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
