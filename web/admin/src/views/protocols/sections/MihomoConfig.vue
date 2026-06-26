<template>
  <div>
    <el-tabs v-model="tab">
      <!-- Basic -->
      <el-tab-pane :label="t('subfmt.basic')" name="basic">
        <el-form label-width="140px" size="small">
          <el-divider content-position="left">{{ t('subfmt.ports') }}</el-divider>
          <el-form-item :label="t('subfmt.mixedPort')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="modelValue._mixedPortEnabled" @change="emit" />
              <el-input-number v-model="modelValue.mixed_port" :min="0" :max="65535" :disabled="!modelValue._mixedPortEnabled" @change="emit" />
            </div>
          </el-form-item>
          <el-form-item :label="t('subfmt.httpPort')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="modelValue._portEnabled" @change="emit" />
              <el-input-number v-model="modelValue.port" :min="0" :max="65535" :disabled="!modelValue._portEnabled" @change="emit" />
            </div>
          </el-form-item>
          <el-form-item :label="t('subfmt.socksPort')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="modelValue._socksPortEnabled" @change="emit" />
              <el-input-number v-model="modelValue.socks_port" :min="0" :max="65535" :disabled="!modelValue._socksPortEnabled" @change="emit" />
            </div>
          </el-form-item>
          <el-form-item :label="t('subfmt.redirPort')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="modelValue._redirPortEnabled" @change="emit" />
              <el-input-number v-model="modelValue.redir_port" :min="0" :max="65535" :disabled="!modelValue._redirPortEnabled" @change="emit" />
            </div>
          </el-form-item>
          <el-form-item :label="t('subfmt.tproxyPort')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="modelValue._tproxyPortEnabled" @change="emit" />
              <el-input-number v-model="modelValue.tproxy_port" :min="0" :max="65535" :disabled="!modelValue._tproxyPortEnabled" @change="emit" />
            </div>
          </el-form-item>
          <el-divider content-position="left">{{ t('subfmt.network') }}</el-divider>
          <el-form-item :label="t('subfmt.allowLan')"><el-switch v-model="modelValue.allow_lan" @change="emit" /></el-form-item>
          <el-form-item :label="t('subfmt.bindAddress')"><el-input v-model="modelValue.bind_address" placeholder="*" style="width:200px" @change="emit" /></el-form-item>
          <el-form-item :label="t('subfmt.ipv6')"><el-switch v-model="modelValue.ipv6" @change="emit" /></el-form-item>
          <el-divider content-position="left">{{ t('subfmt.auth') }}</el-divider>
          <el-form-item :label="t('subfmt.authentication')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="modelValue._authEnabled" @change="emit" />
              <el-input v-model="modelValue._authStr" type="textarea" :rows="2" placeholder="username:password" :disabled="!modelValue._authEnabled" style="flex:1" @change="emit" />
            </div>
          </el-form-item>
          <el-form-item :label="t('subfmt.skipAuthPrefixes')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="modelValue._skipAuthEnabled" @change="emit" />
              <el-input v-model="modelValue._skipAuthStr" type="textarea" :rows="2" placeholder="127.0.0.1/8&#10;::1/128" :disabled="!modelValue._skipAuthEnabled" style="flex:1" @change="emit" />
            </div>
          </el-form-item>
          <el-divider content-position="left">{{ t('subfmt.advanced') }}</el-divider>
          <el-form-item :label="t('subfmt.mode')">
            <el-select v-model="modelValue.mode" style="width:200px" @change="emit">
              <el-option label="rule" value="rule" /><el-option label="global" value="global" /><el-option label="direct" value="direct" />
            </el-select>
          </el-form-item>
          <el-form-item :label="t('subfmt.logLevel')">
            <el-select v-model="modelValue.log_level" style="width:200px" @change="emit">
              <el-option label="silent" value="silent" /><el-option label="error" value="error" /><el-option label="warning" value="warning" /><el-option label="info" value="info" /><el-option label="debug" value="debug" />
            </el-select>
          </el-form-item>
          <el-form-item :label="t('subfmt.findProcessMode')">
            <el-select v-model="modelValue.find_process_mode" style="width:200px" @change="emit">
              <el-option label="strict（默认）" value="strict" /><el-option label="always" value="always" /><el-option label="off" value="off" />
            </el-select>
          </el-form-item>
          <el-divider content-position="left">{{ t('subfmt.profile') }}</el-divider>
          <el-form-item :label="t('subfmt.storeSelected')">
            <el-switch v-model="modelValue._storeSelected" @change="emit" />
            <div class="hint">持久化 select 选择记录</div>
          </el-form-item>
          <el-form-item :label="t('subfmt.storeFakeIP')">
            <el-switch v-model="modelValue._storeFakeIP" @change="emit" />
            <div class="hint">持久化 fake-ip 映射</div>
          </el-form-item>
        </el-form>
      </el-tab-pane>

      <!-- Hosts -->
      <el-tab-pane :label="t('subfmt.hosts')" name="hosts">
        <div class="hint" style="margin-bottom:8px">{{ t('subfmt.hostsHint') }}</div>
        <div v-for="(h, i) in hosts" :key="i" style="display:flex;gap:8px;align-items:center;margin-bottom:6px">
          <el-input v-model="h.domain" size="small" placeholder="example.com" style="width:200px" @change="emitHosts" />
          <el-input v-model="h.ip" size="small" placeholder="1.1.1.1 或 lan 或别名" style="flex:1" @change="emitHosts" />
          <el-button size="small" type="danger" text @click="hosts.splice(i,1);emitHosts()">X</el-button>
        </div>
        <el-button size="small" @click="hosts.push({domain:'',ip:''});emitHosts()">+ {{ t('subfmt.addHost') }}</el-button>
      </el-tab-pane>

      <!-- TUN -->
      <el-tab-pane :label="t('subfmt.tun')" name="tun">
        <el-form label-width="180px" size="small">
          <el-form-item :label="t('subfmt.tunEnable')">
            <el-switch v-model="modelValue._tunEnabled" @change="emit" />
          </el-form-item>
          <el-form-item :label="t('subfmt.tunDevice')">
            <el-input v-model="modelValue._tunDevice" size="small" placeholder="utun" style="width:200px" :disabled="!modelValue._tunEnabled" @change="emit" />
            <span style="font-size:12px;color:#909399;margin-left:8px">留空自动分配</span>
          </el-form-item>
          <el-form-item :label="t('subfmt.tunStack')">
            <el-select v-model="modelValue._tunStack" style="width:200px" :disabled="!modelValue._tunEnabled" @change="emit">
              <el-option label="system" value="system" /><el-option label="gvisor" value="gvisor" /><el-option label="mixed" value="mixed" />
            </el-select>
          </el-form-item>
          <el-form-item :label="t('subfmt.tunDnsHijack')">
            <el-input v-model="modelValue._tunDnsHijack" type="textarea" :rows="2" placeholder="127.0.0.1:53" :disabled="!modelValue._tunEnabled" @change="emit" />
          </el-form-item>
          <el-form-item :label="t('subfmt.tunAutoRoute')">
            <el-switch v-model="modelValue._tunAutoRoute" :disabled="!modelValue._tunEnabled" @change="emit" />
          </el-form-item>
          <el-form-item :label="t('subfmt.tunAutoDetect')">
            <el-switch v-model="modelValue._tunAutoDetect" :disabled="!modelValue._tunEnabled" @change="emit" />
          </el-form-item>
          <el-form-item :label="t('subfmt.tunAutoRedirect')">
            <el-switch v-model="modelValue._tunAutoRedirect" :disabled="!modelValue._tunEnabled" @change="emit" />
          </el-form-item>
          <el-form-item :label="t('subfmt.tunEndpointNAT')">
            <el-switch v-model="modelValue._tunEndpointNAT" :disabled="!modelValue._tunEnabled" @change="emit" />
          </el-form-item>
          <el-form-item :label="t('subfmt.tunMTU')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="modelValue._tunMTUEnabled" :disabled="!modelValue._tunEnabled" @change="emit" />
              <el-input-number v-model="modelValue._tunMTU" :min="1280" :max="65535" :disabled="!modelValue._tunEnabled || !modelValue._tunMTUEnabled" @change="emit" />
            </div>
          </el-form-item>
          <el-form-item :label="t('subfmt.tunStrictRoute')">
            <el-switch v-model="modelValue._tunStrictRoute" :disabled="!modelValue._tunEnabled" @change="emit" />
          </el-form-item>
          <el-form-item :label="t('subfmt.tunGSO')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="modelValue._tunGSO" :disabled="!modelValue._tunEnabled" @change="emit" />
              <span style="font-size:12px;color:#909399">仅 Linux</span>
            </div>
          </el-form-item>
          <el-form-item :label="t('subfmt.tunGSOMaxSize')" v-if="modelValue._tunGSO">
            <el-input-number v-model="modelValue._tunGSOMaxSize" :min="0" :max="65536" :disabled="!modelValue._tunEnabled" @change="emit" />
          </el-form-item>
          <el-form-item :label="t('subfmt.tunIPv6Addr')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="modelValue._tunIPv6Enabled" :disabled="!modelValue._tunEnabled" @change="emit" />
              <el-input v-model="modelValue._tunIPv6Addr" size="small" placeholder="fdfe:dcba:9876::1/126" :disabled="!modelValue._tunEnabled || !modelValue._tunIPv6Enabled" style="width:250px" @change="emit" />
            </div>
          </el-form-item>
          <el-form-item :label="t('subfmt.tunUDPTimeout')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="modelValue._tunUDPTimeoutEnabled" :disabled="!modelValue._tunEnabled" @change="emit" />
              <el-input-number v-model="modelValue._tunUDPTimeout" :min="0" :disabled="!modelValue._tunEnabled || !modelValue._tunUDPTimeoutEnabled" @change="emit" />
              <span style="font-size:12px;color:#909399">秒</span>
            </div>
          </el-form-item>
          <el-form-item :label="t('subfmt.tunDisableICMP')">
            <el-switch v-model="modelValue._tunDisableICMP" :disabled="!modelValue._tunEnabled" @change="emit" />
          </el-form-item>
          <el-divider content-position="left">{{ t('subfmt.tunRoute') }}</el-divider>
          <el-form-item :label="t('subfmt.tunRouteAddrV4')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="modelValue._tunRouteAddrV4Enabled" :disabled="!modelValue._tunEnabled" @change="emit" />
              <el-input v-model="modelValue._tunRouteAddrV4" type="textarea" :rows="2" placeholder="0.0.0.0/1&#10;128.0.0.0/1" :disabled="!modelValue._tunEnabled || !modelValue._tunRouteAddrV4Enabled" style="flex:1" @change="emit" />
            </div>
          </el-form-item>
          <el-form-item :label="t('subfmt.tunRouteAddrV6')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="modelValue._tunRouteAddrV6Enabled" :disabled="!modelValue._tunEnabled" @change="emit" />
              <el-input v-model="modelValue._tunRouteAddrV6" type="textarea" :rows="2" placeholder="::/1&#10;8000::/1" :disabled="!modelValue._tunEnabled || !modelValue._tunRouteAddrV6Enabled" style="flex:1" @change="emit" />
            </div>
          </el-form-item>
          <el-form-item :label="t('subfmt.tunRouteExcludeV4')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="modelValue._tunRouteExcludeV4Enabled" :disabled="!modelValue._tunEnabled" @change="emit" />
              <el-input v-model="modelValue._tunRouteExcludeV4" type="textarea" :rows="2" placeholder="192.168.0.0/16" :disabled="!modelValue._tunEnabled || !modelValue._tunRouteExcludeV4Enabled" style="flex:1" @change="emit" />
            </div>
          </el-form-item>
          <el-form-item :label="t('subfmt.tunRouteExcludeV6')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="modelValue._tunRouteExcludeV6Enabled" :disabled="!modelValue._tunEnabled" @change="emit" />
              <el-input v-model="modelValue._tunRouteExcludeV6" type="textarea" :rows="2" placeholder="fc00::/7" :disabled="!modelValue._tunEnabled || !modelValue._tunRouteExcludeV6Enabled" style="flex:1" @change="emit" />
            </div>
          </el-form-item>
          <el-form-item :label="t('subfmt.tunRouteAddressSet')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="modelValue._tunRouteAddrSetEnabled" :disabled="!modelValue._tunEnabled" @change="emit" />
              <el-input v-model="modelValue._tunRouteAddrSet" type="textarea" :rows="2" placeholder="ruleset-1&#10;ruleset-2" :disabled="!modelValue._tunEnabled || !modelValue._tunRouteAddrSetEnabled" style="flex:1" @change="emit" />
            </div>
          </el-form-item>
          <el-form-item :label="t('subfmt.tunRouteExcludeSet')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="modelValue._tunRouteExcludeSetEnabled" :disabled="!modelValue._tunEnabled" @change="emit" />
              <el-input v-model="modelValue._tunRouteExcludeSet" type="textarea" :rows="2" placeholder="ruleset-3&#10;ruleset-4" :disabled="!modelValue._tunEnabled || !modelValue._tunRouteExcludeSetEnabled" style="flex:1" @change="emit" />
            </div>
          </el-form-item>
          <el-divider content-position="left">{{ t('subfmt.tunInterface') }}</el-divider>
          <el-form-item :label="t('subfmt.tunIncludeInterface')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="modelValue._tunIncludeIfEnabled" :disabled="!modelValue._tunEnabled" @change="emit" />
              <el-input v-model="modelValue._tunIncludeIf" type="textarea" :rows="2" placeholder="eth0" :disabled="!modelValue._tunEnabled || !modelValue._tunIncludeIfEnabled" style="flex:1" @change="emit" />
            </div>
          </el-form-item>
          <el-form-item :label="t('subfmt.tunExcludeInterface')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="modelValue._tunExcludeIfEnabled" :disabled="!modelValue._tunEnabled" @change="emit" />
              <el-input v-model="modelValue._tunExcludeIf" type="textarea" :rows="2" placeholder="eth1" :disabled="!modelValue._tunEnabled || !modelValue._tunExcludeIfEnabled" style="flex:1" @change="emit" />
            </div>
          </el-form-item>
          <el-divider content-position="left">{{ t('subfmt.tunUserFilter') }}</el-divider>
          <el-form-item :label="t('subfmt.tunIncludeUID')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="modelValue._tunIncludeUIDEnabled" :disabled="!modelValue._tunEnabled" @change="emit" />
              <el-input v-model="modelValue._tunIncludeUID" type="textarea" :rows="2" placeholder="0" :disabled="!modelValue._tunEnabled || !modelValue._tunIncludeUIDEnabled" style="flex:1" @change="emit" />
            </div>
          </el-form-item>
          <el-form-item :label="t('subfmt.tunExcludeUID')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="modelValue._tunExcludeUIDEnabled" :disabled="!modelValue._tunEnabled" @change="emit" />
              <el-input v-model="modelValue._tunExcludeUID" type="textarea" :rows="2" placeholder="1000" :disabled="!modelValue._tunEnabled || !modelValue._tunExcludeUIDEnabled" style="flex:1" @change="emit" />
            </div>
          </el-form-item>
          <el-form-item :label="t('subfmt.tunIncludeUIDRange')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="modelValue._tunIncludeUIDRangeEnabled" :disabled="!modelValue._tunEnabled" @change="emit" />
              <el-input v-model="modelValue._tunIncludeUIDRange" type="textarea" :rows="2" placeholder="1000:9999" :disabled="!modelValue._tunEnabled || !modelValue._tunIncludeUIDRangeEnabled" style="flex:1" @change="emit" />
            </div>
          </el-form-item>
          <el-form-item :label="t('subfmt.tunExcludeUIDRange')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="modelValue._tunExcludeUIDRangeEnabled" :disabled="!modelValue._tunEnabled" @change="emit" />
              <el-input v-model="modelValue._tunExcludeUIDRange" type="textarea" :rows="2" placeholder="1000:9999" :disabled="!modelValue._tunEnabled || !modelValue._tunExcludeUIDRangeEnabled" style="flex:1" @change="emit" />
            </div>
          </el-form-item>
          <el-divider content-position="left">{{ t('subfmt.tunMACFilter') }}</el-divider>
          <el-form-item :label="t('subfmt.tunIncludeMAC')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="modelValue._tunIncludeMACEnabled" :disabled="!modelValue._tunEnabled" @change="emit" />
              <el-input v-model="modelValue._tunIncludeMAC" type="textarea" :rows="2" placeholder="00:11:22:33:44:55" :disabled="!modelValue._tunEnabled || !modelValue._tunIncludeMACEnabled" style="flex:1" @change="emit" />
            </div>
          </el-form-item>
          <el-form-item :label="t('subfmt.tunExcludeMAC')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="modelValue._tunExcludeMACEnabled" :disabled="!modelValue._tunEnabled" @change="emit" />
              <el-input v-model="modelValue._tunExcludeMAC" type="textarea" :rows="2" placeholder="00:11:22:33:44:55" :disabled="!modelValue._tunEnabled || !modelValue._tunExcludeMACEnabled" style="flex:1" @change="emit" />
            </div>
          </el-form-item>
          <el-divider content-position="left">{{ t('subfmt.tunAndroid') }}</el-divider>
          <el-form-item :label="t('subfmt.tunIncludeAndroidUser')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="modelValue._tunIncludeAndroidUserEnabled" :disabled="!modelValue._tunEnabled" @change="emit" />
              <el-input v-model="modelValue._tunIncludeAndroidUser" type="textarea" :rows="2" placeholder="0&#10;10" :disabled="!modelValue._tunEnabled || !modelValue._tunIncludeAndroidUserEnabled" style="flex:1" @change="emit" />
            </div>
          </el-form-item>
          <el-form-item :label="t('subfmt.tunIncludePackage')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="modelValue._tunIncludePkgEnabled" :disabled="!modelValue._tunEnabled" @change="emit" />
              <el-input v-model="modelValue._tunIncludePkg" type="textarea" :rows="2" placeholder="com.android.chrome" :disabled="!modelValue._tunEnabled || !modelValue._tunIncludePkgEnabled" style="flex:1" @change="emit" />
            </div>
          </el-form-item>
          <el-form-item :label="t('subfmt.tunExcludePackage')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="modelValue._tunExcludePkgEnabled" :disabled="!modelValue._tunEnabled" @change="emit" />
              <el-input v-model="modelValue._tunExcludePkg" type="textarea" :rows="2" placeholder="com.android.captiveportallogin" :disabled="!modelValue._tunEnabled || !modelValue._tunExcludePkgEnabled" style="flex:1" @change="emit" />
            </div>
          </el-form-item>
        </el-form>
      </el-tab-pane>

      <!-- Proxy Groups -->
      <el-tab-pane :label="t('subfmt.proxyGroups')" name="proxyGroups">
        <el-divider content-position="left">{{ t('subfmt.specialOutbounds') }}</el-divider>
        <el-form label-width="140px" size="small">
          <el-form-item :label="t('subfmt.dnsOutbound')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="modelValue._dnsOutEnabled" @change="emit" />
              <span style="font-size:12px;color:#909399">dns-out: 劫持 DNS 请求到内部模块</span>
            </div>
          </el-form-item>
          <el-form-item :label="t('subfmt.rematchOutbound')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="modelValue._rematchOutEnabled" @change="emit" />
              <el-input v-model="modelValue._rematchTarget" size="small" placeholder="target-rematch-name" style="width:200px" :disabled="!modelValue._rematchOutEnabled" @change="emit" />
            </div>
          </el-form-item>
          <el-form-item :label="t('subfmt.directOutbound')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="modelValue._directOutEnabled" @change="emit" />
              <el-input v-model="modelValue._directInterface" size="small" placeholder="en1" style="width:100px" :disabled="!modelValue._directOutEnabled" @change="emit" />
              <el-input-number v-model="modelValue._directRoutingMark" :min="0" :max="65535" size="small" placeholder="6667" style="width:120px" :disabled="!modelValue._directOutEnabled" @change="emit" />
            </div>
          </el-form-item>
        </el-form>
        <el-divider content-position="left">{{ t('subfmt.proxyGroups') }}</el-divider>
        <div v-for="(g, i) in proxyGroups" :key="i" style="border:1px solid #e4e7ed;border-radius:6px;padding:12px;margin-bottom:8px">
          <div style="display:flex;gap:8px;align-items:center;margin-bottom:8px;flex-wrap:wrap">
            <el-input v-model="g.name" size="small" placeholder="组名" style="width:120px" @change="emitProxyGroups" />
            <el-select v-model="g.type" size="small" style="width:120px" @change="emitProxyGroups">
              <el-option label="select" value="select" /><el-option label="url-test" value="url-test" /><el-option label="fallback" value="fallback" /><el-option label="load-balance" value="load-balance" /><el-option label="relay" value="relay" />
            </el-select>
            <el-switch v-model="g.hidden" size="small" @change="emitProxyGroups" />
            <span style="font-size:12px;color:#909399">隐藏</span>
            <el-switch v-model="g._hasExternalNodes" size="small" @change="emitProxyGroups" />
            <span style="font-size:12px;color:#909399">订阅添加节点</span>
            <el-button size="small" type="danger" text @click="proxyGroups.splice(i,1);emitProxyGroups()">X</el-button>
          </div>
          <div style="font-size:12px;color:#909399;margin-bottom:4px">代理列表（每行一个，订阅会根据类型自动添加节点）</div>
          <el-input v-model="g._proxiesStr" type="textarea" :rows="2" size="small" placeholder="DIRECT&#10;PROXY" @change="emitProxyGroups" />
          <!-- Optional fields toggle -->
          <div style="margin-top:8px">
            <el-button size="small" text @click="g._showAdvanced = !g._showAdvanced; emitProxyGroups()">
              {{ g._showAdvanced ? t('subfmt.hideAdvanced') : t('subfmt.showAdvanced') }}
            </el-button>
          </div>
          <template v-if="g._showAdvanced">
            <el-divider style="margin:8px 0" />
            <el-form label-width="120px" size="small">
              <el-form-item :label="t('subfmt.groupURL')">
                <el-input v-model="g.url" size="small" placeholder="https://www.gstatic.com/generate_204" @change="emitProxyGroups" />
              </el-form-item>
              <el-form-item :label="t('subfmt.groupInterval')">
                <el-input-number v-model="g.interval" :min="0" size="small" @change="emitProxyGroups" />
                <span style="font-size:12px;color:#909399;margin-left:8px">秒</span>
              </el-form-item>
              <el-form-item :label="t('subfmt.groupLazy')">
                <el-switch v-model="g.lazy" @change="emitProxyGroups" />
              </el-form-item>
              <el-form-item :label="t('subfmt.groupTimeout')">
                <el-input-number v-model="g.timeout" :min="0" size="small" @change="emitProxyGroups" />
                <span style="font-size:12px;color:#909399;margin-left:8px">ms</span>
              </el-form-item>
              <el-form-item :label="t('subfmt.groupMaxFailed')">
                <el-input-number v-model="g['max-failed-times']" :min="0" size="small" @change="emitProxyGroups" />
              </el-form-item>
              <el-form-item :label="t('subfmt.groupEmptyFallback')">
                <el-select v-model="g['empty-fallback']" size="small" style="width:150px" clearable @change="emitProxyGroups">
                  <el-option label="COMPATIBLE" value="COMPATIBLE" /><el-option label="PASS" value="PASS" /><el-option label="REJECT" value="REJECT" />
                </el-select>
              </el-form-item>
              <el-form-item :label="t('subfmt.groupDisableUDP')">
                <el-switch v-model="g['disable-udp']" @change="emitProxyGroups" />
              </el-form-item>
              <el-form-item :label="t('subfmt.groupFilter')">
                <el-input v-model="g.filter" size="small" placeholder='(?i)港|hk' @change="emitProxyGroups" />
              </el-form-item>
              <el-form-item :label="t('subfmt.groupExcludeFilter')">
                <el-input v-model="g['exclude-filter']" size="small" placeholder='美|日' @change="emitProxyGroups" />
              </el-form-item>
              <el-form-item :label="t('subfmt.groupExcludeType')">
                <el-input v-model="g['exclude-type']" size="small" placeholder="Shadowsocks|Http" @change="emitProxyGroups" />
              </el-form-item>
              <el-form-item :label="t('subfmt.groupIncludeAll')">
                <el-switch v-model="g['include-all']" @change="emitProxyGroups" />
              </el-form-item>
              <el-form-item :label="t('subfmt.groupInterface')">
                <el-input v-model="g['interface-name']" size="small" placeholder="en0" @change="emitProxyGroups" />
              </el-form-item>
              <el-form-item :label="t('subfmt.groupRoutingMark')">
                <el-input-number v-model="g['routing-mark']" :min="0" size="small" @change="emitProxyGroups" />
              </el-form-item>
            </el-form>
          </template>
        </div>
        <el-button size="small" @click="proxyGroups.push({name:'PROXY-'+(proxyGroups.length+1),type:'select',_proxiesStr:'DIRECT',_showAdvanced:false,hidden:false,_hasExternalNodes:false,url:'https://www.gstatic.com/generate_204',interval:300,lazy:true,timeout:5000,'max-failed-times':5,'empty-fallback':''});emitProxyGroups()">+ {{ t('subfmt.addGroup') }}</el-button>
      </el-tab-pane>

      <!-- Rule Providers -->
      <el-tab-pane :label="t('subfmt.ruleProviders')" name="ruleProviders">
        <div v-for="(rp, i) in ruleProvidersList" :key="i" style="border:1px solid #e4e7ed;border-radius:6px;padding:12px;margin-bottom:8px">
          <div style="display:flex;gap:8px;align-items:center;margin-bottom:8px">
            <el-input v-model="rp.name" size="small" placeholder="规则集名称" style="width:150px" @change="syncRuleProviders" />
            <el-select v-model="rp.type" size="small" style="width:100px" @change="syncRuleProviders">
              <el-option label="http" value="http" /><el-option label="file" value="file" /><el-option label="inline" value="inline" />
            </el-select>
            <el-select v-model="rp.behavior" size="small" style="width:100px" @change="syncRuleProviders">
              <el-option label="domain" value="domain" /><el-option label="ipcidr" value="ipcidr" /><el-option label="classical" value="classical" />
            </el-select>
            <el-select v-model="rp.format" size="small" style="width:100px" @change="syncRuleProviders">
              <el-option label="yaml" value="yaml" /><el-option label="text" value="text" /><el-option label="mrs" value="mrs" />
            </el-select>
            <el-button size="small" type="danger" text @click="ruleProvidersList.splice(i,1);syncRuleProviders()">X</el-button>
          </div>
          <el-form label-width="100px" size="small">
            <el-form-item :label="t('subfmt.rpURL')">
              <el-input v-model="rp.url" size="small" placeholder="https://..." @change="syncRuleProviders" />
            </el-form-item>
            <el-form-item :label="t('subfmt.rpPath')">
              <el-input v-model="rp.path" size="small" placeholder="./rule1.yaml" @change="syncRuleProviders" />
            </el-form-item>
            <el-form-item :label="t('subfmt.rpProxy')">
              <el-input v-model="rp.proxy" size="small" placeholder="DIRECT" @change="syncRuleProviders" />
            </el-form-item>
            <el-form-item :label="t('subfmt.rpPathInBundle')">
              <el-input v-model="rp.pathInBundle" size="small" placeholder="geo/geosite/cn.mrs" @change="syncRuleProviders" />
            </el-form-item>
            <el-form-item :label="t('subfmt.rpInterval')">
              <el-input-number v-model="rp.interval" :min="0" size="small" @change="syncRuleProviders" />
              <span style="font-size:12px;color:#909399;margin-left:8px">秒</span>
            </el-form-item>
          </el-form>
        </div>
        <el-button size="small" @click="ruleProvidersList.push({name:'',type:'http',behavior:'domain',format:'yaml',url:'',path:'',proxy:'DIRECT',pathInBundle:'',interval:86400});syncRuleProviders()">+ {{ t('subfmt.addRuleProvider') }}</el-button>
      </el-tab-pane>

      <!-- DNS -->
      <el-tab-pane :label="t('subfmt.dns')" name="dns">
        <el-form label-width="160px" size="small">
          <el-form-item :label="t('subfmt.dnsEnable')"><el-switch v-model="dns.enable" @change="emitDns" /></el-form-item>
          <el-form-item :label="t('subfmt.dnsListen')">
            <el-input v-model="dns._listen" size="small" placeholder="0.0.0.0:53" style="width:200px" @change="emitDns" />
          </el-form-item>
          <el-form-item :label="t('subfmt.dnsEnhancedMode')">
            <el-select v-model="dns._enhancedMode" style="width:200px" @change="emitDns">
              <el-option label="fake-ip" value="fake-ip" /><el-option label="redir-host" value="redir-host" />
            </el-select>
          </el-form-item>
          <el-form-item :label="t('subfmt.dnsCacheAlgorithm')">
            <el-select v-model="dns._cacheAlgorithm" style="width:200px" @change="emitDns">
              <el-option label="arc" value="arc" /><el-option label="lru" value="lru" />
            </el-select>
          </el-form-item>
          <el-form-item :label="t('subfmt.dnsPreferH3')"><el-switch v-model="dns._preferH3" @change="emitDns" /></el-form-item>
          <el-form-item :label="t('subfmt.dnsIPv6')"><el-switch v-model="dns._ipv6" @change="emitDns" /></el-form-item>
          <el-form-item :label="t('subfmt.dnsUseHosts')"><el-switch v-model="dns._useHosts" @change="emitDns" /></el-form-item>
          <el-form-item :label="t('subfmt.dnsRespectRules')"><el-switch v-model="dns._respectRules" @change="emitDns" /></el-form-item>
          <el-divider content-position="left">{{ t('subfmt.dnsServers') }}</el-divider>
          <el-form-item :label="t('subfmt.dnsDefaultNS')">
            <el-input v-model="dns._defaultNameserverStr" type="textarea" :rows="2" placeholder="114.114.114.114&#10;8.8.8.8" @change="emitDns" />
          </el-form-item>
          <el-form-item :label="t('subfmt.dnsNameserver')">
            <el-input v-model="dns._nameserverStr" type="textarea" :rows="3" placeholder="https://doh.pub/dns-query&#10;https://dns.alidns.com/dns-query" @change="emitDns" />
          </el-form-item>
          <el-form-item :label="t('subfmt.dnsFallback')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="dns._fallbackEnabled" @change="emitDns" />
              <el-input v-model="dns._fallbackStr" type="textarea" :rows="2" :disabled="!dns._fallbackEnabled" style="flex:1" @change="emitDns" />
            </div>
          </el-form-item>
          <el-form-item :label="t('subfmt.dnsProxyServerNS')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="dns._proxyServerNSEnabled" @change="emitDns" />
              <el-input v-model="dns._proxyServerNSStr" type="textarea" :rows="2" :disabled="!dns._proxyServerNSEnabled" style="flex:1" @change="emitDns" />
            </div>
          </el-form-item>
          <el-form-item :label="t('subfmt.dnsDirectNS')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="dns._directNSEnabled" @change="emitDns" />
              <el-input v-model="dns._directNSStr" type="textarea" :rows="2" :disabled="!dns._directNSEnabled" style="flex:1" @change="emitDns" />
            </div>
          </el-form-item>
          <el-divider content-position="left">{{ t('subfmt.dnsFakeIP') }}</el-divider>
          <el-form-item :label="t('subfmt.dnsFakeIPRange')">
            <el-input v-model="dns._fakeIPRange" size="small" placeholder="198.18.0.1/16" style="width:200px" @change="emitDns" />
          </el-form-item>
          <el-form-item :label="t('subfmt.dnsFakeIPFilterMode')">
            <el-select v-model="dns._fakeIPFilterMode" style="width:200px" @change="emitDns">
              <el-option label="blacklist" value="blacklist" /><el-option label="whitelist" value="whitelist" /><el-option label="rule" value="rule" />
            </el-select>
          </el-form-item>
          <el-form-item :label="t('subfmt.dnsFakeIPFilter')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="dns._fakeIPFilterEnabled" @change="emitDns" />
              <el-input v-model="dns._fakeIPFilterStr" type="textarea" :rows="3" placeholder="*.lan&#10;localhost.ptlogin2.qq.com" :disabled="!dns._fakeIPFilterEnabled" style="flex:1" @change="emitDns" />
            </div>
          </el-form-item>
          <el-divider content-position="left">{{ t('subfmt.dnsFallbackFilter') }}</el-divider>
          <el-form-item :label="t('subfmt.dnsFallbackGeoIP')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="dns._fallbackGeoIP" @change="emitDns" />
              <span style="font-size:12px;color:#909399">geoip: true</span>
            </div>
          </el-form-item>
          <el-form-item :label="t('subfmt.dnsFallbackGeoCode')">
            <el-input v-model="dns._fallbackGeoCode" size="small" placeholder="CN" style="width:100px" :disabled="!dns._fallbackGeoIP" @change="emitDns" />
          </el-form-item>
          <el-form-item :label="t('subfmt.dnsFallbackIPCIDR')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="dns._fallbackIPCIDREnabled" @change="emitDns" />
              <el-input v-model="dns._fallbackIPCIDRStr" type="textarea" :rows="2" placeholder="240.0.0.0/4&#10;127.0.0.1/32" :disabled="!dns._fallbackIPCIDREnabled" style="flex:1" @change="emitDns" />
            </div>
          </el-form-item>
          <el-form-item :label="t('subfmt.dnsFallbackDomain')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="dns._fallbackDomainEnabled" @change="emitDns" />
              <el-input v-model="dns._fallbackDomainStr" type="textarea" :rows="2" placeholder="+.google.com&#10;+.facebook.com" :disabled="!dns._fallbackDomainEnabled" style="flex:1" @change="emitDns" />
            </div>
          </el-form-item>
          <el-divider content-position="left">{{ t('subfmt.dnsNameserverPolicy') }}</el-divider>
          <el-form-item :label="t('subfmt.dnsNameserverPolicy')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="dns._nsPolicyEnabled" @change="emitDns" />
              <el-input v-model="dns._nsPolicyStr" type="textarea" :rows="3" placeholder="geosite:cn,private: https://doh.pub/dns-query&#10;geosite:category-ads-all: rcode://success" :disabled="!dns._nsPolicyEnabled" style="flex:1" @change="emitDns" />
            </div>
          </el-form-item>
        </el-form>
      </el-tab-pane>

      <!-- Sniffer -->
      <el-tab-pane :label="t('subfmt.sniffer')" name="sniffer">
        <el-form label-width="180px" size="small">
          <el-form-item :label="t('subfmt.snifferEnable')">
            <el-switch v-model="modelValue._snifferEnabled" @change="emit" />
          </el-form-item>
          <el-form-item :label="t('subfmt.snifferForceDNS')">
            <el-switch v-model="modelValue._snifferForceDNS" :disabled="!modelValue._snifferEnabled" @change="emit" />
          </el-form-item>
          <el-form-item :label="t('subfmt.snifferParsePureIP')">
            <el-switch v-model="modelValue._snifferParsePureIP" :disabled="!modelValue._snifferEnabled" @change="emit" />
          </el-form-item>
          <el-form-item :label="t('subfmt.snifferOverrideDest')">
            <el-switch v-model="modelValue._snifferOverrideDest" :disabled="!modelValue._snifferEnabled" @change="emit" />
          </el-form-item>
          <el-divider content-position="left">{{ t('subfmt.snifferSniff') }}</el-divider>
          <el-form-item label="HTTP">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="modelValue._snifferHTTP" :disabled="!modelValue._snifferEnabled" @change="emit" />
              <el-input v-model="modelValue._snifferHTTPPorts" size="small" placeholder="80, 8080-8880" :disabled="!modelValue._snifferEnabled || !modelValue._snifferHTTP" style="width:200px" @change="emit" />
              <el-switch v-model="modelValue._snifferHTTPOverride" :disabled="!modelValue._snifferEnabled || !modelValue._snifferHTTP" @change="emit" />
              <span style="font-size:12px;color:#909399">覆盖目标</span>
            </div>
          </el-form-item>
          <el-form-item label="TLS">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="modelValue._snifferTLS" :disabled="!modelValue._snifferEnabled" @change="emit" />
              <el-input v-model="modelValue._snifferTLSPorts" size="small" placeholder="443, 8443" :disabled="!modelValue._snifferEnabled || !modelValue._snifferTLS" style="width:200px" @change="emit" />
            </div>
          </el-form-item>
          <el-form-item label="QUIC">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="modelValue._snifferQUIC" :disabled="!modelValue._snifferEnabled" @change="emit" />
              <el-input v-model="modelValue._snifferQUICPorts" size="small" placeholder="443, 8443" :disabled="!modelValue._snifferEnabled || !modelValue._snifferQUIC" style="width:200px" @change="emit" />
            </div>
          </el-form-item>
          <el-divider content-position="left">{{ t('subfmt.snifferFilter') }}</el-divider>
          <el-form-item :label="t('subfmt.snifferForceDomain')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="modelValue._snifferForceDomainEnabled" :disabled="!modelValue._snifferEnabled" @change="emit" />
              <el-input v-model="modelValue._snifferForceDomain" type="textarea" :rows="2" placeholder="+.v2ex.com" :disabled="!modelValue._snifferEnabled || !modelValue._snifferForceDomainEnabled" style="flex:1" @change="emit" />
            </div>
          </el-form-item>
          <el-form-item :label="t('subfmt.snifferSkipDomain')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="modelValue._snifferSkipDomainEnabled" :disabled="!modelValue._snifferEnabled" @change="emit" />
              <el-input v-model="modelValue._snifferSkipDomain" type="textarea" :rows="2" placeholder="Mijia Cloud" :disabled="!modelValue._snifferEnabled || !modelValue._snifferSkipDomainEnabled" style="flex:1" @change="emit" />
            </div>
          </el-form-item>
          <el-form-item :label="t('subfmt.snifferSkipSrcAddr')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="modelValue._snifferSkipSrcEnabled" :disabled="!modelValue._snifferEnabled" @change="emit" />
              <el-input v-model="modelValue._snifferSkipSrcAddr" type="textarea" :rows="2" placeholder="192.168.0.3/32" :disabled="!modelValue._snifferEnabled || !modelValue._snifferSkipSrcEnabled" style="flex:1" @change="emit" />
            </div>
          </el-form-item>
          <el-form-item :label="t('subfmt.snifferSkipDstAddr')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="modelValue._snifferSkipDstEnabled" :disabled="!modelValue._snifferEnabled" @change="emit" />
              <el-input v-model="modelValue._snifferSkipDstAddr" type="textarea" :rows="2" placeholder="192.168.0.3/32" :disabled="!modelValue._snifferEnabled || !modelValue._snifferSkipDstEnabled" style="flex:1" @change="emit" />
            </div>
          </el-form-item>
        </el-form>
      </el-tab-pane>

      <!-- GeoData -->
      <el-tab-pane :label="t('subfmt.geodata')" name="geodata">
        <el-form label-width="140px" size="small">
          <el-form-item :label="t('subfmt.geoAutoUpdate')"><el-switch v-model="modelValue._geoAutoUpdate" @change="emit" /></el-form-item>
          <el-form-item :label="t('subfmt.geoUpdateInterval')">
            <el-input-number v-model="modelValue._geoUpdateInterval" :min="1" :max="168" :disabled="!modelValue._geoAutoUpdate" @change="emit" />
            <span style="font-size:12px;color:#909399;margin-left:8px">小时</span>
          </el-form-item>
          <el-form-item :label="t('subfmt.geoIPUrl')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="modelValue._geoIPEnabled" @change="emit" />
              <el-input v-model="modelValue._geoIPUrl" size="small" placeholder="https://.../geoip.dat" :disabled="!modelValue._geoIPEnabled" style="flex:1" @change="emit" />
            </div>
          </el-form-item>
          <el-form-item :label="t('subfmt.geositeUrl')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="modelValue._geositeEnabled" @change="emit" />
              <el-input v-model="modelValue._geositeUrl" size="small" placeholder="https://.../geosite.dat" :disabled="!modelValue._geositeEnabled" style="flex:1" @change="emit" />
            </div>
          </el-form-item>
          <el-form-item :label="t('subfmt.mmdbUrl')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="modelValue._mmdbEnabled" @change="emit" />
              <el-input v-model="modelValue._mmdbUrl" size="small" placeholder="https://.../geoip.metadb" :disabled="!modelValue._mmdbEnabled" style="flex:1" @change="emit" />
            </div>
          </el-form-item>
        </el-form>
      </el-tab-pane>

      <!-- External Controller -->
      <el-tab-pane :label="t('subfmt.externalController')" name="extController">
        <el-form label-width="140px" size="small">
          <el-form-item :label="t('subfmt.externalController')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="modelValue._extControllerEnabled" @change="emit" />
              <el-input v-model="modelValue._extController" size="small" placeholder="0.0.0.0:9093" :disabled="!modelValue._extControllerEnabled" style="width:200px" @change="emit" />
            </div>
          </el-form-item>
          <el-form-item :label="t('subfmt.extControllerTLS')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="modelValue._extControllerTLSEnabled" :disabled="!modelValue._extControllerEnabled" @change="emit" />
              <el-input v-model="modelValue._extControllerTLS" size="small" placeholder="0.0.0.0:8443" :disabled="!modelValue._extControllerEnabled || !modelValue._extControllerTLSEnabled" style="width:200px" @change="emit" />
            </div>
          </el-form-item>
          <el-form-item :label="t('subfmt.extControllerSecret')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="modelValue._extSecretEnabled" @change="emit" />
              <el-input v-model="modelValue._extSecret" size="small" placeholder="secret" :disabled="!modelValue._extSecretEnabled" style="width:200px" @change="emit" />
            </div>
          </el-form-item>
          <el-form-item :label="t('subfmt.externalUIName')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="modelValue._extUIEnabled" @change="emit" />
              <el-input v-model="modelValue._extUIName" size="small" placeholder="metacubexd" :disabled="!modelValue._extUIEnabled" style="width:200px" @change="emit" />
            </div>
          </el-form-item>
          <el-form-item :label="t('subfmt.externalUIUrl')">
            <el-input v-model="modelValue._extUIUrl" size="small" placeholder="https://github.com/..." :disabled="!modelValue._extUIEnabled" @change="emit" />
          </el-form-item>
        </el-form>
      </el-tab-pane>

      <!-- TLS -->
      <el-tab-pane :label="t('subfmt.tls')" name="tls">
        <el-form label-width="140px" size="small">
          <el-form-item :label="t('subfmt.tlsCertificate')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="modelValue._tlsEnabled" @change="emit" />
              <el-input v-model="modelValue._tlsCert" type="textarea" :rows="3" placeholder="PEM 证书或路径" :disabled="!modelValue._tlsEnabled" style="flex:1" @change="emit" />
            </div>
          </el-form-item>
          <el-form-item :label="t('subfmt.tlsKey')">
            <el-input v-model="modelValue._tlsKey" type="textarea" :rows="3" placeholder="PEM 私钥或路径" :disabled="!modelValue._tlsEnabled" @change="emit" />
          </el-form-item>
        </el-form>
      </el-tab-pane>

      <!-- Tunnels -->
      <el-tab-pane :label="t('subfmt.tunnels')" name="tunnels">
        <div class="hint" style="margin-bottom:8px">{{ t('subfmt.tunnelsHint') }}</div>
        <div v-for="(tun, i) in tunnels" :key="i" style="border:1px solid #e4e7ed;border-radius:6px;padding:12px;margin-bottom:8px">
          <div style="display:flex;gap:8px;align-items:center;flex-wrap:wrap;margin-bottom:8px">
            <el-select v-model="tun._network" size="small" multiple style="width:160px" @change="emitTunnels">
              <el-option label="tcp" value="tcp" /><el-option label="udp" value="udp" />
            </el-select>
            <el-input v-model="tun.address" size="small" placeholder="127.0.0.1:6553" style="width:180px" @change="emitTunnels" />
            <el-input v-model="tun.target" size="small" placeholder="target.com:53" style="width:180px" @change="emitTunnels" />
            <el-input v-model="tun.proxy" size="small" placeholder="proxy" style="width:100px" @change="emitTunnels" />
            <el-button size="small" type="danger" text @click="tunnels.splice(i,1);emitTunnels()">X</el-button>
          </div>
        </div>
        <el-button size="small" @click="tunnels.push({network:['tcp','udp'],address:'',target:'',proxy:''});emitTunnels()">+ {{ t('subfmt.addTunnel') }}</el-button>
      </el-tab-pane>

      <!-- Network Advanced -->
      <el-tab-pane :label="t('subfmt.networkAdvanced')" name="networkAdv">
        <el-form label-width="140px" size="small">
          <el-form-item :label="t('subfmt.interfaceName')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="modelValue._interfaceEnabled" @change="emit" />
              <el-input v-model="modelValue._interfaceName" size="small" placeholder="en0" :disabled="!modelValue._interfaceEnabled" style="width:200px" @change="emit" />
            </div>
          </el-form-item>
          <el-form-item :label="t('subfmt.routingMark')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="modelValue._routingMarkEnabled" @change="emit" />
              <el-input-number v-model="modelValue._routingMark" :min="0" :max="65535" :disabled="!modelValue._routingMarkEnabled" @change="emit" />
            </div>
          </el-form-item>
          <el-form-item :label="t('subfmt.tcpConcurrent')"><el-switch v-model="modelValue._tcpConcurrent" @change="emit" /></el-form-item>
          <el-form-item :label="t('subfmt.keepAliveIdle')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="modelValue._keepAliveEnabled" @change="emit" />
              <el-input-number v-model="modelValue._keepAliveIdle" :min="0" :disabled="!modelValue._keepAliveEnabled" @change="emit" />
              <span style="font-size:12px;color:#909399">秒</span>
            </div>
          </el-form-item>
          <el-form-item :label="t('subfmt.lanControl')">
            <el-input v-model="modelValue._lanAllowedStr" type="textarea" :rows="2" placeholder="0.0.0.0/0&#10;::/0" @change="emit" />
          </el-form-item>
          <el-form-item :label="t('subfmt.lanDisallowedIPs')">
            <div style="display:flex;gap:8px;align-items:center">
              <el-switch v-model="modelValue._lanDisallowedEnabled" @change="emit" />
              <el-input v-model="modelValue._lanDisallowedStr" type="textarea" :rows="2" :disabled="!modelValue._lanDisallowedEnabled" style="flex:1" @change="emit" />
            </div>
          </el-form-item>
        </el-form>
      </el-tab-pane>

      <!-- Experimental -->
      <el-tab-pane :label="t('subfmt.experimental')" name="experimental">
        <el-form label-width="180px" size="small">
          <el-form-item :label="t('subfmt.quicDisableGso')">
            <el-switch v-model="modelValue._quicDisableGso" @change="emit" />
          </el-form-item>
          <el-form-item :label="t('subfmt.quicDisableEcn')">
            <el-switch v-model="modelValue._quicDisableEcn" @change="emit" />
          </el-form-item>
          <el-form-item :label="t('subfmt.dialerIP4PConvert')">
            <el-switch v-model="modelValue._dialerIP4PConvert" @change="emit" />
          </el-form-item>
        </el-form>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const props = defineProps({
  modelValue: { type: Object, default: () => ({}) },
  hosts: { type: Array, default: () => [] },
  proxyGroups: { type: Array, default: () => [] },
  ruleProviders: { type: Object, default: () => ({}) },
  proxyProviders: { type: Object, default: () => ({}) },
  dns: { type: Object, default: () => ({ enable: true, _nameserverStr: '', _fallbackStr: '', _fallbackEnabled: false }) },
  tunnels: { type: Array, default: () => [] }
})

