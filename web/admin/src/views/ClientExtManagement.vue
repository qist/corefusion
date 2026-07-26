<template>
  <div>
    <el-tabs v-model="activeTab" type="border-card">

      <!-- ════════════ 邀请推广 Tab ════════════ -->
      <el-tab-pane label="邀请推广" name="referrals">
        <!-- 返利配置 -->
        <el-card style="margin-bottom:16px">
          <template #header><span style="font-weight:600">返利配置</span></template>
          <el-form label-width="140px" size="small" style="max-width:500px">
            <el-form-item label="返利比例 (%)">
              <el-input-number v-model="referralConfig.commission_rate" :min="0" :max="100" :step="1" />
              <span style="font-size:12px;color:#909399;margin-left:8px">被邀请人首次付费金额的百分比，0=关闭返利</span>
            </el-form-item>
            <el-form-item label="最低提现金额 (元)">
              <el-input-number v-model="referralConfig.min_withdraw_yuan" :min="0" :step="1" :precision="2" />
              <span style="font-size:12px;color:#909399;margin-left:8px">佣金余额达到此金额才能申请提现</span>
            </el-form-item>
            <el-form-item>
              <el-button type="primary" size="small" @click="saveReferralConfig" :loading="referralConfigSaving">保存配置</el-button>
            </el-form-item>
          </el-form>
        </el-card>

        <!-- 推广统计面板 -->
        <el-row :gutter="16" style="margin-bottom:16px">
          <el-col :span="4">
            <el-card shadow="hover" style="text-align:center">
              <div style="font-size:24px;font-weight:700;color:#409eff">{{ referralStats.total_referrers || 0 }}</div>
              <div style="font-size:12px;color:#909399;margin-top:4px">推广人数</div>
            </el-card>
          </el-col>
          <el-col :span="4">
            <el-card shadow="hover" style="text-align:center">
              <div style="font-size:24px;font-weight:700;color:#67c23a">{{ referralStats.total_records || 0 }}</div>
              <div style="font-size:12px;color:#909399;margin-top:4px">邀请记录</div>
            </el-card>
          </el-col>
          <el-col :span="4">
            <el-card shadow="hover" style="text-align:center">
              <div style="font-size:24px;font-weight:700;color:#e6a23c">¥{{ ((referralStats.total_commission || 0)/100).toFixed(2) }}</div>
              <div style="font-size:12px;color:#909399;margin-top:4px">总佣金</div>
            </el-card>
          </el-col>
          <el-col :span="4">
            <el-card shadow="hover" style="text-align:center">
              <div style="font-size:24px;font-weight:700;color:#f56c6c">¥{{ ((referralStats.total_withdrawn || 0)/100).toFixed(2) }}</div>
              <div style="font-size:12px;color:#909399;margin-top:4px">已提现</div>
            </el-card>
          </el-col>
          <el-col :span="4">
            <el-card shadow="hover" style="text-align:center">
              <div style="font-size:24px;font-weight:700;color:#909399">¥{{ ((referralStats.pending_commission || 0)/100).toFixed(2) }}</div>
              <div style="font-size:12px;color:#909399;margin-top:4px">待提现</div>
            </el-card>
          </el-col>
        </el-row>

        <!-- 推广提现管理 -->
        <h4 style="margin:0 0 12px">提现审核</h4>
        <div style="display:flex;justify-content:space-between;margin-bottom:16px">
          <h3 style="margin:0">推广提现管理</h3>
          <div style="display:flex;gap:8px;align-items:center">
            <el-select v-model="withdrawalStatusFilter" size="small" placeholder="状态筛选" clearable style="width:120px">
              <el-option label="待处理" value="pending" />
              <el-option label="已批准" value="approved" />
              <el-option label="已拒绝" value="rejected" />
            </el-select>
            <el-button size="small" @click="fetchWithdrawals">查询</el-button>
          </div>
        </div>
        <el-table :data="withdrawals" v-loading="withdrawalsLoading" stripe size="small">
          <el-table-column prop="id" label="ID" width="60" />
          <el-table-column prop="client_id" label="客户端ID" width="90" />
          <el-table-column label="金额" width="100">
            <template #default="{row}">{{ (row.amount/100).toFixed(2) }} 元</template>
          </el-table-column>
          <el-table-column label="提现方式" width="100">
            <template #default="{row}">{{ row.withdraw_type==='balance'?'到余额':'USDT' }}</template>
          </el-table-column>
          <el-table-column prop="wallet_address" label="钱包地址" show-overflow-tooltip />
          <el-table-column label="状态" width="80">
            <template #default="{row}">
              <el-tag :type="withdrawalStatusType(row.status)" size="small">{{ withdrawalStatusLabel(row.status) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="process_remark" label="处理备注" show-overflow-tooltip />
          <el-table-column prop="created_at" label="申请时间" width="170" />
          <el-table-column label="操作" width="180">
            <template #default="{row}">
              <template v-if="row.status==='pending'">
                <el-button size="small" type="success" @click="approveWithdrawal(row)">批准</el-button>
                <el-button size="small" type="danger" @click="rejectWithdrawal(row)">拒绝</el-button>
              </template>
            </template>
          </el-table-column>
        </el-table>
        <el-pagination v-if="withdrawalTotal>20" :total="withdrawalTotal" :page-size="20" v-model:current-page="withdrawalPage" layout="prev,pager,next" style="margin-top:12px" @current-change="fetchWithdrawals" />

        <!-- 邀请记录查询 -->
        <h4 style="margin:16px 0 12px">邀请记录</h4>
        <div style="display:flex;gap:8px;margin-bottom:12px;align-items:center">
          <el-input v-model="referralRecordReferrerFilter" size="small" placeholder="推广人ID" style="width:180px" clearable />
          <el-button size="small" @click="fetchReferralRecords">查询</el-button>
        </div>
        <el-table :data="referralRecords" v-loading="referralRecordsLoading" stripe size="small">
          <el-table-column prop="id" label="ID" width="60" />
          <el-table-column label="推广人" min-width="160">
            <template #default="{row}">{{ row.referrer_email || row.referrer_id }}</template>
          </el-table-column>
          <el-table-column label="被邀请人" min-width="160">
            <template #default="{row}">{{ row.referee_email || row.referee_id }}</template>
          </el-table-column>
          <el-table-column label="佣金" width="100">
            <template #default="{row}">¥{{ ((row.commission_amount||0)/100).toFixed(2) }}</template>
          </el-table-column>
          <el-table-column label="佣金状态" width="100">
            <template #default="{row}">
              <el-tag :type="row.commission_status==='available'?'success':'info'" size="small">{{ {available:'可提现',paid:'已提现',pending:'待确认'}[row.commission_status] || row.commission_status }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="created_at" label="创建时间" width="170" />
        </el-table>
      </el-tab-pane>

      <!-- ════════════ 福利管理 Tab ════════════ -->
      <el-tab-pane label="福利管理" name="gifts">
        <!-- 福利规则配置 -->
        <el-card style="margin-bottom:16px">
          <template #header><span style="font-weight:600">福利规则配置</span></template>
          <el-form :model="giftRuleForm" label-width="140px" size="small" style="max-width:500px">
            <el-form-item label="新用户注册奖励天数">
              <el-input-number v-model="giftRuleForm.signup_bonus_days" :min="0" />
              <span style="font-size:12px;color:#909399;margin-left:8px">0=无奖励</span>
            </el-form-item>
            <el-form-item label="月度自动福利天数">
              <el-input-number v-model="giftRuleForm.monthly_bonus_days" :min="0" />
              <span style="font-size:12px;color:#909399;margin-left:8px">0=关闭</span>
            </el-form-item>
            <el-form-item label="故障补偿倍率">
              <el-input-number v-model="giftRuleForm.fault_compensation_rate" :min="0" :max="10" :step="0.1" :precision="1" />
              <span style="font-size:12px;color:#909399;margin-left:8px">故障时长 × 倍率 = 补偿天数</span>
            </el-form-item>
            <el-form-item>
              <el-button type="primary" size="small" @click="saveGiftRuleConfig" :loading="giftRuleSaving">保存规则</el-button>
            </el-form-item>
          </el-form>
        </el-card>

        <div style="display:flex;justify-content:space-between;margin-bottom:16px">
          <h3 style="margin:0">福利记录</h3>
          <div style="display:flex;gap:8px">
            <el-button type="primary" size="small" @click="showCreateGift">+ 发放福利</el-button>
            <el-button size="small" @click="showBatchGift">批量发放</el-button>
          </div>
        </div>
        <el-table :data="gifts" v-loading="giftsLoading" stripe size="small">
          <el-table-column prop="id" label="ID" width="60" />
          <el-table-column label="客户端" min-width="180">
            <template #default="{row}">
              <span>{{ row.client_email || ('ID: ' + row.client_id) }}</span>
            </template>
          </el-table-column>
          <el-table-column label="类型" width="80">
            <template #default="{row}">
              <el-tag :type="row.type==='days'?'warning':'success'" size="small">{{ row.type==='days'?'天数':'流量' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="数值" width="100">
            <template #default="{row}">{{ row.type==='days'?row.value+'天':formatBytes(row.value) }}</template>
          </el-table-column>
          <el-table-column prop="source" label="来源" width="120" />
          <el-table-column label="状态" width="80">
            <template #default="{row}">
              <el-tag :type="giftStatusType(row.status)" size="small">{{ giftStatusLabel(row.status) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="created_at" label="发放时间" width="170" />
          <el-table-column label="领取时间" width="170">
            <template #default="{row}">{{ row.claimed_at || '-' }}</template>
          </el-table-column>
        </el-table>

        <el-dialog v-model="giftDialogVisible" :title="giftBatchMode?'批量发放福利':'发放福利'" width="500px">
          <el-form label-width="100px" size="small">
            <el-form-item label="选择客户" v-if="!giftBatchMode">
              <el-select
                v-model="giftForm.client_id"
                filterable
                remote
                reserve-keyword
                placeholder="输入邮箱搜索选择客户"
                :remote-method="searchClients"
                :loading="clientSearchLoading"
                style="width:100%"
              >
                <el-option
                  v-for="item in clientOptions"
                  :key="item.id"
                  :label="item.email"
                  :value="item.id"
                />
              </el-select>
            </el-form-item>
            <el-form-item label="选择客户" v-if="giftBatchMode">
              <el-select
                v-model="giftForm.client_ids_arr"
                filterable
                remote
                multiple
                reserve-keyword
                placeholder="输入邮箱搜索，可多选"
                :remote-method="searchClients"
                :loading="clientSearchLoading"
                style="width:100%"
              >
                <el-option
                  v-for="item in clientOptions"
                  :key="item.id"
                  :label="item.email"
                  :value="item.id"
                />
              </el-select>
            </el-form-item>
            <el-form-item label="类型">
              <el-select v-model="giftForm.type" style="width:100%">
                <el-option label="天数" value="days" />
                <el-option label="流量" value="traffic" />
              </el-select>
            </el-form-item>
            <el-form-item label="数值">
              <el-input-number v-model="giftForm.value" :min="0" :precision="giftForm.type==='traffic'?1:0" :step="giftForm.type==='traffic'?0.5:1" style="width:100%" />
              <span style="font-size:12px;color:#909399;margin-left:8px">{{ giftForm.type==='days'?'天':'GB' }}</span>
            </el-form-item>
            <el-form-item label="来源">
              <el-select v-model="giftForm.source" style="width:100%">
                <el-option label="故障补偿" value="fault_compensation" />
                <el-option label="月度福利" value="monthly_bonus" />
                <el-option label="活动" value="promotion" />
                <el-option label="管理员" value="admin" />
              </el-select>
            </el-form-item>
          </el-form>
          <template #footer>
            <el-button @click="giftDialogVisible=false">取消</el-button>
            <el-button type="primary" @click="submitGift" :loading="giftSubmitting">确认</el-button>
          </template>
        </el-dialog>
      </el-tab-pane>

      <!-- ════════════ 教程文章 Tab ════════════ -->
      <el-tab-pane label="教程文章" name="articles">
        <div style="display:flex;justify-content:space-between;margin-bottom:16px">
          <h3 style="margin:0">教程文章</h3>
          <el-button type="primary" size="small" @click="showCreateArticle">+ 新增文章</el-button>
        </div>
        <el-table :data="articles" v-loading="articlesLoading" stripe size="small">
          <el-table-column prop="id" label="ID" width="60" />
          <el-table-column prop="title" label="标题" show-overflow-tooltip />
          <el-table-column prop="slug" label="Slug" width="150" show-overflow-tooltip />
          <el-table-column label="分类" width="80">
            <template #default="{row}">
              <el-tag size="small">{{ articleCategoryLabel(row.category) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="平台" width="80">
            <template #default="{row}">{{ row.platform||'通用' }}</template>
          </el-table-column>
          <el-table-column label="状态" width="80">
            <template #default="{row}">
              <el-tag :type="row.is_published?'success':'info'" size="small">{{ row.is_published?'已发布':'草稿' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="弹窗" width="60">
            <template #default="{row}">
              <el-tag :type="row.is_popup?'danger':'info'" size="small">{{ row.is_popup?'是':'否' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="sort_order" label="排序" width="60" />
          <el-table-column label="操作" width="180">
            <template #default="{row}">
              <el-button size="small" @click="showEditArticle(row)">编辑</el-button>
              <el-button size="small" type="danger" @click="deleteArticle(row)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>

        <el-dialog v-model="articleDialogVisible" :title="articleForm.id?'编辑文章':'新增文章'" width="800px">
          <el-form label-width="80px" size="small">
            <el-form-item label="标题">
              <el-input v-model="articleForm.title" />
            </el-form-item>
            <el-form-item label="Slug">
              <el-input v-model="articleForm.slug" placeholder="url-friendly slug" />
            </el-form-item>
            <el-form-item label="分类">
              <el-select v-model="articleForm.category" style="width:100%">
                <el-option label="教程" value="tutorial" />
                <el-option label="FAQ" value="faq" />
                <el-option label="公告" value="notice" />
                <el-option label="通用" value="general" />
              </el-select>
            </el-form-item>
            <el-form-item label="平台">
              <el-select v-model="articleForm.platform" clearable style="width:100%">
                <el-option label="Windows" value="windows" />
                <el-option label="iOS" value="ios" />
                <el-option label="Android" value="android" />
                <el-option label="Mac" value="mac" />
                <el-option label="Linux" value="linux" />
                <el-option label="Other" value="other" />
                <el-option label="游戏" value="game" />
              </el-select>
            </el-form-item>
            <el-form-item label="排序">
              <el-input-number v-model="articleForm.sort_order" :min="0" />
            </el-form-item>
            <el-form-item label="发布">
              <el-switch v-model="articleForm.is_published" />
            </el-form-item>
            <el-form-item label="首页弹窗">
              <el-switch v-model="articleForm.is_popup" />
              <span style="margin-left:8px;color:#999;font-size:12px">开启后用户打开首页自动弹窗显示</span>
            </el-form-item>
            <el-form-item label="内容">
              <el-input v-model="articleForm.content" type="textarea" :rows="15" placeholder="Markdown 格式" />
            </el-form-item>
          </el-form>
          <template #footer>
            <el-button @click="articleDialogVisible=false">取消</el-button>
            <el-button type="primary" @click="submitArticle" :loading="articleSubmitting">确认</el-button>
          </template>
        </el-dialog>
      </el-tab-pane>

      <!-- ════════════ 共享账号 Tab ════════════ -->
      <el-tab-pane label="共享账号" name="shared_accounts">
        <div style="display:flex;justify-content:space-between;margin-bottom:16px">
          <h3 style="margin:0">共享账号</h3>
          <el-button type="primary" size="small" @click="showCreateSharedAccount">+ 新增账号</el-button>
        </div>
        <el-table :data="sharedAccounts" v-loading="sharedAccountsLoading" stripe size="small">
          <el-table-column prop="id" label="ID" width="60" />
          <el-table-column prop="name" label="名称" width="150" />
          <el-table-column label="类型" width="100">
            <template #default="{row}">
              <el-tag size="small">{{ sharedAccountTypeLabel(row.type) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="min_plan_category" label="最低套餐" width="100" />
          <el-table-column prop="repair_cooldown" label="报修冷却(分)" width="120" />
          <el-table-column label="状态" width="80">
            <template #default="{row}">
              <el-tag :type="row.enabled?'success':'danger'" size="small">{{ row.enabled?'启用':'禁用' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="sort_order" label="排序" width="60" />
          <el-table-column label="操作" width="200">
            <template #default="{row}">
              <el-button size="small" @click="showEditSharedAccount(row)">编辑</el-button>
              <el-button size="small" @click="viewRepairLogs(row)">报修日志</el-button>
              <el-button size="small" type="danger" @click="deleteSharedAccount(row)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>

        <el-dialog v-model="sharedAccountDialogVisible" :title="sharedAccountForm.id?'编辑账号':'新增账号'" width="500px">
          <el-form label-width="120px" size="small">
            <el-form-item label="名称">
              <el-input v-model="sharedAccountForm.name" />
            </el-form-item>
            <el-form-item label="类型">
              <el-select v-model="sharedAccountForm.type" style="width:100%">
                <el-option label="Apple ID" value="apple_id" />
                <el-option label="Netflix" value="netflix" />
                <el-option label="其他" value="other" />
              </el-select>
            </el-form-item>
            <el-form-item label="图标URL">
              <el-input v-model="sharedAccountForm.icon_url" />
            </el-form-item>
            <el-form-item label="凭据(加密)">
              <el-input v-model="sharedAccountForm.credentials_enc" type="textarea" :rows="3" />
            </el-form-item>
            <el-form-item label="最低套餐等级">
              <el-select v-model="sharedAccountForm.min_plan_category" clearable style="width:100%">
                <el-option label="基础" value="basic" />
                <el-option label="标准" value="standard" />
                <el-option label="高级" value="premium" />
              </el-select>
            </el-form-item>
            <el-form-item label="报修冷却(分钟)">
              <el-input-number v-model="sharedAccountForm.repair_cooldown" :min="0" />
            </el-form-item>
            <el-form-item label="排序">
              <el-input-number v-model="sharedAccountForm.sort_order" :min="0" />
            </el-form-item>
            <el-form-item label="启用">
              <el-switch v-model="sharedAccountForm.enabled" />
            </el-form-item>
          </el-form>
          <template #footer>
            <el-button @click="sharedAccountDialogVisible=false">取消</el-button>
            <el-button type="primary" @click="submitSharedAccount" :loading="sharedAccountSubmitting">确认</el-button>
          </template>
        </el-dialog>

        <el-dialog v-model="repairLogDialogVisible" title="报修日志" width="600px">
          <el-table :data="repairLogs" stripe size="small">
            <el-table-column prop="id" label="ID" width="60" />
            <el-table-column prop="client_id" label="客户端ID" width="90" />
            <el-table-column label="状态" width="80">
              <template #default="{row}">
                <el-tag :type="row.status==='completed'?'success':row.status==='failed'?'danger':'warning'" size="small">{{ row.status }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="created_at" label="时间" width="170" />
          </el-table>
        </el-dialog>
      </el-tab-pane>

      <!-- ════════════ IP白名单 Tab ════════════ -->
      <el-tab-pane label="IP白名单" name="ip_whitelist">
        <div style="display:flex;justify-content:space-between;margin-bottom:16px">
          <h3 style="margin:0">IP白名单管理</h3>
          <div style="display:flex;gap:8px;align-items:center">
            <el-input v-model="ipWhitelistClientFilter" size="small" placeholder="客户端ID筛选" style="width:150px" clearable />
            <el-button size="small" @click="fetchIPWhitelists">查询</el-button>
          </div>
        </div>
        <el-table :data="ipWhitelistEntries" v-loading="ipWhitelistLoading" stripe size="small">
          <el-table-column prop="id" label="ID" width="60" />
          <el-table-column prop="client_id" label="客户端ID" width="100" />
          <el-table-column prop="ip_address" label="IP地址" width="160" />
          <el-table-column label="来源" width="100">
            <template #default="{row}">
              <el-tag :type="row.source==='auto'?'success':'warning'" size="small">{{ row.source==='auto'?'自动添加':'手动添加' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="created_at" label="添加时间" width="170" />
        </el-table>
      </el-tab-pane>

      <!-- ════════════ OAuth绑定 Tab ════════════ -->
      <el-tab-pane label="OAuth绑定" name="oauth_bindings">
        <div style="display:flex;justify-content:space-between;margin-bottom:16px">
          <h3 style="margin:0">OAuth绑定记录</h3>
          <div style="display:flex;gap:8px;align-items:center">
            <el-input v-model="oauthClientFilter" size="small" placeholder="客户端ID筛选" style="width:150px" clearable />
            <el-button size="small" @click="fetchOAuthBindings">查询</el-button>
          </div>
        </div>
        <el-table :data="oauthBindings" v-loading="oauthBindingsLoading" stripe size="small">
          <el-table-column prop="id" label="ID" width="60" />
          <el-table-column prop="client_id" label="客户端ID" width="100" />
          <el-table-column prop="provider" label="平台" width="100">
            <template #default="{row}">
              <el-tag size="small">{{ row.provider }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="provider_user_id" label="第三方用户ID" width="200" show-overflow-tooltip />
          <el-table-column prop="created_at" label="绑定时间" width="170" />
        </el-table>
      </el-tab-pane>

    </el-tabs>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  listReferralWithdrawals, approveReferralWithdrawal, rejectReferralWithdrawal,
  getReferralStats, listReferralRecords, getReferralConfig, setReferralConfig,
  listClientGifts, createClientGift, batchCreateClientGifts, searchGiftClients,
  listArticles, getArticle, createArticle, updateArticle, deleteArticle as deleteArticleApi,
  listSharedAccounts, getSharedAccount, createSharedAccount, updateSharedAccount, deleteSharedAccount as deleteSharedAccountApi, listSharedAccountRepairLogs,
  listAllIPWhitelists, listOAuthBindings
} from '../api/clientext'

const activeTab = ref('referrals')

// ════════════ Referral Stats & Records ════════════
const referralStats = ref({})
const referralRecords = ref([])
const referralRecordsLoading = ref(false)
const referralRecordReferrerFilter = ref('')
const referralConfig = ref({ commission_rate: 15, min_withdraw_yuan: 10 })
const referralConfigSaving = ref(false)

async function fetchReferralConfig() {
  try {
    const result = await getReferralConfig()
    referralConfig.value.commission_rate = parseInt(result?.commission_rate) || 0
    referralConfig.value.min_withdraw_yuan = (parseInt(result?.min_withdraw_amount) || 0) / 100
  } catch {}
}
async function saveReferralConfig() {
  referralConfigSaving.value = true
  try {
    await setReferralConfig({
      commission_rate: referralConfig.value.commission_rate,
      min_withdraw_amount: Math.round(referralConfig.value.min_withdraw_yuan * 100)
    })
    ElMessage.success('返利配置已保存')
  } catch {} finally { referralConfigSaving.value = false }
}

async function fetchReferralStats() {
  try { referralStats.value = (await getReferralStats()) || {} } catch {}
}
async function fetchReferralRecords() {
  referralRecordsLoading.value = true
  try {
    const params = { limit: 50 }
    if (referralRecordReferrerFilter.value) params.referrer_id = referralRecordReferrerFilter.value
    const result = await listReferralRecords(params)
    referralRecords.value = result?.items || []
  } finally { referralRecordsLoading.value = false }
}

// ════════════ Referral Withdrawals ════════════
const withdrawals = ref([])
const withdrawalsLoading = ref(false)
const withdrawalTotal = ref(0)
const withdrawalPage = ref(1)
const withdrawalStatusFilter = ref('')

async function fetchWithdrawals() {
  withdrawalsLoading.value = true
  try {
    const params = { limit: 20, offset: (withdrawalPage.value - 1) * 20 }
    if (withdrawalStatusFilter.value) params.status = withdrawalStatusFilter.value
    const result = await listReferralWithdrawals(params)
    withdrawals.value = result?.items || []
    withdrawalTotal.value = result?.total || 0
  } finally { withdrawalsLoading.value = false }
}

async function approveWithdrawal(row) {
  const { value: remark } = await ElMessageBox.prompt('处理备注', '批准提现', { inputPlaceholder: '可选' })
  await approveReferralWithdrawal(row.id, { remark: remark || '' })
  ElMessage.success('已批准')
  fetchWithdrawals()
}

async function rejectWithdrawal(row) {
  const { value: remark } = await ElMessageBox.prompt('拒绝原因', '拒绝提现', { inputPlaceholder: '请填写原因' })
  await rejectReferralWithdrawal(row.id, { remark: remark || '' })
  ElMessage.success('已拒绝')
  fetchWithdrawals()
}

function withdrawalStatusLabel(s) { return { pending: '待处理', approved: '已批准', rejected: '已拒绝' }[s] || s }
function withdrawalStatusType(s) { return { pending: 'warning', approved: 'success', rejected: 'danger' }[s] || '' }

// ════════════ Gifts ════════════
const gifts = ref([])
const giftsLoading = ref(false)
const giftDialogVisible = ref(false)
const giftBatchMode = ref(false)
const giftForm = ref({ client_id: '', client_ids_arr: [], type: 'days', value: 0, source: 'admin' })
const giftSubmitting = ref(false)
const clientOptions = ref([])
const clientSearchLoading = ref(false)

async function fetchGifts() {
  giftsLoading.value = true
  try {
    const result = await listClientGifts({ limit: 50 })
    gifts.value = result?.items || []
  } finally { giftsLoading.value = false }
}

let searchTimer = null
async function searchClients(query) {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(async () => {
    clientSearchLoading.value = true
    try {
      const result = await searchGiftClients(query || '')
      clientOptions.value = result?.items || []
    } catch {
      // keep existing options on error
    } finally {
      clientSearchLoading.value = false
    }
  }, 300)
}

async function loadClientOptions() {
  clientSearchLoading.value = true
  try {
    const result = await searchGiftClients('')
    clientOptions.value = result?.items || []
  } catch {
    clientOptions.value = []
  } finally {
    clientSearchLoading.value = false
  }
}

function showCreateGift() { giftBatchMode.value = false; giftForm.value = { client_id: '', client_ids_arr: [], type: 'days', value: 0, source: 'admin' }; loadClientOptions(); giftDialogVisible.value = true }
function showBatchGift() { giftBatchMode.value = true; giftForm.value = { client_id: '', client_ids_arr: [], type: 'days', value: 0, source: 'admin' }; loadClientOptions(); giftDialogVisible.value = true }

async function submitGift() {
  giftSubmitting.value = true
  try {
    // 流量类型：GB → 字节；天数类型：直接用整数
    const rawValue = Number(giftForm.value.value)
    if (isNaN(rawValue) || rawValue <= 0) {
      ElMessage.warning('请输入有效的数值')
      giftSubmitting.value = false
      return
    }
    const finalValue = giftForm.value.type === 'traffic' ? Math.round(rawValue * 1073741824) : Math.round(rawValue)
    if (giftBatchMode.value) {
      const ids = giftForm.value.client_ids_arr
      if (!ids || ids.length === 0) {
        ElMessage.warning('请至少选择一个客户')
        giftSubmitting.value = false
        return
      }
      await batchCreateClientGifts({ client_ids: ids, type: giftForm.value.type, source: giftForm.value.source, value: finalValue })
    } else {
      if (!giftForm.value.client_id) {
        ElMessage.warning('请选择客户')
        giftSubmitting.value = false
        return
      }
      await createClientGift({ client_id: parseInt(giftForm.value.client_id), type: giftForm.value.type, value: finalValue, source: giftForm.value.source })
    }
    ElMessage.success('福利已发放')
    giftDialogVisible.value = false
    fetchGifts()
  } catch {
    // error handled by axios interceptor
  } finally { giftSubmitting.value = false }
}

function giftStatusLabel(s) { return { available: '可领取', claimed: '已领取', expired: '已过期' }[s] || s }
function giftStatusType(s) { return { available: 'success', claimed: 'info', expired: 'danger' }[s] || '' }
function formatBytes(b) { if (b === 0) return '0B'; const k = 1024; const s = ['B','KB','MB','GB','TB']; const i = Math.floor(Math.log(b)/Math.log(k)); return (b/Math.pow(k,i)).toFixed(1)+s[i] }

// ════════════ Gift Rule Config ════════════
const giftRuleForm = ref({ signup_bonus_days: 0, monthly_bonus_days: 0, fault_compensation_rate: 1.0 })
const giftRuleSaving = ref(false)

async function fetchGiftRuleConfig() {
  try {
    await listClientGifts({ limit: 1 })
    // Read config from system settings
    // For now use default values; real implementation would call /system/settings
  } catch {}
}

async function saveGiftRuleConfig() {
  giftRuleSaving.value = true
  try {
    // Save via system settings API
    const client = (await import('../api/client')).default
    await client.put('/system/settings/batch', {
      items: [
        { key: 'gift.signup_bonus_days', value: String(giftRuleForm.value.signup_bonus_days) },
        { key: 'gift.monthly_bonus_days', value: String(giftRuleForm.value.monthly_bonus_days) },
        { key: 'gift.fault_compensation_rate', value: String(giftRuleForm.value.fault_compensation_rate) },
      ]
    })
    ElMessage.success('福利规则已保存')
  } catch {} finally { giftRuleSaving.value = false }
}

// ════════════ Articles ════════════
const articles = ref([])
const articlesLoading = ref(false)
const articleDialogVisible = ref(false)
const articleForm = ref({ title: '', slug: '', category: 'tutorial', platform: '', sort_order: 0, is_published: false, is_popup: false, content: '' })
const articleSubmitting = ref(false)

async function fetchArticles() {
  articlesLoading.value = true
  try {
    const result = await listArticles({ limit: 50 })
    articles.value = result?.items || []
  } finally { articlesLoading.value = false }
}

function showCreateArticle() { articleForm.value = { title: '', slug: '', category: 'tutorial', platform: '', sort_order: 0, is_published: false, is_popup: false, content: '' }; articleDialogVisible.value = true }

async function showEditArticle(row) {
  const result = await getArticle(row.id)
  articleForm.value = { ...result }
  articleDialogVisible.value = true
}

async function submitArticle() {
  articleSubmitting.value = true
  try {
    if (articleForm.value.id) {
      await updateArticle(articleForm.value.id, articleForm.value)
    } else {
      await createArticle(articleForm.value)
    }
    ElMessage.success('保存成功')
    articleDialogVisible.value = false
    fetchArticles()
  } finally { articleSubmitting.value = false }
}

async function deleteArticle(row) {
  await ElMessageBox.confirm('确定删除此文章？', '提示')
  await deleteArticleApi(row.id)
  ElMessage.success('已删除')
  fetchArticles()
}

function articleCategoryLabel(c) { return { tutorial: '教程', faq: 'FAQ', notice: '公告', general: '通用' }[c] || c }

// ════════════ Shared Accounts ════════════
const sharedAccounts = ref([])
const sharedAccountsLoading = ref(false)
const sharedAccountDialogVisible = ref(false)
const sharedAccountForm = ref({ name: '', type: 'other', icon_url: '', credentials_enc: '', min_plan_category: '', repair_cooldown: 1440, sort_order: 0, enabled: true })
const sharedAccountSubmitting = ref(false)
const repairLogDialogVisible = ref(false)
const repairLogs = ref([])

async function fetchSharedAccounts() {
  sharedAccountsLoading.value = true
  try {
    const result = await listSharedAccounts({ limit: 50 })
    sharedAccounts.value = result?.items || []
  } finally { sharedAccountsLoading.value = false }
}

function showCreateSharedAccount() { sharedAccountForm.value = { name: '', type: 'other', icon_url: '', credentials_enc: '', min_plan_category: '', repair_cooldown: 1440, sort_order: 0, enabled: true }; sharedAccountDialogVisible.value = true }

async function showEditSharedAccount(row) {
  const result = await getSharedAccount(row.id)
  sharedAccountForm.value = { ...result }
  sharedAccountDialogVisible.value = true
}

async function submitSharedAccount() {
  sharedAccountSubmitting.value = true
  try {
    if (sharedAccountForm.value.id) {
      await updateSharedAccount(sharedAccountForm.value.id, sharedAccountForm.value)
    } else {
      await createSharedAccount(sharedAccountForm.value)
    }
    ElMessage.success('保存成功')
    sharedAccountDialogVisible.value = false
    fetchSharedAccounts()
  } finally { sharedAccountSubmitting.value = false }
}

async function deleteSharedAccount(row) {
  await ElMessageBox.confirm('确定删除此共享账号？', '提示')
  await deleteSharedAccountApi(row.id)
  ElMessage.success('已删除')
  fetchSharedAccounts()
}

async function viewRepairLogs(row) {
  const result = await listSharedAccountRepairLogs(row.id, { limit: 50 })
  repairLogs.value = result?.items || []
  repairLogDialogVisible.value = true
}

function sharedAccountTypeLabel(t) { return { apple_id: 'Apple ID', netflix: 'Netflix', other: '其他' }[t] || t }

// ════════════ IP Whitelist ════════════
const ipWhitelistEntries = ref([])
const ipWhitelistLoading = ref(false)
const ipWhitelistClientFilter = ref('')

async function fetchIPWhitelists() {
  ipWhitelistLoading.value = true
  try {
    const params = { limit: 50 }
    if (ipWhitelistClientFilter.value) params.client_id = ipWhitelistClientFilter.value
    const result = await listAllIPWhitelists(params)
    ipWhitelistEntries.value = result?.items || []
  } finally { ipWhitelistLoading.value = false }
}

// ════════════ OAuth Bindings ════════════
const oauthBindings = ref([])
const oauthBindingsLoading = ref(false)
const oauthClientFilter = ref('')

async function fetchOAuthBindings() {
  oauthBindingsLoading.value = true
  try {
    const params = { limit: 50 }
    if (oauthClientFilter.value) params.client_id = oauthClientFilter.value
    const result = await listOAuthBindings(params)
    oauthBindings.value = result?.items || []
  } finally { oauthBindingsLoading.value = false }
}

onMounted(() => { fetchReferralConfig(); fetchReferralStats(); fetchReferralRecords(); fetchWithdrawals(); fetchGifts(); fetchArticles(); fetchSharedAccounts(); fetchIPWhitelists(); fetchOAuthBindings() })
</script>
