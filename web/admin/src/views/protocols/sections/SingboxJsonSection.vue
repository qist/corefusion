<template>
  <div>
    <el-alert type="info" :closable="false" show-icon style="margin-bottom:12px">
      <div style="font-size:12px;line-height:1.7">
        {{ meta.desc }}<br>
        文档：<a :href="meta.docUrl" target="_blank">{{ meta.docUrl }}</a>
      </div>
    </el-alert>

    <!-- 列表型段落（services / certificate_providers / http_clients / network_namespaces） -->
    <template v-if="meta.mode === 'list'">
      <div style="margin-bottom:12px;display:flex;gap:8px;align-items:center">
        <el-select v-model="addType" placeholder="选择类型后添加" style="width:260px" clearable>
          <el-option v-for="tp in meta.types" :key="tp" :label="tp" :value="tp" />
        </el-select>
        <el-button type="primary" @click="addItem">添加</el-button>
        <span class="hint" v-if="!meta.types.length">该段落无 type 概念，直接添加后编辑字段</span>
      </div>

      <div v-for="(it, i) in items" :key="'it'+i" class="rule-item">
        <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:6px">
          <div style="display:flex;gap:8px;align-items:center">
            <el-tag v-if="it.type" size="small">{{ it.type }}</el-tag>
            <el-tag v-if="it.tag" size="small" type="success">{{ it.tag }}</el-tag>
            <span v-if="it._err" style="font-size:12px;color:#f56c6c">{{ it._err }}</span>
          </div>
          <el-button size="small" type="danger" text @click="rmItem(i)">删除</el-button>
        </div>
        <el-input v-model="it._json" type="textarea" :rows="7" spellcheck="false" @change="applyItem(i)" />
      </div>
      <div v-if="!items.length" style="text-align:center;color:#909399;padding:24px">暂无条目</div>
    </template>

    <!-- 对象型段落（certificate） -->
    <template v-else>
      <div style="margin-bottom:8px;display:flex;gap:8px;flex-wrap:wrap">
        <el-button v-for="t in meta.templates" :key="t.name" size="small" @click="applyObjectTemplate(t)">
          {{ t.name }}
        </el-button>
      </div>
      <el-input v-model="objJson" type="textarea" :rows="12" spellcheck="false" @change="applyObject" />
    </template>
  </div>
