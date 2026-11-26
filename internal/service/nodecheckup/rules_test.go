package nodecheckup

import (
	"strings"
	"testing"

	"corefusion/internal/model/nodecheckup"
)

func itemByID(items []nodecheckup.Item, id string) *nodecheckup.Item {
	for i := range items {
		if items[i].ID == id {
			return &items[i]
		}
	}
	return nil
}

func TestJudgeHighRiskHost(t *testing.T) {
	facts := &nodecheckup.Facts{
		Env: nodecheckup.Env{Kind: "vm", OS: "Debian 12", Kernel: "6.8.0", Systemd: true, Root: true},
		Sysctl: map[string]nodecheckup.Sysctl{
			"net.ipv4.ip_forward":             {Value: "0", Scope: "host", Writable: true},
			"net.ipv4.tcp_syncookies":         {Value: "1", Scope: "host", Writable: true},
			"net.core.somaxconn":              {Value: "128", Scope: "host", Writable: true},
			"net.netfilter.nf_conntrack_max":  {Value: "65536", Scope: "host", Writable: true},
			"net.ipv4.tcp_congestion_control": {Value: "cubic", Scope: "host", Writable: true},
		},
		Modules: map[string]bool{"tcp_bbr": true},
		SSHD: &nodecheckup.SSHD{
			Present: true,
			Effective: map[string]string{
				"passwordauthentication":       "yes",
				"kbdinteractiveauthentication": "yes",
				"permitrootlogin":              "yes",
				"maxauthtries":                 "6",
			},
			Users: map[string]map[string]string{},
			Keys:  map[string]nodecheckup.SSHKey{"root": {HasKey: true, SSHDirMode: "0755", AuthorizedKeysMode: "0644", OwnerOK: true}},
		},
		Firewall: &nodecheckup.Firewall{Type: "nftables", Enabled: false},
		Resources: &nodecheckup.Resources{
			MemTotalKB: 1000000, MemAvailableKB: 50000,
			DiskPath: "/", DiskTotalKB: 100000, DiskFreeKB: 3000,
			NoFileSoft: 1024, NoFileHard: 1048576,
		},
		TimeSync: &nodecheckup.TimeSync{Service: "timedatectl", Synchronized: false},
	}
	_, summary, items := Judge(facts, DefaultThresholds())

	for _, id := range []string{"SSH-01", "SSH-02", "FW-01", "RES-01", "SYS-03", "SYS-04"} {
		it := itemByID(items, id)
		if it == nil {
			t.Fatalf("缺少检查项 %s", id)
		}
		if it.Severity == SeverityOK || it.Severity == SeverityUnknown {
			t.Fatalf("%s 应判为风险，实际 %s", id, it.Severity)
		}
	}
	if summary.High < 4 || summary.Warn < 2 {
		t.Fatalf("汇总不符合预期：%+v", summary)
	}
}

func TestJudgeContainerNoSSHDNoFalsePositive(t *testing.T) {
	facts := &nodecheckup.Facts{
		Env:       nodecheckup.Env{Kind: "container", Runtime: "docker", OS: "Debian 12", Systemd: false},
		Sysctl:    map[string]nodecheckup.Sysctl{"vm.swappiness": {Value: "60", Scope: "host-global"}},
		SSHD:      nil,
		Firewall:  &nodecheckup.Firewall{Type: "nftables", Enabled: true, DefaultInbound: "drop"},
		Resources: &nodecheckup.Resources{MemTotalKB: 1000000, MemAvailableKB: 500000, DiskTotalKB: 100000, DiskFreeKB: 90000, NoFileSoft: 1048576},
	}
	_, summary, items := Judge(facts, DefaultThresholds())
	if summary.High != 0 || summary.Warn != 0 {
		t.Fatalf("容器无 sshd 不应产生 high/warn：%+v %+v", summary, items)
	}
	ssh := itemByID(items, "SSH-06")
	if ssh == nil || ssh.Scope != "na" {
		t.Fatalf("容器无 sshd 应判 SSH-06/na：%+v", ssh)
	}
	if it := itemByID(items, "SYS-07"); it == nil || it.Scope != "host-global" {
		t.Fatalf("宿主全局参数应标 host-global：%+v", it)
	}
}

