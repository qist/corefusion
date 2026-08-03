package xraycontrol

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
)

// portRangePattern matches valid port ranges: single port (e.g. "443") or range (e.g. "20000-30000").
var portRangePattern = regexp.MustCompile(`^\d+(-\d+)?$`)

// ifacePattern matches valid network interface names (e.g. "eth0", "wlan1", "br-lan").
var ifacePattern = regexp.MustCompile(`^[a-zA-Z0-9._-]{1,15}$`)

// validateFirewallInput validates the port_range and interface fields to prevent
// injection of unexpected arguments into nft/iptables commands.
func validateFirewallInput(req FirewallRule) error {
	if !portRangePattern.MatchString(req.PortRange) {
		return fmt.Errorf("invalid port_range format: must be a single port or range (e.g. 443 or 20000-30000)")
	}
	if req.Interface != "" && !ifacePattern.MatchString(req.Interface) {
		return fmt.Errorf("invalid interface name: must be alphanumeric with optional . _ - (max 15 chars)")
	}
	return nil
}

// FirewallRule represents a firewall rule for port hopping.
type FirewallRule struct {
	PortRange  string `json:"port_range"`
	TargetPort int    `json:"target_port"`
	Interface  string `json:"interface"`
}

// ApplyFirewallRules handles POST /api/v1/xray/control/firewall/apply.
func (h *Handler) ApplyFirewallRules(c *gin.Context) {
	var req FirewallRule
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_PARAMS", err.Error())
		return
	}

	if req.PortRange == "" || req.TargetPort == 0 {
		response.BadRequest(c, "INVALID_PARAMS", "port_range and target_port are required")
		return
	}
	if err := validateFirewallInput(req); err != nil {
		response.BadRequest(c, "INVALID_PARAMS", err.Error())
		return
	}

	// Remote: proxy via Agent
	if resp, handled, _ := h.remoteXrayAPI(c, "firewall", "apply", req); handled {
		if resp != nil {
			var result interface{}
			json.Unmarshal(resp, &result)
			response.OK(c, result)
		}
		return
	}

	// Local: execute directly
	useNft := isNftablesAvailableLocal()

	exists, err := checkFirewallRuleExistsLocal(useNft, req.PortRange, req.TargetPort, req.Interface)
	if err != nil {
		response.InternalError(c, "CHECK_FAILED", err.Error())
		return
	}
	if exists {
		response.OK(c, gin.H{"message": "rule already exists", "type": getFirewallTypeLocal(useNft)})
		return
	}

	if err := applyFirewallRuleLocal(useNft, req.PortRange, req.TargetPort, req.Interface); err != nil {
		response.InternalError(c, "APPLY_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{"message": "firewall rule applied", "type": getFirewallTypeLocal(useNft)})
}

// RemoveFirewallRules handles POST /api/v1/xray/control/firewall/remove.
func (h *Handler) RemoveFirewallRules(c *gin.Context) {
	var req FirewallRule
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_PARAMS", err.Error())
		return
	}

	if req.PortRange == "" || req.TargetPort == 0 {
		response.BadRequest(c, "INVALID_PARAMS", "port_range and target_port are required")
		return
	}
	if err := validateFirewallInput(req); err != nil {
		response.BadRequest(c, "INVALID_PARAMS", err.Error())
		return
	}

	// Remote: proxy via Agent
	if resp, handled, _ := h.remoteXrayAPI(c, "firewall", "remove", req); handled {
		if resp != nil {
			var result interface{}
			json.Unmarshal(resp, &result)
			response.OK(c, result)
		}
		return
	}

	// Local: execute directly
	useNft := isNftablesAvailableLocal()

	if err := removeFirewallRuleLocal(useNft, req.PortRange, req.TargetPort, req.Interface); err != nil {
		response.InternalError(c, "REMOVE_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{"message": "firewall rule removed", "type": getFirewallTypeLocal(useNft)})
}

// Local firewall helper functions (suffixed with "Local" to avoid conflicts
// with the Agent-side implementations in xrayapi.go)

func isNftablesAvailableLocal() bool {
	_, err := exec.LookPath("nft")
	return err == nil
}

func getFirewallTypeLocal(useNft bool) string {
	if useNft {
		return "nftables"
	}
	return "iptables"
}

func checkFirewallRuleExistsLocal(useNft bool, portRange string, targetPort int, iface string) (bool, error) {
	if useNft {
		return checkNftRuleExistsLocal(portRange, targetPort, iface)
	}
	return checkIptablesRuleExistsLocal(portRange, targetPort, iface)
}

func checkNftRuleExistsLocal(portRange string, targetPort int, iface string) (bool, error) {
	cmd := exec.Command("nft", "list", "table", "inet", "hysteria_porthopping")
	if err := cmd.Run(); err != nil {
		return false, nil
	}
	handle, err := findNftRuleHandleLocal(portRange, targetPort, iface)
	if err != nil {
		return false, err
	}
	return handle > 0, nil
}

// nftRuleMatchesLocal checks if a rule line from `nft -a list chain` output matches
// the given port range, target port, and interface.
func nftRuleMatchesLocal(line, portRange string, targetPort int, iface string) bool {
	if !strings.Contains(line, "redirect to") {
		return false
	}
	if !strings.Contains(line, fmt.Sprintf("dport %s ", portRange)) {
		return false
	}
	if !strings.Contains(line, fmt.Sprintf("redirect to :%d", targetPort)) {
		return false
	}
	if iface != "" {
		if !strings.Contains(line, fmt.Sprintf(`iifname "%s"`, iface)) {
			return false
		}
	} else {
		if strings.Contains(line, "iifname ") {
			return false
		}
	}
	return true
}

// findNftRuleHandleLocal lists the chain with handles and finds the handle of the
// rule matching the given spec. Returns 0 if not found.
func findNftRuleHandleLocal(portRange string, targetPort int, iface string) (uint64, error) {
	cmd := exec.Command("nft", "-a", "list", "chain", "inet", "hysteria_porthopping", "prerouting")
	out, err := cmd.Output()
	if err != nil {
		return 0, nil
	}
	for _, line := range strings.Split(string(out), "\n") {
		if !nftRuleMatchesLocal(line, portRange, targetPort, iface) {
			continue
		}
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

func checkIptablesRuleExistsLocal(portRange string, targetPort int, iface string) (bool, error) {
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

func applyFirewallRuleLocal(useNft bool, portRange string, targetPort int, iface string) error {
	if useNft {
		return applyNftRuleLocal(portRange, targetPort, iface)
	}
	return applyIptablesRuleLocal(portRange, targetPort, iface)
}

func applyNftRuleLocal(portRange string, targetPort int, iface string) error {
	exec.Command("nft", "add", "table", "inet", "hysteria_porthopping").Run()

	exec.Command("nft", "add", "chain", "inet", "hysteria_porthopping", "prerouting",
		"{ type nat hook prerouting priority dstnat; policy accept; }").Run()

	rule := fmt.Sprintf("udp dport %s counter redirect to :%d", portRange, targetPort)
	if iface != "" {
		rule = fmt.Sprintf("iifname %s %s", iface, rule)
	}

	cmd := exec.Command("nft", "add", "rule", "inet", "hysteria_porthopping", "prerouting", rule)
	return cmd.Run()
}

func applyIptablesRuleLocal(portRange string, targetPort int, iface string) error {
	args := []string{"-t", "nat", "-A", "PREROUTING"}
	if iface != "" {
		args = append(args, "-i", iface)
	}
	args = append(args, "-p", "udp", "--dport", portRange, "-j", "REDIRECT", "--to-ports", fmt.Sprintf("%d", targetPort))

	cmd := exec.Command("iptables", args...)
	return cmd.Run()
}

func removeFirewallRuleLocal(useNft bool, portRange string, targetPort int, iface string) error {
	if useNft {
		return removeNftRuleLocal(portRange, targetPort, iface)
	}
	return removeIptablesRuleLocal(portRange, targetPort, iface)
}

func removeNftRuleLocal(portRange string, targetPort int, iface string) error {
	handle, err := findNftRuleHandleLocal(portRange, targetPort, iface)
	if err != nil {
		return fmt.Errorf("find rule handle: %w", err)
	}
	if handle == 0 {
		return nil // rule doesn't exist, nothing to remove
	}
	return exec.Command("nft", "delete", "rule", "inet", "hysteria_porthopping", "prerouting", "handle", strconv.FormatUint(handle, 10)).Run()
}

func removeIptablesRuleLocal(portRange string, targetPort int, iface string) error {
	args := []string{"-t", "nat", "-D", "PREROUTING"}
	if iface != "" {
		args = append(args, "-i", iface)
	}
	args = append(args, "-p", "udp", "--dport", portRange, "-j", "REDIRECT", "--to-ports", fmt.Sprintf("%d", targetPort))

	cmd := exec.Command("iptables", args...)
	return cmd.Run()
}
