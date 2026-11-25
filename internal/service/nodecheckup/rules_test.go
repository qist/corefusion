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
		"/etc/ssh/sshd_config.d/99-corefusion-ssh-password.conf",
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
