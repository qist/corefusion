<template>
  <el-form label-width="150px" label-position="left">
    <el-form-item label="Tag">
      <el-input v-model="model.tag" placeholder="group-tag" @change="emit" />
    </el-form-item>
    <el-form-item label="Outbounds">
      <el-input v-model="model._outboundsStr" placeholder="Comma-separated tags" @change="emit" />
    </el-form-item>
    <el-form-item v-if="model.type === 'selector'" label="Default">
      <el-input v-model="model.default" placeholder="Default outbound tag" @change="emit" />
    </el-form-item>
    <template v-if="model.type === 'urltest'">
      <el-form-item label="URL">
        <el-input v-model="model.url" placeholder="https://www.gstatic.com/generate_204" @change="emit" />
      </el-form-item>
      <el-form-item label="Interval">
        <el-input v-model="model.interval" placeholder="3m" @change="emit" />
      </el-form-item>
      <el-form-item label="Tolerance">
        <el-input-number v-model="model.tolerance" :min="0" style="width:100%" @change="emit" />
      </el-form-item>
    </template>
    <el-form-item label="Interrupt Conns">
      <el-switch v-model="model.interrupt_exist_connections" @change="emit" />
    </el-form-item>
  </el-form>
</template>

<script setup>
// selector / urltest 出站
defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
</script>