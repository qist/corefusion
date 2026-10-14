// 节点「名称 / 标签」生成器 —— xray 与 sing-box 共用的**唯一**实现。
//
// 之前两边各维护一套（xray 的 xrayInboundUtils + XrayInbounds/XrayOutbounds 内联，
// sing-box 的 singboxDefaults / singboxEndpointBuild 内联），改一处忘一处就会
// 出现「节点名称 / 标签」口径不一致。统一收敛到这里，两侧都从这里 import。

// 短随机后缀（小写字母 + 数字）：与历史实现一致，等价于
// Math.random().toString(36).substring(2, 2 + len)。
export function randSuffix(len = 6) {
  return Math.random().toString(36).substring(2, 2 + len)
}

// 节点名称（界面「节点名称」/ remark）：默认 Node-<6 位随机>
export function genRemark() {
  return 'Node-' + randSuffix(6)
}

// 入站标签（tag）：inbound-<协议>-<端口>
export function genInboundTag(protocol, port) {
  return 'inbound-' + protocol + '-' + port
}

// 出站标签（tag）：outbound-<协议>-<6 位随机>
export function genOutboundTag(protocol) {
  return 'outbound-' + protocol + '-' + randSuffix(6)
}
