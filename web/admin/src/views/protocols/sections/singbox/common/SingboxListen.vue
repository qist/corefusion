<template>
  <el-form label-width="140px" label-position="left">
    <el-form-item :label="t('singbox.listen.tag')">
      <el-input v-model="model.tag" :placeholder="t('singbox.listen.tagPlaceholder')" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.listen.listen')">
      <el-input v-model="model.listen" :placeholder="t('singbox.listen.listenPlaceholder')" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.listen.listenPort')">
      <el-input-number v-model="model.listen_port" :min="0" :max="65535" :disabled="model.type==='tun'" style="width:100%" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.listen.domainStrategy')">
      <el-select v-model="model.domain_strategy" style="width:100%" clearable @change="emit">
        <el-option label="" value="" />
        <el-option label="prefer_ipv4" value="prefer_ipv4" />
        <el-option label="prefer_ipv6" value="prefer_ipv6" />
        <el-option label="ipv4_only" value="ipv4_only" />
        <el-option label="ipv6_only" value="ipv6_only" />
      </el-select>
    </el-form-item>
    <el-form-item :label="t('singbox.listen.sniff')">
      <el-switch v-model="model.sniff" @change="emit" />
    </el-form-item>
    <el-form-item v-if="model.sniff" :label="t('singbox.listen.sniffOverride')">
      <el-switch v-model="model.sniff_override_destination" @change="emit" />
    </el-form-item>
    <!-- TUN specific -->
    <template v-if="model.type === 'tun'">
      <el-form-item :label="t('singbox.listen.interface')">
        <el-input v-model="model.interface_name" :placeholder="t('singbox.listen.interfacePlaceholder')" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.listen.stack')">
        <el-select v-model="model.stack" style="width:100%" @change="emit">
          <el-option label="system" value="system" />
          <el-option label="gvisor" value="gvisor" />
          <el-option label="mixed" value="mixed" />
        </el-select>
      </el-form-item>
      <el-form-item :label="t('singbox.listen.autoRoute')">
        <el-switch v-model="model.auto_route" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.listen.strictRoute')">
        <el-switch v-model="model.strict_route" @change="emit" />
      </el-form-item>
    </template>
  </el-form>
</template>

<script setup>
// 通用监听字段组件（tag/listen/port/sniff/domain_strategy/tun）
// model: 父组件的入站 form 响应式对象
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
</script>