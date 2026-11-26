package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"corefusion/internal/model/nodecheckup"
	agentpb "corefusion/internal/runtime/agent/proto"
)

// defaultSysctlKeys 是体检白名单。面板可以覆盖，但不允许任意 key 落到 Agent。
var defaultSysctlKeys = []string{
	"net.ipv4.ip_forward",
	"net.ipv4.tcp_syncookies",
	"net.ipv4.tcp_fastopen",
	"net.ipv4.tcp_max_syn_backlog",
	"net.ipv4.tcp_max_tw_buckets",
	"net.ipv4.tcp_tw_reuse",
	"net.ipv4.tcp_fin_timeout",
	"net.ipv4.tcp_mtu_probing",
	"net.ipv4.tcp_congestion_control",
	"net.ipv4.tcp_available_congestion_control",
	"net.ipv4.tcp_rmem",
	"net.ipv4.tcp_wmem",
	"net.ipv4.tcp_keepalive_time",
	"net.ipv4.ip_local_port_range",
	"net.ipv4.udp_mem",
	"net.ipv4.udp_rmem_min",
	"net.ipv4.udp_wmem_min",
	"net.ipv4.tcp_notsent_lowat",
	"net.ipv4.tcp_slow_start_after_idle",
	"net.ipv4.tcp_keepalive_intvl",
	"net.ipv4.tcp_keepalive_probes",
	"net.ipv4.tcp_migrate_req",
	"net.ipv4.conf.all.rp_filter",
	"net.ipv4.conf.default.rp_filter",
	"net.ipv4.conf.all.accept_redirects",
	"net.ipv4.conf.all.send_redirects",
	"net.ipv4.conf.all.accept_source_route",
	"net.ipv6.conf.all.forwarding",
	"net.ipv6.conf.all.accept_redirects",
	"net.ipv6.conf.all.accept_source_route",
	"net.ipv6.conf.all.accept_ra",
	"net.ipv6.conf.all.use_tempaddr",
	"net.ipv6.neigh.default.gc_thresh1",
	"net.ipv6.neigh.default.gc_thresh2",
	"net.ipv6.neigh.default.gc_thresh3",
	"net.ipv6.route.max_size",
	"net.core.somaxconn",
	"net.core.rmem_max",
	"net.core.wmem_max",
	"net.core.rmem_default",
	"net.core.wmem_default",
	"net.core.netdev_max_backlog",
	"net.core.netdev_budget",
	"net.core.netdev_budget_usecs",
	"net.core.default_qdisc",
	"net.netfilter.nf_conntrack_max",
	"net.netfilter.nf_conntrack_buckets",
	"net.netfilter.nf_conntrack_tcp_timeout_established",
	"net.netfilter.nf_conntrack_udp_timeout",
	"net.mptcp.enabled",
	"fs.file-max",
	"fs.nr_open",
	"vm.swappiness",
	"kernel.pid_max",
}

// hostGlobalKeys 在容器内也由宿主内核统一决定，体检只给宿主命令，不判错。
var hostGlobalKeys = map[string]bool{
	"fs.file-max":    true,
	"vm.swappiness":  true,
	"kernel.pid_max": true,
}

