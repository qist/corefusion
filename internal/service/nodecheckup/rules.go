package nodecheckup

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"corefusion/internal/model/nodecheckup"
)

const (
	SeverityHigh    = "high"
	SeverityWarn    = "warn"
	SeverityInfo    = "info"
	SeverityOK      = "ok"
	SeverityUnknown = "unknown"
)

// Judge 把只读事实转换为检查项。所有判定都在面板侧完成，Agent 不返回文案。
func Judge(facts *nodecheckup.Facts, th Thresholds) (nodecheckup.Env, nodecheckup.Summary, []nodecheckup.Item) {
	items := make([]nodecheckup.Item, 0, 20)
	summary := nodecheckup.Summary{}
	env := nodecheckup.Env{}
	if facts != nil {
		env = facts.Env
	}
	add := func(it nodecheckup.Item) {
		items = append(items, it)
		switch it.Severity {
		case SeverityHigh:
			summary.High++
		case SeverityWarn:
			summary.Warn++
		case SeverityInfo:
			summary.Info++
		case SeverityOK:
			summary.OK++
		default:
			summary.Unknown++
		}
	}

	envParams := map[string]any{
		"kind": env.Kind, "runtime": env.Runtime, "systemd": env.Systemd,
		"host_network": env.HostNetwork, "root": env.Root,
	}
	if facts != nil && facts.SSHD != nil && facts.SSHD.Port > 0 {
		envParams["ssh_port"] = facts.SSHD.Port
	}
	evidence := []string{"OS: " + env.OS, "Kernel: " + env.Kernel}
	if facts != nil && len(facts.IPv6) > 0 {
		envParams["ipv6"] = facts.IPv6
		evidence = append(evidence, "IPv6: "+strings.Join(facts.IPv6, ", "))
	}
	add(nodecheckup.Item{
		ID: "ENV-01", Category: "system", Severity: SeverityInfo, Scope: env.Kind,
		Params:   envParams,
		Evidence: evidence,
	})

	if facts == nil {
		return env, summary, items
	}

	// ---- 内核 / TCP ----
	if f, ok := facts.Sysctl["net.ipv4.ip_forward"]; !ok || f.Scope == "na" {
		add(unknownItem("SYS-01", "system", "net.ipv4.ip_forward"))
	} else if f.Value != "1" {
		add(nodecheckup.Item{
			ID: "SYS-01", Category: "system", Severity: SeverityWarn, Scope: f.Scope,
			Params:   map[string]any{"current": f.Value, "expected": "1"},
			Evidence: []string{"net.ipv4.ip_forward = " + f.Value},
			Commands: sysctlCommand("net.ipv4.ip_forward", "1", f.Scope),
		})
	} else {
		add(okItem("SYS-01", "system", f.Scope, "net.ipv4.ip_forward", f.Value))
	}

	if f, ok := facts.Sysctl["net.ipv4.tcp_syncookies"]; ok && f.Scope != "na" {
		if f.Value != "1" {
			add(nodecheckup.Item{
				ID: "SYS-02", Category: "system", Severity: SeverityWarn, Scope: f.Scope,
				Params:   map[string]any{"current": f.Value, "expected": "1"},
				Evidence: []string{"net.ipv4.tcp_syncookies = " + f.Value},
				Commands: sysctlCommand("net.ipv4.tcp_syncookies", "1", f.Scope),
			})
		} else {
			add(okItem("SYS-02", "system", f.Scope, "net.ipv4.tcp_syncookies", f.Value))
		}
	}

	if f, ok := facts.Sysctl["net.core.somaxconn"]; ok && f.Scope != "na" {
		cur := atoi(f.Value)
		if cur > 0 && cur < th.SomaxconnMin {
			add(nodecheckup.Item{
				ID: "SYS-03", Category: "tcp", Severity: SeverityWarn, Scope: f.Scope,
				Params:   map[string]any{"current": cur, "expected": th.SomaxconnMin},
				Evidence: []string{fmt.Sprintf("net.core.somaxconn = %d", cur)},
				Commands: sysctlCommand("net.core.somaxconn", strconv.Itoa(th.SomaxconnMin), f.Scope),
			})
		} else {
			add(okItem("SYS-03", "tcp", f.Scope, "net.core.somaxconn", f.Value))
		}
	}

	if f, ok := facts.Sysctl["net.netfilter.nf_conntrack_max"]; ok && f.Scope != "na" {
		cur := int64(atoi(f.Value))
		if cur > 0 && cur < th.ConntrackMin {
			add(nodecheckup.Item{
				ID: "SYS-04", Category: "network", Severity: SeverityWarn, Scope: f.Scope,
				Params:   map[string]any{"current": cur, "expected": th.ConntrackMin},
				Evidence: []string{fmt.Sprintf("nf_conntrack_max = %d", cur)},
				Commands: sysctlCommand("net.netfilter.nf_conntrack_max", fmt.Sprintf("%d", th.ConntrackMin), f.Scope),
			})
		} else if cur > 0 {
			add(okItem("SYS-04", "network", f.Scope, "net.netfilter.nf_conntrack_max", f.Value))
		}
	}

	// BBR 需要内核 >= 4.9（fq 需要 >= 4.13）。CentOS 7 的 3.10 内核根本没有 BBR，
	// 不能一律推荐——否则用户照着执行 sysctl 会得到 Invalid argument。
	bbrKernelOK := kernelAtLeast(env.Kernel, 4, 9)
	if f, ok := facts.Sysctl["net.ipv4.tcp_congestion_control"]; ok && f.Scope != "na" {
		available := facts.Sysctl["net.ipv4.tcp_available_congestion_control"].Value
		if strings.TrimSpace(f.Value) == "bbr" {
			add(okItem("SYS-05", "tcp", f.Scope, "net.ipv4.tcp_congestion_control", f.Value))
		} else if !bbrKernelOK {
			add(nodecheckup.Item{
				ID: "SYS-05", Category: "tcp", Severity: SeverityInfo, Scope: f.Scope,
				Params:   map[string]any{"current": f.Value, "kernel": env.Kernel},
				Evidence: []string{"kernel " + env.Kernel + " does not provide BBR (requires >= 4.9); keep the current algorithm or upgrade the kernel"},
			})
		} else if available == "" || hasWord(available, "bbr") {
			add(nodecheckup.Item{
				ID: "SYS-05", Category: "tcp", Severity: SeverityInfo, Scope: f.Scope,
				Params:   map[string]any{"current": f.Value, "expected": "bbr"},
				Evidence: []string{"tcp_congestion_control = " + f.Value},
				Commands: bbrCommand(f.Scope),
			})
		} else {
			// 内核支持 BBR，但模块没加载（available 里不含 bbr）：先 modprobe 再切换。
			add(nodecheckup.Item{
				ID: "SYS-05", Category: "tcp", Severity: SeverityInfo, Scope: f.Scope,
				Params:   map[string]any{"current": f.Value, "available": available},
				Evidence: []string{"tcp_congestion_control = " + f.Value, "available = " + available, "tcp_bbr module is not loaded; the kernel supports BBR (>= 4.9)"},
				Commands: bbrModuleScript(f.Scope),
			})
		}
	}
	// BBR 可能编译进内核，不能只看 /proc/modules；优先用 available_congestion_control。
	if f, ok := facts.Sysctl["net.ipv4.tcp_available_congestion_control"]; ok && f.Scope != "na" {
		if hasWord(f.Value, "bbr") {
			add(okItem("SYS-06", "tcp", f.Scope, "tcp_available_congestion_control", f.Value))
		} else if !bbrKernelOK {
			add(nodecheckup.Item{
				ID: "SYS-06", Category: "tcp", Severity: SeverityInfo, Scope: f.Scope,
				Params:   map[string]any{"available": f.Value, "kernel": env.Kernel},
				Evidence: []string{"kernel " + env.Kernel + " does not provide BBR (requires >= 4.9)"},
			})
		} else {
			add(nodecheckup.Item{
				ID: "SYS-06", Category: "tcp", Severity: SeverityInfo, Scope: f.Scope,
				Params:   map[string]any{"available": f.Value},
				Evidence: []string{"tcp_available_congestion_control = " + f.Value},
			})
		}
	} else if bbr, ok := facts.Modules["tcp_bbr"]; ok && !bbr {
		evidence := "tcp_bbr module not found"
		if !bbrKernelOK {
			evidence = "kernel " + env.Kernel + " does not provide BBR (requires >= 4.9)"
		}
		add(nodecheckup.Item{
			ID: "SYS-06", Category: "tcp", Severity: SeverityInfo, Scope: "na",
			Params:   map[string]any{"module": "tcp_bbr", "kernel": env.Kernel},
			Evidence: []string{evidence},
		})
	}

	// 容器内宿主全局参数：只提示，不判错。
	for _, key := range []string{"fs.file-max", "vm.swappiness", "kernel.pid_max"} {
		if f, ok := facts.Sysctl[key]; ok && f.Scope == "host-global" {
			add(nodecheckup.Item{
				ID: "SYS-07", Category: "system", Severity: SeverityInfo, Scope: "host-global",
				Params:   map[string]any{"key": key, "current": f.Value},
				Evidence: []string{key + " = " + f.Value + " (decided by host)"},
			})
			break
		}
	}

	// ---- 其余内核调优项（只报未达标，避免几十条 ok 噪音）----
	judgeTuning(facts, add)

	// ---- IPv6（仅节点确实有全局 IPv6 时）----
	judgeIPv6(facts, add)

	// ---- 配置文件覆盖关系（sysctl / sshd）----
	judgeConfigFiles(facts, add)

	// ---- SSH ----
	judgeSSH(facts, th, add)

	// ---- 防火墙 ----
	if facts.Firewall == nil {
		add(unknownItem("FW-01", "firewall", "firewall"))
	} else if !facts.Firewall.Enabled {
		add(nodecheckup.Item{
			ID: "FW-01", Category: "firewall", Severity: SeverityHigh, Scope: facts.Env.Kind,
			Params:   map[string]any{"type": facts.Firewall.Type},
			Evidence: []string{"no active local firewall detected (check cloud security group rules separately)"},
			Commands: []nodecheckup.Command{{Shell: "sh", Text: "nft list ruleset 2>/dev/null || iptables -S"}},
		})
	} else {
		add(okItem("FW-01", "firewall", facts.Env.Kind, "firewall", facts.Firewall.Type))
		if facts.Firewall.DefaultInbound == "accept" {
			add(nodecheckup.Item{
				ID: "FW-02", Category: "firewall", Severity: SeverityHigh, Scope: facts.Env.Kind,
				Params:   map[string]any{"current": "accept", "expected": "drop"},
				Evidence: []string{"default inbound policy: accept"},
			})
		}
		// FW-04：SSH 端口放行但没限制来源 IP。很多节点只靠云安全组兜底，
		// 本机防火墙一旦放开就是全网可扫，必须提醒。
		sshPort := 22
		if facts.SSHD != nil && facts.SSHD.Port > 0 {
			sshPort = facts.SSHD.Port
		}
		if exposed, known := sshExposed(facts.Firewall, sshPort); known && exposed {
			add(nodecheckup.Item{
				ID: "FW-04", Category: "firewall", Severity: SeverityWarn, Scope: facts.Env.Kind,
				Params:   map[string]any{"port": sshPort, "type": facts.Firewall.Type},
				Evidence: []string{fmt.Sprintf("SSH port %d is allowed from any source in %s rules", sshPort, facts.Firewall.Type)},
				Commands: sshRestrictCommands(facts.Firewall.Type, sshPort),
			})
		}
	}

	// ---- 资源 ----
	if r := facts.Resources; r != nil {
		if r.DiskTotalKB > 0 {
			pct := int((r.DiskTotalKB - r.DiskFreeKB) * 100 / r.DiskTotalKB)
			sev := SeverityOK
			if pct >= th.DiskHighPct {
				sev = SeverityHigh
			} else if pct >= th.DiskWarnPct {
				sev = SeverityWarn
			}
			add(nodecheckup.Item{
				ID: "RES-01", Category: "system", Severity: sev, Scope: facts.Env.Kind,
				Params:   map[string]any{"pct": pct, "path": r.DiskPath, "warn": th.DiskWarnPct, "high": th.DiskHighPct},
				Evidence: []string{fmt.Sprintf("%s used %d%% (total %d KB, free %d KB)", r.DiskPath, pct, r.DiskTotalKB, r.DiskFreeKB)},
			})
		}
		if r.MemTotalKB > 0 {
			availPct := int(r.MemAvailableKB * 100 / r.MemTotalKB)
			sev := SeverityOK
			if availPct < th.MemAvailableWarnPct {
				sev = SeverityWarn
			}
			add(nodecheckup.Item{
				ID: "RES-02", Category: "system", Severity: sev, Scope: facts.Env.Kind,
				Params:   map[string]any{"pct": availPct, "threshold": th.MemAvailableWarnPct},
				Evidence: []string{fmt.Sprintf("MemAvailable %d%% (%d/%d KB)", availPct, r.MemAvailableKB, r.MemTotalKB)},
			})
		}
		if r.NoFileSoft > 0 && r.NoFileSoft < 65535 {
			add(nodecheckup.Item{
				ID: "RES-03", Category: "system", Severity: SeverityInfo, Scope: facts.Env.Kind,
				Params:   map[string]any{"current": r.NoFileSoft, "expected": 65535},
				Evidence: []string{fmt.Sprintf("RLIMIT_NOFILE soft=%d hard=%d", r.NoFileSoft, r.NoFileHard)},
			})
		}
	}

	// ---- 时间同步 ----
	if facts.TimeSync != nil && !facts.TimeSync.Synchronized {
		add(nodecheckup.Item{
			ID: "TIME-01", Category: "system", Severity: SeverityInfo, Scope: facts.Env.Kind,
			Params:   map[string]any{"service": facts.TimeSync.Service},
			Evidence: []string{"system time not synchronized"},
			Commands: []nodecheckup.Command{{Shell: "sh", Text: "# Persist: enable the time sync service at boot\n" +
				"systemctl enable --now systemd-timesyncd 2>/dev/null || systemctl enable --now chronyd 2>/dev/null || timedatectl set-ntp true\n" +
				"timedatectl status"}},
		})
	}

	return env, summary, items
}

