<template>
  <div>
    <!-- Core Detail View -->
    <div v-if="selectedCore">
      <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:16px">
        <div style="display:flex;align-items:center;gap:12px">
          <span style="font-size:20px">{{ getCoreIcon(selectedCore) }}</span>
          <span style="font-size:18px;font-weight:600">{{ getCoreLabel(selectedCore) }}</span>
          <el-tag v-if="svc.binary_installed" :type="svc.state==='running'?'success':(svc.state==='crashed'?'danger':'info')" size="small">
            {{ svc.state==='running'?t('protoCenter.status.running'):svc.state==='crashed'?t('protoCenter.status.crashed'):svc.state==='stopped'?t('protoCenter.status.stopped'):t('protoCenter.status.unknown') }}
          </el-tag>
          <el-tag v-else type="danger" size="small">{{ t('protoCenter.status.notInstalled') }}</el-tag>
        </div>
        <el-button plain @click="goBack">{{ t('protoCenter.status.backToCenter') }}</el-button>
      </div>

      <!-- Status Bar -->
      <el-card shadow="hover" style="margin-bottom:16px" body-style="padding:12px 20px">
        <div style="display:flex;align-items:center;gap:24px;font-size:13px;flex-wrap:wrap">
          <div style="display:flex;gap:6px;align-items:center">
            <span style="color:#909399">{{ t('protoCenter.status.version') }}</span>
            <span v-if="!svc.binary_installed" style="color:#f56c6c;cursor:pointer" @click="showVersions">{{ t('protoCenter.status.notInstalled') }}</span>
            <el-tooltip v-else :content="svc.version" placement="bottom">
              <el-link type="primary" @click="showVersions" style="max-width:none;white-space:normal">{{ svc.version || '...' }}</el-link>
            </el-tooltip>
          </div>
          <div style="display:flex;gap:6px;align-items:center" v-if="svc.state==='running'">
            <span style="color:#909399">PID</span><span>{{ svc.pid }}</span>
            <span style="color:#909399;margin-left:8px">{{ t('protoCenter.status.cpu') }}</span><span>{{ svc.cpu?.toFixed(1) }}%</span>
            <span style="color:#909399;margin-left:8px">{{ t('protoCenter.status.memory') }}</span><span>{{ fmtSize(svc.mem) }}</span>
            <span style="color:#909399;margin-left:8px">{{ t('protoCenter.status.uptime') }}</span><span>{{ fmtUptime(svc.uptime) }}</span>
            <span style="color:#909399;margin-left:8px">{{ t('protoCenter.status.startTime') }}</span><span>{{ fmtTime(svc.started_at) }}</span>
          </div>
          <div style="margin-left:auto;display:flex;gap:8px">
            <template v-if="!svc.binary_installed">
              <el-button type="primary" size="small" :loading="acting" @click="showVersions">{{ t('protoCenter.status.install') }}</el-button>
            </template>
            <template v-else>
              <el-button v-if="svc.state!=='running'" type="success" size="small" :loading="acting" @click="doStart">{{ t('protoCenter.status.start') }}</el-button>
              <el-button v-else type="danger" size="small" :loading="acting" @click="doStop">{{ t('protoCenter.status.stop') }}</el-button>
              <el-button size="small" :loading="acting" @click="doRestart">{{ t('protoCenter.status.restart') }}</el-button>
              <el-button type="warning" size="small" plain @click="showVersions">{{ t('protoCenter.status.switchVersion') }}</el-button>
            </template>
          </div>
        </div>
      </el-card>

      <!-- Section Nav + Content -->
      <el-card shadow="never">
        <!-- Section Tabs -->
        <div style="display:flex;gap:4px;margin-bottom:16px;flex-wrap:wrap;border-bottom:1px solid var(--el-border-color-lighter);padding-bottom:12px">
          <div
            v-for="s in sections"
            :key="s.key"
            class="sec-tab"
            :class="{ active: activeSection === s.key }"
            @click="switchSection(s.key)"
          >{{ s.label }}</div>
        </div>

        <!-- Section Content -->
        <div v-if="activeSection">
          <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:16px">
            <span style="font-size:15px;font-weight:600">{{ getLabel(activeSection) }}</span>
            <div v-if="activeSection !== '_preview' && activeSection !== 'inbounds' && activeSection !== 'outbounds' && activeSection !== 'routing'">
              <el-button size="small" @click="resetSection">{{ t('common.reset') }}</el-button>
              <el-button size="small" type="primary" :loading="saving" @click="saveSection">{{ t('common.save') }}</el-button>
            </div>
          </div>
          <!-- Component -->
          <component
            v-if="getComp(activeSection)"
            :is="getComp(activeSection)"
            :key="selectedCore + '/' + activeSection"
            :data="secData"
            :core-type="selectedCore"
            :server-id="serverId"
            @update="onUpdate"
          />
          <!-- Fallback JSON -->
          <el-input v-else v-model="secJson" type="textarea" :rows="14" @change="onJsonChange" />
        </div>
      </el-card>

      <!-- Version Dialog -->
      <el-dialog v-model="verVisible" :title="t('protoCenter.version.dialogTitle')" width="600" :close-on-click-modal="!downloading">
        <div v-if="downloading" style="text-align:center;padding:40px 20px">
          <el-icon class="is-loading" :size="32" color="#409eff"><Loading /></el-icon>
          <div style="margin-top:16px;font-size:14px;color:#606266">{{ downloadProgress }}</div>
        </div>
        <div v-else v-loading="verLoading" style="min-height:80px">
          <div v-if="!releases.length && !verLoading" style="text-align:center;color:#999;padding:16px">{{ t('protoCenter.version.noVersions') }}</div>
          <div style="display:flex;flex-wrap:wrap;gap:8px">
            <div v-for="r in releases" :key="r.version" class="ver-card" :class="{ 'ver-card-current': r.version === svc.version }" @click="doDownload(r)">
              <div style="display:flex;justify-content:space-between;width:100%">
                <span style="font-size:11px" :style="{color:r.prerelease?'#67c23a':'#409eff'}">{{ r.prerelease?'Pre-Release':'Stable' }}</span>
                <el-tag v-if="r.version === svc.version && svc.binary_installed" size="small" type="success">{{ t('protoCenter.version.current') }}</el-tag>
              </div>
              <div style="font-size:13px;font-weight:600;margin-top:2px">{{ r.version }}</div>
            </div>
          </div>
        </div>
      </el-dialog>
    </div>

    <!-- Core Selection -->
    <div v-else>
      <h2 style="margin-bottom:20px">{{ t('protoCenter.title') }}</h2>
      <div style="display:flex;gap:24px;justify-content:center;margin-top:60px;flex-wrap:wrap">
        <div v-for="c in cores" :key="c.type" class="core-card" @click="selectCore(c.type)">
          <div style="text-align:center;padding:20px 40px">
            <div style="font-size:36px;margin-bottom:12px">{{ c.icon }}</div>
            <div style="font-size:18px;font-weight:600;margin-bottom:8px">{{ c.label }}</div>
            <div style="font-size:13px;color:#909399">{{ c.desc }}</div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted, onUnmounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Loading } from '@element-plus/icons-vue'
