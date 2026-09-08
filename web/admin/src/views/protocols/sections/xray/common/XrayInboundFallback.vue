<template>
  <div>
    <div style="margin-bottom:16px;display:flex;align-items:center;gap:12px">
      <span style="font-weight:500">{{ t('protoCenter.fallback.enable') }}</span>
      <el-switch v-model="rf._fallbackEnabled" />
      <span v-if="!rf._fallbackEnabled" style="font-size:12px;color:#909399">{{ t('protoCenter.fallback.enableHint') }}</span>
    </div>
    <template v-if="rf._fallbackEnabled">
      <div class="hint" style="margin-bottom:12px">{{ t('protoCenter.fallback.desc') }}</div>
      <div v-for="(fb, i) in rf.fallbacks" :key="i" style="border:1px solid #e4e7ed;border-radius:6px;padding:12px;margin-bottom:12px">
        <div style="display:flex;align-items:center;justify-content:space-between;margin-bottom:8px">
          <span style="font-weight:500;font-size:13px">{{ t('protoCenter.fallback.entry') }} {{ i+1 }}</span>
          <el-button size="small" type="danger" text @click="rf.fallbacks.splice(i,1)">X</el-button>
        </div>
        <el-form label-width="140px" label-position="left" size="small">
          <el-form-item :label="t('protoCenter.fallback.name')">
            <el-input v-model="fb.name" :placeholder="t('protoCenter.fallback.nameHint')" @change="emit" />
            <div class="hint">{{ t('protoCenter.fallback.nameDesc') }}</div>
          </el-form-item>
          <el-form-item :label="t('protoCenter.fallback.alpn')">
            <el-select v-model="fb._alpn" style="width:100%" clearable @change="emit">
              <el-option label="(any)" value="" />
              <el-option label="h2" value="h2" /><el-option label="http/1.1" value="http/1.1" /><el-option label="h3" value="h3" />
            </el-select>
            <div class="hint">{{ t('protoCenter.fallback.alpnDesc') }}</div>
          </el-form-item>
          <el-form-item :label="t('protoCenter.fallback.path')">
            <el-input v-model="fb.path" placeholder="/path" @change="emit" />
            <div class="hint">{{ t('protoCenter.fallback.pathDesc') }}</div>
          </el-form-item>
          <el-form-item :label="t('protoCenter.fallback.dest')">
            <el-input v-model="fb.dest" placeholder="80 / addr:port / @vless-grpc" @change="emit" />
            <div class="hint">{{ t('protoCenter.fallback.destDesc') }}</div>
          </el-form-item>
          <el-form-item :label="t('protoCenter.fallback.xver')">
            <el-select v-model="fb.xver" style="width:100%" @change="emit">
              <el-option label="0 (disabled)" :value="0" />
              <el-option label="1" :value="1" />
              <el-option label="2" :value="2" />
            </el-select>
            <div class="hint">{{ t('protoCenter.fallback.xverDesc') }}</div>
          </el-form-item>
        </el-form>
      </div>
      <el-button size="small" @click="add">+ {{ t('protoCenter.fallback.add') }}</el-button>
    </template>
  </div>
</template>

<script setup>
import { useI18n } from 'vue-i18n'
const props = defineProps({ rf: { type: Object, required: true } })
const { t } = useI18n()
function emit() {}
function add() {
  props.rf.fallbacks.push({ name: '', _alpn: '', path: '', dest: 80, xver: 0 })
}
</script>