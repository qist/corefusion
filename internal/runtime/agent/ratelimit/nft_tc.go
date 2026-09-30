package ratelimit

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

// NftTcBackend 使用 nftables + tc 实现限速。
// 这是第一阶段的实现，适用于大多数 Linux 场景。
//
// 限速原理：
//   - 下载限速：tc htb 在 eth0 egress 按 mark 分流到不同速率的 class
//   - 上传限速：IFB 设备将 ingress 流量重定向，再在 IFB 上用 htb 限速
//   - nft 在 prerouting 阶段根据 source IP 打 mark
type NftTcBackend struct {
	mu          sync.Mutex
	iface       string       // 主网卡（如 eth0）
	ifbDevice   string       // IFB 设备名（如 ifb0）
	initialized bool         // 是否已初始化
	available   Availability // 可用性缓存

	// ifbSrcPrefs 记录"客户端 IP → ifb 上按源 IP 分流的 u32 filter pref"。
	// 上传限速不能用 fw mark（见 addIfbSrcFilter 注释），只能按源 IP 分流；删除时要按 pref
	// 精确删掉，所以在这里记账（仅内存；重启后 Init 会清空再由 restoreIPEntries 重建）。
	ifbSrcPrefs map[string]uint32
	nextIfbPref uint32
}

const (
	nftTable         = "corefusion_ratelimit"
	nftChain         = "prerouting"      // 上行 mark 链（priority mangle = -150）
	nftPostChain     = "postrouting"     // 下行 mark 链：按 daddr 给"发给客户端的包"打标
	nftConnAcctChain = "connacct_chain"  // 账号并发计数链（priority -145，在 mark 链之后）
	nftConnAcctSet   = "acct_connacct"   // 账号并发计数集合（按 meta mark 聚合 ct count）
	nftConnChain     = "connlimit_chain" // 旧版按 IP connlimit 链（已废弃，Init 时删除）
	nftBlockChain    = "blocked_chain"   // 设备阻断链（priority -160，在 mark 链之前）
	ifbSrcPrefBase   = 100               // ifb 上 u32 源 IP filter 的 pref 起点（避开 fw filter 的 49152）
	defaultClassID   = 9999              // 默认 class（无限速）
	defaultRateMbps  = 10000             // 默认速率 10Gbps（实际不限）
)

// NewNftTcBackend 创建一个新的 nft+tc 限速后端。
func NewNftTcBackend() *NftTcBackend {
	return &NftTcBackend{
		ifbDevice:   "ifb0",
		ifbSrcPrefs: map[string]uint32{},
		available:   DetectAvailability(),
	}
}

// DetectAvailability 全面检测限速能力（二进制 / 内核 / conntrack / 功能探测）。
// 探测在一次性临时表里真实执行"动态集合 + ct count"规则 —— 这是账号并发计数的
// 最小功能单元，探测通过即代表内核真正支持（比只看命令是否存在可靠得多）。
// 结果缓存（构造时调用一次，Agent 生命周期内有效；装完依赖重启 Agent 即可刷新）。
func DetectAvailability() Availability {
	avail := Availability{
		NftAvailable: isCommandAvailable("nft"),
		TcAvailable:  isCommandAvailable("tc"),
		IfbAvailable: isIFBModuleAvailable(),
	}
	avail.ConnTrackAvailable = isConnTrackAvailable()
	if !avail.ConnTrackAvailable {
		avail.UnsupportedReason = "kernel conntrack (nf_conntrack) not available"
	}
	// 功能探测：临时表里真实执行 ct count 规则（需要 nft ≥0.9 + 内核 ≥4.18）
	if avail.NftAvailable {
		ok, reason := probeNftConnLimitSupport()
		avail.KernelSupported = ok
		if !ok {
			avail.UnsupportedReason = reason
		}
	} else {
		avail.KernelSupported = false
		avail.UnsupportedReason = "nftables binary not found"
	}
	return avail
}

// isConnTrackAvailable 检查内核 conntrack 是否可用（账号并发计数依赖它）。
func isConnTrackAvailable() bool {
	// 已加载（proc 接口存在）或模块在内存里
	if _, err := os.Stat("/proc/net/nf_conntrack"); err == nil {
		return true
	}
	if isCommandAvailable("lsmod") {
		if out, err := exec.Command("lsmod").Output(); err == nil && strings.Contains(string(out), "nf_conntrack") {
			return true
		}
	}
	// 未加载但可加载
	if isCommandAvailable("modprobe") {
		if err := exec.Command("modprobe", "--dry-run", "nf_conntrack").Run(); err == nil {
			return true
		}
	}
	return false
}

