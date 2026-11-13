package ratelimit

import (
	"os"
	"strings"
)

// EnvDeviceLimitGlobalBlock 是「恢复旧的全局阻断」的显式开关。
//
// 旧实现把超出设备数的 IP 加进 nft blocked_chain（`ip saddr <ip> drop`），
// 这是**节点全局**丢弃：一旦该 IP 是 NAT/代理共享出口，会把同一出口上
// 所有订阅的连接一起掐断。默认关闭，只记日志；`=global` 可显式恢复。
const EnvDeviceLimitGlobalBlock = "CF_DEVICE_LIMIT_BLOCK"

// DeviceLimitGlobalBlockEnabled 报告是否允许全局 drop 源 IP。
func DeviceLimitGlobalBlockEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(EnvDeviceLimitGlobalBlock)), "global")
}
