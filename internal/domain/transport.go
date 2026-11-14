package domain

import "strings"

// Xray 2026-07 把 streamSettings 的传输字段从 `network` 改名为 `method`，并且
// 新枚举使用全称：raw / xhttp / mkcp / grpc / websocket / httpupgrade / hysteria / masque。
// 旧写法（tcp/kcp/ws…）在新核心上仍作为别名接受，但只写 `method` 的新配置在旧代码里
// 读不到 network，所以读取时统一走下面的 helper。
//
// 内部（存储层 / 旧核心 / 前端表单）统一使用：raw、mkcp、ws、xhttp、grpc、httpupgrade、
// hysteria、masque。

// NormalizeTransport 把新旧传输名归一成内部使用的名字：
// tcp→raw、kcp→mkcp、websocket→ws、splithttp→xhttp。
func NormalizeTransport(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "tcp":
		return "raw"
	case "kcp":
		return "mkcp"
	case "websocket":
		return "ws"
	case "splithttp":
		return "xhttp"
	}
	return strings.ToLower(strings.TrimSpace(name))
}

// StreamTransport 读取 streamSettings 的传输方式（method 优先，其次 network），
// 返回归一化后的名字。两者都没有时返回空串。
func StreamTransport(ss map[string]interface{}) string {
	if ss == nil {
		return ""
	}
	if v, _ := ss["method"].(string); strings.TrimSpace(v) != "" {
		return NormalizeTransport(v)
	}
	v, _ := ss["network"].(string)
	return NormalizeTransport(v)
}

// TransportMethod 返回新核心 `method` 字段应写的新名字（ws→websocket）。
func TransportMethod(name string) string {
	switch NormalizeTransport(name) {
	case "ws":
		return "websocket"
	}
	return NormalizeTransport(name)
}