// kernelAtLeast 解析 uname -r 并比较 (major, minor) 是否不低于目标版本。
func kernelAtLeast(major, minor int) (bool, string) {
	out, err := exec.Command("uname", "-r").Output()
	if err != nil {
		return false, "cannot read kernel version"
	}
	ver := strings.TrimSpace(string(out))
	// "5.14.0-364.el9.x86_64" → 5.14
	parts := strings.SplitN(ver, ".", 3)
	if len(parts) < 2 {
		return false, "unparseable kernel version: " + ver
	}
	maj, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	min, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil {
		return false, "unparseable kernel version: " + ver
	}
	return maj > major || (maj == major && min >= minor), ver
}

// probeNftConnLimitSupport 在一次性临时表里真实执行"动态集合 + ct count"规则，
// 全部成功才认定内核支持账号并发计数。失败原因原样返回（供 UI 展示）。
func probeNftConnLimitSupport() (bool, string) {
	// 内核版本预检（ct count / nft 动态集合需要 ≥4.18）
	if ok, ver := kernelAtLeast(4, 18); !ok {
		return false, "kernel " + ver + " too old for nft dynamic sets / ct count (need >= 4.18)"
	}
	tbl := "corefusion_ratelimit_probe"
	cleanup := func() { _ = exec.Command("nft", "delete", "table", "inet", tbl).Run() }
	cleanup()
	defer cleanup()

	steps := [][]string{
		{"add", "table", "inet", tbl},
		// ⚠ set 规格必须用花括号包裹（与 initNftTable 同一口径）
		{"add", "set", "inet", tbl, "probe_set", "{", "typeof", "meta", "mark", ";", "flags", "dynamic", ";", "}"},
		{"add", "chain", "inet", tbl, "probe_chain", "{", "type", "filter", "hook", "prerouting", "priority", "-145", ";", "policy", "accept", ";", "}"},
		{"add", "rule", "inet", tbl, "probe_chain", "meta", "mark", "0x00000001", "ct", "state", "new",
			"add", "@probe_set", "{", "meta", "mark", "ct", "count", "over", "1", "}", "counter", "drop"},
	}
	for _, args := range steps {
		if out, err := exec.Command("nft", args...).CombinedOutput(); err != nil {
			return false, fmt.Sprintf("kernel probe failed at `nft %s`: %s",
				strings.Join(args, " "), strings.TrimSpace(string(out)))
		}
	}
	return true, ""
}

// IsAvailable 检查后端依赖是否可用。
func (b *NftTcBackend) IsAvailable() Availability {
	return b.available
}

// Init 初始化限速基础设施。
func (b *NftTcBackend) Init(ctx context.Context, iface string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.available.NftAvailable || !b.available.TcAvailable {
		return fmt.Errorf("rate limit backend not available: nft=%v tc=%v",
			b.available.NftAvailable, b.available.TcAvailable)
	}

	if b.initialized && b.iface == iface {
		return nil // 已初始化
	}

	b.iface = iface

	// 1. 创建 nft 表和链
	if err := b.initNftTable(); err != nil {
		return fmt.Errorf("init nft table: %w", err)
	}

	// 2. 初始化 tc egress (下载限速)
	if err := b.initTcEgress(); err != nil {
		return fmt.Errorf("init tc egress: %w", err)
	}

	// 3. 初始化 IFB + tc ingress (上传限速)
	if b.available.IfbAvailable {
		if err := b.initIFB(); err != nil {
			log.Printf("[NftTcBackend] IFB init failed (upload rate limit unavailable): %v", err)
			// IFB 失败不阻塞下载限速
		}
	}

	// 清掉上一次运行遗留的 tc filter：ifb 源 IP filter 的 pref 只存在内存（b.ifbSrcPrefs），
	// 重启后无从对应，只能整组删除；随后 restoreIPEntries 会按当前 IP 重新 ApplyRule。
	runCmd("tc", "filter", "del", "dev", b.iface, "parent", "1:")
	if b.available.IfbAvailable {
		runCmd("tc", "filter", "del", "dev", b.ifbDevice, "parent", "2:")
	}
	b.ifbSrcPrefs = map[string]uint32{}

	b.initialized = true
	log.Printf("[NftTcBackend] initialized on iface=%s ifb=%s", b.iface, b.ifbDevice)
	return nil
}

