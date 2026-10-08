package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	agentpb "corefusion/internal/runtime/agent/proto"
)

// ============================================================
// SyncData 的热应用（reload_strategy=hot_reload）
//
// 目标：镜像/远程节点的行为与主节点一致 —— **支持热下发的改动绝不重启**。
// 主节点的做法是「面板写盘 + 前端调 xray/control/* 热下发」；节点侧没有面板那套
// 编排，所以这里直接对比「上一次的运行配置」与「本次要写入的新配置」，把差异
// 翻译成 xray proxyman API 调用（本地经 LocalAgent.XrayAPIProxy，与面板远程调
// 用走的是同一套实现）。
//
// 能力边界（决定「什么时候必须重启」）：
//   - inbounds 的增 / 删 / 改（含端口、传输层、Reality 参数、用户）→ 可热下发：
//     删用 RemoveInbound、增用 AddInbound、改用「先删后加」（同 tag 不允许共存，
//     先删还能释放旧端口）；
//   - 其余顶层段（routing / dns / outbounds / api / log…）→ **API 不支持整体替换**，
//     回退重启，保证配置最终生效。
// 任何一步热下发失败同样回退重启，避免「半套配置」的运行态。
// ============================================================

// inboundItem 是一个带 tag 的入站配置：Raw 用于下发、Val 用于深度比较。
type inboundItem struct {
	Tag string
	Raw json.RawMessage
	Val interface{}
}

// runtimeConfigDiff 是两份运行配置之间的差异。
type runtimeConfigDiff struct {
	Removals  []string      // 需 RemoveInbound 的 tag
	Additions []inboundItem // 需 AddInbound 的入站
	Alters    []inboundItem // 需「先删后加」的入站
	// Unsupported=true 表示差异落在 proxyman API 覆盖不到的部分 → 必须重启。
	Unsupported bool
	// OutsideReason 说明不被支持的原因（Unsupported=false 时为空）。
	OutsideReason string
}

// empty 报告是否没有任何可热应用的差异。
func (d runtimeConfigDiff) empty() bool {
	return !d.Unsupported && len(d.Removals) == 0 && len(d.Additions) == 0 && len(d.Alters) == 0
}

// diffRuntimeConfig 对比两份完整 xray 配置，只把「inbounds」的差异识别为可热应用。
//
// 比较按**语义**进行（解析成 interface{} 后 DeepEqual），避免键序/空白差异造成误判。
func diffRuntimeConfig(oldCfg, newCfg []byte) (runtimeConfigDiff, error) {
	var diff runtimeConfigDiff

	var oldTop, newTop map[string]json.RawMessage
	if err := json.Unmarshal(newCfg, &newTop); err != nil {
		return diff, fmt.Errorf("parse new config: %w", err)
	}
	if len(oldCfg) == 0 {
		// 没有旧配置（首次写入）→ 无法 diff，交给调用方按「重启」处理
		diff.Unsupported = true
		diff.OutsideReason = "no previous runtime config to diff"
		return diff, nil
	}
	if err := json.Unmarshal(oldCfg, &oldTop); err != nil {
		return diff, fmt.Errorf("parse old config: %w", err)
	}

	// 1) inbounds 以外的任何段变化 → 不支持热应用
	for key, newVal := range newTop {
		if key == "inbounds" {
			continue
		}
		oldVal, exists := oldTop[key]
		if !exists || !jsonEqual(oldVal, newVal) {
			diff.Unsupported = true
			diff.OutsideReason = fmt.Sprintf("section %q changed (hot reload only covers inbounds)", key)
			return diff, nil
		}
	}
	for key := range oldTop {
		if key == "inbounds" {
			continue
		}
		if _, exists := newTop[key]; !exists {
			diff.Unsupported = true
			diff.OutsideReason = fmt.Sprintf("section %q removed", key)
			return diff, nil
		}
	}

	// 2) inbounds 差异（按 tag 匹配）
	oldItems, err := parseInbounds(oldTop["inbounds"])
	if err != nil {
		return diff, fmt.Errorf("parse old inbounds: %w", err)
	}
	newItems, err := parseInbounds(newTop["inbounds"])
	if err != nil {
		return diff, fmt.Errorf("parse new inbounds: %w", err)
	}

	newByTag := make(map[string]inboundItem, len(newItems))
	for _, it := range newItems {
		newByTag[it.Tag] = it
	}
	oldByTag := make(map[string]inboundItem, len(oldItems))
	for _, it := range oldItems {
		oldByTag[it.Tag] = it
	}

	for _, it := range oldItems {
		n, ok := newByTag[it.Tag]
		if !ok {
			diff.Removals = append(diff.Removals, it.Tag)
			continue
		}
		if !reflect.DeepEqual(it.Val, n.Val) {
			diff.Alters = append(diff.Alters, n)
		}
	}
	for _, it := range newItems {
		if _, ok := oldByTag[it.Tag]; !ok {
			diff.Additions = append(diff.Additions, it)
		}
	}
	return diff, nil
}

// parseInbounds 解析 inbounds 数组，逐个取出 tag（缺失 tag 的条目会被跳过，
// 它们无法通过 proxyman API 定位，交由整体重启兜底更稳妥）。
func parseInbounds(raw json.RawMessage) ([]inboundItem, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil, err
	}
	items := make([]inboundItem, 0, len(arr))
	for _, elem := range arr {
		var head struct {
			Tag string `json:"tag"`
		}
		if err := json.Unmarshal(elem, &head); err != nil {
			return nil, err
		}
		if strings.TrimSpace(head.Tag) == "" {
			continue
		}
		var val interface{}
		if err := json.Unmarshal(elem, &val); err != nil {
			return nil, err
		}
		items = append(items, inboundItem{Tag: head.Tag, Raw: elem, Val: val})
	}
	return items, nil
}