func TestSaveSettingsValidation(t *testing.T) {
	s := NewService(nil, nil, nil)
	bad := DefaultSettings()
	bad.Thresholds.DiskWarnPct = 96
	bad.Thresholds.DiskHighPct = 95
	if _, err := s.SaveSettings(nil, bad); err == nil {
		t.Fatal("disk_high_pct <= disk_warn_pct 应报错")
	}
	ok := DefaultSettings()
	ok.IntervalHours = 12
	if got, err := s.SaveSettings(nil, ok); err != nil || got.IntervalHours != 12 {
		t.Fatalf("合法设置保存失败：%v %+v", err, got)
	}
}

// 建议命令必须写配置文件再重载：只给 sysctl -w 会在重启后还原，
// 是用户明确指出的问题（内核优化/SSH 静态配置要落盘）。
func TestSysctlCommandPersists(t *testing.T) {
	cmds := sysctlCommand("net.core.somaxconn", "32768", "host")
	if len(cmds) != 1 {
		t.Fatalf("应生成一条建议命令：%+v", cmds)
	}
	text := cmds[0].Text
	// 所有内核参数集中一个文件，不按 key 拆文件。
	if !strings.Contains(text, sysctlPersistFile) {
		t.Fatalf("内核参数应写入 %s：%s", sysctlPersistFile, text)
	}
	if strings.Contains(text, "99-corefusion-net") || strings.Contains(text, "99-corefusion-bbr") {
		t.Fatalf("不应再按 key 拆文件：%s", text)
	}
	if !strings.Contains(text, "sed -i") {
		t.Fatalf("写入前应删除同名 key，保证幂等：%s", text)
	}
	if !strings.Contains(text, "sysctl --system") {
		t.Fatalf("写文件后应 reload：%s", text)
	}
	if strings.Contains(text, "sysctl -w net.core.somaxconn") {
		t.Fatalf("不应只给临时 sysctl -w：%s", text)
	}
}

func TestBBRCommandWritesBothParams(t *testing.T) {
	cmds := bbrCommand("host")
	if len(cmds) != 1 {
		t.Fatalf("应生成一条建议命令：%+v", cmds)
	}
	text := cmds[0].Text
	if !strings.Contains(text, sysctlPersistFile) {
		t.Fatalf("BBR 应写入统一文件 %s：%s", sysctlPersistFile, text)
	}
	if strings.Contains(text, "99-corefusion-bbr.conf") {
		t.Fatalf("BBR 不应单独开文件：%s", text)
	}
	// 两个参数必须在同一条命令里写入同一个文件。
	if !strings.Contains(text, "net.core.default_qdisc = fq") || !strings.Contains(text, "net.ipv4.tcp_congestion_control = bbr") {
		t.Fatalf("BBR 应同时写 qdisc 与算法：%s", text)
	}
	if strings.Count(text, ">> \"$f\"") != 1 {
		t.Fatalf("BBR 两个参数应一次性追加：%s", text)
	}
}

func TestSSHPersistScriptWritesDropIn(t *testing.T) {
	text := sshPersistScript("ssh-password", "PasswordAuthentication no", "KbdInteractiveAuthentication no")
	for _, want := range []string{
		"/etc/ssh/sshd_config.d/00-corefusion-ssh-password.conf",
		"PasswordAuthentication no",
		"sshd -t && systemctl reload sshd",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("SSH 建议缺少 %q：%s", want, text)
		}
	}
}

