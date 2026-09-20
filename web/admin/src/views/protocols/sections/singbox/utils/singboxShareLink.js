// sing-box 分享链接生成（前端本地生成，参考 xray generateShareLink）
// 每个协议返回标准分享 URI；二维码在组件层用 generateQrDataUrl 生成。

function formatHost(host) {
  if (!host) return ''
  return host.includes(':') && !host.startsWith('[') ? '[' + host + ']' : host
}

function encodeParams(params) {
  return Object.entries(params)
    .filter(([, v]) => v !== '' && v !== undefined && v !== null)
    .map(([k, v]) => `${encodeURIComponent(k)}=${encodeURIComponent(v)}`)
    .join('&')
    .replace(/%20/g, '%20')
}

function getTransportParams(ib) {
  const p = {}
  const t = ib._transportType || ib.transport?.type || ''
  if (!t) return p
  p.type = t
  if (t === 'ws') { p.path = ib._transportPath || ib.transport?.path || '/'; p.host = ib._transportHost || ib.transport?.host || '' }
  else if (t === 'grpc') { p.serviceName = ib._transportServiceName || ib.transport?.service_name || '' }
  else if (t === 'http' || t === 'httpupgrade') { p.path = ib._transportPath || ib.transport?.path || '/'; p.host = ib._transportHost || ib.transport?.host || '' }
  return p
}

function getSecurityParams(ib) {
  const p = {}
  if (!ib._tlsEnabled && !ib.tls?.enabled) return p
  p.security = 'tls'
  p.sni = ib._tlsServerName || ib.tls?.server_name || ''
  if (ib.tls?.alpn?.length) p.alpn = ib.tls.alpn.join(',')
  // 客户端 uTLS 指纹：内部字段 → tls.utls（口径与后端 setSecurityParams 一致）
  const fp = ib._utlsFingerprint || ib.tls?.utls?.fingerprint || ''
  if (fp) p.fp = fp
  if (ib.tls?.insecure) p.insecure = '1'
  if (ib._realityEnabled || ib.tls?.reality?.enabled) {
    p.security = 'reality'
    p.pbk = ib._realityPublicKey || ib.tls?.reality?.public_key || ''
    // REALITY 客户端**强制要求 uTLS**（sing-box 报 "uTLS is required by reality client"），
    // 没有显式指纹时必须给一个，否则拿到的链接客户端直接用不了。
    if (!p.fp) p.fp = 'chrome'
    if (ib._realityShortIds) {
      const sids = ib._realityShortIds.split('\n').map(s => s.trim()).filter(Boolean)
      if (sids.length) p.sid = sids[Math.floor(Math.random() * sids.length)]
    }
    // usage from handshake server name sometimes differs; use server_name as sni
  }
  return p
}

// 兼容不同存储形态（传入裁剪后 form 或原生配置）
function pick(ib, key) { return ib[key] }

