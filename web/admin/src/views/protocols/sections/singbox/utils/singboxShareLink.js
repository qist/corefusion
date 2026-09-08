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
  if (ib._realityEnabled || ib.tls?.reality?.enabled) {
    p.security = 'reality'
    p.pbk = ib._realityPublicKey || ib.tls?.reality?.public_key || ''
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
      const params = {
        sni: ib._tlsServerName || ib.tls?.server_name || host,
        insecure: (ib.tls?.insecure ? '1' : ''),
        ...(ib._transportHost ? { obfs: 'salamander', 'obfs-password': ib._transportHost } : {}),
      }
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
      const params = { congestion_control: ib.congestion_control || 'bbr', ...sec }
      return `tuic://${encodeURIComponent(userinfo)}@${formatHost(host)}:${port}?${encodeParams({ ...params, security: 'tls' })}#${encodeURIComponent(remark)}`
    }
    case 'http': {
      const ui = user?.username ? `${encodeURIComponent(user.username)}:${encodeURIComponent(user.password || '')}` : ''
      return `http://${ui}@${formatHost(host)}:${port}?security=${sec.security || 'none'}&sni=${encodeURIComponent(sec.sni || '')}#${encodeURIComponent(remark)}`
    }
    case 'socks':
    case 'mixed': {
      const scheme = type === 'socks' ? 'socks' : 'socks'
      const ui = user?.username ? `${encodeURIComponent(user.username)}:${encodeURIComponent(user.password || '')}` : ''
      return `${scheme}://${ui}@${formatHost(host)}:${port}?security=${sec.security || 'none'}&type=${tp.type || ''}#${encodeURIComponent(remark)}`
    }
    default:
      return ''
  }
}