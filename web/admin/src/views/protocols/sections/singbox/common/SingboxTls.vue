<template>
  <el-form label-width="140px" label-position="left">
    <!-- Security：none / Standard TLS / Reality，互斥（对齐 xray 下拉模型） -->
    <el-form-item :label="t('singbox.tls.security')">
      <el-select v-model="securityMode" style="width:100%" @change="onSecurityChange">
        <el-option :label="t('singbox.tls.none')" value="none" />
        <el-option :label="t('singbox.tls.standardTls')" value="tls" />
        <el-option v-if="realitySupported" :label="t('singbox.tls.reality')" value="reality" />
      </el-select>
      <div v-if="!realitySupported" class="hint warn-hint">{{ t('singbox.tls.realityProtocolOnly') }}</div>
      <div v-else-if="securityMode === 'reality' && !realityTransportOk" class="hint warn-hint">
        {{ t('singbox.tls.realityTransportOnly', { transport: model._transportType || 'TCP' }) }}
      </div>
      <div v-else class="hint">{{ t('singbox.tls.mutualExclusive') }}</div>
    </el-form-item>

    <!-- Standard TLS（证书模式） -->
    <template v-if="securityMode === 'tls'">
      <el-form-item :label="t('singbox.tls.serverName')">
        <el-input v-model="model._tlsServerName" :placeholder="t('singbox.tls.serverNamePlaceholder')" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.tls.alpn')">
        <el-select v-model="model._tlsAlpn" style="width:100%" multiple clearable @change="emit">
          <el-option label="h2" value="h2" />
          <el-option label="http/1.1" value="http/1.1" />
          <el-option label="h3" value="h3" />
        </el-select>
      </el-form-item>
      <el-form-item :label="t('singbox.tls.minVersion')">
        <el-select v-model="model._tlsMinVersion" style="width:100%" clearable @change="emit">
          <el-option label="" value="" />
          <el-option label="1.0" value="1.0" />
          <el-option label="1.1" value="1.1" />
          <el-option label="1.2" value="1.2" />
          <el-option label="1.3" value="1.3" />
        </el-select>
      </el-form-item>
      <!-- 证书选择：从证书中心选择 -->
      <el-form-item :label="t('singbox.tls.selectCert')">
        <el-select v-model="model._certId" style="width:100%" clearable @change="onCertSelect" :placeholder="t('singbox.tls.selectCertPlaceholder')">
          <el-option v-for="c in certificates" :key="c.id" :label="c.name + ' (' + c.domain + ')'" :value="c.id">
            <span>{{ c.name }}</span>
            <span style="color:#909399;margin-left:8px;font-size:12px">{{ c.domain }}</span>
          </el-option>
        </el-select>
      </el-form-item>
      <el-form-item :label="t('singbox.tls.certPath')">
        <el-input v-model="model._tlsCertPath" :placeholder="t('singbox.tls.certPathPlaceholder')" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.tls.keyPath')">
        <el-input v-model="model._tlsKeyPath" :placeholder="t('singbox.tls.keyPathPlaceholder')" @change="emit" />
      </el-form-item>
    </template>

    <!-- Reality（服务端：仅 handshake + private_key + short_id[] + max_time_difference） -->
    <template v-else-if="securityMode === 'reality'">
      <el-form-item :label="t('singbox.tls.handshakeServer')">
        <div style="display:flex;gap:8px;flex:1">
          <el-select v-model="_realityTarget" style="flex:1" filterable allow-create default-first-option @change="onRealityTargetChange" :placeholder="t('singbox.tls.handshakeServerPlaceholder')">
            <!-- 本地代理 Reality 服务器分组 -->
            <el-option-group v-if="localRealityServers.length > 0" :label="t('singbox.tls.localRealityGroup')">
              <el-option v-for="tgt in ALL_REALITY_TARGETS.filter(t => t.source === 'local')" :key="tgt.target" :label="tgt.target" :value="tgt.target">
                <span style="color:#67c23a">●</span>
                <span style="margin-left:6px">{{ tgt.target }}</span>
                <span style="color:#909399;margin-left:8px;font-size:12px">— {{ t('singbox.tls.serverNamesRandom') }}</span>
              </el-option>
            </el-option-group>
            <!-- 预设公共服务器分组 -->
            <el-option-group :label="t('singbox.tls.presetGroup')">
              <el-option v-for="tgt in ALL_REALITY_TARGETS.filter(t => t.source === 'preset')" :key="tgt.target" :label="tgt.sni + ' (' + tgt.target + ')'" :value="tgt.target">
                <span>{{ tgt.sni }}</span>
                <span style="color:#909399;margin-left:8px;font-size:12px">{{ tgt.target }}</span>
                <el-tag v-if="isPresetSniUsed(tgt.sni)" size="small" type="warning" style="margin-left:8px">{{ t('singbox.tls.sniInUse') }}</el-tag>
              </el-option>
            </el-option-group>
          </el-select>
          <el-button size="small" @click="refreshRealityTarget">{{ t('singbox.tls.refresh') }}</el-button>
        </div>
        <div class="hint">{{ t('singbox.tls.handshakeServerHint') }}</div>
      </el-form-item>
      <el-form-item :label="t('singbox.tls.serverName')">
        <div style="display:flex;gap:8px;flex:1">
          <el-input v-model="model._tlsServerName" :placeholder="t('singbox.tls.serverNamePlaceholder')" @change="onServerNameChange" style="flex:1" />
          <el-button size="small" :title="t('singbox.tls.serverNamesRandomTip')" @click="refreshRandomServerName">{{ t('singbox.tls.refresh') }}</el-button>
        </div>
        <div class="hint">{{ t('singbox.tls.xverLocalBound1') }}</div>
      </el-form-item>
      <el-form-item :label="t('singbox.tls.handshakePort')">
        <el-input-number v-model="model._realityHandshakePort" :min="1" :max="65535" style="width:100%" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.tls.privateKey')">
        <div style="display:flex;gap:8px;flex:1">
          <el-input v-model="model._realityPrivateKey" @change="emit" />
          <el-button size="small" @click="genRealityKey">{{ t('singbox.tls.generate') }}</el-button>
        </div>
      </el-form-item>
      <el-form-item :label="t('singbox.tls.publicKeyClient')">
        <div style="display:flex;gap:8px;flex:1">
          <el-input v-model="model._realityPublicKey" readonly />
          <el-button size="small" @click="copyPublicKey">{{ t('singbox.tls.copy') }}</el-button>
        </div>
        <div class="hint">{{ t('singbox.tls.publicKeyClientHint') }}</div>
      </el-form-item>
      <el-form-item :label="t('singbox.tls.shortIds')">
        <el-input v-model="model._realityShortIds" type="textarea" :rows="2" :placeholder="t('singbox.tls.shortIdsPlaceholder')" @change="emit" />
        <el-button size="small" style="margin-top:4px" @click="genRandomShortId">{{ t('singbox.tls.generateRandomShortIds') }}</el-button>
      </el-form-item>
      <el-form-item :label="t('singbox.tls.maxTimeDiff')">
        <el-input v-model="model._realityMaxTimeDiff" :placeholder="t('singbox.tls.maxTimeDiffPlaceholder')" @change="emit" />
      </el-form-item>
    </template>
  </el-form>