const emit = defineEmits(['update:modelValue', 'update:hosts', 'update:proxyGroups', 'update:ruleProviders', 'update:proxyProviders', 'update:dns', 'update:tunnels'])

const tab = ref('basic')
const ruleProvidersList = ref(Object.entries(props.ruleProviders || {}).map(([name, rp]) => ({
  name,
  type: rp.type || 'http',
  behavior: rp.behavior || 'domain',
  format: rp.format || 'yaml',
  url: rp.url || '',
  path: rp.path || '',
  proxy: rp.proxy || 'DIRECT',
  pathInBundle: rp['path-in-bundle'] || '',
  interval: rp.interval || 86400
})))

// Sync ruleProvidersList to ruleProviders object
function syncRuleProviders() {
  const rp = {}
  ruleProvidersList.value.forEach(item => {
    if (item.name) {
      rp[item.name] = {
        type: item.type,
        behavior: item.behavior,
        format: item.format,
        url: item.url,
        path: item.path,
        proxy: item.proxy,
        'path-in-bundle': item.pathInBundle || undefined,
        interval: item.interval
      }
    }
  })
  emit('update:ruleProviders', rp)
}

function emitHosts() { emit('update:hosts', [...props.hosts]) }
function emitTunnels() { emit('update:tunnels', [...props.tunnels]) }
function emitProxyGroups() { emit('update:proxyGroups', [...props.proxyGroups]) }
function emitRuleProviders() { emit('update:ruleProviders', { ...props.ruleProviders }) }
function emitProxyProviders() { emit('update:proxyProviders', { ...props.proxyProviders }) }
function emitDns() { emit('update:dns', { ...props.dns }) }

function addProxyProvider() {
  if (!newPPName.value) return
  props.proxyProviders[newPPName.value] = { type: 'http', url: '', interval: 3600 }
  emitProxyProviders()
  newPPName.value = ''
}
function deleteProxyProvider(name) { delete props.proxyProviders[name]; emitProxyProviders() }
</script>
<style scoped>.hint { font-size:12px; color:#909399; margin-top:4px }</style>
