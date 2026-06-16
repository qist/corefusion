<template>
  <div>
    <el-form label-width="180px" label-position="left">
      <el-divider content-position="left">Level 0</el-divider>
      <el-form-item label="Handshake Timeout (s)">
        <el-input-number v-model="level0.handshake" :min="0" @change="emit" />
      </el-form-item>
      <el-form-item label="Connection Idle (s)">
        <el-input-number v-model="level0.connIdle" :min="0" @change="emit" />
      </el-form-item>
      <el-form-item label="Uplink Only (s)">
        <el-input-number v-model="level0.uplinkOnly" :min="0" @change="emit" />
      </el-form-item>
      <el-form-item label="Downlink Only (s)">
        <el-input-number v-model="level0.downlinkOnly" :min="0" @change="emit" />
      </el-form-item>
      <el-form-item label="Stats User Uplink">
        <el-switch v-model="level0.statsUserUplink" @change="emit" />
      </el-form-item>
      <el-form-item label="Stats User Downlink">
        <el-switch v-model="level0.statsUserDownlink" @change="emit" />
      </el-form-item>
      <el-form-item label="Buffer Size (bytes)">
        <el-input-number v-model="level0.bufferSize" :min="0" @change="emit" />
      </el-form-item>

      <el-divider content-position="left">System</el-divider>
      <el-form-item label="Stats Inbound Uplink">
        <el-switch v-model="form.system.statsInboundUplink" @change="emit" />
      </el-form-item>
      <el-form-item label="Stats Inbound Downlink">
        <el-switch v-model="form.system.statsInboundDownlink" @change="emit" />
      </el-form-item>
      <el-form-item label="Stats Outbound Uplink">
        <el-switch v-model="form.system.statsOutboundUplink" @change="emit" />
      </el-form-item>
      <el-form-item label="Stats Outbound Downlink">
        <el-switch v-model="form.system.statsOutboundDownlink" @change="emit" />
      </el-form-item>
    </el-form>
  </div>
</template>
<script setup>
import { reactive, computed, onMounted } from 'vue'
const props = defineProps({ data: Object })
const emitFn = defineEmits(['update'])
const form = reactive({
  levels: { '0': { handshake:4, connIdle:300, uplinkOnly:2, downlinkOnly:5, statsUserUplink:true, statsUserDownlink:true, bufferSize:10240 } },
  system: { statsInboundUplink:true, statsInboundDownlink:true, statsOutboundUplink:false, statsOutboundDownlink:false },
})
const level0 = computed(() => form.levels['0'])
function sync(){
  if(!props.data)return
  if(props.data.levels) form.levels = JSON.parse(JSON.stringify(props.data.levels))
  if(props.data.system) Object.assign(form.system, props.data.system)
}
function emit(){ emitFn('update', JSON.parse(JSON.stringify(form))) }
onMounted(sync)
</script>
