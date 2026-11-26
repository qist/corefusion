// Package nodecheckup 定义节点安全体检的只读事实与判定结果结构。
// Agent 只采集 Facts；规则判定与文案渲染在面板/前端完成。
package nodecheckup

// ConfigHit 是一条配置文件中的指令，用于检测多个文件之间的覆盖关系。
type ConfigHit struct {
	File  string `json:"file"`
	Line  int    `json:"line"`
	Key   string `json:"key"`
	Value string `json:"value"`
	// Order 是加载顺序：数值越大越晚加载（sysctl 后者覆盖前者；
	// sshd 相反，先匹配生效，Order 仍按处理顺序记录）。
	Order int `json:"order,omitempty"`
}

// Facts 是 Agent 采集到的本机事实（不落库）。
type Facts struct {
	Env Env `json:"env"`
	// IPv6 是节点**自身服务网卡**上的全局 IPv6（不含 link-local / 回环 / tun）。
	// tun/wg 这类隧道地址不会算成节点的公网 IPv6。
	IPv6 []string `json:"ipv6,omitempty"`
	// IPv6Addrs 是所有全局 IPv6 的明细（含接口与类型），用于展示与解释。
	IPv6Addrs []IPv6Addr `json:"ipv6_addrs,omitempty"`
	// SysctlFiles 是 sysctl 配置目录/文件里命中白名单 key 的行，
	// 用来判断"我们的 99-corefusion.conf 会不会被别人覆盖"。
	SysctlFiles []ConfigHit       `json:"sysctl_files,omitempty"`
	Sysctl      map[string]Sysctl `json:"sysctl,omitempty"`
	Modules     map[string]bool   `json:"modules,omitempty"`
	SSHD        *SSHD             `json:"sshd,omitempty"`
	Listeners   []Listener        `json:"listeners,omitempty"`
	Firewall    *Firewall         `json:"firewall,omitempty"`
	Resources   *Resources        `json:"resources,omitempty"`
	TimeSync    *TimeSync         `json:"time_sync,omitempty"`
	Errors      []string          `json:"errors,omitempty"`
}

// IPv6Addr 是一条全局 IPv6 地址及其所属接口。
type IPv6Addr struct {
	Addr  string `json:"addr"`
	Iface string `json:"iface"`
	// Kind: public（物理/服务网卡）/ tun（隧道）/ container（容器网桥）。
	Kind string `json:"kind"`
}

type Env struct {
	Kind        string `json:"kind"` // host / vm / container
	Runtime     string `json:"runtime"`
	HostNetwork bool   `json:"host_network"`
	OS          string `json:"os"`
	Kernel      string `json:"kernel"`
	Systemd     bool   `json:"systemd"`
	Root        bool   `json:"root"`
	CapNetAdmin bool   `json:"cap_net_admin"`
}

type Sysctl struct {
	Value    string `json:"value"`
	Scope    string `json:"scope"` // host / container-writable / container-readonly / host-global / na
	Writable bool   `json:"writable"`
}

type SSHD struct {
	Present   bool                         `json:"present"`
	Port      int                          `json:"port"`
	Effective map[string]string            `json:"effective,omitempty"` // 全局 sshd -T
	Users     map[string]map[string]string `json:"users,omitempty"`     // root/admin → sshd -T -C user=
	Keys      map[string]SSHKey            `json:"keys,omitempty"`
	// IncludeLine 是主配置里 `Include /etc/ssh/sshd_config.d/*.conf` 的行号（0 = 没有）。
	IncludeLine int `json:"include_line,omitempty"`
	// ConfigHits 是主配置与 drop-in 里命中关键指令的行（sshd 先匹配生效）。
	ConfigHits []ConfigHit `json:"config_hits,omitempty"`
	Error      string      `json:"error,omitempty"`
}

type SSHKey struct {
	HasKey             bool   `json:"has_key"`
	SSHDirMode         string `json:"ssh_dir_mode"`
	AuthorizedKeysMode string `json:"authorized_keys_mode"`
	Owner              string `json:"owner"`
	OwnerOK            bool   `json:"owner_ok"`
	// Immutable：authorized_keys 带 chattr +i。此时 root 也无法 chmod/chown，
	// 建议命令必须先 chattr -i，否则会报 Operation not permitted。
	Immutable bool `json:"immutable,omitempty"`
}

type Listener struct {
	Proto   string `json:"proto"`
	Addr    string `json:"addr"`
	Port    int    `json:"port"`
	Process string `json:"process,omitempty"`
}

type Firewall struct {
	Type           string   `json:"type"` // nftables/iptables/ufw/firewalld
	Enabled        bool     `json:"enabled"`
	DefaultInbound string   `json:"default_inbound"` // accept/drop/unknown
	RulesRaw       string   `json:"rules_raw,omitempty"`
	SSHAllow       []string `json:"ssh_allow,omitempty"`
	Error          string   `json:"error,omitempty"`
}

type Resources struct {
	MemTotalKB     int64   `json:"mem_total_kb"`
	MemAvailableKB int64   `json:"mem_available_kb"`
	DiskPath       string  `json:"disk_path"`
	DiskTotalKB    int64   `json:"disk_total_kb"`
	DiskFreeKB     int64   `json:"disk_free_kb"`
	Load1          float64 `json:"load1"`
	NoFileSoft     uint64  `json:"nofile_soft"`
	NoFileHard     uint64  `json:"nofile_hard"`
}

type TimeSync struct {
	Service      string `json:"service"`
	Synchronized bool   `json:"synchronized"`
	OffsetMS     int64  `json:"offset_ms,omitempty"`
}

// ---- 判定结果 ----

type Command struct {
	Shell string `json:"shell"`
	Text  string `json:"text"`
}

type Item struct {
	ID       string         `json:"id"`
	Category string         `json:"category"`
	Severity string         `json:"severity"` // high / warn / info / ok / unknown
	Scope    string         `json:"scope"`
	Params   map[string]any `json:"params,omitempty"`
	Evidence []string       `json:"evidence,omitempty"`
	Commands []Command      `json:"commands,omitempty"`
}

type Summary struct {
	High    int `json:"high"`
	Warn    int `json:"warn"`
	Info    int `json:"info"`
	OK      int `json:"ok"`
	Unknown int `json:"unknown"`
}

type Report struct {
	ServerID    int64   `json:"server_id"`
	GeneratedAt int64   `json:"generated_at"`
	Cached      bool    `json:"cached"`
	Env         Env     `json:"env"`
	Summary     Summary `json:"summary"`
	Items       []Item  `json:"items"`
}
