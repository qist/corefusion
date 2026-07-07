<template>
  <div>
    <h2 style="margin-bottom:20px">{{ t('service.title') }}</h2>
    <el-tabs v-model="activeTab">
      <el-tab-pane :label="t('service.tabs.shareDomain')" name="shareDomain">
        <ShareDomainTab />
      </el-tab-pane>
      <el-tab-pane :label="t('service.tabs.cfip')" name="cfip">
        <CFIPTab />
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import ShareDomainTab from './services/ShareDomainTab.vue'
import CFIPTab from './services/CFIPTab.vue'

const { t } = useI18n()
const activeTab = ref('shareDomain')
</script>
