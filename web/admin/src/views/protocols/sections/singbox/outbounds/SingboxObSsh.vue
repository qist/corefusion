<template>
  <el-form label-width="170px" label-position="left">
    <el-form-item :label="t('singbox.obSsh.user')">
      <el-input v-model="model.user" placeholder="root" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obProtocol.password')">
      <el-input v-model="model.password" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obSsh.privateKey')">
      <el-input v-model="model.private_key" type="textarea" :rows="3" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obSsh.privateKeyPath')">
      <el-input v-model="model.private_key_path" placeholder="/root/.ssh/id_ed25519" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obSsh.privateKeyPassphrase')">
      <el-input v-model="model.private_key_passphrase" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obSsh.hostKey')">
      <el-input v-model="model.host_key" type="textarea" :rows="2" :placeholder="t('singbox.obSsh.hostKeyHint')" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obSsh.hostKeyAlgorithms')">
      <el-input v-model="model.host_key_algorithms" :placeholder="t('singbox.obSsh.listHint')" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obSsh.clientVersion')">
      <el-input v-model="model.client_version" placeholder="SSH-2.0-OpenSSH_9.0" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obSsh.cipher')">
      <el-input v-model="model.cipher" :placeholder="t('singbox.obSsh.listHint')" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obSsh.mac')">
      <el-input v-model="model.mac" :placeholder="t('singbox.obSsh.listHint')" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.obSsh.kexAlgorithm')">
      <el-input v-model="model.kex_algorithm" :placeholder="t('singbox.obSsh.listHint')" @change="emit" />
    </el-form-item>
  </el-form>
</template>

<script setup>
// SSH 出站：option/ssh.go SSHOutboundOptions
// 注意 server/server_port 由 Base tab 提供（SSH 已加入 OUTBOUND_SERVER_PROTOCOLS）
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
</script>
