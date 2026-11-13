// 流量展示格式化（xray / sing-box 面板共用）。
// 后端在入站配置响应的用户对象里注回 _usedUp/_usedDown（字节），
// 编辑器据此展示已用流量；统一格式化避免各页面精度/单位不一致。

export function formatTraffic(bytes) {
  const b = Number(bytes) || 0
  if (b <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  let i = 0
  let v = b
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return (i === 0 ? v.toFixed(0) : v.toFixed(2)) + ' ' + units[i]
}

export function trafficTotal(up, down) {
  return (Number(up) || 0) + (Number(down) || 0)
}