// CollectSystemFacts 只读采集本机事实。禁止写文件、改 sysctl、加载模块或重启服务。
func CollectSystemFacts(req *agentpb.GetSystemFactsRequest) *agentpb.GetSystemFactsResponse {
	if req == nil {
		req = &agentpb.GetSystemFactsRequest{}
	}
	facts := nodecheckup.Facts{
		Env:       collectEnv(),
		Sysctl:    map[string]nodecheckup.Sysctl{},
		Modules:   map[string]bool{},
		Listeners: []nodecheckup.Listener{},
	}
	keys := req.GetSysctlKeys()
	if len(keys) == 0 {
		keys = defaultSysctlKeys
	}
	wanted := make(map[string]bool, len(keys))
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" || strings.ContainsAny(key, " /\\") {
			continue
		}
		wanted[key] = true
		facts.Sysctl[key] = collectSysctl(key, facts.Env)
	}
	facts.SysctlFiles = collectSysctlFiles(wanted)
	facts.IPv6 = collectIPv6()
	facts.Modules = collectModules()
	if req.GetIncludeSsh() {
		facts.SSHD = collectSSHD()
	}
	facts.Listeners = collectListeners()
	if req.GetIncludeFirewallRules() {
		facts.Firewall = collectFirewall()
	}
	facts.Resources = collectResources()
	facts.TimeSync = collectTimeSync()

	data, err := json.Marshal(facts)
	if err != nil {
		return &agentpb.GetSystemFactsResponse{Success: false, Error: err.Error()}
	}
	return &agentpb.GetSystemFactsResponse{Success: true, Facts: string(data)}
}

// collectIPv6 返回全局 IPv6 地址（排除回环与 link-local），只用于判断
// 节点是否真的在用 IPv6，避免在没有 IPv6 的机器上推荐 IPv6 参数。
func collectIPv6() []string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var out []string
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipnet, ok := a.(*net.IPNet)
			if !ok || ipnet.IP.To4() != nil {
				continue
			}
			ip := ipnet.IP
			if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
				continue
			}
			if ip.IsGlobalUnicast() {
				out = append(out, ip.String())
			}
		}
	}
	sort.Strings(out)
	return out
}

func collectEnv() nodecheckup.Env {
	env := nodecheckup.Env{
		Kind:    "host",
		OS:      readOSName(),
		Kernel:  strings.TrimSpace(readFileString("/proc/sys/kernel/osrelease")),
		Systemd: dirExists("/run/systemd/system"),
		Root:    os.Geteuid() == 0,
	}
	if runtime.GOOS != "linux" {
		env.Kind = "unknown"
		return env
	}
	container, runtimeName := detectContainer()
	if container {
		env.Kind = "container"
		env.Runtime = runtimeName
	}
	env.CapNetAdmin = hasCapNetAdmin()
	return env
}

func detectContainer() (bool, string) {
	if fileExists("/.dockerenv") {
		return true, "docker"
	}
	if fileExists("/run/.containerenv") {
		return true, "podman"
	}
	cg := readFileString("/proc/1/cgroup")
	switch {
	case strings.Contains(cg, "docker"):
		return true, "docker"
	case strings.Contains(cg, "kubepods"):
		return true, "k8s"
	case strings.Contains(cg, "lxc"):
		return true, "lxc"
	case strings.Contains(cg, "containerd"):
		return true, "containerd"
	}
	if out, err := runCommand(2*time.Second, "systemd-detect-virt", "--container"); err == nil {
		name := strings.TrimSpace(out)
		if name != "" && name != "none" {
			return true, name
		}
	}
	return false, ""
}

func hasCapNetAdmin() bool {
	status := readFileString("/proc/self/status")
	for _, line := range strings.Split(status, "\n") {
		if !strings.HasPrefix(line, "CapEff:") {
			continue
		}
		raw := strings.TrimSpace(strings.TrimPrefix(line, "CapEff:"))
		v, err := strconv.ParseUint(raw, 16, 64)
		if err != nil {
			return false
		}
		// CAP_NET_ADMIN = 12
		return v&(1<<12) != 0
	}
	return false
}

func collectSysctl(key string, env nodecheckup.Env) nodecheckup.Sysctl {
	path := "/proc/sys/" + strings.ReplaceAll(key, ".", "/")
	value, err := os.ReadFile(path)
	if err != nil {
		return nodecheckup.Sysctl{Scope: "na"}
	}
	fact := nodecheckup.Sysctl{Value: strings.TrimSpace(string(value)), Scope: "host"}
	if env.Kind == "container" {
		fact.Scope = "container-writable"
		if hostGlobalKeys[key] {
			fact.Scope = "host-global"
		}
	}
	if procSysReadOnly() {
		fact.Writable = false
		if fact.Scope == "container-writable" {
			fact.Scope = "container-readonly"
		}
		return fact
	}
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err == nil {
		_ = f.Close()
		fact.Writable = true
		return fact
	}
	fact.Writable = false
	if fact.Scope == "container-writable" {
		fact.Scope = "container-readonly"
	}
	return fact
}

