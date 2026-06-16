<template>
  <el-form label-width="140px" label-position="left">
    <el-form-item label="Congestion Control">
      <el-select v-model="form" style="width:100%" @change="emitUpdate">
        <el-option label="BBR" value="bbr" /><el-option label="Cubic" value="cubic" /><el-option label="New Reno" value="new_reno" />
      </el-select>
    </el-form-item>
  </el-form>
</template>
<script setup>
import { ref, watch, onMounted } from 'vue'
const props = defineProps({ data: String })
const emit = defineEmits(['update'])
const form = ref('bbr')
function syncFromData() { if (props.data) form.value = props.data }
function emitUpdate() { emit('update', form.value) }
watch(() => props.data, syncFromData)
watch(form, emitUpdate)
onMounted(syncFromData)
</script>