// authorized_keys 被 chattr +i 时，chmod 会报 Operation not permitted；
// 建议命令必须先 chattr -i，不能只给 chmod。
func TestSSH04ImmutableHintAndCommand(t *testing.T) {
	facts := &nodecheckup.Facts{
		Env: nodecheckup.Env{Kind: "host"},
		SSHD: &nodecheckup.SSHD{
			Present: true,
			Effective: map[string]string{
				"passwordauthentication":       "no",
				"kbdinteractiveauthentication": "no",
				"permitrootlogin":              "prohibit-password",
				"maxauthtries":                 "3",
			},
			Users: map[string]map[string]string{},
			Keys: map[string]nodecheckup.SSHKey{
				"root": {HasKey: true, SSHDirMode: "0700", AuthorizedKeysMode: "0400", OwnerOK: true, Immutable: true},
			},
		},
	}
	_, _, items := Judge(facts, DefaultThresholds())
	it := itemByID(items, "SSH-04")
	if it == nil {
		t.Fatal("缺少 SSH-04")
	}
	if len(it.Commands) == 0 || !strings.Contains(it.Commands[0].Text, "chattr -i ~/.ssh/authorized_keys") {
		t.Fatalf("建议命令应先解除 immutable：%+v", it.Commands)
	}
	found := false
	for _, e := range it.Evidence {
		if strings.Contains(e, "immutable") {
			found = true
		}
	}
	if !found {
		t.Fatalf("证据应提示 immutable：%v", it.Evidence)
	}
}

func TestJudgeTuningReportsMissing(t *testing.T) {
	facts := &nodecheckup.Facts{
		Env: nodecheckup.Env{Kind: "host"},
		Sysctl: map[string]nodecheckup.Sysctl{
			"net.core.rmem_max":                  {Value: "212992", Scope: "host"},
			"net.ipv4.tcp_rmem":                  {Value: "4096 87380 6291456", Scope: "host"},
			"net.ipv4.tcp_notsent_lowat":         {Value: "4294967295", Scope: "host"},
			"net.ipv4.conf.all.accept_redirects": {Value: "1", Scope: "host"},
			"vm.swappiness":                      {Value: "60", Scope: "host"},
		},
	}
	_, _, items := Judge(facts, DefaultThresholds())
	for _, id := range []string{"SYS-08", "SYS-22", "SEC-01", "SYS-24"} {
		if itemByID(items, id) == nil {
			t.Fatalf("缺少调优项 %s", id)
		}
	}
	// notsent_lowat=2^32-1 等价于关闭，应提示。
	if itemByID(items, "SYS-18") == nil {
		t.Fatal("tcp_notsent_lowat 未设置时应提示")
	}
}

func TestSSHExposedByFirewall(t *testing.T) {
	cases := []struct {
		name    string
		fw      *nodecheckup.Firewall
		exposed bool
		known   bool
	}{
		{"ufw-any", &nodecheckup.Firewall{Enabled: true, Type: "ufw", RulesRaw: "22/tcp ALLOW Anywhere\n"}, true, true},
		{"ufw-src", &nodecheckup.Firewall{Enabled: true, Type: "ufw", RulesRaw: "22/tcp ALLOW 203.0.113.0/24\n"}, false, true},
		{"ipt-any", &nodecheckup.Firewall{Enabled: true, Type: "iptables", RulesRaw: "-A INPUT -p tcp --dport 22 -j ACCEPT\n"}, true, true},
		{"ipt-src", &nodecheckup.Firewall{Enabled: true, Type: "iptables", RulesRaw: "-A INPUT -p tcp --dport 22 -s 203.0.113.10 -j ACCEPT\n"}, false, true},
		{"nft-any", &nodecheckup.Firewall{Enabled: true, Type: "nftables", RulesRaw: "tcp dport 22 accept\n"}, true, true},
		{"nft-src", &nodecheckup.Firewall{Enabled: true, Type: "nftables", RulesRaw: "ip saddr 203.0.113.0/24 tcp dport 22 accept\n"}, false, true},
		{"fw-svc", &nodecheckup.Firewall{Enabled: true, Type: "firewalld", RulesRaw: "public (active)\n  services: ssh dhcpv6-client\n  ports: \n"}, true, true},
	}
	for _, c := range cases {
		e, k := sshExposed(c.fw, 22)
		if e != c.exposed || k != c.known {
			t.Fatalf("%s: got exposed=%v known=%v, want %v/%v", c.name, e, k, c.exposed, c.known)
		}
	}
}

