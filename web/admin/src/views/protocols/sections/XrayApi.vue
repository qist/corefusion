<template>
  <el-form label-width="140px" label-position="left">
    <el-form-item label="Tag">
      <el-input v-model="form.tag" placeholder="api" @change="emit" />
    </el-form-item>
    <el-form-item label="Listen Address">
      <el-input v-model="form.listen" placeholder="127.0.0.1:10085" @change="emit" />
    </el-form-item>
    <el-form-item label="Services">
      <el-checkbox-group v-model="form.services" @change="emit">
        <el-checkbox label="HandlerService" />
        <el-checkbox label="LoggerService" />
        <el-checkbox label="StatsService" />
        <el-checkbox label="ObservatoryService" />
        <el-checkbox label="RoutingService" />
        <el-checkbox label="BalancerService" />
      </el-checkbox-group>
    </el-form-item>
  </el-form>
</template>
<script setup>
import { reactive, onMounted } from 'vue'
const props = defineProps({ data: Object })
const emitFn = defineEmits(['update'])
const form = reactive({ tag: 'api', listen: '127.0.0.1:10085', services: ['HandlerService', 'LoggerService', 'StatsService'] })
function sync() { if (!props.data) return; form.tag = props.data.tag || ''; form.listen = props.data.listen || ''; form.services = props.data.services || [] }
function emit() { emitFn('update', { ...form }) }
onMounted(sync)
</script>
