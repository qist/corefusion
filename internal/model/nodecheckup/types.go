// Package nodecheckup 定义节点安全体检的只读事实与判定结果结构。
// Agent 只采集 Facts；规则判定与文案渲染在面板/前端完成。
package nodecheckup

// Facts 是 Agent 采集到的本机事实（不落库）。
type Facts struct {
	Env       Env               `json:"env"`
	Sysctl    map[string]Sysctl `json:"sysctl,omitempty"`
	Modules   map[string]bool   `json:"modules,omitempty"`
	SSHD      *SSHD             `json:"sshd,omitempty"`
	Listeners []Listener        `json:"listeners,omitempty"`
	Firewall  *Firewall         `json:"firewall,omitempty"`
	Resources *Resources        `json:"resources,omitempty"`
	TimeSync  *TimeSync         `json:"time_sync,omitempty"`
	Errors    []string          `json:"errors,omitempty"`
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
	Error     string                       `json:"error,omitempty"`
}

type SSHKey struct {
	HasKey             bool   `json:"has_key"`
	SSHDirMode         string `json:"ssh_dir_mode"`
	AuthorizedKeysMode string `json:"authorized_keys_mode"`
	Owner              string `json:"owner"`
	OwnerOK            bool   `json:"owner_ok"`
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
