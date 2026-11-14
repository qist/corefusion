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
	"corefusion/internal/runtime/firewall"
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

	// 端口段是节点级全局资源：同一段只能指向一个入站端口（xray / sing-box 可能同时开启）
	if err := firewall.CheckConflict(firewall.RuleSpec{
		PortRange:  req.PortRange,
		TargetPort: req.TargetPort,
		Interface:  req.Interface,
	}); err != nil {
		response.BadRequest(c, "PORT_RANGE_CONFLICT", err.Error())
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

// isNftablesAvailableLocal 判断本机 nft 是否**真的能用 NAT**。
//
// 只判断二进制存在是不够的：容器里常见 nft 可执行文件在、但内核没有 nft_nat
// 模块，add chain type nat 直接返回 "Could not process rule: No such file or directory"。
// 这时必须回退 iptables（xt_nat 通常可用），否则端口跳跃一直 500。
func isNftablesAvailableLocal() bool {
	if _, err := exec.LookPath("nft"); err != nil {
		return false
	}
	const probe = "corefusion_fw_probe"
	_ = exec.Command("nft", "delete", "table", "inet", probe).Run()
	if out, err := exec.Command("nft", "add", "table", "inet", probe).CombinedOutput(); err != nil {
		_ = out
		return false
	}
	defer exec.Command("nft", "delete", "table", "inet", probe).Run()
	chain := exec.Command("nft", "add", "chain", "inet", probe, "prerouting",
		"{ type nat hook prerouting priority dstnat; policy accept; }")
	if _, err := chain.CombinedOutput(); err != nil {
		return false
	}
	return true
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
	spec := iptablesPortSpec(portRange)
	if !strings.Contains(rules, spec) {
		return false, nil
	}
	// 不同 iptables 版本/后端的展示不同：legacy 是 "to:4443"，-L -n 是 "redir ports 4443"。
	if strings.Contains(rules, fmt.Sprintf("to:%d", targetPort)) ||
		strings.Contains(rules, fmt.Sprintf("redir ports %d", targetPort)) {
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

// runFirewallCmd 执行防火墙命令并把 stderr 带进错误里。
// 之前只用 cmd.Run()，失败时只剩 "exit status 1"，面板上根本看不出原因。
func runFirewallCmd(name string, args ...string) error {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err == nil {
		return nil
	}
	msg := strings.TrimSpace(string(out))
	if msg == "" {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return fmt.Errorf("%s %s: %s", name, strings.Join(args, " "), msg)
}

// iptablesPortSpec 把 "20000-30000" 转成 iptables 的 "20000:30000"。
// iptables 的端口段用**冒号**，用中划线会被当成服务名而报错
// （"Port 20000-30000 does not resolve to anything"）。
func iptablesPortSpec(portRange string) string {
	return strings.ReplaceAll(portRange, "-", ":")
}

func applyNftRuleLocal(portRange string, targetPort int, iface string) error {
	// 表已存在时 add 会报错，忽略即可；链不存在则是真错误（缺 nft_nat）。
	_ = exec.Command("nft", "add", "table", "inet", "hysteria_porthopping").Run()
	if err := runFirewallCmd("nft", "add", "chain", "inet", "hysteria_porthopping", "prerouting",
		"{ type nat hook prerouting priority dstnat; policy accept; }"); err != nil {
		return fmt.Errorf("nft nat chain unavailable: %w", err)
	}

	rule := fmt.Sprintf("udp dport %s counter redirect to :%d", portRange, targetPort)
	if iface != "" {
		rule = fmt.Sprintf("iifname %s %s", iface, rule)
	}

	return runFirewallCmd("nft", "add", "rule", "inet", "hysteria_porthopping", "prerouting", rule)
}

func applyIptablesRuleLocal(portRange string, targetPort int, iface string) error {
	args := []string{"-t", "nat", "-A", "PREROUTING"}
	if iface != "" {
		args = append(args, "-i", iface)
	}
	args = append(args, "-p", "udp", "--dport", iptablesPortSpec(portRange), "-j", "REDIRECT", "--to-ports", fmt.Sprintf("%d", targetPort))

	return runFirewallCmd("iptables", args...)
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
	spec := iptablesPortSpec(portRange)
	args := []string{"-t", "nat", "-D", "PREROUTING"}
	if iface != "" {
		args = append(args, "-i", iface)
	}
	args = append(args, "-p", "udp", "--dport", spec, "-j", "REDIRECT", "--to-ports", fmt.Sprintf("%d", targetPort))

	if err := runFirewallCmd("iptables", args...); err == nil {
		return nil
	}

	// 精确 -D 失败通常是规则根本不存在（例如端口段写错），或者添加时的 match/参数
	// 写法与这里不一致（iptables -D 要求整条规则逐字匹配）。按实际规则表找同
	// 端口段 + 同目标端口的行，用它的原始 -S 表示删除；找不到就视为"已经不存在"，
	// 返回成功而不是 500，保证 remove 幂等。
	del, found, err := findIptablesRuleDeleteArgs(spec, targetPort)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	return runFirewallCmd("iptables", del...)
}

// findIptablesRuleDeleteArgs 在 nat/PREROUTING 里查找匹配端口段与目标端口的规则，
// 返回用 `iptables -S` 形式改写成的删除参数（把 -A 换成 -D）。
//
// 用 `-S` 而不是 `-L`：-S 输出就是可直接回放的规则原文，能避开 match 顺序/隐式
// 模块（-m udp）导致的 "No chain/target/match by that name"。
func findIptablesRuleDeleteArgs(portSpec string, targetPort int) ([]string, bool, error) {
	out, err := exec.Command("iptables", "-t", "nat", "-S", "PREROUTING").CombinedOutput()
	if err != nil {
		return nil, false, fmt.Errorf("iptables -S PREROUTING: %s", strings.TrimSpace(string(out)))
	}
	args, found := matchIptablesDeleteArgs(string(out), portSpec, targetPort)
	return args, found, nil
}

// matchIptablesDeleteArgs 从 `iptables -S` 输出里找到匹配的规则并改写成删除参数。
// 抽成纯函数便于单测（不要在这里执行命令）。
func matchIptablesDeleteArgs(s string, portSpec string, targetPort int) ([]string, bool) {
	wantDport := "--dport " + portSpec
	wantTo := fmt.Sprintf("--to-ports %d", targetPort)
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "-A PREROUTING ") {
			continue
		}
		if !strings.Contains(line, wantDport) || !strings.Contains(line, wantTo) {
			continue
		}
		fields := strings.Fields(line)
		fields[0] = "-D"
		return append([]string{"-t", "nat"}, fields...), true
	}
	return nil, false
}

// ── 入站端口防火墙放行（firewalld / ufw / nftables / iptables）──

// FirewallPortRule 是放行/撤销放行的请求体。
// PortRange 用于 hy2/hysteria 端口跳跃：客户端访问的是端口段（DNAT 到 Port）。
// Network 为 tcp / udp / tcp,udp；空 = 两者。
type FirewallPortRule struct {
	Port      int    `json:"port"`
	PortRange string `json:"port_range"`
	Network   string `json:"network"`
}

// FirewallStatus handles GET /api/v1/node-servers/:id/firewall/status.
// 返回节点上检测到的防火墙类型与是否启用，供编辑器决定是否展示「自动放行」选项。
func (h *Handler) FirewallStatus(c *gin.Context) {
	if resp, handled, failed := h.remoteXrayAPI(c, "firewall", "status", struct{}{}); handled {
		if failed || resp == nil {
			return
		}
		var result interface{}
		json.Unmarshal(resp, &result)
		response.OK(c, result)
		return
	}
	response.OK(c, firewall.Detect())
}

// AllowFirewallPort handles POST /api/v1/node-servers/:id/firewall/allow-port.
func (h *Handler) AllowFirewallPort(c *gin.Context) {
	var req FirewallPortRule
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_PARAMS", err.Error())
		return
	}
	if err := validateFirewallPortRule(req); err != nil {
		response.BadRequest(c, "INVALID_PARAMS", err.Error())
		return
	}
	if resp, handled, failed := h.remoteXrayAPI(c, "firewall", "allow", req); handled {
		if failed || resp == nil {
			return
		}
		var result interface{}
		json.Unmarshal(resp, &result)
		response.OK(c, result)
		return
	}
	if req.Port > 0 {
		if err := firewall.AllowPort(req.Port, req.Network); err != nil {
			response.InternalError(c, "FIREWALL_ALLOW_FAILED", err.Error())
			return
		}
	}
	if req.PortRange != "" {
		if err := firewall.AllowPortRange(req.PortRange, req.Network); err != nil {
			response.InternalError(c, "FIREWALL_ALLOW_FAILED", err.Error())
			return
		}
	}
	response.OK(c, gin.H{"port": req.Port, "port_range": req.PortRange, "network": req.Network})
}

// RemoveFirewallPort handles POST /api/v1/node-servers/:id/firewall/remove-port.
func (h *Handler) RemoveFirewallPort(c *gin.Context) {
	var req FirewallPortRule
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_PARAMS", err.Error())
		return
	}
	if err := validateFirewallPortRule(req); err != nil {
		response.BadRequest(c, "INVALID_PARAMS", err.Error())
		return
	}
	if resp, handled, failed := h.remoteXrayAPI(c, "firewall", "unallow", req); handled {
		if failed || resp == nil {
			return
		}
		var result interface{}
		json.Unmarshal(resp, &result)
		response.OK(c, result)
		return
	}
	if req.Port > 0 {
		if err := firewall.RemovePort(req.Port, req.Network); err != nil {
			response.InternalError(c, "FIREWALL_REMOVE_FAILED", err.Error())
			return
		}
	}
	if req.PortRange != "" {
		if err := firewall.RemovePortRange(req.PortRange, req.Network); err != nil {
			response.InternalError(c, "FIREWALL_REMOVE_FAILED", err.Error())
			return
		}
	}
	response.OK(c, gin.H{"port": req.Port, "port_range": req.PortRange, "network": req.Network})
}

func validateFirewallPortRule(req FirewallPortRule) error {
	if req.Port == 0 && req.PortRange == "" {
		return fmt.Errorf("port or port_range is required")
	}
	if req.Port < 0 || req.Port > 65535 {
		return fmt.Errorf("port must be 1-65535")
	}
	if req.PortRange != "" {
		if _, err := firewall.ParsePortSpecs(req.PortRange); err != nil {
			return err
		}
	}
	return nil
}