func TestBuildGuideContainsFullConfig(t *testing.T) {
	md := BuildGuide(nodecheckup.Env{OS: "Debian 12", Kernel: "6.8", Kind: "host"}, 2222, "firewalld")
	for _, want := range []string{
		"# CoreFusion node tuning & hardening guide",
		"net.core.somaxconn = 32768",
		"net.ipv4.tcp_congestion_control = bbr",
		"sysctl --system",
		"/etc/ssh/sshd_config.d/00-corefusion-hardening.conf",
		"firewall-cmd --permanent --remove-service=ssh",
		"port=\"2222\"",
		"## 4. Rollback",
	} {
		if !strings.Contains(md, want) {
			t.Fatalf("guide 缺少 %q", want)
		}
	}
}

func TestKernelVersionParse(t *testing.T) {
	cases := map[string][2]int{
		"3.10.0-1160.el7.x86_64": {3, 10},
		"4.9.0-1":                {4, 9},
		"6.8.0-45-generic":       {6, 8},
		"":                       {0, 0},
	}
	for in, want := range cases {
		maj, min := parseKernelVersion(in)
		if maj != want[0] || min != want[1] {
			t.Fatalf("parseKernelVersion(%q)=%d.%d, want %d.%d", in, maj, min, want[0], want[1])
		}
	}
	if kernelAtLeast("3.10.0-1160.el7", 4, 9) {
		t.Fatal("3.10 不应判定为 >= 4.9")
	}
	if !kernelAtLeast("4.9.0", 4, 9) {
		t.Fatal("4.9 应判定为 >= 4.9")
	}
}

// CentOS 7 的 3.10 内核没有 BBR：只能提示，不能给 sysctl 命令。
func TestJudgeBBROldKernel(t *testing.T) {
	facts := &nodecheckup.Facts{
		Env: nodecheckup.Env{Kind: "host", Kernel: "3.10.0-1160.el7.x86_64"},
		Sysctl: map[string]nodecheckup.Sysctl{
			"net.ipv4.tcp_congestion_control":           {Value: "cubic", Scope: "host"},
			"net.ipv4.tcp_available_congestion_control": {Value: "reno cubic", Scope: "host"},
		},
		Modules: map[string]bool{"tcp_bbr": false},
	}
	_, _, items := Judge(facts, DefaultThresholds())
	it := itemByID(items, "SYS-05")
	if it == nil {
		t.Fatal("缺少 SYS-05")
	}
	if len(it.Commands) != 0 {
		t.Fatalf("3.10 内核不应给 BBR 命令：%+v", it.Commands)
	}
	found := false
	for _, e := range it.Evidence {
		if strings.Contains(e, "requires >= 4.9") {
			found = true
		}
	}
	if !found {
		t.Fatalf("应说明内核不支持 BBR：%v", it.Evidence)
	}
}

func TestBuildGuideOldKernelOmitsBBR(t *testing.T) {
	md := BuildGuide(nodecheckup.Env{OS: "CentOS 7", Kernel: "3.10.0-1160.el7.x86_64", Kind: "host"}, 22, "firewalld")
	if strings.Contains(md, "net.ipv4.tcp_congestion_control = bbr") {
		t.Fatal("3.10 文档不应包含 BBR")
	}
	if strings.Contains(md, "net.core.default_qdisc = fq\n") {
		t.Fatal("3.10 文档不应包含 fq")
	}
	if !strings.Contains(md, "fq_codel") {
		t.Fatal("3.10 文档应改用 fq_codel")
	}
	if !strings.Contains(md, "does not provide BBR") {
		t.Fatal("3.10 文档应说明内核不支持 BBR")
	}
}

