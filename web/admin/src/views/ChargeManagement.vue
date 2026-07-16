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
              <el-tag v-for="m in parseMethods(row.supported_methods)" :key="m" size="small" style="margin:2px">{{ m }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="状态" width="80">
            <template #default="{row}">
              <el-tag :type="row.enabled?'success':'danger'" size="small">{{ row.enabled?'启用':'禁用' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="sort_order" label="排序" width="60" />
          <el-table-column label="操作" width="180">
            <template #default="{row}">
              <el-button size="small" @click="showEditSite(row)">编辑</el-button>
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
          </div>
        </div>
        <div style="display:flex;gap:8px;margin-bottom:12px">
          <el-select v-model="cardStatusFilter" size="small" placeholder="状态" clearable style="width:100px">
            <el-option label="可用" value="active" />
            <el-option label="已用" value="used" />
            <el-option label="已过期" value="expired" />
            <el-option label="已禁用" value="disabled" />
          </el-select>
          <el-select v-model="cardTypeFilter" size="small" placeholder="类型" clearable style="width:100px">
            <el-option label="金额型" value="balance" />
            <el-option label="套餐型" value="plan" />
          </el-select>
          <el-button size="small" @click="fetchCards">查询</el-button>
        </div>
        <el-table :data="cards" v-loading="cardsLoading" stripe size="small">
          <el-table-column prop="id" label="ID" width="60" />
          <el-table-column prop="card_code" label="卡号" width="200" />
          <el-table-column label="类型" width="80">
            <template #default="{row}">
              <el-tag :type="row.card_type==='balance'?'':'warning'" size="small">{{ row.card_type==='balance'?'金额':'套餐' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="面值/套餐" width="200" show-overflow-tooltip>
            <template #default="{row}">
              <span v-if="row.card_type==='balance'">{{ formatAmount(row.value, row.currency || 'CNY') }}</span>
              <el-tag v-else type="warning" size="small">{{ findVariantName(row.value) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="使用" width="80">
            <template #default="{row}">{{ row.used_count }}/{{ row.max_uses||'∞' }}</template>
          </el-table-column>
          <el-table-column prop="batch_id" label="批次" width="140" show-overflow-tooltip />
          <el-table-column label="状态" width="80">
            <template #default="{row}">
              <el-tag :type="cardStatusColor(row.status)" size="small">{{ cardStatusLabel(row.status) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="过期时间" width="160">
            <template #default="{row}">{{ row.expires_at?.substring(0,10) || '永不过期' }}</template>
          </el-table-column>
          <el-table-column label="操作" width="80">
            <template #default="{row}">
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
          <el-table-column :label="t('currency.balance')" width="140">
            <template #default="{row}">{{ formatAmount(row.balance, row.currency || 'CNY') }}</template>
          </el-table-column>
          <el-table-column :label="t('currency.frozen')" width="140">
            <template #default="{row}">{{ formatAmount(row.frozen, row.currency || 'CNY') }}</template>
          </el-table-column>
          <el-table-column label="操作" width="200">
            <template #default="{row}">
              <el-button size="small" type="primary" @click="showAdjustBalance(row)">调整</el-button>
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
    <el-dialog v-model="siteDialog" :title="editingSite?'编辑充值站点':'新增充值站点'" width="550px">
      <el-form :model="siteForm" label-width="110px" size="small">
        <el-form-item label="名称"><el-input v-model="siteForm.name" /></el-form-item>
        <el-form-item label="站点URL"><el-input v-model="siteForm.site_url" placeholder="https://pay.example.com" /></el-form-item>
        <el-form-item label="API Key"><el-input v-model="siteForm.api_key" type="password" show-password /></el-form-item>
        <el-form-item label="回调密钥"><el-input v-model="siteForm.callback_secret" type="password" show-password /></el-form-item>
        <el-form-item label="支付方式">
          <el-select v-model="siteForm.supported_methods_arr" multiple style="width:100%" placeholder="选择支持的支付方式">
            <el-option label="支付宝" value="alipay" />
            <el-option label="微信支付" value="wechat" />
            <el-option label="USDT" value="usdt" />
            <el-option label="PayPal" value="paypal" />
            <el-option label="信用卡" value="credit_card" />
          </el-select>
        </el-form-item>
        <el-form-item label="图标URL"><el-input v-model="siteForm.icon_url" /></el-form-item>
        <el-form-item label="排序"><el-input-number v-model="siteForm.sort_order" :min="0" /></el-form-item>
        <el-form-item label="启用"><el-switch v-model="siteForm.enabled" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="siteDialog=false">取消</el-button>
        <el-button type="primary" @click="saveSite">保存</el-button>
      </template>
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
        <el-form-item label="卡号前缀">
          <el-input v-model="cardForm.card_prefix" placeholder="CF" style="width:100%" />
          <div style="font-size:12px;color:#909399;margin-top:4px">留空则无前缀，如 CF、VIP、GIFT 等</div>
        </el-form-item>
        <el-form-item label="卡号格式">
          <el-select v-model="cardForm.card_code_format" style="width:100%">
            <el-option v-for="f in cardCodeFormats" :key="f.value" :label="formatPreview(f.value, cardForm.card_prefix)" :value="f.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="可用次数"><el-input-number v-model="cardForm.max_uses" :min="1" style="width:100%" /></el-form-item>
        <el-form-item label="过期时间"><el-date-picker v-model="cardForm.expires_at" type="datetime" value-format="YYYY-MM-DDTHH:mm:ssZ" style="width:100%" /></el-form-item>
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
        <el-form-item label="数量"><el-input-number v-model="batchForm.count" :min="1" :max="1000" style="width:100%" /></el-form-item>
        <el-form-item label="每卡次数"><el-input-number v-model="batchForm.max_uses" :min="1" style="width:100%" /></el-form-item>
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
        <el-form-item label="备注"><el-input v-model="batchForm.remark" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="batchDialog=false">取消</el-button>
        <el-button type="primary" :loading="batchSaving" @click="saveBatch">生成</el-button>
      </template>
    </el-dialog>

    <!-- 余额调整弹窗 -->
    <el-dialog v-model="adjustDialog" :title="'调整余额 - 客户#'+adjustForm.client_id" width="450px">
      <el-form :model="adjustForm" label-width="80px" size="small">
        <el-form-item :label="t('currency.balance')">
          <span style="font-weight:600;color:#409eff">{{ formatAmount(adjustForm.current_balance || 0, adjustForm.currency || 'CNY') }}</span>
        </el-form-item>
        <el-form-item :label="t('currency.amount')">
          <div style="display:flex;gap:8px;width:100%">
            <el-select v-model="adjustForm.currency" style="width:140px" disabled>
              <el-option v-for="c in CURRENCIES" :key="c.code" :label="`${c.code} ${c.symbol}`" :value="c.code" />
            </el-select>
            <el-input-number v-model="adjustForm.amount_yuan" :step="getAmountStep(adjustForm.currency)" :precision="getCurrencyDecimals(adjustForm.currency)" style="flex:1" />
          </div>
          <div style="font-size:12px;color:#909399;margin-top:4px">正数=充值，负数=扣减</div>
        </el-form-item>
        <el-form-item label="备注"><el-input v-model="adjustForm.remark" placeholder="管理员调整" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="adjustDialog=false">取消</el-button>
        <el-button type="primary" :loading="adjustSaving" @click="saveAdjust">确认调整</el-button>
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
        <el-table-column label="次数" width="80">
          <template #default="{row}">{{ row.max_uses }}</template>
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
  return { name: '', site_url: '', api_key: '', callback_secret: '', supported_methods_arr: [], icon_url: '', sort_order: 0, enabled: true }
}
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
async function saveSite() {
  try {
    const payload = { ...siteForm.value, supported_methods: JSON.stringify(siteForm.value.supported_methods_arr || []) }
    delete payload.supported_methods_arr
    delete payload.id; delete payload.created_at; delete payload.updated_at
    if (editingSite.value) {
      await chargeApi.updateChargeSite(editingSite.value.id, payload)
    } else {
      await chargeApi.createChargeSite(payload)
    }
    ElMessage.success('保存成功')
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
const cardPage = ref(1)
const cardTotal = ref(0)
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
    const r = await chargeApi.listChargeCards(params)
    cards.value = r.items || []
    cardTotal.value = r.total || 0
  } catch {} finally { cardsLoading.value = false }
}
function showCreateCard() {
  cardForm.value = {
    card_type: 'balance', value_yuan: 10, currency: defaultCurrency.value, plan_variant_id: null,
    card_prefix: loadSavedPrefix(), card_code_format: loadSavedFormat(),
    max_uses: 1, expires_at: null, remark: ''
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
    await chargeApi.createChargeCard(payload)
    // 记住选择
    saveFormatAndPrefix(cardForm.value.card_code_format, cardForm.value.card_prefix)
    ElMessage.success('创建成功')
    cardDialog.value = false
    fetchCards()
  } catch {}
}
function showBatchCreate() {
  batchForm.value = {
    card_type: 'balance', value_yuan: 10, currency: defaultCurrency.value, plan_variant_id: null, count: 10,
    max_uses: 1, expires_at: null, remark: '',
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
  await ElMessageBox.confirm(`确定删除充值卡「${row.card_code}」？`, '确认')
  try { await chargeApi.deleteChargeCard(row.id); ElMessage.success('删除成功'); fetchCards() } catch {}
}

// ──────── Client Balances ────────
const balances = ref([])
const balancesLoading = ref(false)
const balancePage = ref(1)
const balanceTotal = ref(0)
const adjustDialog = ref(false)
const adjustForm = ref({ client_id: 0, amount: 0, remark: '', current_balance: 0 })
const adjustSaving = ref(false)
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
function showAdjustBalance(row) {
  adjustForm.value = { client_id: row.client_id, amount_yuan: 0, currency: row.currency || 'CNY', remark: '', current_balance: row.balance }
  adjustDialog.value = true
}
async function saveAdjust() {
  if (adjustForm.value.amount_yuan === 0) { ElMessage.warning('调整金额不能为0'); return }
  adjustSaving.value = true
  try {
    const amount = yuanToMinor(adjustForm.value.amount_yuan, adjustForm.value.currency)
    await chargeApi.adjustClientBalance({ client_id: adjustForm.value.client_id, amount, currency: adjustForm.value.currency, remark: adjustForm.value.remark })
    ElMessage.success('调整成功')
    adjustDialog.value = false
    fetchBalances()
  } catch {} finally { adjustSaving.value = false }
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
const txTypeLabel = (t) => ({ charge:'充值', consume:'消费', refund:'退款', gift:'赠送' })[t] || t

// ──────── Plan Data (for card variant selection) ────────
const planTemplates = ref([])
const planVariants = ref([])       // all variants across all templates, with template info
const variantsLoading = ref(false)

async function fetchPlanData() {
  try {
    planTemplates.value = (await planApi.listPlanTemplates()) || []
    const allVariants = []
    for (const tpl of planTemplates.value) {
      try {
        const vars = (await planApi.listPlanVariants(tpl.id)) || []
        for (const v of vars) {
          allVariants.push({ ...v, template_name: tpl.name, template_id: tpl.id })
        }
      } catch {}
    }
    planVariants.value = allVariants
  } catch {}
}

// computed: only plan-type variants (not traffic_pack)
const planTypeVariants = computed(() => planVariants.value.filter(v => v.variant_type === 'plan' && v.enabled))

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
