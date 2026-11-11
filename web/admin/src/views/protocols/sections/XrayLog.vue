<template>
  <div>
    <el-divider content-position="left">{{ t('protoCenter.log.accessLog') }}</el-divider>
    <el-form label-width="140px" label-position="left">
      <el-form-item :label="t('protoCenter.log.outputMode')">
        <el-radio-group v-model="accessMode" @change="onAccessModeChange">
          <el-radio value="file">{{ t('protoCenter.log.file') }}</el-radio>
          <el-radio value="none">{{ t('protoCenter.log.disable') }}</el-radio>
        </el-radio-group>
      </el-form-item>
      <el-form-item v-if="accessMode === 'file'" :label="t('protoCenter.log.filePath')">
        <el-input v-model="form.access" placeholder="/var/log/xray/access.log" @change="emit" />
      </el-form-item>
    </el-form>

    <el-divider content-position="left">{{ t('protoCenter.log.errorLog') }}</el-divider>
    <el-form label-width="140px" label-position="left">
      <el-form-item :label="t('protoCenter.log.outputMode')">
        <el-radio-group v-model="errorMode" @change="onErrorModeChange">
          <el-radio value="file">{{ t('protoCenter.log.file') }}</el-radio>
          <el-radio value="none">{{ t('protoCenter.log.disable') }}</el-radio>
        </el-radio-group>
      </el-form-item>
      <el-form-item v-if="errorMode === 'file'" :label="t('protoCenter.log.filePath')">
        <el-input v-model="form.error" placeholder="/var/log/xray/error.log" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('protoCenter.log.logLevel')">
        <el-select v-model="form.loglevel" style="width:100%" @change="emit">
          <el-option label="debug" value="debug" />
          <el-option label="info" value="info" />
          <el-option label="warning" value="warning" />
          <el-option label="error" value="error" />
          <el-option label="none" value="none" />
        </el-select>
      </el-form-item>
    </el-form>

    <el-divider content-position="left">{{ t('protoCenter.log.options') }}</el-divider>
    <el-form label-width="140px" label-position="left">
      <el-form-item :label="t('protoCenter.log.dnsLog')">
        <el-switch v-model="form.dnsLog" @change="emit" />
        <span class="hint">{{ t('protoCenter.log.dnsLogHint') }}</span>
      </el-form-item>
      <el-form-item :label="t('protoCenter.log.timestamp')">
        <el-switch v-model="form.timestamp" @change="emit" />
        <span class="hint">{{ t('protoCenter.log.timestampHint') }}</span>
      </el-form-item>
      <el-form-item :label="t('protoCenter.log.maskAddress')">
        <el-select v-model="form.maskAddress" style="width:100%" clearable @change="emit">
          <el-option :label="t('protoCenter.log.disabled')" value="" />
          <el-option :label="t('protoCenter.log.quarter')" value="quarter" />
          <el-option :label="t('protoCenter.log.half')" value="half" />
          <el-option :label="t('protoCenter.log.full')" value="full" />
        </el-select>
        <div class="hint">{{ t('protoCenter.log.maskAddressHint') }}</div>
      </el-form-item>
    </el-form>
  </div>
</template>
<script setup>
import { reactive, ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
const props = defineProps({ data: Object })
const emitFn = defineEmits(['update'])
const form = reactive({ access: '', error: '', loglevel: 'info', dnsLog: false, timestamp: true, maskAddress: '' })
const accessMode = ref('none')
const errorMode = ref('none')

function sync() {
  if (!props.data) return
  form.loglevel = props.data.loglevel || 'info'
  form.dnsLog = props.data.dnsLog || false
  form.timestamp = props.data.timestamp !== false
  form.maskAddress = props.data.maskAddress || ''
  // 面板/Agent 负责日志采集与轮转，标准输出模式已下线：
  // 历史配置里 access/error 为空（xray 的 stdout 语义）统一按文件模式展示，
  // 用户下次保存时即覆盖为真实文件路径。
  if (props.data.access === 'none') {
    accessMode.value = 'none'
    form.access = 'none'
  } else {
    accessMode.value = 'file'
    form.access = props.data.access || 'logs/xray-access.log'
  }
  if (props.data.error === 'none') {
    errorMode.value = 'none'
    form.error = 'none'
  } else {
    errorMode.value = 'file'
    form.error = props.data.error || 'logs/xray-error.log'
  }
}

function onAccessModeChange(val) {
  if (val === 'file') form.access = form.access && form.access !== 'none' ? form.access : 'logs/xray-access.log'
  else form.access = 'none'
  emit()
}

function onErrorModeChange(val) {
  if (val === 'file') form.error = form.error && form.error !== 'none' ? form.error : 'logs/xray-error.log'
  else form.error = 'none'
  emit()
}

function emit() {
  emitFn('update', {
    access: form.access, error: form.error, loglevel: form.loglevel,
    dnsLog: form.dnsLog, timestamp: form.timestamp,
    maskAddress: form.maskAddress || undefined,
  })
}

onMounted(sync)
</script>
<style scoped>.hint { margin-left: 12px; font-size: 12px; color: var(--el-text-color-secondary); }</style>
