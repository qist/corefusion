<template>
  <el-form label-width="150px" label-position="left">
    <el-form-item label="Tag">
      <el-input v-model="model.tag" placeholder="direct-or-block-tag" @change="emit" />
    </el-form-item>
  </el-form>
</template>

<script setup>
// direct / block / dns 简单出站：仅需 tag
defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
</script>