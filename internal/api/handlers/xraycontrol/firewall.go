package xraycontrol

import (
	"encoding/json"
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

	// Remote: proxy via Agent
	if resp, _, handled := h.remoteXrayAPI(c, "firewall", "apply", req); handled {
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

	// Remote: proxy via Agent
	if resp, _, handled := h.remoteXrayAPI(c, "firewall", "remove", req); handled {
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

	cmd = exec.Command("nft", "list", "chain", "inet", "hysteria_porthopping", "prerouting")
	out, err := cmd.Output()
	if err != nil {
		return false, nil
	}

	rules := string(out)
	if strings.Contains(rules, portRange) && strings.Contains(rules, fmt.Sprintf(":%d", targetPort)) {
		return true, nil
	}

	return false, nil
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
	cmd := exec.Command("nft", "delete", "table", "inet", "hysteria_porthopping")
	return cmd.Run()
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