func judgeSSH(facts *nodecheckup.Facts, th Thresholds, add func(nodecheckup.Item)) {
	sshd := facts.SSHD
	if sshd == nil || !sshd.Present {
		scope := "na"
		if facts.Env.Kind == "container" {
			scope = "na"
		}
		add(nodecheckup.Item{
			ID: "SSH-06", Category: "ssh", Severity: SeverityInfo, Scope: scope,
			Params:   map[string]any{"present": false},
			Evidence: []string{"sshd not found in this environment"},
		})
		return
	}
	if sshd.Error != "" {
		add(nodecheckup.Item{
			ID: "SSH-01", Category: "ssh", Severity: SeverityUnknown, Scope: facts.Env.Kind,
			Params:   map[string]any{"error": sshd.Error},
			Evidence: []string{"sshd -T failed: " + sshd.Error},
		})
		return
	}

	passwordUsers := usersWith(sshd, "passwordauthentication", "yes")
	if len(passwordUsers) > 0 {
		add(nodecheckup.Item{
			ID: "SSH-01", Category: "ssh", Severity: SeverityHigh, Scope: facts.Env.Kind,
			Params:   map[string]any{"users": passwordUsers, "current": "yes", "expected": "no"},
			Evidence: []string{"sshd -T: PasswordAuthentication yes"},
			Commands: []nodecheckup.Command{{Shell: "sh", Text: sshPersistScript(
				"ssh-password",
				"PasswordAuthentication no",
				"KbdInteractiveAuthentication no")}},
		})
	} else {
		add(okItem("SSH-01", "ssh", facts.Env.Kind, "PasswordAuthentication", "no"))
	}

	kbdUsers := usersWith(sshd, "kbdinteractiveauthentication", "yes")
	if len(kbdUsers) > 0 {
		add(nodecheckup.Item{
			ID: "SSH-02", Category: "ssh", Severity: SeverityHigh, Scope: facts.Env.Kind,
			Params:   map[string]any{"users": kbdUsers, "current": "yes", "expected": "no"},
			Evidence: []string{"sshd -T: KbdInteractiveAuthentication yes"},
			Commands: []nodecheckup.Command{{Shell: "sh", Text: sshPersistScript(
				"ssh-password",
				"PasswordAuthentication no",
				"KbdInteractiveAuthentication no")}},
		})
	} else {
		add(okItem("SSH-02", "ssh", facts.Env.Kind, "KbdInteractiveAuthentication", "no"))
	}

	// root 用私钥登录是允许的（很多运维直接用 root），不需要为了"安全"关闭。
	// 真正的门槛是密码入口：PasswordAuthentication / KbdInteractive 都为 no 时，
	// PermitRootLogin yes 也只接受密钥；prohibit-password 本身就是只允许密钥。
	rootLogin := sshd.Effective["permitrootlogin"]
	passwordAuth := strings.EqualFold(strings.TrimSpace(sshd.Effective["passwordauthentication"]), "yes")
	kbdAuth := strings.EqualFold(strings.TrimSpace(sshd.Effective["kbdinteractiveauthentication"]), "yes")
	passwordPossible := passwordAuth || kbdAuth
	switch {
	case rootLogin == "":
		add(unknownItem("SSH-03", "ssh", "PermitRootLogin"))
	case rootLogin == "no":
		add(okItem("SSH-03", "ssh", facts.Env.Kind, "PermitRootLogin", rootLogin))
	case !passwordPossible:
		add(okItem("SSH-03", "ssh", facts.Env.Kind, "PermitRootLogin", rootLogin+" (key-only)"))
	case rootLogin == "yes":
		add(nodecheckup.Item{
			ID: "SSH-03", Category: "ssh", Severity: SeverityInfo, Scope: facts.Env.Kind,
			Params:   map[string]any{"current": rootLogin, "password_auth": "enabled"},
			Evidence: []string{"PermitRootLogin yes with password authentication enabled; disable password auth (keep root key login)"},
		})
	default:
		// prohibit-password / forced-commands-only：只允许密钥。
		add(okItem("SSH-03", "ssh", facts.Env.Kind, "PermitRootLogin", rootLogin))
	}

	if key, ok := sshd.Keys["root"]; ok && key.HasKey {
		if key.SSHDirMode != "0700" || key.AuthorizedKeysMode != "0600" || !key.OwnerOK {
			evidence := []string{fmt.Sprintf("~/.ssh mode=%s authorized_keys mode=%s owner_ok=%v", key.SSHDirMode, key.AuthorizedKeysMode, key.OwnerOK)}
			if key.Immutable {
				evidence = append(evidence, "authorized_keys is immutable (chattr +i); chmod will fail until it is cleared")
			}
			add(nodecheckup.Item{
				ID: "SSH-04", Category: "ssh", Severity: SeverityWarn, Scope: facts.Env.Kind,
				Params:   map[string]any{"ssh_dir_mode": key.SSHDirMode, "authorized_keys_mode": key.AuthorizedKeysMode, "owner_ok": key.OwnerOK, "immutable": key.Immutable},
				Evidence: evidence,
				Commands: []nodecheckup.Command{{Shell: "sh", Text: "# File permissions persist; if chattr +i is set, clear it first (otherwise chmod fails with EPERM)\n" +
					"chattr -i ~/.ssh/authorized_keys 2>/dev/null || true\n" +
					"chmod 700 ~/.ssh && chmod 600 ~/.ssh/authorized_keys && chown -R root:root ~/.ssh"}},
			})
		} else {
			add(okItem("SSH-04", "ssh", facts.Env.Kind, "ssh_key_permissions", "600"))
		}
	}

	if maxTries := atoi(sshd.Effective["maxauthtries"]); maxTries > 0 && maxTries > th.SSHMaxAuthTries {
		add(nodecheckup.Item{
			ID: "SSH-07", Category: "ssh", Severity: SeverityInfo, Scope: facts.Env.Kind,
			Params:   map[string]any{"current": maxTries, "threshold": th.SSHMaxAuthTries},
			Evidence: []string{fmt.Sprintf("MaxAuthTries %d", maxTries)},
			Commands: []nodecheckup.Command{{Shell: "sh", Text: sshPersistScript(
				"ssh-authtries",
				fmt.Sprintf("MaxAuthTries %d", th.SSHMaxAuthTries))}},
		})
	}
}