</template>
<script setup>
// 通用 sing-box 顶层段落编辑器：
// 用于低频/高级段落（services / certificate / certificate_providers / http_clients /
// network_namespaces），提供“按类型添加 + JSON 编辑字段”，避免为每种类型写专用表单。
// 类型清单取自 sing-box 官方 schema.json，模板字段取自各文档示例。
import { computed, reactive, ref, watch, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { prefetchFreePort, takeFreePort } from './utils/freePort'

const props = defineProps({ data: Object, coreType: String, serverId: Number, sectionKey: String })
const emitFn = defineEmits(['update'])

const REGISTRY = {
  services: {
    mode: 'list',
    desc: 'sing-box 顶层 services 段（与 inbounds/outbounds 并列）。服务端常用：hysteria-realm（P2P 中继）、usbip-server（共享 USB 设备）、ssm-api、api、ccm/ocm/derp/resolved。',
    docUrl: 'https://sing-box.sagernet.org/configuration/service/',
    types: ['api', 'ccm', 'derp', 'dynamic', 'hysteria-realm', 'ocm', 'oom-killer', 'resolved', 'ssm-api', 'usbip-client', 'usbip-server'],
    // 注意：listen_port 留空（listenPort: true 标记），创建时用后端 free-port 填入，
    // 避免与面板自身端口（8080/8443）或已有入站/端点撞端口。
    templates: {
      api: { type: 'api', tag: 'api', listen: '127.0.0.1', listenPort: true },
      ccm: { type: 'ccm', tag: 'ccm', listen: '0.0.0.0', listenPort: true },
      derp: { type: 'derp', tag: 'derp', listen: '0.0.0.0', listenPort: true },
      dynamic: { type: 'dynamic', tag: 'dynamic' },
      'hysteria-realm': { type: 'hysteria-realm', tag: 'realm', listen: '0.0.0.0', listenPort: true, users: [{ name: 'realm1', token: 'CHANGE_ME', max_realms: 0 }] },
      ocm: { type: 'ocm', tag: 'ocm', listen: '0.0.0.0', listenPort: true },
      'oom-killer': { type: 'oom-killer', tag: 'oom-killer' },
      resolved: { type: 'resolved', tag: 'resolved' },
      'ssm-api': { type: 'ssm-api', tag: 'ssm-api', listen: '0.0.0.0', listenPort: true },
      'usbip-client': { type: 'usbip-client', tag: 'usbip-client', server: '', server_port: 3240, devices: [] },
      'usbip-server': { type: 'usbip-server', tag: 'usbip-server', listen: '0.0.0.0', listenPort: true, devices: [] },
    },
  },
  certificate_providers: {
    mode: 'list',
    desc: '证书提供者（可被 TLS 的 certificate_provider 引用）。acme 用于自动签发，cloudflare-origin-ca / tailscale 用于其托管证书。',
    docUrl: 'https://sing-box.sagernet.org/configuration/certificate-provider/',
    types: ['acme', 'cloudflare-origin-ca', 'tailscale'],
    templates: {
      acme: { type: 'acme', tag: 'acme', domain: ['example.com'] },
      'cloudflare-origin-ca': { type: 'cloudflare-origin-ca', tag: 'cf-origin-ca' },
      tailscale: { type: 'tailscale', tag: 'tailscale' },
    },
  },
  http_clients: {
    mode: 'list',
    desc: '可复用的 HTTP 客户端（被 dashboard / rule_set / acme 等通过 tag 引用）。字段与拨号字段一致（detour / bind_interface / tls / headers…）。',
    docUrl: 'https://sing-box.sagernet.org/configuration/http-client/',
    types: [],
    templates: { default: { tag: 'http-client-1' } },
  },
  network_namespaces: {
    mode: 'list',
    desc: '网络命名空间。unshare 会在独立 netns 中运行（需 CAP_SYS_ADMIN），tag 用于 outbound/dial 的 netns 字段引用。',
    docUrl: 'https://sing-box.sagernet.org/configuration/network-namespace/',
    types: ['unshare', 'default'],
    templates: {
      unshare: { type: 'unshare', tag: 'ns1' },
      default: { tag: 'ns0' },
    },
  },
  certificate: {
    mode: 'object',
    desc: '全局证书段：store=system 使用系统证书库（默认）；也可内联 certificate/key 或给目录。',
    docUrl: 'https://sing-box.sagernet.org/configuration/certificate/',
    templates: [
      { name: 'store: system', value: { store: 'system' } },
      { name: 'store: mozilla', value: { store: 'mozilla' } },
      { name: 'certificate_path 目录', value: { certificate_directory_path: '/etc/ssl/certs' } },
    ],
  },
}

const meta = computed(() => REGISTRY[props.sectionKey] || { mode: 'list', desc: '', docUrl: '', types: [], templates: {} })

const addType = ref('')
const items = reactive([])
const objJson = ref('')
const lastSynced = ref('')

function pretty(o) { return JSON.stringify(o, null, 2) }

function sync() {
  const raw = JSON.stringify(props.data ?? null)
  if (raw === lastSynced.value) return // 避免自己 emit 后再重置编辑框
  lastSynced.value = raw
  const d = props.data
  if (meta.value.mode === 'list') {
    items.splice(0, items.length)
    const list = Array.isArray(d) ? d : []
    for (const it of list) {
      const { type, tag, ...rest } = it || {}
      const payload = { ...(type ? { type } : {}), ...(tag ? { tag } : {}), ...rest }
      items.push({ type: type || '', tag: tag || '', _json: pretty(payload), _err: '', _raw: it })
    }
  } else {
    objJson.value = d && Object.keys(d).length ? pretty(d) : '{}'
  }
}

function build() {
  if (meta.value.mode === 'list') {
    const out = []
    for (const it of items) {
      try {
        out.push(JSON.parse(it._json))
      } catch {
        // 保留原始对象，避免编辑中途 JSON 不完整导致丢数据
        if (it._raw) out.push(it._raw)
      }
    }
    return out
  }
  try {
    const v = JSON.parse(objJson.value)
    return v && typeof v === 'object' ? v : {}
  } catch {
    return props.data && typeof props.data === 'object' ? props.data : {}
  }
}

function addItem() {
  const key = addType.value || 'default'
  const tpl = (meta.value.templates && meta.value.templates[key]) || { ...(addType.value ? { type: addType.value } : {}) }
  const { type, tag, listenPort, ...rest } = tpl
  // listenPort 标记的模板：用该节点未被占用的端口填充
  const payload = { ...(type ? { type } : {}), ...(tag ? { tag } : {}), ...rest }
  if (listenPort) payload.listen_port = takeFreePort(props.serverId)
  items.push({ type: type || '', tag: tag || '', _json: pretty(payload), _err: '', _raw: payload })
  addType.value = ''
  emit()
}

function rmItem(i) { items.splice(i, 1); emit() }

function applyItem(i) {
  const it = items[i]
  try {
    const v = JSON.parse(it._json)
    if (!v || typeof v !== 'object' || Array.isArray(v)) throw new Error('必须是 JSON 对象')
    it._raw = v
    it.type = v.type || ''
    it.tag = v.tag || ''
    it._err = ''
  } catch (e) {
    it._err = 'JSON 无效：' + e.message
    ElMessage.error(`第 ${i + 1} 项 JSON 无效：${e.message}`)
    return
  }
  emit()
}

function applyObjectTemplate(t) { objJson.value = pretty(t.value); applyObject() }

function applyObject() {
  try {
    const v = JSON.parse(objJson.value)
    if (!v || typeof v !== 'object' || Array.isArray(v)) throw new Error('必须是 JSON 对象')
    emitFn('update', v)
  } catch (e) {
    ElMessage.error('JSON 无效：' + e.message)
  }
}

function emit() { emitFn('update', build()) }

watch(() => props.data, sync)
watch(() => props.sectionKey, () => { lastSynced.value = ''; sync() })
onMounted(() => {
  sync()
  prefetchFreePort(props.serverId) // 预热空闲端口，模板新增时不撞面板自身/已有入站端口
})
</script>
<style scoped>
.rule-item { background: var(--el-fill-color-light); border-radius: 6px; padding: 12px; margin-bottom: 8px; }
.hint { font-size: 12px; color: var(--el-text-color-secondary); }
</style>
