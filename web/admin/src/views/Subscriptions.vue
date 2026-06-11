<template>
  <div>
    <div style="display:flex;justify-content:space-between;margin-bottom:20px"><h2>{{ t('subscription.title') }}</h2><el-button type="primary" @click="showCreate">{{ t('subscription.create') }}</el-button></div>
    <el-card>
      <el-table :data="subscriptions" v-loading="loading" stripe>
        <el-table-column prop="id" :label="t('common.id')" width="60" />
        <el-table-column :label="t('subscription.token')" width="160"><template #default="{row}"><span style="font-family:monospace;font-size:12px">{{ row.token_masked }}</span></template></el-table-column>
        <el-table-column prop="format" :label="t('subscription.format')" width="100" />
        <el-table-column :label="t('common.status')" width="80"><template #default="{row}"><el-tag :type="{active:'success',revoked:'danger',expired:'warning'}[row.status]" size="small">{{ row.status }}</el-tag></template></el-table-column>
        <el-table-column :label="t('subscription.quotaState')" width="100"><template #default="{row}"><el-tag :type="quotaType(row.quota_state)" size="small">{{ row.quota_state }}</el-tag></template></el-table-column>
        <el-table-column :label="`${t('subscription.usedBytes')}/${t('subscription.quotaBytes')}`" width="140"><template #default="{row}">{{ fmtBytes(row.used_bytes_cache) }} / {{ row.quota_bytes ? fmtBytes(row.quota_bytes) : t('common.unlimited') }}</template></el-table-column>
        <el-table-column :label="t('common.actions')" width="200">
          <template #default="{row}">
            <el-button size="small" @click="handleRotate(row)">{{ t('subscription.rotate') }}</el-button>
            <el-button size="small" type="danger" @click="handleRevoke(row)" :disabled="row.status==='revoked'">{{ t('subscription.revoke') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-pagination v-model:current-page="page" :page-size="20" :total="total" layout="total,prev,pager,next" style="margin-top:16px;justify-content:flex-end" @current-change="fetchData" />
    </el-card>
    <el-dialog v-model="dialogVisible" :title="t('subscription.create')" width="500">
      <el-form :model="form" label-width="100px">
        <el-form-item :label="t('subscription.clientId')"><el-input-number v-model="form.client_id" :min="1" /></el-form-item>
        <el-form-item :label="t('subscription.format')"><el-select v-model="form.format" style="width:100%"><el-option label="sing-box JSON" value="singbox_json" /><el-option label="Xray JSON" value="xray_json" /><el-option label="Mihomo YAML" value="mihomo_yaml" /></el-select></el-form-item>
        <el-form-item :label="`${t('subscription.quotaBytes')} (GB)`"><el-input-number v-model="quotaGB" :min="0" /><span style="margin-left:8px;color:#999">0 = {{ t('common.unlimited') }}</span></el-form-item>
      </el-form>
      <template #footer><el-button @click="dialogVisible=false">{{ t('common.cancel') }}</el-button><el-button type="primary" :loading="saving" @click="handleCreate">{{ t('common.create') }}</el-button></template>
    </el-dialog>
  </div>
</template>
<script setup>
import {ref,onMounted} from 'vue'
import {ElMessage,ElMessageBox} from 'element-plus'
import {useI18n} from 'vue-i18n'
import client from '../api/client'
const {t}=useI18n()
const subscriptions=ref([]);const loading=ref(false);const total=ref(0);const page=ref(1)
const dialogVisible=ref(false);const saving=ref(false);const quotaGB=ref(0)
const form=ref({client_id:1,format:'singbox_json'})
const quotaType=(s)=>({normal:'success',near_limit:'warning',over_limit_soft:'danger',over_limit_hard:'danger',suspended:'danger',expired_days:'warning'}[s]||'info')
function fmtBytes(b){if(!b)return'0 B';const u=['B','KB','MB','GB','TB'];let i=0;while(b>=1024&&i<u.length-1){b/=1024;i++}return b.toFixed(1)+' '+u[i]}
async function fetchData(){loading.value=true;try{const r=await client.get('/subscriptions',{params:{limit:20,offset:(page.value-1)*20}});subscriptions.value=r.items||[];total.value=r.total||0}finally{loading.value=false}}
function showCreate(){form.value={client_id:1,format:'singbox_json'};quotaGB.value=0;dialogVisible.value=true}
async function handleCreate(){saving.value=true;try{const r=await client.post('/subscriptions',{...form.value,quota_bytes:quotaGB.value*1024*1024*1024});ElMessage.success(`${t('subscription.newToken')}: ${r.token}`);dialogVisible.value=false;fetchData()}finally{saving.value=false}}
async function handleRotate(row){await ElMessageBox.confirm(t('subscription.rotateConfirm'),t('common.confirm'));const r=await client.post(`/subscriptions/${row.id}/rotate`,{reason:'manual'});ElMessage.success(`${t('subscription.newToken')}: ${r.token}`);fetchData()}
async function handleRevoke(row){await ElMessageBox.confirm(t('subscription.revokeConfirm'),t('common.confirm'));await client.post(`/subscriptions/${row.id}/revoke`);ElMessage.success(t('common.success'));fetchData()}
onMounted(fetchData)
</script>
