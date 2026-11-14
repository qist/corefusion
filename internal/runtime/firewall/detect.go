package firewall

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// 防火墙类型。检测顺序：firewalld → ufw → nftables → iptables → none。
// 前两者是「管理器」：有明确的开关与放行命令，优先使用；
// 裸 nftables / iptables 只在没有管理器时才作为兜底。
const (
	KindNone      = "none"
	KindFirewalld = "firewalld"
	KindUFW       = "ufw"
	KindNftables  = "nftables"
	KindIptables  = "iptables"
)

// Status 是一次防火墙探测结果。
type Status struct {
	Enabled bool   `json:"enabled"`
	Kind    string `json:"kind"`
	Detail  string `json:"detail,omitempty"`
}

var (
	statusMu     sync.RWMutex
	cachedStatus *Status
)

// Init 在面板 / Agent 启动时调用一次：探测本机防火墙并缓存结果，
// 便于启动日志与后续快速判断（Handler 仍会用 Detect() 取实时值）。
func Init() Status {
	st := Detect()
	statusMu.Lock()
	cachedStatus = &st
	statusMu.Unlock()
	if st.Enabled {
		log.Printf("[Firewall] detected %s (%s)", st.Kind, st.Detail)
	} else {
		log.Printf("[Firewall] no active host firewall detected")
	}
	return st
}

// Cached 返回启动时探测到的状态（未初始化时现场探测一次）。
func Cached() Status {
	statusMu.RLock()
	if cachedStatus != nil {
		st := *cachedStatus
		statusMu.RUnlock()
		return st
	}
	statusMu.RUnlock()
	return Detect()
}

// Detect 现场探测本机防火墙。
func Detect() Status {
	if st, ok := detectFirewalld(); ok {
		return st
	}
	if st, ok := detectUFW(); ok {
		return st
	}
	if st, ok := detectNftables(); ok {
		return st
	}
	if st, ok := detectIptables(); ok {
		return st
	}
	return Status{Enabled: false, Kind: KindNone}
}

func runOut(name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func detectFirewalld() (Status, bool) {
	out, err := runOut("firewall-cmd", "--state")
	if err != nil {
		return Status{}, false
	}
	if strings.EqualFold(out, "running") {
		return Status{Enabled: true, Kind: KindFirewalld, Detail: "firewall-cmd --state: running"}, true
	}
	return Status{}, false
}

func detectUFW() (Status, bool) {
	out, err := runOut("ufw", "status")
	if err != nil {
		return Status{}, false
	}
	if strings.Contains(strings.ToLower(out), "status: active") {
		return Status{Enabled: true, Kind: KindUFW, Detail: "ufw status: active"}, true
	}
	return Status{}, false
}

// detectNftables 把「存在 input hook 的 base chain」视为启用了 nftables 防火墙。
// 只有我们自己的 nat 表（hysteria_porthopping / corefusion_fw）不算。
func detectNftables() (Status, bool) {
	out, err := runOut("nft", "list", "ruleset")
	if err != nil || out == "" {
		return Status{}, false
	}
	if !strings.Contains(out, "hook input") {
		return Status{}, false
	}
	return Status{Enabled: true, Kind: KindNftables, Detail: "nftables input chain detected"}, true
}

func detectIptables() (Status, bool) {
	out, err := runOut("iptables", "-w", "-S", "INPUT")
	if err != nil || out == "" {
		return Status{}, false
	}
	if strings.Contains(out, "policy DROP") || strings.Contains(out, "policy REJECT") {
		return Status{Enabled: true, Kind: KindIptables, Detail: "iptables INPUT policy drop/reject"}, true
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "-A INPUT") {
			return Status{Enabled: true, Kind: KindIptables, Detail: "iptables INPUT rules present"}, true
		}
	}
	return Status{}, false
}

