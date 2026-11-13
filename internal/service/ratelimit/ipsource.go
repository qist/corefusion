package ratelimitsvc

import (
	"os"
	"strings"
)

// EnvIPSource 选择「限速/设备数的 IP 来源」。
//
//	core   （默认）只使用核心观测到的连接源 IP（xray GetUsersStats / ONLINE_USERS）
//	legacy        保留旧的「订阅/登录 HTTP 请求 IP」推送，用于紧急回滚
//
// 旧行为的问题：订阅域名可能走代理/加速器，HTTP 层 IP 并不是客户端连到
// 节点时的源 IP，限速会落在代理出口上；设备超限甚至会全局 drop 该出口。
const EnvIPSource = "CF_RATELIMIT_IP_SOURCE"

// LegacyIPSource 报告是否仍启用旧的 HTTP IP 关联路径。
func LegacyIPSource() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(EnvIPSource)), "legacy")
}
