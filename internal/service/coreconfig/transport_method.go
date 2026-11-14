package coreconfig

import (
	"context"
	"regexp"
	"strconv"
	"strings"

	"corefusion/internal/domain"
)

// 背景：xray 把 streamSettings 的传输字段从 `network` 改名为 `method`
// （新枚举用全称：raw / xhttp / mkcp / grpc / websocket / httpupgrade / hysteria / masque）。
//
// 版本分界：v26.6.27 及以前只认 `network`；v26.7.11 起新增 `method`
// （method 存在时会覆盖 network）。给旧核心写 `method` 不会报错，但会被**静默忽略**，
// 于是 ws/grpc 节点会退化成裸 TCP —— 所以不能只写 method，也不能一直双写。
//
// 现在的策略：按目标核心版本只写一个字段
//   - 版本 >= 26.7.11：只写 `method`（新名）
//   - 版本更旧或未知：只写 `network`（所有版本都认）
//
// 目标核心版本由 CoreVersionProvider 提供（面板本地读二进制、远程问 Agent），
// 拿不到时按旧版本处理，保证不会出现"配置能加载但传输退化"的情况。
// 存储层（inbound/outbound config_json、前端表单）继续用短名，只在最终生成的运行配置里归一。

// transportMethodMinVersion 是 xray 开始支持 streamSettings.method 的版本。
var transportMethodMinVersion = [3]int{26, 7, 11}

var coreVersionPattern = regexp.MustCompile(`(\d+)\.(\d+)\.(\d+)`)

// CoreVersionProvider 返回指定服务器上某核心的实际版本字符串
// （例如 "Xray 26.7.11 ..." 或 "v26.7.11"）。拿不到返回空串。
type CoreVersionProvider func(ctx context.Context, serverID int64, coreType string) string

var coreVersionProvider CoreVersionProvider

// SetCoreVersionProvider 注入核心版本查询（面板/Agent 启动时各挂一次）。
func SetCoreVersionProvider(p CoreVersionProvider) {
	coreVersionProvider = p
}

// resolveCoreVersion 按 options 里的显式版本 → provider → 空串 的顺序取版本。
func resolveCoreVersion(opts PrepareFullConfigOptions) string {
	if v := strings.TrimSpace(opts.CoreVersion); v != "" {
		return v
	}
	if coreVersionProvider == nil {
		return ""
	}
	serverID := opts.OutboundServerID
	if serverID == 0 {
		serverID = opts.ServerID
	}
	return coreVersionProvider(opts.Ctx, serverID, opts.CoreType)
}

// SupportsTransportMethod 判断某个 xray 版本是否支持 `streamSettings.method`。
// 解析不出数字（空串 / "unknown"）时返回 false，按旧核心处理。
func SupportsTransportMethod(version string) bool {
	major, minor, patch, ok := parseCoreVersion(version)
	if !ok {
		return false
	}
	want := transportMethodMinVersion
	if major != want[0] {
		return major > want[0]
	}
	if minor != want[1] {
		return minor > want[1]
	}
	return patch >= want[2]
}

func parseCoreVersion(version string) (major, minor, patch int, ok bool) {
	m := coreVersionPattern.FindStringSubmatch(version)
	if len(m) != 4 {
		return 0, 0, 0, false
	}
	var err error
	if major, err = strconv.Atoi(m[1]); err != nil {
		return 0, 0, 0, false
	}
	if minor, err = strconv.Atoi(m[2]); err != nil {
		return 0, 0, 0, false
	}
	if patch, err = strconv.Atoi(m[3]); err != nil {
		return 0, 0, 0, false
	}
	return major, minor, patch, true
}

// ApplyTransportMethodAlias 按核心版本把传输字段归一到唯一一种写法：
// 新核心只写 method，旧核心/未知版本只写 network。
func ApplyTransportMethodAlias(fullConfig map[string]interface{}, coreVersion string) {
	if fullConfig == nil {
		return
	}
	useMethod := SupportsTransportMethod(coreVersion)
	for _, section := range []string{"inbounds", "outbounds"} {
		items, ok := fullConfig[section].([]interface{})
		if !ok {
			continue
		}
		for _, raw := range items {
			item, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			ss, ok := item["streamSettings"].(map[string]interface{})
			if !ok {
				continue
			}
			canonical := domain.StreamTransport(ss)
			if canonical == "" {
				continue
			}
			if useMethod {
				ss["method"] = domain.TransportMethod(canonical)
				delete(ss, "network")
				continue
			}
			ss["network"] = canonical
			delete(ss, "method")
		}
	}
}
