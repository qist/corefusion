<template>
  <div>
    <el-tabs v-model="activeTab" type="border-card">

      <!-- ════════════ 邀请推广 Tab ════════════ -->
      <el-tab-pane :label="t('clientExt.referralsTab')" name="referrals">
        <!-- 返利配置 -->
        <el-card style="margin-bottom:16px">
          <template #header><span style="font-weight:600">{{ t('clientExt.rebateConfig') }}</span></template>
          <el-form label-width="140px" size="small" style="max-width:500px">
            <el-form-item :label="t('clientExt.rebateRate')">
              <el-input-number v-model="referralConfig.commission_rate" :min="0" :max="100" :step="1" />
              <span style="font-size:12px;color:#909399;margin-left:8px">{{ t('clientExt.rebateRateHint') }}</span>
            </el-form-item>
            <el-form-item :label="t('clientExt.minWithdraw')">
              <el-input-number v-model="referralConfig.min_withdraw_yuan" :min="0" :step="1" :precision="2" />
              <span style="font-size:12px;color:#909399;margin-left:8px">{{ t('clientExt.minWithdrawHint') }}</span>
            </el-form-item>
            <el-form-item>
              <el-button type="primary" size="small" @click="saveReferralConfig" :loading="referralConfigSaving">{{ t('clientExt.saveConfig') }}</el-button>
            </el-form-item>
          </el-form>
        </el-card>

        <!-- 推广统计面板 -->
        <el-row :gutter="16" style="margin-bottom:16px">
          <el-col :span="4">
            <el-card shadow="hover" style="text-align:center">
              <div style="font-size:24px;font-weight:700;color:#409eff">{{ referralStats.total_referrers || 0 }}</div>
              <div style="font-size:12px;color:#909399;margin-top:4px">{{ t('clientExt.totalReferrers') }}</div>
            </el-card>
          </el-col>
          <el-col :span="4">
            <el-card shadow="hover" style="text-align:center">
              <div style="font-size:24px;font-weight:700;color:#67c23a">{{ referralStats.total_records || 0 }}</div>
              <div style="font-size:12px;color:#909399;margin-top:4px">{{ t('clientExt.referralRecords') }}</div>
            </el-card>
          </el-col>
          <el-col :span="4">
            <el-card shadow="hover" style="text-align:center">
              <div style="font-size:24px;font-weight:700;color:#e6a23c">¥{{ ((referralStats.total_commission || 0)/100).toFixed(2) }}</div>
              <div style="font-size:12px;color:#909399;margin-top:4px">{{ t('clientExt.totalCommission') }}</div>
            </el-card>
          </el-col>
          <el-col :span="4">
            <el-card shadow="hover" style="text-align:center">
              <div style="font-size:24px;font-weight:700;color:#f56c6c">¥{{ ((referralStats.total_withdrawn || 0)/100).toFixed(2) }}</div>
              <div style="font-size:12px;color:#909399;margin-top:4px">{{ t('clientExt.withdrawn') }}</div>
            </el-card>
          </el-col>
          <el-col :span="4">
            <el-card shadow="hover" style="text-align:center">
              <div style="font-size:24px;font-weight:700;color:#909399">¥{{ ((referralStats.pending_commission || 0)/100).toFixed(2) }}</div>
              <div style="font-size:12px;color:#909399;margin-top:4px">{{ t('clientExt.pendingWithdraw') }}</div>
            </el-card>
          </el-col>
        </el-row>

        <!-- 推广提现管理 -->
        <h4 style="margin:0 0 12px">{{ t('clientExt.withdrawalReview') }}</h4>
        <div style="display:flex;justify-content:space-between;margin-bottom:16px">
          <h3 style="margin:0">{{ t('clientExt.referralWithdrawalMgmt') }}</h3>
          <div style="display:flex;gap:8px;align-items:center">
            <el-select v-model="withdrawalStatusFilter" size="small" :placeholder="t('clientExt.statusFilter')" clearable style="width:120px">
              <el-option :label="t('clientExt.withdrawPending')" value="pending" />
              <el-option :label="t('clientExt.withdrawApproved')" value="approved" />
              <el-option :label="t('clientExt.withdrawRejected')" value="rejected" />
            </el-select>
            <el-button size="small" @click="fetchWithdrawals">{{ t('common.query') }}</el-button>
          </div>
        </div>
        <el-table v-if="!isMobile" :data="withdrawals" v-loading="withdrawalsLoading" stripe size="small">
          <el-table-column prop="id" label="ID" width="60" />
          <el-table-column prop="client_id" :label="t('clientExt.clientId')" width="90" />
          <el-table-column :label="t('clientExt.amount')" width="100">
            <template #default="{row}">{{ (row.amount/100).toFixed(2) }} {{ t('clientExt.yuan') }}</template>
          </el-table-column>
          <el-table-column :label="t('clientExt.withdrawType')" width="100">
            <template #default="{row}">{{ row.withdraw_type==='balance'?t('clientExt.toBalance'):t('clientExt.usdt') }}</template>
          </el-table-column>
          <el-table-column prop="wallet_address" :label="t('clientExt.walletAddress')" show-overflow-tooltip />
          <el-table-column :label="t('clientExt.status')" width="80">
            <template #default="{row}">
              <el-tag :type="withdrawalStatusType(row.status)" size="small">{{ withdrawalStatusLabel(row.status) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="process_remark" :label="t('clientExt.processRemark')" show-overflow-tooltip />
          <el-table-column prop="created_at" :label="t('clientExt.applicationTime')" width="170" />
          <el-table-column :label="t('common.action')" width="180">
            <template #default="{row}">
              <template v-if="row.status==='pending'">
                <el-button size="small" type="success" @click="approveWithdrawal(row)">{{ t('clientExt.approve') }}</el-button>
                <el-button size="small" type="danger" @click="rejectWithdrawal(row)">{{ t('clientExt.reject') }}</el-button>
              </template>
            </template>
          </el-table-column>
        </el-table>
        <MobileCards v-else :rows="withdrawals" title-field="client_id" :fields="withdrawalCardFields">
          <template #actions="{ row }">
            <template v-if="row.status==='pending'">
              <el-button size="small" type="success" @click="approveWithdrawal(row)">{{ t('clientExt.approve') }}</el-button>
              <el-button size="small" type="danger" @click="rejectWithdrawal(row)">{{ t('clientExt.reject') }}</el-button>
            </template>
          </template>
        </MobileCards>
        <PaginationBar v-model:page="withdrawalPage" v-model:page-size="withdrawalPageSize" :total="withdrawalTotal" @change="fetchWithdrawals" />

        <!-- 邀请记录查询 -->
        <h4 style="margin:16px 0 12px">{{ t('clientExt.referralRecords') }}</h4>
        <div style="display:flex;gap:8px;margin-bottom:12px;align-items:center">
          <el-input v-model="referralRecordReferrerFilter" size="small" :placeholder="t('clientExt.referrerIdFilter')" style="width:180px" clearable />
          <el-button size="small" @click="fetchReferralRecords">{{ t('common.query') }}</el-button>
        </div>
        <el-table v-if="!isMobile" :data="referralRecords" v-loading="referralRecordsLoading" stripe size="small">
          <el-table-column prop="id" label="ID" width="60" />
          <el-table-column :label="t('clientExt.referrer')" min-width="160">
            <template #default="{row}">{{ row.referrer_email || row.referrer_id }}</template>
          </el-table-column>
          <el-table-column :label="t('clientExt.invitee')" min-width="160">
            <template #default="{row}">{{ row.referee_email || row.referee_id }}</template>
          </el-table-column>
          <el-table-column :label="t('clientExt.commission')" width="100">
            <template #default="{row}">¥{{ ((row.commission_amount||0)/100).toFixed(2) }}</template>
          </el-table-column>
          <el-table-column :label="t('clientExt.commissionStatus')" width="100">
            <template #default="{row}">
              <el-tag :type="row.commission_status==='available'?'success':'info'" size="small">{{ {available:t('clientExt.commissionAvailable'),paid:t('clientExt.commissionPaid'),pending:t('clientExt.commissionPending')}[row.commission_status] || row.commission_status }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="created_at" :label="t('clientExt.createdAt')" width="170" />
        </el-table>
        <MobileCards v-else :rows="referralRecords" :title-fn="refRecordTitleFn" :fields="refRecordCardFields" />
        <PaginationBar v-model:page="refPage" v-model:page-size="refSize" :total="refTotal" @change="fetchReferralRecords" />
      </el-tab-pane>

      <!-- ════════════ 福利管理 Tab ════════════ -->
      <el-tab-pane :label="t('clientExt.giftManagementTab')" name="gifts">
        <!-- 福利规则配置 -->
        <el-card style="margin-bottom:16px">
          <template #header><span style="font-weight:600">{{ t('clientExt.giftRuleConfig') }}</span></template>
          <el-form :model="giftRuleForm" label-width="140px" size="small" style="max-width:500px">
            <el-form-item :label="t('clientExt.signupBonusDays')">
              <el-input-number v-model="giftRuleForm.signup_bonus_days" :min="0" />
              <span style="font-size:12px;color:#909399;margin-left:8px">{{ t('clientExt.zeroNoBonus') }}</span>
            </el-form-item>
            <el-form-item :label="t('clientExt.monthlyBonusDays')">
              <el-input-number v-model="giftRuleForm.monthly_bonus_days" :min="0" />
              <span style="font-size:12px;color:#909399;margin-left:8px">{{ t('clientExt.zeroDisabled') }}</span>
            </el-form-item>
            <el-form-item :label="t('clientExt.faultCompensationRate')">
              <el-input-number v-model="giftRuleForm.fault_compensation_rate" :min="0" :max="10" :step="0.1" :precision="1" />
              <span style="font-size:12px;color:#909399;margin-left:8px">{{ t('clientExt.faultCompensationHint') }}</span>
            </el-form-item>
            <el-form-item>
              <el-button type="primary" size="small" @click="saveGiftRuleConfig" :loading="giftRuleSaving">{{ t('clientExt.saveRules') }}</el-button>
            </el-form-item>
          </el-form>
        </el-card>

        <div style="display:flex;justify-content:space-between;margin-bottom:16px">
          <h3 style="margin:0">{{ t('clientExt.giftRecords') }}</h3>
          <div style="display:flex;gap:8px">
            <el-button type="primary" size="small" @click="showCreateGift">{{ t('clientExt.grantGift') }}</el-button>
            <el-button size="small" @click="showBatchGift">{{ t('clientExt.batchGrant') }}</el-button>
          </div>
        </div>
        <el-table v-if="!isMobile" :data="gifts" v-loading="giftsLoading" stripe size="small">
          <el-table-column prop="id" label="ID" width="60" />
          <el-table-column :label="t('clientExt.client')" min-width="180">
            <template #default="{row}">
              <span>{{ row.client_email || ('ID: ' + row.client_id) }}</span>
            </template>
          </el-table-column>
          <el-table-column :label="t('clientExt.type')" width="80">
            <template #default="{row}">
              <el-tag :type="row.type==='days'?'warning':'success'" size="small">{{ row.type==='days'?t('clientExt.days'):t('clientExt.traffic') }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column :label="t('clientExt.value')" width="100">
            <template #default="{row}">{{ row.type==='days'?row.value+t('clientExt.dayUnit'):formatBytes(row.value) }}</template>
          </el-table-column>
          <el-table-column prop="source" :label="t('clientExt.source')" width="120" />
          <el-table-column :label="t('clientExt.status')" width="80">
            <template #default="{row}">
              <el-tag :type="giftStatusType(row.status)" size="small">{{ giftStatusLabel(row.status) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="created_at" :label="t('clientExt.grantTime')" width="170" />
          <el-table-column :label="t('clientExt.claimTime')" width="170">
            <template #default="{row}">{{ row.claimed_at || '-' }}</template>
          </el-table-column>
        </el-table>
        <MobileCards v-else :rows="gifts" :title-fn="giftTitleFn" :fields="giftCardFields" />
        <PaginationBar v-model:page="giftPage" v-model:page-size="giftSize" :total="giftTotal" @change="fetchGifts" />

        <el-dialog v-model="giftDialogVisible" :title="giftBatchMode?t('clientExt.batchGrantGift'):t('clientExt.grantGift')" width="500px">
          <el-form label-width="100px" size="small">
            <el-form-item :label="t('clientExt.selectClient')" v-if="!giftBatchMode">
              <el-select
                v-model="giftForm.client_id"
                filterable
                remote
                reserve-keyword
                :placeholder="t('clientExt.searchByEmail')"
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
            <el-form-item :label="t('clientExt.selectClients')" v-if="giftBatchMode">
              <el-select
                v-model="giftForm.client_ids_arr"
                filterable
                remote
                multiple
                reserve-keyword
                :placeholder="t('clientExt.searchByEmailMulti')"
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
            <el-form-item :label="t('clientExt.type')">
              <el-select v-model="giftForm.type" style="width:100%">
                <el-option :label="t('clientExt.days')" value="days" />
                <el-option :label="t('clientExt.traffic')" value="traffic" />
              </el-select>
            </el-form-item>
            <el-form-item :label="t('clientExt.value')">
              <el-input-number v-model="giftForm.value" :min="0" :precision="giftForm.type==='traffic'?1:0" :step="giftForm.type==='traffic'?0.5:1" style="width:100%" />
              <span style="font-size:12px;color:#909399;margin-left:8px">{{ giftForm.type==='days'?t('clientExt.dayUnit'):t('clientExt.gbUnit') }}</span>
            </el-form-item>
            <el-form-item :label="t('clientExt.source')">
              <el-select v-model="giftForm.source" style="width:100%">
                <el-option :label="t('clientExt.faultCompensation')" value="fault_compensation" />
                <el-option :label="t('clientExt.monthlyBonus')" value="monthly_bonus" />
                <el-option :label="t('clientExt.promotion')" value="promotion" />
                <el-option :label="t('clientExt.admin')" value="admin" />
              </el-select>
            </el-form-item>
          </el-form>
          <template #footer>
            <el-button @click="giftDialogVisible=false">{{ t('common.cancel') }}</el-button>
            <el-button type="primary" @click="submitGift" :loading="giftSubmitting">{{ t('common.confirm') }}</el-button>
          </template>
        </el-dialog>
      </el-tab-pane>

      <!-- ════════════ 教程文章 Tab ════════════ -->
      <el-tab-pane :label="t('clientExt.articlesTab')" name="articles">
        <div style="display:flex;justify-content:space-between;margin-bottom:16px">
          <h3 style="margin:0">{{ t('clientExt.articlesTab') }}</h3>
          <el-button type="primary" size="small" @click="showCreateArticle">{{ t('clientExt.newArticle') }}</el-button>
        </div>
        <el-table v-if="!isMobile" :data="articles" v-loading="articlesLoading" stripe size="small">
          <el-table-column prop="id" label="ID" width="60" />
          <el-table-column prop="title" :label="t('clientExt.title')" show-overflow-tooltip />
          <el-table-column prop="slug" :label="t('clientExt.slug')" width="150" show-overflow-tooltip />
          <el-table-column :label="t('clientExt.category')" width="80">
            <template #default="{row}">
              <el-tag size="small">{{ articleCategoryLabel(row.category) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column :label="t('clientExt.platform')" width="80">
            <template #default="{row}">{{ row.platform||t('clientExt.general') }}</template>
          </el-table-column>
          <el-table-column :label="t('clientExt.status')" width="80">
            <template #default="{row}">
              <el-tag :type="row.is_published?'success':'info'" size="small">{{ row.is_published?t('clientExt.published'):t('clientExt.draft') }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column :label="t('clientExt.popup')" width="60">
            <template #default="{row}">
              <el-tag :type="row.is_popup?'danger':'info'" size="small">{{ row.is_popup?t('common.yes'):t('common.no') }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="sort_order" :label="t('clientExt.sortOrder')" width="60" />
          <el-table-column :label="t('common.action')" width="180">
            <template #default="{row}">
              <el-button size="small" @click="showEditArticle(row)">{{ t('common.edit') }}</el-button>
              <el-button size="small" type="danger" @click="deleteArticle(row)">{{ t('common.delete') }}</el-button>
            </template>
          </el-table-column>
        </el-table>
        <MobileCards v-else :rows="articles" title-field="title" :fields="articleCardFields">
          <template #actions="{ row }">
            <el-button size="small" @click="showEditArticle(row)">{{ t('common.edit') }}</el-button>
            <el-button size="small" type="danger" @click="deleteArticle(row)">{{ t('common.delete') }}</el-button>
          </template>
        </MobileCards>
        <PaginationBar v-model:page="artPage" v-model:page-size="artSize" :total="artTotal" @change="fetchArticles" />

        <el-dialog v-model="articleDialogVisible" :title="articleForm.id?t('clientExt.editArticle'):t('clientExt.newArticle')" width="800px">
          <el-form label-width="80px" size="small">
            <el-form-item :label="t('clientExt.title')">
              <el-input v-model="articleForm.title" />
            </el-form-item>
            <el-form-item :label="t('clientExt.slug')">
              <el-input v-model="articleForm.slug" placeholder="url-friendly slug" />
            </el-form-item>
            <el-form-item :label="t('clientExt.category')">
              <el-select v-model="articleForm.category" style="width:100%">
                <el-option :label="t('clientExt.tutorial')" value="tutorial" />
                <el-option label="FAQ" value="faq" />
                <el-option :label="t('clientExt.notice')" value="notice" />
                <el-option :label="t('clientExt.general')" value="general" />
              </el-select>
            </el-form-item>
            <el-form-item :label="t('clientExt.platform')">
              <el-select v-model="articleForm.platform" clearable style="width:100%">
                <el-option label="Windows" value="windows" />
                <el-option label="iOS" value="ios" />
                <el-option label="Android" value="android" />
                <el-option label="Mac" value="mac" />
                <el-option label="Linux" value="linux" />
                <el-option label="Other" value="other" />
                <el-option :label="t('clientExt.game')" value="game" />
              </el-select>
            </el-form-item>
            <el-form-item :label="t('clientExt.sortOrder')">
              <el-input-number v-model="articleForm.sort_order" :min="0" />
            </el-form-item>
            <el-form-item :label="t('clientExt.publish')">
              <el-switch v-model="articleForm.is_published" />
            </el-form-item>
            <el-form-item :label="t('clientExt.homePopup')">
              <el-switch v-model="articleForm.is_popup" />
              <span style="margin-left:8px;color:#999;font-size:12px">{{ t('clientExt.homePopupHint') }}</span>
            </el-form-item>
            <el-form-item :label="t('clientExt.content')">
              <el-input v-model="articleForm.content" type="textarea" :rows="15" :placeholder="t('clientExt.markdownFormat')" />
            </el-form-item>
          </el-form>
          <template #footer>
            <el-button @click="articleDialogVisible=false">{{ t('common.cancel') }}</el-button>
            <el-button type="primary" @click="submitArticle" :loading="articleSubmitting">{{ t('common.confirm') }}</el-button>
          </template>
        </el-dialog>
      </el-tab-pane>

      <!-- ════════════ 共享账号 Tab ════════════ -->
      <el-tab-pane :label="t('clientExt.sharedAccountsTab')" name="shared_accounts">
        <div style="display:flex;justify-content:space-between;margin-bottom:16px">
          <h3 style="margin:0">{{ t('clientExt.sharedAccountsTab') }}</h3>
          <el-button type="primary" size="small" @click="showCreateSharedAccount">{{ t('clientExt.newAccount') }}</el-button>
        </div>
        <el-table v-if="!isMobile" :data="sharedAccounts" v-loading="sharedAccountsLoading" stripe size="small">
          <el-table-column prop="id" label="ID" width="60" />
          <el-table-column prop="name" :label="t('clientExt.name')" min-width="150" />
          <el-table-column :label="t('clientExt.type')" width="100">
            <template #default="{row}">
              <el-tag size="small">{{ sharedAccountTypeLabel(row.type) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column :label="t('clientExt.minPlan')" width="120">
            <template #default="{row}">{{ planTemplateNameById(row.min_plan_category) }}</template>
          </el-table-column>
          <el-table-column prop="repair_cooldown" :label="t('clientExt.repairCooldown')" width="120" />
          <el-table-column :label="t('clientExt.status')" width="80">
            <template #default="{row}">
              <el-tag :type="row.enabled?'success':'danger'" size="small">{{ row.enabled?t('clientExt.enabled'):t('clientExt.disabled') }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="sort_order" :label="t('clientExt.sortOrder')" width="60" />
          <el-table-column :label="t('common.action')" width="200">
            <template #default="{row}">
              <el-button size="small" @click="showEditSharedAccount(row)">{{ t('common.edit') }}</el-button>
              <el-button size="small" @click="viewRepairLogs(row)">{{ t('clientExt.repairLogs') }}</el-button>
              <el-button size="small" type="danger" @click="deleteSharedAccount(row)">{{ t('common.delete') }}</el-button>
            </template>
          </el-table-column>
        </el-table>
        <MobileCards v-else :rows="sharedAccounts" title-field="name" :fields="sharedAccountCardFields">
          <template #actions="{ row }">
            <el-button size="small" @click="showEditSharedAccount(row)">{{ t('common.edit') }}</el-button>
            <el-button size="small" @click="viewRepairLogs(row)">{{ t('clientExt.repairLogs') }}</el-button>
            <el-button size="small" type="danger" @click="deleteSharedAccount(row)">{{ t('common.delete') }}</el-button>
          </template>
        </MobileCards>
        <PaginationBar v-model:page="acctPage" v-model:page-size="acctSize" :total="acctTotal" @change="fetchSharedAccounts" />

        <el-dialog v-model="sharedAccountDialogVisible" :title="sharedAccountForm.id?t('clientExt.editAccount'):t('clientExt.newAccount')" width="500px">
          <el-form label-width="120px" size="small">
            <el-form-item :label="t('clientExt.name')">
              <el-input v-model="sharedAccountForm.name" />
            </el-form-item>
            <el-form-item :label="t('clientExt.type')">
              <el-select v-model="sharedAccountForm.type" style="width:100%">
                <el-option label="Apple ID" value="apple_id" />
                <el-option label="Netflix" value="netflix" />
                <el-option :label="t('clientExt.other')" value="other" />
              </el-select>
            </el-form-item>
            <el-form-item :label="t('clientExt.iconUrl')">
              <el-input v-model="sharedAccountForm.icon_url" />
            </el-form-item>
            <el-form-item :label="t('clientExt.sharedCredentials')">
              <el-input v-model="sharedAccountForm.credentials_enc" type="textarea" :rows="4" :placeholder="t('clientExt.credentialsPlaceholder')" />
              <span style="font-size:12px;color:#909399;margin-top:4px;display:block">{{ t('clientExt.credentialsHint') }}</span>
            </el-form-item>
            <el-form-item :label="t('clientExt.minPlanRequired')">
              <el-select v-model="sharedAccountForm.min_plan_category" clearable :placeholder="t('clientExt.noLimit')" style="width:100%">
                <el-option v-for="p in planTemplateOptions" :key="p.id" :label="p.name" :value="String(p.id)" />
              </el-select>
              <span style="font-size:12px;color:#909399;margin-top:4px;display:block">{{ t('clientExt.minPlanHint') }}</span>
            </el-form-item>
            <el-form-item :label="t('clientExt.repairCooldownMin')">
              <el-input-number v-model="sharedAccountForm.repair_cooldown" :min="0" />
            </el-form-item>
            <el-form-item :label="t('clientExt.sortOrder')">
              <el-input-number v-model="sharedAccountForm.sort_order" :min="0" />
            </el-form-item>
            <el-form-item :label="t('clientExt.enabled')">
              <el-switch v-model="sharedAccountForm.enabled" />
            </el-form-item>
          </el-form>
          <template #footer>
            <el-button @click="sharedAccountDialogVisible=false">{{ t('common.cancel') }}</el-button>
            <el-button type="primary" @click="submitSharedAccount" :loading="sharedAccountSubmitting">{{ t('common.confirm') }}</el-button>
          </template>
        </el-dialog>

        <el-dialog v-model="repairLogDialogVisible" :title="t('clientExt.repairLogs')" width="600px">
          <el-table v-if="!isMobile" :data="repairLogs" stripe size="small">
            <el-table-column prop="id" label="ID" width="60" />
            <el-table-column prop="client_id" :label="t('clientExt.clientId')" width="90" />
            <el-table-column :label="t('clientExt.status')" width="80">
              <template #default="{row}">
                <el-tag :type="row.status==='completed'?'success':row.status==='failed'?'danger':'warning'" size="small">{{ row.status }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="created_at" :label="t('clientExt.time')" width="170" />
          </el-table>
          <MobileCards v-else :rows="repairLogs" title-field="client_id" :fields="repairLogCardFields" />
        </el-dialog>
      </el-tab-pane>

      <!-- ════════════ IP白名单 Tab ════════════ -->
      <el-tab-pane :label="t('clientExt.ipWhitelistTab')" name="ip_whitelist">
        <div style="display:flex;justify-content:space-between;margin-bottom:16px">
          <h3 style="margin:0">{{ t('clientExt.ipWhitelistMgmt') }}</h3>
          <div style="display:flex;gap:8px;align-items:center">
            <el-input v-model="ipWhitelistClientFilter" size="small" :placeholder="t('clientExt.clientIdFilter')" style="width:150px" clearable />
            <el-button size="small" @click="fetchIPWhitelists">{{ t('common.query') }}</el-button>
          </div>
        </div>
        <el-table v-if="!isMobile" :data="ipWhitelistEntries" v-loading="ipWhitelistLoading" stripe size="small">
          <el-table-column prop="id" label="ID" width="60" />
          <el-table-column prop="client_email" :label="t('clientExt.clientEmail')" min-width="200" show-overflow-tooltip />
          <el-table-column prop="ip_address" :label="t('clientExt.ipAddress')" min-width="160" />
          <el-table-column :label="t('clientExt.source')" width="100">
            <template #default="{row}">
              <el-tag :type="row.source==='auto'?'success':'warning'" size="small">{{ row.source==='auto'?t('clientExt.autoAdded'):t('clientExt.manualAdded') }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column :label="t('clientExt.addedTime')" width="170">
            <template #default="{row}">{{ formatTime(row.created_at) }}</template>
          </el-table-column>
        </el-table>
        <MobileCards v-else :rows="ipWhitelistEntries" title-field="ip_address" :fields="ipCardFields" />
        <PaginationBar v-model:page="ipPage" v-model:page-size="ipSize" :total="ipTotal" @change="fetchIPWhitelists" />
      </el-tab-pane>

      <!-- ════════════ OAuth绑定 Tab ════════════ -->
      <el-tab-pane :label="t('clientExt.oauthBindingsTab')" name="oauth_bindings">
        <div style="display:flex;justify-content:space-between;margin-bottom:16px">
          <h3 style="margin:0">{{ t('clientExt.oauthBindingRecords') }}</h3>
          <div style="display:flex;gap:8px;align-items:center">
            <el-input v-model="oauthClientFilter" size="small" :placeholder="t('clientExt.clientIdFilter')" style="width:150px" clearable />
            <el-button size="small" @click="fetchOAuthBindings">{{ t('common.query') }}</el-button>
          </div>
        </div>
        <el-table v-if="!isMobile" :data="oauthBindings" v-loading="oauthBindingsLoading" stripe size="small">
          <el-table-column prop="id" label="ID" width="60" />
          <el-table-column prop="client_id" :label="t('clientExt.clientId')" width="100" />
          <el-table-column prop="provider" :label="t('clientExt.platform')" width="100">
            <template #default="{row}">
              <el-tag size="small">{{ row.provider }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="provider_user_id" :label="t('clientExt.thirdPartyUserId')" min-width="200" show-overflow-tooltip />
          <el-table-column :label="t('clientExt.bindingTime')" width="170">
            <template #default="{row}">{{ formatTime(row.created_at) }}</template>
          </el-table-column>
        </el-table>
        <MobileCards v-else :rows="oauthBindings" title-field="client_id" :fields="oauthCardFields" />
        <PaginationBar v-model:page="oaPage" v-model:page-size="oaSize" :total="oaTotal" @change="fetchOAuthBindings" />
      </el-tab-pane>

    </el-tabs>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  listReferralWithdrawals, approveReferralWithdrawal, rejectReferralWithdrawal,
  getReferralStats, listReferralRecords, getReferralConfig, setReferralConfig,
  listClientGifts, createClientGift, batchCreateClientGifts, searchGiftClients,
  listArticles, getArticle, createArticle, updateArticle, deleteArticle as deleteArticleApi,
  listSharedAccounts, getSharedAccount, createSharedAccount, updateSharedAccount, deleteSharedAccount as deleteSharedAccountApi, listSharedAccountRepairLogs,
  listAllIPWhitelists, listOAuthBindings
} from '../api/clientext'
import { listPlanTemplates } from '../api/plan'
import { useQueryState } from '../composables/useQueryState'
import { usePagination } from '../composables/usePagination'
import PaginationBar from '../components/PaginationBar.vue'
import MobileCards from '../components/MobileCards.vue'
import { useIsMobile } from '../composables/useIsMobile'

const { isMobile } = useIsMobile()
const activeTab = useQueryState('tab', 'referrals')

// ════════════ Referral Stats & Records ════════════
const referralStats = ref({})
const referralRecords = ref([])
const { page: refPage, pageSize: refSize, total: refTotal, limit: refLimit, offset: refOffset } = usePagination({ pageKey: 'clxRefPage', pageSizeKey: 'clxRefSize' })
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
    ElMessage.success(t('clientExt.rebateConfigSaved'))
  } catch {} finally { referralConfigSaving.value = false }
}

async function fetchReferralStats() {
  try { referralStats.value = (await getReferralStats()) || {} } catch {}
}
async function fetchReferralRecords() {
  referralRecordsLoading.value = true
  try {
    const params = { limit: refLimit.value, offset: refOffset.value }
    if (referralRecordReferrerFilter.value) params.referrer_id = referralRecordReferrerFilter.value
    const result = await listReferralRecords(params)
    referralRecords.value = result?.items || []
    refTotal.value = result?.total || 0
  } finally { referralRecordsLoading.value = false }
}

// ════════════ Referral Withdrawals ════════════
const withdrawals = ref([])
const withdrawalsLoading = ref(false)
const withdrawalTotal = ref(0)
const withdrawalPage = useQueryState('wpage', 1, { type: 'number' })
const withdrawalPageSize = useQueryState('wpageSize', 20, { type: 'number' })
const withdrawalStatusFilter = useQueryState('wstatus', '', { omit: (v) => v === '' })

async function fetchWithdrawals() {
  withdrawalsLoading.value = true
  try {
    const params = { limit: withdrawalPageSize.value, offset: (withdrawalPage.value - 1) * withdrawalPageSize.value }
    if (withdrawalStatusFilter.value) params.status = withdrawalStatusFilter.value
    const result = await listReferralWithdrawals(params)
    withdrawals.value = result?.items || []
    withdrawalTotal.value = result?.total || 0
  } finally { withdrawalsLoading.value = false }
}

async function approveWithdrawal(row) {
  const { value: remark } = await ElMessageBox.prompt(t('clientExt.processRemark'), t('clientExt.approveWithdraw'), { inputPlaceholder: t('clientExt.optional') })
  await approveReferralWithdrawal(row.id, { remark: remark || '' })
  ElMessage.success(t('clientExt.approved'))
  fetchWithdrawals()
}

async function rejectWithdrawal(row) {
  const { value: remark } = await ElMessageBox.prompt(t('clientExt.rejectReason'), t('clientExt.rejectWithdraw'), { inputPlaceholder: t('clientExt.pleaseEnterReason') })
  await rejectReferralWithdrawal(row.id, { remark: remark || '' })
  ElMessage.success(t('clientExt.rejected'))
  fetchWithdrawals()
}

function withdrawalStatusLabel(s) { return { pending: t('clientExt.withdrawPending'), approved: t('clientExt.withdrawApproved'), rejected: t('clientExt.withdrawRejected') }[s] || s }
function withdrawalStatusType(s) { return { pending: 'warning', approved: 'success', rejected: 'danger' }[s] || '' }

// ════════════ Gifts ════════════
const gifts = ref([])
const { page: giftPage, pageSize: giftSize, total: giftTotal, limit: giftLimit, offset: giftOffset } = usePagination({ pageKey: 'clxGiftPage', pageSizeKey: 'clxGiftSize' })
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
    const result = await listClientGifts({ limit: giftLimit.value, offset: giftOffset.value })
    gifts.value = result?.items || []
    giftTotal.value = result?.total || 0
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
      ElMessage.warning(t('clientExt.pleaseEnterValidValue'))
      giftSubmitting.value = false
      return
    }
    const finalValue = giftForm.value.type === 'traffic' ? Math.round(rawValue * 1073741824) : Math.round(rawValue)
    if (giftBatchMode.value) {
      const ids = giftForm.value.client_ids_arr
      if (!ids || ids.length === 0) {
        ElMessage.warning(t('clientExt.pleaseSelectOneClient'))
        giftSubmitting.value = false
        return
      }
      await batchCreateClientGifts({ client_ids: ids, type: giftForm.value.type, source: giftForm.value.source, value: finalValue })
    } else {
      if (!giftForm.value.client_id) {
        ElMessage.warning(t('clientExt.pleaseSelectClient'))
        giftSubmitting.value = false
        return
      }
      await createClientGift({ client_id: parseInt(giftForm.value.client_id), type: giftForm.value.type, value: finalValue, source: giftForm.value.source })
    }
    ElMessage.success(t('clientExt.giftGranted'))
    giftDialogVisible.value = false
    fetchGifts()
  } catch {
    // error handled by axios interceptor
  } finally { giftSubmitting.value = false }
}

function giftStatusLabel(s) { return { available: t('clientExt.giftAvailable'), claimed: t('clientExt.giftClaimed'), expired: t('clientExt.giftExpired') }[s] || s }
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
    ElMessage.success(t('clientExt.giftRuleSaved'))
  } catch {} finally { giftRuleSaving.value = false }
}

// ════════════ Articles ════════════
const articles = ref([])
const { page: artPage, pageSize: artSize, total: artTotal, limit: artLimit, offset: artOffset } = usePagination({ pageKey: 'clxArtPage', pageSizeKey: 'clxArtSize' })
const articlesLoading = ref(false)
const articleDialogVisible = ref(false)
const articleForm = ref({ title: '', slug: '', category: 'tutorial', platform: '', sort_order: 0, is_published: false, is_popup: false, content: '' })
const articleSubmitting = ref(false)

async function fetchArticles() {
  articlesLoading.value = true
  try {
    const result = await listArticles({ limit: artLimit.value, offset: artOffset.value })
    articles.value = result?.items || []
    artTotal.value = result?.total || 0
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
    ElMessage.success(t('common.saveSuccess'))
    articleDialogVisible.value = false
    fetchArticles()
  } finally { articleSubmitting.value = false }
}

async function deleteArticle(row) {
  await ElMessageBox.confirm(t('clientExt.confirmDeleteArticle'), t('common.tip'))
  await deleteArticleApi(row.id)
ElMessage.success(t('common.deleted'))
    fetchArticles()
}

function articleCategoryLabel(c) { return { tutorial: t('clientExt.tutorial'), faq: 'FAQ', notice: t('clientExt.notice'), general: t('clientExt.general') }[c] || c }

// ════════════ Shared Accounts ════════════
const sharedAccounts = ref([])
const { page: acctPage, pageSize: acctSize, total: acctTotal, limit: acctLimit, offset: acctOffset } = usePagination({ pageKey: 'clxAcctPage', pageSizeKey: 'clxAcctSize' })
const sharedAccountsLoading = ref(false)
const sharedAccountDialogVisible = ref(false)
const sharedAccountForm = ref({ name: '', type: 'other', icon_url: '', credentials_enc: '', min_plan_category: '', repair_cooldown: 1440, sort_order: 0, enabled: true })
const sharedAccountSubmitting = ref(false)
const repairLogDialogVisible = ref(false)
const repairLogs = ref([])
const planTemplateOptions = ref([])

function planTemplateNameById(idStr) {
  if (!idStr || idStr === '0') return t('clientExt.noLimit')
  const p = planTemplateOptions.value.find(pt => String(pt.id) === String(idStr))
  return p ? p.name : ('ID: ' + idStr)
}

async function fetchPlanTemplates() {
  try {
    const result = await listPlanTemplates()
    planTemplateOptions.value = Array.isArray(result) ? result : (result?.items || [])
  } catch {}
}

async function fetchSharedAccounts() {
  sharedAccountsLoading.value = true
  try {
    const result = await listSharedAccounts({ limit: acctLimit.value, offset: acctOffset.value })
    sharedAccounts.value = result?.items || []
    acctTotal.value = result?.total || 0
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
    ElMessage.success(t('common.saveSuccess'))
    sharedAccountDialogVisible.value = false
    fetchSharedAccounts()
  } finally { sharedAccountSubmitting.value = false }
}

async function deleteSharedAccount(row) {
  await ElMessageBox.confirm(t('clientExt.confirmDeleteSharedAccount'), t('common.tip'))
  await deleteSharedAccountApi(row.id)
ElMessage.success(t('common.deleted'))
    fetchSharedAccounts()
}

async function viewRepairLogs(row) {
  const result = await listSharedAccountRepairLogs(row.id, { limit: 50 })
  repairLogs.value = result?.items || []
  repairLogDialogVisible.value = true
}

function sharedAccountTypeLabel(t) { return { apple_id: 'Apple ID', netflix: 'Netflix', other: t('clientExt.other') }[t] || t }

// ════════════ IP Whitelist ════════════
function formatTime(ts) {
  if (!ts) return '-'
  const d = new Date(ts)
  if (isNaN(d.getTime())) return ts
  const p = n => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`
}

const ipWhitelistEntries = ref([])
const { page: ipPage, pageSize: ipSize, total: ipTotal, limit: ipLimit, offset: ipOffset } = usePagination({ pageKey: 'clxIpPage', pageSizeKey: 'clxIpSize' })
const ipWhitelistLoading = ref(false)
const ipWhitelistClientFilter = ref('')

async function fetchIPWhitelists() {
  ipWhitelistLoading.value = true
  try {
    const params = { limit: ipLimit.value, offset: ipOffset.value }
    if (ipWhitelistClientFilter.value) params.client_id = ipWhitelistClientFilter.value
    const result = await listAllIPWhitelists(params)
    ipWhitelistEntries.value = result?.items || []
    ipTotal.value = result?.total || 0
  } finally { ipWhitelistLoading.value = false }
}

// ════════════ OAuth Bindings ════════════
const oauthBindings = ref([])
const { page: oaPage, pageSize: oaSize, total: oaTotal, limit: oaLimit, offset: oaOffset } = usePagination({ pageKey: 'clxOaPage', pageSizeKey: 'clxOaSize' })
const oauthBindingsLoading = ref(false)
const oauthClientFilter = ref('')

async function fetchOAuthBindings() {
  oauthBindingsLoading.value = true
  try {
    const params = { limit: oaLimit.value, offset: oaOffset.value }
    if (oauthClientFilter.value) params.client_id = oauthClientFilter.value
    const result = await listOAuthBindings(params)
    oauthBindings.value = result?.items || []
    oaTotal.value = result?.total || 0
  } finally { oauthBindingsLoading.value = false }
}

// ════════════ 移动端卡片字段（与桌面列一一对应）════════════
const refRecordTitleFn = row => row.referrer_email || row.referrer_id
const giftTitleFn = row => row.client_email || ('ID: ' + row.client_id)

const withdrawalCardFields = computed(() => [
  { label: 'ID', text: row => row.id ?? '-' },
  { label: t('clientExt.amount'), text: row => row.amount != null ? ((row.amount / 100).toFixed(2) + ' ' + t('clientExt.yuan')) : '-' },
  { label: t('clientExt.withdrawType'), text: row => row.withdraw_type === 'balance' ? t('clientExt.toBalance') : t('clientExt.usdt') },
  { label: t('clientExt.walletAddress'), text: row => row.wallet_address || '-' },
  { label: t('clientExt.status'), text: row => withdrawalStatusLabel(row.status) },
  { label: t('clientExt.processRemark'), text: row => row.process_remark || '-' },
  { label: t('clientExt.applicationTime'), text: row => row.created_at || '-' },
])

const refRecordCardFields = computed(() => [
  { label: 'ID', text: row => row.id ?? '-' },
  { label: t('clientExt.invitee'), text: row => row.referee_email || row.referee_id || '-' },
  { label: t('clientExt.commission'), text: row => '¥' + ((row.commission_amount || 0) / 100).toFixed(2) },
  { label: t('clientExt.commissionStatus'), text: row => ({ available: t('clientExt.commissionAvailable'), paid: t('clientExt.commissionPaid'), pending: t('clientExt.commissionPending') })[row.commission_status] || row.commission_status || '-' },
  { label: t('clientExt.createdAt'), text: row => row.created_at || '-' },
])

const giftCardFields = computed(() => [
  { label: 'ID', text: row => row.id ?? '-' },
  { label: t('clientExt.type'), text: row => row.type === 'days' ? t('clientExt.days') : t('clientExt.traffic') },
  { label: t('clientExt.value'), text: row => row.type === 'days' ? row.value + t('clientExt.dayUnit') : formatBytes(row.value) },
  { label: t('clientExt.source'), text: row => row.source || '-' },
  { label: t('clientExt.status'), text: row => giftStatusLabel(row.status) },
  { label: t('clientExt.grantTime'), text: row => row.created_at || '-' },
  { label: t('clientExt.claimTime'), text: row => row.claimed_at || '-' },
])

const articleCardFields = computed(() => [
  { label: 'ID', text: row => row.id ?? '-' },
  { label: t('clientExt.slug'), text: row => row.slug || '-' },
  { label: t('clientExt.category'), text: row => articleCategoryLabel(row.category) },
  { label: t('clientExt.platform'), text: row => row.platform || t('clientExt.general') },
  { label: t('clientExt.status'), text: row => row.is_published ? t('clientExt.published') : t('clientExt.draft') },
  { label: t('clientExt.popup'), text: row => row.is_popup ? t('common.yes') : t('common.no') },
  { label: t('clientExt.sortOrder'), text: row => row.sort_order ?? '-' },
])

const sharedAccountCardFields = computed(() => [
  { label: 'ID', text: row => row.id ?? '-' },
  { label: t('clientExt.type'), text: row => sharedAccountTypeLabel(row.type) },
  { label: t('clientExt.minPlan'), text: row => planTemplateNameById(row.min_plan_category) },
  { label: t('clientExt.repairCooldown'), text: row => row.repair_cooldown ?? '-' },
  { label: t('clientExt.status'), text: row => row.enabled ? t('clientExt.enabled') : t('clientExt.disabled') },
  { label: t('clientExt.sortOrder'), text: row => row.sort_order ?? '-' },
])

const repairLogCardFields = computed(() => [
  { label: 'ID', text: row => row.id ?? '-' },
  { label: t('clientExt.status'), text: row => row.status || '-' },
  { label: t('clientExt.time'), text: row => row.created_at || '-' },
])

const ipCardFields = computed(() => [
  { label: 'ID', text: row => row.id ?? '-' },
  { label: t('clientExt.clientEmail'), text: row => row.client_email || `#${row.client_id ?? '-'}` },
  { label: t('clientExt.ipAddress'), text: row => row.ip_address || '-' },
  { label: t('clientExt.source'), text: row => row.source === 'auto' ? t('clientExt.autoAdded') : t('clientExt.manualAdded') },
  { label: t('clientExt.addedTime'), text: row => formatTime(row.created_at) },
])

const oauthCardFields = computed(() => [
  { label: 'ID', text: row => row.id ?? '-' },
  { label: t('clientExt.platform'), text: row => row.provider || '-' },
  { label: t('clientExt.thirdPartyUserId'), text: row => row.provider_user_id || '-' },
  { label: t('clientExt.bindingTime'), text: row => formatTime(row.created_at) },
])

onMounted(() => { fetchReferralConfig(); fetchReferralStats(); fetchReferralRecords(); fetchWithdrawals(); fetchGifts(); fetchArticles(); fetchSharedAccounts(); fetchIPWhitelists(); fetchOAuthBindings(); fetchPlanTemplates() })
</script>