// ApplyRule 为一个 IP 应用限速规则。
func (b *NftTcBackend) ApplyRule(ctx context.Context, ip string, mark uint32, rate *RateConfig) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.initialized {
		return fmt.Errorf("backend not initialized, call Init() first")
	}

	// 1. nft: 添加 mark 规则
	if err := b.addNftMarkRule(ip, mark); err != nil {
		return fmt.Errorf("add nft mark rule for %s: %w", ip, err)
	}

	// 并发连接限制是**账号（订阅）维度**的，由 Manager 统一调 ApplyConnLimit(mark)
	// 按 mark 聚合计数 —— 不在这里按 IP 加（多设备会翻倍额度）。

	// 2. tc: 添加 egress class (下载限速)
	if err := b.addTcClass(b.iface, 1, mark, rate.MaxDownMbps); err != nil {
		return fmt.Errorf("add tc egress class for mark 0x%x: %w", mark, err)
	}

	// 3. tc: 添加 egress filter
	if err := b.addTcFilter(b.iface, 1, mark); err != nil {
		return fmt.Errorf("add tc egress filter for mark 0x%x: %w", mark, err)
	}

	// 4. tc/ifb: 添加上传限速 class + filter
	if b.available.IfbAvailable && rate.MaxUpMbps > 0 {
		if err := b.addTcClass(b.ifbDevice, 2, mark, rate.MaxUpMbps); err != nil {
			log.Printf("[NftTcBackend] add upload class failed: %v", err)
		}
		// ⚠ 上传限速**必须按源 IP 分流**，不能只靠 fw mark：mark 是 nft prerouting 设的，
		// 而 ingress 流量在 **tc ingress** 阶段就被 mirred 重定向到 ifb 了（早于 prerouting）→
		// 到 ifb 时 mark 还是 0，fw filter 永远匹配不上（实测：只有 fw filter 时 ifb 限速类
		// Sent 0 bytes、流量全进默认类；换成 u32 源 IP 后上传立刻被限到设定值）。
		if err := b.addIfbSrcFilter(ip, mark); err != nil {
			log.Printf("[NftTcBackend] add upload src filter for %s failed: %v", ip, err)
		}
		// 保留 fw filter 作兜底（某些内核路径下 mark 可能随 skb 带过去），不影响结果。
		if err := b.addTcFilter(b.ifbDevice, 2, mark); err != nil {
			log.Printf("[NftTcBackend] add upload filter failed: %v", err)
		}
	}

	return nil
}

// UpdateRate 更新一个订阅的限速速率。
// 速率为 0（不限速）时把 class 速率重置为默认不限速值 —— 否则旧 class 残留，
// "从 100Mbps 改为不限"不会生效。
func (b *NftTcBackend) UpdateRate(ctx context.Context, mark uint32, rate *RateConfig) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.initialized {
		return fmt.Errorf("backend not initialized")
	}

	// 更新下载限速（0=不限 → 重置为默认不限速 class）
	downRate := rate.MaxDownMbps
	if downRate <= 0 {
		downRate = defaultRateMbps
	}
	if err := b.replaceTcClass(b.iface, 1, mark, downRate); err != nil {
		return fmt.Errorf("update download rate for mark 0x%x: %w", mark, err)
	}

	// 更新上传限速
	if b.available.IfbAvailable {
		upRate := rate.MaxUpMbps
		if upRate <= 0 {
			upRate = defaultRateMbps
		}
		if err := b.replaceTcClass(b.ifbDevice, 2, mark, upRate); err != nil {
			log.Printf("[NftTcBackend] update upload rate failed for mark 0x%x: %v", mark, err)
		}
	}

	return nil
}

// RemoveRule 移除一个 IP 的限速规则。
func (b *NftTcBackend) RemoveRule(ctx context.Context, ip string, mark uint32) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.initialized {
		return nil
	}

	// 1. 删除 nft 规则（mark + block；并发计数规则是账号维度，见 ApplyConnLimit）
	b.deleteNftMarkRule(ip)
	b.deleteNftBlockRule(ip)

	// 2. 删除 tc egress class + filter
	b.deleteTcClass(b.iface, 1, mark)
	b.deleteTcFilter(b.iface, 1, mark)

	// 3. 删除 ifb 上传 class + filter（含按源 IP 的 u32 filter）
	if b.available.IfbAvailable {
		b.deleteIfbSrcFilter(ip)
		b.deleteTcClass(b.ifbDevice, 2, mark)
		b.deleteTcFilter(b.ifbDevice, 2, mark)
	}

	return nil
}

