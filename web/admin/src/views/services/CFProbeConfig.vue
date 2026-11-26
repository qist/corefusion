<template>
  <!-- 优选探测设置：CF 优选域名 / CF IP 池 共用同一份后端配置 -->
  <div>
  <el-card style="margin-bottom:16px">
    <template #header>
      <div style="display:flex;justify-content:space-between;align-items:center">
        <span style="font-weight:600">{{ t('service.cfip.probeTitle') }}</span>
        <div style="display:flex;gap:8px">
          <el-button size="small" type="primary" @click="saveProbeConfig">{{ t('common.save') }}</el-button>
        </div>
      </div>
    </template>
    <div style="display:flex;gap:16px;flex-wrap:wrap;align-items:center">
      <div>
        <div style="font-size:12px;color:#909399">{{ t('service.cfip.probeEnabled') }}</div>
        <el-switch v-model="probeCfg.enabled" />
      </div>
      <div>
        <div style="font-size:12px;color:#909399">{{ t('service.cfip.probeInterval') }}</div>
        <el-input-number v-model="probeCfg.interval_minutes" :min="1" :max="1440" size="small" style="width:130px" :disabled="!probeCfg.enabled" />
      </div>
      <div>
        <div style="font-size:12px;color:#909399">{{ t('service.cfip.probeMaxLatency') }}</div>
        <el-input-number v-model="probeCfg.max_latency_ms" :min="0" :max="60000" :step="50" size="small" style="width:150px" />
      </div>
      <div>
        <div style="font-size:12px;color:#909399">{{ t('service.cfip.chainAgent') }}</div>
        <el-select v-model="probeCfg.agent_id" size="small" style="width:220px" @change="onProbeAgentChange">
          <el-option :label="t('service.cfip.chainAgentPanel')" :value="0" />
          <el-option v-for="a in probeAgents" :key="a.id" :label="a.name" :value="a.id" />
        </el-select>
      </div>
      <div>
        <div style="font-size:12px;color:#909399">{{ t('service.cfip.chainExitInterface') }}</div>
        <el-select v-model="probeCfg.interface" size="small" style="width:180px" clearable filterable allow-create default-first-option :placeholder="t('service.cfip.chainExitInterfacePlaceholder')">
          <el-option v-for="i in probeInterfaces" :key="i.name" :label="i.label" :value="i.name" />
        </el-select>
      </div>
      <div>
        <div style="font-size:12px;color:#909399">{{ t('service.cfip.probeEntryNode') }}</div>
        <el-select v-model="probeCfg.entry" size="small" style="width:240px" filterable :placeholder="t('service.cfip.probeEntryRequired')">
          <el-option-group v-if="extNodes.length" :label="t('service.cfip.probeEntryExternal')">
            <el-option
              v-for="n in extNodes"
              :key="n.value"
              :label="`${n.name || n.host} (${n.protocol || ''})`"
              :value="n.value"
            />
          </el-option-group>
          <el-option-group v-if="inbNodes.length" :label="t('service.cfip.probeEntryInbound')">
            <el-option
              v-for="n in inbNodes"
              :key="n.value"
              :label="`${n.host ? n.host + ' / ' : ''}${n.name} (${n.protocol || ''})`"
              :value="n.value"
            />
          </el-option-group>
        </el-select>
      </div>
      <div>
        <div style="font-size:12px;color:#909399">{{ t('service.cfip.probeDialer') }}</div>
        <el-select v-model="probeCfg.dialer" size="small" style="width:210px" @change="onDialerChange">
          <el-option :label="t('service.cfip.probeAuto')" value="" />
          <el-option :label="dialerLabel('xray')" value="xray" />
          <el-option :label="dialerLabel('singbox')" value="singbox" />
          <el-option :label="dialerLabel('mihomo')" value="mihomo" />
        </el-select>
        <!-- 已安装的核心也要能换版本/重装：不能只在「未安装」时才给入口 -->
        <el-button
          v-if="probeCfg.dialer"
          size="small"
          style="margin-left:8px"
          @click="openDialerInstall(probeCfg.dialer, 'probe')"
        >
          {{ isDialerInstalled(probeCfg.dialer) ? t('service.cfip.dialerReinstallBtn') : t('service.cfip.dialerInstallBtn') }}
        </el-button>
      </div>
    </div>
    <div v-if="probeCfg.agent_id" style="font-size:11px;color:#e6a23c;margin-top:8px">{{ t('service.cfip.probeAgentDialerHint') }}</div>
    <div v-if="probeAgentDialerError" style="font-size:11px;color:#f56c6c;margin-top:4px">{{ probeAgentDialerError }}</div>
    <div v-if="chainCfg.enabled" style="font-size:11px;color:#e6a23c;margin-top:8px">{{ t('service.cfip.probeChainOwnsHealth') }}</div>
    <div style="font-size:11px;color:#909399;margin-top:8px">{{ t('service.cfip.probeHint') }}</div>
    <div style="font-size:11px;color:#909399;margin-top:4px">{{ t('service.cfip.probeMaxLatencyHint') }}</div>
    <div v-if="!probeCfg.enabled" style="font-size:11px;color:#e6a23c;margin-top:4px">{{ t('service.cfip.probeEnabledHint') }}</div>
  </el-card>

  <!-- 链路探测：国内出口 → CDN 域名/IP → 入站，验证整条链路真能代理流量 -->
  <el-card style="margin-bottom:16px">
    <template #header>
      <div style="display:flex;justify-content:space-between;align-items:center">
        <span style="font-weight:600">{{ t('service.cfip.chainTitle') }}</span>
        <el-button size="small" type="primary" @click="saveChainConfig">{{ t('common.save') }}</el-button>
      </div>
    </template>
    <div style="display:flex;gap:16px;flex-wrap:wrap;align-items:center">
      <div>
        <div style="font-size:12px;color:#909399">{{ t('service.cfip.chainEnabled') }}</div>
        <el-switch v-model="chainCfg.enabled" />
      </div>
      <div>
        <div style="font-size:12px;color:#909399">{{ t('service.cfip.chainInheritProbe') }}</div>
        <el-switch v-model="chainCfg.inherit_probe" />
      </div>
      <div v-if="!chainCfg.inherit_probe">
        <div style="font-size:12px;color:#909399">{{ t('service.cfip.probeInterval') }}</div>
        <el-input-number v-model="chainCfg.interval_minutes" :min="1" :max="1440" size="small" style="width:130px" />
      </div>
      <div v-if="!chainCfg.inherit_probe">
        <div style="font-size:12px;color:#909399">{{ t('service.cfip.probeDialer') }}</div>
        <el-select v-model="chainCfg.dialer" size="small" style="width:200px" @change="onChainDialerChange">
          <el-option :label="t('service.cfip.probeAuto')" value="" />
          <el-option :label="dialerLabel('xray', chainDialerAvailable, chainDialerVersions)" value="xray" />
          <el-option :label="dialerLabel('singbox', chainDialerAvailable, chainDialerVersions)" value="singbox" />
          <el-option :label="dialerLabel('mihomo', chainDialerAvailable, chainDialerVersions)" value="mihomo" />
        </el-select>
        <el-button
          v-if="chainCfg.dialer"
          size="small"
          style="margin-left:8px"
          @click="openDialerInstall(chainCfg.dialer, 'chain')"
        >
          {{ chainDialerAvailable.includes(chainCfg.dialer) ? t('service.cfip.dialerReinstallBtn') : t('service.cfip.dialerInstallBtn') }}
        </el-button>
      </div>
      <div>
        <div style="font-size:12px;color:#909399">{{ t('service.cfip.chainAgent') }}</div>
        <el-select v-model="chainCfg.agent_id" size="small" style="width:220px" @change="onAgentChange">
          <el-option :label="t('service.cfip.chainAgentPanel')" :value="0" />
          <el-option v-for="a in chainAgents" :key="a.id" :label="a.name" :value="a.id" />
        </el-select>
      </div>
      <div>
        <div style="font-size:12px;color:#909399">{{ t('service.cfip.chainExit') }}</div>
        <el-select v-model="chainCfg.exit" size="small" style="width:300px" filterable clearable :placeholder="t('service.cfip.chainExitPlaceholder')">
          <el-option-group :label="t('service.cfip.chainExitExternal')">
            <el-option v-for="o in externalExits" :key="o.value" :label="o.label" :value="o.value" />
          </el-option-group>
          <el-option-group :label="t('service.cfip.chainExitInbound')">
            <el-option v-for="o in inboundExits" :key="o.value" :label="o.label" :value="o.value" />
          </el-option-group>
        </el-select>
      </div>
      <div>
        <div style="font-size:12px;color:#909399">{{ t('service.cfip.chainExitInterface') }}</div>
        <el-select v-model="chainCfg.interface" size="small" style="width:180px" clearable filterable allow-create default-first-option :placeholder="t('service.cfip.chainExitInterfacePlaceholder')">
          <el-option v-for="i in chainInterfaces" :key="i.name" :label="i.label" :value="i.name" />
        </el-select>
      </div>
      <div>
        <div style="font-size:12px;color:#909399">{{ t('service.cfip.chainInbound') }}</div>
        <el-select v-model="chainCfg.inbound" size="small" style="width:300px" filterable clearable :placeholder="t('service.cfip.chainInboundPlaceholder')">
          <el-option-group v-if="chainInboundExt.length" :label="t('service.cfip.probeEntryExternal')">
            <el-option v-for="o in chainInboundExt" :key="o.value" :label="o.label" :value="o.value">
              <span>{{ o.label }}</span>
              <span style="float:right;color:#909399;font-size:12px">{{ o.access_domain || o.access_type }}</span>
            </el-option>
          </el-option-group>
          <el-option-group v-if="chainInboundInb.length" :label="t('service.cfip.probeEntryInbound')">
            <el-option v-for="o in chainInboundInb" :key="o.value" :label="o.label" :value="o.value">
              <span>{{ o.label }}</span>
              <span style="float:right;color:#909399;font-size:12px">{{ o.access_domain || o.access_type }}</span>
            </el-option>
          </el-option-group>
        </el-select>
      </div>
      <div>
        <div style="font-size:12px;color:#909399">{{ t('service.cfip.chainConcurrency') }}</div>
        <el-input-number v-model="chainCfg.concurrency" :min="1" :max="8" size="small" style="width:120px" />
      </div>
      <div style="min-width:320px;flex:1">
        <div style="font-size:12px;color:#909399">{{ t('service.cfip.chainProbeURL') }}</div>
        <el-input v-model="chainCfg.probe_url" size="small" placeholder="https://www.cloudflare.com/cdn-cgi/trace" />
      </div>
    </div>
    <div style="font-size:11px;color:#909399;margin-top:8px">{{ t('service.cfip.chainHint') }}</div>
    <div style="font-size:11px;color:#909399;margin-top:4px">{{ t('service.cfip.chainExitInterfaceHint') }}</div>
    <div style="font-size:11px;color:#909399;margin-top:4px">{{ t('service.cfip.chainAgentHint') }}</div>
    <div style="font-size:11px;color:#909399;margin-top:4px">{{ t('service.cfip.chainInboundHint') }}</div>
    <div v-if="chainDialerError" style="font-size:11px;color:#f56c6c;margin-top:4px">{{ chainDialerError }}</div>
    <div v-if="chainCfg.inherit_probe" style="font-size:11px;color:#909399;margin-top:4px">
      {{ t('service.cfip.chainDependsProbe') }}：{{ t('service.cfip.chainProbeInterval') }} {{ chainDeps.interval_minutes }} min
      · {{ t('service.cfip.chainProbeDialer') }} {{ chainDeps.effective_dialer || '-' }}
      <span v-if="!chainDeps.dialer">（{{ t('service.cfip.chainDialerAuto') }}）</span>
    </div>
    <div v-else style="font-size:11px;color:#909399;margin-top:4px">
      {{ t('service.cfip.chainIndependent') }}：{{ t('service.cfip.chainProbeInterval') }} {{ chainCfg.interval_minutes }} min
      · {{ t('service.cfip.chainProbeDialer') }} {{ chainCfg.dialer || t('service.cfip.chainDialerAuto') }}
    </div>
    <div style="font-size:11px;color:#e6a23c;margin-top:4px">{{ t('service.cfip.chainAccountHint') }}</div>
    <div style="font-size:11px;color:#606266;margin-top:6px">{{ t('service.cfip.chainSplitHint') }}</div>

    <!-- 域名 / IP 各自独立：开关、单轮名额、检测按钮、结果表全部分开，数据不会互相串 -->
    <div v-for="sec in chainSections" :key="sec.key" class="chain-kind-block">
      <div class="chain-kind-head">
        <span class="chain-kind-title">{{ sec.title }}</span>
        <el-switch v-model="chainCfg[sec.enabledKey]" />
        <span class="chain-kind-label">{{ t('service.cfip.chainMaxTargets') }}</span>
        <el-input-number v-model="chainCfg[sec.maxKey]" :min="1" :max="500" size="small" style="width:120px" />
        <el-button size="small" :loading="chainChecking[sec.key]" @click="checkChain(sec.key)">
          {{ sec.checkLabel }}
        </el-button>
        <span class="chain-kind-meta">
          {{ t('service.cfip.chainTargetCount') }}：{{ sec.total }}<template v-if="sec.healthy != null"> · {{ t('service.cfip.chainHealthy') }} {{ sec.healthy }}</template>
          <template v-if="sec.updatedAt"> · {{ t('onlineUsers.updatedAt') }} {{ formatTime(sec.updatedAt) }}</template>
        </span>
      </div>
      <div class="chain-kind-meta" style="margin:6px 0 8px">
        {{ t('service.cfip.chainProbeLabel') }}：{{ sec.probe || chainProbeLabel }}
      </div>
      <el-table v-if="sec.results.length" :data="sec.results" size="small" border stripe max-height="360">
        <el-table-column :label="t('service.cfip.chainResultStatus')" width="90">
          <template #default="{ row }">
            <el-tag :type="row.healthy ? 'success' : 'danger'" size="small">
              {{ row.healthy ? t('service.cfip.chainHealthy') : t('service.cfip.chainUnhealthy') }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="t('service.cfip.chainResultCDN')" min-width="220" show-overflow-tooltip>
          <template #default="{ row }">
            <span style="font-family:monospace">{{ row.host }}:{{ row.port }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="t('service.cfip.chainResultProbe')" min-width="200" show-overflow-tooltip>
          <template #default="{ row }">
            <span>{{ row.name || ('#' + row.probe_inbound_id) }}</span>
            <span style="color:#909399;font-size:12px"> · {{ row.protocol }}</span>
            <span v-if="row.node_name" style="color:#c0c4cc;font-size:12px"> @{{ row.node_name }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="t('service.cfip.chainResultHTTP')" width="80" align="center">
          <template #default="{ row }">{{ row.http_status || '-' }}</template>
        </el-table-column>
        <el-table-column :label="t('service.cfip.chainResultLatency')" width="90" align="right">
          <template #default="{ row }">{{ row.latency_ms ? row.latency_ms + ' ms' : '-' }}</template>
        </el-table-column>
        <el-table-column :label="t('service.cfip.chainResultUp')" width="90" align="right">
          <template #default="{ row }">{{ formatBytes(row.up_bytes) }}</template>
        </el-table-column>
        <el-table-column :label="t('service.cfip.chainResultDown')" width="90" align="right">
          <template #default="{ row }">{{ formatBytes(row.down_bytes) }}</template>
        </el-table-column>
        <el-table-column :label="t('service.cfip.chainResultError')" min-width="160" show-overflow-tooltip>
          <template #default="{ row }"><span style="color:#f56c6c">{{ row.error || '' }}</span></template>
        </el-table-column>
        <el-table-column :label="t('service.cfip.chainResultCheckedAt')" width="160">
          <template #default="{ row }">{{ formatTime(row.checked_at) }}</template>
        </el-table-column>
      </el-table>
      <el-empty v-else :description="t('service.cfip.chainNoResult')" :image-size="60" />
    </div>
  </el-card>

  <!-- 拨号核心未安装：选版本直接安装（面板本机直接装；远程节点由 Agent 下载安装） -->
  <el-dialog
    v-model="dialerInstallVisible"
    :title="t('service.cfip.installCoreTitle', { core: dialerInstallCore })"
    width="620"
    :close-on-click-modal="false"
  >
    <div style="margin-bottom:6px;font-size:12px;color:#606266">
      {{ t('service.cfip.chainAgent') }}：
      <span style="font-weight:500">{{ dialerInstallTargetName }}</span>
    </div>
    <div style="margin-bottom:10px;font-size:12px;color:#606266">
      {{ t('service.cfip.dialerCurrentVersion') }}：
      <span v-if="dialerVersionText(dialerInstallCore, dialerInstallVersions)" style="font-family:monospace">{{ dialerVersionText(dialerInstallCore, dialerInstallVersions) }}</span>
      <span v-else style="color:#e6a23c">{{ t('service.cfip.dialerNotInstalled') }}</span>
    </div>
    <div v-loading="dialerReleasesLoading || dialerInstalling" style="min-height:120px">
      <div v-if="dialerReleases.length === 0 && !dialerReleasesLoading" style="text-align:center;color:#999;padding:20px">
        {{ t('common.noData') }}
      </div>
      <div style="display:flex;flex-wrap:wrap;gap:8px">
        <div
          v-for="r in dialerReleases"
          :key="r.version"
          style="width:calc(16.66% - 8px);min-width:96px;padding:8px;border:1px solid #dcdfe6;border-radius:6px;cursor:pointer;text-align:center;transition:all .2s"
          :style="{ borderColor: r.prerelease ? '#67c23a' : '#409eff', background: r.prerelease ? '#f0f9eb' : '#ecf5ff' }"
          @click="installDialerVersion(r)"
        >
          <div style="font-size:11px;margin-bottom:2px" :style="{ color: r.prerelease ? '#67c23a' : '#409eff' }">
            {{ r.prerelease ? 'Pre' : 'Rel' }}
          </div>
          <div style="font-size:12px;font-weight:500;color:#303133">{{ r.version }}</div>
        </div>
      </div>
      <div v-if="dialerInstalling" style="margin-top:12px;text-align:center;font-size:12px;color:#909399">
        {{ t('service.cfip.installCoreInstalling', { core: dialerInstallCore }) }}
      </div>
    </div>
    <template #footer>
      <el-button :disabled="dialerInstalling" @click="dialerInstallVisible = false">{{ t('common.cancel') }}</el-button>
    </template>
  </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import client from '../../api/client'
import { useAutoRefresh, notifyCfipRefresh, CFIP_REFRESH_EVENT } from '../../composables/useAutoRefresh'

const { t } = useI18n()

const probeCfg = ref({ enabled: false, interval_minutes: 5, max_latency_ms: 0, entry: '', entry_node_id: 0, dialer: '', agent_id: 0, interface: '', available_dialers: [], dialer_versions: {} })
const probeNodes = ref([])
const probeAgents = ref([])
const probeInterfaces = ref([])
let probeLocalInterfaces = []
const extNodes = computed(() => probeNodes.value.filter((n) => n.kind === 'external'))
const inbNodes = computed(() => probeNodes.value.filter((n) => n.kind === 'inbound'))

const chainCfg = ref({
  enabled: false,
  inherit_probe: true,
  interval_minutes: 5,
  dialer: '',
  agent_id: 0,
  exit: '',
  interface: '',
  inbound: '',
  concurrency: 2,
  domain_enabled: true,
  ip_enabled: true,
  domain_max_targets: 20,
  ip_max_targets: 20,
  probe_url: 'https://www.cloudflare.com/cdn-cgi/trace',
  // 告诉后端本次请求显式带了分类型开关/名额（旧前端不会带）
  kinds_configured: true,
})
// 域名 / IP 分开保存结果与状态：两类互不影响，页面不会相互串数据。
const chainResults = ref({ domain: [], ip: [] })
const chainUpdatedAt = ref({ domain: 0, ip: 0 })
const chainExits = ref([])
const chainInterfaces = ref([])
const chainAgents = ref([])
// 链路探测自己的执行节点上装了哪些核心（和探测设置互相独立）。
const chainDialerAvailable = ref([])
const chainDialerVersions = ref({})
const chainDialerError = ref('')
let chainPreviousDialer = ''
let chainLocalInterfaces = []
// 探测设置执行节点的核心查询失败原因（节点离线/未升级 Agent 等）。
const probeAgentDialerError = ref('')
const chainInbounds = ref([])
const chainTargets = ref([])
const chainChecking = ref({ domain: false, ip: false })
// 最近一轮实际使用的探针入站（目标列表按当前选择生成，这里记录真正跑过的那条）。
const chainProbe = ref({ domain: '', ip: '' })
// 链路探测依赖「探测设置」的间隔与拨号核心，这两项只读展示。
const chainDeps = ref({ interval_minutes: 0, dialer: '', effective_dialer: '' })

// 拨号核心未安装 → 选版本安装
const dialerInstallVisible = ref(false)
const dialerInstallCore = ref('')
const dialerReleases = ref([])
const dialerReleasesLoading = ref(false)
const dialerInstalling = ref(false)
// 安装弹窗作用于哪张卡片（probe / chain）与哪台执行节点：核心必须装到执行节点上。
const dialerInstallScope = ref('probe')
const dialerInstallAgentId = ref(0)
const dialerInstallTargetName = computed(() => {
  const id = dialerInstallAgentId.value
  if (!id) return t('service.cfip.chainAgentPanel')
  const a = chainAgents.value.find((x) => x.id === id)
  return a ? a.name : `#${id}`
})
const dialerInstallVersions = computed(() =>
  dialerInstallScope.value === 'chain' ? chainDialerVersions.value : probeCfg.value.dialer_versions,
)
// 记住已安装/上次选择，取消安装时回退，避免把"未安装核心"保存进配置。
let previousDialer = ''

const chainTargetCounts = computed(() => {
  const counts = { domain: 0, ip: 0, domainHealthy: 0, ipHealthy: 0 }
  for (const tg of chainTargets.value) {
    // 旧后端没有 healthy 字段时按健康算，避免页面显示成 0。
    const ok = tg.healthy !== false
    if (tg.kind === 'cf_domain') {
      counts.domain++
      if (ok) counts.domainHealthy++
    } else if (tg.kind === 'cf_ip') {
      counts.ip++
      if (ok) counts.ipHealthy++
    }
  }
  return counts
})
// 两个独立区块：域名 / IP 各自开关、名额、检测按钮、结果表。
const chainSections = computed(() => [
  {
    key: 'domain',
    title: t('service.cfip.chainDomainTitle'),
    checkLabel: t('service.cfip.chainDomainCheck'),
    enabledKey: 'domain_enabled',
    maxKey: 'domain_max_targets',
    results: chainResults.value.domain,
    updatedAt: chainUpdatedAt.value.domain,
    probe: chainProbe.value.domain,
    total: chainTargetCounts.value.domain,
    healthy: chainTargetCounts.value.domainHealthy,
  },
  {
    key: 'ip',
    title: t('service.cfip.chainIPTitle'),
    checkLabel: t('service.cfip.chainIPCheck'),
    enabledKey: 'ip_enabled',
    maxKey: 'ip_max_targets',
    results: chainResults.value.ip,
    updatedAt: chainUpdatedAt.value.ip,
    probe: chainProbe.value.ip,
    total: chainTargetCounts.value.ip,
    healthy: chainTargetCounts.value.ipHealthy,
  },
])
const externalExits = computed(() => chainExits.value.filter((o) => o.kind === 'external'))
const inboundExits = computed(() => chainExits.value.filter((o) => o.kind === 'inbound'))
// 探针入站支持两类入口：外部导入节点（ext:）与节点侧入站（inb:）。
const chainInboundExt = computed(() => chainInbounds.value.filter((o) => String(o.value).startsWith('ext:')))
const chainInboundInb = computed(() => chainInbounds.value.filter((o) => String(o.value).startsWith('inb:')))
// 最近一轮实际使用的探针入站（没跑过时退回当前选择）。
const chainProbeLabel = computed(() => {
  const probe = chainProbe.value.domain || chainProbe.value.ip
  if (probe) return probe
  const picked = chainInbounds.value.find((o) => o.value === chainCfg.value.inbound)
  return picked ? picked.label : '-'
})

// form=false 只刷新候选/结果，不回写表单：自动刷新不能覆盖用户正在改的配置。
async function fetchProbeConfig(opts = {}) {
  const { form = true } = opts
  try {
    const d = await client.get('/cfip/probe-config') || {}
    if (form) {
      probeCfg.value = {
        enabled: !!d.enabled,
        interval_minutes: d.interval_minutes ?? 5,
        max_latency_ms: d.max_latency_ms ?? 0,
        entry: d.entry || (d.entry_node_id ? `ext:${d.entry_node_id}` : ''),
        entry_node_id: d.entry_node_id ?? 0,
        dialer: d.dialer || '',
        agent_id: Number(d.agent_id || 0),
        interface: d.interface || '',
        available_dialers: d.dialers || [],
        dialer_versions: d.dialer_versions || {},
      }
      probeLocalInterfaces = (d.interfaces || []).map((i) => ({
        name: i.name,
        label: i.up ? i.name : `${i.name} (down)`,
      }))
      probeInterfaces.value = probeLocalInterfaces
      probeAgents.value = d.agents || []
      probeAgentDialerError.value = d.dialer_error || ''
      probeCfg.value.available_dialers = d.dialers || []
      probeCfg.value.dialer_versions = d.dialer_versions || {}
      // 已保存的核心可能已被人删掉：不能把它当成"可回退值"，否则取消安装后
      // 保存的仍是不可用核心。回退到自动（自动只会选已安装的）。
      const installed = probeCfg.value.available_dialers || []
      previousDialer = installed.includes(probeCfg.value.dialer) ? probeCfg.value.dialer : ''
    }
    // 候选来自后端：外部节点 + 节点侧入站（如 VLESSGRPC），前端不再单独拼。
    probeNodes.value = Array.isArray(d.nodes) ? d.nodes : []
  } catch { /* 未配置时用默认值（默认关闭探测） */ }
}

// form=false 只刷新结果/候选/依赖，不回写表单：自动刷新不能覆盖用户正在改的配置。
async function fetchChainConfig(opts = {}) {
  const { form = true } = opts
  try {
    const d = await client.get('/cfip/chain-probe-config') || {}
    if (form && d.config) {
      const fallbackMax = d.config.max_targets ?? 20
      chainCfg.value = {
        enabled: !!d.config.enabled,
        inherit_probe: d.config.inherit_probe !== false,
        interval_minutes: d.config.interval_minutes || 5,
        dialer: d.config.dialer || '',
        agent_id: Number(d.config.agent_id || 0),
        exit: d.config.exit || '',
        interface: d.config.interface || '',
        inbound: d.config.inbound || '',
        concurrency: d.config.concurrency ?? 2,
        domain_enabled: d.config.domain_enabled !== false,
        ip_enabled: d.config.ip_enabled !== false,
        domain_max_targets: d.config.domain_max_targets ?? fallbackMax,
        ip_max_targets: d.config.ip_max_targets ?? fallbackMax,
        probe_url: d.config.probe_url || 'https://www.cloudflare.com/cdn-cgi/trace',
        kinds_configured: true,
      }
    }
    chainLocalInterfaces = (d.interfaces || []).map((i) => ({
      name: i.name,
      label: i.up ? i.name : `${i.name} (down)`,
    }))
    chainInterfaces.value = chainLocalInterfaces
    chainAgents.value = d.agents || []
    chainDialerAvailable.value = d.chain_dialers || []
    chainDialerVersions.value = d.chain_dialer_versions || {}
    chainDialerError.value = d.chain_dialer_error || ''
    chainPreviousDialer = chainDialerAvailable.value.includes(chainCfg.value.dialer) ? chainCfg.value.dialer : ''
    chainExits.value = d.exits || []
    chainInbounds.value = d.inbounds || []
    chainTargets.value = d.targets || []
    // 旧后端只有一份合并结果：按 kind 拆开，升级瞬间也不会把两类混在一起。
    const legacy = Array.isArray(d.results) ? d.results : []
    const pickKind = (list, kind) => list.filter((r) => r.kind === kind)
    chainResults.value = {
      domain: sortResults(d.domain_results || pickKind(legacy, 'cf_domain')),
      ip: sortResults(d.ip_results || pickKind(legacy, 'cf_ip')),
    }
    chainUpdatedAt.value = {
      domain: d.domain_results_updated_at || d.results_updated_at || 0,
      ip: d.ip_results_updated_at || d.results_updated_at || 0,
    }
    chainProbe.value = {
      domain: d.domain_probe || d.probe || '',
      ip: d.ip_probe || d.probe || '',
    }
    chainDeps.value = {
      interval_minutes: d.interval_minutes ?? 0,
      dialer: d.dialer || '',
      effective_dialer: d.effective_dialer || '',
    }
  } catch { /* 未配置时保留默认值 */ }
}

function sortResults(list) {
  return [...list].sort((a, b) => {
    if (a.healthy !== b.healthy) return a.healthy ? 1 : -1
    return (b.checked_at || 0) - (a.checked_at || 0)
  })
}

function isDialerInstalled(name) {
  return (probeCfg.value.available_dialers || []).includes(name)
}

// 版本探测返回的是整行输出（如 "Mihomo Meta v1.19.34 linux amd64 ..."），
// 下拉和弹窗里只显示版本号本身，避免撑爆宽度。
function dialerVersionText(name, versions = probeCfg.value.dialer_versions) {
  const raw = ((versions || {})[name] || '').trim()
  if (!raw) return ''
  const m = raw.match(/(?:v\d+\.\d+\.\d+[\w.-]*|alpha-[0-9a-f]+)/i)
  return m ? m[0] : raw
}

// 已安装显示版本，未安装标注；选中未安装核心会自动弹安装框。
function dialerLabel(name, list = probeCfg.value.available_dialers, versions = probeCfg.value.dialer_versions) {
  if (!(list || []).includes(name)) return `${name} ${t('service.cfip.probeMissing')}`
  const v = dialerVersionText(name, versions)
  return v ? `${name} (${v})` : name
}

function isOddVersion(v) {
  const parts = (v || '').replace(/^v/, '').split('.')
  const last = parseInt(parts[parts.length - 1])
  return !isNaN(last) && last % 2 !== 0
}

async function onDialerChange(name) {
  if (!name) {
    previousDialer = ''
    return
  }
  const installed = probeCfg.value.available_dialers || []
  if (installed.includes(name)) {
    previousDialer = name
    return
  }
  // 先回退选择：只有安装成功后才真正切过去，避免把不可用核心保存进探测配置。
  probeCfg.value.dialer = previousDialer
  try {
    await ElMessageBox.confirm(
      t('service.cfip.installCorePrompt', { core: name }),
      t('common.confirm'),
      { type: 'warning' },
    )
  } catch {
    return
  }
  await openDialerInstall(name, 'probe')
}

async function openDialerInstall(core, scope = 'probe') {
  dialerInstallScope.value = scope
  dialerInstallAgentId.value = scope === 'chain' ? (chainCfg.value.agent_id || 0) : (probeCfg.value.agent_id || 0)
  dialerInstallCore.value = core
  dialerInstallVisible.value = true
  dialerReleasesLoading.value = true
  dialerReleases.value = []
  try {
    // 远程节点：走节点自己的 release 列表（按该节点的平台筛选资产）；
    // 面板本机：走面板的拨号核心列表。
    const url = dialerInstallAgentId.value
      ? `/node-servers/${dialerInstallAgentId.value}/binaries/releases`
      : '/cfip/dialers/releases'
    const data = await client.get(url, { params: { core_type: core } })
    const items = Array.isArray(data) ? data : (data?.items || [])
    dialerReleases.value = items.map((r) => ({
      ...r,
      prerelease: r.prerelease !== undefined ? r.prerelease : isOddVersion(r.version),
    }))
  } catch (e) {
    ElMessage.error(e?.response?.data?.message || t('service.cfip.installCoreFailed'))
  } finally {
    dialerReleasesLoading.value = false
  }
}

// 远程安装是异步任务：轮询该节点的二进制列表，直到目标版本出现（最长约 120s）。
function versionNumber(v) {
  const m = String(v || '').match(/\d+\.\d+\.\d+/)
  return m ? m[0] : ''
}

async function waitRemoteBinary(agentId, core, version) {
  const want = versionNumber(version)
  for (let i = 0; i < 60; i++) {
    await new Promise((resolve) => setTimeout(resolve, 2000))
    try {
      const d = await client.get(`/node-servers/${agentId}/binaries`)
      const bins = (d && d.binaries) || []
      const hit = bins.find((b) => b.core_type === core && b.installed)
      if (hit && (!want || !versionNumber(hit.version) || versionNumber(hit.version) === want)) {
        return true
      }
    } catch {
      // 节点短暂不可用时继续等
    }
  }
  return false
}

async function installDialerVersion(release) {
  if (!release?.version || dialerInstalling.value) return
  try {
    await ElMessageBox.confirm(
      t('service.cfip.installCoreConfirm', { core: dialerInstallCore.value, version: release.version }),
      t('common.confirm'),
      { type: 'warning' },
    )
  } catch {
    return
  }
  dialerInstalling.value = true
  const core = dialerInstallCore.value
  const scope = dialerInstallScope.value
  const agentId = dialerInstallAgentId.value
  try {
    if (agentId) {
      // 远程：下发给该节点 Agent 异步安装，再轮询节点状态确认装好。
      await client.post(
        `/node-servers/${agentId}/binaries/install`,
        { core_type: core, version: release.version },
        { timeout: 60000 },
      )
      const ok = await waitRemoteBinary(agentId, core, release.version)
      if (!ok) {
        ElMessage.warning(t('service.cfip.installCorePending', { core }))
        await refreshDialersByScope(scope, agentId, core)
        return
      }
    } else {
      await client.post(
        '/cfip/dialers/install',
        { core_type: core, version: release.version },
        { timeout: 600000 },
      )
    }
    ElMessage.success(t('service.cfip.installCoreSuccess', { core }))
    dialerInstallVisible.value = false
    const installed = await refreshDialersByScope(scope, agentId, core)
    if (!installed) {
      ElMessage.warning(t('service.cfip.probeAgentMissing', { core }))
      return
    }
    // 安装完成后自动选中并保存，用户不用再点一次。
    if (scope === 'chain') {
      chainCfg.value.dialer = core
      await saveChainConfig()
    } else {
      probeCfg.value.dialer = core
      await saveProbeConfig()
    }
  } catch (e) {
    ElMessage.error(e?.response?.data?.message || t('service.cfip.installCoreFailed'))
  } finally {
    dialerInstalling.value = false
  }
}

// 安装后刷新对应卡片的节点核心清单；返回该核心现在是否已可用。
async function refreshDialersByScope(scope, agentId, core) {
  if (scope === 'chain') {
    await refreshChainDialers(agentId)
    return chainDialerAvailable.value.includes(core)
  }
  await refreshProbeDialers(agentId)
  return (probeCfg.value.available_dialers || []).includes(core)
}

// 当前选择的拨号核心没装 → 先弹安装，不保存/不检测。
function dialerNeedsInstall() {
  const name = probeCfg.value.dialer
  if (!name) return false
  return !(probeCfg.value.available_dialers || []).includes(name)
}

async function saveProbeConfig() {
  if (probeCfg.value.enabled && !probeCfg.value.entry) {
    ElMessage.warning(t('service.cfip.probeEntryRequired'))
    return
  }
  if (dialerNeedsInstall()) {
    await onDialerChange(probeCfg.value.dialer)
    return
  }
  try {
    await client.put('/cfip/probe-config', probeCfg.value)
    ElMessage.success(t('common.success'))
    await fetchProbeConfig()
    notifyCfipRefresh()
  } catch (e) {
    ElMessage.error(e?.response?.data?.message || t('common.failed'))
  }
}

// 切换执行节点时重新取该节点的网卡（面板本机用接口返回的列表；
// 远程节点复用现有的 /node-servers/:id/binaries/interfaces）。
async function onAgentChange(agentId) {
  chainCfg.value.interface = ''
  await Promise.all([refreshChainInterfaces(agentId), refreshChainDialers(agentId)])
}

async function refreshChainInterfaces(agentId) {
  if (!agentId) {
    chainInterfaces.value = chainLocalInterfaces
    return
  }
  try {
    const names = await client.get(`/node-servers/${agentId}/binaries/interfaces`)
    chainInterfaces.value = (names || []).map((n) => ({ name: n, label: n }))
  } catch {
    chainInterfaces.value = []
  }
}

// 链路探测的拨号核心同样装在自己的执行节点上。
async function refreshChainDialers(agentId) {
  chainDialerError.value = ''
  try {
    const d = await client.get('/cfip/probe-capabilities', { params: { agent_id: agentId || 0 } })
    chainDialerAvailable.value = (d && d.dialers) || []
    chainDialerVersions.value = (d && d.dialer_versions) || {}
    chainDialerError.value = (d && d.error) || ''
    const list = chainDialerAvailable.value
    chainPreviousDialer = list.includes(chainCfg.value.dialer) ? chainCfg.value.dialer : ''
  } catch (e) {
    chainDialerAvailable.value = []
    chainDialerVersions.value = {}
    chainDialerError.value = e?.response?.data?.message || t('common.failed')
  }
}

// 链路拨号核心切换：本机可直接装，远程节点只能提示缺核心。
async function onChainDialerChange(name) {
  if (!name) {
    chainPreviousDialer = ''
    return
  }
  if (chainDialerAvailable.value.includes(name)) {
    chainPreviousDialer = name
    return
  }
  chainCfg.value.dialer = chainPreviousDialer
  try {
    await ElMessageBox.confirm(
      t('service.cfip.installCorePrompt', { core: name }),
      t('common.confirm'),
      { type: 'warning' },
    )
  } catch {
    return
  }
  await openDialerInstall(name, 'chain')
}

// 探测设置的执行节点切换：网卡列表跟着换（远程复用节点网卡接口）。
async function onProbeAgentChange(agentId) {
  probeCfg.value.interface = ''
  await Promise.all([refreshProbeInterfaces(agentId), refreshProbeDialers(agentId)])
}

async function refreshProbeInterfaces(agentId) {
  if (!agentId) {
    probeInterfaces.value = probeLocalInterfaces
    return
  }
  try {
    const names = await client.get(`/node-servers/${agentId}/binaries/interfaces`)
    probeInterfaces.value = (names || []).map((n) => ({ name: n, label: n }))
  } catch {
    probeInterfaces.value = []
  }
}

// 探测核心装在执行节点上：切节点必须重新问该节点，不能沿用面板的清单。
async function refreshProbeDialers(agentId) {
  probeAgentDialerError.value = ''
  try {
    const d = await client.get('/cfip/probe-capabilities', { params: { agent_id: agentId || 0 } })
    probeCfg.value.available_dialers = (d && d.dialers) || []
    probeCfg.value.dialer_versions = (d && d.dialer_versions) || {}
    probeAgentDialerError.value = (d && d.error) || ''
    const list = probeCfg.value.available_dialers
    previousDialer = list.includes(probeCfg.value.dialer) ? probeCfg.value.dialer : ''
    if (probeCfg.value.dialer && !list.includes(probeCfg.value.dialer)) {
      probeAgentDialerError.value = t('service.cfip.probeAgentMissing', { core: probeCfg.value.dialer })
    }
  } catch (e) {
    probeCfg.value.available_dialers = []
    probeCfg.value.dialer_versions = {}
    probeAgentDialerError.value = e?.response?.data?.message || t('common.failed')
  }
}

async function saveChainConfig() {
  try {
    await client.put('/cfip/chain-probe-config', chainCfg.value)
    ElMessage.success(t('common.success'))
    await fetchChainConfig()
    notifyCfipRefresh()
  } catch (e) {
    ElMessage.error(e?.response?.data?.message || t('common.failed'))
  }
}

async function checkChain(kind) {
  // 继承模式用的是探测设置的核心，但实际跑在链路自己的执行节点上：
  // 必须按链路的节点清单校验，不能拿面板/探测节点的清单判断。
  const effectiveDialer = chainCfg.value.inherit_probe ? probeCfg.value.dialer : chainCfg.value.dialer
  if (effectiveDialer && !chainDialerAvailable.value.includes(effectiveDialer)) {
    await openDialerInstall(effectiveDialer, 'chain')
    return
  }
  if (!chainCfg.value.exit) {
    ElMessage.warning(t('service.cfip.chainNeedExit'))
    return
  }
  if (!chainCfg.value.inbound) {
    ElMessage.warning(t('service.cfip.chainNeedInbound'))
    return
  }
  // 手动检测走保存后的配置：先落库再跑，避免"页面选了但后端还是旧值"。
  try {
    await client.put('/cfip/chain-probe-config', chainCfg.value)
  } catch (e) {
    ElMessage.error(e?.response?.data?.message || t('common.failed'))
    return
  }
  chainChecking.value[kind] = true
  try {
    // kind 明确指定：只测域名或只测 IP，结果分别保存，互不覆盖。
    const res = await client.post('/cfip/chain-probe/check', { kind }, { timeout: 180000 })
    const summary = res || {}
    if (summary.unhealthy) {
      ElMessage.warning(`${t('service.cfip.chainHealthy')} ${summary.healthy || 0} / ${t('service.cfip.chainUnhealthy')} ${summary.unhealthy}`)
    } else {
      ElMessage.success(`${t('service.cfip.chainHealthy')} ${summary.healthy || 0}`)
    }
    await fetchChainConfig()
    // 链路探测会回写池条目健康度：通知同页的其它探测实例与池列表一起刷新。
    notifyCfipRefresh()
  } catch (e) {
    ElMessage.error(e?.response?.data?.message || t('common.failed'))
  } finally {
    chainChecking.value[kind] = false
  }
}

function formatTime(ts) {
  if (!ts) return '-'
  return new Date(ts * 1000).toLocaleString()
}

function formatBytes(n) {
  const v = Number(n) || 0
  if (v <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  let i = 0
  let val = v
  while (val >= 1024 && i < units.length - 1) {
    val /= 1024
    i++
  }
  return `${val.toFixed(i ? 1 : 0)} ${units[i]}`
}

// 自动刷新：定时探测/其它实例跑完检测后，本页结果无需 F5 即可更新。
// form=false 保证自动刷新只更新结果与候选，不覆盖正在编辑的表单。
useAutoRefresh(
  async () => {
    await Promise.all([
      fetchProbeConfig({ form: false }),
      fetchChainConfig({ form: false }),
    ])
  },
  {
    interval: 30000,
    events: [CFIP_REFRESH_EVENT],
    isBusy: () => dialerInstallVisible.value || chainChecking.value.domain || chainChecking.value.ip,
  },
)

onMounted(() => {
  fetchProbeConfig()
  fetchChainConfig()
})
</script>

<style scoped>
/* 域名 / IP 两个独立区块：视觉上分开，避免误以为是一份共用配置 */
.chain-kind-block {
  margin-top: 16px;
  padding-top: 12px;
  border-top: 1px dashed #e4e7ed;
}
.chain-kind-head {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}
.chain-kind-title {
  font-weight: 600;
  font-size: 14px;
  color: #303133;
}
.chain-kind-label {
  font-size: 12px;
  color: #909399;
}
.chain-kind-meta {
  font-size: 11px;
  color: #909399;
}
</style>