import client from '../../api/client'

import XrayLog from './sections/XrayLog.vue'
import XrayDns from './sections/XrayDns.vue'
import XrayRouting from './sections/XrayRouting.vue'
import XrayInbounds from './sections/XrayInbounds.vue'
import XrayOutbounds from './sections/XrayOutbounds.vue'
import XrayPolicy from './sections/XrayPolicy.vue'
import XrayStats from './sections/XrayStats.vue'
import XrayMetrics from './sections/XrayMetrics.vue'
import XrayFakeDns from './sections/XrayFakeDns.vue'
import XrayObservatory from './sections/XrayObservatory.vue'
import XrayApi from './sections/XrayApi.vue'
import XrayBurstObs from './sections/XrayBurstObs.vue'
import XrayVersion from './sections/XrayVersion.vue'
import XrayGeodata from './sections/XrayGeodata.vue'
import SingboxLog from './sections/SingboxLog.vue'
import SingboxDns from './sections/SingboxDns.vue'
import SingboxRoute from './sections/SingboxRoute.vue'
import SingboxInbounds from './sections/SingboxInbounds.vue'
import SingboxOutbounds from './sections/SingboxOutbounds.vue'
import SingboxExperimental from './sections/SingboxExperimental.vue'
import Hy2Listen from './sections/Hy2Listen.vue'
import Hy2Tls from './sections/Hy2Tls.vue'
import Hy2Auth from './sections/Hy2Auth.vue'
import Hy2Bandwidth from './sections/Hy2Bandwidth.vue'
import Hy2Quic from './sections/Hy2Quic.vue'
import Hy2TrafficStats from './sections/Hy2TrafficStats.vue'
import TuicServer from './sections/TuicServer.vue'
import TuicUsers from './sections/TuicUsers.vue'
import TuicTls from './sections/TuicTls.vue'
import TuicCongestion from './sections/TuicCongestion.vue'
import TuicAdvanced from './sections/TuicAdvanced.vue'
import JsonPreview from './sections/JsonPreview.vue'

