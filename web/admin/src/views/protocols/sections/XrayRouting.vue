<template>
  <div>
    <el-form label-width="140px" label-position="left">
      <el-form-item label="Domain Strategy">
        <el-select v-model="form.domainStrategy" style="width:100%" @change="emit">
          <el-option label="AsIs" value="AsIs" /><el-option label="IPIfNonMatch" value="IPIfNonMatch" /><el-option label="IPOnDemand" value="IPOnDemand" />
        </el-select>
      </el-form-item>
      <el-form-item label="Domain Matcher">
        <el-select v-model="form.domainMatcher" style="width:100%" clearable @change="emit">
          <el-option label="default" value="" /><el-option label="hybrid" value="hybrid" /><el-option label="linear" value="linear" />
        </el-select>
      </el-form-item>
    </el-form>

    <div class="sub-section">
      <div class="sub-header"><span>Routing Rules</span><el-button size="small" type="primary" @click="addRule">Add</el-button></div>
      <div v-for="(r, i) in form.rules" :key="i" class="rule-item">
        <el-card shadow="never">
          <div class="rule-row">
            <el-select v-model="r.outboundTag" placeholder="Outbound" style="width:18%" @change="emit">
              <el-option v-for="t in outboundTags" :key="t" :label="t" :value="t" />
            </el-select>
            <el-select v-model="r.protocol" multiple placeholder="Protocol" style="width:20%" @change="emit">
              <el-option label="http" value="http" /><el-option label="tls" value="tls" />
              <el-option label="bittorrent" value="bittorrent" /><el-option label="quic" value="quic" />
            </el-select>
            <el-input v-model="r.domainStr" placeholder="Domains (comma-sep)" style="width:25%" @change="emit" />
            <el-input v-model="r.ipStr" placeholder="IPs (comma-sep)" style="width:25%" @change="emit" />
            <el-button size="small" type="danger" text @click="rmRule(i)">X</el-button>
          </div>
          <div class="rule-row" style="margin-top:6px">
            <el-input v-model="r.port" placeholder="Port" style="width:15%" @change="emit" />
            <el-select v-model="r.network" placeholder="Network" style="width:12%" clearable @change="emit">
              <el-option label="tcp" value="tcp" /><el-option label="udp" value="udp" />
            </el-select>
            <el-input v-model="r.sourceStr" placeholder="Source IP (comma-sep)" style="width:20%" @change="emit" />
            <el-input v-model="r.inboundTagStr" placeholder="Inbound Tags (comma-sep)" style="width:20%" @change="emit" />
            <el-input v-model="r.userStr" placeholder="Users (comma-sep)" style="width:15%" @change="emit" />
          </div>
        </el-card>
      </div>
    </div>

    <div class="sub-section">
      <div class="sub-header"><span>Balancers</span><el-button size="small" type="primary" @click="addBalancer">Add</el-button></div>
      <div v-for="(b, i) in form.balancers" :key="i" class="sub-item">
        <el-input v-model="b.tag" placeholder="Tag" style="width:20%" @change="emit" />
        <el-input v-model="b.selectorStr" placeholder="Selectors (comma-sep)" style="width:30%" @change="emit" />
        <el-select v-model="b.strategyType" style="width:20%" @change="emit">
          <el-option label="random" value="random" /><el-option label="leastPing" value="leastPing" /><el-option label="leastLoad" value="leastLoad" />
        </el-select>
        <el-button size="small" type="danger" text @click="rmBalancer(i)">X</el-button>
      </div>
    </div>
  </div>
</template>
<script setup>
import { reactive, ref, onMounted } from 'vue'
import client from '../../../api/client'
const props = defineProps({ data: Object, coreType: String })
const emitFn = defineEmits(['update'])
const form = reactive({ domainStrategy:'IPIfNonMatch', domainMatcher:'', rules:[], balancers:[] })
const outboundTags = ref(['direct', 'blocked'])

async function loadOutbounds() {
  try {
    const data = await client.get(`/protocols/${props.coreType}/section/outbounds`)
    if (Array.isArray(data)) {
      const tags = data.map(function(o) { return o.tag }).filter(Boolean)
      outboundTags.value = [...new Set(['direct', 'blocked', ...tags])]
    }
  } catch {}
}

function sync(){
  if(!props.data)return
  form.domainStrategy=props.data.domainStrategy||'IPIfNonMatch'
  form.domainMatcher=props.data.domainMatcher||''
  form.rules=(props.data.rules||[]).map(function(r) {
    return {
      outboundTag:r.outboundTag||'',protocol:r.protocol||[],
      domainStr:(r.domain||[]).join(', '),ipStr:(r.ip||[]).join(', '),
      port:r.port||'',network:r.network||'',sourceStr:(r.source||[]).join(', '),
      inboundTagStr:(r.inboundTag||[]).join(', '),userStr:(r.user||[]).join(', '),
    }
  })
  form.balancers=(props.data.balancers||[]).map(function(b) {
    return {tag:b.tag||'',selectorStr:(b.selector||[]).join(', '),strategyType:b.strategy?.type||'random'}
  })
}

function build(){
  return{domainStrategy:form.domainStrategy,domainMatcher:form.domainMatcher||undefined,
    rules:form.rules.filter(function(r){return r.outboundTag}).map(function(r){return{
      type:'field',outboundTag:r.outboundTag,
      protocol:r.protocol.length?r.protocol:undefined,
      domain:r.domainStr?r.domainStr.split(',').map(function(d){return d.trim()}).filter(Boolean):undefined,
      ip:r.ipStr?r.ipStr.split(',').map(function(d){return d.trim()}).filter(Boolean):undefined,
      port:r.port||undefined,network:r.network||undefined,
      source:r.sourceStr?r.sourceStr.split(',').map(function(d){return d.trim()}).filter(Boolean):undefined,
      inboundTag:r.inboundTagStr?r.inboundTagStr.split(',').map(function(d){return d.trim()}).filter(Boolean):undefined,
      user:r.userStr?r.userStr.split(',').map(function(d){return d.trim()}).filter(Boolean):undefined,
    }}),
    balancers:form.balancers.filter(function(b){return b.tag}).map(function(b){return{
      tag:b.tag,selector:b.selectorStr?b.selectorStr.split(',').map(function(s){return s.trim()}).filter(Boolean):[],
      strategy:{type:b.strategyType},
    }})}
}

function emit(){emitFn('update',build())}
function addRule(){form.rules.push({outboundTag:'',protocol:[],domainStr:'',ipStr:'',port:'',network:'',sourceStr:'',inboundTagStr:'',userStr:''});emit()}
function rmRule(i){form.rules.splice(i,1);emit()}
function addBalancer(){form.balancers.push({tag:'',selectorStr:'',strategyType:'random'});emit()}
function rmBalancer(i){form.balancers.splice(i,1);emit()}

onMounted(function(){sync();loadOutbounds()})
</script>
<style scoped>.sub-section{margin-top:20px}.sub-header{display:flex;justify-content:space-between;align-items:center;margin-bottom:12px;font-weight:600}.sub-item{display:flex;gap:8px;margin-bottom:8px;align-items:center}.rule-item{margin-bottom:12px}.rule-row{display:flex;gap:6px;align-items:center}</style>