// RemoveSubscription 移除一个订阅的所有限速规则。
// 注意：调用方应先从 MarkRegistry 获取该订阅的所有 IP，逐个调用 RemoveRule。
func (b *NftTcBackend) RemoveSubscription(ctx context.Context, mark uint32) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.initialized {
		return nil
	}

	// 删除 tc egress class + filter
	b.deleteTcClass(b.iface, 1, mark)
	b.deleteTcFilter(b.iface, 1, mark)

	// 删除 ifb 上传 class + filter
	if b.available.IfbAvailable {
		b.deleteTcClass(b.ifbDevice, 2, mark)
		b.deleteTcFilter(b.ifbDevice, 2, mark)
	}

	return nil
}

// FlushAll 清除所有限速规则。
func (b *NftTcBackend) FlushAll(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	// 删除 nft 表（级联删除所有链和规则，包括 connlimit_chain 和 blocked_chain）
	runCmd("nft", "delete", "table", "inet", nftTable)

	// 清除 tc qdisc（会级联删除所有 class 和 filter）
	runCmd("tc", "qdisc", "del", "dev", b.iface, "root")
	runCmd("tc", "qdisc", "del", "dev", b.iface, "ingress")

	if b.available.IfbAvailable {
		runCmd("tc", "qdisc", "del", "dev", b.ifbDevice, "root")
		runCmd("ip", "link", "set", "dev", b.ifbDevice, "down")
	}

	b.initialized = false
	log.Printf("[NftTcBackend] flushed all rules on iface=%s", b.iface)
	return nil
}

// Close 清理资源。
func (b *NftTcBackend) Close() error {
	return nil
}

// ApplyConnLimit 应用**账号（订阅）维度**的并发连接限制。
// 按 meta mark（订阅唯一标记）聚合 conntrack 计数：该订阅所有设备的并发连接
// 合计超过 connLimit 才丢弃新连接。connLimit = 0 表示移除限制（不限并发）。
func (b *NftTcBackend) ApplyConnLimit(ctx context.Context, mark uint32, connLimit int32) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.initialized {
		return fmt.Errorf("backend not initialized")
	}

	if connLimit > 0 {
		if err := b.addNftConnAcctRule(mark, connLimit); err != nil {
			return fmt.Errorf("add connacct rule for mark 0x%x: %w", mark, err)
		}
		log.Printf("[NftTcBackend] applied account connlimit for mark=0x%x limit=%d", mark, connLimit)
	} else {
		// 0 = 不限并发：移除计数规则
		b.deleteNftConnAcctRule(mark)
	}

	return nil
}

// RemoveConnLimit 移除一个订阅的并发连接限制规则。
func (b *NftTcBackend) RemoveConnLimit(ctx context.Context, mark uint32) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.initialized {
		return nil
	}

	b.deleteNftConnAcctRule(mark)
	log.Printf("[NftTcBackend] removed account connlimit for mark=0x%x", mark)
	return nil
}

// BlockIP 在 nftables blocked 链中添加阻断规则（丢弃该 IP 的所有流量）。
func (b *NftTcBackend) BlockIP(ctx context.Context, ip string, mark uint32) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.initialized {
		return fmt.Errorf("backend not initialized")
	}

	if err := b.addNftBlockRule(ip); err != nil {
		return fmt.Errorf("add block rule for %s: %w", ip, err)
	}

	log.Printf("[NftTcBackend] blocked ip=%s (mark=0x%x)", ip, mark)
	return nil
}

// UnblockIP 移除对 IP 的阻断。
func (b *NftTcBackend) UnblockIP(ctx context.Context, ip string, mark uint32) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.initialized {
		return nil
	}

	b.deleteNftBlockRule(ip)
	log.Printf("[NftTcBackend] unblocked ip=%s (mark=0x%x)", ip, mark)
	return nil
}

// ============================================================
// nft 操作
// ============================================================

