// 端口跳跃（端口段 → 单端口 DNAT）预设段与冲突判断工具。
//
// **唯一定义处**：xray 与 sing-box 编辑器都从这里取预设段，避免两边各写一份导致
// 「定义不一致」（用户反馈：一边显示旧段、一边显示新段，看着像两套定义）。
//
// 冲突口径与后端 internal/runtime/firewall 完全一致：
//   区间重叠 且 目标端口不同 → 冲突（同一端口段只能指向一个入站端口，否则后加的规则
//   不生效、入站静默不可达）；同段同目标不同网卡可以共存，同段同目标同网卡算重复。

// 预设段：互不重叠（每段 10000 宽），方便同一节点给多个入站分配不同段。
export const PORT_HOP_PRESETS = ['10000-19999', '20000-29999', '30000-39999', '40000-49999', '50000-59999']

// parseRange "20000-29999" → [20000, 29999]；单端口 "443" → [443, 443]；非法 → null
export function parseRange(s) {
  const str = String(s || '').trim()
  if (!str) return null
  const i = str.indexOf('-')
  if (i > 0) {
    const lo = parseInt(str.slice(0, i).trim(), 10)
    const hi = parseInt(str.slice(i + 1).trim(), 10)
    if (Number.isNaN(lo) || Number.isNaN(hi) || lo <= 0 || hi < lo || hi > 65535) return null
    return [lo, hi]
  }
  const p = parseInt(str, 10)
  if (Number.isNaN(p) || p <= 0 || p > 65535) return null
  return [p, p]
}

// rangesOverlap 两个端口段/端口是否重叠（含端点）。
export function rangesOverlap(a, b) {
  const ra = parseRange(a)
  const rb = parseRange(b)
  if (!ra || !rb) return false
  return ra[0] <= rb[1] && rb[0] <= ra[1]
}

// findConflict 在既有规则里找出一条与 want 冲突的规则（返回该规则对象或 null）。
//   rules: [{ inbound_id, port_range, target_port, interface, core_type, tag, remark }]
//   want:  { portRange, targetPort }
//   inboundId: 当前编辑的入站 id（自身规则不算冲突）
export function findConflict(rules, want, inboundId) {
  const wantRange = want?.portRange
  const wantTarget = Number(want?.targetPort) || 0
  if (!parseRange(wantRange)) return null
  for (const r of rules || []) {
    if (inboundId != null && r.inbound_id != null && String(r.inbound_id) === String(inboundId)) continue
    if (Number(r.target_port) === wantTarget && wantTarget > 0) continue // 同目标：重复而非冲突
    if (rangesOverlap(r.port_range, wantRange)) return r
  }
  return null
}

// suggestFreeRange 在 10000-60000 内按 width 切块，返回第一个与既有规则完全不重叠的
// 空闲段（找不到返回空串）。与后端 firewall.SuggestFreeRange 行为一致。
export function suggestFreeRange(rules, width = 10000) {
  const w = width > 0 ? width : 10000
  for (let start = 10000; start + w - 1 <= 60000; start += w) {
    const cand = `${start}-${start + w - 1}`
    const busy = (rules || []).some(r => rangesOverlap(r.port_range, cand))
    if (!busy) return cand
  }
  return ''
}
