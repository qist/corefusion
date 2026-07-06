<template>
  <div style="padding:20px">
    <!-- Full proxy management (Caddy/Nginx) for all nodes — local and remote.
         The backend handlers detect the server_id from the URL and route
         operations to the local manager or remote Agent via gRPC. -->
    <Proxy :server-id="serverId" :is-local="isLocal" />
  </div>
</template>

<script setup>
import Proxy from './Proxy.vue'

const props = defineProps({
  serverId: { type: Number, default: 0 },
  isLocal: { type: Boolean, default: false }
})
</script>
