<template>
  <div>
    <div class="sub-header">
      <span>Inbound List</span>
      <el-button size="small" type="primary" @click="addInbound">Add Inbound</el-button>
    </div>
    <div v-for="(ib, i) in form" :key="i" class="item-card">
      <el-card shadow="never">
        <div class="card-header">
          <span>#{{ i + 1 }} {{ ib.tag }}</span>
          <el-button size="small" type="danger" text @click="form.splice(i, 1)">Remove</el-button>
        </div>
        <el-form label-width="120px" label-position="left" size="small">
          <el-form-item label="Type">
            <el-select v-model="ib.type" style="width:100%">
              <el-option label="mixed" value="mixed" /><el-option label="http" value="http" />
              <el-option label="socks" value="socks" /><el-option label="tun" value="tun" />
              <el-option label="vmess" value="vmess" /><el-option label="vless" value="vless" />
              <el-option label="trojan" value="trojan" /><el-option label="hysteria2" value="hysteria2" />
              <el-option label="shadowsocks" value="shadowsocks" /><el-option label="tuic" value="tuic" />
            </el-select>
          </el-form-item>
          <el-form-item label="Tag">
            <el-input v-model="ib.tag" />
          </el-form-item>
          <el-form-item label="Listen">
            <el-input v-model="ib.listen" placeholder="127.0.0.1" />
          </el-form-item>
          <el-form-item label="Listen Port">
            <el-input-number v-model="ib.listen_port" :min="1" :max="65535" />
          </el-form-item>
          <el-form-item label="Sniff">
            <el-switch v-model="ib.sniff" />
          </el-form-item>
          <template v-if="ib.type === 'tun'">
            <el-form-item label="Interface">
              <el-input v-model="ib.interface_name" placeholder="tun0" />
            </el-form-item>
            <el-form-item label="Stack">
              <el-select v-model="ib.stack" style="width:100%">
                <el-option label="system" value="system" /><el-option label="gvisor" value="gvisor" /><el-option label="mixed" value="mixed" />
              </el-select>
            </el-form-item>
            <el-form-item label="Auto Route">
              <el-switch v-model="ib.auto_route" />
            </el-form-item>
            <el-form-item label="Strict Route">
              <el-switch v-model="ib.strict_route" />
            </el-form-item>
          </template>
        </el-form>
      </el-card>
    </div>
  </div>
</template>
<script setup>
import { reactive, watch, onMounted } from 'vue'
const props = defineProps({ data: Array })
const emit = defineEmits(['update'])
const form = reactive([])

function syncFromData() {
  form.splice(0, form.length)
  if (!Array.isArray(props.data)) return
  props.data.forEach(ib => form.push({ ...ib }))
}

function addInbound() {
  form.push({ type: 'mixed', tag: 'mixed-in', listen: '127.0.0.1', listen_port: 1080, sniff: true })
}

watch(() => props.data, syncFromData, { deep: true })
watch(form, () => emit('update', JSON.parse(JSON.stringify(form))), { deep: true })
onMounted(syncFromData)
</script>
<style scoped>
.sub-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; font-weight: 600; }
.item-card { margin-bottom: 16px; }
.card-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; font-weight: 600; }
</style>
