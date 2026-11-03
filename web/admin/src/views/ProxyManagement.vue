<template>
  <div style="padding:20px">
    <!-- Full proxy management (Caddy/Nginx) for all nodes — local and remote.
         The backend handlers detect the server_id from the URL and route
         operations to the local manager or remote Agent via gRPC.
         必须拿到有效节点 ID 才渲染：serverId=0 曾在后端被当成"本地节点"，
         会把面板本机的 Caddy/Nginx 状态显示到远程节点页面上。 -->
    <Proxy v-if="serverId > 0" :server-id="serverId" :is-local="isLocal" />
    <el-empty v-else :description="t('common.loading')" />
  </div>
</template>

<script setup>
import Proxy from './Proxy.vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

const props = defineProps({
  serverId: { type: Number, default: 0 },
  isLocal: { type: Boolean, default: false }
})
</script>