const { t } = useI18n()

const props = defineProps({
  serverId: { type: Number, required: true }
})

// API base path: always node-server-scoped
const apiBase = computed(() => `/node-servers/${props.serverId}/protocols`)

const cores = [
  { type:'xray', label:'Xray', icon:'⚡', desc:t('protoCenter.cores.xrayDesc') },
  { type:'singbox', label:'sing-box', icon:'📦', desc:t('protoCenter.cores.singboxDesc') },
  { type:'hysteria2', label:'Hysteria 2', icon:'🚀', desc:t('protoCenter.cores.hysteria2Desc') },
  { type:'tuic', label:'TUIC', icon:'🌊', desc:t('protoCenter.cores.tuicDesc') },
]

function getSections() {
  return {
    xray: [
      {key:'log',label:t('protoCenter.sections.log')},{key:'api',label:t('protoCenter.sections.api')},{key:'dns',label:t('protoCenter.sections.dns')},{key:'routing',label:t('protoCenter.sections.routing')},{key:'policy',label:t('protoCenter.sections.policy')},
      {key:'inbounds',label:t('protoCenter.sections.inbounds')},{key:'outbounds',label:t('protoCenter.sections.outbounds')},
      {key:'stats',label:t('protoCenter.sections.stats')},{key:'metrics',label:t('protoCenter.sections.metrics')},
      {key:'fakedns',label:t('protoCenter.sections.fakedns')},{key:'observatory',label:t('protoCenter.sections.observatory')},{key:'burstObservatory',label:t('protoCenter.sections.burstObservatory')},
      {key:'version',label:t('protoCenter.sections.version')},{key:'geodata',label:t('protoCenter.sections.geodata')},{key:'_preview',label:t('protoCenter.sections.preview')},
    ],
    singbox: [
      {key:'log',label:t('protoCenter.sections.log')},{key:'dns',label:t('protoCenter.sections.dns')},{key:'route',label:t('protoCenter.sections.route')},
      {key:'inbounds',label:t('protoCenter.sections.inbounds')},{key:'outbounds',label:t('protoCenter.sections.outbounds')},{key:'experimental',label:t('protoCenter.sections.experimental')},
      {key:'_preview',label:t('protoCenter.sections.preview')},
    ],
    hysteria2: [
      {key:'listen',label:t('protoCenter.sections.listen')},{key:'tls',label:t('protoCenter.sections.tls')},{key:'auth',label:t('protoCenter.sections.auth')},
      {key:'bandwidth',label:t('protoCenter.sections.bandwidth')},{key:'quic',label:t('protoCenter.sections.quic')},{key:'trafficStats',label:t('protoCenter.sections.trafficStats')},
      {key:'_preview',label:t('protoCenter.sections.preview')},
    ],
    tuic: [
      {key:'server',label:t('protoCenter.sections.server')},{key:'users',label:t('protoCenter.sections.users')},{key:'certificate',label:t('protoCenter.sections.tls')},
      {key:'congestion_control',label:t('protoCenter.sections.congestion')},{key:'_preview',label:t('protoCenter.sections.preview')},
    ],
  }
}

const COMPS = {
  xray: {log:XrayLog,api:XrayApi,dns:XrayDns,routing:XrayRouting,policy:XrayPolicy,inbounds:XrayInbounds,outbounds:XrayOutbounds,stats:XrayStats,metrics:XrayMetrics,fakedns:XrayFakeDns,observatory:XrayObservatory,burstObservatory:XrayBurstObs,version:XrayVersion,geodata:XrayGeodata,_preview:JsonPreview},
  singbox: {log:SingboxLog,dns:SingboxDns,route:SingboxRoute,inbounds:SingboxInbounds,outbounds:SingboxOutbounds,experimental:SingboxExperimental,_preview:JsonPreview},
  hysteria2: {listen:Hy2Listen,tls:Hy2Tls,auth:Hy2Auth,bandwidth:Hy2Bandwidth,quic:Hy2Quic,trafficStats:Hy2TrafficStats,_preview:JsonPreview},
  tuic: {server:TuicServer,users:TuicUsers,certificate:TuicTls,congestion_control:TuicCongestion,_preview:JsonPreview},
}

const selectedCore = ref('')
const activeSection = ref('')
const fullConfig = ref({})
const secData = ref(null)
const secJson = ref('')
const saving = ref(false)