func procSysReadOnly() bool {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) >= 4 && fields[1] == "/proc/sys" {
			return strings.Contains(","+fields[3]+",", ",ro,")
		}
	}
	return false
}

func collectModules() map[string]bool {
	out := map[string]bool{"tcp_bbr": false, "nf_conntrack": false}
	f, err := os.Open("/proc/modules")
	if err != nil {
		// 模块可能编译进内核，/proc/modules 为空；从 config 再判断一次。
		if cfg := readFileString("/boot/config-" + strings.TrimSpace(readFileString("/proc/sys/kernel/osrelease"))); cfg != "" {
			for k := range out {
				if strings.Contains(cfg, "CONFIG_"+strings.ToUpper(k)+"=y") {
					out[k] = true
				}
			}
		}
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		name := strings.Fields(sc.Text())
		if len(name) == 0 {
			continue
		}
		if _, ok := out[name[0]]; ok {
			out[name[0]] = true
		}
	}
	return out
}

func collectSSHD() *nodecheckup.SSHD {
	res := &nodecheckup.SSHD{Present: false, Effective: map[string]string{}, Users: map[string]map[string]string{}, Keys: map[string]nodecheckup.SSHKey{}}
	path, err := exec.LookPath("sshd")
	if err != nil {
		return res
	}
	res.Present = true
	out, err := runCommand(5*time.Second, path, "-T")
	if err != nil {
		res.Error = truncate(err.Error(), 300)
		return res
	}
	res.Effective = parseSSHDTest(out)
	res.Port = atoiSafe(res.Effective["port"])
	for _, user := range []string{"root"} {
		if uo, uerr := runCommand(5*time.Second, path, "-T", "-C", "user="+user); uerr == nil {
			res.Users[user] = parseSSHDTest(uo)
		}
	}
	for _, user := range []string{"root"} {
		home := "/root"
		if user != "root" {
			home = filepath.Join("/home", user)
		}
		key := nodecheckup.SSHKey{Owner: user}
		sshDir := filepath.Join(home, ".ssh")
		authKeys := filepath.Join(sshDir, "authorized_keys")
		key.SSHDirMode = fileModeString(sshDir)
		key.AuthorizedKeysMode = fileModeString(authKeys)
		key.HasKey = fileExists(authKeys)
		key.OwnerOK = ownerMatches(sshDir, user)
		// chattr +i 会让 root 也无法 chmod/chown（Operation not permitted），
		// 体检建议里必须先解锁，否则用户照抄命令会失败。
		key.Immutable = fileImmutable(authKeys)
		res.Keys[user] = key
	}
	res.IncludeLine, res.ConfigHits = collectSSHDConfigHits()
	return res
}

// fileImmutable 用 lsattr 判断文件是否带 immutable 属性（i）。
// 非 ext4 / 没有 lsattr 时返回 false，不影响其它判定。
func fileImmutable(path string) bool {
	out, err := runCommand(3*time.Second, "lsattr", "-d", path)
	if err != nil {
		return false
	}
	fields := strings.Fields(out)
	if len(fields) == 0 {
		return false
	}
	return strings.Contains(fields[0], "i")
}

func parseSSHDTest(out string) map[string]string {
	m := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}
		m[strings.ToLower(parts[0])] = strings.TrimSpace(parts[1])
	}
	return m
}