// 旧内核缺的参数不能写进配置文件，否则 sysctl --system 报 unknown key。
func TestRecommendedSysctlForOldKernel(t *testing.T) {
	old := RecommendedSysctlFor("3.10.0-1160.el7.x86_64")
	for _, r := range old {
		switch r.Key {
		case "net.ipv4.tcp_congestion_control", "net.core.netdev_budget_usecs", "net.netfilter.nf_conntrack_buckets":
			t.Fatalf("3.10 不应包含 %s", r.Key)
		}
	}
	qdisc := ""
	for _, r := range old {
		if r.Key == "net.core.default_qdisc" {
			qdisc = r.Value
		}
	}
	if qdisc != "fq_codel" {
		t.Fatalf("3.10 应使用 fq_codel，实际 %q", qdisc)
	}

	modern := RecommendedSysctlFor("6.8.0-45-generic")
	has := map[string]bool{}
	for _, r := range modern {
		has[r.Key] = true
	}
	for _, k := range []string{"net.ipv4.tcp_congestion_control", "net.core.netdev_budget_usecs", "net.netfilter.nf_conntrack_buckets"} {
		if !has[k] {
			t.Fatalf("6.8 应包含 %s", k)
		}
	}
}

// 4.9+ 但 bbr 模块没加载：命令必须先 modprobe 并持久化。
func TestBBRModuleScript(t *testing.T) {
	cmds := bbrModuleScript("host")
	if len(cmds) != 1 {
		t.Fatalf("应生成一条命令：%+v", cmds)
	}
	for _, want := range []string{"modprobe tcp_bbr", "/etc/modules-load.d/99-corefusion-bbr.conf", sysctlPersistFile} {
		if !strings.Contains(cmds[0].Text, want) {
			t.Fatalf("缺少 %q：%s", want, cmds[0].Text)
		}
	}
}

func TestBuildGuideModernKernelLoadsModule(t *testing.T) {
	md := BuildGuide(nodecheckup.Env{OS: "Rocky 9", Kernel: "5.14.0-1.el9", Kind: "host"}, 22, "firewalld")
	if !strings.Contains(md, "modprobe tcp_bbr") {
		t.Fatal("4.9+ 文档应包含 modprobe tcp_bbr")
	}
	if !strings.Contains(md, "net.ipv4.tcp_congestion_control = bbr") {
		t.Fatal("4.9+ 文档应包含 BBR")
	}
}

func TestJudgeIPv6(t *testing.T) {
	facts := &nodecheckup.Facts{
		Env:  nodecheckup.Env{Kind: "host", Kernel: "5.14.0-1.el9"},
		IPv6: []string{"2001:db8::10"},
		Sysctl: map[string]nodecheckup.Sysctl{
			"net.ipv6.conf.all.forwarding":          {Value: "0", Scope: "host"},
			"net.ipv6.conf.all.accept_redirects":    {Value: "1", Scope: "host"},
			"net.ipv6.conf.all.accept_source_route": {Value: "0", Scope: "host"},
			"net.ipv6.conf.all.use_tempaddr":        {Value: "2", Scope: "host"},
			"net.ipv6.neigh.default.gc_thresh2":     {Value: "512", Scope: "host"},
			"net.ipv6.route.max_size":               {Value: "4096", Scope: "host"},
		},
	}
	_, summary, items := Judge(facts, DefaultThresholds())
	for _, id := range []string{"IPV6-01", "IPV6-02", "IPV6-04", "IPV6-05", "IPV6-06"} {
		it := itemByID(items, id)
		if it == nil {
			t.Fatalf("缺少 %s", id)
		}
		if len(it.Commands) == 0 {
			t.Fatalf("%s 应给出命令", id)
		}
		if !strings.Contains(it.Commands[0].Text, sysctlPersistFile) {
			t.Fatalf("%s 命令应写统一文件：%s", id, it.Commands[0].Text)
		}
	}
	if itemByID(items, "IPV6-03") != nil {
		t.Fatal("accept_source_route=0 不应报")
	}
	if summary.High != 0 || summary.Warn < 1 {
		t.Fatalf("accept_redirects 应至少 warn：%+v", summary)
	}
}