// jsonEqual 语义比较两段 JSON（键序与空白不敏感）。
func jsonEqual(a, b json.RawMessage) bool {
	var x, y interface{}
	if err := json.Unmarshal(a, &x); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &y); err != nil {
		return false
	}
	return reflect.DeepEqual(x, y)
}

// ============================================================
// 运行态基准（applied basis）
//
// 为什么不能拿「磁盘上的 runtime 文件」当 diff 基准：面板对镜像节点会**同时**下发
// PanelCommand.SyncConfig（LocalAgent.SyncConfig 直接写盘）与 SyncData，两者竞态 ——
// 实测 SyncConfig 抢先落盘，SyncData 再读「旧配置」时文件已经是新内容，diff 永远判
// 「未变」→ 变更只更新了文件、从没应用到运行中的进程（已用真实节点复现）。
//
// 所以基准记录在**独立文件**里，只在「配置真正应用到进程」时更新：
//   - 热应用成功 / 判定无需变更 → 基准 = 本次配置
//   - 启动 / 重启成功（进程加载的就是这份配置）→ 基准 = 本次配置
//   - SyncConfig 只写盘、不动进程 → 不更新基准（运行态没变）
// ============================================================

// appliedBasisPath 返回运行态基准文件路径（与 runtime 配置同目录）。
func (a *AgentApp) appliedBasisPath(coreType string) string {
	rt := a.procMgr.RuntimeConfigPath(coreType)
	return strings.TrimSuffix(rt, filepath.Ext(rt)) + ".applied"
}

// readAppliedBasis 读取「上次应用到运行态的配置」。
// 基准文件不存在（首次升级 / 历史数据）时回退 fallback（写盘前读到的磁盘旧配置）；
// 此时若文件已被 SyncConfig 抢先更新，会退化为「判定未变」，但下一次真实变更即恢复
// （保守方向：宁可漏一次热更、下次补上，也不能错误地重复下发）。
func (a *AgentApp) readAppliedBasis(coreType string, fallback []byte) []byte {
	data, err := os.ReadFile(a.appliedBasisPath(coreType))
	if err != nil || len(data) == 0 {
		return fallback
	}
	return data
}

// writeAppliedBasis 记录「运行态已等于这份配置」。
func (a *AgentApp) writeAppliedBasis(coreType string, data []byte) {
	if len(data) == 0 {
		return
	}
	path := a.appliedBasisPath(coreType)
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		log.Printf("[HotReload] mkdir basis dir failed: %v", err)
		return
	}
	if err := os.WriteFile(path, data, 0640); err != nil {
		log.Printf("[HotReload] write applied basis failed: %v", err)
	}
}

// xrayProxymanCall 通过本地 XrayAPIProxy 发起一次 proxyman 调用
// （与面板对远程节点下发走的是同一实现）。
func (a *AgentApp) xrayProxymanCall(ctx context.Context, method string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	resp, err := a.localAgent.XrayAPIProxy(ctx, &agentpb.XrayAPIRequest{
		Service: "proxyman",
		Method:  method,
		Payload: body,
	})
	if err != nil {
		return err
	}
	if resp == nil || !resp.GetSuccess() {
		if resp != nil {
			return fmt.Errorf("%s", resp.GetError())
		}
		return fmt.Errorf("%s returned no response", method)
	}
	return nil
}

// tryHotApplyXray 把「入站差异」热下发给运行中的 xray。
// 返回 (是否已成功热应用, 说明)：false 表示调用方应改用重启兜底。
func (a *AgentApp) tryHotApplyXray(ctx context.Context, oldCfg, newCfg []byte) (bool, string) {
	diff, err := diffRuntimeConfig(oldCfg, newCfg)
	if err != nil {
		return false, "diff failed: " + err.Error()
	}
	if diff.Unsupported {
		return false, diff.OutsideReason
	}
	if diff.empty() {
		return true, "config unchanged"
	}

	// 先删（释放 tag 与端口）→ 再改（先删后加）→ 最后加
	for _, tag := range diff.Removals {
		if err := a.xrayProxymanCall(ctx, "removeInbound", map[string]string{"tag": tag}); err != nil {
			return false, fmt.Sprintf("remove inbound %q: %v", tag, err)
		}
	}
	for _, it := range diff.Alters {
		if err := a.xrayProxymanCall(ctx, "removeInbound", map[string]string{"tag": it.Tag}); err != nil {
			return false, fmt.Sprintf("replace inbound %q (remove): %v", it.Tag, err)
		}
		if err := a.xrayProxymanCall(ctx, "addInbound", map[string]json.RawMessage{"config": it.Raw}); err != nil {
			return false, fmt.Sprintf("replace inbound %q (add): %v", it.Tag, err)
		}
	}
	for _, it := range diff.Additions {
		if err := a.xrayProxymanCall(ctx, "addInbound", map[string]json.RawMessage{"config": it.Raw}); err != nil {
			return false, fmt.Sprintf("add inbound %q: %v", it.Tag, err)
		}
	}

	summary := fmt.Sprintf("hot-applied inbound changes: -%d ~%d +%d",
		len(diff.Removals), len(diff.Alters), len(diff.Additions))
	log.Printf("[HotReload] %s", summary)
	return true, summary
}