func collectListeners() []nodecheckup.Listener {
	out, err := runCommand(5*time.Second, "ss", "-lntu")
	if err != nil {
		return nil
	}
	var res []nodecheckup.Listener
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 || fields[0] == "Netid" {
			continue
		}
		local := fields[4]
		idx := strings.LastIndex(local, ":")
		if idx < 0 {
			continue
		}
		addr := strings.Trim(local[:idx], "[]")
		port := atoiSafe(local[idx+1:])
		if port <= 0 {
			continue
		}
		res = append(res, nodecheckup.Listener{Proto: fields[0], Addr: addr, Port: port})
	}
	return res
}

func collectFirewall() *nodecheckup.Firewall {
	fw := &nodecheckup.Firewall{DefaultInbound: "unknown"}

	// firewalld 必须最先判：它可能使用 nftables 或 iptables 作为后端，
	// 直接看 `nft list ruleset` / `iptables -S` 会把 firewalld 自己生成的
	// 表/链误判成 nftables/iptables（RHEL 9/10 上很常见）。
	// `--state` 在未运行时返回非 0 且输出 "not running"，所以要求进程成功且文本就是 running。
	if out, err := runCommand(5*time.Second, "firewall-cmd", "--state"); err == nil &&
		strings.EqualFold(strings.TrimSpace(out), "running") {
		fw.Type = "firewalld"
		fw.Enabled = true
		if all, lerr := runCommand(5*time.Second, "firewall-cmd", "--list-all"); lerr == nil {
			raw := all
			// rich rule 才带 source 限制；只取 --list-all 会看不到来源白名单。
			if rich, rerr := runCommand(5*time.Second, "firewall-cmd", "--list-rich-rules"); rerr == nil && strings.TrimSpace(rich) != "" {
				raw = all + "\n" + rich
			}
			fw.RulesRaw = truncate(raw, 8192)
			fw.DefaultInbound = parseFirewalldTarget(all)
		}
		return fw
	}

	// ufw 只有 active 才算启用；inactive 时继续看底层 nft/iptables。
	if out, err := runCommand(5*time.Second, "ufw", "status"); err == nil &&
		strings.Contains(out, "Status: active") {
		fw.Type = "ufw"
		fw.Enabled = true
		fw.RulesRaw = truncate(out, 8192)
		if strings.Contains(out, "Default: deny (incoming)") {
			fw.DefaultInbound = "drop"
		} else if strings.Contains(out, "Default: allow (incoming)") {
			fw.DefaultInbound = "accept"
		}
		return fw
	}

	// nft：只有 input 链里存在真正的过滤规则才算"启用的防火墙"。
	// 否则像 corefusion 的 ratelimit 记账表（无 hook input）、
	// podman/netavark 的 `jump NETAVARK_INPUT` 也会被误判成 nftables。
	if out, err := runCommand(5*time.Second, "nft", "list", "ruleset"); err == nil &&
		strings.TrimSpace(out) != "" && nftInputHasEffectiveRules(out) {
		fw.Type = "nftables"
		fw.Enabled = true
		fw.RulesRaw = truncate(out, 8192)
		fw.DefaultInbound = parseNFTPolicy(out)
		return fw
	}

	// iptables：`-S` 一定能看到 `-P INPUT`，不能据此判定"有防火墙"。
	// 只有 INPUT 策略非 ACCEPT，或存在真正的 `-A INPUT` 规则时才算启用。
	if out, err := runCommand(5*time.Second, "iptables", "-S"); err == nil && strings.TrimSpace(out) != "" {
		fw.Type = "iptables"
		fw.RulesRaw = truncate(out, 8192)
		switch {
		case strings.Contains(out, "-P INPUT DROP"), strings.Contains(out, "-P INPUT REJECT"):
			fw.Enabled = true
			fw.DefaultInbound = "drop"
		case hasIptablesInputRules(out):
			fw.Enabled = true
			fw.DefaultInbound = "accept"
		default:
			// 全 ACCEPT 且无 INPUT 规则 = 没有实际入站过滤。
			fw.Enabled = false
			fw.DefaultInbound = "accept"
		}
		return fw
	}
	return fw
}

