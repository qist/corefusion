<template>
  <div>
    <!-- Clash API（客户端侧统计/面板通道；节点侧仅第三方面板可选） -->
    <el-divider content-position="left">Clash API</el-divider>
    <el-form label-width="160px" label-position="left">
      <el-form-item label="Enabled">
        <el-switch v-model="clashApi.enabled" @change="onClashToggle" />
        <div class="hint" v-if="isNodeContext">默认关闭。节点流量统计由 v2ray_api 负责；clash_api 仅供第三方面板（yacd / metacubexd）连接</div>
        <div class="hint" v-else>客户端侧的控制与流量展示走 Clash API（yacd / metacubexd 等第三方面板）；不启用则订阅里不带该段</div>
      </el-form-item>
      <template v-if="clashApi.enabled">
        <el-form-item label="External Controller">
          <el-input v-model="clashApi.external_controller" placeholder="127.0.0.1:9090" @change="emit" />
          <div class="hint" v-if="isNodeContext">Clash API listening address（不要与 v2ray_api 的 10086 冲突）</div>
          <div class="hint" v-else>Clash API listening address（客户端本机，通常是 127.0.0.1:9090）</div>
        </el-form-item>
        <el-form-item label="Secret">
          <el-input v-model="clashApi.secret" placeholder="Optional auth secret" @change="emit" />
        </el-form-item>
        <el-form-item label="Default Mode">
          <el-select v-model="clashApi.default_mode" style="width:100%" @change="emit">
            <el-option label="rule" value="rule" />
            <el-option label="global" value="global" />
            <el-option label="direct" value="direct" />
          </el-select>
        </el-form-item>
        <el-form-item label="External UI">
          <el-input v-model="clashApi.external_ui" placeholder="/path/to/ui" @change="emit" />
          <div class="hint">Path to Clash dashboard static files</div>
        </el-form-item>
        <el-form-item label="External UI URL">
          <el-input v-model="clashApi.external_ui_url" placeholder="https://github.com/..." @change="emit" />
          <div class="hint">Download URL for Clash dashboard</div>
        </el-form-item>
        <el-form-item label="Cache ID">
          <el-input v-model="clashApi.cache_id" placeholder="Optional cache identifier" @change="emit" />
        </el-form-item>
      </template>
    </el-form>

    <!-- Cache File -->
    <el-divider content-position="left">Cache File</el-divider>
    <el-form label-width="160px" label-position="left">
      <el-form-item label="Enabled">
        <el-switch v-model="cacheFile.enabled" @change="emit" />
      </el-form-item>
      <el-form-item v-if="cacheFile.enabled" label="Path">
        <el-input v-model="cacheFile.path" placeholder="cache.db" @change="emit" />
      </el-form-item>
      <el-form-item v-if="cacheFile.enabled" label="Store FakeIP">
        <el-switch v-model="cacheFile.store_fakeip" @change="emit" />
      </el-form-item>
      <el-form-item v-if="cacheFile.enabled" label="Store DNS">
        <el-switch v-model="cacheFile.store_dns" @change="emit" />
        <div class="hint">
          DNS 缓存也落盘（重启后保留）。旧版的 <code>store_rdrc</code> / <code>rdrc_timeout</code> 已被它取代 ——
          带着旧字段核心会**直接拒绝启动**，所以面板读到旧字段时会自动迁移到这里、不再回写旧字段。
        </div>
      </el-form-item>
      <el-form-item v-if="cacheFile.enabled" label="Flush Interval">
        <el-input v-model="cacheFile.flush_interval" placeholder="1m" @change="emit" />
        <div class="hint">刷盘间隔（duration，如 1m / 30s）；留空用核心默认</div>
      </el-form-item>
    </el-form>

    <!--
      v2ray API（面板流量统计通道）只在**节点上下文**（协议中心）出现。
      客户端订阅里不能带：它是服务端给面板抓 user>>> 计数的通道，且需要
      with_v2ray_api 的 sing-box 构建，官方客户端装了会直接启动失败。
      客户端侧的控制/流量展示走 Clash API（见上）。
    -->
    <template v-if="isNodeContext">
      <el-divider content-position="left">v2ray API（流量统计）</el-divider>
      <el-form label-width="160px" label-position="left">
        <el-form-item label="Enabled">
          <el-tag type="success" size="small">always on</el-tag>
          <div class="hint">
            面板按订阅用户统计流量依赖此项（与 xray 的 StatsService 协议一致），下发配置时会自动启用。<br />
            注意：节点必须使用带 <code>with_v2ray_api</code> 的 sing-box 构建（官方 release 不含该 tag，
            缺失时核心启动即失败），请安装 <code>qist/singbox-binaries</code> 的产物。
          </div>
        </el-form-item>
        <el-form-item label="Listen">
          <el-input v-model="v2rayApi.listen" placeholder="127.0.0.1:10086" @change="emit" />
          <div class="hint">gRPC API listening address</div>
        </el-form-item>
        <el-form-item label="Stats Service">
          <el-switch v-model="v2rayApi.stats" @change="emit" />
          <div class="hint">Enable stats service for traffic monitoring（用户/入站/出站白名单由面板自动补齐）</div>
        </el-form-item>
      </el-form>
    </template>
  </div>