func (b *NftTcBackend) initNftTable() error {
	// 创建表
	if err := runCmd("nft", "add", "table", "inet", nftTable); err != nil {
		return fmt.Errorf("create nft table: %w", err)
	}
	// 创建 mark 链：prerouting, hook prerouting, priority mangle (-150)
	markChainDef := "{ type filter hook prerouting priority mangle ; policy accept ; }"
	if err := runCmd("nft", "add", "chain", "inet", nftTable, nftChain, markChainDef); err != nil {
		log.Printf("[NftTcBackend] nft mark chain create note: %v (may already exist)", err)
	}
	// 创建账号并发计数链：priority -145（在 mark 链 -150 **之后**执行 ——
	// 并发计数按 meta mark（订阅维度）聚合，必须等 mark 先打好）。
	connAcctChainDef := "{ type filter hook prerouting priority -145 ; policy accept ; }"
	if err := runCmd("nft", "add", "chain", "inet", nftTable, nftConnAcctChain, connAcctChainDef); err != nil {
		log.Printf("[NftTcBackend] nft connacct chain create note: %v (may already exist)", err)
	}
	// 创建账号并发计数集合：按 meta mark 聚合 conntrack 计数（flags dynamic）。
	// ⚠ set 规格必须用花括号包裹 —— 无花括号会被 nft 当语法错误（静默失败 → 集合不存在）
	if err := runCmd("nft", "add", "set", "inet", nftTable, nftConnAcctSet,
		"{", "typeof", "meta", "mark", ";", "flags", "dynamic", ";", "}"); err != nil {
		log.Printf("[NftTcBackend] nft connacct set create note: %v (may already exist)", err)
	}
	// 清理旧版本的按 IP connlimit 链（语法本就无效且已被账号维度计数取代）
	runCmd("nft", "delete", "chain", "inet", nftTable, nftConnChain)
	// 创建 postrouting 链：给**下行**方向打标。
	// 客户端"下载"的报文源地址是**节点自身**（不是客户端），prerouting 里按 saddr 匹配不到 →
	// 必须在 postrouting 按 daddr 再打一次标，eth0 egress 的 htb 才能分流。
	// 实测：不加这条链，下载完全不受限（eth0 限速类 Sent 0 bytes）。
	postChainDef := "{ type filter hook postrouting priority mangle ; policy accept ; }"
	if err := runCmd("nft", "add", "chain", "inet", nftTable, nftPostChain, postChainDef); err != nil {
		log.Printf("[NftTcBackend] nft postrouting chain create note: %v (may already exist)", err)
	}
	// 创建 blocked 链：priority -160（在 mark 链之前执行，阻断的包直接丢弃）
	blockChainDef := "{ type filter hook prerouting priority -160 ; policy accept ; }"
	if err := runCmd("nft", "add", "chain", "inet", nftTable, nftBlockChain, blockChainDef); err != nil {
		log.Printf("[NftTcBackend] nft blocked chain create note: %v (may already exist)", err)
	}
	// 清空 mark / postrouting / 账号计数链：这些链完全由本子系统所有，内核里的
	// 残留规则会跨 Agent 重启存活（registry 已清空、没人再删它们）→ Init 后由
	// restoreIPEntries 按持久化 DB **精确重建**，保证内核状态与 DB 一致。
	// （blocked_chain 不清：设备阻断规则不在恢复流程里重建，清了会放行超限设备。）
	runCmd("nft", "flush", "chain", "inet", nftTable, nftChain)
	runCmd("nft", "flush", "chain", "inet", nftTable, nftPostChain)
	runCmd("nft", "flush", "chain", "inet", nftTable, nftConnAcctChain)
	return nil
}

func (b *NftTcBackend) addNftMarkRule(ip string, mark uint32) error {
	markHex := fmt.Sprintf("0x%x", mark)
	// 幂等：先删掉该 IP 已有的 mark 规则再添加（否则重复 ApplyRule 会累积重复规则，
	// 而 deleteNftRuleByIP 每次只删一条，删不干净）。
	b.deleteNftMarkRule(ip)
	// 上行方向（客户端 → 节点）：prerouting 按源地址打标（connlimit / blocked 链也依赖这个 mark）
	if err := runCmd("nft", "add", "rule", "inet", nftTable, nftChain,
		"ip", "saddr", ip, "counter", "meta", "mark", "set", markHex, "accept"); err != nil {
		return err
	}
	// 下行方向（节点 → 客户端）：客户端下载的报文**源地址是节点自己**，prerouting 的 saddr
	// 规则匹配不到 → 在 postrouting 按目的地址补一条，eth0 egress 的 htb 才限得住下载。
	if err := runCmd("nft", "add", "rule", "inet", nftTable, nftPostChain,
		"ip", "daddr", ip, "counter", "meta", "mark", "set", markHex, "accept"); err != nil {
		log.Printf("[NftTcBackend] add postrouting mark rule for %s failed: %v", ip, err)
	}
	return nil
}