const svc = reactive({service_id:0,state:'unknown',binary_installed:false,version:'',pid:0,cpu:0,mem:0})
const acting = ref(false)
const verVisible = ref(false)
const verLoading = ref(false)
const releases = ref([])
const downloading = ref(false)
const downloadProgress = ref('')
let timer = null

const sections = ref([])

function getCoreIcon(t){return cores.find(c=>c.type===t)?.icon||''}
function getCoreLabel(t){return cores.find(c=>c.type===t)?.label||t}
function getLabel(k){return sections.value.find(s=>s.key===k)?.label||k}
function getComp(k){return COMPS[selectedCore.value]?.[k]||null}
function fmtSize(b){if(!b)return'-';return b<1024?b+'B':b<1048576?(b/1024).toFixed(0)+'KB':(b/1048576).toFixed(1)+'MB'}
function fmtTime(ts){if(!ts||ts==='0001-01-01T00:00:00Z')return'-';return new Date(ts).toLocaleString()}
function fmtUptime(s){if(!s)return'-';const d=Math.floor(s/86400),h=Math.floor(s%86400/3600),m=Math.floor(s%3600/60);let r='';if(d>0)r+=d+t('protoCenter.status.days')+' ';if(h>0)r+=h+t('protoCenter.status.hours')+' ';r+=m+t('protoCenter.status.minutes');return r.trim()}

async function selectCore(t){
  selectedCore.value=t
  activeSection.value=''
  sections.value=getSections()[t]||[]
  localStorage.setItem('active_core',t)
  await loadConfig(t)
  await fetchStatus()
}
function goBack(){selectedCore.value='';activeSection.value='';localStorage.removeItem('active_core')}

function switchSection(k){
  activeSection.value=k
  const d=fullConfig.value[k]
  if(k==='_preview'){secData.value=fullConfig.value;secJson.value=JSON.stringify(fullConfig.value,null,2);return}
  secData.value=d!==undefined?d:(Array.isArray(d)?[]:{})
  secJson.value=JSON.stringify(secData.value,null,2)
}

function onUpdate(d){
  secData.value=d
  secJson.value=d===null?'null (disabled)':JSON.stringify(d,null,2)
}
function onJsonChange(v){try{secData.value=JSON.parse(v)}catch{}}
function resetSection(){switchSection(activeSection.value);ElMessage.success(t('common.reset'))}

async function saveSection(){
  if(activeSection.value==='_preview')return
  saving.value=true
  try{
    if(secData.value===null){
      delete fullConfig.value[activeSection.value]
      await client.put(`${apiBase.value}/${selectedCore.value}`,fullConfig.value)
    } else {
      await client.put(`${apiBase.value}/${selectedCore.value}/section/${activeSection.value}`,secData.value)
      fullConfig.value[activeSection.value]=secData.value
    }
    // Auto-restart if service is running
    if(svc.state==='running'){
      try{
        await client.post(`/node-servers/${props.serverId}/control`,{core_type:selectedCore.value,action:'restart'})
      }catch{}
      ElMessage.success(t('protoCenter.messages.savedAndRestarted'))
    } else {
      ElMessage.success(t('protoCenter.messages.saved'))
    }
  }catch{ElMessage.error(t('protoCenter.messages.saveFailed'))}finally{saving.value=false}
}

async function loadConfig(ct){try{fullConfig.value=await client.get(`${apiBase.value}/${ct}`)||{}}catch{fullConfig.value={}}}

async function fetchStatus(){
  if(!selectedCore.value)return
  try{
    // Use node-server-scoped status API
    const data=await client.get(`/node-servers/${props.serverId}/status`)
    const cores=data?.cores||[]
    const s=cores.find(x=>x.core_type===selectedCore.value)
    if(s)Object.assign(svc,{service_id:0,state:s.state||'unknown',binary_installed:s.state==='running',version:s.version||'',pid:s.pid||0,cpu:s.cpu||0,mem:s.mem||0});else Object.assign(svc,{service_id:0,state:'unknown',binary_installed:false})
  }catch{}
}

