<template>
  <div>
    <!-- Global Settings -->
    <el-divider content-position="left">Global Settings</el-divider>
    <el-form label-width="160px" label-position="left">
      <el-form-item label="Final Outbound">
        <el-select v-model="form.final" style="width:100%" filterable allow-create clearable @change="emit">
          <el-option v-for="t in outboundTags" :key="t" :label="t" :value="t" />
        </el-select>
        <div class="hint">
          未命中任何规则的流量走该出站。**留空 = 使用出站列表第一个**（与 xray 的默认出站语义一致）；
          想让代理出口兜底：选出口节点 tag，或清空后把非直连出站在「出站」列表里上移到第一位。
        </div>
      </el-form-item>
      <el-form-item label="Auto Detect Interface">
        <el-switch v-model="form.auto_detect_interface" @change="emit" />
      </el-form-item>
      <el-form-item label="Default Domain Resolver">
        <el-input v-model="form.default_domain_resolver" placeholder="dns-remote" @change="emit" />
        <div class="hint">Default DNS server tag for domain resolution</div>
      </el-form-item>
      <el-form-item label="Default Network Strategy">
        <el-select v-model="form.default_network_strategy" style="width:100%" clearable @change="onNetStrategyChange">
          <el-option label="(默认)" value="" />
          <el-option label="hybrid" value="hybrid" />
          <el-option label="fallback" value="fallback" />
        </el-select>
        <div class="hint">
          多网卡/多网络时的选路策略。实测合法值**只有 hybrid / fallback**（写别的核心会 FATAL
          `unknown network strategy`）；且必须同时开启上面的 Auto Detect Interface。
        </div>
      </el-form-item>
      <el-form-item label="Default Network Type">
        <el-select v-model="form.default_network_type" multiple clearable style="width:100%" @change="emit">
          <el-option label="wifi" value="wifi" /><el-option label="cellular" value="cellular" />
          <el-option label="ethernet" value="ethernet" /><el-option label="other" value="other" />
        </el-select>
      </el-form-item>
      <el-form-item label="Default Fallback Network Type">
        <el-select v-model="form.default_fallback_network_type" multiple clearable style="width:100%" @change="emit">
          <el-option label="wifi" value="wifi" /><el-option label="cellular" value="cellular" />
          <el-option label="ethernet" value="ethernet" /><el-option label="other" value="other" />
        </el-select>
        <div class="hint">strategy=fallback 时的退化网络类型</div>
      </el-form-item>
      <el-form-item label="Default Fallback Delay">
        <el-input v-model="form.default_fallback_delay" placeholder="300ms" @change="emit" />
        <div class="hint">回退等待时长（duration，如 300ms / 1s；非法值核心会拒绝启动）</div>
      </el-form-item>
      <el-form-item label="Default HTTP Client">
        <el-input v-model="form.default_http_client" placeholder="出站 tag" @change="emit" />
        <div class="hint">内部 HTTP 请求（如规则集下载）默认走哪个出站</div>
      </el-form-item>
      <el-form-item label="Default Strategy">
        <el-select v-model="form.default_domain_strategy" style="width:100%" clearable @change="emit">
          <el-option label="" value="" />
          <el-option label="prefer_ipv4" value="prefer_ipv4" />
          <el-option label="prefer_ipv6" value="prefer_ipv6" />
          <el-option label="ipv4_only" value="ipv4_only" />
          <el-option label="ipv6_only" value="ipv6_only" />
        </el-select>
      </el-form-item>
    </el-form>

    <!-- Rule Sets（route.rule_set 定义；此前面板只能靠 API 手写） -->
    <el-divider content-position="left">
      Rule Sets
      <el-button size="small" type="primary" @click="addRuleSet" style="margin-left:12px">Add Rule Set</el-button>
    </el-divider>
    <div v-if="!form.ruleSets.length" class="hint" style="margin-bottom:8px">
      未定义任何 rule_set。规则里引用 rule_set 前，先在这里定义（remote 从 URL 拉取 / local 读本地文件 / inline 直接写规则）。
    </div>
    <div v-for="(rs, ri) in form.ruleSets" :key="'rs' + ri"
         style="border:1px dashed #dcdfe6;border-radius:6px;padding:10px;margin-bottom:10px">
      <div style="display:flex;gap:8px;align-items:center">
        <el-select v-model="rs.type" style="width:140px" @change="emit">
          <el-option label="remote" value="remote" /><el-option label="local" value="local" />
          <el-option label="inline" value="inline" />
        </el-select>
        <el-input v-model="rs.tag" placeholder="tag（规则里用它引用；多个用逗号分隔）" style="flex:1" @change="emit" />
        <el-select v-model="rs.format" style="width:130px" clearable @change="emit">
          <el-option label="(默认)" value="" /><el-option label="source" value="source" />
          <el-option label="binary" value="binary" />
        </el-select>
        <el-button size="small" type="danger" text @click="form.ruleSets.splice(ri,1);emit()">X</el-button>
      </div>
      <template v-if="rs.type === 'remote'">
        <el-input v-model="rs.url" placeholder="https://…/geosite-cn.srs" style="margin-top:8px" @change="emit" />
        <div style="display:flex;gap:8px;margin-top:8px">
          <el-input v-model="rs.update_interval" placeholder="更新间隔，如 1d" @change="emit" />
          <el-input v-model="rs.download_detour" placeholder="download_detour（出站 tag）" @change="emit" />
          <el-input v-model="rs.http_client" placeholder="http_client tag" @change="emit" />
        </div>
        <el-input v-model="rs.initial_path" placeholder="initial_path，如 /etc/sing-box/<tag>.srs" style="margin-top:8px" @change="emit" />
        <div class="hint">initial_path 必须包含 <code>&lt;tag&gt;</code> 占位符（实测否则 FATAL `missing &lt;tag&gt; placeholder in initial_path`），留空则用缓存目录默认值</div>
      </template>
      <template v-else-if="rs.type === 'local'">
        <el-select v-model="rs.path" filterable allow-create default-first-option
          placeholder="从规则库选择，或手动输入路径" style="width:100%;margin-top:8px" @change="onLocalPathChange(rs)">
          <el-option v-for="f in geoRuleFiles" :key="f.id" :value="`rules/${f.file_name}`"
            :label="`rules/${f.file_name}（${formatSize(f.file_size)}）`" />
        </el-select>
        <div class="hint">
          下拉选项来自「规则文件管理」已下载到 <code>data/rules</code> 的文件（面板与节点同路径，相对工作目录解析）；
          也可手动输入路径（核心启动时**读取该文件**：路径不存在会 FATAL）。
          选 <code>.srs</code> 自动带 format=binary、<code>.json</code> 自动带 source。
        </div>
      </template>
      <template v-else>
        <el-input v-model="rs._rulesJson" type="textarea" :rows="3" style="margin-top:8px"
          placeholder='[{"domain_suffix": ["a.example"]}]' @change="emit" />
        <div class="hint">inline 规则数组（JSON）。注意：inline **不支持多 tag**（实测 FATAL `inline rule-set does not support multiple tags`）</div>
      </template>
    </div>


    <!-- Route Rules -->
    <el-divider content-position="left">
      Route Rules
      <el-button size="small" type="primary" @click="showAddRule" style="margin-left:12px">Add Rule</el-button>
    </el-divider>

    <div v-for="(r, i) in rulesPaged" :key="'rr'+i" class="rule-item">
      <div style="display:flex;justify-content:space-between;align-items:center">
        <div style="display:flex;align-items:center;gap:8px;flex-wrap:wrap">
          <el-tag v-if="r.type === 'logical'" size="small" type="warning">logical/{{ r.mode }}</el-tag>
          <el-tag v-if="r.type === 'logical' && r.rules" size="small">{{ (r.rules || []).length }} sub-rules</el-tag>
          <el-tag v-if="r.invert" size="small" type="danger">invert</el-tag>
          <el-tag v-if="r.action" size="small" :type="actionTagType(r.action)">{{ r.action }}</el-tag>
          <el-tag v-if="r.outbound" size="small" :type="r.outbound==='block'?'danger':r.outbound==='direct'?'success':'info'">{{ r.outbound }}</el-tag>
          <span v-if="r._domain" style="font-size:12px;color:#909399">domains: {{ r._domain }}</span>
          <span v-if="r._domainSuffix" style="font-size:12px;color:#909399">suffix: {{ r._domainSuffix }}</span>
          <span v-if="r._ipCIDR" style="font-size:12px;color:#909399">ip: {{ r._ipCIDR }}</span>
          <span v-if="r._ruleSet?.length" style="font-size:12px;color:#909399">rule_set: {{ r._ruleSet.join(',') }}</span>
          <span v-if="r._protocol?.length" style="font-size:12px;color:#909399">{{ r._protocol.join(',') }}</span>
          <span v-if="r.port?.length" style="font-size:12px;color:#909399">port: {{ (r.port || []).join(',') }}</span>
          <span v-if="r.port_range?.length" style="font-size:12px;color:#909399">port_range: {{ (r.port_range || []).join(',') }}</span>
          <span v-if="r._inboundTag" style="font-size:12px;color:#909399">inbound: {{ r._inboundTag }}</span>
          <span v-if="r._authUser" style="font-size:12px;color:#909399">auth_user: {{ r._authUser }}</span>
        </div>
        <div>
          <el-button size="small" @click="editRule(r)">Edit</el-button>
          <el-button size="small" type="danger" text @click="rmRule(r)">X</el-button>
        </div>
      </div>
    </div>
    <PaginationBar v-model:page="rulePage" v-model:page-size="ruleSize" :total="ruleTotal" />
    <div v-if="!form.rules.length" style="text-align:center;color:#909399;padding:24px">No rules</div>

    <!-- Rule Dialog -->
    <el-dialog v-model="dlg" :title="editIdx >= 0 ? 'Edit Route Rule' : 'Add Route Rule'" width="760" top="5vh">
      <el-form label-width="150px" label-position="left">
        <el-divider content-position="left">Rule Type</el-divider>
        <el-row :gutter="12">
          <el-col :span="12">
            <el-form-item label="Type">
              <el-select v-model="rf.type" style="width:100%">
                <el-option label="default" value="" />
                <el-option label="logical" value="logical" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item v-if="rf.type === 'logical'" label="Mode">
              <el-select v-model="rf.mode" style="width:100%">
                <el-option label="and" value="and" />
                <el-option label="or" value="or" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>

        <!-- Logical rule: sub-rules (sing-box 要求 type=logical 时 rules 必填) -->
        <template v-if="rf.type === 'logical'">
          <el-form-item label="Sub Rules (JSON)">
            <el-input v-model="rf._subRules" type="textarea" :rows="8"
              placeholder='[{"domain_suffix":[".example.com"]},{"ip_cidr":["10.0.0.0/8"]}]' />
            <div class="hint">
              JSON 数组，元素为普通规则对象。example:
              <code>[{"domain_suffix":[".example.com"]},{"port_range":["1000:2000"]}]</code>
            </div>
          </el-form-item>
          <el-form-item label="Invert">
            <el-switch v-model="rf.invert" />
            <span class="hint-inline">逻辑取反（不匹配子规则时命中）</span>
          </el-form-item>
        </template>

        <!-- Default rule: match conditions -->
        <template v-else>
          <el-divider content-position="left">Match Conditions</el-divider>
          <el-form-item label="Domain">
            <el-input v-model="rf._domain" type="textarea" :rows="2" placeholder="One domain per line" />
          </el-form-item>
          <el-form-item label="Domain Suffix">
            <el-input v-model="rf._domainSuffix" type="textarea" :rows="2" placeholder="One suffix per line" />
          </el-form-item>
          <el-form-item label="Domain Keyword">
            <el-input v-model="rf._domainKeyword" type="textarea" :rows="2" placeholder="One keyword per line" />
          </el-form-item>
          <el-form-item label="Domain Regex">
            <el-input v-model="rf._domainRegex" type="textarea" :rows="2" placeholder="One regex per line" />
          </el-form-item>

          <el-form-item label="IP CIDR">
            <el-input v-model="rf._ipCIDR" type="textarea" :rows="2" placeholder="One CIDR per line" />
          </el-form-item>
          <el-form-item label="Source IP CIDR">
            <el-input v-model="rf._sourceIPCIDR" type="textarea" :rows="2" placeholder="One CIDR per line" />
          </el-form-item>
          <el-row :gutter="12">
            <el-col :span="8">
              <el-form-item label="IP Version">
                <el-select v-model="rf._ipVersion" style="width:100%" clearable>
                  <el-option label="any" value="" />
                  <el-option label="IPv4" :value="4" />
                  <el-option label="IPv6" :value="6" />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="IP Is Private">
                <el-switch v-model="rf._ipIsPrivate" />
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="Src Is Private">
                <el-switch v-model="rf._sourceIPIsPrivate" />
              </el-form-item>
            </el-col>
          </el-row>

          <el-row :gutter="12">
            <el-col :span="12">
              <el-form-item label="Port">
                <el-input v-model="rf._port" placeholder="53,443 (single ports)" />
                <div class="hint">仅单个端口，逗号分隔；端口范围请用下面的 Port Range</div>
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="Port Range">
                <el-input v-model="rf._portRange" placeholder="1000:2000,3000:4000" />
              </el-form-item>
            </el-col>
          </el-row>
          <el-row :gutter="12">
            <el-col :span="12">
              <el-form-item label="Source Port">
                <el-input v-model="rf._sourcePort" placeholder="53" />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="Source Port Range">
                <el-input v-model="rf._sourcePortRange" placeholder="1000:2000" />
              </el-form-item>
            </el-col>
          </el-row>

          <el-row :gutter="12">
            <el-col :span="8"><el-form-item label="Network"><el-select v-model="rf._network" clearable style="width:100%"><el-option label="tcp" value="tcp" /><el-option label="udp" value="udp" /><el-option label="tcp,udp" value="tcp,udp" /></el-select></el-form-item></el-col>
            <el-col :span="16"><el-form-item label="Protocol"><el-select v-model="rf._protocol" multiple style="width:100%"><el-option label="http" value="http" /><el-option label="tls" value="tls" /><el-option label="quic" value="quic" /><el-option label="stun" value="stun" /><el-option label="dns" value="dns" /><el-option label="bittorrent" value="bittorrent" /><el-option label="dtls" value="dtls" /><el-option label="ssh" value="ssh" /><el-option label="rdp" value="rdp" /><el-option label="ntp" value="ntp" /></el-select></el-form-item></el-col>
          </el-row>

          <el-form-item label="Inbound Tag">
            <el-select v-model="rf._inboundTag" multiple filterable allow-create default-first-option placeholder="Select or type" style="width:100%">
              <el-option v-for="tag in inboundTags" :key="tag" :label="tag" :value="tag" />
            </el-select>
          </el-form-item>
          <el-row :gutter="12">
            <el-col :span="12">
              <el-form-item label="Auth User">
                <el-input v-model="rf._authUser" type="textarea" :rows="2" placeholder="One inbound auth user per line" />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="User">
                <el-input v-model="rf._user" type="textarea" :rows="2" placeholder="One user name per line" />
              </el-form-item>
            </el-col>
          </el-row>
          <el-form-item label="Client">
            <el-input v-model="rf._client" placeholder="Client identifier (comma separated)" />
          </el-form-item>

          <el-form-item label="Process Name">
            <el-input v-model="rf._process" placeholder="One process name per line" type="textarea" :rows="2" />
          </el-form-item>
          <el-form-item label="Process Path">
            <el-input v-model="rf._processPath" placeholder="One full path per line" type="textarea" :rows="2" />
          </el-form-item>
          <el-form-item label="Process Path Regex">
            <el-input v-model="rf._processPathRegex" placeholder="One regex per line" type="textarea" :rows="2" />
          </el-form-item>

          <el-form-item label="Rule Set">
            <el-select v-model="rf._ruleSet" multiple filterable allow-create default-first-option
              placeholder="选择上方 Rule Sets 已定义的 tag（也可手填）" style="width:100%">
              <el-option v-for="rs in ruleSetTags" :key="rs" :label="rs" :value="rs" />
            </el-select>
          </el-form-item>
          <el-form-item label="Match Source IP" v-if="rf._ruleSet?.length">
            <el-switch v-model="rf._ruleSetIPCIDRMatchSource" />
            <span class="hint-inline">rule_set 中的 ip_cidr 匹配源地址（rule_set_ip_cidr_match_source）</span>
          </el-form-item>
          <el-form-item label="Clash Mode">
            <el-select v-model="rf._clashMode" style="width:100%" clearable filterable allow-create @change="emit">
              <el-option label="(不限)" value="" /><el-option label="rule" value="rule" />
              <el-option label="global" value="global" /><el-option label="direct" value="direct" />
            </el-select>
            <div class="hint">按外部控制器（clash_api）当前模式匹配</div>
          </el-form-item>
          <el-form-item label="Interface Address (JSON)">
            <el-input v-model="rf._interfaceAddressJson" type="textarea" :rows="2"
              placeholder='{"eth0": ["192.168.1.10/32"]}' @change="emit" />
            <div class="hint">本地**接口名 → 地址段**映射（JSON，map 形式）；JSON 非法会被忽略</div>
          </el-form-item>
          <el-form-item label="Network Interface Address (JSON)">
            <el-input v-model="rf._networkInterfaceAddressJson" type="textarea" :rows="2"
              placeholder='{"wifi": ["10.0.0.1/32"]}' @change="emit" />
            <div class="hint">按**网络类型 → 地址段**匹配（JSON；键是 wifi/cellular/ethernet/other）</div>
          </el-form-item>
          <el-form-item label="Default Interface Address">
            <el-input v-model="rf._defaultInterfaceAddress" type="textarea" :rows="2" placeholder="10.0.0.1/32" @change="emit" />
            <div class="hint">系统未识别接口时的兜底地址段，每行一条</div>
          </el-form-item>

          <el-form-item label="Invert">
            <el-switch v-model="rf.invert" />
            <span class="hint-inline">整条规则取反</span>
          </el-form-item>

          <el-collapse>
            <el-collapse-item title="Legacy GeoIP / Geosite（xray 兼容字段，sing-box 1.8+ 建议改用 rule_set）">
              <el-form-item label="Geosite">
                <el-input v-model="rf._geosite" type="textarea" :rows="2" placeholder="One code per line" />
              </el-form-item>
              <el-form-item label="GeoIP">
                <el-input v-model="rf._geoip" type="textarea" :rows="2" placeholder="One code per line" />
              </el-form-item>
              <el-form-item label="Source GeoIP">
                <el-input v-model="rf._sourceGeoip" type="textarea" :rows="2" placeholder="One code per line" />
              </el-form-item>
            </el-collapse-item>
          </el-collapse>
        </template>

        <el-divider content-position="left">Action</el-divider>
        <el-form-item label="Action">
          <el-select v-model="rf.action" style="width:100%" clearable>
            <el-option label="route (default)" value="" />
            <el-option label="route-options" value="route-options" />
            <el-option label="reject" value="reject" />
            <el-option label="hijack-dns" value="hijack-dns" />
            <el-option label="sniff" value="sniff" />
            <el-option label="resolve" value="resolve" />
          </el-select>
        </el-form-item>
        <template v-if="rf.action === 'reject'">
          <el-form-item label="Reject Method">
            <el-select v-model="rf.rejectMethod" style="width:100%" clearable @change="onRejectMethodChange">
              <el-option label="default" value="default" />
              <el-option label="drop" value="drop" />
              <el-option label="reply" value="reply" />
            </el-select>
            <div class="hint">拒绝方式（default=关连接 / drop=直接丢包 / reply=回 ICMP 或 TCP RST）</div>
          </el-form-item>
          <el-form-item v-if="rf.rejectMethod !== 'drop'" label="No Drop">
            <el-switch v-model="rf.noDrop" @change="emit" />
            <div class="hint">
              不立刻断开该连接（"静默拦截"体验）；**method=drop 时不能开** ——
              实测会 FATAL `no_drop is not available in current context`
            </div>
          </el-form-item>
        </template>
        <el-form-item label="Outbound">
          <el-select v-model="rf.outbound" style="width:100%" clearable filterable allow-create>
            <el-option v-for="t in outboundTags" :key="t" :label="t" :value="t" />
          </el-select>
          <div class="hint">Outbound tag for matched traffic</div>
        </el-form-item>
        <!-- route-options 动作用的字段（与 action 同级写在 rule 顶层） -->
        <template v-if="rf.action === 'route-options'">
          <el-form-item label="TLS Fragment">
            <el-switch v-model="rf.tlsFragment" @change="onTlsFragmentChange" />
            <div class="hint">
              对命中流量做 TLS 分片（抗特征）；与下面的 Record Fragment <b>互斥</b>
              —— 同时给核心会拒绝启动（实测 `tls_fragment and tls_record_fragment are mutually exclusive`）
            </div>
          </el-form-item>
          <el-form-item label="TLS Record Fragment">
            <el-switch v-model="rf.tlsRecordFragment" @change="onTlsRecordFragmentChange" />
            <div class="hint">按 TLS 记录分片；与上面的 TLS Fragment 互斥</div>
          </el-form-item>
          <el-form-item v-if="rf.tlsFragment || rf.tlsRecordFragment" label="Fragment Fallback Delay">
            <el-input v-model="rf.tlsFragmentFallbackDelay" placeholder="100ms" @change="emit" />
            <div class="hint">分片失败后的回退延迟（duration，如 100ms / 1s；非法值核心会拒绝启动）</div>
          </el-form-item>
          <el-form-item label="Network Strategy">
            <el-select v-model="rf.networkStrategy" style="width:100%" clearable @change="emit">
              <el-option label="(默认)" value="" /><el-option label="hybrid" value="hybrid" />
              <el-option label="fallback" value="fallback" />
            </el-select>
            <div class="hint">该规则命中的连接用哪种选路策略（只有 hybrid / fallback 合法）</div>
          </el-form-item>
          <el-form-item label="Fallback Delay">
            <el-input-number v-model="rf.fallbackDelay" :min="0" @change="emit" />
            <div class="hint">回退延迟，单位毫秒（0 = 不写）</div>
          </el-form-item>
          <el-form-item label="UDP Disable Domain Unmapping">
            <el-switch v-model="rf.udpDisableDomainUnmapping" @change="emit" />
            <div class="hint">关闭 UDP 的域名反查（该字段 1.13 起已从 inbound 迁到规则动作）</div>
          </el-form-item>
          <el-form-item label="UDP Connect">
            <el-switch v-model="rf.udpConnect" @change="emit" />
          </el-form-item>
          <el-form-item label="UDP Timeout">
            <el-input v-model="rf.udpTimeout" placeholder="5m" @change="emit" />
            <div class="hint">UDP 会话超时（duration；非法值核心会拒绝启动）</div>
          </el-form-item>
          <el-form-item label="TLS Spoof">
            <el-input v-model="rf.tlsSpoof" placeholder="www.bing.com" @change="emit" />
            <div class="hint">本 fork 的 TLS spoof 的 route-options 形式（伪装目标，域名或 IP）；留空不启用</div>
          </el-form-item>
          <el-form-item v-if="rf.tlsSpoof" label="TLS Spoof Method">
            <el-select v-model="rf.tlsSpoofMethod" style="width:100%" clearable @change="emit">
              <el-option label="(默认)" value="" />
              <el-option label="wrong-sequence" value="wrong-sequence" />
              <el-option label="wrong-checksum" value="wrong-checksum" />
              <el-option label="wrong-ack" value="wrong-ack" />
              <el-option label="wrong-md5" value="wrong-md5" />
              <el-option label="wrong-timestamp" value="wrong-timestamp" />
            </el-select>
            <div class="hint">必须配合上面的 TLS Spoof（实测：单独给会 FATAL `spoof_method requires spoof`）</div>
          </el-form-item>
        </template>
        <!-- 规则级 strategy / domain_resolver 都不是 sing-box 字段；
             action=resolve 时对应字段是 server + strategy -->
        <template v-if="rf.action === 'resolve'">
          <el-form-item label="Resolve DNS Server">
            <el-input v-model="rf.resolveServer" placeholder="DNS server tag（resolve 动作用 server，不是 domain_resolver）" />
          </el-form-item>
          <el-form-item label="Strategy">
            <el-select v-model="rf.strategy" style="width:100%" clearable>
              <el-option label="" value="" />
              <el-option label="prefer_ipv4" value="prefer_ipv4" />
              <el-option label="prefer_ipv6" value="prefer_ipv6" />
              <el-option label="ipv4_only" value="ipv4_only" />
              <el-option label="ipv6_only" value="ipv6_only" />
            </el-select>
          </el-form-item>
        </template>
        <el-form-item v-else label=" ">
          <span class="hint">Strategy / DNS Server 仅在 Action = resolve 时可用（sing-box 会拒绝其它情况下的这两种字段）</span>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dlg=false">Cancel</el-button>
        <el-button type="primary" @click="saveRule">Save</el-button>
      </template>
    </el-dialog>
  </div>
