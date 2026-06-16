<template>
  <div>
    <div class="sub-header">
      <span>Users (UUID → Password)</span>
      <el-button size="small" type="primary" @click="addUser">Add User</el-button>
    </div>
    <div v-for="(u, i) in usersList" :key="i" class="sub-item">
      <el-input v-model="u.uuid" placeholder="UUID" style="width:45%" />
      <el-input v-model="u.password" placeholder="Password" show-password style="width:35%" />
      <el-button size="small" type="danger" text @click="usersList.splice(i, 1)">Remove</el-button>
    </div>
  </div>
</template>
<script setup>
import { reactive, watch, onMounted } from 'vue'
const props = defineProps({ data: Object })
const emit = defineEmits(['update'])
const usersList = reactive([])

function syncFromData() {
  usersList.splice(0, usersList.length)
  if (!props.data || typeof props.data !== 'object') return
  Object.entries(props.data).forEach(([uuid, password]) => usersList.push({ uuid, password }))
}

function buildOutput() {
  const out = {}
  usersList.forEach(u => { if (u.uuid) out[u.uuid] = u.password })
  return out
}

function addUser() { usersList.push({ uuid: '', password: '' }) }

watch(() => props.data, syncFromData, { deep: true })
watch(usersList, () => emit('update', buildOutput()), { deep: true })
onMounted(syncFromData)
</script>
<style scoped>
.sub-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; font-weight: 600; }
.sub-item { display: flex; gap: 8px; margin-bottom: 8px; align-items: center; }
</style>