func usersWith(sshd *nodecheckup.SSHD, key, want string) []string {
	var users []string
	check := func(m map[string]string) bool {
		return strings.EqualFold(strings.TrimSpace(m[key]), want)
	}
	if check(sshd.Effective) {
		return []string{"global"}
	}
	for user, m := range sshd.Users {
		if check(m) {
			users = append(users, user)
		}
	}
	return users
}

func okItem(id, category, scope, key, value string) nodecheckup.Item {
	return nodecheckup.Item{
		ID: id, Category: category, Severity: SeverityOK, Scope: scope,
		Params: map[string]any{"value": value}, Evidence: []string{key + " = " + value},
	}
}

func unknownItem(id, category, key string) nodecheckup.Item {
	return nodecheckup.Item{
		ID: id, Category: category, Severity: SeverityUnknown, Scope: "na",
		Params: map[string]any{"key": key}, Evidence: []string{"unable to read " + key},
	}
}

// sysctlPersistFile 是所有内核参数建议的统一落盘位置。
// 不按 key 拆文件：一个 key 一个文件会把 /etc/sysctl.d 塞满，排查也不方便。
const sysctlPersistFile = "/etc/sysctl.d/99-corefusion.conf"

// sysctlKeyPattern 把 sysctl key 转成 sed 正则（点号是正则元字符，必须转义）。
func sysctlKeyPattern(key string) string {
	return strings.ReplaceAll(key, ".", `\.`)
}

