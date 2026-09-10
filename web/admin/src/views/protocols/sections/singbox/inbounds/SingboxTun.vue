<template>
  <el-form label-width="180px" label-position="left">
    <!-- 基础 -->
    <el-divider content-position="left">{{ t('singbox.protocol.tun.basicSection') }}</el-divider>
    <el-form-item :label="t('singbox.protocol.tun.interfaceName')">
      <el-input v-model="model.interface_name" placeholder="tun0" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.tun.stack')">
      <el-select v-model="model.stack" style="width:100%" @change="emit">
        <el-option label="system" value="system" />
        <el-option label="gvisor" value="gvisor" />
        <el-option label="mixed" value="mixed" />
        <el-option label="go" value="go" />
      </el-select>
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.tun.mtu')">
      <el-input-number v-model="model._tunMtu" :min="0" :max="65535" style="width:100%" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.tun.netns')">
      <el-input v-model="model._tunNetns" placeholder="" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.tun.address')">
      <el-input v-model="model._tunAddress" placeholder="172.19.0.1/30,fdfe:dcba:9876::1/126" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.tun.autoRoute')">
      <el-switch v-model="model.auto_route" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.tun.strictRoute')">
      <el-switch v-model="model.strict_route" @change="emit" />
    </el-form-item>

    <!-- DNS -->
    <el-divider content-position="left">{{ t('singbox.protocol.tun.dnsSection') }}</el-divider>
    <el-form-item :label="t('singbox.protocol.tun.dnsMode')">
      <el-select v-model="model._tunDnsMode" style="width:100%" clearable @change="emit">
        <el-option label="" value="" />
        <el-option label="disabled" value="disabled" />
        <el-option label="native" value="native" />
        <el-option label="hijack" value="hijack" />
      </el-select>
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.tun.dnsAddress')">
      <el-input v-model="model._tunDnsAddress" placeholder="172.18.0.1, fdfe::1" @change="emit" />
    </el-form-item>

    <!-- 自动重定向 -->
    <el-divider content-position="left">{{ t('singbox.protocol.tun.autoRedirectSection') }}</el-divider>
    <el-form-item :label="t('singbox.protocol.tun.autoRedirect')">
      <el-switch v-model="model._tunAutoRedirect" @change="emit" />
    </el-form-item>
    <template v-if="model._tunAutoRedirect">
      <el-form-item :label="t('singbox.protocol.tun.inputMark')">
        <el-input v-model="model._tunAutoRedirectInputMark" placeholder="0x1234" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.protocol.tun.outputMark')">
        <el-input v-model="model._tunAutoRedirectOutputMark" placeholder="0x5678" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.protocol.tun.resetMark')">
        <el-input v-model="model._tunAutoRedirectResetMark" placeholder="0x0" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.protocol.tun.tproxyMark')">
        <el-input v-model="model._tunAutoRedirectTproxyMark" placeholder="0x9abc" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.protocol.tun.nfqueue')">
        <el-input-number v-model="model._tunAutoRedirectNFQueue" :min="0" :max="65535" style="width:100%" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.protocol.tun.fallbackRuleIndex')">
        <el-input-number v-model="model._tunAutoRedirectFallbackRule" :min="0" style="width:100%" @change="emit" />
      </el-form-item>
    </template>

    <!-- 路由表 -->
    <el-divider content-position="left">{{ t('singbox.protocol.tun.routeSection') }}</el-divider>
    <el-form-item :label="t('singbox.protocol.tun.iproute2Table')">
      <el-input-number v-model="model._tunIpRoute2Table" :min="0" style="width:100%" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.tun.iproute2Rule')">
      <el-input-number v-model="model._tunIpRoute2Rule" :min="0" style="width:100%" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.tun.routeAddress')">
      <el-input v-model="model._tunRouteAddress" type="textarea" :rows="2" placeholder="172.16.0.0/12" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.tun.routeAddressSet')">
      <el-input v-model="model._tunRouteAddressSet" placeholder="geoip-cn" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.tun.routeExcludeAddress')">
      <el-input v-model="model._tunRouteExcludeAddress" type="textarea" :rows="2" placeholder="192.168.0.0/16" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.tun.routeExcludeAddressSet')">
      <el-input v-model="model._tunRouteExcludeAddressSet" placeholder="" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.tun.loopbackAddress')">
      <el-input v-model="model._tunLoopbackAddress" placeholder="127.0.0.1, ::1" @change="emit" />
    </el-form-item>

    <!-- 接口过滤 -->
    <el-divider content-position="left">{{ t('singbox.protocol.tun.interfaceSection') }}</el-divider>
    <el-form-item :label="t('singbox.protocol.tun.includeInterface')">
      <el-input v-model="model._tunIncludeInterface" placeholder="eth0, eth1" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.tun.excludeInterface')">
      <el-input v-model="model._tunExcludeInterface" placeholder="docker0" @change="emit" />
    </el-form-item>

    <!-- UID 过滤 -->
    <el-divider content-position="left">{{ t('singbox.protocol.tun.uidSection') }}</el-divider>
    <el-form-item :label="t('singbox.protocol.tun.includeUid')">
      <el-input v-model="model._tunIncludeUid" placeholder="1000, 1001" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.tun.includeUidRange')">
      <el-input v-model="model._tunIncludeUidRange" placeholder="1000-2000" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.tun.excludeUid')">
      <el-input v-model="model._tunExcludeUid" placeholder="0" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.tun.excludeUidRange')">
      <el-input v-model="model._tunExcludeUidRange" placeholder="1000-2000" @change="emit" />
    </el-form-item>

    <!-- 包名 / Android -->
    <el-divider content-position="left">{{ t('singbox.protocol.tun.packageSection') }}</el-divider>
    <el-form-item :label="t('singbox.protocol.tun.includeAndroidUser')">
      <el-input v-model="model._tunIncludeAndroidUser" placeholder="0, 1" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.tun.includePackage')">
      <el-input v-model="model._tunIncludePackage" type="textarea" :rows="2" placeholder="com.android.chrome" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.tun.excludePackage')">
      <el-input v-model="model._tunExcludePackage" type="textarea" :rows="2" placeholder="com.android.captiveportallogin" @change="emit" />
    </el-form-item>

    <!-- MAC 地址 -->
    <el-divider content-position="left">{{ t('singbox.protocol.tun.macSection') }}</el-divider>
    <el-form-item :label="t('singbox.protocol.tun.includeMac')">
      <el-input v-model="model._tunIncludeMacAddress" placeholder="00:11:22:33:44:55" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.tun.excludeMac')">
      <el-input v-model="model._tunExcludeMacAddress" placeholder="00:11:22:33:44:56" @change="emit" />
    </el-form-item>

    <!-- UDP NAT -->
    <el-divider content-position="left">{{ t('singbox.protocol.tun.udpSection') }}</el-divider>
    <el-form-item :label="t('singbox.protocol.tun.udpTimeout')">
      <el-input v-model="model._tunUdpTimeout" placeholder="300s" @change="emit" />
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.tun.udpMapping')">
      <el-select v-model="model._tunUdpMapping" style="width:100%" clearable @change="emit">
        <el-option label="" value="" />
        <el-option label="endpoint_independent" value="endpoint_independent" />
        <el-option label="address_dependent" value="address_dependent" />
        <el-option label="address_and_port_dependent" value="address_and_port_dependent" />
      </el-select>
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.tun.udpFiltering')">
      <el-select v-model="model._tunUdpFiltering" style="width:100%" clearable @change="emit">
        <el-option label="" value="" />
        <el-option label="endpoint_independent" value="endpoint_independent" />
        <el-option label="address_dependent" value="address_dependent" />
        <el-option label="address_and_port_dependent" value="address_and_port_dependent" />
      </el-select>
    </el-form-item>
    <el-form-item :label="t('singbox.protocol.tun.udpNatMax')">
      <el-input-number v-model="model._tunUdpNatMax" :min="0" style="width:100%" @change="emit" />
    </el-form-item>

    <!-- 其他 -->
    <el-divider content-position="left">{{ t('singbox.protocol.tun.advancedSection') }}</el-divider>
    <el-form-item :label="t('singbox.protocol.tun.excludeMptcp')">
      <el-switch v-model="model._tunExcludeMptcp" @change="emit" />
    </el-form-item>

    <!-- Platform HTTP Proxy -->
    <el-divider content-position="left">{{ t('singbox.protocol.tun.platformSection') }}</el-divider>
    <el-form-item :label="t('singbox.protocol.tun.httpProxyEnabled')">
      <el-switch v-model="model._tunPlatformHttpProxyEnabled" @change="emit" />
    </el-form-item>
    <template v-if="model._tunPlatformHttpProxyEnabled">
      <el-form-item :label="t('singbox.protocol.tun.httpProxyServer')">
        <el-input v-model="model._tunPlatformHttpProxyServer" placeholder="127.0.0.1" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.protocol.tun.httpProxyPort')">
        <el-input-number v-model="model._tunPlatformHttpProxyPort" :min="0" :max="65535" style="width:100%" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.protocol.tun.httpProxyBypassDomain')">
        <el-input v-model="model._tunPlatformHttpProxyBypassDomain" type="textarea" :rows="2" placeholder="*.local, 127.0.0.1" @change="emit" />
      </el-form-item>
      <el-form-item :label="t('singbox.protocol.tun.httpProxyMatchDomain')">
        <el-input v-model="model._tunPlatformHttpProxyMatchDomain" type="textarea" :rows="2" placeholder="*.example.com" @change="emit" />
      </el-form-item>
    </template>
  </el-form>
