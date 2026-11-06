<template>
  <el-form label-width="140px" label-position="left">
    <el-form-item :label="t('protoCenter.transport.sniffingEnabled')">
      <el-switch v-model="rf.sniffing.enabled" />
    </el-form-item>
    <template v-if="rf.sniffing.enabled">
      <el-form-item :label="t('protoCenter.transport.sniffingDestOverride')">
        <el-checkbox-group v-model="rf.sniffing.destOverride">
          <el-checkbox value="http" label="HTTP" />
          <el-checkbox value="tls" label="TLS" />
          <el-checkbox value="quic" label="QUIC" />
          <el-checkbox value="fakedns" label="FakeDNS" />
        </el-checkbox-group>
        <div class="hint">{{ t('protoCenter.transport.sniffingDestOverrideHint') }}</div>
      </el-form-item>
      <el-form-item :label="t('protoCenter.outbounds.reverseSniffingMetadataOnly')">
        <el-switch v-model="rf.sniffing.metadataOnly" />
        <div class="hint">{{ t('protoCenter.transport.sniffingMetadata') }}</div>
      </el-form-item>
      <el-form-item :label="t('protoCenter.outbounds.reverseSniffingRouteOnly')">
        <el-switch v-model="rf.sniffing.routeOnly" />
        <div class="hint">{{ t('protoCenter.transport.sniffingRouteOnly') }}</div>
      </el-form-item>
      <el-form-item :label="t('protoCenter.outbounds.reverseSniffingDomainsExcluded')">
        <el-input v-model="rf.sniffing._domainsExcludedStr" type="textarea" :rows="2" placeholder="courier.push.apple.com&#10;domain:example.com" @change="emit" />
        <div class="hint">{{ t('protoCenter.transport.sniffingDomainsExcluded') }}</div>
      </el-form-item>
      <el-form-item :label="t('protoCenter.outbounds.reverseSniffingIpsExcluded')">
        <el-input v-model="rf.sniffing._ipsExcludedStr" type="textarea" :rows="2" placeholder="10.0.0.0/8" @change="emit" />
        <div class="hint">{{ t('protoCenter.transport.sniffingIpsExcluded') }}</div>
      </el-form-item>
    </template>
  </el-form>
</template>

<script setup>
import { useI18n } from 'vue-i18n'
const props = defineProps({ rf: { type: Object, required: true } })
const { t } = useI18n()
function emit() {}
</script>

<style scoped>
.hint { width: 100%; font-size: 12px; color: var(--el-text-color-secondary); line-height: 1.4; }
</style>
