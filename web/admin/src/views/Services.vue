<template>
  <div>
    <h2 style="margin-bottom:20px">{{ t('service.title') }}</h2>
    <el-tabs v-model="activeTab" @tab-change="onTabChange">
      <el-tab-pane :label="t('service.tabs.shareDomain')" name="shareDomain">
        <ShareDomainTab />
      </el-tab-pane>
      <el-tab-pane :label="t('service.tabs.cfip')" name="cfip">
        <CFIPTab />
      </el-tab-pane>
      <el-tab-pane :label="t('proxy.wafGlobalRules')" name="waf">
        <WafRulesTab />
      </el-tab-pane>
      <el-tab-pane :label="t('service.tabs.rules')" name="rules">
        <RuleTab />
      </el-tab-pane>
      <el-tab-pane :label="t('externalNodes.title')" name="externalNodes">
        <ExternalNodes />
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useQueryState } from '../composables/useQueryState'
import ShareDomainTab from './services/ShareDomainTab.vue'
import CFIPTab from './services/CFIPTab.vue'
import WafRulesTab from './services/WafRulesTab.vue'
import RuleTab from './services/RuleTab.vue'
import ExternalNodes from './ExternalNodes.vue'
import { notifyCfipRefresh } from '../composables/useAutoRefresh'

const { t } = useI18n()
const activeTab = useQueryState('tab', 'shareDomain')

// 标签页是常驻挂载的：切回来时主动刷新探测结果/池健康，避免看到旧数据。
function onTabChange(name) {
  if (name === 'shareDomain' || name === 'cfip') notifyCfipRefresh()
}
</script>