</template>

<script setup>
// TUN 协议专属设置 — 完整参数
// sing-box 源码: TunInboundOptions
//   InterfaceName, NetNs, MTU, Address, DNSMode, DNSAddress,
//   AutoRoute, IPRoute2TableIndex, IPRoute2RuleIndex,
//   AutoRedirect, AutoRedirectInputMark, AutoRedirectOutputMark, AutoRedirectResetMark,
//   AutoRedirectTProxyMark, AutoRedirectNFQueue, AutoRedirectFallbackRuleIndex,
//   ExcludeMPTCP, LoopbackAddress, StrictRoute,
//   RouteAddress, RouteAddressSet, RouteExcludeAddress, RouteExcludeAddressSet,
//   IncludeInterface, ExcludeInterface,
//   IncludeUID, IncludeUIDRange, ExcludeUID, ExcludeUIDRange,
//   IncludeAndroidUser, IncludePackage, ExcludePackage,
//   IncludeMACAddress, ExcludeMACAddress,
//   UDPTimeout, UDPMapping, UDPFiltering, UDPNatMax,
//   Stack, Platform{HTTPProxy}
// 注意：tun 无 TLS/Transport/Multiplex/Users 字段
// 通用 Listen(tag/listen/listen_port/sniff/domain_strategy) 由容器层提供
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
defineProps({ model: { type: Object, required: true } })
const emitFn = defineEmits(['update'])
function emit() { emitFn('update') }
</script>
