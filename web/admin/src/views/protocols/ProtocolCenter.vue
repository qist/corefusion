<template>
  <div>
    <!-- Core Detail View -->
    <div v-if="selectedCore">
      <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:16px">
        <div style="display:flex;align-items:center;gap:12px">
          <span style="font-size:20px">{{ getCoreIcon(selectedCore) }}</span>
          <span style="font-size:18px;font-weight:600">{{ getCoreLabel(selectedCore) }}</span>
          <el-tag v-if="svc.binary_installed" :type="svc.state==='running'?'success':(svc.state==='crashed'?'danger':'info')" size="small">
            {{ svc.state==='running'?'运行中':svc.state==='crashed'?'已崩溃':svc.state==='stopped'?'已停止':'未知' }}
          </el-tag>
          <el-tag v-else type="danger" size="small">未安装</el-tag>
        </div>
        <el-button plain @click="goBack">返回协议中心</el-button>
      </div>

      <!-- Status Bar -->
      <el-card shadow="hover" style="margin-bottom:16px" body-style="padding:12px 20px">
        <div style="display:flex;align-items:center;gap:24px;font-size:13px;flex-wrap:wrap">
          <div style="display:flex;gap:6px;align-items:center">
            <span style="color:#909399">版本</span>
            <span v-if="!svc.binary_installed" style="color:#f56c6c;cursor:pointer" @click="showVersions">未安装</span>
            <el-tooltip v-else :content="svc.version" placement="bottom">
              <el-link type="primary" @click="showVersions" style="max-width:none;white-space:normal">{{ svc.version || '...' }}</el-link>
            </el-tooltip>
          </div>
          <div style="display:flex;gap:6px;align-items:center" v-if="svc.state==='running'">
            <span style="color:#909399">PID</span><span>{{ svc.pid }}</span>
            <span style="color:#909399;margin-left:8px">CPU</span><span>{{ svc.cpu?.toFixed(1) }}%</span>
            <span style="color:#909399;margin-left:8px">内存</span><span>{{ fmtSize(svc.mem) }}</span>
            <span style="color:#909399;margin-left:8px">运行时间</span><span>{{ fmtUptime(svc.uptime) }}</span>
            <span style="color:#909399;margin-left:8px">启动时间</span><span>{{ fmtTime(svc.started_at) }}</span>
          </div>
          <div style="margin-left:auto;display:flex;gap:8px">
            <template v-if="!svc.binary_installed">
              <el-button type="primary" size="small" :loading="acting" @click="showVersions">下载安装</el-button>
            </template>
            <template v-else>
              <el-button v-if="svc.state!=='running'" type="success" size="small" :loading="acting" @click="doStart">启动</el-button>
              <el-button v-else type="danger" size="small" :loading="acting" @click="doStop">停止</el-button>
              <el-button size="small" :loading="acting" @click="doRestart">重启</el-button>
              <el-button type="warning" size="small" plain @click="showVersions">切换版本</el-button>
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
            <div v-if="activeSection !== '_preview'">
              <el-button size="small" @click="resetSection">重置</el-button>
              <el-button size="small" type="primary" :loading="saving" @click="saveSection">保存</el-button>
            </div>
          </div>
          <!-- Component -->
          <component
            v-if="getComp(activeSection)"
            :is="getComp(activeSection)"
            :key="selectedCore + '/' + activeSection"
            :data="secData"
            :core-type="selectedCore"
            @update="onUpdate"
          />
          <!-- Fallback JSON -->
          <el-input v-else v-model="secJson" type="textarea" :rows="14" @change="onJsonChange" />
        </div>
      </el-card>

      <!-- Version Dialog -->
      <el-dialog v-model="verVisible" title="选择版本" width="600" :close-on-click-modal="!downloading">
        <div v-if="downloading" style="text-align:center;padding:40px 20px">
          <el-icon class="is-loading" :size="32" color="#409eff"><Loading /></el-icon>
          <div style="margin-top:16px;font-size:14px;color:#606266">{{ downloadProgress }}</div>
        </div>
        <div v-else v-loading="verLoading" style="min-height:80px">
          <div v-if="!releases.length && !verLoading" style="text-align:center;color:#999;padding:16px">暂无版本</div>
          <div style="display:flex;flex-wrap:wrap;gap:8px">
            <div v-for="r in releases" :key="r.version" class="ver-card" @click="doDownload(r)">
              <div style="font-size:11px;margin-bottom:2px" :style="{color:r.prerelease?'#67c23a':'#409eff'}">{{ r.prerelease?'Pre':'Rel' }}</div>
              <div style="font-size:12px;font-weight:500">{{ r.version }}</div>
            </div>
          </div>
        </div>
      </el-dialog>
    </div>

    <!-- Core Selection -->
    <div v-else>
      <h2 style="margin-bottom:20px">协议中心</h2>
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
import { ref, reactive, onMounted, onUnmounted } from 'vue'
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