async function doStart(){
  acting.value=true
  try{
    await client.post(`/node-servers/${props.serverId}/control`,{core_type:selectedCore.value,action:'start'})
    ElMessage.success(t('protoCenter.messages.started'));fetchStatus()
  }catch{ElMessage.error(t('protoCenter.messages.startFailed'))}finally{acting.value=false}
}
async function doStop(){
  await ElMessageBox.confirm(t('protoCenter.messages.stopConfirm'),t('common.confirm'))
  acting.value=true
  try{
    await client.post(`/node-servers/${props.serverId}/control`,{core_type:selectedCore.value,action:'stop'})
    ElMessage.success(t('protoCenter.messages.stopped'));fetchStatus()
  }catch{ElMessage.error(t('protoCenter.messages.stopFailed'))}finally{acting.value=false}
}
async function doRestart(){
  acting.value=true
  try{
    await client.post(`/node-servers/${props.serverId}/control`,{core_type:selectedCore.value,action:'restart'})
    ElMessage.success(t('protoCenter.messages.restarted'));fetchStatus()
  }catch{ElMessage.error(t('protoCenter.messages.restartFailed'))}finally{acting.value=false}
}

async function showVersions(){verVisible.value=true;verLoading.value=true;try{const d=await client.get(`/node-servers/${props.serverId}/binaries/releases`,{params:{core_type:selectedCore.value,limit:20,offset:0}});releases.value=(d.items||[]).map(r=>({...r,prerelease:r.prerelease??isOdd(r.version)}))}catch{releases.value=[]}finally{verLoading.value=false}}

// Check if a version is already installed on this server
async function checkInstalledVersion(version){try{const data=await client.get(`/node-servers/${props.serverId}/binaries`);const bins=data?.binaries||[];return!!bins.find(b=>b.core_type===selectedCore.value&&b.version===version)}catch{return false}}
function isOdd(v){const p=(v||'').replace(/^v/,'').split('.');const n=parseInt(p[p.length-1]);return!isNaN(n)&&n%2!==0}
async function doDownload(r){
  await ElMessageBox.confirm(t('protoCenter.version.downloadConfirm', { version: r.version }),t('common.confirm'))
  downloading.value=true
  downloadProgress.value=t('protoCenter.version.downloading', { version: r.version })
  try{
    // Use node-server-scoped install API to download & install binary to the target server
    await client.post(`/node-servers/${props.serverId}/binaries/install`,{core_type:selectedCore.value,version:r.version})
    downloadProgress.value=t('protoCenter.version.downloadedStarting')
    // For local server, try to start the core; for remote, the Agent handles it
    try{await client.post(`/node-servers/${props.serverId}/control`,{core_type:selectedCore.value,action:'start'})}catch(e){/* remote server or already running */}
    // Poll for status update
    await new Promise(resolve=>setTimeout(resolve,1000))
    for(let i=0;i<10;i++){
      try{
        const data=await client.get(`/node-servers/${props.serverId}/status`)
        const s=(data?.cores||[]).find(x=>x.core_type===selectedCore.value)
        if(s&&s.state==='running'){
          ElMessage.success(t('protoCenter.version.updateDone'))
          verVisible.value=false
          fetchStatus()
          return
        }
      }catch{}
      await new Promise(resolve=>setTimeout(resolve,1000))
    }
    ElMessage.success(t('protoCenter.version.downloadDone'))
    verVisible.value=false
    fetchStatus()
  }catch{ElMessage.error(t('protoCenter.version.downloadFailed'))}finally{downloading.value=false}
}

onMounted(()=>{const s=localStorage.getItem('active_core');if(s&&cores.some(c=>c.type===s))selectCore(s);timer=setInterval(fetchStatus,10000)})
onUnmounted(()=>{if(timer)clearInterval(timer)})
</script>

<style scoped>
.core-card{cursor:pointer;transition:all .2s;width:260px;background:var(--el-bg-color);border:1px solid var(--el-border-color-lighter);border-radius:8px;box-shadow:0 2px 12px rgba(0,0,0,.06)}
.core-card:hover{border-color:#409eff;transform:translateY(-4px);box-shadow:0 8px 24px rgba(64,158,255,.15)}
.sec-tab{padding:8px 14px;cursor:pointer;border-radius:4px;font-size:13px;color:var(--el-text-color-regular);transition:all .15s}
.sec-tab:hover{background:var(--el-fill-color-light);color:var(--el-color-primary)}
.sec-tab.active{background:var(--el-color-primary-light-9);color:var(--el-color-primary);font-weight:600}
.ver-card{width:calc(16.66%-8px);min-width:100px;padding:10px;border:1px solid #dcdfe6;border-radius:6px;cursor:pointer;text-align:center;transition:all .2s}
.ver-card:hover{border-color:#409eff;background:#ecf5ff}
.ver-card-current{border-color:#67c23a;background:#f0f9eb}
</style>