// nftInputHasEffectiveRules 判断 input hook 链里是否有主机防火墙规则。
//
// 容器运行时（podman/netavark、docker、k8s）也会建 input 链并 jump 到自己的链，
// 那不是主机防火墙。只看 `hook input` 会把这类环境误判成 nftables 已启用，
// 进而给出"默认 accept"的误导结论。
func nftInputHasEffectiveRules(rules string) bool {
	inInput := false
	baseIndent := 0
	for _, ln := range strings.Split(rules, "\n") {
		trimmed := strings.TrimSpace(ln)
		if !inInput {
			if strings.Contains(strings.ToLower(trimmed), "hook input") {
				inInput = true
				baseIndent = leadingTabs(ln)
			}
			continue
		}
		if trimmed == "" {
			continue
		}
		// 链块结束（与 chain 行同缩进的 `}`）
		if strings.HasPrefix(trimmed, "}") && leadingTabs(ln) <= baseIndent {
			inInput = false
			continue
		}
		if strings.HasPrefix(trimmed, "type ") {
			continue
		}
		if isContainerNetRule(trimmed) {
			continue
		}
		if strings.Contains(trimmed, " accept") || strings.Contains(trimmed, " drop") ||
			strings.Contains(trimmed, " reject") || strings.Contains(trimmed, " jump ") ||
			strings.Contains(trimmed, " goto ") || strings.Contains(trimmed, " return") {
			return true
		}
	}
	return false
}

func leadingTabs(s string) int {
	n := 0
	for _, r := range s {
		if r != '\t' {
			break
		}
		n++
	}
	return n
}

// isContainerNetRule 判断规则是否只是容器网络运行时的转发（不算主机防火墙）。
func isContainerNetRule(s string) bool {
	l := strings.ToLower(s)
	for _, p := range []string{"netavark", "docker", "podman", "kube", "cni-"} {
		if strings.Contains(l, p) {
			return true
		}
	}
	return false
}

// hasIptablesInputRules 判断是否有真正的主机 INPUT 链规则：
// `-P`/`-N` 和容器运行时的跳转（netavark/docker/podman/k8s）都不算。
func hasIptablesInputRules(rules string) bool {
	for _, line := range strings.Split(rules, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "-A INPUT ") {
			continue
		}
		if isContainerNetRule(line) {
			continue
		}
		return true
	}
	return false
}

// parseFirewalldTarget 解析 `firewall-cmd --list-all` 的 target 行。
// target=default 表示按 zone 规则处理，未放行的入站默认被拒绝，按 drop 计。
func parseFirewalldTarget(rules string) string {
	for _, line := range strings.Split(rules, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(strings.ToLower(line), "target:") {
			continue
		}
		target := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(strings.ToLower(line), "target:")))
		switch {
		case strings.Contains(target, "drop"), strings.Contains(target, "reject"):
			return "drop"
		case strings.Contains(target, "accept"):
			return "accept"
		default:
			return "drop"
		}
	}
	return "unknown"
}

func parseNFTPolicy(rules string) string {
	lower := strings.ToLower(rules)
	if !strings.Contains(lower, "hook input") {
		return "unknown"
	}
	if strings.Contains(lower, "policy drop") {
		return "drop"
	}
	if strings.Contains(lower, "policy accept") {
		return "accept"
	}
	return "unknown"
}

func collectResources() *nodecheckup.Resources {
	r := &nodecheckup.Resources{DiskPath: "/"}
	mem := readFileString("/proc/meminfo")
	for _, line := range strings.Split(mem, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		switch strings.TrimSuffix(fields[0], ":") {
		case "MemTotal":
			r.MemTotalKB = int64(atoiSafe(fields[1]))
		case "MemAvailable":
			r.MemAvailableKB = int64(atoiSafe(fields[1]))
		}
	}
	var st syscall.Statfs_t
	if err := syscall.Statfs("/", &st); err == nil {
		r.DiskTotalKB = int64(st.Blocks) * int64(st.Bsize) / 1024
		r.DiskFreeKB = int64(st.Bavail) * int64(st.Bsize) / 1024
	}
	if load := strings.Fields(readFileString("/proc/loadavg")); len(load) > 0 {
		if v, err := strconv.ParseFloat(load[0], 64); err == nil {
			r.Load1 = v
		}
	}
	var rl syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rl); err == nil {
		r.NoFileSoft = uint64(rl.Cur)
		r.NoFileHard = uint64(rl.Max)
	}
	return r
}

