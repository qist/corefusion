package agent

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"sort"
	"sync"
	"time"

	"corefusion/internal/store/repository"
)

// LoadBalancer provides weight-based node selection for subscription delivery.
//
// 选择策略：
//   1. 优先级排序：在线 > 离线，低延迟 > 高延迟，高权重 > 低权重
//   2. 权重分配：按权重比例分配用户到不同节点
//   3. 区域感知：优先选择用户 GeoIP 匹配的区域
//   4. 故障排除：自动排除离线或测速失败的节点
//
// 与 SpeedTestManager 协同工作：测速结果影响排序权重。
type LoadBalancer struct {
	mu         sync.RWMutex
	serverRepo *repository.NodeServerRepository
	nodeMgr    *NodeServerManager
	speedTest  *SpeedTestManager

	// 配置
	strategy            LBStrategy
	maxNodesPerSub      int           // 每个订阅最多返回的节点数
	latencyThreshold    int32         // 延迟阈值（ms），超过则降权
	failurePenalty      float64       // 测速失败的惩罚权重（0-1）
	regionBonus         float64       // 同区域加分权重
}

// LBStrategy defines the load balancing strategy.
type LBStrategy string

const (
	// LBStrategyWeighted: 按权重随机分配（默认）
	LBStrategyWeighted LBStrategy = "weighted"
	// LBStrategyLatency: 纯延迟排序（最低延迟优先）
	LBStrategyLatency LBStrategy = "latency"
	// LBStrategyRoundRobin: 轮询分配
	LBStrategyRoundRobin LBStrategy = "round_robin"
	// LBStrategyRegionFirst: 区域优先 + 权重
	LBStrategyRegionFirst LBStrategy = "region_first"
)

// NodeCandidate represents a selectable node for load balancing.
type NodeCandidate struct {
	ServerID    int64
	Hostname    string
	Region      string
	NodeType    string
	Weight      int     // 原始权重（来自数据库或配置）
	Score       float64 // 计算后的综合得分
	LatencyMs   int32   // 最新延迟
	IsOnline    bool
	BandwidthMbps int   // 带宽上限
	IsLocal     bool
}

// NewLoadBalancer creates a new LoadBalancer.
func NewLoadBalancer(
	serverRepo *repository.NodeServerRepository,
	nodeMgr *NodeServerManager,
) *LoadBalancer {
	return &LoadBalancer{
		serverRepo:       serverRepo,
		nodeMgr:          nodeMgr,
		strategy:         LBStrategyWeighted,
		maxNodesPerSub:   10,
		latencyThreshold: 500,
		failurePenalty:   0.1,
		regionBonus:      2.0,
	}
}

// WithSpeedTest injects the SpeedTestManager for latency-aware balancing.
func (lb *LoadBalancer) WithSpeedTest(stm *SpeedTestManager) *LoadBalancer {
	lb.speedTest = stm
	return lb
}

// WithStrategy sets the load balancing strategy.
func (lb *LoadBalancer) WithStrategy(s LBStrategy) *LoadBalancer {
	lb.strategy = s
	return lb
}

// WithMaxNodesPerSub sets the maximum number of nodes returned per subscription.
func (lb *LoadBalancer) WithMaxNodesPerSub(n int) *LoadBalancer {
	lb.maxNodesPerSub = n
	return lb
}

// WithLatencyThreshold sets the latency threshold in milliseconds.
func (lb *LoadBalancer) WithLatencyThreshold(ms int32) *LoadBalancer {
	lb.latencyThreshold = ms
	return lb
}