// normalizeProtos 把 "tcp,udp" / "tcp" / "" 归一成协议列表；空值默认同时放行 TCP+UDP。
func normalizeProtos(network string) ([]string, error) {
	network = strings.TrimSpace(strings.ToLower(network))
	if network == "" || network == "tcp,udp" || network == "udp,tcp" {
		return []string{"tcp", "udp"}, nil
	}
	out := make([]string, 0, 2)
	seen := map[string]bool{}
	for _, p := range strings.Split(network, ",") {
		p = strings.TrimSpace(p)
		if p != "tcp" && p != "udp" {
			return nil, fmt.Errorf("invalid network %q: only tcp/udp supported", network)
		}
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out, nil
}

// AllowPort 确保端口在已启用的防火墙里被放行（TCP/UDP，按 network 指定）。
// 未检测到启用的防火墙时是 no-op：没有过滤就不需要放行规则。
func AllowPort(port int, network string) error {
	if port <= 0 || port > 65535 {
		return fmt.Errorf("invalid port: %d", port)
	}
	protos, err := normalizeProtos(network)
	if err != nil {
		return err
	}
	st := Detect()
	if !st.Enabled {
		return nil
	}
	for _, proto := range protos {
		if err := allowProto(st.Kind, port, proto); err != nil {
			return fmt.Errorf("%s allow %d/%s: %w", st.Kind, port, proto, err)
		}
	}
	return nil
}

// RemovePort 撤销 AllowPort 添加的放行规则；规则不存在时视为成功。
func RemovePort(port int, network string) error {
	if port <= 0 || port > 65535 {
		return fmt.Errorf("invalid port: %d", port)
	}
	protos, err := normalizeProtos(network)
	if err != nil {
		return err
	}
	st := Detect()
	if !st.Enabled {
		return nil
	}
	for _, proto := range protos {
		if err := removeProto(st.Kind, port, proto); err != nil {
			return fmt.Errorf("%s remove %d/%s: %w", st.Kind, port, proto, err)
		}
	}
	return nil
}

// ParsePortSpecs 解析端口规格：支持多段与不同写法，返回去重后的 [lo,hi] 列表。
//
//	"20000-29999"                      → [[20000,29999]]
//	"20000:29999"                      → [[20000,29999]]（sing-box server_ports 写法）
//	"20000-29999,30000-39999"          → 两段（mihomo ports / xray hopPorts 写法）
//	"443" 或 "443,8443"                → 单端口
//
// 任何一段非法都返回错误（用于接口参数校验）。
func ParsePortSpecs(spec string) ([][2]int, error) {
	tokens := splitPortSpecs(spec)
	if len(tokens) == 0 {
		return nil, fmt.Errorf("empty port range")
	}
	if len(tokens) > maxPortSpecs {
		return nil, fmt.Errorf("too many port ranges (max %d)", maxPortSpecs)
	}
	seen := make(map[[2]int]bool, len(tokens))
	out := make([][2]int, 0, len(tokens))
	for _, token := range tokens {
		lo, hi, ok := parsePortRangeToken(token)
		if !ok {
			return nil, fmt.Errorf("invalid port range: %q", token)
		}
		r := [2]int{lo, hi}
		if seen[r] {
			continue
		}
		seen[r] = true
		out = append(out, r)
	}
	return out, nil
}

const maxPortSpecs = 64

// splitPortSpecs 按逗号/分号/空白拆分多段规格。
func splitPortSpecs(spec string) []string {
	return strings.FieldsFunc(spec, func(r rune) bool {
		switch r {
		case ',', ';', ' ', '\t', '\n', '\r':
			return true
		}
		return false
	})
}

// parsePortRangeToken 解析单个 "起-止" / "起:止" / 单端口。
func parsePortRangeToken(token string) (int, int, bool) {
	token = strings.TrimSpace(token)
	if token == "" {
		return 0, 0, false
	}
	loStr, hiStr, found := strings.Cut(token, "-")
	if !found {
		loStr, hiStr, found = strings.Cut(token, ":")
	}
	if !found {
		p, err := strconv.Atoi(token)
		if err != nil || p <= 0 || p > 65535 {
			return 0, 0, false
		}
		return p, p, true
	}
	lo, err1 := strconv.Atoi(strings.TrimSpace(loStr))
	hi, err2 := strconv.Atoi(strings.TrimSpace(hiStr))
	if err1 != nil || err2 != nil || lo <= 0 || hi > 65535 || lo > hi {
		return 0, 0, false
	}
	return lo, hi, true
}

// AllowPortRange 放行端口段（hy2 端口跳跃的访问端口，支持多段），幂等。
func AllowPortRange(spec string, network string) error {
	ranges, err := ParsePortSpecs(spec)
	if err != nil {
		return err
	}
	protos, err := normalizeProtos(network)
	if err != nil {
		return err
	}
	st := Detect()
	if !st.Enabled {
		return nil
	}
	for _, r := range ranges {
		for _, proto := range protos {
			if err := allowRangeProto(st.Kind, r[0], r[1], proto); err != nil {
				return fmt.Errorf("%s allow %d-%d/%s: %w", st.Kind, r[0], r[1], proto, err)
			}
		}
	}
	return nil
}

// RemovePortRange 撤销 AllowPortRange 添加的放行规则。
func RemovePortRange(spec string, network string) error {
	ranges, err := ParsePortSpecs(spec)
	if err != nil {
		return err
	}
	protos, err := normalizeProtos(network)
	if err != nil {
		return err
	}
	st := Detect()
	if !st.Enabled {
		return nil
	}
	for _, r := range ranges {
		for _, proto := range protos {
			if err := removeRangeProto(st.Kind, r[0], r[1], proto); err != nil {
				return fmt.Errorf("%s remove %d-%d/%s: %w", st.Kind, r[0], r[1], proto, err)
			}
		}
	}
	return nil
}

// allowProto 放行单个协议/端口。所有实现都必须**幂等**：
// 面板/Agent 每次启动都会按入站配置恢复一遍放行规则，已经放行过的端口绝不能再加一条。
func allowProto(kind string, port int, proto string) error {
	switch kind {
	case KindFirewalld:
		return firewalldPort("--add-port", port, proto)
	case KindUFW:
		if ufwCoversPort(port, proto) {
			return nil // 已放行（含范围规则覆盖）
		}
		_, err := runOut("ufw", "allow", fmt.Sprintf("%d/%s", port, proto))
		return err
	case KindNftables:
		return nftAllow(port, proto)
	case KindIptables:
		if iptablesCoversPort(port, proto) {
			return nil // 已放行（含范围/multiport 规则覆盖）
		}
		_, err := runOut("iptables", "-w", "-I", "INPUT", "-p", proto, "--dport", strconv.Itoa(port), "-j", "ACCEPT")
		return err
	}
	return nil
}

// allowRangeProto 放行端口段。与 allowProto 一样必须幂等。
func allowRangeProto(kind string, lo, hi int, proto string) error {
	switch kind {
	case KindFirewalld:
		return firewalldPortSpec("--add-port", fmt.Sprintf("%d-%d", lo, hi), proto)
	case KindUFW:
		if ufwStatusCoversRange(proto, lo, hi) {
			return nil
		}
		_, err := runOut("ufw", "allow", fmt.Sprintf("%d:%d/%s", lo, hi, proto))
		return err
	case KindNftables:
		return nftAllowRange(lo, hi, proto)
	case KindIptables:
		if iptablesCoversRange(proto, lo, hi) {
			return nil
		}
		_, err := runOut("iptables", "-w", "-I", "INPUT", "-p", proto,
			"--dport", fmt.Sprintf("%d:%d", lo, hi), "-j", "ACCEPT")
		return err
	}
	return nil
}

func removeRangeProto(kind string, lo, hi int, proto string) error {
	switch kind {
	case KindFirewalld:
		return firewalldPortSpec("--remove-port", fmt.Sprintf("%d-%d", lo, hi), proto)
	case KindUFW:
		for i := 0; i < 16; i++ {
			out, err := runOut("ufw", "delete", "allow", fmt.Sprintf("%d:%d/%s", lo, hi, proto))
			if err == nil {
				continue
			}
			lower := strings.ToLower(out)
			if strings.Contains(lower, "could not delete") || strings.Contains(lower, "not found") {
				return nil
			}
			return err
		}
		return nil
	case KindNftables:
		return nftRemoveRange(lo, hi, proto)
	case KindIptables:
		for i := 0; i < 16; i++ {
			out, err := runOut("iptables", "-w", "-D", "INPUT", "-p", proto,
				"--dport", fmt.Sprintf("%d:%d", lo, hi), "-j", "ACCEPT")
			if err == nil {
				continue
			}
			lower := strings.ToLower(out)
			if strings.Contains(lower, "bad rule") || strings.Contains(lower, "matching rule") {
				return nil
			}
			return err
		}
		return nil
	}
	return nil
}

// iptablesCoversPort 检查 INPUT 链里是否已有覆盖该端口的 ACCEPT 规则。
//
// 只做 `-C --dport N` 精确匹配不够：管理员常写成端口段
// （`--dport 20000:30000`）或多端口（`-m multiport --dports 80,443`），
// 此时端口其实已经放行，再插一条就是重复规则（每次启动多一条）。
func iptablesCoversPort(port int, proto string) bool {
	out, err := runOut("iptables", "-w", "-S", "INPUT")
	if err != nil {
		return false
	}
	return iptablesOutputCoversPort(out, proto, port)
}

// iptablesOutputCoversPort 解析 `iptables -S INPUT` 的输出做覆盖判断（纯函数，便于单测）。
func iptablesOutputCoversPort(output, proto string, port int) bool {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[0] != "-A" || fields[1] != "INPUT" {
			continue
		}
		if !hasJumpAccept(fields) {
			continue
		}
		// -p 缺省 = 所有协议，同样覆盖
		if p, ok := flagValue(fields, "-p"); ok && !strings.EqualFold(p, proto) {
			continue
		}
		if spec, ok := flagValue(fields, "--dport"); ok && portSpecCovers(spec, port, ":") {
			return true
		}
		if spec, ok := flagValue(fields, "--dports"); ok {
			for _, item := range strings.Split(spec, ",") {
				if portSpecCovers(item, port, ":") {
					return true
				}
			}
		}
	}
	return false
}