func collectTimeSync() *nodecheckup.TimeSync {
	out, err := runCommand(2*time.Second, "timedatectl", "show", "-p", "NTPSynchronized", "--value")
	if err != nil {
		return nil
	}
	return &nodecheckup.TimeSync{Service: "timedatectl", Synchronized: strings.TrimSpace(out) == "yes"}
}

// ---- helpers ----

func runCommand(timeout time.Duration, name string, args ...string) (string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func readOSName() string {
	data := readFileString("/etc/os-release")
	for _, line := range strings.Split(data, "\n") {
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			return strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), `"`)
		}
	}
	return runtime.GOOS
}

func readFileString(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func dirExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}

func fileModeString(path string) string {
	st, err := os.Stat(path)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%04o", st.Mode().Perm())
}

func ownerMatches(path, user string) bool {
	st, err := os.Stat(path)
	if err != nil {
		return false
	}
	stat, ok := st.Sys().(*syscall.Stat_t)
	if !ok {
		return true
	}
	uid := os.Getuid()
	if user == "root" {
		return stat.Uid == 0
	}
	return int(stat.Uid) == uid
}

func atoiSafe(s string) int {
	v, _ := strconv.Atoi(strings.TrimSpace(s))
	return v
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// sysctlConfigDirs 是 systemd-sysctl / sysctl --system 的加载目录（按顺序）。
var sysctlConfigDirs = []string{
	"/etc/sysctl.d",
	"/run/sysctl.d",
	"/usr/local/lib/sysctl.d",
	"/usr/lib/sysctl.d",
	"/lib/sysctl.d",
}

// sysctlConfFile 总是最后加载，会覆盖 /etc/sysctl.d 里的同名 key。
const sysctlConfFile = "/etc/sysctl.conf"

// collectSysctlFiles 采集各 sysctl 配置文件中命中白名单 key 的行。
// 只读；顺序与 sysctl --system 一致，用于判断谁覆盖谁。
func collectSysctlFiles(wanted map[string]bool) []nodecheckup.ConfigHit {
	if len(wanted) == 0 {
		return nil
	}
	type cfgFile struct {
		path  string
		real  string
		order int
	}
	var files []cfgFile
	order := 0
	for _, dir := range sysctlConfigDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".conf") {
				continue
			}
			names = append(names, e.Name())
		}
		sort.Strings(names)
		for _, name := range names {
			order++
			p := filepath.Join(dir, name)
			real := p
			if r, err := filepath.EvalSymlinks(p); err == nil {
				real = r
			}
			files = append(files, cfgFile{path: p, real: real, order: order})
		}
	}
	order++
	realConf := sysctlConfFile
	if r, err := filepath.EvalSymlinks(sysctlConfFile); err == nil {
		realConf = r
	}
	files = append(files, cfgFile{path: sysctlConfFile, real: realConf, order: order})

	// /etc/sysctl.d/99-sysctl.conf 常常是 ../sysctl.conf 的软链：同一份内容
	// 只保留最后加载的那次（/etc/sysctl.conf），避免证据里出现重复行。
	lastIdx := make(map[string]int, len(files))
	for i, f := range files {
		lastIdx[f.real] = i
	}
	out := []nodecheckup.ConfigHit{}
	for i, f := range files {
		if lastIdx[f.real] != i {
			continue
		}
		out = append(out, parseSysctlConfigFile(f.path, wanted, f.order)...)
	}
	return out
}