// SelectNodes selects the best nodes for a subscription request.
//
// Parameters:
//   - userRegion: the user's GeoIP region (e.g., "CN", "HK", "US")
//   - count: number of nodes to select (0 = use maxNodesPerSub)
//   - excludeIDs: server IDs to exclude (e.g., already assigned)
//
// Returns a sorted list of NodeCandidate, best first.
func (lb *LoadBalancer) SelectNodes(ctx context.Context, userRegion string, count int, excludeIDs []int64) ([]NodeCandidate, error) {
	if count <= 0 {
		count = lb.maxNodesPerSub
	}

	// Get all enabled servers
	servers, err := lb.serverRepo.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("list servers: %w", err)
	}

	// Build exclusion set
	exclude := make(map[int64]bool)
	for _, id := range excludeIDs {
		exclude[id] = true
	}

	// Build candidates
	candidates := make([]NodeCandidate, 0, len(servers))
	for _, s := range servers {
		if !s.Enabled || exclude[s.ID] {
			continue
		}

		// Skip chain_exit nodes (they don't serve users directly)
		if s.NodeType == "chain_exit" {
			continue
		}

		candidate := NodeCandidate{
			ServerID:      s.ID,
			Hostname:      s.Hostname,
			Region:        s.Region,
			NodeType:      s.NodeType,
			Weight:        lb.getNodeWeight(s.SortOrder),
			IsOnline:      lb.nodeMgr.IsOnlineForServer(s.ID),
			BandwidthMbps: s.BandwidthMbps,
			IsLocal:       s.IsLocal,
		}

		// Get latency from speed test if available
		if lb.speedTest != nil {
			if result := lb.speedTest.GetResult(s.ID); result != nil && result.Error == "" {
				candidate.LatencyMs = result.LatencyMs
			}
		}

		// Calculate score
		candidate.Score = lb.calculateScore(candidate, userRegion)
		candidates = append(candidates, candidate)
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no available nodes")
	}

	// Filter: only online nodes
	onlineCandidates := make([]NodeCandidate, 0, len(candidates))
	for _, c := range candidates {
		if c.IsOnline {
			onlineCandidates = append(onlineCandidates, c)
		}
	}

	// If no online nodes, use all (degraded mode)
	if len(onlineCandidates) == 0 {
		onlineCandidates = candidates
	}

	// Sort by strategy
	switch lb.strategy {
	case LBStrategyLatency:
		sort.Slice(onlineCandidates, func(i, j int) bool {
			return onlineCandidates[i].LatencyMs < onlineCandidates[j].LatencyMs
		})

	case LBStrategyWeighted:
		// Weighted random selection
		onlineCandidates = lb.weightedSelection(onlineCandidates, count)

	case LBStrategyRoundRobin:
		// Simple round-robin (just rotate)
		if len(onlineCandidates) > 0 {
			offset := int(time.Now().Unix()) % len(onlineCandidates)
			rotated := make([]NodeCandidate, len(onlineCandidates))
			for i := range onlineCandidates {
				rotated[i] = onlineCandidates[(i+offset)%len(onlineCandidates)]
			}
			onlineCandidates = rotated
		}
		// Sort by score for consistent ordering after rotation
		sort.Slice(onlineCandidates, func(i, j int) bool {
			return onlineCandidates[i].Score > onlineCandidates[j].Score
		})

	case LBStrategyRegionFirst:
		// Sort: same region first, then by score
		sort.Slice(onlineCandidates, func(i, j int) bool {
			iRegion := onlineCandidates[i].Region == userRegion
			jRegion := onlineCandidates[j].Region == userRegion
			if iRegion != jRegion {
				return iRegion
			}
			return onlineCandidates[i].Score > onlineCandidates[j].Score
		})

	default:
		// Default: sort by score (highest first)
		sort.Slice(onlineCandidates, func(i, j int) bool {
			return onlineCandidates[i].Score > onlineCandidates[j].Score
		})
	}

	// Limit to requested count
	if len(onlineCandidates) > count {
		onlineCandidates = onlineCandidates[:count]
	}

	return onlineCandidates, nil
}