// sysctlPersistScript 生成「写入统一 sysctl 文件 + 立即加载」的脚本。
//
// 只用 `sysctl -w` 是**临时**生效：/proc/sys 不是磁盘文件，重启后会被内核
// 按默认值/发行版配置重置。要告诉用户写文件，否则"优化"在重启后就还原了。
//
// 写入前先删掉同名 key 的旧行再追加，保证：
//   - 重复执行幂等，不会出现重复行；
//   - 只复制某一条建议也不会冲掉文件里其它参数。
func sysctlPersistScript(pairs ...string) string {
	var keys []string
	var lines []string
	for _, p := range pairs {
		k, v, ok := strings.Cut(p, "=")
		if !ok {
			continue
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if k == "" {
			continue
		}
		keys = append(keys, sysctlKeyPattern(k)+`[[:space:]]*=`)
		lines = append(lines, "'"+k+" = "+v+"'")
	}
	if len(lines) == 0 {
		return ""
	}
	del := make([]string, 0, len(keys))
	for _, k := range keys {
		del = append(del, "/^"+k+"/d")
	}
	return fmt.Sprintf(
		"# Persist to %s (one shared file; survives reboot)\n"+
			"f=%s\n"+
			"mkdir -p /etc/sysctl.d && touch \"$f\"\n"+
			"sed -i '%s' \"$f\"\n"+
			"printf '%%s\\n' %s >> \"$f\"\n"+
			"sysctl --system",
		sysctlPersistFile, sysctlPersistFile, strings.Join(del, ";"), strings.Join(lines, " "))
}

func sysctlCommand(key, value, scope string) []nodecheckup.Command {
	script := sysctlPersistScript(key + "=" + value)
	switch scope {
	case "host", "container-writable":
		return []nodecheckup.Command{{Shell: "sh", Text: script}}
	case "host-global":
		return []nodecheckup.Command{{Shell: "sh", Text: "# Run on the host (read-only inside the container)\n" + script}}
	default:
		return nil
	}
}

// sshPersistScript 生成「写入 sshd drop-in 文件 + 校验 + reload」的脚本。
//
// sshd_config 是磁盘文件，改完 reload 后重启不会还原；这里统一写
// /etc/ssh/sshd_config.d/99-corefusion-*.conf（不同检查项用不同文件名，
// 避免互相覆盖），并在最后回读实际生效值。
func sshPersistScript(suffix string, lines ...string) string {
	// sshd 采用「先匹配生效」，drop-in 目录按文件名字典序处理，所以用 00- 前缀
	// 才能优先于 50-/99- 的既有文件；同时要求主配置的 Include 在指令之前。
	file := sshManagedPrefix + suffix + ".conf"
	return fmt.Sprintf(
		"# Persist to %s (survives reboot)\n"+
			"mkdir -p /etc/ssh/sshd_config.d\n"+
			"cat > %s <<'EOF'\n%s\nEOF\n"+
			"sshd -t && systemctl reload sshd\n"+
			"# If sshd_config does not Include sshd_config.d/*.conf, edit /etc/ssh/sshd_config instead\n"+
			"sshd -T | grep -Ei '^(passwordauthentication|kbdinteractiveauthentication|maxauthtries) '",
		file, file, strings.Join(lines, "\n"))
}

// bbrCommand 启用 BBR 需要同时设置拥塞控制算法和 qdisc：
// 只改 congestion_control、不设 default_qdisc=fq 时，部分内核上 BBR 效果打折。
// 两个参数写进同一个统一文件，不额外新开 bbr 专用文件。
// bbrModuleScript 用于「内核支持 BBR 但模块未加载」：先 modprobe 并持久化，
// 再切拥塞控制算法。内核 < 4.9 不走这里（压根没有 bbr 模块）。
func bbrModuleScript(scope string) []nodecheckup.Command {
	script := "# Load BBR and persist the module across reboots\n" +
		"modprobe tcp_bbr 2>/dev/null || { echo 'tcp_bbr module is not available on this kernel'; exit 1; }\n" +
		"mkdir -p /etc/modules-load.d && echo tcp_bbr > /etc/modules-load.d/99-corefusion-bbr.conf\n" +
		sysctlPersistScript(
			"net.core.default_qdisc=fq",
			"net.ipv4.tcp_congestion_control=bbr",
		)
	switch scope {
	case "host", "container-writable":
		return []nodecheckup.Command{{Shell: "sh", Text: script}}
	case "host-global":
		return []nodecheckup.Command{{Shell: "sh", Text: "# Run on the host (read-only inside the container)\n" + script}}
	}
	return nil
}

func bbrCommand(scope string) []nodecheckup.Command {
	script := sysctlPersistScript(
		"net.core.default_qdisc=fq",
		"net.ipv4.tcp_congestion_control=bbr",
	)
	switch scope {
	case "host", "container-writable":
		return []nodecheckup.Command{{Shell: "sh", Text: script}}
	case "host-global":
		return []nodecheckup.Command{{Shell: "sh", Text: "# Run on the host (read-only inside the container)\n" + script}}
	default:
		return nil
	}
}

func atoi(s string) int {
	v, _ := strconv.Atoi(strings.TrimSpace(s))
	return v
}

func hasWord(list, word string) bool {
	for _, f := range strings.Fields(list) {
		if f == word {
			return true
		}
	}
	return false
}

// judgeTuning 覆盖线上常用内核调优项。只报未达标项，避免几十条 ok 噪音；
// 完整的推荐值见 RecommendedSysctl / 生成的 Markdown 配置。
func judgeTuning(facts *nodecheckup.Facts, add func(nodecheckup.Item)) {
	if facts == nil {
		return
	}
	get := func(key string) (nodecheckup.Sysctl, bool) {
		f, ok := facts.Sysctl[key]
		if !ok || f.Scope == "na" || strings.TrimSpace(f.Value) == "" {
			return f, false
		}
		return f, true
	}
	numCheck := func(id, cat, key string, sev string, ok func(int64) bool, recommended string) {
		f, valid := get(key)
		if !valid {
			return
		}
		v, err := strconv.ParseInt(strings.TrimSpace(f.Value), 10, 64)
		if err != nil {
			return
		}
		if ok(v) {
			return
		}
		add(tuneItem(id, cat, f.Scope, key, f.Value, recommended, sev))
	}

	const mb16 = int64(16 << 20)
	numCheck("SYS-08", "tcp", "net.core.rmem_max", SeverityInfo, func(v int64) bool { return v >= mb16 }, "16777216")
	numCheck("SYS-09", "tcp", "net.core.wmem_max", SeverityInfo, func(v int64) bool { return v >= mb16 }, "16777216")
	// 判定阈值取「不痛不痒的起点」，推荐值更高；避免 8096 vs 8192 这类噪音。
	numCheck("SYS-10", "tcp", "net.core.netdev_max_backlog", SeverityInfo, func(v int64) bool { return v >= 4096 }, "16384")
	numCheck("SYS-11", "tcp", "net.ipv4.tcp_max_syn_backlog", SeverityInfo, func(v int64) bool { return v >= 4096 }, "8192")
	numCheck("SYS-12", "tcp", "net.ipv4.tcp_tw_reuse", SeverityInfo, func(v int64) bool { return v == 1 }, "1")
	numCheck("SYS-13", "tcp", "net.ipv4.tcp_fin_timeout", SeverityInfo, func(v int64) bool { return v > 0 && v <= 15 }, "15")
	numCheck("SYS-15", "tcp", "net.ipv4.tcp_mtu_probing", SeverityInfo, func(v int64) bool { return v == 1 }, "1")
	numCheck("SYS-16", "tcp", "net.ipv4.tcp_fastopen", SeverityInfo, func(v int64) bool { return v == 3 }, "3")
	numCheck("SYS-17", "tcp", "net.ipv4.tcp_keepalive_time", SeverityInfo, func(v int64) bool { return v > 0 && v <= 600 }, "600")
	// 4294967295 是内核默认「不通知」值，等于没开。
	numCheck("SYS-18", "tcp", "net.ipv4.tcp_notsent_lowat", SeverityInfo, func(v int64) bool { return v > 0 && v != 4294967295 }, "16384")
	numCheck("SYS-19", "tcp", "net.ipv4.tcp_slow_start_after_idle", SeverityInfo, func(v int64) bool { return v == 0 }, "0")
	numCheck("SYS-20", "network", "net.netfilter.nf_conntrack_buckets", SeverityInfo, func(v int64) bool { return v >= 16384 }, "16384")
	numCheck("SYS-21", "tcp", "net.core.netdev_budget", SeverityInfo, func(v int64) bool { return v >= 300 }, "600")

	// ip_local_port_range：宽度（high-low）≥ 30000，代理出站端口不够会 TIME_WAIT 耗尽。
	if f, ok := get("net.ipv4.ip_local_port_range"); ok {
		if parts := strings.Fields(f.Value); len(parts) == 2 {
			lo, _ := strconv.ParseInt(parts[0], 10, 64)
			hi, _ := strconv.ParseInt(parts[1], 10, 64)
			if hi-lo < 30000 {
				add(tuneItem("SYS-14", "tcp", f.Scope, "net.ipv4.ip_local_port_range", f.Value, "1024 65535", SeverityInfo))
			}
		}
	}

	// tcp_rmem / tcp_wmem：第三个数是自动调优上限，太小会限制高 BDP 链路吞吐。
	for _, tc := range []struct{ id, key, rec string }{
		{"SYS-22", "net.ipv4.tcp_rmem", "4096 87380 16777216"},
		{"SYS-23", "net.ipv4.tcp_wmem", "4096 65536 16777216"},
	} {
		f, ok := get(tc.key)
		if !ok {
			continue
		}
		parts := strings.Fields(f.Value)
		if len(parts) != 3 {
			continue
		}
		if max, _ := strconv.ParseInt(parts[2], 10, 64); max < mb16 {
			add(tuneItem(tc.id, "tcp", f.Scope, tc.key, f.Value, tc.rec, SeverityInfo))
		}
	}

	// 内存与文件句柄：容器内是宿主全局，只提示（scope=host-global）。
	if f, ok := get("vm.swappiness"); ok {
		if v, _ := strconv.ParseInt(strings.TrimSpace(f.Value), 10, 64); v > 10 {
			add(tuneItem("SYS-24", "system", f.Scope, "vm.swappiness", f.Value, "10", SeverityInfo))
		}
	}
	if f, ok := get("fs.file-max"); ok {
		if v, _ := strconv.ParseInt(strings.TrimSpace(f.Value), 10, 64); v > 0 && v < 1000000 {
			add(tuneItem("SYS-25", "system", f.Scope, "fs.file-max", f.Value, "1024000", SeverityInfo))
		}
	}

	// 安全加固：ICMP 重定向 / 源路由。代理节点也应当关闭（防 MITM）。
	numCheck("SEC-01", "system", "net.ipv4.conf.all.accept_redirects", SeverityWarn, func(v int64) bool { return v == 0 }, "0")
	numCheck("SEC-02", "system", "net.ipv4.conf.all.send_redirects", SeverityInfo, func(v int64) bool { return v == 0 }, "0")
	numCheck("SEC-03", "system", "net.ipv4.conf.all.accept_source_route", SeverityInfo, func(v int64) bool { return v == 0 }, "0")
}

// judgeIPv6 只在节点有全局 IPv6 地址时给出 IPv6 建议。
// 没有 IPv6 的节点不产生任何 IPv6 项，避免噪音。
func judgeIPv6(facts *nodecheckup.Facts, add func(nodecheckup.Item)) {
	if facts == nil || len(facts.IPv6) == 0 {
		return
	}
	get := func(key string) (nodecheckup.Sysctl, bool) {
		f, ok := facts.Sysctl[key]
		if !ok || f.Scope == "na" || strings.TrimSpace(f.Value) == "" {
			return f, false
		}
		return f, true
	}
	addr := strings.Join(facts.IPv6, ", ")

	// IPV6-01：有公网 IPv6 但未开启 forwarding。用户态代理不依赖它，
	// 但容器网络 / 透明转发 / 路由场景需要，给 info 提示。
	if f, ok := get("net.ipv6.conf.all.forwarding"); ok {
		if v, err := strconv.Atoi(strings.TrimSpace(f.Value)); err == nil && v == 0 {
			add(nodecheckup.Item{
				ID: "IPV6-01", Category: "network", Severity: SeverityInfo, Scope: f.Scope,
				Params: map[string]any{"current": f.Value, "expected": "1", "addresses": facts.IPv6},
				Evidence: []string{
					"global IPv6: " + addr,
					"net.ipv6.conf.all.forwarding = 0 (needed for IPv6 forwarding / containers)",
				},
				Commands: sysctlCommand("net.ipv6.conf.all.forwarding", "1", f.Scope),
			})
		}
	}

	// IPV6-02：ICMPv6 重定向可被用于 MITM。
	if f, ok := get("net.ipv6.conf.all.accept_redirects"); ok {
		if v, err := strconv.Atoi(strings.TrimSpace(f.Value)); err == nil && v != 0 {
			add(nodecheckup.Item{
				ID: "IPV6-02", Category: "security", Severity: SeverityWarn, Scope: f.Scope,
				Params:   map[string]any{"current": f.Value, "expected": "0"},
				Evidence: []string{"net.ipv6.conf.all.accept_redirects = " + f.Value},
				Commands: sysctlCommand("net.ipv6.conf.all.accept_redirects", "0", f.Scope),
			})
		}
	}

	// IPV6-03：源路由。
	if f, ok := get("net.ipv6.conf.all.accept_source_route"); ok {
		if v, err := strconv.Atoi(strings.TrimSpace(f.Value)); err == nil && v != 0 {
			add(nodecheckup.Item{
				ID: "IPV6-03", Category: "security", Severity: SeverityInfo, Scope: f.Scope,
				Params:   map[string]any{"current": f.Value, "expected": "0"},
				Evidence: []string{"net.ipv6.conf.all.accept_source_route = " + f.Value},
				Commands: sysctlCommand("net.ipv6.conf.all.accept_source_route", "0", f.Scope),
			})
		}
	}

	// IPV6-04：服务器应使用稳定地址，隐私扩展会让源地址漂移。
	if f, ok := get("net.ipv6.conf.all.use_tempaddr"); ok {
		if v, err := strconv.Atoi(strings.TrimSpace(f.Value)); err == nil && v != 0 {
			add(nodecheckup.Item{
				ID: "IPV6-04", Category: "network", Severity: SeverityInfo, Scope: f.Scope,
				Params:   map[string]any{"current": f.Value, "expected": "0"},
				Evidence: []string{"net.ipv6.conf.all.use_tempaddr = " + f.Value + " (server addresses should be stable)"},
				Commands: sysctlCommand("net.ipv6.conf.all.use_tempaddr", "0", f.Scope),
			})
		}
	}

	// IPV6-05：邻居表过小，连接数高时会出现 neighbor table overflow。
	if f, ok := get("net.ipv6.neigh.default.gc_thresh2"); ok {
		if v, err := strconv.Atoi(strings.TrimSpace(f.Value)); err == nil && v < 2048 {
			add(nodecheckup.Item{
				ID: "IPV6-05", Category: "network", Severity: SeverityInfo, Scope: f.Scope,
				Params:   map[string]any{"current": f.Value, "expected": "4096"},
				Evidence: []string{"net.ipv6.neigh.default.gc_thresh2 = " + f.Value + " (recommended: 4096)"},
				Commands: sysctlPersistCommands(f.Scope,
					"net.ipv6.neigh.default.gc_thresh1=1024",
					"net.ipv6.neigh.default.gc_thresh2=4096",
					"net.ipv6.neigh.default.gc_thresh3=8192",
				),
			})
		}
	}

	// IPV6-06：路由缓存上限（4.2+），默认 4096 在高并发下会 No buffer space available。
	if f, ok := get("net.ipv6.route.max_size"); ok && kernelAtLeast(facts.Env.Kernel, 4, 2) {
		if v, err := strconv.Atoi(strings.TrimSpace(f.Value)); err == nil && v > 0 && v < 65536 {
			add(nodecheckup.Item{
				ID: "IPV6-06", Category: "network", Severity: SeverityInfo, Scope: f.Scope,
				Params:   map[string]any{"current": f.Value, "expected": "262144"},
				Evidence: []string{"net.ipv6.route.max_size = " + f.Value + " (recommended: 262144)"},
				Commands: sysctlPersistCommands(f.Scope, "net.ipv6.route.max_size=262144"),
			})
		}
	}
}

// sysctlPersistCommands 把多条 sysctl 写进同一脚本，并按 scope 加执行位置说明。
func sysctlPersistCommands(scope string, pairs ...string) []nodecheckup.Command {
	script := sysctlPersistScript(pairs...)
	if script == "" {
		return nil
	}
	switch scope {
	case "host", "container-writable":
		return []nodecheckup.Command{{Shell: "sh", Text: script}}
	case "host-global":
		return []nodecheckup.Command{{Shell: "sh", Text: "# Run on the host (read-only inside the container)\n" + script}}
	default:
		return nil
	}
}

func tuneItem(id, category, scope, key, current, recommended, severity string) nodecheckup.Item {
	return nodecheckup.Item{
		ID: id, Category: category, Severity: severity, Scope: scope,
		Params:   map[string]any{"current": current, "recommended": recommended},
		Evidence: []string{key + " = " + current + " (recommended: " + recommended + ")"},
	}
}

// sshExposed 判断 SSH 端口是否在本机防火墙里对所有来源放行。
// known=false 表示规则里没找到该端口的放行项（可能由云安全组/上游设备控制），
// 此时不提示，避免误报。
func sshExposed(fw *nodecheckup.Firewall, port int) (exposed bool, known bool) {
	if fw == nil || !fw.Enabled {
		return false, false
	}
	p := strconv.Itoa(port)
	raw := fw.RulesRaw
	lower := strings.ToLower(raw)

	switch fw.Type {
	case "ufw":
		for _, line := range strings.Split(raw, "\n") {
			l := strings.TrimSpace(line)
			if !strings.HasPrefix(l, p+"/tcp") && !strings.HasPrefix(l, p+" ") {
				continue
			}
			if strings.Contains(l, "ALLOW") || strings.Contains(l, "LIMIT") {
				if strings.Contains(l, "Anywhere") {
					return true, true
				}
				return false, true
			}
		}
		return false, false

	case "firewalld":
		allowed := false
		if i := strings.Index(lower, "services:"); i >= 0 {
			line := lower[i:]
			if end := strings.Index(line, "\n"); end >= 0 {
				line = line[:end]
			}
			if hasWord(strings.TrimPrefix(line, "services:"), "ssh") {
				allowed = true
			}
		}
		if i := strings.Index(lower, "ports:"); i >= 0 {
			line := lower[i:]
			if end := strings.Index(line, "\n"); end >= 0 {
				line = line[:end]
			}
			if strings.Contains(line, p+"/tcp") {
				allowed = true
			}
		}
		if !allowed {
			return false, false
		}
		// rich rule 带 source 且端口匹配 → 已限制来源。
		if strings.Contains(lower, "source address") && strings.Contains(lower, "port=\""+p+"\"") {
			return false, true
		}
		return true, true

	case "iptables":
		for _, line := range strings.Split(raw, "\n") {
			l := strings.TrimSpace(line)
			if !strings.Contains(l, "--dport "+p) || !strings.Contains(l, "-j ACCEPT") {
				continue
			}
			if strings.Contains(l, " -s ") || strings.Contains(l, "--source ") {
				return false, true
			}
			return true, true
		}
		return false, false

	case "nftables":
		for _, line := range strings.Split(raw, "\n") {
			l := strings.TrimSpace(line)
			if !strings.Contains(l, "dport "+p) || !strings.Contains(l, "accept") {
				continue
			}
			if strings.Contains(l, "saddr") {
				return false, true
			}
			return true, true
		}
		return false, false
	}
	return false, false
}

// sshRestrictCommands 按节点实际使用的防火墙给出「限制 SSH 来源」示例。
// 只作为提醒，不自动执行；<YOUR_IP> 由用户替换。
func sshRestrictCommands(fwType string, port int) []nodecheckup.Command {
	p := strconv.Itoa(port)
	switch fwType {
	case "ufw":
		return []nodecheckup.Command{{Shell: "sh", Text: "# Restrict SSH to trusted sources (replace <YOUR_IP>)\n" +
			"ufw delete allow " + p + "/tcp\n" +
			"ufw allow from <YOUR_IP> to any port " + p + " proto tcp\n" +
			"ufw reload"}}
	case "firewalld":
		return []nodecheckup.Command{{Shell: "sh", Text: "# Restrict SSH to trusted sources (replace <YOUR_IP>)\n" +
			"firewall-cmd --permanent --remove-service=ssh\n" +
			"firewall-cmd --permanent --add-rich-rule='rule family=\"ipv4\" source address=\"<YOUR_IP>\" port port=\"" + p + "\" protocol=\"tcp\" accept'\n" +
			"firewall-cmd --reload"}}
	case "iptables":
		return []nodecheckup.Command{{Shell: "sh", Text: "# Restrict SSH to trusted sources (replace <YOUR_IP>)\n" +
			"iptables -I INPUT -p tcp --dport " + p + " -s <YOUR_IP> -j ACCEPT\n" +
			"iptables -A INPUT -p tcp --dport " + p + " -j DROP\n" +
			"# Persist: apt install iptables-persistent && netfilter-persistent save"}}
	case "nftables":
		return []nodecheckup.Command{{Shell: "sh", Text: "# Restrict SSH to trusted sources (replace <YOUR_IP> and table/chain names)\n" +
			"nft insert rule inet filter input tcp dport " + p + " ip saddr <YOUR_IP> accept\n" +
			"nft add rule inet filter input tcp dport " + p + " drop\n" +
			"# Persist: nft list ruleset > /etc/nftables.conf && systemctl enable nftables"}}
	}
	return nil
}

// SysctlRecommendation 是一条内核推荐参数（用于生成完整配置文件）。
type SysctlRecommendation struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Group   string `json:"group"`
	Comment string `json:"comment,omitempty"`
}