func iptablesCoversRange(proto string, lo, hi int) bool {
	out, err := runOut("iptables", "-w", "-S", "INPUT")
	if err != nil {
		return false
	}
	return iptablesOutputCoversRange(out, proto, lo, hi)
}

// iptablesOutputCoversRange 判断已有 ACCEPT 规则是否完整覆盖 [lo,hi] 端口段。
func iptablesOutputCoversRange(output, proto string, lo, hi int) bool {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[0] != "-A" || fields[1] != "INPUT" || !hasJumpAccept(fields) {
			continue
		}
		if p, ok := flagValue(fields, "-p"); ok && !strings.EqualFold(p, proto) {
			continue
		}
		if spec, ok := flagValue(fields, "--dport"); ok && portSpecRangeCovers(spec, lo, hi, ":") {
			return true
		}
		if spec, ok := flagValue(fields, "--dports"); ok {
			for _, item := range strings.Split(spec, ",") {
				if portSpecRangeCovers(item, lo, hi, ":") {
					return true
				}
			}
		}
	}
	return false
}

func hasJumpAccept(fields []string) bool {
	for i := 0; i+1 < len(fields); i++ {
		if fields[i] == "-j" {
			return strings.EqualFold(fields[i+1], "ACCEPT")
		}
	}
	return false
}