</template>

<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import client from '../../../../../api/client'
import { REALITY_PROTOCOLS, REALITY_TRANSPORTS } from '../utils/singboxDefaults'
import { collectUsedSnis as collectUsedSnisShared, mergeUsedSnis } from '../../utils/sniUsage'

const { t } = useI18n()
const props = defineProps({
  model: { type: Object, required: true },
  serverId: { type: Number, required: true },
  inbounds: { type: Array, default: () => [] },
  // 该节点上所有入站（xray + sing-box）已占用的 Server Names，由后端登记：
  // 编辑器自己只能看到本核心的入站，跨核心占用需要它（后端 /used-server-names）。
  usedServerNames: { type: Array, default: () => [] },
})
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }

// Generate a completely random domain name (for local Reality serverName)
function randomDomain() {
  const syllables = ['ab','ac','ad','af','ag','al','am','an','ap','ar','as','at','av','ba','be','bi','bo','bu','ca','ce','ci','co','da','de','di','do','du','ea','ed','el','em','en','ep','er','es','et','ev','ex','fa','fe','fi','fo','fu','ga','ge','gi','go','gu','ha','he','hi','ho','hu','id','if','in','ir','is','it','ka','ke','ki','ko','la','le','li','lo','lu','ma','me','mi','mo','mu','na','ne','ni','no','nu','ob','od','of','ok','ol','om','on','op','or','os','ot','pa','pe','pi','po','pu','ra','re','ri','ro','ru','sa','se','si','so','su','ta','te','ti','to','tu','ul','um','un','up','ur','us','ut','va','ve','vi','vo','wa','we','wi','wo','xu','ya','ye','yi','yu','za','ze','zi','zo']
  const tlds = ['com','net','org','io','info','dev','app','xyz','me','cc','top','site','online']
  let domain = ''
  const len = 2 + Math.floor(Math.random() * 3)
  for (let i = 0; i < len; i++) domain += syllables[Math.floor(Math.random() * syllables.length)]
  return domain + '.' + tlds[Math.floor(Math.random() * tlds.length)]
}

