package main

import (
	"log"
	"sync"
	"time"

	"corefusion/internal/runtime/process"
)

const (
	// probeFailThreshold 连续失败多少次后暂停探测
	probeFailThreshold = 3
	// probePause 暂停时长（到点后再试一次，成功即恢复正常频率）
	probePause = 10 * time.Minute
)

// coreProbeGate 判断某核心当前是否值得去探测它的 stats / gRPC API。
//
// 背景：Agent 侧有三处会周期性探测核心 API（流量采集、在线用户上报、限速在线统计），
// 目标地址默认是 127.0.0.1:10085（xray）。以下两种情况下这些探测都是纯浪费，
// 而且会每 30s 刷一条失败日志：
//
//  1. 核心没装、或没在运行（例如只有 sing-box 的节点去连 xray）
//  2. 核心在跑，但它的 stats API 没开 / 端口不通（例如 xray 没配置 api.listen）
//
// 因此门禁做两件事：
//   - Allowed(): 只有「二进制已安装 且 进程正在运行 且 未处于失败退避期」才允许探测
//   - Report(): 每次探测结果回填；连续失败 probeFailThreshold 次后暂停 probePause，
//     期间不再探测（也就不会刷日志），到点后自动再试一次
//
// 状态变化时才打印日志。
type coreProbeGate struct {
	procMgr *process.Manager

	mu    sync.Mutex
	last  map[string]bool
	fails map[string]int
	until map[string]time.Time
}

func newCoreProbeGate(pm *process.Manager) *coreProbeGate {
	return &coreProbeGate{
		procMgr: pm,
		last:    make(map[string]bool),
		fails:   make(map[string]int),
		until:   make(map[string]time.Time),
	}
}

// Allowed 报告该核心当前是否允许探测。
func (g *coreProbeGate) Allowed(coreType string) bool {
	if g == nil || g.procMgr == nil {
		return false
	}

	// 1) 核心必须已安装且正在运行
	running := g.procMgr.IsCoreInstalled(coreType) && g.procMgr.IsCoreRunning(coreType)
	if !running {
		g.logTransition(coreType, false, "not installed or not running")
		return false
	}

	// 2) 处于失败退避期时不再探测（避免刷日志）
	g.mu.Lock()
	until, paused := g.until[coreType]
	g.mu.Unlock()
	if paused && time.Now().Before(until) {
		g.logTransition(coreType, false, "probe backoff")
		return false
	}
	if paused {
		// 退避到点：放行一次重试，并清零计数（失败会重新累计）
		g.mu.Lock()
		delete(g.until, coreType)
		g.fails[coreType] = 0
		g.mu.Unlock()
		log.Printf("[Agent] %s probe backoff elapsed, retrying stats API", coreType)
	}

	g.logTransition(coreType, true, "")
	return true
}

// Report 回填一次探测结果；连续失败达到阈值后进入退避期。
func (g *coreProbeGate) Report(coreType string, ok bool) {
	if g == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if ok {
		if g.fails[coreType] > 0 {
			log.Printf("[Agent] %s stats API reachable again", coreType)
		}
		delete(g.fails, coreType)
		delete(g.until, coreType)
		return
	}
	g.fails[coreType]++
	if g.fails[coreType] >= probeFailThreshold {
		g.until[coreType] = time.Now().Add(probePause)
		log.Printf("[Agent] %s stats API unreachable %d times in a row, pausing probes for %s",
			coreType, g.fails[coreType], probePause)
		g.fails[coreType] = 0
		g.last[coreType] = false
	}
}

// logTransition 仅在「允许/不允许」状态变化时打印一行日志。
func (g *coreProbeGate) logTransition(coreType string, allowed bool, reason string) {
	g.mu.Lock()
	prev, seen := g.last[coreType]
	if seen && prev == allowed {
		g.mu.Unlock()
		return
	}
	g.last[coreType] = allowed
	g.mu.Unlock()

	if allowed {
		log.Printf("[Agent] %s is running, stats probing enabled", coreType)
		return
	}
	if reason == "probe backoff" {
		return // 退避期间不打重复日志（进入退避时已提示）
	}
	log.Printf("[Agent] %s not installed or not running, skip stats probing", coreType)
}
