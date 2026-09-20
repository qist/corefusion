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
	Enabled   bool
	PortRange string
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
}

// InboundConfigInfo holds the minimal info needed from an inbound config for firewall restoration.
type InboundConfigInfo struct {
	Tag        string
	Protocol   string
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
//   iifname "eth0" udp dport 40000-50000 counter packets 0 bytes 0 redirect to :51071 handle 42
//   udp dport 20000-30000 counter packets 0 bytes 0 redirect to :51072 handle 43
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
	return exec.Command("nft", "add", "rule", "inet", "hysteria_porthopping", "prerouting", rule).Run()
}

func applyIptables(portRange string, targetPort int, iface string) error {
	args := []string{"-t", "nat", "-A", "PREROUTING"}
	if iface != "" {
		args = append(args, "-i", iface)
	}
	args = append(args, "-p", "udp", "--dport", portRange, "-j", "REDIRECT", "--to-ports", fmt.Sprintf("%d", targetPort))
	return exec.Command("iptables", args...).Run()
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
