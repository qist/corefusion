<template>
  <div>
    <el-alert type="info" :closable="false" show-icon style="margin-bottom:12px">
      <div style="font-size:12px;line-height:1.7">
        {{ t('singbox.services.alert') }}<br>
        <a href="https://sing-box.sagernet.org/configuration/service/hysteria-realm/" target="_blank">sing-box / Hysteria Realm</a>
      </div>
    </el-alert>

    <div style="margin-bottom:12px;display:flex;gap:8px;align-items:center;flex-wrap:wrap">
      <el-select v-model="addType" :placeholder="t('singbox.services.addPlaceholder')" style="width:240px" clearable>
        <el-option v-for="tp in SERVICE_TYPES" :key="tp" :label="tp" :value="tp" />
      </el-select>
      <el-button type="primary" :disabled="!addType" @click="addService">{{ t('singbox.services.addService') }}</el-button>
    </div>

    <div v-for="(it, i) in items" :key="'svc' + i" class="svc-item">
      <div class="svc-head">
        <div style="display:flex;gap:8px;align-items:center">
          <el-tag size="small">{{ it.type }}</el-tag>
          <el-tag v-if="it.tag" size="small" type="success">{{ it.tag }}</el-tag>
          <span v-if="it._err" class="err">{{ it._err }}</span>
        </div>
        <el-button size="small" type="danger" text @click="rmService(i)">{{ t('common.delete') }}</el-button>
      </div>

      <!-- ===== hysteria-realm 结构化表单 ===== -->
      <template v-if="it._kind === 'realm'">
        <el-form label-width="170px" label-position="left" size="small">
          <el-form-item :label="t('singbox.listen.tag')">
            <el-input v-model="it.tag" placeholder="realm" />
          </el-form-item>
          <el-form-item :label="t('singbox.listen.listen')">
            <el-input v-model="it.listen" placeholder="::" />
          </el-form-item>
          <el-form-item :label="t('singbox.listen.listenPort')">
            <el-input-number v-model="it.listen_port" :min="0" :max="65535" style="width:100%" />
          </el-form-item>

          <el-divider content-position="left">{{ t('singbox.protocol.endpoint.oc.tls') }}</el-divider>
          <el-form-item :label="t('singbox.protocol.endpoint.masque.tlsEnabled')">
            <el-switch v-model="it._tlsEnabled" />
            <div class="hint">{{ t('singbox.services.tlsHint') }}</div>
          </el-form-item>
          <template v-if="it._tlsEnabled">
            <el-form-item :label="t('singbox.protocol.endpoint.ovpn.tlsCert')">
              <el-input v-model="it._tlsCertificate" type="textarea" :rows="3" :placeholder="t('singbox.protocol.endpoint.ovpn.tlsCert')" />
            </el-form-item>
            <el-form-item :label="t('singbox.protocol.endpoint.ovpn.tlsCertPath')">
              <el-input v-model="it._tlsCertificatePath" placeholder="/etc/ssl/certs/realm.pem" />
            </el-form-item>
            <el-form-item :label="t('singbox.protocol.endpoint.ovpn.tlsKey')">
              <el-input v-model="it._tlsKey" type="textarea" :rows="3" show-password />
            </el-form-item>
            <el-form-item :label="t('singbox.protocol.endpoint.ovpn.tlsKeyPath')">
              <el-input v-model="it._tlsKeyPath" placeholder="/etc/ssl/private/realm.key" />
            </el-form-item>
            <el-form-item :label="t('singbox.tls.alpn')">
              <el-input v-model="it._tlsAlpnStr" placeholder="h2" />
              <div class="hint">{{ t('singbox.services.alpnHint') }}</div>
            </el-form-item>
            <el-form-item :label="t('singbox.tls.minVersion')">
              <el-select v-model="it._tlsMinVersion" style="width:100%" clearable>
                <el-option label="1.2" value="1.2" />
                <el-option label="1.3" value="1.3" />
              </el-select>
            </el-form-item>
            <el-form-item :label="t('singbox.tls.maxVersion')">
              <el-select v-model="it._tlsMaxVersion" style="width:100%" clearable>
                <el-option label="1.2" value="1.2" />
                <el-option label="1.3" value="1.3" />
              </el-select>
            </el-form-item>
          </template>

          <el-divider content-position="left">{{ t('singbox.services.http2Section') }}</el-divider>
          <el-form-item :label="t('singbox.protocol.endpoint.masque.idleTimeout')">
            <el-input v-model="it.idle_timeout" placeholder="30s" />
          </el-form-item>
          <el-form-item :label="t('singbox.protocol.endpoint.masque.keepAlivePeriod')">
            <el-input v-model="it.keep_alive_period" placeholder="10s" />
          </el-form-item>
          <el-form-item :label="t('singbox.protocol.endpoint.masque.streamReceiveWindow')">
            <el-input v-model="it.stream_receive_window" placeholder="8MB" />
          </el-form-item>
          <el-form-item :label="t('singbox.protocol.endpoint.masque.connectionReceiveWindow')">
            <el-input v-model="it.connection_receive_window" placeholder="20MB" />
          </el-form-item>
          <el-form-item :label="t('singbox.protocol.endpoint.masque.maxConcurrentStreams')">
            <el-input-number v-model="it.max_concurrent_streams" :min="0" />
          </el-form-item>

          <el-divider content-position="left">{{ t('singbox.protocol.endpoint.ovpn.users') }}</el-divider>
          <div class="hint" style="margin-bottom:8px">{{ t('singbox.services.usersHint') }}</div>
          <div v-for="(u, ui) in it.users" :key="ui" class="user-row">
            <el-input v-model="u.name" :placeholder="t('singbox.services.userName')" style="width:160px" />
            <el-input v-model="u.token" :placeholder="t('singbox.services.userToken')" show-password style="flex:1" />
            <el-button size="small" @click="u.token = generateToken()">{{ t('singbox.services.generateToken') }}</el-button>
            <el-input-number v-model="u.max_realms" :min="0" controls-position="right" style="width:130px" />
            <el-button size="small" type="danger" text @click="it.users.splice(ui, 1)">X</el-button>
          </div>
          <div class="hint">{{ t('singbox.services.maxRealmsHint') }}</div>
          <el-button size="small" style="margin-top:8px" @click="addUser(it)">+ {{ t('singbox.services.addUser') }}</el-button>

          <el-collapse style="margin-top:12px">
            <el-collapse-item :title="t('singbox.protocol.endpoint.masque.listenSection')">
              <EndpointListenFields :model="it" />
            </el-collapse-item>
          </el-collapse>
        </el-form>
      </template>

      <!-- ===== 其它服务类型：JSON 编辑 ===== -->
      <template v-else>
        <el-input v-model="it._json" type="textarea" :rows="8" spellcheck="false" @change="applyJson(i)" />
      </template>
    </div>

    <div v-if="!items.length" class="empty">{{ t('singbox.services.empty') }}</div>
  </div>