// calculateScore computes a composite score for a node candidate.
// Higher score = better candidate.
func (lb *LoadBalancer) calculateScore(c NodeCandidate, userRegion string) float64 {
	score := float64(c.Weight)

	// Latency bonus: lower latency = higher score
	if c.LatencyMs > 0 {
		if c.LatencyMs < lb.latencyThreshold {
			// Good latency: add bonus
			score += float64(lb.latencyThreshold-c.LatencyMs) / float64(lb.latencyThreshold) * 100
		} else {
			// Bad latency: reduce score
			score *= lb.failurePenalty
		}
	}

	// Region bonus: same region as user gets bonus
	if userRegion != "" && c.Region == userRegion {
		score *= lb.regionBonus
	}

	// Bandwidth bonus: higher bandwidth = slight bonus
	if c.BandwidthMbps > 0 {
		score += float64(c.BandwidthMbps) / 1000.0
	}

	// Online bonus
	if c.IsOnline {
		score *= 2.0
	} else {
		score *= 0.1 // Offline nodes get heavily penalized
	}

	// Local server gets a small bonus (lower latency for local users)
	if c.IsLocal {
		score *= 1.5
	}

	return score
}

// weightedSelection performs weighted random selection.
func (lb *LoadBalancer) weightedSelection(candidates []NodeCandidate, count int) []NodeCandidate {
	if len(candidates) <= count {
		// Sort by score and return all
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].Score > candidates[j].Score
		})
		return candidates
	}

	// Calculate total weight
	totalScore := 0.0
	for _, c := range candidates {
		if c.Score > 0 {
			totalScore += c.Score
		}
	}

	if totalScore <= 0 {
		// All scores are zero, just return first N
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].Score > candidates[j].Score
		})
		return candidates[:count]
	}

	// Weighted random selection without replacement
	selected := make([]NodeCandidate, 0, count)
	remaining := make([]NodeCandidate, len(candidates))
	copy(remaining, candidates)

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	for len(selected) < count && len(remaining) > 0 {
		totalScore = 0
		for _, c := range remaining {
			if c.Score > 0 {
				totalScore += c.Score
			}
		}

		if totalScore <= 0 {
			break
		}

		r := rng.Float64() * totalScore
		cumulative := 0.0
		selectedIdx := -1

		for i, c := range remaining {
			if c.Score > 0 {
				cumulative += c.Score
				if r <= cumulative {
					selectedIdx = i
					break
				}
			}
		}

		if selectedIdx < 0 {
			selectedIdx = len(remaining) - 1
		}

		selected = append(selected, remaining[selectedIdx])
		remaining = append(remaining[:selectedIdx], remaining[selectedIdx+1:]...)
	}

	// Sort selected by score for consistent ordering
	sort.Slice(selected, func(i, j int) bool {
		return selected[i].Score > selected[j].Score
	})

	return selected
}

// getNodeWeight returns the weight for a server based on its sort order.
// Higher sort_order = higher weight. Default is 100.
func (lb *LoadBalancer) getNodeWeight(sortOrder int) int {
	if sortOrder <= 0 {
		return 100
	}
	return 100 + sortOrder*10
}

// GetServerLoad returns the current load estimate for a server.
// This is a rough estimate based on connection count from heartbeat data.
func (lb *LoadBalancer) GetServerLoad(serverID int64) int {
	ac := lb.nodeMgr.GetConnection(serverID)
	if ac == nil {
		return 0
	}
	ac.mu.Lock()
	defer ac.mu.Unlock()

	// Estimate load from core count
	load := 0
	for _, cs := range ac.coreStates {
		if cs.State == "running" {
			load++
		}
	}
	return load
}

// LogSelection logs the node selection for debugging.
func (lb *LoadBalancer) LogSelection(userRegion string, selected []NodeCandidate) {
	log.Printf("[LoadBalancer] region=%s strategy=%s selected %d node(s):", userRegion, lb.strategy, len(selected))
	for _, c := range selected {
		log.Printf("[LoadBalancer]   server %d (%s) region=%s latency=%dms score=%.1f online=%v",
			c.ServerID, c.Hostname, c.Region, c.LatencyMs, c.Score, c.IsOnline)
	}
}
