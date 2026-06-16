<template>
  <div>
    <el-form label-width="120px" label-position="left">
      <el-divider content-position="left">Bridges</el-divider>
      <div v-for="(b, i) in form.bridges" :key="i" class="sub-item">
        <el-input v-model="b.tag" placeholder="Tag" style="width:30%" />
        <el-input v-model="b.domain" placeholder="Domain" style="width:50%" />
        <el-button size="small" type="danger" text @click="form.bridges.splice(i, 1)">Remove</el-button>
      </div>
      <el-button size="small" @click="form.bridges.push({ tag: '', domain: '' })">Add Bridge</el-button>

      <el-divider content-position="left">Portals</el-divider>
      <div v-for="(p, i) in form.portals" :key="i" class="sub-item">
        <el-input v-model="p.tag" placeholder="Tag" style="width:30%" />
        <el-input v-model="p.domain" placeholder="Domain" style="width:50%" />
        <el-button size="small" type="danger" text @click="form.portals.splice(i, 1)">Remove</el-button>
      </div>
      <el-button size="small" @click="form.portals.push({ tag: '', domain: '' })">Add Portal</el-button>
    </el-form>
  </div>
</template>
<script setup>
import { reactive, watch, onMounted } from 'vue'
const props = defineProps({ data: Object })
const emit = defineEmits(['update'])
const form = reactive({ bridges: [], portals: [] })
function syncFromData() {
  if (!props.data) return
  form.bridges = (props.data.bridges || []).map(b => ({ ...b }))
  form.portals = (props.data.portals || []).map(p => ({ ...p }))
}
watch(() => props.data, syncFromData, { deep: true })

onMounted(syncFromData)
</script>
<style scoped>.sub-item { display: flex; gap: 8px; margin-bottom: 8px; align-items: center; }</style>