</template>

<script setup>
// sing-box 顶层 services[] 编辑器。
// hysteria-realm（hysteria2 的 rendezvous 服务）走结构化表单，其余服务类型保留 JSON 编辑。
import { reactive, ref, watch, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { prefetchFreePort, takeFreePort } from './utils/freePort'
import EndpointListenFields from './singbox/endpoints/EndpointListenFields.vue'
import {
  SERVICE_TYPES, STRUCTURED_SERVICE_TYPES,
  serviceDefaultsForType, buildService, parseService,
} from './singbox/utils/singboxServices'
import { generatePsk } from './utils/credentials'

const { t } = useI18n()
const props = defineProps({ data: Array, coreType: String, serverId: Number, sectionKey: String })
const emitFn = defineEmits(['update'])

const addType = ref('')
const items = reactive([])
const lastSynced = ref('')

const pretty = o => JSON.stringify(o, null, 2)
const generateToken = () => generatePsk()

function sync() {
  const raw = JSON.stringify(props.data ?? null)
  if (raw === lastSynced.value) return
  lastSynced.value = raw
  items.splice(0, items.length)
  for (const cfg of Array.isArray(props.data) ? props.data : []) {
    if (!cfg || typeof cfg !== 'object') continue
    if (STRUCTURED_SERVICE_TYPES.includes(cfg.type)) {
      items.push({ _kind: 'realm', ...parseService(cfg) })
    } else {
      items.push({ _kind: 'json', type: cfg.type || '', tag: cfg.tag || '', _json: pretty(cfg), _raw: cfg, _err: '' })
    }
  }
}

function build() {
  return items.map(it => {
    if (it._kind === 'realm') return buildService(it)
    try {
      const v = JSON.parse(it._json)
      if (!v || typeof v !== 'object' || Array.isArray(v)) throw new Error(t('singbox.services.mustBeObject'))
      return v
    } catch {
      // JSON 不完整时保留上一次的有效对象，避免编辑过程中丢数据
      return it._raw || { type: it.type, tag: it.tag }
    }
  })
}

function emit() {
  const out = build()
  lastSynced.value = JSON.stringify(out)
  emitFn('update', out)
}

function addService() {
  const type = addType.value
  if (!type) return
  const model = serviceDefaultsForType(type, takeFreePort(props.serverId))
  if (STRUCTURED_SERVICE_TYPES.includes(type)) {
    items.push({ _kind: 'realm', ...model })
  } else {
    items.push({ _kind: 'json', type, tag: model.tag, _json: pretty(model), _raw: model, _err: '' })
  }
  addType.value = ''
}

function rmService(i) {
  items.splice(i, 1)
}

function addUser(it) {
  if (!Array.isArray(it.users)) it.users = []
  it.users.push({ name: '', token: generateToken(), max_realms: 0 })
}

function applyJson(i) {
  const it = items[i]
  try {
    const v = JSON.parse(it._json)
    if (!v || typeof v !== 'object' || Array.isArray(v)) throw new Error(t('singbox.services.mustBeObject'))
    it._raw = v
    it.type = v.type || ''
    it.tag = v.tag || ''
    it._err = ''
  } catch (e) {
    it._err = t('singbox.services.invalidJson') + e.message
    ElMessage.error(t('singbox.services.invalidJson') + e.message)
  }
}

watch(items, () => emit(), { deep: true })
watch(() => props.data, sync)
watch(() => props.sectionKey, () => { lastSynced.value = ''; sync() })
onMounted(() => {
  sync()
  prefetchFreePort(props.serverId)
})
</script>

<style scoped>
.svc-item { background: var(--el-fill-color-light); border-radius: 6px; padding: 12px; margin-bottom: 10px; }
.svc-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 10px; }
.user-row { display: flex; gap: 8px; margin-bottom: 6px; align-items: center; }
.hint { font-size: 12px; color: var(--el-text-color-secondary); margin-top: 2px; }
.err { font-size: 12px; color: #f56c6c; }
.empty { text-align: center; color: #909399; padding: 24px; }
</style>