// RecommendedSysctl 返回完整的内核调优推荐值（参考生产节点实际配置）。
// 只做展示/生成文档，不会自动写入；scope 由当前节点实际环境决定。
func RecommendedSysctl() []SysctlRecommendation {
	return []SysctlRecommendation{
		// core
		{"net.core.somaxconn", "32768", "core", "listen backlog for high connection rate"},
		{"net.core.rmem_max", "16777216", "core", "max socket receive buffer (16 MiB)"},
		{"net.core.wmem_max", "16777216", "core", "max socket send buffer (16 MiB)"},
		{"net.core.rmem_default", "262144", "core", "default socket receive buffer"},
		{"net.core.wmem_default", "262144", "core", "default socket send buffer"},
		{"net.core.netdev_max_backlog", "16384", "core", "packets queued when NIC is faster than the stack"},
		{"net.core.netdev_budget", "600", "core", "NAPI poll budget per softirq round"},
		{"net.core.netdev_budget_usecs", "8000", "core", "max time per softirq round (us)"},
		{"net.core.default_qdisc", "fq", "core", "fair queue, required for BBR"},
		// tcp
		{"net.ipv4.tcp_congestion_control", "bbr", "tcp", "BBR congestion control"},
		{"net.ipv4.tcp_rmem", "4096 87380 16777216", "tcp", "TCP receive buffer auto-tuning"},
		{"net.ipv4.tcp_wmem", "4096 65536 16777216", "tcp", "TCP send buffer auto-tuning"},
		{"net.ipv4.tcp_max_syn_backlog", "8192", "tcp", "SYN queue for burst connections"},
		{"net.ipv4.tcp_max_tw_buckets", "5000", "tcp", "cap TIME_WAIT sockets"},
		{"net.ipv4.tcp_tw_reuse", "1", "tcp", "reuse TIME_WAIT for outbound connections"},
		{"net.ipv4.tcp_fin_timeout", "15", "tcp", "FIN_WAIT timeout (seconds)"},
		{"net.ipv4.ip_local_port_range", "1024 65535", "tcp", "ephemeral port range for outbound"},
		{"net.ipv4.tcp_mtu_probing", "1", "tcp", "enable PLPMTUD when ICMP is blocked"},
		{"net.ipv4.tcp_fastopen", "3", "tcp", "TFO for client and server"},
		{"net.ipv4.tcp_keepalive_time", "600", "tcp", "idle keepalive start (seconds)"},
		{"net.ipv4.tcp_keepalive_intvl", "30", "tcp", "keepalive probe interval"},
		{"net.ipv4.tcp_keepalive_probes", "10", "tcp", "keepalive probe count"},
		{"net.ipv4.tcp_notsent_lowat", "16384", "tcp", "avoid bufferbloat on uploads"},
		{"net.ipv4.tcp_slow_start_after_idle", "0", "tcp", "keep cwnd after idle"},
		{"net.ipv4.tcp_syncookies", "1", "tcp", "SYN flood protection"},
		{"net.ipv4.ip_forward", "1", "tcp", "required for forwarding/proxy nodes"},
		// conntrack
		{"net.netfilter.nf_conntrack_max", "262144", "conntrack", "max tracked connections"},
		{"net.netfilter.nf_conntrack_buckets", "16384", "conntrack", "hash buckets (max/16 is a good start)"},
		{"net.netfilter.nf_conntrack_tcp_timeout_established", "180", "conntrack", "established timeout (seconds)"},
		{"net.netfilter.nf_conntrack_udp_timeout", "60", "conntrack", "UDP timeout (seconds)"},
		// ipv6（只有节点确实有 IPv6 时才需要；没有 IPv6 可整段忽略）
		{"net.ipv6.conf.all.forwarding", "1", "ipv6", "forwarding / container networks (0 if no IPv6 routing)"},
		{"net.ipv6.conf.all.accept_redirects", "0", "ipv6", "ignore ICMPv6 redirects"},
		{"net.ipv6.conf.all.accept_source_route", "0", "ipv6", "drop source-routed packets"},
		{"net.ipv6.conf.all.use_tempaddr", "0", "ipv6", "servers should keep stable addresses"},
		{"net.ipv6.neigh.default.gc_thresh1", "1024", "ipv6", "neighbour table low watermark"},
		{"net.ipv6.neigh.default.gc_thresh2", "4096", "ipv6", "neighbour table soft limit"},
		{"net.ipv6.neigh.default.gc_thresh3", "8192", "ipv6", "neighbour table hard limit"},
		{"net.ipv6.route.max_size", "262144", "ipv6", "route cache size (4.2+)"},
		// security
		{"net.ipv4.conf.all.accept_redirects", "0", "security", "ignore ICMP redirects"},
		{"net.ipv4.conf.all.send_redirects", "0", "security", "do not send ICMP redirects"},
		{"net.ipv4.conf.all.accept_source_route", "0", "security", "drop source-routed packets"},
		{"net.ipv4.conf.all.rp_filter", "0", "security", "0 = loose (proxy/asymmetric), 1 = strict"},
		// system
		{"fs.file-max", "1024000", "system", "host-wide file handle limit"},
		{"fs.nr_open", "1048576", "system", "per-process file handle limit"},
		{"vm.swappiness", "10", "system", "prefer RAM over swap for proxy workloads"},
		{"kernel.pid_max", "4194304", "system", "enough PIDs for many connections/processes"},
	}
}

