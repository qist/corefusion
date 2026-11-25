package nodecheckup

import (
	"fmt"
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

	add(nodecheckup.Item{
		ID: "ENV-01", Category: "system", Severity: SeverityInfo, Scope: env.Kind,
		Params: map[string]any{
			"kind": env.Kind, "runtime": env.Runtime, "systemd": env.Systemd,
			"host_network": env.HostNetwork, "root": env.Root,
		},
		Evidence: []string{"OS: " + env.OS, "Kernel: " + env.Kernel},
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

	if f, ok := facts.Sysctl["net.ipv4.tcp_congestion_control"]; ok && f.Scope != "na" {
		available := facts.Sysctl["net.ipv4.tcp_available_congestion_control"].Value
		if strings.TrimSpace(f.Value) == "bbr" {
			add(okItem("SYS-05", "tcp", f.Scope, "net.ipv4.tcp_congestion_control", f.Value))
		} else if available == "" || hasWord(available, "bbr") {
			add(nodecheckup.Item{
				ID: "SYS-05", Category: "tcp", Severity: SeverityInfo, Scope: f.Scope,
				Params:   map[string]any{"current": f.Value, "expected": "bbr"},
				Evidence: []string{"tcp_congestion_control = " + f.Value},
				Commands: bbrCommand(f.Scope),
			})
		} else {
			add(nodecheckup.Item{
				ID: "SYS-05", Category: "tcp", Severity: SeverityInfo, Scope: f.Scope,
				Params:   map[string]any{"current": f.Value, "available": available},
				Evidence: []string{"tcp_congestion_control = " + f.Value, "available = " + available},
			})
		}
	}
	// BBR 可能编译进内核，不能只看 /proc/modules；优先用 available_congestion_control。
	if f, ok := facts.Sysctl["net.ipv4.tcp_available_congestion_control"]; ok && f.Scope != "na" {
		if hasWord(f.Value, "bbr") {
			add(okItem("SYS-06", "tcp", f.Scope, "tcp_available_congestion_control", f.Value))
		} else {
			add(nodecheckup.Item{
				ID: "SYS-06", Category: "tcp", Severity: SeverityInfo, Scope: f.Scope,
				Params:   map[string]any{"available": f.Value},
				Evidence: []string{"tcp_available_congestion_control = " + f.Value},
			})
		}
	} else if bbr, ok := facts.Modules["tcp_bbr"]; ok && !bbr {
		add(nodecheckup.Item{
			ID: "SYS-06", Category: "tcp", Severity: SeverityInfo, Scope: "na",
			Params:   map[string]any{"module": "tcp_bbr"},
			Evidence: []string{"tcp_bbr module not found"},
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
			Commands: []nodecheckup.Command{{Shell: "sh", Text: "# 持久：启用时间同步服务（enable 写入 systemd 开机自启，重启后仍同步）\n" +
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

	// root 可用私钥登录是允许的；只要密码入口关闭就不算问题。
	rootLogin := sshd.Effective["permitrootlogin"]
	if rootLogin == "" {
		add(unknownItem("SSH-03", "ssh", "PermitRootLogin"))
	} else if rootLogin == "no" {
		add(okItem("SSH-03", "ssh", facts.Env.Kind, "PermitRootLogin", rootLogin))
	} else {
		add(nodecheckup.Item{
			ID: "SSH-03", Category: "ssh", Severity: SeverityInfo, Scope: facts.Env.Kind,
			Params:   map[string]any{"current": rootLogin},
			Evidence: []string{"PermitRootLogin " + rootLogin + " (password login is the gate)"},
		})
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
				Commands: []nodecheckup.Command{{Shell: "sh", Text: "# 文件权限持久生效；若被 chattr +i 锁定，先解锁（否则 chmod 报 Operation not permitted）\n" +
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
		"# 持久化：写入 %s（统一文件，重启后仍生效）\n"+
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
		return []nodecheckup.Command{{Shell: "sh", Text: "# 需在宿主机执行（容器内该参数只读）\n" + script}}
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
	file := "/etc/ssh/sshd_config.d/99-corefusion-" + suffix + ".conf"
	return fmt.Sprintf(
		"# 持久化：写入 %s，重启后仍生效\n"+
			"mkdir -p /etc/ssh/sshd_config.d\n"+
			"cat > %s <<'EOF'\n%s\nEOF\n"+
			"sshd -t && systemctl reload sshd\n"+
			"# 若 sshd_config 未 Include sshd_config.d/*.conf，请改为编辑 /etc/ssh/sshd_config 对应行\n"+
			"sshd -T | grep -Ei '^(passwordauthentication|kbdinteractiveauthentication|maxauthtries) '",
		file, file, strings.Join(lines, "\n"))
}

// bbrCommand 启用 BBR 需要同时设置拥塞控制算法和 qdisc：
// 只改 congestion_control、不设 default_qdisc=fq 时，部分内核上 BBR 效果打折。
// 两个参数写进同一个统一文件，不额外新开 bbr 专用文件。
func bbrCommand(scope string) []nodecheckup.Command {
	script := sysctlPersistScript(
		"net.core.default_qdisc=fq",
		"net.ipv4.tcp_congestion_control=bbr",
	)
	switch scope {
	case "host", "container-writable":
		return []nodecheckup.Command{{Shell: "sh", Text: script}}
	case "host-global":
		return []nodecheckup.Command{{Shell: "sh", Text: "# 需在宿主机执行（容器内该参数只读）\n" + script}}
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