func parseSysctlConfigFile(path string, wanted map[string]bool, order int) []nodecheckup.ConfigHit {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []nodecheckup.ConfigHit
	for i, raw := range strings.Split(string(data), "\n") {
		s := strings.TrimSpace(raw)
		if s == "" || strings.HasPrefix(s, "#") || strings.HasPrefix(s, ";") {
			continue
		}
		k, v, ok := strings.Cut(s, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		if strings.HasPrefix(k, "-") {
			k = strings.TrimSpace(strings.TrimPrefix(k, "-"))
		}
		if !wanted[k] {
			continue
		}
		out = append(out, nodecheckup.ConfigHit{File: path, Line: i + 1, Key: k, Value: strings.TrimSpace(v), Order: order})
	}
	return out
}

// sshdDirectiveKeys 是体检会修改的 sshd 指令（sshd 先匹配生效）。
var sshdDirectiveKeys = map[string]bool{
	"passwordauthentication":       true,
	"kbdinteractiveauthentication": true,
	"permitrootlogin":              true,
	"maxauthtries":                 true,
}

// collectSSHDConfigHits 采集主配置与 drop-in 的关键指令。
//
// sshd 是「第一次出现的值生效」，处理顺序为：主配置 Include 之前的行 →
// drop-in 目录按文件名字典序 → Include 之后的剩余行。Order 按这个顺序编号，
// 数值越小优先级越高。
func collectSSHDConfigHits() (int, []nodecheckup.ConfigHit) {
	const mainPath = "/etc/ssh/sshd_config"
	const dropDir = "/etc/ssh/sshd_config.d"

	includeLine := 0
	before := []nodecheckup.ConfigHit{}
	after := []nodecheckup.ConfigHit{}
	if data, err := os.ReadFile(mainPath); err == nil {
		for i, raw := range strings.Split(string(data), "\n") {
			ln := i + 1
			s := strings.TrimSpace(raw)
			if s == "" || strings.HasPrefix(s, "#") {
				continue
			}
			fields := strings.Fields(s)
			if len(fields) >= 2 && strings.EqualFold(fields[0], "Include") {
				if includeLine == 0 && strings.Contains(strings.Join(fields[1:], " "), "sshd_config.d") {
					includeLine = ln
				}
				continue
			}
			k := strings.ToLower(fields[0])
			if !sshdDirectiveKeys[k] || len(fields) < 2 {
				continue
			}
			hit := nodecheckup.ConfigHit{File: mainPath, Line: ln, Key: k, Value: strings.Join(fields[1:], " ")}
			if includeLine == 0 || ln < includeLine {
				hit.Order = ln
				before = append(before, hit)
			} else {
				hit.Order = 100000 + ln
				after = append(after, hit)
			}
		}
	}

	drops := []nodecheckup.ConfigHit{}
	if entries, err := os.ReadDir(dropDir); err == nil {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".conf") {
				continue
			}
			names = append(names, e.Name())
		}
		sort.Strings(names)
		for idx, name := range names {
			path := filepath.Join(dropDir, name)
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			for i, raw := range strings.Split(string(data), "\n") {
				s := strings.TrimSpace(raw)
				if s == "" || strings.HasPrefix(s, "#") {
					continue
				}
				fields := strings.Fields(s)
				k := strings.ToLower(fields[0])
				if !sshdDirectiveKeys[k] || len(fields) < 2 {
					continue
				}
				order := 10000 + idx
				if includeLine == 0 {
					// 没有 Include 时 drop-in 根本不生效，排到最后。
					order = 200000 + idx
				}
				drops = append(drops, nodecheckup.ConfigHit{
					File: path, Line: i + 1, Key: k,
					Value: strings.Join(fields[1:], " "), Order: order,
				})
			}
		}
	}

	out := make([]nodecheckup.ConfigHit, 0, len(before)+len(drops)+len(after))
	out = append(out, before...)
	out = append(out, drops...)
	out = append(out, after...)
	return includeLine, out
}