// 没有全局 IPv6 的节点不应出现任何 IPV6 项。
func TestJudgeIPv6SkippedWithoutAddress(t *testing.T) {
	facts := &nodecheckup.Facts{
		Env: nodecheckup.Env{Kind: "host", Kernel: "5.14.0-1.el9"},
		Sysctl: map[string]nodecheckup.Sysctl{
			"net.ipv6.conf.all.accept_redirects": {Value: "1", Scope: "host"},
		},
	}
	_, _, items := Judge(facts, DefaultThresholds())
	for _, it := range items {
		if strings.HasPrefix(it.ID, "IPV6-") {
			t.Fatalf("无 IPv6 地址不应产生 %s", it.ID)
		}
	}
}

func TestBuildGuideIPv6AndOldKernel(t *testing.T) {
	modern := BuildGuide(nodecheckup.Env{OS: "Rocky 9", Kernel: "5.14.0-1.el9", Kind: "host"}, 22, "firewalld")
	for _, want := range []string{"# ipv6", "net.ipv6.conf.all.forwarding = 1", "net.ipv6.route.max_size = 262144", "skip this block if the node has no IPv6"} {
		if !strings.Contains(modern, want) {
			t.Fatalf("guide 缺少 %q", want)
		}
	}
	old := BuildGuide(nodecheckup.Env{OS: "CentOS 7", Kernel: "3.10.0-1160.el7.x86_64", Kind: "host"}, 22, "firewalld")
	if strings.Contains(old, "net.ipv6.route.max_size") {
		t.Fatal("3.10 不应包含 route.max_size（4.2+）")
	}
	if !strings.Contains(old, "net.ipv6.neigh.default.gc_thresh2 = 4096") {
		t.Fatal("3.10 仍应包含邻居表参数")
	}
}

// sysctl 后加载覆盖先加载：/etc/sysctl.conf 会让 99-corefusion.conf 失效，
// 必须明确告警，否则用户改了自己的文件却不生效。
func TestJudgeSysctlConfigPrecedence(t *testing.T) {
	facts := &nodecheckup.Facts{
		Env: nodecheckup.Env{Kind: "host", Kernel: "5.14.0-1.el9"},
		SysctlFiles: []nodecheckup.ConfigHit{
			{File: "/etc/sysctl.d/99-corefusion.conf", Line: 3, Key: "net.core.somaxconn", Value: "32768", Order: 5},
			{File: "/etc/sysctl.conf", Line: 85, Key: "net.core.somaxconn", Value: "1024", Order: 99},
		},
	}
	_, summary, items := Judge(facts, DefaultThresholds())
	it := itemByID(items, "CFG-01")
	if it == nil {
		t.Fatal("缺少 CFG-01")
	}
	if it.Severity != SeverityWarn {
		t.Fatalf("被 /etc/sysctl.conf 覆盖应判 warn，实际 %s", it.Severity)
	}
	joined := strings.Join(it.Evidence, "\n")
	if !strings.Contains(joined, "overrides /etc/sysctl.d/99-corefusion.conf") {
		t.Fatalf("证据应说明谁覆盖谁：%s", joined)
	}
	if summary.Warn < 1 {
		t.Fatalf("summary 应包含 warn：%+v", summary)
	}
}

