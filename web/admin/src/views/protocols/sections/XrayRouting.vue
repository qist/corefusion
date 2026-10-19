<template>
  <div>
    <!-- Geo files warning -->
    <el-alert v-if="!geoStatus['geoip.dat'] || !geoStatus['geosite.dat']" type="warning" :closable="false" style="margin-bottom:16px">
      <template #title>
        <span>{{ t('protoCenter.routing.geoWarning') }}</span>
        <span v-if="!geoStatus['geoip.dat']" style="color:#e6a23c;font-weight:600">geoip.dat </span>
        <span v-if="!geoStatus['geosite.dat']" style="color:#e6a23c;font-weight:600">geosite.dat </span>
        <span>— {{ t('protoCenter.routing.geoWarningHint') }}</span>
      </template>
    </el-alert>

    <!-- Global Settings -->
    <el-divider content-position="left">{{ t('protoCenter.routing.globalSettings') }}</el-divider>
    <el-form label-width="140px" label-position="left">
      <el-form-item :label="t('protoCenter.routing.domainStrategy')">
        <el-select v-model="form.domainStrategy" style="width:100%" @change="emit">
          <el-option label="AsIs" value="AsIs" />
          <el-option label="IPIfNonMatch" value="IPIfNonMatch" />
          <el-option label="IPOnDemand" value="IPOnDemand" />
        </el-select>
        <div class="hint">{{ t('protoCenter.routing.domainStrategyHint') }}</div>
      </el-form-item>
    </el-form>

    <!-- Routing Rules -->
    <el-divider content-position="left">
      {{ t('protoCenter.routing.rules') }}
      <el-button size="small" type="primary" @click="showAddRule" style="margin-left:12px">{{ t('protoCenter.routing.addRule') }}</el-button>
    </el-divider>

    <div v-for="(r, i) in rulesPaged" :key="i" class="rule-item">
      <div style="display:flex;justify-content:space-between;align-items:center">
        <div style="display:flex;align-items:center;gap:8px;flex-wrap:wrap">
          <el-switch v-model="r.enabled" @change="emit" style="margin-right:4px" />
          <el-tag v-if="r.ruleTag" size="small">{{ r.ruleTag }}</el-tag>
          <el-tag v-if="r.outboundTag" size="small" :type="r.outboundTag==='blocked'?'danger':r.outboundTag==='direct'?'success':'info'">{{ r.outboundTag }}</el-tag>
          <el-tag v-if="r.balancerTag" size="small" type="warning">{{ r.balancerTag }}</el-tag>
          <span v-if="r.domain && r.domain.length" style="font-size:12px;color:#909399">{{ t('protoCenter.routing.domainCount', { count: r.domain.length }) }}</span>
          <span v-if="r.ip && r.ip.length" style="font-size:12px;color:#909399">{{ t('protoCenter.routing.ipCount', { count: r.ip.length }) }}</span>
          <span v-if="r.protocol && r.protocol.length" style="font-size:12px;color:#909399">{{ r.protocol.join(',') }}</span>
          <span v-if="r.port" style="font-size:12px;color:#909399">{{ t('protoCenter.routing.portValue', { value: r.port }) }}</span>
          <span v-if="r.network" style="font-size:12px;color:#909399">{{ r.network }}</span>
          <span v-if="r.inboundTag && r.inboundTag.length" style="font-size:12px;color:#909399">{{ t('protoCenter.routing.inboundShort', { value: r.inboundTag.join(',') }) }}</span>
          <span v-if="r.webhook" style="font-size:12px;color:#909399">{{ t('protoCenter.routing.webhook') }}</span>
        </div>
        <div>
          <el-button size="small" @click="editRule(r)">{{ t('common.edit') }}</el-button>
          <el-button size="small" type="danger" text @click="rmRule(r)">X</el-button>
        </div>
      </div>
    </div>
    <PaginationBar v-model:page="rulePage" v-model:page-size="ruleSize" :total="ruleTotal" />

    <!-- Balancers -->
    <el-divider content-position="left">
      {{ t('protoCenter.routing.balancers') }}
      <el-button size="small" type="primary" @click="showAddBalancer" style="margin-left:12px">{{ t('protoCenter.routing.addBalancer') }}</el-button>
    </el-divider>
    <div v-for="(b, i) in balPaged" :key="'b'+i" class="rule-item">
      <div style="display:flex;justify-content:space-between;align-items:center">
        <div style="display:flex;align-items:center;gap:8px;flex-wrap:wrap">
          <el-switch v-model="b.enabled" @change="emit" style="margin-right:4px" />
          <el-tag size="small">{{ b.tag }}</el-tag>
          <el-tag size="small" type="info">{{ b.strategyType }}</el-tag>
          <span v-if="b.selectorArr && b.selectorArr.length" style="font-size:12px;color:#909399">selector: {{ b.selectorArr.join(', ') }}</span>
          <span v-if="b.fallbackTag" style="font-size:12px;color:#909399">{{ t('protoCenter.routing.fallbackShort', { value: b.fallbackTag }) }}</span>
        </div>
        <div>
          <el-button size="small" @click="editBalancer(b)">{{ t('common.edit') }}</el-button>
          <el-button size="small" type="danger" text @click="rmBalancer(b)">X</el-button>
        </div>
      </div>
    </div>
    <PaginationBar v-model:page="balPage" v-model:page-size="balSize" :total="balTotal" />

    <!-- Rule Dialog -->
    <el-dialog v-model="dlg" :title="editIdx >= 0 ? t('protoCenter.routing.editRule') : t('protoCenter.routing.addRule')" width="700">
      <el-form label-width="120px" label-position="left">
        <el-form-item :label="t('protoCenter.routing.ruleName')">
          <el-input v-model="rf.ruleTag" />
        </el-form-item>
        <el-alert type="info" :closable="false" show-icon style="margin-bottom:16px">
          {{ t('protoCenter.routing.multiConditionHint') }}
        </el-alert>

        <el-divider content-position="left">{{ t('protoCenter.routing.matchConditions') }}</el-divider>
        <el-form-item :label="t('protoCenter.routing.domain')">
          <el-select v-model="rf._domain" multiple filterable allow-create default-first-option :placeholder="t('protoCenter.routing.selectOrType')" style="width:100%">
            <el-option-group label="Geosite">
              <el-option label="geosite:cn" value="geosite:cn" /><el-option label="geosite:geolocation-cn" value="geosite:geolocation-cn" />
              <el-option label="geosite:google" value="geosite:google" /><el-option label="geosite:github" value="geosite:github" />
              <el-option label="geosite:telegram" value="geosite:telegram" /><el-option label="geosite:openai" value="geosite:openai" />
              <el-option label="geosite:netflix" value="geosite:netflix" /><el-option label="geosite:category-ads-all" value="geosite:category-ads-all" />
            </el-option-group>
          </el-select>
        </el-form-item>
        <el-form-item :label="t('protoCenter.routing.ip')">
          <el-select v-model="rf._ip" multiple filterable allow-create default-first-option :placeholder="t('protoCenter.routing.selectOrType')" style="width:100%">
            <el-option-group label="GeoIP">
              <el-option label="geoip:cn" value="geoip:cn" /><el-option label="geoip:us" value="geoip:us" />
              <el-option label="geoip:telegram" value="geoip:telegram" /><el-option label="geoip:private" value="geoip:private" />
              <el-option label="geoip:!cn" value="geoip:!cn" />
            </el-option-group>
          </el-select>
        </el-form-item>
        <el-row :gutter="12">
          <el-col :span="8"><el-form-item :label="t('protoCenter.routing.port')"><el-input v-model="rf._port" placeholder="53,443,1000-2000" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item :label="t('protoCenter.routing.sourcePort')"><el-input v-model="rf._sourcePort" placeholder="53,443" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item :label="t('protoCenter.routing.localPort')"><el-input v-model="rf._localPort" placeholder="443" /></el-form-item></el-col>
        </el-row>
        <el-row :gutter="12">
          <el-col :span="8"><el-form-item :label="t('protoCenter.routing.network')"><el-select v-model="rf._network" clearable style="width:100%"><el-option label="tcp" value="tcp" /><el-option label="udp" value="udp" /><el-option label="tcp,udp" value="tcp,udp" /></el-select></el-form-item></el-col>
          <el-col :span="16"><el-form-item :label="t('protoCenter.routing.protocol')"><el-select v-model="rf._protocol" multiple style="width:100%"><el-option label="http" value="http" /><el-option label="tls" value="tls" /><el-option label="quic" value="quic" /><el-option label="bittorrent" value="bittorrent" /></el-select></el-form-item></el-col>
        </el-row>
        <el-form-item :label="t('protoCenter.routing.sourceIP')"><el-select v-model="rf._sourceIP" multiple filterable allow-create default-first-option :placeholder="t('protoCenter.routing.selectOrType')" style="width:100%"><el-option label="geoip:cn" value="geoip:cn" /><el-option label="geoip:private" value="geoip:private" /></el-select></el-form-item>
        <el-form-item :label="t('protoCenter.routing.localIP')"><el-select v-model="rf._localIP" multiple filterable allow-create default-first-option :placeholder="t('protoCenter.routing.selectOrType')" style="width:100%"><el-option label="192.168.0.0/16" value="192.168.0.0/16" /><el-option label="10.0.0.0/8" value="10.0.0.0/8" /></el-select></el-form-item>
        <el-form-item :label="t('protoCenter.routing.inboundTag')"><el-select v-model="rf._inboundTag" multiple filterable allow-create default-first-option :placeholder="t('protoCenter.routing.selectOrType')" style="width:100%"><el-option v-for="tag in activeInboundTags" :key="tag" :label="tag" :value="tag" /></el-select></el-form-item>
        <el-form-item :label="t('protoCenter.routing.process')"><el-select v-model="rf._process" multiple filterable allow-create default-first-option :placeholder="t('protoCenter.routing.selectOrType')" style="width:100%"><el-option label="curl" value="curl" /><el-option label="wget" value="wget" /><el-option label="self/" value="self/" /><el-option label="xray/" value="xray/" /></el-select></el-form-item>
        <el-form-item label="Local OS">
          <el-select v-model="rf._localOS" multiple filterable allow-create default-first-option placeholder="linux / windows / darwin ..." style="width:100%">
            <el-option label="linux" value="linux" /><el-option label="windows" value="windows" />
            <el-option label="darwin (macOS)" value="darwin" /><el-option label="android" value="android" />
            <el-option label="freebsd" value="freebsd" /><el-option label="openbsd" value="openbsd" />
            <el-option label="netbsd" value="netbsd" /><el-option label="dragonfly" value="dragonfly" />
            <el-option label="solaris" value="solaris" />
          </el-select>
          <div class="hint">按**运行核心的机器**的操作系统分流（= Go 的 GOOS，大小写不敏感）；匹配到多个时命中任意一个即可</div>
        </el-form-item>
        <el-form-item :label="t('protoCenter.routing.user')"><el-select v-model="rf._user" multiple filterable allow-create default-first-option :placeholder="t('protoCenter.routing.selectOrType')" style="width:100%"><el-option v-for="tag in userTags" :key="tag" :label="tag" :value="tag" /></el-select></el-form-item>
        <el-form-item :label="t('protoCenter.routing.vlessRoute')"><el-input v-model="rf._vlessRoute" placeholder="53,443,1000-2000" /></el-form-item>
        <el-form-item :label="t('protoCenter.routing.attrs')">
          <div v-for="(a, i) in rf._attrs" :key="i" style="display:flex;gap:8px;margin-bottom:6px"><el-input v-model="a.key" placeholder=":method" style="width:40%" /><el-input v-model="a.value" placeholder="GET" style="width:40%" /><el-button size="small" type="danger" text @click="rf._attrs.splice(i,1)">X</el-button></div>
          <el-button size="small" @click="rf._attrs.push({key:'',value:''})">{{ t('protoCenter.routing.addItem') }}</el-button>
        </el-form-item>
        <el-form-item :label="t('protoCenter.routing.webhookUrl')"><el-input v-model="rf._webhookUrl" :placeholder="t('protoCenter.routing.webhookUrlPlaceholder')" /></el-form-item>
        <el-form-item v-if="rf._webhookUrl" :label="t('protoCenter.routing.webhookDedup')"><el-input-number v-model="rf._webhookDedup" :min="0" /></el-form-item>
        <el-form-item v-if="rf._webhookUrl" :label="t('protoCenter.routing.webhookHeaders')">
          <div v-for="(h, i) in rf._webhookHeaders" :key="i" style="display:flex;gap:8px;margin-bottom:6px"><el-input v-model="h.key" :placeholder="t('protoCenter.routing.webhookHeaderName')" style="width:40%" /><el-input v-model="h.value" :placeholder="t('protoCenter.routing.webhookHeaderValue')" style="width:40%" /><el-button size="small" type="danger" text @click="rf._webhookHeaders.splice(i,1)">X</el-button></div>
          <el-button size="small" @click="rf._webhookHeaders.push({key:'',value:''})">{{ t('protoCenter.routing.addItem') }}</el-button>
        </el-form-item>

        <el-divider content-position="left">{{ t('protoCenter.routing.action') }}</el-divider>

        <el-form-item :label="t('protoCenter.routing.outbound')">
          <el-select v-model="rf.outboundTag" style="width:100%" clearable>
            <el-option label="" value="" />
            <el-option v-for="t in activeOutboundTags" :key="t" :label="t" :value="t" />
          </el-select>
          <div class="hint">{{ t('protoCenter.routing.outboundOptionalHint') }}</div>
        </el-form-item>
        <el-form-item :label="t('protoCenter.routing.balancer')">
          <el-select v-model="rf.balancerTag" style="width:100%" clearable>
            <el-option :label="t('protoCenter.routing.none')" value="" />
            <el-option v-for="b in activeBalancers" :key="b.tag" :label="b.tag" :value="b.tag" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dlg=false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="saveRule">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- Balancer Dialog -->
    <el-dialog v-model="balDlg" :title="balIdx >= 0 ? t('protoCenter.routing.editBalancer') : t('protoCenter.routing.addBalancer')" width="500">
      <el-form label-width="120px" label-position="left">
        <el-form-item :label="t('protoCenter.routing.tagLabel')"><el-input v-model="bf.tag" :placeholder="t('protoCenter.routing.balancerTagPlaceholder')" /></el-form-item>
        <el-form-item :label="t('protoCenter.routing.selector')">
          <el-select v-model="bf.selectorArr" multiple filterable allow-create default-first-option :placeholder="t('protoCenter.routing.selectorPlaceholder')" style="width:100%">
            <el-option v-for="t in activeOutboundTags" :key="t" :label="t" :value="t" />
          </el-select>
          <div class="hint">{{ t('protoCenter.routing.selectorHint') }}</div>
        </el-form-item>
        <el-form-item :label="t('protoCenter.routing.fallbackTag')">
          <el-select v-model="bf.fallbackTag" style="width:100%" clearable>
            <el-option :label="t('protoCenter.routing.none')" value="" />
            <el-option v-for="t in activeOutboundTags" :key="t" :label="t" :value="t" />
          </el-select>
          <div class="hint">{{ t('protoCenter.routing.fallbackTagHint') }}</div>
        </el-form-item>
        <el-form-item :label="t('protoCenter.routing.strategy')">
          <el-select v-model="bf.strategyType" style="width:100%">
            <el-option label="random" value="random" /><el-option label="roundRobin" value="roundRobin" />
            <el-option label="leastPing" value="leastPing" /><el-option label="leastLoad" value="leastLoad" />
          </el-select>
          <div class="hint">{{ t('protoCenter.routing.strategyHint') }}</div>
        </el-form-item>
        <template v-if="bf.strategyType === 'leastLoad'">
          <el-divider content-position="left">{{ t('protoCenter.routing.strategySettings') }}</el-divider>
          <el-form-item :label="t('protoCenter.routing.expected')">
            <el-input-number v-model="bf._expected" :min="0" />
            <div class="hint">{{ t('protoCenter.routing.expectedHint') }}</div>
          </el-form-item>
          <el-form-item :label="t('protoCenter.routing.maxRTT')">
            <el-input v-model="bf._maxRTT" placeholder="1s" />
            <div class="hint">{{ t('protoCenter.routing.maxRTTHint') }}</div>
          </el-form-item>
          <el-form-item :label="t('protoCenter.routing.tolerance')">
            <el-input-number v-model="bf._tolerance" :min="0" :max="1" :step="0.01" />
            <div class="hint">{{ t('protoCenter.routing.toleranceHint') }}</div>
          </el-form-item>
          <el-form-item :label="t('protoCenter.routing.baselines')">
            <el-input v-model="bf._baselinesStr" placeholder="1s, 2s" />
            <div class="hint">{{ t('protoCenter.routing.baselinesHint') }}</div>
          </el-form-item>
          <el-form-item label="Costs (JSON)">
            <el-input v-model="bf._costsJson" type="textarea" :rows="4"
              placeholder='[{"match": "tag:out-1", "value": 1.0}, {"regexp": true, "match": "^hk-", "value": 0.5}]' />
            <div class="hint">
              leastLoad 的权重表，数组元素：<b>match</b>（出站 tag 或正则）+ <b>value</b>（float，越小越优先）+
              可选 <b>regexp:true</b>（把 match 当正则）。留空则不写该字段。
            </div>
          </el-form-item>
        </template>
      </el-form>
      <template #footer>
        <el-button @click="balDlg=false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="saveBalancer">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!--
      链式代理自动注入（只读）：仅在**节点上下文**（协议中心）显示。
      订阅格式管理是全局模板，绝不能出现节点侧内容（数据隔离）。
    -->
    <template v-if="serverId">
      <el-divider content-position="left">
        {{ t('protoCenter.routing.chainAutoInjected') }}
      </el-divider>
      <el-alert type="info" :closable="false" show-icon style="margin-bottom:12px">
        {{ t('protoCenter.routing.chainAutoInjectedHint') }}
      </el-alert>
      <div v-if="!chainRules.length && !chainBalancers.length && !chainOutbounds.length" class="rule-item" style="text-align:center;color:#909399">
        {{ t('common.noData') }}
      </div>
      <template v-if="chainOutbounds.length">
        <div style="font-size:13px;color:#909399;margin-bottom:6px">{{ t('protoCenter.routing.chainOutbounds') }} ({{ chainOutbounds.length }})</div>
        <div v-for="(ob, i) in chainOutbounds" :key="'cob'+i" class="rule-item chain-item">
          <div style="display:flex;align-items:center;gap:8px;flex-wrap:wrap">
            <el-tag size="small" type="warning">{{ ob.tag }}</el-tag>
            <el-tag size="small">{{ ob.protocol }}</el-tag>
            <span v-if="ob._server" style="font-size:12px;color:#909399">{{ ob._server }}:{{ ob._port }}</span>
          </div>
        </div>
      </template>
      <template v-if="chainRules.length">
        <div style="font-size:13px;color:#909399;margin:12px 0 6px">{{ t('protoCenter.routing.chainRules') }} ({{ chainRules.length }})</div>
        <div v-for="(r, i) in chainRules" :key="'cr'+i" class="rule-item chain-item">
          <div style="display:flex;align-items:center;gap:8px;flex-wrap:wrap">
            <el-tag v-if="r.ruleTag" size="small" type="warning">{{ r.ruleTag }}</el-tag>
            <el-tag v-if="r.outboundTag" size="small" type="info">{{ r.outboundTag }}</el-tag>
            <el-tag v-if="r.balancerTag" size="small" type="warning">{{ r.balancerTag }}</el-tag>
            <span v-if="r.inboundTag && r.inboundTag.length" style="font-size:12px;color:#909399">{{ t('protoCenter.routing.inboundShort', { value: r.inboundTag.join(',') }) }}</span>
          </div>
        </div>
      </template>
      <template v-if="chainBalancers.length">
        <div style="font-size:13px;color:#909399;margin:12px 0 6px">{{ t('protoCenter.routing.chainBalancers') }} ({{ chainBalancers.length }})</div>
        <div v-for="(b, i) in chainBalancers" :key="'cb'+i" class="rule-item chain-item">
          <div style="display:flex;align-items:center;gap:8px;flex-wrap:wrap">
            <el-tag size="small" type="warning">{{ b.tag }}</el-tag>
            <el-tag size="small" type="info">{{ b.strategy?.type || 'random' }}</el-tag>
            <span v-if="b.selector && b.selector.length" style="font-size:12px;color:#909399">selector: {{ b.selector.join(', ') }}</span>
            <span v-if="b.fallbackTag" style="font-size:12px;color:#909399">{{ t('protoCenter.routing.fallbackShort', { value: b.fallbackTag }) }}</span>
          </div>
        </div>
      </template>
    </template>
  </div>