// sysctlMinKernel 记录部分参数引入的内核版本，低于该版本写进配置文件会让
// `sysctl --system` 报 "unknown key"。
var sysctlMinKernel = map[string][2]int{
	"net.core.netdev_budget_usecs":       {3, 11},
	"net.netfilter.nf_conntrack_buckets": {4, 2},
	"net.ipv4.tcp_migrate_req":           {4, 14},
	"net.ipv6.route.max_size":            {4, 2},
}

// RecommendedSysctlFor 按内核版本过滤：BBR/fq 需要 4.9/4.13+，
// CentOS 7（3.10）上必须省略，否则 sysctl 会报 Invalid argument；
// 新内核才有、旧内核没有的参数也一并剔除。
func RecommendedSysctlFor(kernel string) []SysctlRecommendation {
	all := RecommendedSysctl()
	bbrOK := kernelAtLeast(kernel, 4, 9)
	out := make([]SysctlRecommendation, 0, len(all))
	for _, r := range all {
		if min, ok := sysctlMinKernel[r.Key]; ok && !kernelAtLeast(kernel, min[0], min[1]) {
			continue
		}
		switch r.Key {
		case "net.ipv4.tcp_congestion_control":
			if !bbrOK {
				continue
			}
		case "net.core.default_qdisc":
			if !bbrOK {
				r.Value = "fq_codel"
				r.Comment = "kernel < 4.9: BBR/fq unavailable, fq_codel is the safe choice"
			}
		}
		out = append(out, r)
	}
	return out
}