// 只有我们自己的文件时，不应误报 warn。
func TestJudgeSysctlConfigOnlyManaged(t *testing.T) {
	facts := &nodecheckup.Facts{
		Env: nodecheckup.Env{Kind: "host", Kernel: "5.14.0-1.el9"},
		SysctlFiles: []nodecheckup.ConfigHit{
			{File: "/etc/sysctl.d/99-corefusion.conf", Line: 3, Key: "net.core.somaxconn", Value: "32768", Order: 5},
		},
	}
	_, _, items := Judge(facts, DefaultThresholds())
	it := itemByID(items, "CFG-01")
	if it == nil {
		t.Fatal("缺少 CFG-01")
	}
	if it.Severity != SeverityInfo {
		t.Fatalf("只有自己的文件应为 info，实际 %s", it.Severity)
	}
}

// sshd 先匹配生效：主配置在 Include 之前的行会压过 00-corefusion-*。
func TestJudgeSSHDConfigPrecedence(t *testing.T) {
	facts := &nodecheckup.Facts{
		Env: nodecheckup.Env{Kind: "host", Kernel: "5.14.0-1.el9"},
		SSHD: &nodecheckup.SSHD{
			Present:     true,
			IncludeLine: 12,
			ConfigHits: []nodecheckup.ConfigHit{
				{File: "/etc/ssh/sshd_config", Line: 5, Key: "passwordauthentication", Value: "yes", Order: 5},
				{File: "/etc/ssh/sshd_config.d/00-corefusion-ssh-password.conf", Line: 1, Key: "passwordauthentication", Value: "no", Order: 10000},
			},
		},
	}
	_, _, items := Judge(facts, DefaultThresholds())
	it := itemByID(items, "CFG-02")
	if it == nil {
		t.Fatal("缺少 CFG-02")
	}
	if it.Severity != SeverityWarn {
		t.Fatalf("主配置先命中应判 warn，实际 %s", it.Severity)
	}
	joined := strings.Join(it.Evidence, "\n")
	if !strings.Contains(joined, "first match wins: /etc/ssh/sshd_config:5") {
		t.Fatalf("证据应指出先命中的文件：%s", joined)
	}
}

// 没有 Include 时 drop-in 完全不生效。
func TestJudgeSSHDConfigNoInclude(t *testing.T) {
	facts := &nodecheckup.Facts{
		Env: nodecheckup.Env{Kind: "host", Kernel: "5.14.0-1.el9"},
		SSHD: &nodecheckup.SSHD{
			Present:     true,
			IncludeLine: 0,
			ConfigHits: []nodecheckup.ConfigHit{
				{File: "/etc/ssh/sshd_config.d/00-corefusion-ssh-password.conf", Line: 1, Key: "passwordauthentication", Value: "no", Order: 200000},
			},
		},
	}
	_, _, items := Judge(facts, DefaultThresholds())
	it := itemByID(items, "CFG-02")
	if it == nil || it.Severity != SeverityWarn {
		t.Fatalf("缺 Include 应 warn：%+v", it)
	}
	if !strings.Contains(strings.Join(it.Evidence, "\n"), "no `Include") {
		t.Fatalf("证据应说明 Include 缺失：%v", it.Evidence)
	}
}

func TestBuildGuidePrecedenceSection(t *testing.T) {
	items := []nodecheckup.Item{
		{ID: "CFG-01", Severity: SeverityWarn, Evidence: []string{"net.core.somaxconn | last wins: /etc/sysctl.conf:85 = 1024"}},
		{ID: "CFG-02", Severity: SeverityWarn, Evidence: []string{"passwordauthentication | first match wins: /etc/ssh/sshd_config:5 = yes"}},
	}
	md := BuildGuideWithConflicts(nodecheckup.Env{OS: "Rocky 9", Kernel: "5.14.0-1.el9", Kind: "host"}, 22, "firewalld", items)
	for _, want := range []string{
		"## 0. Existing configuration & precedence",
		"The last definition of a key wins",
		"the **first** value seen wins",
		"/etc/sysctl.conf:85 = 1024",
		"/etc/ssh/sshd_config:5 = yes",
		"00-corefusion-hardening.conf",
	} {
		if !strings.Contains(md, want) {
			t.Fatalf("guide 缺少 %q", want)
		}
	}
}