// flagValue 返回 flag 后面的值（`-p tcp` → "tcp"）。
func flagValue(fields []string, flag string) (string, bool) {
	for i := 0; i+1 < len(fields); i++ {
		if fields[i] == flag {
			return fields[i+1], true
		}
	}
	return "", false
}

// portSpecCovers 判断 "443" / "50001-50020" / "20000:30000" 是否覆盖 port。
// sep 为端口段分隔符：nft/ufw/firewalld 用 "-"，iptables 用 ":"。
func portSpecCovers(spec string, port int, sep string) bool {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return false
	}
	lo, hi, found := strings.Cut(spec, sep)
	if !found {
		v, err := strconv.Atoi(strings.TrimSpace(lo))
		return err == nil && v == port
	}
	low, err1 := strconv.Atoi(strings.TrimSpace(lo))
	high, err2 := strconv.Atoi(strings.TrimSpace(hi))
	if err1 != nil || err2 != nil {
		return false
	}
	return port >= low && port <= high
}

// portSpecRangeCovers 判断端口规则是否**完整覆盖** [lo,hi]（单端口规则只覆盖同值）。
func portSpecRangeCovers(spec string, lo, hi int, sep string) bool {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return false
	}
	a, b, found := strings.Cut(spec, sep)
	if !found {
		v, err := strconv.Atoi(spec)
		return err == nil && v == lo && lo == hi
	}
	x, err1 := strconv.Atoi(strings.TrimSpace(a))
	y, err2 := strconv.Atoi(strings.TrimSpace(b))
	if err1 != nil || err2 != nil {
		return false
	}
	return x <= lo && y >= hi
}

// ufwCoversPort 检查 `ufw status` 里是否已有覆盖该端口的 ALLOW 规则。
// 典型输出：`443/tcp                    ALLOW       Anywhere`。
func ufwCoversPort(port int, proto string) bool {
	out, err := runOut("ufw", "status")
	if err != nil {
		return false
	}
	return ufwStatusCoversPort(out, proto, port)
}

