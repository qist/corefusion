<template>
  <div>
    <el-alert type="info" :closable="false" show-icon style="margin-bottom:12px">
      <div style="font-size:12px;line-height:1.7">
        sing-box 的 NTP 客户端（顶层 <code>ntp</code> 段）。TLS/证书校验依赖系统时间，节点时钟漂移会导致握手失败。<br>
        文档：<a href="https://sing-box.sagernet.org/configuration/ntp/" target="_blank">sing-box / NTP</a>
      </div>
    </el-alert>

    <el-form label-width="160px" label-position="left">
      <el-form-item label="Enabled">
        <el-switch v-model="form.enabled" @change="emit" />
        <span class="hint-inline">关闭时该段落输出空对象 {{ '{}' }}</span>
      </el-form-item>
      <template v-if="form.enabled">
        <el-form-item label="Server">
          <el-input v-model="form.server" placeholder="time.apple.com / ntp.aliyun.com" @change="emit" />
        </el-form-item>
        <el-form-item label="Server Port">
          <el-input-number v-model="form.server_port" :min="0" :max="65535" style="width:100%" @change="emit" />
          <div class="hint">留 0 使用默认 123</div>
        </el-form-item>
        <el-form-item label="Interval">
          <el-input v-model="form.interval" placeholder="30m" @change="emit" />
          <div class="hint">如 30m / 1h</div>
        </el-form-item>

        <el-divider content-position="left">Dial Fields</el-divider>
        <el-form-item label="Detour">
          <el-input v-model="form.detour" placeholder="direct" @change="emit" />
        </el-form-item>
        <el-form-item label="Bind Interface">
          <el-input v-model="form.bind_interface" placeholder="eth0" @change="emit" />
        </el-form-item>
        <el-form-item label="Routing Mark">
          <el-input-number v-model="form.routing_mark" :min="0" style="width:100%" @change="emit" />
          <div class="hint">仅 Linux</div>
        </el-form-item>
      </template>
    </el-form>
  </div>
</template>
<script setup>
// sing-box 顶层 ntp 段编辑器（NTPOptions + DialerOptions）
import { reactive, watch, onMounted } from 'vue'
const props = defineProps({ data: Object, coreType: String, serverId: Number })
const emitFn = defineEmits(['update'])

const form = reactive({
  enabled: false, server: '', server_port: 0, interval: '',
  detour: '', bind_interface: '', routing_mark: 0,
})

function sync() {
  const d = props.data || {}
  form.enabled = d.enabled === true || Object.keys(d).length > 1
  form.server = d.server || ''
  form.server_port = d.server_port || 0
  form.interval = d.interval || ''
  form.detour = d.detour || ''
  form.bind_interface = d.bind_interface || ''
  form.routing_mark = d.routing_mark || 0
}

function build() {
  if (!form.enabled) return {}
  const out = {}
  if (form.server) out.server = form.server
  if (form.server_port) out.server_port = form.server_port
  if (form.interval) out.interval = form.interval
  if (form.detour) out.detour = form.detour
  if (form.bind_interface) out.bind_interface = form.bind_interface
  if (form.routing_mark) out.routing_mark = form.routing_mark
  // 结构体模式下显式声明 enabled，避免被后端 CleanEmpty 当作空段丢弃
  out.enabled = true
  return out
}

function emit() { emitFn('update', build()) }
watch(() => props.data, sync)
onMounted(sync)
</script>
<style scoped>
.hint { font-size: 12px; color: var(--el-text-color-secondary); margin-top: 2px; }
.hint-inline { font-size: 12px; color: var(--el-text-color-secondary); margin-left: 8px; }
</style>
