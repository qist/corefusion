<template>
  <div>
    <!--
      订阅格式（singbox_json）的配置编辑器：复用协议中心的 section 组件，
      与协议中心编辑的是**同一套字段**（log / dns / route / experimental）。
      - 这里只负责把各 section 的编辑结果合并回 modelValue（config_json）；
      - route 组件在订阅上下文没有 serverId，其出站/入站 tag 下拉退化为内置值，
        且不会写任何节点侧数据（见 SingboxRoute 的 serverId 守卫）。
    -->
    <el-tabs v-model="tab">
      <el-tab-pane :label="t('subfmt.cfgRoute')" name="route">
        <SingboxRoute
          :data="cfg.route"
          core-type="singbox"
          :server-id="serverId"
          :full-config="{}"
          @update="v => setSection('route', v)"
        />
      </el-tab-pane>
      <el-tab-pane :label="t('subfmt.cfgDns')" name="dns">
        <SingboxDns :data="cfg.dns" @update="v => setSection('dns', v)" />
      </el-tab-pane>
      <el-tab-pane :label="t('subfmt.cfgLog')" name="log">
        <SingboxLog :data="cfg.log" @update="v => setSection('log', v)" />
      </el-tab-pane>
      <el-tab-pane :label="t('subfmt.cfgExperimental')" name="experimental">
        <SingboxExperimental :data="cfg.experimental" @update="v => setSection('experimental', v)" />
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useQueryState } from '../../../composables/useQueryState'
import SingboxRoute from './SingboxRoute.vue'
import SingboxDns from './SingboxDns.vue'
import SingboxLog from './SingboxLog.vue'
import SingboxExperimental from './SingboxExperimental.vue'

const { t } = useI18n()
const props = defineProps({
  modelValue: { type: Object, default: () => ({}) },
  // 订阅格式是全局的，没有节点上下文；保留该 prop 以便复用协议中心同一组件。
  serverId: { type: Number, default: 0 },
})
const emit = defineEmits(['update:modelValue'])

const tab = useQueryState('sbcfgTab', 'route')
const cfg = computed(() => props.modelValue || {})

function setSection(key, val) {
  emit('update:modelValue', { ...(props.modelValue || {}), [key]: val })
}
</script>
