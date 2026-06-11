<template>
  <div>
    <div style="display:flex;justify-content:space-between;margin-bottom:20px">
      <h2>{{ t('node.title') }}</h2>
      <el-button type="primary" @click="showCreate">{{ t('node.create') }}</el-button>
    </div>
    <el-card>
      <el-table :data="nodes" v-loading="loading" stripe>
        <el-table-column prop="id" :label="t('common.id')" width="60" />
        <el-table-column prop="name" :label="t('common.name')" />
        <el-table-column prop="core_type" :label="t('node.coreType')" width="100" />
        <el-table-column prop="protocol" :label="t('node.protocol')" width="100" />
        <el-table-column prop="host" :label="t('node.host')" />
        <el-table-column prop="port" :label="t('node.port')" width="80" />
        <el-table-column :label="t('common.status')" width="80">
          <template #default="{row}"><el-tag :type="row.status===1?'success':'danger'" size="small">{{ row.status===1?t('common.enabled'):t('common.disabled') }}</el-tag></template>
        </el-table-column>
        <el-table-column :label="t('common.actions')" width="160">
          <template #default="{row}">
            <el-button size="small" @click="showEdit(row)">{{ t('common.edit') }}</el-button>
            <el-button size="small" type="danger" @click="handleDelete(row)">{{ t('common.delete') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-pagination v-model:current-page="page" :page-size="20" :total="total" layout="total,prev,pager,next" style="margin-top:16px;justify-content:flex-end" @current-change="fetchData" />
    </el-card>
    <el-dialog v-model="dialogVisible" :title="editing?t('node.edit'):t('node.create')" width="500">
      <el-form :model="form" label-width="80px">
        <el-form-item :label="t('common.name')"><el-input v-model="form.name" /></el-form-item>
        <el-form-item :label="t('node.coreType')"><el-select v-model="form.core_type" style="width:100%"><el-option label="Xray" value="xray" /><el-option label="sing-box" value="singbox" /><el-option label="Hysteria 2" value="hysteria2" /><el-option label="TUIC" value="tuic" /></el-select></el-form-item>
        <el-form-item :label="t('node.protocol')"><el-select v-model="form.protocol" style="width:100%"><el-option label="VLESS" value="vless" /><el-option label="VMess" value="vmess" /><el-option label="Trojan" value="trojan" /><el-option label="Hysteria 2" value="hysteria2" /></el-select></el-form-item>
        <el-form-item :label="t('node.host')"><el-input v-model="form.host" /></el-form-item>
        <el-form-item :label="t('node.port')"><el-input-number v-model="form.port" :min="1" :max="65535" /></el-form-item>
        <el-form-item :label="t('node.transport')"><el-select v-model="form.transport" style="width:100%" clearable><el-option :label="t('transport.tcp')" value="tcp" /><el-option :label="t('transport.ws')" value="ws" /><el-option :label="t('transport.grpc')" value="grpc" /></el-select></el-form-item>
        <el-form-item :label="t('node.tlsMode')"><el-select v-model="form.tls_mode" style="width:100%" clearable><el-option label="None" value="none" /><el-option label="TLS" value="tls" /><el-option label="Reality" value="reality" /></el-select></el-form-item>
        <el-form-item :label="t('node.region')"><el-input v-model="form.region" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="dialogVisible=false">{{ t('common.cancel') }}</el-button><el-button type="primary" :loading="saving" @click="handleSave">{{ t('common.save') }}</el-button></template>
    </el-dialog>
  </div>
</template>
<script setup>
import {ref,onMounted} from 'vue'
import {ElMessage,ElMessageBox} from 'element-plus'
import {useI18n} from 'vue-i18n'
import client from '../api/client'
const {t}=useI18n()
const nodes=ref([]);const loading=ref(false);const total=ref(0);const page=ref(1)
const dialogVisible=ref(false);const editing=ref(null);const saving=ref(false)
const defaultForm={name:'',core_type:'xray',protocol:'vless',host:'',port:443,transport:'tcp',tls_mode:'tls',region:''}
const form=ref({...defaultForm})
async function fetchData(){loading.value=true;try{const r=await client.get('/nodes',{params:{limit:20,offset:(page.value-1)*20}});nodes.value=r.items||[];total.value=r.total||0}finally{loading.value=false}}
function showCreate(){editing.value=null;form.value={...defaultForm};dialogVisible.value=true}
function showEdit(row){editing.value=row;form.value={...row};dialogVisible.value=true}
async function handleSave(){saving.value=true;try{if(editing.value){await client.put(`/nodes/${editing.value.id}`,form.value)}else{await client.post('/nodes',form.value)}ElMessage.success(t('common.success'));dialogVisible.value=false;fetchData()}finally{saving.value=false}}
async function handleDelete(row){await ElMessageBox.confirm(t('node.deleteConfirm',{name:row.name}),t('common.confirm'));await client.delete(`/nodes/${row.id}`);ElMessage.success(t('common.success'));fetchData()}
onMounted(fetchData)
</script>