// Collect already-used SNI from other inbounds
// 本节点「其它入站」已占用的 SNI（共用 utils/sniUsage：按 id 排除自身，
// 口径为同节点互斥、跨节点允许重复）。
function collectUsedSnis() {
  return collectUsedSnisShared(props.inbounds || [], props.model)
}

// 预设列表里的「已使用」标记数据源（本核心内存中的其它入站 + 后端登记的跨核心占用）
function isPresetSniUsed(sni) {
  const sn = String(sni || '').trim().toLowerCase()
  if (!sn) return false
  const map = collectUsedSnisShared(props.inbounds || [], props.model)
  mergeUsedSnis(map, props.usedServerNames, props.model._id ?? null)
  return map.has(sn)
}

// 证书中心证书列表
const certificates = ref([])

// Reality 目标选择
const _realityTarget = ref('')

// 本地代理 Reality 握手服务器（来自 caddy/nginx 代理站点）
const localRealityServers = ref([])

// 所有可用的 Reality 目标（预设 + 本地服务器）
// 预设公共服务器：与 xray 编辑器同源（面板 /system/reality-targets），
// 此前这里硬编码了 6 个，和 xray 侧的 12 个不一致（用户反馈"两个显示的预设不一样"）。
const realityPresets = ref([])
async function loadRealityTargets() {
  try {
    const data = await client.get('/system/reality-targets')
    const items = data?.items || (Array.isArray(data) ? data : [])
    realityPresets.value = items
      .filter(t => t && t.target)
      .map(t => ({ target: t.target, sni: t.sni || String(t.target).split(':')[0], source: 'preset' }))
  } catch { realityPresets.value = [] }
}

const ALL_REALITY_TARGETS = computed(() => {
  const presets = realityPresets.value
  const locals = localRealityServers.value.map(s => ({
    target: `127.0.0.1:${s.port}`,
    port: s.port,
    source: 'local',
    id: s.id,
  }))
  return [...locals, ...presets]
})

// 协议是否支持 Reality（仅 vless / vmess / trojan）
const realitySupported = computed(() => REALITY_PROTOCOLS.includes(props.model.type))
// 当前传输是否被 Reality 支持
const realityTransportOk = computed(() => REALITY_TRANSPORTS.includes(props.model._transportType || ''))

// Security 下拉模型：none / tls / reality（三者互斥）
const securityMode = computed(() => {
  if (props.model._realityEnabled && realitySupported.value) return 'reality'
  if (props.model._tlsEnabled) return 'tls'
  return 'none'
})

// 加载证书列表
async function loadCertificates() {
  try {
    const res = await client.get('/certificates', { params: { limit: 100 } })
    certificates.value = res?.items || []
    // 如果已有证书路径，尝试匹配证书ID
    if (props.model._tlsCertPath) {
      const found = certificates.value.find(c =>
        (c.fullchain_path && props.model._tlsCertPath.includes(c.fullchain_path)) ||
        (c.cert_path && props.model._tlsCertPath.includes(c.cert_path))
      )
      if (found) props.model._certId = found.id
    }
  } catch {}
}

// 加载本地代理 Reality 握手服务器
async function loadLocalRealityServers() {
  const servers = []
  // 从 caddy 获取
  try {
    const caddySites = await client.get(`/node-servers/${props.serverId}/caddy/sites`)
    if (Array.isArray(caddySites)) {
      caddySites.filter(s => s.template === 'reality').forEach(s => {
        const port = parseInt((s.domain || ':9443').replace(/^:/, '')) || 9443
        servers.push({ id: s.id, port, source: 'caddy' })
      })
    }
  } catch {}
  // 从 nginx 获取
  try {
    const nginxSites = await client.get(`/node-servers/${props.serverId}/nginx/sites`)
    if (Array.isArray(nginxSites)) {
      nginxSites.filter(s => s.template === 'reality').forEach(s => {
        const port = parseInt(s.listen_port || s.domain || '9443') || 9443
        servers.push({ id: s.id, port, source: 'nginx' })
      })
    }
  } catch {}
  localRealityServers.value = servers
}

