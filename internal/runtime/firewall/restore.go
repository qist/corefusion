package firewall

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
)

// PortHopConfig holds the parsed port hopping configuration from an inbound's config_json.
type PortHopConfig struct {
	Enabled    bool
	PortRange  string
	TargetPort int
	Interface  string
}

// FirewallApplier is the interface for applying/removing firewall rules.
// Both the local panel and the Agent implement this.
type FirewallApplier interface {
	Apply(portRange string, targetPort int, iface string) error
	Remove(portRange string, targetPort int, iface string) error
}

// LocalApplier applies firewall rules directly via nftables/iptables.
// Used by the local panel.
type LocalApplier struct{}

func (LocalApplier) Apply(portRange string, targetPort int, iface string) error {
	useNft := isNftablesAvailable()
	exists, err := checkRuleExists(useNft, portRange, targetPort, iface)
	if err != nil {
		return fmt.Errorf("check rule: %w", err)
	}
	if exists {
		return nil // already exists, skip
	}
	return applyRule(useNft, portRange, targetPort, iface)
}

func (LocalApplier) Remove(portRange string, targetPort int, iface string) error {
	useNft := isNftablesAvailable()
	return removeRule(useNft, portRange, targetPort, iface)
}

// DynamicPortSpecs 汇总入站配置里客户端会访问的所有动态端口段（去重、规范成 "起-止"）。
// 兼容多种写法：
//   - 面板 _portHop（range / start-end）
//   - xray hysteriaSettings.hopPorts / udpHop.ports（"10000-19999,30000"）
//   - finalmask.quicParams.udpHop.ports（顶层或 streamSettings 内）
//   - sing-box server_ports（["20000:29999"]）以及直接写 ports 的旧配置
func DynamicPortSpecs(configJSON string, listenPort int) []string {
	cfg, err := decodeConfig(configJSON)
	if err != nil {
		return nil
	}
	var raw []string
	if ph := ParsePortHopFromConfig(configJSON, listenPort); ph != nil {
		raw = append(raw, ph.PortRange)
	}
	for _, path := range []string{"streamSettings.hysteriaSettings", "hysteriaSettings"} {
		if m := mapAt(cfg, path); m != nil {
			raw = append(raw, portValuesAt(m, "hopPorts")...)
			if hop, _ := m["udpHop"].(map[string]interface{}); hop != nil {
				raw = append(raw, portValuesAt(hop, "ports")...)
			}
		}
	}
	for _, path := range []string{"streamSettings.finalmask.quicParams", "finalmask.quicParams", "quicParams"} {
		if m := mapAt(cfg, path); m != nil {
			if hop, _ := m["udpHop"].(map[string]interface{}); hop != nil {
				raw = append(raw, portValuesAt(hop, "ports")...)
			}
		}
	}
	raw = append(raw, portValuesAt(cfg, "ports")...)
	raw = append(raw, portValuesAt(cfg, "server_ports")...)
	return normalizePortSpecs(raw)
}

// normalizePortSpecs 拆分多段字符串、校验并去重（非法项跳过，兼容模板变量）。
func normalizePortSpecs(raw []string) []string {
	seen := make(map[string]bool, len(raw))
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		for _, token := range splitPortSpecs(v) {
			lo, hi, ok := parsePortRangeToken(token)
			if !ok {
				continue
			}
			key := strconv.Itoa(lo)
			if hi != lo {
				key = fmt.Sprintf("%d-%d", lo, hi)
			}
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, key)
			if len(out) >= maxPortSpecs {
				return out
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// mapAt 按 "a.b.c" 取嵌套对象，缺失返回 nil。
func mapAt(root map[string]interface{}, path string) map[string]interface{} {
	cur := root
	for _, key := range strings.Split(path, ".") {
		next, _ := cur[key].(map[string]interface{})
		if next == nil {
			return nil
		}
		cur = next
	}
	return cur
}

// portValuesAt 读取 string / 数字 / 字符串数组形式的端口值。
func portValuesAt(m map[string]interface{}, key string) []string {
	switch v := m[key].(type) {
	case string:
		if strings.TrimSpace(v) != "" {
			return []string{v}
		}
	case float64:
		return []string{strconv.Itoa(int(v))}
	case int:
		return []string{strconv.Itoa(v)}
	case []string:
		return v
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			switch t := item.(type) {
			case string:
				if strings.TrimSpace(t) != "" {
					out = append(out, t)
				}
			case float64:
				out = append(out, strconv.Itoa(int(t)))
			}
		}
		return out
	}
	return nil
}

// ParsePortHopFromConfig parses an inbound config_json and extracts port hop settings.
// Returns nil if port hopping is not enabled.
func ParsePortHopFromConfig(configJSON string, port int) *PortHopConfig {
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return nil
	}

	portHopRaw, ok := cfg["_portHop"]
	if !ok {
		return nil
	}
	portHop, ok := portHopRaw.(map[string]interface{})
	if !ok {
		return nil
	}

	enabled, _ := portHop["enabled"].(bool)
	if !enabled {
		return nil
	}

	// Determine port range
	mode, _ := portHop["mode"].(string)
	rangeStr, _ := portHop["range"].(string)
	startPort, _ := portHop["start"].(float64)
	endPort, _ := portHop["end"].(float64)

	if mode == "manual" {
		rangeStr = fmt.Sprintf("%d-%d", int(startPort), int(endPort))
	}
	if rangeStr == "" {
		rangeStr = "20000-50000"
	}

	iface, _ := portHop["interface"].(string)

	targetPort := port
	if targetPort <= 0 {
		// Try to get port from config
		if p, ok := cfg["port"].(float64); ok {
			targetPort = int(p)
		}
	}
	if targetPort <= 0 {
		return nil
	}

	return &PortHopConfig{
		Enabled:    true,
		PortRange:  rangeStr,
		TargetPort: targetPort,
		Interface:  iface,
	}
}

