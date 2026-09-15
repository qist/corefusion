<template>
  <el-form label-width="150px" label-position="left">
    <el-form-item :label="t('singbox.obSsh.user')">
      <el-input v-model="model.user" placeholder="root" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obSsh.privateKey')">
      <el-input v-model="model.private_key" type="textarea" :rows="3" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obSsh.privateKeyPath')">
      <el-input v-model="model.private_key_path" placeholder="/root/.ssh/id_ed25519" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obSsh.hostKey')">
      <el-input v-model="model.host_key" type="textarea" :rows="2" :placeholder="t('singbox.obSsh.hostKeyHint')" @change="emit" />
    </el-form-item>
  </el-form>
</template>

<script setup>
// SSH 出站：专属设置(User/PrivateKey/PrivateKeyPath/HostKey)；Base 由容器层提供
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
</script>
