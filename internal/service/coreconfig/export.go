package coreconfig

// ConvertXrayOutboundsToSingbox 把 xray 风格出站（protocol + settings + streamSettings）
// 转成 sing-box outbounds，供「CF 优选 IP/域名探活」等需要临时起 sing-box 拨号的场景复用。
// 转换逻辑与正式配置完全一致，避免探活走一套、线上走另一套。
func ConvertXrayOutboundsToSingbox(outbounds []interface{}) []interface{} {
	return convertChainOutboundsToSingbox(outbounds)
}