</template>
<script setup>
import { reactive, computed, onMounted } from 'vue'
const props = defineProps({
  data: Object,
  // 协议中心会传节点 id；订阅格式编辑（全局模板）不传 → 无节点上下文
  serverId: { type: Number, default: 0 },
})
const isNodeContext = computed(() => !!props.serverId)
const emitFn = defineEmits(['update'])
const clashApi = reactive({ enabled: false, external_controller: '', secret: '', default_mode: 'rule', external_ui: '', external_ui_url: '', cache_id: '' })
const cacheFile = reactive({ enabled: false, path: '', store_fakeip: false, store_dns: false, flush_interval: '',
  // 旧配置里出现过 store_rdrc / rdrc_timeout（1.14 弃用，核心会 FATAL 拒绝启动）→ 读到时自动迁移到 store_dns
  _migratedFromRdrc: false })
const v2rayApi = reactive({ listen: '127.0.0.1:10086', stats: true })

const DEFAULT_V2RAY_LISTEN = '127.0.0.1:10086'

function sync() {
  if (!props.data) return
  const ca = props.data.clash_api || {}
  // clash_api 默认不配置：只有显式填了监听地址才视为启用
  clashApi.enabled = !!ca.external_controller
  clashApi.external_controller = ca.external_controller || ''
  clashApi.secret = ca.secret || ''
  clashApi.default_mode = ca.default_mode || 'rule'
  clashApi.external_ui = ca.external_ui || ''
  clashApi.external_ui_url = ca.external_ui_url || ''
  clashApi.cache_id = ca.cache_id || ''
  const cf = props.data.cache_file || {}
  cacheFile.enabled = cf.enabled !== undefined ? cf.enabled : false
  cacheFile.path = cf.path || ''
  cacheFile.store_fakeip = cf.store_fakeip || false
  cacheFile.store_dns = cf.store_dns || false
  cacheFile.flush_interval = cf.flush_interval || ''
  // 旧字段迁移：store_rdrc / rdrc_timeout 在 sing-box 1.14 弃用，**带着它们核心会直接 FATAL**
  // （实测：`FATAL to continuing using this feature, set environment variable ENABLE_DEPRECATED_STORE_RDRC=true`）
  // 所以这里迁移成 store_dns，并且**绝不回写**旧字段（回写等于把节点写挂）。
  cacheFile._migratedFromRdrc = !cf.store_dns && !!cf.store_rdrc
  if (cacheFile._migratedFromRdrc) cacheFile.store_dns = true
  const va = props.data.v2ray_api || {}
  v2rayApi.listen = va.listen || DEFAULT_V2RAY_LISTEN
  v2rayApi.stats = va.stats?.enabled !== false
}

function build() {
  const out = {}
  // clash_api：默认不配置；仅当用户显式启用并填了监听地址时才写入
  if (clashApi.enabled && clashApi.external_controller) {
    out.clash_api = { external_controller: clashApi.external_controller, default_mode: clashApi.default_mode }
    if (clashApi.secret) out.clash_api.secret = clashApi.secret
    if (clashApi.external_ui) out.clash_api.external_ui = clashApi.external_ui
    if (clashApi.external_ui_url) out.clash_api.external_ui_url = clashApi.external_ui_url
    if (clashApi.cache_id) out.clash_api.cache_id = clashApi.cache_id
  }
  if (cacheFile.enabled) {
    out.cache_file = { enabled: true }
    if (cacheFile.path) out.cache_file.path = cacheFile.path
    if (cacheFile.store_fakeip) out.cache_file.store_fakeip = true
    if (cacheFile.store_dns) out.cache_file.store_dns = true
    // flush_interval 必须合法 duration，否则核心启动直接 FATAL
    // （实测：`FATAL decode config: experimental.cache_file.flush_interval: time: invalid duration "abc"`）
    if (cacheFile.flush_interval) {
      if (/^(\d+(\.\d+)?(ns|us|ms|s|m|h))+$/.test(cacheFile.flush_interval.trim())) {
        out.cache_file.flush_interval = cacheFile.flush_interval.trim()
      } else {
        console.warn('flush_interval 不是合法 duration（如 1m / 30s），已忽略: ' + cacheFile.flush_interval)
      }
    }
  }
  // v2ray_api 只在节点上下文写入（面板计费依赖；后端也会自动补齐用户/入站/出站白名单）。
  // 客户端订阅绝不能带：需要 with_v2ray_api 构建，官方客户端会直接启动失败。
  if (isNodeContext.value) {
    out.v2ray_api = { listen: v2rayApi.listen || DEFAULT_V2RAY_LISTEN }
    out.v2ray_api.stats = { enabled: v2rayApi.stats !== false }
  }
  return out
}

function emit() { emitFn('update', build()) }

// 打开开关时补一个默认监听地址：build() 只在填了地址时才写入 clash_api，
// 否则用户开了开关却什么都没保存（客户端/节点本机通常都是 127.0.0.1:9090）。
function onClashToggle() {
  if (clashApi.enabled && !clashApi.external_controller) {
    clashApi.external_controller = '127.0.0.1:9090'
  }
  emit()
}
onMounted(sync)
</script>
<style scoped>.hint { font-size: 12px; color: var(--el-text-color-secondary); margin-top: 2px; }</style>
