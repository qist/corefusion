package xraycontrol

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/gin-gonic/gin"

	"corefusion/internal/api/response"
)

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

	// Check if nftables is available
	useNft := isNftablesAvailable()

	// Check if rule already exists
	exists, err := checkFirewallRuleExists(useNft, req.PortRange, req.TargetPort, req.Interface)
	if err != nil {
		response.InternalError(c, "CHECK_FAILED", err.Error())
		return
	}
	if exists {
		response.OK(c, gin.H{"message": "rule already exists", "type": getFirewallType(useNft)})
		return
	}

	// Apply firewall rule
	if err := applyFirewallRule(useNft, req.PortRange, req.TargetPort, req.Interface); err != nil {
		response.InternalError(c, "APPLY_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{"message": "firewall rule applied", "type": getFirewallType(useNft)})
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

	// Check if nftables is available
	useNft := isNftablesAvailable()

	// Remove firewall rule
	if err := removeFirewallRule(useNft, req.PortRange, req.TargetPort, req.Interface); err != nil {
		response.InternalError(c, "REMOVE_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{"message": "firewall rule removed", "type": getFirewallType(useNft)})
}

// isNftablesAvailable checks if nftables is available on the system.
func isNftablesAvailable() bool {
	_, err := exec.LookPath("nft")
	return err == nil
}

func getFirewallType(useNft bool) string {
	if useNft {
		return "nftables"
	}
	return "iptables"
}

// checkFirewallRuleExists checks if a firewall rule already exists.
func checkFirewallRuleExists(useNft bool, portRange string, targetPort int, iface string) (bool, error) {
	if useNft {
		return checkNftRuleExists(portRange, targetPort, iface)
	}
	return checkIptablesRuleExists(portRange, targetPort, iface)
}

// checkNftRuleExists checks if an nftables rule exists.
func checkNftRuleExists(portRange string, targetPort int, iface string) (bool, error) {
	// Check if the table exists
	cmd := exec.Command("nft", "list", "table", "inet", "hysteria_porthopping")
	if err := cmd.Run(); err != nil {
		return false, nil // Table doesn't exist
	}

	// List rules in the chain
	cmd = exec.Command("nft", "list", "chain", "inet", "hysteria_porthopping", "prerouting")
	out, err := cmd.Output()
	if err != nil {
		return false, nil
	}

	rules := string(out)
	// Check if rule with same port range and target port exists
	if strings.Contains(rules, portRange) && strings.Contains(rules, fmt.Sprintf(":%d", targetPort)) {
		return true, nil
	}

	return false, nil
}

// checkIptablesRuleExists checks if an iptables rule exists.
func checkIptablesRuleExists(portRange string, targetPort int, iface string) (bool, error) {
	cmd := exec.Command("iptables", "-t", "nat", "-L", "PREROUTING", "-n")
	out, err := cmd.Output()
	if err != nil {
		return false, nil
	}

	rules := string(out)
	// Check if rule with same port range and target port exists
	if strings.Contains(rules, portRange) && strings.Contains(rules, fmt.Sprintf("to:%d", targetPort)) {
		return true, nil
	}

	return false, nil
}

// applyFirewallRule applies a firewall rule.
func applyFirewallRule(useNft bool, portRange string, targetPort int, iface string) error {
	if useNft {
		return applyNftRule(portRange, targetPort, iface)
	}
	return applyIptablesRule(portRange, targetPort, iface)
}

// applyNftRule applies an nftables rule.
func applyNftRule(portRange string, targetPort int, iface string) error {
	// Create table if not exists
	cmd := exec.Command("nft", "add", "table", "inet", "hysteria_porthopping")
	cmd.Run()

	// Create chain if not exists
	cmd = exec.Command("nft", "add", "chain", "inet", "hysteria_porthopping", "prerouting",
		"{ type nat hook prerouting priority dstnat; policy accept; }")
	cmd.Run()

	// Build rule
	rule := fmt.Sprintf("udp dport %s counter redirect to :%d", portRange, targetPort)
	if iface != "" {
		rule = fmt.Sprintf("iifname %s %s", iface, rule)
	}

	cmd = exec.Command("nft", "add", "rule", "inet", "hysteria_porthopping", "prerouting", rule)
	return cmd.Run()
}

// applyIptablesRule applies an iptables rule.
func applyIptablesRule(portRange string, targetPort int, iface string) error {
	args := []string{"-t", "nat", "-A", "PREROUTING"}
	if iface != "" {
		args = append(args, "-i", iface)
	}
	args = append(args, "-p", "udp", "--dport", portRange, "-j", "REDIRECT", "--to-ports", fmt.Sprintf("%d", targetPort))

	cmd := exec.Command("iptables", args...)
	return cmd.Run()
}

// removeFirewallRule removes a firewall rule.
func removeFirewallRule(useNft bool, portRange string, targetPort int, iface string) error {
	if useNft {
		return removeNftRule(portRange, targetPort, iface)
	}
	return removeIptablesRule(portRange, targetPort, iface)
}

// removeNftRule removes an nftables rule.
func removeNftRule(portRange string, targetPort int, iface string) error {
	// Delete the entire table (simplest approach)
	cmd := exec.Command("nft", "delete", "table", "inet", "hysteria_porthopping")
	return cmd.Run()
}

// removeIptablesRule removes an iptables rule.
func removeIptablesRule(portRange string, targetPort int, iface string) error {
	args := []string{"-t", "nat", "-D", "PREROUTING"}
	if iface != "" {
		args = append(args, "-i", iface)
	}
	args = append(args, "-p", "udp", "--dport", portRange, "-j", "REDIRECT", "--to-ports", fmt.Sprintf("%d", targetPort))

	cmd := exec.Command("iptables", args...)
	return cmd.Run()
}