const cores = [
  { type:'xray', label:'Xray', icon:'⚡', desc:'VLESS/VMess/Trojan/Shadowsocks 等多协议' },
  { type:'singbox', label:'sing-box', icon:'📦', desc:'模块化通用代理平台，支持 TUN' },
  { type:'hysteria2', label:'Hysteria 2', icon:'🚀', desc:'基于 QUIC 的高速代理，抗丢包' },
  { type:'tuic', label:'TUIC', icon:'🌊', desc:'基于 QUIC 的轻量低延迟代理' },
]

const SECTIONS = {
  xray: [
    {key:'log',label:'日志'},{key:'api',label:'API'},{key:'dns',label:'DNS'},{key:'routing',label:'路由'},{key:'policy',label:'策略'},
    {key:'inbounds',label:'入站'},{key:'outbounds',label:'出站'},
    {key:'stats',label:'统计'},{key:'metrics',label:'指标'},
    {key:'fakedns',label:'FakeDNS'},{key:'observatory',label:'健康检测'},{key:'burstObservatory',label:'突发检测'},
    {key:'version',label:'版本限制'},{key:'geodata',label:'Geo数据'},{key:'_preview',label:'配置预览'},
  ],
  singbox: [
    {key:'log',label:'日志'},{key:'dns',label:'DNS'},{key:'route',label:'路由'},
    {key:'inbounds',label:'入站'},{key:'outbounds',label:'出站'},{key:'experimental',label:'实验功能'},
    {key:'_preview',label:'配置预览'},
  ],
  hysteria2: [
    {key:'listen',label:'监听'},{key:'tls',label:'TLS'},{key:'auth',label:'认证'},
    {key:'bandwidth',label:'带宽'},{key:'quic',label:'QUIC'},{key:'trafficStats',label:'流量统计'},
    {key:'_preview',label:'配置预览'},
  ],
  tuic: [
    {key:'server',label:'监听'},{key:'users',label:'用户'},{key:'certificate',label:'TLS'},
    {key:'congestion_control',label:'拥塞控制'},{key:'_preview',label:'配置预览'},
  ],
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
function fmtUptime(s){if(!s)return'-';const d=Math.floor(s/86400),h=Math.floor(s%86400/3600),m=Math.floor(s%3600/60);let r='';if(d>0)r+=d+'天 ';if(h>0)r+=h+'小时 ';r+=m+'分钟';return r.trim()}

async function selectCore(t){
  selectedCore.value=t
  activeSection.value=''
  sections.value=SECTIONS[t]||[]
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
function resetSection(){switchSection(activeSection.value);ElMessage.success('已重置')}

async function saveSection(){
  if(activeSection.value==='_preview')return
  saving.value=true
  try{
    if(secData.value===null){
      delete fullConfig.value[activeSection.value]
      await client.put(`/protocols/${selectedCore.value}`,fullConfig.value)
    } else {
      await client.put(`/protocols/${selectedCore.value}/section/${activeSection.value}`,secData.value)
      fullConfig.value[activeSection.value]=secData.value
    }
    // Auto-restart if service is running
    if(svc.state==='running' && svc.service_id){
      try{await client.post(`/services/${svc.service_id}/restart`)}catch{}
      ElMessage.success('保存成功，服务已重启')
    } else {
      ElMessage.success('保存成功')
    }
  }catch{ElMessage.error('保存失败')}finally{saving.value=false}
}

async function loadConfig(ct){try{fullConfig.value=await client.get(`/protocols/${ct}`)||{}}catch{fullConfig.value={}}}

async function fetchStatus(){
  if(!selectedCore.value)return
  try{
    const data=await client.get('/services/process-status')
    const s=(data||[]).find(x=>x.core_type===selectedCore.value)
    if(s)Object.assign(svc,s);else Object.assign(svc,{service_id:0,state:'unknown',binary_installed:false})
  }catch{}
}

async function doStart(){if(!svc.service_id)return;acting.value=true;try{await client.post(`/services/${svc.service_id}/start`);ElMessage.success('已启动');fetchStatus()}catch{ElMessage.error('启动失败')}finally{acting.value=false}}
async function doStop(){if(!svc.service_id)return;await ElMessageBox.confirm('确定停止？','确认');acting.value=true;try{await client.post(`/services/${svc.service_id}/stop`);ElMessage.success('已停止');fetchStatus()}catch{ElMessage.error('停止失败')}finally{acting.value=false}}
async function doRestart(){if(!svc.service_id)return;acting.value=true;try{await client.post(`/services/${svc.service_id}/restart`);ElMessage.success('已重启');fetchStatus()}catch{ElMessage.error('重启失败')}finally{acting.value=false}}

async function showVersions(){verVisible.value=true;verLoading.value=true;try{const d=await client.get('/binaries/releases',{params:{core_type:selectedCore.value,limit:20,offset:0}});releases.value=(d.items||[]).map(r=>({...r,prerelease:r.prerelease??isOdd(r.version)}))}catch{releases.value=[]}finally{verLoading.value=false}}
function isOdd(v){const p=(v||'').replace(/^v/,'').split('.');const n=parseInt(p[p.length-1]);return!isNaN(n)&&n%2!==0}
async function doDownload(r){
  await ElMessageBox.confirm(`下载 ${r.version}？`,'确认')
  downloading.value=true
  downloadProgress.value='正在下载 '+r.version+'...'
  try{
    await client.post('/binaries/download',{core_type:selectedCore.value,version:r.version})
    downloadProgress.value='下载完成，正在启动服务...'
    // Start service
    try{await client.post(`/services/${svc.service_id}/start`)}catch{}
    // Check immediately
    await new Promise(resolve=>setTimeout(resolve,500))
    try{
      const data=await client.get('/services/process-status')
      const s=(data||[]).find(x=>x.core_type===selectedCore.value)
      if(s&&s.state==='running'){
        ElMessage.success('更新完成')
        verVisible.value=false
        fetchStatus()
        return
      }
    }catch{}
    // Wait a bit more if not running yet
    for(let i=0;i<5;i++){
      await new Promise(resolve=>setTimeout(resolve,1000))
      try{
        const data=await client.get('/services/process-status')
        const s=(data||[]).find(x=>x.core_type===selectedCore.value)
        if(s&&s.state==='running'){
          ElMessage.success('更新完成')
          verVisible.value=false
          fetchStatus()
          return
        }
      }catch{}
    }
    ElMessage.success('下载完成')
    verVisible.value=false
    fetchStatus()
  }catch{ElMessage.error('下载失败')}finally{downloading.value=false}
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
.ver-card{width:calc(16.66%-8px);min-width:88px;padding:8px;border:1px solid #dcdfe6;border-radius:6px;cursor:pointer;text-align:center;transition:all .2s}
.ver-card:hover{border-color:#409eff;background:#ecf5ff}
</style>
