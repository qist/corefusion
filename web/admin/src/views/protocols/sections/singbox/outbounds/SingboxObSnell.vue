<template>
  <el-form label-width="150px" label-position="left">
    <el-form-item :label="t('singbox.obProtocol.version')">
      <el-select v-model="model.version" style="width:100%" @change="emit">
        <el-option label="4" :value="4" />
        <el-option label="6" :value="6" />
      </el-select>
      <div class="hint">{{ t('singbox.obProtocol.snellVersionHint') }}</div>
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.psk')">
      <div style="display:flex;gap:8px;flex:1">
        <el-input v-model="model.psk" @change="emit" />
        <el-button size="small" @click="model.psk = generateKey(); emit()">{{ t('singbox.users.generate') }}</el-button>
      </div>
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.userkey')">
      <el-input v-model="model.userkey" :placeholder="t('singbox.obProtocol.userkeyHint')" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.reuse')">
      <el-switch v-model="model.reuse" @change="emit" />
    </el-form-item>

    <template v-if="model.version === 4">
      <el-divider content-position="left">v4 {{ t('singbox.obProtocol.obfs') }}</el-divider>
      <el-form-item :label="t('singbox.obProtocol.obfsMode')">
        <el-select v-model="model.obfs" style="width:100%" clearable @change="emit">
          <el-option label="" value="" />
          <el-option label="none" value="none" />
          <el-option label="http" value="http" />
          <el-option label="tls" value="tls" />
        </el-select>
      </el-form-item>
      <el-form-item :label="t('singbox.obProtocol.obfsHost')">
        <el-input v-model="model.obfs_host" placeholder="example.com" @change="emit" />
      </el-form-item>
    </template>

    <template v-if="model.version === 6">
      <el-divider content-position="left">v6 {{ t('singbox.obProtocol.mode') }}</el-divider>
      <el-form-item :label="t('singbox.obProtocol.mode')">
        <el-select v-model="model._snellMode" style="width:100%" clearable @change="emit">
          <el-option label="" value="" />
          <el-option label="default" value="default" />
          <el-option label="unshaped" value="unshaped" />
          <el-option label="unsafe-raw" value="unsafe-raw" />
        </el-select>
      </el-form-item>
    </template>
  </el-form>
</template>

<script setup>
// Snell 出站：version 只接受 4 / 6（5 是入站版本）
import { useI18n } from 'vue-i18n'
import { generateKey } from '../utils/singboxDefaults'
const { t } = useI18n()
defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
</script>

<style scoped>
.hint { color: #909399; font-size: 12px; margin-top: 4px; }
</style>