func ufwStatusCoversPort(output, proto string, port int) bool {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 2 {
			continue
		}
		// 第一列形如 443/tcp 或 50001:50020/tcp（ufw 的段用冒号）
		spec, specProto, ok := strings.Cut(fields[0], "/")
		if !ok || !strings.EqualFold(specProto, proto) {
			continue
		}
		allowed := false
		for _, f := range fields[1:] {
			if strings.EqualFold(f, "ALLOW") {
				allowed = true
				break
			}
		}
		if !allowed {
			continue
		}
		if portSpecCovers(spec, port, ":") || portSpecCovers(spec, port, "-") {
			return true
		}
	}
	return false
}

func ufwStatusCoversRange(proto string, lo, hi int) bool {
	out, err := runOut("ufw", "status")
	if err != nil {
		return false
	}
	return ufwStatusRangeCovers(out, proto, lo, hi)
}

func ufwStatusRangeCovers(output, proto string, lo, hi int) bool {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 2 {
			continue
		}
		spec, specProto, ok := strings.Cut(fields[0], "/")
		if !ok || !strings.EqualFold(specProto, proto) {
			continue
		}
		allowed := false
		for _, f := range fields[1:] {
			if strings.EqualFold(f, "ALLOW") {
				allowed = true
				break
			}
		}
		if !allowed {
			continue
		}
		if portSpecRangeCovers(spec, lo, hi, ":") || portSpecRangeCovers(spec, lo, hi, "-") {
			return true
		}
	}
	return false
}

func removeProto(kind string, port int, proto string) error {
	switch kind {
	case KindFirewalld:
		return firewalldPort("--remove-port", port, proto)
	case KindUFW:
		// 老版本每次启动都会插一条，可能残留多条：删到失败为止。
		for i := 0; i < 16; i++ {
			out, err := runOut("ufw", "delete", "allow", fmt.Sprintf("%d/%s", port, proto))
			if err == nil {
				continue
			}
			lower := strings.ToLower(out)
			if strings.Contains(lower, "could not delete") || strings.Contains(lower, "not found") {
				return nil
			}
			return err
		}
		return nil
	case KindNftables:
		return nftRemove(port, proto)
	case KindIptables:
		for i := 0; i < 16; i++ {
			out, err := runOut("iptables", "-w", "-D", "INPUT", "-p", proto, "--dport", strconv.Itoa(port), "-j", "ACCEPT")
			if err == nil {
				continue
			}
			lower := strings.ToLower(out)
			if strings.Contains(lower, "bad rule") || strings.Contains(lower, "matching rule") {
				return nil
			}
			return err
		}
		return nil
	}
	return nil
}

func firewalldPort(op string, port int, proto string) error {
	return firewalldPortSpec(op, strconv.Itoa(port), proto)
}

// firewalldPortSpec 用 "443" / "20000-50000" 形式的端口规格操作 firewalld。
func firewalldPortSpec(op, spec, proto string) error {
	full := spec + "/" + proto
	if op == "--add-port" {
		if out, err := runOut("firewall-cmd", "--permanent", "--query-port="+full); err == nil && strings.EqualFold(out, "yes") {
			return nil // 精确匹配已放行
		}
		// 已被范围规则覆盖时同样跳过（否则每次调用都会多一个单端口条目）
		if out, err := runOut("firewall-cmd", "--permanent", "--list-ports"); err == nil && firewalldListCovers(out, spec, proto) {
			return nil
		}
	}
	if _, err := runOut("firewall-cmd", "--permanent", op+"="+full); err != nil {
		return err
	}
	_, err := runOut("firewall-cmd", "--reload")
	return err
}

// firewalldListCovers 判断 `firewall-cmd --list-ports` 输出里是否已有覆盖目标规格的条目。
// 输出形如：`443/tcp 20000-50000/udp`。
func firewalldListCovers(output, spec, proto string) bool {
	wantLo, wantHi, ok := parsePortRangeToken(spec)
	if !ok {
		return false
	}
	for _, item := range strings.Fields(output) {
		ports, itemProto, found := strings.Cut(item, "/")
		if !found || !strings.EqualFold(itemProto, proto) {
			continue
		}
		if wantLo == wantHi {
			if portSpecCovers(ports, wantLo, "-") {
				return true
			}
			continue
		}
		if portSpecRangeCovers(ports, wantLo, wantHi, "-") {
			return true
		}
	}
	return false
}