// FlushAllRules removes the entire hysteria_porthopping nftables table (or flushes
// the iptables PREROUTING chain for port hop rules). This ensures stale rules from
// other nodes or previous configurations are cleaned up before restoring.
func FlushAllRules() {
	if isNftablesAvailable() {
		// Delete the entire table — it will be re-created by applyNft if needed.
		exec.Command("nft", "delete", "table", "inet", "hysteria_porthopping").Run()
		log.Printf("[Firewall] flushed nftables hysteria_porthopping table")
	} else {
		// For iptables, flush all REDIRECT rules in the nat PREROUTING chain.
		// We can't easily distinguish port-hop rules from other REDIRECT rules,
		// so we only flush if the hysteria_porthopping chain concept doesn't apply.
		// iptables mode is legacy; nftables is the primary path.
	}
}

// RestoreRules scans inbound configs and restores firewall rules for enabled port hopping.
// This should be called on startup (after process reconcile) to re-create rules lost on reboot.
//
// It first flushes all existing port-hop rules, then re-creates only the rules that
// belong to this node. This prevents stale rules from other nodes from persisting.
//
// configFetcher should return a slice of (configJSON, port) pairs for all enabled hysteria inbounds.
func RestoreRules(ctx context.Context, applier FirewallApplier, configFetcher func() ([]InboundConfigInfo, error)) {
	// Flush all existing rules first to remove any stale entries
	FlushAllRules()

	configs, err := configFetcher()
	if err != nil {
		log.Printf("[Firewall] failed to fetch inbound configs for rule restore: %v", err)
		return
	}

	// 去重 + 冲突跳过：多个入站（甚至 xray 与 sing-box 同时）可能配了相同/重叠的
	// 端口段，而 nft 前一条规则会吃掉后一条 —— 这里只保留第一条，其余跳过并告警，
	// 避免恢复出一堆互相覆盖的规则。
	applied := make([]RuleSpec, 0, len(configs))
	seen := make(map[string]bool, len(configs))
	for _, c := range configs {
		ph := ParsePortHopFromConfig(c.ConfigJSON, c.Port)
		if ph == nil {
			continue
		}
		spec := RuleSpec{PortRange: ph.PortRange, TargetPort: ph.TargetPort, Interface: ph.Interface}
		if seen[spec.Key()] {
			continue // 完全重复
		}
		if conflicts := FindConflicts(applied, spec); len(conflicts) > 0 {
			log.Printf("[Firewall] 跳过入站 %s 的端口跳跃规则：%s", c.Tag, FormatConflicts(conflicts, spec))
			continue
		}
		seen[spec.Key()] = true
		applied = append(applied, spec)

		if err := applier.Apply(ph.PortRange, ph.TargetPort, ph.Interface); err != nil {
			log.Printf("[Firewall] failed to restore rule for inbound %s (port %d, range %s): %v",
				c.Tag, ph.TargetPort, ph.PortRange, err)
		} else {
			log.Printf("[Firewall] restored port hop rule: inbound=%s range=%s → port=%d iface=%s",
				c.Tag, ph.PortRange, ph.TargetPort, ph.Interface)
		}
	}

	// 入站开启「自动创建防火墙放行规则」时，重启后也要把放行规则补回来
	// （firewalld 的 --permanent 规则本身会保留，ufw/nft/iptables 则可能丢失）。
	// 同一端口/端口段被多个入站共用时只处理一次：放行规则是节点级的，
	// 按入站重复调用既浪费时间，也容易在覆盖判断失效时留下重复规则。
	allowSeen := make(map[string]bool, len(configs))
	for _, c := range configs {
		if !c.Enabled {
			continue
		}
		allow, network := ParseAllowFromConfig(c.ConfigJSON)
		if !allow {
			continue
		}
		// Unix Socket 入站不监听 TCP 端口（port 只是订阅标识），不创建放行规则。
		if IsUnixSocketListen(c.Listen) {
			continue
		}
		port := c.Port
		if port <= 0 {
			if cfg, err := decodeConfig(c.ConfigJSON); err == nil {
				if p, ok := cfg["port"].(float64); ok {
					port = int(p)
				}
			}
		}
		if port <= 0 {
			continue
		}
		// 动态端口：hy2/hysteria 客户端访问的是端口段（面板 _portHop 的 DNAT，或核心
		// 原生 hopPorts / server_ports / ports 多段写法）。监听端口与所有端口段都要放行；
		// QUIC 只走 UDP。
		specs := DynamicPortSpecs(c.ConfigJSON, port)
		if len(specs) > 0 {
			network = "udp"
		}
		if key := allowKey(port, network); !allowSeen[key] {
			allowSeen[key] = true
			if err := AllowPort(port, network); err != nil {
				log.Printf("[Firewall] failed to restore allow rule for inbound %s (port %d): %v", c.Tag, port, err)
			} else {
				log.Printf("[Firewall] restored allow rule: inbound=%s port=%d network=%s", c.Tag, port, network)
			}
		}
		for _, spec := range specs {
			if key := "range:" + spec + "|udp"; !allowSeen[key] {
				allowSeen[key] = true
				if err := AllowPortRange(spec, "udp"); err != nil {
					log.Printf("[Firewall] failed to restore allow range for inbound %s (%s): %v", c.Tag, spec, err)
				} else {
					log.Printf("[Firewall] restored allow range: inbound=%s range=%s network=udp", c.Tag, spec)
				}
			}
		}
	}
}