</template>
<script setup>
import { computed, reactive, ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import client from '../../../api/client'
import { useClientPagination } from '../../../composables/useClientPagination'
import PaginationBar from '../../../components/PaginationBar.vue'
const props = defineProps({ data: Object, coreType: String, serverId: Number, fullConfig: Object })
const emitFn = defineEmits(['update'])

const form = reactive({
  final: 'direct', auto_detect_interface: true,
  default_domain_resolver: '', default_domain_strategy: '',
  default_network_strategy: '', default_network_type: [], default_fallback_network_type: [],
  default_fallback_delay: '', default_http_client: '',
  rules: [],
  ruleSets: [],
})

const { paged: rulesPaged, page: rulePage, pageSize: ruleSize, total: ruleTotal } = useClientPagination(computed(() => form.rules), { pageKey: 'sbRoutePage', pageSizeKey: 'sbRouteSize' })
const outboundTags = ref(['direct', 'block'])
const inboundTags = ref([])

// Route Rules 里可引用的 rule_set tag —— 实时取自 Rule Sets 编辑区的当前定义
//（含"刚加还没保存"的），不再只在页面加载时从 props.data 读一次（否则新定义的
// rule_set 在规则编辑对话框里选不到）；多 tag 定义（逗号分隔）逐一展平并去重。
const ruleSetTags = computed(() => {
  const tags = []
  const push = (t) => { t = String(t || '').trim(); if (t && !tags.includes(t)) tags.push(t) }
  for (const rs of form.ruleSets) commas(rs.tag).forEach(push)
  for (const rs of (props.data?.rule_set || [])) {
    (Array.isArray(rs?.tag) ? rs.tag : [rs?.tag]).forEach(push)
  }
  return tags
})

const dlg = ref(false)
const editIdx = ref(-1)

// 空白规则模板（rf 的所有 UI 字段）
function blankRule() {
  return {
    type: '', mode: '', _subRules: '', invert: false,
    _domain: '', _domainSuffix: '', _domainKeyword: '', _domainRegex: '',
    _ipCIDR: '', _sourceIPCIDR: '', _ipVersion: '', _ipIsPrivate: false, _sourceIPIsPrivate: false,
    _port: '', _portRange: '', _sourcePort: '', _sourcePortRange: '',
    _network: '', _protocol: [], _inboundTag: [], _authUser: '', _user: '', _client: '',
    _process: '', _processPath: '', _processPathRegex: '',
    _ruleSet: [], _ruleSetIPCIDRMatchSource: false,
    _geosite: '', _geoip: '', _sourceGeoip: '',
    _clashMode: '', _interfaceAddressJson: '', _networkInterfaceAddressJson: '', _defaultInterfaceAddress: '',
    rejectMethod: '', noDrop: false,
    udpDisableDomainUnmapping: false, udpConnect: false, udpTimeout: '', networkStrategy: '', fallbackDelay: 0,
    action: '', outbound: '', resolveServer: '', strategy: '',
    tlsFragment: false, tlsRecordFragment: false, tlsFragmentFallbackDelay: '',
    tlsSpoof: '', tlsSpoofMethod: '',
  }
}

const rf = reactive(blankRule())

function actionTagType(a) {
  if (a === 'reject') return 'danger'
  if (a === 'hijack-dns') return 'warning'
  if (a === 'sniff') return 'success'
  return 'info'
}

function resetForm() { Object.assign(rf, blankRule()) }

function showAddRule() { editIdx.value = -1; resetForm(); dlg.value = true }

// 多行文本 → 数组
function lines(v) {
  return v ? String(v).split('\n').map(s => s.trim()).filter(Boolean) : []
}
// 逗号分隔 → 数组
function commas(v) {
  return v ? String(v).split(',').map(s => s.trim()).filter(Boolean) : []
}
// 端口输入（逗号分隔的单端口）→ 数字数组；sing-box 的 port/source_port 是 uint16 列表
function portNumbers(v) {
  const arr = commas(v)
  const nums = []
  for (const s of arr) {
    if (!/^\d+$/.test(s)) return { ok: false, bad: s, nums: [] }
    const n = Number(s)
    if (n < 1 || n > 65535) return { ok: false, bad: s, nums: [] }
    nums.push(n)
  }
  return { ok: true, bad: '', nums }
}
// 数字数组 → 逗号字符串（供表单回显）
function portStr(v) {
  if (Array.isArray(v)) return v.join(',')
  return v ? String(v) : ''
}

function editRule(r) {
  editIdx.value = form.rules.indexOf(r)
  Object.assign(rf, blankRule(), {
    type: r.type || '', mode: r.mode || '',
    _subRules: r.type === 'logical' && r.rules ? JSON.stringify(r.rules, null, 2) : (r._subRules || ''),
    invert: !!r.invert,
    _domain: r._domain || '', _domainSuffix: r._domainSuffix || '',
    _domainKeyword: r._domainKeyword || '', _domainRegex: r._domainRegex || '',
    _ipCIDR: r._ipCIDR || '', _sourceIPCIDR: r._sourceIPCIDR || '',
    _ipVersion: r.ip_version || '',
    _ipIsPrivate: !!r.ip_is_private, _sourceIPIsPrivate: !!r.source_ip_is_private,
    _port: portStr(r.port), _portRange: (r.port_range || []).join(','),
    _sourcePort: portStr(r.source_port), _sourcePortRange: (r.source_port_range || []).join(','),
    _network: r.network || '', _protocol: r.protocol || [],
    _inboundTag: r.inbound || r.inbound_tag || [],
    _authUser: (r.auth_user || []).join('\n'), _user: (r.user || []).join('\n'),
    _client: (r.client || []).join(','),
    _process: (r.process_name || r.process || []).join('\n'),
    _processPath: (r.process_path || []).join('\n'),
    _processPathRegex: (r.process_path_regex || []).join('\n'),
    _ruleSet: r.rule_set || [], _ruleSetIPCIDRMatchSource: !!r.rule_set_ip_cidr_match_source,
    _geosite: (r.geosite || []).join('\n'), _geoip: (r.geoip || []).join('\n'),
    _sourceGeoip: (r.source_geoip || []).join('\n'),
    action: r.action || '', outbound: r.outbound || '',
    resolveServer: r.server || r.domain_resolver || '',
    strategy: r.strategy || '',
    tlsFragment: !!r.tls_fragment, tlsRecordFragment: !!r.tls_record_fragment,
    tlsFragmentFallbackDelay: r.tls_fragment_fallback_delay || '',
    tlsSpoof: r.tls_spoof || '', tlsSpoofMethod: r.tls_spoof_method || '',
    _clashMode: r.clash_mode || '',
    _interfaceAddressJson: r.interface_address ? JSON.stringify(r.interface_address, null, 2) : '',
    _networkInterfaceAddressJson: r.network_interface_address ? JSON.stringify(r.network_interface_address, null, 2) : '',
    _defaultInterfaceAddress: (r.default_interface_address || []).join('\n'),
    rejectMethod: r.method || '', noDrop: !!r.no_drop,
    udpDisableDomainUnmapping: !!r.udp_disable_domain_unmapping, udpConnect: !!r.udp_connect,
    udpTimeout: r.udp_timeout || '', networkStrategy: r.network_strategy || '', fallbackDelay: r.fallback_delay || 0,
  })
  dlg.value = true
}

// tls_fragment 与 tls_record_fragment **互斥**（实测：核心报 `are mutually exclusive` 并 FATAL）
function onTlsFragmentChange() { if (rf.tlsFragment) rf.tlsRecordFragment = false }
function onTlsRecordFragmentChange() { if (rf.tlsRecordFragment) rf.tlsFragment = false }

// route-options 动作的字段：三个实测约束都在这里守住
//   - tls_fragment / tls_record_fragment 互斥
//   - tls_spoof_method 必须伴随 tls_spoof
//   - fallback delay 必须是合法 duration
// —— 以下 helper 的每条约束都用 sing-box check 实测过（详见 docs/singbox-field-gap.md）——

// duration 字段给非法值会让核心拒绝启动（如 `time: invalid duration "abc"`）→ 生成前校验
function validDuration(v) {
  const t = String(v || '').trim()
  if (!t) return ""
  return /^(\d+(\.\d+)?(ns|us|ms|s|m|h))+$/.test(t) ? t : ""
}

// 约束：default_network_strategy 必须伴随 auto_detect_interface（实测 FATAL）
function onNetStrategyChange() {
  if (form.default_network_strategy) form.auto_detect_interface = true
  emit()
}

// 约束：reject 的 method=drop 与 no_drop 互斥（实测 `no_drop is not available in current context`）
function onRejectMethodChange() {
  if (rf.rejectMethod === 'drop') rf.noDrop = false
  emit()
}

// interface_address 是 map（接口名 → 地址段），用 JSON 录入；解析失败就忽略，避免写坏 JSON
function parseJsonOrNull(v) {
  const t = String(v || '').trim()
  if (!t) return null
  try {
    const o = JSON.parse(t)
    return (o && typeof o === 'object' && !Array.isArray(o)) ? o : null
  } catch (e) {
    console.warn('interface_address JSON 解析失败，已忽略:', e)
    return null
  }
}

// 规则 matcher（表单 → 配置）
function applyExtraMatchers(rf, rule) {
  if (rf._clashMode) rule.clash_mode = rf._clashMode
  const ia = parseJsonOrNull(rf._interfaceAddressJson); if (ia) rule.interface_address = ia
  const nia = parseJsonOrNull(rf._networkInterfaceAddressJson); if (nia) rule.network_interface_address = nia
  const dia = (rf._defaultInterfaceAddress || '').split('\n').map(x => x.trim()).filter(Boolean)
  if (dia.length) rule.default_interface_address = dia
}

// 规则 matcher（已存规则对象 → 配置：字段是原始 JSON 名）
function applyExtraMatchersRaw(r, rule) {
  if (r.clash_mode) rule.clash_mode = r.clash_mode
  if (r.interface_address) rule.interface_address = r.interface_address
  if (r.network_interface_address) rule.network_interface_address = r.network_interface_address
  if (Array.isArray(r.default_interface_address) && r.default_interface_address.length) {
    rule.default_interface_address = [...r.default_interface_address]
  }
}

// reject 动作：method + no_drop（两种命名都兼容）
function applyRejectOptions(src, rule) {
  const method = src.rejectMethod || src.method || ""
  const noDrop = !!(src.noDrop || src.no_drop)
  if (method) rule.method = method
  if (noDrop && method !== "drop") rule.no_drop = true
}

// route-options 动作字段：三条实测约束都在这里守住
//   - tls_fragment / tls_record_fragment 互斥
//   - tls_spoof_method 必须伴随 tls_spoof
//   - fallback delay / udp_timeout 必须是合法 duration
// 入参可能是表单 rf（camelCase）也可能是**已存规则对象**（原始 JSON 名）→ 两种都读，
// 否则"由已存规则构造"那条路径上字段会静默丢失（上一轮踩过）。
function applyRouteOptions(src, rule) {
  const g = (a, b) => (src[a] !== undefined ? src[a] : src[b])
  const frag = !!g("tlsFragment", "tls_fragment")
  const rec = !!g("tlsRecordFragment", "tls_record_fragment")
  if (frag) rule.tls_fragment = true
  if (rec) rule.tls_record_fragment = true
  const ff = validDuration(g("tlsFragmentFallbackDelay", "tls_fragment_fallback_delay"))
  if (ff && (frag || rec)) rule.tls_fragment_fallback_delay = ff
  const spoof = g("tlsSpoof", "tls_spoof")
  if (spoof) {
    rule.tls_spoof = spoof
    const sm = g("tlsSpoofMethod", "tls_spoof_method")
    if (sm) rule.tls_spoof_method = sm
  }
  if (g("udpDisableDomainUnmapping", "udp_disable_domain_unmapping")) rule.udp_disable_domain_unmapping = true
  if (g("udpConnect", "udp_connect")) rule.udp_connect = true
  const ut = validDuration(g("udpTimeout", "udp_timeout"))
  if (ut) rule.udp_timeout = ut
  const ns = g("networkStrategy", "network_strategy")
  if (ns) rule.network_strategy = ns
  const fd = g("fallbackDelay", "fallback_delay")
  if (fd > 0) rule.fallback_delay = Number(fd)
}

function buildRuleFromForm() {
  const rule = {}
  if (rf.type) rule.type = rf.type

  // 逻辑规则：只允许 type/mode/rules/invert + action
  if (rf.type === 'logical') {
    if (rf.mode) rule.mode = rf.mode
    let sub = []
    if (rf._subRules && rf._subRules.trim()) {
      try {
        sub = JSON.parse(rf._subRules)
      } catch (e) {
        ElMessage.error('Sub Rules 不是合法 JSON: ' + e.message)
        return null
      }
      if (!Array.isArray(sub) || sub.length === 0) {
        ElMessage.error('逻辑规则的 Sub Rules 必须是非空 JSON 数组')
        return null
      }
    } else {
      ElMessage.error('逻辑规则必须填写 Sub Rules（sing-box 要求 rules 必填）')
      return null
    }
    rule.rules = sub
    if (rf.invert) rule.invert = true
    if (rf.action) rule.action = rf.action
    if (rf.outbound) rule.outbound = rf.outbound
    if (rf.action === 'resolve') {
      if (rf.resolveServer) rule.server = rf.resolveServer
      if (rf.strategy) rule.strategy = rf.strategy
    }
    return rule
  }

  const domains = lines(rf._domain); if (domains.length) rule.domain = domains
  const suffixes = lines(rf._domainSuffix); if (suffixes.length) rule.domain_suffix = suffixes
  const keywords = lines(rf._domainKeyword); if (keywords.length) rule.domain_keyword = keywords
  const regexes = lines(rf._domainRegex); if (regexes.length) rule.domain_regex = regexes
  const geosite = lines(rf._geosite); if (geosite.length) rule.geosite = geosite
  const geoip = lines(rf._geoip); if (geoip.length) rule.geoip = geoip
  const srcGeoip = lines(rf._sourceGeoip); if (srcGeoip.length) rule.source_geoip = srcGeoip
  const cidrs = lines(rf._ipCIDR); if (cidrs.length) rule.ip_cidr = cidrs
  const srcCidrs = lines(rf._sourceIPCIDR); if (srcCidrs.length) rule.source_ip_cidr = srcCidrs
  if (rf._ipVersion === 4 || rf._ipVersion === 6) rule.ip_version = rf._ipVersion
  if (rf._ipIsPrivate) rule.ip_is_private = true
  if (rf._sourceIPIsPrivate) rule.source_ip_is_private = true
  // sing-box: port / source_port 是 uint16 数字列表；端口范围必须走 port_range / source_port_range（"1000:2000"）
  if (rf._port) {
    const pr = portNumbers(rf._port)
    if (!pr.ok) { ElMessage.error(`Port 只支持数字端口（1-65535）："${pr.bad}" 非法；端口范围请填 Port Range`); return null }
    if (pr.nums.length) rule.port = pr.nums
  }
  if (rf._portRange) rule.port_range = commas(rf._portRange)
  if (rf._sourcePort) {
    const spr = portNumbers(rf._sourcePort)
    if (!spr.ok) { ElMessage.error(`Source Port 只支持数字端口（1-65535）："${spr.bad}" 非法；端口范围请填 Source Port Range`); return null }
    if (spr.nums.length) rule.source_port = spr.nums
  }
  if (rf._sourcePortRange) rule.source_port_range = commas(rf._sourcePortRange)
  if (rf._network) rule.network = rf._network
  if (rf._protocol?.length) rule.protocol = [...rf._protocol]
  if (rf._inboundTag?.length) rule.inbound = [...rf._inboundTag]
  const authUser = lines(rf._authUser); if (authUser.length) rule.auth_user = authUser
  const user = lines(rf._user); if (user.length) rule.user = user
  const clientList = commas(rf._client); if (clientList.length) rule.client = clientList
  const procs = lines(rf._process); if (procs.length) rule.process_name = procs
  const procPaths = lines(rf._processPath); if (procPaths.length) rule.process_path = procPaths
  const procPathRegex = lines(rf._processPathRegex); if (procPathRegex.length) rule.process_path_regex = procPathRegex
  if (rf._ruleSet?.length) {
    rule.rule_set = [...rf._ruleSet]
    if (rf._ruleSetIPCIDRMatchSource) rule.rule_set_ip_cidr_match_source = true
  }
  if (rf.invert) rule.invert = true
  if (rf.action) rule.action = rf.action
  if (rf.outbound) rule.outbound = rf.outbound
  if (rf.action === 'resolve') {
    if (rf.resolveServer) rule.server = rf.resolveServer
    if (rf.strategy) rule.strategy = rf.strategy
  }
  applyExtraMatchers(rf, rule)
  if (rf.action === 'reject') applyRejectOptions(rf, rule)
  if (rf.action === 'route-options') applyRouteOptions(rf, rule)
  return rule
}

function saveRule() {
  const rule = buildRuleFromForm()
  if (!rule) return // 校验失败
  // 保留 UI 状态字段（供列表展示；build() 会剔除）
  rule._domain = rf._domain
  rule._domainSuffix = rf._domainSuffix
  rule._domainKeyword = rf._domainKeyword
  rule._domainRegex = rf._domainRegex
  rule._ipCIDR = rf._ipCIDR
  rule._sourceIPCIDR = rf._sourceIPCIDR
  rule._protocol = [...(rf._protocol || [])]
  rule._inboundTag = [...(rf._inboundTag || [])]
  rule._authUser = rf._authUser
  rule._ruleSet = [...(rf._ruleSet || [])]

  if (editIdx.value >= 0) form.rules[editIdx.value] = rule
  else form.rules.push(rule)
  dlg.value = false
  emit()
}

function addRuleSet() {
  form.ruleSets.push({ type: 'remote', tag: '', format: '', url: '', path: '', update_interval: '1d',
    download_detour: '', http_client: '', initial_path: '', _rulesJson: '' })
  emit()
}

// ── 规则库联动：local rule_set 从「规则文件管理」已下载的文件中下拉选择 ──
// geo 规则文件是全局的（面板下载后推送所有节点 data/rules/），所以选项对所有节点一致；
// path 写相对工作目录的 `rules/<文件名>`（sing-box 进程工作目录 = 各自 data 目录）。
const geoRuleFiles = ref([])

async function loadGeoRuleFiles() {
  try {
    const res = await client.get('/geo-rules/files')
    geoRuleFiles.value = (res || []).filter(f => f.status === 'available' && f.file_name)
  } catch {}
}

function formatSize(bytes) {
  if (!bytes) return '?'
  if (bytes < 1048576) return (bytes / 1024).toFixed(0) + ' KB'
  return (bytes / 1048576).toFixed(1) + ' MB'
}

function onLocalPathChange(rs) {
  // 按扩展名自动补 format（仅当未显式指定时，不覆盖用户已选值）：
  // .srs = sing-box 二进制 rule-set（binary）；.json = source 格式
  if (!rs.format) {
    if (String(rs.path || '').endsWith('.srs')) rs.format = 'binary'
    else if (String(rs.path || '').endsWith('.json')) rs.format = 'source'
  }
  emit()
}

function rmRule(r) { const i = form.rules.indexOf(r); form.rules.splice(i, 1); emit() }

async function loadTags() {
  let obTags = []
  // 出站/端点 tag 首选**同一份配置文档**里的 outbounds / endpoints 段 ——
  // 协议中心的 Outbounds / Endpoints section 直接编辑的就是这两段，
  // 之前只查独立的 outbound-configs/endpoint-configs API（另一套存储），
  // 导致「Route Rules 的 Outbound 下拉看不到已创建的出站」。
  const docObs = props.fullConfig?.outbounds
  if (Array.isArray(docObs)) obTags = obTags.concat(docObs.map(o => o?.tag).filter(Boolean))
  const docEps = props.fullConfig?.endpoints
  if (Array.isArray(docEps)) obTags = obTags.concat(docEps.map(e => e?.tag).filter(Boolean))
  // API 结果作为补充（历史数据/独立存储），合并去重
  try {
    const res = await client.get(`/node-servers/${props.serverId}/outbound-configs`, { params: { core_type: props.coreType || 'singbox' } })
    const obs = (res || []).filter(o => o?.enabled !== false)
    obTags = obTags.concat(obs.map(o => o.tag).filter(Boolean))
  } catch {}
  try {
    const eps = await client.get(`/node-servers/${props.serverId}/endpoint-configs`, { params: { core_type: props.coreType || 'singbox' } })
    const list = Array.isArray(eps) ? eps : (eps?.items || [])
    obTags = obTags.concat(list.filter(e => e?.enabled !== false).map(e => e.tag).filter(Boolean))
  } catch {}
  outboundTags.value = [...new Set(['direct', 'block', ...obTags])]
  // 入站 tag 同理：先取配置文档的 inbounds 段，API 结果补充合并
  let ibTags = []
  const docIbs = props.fullConfig?.inbounds
  if (Array.isArray(docIbs)) ibTags = ibTags.concat(docIbs.map(i => i?.tag).filter(Boolean))
  try {
    const ibs = await client.get(`/node-servers/${props.serverId}/inbound-configs`, { params: { core_type: props.coreType || 'singbox' } })
    ibTags = ibTags.concat((ibs || []).filter(i => i?.enabled !== false).map(i => i.tag).filter(Boolean))
  } catch {}
  inboundTags.value = [...new Set(ibTags)]
}

  // rule_set 定义（本编辑器可编辑）
  form.ruleSets = (Array.isArray(props.data?.rule_set) ? props.data.rule_set : []).map(x => {
    const tags = Array.isArray(x.tag) ? x.tag : (x.tag ? [x.tag] : [])
    return {
      type: x.type || 'remote',
      tag: tags.join(', '),
      format: x.format || '',
      url: x.url || '', path: x.path || '',
      update_interval: x.update_interval || '',
      download_detour: x.download_detour || '', http_client: x.http_client || '',
      initial_path: x.initial_path || '',
      _rulesJson: Array.isArray(x.rules) ? JSON.stringify(x.rules, null, 1) : '',
    }
  })

function sync() {
  if (!props.data) return
  form.final = props.data.final === '' ? '' : (props.data.final || 'direct')
  form.auto_detect_interface = props.data.auto_detect_interface !== false
  form.default_domain_resolver = props.data.default_domain_resolver || ''
  form.default_domain_strategy = props.data.default_domain_strategy || ''
  form.default_network_strategy = props.data.default_network_strategy || ''
  form.default_network_type = props.data.default_network_type || []
  form.default_fallback_network_type = props.data.default_fallback_network_type || []
  form.default_fallback_delay = props.data.default_fallback_delay || ''
  form.default_http_client = props.data.default_http_client || ''
  form.rules = (props.data.rules || []).map(r => ({
    ...r,
    _domain: (r.domain || []).join('\n'),
    _domainSuffix: (r.domain_suffix || []).join('\n'),
    _domainKeyword: (r.domain_keyword || []).join('\n'),
    _domainRegex: (r.domain_regex || []).join('\n'),
    _ipCIDR: (r.ip_cidr || []).join('\n'),
    _sourceIPCIDR: (r.source_ip_cidr || []).join('\n'),
    _protocol: r.protocol || [],
    _inboundTag: r.inbound || r.inbound_tag || [],
    _authUser: (r.auth_user || []).join('\n'),
    _ruleSet: r.rule_set || [],
    _port: portStr(r.port),
    _portRange: (r.port_range || []).join(','),
    _sourcePort: portStr(r.source_port),
    _sourcePortRange: (r.source_port_range || []).join(','),
    _subRules: r.type === 'logical' && r.rules ? JSON.stringify(r.rules, null, 2) : '',
  }))
}

function build() {
  const out = {
    // final 留空 = 字段整个省略 → sing-box 未命中规则时回落出站列表第一个（与 xray 默认语义一致）
    final: form.final || undefined,
    auto_detect_interface: form.auto_detect_interface,
    default_network_strategy: form.default_network_strategy || undefined,
    default_network_type: form.default_network_type?.length ? [...form.default_network_type] : undefined,
    default_fallback_network_type: form.default_fallback_network_type?.length ? [...form.default_fallback_network_type] : undefined,
    default_fallback_delay: validDuration(form.default_fallback_delay) || undefined,
    default_http_client: form.default_http_client || undefined,
  }
  if (form.default_domain_resolver) out.default_domain_resolver = form.default_domain_resolver
  if (form.default_domain_strategy) out.default_domain_strategy = form.default_domain_strategy
  // 保留未在本编辑器管理的 route 字段（如 rule_set 定义、geo 资源等）
  for (const k of Object.keys(props.data || {})) {
    if (['final', 'auto_detect_interface', 'default_domain_resolver', 'default_domain_strategy', 'rules'].includes(k)) continue
    out[k] = props.data[k]
  }
  // rule_set 定义（放在"保留未托管字段"之后，确保以编辑器内容为准）
  const rss = (form.ruleSets || []).filter(x => (x.tag || '').trim())
  if (rss.length) {
    out.rule_set = rss.map(x => {
      const tags = String(x.tag).split(',').map(s => s.trim()).filter(Boolean)
      const rs = { type: x.type || 'remote', tag: tags.length > 1 ? tags : tags[0] }
      if (x.format) rs.format = x.format
      if (x.type === 'remote') {
        if (x.url) rs.url = x.url
        if (x.update_interval) rs.update_interval = x.update_interval
        if (x.download_detour) rs.download_detour = x.download_detour
        if (x.http_client) rs.http_client = x.http_client
        if (x.initial_path) rs.initial_path = x.initial_path
      } else if (x.type === 'local') {
        if (x.path) rs.path = x.path
      } else if (x._rulesJson) {
        try { rs.rules = JSON.parse(x._rulesJson) } catch (e) { console.warn('inline rules JSON 解析失败，已忽略:', e) }
      }
      return rs
    })
  }

  out.rules = form.rules.map(r => {
    const rule = {}
    if (r.type) rule.type = r.type
    if (r.type === 'logical') {
      if (r.mode) rule.mode = r.mode
      rule.rules = Array.isArray(r.rules) ? r.rules : []
      if (r.invert) rule.invert = true
    } else {
      const domains = lines(r._domain); if (domains.length) rule.domain = domains
      const suffixes = lines(r._domainSuffix); if (suffixes.length) rule.domain_suffix = suffixes
      const keywords = lines(r._domainKeyword); if (keywords.length) rule.domain_keyword = keywords
      const regexes = lines(r._domainRegex); if (regexes.length) rule.domain_regex = regexes
      const geosite = lines(r._geosite); if (geosite.length) rule.geosite = geosite
      const geoip = lines(r._geoip); if (geoip.length) rule.geoip = geoip
      const srcGeoip = lines(r._sourceGeoip); if (srcGeoip.length) rule.source_geoip = srcGeoip
      const cidrs = lines(r._ipCIDR); if (cidrs.length) rule.ip_cidr = cidrs
      const srcCidrs = lines(r._sourceIPCIDR); if (srcCidrs.length) rule.source_ip_cidr = srcCidrs
      if (r.ip_version === 4 || r.ip_version === 6) rule.ip_version = r.ip_version
      if (r.ip_is_private) rule.ip_is_private = true
      if (r.source_ip_is_private) rule.source_ip_is_private = true
      if (r.port?.length) rule.port = [...r.port]
      if (r.port_range?.length) rule.port_range = [...r.port_range]
      if (r.source_port?.length) rule.source_port = [...r.source_port]
      if (r.source_port_range?.length) rule.source_port_range = [...r.source_port_range]
      if (r.network) rule.network = r.network
      if (r.protocol?.length) rule.protocol = [...r.protocol]
      if (r.inbound?.length) rule.inbound = [...r.inbound]
      if (r.auth_user?.length) rule.auth_user = [...r.auth_user]
      if (r.user?.length) rule.user = [...r.user]
      if (r.client?.length) rule.client = [...r.client]
      if (r.process_name?.length) rule.process_name = [...r.process_name]
      if (r.process_path?.length) rule.process_path = [...r.process_path]
      if (r.process_path_regex?.length) rule.process_path_regex = [...r.process_path_regex]
      if (r.rule_set?.length) {
        rule.rule_set = [...r.rule_set]
        if (r.rule_set_ip_cidr_match_source) rule.rule_set_ip_cidr_match_source = true
      }
      if (r.invert) rule.invert = true
    }
    if (r.action) rule.action = r.action
    if (r.outbound) rule.outbound = r.outbound
    if (r.action === 'resolve') {
      if (r.server) rule.server = r.server
      if (r.strategy) rule.strategy = r.strategy
    }
    applyExtraMatchersRaw(r, rule)
    if (r.action === 'reject') applyRejectOptions(r, rule)
    if (r.action === 'route-options') applyRouteOptions(r, rule)
    return rule
  })
  return out
}

function emit() { emitFn('update', build()) }
onMounted(function() { sync(); loadTags(); loadGeoRuleFiles() })
</script>
<style scoped>
.rule-item { background: var(--el-fill-color-light); border-radius: 6px; padding: 12px; margin-bottom: 8px; }
.hint { font-size: 12px; color: var(--el-text-color-secondary); margin-top: 2px; }
.hint-inline { font-size: 12px; color: var(--el-text-color-secondary); margin-left: 8px; }
</style>
