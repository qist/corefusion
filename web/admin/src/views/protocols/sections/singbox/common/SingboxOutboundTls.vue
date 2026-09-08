<template>
  <el-form label-width="140px" label-position="left">
    <el-form-item label="TLS Enabled">
      <el-switch v-model="model._tlsEnabled" @change="emit" />
    </el-form-item>
    <template v-if="model._tlsEnabled">
      <el-form-item label="Server Name">
        <el-input v-model="model._tlsServerName" placeholder="example.com" @change="emit" />
      </el-form-item>
      <el-form-item label="ALPN">
        <el-select v-model="model._tlsAlpn" style="width:100%" multiple clearable @change="emit">
          <el-option label="h2" value="h2" />
          <el-option label="http/1.1" value="http/1.1" />
          <el-option label="h3" value="h3" />
        </el-select>
      </el-form-item>
      <el-form-item label="Insecure">
        <el-switch v-model="model._tlsInsecure" @change="emit" />
      </el-form-item>
      <el-form-item label="uTLS Fingerprint">
        <el-select v-model="model._tlsFingerprint" style="width:100%" clearable @change="emit">
          <el-option label="" value="" />
          <el-option v-for="fp in utlsFingerprints" :key="fp" :label="fp" :value="fp" />
        </el-select>
      </el-form-item>
      <el-form-item label="ECH Config">
        <el-input v-model="model._tlsEchConfig" type="textarea" :rows="2" placeholder="Base64 ECH config" @change="emit" />
      </el-form-item>
      <el-form-item label="Client Cert Path">
        <el-input v-model="model._tlsClientCertPath" placeholder="/path/to/client.crt" @change="emit" />
      </el-form-item>
      <el-form-item label="Client Key Path">
        <el-input v-model="model._tlsClientKeyPath" placeholder="/path/to/client.key" @change="emit" />
      </el-form-item>
    </template>
  </el-form>
</template>

<script setup>
// 出站（客户端）TLS 组件：uTLS / ECH / Insecure / 客户端证书
defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
const utlsFingerprints = ['chrome', 'firefox', 'edge', 'safari', '360', 'qq', 'ios', 'android', 'random', 'randomized']
</script>