export function generateShareLink(ib, user, host, port, tag) {
  const type = ib.type
  const remark = tag || ib.tag || type + '-' + port
  const tp = getTransportParams(ib)
  const sec = getSecurityParams(ib)

  switch (type) {
    case 'vless': {
      const params = { encryption: 'none', ...tp, ...sec }
      if (user?.flow) params.flow = user.flow
      return `vless://${user?.uuid || ''}@${formatHost(host)}:${port}?${encodeParams(params)}#${encodeURIComponent(remark)}`
    }
    case 'vmess': {
      const obj = {
        v: '2', ps: remark, add: host, port, id: user?.uuid || '', aid: user?.alterId || 0,
        scy: 'auto', net: tp.type || '', ...tp, tls: sec.security === 'tls' ? 'tls' : '', sni: sec.sni || '', alpn: sec.alpn || '',
      }
      return 'vmess://' + btoa(JSON.stringify(obj))
    }
    case 'trojan': {
      const params = { ...tp, ...sec }
      if (user?.flow) params.flow = user.flow
      return `trojan://${encodeURIComponent(user?.password || '')}@${formatHost(host)}:${port}?${encodeParams(params)}#${encodeURIComponent(remark)}`
    }
    case 'shadowsocks':
    case 'shadowsocks2022': {
      const method = ib.method || 'aes-256-gcm'
      const password = ib.users?.[0]?.password || ib.password || ''
      const auth = btoa(method + ':' + password)
      return `ss://${auth}@${formatHost(host)}:${port}#${encodeURIComponent(remark)}`
    }
    case 'hysteria2': {
      // 与服务端 config 对齐：混淆、端口跳跃、BBR、带宽、ALPN、指纹、跳过证书校验
      // 都要带上（口径与后端 sharelink.generateHysteriaLink 一致）。
      // 注意 parseInbound 会把 sing-box 的 obfs 结构体搬到 _hy2ObfsType/_hy2ObfsPassword，
      // 只读 ib.obfs 会拿不到 → 服务端开了混淆时链接缺 obfs，客户端连不上。
      const obfsType = ib._hy2ObfsType || ib.obfs?.type || ''
      const obfsPwd = ib._hy2ObfsPassword || ib.obfs?.password || ''
      const bbr = ib._hy2BbrProfile || ib.bbr_profile || ''
      const ph = ib._portHop || {}
      let mport = ''
      if (ph.enabled) {
        mport = (ph.mode || 'preset') === 'manual'
          ? `${ph.start || ''}-${ph.end || ''}`
          : (ph.range || '')
      }
      // fm：端口跳跃写在 quicParams.udpHop、混淆写在 udp（后端同样下发，供 mihomo 互转）
      const fm = {}
      if (mport) fm.quicParams = { udpHop: { ports: mport } }
      if (obfsType && obfsPwd) fm.udp = [{ type: obfsType, settings: { password: obfsPwd } }]
      const params = {
        ...sec,
        sni: sec.sni || ib._tlsServerName || ib.tls?.server_name || host,
        ...(ib.up_mbps ? { upmbps: ib.up_mbps } : {}),
        ...(ib.down_mbps ? { downmbps: ib.down_mbps } : {}),
        ...(obfsType && obfsPwd ? { obfs: obfsType, 'obfs-password': obfsPwd } : {}),
        ...(bbr ? { 'bbr-profile': bbr } : {}),
        ...(mport ? { mport } : {}),
        ...(Object.keys(fm).length ? { fm: JSON.stringify(fm) } : {}),
      }
      if (params.security === 'reality') delete params.security
      params.security = params.security || 'tls'
      return `hysteria2://${encodeURIComponent(user?.password || '')}@${formatHost(host)}:${port}?${encodeParams(params)}#${encodeURIComponent(remark)}`
    }
    case 'hysteria': {
      const params = {
        sni: ib._tlsServerName || ib.tls?.server_name || host,
        insecure: (ib.tls?.insecure ? '1' : ''),
        ...(ib.obfs ? { obfs: ib.obfs } : {}),
      }
      return `hysteria://${encodeURIComponent(user?.auth_str || user?.password || '')}@${formatHost(host)}:${port}?${encodeParams(params)}#${encodeURIComponent(remark)}`
    }
    case 'tuic': {
      const userinfo = user?.uuid ? `${user.uuid}${user?.password ? ':' + user.password : ''}` : (user?.password || '')
      const params = {
        ...sec,
        congestion_control: ib.congestion_control || 'bbr',
        ...(ib.udp_relay_mode ? { udp_relay_mode: ib.udp_relay_mode } : {}),
      }
      return `tuic://${encodeURIComponent(userinfo)}@${formatHost(host)}:${port}?${encodeParams({ ...params, security: 'tls' })}#${encodeURIComponent(remark)}`
    }
    case 'http': {
      const ui = user?.username ? `${encodeURIComponent(user.username)}:${encodeURIComponent(user.password || '')}` : ''
      const params = { security: sec.security || 'none', ...(sec.sni ? { sni: sec.sni } : {}) }
      return `http://${ui}@${formatHost(host)}:${port}?${encodeParams(params)}#${encodeURIComponent(remark)}`
    }
    case 'socks':
    case 'mixed': {
      // mixed = socks + http 同端口，客户端按 socks 识别（与后端 generateAuthLink 一致）
      const ui = user?.username ? `${encodeURIComponent(user.username)}:${encodeURIComponent(user.password || '')}` : ''
      const params = { security: sec.security || 'none', ...(tp.type ? { type: tp.type } : {}) }
      return `socks://${ui}@${formatHost(host)}:${port}?${encodeParams(params)}#${encodeURIComponent(remark)}`
    }
    default:
      return ''
  }
}