// allowKey 生成放行去重键（端口 + 协议）：空 network 会被 normalizeProtos 展开成 tcp+udp。
func allowKey(port int, network string) string {
	return fmt.Sprintf("port:%d|%s", port, strings.TrimSpace(strings.ToLower(network)))
}

// InboundConfigInfo holds the minimal info needed from an inbound config for firewall restoration.
type InboundConfigInfo struct {
	Tag        string
	Protocol   string
	Listen     string
	Port       int
	ConfigJSON string
	Enabled    bool
}

// ============================================================
// nftables/iptables helpers (shared implementation)
// ============================================================

func isNftablesAvailable() bool {
	_, err := exec.LookPath("nft")
	return err == nil
}

func checkRuleExists(useNft bool, portRange string, targetPort int, iface string) (bool, error) {
	if useNft {
		return checkNftRule(portRange, targetPort, iface)
	}
	return checkIptablesRule(portRange, targetPort, iface)
}

// nftRuleMatches checks if a rule line from `nft -a list chain` output matches
// the given port range, target port, and interface.
//
// nft actual output format:
//
//	iifname "eth0" udp dport 40000-50000 counter packets 0 bytes 0 redirect to :51071 handle 42
//	udp dport 20000-30000 counter packets 0 bytes 0 redirect to :51072 handle 43
//
// We match by key fields (not full string) because counter stats vary dynamically.
func nftRuleMatches(line, portRange string, targetPort int, iface string) bool {
	// Must be a rule line with redirect
	if !strings.Contains(line, "redirect to") {
		return false
	}
	// Must contain the exact port range
	if !strings.Contains(line, fmt.Sprintf("dport %s ", portRange)) {
		return false
	}
	// Must contain the exact redirect target
	if !strings.Contains(line, fmt.Sprintf("redirect to :%d", targetPort)) {
		return false
	}
	// Check interface: nft outputs iifname with quotes, e.g. iifname "eth0"
	if iface != "" {
		if !strings.Contains(line, fmt.Sprintf(`iifname "%s"`, iface)) {
			return false
		}
	} else {
		// No interface specified — rule should not have iifname
		if strings.Contains(line, "iifname ") {
			return false
		}
	}
	return true
}

// findNftRuleHandle lists the chain with handles and finds the handle of the
// rule matching the given spec. Returns 0 if not found.
func findNftRuleHandle(portRange string, targetPort int, iface string) (uint64, error) {
	cmd := exec.Command("nft", "-a", "list", "chain", "inet", "hysteria_porthopping", "prerouting")
	out, err := cmd.Output()
	if err != nil {
		return 0, nil // chain doesn't exist or other error → rule doesn't exist
	}

	for _, line := range strings.Split(string(out), "\n") {
		if !nftRuleMatches(line, portRange, targetPort, iface) {
			continue
		}
		// Extract handle number from end of line
		handleIdx := strings.LastIndex(line, "handle ")
		if handleIdx < 0 {
			continue
		}
		handleStr := strings.TrimSpace(line[handleIdx+7:])
		handle, err := strconv.ParseUint(handleStr, 10, 64)
		if err != nil {
			continue
		}
		return handle, nil
	}
	return 0, nil
}

