<template>
  <el-form label-width="120px" label-position="left">
    <el-form-item label="Auth Type">
      <el-select v-model="form.type" style="width:100%">
        <el-option label="Password" value="password" /><el-option label="User/Pass" value="userpass" />
      </el-select>
    </el-form-item>
    <el-form-item v-if="form.type === 'password'" label="Password">
      <el-input v-model="form.password" show-password />
    </el-form-item>
    <template v-if="form.type === 'userpass'">
      <div v-for="(entry, i) in userPassList" :key="i" class="sub-item">
        <el-input v-model="entry.user" placeholder="Username" style="width:40%" />
        <el-input v-model="entry.pass" placeholder="Password" show-password style="width:40%" />
        <el-button size="small" type="danger" text @click="userPassList.splice(i, 1)">X</el-button>
      </div>
      <el-button size="small" @click="userPassList.push({ user: '', pass: '' })">Add User</el-button>
    </template>
  </el-form>
</template>
<script setup>
import { reactive, watch, onMounted } from 'vue'
const props = defineProps({ data: Object })
const emit = defineEmits(['update'])
const form = reactive({ type: 'password', password: '' })
const userPassList = reactive([])

function syncFromData() {
  if (!props.data) return
  form.type = props.data.type || 'password'
  form.password = props.data.password || ''
  userPassList.splice(0, userPassList.length)
  if (props.data.userpass) {
    Object.entries(props.data.userpass).forEach(([u, p]) => userPassList.push({ user: u, pass: p }))
  }
}

function buildOutput() {
  if (form.type === 'password') return { type: 'password', password: form.password }
  const up = {}
  userPassList.forEach(e => { if (e.user) up[e.user] = e.pass })
  return { type: 'userpass', userpass: up }
}

watch(() => props.data, syncFromData, { deep: true })
watch([form, userPassList], () => emit('update', buildOutput()), { deep: true })
onMounted(syncFromData)
</script>
<style scoped>.sub-item { display: flex; gap: 8px; margin-bottom: 8px; align-items: center; }</style>