// nftAllow 把 accept 规则加到已有的 input 过滤链；没有现成链时建一张专用表。
// 之所以优先复用 inet/ip filter：nftables 的多条 base chain 独立判决，
// drop 链里拒绝的包不会因为我们另建的 accept 链而放行。
func nftAllow(port int, proto string) error {
	chain := nftEnsureInputChain()
	parts := strings.Split(chain, " ")
	if out, err := runOut("nft", append([]string{"-a", "list", "chain"}, parts...)...); err == nil {
		if nftOutputCoversPort(out, proto, port) {
			handles := nftExactPortHandles(out, proto, port)
			if len(handles) == 0 {
				return nil // 仅被管理员自建的范围/集合规则覆盖，不动它
			}
			// 已有唯一且位于兜底 drop 之前的规则：直接返回。
			if len(handles) == 1 && nftRuleBeforeDeny(out, handles[0]) {
				return nil
			}
			// 历史重复（老版本每次启动 append 一条）或位置不对（append 在 drop 之后）：
			// 清掉我们自己的这些规则，下面统一重新插到 drop 之前，保证只剩一条且生效。
			_ = nftDeleteHandles(chain, handles)
		}
	}
	return nftInsertRule(chain, fmt.Sprintf("%s dport %d accept", proto, port))
}

// nftAllowRange 放行端口段（幂等，支持已有规则覆盖判断）。
func nftAllowRange(lo, hi int, proto string) error {
	chain := nftEnsureInputChain()
	parts := strings.Split(chain, " ")
	if out, err := runOut("nft", append([]string{"-a", "list", "chain"}, parts...)...); err == nil {
		if nftOutputCoversRange(out, proto, lo, hi) {
			handles := nftExactRangeHandles(out, proto, fmt.Sprintf("%d-%d", lo, hi))
			if len(handles) == 0 {
				return nil
			}
			if len(handles) == 1 && nftRuleBeforeDeny(out, handles[0]) {
				return nil
			}
			_ = nftDeleteHandles(chain, handles)
		}
	}
	return nftInsertRule(chain, fmt.Sprintf("%s dport %d-%d accept", proto, lo, hi))
}

// nftRuleBeforeDeny 判断指定 handle 的规则是否排在链里最后一条 drop/reject 之前。
// 排在兜底 drop 后面的 accept 永远匹配不到，需要重插。
func nftRuleBeforeDeny(output, handle string) bool {
	ruleIdx, denyIdx := -1, -1
	for i, line := range strings.Split(output, "\n") {
		if nftHandle(line) == handle {
			ruleIdx = i
		}
		if _, deny := nftDenyRule(line); deny {
			denyIdx = i
		}
	}
	if ruleIdx < 0 {
		return false
	}
	return denyIdx < 0 || ruleIdx < denyIdx
}

// nftEnsureInputChain 返回可用的 input 过滤链；没有现成链时建一张专用表。
func nftEnsureInputChain() string {
	if chain := nftInputChain(); chain != "" {
		return chain
	}
	runOut("nft", "add", "table", "inet", "corefusion_fw")
	runOut("nft", "add", "chain", "inet", "corefusion_fw", "input",
		"{ type filter hook input priority 0; policy accept; }")
	return "inet corefusion_fw input"
}

// nftOutputCoversPort 解析 `nft list chain` 的输出做覆盖判断（纯函数，便于单测）。
// 支持 `tcp dport 443 accept`、`tcp dport 20000-30000 accept`、
// `tcp dport { 443, 8443 } accept` 三种常见写法。
func nftOutputCoversPort(output, proto string, port int) bool {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if !hasToken(fields, "accept") || !nftLineMatchesProto(fields, proto) {
			continue
		}
		for i, f := range fields {
			if f != "dport" {
				continue
			}
			for _, item := range nftPortTokens(fields[i+1:]) {
				if portSpecCovers(item, port, "-") {
					return true
				}
			}
		}
	}
	return false
}

// nftOutputCoversRange 判断已有 accept 规则是否完整覆盖 [lo,hi]。
// 支持 `tcp dport 20000-50000 accept`、`tcp dport 1024-65535 accept`、集合写法。
func nftOutputCoversRange(output, proto string, lo, hi int) bool {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if !hasToken(fields, "accept") || !nftLineMatchesProto(fields, proto) {
			continue
		}
		for i, f := range fields {
			if f != "dport" {
				continue
			}
			for _, item := range nftPortTokens(fields[i+1:]) {
				if portSpecRangeCovers(item, lo, hi, "-") {
					return true
				}
			}
		}
	}
	return false
}

func hasToken(fields []string, token string) bool {
	for _, f := range fields {
		if strings.EqualFold(f, token) {
			return true
		}
	}
	return false
}

