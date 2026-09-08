<template>
  <el-form label-width="140px" label-position="left">
    <el-form-item label="Allowed Network">
      <el-select v-model="rf.settings.allowedNetwork" style="width:100%">
        <el-option label="tcp" value="tcp" /><el-option label="udp" value="udp" /><el-option label="tcp,udp" value="tcp,udp" />
      </el-select>
    </el-form-item>
    <el-form-item label="Rewrite Address">
      <el-input v-model="rf.settings.rewriteAddress" placeholder="8.8.8.8 or domain.com" />
      <div class="hint">{{ t('protoCenter.transport.tunnelDest') }}</div>
    </el-form-item>
    <el-form-item label="Rewrite Port">
      <el-input-number v-model="rf.settings.rewritePort" :min="0" :max="65535" />
      <div class="hint">{{ t('protoCenter.transport.tunnelPort') }}</div>
    </el-form-item>
    <el-form-item label="Follow Redirect">
      <el-switch v-model="rf.settings.followRedirect" />
      <div class="hint">{{ t('protoCenter.transport.tunnelFollowRedirect') }}</div>
    </el-form-item>
    <el-form-item label="User Level">
      <el-input-number v-model="rf.settings.userLevel" :min="0" />
    </el-form-item>
    <el-form-item label="Port Map">
      <div v-for="(m, i) in rf._portMap" :key="i" style="display:flex;gap:8px;margin-bottom:4px">
        <el-input v-model="m.local" placeholder="5555" style="width:30%" />
        <el-input v-model="m.remote" placeholder="1.1.1.1:7777" style="width:50%" />
        <el-button size="small" type="danger" text @click="rf._portMap.splice(i,1)">X</el-button>
      </div>
      <el-button size="small" @click="rf._portMap.push({local:'',remote:''})">+ Add Mapping</el-button>
      <div class="hint">{{ t('protoCenter.transport.tunnelPortMap') }}</div>
    </el-form-item>
  </el-form>
</template>

<script setup>
import { useI18n } from 'vue-i18n'
defineProps({ rf: { type: Object, required: true }, serverId: Number, coreType: String })
const { t } = useI18n()
</script>