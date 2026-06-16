<template>
  <el-form label-width="160px" label-position="left">
    <el-form-item label="Subject Selector">
      <el-input v-model="selectorStr" placeholder="Comma-separated outbound tags" @change="emit" />
    </el-form-item>
    <el-divider content-position="left">Ping Config</el-divider>
    <el-form-item label="Destination URL">
      <el-input v-model="form.pingConfig.destination" placeholder="http://www.google.com/generate_204" @change="emit" />
    </el-form-item>
    <el-form-item label="Connectivity">
      <el-input v-model="form.pingConfig.connectivity" @change="emit" />
    </el-form-item>
    <el-form-item label="Interval">
      <el-input v-model="form.pingConfig.interval" placeholder="120s" @change="emit" />
    </el-form-item>
    <el-form-item label="Sampling">
      <el-input-number v-model="form.pingConfig.sampling" :min="1" @change="emit" />
    </el-form-item>
    <el-form-item label="Timeout">
      <el-input v-model="form.pingConfig.timeout" placeholder="16s" @change="emit" />
    </el-form-item>
    <el-form-item label="HTTP Method">
      <el-select v-model="form.pingConfig.httpMethod" style="width:100%" @change="emit">
        <el-option label="HEAD" value="HEAD" /><el-option label="GET" value="GET" />
      </el-select>
    </el-form-item>
  </el-form>
</template>
<script setup>
import { reactive, ref, onMounted } from 'vue'
const props = defineProps({ data: Object })
const emitFn = defineEmits(['update'])
const form = reactive({ subjectSelector: [], pingConfig: { destination: 'http://www.google.com/generate_204', connectivity: '', interval: '120s', sampling: 5, timeout: '16s', httpMethod: 'HEAD' } })
const selectorStr = ref('')
function sync() {
  if (!props.data) return
  form.subjectSelector = props.data.subjectSelector || []
  selectorStr.value = form.subjectSelector.join(', ')
  if (props.data.pingConfig) Object.assign(form.pingConfig, props.data.pingConfig)
}
function emit() {
  form.subjectSelector = selectorStr.value.split(',').map(s => s.trim()).filter(Boolean)
  emitFn('update', JSON.parse(JSON.stringify(form)))
}
onMounted(sync)
</script>