func checkNftRule(portRange string, targetPort int, iface string) (bool, error) {
	cmd := exec.Command("nft", "list", "table", "inet", "hysteria_porthopping")
	if err := cmd.Run(); err != nil {
		return false, nil
	}
	handle, err := findNftRuleHandle(portRange, targetPort, iface)
	if err != nil {
		return false, err
	}
	return handle > 0, nil
}

func checkIptablesRule(portRange string, targetPort int, iface string) (bool, error) {
	cmd := exec.Command("iptables", "-t", "nat", "-L", "PREROUTING", "-n")
	out, err := cmd.Output()
	if err != nil {
		return false, nil
	}
	rules := string(out)
	if strings.Contains(rules, portRange) && strings.Contains(rules, fmt.Sprintf("to:%d", targetPort)) {
		return true, nil
	}
	return false, nil
}

func applyRule(useNft bool, portRange string, targetPort int, iface string) error {
	if useNft {
		return applyNft(portRange, targetPort, iface)
	}
	return applyIptables(portRange, targetPort, iface)
}

func applyNft(portRange string, targetPort int, iface string) error {
	exec.Command("nft", "add", "table", "inet", "hysteria_porthopping").Run()
	exec.Command("nft", "add", "chain", "inet", "hysteria_porthopping", "prerouting",
		"{ type nat hook prerouting priority dstnat; policy accept; }").Run()
	rule := fmt.Sprintf("udp dport %s counter redirect to :%d", portRange, targetPort)
	if iface != "" {
		rule = fmt.Sprintf("iifname %s %s", iface, rule)
	}
	// 幂等：RestoreRules 重启时会先 flush，但 Apply API / 重复调用没有 flush，
	// 同一条 DNAT 规则已经存在时直接跳过，避免启动一次多一条。
	if nftPortHopRuleExists(portRange, targetPort, iface) {
		return nil
	}
	return exec.Command("nft", "add", "rule", "inet", "hysteria_porthopping", "prerouting", rule).Run()
}

// nftPortHopRuleExists 判断 hysteria_porthopping 表里是否已有同目标的 DNAT 规则。
func nftPortHopRuleExists(portRange string, targetPort int, iface string) bool {
	out, err := exec.Command("nft", "-a", "list", "table", "inet", "hysteria_porthopping").Output()
	if err != nil {
		return false
	}
	wantDport := "udp dport " + portRange
	wantTarget := fmt.Sprintf("redirect to :%d", targetPort)
	for _, line := range strings.Split(string(out), "\n") {
		t := strings.TrimSpace(line)
		if !strings.Contains(t, wantDport) || !strings.Contains(t, wantTarget) {
			continue
		}
		if iface != "" && !strings.Contains(t, iface) {
			continue
		}
		return true
	}
	return false
}

func applyIptables(portRange string, targetPort int, iface string) error {
	spec := []string{"-t", "nat"}
	if iface != "" {
		spec = append(spec, "-i", iface)
	}
	spec = append(spec, "-p", "udp", "--dport", portRange, "-j", "REDIRECT", "--to-ports", fmt.Sprintf("%d", targetPort))

	// 幂等：先 -C 查同一规则是否已存在（老版本没有 flush 动作，重启会重复 -A）。
	checkArgs := append([]string{"-t", "nat", "-C", "PREROUTING"}, spec[2:]...)
	if err := exec.Command("iptables", checkArgs...).Run(); err == nil {
		return nil
	}
	addArgs := append([]string{"-t", "nat", "-A", "PREROUTING"}, spec[2:]...)
	return exec.Command("iptables", addArgs...).Run()
}

func removeRule(useNft bool, portRange string, targetPort int, iface string) error {
	if useNft {
		return removeNft(portRange, targetPort, iface)
	}
	return removeIptables(portRange, targetPort, iface)
}

func removeNft(portRange string, targetPort int, iface string) error {
	handle, err := findNftRuleHandle(portRange, targetPort, iface)
	if err != nil {
		return fmt.Errorf("find rule handle: %w", err)
	}
	if handle == 0 {
		return nil // rule doesn't exist, nothing to remove
	}
	return exec.Command("nft", "delete", "rule", "inet", "hysteria_porthopping", "prerouting", "handle", strconv.FormatUint(handle, 10)).Run()
}

func removeIptables(portRange string, targetPort int, iface string) error {
	args := []string{"-t", "nat", "-D", "PREROUTING"}
	if iface != "" {
		args = append(args, "-i", iface)
	}
	args = append(args, "-p", "udp", "--dport", portRange, "-j", "REDIRECT", "--to-ports", fmt.Sprintf("%d", targetPort))
	return exec.Command("iptables", args...).Run()
}
