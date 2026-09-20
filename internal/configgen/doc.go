// Package configgen 是 sing-box / xray / hysteria2 / tuic / mihomo 的「配置文件生成器」
// 设计稿实现（见 docs/design/singbox-configgen-design.md）。
//
// ⚠️ 本包当前处于「未接入主链路」状态（dead code）：
//
//	全仓库没有任何地方 import corefusion/internal/configgen，线上的配置生成走的是
//	另一条路径 —— core_configs 分段表 + internal/service/coreconfig.PrepareFullConfig
//	（见 docs/design/singbox-configgen-design.md §0「实现现状说明」）。
//
// 维护须知：
//
//  1. 修改 sing-box 的配置语义请改 internal/service/coreconfig/prepare.go，
//     不要改这里，否则线上行为不变，容易造成「改了没生效」的误判。
//  2. 若计划把本包接回主链路，请先通读上面文档 §0 与 §16.7（会 FATAL 的字段清单），
//     并按 §16.7 的方式用真实二进制 `sing-box check -c` 校验产物。
//  3. 如需删除本包，请同时更新设计文档（§0 / §12 / §13 已标注为 historical）。
package configgen
