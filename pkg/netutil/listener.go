package netutil

import (
	"context"
	"net"
)

// ListenTCP 创建显式禁用 MPTCP（Multipath TCP）的 TCP 监听。
//
// CVE-2026-98432（Linux 内核 IPv4 TCP 握手 Use-After-Free，CVSS 9.8）：公开利用链
// 针对创建了 MPTCP 监听套接字的服务。面板所有监听端口（面板 / 订阅 / 门户 HTTP、
// Agent gRPC、初始化向导）统一走本函数并显式 SetMultipathTCP(false)：
// Go 当前默认不启用 MPTCP，但显式关闭可避免未来 Go 默认行为变化或
// GODEBUG=multipathtcp 环境变量把监听端口暴露进 MPTCP 攻击面。
func ListenTCP(addr string) (net.Listener, error) {
	var lc net.ListenConfig
	lc.SetMultipathTCP(false)
	return lc.Listen(context.Background(), "tcp", addr)
}
