<template>
  <div>
    <div style="display:flex;justify-content:space-between;margin-bottom:20px"><h2>{{ t('user.title') }}</h2><el-button type="primary" @click="showCreate">{{ t('user.create') }}</el-button></div>
    <el-card>
      <el-table :data="users" v-loading="loading" stripe>
        <el-table-column prop="id" :label="t('common.id')" width="60" />
        <el-table-column prop="username" :label="t('user.username')" width="120" />
        <el-table-column prop="email" :label="t('user.email')" />
        <el-table-column prop="role" :label="t('user.role')" width="120"><template #default="{row}"><el-tag size="small">{{ roleLabel(row.role) }}</el-tag></template></el-table-column>
        <el-table-column :label="t('common.status')" width="80"><template #default="{row}"><el-tag :type="row.status===1?'success':'danger'" size="small">{{ row.status===1?t('common.enabled'):t('common.disabled') }}</el-tag></template></el-table-column>
        <el-table-column :label="t('common.actions')" width="160">
          <template #default="{row}"><el-button size="small" @click="showEdit(row)">{{ t('common.edit') }}</el-button><el-button size="small" type="danger" @click="handleDelete(row)" :disabled="row.id===1">{{ t('common.delete') }}</el-button></template>
        </el-table-column>
      </el-table>
    </el-card>
    <PaginationBar v-model:page="page" v-model:page-size="pageSize" :total="total" @change="fetchData" />
    <el-dialog v-model="dialogVisible" :title="editing?t('user.edit'):t('user.create')" width="400">
      <el-form :model="form" label-width="80px">
        <el-form-item :label="t('user.username')"><el-input v-model="form.username" :disabled="!!editing" /></el-form-item>
        <el-form-item :label="t('user.email')"><el-input v-model="form.email" placeholder="user@example.com" /></el-form-item>
        <el-form-item :label="t('user.password')"><el-input v-model="form.password" type="password" :placeholder="editing?t('user.passwordPlaceholder'):t('user.password')" /></el-form-item>
        <el-form-item :label="t('user.role')"><el-select v-model="form.role" style="width:100%"><el-option :label="t('user.roles.superAdmin')" value="super_admin" /><el-option :label="t('user.roles.admin')" value="admin" /><el-option :label="t('user.roles.operator')" value="operator" /><el-option :label="t('user.roles.viewer')" value="viewer" /></el-select></el-form-item>
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
import {usePagination} from '../composables/usePagination'
import PaginationBar from '../components/PaginationBar.vue'
const {t}=useI18n()
const {page,pageSize}=usePagination({pageKey:'usersPage',pageSizeKey:'usersSize'})
const users=ref([]);const loading=ref(false);const total=ref(0);const dialogVisible=ref(false);const editing=ref(null);const saving=ref(false)
const form=ref({username:'',email:'',password:'',role:'viewer'})
const roleLabel=(r)=>({super_admin:t('user.roles.superAdmin'),admin:t('user.roles.admin'),operator:t('user.roles.operator'),viewer:t('user.roles.viewer')}[r]||r)
async function fetchData(){loading.value=true;try{const r=await client.get('/users',{params:{limit:pageSize.value,offset:(page.value-1)*pageSize.value}});users.value=r.items||[];total.value=r.total||0}finally{loading.value=false}}
function showCreate(){editing.value=null;form.value={username:'',email:'',password:'',role:'viewer'};dialogVisible.value=true}
function showEdit(row){editing.value=row;form.value={username:row.username,email:row.email||'',password:'',role:row.role};dialogVisible.value=true}
async function handleSave(){saving.value=true;try{if(editing.value){const p={role:form.value.role,email:form.value.email};if(form.value.password)p.password=form.value.password;await client.put(`/users/${editing.value.id}`,p)}else{await client.post('/users',form.value)}ElMessage.success(t('common.success'));dialogVisible.value=false;fetchData()}finally{saving.value=false}}
async function handleDelete(row){await ElMessageBox.confirm(t('user.deleteConfirm',{username:row.username}),t('common.confirm'));await client.delete(`/users/${row.id}`);ElMessage.success(t('common.success'));fetchData()}
onMounted(fetchData)
</script>
