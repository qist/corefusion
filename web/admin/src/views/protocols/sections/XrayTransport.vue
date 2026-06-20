<template>
  <div>
    <el-form label-width="140px" label-position="left">
      <el-form-item label="Default Network">
        <el-select v-model="form.network" style="width:100%" clearable @change="emit">
          <el-option label="Raw (TCP)" value="raw" /><el-option label="WebSocket" value="ws" />
          <el-option label="gRPC" value="grpc" /><el-option label="HTTP/2" value="http" />
          <el-option label="mKCP" value="kcp" /><el-option label="QUIC" value="quic" />
          <el-option label="HTTPUpgrade" value="httpupgrade" /><el-option label="XHTTP (SplitHTTP)" value="xhttp" />
          <el-option label="Hysteria" value="hysteria" />
        </el-select>
      </el-form-item>
      <el-form-item label="Security">
        <el-select v-model="form.security" style="width:100%" @change="emit">
          <el-option label="None" value="none" /><el-option label="TLS" value="tls" /><el-option label="Reality" value="reality" />
        </el-select>
      </el-form-item>
    </el-form>

    <!-- TLS -->
    <template v-if="form.security === 'tls'">
      <el-divider content-position="left">TLS Settings</el-divider>
      <el-form label-width="180px" label-position="left">
        <el-form-item :label="t('protoCenter.tls.serverName')"><el-input v-model="form.tls.serverName" @change="emit" /></el-form-item>
        <el-form-item :label="t('protoCenter.tls.verifyPeerCertByName')">
          <el-input v-model="form.tls.verifyPeerCertByName" @change="emit" />
          <div class="hint">{{ t('protoCenter.tls.verifyPeerCertByNameHint') }}</div>
        </el-form-item>
        <el-form-item :label="t('protoCenter.tls.alpn')">
          <el-select v-model="form.tls._alpn" style="width:100%" multiple clearable @change="emit">
            <el-option label="h2" value="h2" /><el-option label="http/1.1" value="http/1.1" /><el-option label="h3" value="h3" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('protoCenter.tls.allowInsecure')"><el-switch v-model="form.tls.allowInsecure" @change="emit" /></el-form-item>
        <el-form-item :label="t('protoCenter.tls.minVersion')">
          <el-select v-model="form.tls.minVersion" style="width:100%" clearable @change="emit">
            <el-option label="TLS 1.0" value="1.0" /><el-option label="TLS 1.1" value="1.1" />
            <el-option label="TLS 1.2" value="1.2" /><el-option label="TLS 1.3" value="1.3" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('protoCenter.tls.maxVersion')">
          <el-select v-model="form.tls.maxVersion" style="width:100%" clearable @change="emit">
            <el-option label="TLS 1.2" value="1.2" /><el-option label="TLS 1.3" value="1.3" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('protoCenter.tls.cipherSuites')">
          <el-select v-model="form.tls._cipherSuites" style="width:100%" multiple clearable @change="emit" :placeholder="t('protoCenter.tls.cipherSuitesHint')">
            <el-option label="TLS_AES_128_GCM_SHA256" value="TLS_AES_128_GCM_SHA256" />
            <el-option label="TLS_AES_256_GCM_SHA384" value="TLS_AES_256_GCM_SHA384" />
            <el-option label="TLS_CHACHA20_POLY1305_SHA256" value="TLS_CHACHA20_POLY1305_SHA256" />
            <el-option label="TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA" value="TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA" />
            <el-option label="TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA" value="TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA" />
            <el-option label="TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256" value="TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256" />
            <el-option label="TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384" value="TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384" />
            <el-option label="TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256" value="TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256" />
            <el-option label="TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384" value="TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384" />
            <el-option label="TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256" value="TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256" />
            <el-option label="TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256" value="TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('protoCenter.tls.curvePreferences')">
          <el-select v-model="form.tls._curves" style="width:100%" multiple clearable @change="emit">
            <el-option label="CurveP256" value="CurveP256" /><el-option label="CurveP384" value="CurveP384" />
            <el-option label="CurveP521" value="CurveP521" /><el-option label="X25519" value="X25519" />
            <el-option label="X25519MLKEM768" value="X25519MLKEM768" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('protoCenter.tls.disableSystemRoot')">
          <el-switch v-model="form.tls.disableSystemRoot" @change="emit" />
        </el-form-item>
        <el-form-item :label="t('protoCenter.tls.enableSessionResumption')">
          <el-switch v-model="form.tls.enableSessionResumption" @change="emit" />
        </el-form-item>
        <el-form-item :label="t('protoCenter.tls.pinnedPeerCertSha256')">
          <div v-for="(h, i) in form.tls._pinnedCerts" :key="i" style="display:flex;gap:8px;margin-bottom:4px">
            <el-input v-model="form.tls._pinnedCerts[i]" placeholder="SHA256 hex" style="flex:1" @change="emit" />
            <el-button size="small" type="danger" text @click="form.tls._pinnedCerts.splice(i,1);emit()">X</el-button>
          </div>
          <el-button size="small" @click="form.tls._pinnedCerts.push('')">+ Add</el-button>
          <div class="hint">{{ t('protoCenter.tls.pinnedPeerCertSha256Hint') }}</div>
        </el-form-item>
        <el-form-item :label="t('protoCenter.tls.masterKeyLog')">
          <el-input v-model="form.tls.masterKeyLog" @change="emit" />
        </el-form-item>
        <el-form-item :label="t('protoCenter.tls.fingerprint')">
          <el-select v-model="form.tls.fingerprint" style="width:100%" clearable @change="emit">
            <el-option label="chrome" value="chrome" /><el-option label="firefox" value="firefox" />
            <el-option label="safari" value="safari" /><el-option label="ios" value="ios" />
            <el-option label="android" value="android" /><el-option label="edge" value="edge" />
            <el-option label="random" value="random" /><el-option label="randomized" value="randomized" />
            <el-option label="unsafe" value="unsafe" />
          </el-select>
        </el-form-item>
      </el-form>

      <!-- Client-side ECH -->
      <el-divider content-position="left">{{ t('protoCenter.tls.echSection') }}</el-divider>
      <el-form label-width="180px" label-position="left">
        <el-form-item :label="t('protoCenter.tls.echConfigList')">
          <el-input v-model="form.tls.echConfigList" @change="emit" />
          <div class="hint">{{ t('protoCenter.tls.echConfigListHint') }}</div>
        </el-form-item>
      </el-form>
    </template>

    <!-- REALITY (Client-side) -->
    <template v-if="form.security === 'reality'">
      <el-divider content-position="left">REALITY Settings</el-divider>
      <el-form label-width="160px" label-position="left">
        <el-form-item label="Server Name">
          <el-input v-model="form.reality.serverName" @change="emit" />
          <div class="hint">必须匹配服务端 serverNames 之一</div>
        </el-form-item>
        <el-form-item label="Fingerprint">
          <el-select v-model="form.reality.fingerprint" style="width:100%" @change="emit">
            <el-option label="chrome" value="chrome" /><el-option label="firefox" value="firefox" />
            <el-option label="safari" value="safari" /><el-option label="ios" value="ios" />
            <el-option label="android" value="android" /><el-option label="edge" value="edge" />
            <el-option label="random" value="random" /><el-option label="randomized" value="randomized" />
          </el-select>
        </el-form-item>
        <el-form-item label="Password (Public Key)">
          <el-input v-model="form.reality.password" @change="emit" />
          <div class="hint">服务端 privateKey 对应的公钥</div>
        </el-form-item>
        <el-form-item label="Short ID">
          <el-input v-model="form.reality.shortId" placeholder="hex, 偶数长度" @change="emit" />
        </el-form-item>
        <el-form-item label="SpiderX">
          <el-input v-model="form.reality.spiderX" placeholder="爬虫初始路径" @change="emit" />
        </el-form-item>
      </el-form>
      <!-- Client params in settings -->
      <el-divider content-position="left">{{ t('protoCenter.tls.settingsSection') }}</el-divider>
      <el-form label-width="160px" label-position="left">
        <el-form-item label="Fingerprint">
          <el-select v-model="form.reality._settingsFingerprint" style="width:100%" @change="emit">
            <el-option label="chrome" value="chrome" /><el-option label="firefox" value="firefox" />
            <el-option label="safari" value="safari" /><el-option label="ios" value="ios" />
            <el-option label="android" value="android" /><el-option label="edge" value="edge" />
            <el-option label="random" value="random" /><el-option label="randomized" value="randomized" />
          </el-select>
        </el-form-item>
      </el-form>
    </template>

    <!-- WebSocket -->
    <template v-if="form.network === 'ws'">
      <el-divider content-position="left">WebSocket Settings</el-divider>
      <el-form label-width="160px" label-position="left">
        <el-form-item label="Path"><el-input v-model="form.ws.path" @change="emit" /></el-form-item>
        <el-form-item label="Host"><el-input v-model="form.ws.host" @change="emit" /></el-form-item>
      </el-form>
    </template>

    <!-- gRPC -->
    <template v-if="form.network === 'grpc'">
      <el-divider content-position="left">gRPC Settings</el-divider>
      <el-form label-width="160px" label-position="left">
        <el-form-item label="Service Name"><el-input v-model="form.grpc.serviceName" @change="emit" /></el-form-item>
        <el-form-item label="Multi Mode"><el-switch v-model="form.grpc.multiMode" @change="emit" /></el-form-item>
        <el-form-item label="Idle Timeout (s)"><el-input-number v-model="form.grpc.idle_timeout" :min="0" @change="emit" /></el-form-item>
      </el-form>
    </template>

    <!-- XHTTP (SplitHTTP) -->
    <template v-if="form.network === 'xhttp'">
      <el-divider content-position="left">XHTTP Settings</el-divider>
      <el-form label-width="160px" label-position="left">
        <el-form-item label="Path"><el-input v-model="form.xhttp.path" placeholder="/" @change="emit" /></el-form-item>
        <el-form-item label="Host"><el-input v-model="form.xhttp.host" placeholder="留空不配置" @change="emit" /></el-form-item>
        <el-form-item label="Mode">
          <el-select v-model="form.xhttp.mode" style="width:100%" clearable @change="emit">
            <el-option label="auto" value="" />
            <el-option label="packet-up" value="packet-up" />
            <el-option label="stream-up" value="stream-up" />
            <el-option label="stream-one" value="stream-one" />
          </el-select>
        </el-form-item>
      </el-form>

      <!-- Extra Settings -->
      <el-divider content-position="left">XHTTP Extra</el-divider>
      <el-form label-width="180px" label-position="left">
        <el-form-item label="x Padding Bytes">
          <el-input v-model="form.xhttp.extra.xPaddingBytes" placeholder="100-1000" style="width:200px" @change="emit" />
          <div class="hint">范围字符串或整数，随机化 HTTP 头长度</div>
        </el-form-item>
        <el-form-item label="Uplink HTTP Method">
          <el-checkbox v-model="form.xhttp.extra._showMethod" @change="emit">启用</el-checkbox>
          <el-select v-if="form.xhttp.extra._showMethod" v-model="form.xhttp.extra.uplinkHTTPMethod" style="width:100%;margin-top:4px" @change="emit">
            <el-option label="POST" value="POST" /><el-option label="GET" value="GET" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="['stream-up','stream-one'].includes(form.xhttp.mode)" label="No GRPC Header">
          <el-switch v-model="form.xhttp.extra.noGRPCHeader" @change="emit" />
          <div class="hint">禁用上游默认 application/grpc 头</div>
        </el-form-item>
        <el-form-item label="No SSE Header">
          <el-switch v-model="form.xhttp.extra.noSSEHeader" @change="emit" />
          <div class="hint">Server only</div>
        </el-form-item>
        <el-form-item label="Server Max Header Bytes">
          <el-checkbox v-model="form.xhttp.extra._showMaxHeader" @change="emit">启用</el-checkbox>
          <el-input-number v-if="form.xhttp.extra._showMaxHeader" v-model="form.xhttp.extra.serverMaxHeaderBytes" :min="0" style="margin-top:4px" @change="emit" />
        </el-form-item>
        <el-form-item label="Headers">
          <div v-for="(h, i) in form.xhttp.extra._headers" :key="i" style="display:flex;gap:8px;margin-bottom:4px">
            <el-input v-model="h.key" placeholder="Header name" style="width:30%" />
            <el-input v-model="h.value" placeholder="Header value" style="width:50%" />
            <el-button size="small" type="danger" text @click="form.xhttp.extra._headers.splice(i,1);emit()">X</el-button>
          </div>
          <el-button size="small" @click="form.xhttp.extra._headers.push({key:'',value:''})">+ Add Header</el-button>
        </el-form-item>
      </el-form>

      <!-- Packet-Up Settings -->
      <template v-if="!form.xhttp.mode || form.xhttp.mode === 'packet-up'">
        <el-divider content-position="left">Scamper (packet-up only)</el-divider>
        <el-form label-width="180px" label-position="left">
          <el-form-item label="Max Each Post Bytes">
            <el-input-number v-model="form.xhttp.extra.scMaxEachPostBytes" :min="0" @change="emit" />
            <div class="hint">每个 POST 请求最大数据量，默认 1000000</div>
          </el-form-item>
          <el-form-item label="Min Posts Interval (ms)">
            <el-input-number v-model="form.xhttp.extra.scMinPostsIntervalMs" :min="0" @change="emit" />
            <div class="hint">POST 请求最小间隔，默认 30ms</div>
          </el-form-item>
          <el-form-item label="Max Buffered Posts">
            <el-input v-model="form.xhttp.extra.scMaxBufferedPosts" placeholder="30" style="width:200px" @change="emit" />
            <div class="hint">服务端缓存的最大 POST 数，默认 30</div>
          </el-form-item>
        </el-form>
      </template>

      <!-- Stream-Up Settings -->
      <template v-if="form.xhttp.mode === 'stream-up'">
        <el-divider content-position="left">Stream-Up Settings</el-divider>
        <el-form label-width="180px" label-position="left">
          <el-form-item label="Stream Up Server Secs">
            <el-input v-model="form.xhttp.extra.scStreamUpServerSecs" placeholder="20-80" style="width:200px" @change="emit" />
            <div class="hint">服务端保活间隔，-1 禁用，默认 20-80</div>
          </el-form-item>
        </el-form>
      </template>

      <!-- Xmux -->
      <el-divider content-position="left">Xmux (Multiplexing)</el-divider>
      <el-form label-width="180px" label-position="left">
        <el-form-item label="Max Concurrency">
          <el-input v-model="form.xhttp.extra.xmux.maxConcurrency" placeholder="16-32" style="width:200px" @change="onXmuxConcurrency" />
          <div class="hint">与 Max Connections 互斥</div>
        </el-form-item>
        <el-form-item label="Max Connections">
          <el-input v-model="form.xhttp.extra.xmux.maxConnections" placeholder="0" style="width:200px" @change="onXmuxConnections" />
          <div class="hint">与 Max Concurrency 互斥</div>
        </el-form-item>
        <el-form-item label="C Max Reuse Times">
          <el-input v-model="form.xhttp.extra.xmux.cMaxReuseTimes" placeholder="0" style="width:200px" @change="emit" />
        </el-form-item>
        <el-form-item label="H Max Request Times">
          <el-input v-model="form.xhttp.extra.xmux.hMaxRequestTimes" placeholder="600-900" style="width:200px" @change="emit" />
        </el-form-item>
        <el-form-item label="H Max Reusable Secs">
          <el-input v-model="form.xhttp.extra.xmux.hMaxReusableSecs" placeholder="1800-3000" style="width:200px" @change="emit" />
        </el-form-item>
        <el-form-item label="H Keep Alive Period">
          <el-input-number v-model="form.xhttp.extra.xmux.hKeepAlivePeriod" :min="-1" @change="emit" />
          <div class="hint">0 使用默认值，-1 禁用</div>
        </el-form-item>
      </el-form>

      <!-- Download Settings (Upstream/Downstream Separation) -->
      <el-divider content-position="left">Download Settings (上下行分离)</el-divider>
      <el-form label-width="180px" label-position="left">
        <el-form-item label="启用上下行分离">
          <el-switch v-model="form.xhttp.extra._showDownload" @change="emit" />
        </el-form-item>
      </el-form>
      <template v-if="form.xhttp.extra._showDownload">
        <el-form label-width="180px" label-position="left">
          <el-form-item label="Download Address">
            <el-input v-model="form.xhttp.extra.downloadSettings.address" placeholder="下行地址" @change="emit" />
          </el-form-item>
          <el-form-item label="Download Port">
            <el-input-number v-model="form.xhttp.extra.downloadSettings.port" :min="1" :max="65535" @change="emit" />
          </el-form-item>
          <el-form-item label="Download Security">
            <el-select v-model="form.xhttp.extra.downloadSettings.security" style="width:100%" clearable @change="emit">
              <el-option label="none" value="none" /><el-option label="tls" value="tls" /><el-option label="reality" value="reality" />
            </el-select>
          </el-form-item>
          <el-form-item v-if="form.xhttp.extra.downloadSettings.security === 'tls'" label="TLS Server Name">
            <el-input v-model="form.xhttp.extra.downloadSettings.tlsServerName" @change="emit" />
          </el-form-item>
          <el-form-item v-if="form.xhttp.extra.downloadSettings.security === 'tls'" label="TLS ALPN">
            <el-input v-model="form.xhttp.extra.downloadSettings.tlsAlpnStr" placeholder="h2, http/1.1" @change="emit" />
          </el-form-item>
          <el-form-item v-if="form.xhttp.extra.downloadSettings.security === 'tls'" label="Allow Insecure">
            <el-switch v-model="form.xhttp.extra.downloadSettings.tlsAllowInsecure" @change="emit" />
          </el-form-item>
          <el-form-item v-if="form.xhttp.extra.downloadSettings.security === 'reality'" label="Reality Target">
            <el-input v-model="form.xhttp.extra.downloadSettings.realityTarget" placeholder="www.microsoft.com:443" @change="emit" />
          </el-form-item>
          <el-form-item v-if="form.xhttp.extra.downloadSettings.security === 'reality'" label="Reality Server Names">
            <el-input v-model="form.xhttp.extra.downloadSettings.realityServerNamesStr" @change="emit" />
          </el-form-item>
          <el-form-item v-if="form.xhttp.extra.downloadSettings.security === 'reality'" label="Reality Private Key">
            <el-input v-model="form.xhttp.extra.downloadSettings.realityPrivateKey" @change="emit" />
          </el-form-item>
          <el-form-item v-if="form.xhttp.extra.downloadSettings.security === 'reality'" label="Reality Short IDs">
            <el-input v-model="form.xhttp.extra.downloadSettings.realityShortIdsStr" placeholder="hex IDs, comma-sep" @change="emit" />
          </el-form-item>
          <el-form-item label="Download Path">
            <el-input v-model="form.xhttp.extra.downloadSettings.xhttpPath" placeholder="必须与上传 path 相同" @change="emit" />
          </el-form-item>
          <el-form-item label="Download Mode">
            <el-select v-model="form.xhttp.extra.downloadSettings.xhttpMode" style="width:100%" clearable @change="emit">
              <el-option label="auto" value="" />
              <el-option label="packet-up" value="packet-up" />
              <el-option label="stream-up" value="stream-up" />
              <el-option label="stream-one" value="stream-one" />
            </el-select>
          </el-form-item>
        </el-form>
      </template>
    </template>

    <!-- Hysteria -->
    <template v-if="form.network === 'hysteria'">
      <el-divider content-position="left">Hysteria Settings</el-divider>
      <el-form label-width="160px" label-position="left">
        <el-form-item label="Version"><el-input-number v-model="form.hysteria.version" :min="0" @change="emit" /></el-form-item>
        <el-form-item label="Auth"><el-input v-model="form.hysteria.auth" @change="emit" /></el-form-item>
        <el-form-item label="Up Bandwidth"><el-input v-model="form.hysteria.up" placeholder="10 mbps" @change="emit" /></el-form-item>
        <el-form-item label="Down Bandwidth"><el-input v-model="form.hysteria.down" placeholder="100 mbps" @change="emit" /></el-form-item>
      </el-form>
    </template>

    <!-- Raw TCP -->
    <template v-if="form.network === 'raw'">
      <el-divider content-position="left">Raw TCP Settings</el-divider>
      <el-form label-width="160px" label-position="left">
        <el-form-item label="Header Type">
          <el-select v-model="form.raw.headerType" style="width:100%" clearable @change="emit">
            <el-option label="none" value="none" /><el-option label="http" value="http" />
          </el-select>
        </el-form-item>
      </el-form>
    </template>

    <!-- mKCP -->
    <template v-if="form.network === 'kcp'">
      <el-divider content-position="left">mKCP Settings</el-divider>
      <el-form label-width="160px" label-position="left">
        <el-form-item label="MTU"><el-input-number v-model="form.kcp.mtu" :min="0" @change="emit" /></el-form-item>
        <el-form-item label="TTI"><el-input-number v-model="form.kcp.tti" :min="0" @change="emit" /></el-form-item>
        <el-form-item label="Uplink Capacity"><el-input-number v-model="form.kcp.uplinkCapacity" :min="0" @change="emit" /></el-form-item>
        <el-form-item label="Downlink Capacity"><el-input-number v-model="form.kcp.downlinkCapacity" :min="0" @change="emit" /></el-form-item>
        <el-form-item label="Congestion"><el-switch v-model="form.kcp.congestion" @change="emit" /></el-form-item>
        <el-form-item label="Seed"><el-input v-model="form.kcp.seed" @change="emit" /></el-form-item>
        <el-form-item label="Header Type">
          <el-select v-model="form.kcp.headerType" style="width:100%" clearable @change="emit">
            <el-option label="none" value="none" /><el-option label="srtp" value="srtp" /><el-option label="utp" value="utp" />
            <el-option label="wechat-video" value="wechat-video" /><el-option label="dtls" value="dtls" /><el-option label="wireguard" value="wireguard" />
          </el-select>
        </el-form-item>
      </el-form>
    </template>

    <!-- Socket Options -->
    <el-divider content-position="left">Socket Options</el-divider>
    <el-form label-width="180px" label-position="left">
      <el-form-item label="Mark"><el-input-number v-model="form.sockopt.mark" :min="0" @change="emit" /></el-form-item>
      <el-form-item label="TCP Fast Open"><el-switch v-model="form.sockopt.tcpFastOpen" @change="emit" /></el-form-item>
      <el-form-item label="TProxy">
        <el-select v-model="form.sockopt.tproxy" style="width:100%" clearable @change="emit">
          <el-option label="off" value="off" /><el-option label="redirect" value="redirect" /><el-option label="tproxy" value="tproxy" />
        </el-select>
      </el-form-item>
      <el-form-item label="Domain Strategy">
        <el-select v-model="form.sockopt.domainStrategy" style="width:100%" clearable @change="emit">
          <el-option label="AsIs" value="AsIs" /><el-option label="UseIP" value="UseIP" /><el-option label="UseIPv4" value="UseIPv4" /><el-option label="UseIPv6" value="UseIPv6" />
          <el-option label="ForceIP" value="ForceIP" /><el-option label="ForceIPv4" value="ForceIPv4" /><el-option label="ForceIPv6" value="ForceIPv6" />
        </el-select>
      </el-form-item>
      <el-form-item label="Dialer Proxy"><el-input v-model="form.sockopt.dialerProxy" placeholder="Outbound tag" @change="emit" /></el-form-item>
      <el-form-item label="Interface"><el-input v-model="form.sockopt.interface" placeholder="eth0" @change="emit" /></el-form-item>
      <el-form-item label="TCP Congestion"><el-input v-model="form.sockopt.tcpcongestion" placeholder="bbr, cubic" @change="emit" /></el-form-item>
      <el-form-item label="TCP Max Seg"><el-input-number v-model="form.sockopt.tcpMaxSeg" :min="0" @change="emit" /></el-form-item>
      <el-form-item label="TCP Window Clamp"><el-input-number v-model="form.sockopt.tcpWindowClamp" :min="0" @change="emit" /></el-form-item>
      <el-form-item label="TCP Keep Alive Idle (s)"><el-input-number v-model="form.sockopt.tcpKeepAliveIdle" :min="0" @change="emit" /></el-form-item>
      <el-form-item label="TCP Keep Alive Interval (s)"><el-input-number v-model="form.sockopt.tcpKeepAliveInterval" :min="0" @change="emit" /></el-form-item>
      <el-form-item label="TCP User Timeout (ms)"><el-input-number v-model="form.sockopt.tcpUserTimeout" :min="0" @change="emit" /></el-form-item>
      <el-form-item label="TCP MPTCP"><el-switch v-model="form.sockopt.tcpMptcp" @change="emit" /></el-form-item>
      <el-form-item label="V6 Only"><el-switch v-model="form.sockopt.v6only" @change="emit" /></el-form-item>
      <el-form-item label="Accept Proxy Protocol"><el-switch v-model="form.sockopt.acceptProxyProtocol" @change="emit" /></el-form-item>
      <el-form-item label="Address Port Strategy">
        <el-select v-model="form.sockopt.addressPortStrategy" style="width:100%" clearable @change="emit">
          <el-option label="none" value="" /><el-option label="SrvPortOnly" value="SrvPortOnly" /><el-option label="SrvAddressOnly" value="SrvAddressOnly" /><el-option label="SrvPortAndAddress" value="SrvPortAndAddress" />
          <el-option label="TxtPortOnly" value="TxtPortOnly" /><el-option label="TxtAddressOnly" value="TxtAddressOnly" /><el-option label="TxtPortAndAddress" value="TxtPortAndAddress" />
        </el-select>
      </el-form-item>
    </el-form>

    <!-- Happy Eyeballs -->
    <el-divider content-position="left">Happy Eyeballs</el-divider>
    <el-form label-width="180px" label-position="left">
      <el-form-item label="Try Delay (ms)"><el-input-number v-model="form.sockopt.happyEyeballs.tryDelayMs" :min="0" @change="emit" /></el-form-item>
      <el-form-item label="Prioritize IPv6"><el-switch v-model="form.sockopt.happyEyeballs.prioritizeIPv6" @change="emit" /></el-form-item>
      <el-form-item label="Interleave"><el-input-number v-model="form.sockopt.happyEyeballs.interleave" :min="1" @change="emit" /></el-form-item>
      <el-form-item label="Max Concurrent Try"><el-input-number v-model="form.sockopt.happyEyeballs.maxConcurrentTry" :min="0" @change="emit" /></el-form-item>
    </el-form>
  </div>