func (b *NftTcBackend) deleteNftMarkRule(ip string) {
	b.deleteNftRuleByIP(nftChain, ip)
	b.deleteNftRuleByIP(nftPostChain, ip)
}

// addNftConnAcctRule 添加**账号（订阅）维度**的并发连接限制规则。
// 用 nft 动态集合按 meta mark（订阅唯一标记）聚合 conntrack 计数：该订阅所有
// 设备的新建连接**合计**超过 connLimit 时丢弃。相比按 IP 计数，多设备不会翻倍额度。
//
//	nft add rule inet corefusion_ratelimit connacct_chain \
//	    meta mark <mark> ct state new add @acct_connacct { meta mark ct count over <N> } counter drop
//
// ⚠ 两个语法要点（实测 Rocky9 nft）：
//   - 计数表达式是 **ct count over <N>**（"connlimit count N over" 不是合法 nft 语法）；
//   - 链优先级必须在 mark 链（-150）**之后**（-145），否则读到 mark=0 聚合错桶。
func (b *NftTcBackend) addNftConnAcctRule(mark uint32, connLimit int32) error {
	// 幂等：先删掉该订阅已有的计数规则再添加
	b.deleteNftConnAcctRule(mark)
	markPadded := fmt.Sprintf("0x%08x", mark)
	return runCmd("nft", "add", "rule", "inet", nftTable, nftConnAcctChain,
		"meta", "mark", markPadded,
		"ct", "state", "new",
		"add", "@"+nftConnAcctSet, "{", "meta", "mark", "ct", "count", "over", fmt.Sprintf("%d", connLimit), "}",
		"counter", "drop")
}

func (b *NftTcBackend) deleteNftConnAcctRule(mark uint32) {
	// 规则行形如 "meta mark 0x00001010 ct state new ..."，用定宽 mark + 后缀匹配，
	// 避免相近 mark（0x1010 与 0x10100）前缀误删。
	b.deleteNftRuleByMatch(nftConnAcctChain, fmt.Sprintf("meta mark 0x%08x ct state", mark))
}

// deleteNftRuleByMatch 在指定链中删除所有包含 match 子串的规则（按 handle 删除）。
func (b *NftTcBackend) deleteNftRuleByMatch(chain, match string) {
	out, err := exec.Command("nft", "-a", "list", "chain", "inet", nftTable, chain).Output()
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, match) {
			continue
		}
		handleIdx := strings.LastIndex(line, "handle ")
		if handleIdx < 0 {
			continue
		}
		handleStr := strings.TrimSpace(line[handleIdx+7:])
		runCmd("nft", "delete", "rule", "inet", nftTable, chain, "handle", handleStr)
	}
}

// addNftBlockRule 在 blocked 链中添加阻断规则（丢弃该 IP 的所有流量）。
//
//	nft add rule inet corefusion_ratelimit blocked_chain \
//	    ip saddr <ip> counter drop
func (b *NftTcBackend) addNftBlockRule(ip string) error {
	b.deleteNftBlockRule(ip) // 幂等：避免重复规则累积
	return runCmd("nft", "add", "rule", "inet", nftTable, nftBlockChain,
		"ip", "saddr", ip, "counter", "drop")
}

func (b *NftTcBackend) deleteNftBlockRule(ip string) {
	b.deleteNftRuleByIP(nftBlockChain, ip)
}

// deleteNftRuleByIP 在指定链中查找并删除包含某 IP 的规则（按 handle 删除）。
func (b *NftTcBackend) deleteNftRuleByIP(chain, ip string) {
	out, err := exec.Command("nft", "-a", "list", "chain", "inet", nftTable, chain).Output()
	if err != nil {
		return
	}
	// 注意：要把**所有**匹配该 IP 的规则都删掉（历史实现只删第一条就 return，
	// 重复规则会残留下来继续生效）。
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, ip) {
			continue
		}
		handleIdx := strings.LastIndex(line, "handle ")
		if handleIdx < 0 {
			continue
		}
		handleStr := strings.TrimSpace(line[handleIdx+7:])
		runCmd("nft", "delete", "rule", "inet", nftTable, chain, "handle", handleStr)
	}
}

// ============================================================
// tc 操作（下载限速 / egress）
// ============================================================

