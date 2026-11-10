package coredata

// 链接 xray-core 的全部协议、传输与应用（含 masque）。
//
// 面板和 Agent 都会在进程内用 infra/conf 的 Build() 把 JSON 配置转成 xray 结构，
// 再通过 gRPC 热下发（xraycontrol 的出站/路由、Agent 的 addOutbound/applyRouting）。
// 这些 Build 依赖各协议与传输在 init() 里注册：只 import 少数 account 包时，
// masque、ws、grpc 这类出站会报 "unknown protocol/transport"。
// main/distro/all 是 xray 官方的全量注册入口（含 proxy/masque 与
// transport/internet/masque，即 RFC 9484 CONNECT-IP）。
import _ "github.com/xtls/xray-core/main/distro/all"
