<template>
  <el-tabs>
    <el-tab-pane label="Basic" name="basic"><SingboxOutboundBase :model="model" @update="emit" /></el-tab-pane>
    <el-tab-pane label="WireGuard" name="proto">
      <el-form label-width="150px" label-position="left">
        <el-form-item label="Private Key">
          <div style="display:flex;gap:8px;flex:1">
            <el-input v-model="model._wg_private_key" @change="emit" />
            <el-button size="small" @click="model._wg_private_key = generateKey(); emit()">Generate</el-button>
          </div>
        </el-form-item>
        <el-form-item label="Address">
          <el-input v-model="model._wg_address" placeholder="10.0.0.1/24" @change="emit" />
        </el-form-item>
        <el-form-item label="Peers">
          <div style="width:100%">
            <div v-for="(p, i) in (model.peers || [])" :key="i" style="display:flex;flex-wrap:wrap;gap:6px;margin-bottom:6px">
              <el-input v-model="p.server" placeholder="peer server" style="width:140px" @change="emit" />
              <el-input v-model="p['public_key']" placeholder="public key" style="flex:1" @change="emit" />
              <el-input v-model="p.preshared_key" placeholder="preshared key (opt)" style="width:160px" @change="emit" />
              <el-input v-model="p['allowed_ips']" placeholder="allowed ips" style="width:150px" @change="emit" />
              <el-button size="small" type="danger" text @click="model.peers.splice(i, 1); emit()">X</el-button>
            </div>
            <el-button size="small" @click="(model.peers = model.peers || []).push({ server: '', public_key: '', allowed_ips: '0.0.0.0/0' }); emit()">Add Peer</el-button>
          </div>
        </el-form-item>
      </el-form>
    </el-tab-pane>
  </el-tabs>
</template>

<script setup>
import SingboxOutboundBase from '../common/SingboxOutboundBase.vue'
import { generateKey } from '../utils/singboxDefaults'
defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
</script>