func nftLineMatchesProto(fields []string, proto string) bool {
	if hasToken(fields, proto) {
		return true
	}
	// `meta l4proto tcp th dport 443 accept`
	for i := 0; i+1 < len(fields); i++ {
		if fields[i] == "l4proto" && strings.EqualFold(fields[i+1], proto) {
			return true
		}
	}
	return false
}

// nftPortTokens 从 dport 后面收集端口字面量（去掉集合的 { } 与逗号）。
func nftPortTokens(fields []string) []string {
	out := make([]string, 0, 4)
	for _, f := range fields {
		t := strings.Trim(f, "{}, ")
		if t == "" {
			continue
		}
		ok := true
		for _, r := range t {
			if (r < '0' || r > '9') && r != '-' && r != ':' {
				ok = false
				break
			}
		}
		if !ok {
			break // 遇到 accept/counter 等关键字，端口部分结束
		}
		out = append(out, t)
	}
	return out
}

// nftInsertRule 把规则插到链里「最后一条 drop/reject」之前。
//
// 不能直接 append：常见发行版/防火墙模板的 input 链末尾是
// `counter ... drop`（默认拒绝），append 的 accept 永远匹配不到。
// 没有 drop 规则时才退化为 append。
func nftInsertRule(chain, rule string) error {
	parts := strings.Split(chain, " ")
	out, _ := runOut("nft", append([]string{"-a", "list", "chain"}, parts...)...)
	position := ""
	for _, line := range strings.Split(out, "\n") {
		handle, deny := nftDenyRule(line)
		if deny {
			position = handle // 取最后一条（兜底 drop）
		}
	}
	args := append([]string{"insert", "rule"}, parts...)
	if position != "" {
		args = append(args, "position", position)
	}
	args = append(args, strings.Fields(rule)...)
	_, err := runOut("nft", args...)
	return err
}

// nftDenyRule 判断一条 `nft -a list chain` 输出是否是 drop/reject 规则，
// 并取出它的 handle。
//
// 注意不能对整行做 HasSuffix("drop")：带 -a 时行尾是 `# handle 12` 注释，
// 真正的判定目标是注释之前的规则体（常见形态 `counter packets 0 bytes 0 drop`）。
func nftDenyRule(line string) (handle string, deny bool) {
	t := strings.TrimSpace(line)
	if t == "" {
		return "", false
	}
	// 去掉 " # handle N" 注释
	body := t
	if idx := strings.Index(t, "#"); idx >= 0 {
		body = strings.TrimSpace(t[:idx])
		handle = nftHandle(t[idx:])
	}
	if handle == "" {
		handle = nftHandle(t)
	}
	fields := strings.Fields(body)
	if len(fields) == 0 {
		return "", false
	}
	last := strings.ToLower(fields[len(fields)-1])
	if last != "drop" && last != "reject" {
		return "", false
	}
	if handle == "" {
		return "", false
	}
	if _, err := strconv.ParseUint(handle, 10, 64); err != nil {
		return "", false
	}
	return handle, true
}

// nftInputChain 返回现成的 input base chain（"inet filter input" / "ip filter input"）。
func nftInputChain() string {
	for _, chain := range []string{"inet filter input", "ip filter input"} {
		args := append([]string{"list", "chain"}, strings.Split(chain, " ")...)
		if _, err := runOut("nft", args...); err == nil {
			return chain
		}
	}
	return ""
}

func nftRemove(port int, proto string) error {
	chains := []string{"inet filter input", "ip filter input", "inet corefusion_fw input"}
	for _, chain := range chains {
		if err := nftDeleteMatching(chain, port, proto); err != nil {
			return err
		}
	}
	return nil
}

func nftRemoveRange(lo, hi int, proto string) error {
	chains := []string{"inet filter input", "ip filter input", "inet corefusion_fw input"}
	for _, chain := range chains {
		if err := nftDeleteRangeMatching(chain, lo, hi, proto); err != nil {
			return err
		}
	}
	return nil
}

// nftDeleteRangeMatching 删除我们添加的 `<proto> dport <lo>-<hi> accept` 规则（可能有多条）。
func nftDeleteRangeMatching(chain string, lo, hi int, proto string) error {
	parts := strings.Split(chain, " ")
	out, err := runOut("nft", append([]string{"-a", "list", "chain"}, parts...)...)
	if err != nil {
		return nil // 链不存在
	}
	return nftDeleteHandles(chain, nftExactRangeHandles(out, proto, fmt.Sprintf("%d-%d", lo, hi)))
}