// BuildGuide 生成一份可直接整段复制的 Markdown 运维文档：
// 内核参数 + SSH 加固 + SSH 来源限制（按实际防火墙类型）+ 回滚。
// BuildGuide 保留旧签名（无冲突信息）。
func BuildGuide(env nodecheckup.Env, sshPort int, fwType string) string {
	return BuildGuideWithConflicts(env, sshPort, fwType, nil)
}

// BuildGuideWithConflicts 生成整段 Markdown。items 用于把 CFG-01/CFG-02
// 检测到的「配置覆盖关系」写进第 0 节，避免用户改了自己文件却不生效。
func BuildGuideWithConflicts(env nodecheckup.Env, sshPort int, fwType string, items []nodecheckup.Item) string {
	if sshPort <= 0 {
		sshPort = 22
	}
	if fwType == "" {
		fwType = "nftables"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# CoreFusion node tuning & hardening guide\n\n")
	fmt.Fprintf(&b, "- Host: %s (%s)\n", env.OS, env.Kind)
	fmt.Fprintf(&b, "- Kernel: %s\n", env.Kernel)
	fmt.Fprintf(&b, "- SSH port: %d\n", sshPort)
	fmt.Fprintf(&b, "- Firewall: %s\n\n", fwType)
	b.WriteString("> Review before applying. Run in a maintenance window; keep an existing SSH session open while testing.\n\n")

	writePrecedenceSection(&b, items)

	if !kernelAtLeast(env.Kernel, 4, 9) {
		fmt.Fprintf(&b, "> Kernel %s does not provide BBR (requires >= 4.9). BBR/fq entries are omitted and fq_codel is used instead.\n\n", env.Kernel)
	}

	b.WriteString("## 1. Kernel parameters (persistent)\n\n")
	b.WriteString("Write the whole block to `/etc/sysctl.d/99-corefusion.conf`:\n\n```ini\n")
	group := ""
	for _, r := range RecommendedSysctlFor(env.Kernel) {
		if r.Group != group {
			if group != "" {
				b.WriteString("\n")
			}
			fmt.Fprintf(&b, "# %s\n", r.Group)
			if r.Group == "ipv6" {
				b.WriteString("# skip this block if the node has no IPv6 (check: ip -6 addr show scope global)\n")
			}
			group = r.Group
		}
		if r.Comment != "" {
			fmt.Fprintf(&b, "# %s\n", r.Comment)
		}
		fmt.Fprintf(&b, "%s = %s\n", r.Key, r.Value)
	}
	b.WriteString("```\n\nApply and verify:\n\n```sh\n")
	if kernelAtLeast(env.Kernel, 4, 9) {
		b.WriteString("# BBR ships as a module on most distros; load and persist it before applying\n")
		b.WriteString("modprobe tcp_bbr 2>/dev/null || true\n")
		b.WriteString("mkdir -p /etc/modules-load.d && echo tcp_bbr > /etc/modules-load.d/99-corefusion-bbr.conf\n")
	}
	b.WriteString("sysctl --system\nsysctl net.core.somaxconn net.ipv4.tcp_congestion_control net.netfilter.nf_conntrack_max\n```\n\n")

	b.WriteString("## 2. SSH hardening\n\n")
	b.WriteString("`sshd` uses the **first** value it sees, and drop-ins are read where `Include` appears. Keep\n")
	b.WriteString("`Include /etc/ssh/sshd_config.d/*.conf` near the top of `/etc/ssh/sshd_config` (before\n")
	b.WriteString("`PasswordAuthentication` and friends), otherwise the main file wins and this drop-in is ignored.\n\n")
	b.WriteString("```sh\nmkdir -p /etc/ssh/sshd_config.d\ncat > /etc/ssh/sshd_config.d/00-corefusion-hardening.conf <<'EOF'\n")
	b.WriteString("PasswordAuthentication no\nKbdInteractiveAuthentication no\nMaxAuthTries 4\n")
	b.WriteString("# PermitRootLogin: leave your current value (yes or prohibit-password).\n")
	b.WriteString("# Root key login is fine; never enable password authentication for root.\n")
	b.WriteString("EOF\nsshd -t && systemctl reload sshd\n```\n\n")
	b.WriteString("Keep at least one working key-based session before closing the current one.\n\n")

	b.WriteString("## 3. Restrict SSH to trusted sources\n\n")
	b.WriteString("Replace `<YOUR_IP>` with your office/VPN address (CIDR allowed).\n\n```sh\n")
	switch fwType {
	case "firewalld":
		fmt.Fprintf(&b, "firewall-cmd --permanent --remove-service=ssh\n")
		fmt.Fprintf(&b, "firewall-cmd --permanent --add-rich-rule='rule family=\"ipv4\" source address=\"<YOUR_IP>\" port port=\"%d\" protocol=\"tcp\" accept'\n", sshPort)
		b.WriteString("firewall-cmd --reload\nfirewall-cmd --list-rich-rules\n")
	case "ufw":
		fmt.Fprintf(&b, "ufw delete allow %d/tcp\n", sshPort)
		fmt.Fprintf(&b, "ufw allow from <YOUR_IP> to any port %d proto tcp\n", sshPort)
		b.WriteString("ufw reload\nufw status numbered\n")
	case "iptables":
		fmt.Fprintf(&b, "iptables -I INPUT -p tcp --dport %d -s <YOUR_IP> -j ACCEPT\n", sshPort)
		fmt.Fprintf(&b, "iptables -A INPUT -p tcp --dport %d -j DROP\n", sshPort)
		b.WriteString("# Persist: apt install iptables-persistent && netfilter-persistent save\n")
	default:
		fmt.Fprintf(&b, "nft insert rule inet filter input tcp dport %d ip saddr <YOUR_IP> accept\n", sshPort)
		fmt.Fprintf(&b, "nft add rule inet filter input tcp dport %d drop\n", sshPort)
		b.WriteString("# Adjust table/chain names to your ruleset (nft list ruleset)\n")
		b.WriteString("# Persist: nft list ruleset > /etc/nftables.conf && systemctl enable nftables\n")
	}
	b.WriteString("```\n\n")

	b.WriteString("## 4. Rollback\n\n```sh\nrm -f /etc/sysctl.d/99-corefusion.conf\nsysctl --system\nrm -f /etc/ssh/sshd_config.d/00-corefusion-hardening.conf\nsshd -t && systemctl reload sshd\n```\n\n")
	b.WriteString("Firewall rules must be removed with the matching `-D` / `--remove-rich-rule` / `ufw delete` command.\n")
	return b.String()
}

// parseKernelVersion 解析 `uname -r` 风格的主/次版本（3.10.0-1160.el7 → 3,10）。
func parseKernelVersion(kernel string) (int, int) {
	parts := strings.SplitN(strings.TrimSpace(kernel), ".", 3)
	if len(parts) < 2 {
		return 0, 0
	}
	maj, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0
	}
	minor := parts[1]
	i := 0
	for i < len(minor) && minor[i] >= '0' && minor[i] <= '9' {
		i++
	}
	if i == 0 {
		return 0, 0
	}
	min, err := strconv.Atoi(minor[:i])
	if err != nil {
		return 0, 0
	}
	return maj, min
}

// kernelAtLeast 判断内核版本是否 >= maj.min。未知版本返回 true（不做过度限制）。
func kernelAtLeast(kernel string, maj, min int) bool {
	km, kn := parseKernelVersion(kernel)
	if km == 0 {
		return true
	}
	return km > maj || (km == maj && kn >= min)
}

const (
	// managedSysctlFile 是体检建议写入的统一文件（允许用户自己改，但默认在这里）。
	managedSysctlFile = "/etc/sysctl.d/99-corefusion.conf"
	sysctlConfPath    = "/etc/sysctl.conf"
	// sshManagedPrefix 是 SSH 建议的 drop-in 前缀。sshd 先匹配生效，
	// 所以必须是 00- 才能优先于其它 50-/99- 文件。
	sshManagedPrefix = "/etc/ssh/sshd_config.d/00-corefusion-"
)

