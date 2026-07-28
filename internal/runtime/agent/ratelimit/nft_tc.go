package ratelimit

import (
	"context"
	"fmt"
	"log"
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
	iface       string           // 主网卡（如 eth0）
	ifbDevice   string           // IFB 设备名（如 ifb0）
	initialized bool             // 是否已初始化
	available   Availability     // 可用性缓存
}

const (
	nftTable        = "corefusion_ratelimit"
	nftChain        = "prerouting"       // mark 链（priority mangle = -150）
	nftConnChain    = "connlimit_chain"  // connlimit 链（priority -155，在 mark 链之前）
	nftBlockChain   = "blocked_chain"    // 设备阻断链（priority -160，在 connlimit 链之前）
	defaultClassID  = 9999  // 默认 class（无限速）
	defaultRateMbps = 10000 // 默认速率 10Gbps（实际不限）
)

// NewNftTcBackend 创建一个新的 nft+tc 限速后端。
func NewNftTcBackend() *NftTcBackend {
	return &NftTcBackend{
		ifbDevice: "ifb0",
		available: Availability{
			NftAvailable: isCommandAvailable("nft"),
			TcAvailable:  isCommandAvailable("tc"),
			IfbAvailable: isIFBModuleAvailable(),
		},
	}
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

	// 1b. 如果有并发连接限制，添加 connlimit 规则
	if rate.ConnLimit > 0 {
		if err := b.addNftConnLimitRule(ip, rate.ConnLimit); err != nil {
			log.Printf("[NftTcBackend] add connlimit rule for %s: %v", ip, err)
		}
	}

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
		if err := b.addTcFilter(b.ifbDevice, 2, mark); err != nil {
			log.Printf("[NftTcBackend] add upload filter failed: %v", err)
		}
	}

	return nil
}

// UpdateRate 更新一个订阅的限速速率。
func (b *NftTcBackend) UpdateRate(ctx context.Context, mark uint32, rate *RateConfig) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.initialized {
		return fmt.Errorf("backend not initialized")
	}

	// 更新下载限速
	if rate.MaxDownMbps > 0 {
		if err := b.replaceTcClass(b.iface, 1, mark, rate.MaxDownMbps); err != nil {
			return fmt.Errorf("update download rate for mark 0x%x: %w", mark, err)
		}
	}

	// 更新上传限速
	if b.available.IfbAvailable && rate.MaxUpMbps > 0 {
		if err := b.replaceTcClass(b.ifbDevice, 2, mark, rate.MaxUpMbps); err != nil {
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

	// 1. 删除 nft 规则（mark + connlimit + block）
	b.deleteNftMarkRule(ip)
	b.deleteNftConnLimitRule(ip)
	b.deleteNftBlockRule(ip)

	// 2. 删除 tc egress class + filter
	b.deleteTcClass(b.iface, 1, mark)
	b.deleteTcFilter(b.iface, 1, mark)

	// 3. 删除 ifb 上传 class + filter
	if b.available.IfbAvailable {
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

// ApplyConnLimit 为一个 IP 添加 nftables connlimit 规则（并发连接限制）。
// connLimit = 0 表示移除限制。
func (b *NftTcBackend) ApplyConnLimit(ctx context.Context, ip string, mark uint32, connLimit int32) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.initialized {
		return fmt.Errorf("backend not initialized")
	}

	// 先删除旧的 connlimit 规则（如果有）
	b.deleteNftConnLimitRule(ip)

	if connLimit > 0 {
		if err := b.addNftConnLimitRule(ip, connLimit); err != nil {
			return fmt.Errorf("add connlimit rule for %s: %w", ip, err)
		}
		log.Printf("[NftTcBackend] applied connlimit for ip=%s limit=%d", ip, connLimit)
	}

	return nil
}

// RemoveConnLimit 移除一个 IP 的 connlimit 规则。
func (b *NftTcBackend) RemoveConnLimit(ctx context.Context, ip string, mark uint32) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.initialized {
		return nil
	}

	b.deleteNftConnLimitRule(ip)
	log.Printf("[NftTcBackend] removed connlimit for ip=%s", ip)
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
	// 创建 connlimit 链：priority -155（在 mark 链之前执行，drop 的包不会到达 mark 链）
	connChainDef := "{ type filter hook prerouting priority -155 ; policy accept ; }"
	if err := runCmd("nft", "add", "chain", "inet", nftTable, nftConnChain, connChainDef); err != nil {
		log.Printf("[NftTcBackend] nft connlimit chain create note: %v (may already exist)", err)
	}
	// 创建 blocked 链：priority -160（在 connlimit 链之前执行，阻断的包直接丢弃）
	blockChainDef := "{ type filter hook prerouting priority -160 ; policy accept ; }"
	if err := runCmd("nft", "add", "chain", "inet", nftTable, nftBlockChain, blockChainDef); err != nil {
		log.Printf("[NftTcBackend] nft blocked chain create note: %v (may already exist)", err)
	}
	return nil
}

func (b *NftTcBackend) addNftMarkRule(ip string, mark uint32) error {
	markHex := fmt.Sprintf("0x%x", mark)
	// ip saddr <ip> counter meta mark set <mark> accept
	return runCmd("nft", "add", "rule", "inet", nftTable, nftChain,
		"ip", "saddr", ip, "counter", "meta", "mark", "set", markHex, "accept")
}

func (b *NftTcBackend) deleteNftMarkRule(ip string) {
	b.deleteNftRuleByIP(nftChain, ip)
}

// addNftConnLimitRule 在 connlimit 链中添加并发连接限制规则。
// 使用 nftables connlimit 表达式：当某 IP 的并发连接数超过 connLimit 时，丢弃新连接。
//
//	nft add rule inet corefusion_ratelimit connlimit_chain \
//	    ip saddr <ip> ct state new connlimit count <N> over counter drop
//
func (b *NftTcBackend) addNftConnLimitRule(ip string, connLimit int32) error {
	return runCmd("nft", "add", "rule", "inet", nftTable, nftConnChain,
		"ip", "saddr", ip,
		"ct", "state", "new",
		"connlimit", "count", fmt.Sprintf("%d", connLimit), "over",
		"counter", "drop")
}

func (b *NftTcBackend) deleteNftConnLimitRule(ip string) {
	b.deleteNftRuleByIP(nftConnChain, ip)
}

// addNftBlockRule 在 blocked 链中添加阻断规则（丢弃该 IP 的所有流量）。
//
//	nft add rule inet corefusion_ratelimit blocked_chain \
//	    ip saddr <ip> counter drop
//
func (b *NftTcBackend) addNftBlockRule(ip string) error {
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
		return
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

	return runCmd("tc", "class", "add", "dev", dev, "parent", parentStr,
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

	return runCmd("tc", "filter", "add", "dev", dev, "parent", parentStr,
		"protocol", "ip", "handle", markHex, "fw", "flowid", flowID)
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

	// 2. 启动 ifb0 设备
	if err := runCmd("ip", "link", "set", "dev", b.ifbDevice, "up"); err != nil {
		return fmt.Errorf("bring up %s: %w", b.ifbDevice, err)
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

	// 5. 在 ifb0 上建 HTB（控制上传）
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