func (b *NftTcBackend) initTcEgress() error {
	// 添加根 qdisc: HTB
	if err := runCmd("tc", "qdisc", "add", "dev", b.iface, "root", "handle", "1:", "htb", "default", strconv.Itoa(defaultClassID)); err != nil {
		// 可能已存在，尝试替换
		log.Printf("[NftTcBackend] root qdisc add note: %v (may already exist, trying replace)", err)
	}

	// 添加默认 class（不限速）
	if err := b.addTcClass(b.iface, 1, defaultClassID, defaultRateMbps); err != nil {
		// 可能已存在
		log.Printf("[NftTcBackend] default class add note: %v", err)
	}

	return nil
}

func (b *NftTcBackend) addTcClass(dev string, parentHandle uint32, classID uint32, rateMbps int32) error {
	if rateMbps <= 0 {
		rateMbps = defaultRateMbps // 0 表示不限速
	}
	parentStr := fmt.Sprintf("%d:", parentHandle)
	classStr := fmt.Sprintf("%d:%d", parentHandle, classID)
	rateStr := fmt.Sprintf("%dmbit", rateMbps)
	ceilStr := rateStr // ceil = rate（不允许突发超过限速值）

	// 用 replace 而不是 add：类已存在时 add 会报 `RTNETLINK answers: File exists` →
	// 整个 ApplyRule 失败（实测：agent 重启后按持久化的 IP 恢复规则时 0/1 恢复成功；
	// 生产上"给已有限速的订阅重新下发"同样会失败）。replace = 不存在则建、存在则改，幂等。
	return runCmd("tc", "class", "replace", "dev", dev, "parent", parentStr,
		"classid", classStr, "htb", "rate", rateStr, "ceil", ceilStr)
}

func (b *NftTcBackend) replaceTcClass(dev string, parentHandle uint32, classID uint32, rateMbps int32) error {
	if rateMbps <= 0 {
		rateMbps = defaultRateMbps
	}
	parentStr := fmt.Sprintf("%d:", parentHandle)
	classStr := fmt.Sprintf("%d:%d", parentHandle, classID)
	rateStr := fmt.Sprintf("%dmbit", rateMbps)
	ceilStr := rateStr

	return runCmd("tc", "class", "replace", "dev", dev, "parent", parentStr,
		"classid", classStr, "htb", "rate", rateStr, "ceil", ceilStr)
}

func (b *NftTcBackend) deleteTcClass(dev string, parentHandle uint32, classID uint32) {
	classStr := fmt.Sprintf("%d:%d", parentHandle, classID)
	// 先删 filter 再删 class
	runCmd("tc", "filter", "del", "dev", dev, "parent", fmt.Sprintf("%d:", parentHandle))
	runCmd("tc", "class", "del", "dev", dev, "classid", classStr)
}

func (b *NftTcBackend) addTcFilter(dev string, parentHandle uint32, mark uint32) error {
	markHex := fmt.Sprintf("0x%x", mark)
	flowID := fmt.Sprintf("%d:%d", parentHandle, mark)
	parentStr := fmt.Sprintf("%d:", parentHandle)

	// 同样用 replace：fw filter 的 handle 就是 mark，天然唯一 → 重复下发不会报 File exists。
	return runCmd("tc", "filter", "replace", "dev", dev, "parent", parentStr,
		"protocol", "ip", "handle", markHex, "fw", "flowid", flowID)
}

// addIfbSrcFilter 在 ifb 上按**源 IP** 把该客户端的上传流量分流到对应 class。
//
// 为什么不用 fw mark：mark 在 nft prerouting 阶段设置，而到达网卡的流量在 **tc ingress**
// 阶段就被 mirred 重定向到 ifb 了（早于 netfilter prerouting）→ ifb 上的 fw filter 看到的
// mark 永远是 0。所以只能 u32 直接匹配源 IP（实测唯一可行的做法）。
func (b *NftTcBackend) addIfbSrcFilter(ip string, mark uint32) error {
	pref, ok := b.ifbSrcPrefs[ip]
	if !ok {
		pref = ifbSrcPrefBase + b.nextIfbPref
		b.nextIfbPref++
		b.ifbSrcPrefs[ip] = pref
	}
	return runCmd("tc", "filter", "replace", "dev", b.ifbDevice, "parent", "2:",
		"protocol", "ip", "pref", strconv.FormatUint(uint64(pref), 10),
		"u32", "match", "ip", "src", ip+"/32", "flowid", fmt.Sprintf("2:%d", mark))
}