</template>

<script setup>
import { computed, reactive, ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import client from '../../../api/client'
import { useClientPagination } from '../../../composables/useClientPagination'
import PaginationBar from '../../../components/PaginationBar.vue'
import { ensureXrayRunning, notifyXrayRuntimeApplyFailure } from '../../../utils/xrayRuntime'
const { t } = useI18n()
const props = defineProps({ data: Object, coreType: String, serverId: Number })
const emitFn = defineEmits(['update'])

const nodeBase = computed(() => `/node-servers/${props.serverId}`)

const form = reactive({ domainStrategy: 'IPIfNonMatch', rules: [], balancers: [] })

const { paged: rulesPaged, page: rulePage, pageSize: ruleSize, total: ruleTotal } = useClientPagination(computed(() => form.rules), { pageKey: 'xbRulePage', pageSizeKey: 'xbRuleSize' })
const { paged: balPaged, page: balPage, pageSize: balSize, total: balTotal } = useClientPagination(computed(() => form.balancers), { pageKey: 'xbBalPage', pageSizeKey: 'xbBalSize' })
const outboundTags = ref(['direct', 'blocked'])
const inboundTags = ref([])
const userTags = ref([])
const geoStatus = ref({ 'geoip.dat': false, 'geosite.dat': false })
const builtinOutboundTags = ['direct', 'blocked']
const activeOutboundTags = computed(() => [...new Set(outboundTags.value.filter(Boolean))])
const activeInboundTags = computed(() => [...new Set(inboundTags.value.filter(Boolean))])
const activeBalancers = computed(() => form.balancers.filter(b => b.enabled !== false && b.tag))

const dlg = ref(false)
const editIdx = ref(-1)
const balDlg = ref(false)
const balIdx = ref(-1)
const bf = reactive({ tag: '', selectorArr: [], fallbackTag: '', strategyType: 'random', _expected: 0, _maxRTT: '', _tolerance: 0, _baselinesStr: '', _costsJson: '' })
const rf = reactive({
  ruleTag: '',
  _domain: [], _ip: [], _port: '', _sourcePort: '', _localPort: '',
  _network: '', _protocol: [], _sourceIP: [], _localIP: [],
  _inboundTag: [],
  _process: [], _user: [],
  _attrs: [], _vlessRoute: '',
  _webhookUrl: '', _webhookDedup: 0, _webhookHeaders: [],
  outboundTag: 'direct', _actionInboundTag: [], balancerTag: '',
})

function genTag() { return 'rule-' + Math.random().toString(36).substring(2, 8) }

function resetForm() {
  Object.assign(rf, {
    ruleTag: genTag(),
    _domain: [], _ip: [], _port: '', _sourcePort: '', _localPort: '',
    _network: '', _protocol: [], _sourceIP: [], _localIP: [],
    _inboundTag: [],
    _process: [], _user: [], _localOS: [], _localOS: [],
    _attrs: [], _vlessRoute: '',
    _webhookUrl: '', _webhookDedup: 0, _webhookHeaders: [],
    outboundTag: 'direct', _actionInboundTag: [], balancerTag: '',
  })
}

function showAddRule() { editIdx.value = -1; resetForm(); dlg.value = true }

function editRule(r) {
  editIdx.value = form.rules.indexOf(r)

  const attrs = r.attrs ? Object.entries(r.attrs).map(([k,v]) => ({key:k,value:v})) : []
  const whHeaders = r.webhook && r.webhook.headers ? Object.entries(r.webhook.headers).map(([k,v]) => ({key:k,value:v})) : []

  Object.assign(rf, {
    ruleTag: r.ruleTag || genTag(),
    _domain: r.domain || [], _ip: r.ip || [],
    _port: r.port || '', _sourcePort: r.sourcePort || '', _localPort: r.localPort || '',
    _network: r.network || '', _protocol: r.protocol || [],
    _sourceIP: r.sourceIP || r.source || [], _localIP: r.localIP || [],
    _inboundTag: r.inboundTag || [],
    _process: r.process || [], _user: r.user || [], _localOS: r.localOS || [],
    _attrs: attrs, _vlessRoute: r.vlessRoute || '',
    _webhookUrl: r.webhook ? r.webhook.url || '' : '',
    _webhookDedup: r.webhook ? r.webhook.deduplication || 0 : 0,
    _webhookHeaders: whHeaders,
    outboundTag: r.outboundTag || '',
    balancerTag: r.balancerTag || '',
  })
  dlg.value = true
}

function saveRule() {
  const rule = { type: 'field', enabled: true, ruleTag: rf.ruleTag }
  if (rf.outboundTag) rule.outboundTag = rf.outboundTag
  if (rf.balancerTag) rule.balancerTag = rf.balancerTag
  if (rf._domain.length) rule.domain = [...rf._domain]
  if (rf._ip.length) rule.ip = [...rf._ip]
  if (rf._port) rule.port = rf._port
  if (rf._sourcePort) rule.sourcePort = rf._sourcePort
  if (rf._localPort) rule.localPort = rf._localPort
  if (rf._network) rule.network = rf._network
  if (rf._protocol.length) rule.protocol = [...rf._protocol]
  if (rf._sourceIP.length) rule.sourceIP = [...rf._sourceIP]
  if (rf._localIP.length) rule.localIP = [...rf._localIP]
  if (rf._inboundTag.length) rule.inboundTag = [...rf._inboundTag]
  if (rf._process.length) rule.process = [...rf._process]
  if (rf._localOS && rf._localOS.length) rule.localOS = [...rf._localOS]
  if (rf._user.length) rule.user = [...rf._user]
  if (rf._attrs.length) {
    const attrs = {}; rf._attrs.forEach(a => { if (a.key) attrs[a.key] = a.value })
    if (Object.keys(attrs).length) rule.attrs = attrs
  }
  if (rf._vlessRoute) rule.vlessRoute = rf._vlessRoute
  if (rf._webhookUrl) {
    const wh = { url: rf._webhookUrl }; if (rf._webhookDedup) wh.deduplication = rf._webhookDedup
    if (rf._webhookHeaders.length) { const h = {}; rf._webhookHeaders.forEach(x => { if (x.key) h[x.key] = x.value }); if (Object.keys(h).length) wh.headers = h }
    rule.webhook = wh
  }
  if (editIdx.value >= 0) form.rules[editIdx.value] = rule
  else form.rules.push(rule)
  dlg.value = false
  emit()
}

function showAddBalancer() {
  balIdx.value = -1
  Object.assign(bf, { tag: 'balancer-' + Math.random().toString(36).substring(2, 6), selectorArr: [], fallbackTag: '', strategyType: 'random', _expected: 0, _maxRTT: '', _tolerance: 0, _baselinesStr: '', _costsJson: '' })
  balDlg.value = true
}
function editBalancer(b) {
  balIdx.value = form.balancers.indexOf(b); const s = b.settings || {}
  Object.assign(bf, { tag: b.tag, selectorArr: [...(b.selectorArr || [])], fallbackTag: b.fallbackTag || '', strategyType: b.strategyType, _expected: s.expected || 0, _maxRTT: s.maxRTT || '', _tolerance: s.tolerance || 0, _baselinesStr: (s.baselines || []).join(', '), _costsJson: (s.costs && s.costs.length) ? JSON.stringify(s.costs, null, 2) : '' })
  balDlg.value = true
}
function saveBalancer() {
  if (!bf.selectorArr.length) {
    ElMessage.error(t('protoCenter.routing.balancerSelectorRequired'))
    return
  }
  const bal = { tag: bf.tag, selectorArr: [...bf.selectorArr], fallbackTag: bf.fallbackTag, strategyType: bf.strategyType, settings: null }
  if (bf.strategyType === 'leastLoad') {
    const s = {}; if (bf._expected) s.expected = bf._expected; if (bf._maxRTT) s.maxRTT = bf._maxRTT
    if (bf._tolerance) s.tolerance = bf._tolerance; if (bf._baselinesStr) s.baselines = bf._baselinesStr.split(',').map(x => x.trim()).filter(Boolean)
    if (bf._costsJson) {
      try {
        const costs = JSON.parse(bf._costsJson)
        if (Array.isArray(costs) && costs.length) s.costs = costs
      } catch (e) { console.warn('costs JSON 解析失败，已忽略:', e) }
    }
    if (Object.keys(s).length) bal.settings = s
  }
  if (balIdx.value >= 0) form.balancers[balIdx.value] = bal; else form.balancers.push(bal)
  balDlg.value = false; emit()
}
function rmBalancer(b) { const i = form.balancers.indexOf(b); form.balancers.splice(i, 1); emit() }

async function loadTags() {
  try {
    const res = await client.get(`/node-servers/${props.serverId}/outbound-configs`, { params: { core_type: props.coreType || 'xray' } })
    const obs = (res || []).filter(o => o?.enabled !== false)
    outboundTags.value = [...new Set([...builtinOutboundTags, ...obs.map(o => o.tag).filter(Boolean)])]
  } catch {}
  try {
    const ibs = await client.get(`/node-servers/${props.serverId}/inbound-configs`, { params: { core_type: props.coreType || 'xray' } })
    inboundTags.value = (ibs || []).filter(i => i?.enabled !== false).map(i => i.tag).filter(Boolean)
  } catch {}
}
async function loadGeoStatus() { try { const d = await client.get('/geo-rules/status'); if (d) geoStatus.value = d } catch {} }

function sync() {
  if (!props.data) return
  form.domainStrategy = props.data.domainStrategy || 'IPIfNonMatch'
  form.rules = (props.data.rules || []).map(r => ({ ...r, enabled: r.enabled !== false }))
  form.balancers = (props.data.balancers || []).map(b => ({ tag: b.tag || '', selectorArr: b.selector || [], fallbackTag: b.fallbackTag || '', strategyType: b.strategy?.type || 'random', settings: b.strategy?.settings || null, enabled: b.enabled !== false }))
}
function build() {
  return {
    domainStrategy: form.domainStrategy,
    rules: form.rules.filter(r => r.enabled !== false).map(r => { const rule = { ...r }; delete rule.enabled; return rule }),
    balancers: form.balancers.filter(b => b.enabled !== false && b.tag).map(b => { const bal = { tag: b.tag, selector: b.selectorArr || [], strategy: { type: b.strategyType } }; if (b.fallbackTag) bal.fallbackTag = b.fallbackTag; if (b.settings) bal.strategy.settings = b.settings; return bal }),
  }
}

function getInvalidBalancer(routing) {
  return (routing?.balancers || []).find(b => !Array.isArray(b.selector) || b.selector.length === 0)
}

async function emit() {
  const data = build()
  emitFn('update', data)
  // 无节点上下文（如订阅格式编辑复用本组件）时只更新表单，不写节点侧数据
  if (!props.serverId) return
  // Save to database and apply to xray runtime
  await saveAndApply(data)
}
function rmRule(r) { const i = form.rules.indexOf(r); form.rules.splice(i, 1); emit() }

async function saveAndApply(routing) {
  const invalidBalancer = getInvalidBalancer(routing)
  if (invalidBalancer) {
    ElMessage.error(t('protoCenter.routing.balancerSelectorInvalid', { tag: invalidBalancer.tag || t('protoCenter.routing.unnamed') }))
    return
  }
  try {
    // Save to database
    await client.put(`/node-servers/${props.serverId}/protocols/${props.coreType || 'xray'}/section/routing`, routing)
  } catch (e) {
    console.warn('Failed to save routing to database:', e)
    return
  }
  try {
    if (!(await ensureXrayRunning(client, props.serverId, t('protoCenter.routing.savedToDb')))) {
      ElMessage.success(t('common.success'))
      return
    }
    // Apply to xray runtime
    await client.post(`${nodeBase.value}/xray/control/routing`, { routing }, { suppressErrorMessage: true })
  } catch (e) {
    console.warn('Failed to apply routing to xray:', e)
    notifyXrayRuntimeApplyFailure(e, t('protoCenter.routing.savedToDb'))
    return
  }
  ElMessage.success(t('common.success'))
}

onMounted(function() {
  sync()
  loadGeoStatus()
  // 无节点上下文（如订阅格式编辑复用本组件）时不去拉节点侧数据
  if (props.serverId) { loadTags(); loadChainProxy() }
})

// Chain proxy auto-injected content (read-only, from final xray config)
const chainRules = ref([])
const chainBalancers = ref([])
const chainOutbounds = ref([])

async function loadChainProxy() {
  try {
    // Fetch the final xray config (includes chain proxy auto-injected content)
    const data = await client.get(`/node-servers/${props.serverId}/protocols/${props.coreType || 'xray'}/config-file`, { suppressErrorMessage: true })
    if (!data) return
    const cfg = typeof data === 'string' ? JSON.parse(data) : data
    // Extract chain proxy outbounds (tag starts with 'chain-')
    const allObs = cfg.outbounds || []
    chainOutbounds.value = allObs.filter(ob => {
      const tag = ob?.tag || ''
      return tag.startsWith('chain-')
    }).map(ob => {
      let server = '', port = ''
      const vnext = ob?.settings?.vnext?.[0]
      if (vnext) { server = vnext.address || ''; port = vnext.port || '' }
      const servers = ob?.settings?.servers?.[0]
      if (servers) { server = servers.address || ''; port = servers.port || '' }
      return { ...ob, _server: server, _port: port }
    })
    // Extract chain proxy routing rules (ruleTag starts with 'exit-')
    const routing = cfg.routing || {}
    const allRules = routing.rules || []
    chainRules.value = allRules.filter(r => {
      const rt = r?.ruleTag || ''
      return rt.startsWith('exit-')
    })
    // Extract chain proxy balancers (tag starts with 'bal-chain-')
    chainBalancers.value = (routing.balancers || []).filter(b => {
      const tag = b?.tag || ''
      return tag.startsWith('bal-chain-')
    })
  } catch {
    chainRules.value = []
    chainBalancers.value = []
    chainOutbounds.value = []
  }
}
</script>

<style scoped>
.rule-item { background: var(--el-fill-color-light); border-radius: 6px; padding: 12px; margin-bottom: 8px; }
.chain-item { background: var(--el-color-warning-light-9); border: 1px dashed var(--el-color-warning-light-5); }
.hint { font-size: 12px; color: var(--el-text-color-secondary); margin-top: 2px; }
</style>
