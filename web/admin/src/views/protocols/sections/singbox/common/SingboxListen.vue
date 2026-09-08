<template>
  <el-form label-width="140px" label-position="left">
    <el-form-item label="Tag">
      <el-input v-model="model.tag" placeholder="vless-in" @change="emit" />
    </el-form-item>
    <el-form-item label="Listen">
      <el-input v-model="model.listen" placeholder="0.0.0.0" @change="emit" />
    </el-form-item>
    <el-form-item label="Listen Port">
      <el-input-number v-model="model.listen_port" :min="0" :max="65535" :disabled="model.type==='tun'" style="width:100%" @change="emit" />
    </el-form-item>
    <el-form-item label="Domain Strategy">
      <el-select v-model="model.domain_strategy" style="width:100%" clearable @change="emit">
        <el-option label="" value="" />
        <el-option label="prefer_ipv4" value="prefer_ipv4" />
        <el-option label="prefer_ipv6" value="prefer_ipv6" />
        <el-option label="ipv4_only" value="ipv4_only" />
        <el-option label="ipv6_only" value="ipv6_only" />
      </el-select>
    </el-form-item>
    <el-form-item label="Sniff">
      <el-switch v-model="model.sniff" @change="emit" />
    </el-form-item>
    <el-form-item v-if="model.sniff" label="Sniff Override">
      <el-switch v-model="model.sniff_override_destination" @change="emit" />
    </el-form-item>
    <!-- TUN specific -->
    <template v-if="model.type === 'tun'">
      <el-form-item label="Interface">
        <el-input v-model="model.interface_name" placeholder="tun0" @change="emit" />
      </el-form-item>
      <el-form-item label="Stack">
        <el-select v-model="model.stack" style="width:100%" @change="emit">
          <el-option label="system" value="system" />
          <el-option label="gvisor" value="gvisor" />
          <el-option label="mixed" value="mixed" />
        </el-select>
      </el-form-item>
      <el-form-item label="Auto Route">
        <el-switch v-model="model.auto_route" @change="emit" />
      </el-form-item>
      <el-form-item label="Strict Route">
        <el-switch v-model="model.strict_route" @change="emit" />
      </el-form-item>
    </template>
  </el-form>
</template>

<script setup>
// 通用监听字段组件（tag/listen/port/sniff/domain_strategy/tun）
// model: 父组件的入站 form 响应式对象
defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
</script>