// sysctlLoadsAfterManaged 判断某个定义位置是否会在 99-corefusion.conf 之后加载。
// 顺序：/etc/sysctl.d（字典序）→ /run → /usr/local/lib → /usr/lib → /lib → /etc/sysctl.conf。
func sysctlLoadsAfterManaged(file string) bool {
	if file == managedSysctlFile {
		return false
	}
	if file == sysctlConfPath {
		return true
	}
	if strings.HasPrefix(file, "/etc/sysctl.d/") {
		return filepathBase(file) > filepathBase(managedSysctlFile)
	}
	return true
}

func filepathBase(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

func sortedHitKeys(m map[string][]nodecheckup.ConfigHit) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// judgeConfigFiles 检查内核参数与 SSH 配置是否分散在多个文件里，
// 明确告诉用户「谁覆盖谁」——避免用户在自己的优化文件里改了却不生效。
func judgeConfigFiles(facts *nodecheckup.Facts, add func(nodecheckup.Item)) {
	if facts == nil {
		return
	}

	// ---- sysctl ----
	if len(facts.SysctlFiles) > 0 {
		byKey := map[string][]nodecheckup.ConfigHit{}
		for _, h := range facts.SysctlFiles {
			byKey[h.Key] = append(byKey[h.Key], h)
		}
		var details []string
		overridden := 0
		for _, k := range sortedHitKeys(byKey) {
			hits := byKey[k]
			sort.SliceStable(hits, func(i, j int) bool { return hits[i].Order < hits[j].Order })
			winner := hits[len(hits)-1]
			locs := make([]string, 0, len(hits))
			ours := false
			for _, h := range hits {
				locs = append(locs, fmt.Sprintf("%s:%d=%s", h.File, h.Line, h.Value))
				if h.File == managedSysctlFile {
					ours = true
				}
			}
			mark := ""
			if winner.File != managedSysctlFile && sysctlLoadsAfterManaged(winner.File) {
				mark = fmt.Sprintf(" -> overrides %s", managedSysctlFile)
				// 只有我们自己已经写了这个 key，才说明「写了但被覆盖」；否则只是提醒。
				if ours {
					overridden++
				}
			}
			details = append(details, fmt.Sprintf("%s | last wins: %s:%d = %s%s | all: %s",
				k, winner.File, winner.Line, winner.Value, mark, strings.Join(locs, "; ")))
		}
		details = capEvidence(details, 30)
		if len(details) > 0 {
			sev := SeverityInfo
			if overridden > 0 {
				sev = SeverityWarn
			}
			add(nodecheckup.Item{
				ID: "CFG-01", Category: "config", Severity: sev, Scope: facts.Env.Kind,
				Params: map[string]any{
					"managed_file": managedSysctlFile,
					"overridden":   overridden,
					"conflicts":    len(details),
				},
				Evidence: details,
				Commands: []nodecheckup.Command{{Shell: "sh", Text: "# sysctl.d loads in lexical order and /etc/sysctl.conf is last; the last file wins\n" +
					"grep -rn '^net\\.\\|^fs\\.\\|^vm\\.\\|^kernel\\.' /etc/sysctl.d /run/sysctl.d /usr/lib/sysctl.d /etc/sysctl.conf 2>/dev/null\n" +
					"sysctl net.core.somaxconn net.ipv4.tcp_congestion_control"}},
			})
		}
	}

	// ---- sshd ----
	if facts.SSHD != nil && len(facts.SSHD.ConfigHits) > 0 {
		byKey := map[string][]nodecheckup.ConfigHit{}
		for _, h := range facts.SSHD.ConfigHits {
			byKey[h.Key] = append(byKey[h.Key], h)
		}
		oursPresent := false
		for _, h := range facts.SSHD.ConfigHits {
			if strings.HasPrefix(h.File, sshManagedPrefix) {
				oursPresent = true
				break
			}
		}
		var details []string
		foreignWin := 0
		for _, k := range sortedHitKeys(byKey) {
			hits := byKey[k]
			sort.SliceStable(hits, func(i, j int) bool { return hits[i].Order < hits[j].Order })
			winner := hits[0]
			locs := make([]string, 0, len(hits))
			for _, h := range hits {
				locs = append(locs, fmt.Sprintf("%s:%d=%s", h.File, h.Line, h.Value))
			}
			mark := ""
			if !strings.HasPrefix(winner.File, sshManagedPrefix) {
				mark = " -> takes precedence over " + sshManagedPrefix + "*"
				if oursPresent {
					foreignWin++
				}
			}
			details = append(details, fmt.Sprintf("%s | first match wins: %s:%d = %s%s | all: %s",
				k, winner.File, winner.Line, winner.Value, mark, strings.Join(locs, "; ")))
		}
		details = capEvidence(details, 30)
		sev := SeverityInfo
		if facts.SSHD.IncludeLine == 0 {
			sev = SeverityWarn
			details = append(details, "no `Include /etc/ssh/sshd_config.d/*.conf` found; drop-in files are ignored")
		} else if foreignWin > 0 {
			sev = SeverityWarn
		}
		if len(details) > 0 {
			add(nodecheckup.Item{
				ID: "CFG-02", Category: "config", Severity: sev, Scope: facts.Env.Kind,
				Params: map[string]any{
					"include_line":   facts.SSHD.IncludeLine,
					"managed_prefix": sshManagedPrefix,
					"foreign_winner": foreignWin,
				},
				Evidence: details,
				Commands: []nodecheckup.Command{{Shell: "sh", Text: "# sshd uses the FIRST value it sees; drop-ins are read in lexical order at the Include line\n" +
					"grep -rn -E '^(PasswordAuthentication|KbdInteractiveAuthentication|PermitRootLogin|MaxAuthTries)' /etc/ssh/sshd_config /etc/ssh/sshd_config.d 2>/dev/null\n" +
					"sshd -T | grep -Ei '^(passwordauthentication|kbdinteractiveauthentication|permitrootlogin|maxauthtries) '"}},
			})
		}
	}
}

// writePrecedenceSection 写出「第 0 节：已有配置的覆盖关系」。
// 即使没有检测到冲突，也说明两边加载顺序，避免用户误以为写进去就一定生效。
func writePrecedenceSection(b *strings.Builder, items []nodecheckup.Item) {
	var sysctlDetails, sshDetails []string
	for _, it := range items {
		switch it.ID {
		case "CFG-01":
			sysctlDetails = it.Evidence
		case "CFG-02":
			sshDetails = it.Evidence
		}
	}

	b.WriteString("## 0. Existing configuration & precedence\n\n")
	b.WriteString("If this node already has its own tuning files, reading this section first avoids a silent override.\n\n")
	b.WriteString("- **sysctl**: files in `/etc/sysctl.d` are applied in lexical order, then `/run/sysctl.d`,\n")
	b.WriteString("  `/usr/local/lib/sysctl.d`, `/usr/lib/sysctl.d`, `/lib/sysctl.d`, and `/etc/sysctl.conf` **last**.\n")
	b.WriteString("  The last definition of a key wins, so `/etc/sysctl.conf` always overrides `/etc/sysctl.d`.\n")
	b.WriteString("  This guide writes `/etc/sysctl.d/99-corefusion.conf`.\n")
	b.WriteString("- **sshd**: the **first** value seen wins. Drop-ins are processed at the `Include` line in\n")
	b.WriteString("  `/etc/ssh/sshd_config`, in lexical order. This guide writes `00-corefusion-*.conf` so it comes first;\n")
	b.WriteString("  if the main file defines the same directive above `Include`, the main file wins.\n\n")

	if len(sysctlDetails) > 0 {
		b.WriteString("Detected sysctl definitions (last one wins):\n\n")
		for _, d := range sysctlDetails {
			fmt.Fprintf(b, "- `%s`\n", d)
		}
		b.WriteString("\n")
	}
	if len(sshDetails) > 0 {
		b.WriteString("Detected sshd directives (first one wins):\n\n")
		for _, d := range sshDetails {
			fmt.Fprintf(b, "- `%s`\n", d)
		}
		b.WriteString("\n")
	}
}

// capEvidence 限制证据条数，避免几十个 key 把报告和接口响应撑得太大。
func capEvidence(lines []string, max int) []string {
	if max <= 0 || len(lines) <= max {
		return lines
	}
	out := append([]string{}, lines[:max]...)
	return append(out, fmt.Sprintf("... and %d more keys (see CFG commands)", len(lines)-max))
}