// deleteIfbSrcFilter 删除某个 IP 的 ifb 源 IP 分流 filter（按记账的 pref 精确删除）。
func (b *NftTcBackend) deleteIfbSrcFilter(ip string) {
	pref, ok := b.ifbSrcPrefs[ip]
	if !ok {
		return
	}
	runCmd("tc", "filter", "del", "dev", b.ifbDevice, "parent", "2:",
		"protocol", "ip", "pref", strconv.FormatUint(uint64(pref), 10))
	delete(b.ifbSrcPrefs, ip)
}

func (b *NftTcBackend) deleteTcFilter(dev string, parentHandle uint32, mark uint32) {
	// tc filter del 需要 prio 和 handle，简单处理：重新添加整个 filter
	// 更优的做法是 list filter 找到具体的 prio 和 handle
	// 当前实现：忽略错误（class 删了 filter 自然失效）
}

// ============================================================
// IFB 操作（上传限速 / ingress → ifb egress）
// ============================================================

func (b *NftTcBackend) initIFB() error {
	// 1. 加载 IFB 内核模块
	if err := runCmd("modprobe", "ifb", "numifbs=1"); err != nil {
		return fmt.Errorf("load ifb module: %w", err)
	}

	// 2. 启动 ifb0 设备。部分内核（如 WSL2）modprobe ifb 后**不会**自动创建设备，
	// 需要 `ip link add` 手动创建；创建的设备重启后消失，但本函数在每次 Init 时都会
	// 重新执行，所以代码级兜底即是持久化方案。
	if err := runCmd("ip", "link", "set", "dev", b.ifbDevice, "up"); err != nil {
		if cerr := runCmd("ip", "link", "add", b.ifbDevice, "type", "ifb"); cerr != nil {
			return fmt.Errorf("bring up %s: %w (auto-create also failed: %v)", b.ifbDevice, err, cerr)
		}
		if err := runCmd("ip", "link", "set", "dev", b.ifbDevice, "up"); err != nil {
			return fmt.Errorf("bring up %s after create: %w", b.ifbDevice, err)
		}
		log.Printf("[NftTcBackend] auto-created missing device %s", b.ifbDevice)
	}

	// 3. 在主网卡添加 ingress qdisc，将 ingress 流量重定向到 ifb0
	if err := runCmd("tc", "qdisc", "add", "dev", b.iface, "ingress"); err != nil {
		log.Printf("[NftTcBackend] ingress qdisc add note: %v (may already exist)", err)
	}

	// 4. 添加 ingress filter：将所有流量镜像到 ifb0
	if err := runCmd("tc", "filter", "add", "dev", b.iface, "parent", "ffff:",
		"protocol", "ip", "u32", "match", "u32", "0", "0",
		"action", "mirred", "egress", "redirect", "dev", b.ifbDevice); err != nil {
		log.Printf("[NftTcBackend] ingress redirect filter note: %v", err)
	}

	// 5. 在 ifb0 上建 HTB（控制上传）。先清掉遗留 root qdisc（如系统自动挂的
	// fq_codel）——否则 add 失败只打 note，HTB 建不上，后续 class 全部无效。
	runCmd("tc", "qdisc", "del", "dev", b.ifbDevice, "root")
	if err := runCmd("tc", "qdisc", "add", "dev", b.ifbDevice, "root",
		"handle", "2:", "htb", "default", strconv.Itoa(defaultClassID)); err != nil {
		log.Printf("[NftTcBackend] ifb root qdisc add note: %v", err)
	}

	// 6. ifb 默认 class
	if err := b.addTcClass(b.ifbDevice, 2, defaultClassID, defaultRateMbps); err != nil {
		log.Printf("[NftTcBackend] ifb default class note: %v", err)
	}

	return nil
}

// ============================================================
// 工具函数
// ============================================================

func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w (%s)", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func isCommandAvailable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func isIFBModuleAvailable() bool {
	// 检查 modprobe 是否可用，以及 ifb 模块是否可以加载
	if !isCommandAvailable("modprobe") {
		return false
	}
	// 尝试检查模块是否存在
	out, err := exec.Command("modprobe", "--dry-run", "ifb").CombinedOutput()
	if err != nil {
		log.Printf("[NftTcBackend] IFB module check: %v (%s)", err, strings.TrimSpace(string(out)))
		return false
	}
	return true
}
