<template>
  <el-form label-width="150px" label-position="left">
    <el-form-item label="Tag">
      <el-input v-model="model.tag" placeholder="outbound-tag" @change="emit" />
    </el-form-item>
    <template v-if="isProxy">
      <el-form-item label="Server">
        <el-input v-model="model.server" placeholder="example.com" @change="emit" />
      </el-form-item>
      <el-form-item label="Server Port">
        <el-input-number v-model="model.server_port" :min="1" :max="65535" style="width:100%" @change="emit" />
      </el-form-item>
    </template>
    <!-- Dial Fields -->
    <template v-if="isProxy">
      <el-divider content-position="left">Dial</el-divider>
      <el-form-item label="Bind Interface">
        <el-input v-model="model.bind_interface" placeholder="eth0" @change="emit" />
      </el-form-item>
      <el-form-item label="Routing Mark">
        <el-input-number v-model="model.routing_mark" :min="0" style="width:100%" @change="emit" />
      </el-form-item>
      <el-form-item label="Domain Resolver">
        <el-input v-model="model.domain_resolver" placeholder="dns-server tag" @change="emit" />
      </el-form-item>
    </template>
  </el-form>
</template>

<script setup>
// 出站基础字段（tag / server / port / dial）
defineProps({
  model: { type: Object, required: true },
  isProxy: { type: Boolean, default: true },
})
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
</script>