// nftExactRangeHandles 从 `nft -a list chain` 输出里取出指定端口段的 handle。
func nftExactRangeHandles(output, proto, spec string) []string {
	handles := make([]string, 0, 2)
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if !hasToken(fields, "accept") || !nftLineExactRange(fields, proto, spec) {
			continue
		}
		if handle := nftHandle(line); handle != "" {
			handles = append(handles, handle)
		}
	}
	return handles
}

// nftDeleteHandles 按 handle 删除规则；忽略单条删除失败（规则可能已被并发删除）。
func nftDeleteHandles(chain string, handles []string) error {
	parts := strings.Split(chain, " ")
	var lastErr error
	for _, handle := range handles {
		del := append([]string{"delete", "rule"}, parts...)
		del = append(del, "handle", handle)
		if _, err := runOut("nft", del...); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

func nftLineExactRange(fields []string, proto, spec string) bool {
	if !nftLineMatchesProto(fields, proto) {
		return false
	}
	for i, f := range fields {
		if f != "dport" {
			continue
		}
		toks := nftPortTokens(fields[i+1:])
		return len(toks) == 1 && toks[0] == spec
	}
	return false
}

// nftDeleteMatching 删除指定链里由我们添加的 `<proto> dport <port> accept` 规则（按 handle）。
// 老版本可能留下多条重复规则，这里会全部删掉；管理员自建的端口段/集合规则不动。
func nftDeleteMatching(chain string, port int, proto string) error {
	parts := strings.Split(chain, " ")
	args := append([]string{"-a", "list", "chain"}, parts...)
	out, err := runOut("nft", args...)
	if err != nil {
		return nil // 链不存在 = 没有需要删除的规则
	}
	return nftDeleteHandles(chain, nftExactPortHandles(out, proto, port))
}

// nftExactPortHandles 从 `nft -a list chain` 输出里取出我们添加的
// `<proto> dport <port> accept` 规则的 handle（不含管理员自建的范围/集合规则）。
func nftExactPortHandles(output, proto string, port int) []string {
	handles := make([]string, 0, 2)
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if !hasToken(fields, "accept") || !nftLineExactPort(fields, proto, port) {
			continue
		}
		if handle := nftHandle(line); handle != "" {
			handles = append(handles, handle)
		}
	}
	return handles
}

func nftLineExactPort(fields []string, proto string, port int) bool {
	if !nftLineMatchesProto(fields, proto) {
		return false
	}
	for i, f := range fields {
		if f != "dport" {
			continue
		}
		toks := nftPortTokens(fields[i+1:])
		if len(toks) != 1 {
			return false // 集合/端口段不是我们生成的规则
		}
		v, err := strconv.Atoi(toks[0])
		return err == nil && v == port
	}
	return false
}

func nftHandle(line string) string {
	idx := strings.LastIndex(line, "handle ")
	if idx < 0 {
		return ""
	}
	handle := strings.TrimSpace(line[idx+len("handle "):])
	if _, err := strconv.ParseUint(handle, 10, 64); err != nil {
		return ""
	}
	return handle
}

// ParseAllowFromConfig 从入站 config_json 里读取「自动创建防火墙放行规则」标记。
// enabled=true 时返回要放行的协议（空表示 TCP+UDP）。
func ParseAllowFromConfig(configJSON string) (enabled bool, network string) {
	cfg, err := decodeConfig(configJSON)
	if err != nil {
		return false, ""
	}
	v, _ := cfg["_fwAllow"].(bool)
	if !v {
		return false, ""
	}
	// 老数据里的 Unix Socket 入站（listen 列可能被写成 0.0.0.0）也不能创建放行规则
	if lt, _ := cfg["_listenType"].(string); strings.EqualFold(lt, "sock") {
		return false, ""
	}
	network, _ = cfg["_fwNetwork"].(string)
	return true, network
}

// IsUnixSocketListen 判断监听地址是否为 Unix domain socket
// （绝对/相对路径、@abstract、带 unix: 前缀）。
// 这类入站不占 TCP 端口，无需也不能创建防火墙放行规则：面板里的 port
// 只是订阅/分享用的标识端口。
func IsUnixSocketListen(listen string) bool {
	l := strings.ToLower(strings.TrimSpace(listen))
	if l == "" {
		return false
	}
	if strings.HasPrefix(l, "@") || strings.HasPrefix(l, "unix:") {
		return true
	}
	return strings.Contains(l, "/")
}

func decodeConfig(configJSON string) (map[string]interface{}, error) {
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