// 证书选择事件
function onCertSelect(certId) {
  if (!certId) {
    props.model._tlsCertPath = ''
    props.model._tlsKeyPath = ''
    props.model._certId = ''
    emit()
    return
  }
  // certId may be number or string; find with loose comparison
  const cert = certificates.value.find(c => c.id == certId)
  if (cert) {
    // Prefer fullchain_path; fall back to cert_path
    props.model._tlsCertPath = cert.fullchain_path || cert.cert_path || ''
    props.model._tlsKeyPath = cert.key_path || ''
    // Auto-fill server name from cert domain if empty
    if (cert.domain && !props.model._tlsServerName) {
      props.model._tlsServerName = cert.domain
    }
    emit()
  }
}

// Check if a target is a local Reality server (127.0.0.1:port format)
function isLocalTarget(target) {
  return target && target.startsWith('127.0.0.1:')
}

// Reality 目标选择事件
function onRealityTargetChange(target) {
  if (!target) return
  const tgt = ALL_REALITY_TARGETS.value.find(rt => rt.target === target)
  if (tgt) {
    if (tgt.source === 'local') {
      // 本地 Reality 服务器：连接地址为 127.0.0.1，SNI 为随机域名，xver=1
      props.model._realityHandshakeServer = '127.0.0.1'
      props.model._realityHandshakePort = parseInt(tgt.target.split(':')[1]) || 443
      const used = collectUsedSnis()
      let d = randomDomain()
      let attempts = 0
      while (used.has(d.toLowerCase()) && attempts < 50) { d = randomDomain(); attempts++ }
      props.model._tlsServerName = d
      props.model._realityXver = 1
    } else {
      // 公共预设服务器：地址和 SNI 相同，xver=0
      props.model._realityHandshakeServer = tgt.sni
      props.model._realityHandshakePort = parseInt(tgt.target.split(':')[1]) || 443
      props.model._tlsServerName = tgt.sni
      props.model._realityXver = 0
    }
  } else {
    // 自定义目标，尝试提取域名
    const parts = target.split(':')
    props.model._realityHandshakeServer = parts[0]
    props.model._realityHandshakePort = parseInt(parts[1]) || 443
    props.model._tlsServerName = parts[0]
    props.model._realityXver = 0
  }
  // 验证 SNI 唯一性
  validateSniUnique()
  emit()
}

// Regenerate random domain for local Reality serverName
function refreshRandomServerName() {
  const used = collectUsedSnis()
  let d = randomDomain()
  let attempts = 0
  while (used.has(d.toLowerCase()) && attempts < 50) { d = randomDomain(); attempts++ }
  props.model._tlsServerName = d
  emit()
}

// 验证 SNI 唯一性（同一服务器上不能重复使用相同的 SNI，否则路由冲突）
function validateSniUnique() {
  const mySni = (props.model._tlsServerName || '').trim().toLowerCase()
  if (!mySni || !props.inbounds) return true
  
  // 收集其他入站已使用的 SNI
  const usedByOthers = new Map()
  props.inbounds.forEach((ib, idx) => {
    if (ib === props.model) return // 跳过自己
    if (!ib._realityEnabled) return
    const otherSni = (ib._tlsServerName || '').trim().toLowerCase()
    if (otherSni && otherSni === mySni) {
      usedByOthers.set(otherSni, ib.tag || ib._remark || `#${idx + 1}`)
    }
  })
  
  if (usedByOthers.size > 0) {
    ElMessage.warning(t('singbox.tls.sniConflict', { sni: mySni, tag: usedByOthers.values().next().value }))
    return false
  }
  return true
}

// Server Name 手动编辑时触发 SNI 验证
function onServerNameChange() {
  validateSniUnique()
  emit()
}

// 刷新 Reality 目标（随机换一个）
function refreshRealityTarget() {
  const presets = ALL_REALITY_TARGETS.value.filter(t => t.source === 'preset')
  const tgt = presets[Math.floor(Math.random() * presets.length)]
  _realityTarget.value = tgt.target
  onRealityTargetChange(tgt.target)
}

function onSecurityChange(val) {
  if (val === 'reality') {
    props.model._tlsEnabled = true
    props.model._realityEnabled = true
    // 启用 Reality 后清空标准证书字段，避免同时存在
    props.model._tlsCertPath = ''
    props.model._tlsKeyPath = ''
    // 自动选择随机预设握手服务器
    autoSelectRealityTarget()
    // 自动生成 short_id（keypair 需要异步调用 API，单独触发）
    genRandomShortId()
    // 自动生成 keypair
    genRealityKey()
  } else if (val === 'tls') {
    props.model._tlsEnabled = true
    props.model._realityEnabled = false
    resetReality()
  } else {
    props.model._tlsEnabled = false
    props.model._realityEnabled = false
    resetReality()
  }
  emit()
}

