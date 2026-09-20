// Server Names（Reality SNI）使用情况工具：xray 与 sing-box 编辑器共用同一口径。
//
// 口径：**同一节点内互斥、跨节点允许重复**（SNI 路由只在单机内需要区分），
// 与后端 inboundconfig.checkServerNamesConflict 保持一致。
// 另外「只拦新增」：存量重复（历史数据、镜像/批量导入等）不阻塞编辑，见 findNewConflicts。

// splitNames 把「一行一个 / 逗号分隔」的 SNI 文本切成小写去重数组。
export function splitNames(str) {
  const out = new Set()
  String(str || '')
    .split(/[\n,]/)
    .map(s => s.trim().toLowerCase())
    .filter(Boolean)
    .forEach(s => out.add(s))
  return [...out]
}

// snisOfInbound 取一个入站占用的 SNI，兼容两种核心的字段形态：
//   xray    : streamSettings.security === 'reality' → streamSettings.reality.serverNamesStr / serverNames
//   sing-box: _realityEnabled → _tlsServerName（顶层 tls.server_name 兜底）
export function snisOfInbound(ib) {
  if (!ib) return []
  const out = new Set()
  const ss = ib.streamSettings || {}
  if (ss.security === 'reality') {
    const r = ss.reality || {}
    splitNames(r.serverNamesStr).forEach(s => out.add(s))
    for (const s of r.serverNames || []) {
      const v = String(s || '').trim().toLowerCase()
      if (v) out.add(v)
    }
  }
  if (ib._realityEnabled || ib.tls?.reality?.enabled) {
    const v = String(ib._tlsServerName || ib.tls?.server_name || '').trim().toLowerCase()
    if (v) out.add(v)
  }
  return [...out]
}

// isSameInbound 判断两个入站是否为同一个（优先按 id，其次按引用）。
export function isSameInbound(a, b) {
  if (!a || !b) return false
  if (a === b) return true
  const ida = a._id ?? a.id
  const idb = b._id ?? b.id
  return ida != null && idb != null && String(ida) === String(idb)
}

// collectUsedSnis 返回 Map<sni, 占用方标签>：同一节点内「其它入站」已占用的 SNI。
// self 为当前正在编辑的入站（自身会被排除；传 null 则统计全部，用于列表标记）。
export function collectUsedSnis(inbounds, self) {
  const used = new Map()
  for (const ib of inbounds || []) {
    if (isSameInbound(ib, self)) continue
    const label = ib._remark || ib.remark || ib.tag || 'unknown'
    for (const sn of snisOfInbound(ib)) {
      if (!used.has(sn)) used.set(sn, label)
    }
  }
  return used
}

// findNewConflicts 返回 mySnis 里「本次新增、且已被其它入站占用」的 SNI。
// 存量重复（编辑前就已存在）不算冲突，避免历史数据导致改个备注都存不下去。
export function findNewConflicts(mySnis, oldSnis, usedByOthers) {
  const old = new Set(oldSnis || [])
  return (mySnis || []).filter(s => !old.has(s) && usedByOthers.has(s))
}

// mergeUsedSnis 把后端登记的已用 SNI 合并进本地 Map（用于跨核心标记）。
// items: [{ inbound_id, server_name, core_type, tag, remark }]
// selfId：当前编辑的入站 id，相同则跳过（不把自己算成「已占用」）。
export function mergeUsedSnis(map, items, selfId) {
  for (const it of items || []) {
    if (selfId != null && it.inbound_id != null && String(it.inbound_id) === String(selfId)) continue
    const sn = String(it.server_name || '').trim().toLowerCase()
    if (!sn || map.has(sn)) continue
    map.set(sn, it.remark || it.tag || it.core_type || 'unknown')
  }
  return map
}
