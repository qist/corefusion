<template>
  <div>
    <!--
      订阅格式（xray_json）的配置编辑器：复用协议中心的 section 组件，
      与协议中心编辑的是**同一套字段**（log / dns / routing / policy）。
      - 这里只负责把各 section 的编辑结果合并回 modelValue（config_json）；
      - routing 组件在订阅上下文没有 serverId，其出站/入站 tag 下拉退化为内置值，
        且不会写任何节点侧数据（见 XrayRouting 的 serverId 守卫）。
    -->
    <el-tabs v-model="tab">
      <el-tab-pane :label="t('subfmt.cfgRouting')" name="routing">
        <XrayRouting
          :data="cfg.routing"
          core-type="xray"
          :server-id="serverId"
          @update="v => setSection('routing', v)"
        />
      </el-tab-pane>
      <el-tab-pane :label="t('subfmt.cfgDns')" name="dns">
        <XrayDns :data="cfg.dns" @update="v => setSection('dns', v)" />
      </el-tab-pane>
      <el-tab-pane :label="t('subfmt.cfgLog')" name="log">
        <XrayLog :data="cfg.log" @update="v => setSection('log', v)" />
      </el-tab-pane>
      <el-tab-pane :label="t('subfmt.cfgPolicy')" name="policy">
        <XrayPolicy :data="cfg.policy" @update="v => setSection('policy', v)" />
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useQueryState } from '../../../composables/useQueryState'
import XrayRouting from './XrayRouting.vue'
import XrayDns from './XrayDns.vue'
import XrayLog from './XrayLog.vue'
import XrayPolicy from './XrayPolicy.vue'

const { t } = useI18n()
const props = defineProps({
  modelValue: { type: Object, default: () => ({}) },
  // 订阅格式是全局的，没有节点上下文；保留该 prop 以便复用协议中心同一组件。
  serverId: { type: Number, default: 0 },
})
const emit = defineEmits(['update:modelValue'])

const tab = useQueryState('xrcfgTab', 'routing')
const cfg = computed(() => props.modelValue || {})

function setSection(key, val) {
  emit('update:modelValue', { ...(props.modelValue || {}), [key]: val })
}
</script>