// 自动选择随机预设握手服务器（不选本地，因为本地需要先配置代理站点）
function autoSelectRealityTarget() {
  const presets = ALL_REALITY_TARGETS.value.filter(t => t.source === 'preset')
  if (!presets.length) return
  const tgt = presets[Math.floor(Math.random() * presets.length)]
  _realityTarget.value = tgt.target
  // 公共预设服务器：地址和 SNI 相同，xver=0
  props.model._realityHandshakeServer = tgt.sni
  props.model._realityHandshakePort = parseInt(tgt.target.split(':')[1]) || 443
  props.model._tlsServerName = tgt.sni
  props.model._realityXver = 0
}

function resetReality() {
  props.model._realityHandshakeServer = ''
  props.model._realityHandshakePort = 443
  props.model._realityPrivateKey = ''
  props.model._realityPublicKey = ''
  props.model._realityShortIds = ''
  props.model._realityMaxTimeDiff = ''
  _realityTarget.value = ''
}

// 初始化 Reality 目标显示
function initRealityTarget() {
  const server = props.model._realityHandshakeServer
  const port = props.model._realityHandshakePort || 443
  // 本地 Reality 服务器
  if (server === '127.0.0.1') {
    _realityTarget.value = `127.0.0.1:${port}`
    return
  }
  const sni = props.model._tlsServerName
  if (!sni) return
  // 优先匹配 SNI
  const match = ALL_REALITY_TARGETS.value.find(t => t.sni === sni)
  if (match) {
    _realityTarget.value = match.target
  } else if (server) {
    _realityTarget.value = server + ':' + port
  }
}

// 由私钥推导公钥并回填：sing-box 入站（服务端）不接受 public_key 字段，
// 历史数据里没存公钥时，重新打开编辑器会看不到「公钥（客户端）」。
// x25519 私钥可推导公钥，交给后端算（internal/reality.PublicKeyFromPrivate）。
async function ensureRealityPublicKey() {
  if (!props.model._realityEnabled) return
  if (props.model._realityPublicKey) return
  const pk = String(props.model._realityPrivateKey || '').trim()
  if (!pk) return
  try {
    const data = await client.post(`/node-servers/${props.serverId}/binaries/reality-public-key`, { private_key: pk })
    const pub = data?.publicKey || data?.password || ''
    if (pub) props.model._realityPublicKey = pub
  } catch {}
}

watch(
  () => [props.model._realityEnabled, props.model._realityPrivateKey],
  () => { ensureRealityPublicKey() },
  { immediate: true }
)

// 组件挂载时加载证书和本地 Reality 服务器
onMounted(() => {
  loadCertificates()
  loadLocalRealityServers()
  loadRealityTargets()
  initRealityTarget()
})

// 用 sing-box generate reality-keypair 生成（服务端只需 private_key，public_key 给客户端/分享用）
async function genRealityKey() {
  if (securityMode.value !== 'reality') onSecurityChange('reality')
  try {
    const data = await client.post(`/node-servers/${props.serverId}/binaries/reality-key`)
    if (data && data.privateKey) {
      props.model._realityPrivateKey = data.privateKey
      props.model._realityPublicKey = data.publicKey || ''
      genRandomShortId()
      emit()
      ElMessage.success(t('singbox.tls.keypairGenerated'))
    }
  } catch (e) {
    ElMessage.error(t('singbox.tls.generateFailed', { msg: e.message || e }))
  }
}

function copyPublicKey() {
  const pk = props.model._realityPublicKey || ''
  if (!pk) { ElMessage.warning(t('singbox.tls.generateKeyFirst')); return }
  const ta = document.createElement('textarea')
  ta.value = pk
  document.body.appendChild(ta)
  ta.select()
  try { document.execCommand('copy'); ElMessage.success(t('singbox.tls.copied')) } catch { ElMessage.error(t('singbox.users.copyError')) }
  document.body.removeChild(ta)
}

function genRandomShortId() {
  const ids = ['']
  for (let i = 0; i < 12; i++) {
    const len = Math.random() > 0.5 ? 8 : 16
    ids.push(Array.from(crypto.getRandomValues(new Uint8Array(len / 2))).map(b => b.toString(16).padStart(2, '0')).join(''))
  }
  props.model._realityShortIds = ids.join('\n')
  emit()
}
</script>

<style scoped>
.hint { color: #909399; font-size: 12px; margin-top: 4px; }
.warn-hint { color: #e6a23c; }
</style>