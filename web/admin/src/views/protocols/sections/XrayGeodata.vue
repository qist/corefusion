<template>
  <el-form label-width="140px" label-position="left">
    <el-form-item label="Cron Schedule">
      <el-input v-model="form.cron" placeholder="e.g. 0 3 * * *" @change="emit" />
    </el-form-item>
    <el-form-item label="Outbound Tag">
      <el-input v-model="form.outbound" placeholder="direct" @change="emit" />
    </el-form-item>
    <el-divider content-position="left">Assets</el-divider>
    <div v-for="(a, i) in form.assets" :key="i" class="sub-item">
      <el-input v-model="a.url" placeholder="https://..." style="width:50%" @change="emit" />
      <el-input v-model="a.file" placeholder="geoip.dat" style="width:35%" @change="emit" />
      <el-button size="small" type="danger" text @click="rmAsset(i)">X</el-button>
    </div>
    <el-button size="small" @click="addAsset">Add Asset</el-button>
  </el-form>
</template>
<script setup>
import { reactive, onMounted } from 'vue'
const props = defineProps({ data: Object })
const emitFn = defineEmits(['update'])
const form = reactive({ cron: '', outbound: 'direct', assets: [] })
function sync() { if (!props.data) return; form.cron = props.data.cron || ''; form.outbound = props.data.outbound || 'direct'; form.assets = (props.data.assets || []).map(a => ({ ...a })) }
function emit() { emitFn('update', JSON.parse(JSON.stringify(form))) }
function addAsset() { form.assets.push({ url: '', file: '' }); emit() }
function rmAsset(i) { form.assets.splice(i, 1); emit() }
onMounted(sync)
</script>
<style scoped>.sub-item { display: flex; gap: 8px; margin-bottom: 8px; align-items: center; }</style>