func TestSysctlLoadsAfterManaged(t *testing.T) {
	cases := map[string]bool{
		managedSysctlFile:                   false,
		"/etc/sysctl.conf":                  true,
		"/etc/sysctl.d/99-sysctl.conf":      true,
		"/etc/sysctl.d/99-zzz.conf":         true,
		"/etc/sysctl.d/10-network.conf":     false,
		"/etc/sysctl.d/50-default.conf":     false,
		"/run/sysctl.d/50-user.conf":        true,
		"/usr/lib/sysctl.d/50-default.conf": true,
	}
	for file, want := range cases {
		if got := sysctlLoadsAfterManaged(file); got != want {
			t.Fatalf("sysctlLoadsAfterManaged(%q)=%v, want %v", file, got, want)
		}
	}
}

func sshFacts(effective map[string]string) *nodecheckup.Facts {
	return &nodecheckup.Facts{
		Env: nodecheckup.Env{Kind: "host", Kernel: "5.14.0-1.el9"},
		SSHD: &nodecheckup.SSHD{
			Present:   true,
			Effective: effective,
			Users:     map[string]map[string]string{},
			Keys:      map[string]nodecheckup.SSHKey{},
		},
	}
}

// 用户明确要用 root 登录：密码关闭时 PermitRootLogin yes 应判 ok，不该要求关闭 root。
func TestSSH03RootYesWithPasswordDisabled(t *testing.T) {
	facts := sshFacts(map[string]string{
		"passwordauthentication":       "no",
		"kbdinteractiveauthentication": "no",
		"permitrootlogin":              "yes",
		"maxauthtries":                 "4",
	})
	_, _, items := Judge(facts, DefaultThresholds())
	it := itemByID(items, "SSH-03")
	if it == nil {
		t.Fatal("缺少 SSH-03")
	}
	if it.Severity != SeverityOK {
		t.Fatalf("密码关闭时 root 密钥登录应判 ok，实际 %s：%v", it.Severity, it.Evidence)
	}
}

// 密码入口还开着时，root + 密码才是风险点。
func TestSSH03RootYesWithPasswordEnabled(t *testing.T) {
	facts := sshFacts(map[string]string{
		"passwordauthentication":       "yes",
		"kbdinteractiveauthentication": "no",
		"permitrootlogin":              "yes",
		"maxauthtries":                 "6",
	})
	_, _, items := Judge(facts, DefaultThresholds())
	it := itemByID(items, "SSH-03")
	if it == nil || it.Severity != SeverityInfo {
		t.Fatalf("密码开启时 SSH-03 应为 info：%+v", it)
	}
	if !strings.Contains(strings.Join(it.Evidence, " "), "disable password auth") {
		t.Fatalf("证据应提示先关密码：%v", it.Evidence)
	}
}

func TestSSH03ProhibitPasswordIsOK(t *testing.T) {
	facts := sshFacts(map[string]string{
		"passwordauthentication":       "yes",
		"kbdinteractiveauthentication": "no",
		"permitrootlogin":              "prohibit-password",
		"maxauthtries":                 "4",
	})
	_, _, items := Judge(facts, DefaultThresholds())
	it := itemByID(items, "SSH-03")
	if it == nil || it.Severity != SeverityOK {
		t.Fatalf("prohibit-password 应判 ok：%+v", it)
	}
}

// guide 不应把 PermitRootLogin 写死成 prohibit-password（用户可能要用 root 密钥登录）。
func TestBuildGuideKeepsRootLogin(t *testing.T) {
	md := BuildGuide(nodecheckup.Env{OS: "Ubuntu", Kernel: "7.0.0", Kind: "host"}, 22, "nftables")
	if strings.Contains(md, "\nPermitRootLogin prohibit-password\n") {
		t.Fatal("guide 不应把 PermitRootLogin 写死")
	}
	if !strings.Contains(md, "PermitRootLogin: leave your current value") {
		t.Fatal("guide 应提示保留当前 root 登录方式")
	}
}
