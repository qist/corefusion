<template>
  <div>
    <div style="display:flex;justify-content:space-between;margin-bottom:20px"><h2>{{ t('subscription.title') }}</h2></div>
    <el-card>
      <el-table :data="subscriptions" v-loading="loading" stripe>
        <el-table-column prop="id" :label="t('common.id')" width="60" />
        <el-table-column :label="t('subscription.clientEmail')" width="180">
          <template #default="{row}">{{ clientEmail(row.client_id) }}</template>
        </el-table-column>
        <el-table-column :label="t('subscription.token')" width="160"><template #default="{row}"><span style="font-family:monospace;font-size:12px">{{ row.token_masked }}</span></template></el-table-column>
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
      <PaginationBar v-model:page="page" v-model:page-size="pageSize" :total="total" @change="fetchData" />
    </el-card>
  </div>
</template>
<script setup>
import {ref,onMounted} from 'vue'
import {ElMessage,ElMessageBox} from 'element-plus'
import {useI18n} from 'vue-i18n'
import {useQueryState} from '../composables/useQueryState'
import PaginationBar from '../components/PaginationBar.vue'
import client from '../api/client'
const {t}=useI18n()
const subscriptions=ref([])
const clients=ref([])
const loading=ref(false)
const total=ref(0)
const page=useQueryState('subPage',1,{type:'number'})
const pageSize=useQueryState('subPageSize',20,{type:'number'})

const quotaType=(s)=>({normal:'success',near_limit:'warning',over_limit_soft:'danger',over_limit_hard:'danger',suspended:'danger',expired_days:'warning'}[s]||'info')
function fmtBytes(b){if(!b)return'0 B';const u=['B','KB','MB','GB','TB'];let i=0;while(b>=1024&&i<u.length-1){b/=1024;i++}return b.toFixed(1)+' '+u[i]}

const clientEmail = (clientId) => {
  const c = clients.value.find(cl => cl.id === clientId)
  return c ? c.email : `#${clientId}`
}

async function fetchData(){
  loading.value=true
  try{
    const [subRes, clientRes] = await Promise.all([
      client.get('/subscriptions',{params:{limit:pageSize.value,offset:(page.value-1)*pageSize.value}}),
      client.get('/clients',{params:{limit:100}})
    ])
    subscriptions.value=subRes.items||[]
    total.value=subRes.total||0
    clients.value=clientRes.items||[]
  }finally{loading.value=false}
}

async function handleRotate(row){
  await ElMessageBox.confirm(t('subscription.rotateConfirm'),t('common.confirm'))
  const r=await client.post(`/subscriptions/${row.id}/rotate`,{reason:'manual'})
  ElMessage.success(`${t('subscription.newToken')}: ${r.token}`)
  fetchData()
}

async function handleRevoke(row){
  await ElMessageBox.confirm(t('subscription.revokeConfirm'),t('common.confirm'))
  await client.post(`/subscriptions/${row.id}/revoke`)
  ElMessage.success(t('common.success'))
  fetchData()
}

onMounted(fetchData)
</script>