</template>
<script setup>
import { reactive, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
const props = defineProps({ data: Object })
const emitFn = defineEmits(['update'])

function defaultXhttp() {
  return {
    path: '/', host: '', mode: '',
    extra: {
      _showMethod: false, uplinkHTTPMethod: 'POST',
      noGRPCHeader: false, noSSEHeader: false,
      _showMaxHeader: false, serverMaxHeaderBytes: 0,
      _headers: [],
      xPaddingBytes: '100-1000', xPaddingObfsMode: false,
      scMaxEachPostBytes: 1000000, scMinPostsIntervalMs: 30,
      scMaxBufferedPosts: 30, scStreamUpServerSecs: '20-80',
      xmux: {
        maxConcurrency: '16-32', maxConnections: 0,
        cMaxReuseTimes: 0, hMaxRequestTimes: '600-900',
        hMaxReusableSecs: '1800-3000', hKeepAlivePeriod: 0,
      },
      _showDownload: false,
      downloadSettings: {
        address: '', port: 443, security: 'none',
        tlsServerName: '', tlsAlpnStr: '', tlsAllowInsecure: false,
        realityTarget: '', realityServerNamesStr: '', realityPrivateKey: '', realityShortIdsStr: '',
        xhttpPath: '', xhttpMode: '',
      },
    },
  }
}

const form = reactive({
  network: 'raw',
  security: 'none',
  tls: { serverName: '', verifyPeerCertByName: '', _alpn: [], allowInsecure: false, minVersion: '', maxVersion: '', _cipherSuites: [], _curves: [], disableSystemRoot: false, enableSessionResumption: false, _pinnedCerts: [], masterKeyLog: '', fingerprint: '', echConfigList: '' },
  reality: { serverName: '', fingerprint: 'chrome', password: '', shortId: '', spiderX: '', _settingsFingerprint: '' },
  ws: { path: '/', host: '' },
  grpc: { serviceName: '', multiMode: false, idle_timeout: 60 },
  xhttp: defaultXhttp(),
  hysteria: { version: 0, auth: '', up: '', down: '' },
  raw: { headerType: 'none' },
  kcp: { mtu: 1350, tti: 50, uplinkCapacity: 5, downlinkCapacity: 20, congestion: false, seed: '', headerType: 'none' },
  sockopt: {
    mark: 0, tcpFastOpen: false, tproxy: 'off', domainStrategy: '', dialerProxy: '', interface: '',
    tcpcongestion: '', tcpMaxSeg: 0, tcpWindowClamp: 0, tcpKeepAliveIdle: 0, tcpKeepAliveInterval: 0,
    tcpUserTimeout: 0, tcpMptcp: false, v6only: false, acceptProxyProtocol: false, addressPortStrategy: '',
    happyEyeballs: { tryDelayMs: 0, prioritizeIPv6: false, interleave: 1, maxConcurrentTry: 4 },
  },
})

function sync() {
  if (!props.data) return
  form.network = props.data.network || 'raw'
  form.security = props.data.security || 'none'
  if (props.data.tlsSettings) {
    var t = props.data.tlsSettings
    var cs = (t.settings || [])[0] || {}
    form.tls = {
      serverName: t.serverName || '', verifyPeerCertByName: t.verifyPeerCertByName || '',
      _alpn: t.alpn || [], allowInsecure: t.allowInsecure || false,
      minVersion: t.minVersion || '', maxVersion: t.maxVersion || '',
      _cipherSuites: t.cipherSuites ? t.cipherSuites.split(':') : [], _curves: t.curvePreferences || [],
      disableSystemRoot: t.disableSystemRoot || false, enableSessionResumption: t.enableSessionResumption || false,
      _pinnedCerts: t.pinnedPeerCertSha256 || [], masterKeyLog: t.masterKeyLog || '',
      fingerprint: cs.fingerprint || '', echConfigList: cs.echConfigList || '',
    }
  }
  if (props.data.realitySettings) {
    var r = props.data.realitySettings
    var rs = (r.settings || [])[0] || {}
    form.reality = {
      serverName: r.serverName || '', fingerprint: r.fingerprint || 'chrome',
      password: r.password || '', shortId: r.shortId || '', spiderX: r.spiderX || '',
      _settingsFingerprint: rs.fingerprint || '',
    }
  }
  if (props.data.wsSettings) { var w = props.data.wsSettings; form.ws = { path: w.path || '/', host: w.host || '' } }
  if (props.data.grpcSettings) { var g = props.data.grpcSettings; form.grpc = { serviceName: g.serviceName || '', multiMode: g.multiMode || false, idle_timeout: g.idle_timeout || 60 } }
  if (props.data.xhttpSettings || props.data.splithttpSettings) {
    var x = props.data.xhttpSettings || props.data.splithttpSettings
    var extra = x.extra || x
    var xmux = extra.xmux || {}
    var dl = extra.downloadSettings
    var dlForm = defaultXhttp().extra.downloadSettings
    if (dl) {
      var dlTls = dl.tlsSettings || {}
      var dlReality = dl.realitySettings || {}
      var dlXhttp = dl.xhttpSettings || {}
      dlForm = {
        address: dl.address || '', port: dl.port || 443, security: dl.security || 'none',
        tlsServerName: dlTls.serverName || '', tlsAlpnStr: (dlTls.alpn || []).join(', '), tlsAllowInsecure: dlTls.allowInsecure || false,
        realityTarget: dlReality.target || '', realityServerNamesStr: (dlReality.serverNames || []).join(', '), realityPrivateKey: dlReality.privateKey || '', realityShortIdsStr: (dlReality.shortIds || []).join(', ''),
        xhttpPath: dlXhttp.path || '', xhttpMode: dlXhttp.mode || '',
      }
    }
    form.xhttp = {
      path: x.path || '/', host: x.host || '', mode: x.mode || '',
      extra: {
        _showMethod: !!extra.uplinkHTTPMethod, uplinkHTTPMethod: extra.uplinkHTTPMethod || 'POST',
        noGRPCHeader: extra.noGRPCHeader || false, noSSEHeader: extra.noSSEHeader || false,
        _showMaxHeader: !!extra.serverMaxHeaderBytes, serverMaxHeaderBytes: extra.serverMaxHeaderBytes || 0,
        _headers: Object.entries(extra.headers || {}).map(function(e) { return { key: e[0], value: e[1] } }),
        xPaddingBytes: extra.xPaddingBytes || '100-1000', xPaddingObfsMode: extra.xPaddingObfsMode || false,
        scMaxEachPostBytes: extra.scMaxEachPostBytes || 1000000, scMinPostsIntervalMs: extra.scMinPostsIntervalMs || 30,
        scMaxBufferedPosts: extra.scMaxBufferedPosts || 30, scStreamUpServerSecs: extra.scStreamUpServerSecs || '20-80',
        xmux: {
          maxConcurrency: xmux.maxConcurrency || '16-32', maxConnections: xmux.maxConnections || 0,
          cMaxReuseTimes: xmux.cMaxReuseTimes || 0, hMaxRequestTimes: xmux.hMaxRequestTimes || '600-900',
          hMaxReusableSecs: xmux.hMaxReusableSecs || '1800-3000', hKeepAlivePeriod: xmux.hKeepAlivePeriod || 0,
        },
        _showDownload: !!dl, downloadSettings: dlForm,
      },
    }
  }
  if (props.data.hysteriaSettings) { var h = props.data.hysteriaSettings; form.hysteria = { version: h.version || 0, auth: h.auth || '', up: h.up || '', down: h.down || '' } }
  if (props.data.rawSettings || props.data.tcpSettings) { var rt = props.data.rawSettings || props.data.tcpSettings; form.raw = { headerType: (rt.header && rt.header.type) || 'none' } }
  if (props.data.kcpSettings) { var k = props.data.kcpSettings; form.kcp = { mtu: k.mtu || 1350, tti: k.tti || 50, uplinkCapacity: k.uplinkCapacity || 5, downlinkCapacity: k.downlinkCapacity || 20, congestion: k.congestion || false, seed: k.seed || '', headerType: (k.header && k.header.type) || 'none' } }
  if (props.data.sockopt) {
    var s = props.data.sockopt
    form.sockopt = {
      mark: s.mark || 0, tcpFastOpen: s.tcpFastOpen || false, tproxy: s.tproxy || 'off', domainStrategy: s.domainStrategy || '',
      dialerProxy: s.dialerProxy || '', interface: s.interface || '', tcpcongestion: s.tcpcongestion || s.tcpCongestion || '',
      tcpMaxSeg: s.tcpMaxSeg || 0, tcpWindowClamp: s.tcpWindowClamp || 0, tcpKeepAliveIdle: s.tcpKeepAliveIdle || 0,
      tcpKeepAliveInterval: s.tcpKeepAliveInterval || 0, tcpUserTimeout: s.tcpUserTimeout || 0, tcpMptcp: s.tcpMptcp || false,
      v6only: s.v6only || s.V6Only || false, acceptProxyProtocol: s.acceptProxyProtocol || false, addressPortStrategy: s.addressPortStrategy || '',
      happyEyeballs: s.happyEyeballs || { tryDelayMs: 0, prioritizeIPv6: false, interleave: 1, maxConcurrentTry: 4 },
    }
  }
}

function build() {
  var out = { network: form.network, security: form.security }
  if (form.security === 'tls') {
    var t = form.tls
    var tls = {}
    if (t.serverName) tls.serverName = t.serverName
    if (t.verifyPeerCertByName) tls.verifyPeerCertByName = t.verifyPeerCertByName
    if (t._alpn && t._alpn.length) tls.alpn = t._alpn
    if (t.allowInsecure) tls.allowInsecure = true
    if (t.minVersion) tls.minVersion = t.minVersion
    if (t.maxVersion) tls.maxVersion = t.maxVersion
    if (t._cipherSuites && t._cipherSuites.length) tls.cipherSuites = t._cipherSuites.join(':')
    if (t._curves && t._curves.length) tls.curvePreferences = t._curves
    if (t.disableSystemRoot) tls.disableSystemRoot = true
    if (t.enableSessionResumption) tls.enableSessionResumption = true
    if (t._pinnedCerts && t._pinnedCerts.filter(Boolean).length) tls.pinnedPeerCertSha256 = t._pinnedCerts.filter(Boolean)
    if (t.masterKeyLog) tls.masterKeyLog = t.masterKeyLog
    // Client params in settings array
    var settingsObj = {}
    if (t.fingerprint) settingsObj.fingerprint = t.fingerprint
    if (t.echConfigList) settingsObj.echConfigList = t.echConfigList
    if (Object.keys(settingsObj).length) tls.settings = [settingsObj]
    out.tlsSettings = tls
  }
  if (form.security === 'reality') {
    var r = form.reality
    var reality = {}
    if (r.serverName) reality.serverName = r.serverName
    if (r.fingerprint) reality.fingerprint = r.fingerprint
    if (r.password) reality.password = r.password
    if (r.shortId) reality.shortId = r.shortId
    if (r.spiderX) reality.spiderX = r.spiderX
    // Client params in settings array
    if (r._settingsFingerprint) reality.settings = [{ fingerprint: r._settingsFingerprint }]
    out.realitySettings = reality
  }
  if (form.network === 'ws') out.wsSettings = form.ws
  if (form.network === 'grpc') out.grpcSettings = form.grpc
  if (form.network === 'xhttp') {
    var xh = form.xhttp
    var xhSettings = {}
    if (xh.path && xh.path !== '/') xhSettings.path = xh.path
    if (xh.host && xh.host.trim()) xhSettings.host = xh.host.trim()
    if (xh.mode) xhSettings.mode = xh.mode
    var s = xh.extra || {}
    var extra = {}
    // Client extra settings
    if (s._showMethod && s.uplinkHTTPMethod) extra.uplinkHTTPMethod = s.uplinkHTTPMethod
    if (s.noGRPCHeader) extra.noGRPCHeader = true
    if (s.noSSEHeader) extra.noSSEHeader = true
    if (s._showMaxHeader && s.serverMaxHeaderBytes) extra.serverMaxHeaderBytes = s.serverMaxHeaderBytes
    if (s._headers && s._headers.length) {
      var headers = {}
      s._headers.forEach(function(h) { if (h.key) headers[h.key] = h.value })
      if (Object.keys(headers).length) extra.headers = headers
    }
    // Padding
    if (s.xPaddingBytes && s.xPaddingBytes !== '100-1000') extra.xPaddingBytes = s.xPaddingBytes
    if (s.xPaddingObfsMode) extra.xPaddingObfsMode = true
    // Scamper
    if (s.scMaxEachPostBytes && s.scMaxEachPostBytes !== 1000000) extra.scMaxEachPostBytes = s.scMaxEachPostBytes
    if (s.scMinPostsIntervalMs && s.scMinPostsIntervalMs !== 30) extra.scMinPostsIntervalMs = s.scMinPostsIntervalMs
    if (s.scMaxBufferedPosts && s.scMaxBufferedPosts !== 30) extra.scMaxBufferedPosts = s.scMaxBufferedPosts
    if (s.scStreamUpServerSecs && s.scStreamUpServerSecs !== '20-80') extra.scStreamUpServerSecs = s.scStreamUpServerSecs
    // Xmux
    var xmux = s.xmux || {}
    var xmuxObj = {}
    if (xmux.maxConcurrency && xmux.maxConcurrency !== '16-32') xmuxObj.maxConcurrency = xmux.maxConcurrency
    if (xmux.maxConnections) xmuxObj.maxConnections = xmux.maxConnections
    if (xmux.cMaxReuseTimes) xmuxObj.cMaxReuseTimes = xmux.cMaxReuseTimes
    if (xmux.hMaxRequestTimes && xmux.hMaxRequestTimes !== '600-900') xmuxObj.hMaxRequestTimes = xmux.hMaxRequestTimes
    if (xmux.hMaxReusableSecs && xmux.hMaxReusableSecs !== '1800-3000') xmuxObj.hMaxReusableSecs = xmux.hMaxReusableSecs
    if (xmux.hKeepAlivePeriod) xmuxObj.hKeepAlivePeriod = xmux.hKeepAlivePeriod
    if (Object.keys(xmuxObj).length) extra.xmux = xmuxObj
    // Download settings (upstream/downstream separation)
    if (s._showDownload && s.downloadSettings) {
      var dl = s.downloadSettings
      var dlObj = { network: 'xhttp', port: dl.port || 443 }
      if (dl.address) dlObj.address = dl.address
      if (dl.security && dl.security !== 'none') {
        dlObj.security = dl.security
        if (dl.security === 'tls') {
          var dlTls = {}
          if (dl.tlsServerName) dlTls.serverName = dl.tlsServerName
          if (dl.tlsAlpnStr) dlTls.alpn = dl.tlsAlpnStr.split(',').map(function(v) { return v.trim() })
          if (dl.tlsAllowInsecure) dlTls.allowInsecure = true
          if (Object.keys(dlTls).length) dlObj.tlsSettings = dlTls
        }
        if (dl.security === 'reality') {
          var dlR = {}
          if (dl.realityTarget) dlR.target = dl.realityTarget
          if (dl.realityServerNamesStr) dlR.serverNames = dl.realityServerNamesStr.split(',').map(function(v) { return v.trim() })
          if (dl.realityPrivateKey) dlR.privateKey = dl.realityPrivateKey
          if (dl.realityShortIdsStr) dlR.shortIds = dl.realityShortIdsStr.split(',').map(function(v) { return v.trim() })
          if (Object.keys(dlR).length) dlObj.realitySettings = dlR
        }
      }
      var dlXhttp = {}
      if (dl.xhttpPath) dlXhttp.path = dl.xhttpPath
      if (dl.xhttpMode) dlXhttp.mode = dl.xhttpMode
      if (Object.keys(dlXhttp).length) dlObj.xhttpSettings = dlXhttp
      extra.downloadSettings = dlObj
    }
    if (Object.keys(extra).length) xhSettings.extra = extra
    out.xhttpSettings = xhSettings
  }
  if (form.network === 'hysteria') out.hysteriaSettings = { version: form.hysteria.version, auth: form.hysteria.auth || undefined, up: form.hysteria.up || undefined, down: form.hysteria.down || undefined }
  if (form.network === 'raw') out.rawSettings = { header: { type: form.raw.headerType || 'none' } }
  if (form.network === 'kcp') out.kcpSettings = { mtu: form.kcp.mtu, tti: form.kcp.tti, uplinkCapacity: form.kcp.uplinkCapacity, downlinkCapacity: form.kcp.downlinkCapacity, congestion: form.kcp.congestion, seed: form.kcp.seed || undefined, header: { type: form.kcp.headerType || 'none' } }
  out.sockopt = {
    mark: form.sockopt.mark, tcpFastOpen: form.sockopt.tcpFastOpen, tproxy: form.sockopt.tproxy,
    domainStrategy: form.sockopt.domainStrategy || undefined, dialerProxy: form.sockopt.dialerProxy || undefined,
    interface: form.sockopt.interface || undefined, tcpcongestion: form.sockopt.tcpcongestion || undefined,
    tcpMaxSeg: form.sockopt.tcpMaxSeg || undefined, tcpWindowClamp: form.sockopt.tcpWindowClamp || undefined,
    tcpKeepAliveIdle: form.sockopt.tcpKeepAliveIdle || undefined, tcpKeepAliveInterval: form.sockopt.tcpKeepAliveInterval || undefined,
    tcpUserTimeout: form.sockopt.tcpUserTimeout || undefined, tcpMptcp: form.sockopt.tcpMptcp,
    v6only: form.sockopt.v6only || undefined, acceptProxyProtocol: form.sockopt.acceptProxyProtocol || undefined,
    addressPortStrategy: form.sockopt.addressPortStrategy || undefined,
    happyEyeballs: form.sockopt.happyEyeballs.tryDelayMs ? form.sockopt.happyEyeballs : undefined,
  }
  return out
}

function onXmuxConcurrency() {
  if (form.xhttp.extra.xmux.maxConcurrency > 0) {
    form.xhttp.extra.xmux.maxConnections = 0
  }
  emit()
}

function onXmuxConnections() {
  if (form.xhttp.extra.xmux.maxConnections > 0) {
    form.xhttp.extra.xmux.maxConcurrency = 0
  }
  emit()
}

function emit() { emitFn('update', build()) }
onMounted(sync)
</script>
<style scoped>
.hint { font-size: 12px; color: #909399; margin-top: 2px; }
</style>
